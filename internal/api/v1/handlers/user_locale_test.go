package handlers_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

// G158: the per-user language preference is persisted server-side,
// validated, and can be cleared back to the account default.
func TestUpdateUser_LocalePreference(t *testing.T) {
	cases := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
		wantLocale string
	}{
		{"set", map[string]interface{}{"locale": "es"}, http.StatusOK, "es"},
		{"region canonicalized", map[string]interface{}{"locale": "es_mx"}, http.StatusOK, "es-MX"},
		{"clear to account default", map[string]interface{}{"locale": ""}, http.StatusOK, ""},
		{"absent keeps the saved choice", map[string]interface{}{"name": "Jane Doe"}, http.StatusOK, "es"},
		{"unsupported", map[string]interface{}{"locale": "xx"}, http.StatusBadRequest, ""},
		{"garbage", map[string]interface{}{"locale": "<script>"}, http.StatusBadRequest, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app, repo := setupUserHandler()
			repo.On("FindByID", mock.Anything, uint(1), uint(1)).Return(&models.User{ID: 1, Name: "Jane Doe", Locale: "es"}, nil)
			repo.On("Update", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil).Maybe()

			resp := testutil.MakeRequest(app, http.MethodPut, "/users/1", testutil.JSONBody(map[string]interface{}{"user": tc.body}))
			require.Equal(t, tc.wantStatus, resp.StatusCode)
			if tc.wantStatus != http.StatusOK {
				repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
				return
			}
			out, err := testutil.ParseJSONMap(resp)
			require.NoError(t, err)
			assert.Equal(t, tc.wantLocale, out["locale"])
		})
	}
}

func TestNormalizeUserLocale(t *testing.T) {
	for in, want := range map[string]string{"": "", " en ": "en", "EN-us": "en-US", "es-419": "es-419"} {
		got, err := service.NormalizeUserLocale(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	for _, bad := range []string{"fr", "english", "en-USA-x", "e"} {
		_, err := service.NormalizeUserLocale(bad)
		assert.ErrorIs(t, err, service.ErrUnsupportedLocale, bad)
	}
}
