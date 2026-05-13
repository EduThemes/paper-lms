package models

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID                  uint       `json:"id" gorm:"primaryKey"`
	Name                string     `json:"name" gorm:"not null"`
	SortableName        string     `json:"sortable_name"`
	ShortName           string     `json:"short_name"`
	LoginID             string     `json:"login_id" gorm:"uniqueIndex;not null"`
	SISUserID           *string    `json:"sis_user_id" gorm:"uniqueIndex"`
	Email               string     `json:"email" gorm:"not null"`
	PasswordHash        string     `json:"-" gorm:"not null"`
	AvatarURL           string     `json:"avatar_url"`
	Role                string     `json:"role" gorm:"not null;default:'user'"` // admin, user
	Locale              string     `json:"locale" gorm:"default:'en'"`
	TimeZone            string     `json:"time_zone" gorm:"default:'America/New_York'"`
	// LeaderboardOptOut is the user-facing privacy toggle shipped in
	// W2-C. When true, this user is excluded from public leaderboard
	// surfaces — but their currencies / awards / mastery are unchanged
	// (per SYNTHESIS §5: opting out does NOT zero progress).
	//
	// No SQL `default:` GORM tag is set on purpose: the migration
	// (000040) carries DEFAULT FALSE, and we never want this column's
	// behavior to be ambiguous if a future write path uses `db.Updates`.
	// `db.Save` (which UserRepo.Update uses) writes every column.
	LeaderboardOptOut   bool       `json:"leaderboard_opt_out"`
	ResetToken          string     `json:"-"`
	ResetTokenExpiresAt *time.Time `json:"-"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

func (u *User) HashPassword(password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hashedPassword)
	return nil
}

func (u *User) CheckPassword(password string) error {
	return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
}
