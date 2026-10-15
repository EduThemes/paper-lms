// Package csssanitize cleans and scopes page stylesheets (wiki_pages.css).
// See docs/page-designer/README.md, "Page stylesheet rules".
//
// This is a placeholder with the final signatures: Sanitize keeps nothing
// until the real parser lands, so no unvetted CSS can be stored.
package csssanitize

import "errors"

// MaxBytes is the largest stylesheet Sanitize accepts.
const MaxBytes = 200 * 1024

// ErrTooLarge is returned by Sanitize when the input exceeds MaxBytes.
var ErrTooLarge = errors.New("stylesheet is larger than 200 KB")

// Sanitize parses author CSS and returns it normalized, keeping only what is
// allowed. It is idempotent.
func Sanitize(css string) (string, error) {
	if len(css) > MaxBytes {
		return "", ErrTooLarge
	}
	return "", nil
}

// Scope rewrites sanitized CSS so it applies only inside root
// (e.g. "#paper-page-42").
func Scope(css, root string) string {
	_ = root
	return ""
}
