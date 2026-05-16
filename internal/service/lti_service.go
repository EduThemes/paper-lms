package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// LTIService handles LTI 1.3 launches, OIDC flows, and platform key management.
type LTIService struct {
	devKeyRepo    repository.DeveloperKeyRepository
	ltiConfigRepo repository.LTIToolConfigurationRepository
	nonceRepo     repository.NonceRepository
	enrollRepo    repository.EnrollmentRepository
	courseRepo    repository.CourseRepository
	// Platform RSA key pair, generated once at startup
	platformKey *rsa.PrivateKey
	keyID       string
	// PlatformIssuer is the issuer URL for this LMS platform (e.g. "https://paper-lms.example.com")
	PlatformIssuer string
}

// NewLTIService creates a new LTIService and generates the platform RSA key pair.
func NewLTIService(
	devKeyRepo repository.DeveloperKeyRepository,
	ltiConfigRepo repository.LTIToolConfigurationRepository,
	nonceRepo repository.NonceRepository,
	enrollRepo repository.EnrollmentRepository,
	courseRepo repository.CourseRepository,
	platformIssuer string,
) (*LTIService, error) {
	svc := &LTIService{
		devKeyRepo:     devKeyRepo,
		ltiConfigRepo:  ltiConfigRepo,
		nonceRepo:      nonceRepo,
		enrollRepo:     enrollRepo,
		courseRepo:     courseRepo,
		PlatformIssuer: platformIssuer,
		keyID:          uuid.New().String(),
	}

	// Generate the platform RSA key pair on startup
	key, err := svc.generatePlatformKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate platform key: %w", err)
	}
	svc.platformKey = key

	return svc, nil
}

// generatePlatformKey creates a new 2048-bit RSA private key.
func (s *LTIService) generatePlatformKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
}

// GetPlatformKeyPair returns the platform's RSA private key. The key is
// generated once during service initialization and kept in memory.
func (s *LTIService) GetPlatformKeyPair() (*rsa.PrivateKey, error) {
	if s.platformKey == nil {
		return nil, errors.New("platform key has not been initialized")
	}
	return s.platformKey, nil
}

