package models

import "time"

type ConferenceParticipant struct {
	ID                uint      `json:"id" gorm:"column:id;primaryKey"`
	ConferenceID      uint      `json:"conference_id" gorm:"not null;uniqueIndex:idx_conf_user"`
	UserID            uint      `json:"user_id" gorm:"not null;uniqueIndex:idx_conf_user"`
	ParticipationType string    `json:"participation_type" gorm:"not null;default:'invitee'"` // initiator, invitee, observer
	User              User      `json:"user,omitempty" gorm:"foreignKey:UserID"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
