package handlers

import (
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

// Page states. wiki_pages.workflow_state is a plain text column (no enum
// type yet), so these stay local.
const (
	pageStateActive      = "active"
	pageStateUnpublished = "unpublished"
	pageStateDeleted     = "deleted"
)

// validPageEditingRoles is Canvas's editing_roles vocabulary. "members"
// is the group-wiki spelling of "students"; "public" lets anyone who can
// read the page edit it.
var validPageEditingRoles = map[string]bool{"teachers": true, "students": true, "members": true, "public": true}

// normalizeEditingRoles validates a comma-separated editing_roles value
// and returns it in canonical form ("teachers,students").
func normalizeEditingRoles(raw string) (string, bool) {
	parts := strings.Split(raw, ",")
	seen := map[string]bool{}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		role := strings.TrimSpace(p)
		if role == "" {
			continue
		}
		if !validPageEditingRoles[role] {
			return "", false
		}
		if !seen[role] {
			seen[role] = true
			out = append(out, role)
		}
	}
	if len(out) == 0 {
		return "", false
	}
	return strings.Join(out, ","), true
}

// pageEditingRoleSet parses a stored editing_roles value.
func pageEditingRoleSet(roles string) map[string]bool {
	set := map[string]bool{}
	for _, p := range strings.Split(roles, ",") {
		if r := strings.TrimSpace(p); r != "" {
			set[r] = true
		}
	}
	return set
}

// learnerCanEditPage is Canvas's can_edit_page? for a caller who does not
// manage the course: the page must be published and its editing_roles
// must let students (or anyone) edit. Observers only get "public".
func learnerCanEditPage(c *fiber.Ctx, page *models.WikiPage) bool {
	if page == nil || page.WorkflowState != pageStateActive {
		return false
	}
	roles := pageEditingRoleSet(page.EditingRoles)
	if roles["public"] {
		return true
	}
	enrollmentType, _ := c.Locals("enrollment_type").(string)
	return enrollmentType == "StudentEnrollment" && (roles["students"] || roles["members"])
}

// canEditPage reports whether the caller may change the page's content.
func canEditPage(c *fiber.Ctx, page *models.WikiPage) bool {
	return canViewUnpublishedContent(c) || learnerCanEditPage(c, page)
}

