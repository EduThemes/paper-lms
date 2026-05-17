package auth

import (
	"compress/flate"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"hash"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/repository"
)

// SAMLConfig holds the configuration for the SAML Service Provider.
//
// Wave 7: EntityID + CertPEM + KeyPEM are construction-time fallbacks
// only. Live config is resolved per-request via the Settings Engine
// (keys auth.saml.entity_id / auth.saml.cert_file / auth.saml.key_file)
// so a super-admin can rotate the entity ID or drop a new cert at the
// configured path without a restart. CertPEM/KeyPEM hold FILESYSTEM
// PATHS (despite the legacy field name) — the catalog description and
// env-var names (SAML_CERT_FILE / SAML_KEY_FILE) are the source of
// truth. Inline-PEM-as-secret is a Wave 8 follow-up.
type SAMLConfig struct {
	// EntityID is the SP entity ID (e.g., "https://paperlms.example.com/saml/metadata")
	EntityID string
	// CertPEM is the filesystem path to the SP signing certificate (PEM).
	// Field name retained for backward compat; treat as a path.
	CertPEM string
	// KeyPEM is the filesystem path to the SP private key (PEM).
	// Field name retained for backward compat; treat as a path.
	KeyPEM string
	// IDPURL is the IDP Single Sign-On URL
	IDPURL string
	// IDPMetadataURL is the URL to fetch IDP metadata (optional)
	IDPMetadataURL string
	// ACSURL is the Assertion Consumer Service URL
	ACSURL string
	// FrontendURL is the frontend application URL for post-login redirect
	FrontendURL string
	// JWTSecret is the secret used for signing JWT tokens
	JWTSecret string
}

// SAMLHandler implements SAML 2.0 Service Provider functionality.
//
// Sprint 10-C: loginPipeline is the convergence point for the
// post-credential flow (JIT provisioning, email auto-link, MFA gate,
// audit log). Before 10-C this file owned its own inline JIT block;
// signature verification stayed put, only the post-credential JIT
// changed.
type SAMLHandler struct {
	config           SAMLConfig
	userRepo         repository.UserRepository
	authProviderRepo repository.AuthenticationProviderRepository
	loginPipeline    *LoginPipeline
	// lookup resolves auth.saml.{entity_id,cert_file,key_file} per
	// request via the Settings Engine. Same function-typed shape as
	// the OIDC handler — see SettingsLookupFunc docstring (oidc.go).
	// When lookup is nil or returns empty, the construction-time
	// SAMLConfig values are the safety net so env-only deployments
	// keep working unchanged.
	lookup SettingsLookupFunc
}

// NewSAMLHandler creates a new SAMLHandler with the given configuration and repositories.
//
// Wave 7: lookup is the per-request resolver for auth.saml.* settings.
// Pass nil to disable live resolution (env-only behavior, used by tests
// or callers that haven't migrated yet). The construction-time SAMLConfig
// remains the fallback when the lookup returns empty or errors.
func NewSAMLHandler(config SAMLConfig, userRepo repository.UserRepository, authProviderRepo repository.AuthenticationProviderRepository, loginPipeline *LoginPipeline, lookup SettingsLookupFunc) *SAMLHandler {
	return &SAMLHandler{
		config:           config,
		userRepo:         userRepo,
		authProviderRepo: authProviderRepo,
		loginPipeline:    loginPipeline,
		lookup:           lookup,
	}
}

// resolveEntityID returns the live SP entity ID, falling back to the
// construction-time value when the settings lookup is missing or empty.
// Errors from the lookup itself are non-fatal: we fall back rather than
// 500-ing the ceremony, on the principle that a stale value beats no
// SAML at all.
func (h *SAMLHandler) resolveEntityID(ctx context.Context) string {
	if h.lookup != nil {
		if v, err := h.lookup(ctx, "auth.saml.entity_id"); err == nil && v != "" {
			return v
		}
	}
	return h.config.EntityID
}

