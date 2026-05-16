// leaderboard-snapshot is the weekly cron entry point for the Wave 3
// leaderboard. It iterates every active tenant × course × site-scope
// currency and stores one snapshot row per (course, currency, window)
// via `LeaderboardSnapshotService.ComputeCourseWeekly`.
//
// Idempotent: re-running the same window is a no-op (UNIQUE on
// (scope, currency, kind, window_end) → ON CONFLICT DO NOTHING in
// the repo). Safe to wire to system cron with retry-on-failure.
//
// Usage:
//
//	# Most-recent closed weekly window (Sunday 00:00 UTC):
//	DATABASE_URL=... go run ./cmd/leaderboard-snapshot
//
//	# Specific window-end (RFC3339):
//	DATABASE_URL=... go run ./cmd/leaderboard-snapshot --window-end 2026-05-10T00:00:00Z
//
//	# Restrict to a single course (debug / backfill):
//	DATABASE_URL=... go run ./cmd/leaderboard-snapshot --course 1
//
// Operator wiring (recommended): system cron at Sunday 00:05 UTC, so
// the previous week's close (Sunday 00:00 UTC) is captured before any
// learner activity in the new week shifts wallet balances.
//
//	5 0 * * 0  /usr/local/bin/leaderboard-snapshot
//
// pg_cron integration is a future option; the CLI keeps the operator
// surface portable across deployments that may not have pg_cron.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	gormpg "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	pgRepo "github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
)

func main() { os.Exit(run()) }

func run() int {
	windowEndFlag := flag.String("window-end", "", "RFC3339 window-end (default: most-recent closed weekly Sunday 00:00 UTC)")
	courseFilter := flag.Uint("course", 0, "if non-zero, only compute for this course id (debug / backfill)")
	tenantFilter := flag.Uint("tenant", 0, "if non-zero, only compute for this tenant id (debug / backfill)")
	dry := flag.Bool("dry-run", false, "list windows that would be computed; write nothing")
	flag.Parse()

	_ = godotenv.Load()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL not set")
		return 2
	}

	// Resolve window-end.
	now := time.Now().UTC()
	windowEnd := gamification.MostRecentClosedWeekly(now)
	if *windowEndFlag != "" {
		parsed, err := time.Parse(time.RFC3339, *windowEndFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid --window-end (need RFC3339): %v\n", err)
			return 2
		}
		windowEnd = parsed.UTC()
	}
	log.Printf("window-end: %s", windowEnd.Format(time.RFC3339))

	// Connect.
	db, err := gorm.Open(gormpg.Open(dbURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Printf("open database: %v", err)
		return 2
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	// Wire repos + service.
	enrollmentRepo := pgRepo.NewEnrollmentRepository(db)
	userRepo := pgRepo.NewUserRepository(db)
	walletRepo := pgRepo.NewGamificationWalletRepository(db)
	snapshotRepo := pgRepo.NewGamificationLeaderboardSnapshotRepository(db)
	svc := gamification.NewLeaderboardSnapshotService(enrollmentRepo, userRepo, walletRepo, snapshotRepo)

	ctx := context.Background()

	// Walk active tenants → currencies → courses. Currencies are
	// scoped at the tenant (site scope), so one snapshot per (course,
	// currency). v1 only takes the SYSTEM site-scope currencies — a
	// future flag could widen this to course-scope currencies once
	// the Wave 5 OB pivot lands.
	var tenants []models.Account
	tenantQ := db.WithContext(ctx).Model(&models.Account{}).Where("workflow_state = ?", "active")
	if *tenantFilter != 0 {
		tenantQ = tenantQ.Where("id = ?", *tenantFilter)
	}
	if err := tenantQ.Find(&tenants).Error; err != nil {
		log.Printf("list tenants: %v", err)
		return 2
	}

	totalWritten := 0
	totalSkipped := 0
	for _, t := range tenants {
		var currencies []models.GamificationCurrencyType
		err := db.WithContext(ctx).Model(&models.GamificationCurrencyType{}).
			Where("tenant_id = ? AND scope_type = ? AND scope_id = ?",
				t.ID, models.ScopeSite, t.ID).
			Find(&currencies).Error
		if err != nil {
			log.Printf("tenant %d: list currencies: %v", t.ID, err)
			continue
		}

		var courses []models.Course
		courseQ := db.WithContext(ctx).Model(&models.Course{}).Where("account_id = ? AND workflow_state = ?", t.ID, "available")
		if *courseFilter != 0 {
			courseQ = courseQ.Where("id = ?", *courseFilter)
		}
		if err := courseQ.Find(&courses).Error; err != nil {
			log.Printf("tenant %d: list courses: %v", t.ID, err)
			continue
		}

		for _, c := range courses {
			for _, cur := range currencies {
				if *dry {
					log.Printf("[dry] would snapshot course=%d currency=%s window_end=%s",
						c.ID, cur.Code, windowEnd.Format(time.RFC3339))
					continue
				}
				created, err := svc.ComputeCourseWeekly(ctx, c.ID, cur.ID, windowEnd)
				if err != nil {
					log.Printf("course=%d currency=%s: %v", c.ID, cur.Code, err)
					continue
				}
				if created {
					totalWritten++
					log.Printf("✓ snapshot course=%d currency=%s", c.ID, cur.Code)
				} else {
					totalSkipped++
				}
			}
		}
	}

	if *dry {
		log.Println("[dry-run] no writes performed")
		return 0
	}
	log.Printf("done — %d snapshots written, %d skipped (already exist or empty cohort)", totalWritten, totalSkipped)
	return 0
}
