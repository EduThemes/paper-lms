package handlers

// LTI 1.3 / Advantage endpoints of the LTI parity lane (G123-G134):
// tool registration (admin), dynamic registration, platform keys, tool
// placements and launches, assignment and module tool links, deep
// linking return, LTI 1.1 Basic Outcomes and the tool-service auth used
// by AGS/NRPS.

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"html/template"
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/service"
)

// --- registration JSON ---------------------------------------------------------

func rawJSONOrString(s string) interface{} {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var v interface{}
	if json.Unmarshal([]byte(s), &v) == nil {
		return v
	}
	return s
}

func registrationToJSON(reg *service.LTIRegistration) fiber.Map {
	key, cfg := reg.DeveloperKey, reg.Config
	placements, _ := service.ParseLTIPlacements(cfg.Placements)
	if placements == nil {
		placements = []service.LTIPlacementConfig{}
	}
	scopes := service.ParseScopeList(cfg.Scopes)
	if scopes == nil {
		scopes = []string{}
	}
	redirects := []string{}
	for _, r := range strings.Split(key.RedirectURIs, "\n") {
		if r = strings.TrimSpace(r); r != "" {
			redirects = append(redirects, r)
		}
	}
	out := fiber.Map{
		"id":             key.ID,
		"client_id":      key.ClientID,
		"name":           key.Name,
		"email":          key.Email,
		"notes":          key.Notes,
		"icon_url":       key.Icon,
		"workflow_state": key.WorkflowState,
		"redirect_uris":  redirects,
		"created_at":     key.CreatedAt,
		"tool_configuration": fiber.Map{
			"id":                  cfg.ID,
			"title":               cfg.Title,
			"description":         cfg.Description,
			"target_link_uri":     cfg.TargetLinkURI,
			"oidc_initiation_url": cfg.OIDCInitiationURL,
			"domain":              cfg.Domain,
			"tool_id":             cfg.ToolID,
			"privacy_level":       cfg.PrivacyLevel,
			"public_jwk_url":      cfg.PublicJWKURL,
			"public_jwk":          rawJSONOrString(cfg.PublicJWK),
			"custom_fields":       service.ParseStringMap(cfg.CustomFields),
			"placements":          placements,
			"scopes":              scopes,
		},
		"account_install": nil,
		"deployment_id":   nil,
	}
	if reg.AccountInstall != nil {
		out["account_install"] = fiber.Map{"id": reg.AccountInstall.ID, "workflow_state": reg.AccountInstall.WorkflowState}
		out["deployment_id"] = strconv.FormatUint(uint64(reg.AccountInstall.ID), 10)
	}
	return out
}

// flexRawText accepts a JSON object/array or a string holding JSON and
// returns the JSON text ("" for null/empty).
type flexRawText struct{ Value string }

func (f *flexRawText) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if bytes.Equal(b, []byte("null")) {
		f.Value = ""
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		return json.Unmarshal(b, &f.Value)
	}
	f.Value = string(b)
	return nil
}

type registrationRequest struct {
	Name              string         `json:"name"`
	Email             string         `json:"email"`
	Notes             string         `json:"notes"`
	IconURL           string         `json:"icon_url"`
	RedirectURIs      flexStringList `json:"redirect_uris"`
	WorkflowState     string         `json:"workflow_state"`
	ToolConfiguration struct {
		Title             string                       `json:"title"`
		Description       string                       `json:"description"`
		TargetLinkURI     string                       `json:"target_link_uri"`
		OIDCInitiationURL string                       `json:"oidc_initiation_url"`
		Domain            string                       `json:"domain"`
		ToolID            string                       `json:"tool_id"`
		PrivacyLevel      string                       `json:"privacy_level"`
		PublicJWKURL      string                       `json:"public_jwk_url"`
		PublicJWK         flexRawText                  `json:"public_jwk"`
		CustomFields      flexRawText                  `json:"custom_fields"`
		Placements        []service.LTIPlacementConfig `json:"placements"`
		Scopes            []string                     `json:"scopes"`
	} `json:"tool_configuration"`
}