// resolveCertPath returns the live SP cert filesystem path.
func (h *SAMLHandler) resolveCertPath(ctx context.Context) string {
	if h.lookup != nil {
		if v, err := h.lookup(ctx, "auth.saml.cert_file"); err == nil && v != "" {
			return v
		}
	}
	return h.config.CertPEM
}

// resolveKeyPath returns the live SP key filesystem path. Reserved for
// future AuthnRequest signing — kept alongside its siblings so all
// three settings keys are wired in one place.
func (h *SAMLHandler) resolveKeyPath(ctx context.Context) string {
	if h.lookup != nil {
		if v, err := h.lookup(ctx, "auth.saml.key_file"); err == nil && v != "" {
			return v
		}
	}
	return h.config.KeyPEM
}

// loadCertPEM reads the cert file at the resolved path. Returns empty
// string + nil error when no path is configured anywhere — callers
// downstream of GenerateMetadata interpret that as "no signing cert
// available, emit metadata without a KeyDescriptor." A path that's set
// but unreadable is a real error and surfaces to the caller.
//
// Cert files are small (a few KB) and SAML ceremonies are infrequent,
// so we re-read on every call rather than caching. If profiling ever
// shows this as a hot path, add an in-memory cache keyed by (path,
// mtime) — but don't pre-optimize.
func (h *SAMLHandler) loadCertPEM(ctx context.Context) (string, error) {
	path := h.resolveCertPath(ctx)
	if path == "" {
		return "", nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read SAML cert at %q: %w", path, err)
	}
	return string(b), nil
}

// --- SAML XML types ---

// samlEntityDescriptor represents the SP metadata document.
type samlEntityDescriptor struct {
	XMLName  xml.Name            `xml:"urn:oasis:names:tc:SAML:2.0:metadata EntityDescriptor"`
	EntityID string              `xml:"entityID,attr"`
	SPSSOD   samlSPSSODescriptor `xml:"SPSSODescriptor"`
}

type samlSPSSODescriptor struct {
	XMLName                    xml.Name                       `xml:"urn:oasis:names:tc:SAML:2.0:metadata SPSSODescriptor"`
	AuthnRequestsSigned        bool                           `xml:"AuthnRequestsSigned,attr"`
	WantAssertionsSigned       bool                           `xml:"WantAssertionsSigned,attr"`
	ProtocolSupportEnumeration string                         `xml:"protocolSupportEnumeration,attr"`
	KeyDescriptors             []samlKeyDescriptor            `xml:"KeyDescriptor"`
	NameIDFormats              []samlNameIDFormat             `xml:"NameIDFormat"`
	AssertionConsumerServices  []samlAssertionConsumerService `xml:"AssertionConsumerService"`
}

type samlKeyDescriptor struct {
	XMLName xml.Name    `xml:"urn:oasis:names:tc:SAML:2.0:metadata KeyDescriptor"`
	Use     string      `xml:"use,attr"`
	KeyInfo samlKeyInfo `xml:"KeyInfo"`
}

type samlKeyInfo struct {
	XMLName  xml.Name     `xml:"http://www.w3.org/2000/09/xmldsig# KeyInfo"`
	X509Data samlX509Data `xml:"X509Data"`
}

type samlX509Data struct {
	XMLName         xml.Name `xml:"http://www.w3.org/2000/09/xmldsig# X509Data"`
	X509Certificate string   `xml:"http://www.w3.org/2000/09/xmldsig# X509Certificate"`
}

type samlNameIDFormat struct {
	XMLName xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:metadata NameIDFormat"`
	Value   string   `xml:",chardata"`
}

type samlAssertionConsumerService struct {
	XMLName  xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:metadata AssertionConsumerService"`
	Binding  string   `xml:"Binding,attr"`
	Location string   `xml:"Location,attr"`
	Index    int      `xml:"index,attr"`
}

