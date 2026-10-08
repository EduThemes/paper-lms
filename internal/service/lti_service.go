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

	"github.com/golang-jwt/jwt/v5"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// ltiPlatformKeyName is the lti_platform_keys.name of the active signing key.
const ltiPlatformKeyName = "default"

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
	// PlatformIssuer is the issuer URL for this LMS platform (e.g. "https://paper-lms.example.com")
	PlatformIssuer string
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
		return nil
	}
	if repo == nil {
		return errors.New("no LTI platform key store configured")
	}

	if row, err := repo.FindByName(ctx, ltiPlatformKeyName); err == nil {
		return s.installSealedKey(row, open)
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("generate platform key: %w", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshal platform key: %w", err)
	}
	sealed, err := seal(der)
	if err != nil {
		return fmt.Errorf("seal platform key: %w", err)
	}
	if err := repo.CreateIfAbsent(ctx, &models.LTIPlatformKey{
		Name:                ltiPlatformKeyName,
		KeyID:               JWKThumbprint(&key.PublicKey),
		PrivateKeyEncrypted: sealed,
	}); err != nil {
		return fmt.Errorf("store platform key: %w", err)
	}

	// Re-read: if another replica inserted first, use its key.
	row, err := repo.FindByName(ctx, ltiPlatformKeyName)
	if err != nil {
		return fmt.Errorf("reload platform key: %w", err)
	}
	return s.installSealedKey(row, open)
}

func (s *LTIService) installSealedKey(row *models.LTIPlatformKey, open func([]byte) ([]byte, error)) error {
	der, err := open(row.PrivateKeyEncrypted)
	if err != nil {
		return fmt.Errorf("unseal platform key: %w", err)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return fmt.Errorf("parse platform key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return errors.New("platform key is not RSA")
	}
	s.setPlatformKey(key)
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
	key, kid := s.currentKey()
	if key == nil {
		return nil, errors.New("platform key has not been initialized")
	}

	jwk := rsaPublicKeyToJWK(&key.PublicKey, kid)

	return map[string]interface{}{
		"keys": []interface{}{jwk},
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
	nonceKey := "lti:" + req.ClientID + ":" + req.Nonce
	if used, err := s.nonceRepo.Exists(ctx, nonceKey); err != nil || used {
		return nil, ltiInvalid("nonce has already been used")
	}
	if err := s.nonceRepo.Create(ctx, &models.Nonce{Value: nonceKey, ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		return nil, ltiInvalid("nonce has already been used")
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

// BuildLaunchToken creates a signed JWT (id_token) containing the LTI 1.3
// launch claims, signed RS256 with the platform key. nonce is the tool's
// nonce from the authentication request. The course is resolved under the
// caller's tenant (accountID).
func (s *LTIService) BuildLaunchToken(ctx context.Context, userID, accountID, courseID uint, resourceLinkID, nonce string, devKey *models.DeveloperKey, toolConfig *models.LTIToolConfiguration) (string, error) {
	key, kid := s.currentKey()
	if key == nil {
		return "", errors.New("platform key has not been initialized")
	}

	course, err := s.courseRepo.FindByID(ctx, courseID, accountID)
	if err != nil {
		return "", ErrLTILaunchNotFound
	}

	roles := s.getLTIRolesForUser(ctx, userID, courseID, accountID)

	now := time.Now()

	claims := jwt.MapClaims{
		// Standard OIDC claims
		"iss":   s.PlatformIssuer,
		"sub":   strconv.FormatUint(uint64(userID), 10),
		"aud":   devKey.ClientID,
		"azp":   devKey.ClientID,
		"exp":   now.Add(1 * time.Hour).Unix(),
		"iat":   now.Unix(),
		"nonce": nonce,

		// LTI 1.3 required claims
		"https://purl.imsglobal.org/spec/lti/claim/message_type": "LtiResourceLinkRequest",
		"https://purl.imsglobal.org/spec/lti/claim/version":      "1.3.0",

		"https://purl.imsglobal.org/spec/lti/claim/resource_link": map[string]interface{}{
			"id":    resourceLinkID,
			"title": toolConfig.Title,
		},

		"https://purl.imsglobal.org/spec/lti/claim/roles": roles,

		"https://purl.imsglobal.org/spec/lti/claim/context": map[string]interface{}{
			"id":    strconv.FormatUint(uint64(courseID), 10),
			"label": course.CourseCode,
			"title": course.Name,
			"type":  []string{"http://purl.imsglobal.org/vocab/lis/v2/course#CourseOffering"},
		},

		"https://purl.imsglobal.org/spec/lti/claim/target_link_uri": toolConfig.TargetLinkURI,

		// Deployment ID (the developer key ID is the deployment identifier)
		"https://purl.imsglobal.org/spec/lti/claim/deployment_id": strconv.FormatUint(uint64(devKey.ID), 10),

		"https://purl.imsglobal.org/spec/lti/claim/tool_platform": map[string]interface{}{
			"guid":                s.PlatformIssuer,
			"name":                "Paper LMS",
			"product_family_code": "paper-lms",
			"version":             "1.0.0",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid

	signedToken, err := token.SignedString(key)
	if err != nil {
		return "", fmt.Errorf("failed to sign launch token: %w", err)
	}

	return signedToken, nil
}

// getLTIRolesForUser maps a user's enrollment in a course to LTI role URIs.
func (s *LTIService) getLTIRolesForUser(ctx context.Context, userID, courseID, accountID uint) []string {
	enrollment, err := s.enrollRepo.FindByUserAndCourse(ctx, userID, courseID, accountID)
	if err != nil || enrollment == nil {
		return []string{}
	}

	return mapEnrollmentTypeToLTIRoles(enrollment.Type)
}

// mapEnrollmentTypeToLTIRoles converts a Canvas-style enrollment type string
// to one or more LTI 1.3 role URIs.
func mapEnrollmentTypeToLTIRoles(enrollmentType string) []string {
	switch enrollmentType {
	case "StudentEnrollment":
		return []string{
			"http://purl.imsglobal.org/vocab/lis/v2/membership#Learner",
		}
	case "TeacherEnrollment":
		return []string{
			"http://purl.imsglobal.org/vocab/lis/v2/membership#Instructor",
		}
	case "TaEnrollment":
		return []string{
			"http://purl.imsglobal.org/vocab/lis/v2/membership/Instructor#TeachingAssistant",
		}
	case "ObserverEnrollment":
		return []string{
			"http://purl.imsglobal.org/vocab/lis/v2/membership#Mentor",
		}
	case "DesignerEnrollment":
		return []string{
			"http://purl.imsglobal.org/vocab/lis/v2/membership#ContentDeveloper",
		}
	default:
		return []string{}
	}
}
