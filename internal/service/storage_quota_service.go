package service

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// StorageQuotaService implements storage quotas (G120; Canvas: course,
// user and group storage_quota with account defaults). Usage is the sum of
// a context's live attachment sizes.
type StorageQuotaService struct {
	db *gorm.DB
}

// NewStorageQuotaService builds the service.
func NewStorageQuotaService(db *gorm.DB) *StorageQuotaService {
	return &StorageQuotaService{db: db}
}

// Built-in defaults (Canvas: 500 MB per course, 50 MB per user / group).
const (
	DefaultCourseQuotaMB int64 = 500
	DefaultUserQuotaMB   int64 = 50
	DefaultGroupQuotaMB  int64 = 50
)

// ErrStorageQuotaExceeded is returned when an upload would exceed the
// context's quota.
var ErrStorageQuotaExceeded = errors.New("storage quota exceeded")

// ErrQuotaContextNotFound is returned when the context is missing or in
// another tenant.
var ErrQuotaContextNotFound = errors.New("not found")

var accountDefaultKind = map[string]string{
	"Course": "AccountCourseDefault",
	"User":   "AccountUserDefault",
	"Group":  "AccountGroupDefault",
}

var builtinDefault = map[string]int64{
	"Course": DefaultCourseQuotaMB,
	"User":   DefaultUserQuotaMB,
	"Group":  DefaultGroupQuotaMB,
}

// contextAccount resolves the tenant of a Course, User or Group.
func (s *StorageQuotaService) contextAccount(ctx context.Context, contextType string, contextID uint) (uint, error) {
	var acct uint
	var err error
	switch contextType {
	case "Course":
		err = s.db.WithContext(ctx).Model(&models.Course{}).Where("id = ?", contextID).Pluck("account_id", &acct).Error
	case "User":
		err = s.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", contextID).Pluck("account_id", &acct).Error
	case "Group":
		err = s.db.WithContext(ctx).Raw(`
			SELECT COALESCE(c.account_id, gc.account_id, 0)
			FROM groups g
			JOIN group_categories gc ON gc.id = g.group_category_id
			LEFT JOIN courses c ON g.context_type = 'Course' AND c.id = g.context_id
			WHERE g.id = ?`, contextID).Scan(&acct).Error
	default:
		return 0, ErrQuotaContextNotFound
	}
	if err != nil || acct == 0 {
		return 0, ErrQuotaContextNotFound
	}
	return acct, nil
}

func (s *StorageQuotaService) quotaRow(ctx context.Context, contextType string, contextID uint) (*models.StorageQuota, error) {
	var q models.StorageQuota
	err := s.db.WithContext(ctx).Where("context_type = ? AND context_id = ?", contextType, contextID).First(&q).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &q, err
}

// AccountDefaults returns an account's default quotas (MB) per context type.
func (s *StorageQuotaService) AccountDefaults(ctx context.Context, accountID uint) (map[string]int64, error) {
	out := map[string]int64{}
	for ct, kind := range accountDefaultKind {
		row, err := s.quotaRow(ctx, kind, accountID)
		if err != nil {
			return nil, err
		}
		if row != nil {
			out[ct] = row.QuotaMB
		} else {
			out[ct] = builtinDefault[ct]
		}
	}
	return out, nil
}

func (s *StorageQuotaService) upsert(ctx context.Context, accountID uint, contextType string, contextID uint, mb int64) error {
	row := models.StorageQuota{AccountID: accountID, ContextType: contextType, ContextID: contextID, QuotaMB: mb}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "context_type"}, {Name: "context_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"quota_mb", "updated_at"}),
	}).Create(&row).Error
}

