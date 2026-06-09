package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type ModuleItemHandler struct {
	moduleService *service.ModuleService
	pageService   *service.PageService
}

func NewModuleItemHandler(moduleService *service.ModuleService, pageService *service.PageService) *ModuleItemHandler {
	return &ModuleItemHandler{moduleService: moduleService, pageService: pageService}
}

// requireModuleInCourse verifies moduleID names a module that lives in
// courseID and the caller's tenant (ModuleService.GetByID is tenant-
// scoped). Returns wrote=true (404 written) on any miss so the caller
// short-circuits with `return nil`. Closes the nested-route IDOR — the
// route middleware only guards :course_id.
func (h *ModuleItemHandler) requireModuleInCourse(c *fiber.Ctx, moduleID, courseID uint) bool {
	module, err := h.moduleService.GetByID(c.Context(), moduleID, callerAccountID(c))
	if err != nil || module == nil || module.CourseID != courseID {
		_ = responses.NotFound(c, "module")
		return true
	}
	return false
}

// scopedModuleItem loads the :item_id item and verifies it belongs to
// :module_id, which must itself live in :course_id and the caller's
// tenant. Returns wrote=true (404 written) on any miss.
func (h *ModuleItemHandler) scopedModuleItem(c *fiber.Ctx) (*models.ContentTag, bool) {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid course ID")
		return nil, true
	}
	moduleID, err := c.ParamsInt("module_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid module ID")
		return nil, true
	}
	itemID, err := c.ParamsInt("item_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid item ID")
		return nil, true
	}
	if h.requireModuleInCourse(c, uint(moduleID), uint(courseID)) {
		return nil, true
	}
	item, err := h.moduleService.GetItem(c.Context(), uint(itemID))
	if err != nil || item.ContextModuleID != uint(moduleID) {
		_ = responses.NotFound(c, "module item")
		return nil, true
	}
	return item, false
}

