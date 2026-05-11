package service

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type WikiPageRevisionService struct {
	revisionRepo repository.WikiPageRevisionRepository
}

func NewWikiPageRevisionService(revisionRepo repository.WikiPageRevisionRepository) *WikiPageRevisionService {
	return &WikiPageRevisionService{revisionRepo: revisionRepo}
}

func (s *WikiPageRevisionService) CreateRevision(ctx context.Context, pageID uint, title, body string, editedBy uint) (*models.WikiPageRevision, error) {
	// Determine the next revision number by looking at the latest revision
	nextRevision := 1
	latest, err := s.revisionRepo.GetLatestRevision(ctx, pageID)
	if err == nil && latest != nil {
		nextRevision = latest.RevisionNumber + 1
	}

	revision := &models.WikiPageRevision{
		WikiPageID:     pageID,
		RevisionNumber: nextRevision,
		Title:          title,
		Body:           body,
		EditedBy:       editedBy,
	}

	if err := s.revisionRepo.Create(ctx, revision); err != nil {
		return nil, err
	}

	return revision, nil
}

func (s *WikiPageRevisionService) ListRevisions(ctx context.Context, pageID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.WikiPageRevision], error) {
	return s.revisionRepo.ListByPageID(ctx, pageID, params)
}

func (s *WikiPageRevisionService) GetRevision(ctx context.Context, id uint) (*models.WikiPageRevision, error) {
	return s.revisionRepo.FindByID(ctx, id)
}

func (s *WikiPageRevisionService) RevertToRevision(ctx context.Context, pageID uint, revisionID uint, userID uint) (*models.WikiPageRevision, error) {
	// Find the target revision to revert to
	target, err := s.revisionRepo.FindByID(ctx, revisionID)
	if err != nil {
		return nil, errors.New("revision not found")
	}

	// Verify the revision belongs to the specified page
	if target.WikiPageID != pageID {
		return nil, errors.New("revision does not belong to this page")
	}

	// Create a new revision with the content of the target revision
	return s.CreateRevision(ctx, pageID, target.Title, target.Body, userID)
}
