// Package auth's secretbox provides envelope encryption for at-rest
// secrets — TOTP secrets, OIDC client secrets, LDAP bind passwords,
// anything else the gamification + auth surface needs to keep
// confidential in the DB. AES-256-GCM with a versioned ciphertext
// header so we can rotate keys without re-deploying.
//
// Ciphertext layout on disk (bytes):
//
//	[1: key_id][12: nonce][N: ciphertext+tag]
//
// Why versioned:
//   - v1 ships with a single key. The 1-byte key_id is always 0x01.
//   - When the operator rotates (next phase), they add a second key
//     keyed under 0x02. Decrypt picks the right key by reading the
//     header byte; Encrypt always uses the current ("latest") key id.
//     A background sweep can re-encrypt rows with old key_ids.
//
// Key source: env var MFA_ENCRYPTION_KEY, base64-encoded 32 bytes.
// The package fails fast if the key is unset or the wrong length;
// production deployments must set this, no silent fallback. This is
// the "production secrets leak" failure mode the audit warned about.
package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

// envKeyVar is the env var that must hold a base64-encoded 32-byte
// AES-256 key. Documented in .env.example.
const envKeyVar = "MFA_ENCRYPTION_KEY"

// currentKeyID is the key version used by Encrypt for new writes.
// Decrypt accepts any key id present in keys.
const currentKeyID byte = 0x01

var (
	keysOnce sync.Once
	keys     map[byte][]byte // key_id → 32-byte key
	keysErr  error
)

// loadKeys reads MFA_ENCRYPTION_KEY once and caches the result. Future
// key rotation: read additional MFA_ENCRYPTION_KEY_V2 etc. and add to
// the map keyed by key_id. v1 ships with one key.
func loadKeys() (map[byte][]byte, error) {
	keysOnce.Do(func() {
		raw := os.Getenv(envKeyVar)
		if raw == "" {
			keysErr = fmt.Errorf("%s not set — encryption-at-rest is unavailable", envKeyVar)
			return
		}
		k, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			keysErr = fmt.Errorf("%s is not valid base64: %w", envKeyVar, err)
			return
		}
		if len(k) != 32 {
			keysErr = fmt.Errorf("%s must decode to 32 bytes (AES-256); got %d", envKeyVar, len(k))
			return
		}
		keys = map[byte][]byte{currentKeyID: k}
	})
	return keys, keysErr
}

// EnsureKeysLoaded is a startup-time guard. cmd/server/main.go calls
// it before serving so missing-key failures happen at boot, not on the
// first secret-write request.
func EnsureKeysLoaded() error {
	_, err := loadKeys()
	return err
}

// Encrypt seals plaintext under the current key. Output: header + nonce
// + ciphertext+tag.
func Encrypt(plaintext []byte) ([]byte, error) {
	keyMap, err := loadKeys()
	if err != nil {
		return nil, err
	}
	key, ok := keyMap[currentKeyID]
	if !ok {
		return nil, fmt.Errorf("no key configured under id %d", currentKeyID)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	out := make([]byte, 0, 1+len(nonce)+len(plaintext)+aead.Overhead())
	out = append(out, currentKeyID)
	out = append(out, nonce...)
	out = aead.Seal(out, nonce, plaintext, nil)
	return out, nil
}

// Decrypt opens a ciphertext produced by Encrypt. Validates the key id
// header, picks the right key, decrypts.
func Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < 1+12+16 { // header + min nonce + min GCM tag
		return nil, errors.New("ciphertext too short")
	}
	keyMap, err := loadKeys()
	if err != nil {
		return nil, err
	}
	keyID := ciphertext[0]
	key, ok := keyMap[keyID]
	if !ok {
		return nil, fmt.Errorf("no key configured under id %d (ciphertext may be from a rotated-out key)", keyID)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := aead.NonceSize()
	if len(ciphertext) < 1+nonceSize+aead.Overhead() {
		return nil, errors.New("ciphertext too short for this AEAD")
	}
	nonce := ciphertext[1 : 1+nonceSize]
	body := ciphertext[1+nonceSize:]
	return aead.Open(nil, nonce, body, nil)
}

// GenerateKey is a CLI helper for operators provisioning a new
// deployment. Outputs a fresh 32-byte key base64-encoded — ready to
// paste into .env.
func GenerateKey() (string, error) {
	k := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, k); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(k), nil
}
