package service

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// --- tool public keys (G127, G130) ---------------------------------------------

// parseRSAJWK converts an RSA JWK object to a public key.
func parseRSAJWK(m map[string]interface{}) (*rsa.PublicKey, string, error) {
	if kty, _ := m["kty"].(string); kty != "RSA" {
		return nil, "", errors.New("JWK must be an RSA key")
	}
	nStr, _ := m["n"].(string)
	eStr, _ := m["e"].(string)
	nb, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(nStr, "="))
	if err != nil || len(nb) == 0 {
		return nil, "", errors.New("JWK modulus is not base64url")
	}
	eb, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(eStr, "="))
	if err != nil || len(eb) == 0 {
		return nil, "", errors.New("JWK exponent is not base64url")
	}
	e := new(big.Int).SetBytes(eb)
	if !e.IsInt64() || e.Int64() < 3 {
		return nil, "", errors.New("JWK exponent is invalid")
	}
	pub := &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: int(e.Int64())}
	if pub.N.BitLen() < 2048 {
		return nil, "", errors.New("JWK modulus must be at least 2048 bits")
	}
	kid, _ := m["kid"].(string)
	return pub, kid, nil
}

type toolJWK struct {
	kid string
	key *rsa.PublicKey
}

// parseJWKSet accepts a single JWK object or a {"keys": [...]} set.
func parseJWKSet(raw []byte) ([]toolJWK, error) {
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, errors.New("public key is not a JSON object")
	}
	var entries []map[string]interface{}
	if keys, ok := obj["keys"].([]interface{}); ok {
		for _, k := range keys {
			if m, ok := k.(map[string]interface{}); ok {
				entries = append(entries, m)
			}
		}
	} else {
		entries = append(entries, obj)
	}
	var out []toolJWK
	for _, m := range entries {
		if use, _ := m["use"].(string); use != "" && use != "sig" {
			continue
		}
		pub, kid, err := parseRSAJWK(m)
		if err != nil {
			continue
		}
		out = append(out, toolJWK{kid: kid, key: pub})
	}
	if len(out) == 0 {
		return nil, errors.New("no usable RSA signing key")
	}
	return out, nil
}

// ValidateToolPublicJWK checks an admin-entered public JWK (G123).
func ValidateToolPublicJWK(raw string) error {
	_, err := parseJWKSet([]byte(raw))
	return err
}

type jwksCacheEntry struct {
	keys      []toolJWK
	fetchedAt time.Time
}

var (
	toolJWKSCacheMu sync.Mutex
	toolJWKSCache   = map[string]jwksCacheEntry{}
)

const (
	toolJWKSCacheTTL      = 5 * time.Minute
	toolJWKSMinRefetch    = 10 * time.Second
	toolJWKSMaxBodyBytes  = 256 << 10
	toolJWKSFetchDeadline = 10 * time.Second
)

// fetchToolJWKS GETs a tool's JWKS URL through the SSRF guard.
func (s *LTIService) fetchToolJWKS(ctx context.Context, jwksURL string) ([]toolJWK, error) {
	if s.adv.ValidateURL == nil || s.adv.HTTPClient == nil {
		return nil, errors.New("tool JWKS fetching is not configured")
	}
	if err := s.adv.ValidateURL(ctx, jwksURL); err != nil {
		return nil, fmt.Errorf("tool JWKS URL refused: %w", err)
	}
	fctx, cancel := context.WithTimeout(ctx, toolJWKSFetchDeadline)
	defer cancel()
	req, err := http.NewRequestWithContext(fctx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := s.adv.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch tool JWKS: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch tool JWKS: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, toolJWKSMaxBodyBytes))
	if err != nil {
		return nil, err
	}
	return parseJWKSet(body)
}

func pickJWK(keys []toolJWK, kid string) *rsa.PublicKey {
	if kid == "" && len(keys) == 1 {
		return keys[0].key
	}
	for _, k := range keys {
		if k.kid == kid {
			return k.key
		}
	}
	return nil
}

