package handlers_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// G155 fixture: tenant 1. User 5 owns private portfolio 10 (share token
// "tok-0123456789abcdef"); user 7 is another user of the tenant.
const sharedToken = "tok-0123456789abcdef"

type sharingFixture struct {
	portfolioRepo  *mocks.MockPortfolioRepository
	sectionRepo    *mocks.MockPortfolioSectionRepository
	artifactRepo   *mocks.MockPortfolioArtifactRepository
	commentRepo    *mocks.MockPortfolioCommentRepository
	submissionRepo *mocks.MockSubmissionRepository
	assignmentRepo *mocks.MockAssignmentRepository
	portfolio      *models.Portfolio
	app            *fiber.App
}

func newSharingFixture(t *testing.T, callerID uint) *sharingFixture {
	t.Helper()
	tok := sharedToken
	f := &sharingFixture{
		portfolioRepo:  new(mocks.MockPortfolioRepository),
		sectionRepo:    new(mocks.MockPortfolioSectionRepository),
		artifactRepo:   new(mocks.MockPortfolioArtifactRepository),
		commentRepo:    new(mocks.MockPortfolioCommentRepository),
		submissionRepo: new(mocks.MockSubmissionRepository),
		assignmentRepo: new(mocks.MockAssignmentRepository),
		portfolio:      &models.Portfolio{ID: 10, UserID: 5, Title: "Mine", Slug: "mine-1", WorkflowState: models.PortfolioDraft, ShareToken: &tok},
	}
	userRepo := new(mocks.MockUserRepository)
	userRepo.On("FindByID", mock.Anything, mock.Anything, mock.Anything).Return(&models.User{ID: 7, Role: "student"}, nil).Maybe()
	userRepo.On("FindByIDs", mock.Anything, mock.Anything, mock.Anything).Return([]models.User{{ID: 5, Name: "Jane Doe"}}, nil).Maybe()
	f.portfolioRepo.On("FindByID", mock.Anything, uint(10), uint(1)).Return(f.portfolio, nil).Maybe()
	f.portfolioRepo.On("FindByShareToken", mock.Anything, sharedToken).Return(f.portfolio, nil).Maybe()
	f.portfolioRepo.On("FindByShareToken", mock.Anything, mock.Anything).Return(nil, gorm.ErrRecordNotFound).Maybe()
	f.portfolioRepo.On("IncrementViewCount", mock.Anything, uint(10)).Return(nil).Maybe()
	f.sectionRepo.On("ListByPortfolioID", mock.Anything, uint(10)).Return([]models.PortfolioSection{
		{ID: 1, PortfolioID: 10, Title: "Shown", IsVisible: true},
		{ID: 2, PortfolioID: 10, Title: "Hidden", IsVisible: false},
	}, nil).Maybe()
	attID := uint(77)
	f.artifactRepo.On("ListByPortfolioID", mock.Anything, uint(10), mock.Anything).Return(&repository.PaginatedResult[models.PortfolioArtifact]{
		Items: []models.PortfolioArtifact{{ID: 3, PortfolioID: 10, Title: "Essay file", SourceType: "upload", SourceID: &attID, ContentURL: "/api/v1/portfolios/10/files/77"}},
	}, nil).Maybe()
	reflections := new(mocks.MockPortfolioReflectionRepository)
	reflections.On("ListByArtifactID", mock.Anything, mock.Anything).Return([]models.PortfolioReflection{}, nil).Maybe()

	svc := service.NewPortfolioService(f.portfolioRepo, f.sectionRepo, f.artifactRepo, reflections,
		new(mocks.MockPortfolioTemplateRepository), f.commentRepo, f.submissionRepo, f.assignmentRepo)
	svc.SetUserRepository(userRepo)
	authz := handlers.NewResourceAuthorizer(new(mocks.MockEnrollmentRepository), userRepo, new(mocks.MockCourseRepository))
	h := handlers.NewPortfolioHandler(svc, authz)

	app := testutil.SetupTestApp()
	// Public routes first (no auth), as in the router.
	app.Get("/portfolios/shared/:token", h.GetSharedPortfolio)
	app.Use(authStub(callerID, 1), middleware.PaginationParams())
	app.Get("/portfolios/:id", h.GetPortfolio)
	app.Post("/portfolios/:id/share_link", h.CreateShareLink)
	app.Delete("/portfolios/:id/share_link", h.RevokeShareLink)
	app.Post("/portfolios/:id/comments", h.AddComment)
	app.Get("/portfolios/:id/comments", h.ListComments)
	app.Delete("/portfolios/:id/comments/:comment_id", h.DeleteComment)
	app.Get("/portfolios/:id/importable_submissions", h.ListImportableSubmissions)
	app.Post("/portfolios/:id/import", h.ImportFromCourse)
	f.app = app
	return f
}

