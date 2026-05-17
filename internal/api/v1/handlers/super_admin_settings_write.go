package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/service/settings"
	"github.com/EduThemes/paper-lms/internal/storage"
)

// Wave 3 write API + test actions for the Super-Admin Settings Engine.
// Three groups of endpoints, all gated by RequireSuperAdmin via
// registerSuperAdminRoutes:
//
//	PUT    /api/v1/superadmin/settings/:key     — set / replace
//	DELETE /api/v1/superadmin/settings/:key     — clear (falls back)
//	POST   /api/v1/superadmin/settings/test/email
//	POST   /api/v1/superadmin/settings/test/oidc
//	POST   /api/v1/superadmin/settings/test/anthropic
//	POST   /api/v1/superadmin/settings/test/s3
//
// SECURITY CONTRACT (Wave 3-specific additions to the Wave 2 contract
// in super_admin_settings.go):
//
//   1. Write endpoints accept scope+scope_id from the JSON body, not
//      from the URL. The handler validates that scope_id is consistent
//      with the chosen scope (instance ⇒ 0; account/user ⇒ non-zero +
//      existence check) before delegating to the service.
//
//   2. Test actions NEVER take credential overrides in the body.
//      Whatever the resolved-effective settings hold IS what's tested.
//      Adversarial framing: a body-overridable api_key parameter on
//      /test/anthropic would let any super-admin exfiltrate the
//      bootstrap-time Anthropic key by routing the test ping through
//      an attacker-controlled URL with their own key.
//
//   3. The email test sends ONLY to the caller's own email (resolved
//      from the authenticated session, not the body). The endpoint is
//      not an open SMTP relay for any address an operator types in.
//
//   4. The OIDC discovery test enforces SSRF defense (validateExternalURL).
//      A super-admin's authority is the deployment's settings, not its
//      network position; this endpoint must not become a probe for
//      internal services.
//
//   5. The Anthropic test endpoint is HARD-CODED. No body parameter
//      can change which URL the API key is sent to.
//
//   6. The S3 test writes to a randomized key under a fixed prefix
//      (paper-lms-settings-test/). No body parameter controls the
//      object key — operators can't read or overwrite production
//      objects via this test.
//
//   7. ALL test endpoints emit an audit_log row (setting.tested)
//      regardless of whether the test succeeded or failed. Audit
//      payload carries the test name + duration_ms + success bool +
//      one-line error class (no secrets, no PII).
//
//   8. Each test endpoint sits behind SuperAdminTestRateLimit(action),
//      1 per 30s per (super_admin, action) to block loop-testing.

// ── PUT / DELETE ───────────────────────────────────────────────────

type setRequest struct {
	Scope   string `json:"scope"`
	ScopeID uint   `json:"scope_id"`
	Value   string `json:"value"`
}

// Set handles PUT /api/v1/superadmin/settings/:key. Body specifies
// the scope and scope_id at which to write; the URL parameter is the
// key, validated against the catalog inside the service.
//
// account-scope writes verify the account exists — a stray scope_id
// could otherwise create an orphan setting row keyed to a deleted or
// never-existing account, which would silently bind to a future
// recycled ID. (Audit L2 from the Wave 2 review.)
func (h *SuperAdminSettingsHandler) Set(c *fiber.Ctx) error {
	key := c.Params("key")
	if key == "" {
		return responses.BadRequest(c, "key is required")
	}

	var input setRequest
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid JSON body")
	}

	scope := settings.ScopeType(strings.TrimSpace(input.Scope))
	if scope == "" {
		return responses.BadRequest(c, "scope is required")
	}

	// Account-existence guard. The service layer already rejects
	// account/user scope with scope_id=0, but it doesn't verify the
	// row exists — leaving the door open for a setting bound to a
	// nonexistent account. Verify here, before the service.
	if scope == settings.ScopeAccount {
		if h.accountChecker == nil {
			return responses.InternalError(c, "account validator not wired")
		}
		if _, err := h.accountChecker.FindByID(c.Context(), input.ScopeID); err != nil {
			return responses.NotFound(c, "account")
		}
	}

	byUser, _ := c.Locals("user_id").(uint)
	if err := h.svc.Set(c.Context(), scope, input.ScopeID, key, input.Value, byUser); err != nil {
		return mapSettingsServiceError(c, err)
	}

	// Return the freshly-resolved effective value so the UI doesn't
	// have to round-trip a GET to refresh. Same masking contract as
	// Wave 2 read endpoints.
	def, _ := settings.Find(key)
	ev, _ := h.svc.Get(c.Context(), key, settings.ScopeHints{AccountID: input.ScopeID})
	return c.JSON(toResponse(def, ev))
}

