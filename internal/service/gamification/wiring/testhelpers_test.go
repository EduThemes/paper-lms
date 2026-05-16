package wiring_test

// Shared DB plumbing for every wiring_test integration test. Mirrors the
// PARITY_DB_URL / DATABASE_URL skip pattern in
// internal/service/gamification/seed_test.go (which can't be imported
// because it lives in package gamification_test).

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
	pgrepo "github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
	"github.com/EduThemes/paper-lms/internal/service/gamification/effects"
	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// seedTestUser inserts a User row with the given email under the given
// tenant. User.BeforeCreate fills in WebauthnUserHandle (NOT NULL,
// 000046). Required wherever a test references a user_id that needs to
// resolve to a real row to satisfy FKs from 000054 (Phase 13.2 — core
// FK migration on enrollments/submissions/audit_logs/etc).
func seedTestUser(t *testing.T, g *gorm.DB, accountID uint, email string) models.User {
	t.Helper()
	u := models.User{
		Name:      "Test User " + email,
		Email:     email,
		LoginID:   email,
		Role:      "user",
		AccountID: accountID,
	}
	if err := u.HashPassword("placeholder"); err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if err := g.Create(&u).Error; err != nil {
		t.Fatalf("seed test user %s: %v", email, err)
	}
	return u
}

// freshDB returns a fully migrated scratch Postgres + a cleanup func
// that drops the database. Skips the calling test when no Postgres is
// reachable so `go test ./...` stays green on dev laptops without the
// dev container.
func freshDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()
	parityURL := os.Getenv("PARITY_DB_URL")
	if parityURL == "" {
		parityURL = os.Getenv("DATABASE_URL")
	}
	if parityURL == "" {
		t.Skip("set PARITY_DB_URL (or DATABASE_URL) to run wiring integration tests")
	}

	adminURL := swapDatabase(t, parityURL, "postgres")
	admin, err := sql.Open("postgres", adminURL)
	if err != nil {
		t.Fatalf("open admin: %v", err)
	}

	name := fmt.Sprintf("paper_lms_wiring_%d", time.Now().UnixNano())
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

// buildEmitter assembles a full gamification.Emitter against a fresh
// GORM connection — mirrors the production wiring in cmd/server/main.go.
// Shared across every wiring_test that needs a real Emitter.
func buildEmitter(t *testing.T, g *gorm.DB) *gamification.Emitter {
	t.Helper()
	subRepo := pgrepo.NewSubmissionRepository(g)
	quizSubRepo := pgrepo.NewQuizSubmissionRepository(g)
	outcomeRepo := pgrepo.NewLearningOutcomeResultRepository(g)
	contentViewRepo := pgrepo.NewContentViewRepository(g)
	walletRepo := pgrepo.NewGamificationWalletRepository(g)
	currencyRepo := pgrepo.NewGamificationCurrencyTypeRepository(g)
	ruleRepo := pgrepo.NewGamificationRuleRepository(g)
	eventRepo := pgrepo.NewGamificationEventRepository(g)
	ferpaRepo := pgrepo.NewGamificationFerpaFieldTagRepository(g)

	deps := gamification.EmitterDeps{
		Dispatch: gamification.DispatchDeps{
			Snapshot: gamification.SnapshotDeps{
				Submissions:     subRepo,
				QuizSubmissions: quizSubRepo,
				OutcomeResults:  outcomeRepo,
				ContentViews:    contentViewRepo,
				Wallet:          walletRepo,
				CurrencyType:    currencyRepo,
			},
			Rules: ruleRepo,
			Effects: effects.EffectDeps{
				Wallet:       walletRepo,
				CurrencyType: currencyRepo,
			},
		},
		Events:    eventRepo,
		FerpaTags: ferpaRepo,
	}
	return gamification.NewEmitter(deps)
}
