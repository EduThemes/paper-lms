package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

// GamificationRuleRepo persists rules and their evaluation history.
type GamificationRuleRepo struct {
	db *gorm.DB
}

func NewGamificationRuleRepository(db *gorm.DB) *GamificationRuleRepo {
	return &GamificationRuleRepo{db: db}
}

func (r *GamificationRuleRepo) Create(ctx context.Context, rule *models.GamificationRule) error {
	return r.db.WithContext(ctx).Create(rule).Error
}

func (r *GamificationRuleRepo) FindByID(ctx context.Context, id uint) (*models.GamificationRule, error) {
	var rule models.GamificationRule
	if err := r.db.WithContext(ctx).First(&rule, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &rule, nil
}

func (r *GamificationRuleRepo) Update(ctx context.Context, rule *models.GamificationRule) error {
	return r.db.WithContext(ctx).Save(rule).Error
}

func (r *GamificationRuleRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.GamificationRule{}, id).Error
}

func (r *GamificationRuleRepo) ListEnabledByScope(ctx context.Context, scopeType models.GamificationScopeType, scopeID uint) ([]models.GamificationRule, error) {
	var rules []models.GamificationRule
	err := r.db.WithContext(ctx).
		Where("enabled = ? AND scope_type = ? AND scope_id = ?", true, scopeType, scopeID).
		Order("id ASC").
		Find(&rules).Error
	return rules, err
}

func (r *GamificationRuleRepo) ListByTenantID(ctx context.Context, tenantID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GamificationRule], error) {
	query := r.db.WithContext(ctx).Model(&models.GamificationRule{}).Where("tenant_id = ?", tenantID)

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, err
	}

	var rules []models.GamificationRule
	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("id DESC").Find(&rules).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.GamificationRule]{
		Items:      rules,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *GamificationRuleRepo) RecordEvaluation(ctx context.Context, eval *models.GamificationRuleEvaluation) error {
	return r.db.WithContext(ctx).Create(eval).Error
}

// LastFiringForUserRule returns the most recent successful evaluation
// (result=true) for (rule_id, user_id). Returns (nil, nil) when the rule
// has never successfully fired for this user — callers treat that as "no
// cooldown applies."
func (r *GamificationRuleRepo) LastFiringForUserRule(ctx context.Context, userID, ruleID uint) (*models.GamificationRuleEvaluation, error) {
	var eval models.GamificationRuleEvaluation
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND rule_id = ? AND result = ?", userID, ruleID, true).
		Order("evaluated_at DESC").
		First(&eval).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &eval, nil
}

// CountFiringsInWindow counts successful evaluations for (rule_id, user_id)
// strictly since `since`. Powers the max_per_window guard.
func (r *GamificationRuleRepo) CountFiringsInWindow(ctx context.Context, userID, ruleID uint, since time.Time) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.GamificationRuleEvaluation{}).
		Where("user_id = ? AND rule_id = ? AND result = ? AND evaluated_at > ?", userID, ruleID, true, since).
		Count(&count).Error
	return count, err
}

func (r *GamificationRuleRepo) ListEvaluationsForUserRule(ctx context.Context, userID, ruleID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GamificationRuleEvaluation], error) {
	query := r.db.WithContext(ctx).
		Model(&models.GamificationRuleEvaluation{}).
		Where("user_id = ? AND rule_id = ?", userID, ruleID)

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, err
	}

	var evals []models.GamificationRuleEvaluation
	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("evaluated_at DESC").Find(&evals).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.GamificationRuleEvaluation]{
		Items:      evals,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
