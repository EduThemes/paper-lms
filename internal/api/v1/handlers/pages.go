package handlers

import (
	"log/slog"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
)

type PageHandler struct {
	pageService *service.PageService

	// contentViewService is optional. When set (via SetContentViewService
	// at startup), every successful single-page fetch records a view that
	// the gamification engine consumes asynchronously. Nil is the safe
	// default: tests and the Phase 0 wiring leave it unset, and the
	// handler quietly skips view-recording in that case.
	contentViewService *service.ContentViewService
}

func NewPageHandler(pageService *service.PageService) *PageHandler {
	return &PageHandler{pageService: pageService}
}

// SetContentViewService attaches the optional content-view sink after
// construction. cmd/server/main.go (Sprint D-1 Phase 3) calls this once
// the gamification emitter is wired; the constructor stays single-arg
// so older call-sites and tests don't need to thread the new dep.
func (h *PageHandler) SetContentViewService(s *service.ContentViewService) {
	h.contentViewService = s
}

// recordPageView fires a RecordView for the just-served page if a
// ContentViewService is wired. Errors are logged and swallowed: a
// gamification failure must never fail the page render.
func (h *PageHandler) recordPageView(c *fiber.Ctx, page *models.WikiPage) {
	if h.contentViewService == nil || page == nil {
		return
	}
	userID, ok := c.Locals("user_id").(uint)
	if !ok || userID == 0 {
		return
	}
	// durationSeconds=0: Wave 1 doesn't track duration. Sprint D-2 will
	// supply real values from a client beacon.
	if err := h.contentViewService.RecordView(c.Context(), userID, gamification.ObjectPage, page.ID, 0); err != nil {
		slog.Error("pages: record content view",
			"page_id", page.ID,
			"user_id", userID,
			"error", err,
		)
	}
}

func pageToJSON(p *models.WikiPage) fiber.Map {
	return fiber.Map{
		"page_id":        p.ID,
		"url":            p.URL,
		"title":          p.Title,
		"body":           p.Body,
		"workflow_state": p.WorkflowState,
		"editing_roles":  p.EditingRoles,
		"front_page":     p.FrontPage,
		"public":         p.Public,
		"website_mode":   p.WebsiteMode,
		"published":      p.WorkflowState == "active",
		"created_at":     p.CreatedAt,
		"updated_at":     p.UpdatedAt,
	}
}

func (h *PageHandler) ListPages(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.pageService.ListByCourse(c.Context(), uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch pages")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	pages := make([]fiber.Map, len(result.Items))
	for i, p := range result.Items {
		pages[i] = pageToJSON(&p)
	}

	return c.JSON(pages)
}

func (h *PageHandler) GetPage(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	urlOrID := c.Params("url_or_id")

	// Try as numeric ID first
	if id, err := strconv.Atoi(urlOrID); err == nil {
		page, err := h.pageService.GetByID(c.Context(), uint(id), callerAccountID(c))
		if err != nil {
			return responses.NotFound(c, "page")
		}
		h.recordPageView(c, page)
		return c.JSON(pageToJSON(page))
	}

	// Try as URL slug
	page, err := h.pageService.GetByURL(c.Context(), uint(courseID), urlOrID)
	if err != nil {
		return responses.NotFound(c, "page")
	}

	h.recordPageView(c, page)
	return c.JSON(pageToJSON(page))
}

func (h *PageHandler) CreatePage(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		WikiPage struct {
			Title        string `json:"title"`
			Body         string `json:"body"`
			EditingRoles string `json:"editing_roles"`
			Published    bool   `json:"published"`
			FrontPage    bool   `json:"front_page"`
			Public       bool   `json:"public"`
			WebsiteMode  bool   `json:"website_mode"`
		} `json:"wiki_page"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if strings.TrimSpace(input.WikiPage.Title) == "" {
		return responses.BadRequest(c, "Page title is required")
	}

	state := "unpublished"
	if input.WikiPage.Published {
		state = "active"
	}

	page := &models.WikiPage{
		CourseID:      uint(courseID),
		Title:         input.WikiPage.Title,
		Body:          input.WikiPage.Body,
		EditingRoles:  input.WikiPage.EditingRoles,
		FrontPage:     input.WikiPage.FrontPage,
		Public:        input.WikiPage.Public,
		WebsiteMode:   input.WikiPage.WebsiteMode,
		WorkflowState: state,
	}

	if page.EditingRoles == "" {
		page.EditingRoles = "teachers"
	}

	if err := h.pageService.Create(c.Context(), page); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(pageToJSON(page))
}

func (h *PageHandler) UpdatePage(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	urlOrID := c.Params("url_or_id")

	var page *models.WikiPage
	if id, err := strconv.Atoi(urlOrID); err == nil {
		page, err = h.pageService.GetByID(c.Context(), uint(id), callerAccountID(c))
		if err != nil {
			return responses.NotFound(c, "page")
		}
	} else {
		page, err = h.pageService.GetByURL(c.Context(), uint(courseID), urlOrID)
		if err != nil {
			return responses.NotFound(c, "page")
		}
	}

	var input struct {
		WikiPage struct {
			Title        *string `json:"title"`
			Body         *string `json:"body"`
			EditingRoles *string `json:"editing_roles"`
			Published    *bool   `json:"published"`
			FrontPage    *bool   `json:"front_page"`
			Public       *bool   `json:"public"`
			WebsiteMode  *bool   `json:"website_mode"`
		} `json:"wiki_page"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.WikiPage.Title != nil {
		page.Title = *input.WikiPage.Title
	}
	if input.WikiPage.Body != nil {
		page.Body = *input.WikiPage.Body
	}
	if input.WikiPage.EditingRoles != nil {
		page.EditingRoles = *input.WikiPage.EditingRoles
	}
	if input.WikiPage.Published != nil {
		if *input.WikiPage.Published {
			page.WorkflowState = "active"
		} else {
			page.WorkflowState = "unpublished"
		}
	}
	if input.WikiPage.FrontPage != nil {
		page.FrontPage = *input.WikiPage.FrontPage
	}
	if input.WikiPage.Public != nil {
		page.Public = *input.WikiPage.Public
	}
	if input.WikiPage.WebsiteMode != nil {
		page.WebsiteMode = *input.WikiPage.WebsiteMode
	}

	if err := h.pageService.Update(c.Context(), page); err != nil {
		return responses.InternalError(c, "Could not update page")
	}

	return c.JSON(pageToJSON(page))
}

func (h *PageHandler) DeletePage(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	urlOrID := c.Params("url_or_id")

	var page *models.WikiPage
	if id, err := strconv.Atoi(urlOrID); err == nil {
		page, err = h.pageService.GetByID(c.Context(), uint(id), callerAccountID(c))
		if err != nil {
			return responses.NotFound(c, "page")
		}
	} else {
		page, err = h.pageService.GetByURL(c.Context(), uint(courseID), urlOrID)
		if err != nil {
			return responses.NotFound(c, "page")
		}
	}

	if err := h.pageService.Delete(c.Context(), page.ID); err != nil {
		return responses.InternalError(c, "Could not delete page")
	}

	return c.JSON(fiber.Map{"delete": true})
}

func (h *PageHandler) GetPublicPage(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	slug := c.Params("slug")
	if slug == "" {
		return responses.BadRequest(c, "Page slug is required")
	}

	page, err := h.pageService.GetPublicPage(c.Context(), uint(courseID), slug)
	if err != nil {
		return responses.NotFound(c, "page")
	}

	return c.JSON(pageToJSON(page))
}
