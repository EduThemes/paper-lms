package handlers

import (
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"

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
// quiz's questions / question groups. Case-insensitive, as Fiber's
// router is: /API/V1/Courses/1/Assignments/2 reaches the same handler.
var reBlueprintGuardedPath = regexp.MustCompile(
	`(?i)^/api/v1/courses/(\d+)/(assignments|quizzes|pages|discussion_topics)/([^/]+)(?:/(questions|groups)(?:/[^/]+)?)?/?$`)

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
		section, sub := strings.ToLower(m[2]), strings.ToLower(m[4])
		// POST only creates sub-resources (a new quiz question).
		if method == fiber.MethodPost && sub == "" {
			return c.Next()
		}
		if sub != "" && section != "quizzes" {
			return c.Next()
		}
		courseID64, err := strconv.ParseUint(m[1], 10, 64)
		if err != nil {
			return c.Next()
		}
		courseID := uint(courseID64)
		contentType := blueprintGuardTypes[section]

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
			edit.Changes, edit.Opaque = guardBodyChanges(c, contentType)
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
var reBlueprintBulkDates = regexp.MustCompile(`(?i)^/api/v1/courses/(\d+)/assignments/bulk_update_dates/?$`)

// guardBulkDates applies the date locks to every assignment a bulk
// date update touches.
func guardBulkDates(c *fiber.Ctx, d BlueprintLockGuardDeps, courseSeg string) error {
	courseID64, err := strconv.ParseUint(courseSeg, 10, 64)
	if err != nil {
		return c.Next()
	}
	courseID := uint(courseID64)
	refuse := func(locked *service.BlueprintLockedError) error {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"errors":            []fiber.Map{{"message": locked.Error()}},
			"locked_attributes": locked.Attributes,
			"blueprint_locked":  true,
		})
	}

	// The handler binds with BodyParser: JSON with case-insensitive keys,
	// but also form and XML bodies. Anything but a JSON body the guard can
	// read is refused when any assignment copy here has a date lock.
	var top map[string]json.RawMessage
	if len(c.Body()) == 0 {
		return c.Next()
	}
	if !strings.HasSuffix(guardContentType(c), "json") || json.Unmarshal(c.Body(), &top) != nil {
		if locked := bulkDateLockedCopies(c, d, courseID); locked != nil && guardCallerIsCourseStaff(c, d, courseID) {
			return refuse(locked)
		}
		return c.Next()
	}
	var updates []map[string]json.RawMessage
	for k, raw := range top {
		if !strings.EqualFold(k, "updates") {
			continue
		}
		var list []map[string]json.RawMessage
		if json.Unmarshal(raw, &list) == nil {
			updates = append(updates, list...)
		}
	}
	dateFields := []string{"due_at", "unlock_at", "lock_at"}
	for _, u := range updates {
		changes, _ := canonicalLockedChanges(dateFields, u)
		for k, raw := range u {
			var id uint
			if !strings.EqualFold(k, "assignment_id") || json.Unmarshal(raw, &id) != nil || id == 0 {
				continue
			}
			err := d.Blueprints.CheckChildEdit(c.Context(), courseID, service.ChildEdit{ContentType: "Assignment", ChildID: id, Changes: changes})
			var locked *service.BlueprintLockedError
			if errors.As(err, &locked) && guardCallerIsCourseStaff(c, d, courseID) {
				return refuse(locked)
			}
		}
	}
	return c.Next()
}

// bulkDateLockedCopies returns a lock error when the course holds an
// assignment copy whose due or availability dates are locked.
func bulkDateLockedCopies(c *fiber.Ctx, d BlueprintLockGuardDeps, courseID uint) *service.BlueprintLockedError {
	items, err := d.Blueprints.ListChildLockedContent(c.Context(), courseID)
	if err != nil {
		return nil
	}
	cats := map[string]bool{}
	for _, it := range items {
		if it.ContentType != "Assignment" {
			continue
		}
		if it.Restrictions.DueDates {
			cats["due_dates"] = true
		}
		if it.Restrictions.AvailabilityDates {
			cats["availability_dates"] = true
		}
	}
	if len(cats) == 0 {
		return nil
	}
	out := &service.BlueprintLockedError{}
	for _, cat := range []string{"availability_dates", "due_dates"} {
		if cats[cat] {
			out.Attributes = append(out.Attributes, cat)
		}
	}
	return out
}

