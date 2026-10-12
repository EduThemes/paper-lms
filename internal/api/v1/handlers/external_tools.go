package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/auth"
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
// custom_fields is returned as a JSON object when the stored text is one
// (Canvas shape); the LTI 1.1 consumer_key is staff-only.
func externalToolToJSON(tool *models.ContextExternalTool, includeConsumerKey bool) fiber.Map {
	var customFields interface{} = tool.CustomFields
	var obj map[string]interface{}
	if tool.CustomFields != "" && json.Unmarshal([]byte(tool.CustomFields), &obj) == nil {
		customFields = obj
	}
	m := fiber.Map{
		"id":               tool.ID,
		"context_type":     tool.ContextType,
		"context_id":       tool.ContextID,
		"developer_key_id": tool.DeveloperKeyID,
		"name":             tool.Name,
		"description":      tool.Description,
		"url":              tool.URL,
		"domain":           tool.Domain,
		"custom_fields":    customFields,
		"workflow_state":   tool.WorkflowState,
		"created_at":       tool.CreatedAt,
		"updated_at":       tool.UpdatedAt,
	}
	if includeConsumerKey {
		m["consumer_key"] = tool.ConsumerKey
	}
	return m
}

// flexUint accepts a JSON number or a numeric JSON string (HTML form
// inputs always yield strings). B084.
type flexUint struct {
	Set   bool
	Value uint
}

func (f *flexUint) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if bytes.Equal(b, []byte("null")) {
		return nil
	}
	f.Set = true
	var s string
	if len(b) > 0 && b[0] == '"' {
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		if s == "" {
			f.Set = false
			return nil
		}
	} else {
		s = string(b)
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return fmt.Errorf("expected an integer id, got %q", s)
	}
	f.Value = uint(n)
	return nil
}

// flexJSONText accepts custom_fields as a JSON object (Canvas shape) or as
// a string holding JSON; either way it is stored as text. B084.
type flexJSONText struct {
	Set   bool
	Value string
}

func (f *flexJSONText) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	f.Set = true
	if bytes.Equal(b, []byte("null")) {
		f.Value = ""
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		return json.Unmarshal(b, &f.Value)
	}
	f.Value = string(b)
	return nil
}

// requireToolInCourse loads the external tool named by :id and enforces
// the F-013 parent-tie (B185): it must be installed in the course named
// by :course_id (and live in the caller's tenant). Every mismatch is the
// existence-leak 404. Returns (tool, wrote); wrote==true means a response
// was already written and the caller must `return nil`.
func (h *ExternalToolHandler) requireToolInCourse(c *fiber.Ctx) (*models.ContextExternalTool, bool) {
	id, err := c.ParamsInt("id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid external tool ID")
		return nil, true
	}
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid course ID")
		return nil, true
	}
	tool, err := h.toolService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil || tool.ContextType != "Course" || tool.ContextID != uint(courseID) {
		_ = responses.NotFound(c, "external tool")
		return nil, true
	}
	return tool, false
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

	staff := callerIsCourseStaff(c)
	tools := make([]fiber.Map, len(result.Items))
	for i, tool := range result.Items {
		tools[i] = externalToolToJSON(&tool, staff)
	}

	return c.JSON(tools)
}

// GetExternalTool returns a single external tool.
// GET /api/v1/courses/:course_id/external_tools/:id
func (h *ExternalToolHandler) GetExternalTool(c *fiber.Ctx) error {
	tool, wrote := h.requireToolInCourse(c)
	if wrote {
		return nil
	}

	return c.JSON(externalToolToJSON(tool, callerIsCourseStaff(c)))
}

type createExternalToolRequest struct {
	ExternalTool struct {
		Name           string       `json:"name"`
		DeveloperKeyID flexUint     `json:"developer_key_id"`
		URL            string       `json:"url"`
		Domain         string       `json:"domain"`
		Description    string       `json:"description"`
		ConsumerKey    string       `json:"consumer_key"`
		SharedSecret   string       `json:"shared_secret"`
		CustomFields   flexJSONText `json:"custom_fields"`
	} `json:"external_tool"`
}

