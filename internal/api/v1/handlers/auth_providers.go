package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/auth"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type AuthProviderHandler struct {
	service *service.AuthProviderService
}

func NewAuthProviderHandler(service *service.AuthProviderService) *AuthProviderHandler {
	return &AuthProviderHandler{service: service}
}

// authProviderToJSON serializes an authentication provider for API responses.
func authProviderToJSON(p *models.AuthenticationProvider) fiber.Map {
	result := fiber.Map{
		"id":              p.ID,
		"account_id":     p.AccountID,
		"auth_type":      p.AuthType,
		"position":       p.Position,
		"jit_provisioning": p.JITProvisioning,
		"workflow_state":  p.WorkflowState,
		"created_at":     p.CreatedAt,
		"updated_at":     p.UpdatedAt,
	}

	switch p.AuthType {
	case "saml":
		result["idp_entity_id"] = p.IDPEntityID
		result["log_in_url"] = p.LogInURL
		result["log_out_url"] = p.LogOutURL
		result["certificate_fingerprint"] = p.CertificateFingerprint
	case "ldap":
		result["ldap_host"] = p.LDAPHost
		result["ldap_port"] = p.LDAPPort
		result["ldap_base"] = p.LDAPBase
		result["ldap_filter"] = p.LDAPFilter
		result["ldap_bind_dn"] = p.LDAPBindDN
		result["ldap_use_tls"] = p.LDAPUseTLS
		result["ldap_login_attribute"] = p.LDAPLoginAttribute
		// LDAPBindPassword is never exposed
	case "cas":
		result["cas_base_url"] = p.CASBaseURL
		result["cas_login_url"] = p.CASLoginURL
		result["cas_validate_url"] = p.CASValidateURL
		result["cas_logout_url"] = p.CASLogoutURL
	case "oidc":
		result["oidc_preset"] = p.OIDCPreset
		result["oidc_issuer_url"] = p.OIDCIssuerURL
		result["oidc_client_id"] = p.OIDCClientID
		result["oidc_scopes"] = p.OIDCScopes
		result["auto_provision"] = p.AutoProvision
		// Secret is never exposed back to the admin UI. The presence
		// flag tells the form whether a secret is configured (so it
		// shows "(currently set — type to rotate)" vs. "required").
		result["oidc_client_secret_configured"] = len(p.OIDCClientSecretEncrypted) > 0
	}

	if p.FederatedAttributes != nil {
		result["federated_attributes"] = p.FederatedAttributes
	}

	return result
}

// ListProviders returns a paginated list of authentication providers for an account.
// GET /api/v1/accounts/:account_id/authentication_providers
func (h *AuthProviderHandler) ListProviders(c *fiber.Ctx) error {
	accountID, err := strconv.Atoi(c.Params("account_id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.service.ListProviders(c.Context(), uint(accountID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch authentication providers")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	providers := make([]fiber.Map, len(result.Items))
	for i, p := range result.Items {
		providers[i] = authProviderToJSON(&p)
	}

	return c.JSON(providers)
}

// GetProvider returns a single authentication provider.
// GET /api/v1/accounts/:account_id/authentication_providers/:id
func (h *AuthProviderHandler) GetProvider(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid authentication provider ID")
	}

	provider, err := h.service.GetProvider(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "authentication provider")
	}

	return c.JSON(authProviderToJSON(provider))
}

// ListOIDCPresets returns the catalog of OIDC presets the admin UI
// uses to pre-fill issuer URLs + recommended scopes (Phase 10-A.1).
// GET /api/v1/auth/oidc/presets
//
// Public because the catalog is informational — no secrets, no
// account-specific data. The admin form fetches it before rendering.
func (h *AuthProviderHandler) ListOIDCPresets(c *fiber.Ctx) error {
	out := make([]fiber.Map, 0, len(service.OIDCPresets))
	for _, p := range service.OIDCPresets {
		out = append(out, fiber.Map{
			"code":                    p.Code,
			"label":                   p.Label,
			"issuer":                  p.Issuer,
			"scopes":                  p.Scopes,
			"description":             p.Description,
			"first_login_only_claims": p.FirstLoginOnlyClaims,
		})
	}
	return c.JSON(fiber.Map{"presets": out})
}