// parseOptionalTimeJSON decodes an optional, nullable timestamp field.
// set=false means the field was absent; set=true with a nil time means
// an explicit null (clear the value).
func parseOptionalTimeJSON(raw json.RawMessage) (set bool, value *time.Time, err error) {
	if len(raw) == 0 {
		return false, nil, nil
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "null" || trimmed == `""` {
		return true, nil, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return true, nil, err
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return true, nil, err
	}
	return true, &t, nil
}

// actingUserID returns the caller's user id, or 0.
func actingUserID(c *fiber.Ctx) uint {
	id, _ := c.Locals("user_id").(uint)
	return id
}

// loadCoursePage resolves :url_or_id inside courseID. A numeric id is
// loaded tenant-scoped and tied to the course (F-013); deleted pages are
// gone either way. Returns wrote=true after writing a 404 — the caller
// must then return err as-is.
func (h *PageHandler) loadCoursePage(c *fiber.Ctx, courseID uint, urlOrID string) (*models.WikiPage, bool, error) {
	var (
		page *models.WikiPage
		err  error
	)
	if id, convErr := strconv.Atoi(urlOrID); convErr == nil {
		page, err = h.pageService.GetByID(c.Context(), uint(id), callerAccountID(c))
		if err != nil || page == nil || page.CourseID != courseID || page.WorkflowState == pageStateDeleted {
			return nil, true, responses.NotFound(c, "page")
		}
		return page, false, nil
	}
	page, err = h.pageService.GetByURL(c.Context(), courseID, urlOrID)
	if err != nil || page == nil || page.WorkflowState == pageStateDeleted {
		return nil, true, responses.NotFound(c, "page")
	}
	return page, false, nil
}

func (h *PageHandler) recordRevision(c *fiber.Ctx, page *models.WikiPage, prevTitle, prevBody string, created bool) {
	if h.revisionService == nil || page == nil {
		return
	}
	var err error
	if created {
		err = h.revisionService.RecordCreate(c.Context(), page, actingUserID(c))
	} else {
		err = h.revisionService.RecordSave(c.Context(), page, prevTitle, prevBody, actingUserID(c))
	}
	if err != nil {
		// History is best-effort: the page itself is saved.
		slog.Error("pages: record revision", "page_id", page.ID, "error", err)
	}
}

func (h *PageHandler) CreatePage(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		WikiPage struct {
			Title        string          `json:"title"`
			Body         string          `json:"body"`
			EditingRoles string          `json:"editing_roles"`
			Published    bool            `json:"published"`
			FrontPage    bool            `json:"front_page"`
			Public       bool            `json:"public"`
			WebsiteMode  bool            `json:"website_mode"`
			TodoDate     json.RawMessage `json:"todo_date"`
		} `json:"wiki_page"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if strings.TrimSpace(input.WikiPage.Title) == "" {
		return responses.BadRequest(c, "Page title is required")
	}

	state := pageStateUnpublished
	if input.WikiPage.Published {
		state = pageStateActive
	}

	editingRoles := "teachers"
	if strings.TrimSpace(input.WikiPage.EditingRoles) != "" {
		roles, ok := normalizeEditingRoles(input.WikiPage.EditingRoles)
		if !ok {
			return responses.BadRequest(c, "The provided editing roles are invalid")
		}
		editingRoles = roles
	}

	_, todoDate, err := parseOptionalTimeJSON(input.WikiPage.TodoDate)
	if err != nil {
		return responses.BadRequest(c, "Invalid todo_date")
	}

	// Canvas: the front page must be published.
	if input.WikiPage.FrontPage && state != pageStateActive {
		return responses.BadRequest(c, "The front page cannot be unpublished")
	}

	page := &models.WikiPage{
		CourseID:      uint(courseID),
		Title:         input.WikiPage.Title,
		Body:          service.SanitizeHTML(input.WikiPage.Body),
		EditingRoles:  editingRoles,
		FrontPage:     input.WikiPage.FrontPage,
		Public:        input.WikiPage.Public,
		WebsiteMode:   input.WikiPage.WebsiteMode,
		WorkflowState: state,
		TodoDate:      todoDate,
	}

	if err := h.pageService.Create(c.Context(), page); err != nil {
		return responses.BadRequest(c, err.Error())
	}
	if err := h.pageService.EnsureSingleFrontPage(c.Context(), page); err != nil {
		return responses.InternalError(c, "Could not set front page")
	}
	h.recordRevision(c, page, "", "", true)

	return c.Status(fiber.StatusCreated).JSON(pageToJSON(page))
}

// UpdatePage handles PUT /courses/:course_id/pages/:url_or_id. The route
// is open to every enrolled user (G074): course staff may change any
// field, while a student may change only the title and body of a
// published page whose editing_roles admit students (Canvas
// :update_content). Anything else is refused.
func (h *PageHandler) UpdatePage(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	page, wrote, err := h.loadCoursePage(c, uint(courseID), c.Params("url_or_id"))
	if wrote {
		return err
	}

	isManager := canViewUnpublishedContent(c)
	if !isManager {
		// A draft is invisible to learners (B045): 404, not 403.
		if page.WorkflowState != pageStateActive {
			return responses.NotFound(c, "page")
		}
		if !learnerCanEditPage(c, page) {
			return responses.Forbidden(c, "You do not have permission to edit this page")
		}
	}

	var input struct {
		WikiPage struct {
			Title        *string         `json:"title"`
			Body         *string         `json:"body"`
			EditingRoles *string         `json:"editing_roles"`
			Published    *bool           `json:"published"`
			FrontPage    *bool           `json:"front_page"`
			Public       *bool           `json:"public"`
			WebsiteMode  *bool           `json:"website_mode"`
			TodoDate     json.RawMessage `json:"todo_date"`
		} `json:"wiki_page"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	in := input.WikiPage

	if !isManager && (in.EditingRoles != nil || in.Published != nil || in.FrontPage != nil ||
		in.Public != nil || in.WebsiteMode != nil || len(in.TodoDate) > 0) {
		return responses.Forbidden(c, "You may only change the title and content of this page")
	}

	prevTitle, prevBody := page.Title, page.Body

	if in.Title != nil {
		if strings.TrimSpace(*in.Title) == "" {
			return responses.BadRequest(c, "Page title is required")
		}
		page.Title = *in.Title
	}
	if in.Body != nil {
		page.Body = service.SanitizeHTML(*in.Body)
	}
	if in.EditingRoles != nil {
		roles, ok := normalizeEditingRoles(*in.EditingRoles)
		if !ok {
			return responses.BadRequest(c, "The provided editing roles are invalid")
		}
		page.EditingRoles = roles
	}
	if in.Published != nil {
		if *in.Published {
			page.WorkflowState = pageStateActive
		} else {
			page.WorkflowState = pageStateUnpublished
		}
	}
	if in.FrontPage != nil {
		page.FrontPage = *in.FrontPage
	}
	if in.Public != nil {
		page.Public = *in.Public
	}
	if in.WebsiteMode != nil {
		page.WebsiteMode = *in.WebsiteMode
	}
	if set, todo, err := parseOptionalTimeJSON(in.TodoDate); err != nil {
		return responses.BadRequest(c, "Invalid todo_date")
	} else if set {
		page.TodoDate = todo
	}

	// Canvas: the front page must stay published.
	if page.FrontPage && page.WorkflowState != pageStateActive {
		return responses.BadRequest(c, "The front page cannot be unpublished")
	}

	if err := h.pageService.Update(c.Context(), page); err != nil {
		return responses.InternalError(c, "Could not update page")
	}
	if in.FrontPage != nil && *in.FrontPage {
		if err := h.pageService.EnsureSingleFrontPage(c.Context(), page); err != nil {
			return responses.InternalError(c, "Could not set front page")
		}
	}
	h.recordRevision(c, page, prevTitle, prevBody, false)

	return c.JSON(pageToJSON(page))
}

func (h *PageHandler) DeletePage(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	// loadCoursePage carries the F-013 parent-course tie for numeric ids.
	page, wrote, err := h.loadCoursePage(c, uint(courseID), c.Params("url_or_id"))
	if wrote {
		return err
	}

	// Canvas: the front page cannot be deleted; pick another first.
	if page.FrontPage {
		return responses.BadRequest(c, "The front page cannot be deleted")
	}

	if err := h.pageService.Delete(c.Context(), page.ID); err != nil {
		return responses.InternalError(c, "Could not delete page")
	}

	return c.JSON(fiber.Map{"delete": true})
}