func (r *registrationRequest) input() (service.LTIRegistrationInput, string) {
	tc := r.ToolConfiguration
	custom := map[string]string{}
	if strings.TrimSpace(tc.CustomFields.Value) != "" {
		var probe map[string]interface{}
		if json.Unmarshal([]byte(tc.CustomFields.Value), &probe) != nil {
			return service.LTIRegistrationInput{}, "custom_fields must be a JSON object"
		}
		custom = service.ParseStringMap(tc.CustomFields.Value)
	}
	return service.LTIRegistrationInput{
		Name: r.Name, Email: strings.TrimSpace(r.Email), Notes: r.Notes, IconURL: r.IconURL,
		Title: tc.Title, Description: tc.Description, TargetLinkURI: tc.TargetLinkURI,
		OIDCInitiationURL: tc.OIDCInitiationURL, Domain: tc.Domain, ToolID: tc.ToolID,
		PrivacyLevel: tc.PrivacyLevel, PublicJWKURL: tc.PublicJWKURL, PublicJWK: tc.PublicJWK.Value,
		RedirectURIs: r.RedirectURIs.values(), CustomFields: custom, Placements: tc.Placements,
		Scopes: tc.Scopes, WorkflowState: r.WorkflowState,
	}, ""
}

func writeRegistrationError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, service.ErrLTIRegistrationInvalid):
		return responses.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrLTILaunchNotFound):
		return responses.NotFound(c, "LTI registration")
	default:
		return responses.InternalError(c, "Could not save the LTI registration")
	}
}

// ListLTIRegistrations — GET /accounts/:account_id/lti_registrations (admin)
func (h *LTIHandler) ListLTIRegistrations(c *fiber.Ctx) error {
	accountID, wrote := resolveDeveloperKeyAccount(c)
	if wrote {
		return nil
	}
	regs, err := h.ltiService.ListRegistrations(c.Context(), accountID)
	if err != nil {
		return responses.InternalError(c, "Could not list LTI registrations")
	}
	out := make([]fiber.Map, 0, len(regs))
	for i := range regs {
		out = append(out, registrationToJSON(&regs[i]))
	}
	return c.JSON(out)
}

// CreateLTIRegistration — POST /accounts/:account_id/lti_registrations (admin)
func (h *LTIHandler) CreateLTIRegistration(c *fiber.Ctx) error {
	accountID, wrote := resolveDeveloperKeyAccount(c)
	if wrote {
		return nil
	}
	var req registrationRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	in, msg := req.input()
	if msg != "" {
		return responses.BadRequest(c, msg)
	}
	reg, err := h.ltiService.CreateRegistration(c.Context(), accountID, in)
	if err != nil {
		return writeRegistrationError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(registrationToJSON(reg))
}

func registrationID(c *fiber.Ctx) (uint, bool) {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		_ = responses.BadRequest(c, "Invalid registration ID")
		return 0, true
	}
	return uint(id), false
}

// GetLTIRegistration — GET /accounts/:account_id/lti_registrations/:id (admin)
func (h *LTIHandler) GetLTIRegistration(c *fiber.Ctx) error {
	accountID, wrote := resolveDeveloperKeyAccount(c)
	if wrote {
		return nil
	}
	id, wrote := registrationID(c)
	if wrote {
		return nil
	}
	reg, err := h.ltiService.GetRegistration(c.Context(), accountID, id)
	if err != nil {
		return responses.NotFound(c, "LTI registration")
	}
	return c.JSON(registrationToJSON(reg))
}

// UpdateLTIRegistration — PUT /accounts/:account_id/lti_registrations/:id (admin)
func (h *LTIHandler) UpdateLTIRegistration(c *fiber.Ctx) error {
	accountID, wrote := resolveDeveloperKeyAccount(c)
	if wrote {
		return nil
	}
	id, wrote := registrationID(c)
	if wrote {
		return nil
	}
	var req registrationRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	in, msg := req.input()
	if msg != "" {
		return responses.BadRequest(c, msg)
	}
	reg, err := h.ltiService.UpdateRegistration(c.Context(), accountID, id, in)
	if err != nil {
		return writeRegistrationError(c, err)
	}
	return c.JSON(registrationToJSON(reg))
}

