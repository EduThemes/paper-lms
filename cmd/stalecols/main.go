// stalecols categorizes the "stale columns" reported by schemadiff —
// columns present in the SQL migration chain but absent from GORM's
// AutoMigrate output. Each column is assigned a category guess
// (SOFT_DELETE_LEFTOVER, RENAME_CANDIDATE, POLYMORPHIC_REFACTOR, UNKNOWN)
// and written to STALE_COLUMNS.md so a human can decide per-column whether
// to migrate data, drop, or re-add to the model.
//
// Categories are guesses, not commitments — the file is meant to be edited
// by hand before Wave 2b/2c migrations are authored.
//
// Usage:
//
//	DATABASE_URL=postgres://paper:paper@localhost:5433/paper_lms?sslmode=disable \
//	    go run ./cmd/stalecols [-o STALE_COLUMNS.md]
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/db"
	"github.com/EduThemes/paper-lms/internal/db/schemagen"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type category string

const (
	catSoftDelete  category = "SOFT_DELETE_LEFTOVER"
	catRename      category = "RENAME_CANDIDATE"
	catPolymorphic category = "POLYMORPHIC_REFACTOR"
	catUnknown     category = "UNKNOWN"
)

// classification is the per-column result. SuggestedTarget is the AM column
// the heuristic thinks the stale column was renamed to (RENAME_CANDIDATE) or
// the polymorphic pair we found (POLYMORPHIC_REFACTOR). Empty otherwise.
type classification struct {
	Table           string
	Column          *schemagen.Column
	Category        category
	SuggestedTarget string
	Notes           string
}

func main() {
	os.Exit(run())
}

func run() int {
	outPath := flag.String("o", "STALE_COLUMNS.md", "output path for the categorization report")
	keep := flag.Bool("keep", false, "do not drop scratch databases on exit")
	flag.Parse()

	_ = godotenv.Load()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL not set")
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	suffix := fmt.Sprintf("%d_%d", time.Now().UnixNano(), os.Getpid())
	amName := "paper_lms_stalecols_am_" + suffix
	sqlName := "paper_lms_stalecols_sql_" + suffix

	adminURL := swapDatabase(dbURL, "postgres")
	admin, err := sql.Open("postgres", adminURL)
	if err != nil {
		return fatal("open admin: %v", err)
	}
	defer admin.Close()

	if err := createDB(ctx, admin, amName); err != nil {
		return fatal("create %s: %v", amName, err)
	}
	if err := createDB(ctx, admin, sqlName); err != nil {
		_ = dropDB(context.Background(), admin, amName)
		return fatal("create %s: %v", sqlName, err)
	}
	defer func() {
		if *keep {
			fmt.Fprintf(os.Stderr, "scratch DBs kept: %s, %s\n", amName, sqlName)
			return
		}
		_ = dropDB(context.Background(), admin, amName)
		_ = dropDB(context.Background(), admin, sqlName)
	}()

	amURL := swapDatabase(dbURL, amName)
	sqlURL := swapDatabase(dbURL, sqlName)
	for _, u := range []string{amURL, sqlURL} {
		if err := bootstrapExtensions(u); err != nil {
			return fatal("bootstrap extensions: %v", err)
		}
	}

	fmt.Fprintln(os.Stderr, "→ building AutoMigrate schema...")
	want, err := buildAutoMigrateSchema(amURL)
	if err != nil {
		return fatal("automigrate schema: %v", err)
	}
	fmt.Fprintf(os.Stderr, "  %d tables\n", len(want.Tables))

	fmt.Fprintln(os.Stderr, "→ building SQL-migration schema...")
	got, err := buildSQLMigrationSchema(sqlURL)
	if err != nil {
		return fatal("sql migration schema: %v", err)
	}
	fmt.Fprintf(os.Stderr, "  %d tables\n", len(got.Tables))

	d := schemagen.ComputeDiff(want, got)
	if len(d.StaleColumns) == 0 {
		fmt.Fprintln(os.Stderr, "✓ no stale columns to report")
		return 0
	}

	classifications := classifyAll(d.StaleColumns, want)
	md := renderMarkdown(classifications, len(d.StaleColumns))

	if err := os.WriteFile(*outPath, []byte(md), 0o644); err != nil {
		return fatal("write %s: %v", *outPath, err)
	}
	total := 0
	for _, cs := range groupByTable(classifications) {
		total += len(cs)
	}
	fmt.Fprintf(os.Stderr, "✓ wrote %s — %d stale columns across %d tables\n",
		*outPath, total, len(d.StaleColumns))
	return 0
}

