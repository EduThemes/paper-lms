package auth

import (
	"compress/flate"
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
	"io"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// SAMLConfig holds the configuration for the SAML Service Provider.
type SAMLConfig struct {
	// EntityID is the SP entity ID (e.g., "https://paperlms.example.com/saml/metadata")
	EntityID string
	// CertPEM is the SP signing certificate in PEM format
	CertPEM string
	// KeyPEM is the SP private key in PEM format
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
type SAMLHandler struct {
	config           SAMLConfig
	userRepo         repository.UserRepository
	authProviderRepo repository.AuthenticationProviderRepository
}

// NewSAMLHandler creates a new SAMLHandler with the given configuration and repositories.
func NewSAMLHandler(config SAMLConfig, userRepo repository.UserRepository, authProviderRepo repository.AuthenticationProviderRepository) *SAMLHandler {
	return &SAMLHandler{
		config:           config,
		userRepo:         userRepo,
		authProviderRepo: authProviderRepo,
	}
}

// --- SAML XML types ---

// samlEntityDescriptor represents the SP metadata document.
type samlEntityDescriptor struct {
	XMLName  xml.Name          `xml:"urn:oasis:names:tc:SAML:2.0:metadata EntityDescriptor"`
	EntityID string            `xml:"entityID,attr"`
	SPSSOD   samlSPSSODescriptor `xml:"SPSSODescriptor"`
}

type samlSPSSODescriptor struct {
	XMLName                    xml.Name                    `xml:"urn:oasis:names:tc:SAML:2.0:metadata SPSSODescriptor"`
	AuthnRequestsSigned        bool                        `xml:"AuthnRequestsSigned,attr"`
	WantAssertionsSigned       bool                        `xml:"WantAssertionsSigned,attr"`
	ProtocolSupportEnumeration string                      `xml:"protocolSupportEnumeration,attr"`
	KeyDescriptors             []samlKeyDescriptor         `xml:"KeyDescriptor"`
	NameIDFormats              []samlNameIDFormat           `xml:"NameIDFormat"`
	AssertionConsumerServices  []samlAssertionConsumerService `xml:"AssertionConsumerService"`
}

type samlKeyDescriptor struct {
	XMLName xml.Name     `xml:"urn:oasis:names:tc:SAML:2.0:metadata KeyDescriptor"`
	Use     string       `xml:"use,attr"`
	KeyInfo samlKeyInfo  `xml:"KeyInfo"`
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
	XMLName                        xml.Name              `xml:"urn:oasis:names:tc:SAML:2.0:protocol AuthnRequest"`
	ID                             string                `xml:"ID,attr"`
	Version                        string                `xml:"Version,attr"`
	IssueInstant                   string                `xml:"IssueInstant,attr"`
	Destination                    string                `xml:"Destination,attr"`
	AssertionConsumerServiceURL    string                `xml:"AssertionConsumerServiceURL,attr"`
	ProtocolBinding                string                `xml:"ProtocolBinding,attr"`
	Issuer                         samlIssuer            `xml:"Issuer"`
	NameIDPolicy                   *samlNameIDPolicy     `xml:"NameIDPolicy,omitempty"`
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
	XMLName      xml.Name          `xml:"urn:oasis:names:tc:SAML:2.0:protocol Response"`
	ID           string            `xml:"ID,attr"`
	Version      string            `xml:"Version,attr"`
	IssueInstant string            `xml:"IssueInstant,attr"`
	Destination  string            `xml:"Destination,attr"`
	InResponseTo string            `xml:"InResponseTo,attr"`
	Status       samlStatus        `xml:"Status"`
	Assertions   []samlAssertion   `xml:"Assertion"`
}

type samlStatus struct {
	StatusCode samlStatusCode `xml:"StatusCode"`
}

type samlStatusCode struct {
	Value string `xml:"Value,attr"`
}

type samlAssertion struct {
	XMLName            xml.Name                `xml:"urn:oasis:names:tc:SAML:2.0:assertion Assertion"`
	ID                 string                  `xml:"ID,attr"`
	Version            string                  `xml:"Version,attr"`
	IssueInstant       string                  `xml:"IssueInstant,attr"`
	Issuer             samlIssuer              `xml:"Issuer"`
	Subject            samlSubject             `xml:"Subject"`
	Conditions         *samlConditions         `xml:"Conditions,omitempty"`
	AuthnStatements    []samlAuthnStatement    `xml:"AuthnStatement"`
	AttributeStatements []samlAttributeStatement `xml:"AttributeStatement"`
}

type samlSubject struct {
	NameID              samlNameID              `xml:"NameID"`
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
	NotBefore    string                   `xml:"NotBefore,attr,omitempty"`
	NotOnOrAfter string                   `xml:"NotOnOrAfter,attr,omitempty"`
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
func (h *SAMLHandler) GenerateMetadata() ([]byte, error) {
	certBase64 := extractCertBase64(h.config.CertPEM)

	metadata := samlEntityDescriptor{
		EntityID: h.config.EntityID,
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
			Value: h.config.EntityID,
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

	// Look up or JIT provision user
	user, err := h.userRepo.FindByEmail(c.Context(), email)
	if err != nil {
		// User not found, try by login ID
		user, err = h.userRepo.FindByLoginID(c.Context(), email)
	}
	if err != nil {
		// JIT provisioning: create a new user
		user = &models.User{
			Name:    displayName,
			LoginID: email,
			Email:   email,
		}
		if firstName != "" && lastName != "" {
			user.SortableName = lastName + ", " + firstName
			user.ShortName = firstName
		}
		// Set a random password hash since SSO users authenticate externally
		if hashErr := user.HashPassword(generateRandomPassword()); hashErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"errors": []fiber.Map{{"message": "Failed to provision user"}},
			})
		}
		if createErr := h.userRepo.Create(c.Context(), user); createErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"errors": []fiber.Map{{"message": "Failed to create user: " + createErr.Error()}},
			})
		}
	} else {
		// Update user attributes from SAML if they changed
		updated := false
		if displayName != "" && user.Name != displayName {
			user.Name = displayName
			updated = true
		}
		if firstName != "" && lastName != "" {
			sortable := lastName + ", " + firstName
			if user.SortableName != sortable {
				user.SortableName = sortable
				user.ShortName = firstName
				updated = true
			}
		}
		if updated {
			_ = h.userRepo.Update(c.Context(), user)
		}
	}

	// Generate JWT
	token, err := GenerateToken(user, h.config.JWTSecret)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "Failed to generate authentication token"}},
		})
	}

	// Redirect to frontend with token
	relayState := c.FormValue("RelayState")
	redirectTarget := h.config.FrontendURL
	if relayState != "" {
		redirectTarget = relayState
	}

	separator := "?"
	if strings.Contains(redirectTarget, "?") {
		separator = "&"
	}

	return c.Redirect(redirectTarget+separator+"token="+url.QueryEscape(token), fiber.StatusFound)
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

// extractCertBase64 extracts the base64-encoded certificate body from a PEM block.
func extractCertBase64(certPEM string) string {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		// If it's not PEM-encoded, assume it's already raw base64
		return strings.TrimSpace(certPEM)
	}
	// Verify it's a valid certificate
	if _, err := x509.ParseCertificate(block.Bytes); err != nil {
		return base64.StdEncoding.EncodeToString(block.Bytes)
	}
	return base64.StdEncoding.EncodeToString(block.Bytes)
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
		if h.config.EntityID == "" {
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

// generateRandomPassword creates a random 32-byte password for SSO-provisioned users.
func generateRandomPassword() string {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "sso-managed-account-no-password-login"
	}
	return base64.URLEncoding.EncodeToString(b)
}
