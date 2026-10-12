package handlers

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

// WikiPageRevisionHandler serves page history (G065), Canvas-compatible:
//
//	GET  /courses/:course_id/pages/:url_or_id/revisions
//	GET  /courses/:course_id/pages/:url_or_id/revisions/latest
//	GET  /courses/:course_id/pages/:url_or_id/revisions/:revision_id
//	POST /courses/:course_id/pages/:url_or_id/revisions/:revision_id  (revert)
//
// :revision_id is the per-page revision number. Reading history needs
// edit rights on the page (Canvas :read_revisions); reverting needs
// course-manager rights (Canvas :update).
type WikiPageRevisionHandler struct {
	pages           *PageHandler
	revisionService *service.WikiPageRevisionService
}

func NewWikiPageRevisionHandler(pages *PageHandler, revisionService *service.WikiPageRevisionService) *WikiPageRevisionHandler {
	return &WikiPageRevisionHandler{pages: pages, revisionService: revisionService}
}

func revisionToJSON(r *models.WikiPageRevision, page *models.WikiPage, latest int, includeBody bool) fiber.Map {
	out := fiber.Map{
		"revision_id":     r.RevisionNumber,
		"revision_number": r.RevisionNumber,
		"wiki_page_id":    r.WikiPageID,
		"url":             page.URL,
		"title":           r.Title,
		"edited_by":       r.EditedBy,
		"updated_at":      r.CreatedAt,
		"created_at":      r.CreatedAt,
		"latest":          r.RevisionNumber == latest,
	}
	if includeBody {
		out["body"] = r.Body
	}
	return out
}

// loadPageForHistory loads the page and checks the caller may read its
// history. Returns wrote=true after writing the response.
func (h *WikiPageRevisionHandler) loadPageForHistory(c *fiber.Ctx) (*models.WikiPage, bool, error) {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return nil, true, responses.BadRequest(c, "Invalid course ID")
	}
	page, wrote, err := h.pages.loadCoursePage(c, uint(courseID), c.Params("url_or_id"))
	if wrote {
		return nil, true, err
	}
	if page.WorkflowState != pageStateActive && !canViewUnpublishedContent(c) {
		return nil, true, responses.NotFound(c, "page")
	}
	if !canEditPage(c, page) {
		return nil, true, responses.Forbidden(c, "You do not have permission to view this page's history")
	}
	return page, false, nil
}

func (h *WikiPageRevisionHandler) ListRevisions(c *fiber.Ctx) error {
	page, wrote, err := h.loadPageForHistory(c)
	if wrote {
		return err
	}

	params := middleware.GetPagination(c)
	result, err := h.revisionService.ListRevisions(c.Context(), page.ID, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch revisions")
	}
	latest := h.revisionService.LatestNumber(c.Context(), page.ID)

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	// The list carries bodies too: the history view diffs neighbours, and
	// a page of 10–50 revisions is small.
	includeBody := c.Query("summary") != "true"
	revisions := make([]fiber.Map, len(result.Items))
	for i := range result.Items {
		revisions[i] = revisionToJSON(&result.Items[i], page, latest, includeBody)
	}
	return c.JSON(revisions)
}

func (h *WikiPageRevisionHandler) GetRevision(c *fiber.Ctx) error {
	page, wrote, err := h.loadPageForHistory(c)
	if wrote {
		return err
	}

	number := 0 // latest
	if raw := c.Params("revision_id"); raw != "latest" {
		n, convErr := strconv.Atoi(raw)
		if convErr != nil || n <= 0 {
			return responses.BadRequest(c, "Invalid revision ID")
		}
		number = n
	}

	revision, err := h.revisionService.GetRevision(c.Context(), page.ID, number)
	if err != nil {
		return responses.NotFound(c, "revision")
	}
	latest := h.revisionService.LatestNumber(c.Context(), page.ID)
	return c.JSON(revisionToJSON(revision, page, latest, c.Query("summary") != "true"))
}

func (h *WikiPageRevisionHandler) RevertToRevision(c *fiber.Ctx) error {
	page, wrote, err := h.loadPageForHistory(c)
	if wrote {
		return err
	}
	if !canViewUnpublishedContent(c) {
		return responses.Forbidden(c, "Only course staff can restore a page revision")
	}

	number, err := strconv.Atoi(c.Params("revision_id"))
	if err != nil || number <= 0 {
		return responses.BadRequest(c, "Invalid revision ID")
	}

	revision, err := h.revisionService.RevertToRevision(c.Context(), page, number, actingUserID(c))
	if err != nil {
		if errors.Is(err, service.ErrRevisionNotFound) {
			return responses.NotFound(c, "revision")
		}
		return responses.InternalError(c, "Could not restore revision")
	}

	return c.JSON(revisionToJSON(revision, page, revision.RevisionNumber, true))
}
