package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// LTIAdvantageDeps wires the collaborators of the LTI 1.3 / Advantage
// features (G123-G134). Every field is optional: a nil collaborator
// disables only the feature that needs it, so older tests that build a
// bare LTIService keep working.
type LTIAdvantageDeps struct {
	Store          repository.LTIAdvantageRepository
	UserRepo       repository.UserRepository
	AssignmentRepo repository.AssignmentRepository
	LineItemRepo   repository.LTILineItemRepository
	ResultRepo     repository.LTIResultRepository
	ModuleRepo     repository.ModuleRepository
	ModuleItemRepo repository.ModuleItemRepository
	// OpenSecret unseals an LTI 1.1 shared secret (auth.OpenString).
	OpenSecret func(string) (string, error)
	// SealSecret seals an LTI 1.1 shared secret for storage.
	SealSecret func(string) (string, error)
	// Grader writes LTI 1.1 Outcomes scores into the gradebook.
	Grader AGSGrader
	// ValidateURL is the SSRF guard (security.ValidateExternalURL) run
	// before any fetch of a tool's JWKS URL.
	ValidateURL func(ctx context.Context, rawURL string) error
	// HTTPClient fetches tool JWKS. It should use the SSRF-safe dialer.
	HTTPClient *http.Client
	// APIBaseURL is the absolute base of Paper's REST API, e.g.
	// "https://lms.example.com/api/v1". Defaults to PlatformIssuer+"/api/v1".
	APIBaseURL string
}

// SetAdvantageDeps installs the LTI Advantage collaborators.
func (s *LTIService) SetAdvantageDeps(d LTIAdvantageDeps) {
	s.adv = d
}

// APIBaseURL is the absolute base URL of the REST API that LTI service
// endpoints (AGS, NRPS, deep linking return, token) live under.
func (s *LTIService) APIBaseURL() string {
	if s.adv.APIBaseURL != "" {
		return strings.TrimSuffix(s.adv.APIBaseURL, "/")
	}
	return strings.TrimSuffix(s.PlatformIssuer, "/") + "/api/v1"
}

// TokenEndpointURL is the OAuth2 token endpoint tools post their
// client_credentials grant to.
func (s *LTIService) TokenEndpointURL() string { return s.APIBaseURL() + "/login/oauth2/token" }

// AuthorizationEndpointURL is the OIDC authorization endpoint of LTI launches.
func (s *LTIService) AuthorizationEndpointURL() string { return s.APIBaseURL() + "/lti/authorize" }

// JWKSURL is the platform's public key set.
func (s *LTIService) JWKSURL() string { return s.APIBaseURL() + "/lti/jwks" }

// DeepLinkReturnURL is where tools post LtiDeepLinkingResponse messages.
func (s *LTIService) DeepLinkReturnURL() string { return s.APIBaseURL() + "/lti/deep_linking_response" }

// LTI message types.
const (
	LTIMessageResourceLink = "LtiResourceLinkRequest"
	LTIMessageDeepLinking  = "LtiDeepLinkingRequest"
)

// LTI placements Paper renders (G131).
const (
	LTIPlacementCourseNavigation    = "course_navigation"
	LTIPlacementEditorButton        = "editor_button"
	LTIPlacementAssignmentSelection = "assignment_selection"
	LTIPlacementLinkSelection       = "link_selection"
	LTIPlacementModuleIndexMenu     = "module_index_menu"
	LTIPlacementHomeworkSubmission  = "homework_submission"
	LTIPlacementAccountNavigation   = "account_navigation"
	// LTIPlacementResourceLink is the pseudo-placement of a launch of an
	// existing resource link (module item, assignment, embedded link).
	LTIPlacementResourceLink = "resource_link"
)

// KnownLTIPlacements is the set of placements a tool configuration may
// declare.
var KnownLTIPlacements = map[string]bool{
	LTIPlacementCourseNavigation:    true,
	LTIPlacementEditorButton:        true,
	LTIPlacementAssignmentSelection: true,
	LTIPlacementLinkSelection:       true,
	LTIPlacementModuleIndexMenu:     true,
	LTIPlacementHomeworkSubmission:  true,
	LTIPlacementAccountNavigation:   true,
}

// LTI privacy levels (Canvas parity).
const (
	LTIPrivacyAnonymous = "anonymous"
	LTIPrivacyNameOnly  = "name_only"
	LTIPrivacyEmailOnly = "email_only"
	LTIPrivacyPublic    = "public"
)

// ValidLTIPrivacyLevel reports whether p is a known privacy level.
func ValidLTIPrivacyLevel(p string) bool {
	switch p {
	case LTIPrivacyAnonymous, LTIPrivacyNameOnly, LTIPrivacyEmailOnly, LTIPrivacyPublic:
		return true
	}
	return false
}

func privacyAllowsName(p string) bool  { return p == LTIPrivacyNameOnly || p == LTIPrivacyPublic }
func privacyAllowsEmail(p string) bool { return p == LTIPrivacyEmailOnly || p == LTIPrivacyPublic }
func privacyAllowsSIS(p string) bool   { return p == LTIPrivacyPublic }

