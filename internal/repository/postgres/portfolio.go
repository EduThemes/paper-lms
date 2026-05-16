package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// PortfolioRepository
// ---------------------------------------------------------------------------

type portfolioRepo struct {
	db *gorm.DB
}

func NewPortfolioRepository(db *gorm.DB) repository.PortfolioRepository {
	return &portfolioRepo{db: db}
}

func (r *portfolioRepo) Create(ctx context.Context, portfolio *models.Portfolio) error {
	return r.db.WithContext(ctx).Create(portfolio).Error
}

func (r *portfolioRepo) FindByID(ctx context.Context, id uint) (*models.Portfolio, error) {
	var portfolio models.Portfolio
	if err := r.db.WithContext(ctx).First(&portfolio, id).Error; err != nil {
		return nil, err
	}
	return &portfolio, nil
}

func (r *portfolioRepo) FindBySlug(ctx context.Context, slug string) (*models.Portfolio, error) {
	var portfolio models.Portfolio
	if err := r.db.WithContext(ctx).Where("slug = ? AND workflow_state != ?", slug, "deleted").First(&portfolio).Error; err != nil {
		return nil, err
	}
	return &portfolio, nil
}

func (r *portfolioRepo) FindByPublicURL(ctx context.Context, publicURL string) (*models.Portfolio, error) {
	var portfolio models.Portfolio
	if err := r.db.WithContext(ctx).Where("public_url = ? AND workflow_state = ?", publicURL, "published").First(&portfolio).Error; err != nil {
		return nil, err
	}
	return &portfolio, nil
}

func (r *portfolioRepo) Update(ctx context.Context, portfolio *models.Portfolio) error {
	return r.db.WithContext(ctx).Save(portfolio).Error
}

func (r *portfolioRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.Portfolio{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *portfolioRepo) ListByUserID(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Portfolio], error) {
	var portfolios []models.Portfolio
	var count int64

	query := r.db.WithContext(ctx).Model(&models.Portfolio{}).Where("user_id = ? AND workflow_state != ?", userID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("updated_at DESC").Find(&portfolios).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Portfolio]{
		Items:      portfolios,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *portfolioRepo) ListPublic(ctx context.Context, params repository.PaginationParams) (*repository.PaginatedResult[models.Portfolio], error) {
	var portfolios []models.Portfolio
	var count int64

	query := r.db.WithContext(ctx).Model(&models.Portfolio{}).Where("is_public = ? AND workflow_state = ?", true, "published")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("view_count DESC").Find(&portfolios).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Portfolio]{
		Items:      portfolios,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *portfolioRepo) IncrementViewCount(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.Portfolio{}).Where("id = ?", id).UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
}

// ---------------------------------------------------------------------------
// PortfolioSectionRepository
// ---------------------------------------------------------------------------

type portfolioSectionRepo struct {
	db *gorm.DB
}

func NewPortfolioSectionRepository(db *gorm.DB) repository.PortfolioSectionRepository {
	return &portfolioSectionRepo{db: db}
}

func (r *portfolioSectionRepo) Create(ctx context.Context, section *models.PortfolioSection) error {
	return r.db.WithContext(ctx).Create(section).Error
}

func (r *portfolioSectionRepo) FindByID(ctx context.Context, id uint) (*models.PortfolioSection, error) {
	var section models.PortfolioSection
	if err := r.db.WithContext(ctx).First(&section, id).Error; err != nil {
		return nil, err
	}
	return &section, nil
}

func (r *portfolioSectionRepo) FindByIDs(ctx context.Context, ids []uint) ([]models.PortfolioSection, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var sections []models.PortfolioSection
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&sections).Error; err != nil {
		return nil, err
	}
	return sections, nil
}

func (r *portfolioSectionRepo) Update(ctx context.Context, section *models.PortfolioSection) error {
	return r.db.WithContext(ctx).Save(section).Error
}

func (r *portfolioSectionRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.PortfolioSection{}, id).Error
}

