package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/EduThemes/paper-lms/internal/service"
)

// EveryTick is a predicate that fires on every scheduler tick (hourly in
// production). Jobs using it decide for themselves what is due.
func EveryTick() PredicateFunc {
	return func(time.Time) bool { return true }
}

// NewOneRosterAutoSyncJob runs the full sync of every auto_sync OneRoster
// connection whose AutoSyncInterval has elapsed (B173).
func NewOneRosterAutoSyncJob(onerosterService *service.OneRosterService) JobFunc {
	return func(ctx context.Context) error {
		ran, err := onerosterService.RunDueAutoSyncs(ctx, time.Now())
		if ran > 0 {
			slog.Info("oneroster auto-sync processed", "connections", ran)
		}
		return err
	}
}

// NewDigestJobs builds the weekly + daily digest job functions backed by the
// notification delivery service. The service's ProcessDigests returns the
// number of digests sent along with an error; we discard the count here and
// surface only the error to the scheduler (the count is logged for ops).
func NewDigestJobs(notificationDeliveryService *service.NotificationDeliveryService) (weekly, daily JobFunc) {
	weekly = func(ctx context.Context) error {
		count, err := notificationDeliveryService.ProcessDigests(ctx, "weekly")
		if err != nil {
			return err
		}
		slog.Info("weekly digest processed", "count", count)
		return nil
	}
	daily = func(ctx context.Context) error {
		count, err := notificationDeliveryService.ProcessDigests(ctx, "daily")
		if err != nil {
			return err
		}
		slog.Info("daily digest processed", "count", count)
		return nil
	}
	return
}
