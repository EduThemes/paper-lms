package handlers_test

// DB-backed end-to-end tests of the LTI 1.3 / Advantage lane (G123-G134):
// registration, account installs, placements, launches + id_token claims,
// client_credentials service tokens, AGS/NRPS as a tool, deep linking,
// assignment tools, LTI 1.1 launches + outcomes, dynamic registration and
// platform key rotation. Gated on PARITY_DB_URL / DATABASE_URL.

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec // oauth_body_hash
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pgdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/db"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

const ltiIssuer = "https://lms.example.com"

func ltiLaneDB(t *testing.T) *gorm.DB {
	t.Helper()
	base := os.Getenv("PARITY_DB_URL")
	if base == "" {
		base = os.Getenv("DATABASE_URL")
	}
	if base == "" {
		t.Skip("set PARITY_DB_URL (or DATABASE_URL) to run DB-backed LTI tests")
	}
	swap := func(name string) string {
		u, err := url.Parse(base)
		require.NoError(t, err)
		u.Path = "/" + name
		return u.String()
	}
	admin, err := sql.Open("postgres", swap("postgres"))
	require.NoError(t, err)
	name := fmt.Sprintf("paper_lms_ltilane_%d", time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err = admin.ExecContext(ctx, fmt.Sprintf(`CREATE DATABASE %q`, name))
	require.NoError(t, err)
	bs, err := sql.Open("postgres", swap(name))
	require.NoError(t, err)
	_, err = bs.Exec(`CREATE EXTENSION IF NOT EXISTS vector`)
	_ = bs.Close()
	require.NoError(t, err)
	g, err := gorm.Open(pgdriver.Open(swap(name)), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.MigrateUp(g))
	t.Cleanup(func() {
		if raw, err := g.DB(); err == nil {
			_ = raw.Close()
		}
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer dropCancel()
		_, _ = admin.ExecContext(dropCtx, fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, name))
		_ = admin.Close()
	})
	return g
}

type recordingGrader struct {
	mu    sync.Mutex
	calls []string
}

func (g *recordingGrader) Grade(_ context.Context, assignmentID, userID, graderID, accountID uint, grade string) (*models.Submission, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls = append(g.calls, fmt.Sprintf("a=%d u=%d g=%d acct=%d grade=%s", assignmentID, userID, graderID, accountID, grade))
	return &models.Submission{AssignmentID: assignmentID, UserID: userID}, nil
}

func (g *recordingGrader) last() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.calls) == 0 {
		return ""
	}
	return g.calls[len(g.calls)-1]
}

type recordingSubmitter struct {
	mu   sync.Mutex
	subs []models.Submission
}

func (s *recordingSubmitter) Submit(_ context.Context, sub *models.Submission, courseID, accountID uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subs = append(s.subs, *sub)
	return nil
}

type ltiLane struct {
	t         *testing.T
	g         *gorm.DB
	svc       *service.LTIService
	grader    *recordingGrader
	submitter *recordingSubmitter
	app       *fiber.App
	toolKey   *rsa.PrivateKey

	admin, teacher, student, outsider *models.User
	course, otherCourse               *models.Course
	module                            *models.ContextModule
}

func identitySeal(b []byte) ([]byte, error) { return append([]byte{}, b...), nil }

// ltiCaller stands in for the session middleware: X-User / X-Account set
// the caller, X-Enrollment the enrollment type the course guard would set.
func ltiCaller(c *fiber.Ctx) error {
	if u := c.Get("X-User"); u != "" {
		n, _ := strconv.Atoi(u)
		c.Locals("user_id", uint(n))
	}
	if a := c.Get("X-Account"); a != "" {
		n, _ := strconv.Atoi(a)
		c.Locals("account_id", uint(n))
	}
	if e := c.Get("X-Enrollment"); e != "" {
		c.Locals("enrollment_type", e)
	}
	if c.Get("X-Admin") == "1" {
		c.Locals("is_admin", true)
	}
	return c.Next()
}

func pass(c *fiber.Ctx) error { return c.Next() }

