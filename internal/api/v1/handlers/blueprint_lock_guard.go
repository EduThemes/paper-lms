package handlers

import (
	"encoding/json"
	"errors"
	"regexp"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

// BlueprintLockGuardDeps are what the blueprint lock guard needs.
type BlueprintLockGuardDeps struct {
	Blueprints  *service.BlueprintService
	Pages       repository.PageRepository
	Courses     repository.CourseRepository
	Enrollments repository.EnrollmentRepository
}

// reBlueprintGuardedPath matches the content write routes the guard
// checks: an assignment, quiz, page or discussion topic itself, and a
// quiz's questions / question groups.
var reBlueprintGuardedPath = regexp.MustCompile(
	`^/api/v1/courses/(\d+)/(assignments|quizzes|pages|discussion_topics)/([^/]+)(?:/(questions|groups)(?:/[^/]+)?)?/?$`)

var blueprintGuardTypes = map[string]string{
	"assignments":       "Assignment",
	"quizzes":           "Quiz",
	"pages":             "WikiPage",
	"discussion_topics": "DiscussionTopic",
}

// blueprintGuardEnvelopes are the body keys the write handlers nest
// their attributes under.
var blueprintGuardEnvelopes = map[string]string{
	"Assignment":      "assignment",
	"Quiz":            "quiz",
	"WikiPage":        "wiki_page",
	"DiscussionTopic": "discussion_topic",
}

// NewBlueprintLockGuard returns middleware that enforces blueprint locks
// in associated courses (G069): a write to a synced copy of locked
// blueprint content that changes a locked attribute — or deletes the
// copy — is refused with 403 before the route handler runs.
//
// It is mounted once on the protected group and only inspects the
// content write routes it recognizes. It never grants anything: when the
// caller is not staff of the course (or anything is unclear) it passes
// the request on, and the route's own permission gate answers — so the
// lock state of a course is never revealed to an outsider.
func NewBlueprintLockGuard(d BlueprintLockGuardDeps) fiber.Handler {
	return func(c *fiber.Ctx) error {
		method := c.Method()
		if method != fiber.MethodPut && method != fiber.MethodPatch && method != fiber.MethodDelete && method != fiber.MethodPost {
			return c.Next()
		}
		if d.Blueprints == nil {
			return c.Next()
		}
		if bm := reBlueprintBulkDates.FindStringSubmatch(c.Path()); bm != nil && method == fiber.MethodPost {
			return guardBulkDates(c, d, bm[1])
		}
		m := reBlueprintGuardedPath.FindStringSubmatch(c.Path())
		if m == nil {
			return c.Next()
		}
		sub := m[4]
		// POST only creates sub-resources (a new quiz question).
		if method == fiber.MethodPost && sub == "" {
			return c.Next()
		}
		if sub != "" && m[2] != "quizzes" {
			return c.Next()
		}
		courseID64, err := strconv.ParseUint(m[1], 10, 64)
		if err != nil {
			return c.Next()
		}
		courseID := uint(courseID64)
		contentType := blueprintGuardTypes[m[2]]

		childID, ok := resolveGuardedID(c, d, courseID, contentType, m[3])
		if !ok {
			return c.Next()
		}

		edit := service.ChildEdit{ContentType: contentType, ChildID: childID}
		switch {
		case sub != "":
			edit.ContentWrite = true
		case method == fiber.MethodDelete:
			edit.Delete = true
		default:
			edit.Changes = guardBodyChanges(c.Body(), blueprintGuardEnvelopes[contentType])
		}

		checkErr := d.Blueprints.CheckChildEdit(c.Context(), courseID, edit)
		var locked *service.BlueprintLockedError
		if !errors.As(checkErr, &locked) {
			// Not locked, or the check itself failed: the handler decides.
			return c.Next()
		}
		if !guardCallerIsCourseStaff(c, d, courseID) {
			return c.Next()
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"errors":            []fiber.Map{{"message": locked.Error()}},
			"locked_attributes": locked.Attributes,
			"blueprint_locked":  true,
		})
	}
}

