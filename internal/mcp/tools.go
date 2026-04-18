package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const maxRows = 1000

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
		Name:        "explain_query",
		Description: "Return the query plan for a SQL statement. Set analyze=true to also execute and return runtime stats.",
	}, s.handleExplainQuery)

	mcpsdk.AddTool(s.server, &mcpsdk.Tool{
		Name:        "execute_query",
		Description: "Execute a read-only SQL SELECT query and return results (max 1000 rows)",
	}, s.handleExecuteQuery)

	if s.cfg.AllowWrite {
		mcpsdk.AddTool(s.server, &mcpsdk.Tool{
			Name:        "execute_statement",
			Description: "Execute an INSERT/UPDATE/DELETE/DDL statement. Returns affected row count.",
		}, s.handleExecuteStatement)
	}

	mcpsdk.AddTool(s.server, &mcpsdk.Tool{
		Name:        "get_server_info",
		Description: "Get database server version, name, and connection info",
	}, s.handleGetServerInfo)
}

// --- input types ---

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

// --- handlers ---

func (s *Server) handleListSchemas(
	ctx context.Context, _ *mcpsdk.CallToolRequest, input listSchemasInput,
) (*mcpsdk.CallToolResult, any, error) {
	schemas, err := s.driver.ListSchemas(ctx, input.Filter)
	if err != nil {
		return nil, nil, fmt.Errorf("list_schemas: %w", err)
	}
	return jsonResult(schemas)
}

func (s *Server) handleDescribeTable(
	ctx context.Context, _ *mcpsdk.CallToolRequest, input describeTableInput,
) (*mcpsdk.CallToolResult, any, error) {
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
	info, err := s.driver.GetServerInfo(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("get_server_info: %w", err)
	}
	return jsonResult(info)
}

func (s *Server) handleListEnumTypes(
	ctx context.Context, _ *mcpsdk.CallToolRequest, _ listEnumTypesInput,
) (*mcpsdk.CallToolResult, any, error) {
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

// --- helpers ---

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

