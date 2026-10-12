package handlers

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/gofiber/fiber/v2"
)

type BlueprintHandler struct {
	blueprintService *service.BlueprintService
}

func NewBlueprintHandler(blueprintService *service.BlueprintService) *BlueprintHandler {
	return &BlueprintHandler{blueprintService: blueprintService}
}

func blueprintTemplateToJSON(t *models.BlueprintTemplate) fiber.Map {
	return fiber.Map{
		"id":                       t.ID,
		"course_id":                t.CourseID,
		"default_restrictions":     t.DefaultRestrictions,
		"use_default_restrictions": t.UseDefaultRestrictions,
		"workflow_state":           t.WorkflowState,
		"created_at":               t.CreatedAt,
		"updated_at":               t.UpdatedAt,
	}
}

func blueprintSubscriptionToJSON(s *models.BlueprintSubscription) fiber.Map {
	return fiber.Map{
		"id":                    s.ID,
		"blueprint_template_id": s.BlueprintTemplateID,
		"child_course_id":       s.ChildCourseID,
		"workflow_state":        s.WorkflowState,
		"created_at":            s.CreatedAt,
		"updated_at":            s.UpdatedAt,
	}
}

func blueprintMigrationToJSON(m *models.BlueprintMigration) fiber.Map {
	return fiber.Map{
		"id":                    m.ID,
		"blueprint_template_id": m.BlueprintTemplateID,
		"user_id":               m.UserID,
		"workflow_state":        m.WorkflowState,
		"comment":               m.Comment,
		"export_settings":       m.ExportSettings,
		"completed_at":          m.CompletedAt,
		"created_at":            m.CreatedAt,
		"updated_at":            m.UpdatedAt,
	}
}

// ListTemplates returns blueprint templates for a course.
// GET /api/v1/courses/:course_id/blueprint_templates
func (h *BlueprintHandler) ListTemplates(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)
	result, err := h.blueprintService.ListTemplates(c.Context(), uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch blueprint templates")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	templates := make([]fiber.Map, len(result.Items))
	for i, t := range result.Items {
		templates[i] = blueprintTemplateToJSON(&t)
	}

	return c.JSON(templates)
}

// CreateTemplate creates a blueprint template for a course (or returns existing).
// POST /api/v1/courses/:course_id/blueprint_templates
func (h *BlueprintHandler) CreateTemplate(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	template, err := h.blueprintService.GetOrCreateTemplate(c.Context(), uint(courseID))
	if err != nil {
		return responses.InternalError(c, "Could not create blueprint template")
	}

	return c.Status(fiber.StatusCreated).JSON(blueprintTemplateToJSON(template))
}

// GetDefaultTemplate returns the default (first) blueprint template for a course.
// GET /api/v1/courses/:course_id/blueprint_templates/default
func (h *BlueprintHandler) GetDefaultTemplate(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	template, err := h.blueprintService.GetOrCreateTemplate(c.Context(), uint(courseID))
	if err != nil {
		return responses.NotFound(c, "blueprint template")
	}

	return c.JSON(blueprintTemplateToJSON(template))
}

