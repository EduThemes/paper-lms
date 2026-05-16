package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockAppointmentGroupRepository implements repository.AppointmentGroupRepository for testing.
type MockAppointmentGroupRepository struct {
	mock.Mock
}

func (m *MockAppointmentGroupRepository) Create(ctx context.Context, group *models.AppointmentGroup) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}

func (m *MockAppointmentGroupRepository) FindByID(ctx context.Context, id, accountID uint) (*models.AppointmentGroup, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AppointmentGroup), args.Error(1)
}

func (m *MockAppointmentGroupRepository) Update(ctx context.Context, group *models.AppointmentGroup) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}

func (m *MockAppointmentGroupRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockAppointmentGroupRepository) ListByCourse(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AppointmentGroup], error) {
	args := m.Called(ctx, courseID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.AppointmentGroup]), args.Error(1)
}

// MockAppointmentSlotRepository implements repository.AppointmentSlotRepository for testing.
type MockAppointmentSlotRepository struct {
	mock.Mock
}

func (m *MockAppointmentSlotRepository) Create(ctx context.Context, slot *models.AppointmentSlot) error {
	args := m.Called(ctx, slot)
	return args.Error(0)
}

func (m *MockAppointmentSlotRepository) FindByID(ctx context.Context, id, accountID uint) (*models.AppointmentSlot, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AppointmentSlot), args.Error(1)
}

func (m *MockAppointmentSlotRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockAppointmentSlotRepository) ListByGroup(ctx context.Context, groupID uint) ([]models.AppointmentSlot, error) {
	args := m.Called(ctx, groupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.AppointmentSlot), args.Error(1)
}

func (m *MockAppointmentSlotRepository) DeleteByGroup(ctx context.Context, groupID uint) error {
	args := m.Called(ctx, groupID)
	return args.Error(0)
}

// MockAppointmentReservationRepository implements repository.AppointmentReservationRepository for testing.
type MockAppointmentReservationRepository struct {
	mock.Mock
}

func (m *MockAppointmentReservationRepository) Create(ctx context.Context, res *models.AppointmentReservation) error {
	args := m.Called(ctx, res)
	return args.Error(0)
}

func (m *MockAppointmentReservationRepository) FindByID(ctx context.Context, id, accountID uint) (*models.AppointmentReservation, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AppointmentReservation), args.Error(1)
}

func (m *MockAppointmentReservationRepository) Update(ctx context.Context, res *models.AppointmentReservation) error {
	args := m.Called(ctx, res)
	return args.Error(0)
}

func (m *MockAppointmentReservationRepository) ListBySlot(ctx context.Context, slotID uint) ([]models.AppointmentReservation, error) {
	args := m.Called(ctx, slotID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.AppointmentReservation), args.Error(1)
}

func (m *MockAppointmentReservationRepository) CountBySlot(ctx context.Context, slotID uint) (int64, error) {
	args := m.Called(ctx, slotID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockAppointmentReservationRepository) ListByUser(ctx context.Context, userID uint) ([]models.AppointmentReservation, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.AppointmentReservation), args.Error(1)
}

func (m *MockAppointmentReservationRepository) CountByGroupAndUser(ctx context.Context, groupID, userID uint) (int64, error) {
	args := m.Called(ctx, groupID, userID)
	return args.Get(0).(int64), args.Error(1)
}
