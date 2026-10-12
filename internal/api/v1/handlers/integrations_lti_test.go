package handlers_test

// Regression tests for the LTI 1.3 launch, AGS and NRPS fixes of the
// integrations parity wave (B077, B079, B080, B081, B082, B083, B086-adjacent,
// B186, B187, B188).

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// --- fakes ------------------------------------------------------------------

type fakeNonceRepo struct {
	mu   sync.Mutex
	seen map[string]bool
}

func (f *fakeNonceRepo) Create(_ context.Context, n *models.Nonce) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.seen == nil {
		f.seen = map[string]bool{}
	}
	if f.seen[n.Value] {
		return errors.New("duplicate")
	}
	f.seen[n.Value] = true
	return nil
}

func (f *fakeNonceRepo) Exists(_ context.Context, v string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.seen[v], nil
}

func (f *fakeNonceRepo) DeleteExpired(context.Context) error { return nil }

type fakeGrader struct {
	calls []string
	err   error
}

func (g *fakeGrader) Grade(_ context.Context, assignmentID, userID, graderID, accountID uint, grade string) (*models.Submission, error) {
	g.calls = append(g.calls, fmt.Sprintf("a=%d u=%d g=%d acct=%d grade=%s", assignmentID, userID, graderID, accountID, grade))
	if g.err != nil {
		return nil, g.err
	}
	return &models.Submission{AssignmentID: assignmentID, UserID: userID}, nil
}

// --- LTI launch (B080, B083, B186) --------------------------------------------

type ltiLaunchFixture struct {
	devRepo    *mocks.MockDeveloperKeyRepository
	configRepo *mocks.MockLTIToolConfigurationRepository
	courseRepo *mocks.MockCourseRepository
	enrollRepo *mocks.MockEnrollmentRepository
	toolRepo   *mocks.MockContextExternalToolRepository
	svc        *service.LTIService
	app        *fiber.App
}

func newLTILaunchFixture(t *testing.T) *ltiLaunchFixture {
	t.Helper()
	f := &ltiLaunchFixture{
		devRepo:    new(mocks.MockDeveloperKeyRepository),
		configRepo: new(mocks.MockLTIToolConfigurationRepository),
		courseRepo: new(mocks.MockCourseRepository),
		enrollRepo: new(mocks.MockEnrollmentRepository),
		toolRepo:   new(mocks.MockContextExternalToolRepository),
	}
	f.devRepo.On("FindByClientID", mock.Anything, "dk_tool").Return(&models.DeveloperKey{
		ID: 5, AccountID: 1, ClientID: "dk_tool", IsLTIKey: true, WorkflowState: "active",
	}, nil)
	f.configRepo.On("FindByDeveloperKeyID", mock.Anything, uint(5)).Return(&models.LTIToolConfiguration{
		ID: 1, DeveloperKeyID: 5, Title: "Example Tool",
		TargetLinkURI: "https://tool.example.com/launch", OIDCInitiationURL: "https://tool.example.com/login",
	}, nil)
	svc, err := service.NewLTIService(f.devRepo, f.configRepo, &fakeNonceRepo{}, f.enrollRepo, f.courseRepo, f.toolRepo, "https://lms.example.com")
	require.NoError(t, err)
	f.svc = svc

	h := handlers.NewLTIHandler(svc, nil, nil, nil, nil, nil, nil, nil)
	f.app = testutil.SetupTestApp()
	f.app.Use(integrationsCallerStub(7, 1, ""))
	f.app.Get("/lti/authorize", h.LaunchDirect)
	f.app.Post("/lti/oidc/login", h.OIDCLogin)
	return f
}

