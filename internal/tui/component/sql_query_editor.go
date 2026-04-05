package component

import (
	"slices"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

const SQLQueryEditorId = "SQLQueryEditor"

// SQLQueryEditor is an in-TUI multi-line SQL editor backed by a tview.TextArea.
// It supports syntax highlighting via SetStyleFunc and context-aware autocomplete.
const (
	sqlEditorDefaultHeight  = 10
	sqlEditorExpandedHeight = 20
)

type SQLQueryEditor struct {
	*core.BaseElement
	*tview.TextArea

	style         *config.SQLEditorStyle
	schemas       []database.SchemaWithTables
	columns       []string
	columnCache   map[string][]string // key: "schema.table" or "table"
	columnFetcher func(schema, table string) ([]string, error)
	onExecute     func(sql string)
	queryBarText  func() string
	onClose       func()
	onExpand      func()
	expanded      bool
}

func NewSQLQueryEditor() *SQLQueryEditor {
	e := &SQLQueryEditor{
		BaseElement: core.NewBaseElement(),
		TextArea:    tview.NewTextArea(),
		columnCache: make(map[string][]string),
	}
	e.SetIdentifier(SQLQueryEditorId)
	e.SetAfterInitFunc(e.init)
	return e
}

func (e *SQLQueryEditor) init() error {
	e.style = &e.App.GetStyles().SQLEditor
	e.setStyle()
	e.setAutocomplete()
	e.setHighlighting()
	e.handleEvents()
	return nil
}

func (e *SQLQueryEditor) setStyle() {
	styles := e.App.GetStyles()
	bg := styles.Global.BackgroundColor.Color()
	fg := styles.Global.TextColor.Color()
	border := styles.Global.FocusColor.Color()

	e.TextArea.SetTextStyle(tcell.StyleDefault.Background(bg).Foreground(fg))
	e.TextArea.SetBorder(true)
	e.TextArea.SetBorderColor(border)
	e.TextArea.SetTitle(" SQL Editor ")
	e.TextArea.SetTitleAlign(tview.AlignCenter)
	e.TextArea.SetBorderPadding(0, 0, 1, 1)

	a := styles.InputBar.Autocomplete
	acBackground := a.BackgroundColor.Color()
	acMain := tcell.StyleDefault.
		Background(acBackground).
		Foreground(a.TextColor.Color())
	acSelected := tcell.StyleDefault.
		Background(a.ActiveBackgroundColor.Color()).
		Foreground(a.ActiveTextColor.Color())
	e.TextArea.SetAutocompleteStyles(acBackground, acMain, acSelected)
	e.TextArea.SetAutocompleteBorderColor(a.BorderColor.Color())
}

func (e *SQLQueryEditor) setHighlighting() {
	type cache struct {
		text   string
		tokens []database.Token
	}
	var c cache

	e.SetStyleFunc(func(byteOffset int) tcell.Style {
		text := e.GetText()
		if c.text != text {
			c.text = text
			c.tokens = database.Tokenize(text)
		}
		return core.SQLTokenStyle(c.tokens, byteOffset, e.style)
	})
}

// resolveColumnsForQuery merges columns from all tables referenced in the FROM/JOIN
// clauses of text. Falls back to e.columns when no references are resolvable.
func (e *SQLQueryEditor) resolveColumnsForQuery(text string) []string {
	refs := database.ExtractFromTableRefs(text)
	if len(refs) == 0 {
		return e.columns
	}

	seen := make(map[string]bool)
	var merged []string

	for _, ref := range refs {
		schema := ref.Schema
		table := ref.Table

		// Resolve schema if missing
		if schema == "" {
			for _, s := range e.schemas {
				for _, t := range s.Tables {
					if strings.EqualFold(t, table) {
						schema = s.Schema
						break
					}
				}
				if schema != "" {
					break
				}
			}
		}

		key := schema + "." + table
		cols, ok := e.columnCache[key]
		if !ok && key == "."+table {
			// try unqualified key
			cols, ok = e.columnCache[table]
		}
		if !ok && e.columnFetcher != nil {
			fetched, err := e.columnFetcher(schema, table)
			if err == nil {
				cols = fetched
				if key != "."+table {
					e.columnCache[key] = cols
				}
				e.columnCache[table] = cols
			}
		}

		for _, col := range cols {
			if !seen[col] {
				seen[col] = true
				merged = append(merged, col)
			}
		}
	}

	if len(merged) == 0 {
		return e.columns
	}
	return merged
}

// resolveColumnsForTable returns columns for a specific table (for CtxAfterDot).
func (e *SQLQueryEditor) resolveColumnsForTable(tableName string) []string {
	// Try qualified keys first
	for _, s := range e.schemas {
		for _, t := range s.Tables {
			if strings.EqualFold(t, tableName) {
				key := s.Schema + "." + t
				if cols, ok := e.columnCache[key]; ok {
					return cols
				}
				if e.columnFetcher != nil {
					cols, err := e.columnFetcher(s.Schema, t)
					if err == nil {
						e.columnCache[key] = cols
						e.columnCache[t] = cols
						return cols
					}
				}
			}
		}
	}
	// Fall back to unqualified cache
	if cols, ok := e.columnCache[tableName]; ok {
		return cols
	}
	return e.columns
}

func (e *SQLQueryEditor) setAutocomplete() {
	e.TextArea.SetAutocompleteFunc(func(text string, cursorBytePos int) []tview.AutocompleteItem {
		if cursorBytePos > 0 && strings.HasSuffix(strings.TrimSpace(text[:cursorBytePos]), ";") {
			return nil
		}

		// Detect context to decide which columns to pass
		tokens := database.Tokenize(text)
		ctx := database.DetectContext(tokens, cursorBytePos)

		var cols []string
		switch ctx.Type {
		case database.CtxAfterDot:
			isSchema := slices.ContainsFunc(e.schemas, func(s database.SchemaWithTables) bool {
				return strings.EqualFold(ctx.TableName, s.Schema)
			})
			if !isSchema && ctx.TableName != "" {
				cols = e.resolveColumnsForTable(ctx.TableName)
			} else {
				cols = e.columns
			}
		case database.CtxAfterSelect, database.CtxAfterWhere, database.CtxAfterOn,
			database.CtxAfterOrderBy, database.CtxAfterGroupBy, database.CtxAfterSet:
			cols = e.resolveColumnsForQuery(text)
		default:
			cols = e.columns
		}

		entries := database.BuildSQLAutocomplete(text, cursorBytePos, e.schemas, cols)
		items := make([]tview.AutocompleteItem, len(entries))
		for i, en := range entries {
			items[i] = tview.AutocompleteItem{Main: en.Main, Secondary: en.Secondary}
		}
		return items
	})

	e.TextArea.SetAutocompletedFunc(func(text string, index int, source int) bool {
		if source == tview.AutocompletedNavigate {
			return false
		}
		before := e.GetTextBeforeCursor()
		ctx := database.DetectContext(database.Tokenize(e.GetText()), len(before))
		startPos := len(before) - len(ctx.PartialWord)
		e.Replace(startPos, len(before), text)
		return true
	})
}

func (e *SQLQueryEditor) handleEvents() {
	go e.HandleEvents(SQLQueryEditorId, func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			e.style = &e.App.GetStyles().SQLEditor
			e.setStyle()
			e.setHighlighting()
		}
	})
}

