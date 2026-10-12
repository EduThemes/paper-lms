package handlers

import (
	"encoding/json"
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

// Section management (G109) and cross-listing (G108), Canvas API shape:
//
//	PUT    /sections/:id                              rename, dates, restrict to section dates
//	DELETE /sections/:id                              delete an empty section
//	POST   /sections/:id/crosslist/:new_course_id     cross-list into another course
//	DELETE /sections/:id/crosslist                    move back to the original course
//
// These routes carry no :course_id, so the handler authorizes against
// the section's course: teacher or admin (Canvas does not let TAs manage
// sections). Cross-listing needs teacher/admin rights in BOTH courses.
// A section outside the caller's tenant is a 404.

// SetSectionService wires section management. Returns the handler for
// chaining.
func (h *SectionHandler) SetSectionService(s *service.SectionService) *SectionHandler {
	h.sections = s
	return h
}

// loadManagedSection loads a section the caller may manage (teacher or
// admin of its course). It writes the error response itself; the caller
// stops when wrote is true.
func (h *SectionHandler) loadManagedSection(c *fiber.Ctx) (sec *models.CourseSection, wrote bool, err error) {
	if h.sections == nil {
		return nil, true, responses.InternalError(c, "section management not configured")
	}
	id, perr := c.ParamsInt("id")
	if perr != nil || id <= 0 {
		return nil, true, responses.BadRequest(c, "Invalid section ID")
	}
	sec, ferr := h.sections.Find(c.Context(), uint(id), enrollmentCourseScope(c))
	if ferr != nil {
		return nil, true, responses.NotFound(c, "section")
	}
	// ResourceAuthorizer returns a *fiber.Error (it never writes).
	if aerr := h.authz.RequireCourseTeacher(c, sec.CourseID); aerr != nil {
		return nil, true, aerr
	}
	return sec, false, nil
}

func sectionError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, service.ErrSectionNameRequired), errors.Is(err, service.ErrSectionDates),
		errors.Is(err, service.ErrSectionHasEnrollments), errors.Is(err, service.ErrSectionNotCrosslisted),
		errors.Is(err, service.ErrCrosslistSameCourse):
		return responses.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrCrosslistCourseInvalid):
		return responses.NotFound(c, "course")
	case errors.Is(err, service.ErrSectionNotFound):
		return responses.NotFound(c, "section")
	}
	return responses.InternalError(c, "Could not update section")
}

// UpdateSection handles PUT /sections/:id with {"course_section": {"name",
// "start_at", "end_at", "restrict_enrollments_to_section_dates"}}.
func (h *SectionHandler) UpdateSection(c *fiber.Ctx) error {
	sec, wrote, err := h.loadManagedSection(c)
	if wrote {
		return err
	}
	var input struct {
		CourseSection map[string]json.RawMessage `json:"course_section"`
	}
	if err := json.Unmarshal(c.Body(), &input); err != nil || input.CourseSection == nil {
		return responses.BadRequest(c, "Invalid input")
	}
	var upd service.SectionUpdate
	for key, raw := range input.CourseSection {
		switch key {
		case "name":
			var name string
			if err := json.Unmarshal(raw, &name); err != nil {
				return responses.BadRequest(c, "Invalid name")
			}
			upd.Name = &name
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
		case "restrict_enrollments_to_section_dates":
			var b bool
			if err := json.Unmarshal(raw, &b); err != nil {
				return responses.BadRequest(c, "Invalid restrict_enrollments_to_section_dates")
			}
			upd.RestrictEnrollmentsToSectionDates = &b
		}
	}
	if err := h.sections.Update(c.Context(), sec, upd); err != nil {
		return sectionError(c, err)
	}
	return c.JSON(sectionToJSON(sec))
}

// DeleteSection handles DELETE /sections/:id.
func (h *SectionHandler) DeleteSection(c *fiber.Ctx) error {
	sec, wrote, err := h.loadManagedSection(c)
	if wrote {
		return err
	}
	if err := h.sections.Delete(c.Context(), sec); err != nil {
		return sectionError(c, err)
	}
	return c.JSON(sectionToJSON(sec))
}

// CrosslistSection handles POST /sections/:id/crosslist/:new_course_id.
func (h *SectionHandler) CrosslistSection(c *fiber.Ctx) error {
	sec, wrote, err := h.loadManagedSection(c)
	if wrote {
		return err
	}
	target, perr := c.ParamsInt("new_course_id")
	if perr != nil || target <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}
	// The caller must also manage the destination course (404 when it is
	// in another tenant, 403 when they don't teach it).
	if aerr := h.authz.RequireCourseTeacher(c, uint(target)); aerr != nil {
		return aerr
	}
	if err := h.sections.CrosslistTo(c.Context(), sec, uint(target), enrollmentCourseScope(c)); err != nil {
		return sectionError(c, err)
	}
	return c.JSON(sectionToJSON(sec))
}

// UncrosslistSection handles DELETE /sections/:id/crosslist.
func (h *SectionHandler) UncrosslistSection(c *fiber.Ctx) error {
	sec, wrote, err := h.loadManagedSection(c)
	if wrote {
		return err
	}
	if sec.NonxlistCourseID == nil {
		return sectionError(c, service.ErrSectionNotCrosslisted)
	}
	if aerr := h.authz.RequireCourseTeacher(c, *sec.NonxlistCourseID); aerr != nil {
		return aerr
	}
	if err := h.sections.Uncrosslist(c.Context(), sec); err != nil {
		return sectionError(c, err)
	}
	return c.JSON(sectionToJSON(sec))
}