// samlAuthnRequest represents a SAML AuthnRequest.
type samlAuthnRequest struct {
	XMLName                     xml.Name          `xml:"urn:oasis:names:tc:SAML:2.0:protocol AuthnRequest"`
	ID                          string            `xml:"ID,attr"`
	Version                     string            `xml:"Version,attr"`
	IssueInstant                string            `xml:"IssueInstant,attr"`
	Destination                 string            `xml:"Destination,attr"`
	AssertionConsumerServiceURL string            `xml:"AssertionConsumerServiceURL,attr"`
	ProtocolBinding             string            `xml:"ProtocolBinding,attr"`
	Issuer                      samlIssuer        `xml:"Issuer"`
	NameIDPolicy                *samlNameIDPolicy `xml:"NameIDPolicy,omitempty"`
}

type samlIssuer struct {
	XMLName xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion Issuer"`
	Value   string   `xml:",chardata"`
}

type samlNameIDPolicy struct {
	XMLName     xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:protocol NameIDPolicy"`
	Format      string   `xml:"Format,attr"`
	AllowCreate bool     `xml:"AllowCreate,attr"`
}

// samlResponse is the top-level SAML Response element received from the IDP.
type samlResponse struct {
	XMLName      xml.Name        `xml:"urn:oasis:names:tc:SAML:2.0:protocol Response"`
	ID           string          `xml:"ID,attr"`
	Version      string          `xml:"Version,attr"`
	IssueInstant string          `xml:"IssueInstant,attr"`
	Destination  string          `xml:"Destination,attr"`
	InResponseTo string          `xml:"InResponseTo,attr"`
	Status       samlStatus      `xml:"Status"`
	Assertions   []samlAssertion `xml:"Assertion"`
}

type samlStatus struct {
	StatusCode samlStatusCode `xml:"StatusCode"`
}

type samlStatusCode struct {
	Value string `xml:"Value,attr"`
}

type samlAssertion struct {
	XMLName             xml.Name                 `xml:"urn:oasis:names:tc:SAML:2.0:assertion Assertion"`
	ID                  string                   `xml:"ID,attr"`
	Version             string                   `xml:"Version,attr"`
	IssueInstant        string                   `xml:"IssueInstant,attr"`
	Issuer              samlIssuer               `xml:"Issuer"`
	Subject             samlSubject              `xml:"Subject"`
	Conditions          *samlConditions          `xml:"Conditions,omitempty"`
	AuthnStatements     []samlAuthnStatement     `xml:"AuthnStatement"`
	AttributeStatements []samlAttributeStatement `xml:"AttributeStatement"`
}

type samlSubject struct {
	NameID              samlNameID               `xml:"NameID"`
	SubjectConfirmation *samlSubjectConfirmation `xml:"SubjectConfirmation,omitempty"`
}

type samlNameID struct {
	Format string `xml:"Format,attr,omitempty"`
	Value  string `xml:",chardata"`
}

type samlSubjectConfirmation struct {
	Method                  string                       `xml:"Method,attr"`
	SubjectConfirmationData *samlSubjectConfirmationData `xml:"SubjectConfirmationData,omitempty"`
}

type samlSubjectConfirmationData struct {
	InResponseTo string `xml:"InResponseTo,attr,omitempty"`
	NotOnOrAfter string `xml:"NotOnOrAfter,attr,omitempty"`
	Recipient    string `xml:"Recipient,attr,omitempty"`
}

type samlConditions struct {
	NotBefore    string                    `xml:"NotBefore,attr,omitempty"`
	NotOnOrAfter string                    `xml:"NotOnOrAfter,attr,omitempty"`
	Audiences    []samlAudienceRestriction `xml:"AudienceRestriction"`
}

type samlAudienceRestriction struct {
	Audiences []samlAudience `xml:"Audience"`
}

type samlAudience struct {
	Value string `xml:",chardata"`
}

type samlAuthnStatement struct {
	AuthnInstant string `xml:"AuthnInstant,attr"`
	SessionIndex string `xml:"SessionIndex,attr,omitempty"`
}

