package service_test

// Unit tests of the LTI Advantage helpers (G125, G127, G131, G133).

import (
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // OAuth 1.0a
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

func TestExpandLTIVariables_GatesPIIByPrivacy(t *testing.T) {
	sis := "SIS-1"
	vc := &service.LTIVariableContext{
		Issuer: "https://lms.example.com", AccountID: 3,
		User:   &models.User{ID: 7, Name: "Jane Doe", Email: "jane@example.com", LoginID: "jdoe", SISUserID: &sis},
		Course: &models.Course{ID: 10, Name: "Biology", CourseCode: "BIO-1"},
	}
	custom := map[string]string{
		"uid": "$Canvas.user.id", "name": "$Person.name.full", "given": "$Person.name.given", "mail": "$Person.email.primary",
		"sis": "$Person.sourcedId", "course": "$Context.title", "label": "$Context.label", "domain": "$Canvas.api.domain",
		"unknown": "$Not.a.variable", "plain": "hello",
	}

	vc.PrivacyLevel = service.LTIPrivacyAnonymous
	out := service.ExpandLTIVariables(custom, vc)
	assert.Equal(t, "7", out["uid"])
	assert.Equal(t, "$Person.name.full", out["name"])
	assert.Equal(t, "$Person.email.primary", out["mail"])
	assert.Equal(t, "$Person.sourcedId", out["sis"])
	assert.Equal(t, "Biology", out["course"])
	assert.Equal(t, "BIO-1", out["label"])
	assert.Equal(t, "lms.example.com", out["domain"])
	assert.Equal(t, "$Not.a.variable", out["unknown"])
	assert.Equal(t, "hello", out["plain"])

	vc.PrivacyLevel = service.LTIPrivacyPublic
	out = service.ExpandLTIVariables(custom, vc)
	assert.Equal(t, "Jane Doe", out["name"])
	assert.Equal(t, "Jane", out["given"])
	assert.Equal(t, "jane@example.com", out["mail"])
	assert.Equal(t, "SIS-1", out["sis"])
}

func TestLTIPlacementMessageTypeDefaults(t *testing.T) {
	ps, err := service.ParseLTIPlacements(`[{"placement":"editor_button"},{"placement":"course_navigation"},{"placement":"link_selection","message_type":"LtiResourceLinkRequest","enabled":false}]`)
	require.NoError(t, err)
	require.Len(t, ps, 3)
	assert.Equal(t, service.LTIMessageDeepLinking, ps[0].EffectiveMessageType())
	assert.Equal(t, service.LTIMessageResourceLink, ps[1].EffectiveMessageType())
	assert.Equal(t, service.LTIMessageResourceLink, ps[2].EffectiveMessageType())
	assert.False(t, ps[2].IsEnabled())
	_, err = service.ParseLTIPlacements(`{"placement":"x"}`)
	assert.Error(t, err)
}

func TestOAuth1Signature(t *testing.T) {
	params := map[string]string{"b": "2 & 3", "a": "1", "oauth_nonce": "n", "oauth_signature": "ignored"}
	sig, err := service.OAuth1Signature("post", "HTTPS://Tool.Example.com:443/launch?z=9", params, "s3cret")
	require.NoError(t, err)

	// Independent computation of the RFC 5849 base string.
	base := "POST&https%3A%2F%2Ftool.example.com%2Flaunch&" +
		"a%3D1%26b%3D2%2520%2526%25203%26oauth_nonce%3Dn%26z%3D9"
	mac := hmac.New(sha1.New, []byte("s3cret&"))
	mac.Write([]byte(base))
	assert.Equal(t, base64.StdEncoding.EncodeToString(mac.Sum(nil)), sig)

	other, _ := service.OAuth1Signature("POST", "https://tool.example.com/launch?z=9", params, "different")
	assert.NotEqual(t, sig, other)
}

func TestLooksLikeLTIServiceToken(t *testing.T) {
	rs := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	hs := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	assert.True(t, service.LooksLikeLTIServiceToken(rs+".e30.sig"))
	assert.False(t, service.LooksLikeLTIServiceToken(hs+".e30.sig"), "session tokens (HS256) take the session path")
	assert.False(t, service.LooksLikeLTIServiceToken("opaque-personal-access-token"))
}

func TestValidateToolPublicJWK(t *testing.T) {
	assert.Error(t, service.ValidateToolPublicJWK(`{"kty":"EC"}`))
	assert.Error(t, service.ValidateToolPublicJWK(`not json`))
	assert.Error(t, service.ValidateToolPublicJWK(`{"kty":"RSA","n":"AQAB","e":"AQAB"}`), "keys under 2048 bits are refused")
}