// toolPublicKey resolves the key a tool signed with (kid from the JWT
// header): the configured public JWK, else the tool's JWKS URL (cached,
// refetched on an unknown kid).
func (s *LTIService) toolPublicKey(ctx context.Context, cfg *models.LTIToolConfiguration, kid string) (*rsa.PublicKey, error) {
	if cfg == nil {
		return nil, errors.New("tool is not configured")
	}
	if strings.TrimSpace(cfg.PublicJWK) != "" {
		keys, err := parseJWKSet([]byte(cfg.PublicJWK))
		if err != nil {
			return nil, err
		}
		if k := pickJWK(keys, kid); k != nil {
			return k, nil
		}
		if len(keys) == 1 {
			return keys[0].key, nil
		}
		return nil, errors.New("no tool key matches the token's kid")
	}
	jwksURL := strings.TrimSpace(cfg.PublicJWKURL)
	if jwksURL == "" {
		return nil, errors.New("tool has no public key configured")
	}
	toolJWKSCacheMu.Lock()
	entry, ok := toolJWKSCache[jwksURL]
	toolJWKSCacheMu.Unlock()
	if ok && time.Since(entry.fetchedAt) < toolJWKSCacheTTL {
		if k := pickJWK(entry.keys, kid); k != nil {
			return k, nil
		}
		if time.Since(entry.fetchedAt) < toolJWKSMinRefetch {
			return nil, errors.New("no tool key matches the token's kid")
		}
	}
	keys, err := s.fetchToolJWKS(ctx, jwksURL)
	if err != nil {
		return nil, err
	}
	toolJWKSCacheMu.Lock()
	toolJWKSCache[jwksURL] = jwksCacheEntry{keys: keys, fetchedAt: time.Now()}
	toolJWKSCacheMu.Unlock()
	if k := pickJWK(keys, kid); k != nil {
		return k, nil
	}
	return nil, errors.New("no tool key matches the token's kid")
}

// toolSignedClaims parses a JWT signed by the tool of clientID (or, when
// clientID is "", the tool named by the token's iss) and returns the
// claims, the developer key and its configuration. Signature, exp and
// RS256 are verified here; callers check aud and message specifics.
func (s *LTIService) toolSignedClaims(ctx context.Context, tokenStr, clientID string) (jwt.MapClaims, *models.DeveloperKey, *models.LTIToolConfiguration, error) {
	unverified := jwt.MapClaims{}
	tok, _, err := jwt.NewParser().ParseUnverified(tokenStr, unverified)
	if err != nil {
		return nil, nil, nil, errors.New("malformed JWT")
	}
	if clientID == "" {
		clientID, _ = unverified["iss"].(string)
	}
	if clientID == "" {
		return nil, nil, nil, errors.New("JWT has no issuer")
	}
	devKey, err := s.devKeyRepo.FindByClientID(ctx, clientID)
	if err != nil || devKey == nil || devKey.WorkflowState != "active" || !devKey.IsLTIKey {
		return nil, nil, nil, errors.New("unknown client")
	}
	cfg, err := s.ltiConfigRepo.FindByDeveloperKeyID(ctx, devKey.ID)
	if err != nil || cfg == nil {
		return nil, nil, nil, errors.New("unknown client")
	}
	kid, _ := tok.Header["kid"].(string)
	pub, err := s.toolPublicKey(ctx, cfg, kid)
	if err != nil {
		return nil, nil, nil, err
	}
	claims := jwt.MapClaims{}
	_, err = jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if t.Method.Alg() != jwt.SigningMethodRS256.Alg() {
			return nil, jwt.ErrSignatureInvalid
		}
		return pub, nil
	}, jwt.WithExpirationRequired(), jwt.WithLeeway(30*time.Second))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("JWT signature or lifetime invalid: %w", err)
	}
	return claims, devKey, cfg, nil
}

