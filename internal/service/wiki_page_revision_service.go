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
// wiki page history). Every save that changes a page's title, body,
// stylesheet or design appends a revision; a revert copies an old
// revision's content back onto the page and records that as a new
// revision.
type WikiPageRevisionService struct {
	revisionRepo repository.WikiPageRevisionRepository
	pageRepo     repository.PageRepository
	// cssPolicy resolves pages.custom_css_enabled for the page's course on
	// restore. Nil: on.
	cssPolicy CourseCSSPolicy
}

func NewWikiPageRevisionService(revisionRepo repository.WikiPageRevisionRepository, pageRepo repository.PageRepository) *WikiPageRevisionService {
	return &WikiPageRevisionService{revisionRepo: revisionRepo, pageRepo: pageRepo}
}

// SetCourseCSSPolicy attaches the pages.custom_css_enabled resolution a
// restore applies.
func (s *WikiPageRevisionService) SetCourseCSSPolicy(p CourseCSSPolicy) *WikiPageRevisionService {
	s.cssPolicy = p
	return s
}

func editorPtr(editedBy uint) *uint {
	if editedBy == 0 {
		return nil
	}
	return &editedBy
}

// PageSnapshot is the part of a page that is versioned: title, body,
// stylesheet and design.
type PageSnapshot struct {
	Title     string
	Body      string
	CSS       string
	CSSSource string
	Design    models.NullJSON
}

// SnapshotOf returns page's versioned content.
func SnapshotOf(page *models.WikiPage) PageSnapshot {
	if page == nil {
		return PageSnapshot{}
	}
	return PageSnapshot{Title: page.Title, Body: page.Body, CSS: page.CSS, CSSSource: page.CSSSource, Design: page.Design}
}

// Equal reports whether two snapshots hold the same content. Designs are
// compared as JSON values: jsonb hands documents back re-formatted.
func (a PageSnapshot) Equal(b PageSnapshot) bool {
	return a.Title == b.Title && a.Body == b.Body && a.CSS == b.CSS && a.CSSSource == b.CSSSource && jsonValuesEqual(a.Design, b.Design)
}

// CreateRevision appends snap as the page's next revision.
func (s *WikiPageRevisionService) CreateRevision(ctx context.Context, pageID uint, snap PageSnapshot, editedBy uint) (*models.WikiPageRevision, error) {
	revision := &models.WikiPageRevision{
		WikiPageID: pageID,
		Title:      snap.Title,
		Body:       snap.Body,
		CSS:        snap.CSS,
		CSSSource:  snap.CSSSource,
		Design:     snap.Design,
		EditedBy:   editorPtr(editedBy),
	}
	if err := s.revisionRepo.CreateNext(ctx, revision); err != nil {
		return nil, err
	}
	return revision, nil
}

// RecordSave is called after a page save. prev is the page's content
// before the save. Nothing is recorded when nothing versioned changed. A
// page that has no history yet (created by an import, a module item or
// blueprint sync, which bypass the handler) first gets its pre-save
// content as revision 1, so the edit can still be undone.
func (s *WikiPageRevisionService) RecordSave(ctx context.Context, page *models.WikiPage, prev PageSnapshot, editedBy uint) error {
	if page == nil {
		return nil
	}
	cur := SnapshotOf(page)
	if cur.Equal(prev) {
		return nil
	}
	if _, err := s.revisionRepo.GetLatestRevision(ctx, page.ID); err != nil {
		if _, err := s.CreateRevision(ctx, page.ID, prev, 0); err != nil {
			return err
		}
	}
	_, err := s.CreateRevision(ctx, page.ID, cur, editedBy)
	return err
}

// RecordCreate stores revision 1 for a newly created page.
func (s *WikiPageRevisionService) RecordCreate(ctx context.Context, page *models.WikiPage, editedBy uint) error {
	if page == nil {
		return nil
	}
	_, err := s.CreateRevision(ctx, page.ID, SnapshotOf(page), editedBy)
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

// RevertToRevision restores the page's title, body, stylesheet and design
// from revision number and saves the page, recording the restore as a new
// revision. The body, stylesheet and design are re-checked on the way back
// in (they were checked when they were saved, but the policy may have
// tightened since), and with pages.custom_css_enabled off for the course a
// restored page without a design gets no stylesheet.
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
	page.CSS = target.CSS
	page.CSSSource = target.CSSSource
	page.Design = target.Design
	cleanPageDesign(page, s.cssPolicy.Allowed(ctx, page.CourseID))
	if err := s.pageRepo.Update(ctx, page); err != nil {
		return nil, err
	}
	// The target exists, so the page already has history; no baseline
	// revision is needed before recording the restore.
	return s.CreateRevision(ctx, page.ID, SnapshotOf(page), userID)
}
