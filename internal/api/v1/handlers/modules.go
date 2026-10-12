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
		"completion_type":   item.CompletionType,
		"min_score":         item.MinScore,
	}
	if item.ContentType == "WikiPage" && item.URL != "" {
		result["page_url"] = item.URL
	}
	return result
}

// visibleItems drops deleted items always and unpublished items unless
// the caller may see drafts (B045/B140: FindByID preloads every item).
func visibleItems(items []models.ContentTag, includeUnpublished bool) []models.ContentTag {
	out := make([]models.ContentTag, 0, len(items))
	for _, it := range items {
		if it.WorkflowState == models.ContentTagDeleted {
			continue
		}
		if !includeUnpublished && it.WorkflowState != models.ContentTagActive {
			continue
		}
		out = append(out, it)
	}
	return out
}

// loadModuleInCourse loads :id under the caller's tenant and verifies it
// is a live module of courseID that the caller may see (unpublished
// modules are hidden from students/observers). Writes a 404 and returns
// wrote=true on any miss — the caller must `return nil`.
func (h *ModuleHandler) loadModuleInCourse(c *fiber.Ctx, moduleID, courseID uint) (*models.ContextModule, bool) {
	module, err := h.moduleService.GetByID(c.Context(), moduleID, callerAccountID(c))
	if err != nil || module == nil || module.CourseID != courseID || module.WorkflowState == "deleted" {
		_ = responses.NotFound(c, "module")
		return nil, true
	}
	if module.WorkflowState != "active" && !canViewUnpublishedContent(c) {
		_ = responses.NotFound(c, "module")
		return nil, true
	}
	return module, false
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
	case "Attachment":
		return "File"
	case "Quiz":
		// IMSCC imports write "Quiz"; the API type is the same.
		return "Quiz"
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

	// B045: students and observers see only published modules and
	// published items; drafts are for course managers.
	list := h.moduleService.ListPublishedByCourse
	if canViewUnpublishedContent(c) {
		list = h.moduleService.ListByCourse
	}
	result, err := list(c.Context(), uint(courseID), params)
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
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid module ID")
	}

	// B140: F-013 parent-tie (the module must live in :course_id) plus
	// deleted/unpublished filtering. Pre-fix any enrolled user could read
	// a same-tenant module of another course, deleted items included.
	module, wrote := h.loadModuleInCourse(c, uint(id), uint(courseID))
	if wrote {
		return nil
	}
	module.Items = visibleItems(module.Items, canViewUnpublishedContent(c))

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
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid module ID")
	}

	module, err := h.moduleService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "module")
	}

	// F-013 parent-tie: same shape as DeleteModule. A module loaded
	// under the caller's tenant must still belong to the URL's
	// :course_id, otherwise a teacher in course A could PUT a module
	// from course B in the same tenant.
	if module.CourseID != uint(courseID) {
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
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid module ID")
	}

	// F-011 / F-013: load under the caller's tenant + verify the
	// module belongs to the URL's :course_id before the destructive op.
	module, err := h.moduleService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "module")
	}
	if module.CourseID != uint(courseID) {
		return responses.NotFound(c, "module")
	}

	if err := h.moduleService.Delete(c.Context(), uint(id)); err != nil {
		return responses.InternalError(c, "Could not delete module")
	}

	return c.JSON(fiber.Map{"delete": true})
}

func (h *ModuleHandler) GetPrerequisites(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid module ID")
	}
	// B140: the prerequisite repo is keyed by module_id alone, so the
	// module must be tied to :course_id (and the tenant) first.
	if _, wrote := h.loadModuleInCourse(c, uint(id), uint(courseID)); wrote {
		return nil
	}

	prereqs, err := h.moduleService.GetPrerequisites(c.Context(), uint(id))
	if err != nil {
		return responses.InternalError(c, "Could not get prerequisites")
	}
	if prereqs == nil {
		prereqs = []uint{}
	}

	return c.JSON(fiber.Map{"module_id": id, "prerequisite_module_ids": prereqs})
}

func (h *ModuleHandler) SetPrerequisites(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid module ID")
	}
	// B140: tie the target module to :course_id + tenant before the
	// destructive replace.
	if _, wrote := h.loadModuleInCourse(c, uint(id), uint(courseID)); wrote {
		return nil
	}

	var input struct {
		PrerequisiteModuleIDs []uint `json:"prerequisite_module_ids"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	// Every prerequisite must be another live module of the same course
	// (Canvas only accepts context_module prerequisites from the module's
	// own course). Duplicates are collapsed.
	seen := make(map[uint]bool, len(input.PrerequisiteModuleIDs))
	prereqIDs := make([]uint, 0, len(input.PrerequisiteModuleIDs))
	for _, pid := range input.PrerequisiteModuleIDs {
		if seen[pid] {
			continue
		}
		seen[pid] = true
		if pid == uint(id) {
			return responses.BadRequest(c, "A module cannot be its own prerequisite")
		}
		pre, err := h.moduleService.GetByID(c.Context(), pid, callerAccountID(c))
		if err != nil || pre == nil || pre.CourseID != uint(courseID) || pre.WorkflowState == "deleted" {
			return responses.BadRequest(c, "Prerequisite modules must belong to this course")
		}
		prereqIDs = append(prereqIDs, pid)
	}

	if err := h.moduleService.SetPrerequisites(c.Context(), uint(id), prereqIDs); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{"module_id": id, "prerequisite_module_ids": prereqIDs})
}