// SetAccountDefaults updates an account's defaults; keys are Course, User,
// Group.
func (s *StorageQuotaService) SetAccountDefaults(ctx context.Context, accountID uint, defaults map[string]int64) error {
	for ct, mb := range defaults {
		kind, ok := accountDefaultKind[ct]
		if !ok {
			return fmt.Errorf("unknown quota type %q", ct)
		}
		if mb < 0 || mb > 10*1024*1024 {
			return errors.New("quota must be between 0 and 10485760 MB")
		}
		if err := s.upsert(ctx, accountID, kind, accountID, mb); err != nil {
			return err
		}
	}
	return nil
}

// QuotaInfo is a context's quota and usage.
type QuotaInfo struct {
	ContextType string `json:"context_type"`
	ContextID   uint   `json:"context_id"`
	QuotaMB     int64  `json:"quota_mb"`
	QuotaBytes  int64  `json:"quota"`
	UsedBytes   int64  `json:"quota_used"`
	Overridden  bool   `json:"overridden"` // a per-context quota is set
}

// Info returns the quota and usage of a context of accountID (0 =
// unscoped, internal callers only).
func (s *StorageQuotaService) Info(ctx context.Context, contextType string, contextID, accountID uint) (*QuotaInfo, error) {
	acct, err := s.contextAccount(ctx, contextType, contextID)
	if err != nil {
		return nil, err
	}
	if accountID != 0 && acct != accountID {
		return nil, ErrQuotaContextNotFound
	}
	info := &QuotaInfo{ContextType: contextType, ContextID: contextID}
	row, err := s.quotaRow(ctx, contextType, contextID)
	if err != nil {
		return nil, err
	}
	if row != nil {
		info.QuotaMB, info.Overridden = row.QuotaMB, true
	} else {
		defs, err := s.AccountDefaults(ctx, acct)
		if err != nil {
			return nil, err
		}
		info.QuotaMB = defs[contextType]
	}
	info.QuotaBytes = info.QuotaMB * 1024 * 1024
	used, err := s.Usage(ctx, contextType, contextID)
	if err != nil {
		return nil, err
	}
	info.UsedBytes = used
	return info, nil
}

// Usage sums a context's live attachments.
func (s *StorageQuotaService) Usage(ctx context.Context, contextType string, contextID uint) (int64, error) {
	var used int64
	err := s.db.WithContext(ctx).Model(&models.Attachment{}).
		Where("context_type = ? AND context_id = ? AND workflow_state <> ?", contextType, contextID, "deleted").
		Select("COALESCE(SUM(size), 0)").Scan(&used).Error
	return used, err
}

// SetContextQuota sets (mb >= 0) or clears (mb < 0) a per-context quota.
func (s *StorageQuotaService) SetContextQuota(ctx context.Context, contextType string, contextID, accountID uint, mb int64) (*QuotaInfo, error) {
	acct, err := s.contextAccount(ctx, contextType, contextID)
	if err != nil || (accountID != 0 && acct != accountID) {
		return nil, ErrQuotaContextNotFound
	}
	if mb < 0 {
		if err := s.db.WithContext(ctx).Where("context_type = ? AND context_id = ?", contextType, contextID).
			Delete(&models.StorageQuota{}).Error; err != nil {
			return nil, err
		}
	} else {
		if mb > 10*1024*1024 {
			return nil, errors.New("quota must be at most 10485760 MB")
		}
		if err := s.upsert(ctx, acct, contextType, contextID, mb); err != nil {
			return nil, err
		}
	}
	return s.Info(ctx, contextType, contextID, accountID)
}

// CheckQuota refuses an upload of addBytes that would push a Course or
// Group over its quota. User contexts are not enforced here: submission
// files live under the student's User context and Canvas never counts
// them against a quota.
func (s *StorageQuotaService) CheckQuota(ctx context.Context, contextType string, contextID uint, addBytes int64) error {
	if contextType != "Course" && contextType != "Group" {
		return nil
	}
	info, err := s.Info(ctx, contextType, contextID, 0)
	if err != nil {
		// Unknown context: let the upload's own validation decide.
		return nil
	}
	if info.UsedBytes+addBytes > info.QuotaBytes {
		return ErrStorageQuotaExceeded
	}
	return nil
}
