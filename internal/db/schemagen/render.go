package schemagen

import (
	"fmt"
	"strings"
)

// RenderCreateTable emits a CREATE TABLE IF NOT EXISTS statement equivalent to
// what AutoMigrate produced. Output is deterministic: column order, PK position,
// and unique-constraint ordering all follow the Table struct's stored order
// (which itself comes from information_schema, sorted).
//
// The IF NOT EXISTS guard matches the pattern used by migrations 000001–000015
// and lets the backfill migration coexist with a partially-migrated DB.
func RenderCreateTable(t *Table) string {
	var lines []string
	for _, c := range t.Columns {
		lines = append(lines, "    "+renderColumn(c))
	}
	if len(t.PrimaryKey) > 0 {
		lines = append(lines, fmt.Sprintf("    PRIMARY KEY (%s)", strings.Join(t.PrimaryKey, ", ")))
	}
	for _, uc := range t.UniqueConstraints {
		lines = append(lines, fmt.Sprintf("    CONSTRAINT %s UNIQUE (%s)", uc.Name, strings.Join(uc.Columns, ", ")))
	}
	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n%s\n);", t.Name, strings.Join(lines, ",\n"))
}

// RenderAddColumn emits an idempotent `ALTER TABLE … ADD COLUMN IF NOT EXISTS`
// statement that matches what AutoMigrate would produce. Use this to backfill
// columns that AutoMigrate creates but the SQL migration chain doesn't.
//
// The IF NOT EXISTS guard makes the statement safe to re-run, including
// against databases that have been kept in sync via AutoMigrate up to now.
func RenderAddColumn(table string, c *Column) string {
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s;", table, renderColumn(c))
}

// RenderIndex emits an idempotent CREATE INDEX (rewriting pg_indexes.indexdef
// to use IF NOT EXISTS).
func RenderIndex(idx *Index) string {
	def := idx.Def
	def = strings.Replace(def, "CREATE INDEX ", "CREATE INDEX IF NOT EXISTS ", 1)
	def = strings.Replace(def, "CREATE UNIQUE INDEX ", "CREATE UNIQUE INDEX IF NOT EXISTS ", 1)
	if !strings.HasSuffix(def, ";") {
		def += ";"
	}
	return def
}

func renderColumn(c *Column) string {
	// Collapse `<int> NOT NULL DEFAULT nextval(...)` to the serial sugar.
	// Postgres treats them identically, but `bigserial` matches the convention
	// established in migration 000001 and is far more readable in code review.
	if serial := serialSugar(c); serial != "" {
		return fmt.Sprintf("%s %s", c.Name, serial)
	}

	out := fmt.Sprintf("%s %s", c.Name, renderType(c))
	if !c.Nullable {
		out += " NOT NULL"
	}
	if c.Default != nil {
		out += " DEFAULT " + *c.Default
	}
	return out
}

func serialSugar(c *Column) string {
	if c.Default == nil || c.Nullable {
		return ""
	}
	if !strings.HasPrefix(*c.Default, "nextval(") {
		return ""
	}
	switch c.DataType {
	case "bigint":
		return "bigserial"
	case "integer":
		return "serial"
	case "smallint":
		return "smallserial"
	}
	return ""
}

func renderType(c *Column) string {
	switch c.DataType {
	case "character varying":
		if c.MaxLength != nil {
			return fmt.Sprintf("varchar(%d)", *c.MaxLength)
		}
		return "text"
	case "character":
		if c.MaxLength != nil {
			return fmt.Sprintf("char(%d)", *c.MaxLength)
		}
		return "char(1)"
	case "numeric":
		if c.NumericP != nil && c.NumericS != nil {
			return fmt.Sprintf("numeric(%d,%d)", *c.NumericP, *c.NumericS)
		}
		return "numeric"
	case "integer":
		return "integer"
	case "bigint":
		return "bigint"
	case "smallint":
		return "smallint"
	case "boolean":
		return "boolean"
	case "text":
		return "text"
	case "timestamp with time zone":
		return "timestamptz"
	case "timestamp without time zone":
		return "timestamp"
	case "date":
		return "date"
	case "time without time zone":
		return "time"
	case "time with time zone":
		return "timetz"
	case "double precision":
		return "double precision"
	case "real":
		return "real"
	case "bytea":
		return "bytea"
	case "jsonb":
		return "jsonb"
	case "json":
		return "json"
	case "uuid":
		return "uuid"
	case "inet":
		return "inet"
	case "USER-DEFINED", "ARRAY":
		return c.UDTName
	default:
		return c.DataType
	}
}
