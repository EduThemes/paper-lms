package service

import (
	"context"
	"fmt"
	"log"

	"gorm.io/gorm"
)

// UserDeletionService walks dependent tables and erases PII columns
// when a user is deleted under FERPA / state-DPA rules.
//
// Phase 13.3 (full): the user-row anonymization in FERPAService.ProcessDeletion
// is not sufficient on its own — dependent tables still hold user-authored
// PII (submission bodies, comments, conversation messages, discussion
// posts, attendance notes, notification delivery metadata).
//
// This service walks each dependent table in a single Postgres transaction,
// UPDATE-ing PII columns to NULL or a placeholder. It deliberately does
// NOT touch audit-trail tables (audit_logs, grade_change_logs,
// pii_access_logs, data_export_requests, data_deletion_requests) which
// FERPA and state DPAs require to be append-only.
//
// Cross-row identifiers like submissions.user_id, submissions.grader_id,
// submission_comments.author_id are intentionally preserved — they are
// needed for academic-record reconstruction and audit attribution. Only
// the PII payload columns (free text the user authored) are erased.
type UserDeletionService struct {
	db *gorm.DB
}

// NewUserDeletionService constructs a UserDeletionService.
func NewUserDeletionService(db *gorm.DB) *UserDeletionService {
	return &UserDeletionService{db: db}
}

// EraseDependents walks all dependent tables for the given user and
// nulls/replaces PII columns inside a single transaction. Returns a
// map of {table_name: rows_affected} for stitching into the deletion
// log. If a table or column does not exist in the current schema the
// step is logged and skipped — the function never invents column names.
//
// Tables touched (UPDATE only; no DELETE because grade-bearing FKs are
// ON DELETE RESTRICT per migration 000054):
//
//	submissions             body, url, attachments → NULL (score/grade/grader preserved)
//	submission_comments     comment → NULL (author_id preserved for attribution)
//	conversation_messages   body → '[deleted]'
//	discussion_entries      message → '[deleted]'
//	attendance_records      notes → NULL (status/marked_by_id preserved)
//	notification_deliveries address, subject, body → NULL
//
// The following are EXPLICITLY NOT touched (audit trail):
//
//	audit_logs                — append-only, migration 000053
//	grade_change_logs         — immutable history
//	pii_access_logs           — required for FERPA audit
//	data_export_requests      — paper trail
//	data_deletion_requests    — paper trail
func (s *UserDeletionService) EraseDependents(ctx context.Context, userID uint) (map[string]int, error) {
	if s.db == nil {
		return nil, fmt.Errorf("user deletion service: nil db handle")
	}
	if userID == 0 {
		return nil, fmt.Errorf("user deletion service: user_id is required")
	}

	rowsTouched := make(map[string]int, 6)

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return s.eraseDependentsTx(tx, userID, rowsTouched)
	})

	if err != nil {
		return nil, err
	}
	return rowsTouched, nil
}

// eraseDependentsTx is the actual per-table walk. Split out so tests
// can drive it against a non-transactional fake *gorm.DB; production
// always invokes it inside Transaction() above.
func (s *UserDeletionService) eraseDependentsTx(tx *gorm.DB, userID uint, rowsTouched map[string]int) error {
		// 1. submissions — null free-text PII; keep score/grade/grader_id
		//    for academic record integrity (FK is RESTRICT per 000054).
		//    TODO: S3 attachment objects keyed by submission ID need
		//    separate deletion outside the DB transaction.
		if n, err := updatePIIColumns(tx, "submissions", "user_id = ?", userID, map[string]interface{}{
			"body":        nil,
			"url":         nil,
			"attachments": nil,
		}); err != nil {
			return fmt.Errorf("submissions: %w", err)
		} else {
			rowsTouched["submissions"] = n
		}

		// 2. submission_comments — null the comment text; keep author_id
		//    for audit attribution. (Model column: author_id.)
		if n, err := updatePIIColumns(tx, "submission_comments", "author_id = ?", userID, map[string]interface{}{
			"comment": nil,
		}); err != nil {
			return fmt.Errorf("submission_comments: %w", err)
		} else {
			rowsTouched["submission_comments"] = n
		}

		// 3. conversation_messages — replace body with placeholder.
		//    NOTE: the model column is `user_id`, not `author_id` —
		//    using actual schema (see internal/domain/models/conversation_message.go).
		if n, err := updatePIIColumns(tx, "conversation_messages", "user_id = ?", userID, map[string]interface{}{
			"body": "[deleted]",
		}); err != nil {
			return fmt.Errorf("conversation_messages: %w", err)
		} else {
			rowsTouched["conversation_messages"] = n
		}

		// 4. discussion_entries — replace message with placeholder.
		if n, err := updatePIIColumns(tx, "discussion_entries", "user_id = ?", userID, map[string]interface{}{
			"message": "[deleted]",
		}); err != nil {
			return fmt.Errorf("discussion_entries: %w", err)
		} else {
			rowsTouched["discussion_entries"] = n
		}

		// 5. attendance_records — null notes; keep status/marked_by_id.
		//    NOTE: the model column is `user_id`, not `student_user_id` —
		//    using actual schema (see internal/domain/models/attendance.go).
		if n, err := updatePIIColumns(tx, "attendance_records", "user_id = ?", userID, map[string]interface{}{
			"notes": nil,
		}); err != nil {
			return fmt.Errorf("attendance_records: %w", err)
		} else {
			rowsTouched["attendance_records"] = n
		}

		// 6. notification_deliveries — null address/subject/body.
		//    NOTE: the model column is `user_id`, not `recipient_user_id` —
		//    using actual schema (see internal/domain/models/notification_delivery.go).
		if n, err := updatePIIColumns(tx, "notification_deliveries", "user_id = ?", userID, map[string]interface{}{
			"address": nil,
			"subject": nil,
			"body":    nil,
		}); err != nil {
			return fmt.Errorf("notification_deliveries: %w", err)
		} else {
			rowsTouched["notification_deliveries"] = n
		}

		return nil
}

// updatePIIColumns runs a single UPDATE on a dependent table, guarding
// against missing tables/columns: if Migrator says the table is absent
// it logs a warning and reports zero rows touched rather than erroring.
// This keeps the deletion path idempotent across schema versions.
func updatePIIColumns(tx *gorm.DB, table string, whereSQL string, whereArg interface{}, updates map[string]interface{}) (int, error) {
	// Best-effort schema guard: if the dialector supports a real
	// Migrator (Postgres in production), skip tables / columns that
	// were dropped by a downstream migration so user deletion stays
	// idempotent. Test dialectors (e.g. DummyDialector) return nil
	// from Migrator() — fall through and just execute the UPDATE.
	migrator := tx.Migrator()
	filtered := updates
	if migrator != nil {
		if !migrator.HasTable(table) {
			log.Printf("user_deletion_service: table %q not present, skipping", table)
			return 0, nil
		}
		filtered = make(map[string]interface{}, len(updates))
		for col, val := range updates {
			if migrator.HasColumn(table, col) {
				filtered[col] = val
			} else {
				log.Printf("user_deletion_service: column %q.%q not present, skipping", table, col)
			}
		}
		if len(filtered) == 0 {
			return 0, nil
		}
	}
	result := tx.Table(table).Where(whereSQL, whereArg).Updates(filtered)
	if result.Error != nil {
		return 0, result.Error
	}
	return int(result.RowsAffected), nil
}