func newLTILane(t *testing.T) *ltiLane {
	g := ltiLaneDB(t)
	ctx := context.Background()
	for _, id := range []uint{1, 2} {
		require.NoError(t, g.Exec(
			`INSERT INTO accounts (id, name, workflow_state, mfa_policy, default_locale, tenant_mode, max_upload_size_mb)
			 VALUES (?, ?, 'active', 'off', 'en', 'higher_ed', 500) ON CONFLICT (id) DO NOTHING`, id, fmt.Sprintf("District %d", id)).Error)
	}
	l := &ltiLane{t: t, g: g, grader: &recordingGrader{}, submitter: &recordingSubmitter{}}
	mkUser := func(acct uint, login, name, role string) *models.User {
		u := &models.User{AccountID: acct, Name: name, LoginID: login, Email: login + "@example.com", PasswordHash: "x", Role: role}
		require.NoError(t, g.Create(u).Error)
		return u
	}
	l.admin = mkUser(1, "admin1", "Ada Admin", models.RoleAdmin)
	l.teacher = mkUser(1, "teacher1", "Tom Teacher", "user")
	l.student = mkUser(1, "student1", "Jane Doe", "user")
	l.outsider = mkUser(2, "other2", "Olga Other", "user")
	courseRepo := postgres.NewCourseRepository(g)
	l.course = &models.Course{AccountID: 1, Name: "Biology", CourseCode: "BIO-1", WorkflowState: models.CourseAvailable}
	require.NoError(t, courseRepo.Create(ctx, l.course))
	l.otherCourse = &models.Course{AccountID: 2, Name: "Chemistry", CourseCode: "CHEM-1", WorkflowState: models.CourseAvailable}
	require.NoError(t, courseRepo.Create(ctx, l.otherCourse))
	for _, e := range []*models.Enrollment{
		{UserID: l.teacher.ID, CourseID: l.course.ID, Type: "TeacherEnrollment", Role: "TeacherEnrollment", WorkflowState: "active"},
		{UserID: l.student.ID, CourseID: l.course.ID, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: "active"},
		{UserID: l.outsider.ID, CourseID: l.otherCourse.ID, Type: "TeacherEnrollment", Role: "TeacherEnrollment", WorkflowState: "active"},
	} {
		require.NoError(t, g.Create(e).Error)
	}
	l.module = &models.ContextModule{CourseID: l.course.ID, Name: "Week 1", WorkflowState: "active"}
	require.NoError(t, postgres.NewModuleRepository(g).Create(ctx, l.module))

	devKeyRepo := postgres.NewDeveloperKeyRepository(g)
	configRepo := postgres.NewLTIToolConfigurationRepository(g)
	enrollRepo := postgres.NewEnrollmentRepository(g)
	toolRepo := postgres.NewContextExternalToolRepository(g)
	userRepo := postgres.NewUserRepository(g)
	assignmentRepo := postgres.NewAssignmentRepository(g)
	lineItemRepo := postgres.NewLTILineItemRepository(g)
	resultRepo := postgres.NewLTIResultRepository(g)
	store := postgres.NewLTIAdvantageRepository(g)

	svc, err := service.NewLTIService(devKeyRepo, configRepo, postgres.NewNonceRepository(g), enrollRepo, courseRepo, toolRepo, ltiIssuer)
	require.NoError(t, err)
	require.NoError(t, svc.LoadPersistentPlatformKey(ctx, postgres.NewLTIPlatformKeyRepository(g), "", identitySeal, identitySeal))
	svc.SetAdvantageDeps(service.LTIAdvantageDeps{
		Store: store, UserRepo: userRepo, AssignmentRepo: assignmentRepo, LineItemRepo: lineItemRepo, ResultRepo: resultRepo,
		ModuleRepo: postgres.NewModuleRepository(g), ModuleItemRepo: postgres.NewModuleItemRepository(g),
		OpenSecret: func(s string) (string, error) { return s, nil }, Grader: l.grader, Submitter: l.submitter,
	})
	devKeyService := service.NewDeveloperKeyService(devKeyRepo)
	svc.SetDeveloperKeyCreator(devKeyService)
	ags := service.NewLTIAGSService(lineItemRepo, resultRepo, assignmentRepo, enrollRepo, l.grader)
	ags.SetAdvantageStore(store)
	nrps := service.NewLTINRPSService(enrollRepo, userRepo)
	nrps.SetCourseRepo(courseRepo)
	l.svc = svc

	h := handlers.NewLTIHandler(svc, ags, nrps, toolRepo, configRepo, userRepo, nil, nil)
	oauth := handlers.NewOAuth2Handler(nil, devKeyService, nil, nil)
	oauth.SetLTIService(svc)
	toolHandler := handlers.NewExternalToolHandler(service.NewExternalToolService(toolRepo, devKeyRepo), devKeyService)

	app := testutil.SetupTestApp()
	app.Use(middleware.PaginationParams())
	// tool-facing
	app.Post("/login/oauth2/token", oauth.Token)
	app.Get("/lti/security/openid-configuration", h.LTIOpenIDConfiguration)
	app.Post("/lti/registrations", h.LTIDynamicRegister)
	app.Post("/lti/deep_linking_response", h.LTIDeepLinkingResponse)
	app.Post("/lti/v1p1/tools/:tool_id/outcomes", h.LTI11Outcomes)
	tool := h.ToolOrSessionAuth(ltiCaller)
	app.Get("/lti/courses/:course_id/line_items", tool, h.ToolScopeOrGuard(pass, service.LTIScopeLineItem, service.LTIScopeLineItemReadOnly), h.ListLineItems)
	app.Post("/lti/courses/:course_id/line_items", tool, h.ToolScopeOrGuard(pass, service.LTIScopeLineItem), h.CreateLineItem)
	app.Post("/lti/courses/:course_id/line_items/:id/scores", tool, h.ToolScopeOrGuard(pass, service.LTIScopeScore), h.PostScore)
	app.Get("/lti/courses/:course_id/line_items/:id/results", tool, h.ToolScopeOrGuard(pass, service.LTIScopeResultReadOnly), h.GetResults)
	app.Get("/lti/courses/:course_id/names_and_roles", tool, h.ToolScopeOrGuard(pass, service.LTIScopeNRPS), h.GetMemberships)
	// session
	app.Get("/lti/authorize", ltiCaller, h.LaunchDirect)
	app.Get("/accounts/:account_id/lti_registrations", ltiCaller, h.ListLTIRegistrations)
	app.Post("/accounts/:account_id/lti_registrations", ltiCaller, h.CreateLTIRegistration)
	app.Post("/accounts/:account_id/lti_registrations/dynamic_registration_token", ltiCaller, h.LTIDynamicRegistrationToken)
	app.Put("/accounts/:account_id/lti_registrations/:id", ltiCaller, h.UpdateLTIRegistration)
	app.Put("/accounts/:account_id/lti_registrations/:id/account_install", ltiCaller, h.SetLTIAccountInstall)
	app.Get("/accounts/:account_id/lti_platform", ltiCaller, h.LTIPlatformInfo)
	app.Post("/lti/platform_keys/rotate", ltiCaller, h.RotateLTIPlatformKey)
	app.Get("/courses/:course_id/external_tools", ltiCaller, toolHandler.ListExternalTools)
	app.Get("/courses/:course_id/lti/tools", ltiCaller, h.ListCourseLTITools)
	app.Get("/courses/:course_id/external_tools/:id/launch", ltiCaller, h.LaunchCourseTool)
	app.Get("/courses/:course_id/assignments/:assignment_id/external_tool", ltiCaller, h.GetAssignmentExternalTool)
	app.Put("/courses/:course_id/assignments/:assignment_id/external_tool", ltiCaller, h.PutAssignmentExternalTool)
	app.Delete("/courses/:course_id/assignments/:assignment_id/external_tool", ltiCaller, h.DeleteAssignmentExternalTool)
	app.Post("/courses/:course_id/modules/:module_id/external_tool_items", ltiCaller, h.CreateModuleExternalToolItem)
	l.app = app

	l.toolKey, err = rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return l
}

func (l *ltiLane) toolJWK() map[string]interface{} {
	return map[string]interface{}{
		"kty": "RSA", "kid": "tool-kid", "use": "sig", "alg": "RS256",
		"n": base64.RawURLEncoding.EncodeToString(l.toolKey.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(l.toolKey.E)).Bytes()),
	}
}

func (l *ltiLane) signTool(claims jwt.MapClaims) string {
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = "tool-kid"
	s, err := tok.SignedString(l.toolKey)
	require.NoError(l.t, err)
	return s
}

type ltiReq struct {
	method, path, contentType string
	body                      io.Reader
	user, account             uint
	enrollment, bearer        string
	admin                     bool
	headers                   map[string]string
}

func (l *ltiLane) do(r ltiReq) (*http.Response, string) {
	req := httptest.NewRequest(r.method, r.path, r.body)
	ct := r.contentType
	if ct == "" {
		ct = "application/json"
	}
	req.Header.Set("Content-Type", ct)
	if r.user != 0 {
		req.Header.Set("X-User", strconv.Itoa(int(r.user)))
	}
	if r.account != 0 {
		req.Header.Set("X-Account", strconv.Itoa(int(r.account)))
	}
	if r.enrollment != "" {
		req.Header.Set("X-Enrollment", r.enrollment)
	}
	if r.admin {
		req.Header.Set("X-Admin", "1")
	}
	if r.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+r.bearer)
	}
	for k, v := range r.headers {
		req.Header.Set(k, v)
	}
	resp, err := l.app.Test(req, -1)
	require.NoError(l.t, err)
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

func jsonReader(v interface{}) io.Reader {
	b, _ := json.Marshal(v)
	return strings.NewReader(string(b))
}

// register creates the example LTI 1.3 tool through the admin API.
func (l *ltiLane) register(extra map[string]interface{}) map[string]interface{} {
	cfg := map[string]interface{}{
		"title":               "Example Tool",
		"target_link_uri":     "https://tool.example.com/launch",
		"oidc_initiation_url": "https://tool.example.com/login",
		"privacy_level":       "name_only",
		"public_jwk":          l.toolJWK(),
		"custom_fields": map[string]string{
			"uid": "$Canvas.user.id", "mail": "$Person.email.primary", "course": "$Context.title", "fixed": "plain",
		},
		"placements": []map[string]interface{}{
			{"placement": "course_navigation", "text": "Example Library", "target_link_uri": "https://tool.example.com/nav"},
			{"placement": "editor_button", "message_type": "LtiDeepLinkingRequest"},
			{"placement": "link_selection"},
			{"placement": "assignment_selection"},
			{"placement": "homework_submission"},
		},
		"scopes": service.LTIAdvantageScopes,
	}
	for k, v := range extra {
		cfg[k] = v
	}
	resp, body := l.do(ltiReq{method: "POST", path: "/accounts/self/lti_registrations", user: l.admin.ID, account: 1,
		body: jsonReader(map[string]interface{}{"name": "Example Tool", "tool_configuration": cfg})})
	require.Equal(l.t, http.StatusCreated, resp.StatusCode, body)
	var reg map[string]interface{}
	require.NoError(l.t, json.Unmarshal([]byte(body), &reg))
	return reg
}

var idTokenRe = regexp.MustCompile(`name="id_token" value="([^"]+)"`)

