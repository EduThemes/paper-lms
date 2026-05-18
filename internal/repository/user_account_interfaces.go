package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// UserRepository is the data-access surface for users.
//
// Tenant-scope boundary (Sprint 2.3 widening, follow-up to PR #34):
//   - FindByID / FindByIDs / List / Search accept `accountID uint`.
//     Handler callers MUST pass `callerAccountID(c)`; accountID == 0
//     means "no tenant scope" and is reserved for internal background
//     callers (auth middleware before Locals is set, GraphQL until
//     plumbed, gamification wiring, seed scripts, setup wizard).
//   - FindByLoginID / FindByEmail / FindBySISUserID / FindByResetToken /
//     Create are AUTH-INTERNAL methods: they run pre-authentication
//     (credential resolution, SIS import, password reset, JIT
//     provisioning) and have no tenant context to enforce. Handler
//     callers MUST NOT use these directly; add a scoped variant if a
//     new handler need arises. See comments on each method in
//     `internal/repository/postgres/user.go`.
//   - Update operates on a user row already loaded via a scoped
//     FindByID, matching the 2-arg shape on courseRepo.Update.
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	FindByID(ctx context.Context, id, accountID uint) (*models.User, error)
	FindByLoginID(ctx context.Context, loginID string) (*models.User, error)
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	FindBySISUserID(ctx context.Context, sisUserID string) (*models.User, error)
	FindByIDs(ctx context.Context, ids []uint, accountID uint) ([]models.User, error)
	Update(ctx context.Context, user *models.User) error
	List(ctx context.Context, params PaginationParams, accountID uint) (*PaginatedResult[models.User], error)
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
