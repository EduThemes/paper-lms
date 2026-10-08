package handlers

import (
	"context"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

// observerChecker is the narrow observer-link check the submissions feed
// needs (satisfied by *service.ObserverService).
type observerChecker interface {
	IsObserverOf(ctx context.Context, observerUserID, studentUserID uint) (bool, error)
}

// CanvasAPIHandler serves the Canvas collection endpoints that scripts
// and integrations call (G138):
//
//	GET /courses/:course_id/users (and /search_users)
//	GET /courses/:course_id/students/submissions
//	GET /accounts/:account_id/courses
//	GET /accounts/:account_id/users
type CanvasAPIHandler struct {
	svc       *service.CanvasAPIService
	observers observerChecker
}

// NewCanvasAPIHandler wires the handler. observers may be nil (no observer
// access to the submissions feed).
func NewCanvasAPIHandler(svc *service.CanvasAPIService, observers observerChecker) *CanvasAPIHandler {
	return &CanvasAPIHandler{svc: svc, observers: observers}
}

// callerSeesCourseStaffData reports whether the caller may see login ids,
// SIS ids, emails and non-active enrollments on a course roster: admins
// and the course's teachers, TAs and designers.
func callerSeesCourseStaffData(c *fiber.Ctx) bool {
	if callerIsCourseStaff(c) {
		return true
	}
	et, _ := c.Locals("enrollment_type").(string)
	return et == "DesignerEnrollment"
}

// urlAccountScope returns the tenant an /accounts/:account_id route acts
// on. RequireAdmin has already verified it is the caller's own account
// (or that the caller is a super admin); CanvasIDs has already turned
// "self" and sis_account_id: into the numeric id.
func urlAccountScope(c *fiber.Ctx) (uint, bool) {
	raw := c.Params("account_id")
	if raw == "self" || raw == "" {
		return callerAccountID(c), true
	}
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || n == 0 {
		return 0, false
	}
	return uint(n), true
}

func canvasEnrollmentJSON(e *models.Enrollment) fiber.Map {
	return fiber.Map{
		"id":                 e.ID,
		"course_id":          e.CourseID,
		"course_section_id":  e.CourseSectionID,
		"user_id":            e.UserID,
		"type":               e.Type,
		"role":               e.Role,
		"enrollment_state":   e.WorkflowState,
		"associated_user_id": e.AssociatedUserID,
		"created_at":         e.CreatedAt,
		"updated_at":         e.UpdatedAt,
		"last_activity_at":   e.LastActivityAt,
	}
}

// canvasUserJSON is Canvas's User object. Login and SIS ids are only shown
// to callers who can manage the user (staff / admins); email and avatar
// follow include[].
func canvasUserJSON(u *models.User, inc map[string]bool, privileged bool) fiber.Map {
	m := fiber.Map{
		"id":            u.ID,
		"name":          u.Name,
		"sortable_name": u.SortableName,
		"short_name":    u.ShortName,
		"created_at":    u.CreatedAt,
	}
	if privileged {
		m["sis_user_id"] = u.SISUserID
		m["login_id"] = u.LoginID
		if inc["email"] {
			m["email"] = u.Email
		}
	}
	if inc["avatar_url"] {
		m["avatar_url"] = u.AvatarURL
	}
	if inc["locale"] {
		m["locale"] = u.Locale
	}
	return m
}

// ListCourseUsers returns the course roster.
// GET /api/v1/courses/:course_id/users (enrolled)
//
// Canvas parameters: search_term, sort (username|email|sis_id),
// enrollment_type[] (student, teacher, ta, observer, designer),
// enrollment_state[] (active, invited, rejected, completed, inactive;
// staff only), user_ids[], include[] (enrollments, email, avatar_url).
// Students only see active members.
func (h *CanvasAPIHandler) ListCourseUsers(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}
	staff := callerSeesCourseStaffData(c)
	inc := includeSet(c)

	types, bad, ok := canvasEnrollmentTypes(queryList(c, "enrollment_type"))
	if !ok {
		return responses.BadRequest(c, "Invalid enrollment_type "+strconv.Quote(bad))
	}
	userIDs, bad, ok := queryUintList(c, "user_ids")
	if !ok {
		return responses.BadRequest(c, "Invalid user id "+strconv.Quote(bad))
	}
	var states []string
	if staff {
		for _, s := range queryList(c, "enrollment_state") {
			switch models.EnrollmentWorkflow(s) {
			case models.EnrollmentActive, models.EnrollmentInvited, models.EnrollmentRejected,
				models.EnrollmentCompleted, models.EnrollmentInactive:
				states = append(states, s)
			default:
				return responses.BadRequest(c, "Invalid enrollment_state "+strconv.Quote(s))
			}
		}
	} else {
		states = []string{string(models.EnrollmentActive)}
	}

	searchTerm := strings.TrimSpace(c.Query("search_term"))
	if searchTerm != "" && len([]rune(searchTerm)) < 2 {
		return responses.BadRequest(c, "search_term must be at least 2 characters")
	}
	sort := c.Query("sort")
	if !staff && (sort == "email" || sort == "sis_id") {
		sort = ""
	}

	filter := repository.CourseUserFilter{
		EnrollmentTypes:  types,
		EnrollmentStates: states,
		UserIDs:          userIDs,
		SearchTerm:       searchTerm,
		Sort:             sort,
	}
	params := middleware.GetPagination(c)
	rows, total, err := h.svc.ListCourseUsers(c.Context(), uint(courseID), filter, params, callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not fetch course users")
	}
	responses.SetPaginationHeaders(c, total, params.Page, params.PerPage)

	out := make([]fiber.Map, len(rows))
	for i := range rows {
		m := canvasUserJSON(&rows[i].User, inc, staff)
		if inc["enrollments"] {
			enr := make([]fiber.Map, len(rows[i].Enrollments))
			for j := range rows[i].Enrollments {
				enr[j] = canvasEnrollmentJSON(&rows[i].Enrollments[j])
			}
			m["enrollments"] = enr
		}
		out[i] = m
	}
	return c.JSON(out)
}

