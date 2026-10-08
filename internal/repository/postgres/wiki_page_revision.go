package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type wikiPageRevisionRepo struct {
	db *gorm.DB
}

func NewWikiPageRevisionRepository(db *gorm.DB) repository.WikiPageRevisionRepository {
	return &wikiPageRevisionRepo{db: db}
}

func (r *wikiPageRevisionRepo) Create(ctx context.Context, revision *models.WikiPageRevision) error {
	return r.db.WithContext(ctx).Create(revision).Error
}

// CreateNext stores revision as the page's next revision, numbering it
// MAX(revision_number)+1. Two concurrent saves can pick the same number;
// the unique (wiki_page_id, revision_number) index rejects the loser,
// which retries with a fresh number.
func (r *wikiPageRevisionRepo) CreateNext(ctx context.Context, revision *models.WikiPageRevision) error {
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		var max int
		if err := r.db.WithContext(ctx).Model(&models.WikiPageRevision{}).
			Where("wiki_page_id = ?", revision.WikiPageID).
			Select("COALESCE(MAX(revision_number), 0)").Scan(&max).Error; err != nil {
			return err
		}
		revision.ID = 0
		revision.RevisionNumber = max + 1
		err := r.db.WithContext(ctx).Create(revision).Error
		if err == nil {
			return nil
		}
		if !isUniqueViolation(err) {
			return err
		}
		lastErr = err
	}
	return lastErr
}

func (r *wikiPageRevisionRepo) FindByID(ctx context.Context, id uint) (*models.WikiPageRevision, error) {
	var revision models.WikiPageRevision
	if err := r.db.WithContext(ctx).First(&revision, id).Error; err != nil {
		return nil, err
	}
	return &revision, nil
}

func (r *wikiPageRevisionRepo) ListByPageID(ctx context.Context, pageID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.WikiPageRevision], error) {
	var revisions []models.WikiPageRevision
	var count int64

	query := r.db.WithContext(ctx).Model(&models.WikiPageRevision{}).Where("wiki_page_id = ?", pageID)
	if err := query.Count(&count).Error; err != nil {
		return nil, err
	}

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("revision_number DESC").Find(&revisions).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.WikiPageRevision]{
		Items:      revisions,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *wikiPageRevisionRepo) GetLatestRevision(ctx context.Context, pageID uint) (*models.WikiPageRevision, error) {
	var revision models.WikiPageRevision
	if err := r.db.WithContext(ctx).Where("wiki_page_id = ?", pageID).Order("revision_number DESC").First(&revision).Error; err != nil {
		return nil, err
	}
	return &revision, nil
}

func (r *wikiPageRevisionRepo) GetRevisionByNumber(ctx context.Context, pageID uint, revisionNumber int) (*models.WikiPageRevision, error) {
	var revision models.WikiPageRevision
	if err := r.db.WithContext(ctx).Where("wiki_page_id = ? AND revision_number = ?", pageID, revisionNumber).First(&revision).Error; err != nil {
		return nil, err
	}
	return &revision, nil
}
