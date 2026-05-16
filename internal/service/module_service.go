package service

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type ModuleService struct {
	moduleRepo         repository.ModuleRepository
	moduleItemRepo     repository.ModuleItemRepository
	prerequisiteRepo   repository.ModulePrerequisiteRepository
}

func NewModuleService(moduleRepo repository.ModuleRepository, moduleItemRepo repository.ModuleItemRepository, opts ...func(*ModuleService)) *ModuleService {
	s := &ModuleService{
		moduleRepo:     moduleRepo,
		moduleItemRepo: moduleItemRepo,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func WithPrerequisiteRepo(repo repository.ModulePrerequisiteRepository) func(*ModuleService) {
	return func(s *ModuleService) {
		s.prerequisiteRepo = repo
	}
}

func (s *ModuleService) Create(ctx context.Context, module *models.ContextModule) error {
	if module.Name == "" {
		return errors.New("module name is required")
	}
	if module.WorkflowState == "" {
		module.WorkflowState = "active"
	}
	return s.moduleRepo.Create(ctx, module)
}

// GetByID is the public read path. Sprint 2.3 threaded accountID
// through from the handler; pass 0 from internal callers.
func (s *ModuleService) GetByID(ctx context.Context, id, accountID uint) (*models.ContextModule, error) {
	return s.moduleRepo.FindByID(ctx, id, accountID)
}

func (s *ModuleService) Update(ctx context.Context, module *models.ContextModule) error {
	return s.moduleRepo.Update(ctx, module)
}

func (s *ModuleService) Delete(ctx context.Context, id uint) error {
	return s.moduleRepo.Delete(ctx, id)
}

func (s *ModuleService) ListByCourse(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.ContextModule], error) {
	return s.moduleRepo.ListByCourseID(ctx, courseID, params)
}

func (s *ModuleService) CreateItem(ctx context.Context, item *models.ContentTag) error {
	if item.Title == "" {
		return errors.New("item title is required")
	}
	if item.WorkflowState == "" {
		item.WorkflowState = "active"
	}
	return s.moduleItemRepo.Create(ctx, item)
}

func (s *ModuleService) GetItem(ctx context.Context, id uint) (*models.ContentTag, error) {
	return s.moduleItemRepo.FindByID(ctx, id)
}

func (s *ModuleService) UpdateItem(ctx context.Context, item *models.ContentTag) error {
	return s.moduleItemRepo.Update(ctx, item)
}

func (s *ModuleService) DeleteItem(ctx context.Context, id uint) error {
	return s.moduleItemRepo.Delete(ctx, id)
}

func (s *ModuleService) ListItems(ctx context.Context, moduleID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.ContentTag], error) {
	return s.moduleItemRepo.ListByModuleID(ctx, moduleID, params)
}

func (s *ModuleService) ReorderModules(ctx context.Context, courseID uint, moduleIDs []uint) error {
	if len(moduleIDs) == 0 {
		return errors.New("module IDs are required")
	}
	return s.moduleRepo.ReorderModules(ctx, courseID, moduleIDs)
}

func (s *ModuleService) ReorderItems(ctx context.Context, moduleID uint, itemIDs []uint) error {
	if len(itemIDs) == 0 {
		return errors.New("item IDs are required")
	}
	return s.moduleItemRepo.ReorderItems(ctx, moduleID, itemIDs)
}

func (s *ModuleService) MoveItemToModule(ctx context.Context, itemID uint, targetModuleID uint, position int) error {
	return s.moduleItemRepo.MoveItemToModule(ctx, itemID, targetModuleID, position)
}

func (s *ModuleService) SetPrerequisites(ctx context.Context, moduleID uint, prerequisiteModuleIDs []uint) error {
	if s.prerequisiteRepo == nil {
		return errors.New("prerequisite repository not configured")
	}
	return s.prerequisiteRepo.SetPrerequisites(ctx, moduleID, prerequisiteModuleIDs)
}

func (s *ModuleService) GetPrerequisites(ctx context.Context, moduleID uint) ([]uint, error) {
	if s.prerequisiteRepo == nil {
		return nil, nil
	}
	return s.prerequisiteRepo.GetPrerequisites(ctx, moduleID)
}
