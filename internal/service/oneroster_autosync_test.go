package service_test

// B173 regression: auto-sync connections are actually synced when their
// interval elapses (previously no job ever read the auto_sync flag).

import (
	"context"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

func TestOneRosterRunDueAutoSyncs(t *testing.T) {
	svc, g, cleanup := newDeprovisionFixture(t)
	defer cleanup()
	ctx := context.Background()

	recent := time.Now().Add(-1 * time.Hour)
	stale := time.Now().Add(-25 * time.Hour)
	mk := func(name string, auto bool, last *time.Time) *models.OneRosterConnection {
		c := &models.OneRosterConnection{
			AccountID: 1, Name: name,
			// Loopback is refused by the SSRF guard without any network
			// I/O, so the sync runs and records a failure deterministically.
			BaseURL: "http://127.0.0.1:1", TokenURL: "http://127.0.0.1:1/token",
			ClientID: "client", ClientSecret: "secret",
			AutoSync: auto, AutoSyncInterval: 24,
		}
		if err := svc.CreateConnection(ctx, c); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if err := g.Model(&models.OneRosterConnection{}).Where("id = ?", c.ID).
			Updates(map[string]interface{}{"auto_sync": auto, "last_sync_at": last}).Error; err != nil {
			t.Fatal(err)
		}
		return c
	}
	never := mk("never-synced", true, nil)
	overdue := mk("overdue", true, &stale)
	fresh := mk("fresh", true, &recent)
	manual := mk("manual-only", false, nil)

	ran, err := svc.RunDueAutoSyncs(ctx, time.Now())
	if ran != 2 {
		t.Fatalf("ran %d auto-syncs, want 2 (never-synced + overdue): %v", ran, err)
	}

	logs := func(connID uint) int64 {
		var n int64
		g.Model(&models.OneRosterSyncLog{}).Where("connection_id = ?", connID).Count(&n)
		return n
	}
	for _, c := range []*models.OneRosterConnection{never, overdue} {
		if logs(c.ID) != 1 {
			t.Errorf("due connection %q has %d sync logs, want 1", c.Name, logs(c.ID))
		}
	}
	for _, c := range []*models.OneRosterConnection{fresh, manual} {
		if logs(c.ID) != 0 {
			t.Errorf("connection %q was synced but is not due", c.Name)
		}
	}

	// The run stamped last_sync_at, so an immediate second pass is a no-op.
	if ran, _ := svc.RunDueAutoSyncs(ctx, time.Now()); ran != 0 {
		t.Fatalf("second pass ran %d syncs, want 0", ran)
	}
}
