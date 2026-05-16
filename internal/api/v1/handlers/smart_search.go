package handlers

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

// ReindexSourceLister is implemented by anything that can enumerate the
// indexable content for a course. Wired in main.go from the existing
// announcement / assignment / page / discussion repos. Kept narrow so
// the handler doesn't depend on every content repo concretely.
type ReindexSourceLister interface {
	ListIndexable(ctx context.Context, courseID uint) ([]ReindexableItem, error)
}

// ReindexableItem is the minimal payload the smart-search service needs
// to (re)build an embedding for one piece of content.
type ReindexableItem struct {
	ContentType string
	ContentID   uint
	Title       string
	Body        string
}

// SmartSearchHandler exposes /smart_search and /smart_search/reindex.
type SmartSearchHandler struct {
	search *service.SmartSearchService
	// Sources is optional — when nil the Reindex endpoint returns 501.
	// The integration agent wires this in main.go once the per-content
	// adapters exist; the search endpoint works independently.
	Sources ReindexSourceLister
}

// NewSmartSearchHandler constructs the handler. Sources may be nil.
func NewSmartSearchHandler(search *service.SmartSearchService, sources ReindexSourceLister) *SmartSearchHandler {
	return &SmartSearchHandler{search: search, Sources: sources}
}

// Search GET /api/v1/courses/:course_id/smart_search?q=...&limit=10
func (h *SmartSearchHandler) Search(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}
	q := c.Query("q")
	limit := c.QueryInt("limit", 10)
	results, err := h.search.Search(c.Context(), uint(courseID), callerAccountID(c), q, limit)
	if err != nil {
		return responses.InternalError(c, "Smart search failed")
	}
	return c.JSON(fiber.Map{
		"query":   q,
		"results": results,
	})
}

// Reindex POST /api/v1/courses/:course_id/smart_search/reindex
// Admin/instructor-gated by router middleware. Re-embeds every piece of
// indexable course content. Synchronous for now — fine for K-12 course
// sizes; move to a background job if total content exceeds a few thousand items.
func (h *SmartSearchHandler) Reindex(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}
	if h.Sources == nil {
		return responses.Error(c, fiber.StatusNotImplemented, "Reindex sources not wired")
	}
	items, err := h.Sources.ListIndexable(c.Context(), uint(courseID))
	if err != nil {
		return responses.InternalError(c, "Could not enumerate course content")
	}
	indexed := 0
	for _, it := range items {
		if err := h.search.IndexContent(c.Context(), uint(courseID), it.ContentType, it.ContentID, it.Title, it.Body); err != nil {
			continue // best-effort; one bad item shouldn't fail the whole job
		}
		indexed++
	}
	return c.JSON(fiber.Map{
		"course_id": uint(courseID),
		"indexed":   indexed,
		"total":     len(items),
	})
}

// Compile-time check that the search service contract is satisfied.
var _ repository.ContentEmbeddingRepository = (repository.ContentEmbeddingRepository)(nil)
