package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

func TestParseGroupRules(t *testing.T) {
	r := ParseGroupRules(`{"drop_lowest":"2","drop_highest":1,"never_drop":[5,"7"]}`)
	assert.Equal(t, 2, r.DropLowest)
	assert.Equal(t, 1, r.DropHighest)
	assert.Equal(t, []uint{5, 7}, r.NeverDrop)
	assert.Equal(t, GroupRules{}, ParseGroupRules(`{}`))
	assert.Equal(t, GroupRules{}, ParseGroupRules(`not json`))
	assert.Equal(t, 0, ParseGroupRules(`{"drop_lowest":-3}`).DropLowest)
}

func keptIDs(items []GradedItem) []uint {
	out := []uint{}
	for _, it := range items {
		out = append(out, it.AssignmentID)
	}
	return out
}

// With unequal points possible, dropping the lowest *percentage* is not
// what helps most: Canvas keeps the subset with the best total ratio.
func TestApplyDropRules_UnequalPointsMaximizesGrade(t *testing.T) {
	items := []GradedItem{
		{AssignmentID: 1, Score: 50, Possible: 100}, // 50%
		{AssignmentID: 2, Score: 0, Possible: 5},    // 0%, but tiny
		{AssignmentID: 3, Score: 95, Possible: 100}, // 95%
	}
	kept, dropped := ApplyDropRules(items, GroupRules{DropLowest: 1})
	// Keeping {2,3}: 95/105 = 90.5%; keeping {1,3}: 145/200 = 72.5%.
	assert.ElementsMatch(t, []uint{2, 3}, keptIDs(kept))
	assert.True(t, dropped[1])
}

func TestApplyDropRules_NeverDropHighestAndFloor(t *testing.T) {
	items := []GradedItem{
		{AssignmentID: 1, Score: 2, Possible: 10},
		{AssignmentID: 2, Score: 9, Possible: 10},
		{AssignmentID: 3, Score: 10, Possible: 10},
	}
	kept, dropped := ApplyDropRules(items, GroupRules{DropLowest: 1, NeverDrop: []uint{1}})
	assert.ElementsMatch(t, []uint{1, 3}, keptIDs(kept), "never_drop protects the lowest")
	assert.True(t, dropped[2])

	kept, dropped = ApplyDropRules(items, GroupRules{DropHighest: 1})
	assert.ElementsMatch(t, []uint{1, 2}, keptIDs(kept))
	assert.True(t, dropped[3])

	// Dropping more than exists still keeps one item.
	kept, _ = ApplyDropRules(items, GroupRules{DropLowest: 10})
	assert.Equal(t, []uint{3}, keptIDs(kept))

	// Extra credit (0 possible) is never dropped.
	withEC := append(items, GradedItem{AssignmentID: 4, Score: 3, Possible: 0})
	kept, _ = ApplyDropRules(withEC, GroupRules{DropLowest: 2})
	assert.Contains(t, keptIDs(kept), uint(4))
}

func TestEffectiveDueAt(t *testing.T) {
	base := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	later := base.Add(48 * time.Hour)
	earlier := base.Add(-48 * time.Hour)
	assert.Equal(t, &base, effectiveDueAt(&base, nil))
	assert.Equal(t, later, *effectiveDueAt(&base, []*time.Time{&earlier, &later}), "most lenient override wins")
	assert.Nil(t, effectiveDueAt(&base, []*time.Time{&earlier, nil}), "an override without a due date means none")
}

func TestIsMissing(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)
	a := &models.Assignment{SubmissionTypes: "online_upload"}
	assert.True(t, isMissing(a, &past, nil, now))
	assert.False(t, isMissing(a, &future, nil, now))
	assert.False(t, isMissing(a, &past, &models.Submission{SubmittedAt: &past}, now))
	assert.False(t, isMissing(a, &past, &models.Submission{Excused: true}, now))
	assert.True(t, isMissing(a, &future, &models.Submission{Missing: true}, now), "teacher-marked missing")
	assert.False(t, isMissing(&models.Assignment{SubmissionTypes: "on_paper"}, &past, nil, now))
	assert.False(t, isMissing(&models.Assignment{GradingType: "not_graded"}, &past, nil, now))
}

func TestNormalizeHexColor(t *testing.T) {
	c, err := NormalizeHexColor("AbCdEf")
	assert.NoError(t, err)
	assert.Equal(t, "#abcdef", c)
	_, err = NormalizeHexColor("#abc")
	assert.ErrorIs(t, err, ErrInvalidColor)
	_, err = NormalizeHexColor("red")
	assert.ErrorIs(t, err, ErrInvalidColor)
}

func TestObserverAlertTransitions(t *testing.T) {
	assert.True(t, models.ObserverAlertUnread.CanTransitionTo(models.ObserverAlertRead))
	assert.True(t, models.ObserverAlertRead.CanTransitionTo(models.ObserverAlertDismissed))
	assert.False(t, models.ObserverAlertDismissed.CanTransitionTo(models.ObserverAlertRead))
	assert.False(t, models.ObserverAlertRead.CanTransitionTo(models.ObserverAlertUnread))
}

func TestNormalizeProfileURL(t *testing.T) {
	u, err := normalizeProfileURL("example.test/me")
	assert.NoError(t, err)
	assert.Equal(t, "https://example.test/me", u)
	for _, bad := range []string{"javascript:alert(1)", "data:text/html,x", "ftp://example.test", ""} {
		_, err := normalizeProfileURL(bad)
		assert.Error(t, err, bad)
	}
}
