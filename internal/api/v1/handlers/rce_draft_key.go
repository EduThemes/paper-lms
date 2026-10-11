package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// RCEDraftKeyHandler hands the rich content editor the key it encrypts its
// local autosave drafts with (G012).
//
// Canvas keeps RCE autosave drafts in browser storage encrypted with a
// per-user key and keyed by user. Paper's drafts used to be plaintext and
// shared by everyone who used the browser. The key is derived, not stored:
// HMAC-SHA256(server secret, label || user id || account id). It is stable
// for a user across sessions (so a draft written before a session expired
// can be restored after signing back in), different for every user (so one
// person on a shared computer cannot read another's drafts), and only
// obtainable with that user's authenticated session.
type RCEDraftKeyHandler struct {
	secret []byte
}

// rceDraftKeyLabel domain-separates this derivation from every other use of
// the server secret (JWT signing).
const rceDraftKeyLabel = "paper-lms/rce-draft-key/v1"

// NewRCEDraftKeyHandler builds the handler from the server's signing secret.
func NewRCEDraftKeyHandler(serverSecret string) *RCEDraftKeyHandler {
	return &RCEDraftKeyHandler{secret: []byte(serverSecret)}
}

// DeriveRCEDraftKey returns the 32-byte AES-256 key for a user's drafts.
func (h *RCEDraftKeyHandler) DeriveRCEDraftKey(userID, accountID uint) []byte {
	mac := hmac.New(sha256.New, h.secret)
	mac.Write([]byte(rceDraftKeyLabel))
	mac.Write([]byte{0})
	mac.Write([]byte(strconv.FormatUint(uint64(userID), 10)))
	mac.Write([]byte{0})
	mac.Write([]byte(strconv.FormatUint(uint64(accountID), 10)))
	return mac.Sum(nil)
}

// GetDraftKey handles GET /api/v1/users/self/rce_draft_key.
func (h *RCEDraftKeyHandler) GetDraftKey(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	if len(h.secret) == 0 {
		return fiber.NewError(fiber.StatusServiceUnavailable, "Draft encryption is not configured")
	}
	accountID, _ := c.Locals("account_id").(uint)
	key := h.DeriveRCEDraftKey(userID, accountID)
	// Never cache a key in a shared proxy or the browser's HTTP cache.
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(fiber.Map{
		"key":       base64.StdEncoding.EncodeToString(key),
		"algorithm": "AES-GCM",
		"user_id":   userID,
	})
}
