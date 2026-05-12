package schemagen_test

// TestSchemaParity exists to keep the SQL migration chain in lockstep with the
// GORM AutoMigrate registration. Without this test, the chain drifts the
// moment someone adds a model and forgets a migration — exactly how the
// project ended up with the Wave 1 backfill in the first place.
//
// The test spins up two ephemeral databases on a real Postgres pointed to by
// PARITY_DB_URL (or DATABASE_URL as a fallback), runs both schema builders,
// and compares. It is skipped on machines without a reachable Postgres so
// `go test ./...` stays green on dev laptops that aren't running the dev
// container; CI sets PARITY_DB_URL explicitly.
//
// The assertion is strict: `Diff.Empty()` must hold. No missing tables, no
// missing columns, no creatable indexes outstanding. Stale columns (SQL chain
// has, AutoMigrate doesn't) are reported via t.Logf but don't fail the test —
// they're informational signal for future cleanup, not deployment blockers.

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/db"
	"github.com/EduThemes/paper-lms/internal/db/schemagen"
	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSchemaParity_Wave1(t *testing.T) {
	parityURL := os.Getenv("PARITY_DB_URL")
	if parityURL == "" {
		parityURL = os.Getenv("DATABASE_URL")
	}
	if parityURL == "" {
		t.Skip("set PARITY_DB_URL (or DATABASE_URL) to a Postgres admin connection to run this test")
	}

	adminURL := swapDatabase(t, parityURL, "postgres")
	admin, err := sql.Open("postgres", adminURL)
	if err != nil {
		t.Fatalf("open admin: %v", err)
	}
	t.Cleanup(func() { _ = admin.Close() })

	stamp := time.Now().UnixNano()
	amName := fmt.Sprintf("paper_lms_parity_am_%d", stamp)
	sqlName := fmt.Sprintf("paper_lms_parity_sql_%d", stamp)

	createScratchDB(t, admin, amName)
	createScratchDB(t, admin, sqlName)

	amURL := swapDatabase(t, parityURL, amName)
	sqlURL := swapDatabase(t, parityURL, sqlName)

	bootstrapExtensions(t, amURL)
	bootstrapExtensions(t, sqlURL)

	wantSchema, err := buildAutoMigrateSchema(amURL)
	if err != nil {
		t.Fatalf("automigrate schema: %v", err)
	}
	gotSchema, err := buildSQLChainSchema(sqlURL)
	if err != nil {
		t.Fatalf("sql chain schema: %v", err)
	}

	d := schemagen.ComputeDiff(wantSchema, gotSchema)

	if n := len(d.MissingTables); n > 0 {
		names := make([]string, n)
		for i, t := range d.MissingTables {
			names[i] = t.Name
		}
		t.Errorf("SQL chain is missing %d table(s) that AutoMigrate creates: %v\n"+
			"Run `make schema-diff` and add the output to a new migration.", n, names)
	}
	if n := len(d.MissingColumns); n > 0 {
		t.Errorf("SQL chain is missing columns on %d table(s) that AutoMigrate creates.\n"+
			"Run `make schema-diff` for the full list and `make schema-diff-sql` for the ALTER TABLE statements.",
			n)
		// Log the first few for diagnostic context without overwhelming the log.
		count := 0
		for table, cols := range d.MissingColumns {
			if count >= 5 {
				t.Logf("  … and %d more table(s)", len(d.MissingColumns)-count)
				break
			}
			names := make([]string, len(cols))
			for i, c := range cols {
				names[i] = c.Name
			}
			t.Logf("  %s: %v", table, names)
			count++
		}
	}
	if n := len(d.SafeIndexes); n > 0 {
		names := make([]string, n)
		for i, idx := range d.SafeIndexes {
			names[i] = idx.Name
		}
		t.Errorf("SQL chain is missing %d creatable index(es) that AutoMigrate produces: %v", n, names)
	}
	if n := len(d.DeferredIndexes); n > 0 {
		t.Errorf("SQL chain has %d index(es) deferred behind missing columns — fix the columns and the indexes will follow", n)
	}

	// Stale columns are informational. They typically come from model
	// refactors that bypassed migrations; dropping them is data-destructive,
	// so we surface but never auto-fix.
	if n := len(d.StaleColumns); n > 0 {
		staleTotal := 0
		for _, cols := range d.StaleColumns {
			staleTotal += len(cols)
		}
		t.Logf("informational: %d stale column(s) across %d table(s) — usually leftover from model refactors. See `make schema-diff` for the full list; cleanup is a separate decision per column.",
			staleTotal, n)
	}
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

func createScratchDB(t *testing.T, admin *sql.DB, name string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Identifier is from a fixed format with a timestamp, no user input.
	if _, err := admin.ExecContext(ctx, fmt.Sprintf(`CREATE DATABASE %q`, name)); err != nil {
		t.Fatalf("create database %s: %v", name, err)
	}
	t.Cleanup(func() {
		// FORCE drops sessions; needed because the test connections may not be
		// fully closed yet when cleanup runs.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _ = admin.ExecContext(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, name))
	})
}

func bootstrapExtensions(t *testing.T, dbURL string) {
	t.Helper()
	conn, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("open %s: %v", dbURL, err)
	}
	defer conn.Close()
	if _, err := conn.Exec(`CREATE EXTENSION IF NOT EXISTS vector`); err != nil {
		t.Fatalf("create extension vector: %v", err)
	}
}

func buildAutoMigrateSchema(dbURL string) (*schemagen.Schema, error) {
	gdb, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}
	defer closeGorm(gdb)
	if err := db.AutoMigrate(gdb); err != nil {
		return nil, err
	}
	raw, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	return schemagen.Introspect(raw)
}

func buildSQLChainSchema(dbURL string) (*schemagen.Schema, error) {
	gdb, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}
	defer closeGorm(gdb)
	if err := db.MigrateUp(gdb); err != nil {
		return nil, err
	}
	raw, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	return schemagen.Introspect(raw)
}

func closeGorm(g *gorm.DB) {
	if raw, err := g.DB(); err == nil {
		_ = raw.Close()
	}
}
