package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

func mpU(v uint) *uint           { return &v }
func mpF(v float64) *float64     { return &v }
func mpT(v time.Time) *time.Time { return &v }

func evalItem(id uint, pos int, ct, req string, minScore *float64) models.ContentTag {
	return models.ContentTag{ID: id, Position: pos, ContentType: ct, ContentID: mpU(id * 10), CompletionType: req, MinScore: minScore, WorkflowState: models.ContentTagActive}
}

func byModule(t *testing.T, out []ModuleProgress, id uint) ModuleProgress {
	t.Helper()
	for _, m := range out {
		if m.ModuleID == id {
			return m
		}
	}
	t.Fatalf("module %d not evaluated", id)
	return ModuleProgress{}
}

func TestEvaluate_NoRequirementsIsCompleted(t *testing.T) {
	out := EvaluateModuleProgress(ModuleEvalInput{Modules: []models.ContextModule{
		{ID: 1, Name: "Intro", Items: []models.ContentTag{evalItem(1, 1, "WikiPage", "", nil)}},
	}})
	m := byModule(t, out, 1)
	assert.Equal(t, models.ModuleProgressionCompleted, m.State)
	assert.False(t, m.Items[0].Locked)
}

func TestEvaluate_UnlockAtInFutureLocksEverything(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	out := EvaluateModuleProgress(ModuleEvalInput{Now: now, Modules: []models.ContextModule{
		{ID: 1, Name: "Week 2", UnlockAt: mpT(now.Add(48 * time.Hour)), Items: []models.ContentTag{evalItem(1, 1, "WikiPage", "must_view", nil)}},
	}, Facts: map[uint]ItemFacts{1: {Viewed: true}}})
	m := byModule(t, out, 1)
	assert.Equal(t, models.ModuleProgressionLocked, m.State)
	assert.True(t, m.LockedUntilDate)
	assert.True(t, m.Items[0].Locked)
	assert.False(t, m.Items[0].Completed, "a locked module records no progress")
	assert.Contains(t, m.LockExplanation(func(uint) string { return "" }), "will unlock")
	assert.Contains(t, m.LockExplanation(func(uint) string { return "" }), "Sep 3, 2026 at 12:00 PM UTC")

	// G157: the date reads in the student's profile zone when known.
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("tzdata unavailable")
	}
	m.Location = ny
	assert.Contains(t, m.LockExplanation(func(uint) string { return "" }), "Sep 3, 2026 at 8:00 AM EDT")

	// Once the date passes the module opens.
	out = EvaluateModuleProgress(ModuleEvalInput{Now: now.Add(72 * time.Hour), Modules: []models.ContextModule{
		{ID: 1, Name: "Week 2", UnlockAt: mpT(now.Add(48 * time.Hour)), Items: []models.ContentTag{evalItem(1, 1, "WikiPage", "must_view", nil)}},
	}, Facts: map[uint]ItemFacts{1: {Viewed: true}}})
	assert.Equal(t, models.ModuleProgressionCompleted, byModule(t, out, 1).State)
}

func TestEvaluate_PrerequisitesGateTheNextModule(t *testing.T) {
	mods := func() []models.ContextModule {
		return []models.ContextModule{
			{ID: 1, Name: "Week 1", Position: 1, Items: []models.ContentTag{
				evalItem(1, 1, "WikiPage", "must_view", nil),
				evalItem(2, 2, "Assignment", "must_submit", nil),
			}},
			{ID: 2, Name: "Week 2", Position: 2, Items: []models.ContentTag{evalItem(3, 1, "WikiPage", "must_view", nil)}},
		}
	}
	prereqs := map[uint][]uint{2: {1}}

	out := EvaluateModuleProgress(ModuleEvalInput{Modules: mods(), Prerequisites: prereqs, Facts: map[uint]ItemFacts{1: {Viewed: true}}})
	w1, w2 := byModule(t, out, 1), byModule(t, out, 2)
	assert.Equal(t, models.ModuleProgressionStarted, w1.State)
	assert.Equal(t, []uint{1}, w1.RequirementsMet)
	assert.Equal(t, models.ModuleProgressionLocked, w2.State)
	assert.Equal(t, []uint{1}, w2.PrerequisitesUnmet)
	assert.Equal(t, `Complete "Week 1" to unlock this module.`, w2.LockExplanation(func(id uint) string { return map[uint]string{1: "Week 1"}[id] }))

	out = EvaluateModuleProgress(ModuleEvalInput{Modules: mods(), Prerequisites: prereqs, Facts: map[uint]ItemFacts{1: {Viewed: true}, 2: {Submitted: true}}})
	assert.Equal(t, models.ModuleProgressionCompleted, byModule(t, out, 1).State)
	assert.Equal(t, models.ModuleProgressionUnlocked, byModule(t, out, 2).State)
}

