// Package service — html_sanitizer.go
//
// Defense-in-depth server-side HTML sanitization for user-generated content.
//
// HTML originating from rich text editors (RichContentEditor / TinyMCE-style
// inputs) is sanitized at write time before being persisted to the database.
// The frontend additionally runs DOMPurify on render — this file provides the
// second layer of defense, mirroring what Canvas does with the canvas_sanitize
// Ruby gem.
//
// Library: github.com/microcosm-cc/bluemonday (the Go equivalent of OWASP's
// AntiSamy / Ruby's sanitize gem).
package service

import (
	"regexp"
	"sync"

	"github.com/microcosm-cc/bluemonday"
)

// trustedIframeHostsRegex matches src URLs of iframes we permit for embedded
// media. Hosts are matched as the full host portion of an https URL.
//
// Allowed:
//   - YouTube: youtube.com, www.youtube.com, youtube-nocookie.com, youtu.be
//   - Vimeo:   player.vimeo.com, vimeo.com
//   - Khan Academy, Edpuzzle, H5P, Loom (common K-12 embeds)
//
// Anything not matching this regex is stripped from <iframe src="...">,
// effectively blocking the iframe (bluemonday drops the element when no
// allowed src remains).
var trustedIframeHostsRegex = regexp.MustCompile(
	`^https://(` +
		`(www\.)?youtube(-nocookie)?\.com/embed/|` +
		`youtu\.be/|` +
		`player\.vimeo\.com/video/|` +
		`(www\.)?vimeo\.com/|` +
		`(www\.)?khanacademy\.org/embed/|` +
		`(www\.)?edpuzzle\.com/embed/|` +
		`h5p\.org/h5p/embed/|` +
		`(www\.)?loom\.com/embed/` +
		`)`,
)

var (
	htmlPolicy     *bluemonday.Policy
	htmlPolicyOnce sync.Once
)

// buildPolicy constructs the shared bluemonday policy used for sanitization.
//
// Base: UGCPolicy (User-Generated Content) — already allows standard formatting
// elements (p, b, i, ul, ol, li, blockquote, code, pre, h1-h6, a, img, table,
// thead, tbody, tr, td, th, etc.), allows standard URL schemes, requires
// rel="nofollow" on links, and blocks scripts, styles, on*= handlers, etc.
//
// Additions:
//   - class attribute on common elements (KaTeX math classes, code highlighting,
//     callout boxes, etc.)
//   - data-* attributes globally (KaTeX uses data-mathml; many editors use
//     data-* for soft markers).
//   - <iframe> with a strict whitelist of trusted media hosts (YouTube, Vimeo,
//     Khan Academy, Edpuzzle, H5P, Loom). https:// only.
func buildPolicy() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()

	// Allow class on commonly-styled elements (syntax highlighting, KaTeX, etc.)
	p.AllowAttrs("class").OnElements(
		"span", "div", "p", "a", "code", "pre",
		"table", "tr", "td", "th",
		"ol", "ul", "li",
		"h1", "h2", "h3", "h4", "h5", "h6",
		"blockquote", "img",
	)

	// Allow data-* attributes globally — KaTeX (data-mathml), tooltip libs, etc.
	p.AllowDataAttributes()

	// Allow <iframe> with whitelisted src hosts only.
	// bluemonday treats the iframe element as allowed once we register
	// any attribute on it; the URL scheme + src regex restrict where it
	// can point.
	p.AllowElements("iframe")
	p.AllowAttrs("src").Matching(trustedIframeHostsRegex).OnElements("iframe")
	p.AllowAttrs("width", "height").Matching(regexp.MustCompile(`^[0-9]+$`)).OnElements("iframe")
	p.AllowAttrs("allowfullscreen").OnElements("iframe")
	p.AllowAttrs("frameborder").Matching(regexp.MustCompile(`^[0-9]+$`)).OnElements("iframe")
	p.AllowAttrs("title").OnElements("iframe")
	// Note: scheme restriction for iframe src is enforced by the
	// trustedIframeHostsRegex above (which requires "^https://").

	return p
}

// SanitizeHTML cleans untrusted HTML before it is persisted. It is safe to
// call on already-sanitized HTML (idempotent) and on plain text (returned
// unchanged aside from HTML entity escaping of any < / > / & characters
// that were not part of valid markup).
//
// An empty input returns an empty string with no allocation.
func SanitizeHTML(input string) string {
	if input == "" {
		return ""
	}
	htmlPolicyOnce.Do(func() {
		htmlPolicy = buildPolicy()
	})
	return htmlPolicy.Sanitize(input)
}