// DeleteLTIRegistration — DELETE /accounts/:account_id/lti_registrations/:id (admin)
func (h *LTIHandler) DeleteLTIRegistration(c *fiber.Ctx) error {
	accountID, wrote := resolveDeveloperKeyAccount(c)
	if wrote {
		return nil
	}
	id, wrote := registrationID(c)
	if wrote {
		return nil
	}
	if err := h.ltiService.DeleteRegistration(c.Context(), accountID, id); err != nil {
		return writeRegistrationError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// SetLTIAccountInstall — PUT /accounts/:account_id/lti_registrations/:id/account_install
// Body {"enabled": bool}. Deploys the tool to every course of the tenant (G132).
func (h *LTIHandler) SetLTIAccountInstall(c *fiber.Ctx) error {
	accountID, wrote := resolveDeveloperKeyAccount(c)
	if wrote {
		return nil
	}
	id, wrote := registrationID(c)
	if wrote {
		return nil
	}
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.Unmarshal(c.Body(), &body); err != nil || body.Enabled == nil {
		return responses.BadRequest(c, "enabled is required")
	}
	if _, err := h.ltiService.SetAccountInstall(c.Context(), accountID, id, *body.Enabled); err != nil {
		return writeRegistrationError(c, err)
	}
	reg, err := h.ltiService.GetRegistration(c.Context(), accountID, id)
	if err != nil {
		return responses.NotFound(c, "LTI registration")
	}
	return c.JSON(registrationToJSON(reg))
}

// LTIPlatformInfo — GET /accounts/:account_id/lti_platform (admin)
// The values a tool vendor needs to register Paper as a platform.
func (h *LTIHandler) LTIPlatformInfo(c *fiber.Ctx) error {
	if _, wrote := resolveDeveloperKeyAccount(c); wrote {
		return nil
	}
	s := h.ltiService
	return c.JSON(fiber.Map{
		"issuer":                 s.PlatformIssuer,
		"authorization_endpoint": s.AuthorizationEndpointURL(),
		"token_endpoint":         s.TokenEndpointURL(),
		"jwks_uri":               s.JWKSURL(),
		"deep_link_return_url":   s.DeepLinkReturnURL(),
		"openid_configuration":   s.OpenIDConfigurationURL(),
		"registration_endpoint":  s.RegistrationEndpointURL(),
		"keys":                   s.GetPlatformKeyStatus(),
	})
}

// RotateLTIPlatformKey — POST /lti/platform_keys/rotate (super admin, G126)
func (h *LTIHandler) RotateLTIPlatformKey(c *fiber.Ctx) error {
	kid, err := h.ltiService.RotatePlatformKey(c.Context())
	if err != nil {
		if errors.Is(err, service.ErrLTIKeyRotationUnavailable) {
			return responses.Error(c, fiber.StatusConflict, "The platform key is operator-managed (LTI_PLATFORM_PRIVATE_KEY) and cannot be rotated here")
		}
		return responses.InternalError(c, "Could not rotate the platform key")
	}
	return c.JSON(fiber.Map{"current_kid": kid, "keys": h.ltiService.GetPlatformKeyStatus()})
}

// --- dynamic registration (G134) ---------------------------------------------------

// LTIDynamicRegistrationToken — POST /accounts/:account_id/lti_registrations/dynamic_registration_token (admin)
// Returns the registration_token and openid_configuration URL to append
// to a tool's registration initiation URL.
func (h *LTIHandler) LTIDynamicRegistrationToken(c *fiber.Ctx) error {
	accountID, wrote := resolveDeveloperKeyAccount(c)
	if wrote {
		return nil
	}
	userID, _ := c.Locals("user_id").(uint)
	tok, err := h.ltiService.IssueRegistrationToken(accountID, userID)
	if err != nil {
		return responses.InternalError(c, "Could not start the registration")
	}
	return c.JSON(fiber.Map{
		"registration_token":   tok,
		"openid_configuration": h.ltiService.OpenIDConfigurationURL(),
	})
}

// LTIOpenIDConfiguration — GET /lti/security/openid-configuration (public)
func (h *LTIHandler) LTIOpenIDConfiguration(c *fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	return c.JSON(h.ltiService.OpenIDConfiguration())
}

// LTIDynamicRegister — POST /lti/registrations (public; the Bearer
// registration_token is the credential).
func (h *LTIHandler) LTIDynamicRegister(c *fiber.Ctx) error {
	auth := c.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid_token", "error_description": "registration token required"})
	}
	resp, err := h.ltiService.RegisterDynamicTool(c.Context(), strings.TrimPrefix(auth, "Bearer "), c.Body())
	if err != nil {
		if errors.Is(err, service.ErrLTIRegistrationInvalid) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid_client_metadata", "error_description": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "server_error"})
	}
	return c.Status(fiber.StatusOK).JSON(resp)
}

// --- course tools and launches (G131, G132) ------------------------------------------

// ListCourseLTITools — GET /courses/:course_id/lti/tools?placement=… (enrolled)
func (h *LTIHandler) ListCourseLTITools(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}
	placement := c.Query("placement")
	if placement != "" && !service.KnownLTIPlacements[placement] {
		return responses.BadRequest(c, "Unknown placement")
	}
	// Selection placements are staff tools.
	switch placement {
	case service.LTIPlacementEditorButton, service.LTIPlacementAssignmentSelection, service.LTIPlacementLinkSelection, service.LTIPlacementModuleIndexMenu:
		if !callerIsCourseStaff(c) {
			return c.JSON([]fiber.Map{})
		}
	}
	tools, err := h.ltiService.ListCourseTools(c.Context(), uint(courseID), callerAccountID(c), placement)
	if err != nil {
		if errors.Is(err, service.ErrLTILaunchNotFound) {
			return responses.NotFound(c, "course")
		}
		return responses.InternalError(c, "Could not list tools")
	}
	return c.JSON(tools)
}

