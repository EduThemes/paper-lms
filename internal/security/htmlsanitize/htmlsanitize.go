// Package htmlsanitize holds the single server-side HTML sanitization policy
// for user-generated rich content (page bodies, assignment/quiz descriptions,
// question text, discussion and announcement messages, syllabi, submission
// bodies, ...).
//
// It is a leaf package (it depends only on bluemonday) so that both the
// service layer (service.SanitizeHTML, used by handlers on Create AND Update)
// and the domain models (BeforeSave hooks that make every write path —
// imports, blueprint syncs, Commons, question banks — store sanitized HTML)
// can share one policy without an import cycle.
//
// The policy mirrors Canvas's canvas_sanitize gem
// (gems/canvas_sanitize/lib/canvas_sanitize/canvas_sanitize.rb): standard UGC
// markup, class + data-* attributes, a safe subset of inline CSS (so editor
// formatting such as text alignment and highlight colour survives a save),
// link targets, HTML5 media, and iframes restricted to trusted media hosts.
package htmlsanitize

import (
	"html"
	"regexp"
	"sync"

	"github.com/microcosm-cc/bluemonday"
)

// TrustedIframeSrc matches src URLs of iframes we permit for embedded media.
// Hosts are matched as the full host portion of an https URL.
//
// Allowed:
//   - YouTube: youtube.com, www.youtube.com, youtube-nocookie.com, youtu.be
//   - Vimeo:   player.vimeo.com, vimeo.com
//   - Khan Academy, Edpuzzle, H5P, Loom (common K-12 embeds)
//
// Anything not matching this regex is stripped from <iframe src="...">,
// effectively blocking the iframe. The web client mirrors this list in
// web/src/components/RichContentViewer.jsx (TRUSTED_IFRAME_SRC).
var TrustedIframeSrc = regexp.MustCompile(
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

// allowedStyleProperties is the inline-CSS allowlist. It is a deliberately
// conservative subset of Canvas's list: typography, colour, box model and
// alignment. Properties that let content escape its container or overlay the
// application chrome (position, top/left/right/bottom, z-index, visibility)
// and anything that can fetch a URL (background-image, list-style-image) are
// NOT allowed. Each property's value is validated by bluemonday's per-property
// default handler.
var allowedStyleProperties = []string{
	"text-align", "text-decoration", "text-indent", "vertical-align",
	"color", "background-color",
	"font-weight", "font-style", "font-size", "font-family", "line-height",
	"margin", "margin-top", "margin-right", "margin-bottom", "margin-left",
	"padding", "padding-top", "padding-right", "padding-bottom", "padding-left",
	"border", "border-width", "border-style", "border-color", "border-collapse",
	"border-top", "border-right", "border-bottom", "border-left",
	"border-radius", "border-spacing",
	"width", "height", "max-width", "max-height", "min-width", "min-height",
	"float", "clear", "white-space", "list-style-type", "table-layout",
	"direction",
}

// ariaAttributes are the WAI-ARIA states and properties allowed on every
// element (the set Canvas's canvas_sanitize allows).
var ariaAttributes = []string{
	"aria-atomic", "aria-autocomplete", "aria-busy", "aria-checked",
	"aria-colcount", "aria-colindex", "aria-colspan", "aria-controls",
	"aria-current", "aria-describedby", "aria-description", "aria-details",
	"aria-disabled", "aria-expanded", "aria-flowto", "aria-haspopup",
	"aria-hidden", "aria-invalid", "aria-label", "aria-labelledby",
	"aria-level", "aria-live", "aria-multiline", "aria-multiselectable",
	"aria-orientation", "aria-owns", "aria-posinset", "aria-pressed",
	"aria-readonly", "aria-relevant", "aria-required", "aria-roledescription",
	"aria-rowcount", "aria-rowindex", "aria-rowspan", "aria-selected",
	"aria-setsize", "aria-sort", "aria-valuemax", "aria-valuemin",
	"aria-valuenow", "aria-valuetext",
}

// ariaRoleValue matches one or more space-separated ARIA role tokens.
var ariaRoleValue = regexp.MustCompile(`^[a-z]+( [a-z]+)*$`)

var (
	policy     *bluemonday.Policy
	policyOnce sync.Once
)

func buildPolicy() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()

	// class on commonly-styled elements (syntax highlighting, KaTeX, callouts,
	// embed wrappers, Tiptap task lists, etc.)
	p.AllowAttrs("class").OnElements(
		"span", "div", "p", "a", "code", "pre",
		"table", "tr", "td", "th",
		"ol", "ul", "li",
		"h1", "h2", "h3", "h4", "h5", "h6",
		"blockquote", "img", "mark", "figure",
		"video", "audio",
	)

	// data-* attributes globally — KaTeX (data-mathml), Canvas equation images
	// (data-equation-content), Tiptap (data-type / data-checked / data-color).
	p.AllowDataAttributes()

	// Safe inline styles (text-align, highlight colour, ...). Without this the
	// editor's alignment / highlight formatting was silently discarded on save.
	p.AllowStyles(allowedStyleProperties...).Globally()

	// Link targets. bluemonday adds rel="noopener" automatically whenever
	// target="_blank" is present.
	p.AllowAttrs("target").Matching(regexp.MustCompile(`^_blank$`)).OnElements("a")

	// tel: links (phone numbers in contact/syllabus pages). Canvas allows
	// them; UGCPolicy's standard URL set is only http/https/mailto (G014).
	p.AllowURLSchemes("tel")

	// Accessibility markup (G013). Canvas allows role plus the aria-* state
	// and property attributes on every element; without them imported and
	// editor-authored content lost aria-label / aria-hidden / role
	// (including role="presentation" on decorative images) on save. Values
	// are attribute-escaped by bluemonday; role is limited to ARIA role
	// tokens.
	p.AllowAttrs("role").Matching(ariaRoleValue).Globally()
	p.AllowAttrs(ariaAttributes...).Globally()

	// Legacy and editor table/list markup: resized column widths (colwidth
	// on cells, width on col), ordered-list start numbers, and the <font>
	// element old Canvas/TinyMCE content still carries (G000, G015).
	p.AllowAttrs("colwidth").Matching(regexp.MustCompile(`^[0-9]{1,4}(,[0-9]{1,4})*$`)).OnElements("td", "th")
	p.AllowAttrs("width").Matching(regexp.MustCompile(`^[0-9]{1,4}(px|%)?$`)).OnElements("col", "colgroup")
	p.AllowAttrs("start").Matching(regexp.MustCompile(`^-?[0-9]{1,6}$`)).OnElements("ol")
	p.AllowAttrs("reversed").Matching(regexp.MustCompile(`^(|reversed)$`)).OnElements("ol")
	p.AllowElements("font")
	p.AllowAttrs("color").Matching(regexp.MustCompile(`^#?[a-zA-Z0-9]{1,20}$`)).OnElements("font")
	p.AllowAttrs("size").Matching(regexp.MustCompile(`^[+-]?[1-7]$`)).OnElements("font")
	p.AllowAttrs("face").Matching(regexp.MustCompile(`^[a-zA-Z0-9 ,'"-]{1,100}$`)).OnElements("font")

	// HTML5 media (Canvas media embeds, uploaded video/audio files). src values
	// go through the policy's URL-scheme check (http/https/mailto + relative).
	p.AllowElements("video", "audio", "source", "track")
	p.AllowAttrs("src", "poster").OnElements("video", "audio", "source")
	p.AllowAttrs("controls", "loop", "muted", "playsinline").OnElements("video", "audio")
	p.AllowAttrs("preload").Matching(regexp.MustCompile(`^(none|metadata|auto)$`)).OnElements("video", "audio")
	p.AllowAttrs("width", "height").Matching(regexp.MustCompile(`^[0-9]+%?$`)).OnElements("video")
	p.AllowAttrs("type").Matching(regexp.MustCompile(`^[a-zA-Z0-9.+/-]+(;[^"<>]*)?$`)).OnElements("source")
	p.AllowAttrs("src", "kind", "srclang", "label", "default").OnElements("track")

	// <iframe> with allow-listed src hosts only.
	p.AllowElements("iframe")
	p.AllowAttrs("src").Matching(TrustedIframeSrc).OnElements("iframe")
	p.AllowAttrs("width", "height").Matching(regexp.MustCompile(`^[0-9]+%?$`)).OnElements("iframe")
	p.AllowAttrs("allowfullscreen").OnElements("iframe")
	p.AllowAttrs("frameborder").Matching(regexp.MustCompile(`^[0-9]+$`)).OnElements("iframe")
	p.AllowAttrs("title").OnElements("iframe")

	return p
}

