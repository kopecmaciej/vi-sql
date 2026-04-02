package component

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
)

const SQLQueryEditorId = "SQLQueryEditor"

// SQLQueryEditor is an in-TUI multi-line SQL editor backed by a tview.TextArea.
// It supports syntax highlighting via SetStyleFunc and context-aware autocomplete.
type SQLQueryEditor struct {
	*core.BaseElement
	*tview.TextArea

	style     *config.SQLEditorStyle
	schemas   []database.SchemaWithTables
	columns   []string
	onExecute func(sql string)
	loadQuery func() string
}

func NewSQLQueryEditor() *SQLQueryEditor {
	e := &SQLQueryEditor{
		BaseElement: core.NewBaseElement(),
		TextArea:    tview.NewTextArea(),
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
	e.TextArea.SetTitle(" SQL Editor  F5/Ctrl+Enter: execute  Ctrl+L: load query  Esc: close ")
	e.TextArea.SetTitleAlign(tview.AlignLeft)
}

func (e *SQLQueryEditor) setHighlighting() {
	style := e.style
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
		return sqlTokenStyle(c.tokens, byteOffset, style)
	})
}

func (e *SQLQueryEditor) setAutocomplete() {
	e.TextArea.SetAutocompleteFunc(func(text string, cursorBytePos int) []tview.AutocompleteItem {
		if cursorBytePos > 0 && strings.HasSuffix(strings.TrimSpace(text[:cursorBytePos]), ";") {
			return nil
		}
		entries := database.BuildSQLAutocomplete(text, cursorBytePos, e.schemas, e.columns)
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
		after := e.GetText()[len(before):]
		ctx := database.DetectContext(database.Tokenize(e.GetText()), len(before))
		trimmed := strings.TrimSuffix(before, ctx.PartialWord)
		e.SetText(trimmed+text+after, false)
		e.Select(len(trimmed+text), len(trimmed+text))
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

// SetColumns updates the column list for SELECT/WHERE autocomplete.
func (e *SQLQueryEditor) SetColumns(columns []string) {
	e.columns = columns
}

// SetOnExecute sets the callback invoked when the user presses F5/Ctrl+Enter.
func (e *SQLQueryEditor) SetOnExecute(fn func(sql string)) {
	e.onExecute = fn
}

// SetLoadQuery sets a callback that returns the current query bar text.
// Bound to Ctrl+L so the user can pull it into the editor on demand.
func (e *SQLQueryEditor) SetLoadQuery(fn func() string) {
	e.loadQuery = fn
}

// InputHandler intercepts Ctrl+Enter (execute) and Escape (close), passing
// everything else to the underlying TextArea.
func (e *SQLQueryEditor) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return e.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		execute := func() {
			if e.onExecute != nil {
				sql := strings.TrimRight(strings.TrimSpace(e.GetText()), ";")
				if sql != "" {
					e.onExecute(sql)
				}
			}
		}
		switch event.Key() {
		case tcell.KeyF5:
			execute()
			return
		case tcell.KeyCtrlL:
			if e.loadQuery != nil {
				if q := e.loadQuery(); q != "" {
					e.SetText(q, true)
				}
			}
			return
		case tcell.KeyEnter:
			if event.Modifiers()&tcell.ModCtrl != 0 {
				execute()
				return
			}
		case tcell.KeyEscape:
			if e.TextArea.IsAutocompleteVisible() {
				e.TextArea.InputHandler()(event, setFocus)
				return
			}
			e.App.Pages.RemovePage(SQLQueryEditorId)
			return
		}
		e.TextArea.InputHandler()(event, setFocus)
	})
}

// Open adds the editor as a centered overlay on app.Pages.
func (e *SQLQueryEditor) Open(initial string) {
	e.SetText(initial, initial == "")
	e.App.Pages.AddPage(SQLQueryEditorId, e, true, true)
	e.App.SetFocusInternal(e)
}
