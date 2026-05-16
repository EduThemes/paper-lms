package handlers

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

// CommonsHandler exposes the Commons content library — a Canvas-Commons
// equivalent for district-wide template sharing.
type CommonsHandler struct {
	commonsService *service.CommonsService
	courseRepo     repository.CourseRepository
}

func NewCommonsHandler(commonsService *service.CommonsService, courseRepo repository.CourseRepository) *CommonsHandler {
	return &CommonsHandler{commonsService: commonsService, courseRepo: courseRepo}
}

func sharedContentToJSON(s *models.SharedContent, includeSnapshot bool) fiber.Map {
	var tags []string
	if s.Tags != "" {
		_ = json.Unmarshal([]byte(s.Tags), &tags)
	}
	if tags == nil {
		tags = []string{}
	}
	out := fiber.Map{
		"id":                s.ID,
		"account_id":        s.AccountID,
		"author_user_id":    s.AuthorUserID,
		"title":             s.Title,
		"description":       s.Description,
		"resource_type":     s.ResourceType,
		"source_course_id":  s.SourceCourseID,
		"source_content_id": s.SourceContentID,
		"subject":           s.Subject,
		"grade_level":       s.GradeLevel,
		"tags":              tags,
		"thumbnail_url":     s.ThumbnailURL,
		"download_count":    s.DownloadCount,
		"favorite_count":    s.FavoriteCount,
		"visibility":        s.Visibility,
		"created_at":        s.CreatedAt,
		"updated_at":        s.UpdatedAt,
	}
	if includeSnapshot {
		out["content_snapshot"] = json.RawMessage(s.ContentSnapshot)
	}
	return out
}

// assertSameTenant returns wrote=true if the caller's tenant differs
// from the resource's account_id. On mismatch it writes a 404 (NOT
// 403) — 403 would leak the existence of the resource to a different
// tenant. Per the Fiber `(result, wrote, err)` convention the caller
// short-circuits when wrote=true.
func assertSameTenant(c *fiber.Ctx, resourceAccountID uint) bool {
	if resourceAccountID != callerAccountID(c) {
		_ = responses.NotFound(c, "resource")
		return true
	}
	return false
}

// callerAccountID returns the caller's tenant scope from the JWT claim
// populated by middleware.Protected (13.1.B). The 13.1.C contract:
// every tenant-keyed handler MUST be mounted behind Protected; a
// missing Locals value is a programming error and panics. The 12.1
// recover middleware catches the panic and surfaces a sanitized 500
// to the client.
//
// The previous behavior — returning a hardcoded 1 — silently routed
// every tenant-mismatched read to account 1 and was the load-bearing
// vector behind the audit's "multi-tenancy in disguise" finding.
func callerAccountID(c *fiber.Ctx) uint {
	v, ok := c.Locals("account_id").(uint)
	if !ok || v == 0 {
		panic("callerAccountID: account_id Locals not set — handler mounted without Protected middleware")
	}
	return v
}

// Browse handles GET /api/v1/commons.
// Query params: resource_type, subject, grade_level, q (search), author_user_id, page, per_page.
func (h *CommonsHandler) Browse(c *fiber.Ctx) error {
	accountID := callerAccountID(c)
	params := middleware.GetPagination(c)
	filters := repository.SharedContentFilters{
		ResourceType: c.Query("resource_type"),
		Subject:      c.Query("subject"),
		GradeLevel:   c.Query("grade_level"),
		Search:       c.Query("q"),
	}
	if v := c.QueryInt("author_user_id"); v > 0 {
		filters.AuthorUserID = uint(v)
	}

	result, err := h.commonsService.Browse(c.Context(), accountID, filters, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch commons items")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	items := make([]fiber.Map, len(result.Items))
	for i, it := range result.Items {
		items[i] = sharedContentToJSON(&it, false)
	}
	return c.JSON(items)
}

// Get handles GET /api/v1/commons/:id. Includes the content_snapshot in
// the response so clients can preview before importing.
func (h *CommonsHandler) Get(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid commons ID")
	}
	item, err := h.commonsService.Get(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "commons item")
	}
	return c.JSON(sharedContentToJSON(item, true))
}

// Publish handles POST /api/v1/courses/:course_id/commons/publish.
func (h *CommonsHandler) Publish(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		ResourceType string   `json:"resource_type"`
		ResourceID   uint     `json:"resource_id"`
		Title        string   `json:"title"`
		Description  string   `json:"description"`
		Subject      string   `json:"subject"`
		GradeLevel   string   `json:"grade_level"`
		Tags         []string `json:"tags"`
		ThumbnailURL string   `json:"thumbnail_url"`
		Visibility   string   `json:"visibility"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	userID, _ := c.Locals("user_id").(uint)

	item, err := h.commonsService.Publish(c.Context(), userID, uint(courseID), service.CommonsPublishOptions{
		ResourceType: input.ResourceType,
		ResourceID:   input.ResourceID,
		Title:        input.Title,
		Description:  input.Description,
		Subject:      input.Subject,
		GradeLevel:   input.GradeLevel,
		Tags:         input.Tags,
		ThumbnailURL: input.ThumbnailURL,
		Visibility:   input.Visibility,
	})
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(sharedContentToJSON(item, false))
}

// Import handles POST /api/v1/commons/:id/import?course_id=NN.
func (h *CommonsHandler) Import(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid commons ID")
	}
	courseID := c.QueryInt("course_id")
	if courseID <= 0 {
		// Allow course_id in the JSON body too.
		var body struct {
			CourseID uint `json:"course_id"`
		}
		_ = c.BodyParser(&body)
		courseID = int(body.CourseID)
	}
	if courseID <= 0 {
		return responses.BadRequest(c, "course_id is required")
	}
	userID, _ := c.Locals("user_id").(uint)

	result, err := h.commonsService.Import(c.Context(), userID, uint(courseID), uint(id), callerAccountID(c))
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

// Favorite handles POST /api/v1/commons/:id/favorite (toggle).
func (h *CommonsHandler) Favorite(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid commons ID")
	}
	userID, _ := c.Locals("user_id").(uint)
	if userID == 0 {
		return responses.Unauthorized(c)
	}
	favorited, err := h.commonsService.ToggleFavorite(c.Context(), userID, uint(id), callerAccountID(c))
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.JSON(fiber.Map{"favorited": favorited})
}

// ListFavorites handles GET /api/v1/commons/favorites.
func (h *CommonsHandler) ListFavorites(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(uint)
	if userID == 0 {
		return responses.Unauthorized(c)
	}
	params := middleware.GetPagination(c)
	result, err := h.commonsService.ListFavorites(c.Context(), userID, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch favorites")
	}
	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)
	items := make([]fiber.Map, len(result.Items))
	for i, it := range result.Items {
		items[i] = sharedContentToJSON(&it, false)
	}
	return c.JSON(items)
}
