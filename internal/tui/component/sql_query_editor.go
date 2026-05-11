package component

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	sqlpkg "github.com/kopecmaciej/vi-sql/internal/sql"
	"github.com/kopecmaciej/vi-sql/internal/sql/completion"
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

	vim              *vimHandler
	style            *config.SQLEditorStyle
	schemas          []database.Schema
	columnCache      map[string][]completion.Column // key: "schema.table" or "table"
	columnFetcher    func(schema, table string) ([]completion.Column, error)
	completionEngine *completion.Engine
	history          *modal.History
	onExecute        func(sql string)
	onFullscreen     func()
	onFocusDown      func()
	onOpenInEditor   func()
	onCancel         func()
	onModeChange     func(indicator string)
	// Yank highlight: active byte range [yankHLStart, yankHLEnd) and a generation
	// counter so the clearing goroutine doesn't overwrite a newer highlight.
	yankHLStart int
	yankHLEnd   int
	yankHLGen   uint64
	// tokenCache is shared between syntax highlighting and autocomplete
	tokenCache struct {
		text   string
		tokens []sqlpkg.Token
	}
}

func NewSQLQueryEditor(ownerID string) *SQLQueryEditor {
	e := &SQLQueryEditor{
		BaseElement:      core.NewBaseElement(),
		TextArea:         core.NewTextArea(),
		columnCache:      make(map[string][]completion.Column),
		completionEngine: completion.NewDefaultEngine(),
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
		e.TextArea.SetTitle(fmt.Sprintf(" SQL Editor [%s]Normal[-] ", s.KeywordColor))
		if e.onModeChange != nil {
			e.onModeChange(fmt.Sprintf("[%s]Normal[-]", s.KeywordColor))
		}
	case vimVisual:
		e.TextArea.SetTitle(fmt.Sprintf(" SQL Editor [%s]Visual[-] ", s.NumberColor))
		if e.onModeChange != nil {
			e.onModeChange(fmt.Sprintf("[%s]Visual[-]", s.NumberColor))
		}
	case vimVisualLine:
		e.TextArea.SetTitle(fmt.Sprintf(" SQL Editor [%s]V-Line[-] ", s.NumberColor))
		if e.onModeChange != nil {
			e.onModeChange(fmt.Sprintf("[%s]V-Line[-]", s.NumberColor))
		}
	default:
		e.TextArea.SetTitle(fmt.Sprintf(" SQL Editor [%s]Insert[-] ", s.OperatorColor))
		if e.onModeChange != nil {
			e.onModeChange(fmt.Sprintf("[%s]Insert[-]", s.OperatorColor))
		}
	}
}

// SetOnModeChange registers a callback invoked on every vim mode transition (Normal/Insert/...).
func (e *SQLQueryEditor) SetOnModeChange(f func(indicator string)) {
	e.onModeChange = f
	e.refreshTitle()
}

func (e *SQLQueryEditor) IsInsertMode() bool {
	return e.vim == nil || e.vim.mode == vimInsert
}

func (e *SQLQueryEditor) IsVisualMode() bool {
	return e.vim != nil && (e.vim.mode == vimVisual || e.vim.mode == vimVisualLine)
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

	e.TextArea.SetSelectedStyle(tcell.StyleDefault.
		Background(styles.Global.TextColor.Color()).
		Foreground(styles.Global.BackgroundColor.Color()))

	a := styles.Autocomplete
	acBackground := a.BackgroundColor.Color()
	acMain := tcell.StyleDefault.
		Background(acBackground).
		Foreground(a.TextColor.Color())
	acSelected := tcell.StyleDefault.
		Background(a.ActiveBackgroundColor.Color()).
		Foreground(a.ActiveTextColor.Color())
	e.TextArea.SetAutocompleteStyles(acBackground, acMain, acSelected)
	e.TextArea.SetAutocompleteBorderColor(a.BorderColor.Color())
	e.TextArea.SetAutocompleteMaxHeight(autocompleteMaxItems)
}

