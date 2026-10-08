package handlers

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

// Enrollment lifecycle endpoints (admin-enrollments lane):
//
//	PUT    /courses/:course_id/enrollments/:id              edit role, section, dates, section limit (G105/G107)
//	DELETE /courses/:course_id/enrollments/:id?task=        conclude (default) | deactivate | inactivate | delete (G105)
//	PUT    /courses/:course_id/enrollments/:id/reactivate   inactive|completed -> active (G105)
//	POST   /courses/:course_id/enrollments/:id/accept       invitee accepts (G122)
//	POST   /courses/:course_id/enrollments/:id/reject       invitee declines (G122)
//	GET    /users/self/course_invitations                   pending invitations (G122)
//
// Who may manage an enrollment mirrors who may add it
// (canAddEnrollmentType): teachers and admins any role, TAs and designers
// only students and observers. Nobody but an admin changes their own
// enrollment, and a section-limited teacher only manages people in their
// sections.

// SetLifecycle wires the lifecycle service. Returns the handler for
// chaining.
func (h *EnrollmentHandler) SetLifecycle(l *service.EnrollmentLifecycleService) *EnrollmentHandler {
	h.lifecycle = l
	return h
}

var rosterStates = map[string]bool{
	string(models.EnrollmentActive):    true,
	string(models.EnrollmentInvited):   true,
	string(models.EnrollmentCompleted): true,
	string(models.EnrollmentInactive):  true,
	string(models.EnrollmentRejected):  true,
}

// rosterFilter builds the roster filter from ?state[]= / ?state= and the
// caller's access. Students and observers only ever see active and
// invited people (Canvas); staff see every live state.
func rosterFilter(c *fiber.Ctx) repository.EnrollmentListFilter {
	var f repository.EnrollmentListFilter
	var requested []string
	for _, raw := range c.Context().QueryArgs().PeekMulti("state[]") {
		requested = append(requested, string(raw))
	}
	if s := c.Query("state"); s != "" {
		requested = append(requested, s)
	}
	staff := callerIsCourseStaff(c)
	for _, s := range requested {
		if !rosterStates[s] {
			continue
		}
		if !staff && s != string(models.EnrollmentActive) && s != string(models.EnrollmentInvited) {
			continue
		}
		f.States = append(f.States, s)
	}
	if !staff && len(f.States) == 0 {
		f.States = []string{string(models.EnrollmentActive), string(models.EnrollmentInvited)}
	}
	if ids, limited := middleware.VisibleSectionIDs(c); limited {
		f.SectionIDs = ids
		f.AlwaysIncludeTypes = []string{models.TeacherEnrollmentType, models.TaEnrollmentType, models.DesignerEnrollmentType}
	}
	return f
}

func parseEnrollmentPath(c *fiber.Ctx) (courseID, id uint, wrote bool, err error) {
	cid, err := c.ParamsInt("course_id")
	if err != nil || cid <= 0 {
		return 0, 0, true, responses.BadRequest(c, "Invalid course ID")
	}
	eid, err := c.ParamsInt("id")
	if err != nil || eid <= 0 {
		return 0, 0, true, responses.BadRequest(c, "Invalid enrollment ID")
	}
	return uint(cid), uint(eid), false, nil
}

// loadManagedEnrollment loads an enrollment of the URL's course that the
// caller may manage. It writes the error response itself, so the caller
// must stop when wrote is true.
func (h *EnrollmentHandler) loadManagedEnrollment(c *fiber.Ctx) (e *models.Enrollment, wrote bool, err error) {
	if h.lifecycle == nil {
		return nil, true, responses.InternalError(c, "enrollment lifecycle not configured")
	}
	courseID, id, wrote, err := parseEnrollmentPath(c)
	if wrote {
		return nil, true, err
	}
	// Course parent-tie + tenant scope: the enrollment must be in this
	// course, and the course in the caller's tenant (404 otherwise).
	e, findErr := h.lifecycle.FindInCourse(c.Context(), courseID, id, enrollmentCourseScope(c))
	if findErr != nil {
		return nil, true, responses.NotFound(c, "enrollment")
	}
	if !sectionVisibleUser(c, e.UserID) {
		return nil, true, responses.NotFound(c, "enrollment")
	}
	if !canAddEnrollmentType(c, e.Type) {
		return nil, true, responses.Forbidden(c, "You are not allowed to manage this enrollment")
	}
	callerID, _ := c.Locals("user_id").(uint)
	isAdmin, _ := c.Locals("is_admin").(bool)
	if e.UserID == callerID && !isAdmin {
		return nil, true, responses.Forbidden(c, "You can't change your own enrollment")
	}
	return e, false, nil
}

