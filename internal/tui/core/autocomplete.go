package core

import (
	"regexp"
	"strings"

	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	sqlpkg "github.com/kopecmaciej/vi-sql/internal/sql"
	"github.com/kopecmaciej/vi-sql/internal/sql/completion"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

const AutocompleteMaxItems = 10

// QuoteCompletion based on symbol Kind quotes columns and table name parts
// when needed (e.g. camelCase in postgres or names with numbers like `2fa`).
func QuoteCompletion(sym completion.Symbol, q util.Quoter) string {
	quote := quoteIfNeeded
	if sym.Quoted {
		quote = func(name string, q util.Quoter) string { return q.Ident(name) }
	}
	switch sym.Kind {
	case completion.KindColumn:
		return quote(sym.Name, q)
	case completion.KindTable:
		parts := strings.Split(sym.Name, ".")
		for i, p := range parts {
			parts[i] = quote(p, q)
		}
		return strings.Join(parts, ".")
	default:
		return sym.Name
	}
}

func quoteIfNeeded(name string, q util.Quoter) string {
	if name == "" || !identNeedsQuoting(name, q) {
		return name
	}
	return q.Ident(name)
}

var (
	safeAnsi    = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)
	safeAnyCase = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
)

func identNeedsQuoting(name string, q util.Quoter) bool {
	if sqlpkg.IsKeyword(name) {
		return true
	}
	if q == util.ANSIQuoter {
		return !safeAnsi.MatchString(name)
	}
	return !safeAnyCase.MatchString(name)
}

// BuildAutocompleteItems converts completion symbols into display items: a
// colored icon prefix, the name padded for alignment, and (for columns) a
// right-aligned type label.
func BuildAutocompleteItems(symbols []completion.Symbol, styles *config.Styles) []tview.AutocompleteItem {
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

func buildAutocompleteDisplay(sym completion.Symbol, maxNameLen int, styles *config.Styles) string {
	icons := &styles.Icons
	var iconColor config.Style
	switch {
	case sym.IsPK:
		iconColor = styles.SQLEditor.NumberColor
	case sym.IsFK:
		iconColor = styles.SQLEditor.StringColor
	default:
		switch sym.Kind {
		case completion.KindColumn, completion.KindTable:
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
	switch {
	case sym.IsPK:
		glyph = icons.PrimaryKey
	case sym.IsFK:
		glyph = icons.ForeignKey
	default:
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
