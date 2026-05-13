package core

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	sql "github.com/kopecmaciej/vi-sql/internal/sql"
)

// SQLTokenStyle returns a tcell.Style for the token that contains byteOffset.
// It performs a linear scan through tokens (they are sorted by Start).
func SQLTokenStyle(tokens []sql.Token, byteOffset int, s *config.SQLEditorStyle) tcell.Style {
	for _, tok := range tokens {
		if tok.Start <= byteOffset && byteOffset < tok.End {
			switch tok.Type {
			case sql.TokenKeyword:
				return tcell.StyleDefault.Foreground(s.KeywordColor.Color())
			case sql.TokenString:
				return tcell.StyleDefault.Foreground(s.StringColor.Color())
			case sql.TokenNumber:
				return tcell.StyleDefault.Foreground(s.NumberColor.Color())
			case sql.TokenComment:
				return tcell.StyleDefault.Foreground(s.CommentColor.Color())
			case sql.TokenOperator, sql.TokenTypecast:
				return tcell.StyleDefault.Foreground(s.OperatorColor.Color())
			default:
				return tcell.StyleDefault.Foreground(s.IdentifierColor.Color())
			}
		}
	}
	return tcell.StyleDefault.Foreground(s.IdentifierColor.Color())
}

// ColorizeSQLText converts a SQL string into a tview dynamic-color string
// using the app's SQL editor color scheme. Use for read-only text views.
func ColorizeSQLText(sqlText string, style *config.SQLEditorStyle) string {
	tokens := sql.Tokenize(sqlText)
	var sb strings.Builder
	for _, tok := range tokens {
		escaped := tview.Escape(tok.Value)
		var color string
		switch tok.Type {
		case sql.TokenKeyword:
			color = style.KeywordColor.String()
		case sql.TokenString:
			color = style.StringColor.String()
		case sql.TokenNumber:
			color = style.NumberColor.String()
		case sql.TokenComment:
			color = style.CommentColor.String()
		case sql.TokenOperator, sql.TokenTypecast:
			color = style.OperatorColor.String()
		default:
			color = style.IdentifierColor.String()
		}
		fmt.Fprintf(&sb, "[%s]%s[-]", color, escaped)
	}
	return sb.String()
}
