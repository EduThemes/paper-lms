package auth

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// CASAuthenticator implements CAS 2.0 protocol authentication.
//
// Sprint 10-C: the CAS authenticator no longer carries a
// UserRepository — the post-credential JIT / auto-link flow moved
// to LoginPipeline. Callers (SSOHandler.HandleCASCallback) receive
// an SSOOutcome from ValidateTicketOutcome.
type CASAuthenticator struct {
	httpClient *http.Client
}

// NewCASAuthenticator creates a new CASAuthenticator.
func NewCASAuthenticator() *CASAuthenticator {
	return &CASAuthenticator{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// InitiateLogin redirects the user to the CAS server's login page.
func (a *CASAuthenticator) InitiateLogin(c *fiber.Ctx, provider *models.AuthenticationProvider) error {
	casBaseURL := provider.CASBaseURL
	if casBaseURL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "CAS base URL is not configured"}},
		})
	}

	// Build CAS login URL
	loginURL := provider.CASLoginURL
	if loginURL == "" {
		loginURL = strings.TrimRight(casBaseURL, "/") + "/login"
	}

	// The service URL is where CAS will redirect after authentication
	serviceURL := c.Query("service_url")
	if serviceURL == "" {
		// Default to the CAS callback endpoint on this server
		scheme := "https"
		if c.Protocol() == "http" {
			scheme = "http"
		}
		serviceURL = fmt.Sprintf("%s://%s/api/v1/auth/cas/callback?provider_id=%d", scheme, c.Hostname(), provider.ID)
	}

	parsedLoginURL, err := url.Parse(loginURL)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "Invalid CAS login URL"}},
		})
	}

	q := parsedLoginURL.Query()
	q.Set("service", serviceURL)
	parsedLoginURL.RawQuery = q.Encode()

	return c.Redirect(parsedLoginURL.String(), fiber.StatusFound)
}

// ValidateTicketOutcome validates a CAS service ticket against the
// CAS server and returns an SSOOutcome ready for LoginPipeline.Execute.
//
// Sprint 10-C: the CAS XML parsing, ticket validation HTTP call, and
// attribute extraction are byte-identical to the pre-10-C
// implementation. Only the post-credential JIT / user lookup is gone,
// replaced by the SSOOutcome return.
func (a *CASAuthenticator) ValidateTicketOutcome(ctx context.Context, provider *models.AuthenticationProvider, ticket, serviceURL string) (SSOOutcome, error) {
	if ticket == "" {
		return SSOOutcome{}, fmt.Errorf("CAS ticket is required")
	}
	if serviceURL == "" {
		return SSOOutcome{}, fmt.Errorf("service URL is required")
	}

	casBaseURL := provider.CASBaseURL
	if casBaseURL == "" {
		return SSOOutcome{}, fmt.Errorf("CAS base URL is not configured")
	}

	// Build validation URL
	validateURL := provider.CASValidateURL
	if validateURL == "" {
		validateURL = strings.TrimRight(casBaseURL, "/") + "/serviceValidate"
	}

	parsedURL, err := url.Parse(validateURL)
	if err != nil {
		return SSOOutcome{}, fmt.Errorf("invalid CAS validate URL: %w", err)
	}

	q := parsedURL.Query()
	q.Set("ticket", ticket)
	q.Set("service", serviceURL)
	parsedURL.RawQuery = q.Encode()

	// Make the validation request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return SSOOutcome{}, fmt.Errorf("failed to create validation request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return SSOOutcome{}, fmt.Errorf("CAS validation request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return SSOOutcome{}, fmt.Errorf("CAS validation returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return SSOOutcome{}, fmt.Errorf("failed to read CAS validation response: %w", err)
	}

	// Parse the CAS 2.0 XML response
	casResp, err := parseCASResponse(body)
	if err != nil {
		return SSOOutcome{}, fmt.Errorf("failed to parse CAS response: %w", err)
	}

	if casResp.Failure != nil {
		return SSOOutcome{}, fmt.Errorf("CAS authentication failed: [%s] %s",
			casResp.Failure.Code, strings.TrimSpace(casResp.Failure.Message))
	}

	if casResp.Success == nil {
		return SSOOutcome{}, fmt.Errorf("unexpected CAS response: no success or failure element")
	}

	username := strings.TrimSpace(casResp.Success.User)
	if username == "" {
		return SSOOutcome{}, fmt.Errorf("CAS response contained empty username")
	}

	// Extract attributes from CAS response
	attrs := casResp.Success.Attributes
	email := username
	displayName := ""
	firstName := ""
	lastName := ""

	if attrs != nil {
		if attrs.Email != "" {
			email = attrs.Email
		}
		if attrs.Mail != "" {
			email = attrs.Mail
		}
		if attrs.DisplayName != "" {
			displayName = attrs.DisplayName
		}
		if attrs.FirstName != "" {
			firstName = attrs.FirstName
		}
		if attrs.GivenName != "" {
			firstName = attrs.GivenName
		}
		if attrs.LastName != "" {
			lastName = attrs.LastName
		}
		if attrs.Surname != "" {
			lastName = attrs.Surname
		}
	}

	if displayName == "" && firstName != "" {
		displayName = firstName
		if lastName != "" {
			displayName = firstName + " " + lastName
		}
	}
	if displayName == "" {
		displayName = username
	}

	// Build the SSOOutcome. EmailVerified is true because the CAS
	// server already validated the ticket against the directory — the
	// principal IS authenticated by the time we get here.
	return SSOOutcome{
		ProviderID:      provider.ID,
		ProviderType:    "cas",
		ExternalSubject: username, // CAS principal name is IdP-stable
		Email:           email,
		EmailVerified:   true,
		Name:            displayName,
		Attributes: map[string]any{
			"username":     username,
			"display_name": displayName,
			"first_name":   firstName,
			"last_name":    lastName,
			"email":        email,
		},
	}, nil
}

