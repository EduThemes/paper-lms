package handlers

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
)

// ltiLaunchTemplate renders the form_post response. html/template's
// contextual auto-escape blocks HTML injection in the action= attribute
// and the id_token/state values. Compiled once at package init; Execute is
// goroutine-safe.
//
// B080: the auto-submit script carries a per-response CSP nonce (the
// handler sends a matching Content-Security-Policy), and the <noscript>
// fallback is a real submit button.
var ltiLaunchTemplate = template.Must(template.New("lti_launch").Parse(`<!DOCTYPE html>
<html>
<head><title>LTI Launch</title></head>
<body>
<form id="lti_launch_form" action="{{.RedirectURI}}" method="POST">
    <input type="hidden" name="id_token" value="{{.IDToken}}" />
    {{if .State}}<input type="hidden" name="state" value="{{.State}}" />{{end}}
    <noscript><button type="submit">Continue</button></noscript>
</form>
<script nonce="{{.Nonce}}">document.getElementById('lti_launch_form').submit();</script>
</body>
</html>`))

type ltiLaunchPage struct {
	RedirectURI string
	IDToken     string
	State       string
	Nonce       string
}

// RenderLTILaunchForTest exposes the launch-HTML template render so
// tests in this package's external _test package can assert on the
// escape contract (F-056). NOT for production callers.
func RenderLTILaunchForTest(w interface{ Write([]byte) (int, error) }, redirectURI, idToken string) error {
	return ltiLaunchTemplate.Execute(w, ltiLaunchPage{RedirectURI: redirectURI, IDToken: idToken, Nonce: "test-nonce"})
}

// ltiLaunchCSP builds the Content-Security-Policy for the auto-post page
// (B080). The app-wide policy (script-src 'self'; form-action 'self';
// frame-ancestors 'none') blocks exactly what this page must do: run its
// inline submit script and post cross-origin to the tool. This policy
// allows only the nonce'd script, only form posts to the tool's origin
// (plus https: for tools that redirect after the launch POST), and
// framing by Paper itself so a launch can render in a course iframe.
func ltiLaunchCSP(redirectURI, nonce string) string {
	formAction := "https:"
	if u, err := url.Parse(redirectURI); err == nil && u.Scheme != "" && u.Host != "" {
		formAction = u.Scheme + "://" + u.Host + " https:"
	}
	return "default-src 'none'; " +
		"script-src 'nonce-" + nonce + "'; " +
		"form-action " + formAction + "; " +
		"frame-ancestors 'self'; " +
		"base-uri 'none'"
}

// requireSessionUserMatches verifies the authenticated caller IS the user
// the LTI launch is being minted for. login_hint is attacker-controlled
// in a pre-authentication context; without this check, anyone hitting
// the launch endpoint can mint a signed id_token impersonating any user.
//
// Returns the resolved user ID (always equal to the session user) and
// true on success. On failure, writes the appropriate 401/403 response
// and returns false — caller short-circuits.
//
// loginHint may be empty (use session) or a decimal user ID (must match
// session). Anything else 400s.
func (h *LTIHandler) requireSessionUserMatches(c *fiber.Ctx, loginHint string) (uint, bool) {
	sessionUserID, _ := c.Locals("user_id").(uint)
	if sessionUserID == 0 {
		_ = responses.Unauthorized(c)
		return 0, false
	}
	if loginHint == "" {
		return sessionUserID, true
	}
	hinted, err := strconv.ParseUint(loginHint, 10, 64)
	if err != nil {
		_ = responses.BadRequest(c, "Invalid login_hint")
		return 0, false
	}
	if uint(hinted) != sessionUserID {
		// 401, not 403 — declaring the mismatch would leak that other
		// user IDs exist. The browser cookie session establishes who
		// the caller IS; login_hint is informational.
		_ = responses.Unauthorized(c)
		return 0, false
	}
	return sessionUserID, true
}

type LTIHandler struct {
	ltiService  *service.LTIService
	agsService  *service.LTIAGSService
	nrpsService *service.LTINRPSService
	toolRepo    repository.ContextExternalToolRepository
	configRepo  repository.LTIToolConfigurationRepository
	// 13.4 (Wave C.2) — COPPA gate dependencies. All nil-safe; nil
	// repos skip the gate (development fallback). Production wires all.
	userRepo            repository.UserRepository
	accountRepo         repository.AccountRepository
	parentalConsentRepo postgres.ParentalConsentRepository
}

