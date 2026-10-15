package handlers_test

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/designassets"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

func TestDesignIcon_ServesEngineSVGWithHeaders(t *testing.T) {
	app := testutil.SetupTestApp()
	app.Get("/api/v1/design/icons/:style/:file", handlers.DesignIcon)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/design/icons/duotone/star.svg?fg=12355b&bg=f5c518", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, designassets.RenderIconSVG("star", "duotone", "12355b", "f5c518"), string(body))
	assert.Equal(t, "image/svg+xml", resp.Header.Get("Content-Type"))
	assert.Equal(t, "public, max-age=31536000, immutable", resp.Header.Get("Cache-Control"))
	assert.Equal(t, "default-src 'none'; style-src 'unsafe-inline'", resp.Header.Get("Content-Security-Policy"))
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))

	// Unknown names draw the fallback; bad colours fall back to defaults.
	resp = testutil.MakeRequest(app, http.MethodGet, "/api/v1/design/icons/outline/no-such-icon.svg?fg=zzz", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ = io.ReadAll(resp.Body)
	assert.Equal(t, designassets.RenderIconSVG(designassets.Fallback, "outline", "", ""), string(body))

	for _, path := range []string{
		"/api/v1/design/icons/sparkly/star.svg",
		"/api/v1/design/icons/outline/star.png",
		"/api/v1/design/icons/outline/star",
	} {
		assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(app, http.MethodGet, path, nil).StatusCode, path)
	}
}
