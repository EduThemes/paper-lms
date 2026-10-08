package service_test

// B071 regression: a blank SIS term id is stored as NULL so any number of
// terms can be created without one; SIS term ids are unique per account
// (not deployment-wide); and term reads are tenant-scoped.

import (
	"context"
	"errors"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
)

func TestEnrollmentTerms_BlankAndPerTenantSISTermID(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	seedSISTenants(t, g)
	ctx := context.Background()
	svc := service.NewEnrollmentTermService(postgres.NewEnrollmentTermRepository(g), g)

	blank := ""
	for _, name := range []string{"Fall", "Spring", "Summer"} {
		term := &models.EnrollmentTerm{AccountID: 1, Name: name, SISTermID: &blank}
		if err := svc.CreateTerm(ctx, term); err != nil {
			t.Fatalf("create %s term without SIS id: %v", name, err)
		}
		if term.SISTermID != nil {
			t.Fatalf("blank SIS id stored as %q, want NULL", *term.SISTermID)
		}
	}

	sis := "2026-FA"
	first := &models.EnrollmentTerm{AccountID: 1, Name: "Fall 2026", SISTermID: &sis}
	if err := svc.CreateTerm(ctx, first); err != nil {
		t.Fatalf("create SIS term: %v", err)
	}
	// Another tenant may reuse the same SIS term id.
	other := &models.EnrollmentTerm{AccountID: 2, Name: "Fall 2026", SISTermID: &sis}
	if err := svc.CreateTerm(ctx, other); err != nil {
		t.Fatalf("tenant 2 reusing a SIS term id: %v", err)
	}
	// The same tenant may not.
	dup := &models.EnrollmentTerm{AccountID: 1, Name: "Dup", SISTermID: &sis}
	if err := svc.CreateTerm(ctx, dup); !errors.Is(err, service.ErrSISTermIDTaken) {
		t.Fatalf("duplicate SIS id in one tenant: got %v, want ErrSISTermIDTaken", err)
	}

	// Updating a term with its own SIS id is not a conflict.
	first.Name = "Fall 2026 (renamed)"
	if err := svc.UpdateTerm(ctx, first); err != nil {
		t.Fatalf("rename SIS term: %v", err)
	}

	// Reads are tenant-scoped.
	if _, err := svc.GetTerm(ctx, first.ID, 2); err == nil {
		t.Fatal("tenant 2 read tenant 1's term")
	}
	if got, err := svc.GetTerm(ctx, first.ID, 1); err != nil || got.ID != first.ID {
		t.Fatalf("owner read: %v", err)
	}
	if found, err := svc.FindBySISTermID(ctx, sis, 2); err != nil || found.ID != other.ID {
		t.Fatalf("FindBySISTermID scoped lookup: %v", err)
	}
}
