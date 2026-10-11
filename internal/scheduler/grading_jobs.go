package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// MissingPolicyApplier is the slice of GradingPolicyService the
// missing-submission job needs.
type MissingPolicyApplier interface {
	ApplyMissingPolicy(ctx context.Context, lookback time.Duration) (int, error)
}

// MissingPolicyLookback is how far back the job re-checks due dates. It
// covers several missed ticks so a restart never skips an assignment.
const MissingPolicyLookback = 72 * time.Hour

// NewMissingPolicyJob marks past-due unsubmitted work missing and applies
// each course's missing-submission deduction (G022). A nil applier makes
// the job a no-op.
func NewMissingPolicyJob(applier MissingPolicyApplier) JobFunc {
	return func(ctx context.Context) error {
		if applier == nil {
			return nil
		}
		n, err := applier.ApplyMissingPolicy(ctx, MissingPolicyLookback)
		if n > 0 {
			slog.Info("missing submission policy applied", "updated", n)
		}
		return err
	}
}