// canvasCourseJSON is courseToJSON plus the Canvas fields scripts read
// (sis_course_id) and the include[] extras.
func canvasCourseJSON(row *service.AccountCourse, inc map[string]bool) fiber.Map {
	m := courseToJSON(&row.Course)
	m["sis_course_id"] = row.Course.SISCourseID
	if !inc["syllabus_body"] {
		delete(m, "syllabus_body")
	}
	if inc["total_students"] {
		m["total_students"] = row.TotalStudents
	}
	if inc["term"] {
		if row.Term != nil {
			m["term"] = fiber.Map{
				"id":             row.Term.ID,
				"name":           row.Term.Name,
				"start_at":       row.Term.StartAt,
				"end_at":         row.Term.EndAt,
				"workflow_state": row.Term.WorkflowState,
				"sis_term_id":    row.Term.SISTermID,
			}
		} else {
			m["term"] = nil
		}
	}
	if inc["teachers"] {
		teachers := make([]fiber.Map, len(row.Teachers))
		for i, t := range row.Teachers {
			teachers[i] = fiber.Map{
				"id":               t.ID,
				"display_name":     t.Name,
				"avatar_image_url": t.AvatarURL,
			}
		}
		m["teachers"] = teachers
	}
	return m
}

// ListAccountCourses lists the account's courses.
// GET /api/v1/accounts/:account_id/courses (admin)
//
// Canvas parameters: search_term, enrollment_term_id, published,
// completed, with_enrollments, state[] (created, claimed, available,
// completed, deleted, all), by_teachers[], sort (course_name,
// sis_course_id, teacher, term), order (asc|desc), include[] (term,
// teachers, total_students, syllabus_body).
func (h *CanvasAPIHandler) ListAccountCourses(c *fiber.Ctx) error {
	accountID, ok := urlAccountScope(c)
	if !ok {
		return responses.NotFound(c, "account")
	}
	inc := includeSet(c)

	f := repository.AccountCourseFilter{
		SearchTerm:      strings.TrimSpace(c.Query("search_term")),
		Published:       queryBool(c, "published"),
		Completed:       queryBool(c, "completed"),
		WithEnrollments: queryBool(c, "with_enrollments"),
		Sort:            c.Query("sort"),
		Order:           c.Query("order"),
	}
	if f.SearchTerm != "" && len([]rune(f.SearchTerm)) < 2 {
		return responses.BadRequest(c, "search_term must be at least 2 characters")
	}
	if raw := c.Query("enrollment_term_id"); raw != "" {
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return responses.BadRequest(c, "Invalid enrollment_term_id")
		}
		id := uint(n)
		f.EnrollmentTermID = &id
	}
	for _, s := range queryList(c, "state") {
		switch s {
		case "all":
			f.States = []string{
				string(models.CourseCreated), string(models.CourseClaimed), string(models.CourseUnpublished),
				string(models.CourseAvailable), string(models.CourseCompleted), string(models.CourseDeleted),
			}
		case "created", "claimed", "unpublished":
			f.States = append(f.States, string(models.CourseCreated), string(models.CourseClaimed), string(models.CourseUnpublished))
		case "available", "completed", "deleted":
			f.States = append(f.States, s)
		default:
			return responses.BadRequest(c, "Invalid state "+strconv.Quote(s))
		}
	}
	teachers, bad, ok := queryUintList(c, "by_teachers")
	if !ok {
		return responses.BadRequest(c, "Invalid teacher id "+strconv.Quote(bad))
	}
	f.ByTeachers = teachers

	params := middleware.GetPagination(c)
	rows, total, err := h.svc.ListAccountCourses(c.Context(), f, service.AccountCourseIncludes{
		Term:          inc["term"],
		Teachers:      inc["teachers"],
		TotalStudents: inc["total_students"],
	}, params, accountID)
	if err != nil {
		return responses.InternalError(c, "Could not fetch courses")
	}
	responses.SetPaginationHeaders(c, total, params.Page, params.PerPage)
	out := make([]fiber.Map, len(rows))
	for i := range rows {
		out[i] = canvasCourseJSON(&rows[i], inc)
	}
	return c.JSON(out)
}

