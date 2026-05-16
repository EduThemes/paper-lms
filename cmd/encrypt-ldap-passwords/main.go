// encrypt-ldap-passwords backfills the AES-256-GCM ciphertext of every
// LDAP service-account bind password that is still stored only in
// plaintext (Phase 9-PRE shipped both columns side-by-side; Phase
// 11-A.1 is the migration cmd that flips every row to encrypted).
//
// The cmd is idempotent by query construction: only rows where
// `ldap_bind_password != '' AND ldap_bind_password_encrypted IS NULL`
// are selected. A second run after a successful first run reports
// 0 rows to process.
//
// Usage:
//
//	# Dry run — list rows that WOULD be encrypted, write nothing.
//	DATABASE_URL=... MFA_ENCRYPTION_KEY=... \
//	    go run ./cmd/encrypt-ldap-passwords --dry-run
//
//	# Real run, all tenants.
//	DATABASE_URL=... MFA_ENCRYPTION_KEY=... \
//	    go run ./cmd/encrypt-ldap-passwords
//
//	# Restrict to a single tenant (operational backfill on a per-account basis).
//	DATABASE_URL=... MFA_ENCRYPTION_KEY=... \
//	    go run ./cmd/encrypt-ldap-passwords --tenant 42
//
//	# Cap the number of rows processed in this run (rolling backfill).
//	DATABASE_URL=... MFA_ENCRYPTION_KEY=... \
//	    go run ./cmd/encrypt-ldap-passwords --limit 100
//
// Sequencing:
//
//   - Ship this cmd. Operator runs it against production. Verify second
//     run reports 0 rows ("✓ no rows need encryption").
//   - A subsequent migration (000050) drops the plaintext
//     `ldap_bind_password` column. That migration MUST run AFTER this
//     cmd, AFTER the operator has confirmed zero plaintext-only rows
//     remain, AND AFTER the read-side stops referring to the plaintext
//     column. Two-release sequence — do not bundle.
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

	"github.com/EduThemes/paper-lms/internal/auth"
	"github.com/EduThemes/paper-lms/internal/domain/models"
)

func main() { os.Exit(run()) }

func run() int {
	tenantFilter := flag.Uint("tenant", 0, "if non-zero, only encrypt rows for this account_id")
	limit := flag.Int("limit", 0, "if non-zero, cap rows processed in this run (rolling backfill)")
	dry := flag.Bool("dry-run", false, "list rows that would be encrypted; write nothing")
	flag.Parse()

	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL not set")
		return 2
	}

	// Encryption key is required even for --dry-run so we fail fast at
	// startup instead of after walking the table. Mirrors the
	// fail-fast behavior in cmd/server.
	if err := auth.EnsureKeysLoaded(); err != nil {
		fmt.Fprintf(os.Stderr, "encryption key not available: %v\n", err)
		return 2
	}

	db, err := gorm.Open(gormpg.Open(dbURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Printf("open database: %v", err)
		return 2
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	rows, err := selectBackfillCandidates(ctx, db, *tenantFilter, *limit)
	if err != nil {
		log.Printf("list candidates: %v", err)
		return 2
	}

	if len(rows) == 0 {
		log.Println("✓ no rows need encryption — every LDAP provider already has an encrypted bind password")
		return 0
	}

	log.Printf("found %d row(s) needing encryption", len(rows))

	written := 0
	skipped := 0
	for _, row := range rows {
		if *dry {
			log.Printf("[dry] would encrypt provider id=%d account_id=%d", row.ID, row.AccountID)
			continue
		}

		ciphertext, err := auth.Encrypt([]byte(row.LDAPBindPassword))
		if err != nil {
			log.Printf("✗ provider id=%d: encrypt failed: %v", row.ID, err)
			skipped++
			continue
		}

		// Targeted UPDATE — we set only the ciphertext column, leaving
		// the plaintext column intact for one release. The plaintext
		// column is dropped in migration 000050 in a later PR.
		if err := db.WithContext(ctx).
			Model(&models.AuthenticationProvider{}).
			Where("id = ?", row.ID).
			Update("ldap_bind_password_encrypted", ciphertext).Error; err != nil {
			log.Printf("✗ provider id=%d: update failed: %v", row.ID, err)
			skipped++
			continue
		}
		written++
		log.Printf("✓ encrypted provider id=%d account_id=%d", row.ID, row.AccountID)
	}

	if *dry {
		log.Printf("[dry-run] %d row(s) would be written; nothing changed", len(rows))
		return 0
	}
	log.Printf("done — %d encrypted, %d skipped", written, skipped)
	if skipped > 0 {
		return 1
	}
	return 0
}

// selectBackfillCandidates returns every active authentication_providers
// row where the plaintext bind password is set AND the encrypted column
// is still NULL. The query is the idempotency guarantee: re-running
// after a successful pass returns zero rows.
//
// account_id and limit are optional bounds for operator-controlled
// rolling backfills. Result ordering by id is stable for deterministic
// logs across runs.
func selectBackfillCandidates(ctx context.Context, db *gorm.DB, accountID uint, limit int) ([]models.AuthenticationProvider, error) {
	q := db.WithContext(ctx).
		Model(&models.AuthenticationProvider{}).
		Where("auth_type = ?", "ldap").
		Where("workflow_state != ?", "deleted").
		Where("ldap_bind_password <> ''").
		Where("ldap_bind_password_encrypted IS NULL").
		Order("id ASC")

	if accountID != 0 {
		q = q.Where("account_id = ?", accountID)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}

	var rows []models.AuthenticationProvider
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
