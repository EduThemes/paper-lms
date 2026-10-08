package middleware

import (
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"
)

// Course navigation tab ids whose list endpoints are gated server-side
// when the course hides the tab (G078). They match the ids the course
// settings page writes into courses.navigation_tabs.
const (
	CourseTabPages   = "pages"
	CourseTabFiles   = "files"
	CourseTabQuizzes = "quizzes"
)

// tabDisabledMessage is the Canvas wording for a hidden tab.
const tabDisabledMessage = "That page has been disabled for this course"

// CourseTabHidden reports whether navTabs (the course's navigation_tabs
// JSON: [{"id": "pages", "hidden": true, "position": 3}, ...]) hides
// tabID. An empty or unreadable configuration hides nothing, so a bad
// value can never lock content away.
func CourseTabHidden(navTabs, tabID string) bool {
	if navTabs == "" {
		return false
	}
	var tabs []map[string]any
	if err := json.Unmarshal([]byte(navTabs), &tabs); err != nil {
		return false
	}
	for _, t := range tabs {
		if fmt.Sprint(t["id"]) != tabID {
			continue
		}
		hidden, _ := t["hidden"].(bool)
		return hidden
	}
	return false
}

// courseStaffEnrollment: roles that see hidden tabs (Canvas shows hidden
// tabs to anyone who can manage the course content).
func courseStaffEnrollment(enrollmentType string) bool {
	switch enrollmentType {
	case "TeacherEnrollment", "TaEnrollment", "DesignerEnrollment":
		return true
	}
	return false
}

// RequireCourseTab refuses a course's list endpoint for tabID when the
// course hides that tab and the caller is a learner (student, observer,
// or a Student View session). It mirrors Canvas tab_enabled?: the index
// answers 404 "That page has been disabled for this course", while the
// individual items stay reachable (a page linked from a module still
// opens).
//
// It is registered as its own route ahead of the real one and calls
// c.Next() to continue to it, so it never decides access on its own:
// anything it cannot place (no user, an unknown course, no active
// enrollment, an admin) falls through to the route's real gate.
func (pm *PermissionMiddleware) RequireCourseTab(tabID string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, _ := c.Locals("user_id").(uint)
		if userID == 0 || pm.courseRepo == nil || pm.enrollmentRepo == nil {
			return c.Next()
		}
		courseID, err := pm.extractCourseID(c)
		if err != nil {
			return c.Next()
		}
		if RoleRank(EffectiveRole(c, pm.userRepo)) >= 1 {
			return c.Next() // admins see hidden tabs; the route gate scopes the tenant
		}
		callerAccount, _ := c.Locals("account_id").(uint)
		if callerAccount == 0 {
			return c.Next()
		}
		course, err := pm.courseRepo.FindByID(c.Context(), courseID, callerAccount)
		if err != nil || course == nil || !CourseTabHidden(course.NavigationTabs, tabID) {
			return c.Next()
		}
		enrollment, err := pm.enrollmentRepo.FindByUserAndCourse(c.Context(), userID, courseID, callerAccount)
		if err != nil || enrollment == nil || enrollment.WorkflowState != "active" {
			return c.Next() // not enrolled: the route gate answers
		}
		if courseStaffEnrollment(enrollment.Type) {
			return c.Next()
		}
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": tabDisabledMessage,
			"errors":  []fiber.Map{{"message": tabDisabledMessage}},
		})
	}
}
