package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// ltiPlatformKeyName is the lti_platform_keys.name of the active signing key.
const ltiPlatformKeyName = "default"

// ltiNextKeyName / ltiPreviousKeyName name the published upcoming and
// retired platform keys (G126 rotation).
const (
	ltiNextKeyName     = "next"
	ltiPreviousKeyName = "previous"
	// ltiStableKeyName names a stored key that is never rotated nor
	// published: the HMAC root of the LTI 1.1 opaque user_id and the
	// Basic Outcomes sourcedid, which must stay the same across platform
	// key rotations (a rotation would otherwise re-identify every LTI 1.1
	// user and invalidate every outstanding sourcedid).
	ltiStableKeyName = "lti11"
)

// LTIPlatformKeyEnvVar optionally carries the platform RSA private key as
// PEM (PKCS#1 or PKCS#8). When set it wins over the database-held key.
const LTIPlatformKeyEnvVar = "LTI_PLATFORM_PRIVATE_KEY"

// Errors returned by AuthorizeLaunch. ErrLTILaunchNotFound covers every
// "unknown / other tenant / not installed / not enrolled" case so the
// handler can answer one indistinguishable 404 (existence-leak contract).
var (
	ErrLTILaunchInvalid  = errors.New("invalid LTI authentication request")
	ErrLTILaunchNotFound = errors.New("LTI launch target not found")
)

type ltiInvalidError struct{ msg string }

func (e *ltiInvalidError) Error() string { return e.msg }
func (e *ltiInvalidError) Unwrap() error { return ErrLTILaunchInvalid }

func ltiInvalid(msg string) error { return &ltiInvalidError{msg: msg} }

// LTIService handles LTI 1.3 launches, OIDC flows, and platform key management.
type LTIService struct {
	devKeyRepo    repository.DeveloperKeyRepository
	ltiConfigRepo repository.LTIToolConfigurationRepository
	nonceRepo     repository.NonceRepository
	enrollRepo    repository.EnrollmentRepository
	courseRepo    repository.CourseRepository
	toolRepo      repository.ContextExternalToolRepository

	// Platform RSA key pair. NewLTIService installs an ephemeral key;
	// LoadPersistentPlatformKey swaps in the shared, persisted one (B181).
	keyMu       sync.RWMutex
	platformKey *rsa.PrivateKey
	keyID       string
	// extraKeys are the published-but-not-signing keys (G126): the
	// upcoming "next" key (published ahead of a rotation so tools have it
	// cached) and the retired "previous" key (still verifiable while
	// tokens signed with it are in flight).
	extraKeys []ltiSigningKey
	// stableKey is the never-rotated "lti11" key (nil with an env key).
	stableKey   *rsa.PrivateKey
	keyLoadedAt time.Time
	keyRepo     repository.LTIPlatformKeyRepository
	keySeal     func([]byte) ([]byte, error)
	keyOpen     func([]byte) ([]byte, error)
	envKey      bool
	// PlatformIssuer is the issuer URL for this LMS platform (e.g. "https://paper-lms.example.com")
	PlatformIssuer string

	// adv holds the LTI Advantage collaborators (G123-G134), wired with
	// SetAdvantageDeps. Nil members disable the matching feature.
	adv LTIAdvantageDeps
	// devKeyCreator creates the developer key of a tool registration.
	devKeyCreator DeveloperKeyCreator
	// launchGate applies module locks and assignment availability to a
	// learner's launch (SetLaunchGate). Nil = publish checks only.
	launchGate LTILaunchGate
}

// ltiSigningKey is one platform RSA key and its kid.
type ltiSigningKey struct {
	key *rsa.PrivateKey
	kid string
}

