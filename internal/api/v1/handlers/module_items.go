package handlers

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type ModuleItemHandler struct {
	moduleService *service.ModuleService
	pageService   *service.PageService
	// progression (optional): hides unreleased Mastery Paths items from
	// students and enforces module locks on tool launches.
	progression *service.ModuleProgressionService
}

// SetProgressionService wires student item visibility and lock checks.
func (h *ModuleItemHandler) SetProgressionService(p *service.ModuleProgressionService) {
	h.progression = p
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
	if err != nil || module == nil || module.CourseID != courseID || module.WorkflowState == "deleted" {
		_ = responses.NotFound(c, "module")
		return true
	}
	// B045: an unpublished module is invisible to students/observers.
	if module.WorkflowState != "active" && !canViewUnpublishedContent(c) {
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
	if err != nil || item.ContextModuleID != uint(moduleID) || item.WorkflowState == models.ContentTagDeleted {
		_ = responses.NotFound(c, "module item")
		return nil, true
	}
	// B045: unpublished items are invisible to students/observers.
	if item.WorkflowState != models.ContentTagActive && !canViewUnpublishedContent(c) {
		_ = responses.NotFound(c, "module item")
		return nil, true
	}
	// G080: so are unreleased Mastery Paths assignments.
	visible, err := studentVisibleItems(c, h.progression, uint(courseID), []models.ContentTag{*item})
	if err != nil || len(visible) == 0 {
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

	list := h.moduleService.ListPublishedItems
	if canViewUnpublishedContent(c) {
		list = h.moduleService.ListItems
	}
	result, err := list(c.Context(), uint(moduleID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch module items")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	visible, err := studentVisibleItems(c, h.progression, uint(courseID), result.Items)
	if err != nil {
		return responses.InternalError(c, "Could not fetch module items")
	}
	items := make([]fiber.Map, len(visible))
	for i, item := range visible {
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
			Type        string `json:"type"` // Page, Assignment, Quiz, ExternalUrl, SubHeader, Discussion, File, ExternalTool
			ContentID   *uint  `json:"content_id"`
			PageURL     string `json:"page_url"`
			ExternalURL string `json:"external_url"`
			NewTab      bool   `json:"new_tab"`
			Position    int    `json:"position"`
			Indent      int    `json:"indent"`
			// Optional completion requirement (Canvas completion_requirement).
			CompletionType *string  `json:"completion_type"`
			MinScore       *float64 `json:"min_score"`
		} `json:"module_item"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	contentType := itemTypeToContentType(input.ModuleItem.Type)
	input.ModuleItem.Title = strings.TrimSpace(service.SanitizePlainText(input.ModuleItem.Title))
	switch contentType {
	case "WikiPage", "Assignment", "Quizzes::Quiz", "DiscussionTopic", "ExternalUrl", "ContextModuleSubHeader",
		service.ModuleItemContentAttachment, service.ModuleItemContentExternalTool:
	default:
		return responses.BadRequest(c, "Unsupported module item type")
	}

	// B042: Assignment / Quiz / Discussion items are links to existing
	// content. Canvas requires content_id for them; without it the item
	// is a dead row that pacing, blueprint sync and the student view all
	// skip. The content must also live in this course (a foreign id
	// would otherwise link — and expose — another course's content).
	switch input.ModuleItem.Type {
	case "Assignment", "Quiz", "Discussion":
		if input.ModuleItem.ContentID == nil || *input.ModuleItem.ContentID == 0 {
			return responses.BadRequest(c, "content_id is required for "+input.ModuleItem.Type+" items")
		}
		name, err := h.moduleService.ValidateItemContent(c.Context(), contentType, *input.ModuleItem.ContentID, uint(courseID), callerAccountID(c))
		if err != nil {
			return responses.BadRequest(c, "content_id does not name a "+strings.ToLower(input.ModuleItem.Type)+" in this course")
		}
		if input.ModuleItem.Title == "" {
			input.ModuleItem.Title = name
		}
	}

	// For Page items: resolve page_url to content_id, or auto-create a page
	var pageSlug string
	if input.ModuleItem.Type == "Page" && h.pageService != nil {
		if input.ModuleItem.ContentID != nil {
			// Look up existing page to get its slug — it must be a live
			// page of this course.
			page, err := h.pageService.GetByID(c.Context(), *input.ModuleItem.ContentID, callerAccountID(c))
			if err != nil || page == nil || page.CourseID != uint(courseID) || page.WorkflowState == "deleted" {
				return responses.BadRequest(c, "content_id does not name a page in this course")
			}
			pageSlug = page.URL
			if input.ModuleItem.Title == "" {
				input.ModuleItem.Title = page.Title
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

	// G064: File items link a file of this course; External Tool items
	// link an LTI tool installed in the course (or its account), with an
	// optional launch URL on the tool's domain.
	switch contentType {
	case service.ModuleItemContentAttachment:
		if input.ModuleItem.ContentID == nil || *input.ModuleItem.ContentID == 0 {
			return responses.BadRequest(c, "content_id is required for File items")
		}
		name, err := h.moduleService.ValidateFileItem(c.Context(), *input.ModuleItem.ContentID, uint(courseID), callerAccountID(c))
		if err != nil {
			return responses.BadRequest(c, "content_id does not name a file in this course")
		}
		if input.ModuleItem.Title == "" {
			input.ModuleItem.Title = name
		}
		itemURL = ""
	case service.ModuleItemContentExternalTool:
		if input.ModuleItem.ContentID == nil || *input.ModuleItem.ContentID == 0 {
			return responses.BadRequest(c, "content_id is required for External Tool items")
		}
		tool, err := h.moduleService.ExternalToolForCourse(c.Context(), *input.ModuleItem.ContentID, uint(courseID), callerAccountID(c))
		if err != nil {
			return responses.BadRequest(c, "content_id does not name an external tool available in this course")
		}
		launchURL, err := service.ValidateToolLaunchURL(tool, input.ModuleItem.ExternalURL)
		if err != nil {
			return responses.BadRequest(c, err.Error())
		}
		itemURL = launchURL
		if input.ModuleItem.Title == "" {
			input.ModuleItem.Title = tool.Name
		}
	case "ExternalUrl":
		lower := strings.ToLower(strings.TrimSpace(itemURL))
		if !strings.HasPrefix(lower, "https://") && !strings.HasPrefix(lower, "http://") {
			return responses.BadRequest(c, "external_url must be an absolute http(s) URL")
		}
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
	if input.ModuleItem.CompletionType != nil {
		if msg := applyCompletionRequirement(item, *input.ModuleItem.CompletionType, input.ModuleItem.MinScore); msg != "" {
			return responses.BadRequest(c, msg)
		}
	}

	if err := h.moduleService.CreateItem(c.Context(), item); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(moduleItemToJSON(item))
}

// applyCompletionRequirement validates and sets a completion requirement
// on item (B043). completionType "" clears it. Returns a non-empty
// message when the type is not allowed for the item's content type or
// min_score is missing/invalid; the caller writes the 400.
func applyCompletionRequirement(item *models.ContentTag, completionType string, minScore *float64) string {
	completionType = strings.TrimSpace(completionType)
	if completionType == "" {
		item.CompletionType = ""
		item.MinScore = nil
		return ""
	}
	allowed := false
	for _, t := range models.ValidCompletionTypesFor(item.ContentType) {
		if t == completionType {
			allowed = true
			break
		}
	}
	if !allowed {
		return "completion_type " + completionType + " is not valid for this item"
	}
	if completionType == models.ModuleCompletionMinScore {
		if minScore == nil || *minScore < 0 {
			return "min_score is required for a min_score requirement"
		}
		v := *minScore
		item.MinScore = &v
	} else {
		item.MinScore = nil
	}
	item.CompletionType = completionType
	return ""
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
			// B043: completion requirement. Pre-fix these were silently
			// dropped (200 + optimistic UI, nothing persisted).
			CompletionType *string  `json:"completion_type"`
			MinScore       *float64 `json:"min_score"`
		} `json:"module_item"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.ModuleItem.CompletionType != nil {
		if msg := applyCompletionRequirement(item, *input.ModuleItem.CompletionType, input.ModuleItem.MinScore); msg != "" {
			return responses.BadRequest(c, msg)
		}
	} else if input.ModuleItem.MinScore != nil && item.CompletionType == models.ModuleCompletionMinScore {
		if msg := applyCompletionRequirement(item, item.CompletionType, input.ModuleItem.MinScore); msg != "" {
			return responses.BadRequest(c, msg)
		}
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
	case "File":
		return service.ModuleItemContentAttachment
	case "ExternalTool":
		return service.ModuleItemContentExternalTool
	default:
		return itemType
	}
}

// LaunchParams returns the LTI 1.3 launch parameters for an External
// Tool item; the browser posts them to /api/v1/lti/oidc/login, which
// re-checks the caller, COPPA and the tool install. A locked item cannot
// be launched by a student, and launching counts as viewing it.
// GET /courses/:course_id/modules/:module_id/items/:item_id/launch
func (h *ModuleItemHandler) LaunchParams(c *fiber.Ctx) error {
	item, wrote := h.scopedModuleItem(c)
	if wrote {
		return nil
	}
	courseID, _ := c.ParamsInt("course_id")
	if item.ContentType != service.ModuleItemContentExternalTool {
		return responses.NotFound(c, "external tool item")
	}
	learner := false
	if h.progression != nil {
		var err error
		learner, err = h.progression.IsLearner(c.Context(), callerUserID(c), uint(courseID))
		if err != nil {
			return responses.InternalError(c, "Could not check module access")
		}
		if learner {
			progress, err := h.progression.Evaluate(c.Context(), uint(courseID), callerUserID(c))
			if err != nil {
				return responses.InternalError(c, "Could not check module access")
			}
			if _, ip := progress.Item(item.ID); ip == nil || ip.Locked {
				return responses.Forbidden(c, "This module item is locked")
			}
		}
	}
	if item.ContentID == nil {
		return responses.NotFound(c, "external tool")
	}
	tool, err := h.moduleService.ExternalToolForCourse(c.Context(), *item.ContentID, uint(courseID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "external tool")
	}
	if learner {
		_ = h.progression.RecordContentView(c.Context(), callerUserID(c), []uint{item.ID})
	}
	// launch_path is the course tool launch page (LTI lane, G131): it
	// launches through the item's resource link with a platform-signed
	// hint, and handles LTI 1.1 tools too. The legacy LTI 1.3 login
	// parameters stay for API clients that post /lti/oidc/login directly.
	out := fiber.Map{
		"tool_id":        tool.ID,
		"tool_name":      tool.Name,
		"module_item_id": item.ID,
		"new_tab":        item.NewTab,
		"launch_path":    fmt.Sprintf("/courses/%d/external_tools/%d?module_item_id=%d", courseID, tool.ID, item.ID),
	}
	if launch, lerr := h.moduleService.ExternalToolLaunch(c.Context(), item, uint(courseID), callerAccountID(c)); lerr == nil {
		out["client_id"] = launch.ClientID
		out["target_link_uri"] = launch.TargetLinkURI
		out["lti_message_hint"] = launch.LTIMessageHint
	}
	return c.JSON(out)
}
