package postgres_test

import (
	"context"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
)

// FindByIDInAccount finds a tenant's account-level schemes and its
// courses' schemes, never another tenant's.
func TestGradingStandardRepo_FindByIDInAccount(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	seedCommsAccount(t, g, 1)
	seedCommsAccount(t, g, 2)
	course1 := &models.Course{AccountID: 1, Name: "Biology", CourseCode: "BIO", WorkflowState: models.CourseAvailable}
	course2 := &models.Course{AccountID: 2, Name: "History", CourseCode: "HIS", WorkflowState: models.CourseAvailable}
	for _, c := range []*models.Course{course1, course2} {
		if err := g.Create(c).Error; err != nil {
			t.Fatalf("course: %v", err)
		}
	}
	repo := postgres.NewGradingStandardRepository(g)
	mk := func(ctxType string, ctxID uint, title string) *models.GradingStandard {
		gs := &models.GradingStandard{ContextType: ctxType, ContextID: ctxID, Title: title, WorkflowState: "active", ScalingFactor: 1}
		if err := repo.Create(ctx, gs); err != nil {
			t.Fatalf("create %s: %v", title, err)
		}
		return gs
	}
	acct1 := mk("Account", 1, "District 1 scale")
	acct2 := mk("Account", 2, "District 2 scale")
	crs1 := mk("Course", course1.ID, "Biology scale")
	crs2 := mk("Course", course2.ID, "History scale")

	for _, gs := range []*models.GradingStandard{acct1, crs1} {
		if got, err := repo.FindByIDInAccount(ctx, gs.ID, 1); err != nil || got == nil || got.ID != gs.ID {
			t.Fatalf("own scheme %q not found: %v", gs.Title, err)
		}
	}
	for _, gs := range []*models.GradingStandard{acct2, crs2} {
		if _, err := repo.FindByIDInAccount(ctx, gs.ID, 1); err == nil {
			t.Fatalf("another tenant's scheme %q must not be found", gs.Title)
		}
	}
}
