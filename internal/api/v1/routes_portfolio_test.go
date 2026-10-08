package v1

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// Route-level tests for the ePortfolio routes, run through the real
// Router.Register so registration order and group middleware are covered.
//
//   - B094: GET /portfolios/public/:slug must be reachable without a session
//     (it used to be mounted after the protected group's auth middleware).
//   - B201: PUT /portfolios/:id/sections/reorder must not be shadowed by
//     PUT /portfolios/:id/sections/:section_id.
func newPortfolioRouterApp(t *testing.T) (*fiber.App, *mocks.MockPortfolioRepository, *mocks.MockPortfolioSectionRepository) {
	t.Helper()
	portfolioRepo := new(mocks.MockPortfolioRepository)
	sectionRepo := new(mocks.MockPortfolioSectionRepository)
	artifactRepo := new(mocks.MockPortfolioArtifactRepository)
	reflectionRepo := new(mocks.MockPortfolioReflectionRepository)
	artifactRepo.On("ListByPortfolioID", mock.Anything, mock.Anything, mock.Anything).
		Return(&repository.PaginatedResult[models.PortfolioArtifact]{}, nil).Maybe()

	svc := service.NewPortfolioService(portfolioRepo, sectionRepo, artifactRepo, reflectionRepo,
		new(mocks.MockPortfolioTemplateRepository), new(mocks.MockPortfolioCommentRepository),
		new(mocks.MockSubmissionRepository), new(mocks.MockAssignmentRepository))
	authz := handlers.NewResourceAuthorizer(new(mocks.MockEnrollmentRepository), new(mocks.MockUserRepository))

	r := &Router{
		PortfolioHandler: handlers.NewPortfolioHandler(svc, authz),
		AuthMiddleware:   &middleware.AuthMiddleware{},
		// Register dereferences these while mounting their routes.
		SmartSearchHandler: handlers.NewSmartSearchHandler(nil, nil),
	}
	app := fiber.New()
	r.Register(app)
	return app, portfolioRepo, sectionRepo
}

func TestPortfolioRoutes_PublicViewNeedsNoSession(t *testing.T) {
	app, portfolioRepo, sectionRepo := newPortfolioRouterApp(t)
	portfolioRepo.On("FindBySlug", mock.Anything, "jane-doe-1").Return(&models.Portfolio{
		ID: 9, UserID: 5, Title: "Jane", Slug: "jane-doe-1",
		WorkflowState: models.PortfolioPublished, IsPublic: true,
	}, nil)
	portfolioRepo.On("IncrementViewCount", mock.Anything, uint(9)).Return(nil)
	sectionRepo.On("ListByPortfolioID", mock.Anything, uint(9)).Return([]models.PortfolioSection{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portfolios/public/jane-doe-1", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "anonymous visitors must reach the public portfolio")
}

func TestPortfolioRoutes_ReorderNotShadowedBySectionID(t *testing.T) {
	app, _, _ := newPortfolioRouterApp(t)
	// No session: the protected group answers 401 either way, so check the
	// route table instead — reorder must be registered before :section_id.
	var reorderIdx, sectionIdx = -1, -1
	for i, rt := range app.GetRoutes(true) {
		if rt.Method != http.MethodPut {
			continue
		}
		switch rt.Path {
		case "/api/v1/portfolios/:id/sections/reorder":
			reorderIdx = i
		case "/api/v1/portfolios/:id/sections/:section_id":
			sectionIdx = i
		}
	}
	require.NotEqual(t, -1, reorderIdx, "reorder route must be mounted")
	require.NotEqual(t, -1, sectionIdx)
	assert.Less(t, reorderIdx, sectionIdx, "reorder must be registered before :section_id")

	// And the unpublish/duplicate routes the UI calls exist.
	paths := map[string]bool{}
	for _, rt := range app.GetRoutes(true) {
		paths[rt.Method+" "+rt.Path] = true
	}
	for _, want := range []string{
		"DELETE /api/v1/portfolios/:id/publish",
		"POST /api/v1/portfolios/:id/duplicate",
		"POST /api/v1/portfolios/:id/artifacts/upload",
	} {
		assert.True(t, paths[want], strings.TrimSpace(want)+" must be mounted")
	}
}