// launch drives a full LTI 1.3 launch: the Paper launch endpoint, then the
// tool's OIDC authentication request to /lti/authorize. Returns the
// verified id_token claims.
func (l *ltiLane) launch(user *models.User, enrollment, clientID, query string) jwt.MapClaims {
	t := l.t
	resp, body := l.do(ltiReq{method: "GET", path: fmt.Sprintf("/courses/%d/external_tools/%s", l.course.ID, query), user: user.ID, account: user.AccountID, enrollment: enrollment})
	require.Equal(t, http.StatusFound, resp.StatusCode, body)
	loc, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "tool.example.com", loc.Host)
	lq := loc.Query()
	require.Equal(t, ltiIssuer, lq.Get("iss"))
	require.Equal(t, clientID, lq.Get("client_id"))
	require.Equal(t, strconv.Itoa(int(user.ID)), lq.Get("login_hint"))

	q := url.Values{}
	q.Set("scope", "openid")
	q.Set("response_type", "id_token")
	q.Set("response_mode", "form_post")
	q.Set("prompt", "none")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", "https://tool.example.com/launch")
	q.Set("login_hint", lq.Get("login_hint"))
	q.Set("lti_message_hint", lq.Get("lti_message_hint"))
	q.Set("state", "st-1")
	q.Set("nonce", fmt.Sprintf("n-%d", time.Now().UnixNano()))
	resp, body = l.do(ltiReq{method: "GET", path: "/lti/authorize?" + q.Encode(), user: user.ID, account: user.AccountID})
	require.Equal(t, http.StatusOK, resp.StatusCode, body)
	m := idTokenRe.FindStringSubmatch(body)
	require.Len(t, m, 2)
	claims := jwt.MapClaims{}
	_, err = jwt.ParseWithClaims(m[1], claims, func(tok *jwt.Token) (interface{}, error) {
		key, kErr := l.svc.GetPlatformKeyPair()
		return &key.PublicKey, kErr
	})
	require.NoError(t, err)
	assert.Equal(t, lq.Get("lti_deployment_id"), claims["https://purl.imsglobal.org/spec/lti/claim/deployment_id"])
	return claims
}

func (l *ltiLane) serviceToken(clientID, scope, jti string) (*http.Response, map[string]interface{}) {
	assertion := l.signTool(jwt.MapClaims{
		"iss": clientID, "sub": clientID, "aud": ltiIssuer + "/api/v1/login/oauth2/token",
		"iat": time.Now().Unix(), "exp": time.Now().Add(5 * time.Minute).Unix(), "jti": jti,
	})
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
	form.Set("client_assertion", assertion)
	form.Set("scope", scope)
	resp, body := l.do(ltiReq{method: "POST", path: "/login/oauth2/token", contentType: "application/x-www-form-urlencoded", body: strings.NewReader(form.Encode())})
	out := map[string]interface{}{}
	_ = json.Unmarshal([]byte(body), &out)
	return resp, out
}

