package handlers_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// Fixture: tenant 1. User 5 owns portfolio 10; user 6 owns portfolio 11.
type portfolioFixture struct {
	portfolioRepo  *mocks.MockPortfolioRepository
	sectionRepo    *mocks.MockPortfolioSectionRepository
	artifactRepo   *mocks.MockPortfolioArtifactRepository
	reflectionRepo *mocks.MockPortfolioReflectionRepository
	commentRepo    *mocks.MockPortfolioCommentRepository
	submissionRepo *mocks.MockSubmissionRepository
	assignmentRepo *mocks.MockAssignmentRepository
	userRepo       *mocks.MockUserRepository
	app            *fiber.App
}

func newPortfolioFixture(t *testing.T, callerID uint) *portfolioFixture {
	t.Helper()
	f := &portfolioFixture{
		portfolioRepo:  new(mocks.MockPortfolioRepository),
		sectionRepo:    new(mocks.MockPortfolioSectionRepository),
		artifactRepo:   new(mocks.MockPortfolioArtifactRepository),
		reflectionRepo: new(mocks.MockPortfolioReflectionRepository),
		commentRepo:    new(mocks.MockPortfolioCommentRepository),
		submissionRepo: new(mocks.MockSubmissionRepository),
		assignmentRepo: new(mocks.MockAssignmentRepository),
		userRepo:       new(mocks.MockUserRepository),
	}
	// Non-admin roles for the authorizer's admin lookup.
	f.userRepo.On("FindByID", mock.Anything, uint(5), uint(0)).Return(&models.User{ID: 5, Role: "student"}, nil).Maybe()
	f.userRepo.On("FindByID", mock.Anything, uint(6), uint(0)).Return(&models.User{ID: 6, Role: "student"}, nil).Maybe()
	f.userRepo.On("FindByID", mock.Anything, uint(7), uint(0)).Return(&models.User{ID: 7, Role: "teacher"}, nil).Maybe()
	f.userRepo.On("FindByIDs", mock.Anything, mock.Anything, uint(0)).Return([]models.User{
		{ID: 5, Name: "Jane Doe"}, {ID: 7, Name: "John Teacher"},
	}, nil).Maybe()

	f.portfolioRepo.On("FindByID", mock.Anything, uint(10), uint(1)).Return(&models.Portfolio{
		ID: 10, UserID: 5, Title: "Mine", Slug: "mine-1", WorkflowState: models.PortfolioDraft,
	}, nil).Maybe()
	f.portfolioRepo.On("FindByID", mock.Anything, uint(11), uint(1)).Return(&models.Portfolio{
		ID: 11, UserID: 6, Title: "Theirs", Slug: "theirs-1", WorkflowState: models.PortfolioDraft,
	}, nil).Maybe()

	svc := service.NewPortfolioService(f.portfolioRepo, f.sectionRepo, f.artifactRepo, f.reflectionRepo,
		new(mocks.MockPortfolioTemplateRepository), f.commentRepo, f.submissionRepo, f.assignmentRepo)
	svc.SetUserRepository(f.userRepo)
	authz := handlers.NewResourceAuthorizer(new(mocks.MockEnrollmentRepository), f.userRepo)
	h := handlers.NewPortfolioHandler(svc, authz)

	app := testutil.SetupTestApp()
	app.Use(authStub(callerID, 1), middleware.PaginationParams())
	app.Get("/portfolios/:id", h.GetPortfolio)
	app.Put("/portfolios/:id", h.UpdatePortfolio)
	app.Put("/portfolios/:id/sections/:section_id", h.UpdateSection)
	app.Delete("/portfolios/:id/sections/:section_id", h.DeleteSection)
	app.Put("/portfolios/:id/artifacts/:artifact_id", h.UpdateArtifact)
	app.Delete("/portfolios/:id/artifacts/:artifact_id", h.DeleteArtifact)
	app.Post("/portfolios/:id/artifacts/:artifact_id/reflections", h.AddReflection)
	app.Post("/portfolios/:id/import", h.ImportFromCourse)
	app.Get("/portfolios/:id/export/pdf", h.ExportAsPDF)
	app.Post("/portfolios/:id/comments", h.AddComment)
	f.app = app
	return f
}

func body(s string) io.Reader { return bytes.NewReader([]byte(s)) }

