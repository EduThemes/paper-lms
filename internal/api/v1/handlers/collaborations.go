package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type CollaborationHandler struct {
	collaborationService *service.CollaborationService
	authz                *ResourceAuthorizer
}

func NewCollaborationHandler(collaborationService *service.CollaborationService, authz *ResourceAuthorizer) *CollaborationHandler {
	return &CollaborationHandler{collaborationService: collaborationService, authz: authz}
}

func collaborationToJSON(c *models.Collaboration) fiber.Map {
	return fiber.Map{
		"id":                 c.ID,
		"context_type":       c.ContextType,
		"context_id":         c.ContextID,
		"collaboration_type": c.CollaborationType,
		"title":              c.Title,
		"description":        c.Description,
		"url":                c.URL,
		"document_id":        c.DocumentID,
		"user_id":            c.UserID,
		"workflow_state":     c.WorkflowState,
		"created_at":         c.CreatedAt,
		"updated_at":         c.UpdatedAt,
	}
}

// ListCollaborations returns a paginated list of collaborations for a course.
// GET /api/v1/courses/:course_id/collaborations
func (h *CollaborationHandler) ListCollaborations(c *fiber.Ctx) error {
	courseID, err := strconv.Atoi(c.Params("course_id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.collaborationService.ListByContext(c.Context(), "Course", uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch collaborations")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	collaborations := make([]fiber.Map, len(result.Items))
	for i, collab := range result.Items {
		collaborations[i] = collaborationToJSON(&collab)
	}

	return c.JSON(collaborations)
}

// CreateCollaboration creates a new collaboration for a course.
// POST /api/v1/courses/:course_id/collaborations
func (h *CollaborationHandler) CreateCollaboration(c *fiber.Ctx) error {
	courseID, err := strconv.Atoi(c.Params("course_id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		Collaboration struct {
			Title             string `json:"title"`
			Description       string `json:"description"`
			CollaborationType string `json:"collaboration_type"`
			URL               string `json:"url"`
			DocumentID        string `json:"document_id"`
		} `json:"collaboration"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	userID, _ := c.Locals("user_id").(uint)

	collaboration := &models.Collaboration{
		ContextType:       "Course",
		ContextID:         uint(courseID),
		CollaborationType: input.Collaboration.CollaborationType,
		Title:             input.Collaboration.Title,
		Description:       input.Collaboration.Description,
		URL:               input.Collaboration.URL,
		DocumentID:        input.Collaboration.DocumentID,
		UserID:            userID,
	}

	if err := h.collaborationService.Create(c.Context(), collaboration); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(collaborationToJSON(collaboration))
}

// GetCollaboration returns a single collaboration.
// GET /api/v1/courses/:course_id/collaborations/:id
func (h *CollaborationHandler) GetCollaboration(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid collaboration ID")
	}

	collaboration, err := h.collaborationService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "collaboration")
	}

	// Authorization: require enrollment for course-scoped collaborations
	if collaboration.ContextType == "Course" {
		if err := h.authz.RequireCourseEnrolled(c, collaboration.ContextID); err != nil {
			return err
		}
	}

	return c.JSON(collaborationToJSON(collaboration))
}

// UpdateCollaboration updates an existing collaboration.
// PUT /api/v1/courses/:course_id/collaborations/:id
func (h *CollaborationHandler) UpdateCollaboration(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid collaboration ID")
	}

	collaboration, err := h.collaborationService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "collaboration")
	}

	// Authorization: require instructor for course-scoped collaborations
	if collaboration.ContextType == "Course" {
		if err := h.authz.RequireCourseInstructor(c, collaboration.ContextID); err != nil {
			return err
		}
	}

	var input struct {
		Collaboration struct {
			Title             *string `json:"title"`
			Description       *string `json:"description"`
			CollaborationType *string `json:"collaboration_type"`
			URL               *string `json:"url"`
			DocumentID        *string `json:"document_id"`
			WorkflowState     *string `json:"workflow_state"`
		} `json:"collaboration"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Collaboration.Title != nil {
		collaboration.Title = *input.Collaboration.Title
	}
	if input.Collaboration.Description != nil {
		collaboration.Description = *input.Collaboration.Description
	}
	if input.Collaboration.CollaborationType != nil {
		collaboration.CollaborationType = *input.Collaboration.CollaborationType
	}
	if input.Collaboration.URL != nil {
		collaboration.URL = *input.Collaboration.URL
	}
	if input.Collaboration.DocumentID != nil {
		collaboration.DocumentID = *input.Collaboration.DocumentID
	}
	if input.Collaboration.WorkflowState != nil {
		collaboration.WorkflowState = *input.Collaboration.WorkflowState
	}

	if err := h.collaborationService.Update(c.Context(), collaboration); err != nil {
		return responses.InternalError(c, "Could not update collaboration")
	}

	return c.JSON(collaborationToJSON(collaboration))
}

// DeleteCollaboration deletes a collaboration.
// DELETE /api/v1/courses/:course_id/collaborations/:id
func (h *CollaborationHandler) DeleteCollaboration(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid collaboration ID")
	}

	// Fetch first to check authorization
	collaboration, err := h.collaborationService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "collaboration")
	}

	// Authorization: require instructor for course-scoped collaborations
	if collaboration.ContextType == "Course" {
		if err := h.authz.RequireCourseInstructor(c, collaboration.ContextID); err != nil {
			return err
		}
	}

	if err := h.collaborationService.Delete(c.Context(), collaboration.ID); err != nil {
		return responses.NotFound(c, "collaboration")
	}

	return c.JSON(fiber.Map{"delete": true})
}
