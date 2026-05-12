// seedgamification is a one-shot backfill binary that walks every row in
// the accounts table and seeds the four system-owned currencies
// (xp, gems, mastery_points, reputation) at site scope for each tenant.
//
// Safe to re-run: SeedSystemCurrenciesForTenant uses ON CONFLICT DO
// NOTHING against the uniq_gam_currency_scope_code unique index, so
// re-runs against an already-populated tenant are no-ops.
//
// Usage:
//
//	DATABASE_URL=postgres://paper:paper@localhost:5433/paper_lms?sslmode=disable \
//	    go run ./cmd/seedgamification
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	os.Exit(run())
}

func run() int {
	_ = godotenv.Load()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL not set")
		return 2
	}

	g, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Printf("open database: %v", err)
		return 2
	}
	defer func() {
		if raw, err := g.DB(); err == nil {
			_ = raw.Close()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var accounts []models.Account
	if err := g.WithContext(ctx).Select("id").Find(&accounts).Error; err != nil {
		log.Printf("list accounts: %v", err)
		return 2
	}
	if len(accounts) == 0 {
		fmt.Fprintln(os.Stderr, "no accounts found; nothing to seed")
		return 0
	}

	for _, a := range accounts {
		if err := gamification.SeedSystemCurrenciesForTenant(ctx, g, a.ID); err != nil {
			log.Printf("seed account %d: %v", a.ID, err)
			return 1
		}
	}
	fmt.Fprintf(os.Stderr, "seeded %d account(s) with system currencies\n", len(accounts))
	return 0
}