// NewLTIService creates a new LTIService with an ephemeral platform key.
// Production calls LoadPersistentPlatformKey right after so every replica
// signs with the same, restart-stable key.
func NewLTIService(
	devKeyRepo repository.DeveloperKeyRepository,
	ltiConfigRepo repository.LTIToolConfigurationRepository,
	nonceRepo repository.NonceRepository,
	enrollRepo repository.EnrollmentRepository,
	courseRepo repository.CourseRepository,
	toolRepo repository.ContextExternalToolRepository,
	platformIssuer string,
) (*LTIService, error) {
	svc := &LTIService{
		devKeyRepo:     devKeyRepo,
		ltiConfigRepo:  ltiConfigRepo,
		nonceRepo:      nonceRepo,
		enrollRepo:     enrollRepo,
		courseRepo:     courseRepo,
		toolRepo:       toolRepo,
		PlatformIssuer: platformIssuer,
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate platform key: %w", err)
	}
	svc.setPlatformKey(key)

	return svc, nil
}

func (s *LTIService) setPlatformKey(key *rsa.PrivateKey) {
	s.keyMu.Lock()
	defer s.keyMu.Unlock()
	s.platformKey = key
	s.keyID = JWKThumbprint(&key.PublicKey)
}

func (s *LTIService) currentKey() (*rsa.PrivateKey, string) {
	s.keyMu.RLock()
	defer s.keyMu.RUnlock()
	return s.platformKey, s.keyID
}

// LoadPersistentPlatformKey installs the platform signing key shared by
// every replica (B181). Order of precedence:
//
//  1. LTI_PLATFORM_PRIVATE_KEY (PEM) — operator-managed key.
//  2. The lti_platform_keys row, sealed with secretbox (open/seal are
//     auth.Decrypt/auth.Encrypt; injected to avoid an import cycle).
//  3. None yet: generate one, seal it and insert it. A concurrent replica
//     may win the insert; the row is then re-read, so all replicas
//     converge on one key.
//
// The kid is the RFC 7638 thumbprint of the public key, so it is a pure
// function of the key: identical on every pod and across restarts.
func (s *LTIService) LoadPersistentPlatformKey(
	ctx context.Context,
	repo repository.LTIPlatformKeyRepository,
	envPEM string,
	seal func([]byte) ([]byte, error),
	open func([]byte) ([]byte, error),
) error {
	if strings.TrimSpace(envPEM) != "" {
		key, err := parseRSAPrivateKeyPEM([]byte(envPEM))
		if err != nil {
			return fmt.Errorf("%s: %w", LTIPlatformKeyEnvVar, err)
		}
		s.setPlatformKey(key)
		s.keyMu.Lock()
		s.envKey = true
		s.keyMu.Unlock()
		return nil
	}
	if repo == nil {
		return errors.New("no LTI platform key store configured")
	}

	if err := s.ensureStoredKey(ctx, repo, ltiPlatformKeyName, seal); err != nil {
		return err
	}
	// G126: publish an upcoming key from the start, so the first rotation
	// promotes a key tools have already cached from the JWKS.
	if err := s.ensureStoredKey(ctx, repo, ltiNextKeyName, seal); err != nil {
		return err
	}
	if err := s.ensureStoredKey(ctx, repo, ltiStableKeyName, seal); err != nil {
		return err
	}

	s.keyMu.Lock()
	s.keyRepo = repo
	s.keySeal = seal
	s.keyOpen = open
	s.keyMu.Unlock()
	return s.reloadPlatformKeys(ctx)
}

// ensureStoredKey inserts a freshly generated, sealed key under name
// unless one already exists (another replica may win the insert).
func (s *LTIService) ensureStoredKey(ctx context.Context, repo repository.LTIPlatformKeyRepository, name string, seal func([]byte) ([]byte, error)) error {
	if _, err := repo.FindByName(ctx, name); err == nil {
		return nil
	}
	row, err := newSealedPlatformKey(name, seal)
	if err != nil {
		return err
	}
	if err := repo.CreateIfAbsent(ctx, row); err != nil {
		return fmt.Errorf("store platform key: %w", err)
	}
	return nil
}