// ltiAutoPostTemplate renders an auto-submitting form (LTI 1.1 launch).
var ltiAutoPostTemplate = template.Must(template.New("lti_autopost").Parse(`<!DOCTYPE html>
<html>
<head><title>Launching…</title></head>
<body>
<form id="lti_autopost" action="{{.Action}}" method="POST">
{{range $k, $v := .Params}}<input type="hidden" name="{{$k}}" value="{{$v}}" />
{{end}}<noscript><button type="submit">Continue</button></noscript>
</form>
<script nonce="{{.Nonce}}">document.getElementById('lti_autopost').submit();</script>
</body>
</html>`))

func cspNonce() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

func uintQuery(c *fiber.Ctx, name string) (uint, bool) {
	raw := c.Query(name)
	if raw == "" {
		return 0, true
	}
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, false
	}
	return uint(n), true
}

// LaunchCourseTool — GET /courses/:course_id/external_tools/:id/launch (enrolled)
//
// Query: placement, resource_link_id, module_item_id, assignment_id, and
// for content selection deep_link=module|assignment|editor|link with
// module_id / assignment_id. LTI 1.3 tools get a 302 to their OIDC login
// initiation URL (G124); LTI 1.1 tools an auto-posted signed form (G133).
// Meant to be the src of an iframe (or a new window).
func (h *LTIHandler) LaunchCourseTool(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}
	toolID, err := c.ParamsInt("id")
	if err != nil || toolID <= 0 {
		return responses.BadRequest(c, "Invalid tool ID")
	}
	userID, _ := c.Locals("user_id").(uint)
	if userID == 0 {
		return responses.Unauthorized(c)
	}
	if h.gateLTILaunchForCOPPA(c, userID) {
		return nil
	}
	moduleItemID, ok1 := uintQuery(c, "module_item_id")
	assignmentID, ok2 := uintQuery(c, "assignment_id")
	moduleID, ok3 := uintQuery(c, "module_id")
	if !ok1 || !ok2 || !ok3 {
		return responses.BadRequest(c, "Invalid id parameter")
	}
	placement := c.Query("placement")
	if placement != "" && !service.KnownLTIPlacements[placement] {
		return responses.BadRequest(c, "Unknown placement")
	}
	accountID := callerAccountID(c)
	req := service.LTILaunchRequest{
		UserID: userID, AccountID: accountID, IsAdmin: h.callerIsTenantAdmin(c, userID, accountID),
		IsStaff: callerIsCourseStaff(c), CourseID: uint(courseID), ToolID: uint(toolID), Placement: placement,
		ResourceLinkUUID: c.Query("resource_link_id"), ModuleItemID: moduleItemID, AssignmentID: assignmentID,
	}
	if kind := c.Query("deep_link"); kind != "" {
		if !service.ValidDeepLinkTarget(kind) {
			return responses.BadRequest(c, "Invalid deep_link target")
		}
		req.DeepLink = &service.LTIDeepLinkTarget{Kind: kind, ModuleID: moduleID, AssignmentID: assignmentID}
		req.AssignmentID = 0
	}
	if loc, _ := c.Locals("user_locale").(string); loc != "" {
		req.Locale = loc
	}

	res, err := h.ltiService.PrepareLaunch(c.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrLTILaunchInvalid):
			return responses.BadRequest(c, err.Error())
		case errors.Is(err, service.ErrLTILaunchNotFound):
			return responses.NotFound(c, "external tool")
		default:
			return responses.InternalError(c, "Could not launch the tool")
		}
	}
	c.Set("Cache-Control", "no-store")
	if res.RedirectURL != "" {
		return c.Redirect(res.RedirectURL, fiber.StatusFound)
	}
	nonce, err := cspNonce()
	if err != nil {
		return responses.InternalError(c, "Could not build launch page")
	}
	c.Set("Content-Type", "text/html; charset=utf-8")
	c.Set("Content-Security-Policy", ltiLaunchCSP(res.FormAction, nonce))
	c.Set("X-Frame-Options", "SAMEORIGIN")
	return ltiAutoPostTemplate.Execute(c.Response().BodyWriter(), struct {
		Action string
		Params map[string]string
		Nonce  string
	}{res.FormAction, res.FormParams, nonce})
}

