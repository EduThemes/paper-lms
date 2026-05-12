package main

import (
	"testing"

	"github.com/EduThemes/paper-lms/internal/db/schemagen"
)

// The heuristics under test are pure functions, so the suite stays out of
// Postgres territory. Each test names the input shape and the expected
// classification — when a heuristic gets retuned, these are the cases that
// must still hold.

func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"a", "", 1},
		{"", "abc", 3},
		{"kitten", "sitting", 3},
		{"isunder13", "isunder13", 0},
		{"signedby", "signedat", 2},
		{"comment", "content", 2},
	}
	for _, c := range cases {
		if got := levenshtein(c.a, c.b); got != c.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestNormalize(t *testing.T) {
	cases := []struct{ in, want string }{
		{"foo_bar", "foobar"},
		{"FooBar", "foobar"},
		{"IDP_ENTITY_ID", "idpentityid"},
		{"id_p_entity_id", "idpentityid"},
		{"", ""},
	}
	for _, c := range cases {
		if got := normalize(c.in); got != c.want {
			t.Errorf("normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// findRenameTarget accepts: normalize-distance 0 always; distance 1 only on
// identifiers ≥ 8 chars normalized. Distance 2 is always rejected so the
// suffix-swap pattern (signed_by ↔ signed_at) doesn't pollute renames.
func TestFindRenameTarget(t *testing.T) {
	amCols := func(names ...string) []*schemagen.Column {
		cols := make([]*schemagen.Column, len(names))
		for i, n := range names {
			cols[i] = &schemagen.Column{Name: n}
		}
		return cols
	}

	cases := []struct {
		name  string
		stale string
		am    []*schemagen.Column
		want  string
	}{
		{
			name:  "exact normalize match (acronym mangling)",
			stale: "idp_entity_id",
			am:    amCols("id_p_entity_id", "name"),
			want:  "id_p_entity_id",
		},
		{
			name:  "underscore-removed rename",
			stale: "is_under_13",
			am:    amCols("is_under13", "age"),
			want:  "is_under13",
		},
		{
			name:  "distance-1 on long identifier (≥8 chars)",
			stale: "applied_at",
			am:    amCols("applied_ats"),
			want:  "applied_ats",
		},
		{
			name:  "distance-2 rejected (suffix swap)",
			stale: "signed_by",
			am:    amCols("signed_at"),
			want:  "",
		},
		{
			name:  "distance-2 rejected (comment vs content)",
			stale: "comment",
			am:    amCols("content"),
			want:  "",
		},
		{
			name:  "short name skipped",
			stale: "id",
			am:    amCols("ip"),
			want:  "",
		},
		{
			name:  "no candidate",
			stale: "deprecated_flag",
			am:    amCols("name", "created_at"),
			want:  "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := findRenameTarget(c.stale, c.am); got != c.want {
				t.Errorf("findRenameTarget(%q) = %q, want %q", c.stale, got, c.want)
			}
		})
	}
}

// findPolymorphicPair returns the `*_id` half of a (*_type, *_id) pair so
// classify() can mark a legacy typed FK for migration into the pair.
func TestFindPolymorphicPair(t *testing.T) {
	amCols := func(names ...string) []*schemagen.Column {
		cols := make([]*schemagen.Column, len(names))
		for i, n := range names {
			cols[i] = &schemagen.Column{Name: n}
		}
		return cols
	}

	if got := findPolymorphicPair(amCols("resource_type", "resource_id", "name")); got != "resource_id" {
		t.Errorf("expected resource_id, got %q", got)
	}
	// Only the type half present — no pair.
	if got := findPolymorphicPair(amCols("resource_type", "name")); got != "" {
		t.Errorf("expected empty (no _id half), got %q", got)
	}
	// Only the id half present, no matching _type — no pair.
	if got := findPolymorphicPair(amCols("resource_id", "name")); got != "" {
		t.Errorf("expected empty (no _type half), got %q", got)
	}
	if got := findPolymorphicPair(amCols("name", "created_at")); got != "" {
		t.Errorf("expected empty (no pair), got %q", got)
	}
}

// classify runs the full heuristic ladder. One case per category to lock the
// dispatch order: soft-delete first, then polymorphic, then rename, else
// unknown.
func TestClassify(t *testing.T) {
	tstz := func(name string) *schemagen.Column {
		return &schemagen.Column{Name: name, DataType: "timestamp with time zone"}
	}
	bigint := func(name string) *schemagen.Column {
		return &schemagen.Column{Name: name, DataType: "bigint"}
	}
	textCol := func(name string) *schemagen.Column {
		return &schemagen.Column{Name: name, DataType: "text"}
	}

	t.Run("soft delete", func(t *testing.T) {
		r := classify("users", tstz("deleted_at"), nil)
		if r.Category != catSoftDelete {
			t.Errorf("got %s, want %s", r.Category, catSoftDelete)
		}
	})

	t.Run("polymorphic refactor", func(t *testing.T) {
		am := []*schemagen.Column{
			{Name: "resource_type", DataType: "text"},
			{Name: "resource_id", DataType: "bigint"},
		}
		r := classify("apps", bigint("assignment_id"), am)
		if r.Category != catPolymorphic {
			t.Errorf("got %s, want %s", r.Category, catPolymorphic)
		}
		if r.SuggestedTarget != "resource_id" {
			t.Errorf("got target %q, want resource_id", r.SuggestedTarget)
		}
	})

	t.Run("bool to timestamp refactor", func(t *testing.T) {
		am := []*schemagen.Column{{Name: "applied_at", DataType: "timestamp with time zone"}}
		r := classify("apps", &schemagen.Column{Name: "applied", DataType: "boolean"}, am)
		if r.Category != catBoolTimestamp {
			t.Errorf("got %s, want %s", r.Category, catBoolTimestamp)
		}
		if r.SuggestedTarget != "applied_at" {
			t.Errorf("got target %q, want applied_at", r.SuggestedTarget)
		}
	})

	t.Run("rename candidate (acronym mangling)", func(t *testing.T) {
		am := []*schemagen.Column{{Name: "id_p_entity_id", DataType: "text"}}
		r := classify("auth_providers", textCol("idp_entity_id"), am)
		if r.Category != catRename {
			t.Errorf("got %s, want %s", r.Category, catRename)
		}
		if r.SuggestedTarget != "id_p_entity_id" {
			t.Errorf("got target %q, want id_p_entity_id", r.SuggestedTarget)
		}
	})

	t.Run("unknown — no signal", func(t *testing.T) {
		am := []*schemagen.Column{{Name: "name", DataType: "text"}}
		r := classify("things", textCol("legacy_blob"), am)
		if r.Category != catUnknown {
			t.Errorf("got %s, want %s", r.Category, catUnknown)
		}
	})
}
