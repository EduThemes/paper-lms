package service

// Test-only exports. Compiled into the test binary alongside the
// external service_test package so integration tests can reach
// unexported internals without widening the production API.

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// ReconcileDeprovision exposes reconcileDeprovision for the
// oneroster_deprovision_test.go integration tests.
func (s *OneRosterService) ReconcileDeprovision(ctx context.Context, conn *models.OneRosterConnection, presentSISIDs map[string]bool, apply bool) (*DeprovisionResult, error) {
	return s.reconcileDeprovision(ctx, conn, presentSISIDs, apply)
}

// OneRosterPresentSet exposes onerosterPresentSet so tests can lock the
// tobedeleted-users-are-absent contract.
func OneRosterPresentSet(users []struct{ SourcedID, Status string }) map[string]bool {
	converted := make([]onerosterUser, len(users))
	for i, u := range users {
		converted[i] = onerosterUser{SourcedID: u.SourcedID, Status: u.Status}
	}
	return onerosterPresentSet(converted)
}