// newSealedPlatformKey generates an RSA-2048 key and seals its PKCS#8 DER.
func newSealedPlatformKey(name string, seal func([]byte) ([]byte, error)) (*models.LTIPlatformKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate platform key: %w", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshal platform key: %w", err)
	}
	sealed, err := seal(der)
	if err != nil {
		return nil, fmt.Errorf("seal platform key: %w", err)
	}
	return &models.LTIPlatformKey{Name: name, KeyID: JWKThumbprint(&key.PublicKey), PrivateKeyEncrypted: sealed}, nil
}

func unsealPlatformKey(row *models.LTIPlatformKey, open func([]byte) ([]byte, error)) (*rsa.PrivateKey, error) {
	der, err := open(row.PrivateKeyEncrypted)
	if err != nil {
		return nil, fmt.Errorf("unseal platform key: %w", err)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse platform key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("platform key is not RSA")
	}
	return key, nil
}

// reloadPlatformKeys re-reads the stored keys: "default" signs, "next"
// and "previous" are published and accepted for verification.
func (s *LTIService) reloadPlatformKeys(ctx context.Context) error {
	s.keyMu.RLock()
	repo, open := s.keyRepo, s.keyOpen
	s.keyMu.RUnlock()
	if repo == nil || open == nil {
		return nil
	}
	row, err := repo.FindByName(ctx, ltiPlatformKeyName)
	if err != nil {
		return fmt.Errorf("reload platform key: %w", err)
	}
	current, err := unsealPlatformKey(row, open)
	if err != nil {
		return err
	}
	var extras []ltiSigningKey
	for _, name := range []string{ltiNextKeyName, ltiPreviousKeyName} {
		r, findErr := repo.FindByName(ctx, name)
		if findErr != nil {
			continue
		}
		k, openErr := unsealPlatformKey(r, open)
		if openErr != nil {
			continue
		}
		extras = append(extras, ltiSigningKey{key: k, kid: JWKThumbprint(&k.PublicKey)})
	}
	var stable *rsa.PrivateKey
	if r, findErr := repo.FindByName(ctx, ltiStableKeyName); findErr == nil {
		if k, openErr := unsealPlatformKey(r, open); openErr == nil {
			stable = k
		}
	}
	s.keyMu.Lock()
	s.platformKey = current
	s.keyID = JWKThumbprint(&current.PublicKey)
	s.extraKeys = extras
	if stable != nil {
		s.stableKey = stable
	}
	s.keyLoadedAt = time.Now()
	s.keyMu.Unlock()
	return nil
}

// ltiKeyRefreshInterval bounds how stale a replica's view of the stored
// keys may get after another replica rotates them.
const ltiKeyRefreshInterval = time.Minute

// RefreshPlatformKeys reloads the stored keys when the cached copy is
// older than ltiKeyRefreshInterval, so a rotation done on one replica
// reaches every replica within a minute. A no-op for the env-provided
// key or when no key store is wired.
func (s *LTIService) RefreshPlatformKeys(ctx context.Context) {
	s.keyMu.RLock()
	stale := s.keyRepo != nil && !s.envKey && time.Since(s.keyLoadedAt) > ltiKeyRefreshInterval
	s.keyMu.RUnlock()
	if stale {
		_ = s.reloadPlatformKeys(ctx)
	}
}

// ErrLTIKeyRotationUnavailable is returned when the platform key cannot
// be rotated (operator-managed env key, or no rotating key store).
var ErrLTIKeyRotationUnavailable = errors.New("platform key rotation is not available")

