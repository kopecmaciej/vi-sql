package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog/log"
)

const maxRows = 100

func (s *Server) registerTools() {
	mcpsdk.AddTool(s.server, &mcpsdk.Tool{
		Name:        "list_schemas",
		Description: "List all schemas and their tables in the connected database",
	}, s.handleListSchemas)

	mcpsdk.AddTool(s.server, &mcpsdk.Tool{
		Name:        "describe_table",
		Description: "Get columns, constraints, foreign keys, and indexes for a table",
	}, s.handleDescribeTable)

	mcpsdk.AddTool(s.server, &mcpsdk.Tool{
		Name:        "list_enum_types",
		Description: "List all custom enum types and their valid values in the database",
	}, s.handleListEnumTypes)

	mcpsdk.AddTool(s.server, &mcpsdk.Tool{
		Name:        "sample_table",
		Description: "Return up to N rows from a table without writing SQL",
	}, s.handleSampleTable)

	mcpsdk.AddTool(s.server, &mcpsdk.Tool{
		Name:        "open_query_in_tab",
		Description: "Open a new query tab in vi-sql and pre-fill the SQL editor with the given query (does not execute it). Returns a tab_id you can later pass to get_query_from_tab.",
	}, s.handleOpenQueryInTab)

	mcpsdk.AddTool(s.server, &mcpsdk.Tool{
		Name:        "get_query_from_tab",
		Description: "Return the current editor text from a specific query tab opened by open_query_in_tab. Works regardless of which tab is currently active.",
	}, s.handleGetQueryFromTab)

	mcpsdk.AddTool(s.server, &mcpsdk.Tool{
		Name:        "get_last_query_result",
		Description: "Return the result of the last query executed by the user inside the vi-sql app",
	}, s.handleGetLastQueryResult)

	if s.cfg.AllowExecute {
		mcpsdk.AddTool(s.server, &mcpsdk.Tool{
			Name:        "execute_query",
			Description: "Execute a read-only SQL SELECT query and return results (max 100 rows)",
		}, s.handleExecuteQuery)

		if s.cfg.AllowWrite {
			mcpsdk.AddTool(s.server, &mcpsdk.Tool{
				Name:        "execute_statement",
				Description: "Execute an INSERT/UPDATE/DELETE/DDL statement. Returns affected row count.",
			}, s.handleExecuteStatement)
		}
	}

	mcpsdk.AddTool(s.server, &mcpsdk.Tool{
		Name:        "explain_query",
		Description: "Return the query plan for a SQL statement. Set analyze=true to also execute and return runtime stats.",
	}, s.handleExplainQuery)

	mcpsdk.AddTool(s.server, &mcpsdk.Tool{
		Name:        "get_server_info",
		Description: "Get database server version, name, and connection info",
	}, s.handleGetServerInfo)
}

type listSchemasInput struct {
	Filter string `json:"filter" jsonschema:"Optional name filter"`
}

type describeTableInput struct {
	Schema string `json:"schema" jsonschema:"Schema name,required"`
	Table  string `json:"table"  jsonschema:"Table name,required"`
}

type executeQueryInput struct {
	Query string `json:"query" jsonschema:"SQL SELECT query,required"`
}

type executeStatementInput struct {
	Statement string `json:"statement" jsonschema:"SQL DML/DDL statement,required"`
}

type getServerInfoInput struct{}

type listEnumTypesInput struct{}

type sampleTableInput struct {
	Schema string `json:"schema" jsonschema:"Schema name,required"`
	Table  string `json:"table"  jsonschema:"Table name,required"`
	Limit  int    `json:"limit"  jsonschema:"Number of rows to return (default 10, max 100)"`
}

type explainQueryInput struct {
	Query   string `json:"query"   jsonschema:"SQL query to explain,required"`
	Analyze bool   `json:"analyze" jsonschema:"If true, executes the query and includes runtime statistics"`
}

type openQueryInTabInput struct {
	Query string `json:"query" jsonschema:"SQL query to pre-fill in the editor,required"`
	Name  string `json:"name"  jsonschema:"Optional display name for the tab"`
}

type getQueryFromTabInput struct {
	TabID string `json:"tab_id" jsonschema:"Tab ID returned by open_query_in_tab,required"`
}

type getLastQueryResultInput struct{}

func (s *Server) handleListSchemas(
	ctx context.Context, _ *mcpsdk.CallToolRequest, input listSchemasInput,
) (*mcpsdk.CallToolResult, any, error) {
	log.Debug().Str("tool", "list_schemas").Msg("MCP tool called")
	schemas, err := s.driver.ListSchemas(ctx, input.Filter)
	if err != nil {
		return nil, nil, fmt.Errorf("list_schemas: %w", err)
	}
	return jsonResult(schemas)
}