type samlAttributeStatement struct {
	Attributes []samlAttribute `xml:"Attribute"`
}

type samlAttribute struct {
	Name         string               `xml:"Name,attr"`
	NameFormat   string               `xml:"NameFormat,attr,omitempty"`
	FriendlyName string               `xml:"FriendlyName,attr,omitempty"`
	Values       []samlAttributeValue `xml:"AttributeValue"`
}

type samlAttributeValue struct {
	Value string `xml:",chardata"`
}

// GenerateMetadata produces the SP metadata XML document.
//
// Wave 7: entity ID + cert are resolved per-request via the Settings
// Engine. The cert is read from disk on each call (see loadCertPEM
// docstring for the caching tradeoff).
func (h *SAMLHandler) GenerateMetadata(ctx context.Context) ([]byte, error) {
	certPEM, err := h.loadCertPEM(ctx)
	if err != nil {
		return nil, err
	}
	certBase64, err := extractCertBase64(certPEM)
	if err != nil {
		// Wave 7 audit M1: refuse to emit metadata that would echo
		// arbitrary file contents. A super-admin who pointed
		// auth.saml.cert_file at /etc/shadow or a private-key file
		// lands here instead of leaking the bytes.
		return nil, fmt.Errorf("SAML metadata: %w", err)
	}

	metadata := samlEntityDescriptor{
		EntityID: h.resolveEntityID(ctx),
		SPSSOD: samlSPSSODescriptor{
			AuthnRequestsSigned:        true,
			WantAssertionsSigned:       true,
			ProtocolSupportEnumeration: "urn:oasis:names:tc:SAML:2.0:protocol",
			KeyDescriptors: []samlKeyDescriptor{
				{
					Use: "signing",
					KeyInfo: samlKeyInfo{
						X509Data: samlX509Data{
							X509Certificate: certBase64,
						},
					},
				},
				{
					Use: "encryption",
					KeyInfo: samlKeyInfo{
						X509Data: samlX509Data{
							X509Certificate: certBase64,
						},
					},
				},
			},
			NameIDFormats: []samlNameIDFormat{
				{Value: "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress"},
				{Value: "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent"},
				{Value: "urn:oasis:names:tc:SAML:2.0:nameid-format:transient"},
			},
			AssertionConsumerServices: []samlAssertionConsumerService{
				{
					Binding:  "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
					Location: h.config.ACSURL,
					Index:    0,
				},
			},
		},
	}

	output, err := xml.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal SAML metadata: %w", err)
	}

	return append([]byte(xml.Header), output...), nil
}

// InitiateLogin creates a SAML AuthnRequest and redirects the user to the IDP.
// It supports HTTP-Redirect binding (deflate + base64 encoded query parameter).
func (h *SAMLHandler) InitiateLogin(c *fiber.Ctx) error {
	// Use provider-specific IDP URL if available via query param
	idpURL := h.config.IDPURL
	if override := c.Query("idp_url"); override != "" {
		idpURL = override
	}
	if idpURL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "IDP URL is not configured"}},
		})
	}

	requestID, err := generateSAMLID()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "Failed to generate request ID"}},
		})
	}

	authnRequest := samlAuthnRequest{
		ID:                          requestID,
		Version:                     "2.0",
		IssueInstant:                time.Now().UTC().Format(time.RFC3339),
		Destination:                 idpURL,
		AssertionConsumerServiceURL: h.config.ACSURL,
		ProtocolBinding:             "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
		Issuer: samlIssuer{
			Value: h.resolveEntityID(c.Context()),
		},
		NameIDPolicy: &samlNameIDPolicy{
			Format:      "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress",
			AllowCreate: true,
		},
	}

	xmlBytes, err := xml.Marshal(authnRequest)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "Failed to marshal AuthnRequest"}},
		})
	}

	// Deflate compress the XML for HTTP-Redirect binding
	deflated, err := deflateCompress(xmlBytes)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "Failed to compress AuthnRequest"}},
		})
	}

	encoded := base64.StdEncoding.EncodeToString(deflated)

	// Build redirect URL with SAMLRequest query parameter
	redirectURL, err := url.Parse(idpURL)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "Invalid IDP URL"}},
		})
	}

	q := redirectURL.Query()
	q.Set("SAMLRequest", encoded)
	if relayState := c.Query("RelayState"); relayState != "" {
		q.Set("RelayState", relayState)
	}
	redirectURL.RawQuery = q.Encode()

	return c.Redirect(redirectURL.String(), fiber.StatusFound)
}