func lifecycleError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, service.ErrEnrollmentSectionNotFound):
		return responses.NotFound(c, "course section")
	case errors.Is(err, service.ErrEnrollmentDuplicate):
		return responses.Error(c, fiber.StatusConflict, err.Error())
	case errors.Is(err, service.ErrEnrollmentTransition),
		errors.Is(err, service.ErrEnrollmentInvalidTask),
		errors.Is(err, service.ErrEnrollmentDates):
		return responses.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrEnrollmentNotFound), errors.Is(err, service.ErrEnrollmentNotInvitee):
		return responses.NotFound(c, "enrollment")
	}
	if err.Error() == "invalid enrollment type" {
		return responses.BadRequest(c, err.Error())
	}
	return responses.InternalError(c, "Could not update enrollment")
}

// parseOptionalTime decodes a JSON time that may be null or "" (clear).
func parseOptionalTime(raw json.RawMessage) (*time.Time, error) {
	var s *string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	if s == nil || *s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// UpdateEnrollment handles PUT /courses/:course_id/enrollments/:id.
// Body: {"enrollment": {"type", "course_section_id", "start_at",
// "end_at", "limit_privileges_to_course_section"}} — every key optional;
// null clears a section or date.
func (h *EnrollmentHandler) UpdateEnrollment(c *fiber.Ctx) error {
	e, wrote, err := h.loadManagedEnrollment(c)
	if wrote {
		return err
	}

	var input struct {
		Enrollment map[string]json.RawMessage `json:"enrollment"`
	}
	if err := json.Unmarshal(c.Body(), &input); err != nil || input.Enrollment == nil {
		return responses.BadRequest(c, "Invalid input")
	}
	var upd service.EnrollmentUpdate
	for key, raw := range input.Enrollment {
		switch key {
		case "type":
			var t string
			if err := json.Unmarshal(raw, &t); err != nil || t == "" {
				return responses.BadRequest(c, "Invalid type")
			}
			if t != e.Type && !canAddEnrollmentType(c, t) {
				return responses.Forbidden(c, "You are not allowed to assign this role")
			}
			upd.Type = &t
		case "course_section_id":
			var sid *uint
			if err := json.Unmarshal(raw, &sid); err != nil {
				return responses.BadRequest(c, "Invalid course_section_id")
			}
			if sid != nil && *sid == 0 {
				sid = nil
			}
			upd.SectionSet, upd.CourseSectionID = true, sid
		case "start_at":
			t, err := parseOptionalTime(raw)
			if err != nil {
				return responses.BadRequest(c, "Invalid start_at")
			}
			upd.StartAtSet, upd.StartAt = true, t
		case "end_at":
			t, err := parseOptionalTime(raw)
			if err != nil {
				return responses.BadRequest(c, "Invalid end_at")
			}
			upd.EndAtSet, upd.EndAt = true, t
		case "limit_privileges_to_course_section":
			var b bool
			if err := json.Unmarshal(raw, &b); err != nil {
				return responses.BadRequest(c, "Invalid limit_privileges_to_course_section")
			}
			upd.LimitPrivileges = &b
		}
	}
	// A section-limited teacher can't move someone out of their sections.
	if ids, limited := middleware.VisibleSectionIDs(c); limited && upd.SectionSet {
		target := uint(0)
		if upd.CourseSectionID != nil {
			target = *upd.CourseSectionID
		}
		allowed := false
		for _, id := range ids {
			if id == target {
				allowed = true
			}
		}
		if !allowed {
			return responses.NotFound(c, "course section")
		}
	}

	if err := h.lifecycle.Update(c.Context(), e, upd); err != nil {
		return lifecycleError(c, err)
	}
	return c.JSON(enrollmentToJSON(e, true))
}

// DeleteEnrollment handles DELETE /courses/:course_id/enrollments/:id
// with ?task=conclude (default), deactivate / inactivate, or delete.
func (h *EnrollmentHandler) DeleteEnrollment(c *fiber.Ctx) error {
	e, wrote, err := h.loadManagedEnrollment(c)
	if wrote {
		return err
	}
	task := c.Query("task", service.EnrollmentTaskConclude)
	if err := h.lifecycle.ApplyTask(c.Context(), e, task); err != nil {
		return lifecycleError(c, err)
	}
	return c.JSON(enrollmentToJSON(e, true))
}

// ReactivateEnrollment handles PUT /courses/:course_id/enrollments/:id/reactivate.
func (h *EnrollmentHandler) ReactivateEnrollment(c *fiber.Ctx) error {
	e, wrote, err := h.loadManagedEnrollment(c)
	if wrote {
		return err
	}
	if err := h.lifecycle.Reactivate(c.Context(), e); err != nil {
		return lifecycleError(c, err)
	}
	return c.JSON(enrollmentToJSON(e, true))
}

// respondToInvitation backs accept / reject. These routes carry no course
// gate (the invitee is not active yet); the enrollment must be the
// caller's own, in the URL's course and the caller's tenant — anything
// else is a 404.
func (h *EnrollmentHandler) respondToInvitation(c *fiber.Ctx, accept bool) error {
	if h.lifecycle == nil {
		return responses.InternalError(c, "enrollment lifecycle not configured")
	}
	courseID, id, wrote, err := parseEnrollmentPath(c)
	if wrote {
		return err
	}
	callerID, uerr := getUserID(c)
	if uerr != nil {
		return uerr
	}
	e, findErr := h.lifecycle.FindInCourse(c.Context(), courseID, id, callerAccountID(c))
	if findErr != nil || e.UserID != callerID {
		return responses.NotFound(c, "enrollment")
	}
	if err := h.lifecycle.RespondToInvitation(c.Context(), e, callerID, accept); err != nil {
		return lifecycleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "enrollment": enrollmentToJSON(e, true)})
}