func (h *ModuleItemHandler) ListModuleItems(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	moduleID, err := c.ParamsInt("module_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid module ID")
	}
	if h.requireModuleInCourse(c, uint(moduleID), uint(courseID)) {
		return nil
	}

	params := middleware.GetPagination(c)

	result, err := h.moduleService.ListItems(c.Context(), uint(moduleID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch module items")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	items := make([]fiber.Map, len(result.Items))
	for i, item := range result.Items {
		items[i] = moduleItemToJSON(&item)
	}

	return c.JSON(items)
}

func (h *ModuleItemHandler) GetModuleItem(c *fiber.Ctx) error {
	item, wrote := h.scopedModuleItem(c)
	if wrote {
		return nil
	}

	return c.JSON(moduleItemToJSON(item))
}

func (h *ModuleItemHandler) CreateModuleItem(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	moduleID, err := c.ParamsInt("module_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid module ID")
	}
	if h.requireModuleInCourse(c, uint(moduleID), uint(courseID)) {
		return nil
	}

	var input struct {
		ModuleItem struct {
			Title       string `json:"title"`
			Type        string `json:"type"` // Page, Assignment, Quiz, ExternalUrl, SubHeader, Discussion
			ContentID   *uint  `json:"content_id"`
			PageURL     string `json:"page_url"`
			ExternalURL string `json:"external_url"`
			NewTab      bool   `json:"new_tab"`
			Position    int    `json:"position"`
			Indent      int    `json:"indent"`
		} `json:"module_item"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	contentType := itemTypeToContentType(input.ModuleItem.Type)

	// For Page items: resolve page_url to content_id, or auto-create a page
	var pageSlug string
	if input.ModuleItem.Type == "Page" && h.pageService != nil {
		if input.ModuleItem.ContentID != nil {
			// Look up existing page to get its slug
			if page, err := h.pageService.GetByID(c.Context(), *input.ModuleItem.ContentID, callerAccountID(c)); err == nil {
				pageSlug = page.URL
			}
		} else if input.ModuleItem.PageURL != "" {
			// Look up existing page by URL slug
			page, err := h.pageService.GetByURL(c.Context(), uint(courseID), input.ModuleItem.PageURL)
			if err == nil && page != nil {
				input.ModuleItem.ContentID = &page.ID
				pageSlug = page.URL
			}
		}

		// If still no content_id, auto-create a new page
		if input.ModuleItem.ContentID == nil && input.ModuleItem.Title != "" {
			newPage := &models.WikiPage{
				CourseID:      uint(courseID),
				Title:         input.ModuleItem.Title,
				Body:          "",
				WorkflowState: "active",
			}
			if err := h.pageService.Create(c.Context(), newPage); err == nil {
				input.ModuleItem.ContentID = &newPage.ID
				pageSlug = newPage.URL
			}
		}
	}

	itemURL := input.ModuleItem.ExternalURL
	if input.ModuleItem.Type == "Page" && pageSlug != "" {
		itemURL = pageSlug
	}

	item := &models.ContentTag{
		ContextModuleID: uint(moduleID),
		Title:           input.ModuleItem.Title,
		ContentType:     contentType,
		ContentID:       input.ModuleItem.ContentID,
		URL:             itemURL,
		NewTab:          input.ModuleItem.NewTab,
		Position:        input.ModuleItem.Position,
		Indent:          input.ModuleItem.Indent,
		WorkflowState:   models.ContentTagActive,
	}

	if err := h.moduleService.CreateItem(c.Context(), item); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(moduleItemToJSON(item))
}

func (h *ModuleItemHandler) UpdateModuleItem(c *fiber.Ctx) error {
	item, wrote := h.scopedModuleItem(c)
	if wrote {
		return nil
	}

	var input struct {
		ModuleItem struct {
			Title     *string `json:"title"`
			Position  *int    `json:"position"`
			Indent    *int    `json:"indent"`
			Published *bool   `json:"published"`
		} `json:"module_item"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.ModuleItem.Title != nil {
		item.Title = *input.ModuleItem.Title
	}
	if input.ModuleItem.Position != nil {
		item.Position = *input.ModuleItem.Position
	}
	if input.ModuleItem.Indent != nil {
		item.Indent = *input.ModuleItem.Indent
	}
	if input.ModuleItem.Published != nil {
		if *input.ModuleItem.Published {
			item.WorkflowState = "active"
		} else {
			item.WorkflowState = "unpublished"
		}
	}

	if err := h.moduleService.UpdateItem(c.Context(), item); err != nil {
		return responses.InternalError(c, "Could not update module item")
	}

	return c.JSON(moduleItemToJSON(item))
}

func (h *ModuleItemHandler) DeleteModuleItem(c *fiber.Ctx) error {
	item, wrote := h.scopedModuleItem(c)
	if wrote {
		return nil
	}

	if err := h.moduleService.DeleteItem(c.Context(), item.ID); err != nil {
		return responses.InternalError(c, "Could not delete module item")
	}

	return c.JSON(fiber.Map{"delete": true})
}

func (h *ModuleItemHandler) ReorderItems(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	moduleID, err := c.ParamsInt("module_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid module ID")
	}
	if h.requireModuleInCourse(c, uint(moduleID), uint(courseID)) {
		return nil
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

	if err := h.moduleService.ReorderItems(c.Context(), uint(moduleID), input.Order); err != nil {
		return responses.InternalError(c, "Could not reorder items")
	}

	return c.JSON(fiber.Map{"reorder": true})
}

func (h *ModuleItemHandler) MoveItem(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	// Verifies the source item belongs to :module_id in :course_id and
	// the caller's tenant.
	item, wrote := h.scopedModuleItem(c)
	if wrote {
		return nil
	}

	var input struct {
		ModuleID uint `json:"module_id"`
		Position int  `json:"position"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.ModuleID == 0 {
		return responses.BadRequest(c, "Target module_id is required")
	}
	// The destination module must also live in the same course/tenant —
	// otherwise an item could be moved into another tenant's module.
	if h.requireModuleInCourse(c, input.ModuleID, uint(courseID)) {
		return nil
	}

	if err := h.moduleService.MoveItemToModule(c.Context(), item.ID, input.ModuleID, input.Position); err != nil {
		return responses.InternalError(c, "Could not move item")
	}

	return c.JSON(fiber.Map{"moved": true})
}

func itemTypeToContentType(itemType string) string {
	switch itemType {
	case "Page":
		return "WikiPage"
	case "Assignment":
		return "Assignment"
	case "Quiz":
		return "Quizzes::Quiz"
	case "Discussion":
		return "DiscussionTopic"
	case "ExternalUrl":
		return "ExternalUrl"
	case "SubHeader":
		return "ContextModuleSubHeader"
	default:
		return itemType
	}
}