func (s *Server) handleDescribeTable(
	ctx context.Context, _ *mcpsdk.CallToolRequest, input describeTableInput,
) (*mcpsdk.CallToolResult, any, error) {
	log.Debug().Str("tool", "describe_table").Str("schema", input.Schema).Str("table", input.Table).Msg("MCP tool called")
	if input.Schema == "" || input.Table == "" {
		return nil, nil, fmt.Errorf("schema and table are required")
	}

	cols, err := s.driver.GetTableColumns(ctx, input.Schema, input.Table)
	if err != nil {
		return nil, nil, fmt.Errorf("describe_table columns: %w", err)
	}
	constraints, err := s.driver.GetTableConstraints(ctx, input.Schema, input.Table)
	if err != nil {
		return nil, nil, fmt.Errorf("describe_table constraints: %w", err)
	}
	fks, err := s.driver.GetTableForeignKeys(ctx, input.Schema, input.Table)
	if err != nil {
		return nil, nil, fmt.Errorf("describe_table foreign_keys: %w", err)
	}
	indexes, err := s.driver.GetIndexes(ctx, input.Schema, input.Table)
	if err != nil {
		return nil, nil, fmt.Errorf("describe_table indexes: %w", err)
	}

	result := map[string]any{
		"columns":      cols,
		"constraints":  constraints,
		"foreign_keys": fks,
		"indexes":      indexes,
	}
	return jsonResult(result)
}

func (s *Server) handleExecuteQuery(
	ctx context.Context, _ *mcpsdk.CallToolRequest, input executeQueryInput,
) (*mcpsdk.CallToolResult, any, error) {
	log.Debug().Str("tool", "execute_query").Msg("MCP tool called")
	if input.Query == "" {
		return nil, nil, fmt.Errorf("query is required")
	}

	if !s.cfg.AllowWrite {
		if err := assertReadOnly(input.Query); err != nil {
			return nil, nil, err
		}
	}

	rows, cols, err := s.driver.ExecuteQuery(ctx, input.Query)
	if err != nil {
		return nil, nil, fmt.Errorf("execute_query: %w", err)
	}

	if len(rows) > maxRows {
		rows = rows[:maxRows]
	}

	colNames := make([]string, len(cols))
	for i, c := range cols {
		colNames[i] = c.Name
	}

	result := map[string]any{
		"columns":   colNames,
		"rows":      rows,
		"row_count": len(rows),
	}
	return jsonResult(result)
}

func (s *Server) handleExecuteStatement(
	ctx context.Context, _ *mcpsdk.CallToolRequest, input executeStatementInput,
) (*mcpsdk.CallToolResult, any, error) {
	log.Debug().Str("tool", "execute_statement").Msg("MCP tool called")
	if input.Statement == "" {
		return nil, nil, fmt.Errorf("statement is required")
	}

	affected, err := s.driver.ExecuteStatement(ctx, input.Statement)
	if err != nil {
		return nil, nil, fmt.Errorf("execute_statement: %w", err)
	}

	result := map[string]any{"affected_rows": affected}
	return jsonResult(result)
}

func (s *Server) handleGetServerInfo(
	ctx context.Context, _ *mcpsdk.CallToolRequest, _ getServerInfoInput,
) (*mcpsdk.CallToolResult, any, error) {
	log.Debug().Str("tool", "get_server_info").Msg("MCP tool called")
	info, err := s.driver.GetServerInfo(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("get_server_info: %w", err)
	}
	return jsonResult(info)
}

func (s *Server) handleListEnumTypes(
	ctx context.Context, _ *mcpsdk.CallToolRequest, _ listEnumTypesInput,
) (*mcpsdk.CallToolResult, any, error) {
	log.Debug().Str("tool", "list_enum_types").Msg("MCP tool called")
	const q = `
		SELECT t.typname AS type, e.enumlabel AS value
		FROM pg_type t
		JOIN pg_enum e ON e.enumtypid = t.oid
		ORDER BY t.typname, e.enumsortorder`

	rows, _, err := s.driver.ExecuteQuery(ctx, q)
	if err != nil {
		return nil, nil, fmt.Errorf("list_enum_types: %w", err)
	}

	grouped := map[string][]string{}
	order := []string{}
	for _, row := range rows {
		typ := fmt.Sprintf("%v", row["type"])
		val := fmt.Sprintf("%v", row["value"])
		if _, seen := grouped[typ]; !seen {
			order = append(order, typ)
		}
		grouped[typ] = append(grouped[typ], val)
	}

	type enumType struct {
		Name   string   `json:"name"`
		Values []string `json:"values"`
	}
	result := make([]enumType, len(order))
	for i, name := range order {
		result[i] = enumType{Name: name, Values: grouped[name]}
	}
	return jsonResult(result)
}

