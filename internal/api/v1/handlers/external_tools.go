package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type ExternalToolHandler struct {
	toolService   *service.ExternalToolService
	devKeyService *service.DeveloperKeyService
}

func NewExternalToolHandler(toolService *service.ExternalToolService, devKeyService *service.DeveloperKeyService) *ExternalToolHandler {
	return &ExternalToolHandler{
		toolService:   toolService,
		devKeyService: devKeyService,
	}
}

// externalToolToJSON serializes a context external tool for API responses.
func externalToolToJSON(tool *models.ContextExternalTool) fiber.Map {
	return fiber.Map{
		"id":               tool.ID,
		"context_type":     tool.ContextType,
		"context_id":       tool.ContextID,
		"developer_key_id": tool.DeveloperKeyID,
		"name":             tool.Name,
		"description":      tool.Description,
		"url":              tool.URL,
		"domain":           tool.Domain,
		"consumer_key":     tool.ConsumerKey,
		"custom_fields":    tool.CustomFields,
		"workflow_state":   tool.WorkflowState,
		"created_at":       tool.CreatedAt,
		"updated_at":       tool.UpdatedAt,
	}
}

// ListExternalTools returns a paginated list of external tools for a course.
// GET /api/v1/courses/:course_id/external_tools
func (h *ExternalToolHandler) ListExternalTools(c *fiber.Ctx) error {
	courseID, err := strconv.Atoi(c.Params("course_id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.toolService.ListByContext(c.Context(), "Course", uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch external tools")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	tools := make([]fiber.Map, len(result.Items))
	for i, tool := range result.Items {
		tools[i] = externalToolToJSON(&tool)
	}

	return c.JSON(tools)
}

// GetExternalTool returns a single external tool.
// GET /api/v1/courses/:course_id/external_tools/:id
func (h *ExternalToolHandler) GetExternalTool(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid external tool ID")
	}

	tool, err := h.toolService.GetByID(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "external tool")
	}

	return c.JSON(externalToolToJSON(tool))
}

type createExternalToolRequest struct {
	ExternalTool struct {
		Name           string `json:"name"`
		DeveloperKeyID uint   `json:"developer_key_id"`
		URL            string `json:"url"`
		Domain         string `json:"domain"`
		Description    string `json:"description"`
		ConsumerKey    string `json:"consumer_key"`
		SharedSecret   string `json:"shared_secret"`
		CustomFields   string `json:"custom_fields"`
	} `json:"external_tool"`
}

// CreateExternalTool creates a new external tool for a course.
// POST /api/v1/courses/:course_id/external_tools
func (h *ExternalToolHandler) CreateExternalTool(c *fiber.Ctx) error {
	courseID, err := strconv.Atoi(c.Params("course_id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input createExternalToolRequest
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.ExternalTool.Name == "" {
		return responses.BadRequest(c, "External tool name is required")
	}

	// Validate developer key exists if provided
	if input.ExternalTool.DeveloperKeyID != 0 {
		_, err := h.devKeyService.GetByID(c.Context(), input.ExternalTool.DeveloperKeyID)
		if err != nil {
			return responses.BadRequest(c, "Invalid developer_key_id")
		}
	}

	tool := &models.ContextExternalTool{
		ContextType:    "Course",
		ContextID:      uint(courseID),
		DeveloperKeyID: input.ExternalTool.DeveloperKeyID,
		Name:           input.ExternalTool.Name,
		Description:    input.ExternalTool.Description,
		URL:            input.ExternalTool.URL,
		Domain:         input.ExternalTool.Domain,
		ConsumerKey:    input.ExternalTool.ConsumerKey,
		SharedSecret:   input.ExternalTool.SharedSecret,
		CustomFields:   input.ExternalTool.CustomFields,
	}

	if err := h.toolService.Create(c.Context(), tool); err != nil {
		return responses.InternalError(c, "Could not create external tool")
	}

	return c.Status(fiber.StatusCreated).JSON(externalToolToJSON(tool))
}

// UpdateExternalTool updates an existing external tool.
// PUT /api/v1/courses/:course_id/external_tools/:id
func (h *ExternalToolHandler) UpdateExternalTool(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid external tool ID")
	}

	tool, err := h.toolService.GetByID(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "external tool")
	}

	var input struct {
		ExternalTool struct {
			Name           *string `json:"name"`
			URL            *string `json:"url"`
			Domain         *string `json:"domain"`
			Description    *string `json:"description"`
			ConsumerKey    *string `json:"consumer_key"`
			SharedSecret   *string `json:"shared_secret"`
			CustomFields   *string `json:"custom_fields"`
			DeveloperKeyID *uint   `json:"developer_key_id"`
			WorkflowState  *string `json:"workflow_state"`
		} `json:"external_tool"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.ExternalTool.Name != nil {
		tool.Name = *input.ExternalTool.Name
	}
	if input.ExternalTool.URL != nil {
		tool.URL = *input.ExternalTool.URL
	}
	if input.ExternalTool.Domain != nil {
		tool.Domain = *input.ExternalTool.Domain
	}
	if input.ExternalTool.Description != nil {
		tool.Description = *input.ExternalTool.Description
	}
	if input.ExternalTool.ConsumerKey != nil {
		tool.ConsumerKey = *input.ExternalTool.ConsumerKey
	}
	if input.ExternalTool.SharedSecret != nil {
		tool.SharedSecret = *input.ExternalTool.SharedSecret
	}
	if input.ExternalTool.CustomFields != nil {
		tool.CustomFields = *input.ExternalTool.CustomFields
	}
	if input.ExternalTool.DeveloperKeyID != nil {
		// Validate the new developer key exists
		_, dkErr := h.devKeyService.GetByID(c.Context(), *input.ExternalTool.DeveloperKeyID)
		if dkErr != nil {
			return responses.BadRequest(c, "Invalid developer_key_id")
		}
		tool.DeveloperKeyID = *input.ExternalTool.DeveloperKeyID
	}
	if input.ExternalTool.WorkflowState != nil {
		state := *input.ExternalTool.WorkflowState
		if state != "active" && state != "inactive" && state != "deleted" {
			return responses.BadRequest(c, "workflow_state must be 'active', 'inactive', or 'deleted'")
		}
		tool.WorkflowState = state
	}

	if err := h.toolService.Update(c.Context(), tool); err != nil {
		return responses.InternalError(c, "Could not update external tool")
	}

	return c.JSON(externalToolToJSON(tool))
}

// DeleteExternalTool deletes an external tool.
// DELETE /api/v1/courses/:course_id/external_tools/:id
func (h *ExternalToolHandler) DeleteExternalTool(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid external tool ID")
	}

	if err := h.toolService.Delete(c.Context(), uint(id)); err != nil {
		return responses.NotFound(c, "external tool")
	}

	return c.JSON(fiber.Map{"delete": true})
}
