package schemagen

import (
	"strings"
	"testing"
)

func mkTable(name string, fks ...string) *Table {
	t := &Table{Name: name}
	for _, ref := range fks {
		t.ForeignKeys = append(t.ForeignKeys, &ForeignKey{ReferencedTable: ref})
	}
	return t
}

func mkSchema(tables ...*Table) *Schema {
	s := &Schema{Tables: map[string]*Table{}, Indexes: map[string]*Index{}}
	for _, t := range tables {
		s.Tables[t.Name] = t
	}
	return s
}

func names(ts []*Table) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Name
	}
	return out
}

func mkTableWithCols(name string, colNames ...string) *Table {
	t := &Table{Name: name}
	for _, c := range colNames {
		t.Columns = append(t.Columns, &Column{Name: c, DataType: "text", Nullable: true})
	}
	return t
}

func TestComputeDiff_FindsMissingColumns(t *testing.T) {
	want := mkSchema(mkTableWithCols("users", "id", "email", "display_name"))
	got := mkSchema(mkTableWithCols("users", "id", "email"))

	d := ComputeDiff(want, got)
	if d.Empty() {
		t.Fatal("expected non-empty diff")
	}
	if len(d.MissingColumns["users"]) != 1 || d.MissingColumns["users"][0].Name != "display_name" {
		t.Errorf("expected missing display_name on users, got %+v", d.MissingColumns)
	}
}

func TestComputeDiff_FindsStaleColumns(t *testing.T) {
	// SQL chain has an old column (e.g. a renamed field). AM doesn't know about it.
	want := mkSchema(mkTableWithCols("users", "id", "email"))
	got := mkSchema(mkTableWithCols("users", "id", "email", "legacy_username"))

	d := ComputeDiff(want, got)
	if len(d.MissingColumns) != 0 {
		t.Errorf("expected no missing columns, got %+v", d.MissingColumns)
	}
	if len(d.StaleColumns["users"]) != 1 || d.StaleColumns["users"][0].Name != "legacy_username" {
		t.Errorf("expected stale legacy_username on users, got %+v", d.StaleColumns)
	}
}

func TestComputeDiff_StaleColumnsDoNotMakeDiffNonEmpty(t *testing.T) {
	// Diff.Empty() should ignore stale columns — they're informational only,
	// not blockers for the SQL chain being deployable.
	want := mkSchema(mkTableWithCols("users", "id"))
	got := mkSchema(mkTableWithCols("users", "id", "old_col"))

	d := ComputeDiff(want, got)
	if !d.Empty() {
		t.Errorf("stale-only diff should be Empty(), got MissingTables=%d MissingCols=%d SafeIdx=%d",
			len(d.MissingTables), len(d.MissingColumns), len(d.SafeIndexes))
	}
}

func TestComputeDiff_MissingColumnsRespectAutoMigrateOrder(t *testing.T) {
	// Output order should follow want.Columns ordinal_position — important so
	// reviewers see a stable diff when regenerating the migration.
	want := mkSchema(mkTableWithCols("things", "id", "alpha", "beta", "gamma"))
	got := mkSchema(mkTableWithCols("things", "id"))

	d := ComputeDiff(want, got)
	cols := d.MissingColumns["things"]
	if len(cols) != 3 || cols[0].Name != "alpha" || cols[1].Name != "beta" || cols[2].Name != "gamma" {
		t.Errorf("expected alpha,beta,gamma in order, got %v", cols)
	}
}

func TestComputeDiff_FindsMissingTables(t *testing.T) {
	want := mkSchema(mkTable("users"), mkTable("courses"), mkTable("posts"))
	got := mkSchema(mkTable("users"))

	d := ComputeDiff(want, got)

	if d.Empty() {
		t.Fatal("expected non-empty diff")
	}
	got_ := names(d.MissingTables)
	if !contains(got_, "courses") || !contains(got_, "posts") {
		t.Errorf("expected courses and posts as missing, got %v", got_)
	}
}

func TestComputeDiff_EmptyWhenIdentical(t *testing.T) {
	a := mkSchema(mkTable("users"), mkTable("courses"))
	b := mkSchema(mkTable("users"), mkTable("courses"))

	if !ComputeDiff(a, b).Empty() {
		t.Fatal("identical schemas should produce empty diff")
	}
}

func TestComputeDiff_TopologicalOrderForFKs(t *testing.T) {
	// posts FK courses; courses FK users. Missing in `got`: all three.
	// Expected order: users → courses → posts.
	want := mkSchema(
		mkTable("posts", "courses"),
		mkTable("courses", "users"),
		mkTable("users"),
	)
	got := mkSchema()

	d := ComputeDiff(want, got)
	order := names(d.MissingTables)

	uIdx, cIdx, pIdx := indexOf(order, "users"), indexOf(order, "courses"), indexOf(order, "posts")
	if uIdx > cIdx || cIdx > pIdx {
		t.Errorf("expected users < courses < posts, got %v", order)
	}
}

func TestComputeDiff_IgnoresFKsToExistingTables(t *testing.T) {
	// posts FK users. users already exists in `got`. Only posts is missing,
	// and it should sort cleanly (no waiting on users since it's not in the
	// missing set).
	want := mkSchema(mkTable("posts", "users"), mkTable("users"))
	got := mkSchema(mkTable("users"))

	d := ComputeDiff(want, got)
	if got_ := names(d.MissingTables); len(got_) != 1 || got_[0] != "posts" {
		t.Errorf("expected [posts], got %v", got_)
	}
}

func TestComputeDiff_DeterministicTieBreak(t *testing.T) {
	// All independent — should sort alphabetically.
	want := mkSchema(mkTable("zebra"), mkTable("alpha"), mkTable("mango"))
	got := mkSchema()

	d := ComputeDiff(want, got)
	order := names(d.MissingTables)

	if order[0] != "alpha" || order[1] != "mango" || order[2] != "zebra" {
		t.Errorf("expected alphabetical, got %v", order)
	}
}

func TestComputeDiff_HandlesSelfReference(t *testing.T) {
	// folders.parent_id → folders.id is legal (and used by the Folder model).
	want := mkSchema(mkTable("folders", "folders"))
	got := mkSchema()

	d := ComputeDiff(want, got)
	if got_ := names(d.MissingTables); len(got_) != 1 || got_[0] != "folders" {
		t.Errorf("self-ref should not stall topo sort, got %v", got_)
	}
}

func TestRenderCreateTable_Basic(t *testing.T) {
	tbl := &Table{
		Name: "feature_flags",
		Columns: []*Column{
			{Name: "id", DataType: "bigint", Nullable: false},
			{Name: "name", DataType: "character varying", MaxLength: int64Ptr(255), Nullable: false},
			{Name: "enabled", DataType: "boolean", Nullable: false, Default: strPtr("false")},
		},
		PrimaryKey: []string{"id"},
		UniqueConstraints: []*UniqueConstraint{
			{Name: "uq_feature_flags_name", Columns: []string{"name"}},
		},
	}
	got := RenderCreateTable(tbl)

	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS feature_flags",
		"id bigint NOT NULL",
		"name varchar(255) NOT NULL",
		"enabled boolean NOT NULL DEFAULT false",
		"PRIMARY KEY (id)",
		"CONSTRAINT uq_feature_flags_name UNIQUE (name)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

func indexOf(haystack []string, needle string) int {
	for i, h := range haystack {
		if h == needle {
			return i
		}
	}
	return -1
}

func int64Ptr(v int64) *int64 { return &v }
func strPtr(v string) *string { return &v }