// RotatePlatformKey performs a key rotation (G126): the current key is
// retired to "previous" (still published and verifiable), the published
// "next" key starts signing, and a new "next" key is generated and
// published. Returns the kid of the new signing key.
func (s *LTIService) RotatePlatformKey(ctx context.Context) (string, error) {
	s.keyMu.RLock()
	repo, seal, envKey := s.keyRepo, s.keySeal, s.envKey
	s.keyMu.RUnlock()
	rotator, ok := repo.(repository.LTIPlatformKeyRotator)
	if envKey || !ok || seal == nil {
		return "", ErrLTIKeyRotationUnavailable
	}
	newNext, err := newSealedPlatformKey(ltiNextKeyName, seal)
	if err != nil {
		return "", err
	}
	fallback, err := newSealedPlatformKey(ltiPlatformKeyName, seal)
	if err != nil {
		return "", err
	}
	if err := rotator.Rotate(ctx, newNext, fallback); err != nil {
		return "", fmt.Errorf("rotate platform key: %w", err)
	}
	if err := s.reloadPlatformKeys(ctx); err != nil {
		return "", err
	}
	_, kid := s.currentKey()
	return kid, nil
}

// PlatformKeyStatus describes the published keys for the admin UI.
type PlatformKeyStatus struct {
	CurrentKID    string   `json:"current_kid"`
	PublishedKIDs []string `json:"published_kids"`
	Rotatable     bool     `json:"rotatable"`
}

// GetPlatformKeyStatus reports the signing kid and every published kid.
func (s *LTIService) GetPlatformKeyStatus() PlatformKeyStatus {
	s.keyMu.RLock()
	defer s.keyMu.RUnlock()
	st := PlatformKeyStatus{CurrentKID: s.keyID, PublishedKIDs: []string{s.keyID}}
	for _, k := range s.extraKeys {
		st.PublishedKIDs = append(st.PublishedKIDs, k.kid)
	}
	_, rotatable := s.keyRepo.(repository.LTIPlatformKeyRotator)
	st.Rotatable = rotatable && !s.envKey && s.keySeal != nil
	return st
}

// publicKeyForKID returns the published platform public key with kid.
func (s *LTIService) publicKeyForKID(kid string) *rsa.PublicKey {
	s.keyMu.RLock()
	defer s.keyMu.RUnlock()
	if s.platformKey != nil && (kid == "" || kid == s.keyID) {
		return &s.platformKey.PublicKey
	}
	for _, k := range s.extraKeys {
		if k.kid == kid {
			return &k.key.PublicKey
		}
	}
	return nil
}

func parseRSAPrivateKeyPEM(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("not PEM encoded")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("not an RSA private key (PKCS#1 or PKCS#8)")
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("not an RSA private key")
	}
	return key, nil
}

