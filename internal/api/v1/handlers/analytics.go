package handlers

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type AnalyticsHandler struct {
	analyticsService *service.AnalyticsService
	auditService     *service.AuditService
}

func NewAnalyticsHandler(analyticsService *service.AnalyticsService, auditService *service.AuditService) *AnalyticsHandler {
	return &AnalyticsHandler{analyticsService: analyticsService, auditService: auditService}
}

func pageViewToJSON(pv *models.PageView) fiber.Map {
	return fiber.Map{
		"id":                  pv.ID,
		"user_id":             pv.UserID,
		"context_type":        pv.ContextType,
		"context_id":          pv.ContextID,
		"url":                 pv.URL,
		"action":              pv.Action,
		"participated":        pv.Participated,
		"interaction_seconds": pv.InteractionSeconds,
		"asset_category":      pv.AssetCategory,
		"created_at":          pv.CreatedAt,
	}
}

// parseSince reads an optional ?since= (RFC 3339 or YYYY-MM-DD) bound.
func parseSince(c *fiber.Ctx) (*time.Time, bool) {
	raw := strings.TrimSpace(c.Query("since"))
	if raw == "" {
		return nil, true
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return &t, true
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return &t, true
	}
	return nil, false
}

// GetCourseActivity handles GET /courses/:course_id/analytics/activity: the
// course's student page views and participations per day.
func (h *AnalyticsHandler) GetCourseActivity(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	since, ok := parseSince(c)
	if !ok {
		return responses.BadRequest(c, "Invalid since date")
	}

	activity, err := h.analyticsService.GetCourseActivity(c.Context(), uint(courseID), since)
	if err != nil {
		return responses.InternalError(c, "Could not fetch course activity")
	}

	return c.JSON(activity)
}

// GetCourseActivityCategories handles
// GET /courses/:course_id/analytics/activity/categories.
func (h *AnalyticsHandler) GetCourseActivityCategories(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	cats, err := h.analyticsService.GetCourseActivityCategories(c.Context(), uint(courseID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch course activity")
	}
	return c.JSON(cats)
}

// GetCourseAssignmentStats handles GET /courses/:course_id/analytics/assignments
func (h *AnalyticsHandler) GetCourseAssignmentStats(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	stats, err := h.analyticsService.GetCourseAssignmentStats(c.Context(), uint(courseID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch assignment statistics")
	}

	return c.JSON(stats)
}

// GetStudentSummaries handles GET /courses/:course_id/analytics/student_summaries
func (h *AnalyticsHandler) GetStudentSummaries(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	summaries, total, err := h.analyticsService.GetStudentSummaries(c.Context(), uint(courseID), callerAccountID(c), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch student summaries")
	}
	responses.SetPaginationHeaders(c, total, params.Page, params.PerPage)

	if callerID, _ := getUserID(c); callerID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, 0, "read", "student_summaries", "courses", uint(courseID), c.IP(), c.Get("User-Agent"))
	}

	return c.JSON(summaries)
}

// GetStudentActivity handles GET /courses/:course_id/analytics/users/:user_id/activity:
// one student's activity drill-down. A user who is not a student of the
// course (or not in the caller's tenant) is a 404.
func (h *AnalyticsHandler) GetStudentActivity(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	userID, err := c.ParamsInt("user_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	activity, err := h.analyticsService.GetStudentActivity(c.Context(), uint(courseID), uint(userID), callerAccountID(c))
	if errors.Is(err, service.ErrAnalyticsStudentNotFound) {
		return responses.NotFound(c, "student enrollment")
	}
	if err != nil {
		return responses.InternalError(c, "Could not fetch student activity")
	}

	if callerID, _ := getUserID(c); callerID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, uint(userID), "read", "student_activity", "courses", uint(courseID), c.IP(), c.Get("User-Agent"))
	}

	recent := make([]fiber.Map, len(activity.Recent))
	for i := range activity.Recent {
		recent[i] = pageViewToJSON(&activity.Recent[i])
	}
	return c.JSON(fiber.Map{
		"user_id":           activity.UserID,
		"by_day":            activity.ByDay,
		"categories":        activity.Categories,
		"recent_page_views": recent,
		"totals":            activity.Totals,
	})
}

// GetStudentAssignments handles GET /courses/:course_id/analytics/users/:user_id/assignments
func (h *AnalyticsHandler) GetStudentAssignments(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	userID, err := c.ParamsInt("user_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	assignments, err := h.analyticsService.GetStudentAssignments(c.Context(), uint(courseID), uint(userID), callerAccountID(c))
	if errors.Is(err, service.ErrAnalyticsStudentNotFound) {
		return responses.NotFound(c, "student enrollment")
	}
	if err != nil {
		return responses.InternalError(c, "Could not fetch student assignments")
	}

	if callerID, _ := getUserID(c); callerID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, uint(userID), "read", "student_assignments", "courses", uint(courseID), c.IP(), c.Get("User-Agent"))
	}

	return c.JSON(assignments)
}

// GetDepartmentActivity handles GET /accounts/:account_id/analytics/current/activity
func (h *AnalyticsHandler) GetDepartmentActivity(c *fiber.Ctx) error {
	accountID, err := c.ParamsInt("account_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	activity, err := h.analyticsService.GetDepartmentActivity(c.Context(), uint(accountID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch department activity")
	}

	return c.JSON(activity)
}

// GetDepartmentGrades handles GET /accounts/:account_id/analytics/current/grades
func (h *AnalyticsHandler) GetDepartmentGrades(c *fiber.Ctx) error {
	accountID, err := c.ParamsInt("account_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	grades, err := h.analyticsService.GetDepartmentGrades(c.Context(), uint(accountID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch department grades")
	}

	return c.JSON(grades)
}