// ListAccountUsers lists the account's users.
// GET /api/v1/accounts/:account_id/users (admin)
//
// Canvas parameters: search_term, sort (username, email, sis_id,
// last_login), order (asc|desc), include[] (email, avatar_url, locale).
func (h *CanvasAPIHandler) ListAccountUsers(c *fiber.Ctx) error {
	accountID, ok := urlAccountScope(c)
	if !ok {
		return responses.NotFound(c, "account")
	}
	inc := includeSet(c)
	f := repository.AccountUserFilter{
		SearchTerm: strings.TrimSpace(c.Query("search_term")),
		Sort:       c.Query("sort"),
		Order:      c.Query("order"),
	}
	if f.SearchTerm != "" && len([]rune(f.SearchTerm)) < 2 {
		return responses.BadRequest(c, "search_term must be at least 2 characters")
	}
	params := middleware.GetPagination(c)
	page, err := h.svc.ListAccountUsers(c.Context(), f, params, accountID)
	if err != nil {
		return responses.InternalError(c, "Could not fetch users")
	}
	responses.SetPaginationHeaders(c, page.TotalCount, page.Page, page.PerPage)
	out := make([]fiber.Map, len(page.Items))
	for i := range page.Items {
		out[i] = canvasUserJSON(&page.Items[i], inc, true)
	}
	return c.JSON(out)
}