// AcceptInvitation handles POST /courses/:course_id/enrollments/:id/accept.
func (h *EnrollmentHandler) AcceptInvitation(c *fiber.Ctx) error {
	return h.respondToInvitation(c, true)
}

// RejectInvitation handles POST /courses/:course_id/enrollments/:id/reject.
func (h *EnrollmentHandler) RejectInvitation(c *fiber.Ctx) error {
	return h.respondToInvitation(c, false)
}

// ListMyInvitations handles GET /users/self/course_invitations.
func (h *EnrollmentHandler) ListMyInvitations(c *fiber.Ctx) error {
	if h.lifecycle == nil {
		return c.JSON([]fiber.Map{})
	}
	callerID, err := getUserID(c)
	if err != nil {
		return err
	}
	rows, err := h.lifecycle.ListInvitations(c.Context(), callerID, callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not fetch invitations")
	}
	out := make([]fiber.Map, 0, len(rows))
	for i := range rows {
		e := &rows[i]
		item := fiber.Map{
			"id":                e.ID,
			"course_id":         e.CourseID,
			"course_section_id": e.CourseSectionID,
			"type":              e.Type,
			"role":              e.Role,
			"enrollment_state":  e.WorkflowState,
			"created_at":        e.CreatedAt,
		}
		if e.Course != nil {
			item["course_name"] = e.Course.Name
			item["course_code"] = e.Course.CourseCode
		}
		out = append(out, item)
	}
	return c.JSON(out)
}
