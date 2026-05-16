package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type ConferenceHandler struct {
	conferenceService *service.ConferenceService
	authz             *ResourceAuthorizer
}

func NewConferenceHandler(conferenceService *service.ConferenceService, authz *ResourceAuthorizer) *ConferenceHandler {
	return &ConferenceHandler{conferenceService: conferenceService, authz: authz}
}

func conferenceToJSON(conf *models.Conference) fiber.Map {
	return fiber.Map{
		"id":              conf.ID,
		"context_type":    conf.ContextType,
		"context_id":      conf.ContextID,
		"conference_type": conf.ConferenceType,
		"title":           conf.Title,
		"description":     conf.Description,
		"user_id":         conf.UserID,
		"started_at":      conf.StartedAt,
		"ended_at":        conf.EndedAt,
		"duration":        conf.Duration,
		"join_url":        conf.JoinURL,
		"recordings":      conf.Recordings,
		"settings":        conf.Settings,
		"workflow_state":  conf.WorkflowState,
		"created_at":      conf.CreatedAt,
		"updated_at":      conf.UpdatedAt,
	}
}

func conferenceParticipantToJSON(p *models.ConferenceParticipant) fiber.Map {
	result := fiber.Map{
		"id":                 p.ID,
		"conference_id":      p.ConferenceID,
		"user_id":            p.UserID,
		"participation_type": p.ParticipationType,
		"created_at":         p.CreatedAt,
		"updated_at":         p.UpdatedAt,
	}
	if p.User.ID != 0 {
		result["user"] = fiber.Map{
			"id":   p.User.ID,
			"name": p.User.Name,
		}
	}
	return result
}

// ListConferences returns a paginated list of conferences for a course.
// GET /api/v1/courses/:course_id/conferences
func (h *ConferenceHandler) ListConferences(c *fiber.Ctx) error {
	courseID, err := strconv.Atoi(c.Params("course_id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.conferenceService.ListByContext(c.Context(), "Course", uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch conferences")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	conferences := make([]fiber.Map, len(result.Items))
	for i, conf := range result.Items {
		conferences[i] = conferenceToJSON(&conf)
	}

	return c.JSON(conferences)
}

// CreateConference creates a new conference for a course.
// POST /api/v1/courses/:course_id/conferences
func (h *ConferenceHandler) CreateConference(c *fiber.Ctx) error {
	courseID, err := strconv.Atoi(c.Params("course_id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		Conference struct {
			Title          string `json:"title"`
			Description    string `json:"description"`
			ConferenceType string `json:"conference_type"`
			Duration       int    `json:"duration"`
			Settings       string `json:"settings"`
		} `json:"conference"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	userID, _ := c.Locals("user_id").(uint)

	conference := &models.Conference{
		ContextType:    "Course",
		ContextID:      uint(courseID),
		ConferenceType: input.Conference.ConferenceType,
		Title:          input.Conference.Title,
		Description:    input.Conference.Description,
		Duration:       input.Conference.Duration,
		Settings:       input.Conference.Settings,
		UserID:         userID,
	}

	if err := h.conferenceService.Create(c.Context(), conference); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	// Add the creator as initiator
	_ = h.conferenceService.AddParticipant(c.Context(), conference.ID, userID, "initiator")

	return c.Status(fiber.StatusCreated).JSON(conferenceToJSON(conference))
}

// GetConference returns a single conference.
// GET /api/v1/courses/:course_id/conferences/:id
func (h *ConferenceHandler) GetConference(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid conference ID")
	}

	conference, err := h.conferenceService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "conference")
	}

	// Authorization: require enrollment for course-scoped conferences
	if conference.ContextType == "Course" {
		if err := h.authz.RequireCourseEnrolled(c, conference.ContextID); err != nil {
			return err
		}
	}

	return c.JSON(conferenceToJSON(conference))
}

// UpdateConference updates an existing conference.
// PUT /api/v1/courses/:course_id/conferences/:id
func (h *ConferenceHandler) UpdateConference(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid conference ID")
	}

	conference, err := h.conferenceService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "conference")
	}

	// Authorization: require instructor for course-scoped conferences
	if conference.ContextType == "Course" {
		if err := h.authz.RequireCourseInstructor(c, conference.ContextID); err != nil {
			return err
		}
	}

	var input struct {
		Conference struct {
			Title          *string `json:"title"`
			Description    *string `json:"description"`
			ConferenceType *string `json:"conference_type"`
			Duration       *int    `json:"duration"`
			Settings       *string `json:"settings"`
			WorkflowState  *string `json:"workflow_state"`
		} `json:"conference"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Conference.Title != nil {
		conference.Title = *input.Conference.Title
	}
	if input.Conference.Description != nil {
		conference.Description = *input.Conference.Description
	}
	if input.Conference.ConferenceType != nil {
		conference.ConferenceType = *input.Conference.ConferenceType
	}
	if input.Conference.Duration != nil {
		conference.Duration = *input.Conference.Duration
	}
	if input.Conference.Settings != nil {
		conference.Settings = *input.Conference.Settings
	}
	if input.Conference.WorkflowState != nil {
		conference.WorkflowState = *input.Conference.WorkflowState
	}

	if err := h.conferenceService.Update(c.Context(), conference); err != nil {
		return responses.InternalError(c, "Could not update conference")
	}

	return c.JSON(conferenceToJSON(conference))
}

// DeleteConference deletes a conference.
// DELETE /api/v1/courses/:course_id/conferences/:id
func (h *ConferenceHandler) DeleteConference(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid conference ID")
	}

	// Fetch first to check authorization
	conference, err := h.conferenceService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "conference")
	}

	// Authorization: require instructor for course-scoped conferences
	if conference.ContextType == "Course" {
		if err := h.authz.RequireCourseInstructor(c, conference.ContextID); err != nil {
			return err
		}
	}

	if err := h.conferenceService.Delete(c.Context(), conference.ID); err != nil {
		return responses.NotFound(c, "conference")
	}

	return c.JSON(fiber.Map{"delete": true})
}

