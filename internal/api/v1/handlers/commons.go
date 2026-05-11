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

// callerAccountID returns the caller's account scope. Defaults to 1
// (the root account) so this works in single-tenant setups; once a real
// account_id is exposed via auth context we will read it from there.
func callerAccountID(c *fiber.Ctx) uint {
	if v, ok := c.Locals("account_id").(uint); ok && v != 0 {
		return v
	}
	return 1
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
	item, err := h.commonsService.Get(c.Context(), uint(id))
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

	result, err := h.commonsService.Import(c.Context(), userID, uint(courseID), uint(id))
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
	favorited, err := h.commonsService.ToggleFavorite(c.Context(), userID, uint(id))
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