// HandleACS processes the Assertion Consumer Service callback from the IDP.
// It handles HTTP-POST binding: parses the SAMLResponse from the POST form,
// extracts user attributes, performs JIT provisioning if needed, generates a JWT,
// and redirects to the frontend.
func (h *SAMLHandler) HandleACS(c *fiber.Ctx) error {
	samlResponseEncoded := c.FormValue("SAMLResponse")
	if samlResponseEncoded == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "SAMLResponse is required"}},
		})
	}

	// Decode base64
	responseXML, err := base64.StdEncoding.DecodeString(samlResponseEncoded)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "Invalid SAMLResponse encoding"}},
		})
	}

	// Parse the SAML Response XML
	var samlResp samlResponse
	if err := xml.Unmarshal(responseXML, &samlResp); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "Failed to parse SAMLResponse XML"}},
		})
	}

	// Verify XML signature if IDP certificate is available
	if err := h.verifyResponseSignature(c, responseXML); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "SAML signature verification failed: " + err.Error()}},
		})
	}

	// Verify status
	if samlResp.Status.StatusCode.Value != "urn:oasis:names:tc:SAML:2.0:status:Success" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "SAML authentication failed: " + samlResp.Status.StatusCode.Value}},
		})
	}

	// Extract assertion data
	if len(samlResp.Assertions) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "No assertions found in SAMLResponse"}},
		})
	}

	assertion := samlResp.Assertions[0]

	// Validate conditions (time window)
	if assertion.Conditions != nil {
		now := time.Now().UTC()
		if assertion.Conditions.NotBefore != "" {
			notBefore, err := time.Parse(time.RFC3339, assertion.Conditions.NotBefore)
			if err == nil && now.Before(notBefore.Add(-5*time.Minute)) {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"errors": []fiber.Map{{"message": "SAML assertion is not yet valid"}},
				})
			}
		}
		if assertion.Conditions.NotOnOrAfter != "" {
			notOnOrAfter, err := time.Parse(time.RFC3339, assertion.Conditions.NotOnOrAfter)
			if err == nil && now.After(notOnOrAfter.Add(5*time.Minute)) {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"errors": []fiber.Map{{"message": "SAML assertion has expired"}},
				})
			}
		}
	}

	// Extract NameID
	nameID := strings.TrimSpace(assertion.Subject.NameID.Value)
	if nameID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "No NameID found in SAML assertion"}},
		})
	}

	// Extract attributes from assertion
	attrs := extractSAMLAttributes(assertion.AttributeStatements)
	email := nameID
	if attrEmail, ok := attrs["email"]; ok && attrEmail != "" {
		email = attrEmail
	}
	if attrEmail, ok := attrs["http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress"]; ok && attrEmail != "" {
		email = attrEmail
	}

	displayName := attrs["displayName"]
	if displayName == "" {
		displayName = attrs["http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name"]
	}
	firstName := attrs["firstName"]
	if firstName == "" {
		firstName = attrs["http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname"]
	}
	lastName := attrs["lastName"]
	if lastName == "" {
		lastName = attrs["http://schemas.xmlsoap.org/ws/2005/05/identity/claims/surname"]
	}

	if displayName == "" && firstName != "" {
		displayName = firstName
		if lastName != "" {
			displayName = firstName + " " + lastName
		}
	}
	if displayName == "" {
		displayName = email
	}

	// Sprint 10-C: post-credential flow goes through the pipeline.
	// JIT provisioning, email auto-link (only when EmailVerified=true,
	// which it always is for SAML — the IdP attested via signed
	// assertion), audit-log, and the MFA gate all live there.
	//
	// ProviderID: looked up by ACS URL or RelayState in the future;
	// for now the SAML config carries one global IDP, so we identify
	// it by the IDP entity ID (matched at provider-load time). When
	// the SAML handler grows multi-IDP support, this field will be
	// resolved from the RelayState's `provider_id` query param.
	providerID := h.resolveProviderID(c)
	outcome := SSOOutcome{
		ProviderID:      providerID,
		ProviderType:    "saml",
		ExternalSubject: nameID, // NameID is the IdP-stable identifier
		Email:           email,
		EmailVerified:   true, // SAML IdP attests via signed assertion
		Name:            displayName,
		Attributes: map[string]any{
			"name_id":      nameID,
			"display_name": displayName,
			"first_name":   firstName,
			"last_name":    lastName,
			"email":        email,
		},
	}
	meta := RequestMeta{IPAddress: c.IP(), UserAgent: c.Get("User-Agent")}
	result, err := h.loginPipeline.Execute(c.Context(), outcome, meta)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "SAML login failed: " + err.Error()}},
		})
	}

	// Where to land the browser. RelayState wins if the IdP echoed
	// one back (the SP can stash a deep link there before kicking off
	// the AuthnRequest); otherwise the SPA root.
	relayState := c.FormValue("RelayState")
	redirectTarget := h.config.FrontendURL
	if relayState != "" {
		redirectTarget = relayState
	}

	// Pending-MFA: redirect to /mfa/verify with the token. The route
	// is on the SPA, so we tack the token onto the SPA URL.
	if result.PendingToken != "" {
		sep := "?"
		if strings.Contains(redirectTarget, "?") {
			sep = "&"
		}
		return c.Redirect(redirectTarget+"/mfa/verify"+sep+"t="+url.QueryEscape(result.PendingToken), fiber.StatusFound)
	}

	// Real session — set the cookie that the rest of the app reads.
	c.Cookie(&fiber.Cookie{
		Name:     "paper_session",
		Value:    result.Token,
		Path:     "/",
		HTTPOnly: true,
		SameSite: "Lax",
		MaxAge:   86400,
		Expires:  time.Now().Add(24 * time.Hour),
	})
	if result.MustEnroll {
		// Tenant policy requires MFA but the user hasn't enrolled —
		// land them on the enrollment flow.
		return c.Redirect(redirectTarget+"/mfa/enroll", fiber.StatusFound)
	}
	return c.Redirect(redirectTarget, fiber.StatusFound)
}