// LTI Advantage scopes (AGS, NRPS) a tool may be granted.
const (
	LTIScopeLineItem         = "https://purl.imsglobal.org/spec/lti-ags/scope/lineitem"
	LTIScopeLineItemReadOnly = "https://purl.imsglobal.org/spec/lti-ags/scope/lineitem.readonly"
	LTIScopeResultReadOnly   = "https://purl.imsglobal.org/spec/lti-ags/scope/result.readonly"
	LTIScopeScore            = "https://purl.imsglobal.org/spec/lti-ags/scope/score"
	LTIScopeNRPS             = "https://purl.imsglobal.org/spec/lti-nrps/scope/contextmembership.readonly"
)

// LTIAdvantageScopes lists every service scope Paper grants.
var LTIAdvantageScopes = []string{LTIScopeLineItem, LTIScopeLineItemReadOnly, LTIScopeResultReadOnly, LTIScopeScore, LTIScopeNRPS}

func isLTIAdvantageScope(s string) bool {
	for _, v := range LTIAdvantageScopes {
		if v == s {
			return true
		}
	}
	return false
}

// LTIPlacementConfig is one entry of a tool configuration's placements
// JSON array (Canvas tool configuration shape).
type LTIPlacementConfig struct {
	Placement     string `json:"placement"`
	Enabled       *bool  `json:"enabled,omitempty"`
	MessageType   string `json:"message_type,omitempty"`
	TargetLinkURI string `json:"target_link_uri,omitempty"`
	Text          string `json:"text,omitempty"`
	IconURL       string `json:"icon_url,omitempty"`
	// Width/Height size the launch frame (editor_button dialogs).
	SelectionWidth  int `json:"selection_width,omitempty"`
	SelectionHeight int `json:"selection_height,omitempty"`
}

// IsEnabled reports whether the placement is enabled (default true).
func (p LTIPlacementConfig) IsEnabled() bool { return p.Enabled == nil || *p.Enabled }

// EffectiveMessageType defaults the message type by placement: selection
// placements default to deep linking, everything else to a resource link.
func (p LTIPlacementConfig) EffectiveMessageType() string {
	if p.MessageType != "" {
		return p.MessageType
	}
	switch p.Placement {
	case LTIPlacementEditorButton, LTIPlacementAssignmentSelection, LTIPlacementLinkSelection, LTIPlacementModuleIndexMenu, LTIPlacementHomeworkSubmission:
		return LTIMessageDeepLinking
	}
	return LTIMessageResourceLink
}

// ParseLTIPlacements decodes a tool configuration's placements JSON.
func ParseLTIPlacements(raw string) ([]LTIPlacementConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil, nil
	}
	var out []LTIPlacementConfig
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, errors.New("placements must be a JSON array of placement objects")
	}
	return out, nil
}

// findPlacement returns the enabled config of placement, if any.
func findPlacement(cfg *models.LTIToolConfiguration, placement string) (LTIPlacementConfig, bool) {
	if cfg == nil {
		return LTIPlacementConfig{}, false
	}
	ps, _ := ParseLTIPlacements(cfg.Placements)
	for _, p := range ps {
		if p.Placement == placement && p.IsEnabled() {
			return p, true
		}
	}
	return LTIPlacementConfig{}, false
}

// ParseStringMap decodes a JSON object of custom parameters. Non-string
// values are stringified (custom claim values are always strings).
func ParseStringMap(raw string) map[string]string {
	out := map[string]string{}
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return out
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return out
	}
	for k, v := range m {
		switch tv := v.(type) {
		case string:
			out[k] = tv
		case nil:
			out[k] = ""
		default:
			b, _ := json.Marshal(tv)
			out[k] = string(b)
		}
	}
	return out
}

// ParseScopeList decodes the JSON-array scope column.
func ParseScopeList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return strings.Fields(raw)
	}
	return out
}

func scopeListHas(scopes []string, want ...string) bool {
	for _, s := range scopes {
		for _, w := range want {
			if s == w {
				return true
			}
		}
	}
	return false
}

// --- platform-signed JWTs ----------------------------------------------------

// Purposes of the platform-signed JWTs Paper hands out. Each carries a
// distinct aud so one can never be replayed as another (and none is an
// id_token, whose aud is a tool client_id, or a session token, which is
// HS256).
const (
	ltiPurposeHint         = "paper-lms:lti-message-hint"
	ltiPurposeDeepLinkData = "paper-lms:lti-deep-link-data"
	ltiPurposeService      = "paper-lms:lti-service-token"
	ltiPurposeRegistration = "paper-lms:lti-registration"
)

