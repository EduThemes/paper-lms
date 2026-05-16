package gamification

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/datatypes"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// LeaderboardSnapshotService owns the snapshot-time composition: it
// walks an active scope's candidate set, applies the W2-C opt-out
// filter, ranks, and writes the result as a single JSONB row.
//
// Sprint 7-B intent (per behavioral research:277): weekly windows give
// every learner a fresh chance. Live compute can't easily express
// "show me last week's standings" without re-walking the wallet
// ledger and re-applying the opt-out set as it existed at window close.
// The snapshot is the canonical artifact for that read.
//
// FERPA contract (audit decision): we filter at BOTH snapshot-write
// time (so a learner who opted out before window-close never enters
// the payload) AND snapshot-read time at the handler (so a learner
// who opts out *after* the snapshot vanishes from peer views without
// a snapshot rewrite). This service handles the write-time half.
type LeaderboardSnapshotService struct {
	enrollments repository.EnrollmentRepository
	users       repository.UserRepository
	wallets     repository.GamificationWalletRepository
	snapshots   repository.GamificationLeaderboardSnapshotRepository
}

func NewLeaderboardSnapshotService(
	enrollments repository.EnrollmentRepository,
	users repository.UserRepository,
	wallets repository.GamificationWalletRepository,
	snapshots repository.GamificationLeaderboardSnapshotRepository,
) *LeaderboardSnapshotService {
	return &LeaderboardSnapshotService{
		enrollments: enrollments,
		users:       users,
		wallets:     wallets,
		snapshots:   snapshots,
	}
}

// ComputeCourseWeekly composes and stores one weekly snapshot for one
// (course, currency) pair. Idempotent: a second call with the same
// windowEnd is a no-op (returns created=false, no error).
//
// The window is (windowEnd - 7 days, windowEnd]. v1 doesn't actually
// scope the ranking BY the window — `lifetime_earned` is the ranking
// column, monotonic across windows. The window is just the "as-of"
// timestamp the snapshot is anchored to. A future "earned-this-week"
// ranking would re-shape this signature; v1 ships the simpler
// all-time-as-of-window mechanic per the W3-C plan.
func (s *LeaderboardSnapshotService) ComputeCourseWeekly(
	ctx context.Context,
	courseID, currencyTypeID uint,
	windowEnd time.Time,
) (created bool, err error) {
	if windowEnd.IsZero() {
		return false, fmt.Errorf("windowEnd must be set")
	}

	// Candidate set: active StudentEnrollments for this course.
	candidates, err := s.enrollments.ListActiveStudentUserIDsByCourse(ctx, courseID)
	if err != nil {
		return false, fmt.Errorf("list enrollments: %w", err)
	}
	if len(candidates) == 0 {
		// Empty cohort: no snapshot written. Distinguish from
		// "snapshot already exists" via created=false + nil err.
		return false, nil
	}

	// Opt-out filter at write time (W2-C).
	visible, err := s.users.FilterPublicLeaderboardCandidates(ctx, candidates)
	if err != nil {
		return false, fmt.Errorf("filter opt-out: %w", err)
	}
	if len(visible) == 0 {
		return false, nil
	}

	// Rank.
	ranked, err := s.wallets.RankByCurrency(ctx, currencyTypeID, visible)
	if err != nil {
		return false, fmt.Errorf("rank: %w", err)
	}

	// Encode payload.
	payload := make([]models.SnapshotRow, 0, len(ranked))
	for _, r := range ranked {
		payload = append(payload, models.SnapshotRow{
			UserID:         r.UserID,
			Rank:           r.Rank,
			LifetimeEarned: r.LifetimeEarned,
		})
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Errorf("marshal payload: %w", err)
	}

	snap := &models.GamificationLeaderboardSnapshot{
		ScopeType:      models.ScopeCourse,
		ScopeID:        courseID,
		CurrencyTypeID: currencyTypeID,
		WindowKind:     string(models.WindowKindWeekly),
		WindowStart:    windowEnd.Add(-7 * 24 * time.Hour),
		WindowEnd:      windowEnd,
		Payload:        datatypes.JSON(payloadJSON),
	}
	return s.snapshots.Upsert(ctx, snap)
}