// Clear handles DELETE /api/v1/superadmin/settings/:key. Body
// specifies scope + scope_id (DELETE-with-body is unusual but the
// alternative — encoding scope in the URL — bloats the route surface
// and makes Wave 4 form handling more complex). Idempotent: clearing
// an absent row is not an error.
//
// SECURITY (Wave 3 audit H2): scope is REQUIRED. A previous draft
// defaulted to scope=instance when the body was missing or invalid,
// which created a UX footgun where an operator who meant to clear an
// account override would accidentally clear the instance value (and
// then a surprise env-fallback or default would unmask). Destructive
// endpoint → explicit > implicit.
func (h *SuperAdminSettingsHandler) Clear(c *fiber.Ctx) error {
	key := c.Params("key")
	if key == "" {
		return responses.BadRequest(c, "key is required")
	}

	var input setRequest
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "invalid JSON body (scope is required)")
	}
	scope := settings.ScopeType(strings.TrimSpace(input.Scope))
	if scope == "" {
		return responses.BadRequest(c, "scope is required (one of instance|account|user)")
	}

	byUser, _ := c.Locals("user_id").(uint)
	if err := h.svc.Clear(c.Context(), scope, input.ScopeID, key, byUser); err != nil {
		return mapSettingsServiceError(c, err)
	}

	def, ok := settings.Find(key)
	if !ok {
		// The catalog lookup happens inside Clear; if it returned
		// nil error, the key WAS valid. This branch is defensive
		// (e.g. catalog mutated between calls).
		return c.JSON(fiber.Map{"cleared": true, "key": key})
	}
	ev, _ := h.svc.Get(c.Context(), key, settings.ScopeHints{AccountID: input.ScopeID})
	return c.JSON(toResponse(def, ev))
}

// mapSettingsServiceError converts service-layer error types into
// appropriate HTTP responses without leaking implementation detail.
func mapSettingsServiceError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, settings.ErrUnknownKey):
		return responses.NotFound(c, "setting")
	case errors.Is(err, settings.ErrScopeNotAllowed):
		return responses.BadRequest(c, "scope not allowed for this key")
	case errors.Is(err, settings.ErrInvalidValue):
		return responses.BadRequest(c, "value invalid for this setting's type")
	default:
		return responses.InternalError(c, "could not write setting")
	}
}

// ── Test actions ───────────────────────────────────────────────────

// testActionResponse is the JSON shape every /test/* endpoint emits.
// Detail carries human-readable diagnostics (e.g. "SMTP connection
// timed out after 10s") but NEVER credentials or response bodies that
// might contain secrets. Server side has full plaintext for debug
// logs; this struct is what crosses the API boundary.
type testActionResponse struct {
	OK         bool   `json:"ok"`
	Detail     string `json:"detail,omitempty"`
	DurationMs int64  `json:"duration_ms"`
	Action     string `json:"action"`
}

