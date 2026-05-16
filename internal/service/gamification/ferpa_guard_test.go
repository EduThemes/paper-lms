package gamification_test

// Table-driven tests for the FERPA guard. DB-free: a fake repo holds
// the tag set, fixtures build events from raw JSON bytes. Sprint C
// Wave 1 only enforces education_record; the other classifications
// are advisory and have a negative-case test below to lock that in.

import (
	"context"
	"errors"
	"sort"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
	"github.com/lib/pq"
	"gorm.io/datatypes"
)

// fakeTagRepo satisfies repository.GamificationFerpaFieldTagRepository
// with a static tag list. Upsert/Find aren't exercised by the guard,
// so they panic if a future change wires them in unexpectedly.
type fakeTagRepo struct {
	tags []models.GamificationFerpaFieldTag
}

func (f *fakeTagRepo) ListByObjectType(_ context.Context, _ string) ([]models.GamificationFerpaFieldTag, error) {
	return f.tags, nil
}

func (f *fakeTagRepo) Upsert(_ context.Context, _ *models.GamificationFerpaFieldTag) error {
	panic("Upsert not used by guard")
}

func (f *fakeTagRepo) Find(_ context.Context, _, _ string) (*models.GamificationFerpaFieldTag, error) {
	panic("Find not used by guard")
}

// erroringTagRepo lets us verify repo errors propagate. Not used in
// the table cases (those want a happy-path repo) but kept so the
// behavior is explicit.
type erroringTagRepo struct{ err error }

func (e *erroringTagRepo) ListByObjectType(_ context.Context, _ string) ([]models.GamificationFerpaFieldTag, error) {
	return nil, e.err
}
func (e *erroringTagRepo) Upsert(_ context.Context, _ *models.GamificationFerpaFieldTag) error {
	panic("unused")
}
func (e *erroringTagRepo) Find(_ context.Context, _, _ string) (*models.GamificationFerpaFieldTag, error) {
	panic("unused")
}

func tag(objType, fieldPath, classification string) models.GamificationFerpaFieldTag {
	return models.GamificationFerpaFieldTag{
		ObjectType:     objType,
		FieldPath:      fieldPath,
		Classification: models.FerpaClassification(classification),
	}
}

func event(objType string, result, contextJSON string, flags ...string) *models.GamificationEvent {
	e := &models.GamificationEvent{
		ObjectType:  objType,
		PolicyFlags: pq.StringArray(flags),
	}
	if result != "" {
		e.Result = datatypes.JSON([]byte(result))
	}
	if contextJSON != "" {
		e.Context = datatypes.JSON([]byte(contextJSON))
	}
	return e
}

