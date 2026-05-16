package service

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type CollaborationService struct {
	collabRepo repository.CollaborationRepository
}

func NewCollaborationService(collabRepo repository.CollaborationRepository) *CollaborationService {
	return &CollaborationService{collabRepo: collabRepo}
}

func (s *CollaborationService) Create(ctx context.Context, collaboration *models.Collaboration) error {
	if collaboration.Title == "" {
		return errors.New("collaboration title is required")
	}
	if collaboration.CollaborationType == "" {
		return errors.New("collaboration_type is required")
	}
	if collaboration.CollaborationType != "google_docs" && collaboration.CollaborationType != "etherpad" {
		return errors.New("collaboration_type must be 'google_docs' or 'etherpad'")
	}
	if collaboration.ContextType == "" || collaboration.ContextID == 0 {
		return errors.New("context_type and context_id are required")
	}
	if collaboration.WorkflowState == "" {
		collaboration.WorkflowState = "active"
	}

	return s.collabRepo.Create(ctx, collaboration)
}

func (s *CollaborationService) GetByID(ctx context.Context, id uint) (*models.Collaboration, error) {
	collaboration, err := s.collabRepo.FindByID(ctx, id, 0)
	if err != nil {
		return nil, errors.New("collaboration not found")
	}
	return collaboration, nil
}

func (s *CollaborationService) Update(ctx context.Context, collaboration *models.Collaboration) error {
	_, err := s.collabRepo.FindByID(ctx, collaboration.ID, 0)
	if err != nil {
		return errors.New("collaboration not found")
	}

	return s.collabRepo.Update(ctx, collaboration)
}

func (s *CollaborationService) Delete(ctx context.Context, id uint) error {
	return s.collabRepo.Delete(ctx, id)
}

func (s *CollaborationService) ListByContext(ctx context.Context, contextType string, contextID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Collaboration], error) {
	return s.collabRepo.ListByContext(ctx, contextType, contextID, 0, params)
}