// SetSchemas updates the list of schemas/tables for FROM/JOIN autocomplete.
func (e *SQLQueryEditor) SetSchemas(schemas []database.SchemaWithTables) {
	e.schemas = schemas
}

// SetColumns updates the column list for the currently selected table (fallback).
func (e *SQLQueryEditor) SetColumns(columns []string) {
	e.columns = columns
}

// SetColumnsForTable caches columns for a specific schema.table and sets them
// as the fallback column list.
func (e *SQLQueryEditor) SetColumnsForTable(schema, table string, columns []string) {
	key := schema + "." + table
	e.columnCache[key] = columns
	e.columnCache[table] = columns
	e.columns = columns
}

// SetColumnFetcher provides a function to fetch columns on demand for tables
// referenced in the SQL editor that haven't been cached yet.
func (e *SQLQueryEditor) SetColumnFetcher(fn func(schema, table string) ([]string, error)) {
	e.columnFetcher = fn
}

// SetOnExecute sets the callback invoked when the user presses the execute key.
func (e *SQLQueryEditor) SetOnExecute(fn func(sql string)) {
	e.onExecute = fn
}

// SetQueryBarSource sets a callback that returns the current query bar text,
// used to pre-populate the editor when the user presses the load-query key.
func (e *SQLQueryEditor) SetQueryBarSource(fn func() string) {
	e.queryBarText = fn
}