func (f *ltiLaunchFixture) allowCourse10() {
	f.courseRepo.On("FindByID", mock.Anything, uint(10), uint(1)).Return(&models.Course{ID: 10, AccountID: 1, Name: "Biology", CourseCode: "BIO-1"}, nil)
	f.enrollRepo.On("FindByUserAndCourse", mock.Anything, uint(7), uint(10), uint(1)).Return(&models.Enrollment{
		UserID: 7, CourseID: 10, Type: "StudentEnrollment", WorkflowState: "active",
	}, nil)
	f.toolRepo.On("ListByContext", mock.Anything, "Course", uint(10), mock.Anything).Return(&repository.PaginatedResult[models.ContextExternalTool]{
		Items: []models.ContextExternalTool{{ID: 1, DeveloperKeyID: 5, WorkflowState: "active"}}, TotalCount: 1,
	}, nil)
}

func authorizeQuery(overrides map[string]string) string {
	q := url.Values{}
	q.Set("scope", "openid")
	q.Set("response_type", "id_token")
	q.Set("response_mode", "form_post")
	q.Set("prompt", "none")
	q.Set("client_id", "dk_tool")
	q.Set("redirect_uri", "https://tool.example.com/launch")
	q.Set("login_hint", "7")
	q.Set("lti_message_hint", "10:rl-1:5")
	q.Set("state", "tool-state-1")
	q.Set("nonce", "tool-nonce-1")
	for k, v := range overrides {
		q.Set(k, v)
	}
	return "/lti/authorize?" + q.Encode()
}

func TestLTIOIDCLogin_SendsInitiationParameters(t *testing.T) {
	f := newLTILaunchFixture(t)
	resp := testutil.MakeRequest(f.app, http.MethodPost, "/lti/oidc/login", testutil.JSONBody(map[string]string{
		"client_id": "dk_tool", "lti_message_hint": "10:rl-1:5",
	}))
	require.Equal(t, http.StatusFound, resp.StatusCode)
	loc, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "tool.example.com", loc.Host)
	q := loc.Query()
	assert.Equal(t, "https://lms.example.com", q.Get("iss"))
	assert.Equal(t, "7", q.Get("login_hint"))
	assert.Equal(t, "https://tool.example.com/launch", q.Get("target_link_uri"))
	assert.Equal(t, "10:rl-1:5", q.Get("lti_message_hint"))
	assert.Equal(t, "dk_tool", q.Get("client_id"))
	assert.Equal(t, "5", q.Get("lti_deployment_id"))
	for _, p := range []string{"nonce", "state", "response_type", "scope", "prompt", "redirect_uri"} {
		assert.Empty(t, q.Get(p), "initiation must not carry %s", p)
	}
}