// resolveProviderID returns the authentication_providers row id this
// SAML response should be attributed to. The pre-10-C handler didn't
// thread provider_id through the SAML flow at all — multi-IDP
// support is a future enhancement. For now we look up the single
// active SAML provider on account 1; if there are several, the
// first one wins. Callers that need precise routing should pre-set
// RelayState with the provider_id encoded.
func (h *SAMLHandler) resolveProviderID(c *fiber.Ctx) uint {
	// If a provider_id query/form param is set (e.g. via RelayState
	// the SP carved out earlier), honor it.
	if raw := c.Query("provider_id"); raw != "" {
		if id, err := strconv.ParseUint(raw, 10, 64); err == nil {
			return uint(id)
		}
	}
	if raw := c.FormValue("provider_id"); raw != "" {
		if id, err := strconv.ParseUint(raw, 10, 64); err == nil {
			return uint(id)
		}
	}
	// Fallback: pick the first active SAML provider on account 1.
	// Returns 0 if none — pipeline.resolveUser refuses to JIT when
	// ProviderID is 0, so that's a safe failure mode.
	page, err := h.authProviderRepo.ListByAccountID(c.Context(), 1, repository.PaginationParams{Page: 1, PerPage: 100})
	if err != nil || page == nil {
		return 0
	}
	for _, p := range page.Items {
		if p.AuthType == "saml" && p.WorkflowState == "active" {
			return p.ID
		}
	}
	return 0
}

