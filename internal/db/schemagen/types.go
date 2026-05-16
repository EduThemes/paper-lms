// Package schemagen introspects, renders, and diffs Postgres schemas.
//
// It exists to keep the SQL migration chain (`internal/db/migrations/`) and
// GORM's AutoMigrate output in sync. AutoMigrate is convenient for development
// but production deploys run the SQL chain; when the two drift, a fresh prod
// install will be missing tables.
//
// Two scratch databases are introspected — one built by AutoMigrate, one by
// the SQL chain — and Diff() reports what the SQL chain is missing.
package schemagen

// Schema is a snapshot of a Postgres database's structural objects.
//
// Only the public schema is captured. The schema_migrations bookkeeping table
// (used by golang-migrate) is excluded so it never shows up as a diff.
type Schema struct {
	Tables  map[string]*Table // keyed by table name
	Indexes map[string]*Index // keyed by index name (globally unique in Postgres)
}

// Table is a single relation in the schema.
type Table struct {
	Name              string
	Columns           []*Column // ordered by ordinal_position
	PrimaryKey        []string  // column names in PK order
	UniqueConstraints []*UniqueConstraint
	ForeignKeys       []*ForeignKey
}

// Column captures everything we need to emit a CREATE TABLE column definition.
type Column struct {
	Name       string
	DataType   string         // information_schema.columns.data_type (logical)
	UDTName    string         // pg_type name (for USER-DEFINED / ARRAY)
	MaxLength  *int64         // character_maximum_length
	NumericP   *int64         // numeric_precision
	NumericS   *int64         // numeric_scale
	Nullable   bool
	Default    *string        // column_default, raw
}

// UniqueConstraint is a named UNIQUE constraint over one or more columns.
type UniqueConstraint struct {
	Name    string
	Columns []string
}

// ForeignKey is a named FK constraint. Used for topological ordering of
// CREATE TABLE statements when emitting backfill migrations.
type ForeignKey struct {
	Name              string
	Columns           []string
	ReferencedTable   string
	ReferencedColumns []string
	OnDelete          string // NO ACTION | RESTRICT | CASCADE | SET NULL | SET DEFAULT
	OnUpdate          string
}

// Index represents a non-PK index. PK indexes are implied by Table.PrimaryKey
// and are excluded so they don't show up twice.
type Index struct {
	Name    string
	Table   string
	Def     string // verbatim pg_indexes.indexdef (preserves USING, WHERE, INCLUDE)
	Unique  bool
}

// HasTable reports whether the schema declares a table with the given name.
// Convenience wrapper for targeted parity tests that assert presence of a
// specific table set (e.g. TestSchemaParity_Wave3).
func (s *Schema) HasTable(name string) bool {
	_, ok := s.Tables[name]
	return ok
}