// reBlueprintBulkDates matches the bulk assignment-date editor, which
// writes due/availability dates of many assignments at once.
var reBlueprintBulkDates = regexp.MustCompile(`^/api/v1/courses/(\d+)/assignments/bulk_update_dates/?$`)

// guardBulkDates applies the date locks to every assignment a bulk
// date update touches.
func guardBulkDates(c *fiber.Ctx, d BlueprintLockGuardDeps, courseSeg string) error {
	courseID64, err := strconv.ParseUint(courseSeg, 10, 64)
	if err != nil {
		return c.Next()
	}
	var body struct {
		Updates []map[string]json.RawMessage `json:"updates"`
	}
	if json.Unmarshal(c.Body(), &body) != nil {
		return c.Next()
	}
	courseID := uint(courseID64)
	for _, u := range body.Updates {
		var id uint
		if raw, ok := u["assignment_id"]; !ok || json.Unmarshal(raw, &id) != nil || id == 0 {
			continue
		}
		changes := map[string]json.RawMessage{}
		for _, k := range []string{"due_at", "unlock_at", "lock_at"} {
			if raw, ok := u[k]; ok {
				changes[k] = raw
			}
		}
		err := d.Blueprints.CheckChildEdit(c.Context(), courseID, service.ChildEdit{ContentType: "Assignment", ChildID: id, Changes: changes})
		var locked *service.BlueprintLockedError
		if errors.As(err, &locked) && guardCallerIsCourseStaff(c, d, courseID) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"errors":            []fiber.Map{{"message": locked.Error()}},
				"locked_attributes": locked.Attributes,
				"blueprint_locked":  true,
			})
		}
	}
	return c.Next()
}

// resolveGuardedID turns the route's id segment into the row id. Pages
// are addressed by id or slug.
func resolveGuardedID(c *fiber.Ctx, d BlueprintLockGuardDeps, courseID uint, contentType, seg string) (uint, bool) {
	if id, err := strconv.ParseUint(seg, 10, 64); err == nil && id > 0 {
		return uint(id), true
	}
	if contentType != "WikiPage" || d.Pages == nil {
		return 0, false
	}
	p, err := d.Pages.FindByCourseAndURL(c.Context(), courseID, seg)
	if err != nil || p == nil {
		return 0, false
	}
	return p.ID, true
}

// guardBodyChanges decodes a JSON write body into its attribute map,
// unwrapping the handler's envelope ({"assignment": {...}}).
func guardBodyChanges(body []byte, envelope string) map[string]json.RawMessage {
	if len(body) == 0 {
		return nil
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return nil
	}
	if raw, ok := top[envelope]; ok {
		var inner map[string]json.RawMessage
		if err := json.Unmarshal(raw, &inner); err == nil {
			return inner
		}
	}
	return top
}

// guardCallerIsCourseStaff reports whether the caller is an admin of the
// course's tenant or holds an active enrollment in it — i.e. someone the
// route itself would not simply turn away.
func guardCallerIsCourseStaff(c *fiber.Ctx, d BlueprintLockGuardDeps, courseID uint) bool {
	account, _ := c.Locals("account_id").(uint)
	if d.Courses == nil || account == 0 {
		return false
	}
	course, err := d.Courses.FindByID(c.Context(), courseID, account)
	if err != nil || course == nil {
		return false
	}
	if isAdmin, _ := c.Locals("is_admin").(bool); isAdmin {
		return true
	}
	if d.Enrollments == nil {
		return false
	}
	userID, _ := c.Locals("user_id").(uint)
	e, err := d.Enrollments.FindByUserAndCourse(c.Context(), userID, courseID, account)
	return err == nil && e != nil && e.WorkflowState == "active" && e.Type != "StudentEnrollment" && e.Type != "ObserverEnrollment"
}
