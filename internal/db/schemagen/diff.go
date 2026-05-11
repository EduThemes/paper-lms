package schemagen

import (
	"fmt"
	"sort"
	"strings"
)

// Diff is the structural difference between two schemas. It's mostly
// one-directional — for the backfill problem we care about what `want`
// (AutoMigrate) has that `got` (the SQL chain) is missing — but we also report
// stale columns (in `got` but not `want`) as informational signal so operators
// can decide whether to clean up old refactor leftovers.
//
// MissingTables is ordered topologically by foreign-key dependency: a table
// always appears after the tables it references. Ties are broken alphabetically
// so the output is deterministic across runs.
//
// MissingColumns is the per-table list of columns AutoMigrate adds that the
// SQL chain hasn't caught up with. Each entry carries the full Column struct
// (type, nullability, default) so the renderer can emit complete ALTER TABLE
// ADD COLUMN statements.
//
// StaleColumns is the reverse: columns the SQL chain creates that AutoMigrate
// no longer recognizes. These are usually leftovers from model refactors. We
// surface them but never auto-emit DROP COLUMN — that's a data-destructive
// decision that must be made per-column by a human.
//
// SafeIndexes are indexes whose host table and all referenced columns exist in
// the post-backfill schema. DeferredIndexes reference columns the SQL chain is
// missing — once those columns are added, the deferred indexes graduate to
// safe on the next diff.
type Diff struct {
	MissingTables   []*Table
	MissingColumns  map[string][]*Column // table → columns AM has that SQL chain doesn't
	StaleColumns    map[string][]*Column // table → columns SQL chain has that AM doesn't (informational)
	SafeIndexes     []*Index
	DeferredIndexes []*Index
	ColumnDrift     map[string][]string // table → missing column names referenced by indexes (subset of MissingColumns)
}

// Empty reports whether the schemas are in full parity. The bar is strict:
// no missing tables, no missing columns, no creatable indexes outstanding.
// Stale columns and deferred indexes are informational and don't count.
func (d *Diff) Empty() bool {
	return len(d.MissingTables) == 0 &&
		len(d.MissingColumns) == 0 &&
		len(d.SafeIndexes) == 0 &&
		len(d.DeferredIndexes) == 0
}

// ComputeDiff returns the structural difference between `want` and `got`.
// Three layers are checked: table presence, column presence (in shared tables,
// both directions), and index presence.
func ComputeDiff(want, got *Schema) *Diff {
	d := &Diff{
		MissingColumns: map[string][]*Column{},
		StaleColumns:   map[string][]*Column{},
		ColumnDrift:    map[string][]string{},
	}

	var missingNames []string
	for name := range want.Tables {
		if _, ok := got.Tables[name]; !ok {
			missingNames = append(missingNames, name)
		}
	}

	d.MissingTables = topoSortTables(want, missingNames)

	// Column drift on shared tables — both directions.
	for name, wantTable := range want.Tables {
		gotTable, shared := got.Tables[name]
		if !shared {
			continue
		}
		gotCols := map[string]*Column{}
		for _, c := range gotTable.Columns {
			gotCols[c.Name] = c
		}
		wantCols := map[string]*Column{}
		for _, c := range wantTable.Columns {
			wantCols[c.Name] = c
		}

		// Missing: AM has, SQL doesn't. Order preserved from want.Columns
		// (which is information_schema ordinal_position) so the emitted ALTER
		// TABLE statements come out in a stable, AutoMigrate-style order.
		for _, c := range wantTable.Columns {
			if _, ok := gotCols[c.Name]; !ok {
				d.MissingColumns[name] = append(d.MissingColumns[name], c)
			}
		}
		// Stale: SQL has, AM doesn't.
		for _, c := range gotTable.Columns {
			if _, ok := wantCols[c.Name]; !ok {
				d.StaleColumns[name] = append(d.StaleColumns[name], c)
			}
		}
	}

	// Indexes are matched by name (Postgres index names are unique per schema).
	// We split into:
	//   - SafeIndexes: host table exists AND every referenced column exists in
	//     the post-backfill schema. These go into the up migration.
	//   - DeferredIndexes: host table exists in `got` but at least one column
	//     referenced by the index is missing. The underlying column drift must
	//     be fixed before these indexes can be created.
	//
	// Indexes on missing tables are always safe — those columns are created by
	// our own CREATE TABLE statements.

	d.ColumnDrift = map[string][]string{}

	missingTableSet := make(map[string]bool, len(d.MissingTables))
	for _, t := range d.MissingTables {
		missingTableSet[t.Name] = true
	}

	var safeNames, deferredNames []string
	for name, idx := range want.Indexes {
		if _, ok := got.Indexes[name]; ok {
			continue
		}
		// Index on a missing table → safe (we'll create the table + columns).
		if missingTableSet[idx.Table] {
			safeNames = append(safeNames, name)
			continue
		}
		// Index on an existing table — check column presence in `got`.
		gotTable, hostExists := got.Tables[idx.Table]
		if !hostExists {
			// Host neither exists nor is being created. Skip silently —
			// shouldn't happen in practice but guards against bad inputs.
			continue
		}
		missingCols := indexColumnsMissing(idx, gotTable)
		if len(missingCols) == 0 {
			safeNames = append(safeNames, name)
			continue
		}
		deferredNames = append(deferredNames, name)
		for _, c := range missingCols {
			if !containsString(d.ColumnDrift[idx.Table], c) {
				d.ColumnDrift[idx.Table] = append(d.ColumnDrift[idx.Table], c)
			}
		}
	}

	sort.Strings(safeNames)
	for _, n := range safeNames {
		d.SafeIndexes = append(d.SafeIndexes, want.Indexes[n])
	}
	sort.Strings(deferredNames)
	for _, n := range deferredNames {
		d.DeferredIndexes = append(d.DeferredIndexes, want.Indexes[n])
	}
	for t := range d.ColumnDrift {
		sort.Strings(d.ColumnDrift[t])
	}

	return d
}

