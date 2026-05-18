package models

import "time"

// AppointmentGroup represents a Canvas-compatible Scheduler appointment group.
// Instructors create a group of available time slots; students reserve slots.
type AppointmentGroup struct {
	ID                            uint      `json:"id" gorm:"column:id;primaryKey"`
	CourseID                      uint      `json:"course_id" gorm:"not null;index:idx_appt_group_course"`
	Title                         string    `json:"title" gorm:"not null"`
	Description                   string    `json:"description" gorm:"type:text"`
	LocationName                  string    `json:"location_name"`
	LocationAddress               string    `json:"location_address"`
	MinAppointmentsPerParticipant int       `json:"min_appointments_per_participant" gorm:"default:0"`
	MaxAppointmentsPerParticipant int       `json:"max_appointments_per_participant" gorm:"default:1"`
	ParticipantsPerAppointment    int       `json:"participants_per_appointment" gorm:"default:1"`
	CreatedByUserID               uint      `json:"created_by_user_id" gorm:"not null"`
	WorkflowState                 string    `json:"workflow_state" gorm:"not null;default:'pending'"`
	CreatedAt                     time.Time `json:"created_at"`
	UpdatedAt                     time.Time `json:"updated_at"`
}

// TableName overrides Gorm's default pluralization to keep the SQL migration
// table name consistent with our migration file.
func (AppointmentGroup) TableName() string { return "appointment_groups" }

// AppointmentSlot represents a single bookable time slot within a group.
type AppointmentSlot struct {
	ID                uint      `json:"id" gorm:"column:id;primaryKey"`
	GroupID           uint      `json:"group_id" gorm:"not null;index:idx_appt_slot_group"`
	StartAt           time.Time `json:"start_at" gorm:"not null;index"`
	EndAt             time.Time `json:"end_at" gorm:"not null"`
	ParticipantsLimit *int      `json:"participants_limit"` // overrides group's ParticipantsPerAppointment when set
	CreatedAt         time.Time `json:"created_at"`
}

func (AppointmentSlot) TableName() string { return "appointment_slots" }

// AppointmentReservation represents a user's hold on a single slot.
type AppointmentReservation struct {
	ID            uint       `json:"id" gorm:"column:id;primaryKey"`
	SlotID        uint       `json:"slot_id" gorm:"not null;index:idx_appt_res_slot"`
	GroupID       uint       `json:"group_id" gorm:"not null;index"`
	UserID        uint       `json:"user_id" gorm:"not null;index:idx_appt_res_user"`
	ReservedAt    time.Time  `json:"reserved_at"`
	CanceledAt    *time.Time `json:"canceled_at"`
	WorkflowState string     `json:"workflow_state" gorm:"not null;default:'reserved'"`
}

func (AppointmentReservation) TableName() string { return "appointment_reservations" }
