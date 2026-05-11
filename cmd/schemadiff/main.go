// schemadiff compares the GORM AutoMigrate output against the versioned SQL
// migration chain and reports what the SQL chain is missing.
//
// It works by creating two transient databases on the same Postgres instance,
// running AutoMigrate on one and the SQL migration chain on the other, and
// structurally diffing the result. The two databases are dropped on exit
// regardless of outcome.
//
// Usage:
//
//	DATABASE_URL=postgres://paper:paper@localhost:5433/paper_lms?sslmode=disable \
//	    go run ./cmd/schemadiff [--emit-sql]
//
// Flags:
//
//	--emit-sql        Print the missing CREATE TABLE / CREATE INDEX statements
//	                  ready to paste into a new migration. Without this flag,
//	                  only a human-readable summary is printed.
//	--keep            Don't drop the scratch databases on exit (debugging).
//
// Exit code 0 = schemas match. Exit code 1 = drift detected (suitable for CI).
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"net/url"
	"os"
	"sort"
	"time"

	"github.com/EduThemes/paper-lms/internal/db"
	"github.com/EduThemes/paper-lms/internal/db/schemagen"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// sectionGroups maps table names to a human-readable section label used when
// emitting the backfill migration. The groupings mirror the comment blocks in
// internal/db/postgres.go::AutoMigrate so reviewers see the same logical
// divisions in the SQL as in the Go source.
//
// Unmapped tables fall into "uncategorized" — that bucket should be empty in a
// healthy state and grows when someone adds a model without updating this map.
var sectionGroups = map[string]string{
	// Identity / accounts
	"users": "identity", "accounts": "identity", "developer_keys": "identity",
	"access_tokens": "identity", "authentication_providers": "identity",
	"nonces": "identity", "communication_channels": "identity",
	// Courses / enrollments
	"courses": "courses", "course_sections": "courses", "enrollments": "courses",
	"enrollment_terms": "courses", "course_paces": "courses",
	"course_pace_module_items": "courses", "course_home_buttons": "courses",
	"todays_lesson_overrides": "courses", "course_visits": "courses",
	// Content / modules / files
	"context_modules": "content", "content_tags": "content",
	"module_prerequisites": "content", "wiki_pages": "content",
	"folders": "content", "attachments": "content",
	"document_annotations": "content",
	// Assignments / submissions / rubrics
	"assignments": "assignments", "assignment_groups": "assignments",
	"submissions": "assignments", "submission_comments": "assignments",
	"grading_standards": "assignments", "grading_period_groups": "assignments",
	"grading_periods": "assignments", "assignment_overrides": "assignments",
	"assignment_override_students": "assignments", "late_policies": "assignments",
	"rubrics": "assignments", "rubric_associations": "assignments",
	"rubric_assessments": "assignments", "peer_reviews": "assignments",
	// Quiz engine
	"quizzes": "quizzes", "quiz_questions": "quizzes",
	"quiz_submissions": "quizzes", "quiz_submission_answers": "quizzes",
	"question_banks": "quizzes", "question_bank_entries": "quizzes",
	"quiz_question_groups": "quizzes",
	"quiz_item_banks": "quizzes", "quiz_item_bank_items": "quizzes",
	"quiz_stimuli": "quizzes", "quiz_question_outcome_alignments": "quizzes",
	// Discussions
	"discussion_topics": "discussions", "discussion_entries": "discussions",
	"discussion_entry_ratings": "discussions",
	"discussion_entry_participants": "discussions",
	"discussion_topic_participants": "discussions",
	"discussion_entry_versions": "discussions",
	"discussion_checkpoints": "discussions",
	"discussion_checkpoint_submissions": "discussions",
	// LTI / external tools
	"lti_tool_configurations": "lti", "context_external_tools": "lti",
	"lti_resource_links": "lti", "lti_line_items": "lti", "lti_results": "lti",
	// Calendar / messaging / notifications
	"calendar_events": "messaging", "conversations": "messaging",
	"conversation_participants": "messaging", "conversation_messages": "messaging",
	"notification_preferences": "messaging", "notifications": "messaging",
	"notification_deliveries": "messaging",
	// Outcomes
	"learning_outcome_groups": "outcomes", "learning_outcomes": "outcomes",
	"learning_outcome_results": "outcomes", "outcome_alignments": "outcomes",
	"outcome_proficiencies": "outcomes", "outcome_proficiency_ratings": "outcomes",
	// Groups / blueprint / SIS
	"group_categories": "groups", "groups": "groups",
	"group_memberships": "groups",
	"blueprint_templates": "blueprint", "blueprint_subscriptions": "blueprint",
	"blueprint_migrations": "blueprint",
	"sis_batches": "sis", "sis_batch_errors": "sis",
	"one_roster_connections": "sis", "one_roster_sync_logs": "sis",
	// Content migration
	"content_migrations": "migration",
	// Conferences / collaborations / analytics
	"collaborations": "collaboration", "conferences": "collaboration",
	"conference_participants": "collaboration", "page_views": "analytics",
	// Announcements
	"announcements": "announcements", "announcement_read_receipts": "announcements",
	// Audit / logs
	"audit_logs": "audit", "grade_change_logs": "audit",
	"pii_access_logs": "audit",
	// Custom roles
	"custom_roles": "roles", "role_overrides": "roles",
	// COPPA / FERPA / accommodations / attendance / portfolios
	"parental_consents": "compliance", "data_processing_agreements": "compliance",
	"age_verifications": "compliance", "data_retention_policies": "compliance",
	"data_deletion_requests": "compliance", "data_export_requests": "compliance",
	"student_accommodations": "accommodations",
	"accommodation_applications": "accommodations",
	"attendance_records": "attendance",
	"portfolios": "portfolios", "portfolio_sections": "portfolios",
	"portfolio_artifacts": "portfolios", "portfolio_reflections": "portfolios",
	"portfolio_templates": "portfolios", "portfolio_comments": "portfolios",
	// Feature flags / gradebook / mastery paths / appointments / pairing
	"feature_flags": "features",
	"custom_gradebook_columns": "gradebook", "custom_gradebook_column_data": "gradebook",
	"comment_bank_items": "gradebook",
	"conditional_release_rules": "mastery_paths",
	"conditional_release_scoring_ranges": "mastery_paths",
	"conditional_release_assignment_sets": "mastery_paths",
	"conditional_release_assignment_set_associations": "mastery_paths",
	"conditional_release_assignment_set_actions": "mastery_paths",
	"appointment_groups": "appointments", "appointment_slots": "appointments",
	"appointment_reservations": "appointments",
	"pairing_codes": "pairing",
	// Smart search / commons
	"content_embeddings": "smart_search",
	"shared_content": "commons", "shared_content_favorites": "commons",
}

