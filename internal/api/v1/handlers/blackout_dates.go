package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

func blackoutDateToJSON(b *models.BlackoutDate) fiber.Map {
	return fiber.Map{
		"id":          b.ID,
		"course_id":   b.CourseID,
		"event_title": b.EventTitle,
		"start_date":  b.StartDate.Format("2006-01-02"),
		"end_date":    b.EndDate.Format("2006-01-02"),
		"created_at":  b.CreatedAt,
		"updated_at":  b.UpdatedAt,
	}
}

// ListBlackoutDates returns the course's pacing blackout dates (G079).
// GET /courses/:course_id/blackout_dates
func (h *CoursePaceHandler) ListBlackoutDates(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}
	rows, err := h.coursePaceService.ListBlackoutDates(c.Context(), uint(courseID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch blackout dates")
	}
	out := make([]fiber.Map, len(rows))
	for i := range rows {
		out[i] = blackoutDateToJSON(&rows[i])
	}
	return c.JSON(out)
}

// CreateBlackoutDate adds a blackout date to the course.
// POST /courses/:course_id/blackout_dates
// {"blackout_date": {"event_title", "start_date", "end_date"}}
func (h *CoursePaceHandler) CreateBlackoutDate(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}
	var input struct {
		BlackoutDate struct {
			EventTitle string `json:"event_title"`
			StartDate  string `json:"start_date"`
			EndDate    string `json:"end_date"`
		} `json:"blackout_date"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	start, err := parseTime(input.BlackoutDate.StartDate)
	if err != nil {
		return responses.BadRequest(c, "Invalid start_date format")
	}
	end, err := parseTime(input.BlackoutDate.EndDate)
	if err != nil {
		return responses.BadRequest(c, "Invalid end_date format")
	}
	b := &models.BlackoutDate{
		CourseID:   uint(courseID),
		EventTitle: input.BlackoutDate.EventTitle,
		StartDate:  start,
		EndDate:    end,
	}
	if err := h.coursePaceService.CreateBlackoutDate(c.Context(), b); err != nil {
		if errors.Is(err, service.ErrBlackoutDateInvalid) {
			return responses.BadRequest(c, err.Error())
		}
		return responses.InternalError(c, "Could not create blackout date")
	}
	return c.Status(fiber.StatusCreated).JSON(blackoutDateToJSON(b))
}

// DeleteBlackoutDate removes one of the course's blackout dates; another
// course's id is a 404.
// DELETE /courses/:course_id/blackout_dates/:id
func (h *CoursePaceHandler) DeleteBlackoutDate(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return responses.BadRequest(c, "Invalid blackout date ID")
	}
	found, err := h.coursePaceService.DeleteBlackoutDate(c.Context(), uint(id), uint(courseID))
	if err != nil {
		return responses.InternalError(c, "Could not delete blackout date")
	}
	if !found {
		return responses.NotFound(c, "blackout date")
	}
	return c.JSON(fiber.Map{"delete": true})
}
