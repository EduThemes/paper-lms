package handlers

import (
	"fmt"
	"html/template"
	"strconv"
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
// and the id_token value. Compiled once at package init; Execute is
// goroutine-safe.
var ltiLaunchTemplate = template.Must(template.New("lti_launch").Parse(`<!DOCTYPE html>
<html>
<head><title>LTI Launch</title></head>
<body>
<form id="lti_launch_form" action="{{.RedirectURI}}" method="POST">
    <input type="hidden" name="id_token" value="{{.IDToken}}" />
    <input type="hidden" name="state" value="" />
    <noscript><input type="submit" value="Continue" /></noscript>
</form>
<script>document.getElementById('lti_launch_form').submit();</script>
</body>
</html>`))

// RenderLTILaunchForTest exposes the launch-HTML template render so
// tests in this package's external _test package can assert on the
// escape contract (F-056). NOT for production callers.
func RenderLTILaunchForTest(w interface{ Write([]byte) (int, error) }, redirectURI, idToken string) error {
	return ltiLaunchTemplate.Execute(w, struct {
		RedirectURI string
		IDToken     string
	}{RedirectURI: redirectURI, IDToken: idToken})
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

// --------------------------------------------------------------------------
// JWKS Endpoint
// --------------------------------------------------------------------------

// JWKS returns the platform's public keys in JSON Web Key Set format.
// GET /api/v1/lti/jwks (PUBLIC - no auth)
func (h *LTIHandler) JWKS(c *fiber.Ctx) error {
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

// OIDCLogin handles the LTI 1.3 OIDC third-party login initiation.
// POST /api/v1/lti/oidc/login (PUBLIC)
//
// The tool platform receives the login initiation request from the browser
// and responds with a redirect to the tool's OIDC authorization endpoint.
func (h *LTIHandler) OIDCLogin(c *fiber.Ctx) error {
	// Accept both form-encoded and JSON bodies
	clientID := c.FormValue("client_id")
	loginHint := c.FormValue("login_hint")
	targetLinkURI := c.FormValue("target_link_uri")
	ltiMessageHint := c.FormValue("lti_message_hint")

	// Fall back to JSON body if form values are empty
	if clientID == "" {
		var body struct {
			ClientID       string `json:"client_id"`
			LoginHint      string `json:"login_hint"`
			TargetLinkURI  string `json:"target_link_uri"`
			LTIMessageHint string `json:"lti_message_hint"`
			Iss            string `json:"iss"`
		}
		if err := c.BodyParser(&body); err == nil {
			clientID = body.ClientID
			loginHint = body.LoginHint
			targetLinkURI = body.TargetLinkURI
			ltiMessageHint = body.LTIMessageHint
		}
	}

	if clientID == "" {
		return responses.BadRequest(c, "client_id is required")
	}
	if targetLinkURI == "" {
		return responses.BadRequest(c, "target_link_uri is required")
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
// LTI Launch
// --------------------------------------------------------------------------

// --------------------------------------------------------------------------
// LTI Launch
// --------------------------------------------------------------------------

// LaunchDirect handles the LTI 1.3 resource link launch with direct config lookup.
// This is a more complete implementation that resolves the tool configuration
// through the developer key ID stored in the message hint.
//
// POST /api/v1/lti/launch (PUBLIC)
//
// Message hint format: "courseID:resourceLinkID:developerKeyID"
func (h *LTIHandler) LaunchDirect(c *fiber.Ctx) error {
	loginHint := c.FormValue("login_hint")
	ltiMessageHint := c.FormValue("lti_message_hint")

	if loginHint == "" {
		var body struct {
			LoginHint      string `json:"login_hint"`
			LTIMessageHint string `json:"lti_message_hint"`
		}
		if err := c.BodyParser(&body); err == nil {
			loginHint = body.LoginHint
			ltiMessageHint = body.LTIMessageHint
		}
	}

	// SECURITY (F-055): the session JWT is the source of identity, not
	// the login_hint form value. See OIDCLogin and Launch above.
	userID, ok := h.requireSessionUserMatches(c, loginHint)
	if !ok {
		return nil
	}

	// 13.4 (Wave C.2) — COPPA gate.
	if h.gateLTILaunchForCOPPA(c, userID) {
		return nil
	}

	// Parse the message hint to extract course ID, resource link ID, and developer key ID
	courseID, resourceLinkID, devKeyID, err := parseExtendedMessageHint(ltiMessageHint)
	if err != nil {
		return responses.BadRequest(c, "Invalid lti_message_hint")
	}

	// Look up the tool configuration by developer key ID
	toolConfig, err := h.configRepo.FindByDeveloperKeyID(c.Context(), devKeyID)
	if err != nil {
		return responses.BadRequest(c, "LTI tool configuration not found")
	}

	// Build the signed LTI launch token
	idToken, err := h.ltiService.BuildLaunchToken(
		c.Context(),
		userID,
		courseID,
		resourceLinkID,
		toolConfig,
	)
	if err != nil {
		return responses.InternalError(c, "Could not build launch token")
	}

	// SECURITY (F-056): redirect_uri is always the tool config's
	// registered TargetLinkURI. The pre-fix path read it from the request
	// body and HTML-injected it into the action= attribute via Sprintf,
	// letting an attacker craft `"><script>...</script>` payloads.
	c.Set("Content-Type", "text/html; charset=utf-8")
	return ltiLaunchTemplate.Execute(c.Response().BodyWriter(), struct {
		RedirectURI string
		IDToken     string
	}{RedirectURI: toolConfig.TargetLinkURI, IDToken: idToken})
}

// parseExtendedMessageHint parses a message hint string in the format
// "courseID:resourceLinkID:developerKeyID".
func parseExtendedMessageHint(hint string) (courseID uint, resourceLinkID string, devKeyID uint, err error) {
	if hint == "" {
		return 0, "", 0, fmt.Errorf("empty message hint")
	}

	// Split on colons
	parts := splitOnColons(hint, 3)

	if len(parts) < 1 {
		return 0, "", 0, fmt.Errorf("invalid message hint format")
	}

	// Parse course ID
	cid, parseErr := strconv.ParseUint(parts[0], 10, 64)
	if parseErr != nil {
		return 0, "", 0, fmt.Errorf("invalid course ID in message hint")
	}
	courseID = uint(cid)

	// Parse resource link ID (optional)
	if len(parts) >= 2 {
		resourceLinkID = parts[1]
	}

	// Parse developer key ID (optional but needed for tool config lookup)
	if len(parts) >= 3 && parts[2] != "" {
		dkid, parseErr := strconv.ParseUint(parts[2], 10, 64)
		if parseErr != nil {
			return 0, "", 0, fmt.Errorf("invalid developer key ID in message hint")
		}
		devKeyID = uint(dkid)
	}

	return courseID, resourceLinkID, devKeyID, nil
}

// splitOnColons splits a string on colon characters, returning up to maxParts parts.
func splitOnColons(s string, maxParts int) []string {
	var parts []string
	start := 0
	for i, ch := range s {
		if ch == ':' && len(parts) < maxParts-1 {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// --------------------------------------------------------------------------
// AGS (Assignment and Grade Services) Endpoints
// --------------------------------------------------------------------------

// lineItemToJSON serializes an LTI line item for API responses.
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

	return result
}

// ListLineItems returns a paginated list of LTI line items for a course.
// GET /api/v1/lti/courses/:course_id/line_items
func (h *LTIHandler) ListLineItems(c *fiber.Ctx) error {
	courseID, err := strconv.Atoi(c.Params("course_id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.agsService.ListLineItems(c.Context(), uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch line items")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	baseURL := c.BaseURL() + c.Path()
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
}

// CreateLineItem creates a new LTI line item for a course.
// POST /api/v1/lti/courses/:course_id/line_items
func (h *LTIHandler) CreateLineItem(c *fiber.Ctx) error {
	courseID, err := strconv.Atoi(c.Params("course_id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input createLineItemRequest
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Label == "" {
		return responses.BadRequest(c, "Line item label is required")
	}
	if input.ScoreMaximum <= 0 {
		return responses.BadRequest(c, "scoreMaximum must be greater than 0")
	}

	item := &models.LTILineItem{
		Label:             input.Label,
		ScoreMaximum:      input.ScoreMaximum,
		Tag:               input.Tag,
		ResourceID:        input.ResourceID,
		ResourceLinkIDStr: input.ResourceLinkIDStr,
		AssignmentID:      input.AssignmentID,
	}

	if err := h.agsService.CreateLineItem(c.Context(), uint(courseID), item); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	baseURL := c.BaseURL() + c.Path()
	c.Set("Content-Type", "application/vnd.ims.lis.v2.lineitem+json")
	return c.Status(fiber.StatusCreated).JSON(lineItemToJSON(item, baseURL))
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
	return item, false
}

// GetLineItem returns a single LTI line item.
// GET /api/v1/lti/courses/:course_id/line_items/:id
func (h *LTIHandler) GetLineItem(c *fiber.Ctx) error {
	item, wrote := h.requireLineItemInCourse(c)
	if wrote {
		return nil
	}

	baseURL := fmt.Sprintf("%s%s", c.BaseURL(), c.Path())
	c.Set("Content-Type", "application/vnd.ims.lis.v2.lineitem+json")
	return c.JSON(lineItemToJSON(item, baseURL))
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
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Label != nil {
		item.Label = *input.Label
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

	if err := h.agsService.UpdateLineItem(c.Context(), item); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	baseURL := fmt.Sprintf("%s%s", c.BaseURL(), c.Path())
	c.Set("Content-Type", "application/vnd.ims.lis.v2.lineitem+json")
	return c.JSON(lineItemToJSON(item, baseURL))
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

	if err := h.agsService.DeleteLineItem(c.Context(), uint(id), accountID); err != nil {
		return responses.NotFound(c, "line item")
	}

	return c.SendStatus(fiber.StatusNoContent)
}

type postScoreRequest struct {
	UserID           uint     `json:"userId"`
	ScoreGiven       *float64 `json:"scoreGiven"`
	ScoreMaximum     *float64 `json:"scoreMaximum"`
	ActivityProgress string   `json:"activityProgress"`
	GradingProgress  string   `json:"gradingProgress"`
	Timestamp        string   `json:"timestamp"`
	Comment          string   `json:"comment"`
}

// PostScore posts a score (result) to an LTI line item.
// POST /api/v1/lti/courses/:course_id/line_items/:id/scores
func (h *LTIHandler) PostScore(c *fiber.Ctx) error {
	item, wrote := h.requireLineItemInCourse(c)
	if wrote {
		return nil
	}

	var input postScoreRequest
	if err := c.BodyParser(&input); err != nil {
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
		UserID:           input.UserID,
		ResultScore:      input.ScoreGiven,
		ResultMaximum:    input.ScoreMaximum,
		ActivityProgress: input.ActivityProgress,
		GradingProgress:  input.GradingProgress,
		Comment:          input.Comment,
	}

	// Parse the timestamp if provided
	if input.Timestamp != "" {
		t, parseErr := time.Parse(time.RFC3339, input.Timestamp)
		if parseErr != nil {
			return responses.BadRequest(c, "Invalid timestamp format. Use RFC3339 (e.g., 2024-01-15T10:30:00Z)")
		}
		result.Timestamp = &t
	}

	if err := h.agsService.PostScore(c.Context(), item.ID, result); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ltiResultToJSON serializes an LTI result for API responses.
func ltiResultToJSON(result *models.LTIResult) fiber.Map {
	r := fiber.Map{
		"id":               result.ID,
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

// GetResults returns all results (scores) for an LTI line item.
// GET /api/v1/lti/courses/:course_id/line_items/:id/results
func (h *LTIHandler) GetResults(c *fiber.Ctx) error {
	item, wrote := h.requireLineItemInCourse(c)
	if wrote {
		return nil
	}

	params := middleware.GetPagination(c)

	resultSet, err := h.agsService.GetResults(c.Context(), item.ID, params)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	responses.SetPaginationHeaders(c, resultSet.TotalCount, resultSet.Page, resultSet.PerPage)

	results := make([]fiber.Map, len(resultSet.Items))
	for i, result := range resultSet.Items {
		results[i] = ltiResultToJSON(&result)
	}

	c.Set("Content-Type", "application/vnd.ims.lis.v2.resultcontainer+json")
	return c.JSON(results)
}

// --------------------------------------------------------------------------
// NRPS (Names and Role Provisioning Services) Endpoint
// --------------------------------------------------------------------------

// GetMemberships returns course memberships in LTI NRPS format.
// GET /api/v1/lti/courses/:course_id/memberships
func (h *LTIHandler) GetMemberships(c *fiber.Ctx) error {
	courseID, err := strconv.Atoi(c.Params("course_id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	// Gap-2: the NRPS roster (names, emails, LTI roles) is staff/tool data.
	// The `enrolled` route guard alone let any enrolled student pull the full
	// course roster. Restrict to course staff (teacher/TA/admin).
	if !callerIsCourseStaff(c) {
		return responses.Forbidden(c, "not authorized to view course memberships")
	}

	params := middleware.GetPagination(c)

	members, err := h.nrpsService.GetMemberships(c.Context(), uint(courseID), callerAccountID(c), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch memberships")
	}

	c.Set("Content-Type", "application/vnd.ims.lti-nrps.v2.membershipcontainer+json")
	return c.JSON(fiber.Map{
		"id": fmt.Sprintf("%s%s", c.BaseURL(), c.Path()),
		"context": fiber.Map{
			"id":    strconv.Itoa(courseID),
			"label": "",
			"title": "",
		},
		"members": members,
	})
}
