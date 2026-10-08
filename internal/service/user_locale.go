package service

import (
	"errors"
	"regexp"
	"strings"
)

// SupportedUILocales are the languages the web UI ships translations for
// (web/src/utils/localePreference.js SUPPORTED_LOCALES). A user's saved
// locale must be one of them, optionally with a region (es-MX).
var SupportedUILocales = map[string]bool{"en": true, "es": true}

// ErrUnsupportedLocale is returned for a locale the UI cannot display.
var ErrUnsupportedLocale = errors.New("unsupported locale")

var localeTag = regexp.MustCompile(`^([A-Za-z]{2,3})(?:[-_]([A-Za-z]{2}|[0-9]{3}))?$`)

// NormalizeUserLocale validates a per-user language preference (G158).
// "" means "no explicit choice" (fall back to the device, then the account
// default). Anything else must be a supported language, optionally with a
// region; it is returned canonicalized ("es_mx" -> "es-MX").
func NormalizeUserLocale(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	m := localeTag.FindStringSubmatch(raw)
	if m == nil {
		return "", ErrUnsupportedLocale
	}
	lang := strings.ToLower(m[1])
	if !SupportedUILocales[lang] {
		return "", ErrUnsupportedLocale
	}
	if m[2] == "" {
		return lang, nil
	}
	return lang + "-" + strings.ToUpper(m[2]), nil
}
