package core

import (
	"testing"

	"github.com/kopecmaciej/vi-sql/internal/sql/completion"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

func TestQuoteCompletion(t *testing.T) {
	tests := []struct {
		name   string
		sym    completion.Symbol
		quoter util.Quoter
		want   string
	}{
		{"plain column stays bare", completion.Symbol{Kind: completion.KindColumn, Name: "name"}, util.ANSIQuoter, "name"},
		{"reserved word column quoted", completion.Symbol{Kind: completion.KindColumn, Name: "order"}, util.ANSIQuoter, `"order"`},
		{"mixed case column quoted (ansi)", completion.Symbol{Kind: completion.KindColumn, Name: "createdAt"}, util.ANSIQuoter, `"createdAt"`},
		{"mixed case column bare (backtick)", completion.Symbol{Kind: completion.KindColumn, Name: "createdAt"}, util.BacktickQuoter, "createdAt"},
		{"special char column quoted", completion.Symbol{Kind: completion.KindColumn, Name: "total $"}, util.ANSIQuoter, `"total $"`},
		{"reserved word backtick", completion.Symbol{Kind: completion.KindColumn, Name: "order"}, util.BacktickQuoter, "`order`"},
		{"plain qualified table stays bare", completion.Symbol{Kind: completion.KindTable, Name: "public.users"}, util.ANSIQuoter, "public.users"},
		{"mixed case qualified table quoted (ansi)", completion.Symbol{Kind: completion.KindTable, Name: "public.Users"}, util.ANSIQuoter, `public."Users"`},
		{"keyword symbol inserted verbatim", completion.Symbol{Kind: completion.KindKeyword, Name: "SELECT"}, util.ANSIQuoter, "SELECT"},
		{"quoted flag forces ansi quotes", completion.Symbol{Kind: completion.KindColumn, Name: "category", Quoted: true}, util.ANSIQuoter, `"category"`},
		{"quoted flag forces backtick quotes", completion.Symbol{Kind: completion.KindColumn, Name: "category", Quoted: true}, util.BacktickQuoter, "`category`"},
		{"quoted flag forces bracket quotes", completion.Symbol{Kind: completion.KindColumn, Name: "category", Quoted: true}, util.BracketQuoter, "[category]"},
		{"quoted flag quotes each table part", completion.Symbol{Kind: completion.KindTable, Name: "public.users", Quoted: true}, util.ANSIQuoter, `"public"."users"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := QuoteCompletion(tt.sym, tt.quoter); got != tt.want {
				t.Errorf("QuoteCompletion() = %q, want %q", got, tt.want)
			}
		})
	}
}