func TestCheckFerpa(t *testing.T) {
	cases := []struct {
		name     string
		tags     []models.GamificationFerpaFieldTag
		event    *models.GamificationEvent
		expected []gamification.FerpaViolation
	}{
		{
			name:     "no tags declared → allow",
			tags:     nil,
			event:    event("assignment_submission", `{"score":91}`, "", "ferpa_protected", "education_record"),
			expected: nil,
		},
		{
			name: "education_record with both required flags → allow",
			tags: []models.GamificationFerpaFieldTag{
				tag("assignment_submission", "result.score", "education_record"),
			},
			event:    event("assignment_submission", `{"score":91}`, "", "ferpa_protected", "education_record"),
			expected: nil,
		},
		{
			name: "education_record missing ferpa_protected → violation",
			tags: []models.GamificationFerpaFieldTag{
				tag("assignment_submission", "result.score", "education_record"),
			},
			event: event("assignment_submission", `{"score":91}`, "", "education_record"),
			expected: []gamification.FerpaViolation{
				{
					ObjectType:     "assignment_submission",
					FieldPath:      "result.score",
					Classification: "education_record",
					Missing:        []string{"ferpa_protected"},
				},
			},
		},
		{
			name: "education_record with no flags → violation lists both",
			tags: []models.GamificationFerpaFieldTag{
				tag("assignment_submission", "result.score", "education_record"),
			},
			event: event("assignment_submission", `{"score":91}`, ""),
			expected: []gamification.FerpaViolation{
				{
					ObjectType:     "assignment_submission",
					FieldPath:      "result.score",
					Classification: "education_record",
					Missing:        []string{"ferpa_protected", "education_record"},
				},
			},
		},
		{
			name: "context.course_id tagged, event carries it without flags → violation",
			tags: []models.GamificationFerpaFieldTag{
				tag("assignment_submission", "context.course_id", "education_record"),
			},
			event: event("assignment_submission", "", `{"course_id":42}`),
			expected: []gamification.FerpaViolation{
				{
					ObjectType:     "assignment_submission",
					FieldPath:      "context.course_id",
					Classification: "education_record",
					Missing:        []string{"ferpa_protected", "education_record"},
				},
			},
		},
		{
			name: "tagged field absent on event → allow",
			tags: []models.GamificationFerpaFieldTag{
				tag("assignment_submission", "result.score", "education_record"),
			},
			event:    event("assignment_submission", `{"feedback":"ok"}`, ""),
			expected: nil,
		},
		{
			name: "two education_record fields, both unflagged → two violations",
			tags: []models.GamificationFerpaFieldTag{
				tag("assignment_submission", "result.score", "education_record"),
				tag("assignment_submission", "context.course_id", "education_record"),
			},
			event: event("assignment_submission", `{"score":91}`, `{"course_id":42}`),
			expected: []gamification.FerpaViolation{
				{
					ObjectType:     "assignment_submission",
					FieldPath:      "result.score",
					Classification: "education_record",
					Missing:        []string{"ferpa_protected", "education_record"},
				},
				{
					ObjectType:     "assignment_submission",
					FieldPath:      "context.course_id",
					Classification: "education_record",
					Missing:        []string{"ferpa_protected", "education_record"},
				},
			},
		},
		{
			name: "directory_information classification → no violation (advisory)",
			tags: []models.GamificationFerpaFieldTag{
				tag("assignment_submission", "result.score", "directory_information"),
			},
			event:    event("assignment_submission", `{"score":91}`, ""),
			expected: nil,
		},
		{
			name: "empty Result and Context → no violations, no panic",
			tags: []models.GamificationFerpaFieldTag{
				tag("assignment_submission", "result.score", "education_record"),
			},
			event:    event("assignment_submission", "", ""),
			expected: nil,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeTagRepo{tags: tc.tags}
			got, err := gamification.CheckFerpa(context.Background(), repo, tc.event)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertViolationsEqual(t, tc.expected, got)
		})
	}
}

func TestCheckFerpa_MalformedJSON(t *testing.T) {
	repo := &fakeTagRepo{tags: []models.GamificationFerpaFieldTag{
		tag("assignment_submission", "result.score", "education_record"),
	}}
	ev := event("assignment_submission", `{not json`, "", "ferpa_protected", "education_record")

	got, err := gamification.CheckFerpa(context.Background(), repo, ev)
	if err == nil {
		t.Fatalf("expected error for malformed Result JSON, got nil (violations=%v)", got)
	}
	if got != nil {
		t.Fatalf("expected nil violations on error, got %v", got)
	}
}

func TestCheckFerpa_RepoError(t *testing.T) {
	sentinel := errors.New("repo down")
	repo := &erroringTagRepo{err: sentinel}
	ev := event("assignment_submission", `{"score":91}`, "")

	_, err := gamification.CheckFerpa(context.Background(), repo, ev)
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected repo error to propagate, got %v", err)
	}
}

