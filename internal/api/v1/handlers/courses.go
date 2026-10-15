package handlers

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

type CourseHandler struct {
	courseService     *service.CourseService
	enrollmentService *service.EnrollmentService
	// userRepo backs the DB role check (middleware.EffectiveRole) that
	// gates ?scope=all. Nil means nobody gets the tenant-wide list.
	userRepo repository.UserRepository
	// unpublishGuard (optional, G077) refuses unpublishing a course with
	// graded student work; see SetUnpublishGuard.
	unpublishGuard UnpublishGuard
	// quotas (optional, G072/G120) backs storage_quota_mb; see
	// SetStorageQuotas.
	quotas CourseStorageQuotas
	// canvasAPI adds Canvas course fields / include[] (G137); see
	// canvas_course_includes.go. Nil leaves the JSON as is.
	canvasAPI *service.CanvasAPIService
	// Page Designer (course_look.go): banner files of the course and the
	// custom_css_enabled setting. See SetPageDesigner.
	lookFiles      repository.AttachmentRepository
	settingsLookup service.SettingsLookupFunc
}

// SetUserRepo wires the user repository used for the admin check on
// GET /courses?scope=all.
func (h *CourseHandler) SetUserRepo(r repository.UserRepository) *CourseHandler {
	h.userRepo = r
	return h
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
		"id":                 c.ID,
		"account_id":         c.AccountID,
		"name":               c.Name,
		"course_code":        c.CourseCode,
		"workflow_state":     c.WorkflowState,
		"enrollment_term_id": c.EnrollmentTermID,
		"start_at":           c.StartAt,
		"end_at":         c.EndAt,
		"default_view":   c.DefaultView,
		"syllabus_body":  c.SyllabusBody,
		"license":        c.License,
		"is_public":      c.IsPublic,
		"ui_mode":                          c.UIMode,
		"apply_assignment_group_weights":   c.ApplyGroupWeights,
		"hide_final_grades":                c.HideFinalGrades,
		"navigation_tabs":                  navTabs,
		"created_at":                       c.CreatedAt,
		// Date-based access settings (G106).
		"restrict_enrollments_to_course_dates": c.RestrictEnrollmentsToCourseDates,
		"restrict_student_future_view":         c.RestrictStudentFutureView,
		"restrict_student_past_view":           c.RestrictStudentPastView,
		// Page Designer course look (null = Paper's generated header).
		"look": c.Look,
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

	// Default: return user's enrolled courses (matches Canvas behavior).
	// ?scope=all lists every course in the tenant (the admin course
	// browser), so it is honored only for admins — the DB role, capped
	// during masquerade. Anyone else gets their own courses, as without it.
	scopeAll := false
	if c.Query("scope") == "all" {
		switch middleware.EffectiveRole(c, h.userRepo) {
		case middleware.RoleAdmin, middleware.RoleSuperAdmin:
			scopeAll = true
		}
	}
	if scopeAll {
		r, err := h.courseService.List(c.Context(), callerAccountID(c), params)
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
	studentCounts, _ := h.enrollmentService.CountStudentsByCourseIDs(c.Context(), courseIDs, callerAccountID(c))
	if studentCounts == nil {
		studentCounts = map[uint]int64{}
	}

	courses := make([]fiber.Map, len(items))
	for i, course := range items {
		cj := courseToJSON(&course)
		cj["total_students"] = studentCounts[course.ID]
		courses[i] = cj
	}
	h.applyCourseIncludes(c, items, courses)

	return c.JSON(courses)
}

func (h *CourseHandler) GetCourse(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	course, err := h.courseService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "course")
	}

	out := h.courseJSONWithQuota(c, course.ID, courseToJSON(course))
	h.withDesignerFields(c, course, out)
	h.applyCourseIncludes(c, []models.Course{*course}, []fiber.Map{out})
	return c.JSON(out)
}