func (s *Server) handleSampleTable(
	ctx context.Context, _ *mcpsdk.CallToolRequest, input sampleTableInput,
) (*mcpsdk.CallToolResult, any, error) {
	log.Debug().Str("tool", "sample_table").Str("schema", input.Schema).Str("table", input.Table).Msg("MCP tool called")
	if input.Schema == "" || input.Table == "" {
		return nil, nil, fmt.Errorf("schema and table are required")
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	q := fmt.Sprintf("SELECT * FROM %s.%s LIMIT %d", input.Schema, input.Table, limit)
	rows, cols, err := s.driver.ExecuteQuery(ctx, q)
	if err != nil {
		return nil, nil, fmt.Errorf("sample_table: %w", err)
	}

	colNames := make([]string, len(cols))
	for i, c := range cols {
		colNames[i] = c.Name
	}
	result := map[string]any{
		"columns":   colNames,
		"rows":      rows,
		"row_count": len(rows),
	}
	return jsonResult(result)
}

func (s *Server) handleExplainQuery(
	ctx context.Context, _ *mcpsdk.CallToolRequest, input explainQueryInput,
) (*mcpsdk.CallToolResult, any, error) {
	log.Debug().Str("tool", "explain_query").Bool("analyze", input.Analyze).Msg("MCP tool called")
	if input.Query == "" {
		return nil, nil, fmt.Errorf("query is required")
	}

	var (
		plan string
		err  error
	)
	if input.Analyze {
		plan, err = s.driver.ExplainAnalyze(ctx, input.Query)
	} else {
		plan, err = s.driver.ExplainPlan(ctx, input.Query)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("explain_query: %w", err)
	}

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: plan},
		},
	}, nil, nil
}

func (s *Server) handleOpenQueryInTab(
	_ context.Context, _ *mcpsdk.CallToolRequest, input openQueryInTabInput,
) (*mcpsdk.CallToolResult, any, error) {
	log.Debug().Str("tool", "open_query_in_tab").Msg("MCP tool called")
	if input.Query == "" {
		return nil, nil, fmt.Errorf("query is required")
	}
	if s.manager == nil {
		return nil, nil, fmt.Errorf("open_query_in_tab: TUI manager not available")
	}

	tabID := uuid.New().String()
	s.manager.Broadcast(manager.EventMsg{
		Message: manager.Message{
			Type: manager.OpenQueryTab,
			Data: manager.OpenQueryTabRequest{
				TabID: tabID,
				Query: input.Query,
				Name:  input.Name,
			},
		},
	})

	return jsonResult(map[string]string{"tab_id": tabID, "status": "Query opened in new tab"})
}

func (s *Server) handleGetQueryFromTab(
	_ context.Context, _ *mcpsdk.CallToolRequest, input getQueryFromTabInput,
) (*mcpsdk.CallToolResult, any, error) {
	log.Debug().Str("tool", "get_query_from_tab").Str("tab_id", input.TabID).Msg("MCP tool called")
	if input.TabID == "" {
		return nil, nil, fmt.Errorf("tab_id is required")
	}
	if s.tabRegistry == nil {
		return nil, nil, fmt.Errorf("get_query_from_tab: tab registry not available")
	}

	text, ok := s.tabRegistry.GetText(input.TabID)
	if !ok {
		return nil, nil, fmt.Errorf("tab %q not found or has been closed", input.TabID)
	}
	return jsonResult(map[string]string{"tab_id": input.TabID, "query": text})
}

func (s *Server) handleGetLastQueryResult(
	_ context.Context, _ *mcpsdk.CallToolRequest, _ getLastQueryResultInput,
) (*mcpsdk.CallToolResult, any, error) {
	log.Debug().Str("tool", "get_last_query_result").Msg("MCP tool called")
	s.resultMu.RLock()
	result := s.lastResult
	s.resultMu.RUnlock()

	if result == nil {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: "No query has been executed in vi-sql yet"},
			},
		}, nil, nil
	}
	return jsonResult(result)
}

func jsonResult(v any) (*mcpsdk.CallToolResult, any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, nil, fmt.Errorf("json marshal: %w", err)
	}
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: string(data)},
		},
	}, nil, nil
}