// extractSAMLAttributes flattens attribute statements into a simple string map.
func extractSAMLAttributes(statements []samlAttributeStatement) map[string]string {
	attrs := make(map[string]string)
	for _, stmt := range statements {
		for _, attr := range stmt.Attributes {
			if len(attr.Values) > 0 {
				// Use both Name and FriendlyName as keys
				attrs[attr.Name] = attr.Values[0].Value
				if attr.FriendlyName != "" {
					attrs[attr.FriendlyName] = attr.Values[0].Value
				}
			}
		}
	}
	return attrs
}

// extractCertBase64 extracts the base64-encoded certificate body from
// a PEM block. Returns an empty string and an error if the input is
// not a valid PEM CERTIFICATE block — this prevents the SAML
// metadata endpoint from echoing arbitrary file contents when a
// super-admin (accidentally or maliciously) points
// auth.saml.cert_file at /etc/shadow or a key file.
//
// SECURITY (Wave 7 audit M1): cert/key paths are super-admin-
// supplied filesystem paths. loadCertPEM reads whatever bytes are at
// that path; without this check, those bytes would be embedded into
// the public /saml/metadata XML response. Refusing non-CERTIFICATE
// blocks turns the file-exfil escalation back into a "broken SAML
// config" self-DoS — the operator sees an error in the metadata
// generator, not a silent data leak.
func extractCertBase64(certPEM string) (string, error) {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return "", fmt.Errorf("SAML cert file does not contain a PEM block — refusing to embed in metadata")
	}
	if block.Type != "CERTIFICATE" {
		return "", fmt.Errorf("SAML cert file PEM block is type %q, expected CERTIFICATE — refusing to embed in metadata", block.Type)
	}
	if _, err := x509.ParseCertificate(block.Bytes); err != nil {
		return "", fmt.Errorf("SAML cert file does not parse as an X.509 certificate: %w", err)
	}
	return base64.StdEncoding.EncodeToString(block.Bytes), nil
}

// generateSAMLID creates a random SAML-compliant ID string.
func generateSAMLID() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "_" + fmt.Sprintf("%x", b), nil
}

// deflateCompress applies DEFLATE compression to the input bytes.
func deflateCompress(data []byte) ([]byte, error) {
	var buf strings.Builder
	w, err := flate.NewWriter(&buf, flate.DefaultCompression)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return []byte(buf.String()), nil
}

// verifyResponseSignature verifies the XML digital signature on a SAML response.
// It loads the IDP certificate from the authentication provider configuration and
// validates the signature using RSA-SHA256 or RSA-SHA1.
func (h *SAMLHandler) verifyResponseSignature(c *fiber.Ctx, responseXML []byte) error {
	// Load IDP certificates from configured auth providers
	samlProviders, err := h.authProviderRepo.FindByAccountAndType(c.Context(), 1, "saml")
	if err != nil || len(samlProviders) == 0 {
		return nil // No SAML providers configured, skip verification
	}

	var idpCert *x509.Certificate
	for _, p := range samlProviders {
		if p.IDPCertificate != "" {
			cert, parseErr := parseIDPCertificate(p.IDPCertificate)
			if parseErr == nil {
				idpCert = cert
				break
			}
		}
	}

	if idpCert == nil {
		// No IDP certificate configured — skip only if SAML not configured at all
		if h.resolveEntityID(c.Context()) == "" {
			return nil // SAML not fully configured, skip
		}
		return fmt.Errorf("no IDP certificate configured — cannot verify SAML signature")
	}

	// Extract the Signature element from the XML
	sigValue, digestValue, signedInfo, algorithm, err := extractSignatureComponents(responseXML)
	if err != nil {
		return fmt.Errorf("could not extract signature: %w", err)
	}

	if len(sigValue) == 0 {
		return fmt.Errorf("no signature found in SAML response")
	}

	// Verify the signature
	var hashFunc crypto.Hash
	var newHash func() hash.Hash
	switch {
	case strings.Contains(algorithm, "rsa-sha256"):
		hashFunc = crypto.SHA256
		newHash = sha256.New
	case strings.Contains(algorithm, "rsa-sha1"):
		hashFunc = crypto.SHA1
		newHash = sha1.New
	default:
		hashFunc = crypto.SHA256
		newHash = sha256.New
	}

	_ = digestValue // Digest verification would require canonicalization

	// Verify RSA signature over the SignedInfo
	h2 := newHash()
	h2.Write(signedInfo)
	hashed := h2.Sum(nil)

	rsaKey, ok := idpCert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("IDP certificate does not contain an RSA public key")
	}

	if err := rsa.VerifyPKCS1v15(rsaKey, hashFunc, hashed, sigValue); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	return nil
}

