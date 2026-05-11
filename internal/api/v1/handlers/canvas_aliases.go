package handlers

// CanvasAliasHandler provides Canvas-canonical route aliases for endpoints
// that the Canvas LMS API exposes under `/users/self/...` and `/dashboard/...`.
//
// These routes mirror existing course / enrollment / planner data from the
// authenticated user's perspective so that Canvas-compatible frontend code
// (and third-party clients ported from Canvas) can hit the same URLs without
// rewriting.
//
// Endpoints exposed by this handler:
//   GET /api/v1/users/self/courses       -> SelfCourses
//   GET /api/v1/users/self/enrollments   -> SelfEnrollments
//   GET /api/v1/users/self/todo          -> SelfTodo
//   GET /api/v1/dashboard/dashboard_cards-> DashboardCards
//
// All routes require auth middleware to have populated c.Locals("user_id").

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/service"
)

type CanvasAliasHandler struct {
	courseService     *service.CourseService
	enrollmentService *service.EnrollmentService
	plannerService    *service.PlannerService
}

func NewCanvasAliasHandler(
	courseService *service.CourseService,
	enrollmentService *service.EnrollmentService,
	plannerService *service.PlannerService,
) *CanvasAliasHandler {
	return &CanvasAliasHandler{
		courseService:     courseService,
		enrollmentService: enrollmentService,
		plannerService:    plannerService,
	}
}

