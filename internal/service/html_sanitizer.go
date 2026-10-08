// Package service — html_sanitizer.go
//
// Defense-in-depth server-side HTML sanitization for user-generated content.
//
// HTML originating from rich text editors is sanitized at write time before
// being persisted to the database. The frontend additionally runs DOMPurify on
// render — this is the second layer of defense, mirroring what Canvas does
// with the canvas_sanitize Ruby gem.
//
// The policy itself lives in internal/security/htmlsanitize so the domain
// models can apply it from BeforeSave hooks (covering imports, blueprint
// syncs, Commons and question-bank writes that never pass through a handler).
package service

import "github.com/EduThemes/paper-lms/internal/security/htmlsanitize"

// SanitizeHTML cleans untrusted HTML before it is persisted. It is safe to
// call on already-sanitized HTML (idempotent). It entity-encodes text, so it
// must only wrap rich-content (HTML) fields — never plain-text fields that the
// client renders as text (doing so shows "Don&#39;t" to the user).
//
// An empty input returns an empty string with no allocation.
func SanitizeHTML(input string) string {
	return htmlsanitize.Sanitize(input)
}

// SanitizePlainText is the sanitizer for plain-text fields (descriptions and
// messages typed into a textarea and rendered by the client as text). It
// strips all markup but keeps the text literal — "Don't forget A & B" is
// stored as typed instead of "Don&#39;t forget A &amp; B" (B005). See
// htmlsanitize.PlainText.
func SanitizePlainText(input string) string {
	return htmlsanitize.PlainText(input)
}
