package service

import (
	"context"
	"errors"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// SmartSearchService indexes course content and answers similarity queries.
// The Embedder is injected so production deployments can swap the
// dependency-free HashingEmbedder for a semantic model.
type SmartSearchService struct {
	embeddings repository.ContentEmbeddingRepository
	embedder   Embedder
}

// NewSmartSearchService constructs a SmartSearchService. embedder.Dim()
// must match the persisted vector dimension (models.EmbeddingDim) — the
// constructor returns an error if it doesn't, since silently re-indexing
// a different dimension would corrupt cosine search.
func NewSmartSearchService(embeddings repository.ContentEmbeddingRepository, embedder Embedder) (*SmartSearchService, error) {
	if embeddings == nil {
		return nil, errors.New("smart_search: nil embeddings repo")
	}
	if embedder == nil {
		return nil, errors.New("smart_search: nil embedder")
	}
	if embedder.Dim() != models.EmbeddingDim {
		return nil, errors.New("smart_search: embedder dimension does not match models.EmbeddingDim")
	}
	return &SmartSearchService{embeddings: embeddings, embedder: embedder}, nil
}

// SmartSearchResult is the API-facing search result returned to callers.
type SmartSearchResult struct {
	ContentType string  `json:"content_type"`
	ContentID   uint    `json:"content_id"`
	Title       string  `json:"title"`
	Excerpt     string  `json:"excerpt"`
	Score       float32 `json:"score"`
}

// IndexContent (re)indexes one piece of content. body is rich text — we
// strip HTML naively before embedding and store a truncated excerpt for
// display in search results.
func (s *SmartSearchService) IndexContent(ctx context.Context, courseID uint, contentType string, contentID uint, title, body string) error {
	if courseID == 0 || contentID == 0 || contentType == "" {
		return errors.New("smart_search: invalid index target")
	}
	plain := stripHTML(body)
	combined := strings.TrimSpace(title + "\n" + plain)
	vec, err := s.embedder.Embed(combined)
	if err != nil {
		return err
	}
	excerpt := plain
	if len(excerpt) > 240 {
		excerpt = excerpt[:240] + "…"
	}
	return s.embeddings.Upsert(ctx, &models.ContentEmbedding{
		CourseID:    courseID,
		ContentType: contentType,
		ContentID:   contentID,
		Title:       title,
		Excerpt:     excerpt,
		Embedding:   models.Vector(vec),
	})
}

// RemoveContent unindexes a piece of content (call on hard or soft delete).
func (s *SmartSearchService) RemoveContent(ctx context.Context, contentType string, contentID uint) error {
	return s.embeddings.DeleteByContent(ctx, contentType, contentID)
}

// Search returns up to `limit` results ranked by cosine similarity.
func (s *SmartSearchService) Search(ctx context.Context, courseID uint, query string, limit int) ([]SmartSearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []SmartSearchResult{}, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	vec, err := s.embedder.Embed(query)
	if err != nil {
		return nil, err
	}
	hits, err := s.embeddings.SearchByCourse(ctx, courseID, vec, limit)
	if err != nil {
		return nil, err
	}
	out := make([]SmartSearchResult, len(hits))
	for i, h := range hits {
		out[i] = SmartSearchResult{
			ContentType: h.ContentType,
			ContentID:   h.ContentID,
			Title:       h.Title,
			Excerpt:     h.Excerpt,
			Score:       h.Score,
		}
	}
	return out, nil
}

// stripHTML is a tiny tag remover — enough for the hashing embedder.
// We avoid pulling bluemonday here because we don't need sanitization;
// we just want plain text for tokenization.
func stripHTML(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			b.WriteByte(' ')
		case !inTag:
			b.WriteRune(r)
		}
	}
	// Collapse whitespace.
	return strings.Join(strings.Fields(b.String()), " ")
}
