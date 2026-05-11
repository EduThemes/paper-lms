package models

import "time"

type NotificationDelivery struct {
	ID             uint       `json:"id" gorm:"primaryKey"`
	NotificationID uint       `json:"notification_id" gorm:"index"`
	UserID         uint       `json:"user_id" gorm:"index"`
	ChannelType    string     `json:"channel_type"`                                  // email, webhook
	Address        string     `json:"address"`
	Subject        string     `json:"subject"`
	Body           string     `json:"body"`
	DeliveryStatus string     `json:"delivery_status" gorm:"default:pending;index"` // pending, queued, sent, delivered, bounced, failed
	DigestType     string     `json:"digest_type"`                                   // immediate, hourly, daily, weekly
	RetryCount     int        `json:"retry_count" gorm:"default:0"`
	MaxRetries     int        `json:"max_retries" gorm:"default:3"`
	LastError      string     `json:"last_error"`
	SentAt         *time.Time `json:"sent_at"`
	DeliveredAt    *time.Time `json:"delivered_at"`
	ScheduledFor   *time.Time `json:"scheduled_for" gorm:"index"` // for digest batching
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