// B096: the owner of portfolio 10 must not touch a section/artifact that
// belongs to portfolio 11 by putting its id under their own portfolio.
func TestPortfolio_CrossPortfolioSectionAndArtifact_Return404(t *testing.T) {
	f := newPortfolioFixture(t, 5)
	f.sectionRepo.On("FindByID", mock.Anything, uint(77), uint(1)).
		Return(&models.PortfolioSection{ID: 77, PortfolioID: 11, Title: "B's", SectionType: "about"}, nil)
	f.artifactRepo.On("FindByID", mock.Anything, uint(88), uint(1)).
		Return(&models.PortfolioArtifact{ID: 88, PortfolioID: 11, Title: "B's", ArtifactType: "link"}, nil)

	resp := testutil.MakeRequest(f.app, http.MethodDelete, "/portfolios/10/sections/77", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp = testutil.MakeRequest(f.app, http.MethodPut, "/portfolios/10/sections/77", body(`{"section":{"title":"pwned"}}`))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp = testutil.MakeRequest(f.app, http.MethodDelete, "/portfolios/10/artifacts/88", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp = testutil.MakeRequest(f.app, http.MethodPut, "/portfolios/10/artifacts/88", body(`{"artifact":{"title":"pwned"}}`))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp = testutil.MakeRequest(f.app, http.MethodPost, "/portfolios/10/artifacts/88/reflections", body(`{"reflection":{"content":"x"}}`))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	f.sectionRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
	f.sectionRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	f.artifactRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
	f.artifactRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

// B095: a partial update (moving a section) must keep the other columns.
func TestPortfolio_UpdateSection_PartialUpdateKeepsColumns(t *testing.T) {
	f := newPortfolioFixture(t, 5)
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	f.sectionRepo.On("FindByID", mock.Anything, uint(70), uint(1)).Return(&models.PortfolioSection{
		ID: 70, PortfolioID: 10, Title: "About me", SectionType: "about", Content: "<p>hi</p>",
		Layout: "standard", IsVisible: true, Position: 0, CreatedAt: created,
	}, nil)
	var saved *models.PortfolioSection
	f.sectionRepo.On("Update", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		saved = args.Get(1).(*models.PortfolioSection)
	}).Return(nil)

	resp := testutil.MakeRequest(f.app, http.MethodPut, "/portfolios/10/sections/70", body(`{"section":{"position":2}}`))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotNil(t, saved)
	assert.Equal(t, uint(10), saved.PortfolioID)
	assert.Equal(t, "About me", saved.Title)
	assert.Equal(t, "about", saved.SectionType)
	assert.Equal(t, "<p>hi</p>", saved.Content)
	assert.Equal(t, 2, saved.Position)
	assert.True(t, saved.IsVisible)
}

func TestPortfolio_UpdateArtifact_PartialUpdateKeepsColumns(t *testing.T) {
	f := newPortfolioFixture(t, 5)
	f.artifactRepo.On("FindByID", mock.Anything, uint(80), uint(1)).Return(&models.PortfolioArtifact{
		ID: 80, PortfolioID: 10, Title: "Essay", ArtifactType: "link", ContentURL: "https://example.com/essay",
		Tags: `["writing"]`,
	}, nil)
	var saved *models.PortfolioArtifact
	f.artifactRepo.On("Update", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		saved = args.Get(1).(*models.PortfolioArtifact)
	}).Return(nil)

	resp := testutil.MakeRequest(f.app, http.MethodPut, "/portfolios/10/artifacts/80", body(`{"artifact":{"is_featured":true}}`))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotNil(t, saved)
	assert.Equal(t, "Essay", saved.Title)
	assert.Equal(t, "link", saved.ArtifactType)
	assert.Equal(t, "https://example.com/essay", saved.ContentURL)
	assert.Equal(t, `["writing"]`, saved.Tags)
	assert.True(t, saved.IsFeatured)
}

// B098: the editor needs sections and artifacts from GET and PUT.
func TestPortfolio_GetAndUpdateReturnSectionsAndArtifacts(t *testing.T) {
	f := newPortfolioFixture(t, 5)
	sectionID := uint(70)
	f.sectionRepo.On("ListByPortfolioID", mock.Anything, uint(10)).Return([]models.PortfolioSection{
		{ID: 70, PortfolioID: 10, Title: "About", SectionType: "about", IsVisible: true},
	}, nil)
	f.artifactRepo.On("ListByPortfolioID", mock.Anything, uint(10), mock.Anything).Return(&repository.PaginatedResult[models.PortfolioArtifact]{
		Items: []models.PortfolioArtifact{{ID: 80, PortfolioID: 10, SectionID: &sectionID, Title: "Essay", ArtifactType: "link"}},
	}, nil)
	f.reflectionRepo.On("ListByArtifactID", mock.Anything, uint(80)).Return([]models.PortfolioReflection{
		{ID: 1, ArtifactID: 80, UserID: 5, Content: "I learned a lot"},
	}, nil)
	f.portfolioRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

	for _, req := range []struct{ method, body string }{
		{http.MethodGet, ""},
		{http.MethodPut, `{"portfolio":{"tagline":"hello"}}`},
	} {
		var rdr io.Reader
		if req.body != "" {
			rdr = body(req.body)
		}
		resp := testutil.MakeRequest(f.app, req.method, "/portfolios/10", rdr)
		require.Equal(t, http.StatusOK, resp.StatusCode, req.method)
		m, err := testutil.ParseJSONMap(resp)
		require.NoError(t, err)
		require.Len(t, m["sections"], 1, req.method)
		arts := m["artifacts"].([]interface{})
		require.Len(t, arts, 1, req.method)
		assert.Equal(t, "I learned a lot", arts[0].(map[string]interface{})["reflection"])
	}
}

// B197: Archive (workflow_state via PUT) must take effect.
func TestPortfolio_UpdateWorkflowStateArchives(t *testing.T) {
	f := newPortfolioFixture(t, 5)
	f.sectionRepo.On("ListByPortfolioID", mock.Anything, uint(10)).Return([]models.PortfolioSection{}, nil)
	f.artifactRepo.On("ListByPortfolioID", mock.Anything, uint(10), mock.Anything).Return(&repository.PaginatedResult[models.PortfolioArtifact]{}, nil)
	var saved *models.Portfolio
	f.portfolioRepo.On("Update", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		saved = args.Get(1).(*models.Portfolio)
	}).Return(nil)

	resp := testutil.MakeRequest(f.app, http.MethodPut, "/portfolios/10", body(`{"portfolio":{"workflow_state":"archived"}}`))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotNil(t, saved)
	assert.Equal(t, models.PortfolioArchived, saved.WorkflowState)

	resp = testutil.MakeRequest(f.app, http.MethodPut, "/portfolios/10", body(`{"portfolio":{"workflow_state":"bogus"}}`))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// B198: import takes the course from the body, imports only the owner's
// submitted work in that course, and skips anyone else's submission.
func TestPortfolio_ImportFromCourse(t *testing.T) {
	f := newPortfolioFixture(t, 5)
	now := time.Now()
	f.submissionRepo.On("ListByUserAndCourse", mock.Anything, uint(5), uint(300)).Return([]models.Submission{
		{ID: 1, UserID: 5, AssignmentID: 40, SubmittedAt: &now},
		{ID: 2, UserID: 5, AssignmentID: 41}, // never submitted
	}, nil)
	f.assignmentRepo.On("FindByID", mock.Anything, uint(40), uint(1)).Return(&models.Assignment{ID: 40, CourseID: 300, Name: "Lab report"}, nil)
	f.artifactRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	resp := testutil.MakeRequest(f.app, http.MethodPost, "/portfolios/10/import", body(`{"course_id":300}`))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	m, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	assert.Equal(t, float64(1), m["imported_count"])

	// Explicit submission ids: a classmate's submission is skipped.
	f2 := newPortfolioFixture(t, 5)
	f2.submissionRepo.On("FindByID", mock.Anything, uint(9), uint(1)).Return(&models.Submission{ID: 9, UserID: 6, AssignmentID: 40, SubmittedAt: &now}, nil)
	resp = testutil.MakeRequest(f2.app, http.MethodPost, "/portfolios/10/import", body(`{"course_id":300,"submission_ids":[9]}`))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	m, err = testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), m["imported_count"])
	f2.artifactRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// B200: Export as PDF must return an actual PDF.