func TestLTIAdvantage_EndToEnd(t *testing.T) {
	l := newLTILane(t)
	ctx := context.Background()

	// --- G123: registration -------------------------------------------------
	reg := l.register(nil)
	clientID := reg["client_id"].(string)
	devKeyID := uint(reg["id"].(float64))
	require.NotEmpty(t, clientID)

	t.Run("registration validation and tenancy", func(t *testing.T) {
		resp, _ := l.do(ltiReq{method: "POST", path: "/accounts/self/lti_registrations", user: l.admin.ID, account: 1,
			body: jsonReader(map[string]interface{}{"tool_configuration": map[string]interface{}{
				"title": "No key", "target_link_uri": "https://t.example.com", "oidc_initiation_url": "https://t.example.com/l",
			}})})
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "a tool without a public key is refused")
		resp, _ = l.do(ltiReq{method: "GET", path: "/accounts/2/lti_registrations", user: l.admin.ID, account: 1})
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "another tenant's registrations are a 404")
		resp, body := l.do(ltiReq{method: "GET", path: "/accounts/self/lti_registrations", user: l.admin.ID, account: 1})
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, body, clientID)
	})

	// --- G132: account-wide deployment --------------------------------------
	resp, body := l.do(ltiReq{method: "PUT", path: fmt.Sprintf("/accounts/self/lti_registrations/%d/account_install", devKeyID), user: l.admin.ID, account: 1,
		body: jsonReader(map[string]bool{"enabled": true})})
	require.Equal(t, http.StatusOK, resp.StatusCode, body)
	var installed map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(body), &installed))
	deploymentID := installed["deployment_id"].(string)
	require.NotEmpty(t, deploymentID)
	toolID := deploymentID

	t.Run("placements list inherits the account install", func(t *testing.T) {
		resp, body := l.do(ltiReq{method: "GET", path: fmt.Sprintf("/courses/%d/lti/tools?placement=course_navigation", l.course.ID), user: l.student.ID, account: 1, enrollment: "StudentEnrollment"})
		require.Equal(t, http.StatusOK, resp.StatusCode, body)
		assert.Contains(t, body, `"text":"Example Library"`)
		resp, body = l.do(ltiReq{method: "GET", path: fmt.Sprintf("/courses/%d/lti/tools?placement=editor_button", l.course.ID), user: l.student.ID, account: 1, enrollment: "StudentEnrollment"})
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "[]", strings.TrimSpace(body), "selection placements are staff-only")
		resp, body = l.do(ltiReq{method: "GET", path: fmt.Sprintf("/courses/%d/lti/tools?placement=editor_button", l.course.ID), user: l.teacher.ID, account: 1, enrollment: "TeacherEnrollment"})
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, body, `"message_type":"LtiDeepLinkingRequest"`)
		resp, body = l.do(ltiReq{method: "GET", path: fmt.Sprintf("/courses/%d/external_tools?include_parents=true", l.course.ID), user: l.teacher.ID, account: 1, enrollment: "TeacherEnrollment"})
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, body, `"context_type":"Account"`)
	})

	// --- G124 / G125: launch and id_token claims -------------------------------
	var resourceLinkID string
	t.Run("course_navigation launch carries the full claim set", func(t *testing.T) {
		claims := l.launch(l.student, "StudentEnrollment", clientID, toolID+"/launch?placement=course_navigation")
		assert.Equal(t, "LtiResourceLinkRequest", claims["https://purl.imsglobal.org/spec/lti/claim/message_type"])
		assert.Equal(t, deploymentID, claims["https://purl.imsglobal.org/spec/lti/claim/deployment_id"])
		assert.Equal(t, "https://tool.example.com/nav", claims["https://purl.imsglobal.org/spec/lti/claim/target_link_uri"])
		assert.Equal(t, "Jane Doe", claims["name"])
		assert.Equal(t, "Jane", claims["given_name"])
		assert.NotContains(t, claims, "email", "name_only must not send the email")
		custom := claims["https://purl.imsglobal.org/spec/lti/claim/custom"].(map[string]interface{})
		assert.Equal(t, strconv.Itoa(int(l.student.ID)), custom["uid"])
		assert.Equal(t, "$Person.email.primary", custom["mail"], "email variables stay unexpanded below email privacy")
		assert.Equal(t, "Biology", custom["course"])
		assert.Equal(t, "plain", custom["fixed"])
		roles := fmt.Sprint(claims["https://purl.imsglobal.org/spec/lti/claim/roles"])
		assert.Contains(t, roles, "membership#Learner")
		ctxClaim := claims["https://purl.imsglobal.org/spec/lti/claim/context"].(map[string]interface{})
		assert.Equal(t, "BIO-1", ctxClaim["label"])
		ags := claims["https://purl.imsglobal.org/spec/lti-ags/claim/endpoint"].(map[string]interface{})
		assert.Equal(t, fmt.Sprintf("%s/api/v1/lti/courses/%d/line_items", ltiIssuer, l.course.ID), ags["lineitems"])
		nrps := claims["https://purl.imsglobal.org/spec/lti-nrps/claim/namesroleservice"].(map[string]interface{})
		assert.Contains(t, nrps["context_memberships_url"], "/names_and_roles")
		pres := claims["https://purl.imsglobal.org/spec/lti/claim/launch_presentation"].(map[string]interface{})
		assert.Equal(t, "iframe", pres["document_target"])
		rl := claims["https://purl.imsglobal.org/spec/lti/claim/resource_link"].(map[string]interface{})
		resourceLinkID = rl["id"].(string)
		assert.NotEmpty(t, resourceLinkID)

		// The same placement reuses its resource link.
		again := l.launch(l.student, "StudentEnrollment", clientID, toolID+"/launch?placement=course_navigation")
		assert.Equal(t, resourceLinkID, again["https://purl.imsglobal.org/spec/lti/claim/resource_link"].(map[string]interface{})["id"])
	})

	t.Run("launch refusals", func(t *testing.T) {
		resp, _ := l.do(ltiReq{method: "GET", path: fmt.Sprintf("/courses/%d/external_tools/%s/launch?placement=course_navigation", l.otherCourse.ID, toolID), user: l.outsider.ID, account: 2, enrollment: "TeacherEnrollment"})
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "another tenant's course cannot launch the tool")
		resp, _ = l.do(ltiReq{method: "GET", path: fmt.Sprintf("/courses/%d/external_tools/%s/launch?placement=editor_button&deep_link=editor", l.course.ID, toolID), user: l.student.ID, account: 1, enrollment: "StudentEnrollment"})
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "students cannot start content selection")

		// A hint minted for the student cannot be redeemed by the teacher.
		resp, _ = l.do(ltiReq{method: "GET", path: fmt.Sprintf("/courses/%d/external_tools/%s/launch?placement=course_navigation", l.course.ID, toolID), user: l.student.ID, account: 1, enrollment: "StudentEnrollment"})
		loc, _ := url.Parse(resp.Header.Get("Location"))
		q := url.Values{"scope": {"openid"}, "response_type": {"id_token"}, "client_id": {clientID}, "redirect_uri": {"https://tool.example.com/launch"},
			"nonce": {"n-x"}, "lti_message_hint": {loc.Query().Get("lti_message_hint")}}
		resp, _ = l.do(ltiReq{method: "GET", path: "/lti/authorize?" + q.Encode(), user: l.teacher.ID, account: 1})
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	// --- G127: client_credentials ----------------------------------------------
	resp, tok := l.serviceToken(clientID, strings.Join(service.LTIAdvantageScopes, " "), "jti-1")
	require.Equal(t, http.StatusOK, resp.StatusCode, tok)
	accessToken := tok["access_token"].(string)
	t.Run("client_credentials grant", func(t *testing.T) {
		assert.Equal(t, "Bearer", tok["token_type"])
		resp, out := l.serviceToken(clientID, service.LTIScopeScore, "jti-1")
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.Equal(t, "invalid_grant", out["error"], "a client_assertion jti is single use")
		bad := l.signTool(jwt.MapClaims{"iss": clientID, "sub": clientID, "aud": "https://elsewhere.example.com", "exp": time.Now().Add(time.Minute).Unix(), "jti": "jti-2"})
		form := url.Values{"grant_type": {"client_credentials"}, "client_assertion_type": {"urn:ietf:params:oauth:client-assertion-type:jwt-bearer"}, "client_assertion": {bad}, "scope": {service.LTIScopeScore}}
		resp, _ = l.do(ltiReq{method: "POST", path: "/login/oauth2/token", contentType: "application/x-www-form-urlencoded", body: strings.NewReader(form.Encode())})
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	// --- G128: AGS as a tool -----------------------------------------------------
	var toolLineItemURL string
	t.Run("AGS line items and scores with a service token", func(t *testing.T) {
		// A staff line item is invisible to the tool.
		staffItem := &models.LTILineItem{CourseID: l.course.ID, Label: "Staff", ScoreMaximum: 10}
		require.NoError(t, postgres.NewLTILineItemRepository(l.g).Create(ctx, staffItem))

		resp, body := l.do(ltiReq{method: "POST", path: fmt.Sprintf("/lti/courses/%d/line_items", l.course.ID), bearer: accessToken,
			contentType: "application/vnd.ims.lis.v2.lineitem+json", body: jsonReader(map[string]interface{}{"label": "Unit Quiz", "scoreMaximum": 20, "tag": "quiz"})})
		require.Equal(t, http.StatusCreated, resp.StatusCode, body)
		var item map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(body), &item))
		toolLineItemURL = item["id"].(string)
		aid := uint(item["assignmentId"].(float64))
		var a models.Assignment
		require.NoError(t, l.g.First(&a, aid).Error)
		assert.Equal(t, "Unit Quiz", a.Name)
		assert.Equal(t, 20.0, *a.PointsPossible)
		assert.True(t, a.IsPublished())

		resp, body = l.do(ltiReq{method: "GET", path: fmt.Sprintf("/lti/courses/%d/line_items?tag=quiz", l.course.ID), bearer: accessToken})
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, body, "Unit Quiz")
		assert.NotContains(t, body, `"Staff"`)

		itemPath := strings.TrimPrefix(toolLineItemURL, "http://example.com")
		now := time.Now().UTC()
		resp, body = l.do(ltiReq{method: "POST", path: itemPath + "/scores", bearer: accessToken, contentType: "application/vnd.ims.lis.v1.score+json",
			body: jsonReader(map[string]interface{}{"userId": strconv.Itoa(int(l.student.ID)), "scoreGiven": 15, "scoreMaximum": 20,
				"activityProgress": "Completed", "gradingProgress": "FullyGraded", "timestamp": now.Format(time.RFC3339)})})
		require.Equal(t, http.StatusNoContent, resp.StatusCode, body)
		assert.Equal(t, fmt.Sprintf("a=%d u=%d g=0 acct=1 grade=15", aid, l.student.ID), l.grader.last())

		resp, _ = l.do(ltiReq{method: "POST", path: itemPath + "/scores", bearer: accessToken, contentType: "application/vnd.ims.lis.v1.score+json",
			body: jsonReader(map[string]interface{}{"userId": strconv.Itoa(int(l.student.ID)), "scoreGiven": 1, "scoreMaximum": 20,
				"activityProgress": "Completed", "gradingProgress": "FullyGraded", "timestamp": now.Add(-time.Hour).Format(time.RFC3339)})})
		assert.Equal(t, http.StatusConflict, resp.StatusCode, "an older score is refused")

		resp, body = l.do(ltiReq{method: "GET", path: itemPath + "/results", bearer: accessToken})
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, body, `"scoreOf":"`+toolLineItemURL+`"`)

		// A tool cannot hang a line item on (and grade) an assignment that
		// does not launch it.
		other := &models.Assignment{CourseID: l.course.ID, Name: "Teacher quiz", WorkflowState: models.AssignmentPublished, Published: true}
		require.NoError(t, postgres.NewAssignmentRepository(l.g).Create(ctx, other))
		resp, _ = l.do(ltiReq{method: "POST", path: fmt.Sprintf("/lti/courses/%d/line_items", l.course.ID), bearer: accessToken,
			body: jsonReader(map[string]interface{}{"label": "Steal", "scoreMaximum": 5, "assignmentId": other.ID})})
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		// Scope and install checks.
		_, ro := l.serviceToken(clientID, service.LTIScopeLineItemReadOnly, "jti-ro")
		resp, _ = l.do(ltiReq{method: "POST", path: itemPath + "/scores", bearer: ro["access_token"].(string), contentType: "application/vnd.ims.lis.v1.score+json",
			body: jsonReader(map[string]interface{}{"userId": "1", "activityProgress": "Completed", "gradingProgress": "FullyGraded"})})
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "the score scope is required")
		resp, _ = l.do(ltiReq{method: "GET", path: fmt.Sprintf("/lti/courses/%d/line_items", l.otherCourse.ID), bearer: accessToken})
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "a tool cannot reach a course it is not installed in")
		resp, _ = l.do(ltiReq{method: "GET", path: fmt.Sprintf("/lti/courses/%d/line_items", l.course.ID), bearer: accessToken + "x"})
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	// --- G129: NRPS as a tool -------------------------------------------------------
	t.Run("NRPS roster with privacy gating and role filter", func(t *testing.T) {
		resp, body := l.do(ltiReq{method: "GET", path: fmt.Sprintf("/lti/courses/%d/names_and_roles?role=Learner", l.course.ID), bearer: accessToken})
		require.Equal(t, http.StatusOK, resp.StatusCode, body)
		var out map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(body), &out))
		assert.Equal(t, "Biology", out["context"].(map[string]interface{})["title"])
		members := out["members"].([]interface{})
		require.Len(t, members, 1)
		m := members[0].(map[string]interface{})
		assert.Equal(t, "Jane Doe", m["name"])
		assert.NotContains(t, m, "email", "name_only tools do not get emails")
	})

	// --- G130: deep linking into a module ------------------------------------------------
	t.Run("deep linking creates module items", func(t *testing.T) {
		claims := l.launch(l.teacher, "TeacherEnrollment", clientID, fmt.Sprintf("%s/launch?placement=link_selection&deep_link=module&module_id=%d", toolID, l.module.ID))
		assert.Equal(t, "LtiDeepLinkingRequest", claims["https://purl.imsglobal.org/spec/lti/claim/message_type"])
		settings := claims["https://purl.imsglobal.org/spec/lti-dl/claim/deep_linking_settings"].(map[string]interface{})
		assert.Equal(t, ltiIssuer+"/api/v1/lti/deep_linking_response", settings["deep_link_return_url"])
		respJWT := l.signTool(jwt.MapClaims{
			"iss": clientID, "aud": ltiIssuer, "iat": time.Now().Unix(), "exp": time.Now().Add(5 * time.Minute).Unix(), "nonce": "dl-nonce-1",
			"https://purl.imsglobal.org/spec/lti/claim/message_type":  "LtiDeepLinkingResponse",
			"https://purl.imsglobal.org/spec/lti/claim/version":       "1.3.0",
			"https://purl.imsglobal.org/spec/lti/claim/deployment_id": deploymentID,
			"https://purl.imsglobal.org/spec/lti-dl/claim/data":       settings["data"],
			"https://purl.imsglobal.org/spec/lti-dl/claim/content_items": []map[string]interface{}{
				{"type": "ltiResourceLink", "title": "Chapter 1 <b>quiz</b>", "url": "https://tool.example.com/ch1", "custom": map[string]interface{}{"chapter": 1}},
				{"type": "link", "title": "Reading", "url": "https://reading.example.com/r1"},
				{"type": "html", "html": "<p>ignored for modules</p>"},
			},
		})
		form := url.Values{"JWT": {respJWT}}
		resp, body := l.do(ltiReq{method: "POST", path: "/lti/deep_linking_response", contentType: "application/x-www-form-urlencoded", body: strings.NewReader(form.Encode())})
		require.Equal(t, http.StatusOK, resp.StatusCode, body)
		assert.Contains(t, body, "LtiDeepLinkingResponse")
		assert.Contains(t, resp.Header.Get("Content-Security-Policy"), "script-src 'nonce-")

		var tags []models.ContentTag
		require.NoError(t, l.g.Where("context_module_id = ?", l.module.ID).Order("position").Find(&tags).Error)
		require.Len(t, tags, 2)
		assert.Equal(t, service.ContentTypeExternalTool, tags[0].ContentType)
		assert.Equal(t, "Chapter 1 quiz", tags[0].Title, "titles are plain text")
		assert.Equal(t, "ExternalUrl", tags[1].ContentType)
		var link models.LTIResourceLink
		require.NoError(t, l.g.Where("content_tag_id = ?", tags[0].ID).First(&link).Error)
		assert.JSONEq(t, `{"chapter":"1"}`, link.CustomParameters)

		resp, _ = l.do(ltiReq{method: "POST", path: "/lti/deep_linking_response", contentType: "application/x-www-form-urlencoded", body: strings.NewReader(form.Encode())})
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "a deep linking response is single use")

		// The module item launches with its resource link once published.
		resp, _ = l.do(ltiReq{method: "GET", path: fmt.Sprintf("/courses/%d/external_tools/%s/launch?module_item_id=%d", l.course.ID, toolID, tags[0].ID), user: l.student.ID, account: 1, enrollment: "StudentEnrollment"})
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "unpublished items do not launch for students")
		require.NoError(t, l.g.Model(&models.ContentTag{}).Where("id = ?", tags[0].ID).Update("workflow_state", "active").Error)
		claims = l.launch(l.student, "StudentEnrollment", clientID, fmt.Sprintf("%s/launch?module_item_id=%d", toolID, tags[0].ID))
		assert.Equal(t, link.ResourceLinkID, claims["https://purl.imsglobal.org/spec/lti/claim/resource_link"].(map[string]interface{})["id"])
		assert.Equal(t, "https://tool.example.com/ch1", claims["https://purl.imsglobal.org/spec/lti/claim/target_link_uri"])
		assert.Equal(t, "1", claims["https://purl.imsglobal.org/spec/lti/claim/custom"].(map[string]interface{})["chapter"])
	})

	// --- G131: assignment tool + module tool item ----------------------------------------
	t.Run("assignment external tool gets a line item and an AGS lineitem claim", func(t *testing.T) {
		a := &models.Assignment{CourseID: l.course.ID, Name: "Lab", WorkflowState: models.AssignmentPublished, Published: true}
		pts := 50.0
		a.PointsPossible = &pts
		require.NoError(t, postgres.NewAssignmentRepository(l.g).Create(ctx, a))
		resp, body := l.do(ltiReq{method: "PUT", path: fmt.Sprintf("/courses/%d/assignments/%d/external_tool", l.course.ID, a.ID), user: l.teacher.ID, account: 1, enrollment: "TeacherEnrollment",
			body: jsonReader(map[string]interface{}{"tool_id": toolID, "url": "https://tool.example.com/lab"})})
		require.Equal(t, http.StatusOK, resp.StatusCode, body)
		var fresh models.Assignment
		require.NoError(t, l.g.First(&fresh, a.ID).Error)
		assert.Equal(t, "external_tool", fresh.SubmissionTypes)
		li, err := postgres.NewLTILineItemRepository(l.g).FindByAssignmentID(ctx, a.ID)
		require.NoError(t, err)
		require.NotNil(t, li.DeveloperKeyID)
		assert.Equal(t, devKeyID, *li.DeveloperKeyID)
		assert.Equal(t, 50.0, li.ScoreMaximum)

		resp, body = l.do(ltiReq{method: "GET", path: fmt.Sprintf("/courses/%d/assignments/%d/external_tool", l.course.ID, a.ID), user: l.student.ID, account: 1, enrollment: "StudentEnrollment"})
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, body, `"url":""`, "students do not see the launch URL")

		claims := l.launch(l.student, "StudentEnrollment", clientID, fmt.Sprintf("%s/launch?assignment_id=%d", toolID, a.ID))
		ags := claims["https://purl.imsglobal.org/spec/lti-ags/claim/endpoint"].(map[string]interface{})
		assert.Equal(t, fmt.Sprintf("%s/api/v1/lti/courses/%d/line_items/%d", ltiIssuer, l.course.ID, li.ID), ags["lineitem"])
		assert.Equal(t, "https://tool.example.com/lab", claims["https://purl.imsglobal.org/spec/lti/claim/target_link_uri"])

		resp, body = l.do(ltiReq{method: "POST", path: fmt.Sprintf("/courses/%d/modules/%d/external_tool_items", l.course.ID, l.module.ID), user: l.teacher.ID, account: 1, enrollment: "TeacherEnrollment",
			body: jsonReader(map[string]interface{}{"tool_id": toolID, "title": "Library"})})
		require.Equal(t, http.StatusCreated, resp.StatusCode, body)
		// Module item JSON uses Canvas's item type name (content_type
		// ContextExternalTool is stored on the row).
		assert.Contains(t, body, `"type":"ExternalTool"`)
	})

	t.Run("homework_submission submits the student's selection", func(t *testing.T) {
		a := &models.Assignment{CourseID: l.course.ID, Name: "Essay", SubmissionTypes: "online_url", WorkflowState: models.AssignmentPublished, Published: true}
		require.NoError(t, postgres.NewAssignmentRepository(l.g).Create(ctx, a))
		claims := l.launch(l.student, "StudentEnrollment", clientID, fmt.Sprintf("%s/launch?placement=homework_submission&deep_link=submission&assignment_id=%d", toolID, a.ID))
		settings := claims["https://purl.imsglobal.org/spec/lti-dl/claim/deep_linking_settings"].(map[string]interface{})
		assert.Equal(t, []interface{}{"link", "file"}, settings["accept_types"])
		respJWT := l.signTool(jwt.MapClaims{
			"iss": clientID, "aud": ltiIssuer, "exp": time.Now().Add(5 * time.Minute).Unix(), "nonce": "hw-1",
			"https://purl.imsglobal.org/spec/lti/claim/message_type":  "LtiDeepLinkingResponse",
			"https://purl.imsglobal.org/spec/lti/claim/version":       "1.3.0",
			"https://purl.imsglobal.org/spec/lti/claim/deployment_id": deploymentID,
			"https://purl.imsglobal.org/spec/lti-dl/claim/data":       settings["data"],
			"https://purl.imsglobal.org/spec/lti-dl/claim/content_items": []map[string]interface{}{
				{"type": "link", "url": "https://tool.example.com/work/42", "title": "My work"},
			},
		})
		resp, body := l.do(ltiReq{method: "POST", path: "/lti/deep_linking_response", contentType: "application/x-www-form-urlencoded",
			body: strings.NewReader(url.Values{"JWT": {respJWT}}.Encode())})
		require.Equal(t, http.StatusOK, resp.StatusCode, body)
		require.Len(t, l.submitter.subs, 1)
		sub := l.submitter.subs[0]
		assert.Equal(t, a.ID, sub.AssignmentID)
		assert.Equal(t, l.student.ID, sub.UserID)
		assert.Equal(t, "online_url", *sub.SubmissionType)
		assert.Equal(t, "https://tool.example.com/work/42", *sub.URL)
	})

	// --- G133: LTI 1.1 ---------------------------------------------------------------------
	t.Run("LTI 1.1 launch and Basic Outcomes", func(t *testing.T) {
		lti11 := &models.ContextExternalTool{ContextType: "Course", ContextID: l.course.ID, Name: "Legacy", URL: "https://legacy.example.com/lti",
			ConsumerKey: "legacy-key", SharedSecret: "legacy-secret", WorkflowState: "active", PrivacyLevel: "public"}
		require.NoError(t, postgres.NewContextExternalToolRepository(l.g).Create(ctx, lti11))
		a := &models.Assignment{CourseID: l.course.ID, Name: "Legacy Quiz", WorkflowState: models.AssignmentPublished, Published: true}
		pts := 10.0
		a.PointsPossible = &pts
		require.NoError(t, postgres.NewAssignmentRepository(l.g).Create(ctx, a))
		resp, body := l.do(ltiReq{method: "PUT", path: fmt.Sprintf("/courses/%d/assignments/%d/external_tool", l.course.ID, a.ID), user: l.teacher.ID, account: 1, enrollment: "TeacherEnrollment",
			body: jsonReader(map[string]interface{}{"tool_id": lti11.ID})})
		require.Equal(t, http.StatusOK, resp.StatusCode, body)

		resp, body = l.do(ltiReq{method: "GET", path: fmt.Sprintf("/courses/%d/external_tools/%d/launch?assignment_id=%d", l.course.ID, lti11.ID, a.ID), user: l.student.ID, account: 1, enrollment: "StudentEnrollment"})
		require.Equal(t, http.StatusOK, resp.StatusCode, body)
		params := map[string]string{}
		for _, m := range regexp.MustCompile(`name="([^"]+)" value="([^"]*)"`).FindAllStringSubmatch(body, -1) {
			params[m[1]] = htmlUnescape(m[2])
		}
		assert.Equal(t, "basic-lti-launch-request", params["lti_message_type"])
		assert.Equal(t, "Jane Doe", params["lis_person_name_full"])
		assert.Equal(t, "Learner", params["roles"])
		want, err := service.OAuth1Signature("POST", "https://legacy.example.com/lti", params, "legacy-secret")
		require.NoError(t, err)
		assert.Equal(t, want, params["oauth_signature"], "the launch is OAuth1 signed with the shared secret")
		sourcedID := params["lis_result_sourcedid"]
		require.NotEmpty(t, sourcedID)
		outcomesURL := params["lis_outcome_service_url"]
		require.Equal(t, fmt.Sprintf("%s/api/v1/lti/v1p1/tools/%d/outcomes", ltiIssuer, lti11.ID), outcomesURL)

		pox := `<?xml version="1.0" encoding="UTF-8"?>
<imsx_POXEnvelopeRequest xmlns="http://www.imsglobal.org/services/ltiv1p1/xsd/imsoms_v1p0">
  <imsx_POXHeader><imsx_POXRequestHeaderInfo><imsx_version>V1.0</imsx_version><imsx_messageIdentifier>m1</imsx_messageIdentifier></imsx_POXRequestHeaderInfo></imsx_POXHeader>
  <imsx_POXBody><replaceResultRequest><resultRecord><sourcedGUID><sourcedId>` + sourcedID + `</sourcedId></sourcedGUID>
  <result><resultScore><language>en</language><textString>0.8</textString></resultScore></result></resultRecord></replaceResultRequest></imsx_POXBody>
</imsx_POXEnvelopeRequest>`
		post := func(secret, nonce, sid string) (*http.Response, string) {
			bodyStr := strings.ReplaceAll(pox, sourcedID, sid)
			sum := sha1.Sum([]byte(bodyStr)) //nolint:gosec
			op := map[string]string{
				"oauth_consumer_key": "legacy-key", "oauth_signature_method": "HMAC-SHA1", "oauth_timestamp": strconv.FormatInt(time.Now().Unix(), 10),
				"oauth_nonce": nonce, "oauth_version": "1.0", "oauth_body_hash": base64.StdEncoding.EncodeToString(sum[:]),
			}
			sig, _ := service.OAuth1Signature("POST", outcomesURL, op, secret)
			op["oauth_signature"] = sig
			var parts []string
			for k, v := range op {
				parts = append(parts, fmt.Sprintf(`%s="%s"`, k, url.QueryEscape(v)))
			}
			return l.do(ltiReq{method: "POST", path: fmt.Sprintf("/lti/v1p1/tools/%d/outcomes", lti11.ID), contentType: "application/xml",
				body: strings.NewReader(bodyStr), headers: map[string]string{"Authorization": "OAuth " + strings.Join(parts, ", ")}})
		}
		resp, body = post("legacy-secret", "nonce-1", sourcedID)
		require.Equal(t, http.StatusOK, resp.StatusCode, body)
		assert.Contains(t, body, "<imsx_codeMajor>success</imsx_codeMajor>")
		assert.Equal(t, fmt.Sprintf("a=%d u=%d g=0 acct=1 grade=8", a.ID, l.student.ID), l.grader.last())

		resp, _ = post("wrong-secret", "nonce-2", sourcedID)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		resp, body = post("legacy-secret", "nonce-3", strings.Replace(sourcedID, ":"+strconv.Itoa(int(l.student.ID))+":", ":"+strconv.Itoa(int(l.teacher.ID))+":", 1))
		assert.Contains(t, body, "<imsx_codeMajor>failure</imsx_codeMajor>", "a forged sourcedid is refused")
		resp, _ = post("legacy-secret", "nonce-1", sourcedID)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "OAuth nonces are single use")
	})

	// --- G134: dynamic registration ------------------------------------------------------
	t.Run("dynamic registration", func(t *testing.T) {
		resp, body := l.do(ltiReq{method: "GET", path: "/lti/security/openid-configuration"})
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, body, `"registration_endpoint":"`+ltiIssuer+`/api/v1/lti/registrations"`)
		resp, body = l.do(ltiReq{method: "POST", path: "/accounts/self/lti_registrations/dynamic_registration_token", user: l.admin.ID, account: 1})
		require.Equal(t, http.StatusOK, resp.StatusCode, body)
		var tok map[string]string
		require.NoError(t, json.Unmarshal([]byte(body), &tok))
		regBody := map[string]interface{}{
			"application_type": "web", "response_types": []string{"id_token"}, "grant_types": []string{"implicit", "client_credentials"},
			"initiate_login_uri": "https://dyn.example.com/login", "redirect_uris": []string{"https://dyn.example.com/launch"},
			"client_name": "Dynamic Tool", "jwks_uri": "https://dyn.example.com/jwks", "token_endpoint_auth_method": "private_key_jwt",
			"scope": service.LTIScopeScore + " " + service.LTIScopeNRPS,
			"https://purl.imsglobal.org/spec/lti-tool-configuration": map[string]interface{}{
				"domain": "dyn.example.com", "target_link_uri": "https://dyn.example.com/launch", "claims": []string{"sub", "name", "email"},
				"messages": []map[string]interface{}{{"type": "LtiDeepLinkingRequest", "placements": []string{"editor_button"}, "label": "Dyn"}},
			},
		}
		resp, body = l.do(ltiReq{method: "POST", path: "/lti/registrations", bearer: tok["registration_token"], body: jsonReader(regBody)})
		require.Equal(t, http.StatusOK, resp.StatusCode, body)
		var out map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(body), &out))
		newClient := out["client_id"].(string)
		require.NotEmpty(t, newClient)
		assert.NotEmpty(t, out["https://purl.imsglobal.org/spec/lti-tool-configuration"].(map[string]interface{})["deployment_id"])
		var cfg models.LTIToolConfiguration
		require.NoError(t, l.g.Joins("JOIN developer_keys ON developer_keys.id = lti_tool_configurations.developer_key_id").
			Where("developer_keys.client_id = ? AND developer_keys.account_id = 1", newClient).First(&cfg).Error)
		assert.Equal(t, "public", cfg.PrivacyLevel)
		assert.Contains(t, cfg.Placements, "editor_button")

		resp, _ = l.do(ltiReq{method: "POST", path: "/lti/registrations", bearer: tok["registration_token"], body: jsonReader(regBody)})
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "a registration token is single use")
	})

	// --- G126: key rotation ---------------------------------------------------------------
	t.Run("platform key rotation keeps old tokens verifiable", func(t *testing.T) {
		before := l.svc.GetPlatformKeyStatus()
		require.True(t, before.Rotatable)
		require.Len(t, before.PublishedKIDs, 2, "the upcoming key is published before any rotation")
		resp, body := l.do(ltiReq{method: "POST", path: "/lti/platform_keys/rotate", user: l.admin.ID, account: 1})
		require.Equal(t, http.StatusOK, resp.StatusCode, body)
		after := l.svc.GetPlatformKeyStatus()
		assert.Equal(t, before.PublishedKIDs[1], after.CurrentKID, "the published next key starts signing")
		assert.Contains(t, after.PublishedKIDs, before.CurrentKID, "the retired key stays published")
		assert.Len(t, after.PublishedKIDs, 3)
		// A service token signed by the retired key still works.
		resp, _ = l.do(ltiReq{method: "GET", path: fmt.Sprintf("/lti/courses/%d/line_items", l.course.ID), bearer: accessToken})
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// --- review repairs ---------------------------------------------------------------------
	t.Run("detaching a tool from an assignment revokes its line items", func(t *testing.T) {
		_, fresh := l.serviceToken(clientID, strings.Join(service.LTIAdvantageScopes, " "), "jti-detach")
		token := fresh["access_token"].(string)
		a := &models.Assignment{CourseID: l.course.ID, Name: "Detached lab", WorkflowState: models.AssignmentPublished, Published: true}
		require.NoError(t, postgres.NewAssignmentRepository(l.g).Create(ctx, a))
		path := fmt.Sprintf("/courses/%d/assignments/%d/external_tool", l.course.ID, a.ID)
		resp, body := l.do(ltiReq{method: "PUT", path: path, user: l.teacher.ID, account: 1, enrollment: "TeacherEnrollment",
			body: jsonReader(map[string]interface{}{"tool_id": toolID})})
		require.Equal(t, http.StatusOK, resp.StatusCode, body)
		li, err := postgres.NewLTILineItemRepository(l.g).FindByAssignmentID(ctx, a.ID)
		require.NoError(t, err)
		require.NotNil(t, li.DeveloperKeyID)
		scorePath := fmt.Sprintf("/lti/courses/%d/line_items/%d/scores", l.course.ID, li.ID)
		score := func() int {
			resp, _ := l.do(ltiReq{method: "POST", path: scorePath, bearer: token, contentType: "application/vnd.ims.lis.v1.score+json",
				body: jsonReader(map[string]interface{}{"userId": strconv.Itoa(int(l.student.ID)), "scoreGiven": 3, "scoreMaximum": 100,
					"activityProgress": "Completed", "gradingProgress": "FullyGraded"})})
			return resp.StatusCode
		}
		require.Equal(t, http.StatusNoContent, score(), "the attached tool grades the assignment")

		resp, body = l.do(ltiReq{method: "DELETE", path: path, user: l.teacher.ID, account: 1, enrollment: "TeacherEnrollment"})
		require.Equal(t, http.StatusNoContent, resp.StatusCode, body)
		li, err = postgres.NewLTILineItemRepository(l.g).FindByAssignmentID(ctx, a.ID)
		require.NoError(t, err)
		assert.Nil(t, li.DeveloperKeyID, "the detached tool no longer owns the line item")
		assert.Equal(t, http.StatusNotFound, score(), "the detached tool can no longer post scores")
		resp, body = l.do(ltiReq{method: "GET", path: fmt.Sprintf("/lti/courses/%d/line_items", l.course.ID), bearer: token})
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotContains(t, body, "Detached lab")
	})

	t.Run("module item launches follow module publication", func(t *testing.T) {
		hidden := &models.ContextModule{CourseID: l.course.ID, Name: "Draft week", WorkflowState: "unpublished"}
		require.NoError(t, postgres.NewModuleRepository(l.g).Create(ctx, hidden))
		resp, body := l.do(ltiReq{method: "POST", path: fmt.Sprintf("/courses/%d/modules/%d/external_tool_items", l.course.ID, hidden.ID), user: l.teacher.ID, account: 1, enrollment: "TeacherEnrollment",
			body: jsonReader(map[string]interface{}{"tool_id": toolID, "title": "Draft tool"})})
		require.Equal(t, http.StatusCreated, resp.StatusCode, body)
		var item map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(body), &item))
		tagID := uint(item["id"].(float64))
		var link models.LTIResourceLink
		require.NoError(t, l.g.Where("content_tag_id = ?", tagID).First(&link).Error)

		launch := func(u *models.User, enrollment, query string) int {
			resp, _ := l.do(ltiReq{method: "GET", path: fmt.Sprintf("/courses/%d/external_tools/%s/launch?%s", l.course.ID, toolID, query), user: u.ID, account: 1, enrollment: enrollment})
			return resp.StatusCode
		}
		byItem := fmt.Sprintf("module_item_id=%d", tagID)
		byLink := "resource_link_id=" + link.ResourceLinkID
		assert.Equal(t, http.StatusNotFound, launch(l.student, "StudentEnrollment", byItem), "items of an unpublished module do not launch for students")
		assert.Equal(t, http.StatusNotFound, launch(l.student, "StudentEnrollment", byLink), "nor through their resource link id")
		assert.Equal(t, http.StatusFound, launch(l.teacher, "TeacherEnrollment", byItem), "staff preview unpublished modules")

		require.NoError(t, l.g.Model(&models.ContextModule{}).Where("id = ?", hidden.ID).Update("workflow_state", "active").Error)
		assert.Equal(t, http.StatusFound, launch(l.student, "StudentEnrollment", byItem))
		assert.Equal(t, http.StatusFound, launch(l.student, "StudentEnrollment", byLink))
	})

	t.Run("deep linking drops a non-https resource link url", func(t *testing.T) {
		claims := l.launch(l.teacher, "TeacherEnrollment", clientID, fmt.Sprintf("%s/launch?placement=link_selection&deep_link=module&module_id=%d", toolID, l.module.ID))
		settings := claims["https://purl.imsglobal.org/spec/lti-dl/claim/deep_linking_settings"].(map[string]interface{})
		respJWT := l.signTool(jwt.MapClaims{
			"iss": clientID, "aud": ltiIssuer, "exp": time.Now().Add(5 * time.Minute).Unix(), "nonce": "dl-js-url",
			"https://purl.imsglobal.org/spec/lti/claim/message_type":  "LtiDeepLinkingResponse",
			"https://purl.imsglobal.org/spec/lti/claim/version":       "1.3.0",
			"https://purl.imsglobal.org/spec/lti/claim/deployment_id": deploymentID,
			"https://purl.imsglobal.org/spec/lti-dl/claim/data":       settings["data"],
			"https://purl.imsglobal.org/spec/lti-dl/claim/content_items": []map[string]interface{}{
				{"type": "ltiResourceLink", "title": "Scripted", "url": "javascript:alert(document.cookie)"},
			},
		})
		resp, body := l.do(ltiReq{method: "POST", path: "/lti/deep_linking_response", contentType: "application/x-www-form-urlencoded",
			body: strings.NewReader(url.Values{"JWT": {respJWT}}.Encode())})
		require.Equal(t, http.StatusOK, resp.StatusCode, body)
		var tag models.ContentTag
		require.NoError(t, l.g.Where("context_module_id = ? AND title = ?", l.module.ID, "Scripted").First(&tag).Error)
		assert.Empty(t, tag.URL)
		var link models.LTIResourceLink
		require.NoError(t, l.g.Where("content_tag_id = ?", tag.ID).First(&link).Error)
		assert.Empty(t, link.URL)
	})

	t.Run("LTI 1.1 user ids and sourcedids survive key rotations", func(t *testing.T) {
		lti11 := &models.ContextExternalTool{ContextType: "Course", ContextID: l.course.ID, Name: "Legacy 2", URL: "https://legacy2.example.com/lti",
			ConsumerKey: "legacy2-key", SharedSecret: "legacy2-secret", WorkflowState: "active", PrivacyLevel: "anonymous"}
		require.NoError(t, postgres.NewContextExternalToolRepository(l.g).Create(ctx, lti11))
		a := &models.Assignment{CourseID: l.course.ID, Name: "Legacy 2 quiz", WorkflowState: models.AssignmentPublished, Published: true}
		pts := 10.0
		a.PointsPossible = &pts
		require.NoError(t, postgres.NewAssignmentRepository(l.g).Create(ctx, a))
		resp, body := l.do(ltiReq{method: "PUT", path: fmt.Sprintf("/courses/%d/assignments/%d/external_tool", l.course.ID, a.ID), user: l.teacher.ID, account: 1, enrollment: "TeacherEnrollment",
			body: jsonReader(map[string]interface{}{"tool_id": lti11.ID})})
		require.Equal(t, http.StatusOK, resp.StatusCode, body)
		launchParams := func() map[string]string {
			resp, body := l.do(ltiReq{method: "GET", path: fmt.Sprintf("/courses/%d/external_tools/%d/launch?assignment_id=%d", l.course.ID, lti11.ID, a.ID), user: l.student.ID, account: 1, enrollment: "StudentEnrollment"})
			require.Equal(t, http.StatusOK, resp.StatusCode, body)
			params := map[string]string{}
			for _, m := range regexp.MustCompile(`name="([^"]+)" value="([^"]*)"`).FindAllStringSubmatch(body, -1) {
				params[m[1]] = htmlUnescape(m[2])
			}
			return params
		}
		before := launchParams()
		require.NotEmpty(t, before["user_id"])
		require.NotEmpty(t, before["lis_result_sourcedid"])

		for i := 0; i < 2; i++ {
			_, err := l.svc.RotatePlatformKey(ctx)
			require.NoError(t, err)
		}
		after := launchParams()
		assert.Equal(t, before["user_id"], after["user_id"], "a key rotation must not re-identify LTI 1.1 users")
		assert.Equal(t, before["lis_result_sourcedid"], after["lis_result_sourcedid"])

		resp, body = lti11Outcome(l, lti11, before["lis_outcome_service_url"], "legacy2-secret", "rot-nonce-1", before["lis_result_sourcedid"], "0.5")
		require.Equal(t, http.StatusOK, resp.StatusCode, body)
		assert.Contains(t, body, "<imsx_codeMajor>success</imsx_codeMajor>", "a sourcedid issued before two rotations still grades")
		assert.Equal(t, fmt.Sprintf("a=%d u=%d g=0 acct=1 grade=5", a.ID, l.student.ID), l.grader.last())
	})
}

