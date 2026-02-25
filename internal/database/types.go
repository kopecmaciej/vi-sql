package database

// Row represents a single database row as a map of column name to value.
type Row = map[string]any

// PrimaryKey identifies a row by its primary key column(s).
// Supports composite primary keys.
type PrimaryKey struct {
	Columns map[string]any
}

// SchemaWithTables represents a database schema and its tables.
type SchemaWithTables struct {
	Schema string
	Tables []string
}

// ColumnInfo describes a table column.
type ColumnInfo struct {
	Name       string
	DataType   string
	IsNullable bool
	Default    *string
	IsPK       bool
	Ordinal    int
	Comment    string
}

// ConstraintInfo describes a table constraint.
type ConstraintInfo struct {
	Name    string
	Type    string // PRIMARY KEY, UNIQUE, CHECK, EXCLUDE
	Columns []string
	Def     string
}

// ForeignKeyInfo describes a foreign key relationship.
type ForeignKeyInfo struct {
	Name             string
	Columns          []string
	ReferencedSchema string
	ReferencedTable  string
	ReferencedCols   []string
	OnUpdate         string
	OnDelete         string
}

// IndexInfo describes an existing index.
type IndexInfo struct {
	Name       string
	Columns    []string
	IsUnique   bool
	IsPrimary  bool
	Type       string // btree, hash, gin, gist, etc.
	Size       int64
	Definition string
}

// IndexDefinition is used to create a new index.
type IndexDefinition struct {
	Name     string
	Columns  []string
	IsUnique bool
	Type     string
}

// ServerInfo provides database server metadata.
type ServerInfo struct {
	Version        string
	Uptime         string
	ActiveSessions int64
	CurrentDB      string
	Host           string
	Port           int
	Extra          map[string]string
}
