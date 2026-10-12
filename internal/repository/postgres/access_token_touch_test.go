package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
)

// racingTokenRepo runs onFind between the auth middleware's token read and
// its usage write, standing in for a refresh/revoke that lands mid-request.
type racingTokenRepo struct {
	repository.AccessTokenRepository
	onFind func()
}

func (r *racingTokenRepo) FindByToken(ctx context.Context, tokenHash string) (*models.AccessToken, error) {
	tok, err := r.AccessTokenRepository.FindByToken(ctx, tokenHash)
	if r.onFind != nil {
		r.onFind()
		r.onFind = nil
	}
	return tok, err
}

func (r *racingTokenRepo) TouchLastUsed(ctx context.Context, id uint, at time.Time) error {
	return r.AccessTokenRepository.(interface {
		TouchLastUsed(context.Context, uint, time.Time) error
	}).TouchLastUsed(ctx, id, at)
}

// A request that validated a token just before it was revoked (or
// refreshed) must not write the stale row back: the revoke must stick and
// the refreshed access-token hash must survive.
func TestValidateToken_UsageWriteDoesNotUndoConcurrentRevokeOrRefresh(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	seedTenantAccountForWallet(t, g, 1)
	seedUserForWallet(t, g, 10, 1, "token-race@example.com")

	base := postgres.NewAccessTokenRepository(g)
	repo := &racingTokenRepo{AccessTokenRepository: base}
	svc := service.NewAccessTokenService(repo)

	// Revoke lands mid-request.
	tok, raw, err := svc.CreatePersonalAccessToken(ctx, 10, "script", nil)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	repo.onFind = func() {
		if err := base.Delete(ctx, tok.ID); err != nil {
			t.Fatalf("revoke: %v", err)
		}
	}
	if _, err := svc.ValidateToken(ctx, raw); err != nil {
		t.Fatalf("validate (read before revoke): %v", err)
	}
	var row models.AccessToken
	if err := g.First(&row, tok.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if row.WorkflowState != "deleted" {
		t.Fatalf("revocation was undone by the usage write: workflow_state=%q", row.WorkflowState)
	}
	if row.LastUsedAt == nil {
		t.Fatal("usage must still be recorded")
	}

	// Refresh (new access-token hash written in place) lands mid-request.
	tok2, raw2, err := svc.CreatePersonalAccessToken(ctx, 10, "script 2", nil)
	if err != nil {
		t.Fatalf("create token 2: %v", err)
	}
	const refreshedHash = "refreshed-access-token-hash"
	repo.onFind = func() {
		if err := g.Model(&models.AccessToken{}).Where("id = ?", tok2.ID).Update("token", refreshedHash).Error; err != nil {
			t.Fatalf("refresh: %v", err)
		}
	}
	if _, err := svc.ValidateToken(ctx, raw2); err != nil {
		t.Fatalf("validate (read before refresh): %v", err)
	}
	var row2 models.AccessToken
	if err := g.First(&row2, tok2.ID).Error; err != nil {
		t.Fatalf("reload 2: %v", err)
	}
	if row2.Token != refreshedHash {
		t.Fatal("the refreshed access-token hash was overwritten by the stale usage write")
	}
}
