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
	"bufio"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
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
	catSoftDelete    category = "SOFT_DELETE_LEFTOVER"
	catRename        category = "RENAME_CANDIDATE"
	catPolymorphic   category = "POLYMORPHIC_REFACTOR"
	catBoolTimestamp category = "BOOL_TO_TIMESTAMP_REFACTOR"
	catUnknown       category = "UNKNOWN"
)

// classification is the per-column result. SuggestedTarget is the AM column
// the heuristic thinks the stale column was renamed to (RENAME_CANDIDATE,
// BOOL_TO_TIMESTAMP_REFACTOR) or the polymorphic pair we found
// (POLYMORPHIC_REFACTOR). Empty otherwise.
//
// References are file:line locations in the Go source where the column name
// appears as a word-bounded token. Populated by the codebase scan after
// categorization. A non-empty References list on an UNKNOWN row means the
// column is referenced outside GORM's model registration — typically
// hand-written SQL or a model bug — and must be investigated before any
// Wave 2c drop.
type classification struct {
	Table           string
	Column          *schemagen.Column
	Category        category
	SuggestedTarget string
	Notes           string
	References      []string
	ExtraRefs       int // overflow beyond the kept References slice
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
		if err := os.WriteFile(*outPath, []byte(renderEmptyMarkdown()), 0o644); err != nil {
			return fatal("write %s: %v", *outPath, err)
		}
		fmt.Fprintf(os.Stderr, "✓ no stale columns to report (wrote clean %s)\n", *outPath)
		return 0
	}

	classifications := classifyAll(d.StaleColumns, want)

	fmt.Fprintln(os.Stderr, "→ scanning Go source for column references...")
	if err := populateReferences(classifications, "."); err != nil {
		return fatal("scan references: %v", err)
	}

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

	// Heuristic 3: bool→timestamp refactor. Common pattern: replace a boolean
	// `did_X` flag with a `did_X_at` timestamp where presence implies true.
	// Data migration is non-trivial (true → some-timestamp, false → NULL), so
	// these are split out from plain renames.
	if isBoolean(c) {
		target := c.Name + "_at"
		for _, am := range amCols {
			if am.Name == target && isTimestamptz(am) {
				r.Category = catBoolTimestamp
				r.SuggestedTarget = target
				r.Notes = "Bool→timestamp refactor: presence of " + target + " implies true. Data migration needs a chosen seed timestamp for true rows (created_at? updated_at? NOW()?)."
				return r
			}
		}
	}

	// Heuristic 4: rename. Look for an AM column on the same table whose
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

func isBoolean(c *schemagen.Column) bool {
	return c.DataType == "boolean"
}

// populateReferences walks the Go source under root and records word-bounded
// occurrences of each stale column name. The result lives on each
// classification's References slice (capped at maxRefs; overflow tracked in
// ExtraRefs).
//
// Excludes: vendor, node_modules, web (frontend), STALE_COLUMNS.md, the
// stalecols tool itself, and the SQL migrations directory (those by
// definition reference the columns we're investigating and are not evidence
// of live use).
//
// Excluding *_test.go files would risk missing tests that hardcode column
// names — those are also maintenance signal, so they stay in.
//
// One alternation regex is built up-front and applied per-file so the walk
// stays O(files), not O(files × columns).
func populateReferences(cs []classification, root string) error {
	const maxRefs = 3

	// Build a lookup from column name → indexes into cs. Multiple stale
	// columns can share a name across tables (e.g. `deleted_at`), so the
	// value is a slice of every classification that wants the reference.
	idx := map[string][]int{}
	names := make([]string, 0, len(cs))
	for i, c := range cs {
		if _, seen := idx[c.Column.Name]; !seen {
			names = append(names, c.Column.Name)
		}
		idx[c.Column.Name] = append(idx[c.Column.Name], i)
	}
	if len(names) == 0 {
		return nil
	}

	// One big alternation. Escape names defensively even though column names
	// are identifiers and shouldn't contain regex metacharacters.
	escaped := make([]string, len(names))
	for i, n := range names {
		escaped[i] = regexp.QuoteMeta(n)
	}
	pattern := `\b(` + strings.Join(escaped, "|") + `)\b`
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("compile alternation: %w", err)
	}

	skipDir := map[string]bool{
		"vendor":       true,
		"node_modules": true,
		"web":          true,
		".git":         true,
		// .claude holds agent worktrees — full clones of the repo that would
		// double or triple every reference count and make the column look
		// vastly more in-use than it is.
		".claude": true,
	}
	// Path-suffix excludes (relative to root). These directories are valid
	// Go but reference the very columns we're cataloging; counting them as
	// "live use" would defeat the purpose.
	skipRelDir := map[string]bool{
		"cmd/stalecols":         true,
		"cmd/schemadiff":        true,
		"cmd/genschema":         true,
		"internal/db/migrations": true,
	}

	return filepath.WalkDir(root, func(path string, dirent fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if dirent.IsDir() {
			if skipDir[dirent.Name()] {
				return filepath.SkipDir
			}
			rel, _ := filepath.Rel(root, path)
			if skipRelDir[filepath.ToSlash(rel)] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		return scanFileForReferences(path, root, re, idx, cs, maxRefs)
	})
}