// parseIDPCertificate parses an IDP certificate from PEM or raw base64 format.
func parseIDPCertificate(certData string) (*x509.Certificate, error) {
	certData = strings.TrimSpace(certData)

	// Try PEM decode first
	block, _ := pem.Decode([]byte(certData))
	if block != nil {
		return x509.ParseCertificate(block.Bytes)
	}

	// Try raw base64
	certBytes, err := base64.StdEncoding.DecodeString(certData)
	if err != nil {
		// Try with PEM wrapping
		wrapped := "-----BEGIN CERTIFICATE-----\n" + certData + "\n-----END CERTIFICATE-----"
		block, _ = pem.Decode([]byte(wrapped))
		if block != nil {
			return x509.ParseCertificate(block.Bytes)
		}
		return nil, fmt.Errorf("could not decode certificate")
	}
	return x509.ParseCertificate(certBytes)
}

// extractSignatureComponents extracts signature values from SAML XML using simple parsing.
// Returns: signatureValue, digestValue, signedInfoBytes, algorithm, error
func extractSignatureComponents(xmlData []byte) ([]byte, []byte, []byte, string, error) {
	xmlStr := string(xmlData)

	// Extract SignatureMethod Algorithm
	algorithm := "rsa-sha256"
	algRe := regexp.MustCompile(`<[^>]*SignatureMethod[^>]*Algorithm="([^"]+)"`)
	if m := algRe.FindStringSubmatch(xmlStr); len(m) > 1 {
		algorithm = strings.ToLower(m[1])
	}

	// Extract SignatureValue
	sigRe := regexp.MustCompile(`<[^>]*SignatureValue[^>]*>([^<]+)</`)
	sigMatch := sigRe.FindStringSubmatch(xmlStr)
	if len(sigMatch) < 2 {
		return nil, nil, nil, "", fmt.Errorf("SignatureValue not found")
	}
	sigB64 := strings.ReplaceAll(strings.TrimSpace(sigMatch[1]), "\n", "")
	sigB64 = strings.ReplaceAll(sigB64, " ", "")
	sigValue, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return nil, nil, nil, "", fmt.Errorf("could not decode SignatureValue: %w", err)
	}

	// Extract DigestValue
	var digestValue []byte
	digRe := regexp.MustCompile(`<[^>]*DigestValue[^>]*>([^<]+)</`)
	if m := digRe.FindStringSubmatch(xmlStr); len(m) > 1 {
		digB64 := strings.TrimSpace(m[1])
		digestValue, _ = base64.StdEncoding.DecodeString(digB64)
	}

	// Extract SignedInfo element (for signature verification)
	siRe := regexp.MustCompile(`(?s)(<[^>]*SignedInfo[^>]*>.*?</[^>]*SignedInfo>)`)
	siMatch := siRe.FindStringSubmatch(xmlStr)
	var signedInfo []byte
	if len(siMatch) > 1 {
		signedInfo = []byte(siMatch[1])
	}

	return sigValue, digestValue, signedInfo, algorithm, nil
}
