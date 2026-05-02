package component

import (
	"fmt"
	"slices"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/tui/modal"
	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/rs/zerolog/log"
)

const SQLQueryEditorId = "SQLQueryEditor"

// SQLQueryEditor is an in-TUI multi-line SQL editor backed by a tview.TextArea.
// It supports syntax highlighting via SetStyleFunc and context-aware autocomplete.
type SQLQueryEditor struct {
	*core.BaseElement
	*core.TextArea

	vim            *vimHandler
	style          *config.SQLEditorStyle
	schemas        []database.Schema
	columns        []string
	columnCache    map[string][]string // key: "schema.table" or "table"
	columnFetcher  func(schema, table string) ([]string, error)
	history        *modal.History
	onExecute      func(sql string)
	onFullscreen   func()
	onFocusDown    func()
	onOpenInEditor func()
	onCancel       func()
	onModeChange   func(indicator string)
}

func NewSQLQueryEditor(ownerID string) *SQLQueryEditor {
	e := &SQLQueryEditor{
		BaseElement: core.NewBaseElement(),
		TextArea:    core.NewTextArea(),
		columnCache: make(map[string][]string),
	}
	e.SetIdentifier(tview.Identifier(ownerID + EditorSuffix))
	e.SetAfterInitFunc(e.init)
	return e
}

func (e *SQLQueryEditor) init() error {
	e.style = &e.App.GetStyles().SQLEditor
	if e.App.GetConfig().UI.VimMode {
		e.vim = newVimHandler(e)
	}
	e.setStyle()
	e.setAutocomplete()
	e.setHighlighting()
	e.initHistory()
	e.handleEvents()
	return nil
}

func (e *SQLQueryEditor) initHistory() {
	e.history = modal.NewHistoryModal()
	if err := e.history.Init(e.App); err != nil {
		log.Error().Err(err).Msg("Failed to initialize history modal")
		e.history = nil
		return
	}
	e.history.SetOnAccept(func(query string) {
		e.SetText(query, true)
	})
}

// refreshTitle updates the editor border title to reflect the current vim mode.
// Called from setStyle() and vimHandler on every mode transition.
// Note: tview treats [word] as a style tag, so brackets are escaped with [[] .
func (e *SQLQueryEditor) refreshTitle() {
	if e.vim == nil {
		e.TextArea.SetTitle(" SQL Editor ")
		if e.onModeChange != nil {
			e.onModeChange("")
		}
		return
	}
	s := e.style
	switch e.vim.mode {
	case vimNormal:
		e.TextArea.SetTitle(fmt.Sprintf(" SQL Editor [%s]NORMAL[-] ", s.KeywordColor))
		if e.onModeChange != nil {
			e.onModeChange(fmt.Sprintf("[%s]NORMAL[-]", s.KeywordColor))
		}
	case vimVisual:
		e.TextArea.SetTitle(fmt.Sprintf(" SQL Editor [%s]VISUAL[-] ", s.NumberColor))
		if e.onModeChange != nil {
			e.onModeChange(fmt.Sprintf("[%s]VISUAL[-]", s.NumberColor))
		}
	default:
		e.TextArea.SetTitle(fmt.Sprintf(" SQL Editor [%s]INSERT[-] ", s.OperatorColor))
		if e.onModeChange != nil {
			e.onModeChange(fmt.Sprintf("[%s]INSERT[-]", s.OperatorColor))
		}
	}
}

// SetOnModeChange registers a callback invoked on every vim mode transition (NORMAL/INSERT/...).
func (e *SQLQueryEditor) SetOnModeChange(f func(indicator string)) {
	e.onModeChange = f
	e.refreshTitle()
}

func (e *SQLQueryEditor) IsInsertMode() bool {
	return e.vim == nil || e.vim.mode == vimInsert
}

func (e *SQLQueryEditor) setStyle() {
	styles := e.App.GetStyles()
	e.TextArea.SetStyle(styles)
	e.TextArea.SetBorder(true)
	e.refreshTitle()
	e.TextArea.SetTitleAlign(tview.AlignCenter)
	e.TextArea.SetBorderPadding(0, 0, 1, 1)
	e.TextArea.SetLineNumbers(true)
	e.TextArea.SetLineNumberStyle(tcell.StyleDefault.
		Foreground(styles.Global.BorderColor.Color()).
		Background(styles.Global.BackgroundColor.Color()))

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

// buildAliasMap extracts alias→table mappings from FROM/JOIN clauses in text.
func (e *SQLQueryEditor) buildAliasMap(text string) map[string]string {
	refs := database.ExtractFromTableRefs(text)
	aliases := make(map[string]string, len(refs))
	for _, ref := range refs {
		if ref.Alias != "" {
			aliases[strings.ToLower(ref.Alias)] = ref.Table
		}
	}
	return aliases
}

// resolveColumnsForQuery merges columns from all tables referenced in the FROM/JOIN
// clauses of text, resolving aliases where needed.
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

// resolveColumnsForTable returns columns for a specific table or alias (for CtxAfterDot).
func (e *SQLQueryEditor) resolveColumnsForTable(tableName string, aliases map[string]string) []string {
	// Resolve alias to real table name first
	if real, ok := aliases[strings.ToLower(tableName)]; ok {
		tableName = real
	}
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

		tokens := database.Tokenize(text)
		ctx := database.DetectContext(tokens, cursorBytePos)
		aliases := e.buildAliasMap(text)
		variables := database.ExtractCTENames(text)

		var cols []string
		switch ctx.Type {
		case database.CtxAfterDot:
			isSchema := slices.ContainsFunc(e.schemas, func(s database.Schema) bool {
				return strings.EqualFold(ctx.TableName, s.Schema)
			})
			if !isSchema && ctx.TableName != "" {
				cols = e.resolveColumnsForTable(ctx.TableName, aliases)
			} else {
				cols = e.columns
			}
		case database.CtxAfterSelect, database.CtxAfterWhere, database.CtxAfterOn,
			database.CtxAfterOrderBy, database.CtxAfterGroupBy, database.CtxAfterSet:
			cols = e.resolveColumnsForQuery(text)
		default:
			cols = e.columns
		}

		entries := database.BuildSQLAutocomplete(text, cursorBytePos, e.schemas, cols, variables)
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
	go e.HandleEvents(e.GetIdentifier(), func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.FocusChanged:
			if e.vim != nil {
				if id, ok := event.Message.Data.(tview.Identifier); ok && id != e.GetIdentifier() {
					e.vim.reset()
				}
			}
		case manager.StyleChanged:
			e.style = &e.App.GetStyles().SQLEditor
			e.setStyle()
			e.setHighlighting()
		}
	})
}

