package middleware_test

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/stretchr/testify/assert"
)

// setupPaginationApp creates a Fiber app that applies the PaginationParams
// middleware and returns the resolved pagination values in the response body.
func setupPaginationApp() *fiber.App {
	app := testutil.SetupTestApp()

	app.Get("/items", middleware.PaginationParams(), func(c *fiber.Ctx) error {
		p := middleware.GetPagination(c)
		return c.JSON(fiber.Map{
			"page":     p.Page,
			"per_page": p.PerPage,
		})
	})

	return app
}

// parsePagination is a small helper that extracts page and per_page from the
// JSON response body as integers.
func parsePagination(t *testing.T, resp *http.Response) (int, int) {
	t.Helper()
	body, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)
	page := int(body["page"].(float64))
	perPage := int(body["per_page"].(float64))
	return page, perPage
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestPagination_Defaults(t *testing.T) {
	app := setupPaginationApp()

	resp := testutil.MakeRequest(app, http.MethodGet, "/items", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	page, perPage := parsePagination(t, resp)
	assert.Equal(t, 1, page)
	assert.Equal(t, repository.PaginationParams{Page: 1, PerPage: 10},
		repository.PaginationParams{Page: page, PerPage: perPage})
}

func TestPagination_CustomValues(t *testing.T) {
	app := setupPaginationApp()

	resp := testutil.MakeRequest(app, http.MethodGet, "/items?page=3&per_page=25", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	page, perPage := parsePagination(t, resp)
	assert.Equal(t, 3, page)
	assert.Equal(t, 25, perPage)
}

func TestPagination_MaxPerPage(t *testing.T) {
	app := setupPaginationApp()

	resp := testutil.MakeRequest(app, http.MethodGet, "/items?per_page=200", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	_, perPage := parsePagination(t, resp)
	assert.Equal(t, 100, perPage, "per_page should be capped at MaxPerPage (100)")
}

func TestPagination_NegativePage(t *testing.T) {
	app := setupPaginationApp()

	resp := testutil.MakeRequest(app, http.MethodGet, "/items?page=-1", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	page, _ := parsePagination(t, resp)
	assert.Equal(t, 1, page, "negative page should be clamped to 1")
}

func TestPagination_ZeroPerPage(t *testing.T) {
	app := setupPaginationApp()

	resp := testutil.MakeRequest(app, http.MethodGet, "/items?per_page=0", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	_, perPage := parsePagination(t, resp)
	assert.Equal(t, 10, perPage, "per_page=0 should fall back to the default (10)")
}