func NewLTIHandler(
	ltiService *service.LTIService,
	agsService *service.LTIAGSService,
	nrpsService *service.LTINRPSService,
	toolRepo repository.ContextExternalToolRepository,
	configRepo repository.LTIToolConfigurationRepository,
	userRepo repository.UserRepository,
	accountRepo repository.AccountRepository,
	parentalConsentRepo postgres.ParentalConsentRepository,
) *LTIHandler {
	return &LTIHandler{
		ltiService:          ltiService,
		agsService:          agsService,
		nrpsService:         nrpsService,
		toolRepo:            toolRepo,
		configRepo:          configRepo,
		userRepo:            userRepo,
		accountRepo:         accountRepo,
		parentalConsentRepo: parentalConsentRepo,
	}
}

// gateLTILaunchForCOPPA enforces the 13.4 LTI parental-consent gate.
// Returns true (and writes a 403) when the launch is denied; false when
// the launch may proceed. Nil-safe: if the COPPA dependencies aren't
// wired, the gate is bypassed (development / older test paths).
//
// Rule (locked 2026-05-15): in tenants with CoppaStrict=true OR
// tenant_mode in {k5,m68}, the calling user MUST have a granted
// ParentalConsent row with consent_type = "third_party_sharing". Without
// it, LTI tool launches (which dispatch student data to third-party
// vendors) are refused.
func (h *LTIHandler) gateLTILaunchForCOPPA(c *fiber.Ctx, userID uint) bool {
	if h.accountRepo == nil || h.userRepo == nil {
		return false
	}
	// AUTH-INTERNAL: COPPA gate looks up the launching user's home
	// account_id. userID is the JWT subject; accountID=0 is correct.
	user, err := h.userRepo.FindByID(c.Context(), userID, 0)
	if err != nil || user == nil {
		// Unknown user — let the downstream launch fail naturally.
		return false
	}
	account, err := h.accountRepo.FindByID(c.Context(), user.AccountID)
	if err != nil || account == nil {
		return false
	}
	if !isCOPPATenant(account) {
		return false
	}
	// COPPA tenant: require granted third_party_sharing consent.
	if h.parentalConsentRepo == nil {
		_ = responses.Error(c, fiber.StatusForbidden, "LTI tool launch requires parental consent for third-party data sharing.")
		return true
	}
	consents, err := h.parentalConsentRepo.FindByStudentID(c.Context(), userID)
	if err != nil {
		_ = responses.Error(c, fiber.StatusForbidden, "LTI tool launch requires parental consent for third-party data sharing.")
		return true
	}
	for _, cn := range consents {
		if cn.ConsentType == "third_party_sharing" && cn.Status == "granted" {
			return false
		}
	}
	_ = responses.Error(c, fiber.StatusForbidden, "LTI tool launch requires parental consent for third-party data sharing.")
	return true
}

// callerIsTenantAdmin re-checks the admin role against the user row (the
// JWT role claim is only a soft hint) and requires the admin to belong to
// the caller's tenant.
func (h *LTIHandler) callerIsTenantAdmin(c *fiber.Ctx, userID, accountID uint) bool {
	if h.userRepo == nil {
		return false
	}
	// AUTH-INTERNAL: userID is the session subject; the tenant match is
	// asserted below.
	user, err := h.userRepo.FindByID(c.Context(), userID, 0)
	if err != nil || user == nil || user.AccountID != accountID {
		return false
	}
	return user.Role == models.RoleAdmin || user.Role == models.RoleSuperUser
}

// ltiParams reads LTI/OIDC parameters from the query string, a form body
// or a JSON body, in that order (the OIDC flows use GET and form_post;
// Paper's own UI posts JSON).
type ltiParams struct {
	c    *fiber.Ctx
	json map[string]string
}

func newLTIParams(c *fiber.Ctx) *ltiParams {
	p := &ltiParams{c: c, json: map[string]string{}}
	body := bytes.TrimSpace(c.Body())
	if len(body) > 0 && body[0] == '{' {
		var raw map[string]interface{}
		if json.Unmarshal(body, &raw) == nil {
			for k, v := range raw {
				switch tv := v.(type) {
				case string:
					p.json[k] = tv
				case float64:
					p.json[k] = strconv.FormatFloat(tv, 'f', -1, 64)
				}
			}
		}
	}
	return p
}

func (p *ltiParams) get(name string) string {
	if v := p.c.Query(name); v != "" {
		return v
	}
	if v := p.c.FormValue(name); v != "" {
		return v
	}
	return p.json[name]
}