func TestPortfolio_ExportAsPDF_IsPDF(t *testing.T) {
	f := newPortfolioFixture(t, 5)
	f.sectionRepo.On("ListByPortfolioID", mock.Anything, uint(10)).Return([]models.PortfolioSection{
		{ID: 70, PortfolioID: 10, Title: "About (me)", SectionType: "about", Content: "<p>Hello <b>world</b></p>", IsVisible: true},
	}, nil)
	f.artifactRepo.On("ListByPortfolioID", mock.Anything, uint(10), mock.Anything).Return(&repository.PaginatedResult[models.PortfolioArtifact]{}, nil)

	resp := testutil.MakeRequest(f.app, http.MethodGet, "/portfolios/10/export/pdf", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/pdf", resp.Header.Get("Content-Type"))
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.True(t, bytes.HasPrefix(data, []byte("%PDF-1.4")))
	assert.True(t, bytes.HasSuffix(bytes.TrimSpace(data), []byte("%%EOF")))
	assert.Contains(t, string(data), `About \(me\)`)
	assert.Contains(t, string(data), "Hello world")
}

// B199: a viewer of a public portfolio can comment as themselves; the
// body's user_id is ignored; a private portfolio is not commentable by
// non-owners.
func TestPortfolio_AddComment_ViewerPostsAsSelf(t *testing.T) {
	f := newPortfolioFixture(t, 7)
	f.portfolioRepo.On("FindByID", mock.Anything, uint(12), uint(1)).Return(&models.Portfolio{
		ID: 12, UserID: 5, Title: "Public", Slug: "pub-1", WorkflowState: models.PortfolioPublished, IsPublic: true,
	}, nil)
	var saved *models.PortfolioComment
	f.commentRepo.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		saved = args.Get(1).(*models.PortfolioComment)
	}).Return(nil)

	resp := testutil.MakeRequest(f.app, http.MethodPost, "/portfolios/12/comments", body(`{"comment":{"content":"Great work","user_id":5}}`))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.NotNil(t, saved)
	assert.Equal(t, uint(7), saved.UserID, "author must be the caller, not the body user_id")
	m, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	assert.Equal(t, "John Teacher", m["user_name"])

	// Draft portfolio of user 5: user 7 may not comment.
	resp = testutil.MakeRequest(f.app, http.MethodPost, "/portfolios/10/comments", body(`{"comment":{"content":"hi"}}`))
	assert.GreaterOrEqual(t, resp.StatusCode, 400)
	f.commentRepo.AssertNumberOfCalls(t, "Create", 1)
}

