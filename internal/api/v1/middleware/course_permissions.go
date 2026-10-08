package middleware

import (
	"context"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/gofiber/fiber/v2"
)

// CoursePermissionResolver resolves the permission set an enrollment
// carries (G102). *service.CustomRoleService implements it.
type CoursePermissionResolver interface {
	EnrollmentPermissions(ctx context.Context, enrollment *models.Enrollment, accountID uint) (map[string]bool, error)
}

// SetPermissionResolver enables custom-role enforcement on the course
// gates. Without a resolver the gates check enrollment types only.
func (pm *PermissionMiddleware) SetPermissionResolver(r CoursePermissionResolver) *PermissionMiddleware {
	pm.permResolver = r
	return pm
}

// staffEnrollment reports whether an enrollment type is a course-staff
// role. Only staff-based custom roles may be granted access to a staff
// route their base type would not pass: the handlers behind those routes
// assume a staff caller, so a student- or observer-based role never
// crosses into them, whatever its permission set says.
func staffEnrollment(enrollmentType string) bool {
	switch enrollmentType {
	case "TeacherEnrollment", "TaEnrollment", "DesignerEnrollment":
		return true
	}
	return false
}

// coursePermissionRule maps a course route onto the permission that
// governs it. Rules are checked in order; the first match wins.
type coursePermissionRule struct {
	// contains is matched against the route pattern after the course id
	// (e.g. "/assignments/:id/submissions/:user_id").
	contains string
	// readPerm governs GET/HEAD; "" means reads stay type-gated.
	readPerm string
	// writePerm governs POST/PUT/PATCH/DELETE; "" means type-gated.
	writePerm string
}

// coursePermissionRules is the permission matrix's map onto the course
// routes (Canvas: the RoleOverride permission each controller checks).
// Only staff-gated routes are consulted (see courseGate), so a learner's
// read paths are never affected. Grading and submission rules come first
// because they nest under other segments (/assignments/:id/submissions).
var coursePermissionRules = []coursePermissionRule{
	{contains: "/moderat", readPerm: models.PermModerateGrades, writePerm: models.PermModerateGrades},
	{contains: "/provisional_grades", readPerm: models.PermModerateGrades, writePerm: models.PermModerateGrades},
	{contains: "/comments", writePerm: models.PermCommentOnSubmissions},
	{contains: "/speed_grader", readPerm: models.PermGradeSubmissions, writePerm: models.PermGradeSubmissions},
	{contains: "/submissions", readPerm: models.PermViewSubmissionDetails, writePerm: models.PermGradeSubmissions},
	{contains: "/grade_change_log", readPerm: models.PermViewAllGrades, writePerm: models.PermManageGrades},
	{contains: "/learning_mastery_gradebook", readPerm: models.PermViewAllGrades, writePerm: models.PermManageGrades},
	{contains: "/gradebook", readPerm: models.PermViewAllGrades, writePerm: models.PermManageGrades},
	{contains: "/custom_gradebook_columns", readPerm: models.PermViewAllGrades, writePerm: models.PermManageGrades},
	{contains: "/outcome_results", readPerm: models.PermViewAllGrades, writePerm: models.PermManageGrades},
	{contains: "/late_policy", writePerm: models.PermManageGrades},
	{contains: "/grading_standards", writePerm: models.PermManageGrades},
	{contains: "/grading_periods", writePerm: models.PermManageGrades},
	{contains: "/analytics", readPerm: models.PermViewAnalytics, writePerm: models.PermViewAnalytics},
	{contains: "/audit_log", readPerm: models.PermViewAuditLog, writePerm: models.PermViewAuditLog},
	{contains: "/enrollable_users", readPerm: models.PermManageEnrollments, writePerm: models.PermManageEnrollments},
	{contains: "/enrollments", writePerm: models.PermManageEnrollments},
	{contains: "/sections", writePerm: models.PermManageSections},
	{contains: "/mastery_paths", writePerm: models.PermManageAssignments},
	{contains: "/assignment_groups", writePerm: models.PermManageAssignments},
	{contains: "/assignments", writePerm: models.PermManageAssignments},
	{contains: "/quiz", writePerm: models.PermManageQuizzes}, // quizzes, quiz_item_banks, quiz_stimuli
	{contains: "/question_banks", writePerm: models.PermManageQuizzes},
	{contains: "/qti_import", writePerm: models.PermManageQuizzes},
	{contains: "/modules", writePerm: models.PermManageModules},
	{contains: "/course_pacing", writePerm: models.PermManagePacing},
	{contains: "/blueprint", writePerm: models.PermManageBlueprint},
	{contains: "/announcements", writePerm: models.PermManageAnnouncements},
	{contains: "/discussion_topics", writePerm: models.PermManageDiscussions},
	{contains: "/pages", writePerm: models.PermManagePages},
	{contains: "/files", writePerm: models.PermManageFiles},
	{contains: "/folders", writePerm: models.PermManageFiles},
	{contains: "/rubric", writePerm: models.PermManageRubrics},
	{contains: "/outcome", writePerm: models.PermManageOutcomes}, // outcomes, outcome_groups, outcome_alignments, outcome_proficiency
	{contains: "/group_categories", writePerm: models.PermManageGroups},
	{contains: "/groups", writePerm: models.PermManageGroups},
	{contains: "/conferences", writePerm: models.PermManageConferences},
	{contains: "/collaborations", writePerm: models.PermManageCollaborations},
	{contains: "/calendar_events", writePerm: models.PermManageCalendar},
	{contains: "/home", writePerm: models.PermManageContent},
	{contains: "/external_tools", writePerm: models.PermManageContent},
	{contains: "/features", writePerm: models.PermManageContent},
	{contains: "/content_migrations", writePerm: models.PermManageContent},
	{contains: "/content_imports", writePerm: models.PermManageContent},
	{contains: "/date_shift", writePerm: models.PermManageContent},
	{contains: "/commons", writePerm: models.PermManageContent},
	{contains: "/gamification", writePerm: models.PermManageContent},
}

