package postgres

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type designColourSchemeRepo struct {
	db *gorm.DB
}

// NewDesignColourSchemeRepository returns the Page Designer saved-palette
// store.
func NewDesignColourSchemeRepository(db *gorm.DB) repository.DesignColourSchemeRepository {
	return &designColourSchemeRepo{db: db}
}

func (r *designColourSchemeRepo) owned(ctx context.Context, userID, accountID uint) *gorm.DB {
	return r.db.WithContext(ctx).Model(&models.DesignColourScheme{}).
		Where("user_id = ? AND account_id = ?", userID, accountID)
}

func (r *designColourSchemeRepo) ListByUser(ctx context.Context, userID, accountID uint) ([]models.DesignColourScheme, error) {
	var out []models.DesignColourScheme
	err := r.owned(ctx, userID, accountID).Order("created_at ASC, id ASC").Find(&out).Error
	return out, err
}

func (r *designColourSchemeRepo) FindByID(ctx context.Context, id, userID, accountID uint) (*models.DesignColourScheme, error) {
	var s models.DesignColourScheme
	if err := r.owned(ctx, userID, accountID).Where("id = ?", id).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *designColourSchemeRepo) Create(ctx context.Context, scheme *models.DesignColourScheme, limit int) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the owner's user row so two concurrent saves can't both
		// pass the count check.
		if err := tx.Exec(`SELECT id FROM users WHERE id = ? FOR UPDATE`, scheme.UserID).Error; err != nil {
			return err
		}
		var n int64
		if err := tx.Model(&models.DesignColourScheme{}).
			Where("user_id = ? AND account_id = ?", scheme.UserID, scheme.AccountID).
			Count(&n).Error; err != nil {
			return err
		}
		if limit > 0 && n >= int64(limit) {
			return repository.ErrColourSchemeLimit
		}
		return tx.Create(scheme).Error
	})
}

func (r *designColourSchemeRepo) Update(ctx context.Context, scheme *models.DesignColourScheme) error {
	scheme.UpdatedAt = time.Now()
	res := r.owned(ctx, scheme.UserID, scheme.AccountID).Where("id = ?", scheme.ID).
		Updates(map[string]interface{}{"name": scheme.Name, "palette": scheme.Palette, "updated_at": scheme.UpdatedAt})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *designColourSchemeRepo) Delete(ctx context.Context, id, userID, accountID uint) error {
	res := r.owned(ctx, userID, accountID).Where("id = ?", id).Delete(&models.DesignColourScheme{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