// sealSharedSecret encrypts an LTI 1.1 shared secret for storage (B191).
// Returns (value, wrote).
func sealSharedSecret(c *fiber.Ctx, plaintext string) (string, bool) {
	sealed, err := auth.SealString(plaintext)
	if err != nil {
		_ = responses.InternalError(c, "Could not encrypt shared secret")
		return "", true
	}
	return sealed, false
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
	if !input.ExternalTool.DeveloperKeyID.Set || input.ExternalTool.DeveloperKeyID.Value == 0 {
		return responses.BadRequest(c, "developer_key_id is required")
	}

	sharedSecret, wrote := sealSharedSecret(c, input.ExternalTool.SharedSecret)
	if wrote {
		return nil
	}

	tool := &models.ContextExternalTool{
		ContextType:    "Course",
		ContextID:      uint(courseID),
		DeveloperKeyID: input.ExternalTool.DeveloperKeyID.Value,
		Name:           input.ExternalTool.Name,
		Description:    service.SanitizePlainText(input.ExternalTool.Description),
		URL:            input.ExternalTool.URL,
		Domain:         input.ExternalTool.Domain,
		ConsumerKey:    input.ExternalTool.ConsumerKey,
		SharedSecret:   sharedSecret,
		CustomFields:   input.ExternalTool.CustomFields.Value,
	}

	// Developer key is validated tenant-scoped inside the service.
	if err := h.toolService.Create(c.Context(), tool, callerAccountID(c)); err != nil {
		if errors.Is(err, service.ErrExternalToolInvalid) {
			return responses.BadRequest(c, err.Error())
		}
		return responses.InternalError(c, "Could not create external tool")
	}

	return c.Status(fiber.StatusCreated).JSON(externalToolToJSON(tool, true))
}

// UpdateExternalTool updates an existing external tool.
// PUT /api/v1/courses/:course_id/external_tools/:id
func (h *ExternalToolHandler) UpdateExternalTool(c *fiber.Ctx) error {
	tool, wrote := h.requireToolInCourse(c)
	if wrote {
		return nil
	}

	var input struct {
		ExternalTool struct {
			Name           *string      `json:"name"`
			URL            *string      `json:"url"`
			Domain         *string      `json:"domain"`
			Description    *string      `json:"description"`
			ConsumerKey    *string      `json:"consumer_key"`
			SharedSecret   *string      `json:"shared_secret"`
			CustomFields   flexJSONText `json:"custom_fields"`
			DeveloperKeyID flexUint     `json:"developer_key_id"`
			WorkflowState  *string      `json:"workflow_state"`
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
		tool.Description = service.SanitizePlainText(*input.ExternalTool.Description)
	}
	if input.ExternalTool.ConsumerKey != nil {
		tool.ConsumerKey = *input.ExternalTool.ConsumerKey
	}
	if input.ExternalTool.SharedSecret != nil {
		sealed, wrote := sealSharedSecret(c, *input.ExternalTool.SharedSecret)
		if wrote {
			return nil
		}
		tool.SharedSecret = sealed
	}
	if input.ExternalTool.CustomFields.Set {
		tool.CustomFields = input.ExternalTool.CustomFields.Value
	}
	if input.ExternalTool.DeveloperKeyID.Set {
		// Validate the new developer key exists (tenant-scoped)
		_, dkErr := h.devKeyService.GetByID(c.Context(), input.ExternalTool.DeveloperKeyID.Value, callerAccountID(c))
		if dkErr != nil {
			return responses.BadRequest(c, "Invalid developer_key_id")
		}
		tool.DeveloperKeyID = input.ExternalTool.DeveloperKeyID.Value
	}
	if input.ExternalTool.WorkflowState != nil {
		state := *input.ExternalTool.WorkflowState
		if state != "active" && state != "inactive" && state != "deleted" {
			return responses.BadRequest(c, "workflow_state must be 'active', 'inactive', or 'deleted'")
		}
		tool.WorkflowState = state
	}

	if err := h.toolService.Update(c.Context(), tool, callerAccountID(c)); err != nil {
		return responses.InternalError(c, "Could not update external tool")
	}

	return c.JSON(externalToolToJSON(tool, true))
}

// DeleteExternalTool deletes an external tool.
// DELETE /api/v1/courses/:course_id/external_tools/:id
func (h *ExternalToolHandler) DeleteExternalTool(c *fiber.Ctx) error {
	tool, wrote := h.requireToolInCourse(c)
	if wrote {
		return nil
	}

	if err := h.toolService.Delete(c.Context(), tool.ID, callerAccountID(c)); err != nil {
		return responses.NotFound(c, "external tool")
	}

	return c.JSON(fiber.Map{"delete": true})
}
