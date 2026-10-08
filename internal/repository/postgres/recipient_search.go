package postgres

import (
	"context"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"gorm.io/gorm"
)

// RecipientSearchRepository finds the users a caller may address in the
// inbox. Canvas's /search/recipients scopes candidates to people who share
// a context (course) with the sender; account admins may address anyone in
// their account.
type RecipientSearchRepository struct {
	db *gorm.DB
}

// NewRecipientSearchRepository creates a postgres-backed recipient search.
func NewRecipientSearchRepository(db *gorm.DB) *RecipientSearchRepository {
	return &RecipientSearchRepository{db: db}
}

// escapeLike escapes LIKE metacharacters so a search term is matched
// literally.
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// SearchRecipients returns up to limit non-suspended users of accountID
// (excluding the caller) whose name or sortable name contains term
// (case-insensitive; admins, allInAccount, also match on email). Unless
// allInAccount is set, candidates are limited to
// users who share an active enrollment in a course of that account with
// the caller.
func (r *RecipientSearchRepository) SearchRecipients(ctx context.Context, callerID, accountID uint, term string, allInAccount bool, limit int) ([]models.User, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	pattern := "%" + escapeLike(strings.ToLower(strings.TrimSpace(term))) + "%"

	q := r.db.WithContext(ctx).Model(&models.User{}).
		Where("users.account_id = ? AND users.id <> ? AND users.suspended = ?", accountID, callerID, false)

	if allInAccount {
		// Admins may look people up by email address too.
		q = q.Where(`(LOWER(users.name) LIKE ? ESCAPE '\' OR LOWER(COALESCE(users.sortable_name, '')) LIKE ? ESCAPE '\' OR LOWER(users.email) LIKE ? ESCAPE '\')`, pattern, pattern, pattern)
	} else {
		// Names only: matching on email would let a student confirm
		// which classmate owns a given address even though results
		// never include emails.
		q = q.Where(`(LOWER(users.name) LIKE ? ESCAPE '\' OR LOWER(COALESCE(users.sortable_name, '')) LIKE ? ESCAPE '\')`, pattern, pattern)
	}

	if !allInAccount {
		// The address book: people who share an active course or an
		// accepted group membership with the caller (address_book.go).
		q = q.Where("(users.id IN ("+coursePeersSQL+") OR users.id IN ("+groupPeersSQL+"))", callerID, accountID, callerID, accountID, accountID)
	}

	var users []models.User
	if err := q.Order("users.sortable_name ASC, users.name ASC, users.id ASC").Limit(limit).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}