func (e *SQLQueryEditor) setHighlighting() {
	e.SetStyleFunc(func(byteOffset int) tcell.Style {
		text := e.GetText()
		if e.tokenCache.text != text {
			e.tokenCache.text = text
			e.tokenCache.tokens = sqlpkg.Tokenize(text)
		}
		base := core.SQLTokenStyle(e.tokenCache.tokens, byteOffset, e.style)
		hStyle := e.App.GetStyles().Global.MoreContrastBackgroundColor.Color()
		return yankOverlayStyle(base, hStyle, e.yankHLStart, e.yankHLEnd, byteOffset)
	})
}

// yankOverlayStyle returns base with its background replaced by bg when offset
// falls in [start, end). Returns base unchanged otherwise.
func yankOverlayStyle(base tcell.Style, bg tcell.Color, start, end, offset int) tcell.Style {
	if start < end && offset >= start && offset < end {
		return base.Background(bg)
	}
	return base
}

// BeginYankHighlight highlights [start, end) for 350 ms, then clears.
// The highlight is applied via the styleFunc so cursor position is untouched.
func (e *SQLQueryEditor) BeginYankHighlight(start, end int) {
	e.yankHLGen++
	e.yankHLStart = start
	e.yankHLEnd = end
	gen := e.yankHLGen
	go func() {
		time.Sleep(350 * time.Millisecond)
		e.App.QueueUpdateDraw(func() {
			if e.yankHLGen == gen {
				e.yankHLStart = 0
				e.yankHLEnd = 0
			}
		})
	}()
}

func (e *SQLQueryEditor) setAutocomplete() {
	var lastSymbols []completion.Symbol

	e.TextArea.SetAutocompleteFunc(func(text string, cursorBytePos int) []tview.AutocompleteItem {
		if cursorBytePos > 0 && strings.HasSuffix(strings.TrimSpace(text[:cursorBytePos]), ";") {
			return nil
		}
		if e.tokenCache.text != text {
			e.tokenCache.text = text
			e.tokenCache.tokens = sqlpkg.Tokenize(text)
		}
		symbols := e.completionEngine.SuggestTokens(e.tokenCache.tokens, text, cursorBytePos, completion.Context{
			Schemas:       e.schemas,
			ColumnFetcher: e.columnFetcher,
			ColumnCache:   e.columnCache,
		})
		lastSymbols = symbols
		return buildAutocompleteItems(symbols, e.App.GetStyles())
	})

	e.TextArea.SetAutocompletedFunc(func(text string, index int, source int) bool {
		if source == tview.AutocompletedNavigate {
			return false
		}
		if index < 0 || index >= len(lastSymbols) {
			return false
		}
		sym := lastSymbols[index]
		e.Replace(sym.Replace.Start, sym.Replace.End, sym.Name)
		return true
	})
}

// autocompleteMaxItems is the max visible rows in the autocomplete dropdown.
const autocompleteMaxItems = 8

// buildAutocompleteItems converts symbols into display items: icon prefix, padded
// name, and (for columns) a right-aligned type label.
func buildAutocompleteItems(symbols []completion.Symbol, styles *config.Styles) []tview.AutocompleteItem {
	maxLen := 0
	for _, sym := range symbols {
		if n := len(sym.Name); n > maxLen {
			maxLen = n
		}
	}
	items := make([]tview.AutocompleteItem, len(symbols))
	for i, sym := range symbols {
		items[i] = tview.AutocompleteItem{Main: buildAutocompleteDisplay(sym, maxLen, styles)}
	}
	return items
}