// --- assignment external tool (G131 assignment_selection) -------------------------

func assignmentToolIDs(c *fiber.Ctx) (uint, uint, bool) {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		_ = responses.BadRequest(c, "Invalid course ID")
		return 0, 0, true
	}
	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil || assignmentID <= 0 {
		_ = responses.BadRequest(c, "Invalid assignment ID")
		return 0, 0, true
	}
	return uint(courseID), uint(assignmentID), false
}

// GetAssignmentExternalTool — GET /courses/:course_id/assignments/:assignment_id/external_tool (enrolled)
func (h *LTIHandler) GetAssignmentExternalTool(c *fiber.Ctx) error {
	courseID, assignmentID, wrote := assignmentToolIDs(c)
	if wrote {
		return nil
	}
	v, err := h.ltiService.GetAssignmentTool(c.Context(), callerAccountID(c), courseID, assignmentID)
	if err != nil {
		return responses.NotFound(c, "assignment external tool")
	}
	if !callerIsCourseStaff(c) {
		if !v.Published {
			return responses.NotFound(c, "assignment external tool")
		}
		v.URL = ""
	}
	return c.JSON(v)
}

// PutAssignmentExternalTool — PUT /courses/:course_id/assignments/:assignment_id/external_tool (instructor)
// Body {"tool_id", "url", "title", "custom": {...}}.
func (h *LTIHandler) PutAssignmentExternalTool(c *fiber.Ctx) error {
	courseID, assignmentID, wrote := assignmentToolIDs(c)
	if wrote {
		return nil
	}
	var body struct {
		ToolID flexUint          `json:"tool_id"`
		URL    string            `json:"url"`
		Title  string            `json:"title"`
		Custom map[string]string `json:"custom"`
	}
	if err := json.Unmarshal(c.Body(), &body); err != nil || !body.ToolID.Set || body.ToolID.Value == 0 {
		return responses.BadRequest(c, "tool_id is required")
	}
	accountID := callerAccountID(c)
	if _, err := h.ltiService.AttachToolToAssignment(c.Context(), service.AssignmentToolInput{
		AccountID: accountID, CourseID: courseID, AssignmentID: assignmentID, ToolID: body.ToolID.Value,
		URL: strings.TrimSpace(body.URL), Title: service.SanitizePlainText(body.Title), Custom: body.Custom,
	}); err != nil {
		if errors.Is(err, service.ErrLTILaunchInvalid) {
			return responses.BadRequest(c, err.Error())
		}
		return responses.NotFound(c, "assignment or external tool")
	}
	v, err := h.ltiService.GetAssignmentTool(c.Context(), accountID, courseID, assignmentID)
	if err != nil {
		return responses.InternalError(c, "Could not load the assignment tool")
	}
	return c.JSON(v)
}

