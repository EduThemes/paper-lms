package models

import "time"

// LTIPlatformKey persists the RSA key Paper signs LTI 1.3 id_tokens with
// (B181). Every replica loads the same row at boot, so a launch signed on
// one pod verifies against the JWKS served by any other pod, and the key
// (and its kid) survive restarts. The private key is a PKCS#8 DER blob
// sealed with secretbox (MFA_ENCRYPTION_KEY); plaintext never persists.
type LTIPlatformKey struct {
	ID                  uint      `json:"id" gorm:"column:id;primaryKey"`
	Name                string    `json:"name" gorm:"column:name;uniqueIndex;not null"`
	KeyID               string    `json:"key_id" gorm:"column:key_id;not null"`
	PrivateKeyEncrypted []byte    `json:"-" gorm:"column:private_key_encrypted;not null"`
	CreatedAt           time.Time `json:"created_at"`
}

func (LTIPlatformKey) TableName() string { return "lti_platform_keys" }
