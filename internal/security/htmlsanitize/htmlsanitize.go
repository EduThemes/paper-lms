// Package htmlsanitize holds the single server-side HTML sanitization policy
// for user-generated rich content (page bodies, assignment/quiz descriptions,
// question text, discussion and announcement messages, syllabi, submission
// bodies, ...).
//
// It is a leaf package (it depends only on bluemonday, golang.org/x/net/html
// and the tdewolff CSS tokenizer) so that both the service layer
// (service.SanitizeHTML, used by handlers on Create AND Update) and the domain
// models (BeforeSave hooks that make every write path — imports, blueprint
// syncs, Commons, question banks — store sanitized HTML) can share one policy
// without an import cycle. The page stylesheet sanitizer
// (internal/security/csssanitize) imports it for the shared CSS value
// validator, CSSValueOK.
//
// The element and attribute policy starts from Canvas's canvas_sanitize gem
// (standard UGC markup, class + data-* attributes, link targets, HTML5 media,
// iframes restricted to trusted media hosts) and adds the sectioning and
// responsive-image markup the page designer emits (header, footer, nav,
// picture, source[srcset], img[srcset]).
//
// Inline CSS is deliberately wider than Canvas's: layout (display, flex, grid,
// gap, alignment), positioning (static/relative/absolute with offsets and a
// small z-index range), backgrounds and gradients, shadows, transforms,
// transitions, animations, filters, logical properties and maths functions.
// Every declaration VALUE is validated by CSSValueOK (cssvalue.go), not just
// its property name: only listed functions; url() and image-set() limited to
// https: and Paper's own file, icon and artwork paths, and allowed only in
// background and background-image (URLStyleProperties); cursor takes
// keywords only; no escapes, comments or at-keywords. Declarations written with backslash escapes are removed
// before bluemonday sees them (stripEscapedStyleDeclarations), because
// bluemonday decodes escapes before calling a value handler but writes the
// raw value out. "!important" is dropped by bluemonday's style parser.
//
// The containment model is what makes positioning safe: every rich-content
// viewer wraps sanitized content in a box with
// "contain: layout paint; isolation: isolate". Layout and paint containment
// make that box the containing block for positioned descendants and clip
// everything they draw to it, and isolation keeps their z-index stacking
// inside it, so content can arrange itself freely but never draw over Paper's
// own interface. position: fixed and sticky are still refused inline (they
// only mean something relative to the viewport, and content shown outside a
// containing viewer, in an email or an export, would not be contained), and
// z-index is limited to -10..100.
package htmlsanitize

import (
	"html"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/microcosm-cc/bluemonday"
)

// The iframe allowlist (TrustedIframeSrc, TrustedIframeHosts) lives in
// iframe_policy.go; the shared CSS value validator in cssvalue.go.

