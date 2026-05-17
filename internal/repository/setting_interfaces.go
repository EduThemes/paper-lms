package repository

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// ErrSettingNotFound is returned by SettingRepository.FindByScope when
// no row exists at the requested (scope_type, scope_id, key). The
// service layer treats this as "fall through to the next scope" rather
// than a hard error, so callers MUST `errors.Is(err, ErrSettingNotFound)`
// rather than `err != nil`.
var ErrSettingNotFound = errors.New("setting not found")

// SettingRepository persists the rows backing the Super-Admin Settings
// Engine. Resolution-chain walking lives one layer up in
// internal/service/settings — this interface is just durable storage
// for individual (scope, key) entries.
type SettingRepository interface {
	// FindByScope returns the single row at (scope_type, scope_id, key),
	// or ErrSettingNotFound if absent. The service walks the scope
	// chain by calling this once per candidate scope.
	FindByScope(ctx context.Context, scopeType string, scopeID uint, key string) (*models.Setting, error)

	// ListByScope returns every setting bound to one (scope_type,
	// scope_id) tuple. Used by the GetEffective path to bulk-load a
	// scope's settings before the service merges them with parent
	// scopes' values.
	ListByScope(ctx context.Context, scopeType string, scopeID uint) ([]models.Setting, error)

	// Upsert writes setting.ValuePlain / ValueEncrypted / ValueType /
	// UpdatedBy / UpdatedAt at (ScopeType, ScopeID, Key). On conflict
	// it overwrites — the natural-key collision is the settings_scope_unique
	// constraint defined in migration 000057.
	Upsert(ctx context.Context, setting *models.Setting) error

	// Delete removes the row at (scope_type, scope_id, key). Returns
	// nil whether or not a row existed — "clear" is idempotent at the
	// service layer.
	Delete(ctx context.Context, scopeType string, scopeID uint, key string) error
}