func TestSharedPortfolio_LinkGivesVisitorView(t *testing.T) {
	f := newSharingFixture(t, 7)

	resp := testutil.MakeRequest(f.app, http.MethodGet, "/portfolios/shared/"+sharedToken, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	m, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	sections := m["sections"].([]interface{})
	require.Len(t, sections, 1, "hidden sections stay hidden")
	art := m["artifacts"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, "/api/v1/portfolios/shared/"+sharedToken+"/files/77", art["content_url"])
	_, leaked := m["share_token"]
	assert.False(t, leaked, "the visitor view never echoes the token")

	resp = testutil.MakeRequest(f.app, http.MethodGet, "/portfolios/shared/wrong-token-000000000", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Logged in, the private portfolio is readable only with the token.
	resp = testutil.MakeRequest(f.app, http.MethodGet, "/portfolios/10", nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp = testutil.MakeRequest(f.app, http.MethodGet, "/portfolios/10?share_token="+sharedToken, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestShareLink_OwnerOnly(t *testing.T) {
	f := newSharingFixture(t, 7)
	resp := testutil.MakeRequest(f.app, http.MethodPost, "/portfolios/10/share_link", nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	owner := newSharingFixture(t, 5)
	var saved *models.Portfolio
	owner.portfolioRepo.On("Update", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		saved = args.Get(1).(*models.Portfolio)
	}).Return(nil)
	resp = testutil.MakeRequest(owner.app, http.MethodPost, "/portfolios/10/share_link", nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	m, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	tok, _ := m["share_token"].(string)
	assert.GreaterOrEqual(t, len(tok), 30)
	require.NotNil(t, saved.ShareToken)
	assert.Equal(t, tok, *saved.ShareToken)
	assert.NotEqual(t, sharedToken, tok, "a new link replaces the old one")

	resp = testutil.MakeRequest(owner.app, http.MethodDelete, "/portfolios/10/share_link", nil)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Nil(t, saved.ShareToken)
}

func TestPortfolioComments_DisabledAndModeration(t *testing.T) {
	f := newSharingFixture(t, 7)
	f.portfolio.CommentsDisabled = true
	resp := testutil.MakeRequest(f.app, http.MethodPost, "/portfolios/10/comments?share_token="+sharedToken, body(`{"comment":{"content":"Nice"}}`))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "comments turned off")

	// A viewer can delete only their own comment; the owner any comment.
	viewer := newSharingFixture(t, 7)
	viewer.commentRepo.On("FindByID", mock.Anything, uint(50), uint(1)).Return(&models.PortfolioComment{ID: 50, PortfolioID: 10, UserID: 8}, nil)
	viewer.commentRepo.On("FindByID", mock.Anything, uint(51), uint(1)).Return(&models.PortfolioComment{ID: 51, PortfolioID: 10, UserID: 7}, nil)
	viewer.commentRepo.On("FindByID", mock.Anything, uint(52), uint(1)).Return(&models.PortfolioComment{ID: 52, PortfolioID: 99, UserID: 7}, nil)
	viewer.commentRepo.On("Delete", mock.Anything, uint(51)).Return(nil)
	q := "?share_token=" + sharedToken
	resp = testutil.MakeRequest(viewer.app, http.MethodDelete, "/portfolios/10/comments/50"+q, nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp = testutil.MakeRequest(viewer.app, http.MethodDelete, "/portfolios/10/comments/52"+q, nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "another portfolio's comment")
	resp = testutil.MakeRequest(viewer.app, http.MethodDelete, "/portfolios/10/comments/51"+q, nil)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	owner := newSharingFixture(t, 5)
	owner.commentRepo.On("FindByID", mock.Anything, uint(50), uint(1)).Return(&models.PortfolioComment{ID: 50, PortfolioID: 10, UserID: 8}, nil)
	owner.commentRepo.On("Delete", mock.Anything, uint(50)).Return(nil)
	resp = testutil.MakeRequest(owner.app, http.MethodDelete, "/portfolios/10/comments/50", nil)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestImportableSubmissionsAndImportWithFiles(t *testing.T) {
	f := newSharingFixture(t, 5)
	now := time.Now()
	earlier := now.Add(-time.Hour)
	files := `[{"id":91,"display_name":"lab.pdf","content_type":"application/pdf","size":1200}]`
	link := "javascript:alert(1)"
	f.submissionRepo.On("ListByUserAndCourse", mock.Anything, uint(5), uint(300)).Return([]models.Submission{
		{ID: 1, UserID: 5, AssignmentID: 40, SubmittedAt: &earlier, Attachments: &files, URL: &link},
		{ID: 2, UserID: 5, AssignmentID: 41, SubmittedAt: &now},
		{ID: 3, UserID: 5, AssignmentID: 40}, // not turned in
	}, nil)
	f.assignmentRepo.On("FindByID", mock.Anything, uint(40), uint(1)).Return(&models.Assignment{ID: 40, CourseID: 300, Name: "Lab report"}, nil)
	f.assignmentRepo.On("FindByID", mock.Anything, uint(41), uint(1)).Return(&models.Assignment{ID: 41, CourseID: 300, Name: "Essay"}, nil)

	resp := testutil.MakeRequest(f.app, http.MethodGet, "/portfolios/10/importable_submissions?course_id=300", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	list, err := testutil.ParseJSONArray(resp)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "Essay", list[0]["assignment_name"], "newest first")
	assert.Equal(t, float64(1), list[1]["file_count"])
	assert.Nil(t, list[1]["url"], "only web links are offered")

	var created []*models.PortfolioArtifact
	f.artifactRepo.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		created = append(created, args.Get(1).(*models.PortfolioArtifact))
	}).Return(nil)
	f.sectionRepo.On("FindByID", mock.Anything, uint(1), uint(1)).Return(&models.PortfolioSection{ID: 1, PortfolioID: 10}, nil)
	f.submissionRepo.On("FindByID", mock.Anything, uint(1), uint(1)).Return(&models.Submission{ID: 1, UserID: 5, AssignmentID: 40, SubmittedAt: &earlier, Attachments: &files, URL: &link}, nil)

	resp = testutil.MakeRequest(f.app, http.MethodPost, "/portfolios/10/import", body(`{"submission_ids":[1,1],"section_id":1}`))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Len(t, created, 2, "the course_work artifact plus one file artifact; a repeated id is imported once")
	assert.Equal(t, "course_submission", created[0].SourceType)
	assert.Empty(t, created[0].ContentURL, "a javascript: link is never copied")
	assert.Equal(t, "upload", created[1].SourceType)
	assert.Equal(t, uint(91), *created[1].SourceID)
	assert.Equal(t, "/api/v1/portfolios/10/files/91", created[1].ContentURL)
	assert.Equal(t, "pdf", created[1].FileType)
	for _, a := range created {
		require.NotNil(t, a.SectionID)
		assert.Equal(t, uint(1), *a.SectionID)
	}
}