// SetOnClose sets the callback invoked when the user closes the editor.
func (e *SQLQueryEditor) SetOnClose(fn func()) {
	e.onClose = fn
}

// SetOnExpand sets the callback invoked when the user toggles the editor size.
func (e *SQLQueryEditor) SetOnExpand(fn func()) {
	e.onExpand = fn
}

// Toggle flips the expanded state and returns the new required height in terminal rows.
func (e *SQLQueryEditor) Toggle() int {
	e.expanded = !e.expanded
	if e.expanded {
		return sqlEditorExpandedHeight
	}
	return sqlEditorDefaultHeight
}

// InputHandler intercepts execute/load/paste/close keys, passing everything
// else to the underlying TextArea.
func (e *SQLQueryEditor) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return e.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		k := e.App.GetKeys()

		execute := func() {
			if e.onExecute != nil {
				sql := strings.TrimRight(strings.TrimSpace(e.GetText()), ";")
				if sql != "" {
					e.onExecute(sql)
				}
			}
		}

		switch {
		case k.Contains(k.SQLQueryEditor.Execute, event.Name()):
			execute()
			return
		case k.Contains(k.SQLQueryEditor.LoadQuery, event.Name()):
			if e.queryBarText != nil {
				if q := e.queryBarText(); q != "" {
					e.SetText(q, true)
				}
			}
			return
		case k.Contains(k.Navigation.AutocompleteDown, event.Name()):
			if e.TextArea.IsAutocompleteVisible() {
				e.TextArea.InputHandler()(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone), setFocus)
				return
			}
		case k.Contains(k.Navigation.AutocompleteUp, event.Name()):
			if e.TextArea.IsAutocompleteVisible() {
				e.TextArea.InputHandler()(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone), setFocus)
				return
			}
		case k.Contains(k.Navigation.AutocompleteAccept, event.Name()):
			if e.TextArea.IsAutocompleteVisible() {
				e.TextArea.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), setFocus)
				return
			}
		case k.Contains(k.InputBar.Paste, event.Name()):
			_, pasteFunc := util.GetClipboard()
			if text := pasteFunc(); text != "" {
				cursorPos := len(e.GetTextBeforeCursor())
				e.Replace(cursorPos, cursorPos, text)
			}
			return
		case k.Contains(k.SQLQueryEditor.Expand, event.Name()):
			if e.onExpand != nil {
				e.onExpand()
			}
			return
		case k.Contains(k.SQLQueryEditor.Clear, event.Name()):
			e.SetText("", true)
			return
		case k.Contains(k.SQLQueryEditor.Close, event.Name()):
			if e.TextArea.IsAutocompleteVisible() {
				e.TextArea.InputHandler()(event, setFocus)
				return
			}
			if e.onClose != nil {
				e.onClose()
			}
			return
		}
		e.TextArea.InputHandler()(event, setFocus)
	})
}
