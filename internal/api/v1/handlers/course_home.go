package handlers

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type CourseHomeHandler struct {
	courseHomeService *service.CourseHomeService
}

func NewCourseHomeHandler(courseHomeService *service.CourseHomeService) *CourseHomeHandler {
	return &CourseHomeHandler{courseHomeService: courseHomeService}
}

func (h *CourseHomeHandler) GetHomeData(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	data, err := h.courseHomeService.GetHomeData(c.Context(), uint(courseID), userID)
	if err != nil {
		return responses.NotFound(c, "course")
	}

	return c.JSON(data)
}

func (h *CourseHomeHandler) RecordVisit(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	var input struct {
		URL   string `json:"url"`
		Title string `json:"title"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if err := h.courseHomeService.RecordVisit(c.Context(), userID, uint(courseID), input.URL, input.Title); err != nil {
		return responses.InternalError(c, "Could not record visit")
	}

	return c.JSON(fiber.Map{"success": true})
}

func (h *CourseHomeHandler) ListButtons(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	buttons, err := h.courseHomeService.ListButtons(c.Context(), uint(courseID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch buttons")
	}

	return c.JSON(buttons)
}

func (h *CourseHomeHandler) CreateButton(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		ButtonType string `json:"button_type"`
		Label      string `json:"label"`
		Icon       string `json:"icon"`
		Color      string `json:"color"`
		LinkType   string `json:"link_type"`
		LinkID     *uint  `json:"link_id"`
		LinkURL    string `json:"link_url"`
		Position   int    `json:"position"`
		Visible    *bool  `json:"visible"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	button := &models.CourseHomeButton{
		CourseID:   uint(courseID),
		ButtonType: input.ButtonType,
		Label:      input.Label,
		Icon:       input.Icon,
		Color:      input.Color,
		LinkType:   input.LinkType,
		LinkID:     input.LinkID,
		LinkURL:    input.LinkURL,
		Position:   input.Position,
		Visible:    true,
	}
	if input.Visible != nil {
		button.Visible = *input.Visible
	}

	if err := h.courseHomeService.CreateButton(c.Context(), button); err != nil {
		return responses.InternalError(c, "Could not create button")
	}

	return c.Status(fiber.StatusCreated).JSON(button)
}

func (h *CourseHomeHandler) UpdateButton(c *fiber.Ctx) error {
	_, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	buttonID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid button ID")
	}

	existing, err := h.courseHomeService.GetButtonByID(c.Context(), uint(buttonID))
	if err != nil {
		return responses.NotFound(c, "button")
	}

	var input struct {
		ButtonType *string `json:"button_type"`
		Label      *string `json:"label"`
		Icon       *string `json:"icon"`
		Color      *string `json:"color"`
		LinkType   *string `json:"link_type"`
		LinkID     *uint   `json:"link_id"`
		LinkURL    *string `json:"link_url"`
		Position   *int    `json:"position"`
		Visible    *bool   `json:"visible"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.ButtonType != nil {
		existing.ButtonType = *input.ButtonType
	}
	if input.Label != nil {
		existing.Label = *input.Label
	}
	if input.Icon != nil {
		existing.Icon = *input.Icon
	}
	if input.Color != nil {
		existing.Color = *input.Color
	}
	if input.LinkType != nil {
		existing.LinkType = *input.LinkType
	}
	if input.LinkID != nil {
		existing.LinkID = input.LinkID
	}
	if input.LinkURL != nil {
		existing.LinkURL = *input.LinkURL
	}
	if input.Position != nil {
		existing.Position = *input.Position
	}
	if input.Visible != nil {
		existing.Visible = *input.Visible
	}

	if err := h.courseHomeService.UpdateButton(c.Context(), existing); err != nil {
		return responses.InternalError(c, "Could not update button")
	}

	return c.JSON(existing)
}

func (h *CourseHomeHandler) DeleteButton(c *fiber.Ctx) error {
	_, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	buttonID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid button ID")
	}

	if err := h.courseHomeService.DeleteButton(c.Context(), uint(buttonID)); err != nil {
		return responses.InternalError(c, "Could not delete button")
	}

	return c.JSON(fiber.Map{"delete": true})
}

func (h *CourseHomeHandler) ReorderButtons(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		Positions map[string]int `json:"positions"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	positions := make(map[uint]int, len(input.Positions))
	for key, pos := range input.Positions {
		id, err := strconv.ParseUint(key, 10, 64)
		if err != nil {
			return responses.BadRequest(c, "Invalid button ID in positions: "+key)
		}
		positions[uint(id)] = pos
	}

	if err := h.courseHomeService.ReorderButtons(c.Context(), uint(courseID), positions); err != nil {
		return responses.InternalError(c, "Could not reorder buttons")
	}

	return c.JSON(fiber.Map{"success": true})
}

func (h *CourseHomeHandler) ListOverrides(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.courseHomeService.ListOverrides(c.Context(), uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch overrides")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	return c.JSON(result.Items)
}

func (h *CourseHomeHandler) CreateOverride(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		Date     string `json:"date"`
		LinkType string `json:"link_type"`
		LinkID   *uint  `json:"link_id"`
		LinkURL  string `json:"link_url"`
		Label    string `json:"label"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	date, err := time.Parse("2006-01-02", input.Date)
	if err != nil {
		return responses.BadRequest(c, "Invalid date format, expected YYYY-MM-DD")
	}

	override := &models.TodaysLessonOverride{
		CourseID: uint(courseID),
		Date:     date,
		LinkType: input.LinkType,
		LinkID:   input.LinkID,
		LinkURL:  input.LinkURL,
		Label:    input.Label,
	}

	if err := h.courseHomeService.CreateOverride(c.Context(), override); err != nil {
		return responses.InternalError(c, "Could not create override")
	}

	return c.Status(fiber.StatusCreated).JSON(override)
}

func (h *CourseHomeHandler) UpdateOverride(c *fiber.Ctx) error {
	_, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	overrideID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid override ID")
	}

	existing, err := h.courseHomeService.GetOverrideByID(c.Context(), uint(overrideID))
	if err != nil {
		return responses.NotFound(c, "override")
	}

	var input struct {
		Date     *string `json:"date"`
		LinkType *string `json:"link_type"`
		LinkID   *uint   `json:"link_id"`
		LinkURL  *string `json:"link_url"`
		Label    *string `json:"label"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Date != nil {
		date, err := time.Parse("2006-01-02", *input.Date)
		if err != nil {
			return responses.BadRequest(c, "Invalid date format, expected YYYY-MM-DD")
		}
		existing.Date = date
	}
	if input.LinkType != nil {
		existing.LinkType = *input.LinkType
	}
	if input.LinkID != nil {
		existing.LinkID = input.LinkID
	}
	if input.LinkURL != nil {
		existing.LinkURL = *input.LinkURL
	}
	if input.Label != nil {
		existing.Label = *input.Label
	}

	if err := h.courseHomeService.UpdateOverride(c.Context(), existing); err != nil {
		return responses.InternalError(c, "Could not update override")
	}

	return c.JSON(existing)
}

func (h *CourseHomeHandler) DeleteOverride(c *fiber.Ctx) error {
	_, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	overrideID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid override ID")
	}

	if err := h.courseHomeService.DeleteOverride(c.Context(), uint(overrideID)); err != nil {
		return responses.InternalError(c, "Could not delete override")
	}

	return c.JSON(fiber.Map{"delete": true})
}
