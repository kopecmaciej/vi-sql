package completion

import (
	"sort"
	"strings"
)

// matchesPartial reports whether lower (already lowercased) matches partial
// by prefix or substring. Empty partial matches everything.
func matchesPartial(lower, partial string) bool {
	if partial == "" {
		return true
	}
	return strings.HasPrefix(lower, partial) || strings.Contains(lower, partial)
}

// Rank sorts symbols by effective priority (descending).
// Priority = symbol.Priority + prefixBonus + scopeBonus.
//   - prefixBonus: +20 for exact prefix match, +10 for substring match
//   - scopeBonus:  +10 when the symbol's table is in scope (already applied by column provider)
func Rank(symbols []Symbol, partial string, scope *QueryScope) []Symbol {
	type ranked struct {
		sym       Symbol
		effective int
	}

	out := make([]ranked, 0, len(symbols))

	for _, s := range symbols {
		ep := s.Priority
		lower := strings.ToLower(s.Name)
		if partial != "" {
			if strings.HasPrefix(lower, partial) {
				ep += 20
			} else if strings.Contains(lower, partial) {
				ep += 10
			}
		}

		if scope != nil && scope.HasTable(s.Qualifier) {
			ep += 10
		}

		out = append(out, ranked{s, ep})
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].effective > out[j].effective
	})

	result := make([]Symbol, len(out))
	for i, r := range out {
		result[i] = r.sym
	}
	return result
}