// --------------------------------------------------------------------------
// JWKS Endpoint
// --------------------------------------------------------------------------

// JWKS returns the platform's public keys in JSON Web Key Set format.
// GET /api/v1/lti/jwks (PUBLIC - no auth)
func (h *LTIHandler) JWKS(c *fiber.Ctx) error {
	// G126: a rotation on another replica must reach this replica's key
	// set too, or tools keep fetching a JWKS without the signing key.
	h.ltiService.RefreshPlatformKeys(c.Context())
	jwks, err := h.ltiService.GetJWKS()
	if err != nil {
		return responses.InternalError(c, "Could not retrieve platform keys")
	}

	c.Set("Content-Type", "application/json")
	c.Set("Cache-Control", "public, max-age=3600")
	return c.JSON(jwks)
}

// --------------------------------------------------------------------------
// OIDC Login Initiation
// --------------------------------------------------------------------------

// OIDCLogin starts an LTI 1.3 launch: the platform's third-party-initiated
// login. POST /api/v1/lti/oidc/login (session required)
//
// The browser is redirected to the tool's OIDC initiation URL with iss,
// login_hint, target_link_uri, lti_message_hint, client_id and
// lti_deployment_id (B083). The tool answers by sending the browser to
// the platform authorization endpoint (LaunchDirect).
func (h *LTIHandler) OIDCLogin(c *fiber.Ctx) error {
	p := newLTIParams(c)
	clientID := p.get("client_id")
	loginHint := p.get("login_hint")
	targetLinkURI := p.get("target_link_uri")
	ltiMessageHint := p.get("lti_message_hint")

	if clientID == "" {
		return responses.BadRequest(c, "client_id is required")
	}

	// SECURITY (F-055): the caller MUST be the user being launched.
	// login_hint is otherwise attacker-controlled in this anonymous-
	// reachable endpoint and would let any actor mint a signed id_token
	// impersonating any user. Verify the session JWT matches the hint
	// (or fill the hint from the session when omitted).
	userID, ok := h.requireSessionUserMatches(c, loginHint)
	if !ok {
		return nil
	}
	loginHint = strconv.FormatUint(uint64(userID), 10)

	// 13.4 (Wave C.2) — COPPA gate on the initiation step too. Refusing
	// at /oidc/login avoids round-tripping the user's identity to the
	// tool's OIDC endpoint before we decide to refuse.
	if h.gateLTILaunchForCOPPA(c, userID) {
		return nil
	}

	redirectURL, err := h.ltiService.InitiateLogin(
		c.Context(),
		callerAccountID(c),
		clientID,
		loginHint,
		targetLinkURI,
		ltiMessageHint,
	)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Redirect(redirectURL, fiber.StatusFound)
}

// --------------------------------------------------------------------------
// LTI Launch (OIDC authorization endpoint)
// --------------------------------------------------------------------------

