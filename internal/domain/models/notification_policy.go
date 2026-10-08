package models

import (
	"time"

	"gorm.io/gorm"
)

// Notification frequencies (Canvas notification_policies.frequency).
const (
	NotificationFrequencyImmediately = "immediately"
	NotificationFrequencyDaily       = "daily"
	NotificationFrequencyWeekly      = "weekly"
	NotificationFrequencyNever       = "never"
)

// PrimaryEmailChannelID is the CommunicationChannelID a NotificationPolicy
// uses for the user's account (login) email, which has no
// communication_channels row of its own.
const PrimaryEmailChannelID uint = 0

// NotificationPolicy is one cell of the per-category x per-channel
// notification preference grid: how often notifications of Category reach
// the user on one channel (migration 000129). A missing row falls back to
// the user's NotificationPreference (global policy + per-type toggles).
type NotificationPolicy struct {
	ID     uint `json:"id" gorm:"column:id;primaryKey"`
	UserID uint `json:"user_id" gorm:"not null;uniqueIndex:idx_notification_policy_user_channel_category,priority:1"`
	// CommunicationChannelID is a communication_channels.id, or
	// PrimaryEmailChannelID (0) for the account email.
	CommunicationChannelID uint      `json:"communication_channel_id" gorm:"not null;default:0;uniqueIndex:idx_notification_policy_user_channel_category,priority:2"`
	Category               string    `json:"category" gorm:"type:text;not null;uniqueIndex:idx_notification_policy_user_channel_category,priority:3"`
	Frequency              string    `json:"frequency" gorm:"type:text;not null;default:'daily'"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// BeforeCreate mirrors the SQL DEFAULT (GORM writes '' for a zero string,
// which the frequency CHECK refuses).
func (p *NotificationPolicy) BeforeCreate(_ *gorm.DB) error {
	if p.Frequency == "" {
		p.Frequency = NotificationFrequencyDaily
	}
	return nil
}
