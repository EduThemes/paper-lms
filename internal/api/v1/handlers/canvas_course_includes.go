package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

// SetCanvasAPI enables the Canvas course fields on GET /courses and
// GET /courses/:id (G137): the caller's "enrollments" array, sis_course_id
// for staff, and include[]=term / include[]=teachers.
func (h *CourseHandler) SetCanvasAPI(svc *service.CanvasAPIService) *CourseHandler {
	h.canvasAPI = svc
	return h
}

// canvasShortEnrollmentType is Canvas's course-JSON enrollment type
// ("student", "teacher", "ta", "observer", "designer").
func canvasShortEnrollmentType(t string) string {
	switch t {
	case "TaEnrollment":
		return "ta"
	default:
		return strings.ToLower(strings.TrimSuffix(t, "Enrollment"))
	}
}

func isStaffEnrollmentType(t string) bool {
	return t == "TeacherEnrollment" || t == "TaEnrollment" || t == "DesignerEnrollment"
}

// applyCourseIncludes adds the Canvas course fields to out[i] (the JSON of
// courses[i]). Failures leave the base JSON untouched.
func (h *CourseHandler) applyCourseIncludes(c *fiber.Ctx, courses []models.Course, out []fiber.Map) {
	if h.canvasAPI == nil || len(courses) == 0 || len(courses) != len(out) {
		return
	}
	userID, _ := c.Locals("user_id").(uint)
	inc := includeSet(c)
	rows, err := h.canvasAPI.DecorateCourses(c.Context(), courses, service.AccountCourseIncludes{
		Term:           inc["term"],
		Teachers:       inc["teachers"],
		EnrollmentsFor: userID,
	}, callerAccountID(c))
	if err != nil || len(rows) != len(out) {
		return
	}
	isAdmin, _ := c.Locals("is_admin").(bool)
	for i := range rows {
		row := &rows[i]
		staff := isAdmin
		enrollments := make([]fiber.Map, 0, len(row.Enrollments))
		for _, e := range row.Enrollments {
			if isStaffEnrollmentType(e.Type) && e.WorkflowState == models.EnrollmentActive {
				staff = true
			}
			enrollments = append(enrollments, fiber.Map{
				"type":               canvasShortEnrollmentType(e.Type),
				"role":               e.Role,
				"user_id":            e.UserID,
				"enrollment_state":   e.WorkflowState,
				"course_section_id":  e.CourseSectionID,
				"associated_user_id": e.AssociatedUserID,
			})
		}
		out[i]["enrollments"] = enrollments
		if staff {
			out[i]["sis_course_id"] = row.Course.SISCourseID
		}
		ext := canvasCourseJSON(row, inc)
		for _, k := range []string{"term", "teachers"} {
			if v, ok := ext[k]; ok {
				out[i][k] = v
			}
		}
	}
}
