package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/EduThemes/paper-lms/internal/service"
)

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

// ScheduledAnnouncementPublisher is the slice of AnnouncementService the
// delayed-post job needs.
type ScheduledAnnouncementPublisher interface {
	PublishScheduledAnnouncements(ctx context.Context) (int, error)
}

// ImmediateDeliveryProcessor is the slice of NotificationDeliveryService
// the "immediately" delivery job needs.
type ImmediateDeliveryProcessor interface {
	ProcessPendingDeliveries(ctx context.Context) (int, error)
}

// NewMinuteJobs builds the short-cadence jobs run on the per-minute
// scheduler:
//
//   - publishAnnouncements flips delayed (scheduled) announcements whose
//     delayed_post_at has passed to published and notifies their audience.
//   - deliverImmediate sends notification deliveries queued with the
//     "immediately" policy.
//
// Either argument may be nil, in which case the matching job is a no-op.
func NewMinuteJobs(announcements ScheduledAnnouncementPublisher, deliveries ImmediateDeliveryProcessor) (publishAnnouncements, deliverImmediate JobFunc) {
	publishAnnouncements = func(ctx context.Context) error {
		if announcements == nil {
			return nil
		}
		count, err := announcements.PublishScheduledAnnouncements(ctx)
		if err != nil {
			return err
		}
		if count > 0 {
			slog.Info("scheduled announcements published", "count", count)
		}
		return nil
	}
	deliverImmediate = func(ctx context.Context) error {
		if deliveries == nil {
			return nil
		}
		count, err := deliveries.ProcessPendingDeliveries(ctx)
		if err != nil {
			return err
		}
		if count > 0 {
			slog.Info("immediate notifications delivered", "count", count)
		}
		return nil
	}
	return
}
