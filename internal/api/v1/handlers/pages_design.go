package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/designdoc"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/security/csssanitize"
	"github.com/EduThemes/paper-lms/internal/service"
)

// Page Designer additions to the page API (docs/page-designer/README.md):
// wiki_page[css] (the page stylesheet) and wiki_page[design] (the PageDoc).

// customCSSEnabled resolves pages.custom_css_enabled (see
// service.CustomCSSEnabled).
func customCSSEnabled(ctx context.Context, lookup service.SettingsLookupFunc, accountID uint) bool {
	return service.CustomCSSEnabled(ctx, lookup, accountID)
}

func unprocessable(c *fiber.Ctx, msg string) error {
	return responses.Error(c, fiber.StatusUnprocessableEntity, msg)
}

// applyPageDesignInput validates wiki_page[css] / wiki_page[design] from
// a create or update request and applies them to page. css nil and an
// empty design mean "not sent". Returns wrote=true after writing an error
// response; the caller must then return err as-is.
//
//   - Only course staff (who can manage course content) may send either
//     field; a student editor gets 403 even for an empty value.
//   - css goes through csssanitize.Sanitize (stored unscoped).
//   - design must be a JSON object with version 1 and a string id, at
//     most 256 KB; null clears it (the page becomes an ordinary page).
//   - pages.custom_css_enabled is resolved for the page's course (its
//     account's setting, the value course show reports), not the
//     caller's account. With it off, a non-empty css is accepted only with
//     a non-null design in the same request, and a request that detaches
//     the design (design: null) also clears the stored stylesheet and its
//     source: without a design the page may not carry hand-written CSS.
//     With it on, a request without css leaves the stored stylesheet
//     alone, so "Edit as HTML" keeps the page's look.
func (h *PageHandler) applyPageDesignInput(c *fiber.Ctx, page *models.WikiPage, css *string, design json.RawMessage) (bool, error) {
	if css == nil && len(design) == 0 {
		return false, nil
	}
	if !canViewUnpublishedContent(c) {
		return true, responses.Forbidden(c, "Only course staff can set a page's stylesheet or design")
	}

	designSent := len(design) > 0
	var newDesign models.NullJSON
	if designSent && !models.NullJSON(design).IsNull() {
		normalized, err := designdoc.ValidateDesign(design)
		if err != nil {
			return true, unprocessable(c, err.Error())
		}
		newDesign = normalized
	}

	var newCSS string
	if css != nil && strings.TrimSpace(*css) != "" {
		sanitized, err := csssanitize.Sanitize(*css)
		if errors.Is(err, csssanitize.ErrTooLarge) {
			return true, unprocessable(c, "The page stylesheet is larger than 200 KB")
		}
		if err != nil {
			return true, unprocessable(c, "The page stylesheet could not be read")
		}
		newCSS = sanitized
	}

	enabled := h.cssPolicy.Allowed(c.Context(), page.CourseID)
	if !enabled && css != nil && strings.TrimSpace(*css) != "" && (!designSent || newDesign.IsNull()) {
		return true, unprocessable(c, "Custom CSS is turned off for this account: a page stylesheet can only be saved together with a Page Designer design")
	}

	if designSent {
		page.Design = newDesign
	}
	if css != nil {
		page.CSS = newCSS
		page.CSSSource = ""
		if newCSS != "" {
			page.CSSSource = *css
		}
	}
	if !enabled && designSent && newDesign.IsNull() {
		// Detached with hand-written CSS turned off: no design, no CSS.
		page.CSS = ""
		page.CSSSource = ""
	}
	return false, nil
}
