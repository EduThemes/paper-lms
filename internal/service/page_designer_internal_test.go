package service

import (
	"context"
	"strings"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// cleanPageDesign is what every non-API page write path (revision restore,
// Commons import, blueprint sync, course copy, batch clone) applies.
func TestCleanPageDesign(t *testing.T) {
	designed := func() *models.WikiPage {
		return &models.WikiPage{
			CSS: "p{color:red}", CSSSource: "p { color: red }",
			Design: models.NullJSON(`{"version":1,"id":"d","sections":[{"id":"w","type":"welcome","heading":"Hi","body":"<p>ok</p><img src=x onerror=alert(1)>"}]}`),
		}
	}
	plain := func() *models.WikiPage {
		return &models.WikiPage{CSS: "p{color:red}", CSSSource: "p { color: red }"}
	}

	p := designed()
	cleanPageDesign(p, false)
	if p.Design.IsNull() || strings.Contains(string(p.Design), "onerror") || !strings.Contains(string(p.Design), "<p>ok</p>") {
		t.Errorf("design not cleaned: %s", p.Design)
	}
	if p.CSS == "" || p.CSSSource == "" {
		t.Error("a designed page keeps its stylesheet with the setting off")
	}

	p = plain()
	cleanPageDesign(p, true)
	if p.CSS == "" || p.CSSSource == "" {
		t.Error("with the setting on a plain page keeps its stylesheet")
	}

	p = plain()
	cleanPageDesign(p, false)
	if p.CSS != "" || p.CSSSource != "" {
		t.Errorf("with the setting off a plain page keeps css %q / source %q", p.CSS, p.CSSSource)
	}

	// An invalid design is dropped, and the page is then a plain page.
	p = &models.WikiPage{CSS: "p{color:red}", Design: models.NullJSON(`{"version":2,"id":"d"}`)}
	cleanPageDesign(p, false)
	if !p.Design.IsNull() || p.CSS != "" {
		t.Errorf("invalid design kept (%s) or css kept (%q)", p.Design, p.CSS)
	}

	// A stylesheet the sanitizer refuses entirely takes its source with it.
	p = &models.WikiPage{CSS: "@import url(https://evil.example/x.css);", CSSSource: "@import url(https://evil.example/x.css);"}
	cleanPageDesign(p, true)
	if p.CSS != "" || p.CSSSource != "" {
		t.Errorf("refused css kept: %q / %q", p.CSS, p.CSSSource)
	}
}

func TestCourseCSSPolicyDefaults(t *testing.T) {
	var none CourseCSSPolicy
	if !none.Allowed(context.Background(), 1) {
		t.Error("a nil policy must mean the default (on)")
	}
	if !NewCourseCSSPolicy(nil, nil).Allowed(context.Background(), 1) {
		t.Error("no course repository: default (on)")
	}
}
