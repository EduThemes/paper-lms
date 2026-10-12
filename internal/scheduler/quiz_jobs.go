package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// QuizAutoSubmitter is the slice of QuizService the overdue-attempt job
// needs.
type QuizAutoSubmitter interface {
	AutoSubmitOverdueAttempts(ctx context.Context, now time.Time) (int, error)
}

// NewQuizAutoSubmitJob turns in quiz attempts whose time limit (or lock
// date) ran out while the student was away (G048): the client timer only
// submits while the tab is open. Runs on the per-minute scheduler.
func NewQuizAutoSubmitJob(svc QuizAutoSubmitter) JobFunc {
	return func(ctx context.Context) error {
		if svc == nil {
			return nil
		}
		n, err := svc.AutoSubmitOverdueAttempts(ctx, time.Now())
		if n > 0 {
			slog.Info("overdue quiz attempts auto-submitted", "count", n)
		}
		return err
	}
}

// QuizAssignmentLinker is the slice of QuizService the gradebook-link job
// needs.
type QuizAssignmentLinker interface {
	LinkGradedQuizzes(ctx context.Context) (int, error)
}

// NewQuizAssignmentLinkJob gives graded quizzes that predate the
// gradebook link (or were copied / imported without one) their
// online_quiz assignment (G041). Runs on the hourly scheduler.
func NewQuizAssignmentLinkJob(svc QuizAssignmentLinker) JobFunc {
	return func(ctx context.Context) error {
		if svc == nil {
			return nil
		}
		n, err := svc.LinkGradedQuizzes(ctx)
		if n > 0 {
			slog.Info("graded quizzes linked to the gradebook", "count", n)
		}
		return err
	}
}
