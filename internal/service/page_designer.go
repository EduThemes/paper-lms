package service

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"reflect"
	"strconv"
	"strings"

	"github.com/EduThemes/paper-lms/internal/designdoc"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/security/csssanitize"
	"github.com/EduThemes/paper-lms/internal/settingsctx"
)

// PagesCustomCSSEnabledKey is the Settings Engine key for the page
// stylesheet switch (declared in internal/service/settings/catalog.go;
// service cannot import that package — settings → auth → service).
const PagesCustomCSSEnabledKey = "pages.custom_css_enabled"

// CustomCSSEnabled resolves pages.custom_css_enabled for accountID. The
// key is "off anywhere above wins" (settings.Definition.OffLocksBelow):
// the Settings Engine reports false when the instance or any account in
// accountID's parent chain, the account itself included, has it off, so
// a sub-account can narrow its parent's choice but never widen it.
//
// A nil lookup, a lookup error or an unparsable value fall back to the
// catalog default (on): the setting only narrows what teachers may do, so
// failing open keeps the behaviour pages had before the setting existed.
// Errors are logged, since failing open hides them otherwise.
func CustomCSSEnabled(ctx context.Context, lookup SettingsLookupFunc, accountID uint) bool {
	if lookup == nil {
		return true
	}
	v, err := lookup(settingsctx.WithAccountID(ctx, accountID), PagesCustomCSSEnabledKey)
	if err != nil {
		slog.Warn("pages.custom_css_enabled lookup failed; allowing page CSS", "account_id", accountID, "error", err)
		return true
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		slog.Warn("pages.custom_css_enabled has an unparsable value; allowing page CSS", "account_id", accountID, "value", v)
		return true
	}
	return b
}

// CourseCSSPolicy reports whether pages.custom_css_enabled is on for the
// account that owns a course. The setting is the course's account's, not
// the caller's: an admin of a parent account editing a sub-account's course
// gets that sub-account's rule, the same value course show reports. A nil
// policy means the setting's default (on).
type CourseCSSPolicy func(ctx context.Context, courseID uint) bool

// NewCourseCSSPolicy resolves the setting through the course's account. A
// course that can't be loaded falls back to the default (on), like
// CustomCSSEnabled does for a failed lookup.
func NewCourseCSSPolicy(courses repository.CourseRepository, lookup SettingsLookupFunc) CourseCSSPolicy {
	return func(ctx context.Context, courseID uint) bool {
		if courses == nil {
			return true
		}
		// accountID=0: auth-internal — the caller already tied courseID to
		// its tenant (a page it loaded, or a copy/import target it checked).
		course, err := courses.FindByID(ctx, courseID, 0)
		if err != nil || course == nil {
			return true
		}
		return CustomCSSEnabled(ctx, lookup, course.AccountID)
	}
}

// Allowed reports whether hand-written page CSS is allowed in courseID.
func (p CourseCSSPolicy) Allowed(ctx context.Context, courseID uint) bool {
	if p == nil {
		return true
	}
	return p(ctx, courseID)
}

// cleanPageDesign re-checks a page's design and stylesheet before a write
// path that did not come through the page API stores them (revision
// restore, Commons import, blueprint sync, course copy, batch clone): the
// design is cleaned by designdoc.ValidateDesign (dropped if invalid), the
// stylesheet re-sanitized, and when cssAllowed is false (the destination
// course's account has pages.custom_css_enabled off) a page without a
// design loses its stylesheet, since only designer CSS is allowed there.
// css_source is cleared whenever the stylesheet is.
func cleanPageDesign(p *models.WikiPage, cssAllowed bool) {
	p.Design = revalidateDesign(p.Design)
	p.CSS = resanitizePageCSS(p.CSS)
	if !cssAllowed && p.Design.IsNull() {
		p.CSS = ""
	}
	if p.CSS == "" {
		p.CSSSource = ""
	}
}

// resanitizePageCSS runs a stored or imported page stylesheet through the
// sanitizer again (it is idempotent); anything it refuses is dropped.
func resanitizePageCSS(css string) string {
	if strings.TrimSpace(css) == "" {
		return ""
	}
	out, err := csssanitize.Resanitize(css)
	if err != nil {
		return ""
	}
	return out
}

// revalidateDesign returns d cleaned by designdoc.ValidateDesign, or nil
// when it is not a valid page design.
func revalidateDesign(d models.NullJSON) models.NullJSON {
	if d.IsNull() {
		return nil
	}
	out, err := designdoc.ValidateDesign(d)
	if err != nil {
		return nil
	}
	return out
}

// jsonValuesEqual compares two JSON documents as values: jsonb hands
// documents back re-formatted (key order, spacing), so bytes differ.
func jsonValuesEqual(a, b models.NullJSON) bool {
	if a.IsNull() || b.IsNull() {
		return a.IsNull() == b.IsNull()
	}
	if bytes.Equal(a, b) {
		return true
	}
	var va, vb interface{}
	if json.Unmarshal(a, &va) != nil || json.Unmarshal(b, &vb) != nil {
		return false
	}
	return reflect.DeepEqual(va, vb)
}
