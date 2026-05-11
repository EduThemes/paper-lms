package models

import "time"

type CommunicationChannel struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	UserID        uint       `json:"user_id" gorm:"index"`
	ChannelType   string     `json:"channel_type"`                    // email, webhook, push
	Address       string     `json:"address"`                         // email address, webhook URL, push token
	Position      int        `json:"position" gorm:"default:1"`
	Confirmed     bool       `json:"confirmed" gorm:"default:false"`
	ConfirmCode   string     `json:"-"`
	ConfirmedAt   *time.Time `json:"confirmed_at"`
	WorkflowState string     `json:"workflow_state" gorm:"default:active"` // active, retired
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
