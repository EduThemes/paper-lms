package models

import (
	"time"

	"github.com/lib/pq"
)

// GamificationWalletBalance holds one user's current balance in one
// currency. Composite PK (user_id, currency_type_id) — a single row per
// pair. lifetime_earned is monotonic even when balance is spendable, so
// leaderboards over earned-rather-than-held semantics stay accurate.
//
// The CHECK (balance >= 0) constraint lives in the SQL chain (000034).
type GamificationWalletBalance struct {
	UserID         uint      `json:"user_id" gorm:"primaryKey"`
	CurrencyTypeID uint      `json:"currency_type_id" gorm:"primaryKey"`
	Balance        int64     `json:"balance" gorm:"not null;default:0"`
	LifetimeEarned int64     `json:"lifetime_earned" gorm:"not null;default:0"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"not null;default:now()"`
}

func (GamificationWalletBalance) TableName() string { return "gamification_wallet_balances" }

// GamificationWalletTransaction is the immutable ledger entry. Positive
// delta = earn, negative delta = spend. Reason values follow the pattern
// "rule:<rule_id>" | "manual:<actor_id>" | "spend:<sku>" | "seed:<source>".
// Every balance change in the system produces a row here; balance rows
// are derived (or could be) from a sum of transactions.
type GamificationWalletTransaction struct {
	ID                uint           `json:"id" gorm:"column:id;primaryKey"`
	UserID            uint           `json:"user_id" gorm:"not null"`
	CurrencyTypeID    uint           `json:"currency_type_id" gorm:"not null"`
	Delta             int64          `json:"delta" gorm:"not null"`
	Reason            string         `json:"reason" gorm:"not null"`
	TriggeringEventID *uint          `json:"triggering_event_id,omitempty"`
	TriggeringRuleID  *uint          `json:"triggering_rule_id,omitempty"`
	PolicyFlags       pq.StringArray `json:"policy_flags" gorm:"type:text[];not null;default:'{}'"`
	OccurredAt        time.Time      `json:"occurred_at" gorm:"not null;default:now()"`
}

func (GamificationWalletTransaction) TableName() string { return "gamification_wallet_transactions" }