// DeleteAssignmentExternalTool — DELETE /courses/:course_id/assignments/:assignment_id/external_tool (instructor)
func (h *LTIHandler) DeleteAssignmentExternalTool(c *fiber.Ctx) error {
	courseID, assignmentID, wrote := assignmentToolIDs(c)
	if wrote {
		return nil
	}
	if err := h.ltiService.DetachToolFromAssignment(c.Context(), callerAccountID(c), courseID, assignmentID); err != nil {
		return responses.NotFound(c, "assignment")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// --- module external tool items (G131) ---------------------------------------------

// CreateModuleExternalToolItem — POST /courses/:course_id/modules/:module_id/external_tool_items (instructor)
// Body {"tool_id", "title", "url", "new_tab", "position"}. Creates a
// ContextExternalTool module item and its resource link.
func (h *LTIHandler) CreateModuleExternalToolItem(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}
	moduleID, err := c.ParamsInt("module_id")
	if err != nil || moduleID <= 0 {
		return responses.BadRequest(c, "Invalid module ID")
	}
	var body struct {
		ToolID   flexUint `json:"tool_id"`
		Title    string   `json:"title"`
		URL      string   `json:"url"`
		NewTab   bool     `json:"new_tab"`
		Position int      `json:"position"`
	}
	if err := json.Unmarshal(c.Body(), &body); err != nil || !body.ToolID.Set || body.ToolID.Value == 0 {
		return responses.BadRequest(c, "tool_id is required")
	}
	item, err := h.ltiService.CreateExternalToolModuleItem(c.Context(), service.ModuleToolItemInput{
		AccountID: callerAccountID(c), CourseID: uint(courseID), ModuleID: uint(moduleID), ToolID: body.ToolID.Value,
		Title: body.Title, URL: strings.TrimSpace(body.URL), NewTab: body.NewTab, Position: body.Position,
	})
	if err != nil {
		if errors.Is(err, service.ErrLTILaunchInvalid) {
			return responses.BadRequest(c, err.Error())
		}
		return responses.NotFound(c, "module or external tool")
	}
	return c.Status(fiber.StatusCreated).JSON(moduleItemToJSON(item))
}

// --- deep linking return (G130) ------------------------------------------------------

var ltiDeepLinkReturnTemplate = template.Must(template.New("lti_dl_return").Parse(`<!DOCTYPE html>
<html>
<head><title>Content selected</title></head>
<body>
<p>{{if .Error}}{{.Error}}{{else}}Content added. You can close this window.{{end}}</p>
<script nonce="{{.Nonce}}">
(function () {
  var msg = {{.Payload}};
  var target = window.parent !== window ? window.parent : window.opener;
  if (target) { target.postMessage(msg, {{.Origin}}); }
})();
</script>
</body>
</html>`))

// LTIDeepLinkingResponse — POST /lti/deep_linking_response (public; the
// tool-signed JWT is the credential). The tool's frame form-posts here;
// the page relays the result to the Paper window that opened the frame
// with postMessage (subject "LtiDeepLinkingResponse").
func (h *LTIHandler) LTIDeepLinkingResponse(c *fiber.Ctx) error {
	token := c.FormValue("JWT")
	if token == "" {
		token = c.FormValue("jwt")
	}
	payload := fiber.Map{"subject": "LtiDeepLinkingResponse"}
	status := fiber.StatusOK
	errMsg := ""
	if token == "" {
		status, errMsg = fiber.StatusBadRequest, "The tool did not return any content."
	} else if res, err := h.ltiService.HandleDeepLinkingResponse(c.Context(), token); err != nil {
		status = fiber.StatusBadRequest
		errMsg = "The tool's response could not be accepted."
		if errors.Is(err, service.ErrLTILaunchNotFound) {
			status = fiber.StatusNotFound
		}
	} else {
		payload["result"] = res
		if res.ErrorMsg != "" {
			payload["errormsg"] = res.ErrorMsg
		}
	}
	if errMsg != "" {
		payload["error"] = errMsg
	}
	nonce, err := cspNonce()
	if err != nil {
		return responses.InternalError(c, "Could not build the page")
	}
	origin := h.ltiService.PlatformIssuer
	if u, perr := url.Parse(origin); perr == nil && u.Scheme != "" && u.Host != "" {
		origin = u.Scheme + "://" + u.Host
	}
	c.Status(status)
	c.Set("Content-Type", "text/html; charset=utf-8")
	c.Set("Content-Security-Policy", "default-src 'none'; script-src 'nonce-"+nonce+"'; frame-ancestors 'self'; base-uri 'none'; form-action 'none'")
	c.Set("X-Frame-Options", "SAMEORIGIN")
	c.Set("Cache-Control", "no-store")
	return ltiDeepLinkReturnTemplate.Execute(c.Response().BodyWriter(), struct {
		Payload interface{}
		Origin  string
		Nonce   string
		Error   string
	}{payload, origin, nonce, errMsg})
}

// --- LTI 1.1 Basic Outcomes (G133) -----------------------------------------------------

// LTI11Outcomes — POST /lti/v1p1/tools/:tool_id/outcomes (public; OAuth1-signed)
func (h *LTIHandler) LTI11Outcomes(c *fiber.Ctx) error {
	toolID, err := c.ParamsInt("tool_id")
	if err != nil || toolID <= 0 {
		return responses.BadRequest(c, "Invalid tool ID")
	}
	reqURL := h.ltiService.OutcomesURL(uint(toolID))
	res := h.ltiService.HandleLTI11Outcome(c.Context(), uint(toolID), reqURL, c.Get("Authorization"), c.Body())
	c.Set("Content-Type", "application/xml; charset=utf-8")
	return c.Status(res.Status).SendString(res.XML)
}

// --- tool service auth for AGS / NRPS (G127) --------------------------------------------

const ltiToolCallerKey = "lti_tool_caller"

// ltiToolCaller returns the tool authenticated by a service token, or nil
// for a user (session/token) caller.
func ltiToolCaller(c *fiber.Ctx) *service.LTIServiceCaller {
	v, _ := c.Locals(ltiToolCallerKey).(*service.LTIServiceCaller)
	return v
}

// ToolOrSessionAuth authenticates an AGS/NRPS request either as an LTI
// tool (an RS256 platform service token from the client_credentials
// grant) or, for every other credential, through the normal session
// middleware (protected). Tool callers get Locals account_id from their
// developer key.
func (h *LTIHandler) ToolOrSessionAuth(protected fiber.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		auth := c.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") && h.ltiService != nil {
			tok := strings.TrimPrefix(auth, "Bearer ")
			if service.LooksLikeLTIServiceToken(tok) {
				caller, err := h.ltiService.VerifyServiceToken(c.Context(), tok)
				if err != nil {
					return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"errors": []fiber.Map{{"message": "Invalid access token"}}})
				}
				c.Locals(ltiToolCallerKey, caller)
				c.Locals("account_id", caller.AccountID)
				return c.Next()
			}
		}
		return protected(c)
	}
}

