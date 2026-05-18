package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type userRepo struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) repository.UserRepository {
	return &userRepo{db: db}
}

func (r *userRepo) Create(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// FindByID resolves a user by primary key, scoped to the caller's tenant.
//
// Tenant scope (Phase 13.1.D + Sprint 2.3 widening): when accountID != 0
// the query is constrained to users in that account; accountID == 0
// means "no tenant scope" and is reserved for internal callers that
// have either no tenant context (auth middleware resolving the JWT
// subject before account_id is set on Locals) or have already
// validated tenant ownership upstream (login pipeline / passkey
// engine — see the auth-internal boundary policy below).
//
// Handler/service callers MUST pass `callerAccountID(c)` — see
// `internal/api/v1/handlers/helpers.go`. A cross-tenant ID with this
// scope returns gorm.ErrRecordNotFound, which the handler maps to 404
// (NOT 403) to preserve the existence-leak contract.
func (r *userRepo) FindByID(ctx context.Context, id, accountID uint) (*models.User, error) {
	var user models.User
	q := r.db.WithContext(ctx)
	if accountID != 0 {
		q = q.Where("account_id = ?", accountID)
	}
	if err := q.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByIDs returns the subset of users matching the given IDs, scoped
// to the caller's tenant. accountID == 0 follows the same semantics as
// FindByID (no scope; reserved for internal callers). Cross-tenant IDs
// are silently dropped from the result set rather than surfacing as an
// error — the caller already has the candidate set from a prior query
// (gamification rank rendering, peer-name resolution) and just needs a
// tenant-safe filter on the way out.
func (r *userRepo) FindByIDs(ctx context.Context, ids []uint, accountID uint) ([]models.User, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var users []models.User
	q := r.db.WithContext(ctx).Where("id IN ?", ids)
	if accountID != 0 {
		q = q.Where("account_id = ?", accountID)
	}
	if err := q.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// FindByLoginID is part of the AUTH-INTERNAL boundary.
//
// Tenant scope deliberately NOT enforced: this method is called by
// `LoginPipeline.Execute` (internal/auth/login_pipeline.go) during the
// credential-resolution phase, BEFORE the user has been authenticated
// and therefore BEFORE a tenant context exists. The login provider
// determines the tenant; we can't know account_id at this point.
//
// New handler callers MUST NOT use this method directly. If a handler
// needs a login-id lookup post-authentication, add a scoped variant
// (e.g. `FindByLoginIDInAccount(ctx, login, accountID)`) rather than
// stretching this one. Today there are no such handler callers — every
// usage is in the auth package or `UserService.Authenticate`.
func (r *userRepo) FindByLoginID(ctx context.Context, loginID string) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).Where("login_id = ?", loginID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByEmail is part of the AUTH-INTERNAL boundary.
//
// Tenant scope deliberately NOT enforced. Callers:
//   - `LoginPipeline.Execute` email auto-link (pre-auth, no tenant yet)
//   - `UserService.Authenticate` fallback (pre-auth)
//   - `UserService.RequestPasswordReset` (pre-auth; the email IS the
//     identity claim, the reset-token gates the followup)
//   - `UserService.Register` duplicate-email check (no tenant for
//     self-signup; multi-tenant signup invokes a different code path)
//
// New handler callers MUST NOT use this method directly. See
// FindByLoginID for the scoped-variant pattern.
func (r *userRepo) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// FindBySISUserID is part of the AUTH-INTERNAL boundary.
//
// Tenant scope deliberately NOT enforced. Callers are SIS import paths
// (`oneroster_service.go`) that operate at job/batch granularity with
// admin-level credentials; tenant scope is enforced at the job
// boundary, not the lookup.
func (r *userRepo) FindBySISUserID(ctx context.Context, sisUserID string) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).Where("sis_user_id = ?", sisUserID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// Update saves changes to a user row.
//
// Tenant scope NOT enforced at the SQL level here: GORM's Save uses
// the primary key, and the caller has already loaded the user via a
// scoped FindByID (which enforced the tenant). The User model carries
// account_id on the row itself, so an Update that targets the wrong
// row would fail the prior FindByID check first. This matches the
// 2-arg shape used by courseRepo.Update (see internal/repository/
// postgres/course.go:43).
func (r *userRepo) Update(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

// FindByResetToken is part of the AUTH-INTERNAL boundary.
//
// Tenant scope deliberately NOT enforced. The reset token is the
// secret — anyone with the token already proved control of the
// associated email address. Tenant context is not available at the
// reset-token-submit endpoint (the user is not yet authenticated).
func (r *userRepo) FindByResetToken(ctx context.Context, token string) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).Where("reset_token = ? AND reset_token != ''", token).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// List returns paginated users, scoped to the caller's tenant when
// accountID != 0. accountID == 0 is reserved for super-admin and
// internal background callers (e.g. the setup wizard that runs
// pre-auth and needs to enumerate deployment-wide admin presence).
// Matches the Search tenant-filter shape added in PR #34.
func (r *userRepo) List(ctx context.Context, params repository.PaginationParams, accountID uint) (*repository.PaginatedResult[models.User], error) {
	var users []models.User
	var count int64

	q := r.db.WithContext(ctx).Model(&models.User{})
	if accountID != 0 {
		q = q.Where("account_id = ?", accountID)
	}
	q.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := q.Offset(offset).Limit(params.PerPage).Order("id ASC").Find(&users).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.User]{
		Items:      users,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

// Search returns paginated users whose name or email matches searchTerm.
//
// Tenant scope (Phase 13.1.D pattern): when accountID != 0 the query is
// constrained to users in that account; accountID == 0 means "no tenant
// scope" and is reserved for internal background callers. Handler
// callers MUST pass `callerAccountID(c)` — see
// `internal/api/v1/handlers/helpers.go`. This method previously had no
// tenant filter, which let any admin in any tenant enumerate users in
// any other tenant via name/email substring (Canvas-CVE-class
// cross-tenant info leak).
func (r *userRepo) Search(ctx context.Context, searchTerm string, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.User], error) {
	var users []models.User
	var count int64

	like := "%" + searchTerm + "%"
	query := r.db.WithContext(ctx).Model(&models.User{}).Where("name ILIKE ? OR email ILIKE ?", like, like)
	if accountID != 0 {
		query = query.Where("account_id = ?", accountID)
	}
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("name ASC").Find(&users).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.User]{
		Items:      users,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

// FilterPublicLeaderboardCandidates returns the subset of `candidateIDs`
// that have `leaderboard_opt_out = FALSE`. Order is not preserved (the
// caller already has the relevance ordering from the leaderboard query;
// this is a set-membership filter, not a ranking).
//
// Implementation note: phrased as an inclusive SELECT rather than a
// NOT IN against the opted-out partial index because the candidate set
// is typically already small (≤30 per leaderboard cohort per SYNTHESIS
// §7's relative-leaderboard sizing). Wave 3 may revisit if larger boards
// land.
//
// Tenant scope NOT applied here: the caller already obtained
// candidateIDs from a tenant-scoped leaderboard query upstream. This
// method only applies the per-learner opt-out filter on top.
func (r *userRepo) FilterPublicLeaderboardCandidates(ctx context.Context, candidateIDs []uint) ([]uint, error) {
	if len(candidateIDs) == 0 {
		return nil, nil
	}
	var ids []uint
	err := r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id IN ? AND leaderboard_opt_out = ?", candidateIDs, false).
		Pluck("id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}
