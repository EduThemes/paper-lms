package handlers

import (
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type SectionHandler struct {
	sectionRepo repository.SectionRepository
	authz       *ResourceAuthorizer
	// sections (SetSectionService) backs update / delete / cross-listing
	// (section_management.go).
	sections *service.SectionService
}

func NewSectionHandler(sectionRepo repository.SectionRepository, authz *ResourceAuthorizer) *SectionHandler {
	return &SectionHandler{sectionRepo: sectionRepo, authz: authz}
}

func sectionToJSON(s *models.CourseSection) fiber.Map {
	return fiber.Map{
		"id":             s.ID,
		"course_id":      s.CourseID,
		"name":           s.Name,
		"sis_section_id": s.SISSectionID,
		"workflow_state": s.WorkflowState,
		"start_at":       s.StartAt,
		"end_at":         s.EndAt,
		"created_at":     s.CreatedAt,
		"restrict_enrollments_to_section_dates": s.RestrictEnrollmentsToSectionDates,
		"nonxlist_course_id":                    s.NonxlistCourseID,
	}
}

func (h *SectionHandler) ListSections(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.sectionRepo.ListByCourseID(c.Context(), uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch sections")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	sections := make([]fiber.Map, len(result.Items))
	for i, s := range result.Items {
		sections[i] = sectionToJSON(&s)
	}

	return c.JSON(sections)
}

func (h *SectionHandler) CreateSection(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		CourseSection struct {
			Name                              string     `json:"name"`
			StartAt                           *time.Time `json:"start_at"`
			EndAt                             *time.Time `json:"end_at"`
			RestrictEnrollmentsToSectionDates bool       `json:"restrict_enrollments_to_section_dates"`
		} `json:"course_section"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	// Plain-text field: strip markup, never HTML-sanitize.
	name := strings.TrimSpace(service.SanitizePlainText(input.CourseSection.Name))
	if name == "" {
		return responses.BadRequest(c, "Section name is required")
	}
	if s, e := input.CourseSection.StartAt, input.CourseSection.EndAt; s != nil && e != nil && e.Before(*s) {
		return responses.BadRequest(c, "end_at must be after start_at")
	}

	section := &models.CourseSection{
		CourseID:                          uint(courseID),
		Name:                              name,
		WorkflowState:                     "active",
		StartAt:                           input.CourseSection.StartAt,
		EndAt:                             input.CourseSection.EndAt,
		RestrictEnrollmentsToSectionDates: input.CourseSection.RestrictEnrollmentsToSectionDates,
	}

	if err := h.sectionRepo.Create(c.Context(), section); err != nil {
		return responses.InternalError(c, "Could not create section")
	}

	return c.Status(fiber.StatusCreated).JSON(sectionToJSON(section))
}

func (h *SectionHandler) GetSection(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid section ID")
	}

	section, err := h.sectionRepo.FindByID(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "section")
	}

	// Section authz lives on the owning course: only enrolled callers
	// see it. Pre-2026-05-22 this endpoint was open to any authenticated
	// user, including cross-tenant — a course enumeration vector.
	if err := h.authz.RequireCourseEnrolled(c, section.CourseID); err != nil {
		return err
	}

	return c.JSON(sectionToJSON(section))
}