// signPlatformJWT signs claims RS256 with the current platform key.
func (s *LTIService) signPlatformJWT(claims jwt.MapClaims) (string, error) {
	key, kid := s.currentKey()
	if key == nil {
		return "", errors.New("platform key has not been initialized")
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	return tok.SignedString(key)
}

// verifyPlatformJWT verifies a platform-signed JWT of the given purpose
// against every published platform key (so tokens survive a rotation).
func (s *LTIService) verifyPlatformJWT(tokenStr, purpose string) (jwt.MapClaims, error) {
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		kid, _ := t.Header["kid"].(string)
		pub := s.publicKeyForKID(kid)
		if pub == nil {
			return nil, errors.New("unknown key id")
		}
		return pub, nil
	}, jwt.WithAudience(purpose), jwt.WithIssuer(s.PlatformIssuer), jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	return claims, nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func claimUint(c jwt.MapClaims, k string) uint {
	switch v := c[k].(type) {
	case float64:
		if v > 0 {
			return uint(v)
		}
	case json.Number:
		n, _ := v.Int64()
		if n > 0 {
			return uint(n)
		}
	}
	return 0
}

func claimString(c jwt.MapClaims, k string) string {
	v, _ := c[k].(string)
	return v
}

// --- message hint (G124/G125) --------------------------------------------------

// LTIDeepLinkTarget says where the content items of a deep linking
// response go.
type LTIDeepLinkTarget struct {
	// Kind is "module" (create module items), "assignment" (attach the
	// selected link to an assignment), "editor" (insert into the rich
	// content editor) or "link" (return a link to the caller).
	Kind         string `json:"kind"`
	ModuleID     uint   `json:"module_id,omitempty"`
	AssignmentID uint   `json:"assignment_id,omitempty"`
}

// ltiLaunchHint is the platform's signed lti_message_hint: the full
// launch context, so the authorization endpoint never trusts a client
// supplied course, tool or resource link.
type ltiLaunchHint struct {
	UserID           uint
	AccountID        uint
	CourseID         uint
	ToolID           uint // ContextExternalTool (the deployment)
	DevKeyID         uint
	Placement        string
	MessageType      string
	ResourceLinkUUID string
	AssignmentID     uint
	TargetLinkURI    string
	DocumentTarget   string
	DeepLink         *LTIDeepLinkTarget
	Locale           string
	ReturnURL        string
}

const ltiHintTTL = 10 * time.Minute

func (s *LTIService) signHint(h ltiLaunchHint) (string, error) {
	now := time.Now()
	c := jwt.MapClaims{
		"iss": s.PlatformIssuer, "aud": ltiPurposeHint,
		"iat": now.Unix(), "exp": now.Add(ltiHintTTL).Unix(), "jti": randomHex(8),
		"uid": h.UserID, "acct": h.AccountID, "cid": h.CourseID, "tool": h.ToolID, "dk": h.DevKeyID,
		"pl": h.Placement, "mt": h.MessageType, "rl": h.ResourceLinkUUID, "aid": h.AssignmentID,
		"tlu": h.TargetLinkURI, "dt": h.DocumentTarget, "loc": h.Locale, "ret": h.ReturnURL,
	}
	if h.DeepLink != nil {
		c["dl"] = map[string]interface{}{"kind": h.DeepLink.Kind, "module_id": h.DeepLink.ModuleID, "assignment_id": h.DeepLink.AssignmentID}
	}
	return s.signPlatformJWT(c)
}

func (s *LTIService) parseHint(tok string) (*ltiLaunchHint, error) {
	c, err := s.verifyPlatformJWT(tok, ltiPurposeHint)
	if err != nil {
		return nil, err
	}
	h := &ltiLaunchHint{
		UserID: claimUint(c, "uid"), AccountID: claimUint(c, "acct"), CourseID: claimUint(c, "cid"),
		ToolID: claimUint(c, "tool"), DevKeyID: claimUint(c, "dk"), Placement: claimString(c, "pl"),
		MessageType: claimString(c, "mt"), ResourceLinkUUID: claimString(c, "rl"), AssignmentID: claimUint(c, "aid"),
		TargetLinkURI: claimString(c, "tlu"), DocumentTarget: claimString(c, "dt"), Locale: claimString(c, "loc"),
		ReturnURL: claimString(c, "ret"),
	}
	if dl, ok := c["dl"].(map[string]interface{}); ok {
		t := &LTIDeepLinkTarget{}
		t.Kind, _ = dl["kind"].(string)
		if v, ok := dl["module_id"].(float64); ok {
			t.ModuleID = uint(v)
		}
		if v, ok := dl["assignment_id"].(float64); ok {
			t.AssignmentID = uint(v)
		}
		h.DeepLink = t
	}
	if h.UserID == 0 || h.CourseID == 0 || h.DevKeyID == 0 {
		return nil, errors.New("incomplete message hint")
	}
	return h, nil
}

// looksLikeJWT distinguishes the signed hint from the legacy
// "course:link:key" hint.
func looksLikeJWT(s string) bool { return strings.Count(s, ".") == 2 && !strings.Contains(s, ":") }

// sortedKeys returns m's keys in order (stable claims for tests/logs).
func sortedKeys(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// courseURL builds an absolute SPA URL under the course.
func (s *LTIService) courseURL(courseID uint, suffix string) string {
	return fmt.Sprintf("%s/courses/%d%s", strings.TrimSuffix(s.PlatformIssuer, "/"), courseID, suffix)
}

// validToolURL accepts an absolute https URL (http only for localhost,
// for local tool development).
func validToolURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	h := u.Hostname()
	return u.Scheme == "http" && (h == "localhost" || h == "127.0.0.1")
}
