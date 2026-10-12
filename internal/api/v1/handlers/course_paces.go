package handlers

import (
	"errors"
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
		if errors.Is(err, service.ErrPaceTargetNotInCourse) {
			return responses.NotFound(c, "user or section")
		}
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(coursePaceToJSON(pace))
}


// requirePaceInCourse loads :id under the caller's tenant and verifies
// the pace belongs to the URL's :course_id (F-013 parent-tie). Writes a
// 404 and returns wrote=true on any miss — the caller must `return nil`.
//
// B039: pre-fix every :id handler skipped the course tie, so a teacher
// of course A could read, edit, delete or publish another course's pace.
func (h *CoursePaceHandler) requirePaceInCourse(c *fiber.Ctx) (*models.CoursePace, bool) {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		_ = responses.BadRequest(c, "Invalid course ID")
		return nil, true
	}
	paceID, err := c.ParamsInt("id")
	if err != nil || paceID <= 0 {
		_ = responses.BadRequest(c, "Invalid course pace ID")
		return nil, true
	}
	pace, err := h.coursePaceService.GetByID(c.Context(), uint(paceID), callerAccountID(c))
	if err != nil || pace == nil || pace.CourseID != uint(courseID) {
		_ = responses.NotFound(c, "course pace")
		return nil, true
	}
	return pace, false
}

func (h *CoursePaceHandler) GetCoursePace(c *fiber.Ctx) error {
	pace, wrote := h.requirePaceInCourse(c)
	if wrote {
		return nil
	}

	return c.JSON(coursePaceToJSON(pace))
}

func (h *CoursePaceHandler) UpdateCoursePace(c *fiber.Ctx) error {
	pace, wrote := h.requirePaceInCourse(c)
	if wrote {
		return nil
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
	pace, wrote := h.requirePaceInCourse(c)
	if wrote {
		return nil
	}

	if err := h.coursePaceService.Delete(c.Context(), pace.ID); err != nil {
		return responses.InternalError(c, "Could not delete course pace")
	}

	return c.JSON(fiber.Map{"delete": true})
}

func (h *CoursePaceHandler) PublishCoursePace(c *fiber.Ctx) error {
	pace, wrote := h.requirePaceInCourse(c)
	if wrote {
		return nil
	}

	published, err := h.coursePaceService.PublishPace(c.Context(), pace.ID, callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not publish course pace")
	}

	return c.JSON(coursePaceToJSON(published))
}

func (h *CoursePaceHandler) GetPaceModuleItems(c *fiber.Ctx) error {
	pace, wrote := h.requirePaceInCourse(c)
	if wrote {
		return nil
	}

	items, err := h.coursePaceService.GetPaceItems(c.Context(), pace.ID)
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
	pace, wrote := h.requirePaceInCourse(c)
	if wrote {
		return nil
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
			CoursePaceID: pace.ID,
			ModuleItemID: mi.ModuleItemID,
			Duration:     mi.Duration,
		}
	}

	// B039: the service ties every module_item_id to the pace's course;
	// a foreign item (which publish would otherwise follow to another
	// course's assignment) is refused before anything is stored.
	updated, err := h.coursePaceService.UpdatePaceItems(c.Context(), pace, items)
	if errors.Is(err, service.ErrPaceItemNotInCourse) {
		return responses.NotFound(c, "module item")
	}
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	result := make([]fiber.Map, len(updated))
	for i, item := range updated {
		result[i] = coursePaceModuleItemToJSON(&item)
	}

	return c.JSON(result)
}

func (h *CoursePaceHandler) GetTimeline(c *fiber.Ctx) error {
	pace, wrote := h.requirePaceInCourse(c)
	if wrote {
		return nil
	}

	timeline, err := h.coursePaceService.ComputeTimeline(c.Context(), pace.ID, callerAccountID(c))
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
