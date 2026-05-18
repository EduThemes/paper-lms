package models

import "time"

// ContentView is the per-user, per-content aggregate of page-view activity.
// One row exists per (user_id, object_type, object_id); `IncrementView`
// upserts against that unique key and increments the count + last_viewed_at
// in place rather than appending event rows.
//
// The xAPI raw-event stream (gamification_events with verb=viewed) is the
// system of record; this table is the read-path optimization the
// ViewedContent predicate hits during rule evaluation. Keeping the two in
// sync is the IncrementView caller's job.
//
// Indexes live in the SQL chain (000036); the model declares no `index:`
// tags so AutoMigrate doesn't fabricate shadow indexes the parity test
// would flag. The UNIQUE (user_id, object_type, object_id) constraint
// likewise lives in the SQL chain — Postgres creates the underlying index
// implicitly, which IncrementView's ON CONFLICT clause targets by name.
type ContentView struct {
	ID            uint      `json:"id" gorm:"column:id;primaryKey"`
	UserID        uint      `json:"user_id" gorm:"not null"`
	ObjectType    string    `json:"object_type" gorm:"not null"`
	ObjectID      uint      `json:"object_id" gorm:"not null"`
	ViewCount     int       `json:"view_count" gorm:"not null;default:1"`
	TotalSeconds  int64     `json:"total_seconds" gorm:"not null;default:0"`
	FirstViewedAt time.Time `json:"first_viewed_at" gorm:"not null;default:now()"`
	LastViewedAt  time.Time `json:"last_viewed_at" gorm:"not null;default:now()"`
}

func (ContentView) TableName() string { return "content_views" }
