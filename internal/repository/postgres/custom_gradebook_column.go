package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CustomGradebookColumnRepository ----------------------------------------------------

type CustomGradebookColumnRepository interface {
	Create(ctx context.Context, col *models.CustomGradebookColumn) error
	FindByID(ctx context.Context, id uint) (*models.CustomGradebookColumn, error)
	Update(ctx context.Context, col *models.CustomGradebookColumn) error
	Delete(ctx context.Context, id uint) error
	ListByCourse(ctx context.Context, courseID uint, includeHidden bool) ([]models.CustomGradebookColumn, error)
	Reorder(ctx context.Context, courseID uint, ids []uint) error
	NextPosition(ctx context.Context, courseID uint) (int, error)
}

type customGradebookColumnRepo struct{ db *gorm.DB }

func NewCustomGradebookColumnRepository(db *gorm.DB) CustomGradebookColumnRepository {
	return &customGradebookColumnRepo{db: db}
}

func (r *customGradebookColumnRepo) Create(ctx context.Context, col *models.CustomGradebookColumn) error {
	return r.db.WithContext(ctx).Create(col).Error
}

func (r *customGradebookColumnRepo) FindByID(ctx context.Context, id uint) (*models.CustomGradebookColumn, error) {
	var col models.CustomGradebookColumn
	if err := r.db.WithContext(ctx).First(&col, id).Error; err != nil {
		return nil, err
	}
	return &col, nil
}

func (r *customGradebookColumnRepo) Update(ctx context.Context, col *models.CustomGradebookColumn) error {
	return r.db.WithContext(ctx).Save(col).Error
}

func (r *customGradebookColumnRepo) Delete(ctx context.Context, id uint) error {
	// Soft delete via workflow_state, mirroring Canvas conventions.
	return r.db.WithContext(ctx).
		Model(&models.CustomGradebookColumn{}).
		Where("id = ?", id).
		Update("workflow_state", "deleted").Error
}

func (r *customGradebookColumnRepo) ListByCourse(ctx context.Context, courseID uint, includeHidden bool) ([]models.CustomGradebookColumn, error) {
	var cols []models.CustomGradebookColumn
	q := r.db.WithContext(ctx).
		Where("course_id = ? AND workflow_state <> ?", courseID, "deleted")
	if !includeHidden {
		q = q.Where("hidden = ?", false)
	}
	if err := q.Order("position ASC, id ASC").Find(&cols).Error; err != nil {
		return nil, err
	}
	return cols, nil
}

func (r *customGradebookColumnRepo) Reorder(ctx context.Context, courseID uint, ids []uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, id := range ids {
			if err := tx.Model(&models.CustomGradebookColumn{}).
				Where("id = ? AND course_id = ?", id, courseID).
				Update("position", i+1).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *customGradebookColumnRepo) NextPosition(ctx context.Context, courseID uint) (int, error) {
	var max int
	row := r.db.WithContext(ctx).
		Model(&models.CustomGradebookColumn{}).
		Where("course_id = ? AND workflow_state <> ?", courseID, "deleted").
		Select("COALESCE(MAX(position), 0)").Row()
	if err := row.Scan(&max); err != nil {
		return 0, err
	}
	return max + 1, nil
}

// CustomColumnDatumRepository --------------------------------------------------------

type CustomColumnDatumRepository interface {
	ListByColumnID(ctx context.Context, columnID uint) ([]models.CustomColumnDatum, error)
	ListByCourse(ctx context.Context, columnIDs []uint) ([]models.CustomColumnDatum, error)
	FindByColumnAndUser(ctx context.Context, columnID, userID uint) (*models.CustomColumnDatum, error)
	Upsert(ctx context.Context, datum *models.CustomColumnDatum) error
	BulkUpsert(ctx context.Context, data []models.CustomColumnDatum) error
	DeleteByColumnID(ctx context.Context, columnID uint) error
}

type customColumnDatumRepo struct{ db *gorm.DB }

func NewCustomColumnDatumRepository(db *gorm.DB) CustomColumnDatumRepository {
	return &customColumnDatumRepo{db: db}
}

func (r *customColumnDatumRepo) ListByColumnID(ctx context.Context, columnID uint) ([]models.CustomColumnDatum, error) {
	var out []models.CustomColumnDatum
	if err := r.db.WithContext(ctx).
		Where("custom_gradebook_column_id = ?", columnID).
		Order("user_id ASC").
		Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func (r *customColumnDatumRepo) ListByCourse(ctx context.Context, columnIDs []uint) ([]models.CustomColumnDatum, error) {
	if len(columnIDs) == 0 {
		return []models.CustomColumnDatum{}, nil
	}
	var out []models.CustomColumnDatum
	if err := r.db.WithContext(ctx).
		Where("custom_gradebook_column_id IN ?", columnIDs).
		Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func (r *customColumnDatumRepo) FindByColumnAndUser(ctx context.Context, columnID, userID uint) (*models.CustomColumnDatum, error) {
	var d models.CustomColumnDatum
	if err := r.db.WithContext(ctx).
		Where("custom_gradebook_column_id = ? AND user_id = ?", columnID, userID).
		First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *customColumnDatumRepo) Upsert(ctx context.Context, datum *models.CustomColumnDatum) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "custom_gradebook_column_id"}, {Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"content", "updated_at"}),
		}).
		Create(datum).Error
}

func (r *customColumnDatumRepo) BulkUpsert(ctx context.Context, data []models.CustomColumnDatum) error {
	if len(data) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "custom_gradebook_column_id"}, {Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"content", "updated_at"}),
		}).
		CreateInBatches(data, 200).Error
}

func (r *customColumnDatumRepo) DeleteByColumnID(ctx context.Context, columnID uint) error {
	return r.db.WithContext(ctx).
		Where("custom_gradebook_column_id = ?", columnID).
		Delete(&models.CustomColumnDatum{}).Error
}