func (r *portfolioSectionRepo) ListByPortfolioID(ctx context.Context, portfolioID uint) ([]models.PortfolioSection, error) {
	var sections []models.PortfolioSection
	if err := r.db.WithContext(ctx).Where("portfolio_id = ?", portfolioID).Order("position ASC").Find(&sections).Error; err != nil {
		return nil, err
	}
	return sections, nil
}

// ---------------------------------------------------------------------------
// PortfolioArtifactRepository
// ---------------------------------------------------------------------------

type portfolioArtifactRepo struct {
	db *gorm.DB
}

func NewPortfolioArtifactRepository(db *gorm.DB) repository.PortfolioArtifactRepository {
	return &portfolioArtifactRepo{db: db}
}

func (r *portfolioArtifactRepo) Create(ctx context.Context, artifact *models.PortfolioArtifact) error {
	return r.db.WithContext(ctx).Create(artifact).Error
}

func (r *portfolioArtifactRepo) FindByID(ctx context.Context, id uint) (*models.PortfolioArtifact, error) {
	var artifact models.PortfolioArtifact
	if err := r.db.WithContext(ctx).First(&artifact, id).Error; err != nil {
		return nil, err
	}
	return &artifact, nil
}

func (r *portfolioArtifactRepo) Update(ctx context.Context, artifact *models.PortfolioArtifact) error {
	return r.db.WithContext(ctx).Save(artifact).Error
}

func (r *portfolioArtifactRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.PortfolioArtifact{}, id).Error
}

func (r *portfolioArtifactRepo) ListByPortfolioID(ctx context.Context, portfolioID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PortfolioArtifact], error) {
	var artifacts []models.PortfolioArtifact
	var count int64

	query := r.db.WithContext(ctx).Model(&models.PortfolioArtifact{}).Where("portfolio_id = ?", portfolioID)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("position ASC").Find(&artifacts).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.PortfolioArtifact]{
		Items:      artifacts,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *portfolioArtifactRepo) ListBySectionID(ctx context.Context, sectionID uint) ([]models.PortfolioArtifact, error) {
	var artifacts []models.PortfolioArtifact
	if err := r.db.WithContext(ctx).Where("section_id = ?", sectionID).Order("position ASC").Find(&artifacts).Error; err != nil {
		return nil, err
	}
	return artifacts, nil
}

func (r *portfolioArtifactRepo) ListFeatured(ctx context.Context, portfolioID uint) ([]models.PortfolioArtifact, error) {
	var artifacts []models.PortfolioArtifact
	if err := r.db.WithContext(ctx).Where("portfolio_id = ? AND is_featured = ?", portfolioID, true).Order("position ASC").Find(&artifacts).Error; err != nil {
		return nil, err
	}
	return artifacts, nil
}

// ---------------------------------------------------------------------------
// PortfolioReflectionRepository
// ---------------------------------------------------------------------------

type portfolioReflectionRepo struct {
	db *gorm.DB
}

func NewPortfolioReflectionRepository(db *gorm.DB) repository.PortfolioReflectionRepository {
	return &portfolioReflectionRepo{db: db}
}

func (r *portfolioReflectionRepo) Create(ctx context.Context, reflection *models.PortfolioReflection) error {
	return r.db.WithContext(ctx).Create(reflection).Error
}

func (r *portfolioReflectionRepo) FindByID(ctx context.Context, id uint) (*models.PortfolioReflection, error) {
	var reflection models.PortfolioReflection
	if err := r.db.WithContext(ctx).First(&reflection, id).Error; err != nil {
		return nil, err
	}
	return &reflection, nil
}

func (r *portfolioReflectionRepo) Update(ctx context.Context, reflection *models.PortfolioReflection) error {
	return r.db.WithContext(ctx).Save(reflection).Error
}

func (r *portfolioReflectionRepo) ListByArtifactID(ctx context.Context, artifactID uint) ([]models.PortfolioReflection, error) {
	var reflections []models.PortfolioReflection
	if err := r.db.WithContext(ctx).Where("artifact_id = ?", artifactID).Order("created_at ASC").Find(&reflections).Error; err != nil {
		return nil, err
	}
	return reflections, nil
}