func TestEvaluate_PrerequisiteOnUnknownModuleIsIgnored(t *testing.T) {
	out := EvaluateModuleProgress(ModuleEvalInput{
		Modules:       []models.ContextModule{{ID: 2, Name: "Week 2"}},
		Prerequisites: map[uint][]uint{2: {99}}, // deleted / unpublished module
	})
	assert.Equal(t, models.ModuleProgressionCompleted, byModule(t, out, 2).State)
}

func TestEvaluate_PrerequisiteCycleLocksBoth(t *testing.T) {
	out := EvaluateModuleProgress(ModuleEvalInput{
		Modules:       []models.ContextModule{{ID: 1, Name: "A"}, {ID: 2, Name: "B"}},
		Prerequisites: map[uint][]uint{1: {2}, 2: {1}},
	})
	assert.Equal(t, models.ModuleProgressionLocked, byModule(t, out, 1).State)
	assert.Equal(t, models.ModuleProgressionLocked, byModule(t, out, 2).State)
}

func TestEvaluate_RequirementCountOne(t *testing.T) {
	out := EvaluateModuleProgress(ModuleEvalInput{Modules: []models.ContextModule{
		{ID: 1, Name: "Choice", RequirementCount: 1, Items: []models.ContentTag{
			evalItem(1, 1, "WikiPage", "must_view", nil),
			evalItem(2, 2, "WikiPage", "must_view", nil),
		}},
	}, Facts: map[uint]ItemFacts{2: {Viewed: true}}})
	assert.Equal(t, models.ModuleProgressionCompleted, byModule(t, out, 1).State)
}

func TestEvaluate_SequentialProgressLocksLaterItems(t *testing.T) {
	mod := models.ContextModule{ID: 1, Name: "Seq", RequireSequentialProgress: true, Items: []models.ContentTag{
		evalItem(1, 1, "WikiPage", "must_view", nil),
		evalItem(2, 2, "ContextModuleSubHeader", "", nil),
		evalItem(3, 3, "Assignment", "min_score", mpF(7)),
		evalItem(4, 4, "WikiPage", "", nil),
	}}
	out := EvaluateModuleProgress(ModuleEvalInput{Modules: []models.ContextModule{mod}, Facts: map[uint]ItemFacts{1: {Viewed: true}, 3: {Score: mpF(5)}}})
	m := byModule(t, out, 1)
	require.Len(t, m.Items, 4)
	assert.False(t, m.Items[0].Locked)
	assert.False(t, m.Items[1].Locked)
	assert.False(t, m.Items[2].Locked, "the current item stays open")
	assert.True(t, m.Items[3].Locked, "items after the first unmet requirement are locked")
	require.NotNil(t, m.CurrentPosition)
	assert.Equal(t, 3, *m.CurrentPosition)
	assert.Equal(t, mpF(5), m.Items[2].Score)

	// A score within float noise of the bar meets it.
	out = EvaluateModuleProgress(ModuleEvalInput{Modules: []models.ContextModule{mod}, Facts: map[uint]ItemFacts{1: {Viewed: true}, 3: {Score: mpF(6.99999999)}}})
	m = byModule(t, out, 1)
	assert.Equal(t, models.ModuleProgressionCompleted, m.State)
	assert.Nil(t, m.CurrentPosition)
	assert.False(t, m.Items[3].Locked)
}

