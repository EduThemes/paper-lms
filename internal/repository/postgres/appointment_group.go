package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

// ----- AppointmentGroupRepository -----

type appointmentGroupRepo struct {
	db *gorm.DB
}

func NewAppointmentGroupRepository(db *gorm.DB) *appointmentGroupRepo {
	return &appointmentGroupRepo{db: db}
}

func (r *appointmentGroupRepo) Create(ctx context.Context, group *models.AppointmentGroup) error {
	return r.db.WithContext(ctx).Create(group).Error
}

func (r *appointmentGroupRepo) FindByID(ctx context.Context, id uint) (*models.AppointmentGroup, error) {
	var g models.AppointmentGroup
	if err := r.db.WithContext(ctx).First(&g, id).Error; err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *appointmentGroupRepo) Update(ctx context.Context, group *models.AppointmentGroup) error {
	return r.db.WithContext(ctx).Save(group).Error
}

func (r *appointmentGroupRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&models.AppointmentGroup{}).
		Where("id = ?", id).
		Update("workflow_state", "deleted").Error
}

func (r *appointmentGroupRepo) ListByCourse(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AppointmentGroup], error) {
	var items []models.AppointmentGroup
	var total int64

	q := r.db.WithContext(ctx).
		Model(&models.AppointmentGroup{}).
		Where("course_id = ? AND workflow_state != ?", courseID, "deleted")
	q.Count(&total)

	offset := (params.Page - 1) * params.PerPage
	if err := q.Order("created_at DESC").Offset(offset).Limit(params.PerPage).Find(&items).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.AppointmentGroup]{
		Items:      items,
		TotalCount: total,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

// ----- AppointmentSlotRepository -----

type appointmentSlotRepo struct {
	db *gorm.DB
}

func NewAppointmentSlotRepository(db *gorm.DB) *appointmentSlotRepo {
	return &appointmentSlotRepo{db: db}
}

func (r *appointmentSlotRepo) Create(ctx context.Context, slot *models.AppointmentSlot) error {
	return r.db.WithContext(ctx).Create(slot).Error
}

func (r *appointmentSlotRepo) FindByID(ctx context.Context, id uint) (*models.AppointmentSlot, error) {
	var s models.AppointmentSlot
	if err := r.db.WithContext(ctx).First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *appointmentSlotRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.AppointmentSlot{}, id).Error
}

func (r *appointmentSlotRepo) ListByGroup(ctx context.Context, groupID uint) ([]models.AppointmentSlot, error) {
	var slots []models.AppointmentSlot
	if err := r.db.WithContext(ctx).
		Where("group_id = ?", groupID).
		Order("start_at ASC").
		Find(&slots).Error; err != nil {
		return nil, err
	}
	return slots, nil
}

func (r *appointmentSlotRepo) DeleteByGroup(ctx context.Context, groupID uint) error {
	return r.db.WithContext(ctx).
		Where("group_id = ?", groupID).
		Delete(&models.AppointmentSlot{}).Error
}

// ----- AppointmentReservationRepository -----

type appointmentReservationRepo struct {
	db *gorm.DB
}

func NewAppointmentReservationRepository(db *gorm.DB) *appointmentReservationRepo {
	return &appointmentReservationRepo{db: db}
}

func (r *appointmentReservationRepo) Create(ctx context.Context, res *models.AppointmentReservation) error {
	return r.db.WithContext(ctx).Create(res).Error
}

func (r *appointmentReservationRepo) FindByID(ctx context.Context, id uint) (*models.AppointmentReservation, error) {
	var rv models.AppointmentReservation
	if err := r.db.WithContext(ctx).First(&rv, id).Error; err != nil {
		return nil, err
	}
	return &rv, nil
}

func (r *appointmentReservationRepo) Update(ctx context.Context, res *models.AppointmentReservation) error {
	return r.db.WithContext(ctx).Save(res).Error
}

func (r *appointmentReservationRepo) ListBySlot(ctx context.Context, slotID uint) ([]models.AppointmentReservation, error) {
	var items []models.AppointmentReservation
	if err := r.db.WithContext(ctx).
		Where("slot_id = ? AND workflow_state = ?", slotID, "reserved").
		Order("reserved_at ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *appointmentReservationRepo) CountBySlot(ctx context.Context, slotID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&models.AppointmentReservation{}).
		Where("slot_id = ? AND workflow_state = ?", slotID, "reserved").
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *appointmentReservationRepo) ListByUser(ctx context.Context, userID uint) ([]models.AppointmentReservation, error) {
	var items []models.AppointmentReservation
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND workflow_state = ?", userID, "reserved").
		Order("reserved_at DESC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *appointmentReservationRepo) CountByGroupAndUser(ctx context.Context, groupID, userID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&models.AppointmentReservation{}).
		Where("group_id = ? AND user_id = ? AND workflow_state = ?", groupID, userID, "reserved").
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
