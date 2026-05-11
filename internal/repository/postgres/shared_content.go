package postgres

import (
	"context"
	"errors"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type sharedContentRepo struct {
	db *gorm.DB
}

// NewSharedContentRepository returns a GORM-backed implementation of
// SharedContentRepository for the Commons content library.
func NewSharedContentRepository(db *gorm.DB) repository.SharedContentRepository {
	return &sharedContentRepo{db: db}
}

func (r *sharedContentRepo) Create(ctx context.Context, item *models.SharedContent) error {
	if item.Tags == "" {
		item.Tags = "[]"
	}
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *sharedContentRepo) FindByID(ctx context.Context, id uint) (*models.SharedContent, error) {
	var item models.SharedContent
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *sharedContentRepo) Update(ctx context.Context, item *models.SharedContent) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *sharedContentRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.SharedContent{}, id).Error
}

func (r *sharedContentRepo) ListByAccount(ctx context.Context, accountID uint, filters repository.SharedContentFilters, params repository.PaginationParams) (*repository.PaginatedResult[models.SharedContent], error) {
	var items []models.SharedContent
	var totalCount int64

	query := r.db.WithContext(ctx).Model(&models.SharedContent{}).Where("account_id = ?", accountID)

	if filters.ResourceType != "" {
		query = query.Where("resource_type = ?", filters.ResourceType)
	}
	if filters.Subject != "" {
		query = query.Where("subject = ?", filters.Subject)
	}
	if filters.GradeLevel != "" {
		query = query.Where("grade_level = ?", filters.GradeLevel)
	}
	if filters.AuthorUserID != 0 {
		query = query.Where("author_user_id = ?", filters.AuthorUserID)
	}
	if s := strings.TrimSpace(filters.Search); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		query = query.Where("LOWER(title) LIKE ? OR LOWER(description) LIKE ?", like, like)
	}

	if err := query.Count(&totalCount).Error; err != nil {
		return nil, err
	}

	if params.Page < 1 {
		params.Page = 1
	}
	if params.PerPage < 1 {
		params.PerPage = 20
	}
	offset := (params.Page - 1) * params.PerPage

	if err := query.Order("created_at DESC").Offset(offset).Limit(params.PerPage).Find(&items).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.SharedContent]{
		Items:      items,
		TotalCount: totalCount,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *sharedContentRepo) IncrementDownloadCount(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.SharedContent{}).
		Where("id = ?", id).
		UpdateColumn("download_count", gorm.Expr("download_count + 1")).Error
}

// ToggleFavorite flips a user's favorite for a Commons item. Returns the
// new favorited state. The favorite_count column on shared_content is
// updated atomically alongside.
func (r *sharedContentRepo) ToggleFavorite(ctx context.Context, sharedContentID, userID uint) (bool, error) {
	var favorited bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing models.SharedContentFavorite
		err := tx.Where("shared_content_id = ? AND user_id = ?", sharedContentID, userID).First(&existing).Error
		if err == nil {
			// Unfavorite
			if delErr := tx.Delete(&existing).Error; delErr != nil {
				return delErr
			}
			if upErr := tx.Model(&models.SharedContent{}).
				Where("id = ? AND favorite_count > 0", sharedContentID).
				UpdateColumn("favorite_count", gorm.Expr("favorite_count - 1")).Error; upErr != nil {
				return upErr
			}
			favorited = false
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		// Favorite
		fav := &models.SharedContentFavorite{SharedContentID: sharedContentID, UserID: userID}
		if createErr := tx.Create(fav).Error; createErr != nil {
			return createErr
		}
		if upErr := tx.Model(&models.SharedContent{}).
			Where("id = ?", sharedContentID).
			UpdateColumn("favorite_count", gorm.Expr("favorite_count + 1")).Error; upErr != nil {
			return upErr
		}
		favorited = true
		return nil
	})
	return favorited, err
}

func (r *sharedContentRepo) IsFavorited(ctx context.Context, sharedContentID, userID uint) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.SharedContentFavorite{}).
		Where("shared_content_id = ? AND user_id = ?", sharedContentID, userID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *sharedContentRepo) ListUserFavorites(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.SharedContent], error) {
	var items []models.SharedContent
	var totalCount int64

	base := r.db.WithContext(ctx).Model(&models.SharedContent{}).
		Joins("JOIN shared_content_favorites f ON f.shared_content_id = shared_content.id").
		Where("f.user_id = ?", userID)

	if err := base.Count(&totalCount).Error; err != nil {
		return nil, err
	}

	if params.Page < 1 {
		params.Page = 1
	}
	if params.PerPage < 1 {
		params.PerPage = 20
	}
	offset := (params.Page - 1) * params.PerPage

	if err := base.Order("f.created_at DESC").Offset(offset).Limit(params.PerPage).Find(&items).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.SharedContent]{
		Items:      items,
		TotalCount: totalCount,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