// ReadCourseSnapshot returns the snapshot for (course, currency, kind,
// windowEnd). nil + nil means "no snapshot for that window" — the
// handler returns 404 in that case rather than synthesizing one.
//
// Opt-out filtering at READ time is the handler's job (see
// GetCourseLeaderboard); this method just hands back the stored payload.
func (s *LeaderboardSnapshotService) ReadCourseSnapshot(
	ctx context.Context,
	courseID, currencyTypeID uint,
	kind models.SnapshotWindowKind,
	windowEnd time.Time,
) (*models.GamificationLeaderboardSnapshot, []models.SnapshotRow, error) {
	row, err := s.snapshots.FindByWindow(ctx, models.ScopeCourse, courseID, currencyTypeID, string(kind), windowEnd)
	if err != nil {
		return nil, nil, err
	}
	if row == nil {
		return nil, nil, nil
	}
	var payload []models.SnapshotRow
	if err := json.Unmarshal([]byte(row.Payload), &payload); err != nil {
		return nil, nil, fmt.Errorf("unmarshal snapshot payload: %w", err)
	}
	return row, payload, nil
}

// MostRecentClosedWeekly returns the canonical "previous closed
// weekly window-end" derived from `now`. Convention (locked 2026-05-14):
// Sunday 00:00 UTC → Sunday 00:00 UTC. So at any moment, the most
// recent CLOSED window ended at the most recent Sunday 00:00 UTC
// strictly before `now`.
//
// Example: now = 2026-05-14T12:00:00Z (Thursday). Most recent
// previous Sunday is 2026-05-10T00:00:00Z.
func MostRecentClosedWeekly(now time.Time) time.Time {
	utcNow := now.UTC()
	dayOffset := int(utcNow.Weekday()) // Sunday=0..Saturday=6
	// Truncate to today 00:00 UTC.
	todayMidnight := time.Date(utcNow.Year(), utcNow.Month(), utcNow.Day(), 0, 0, 0, 0, time.UTC)
	switch {
	case dayOffset == 0 && utcNow.Equal(todayMidnight):
		// Exactly Sunday 00:00 — the current window just opened, so
		// the most recent CLOSED window ended on the prior Sunday.
		return todayMidnight.AddDate(0, 0, -7)
	case dayOffset == 0:
		// Sunday but after 00:00 — most recent close is today's 00:00.
		return todayMidnight
	default:
		// Any other day → step back to the prior Sunday.
		return todayMidnight.AddDate(0, 0, -dayOffset)
	}
}

// WeeklyWindowForOffset(now, offsetWeeks) returns the (start, end) of
// the weekly window `offsetWeeks` BEFORE the current open window.
//
//   offsetWeeks=0 → current open week ends at the NEXT Sunday 00:00 UTC.
//   offsetWeeks=1 → previous closed week (ended at most recent Sunday).
//   offsetWeeks=2 → the week before that.
//
// The handler uses offsetWeeks=0 for live compute and offsetWeeks=N>=1
// for snapshot reads.
func WeeklyWindowForOffset(now time.Time, offsetWeeks int) (start, end time.Time) {
	if offsetWeeks <= 0 {
		// Current open window: ends at the NEXT Sunday 00:00 UTC.
		mostRecent := MostRecentClosedWeekly(now)
		end = mostRecent.AddDate(0, 0, 7)
		start = mostRecent
		return start, end
	}
	mostRecent := MostRecentClosedWeekly(now)
	end = mostRecent.AddDate(0, 0, -7*(offsetWeeks-1))
	start = end.AddDate(0, 0, -7)
	return start, end
}