// --- CAS 2.0 XML response types ---

// casServiceResponse is the root element of a CAS 2.0 validation response.
type casServiceResponse struct {
	XMLName xml.Name           `xml:"serviceResponse"`
	Success *casSuccess        `xml:"authenticationSuccess"`
	Failure *casFailure        `xml:"authenticationFailure"`
}

type casSuccess struct {
	User       string          `xml:"user"`
	Attributes *casAttributes  `xml:"attributes"`
}

type casAttributes struct {
	Email       string `xml:"email"`
	Mail        string `xml:"mail"`
	DisplayName string `xml:"displayName"`
	FirstName   string `xml:"firstName"`
	GivenName   string `xml:"givenName"`
	LastName    string `xml:"lastName"`
	Surname     string `xml:"sn"`
	CN          string `xml:"cn"`
	UID         string `xml:"uid"`
}

type casFailure struct {
	Code    string `xml:"code,attr"`
	Message string `xml:",chardata"`
}

// parseCASResponse parses a CAS 2.0 serviceValidate XML response.
// It handles both namespaced and non-namespaced CAS responses.
func parseCASResponse(data []byte) (*casServiceResponse, error) {
	// Try standard CAS 2.0 namespace first
	var resp casServiceResponse
	if err := xml.Unmarshal(data, &resp); err == nil {
		if resp.Success != nil || resp.Failure != nil {
			return &resp, nil
		}
	}

	// Some CAS servers use the cas: namespace prefix. Try with namespace-aware types.
	var nsResp casServiceResponseNS
	if err := xml.Unmarshal(data, &nsResp); err == nil {
		result := &casServiceResponse{}
		if nsResp.Success != nil {
			result.Success = &casSuccess{
				User: nsResp.Success.User,
			}
			if nsResp.Success.Attributes != nil {
				result.Success.Attributes = &casAttributes{
					Email:       nsResp.Success.Attributes.Email,
					Mail:        nsResp.Success.Attributes.Mail,
					DisplayName: nsResp.Success.Attributes.DisplayName,
					FirstName:   nsResp.Success.Attributes.FirstName,
					GivenName:   nsResp.Success.Attributes.GivenName,
					LastName:    nsResp.Success.Attributes.LastName,
					Surname:     nsResp.Success.Attributes.Surname,
					CN:          nsResp.Success.Attributes.CN,
					UID:         nsResp.Success.Attributes.UID,
				}
			}
		}
		if nsResp.Failure != nil {
			result.Failure = &casFailure{
				Code:    nsResp.Failure.Code,
				Message: nsResp.Failure.Message,
			}
		}
		if result.Success != nil || result.Failure != nil {
			return result, nil
		}
	}

	return nil, fmt.Errorf("unable to parse CAS response XML")
}

// Namespaced variants for CAS servers that use the standard CAS namespace.
type casServiceResponseNS struct {
	XMLName xml.Name         `xml:"http://www.yale.edu/tp/cas serviceResponse"`
	Success *casSuccessNS    `xml:"http://www.yale.edu/tp/cas authenticationSuccess"`
	Failure *casFailureNS    `xml:"http://www.yale.edu/tp/cas authenticationFailure"`
}

type casSuccessNS struct {
	User       string            `xml:"http://www.yale.edu/tp/cas user"`
	Attributes *casAttributesNS  `xml:"http://www.yale.edu/tp/cas attributes"`
}

type casAttributesNS struct {
	Email       string `xml:"http://www.yale.edu/tp/cas email"`
	Mail        string `xml:"http://www.yale.edu/tp/cas mail"`
	DisplayName string `xml:"http://www.yale.edu/tp/cas displayName"`
	FirstName   string `xml:"http://www.yale.edu/tp/cas firstName"`
	GivenName   string `xml:"http://www.yale.edu/tp/cas givenName"`
	LastName    string `xml:"http://www.yale.edu/tp/cas lastName"`
	Surname     string `xml:"http://www.yale.edu/tp/cas sn"`
	CN          string `xml:"http://www.yale.edu/tp/cas cn"`
	UID         string `xml:"http://www.yale.edu/tp/cas uid"`
}

type casFailureNS struct {
	Code    string `xml:"code,attr"`
	Message string `xml:",chardata"`
}
