package gamification

import "github.com/EduThemes/paper-lms/internal/service/gamification/pseudonym"

// ViewerRole captures who is looking at the leaderboard. Only these
// three categories matter for render policy — privileged authors
// (admin / teacher) always see truthful data; learners see whatever
// the tenant mode dictates.
type ViewerRole string

const (
	ViewerAdmin   ViewerRole = "admin"
	ViewerTeacher ViewerRole = "teacher" // TeacherEnrollment or TaEnrollment in this course
	ViewerStudent ViewerRole = "student"
)

// DefaultTopNSize is the W3-B-locked top-N row count (user decision
// 2026-05-14). Lifted to a package constant so changing the value is
// one edit, not three (the previous in-line literal recurred in every
// switch arm of RenderPolicyFor).
const DefaultTopNSize = 5

// LeaderboardRenderPolicy decides what a single viewer sees on a
// single course leaderboard at a single moment. Composed entirely
// server-side; the frontend renders whatever bytes the response
// carries.
//
// Tenant-mode behavior (W3-B requirements, locked with user 2026-05-14):
//
//	┌────────────────┬────────────┬───────────┬─────────────┬──────────┐
//	│ tenant_mode    │ Pseudonyms │ Top-N      │ Switch pool │ First-name│
//	├────────────────┼────────────┼───────────┼─────────────┼──────────┤
//	│ k5, m68        │ Always     │ Never (any │ No (teacher │ No       │
//	│                │            │  viewer)   │  controlled)│          │
//	│ h912           │ Default on │ Top 5 see  │ Yes         │ Yes      │
//	│                │            │  top 5     │             │          │
//	│ higher_ed,     │ Off (real  │ Top 5 see  │ N/A         │ N/A      │
//	│ corp, pro      │  names)    │  top 5     │             │          │
//	└────────────────┴────────────┴───────────┴─────────────┴──────────┘
//
// Admins + course teachers/TAs always see real names and the full list,
// regardless of tenant mode. They are FERPA-cleared to view roster data
// for their own course.
type LeaderboardRenderPolicy struct {
	// UsePseudonyms: when true, the response substitutes
	// `enrollments.pseudonym_name` for `users.name` on every row
	// except the viewer's own (the viewer always sees their own legal
	// name so they know who they are on the board).
	UsePseudonyms bool

	// DefaultPoolCode: which pool to use for any enrollment whose
	// pseudonym hasn't been assigned yet (lazy fill on first read).
	DefaultPoolCode pseudonym.PoolCode

	// ShowTopN: when false, the response is the relative window only
	// (W3-C); no rows beyond the viewer's ±N neighborhood. When true,
	// the response contains the top N rows AND the viewer is one of
	// them.
	ShowTopN bool

	// TopNSize: how many "top" rows count. Always 5 in W3-B (user
	// decision 2026-05-14).
	TopNSize int

	// LearnerCanSwitch: allowed to change pseudonym pool / regenerate
	// a fresh name via PUT /enrollments/self/pseudonym.
	LearnerCanSwitch bool

	// AllowFirstName: pool "first_name" is permitted as a target for
	// the switch endpoint.
	AllowFirstName bool
}

// RenderPolicyFor resolves the render policy for one (tenantMode, role,
// viewerRank, totalCandidates) tuple. `viewerRank` is 1-indexed; pass
// 0 if unknown (the function then treats the viewer as mid-pack).
func RenderPolicyFor(tenantMode string, role ViewerRole, viewerRank int) LeaderboardRenderPolicy {
	// Admins + teachers always get the truthful view.
	if role == ViewerAdmin || role == ViewerTeacher {
		return LeaderboardRenderPolicy{
			UsePseudonyms:    false,
			DefaultPoolCode:  pseudonym.PoolAnimals,
			ShowTopN:         true,
			TopNSize:         DefaultTopNSize,
			LearnerCanSwitch: false,
			AllowFirstName:   false,
		}
	}

	// Student paths split by tenant mode.
	switch tenantMode {
	case "k5", "m68":
		return LeaderboardRenderPolicy{
			UsePseudonyms:    true,
			DefaultPoolCode:  pseudonym.PoolAnimals,
			ShowTopN:         false, // no top-N visible to any student under M68
			TopNSize:         DefaultTopNSize,
			LearnerCanSwitch: false, // teacher-controlled at this age
			AllowFirstName:   false,
		}
	case "h912":
		viewerInTopN := viewerRank > 0 && viewerRank <= DefaultTopNSize
		return LeaderboardRenderPolicy{
			UsePseudonyms:    true,
			DefaultPoolCode:  pseudonym.PoolAnimals,
			ShowTopN:         viewerInTopN, // top 5 see top 5; everyone else gets relative window
			TopNSize:         DefaultTopNSize,
			LearnerCanSwitch: true,
			AllowFirstName:   true,
		}
	default: // higher_ed, corp, pro, or any unknown tenant mode
		viewerInTopN := viewerRank > 0 && viewerRank <= DefaultTopNSize
		return LeaderboardRenderPolicy{
			UsePseudonyms:    false, // real names by default
			DefaultPoolCode:  pseudonym.PoolAnimals,
			ShowTopN:         viewerInTopN,
			TopNSize:         DefaultTopNSize,
			LearnerCanSwitch: false,
			AllowFirstName:   false,
		}
	}
}
