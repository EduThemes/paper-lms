package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type SectionHandler struct {
	sectionRepo repository.SectionRepository
}

func NewSectionHandler(sectionRepo repository.SectionRepository) *SectionHandler {
	return &SectionHandler{sectionRepo: sectionRepo}
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
			Name string `json:"name"`
		} `json:"course_section"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.CourseSection.Name == "" {
		return responses.BadRequest(c, "Section name is required")
	}

	section := &models.CourseSection{
		CourseID:      uint(courseID),
		Name:          input.CourseSection.Name,
		WorkflowState: "active",
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

	return c.JSON(sectionToJSON(section))
}