// ---------------------------------------------------------------------------
// PortfolioTemplateRepository
// ---------------------------------------------------------------------------

type portfolioTemplateRepo struct {
	db *gorm.DB
}

func NewPortfolioTemplateRepository(db *gorm.DB) repository.PortfolioTemplateRepository {
	return &portfolioTemplateRepo{db: db}
}

func (r *portfolioTemplateRepo) Create(ctx context.Context, template *models.PortfolioTemplate) error {
	return r.db.WithContext(ctx).Create(template).Error
}

func (r *portfolioTemplateRepo) FindByID(ctx context.Context, id, accountID uint) (*models.PortfolioTemplate, error) {
	var tmpl models.PortfolioTemplate
	q := r.db.WithContext(ctx)
	if accountID != 0 {
		// Templates are tenant-scoped, BUT a public template (is_public=true)
		// is intentionally cross-tenant readable for discoverability.
		q = q.Where("account_id = ? OR is_public = ?", accountID, true)
	}
	if err := q.First(&tmpl, id).Error; err != nil {
		return nil, err
	}
	return &tmpl, nil
}

func (r *portfolioTemplateRepo) Update(ctx context.Context, template *models.PortfolioTemplate) error {
	return r.db.WithContext(ctx).Save(template).Error
}

func (r *portfolioTemplateRepo) ListPublic(ctx context.Context, params repository.PaginationParams) (*repository.PaginatedResult[models.PortfolioTemplate], error) {
	var templates []models.PortfolioTemplate
	var count int64

	query := r.db.WithContext(ctx).Model(&models.PortfolioTemplate{}).Where("is_public = ?", true)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("usage_count DESC").Find(&templates).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.PortfolioTemplate]{
		Items:      templates,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *portfolioTemplateRepo) ListByAccountID(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PortfolioTemplate], error) {
	var templates []models.PortfolioTemplate
	var count int64

	query := r.db.WithContext(ctx).Model(&models.PortfolioTemplate{}).Where("account_id = ?", accountID)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at DESC").Find(&templates).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.PortfolioTemplate]{
		Items:      templates,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

// ---------------------------------------------------------------------------
// PortfolioCommentRepository
// ---------------------------------------------------------------------------

type portfolioCommentRepo struct {
	db *gorm.DB
}

func NewPortfolioCommentRepository(db *gorm.DB) repository.PortfolioCommentRepository {
	return &portfolioCommentRepo{db: db}
}

func (r *portfolioCommentRepo) Create(ctx context.Context, comment *models.PortfolioComment) error {
	return r.db.WithContext(ctx).Create(comment).Error
}

func (r *portfolioCommentRepo) FindByID(ctx context.Context, id uint) (*models.PortfolioComment, error) {
	var comment models.PortfolioComment
	if err := r.db.WithContext(ctx).First(&comment, id).Error; err != nil {
		return nil, err
	}
	return &comment, nil
}

func (r *portfolioCommentRepo) Update(ctx context.Context, comment *models.PortfolioComment) error {
	return r.db.WithContext(ctx).Save(comment).Error
}

func (r *portfolioCommentRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.PortfolioComment{}, id).Error
}

func (r *portfolioCommentRepo) ListByPortfolioID(ctx context.Context, portfolioID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PortfolioComment], error) {
	var comments []models.PortfolioComment
	var count int64

	query := r.db.WithContext(ctx).Model(&models.PortfolioComment{}).Where("portfolio_id = ?", portfolioID)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at ASC").Find(&comments).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.PortfolioComment]{
		Items:      comments,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *portfolioCommentRepo) ListByArtifactID(ctx context.Context, artifactID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PortfolioComment], error) {
	var comments []models.PortfolioComment
	var count int64

	query := r.db.WithContext(ctx).Model(&models.PortfolioComment{}).Where("artifact_id = ?", artifactID)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at ASC").Find(&comments).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.PortfolioComment]{
		Items:      comments,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