// allowedStyleProperties is the inline-CSS property allowlist. Each value must
// also pass CSSValueOK. position and z-index are not listed here: they have
// their own stricter handlers (positionValueOK, zIndexValueOK).
//
// bluemonday strips vendor prefixes (-webkit-, -moz-, ...) from the property
// name before the lookup, so -webkit-box-shadow is allowed as box-shadow and
// -webkit-line-clamp as line-clamp (and -moz-binding is looked up as
// "binding", which is not listed).
//
// Deliberately NOT allowed:
//   - position: fixed / sticky (see the package comment).
//   - content, quotes: they only apply to ::before/::after, which inline
//     styles cannot reach.
//   - mask, mask-*: a second image-loading surface that nothing needs.
//   - behavior, binding, zoom, all: legacy script hooks, or resets no editor
//     or designer output uses.
//   - pointer-events, will-change, appearance, resize, scroll-snap-*:
//     interaction tuning no editor or designer output uses.
//   - anchor-name, position-anchor, position-area, offset-*: anchor
//     positioning and motion paths can tie an element to elements outside
//     the content.
//   - container, container-*: container queries need a stylesheet anyway.
var allowedStyleProperties = []string{
	// Text and colour.
	"color", "direction", "text-align", "text-align-last", "text-indent",
	"text-justify", "text-decoration", "text-decoration-line",
	"text-decoration-color", "text-decoration-style",
	"text-decoration-thickness", "text-decoration-skip-ink",
	"text-underline-offset", "text-underline-position", "text-transform",
	"text-shadow", "text-wrap", "text-wrap-mode", "text-wrap-style",
	"text-overflow", "text-orientation", "writing-mode", "vertical-align",
	"white-space", "word-break", "word-spacing", "overflow-wrap", "word-wrap",
	"hyphens", "letter-spacing", "line-height", "tab-size", "line-clamp",
	"box-orient", "caret-color", "accent-color", "user-select",
	// Fonts.
	"font", "font-family", "font-size", "font-style", "font-weight",
	"font-variant", "font-variant-caps", "font-variant-numeric",
	"font-variant-ligatures", "font-variant-east-asian", "font-stretch",
	"font-kerning", "font-feature-settings", "font-variation-settings",
	"font-optical-sizing", "font-size-adjust",
	// Box model.
	"margin", "margin-top", "margin-right", "margin-bottom", "margin-left",
	"margin-inline", "margin-inline-start", "margin-inline-end",
	"margin-block", "margin-block-start", "margin-block-end",
	"padding", "padding-top", "padding-right", "padding-bottom", "padding-left",
	"padding-inline", "padding-inline-start", "padding-inline-end",
	"padding-block", "padding-block-start", "padding-block-end",
	"width", "height", "max-width", "max-height", "min-width", "min-height",
	"inline-size", "block-size", "min-inline-size", "max-inline-size",
	"min-block-size", "max-block-size", "box-sizing", "aspect-ratio",
	"overflow", "overflow-x", "overflow-y", "overflow-inline", "overflow-block",
	"visibility", "contain", "isolation", "float", "clear",
	// clip: rect(0 0 0 0) is the visually-hidden pattern the designer uses.
	"clip",
	// Borders and outlines.
	"border", "border-width", "border-style", "border-color",
	"border-top", "border-right", "border-bottom", "border-left",
	"border-top-width", "border-top-style", "border-top-color",
	"border-right-width", "border-right-style", "border-right-color",
	"border-bottom-width", "border-bottom-style", "border-bottom-color",
	"border-left-width", "border-left-style", "border-left-color",
	"border-inline", "border-inline-width", "border-inline-style", "border-inline-color",
	"border-inline-start", "border-inline-start-width", "border-inline-start-style", "border-inline-start-color",
	"border-inline-end", "border-inline-end-width", "border-inline-end-style", "border-inline-end-color",
	"border-block", "border-block-width", "border-block-style", "border-block-color",
	"border-block-start", "border-block-start-width", "border-block-start-style", "border-block-start-color",
	"border-block-end", "border-block-end-width", "border-block-end-style", "border-block-end-color",
	"border-radius", "border-top-left-radius", "border-top-right-radius",
	"border-bottom-right-radius", "border-bottom-left-radius",
	"border-start-start-radius", "border-start-end-radius",
	"border-end-start-radius", "border-end-end-radius",
	"border-image", "border-image-source", "border-image-slice",
	"border-image-width", "border-image-outset", "border-image-repeat",
	"border-collapse", "border-spacing",
	"outline", "outline-width", "outline-style", "outline-color", "outline-offset",
	"box-shadow",
	// Backgrounds.
	"background", "background-color", "background-image", "background-position",
	"background-position-x", "background-position-y", "background-size",
	"background-repeat", "background-clip", "background-origin",
	"background-blend-mode", "background-attachment",
	// Flexbox and grid.
	"display", "flex", "flex-basis", "flex-direction", "flex-flow",
	"flex-grow", "flex-shrink", "flex-wrap", "order",
	"gap", "row-gap", "column-gap", "grid-gap", "grid-row-gap", "grid-column-gap",
	"grid", "grid-area", "grid-template", "grid-template-areas",
	"grid-template-columns", "grid-template-rows", "grid-auto-columns",
	"grid-auto-rows", "grid-auto-flow", "grid-column", "grid-column-start",
	"grid-column-end", "grid-row", "grid-row-start", "grid-row-end",
	"align-content", "align-items", "align-self",
	"justify-content", "justify-items", "justify-self",
	"place-content", "place-items", "place-self",
	// Offsets (position itself has its own handler).
	"top", "right", "bottom", "left", "inset", "inset-inline",
	"inset-inline-start", "inset-inline-end", "inset-block",
	"inset-block-start", "inset-block-end",
	// Multi-column.
	"columns", "column-count", "column-width", "column-rule",
	"column-rule-width", "column-rule-style", "column-rule-color",
	"column-span", "column-fill",
	// Lists and tables.
	"list-style", "list-style-type", "list-style-position", "list-style-image",
	"counter-reset", "counter-increment", "counter-set",
	"table-layout", "caption-side", "empty-cells",
	// Effects.
	"opacity", "filter", "backdrop-filter", "mix-blend-mode", "clip-path",
	"transform", "transform-origin", "transform-style", "transform-box",
	"translate", "rotate", "scale", "perspective", "perspective-origin",
	"backface-visibility", "object-fit", "object-position", "image-rendering",
	"shape-outside", "shape-margin", "shape-image-threshold",
	// Motion. Animation names refer to @keyframes in a stylesheet.
	"transition", "transition-property", "transition-duration",
	"transition-timing-function", "transition-delay", "transition-behavior",
	"animation", "animation-name", "animation-duration",
	"animation-timing-function", "animation-delay",
	"animation-iteration-count", "animation-direction",
	"animation-fill-mode", "animation-play-state", "animation-composition",
	// Scrolling.
	"scroll-margin", "scroll-margin-top", "scroll-margin-right",
	"scroll-margin-bottom", "scroll-margin-left", "scroll-margin-inline",
	"scroll-margin-block",
}

