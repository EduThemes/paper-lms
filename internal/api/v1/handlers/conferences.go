package handlers

import (
	"encoding/json"
	"errors"
	"net/url"
	"strconv"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/gofiber/fiber/v2"
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
		"id":                   conf.ID,
		"context_type":         conf.ContextType,
		"context_id":           conf.ContextID,
		"conference_type":      conf.ConferenceType,
		"title":                conf.Title,
		"description":          conf.Description,
		"user_id":              conf.UserID,
		"started_at":           conf.StartedAt,
		"ended_at":             conf.EndedAt,
		"duration":             conf.Duration,
		"join_url":             conf.JoinURL,
		"recording_enabled":    conf.RecordingEnabled,
		"has_provider_meeting": conf.ProviderMeetingID != "",
		"recordings":           conf.Recordings,
		"settings":             conf.Settings,
		"workflow_state":       conf.WorkflowState,
		"created_at":           conf.CreatedAt,
		"updated_at":           conf.UpdatedAt,
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

// loadConference fetches a conference in the caller's tenant and applies
// the course-role gate (instructor for manage, enrollment otherwise). It
// returns wrote=true after writing the refusal; the caller MUST return
// err immediately (the (result, wrote, err) convention).
func (h *ConferenceHandler) loadConference(c *fiber.Ctx, manage bool) (*models.Conference, bool, error) {
	id, err := c.ParamsInt("id")
	if err != nil {
		return nil, true, responses.BadRequest(c, "Invalid conference ID")
	}
	conference, err := h.conferenceService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return nil, true, responses.NotFound(c, "conference")
	}
	if conference.ContextType == "Course" {
		var aerr error
		if manage {
			aerr = h.authz.RequireCourseInstructor(c, conference.ContextID)
		} else {
			aerr = h.authz.RequireCourseEnrolled(c, conference.ContextID)
		}
		if authzDenied(c, aerr) {
			return nil, true, aerr
		}
	}
	return conference, false, nil
}

// conferenceLifecycleError maps lifecycle errors onto HTTP statuses.
func conferenceLifecycleError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, service.ErrConferenceNotConfigured),
		errors.Is(err, service.ErrConferenceNotStarted),
		errors.Is(err, service.ErrConferenceNotRunning),
		errors.Is(err, service.ErrConferenceEnded),
		errors.Is(err, service.ErrRecordingsUnsupported):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": err.Error()}},
		})
	case errors.Is(err, service.ErrRecordingNotFound):
		return responses.NotFound(c, "recording")
	default:
		return responses.Error(c, fiber.StatusBadGateway, err.Error())
	}
}