func main() {
	// run() owns all work and cleanup; main() only translates the result code
	// into the process exit. This is necessary because os.Exit skips deferred
	// scratch-DB cleanup, and we don't want orphan databases piling up.
	os.Exit(run())
}

func run() int {
	emitSQL := flag.Bool("emit-sql", false, "print missing CREATE TABLE/INDEX as ready-to-paste SQL")
	keep := flag.Bool("keep", false, "do not drop scratch databases on exit")
	flag.Parse()

	_ = godotenv.Load()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL not set")
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Names use UnixNano + PID so concurrent runs (e.g., developer running
	// schema-diff while CI runs the parity test) can't collide. A bare Unix
	// timestamp at second resolution is not enough.
	suffix := fmt.Sprintf("%d_%d", time.Now().UnixNano(), os.Getpid())
	amName := "paper_lms_schemadiff_am_" + suffix
	sqlName := "paper_lms_schemadiff_sql_" + suffix

	adminURL := swapDatabase(dbURL, "postgres")
	admin, err := sql.Open("postgres", adminURL)
	if err != nil {
		return fatal("open admin: %v", err)
	}
	defer admin.Close()

	if err := createDB(ctx, admin, amName); err != nil {
		return fatal("create %s: %v", amName, err)
	}
	if err := createDB(ctx, admin, sqlName); err != nil {
		_ = dropDB(context.Background(), admin, amName)
		return fatal("create %s: %v", sqlName, err)
	}

	// Register cleanup BEFORE any work that can fail. A failure in extension
	// bootstrap, AutoMigrate, or MigrateUp must still drop the scratch DBs —
	// otherwise rerunning the tool would leak databases on every failure.
	defer func() {
		if *keep {
			fmt.Fprintf(os.Stderr, "scratch DBs kept: %s, %s\n", amName, sqlName)
			return
		}
		_ = dropDB(context.Background(), admin, amName)
		_ = dropDB(context.Background(), admin, sqlName)
	}()

	// Bootstrap pgvector on both scratch DBs. The SQL migration chain installs
	// it inside 000010_content_embeddings.up.sql with a try/catch fallback;
	// AutoMigrate's ContentEmbedding model assumes the type already exists.
	// Installing it up-front makes the comparison apples-to-apples and matches
	// the dev container's actual deploy posture.
	amURLBootstrap := swapDatabase(dbURL, amName)
	sqlURLBootstrap := swapDatabase(dbURL, sqlName)
	for _, u := range []string{amURLBootstrap, sqlURLBootstrap} {
		if err := bootstrapExtensions(u); err != nil {
			return fatal("bootstrap extensions: %v", err)
		}
	}

	amURL := swapDatabase(dbURL, amName)
	sqlURL := swapDatabase(dbURL, sqlName)

	fmt.Fprintln(os.Stderr, "→ building AutoMigrate schema...")
	wantSchema, err := buildAutoMigrateSchema(amURL)
	if err != nil {
		return fatal("automigrate schema: %v", err)
	}
	fmt.Fprintf(os.Stderr, "  %d tables\n", len(wantSchema.Tables))

	fmt.Fprintln(os.Stderr, "→ building SQL-migration schema...")
	gotSchema, err := buildSQLMigrationSchema(sqlURL)
	if err != nil {
		return fatal("sql migration schema: %v", err)
	}
	fmt.Fprintf(os.Stderr, "  %d tables\n", len(gotSchema.Tables))

	d := schemagen.ComputeDiff(wantSchema, gotSchema)

	if d.Empty() {
		fmt.Fprintln(os.Stderr, "✓ schemas match")
		return 0
	}

	missingColCount := 0
	for _, cols := range d.MissingColumns {
		missingColCount += len(cols)
	}
	staleColCount := 0
	for _, cols := range d.StaleColumns {
		staleColCount += len(cols)
	}
	fmt.Fprintf(os.Stderr,
		"✗ %d missing table(s), %d missing column(s) across %d table(s), %d safe index(es), %d deferred index(es); %d stale column(s) across %d table(s) (informational)\n",
		len(d.MissingTables),
		missingColCount, len(d.MissingColumns),
		len(d.SafeIndexes), len(d.DeferredIndexes),
		staleColCount, len(d.StaleColumns))

	if *emitSQL {
		header := "-- Backfill: tables and indexes present in GORM AutoMigrate but\n" +
			"-- absent from the versioned SQL migration chain. Generated by\n" +
			"-- cmd/schemadiff --emit-sql. Review before committing."
		fmt.Println(schemagen.RenderMigration(d, sectionGroups, header))
	} else {
		if len(d.MissingTables) > 0 {
			fmt.Println("Missing tables (topologically ordered):")
			for _, t := range d.MissingTables {
				label := sectionGroups[t.Name]
				if label == "" {
					label = "uncategorized"
				}
				fmt.Printf("  [%s] %s\n", label, t.Name)
			}
		}
		if len(d.MissingColumns) > 0 {
			fmt.Printf("\nMissing columns (AutoMigrate has, SQL chain doesn't — production deploys block on these):\n")
			tables := make([]string, 0, len(d.MissingColumns))
			for t := range d.MissingColumns {
				tables = append(tables, t)
			}
			sort.Strings(tables)
			for _, t := range tables {
				names := make([]string, len(d.MissingColumns[t]))
				for i, c := range d.MissingColumns[t] {
					names[i] = c.Name
				}
				fmt.Printf("  %s: %v\n", t, names)
			}
		}
		if len(d.SafeIndexes) > 0 {
			fmt.Printf("\nMissing indexes (safe to create): %d\n", len(d.SafeIndexes))
		}
		if len(d.StaleColumns) > 0 {
			fmt.Printf("\nStale columns (SQL chain has, AutoMigrate doesn't — usually old refactor leftovers, NOT auto-dropped):\n")
			tables := make([]string, 0, len(d.StaleColumns))
			for t := range d.StaleColumns {
				tables = append(tables, t)
			}
			sort.Strings(tables)
			for _, t := range tables {
				names := make([]string, len(d.StaleColumns[t]))
				for i, c := range d.StaleColumns[t] {
					names[i] = c.Name
				}
				fmt.Printf("  %s: %v\n", t, names)
			}
		}
		if len(d.DeferredIndexes) > 0 {
			fmt.Printf("\n%d index(es) deferred until missing columns are added.\n", len(d.DeferredIndexes))
		}
		fmt.Fprintln(os.Stderr, "\nrun with --emit-sql to print pasteable migration SQL.")
	}
	return 1
}

// fatal prints the error to stderr and returns process exit code 2. Use with
// `return fatal(...)` to ensure deferred scratch-DB cleanup still runs — a
// plain log.Fatalf would skip defers and leak databases.
func fatal(format string, args ...any) int {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	return 2
}

// swapDatabase rewrites a Postgres connection URL to point at a different
// database on the same server. Used to bounce between the target database and
// the `postgres` admin database (since `CREATE DATABASE` and `DROP DATABASE`
// can't run while connected to the target).
func swapDatabase(rawURL, dbName string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	u.Path = "/" + dbName
	return u.String()
}

func createDB(ctx context.Context, admin *sql.DB, name string) error {
	// Quoted identifier — name is from a fixed format string with a timestamp,
	// no user input.
	_, err := admin.ExecContext(ctx, fmt.Sprintf(`CREATE DATABASE %q`, name))
	return err
}

func dropDB(ctx context.Context, admin *sql.DB, name string) error {
	_, err := admin.ExecContext(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, name))
	return err
}

// bootstrapExtensions installs pgvector on a target database. If the extension
// is not available, we continue silently — AutoMigrate will then fail with a
// clear error, surfacing the misconfiguration to the operator.
func bootstrapExtensions(dbURL string) error {
	conn, err := sql.Open("postgres", dbURL)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Exec(`CREATE EXTENSION IF NOT EXISTS vector`)
	return err
}

func buildAutoMigrateSchema(dbURL string) (*schemagen.Schema, error) {
	gdb, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("gorm open: %w", err)
	}
	defer closeGorm(gdb)

	if err := db.AutoMigrate(gdb); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}
	raw, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	return schemagen.Introspect(raw)
}

func buildSQLMigrationSchema(dbURL string) (*schemagen.Schema, error) {
	gdb, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("gorm open: %w", err)
	}
	defer closeGorm(gdb)

	if err := db.MigrateUp(gdb); err != nil {
		return nil, fmt.Errorf("migrate up: %w", err)
	}
	raw, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	return schemagen.Introspect(raw)
}

func closeGorm(g *gorm.DB) {
	if sqlDB, err := g.DB(); err == nil {
		_ = sqlDB.Close()
	}
}