// SelfCourses handles GET /api/v1/users/self/courses
// Canvas alias for "list the current user's enrolled courses".
// Mirrors CourseHandler.ListCourses with scope implicitly set to the current user.
func (h *CanvasAliasHandler) SelfCourses(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	params := middleware.GetPagination(c)
	result, err := h.courseService.ListForUser(c.Context(), userID, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch courses")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	// Optional: include enrollment student counts (matches /api/v1/courses behavior)
	courseIDs := make([]uint, len(result.Items))
	for i, course := range result.Items {
		courseIDs[i] = course.ID
	}
	studentCounts, _ := h.enrollmentService.CountStudentsByCourseIDs(c.Context(), courseIDs)
	if studentCounts == nil {
		studentCounts = map[uint]int64{}
	}

	courses := make([]fiber.Map, len(result.Items))
	for i, course := range result.Items {
		cj := courseToJSON(&course)
		cj["total_students"] = studentCounts[course.ID]
		courses[i] = cj
	}

	return c.JSON(courses)
}

// SelfEnrollments handles GET /api/v1/users/self/enrollments
// Canvas alias for "list the current user's enrollments across all courses".
func (h *CanvasAliasHandler) SelfEnrollments(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	enrollments, err := h.enrollmentService.ListByUser(c.Context(), userID)
	if err != nil {
		return responses.InternalError(c, "Could not fetch enrollments")
	}

	out := make([]fiber.Map, len(enrollments))
	for i := range enrollments {
		out[i] = enrollmentToJSON(&enrollments[i])
	}

	return c.JSON(out)
}

// SelfTodo handles GET /api/v1/users/self/todo
// Canvas alias for "list todo items (assignments due, quizzes upcoming,
// ungraded discussions, etc.) for the current user".
//
// We delegate to the planner aggregator with a forward-looking window
// (today through ~4 weeks ahead) and surface the results in the Canvas
// `todo` shape: a flat array of objects with `type`, `assignment`/`quiz`,
// `course_id`, `html_url`, etc.
//
// NOTE: This is a thin shim on top of PlannerService.GetPlannerItems.
// If a future Canvas-strict todo aggregator is added (e.g. filtered to
// only un-submitted graded items + ungraded teacher items), it should
// replace this implementation. For now this unblocks the frontend by
// returning a sensible non-empty payload instead of a 404.
func (h *CanvasAliasHandler) SelfTodo(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	// Default window: now through 4 weeks ahead. Canvas `/users/self/todo`
	// is "what needs your attention soon", so past items are excluded.
	now := time.Now()
	startDate := now
	endDate := now.AddDate(0, 0, 28)

	if h.plannerService == nil {
		// TODO(canvas-aliases): wire PlannerService once available; for now
		// return an empty array so the frontend can render an empty todo state
		// rather than a 404.
		return c.JSON([]fiber.Map{})
	}

	items, err := h.plannerService.GetPlannerItems(c.Context(), userID, startDate, endDate)
	if err != nil {
		return responses.InternalError(c, "Could not fetch todo items")
	}

	// Translate planner items -> Canvas todo shape.
	// Only items with a plannable_type that maps to a Canvas todo bucket
	// (assignment, quiz, discussion_topic) are included; calendar events
	// and planner notes are skipped because Canvas's /users/self/todo
	// doesn't include them.
	out := make([]fiber.Map, 0, len(items))
	for _, item := range items {
		var todoType string
		switch item.PlannableType {
		case "assignment":
			todoType = "submitting"
		case "quiz":
			todoType = "submitting"
		case "discussion_topic":
			todoType = "submitting"
		default:
			continue
		}

		entry := fiber.Map{
			"type":             todoType,
			"plannable_type":   item.PlannableType,
			"plannable_id":     item.PlannableID,
			"plannable_date":   item.PlannableDate,
			"context_name":     item.ContextName,
			"course_id":        item.CourseID,
			"needs_grading_count": 0,
		}
		// Embed the underlying object under its Canvas-canonical key.
		switch item.PlannableType {
		case "assignment":
			entry["assignment"] = item.Plannable
		case "quiz":
			entry["quiz"] = item.Plannable
		case "discussion_topic":
			entry["discussion_topic"] = item.Plannable
		}
		out = append(out, entry)
	}

	return c.JSON(out)
}

// DashboardCards handles GET /api/v1/dashboard/dashboard_cards
// Canvas alias that returns the current user's enrolled courses formatted
// as dashboard cards.
//
// Canvas dashboard card shape:
//   { id, courseCode, shortName, originalName, term, position,
//     image, isFavorited, links, ... }
//
// Until per-user favoriting and term metadata are wired up we return all
// active enrollments as cards; isFavorited defaults to true so every
// enrolled course shows on the dashboard (Canvas behavior when a user
// has not curated their dashboard).
func (h *CanvasAliasHandler) DashboardCards(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	// Pull a generous page; dashboard cards are not paginated in Canvas.
	params := middleware.GetPagination(c)
	if params.PerPage < 100 {
		params.PerPage = 100
	}
	if params.Page < 1 {
		params.Page = 1
	}

	result, err := h.courseService.ListForUser(c.Context(), userID, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch dashboard cards")
	}

	cards := make([]fiber.Map, 0, len(result.Items))
	for i, course := range result.Items {
		cards = append(cards, fiber.Map{
			"id":             course.ID,
			"longName":       course.Name,
			"shortName":      course.Name,
			"originalName":   course.Name,
			"courseCode":     course.CourseCode,
			"assetString":    canvasAssetString("course", course.ID),
			"href":           "/courses/" + uintToStr(course.ID),
			"term":           "",
			"subtitle":       course.CourseCode,
			"image":          nil,
			"color":          nil,
			"position":       i,
			"isFavorited":    true,
			"isK5Subject":    false,
			"isHomeroom":     false,
			"useClassicFont": false,
			"canManage":      false,
			"canReadAnnouncements": true,
			"published":      course.WorkflowState == "available",
			"links":          dashboardCardLinks(course.ID),
		})
	}

	return c.JSON(cards)
}

// --- small local helpers (kept private to this file) ---

func canvasAssetString(kind string, id uint) string {
	return kind + "_" + uintToStr(id)
}

func uintToStr(n uint) string {
	if n == 0 {
		return "0"
	}
	// avoid pulling strconv into a hot path twice; small inline impl
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func dashboardCardLinks(courseID uint) []fiber.Map {
	cid := uintToStr(courseID)
	return []fiber.Map{
		{"label": "Announcements", "icon": "icon-announcement", "path": "/courses/" + cid + "/announcements"},
		{"label": "Assignments", "icon": "icon-assignment", "path": "/courses/" + cid + "/assignments"},
		{"label": "Discussions", "icon": "icon-discussion", "path": "/courses/" + cid + "/discussion_topics"},
		{"label": "Grades", "icon": "icon-gradebook", "path": "/courses/" + cid + "/grades"},
		{"label": "Files", "icon": "icon-folder", "path": "/courses/" + cid + "/files"},
	}
}
