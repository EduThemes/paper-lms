package handlers

import (
	"log/slog"
	"strconv"
	"strings"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/security/csssanitize"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
	"github.com/gofiber/fiber/v2"
)

type PageHandler struct {
	pageService *service.PageService

	// contentViewService is optional. When set (via SetContentViewService
	// at startup), every successful single-page fetch records a view that
	// the gamification engine consumes asynchronously. Nil is the safe
	// default: tests and the Phase 0 wiring leave it unset, and the
	// handler quietly skips view-recording in that case.
	contentViewService *service.ContentViewService

	// revisionService records page history on create/update (G065). Nil
	// disables history (older tests build the handler without it).
	revisionService *service.WikiPageRevisionService

	// settingsLookup resolves pages.custom_css_enabled for the caller's
	// account. Nil means the setting's default (on).
	settingsLookup service.SettingsLookupFunc
}

// SetSettingsLookup attaches the Settings Engine lookup used for
// pages.custom_css_enabled.
func (h *PageHandler) SetSettingsLookup(f service.SettingsLookupFunc) *PageHandler {
	h.settingsLookup = f
	return h
}

// SetRevisionService attaches page history recording.
func (h *PageHandler) SetRevisionService(s *service.WikiPageRevisionService) *PageHandler {
	h.revisionService = s
	return h
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

// pageToJSON is the page shape shared by list and show. The list adds
// has_design (pageListJSON); show adds the stylesheet and design
// (pageDetailJSON).
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
		"todo_date":      p.TodoDate,
		"created_at":     p.CreatedAt,
		"updated_at":     p.UpdatedAt,
	}
}

// pageListJSON is a page in GET /pages: whether it is a designed page, but
// not its stylesheet or design (docs/page-designer, "API").
func pageListJSON(p *models.WikiPage) fiber.Map {
	out := pageToJSON(p)
	out["has_design"] = !p.Design.IsNull()
	return out
}

// pageDetailJSON is a single page: its stylesheet as stored (css), the
// same stylesheet scoped to the page root #paper-page-<id> (scoped_css,
// what readers apply), and the Page Designer document (design, or null).
// Course staff also get css_source, the stylesheet as typed, for the
// editor. The client wraps the body in <div id="paper-page-<id>">.
func pageDetailJSON(c *fiber.Ctx, p *models.WikiPage) fiber.Map {
	out := pageToJSON(p)
	out["css"] = p.CSS
	if src := pageCSSSource(p); src != "" && canViewUnpublishedContent(c) {
		out["css_source"] = src
	}
	out["scoped_css"] = scopedPageCSS(p)
	out["design"] = p.Design
	out["has_design"] = !p.Design.IsNull()
	return out
}

// pageCSSSource is the teacher's own text for the page stylesheet, or ""
// when it no longer sanitizes to the stored CSS (a course copy rewrote
// links, a sync or import replaced CSS without it): then the editor falls
// back to css rather than showing text that isn't what the page uses.
func pageCSSSource(p *models.WikiPage) string {
	if p.CSSSource == "" || p.CSS == "" {
		return ""
	}
	if css, err := csssanitize.Sanitize(p.CSSSource); err != nil || css != p.CSS {
		return ""
	}
	return p.CSSSource
}

// pageRootID is the element id a page's stylesheet is scoped to.
func pageRootID(p *models.WikiPage) string {
	return "#paper-page-" + strconv.FormatUint(uint64(p.ID), 10)
}

func scopedPageCSS(p *models.WikiPage) string {
	if strings.TrimSpace(p.CSS) == "" {
		return ""
	}
	return csssanitize.Scope(p.CSS, pageRootID(p))
}

func (h *PageHandler) ListPages(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	// B045: students and observers only ever see published pages.
	list := h.pageService.ListPublishedByCourse
	if canViewUnpublishedContent(c) {
		list = h.pageService.ListByCourse
	}
	result, err := list(c.Context(), uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch pages")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	pages := make([]fiber.Map, len(result.Items))
	for i, p := range result.Items {
		pages[i] = pageListJSON(&p)
	}

	return c.JSON(pages)
}

func (h *PageHandler) GetPage(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	urlOrID := c.Params("url_or_id")

	var page *models.WikiPage
	if id, convErr := strconv.Atoi(urlOrID); convErr == nil {
		// Numeric id. B137: FindByID is tenant-scoped but neither
		// course-scoped nor deleted-filtered, so tie it to :course_id
		// (F-013) and drop deleted rows — otherwise a student in course
		// A could read any same-tenant page of course B by id.
		page, err = h.pageService.GetByID(c.Context(), uint(id), callerAccountID(c))
		if err != nil || page == nil || page.CourseID != uint(courseID) || page.WorkflowState == "deleted" {
			return responses.NotFound(c, "page")
		}
	} else {
		page, err = h.pageService.GetByURL(c.Context(), uint(courseID), urlOrID)
		if err != nil || page == nil {
			return responses.NotFound(c, "page")
		}
	}

	// B045: a draft page is invisible to students/observers.
	if page.WorkflowState != "active" && !canViewUnpublishedContent(c) {
		return responses.NotFound(c, "page")
	}

	h.recordPageView(c, page)
	out := pageDetailJSON(c, page)
	// G074: tells the client whether to offer Edit / History.
	out["can_edit"] = canEditPage(c, page)
	return c.JSON(out)
}

// GetFrontPage handles GET /courses/:course_id/front_page — the page a
// course with default_view "wiki" shows on its home (B147). Students
// only get it while it is published.
func (h *PageHandler) GetFrontPage(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	page, err := h.pageService.GetFrontPage(c.Context(), uint(courseID))
	if err != nil || page == nil {
		return responses.NotFound(c, "front page")
	}
	if page.WorkflowState != "active" && !canViewUnpublishedContent(c) {
		return responses.NotFound(c, "front page")
	}
	h.recordPageView(c, page)
	return c.JSON(pageDetailJSON(c, page))
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

	return c.JSON(pageDetailJSON(c, page))
}