func TestLTIAuthorize_PostsToolNonceAndStateUnderLaunchCSP(t *testing.T) {
	f := newLTILaunchFixture(t)
	f.allowCourse10()

	resp := testutil.MakeRequest(f.app, http.MethodGet, authorizeQuery(nil), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	raw, _ := io.ReadAll(resp.Body)
	html := string(raw)

	// B083: the tool's state is echoed; the tool's nonce is in the token.
	assert.Contains(t, html, `name="state" value="tool-state-1"`)
	m := regexp.MustCompile(`name="id_token" value="([^"]+)"`).FindStringSubmatch(html)
	require.Len(t, m, 2)
	key, err := f.svc.GetPlatformKeyPair()
	require.NoError(t, err)
	claims := jwt.MapClaims{}
	_, err = jwt.ParseWithClaims(m[1], claims, func(*jwt.Token) (interface{}, error) { return &key.PublicKey, nil })
	require.NoError(t, err)
	assert.Equal(t, "tool-nonce-1", claims["nonce"])
	assert.Equal(t, "dk_tool", claims["aud"])

	// B080: the page's own CSP allows its nonce'd script and the tool origin.
	csp := resp.Header.Get("Content-Security-Policy")
	nonce := regexp.MustCompile(`<script nonce="([^"]+)">`).FindStringSubmatch(html)
	require.Len(t, nonce, 2)
	assert.Contains(t, csp, "script-src 'nonce-"+nonce[1]+"'")
	assert.Contains(t, csp, "form-action https://tool.example.com")
	assert.NotContains(t, csp, "frame-ancestors 'none'")
	assert.Equal(t, "SAMEORIGIN", resp.Header.Get("X-Frame-Options"))

	// The tool nonce is single-use.
	resp = testutil.MakeRequest(f.app, http.MethodGet, authorizeQuery(nil), nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestLTIAuthorize_RefusesUnauthorizedLaunches(t *testing.T) {
	t.Run("not enrolled", func(t *testing.T) {
		f := newLTILaunchFixture(t)
		f.courseRepo.On("FindByID", mock.Anything, uint(10), uint(1)).Return(&models.Course{ID: 10, AccountID: 1}, nil)
		f.enrollRepo.On("FindByUserAndCourse", mock.Anything, uint(7), uint(10), uint(1)).Return(nil, errors.New("not found"))
		resp := testutil.MakeRequest(f.app, http.MethodGet, authorizeQuery(nil), nil)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
	t.Run("course in another tenant", func(t *testing.T) {
		f := newLTILaunchFixture(t)
		f.courseRepo.On("FindByID", mock.Anything, uint(10), uint(1)).Return(nil, errors.New("not found"))
		resp := testutil.MakeRequest(f.app, http.MethodGet, authorizeQuery(nil), nil)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
	t.Run("tool not installed in course", func(t *testing.T) {
		f := newLTILaunchFixture(t)
		f.courseRepo.On("FindByID", mock.Anything, uint(10), uint(1)).Return(&models.Course{ID: 10, AccountID: 1}, nil)
		f.enrollRepo.On("FindByUserAndCourse", mock.Anything, uint(7), uint(10), uint(1)).Return(&models.Enrollment{Type: "StudentEnrollment", WorkflowState: "active"}, nil)
		f.toolRepo.On("ListByContext", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&repository.PaginatedResult[models.ContextExternalTool]{}, nil)
		resp := testutil.MakeRequest(f.app, http.MethodGet, authorizeQuery(nil), nil)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
	t.Run("unregistered redirect_uri", func(t *testing.T) {
		f := newLTILaunchFixture(t)
		f.allowCourse10()
		resp := testutil.MakeRequest(f.app, http.MethodGet, authorizeQuery(map[string]string{"redirect_uri": "https://evil.example.net/x"}), nil)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
	t.Run("missing nonce", func(t *testing.T) {
		f := newLTILaunchFixture(t)
		f.allowCourse10()
		resp := testutil.MakeRequest(f.app, http.MethodGet, authorizeQuery(map[string]string{"nonce": ""}), nil)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

// --- AGS (B077, B079, B081, B082, B187) ---------------------------------------

type agsFixture struct {
	lineItemRepo   *mocks.MockLTILineItemRepository
	resultRepo     *mocks.MockLTIResultRepository
	assignmentRepo *mocks.MockAssignmentRepository
	enrollRepo     *mocks.MockEnrollmentRepository
	grader         *fakeGrader
	handler        *handlers.LTIHandler
}

func newAGSFixture() *agsFixture {
	f := &agsFixture{
		lineItemRepo:   new(mocks.MockLTILineItemRepository),
		resultRepo:     new(mocks.MockLTIResultRepository),
		assignmentRepo: new(mocks.MockAssignmentRepository),
		enrollRepo:     new(mocks.MockEnrollmentRepository),
		grader:         &fakeGrader{},
	}
	svc := service.NewLTIAGSService(f.lineItemRepo, f.resultRepo, f.assignmentRepo, f.enrollRepo, f.grader)
	f.handler = handlers.NewLTIHandler(nil, svc, nil, nil, nil, nil, nil, nil)
	return f
}

func (f *agsFixture) scoreApp() *fiber.App {
	app := testutil.SetupTestApp()
	app.Post("/lti/courses/:course_id/line_items/:id/scores", integrationsCallerStub(3, 1, "TeacherEnrollment"), f.handler.PostScore)
	return app
}

func assignmentLinkedLineItem() *models.LTILineItem {
	aid := uint(70)
	return &models.LTILineItem{ID: 42, CourseID: 10, AssignmentID: &aid, Label: "Quiz", ScoreMaximum: 20}
}

func TestAGSCreateLineItem_RefusesAssignmentOfAnotherCourse(t *testing.T) {
	f := newAGSFixture()
	f.assignmentRepo.On("FindByID", mock.Anything, uint(99), uint(1)).Return(&models.Assignment{ID: 99, CourseID: 20}, nil)
	app := testutil.SetupTestApp()
	app.Post("/lti/courses/:course_id/line_items", integrationsCallerStub(3, 1, "TeacherEnrollment"), f.handler.CreateLineItem)

	resp := testutil.MakeRequest(app, http.MethodPost, "/lti/courses/10/line_items", testutil.JSONBody(map[string]interface{}{
		"label": "Steal", "scoreMaximum": 10, "assignmentId": 99,
	}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	f.lineItemRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestAGSPostScore_StringUserIDGradesThroughGradebook(t *testing.T) {
	f := newAGSFixture()
	f.lineItemRepo.On("FindByID", mock.Anything, uint(42)).Return(assignmentLinkedLineItem(), nil)
	f.enrollRepo.On("FindByUserAndCourse", mock.Anything, uint(500), uint(10), uint(1)).Return(&models.Enrollment{
		UserID: 500, CourseID: 10, Type: "StudentEnrollment", WorkflowState: "active",
	}, nil)
	f.assignmentRepo.On("FindByID", mock.Anything, uint(70), uint(1)).Return(&models.Assignment{ID: 70, CourseID: 10}, nil)
	f.resultRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*models.LTIResult")).Return(nil)

	// B081: userId as a JSON string, per the AGS spec.
	resp := testutil.MakeRequest(f.scoreApp(), http.MethodPost, "/lti/courses/10/line_items/42/scores", testutil.JSONBody(map[string]interface{}{
		"userId": "500", "scoreGiven": 8, "scoreMaximum": 10,
		"activityProgress": "Completed", "gradingProgress": "FullyGraded",
	}))
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	// B082: graded through SubmissionService.Grade (which creates the
	// submission when none exists), scaled to the line item's maximum.
	require.Len(t, f.grader.calls, 1)
	assert.Equal(t, "a=70 u=500 g=3 acct=1 grade=16", f.grader.calls[0])
	f.resultRepo.AssertCalled(t, "Upsert", mock.Anything, mock.Anything)
}

// A line item linked to an existing assignment can carry a scoreMaximum
// that differs from the assignment's points_possible (Paper does not sync
// them as Canvas does). The gradebook grade must be on the assignment's
// scale: 8/10 on a 100-point assignment is 80, not 16.
func TestAGSPostScore_ScalesToAssignmentPointsPossible(t *testing.T) {
	f := newAGSFixture()
	f.lineItemRepo.On("FindByID", mock.Anything, uint(42)).Return(assignmentLinkedLineItem(), nil)
	f.enrollRepo.On("FindByUserAndCourse", mock.Anything, uint(500), uint(10), uint(1)).Return(&models.Enrollment{
		UserID: 500, CourseID: 10, Type: "StudentEnrollment", WorkflowState: "active",
	}, nil)
	points := 100.0
	f.assignmentRepo.On("FindByID", mock.Anything, uint(70), uint(1)).Return(&models.Assignment{ID: 70, CourseID: 10, PointsPossible: &points}, nil)
	f.resultRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*models.LTIResult")).Return(nil)

	resp := testutil.MakeRequest(f.scoreApp(), http.MethodPost, "/lti/courses/10/line_items/42/scores", testutil.JSONBody(map[string]interface{}{
		"userId": "500", "scoreGiven": 8, "scoreMaximum": 10,
		"activityProgress": "Completed", "gradingProgress": "FullyGraded",
	}))
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	require.Len(t, f.grader.calls, 1)
	assert.Equal(t, "a=70 u=500 g=3 acct=1 grade=80", f.grader.calls[0])
}

func TestAGSPostScore_PendingIsNotWrittenToGradebook(t *testing.T) {
	f := newAGSFixture()
	f.lineItemRepo.On("FindByID", mock.Anything, uint(42)).Return(assignmentLinkedLineItem(), nil)
	f.enrollRepo.On("FindByUserAndCourse", mock.Anything, uint(500), uint(10), uint(1)).Return(&models.Enrollment{Type: "StudentEnrollment", WorkflowState: "active"}, nil)
	f.assignmentRepo.On("FindByID", mock.Anything, uint(70), uint(1)).Return(&models.Assignment{ID: 70, CourseID: 10}, nil)
	f.resultRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	resp := testutil.MakeRequest(f.scoreApp(), http.MethodPost, "/lti/courses/10/line_items/42/scores", testutil.JSONBody(map[string]interface{}{
		"userId": 500, "scoreGiven": 8, "scoreMaximum": 10,
		"activityProgress": "Submitted", "gradingProgress": "Pending",
	}))
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Empty(t, f.grader.calls)
}

func TestAGSPostScore_ClosedPeriodRefusalFailsTheRequest(t *testing.T) {
	f := newAGSFixture()
	f.grader.err = errors.New(`grading period "Q1" is closed`)
	f.lineItemRepo.On("FindByID", mock.Anything, uint(42)).Return(assignmentLinkedLineItem(), nil)
	f.enrollRepo.On("FindByUserAndCourse", mock.Anything, uint(500), uint(10), uint(1)).Return(&models.Enrollment{Type: "StudentEnrollment", WorkflowState: "active"}, nil)
	f.assignmentRepo.On("FindByID", mock.Anything, uint(70), uint(1)).Return(&models.Assignment{ID: 70, CourseID: 10}, nil)

	resp := testutil.MakeRequest(f.scoreApp(), http.MethodPost, "/lti/courses/10/line_items/42/scores", testutil.JSONBody(map[string]interface{}{
		"userId": "500", "scoreGiven": 8, "scoreMaximum": 10,
		"activityProgress": "Completed", "gradingProgress": "FullyGraded",
	}))
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	f.resultRepo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
}

func TestAGSPostScore_RefusesUserOutsideCourse(t *testing.T) {
	f := newAGSFixture()
	f.lineItemRepo.On("FindByID", mock.Anything, uint(42)).Return(assignmentLinkedLineItem(), nil)
	// Student of another course / tenant: no enrollment under (course 10, tenant 1).
	f.enrollRepo.On("FindByUserAndCourse", mock.Anything, uint(900), uint(10), uint(1)).Return(nil, errors.New("not found"))

	resp := testutil.MakeRequest(f.scoreApp(), http.MethodPost, "/lti/courses/10/line_items/42/scores", testutil.JSONBody(map[string]interface{}{
		"userId": "900", "scoreGiven": 10, "scoreMaximum": 10,
		"activityProgress": "Completed", "gradingProgress": "FullyGraded",
	}))
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	assert.Empty(t, f.grader.calls)
	f.resultRepo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
}

func TestAGSGetResults_StudentSeesOnlyOwnResult(t *testing.T) {
	f := newAGSFixture()
	f.lineItemRepo.On("FindByID", mock.Anything, uint(42)).Return(assignmentLinkedLineItem(), nil)
	f.resultRepo.On("ListByLineItem", mock.Anything, uint(42), uint(7), mock.Anything).Return(&repository.PaginatedResult[models.LTIResult]{
		Items: []models.LTIResult{{ID: 1, UserID: 7}}, TotalCount: 1, Page: 1, PerPage: 10,
	}, nil)
	f.resultRepo.On("ListByLineItem", mock.Anything, uint(42), uint(0), mock.Anything).Return(&repository.PaginatedResult[models.LTIResult]{
		Items: []models.LTIResult{{ID: 1, UserID: 7}, {ID: 2, UserID: 8}}, TotalCount: 2, Page: 1, PerPage: 10,
	}, nil)

	student := testutil.SetupTestApp()
	student.Get("/lti/courses/:course_id/line_items/:id/results", integrationsCallerStub(7, 1, "StudentEnrollment"), f.handler.GetResults)
	resp := testutil.MakeRequest(student, http.MethodGet, "/lti/courses/10/line_items/42/results", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	arr, _ := testutil.ParseJSONArray(resp)
	assert.Len(t, arr, 1)
	f.resultRepo.AssertNotCalled(t, "ListByLineItem", mock.Anything, uint(42), uint(0), mock.Anything)

	teacher := testutil.SetupTestApp()
	teacher.Get("/lti/courses/:course_id/line_items/:id/results", integrationsCallerStub(3, 1, "TeacherEnrollment"), f.handler.GetResults)
	resp = testutil.MakeRequest(teacher, http.MethodGet, "/lti/courses/10/line_items/42/results", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	arr, _ = testutil.ParseJSONArray(resp)
	assert.Len(t, arr, 2)
}

func TestAGSGetLineItem_IDIsTheItemURL(t *testing.T) {
	f := newAGSFixture()
	f.lineItemRepo.On("FindByID", mock.Anything, uint(42)).Return(assignmentLinkedLineItem(), nil)
	app := testutil.SetupTestApp()
	app.Get("/lti/courses/:course_id/line_items/:id", integrationsCallerStub(3, 1, "TeacherEnrollment"), f.handler.GetLineItem)

	resp := testutil.MakeRequest(app, http.MethodGet, "/lti/courses/10/line_items/42", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := testutil.ParseJSONMap(resp)
	id, _ := body["id"].(string)
	assert.True(t, strings.HasSuffix(id, "/lti/courses/10/line_items/42"), id)
	assert.NotContains(t, id, "/42/42")
}

// --- NRPS (B188) ----------------------------------------------------------------

func TestNRPSMemberships_LinkHeaderPagesTheRoster(t *testing.T) {
	enrollmentRepo := new(mocks.MockEnrollmentRepository)
	userRepo := new(mocks.MockUserRepository)
	items := make([]models.Enrollment, 0, 10)
	for i := 1; i <= 10; i++ {
		items = append(items, models.Enrollment{UserID: uint(100 + i), CourseID: 10, Type: "StudentEnrollment", WorkflowState: "active"})
	}
	enrollmentRepo.On("ListByCourseID", mock.Anything, uint(10), uint(1), repository.PaginationParams{Page: 1, PerPage: 10}).
		Return(&repository.PaginatedResult[models.Enrollment]{Items: items, TotalCount: 30, Page: 1, PerPage: 10}, nil)
	userRepo.On("FindByID", mock.Anything, mock.Anything, uint(1)).Return(&models.User{ID: 101, Name: "Jane Doe"}, nil)

	h := handlers.NewLTIHandler(nil, nil, service.NewLTINRPSService(enrollmentRepo, userRepo), nil, nil, nil, nil, nil)
	app := testutil.SetupTestApp()
	app.Get("/lti/courses/:course_id/memberships", integrationsCallerStub(3, 1, "TeacherEnrollment"), h.GetMemberships)

	resp := testutil.MakeRequest(app, http.MethodGet, "/lti/courses/10/memberships?limit=10", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	link := resp.Header.Get("Link")
	assert.Contains(t, link, `page=2&per_page=10>; rel="next"`)
	assert.Contains(t, link, `rel="last"`)
}

// --- JWKS/platform key sanity (B181 handler side) --------------------------------

func TestLTIJWKS_KidIsKeyThumbprint(t *testing.T) {
	f := newLTILaunchFixture(t)
	jwks, err := f.svc.GetJWKS()
	require.NoError(t, err)
	k := jwks["keys"].([]interface{})[0].(map[string]interface{})
	n, _ := base64.RawURLEncoding.DecodeString(k["n"].(string))
	pub := &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: 65537}
	assert.Equal(t, service.JWKThumbprint(pub), k["kid"])
}