// JWKThumbprint computes the RFC 7638 SHA-256 thumbprint of an RSA public
// key, base64url-encoded. Used as the JWKS kid.
func JWKThumbprint(pub *rsa.PublicKey) string {
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	canonical := `{"e":"` + e + `","kty":"RSA","n":"` + n + `"}`
	sum := sha256.Sum256([]byte(canonical))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// GetPlatformKeyPair returns the platform's RSA private key.
func (s *LTIService) GetPlatformKeyPair() (*rsa.PrivateKey, error) {
	key, _ := s.currentKey()
	if key == nil {
		return nil, errors.New("platform key has not been initialized")
	}
	return key, nil
}

// GetJWKS returns the platform's public key formatted as a JSON Web Key Set.
// The returned map can be serialized directly to JSON for the JWKS endpoint.
func (s *LTIService) GetJWKS() (map[string]interface{}, error) {
	s.keyMu.RLock()
	defer s.keyMu.RUnlock()
	if s.platformKey == nil {
		return nil, errors.New("platform key has not been initialized")
	}

	// The signing key is always first; the published next/previous keys
	// follow (G126).
	keys := []interface{}{rsaPublicKeyToJWK(&s.platformKey.PublicKey, s.keyID)}
	for _, k := range s.extraKeys {
		keys = append(keys, rsaPublicKeyToJWK(&k.key.PublicKey, k.kid))
	}

	return map[string]interface{}{
		"keys": keys,
	}, nil
}

// rsaPublicKeyToJWK converts an RSA public key to a JWK representation.
func rsaPublicKeyToJWK(key *rsa.PublicKey, kid string) map[string]interface{} {
	return map[string]interface{}{
		"kty": "RSA",
		"kid": kid,
		"use": "sig",
		"alg": "RS256",
		"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}
}

// resolveLTIKey loads an active LTI developer key of the caller's tenant
// and its tool configuration.
func (s *LTIService) resolveLTIKey(ctx context.Context, clientID string, accountID uint) (*models.DeveloperKey, *models.LTIToolConfiguration, error) {
	devKey, err := s.devKeyRepo.FindByClientID(ctx, clientID)
	if err != nil || devKey.WorkflowState != "active" || !devKey.IsLTIKey {
		return nil, nil, ErrLTILaunchNotFound
	}
	if accountID == 0 || devKey.AccountID != accountID {
		return nil, nil, ErrLTILaunchNotFound
	}
	toolConfig, err := s.ltiConfigRepo.FindByDeveloperKeyID(ctx, devKey.ID)
	if err != nil {
		return nil, nil, ErrLTILaunchNotFound
	}
	return devKey, toolConfig, nil
}

// InitiateLogin starts the LTI 1.3 third-party-initiated login (OIDC login
// initiation, LTI Core §5.1.1 / IMS Security Framework §5.1.1). The
// browser is sent to the tool's OIDC initiation URL carrying exactly the
// initiation parameters: iss, login_hint, target_link_uri, plus
// lti_message_hint, client_id and lti_deployment_id.
//
// B083: the previous implementation sent authorization-request parameters
// (response_type, scope, nonce, state, prompt, redirect_uri) and no iss,
// which standard tool libraries reject. The tool generates nonce and state
// itself and comes back to Paper's authorization endpoint
// (GET|POST /api/v1/lti/authorize).
func (s *LTIService) InitiateLogin(ctx context.Context, accountID uint, clientID string, loginHint string, targetLinkURI string, ltiMessageHint string) (string, error) {
	devKey, toolConfig, err := s.resolveLTIKey(ctx, clientID, accountID)
	if err != nil {
		return "", errors.New("invalid client_id")
	}

	if targetLinkURI == "" {
		targetLinkURI = toolConfig.TargetLinkURI
	}

	redirectURL, err := url.Parse(toolConfig.OIDCInitiationURL)
	if err != nil || redirectURL.Scheme == "" {
		return "", errors.New("invalid OIDC initiation URL")
	}

	params := redirectURL.Query()
	params.Set("iss", s.PlatformIssuer)
	params.Set("login_hint", loginHint)
	params.Set("target_link_uri", targetLinkURI)
	params.Set("client_id", clientID)
	params.Set("lti_deployment_id", strconv.FormatUint(uint64(devKey.ID), 10))
	if ltiMessageHint != "" {
		params.Set("lti_message_hint", ltiMessageHint)
	}
	redirectURL.RawQuery = params.Encode()

	return redirectURL.String(), nil
}

// LTIAuthRequest is the OIDC authentication request a tool sends back to
// the platform's authorization endpoint (IMS Security Framework §5.1.1.2).
type LTIAuthRequest struct {
	UserID    uint
	AccountID uint
	// IsAdmin is the DB-verified tenant-admin flag; admins may launch a
	// tool installed in any course of their tenant without an enrollment.
	IsAdmin bool

	Scope          string
	ResponseType   string
	ResponseMode   string
	ClientID       string
	RedirectURI    string
	Nonce          string
	State          string
	Prompt         string
	LTIMessageHint string
}

// LTIAuthResponse is what the handler auto-posts to the tool.
type LTIAuthResponse struct {
	RedirectURI string
	IDToken     string
	State       string
}

// AuthorizeLaunch validates an OIDC authentication request and, when the
// user may launch the tool in the hinted course, returns the signed
// id_token to form_post to the tool's redirect_uri.
//
// B083: nonce and state come from the tool's request and are echoed back
// (the id_token carries the tool's nonce; state is posted alongside).
// B186: the course must be in the caller's tenant, the caller must be an
// active member of it (or a tenant admin), and the tool's developer key
// must be active, in the same tenant and actually installed (active) in
// that course or the tenant account.
func (s *LTIService) AuthorizeLaunch(ctx context.Context, req LTIAuthRequest) (*LTIAuthResponse, error) {
	if req.Scope != "openid" {
		return nil, ltiInvalid("scope must be openid")
	}
	if req.ResponseType != "id_token" {
		return nil, ltiInvalid("response_type must be id_token")
	}
	if req.ResponseMode != "" && req.ResponseMode != "form_post" {
		return nil, ltiInvalid("response_mode must be form_post")
	}
	if req.Prompt != "" && req.Prompt != "none" {
		return nil, ltiInvalid("prompt must be none")
	}
	if req.Nonce == "" {
		return nil, ltiInvalid("nonce is required")
	}
	if req.RedirectURI == "" {
		return nil, ltiInvalid("redirect_uri is required")
	}
	s.RefreshPlatformKeys(ctx)

	// G124/G125: launches started by Paper's own launch endpoint carry a
	// platform-signed hint with the full launch context.
	if IsSignedLTIMessageHint(req.LTIMessageHint) {
		if s.adv.Store == nil {
			return nil, ltiInvalid("invalid lti_message_hint")
		}
		return s.authorizeHintLaunch(ctx, req)
	}

	courseID, resourceLinkID, hintKeyID, err := ParseLTIMessageHint(req.LTIMessageHint)
	if err != nil {
		return nil, ltiInvalid("invalid lti_message_hint")
	}

	devKey, toolConfig, err := s.resolveLTIKey(ctx, req.ClientID, req.AccountID)
	if err != nil {
		return nil, err
	}
	if hintKeyID != 0 && hintKeyID != devKey.ID {
		return nil, ltiInvalid("lti_message_hint does not match client_id")
	}

	// The id_token may only be posted to a URI registered for the tool.
	if req.RedirectURI != toolConfig.TargetLinkURI && !DeveloperKeyAllowsRedirect(devKey, req.RedirectURI) {
		return nil, ltiInvalid("redirect_uri is not registered for this tool")
	}

	if err := s.authorizeCourseLaunch(ctx, req.UserID, req.AccountID, req.IsAdmin, courseID, devKey.ID); err != nil {
		return nil, err
	}

	// Replay guard: a tool nonce is single-use.
	if err := s.consumeToolNonce(ctx, req.ClientID, req.Nonce); err != nil {
		return nil, err
	}

	idToken, err := s.BuildLaunchToken(ctx, req.UserID, req.AccountID, courseID, resourceLinkID, req.Nonce, devKey, toolConfig)
	if err != nil {
		return nil, err
	}

	return &LTIAuthResponse{RedirectURI: req.RedirectURI, IDToken: idToken, State: req.State}, nil
}

// authorizeCourseLaunch enforces the B186 course/tenant/install checks.
func (s *LTIService) authorizeCourseLaunch(ctx context.Context, userID, accountID uint, isAdmin bool, courseID, devKeyID uint) error {
	if _, err := s.courseRepo.FindByID(ctx, courseID, accountID); err != nil {
		return ErrLTILaunchNotFound
	}
	if !isAdmin {
		enrollment, err := s.enrollRepo.FindByUserAndCourse(ctx, userID, courseID, accountID)
		if err != nil || enrollment == nil || enrollment.WorkflowState != "active" {
			return ErrLTILaunchNotFound
		}
	}
	if !s.toolInstalled(ctx, devKeyID, courseID, accountID) {
		return ErrLTILaunchNotFound
	}
	return nil
}

// toolInstalled reports whether an active ContextExternalTool for the key
// is installed in the course or in the tenant's account.
func (s *LTIService) toolInstalled(ctx context.Context, devKeyID, courseID, accountID uint) bool {
	if s.toolRepo == nil {
		return false
	}
	contexts := []struct {
		typ string
		id  uint
	}{{"Course", courseID}, {"Account", accountID}}
	for _, ctxRef := range contexts {
		for page := 1; ; page++ {
			res, err := s.toolRepo.ListByContext(ctx, ctxRef.typ, ctxRef.id, repository.PaginationParams{Page: page, PerPage: 100})
			if err != nil || res == nil {
				break
			}
			for _, tool := range res.Items {
				if tool.DeveloperKeyID == devKeyID && tool.WorkflowState == "active" {
					return true
				}
			}
			if int64(page*100) >= res.TotalCount || len(res.Items) == 0 {
				break
			}
		}
	}
	return false
}

// ParseLTIMessageHint parses the platform's lti_message_hint, formatted
// "courseID:resourceLinkID:developerKeyID" (the last two parts optional).
func ParseLTIMessageHint(hint string) (courseID uint, resourceLinkID string, devKeyID uint, err error) {
	if hint == "" {
		return 0, "", 0, errors.New("empty message hint")
	}
	parts := strings.SplitN(hint, ":", 3)

	cid, parseErr := strconv.ParseUint(parts[0], 10, 64)
	if parseErr != nil || cid == 0 {
		return 0, "", 0, errors.New("invalid course ID in message hint")
	}
	courseID = uint(cid)

	if len(parts) >= 2 {
		resourceLinkID = parts[1]
	}
	if len(parts) >= 3 && parts[2] != "" {
		dkid, parseErr := strconv.ParseUint(parts[2], 10, 64)
		if parseErr != nil {
			return 0, "", 0, errors.New("invalid developer key ID in message hint")
		}
		devKeyID = uint(dkid)
	}
	return courseID, resourceLinkID, devKeyID, nil
}

// BuildLaunchToken creates a signed JWT (id_token) for a legacy launch
// (the unsigned "course:link:key" message hint of POST /lti/oidc/login),
// signed RS256 with the platform key. nonce is the tool's nonce from the
// authentication request. The claims are the same as a hint launch
// (G125), with the developer key ID as deployment_id.
func (s *LTIService) BuildLaunchToken(ctx context.Context, userID, accountID, courseID uint, resourceLinkID, nonce string, devKey *models.DeveloperKey, toolConfig *models.LTIToolConfiguration) (string, error) {
	return s.buildIDToken(ctx, launchContext{
		hint: ltiLaunchHint{
			UserID: userID, AccountID: accountID, CourseID: courseID, DevKeyID: devKey.ID,
			MessageType: LTIMessageResourceLink, ResourceLinkUUID: resourceLinkID,
			TargetLinkURI: toolConfig.TargetLinkURI,
		},
		nonce: nonce, devKey: devKey, cfg: toolConfig,
	})
}

// IsSignedLTIMessageHint reports whether hint is a platform-signed hint
// (as opposed to the legacy "course:link:key" format).
func IsSignedLTIMessageHint(hint string) bool { return looksLikeJWT(hint) }

// mapEnrollmentTypeToLTIRoles converts a Canvas-style enrollment type string
// to one or more LTI 1.3 context role URIs.
func mapEnrollmentTypeToLTIRoles(enrollmentType string) []string {
	switch enrollmentType {
	case "StudentEnrollment":
		return []string{ltiRoleLearner}
	case "TeacherEnrollment":
		return []string{ltiRoleInstructor}
	case "TaEnrollment":
		return []string{ltiRoleTA, ltiRoleInstructor}
	case "ObserverEnrollment":
		return []string{ltiRoleMentor}
	case "DesignerEnrollment":
		return []string{ltiRoleContentDev}
	default:
		return []string{}
	}
}