func TestRequirementMet(t *testing.T) {
	assign := evalItem(1, 1, "Assignment", "must_view", nil)
	assert.True(t, RequirementMet(assign, ItemFacts{Submitted: true}), "submitting implies viewing")
	assert.False(t, RequirementMet(assign, ItemFacts{}))

	score := evalItem(1, 1, "Quizzes::Quiz", "min_score", mpF(8))
	assert.False(t, RequirementMet(score, ItemFacts{Score: mpF(7.5)}))
	assert.True(t, RequirementMet(score, ItemFacts{Score: mpF(8)}))
	assert.True(t, RequirementMet(score, ItemFacts{Excused: true}), "excused work meets any requirement")

	page := evalItem(1, 1, "WikiPage", "must_mark_done", nil)
	assert.False(t, RequirementMet(page, ItemFacts{Viewed: true}))
	assert.True(t, RequirementMet(page, ItemFacts{Done: true}))
	assert.False(t, RequirementMet(page, ItemFacts{Excused: true}), "excused does not apply to unscored content")

	disc := evalItem(1, 1, "DiscussionTopic", "must_contribute", nil)
	assert.True(t, RequirementMet(disc, ItemFacts{Contributed: true}))
	assert.False(t, RequirementMet(evalItem(1, 1, "WikiPage", "", nil), ItemFacts{Viewed: true}))
}

func TestPaceSchedule(t *testing.T) {
	// Friday 2026-09-04.
	start := time.Date(2026, 9, 4, 9, 0, 0, 0, time.UTC)

	dates := PaceSchedule(start, []int{1, 2}, true, nil, nil)
	assert.Equal(t, "2026-09-07", dates[0].Format("2006-01-02"), "weekend skipped")
	assert.Equal(t, "2026-09-09", dates[1].Format("2006-01-02"))

	blackout := []models.BlackoutDate{{StartDate: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)}}
	dates = PaceSchedule(start, []int{1, 2}, true, blackout, nil)
	assert.Equal(t, "2026-09-09", dates[0].Format("2006-01-02"), "blackout days skipped")
	assert.Equal(t, "2026-09-11", dates[1].Format("2006-01-02"))

	// Hard end date: 10 days of work squeezed into the 5 days before the end.
	end := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	dates = PaceSchedule(start, []int{5, 5}, true, nil, &end)
	assert.Equal(t, "2026-09-08", dates[0].Format("2006-01-02"))
	assert.Equal(t, "2026-09-11", dates[1].Format("2006-01-02"), "last item lands on the hard end date")

	// A plan that fits is not stretched.
	dates = PaceSchedule(start, []int{1}, true, nil, &end)
	assert.Equal(t, "2026-09-07", dates[0].Format("2006-01-02"))
}

func TestValidateToolLaunchURL(t *testing.T) {
	tool := &models.ContextExternalTool{URL: "https://tool.example.org/launch", Domain: "example.org"}
	u, err := ValidateToolLaunchURL(tool, "")
	require.NoError(t, err)
	assert.Equal(t, "https://tool.example.org/launch", u)

	u, err = ValidateToolLaunchURL(tool, "https://deep.example.org/unit/3")
	require.NoError(t, err)
	assert.Equal(t, "https://deep.example.org/unit/3", u)

	_, err = ValidateToolLaunchURL(tool, "https://evil.example.com/x")
	assert.Error(t, err)
	_, err = ValidateToolLaunchURL(tool, "javascript:alert(1)")
	assert.Error(t, err)

	noDomain := &models.ContextExternalTool{URL: "https://lti.vendor.test/start"}
	_, err = ValidateToolLaunchURL(noDomain, "https://lti.vendor.test/other")
	assert.NoError(t, err)
	_, err = ValidateToolLaunchURL(noDomain, "https://other.test/")
	assert.Error(t, err)
}
