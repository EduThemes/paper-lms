package service

// What a student (or their observer) may see of a finished attempt (G043):
// Canvas hide_results, one_time_results, show_correct_answers with its
// show/hide dates and "only after the last attempt", and — for a graded
// quiz — the assignment's post policy (a hidden grade hides the results).

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// OneTimeResultsWindow is how long after the first view a one-time result
// stays open (the review page loads questions and answers in several
// requests, and a reload right away should not lock the student out).
const OneTimeResultsWindow = 15 * time.Minute

// QuizResultsView says which parts of a finished attempt a non-staff
// viewer may see.
type QuizResultsView struct {
	// Score: the attempt's score and kept score.
	Score bool
	// Results: per-question correctness and points.
	Results bool
	// AnswerKey: the correct answers and per-question feedback.
	AnswerKey bool
}

// QuizResultsPolicy is the pure decision. attemptsLeft is the student's
// remaining attempts (-1 unlimited); posted says whether the gradebook
// grade is visible to the student (always true for ungraded quizzes).
func QuizResultsPolicy(quiz *models.Quiz, sub *models.QuizSubmission, attemptsLeft int, posted bool, now time.Time) QuizResultsView {
	var v QuizResultsView
	if quiz == nil || sub == nil || !sub.IsCompleted() || !posted {
		return v
	}
	v.Score = true

	lastAttemptDone := attemptsLeft == 0
	switch quiz.HideResults {
	case "always":
		return v
	case "until_after_last_attempt":
		if !lastAttemptDone {
			return v
		}
	}
	if quiz.OneTimeResults && sub.ResultsSeenAt != nil && now.After(sub.ResultsSeenAt.Add(OneTimeResultsWindow)) {
		return v
	}
	v.Results = true

	if !quiz.ShowCorrectAnswers {
		return v
	}
	if quiz.ShowCorrectAnswersLastAttempt {
		v.AnswerKey = lastAttemptDone
		return v
	}
	if quiz.OneTimeResults {
		// Shown once regardless of the release window (Canvas).
		v.AnswerKey = true
		return v
	}
	if quiz.HideCorrectAnswersAt != nil && now.After(*quiz.HideCorrectAnswersAt) {
		return v
	}
	if quiz.ShowCorrectAnswersAt != nil && !now.After(*quiz.ShowCorrectAnswersAt) {
		return v
	}
	v.AnswerKey = true
	return v
}

// StudentResultsView resolves the policy for a real attempt: the owner's
// remaining attempts (extensions included) and the gradebook post state.
func (s *QuizService) StudentResultsView(ctx context.Context, quiz *models.Quiz, sub *models.QuizSubmission, now time.Time) QuizResultsView {
	if quiz == nil || sub == nil {
		return QuizResultsView{}
	}
	used := sub.Attempt
	ext := models.QuizExtension{}
	posted := true
	if s.lifecycle != nil {
		ext = s.quizExtensionFor(ctx, quiz.ID, sub.UserID)
		if attempts, err := s.userAttempts(ctx, quiz.ID, sub.UserID); err == nil {
			used = 0
			for i := range attempts {
				if attempts[i].IsCompleted() {
					used++
				}
			}
		}
		if s.lifecycle.Gradebook != nil {
			if a := s.linkedAssignment(ctx, quiz); a != nil {
				posted = s.lifecycle.Gradebook.QuizScorePosted(ctx, a, sub.UserID)
			}
		}
	}
	return QuizResultsPolicy(quiz, sub, AttemptsLeft(quiz, ext, used), posted, now)
}

// MarkResultsSeen records the owner's first look at an attempt's results
// (one_time_results).
func (s *QuizService) MarkResultsSeen(ctx context.Context, quiz *models.Quiz, sub *models.QuizSubmission, now time.Time) {
	if quiz == nil || !quiz.OneTimeResults || sub == nil || sub.ResultsSeenAt != nil || !sub.IsCompleted() {
		return
	}
	t := now
	sub.ResultsSeenAt = &t
	// Write only results_seen_at: a full-row Save of the attempt loaded at
	// the start of this request would undo a grader's concurrent manual
	// score or a regrade.
	if m, ok := s.submissionRepo.(quizResultsSeenMarker); ok {
		_ = m.MarkResultsSeen(ctx, sub.ID, t)
		return
	}
	_ = s.submissionRepo.Update(ctx, sub)
}

// quizResultsSeenMarker is implemented by the Postgres repository.
type quizResultsSeenMarker interface {
	MarkResultsSeen(ctx context.Context, id uint, at time.Time) error
}