// GetJWKS returns the platform's public key formatted as a JSON Web Key Set.
// The returned map can be serialized directly to JSON for the JWKS endpoint.
func (s *LTIService) GetJWKS() (map[string]interface{}, error) {
	if s.platformKey == nil {
		return nil, errors.New("platform key has not been initialized")
	}

	jwk := rsaPublicKeyToJWK(&s.platformKey.PublicKey, s.keyID)

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

// InitiateLogin starts the OIDC third-party login flow for an LTI 1.3 launch.
// It validates the tool configuration, generates a nonce, and returns the
// redirect URL that the user's browser should be sent to.
func (s *LTIService) InitiateLogin(ctx context.Context, clientID string, loginHint string, targetLinkURI string, ltiMessageHint string) (string, error) {
	// Look up the developer key by client_id
	devKey, err := s.devKeyRepo.FindByClientID(ctx, clientID)
	if err != nil {
		return "", errors.New("invalid client_id")
	}

	if devKey.WorkflowState != "active" {
		return "", errors.New("developer key is not active")
	}

	if !devKey.IsLTIKey {
		return "", errors.New("developer key is not an LTI key")
	}

	// Look up the LTI tool configuration for this developer key
	toolConfig, err := s.ltiConfigRepo.FindByDeveloperKeyID(ctx, devKey.ID)
	if err != nil {
		return "", errors.New("LTI tool configuration not found")
	}

	// Generate a nonce
	nonceValue := uuid.New().String()
	nonce := &models.Nonce{
		Value:     nonceValue,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	if err := s.nonceRepo.Create(ctx, nonce); err != nil {
		return "", errors.New("failed to create nonce")
	}

	// Generate a state parameter
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return "", errors.New("failed to generate state")
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)

	// Build the OIDC authorization redirect URL
	// The redirect goes to the tool's OIDC initiation URL
	redirectURL, err := url.Parse(toolConfig.OIDCInitiationURL)
	if err != nil {
		return "", fmt.Errorf("invalid OIDC initiation URL: %w", err)
	}

	params := url.Values{}
	params.Set("response_type", "id_token")
	params.Set("response_mode", "form_post")
	params.Set("scope", "openid")
	params.Set("client_id", clientID)
	params.Set("redirect_uri", targetLinkURI)
	params.Set("login_hint", loginHint)
	params.Set("nonce", nonceValue)
	params.Set("state", state)
	if ltiMessageHint != "" {
		params.Set("lti_message_hint", ltiMessageHint)
	}
	params.Set("prompt", "none")

	redirectURL.RawQuery = params.Encode()

	return redirectURL.String(), nil
}

// BuildLaunchToken creates a signed JWT (id_token) containing the LTI 1.3
// launch claims. This token is signed with the platform's RSA private key
// using RS256.
func (s *LTIService) BuildLaunchToken(ctx context.Context, userID uint, courseID uint, resourceLinkID string, toolConfig *models.LTIToolConfiguration) (string, error) {
	if s.platformKey == nil {
		return "", errors.New("platform key has not been initialized")
	}

	// Look up the developer key to get the client_id (audience)
	devKey, err := s.devKeyRepo.FindByID(ctx, toolConfig.DeveloperKeyID)
	if err != nil {
		return "", errors.New("developer key not found for tool configuration")
	}

	// Look up the course for context claims
	course, err := s.courseRepo.FindByID(ctx, courseID, 0)
	if err != nil {
		return "", errors.New("course not found")
	}

	// Look up the user's enrollment to determine their role
	roles := s.getLTIRolesForUser(ctx, userID, courseID)

	// Generate a nonce for this token
	nonceValue := uuid.New().String()
	nonce := &models.Nonce{
		Value:     nonceValue,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	if err := s.nonceRepo.Create(ctx, nonce); err != nil {
		return "", errors.New("failed to create nonce")
	}

	now := time.Now()

	// Build the JWT claims
	claims := jwt.MapClaims{
		// Standard OIDC claims
		"iss":   s.PlatformIssuer,
		"sub":   strconv.FormatUint(uint64(userID), 10),
		"aud":   devKey.ClientID,
		"exp":   now.Add(1 * time.Hour).Unix(),
		"iat":   now.Unix(),
		"nonce": nonceValue,

		// LTI 1.3 required claims
		"https://purl.imsglobal.org/spec/lti/claim/message_type": "LtiResourceLinkRequest",
		"https://purl.imsglobal.org/spec/lti/claim/version":      "1.3.0",

		// Resource link claim
		"https://purl.imsglobal.org/spec/lti/claim/resource_link": map[string]interface{}{
			"id":    resourceLinkID,
			"title": toolConfig.Title,
		},

		// Roles claim
		"https://purl.imsglobal.org/spec/lti/claim/roles": roles,

		// Context claim (course information)
		"https://purl.imsglobal.org/spec/lti/claim/context": map[string]interface{}{
			"id":    strconv.FormatUint(uint64(courseID), 10),
			"label": course.CourseCode,
			"title": course.Name,
			"type":  []string{"http://purl.imsglobal.org/vocab/lis/v2/course#CourseOffering"},
		},

		// Target link URI
		"https://purl.imsglobal.org/spec/lti/claim/target_link_uri": toolConfig.TargetLinkURI,

		// Deployment ID (using the developer key ID as deployment identifier)
		"https://purl.imsglobal.org/spec/lti/claim/deployment_id": strconv.FormatUint(uint64(devKey.ID), 10),

		// Tool platform claim
		"https://purl.imsglobal.org/spec/lti/claim/tool_platform": map[string]interface{}{
			"guid":             s.PlatformIssuer,
			"name":             "Paper LMS",
			"product_family_code": "paper-lms",
			"version":          "1.0.0",
		},
	}

	// Sign with RS256
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = s.keyID

	signedToken, err := token.SignedString(s.platformKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign launch token: %w", err)
	}

	return signedToken, nil
}

// getLTIRolesForUser maps a user's enrollment in a course to LTI role URIs.
func (s *LTIService) getLTIRolesForUser(ctx context.Context, userID uint, courseID uint) []string {
	enrollment, err := s.enrollRepo.FindByUserAndCourse(ctx, userID, courseID)
	if err != nil {
		// If no enrollment found, return an empty role set
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
