package gamification_test

// Integration test for SeedSystemCurrenciesForTenant. Skipped when no
// Postgres is reachable — matches the project's PARITY_DB_URL pattern in
// internal/db/schemagen/parity_test.go so `go test ./...` stays green on
// laptops without a dev container.

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
	"github.com/EduThemes/paper-lms/internal/service/gamification"
	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSeedSystemCurrenciesForTenant(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()

	ctx := context.Background()
	seedTenantAccount(t, g, 1)
	if err := gamification.SeedSystemCurrenciesForTenant(ctx, g, 1); err != nil {
		t.Fatalf("seed tenant 1: %v", err)
	}

	var rows []models.GamificationCurrencyType
	if err := g.Where("tenant_id = ?", 1).Order("display_order").Find(&rows).Error; err != nil {
		t.Fatalf("list rows: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("expected 4 system currencies, got %d", len(rows))
	}
	wantCodes := []string{"xp", "gems", "mastery_points", "reputation"}
	// Per SYNTHESIS §2 and §4-currency design: lock down both bool fields
	// the seed needs to write as `false` against SQL DEFAULT TRUE columns.
	// History: GORM's `default:` tag silently elided these zero-valued
	// inserts, flipping mastery_points into the topbar (FERPA breach) and
	// gems into the monotonic-currency set (breaks spendable semantics).
	wantTopbar := map[string]bool{
		"xp":             true,
		"gems":           true,
		"mastery_points": false,
		"reputation":     true,
	}
	wantMonotonic := map[string]bool{
		"xp":             true,
		"gems":           false,
		"mastery_points": true,
		"reputation":     true,
	}
	wantSpendable := map[string]bool{
		"xp":             false,
		"gems":           true,
		"mastery_points": false,
		"reputation":     false,
	}
	for i, want := range wantCodes {
		if rows[i].Code != want {
			t.Errorf("row %d Code = %q, want %q", i, rows[i].Code, want)
		}
		if !rows[i].SystemOwned {
			t.Errorf("row %d SystemOwned = false, want true", i)
		}
		if rows[i].ScopeType != models.ScopeSite || rows[i].ScopeID != 1 {
			t.Errorf("row %d scope = %s/%d, want site/1", i, rows[i].ScopeType, rows[i].ScopeID)
		}
		if got, want := rows[i].VisibleInTopbar, wantTopbar[rows[i].Code]; got != want {
			t.Errorf("row %s VisibleInTopbar = %v, want %v (FERPA contract)", rows[i].Code, got, want)
		}
		if got, want := rows[i].Monotonic, wantMonotonic[rows[i].Code]; got != want {
			t.Errorf("row %s Monotonic = %v, want %v", rows[i].Code, got, want)
		}
		if got, want := rows[i].Spendable, wantSpendable[rows[i].Code]; got != want {
			t.Errorf("row %s Spendable = %v, want %v", rows[i].Code, got, want)
		}
	}
}

func TestSeedSystemCurrenciesForTenant_Idempotent(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()

	ctx := context.Background()
	seedTenantAccount(t, g, 1)
	for i := 0; i < 3; i++ {
		if err := gamification.SeedSystemCurrenciesForTenant(ctx, g, 1); err != nil {
			t.Fatalf("seed attempt %d: %v", i+1, err)
		}
	}

	var count int64
	if err := g.Model(&models.GamificationCurrencyType{}).Where("tenant_id = ?", 1).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 4 {
		t.Fatalf("expected 4 rows after re-running 3×, got %d", count)
	}
}

func TestSeedSystemCurrenciesForTenant_MultipleTenants(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()

	ctx := context.Background()
	for _, tenant := range []uint{1, 2, 3} {
		seedTenantAccount(t, g, tenant)
		if err := gamification.SeedSystemCurrenciesForTenant(ctx, g, tenant); err != nil {
			t.Fatalf("seed tenant %d: %v", tenant, err)
		}
	}

	var count int64
	if err := g.Model(&models.GamificationCurrencyType{}).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 12 {
		t.Fatalf("expected 12 rows across 3 tenants, got %d", count)
	}
}

func TestSeedSystemCurrenciesForTenant_RejectsZeroTenant(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()

	if err := gamification.SeedSystemCurrenciesForTenant(context.Background(), g, 0); err == nil {
		t.Fatalf("expected error for tenantID=0")
	}
}

// --- DB plumbing — mirrors internal/db/schemagen/parity_test.go ---

func freshDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()
	parityURL := os.Getenv("PARITY_DB_URL")
	if parityURL == "" {
		parityURL = os.Getenv("DATABASE_URL")
	}
	if parityURL == "" {
		t.Skip("set PARITY_DB_URL (or DATABASE_URL) to run seeder integration tests")
	}

	adminURL := swapDatabase(t, parityURL, "postgres")
	admin, err := sql.Open("postgres", adminURL)
	if err != nil {
		t.Fatalf("open admin: %v", err)
	}

	name := fmt.Sprintf("paper_lms_seed_%d", time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := admin.ExecContext(ctx, fmt.Sprintf(`CREATE DATABASE %q`, name)); err != nil {
		_ = admin.Close()
		t.Fatalf("create db %s: %v", name, err)
	}

	dbURL := swapDatabase(t, parityURL, name)
	// pgvector extension is required by the standard migration chain.
	bs, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("open %s: %v", dbURL, err)
	}
	if _, err := bs.Exec(`CREATE EXTENSION IF NOT EXISTS vector`); err != nil {
		_ = bs.Close()
		t.Fatalf("create extension: %v", err)
	}
	_ = bs.Close()

	g, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{
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

// seedTenantAccount inserts an accounts row with the given id so the
// fk_gam_currencies_tenant FK (000050) resolves when
// SeedSystemCurrenciesForTenant writes its 4 system-currency rows. The
// FK landed in Phase 7-A; pre-FK these tests trusted the orphan id.
// Re-entrant via ON CONFLICT.
func seedTenantAccount(t *testing.T, g *gorm.DB, id uint) {
	t.Helper()
	if err := g.Exec(
		`INSERT INTO accounts (id, name, workflow_state, mfa_policy, default_locale, tenant_mode, max_upload_size_mb)
		 VALUES (?, 'Test Tenant', 'active', 'off', 'en', 'higher_ed', 500)
		 ON CONFLICT (id) DO NOTHING`,
		id,
	).Error; err != nil {
		t.Fatalf("seed tenant account %d: %v", id, err)
	}
}
