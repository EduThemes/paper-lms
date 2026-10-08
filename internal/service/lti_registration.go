package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// --- LTI 1.3 tool registration (G123) ---------------------------------------------

// LTIRegistrationInput is an admin's LTI 1.3 tool registration: the LTI
// developer key and its tool configuration, created or edited together.
type LTIRegistrationInput struct {
	Name              string
	Email             string
	Notes             string
	IconURL           string
	Title             string
	Description       string
	TargetLinkURI     string
	OIDCInitiationURL string
	Domain            string
	ToolID            string
	PrivacyLevel      string
	PublicJWKURL      string
	PublicJWK         string
	RedirectURIs      []string
	CustomFields      map[string]string
	Placements        []LTIPlacementConfig
	Scopes            []string
	// WorkflowState (update only): "active" or "inactive"; "" keeps it.
	WorkflowState string
}

// LTIRegistration is a registered LTI 1.3 tool.
type LTIRegistration struct {
	DeveloperKey   *models.DeveloperKey
	Config         *models.LTIToolConfiguration
	AccountInstall *models.ContextExternalTool
}

// ErrLTIRegistrationInvalid wraps every validation failure of a
// registration (handlers answer 400).
var ErrLTIRegistrationInvalid = errors.New("invalid LTI registration")

func regInvalid(format string, a ...interface{}) error {
	return fmt.Errorf("%w: %s", ErrLTIRegistrationInvalid, fmt.Sprintf(format, a...))
}

// DeveloperKeyCreator creates a developer key with generated client
// credentials (*DeveloperKeyService satisfies it).
type DeveloperKeyCreator interface {
	Create(ctx context.Context, key *models.DeveloperKey) (string, error)
}

// SetDeveloperKeyCreator wires the developer key creator used by tool
// registration (G123, G134).
func (s *LTIService) SetDeveloperKeyCreator(c DeveloperKeyCreator) { s.devKeyCreator = c }

func (in *LTIRegistrationInput) normalize() error {
	in.Title = SanitizePlainText(strings.TrimSpace(in.Title))
	in.Name = SanitizePlainText(strings.TrimSpace(in.Name))
	in.Description = SanitizePlainText(strings.TrimSpace(in.Description))
	in.Notes = SanitizePlainText(strings.TrimSpace(in.Notes))
	in.TargetLinkURI = strings.TrimSpace(in.TargetLinkURI)
	in.OIDCInitiationURL = strings.TrimSpace(in.OIDCInitiationURL)
	in.PublicJWKURL = strings.TrimSpace(in.PublicJWKURL)
	in.PublicJWK = strings.TrimSpace(in.PublicJWK)
	in.Domain = strings.TrimSpace(in.Domain)
	in.ToolID = strings.TrimSpace(in.ToolID)
	in.IconURL = strings.TrimSpace(in.IconURL)
	if in.Name == "" {
		in.Name = in.Title
	}
	if in.Title == "" {
		return regInvalid("title is required")
	}
	if !validToolURL(in.TargetLinkURI) {
		return regInvalid("target_link_uri must be an https URL")
	}
	if !validToolURL(in.OIDCInitiationURL) {
		return regInvalid("oidc_initiation_url must be an https URL")
	}
	if in.IconURL != "" && !validToolURL(in.IconURL) {
		return regInvalid("icon_url must be an https URL")
	}
	switch {
	case in.PublicJWK != "":
		if err := ValidateToolPublicJWK(in.PublicJWK); err != nil {
			return regInvalid("public_jwk: %s", err.Error())
		}
	case in.PublicJWKURL != "":
		if !strings.HasPrefix(in.PublicJWKURL, "https://") || !validToolURL(in.PublicJWKURL) {
			return regInvalid("public_jwk_url must be an https URL")
		}
	default:
		return regInvalid("a public_jwk or public_jwk_url is required")
	}
	if in.PrivacyLevel == "" {
		in.PrivacyLevel = LTIPrivacyAnonymous
	}
	if !ValidLTIPrivacyLevel(in.PrivacyLevel) {
		return regInvalid("privacy_level must be anonymous, name_only, email_only or public")
	}
	var redirects []string
	seen := map[string]bool{}
	for _, r := range in.RedirectURIs {
		r = strings.TrimSpace(r)
		if r == "" || seen[r] {
			continue
		}
		if !validToolURL(r) {
			return regInvalid("redirect URI %q must be an https URL", r)
		}
		seen[r] = true
		redirects = append(redirects, r)
	}
	if !seen[in.TargetLinkURI] {
		redirects = append([]string{in.TargetLinkURI}, redirects...)
	}
	in.RedirectURIs = redirects
	for i := range in.Placements {
		p := &in.Placements[i]
		if !KnownLTIPlacements[p.Placement] {
			return regInvalid("unknown placement %q", p.Placement)
		}
		if p.MessageType != "" && p.MessageType != LTIMessageResourceLink && p.MessageType != LTIMessageDeepLinking {
			return regInvalid("placement %s: message_type must be LtiResourceLinkRequest or LtiDeepLinkingRequest", p.Placement)
		}
		if p.TargetLinkURI != "" && !validToolURL(p.TargetLinkURI) {
			return regInvalid("placement %s: target_link_uri must be an https URL", p.Placement)
		}
		if p.IconURL != "" && !validToolURL(p.IconURL) {
			return regInvalid("placement %s: icon_url must be an https URL", p.Placement)
		}
		p.Text = SanitizePlainText(strings.TrimSpace(p.Text))
	}
	var scopes []string
	for _, sc := range in.Scopes {
		sc = strings.TrimSpace(sc)
		if sc == "" {
			continue
		}
		if !isLTIAdvantageScope(sc) {
			return regInvalid("unsupported scope %q", sc)
		}
		if !scopeListHas(scopes, sc) {
			scopes = append(scopes, sc)
		}
	}
	in.Scopes = scopes
	for k := range in.CustomFields {
		if strings.TrimSpace(k) == "" {
			return regInvalid("custom field names must not be empty")
		}
	}
	return nil
}

