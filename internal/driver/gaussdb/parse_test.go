package gaussdb

import (
	"reflect"
	"testing"
)

func TestNormalizeQueryResultTypeName(t *testing.T) {
	tests := []struct {
		name       string
		pgName     string
		compatMode CompatMode
		want       string
	}{
		{"M mode numeric maps to decimal", "NUMERIC", CompatMySQL, "decimal"},
		{"M mode int4 maps to int", "INT4", CompatMySQL, "int"},
		{"M mode int8 maps to bigint", "INT8", CompatMySQL, "bigint"},
		{"M mode int2 maps to smallint", "INT2", CompatMySQL, "smallint"},
		{"M mode bool maps to boolean", "BOOL", CompatMySQL, "boolean"},
		{"M mode float4 maps to float", "FLOAT4", CompatMySQL, "float"},
		{"M mode float8 maps to double", "FLOAT8", CompatMySQL, "double"},
		{"M mode bpchar maps to char", "BPCHAR", CompatMySQL, "char"},
		{"M mode unaffected types are lowercased", "TEXT", CompatMySQL, "text"},
		{"A mode keeps the PG canonical name", "NUMERIC", CompatOracle, "numeric"},
		{"A mode int4 stays int4", "INT4", CompatOracle, "int4"},
		{"A mode unknown name is lowercased", "VARCHAR", CompatOracle, "varchar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeQueryResultTypeName(tt.pgName, tt.compatMode)
			if got != tt.want {
				t.Errorf("normalizeQueryResultTypeName(%q, %q) = %q, want %q",
					tt.pgName, tt.compatMode, got, tt.want)
			}
		})
	}
}

func TestParseIndexColumns(t *testing.T) {
	tests := []struct {
		name       string
		definition string
		want       []string
	}{
		{
			name:       "simple single column",
			definition: "CREATE INDEX idx_users_email ON users USING btree (email)",
			want:       []string{"email"},
		},
		{
			name:       "composite index",
			definition: "CREATE INDEX idx_users_names ON users USING btree (first_name, last_name)",
			want:       []string{"first_name", "last_name"},
		},
		{
			name:       "unique primary index",
			definition: "CREATE UNIQUE INDEX users_pkey ON users USING btree (id)",
			want:       []string{"id"},
		},
		{
			name:       "partial index with trailing where clause",
			definition: "CREATE INDEX idx_users_active ON users USING btree (active) WHERE (deleted_at IS NULL)",
			want:       []string{"active"},
		},
		{
			name:       "expression index with commas inside function call",
			definition: "CREATE INDEX idx_users_fullname ON users USING btree (concat(first_name, ' ', last_name))",
			want:       []string{"concat(first_name, ' ', last_name)"},
		},
		{
			name:       "mixed expression and column",
			definition: "CREATE INDEX idx_orders_total ON orders USING btree (lower(customer_name), total)",
			want:       []string{"lower(customer_name)", "total"},
		},
		{
			name:       "quoted identifier with comma in name",
			definition: `CREATE INDEX idx_weird ON t USING btree ("my,col")`,
			want:       []string{`"my,col"`},
		},
		{
			name:       "gin index with include clause",
			definition: "CREATE INDEX idx_docs ON docs USING gin (body) INCLUDE (id, title)",
			want:       []string{"body"},
		},
		{
			name:       "no parentheses",
			definition: "CREATE INDEX idx_users_email ON users USING btree email",
			want:       nil,
		},
		{
			name:       "table with schema prefix",
			definition: "CREATE INDEX idx_users_email ON public.users USING btree (email)",
			want:       []string{"email"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseIndexColumns(tt.definition)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseIndexColumns(%q) = %v, want %v", tt.definition, got, tt.want)
			}
		})
	}
}
