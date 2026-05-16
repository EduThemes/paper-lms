package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type ModuleHandler struct {
	moduleService *service.ModuleService
}

func NewModuleHandler(moduleService *service.ModuleService) *ModuleHandler {
	return &ModuleHandler{moduleService: moduleService}
}

func moduleToJSON(m *models.ContextModule) fiber.Map {
	result := fiber.Map{
		"id":                          m.ID,
		"course_id":                   m.CourseID,
		"name":                        m.Name,
		"position":                    m.Position,
		"unlock_at":                   m.UnlockAt,
		"end_at":                      m.EndAt,
		"require_sequential_progress": m.RequireSequentialProgress,
		"workflow_state":              m.WorkflowState,
		"published":                   m.WorkflowState == "active",
		"items_count":                 len(m.Items),
	}

	if m.Items != nil {
		items := make([]fiber.Map, len(m.Items))
		for i, item := range m.Items {
			items[i] = moduleItemToJSON(&item)
		}
		result["items"] = items
	}

	return result
}

func moduleItemToJSON(item *models.ContentTag) fiber.Map {
	result := fiber.Map{
		"id":                item.ID,
		"module_id":         item.ContextModuleID,
		"title":             item.Title,
		"position":          item.Position,
		"indent":            item.Indent,
		"type":              contentTypeToItemType(item.ContentType),
		"content_id":        item.ContentID,
		"url":               item.URL,
		"new_tab":           item.NewTab,
		"workflow_state":    item.WorkflowState,
		"published":         item.WorkflowState == "active",
	}
	if item.ContentType == "WikiPage" && item.URL != "" {
		result["page_url"] = item.URL
	}
	return result
}

func contentTypeToItemType(ct string) string {
	switch ct {
	case "WikiPage":
		return "Page"
	case "Assignment":
		return "Assignment"
	case "Quizzes::Quiz":
		return "Quiz"
	case "DiscussionTopic":
		return "Discussion"
	case "ExternalUrl":
		return "ExternalUrl"
	case "ContextModuleSubHeader":
		return "SubHeader"
	default:
		return ct
	}
}

func (h *ModuleHandler) ListModules(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.moduleService.ListByCourse(c.Context(), uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch modules")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	include := c.Query("include[]")

	mods := make([]fiber.Map, len(result.Items))
	for i, m := range result.Items {
		mj := moduleToJSON(&m)
		if include != "items" {
			delete(mj, "items")
		}
		mods[i] = mj
	}

	return c.JSON(mods)
}

func (h *ModuleHandler) GetModule(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid module ID")
	}

	module, err := h.moduleService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "module")
	}

	return c.JSON(moduleToJSON(module))
}

func (h *ModuleHandler) CreateModule(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		Module struct {
			Name                      string     `json:"name"`
			Position                  int        `json:"position"`
			UnlockAt                  *time.Time `json:"unlock_at"`
			EndAt                     *time.Time `json:"end_at"`
			RequireSequentialProgress bool       `json:"require_sequential_progress"`
		} `json:"module"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	module := &models.ContextModule{
		CourseID:                  uint(courseID),
		Name:                     input.Module.Name,
		Position:                 input.Module.Position,
		UnlockAt:                 input.Module.UnlockAt,
		EndAt:                    input.Module.EndAt,
		RequireSequentialProgress: input.Module.RequireSequentialProgress,
		WorkflowState:            "active",
	}

	if err := h.moduleService.Create(c.Context(), module); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(moduleToJSON(module))
}

func (h *ModuleHandler) UpdateModule(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid module ID")
	}

	module, err := h.moduleService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "module")
	}

	var input struct {
		Module struct {
			Name                      *string    `json:"name"`
			Position                  *int       `json:"position"`
			UnlockAt                  *time.Time `json:"unlock_at"`
			EndAt                     *time.Time `json:"end_at"`
			RequireSequentialProgress *bool      `json:"require_sequential_progress"`
			Published                 *bool      `json:"published"`
		} `json:"module"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Module.Name != nil {
		module.Name = *input.Module.Name
	}
	if input.Module.Position != nil {
		module.Position = *input.Module.Position
	}
	if input.Module.UnlockAt != nil {
		module.UnlockAt = input.Module.UnlockAt
	}
	if input.Module.EndAt != nil {
		module.EndAt = input.Module.EndAt
	}
	if input.Module.RequireSequentialProgress != nil {
		module.RequireSequentialProgress = *input.Module.RequireSequentialProgress
	}
	if input.Module.Published != nil {
		if *input.Module.Published {
			module.WorkflowState = "active"
		} else {
			module.WorkflowState = "unpublished"
		}
	}

	if err := h.moduleService.Update(c.Context(), module); err != nil {
		return responses.InternalError(c, "Could not update module")
	}

	return c.JSON(moduleToJSON(module))
}

func (h *ModuleHandler) ReorderModules(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		Order []uint `json:"order"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if len(input.Order) == 0 {
		return responses.BadRequest(c, "Order array is required")
	}

	if err := h.moduleService.ReorderModules(c.Context(), uint(courseID), input.Order); err != nil {
		return responses.InternalError(c, "Could not reorder modules")
	}

	return c.JSON(fiber.Map{"reorder": true})
}

func (h *ModuleHandler) DeleteModule(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid module ID")
	}

	if err := h.moduleService.Delete(c.Context(), uint(id)); err != nil {
		return responses.InternalError(c, "Could not delete module")
	}

	return c.JSON(fiber.Map{"delete": true})
}

func (h *ModuleHandler) GetPrerequisites(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid module ID")
	}

	prereqs, err := h.moduleService.GetPrerequisites(c.Context(), uint(id))
	if err != nil {
		return responses.InternalError(c, "Could not get prerequisites")
	}

	return c.JSON(fiber.Map{"module_id": id, "prerequisite_module_ids": prereqs})
}

func (h *ModuleHandler) SetPrerequisites(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid module ID")
	}

	var input struct {
		PrerequisiteModuleIDs []uint `json:"prerequisite_module_ids"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if err := h.moduleService.SetPrerequisites(c.Context(), uint(id), input.PrerequisiteModuleIDs); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{"module_id": id, "prerequisite_module_ids": input.PrerequisiteModuleIDs})
}