// classifyAll walks every stale column on every table and assigns a category.
// Heuristics are local to each table; cross-table renames aren't considered.
func classifyAll(stale map[string][]*schemagen.Column, want *schemagen.Schema) []classification {
	var out []classification
	for table, cols := range stale {
		amTable := want.Tables[table]
		// amTable can be nil if the entire table is "stale" — but the diff
		// only populates StaleColumns on shared tables, so amTable should
		// always be present. Guard anyway.
		var amCols []*schemagen.Column
		if amTable != nil {
			amCols = amTable.Columns
		}
		for _, c := range cols {
			out = append(out, classify(table, c, amCols))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Table != out[j].Table {
			return out[i].Table < out[j].Table
		}
		return out[i].Column.Name < out[j].Column.Name
	})
	return out
}

func classify(table string, c *schemagen.Column, amCols []*schemagen.Column) classification {
	r := classification{Table: table, Column: c}

	// Heuristic 1: gorm.DeletedAt soft-delete column the model removed.
	if c.Name == "deleted_at" && isTimestamptz(c) {
		r.Category = catSoftDelete
		r.Notes = "GORM soft-delete column removed from the model. Safe to drop if no soft-deleted rows exist."
		return r
	}

	// Heuristic 2: polymorphic refactor. A *_id column on a table whose AM
	// model now exposes a (resource_type, resource_id) pair (or any matching
	// type+id polymorphic pair) is almost certainly a leftover from the
	// flattening of a one-target FK into a polymorphic association.
	if strings.HasSuffix(c.Name, "_id") {
		if pair := findPolymorphicPair(amCols); pair != "" {
			r.Category = catPolymorphic
			r.SuggestedTarget = pair
			r.Notes = fmt.Sprintf("Pre-polymorphic-refactor FK. Data migration must populate %s with a semantic value derived from the old %s.", pair, c.Name)
			return r
		}
	}

	// Heuristic 3: rename. Look for an AM column on the same table whose
	// normalized name is identical or one edit away. Normalization strips
	// underscores and lowercases — catches both `is_under_13` → `is_under13`
	// and case-difference variants without flagging unrelated short names.
	if target := findRenameTarget(c.Name, amCols); target != "" {
		r.Category = catRename
		r.SuggestedTarget = target
		r.Notes = "Likely renamed. Wave 2b should copy data forward; drop happens in Wave 2c."
		return r
	}

	r.Category = catUnknown
	r.Notes = "No matching AM column found. Could be a removed feature, an in-use column missing from the model, or a column referenced only by hand-written SQL. Grep the codebase before dropping."
	return r
}

// findPolymorphicPair returns the *_id half of a polymorphic pair on the AM
// table, or "" if no pair exists. A pair is two columns whose names share a
// prefix and end in `_type` and `_id` respectively. The pair name is the
// `<prefix>_id` column (the FK), since that's the column the data migration
// has to write into.
func findPolymorphicPair(amCols []*schemagen.Column) string {
	have := make(map[string]bool, len(amCols))
	for _, c := range amCols {
		have[c.Name] = true
	}
	for _, c := range amCols {
		if !strings.HasSuffix(c.Name, "_type") {
			continue
		}
		prefix := strings.TrimSuffix(c.Name, "_type")
		idName := prefix + "_id"
		if have[idName] {
			return idName
		}
	}
	return ""
}