// scanFileForReferences reads one Go file and, for every match of the
// alternation regex, appends a "path:line" entry to each classification that
// owns the matched column name. Matches beyond maxRefs are counted but not
// stored, so the report shows "+N more" without unbounded memory growth.
func scanFileForReferences(path, root string, re *regexp.Regexp, idx map[string][]int, cs []classification, maxRefs int) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	rel, _ := filepath.Rel(root, path)
	rel = filepath.ToSlash(rel)

	// Track which (file, column) pairs we've already credited so a column
	// referenced 20 times in one file doesn't drown out signal from columns
	// referenced once across many files.
	seen := map[int]bool{}

	scanner := bufio.NewScanner(f)
	// Allow long generated lines (some embedded SQL strings exceed 64KB).
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	line := 0
	for scanner.Scan() {
		line++
		matches := re.FindAllString(scanner.Text(), -1)
		if len(matches) == 0 {
			continue
		}
		for _, m := range matches {
			for _, ci := range idx[m] {
				if seen[ci] {
					continue
				}
				seen[ci] = true
				if len(cs[ci].References) < maxRefs {
					cs[ci].References = append(cs[ci].References, fmt.Sprintf("%s:%d", rel, line))
				} else {
					cs[ci].ExtraRefs++
				}
			}
		}
	}
	return scanner.Err()
}

// renderEmptyMarkdown produces the report content when the SQL chain and
// AutoMigrate are in full alignment. Keeps the file in lockstep with the
// repo's actual state so it doesn't pretend stale columns still exist.
func renderEmptyMarkdown() string {
	return fmt.Sprintf(`# Stale Columns — clean

Generated by `+"`make stale-cols`"+` on %s.

No stale columns: every column the SQL migration chain creates is also
declared by a GORM model. Waves 2b (data migration) and 2c (legacy drops)
are complete for the current model set.

Re-run `+"`make stale-cols`"+` after any model change to refresh this report.
`, time.Now().Format("2006-01-02"))
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
	b.WriteString(fmt.Sprintf("| `BOOL_TO_TIMESTAMP_REFACTOR` | %d | Bool flag replaced by `<name>_at` timestamp. Data migration must choose a seed timestamp for true rows. |\n", counts[catBoolTimestamp]))
	b.WriteString(fmt.Sprintf("| `UNKNOWN` | %d | No matching AM column. See **References** for Go-source occurrences before dropping. |\n", counts[catUnknown]))
	b.WriteString("\n")

	// Reference counts feed Wave 2c safety. A non-empty References list on an
	// UNKNOWN row almost always means hand-written SQL — must be refactored or
	// the column kept.
	withRefs, withoutRefs := 0, 0
	for _, c := range cs {
		if len(c.References) > 0 {
			withRefs++
		} else {
			withoutRefs++
		}
	}
	fmt.Fprintf(&b, "**References:** %d columns are referenced by name in Go source (likely live use); %d have no references found (likely dead, but verify per-domain).\n\n", withRefs, withoutRefs)

	b.WriteString("## How to read this file\n\n")
	b.WriteString("Each row is a single stale column. The **Category** is a heuristic guess — please verify before authoring migrations.\n\n")
	b.WriteString("Suggested workflow:\n\n")
	b.WriteString("1. For each table, confirm each row's category. Edit this file in place.\n")
	b.WriteString("2. Group rows by domain (assignments, outcomes, accommodations, etc.) for Wave 2b data migrations — one migration per domain.\n")
	b.WriteString("3. **References** is a pre-run grep of the Go codebase for the column name as a word-bounded token. A non-empty References cell on an UNKNOWN row means hand-written SQL or a model bug — investigate before any Wave 2c drop.\n")
	b.WriteString("4. References are matched **by column name only**, not by (table, column). Common names like `updated_at`, `created_at`, `name`, `description` will show inflated reference lists that include matches against other tables' columns of the same name. Use the file paths to judge relevance.\n")
	b.WriteString("5. The scan excludes `web/`, `vendor/`, `node_modules/`, `internal/db/migrations/`, `.claude/`, and the schema tooling under `cmd/`. Test files are included intentionally — tests that hardcode column names are also signal.\n\n")

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
		b.WriteString("| Column | Type | Nullable | Default | Category | Suggested target | References | Notes |\n")
		b.WriteString("|---|---|---|---|---|---|---|---|\n")
		for _, r := range rows {
			fmt.Fprintf(&b, "| `%s` | `%s` | %s | %s | `%s` | %s | %s | %s |\n",
				r.Column.Name,
				typeOf(r.Column),
				yesNo(r.Column.Nullable),
				defaultOf(r.Column),
				r.Category,
				codeOrDash(r.SuggestedTarget),
				renderRefs(r.References, r.ExtraRefs),
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

// renderRefs formats the References slice for the markdown table. Returns an
// em-dash when empty so the column reads naturally. Trailing "(+N more)" shows
// when the scan found more matches than maxRefs kept.
func renderRefs(refs []string, extra int) string {
	if len(refs) == 0 {
		return "—"
	}
	quoted := make([]string, len(refs))
	for i, r := range refs {
		quoted[i] = "`" + r + "`"
	}
	s := strings.Join(quoted, "<br>")
	if extra > 0 {
		s += fmt.Sprintf("<br>(+%d more)", extra)
	}
	return s
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
