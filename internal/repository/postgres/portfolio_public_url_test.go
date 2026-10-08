package postgres_test

import (
	"context"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
)

// G155: a user can hold several unpublished portfolios (public_url used to
// be '' under a UNIQUE index, so the second one failed), and share tokens
// resolve only their own portfolio.
func TestPortfolioRepo_SeveralUnpublishedAndShareToken(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	seedCommsAccount(t, g, 1)
	seedUserForWallet(t, g, 101, 1, "owner@example.test")
	repo := postgres.NewPortfolioRepository(g)

	tok := "tok-abcdefghijklmnopqrstuvwx"
	a := &models.Portfolio{UserID: 101, Title: "One", Slug: "one-1", WorkflowState: models.PortfolioDraft, ShareToken: &tok}
	b := &models.Portfolio{UserID: 101, Title: "Two", Slug: "two-1", WorkflowState: models.PortfolioDraft}
	if err := repo.Create(ctx, a); err != nil {
		t.Fatalf("first portfolio: %v", err)
	}
	if err := repo.Create(ctx, b); err != nil {
		t.Fatalf("second unpublished portfolio must be allowed: %v", err)
	}

	got, err := repo.FindByShareToken(ctx, tok)
	if err != nil || got.ID != a.ID {
		t.Fatalf("FindByShareToken = %+v, %v", got, err)
	}
	if _, err := repo.FindByShareToken(ctx, "tok-unknown-000000000000000"); err == nil {
		t.Fatalf("unknown token must not resolve")
	}
	dup := &models.Portfolio{UserID: 101, Title: "Three", Slug: "three-1", WorkflowState: models.PortfolioDraft, ShareToken: &tok}
	if err := repo.Create(ctx, dup); err == nil {
		t.Fatalf("share tokens must be unique")
	}
	if err := repo.Delete(ctx, a.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.FindByShareToken(ctx, tok); err == nil {
		t.Fatalf("a deleted portfolio's link must stop working")
	}
}
