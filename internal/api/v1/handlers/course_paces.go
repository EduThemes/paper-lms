package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type CoursePaceHandler struct {
	coursePaceService *service.CoursePaceService
}

func NewCoursePaceHandler(coursePaceService *service.CoursePaceService) *CoursePaceHandler {
	return &CoursePaceHandler{coursePaceService: coursePaceService}
}

func coursePaceToJSON(p *models.CoursePace) fiber.Map {
	return fiber.Map{
		"id":                p.ID,
		"course_id":         p.CourseID,
		"user_id":           p.UserID,
		"course_section_id": p.CourseSectionID,
		"workflow_state":    p.WorkflowState,
		"end_date":          p.EndDate,
		"exclude_weekends":  p.ExcludeWeekends,
		"hard_end_dates":    p.HardEndDates,
		"published_at":      p.PublishedAt,
		"created_at":        p.CreatedAt,
		"updated_at":        p.UpdatedAt,
	}
}

func coursePaceModuleItemToJSON(item *models.CoursePaceModuleItem) fiber.Map {
	return fiber.Map{
		"id":             item.ID,
		"course_pace_id": item.CoursePaceID,
		"module_item_id": item.ModuleItemID,
		"duration":       item.Duration,
		"created_at":     item.CreatedAt,
		"updated_at":     item.UpdatedAt,
	}
}

func (h *CoursePaceHandler) ListCoursePaces(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.coursePaceService.ListByCourse(c.Context(), uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch course paces")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	paces := make([]fiber.Map, len(result.Items))
	for i, p := range result.Items {
		paces[i] = coursePaceToJSON(&p)
	}

	return c.JSON(paces)
}

func (h *CoursePaceHandler) CreateCoursePace(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		CoursePace struct {
			UserID          *uint   `json:"user_id"`
			CourseSectionID *uint   `json:"course_section_id"`
			EndDate         *string `json:"end_date"`
			ExcludeWeekends *bool   `json:"exclude_weekends"`
			HardEndDates    *bool   `json:"hard_end_dates"`
		} `json:"course_pace"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	pace := &models.CoursePace{
		CourseID:        uint(courseID),
		UserID:          input.CoursePace.UserID,
		CourseSectionID: input.CoursePace.CourseSectionID,
	}

	if input.CoursePace.ExcludeWeekends != nil {
		pace.ExcludeWeekends = *input.CoursePace.ExcludeWeekends
	}
	if input.CoursePace.HardEndDates != nil {
		pace.HardEndDates = *input.CoursePace.HardEndDates
	}
	if input.CoursePace.EndDate != nil {
		t, parseErr := parseTime(*input.CoursePace.EndDate)
		if parseErr != nil {
			return responses.BadRequest(c, "Invalid end_date format")
		}
		pace.EndDate = &t
	}

	if err := h.coursePaceService.Create(c.Context(), pace); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(coursePaceToJSON(pace))
}

func (h *CoursePaceHandler) GetCoursePace(c *fiber.Ctx) error {
	paceID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course pace ID")
	}

	pace, err := h.coursePaceService.GetByID(c.Context(), uint(paceID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "course pace")
	}

	return c.JSON(coursePaceToJSON(pace))
}

func (h *CoursePaceHandler) UpdateCoursePace(c *fiber.Ctx) error {
	paceID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course pace ID")
	}

	pace, err := h.coursePaceService.GetByID(c.Context(), uint(paceID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "course pace")
	}

	var input struct {
		CoursePace struct {
			EndDate         *string `json:"end_date"`
			ExcludeWeekends *bool   `json:"exclude_weekends"`
			HardEndDates    *bool   `json:"hard_end_dates"`
		} `json:"course_pace"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.CoursePace.ExcludeWeekends != nil {
		pace.ExcludeWeekends = *input.CoursePace.ExcludeWeekends
	}
	if input.CoursePace.HardEndDates != nil {
		pace.HardEndDates = *input.CoursePace.HardEndDates
	}
	if input.CoursePace.EndDate != nil {
		t, parseErr := parseTime(*input.CoursePace.EndDate)
		if parseErr != nil {
			return responses.BadRequest(c, "Invalid end_date format")
		}
		pace.EndDate = &t
	}

	if err := h.coursePaceService.Update(c.Context(), pace); err != nil {
		return responses.InternalError(c, "Could not update course pace")
	}

	return c.JSON(coursePaceToJSON(pace))
}

func (h *CoursePaceHandler) DeleteCoursePace(c *fiber.Ctx) error {
	paceID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course pace ID")
	}

	if err := h.coursePaceService.Delete(c.Context(), uint(paceID)); err != nil {
		return responses.InternalError(c, "Could not delete course pace")
	}

	return c.JSON(fiber.Map{"delete": true})
}

func (h *CoursePaceHandler) PublishCoursePace(c *fiber.Ctx) error {
	paceID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course pace ID")
	}

	pace, err := h.coursePaceService.PublishPace(c.Context(), uint(paceID), callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not publish course pace")
	}

	return c.JSON(coursePaceToJSON(pace))
}

func (h *CoursePaceHandler) GetPaceModuleItems(c *fiber.Ctx) error {
	paceID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course pace ID")
	}

	items, err := h.coursePaceService.GetPaceItems(c.Context(), uint(paceID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch pace module items")
	}

	result := make([]fiber.Map, len(items))
	for i, item := range items {
		result[i] = coursePaceModuleItemToJSON(&item)
	}

	return c.JSON(result)
}

func (h *CoursePaceHandler) UpdatePaceModuleItems(c *fiber.Ctx) error {
	paceID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course pace ID")
	}

	var input struct {
		ModuleItems []struct {
			ModuleItemID uint `json:"module_item_id"`
			Duration     int  `json:"duration"`
		} `json:"module_items"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	items := make([]models.CoursePaceModuleItem, len(input.ModuleItems))
	for i, mi := range input.ModuleItems {
		items[i] = models.CoursePaceModuleItem{
			CoursePaceID: uint(paceID),
			ModuleItemID: mi.ModuleItemID,
			Duration:     mi.Duration,
		}
	}

	updated, err := h.coursePaceService.UpdatePaceItems(c.Context(), uint(paceID), items)
	if err != nil {
		return responses.InternalError(c, "Could not update pace module items")
	}

	result := make([]fiber.Map, len(updated))
	for i, item := range updated {
		result[i] = coursePaceModuleItemToJSON(&item)
	}

	return c.JSON(result)
}

func (h *CoursePaceHandler) GetTimeline(c *fiber.Ctx) error {
	paceID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course pace ID")
	}

	timeline, err := h.coursePaceService.ComputeTimeline(c.Context(), uint(paceID), callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not compute timeline")
	}

	return c.JSON(timeline)
}

// parseTime tries RFC3339 first, then falls back to date-only format.
func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, err = time.Parse("2006-01-02", s)
	}
	return t, err
}
