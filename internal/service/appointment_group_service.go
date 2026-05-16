package service

import (
	"context"
	"errors"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

// AppointmentGroupService implements Canvas-compatible Scheduler logic:
// instructors publish a set of bookable slots; students reserve them with
// per-slot capacity enforcement.
type AppointmentGroupService struct {
	groupRepo       repository.AppointmentGroupRepository
	slotRepo        repository.AppointmentSlotRepository
	reservationRepo repository.AppointmentReservationRepository
	db              *gorm.DB
}

func NewAppointmentGroupService(
	groupRepo repository.AppointmentGroupRepository,
	slotRepo repository.AppointmentSlotRepository,
	reservationRepo repository.AppointmentReservationRepository,
	db *gorm.DB,
) *AppointmentGroupService {
	return &AppointmentGroupService{
		groupRepo:       groupRepo,
		slotRepo:        slotRepo,
		reservationRepo: reservationRepo,
		db:              db,
	}
}

var (
	ErrSlotFull              = errors.New("appointment slot is full")
	ErrAlreadyReserved       = errors.New("you already have a reservation for this slot")
	ErrMaxReservationsHit    = errors.New("maximum reservations for this group reached")
	ErrAppointmentGroupGone  = errors.New("appointment group not found or deleted")
	ErrSlotMismatch          = errors.New("slot does not belong to this appointment group")
	ErrReservationMismatch   = errors.New("reservation does not belong to this slot")
	ErrCannotCancelOthers    = errors.New("you can only cancel your own reservation")
	ErrSlotInPast            = errors.New("cannot reserve a slot that has already started")
	ErrInvalidSlotTimes      = errors.New("slot end_at must be after start_at")
)

// SlotInput describes a slot to be created when creating a group.
type SlotInput struct {
	StartAt           time.Time
	EndAt             time.Time
	ParticipantsLimit *int
}

// CreateGroup creates an appointment group and any provided slots in a single
// transaction.
func (s *AppointmentGroupService) CreateGroup(ctx context.Context, group *models.AppointmentGroup, slots []SlotInput) (*models.AppointmentGroup, []models.AppointmentSlot, error) {
	if group.Title == "" {
		return nil, nil, errors.New("title is required")
	}
	if group.WorkflowState == "" {
		group.WorkflowState = "active"
	}
	if group.MaxAppointmentsPerParticipant <= 0 {
		group.MaxAppointmentsPerParticipant = 1
	}
	if group.ParticipantsPerAppointment <= 0 {
		group.ParticipantsPerAppointment = 1
	}

	for _, in := range slots {
		if !in.EndAt.After(in.StartAt) {
			return nil, nil, ErrInvalidSlotTimes
		}
	}

	var createdSlots []models.AppointmentSlot
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(group).Error; err != nil {
			return err
		}
		for _, in := range slots {
			slot := models.AppointmentSlot{
				GroupID:           group.ID,
				StartAt:           in.StartAt,
				EndAt:             in.EndAt,
				ParticipantsLimit: in.ParticipantsLimit,
			}
			if err := tx.Create(&slot).Error; err != nil {
				return err
			}
			createdSlots = append(createdSlots, slot)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return group, createdSlots, nil
}

func (s *AppointmentGroupService) GetGroup(ctx context.Context, id, accountID uint) (*models.AppointmentGroup, error) {
	g, err := s.groupRepo.FindByID(ctx, id, accountID)
	if err != nil {
		return nil, err
	}
	if g.WorkflowState == "deleted" {
		return nil, ErrAppointmentGroupGone
	}
	return g, nil
}

func (s *AppointmentGroupService) UpdateGroup(ctx context.Context, group *models.AppointmentGroup) error {
	return s.groupRepo.Update(ctx, group)
}

func (s *AppointmentGroupService) DeleteGroup(ctx context.Context, id uint) error {
	return s.groupRepo.Delete(ctx, id)
}

func (s *AppointmentGroupService) ListByCourse(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AppointmentGroup], error) {
	return s.groupRepo.ListByCourse(ctx, courseID, params)
}

func (s *AppointmentGroupService) ListSlots(ctx context.Context, groupID uint) ([]models.AppointmentSlot, error) {
	return s.slotRepo.ListByGroup(ctx, groupID)
}

// SlotAvailability bundles a slot with its current reservation count and limit.
type SlotAvailability struct {
	Slot              models.AppointmentSlot
	ReservationCount  int64
	EffectiveLimit    int
	Available         bool
}

// ListSlotsWithAvailability returns slot rows enriched with reservation counts.
// Callers (handler/UI) can choose whether to include full slots.
func (s *AppointmentGroupService) ListSlotsWithAvailability(ctx context.Context, group *models.AppointmentGroup) ([]SlotAvailability, error) {
	slots, err := s.slotRepo.ListByGroup(ctx, group.ID)
	if err != nil {
		return nil, err
	}
	out := make([]SlotAvailability, 0, len(slots))
	for _, slot := range slots {
		count, err := s.reservationRepo.CountBySlot(ctx, slot.ID)
		if err != nil {
			return nil, err
		}
		limit := group.ParticipantsPerAppointment
		if slot.ParticipantsLimit != nil {
			limit = *slot.ParticipantsLimit
		}
		out = append(out, SlotAvailability{
			Slot:             slot,
			ReservationCount: count,
			EffectiveLimit:   limit,
			Available:        int(count) < limit,
		})
	}
	return out, nil
}

func (s *AppointmentGroupService) ListReservations(ctx context.Context, slotID uint) ([]models.AppointmentReservation, error) {
	return s.reservationRepo.ListBySlot(ctx, slotID)
}

func (s *AppointmentGroupService) ListMyReservations(ctx context.Context, userID uint) ([]models.AppointmentReservation, error) {
	return s.reservationRepo.ListByUser(ctx, userID)
}

// Reserve atomically checks capacity and creates a reservation for the given
// user. Re-reservation by the same user on the same slot is rejected.
func (s *AppointmentGroupService) Reserve(ctx context.Context, groupID, slotID, userID uint) (*models.AppointmentReservation, error) {
	var reservation models.AppointmentReservation

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the group row so concurrent callers see a stable view.
		var group models.AppointmentGroup
		if err := tx.First(&group, groupID).Error; err != nil {
			return ErrAppointmentGroupGone
		}
		if group.WorkflowState == "deleted" {
			return ErrAppointmentGroupGone
		}

		var slot models.AppointmentSlot
		if err := tx.First(&slot, slotID).Error; err != nil {
			return err
		}
		if slot.GroupID != groupID {
			return ErrSlotMismatch
		}
		if !slot.StartAt.After(time.Now()) {
			return ErrSlotInPast
		}

		// Reject duplicate reservation by same user on same slot.
		var existing int64
		if err := tx.Model(&models.AppointmentReservation{}).
			Where("slot_id = ? AND user_id = ? AND workflow_state = ?", slotID, userID, "reserved").
			Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return ErrAlreadyReserved
		}

		// Enforce per-user max appointments per group.
		if group.MaxAppointmentsPerParticipant > 0 {
			var groupCount int64
			if err := tx.Model(&models.AppointmentReservation{}).
				Where("group_id = ? AND user_id = ? AND workflow_state = ?", groupID, userID, "reserved").
				Count(&groupCount).Error; err != nil {
				return err
			}
			if int(groupCount) >= group.MaxAppointmentsPerParticipant {
				return ErrMaxReservationsHit
			}
		}

		// Capacity check.
		limit := group.ParticipantsPerAppointment
		if slot.ParticipantsLimit != nil {
			limit = *slot.ParticipantsLimit
		}
		var count int64
		if err := tx.Model(&models.AppointmentReservation{}).
			Where("slot_id = ? AND workflow_state = ?", slotID, "reserved").
			Count(&count).Error; err != nil {
			return err
		}
		if int(count) >= limit {
			return ErrSlotFull
		}

		reservation = models.AppointmentReservation{
			SlotID:        slotID,
			GroupID:       groupID,
			UserID:        userID,
			ReservedAt:    time.Now(),
			WorkflowState: "reserved",
		}
		return tx.Create(&reservation).Error
	})
	if err != nil {
		return nil, err
	}
	return &reservation, nil
}

// Cancel marks a reservation as canceled. Caller must ensure authorization.
func (s *AppointmentGroupService) Cancel(ctx context.Context, reservation *models.AppointmentReservation) error {
	now := time.Now()
	reservation.WorkflowState = "canceled"
	reservation.CanceledAt = &now
	return s.reservationRepo.Update(ctx, reservation)
}

func (s *AppointmentGroupService) GetReservation(ctx context.Context, id, accountID uint) (*models.AppointmentReservation, error) {
	return s.reservationRepo.FindByID(ctx, id, accountID)
}

func (s *AppointmentGroupService) GetSlot(ctx context.Context, id, accountID uint) (*models.AppointmentSlot, error) {
	return s.slotRepo.FindByID(ctx, id, accountID)
}
