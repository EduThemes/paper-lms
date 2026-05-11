package postgres

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type contentEmbeddingRepo struct{ db *gorm.DB }

// NewContentEmbeddingRepository constructs a Postgres-backed ContentEmbeddingRepository.
func NewContentEmbeddingRepository(db *gorm.DB) *contentEmbeddingRepo {
	return &contentEmbeddingRepo{db: db}
}

func (r *contentEmbeddingRepo) Upsert(ctx context.Context, e *models.ContentEmbedding) error {
	if e == nil {
		return errors.New("nil embedding")
	}
	// Replace by (content_type, content_id). We do this as delete-then-insert
	// rather than ON CONFLICT to avoid needing a Postgres unique constraint
	// on (content_type, content_id) — useful when pgvector isn't installed
	// and the column type fell back to TEXT.
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("content_type = ? AND content_id = ?", e.ContentType, e.ContentID).
			Delete(&models.ContentEmbedding{}).Error; err != nil {
			return err
		}
		e.ID = 0
		return tx.Create(e).Error
	})
}

func (r *contentEmbeddingRepo) DeleteByContent(ctx context.Context, contentType string, contentID uint) error {
	return r.db.WithContext(ctx).
		Where("content_type = ? AND content_id = ?", contentType, contentID).
		Delete(&models.ContentEmbedding{}).Error
}

func (r *contentEmbeddingRepo) SearchByCourse(ctx context.Context, courseID uint, queryVec []float32, limit int) ([]repository.SearchHit, error) {
	if limit <= 0 {
		limit = 10
	}
	if len(queryVec) == 0 {
		return []repository.SearchHit{}, nil
	}

	// Try the pgvector cosine-distance operator first. If pgvector is not
	// installed the column was created as TEXT and `<=>` won't exist —
	// catch that error and fall back to in-Go cosine ranking.
	q := models.Vector(queryVec)
	qval, _ := q.Value()

	type row struct {
		ID          uint
		CourseID    uint
		ContentType string
		ContentID   uint
		Title       string
		Excerpt     string
		Distance    float64
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Raw(`SELECT id, course_id, content_type, content_id, title, excerpt,
		            (embedding <=> ?::vector) AS distance
		     FROM content_embeddings
		     WHERE course_id = ?
		     ORDER BY distance ASC
		     LIMIT ?`, qval, courseID, limit).
		Scan(&rows).Error
	if err == nil && len(rows) > 0 {
		hits := make([]repository.SearchHit, len(rows))
		for i, r := range rows {
			// pgvector cosine distance is 1 - cosine_similarity, so similarity = 1 - distance.
			hits[i] = repository.SearchHit{
				ID:          r.ID,
				CourseID:    r.CourseID,
				ContentType: r.ContentType,
				ContentID:   r.ContentID,
				Title:       r.Title,
				Excerpt:     r.Excerpt,
				Score:       float32(1.0 - r.Distance),
			}
		}
		return hits, nil
	}
	// Fall back to in-process ranking if pgvector / `<=>` is unavailable
	// or the raw query produced no rows due to an operator error.
	if err != nil && !isPgvectorMissing(err) {
		// A real DB error — surface it.
		return nil, err
	}
	return r.searchByCourseInGo(ctx, courseID, queryVec, limit)
}

func (r *contentEmbeddingRepo) ListByCourse(ctx context.Context, courseID uint) ([]models.ContentEmbedding, error) {
	var items []models.ContentEmbedding
	if err := r.db.WithContext(ctx).
		Where("course_id = ?", courseID).
		Order("id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// isPgvectorMissing returns true when the error looks like pgvector isn't installed.
func isPgvectorMissing(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "operator does not exist") ||
		strings.Contains(msg, "type \"vector\"") ||
		strings.Contains(msg, "type vector") ||
		strings.Contains(msg, "extension \"vector\"")
}

// searchByCourseInGo loads all embeddings for a course and ranks them in
// memory. Used when the pgvector extension isn't available. Fine for the
// hashing-embedder MVP; swap to a server-side ANN index when scaling.
func (r *contentEmbeddingRepo) searchByCourseInGo(ctx context.Context, courseID uint, queryVec []float32, limit int) ([]repository.SearchHit, error) {
	var items []models.ContentEmbedding
	if err := r.db.WithContext(ctx).
		Where("course_id = ?", courseID).
		Find(&items).Error; err != nil {
		return nil, err
	}
	hits := make([]repository.SearchHit, 0, len(items))
	for _, it := range items {
		score := cosineSim(queryVec, []float32(it.Embedding))
		hits = append(hits, repository.SearchHit{
			ID:          it.ID,
			CourseID:    it.CourseID,
			ContentType: it.ContentType,
			ContentID:   it.ContentID,
			Title:       it.Title,
			Excerpt:     it.Excerpt,
			Score:       score,
		})
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

func cosineSim(a, b []float32) float32 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		fa, fb := float64(a[i]), float64(b[i])
		dot += fa * fb
		na += fa * fa
		nb += fb * fb
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(na) * math.Sqrt(nb)))
}
