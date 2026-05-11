package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
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
			DefaultRestrictions    *string `json:"default_restrictions"`
			UseDefaultRestrictions *bool   `json:"use_default_restrictions"`
		} `json:"blueprint_template"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.BlueprintTemplate.DefaultRestrictions != nil {
		template.DefaultRestrictions = *input.BlueprintTemplate.DefaultRestrictions
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

	// Build the final set: current + add - remove
	params := middleware.GetPagination(c)
	existing, err := h.blueprintService.ListAssociatedCourses(c.Context(), template.ID, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch current associations")
	}

	courseSet := make(map[uint]bool)
	for _, sub := range existing.Items {
		courseSet[sub.ChildCourseID] = true
	}
	for _, id := range input.CourseIDsToAdd {
		courseSet[id] = true
	}
	for _, id := range input.CourseIDsToRemove {
		delete(courseSet, id)
	}

	finalIDs := make([]uint, 0, len(courseSet))
	for id := range courseSet {
		finalIDs = append(finalIDs, id)
	}

	if err := h.blueprintService.UpdateAssociations(c.Context(), template.ID, finalIDs); err != nil {
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
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	userID, _ := c.Locals("user_id").(uint)

	migration, err := h.blueprintService.TriggerSync(c.Context(), template.ID, userID, input.Comment)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(blueprintMigrationToJSON(migration))
}

// GetMigration returns a specific migration.
// GET /api/v1/courses/:course_id/blueprint_templates/default/migrations/:migration_id
func (h *BlueprintHandler) GetMigration(c *fiber.Ctx) error {
	migrationID, err := c.ParamsInt("migration_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid migration ID")
	}

	migration, err := h.blueprintService.GetMigration(c.Context(), uint(migrationID))
	if err != nil {
		return responses.NotFound(c, "blueprint migration")
	}

	return c.JSON(blueprintMigrationToJSON(migration))
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
	subscriptionID, err := c.ParamsInt("subscription_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid subscription ID")
	}

	params := middleware.GetPagination(c)
	result, err := h.blueprintService.ListSubscriptionMigrations(c.Context(), uint(subscriptionID), params)
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
	migrationID, err := c.ParamsInt("migration_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid migration ID")
	}

	migration, err := h.blueprintService.GetMigration(c.Context(), uint(migrationID))
	if err != nil {
		return responses.NotFound(c, "blueprint migration")
	}

	return c.JSON(blueprintMigrationToJSON(migration))
}
