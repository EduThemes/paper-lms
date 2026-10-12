package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// The grade summary reads the gradebook engine's drop rules; the rules
// parser accepts the string numbers Canvas forms post.
func TestParseDropRules_SummaryShapes(t *testing.T) {
	r := ParseDropRules(`{"drop_lowest":"2","drop_highest":1,"never_drop":[5,"7"]}`)
	assert.Equal(t, 2, r.DropLowest)
	assert.Equal(t, 1, r.DropHighest)
	assert.Equal(t, []uint{5, 7}, r.NeverDrop)
	assert.Equal(t, DropRules{}, ParseDropRules(`{}`))
	assert.Equal(t, DropRules{}, ParseDropRules(`not json`))
	assert.Equal(t, 0, ParseDropRules(`{"drop_lowest":-3}`).DropLowest)
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
	missing := models.LatePolicyStatusMissing
	assert.True(t, isMissing(a, &future, &models.Submission{LatePolicyStatus: &missing}, now), "teacher-marked missing")
	none := models.LatePolicyStatusNone
	assert.False(t, isMissing(a, &past, &models.Submission{LatePolicyStatus: &none}, now), "teacher cleared the status")
	extended := models.LatePolicyStatusExtended
	assert.False(t, isMissing(a, &past, &models.Submission{LatePolicyStatus: &extended}, now), "extended is not missing")
	grader := uint(9)
	score := 5.0
	assert.False(t, isMissing(a, &past, &models.Submission{GraderID: &grader, Score: &score}, now), "graded by an instructor")
	assert.False(t, isMissing(&models.Assignment{}, &past, nil, now), "no submission types: nothing expected online")
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
