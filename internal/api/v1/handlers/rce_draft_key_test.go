package handlers

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func draftKeyApp(h *RCEDraftKeyHandler, userID, accountID uint) *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		if userID != 0 {
			c.Locals("user_id", userID)
			c.Locals("account_id", accountID)
		}
		return c.Next()
	})
	app.Get("/users/self/rce_draft_key", h.GetDraftKey)
	return app
}

func fetchDraftKey(t *testing.T, app *fiber.App) (int, map[string]any, string) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest("GET", "/users/self/rce_draft_key", nil), -1)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	out := map[string]any{}
	_ = json.Unmarshal(body, &out)
	return resp.StatusCode, out, resp.Header.Get("Cache-Control")
}

// G012: the editor's draft key is per user, stable across sessions, never
// cached, and only handed to an authenticated caller.
func TestRCEDraftKey_PerUserStableAndUncached(t *testing.T) {
	h := NewRCEDraftKeyHandler("test-signing-secret")

	status, a1, cache := fetchDraftKey(t, draftKeyApp(h, 11, 1))
	require.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, "no-store", cache)
	assert.Equal(t, "AES-GCM", a1["algorithm"])
	assert.EqualValues(t, 11, a1["user_id"])
	raw, err := base64.StdEncoding.DecodeString(a1["key"].(string))
	require.NoError(t, err)
	assert.Len(t, raw, 32, "AES-256 key")

	_, again, _ := fetchDraftKey(t, draftKeyApp(h, 11, 1))
	assert.Equal(t, a1["key"], again["key"], "same user gets the same key in a later session")

	_, other, _ := fetchDraftKey(t, draftKeyApp(h, 12, 1))
	assert.NotEqual(t, a1["key"], other["key"], "another user gets a different key")

	_, otherTenant, _ := fetchDraftKey(t, draftKeyApp(h, 11, 2))
	assert.NotEqual(t, a1["key"], otherTenant["key"], "the key is tenant-bound")

	_, otherSecret, _ := fetchDraftKey(t, draftKeyApp(NewRCEDraftKeyHandler("rotated-secret"), 11, 1))
	assert.NotEqual(t, a1["key"], otherSecret["key"], "rotating the server secret rotates draft keys")
}

func TestRCEDraftKey_RequiresAuthentication(t *testing.T) {
	status, body, _ := fetchDraftKey(t, draftKeyApp(NewRCEDraftKeyHandler("s"), 0, 0))
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Nil(t, body["key"])
}

func TestRCEDraftKey_UnconfiguredSecret(t *testing.T) {
	status, body, _ := fetchDraftKey(t, draftKeyApp(NewRCEDraftKeyHandler(""), 3, 1))
	assert.Equal(t, fiber.StatusServiceUnavailable, status)
	assert.Nil(t, body["key"])
}