// Sanitize cleans untrusted HTML before it is persisted. It is idempotent and
// returns "" for "". Note that it entity-encodes text (' becomes &#39;), so it
// must only be applied to fields that are rendered as HTML — use PlainText for
// plain-text fields that the client renders as text.
func Sanitize(input string) string {
	if input == "" {
		return ""
	}
	policyOnce.Do(func() {
		policy = buildPolicy()
	})
	return policy.Sanitize(input)
}

var strictPolicy = bluemonday.StrictPolicy()

// maxPlainTextPasses bounds the strip/decode loop in PlainText. Each pass can
// only shorten the string or leave it unchanged, so in practice it converges
// in one or two passes; the bound just guarantees termination.
const maxPlainTextPasses = 8

// PlainText sanitizes a plain-text field (a description or message typed into
// a <textarea> and rendered by the client as text).
//
// Sanitize is wrong for these fields: bluemonday re-serializes text with HTML
// entity escaping, so "Don't forget A & B" was stored as
// "Don&#39;t forget A &amp; B" and shown literally by the client (B005).
//
// PlainText strips every tag (StrictPolicy) and decodes entities, repeating
// until the value is stable. The result is literal text — apostrophes and
// ampersands are kept as typed — and still contains no markup an HTML parser
// would recognise, so it stays inert even if a consumer renders it as HTML
// (an encoded payload such as "&lt;img onerror=...&gt;" is decoded and then
// stripped on the next pass). Text that merely looks like a tag ("<b>") is
// removed, which is acceptable for a field that must not contain HTML.
func PlainText(input string) string {
	out := input
	for i := 0; i < maxPlainTextPasses; i++ {
		if out == "" {
			return out
		}
		next := html.UnescapeString(strictPolicy.Sanitize(out))
		if next == out {
			return out
		}
		out = next
	}
	// Did not converge (pathological nesting): fall back to the escaped,
	// tag-free form, which is always inert.
	return strictPolicy.Sanitize(out)
}
