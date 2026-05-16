package service

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type GradingPeriodService struct {
	groupRepo  repository.GradingPeriodGroupRepository
	periodRepo repository.GradingPeriodRepository
}

func NewGradingPeriodService(groupRepo repository.GradingPeriodGroupRepository, periodRepo repository.GradingPeriodRepository) *GradingPeriodService {
	return &GradingPeriodService{groupRepo: groupRepo, periodRepo: periodRepo}
}

// Group operations

func (s *GradingPeriodService) CreateGroup(ctx context.Context, group *models.GradingPeriodGroup) error {
	if group.Title == "" {
		return errors.New("grading period group title is required")
	}
	if group.WorkflowState == "" {
		group.WorkflowState = "active"
	}
	return s.groupRepo.Create(ctx, group)
}

func (s *GradingPeriodService) GetGroup(ctx context.Context, id uint) (*models.GradingPeriodGroup, error) {
	return s.groupRepo.FindByID(ctx, id, 0)
}

func (s *GradingPeriodService) UpdateGroup(ctx context.Context, group *models.GradingPeriodGroup) error {
	return s.groupRepo.Update(ctx, group)
}

func (s *GradingPeriodService) DeleteGroup(ctx context.Context, id uint) error {
	return s.groupRepo.Delete(ctx, id)
}

func (s *GradingPeriodService) ListGroupsByAccount(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GradingPeriodGroup], error) {
	return s.groupRepo.ListByAccountID(ctx, accountID, params)
}

// Period operations

func (s *GradingPeriodService) CreatePeriod(ctx context.Context, period *models.GradingPeriod) error {
	if period.Title == "" {
		return errors.New("grading period title is required")
	}
	if !period.StartDate.Before(period.EndDate) {
		return errors.New("start_date must be before end_date")
	}
	if period.WorkflowState == "" {
		period.WorkflowState = "active"
	}
	return s.periodRepo.Create(ctx, period)
}

func (s *GradingPeriodService) GetPeriod(ctx context.Context, id uint) (*models.GradingPeriod, error) {
	return s.periodRepo.FindByID(ctx, id, 0)
}

func (s *GradingPeriodService) UpdatePeriod(ctx context.Context, period *models.GradingPeriod) error {
	if !period.StartDate.Before(period.EndDate) {
		return errors.New("start_date must be before end_date")
	}
	return s.periodRepo.Update(ctx, period)
}

func (s *GradingPeriodService) DeletePeriod(ctx context.Context, id uint) error {
	return s.periodRepo.Delete(ctx, id)
}

func (s *GradingPeriodService) ListPeriodsByGroup(ctx context.Context, groupID uint) ([]models.GradingPeriod, error) {
	return s.periodRepo.ListByGroupID(ctx, groupID, 0)
}
