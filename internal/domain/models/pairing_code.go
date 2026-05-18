package models

import (
	"crypto/rand"
	"math/big"
	"strings"
	"time"
)

// PairingCode is a Canvas-compatible parent/observer pairing code. A student
// (or someone acting on their behalf) generates a short, human-readable code,
// shares it out-of-band with a parent/observer, and the observer redeems it to
// link their account to the student's account.
//
// Code format: 9 alphanumeric characters arranged in three groups of three
// separated by hyphens, e.g. "K7H-PQM-3RD". The character set excludes the
// visually-confusing letters O/0 and I/1.
type PairingCode struct {
	ID         uint       `json:"id" gorm:"column:id;primaryKey"`
	Code       string     `json:"code" gorm:"uniqueIndex;not null"`
	UserID     uint       `json:"user_id" gorm:"not null;index"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at" gorm:"not null;index"`
	RedeemedAt *time.Time `json:"redeemed_at,omitempty"`
}

// pairingCodeAlphabet excludes O, 0, I, and 1 to avoid visual confusion when a
// user reads or types the code.
const pairingCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// GeneratePairingCodeString returns a new 9-character code formatted in three
// groups of three separated by hyphens, e.g. "K7H-PQM-3RD". It uses
// crypto/rand for unbiased selection.
func GeneratePairingCodeString() (string, error) {
	const groupSize = 3
	const groups = 3
	max := big.NewInt(int64(len(pairingCodeAlphabet)))

	var sb strings.Builder
	for g := 0; g < groups; g++ {
		if g > 0 {
			sb.WriteByte('-')
		}
		for i := 0; i < groupSize; i++ {
			n, err := rand.Int(rand.Reader, max)
			if err != nil {
				return "", err
			}
			sb.WriteByte(pairingCodeAlphabet[n.Int64()])
		}
	}
	return sb.String(), nil
}

// IsExpired reports whether the code has passed its expiry timestamp.
func (p *PairingCode) IsExpired(now time.Time) bool {
	return !now.Before(p.ExpiresAt)
}

// IsRedeemed reports whether the code has already been used.
func (p *PairingCode) IsRedeemed() bool {
	return p.RedeemedAt != nil
}
