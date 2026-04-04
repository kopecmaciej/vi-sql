package core

import (
	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
)

// SQLTokenStyle returns a tcell.Style for the token that contains byteOffset.
// It performs a linear scan through tokens (they are sorted by Start).
func SQLTokenStyle(tokens []database.Token, byteOffset int, s *config.SQLEditorStyle) tcell.Style {
	for _, tok := range tokens {
		if tok.Start <= byteOffset && byteOffset < tok.End {
			switch tok.Type {
			case database.TokenKeyword:
				return tcell.StyleDefault.Foreground(s.KeywordColor.Color())
			case database.TokenString:
				return tcell.StyleDefault.Foreground(s.StringColor.Color())
			case database.TokenNumber:
				return tcell.StyleDefault.Foreground(s.NumberColor.Color())
			case database.TokenComment:
				return tcell.StyleDefault.Foreground(s.CommentColor.Color())
			case database.TokenOperator, database.TokenTypecast:
				return tcell.StyleDefault.Foreground(s.OperatorColor.Color())
			default:
				return tcell.StyleDefault.Foreground(s.IdentifierColor.Color())
			}
		}
	}
	return tcell.StyleDefault.Foreground(s.IdentifierColor.Color())
}
