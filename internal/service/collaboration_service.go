package service

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type CollaborationService struct {
	collabRepo repository.CollaborationRepository
}

func NewCollaborationService(collabRepo repository.CollaborationRepository) *CollaborationService {
	return &CollaborationService{collabRepo: collabRepo}
}

// ErrInvalidCollaborationURL is returned when a collaboration's document
// URL is not an absolute http(s) URL.
var ErrInvalidCollaborationURL = errors.New("url must be an absolute http(s) URL")

// ValidateCollaborationURL accepts an empty URL or an absolute http(s)
// URL. The URL is rendered as a link that teachers and students click, so
// javascript:, data: and other schemes are refused (stored XSS).
func ValidateCollaborationURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ErrInvalidCollaborationURL
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return nil
	}
	return ErrInvalidCollaborationURL
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
	if err := ValidateCollaborationURL(collaboration.URL); err != nil {
		return err
	}
	if collaboration.WorkflowState == "" {
		collaboration.WorkflowState = "active"
	}

	return s.collabRepo.Create(ctx, collaboration)
}

// GetByID is the public read path. Sprint 2.3 threaded accountID through
// from the handler; pass 0 from internal callers where the tenant was
// verified upstream.
func (s *CollaborationService) GetByID(ctx context.Context, id, accountID uint) (*models.Collaboration, error) {
	collaboration, err := s.collabRepo.FindByID(ctx, id, accountID)
	if err != nil {
		return nil, errors.New("collaboration not found")
	}
	return collaboration, nil
}

func (s *CollaborationService) Update(ctx context.Context, collaboration *models.Collaboration) error {
	if err := ValidateCollaborationURL(collaboration.URL); err != nil {
		return err
	}
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