// assertViolationsEqual compares two slices of FerpaViolation
// ignoring order (the guard iterates tags in input order, but we
// don't want tests to be brittle if that ever changes) and treating
// the Missing slice as a set.
func assertViolationsEqual(t *testing.T, want, got []gamification.FerpaViolation) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("violation count mismatch: want %d (%v), got %d (%v)", len(want), want, len(got), got)
	}
	normalize := func(vs []gamification.FerpaViolation) []gamification.FerpaViolation {
		out := make([]gamification.FerpaViolation, len(vs))
		copy(out, vs)
		for i := range out {
			m := append([]string(nil), out[i].Missing...)
			sort.Strings(m)
			out[i].Missing = m
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].FieldPath != out[j].FieldPath {
				return out[i].FieldPath < out[j].FieldPath
			}
			return out[i].ObjectType < out[j].ObjectType
		})
		return out
	}
	w := normalize(want)
	g := normalize(got)
	for i := range w {
		if w[i].ObjectType != g[i].ObjectType ||
			w[i].FieldPath != g[i].FieldPath ||
			w[i].Classification != g[i].Classification ||
			!stringSliceEqual(w[i].Missing, g[i].Missing) {
			t.Errorf("violation[%d] mismatch:\n  want %+v\n   got %+v", i, w[i], g[i])
		}
	}
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestDerivePolicyFlags pins the Sprint D-3 derivation contract: when
// any education_record-tagged field is present on an event, the
// required policy_flags (ferpa_protected + education_record) are
// appended in-place. Idempotent. Other classifications don't trigger
// flag additions in Wave 1.
func TestDerivePolicyFlags(t *testing.T) {
	cases := []struct {
		name      string
		tags      []models.GamificationFerpaFieldTag
		event     *models.GamificationEvent
		wantFlags []string
		wantErr   bool
	}{
		{
			name: "education_record field present, no flags → both added",
			tags: []models.GamificationFerpaFieldTag{
				tag("Submission", "result.score", "education_record"),
			},
			event:     event("Submission", `{"score":91}`, ""),
			wantFlags: []string{"ferpa_protected", "education_record"},
		},
		{
			name: "education_record field present, one flag already set → only missing one added",
			tags: []models.GamificationFerpaFieldTag{
				tag("Submission", "result.score", "education_record"),
			},
			event:     event("Submission", `{"score":91}`, "", "education_record"),
			wantFlags: []string{"education_record", "ferpa_protected"},
		},
		{
			name: "education_record field present, both flags already set → no-op (no duplicates)",
			tags: []models.GamificationFerpaFieldTag{
				tag("Submission", "result.score", "education_record"),
			},
			event:     event("Submission", `{"score":91}`, "", "ferpa_protected", "education_record"),
			wantFlags: []string{"ferpa_protected", "education_record"},
		},
		{
			name: "education_record tag exists but field absent on event → no flags added",
			tags: []models.GamificationFerpaFieldTag{
				tag("Submission", "result.score", "education_record"),
			},
			event:     event("Submission", `{"other":1}`, ""),
			wantFlags: []string{},
		},
		{
			name: "only directory_information tags → no flags added (advisory in Wave 1)",
			tags: []models.GamificationFerpaFieldTag{
				tag("Submission", "context.course_id", "directory_information"),
			},
			event:     event("Submission", "", `{"course_id":7}`),
			wantFlags: []string{},
		},
		{
			name:      "no tags for object_type → no flags added",
			tags:      []models.GamificationFerpaFieldTag{},
			event:     event("Submission", `{"score":91}`, ""),
			wantFlags: []string{},
		},
		{
			name: "education_record field in context bucket → flags added",
			tags: []models.GamificationFerpaFieldTag{
				tag("Outcome", "context.mastery_level", "education_record"),
			},
			event:     event("Outcome", "", `{"mastery_level":"proficient"}`),
			wantFlags: []string{"ferpa_protected", "education_record"},
		},
		{
			name: "malformed result JSON → error",
			tags: []models.GamificationFerpaFieldTag{
				tag("Submission", "result.score", "education_record"),
			},
			event:   event("Submission", `not-json`, ""),
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeTagRepo{tags: tc.tags}
			err := gamification.DerivePolicyFlags(context.Background(), repo, tc.event)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			gotSorted := append([]string{}, []string(tc.event.PolicyFlags)...)
			wantSorted := append([]string{}, tc.wantFlags...)
			sort.Strings(gotSorted)
			sort.Strings(wantSorted)
			if !stringSliceEqual(gotSorted, wantSorted) {
				t.Errorf("flags mismatch:\n  want %v\n   got %v", wantSorted, gotSorted)
			}
		})
	}
}

// TestDerivePolicyFlags_NilEvent is a defensive guard. The Emitter's
// nil-check happens before derivation, so this shouldn't be reachable
// in prod — but the function's godoc promises nil-safety.
func TestDerivePolicyFlags_NilEvent(t *testing.T) {
	repo := &fakeTagRepo{tags: []models.GamificationFerpaFieldTag{
		tag("Submission", "result.score", "education_record"),
	}}
	if err := gamification.DerivePolicyFlags(context.Background(), repo, nil); err != nil {
		t.Fatalf("unexpected error for nil event: %v", err)
	}
}

// TestDerivePolicyFlags_RepoError propagates the tag-lookup error
// rather than silently skipping. Silent skip would leave the event
// mis-classified — fail loud instead.
func TestDerivePolicyFlags_RepoError(t *testing.T) {
	repo := &erroringTagRepo{err: errors.New("boom")}
	ev := event("Submission", `{"score":91}`, "")
	if err := gamification.DerivePolicyFlags(context.Background(), repo, ev); err == nil {
		t.Fatalf("expected error, got nil")
	}
}
