package postgres

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type ltiAdvantageRepo struct {
	db *gorm.DB
}

// NewLTIAdvantageRepository returns the Postgres store for the LTI 1.3 /
// Advantage cross-table queries (resource links, line item filters, tool
// installs reachable from a course, module item lookups).
func NewLTIAdvantageRepository(db *gorm.DB) repository.LTIAdvantageRepository {
	return &ltiAdvantageRepo{db: db}
}

// courseTenantClause scopes a course_id column to the tenant. accountID 0
// is the auth-internal escape hatch (no tenant filter).
func courseTenantClause(q *gorm.DB, column string, accountID uint) *gorm.DB {
	if accountID == 0 {
		return q
	}
	return q.Where(column+" IN (SELECT id FROM courses WHERE account_id = ?)", accountID)
}

func (r *ltiAdvantageRepo) CreateResourceLink(ctx context.Context, link *models.LTIResourceLink) error {
	return r.db.WithContext(ctx).Create(link).Error
}

func (r *ltiAdvantageRepo) UpdateResourceLink(ctx context.Context, link *models.LTIResourceLink) error {
	return r.db.WithContext(ctx).Save(link).Error
}

func (r *ltiAdvantageRepo) findResourceLink(ctx context.Context, accountID uint, where string, args ...interface{}) (*models.LTIResourceLink, error) {
	var link models.LTIResourceLink
	q := r.db.WithContext(ctx).Where("context_type = ?", "Course").Where(where, args...)
	q = courseTenantClause(q, "context_id", accountID)
	if err := q.Order("id DESC").First(&link).Error; err != nil {
		return nil, err
	}
	return &link, nil
}

func (r *ltiAdvantageRepo) FindResourceLinkByUUID(ctx context.Context, uuid string, accountID uint) (*models.LTIResourceLink, error) {
	if uuid == "" {
		return nil, gorm.ErrRecordNotFound
	}
	return r.findResourceLink(ctx, accountID, "resource_link_id = ?", uuid)
}

func (r *ltiAdvantageRepo) FindResourceLinkByAssignment(ctx context.Context, assignmentID, accountID uint) (*models.LTIResourceLink, error) {
	return r.findResourceLink(ctx, accountID, "assignment_id = ?", assignmentID)
}

func (r *ltiAdvantageRepo) FindResourceLinkByContentTag(ctx context.Context, contentTagID, accountID uint) (*models.LTIResourceLink, error) {
	return r.findResourceLink(ctx, accountID, "content_tag_id = ?", contentTagID)
}

func (r *ltiAdvantageRepo) DeleteResourceLinksByAssignment(ctx context.Context, assignmentID, accountID uint) error {
	q := r.db.WithContext(ctx).Where("assignment_id = ? AND context_type = ?", assignmentID, "Course")
	q = courseTenantClause(q, "context_id", accountID)
	return q.Delete(&models.LTIResourceLink{}).Error
}

func (r *ltiAdvantageRepo) FindPlacementResourceLink(ctx context.Context, toolID, courseID uint, placement string) (*models.LTIResourceLink, error) {
	var link models.LTIResourceLink
	err := r.db.WithContext(ctx).
		Where("context_type = ? AND context_id = ? AND context_external_tool_id = ? AND placement = ?", "Course", courseID, toolID, placement).
		Where("assignment_id IS NULL AND content_tag_id IS NULL").
		Order("id ASC").First(&link).Error
	if err != nil {
		return nil, err
	}
	return &link, nil
}

func (r *ltiAdvantageRepo) ListLineItems(ctx context.Context, courseID uint, filter repository.LTILineItemFilter, params repository.PaginationParams) (*repository.PaginatedResult[models.LTILineItem], error) {
	q := r.db.WithContext(ctx).Model(&models.LTILineItem{}).Where("course_id = ?", courseID)
	if filter.ResourceLinkID != "" {
		q = q.Where("resource_link_id_str = ?", filter.ResourceLinkID)
	}
	if filter.ResourceID != "" {
		q = q.Where("resource_id = ?", filter.ResourceID)
	}
	if filter.Tag != "" {
		q = q.Where("tag = ?", filter.Tag)
	}
	if filter.DeveloperKeyID != nil {
		q = q.Where("developer_key_id = ?", *filter.DeveloperKeyID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return nil, err
	}
	page, perPage := params.Page, params.PerPage
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 10
	}
	var items []models.LTILineItem
	if err := q.Order("id ASC").Offset((page - 1) * perPage).Limit(perPage).Find(&items).Error; err != nil {
		return nil, err
	}
	return &repository.PaginatedResult[models.LTILineItem]{Items: items, TotalCount: count, Page: page, PerPage: perPage}, nil
}

// toolsForCourseQuery selects the active installs reachable from a course:
// those installed in the course itself and those installed at the
// course's tenant account. The course must belong to accountID when
// accountID != 0.
func (r *ltiAdvantageRepo) toolsForCourseQuery(ctx context.Context, courseID, accountID uint) *gorm.DB {
	q := r.db.WithContext(ctx).Model(&models.ContextExternalTool{}).
		Where("workflow_state = ?", "active").
		Where(`(context_type = 'Course' AND context_id = ?)
			OR (context_type = 'Account' AND context_id = (SELECT account_id FROM courses WHERE id = ?))`, courseID, courseID)
	if accountID != 0 {
		q = q.Where("EXISTS (SELECT 1 FROM courses WHERE id = ? AND account_id = ?)", courseID, accountID)
	}
	return q
}