// auditTestAction emits the setting.tested audit row. Called from
// every test endpoint in a defer so success-and-failure both audit.
//
// SECURITY (Wave 3 audit M3/M4): forensic context — IP, user-agent,
// and the operator's home tenant (admin_account_id when present,
// account_id otherwise) — flow through to the audit row so a
// post-incident query can filter by tenant, identify the client
// fingerprint, and correlate against access logs.
func (h *SuperAdminSettingsHandler) auditTestAction(c *fiber.Ctx, byUser uint, action string, result *testActionResponse) {
	if h.audit == nil || result == nil {
		return
	}
	payloadStruct := struct {
		Action     string `json:"action"`
		Success    bool   `json:"success"`
		DurationMs int64  `json:"duration_ms"`
		Detail     string `json:"detail,omitempty"`
	}{
		Action:     action,
		Success:    result.OK,
		DurationMs: result.DurationMs,
		Detail:     result.Detail,
	}
	payload, _ := json.Marshal(payloadStruct)

	var accountID *uint
	if v, ok := c.Locals("admin_account_id").(uint); ok && v != 0 {
		accountID = &v
	} else if v, ok := c.Locals("account_id").(uint); ok && v != 0 {
		accountID = &v
	}
	userAgent := string(c.Request().Header.UserAgent())

	_ = h.audit.LogEvent(
		c.Context(),
		"setting_change",
		byUser,
		nil,
		accountID,
		"Setting",
		0,
		"setting.tested",
		string(payload),
		c.IP(),
		userAgent,
	)
}

// ── Email test ─────────────────────────────────────────────────────

const smtpTestTimeout = 15 * time.Second

// TestEmail sends a test message to the caller's OWN email address
// using the current effective SMTP settings. The "to" address is
// resolved from the authenticated session — NOT from the body — to
// prevent this endpoint becoming an open relay for arbitrary
// recipients via the deployment's SMTP creds.
func (h *SuperAdminSettingsHandler) TestEmail(c *fiber.Ctx) error {
	started := time.Now()
	byUser, _ := c.Locals("user_id").(uint)
	callerEmail, _ := c.Locals("user_email").(string)

	result := &testActionResponse{Action: "email"}
	defer func() {
		result.DurationMs = time.Since(started).Milliseconds()
		h.auditTestAction(c, byUser, "email", result)
	}()

	callerEmail = strings.TrimSpace(callerEmail)
	if callerEmail == "" || !strings.Contains(callerEmail, "@") {
		result.Detail = "no caller email in session — log in again"
		return c.Status(fiber.StatusBadRequest).JSON(result)
	}

	// Wave 8: scope SMTP resolution to the optional ?account_id
	// hint OR (default) the caller's home account. A super-admin
	// testing district SMTP can use ?account_id=<district> to
	// resolve from that district's overrides; otherwise the test
	// uses instance-scope plus the caller's account walk.
	hints := hintsFromRequest(c)
	if hints.AccountID == 0 {
		if v, ok := c.Locals("account_id").(uint); ok {
			hints.AccountID = v
		}
	}
	host, _ := h.resolveString(c.Context(), "smtp.host", hints)
	port, _ := h.resolveString(c.Context(), "smtp.port", hints)
	user, _ := h.resolveString(c.Context(), "smtp.username", hints)
	pass, _ := h.resolveString(c.Context(), "smtp.password", hints)
	from, _ := h.resolveString(c.Context(), "smtp.from", hints)

	if host == "" || port == "" || from == "" {
		result.Detail = "SMTP not configured: smtp.host, smtp.port, and smtp.from are required"
		return c.Status(fiber.StatusFailedDependency).JSON(result)
	}

	addr := host + ":" + port
	msg := []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: Paper LMS — test email\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=\"utf-8\"\r\n\r\nIf you received this, your SMTP configuration is working.\r\nSent at %s.\r\n",
		from, callerEmail, started.UTC().Format(time.RFC3339)))

	var auth smtp.Auth
	if user != "" {
		auth = smtp.PlainAuth("", user, pass, host)
	}

	// net/smtp doesn't expose a context — but we want a bounded
	// failure mode. Run the send in a goroutine and select on a
	// timeout. The orphaned goroutine will finish eventually
	// (smtp.SendMail's net.Dial has its own timeouts).
	sendErr := make(chan error, 1)
	go func() {
		sendErr <- smtp.SendMail(addr, auth, from, []string{callerEmail}, msg)
	}()

	select {
	case err := <-sendErr:
		if err != nil {
			result.Detail = sanitizeSMTPError(err)
			return c.Status(fiber.StatusBadGateway).JSON(result)
		}
		result.OK = true
		result.Detail = "test email sent to " + callerEmail
		return c.JSON(result)
	case <-time.After(smtpTestTimeout):
		result.Detail = fmt.Sprintf("SMTP send did not complete within %s", smtpTestTimeout)
		return c.Status(fiber.StatusGatewayTimeout).JSON(result)
	}
}