// GetDepartmentStatistics handles GET /accounts/:account_id/analytics/current/statistics
func (h *AnalyticsHandler) GetDepartmentStatistics(c *fiber.Ctx) error {
	accountID, err := c.ParamsInt("account_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	statistics, err := h.analyticsService.GetDepartmentStatistics(c.Context(), uint(accountID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch department statistics")
	}

	return c.JSON(statistics)
}

// maxPageViewURL bounds the stored URL of a client-reported page view.
const maxPageViewURL = 2000

// CreatePageView handles POST /page_views: the web client reports a course
// page the user opened. Only course page views of users enrolled in that
// course are kept; anything else answers 204 with nothing recorded (the
// same answer for a course that does not exist, so nothing leaks).
// participated is ignored — participations are recorded server-side.
func (h *AnalyticsHandler) CreatePageView(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	var input struct {
		ContextType        string `json:"context_type"`
		ContextID          uint   `json:"context_id"`
		URL                string `json:"url"`
		Action             string `json:"action"`
		InteractionSeconds int    `json:"interaction_seconds"`
		AssetCategory      string `json:"asset_category"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	url := strings.TrimSpace(input.URL)
	if !strings.HasPrefix(url, "/") || strings.HasPrefix(url, "//") {
		return responses.BadRequest(c, "url must be an absolute path")
	}
	if len(url) > maxPageViewURL {
		url = url[:maxPageViewURL]
	}
	category := ""
	if input.AssetCategory != "" {
		category = service.AnalyticsCategory(input.AssetCategory)
	}
	action := input.Action
	if len(action) > 100 {
		action = action[:100]
	}

	pageView := &models.PageView{
		UserID:             userID,
		ContextType:        input.ContextType,
		ContextID:          input.ContextID,
		URL:                url,
		Action:             action,
		InteractionSeconds: input.InteractionSeconds,
		AssetCategory:      category,
	}

	// Masquerading admins browse as the student; don't count their views
	// as the student's activity.
	if realUser, ok := c.Locals("masquerade_by").(uint); ok && realUser != 0 {
		return c.SendStatus(fiber.StatusNoContent)
	}

	if err := h.analyticsService.RecordPageView(c.Context(), pageView, callerAccountID(c)); err != nil {
		if errors.Is(err, service.ErrPageViewNotTracked) {
			return c.SendStatus(fiber.StatusNoContent)
		}
		return responses.InternalError(c, "Could not record page view")
	}

	return c.Status(fiber.StatusCreated).JSON(pageViewToJSON(pageView))
}

// UpdatePageView handles PUT /page_views/:id: the client reports how long
// the page was open and visible. Only the caller's own page view can be
// updated; anything else is a 404.
func (h *AnalyticsHandler) UpdatePageView(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return responses.BadRequest(c, "Invalid page view ID")
	}
	var input struct {
		InteractionSeconds int `json:"interaction_seconds"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	if err := h.analyticsService.UpdatePageViewInteraction(c.Context(), uint(id), userID, input.InteractionSeconds); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return responses.NotFound(c, "page view")
		}
		return responses.InternalError(c, "Could not update page view")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ListUserPageViews handles GET /users/self/page_views
func (h *AnalyticsHandler) ListUserPageViews(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	params := middleware.GetPagination(c)

	result, err := h.analyticsService.GetUserPageViews(c.Context(), userID, callerAccountID(c), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch page views")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	pageViews := make([]fiber.Map, len(result.Items))
	for i, pv := range result.Items {
		pageViews[i] = pageViewToJSON(&pv)
	}

	return c.JSON(pageViews)
}

// participationCourseID returns the course of a matched course-scoped
// route: /courses/:course_id/... or /courses/:id/....
func participationCourseID(c *fiber.Ctx) uint {
	route := c.Route()
	if route == nil {
		return 0
	}
	path := route.Path
	idx := strings.Index(path, "/courses/:")
	if idx < 0 {
		return 0
	}
	rest := path[idx+len("/courses/:"):]
	name := rest
	if slash := strings.IndexByte(rest, '/'); slash >= 0 {
		name = rest[:slash]
	}
	if name != "course_id" && name != "id" {
		return 0
	}
	v, err := c.ParamsInt(name)
	if err != nil || v <= 0 {
		return 0
	}
	return uint(v)
}

// ParticipationRecorder is mounted once on the protected group (like
// AuditWrites). After a successful (2xx) POST/PUT/PATCH on a course-scoped
// route it records a participation for the caller when they are a student
// of that course — the "participations" series of course analytics. It
// never changes the response, runs off the request path, and skips
// masquerade sessions. A nil handler is a pass-through.
func (h *AnalyticsHandler) ParticipationRecorder() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if err := c.Next(); err != nil {
			return err
		}
		if h == nil || h.analyticsService == nil {
			return nil
		}
		method := c.Method()
		if method != fiber.MethodPost && method != fiber.MethodPut && method != fiber.MethodPatch {
			return nil
		}
		status := c.Response().StatusCode()
		if status < 200 || status >= 300 {
			return nil
		}
		if realUser, ok := c.Locals("masquerade_by").(uint); ok && realUser != 0 {
			return nil
		}
		userID, _ := c.Locals("user_id").(uint)
		accountID, _ := c.Locals("account_id").(uint)
		courseID := participationCourseID(c)
		if userID == 0 || accountID == 0 || courseID == 0 {
			return nil
		}
		// Copy what the goroutine needs: fasthttp reuses the request
		// context once the handler returns.
		path := strings.Clone(c.Path())
		routePath := strings.Clone(c.Route().Path)
		go func() {
			defer func() { _ = recover() }()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			h.analyticsService.RecordParticipation(ctx, userID, courseID, accountID, method, path, routePath)
		}()
		return nil
	}
}
