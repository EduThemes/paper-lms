package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

func finished(attempt int, score float64) models.QuizSubmission {
	s := score
	return models.QuizSubmission{Attempt: attempt, Score: &s, WorkflowState: models.QuizSubmissionComplete}
}

// G044
func TestKeptScore_Policies(t *testing.T) {
	attempts := []models.QuizSubmission{finished(1, 8), finished(2, 4), finished(3, 6),
		{Attempt: 4, WorkflowState: models.QuizSubmissionUntaken}}
	cases := map[string]float64{
		models.QuizScoringKeepHighest: 8,
		"":                            8,
		models.QuizScoringKeepLatest:  6,
		models.QuizScoringKeepAverage: 6,
	}
	for policy, want := range cases {
		got, ok := KeptScore(policy, attempts)
		assert.True(t, ok, policy)
		assert.InDelta(t, want, got, 1e-9, policy)
	}
	_, ok := KeptScore(models.QuizScoringKeepHighest, []models.QuizSubmission{{Attempt: 1, WorkflowState: models.QuizSubmissionUntaken}})
	assert.False(t, ok, "no finished attempt, no kept score")
}

// G047
func TestAttemptsAllowedAndLeft(t *testing.T) {
	q := &models.Quiz{AllowedAttempts: 2}
	assert.Equal(t, 3, AttemptsAllowedFor(q, models.QuizExtension{ExtraAttempts: 1}))
	assert.Equal(t, 1, AttemptsLeft(q, models.QuizExtension{ExtraAttempts: 1}, 2))
	assert.Equal(t, 0, AttemptsLeft(q, models.QuizExtension{}, 5))
	assert.Equal(t, -1, AttemptsLeft(&models.Quiz{AllowedAttempts: -1}, models.QuizExtension{}, 9))
}

// G043
func TestQuizResultsPolicy(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	done := &models.QuizSubmission{Attempt: 1, WorkflowState: models.QuizSubmissionComplete}
	base := func() *models.Quiz { return &models.Quiz{ShowCorrectAnswers: true, AllowedAttempts: 2} }

	v := QuizResultsPolicy(base(), done, 1, true, now)
	assert.Equal(t, QuizResultsView{Score: true, Results: true, AnswerKey: true}, v)

	// In progress: nothing.
	assert.Equal(t, QuizResultsView{}, QuizResultsPolicy(base(), &models.QuizSubmission{WorkflowState: "untaken"}, 1, true, now))
	// Grade not posted: nothing.
	assert.Equal(t, QuizResultsView{}, QuizResultsPolicy(base(), done, 1, false, now))

	q := base()
	q.HideResults = "always"
	assert.Equal(t, QuizResultsView{Score: true}, QuizResultsPolicy(q, done, 0, true, now))

	q = base()
	q.HideResults = "until_after_last_attempt"
	assert.False(t, QuizResultsPolicy(q, done, 1, true, now).Results)
	assert.True(t, QuizResultsPolicy(q, done, 0, true, now).Results)

	q = base()
	q.ShowCorrectAnswers = false
	assert.Equal(t, QuizResultsView{Score: true, Results: true}, QuizResultsPolicy(q, done, 1, true, now))

	q = base()
	q.ShowCorrectAnswersLastAttempt = true
	assert.False(t, QuizResultsPolicy(q, done, 1, true, now).AnswerKey)
	assert.True(t, QuizResultsPolicy(q, done, 0, true, now).AnswerKey)

	later := now.Add(time.Hour)
	earlier := now.Add(-time.Hour)
	q = base()
	q.ShowCorrectAnswersAt = &later
	assert.False(t, QuizResultsPolicy(q, done, 1, true, now).AnswerKey, "not released yet")
	q = base()
	q.HideCorrectAnswersAt = &earlier
	assert.False(t, QuizResultsPolicy(q, done, 1, true, now).AnswerKey, "release window closed")
	q = base()
	q.ShowCorrectAnswersAt, q.HideCorrectAnswersAt = &earlier, &later
	assert.True(t, QuizResultsPolicy(q, done, 1, true, now).AnswerKey)

	// One-time results: open for a short window after the first view.
	q = base()
	q.OneTimeResults = true
	q.ShowCorrectAnswersAt = &later // ignored for one-time results (Canvas)
	seen := now.Add(-5 * time.Minute)
	viewed := &models.QuizSubmission{Attempt: 1, WorkflowState: models.QuizSubmissionComplete, ResultsSeenAt: &seen}
	assert.True(t, QuizResultsPolicy(q, viewed, 1, true, now).AnswerKey)
	longAgo := now.Add(-2 * OneTimeResultsWindow)
	viewed.ResultsSeenAt = &longAgo
	assert.Equal(t, QuizResultsView{Score: true}, QuizResultsPolicy(q, viewed, 1, true, now))
}