// JoinConference generates a join URL for the current user.
// POST /api/v1/courses/:course_id/conferences/:id/join
func (h *ConferenceHandler) JoinConference(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid conference ID")
	}

	// Fetch to check authorization
	conference, err := h.conferenceService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "conference")
	}

	// Authorization: require enrollment for course-scoped conferences
	if conference.ContextType == "Course" {
		if err := h.authz.RequireCourseEnrolled(c, conference.ContextID); err != nil {
			return err
		}
	}

	userID, _ := c.Locals("user_id").(uint)

	joinURL, err := h.conferenceService.JoinConference(c.Context(), conference.ID, userID)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{
		"status":   "ok",
		"join_url": joinURL,
	})
}

// EndConference ends an active conference.
// POST /api/v1/courses/:course_id/conferences/:id/end
func (h *ConferenceHandler) EndConference(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid conference ID")
	}

	// Fetch first to check authorization
	conf, err := h.conferenceService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "conference")
	}

	// Authorization: require instructor for course-scoped conferences
	if conf.ContextType == "Course" {
		if err := h.authz.RequireCourseInstructor(c, conf.ContextID); err != nil {
			return err
		}
	}

	conference, err := h.conferenceService.EndConference(c.Context(), conf.ID)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(conferenceToJSON(conference))
}

// GetRecordings returns the recordings for a conference.
// GET /api/v1/courses/:course_id/conferences/:id/recordings
func (h *ConferenceHandler) GetRecordings(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid conference ID")
	}

	// Fetch to check authorization
	conference, err := h.conferenceService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "conference")
	}

	// Authorization: require enrollment for course-scoped conferences
	if conference.ContextType == "Course" {
		if err := h.authz.RequireCourseEnrolled(c, conference.ContextID); err != nil {
			return err
		}
	}

	recordings, err := h.conferenceService.GetRecordings(c.Context(), conference.ID)
	if err != nil {
		return responses.NotFound(c, "conference")
	}

	return c.JSON(fiber.Map{
		"conference_id": id,
		"recordings":    recordings,
	})
}

// GetParticipants returns the participants for a conference.
// GET /api/v1/courses/:course_id/conferences/:id/participants
func (h *ConferenceHandler) GetParticipants(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid conference ID")
	}

	// Fetch to check authorization
	conference, err := h.conferenceService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "conference")
	}

	// Authorization: require enrollment for course-scoped conferences
	if conference.ContextType == "Course" {
		if err := h.authz.RequireCourseEnrolled(c, conference.ContextID); err != nil {
			return err
		}
	}

	participants, err := h.conferenceService.ListParticipants(c.Context(), conference.ID)
	if err != nil {
		return responses.InternalError(c, "Could not fetch participants")
	}

	result := make([]fiber.Map, len(participants))
	for i, p := range participants {
		result[i] = conferenceParticipantToJSON(&p)
	}

	return c.JSON(result)
}