// URLStyleProperties are the only properties whose values may load a URL
// (url() or image-set()), inline or in a page stylesheet: backgrounds. Every
// other property — filter, backdrop-filter, border-image*, list-style*,
// clip-path, shape-outside, mask*, cursor — must not, so content has one
// image-loading surface to reason about, not a dozen.
var URLStyleProperties = map[string]bool{"background": true, "background-image": true}

// cursorKeywords are the values inline cursor may take: keywords only, no
// url() cursor images.
var cursorKeywords = map[string]bool{
	"auto": true, "default": true, "none": true, "context-menu": true, "help": true,
	"pointer": true, "progress": true, "wait": true, "cell": true, "crosshair": true,
	"text": true, "vertical-text": true, "alias": true, "copy": true, "move": true,
	"no-drop": true, "not-allowed": true, "grab": true, "grabbing": true,
	"all-scroll": true, "col-resize": true, "row-resize": true, "n-resize": true,
	"e-resize": true, "s-resize": true, "w-resize": true, "ne-resize": true,
	"nw-resize": true, "se-resize": true, "sw-resize": true, "ew-resize": true,
	"ns-resize": true, "nesw-resize": true, "nwse-resize": true, "zoom-in": true,
	"zoom-out": true, "inherit": true, "initial": true, "unset": true,
	"revert": true, "revert-layer": true,
}

// cursorValueOK allows a single cursor keyword.
func cursorValueOK(v string) bool {
	return cursorKeywords[strings.ToLower(strings.TrimSpace(v))]
}

// positionValueOK limits inline position to the values that stay inside the
// containing viewer: never fixed or sticky.
func positionValueOK(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "static", "relative", "absolute":
		return true
	}
	return false
}

// minZIndex and maxZIndex bound inline z-index. The viewer isolates stacking,
// so the bound keeps content layers predictable; it is not what contains
// them.
const (
	minZIndex = -10
	maxZIndex = 100
)