// buildAutocompleteDisplay formats one item: colored icon, name padded to
// maxNameLen, then the column type label (columns only) in the autocomplete
// secondary color.
func buildAutocompleteDisplay(sym completion.Symbol, maxNameLen int, styles *config.Styles) string {
	icons := &styles.Icons
	var iconColor config.Style
	if sym.IsPK {
		iconColor = styles.Global.ContrastBackgroundColor
	} else {
		switch sym.Kind {
		case completion.KindColumn:
			iconColor = styles.Others.LeafIconColor
		case completion.KindTable:
			iconColor = styles.Others.LeafIconColor
		case completion.KindSchema:
			iconColor = styles.Global.SecondaryTextColor
		case completion.KindKeyword, completion.KindDDLObject:
			iconColor = styles.SQLEditor.KeywordColor
		case completion.KindFunction:
			iconColor = styles.SQLEditor.StringColor
		case completion.KindCTE, completion.KindAlias:
			iconColor = styles.Global.DimColor
		default:
			iconColor = styles.Global.DimColor
		}
	}

	var glyph config.Style
	if sym.IsPK {
		glyph = icons.PrimaryKey
	} else {
		switch sym.Kind {
		case completion.KindColumn:
			glyph = config.Style(icons.TypeSymbol(sym.TypeHint))
		case completion.KindTable:
			glyph = icons.Leaf
		case completion.KindSchema:
			glyph = icons.ClosedNode
		case completion.KindCTE:
			glyph = icons.CompletionCTE
		case completion.KindAlias:
			glyph = icons.CompletionAlias
		case completion.KindFunction:
			glyph = icons.CompletionFunction
		case completion.KindKeyword, completion.KindDDLObject:
			glyph = icons.CompletionKeyword
		default:
			glyph = icons.TypeDefault
		}
	}

	icon := icons.IconWithColor(glyph, iconColor)

	label := ""
	if sym.Kind == completion.KindColumn && sym.TypeHint != "" {
		label = sym.TypeHint
	}
	if label == "" {
		return icon + sym.Name
	}
	labelColor := styles.Autocomplete.SecondaryTextColor.String()
	pad := maxNameLen - len(sym.Name) + 2
	return icon + sym.Name + strings.Repeat(" ", pad) + "[" + labelColor + "]" + label + "[-]"
}

func (e *SQLQueryEditor) handleEvents() {
	go e.HandleEvents(e.GetIdentifier(), func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.FocusChanged:
			if e.vim != nil {
				if id, ok := event.Message.Data.(tview.Identifier); ok && id != e.GetIdentifier() {
					e.vim.transitionTo(vimNormal)
				}
			}
		case manager.StyleChanged:
			e.style = &e.App.GetStyles().SQLEditor
			e.setStyle()
			e.setHighlighting()
		case manager.ConfigChanged:
			vimMode := e.App.GetConfig().UI.VimMode
			if vimMode && e.vim == nil {
				e.vim = newVimHandler(e)
			} else if !vimMode && e.vim != nil {
				e.vim = nil
			}
			go e.App.QueueUpdateDraw(e.refreshTitle)
		}
	})
}

func (e *SQLQueryEditor) SetSchemas(schemas []database.Schema) {
	e.schemas = schemas
}

// SetColumnsForTable pre-populates the column cache for a specific table so
// that autocomplete works without a network round-trip for that table.
func (e *SQLQueryEditor) SetColumnsForTable(schema, table string, columns []completion.Column) {
	key := schema + "." + table
	e.columnCache[key] = columns
	e.columnCache[table] = columns
}

// SetColumnFetcher provides a function to fetch columns on demand for tables
// referenced in the SQL editor that haven't been cached yet.
func (e *SQLQueryEditor) SetColumnFetcher(fn func(schema, table string) ([]completion.Column, error)) {
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
		case event.Key() == tcell.KeyRune && event.Rune() == '(' && e.IsInsertMode():
			cursorPos := len(e.GetTextBeforeCursor())
			e.Replace(cursorPos, cursorPos, "()")
			e.TextArea.InputHandler()(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone), setFocus)
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
