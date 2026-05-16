package gamification

import (
	"testing"

	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service/gamification/pseudonym"
)

func testPool() pseudonym.Pool {
	return pseudonym.Pool{
		Code:       pseudonym.PoolAnimals,
		Label:      "Test",
		Adjectives: []string{"Brave", "Calm", "Clever"},
		Nouns:      []string{"Otter", "Fox", "Owl"},
	}
}

func ranks(t *testing.T, scores ...int64) []repository.RankRow {
	t.Helper()
	rows := make([]repository.RankRow, len(scores))
	for i, s := range scores {
		rows[i] = repository.RankRow{
			UserID:         uint(100 + i),
			LifetimeEarned: s,
			Rank:           i + 1,
		}
	}
	return rows
}

// Viewer at rank 1 of a 1-student cohort: window pads 4 fillers below.
func TestComposeRelativeWindow_LoneViewerPadsFillersBelow(t *testing.T) {
	pool := testPool()
	ranked := ranks(t, 100)
	w := ComposeRelativeWindow(ranked, 100, pool, 7)

	if len(w.Rows) != RelativeWindowSize {
		t.Fatalf("want %d rows, got %d", RelativeWindowSize, len(w.Rows))
	}
	if !w.Rows[0].IsViewer {
		t.Errorf("viewer should be at index 0 when alone in cohort, got %#v", w.Rows[0])
	}
	for i := 1; i < RelativeWindowSize; i++ {
		if !w.Rows[i].IsFiller {
			t.Errorf("row %d should be a filler, got real row %#v", i, w.Rows[i])
		}
	}
	// Filler scores must decay from the VIEWER's score, not cumulatively
	// from each previous filler (the F4.1 audit fix). For an anchor of 100
	// and decay 0.85: 85, 72, 61, 52.
	expected := []int64{85, 72, 61, 52}
	for i, want := range expected {
		got := w.Rows[i+1].LifetimeEarned
		if got != want {
			t.Errorf("filler %d: want %d (100 * 0.85^%d), got %d", i, want, i+1, got)
		}
	}
	if w.NextToBeat != nil {
		t.Errorf("rank-1 viewer should not have a next-to-beat row, got %#v", w.NextToBeat)
	}
}

// Mid-pack viewer at rank 3 of a 6-cohort: window slices ±2 around them,
// no fillers needed.
func TestComposeRelativeWindow_MidPackNoFillers(t *testing.T) {
	pool := testPool()
	ranked := ranks(t, 500, 400, 300, 200, 100, 50)
	viewerID := uint(102) // rank 3
	w := ComposeRelativeWindow(ranked, viewerID, pool, 7)

	if len(w.Rows) != RelativeWindowSize {
		t.Fatalf("want %d rows, got %d", RelativeWindowSize, len(w.Rows))
	}
	// Index 2 should be the viewer (middle of 5-row window).
	if !w.Rows[2].IsViewer {
		t.Errorf("viewer should be at index 2, got %#v", w.Rows[2])
	}
	for _, r := range w.Rows {
		if r.IsFiller {
			t.Errorf("no fillers expected in mid-pack window, got %#v", r)
		}
	}
	if w.NextToBeat == nil {
		t.Fatal("mid-pack viewer should have a next-to-beat row")
	}
	if w.NextToBeat.UserID != 101 {
		t.Errorf("next-to-beat should be rank 2 (user 101), got %#v", w.NextToBeat)
	}
	// Gap = (above_score - viewer_score) + 1
	if w.NextToBeat.Gap != 101 {
		t.Errorf("gap should be 101 (400 - 300 + 1), got %d", w.NextToBeat.Gap)
	}
}

// Viewer in last place of a 4-cohort: 3 real peers above, 1 filler below.
// The user's W3-C requirement: never see myself dead last.
func TestComposeRelativeWindow_LastPlaceNeverDeadLast(t *testing.T) {
	pool := testPool()
	ranked := ranks(t, 500, 400, 300, 100)
	viewerID := uint(103) // rank 4 (last)
	w := ComposeRelativeWindow(ranked, viewerID, pool, 7)

	if len(w.Rows) != RelativeWindowSize {
		t.Fatalf("want %d rows, got %d", RelativeWindowSize, len(w.Rows))
	}
	// Viewer should NOT be at the bottom of the window — the last row
	// must be a filler.
	if w.Rows[RelativeWindowSize-1].IsViewer {
		t.Errorf("viewer should not be at the bottom; %#v", w.Rows[RelativeWindowSize-1])
	}
	if !w.Rows[RelativeWindowSize-1].IsFiller {
		t.Errorf("last row should be a filler so viewer is never dead last, got %#v", w.Rows[RelativeWindowSize-1])
	}
}

// Stable filler identity per (viewerEnrollmentID): same kid sees the
// same fillers across composes.
func TestComposeRelativeWindow_StableFillerIdentity(t *testing.T) {
	pool := testPool()
	ranked := ranks(t, 100)
	a := ComposeRelativeWindow(ranked, 100, pool, 42)
	b := ComposeRelativeWindow(ranked, 100, pool, 42)
	for i := range a.Rows {
		if a.Rows[i].IsFiller && a.Rows[i].Pseudonym != b.Rows[i].Pseudonym {
			t.Errorf("row %d filler name should be stable, got %q vs %q", i, a.Rows[i].Pseudonym, b.Rows[i].Pseudonym)
		}
	}
	// And different viewers see different fillers.
	c := ComposeRelativeWindow(ranked, 100, pool, 99)
	differs := false
	for i := range a.Rows {
		if a.Rows[i].IsFiller && a.Rows[i].Pseudonym != c.Rows[i].Pseudonym {
			differs = true
			break
		}
	}
	if !differs {
		t.Errorf("different viewer seed should produce different fillers")
	}
}

// Tenant-mode render policy: K-5 hides top-N from every student;
// HigherEd shows top-N only to top-5 viewers.
func TestRenderPolicyFor_TenantModeMatrix(t *testing.T) {
	cases := []struct {
		name       string
		tenant     string
		role       ViewerRole
		viewerRank int
		wantPseudo bool
		wantTopN   bool
	}{
		{"K-5 student rank 1 sees relative", "k5", ViewerStudent, 1, true, false},
		{"K-5 student rank 17 sees relative", "k5", ViewerStudent, 17, true, false},
		{"M68 student same", "m68", ViewerStudent, 3, true, false},
		{"H912 top-5 student sees top-N", "h912", ViewerStudent, 4, true, true},
		{"H912 mid-pack student sees relative", "h912", ViewerStudent, 12, true, false},
		{"HigherEd student real names, top-N if top-5", "higher_ed", ViewerStudent, 2, false, true},
		{"HigherEd mid-pack student real names, relative", "higher_ed", ViewerStudent, 99, false, false},
		{"Admin always sees real names + top-N", "k5", ViewerAdmin, 0, false, true},
		{"Teacher always sees real names + top-N", "k5", ViewerTeacher, 0, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := RenderPolicyFor(tc.tenant, tc.role, tc.viewerRank)
			if p.UsePseudonyms != tc.wantPseudo {
				t.Errorf("UsePseudonyms: want %v got %v", tc.wantPseudo, p.UsePseudonyms)
			}
			if p.ShowTopN != tc.wantTopN {
				t.Errorf("ShowTopN: want %v got %v", tc.wantTopN, p.ShowTopN)
			}
		})
	}
}
