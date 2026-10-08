package handlers_test

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// B005: portfolio section content is typed in a textarea and rendered as text
// by the portfolio pages. Running it through the HTML sanitizer stored
// "Don&#39;t forget A &amp; B", which the public page showed literally. It
// must be stored as the literal text, still with no markup (the exported
// portfolio site embeds it raw).
func TestAddPortfolioSection_StoresPlainTextLiterally(t *testing.T) {
	portfolioRepo := new(mocks.MockPortfolioRepository)
	sectionRepo := new(mocks.MockPortfolioSectionRepository)
	svc := service.NewPortfolioService(portfolioRepo, sectionRepo, nil, nil, nil, nil, nil, nil)
	h := handlers.NewPortfolioHandler(svc, handlers.NewResourceAuthorizer(nil, nil))

	portfolioRepo.On("FindByID", mock.Anything, uint(7), uint(1)).
		Return(&models.Portfolio{ID: 7, UserID: 10}, nil)
	sectionRepo.On("ListByPortfolioID", mock.Anything, uint(7)).Return([]models.PortfolioSection{}, nil)
	var stored *models.PortfolioSection
	sectionRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.PortfolioSection")).
		Run(func(args mock.Arguments) { stored = args.Get(1).(*models.PortfolioSection) }).
		Return(nil)

	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(10))
		c.Locals("account_id", uint(1))
		return c.Next()
	})
	app.Post("/portfolios/:id/sections", h.AddSection)

	body := testutil.JSONBody(map[string]interface{}{
		"section": map[string]interface{}{
			"title":        "About me",
			"section_type": "about",
			"content":      "Don't forget A & B\n<img src=x onerror=alert(1)>x < y",
		},
	})
	resp := testutil.MakeRequest(app, http.MethodPost, "/portfolios/7/sections", body)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.NotNil(t, stored)
	assert.Equal(t, "Don't forget A & B\nx < y", stored.Content)
}