// lti11Outcome posts an OAuth1-signed Basic Outcomes replaceResult.
func lti11Outcome(l *ltiLane, tool *models.ContextExternalTool, outcomesURL, secret, nonce, sourcedID, score string) (*http.Response, string) {
	body := `<?xml version="1.0" encoding="UTF-8"?>
<imsx_POXEnvelopeRequest xmlns="http://www.imsglobal.org/services/ltiv1p1/xsd/imsoms_v1p0">
  <imsx_POXHeader><imsx_POXRequestHeaderInfo><imsx_version>V1.0</imsx_version><imsx_messageIdentifier>m-` + nonce + `</imsx_messageIdentifier></imsx_POXRequestHeaderInfo></imsx_POXHeader>
  <imsx_POXBody><replaceResultRequest><resultRecord><sourcedGUID><sourcedId>` + sourcedID + `</sourcedId></sourcedGUID>
  <result><resultScore><language>en</language><textString>` + score + `</textString></resultScore></result></resultRecord></replaceResultRequest></imsx_POXBody>
</imsx_POXEnvelopeRequest>`
	sum := sha1.Sum([]byte(body)) //nolint:gosec // oauth_body_hash is SHA-1 by spec
	op := map[string]string{
		"oauth_consumer_key": tool.ConsumerKey, "oauth_signature_method": "HMAC-SHA1", "oauth_timestamp": strconv.FormatInt(time.Now().Unix(), 10),
		"oauth_nonce": nonce, "oauth_version": "1.0", "oauth_body_hash": base64.StdEncoding.EncodeToString(sum[:]),
	}
	sig, err := service.OAuth1Signature("POST", outcomesURL, op, secret)
	require.NoError(l.t, err)
	op["oauth_signature"] = sig
	var parts []string
	for k, v := range op {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, k, url.QueryEscape(v)))
	}
	return l.do(ltiReq{method: "POST", path: fmt.Sprintf("/lti/v1p1/tools/%d/outcomes", tool.ID), contentType: "application/xml",
		body: strings.NewReader(body), headers: map[string]string{"Authorization": "OAuth " + strings.Join(parts, ", ")}})
}

func htmlUnescape(s string) string {
	r := strings.NewReplacer("&amp;", "&", "&#43;", "+", "&#34;", `"`, "&#39;", "'", "&lt;", "<", "&gt;", ">", "&#61;", "=")
	return r.Replace(s)
}
