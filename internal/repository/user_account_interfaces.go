package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	FindByID(ctx context.Context, id uint) (*models.User, error)
	FindByLoginID(ctx context.Context, loginID string) (*models.User, error)
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	FindBySISUserID(ctx context.Context, sisUserID string) (*models.User, error)
	FindByIDs(ctx context.Context, ids []uint) ([]models.User, error)
	Update(ctx context.Context, user *models.User) error
	List(ctx context.Context, params PaginationParams) (*PaginatedResult[models.User], error)
	FindByResetToken(ctx context.Context, token string) (*models.User, error)
	Search(ctx context.Context, searchTerm string, accountID uint, params PaginationParams) (*PaginatedResult[models.User], error)
	// FilterPublicLeaderboardCandidates returns the subset of `candidateIDs`
	// that have NOT opted out of public leaderboards (W2-C). Used by any
	// leaderboard query path before projection. Stacks with the data-access
	// FERPA block on `mastery_points` — opt-out is the per-learner privacy
	// control, FERPA is the field-classification control; both must allow
	// for a row to surface on a public board. Ships in W2-C so Wave 3's
	// leaderboard primitives don't retrofit the privacy guard later.
	FilterPublicLeaderboardCandidates(ctx context.Context, candidateIDs []uint) ([]uint, error)
}

type AccountRepository interface {
	Create(ctx context.Context, account *models.Account) error
	FindByID(ctx context.Context, id uint) (*models.Account, error)
	Update(ctx context.Context, account *models.Account) error
	List(ctx context.Context, params PaginationParams) (*PaginatedResult[models.Account], error)
}