// ListConferences returns a paginated list of conferences for a course.
// GET /api/v1/courses/:course_id/conferences
func (h *ConferenceHandler) ListConferences(c *fiber.Ctx) error {
	courseID, err := strconv.Atoi(c.Params("course_id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.conferenceService.ListByContext(c.Context(), "Course", uint(courseID), callerAccountID(c), params)
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
			// RecordingEnabled makes the provider meeting recordable.
			RecordingEnabled bool `json:"recording_enabled"`
			// JoinURL is an optional external meeting link (e.g. a Zoom
			// link) used when no BigBlueButton server is configured.
			JoinURL string `json:"join_url"`
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
		Description:    service.SanitizePlainText(input.Conference.Description),
		Duration:       input.Conference.Duration,
		Settings:       input.Conference.Settings,
		JoinURL:        input.Conference.JoinURL,
		UserID:         userID,

		RecordingEnabled: input.Conference.RecordingEnabled,
	}
	if conference.Settings != "" && !json.Valid([]byte(conference.Settings)) {
		return responses.BadRequest(c, "settings must be a JSON object")
	}

	if err := h.conferenceService.Create(c.Context(), conference); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	// Add the creator as initiator
	_ = h.conferenceService.AddParticipant(c.Context(), conference.ID, userID, "initiator")

	return c.Status(fiber.StatusCreated).JSON(conferenceToJSON(conference))
}

// GetConference returns a single conference.
// GET /api/v1/conferences/:id
func (h *ConferenceHandler) GetConference(c *fiber.Ctx) error {
	conference, wrote, err := h.loadConference(c, false)
	if wrote {
		return err
	}
	return c.JSON(conferenceToJSON(conference))
}

// UpdateConference updates an existing conference.
// PUT /api/v1/conferences/:id
func (h *ConferenceHandler) UpdateConference(c *fiber.Ctx) error {
	conference, wrote, err := h.loadConference(c, true)
	if wrote {
		return err
	}

	var input struct {
		Conference struct {
			Title          *string `json:"title"`
			Description    *string `json:"description"`
			ConferenceType *string `json:"conference_type"`
			Duration       *int    `json:"duration"`
			Settings       *string `json:"settings"`
			JoinURL        *string `json:"join_url"`

			RecordingEnabled *bool `json:"recording_enabled"`
		} `json:"conference"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Conference.Title != nil {
		conference.Title = *input.Conference.Title
	}
	if input.Conference.Description != nil {
		conference.Description = service.SanitizePlainText(*input.Conference.Description)
	}
	if input.Conference.ConferenceType != nil {
		if *input.Conference.ConferenceType != "BigBlueButton" && *input.Conference.ConferenceType != "Zoom" {
			return responses.BadRequest(c, "conference_type must be 'BigBlueButton' or 'Zoom'")
		}
		conference.ConferenceType = *input.Conference.ConferenceType
	}
	if input.Conference.Duration != nil {
		if *input.Conference.Duration < 0 {
			return responses.BadRequest(c, "duration must not be negative")
		}
		conference.Duration = *input.Conference.Duration
	}
	if input.Conference.Settings != nil {
		if !json.Valid([]byte(*input.Conference.Settings)) {
			return responses.BadRequest(c, "settings must be a JSON object")
		}
		conference.Settings = *input.Conference.Settings
	}
	if input.Conference.RecordingEnabled != nil {
		conference.RecordingEnabled = *input.Conference.RecordingEnabled
	}
	if input.Conference.JoinURL != nil {
		if err := service.ValidateMeetingURL(*input.Conference.JoinURL); err != nil {
			return responses.BadRequest(c, err.Error())
		}
		conference.JoinURL = *input.Conference.JoinURL
	}
	// workflow_state is driven by start/end, not by direct writes.

	if err := h.conferenceService.Update(c.Context(), conference); err != nil {
		return responses.InternalError(c, "Could not update conference")
	}

	return c.JSON(conferenceToJSON(conference))
}

// DeleteConference deletes a conference.
// DELETE /api/v1/conferences/:id
func (h *ConferenceHandler) DeleteConference(c *fiber.Ctx) error {
	conference, wrote, err := h.loadConference(c, true)
	if wrote {
		return err
	}

	if err := h.conferenceService.Delete(c.Context(), conference.ID); err != nil {
		return responses.NotFound(c, "conference")
	}

	return c.JSON(fiber.Map{"delete": true})
}

// StartConference starts a conference (instructor only).
// POST /api/v1/conferences/:id/start
func (h *ConferenceHandler) StartConference(c *fiber.Ctx) error {
	conference, wrote, err := h.loadConference(c, true)
	if wrote {
		return err
	}
	if conference.EndedAt != nil {
		return conferenceLifecycleError(c, service.ErrConferenceEnded)
	}

	started, err := h.conferenceService.StartConference(c.Context(), conference, callerAccountID(c))
	if err != nil {
		return conferenceLifecycleError(c, err)
	}
	return c.JSON(conferenceToJSON(started))
}

// JoinConference returns the URL the current user opens to join.
// POST /api/v1/conferences/:id/join
//
// Like Canvas, an instructor joining a conference that has not started
// yet starts it; students can only join once it is running.
func (h *ConferenceHandler) JoinConference(c *fiber.Ctx) error {
	conference, wrote, err := h.loadConference(c, false)
	if wrote {
		return err
	}

	userID, _ := c.Locals("user_id").(uint)
	moderator := conference.ContextType == "Course" && callerIsCourseInstructor(c, h.authz, conference.ContextID)

	if conference.StartedAt == nil && moderator {
		if _, err := h.conferenceService.StartConference(c.Context(), conference, callerAccountID(c)); err != nil {
			return conferenceLifecycleError(c, err)
		}
	}

	userName, _ := c.Locals("user_name").(string)
	joinURL, err := h.conferenceService.JoinConference(c.Context(), conference, userID, userName, moderator, callerAccountID(c))
	if err != nil {
		return conferenceLifecycleError(c, err)
	}

	return c.JSON(fiber.Map{
		"status":     "ok",
		"join_url":   joinURL,
		"conference": conferenceToJSON(conference),
	})
}

// EndConference ends an active conference.
// POST /api/v1/conferences/:id/end
func (h *ConferenceHandler) EndConference(c *fiber.Ctx) error {
	conf, wrote, err := h.loadConference(c, true)
	if wrote {
		return err
	}

	conference, err := h.conferenceService.EndConference(c.Context(), conf, callerAccountID(c))
	if err != nil {
		return conferenceLifecycleError(c, err)
	}

	return c.JSON(conferenceToJSON(conference))
}

// GetRecordings returns the recordings for a conference.
// GET /api/v1/conferences/:id/recordings
func (h *ConferenceHandler) GetRecordings(c *fiber.Ctx) error {
	conference, wrote, err := h.loadConference(c, false)
	if wrote {
		return err
	}

	recordings, err := h.conferenceService.GetRecordings(c.Context(), conference, callerAccountID(c))
	if err != nil {
		return conferenceLifecycleError(c, err)
	}

	return c.JSON(fiber.Map{
		"conference_id": conference.ID,
		"recordings":    recordings,
	})
}

// DeleteRecording deletes one of the conference's provider recordings
// (instructor only).
// DELETE /api/v1/conferences/:id/recordings/:recording_id
func (h *ConferenceHandler) DeleteRecording(c *fiber.Ctx) error {
	conference, wrote, err := h.loadConference(c, true)
	if wrote {
		return err
	}
	recordingID, uerr := url.PathUnescape(c.Params("recording_id"))
	if uerr != nil || recordingID == "" {
		return responses.BadRequest(c, "Invalid recording ID")
	}
	if err := h.conferenceService.DeleteRecording(c.Context(), conference, recordingID, callerAccountID(c)); err != nil {
		return conferenceLifecycleError(c, err)
	}
	return c.JSON(fiber.Map{"delete": true})
}

// ListProviders reports which conference types this tenant can hold
// natively (BigBlueButton / Zoom configured) for the course's UI.
// GET /api/v1/courses/:course_id/conferences/providers
func (h *ConferenceHandler) ListProviders(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"providers": h.conferenceService.Providers(c.Context(), callerAccountID(c)),
	})
}

// GetParticipants returns the participants for a conference.
// GET /api/v1/conferences/:id/participants
func (h *ConferenceHandler) GetParticipants(c *fiber.Ctx) error {
	conference, wrote, err := h.loadConference(c, false)
	if wrote {
		return err
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
