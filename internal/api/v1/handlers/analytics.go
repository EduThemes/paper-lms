package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type AnalyticsHandler struct {
	analyticsService *service.AnalyticsService
}

func NewAnalyticsHandler(analyticsService *service.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{analyticsService: analyticsService}
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
		"created_at":          pv.CreatedAt,
	}
}

// GetCourseActivity handles GET /courses/:course_id/analytics/activity
func (h *AnalyticsHandler) GetCourseActivity(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	activity, err := h.analyticsService.GetCourseActivity(c.Context(), uint(courseID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch course activity")
	}

	return c.JSON(activity)
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

	summaries, err := h.analyticsService.GetStudentSummaries(c.Context(), uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch student summaries")
	}

	return c.JSON(summaries)
}

// GetStudentActivity handles GET /courses/:course_id/analytics/users/:user_id/activity
func (h *AnalyticsHandler) GetStudentActivity(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	userID, err := c.ParamsInt("user_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	activity, err := h.analyticsService.GetStudentActivity(c.Context(), uint(courseID), uint(userID))
	if err != nil {
		return responses.NotFound(c, "student enrollment")
	}

	return c.JSON(activity)
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

	assignments, err := h.analyticsService.GetStudentAssignments(c.Context(), uint(courseID), uint(userID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch student assignments")
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

// CreatePageView handles POST /page_views
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
		Participated       bool   `json:"participated"`
		InteractionSeconds int    `json:"interaction_seconds"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	pageView := &models.PageView{
		UserID:             userID,
		ContextType:        input.ContextType,
		ContextID:          input.ContextID,
		URL:                input.URL,
		Action:             input.Action,
		Participated:       input.Participated,
		InteractionSeconds: input.InteractionSeconds,
	}

	if err := h.analyticsService.RecordPageView(c.Context(), pageView); err != nil {
		return responses.InternalError(c, "Could not record page view")
	}

	return c.Status(fiber.StatusCreated).JSON(pageViewToJSON(pageView))
}

// ListUserPageViews handles GET /users/self/page_views
func (h *AnalyticsHandler) ListUserPageViews(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	params := middleware.GetPagination(c)

	result, err := h.analyticsService.GetUserPageViews(c.Context(), userID, params)
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
