package service_test

// Integration test for SISImportService.ProcessImport's users.csv
// password-default path. Skipped when no Postgres is reachable —
// matches the project's PARITY_DB_URL pattern (see
// internal/service/gamification/seed_test.go and
// internal/repository/postgres/user_test.go).
//
// The fix under test: prior to 2026-05-17, a CSV row that omitted
// the `password` column was given bcrypt("changeme") as its initial
// password — a static, universally-known default. Any operator who
// imported users via CSV without specifying passwords created
// every-user-has-the-same-password accounts. The new code path
// generates a 32-byte crypto/rand initial password per row, so the
// hash MUST NOT match bcrypt("changeme").

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/db"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
	pgdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSISImportUsersCSV_DefaultPasswordIsRandom(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()

	ctx := context.Background()

	// Seed the root account so users.account_id FK (000052) resolves
	// — User.BeforeCreate defaults AccountID to 1 when unset.
	if err := g.Exec(
		`INSERT INTO accounts (id, name, workflow_state, mfa_policy, default_locale, tenant_mode, max_upload_size_mb)
		 VALUES (1, 'Root', 'active', 'off', 'en', 'higher_ed', 500)
		 ON CONFLICT (id) DO NOTHING`,
	).Error; err != nil {
		t.Fatalf("seed root account: %v", err)
	}

	batchRepo := postgres.NewSISBatchRepository(g)
	errorRepo := postgres.NewSISBatchErrorRepository(g)
	userRepo := postgres.NewUserRepository(g)
	courseRepo := postgres.NewCourseRepository(g)
	sectionRepo := postgres.NewSectionRepository(g)
	enrollmentRepo := postgres.NewEnrollmentRepository(g)

	svc := service.NewSISImportService(batchRepo, errorRepo, userRepo, courseRepo, sectionRepo, enrollmentRepo, g)

	batch, err := svc.CreateBatch(ctx, 1)
	if err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}

	// CSV with no `password` column — exercises the default-password
	// branch that previously stored bcrypt("changeme").
	csvBody := bytes.NewBufferString("user_id,login_id,first_name,last_name,email,status\n" +
		"sis-001,alice.import,Alice,Importer,alice.import@example.com,active\n")

	if err := svc.ProcessImport(ctx, batch.ID, "users", csvBody); err != nil {
		t.Fatalf("ProcessImport: %v", err)
	}

	user, err := userRepo.FindByLoginID(ctx, "alice.import")
	if err != nil {
		t.Fatalf("FindByLoginID after import: %v", err)
	}
	if user.PasswordHash == "" {
		t.Fatalf("expected imported user to have a password hash set")
	}

	// The regression lock: the stored hash MUST NOT match bcrypt("changeme").
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("changeme")); err == nil {
		t.Fatalf("CVE-class regression: SIS-imported user's password hash matches the static default 'changeme'")
	}

	// Wave 1.6 follow-up: the random password is irrecoverable, so
	// the SIS path MUST set RequiresPasswordReset so the LoginPipeline
	// gates session minting and forces the user to choose a real
	// password before getting a session.
	if !user.RequiresPasswordReset {
		t.Fatal("SIS-imported user with a random default password must have RequiresPasswordReset=true")
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
		t.Skip("set PARITY_DB_URL (or DATABASE_URL) to run SIS import integration tests")
	}

	adminURL := swapDatabase(t, parityURL, "postgres")
	admin, err := sql.Open("postgres", adminURL)
	if err != nil {
		t.Fatalf("open admin: %v", err)
	}

	name := fmt.Sprintf("paper_lms_sisimp_%d", time.Now().UnixNano())
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
