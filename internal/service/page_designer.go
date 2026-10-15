package service

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"

	"github.com/EduThemes/paper-lms/internal/designdoc"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/security/csssanitize"
	"github.com/EduThemes/paper-lms/internal/settingsctx"
)

// PagesCustomCSSEnabledKey is the Settings Engine key for the page
// stylesheet switch (declared in internal/service/settings/catalog.go;
// service cannot import that package — settings → auth → service).
const PagesCustomCSSEnabledKey = "pages.custom_css_enabled"

// CustomCSSEnabled resolves pages.custom_css_enabled for accountID. A nil
// lookup, a lookup error or an unparsable value fall back to the catalog
// default (on): the setting only narrows what teachers may do, so failing
// open keeps the behaviour pages had before the setting existed.
func CustomCSSEnabled(ctx context.Context, lookup SettingsLookupFunc, accountID uint) bool {
	if lookup == nil {
		return true
	}
	v, err := lookup(settingsctx.WithAccountID(ctx, accountID), PagesCustomCSSEnabledKey)
	if err != nil {
		return true
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return true
	}
	return b
}

// resanitizePageCSS runs a stored or imported page stylesheet through the
// sanitizer again (it is idempotent); anything it refuses is dropped.
func resanitizePageCSS(css string) string {
	if strings.TrimSpace(css) == "" {
		return ""
	}
	out, err := csssanitize.Sanitize(css)
	if err != nil {
		return ""
	}
	return out
}

// revalidateDesign returns d when it is a valid page design (compacted),
// or nil.
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