func jsonOrEmpty(v interface{}, empty bool) string {
	if empty {
		return ""
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func (in *LTIRegistrationInput) applyConfig(cfg *models.LTIToolConfiguration) {
	cfg.Title = in.Title
	cfg.Description = in.Description
	cfg.TargetLinkURI = in.TargetLinkURI
	cfg.OIDCInitiationURL = in.OIDCInitiationURL
	cfg.Domain = in.Domain
	cfg.ToolID = in.ToolID
	cfg.PrivacyLevel = in.PrivacyLevel
	cfg.PublicJWKURL = in.PublicJWKURL
	cfg.PublicJWK = in.PublicJWK
	cfg.CustomFields = jsonOrEmpty(in.CustomFields, len(in.CustomFields) == 0)
	cfg.Placements = jsonOrEmpty(in.Placements, len(in.Placements) == 0)
	cfg.Scopes = jsonOrEmpty(in.Scopes, len(in.Scopes) == 0)
}

// CreateRegistration registers an LTI 1.3 tool: an LTI developer key of
// the tenant plus its tool configuration.
func (s *LTIService) CreateRegistration(ctx context.Context, accountID uint, in LTIRegistrationInput) (*LTIRegistration, error) {
	if s.devKeyCreator == nil {
		return nil, errors.New("developer keys are not configured")
	}
	if accountID == 0 {
		return nil, regInvalid("account is required")
	}
	if err := in.normalize(); err != nil {
		return nil, err
	}
	key := &models.DeveloperKey{
		AccountID: accountID, Name: in.Name, Email: in.Email, Notes: in.Notes, Icon: in.IconURL,
		RedirectURIs: strings.Join(in.RedirectURIs, "\n"), IsLTIKey: true, Scopes: "[]",
	}
	if _, err := s.devKeyCreator.Create(ctx, key); err != nil {
		return nil, err
	}
	cfg := &models.LTIToolConfiguration{DeveloperKeyID: key.ID}
	in.applyConfig(cfg)
	if err := s.ltiConfigRepo.Create(ctx, cfg); err != nil {
		key.WorkflowState = "deleted"
		_ = s.devKeyRepo.Update(ctx, key)
		return nil, err
	}
	return &LTIRegistration{DeveloperKey: key, Config: cfg}, nil
}

// GetRegistration loads a tenant's LTI registration by developer key id.
func (s *LTIService) GetRegistration(ctx context.Context, accountID, devKeyID uint) (*LTIRegistration, error) {
	key, err := s.devKeyRepo.FindByID(ctx, devKeyID, accountID)
	if err != nil || key == nil || !key.IsLTIKey || key.WorkflowState == "deleted" || key.AccountID != accountID {
		return nil, ErrLTILaunchNotFound
	}
	cfg, err := s.ltiConfigRepo.FindByDeveloperKeyID(ctx, key.ID)
	if err != nil || cfg == nil {
		return nil, ErrLTILaunchNotFound
	}
	reg := &LTIRegistration{DeveloperKey: key, Config: cfg}
	reg.AccountInstall = s.accountInstall(ctx, accountID, key.ID)
	return reg, nil
}

// UpdateRegistration replaces a registration's configuration.
func (s *LTIService) UpdateRegistration(ctx context.Context, accountID, devKeyID uint, in LTIRegistrationInput) (*LTIRegistration, error) {
	reg, err := s.GetRegistration(ctx, accountID, devKeyID)
	if err != nil {
		return nil, err
	}
	if err := in.normalize(); err != nil {
		return nil, err
	}
	key := reg.DeveloperKey
	key.Name, key.Email, key.Notes, key.Icon = in.Name, in.Email, in.Notes, in.IconURL
	key.RedirectURIs = strings.Join(in.RedirectURIs, "\n")
	switch in.WorkflowState {
	case "":
	case "active", "inactive":
		key.WorkflowState = in.WorkflowState
	default:
		return nil, regInvalid("workflow_state must be active or inactive")
	}
	if err := s.devKeyRepo.Update(ctx, key); err != nil {
		return nil, err
	}
	in.applyConfig(reg.Config)
	if err := s.ltiConfigRepo.Update(ctx, reg.Config); err != nil {
		return nil, err
	}
	return reg, nil
}

// DeleteRegistration retires an LTI tool: the developer key is soft
// deleted (every install, launch and service token of it stops working)
// and the tool configuration removed.
func (s *LTIService) DeleteRegistration(ctx context.Context, accountID, devKeyID uint) error {
	reg, err := s.GetRegistration(ctx, accountID, devKeyID)
	if err != nil {
		return err
	}
	if err := s.devKeyRepo.Delete(ctx, reg.DeveloperKey.ID, accountID); err != nil {
		return err
	}
	return s.ltiConfigRepo.Delete(ctx, reg.Config.ID, accountID)
}

// ListRegistrations lists the tenant's LTI 1.3 registrations.
func (s *LTIService) ListRegistrations(ctx context.Context, accountID uint) ([]LTIRegistration, error) {
	if err := s.requireAdvantage(); err != nil {
		return nil, err
	}
	configs, err := s.adv.Store.ListToolConfigurationsByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	installs := map[uint]*models.ContextExternalTool{}
	if s.toolRepo != nil {
		if res, lErr := s.toolRepo.ListByContext(ctx, "Account", accountID, repository.PaginationParams{Page: 1, PerPage: 500}); lErr == nil && res != nil {
			for i := range res.Items {
				t := res.Items[i]
				if t.WorkflowState == "active" && t.DeveloperKeyID != 0 {
					installs[t.DeveloperKeyID] = &t
				}
			}
		}
	}
	out := make([]LTIRegistration, 0, len(configs))
	for i := range configs {
		cfg := configs[i]
		key, kErr := s.devKeyRepo.FindByID(ctx, cfg.DeveloperKeyID, accountID)
		if kErr != nil || key == nil {
			continue
		}
		out = append(out, LTIRegistration{DeveloperKey: key, Config: &cfg, AccountInstall: installs[key.ID]})
	}
	return out, nil
}

func (s *LTIService) accountInstall(ctx context.Context, accountID, devKeyID uint) *models.ContextExternalTool {
	if s.toolRepo == nil {
		return nil
	}
	res, err := s.toolRepo.ListByContext(ctx, "Account", accountID, repository.PaginationParams{Page: 1, PerPage: 500})
	if err != nil || res == nil {
		return nil
	}
	for i := range res.Items {
		if res.Items[i].DeveloperKeyID == devKeyID && res.Items[i].WorkflowState == "active" {
			t := res.Items[i]
			return &t
		}
	}
	return nil
}

// SetAccountInstall installs (enabled) or removes the tenant-wide
// deployment of a registered tool (G132): an Account-context install is
// available in every course of the tenant.
func (s *LTIService) SetAccountInstall(ctx context.Context, accountID, devKeyID uint, enabled bool) (*models.ContextExternalTool, error) {
	reg, err := s.GetRegistration(ctx, accountID, devKeyID)
	if err != nil {
		return nil, err
	}
	existing := reg.AccountInstall
	if !enabled {
		if existing != nil {
			existing.WorkflowState = "deleted"
			if err := s.toolRepo.Update(ctx, existing); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}
	if existing != nil {
		return existing, nil
	}
	tool := &models.ContextExternalTool{
		ContextType: "Account", ContextID: accountID, DeveloperKeyID: reg.DeveloperKey.ID,
		Name: reg.Config.Title, Description: reg.Config.Description, URL: reg.Config.TargetLinkURI,
		Domain: reg.Config.Domain, WorkflowState: "active", PrivacyLevel: reg.Config.PrivacyLevel,
	}
	if err := s.toolRepo.Create(ctx, tool); err != nil {
		return nil, err
	}
	return tool, nil
}

// --- tools for a course and placement (G131, G132) -------------------------------

// LTIPlacementTool is a tool the UI can render at a placement.
type LTIPlacementTool struct {
	ToolID          uint   `json:"id"`
	Name            string `json:"name"`
	Text            string `json:"text"`
	Description     string `json:"description,omitempty"`
	IconURL         string `json:"icon_url,omitempty"`
	Placement       string `json:"placement"`
	MessageType     string `json:"message_type"`
	ContextType     string `json:"context_type"`
	LTIVersion      string `json:"lti_version"`
	SelectionWidth  int    `json:"selection_width,omitempty"`
	SelectionHeight int    `json:"selection_height,omitempty"`
}

// ListCourseTools returns the active tools reachable from a course
// (course installs plus account installs, de-duplicated by developer key
// with the course install winning). When placement is set only tools
// with that placement enabled are returned.
func (s *LTIService) ListCourseTools(ctx context.Context, courseID, accountID uint, placement string) ([]LTIPlacementTool, error) {
	if err := s.requireAdvantage(); err != nil {
		return nil, err
	}
	if _, err := s.courseRepo.FindByID(ctx, courseID, accountID); err != nil {
		return nil, ErrLTILaunchNotFound
	}
	tools, err := s.adv.Store.ListActiveToolsForCourse(ctx, courseID, accountID)
	if err != nil {
		return nil, err
	}
	seenKey := map[uint]bool{}
	out := []LTIPlacementTool{}
	for i := range tools {
		t := tools[i]
		if t.IsLTI11() {
			if placement == "" || placement == LTIPlacementLinkSelection || placement == LTIPlacementResourceLink {
				out = append(out, LTIPlacementTool{
					ToolID: t.ID, Name: t.Name, Text: t.Name, Description: t.Description, Placement: placement,
					MessageType: "basic-lti-launch-request", ContextType: t.ContextType, LTIVersion: "1.1",
				})
			}
			continue
		}
		if t.DeveloperKeyID == 0 || seenKey[t.DeveloperKeyID] {
			continue
		}
		key, kErr := s.devKeyRepo.FindByID(ctx, t.DeveloperKeyID, accountID)
		if kErr != nil || key == nil || key.WorkflowState != "active" || !key.IsLTIKey {
			continue
		}
		cfg, cErr := s.ltiConfigRepo.FindByDeveloperKeyID(ctx, key.ID)
		if cErr != nil || cfg == nil {
			continue
		}
		seenKey[t.DeveloperKeyID] = true
		base := LTIPlacementTool{
			ToolID: t.ID, Name: t.Name, Text: t.Name, Description: cfg.Description, IconURL: key.Icon,
			ContextType: t.ContextType, LTIVersion: "1.3", MessageType: LTIMessageResourceLink,
		}
		if placement == "" {
			out = append(out, base)
			continue
		}
		pc, ok := findPlacement(cfg, placement)
		if !ok {
			continue
		}
		base.Placement = placement
		base.MessageType = pc.EffectiveMessageType()
		base.Text = firstNonEmpty(pc.Text, t.Name)
		base.IconURL = firstNonEmpty(pc.IconURL, key.Icon)
		base.SelectionWidth, base.SelectionHeight = pc.SelectionWidth, pc.SelectionHeight
		out = append(out, base)
	}
	sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i].Text) < strings.ToLower(out[j].Text) })
	return out, nil
}