// RequiredCoursePermission returns the permission governing a course
// route, or ok=false when the route has no permission mapping (and so
// stays gated by enrollment type alone). routePath is the matched route
// pattern (c.Route().Path).
func RequiredCoursePermission(method, routePath string) (string, bool) {
	rest, ok := courseRouteRemainder(routePath)
	if !ok {
		return "", false
	}
	read := method == fiber.MethodGet || method == fiber.MethodHead
	if rest == "" {
		// The course itself: PUT /courses/:id edits course settings.
		if !read && method != fiber.MethodDelete {
			return models.PermManageContent, true
		}
		return "", false
	}
	for _, rule := range coursePermissionRules {
		if !strings.Contains(rest, rule.contains) {
			continue
		}
		perm := rule.writePerm
		if read {
			perm = rule.readPerm
		}
		return perm, perm != ""
	}
	return "", false
}

// courseRouteRemainder strips "/api/v1/courses/:course_id" (or ":id")
// from a route pattern and returns what follows.
func courseRouteRemainder(routePath string) (string, bool) {
	i := strings.Index(routePath, "/courses/:")
	if i < 0 {
		return "", false
	}
	rest := routePath[i+len("/courses/:"):]
	j := strings.Index(rest, "/")
	if j < 0 {
		return "", true
	}
	return rest[j:], true
}

// customRoleDecision applies a custom role's permission set to a
// staff-gated course route. decided=false means the route has no
// permission mapping (or the enrollment carries no custom role), and the
// caller falls back to the enrollment-type check.
func (pm *PermissionMiddleware) customRoleDecision(c *fiber.Ctx, enrollment *models.Enrollment, course *models.Course, roleSet map[string]bool) (allowed bool, decided bool) {
	if pm.permResolver == nil || roleSet == nil || enrollment.RoleID == nil || course == nil {
		return false, false
	}
	route := c.Route()
	if route == nil {
		return false, false
	}
	perm, mapped := RequiredCoursePermission(c.Method(), route.Path)
	if !mapped {
		return false, false
	}
	perms, err := pm.permResolver.EnrollmentPermissions(c.Context(), enrollment, course.AccountID)
	if err != nil {
		// Fail closed: the role exists but its permissions can't be read.
		return false, true
	}
	if !perms[perm] {
		return false, true
	}
	// A granted permission lets a staff-based role through a route its
	// base type would not pass (e.g. a TA-based role given
	// manage_assignments), but never a learner-based role.
	if !roleSet[enrollment.Type] && !staffEnrollment(enrollment.Type) {
		return false, true
	}
	return true, true
}
