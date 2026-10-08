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