// sanitizeSMTPError keeps the error class but strips credentials —
// some SMTP server error messages echo the AUTH challenge or username
// back in the response. The audit log + API response should carry
// only the failure class, not the raw upstream message.
func sanitizeSMTPError(err error) string {
	msg := err.Error()
	if strings.Contains(strings.ToLower(msg), "auth") {
		return "SMTP authentication failed"
	}
	if strings.Contains(strings.ToLower(msg), "timeout") || strings.Contains(strings.ToLower(msg), "deadline") {
		return "SMTP connection timed out"
	}
	if strings.Contains(strings.ToLower(msg), "refused") {
		return "SMTP connection refused"
	}
	return "SMTP send failed"
}

// ── OIDC discovery test ────────────────────────────────────────────

const oidcDiscoveryTimeout = 10 * time.Second
const oidcDiscoveryMaxBytes = 64 * 1024

type oidcTestRequest struct {
	Issuer string `json:"issuer"`
}

// TestOIDC fetches /.well-known/openid-configuration from the
// given issuer URL and verifies the document parses + the issuer
// claim matches. SSRF defense via validateExternalURL — the body
// URL must resolve to public unicast space.
func (h *SuperAdminSettingsHandler) TestOIDC(c *fiber.Ctx) error {
	started := time.Now()
	byUser, _ := c.Locals("user_id").(uint)
	result := &testActionResponse{Action: "oidc"}
	defer func() {
		result.DurationMs = time.Since(started).Milliseconds()
		h.auditTestAction(c, byUser, "oidc", result)
	}()

	var input oidcTestRequest
	if err := c.BodyParser(&input); err != nil {
		result.Detail = "invalid JSON body"
		return c.Status(fiber.StatusBadRequest).JSON(result)
	}
	issuer := strings.TrimRight(strings.TrimSpace(input.Issuer), "/")
	if issuer == "" {
		result.Detail = "issuer URL required"
		return c.Status(fiber.StatusBadRequest).JSON(result)
	}

	discoveryURL := issuer + "/.well-known/openid-configuration"

	ctx, cancel := context.WithTimeout(c.Context(), oidcDiscoveryTimeout)
	defer cancel()

	if err := validateExternalURL(ctx, discoveryURL); err != nil {
		result.Detail = err.Error()
		return c.Status(fiber.StatusForbidden).JSON(result)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		result.Detail = "could not construct OIDC discovery request"
		return c.Status(fiber.StatusInternalServerError).JSON(result)
	}
	req.Header.Set("Accept", "application/json")

	// SECURITY (Wave 3 audit C1): refuse to follow redirects.
	// validateExternalURL runs once on the original body URL; if
	// Go's default redirect-follow let the discovery server 302
	// to http://169.254.169.254/latest/meta-data/... the SSRF
	// guard would be bypassed (the new URL is never re-validated
	// and even the https-only check is dropped by net/http). OIDC
	// discovery docs are conventionally a single GET; refusing
	// redirects is the canonical posture. http.ErrUseLastResponse
	// surfaces the 3xx as the response, which we then reject in
	// the status-code check below.
	httpClient := &http.Client{
		Timeout: oidcDiscoveryTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		result.Detail = "OIDC discovery request failed"
		return c.Status(fiber.StatusBadGateway).JSON(result)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		result.Detail = fmt.Sprintf("OIDC discovery returned HTTP %d", resp.StatusCode)
		return c.Status(fiber.StatusBadGateway).JSON(result)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, oidcDiscoveryMaxBytes))
	if err != nil {
		result.Detail = "OIDC discovery body read failed"
		return c.Status(fiber.StatusBadGateway).JSON(result)
	}

	var doc struct {
		Issuer string `json:"issuer"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		result.Detail = "OIDC discovery document is not valid JSON"
		return c.Status(fiber.StatusBadGateway).JSON(result)
	}
	if strings.TrimRight(doc.Issuer, "/") != issuer {
		result.Detail = "OIDC issuer claim does not match requested URL"
		return c.Status(fiber.StatusBadGateway).JSON(result)
	}

	result.OK = true
	result.Detail = "OIDC discovery document fetched and issuer verified"
	return c.JSON(result)
}

// ── Anthropic ping test ────────────────────────────────────────────

const (
	anthropicTestEndpoint = "https://api.anthropic.com/v1/messages"
	anthropicTestVersion  = "2023-06-01"
	anthropicTestModel    = "claude-haiku-4-5-20251001"
	anthropicTestTimeout  = 15 * time.Second
)

// TestAnthropic sends a 5-token "ping" message to the Anthropic
// Messages API using the current effective API key. The endpoint is
// hard-coded — no body parameter can change the destination URL.
// Returns input/output token counts on success.
func (h *SuperAdminSettingsHandler) TestAnthropic(c *fiber.Ctx) error {
	started := time.Now()
	byUser, _ := c.Locals("user_id").(uint)
	result := &testActionResponse{Action: "anthropic"}
	defer func() {
		result.DurationMs = time.Since(started).Milliseconds()
		h.auditTestAction(c, byUser, "anthropic", result)
	}()

	apiKey, _ := h.resolveString(c.Context(), "ai.anthropic.api_key", settings.ScopeHints{})
	if apiKey == "" {
		result.Detail = "ai.anthropic.api_key is not configured"
		return c.Status(fiber.StatusFailedDependency).JSON(result)
	}

	body := map[string]interface{}{
		"model":      anthropicTestModel,
		"max_tokens": 5,
		"messages":   []map[string]string{{"role": "user", "content": "ping"}},
	}
	payload, _ := json.Marshal(body)

	ctx, cancel := context.WithTimeout(c.Context(), anthropicTestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicTestEndpoint, bytes.NewReader(payload))
	if err != nil {
		result.Detail = "could not construct Anthropic request"
		return c.Status(fiber.StatusInternalServerError).JSON(result)
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", anthropicTestVersion)
	req.Header.Set("content-type", "application/json")

	// SECURITY (Wave 3 audit C1): refuse redirects. The Anthropic
	// API doesn't 3xx in practice, but a poisoned DNS response or
	// future-proofing concern is enough to lock it down. We don't
	// want the x-api-key header following a 302 to an attacker-
	// controlled host.
	httpClient := &http.Client{
		Timeout: anthropicTestTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		result.Detail = "Anthropic API request failed"
		return c.Status(fiber.StatusBadGateway).JSON(result)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		result.Detail = "Anthropic rejected the API key"
		return c.Status(fiber.StatusBadGateway).JSON(result)
	}
	if resp.StatusCode != http.StatusOK {
		result.Detail = fmt.Sprintf("Anthropic returned HTTP %d", resp.StatusCode)
		return c.Status(fiber.StatusBadGateway).JSON(result)
	}

	var parsed struct {
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	_ = json.Unmarshal(respBody, &parsed)
	result.OK = true
	result.Detail = fmt.Sprintf("Anthropic responded — input_tokens=%d output_tokens=%d", parsed.Usage.InputTokens, parsed.Usage.OutputTokens)
	return c.JSON(result)
}

// ── S3 round-trip test ─────────────────────────────────────────────

const s3TestKeyPrefix = "paper-lms-settings-test/"

// TestS3 exercises the storage backend by writing a small object,
// reading it back, and deleting it. The object key is randomized
// inside a fixed prefix — no body parameter controls the key, so an
// operator cannot probe or overwrite production objects through this
// endpoint.
func (h *SuperAdminSettingsHandler) TestS3(c *fiber.Ctx) error {
	started := time.Now()
	byUser, _ := c.Locals("user_id").(uint)
	result := &testActionResponse{Action: "s3"}
	defer func() {
		result.DurationMs = time.Since(started).Milliseconds()
		h.auditTestAction(c, byUser, "s3", result)
	}()

	hints := settings.ScopeHints{}
	bucket, _ := h.resolveString(c.Context(), "storage.s3.bucket", hints)
	region, _ := h.resolveString(c.Context(), "storage.s3.region", hints)
	endpoint, _ := h.resolveString(c.Context(), "storage.s3.endpoint", hints)
	accessKey, _ := h.resolveString(c.Context(), "storage.s3.access_key", hints)
	secretKey, _ := h.resolveString(c.Context(), "storage.s3.secret_key", hints)

	if bucket == "" || accessKey == "" || secretKey == "" {
		result.Detail = "S3 not configured: bucket, access_key, and secret_key required"
		return c.Status(fiber.StatusFailedDependency).JSON(result)
	}

	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()

	// Wave 6: this handler already resolves every storage.s3.* key via
	// the Settings Engine above, so the per-request lookup is wired here
	// as nil — the backend's boot snapshot already reflects the freshly
	// resolved values and we don't want a second round-trip per Put/Get.
	backend, err := storage.NewS3Backend(ctx, storage.S3Config{
		Bucket:    bucket,
		Region:    region,
		Endpoint:  endpoint,
		AccessKey: accessKey,
		SecretKey: secretKey,
	}, nil)
	if err != nil {
		result.Detail = "S3 client could not initialize"
		return c.Status(fiber.StatusBadGateway).JSON(result)
	}

	key, err := randomS3TestKey()
	if err != nil {
		result.Detail = "could not generate test key"
		return c.Status(fiber.StatusInternalServerError).JSON(result)
	}

	probeBody := []byte("paper-lms-settings-test " + started.UTC().Format(time.RFC3339))
	if err := backend.Put(ctx, key, bytes.NewReader(probeBody), "text/plain"); err != nil {
		result.Detail = "S3 PutObject failed"
		return c.Status(fiber.StatusBadGateway).JSON(result)
	}

	rc, err := backend.Get(ctx, key)
	if err != nil {
		_ = backend.Delete(ctx, key) // best-effort cleanup
		result.Detail = "S3 GetObject failed"
		return c.Status(fiber.StatusBadGateway).JSON(result)
	}
	got, _ := io.ReadAll(io.LimitReader(rc, int64(len(probeBody)+16)))
	_ = rc.Close()

	if err := backend.Delete(ctx, key); err != nil {
		// Object was written + read, but cleanup failed. Surface
		// this — leaving the test object behind is mildly bad
		// hygiene, but the round-trip itself succeeded.
		result.Detail = "wrote + read but DeleteObject failed (object " + key + " orphaned)"
		return c.Status(fiber.StatusBadGateway).JSON(result)
	}

	if !bytes.Equal(probeBody, got) {
		result.Detail = "S3 round-trip data mismatch"
		return c.Status(fiber.StatusBadGateway).JSON(result)
	}

	result.OK = true
	result.Detail = fmt.Sprintf("S3 write→read→delete succeeded against bucket=%s", bucket)
	return c.JSON(result)
}

func randomS3TestKey() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return s3TestKeyPrefix + hex.EncodeToString(raw) + ".txt", nil
}

// ── helpers ─────────────────────────────────────────────────────────

// resolveString resolves an effective value via the service and
// returns the plaintext string for server-side consumption. Returns
// "" + error if the key is unknown or if a secret fails to decrypt.
// Test actions call this for the credentials they need to exercise.
// The plaintext NEVER crosses the API boundary — it stays inside the
// test-action handler.
func (h *SuperAdminSettingsHandler) resolveString(ctx context.Context, key string, hints settings.ScopeHints) (string, error) {
	ev, err := h.svc.Get(ctx, key, hints)
	if err != nil {
		return "", err
	}
	if !ev.HasValue {
		return "", nil
	}
	return ev.Value, nil
}
