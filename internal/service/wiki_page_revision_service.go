package service

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// ErrRevisionNotFound is returned when a revision number does not exist
// for the page.
var ErrRevisionNotFound = errors.New("revision not found")

// WikiPageRevisionService records and restores page versions (Canvas
// wiki page history). Every save that changes a page's title or body
// appends a revision; a revert copies an old revision's title and body
// back onto the page and records that as a new revision.
type WikiPageRevisionService struct {
	revisionRepo repository.WikiPageRevisionRepository
	pageRepo     repository.PageRepository
}

func NewWikiPageRevisionService(revisionRepo repository.WikiPageRevisionRepository, pageRepo repository.PageRepository) *WikiPageRevisionService {
	return &WikiPageRevisionService{revisionRepo: revisionRepo, pageRepo: pageRepo}
}

func editorPtr(editedBy uint) *uint {
	if editedBy == 0 {
		return nil
	}
	return &editedBy
}

// CreateRevision appends the page's current title and body as its next
// revision.
func (s *WikiPageRevisionService) CreateRevision(ctx context.Context, pageID uint, title, body string, editedBy uint) (*models.WikiPageRevision, error) {
	revision := &models.WikiPageRevision{
		WikiPageID: pageID,
		Title:      title,
		Body:       body,
		EditedBy:   editorPtr(editedBy),
	}
	if err := s.revisionRepo.CreateNext(ctx, revision); err != nil {
		return nil, err
	}
	return revision, nil
}

// RecordSave is called after a page save. prevTitle / prevBody are the
// values before the save. Nothing is recorded when neither changed. A
// page that has no history yet (created by an import, a module item or
// blueprint sync, which bypass the handler) first gets its pre-save
// content as revision 1, so the edit can still be undone.
func (s *WikiPageRevisionService) RecordSave(ctx context.Context, page *models.WikiPage, prevTitle, prevBody string, editedBy uint) error {
	if page == nil || (page.Title == prevTitle && page.Body == prevBody) {
		return nil
	}
	if _, err := s.revisionRepo.GetLatestRevision(ctx, page.ID); err != nil {
		if _, err := s.CreateRevision(ctx, page.ID, prevTitle, prevBody, 0); err != nil {
			return err
		}
	}
	_, err := s.CreateRevision(ctx, page.ID, page.Title, page.Body, editedBy)
	return err
}

// RecordCreate stores revision 1 for a newly created page.
func (s *WikiPageRevisionService) RecordCreate(ctx context.Context, page *models.WikiPage, editedBy uint) error {
	if page == nil {
		return nil
	}
	_, err := s.CreateRevision(ctx, page.ID, page.Title, page.Body, editedBy)
	return err
}

func (s *WikiPageRevisionService) ListRevisions(ctx context.Context, pageID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.WikiPageRevision], error) {
	return s.revisionRepo.ListByPageID(ctx, pageID, params)
}

// GetRevision returns revision number of the page (number <= 0 means the
// latest one).
func (s *WikiPageRevisionService) GetRevision(ctx context.Context, pageID uint, number int) (*models.WikiPageRevision, error) {
	var (
		rev *models.WikiPageRevision
		err error
	)
	if number <= 0 {
		rev, err = s.revisionRepo.GetLatestRevision(ctx, pageID)
	} else {
		rev, err = s.revisionRepo.GetRevisionByNumber(ctx, pageID, number)
	}
	if err != nil || rev == nil || rev.WikiPageID != pageID {
		return nil, ErrRevisionNotFound
	}
	return rev, nil
}

// LatestNumber returns the page's newest revision number (0 if none).
func (s *WikiPageRevisionService) LatestNumber(ctx context.Context, pageID uint) int {
	rev, err := s.revisionRepo.GetLatestRevision(ctx, pageID)
	if err != nil || rev == nil {
		return 0
	}
	return rev.RevisionNumber
}

// RevertToRevision restores the page's title and body from revision
// number and saves the page, recording the restore as a new revision.
// The body is re-sanitized on the way back in: it was sanitized when it
// was saved, but the policy may have tightened since.
func (s *WikiPageRevisionService) RevertToRevision(ctx context.Context, page *models.WikiPage, number int, userID uint) (*models.WikiPageRevision, error) {
	if page == nil {
		return nil, ErrRevisionNotFound
	}
	target, err := s.GetRevision(ctx, page.ID, number)
	if err != nil {
		return nil, err
	}
	page.Title = target.Title
	page.Body = SanitizeHTML(target.Body)
	if err := s.pageRepo.Update(ctx, page); err != nil {
		return nil, err
	}
	// The target exists, so the page already has history; no baseline
	// revision is needed before recording the restore.
	return s.CreateRevision(ctx, page.ID, page.Title, page.Body, userID)
}