// UnlessTool runs mw for user callers and skips it for tool callers.
func UnlessTool(mw fiber.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if ltiToolCaller(c) != nil {
			return c.Next()
		}
		return mw(c)
	}
}

// ToolScopeOrGuard admits a tool caller that holds one of scopes and has
// the tool installed in :course_id; user callers go through guard.
func (h *LTIHandler) ToolScopeOrGuard(guard fiber.Handler, scopes ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		caller := ltiToolCaller(c)
		if caller == nil {
			return guard(c)
		}
		if !caller.HasScope(scopes...) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"errors": []fiber.Map{{"message": "Insufficient scopes on access token"}}})
		}
		courseID, err := c.ParamsInt("course_id")
		if err != nil || courseID <= 0 || !h.ltiService.ToolInstalledInCourse(c.Context(), caller, uint(courseID)) {
			return responses.NotFound(c, "course")
		}
		return c.Next()
	}
}

// decodeJSONBody parses a JSON request body whatever its (vendor) media
// type: AGS uses application/vnd.ims.lis.v2.lineitem+json and
// application/vnd.ims.lis.v1.score+json.
func decodeJSONBody(c *fiber.Ctx, v interface{}) error {
	body := bytes.TrimSpace(c.Body())
	if len(body) == 0 {
		return errors.New("empty body")
	}
	return json.Unmarshal(body, v)
}
