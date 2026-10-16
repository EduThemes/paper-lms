package repository

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// ErrColourSchemeLimit is returned by DesignColourSchemeRepository.Create
// when the user already has the maximum number of saved schemes.
var ErrColourSchemeLimit = errors.New("colour scheme limit reached")

// DesignColourSchemeRepository stores a teacher's saved Page Designer
// palettes. Every read and write is scoped to the owning user AND their
// account: another user's id (or another tenant's) is simply not found.
type DesignColourSchemeRepository interface {
	ListByUser(ctx context.Context, userID, accountID uint) ([]models.DesignColourScheme, error)
	FindByID(ctx context.Context, id, userID, accountID uint) (*models.DesignColourScheme, error)
	// Create inserts the scheme unless the user already has limit schemes
	// (ErrColourSchemeLimit). The count and insert are serialized per user.
	Create(ctx context.Context, scheme *models.DesignColourScheme, limit int) error
	// Update saves name and palette of a scheme the user owns.
	Update(ctx context.Context, scheme *models.DesignColourScheme) error
	Delete(ctx context.Context, id, userID, accountID uint) error
}
