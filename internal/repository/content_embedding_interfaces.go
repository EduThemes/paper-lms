package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// SearchHit is a single ranked smart-search result.
// Score is cosine similarity in [-1, 1] (higher = more similar).
type SearchHit struct {
	ID          uint    `json:"id"`
	CourseID    uint    `json:"course_id"`
	ContentType string  `json:"content_type"`
	ContentID   uint    `json:"content_id"`
	Title       string  `json:"title"`
	Excerpt     string  `json:"excerpt"`
	Score       float32 `json:"score"`
}

// ContentEmbeddingRepository persists and searches content embeddings.
// Kept in its own interfaces file (per PROJECT.md guidance) so adding it
// does not require editing the shared interfaces.go from a feature agent.
type ContentEmbeddingRepository interface {
	// Upsert replaces any existing row for (ContentType, ContentID).
	Upsert(ctx context.Context, e *models.ContentEmbedding) error
	// DeleteByContent removes the row for a given content reference.
	DeleteByContent(ctx context.Context, contentType string, contentID uint) error
	// SearchByCourse returns the top `limit` most-similar rows in a course
	// using pgvector's cosine-distance operator (`<=>`). Falls back to
	// in-process cosine ranking when pgvector is not installed.
	SearchByCourse(ctx context.Context, courseID uint, queryVec []float32, limit int) ([]SearchHit, error)
	// ListByCourse is used for full re-index loops over an existing index.
	ListByCourse(ctx context.Context, courseID uint) ([]models.ContentEmbedding, error)
}