// UpdateDefaultTemplate updates the default blueprint template for a course.
// PUT /api/v1/courses/:course_id/blueprint_templates/default
func (h *BlueprintHandler) UpdateDefaultTemplate(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	template, err := h.blueprintService.GetOrCreateTemplate(c.Context(), uint(courseID))
	if err != nil {
		return responses.NotFound(c, "blueprint template")
	}

	var input struct {
		BlueprintTemplate struct {
			// An object ({"content": true}, as Canvas sends it) or that
			// object JSON-encoded as a string (Paper's settings form).
			DefaultRestrictions    json.RawMessage `json:"default_restrictions"`
			UseDefaultRestrictions *bool           `json:"use_default_restrictions"`
		} `json:"blueprint_template"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if raw := input.BlueprintTemplate.DefaultRestrictions; len(raw) > 0 && string(raw) != "null" {
		var asString string
		if json.Unmarshal(raw, &asString) == nil {
			raw = json.RawMessage(strings.TrimSpace(asString))
			if len(raw) == 0 {
				raw = json.RawMessage("{}")
			}
		}
		var r service.BlueprintRestrictions
		var probe map[string]interface{}
		if json.Unmarshal(raw, &probe) != nil || json.Unmarshal(raw, &r) != nil {
			return responses.BadRequest(c, "default_restrictions must be an object of content, points, due_dates and availability_dates flags")
		}
		template.DefaultRestrictions = r.JSON()
	}
	if input.BlueprintTemplate.UseDefaultRestrictions != nil {
		template.UseDefaultRestrictions = *input.BlueprintTemplate.UseDefaultRestrictions
	}

	if err := h.blueprintService.UpdateTemplate(c.Context(), template); err != nil {
		return responses.InternalError(c, "Could not update blueprint template")
	}

	return c.JSON(blueprintTemplateToJSON(template))
}

// GetAssociatedCourses lists courses associated with the default blueprint template.
// GET /api/v1/courses/:course_id/blueprint_templates/default/associated_courses
func (h *BlueprintHandler) GetAssociatedCourses(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	template, err := h.blueprintService.GetOrCreateTemplate(c.Context(), uint(courseID))
	if err != nil {
		return responses.NotFound(c, "blueprint template")
	}

	params := middleware.GetPagination(c)
	result, err := h.blueprintService.ListAssociatedCourses(c.Context(), template.ID, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch associated courses")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	subscriptions := make([]fiber.Map, len(result.Items))
	for i, s := range result.Items {
		subscriptions[i] = blueprintSubscriptionToJSON(&s)
	}

	return c.JSON(subscriptions)
}

// UpdateAssociations updates the associated courses for the default blueprint template.
// PUT /api/v1/courses/:course_id/blueprint_templates/default/associated_courses
func (h *BlueprintHandler) UpdateAssociations(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	template, err := h.blueprintService.GetOrCreateTemplate(c.Context(), uint(courseID))
	if err != nil {
		return responses.NotFound(c, "blueprint template")
	}

	var input struct {
		CourseIDsToAdd    []uint `json:"course_ids_to_add"`
		CourseIDsToRemove []uint `json:"course_ids_to_remove"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	// B037: every course to add is authorized by the service (same
	// account as the blueprint, caller teaches it or is an admin, not
	// already in another blueprint). B040: the service applies the
	// add/remove deltas to the FULL association set; the pre-fix handler
	// rebuilt the set from one paginated page (default 10) and silently
	// dropped every association past it.
	userID, _ := c.Locals("user_id").(uint)
	isAdmin, _ := c.Locals("is_admin").(bool)
	caller := service.BlueprintCaller{UserID: userID, AccountID: callerAccountID(c), IsAdmin: isAdmin}
	if err := h.blueprintService.ModifyAssociations(c.Context(), template, caller, input.CourseIDsToAdd, input.CourseIDsToRemove); err != nil {
		if errors.Is(err, service.ErrBlueprintInvalidAssociation) {
			return responses.BadRequest(c, err.Error())
		}
		return responses.InternalError(c, "Could not update associations")
	}

	return c.JSON(fiber.Map{"message": "Associations updated"})
}

// ListMigrations lists migrations for the default blueprint template.
// GET /api/v1/courses/:course_id/blueprint_templates/default/migrations
func (h *BlueprintHandler) ListMigrations(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	template, err := h.blueprintService.GetOrCreateTemplate(c.Context(), uint(courseID))
	if err != nil {
		return responses.NotFound(c, "blueprint template")
	}

	params := middleware.GetPagination(c)
	result, err := h.blueprintService.ListMigrations(c.Context(), template.ID, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch migrations")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	migrations := make([]fiber.Map, len(result.Items))
	for i, m := range result.Items {
		migrations[i] = blueprintMigrationToJSON(&m)
	}

	return c.JSON(migrations)
}

// CreateMigration triggers a blueprint sync (creates a migration).
// POST /api/v1/courses/:course_id/blueprint_templates/default/migrations
func (h *BlueprintHandler) CreateMigration(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	template, err := h.blueprintService.GetOrCreateTemplate(c.Context(), uint(courseID))
	if err != nil {
		return responses.NotFound(c, "blueprint template")
	}

	var input struct {
		Comment          string `json:"comment"`
		SendNotification bool   `json:"send_notification"`
		// Canvas: "Include Course Settings".
		CopySettings bool `json:"copy_settings"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	userID, _ := c.Locals("user_id").(uint)

	migration, err := h.blueprintService.TriggerSyncWithOptions(c.Context(), template.ID, userID, input.Comment,
		service.BlueprintSyncOptions{CopySettings: input.CopySettings})
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(blueprintMigrationToJSON(migration))
}

// GetMigration returns a specific migration.
// GET /api/v1/courses/:course_id/blueprint_templates/default/migrations/:migration_id
func (h *BlueprintHandler) GetMigration(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	migrationID, err := c.ParamsInt("migration_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid migration ID")
	}

	template, err := h.blueprintService.GetOrCreateTemplate(c.Context(), uint(courseID))
	if err != nil {
		return responses.NotFound(c, "blueprint migration")
	}
	migration, err := h.blueprintService.GetMigration(c.Context(), uint(migrationID))
	// F-013 parent-tie: the migration must belong to this course's
	// blueprint template.
	if err != nil || migration == nil || migration.BlueprintTemplateID != template.ID {
		return responses.NotFound(c, "blueprint migration")
	}

	return c.JSON(blueprintMigrationToJSON(migration))
}

// requireSubscriptionInCourse verifies :subscription_id is a
// subscription whose child course is the URL's :course_id. Writes a 404
// and returns wrote=true on a miss.
func (h *BlueprintHandler) requireSubscriptionInCourse(c *fiber.Ctx) (*models.BlueprintSubscription, bool) {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid course ID")
		return nil, true
	}
	subscriptionID, err := c.ParamsInt("subscription_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid subscription ID")
		return nil, true
	}
	sub, err := h.blueprintService.GetSubscription(c.Context(), uint(subscriptionID))
	if err != nil || sub == nil || sub.ChildCourseID != uint(courseID) {
		_ = responses.NotFound(c, "blueprint subscription")
		return nil, true
	}
	return sub, false
}

// GetUnsyncedChanges returns unsynced changes for the default template (placeholder).
// GET /api/v1/courses/:course_id/blueprint_templates/default/unsynced_changes
func (h *BlueprintHandler) GetUnsyncedChanges(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	template, err := h.blueprintService.GetOrCreateTemplate(c.Context(), uint(courseID))
	if err != nil {
		return responses.NotFound(c, "blueprint template")
	}

	changes, err := h.blueprintService.GetUnsyncedChanges(c.Context(), template.ID)
	if err != nil {
		return responses.InternalError(c, "Could not fetch unsynced changes")
	}

	return c.JSON(changes)
}

// ListSubscriptions lists blueprint subscriptions for a child course.
// GET /api/v1/courses/:course_id/blueprint_subscriptions
func (h *BlueprintHandler) ListSubscriptions(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)
	result, err := h.blueprintService.ListSubscriptions(c.Context(), uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch subscriptions")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	subscriptions := make([]fiber.Map, len(result.Items))
	for i, s := range result.Items {
		subscriptions[i] = blueprintSubscriptionToJSON(&s)
	}

	return c.JSON(subscriptions)
}

// GetSubscriptionMigrations lists migrations for a specific subscription.
// GET /api/v1/courses/:course_id/blueprint_subscriptions/:subscription_id/migrations
func (h *BlueprintHandler) GetSubscriptionMigrations(c *fiber.Ctx) error {
	sub, wrote := h.requireSubscriptionInCourse(c)
	if wrote {
		return nil
	}

	params := middleware.GetPagination(c)
	result, err := h.blueprintService.ListSubscriptionMigrations(c.Context(), sub.ID, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch subscription migrations")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	migrations := make([]fiber.Map, len(result.Items))
	for i, m := range result.Items {
		migrations[i] = blueprintMigrationToJSON(&m)
	}

	return c.JSON(migrations)
}

// GetSubscriptionMigration returns a specific migration for a subscription.
// GET /api/v1/courses/:course_id/blueprint_subscriptions/:subscription_id/migrations/:migration_id
func (h *BlueprintHandler) GetSubscriptionMigration(c *fiber.Ctx) error {
	sub, wrote := h.requireSubscriptionInCourse(c)
	if wrote {
		return nil
	}
	migrationID, err := c.ParamsInt("migration_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid migration ID")
	}

	migration, err := h.blueprintService.GetMigration(c.Context(), uint(migrationID))
	if err != nil || migration == nil || migration.BlueprintTemplateID != sub.BlueprintTemplateID {
		return responses.NotFound(c, "blueprint migration")
	}

	return c.JSON(blueprintMigrationToJSON(migration))
}
