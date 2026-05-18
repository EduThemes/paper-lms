package postgres_test

// Integration test for UserRepo.FilterPublicLeaderboardCandidates.
// Skipped when no Postgres is reachable — mirrors the project's
// PARITY_DB_URL pattern (see internal/service/gamification/seed_test.go
// and internal/db/schemagen/parity_test.go).

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/db"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	_ "github.com/lib/pq"
	pgdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// FilterPublicLeaderboardCandidates is the W2-C privacy guard. The
// integration test pins the contract directly against migration 000040:
//
//   - empty input → nil, no DB hit
//   - candidate of an opted-in user → present in result
//   - candidate of an opted-out user → absent from result
//   - mixed set → only the opted-in subset survives
//   - non-existent IDs → silently dropped
func TestFilterPublicLeaderboardCandidates(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()

	ctx := context.Background()
	repo := postgres.NewUserRepository(g)

	// Seed the root account so users.account_id FK (000052) resolves —
	// User.BeforeCreate defaults AccountID to 1 when unset.
	if err := g.Exec(
		`INSERT INTO accounts (id, name, workflow_state, mfa_policy, default_locale, tenant_mode, max_upload_size_mb)
		 VALUES (1, 'Root', 'active', 'off', 'en', 'higher_ed', 500)
		 ON CONFLICT (id) DO NOTHING`,
	).Error; err != nil {
		t.Fatalf("seed root account: %v", err)
	}

	// Seed 4 users: two opted in (5, 6), two opted out (7, 8).
	for _, u := range []models.User{
		{ID: 5, Name: "In Alice", LoginID: "alice@in.test", Email: "alice@in.test", PasswordHash: "x", Role: "user", LeaderboardOptOut: false},
		{ID: 6, Name: "In Bob", LoginID: "bob@in.test", Email: "bob@in.test", PasswordHash: "x", Role: "user", LeaderboardOptOut: false},
		{ID: 7, Name: "Out Carol", LoginID: "carol@out.test", Email: "carol@out.test", PasswordHash: "x", Role: "user", LeaderboardOptOut: true},
		{ID: 8, Name: "Out Dan", LoginID: "dan@out.test", Email: "dan@out.test", PasswordHash: "x", Role: "user", LeaderboardOptOut: true},
	} {
		u := u
		if err := g.WithContext(ctx).Create(&u).Error; err != nil {
			t.Fatalf("seed user %d: %v", u.ID, err)
		}
	}

	t.Run("empty input returns nil", func(t *testing.T) {
		got, err := repo.FilterPublicLeaderboardCandidates(ctx, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("want nil, got %v", got)
		}
	})

	t.Run("all opted-in survive", func(t *testing.T) {
		got, err := repo.FilterPublicLeaderboardCandidates(ctx, []uint{5, 6})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertSet(t, got, []uint{5, 6})
	})

	t.Run("all opted-out filtered", func(t *testing.T) {
		got, err := repo.FilterPublicLeaderboardCandidates(ctx, []uint{7, 8})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("want empty, got %v", got)
		}
	})

	t.Run("mixed set keeps only opted-in", func(t *testing.T) {
		got, err := repo.FilterPublicLeaderboardCandidates(ctx, []uint{5, 6, 7, 8})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertSet(t, got, []uint{5, 6})
	})

	t.Run("non-existent IDs silently dropped", func(t *testing.T) {
		got, err := repo.FilterPublicLeaderboardCandidates(ctx, []uint{5, 999, 1000})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertSet(t, got, []uint{5})
	})
}