func (r *ltiAdvantageRepo) ListActiveToolsForCourse(ctx context.Context, courseID, accountID uint) ([]models.ContextExternalTool, error) {
	var tools []models.ContextExternalTool
	// Course installs sort before account installs so a course-level
	// install of the same tool wins in callers that de-duplicate.
	err := r.toolsForCourseQuery(ctx, courseID, accountID).
		Order("CASE WHEN context_type = 'Course' THEN 0 ELSE 1 END, id ASC").
		Limit(500).Find(&tools).Error
	return tools, err
}

func (r *ltiAdvantageRepo) FindActiveToolForCourse(ctx context.Context, toolID, courseID, accountID uint) (*models.ContextExternalTool, error) {
	var tool models.ContextExternalTool
	if err := r.toolsForCourseQuery(ctx, courseID, accountID).Where("id = ?", toolID).First(&tool).Error; err != nil {
		return nil, err
	}
	return &tool, nil
}

func (r *ltiAdvantageRepo) FindActiveToolByDeveloperKeyForCourse(ctx context.Context, devKeyID, courseID, accountID uint) (*models.ContextExternalTool, error) {
	if devKeyID == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var tool models.ContextExternalTool
	err := r.toolsForCourseQuery(ctx, courseID, accountID).
		Where("developer_key_id = ?", devKeyID).
		Order("CASE WHEN context_type = 'Course' THEN 0 ELSE 1 END, id ASC").
		First(&tool).Error
	if err != nil {
		return nil, err
	}
	return &tool, nil
}

func (r *ltiAdvantageRepo) ListToolConfigurationsByAccount(ctx context.Context, accountID uint) ([]models.LTIToolConfiguration, error) {
	if accountID == 0 {
		return nil, errors.New("account id is required")
	}
	var configs []models.LTIToolConfiguration
	err := r.db.WithContext(ctx).
		Where(`developer_key_id IN (
			SELECT id FROM developer_keys
			WHERE account_id = ? AND is_lti_key = true AND workflow_state != 'deleted'
		)`, accountID).
		Order("id ASC").Find(&configs).Error
	return configs, err
}

func (r *ltiAdvantageRepo) FindContentTagInCourse(ctx context.Context, contentTagID, courseID, accountID uint) (*models.ContentTag, error) {
	var tag models.ContentTag
	q := r.db.WithContext(ctx).
		Where("id = ? AND workflow_state != ?", contentTagID, models.ContentTagDeleted).
		Where("context_module_id IN (SELECT id FROM context_modules WHERE course_id = ?)", courseID)
	if accountID != 0 {
		q = q.Where("EXISTS (SELECT 1 FROM courses WHERE id = ? AND account_id = ?)", courseID, accountID)
	}
	if err := q.First(&tag).Error; err != nil {
		return nil, err
	}
	return &tag, nil
}

func (r *ltiAdvantageRepo) NextModuleItemPosition(ctx context.Context, moduleID uint) (int, error) {
	var maxPos *int
	err := r.db.WithContext(ctx).Model(&models.ContentTag{}).
		Where("context_module_id = ? AND workflow_state != ?", moduleID, models.ContentTagDeleted).
		Select("MAX(position)").Scan(&maxPos).Error
	if err != nil {
		return 0, err
	}
	if maxPos == nil {
		return 1, nil
	}
	return *maxPos + 1, nil
}

// --- platform key rotation (G126) -------------------------------------------

func (r *ltiPlatformKeyRepo) ListKeys(ctx context.Context) ([]models.LTIPlatformKey, error) {
	var keys []models.LTIPlatformKey
	err := r.db.WithContext(ctx).Order("id ASC").Find(&keys).Error
	return keys, err
}

// Rotate implements repository.LTIPlatformKeyRotator. The rename order
// keeps the unique name index satisfied at every statement.
func (r *ltiPlatformKeyRepo) Rotate(ctx context.Context, newNext, promoteFallback *models.LTIPlatformKey) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Serialize concurrent rotations.
		if err := tx.Exec("LOCK TABLE lti_platform_keys IN SHARE ROW EXCLUSIVE MODE").Error; err != nil {
			return err
		}
		if err := tx.Where("name = ?", "previous").Delete(&models.LTIPlatformKey{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.LTIPlatformKey{}).Where("name = ?", "default").Update("name", "previous").Error; err != nil {
			return err
		}
		res := tx.Model(&models.LTIPlatformKey{}).Where("name = ?", "next").Update("name", "default")
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			if promoteFallback == nil {
				return errors.New("no next platform key to promote")
			}
			promoteFallback.Name = "default"
			if err := tx.Create(promoteFallback).Error; err != nil {
				return err
			}
		}
		newNext.Name = "next"
		return tx.Create(newNext).Error
	})
}

var _ repository.LTIPlatformKeyRotator = (*ltiPlatformKeyRepo)(nil)
