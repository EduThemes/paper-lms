package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// sealedStringPrefix marks a text column value that holds a secretbox
// ciphertext (base64) rather than plaintext. Text columns that predate
// encryption-at-rest (e.g. context_external_tools.shared_secret) keep their
// type; the prefix lets readers tell a sealed value from a legacy one.
const sealedStringPrefix = "enc:v1:"

// IsSealedString reports whether s was produced by SealString.
func IsSealedString(s string) bool {
	return strings.HasPrefix(s, sealedStringPrefix)
}

// SealString encrypts plaintext for storage in a text column. Empty input
// and already-sealed input are returned unchanged, so callers can pass a
// value straight through on every save without double-sealing.
func SealString(plaintext string) (string, error) {
	if plaintext == "" || IsSealedString(plaintext) {
		return plaintext, nil
	}
	ct, err := Encrypt([]byte(plaintext))
	if err != nil {
		return "", err
	}
	return sealedStringPrefix + base64.StdEncoding.EncodeToString(ct), nil
}

// OpenString reverses SealString. A legacy (unprefixed) value is returned
// as-is so readers keep working until the backfill seals it.
func OpenString(stored string) (string, error) {
	if !IsSealedString(stored) {
		return stored, nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, sealedStringPrefix))
	if err != nil {
		return "", errors.New("sealed string is not valid base64")
	}
	pt, err := Decrypt(raw)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// BackfillExternalToolSharedSecrets seals every LTI 1.1 shared secret still
// stored in plaintext (B191). Idempotent: sealed rows are skipped, so it is
// safe to run on every boot.
func BackfillExternalToolSharedSecrets(ctx context.Context, db *gorm.DB) (int, error) {
	if db == nil {
		return 0, nil
	}
	if err := EnsureKeysLoaded(); err != nil {
		return 0, fmt.Errorf("encryption keys unavailable: %w", err)
	}

	type row struct {
		ID           uint
		SharedSecret string
	}
	var rows []row
	if err := db.WithContext(ctx).
		Table("context_external_tools").
		Select("id, shared_secret").
		Where("shared_secret IS NOT NULL AND shared_secret <> ''").
		Where("shared_secret NOT LIKE ?", sealedStringPrefix+"%").
		Find(&rows).Error; err != nil {
		return 0, fmt.Errorf("list backfill candidates: %w", err)
	}

	sealed := 0
	for _, r := range rows {
		v, err := SealString(r.SharedSecret)
		if err != nil {
			return sealed, fmt.Errorf("seal row %d: %w", r.ID, err)
		}
		if err := db.WithContext(ctx).
			Table("context_external_tools").
			Where("id = ?", r.ID).
			Update("shared_secret", v).Error; err != nil {
			return sealed, fmt.Errorf("update row %d: %w", r.ID, err)
		}
		sealed++
	}
	return sealed, nil
}