// Wave 1.6 follow-up — requires_password_reset column round-trip.
// The column defaults to FALSE; setting it to TRUE and reading the
// row back must preserve the flag. Migration 000061 adds the column;
// this test pins the contract.
func TestUserRequiresPasswordReset_RoundTrip(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()

	ctx := context.Background()
	repo := postgres.NewUserRepository(g)

	if err := g.Exec(
		`INSERT INTO accounts (id, name, workflow_state, mfa_policy, default_locale, tenant_mode, max_upload_size_mb)
		 VALUES (1, 'Root', 'active', 'off', 'en', 'higher_ed', 500)
		 ON CONFLICT (id) DO NOTHING`,
	).Error; err != nil {
		t.Fatalf("seed root account: %v", err)
	}

	// Default: flag is FALSE without an explicit set.
	defaulted := &models.User{
		Name: "Default User", LoginID: "default@in.test", Email: "default@in.test",
		PasswordHash: "x", Role: "user",
	}
	if err := repo.Create(ctx, defaulted); err != nil {
		t.Fatalf("create default user: %v", err)
	}
	got, err := repo.FindByID(ctx, defaulted.ID)
	if err != nil {
		t.Fatalf("find default user: %v", err)
	}
	if got.RequiresPasswordReset {
		t.Errorf("default RequiresPasswordReset should be false, got true")
	}

	// Explicit TRUE round-trips.
	flagged := &models.User{
		Name: "Flagged User", LoginID: "flagged@in.test", Email: "flagged@in.test",
		PasswordHash: "x", Role: "user", RequiresPasswordReset: true,
	}
	if err := repo.Create(ctx, flagged); err != nil {
		t.Fatalf("create flagged user: %v", err)
	}
	got, err = repo.FindByID(ctx, flagged.ID)
	if err != nil {
		t.Fatalf("find flagged user: %v", err)
	}
	if !got.RequiresPasswordReset {
		t.Errorf("RequiresPasswordReset should be true after set, got false")
	}

	// Clearing the flag via Update persists.
	got.RequiresPasswordReset = false
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("update flagged user: %v", err)
	}
	after, err := repo.FindByID(ctx, flagged.ID)
	if err != nil {
		t.Fatalf("re-find flagged user: %v", err)
	}
	if after.RequiresPasswordReset {
		t.Errorf("RequiresPasswordReset should be false after clearing, got true")
	}
}

// assertSet checks two id slices have the same membership (order-free).
// The filter is not required to preserve order — the caller already has
// the ranking from its own leaderboard query.
func assertSet(t *testing.T, got, want []uint) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len mismatch: got %v, want %v", got, want)
	}
	seen := make(map[uint]bool, len(got))
	for _, id := range got {
		seen[id] = true
	}
	for _, id := range want {
		if !seen[id] {
			t.Errorf("missing id %d: got %v, want %v", id, got, want)
		}
	}
}

// --- DB plumbing — mirrors internal/service/gamification/seed_test.go ---

func freshDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()
	parityURL := os.Getenv("PARITY_DB_URL")
	if parityURL == "" {
		parityURL = os.Getenv("DATABASE_URL")
	}
	if parityURL == "" {
		t.Skip("set PARITY_DB_URL (or DATABASE_URL) to run repo integration tests")
	}

	adminURL := swapDatabase(t, parityURL, "postgres")
	admin, err := sql.Open("postgres", adminURL)
	if err != nil {
		t.Fatalf("open admin: %v", err)
	}

	name := fmt.Sprintf("paper_lms_userrepo_%d", time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := admin.ExecContext(ctx, fmt.Sprintf(`CREATE DATABASE %q`, name)); err != nil {
		_ = admin.Close()
		t.Fatalf("create db %s: %v", name, err)
	}

	dbURL := swapDatabase(t, parityURL, name)
	bs, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("open %s: %v", dbURL, err)
	}
	if _, err := bs.Exec(`CREATE EXTENSION IF NOT EXISTS vector`); err != nil {
		_ = bs.Close()
		t.Fatalf("create extension: %v", err)
	}
	_ = bs.Close()

	g, err := gorm.Open(pgdriver.Open(dbURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("gorm open: %v", err)
	}
	if err := db.MigrateUp(g); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	cleanup := func() {
		if raw, err := g.DB(); err == nil {
			_ = raw.Close()
		}
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer dropCancel()
		_, _ = admin.ExecContext(dropCtx, fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, name))
		_ = admin.Close()
	}
	return g, cleanup
}

func swapDatabase(t *testing.T, rawURL, dbName string) string {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	u.Path = "/" + dbName
	return u.String()
}