// guardContentType resolves the request content type the way
// BodyParser does: lower-cased, vendor suffix folded ("+json"), without
// parameters.
func guardContentType(c *fiber.Ctx) string {
	ct := utils.ParseVendorSpecificContentType(strings.ToLower(string(c.Request().Header.ContentType())))
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.TrimSpace(ct)
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

// guardBodyChanges decodes a write body into its attribute map,
// unwrapping the handler's envelope ({"assignment": {...}}). JSON is
// the common case, but the write handlers' BodyParser binds
// form-encoded and multipart bodies too (assignment[name]=...,
// assignment.name=...), so those are decoded as well — otherwise a form
// post would slip past the lock check. The content type is resolved the
// way BodyParser resolves it (vendor suffixes, parameters). A body the
// guard cannot read but the handler might bind (XML, malformed JSON) is
// reported as opaque: it is refused for any locked copy.
func guardBodyChanges(c *fiber.Ctx, contentType string) (changes map[string]json.RawMessage, opaque bool) {
	body := c.Body()
	if len(body) == 0 {
		return nil, false
	}
	envelope := blueprintGuardEnvelopes[contentType]
	ct := guardContentType(c)
	switch {
	case strings.HasSuffix(ct, "json"):
	case ct == fiber.MIMEApplicationForm:
		return guardFormChanges(c, contentType, false)
	case ct == fiber.MIMEMultipartForm:
		return guardFormChanges(c, contentType, true)
	default:
		return nil, true
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return nil, true
	}
	// encoding/json matches keys case-insensitively, so {"Assignment":
	// {"NAME": ..}} binds like {"assignment": {"name": ..}}. Read every
	// envelope spelling and the top level, and fold the keys.
	objs := []map[string]json.RawMessage{top}
	for k, raw := range top {
		if !strings.EqualFold(k, envelope) {
			continue
		}
		var inner map[string]json.RawMessage
		if err := json.Unmarshal(raw, &inner); err == nil {
			objs = append(objs, inner)
		}
	}
	return canonicalLockedChanges(service.BlueprintLockedFields(contentType), objs...)
}

// canonicalLockedChanges maps the keys of decoded JSON objects onto the
// lockable attribute names (case and underscores ignored). An attribute
// given more than once is compared as a list of its values, so a
// differing duplicate is not hidden behind an unchanged one.
func canonicalLockedChanges(known []string, objs ...map[string]json.RawMessage) (map[string]json.RawMessage, bool) {
	byLoose := make(map[string]string, len(known))
	for _, k := range known {
		byLoose[looseFieldName(k)] = k
	}
	values := map[string][]json.RawMessage{}
	for _, obj := range objs {
		for k, raw := range obj {
			if name, ok := byLoose[looseFieldName(k)]; ok {
				values[name] = append(values[name], raw)
			}
		}
	}
	out := make(map[string]json.RawMessage, len(values))
	for name, vs := range values {
		if len(vs) == 1 {
			out[name] = vs[0]
			continue
		}
		b, err := json.Marshal(vs)
		if err != nil {
			return nil, true
		}
		out[name] = b
	}
	return out, false
}

// guardFormChanges maps form fields onto the lockable attribute names.
// Fiber's form binder matches a field case-insensitively, through any
// envelope prefix ("assignment[name]", "Assignment.PointsPossible",
// "name"), so the leaf key is compared loosely: case and underscores
// ignored. A field sent more than once is compared as a list, so any
// differing value counts as a change.
func guardFormChanges(c *fiber.Ctx, contentType string, multipart bool) (map[string]json.RawMessage, bool) {
	known := service.BlueprintLockedFields(contentType)
	if len(known) == 0 {
		return nil, false
	}
	byLoose := make(map[string]string, len(known))
	for _, k := range known {
		byLoose[looseFieldName(k)] = k
	}
	values := map[string][]string{}
	add := func(key, value string) {
		leaf := strings.TrimRight(key, "]")
		if i := strings.LastIndexAny(leaf, "[."); i >= 0 {
			leaf = leaf[i+1:]
		}
		if name, ok := byLoose[looseFieldName(leaf)]; ok {
			values[name] = append(values[name], value)
		}
	}
	if multipart {
		form, err := c.MultipartForm()
		if err != nil || form == nil {
			return nil, true
		}
		for k, vs := range form.Value {
			for _, v := range vs {
				add(k, v)
			}
		}
	} else {
		c.Request().PostArgs().VisitAll(func(k, v []byte) {
			add(string(k), string(v))
		})
	}
	out := make(map[string]json.RawMessage, len(values))
	for name, vs := range values {
		var b []byte
		var err error
		if len(vs) == 1 {
			b, err = json.Marshal(vs[0])
		} else {
			b, err = json.Marshal(vs)
		}
		if err != nil {
			return nil, true
		}
		out[name] = b
	}
	return out, false
}

func looseFieldName(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), "_", "")
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