// zIndexValueOK allows auto or an integer in [minZIndex, maxZIndex].
func zIndexValueOK(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "auto" {
		return true
	}
	if v == "" || len(v) > 4 {
		return false
	}
	n, err := strconv.Atoi(v)
	return err == nil && n >= minZIndex && n <= maxZIndex
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

// MediaURL matches the media URLs kept where bluemonday does no scheme
// check (<video poster>, <source src>): an http(s) URL, a root-relative
// path (not protocol-relative "//host"), or a relative path. Any other
// scheme (javascript:, data:, vbscript:, ...) fails, as does whitespace or
// markup.
var MediaURL = regexp.MustCompile(`(?i)^(https?://[^\s"'<>]+|/([^/\s"'<>][^\s"'<>]*)?|\.{1,2}/[^\s"'<>]*|[a-z0-9_-][a-z0-9_.-]*([/?#][^\s"'<>]*)?)$`)

// srcsetURL is one image candidate URL in a srcset: the MediaURL rule
// (http(s), root-relative or relative; no other scheme), without commas,
// which separate candidates, or backslashes.
const srcsetURL = `(https?://[^\s,"'<>\\]+|/([^/\s,"'<>\\][^\s,"'<>\\]*)?|\.{1,2}/[^\s,"'<>\\]*|[a-z0-9_-][a-z0-9_.-]*([/?#][^\s,"'<>\\]*)?)`

// srcsetCandidate is a srcset URL with an optional width (480w) or pixel
// density (2x, 1.5x) descriptor.
const srcsetCandidate = `\s*` + srcsetURL + `(\s+([0-9]{1,5}w|[0-9]{1,2}(\.[0-9]{1,3})?x))?\s*`

// SrcsetValue matches an img/source srcset attribute in which every
// candidate URL passes the same rule as an image URL elsewhere.
var SrcsetValue = regexp.MustCompile(`(?i)^` + srcsetCandidate + `(,` + srcsetCandidate + `)*,?\s*$`)

var (
	// imgSizes matches a sizes attribute: media conditions and lengths.
	imgSizes = regexp.MustCompile(`^[a-zA-Z0-9 (),.:%+*/-]{1,512}$`)
	// sourceMedia matches a <source media="..."> media query.
	sourceMedia = regexp.MustCompile(`^[a-zA-Z0-9 (),.:/<>=-]{1,256}$`)

	// attrText matches free text in an attribute (img alt, title): anything
	// but control characters, up to 1000 characters. bluemonday entity-
	// escapes attribute values on output, so quotes, ampersands, colons and
	// dashes are safe; the old Paragraph pattern silently dropped alt text
	// containing any of them.
	attrText = regexp.MustCompile(`^[^\x00-\x08\x0B\x0C\x0E-\x1F\x7F\x{80}-\x{9F}]{0,1000}$`)
)

// ariaRoleValue matches one or more space-separated ARIA role tokens.
var ariaRoleValue = regexp.MustCompile(`^[a-z]+( [a-z]+)*$`)

// emailStyleProperties is the inline-CSS allowlist for outgoing email
// (SanitizeEmail): the conservative set rich content had before the Page
// Designer widened it — typography, colour, box model and alignment. An
// email is read outside Paper, in a mail client that applies none of the
// containment the in-app viewers do, so nothing that positions, layers,
// transforms or offsets an element, and nothing that loads a URL, is
// allowed. Values pass the same validator (CSSValueOK) and may not contain
// url() or image-set().
var emailStyleProperties = []string{
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

var (
	policy     *bluemonday.Policy
	policyOnce sync.Once

	emailPolicy     *bluemonday.Policy
	emailPolicyOnce sync.Once
)

func buildPolicy() *bluemonday.Policy {
	p := buildBasePolicy()

	// Inline styles: property allowlist plus value validation. Only the
	// background properties may load a URL (URLStyleProperties).
	var urlProps, otherProps []string
	for _, prop := range allowedStyleProperties {
		if URLStyleProperties[prop] {
			urlProps = append(urlProps, prop)
		} else {
			otherProps = append(otherProps, prop)
		}
	}
	p.AllowStyles(urlProps...).MatchingHandler(CSSValueOK).Globally()
	p.AllowStyles(otherProps...).MatchingHandler(cssValueNoURL).Globally()
	p.AllowStyles("cursor").MatchingHandler(cursorValueOK).Globally()
	p.AllowStyles("position").MatchingHandler(positionValueOK).Globally()
	p.AllowStyles("z-index").MatchingHandler(zIndexValueOK).Globally()
	return p
}

// buildEmailPolicy is the rich-content policy with the conservative email
// style allowlist instead of the widened inline one.
func buildEmailPolicy() *bluemonday.Policy {
	p := buildBasePolicy()
	p.AllowStyles(emailStyleProperties...).MatchingHandler(cssValueNoURL).Globally()
	return p
}

// buildBasePolicy is everything but inline styles: the elements and
// attributes shared by the content and email policies.
func buildBasePolicy() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()

	// class on every element (syntax highlighting, KaTeX, callouts, embed
	// wrappers, Tiptap task lists, designer markup, ...).
	p.AllowAttrs("class").Globally()

	// data-* attributes globally — KaTeX (data-mathml), Canvas equation images
	// (data-equation-content), Tiptap (data-type / data-checked / data-color).
	p.AllowDataAttributes()

	// Free-text alt and title. UGCPolicy limits both to its Paragraph
	// pattern, which drops alt text with a colon, question mark, quote,
	// ampersand, semicolon or em dash (an accessibility bug). bluemonday
	// keeps an attribute if ANY of its policies matches, so this widens them.
	p.AllowAttrs("alt").Matching(attrText).OnElements("img")
	p.AllowAttrs("title").Matching(attrText).Globally()

	// Sectioning elements the page designer emits. main is not allowed: a
	// page has exactly one, and it is Paper's.
	p.AllowElements("header", "footer", "nav")

	// Responsive images: <picture><source srcset media type sizes><img
	// srcset sizes></picture>. bluemonday does not URL-check srcset, so
	// every candidate must match SrcsetValue.
	p.AllowElements("picture")
	p.AllowAttrs("srcset").Matching(SrcsetValue).OnElements("img", "source")
	p.AllowAttrs("sizes").Matching(imgSizes).OnElements("img", "source")
	p.AllowAttrs("media").Matching(sourceMedia).OnElements("source")
	p.AllowAttrs("width", "height").Matching(regexp.MustCompile(`^[0-9]{1,5}$`)).OnElements("source")
	p.AllowAttrs("loading").Matching(regexp.MustCompile(`^(lazy|eager)$`)).OnElements("img")
	p.AllowAttrs("decoding").Matching(regexp.MustCompile(`^(async|sync|auto)$`)).OnElements("img")

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

	// HTML5 media (Canvas media embeds, uploaded video/audio files). video /
	// audio src values go through the policy's URL-scheme check
	// (http/https/mailto + relative). bluemonday does not URL-check
	// <video poster> or <source src>, so those must match MediaURL.
	p.AllowElements("video", "audio", "source", "track")
	p.AllowAttrs("src").OnElements("video", "audio")
	p.AllowAttrs("poster").Matching(MediaURL).OnElements("video")
	p.AllowAttrs("src").Matching(MediaURL).OnElements("source")
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
	p.AllowAttrs("allow").Matching(IframeAllowAttr).OnElements("iframe")
	p.AllowAttrs("referrerpolicy").Matching(iframeReferrerPolicy).OnElements("iframe")
	p.AllowAttrs("loading").Matching(regexp.MustCompile(`^(lazy|eager)$`)).OnElements("iframe")

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
	return policy.Sanitize(stripEscapedStyleDeclarations(input))
}

// SanitizeEmail cleans HTML for the body of an outgoing email. It is
// Sanitize's element and attribute policy with the conservative inline
// style allowlist (emailStyleProperties): no position, offsets, z-index,
// transforms or URL-loading values, because a mail client applies none of
// the containment Paper's viewers do. Idempotent; "" for "".
func SanitizeEmail(input string) string {
	if input == "" {
		return ""
	}
	emailPolicyOnce.Do(func() {
		emailPolicy = buildEmailPolicy()
	})
	return emailPolicy.Sanitize(stripEscapedStyleDeclarations(input))
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
