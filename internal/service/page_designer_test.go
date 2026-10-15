package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/settingsctx"
)

func TestCustomCSSEnabled(t *testing.T) {
	ctx := context.Background()
	assert.True(t, service.CustomCSSEnabled(ctx, nil, 1), "no settings engine: on")

	var gotAccount uint
	lookup := func(v string, err error) service.SettingsLookupFunc {
		return func(ctx context.Context, key string) (string, error) {
			assert.Equal(t, service.PagesCustomCSSEnabledKey, key)
			gotAccount = settingsctx.AccountIDFromContext(ctx)
			return v, err
		}
	}
	assert.False(t, service.CustomCSSEnabled(ctx, lookup("false", nil), 7))
	assert.EqualValues(t, 7, gotAccount, "resolved for the given account")
	assert.True(t, service.CustomCSSEnabled(ctx, lookup("true", nil), 7))
	assert.True(t, service.CustomCSSEnabled(ctx, lookup("", nil), 7))
	assert.True(t, service.CustomCSSEnabled(ctx, lookup("maybe", nil), 7))
	assert.True(t, service.CustomCSSEnabled(ctx, lookup("", errors.New("down")), 7))
}

func TestDashboardCard_CarriesCourseLook(t *testing.T) {
	out, err := json.Marshal(service.DashboardCard{ID: 1})
	require.NoError(t, err)
	assert.Contains(t, string(out), `"look":null`)

	out, err = json.Marshal(service.DashboardCard{ID: 1, Look: models.NullJSON(`{"theme_id":"tide-pool"}`)})
	require.NoError(t, err)
	assert.Contains(t, string(out), `"look":{"theme_id":"tide-pool"}`)
}

func TestNullJSON_RoundTrip(t *testing.T) {
	var j models.NullJSON
	require.NoError(t, j.Scan(nil))
	assert.True(t, j.IsNull())
	v, err := j.Value()
	require.NoError(t, err)
	assert.Nil(t, v, "NULL stays NULL")

	require.NoError(t, j.Scan([]byte(`{"a": 1}`)))
	v, _ = j.Value()
	assert.Equal(t, `{"a": 1}`, v)

	require.NoError(t, json.Unmarshal([]byte(`null`), &j))
	assert.True(t, j.IsNull())
	v, _ = j.Value()
	assert.Nil(t, v, "JSON null is written as SQL NULL, not jsonb 'null'")
}
