package scheduler

import (
	"context"
	"log/slog"

	"github.com/EduThemes/paper-lms/internal/service"
)

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