// audContains reports whether the JWT aud (string or array) holds want.
func audContains(c jwt.MapClaims, want ...string) bool {
	auds, err := c.GetAudience()
	if err != nil {
		return false
	}
	for _, a := range auds {
		for _, w := range want {
			if w != "" && strings.TrimSuffix(a, "/") == strings.TrimSuffix(w, "/") {
				return true
			}
		}
	}
	return false
}

// consumeJTI makes a tool-signed JWT single use.
func (s *LTIService) consumeJTI(ctx context.Context, prefix, clientID, jti string, ttl time.Duration) error {
	if jti == "" {
		return errors.New("jti is required")
	}
	if s.nonceRepo == nil {
		return errors.New("replay protection is not configured")
	}
	k := prefix + ":" + clientID + ":" + jti
	if used, err := s.nonceRepo.Exists(ctx, k); err != nil || used {
		return errors.New("token has already been used")
	}
	if err := s.nonceRepo.Create(ctx, &models.Nonce{Value: k, ExpiresAt: time.Now().Add(ttl)}); err != nil {
		return errors.New("token has already been used")
	}
	return nil
}

// --- client_credentials grant (G127) ----------------------------------------------

// Client-credentials grant errors (RFC 6749 §5.2 error codes).
var (
	ErrLTIInvalidClient = errors.New("invalid_client")
	ErrLTIInvalidGrant  = errors.New("invalid_grant")
	ErrLTIInvalidScope  = errors.New("invalid_scope")
)

type ltiGrantError struct {
	kind error
	msg  string
}

func (e *ltiGrantError) Error() string { return e.msg }
func (e *ltiGrantError) Unwrap() error { return e.kind }

// LTIServiceGrant is a client_credentials token response.
type LTIServiceGrant struct {
	AccessToken string
	ExpiresIn   int
	Scope       string
}

const (
	clientAssertionTypeJWT = "urn:ietf:params:oauth:client-assertion-type:jwt-bearer"
	ltiServiceTokenTTL     = time.Hour
)

// GrantClientCredentials implements the LTI Advantage service token
// (IMS Security Framework §4.1): the tool authenticates with a JWT
// client_assertion signed by its own key (iss = sub = client_id, aud =
// the token endpoint, single-use jti) and receives a short-lived bearer
// token carrying the requested scopes it was granted.
func (s *LTIService) GrantClientCredentials(ctx context.Context, assertionType, assertion, scope string) (*LTIServiceGrant, error) {
	if assertionType != clientAssertionTypeJWT || assertion == "" {
		return nil, &ltiGrantError{ErrLTIInvalidClient, "client_assertion_type must be " + clientAssertionTypeJWT}
	}
	claims, devKey, cfg, err := s.toolSignedClaims(ctx, assertion, "")
	if err != nil {
		return nil, &ltiGrantError{ErrLTIInvalidClient, err.Error()}
	}
	iss, _ := claims["iss"].(string)
	sub, _ := claims["sub"].(string)
	if iss != devKey.ClientID || sub != devKey.ClientID {
		return nil, &ltiGrantError{ErrLTIInvalidClient, "iss and sub must be the client_id"}
	}
	if !audContains(claims, s.TokenEndpointURL(), s.PlatformIssuer, s.PlatformIssuer+"/login/oauth2/token") {
		return nil, &ltiGrantError{ErrLTIInvalidClient, "aud must be the token endpoint"}
	}
	jti, _ := claims["jti"].(string)
	if err := s.consumeJTI(ctx, "lti-ca", devKey.ClientID, jti, 2*time.Hour); err != nil {
		return nil, &ltiGrantError{ErrLTIInvalidGrant, err.Error()}
	}

	allowed := ParseScopeList(cfg.Scopes)
	var granted []string
	for _, sc := range strings.Fields(scope) {
		if !isLTIAdvantageScope(sc) {
			return nil, &ltiGrantError{ErrLTIInvalidScope, "unsupported scope " + sc}
		}
		if scopeListHas(allowed, sc) {
			granted = append(granted, sc)
		}
	}
	if len(granted) == 0 {
		return nil, &ltiGrantError{ErrLTIInvalidScope, "none of the requested scopes is granted to this tool"}
	}

	now := time.Now()
	tokenStr, err := s.signPlatformJWT(jwt.MapClaims{
		"iss": s.PlatformIssuer, "aud": ltiPurposeService, "sub": devKey.ClientID,
		"iat": now.Unix(), "exp": now.Add(ltiServiceTokenTTL).Unix(), "jti": ltiRandomHex(12),
		"scope": strings.Join(granted, " "), "dk": devKey.ID, "acct": devKey.AccountID,
	})
	if err != nil {
		return nil, err
	}
	return &LTIServiceGrant{AccessToken: tokenStr, ExpiresIn: int(ltiServiceTokenTTL.Seconds()), Scope: strings.Join(granted, " ")}, nil
}

