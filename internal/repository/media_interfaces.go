package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// MediaObjectRepository persists rich-content audio/video (G002). Every
// read is tenant-scoped by a trailing accountID (0 = auth-internal only).
type MediaObjectRepository interface {
	Create(ctx context.Context, m *models.MediaObject) error
	// FindByMediaID returns an active media object of the tenant.
	FindByMediaID(ctx context.Context, mediaID string, accountID uint) (*models.MediaObject, error)
	Update(ctx context.Context, m *models.MediaObject) error
	// ListByUser lists a user's active media objects, newest first.
	ListByUser(ctx context.Context, userID, accountID uint, params PaginationParams) (*PaginatedResult[models.MediaObject], error)
}

// MediaTrackRepository persists caption/subtitle tracks of media objects.
type MediaTrackRepository interface {
	ListByMediaObject(ctx context.Context, mediaObjectID, accountID uint) ([]models.MediaTrack, error)
	FindByID(ctx context.Context, id, accountID uint) (*models.MediaTrack, error)
	// Upsert creates the track, or replaces the existing track of the same
	// (media object, locale) — Canvas keeps one track per locale.
	Upsert(ctx context.Context, t *models.MediaTrack) error
	Delete(ctx context.Context, id, accountID uint) error
}
