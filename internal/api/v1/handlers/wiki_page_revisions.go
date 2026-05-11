package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type WikiPageRevisionHandler struct {
	revisionService *service.WikiPageRevisionService
}

func NewWikiPageRevisionHandler(revisionService *service.WikiPageRevisionService) *WikiPageRevisionHandler {
	return &WikiPageRevisionHandler{revisionService: revisionService}
}

func revisionToJSON(r *models.WikiPageRevision) fiber.Map {
	return fiber.Map{
		"revision_id":     r.ID,
		"wiki_page_id":   r.WikiPageID,
		"revision_number": r.RevisionNumber,
		"title":           r.Title,
		"body":            r.Body,
		"edited_by":       r.EditedBy,
		"created_at":      r.CreatedAt,
	}
}

func (h *WikiPageRevisionHandler) ListRevisions(c *fiber.Ctx) error {
	_, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	pageID, err := c.ParamsInt("page_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid page ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.revisionService.ListRevisions(c.Context(), uint(pageID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch revisions")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	revisions := make([]fiber.Map, len(result.Items))
	for i, r := range result.Items {
		revisions[i] = revisionToJSON(&r)
	}

	return c.JSON(revisions)
}

func (h *WikiPageRevisionHandler) GetRevision(c *fiber.Ctx) error {
	_, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	_, err = c.ParamsInt("page_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid page ID")
	}

	revisionID, err := c.ParamsInt("revision_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid revision ID")
	}

	revision, err := h.revisionService.GetRevision(c.Context(), uint(revisionID))
	if err != nil {
		return responses.NotFound(c, "revision")
	}

	return c.JSON(revisionToJSON(revision))
}

func (h *WikiPageRevisionHandler) RevertToRevision(c *fiber.Ctx) error {
	_, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	pageID, err := c.ParamsInt("page_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid page ID")
	}

	revisionID, err := c.ParamsInt("revision_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid revision ID")
	}

	userID, _ := c.Locals("user_id").(uint)

	revision, err := h.revisionService.RevertToRevision(c.Context(), uint(pageID), uint(revisionID), userID)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(revisionToJSON(revision))
}