// LTIServiceCaller is a tool authenticated with a service token.
type LTIServiceCaller struct {
	DevKeyID  uint
	AccountID uint
	ClientID  string
	Scopes    []string
}

// HasScope reports whether the caller holds any of the scopes.
func (c *LTIServiceCaller) HasScope(scopes ...string) bool {
	return c != nil && scopeListHas(c.Scopes, scopes...)
}

// LooksLikeLTIServiceToken is a cheap pre-check (RS256 platform JWT) used
// by the route middleware to pick the tool-auth path.
func LooksLikeLTIServiceToken(tok string) bool {
	if strings.Count(tok, ".") != 2 {
		return false
	}
	head, err := base64.RawURLEncoding.DecodeString(strings.SplitN(tok, ".", 2)[0])
	if err != nil {
		return false
	}
	var h map[string]interface{}
	if json.Unmarshal(head, &h) != nil {
		return false
	}
	alg, _ := h["alg"].(string)
	return alg == "RS256"
}

// VerifyServiceToken validates a tool's service token and re-checks that
// its developer key is still an active LTI key.
func (s *LTIService) VerifyServiceToken(ctx context.Context, tokenStr string) (*LTIServiceCaller, error) {
	s.RefreshPlatformKeys(ctx)
	c, err := s.verifyPlatformJWT(tokenStr, ltiPurposeService)
	if err != nil {
		return nil, errors.New("invalid service token")
	}
	dk := claimUint(c, "dk")
	acct := claimUint(c, "acct")
	if dk == 0 || acct == 0 {
		return nil, errors.New("invalid service token")
	}
	devKey, err := s.devKeyRepo.FindByID(ctx, dk, acct)
	if err != nil || devKey == nil || devKey.WorkflowState != "active" || !devKey.IsLTIKey {
		return nil, errors.New("developer key is no longer active")
	}
	return &LTIServiceCaller{
		DevKeyID: dk, AccountID: acct, ClientID: claimString(c, "sub"),
		Scopes: strings.Fields(claimString(c, "scope")),
	}, nil
}

// ToolInstalledInCourse reports whether the caller's tool is installed
// (active) in the course or at its tenant account, and the course is in
// the caller's tenant.
func (s *LTIService) ToolInstalledInCourse(ctx context.Context, caller *LTIServiceCaller, courseID uint) bool {
	if caller == nil || s.adv.Store == nil {
		return false
	}
	tool, err := s.adv.Store.FindActiveToolByDeveloperKeyForCourse(ctx, caller.DevKeyID, courseID, caller.AccountID)
	return err == nil && tool != nil
}

// ToolPrivacyLevel returns the privacy level of a developer key's tool
// configuration (anonymous when unknown).
func (s *LTIService) ToolPrivacyLevel(ctx context.Context, devKeyID uint) string {
	if s.ltiConfigRepo == nil {
		return LTIPrivacyAnonymous
	}
	cfg, err := s.ltiConfigRepo.FindByDeveloperKeyID(ctx, devKeyID)
	if err != nil || cfg == nil || !ValidLTIPrivacyLevel(cfg.PrivacyLevel) {
		return LTIPrivacyAnonymous
	}
	return cfg.PrivacyLevel
}