// B094: the public endpoint resolves the slug and hides hidden sections.
func TestPortfolio_GetPublicPortfolio_BySlug(t *testing.T) {
	portfolioRepo := new(mocks.MockPortfolioRepository)
	sectionRepo := new(mocks.MockPortfolioSectionRepository)
	artifactRepo := new(mocks.MockPortfolioArtifactRepository)
	reflectionRepo := new(mocks.MockPortfolioReflectionRepository)
	portfolioRepo.On("FindBySlug", mock.Anything, "jane-doe-1").Return(&models.Portfolio{
		ID: 9, UserID: 5, Title: "Jane", Slug: "jane-doe-1", WorkflowState: models.PortfolioPublished, IsPublic: true,
	}, nil)
	portfolioRepo.On("FindBySlug", mock.Anything, "draft-1").Return(&models.Portfolio{
		ID: 3, UserID: 5, Title: "Draft", Slug: "draft-1", WorkflowState: models.PortfolioDraft,
	}, nil)
	portfolioRepo.On("IncrementViewCount", mock.Anything, uint(9)).Return(nil)
	hiddenID, attachID := uint(2), uint(55)
	sectionRepo.On("ListByPortfolioID", mock.Anything, uint(9)).Return([]models.PortfolioSection{
		{ID: 1, PortfolioID: 9, Title: "Shown", SectionType: "about", IsVisible: true},
		{ID: 2, PortfolioID: 9, Title: "Hidden", SectionType: "custom", IsVisible: false},
	}, nil)
	artifactRepo.On("ListByPortfolioID", mock.Anything, uint(9), mock.Anything).Return(&repository.PaginatedResult[models.PortfolioArtifact]{
		Items: []models.PortfolioArtifact{
			{ID: 1, PortfolioID: 9, Title: "loose", ArtifactType: "document", SourceType: "upload", SourceID: &attachID, ContentURL: "/api/v1/portfolios/9/files/55"},
			{ID: 2, PortfolioID: 9, SectionID: &hiddenID, Title: "in hidden", ArtifactType: "link"},
		},
	}, nil)
	reflectionRepo.On("ListByArtifactID", mock.Anything, mock.Anything).Return([]models.PortfolioReflection{}, nil)

	svc := service.NewPortfolioService(portfolioRepo, sectionRepo, artifactRepo, reflectionRepo,
		new(mocks.MockPortfolioTemplateRepository), new(mocks.MockPortfolioCommentRepository),
		new(mocks.MockSubmissionRepository), new(mocks.MockAssignmentRepository))
	h := handlers.NewPortfolioHandler(svc, handlers.NewResourceAuthorizer(new(mocks.MockEnrollmentRepository), new(mocks.MockUserRepository)))
	app := testutil.SetupTestApp()
	app.Get("/portfolios/public/:slug", h.GetPublicPortfolio)

	resp := testutil.MakeRequest(app, http.MethodGet, "/portfolios/public/jane-doe-1", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	m, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	sections := m["sections"].([]interface{})
	require.Len(t, sections, 1)
	assert.Equal(t, "Shown", sections[0].(map[string]interface{})["title"])
	arts := m["artifacts"].([]interface{})
	require.Len(t, arts, 1, "artifacts of hidden sections stay private")
	assert.Equal(t, "/api/v1/portfolios/public/jane-doe-1/files/55", arts[0].(map[string]interface{})["content_url"])

	resp = testutil.MakeRequest(app, http.MethodGet, "/portfolios/public/draft-1", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