// LaunchDirect is the platform's OIDC authorization endpoint for LTI 1.3
// launches. GET|POST /api/v1/lti/authorize (and POST /api/v1/lti/launch).
//
// The tool sends the browser here (after OIDCLogin) with scope=openid,
// response_type=id_token, response_mode=form_post, prompt=none,
// client_id, redirect_uri, login_hint, lti_message_hint, and its own
// nonce and state. The handler checks the session user is login_hint,
// the COPPA gate, and (in the service) the client/redirect/course/
// enrollment/installation rules (B186), then auto-posts id_token + state
// to the tool's redirect_uri.
//
// Message hint format: "courseID:resourceLinkID:developerKeyID"
func (h *LTIHandler) LaunchDirect(c *fiber.Ctx) error {
	p := newLTIParams(c)
	loginHint := p.get("login_hint")

	// SECURITY (F-055): the session JWT is the source of identity, not
	// the login_hint form value.
	userID, ok := h.requireSessionUserMatches(c, loginHint)
	if !ok {
		return nil
	}

	// 13.4 (Wave C.2) — COPPA gate.
	if h.gateLTILaunchForCOPPA(c, userID) {
		return nil
	}

	req := service.LTIAuthRequest{
		UserID:         userID,
		Scope:          p.get("scope"),
		ResponseType:   p.get("response_type"),
		ResponseMode:   p.get("response_mode"),
		ClientID:       p.get("client_id"),
		RedirectURI:    p.get("redirect_uri"),
		Nonce:          p.get("nonce"),
		State:          p.get("state"),
		Prompt:         p.get("prompt"),
		LTIMessageHint: p.get("lti_message_hint"),
	}
	if !service.IsSignedLTIMessageHint(req.LTIMessageHint) {
		if _, _, _, err := service.ParseLTIMessageHint(req.LTIMessageHint); err != nil {
			return responses.BadRequest(c, "Invalid lti_message_hint")
		}
	}
	for _, f := range []struct{ name, val string }{
		{"client_id", req.ClientID},
		{"redirect_uri", req.RedirectURI},
		{"nonce", req.Nonce},
		{"scope", req.Scope},
		{"response_type", req.ResponseType},
	} {
		if f.val == "" {
			return responses.BadRequest(c, f.name+" is required")
		}
	}

	req.AccountID = callerAccountID(c)
	req.IsAdmin = h.callerIsTenantAdmin(c, userID, req.AccountID)

	authResp, err := h.ltiService.AuthorizeLaunch(c.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrLTILaunchInvalid):
			return responses.BadRequest(c, err.Error())
		case errors.Is(err, service.ErrLTILaunchNotFound):
			return responses.NotFound(c, "LTI launch")
		default:
			return responses.InternalError(c, "Could not build launch token")
		}
	}

	var nonceBytes [16]byte
	if _, err := rand.Read(nonceBytes[:]); err != nil {
		return responses.InternalError(c, "Could not build launch page")
	}
	// URL-safe alphabet (valid CSP base64-value): html/template would
	// entity-escape a '+' in the nonce attribute, leaving the page's
	// literal nonce different from the header's.
	cspNonce := base64.RawURLEncoding.EncodeToString(nonceBytes[:])

	// SECURITY (F-056): redirect_uri was validated against the tool's
	// registered URIs in the service; html/template escapes it anyway.
	c.Set("Content-Type", "text/html; charset=utf-8")
	c.Set("Content-Security-Policy", ltiLaunchCSP(authResp.RedirectURI, cspNonce))
	c.Set("X-Frame-Options", "SAMEORIGIN")
	c.Set("Cache-Control", "no-store")
	return ltiLaunchTemplate.Execute(c.Response().BodyWriter(), ltiLaunchPage{
		RedirectURI: authResp.RedirectURI,
		IDToken:     authResp.IDToken,
		State:       authResp.State,
		Nonce:       cspNonce,
	})
}

// --------------------------------------------------------------------------
// AGS (Assignment and Grade Services) Endpoints
// --------------------------------------------------------------------------
//
// Every AGS/NRPS endpoint is reachable by course staff (session or user
// token) and, through ToolOrSessionAuth, by an LTI tool holding a
// client_credentials service token with the right scope (G127). A tool
// caller sees and edits only the line items its developer key owns.

// lineItemsBaseURL returns the absolute URL of the course's line item
// container, whatever endpoint (list, item, item/scores, ...) the request
// hit. B187: the item `id` used to be built from c.Path() of an item
// request, which already ends in /line_items/:id, so it came out
// ".../line_items/7/7".
func lineItemsBaseURL(c *fiber.Ctx) string {
	path := c.Path()
	const marker = "/line_items"
	if i := strings.Index(path, marker); i >= 0 {
		path = path[:i+len(marker)]
	}
	return c.BaseURL() + path
}

// lineItemToJSON serializes an LTI line item for API responses. baseURL is
// the line item container URL (see lineItemsBaseURL).
func lineItemToJSON(item *models.LTILineItem, baseURL string) fiber.Map {
	result := fiber.Map{
		"id":           fmt.Sprintf("%s/%d", baseURL, item.ID),
		"label":        item.Label,
		"scoreMaximum": item.ScoreMaximum,
		"tag":          item.Tag,
		"resourceId":   item.ResourceID,
		"created_at":   item.CreatedAt,
		"updated_at":   item.UpdatedAt,
	}

	if item.ResourceLinkIDStr != "" {
		result["resourceLinkId"] = item.ResourceLinkIDStr
	}
	if item.AssignmentID != nil {
		result["assignmentId"] = *item.AssignmentID
	}
	if item.StartDateTime != nil {
		result["startDateTime"] = item.StartDateTime.UTC().Format(time.RFC3339)
	}
	if item.EndDateTime != nil {
		result["endDateTime"] = item.EndDateTime.UTC().Format(time.RFC3339)
	}

	return result
}

