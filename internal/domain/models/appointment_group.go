package models

import (
	"time"

	"gorm.io/gorm"
)

// Appointment group participant types (Canvas: participant_type). A
// "Group" appointment group is reserved by a student group of the
// group's GroupCategoryID: one reservation holds the slot for the whole
// group.
const (
	AppointmentParticipantUser  = "User"
	AppointmentParticipantGroup = "Group"
)

// AppointmentGroup represents a Canvas-compatible Scheduler appointment group.
// Instructors create a group of available time slots; students reserve slots.
//
// CourseID is the primary course (it scopes the group to a tenant). More
// courses, and section limits, live in AppointmentGroupContext rows.
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
	ParticipantType               string    `json:"participant_type" gorm:"type:text;not null;default:'User'"`
	GroupCategoryID               *uint     `json:"group_category_id" gorm:"index:idx_appt_group_category"`
	CreatedAt                     time.Time `json:"created_at"`
	UpdatedAt                     time.Time `json:"updated_at"`
}

// TableName overrides Gorm's default pluralization to keep the SQL migration
// table name consistent with our migration file.
func (AppointmentGroup) TableName() string { return "appointment_groups" }

// BeforeCreate mirrors the participant_type SQL DEFAULT: GORM writes ”
// for a zero string, which would otherwise bypass it.
func (g *AppointmentGroup) BeforeCreate(_ *gorm.DB) error {
	if g.ParticipantType == "" {
		g.ParticipantType = AppointmentParticipantUser
	}
	return nil
}

// AppointmentGroupContext adds a course (ContextType "Course") or limits
// signup to a section (ContextType "CourseSection") for an appointment
// group. When a course has section rows, only students in those sections
// may sign up from that course.
type AppointmentGroupContext struct {
	ID                 uint      `json:"id" gorm:"column:id;primaryKey"`
	AppointmentGroupID uint      `json:"appointment_group_id" gorm:"not null;uniqueIndex:idx_appt_group_ctx_unique,priority:1"`
	ContextType        string    `json:"context_type" gorm:"type:text;not null;uniqueIndex:idx_appt_group_ctx_unique,priority:2"`
	ContextID          uint      `json:"context_id" gorm:"not null;uniqueIndex:idx_appt_group_ctx_unique,priority:3"`
	CreatedAt          time.Time `json:"created_at"`
}

func (AppointmentGroupContext) TableName() string { return "appointment_group_contexts" }

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

// AppointmentReservation represents a user's hold on a single slot. For a
// Group appointment group, ParticipantGroupID is the student group the
// reservation was made for, and UserID is the member who made it.
type AppointmentReservation struct {
	ID                 uint       `json:"id" gorm:"column:id;primaryKey"`
	SlotID             uint       `json:"slot_id" gorm:"not null;index:idx_appt_res_slot"`
	GroupID            uint       `json:"group_id" gorm:"not null;index"`
	UserID             uint       `json:"user_id" gorm:"not null;index:idx_appt_res_user"`
	ParticipantGroupID *uint      `json:"participant_group_id" gorm:"index:idx_appt_res_participant_group"`
	Comments           string     `json:"comments" gorm:"type:text"`
	ReservedAt         time.Time  `json:"reserved_at"`
	CanceledAt         *time.Time `json:"canceled_at"`
	WorkflowState      string     `json:"workflow_state" gorm:"not null;default:'reserved'"`
}

func (AppointmentReservation) TableName() string { return "appointment_reservations" }
