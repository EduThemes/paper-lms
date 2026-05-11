package schemagen

import (
	"database/sql"
	"fmt"
	"strings"
)

// Introspect builds a Schema snapshot of the connected database's public schema.
//
// schema_migrations is excluded — it's golang-migrate's bookkeeping table, not
// part of the application schema.
func Introspect(db *sql.DB) (*Schema, error) {
	s := &Schema{
		Tables:  make(map[string]*Table),
		Indexes: make(map[string]*Index),
	}

	tableNames, err := listTables(db)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}

	for _, name := range tableNames {
		t := &Table{Name: name}

		if t.Columns, err = listColumns(db, name); err != nil {
			return nil, fmt.Errorf("columns(%s): %w", name, err)
		}
		if t.PrimaryKey, err = listPrimaryKey(db, name); err != nil {
			return nil, fmt.Errorf("primary key(%s): %w", name, err)
		}
		if t.UniqueConstraints, err = listUniqueConstraints(db, name); err != nil {
			return nil, fmt.Errorf("unique constraints(%s): %w", name, err)
		}
		if t.ForeignKeys, err = listForeignKeys(db, name); err != nil {
			return nil, fmt.Errorf("foreign keys(%s): %w", name, err)
		}
		s.Tables[name] = t
	}

	if err := loadIndexes(db, s); err != nil {
		return nil, fmt.Errorf("indexes: %w", err)
	}

	return s, nil
}

func listTables(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public'
		  AND table_type   = 'BASE TABLE'
		  AND table_name != 'schema_migrations'
		ORDER BY table_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func listColumns(db *sql.DB, table string) ([]*Column, error) {
	rows, err := db.Query(`
		SELECT column_name, data_type, udt_name,
		       character_maximum_length, numeric_precision, numeric_scale,
		       is_nullable, column_default
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1
		ORDER BY ordinal_position
	`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []*Column
	for rows.Next() {
		c := &Column{}
		var nullable string
		var maxLen, numP, numS sql.NullInt64
		var def sql.NullString
		if err := rows.Scan(
			&c.Name, &c.DataType, &c.UDTName,
			&maxLen, &numP, &numS,
			&nullable, &def,
		); err != nil {
			return nil, err
		}
		c.Nullable = nullable == "YES"
		if maxLen.Valid {
			v := maxLen.Int64
			c.MaxLength = &v
		}
		if numP.Valid {
			v := numP.Int64
			c.NumericP = &v
		}
		if numS.Valid {
			v := numS.Int64
			c.NumericS = &v
		}
		if def.Valid {
			v := def.String
			c.Default = &v
		}
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

func listPrimaryKey(db *sql.DB, table string) ([]string, error) {
	rows, err := db.Query(`
		SELECT kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name
		 AND tc.table_schema    = kcu.table_schema
		WHERE tc.table_schema = 'public'
		  AND tc.table_name   = $1
		  AND tc.constraint_type = 'PRIMARY KEY'
		ORDER BY kcu.ordinal_position
	`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

func listUniqueConstraints(db *sql.DB, table string) ([]*UniqueConstraint, error) {
	rows, err := db.Query(`
		SELECT tc.constraint_name, kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name
		 AND tc.table_schema    = kcu.table_schema
		WHERE tc.table_schema = 'public'
		  AND tc.table_name   = $1
		  AND tc.constraint_type = 'UNIQUE'
		ORDER BY tc.constraint_name, kcu.ordinal_position
	`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byName := map[string]*UniqueConstraint{}
	var order []string
	for rows.Next() {
		var name, col string
		if err := rows.Scan(&name, &col); err != nil {
			return nil, err
		}
		if _, ok := byName[name]; !ok {
			byName[name] = &UniqueConstraint{Name: name}
			order = append(order, name)
		}
		byName[name].Columns = append(byName[name].Columns, col)
	}

	out := make([]*UniqueConstraint, 0, len(order))
	for _, n := range order {
		out = append(out, byName[n])
	}
	return out, rows.Err()
}

// listForeignKeys reads FK metadata from pg_catalog. For Wave 1 we only need
// the constraint name and the referenced table — the topological sort uses
// only ReferencedTable. Column lists and ON DELETE/UPDATE actions will be
// captured in Wave 2 when ALTER TABLE backfill is required.
func listForeignKeys(db *sql.DB, table string) ([]*ForeignKey, error) {
	rows, err := db.Query(`
		SELECT con.conname, cl_ref.relname
		FROM pg_constraint con
		JOIN pg_class cl ON con.conrelid = cl.oid
		JOIN pg_namespace ns ON cl.relnamespace = ns.oid
		JOIN pg_class cl_ref ON con.confrelid = cl_ref.oid
		WHERE ns.nspname = 'public'
		  AND cl.relname = $1
		  AND con.contype = 'f'
		ORDER BY con.conname
	`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*ForeignKey
	for rows.Next() {
		fk := &ForeignKey{}
		if err := rows.Scan(&fk.Name, &fk.ReferencedTable); err != nil {
			return nil, err
		}
		out = append(out, fk)
	}
	return out, rows.Err()
}

func loadIndexes(db *sql.DB, s *Schema) error {
	rows, err := db.Query(`
		SELECT indexname, tablename, indexdef
		FROM pg_indexes
		WHERE schemaname = 'public'
		  AND tablename != 'schema_migrations'
		  AND indexname NOT LIKE '%_pkey'
		ORDER BY tablename, indexname
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		idx := &Index{}
		if err := rows.Scan(&idx.Name, &idx.Table, &idx.Def); err != nil {
			return err
		}
		idx.Unique = strings.HasPrefix(idx.Def, "CREATE UNIQUE")
		s.Indexes[idx.Name] = idx
	}
	return rows.Err()
}