// indexColumnsMissing returns the column names referenced by `idx` that are
// absent from `gotTable`. Columns we can't parse out of the indexdef are
// treated as present — pg_indexes.indexdef is stable for the GORM-style
// `... USING btree (col1, col2)` form we care about, but expression indexes
// like `lower(name)` aren't safely parseable without a full SQL parser.
func indexColumnsMissing(idx *Index, gotTable *Table) []string {
	cols := parseIndexColumns(idx.Def)
	if len(cols) == 0 {
		return nil
	}
	have := map[string]bool{}
	for _, c := range gotTable.Columns {
		have[c.Name] = true
	}
	var missing []string
	for _, c := range cols {
		if !have[c] {
			missing = append(missing, c)
		}
	}
	return missing
}

func containsString(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

// parseIndexColumns extracts column names from a pg_indexes.indexdef string.
// Handles the GORM-generated form:
//   CREATE [UNIQUE] INDEX <name> ON <schema>.<table> USING <method> (col1, col2 ...)
// Expressions (e.g. lower(name)) are skipped — we can't safely validate those
// without a full SQL parser, so they default to "safe" and we'll catch any
// failure at migration-run time.
func parseIndexColumns(def string) []string {
	open := strings.LastIndex(def, "(")
	close := strings.LastIndex(def, ")")
	if open < 0 || close < 0 || close < open {
		return nil
	}
	inner := def[open+1 : close]
	parts := strings.Split(inner, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		// Drop ASC/DESC, NULLS FIRST/LAST modifiers.
		if i := strings.IndexAny(p, " \t"); i >= 0 {
			p = p[:i]
		}
		// Skip anything that looks like an expression.
		if strings.ContainsAny(p, "()") {
			continue
		}
		// Skip empty.
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

// topoSortTables orders `names` (a subset of want.Tables) so that any table
// appears after all referenced tables that are also in `names`. References to
// tables outside `names` (i.e., already-existing in `got`) are ignored — those
// will already be present when the CREATE TABLE runs.
//
// Algorithm: Kahn's algorithm with alphabetical tie-breaking. Self-references
// are tolerated (Postgres allows them and AutoMigrate can produce them).
func topoSortTables(want *Schema, names []string) []*Table {
	if len(names) == 0 {
		return nil
	}

	inSet := make(map[string]bool, len(names))
	for _, n := range names {
		inSet[n] = true
	}

	// Build dependency graph restricted to `names`. edges[a] = tables that
	// must come before a.
	deps := make(map[string]map[string]bool, len(names))
	for _, n := range names {
		deps[n] = map[string]bool{}
		for _, fk := range want.Tables[n].ForeignKeys {
			if fk.ReferencedTable == n {
				continue // self-ref, ignore
			}
			if inSet[fk.ReferencedTable] {
				deps[n][fk.ReferencedTable] = true
			}
		}
	}

	// Kahn: repeatedly pick zero-indegree nodes alphabetically.
	var out []*Table
	for len(deps) > 0 {
		var ready []string
		for n, dset := range deps {
			if len(dset) == 0 {
				ready = append(ready, n)
			}
		}
		if len(ready) == 0 {
			// Cycle. Emit remaining alphabetically — Postgres will tolerate
			// circular FKs as long as both tables exist before the constraints
			// are validated, but with our IF NOT EXISTS + deferred-FK-via-
			// ALTER-TABLE pattern this shouldn't happen in practice.
			remaining := make([]string, 0, len(deps))
			for n := range deps {
				remaining = append(remaining, n)
			}
			sort.Strings(remaining)
			for _, n := range remaining {
				out = append(out, want.Tables[n])
			}
			break
		}
		sort.Strings(ready)
		for _, n := range ready {
			out = append(out, want.Tables[n])
			delete(deps, n)
		}
		for n := range deps {
			for _, r := range ready {
				delete(deps[n], r)
			}
		}
	}
	return out
}

// RenderMigration emits the up-migration SQL for a Diff. Tables are grouped by
// the section labels provided in groupOf — typically derived from the
// AutoMigrate registration list — so the output is readable in review.
// Tables without a known group are placed in an "uncategorized" bucket at the
// end (this should be zero once the project is wired correctly).
func RenderMigration(d *Diff, groupOf map[string]string, header string) string {
	var b strings.Builder
	if header != "" {
		b.WriteString(header)
		if !strings.HasSuffix(header, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Group tables in stable order: by first appearance in d.MissingTables.
	type group struct {
		label  string
		tables []*Table
	}
	var groups []*group
	byLabel := map[string]*group{}
	for _, t := range d.MissingTables {
		label := groupOf[t.Name]
		if label == "" {
			label = "uncategorized"
		}
		g, ok := byLabel[label]
		if !ok {
			g = &group{label: label}
			byLabel[label] = g
			groups = append(groups, g)
		}
		g.tables = append(g.tables, t)
	}

	for _, g := range groups {
		fmt.Fprintf(&b, "-- === Section: %s ===\n\n", g.label)
		for _, t := range g.tables {
			b.WriteString(RenderCreateTable(t))
			b.WriteString("\n\n")
		}
	}

	// ALTER TABLE ADD COLUMN section. Tables are listed alphabetically and
	// columns within a table preserve their AutoMigrate ordinal order — both
	// are deterministic so re-running the tool produces identical output.
	if len(d.MissingColumns) > 0 {
		b.WriteString("-- === Section: columns ===\n\n")
		tableNames := make([]string, 0, len(d.MissingColumns))
		for n := range d.MissingColumns {
			tableNames = append(tableNames, n)
		}
		sort.Strings(tableNames)
		for _, name := range tableNames {
			for _, c := range d.MissingColumns[name] {
				b.WriteString(RenderAddColumn(name, c))
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}

	// Indexes section. Combine safe + deferred: deferred indexes are only
	// "deferred" because of missing columns, but those columns appear earlier
	// in this same migration (Section: columns above), so by the time these
	// CREATE INDEX statements run, the columns exist. Combining them produces
	// a single coherent backfill that doesn't leave indexes orphaned.
	allIndexes := make([]*Index, 0, len(d.SafeIndexes)+len(d.DeferredIndexes))
	allIndexes = append(allIndexes, d.SafeIndexes...)
	allIndexes = append(allIndexes, d.DeferredIndexes...)
	sort.SliceStable(allIndexes, func(i, j int) bool { return allIndexes[i].Name < allIndexes[j].Name })
	if len(allIndexes) > 0 {
		b.WriteString("-- === Section: indexes ===\n\n")
		for _, idx := range allIndexes {
			b.WriteString(RenderIndex(idx))
			b.WriteString("\n")
		}
	}
	return b.String()
}