// --- Dynamic Registration (G134) -----------------------------------------------------

const ltiRegistrationTokenTTL = time.Hour

// IssueRegistrationToken mints the registration_token an admin's browser
// hands to a tool's dynamic registration initiation URL.
func (s *LTIService) IssueRegistrationToken(accountID, userID uint) (string, error) {
	if accountID == 0 || userID == 0 {
		return "", errors.New("account and user are required")
	}
	now := time.Now()
	return s.signPlatformJWT(jwt.MapClaims{
		"iss": s.PlatformIssuer, "aud": ltiPurposeRegistration,
		"iat": now.Unix(), "exp": now.Add(ltiRegistrationTokenTTL).Unix(), "jti": ltiRandomHex(12),
		"acct": accountID, "uid": userID,
	})
}

// OpenIDConfigurationURL is where tools read the platform configuration.
func (s *LTIService) OpenIDConfigurationURL() string {
	return s.APIBaseURL() + "/lti/security/openid-configuration"
}

// RegistrationEndpointURL is where tools post their registration.
func (s *LTIService) RegistrationEndpointURL() string { return s.APIBaseURL() + "/lti/registrations" }

// OpenIDConfiguration is the platform's OpenID configuration with the
// LTI platform configuration extension (LTI DR §3.5).
func (s *LTIService) OpenIDConfiguration() map[string]interface{} {
	scopes := append([]string{"openid"}, LTIAdvantageScopes...)
	return map[string]interface{}{
		"issuer":                                s.PlatformIssuer,
		"authorization_endpoint":                s.AuthorizationEndpointURL(),
		"registration_endpoint":                 s.RegistrationEndpointURL(),
		"jwks_uri":                              s.JWKSURL(),
		"token_endpoint":                        s.TokenEndpointURL(),
		"token_endpoint_auth_methods_supported": []string{"private_key_jwt"},
		"token_endpoint_auth_signing_alg_values_supported": []string{"RS256"},
		"scopes_supported":                      scopes,
		"response_types_supported":              []string{"id_token"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"subject_types_supported":               []string{"public"},
		"claims_supported":                      []string{"sub", "iss", "name", "given_name", "family_name", "email", "picture", "locale"},
		"https://purl.imsglobal.org/spec/lti-platform-configuration": map[string]interface{}{
			"product_family_code": "paper-lms",
			"version":             "1.0.0",
			"messages_supported": []map[string]interface{}{
				{"type": LTIMessageResourceLink, "placements": []string{LTIPlacementCourseNavigation}},
				{"type": LTIMessageDeepLinking, "placements": []string{LTIPlacementEditorButton, LTIPlacementAssignmentSelection, LTIPlacementLinkSelection, LTIPlacementModuleIndexMenu, LTIPlacementHomeworkSubmission}},
			},
			"variables": SupportedLTIVariables(),
		},
	}
}

// dynamicRegistration is the tool's registration request (OIDC dynamic
// client registration + the LTI tool configuration extension).
type dynamicRegistration struct {
	ApplicationType         string   `json:"application_type"`
	ResponseTypes           []string `json:"response_types"`
	GrantTypes              []string `json:"grant_types"`
	InitiateLoginURI        string   `json:"initiate_login_uri"`
	RedirectURIs            []string `json:"redirect_uris"`
	ClientName              string   `json:"client_name"`
	JWKSURI                 string   `json:"jwks_uri"`
	LogoURI                 string   `json:"logo_uri"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	Contacts                []string `json:"contacts"`
	Scope                   string   `json:"scope"`
	ToolConfig              struct {
		Domain           string            `json:"domain"`
		Description      string            `json:"description"`
		TargetLinkURI    string            `json:"target_link_uri"`
		CustomParameters map[string]string `json:"custom_parameters"`
		Claims           []string          `json:"claims"`
		Messages         []struct {
			Type             string            `json:"type"`
			TargetLinkURI    string            `json:"target_link_uri"`
			Label            string            `json:"label"`
			IconURI          string            `json:"icon_uri"`
			Placements       []string          `json:"placements"`
			CustomParameters map[string]string `json:"custom_parameters"`
		} `json:"messages"`
	} `json:"https://purl.imsglobal.org/spec/lti-tool-configuration"`
}

func containsStr(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// RegisterDynamicTool completes an LTI Dynamic Registration: it verifies
// the single-use registration token, validates the tool's registration,
// creates the LTI developer key + tool configuration in the token's
// tenant, deploys the tool at the account (the deployment_id returned to
// the tool) and answers the registration response.
func (s *LTIService) RegisterDynamicTool(ctx context.Context, registrationToken string, body []byte) (map[string]interface{}, error) {
	tok, err := s.verifyPlatformJWT(registrationToken, ltiPurposeRegistration)
	if err != nil {
		return nil, regInvalid("registration token is invalid or expired")
	}
	accountID := claimUint(tok, "acct")
	if accountID == 0 {
		return nil, regInvalid("registration token is invalid")
	}
	var reg dynamicRegistration
	if err := json.Unmarshal(body, &reg); err != nil {
		return nil, regInvalid("registration body is not valid JSON")
	}
	if reg.ApplicationType != "" && reg.ApplicationType != "web" {
		return nil, regInvalid("application_type must be web")
	}
	if !containsStr(reg.ResponseTypes, "id_token") {
		return nil, regInvalid("response_types must include id_token")
	}
	if !containsStr(reg.GrantTypes, "implicit") || !containsStr(reg.GrantTypes, "client_credentials") {
		return nil, regInvalid("grant_types must include implicit and client_credentials")
	}
	if reg.TokenEndpointAuthMethod != "" && reg.TokenEndpointAuthMethod != "private_key_jwt" {
		return nil, regInvalid("token_endpoint_auth_method must be private_key_jwt")
	}
	if reg.JWKSURI == "" {
		return nil, regInvalid("jwks_uri is required")
	}

	tc := reg.ToolConfig
	privacy := LTIPrivacyAnonymous
	hasName, hasEmail := containsStr(tc.Claims, "name") || containsStr(tc.Claims, "given_name"), containsStr(tc.Claims, "email")
	switch {
	case hasName && hasEmail:
		privacy = LTIPrivacyPublic
	case hasName:
		privacy = LTIPrivacyNameOnly
	case hasEmail:
		privacy = LTIPrivacyEmailOnly
	}
	var placements []LTIPlacementConfig
	for _, m := range tc.Messages {
		if m.Type != LTIMessageResourceLink && m.Type != LTIMessageDeepLinking {
			continue
		}
		for _, p := range m.Placements {
			// Accept both bare names and Canvas-prefixed ones.
			p = strings.TrimPrefix(p, "https://canvas.instructure.com/lti/")
			if !KnownLTIPlacements[p] {
				continue
			}
			placements = append(placements, LTIPlacementConfig{
				Placement: p, MessageType: m.Type, TargetLinkURI: m.TargetLinkURI, Text: m.Label, IconURL: m.IconURI,
			})
		}
	}
	var scopes []string
	for _, sc := range strings.Fields(reg.Scope) {
		if isLTIAdvantageScope(sc) {
			scopes = append(scopes, sc)
		}
	}
	name := firstNonEmpty(reg.ClientName, tc.Domain, "LTI tool")
	contact := ""
	if len(reg.Contacts) > 0 {
		contact = reg.Contacts[0]
	}
	target := firstNonEmpty(tc.TargetLinkURI, firstOf(reg.RedirectURIs))
	in := LTIRegistrationInput{
		Name: name, Title: name, Email: contact, Description: tc.Description, TargetLinkURI: target,
		OIDCInitiationURL: reg.InitiateLoginURI, Domain: tc.Domain, PrivacyLevel: privacy,
		PublicJWKURL: reg.JWKSURI, RedirectURIs: reg.RedirectURIs, CustomFields: tc.CustomParameters,
		Placements: placements, Scopes: scopes, IconURL: reg.LogoURI, Notes: "Created by LTI Dynamic Registration",
	}
	if in.IconURL != "" && !validToolURL(in.IconURL) {
		in.IconURL = ""
	}

	// The token is single use: consume it before creating anything.
	if err := s.consumeJTI(ctx, "lti-dr", fmt.Sprint(accountID), claimString(tok, "jti"), 2*ltiRegistrationTokenTTL); err != nil {
		return nil, regInvalid("registration token has already been used")
	}
	created, err := s.CreateRegistration(ctx, accountID, in)
	if err != nil {
		return nil, err
	}
	install, err := s.SetAccountInstall(ctx, accountID, created.DeveloperKey.ID, true)
	if err != nil {
		return nil, err
	}

	resp := map[string]interface{}{}
	_ = json.Unmarshal(body, &resp)
	resp["client_id"] = created.DeveloperKey.ClientID
	resp["registration_client_uri"] = fmt.Sprintf("%s/lti/registrations/%s", s.APIBaseURL(), created.DeveloperKey.ClientID)
	toolCfg, _ := resp["https://purl.imsglobal.org/spec/lti-tool-configuration"].(map[string]interface{})
	if toolCfg == nil {
		toolCfg = map[string]interface{}{}
	}
	if install != nil {
		toolCfg["deployment_id"] = fmt.Sprint(install.ID)
	}
	resp["https://purl.imsglobal.org/spec/lti-tool-configuration"] = toolCfg
	return resp, nil
}

func firstOf(list []string) string {
	if len(list) == 0 {
		return ""
	}
	return list[0]
}
