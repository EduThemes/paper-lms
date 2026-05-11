package models

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// EmbeddingDim is the fixed dimensionality of stored embedding vectors.
// 384 mirrors common open-source sentence-embedding sizes (e.g. MiniLM)
// and lets us swap the local hashing embedder for a real model later
// without changing the schema.
const EmbeddingDim = 384

// Vector is a fixed-length float32 slice persisted in PostgreSQL using
// the pgvector text format ("[1,2,3]"). We avoid taking a dependency on
// github.com/pgvector/pgvector-go by implementing Scan/Value directly —
// the wire format is identical and the column type is `vector(N)` when
// the pgvector extension is installed. If the extension is missing the
// migration falls back to a `TEXT` column and Scan/Value still work
// because we use the same bracketed text representation in both cases.
type Vector []float32

// Value implements driver.Valuer — emits "[v1,v2,...]".
func (v Vector) Value() (driver.Value, error) {
	if v == nil {
		return nil, nil
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(f), 'f', -1, 32))
	}
	b.WriteByte(']')
	return b.String(), nil
}

// Scan implements sql.Scanner — accepts string or []byte in pgvector text format.
func (v *Vector) Scan(src interface{}) error {
	if src == nil {
		*v = nil
		return nil
	}
	var s string
	switch x := src.(type) {
	case string:
		s = x
	case []byte:
		s = string(x)
	default:
		return fmt.Errorf("Vector.Scan: unsupported type %T", src)
	}
	s = strings.TrimSpace(s)
	if s == "" {
		*v = nil
		return nil
	}
	if s[0] != '[' || s[len(s)-1] != ']' {
		return errors.New("Vector.Scan: expected bracketed pgvector text")
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	if inner == "" {
		*v = Vector{}
		return nil
	}
	parts := strings.Split(inner, ",")
	out := make(Vector, len(parts))
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
		if err != nil {
			return fmt.Errorf("Vector.Scan: bad component %q: %w", p, err)
		}
		out[i] = float32(f)
	}
	*v = out
	return nil
}

// ContentEmbedding stores a vector representation of a piece of course
// content for cosine-similarity smart search. One row per
// (content_type, content_id). The embedding column type is
// `vector(384)` when the pgvector extension is installed; otherwise the
// migration falls back to TEXT and the same text encoding round-trips.
type ContentEmbedding struct {
	ID          uint   `json:"id" gorm:"primaryKey"`
	CourseID    uint   `json:"course_id" gorm:"not null;index"`
	ContentType string `json:"content_type" gorm:"not null;size:32;index:idx_content_embeddings_type_id"`
	ContentID   uint   `json:"content_id" gorm:"not null;index:idx_content_embeddings_type_id"`
	Title       string `json:"title" gorm:"type:text"`
	Excerpt     string `json:"excerpt" gorm:"type:text"`
	// Embedding is the dense vector. Use type:vector(384) when pgvector
	// is installed; the migration creates the column with the right type.
	Embedding Vector    `json:"-" gorm:"type:vector(384)"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName pins the table name (avoids GORM pluralization surprises).
func (ContentEmbedding) TableName() string { return "content_embeddings" }
