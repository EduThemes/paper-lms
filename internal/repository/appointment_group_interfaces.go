package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// AppointmentGroupRepository persists Canvas-compatible Scheduler groups.
type AppointmentGroupRepository interface {
	Create(ctx context.Context, group *models.AppointmentGroup) error
	// FindByID — 13.1.D: tenant scope via parent course's account_id.
	FindByID(ctx context.Context, id, accountID uint) (*models.AppointmentGroup, error)
	Update(ctx context.Context, group *models.AppointmentGroup) error
	Delete(ctx context.Context, id uint) error
	ListByCourse(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.AppointmentGroup], error)
}

// AppointmentSlotRepository persists individual bookable slots.
type AppointmentSlotRepository interface {
	Create(ctx context.Context, slot *models.AppointmentSlot) error
	// FindByID — 13.1.D: tenant scope via slot→group→course→account_id.
	FindByID(ctx context.Context, id, accountID uint) (*models.AppointmentSlot, error)
	Delete(ctx context.Context, id uint) error
	ListByGroup(ctx context.Context, groupID uint) ([]models.AppointmentSlot, error)
	DeleteByGroup(ctx context.Context, groupID uint) error
}

// AppointmentReservationRepository persists user holds on slots.
type AppointmentReservationRepository interface {
	Create(ctx context.Context, res *models.AppointmentReservation) error
	// FindByID — 13.1.D: tenant scope via reservation→slot→group→course→account_id.
	FindByID(ctx context.Context, id, accountID uint) (*models.AppointmentReservation, error)
	Update(ctx context.Context, res *models.AppointmentReservation) error
	ListBySlot(ctx context.Context, slotID uint) ([]models.AppointmentReservation, error)
	CountBySlot(ctx context.Context, slotID uint) (int64, error)
	ListByUser(ctx context.Context, userID uint) ([]models.AppointmentReservation, error)
	CountByGroupAndUser(ctx context.Context, groupID, userID uint) (int64, error)
}