// agsLimit applies the AGS/NRPS `limit` query parameter to the page size.
// Returns wrote=true after a 400.
func agsLimit(c *fiber.Ctx, params *repository.PaginationParams) bool {
	if raw := c.Query("limit"); raw != "" {
		limit, convErr := strconv.Atoi(raw)
		if convErr != nil || limit < 1 {
			_ = responses.BadRequest(c, "Invalid limit")
			return true
		}
		if limit > nrpsMaxLimit {
			limit = nrpsMaxLimit
		}
		params.PerPage = limit
	}
	return false
}

// ListLineItems returns a paginated list of LTI line items for a course.
// GET /api/v1/lti/courses/:course_id/line_items
// Filters (G128): resource_link_id, resource_id, tag, limit.
func (h *LTIHandler) ListLineItems(c *fiber.Ctx) error {
	courseID, err := strconv.Atoi(c.Params("course_id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)
	if agsLimit(c, &params) {
		return nil
	}
	filter := repository.LTILineItemFilter{
		ResourceLinkID: c.Query("resource_link_id"),
		ResourceID:     c.Query("resource_id"),
		Tag:            c.Query("tag"),
	}
	if caller := ltiToolCaller(c); caller != nil {
		dk := caller.DevKeyID
		filter.DeveloperKeyID = &dk
	}

	result, err := h.agsService.ListLineItemsFiltered(c.Context(), uint(courseID), filter, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch line items")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	baseURL := lineItemsBaseURL(c)
	items := make([]fiber.Map, len(result.Items))
	for i, item := range result.Items {
		items[i] = lineItemToJSON(&item, baseURL)
	}

	c.Set("Content-Type", "application/vnd.ims.lis.v2.lineitemcontainer+json")
	return c.JSON(items)
}

type createLineItemRequest struct {
	Label             string  `json:"label"`
	ScoreMaximum      float64 `json:"scoreMaximum"`
	Tag               string  `json:"tag"`
	ResourceID        string  `json:"resourceId"`
	ResourceLinkIDStr string  `json:"resourceLinkId"`
	AssignmentID      *uint   `json:"assignmentId"`
	StartDateTime     string  `json:"startDateTime"`
	EndDateTime       string  `json:"endDateTime"`
}

// parseAGSTime parses an optional ISO 8601 AGS date.
func parseAGSTime(raw string) (*time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// CreateLineItem creates a new LTI line item for a course.
// POST /api/v1/lti/courses/:course_id/line_items
func (h *LTIHandler) CreateLineItem(c *fiber.Ctx) error {
	courseID, err := strconv.Atoi(c.Params("course_id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input createLineItemRequest
	if err := decodeJSONBody(c, &input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Label == "" {
		return responses.BadRequest(c, "Line item label is required")
	}
	if input.ScoreMaximum <= 0 {
		return responses.BadRequest(c, "scoreMaximum must be greater than 0")
	}
	start, err := parseAGSTime(input.StartDateTime)
	if err != nil {
		return responses.BadRequest(c, "startDateTime must be ISO 8601")
	}
	end, err := parseAGSTime(input.EndDateTime)
	if err != nil {
		return responses.BadRequest(c, "endDateTime must be ISO 8601")
	}

	item := &models.LTILineItem{
		Label:             service.SanitizePlainText(input.Label),
		ScoreMaximum:      input.ScoreMaximum,
		Tag:               input.Tag,
		ResourceID:        input.ResourceID,
		ResourceLinkIDStr: input.ResourceLinkIDStr,
		AssignmentID:      input.AssignmentID,
		StartDateTime:     start,
		EndDateTime:       end,
	}

	// B077: the service ties assignmentId to this course + tenant.
	if caller := ltiToolCaller(c); caller != nil {
		err = h.agsService.CreateToolLineItem(c.Context(), uint(courseID), callerAccountID(c), caller.DevKeyID, item)
	} else {
		err = h.agsService.CreateLineItem(c.Context(), uint(courseID), callerAccountID(c), item)
	}
	if err != nil {
		if errors.Is(err, service.ErrAGSInvalid) {
			return responses.BadRequest(c, err.Error())
		}
		return responses.InternalError(c, "Could not create line item")
	}

	c.Set("Content-Type", "application/vnd.ims.lis.v2.lineitem+json")
	return c.Status(fiber.StatusCreated).JSON(lineItemToJSON(item, lineItemsBaseURL(c)))
}

// requireLineItemInCourse loads an LTI line item and enforces the F-012/F-013
// parent-tie: the item must belong to the course named in the URL. Returns
// (item, wrote); wrote==true means a 4xx was already written and the caller
// must `return nil`. Mirrors DeleteLineItem's inline check; existence-leak
// contract → 404 on any mismatch. Without this, GetLineItem/UpdateLineItem/
// PostScore/GetResults resolved a line item by :id alone, allowing cross-tenant
// grade read and write (an actor in course A could address a line item owned by
// course B in another tenant; the enrolled/instructor guard only ties the
// caller to :course_id, not the line item to that course).
//
// A tool caller may only address line items its developer key owns (G128).
func (h *LTIHandler) requireLineItemInCourse(c *fiber.Ctx) (*models.LTILineItem, bool) {
	id, err := c.ParamsInt("id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid line item ID")
		return nil, true
	}
	urlCourseID, err := c.ParamsInt("course_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid course ID")
		return nil, true
	}
	item, err := h.agsService.GetLineItem(c.Context(), uint(id))
	if err != nil {
		_ = responses.NotFound(c, "line item")
		return nil, true
	}
	if item.CourseID != uint(urlCourseID) {
		_ = responses.NotFound(c, "line item")
		return nil, true
	}
	if caller := ltiToolCaller(c); caller != nil && (item.DeveloperKeyID == nil || *item.DeveloperKeyID != caller.DevKeyID) {
		_ = responses.NotFound(c, "line item")
		return nil, true
	}
	return item, false
}

// GetLineItem returns a single LTI line item.
// GET /api/v1/lti/courses/:course_id/line_items/:id
func (h *LTIHandler) GetLineItem(c *fiber.Ctx) error {
	item, wrote := h.requireLineItemInCourse(c)
	if wrote {
		return nil
	}

	c.Set("Content-Type", "application/vnd.ims.lis.v2.lineitem+json")
	return c.JSON(lineItemToJSON(item, lineItemsBaseURL(c)))
}

// UpdateLineItem updates an existing LTI line item.
// PUT /api/v1/lti/courses/:course_id/line_items/:id
func (h *LTIHandler) UpdateLineItem(c *fiber.Ctx) error {
	item, wrote := h.requireLineItemInCourse(c)
	if wrote {
		return nil
	}

	var input struct {
		Label             *string  `json:"label"`
		ScoreMaximum      *float64 `json:"scoreMaximum"`
		Tag               *string  `json:"tag"`
		ResourceID        *string  `json:"resourceId"`
		ResourceLinkIDStr *string  `json:"resourceLinkId"`
		StartDateTime     *string  `json:"startDateTime"`
		EndDateTime       *string  `json:"endDateTime"`
	}

	if err := decodeJSONBody(c, &input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Label != nil {
		item.Label = service.SanitizePlainText(*input.Label)
	}
	if input.ScoreMaximum != nil {
		item.ScoreMaximum = *input.ScoreMaximum
	}
	if input.Tag != nil {
		item.Tag = *input.Tag
	}
	if input.ResourceID != nil {
		item.ResourceID = *input.ResourceID
	}
	if input.ResourceLinkIDStr != nil {
		item.ResourceLinkIDStr = *input.ResourceLinkIDStr
	}
	if input.StartDateTime != nil {
		t, err := parseAGSTime(*input.StartDateTime)
		if err != nil {
			return responses.BadRequest(c, "startDateTime must be ISO 8601")
		}
		item.StartDateTime = t
	}
	if input.EndDateTime != nil {
		t, err := parseAGSTime(*input.EndDateTime)
		if err != nil {
			return responses.BadRequest(c, "endDateTime must be ISO 8601")
		}
		item.EndDateTime = t
	}

	if err := h.agsService.UpdateLineItem(c.Context(), item); err != nil {
		return responses.BadRequest(c, err.Error())
	}
	if ltiToolCaller(c) != nil {
		_ = h.agsService.SyncToolLineItemAssignment(c.Context(), item, callerAccountID(c))
	}

	c.Set("Content-Type", "application/vnd.ims.lis.v2.lineitem+json")
	return c.JSON(lineItemToJSON(item, lineItemsBaseURL(c)))
}

// DeleteLineItem deletes an LTI line item.
// DELETE /api/v1/lti/courses/:course_id/line_items/:id
//
// F-012 / F-011 (parent-tie): the handler resolves the line item under
// the caller's tenant first, then enforces line_item.course_id ==
// URL :course_id. A teacher in course 10 attempting to DELETE
// /lti/courses/10/line_items/<id from course 99> sees a 404, and the
// repo write never fires.
func (h *LTIHandler) DeleteLineItem(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid line item ID")
	}
	urlCourseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	accountID := callerAccountID(c)

	// Parent-tie load: verify the line item exists AND belongs to the
	// course in the URL. Existence-leak contract: any mismatch → 404.
	item, err := h.agsService.GetLineItem(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "line item")
	}
	if item.CourseID != uint(urlCourseID) {
		return responses.NotFound(c, "line item")
	}
	if caller := ltiToolCaller(c); caller != nil && (item.DeveloperKeyID == nil || *item.DeveloperKeyID != caller.DevKeyID) {
		return responses.NotFound(c, "line item")
	}

	if err := h.agsService.DeleteLineItem(c.Context(), uint(id), accountID); err != nil {
		return responses.NotFound(c, "line item")
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ltiUserID accepts the AGS score userId as a JSON string (what the LTI
// spec and Paper's own launch `sub` claim use) or a JSON number. B081.
type ltiUserID uint

func (u *ltiUserID) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if bytes.Equal(b, []byte("null")) {
		*u = 0
		return nil
	}
	s := string(b)
	if len(b) > 0 && b[0] == '"' {
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
	}
	n, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return fmt.Errorf("userId must be a numeric user id, got %q", s)
	}
	*u = ltiUserID(n)
	return nil
}

type postScoreRequest struct {
	UserID           ltiUserID `json:"userId"`
	ScoreGiven       *float64  `json:"scoreGiven"`
	ScoreMaximum     *float64  `json:"scoreMaximum"`
	ActivityProgress string    `json:"activityProgress"`
	GradingProgress  string    `json:"gradingProgress"`
	Timestamp        string    `json:"timestamp"`
	Comment          string    `json:"comment"`
}

// PostScore posts a score (result) to an LTI line item.
// POST /api/v1/lti/courses/:course_id/line_items/:id/scores
// Accepts application/vnd.ims.lis.v1.score+json (G128).
func (h *LTIHandler) PostScore(c *fiber.Ctx) error {
	item, wrote := h.requireLineItemInCourse(c)
	if wrote {
		return nil
	}

	var input postScoreRequest
	if err := decodeJSONBody(c, &input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.UserID == 0 {
		return responses.BadRequest(c, "userId is required")
	}
	if input.ActivityProgress == "" {
		return responses.BadRequest(c, "activityProgress is required")
	}
	if input.GradingProgress == "" {
		return responses.BadRequest(c, "gradingProgress is required")
	}

	result := &models.LTIResult{
		UserID:           uint(input.UserID),
		ResultScore:      input.ScoreGiven,
		ResultMaximum:    input.ScoreMaximum,
		ActivityProgress: input.ActivityProgress,
		GradingProgress:  input.GradingProgress,
		Comment:          service.SanitizePlainText(input.Comment),
	}

	// Parse the timestamp if provided
	if input.Timestamp != "" {
		t, parseErr := time.Parse(time.RFC3339, input.Timestamp)
		if parseErr != nil {
			return responses.BadRequest(c, "Invalid timestamp format. Use RFC3339 (e.g., 2024-01-15T10:30:00Z)")
		}
		result.Timestamp = &t
	}

	// A tool's score has no Paper grader (Canvas records none either).
	var graderID uint
	if ltiToolCaller(c) == nil {
		graderID, _ = c.Locals("user_id").(uint)
	}
	if err := h.agsService.PostScore(c.Context(), item, callerAccountID(c), graderID, result); err != nil {
		switch {
		case errors.Is(err, service.ErrAGSInvalid):
			return responses.BadRequest(c, err.Error())
		case errors.Is(err, service.ErrAGSConflict):
			return responses.Error(c, fiber.StatusConflict, err.Error())
		case errors.Is(err, service.ErrAGSUserNotInCourse), errors.Is(err, service.ErrAGSGradeRefused):
			return responses.Error(c, fiber.StatusUnprocessableEntity, err.Error())
		case errors.Is(err, service.ErrAGSNotFound):
			return responses.NotFound(c, "line item")
		default:
			return responses.InternalError(c, "Could not save score")
		}
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ltiResultToJSON serializes an LTI result in the AGS result shape: id is
// the result URL and scoreOf the line item URL (G128).
func ltiResultToJSON(result *models.LTIResult, lineItemURL string) fiber.Map {
	r := fiber.Map{
		"id":               fmt.Sprintf("%s/results/%d", lineItemURL, result.ID),
		"scoreOf":          lineItemURL,
		"userId":           strconv.FormatUint(uint64(result.UserID), 10),
		"activityProgress": result.ActivityProgress,
		"gradingProgress":  result.GradingProgress,
		"comment":          result.Comment,
		"timestamp":        result.Timestamp,
	}

	if result.ResultScore != nil {
		r["resultScore"] = *result.ResultScore
	}
	if result.ResultMaximum != nil {
		r["resultMaximum"] = *result.ResultMaximum
	}

	return r
}

// GetResults returns the results (scores) for an LTI line item.
// GET /api/v1/lti/courses/:course_id/line_items/:id/results
//
// B079: only course staff (and tools holding result.readonly) see the
// whole class. Anyone else (the route is `enrolled`) sees only their own
// result, whatever ?user_id they pass. Staff may narrow to one learner
// with the AGS ?user_id filter.
func (h *LTIHandler) GetResults(c *fiber.Ctx) error {
	item, wrote := h.requireLineItemInCourse(c)
	if wrote {
		return nil
	}

	var filterUserID uint
	if ltiToolCaller(c) != nil || callerIsCourseStaff(c) {
		if raw := c.Query("user_id"); raw != "" {
			n, err := strconv.ParseUint(raw, 10, 64)
			if err != nil {
				return responses.BadRequest(c, "Invalid user_id")
			}
			filterUserID = uint(n)
		}
	} else {
		self, _ := c.Locals("user_id").(uint)
		if self == 0 {
			return responses.Unauthorized(c)
		}
		filterUserID = self
	}

	params := middleware.GetPagination(c)
	if agsLimit(c, &params) {
		return nil
	}

	resultSet, err := h.agsService.GetResults(c.Context(), item.ID, filterUserID, params)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	responses.SetPaginationHeaders(c, resultSet.TotalCount, resultSet.Page, resultSet.PerPage)

	lineItemURL := fmt.Sprintf("%s/%d", lineItemsBaseURL(c), item.ID)
	results := make([]fiber.Map, len(resultSet.Items))
	for i, result := range resultSet.Items {
		results[i] = ltiResultToJSON(&result, lineItemURL)
	}

	c.Set("Content-Type", "application/vnd.ims.lis.v2.resultcontainer+json")
	return c.JSON(results)
}

// --------------------------------------------------------------------------
// NRPS (Names and Role Provisioning Services) Endpoint
// --------------------------------------------------------------------------

// nrpsMaxLimit caps the NRPS `limit` page size.
const nrpsMaxLimit = 100

// GetMemberships returns course memberships in LTI NRPS format.
// GET /api/v1/lti/courses/:course_id/memberships (alias: names_and_roles)
//
// B188: the response is paged and carries a Link header (rel="next" while
// more members remain), which is how NRPS clients discover the rest of
// the roster. The NRPS `limit` query parameter sets the page size.
// G129: `role` filters by LIS role; the context carries the course label
// and title; a tool caller sees only the PII its privacy level allows.
func (h *LTIHandler) GetMemberships(c *fiber.Ctx) error {
	courseID, err := strconv.Atoi(c.Params("course_id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	// Gap-2: the NRPS roster (names, emails, LTI roles) is staff/tool data.
	// The `enrolled` route guard alone let any enrolled student pull the full
	// course roster. Restrict to course staff (teacher/TA/admin) or a tool.
	caller := ltiToolCaller(c)
	if caller == nil && !callerIsCourseStaff(c) {
		return responses.Forbidden(c, "not authorized to view course memberships")
	}

	params := middleware.GetPagination(c)
	if agsLimit(c, &params) {
		return nil
	}

	filter := service.NRPSFilter{Role: c.Query("role")}
	if caller != nil {
		filter.Privacy = h.ltiService.ToolPrivacyLevel(c.Context(), caller.DevKeyID)
	}
	accountID := callerAccountID(c)
	members, total, err := h.nrpsService.GetMembershipsFiltered(c.Context(), uint(courseID), accountID, filter, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch memberships")
	}

	responses.SetPaginationHeaders(c, total, params.Page, params.PerPage)

	label, title := h.nrpsService.CourseContext(c.Context(), uint(courseID), accountID)
	c.Set("Content-Type", "application/vnd.ims.lti-nrps.v2.membershipcontainer+json")
	return c.JSON(fiber.Map{
		"id": fmt.Sprintf("%s%s", c.BaseURL(), c.OriginalURL()),
		"context": fiber.Map{
			"id":    strconv.Itoa(courseID),
			"label": label,
			"title": title,
		},
		"members": members,
	})
}