type createCourseRequest struct {
	Course struct {
		Name             string     `json:"name"`
		CourseCode       string     `json:"course_code"`
		EnrollmentTermID *uint      `json:"enrollment_term_id"`
		StartAt          *time.Time `json:"start_at"`
		EndAt            *time.Time `json:"end_at"`
		DefaultView      string     `json:"default_view"`
		SyllabusBody     string     `json:"syllabus_body"`
		License          string     `json:"license"`
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
		Name:             input.Course.Name,
		CourseCode:       input.Course.CourseCode,
		EnrollmentTermID: input.Course.EnrollmentTermID,
		StartAt:          input.Course.StartAt,
		EndAt:            input.Course.EndAt,
		DefaultView:      input.Course.DefaultView,
		SyllabusBody:     service.SanitizeHTML(input.Course.SyllabusBody),
		License:          input.Course.License,
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

	course, err := h.courseService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "course")
	}

	var input struct {
		Course struct {
			Name          *string    `json:"name"`
			CourseCode       *string    `json:"course_code"`
			EnrollmentTermID *uint      `json:"enrollment_term_id"`
			StartAt          *time.Time `json:"start_at"`
			EndAt            *time.Time `json:"end_at"`
			DefaultView      *string    `json:"default_view"`
			SyllabusBody  *string    `json:"syllabus_body"`
			License       *string    `json:"license"`
			IsPublic      *bool      `json:"is_public"`
			UIMode            *string          `json:"ui_mode"`
			WorkflowState     *string          `json:"workflow_state"`
			ApplyGroupWeights *bool            `json:"apply_assignment_group_weights"`
			HideFinalGrades   *bool            `json:"hide_final_grades"`
			NavigationTabs    *json.RawMessage `json:"navigation_tabs"`
			StorageQuotaMB    json.RawMessage  `json:"storage_quota_mb"` // not a pointer: null must arrive as "null"
			// Date-based access settings (G106).
			RestrictEnrollmentsToCourseDates *bool `json:"restrict_enrollments_to_course_dates"`
			RestrictStudentFutureView        *bool `json:"restrict_student_future_view"`
			RestrictStudentPastView          *bool `json:"restrict_student_past_view"`
			// Page Designer course look; null clears it (course_look.go).
			Look json.RawMessage `json:"look"`
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
	if input.Course.EnrollmentTermID != nil {
		course.EnrollmentTermID = input.Course.EnrollmentTermID
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
		course.SyllabusBody = service.SanitizeHTML(*input.Course.SyllabusBody)
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
		// F-018: workflow_state changes are gated by the model's
		// state machine. An attempted invalid transition (e.g.,
		// deleted→published) or one disallowed for the caller's role
		// is rejected as 400; the requested value is NOT silently
		// dropped — that would mask a misbehaving client.
		newState := models.CourseWorkflow(*input.Course.WorkflowState)
		if err := models.TransitionCourseWorkflowState(course.WorkflowState, newState, callerRoleForStateMachine(c)); err != nil {
			return responses.BadRequest(c, err.Error())
		}
		if wrote, gerr := h.checkUnpublishable(c, course, newState); wrote {
			return gerr
		}
		course.WorkflowState = newState
	}
	if input.Course.ApplyGroupWeights != nil {
		course.ApplyGroupWeights = *input.Course.ApplyGroupWeights
	}
	if input.Course.HideFinalGrades != nil {
		course.HideFinalGrades = *input.Course.HideFinalGrades
	}
	if input.Course.NavigationTabs != nil {
		course.NavigationTabs = string(*input.Course.NavigationTabs)
	}
	if len(input.Course.StorageQuotaMB) > 0 {
		// G072: only admins set a course's storage quota (Canvas
		// manage_storage_quotas); null resets it to the account default.
		if wrote, qerr := applyStorageQuotaMB(c, h.userRepo, h.quotas, course.ID, input.Course.StorageQuotaMB); wrote {
			return qerr
		}
	}
	applyCourseDateAccessSettings(course, input.Course.RestrictEnrollmentsToCourseDates, input.Course.RestrictStudentFutureView, input.Course.RestrictStudentPastView)
	if wrote, lerr := h.applyCourseLook(c, course, input.Course.Look); wrote {
		return lerr
	}

	if err := h.courseService.Update(c.Context(), course); err != nil {
		return responses.InternalError(c, "Could not update course")
	}

	return c.JSON(h.withDesignerFields(c, course, h.courseJSONWithQuota(c, course.ID, courseToJSON(course))))
}

func (h *CourseHandler) DeleteCourse(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	// courseService.Delete is keyed by bare id; confirm the course is in
	// the caller's tenant first (404 across tenants), independent of the
	// route middleware.
	if _, err := h.courseService.GetByID(c.Context(), uint(id), callerAccountID(c)); err != nil {
		return responses.NotFound(c, "course")
	}

	if err := h.courseService.Delete(c.Context(), uint(id)); err != nil {
		return responses.InternalError(c, "Could not delete course")
	}

	return c.JSON(fiber.Map{"delete": true})
}
