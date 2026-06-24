package core

import (
	"strings"

	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	sqlpkg "github.com/kopecmaciej/vi-sql/internal/sql"
	"github.com/kopecmaciej/vi-sql/internal/sql/completion"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

// AutocompleteMaxItems is the max visible rows in any autocomplete dropdown.
const AutocompleteMaxItems = 10

// QuoteCompletion returns the text to insert for an accepted completion symbol.
// Columns and each part of a (possibly schema-qualified) table name are quoted
// only when the identifier requires it for the dialect; ordinary names are
// inserted bare so they stay editable and keep re-triggering autocomplete.
func QuoteCompletion(sym completion.Symbol, q util.Quoter) string {
	switch sym.Kind {
	case completion.KindColumn:
		return quoteIfNeeded(sym.Name, q)
	case completion.KindTable:
		parts := strings.Split(sym.Name, ".")
		for i, p := range parts {
			parts[i] = quoteIfNeeded(p, q)
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

// identNeedsQuoting reports whether name must be quoted to be used verbatim:
// it is a reserved word, starts with a digit, contains a special character, or
// (for case-folding dialects via ANSIQuoter) is not all lowercase.
func identNeedsQuoting(name string, q util.Quoter) bool {
	if sqlpkg.IsKeyword(name) {
		return true
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z', c == '_':
		case c >= '0' && c <= '9':
			if i == 0 {
				return true
			}
		case c >= 'A' && c <= 'Z':
			if q == util.ANSIQuoter {
				return true
			}
		default:
			return true
		}
	}
	return false
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
