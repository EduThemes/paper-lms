package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type CalendarEventHandler struct {
	calendarService *service.CalendarService
	authz           *ResourceAuthorizer
}

func NewCalendarEventHandler(calendarService *service.CalendarService, authz *ResourceAuthorizer) *CalendarEventHandler {
	return &CalendarEventHandler{calendarService: calendarService, authz: authz}
}

func calendarEventToJSON(e *models.CalendarEvent) fiber.Map {
	return fiber.Map{
		"id":                 e.ID,
		"context_type":       e.ContextType,
		"context_id":         e.ContextID,
		"title":              e.Title,
		"description":        e.Description,
		"start_at":           e.StartAt,
		"end_at":             e.EndAt,
		"location_name":      e.LocationName,
		"location_address":   e.LocationAddress,
		"all_day":            e.AllDay,
		"created_by_user_id": e.CreatedByUserID,
		"workflow_state":     e.WorkflowState,
		"created_at":         e.CreatedAt,
		"updated_at":         e.UpdatedAt,
	}
}

func (h *CalendarEventHandler) ListEvents(c *fiber.Ctx) error {
	var contextType string
	var contextID uint

	courseID, err := c.ParamsInt("course_id")
	if err == nil && courseID > 0 {
		contextType = "Course"
		contextID = uint(courseID)
	} else {
		userID, _ := c.Locals("user_id").(uint)
		if userID == 0 {
			return responses.Unauthorized(c)
		}
		contextType = "User"
		contextID = userID
	}

	params := middleware.GetPagination(c)

	result, err := h.calendarService.ListByContext(c.Context(), contextType, contextID, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch calendar events")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	events := make([]fiber.Map, len(result.Items))
	for i, e := range result.Items {
		events[i] = calendarEventToJSON(&e)
	}

	return c.JSON(events)
}

func (h *CalendarEventHandler) GetEvent(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid calendar event ID")
	}

	event, err := h.calendarService.GetByID(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "calendar event")
	}

	return c.JSON(calendarEventToJSON(event))
}

func (h *CalendarEventHandler) CreateEvent(c *fiber.Ctx) error {
	var input struct {
		CalendarEvent struct {
			ContextType     string     `json:"context_type"`
			ContextID       uint       `json:"context_id"`
			Title           string     `json:"title"`
			Description     string     `json:"description"`
			StartAt         time.Time  `json:"start_at"`
			EndAt           *time.Time `json:"end_at"`
			LocationName    string     `json:"location_name"`
			LocationAddress string     `json:"location_address"`
			AllDay          bool       `json:"all_day"`
		} `json:"calendar_event"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	userID, _ := c.Locals("user_id").(uint)

	event := &models.CalendarEvent{
		ContextType:     input.CalendarEvent.ContextType,
		ContextID:       input.CalendarEvent.ContextID,
		Title:           input.CalendarEvent.Title,
		Description:     input.CalendarEvent.Description,
		StartAt:         input.CalendarEvent.StartAt,
		EndAt:           input.CalendarEvent.EndAt,
		LocationName:    input.CalendarEvent.LocationName,
		LocationAddress: input.CalendarEvent.LocationAddress,
		AllDay:          input.CalendarEvent.AllDay,
		CreatedByUserID: userID,
	}

	if err := h.calendarService.Create(c.Context(), event); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(calendarEventToJSON(event))
}

func (h *CalendarEventHandler) UpdateEvent(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid calendar event ID")
	}

	event, err := h.calendarService.GetByID(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "calendar event")
	}

	// Authorization: only the event creator or admin can update
	if err := h.authz.RequireOwnerOrAdmin(c, event.CreatedByUserID); err != nil {
		return err
	}

	var input struct {
		CalendarEvent struct {
			Title           *string    `json:"title"`
			Description     *string    `json:"description"`
			StartAt         *time.Time `json:"start_at"`
			EndAt           *time.Time `json:"end_at"`
			LocationName    *string    `json:"location_name"`
			LocationAddress *string    `json:"location_address"`
			AllDay          *bool      `json:"all_day"`
		} `json:"calendar_event"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.CalendarEvent.Title != nil {
		event.Title = *input.CalendarEvent.Title
	}
	if input.CalendarEvent.Description != nil {
		event.Description = *input.CalendarEvent.Description
	}
	if input.CalendarEvent.StartAt != nil {
		event.StartAt = *input.CalendarEvent.StartAt
	}
	if input.CalendarEvent.EndAt != nil {
		event.EndAt = input.CalendarEvent.EndAt
	}
	if input.CalendarEvent.LocationName != nil {
		event.LocationName = *input.CalendarEvent.LocationName
	}
	if input.CalendarEvent.LocationAddress != nil {
		event.LocationAddress = *input.CalendarEvent.LocationAddress
	}
	if input.CalendarEvent.AllDay != nil {
		event.AllDay = *input.CalendarEvent.AllDay
	}

	if err := h.calendarService.Update(c.Context(), event); err != nil {
		return responses.InternalError(c, "Could not update calendar event")
	}

	return c.JSON(calendarEventToJSON(event))
}

func (h *CalendarEventHandler) DeleteEvent(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid calendar event ID")
	}

	// Authorization: only the event creator or admin can delete
	event, err := h.calendarService.GetByID(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "calendar event")
	}
	if err := h.authz.RequireOwnerOrAdmin(c, event.CreatedByUserID); err != nil {
		return err
	}

	if err := h.calendarService.Delete(c.Context(), uint(id)); err != nil {
		return responses.InternalError(c, "Could not delete calendar event")
	}

	return c.JSON(fiber.Map{"delete": true})
}

func (h *CalendarEventHandler) ExportAsICal(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(uint)
	if userID == 0 {
		return responses.Unauthorized(c)
	}

	now := time.Now()
	startDate := now.AddDate(0, 0, -30)
	endDate := now.AddDate(0, 0, 90)

	if sd := c.Query("start_date"); sd != "" {
		parsed, err := time.Parse("2006-01-02", sd)
		if err == nil {
			startDate = parsed
		}
	}
	if ed := c.Query("end_date"); ed != "" {
		parsed, err := time.Parse("2006-01-02", ed)
		if err == nil {
			endDate = parsed
		}
	}

	data, err := h.calendarService.ExportAsICalendar(c.Context(), "User", userID, startDate, endDate)
	if err != nil {
		return responses.InternalError(c, "Could not export calendar")
	}

	c.Set("Content-Type", "text/calendar; charset=utf-8")
	c.Set("Content-Disposition", "attachment; filename=\"calendar.ics\"")
	return c.Send(data)
}
