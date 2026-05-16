package handlers

import (
	"fmt"
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
	user, err := h.userRepo.FindByID(c.Context(), userID)
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
	if loginHint == "" {
		return responses.BadRequest(c, "login_hint is required")
	}
	if targetLinkURI == "" {
		return responses.BadRequest(c, "target_link_uri is required")
	}

	// 13.4 (Wave C.2) — COPPA gate on the initiation step too. Refusing
	// at /oidc/login avoids round-tripping the user's identity to the
	// tool's OIDC endpoint before we decide to refuse.
	if userID, parseErr := strconv.ParseUint(loginHint, 10, 64); parseErr == nil {
		if h.gateLTILaunchForCOPPA(c, uint(userID)) {
			return nil
		}
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

// Launch handles the LTI 1.3 resource link launch.
// POST /api/v1/lti/launch (PUBLIC)
//
// After the OIDC flow completes, the tool redirects back to this endpoint.
// The platform generates and returns a signed LTI launch JWT via an HTML
// auto-submit form (form_post response mode).
func (h *LTIHandler) Launch(c *fiber.Ctx) error {
	// The launch typically receives the user context from the OIDC flow.
	// The login_hint contains the user ID, and the lti_message_hint contains
	// the resource link context.
	loginHint := c.FormValue("login_hint")
	clientID := c.FormValue("client_id")
	ltiMessageHint := c.FormValue("lti_message_hint")
	redirectURI := c.FormValue("redirect_uri")

	// Fall back to JSON
	if loginHint == "" {
		var body struct {
			LoginHint      string `json:"login_hint"`
			ClientID       string `json:"client_id"`
			LTIMessageHint string `json:"lti_message_hint"`
			RedirectURI    string `json:"redirect_uri"`
		}
		if err := c.BodyParser(&body); err == nil {
			loginHint = body.LoginHint
			clientID = body.ClientID
			ltiMessageHint = body.LTIMessageHint
			redirectURI = body.RedirectURI
		}
	}

	if loginHint == "" {
		return responses.BadRequest(c, "login_hint is required")
	}
	if clientID == "" {
		return responses.BadRequest(c, "client_id is required")
	}

	// Parse the user ID from the login hint
	userID, err := strconv.ParseUint(loginHint, 10, 64)
	if err != nil {
		return responses.BadRequest(c, "Invalid login_hint")
	}

	// 13.4 (Wave C.2) — COPPA gate. K-12 tenant + no granted
	// third_party_sharing parental consent = refused.
	if h.gateLTILaunchForCOPPA(c, uint(userID)) {
		return nil
	}

	// Parse the LTI message hint to extract course ID and resource link ID.
	// The message hint format is: "courseID:resourceLinkID"
	courseID, resourceLinkID, err := parseLTIMessageHint(ltiMessageHint)
	if err != nil {
		return responses.BadRequest(c, "Invalid lti_message_hint")
	}

	// Look up the LTI tool configuration by client_id
	toolConfig, err := h.findToolConfigByClientID(c, clientID)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	// Build the signed LTI launch token
	idToken, err := h.ltiService.BuildLaunchToken(
		c.Context(),
		uint(userID),
		courseID,
		resourceLinkID,
		toolConfig,
	)
	if err != nil {
		return responses.InternalError(c, "Could not build launch token")
	}

	// Determine the redirect URI (use the tool's target_link_uri if not
	// provided in the request)
	if redirectURI == "" {
		redirectURI = toolConfig.TargetLinkURI
	}

	// Return an HTML auto-submit form (form_post response mode)
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><title>LTI Launch</title></head>
<body>
<form id="lti_launch_form" action="%s" method="POST">
    <input type="hidden" name="id_token" value="%s" />
    <input type="hidden" name="state" value="" />
    <noscript><input type="submit" value="Continue" /></noscript>
</form>
<script>document.getElementById('lti_launch_form').submit();</script>
</body>
</html>`, redirectURI, idToken)

	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.SendString(html)
}

// findToolConfigByClientID looks up the LTI tool configuration associated
// with the given client_id through the developer key.
func (h *LTIHandler) findToolConfigByClientID(c *fiber.Ctx, clientID string) (*models.LTIToolConfiguration, error) {
	// We need to find the developer key by its client_id, then look up the
	// LTI tool configuration by developer key ID. Since we don't have a
	// direct service dependency on DeveloperKeyService here, we use the
	// configRepo which can find by developer key ID. First, we need to
	// find the developer key ID.
	//
	// The LTI service already validates the client_id internally, but we
	// need the tool config to build the launch token. We'll iterate through
	// the approach of using the lti_message_hint or looking up by client_id.
	//
	// For simplicity, use the LTI service's internal repos via the configRepo.
	// The handler has access to configRepo.
	//
	// We need a way to go from clientID -> devKey -> toolConfig.
	// Since we only have configRepo.FindByDeveloperKeyID, we need the devKeyID.
	// The LTI handler doesn't directly have the devKeyRepo, so we pass
	// through the ltiService which does. For this handler, we add the lookup
	// indirectly.
	//
	// Actually, let's just use the fact that InitiateLogin validates the
	// client_id and finds the tool config. For the launch endpoint, we can
	// store the necessary info in the message_hint or look it up.
	//
	// The cleanest approach: search all tool configs. In practice this would
	// be a direct lookup. For now, since the configRepo does not have a
	// FindByClientID method, we return an error and let the caller handle it.

	// This is handled through the LTI service internally. The handler
	// should not need to do this lookup directly. Instead, we can extend
	// the approach to pass the tool config through the message hint.
	return nil, fmt.Errorf("tool configuration lookup by client_id requires LTI service")
}

// parseLTIMessageHint parses a message hint string in the format
// "courseID:resourceLinkID" and returns the individual components.
func parseLTIMessageHint(hint string) (courseID uint, resourceLinkID string, err error) {
	if hint == "" {
		return 0, "", fmt.Errorf("empty message hint")
	}

	// Find the colon separator
	colonIdx := -1
	for i, ch := range hint {
		if ch == ':' {
			colonIdx = i
			break
		}
	}

	if colonIdx < 0 {
		// Try parsing the entire hint as a course ID with a generated resource link
		id, parseErr := strconv.ParseUint(hint, 10, 64)
		if parseErr != nil {
			return 0, "", fmt.Errorf("invalid message hint format")
		}
		return uint(id), "", nil
	}

	courseIDStr := hint[:colonIdx]
	resourceLinkID = hint[colonIdx+1:]

	id, parseErr := strconv.ParseUint(courseIDStr, 10, 64)
	if parseErr != nil {
		return 0, "", fmt.Errorf("invalid course ID in message hint")
	}

	return uint(id), resourceLinkID, nil
}

// --------------------------------------------------------------------------
// LTI Launch (Improved - Direct Token Build)
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
	redirectURI := c.FormValue("redirect_uri")

	if loginHint == "" {
		var body struct {
			LoginHint      string `json:"login_hint"`
			LTIMessageHint string `json:"lti_message_hint"`
			RedirectURI    string `json:"redirect_uri"`
		}
		if err := c.BodyParser(&body); err == nil {
			loginHint = body.LoginHint
			ltiMessageHint = body.LTIMessageHint
			redirectURI = body.RedirectURI
		}
	}

	if loginHint == "" {
		return responses.BadRequest(c, "login_hint is required")
	}

	// Parse user ID from login hint
	userID, err := strconv.ParseUint(loginHint, 10, 64)
	if err != nil {
		return responses.BadRequest(c, "Invalid login_hint")
	}

	// 13.4 (Wave C.2) — COPPA gate.
	if h.gateLTILaunchForCOPPA(c, uint(userID)) {
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
		uint(userID),
		courseID,
		resourceLinkID,
		toolConfig,
	)
	if err != nil {
		return responses.InternalError(c, "Could not build launch token")
	}

	if redirectURI == "" {
		redirectURI = toolConfig.TargetLinkURI
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><title>LTI Launch</title></head>
<body>
<form id="lti_launch_form" action="%s" method="POST">
    <input type="hidden" name="id_token" value="%s" />
    <input type="hidden" name="state" value="" />
    <noscript><input type="submit" value="Continue" /></noscript>
</form>
<script>document.getElementById('lti_launch_form').submit();</script>
</body>
</html>`, redirectURI, idToken)

	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.SendString(html)
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

// GetLineItem returns a single LTI line item.
// GET /api/v1/lti/courses/:course_id/line_items/:id
func (h *LTIHandler) GetLineItem(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid line item ID")
	}

	item, err := h.agsService.GetLineItem(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "line item")
	}

	baseURL := fmt.Sprintf("%s%s", c.BaseURL(), c.Path())
	c.Set("Content-Type", "application/vnd.ims.lis.v2.lineitem+json")
	return c.JSON(lineItemToJSON(item, baseURL))
}

// UpdateLineItem updates an existing LTI line item.
// PUT /api/v1/lti/courses/:course_id/line_items/:id
func (h *LTIHandler) UpdateLineItem(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid line item ID")
	}

	item, err := h.agsService.GetLineItem(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "line item")
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
func (h *LTIHandler) DeleteLineItem(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid line item ID")
	}

	if err := h.agsService.DeleteLineItem(c.Context(), uint(id)); err != nil {
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
	lineItemID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid line item ID")
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

	if err := h.agsService.PostScore(c.Context(), uint(lineItemID), result); err != nil {
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
	lineItemID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid line item ID")
	}

	params := middleware.GetPagination(c)

	resultSet, err := h.agsService.GetResults(c.Context(), uint(lineItemID), params)
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

	params := middleware.GetPagination(c)

	members, err := h.nrpsService.GetMemberships(c.Context(), uint(courseID), params)
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
