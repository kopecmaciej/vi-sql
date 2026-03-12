package postgres

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kopecmaciej/vi-sql/internal/database"
)

// convertValue converts pgx/pgtype-specific types to plain Go types so that
// the driver-agnostic database package can stringify them correctly.
func convertValue(v any) any {
	switch val := v.(type) {
	case pgtype.Numeric:
		if !val.Valid {
			return nil
		}
		if val.NaN {
			return "NaN"
		}
		if val.InfinityModifier == pgtype.Infinity {
			return "Infinity"
		}
		if val.InfinityModifier == pgtype.NegativeInfinity {
			return "-Infinity"
		}
		if val.Int == nil {
			return "0"
		}
		if val.Exp >= 0 {
			result := new(big.Int).Mul(val.Int, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(val.Exp)), nil))
			return result.String()
		}
		// Exp < 0: format with a decimal point
		absExp := int(-val.Exp)
		s := val.Int.String()
		negative := len(s) > 0 && s[0] == '-'
		if negative {
			s = s[1:]
		}
		for len(s) <= absExp {
			s = "0" + s
		}
		dotPos := len(s) - absExp
		result := s[:dotPos] + "." + s[dotPos:]
		if negative {
			return "-" + result
		}
		return result
	case pgtype.UUID:
		if !val.Valid {
			return nil
		}
		return val.Bytes // [16]byte — StringifyValue formats this as a UUID string
	case pgtype.Range[any]:
		if !val.Valid {
			return nil
		}
		if val.LowerType == pgtype.Empty {
			return "empty"
		}
		var b strings.Builder
		if val.LowerType == pgtype.Inclusive {
			b.WriteByte('[')
		} else {
			b.WriteByte('(')
		}
		if val.LowerType != pgtype.Unbounded {
			b.WriteString(formatRangeBound(val.Lower))
		}
		b.WriteByte(',')
		if val.UpperType != pgtype.Unbounded {
			b.WriteString(formatRangeBound(val.Upper))
		}
		if val.UpperType == pgtype.Inclusive {
			b.WriteByte(']')
		} else {
			b.WriteByte(')')
		}
		return b.String()
	default:
		return v
	}
}

// formatRangeBound converts a single range bound value (as returned by pgx DecodeValue) to
// a string suitable for embedding inside a PostgreSQL range literal.
func formatRangeBound(v any) string {
	switch val := v.(type) {
	case time.Time:
		return val.Format(time.RFC3339Nano)
	case pgtype.InfinityModifier:
		if val == pgtype.Infinity {
			return "infinity"
		}
		return "-infinity"
	case int32:
		return strconv.FormatInt(int64(val), 10)
	case int64:
		return strconv.FormatInt(val, 10)
	case pgtype.Numeric:
		s := convertValue(val)
		if s == nil {
			return ""
		}
		return fmt.Sprintf("%v", s)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// buildPKWhere creates WHERE clause parts and args from a PrimaryKey.
func buildPKWhere(pk database.PrimaryKey) ([]string, []any) {
	parts := make([]string, 0, len(pk.Columns))
	args := make([]any, 0, len(pk.Columns))
	i := 1
	for col, val := range pk.Columns {
		parts = append(parts, fmt.Sprintf("%s = $%d", pgx.Identifier{col}.Sanitize(), i))
		args = append(args, val)
		i++
	}
	return parts, args
}

// Formatter provides PostgreSQL-specific value formatting for the TUI layer.
type Formatter struct{}

// SQLLiteral formats a value as a SQL literal for use in a generated UPDATE template.
// time.Time is rendered in RFC3339Nano (offset-based) so PostgreSQL can parse it back.
func (f *Formatter) SQLLiteral(val any) string {
	if val == nil {
		return "NULL"
	}
	if t, ok := val.(time.Time); ok {
		return "'" + t.Format(time.RFC3339Nano) + "'"
	}
	s := database.StringifyValue(val)
	if s == "NULL" {
		return "NULL"
	}
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// EditableString returns the value formatted for display in the inline edit input.
// time.Time uses RFC3339Nano so the user can submit it back to PostgreSQL unchanged.
func (f *Formatter) EditableString(val any) string {
	if t, ok := val.(time.Time); ok {
		return t.Format(time.RFC3339Nano)
	}
	return database.StringifyValue(val)
}
