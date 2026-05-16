package mocks

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/stretchr/testify/mock"
)

// MockGamificationLeaderboardSnapshotRepository mocks the W3-C / 7-B
// snapshot repo. Handler tests use it to drive the historical-window
// path; existing W2 tests pass an inert instance via setupGamification-
// Handler to satisfy the constructor.
type MockGamificationLeaderboardSnapshotRepository struct {
	mock.Mock
}

func (m *MockGamificationLeaderboardSnapshotRepository) Upsert(ctx context.Context, snap *models.GamificationLeaderboardSnapshot) (bool, error) {
	args := m.Called(ctx, snap)
	return args.Bool(0), args.Error(1)
}

func (m *MockGamificationLeaderboardSnapshotRepository) FindByWindow(ctx context.Context, scopeType models.GamificationScopeType, scopeID, currencyTypeID uint, kind string, windowEnd time.Time) (*models.GamificationLeaderboardSnapshot, error) {
	args := m.Called(ctx, scopeType, scopeID, currencyTypeID, kind, windowEnd)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GamificationLeaderboardSnapshot), args.Error(1)
}
