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

	// Optional content repositories (SetContentRepos) used to verify a
	// module item's content_id names content in the module's course.
	assignmentRepo repository.AssignmentRepository
	quizRepo       repository.QuizRepository
	discussionRepo repository.DiscussionTopicRepository
	pageRepo       repository.PageRepository

	// File and External Tool items (SetLinkRepos, module_item_links.go).
	attachmentRepo repository.AttachmentRepository
	toolRepo       repository.ContextExternalToolRepository
	devKeyRepo     repository.DeveloperKeyRepository
	courseRepo     repository.CourseRepository
}

// ErrModuleContentNotInCourse is returned when a module item would link
// content that does not exist in the module's course.
var ErrModuleContentNotInCourse = errors.New("content not found in this course")

// SetContentRepos wires the repositories ValidateItemContent needs.
func (s *ModuleService) SetContentRepos(assignmentRepo repository.AssignmentRepository, quizRepo repository.QuizRepository, discussionRepo repository.DiscussionTopicRepository, pageRepo repository.PageRepository) {
	s.assignmentRepo = assignmentRepo
	s.quizRepo = quizRepo
	s.discussionRepo = discussionRepo
	s.pageRepo = pageRepo
}

// ValidateItemContent checks that contentID names live content of the
// given module-item content type inside courseID (and the caller's
// tenant). It returns the content's title so a blank item title can
// default to it. Content types without a backing row (ExternalUrl,
// SubHeader) are not validated here.
func (s *ModuleService) ValidateItemContent(ctx context.Context, contentType string, contentID, courseID, accountID uint) (string, error) {
	switch normalizeModuleContentType(contentType) {
	case "Assignment":
		if s.assignmentRepo == nil {
			return "", nil
		}
		a, err := s.assignmentRepo.FindByID(ctx, contentID, accountID)
		if err != nil || a == nil || a.CourseID != courseID || a.WorkflowState == models.AssignmentDeleted {
			return "", ErrModuleContentNotInCourse
		}
		return a.Name, nil
	case "Quiz":
		if s.quizRepo == nil {
			return "", nil
		}
		q, err := s.quizRepo.FindByID(ctx, contentID, accountID)
		if err != nil || q == nil || q.CourseID != courseID || q.WorkflowState == "deleted" {
			return "", ErrModuleContentNotInCourse
		}
		return q.Title, nil
	case "DiscussionTopic":
		if s.discussionRepo == nil {
			return "", nil
		}
		d, err := s.discussionRepo.FindByID(ctx, contentID, accountID)
		if err != nil || d == nil || d.CourseID != courseID || d.WorkflowState == "deleted" {
			return "", ErrModuleContentNotInCourse
		}
		return d.Title, nil
	case "WikiPage":
		if s.pageRepo == nil {
			return "", nil
		}
		p, err := s.pageRepo.FindByID(ctx, contentID, accountID)
		if err != nil || p == nil || p.CourseID != courseID || p.WorkflowState == "deleted" {
			return "", ErrModuleContentNotInCourse
		}
		return p.Title, nil
	}
	return "", nil
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

// ListPublishedByCourse is the student-facing module list: published
// modules with only their published items.
func (s *ModuleService) ListPublishedByCourse(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.ContextModule], error) {
	return s.moduleRepo.ListPublishedByCourseID(ctx, courseID, params)
}

// ListPublishedItems is the student-facing item list for one module.
func (s *ModuleService) ListPublishedItems(ctx context.Context, moduleID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.ContentTag], error) {
	return s.moduleItemRepo.ListPublishedByModuleID(ctx, moduleID, params)
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