// ListStudentSubmissions is Canvas's multi-student submissions feed.
// GET /api/v1/courses/:course_id/students/submissions (enrolled)
//
// student_ids[] defaults to the caller. "all" (staff only) means every
// student; students may name only themselves and observers only their
// linked students. Other parameters: assignment_ids[], workflow_state,
// submitted_since, graded_since, order (id|graded_at), order_direction,
// grouped, include[] (assignment, user). Grades are withheld from
// non-staff until posted, as everywhere else.
func (h *CanvasAPIHandler) ListStudentSubmissions(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}
	callerID, _ := c.Locals("user_id").(uint)
	staff := callerIsCourseStaff(c)
	inc := includeSet(c)

	f := repository.StudentSubmissionFilter{IncludeUnpublished: staff}
	rawStudents := queryList(c, "student_ids")
	all := false
	for _, v := range rawStudents {
		if v == "all" {
			all = true
		}
	}
	switch {
	case all:
		if !staff {
			return responses.Forbidden(c, "Only course staff may list every student's submissions")
		}
	case len(rawStudents) == 0:
		f.StudentIDs = []uint{callerID}
	default:
		ids, bad, ok := queryUintList(c, "student_ids")
		if !ok {
			return responses.BadRequest(c, "Invalid student id "+strconv.Quote(bad))
		}
		if !staff {
			for _, id := range ids {
				if id == callerID {
					continue
				}
				linked := false
				if h.observers != nil {
					linked, _ = h.observers.IsObserverOf(c.Context(), callerID, id)
				}
				if !linked {
					return responses.Forbidden(c, "Not authorized to view this student's submissions")
				}
			}
		}
		f.StudentIDs = ids
	}

	aIDs, bad, ok := queryUintList(c, "assignment_ids")
	if !ok {
		return responses.BadRequest(c, "Invalid assignment id "+strconv.Quote(bad))
	}
	f.AssignmentIDs = aIDs

	// Grade-state filters would reveal unposted grades, so they are
	// staff-only.
	if staff {
		if ws := c.Query("workflow_state"); ws != "" {
			switch models.SubmissionWorkflow(ws) {
			case models.SubmissionSubmitted, models.SubmissionUnsubmitted, models.SubmissionGraded, models.SubmissionPendingReview:
				f.WorkflowState = ws
			default:
				return responses.BadRequest(c, "Invalid workflow_state")
			}
		}
		gs, ok := queryTime(c, "graded_since")
		if !ok {
			return responses.BadRequest(c, "Invalid graded_since")
		}
		f.GradedSince = gs
		if c.Query("order") == "graded_at" {
			f.Order = "graded_at"
		}
	}
	ss, ok := queryTime(c, "submitted_since")
	if !ok {
		return responses.BadRequest(c, "Invalid submitted_since")
	}
	f.SubmittedSince = ss
	f.OrderDirection = c.Query("order_direction")

	params := middleware.GetPagination(c)
	page, err := h.svc.ListStudentSubmissions(c.Context(), uint(courseID), f, params, callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not fetch submissions")
	}
	responses.SetPaginationHeaders(c, page.Total, params.Page, params.PerPage)

	render := func(s *models.Submission) fiber.Map {
		var m fiber.Map
		if staff {
			m = submissionToJSON(s)
		} else {
			m = studentSubmissionToJSON(s)
		}
		if inc["assignment"] {
			if a, ok := page.Assignments[s.AssignmentID]; ok {
				m["assignment"] = fiber.Map{
					"id":              a.ID,
					"course_id":       a.CourseID,
					"name":            a.Name,
					"points_possible": a.PointsPossible,
					"due_at":          a.DueAt,
					"grading_type":    a.GradingType,
					"published":       a.IsPublished(),
				}
			}
		}
		if inc["user"] {
			if u, ok := page.Users[s.UserID]; ok {
				m["user"] = canvasUserJSON(&u, map[string]bool{"avatar_url": true}, staff)
			}
		}
		return m
	}

	if g := queryBool(c, "grouped"); g != nil && *g {
		order := []uint{}
		groups := map[uint][]fiber.Map{}
		for i := range page.Submissions {
			s := &page.Submissions[i]
			if _, seen := groups[s.UserID]; !seen {
				order = append(order, s.UserID)
			}
			groups[s.UserID] = append(groups[s.UserID], render(s))
		}
		out := make([]fiber.Map, 0, len(order))
		for _, uid := range order {
			row := fiber.Map{"user_id": uid, "submissions": groups[uid]}
			if staff {
				if u, ok := page.Users[uid]; ok {
					row["sis_user_id"] = u.SISUserID
				}
			}
			out = append(out, row)
		}
		return c.JSON(out)
	}

	out := make([]fiber.Map, len(page.Submissions))
	for i := range page.Submissions {
		out[i] = render(&page.Submissions[i])
	}
	return c.JSON(out)
}