func (e *SQLQueryEditor) SetSchemas(schemas []database.Schema) {
	e.schemas = schemas
}

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

func (e *SQLQueryEditor) SetOnExecute(fn func(sql string)) {
	e.onExecute = fn
}

func (e *SQLQueryEditor) Execute() {
	if e.onExecute == nil {
		return
	}
	sql := strings.TrimRight(strings.TrimSpace(e.GetText()), ";")
	if sql != "" {
		e.onExecute(sql)
	}
}

func (e *SQLQueryEditor) SetOnFullscreen(fn func()) {
	e.onFullscreen = fn
}

// SetOnFocusDown sets the callback invoked when the user presses the focus-down key,
// used to move focus from the editor to the results table below.
func (e *SQLQueryEditor) SetOnFocusDown(fn func()) {
	e.onFocusDown = fn
}

// SetOnOpenInEditor sets the callback invoked when the user presses the TermEditor key,
// used to pop the current buffer out to $EDITOR and return the result.
func (e *SQLQueryEditor) SetOnOpenInEditor(fn func()) {
	e.onOpenInEditor = fn
}

func (e *SQLQueryEditor) SetOnCancel(fn func()) {
	e.onCancel = fn
}

// InputHandler intercepts execute/load/paste keys, passing everything
// else to the underlying TextArea.
func (e *SQLQueryEditor) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return e.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		// Vim mode intercept — runs before all other key handling.
		// Non-rune keys (Ctrl+Enter, arrows, etc.) fall through unchanged,
		// so execute, paste, history, and autocomplete bindings are unaffected.
		if e.vim != nil && e.vim.Handle(event, setFocus) {
			return
		}

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
		case k.Match(k.Common.Confirm, event):
			execute()
			return
		case k.Match(k.Navigation.FocusDown, event):
			if !e.TextArea.IsAutocompleteVisible() {
				if e.onFocusDown != nil {
					e.onFocusDown()
				}
				return
			}
		case k.Match(k.Navigation.AutocompleteDown, event):
			if e.TextArea.IsAutocompleteVisible() {
				e.TextArea.InputHandler()(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone), setFocus)
				return
			}
		case k.Match(k.Navigation.AutocompleteUp, event):
			if e.TextArea.IsAutocompleteVisible() {
				e.TextArea.InputHandler()(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone), setFocus)
				return
			}
		case k.Match(k.Navigation.AutocompleteAccept, event):
			if e.TextArea.IsAutocompleteVisible() {
				e.TextArea.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), setFocus)
				return
			}
		case k.Match(k.Common.Paste, event):
			if text := util.Paste(); text != "" {
				cursorPos := len(e.GetTextBeforeCursor())
				e.Replace(cursorPos, cursorPos, text)
			}
			return
		case k.Match(k.SQLQueryEditor.Fullscreen, event):
			if e.onFullscreen != nil {
				e.onFullscreen()
			}
			return
		case k.Match(k.Common.Clear, event):
			e.SetText("", true)
			return
		case k.Match(k.SQLQueryEditor.OpenHistory, event):
			if e.history != nil {
				e.history.Render()
			}
			return
		case k.Match(k.SQLQueryEditor.TermEditor, event):
			if e.onOpenInEditor != nil {
				e.onOpenInEditor()
			}
			return
		case event.Key() == tcell.KeyEscape && !e.TextArea.IsAutocompleteVisible() && e.onCancel != nil:
			e.onCancel()
			return
		}
		e.TextArea.InputHandler()(event, setFocus)
	})
}

// SaveQueryToHistory persists sql to the history file after a successful execution.
// No-op if history is not initialized.
func (e *SQLQueryEditor) SaveQueryToHistory(sql string) {
	if e.history != nil {
		if err := e.history.SaveToHistory(sql); err != nil {
			log.Error().Err(err).Msg("Failed to save to history")
		}
	}
}

// OpenHistory renders the SQL history modal.
func (e *SQLQueryEditor) OpenHistory() {
	if e.history != nil {
		e.history.Render()
	}
}

// OpenHistoryWithCallback renders the SQL history modal and temporarily
// overrides the onAccept callback. The original callback is restored on close.
func (e *SQLQueryEditor) OpenHistoryWithCallback(onAccept func(query string)) {
	if e.history == nil {
		return
	}
	original := e.history.GetOnAccept()
	e.history.SetOnAccept(onAccept)
	e.history.SetOnClose(func() {
		e.history.SetOnAccept(original)
	})
	e.history.Render()
}