// CreateProvider creates a new authentication provider.
// POST /api/v1/accounts/:account_id/authentication_providers
func (h *AuthProviderHandler) CreateProvider(c *fiber.Ctx) error {
	accountID, err := strconv.Atoi(c.Params("account_id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	var input struct {
		AuthType               string            `json:"auth_type"`
		Position               int               `json:"position"`
		IDPEntityID            string            `json:"idp_entity_id"`
		LogInURL               string            `json:"log_in_url"`
		LogOutURL              string            `json:"log_out_url"`
		CertificateFingerprint string            `json:"certificate_fingerprint"`
		LDAPHost               string            `json:"ldap_host"`
		LDAPPort               int               `json:"ldap_port"`
		LDAPBase               string            `json:"ldap_base"`
		LDAPFilter             string            `json:"ldap_filter"`
		LDAPBindDN             string            `json:"ldap_bind_dn"`
		LDAPBindPassword       string            `json:"ldap_bind_password"`
		LDAPUseTLS             bool              `json:"ldap_use_tls"`
		LDAPLoginAttribute     string            `json:"ldap_login_attribute"`
		CASBaseURL             string            `json:"cas_base_url"`
		CASLoginURL            string            `json:"cas_login_url"`
		CASValidateURL         string            `json:"cas_validate_url"`
		CASLogoutURL           string            `json:"cas_logout_url"`
		JITProvisioning        bool              `json:"jit_provisioning"`
		FederatedAttributes    map[string]string `json:"federated_attributes"`
		// Phase 9-A / 10-A OIDC fields. Secret is plaintext on the wire
		// (admin typed it into the form) and gets encrypted server-side
		// via secretbox.Encrypt before persistence.
		OIDCIssuerURL    string   `json:"oidc_issuer_url"`
		OIDCClientID     string   `json:"oidc_client_id"`
		OIDCClientSecret string   `json:"oidc_client_secret"`
		OIDCScopes       []string `json:"oidc_scopes"`
		OIDCPreset       string   `json:"oidc_preset"`
		AutoProvision    bool     `json:"auto_provision"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.AuthType == "" {
		return responses.BadRequest(c, "auth_type is required")
	}

	// Phase 10-A.1: OIDC client_secret encrypts via secretbox before
	// storage. Plaintext never persists.
	var oidcSecretEncrypted []byte
	if input.AuthType == "oidc" && input.OIDCClientSecret != "" {
		ct, err := auth.Encrypt([]byte(input.OIDCClientSecret))
		if err != nil {
			return responses.InternalError(c, "failed to encrypt oidc client secret: "+err.Error())
		}
		oidcSecretEncrypted = ct
	}

	provider := &models.AuthenticationProvider{
		AccountID:                 uint(accountID),
		AuthType:                  input.AuthType,
		Position:                  input.Position,
		IDPEntityID:               input.IDPEntityID,
		LogInURL:                  input.LogInURL,
		LogOutURL:                 input.LogOutURL,
		CertificateFingerprint:    input.CertificateFingerprint,
		LDAPHost:                  input.LDAPHost,
		LDAPPort:                  input.LDAPPort,
		LDAPBase:                  input.LDAPBase,
		LDAPFilter:                input.LDAPFilter,
		LDAPBindDN:                input.LDAPBindDN,
		LDAPBindPassword:          input.LDAPBindPassword,
		LDAPUseTLS:                input.LDAPUseTLS,
		LDAPLoginAttribute:        input.LDAPLoginAttribute,
		CASBaseURL:                input.CASBaseURL,
		CASLoginURL:               input.CASLoginURL,
		CASValidateURL:            input.CASValidateURL,
		CASLogoutURL:              input.CASLogoutURL,
		JITProvisioning:           input.JITProvisioning,
		FederatedAttributes:       input.FederatedAttributes,
		OIDCIssuerURL:             input.OIDCIssuerURL,
		OIDCClientID:              input.OIDCClientID,
		OIDCClientSecretEncrypted: oidcSecretEncrypted,
		OIDCScopes:                input.OIDCScopes,
		OIDCPreset:                input.OIDCPreset,
		AutoProvision:             input.AutoProvision,
	}

	if err := h.service.CreateProvider(c.Context(), provider); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(authProviderToJSON(provider))
}

// UpdateProvider updates an existing authentication provider.
// PUT /api/v1/accounts/:account_id/authentication_providers/:id
func (h *AuthProviderHandler) UpdateProvider(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid authentication provider ID")
	}

	var input struct {
		AuthType               string            `json:"auth_type"`
		Position               int               `json:"position"`
		IDPEntityID            string            `json:"idp_entity_id"`
		LogInURL               string            `json:"log_in_url"`
		LogOutURL              string            `json:"log_out_url"`
		CertificateFingerprint string            `json:"certificate_fingerprint"`
		LDAPHost               string            `json:"ldap_host"`
		LDAPPort               int               `json:"ldap_port"`
		LDAPBase               string            `json:"ldap_base"`
		LDAPFilter             string            `json:"ldap_filter"`
		LDAPBindDN             string            `json:"ldap_bind_dn"`
		LDAPBindPassword       string            `json:"ldap_bind_password"`
		LDAPUseTLS             bool              `json:"ldap_use_tls"`
		LDAPLoginAttribute     string            `json:"ldap_login_attribute"`
		CASBaseURL             string            `json:"cas_base_url"`
		CASLoginURL            string            `json:"cas_login_url"`
		CASValidateURL         string            `json:"cas_validate_url"`
		CASLogoutURL           string            `json:"cas_logout_url"`
		JITProvisioning        bool              `json:"jit_provisioning"`
		FederatedAttributes    map[string]string `json:"federated_attributes"`
		WorkflowState          string            `json:"workflow_state"`
		// Phase 10-A.1 mirror of CreateProvider: OIDC fields + secret encryption.
		// Empty OIDCClientSecret on update means "don't change" — the
		// secretbox-encrypted column stays as-is.
		OIDCIssuerURL    string   `json:"oidc_issuer_url"`
		OIDCClientID     string   `json:"oidc_client_id"`
		OIDCClientSecret string   `json:"oidc_client_secret"`
		OIDCScopes       []string `json:"oidc_scopes"`
		OIDCPreset       string   `json:"oidc_preset"`
		AutoProvision    bool     `json:"auto_provision"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	var oidcSecretEncrypted []byte
	if input.AuthType == "oidc" && input.OIDCClientSecret != "" {
		ct, err := auth.Encrypt([]byte(input.OIDCClientSecret))
		if err != nil {
			return responses.InternalError(c, "failed to encrypt oidc client secret: "+err.Error())
		}
		oidcSecretEncrypted = ct
	}

	updates := &models.AuthenticationProvider{
		AuthType:                  input.AuthType,
		Position:                  input.Position,
		IDPEntityID:               input.IDPEntityID,
		LogInURL:                  input.LogInURL,
		LogOutURL:                 input.LogOutURL,
		CertificateFingerprint:    input.CertificateFingerprint,
		LDAPHost:                  input.LDAPHost,
		LDAPPort:                  input.LDAPPort,
		LDAPBase:                  input.LDAPBase,
		LDAPFilter:                input.LDAPFilter,
		LDAPBindDN:                input.LDAPBindDN,
		LDAPBindPassword:          input.LDAPBindPassword,
		LDAPUseTLS:                input.LDAPUseTLS,
		LDAPLoginAttribute:        input.LDAPLoginAttribute,
		CASBaseURL:                input.CASBaseURL,
		CASLoginURL:               input.CASLoginURL,
		CASValidateURL:            input.CASValidateURL,
		CASLogoutURL:              input.CASLogoutURL,
		JITProvisioning:           input.JITProvisioning,
		FederatedAttributes:       input.FederatedAttributes,
		WorkflowState:             input.WorkflowState,
		OIDCIssuerURL:             input.OIDCIssuerURL,
		OIDCClientID:              input.OIDCClientID,
		OIDCClientSecretEncrypted: oidcSecretEncrypted,
		OIDCScopes:                input.OIDCScopes,
		OIDCPreset:                input.OIDCPreset,
		AutoProvision:             input.AutoProvision,
	}

	provider, err := h.service.UpdateProvider(c.Context(), uint(id), updates)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(authProviderToJSON(provider))
}

// DeleteProvider soft-deletes an authentication provider.
// DELETE /api/v1/accounts/:account_id/authentication_providers/:id
func (h *AuthProviderHandler) DeleteProvider(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid authentication provider ID")
	}

	if err := h.service.DeleteProvider(c.Context(), uint(id)); err != nil {
		return responses.NotFound(c, "authentication provider")
	}

	return c.JSON(fiber.Map{"delete": true})
}

// TestConnection tests the LDAP connection for an authentication provider.
// POST /api/v1/accounts/:account_id/authentication_providers/:id/test
func (h *AuthProviderHandler) TestConnection(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid authentication provider ID")
	}

	result, err := h.service.TestLDAPConnection(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "authentication provider")
	}

	return c.JSON(fiber.Map{
		"success": result.Success,
		"message": result.Message,
	})
}
