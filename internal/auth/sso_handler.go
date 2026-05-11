package auth

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/EduThemes/paper-lms/internal/config"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// SSOHandler ties together all SSO protocol handlers (SAML, LDAP, CAS) and
// provides Fiber-compatible HTTP handlers for each authentication flow.
type SSOHandler struct {
	samlHandler      *SAMLHandler
	ldapAuth         *LDAPAuthenticator
	casAuth          *CASAuthenticator
	userRepo         repository.UserRepository
	authProviderRepo repository.AuthenticationProviderRepository
	config           *config.Config
}

// NewSSOHandler creates a new SSOHandler with all protocol handlers wired up.
func NewSSOHandler(
	samlHandler *SAMLHandler,
	ldapAuth *LDAPAuthenticator,
	casAuth *CASAuthenticator,
	userRepo repository.UserRepository,
	authProviderRepo repository.AuthenticationProviderRepository,
	cfg *config.Config,
) *SSOHandler {
	return &SSOHandler{
		samlHandler:      samlHandler,
		ldapAuth:         ldapAuth,
		casAuth:          casAuth,
		userRepo:         userRepo,
		authProviderRepo: authProviderRepo,
		config:           cfg,
	}
}

// HandleSAMLLogin initiates a SAML login flow by redirecting to the IDP.
// GET /api/v1/auth/saml/login?provider_id=:id
func (h *SSOHandler) HandleSAMLLogin(c *fiber.Ctx) error {
	providerID, err := strconv.Atoi(c.Query("provider_id"))
	if err != nil || providerID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "provider_id query parameter is required"}},
		})
	}

	provider, err := h.authProviderRepo.FindByID(c.Context(), uint(providerID))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "Authentication provider not found"}},
		})
	}

	if provider.AuthType != "saml" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "Provider is not a SAML provider"}},
		})
	}

	if provider.WorkflowState != "active" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "Authentication provider is not active"}},
		})
	}

	// If the SAML handler was initialized with a global IDP URL, use it.
	// Otherwise, override with the provider-specific IDP URL.
	if provider.LogInURL != "" {
		c.Request().URI().QueryArgs().Set("idp_url", provider.LogInURL)
	}

	return h.samlHandler.InitiateLogin(c)
}

// HandleSAMLACS handles the SAML Assertion Consumer Service callback.
// POST /api/v1/auth/saml/acs
func (h *SSOHandler) HandleSAMLACS(c *fiber.Ctx) error {
	return h.samlHandler.HandleACS(c)
}

// HandleSAMLMetadata serves the SAML SP metadata XML document.
// GET /api/v1/auth/saml/metadata
func (h *SSOHandler) HandleSAMLMetadata(c *fiber.Ctx) error {
	metadata, err := h.samlHandler.GenerateMetadata()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "Failed to generate SAML metadata"}},
		})
	}

	c.Set("Content-Type", "application/xml")
	return c.Send(metadata)
}

// HandleCASLogin initiates a CAS login flow by redirecting to the CAS server.
// GET /api/v1/auth/cas/login?provider_id=:id
func (h *SSOHandler) HandleCASLogin(c *fiber.Ctx) error {
	providerID, err := strconv.Atoi(c.Query("provider_id"))
	if err != nil || providerID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "provider_id query parameter is required"}},
		})
	}

	provider, err := h.authProviderRepo.FindByID(c.Context(), uint(providerID))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "Authentication provider not found"}},
		})
	}

	if provider.AuthType != "cas" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "Provider is not a CAS provider"}},
		})
	}

	if provider.WorkflowState != "active" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "Authentication provider is not active"}},
		})
	}

	return h.casAuth.InitiateLogin(c, provider)
}

// HandleCASCallback handles the CAS ticket validation callback.
// GET /api/v1/auth/cas/callback?ticket=:ticket&provider_id=:id
func (h *SSOHandler) HandleCASCallback(c *fiber.Ctx) error {
	providerID, err := strconv.Atoi(c.Query("provider_id"))
	if err != nil || providerID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "provider_id query parameter is required"}},
		})
	}

	ticket := c.Query("ticket")
	if ticket == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "CAS ticket is required"}},
		})
	}

	provider, err := h.authProviderRepo.FindByID(c.Context(), uint(providerID))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "Authentication provider not found"}},
		})
	}

	if provider.AuthType != "cas" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "Provider is not a CAS provider"}},
		})
	}

	// Reconstruct the service URL (the URL CAS redirected back to, minus the ticket param)
	scheme := "https"
	if c.Protocol() == "http" {
		scheme = "http"
	}
	serviceURL := fmt.Sprintf("%s://%s/api/v1/auth/cas/callback?provider_id=%d", scheme, c.Hostname(), provider.ID)

	user, err := h.casAuth.ValidateTicket(c.Context(), provider, ticket, serviceURL)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "CAS authentication failed: " + err.Error()}},
		})
	}

	// Generate JWT
	token, err := generateJWT(user, h.config.JWTSecret)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "Failed to generate authentication token"}},
		})
	}

	// Redirect to frontend with token
	frontendURL := h.config.FrontendURL
	separator := "?"
	if strings.Contains(frontendURL, "?") {
		separator = "&"
	}

	return c.Redirect(frontendURL+separator+"token="+url.QueryEscape(token), fiber.StatusFound)
}

// HandleLDAPLogin authenticates a user via LDAP using username/password from the request body.
// POST /api/v1/auth/ldap/login
// Body: {"provider_id": 1, "username": "...", "password": "..."}
func (h *SSOHandler) HandleLDAPLogin(c *fiber.Ctx) error {
	var input struct {
		ProviderID uint   `json:"provider_id"`
		Username   string `json:"username"`
		Password   string `json:"password"`
	}

	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "Invalid request body"}},
		})
	}

	if input.ProviderID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "provider_id is required"}},
		})
	}
	if input.Username == "" || input.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "username and password are required"}},
		})
	}

	provider, err := h.authProviderRepo.FindByID(c.Context(), input.ProviderID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "Authentication provider not found"}},
		})
	}

	if provider.AuthType != "ldap" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "Provider is not an LDAP provider"}},
		})
	}

	if provider.WorkflowState != "active" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "Authentication provider is not active"}},
		})
	}

	user, err := h.ldapAuth.Authenticate(c.Context(), provider, input.Username, input.Password)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "LDAP authentication failed: " + err.Error()}},
		})
	}

	// Generate JWT
	token, err := generateJWT(user, h.config.JWTSecret)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "Failed to generate authentication token"}},
		})
	}

	return c.JSON(fiber.Map{
		"token": token,
		"user": fiber.Map{
			"id":    user.ID,
			"name":  user.Name,
			"email": user.Email,
		},
	})
}

// generateJWT creates a signed JWT token for the authenticated user.
func generateJWT(user *models.User, jwtSecret string) (string, error) {
	claims := jwt.MapClaims{
		"id":    user.ID,
		"email": user.Email,
		"name":  user.Name,
		"role":  user.Role,
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtSecret))
}
