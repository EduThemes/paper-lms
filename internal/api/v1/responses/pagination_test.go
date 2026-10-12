package responses_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
)

func linkRels(t *testing.T, header string) map[string]string {
	t.Helper()
	out := map[string]string{}
	re := regexp.MustCompile(`<([^>]+)>;\s*rel="([^"]+)"`)
	for _, m := range re.FindAllStringSubmatch(header, -1) {
		out[m[2]] = m[1]
	}
	return out
}

// B189: Link headers must keep the request's filter/include parameters so a
// client following rel="next" keeps getting the filtered set.
func TestSetPaginationHeaders_PreservesQueryParams(t *testing.T) {
	app := fiber.New()
	app.Get("/api/v1/courses/:id/enrollments", func(c *fiber.Ctx) error {
		responses.SetPaginationHeaders(c, 120, 1, 50)
		return c.SendStatus(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/courses/1/enrollments?type[]=StudentEnrollment&type[]=TaEnrollment&state[]=active&per_page=50&page=1", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	rels := linkRels(t, resp.Header.Get("Link"))

	next, ok := rels["next"]
	if !ok {
		t.Fatalf("expected rel=next in %q", resp.Header.Get("Link"))
	}
	u, err := url.Parse(next)
	if err != nil {
		t.Fatalf("parse next link: %v", err)
	}
	q := u.Query()
	if got := q["type[]"]; len(got) != 2 || got[0] != "StudentEnrollment" || got[1] != "TaEnrollment" {
		t.Errorf("next link lost type[] filters: %v (link %s)", got, next)
	}
	if q.Get("state[]") != "active" {
		t.Errorf("next link lost state[] filter (link %s)", next)
	}
	if q.Get("page") != "2" || q.Get("per_page") != "50" {
		t.Errorf("next link page/per_page = %s/%s, want 2/50", q.Get("page"), q.Get("per_page"))
	}
	if len(q["page"]) != 1 || len(q["per_page"]) != 1 {
		t.Errorf("page/per_page must not be duplicated: %s", next)
	}
	last, _ := url.Parse(rels["last"])
	if last.Query().Get("page") != "3" {
		t.Errorf("last page = %s, want 3", last.Query().Get("page"))
	}
	if _, ok := rels["prev"]; ok {
		t.Errorf("page 1 must not have rel=prev")
	}
}

func TestSetPaginationHeaders_NoQueryParams(t *testing.T) {
	app := fiber.New()
	app.Get("/items", func(c *fiber.Ctx) error {
		responses.SetPaginationHeaders(c, 5, 1, 10)
		return c.SendStatus(http.StatusOK)
	})
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/items", nil), -1)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	rels := linkRels(t, resp.Header.Get("Link"))
	cur, _ := url.Parse(rels["current"])
	if cur.RawQuery != "page=1&per_page=10" {
		t.Errorf("current link query = %q, want page=1&per_page=10", cur.RawQuery)
	}
	if _, ok := rels["next"]; ok {
		t.Errorf("single page must not have rel=next")
	}
}
