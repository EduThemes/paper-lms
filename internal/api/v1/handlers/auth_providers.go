package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
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
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.AuthType == "" {
		return responses.BadRequest(c, "auth_type is required")
	}

	provider := &models.AuthenticationProvider{
		AccountID:              uint(accountID),
		AuthType:               input.AuthType,
		Position:               input.Position,
		IDPEntityID:            input.IDPEntityID,
		LogInURL:               input.LogInURL,
		LogOutURL:              input.LogOutURL,
		CertificateFingerprint: input.CertificateFingerprint,
		LDAPHost:               input.LDAPHost,
		LDAPPort:               input.LDAPPort,
		LDAPBase:               input.LDAPBase,
		LDAPFilter:             input.LDAPFilter,
		LDAPBindDN:             input.LDAPBindDN,
		LDAPBindPassword:       input.LDAPBindPassword,
		LDAPUseTLS:             input.LDAPUseTLS,
		LDAPLoginAttribute:     input.LDAPLoginAttribute,
		CASBaseURL:             input.CASBaseURL,
		CASLoginURL:            input.CASLoginURL,
		CASValidateURL:         input.CASValidateURL,
		CASLogoutURL:           input.CASLogoutURL,
		JITProvisioning:        input.JITProvisioning,
		FederatedAttributes:    input.FederatedAttributes,
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
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	updates := &models.AuthenticationProvider{
		AuthType:               input.AuthType,
		Position:               input.Position,
		IDPEntityID:            input.IDPEntityID,
		LogInURL:               input.LogInURL,
		LogOutURL:              input.LogOutURL,
		CertificateFingerprint: input.CertificateFingerprint,
		LDAPHost:               input.LDAPHost,
		LDAPPort:               input.LDAPPort,
		LDAPBase:               input.LDAPBase,
		LDAPFilter:             input.LDAPFilter,
		LDAPBindDN:             input.LDAPBindDN,
		LDAPBindPassword:       input.LDAPBindPassword,
		LDAPUseTLS:             input.LDAPUseTLS,
		LDAPLoginAttribute:     input.LDAPLoginAttribute,
		CASBaseURL:             input.CASBaseURL,
		CASLoginURL:            input.CASLoginURL,
		CASValidateURL:         input.CASValidateURL,
		CASLogoutURL:           input.CASLogoutURL,
		JITProvisioning:        input.JITProvisioning,
		FederatedAttributes:    input.FederatedAttributes,
		WorkflowState:          input.WorkflowState,
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