// findRenameTarget returns the AM column name that most likely replaced this
// stale column, or "" if no plausible target is found. Heuristic: normalize
// (lowercase, strip underscores), then accept either:
//
//   - normalize-distance 0 (pure punctuation/case rename, e.g.
//     `idp_entity_id` ↔ `id_p_entity_id`, `is_under_13` ↔ `is_under13`)
//   - normalize-distance 1 when the longer normalized name is at least 8 chars
//     (e.g. a typo-correction or single-letter change on a long identifier)
//
// Distance 2 was tried and produced too many false positives: it conflates
// suffix swaps like `signed_by` ↔ `signed_at` or `comment` ↔ `content`, which
// are different fields rather than renames. Better to mark those UNKNOWN and
// let the human spot them than to mislead reviewers with confident guesses.
func findRenameTarget(stale string, amCols []*schemagen.Column) string {
	want := normalize(stale)
	if len(want) < 4 {
		return ""
	}
	for _, c := range amCols {
		got := normalize(c.Name)
		if got == want {
			return c.Name
		}
	}
	var bestName string
	bestDist := 2
	for _, c := range amCols {
		got := normalize(c.Name)
		d := levenshtein(want, got)
		if d < bestDist {
			bestDist = d
			bestName = c.Name
		}
	}
	if bestDist == 1 {
		longer := len(want)
		if l := len(normalize(bestName)); l > longer {
			longer = l
		}
		if longer >= 8 {
			return bestName
		}
	}
	return ""
}

func normalize(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, "_", ""))
}

