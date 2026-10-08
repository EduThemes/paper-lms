package handlers_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

func publicPortfolio12() *models.Portfolio {
	return &models.Portfolio{
		ID: 12, UserID: 5, Title: "Public", Slug: "pub-1", WorkflowState: models.PortfolioPublished, IsPublic: true,
	}
}

// A tenant user who is not the owner reads a published, public portfolio
// through GET /portfolios/:id: they must get the public view (hidden
// sections and their artifacts removed), not the owner's editor view.
func TestPortfolio_GetPortfolio_NonOwnerGetsPublicView(t *testing.T) {
	hiddenID, attachID := uint(2), uint(55)
	sections := []models.PortfolioSection{
		{ID: 1, PortfolioID: 12, Title: "Shown", SectionType: "about", IsVisible: true},
		{ID: 2, PortfolioID: 12, Title: "Private notes", SectionType: "custom", IsVisible: false},
	}
	artifacts := &repository.PaginatedResult[models.PortfolioArtifact]{
		Items: []models.PortfolioArtifact{
			{ID: 1, PortfolioID: 12, Title: "loose", ArtifactType: "document", SourceType: "upload", SourceID: &attachID, ContentURL: "/api/v1/portfolios/12/files/55"},
			{ID: 2, PortfolioID: 12, SectionID: &hiddenID, Title: "in hidden", ArtifactType: "link"},
		},
	}
	setup := func(callerID uint) *portfolioFixture {
		f := newPortfolioFixture(t, callerID)
		f.portfolioRepo.On("FindByID", mock.Anything, uint(12), uint(1)).Return(publicPortfolio12(), nil)
		f.sectionRepo.On("ListByPortfolioID", mock.Anything, uint(12)).Return(sections, nil)
		f.artifactRepo.On("ListByPortfolioID", mock.Anything, uint(12), mock.Anything).Return(artifacts, nil)
		f.reflectionRepo.On("ListByArtifactID", mock.Anything, mock.Anything).Return([]models.PortfolioReflection{}, nil)
		return f
	}

	viewer := setup(7)
	resp := testutil.MakeRequest(viewer.app, http.MethodGet, "/portfolios/12", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	m, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	secs := m["sections"].([]interface{})
	require.Len(t, secs, 1, "hidden sections must not reach non-owners")
	assert.Equal(t, "Shown", secs[0].(map[string]interface{})["title"])
	arts := m["artifacts"].([]interface{})
	require.Len(t, arts, 1, "artifacts of hidden sections must not reach non-owners")
	assert.Equal(t, "/api/v1/portfolios/public/pub-1/files/55", arts[0].(map[string]interface{})["content_url"])

	// The owner still sees everything.
	owner := setup(5)
	resp = testutil.MakeRequest(owner.app, http.MethodGet, "/portfolios/12", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	m, err = testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	assert.Len(t, m["sections"], 2)
	assert.Len(t, m["artifacts"], 2)
}

// Public viewers must not be able to download a file that only backs an
// artifact in a hidden section; the owner can.
func TestPortfolioHasUpload_HiddenSectionFilesArePrivate(t *testing.T) {
	sectionRepo := new(mocks.MockPortfolioSectionRepository)
	artifactRepo := new(mocks.MockPortfolioArtifactRepository)
	hiddenID, shownID := uint(2), uint(1)
	hiddenFile, shownFile := uint(55), uint(56)
	sectionRepo.On("ListByPortfolioID", mock.Anything, uint(9)).Return([]models.PortfolioSection{
		{ID: shownID, PortfolioID: 9, IsVisible: true},
		{ID: hiddenID, PortfolioID: 9, IsVisible: false},
	}, nil)
	artifactRepo.On("ListByPortfolioID", mock.Anything, uint(9), mock.Anything).Return(&repository.PaginatedResult[models.PortfolioArtifact]{
		Items: []models.PortfolioArtifact{
			{ID: 1, PortfolioID: 9, SectionID: &hiddenID, SourceType: "upload", SourceID: &hiddenFile},
			{ID: 2, PortfolioID: 9, SectionID: &shownID, SourceType: "upload", SourceID: &shownFile},
		},
	}, nil)
	svc := service.NewPortfolioService(new(mocks.MockPortfolioRepository), sectionRepo, artifactRepo,
		new(mocks.MockPortfolioReflectionRepository), new(mocks.MockPortfolioTemplateRepository),
		new(mocks.MockPortfolioCommentRepository), new(mocks.MockSubmissionRepository), new(mocks.MockAssignmentRepository))
	ctx := context.Background()

	assert.False(t, svc.PortfolioHasUpload(ctx, 9, hiddenFile, true), "public viewer, hidden section")
	assert.True(t, svc.PortfolioHasUpload(ctx, 9, shownFile, true), "public viewer, visible section")
	assert.True(t, svc.PortfolioHasUpload(ctx, 9, hiddenFile, false), "owner sees hidden-section files")
	assert.False(t, svc.PortfolioHasUpload(ctx, 9, 999, false), "unrelated attachment")
}

// Duplicating a portfolio must point uploaded-file artifacts at the copy,
// not at the original portfolio's file route.
func TestPortfolio_DuplicateRewritesUploadLinks(t *testing.T) {
	portfolioRepo := new(mocks.MockPortfolioRepository)
	sectionRepo := new(mocks.MockPortfolioSectionRepository)
	artifactRepo := new(mocks.MockPortfolioArtifactRepository)
	reflectionRepo := new(mocks.MockPortfolioReflectionRepository)
	portfolioRepo.On("FindByID", mock.Anything, uint(10), uint(1)).Return(&models.Portfolio{
		ID: 10, UserID: 5, Title: "Mine", Slug: "mine-1", WorkflowState: models.PortfolioPublished, IsPublic: true,
	}, nil)
	portfolioRepo.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		args.Get(1).(*models.Portfolio).ID = 20
	}).Return(nil)
	sectionRepo.On("ListByPortfolioID", mock.Anything, uint(10)).Return([]models.PortfolioSection{}, nil)
	attachID := uint(55)
	artifactRepo.On("ListByPortfolioID", mock.Anything, uint(10), mock.Anything).Return(&repository.PaginatedResult[models.PortfolioArtifact]{
		Items: []models.PortfolioArtifact{
			{ID: 1, PortfolioID: 10, Title: "file", ArtifactType: "document", SourceType: "upload", SourceID: &attachID, ContentURL: "/api/v1/portfolios/10/files/55"},
			{ID: 2, PortfolioID: 10, Title: "link", ArtifactType: "link", ContentURL: "https://example.com"},
		},
	}, nil)
	reflectionRepo.On("ListByArtifactID", mock.Anything, mock.Anything).Return([]models.PortfolioReflection{}, nil)
	var created []models.PortfolioArtifact
	artifactRepo.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		created = append(created, *args.Get(1).(*models.PortfolioArtifact))
	}).Return(nil)

	svc := service.NewPortfolioService(portfolioRepo, sectionRepo, artifactRepo, reflectionRepo,
		new(mocks.MockPortfolioTemplateRepository), new(mocks.MockPortfolioCommentRepository),
		new(mocks.MockSubmissionRepository), new(mocks.MockAssignmentRepository))
	dup, err := svc.DuplicatePortfolio(context.Background(), 10, 1)
	require.NoError(t, err)
	assert.Equal(t, uint(20), dup.ID)
	assert.Equal(t, models.PortfolioDraft, dup.WorkflowState)
	require.Len(t, created, 2)
	assert.Equal(t, uint(20), created[0].PortfolioID)
	assert.Equal(t, "/api/v1/portfolios/20/files/55", created[0].ContentURL)
	assert.Equal(t, "https://example.com", created[1].ContentURL)
}
