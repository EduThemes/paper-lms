package handlers

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type CourseHandler struct {
	courseService     *service.CourseService
	enrollmentService *service.EnrollmentService
}

func NewCourseHandler(courseService *service.CourseService, enrollmentService *service.EnrollmentService) *CourseHandler {
	return &CourseHandler{
		courseService:     courseService,
		enrollmentService: enrollmentService,
	}
}

func courseToJSON(c *models.Course) fiber.Map {
	// Parse navigation_tabs JSON string into a raw value for the response
	var navTabs interface{}
	if c.NavigationTabs != "" {
		if err := json.Unmarshal([]byte(c.NavigationTabs), &navTabs); err != nil {
			navTabs = nil
		}
	}

	return fiber.Map{
		"id":             c.ID,
		"account_id":     c.AccountID,
		"name":           c.Name,
		"course_code":    c.CourseCode,
		"workflow_state": c.WorkflowState,
		"start_at":       c.StartAt,
		"end_at":         c.EndAt,
		"default_view":   c.DefaultView,
		"syllabus_body":  c.SyllabusBody,
		"license":        c.License,
		"is_public":      c.IsPublic,
		"ui_mode":                          c.UIMode,
		"apply_assignment_group_weights":   c.ApplyGroupWeights,
		"navigation_tabs":                  navTabs,
		"created_at":                       c.CreatedAt,
	}
}

func (h *CourseHandler) ListCourses(c *fiber.Ctx) error {
	params := middleware.GetPagination(c)
	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	var items []models.Course
	var totalCount int64
	var page, perPage int

	// Default: return user's enrolled courses (matches Canvas behavior)
	// Use ?scope=all to get all courses (admin use case, e.g. course browser)
	if c.Query("scope") == "all" {
		r, err := h.courseService.List(c.Context(), params)
		if err != nil {
			return responses.InternalError(c, "Could not fetch courses")
		}
		items = r.Items
		totalCount = r.TotalCount
		page = r.Page
		perPage = r.PerPage
	} else {
		r, err := h.courseService.ListForUser(c.Context(), userID, params)
		if err != nil {
			return responses.InternalError(c, "Could not fetch courses")
		}
		items = r.Items
		totalCount = r.TotalCount
		page = r.Page
		perPage = r.PerPage
	}

	responses.SetPaginationHeaders(c, totalCount, page, perPage)

	// Batch-fetch student enrollment counts (single GROUP BY query)
	courseIDs := make([]uint, len(items))
	for i, course := range items {
		courseIDs[i] = course.ID
	}
	studentCounts, _ := h.enrollmentService.CountStudentsByCourseIDs(c.Context(), courseIDs)
	if studentCounts == nil {
		studentCounts = map[uint]int64{}
	}

	courses := make([]fiber.Map, len(items))
	for i, course := range items {
		cj := courseToJSON(&course)
		cj["total_students"] = studentCounts[course.ID]
		courses[i] = cj
	}

	return c.JSON(courses)
}

func (h *CourseHandler) GetCourse(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	course, err := h.courseService.GetByID(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "course")
	}

	return c.JSON(courseToJSON(course))
}

type createCourseRequest struct {
	Course struct {
		Name          string     `json:"name"`
		CourseCode    string     `json:"course_code"`
		StartAt       *time.Time `json:"start_at"`
		EndAt         *time.Time `json:"end_at"`
		DefaultView   string     `json:"default_view"`
		SyllabusBody  string     `json:"syllabus_body"`
		License       string     `json:"license"`
		IsPublic          bool       `json:"is_public"`
		UIMode            string     `json:"ui_mode"`
		ApplyGroupWeights bool       `json:"apply_assignment_group_weights"`
	} `json:"course"`
}

func (h *CourseHandler) CreateCourse(c *fiber.Ctx) error {
	var input createCourseRequest
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if strings.TrimSpace(input.Course.Name) == "" {
		return responses.BadRequest(c, "Course name is required")
	}

	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	course := &models.Course{
		Name:         input.Course.Name,
		CourseCode:   input.Course.CourseCode,
		StartAt:      input.Course.StartAt,
		EndAt:        input.Course.EndAt,
		DefaultView:  input.Course.DefaultView,
		SyllabusBody: input.Course.SyllabusBody,
		License:      input.Course.License,
		IsPublic:          input.Course.IsPublic,
		UIMode:            input.Course.UIMode,
		ApplyGroupWeights: input.Course.ApplyGroupWeights,
	}

	if course.DefaultView == "" {
		course.DefaultView = "modules"
	}
	if course.UIMode == "" {
		course.UIMode = "standard"
	}

	if err := h.courseService.Create(c.Context(), course, userID); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(courseToJSON(course))
}

func (h *CourseHandler) UpdateCourse(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	course, err := h.courseService.GetByID(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "course")
	}

	var input struct {
		Course struct {
			Name          *string    `json:"name"`
			CourseCode    *string    `json:"course_code"`
			StartAt       *time.Time `json:"start_at"`
			EndAt         *time.Time `json:"end_at"`
			DefaultView   *string    `json:"default_view"`
			SyllabusBody  *string    `json:"syllabus_body"`
			License       *string    `json:"license"`
			IsPublic      *bool      `json:"is_public"`
			UIMode            *string          `json:"ui_mode"`
			WorkflowState     *string          `json:"workflow_state"`
			ApplyGroupWeights *bool            `json:"apply_assignment_group_weights"`
			NavigationTabs    *json.RawMessage `json:"navigation_tabs"`
		} `json:"course"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Course.Name != nil {
		course.Name = *input.Course.Name
	}
	if input.Course.CourseCode != nil {
		course.CourseCode = *input.Course.CourseCode
	}
	if input.Course.StartAt != nil {
		course.StartAt = input.Course.StartAt
	}
	if input.Course.EndAt != nil {
		course.EndAt = input.Course.EndAt
	}
	if input.Course.DefaultView != nil {
		course.DefaultView = *input.Course.DefaultView
	}
	if input.Course.SyllabusBody != nil {
		course.SyllabusBody = *input.Course.SyllabusBody
	}
	if input.Course.License != nil {
		course.License = *input.Course.License
	}
	if input.Course.IsPublic != nil {
		course.IsPublic = *input.Course.IsPublic
	}
	if input.Course.UIMode != nil {
		course.UIMode = *input.Course.UIMode
	}
	if input.Course.WorkflowState != nil {
		course.WorkflowState = *input.Course.WorkflowState
	}
	if input.Course.ApplyGroupWeights != nil {
		course.ApplyGroupWeights = *input.Course.ApplyGroupWeights
	}
	if input.Course.NavigationTabs != nil {
		course.NavigationTabs = string(*input.Course.NavigationTabs)
	}

	if err := h.courseService.Update(c.Context(), course); err != nil {
		return responses.InternalError(c, "Could not update course")
	}

	return c.JSON(courseToJSON(course))
}

func (h *CourseHandler) DeleteCourse(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	if err := h.courseService.Delete(c.Context(), uint(id)); err != nil {
		return responses.InternalError(c, "Could not delete course")
	}

	return c.JSON(fiber.Map{"delete": true})
}