// levenshtein returns the edit distance between a and b. Standard DP table.
// Used only for short identifiers (< 50 chars) so the O(n*m) cost is fine.
func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min3(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

func isTimestamptz(c *schemagen.Column) bool {
	return c.DataType == "timestamp with time zone"
}

// renderMarkdown writes the categorization report. Top-level summary first,
// then a per-table section with one row per stale column.
func renderMarkdown(cs []classification, tableCount int) string {
	var b strings.Builder
	total := len(cs)

	counts := map[category]int{}
	for _, c := range cs {
		counts[c.Category]++
	}

	fmt.Fprintf(&b, "# Stale Columns — Wave 2a Categorization\n\n")
	fmt.Fprintf(&b, "Generated by `make stale-cols` on %s.\n\n", time.Now().Format("2006-01-02"))
	fmt.Fprintf(&b, "**Stale column** = present in the SQL migration chain, absent from GORM AutoMigrate. ")
	fmt.Fprintf(&b, "Surfaced by `cmd/schemadiff` (Wave 1) but never auto-dropped. ")
	fmt.Fprintf(&b, "Wave 2b will author data migrations for clear renames; Wave 2c will drop confirmed-dead columns.\n\n")
	fmt.Fprintf(&b, "Total: **%d** stale columns across **%d** tables.\n\n", total, tableCount)

	b.WriteString("## Summary by category\n\n")
	b.WriteString("| Category | Count | Meaning |\n")
	b.WriteString("|---|---:|---|\n")
	b.WriteString(fmt.Sprintf("| `SOFT_DELETE_LEFTOVER` | %d | `deleted_at` column whose model dropped the `gorm.DeletedAt` field. Safe to drop if no soft-deleted rows. |\n", counts[catSoftDelete]))
	b.WriteString(fmt.Sprintf("| `RENAME_CANDIDATE` | %d | AM has a near-identical name on the same table — copy data forward, then drop. |\n", counts[catRename]))
	b.WriteString(fmt.Sprintf("| `POLYMORPHIC_REFACTOR` | %d | AM table has a `(*_type, *_id)` polymorphic pair; this is the old typed FK. Needs semantic data migration. |\n", counts[catPolymorphic]))
	b.WriteString(fmt.Sprintf("| `UNKNOWN` | %d | No matching AM column. Could be removed feature, model bug, or string-SQL reference. Investigate before dropping. |\n", counts[catUnknown]))
	b.WriteString("\n")

	b.WriteString("## How to read this file\n\n")
	b.WriteString("Each row is a single stale column. The **Category** is a heuristic guess — please verify before authoring migrations.\n\n")
	b.WriteString("Suggested workflow:\n\n")
	b.WriteString("1. For each table, confirm each row's category. Edit this file in place.\n")
	b.WriteString("2. Group rows by domain (assignments, outcomes, accommodations, etc.) for Wave 2b data migrations — one migration per domain.\n")
	b.WriteString("3. Before any drop in Wave 2c, grep the Go codebase for the column name (string SQL queries can reference columns that GORM models don't).\n\n")

	b.WriteString("## By table\n\n")

	grouped := groupByTable(cs)
	tables := make([]string, 0, len(grouped))
	for t := range grouped {
		tables = append(tables, t)
	}
	sort.Strings(tables)

	for _, t := range tables {
		rows := grouped[t]
		fmt.Fprintf(&b, "### `%s` (%d)\n\n", t, len(rows))
		b.WriteString("| Column | Type | Nullable | Default | Category | Suggested target | Notes |\n")
		b.WriteString("|---|---|---|---|---|---|---|\n")
		for _, r := range rows {
			fmt.Fprintf(&b, "| `%s` | `%s` | %s | %s | `%s` | %s | %s |\n",
				r.Column.Name,
				typeOf(r.Column),
				yesNo(r.Column.Nullable),
				defaultOf(r.Column),
				r.Category,
				codeOrDash(r.SuggestedTarget),
				r.Notes,
			)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func groupByTable(cs []classification) map[string][]classification {
	g := map[string][]classification{}
	for _, c := range cs {
		g[c.Table] = append(g[c.Table], c)
	}
	return g
}

// typeOf renders a human-readable type for the markdown report. Doesn't need
// to round-trip — the schemagen render package handles that for migrations.
func typeOf(c *schemagen.Column) string {
	switch c.DataType {
	case "USER-DEFINED", "ARRAY":
		return c.UDTName
	case "character varying":
		if c.MaxLength != nil {
			return fmt.Sprintf("varchar(%d)", *c.MaxLength)
		}
		return "varchar"
	case "timestamp with time zone":
		return "timestamptz"
	case "timestamp without time zone":
		return "timestamp"
	case "numeric":
		if c.NumericP != nil && c.NumericS != nil {
			return fmt.Sprintf("numeric(%d,%d)", *c.NumericP, *c.NumericS)
		}
		return "numeric"
	default:
		return c.DataType
	}
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func defaultOf(c *schemagen.Column) string {
	if c.Default == nil {
		return "—"
	}
	// Truncate long defaults (sequence references, function calls) so the
	// table stays readable in a normal editor width.
	s := *c.Default
	if len(s) > 40 {
		s = s[:37] + "..."
	}
	return "`" + s + "`"
}

func codeOrDash(s string) string {
	if s == "" {
		return "—"
	}
	return "`" + s + "`"
}

// --- scratch DB plumbing (copied from cmd/schemadiff — keeping this tool
// self-contained avoids a refactor that would touch Wave 1's working tool) ---

func fatal(format string, args ...any) int {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	return 2
}

func swapDatabase(rawURL, dbName string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	u.Path = "/" + dbName
	return u.String()
}

func createDB(ctx context.Context, admin *sql.DB, name string) error {
	_, err := admin.ExecContext(ctx, fmt.Sprintf(`CREATE DATABASE %q`, name))
	return err
}

func dropDB(ctx context.Context, admin *sql.DB, name string) error {
	_, err := admin.ExecContext(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, name))
	return err
}

func bootstrapExtensions(dbURL string) error {
	conn, err := sql.Open("postgres", dbURL)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Exec(`CREATE EXTENSION IF NOT EXISTS vector`)
	return err
}

func buildAutoMigrateSchema(dbURL string) (*schemagen.Schema, error) {
	gdb, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("gorm open: %w", err)
	}
	defer closeGorm(gdb)
	if err := db.AutoMigrate(gdb); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}
	raw, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	return schemagen.Introspect(raw)
}

func buildSQLMigrationSchema(dbURL string) (*schemagen.Schema, error) {
	gdb, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("gorm open: %w", err)
	}
	defer closeGorm(gdb)
	if err := db.MigrateUp(gdb); err != nil {
		return nil, fmt.Errorf("migrate up: %w", err)
	}
	raw, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	return schemagen.Introspect(raw)
}

func closeGorm(g *gorm.DB) {
	if sqlDB, err := g.DB(); err == nil {
		_ = sqlDB.Close()
	}
}
