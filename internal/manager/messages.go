package manager

import (
	"time"

	"github.com/kopecmaciej/tview"
)

// Message type constants. No direct use when broadcasting, instead use below constructors
const (
	FocusChanged           MessageType = "focus_changed"
	StyleChanged           MessageType = "style_changed"
	ConfigChanged          MessageType = "config_changed"
	UpdateAutocompleteKeys MessageType = "update_autocomplete"
	FooterHeightChanged    MessageType = "footer_height_changed"
	MCPStateChanged        MessageType = "mcp_state_changed"
	OpenQueryTab           MessageType = "open_query_tab"
	UpdateQueryTab         MessageType = "update_query_tab"
	OpenTableTab           MessageType = "open_table_tab"
	QueryExecuted          MessageType = "query_executed"
	SequencePendingChanged MessageType = "sequence_pending_changed"
)

// OpenQueryTabRequest is broadcast by the MCP open_query_in_tab tool.
type OpenQueryTabRequest struct {
	TabID string
	Query string
	Name  string
}

// UpdateQueryTabRequest is broadcast by the MCP update_query_in_tab tool.
type UpdateQueryTabRequest struct {
	TabID string
	Query string
}

// TableTabRequest is broadcast when FK navigation needs to open a new table tab.
type TableTabRequest struct {
	Schema      string
	Table       string
	Where       string
	FocusColumn string // column name to select after loading (empty = default col 0)
}

// QueryResult is broadcast after the user executes a query and also
// returned directly by the get_last_query_result MCP tool.
type QueryResult struct {
	Query      string           `json:"query"`
	Columns    []string         `json:"columns,omitempty"`
	Rows       []map[string]any `json:"rows,omitempty"`
	RowCount   int              `json:"row_count"`
	Affected   int64            `json:"affected_rows,omitempty"`
	ExecutedAt *time.Time       `json:"executed_at,omitempty"`
}

func NewFocusChangedMsg(id tview.Identifier) EventMsg {
	return EventMsg{Message: Message{Type: FocusChanged, Data: id}}
}

func NewStyleChangedMsg() EventMsg {
	return EventMsg{Message: Message{Type: StyleChanged}}
}

func NewConfigChangedMsg() EventMsg {
	return EventMsg{Message: Message{Type: ConfigChanged}}
}

func NewUpdateAutocompleteKeysMsg(cols []string) EventMsg {
	return EventMsg{Message: Message{Type: UpdateAutocompleteKeys, Data: cols}}
}

func NewFooterHeightChangedMsg(height int) EventMsg {
	return EventMsg{Message: Message{Type: FooterHeightChanged, Data: height}}
}

func NewMCPStateChangedMsg(enabled bool) EventMsg {
	return EventMsg{Message: Message{Type: MCPStateChanged, Data: enabled}}
}

func NewOpenQueryTabMsg(req OpenQueryTabRequest) EventMsg {
	return EventMsg{Message: Message{Type: OpenQueryTab, Data: req}}
}

func NewUpdateQueryTabMsg(req UpdateQueryTabRequest) EventMsg {
	return EventMsg{Message: Message{Type: UpdateQueryTab, Data: req}}
}

func NewOpenTableTabMsg(req TableTabRequest) EventMsg {
	return EventMsg{Message: Message{Type: OpenTableTab, Data: req}}
}

func NewQueryExecutedMsg(result QueryResult) EventMsg {
	return EventMsg{Message: Message{Type: QueryExecuted, Data: result}}
}

// NewSequencePendingChangedMsg broadcasts the current sequence prefix ("" = cleared).
func NewSequencePendingChangedMsg(s string) EventMsg {
	return EventMsg{Message: Message{Type: SequencePendingChanged, Data: s}}
}
