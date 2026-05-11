package completion

import (
	"strings"

	sql "github.com/kopecmaciej/vi-sql/internal/sql"
)

// TableRef holds a schema-qualified table reference parsed from a FROM/JOIN clause.
type TableRef struct {
	Schema string
	Table  string
	Alias  string
}

// QueryScope holds the in-scope tables, CTEs, and alias map extracted from a SQL text.
type QueryScope struct {
	TableRefs []TableRef
	CTENames  []string
	AliasMap  map[string]string // lowercase alias → real table name
}

// tableTerminators are keywords that end a table reference and cannot be an alias.
var tableTerminators = map[string]bool{
	"WHERE": true, "JOIN": true, "LEFT": true, "RIGHT": true, "INNER": true,
	"FULL": true, "CROSS": true, "NATURAL": true, "ON": true, "USING": true,
	"GROUP": true, "ORDER": true, "HAVING": true, "LIMIT": true, "OFFSET": true,
	"UNION": true, "INTERSECT": true, "EXCEPT": true, "SET": true,
	"RETURNING": true, "INTO": true, "VALUES": true, "SELECT": true, "FROM": true,
	"WITH": true, "FETCH": true, "FOR": true,
}

// BuildScope extracts all in-scope table references, CTE names, and aliases
// from a pre-tokenized SQL statement. It stops at `;` to avoid leaking context
// from a preceding statement.
func BuildScope(tokens []sql.Token) *QueryScope {
	tokens = stopAtSemicolon(tokens)

	scope := &QueryScope{
		AliasMap: make(map[string]string),
	}
	scope.TableRefs = extractTableRefs(tokens)
	scope.CTENames = extractCTENames(tokens)

	for _, ref := range scope.TableRefs {
		if ref.Alias != "" {
			scope.AliasMap[strings.ToLower(ref.Alias)] = ref.Table
		}
	}
	return scope
}

// ResolveTable returns the real table name for an alias, or name itself if not an alias.
func (s *QueryScope) ResolveTable(name string) string {
	if real, ok := s.AliasMap[strings.ToLower(name)]; ok {
		return real
	}
	return name
}

// HasTable reports whether tableName (or its alias) is in scope.
func (s *QueryScope) HasTable(tableName string) bool {
	lower := strings.ToLower(tableName)
	for _, ref := range s.TableRefs {
		if strings.ToLower(ref.Table) == lower || strings.ToLower(ref.Alias) == lower {
			return true
		}
	}
	return false
}

// stopAtSemicolon trims tokens to the last statement (after the final `;`).
func stopAtSemicolon(tokens []sql.Token) []sql.Token {
	last := 0
	for i, tok := range tokens {
		if tok.Type == sql.TokenPunctuation && tok.Value == ";" {
			last = i + 1
		}
	}
	return tokens[last:]
}

func skipWS(tokens []sql.Token, start int) int {
	for start < len(tokens) && tokens[start].Type == sql.TokenWhitespace {
		start++
	}
	return start
}

func extractTableRefs(tokens []sql.Token) []TableRef {
	var refs []TableRef
	fromKeywords := map[string]bool{"FROM": true, "JOIN": true, "INTO": true}

	i := 0
	for i < len(tokens) {
		tok := tokens[i]
		if tok.Type == sql.TokenKeyword && fromKeywords[strings.ToUpper(tok.Value)] {
			j := skipWS(tokens, i+1)
			if j >= len(tokens) {
				break
			}
			first := tokens[j]
			// Skip subquery-in-FROM: FROM (SELECT …) sub — skip the entire paren block
			// so inner FROM/JOIN clauses don't pollute the outer scope.
			if first.Type == sql.TokenPunctuation && first.Value == "(" {
				depth := 1
				j++
				for j < len(tokens) && depth > 0 {
					if tokens[j].Type == sql.TokenPunctuation {
						switch tokens[j].Value {
						case "(":
							depth++
						case ")":
							depth--
						}
					}
					j++
				}
				i = j
				continue
			}
			if first.Type != sql.TokenIdentifier && first.Type != sql.TokenQuotedIdentifier {
				i++
				continue
			}

			ref := TableRef{}
			// check for schema.table
			if j+2 < len(tokens) && tokens[j+1].Type == sql.TokenDot {
				second := tokens[j+2]
				if second.Type == sql.TokenIdentifier || second.Type == sql.TokenQuotedIdentifier {
					ref.Schema = first.Value
					ref.Table = second.Value
					j = j + 3
				} else {
					ref.Table = first.Value
					j = j + 1
				}
			} else {
				ref.Table = first.Value
				j = j + 1
			}

			// Look for optional alias: [AS] identifier
			k := skipWS(tokens, j)
			if k < len(tokens) {
				next := tokens[k]
				if next.Type == sql.TokenKeyword && strings.ToUpper(next.Value) == "AS" {
					k = skipWS(tokens, k+1)
					if k < len(tokens) && (tokens[k].Type == sql.TokenIdentifier || tokens[k].Type == sql.TokenQuotedIdentifier) {
						ref.Alias = tokens[k].Value
					}
				} else if next.Type == sql.TokenIdentifier && !tableTerminators[strings.ToUpper(next.Value)] {
					ref.Alias = next.Value
				}
			}

			refs = append(refs, ref)
			i = j
			continue
		}
		i++
	}
	return refs
}

func extractCTENames(tokens []sql.Token) []string {
	var names []string
	for i, tok := range tokens {
		if tok.Type != sql.TokenKeyword || strings.ToUpper(tok.Value) != "WITH" {
			continue
		}
		j := skipWS(tokens, i+1)
		// skip RECURSIVE if present (it's not in the keyword set, so check value only)
		if j < len(tokens) && strings.ToUpper(tokens[j].Value) == "RECURSIVE" {
			j = skipWS(tokens, j+1)
		}
		for j < len(tokens) {
			if tokens[j].Type != sql.TokenIdentifier {
				break
			}
			cteName := tokens[j].Value
			j = skipWS(tokens, j+1)

			// skip optional column list: cte(a, b) AS (…)
			if j < len(tokens) && tokens[j].Type == sql.TokenPunctuation && tokens[j].Value == "(" {
				depth := 1
				j++
				for j < len(tokens) && depth > 0 {
					if tokens[j].Type == sql.TokenPunctuation {
						switch tokens[j].Value {
						case "(":
							depth++
						case ")":
							depth--
						}
					}
					j++
				}
				j = skipWS(tokens, j)
			}

			if j >= len(tokens) || strings.ToUpper(tokens[j].Value) != "AS" {
				break
			}
			names = append(names, cteName)
			j = skipWS(tokens, j+1)
			if j >= len(tokens) || tokens[j].Value != "(" {
				break
			}
			// skip the CTE body tracking parenthesis depth
			depth := 0
			for j < len(tokens) {
				if tokens[j].Type == sql.TokenPunctuation {
					switch tokens[j].Value {
					case "(":
						depth++
					case ")":
						depth--
						if depth == 0 {
							j++
							goto afterBody
						}
					}
				}
				j++
			}
		afterBody:
			j = skipWS(tokens, j)
			// comma → another CTE follows
			if j < len(tokens) && tokens[j].Type == sql.TokenPunctuation && tokens[j].Value == "," {
				j = skipWS(tokens, j+1)
				continue
			}
			break
		}
	}
	return names
}
