package testutil

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/gofiber/fiber/v2"
)

// SetupTestApp creates a Fiber app for testing with no unnecessary middleware.
func SetupTestApp() *fiber.App {
	return fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"errors": []fiber.Map{{"message": err.Error()}},
			})
		},
	})
}

// MakeRequest makes a request to the test app and returns the response.
func MakeRequest(app *fiber.App, method, path string, body io.Reader) *http.Response {
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	return resp
}

// MakeAuthenticatedRequest makes a request with a JWT bearer token.
func MakeAuthenticatedRequest(app *fiber.App, method, path, token string, body io.Reader) *http.Response {
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ := app.Test(req, -1)
	return resp
}

// MakeAuthenticatedCookieRequest makes a request with a session cookie.
func MakeAuthenticatedCookieRequest(app *fiber.App, method, path, token string, body io.Reader) *http.Response {
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "paper_session", Value: token})
	resp, _ := app.Test(req, -1)
	return resp
}

// JSONBody converts a value to a JSON reader for request bodies.
func JSONBody(v interface{}) io.Reader {
	data, _ := json.Marshal(v)
	return bytes.NewReader(data)
}

// ParseJSONResponse reads and parses the response body into the target.
func ParseJSONResponse(resp *http.Response, target interface{}) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(target)
}

// ParseJSONMap reads and parses the response body as a map.
func ParseJSONMap(resp *http.Response) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := ParseJSONResponse(resp, &result)
	return result, err
}

// ParseJSONArray reads and parses the response body as an array.
func ParseJSONArray(resp *http.Response) ([]map[string]interface{}, error) {
	var result []map[string]interface{}
	err := ParseJSONResponse(resp, &result)
	return result, err
}
