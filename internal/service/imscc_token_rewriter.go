package service

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// tokenRewriteCtx is the resolution context for a single ParsePackage run.
// All maps are populated by pre-2 (file extraction) and pass-1 (entity
// creation), then read-only during pass-2 (rewriteAllBodies).
type tokenRewriteCtx struct {
	courseID      uint
	fileURLByPath map[string]string    // "web_resources/foo.png" → "/api/v1/files/N/download"
	pageBySlug    map[string]uint      // wiki slug → WikiPage.ID
	entityByMigID map[string]EntityRef // resource Identifier → entity
}

// Token regexes — match Canvas's own placeholder format documented in
// canvas-lms-master/lib/cc/cc_helper.rb (`WEB_CONTENT_TOKEN`, `WIKI_TOKEN`,
// `OBJECT_TOKEN`). Each one is intentionally permissive: the path that
// follows the token can contain any non-quote, non-whitespace, non-angle-
// bracket characters since the token typically appears inside an HTML
// attribute value.
var (
	reFileBase = regexp.MustCompile(`\$IMS-CC-FILEBASE\$/([^"'\s<>]+)`)
	reWikiRef  = regexp.MustCompile(`\$WIKI_REFERENCE\$/wiki/([^"'\s<>]+)`)
	reObjRef   = regexp.MustCompile(`\$CANVAS_OBJECT_REFERENCE\$/([a-zA-Z_]+)/([^"'\s<>]+)`)

	// Match a complete <img> tag, treating quoted attribute values as opaque
	// so a literal `>` inside e.g. alt="x>y" doesn't terminate the match
	// early. RE2 has no backrefs but it does support alternation inside
	// repetition.
	reImgTag = regexp.MustCompile(`(?is)<img\b(?:"[^"]*"|'[^']*'|[^>"'])*>`)

	// Inside an <img> tag, find a src= attribute whose URL points at any
	// Canvas instance's /equation_images/ endpoint and capture the
	// (double-URL-encoded) LaTeX path.
	reEquationSrc = regexp.MustCompile(`(?is)\bsrc=["']https?://[^"'/]+/equation_images/([^"'?#]+)[^"']*["']`)

	// Canvas attaches the original LaTeX in plain form on every equation
	// image as data-equation-content. When present, prefer it over decoding
	// the URL — fewer escaping round-trips, fewer ways to break.
	reEquationDataContent = regexp.MustCompile(`(?is)\bdata-equation-content=["']([^"']+)["']`)

	// Escapes a LaTeX string for safe embedding in HTML text content.
	// Math may legitimately contain & < > but we don't want them to break
	// HTML parsing or render as tags.
	htmlMathEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
)

// rewriteEquationImages converts Canvas-hosted equation-image <img> tags
// into LaTeX rendered by KaTeX/MathJax on this LMS. Canvas's
// /equation_images/<latex> URLs are double-URL-encoded; we decode twice and
// emit a span the renderer can pick up.
//
// Format chosen: <span class="math_equation_latex">\(<latex>\)</span>
// — `\(...\)` is the standard MathJax inline-math delimiter and KaTeX's
// auto-render extension also handles it. The class hook lets the editor
// preserve the markup on round-trips.
func rewriteEquationImages(html string) string {
	if html == "" {
		return html
	}
	return reImgTag.ReplaceAllStringFunc(html, func(tag string) string {
		// Skip non-equation images entirely.
		srcMatch := reEquationSrc.FindStringSubmatch(tag)
		if len(srcMatch) < 2 {
			return tag
		}

		// Prefer the canvas-supplied data-equation-content (raw LaTeX) when
		// available; fall back to double-decoding the URL.
		var latex string
		if dc := reEquationDataContent.FindStringSubmatch(tag); len(dc) >= 2 {
			latex = dc[1]
		} else {
			first, err := url.QueryUnescape(srcMatch[1])
			if err != nil {
				first = srcMatch[1]
			}
			latex, err = url.QueryUnescape(first)
			if err != nil {
				latex = first
			}
		}
		latex = strings.TrimSpace(latex)
		if latex == "" {
			return tag
		}
		return `<span class="math_equation_latex">\(` + htmlMathEscaper.Replace(latex) + `\)</span>`
	})
}

// rewriteTokens replaces Canvas placeholder tokens in HTML with concrete URLs
// for this Paper LMS course. Tokens that can't be resolved (missing file or
// missing referenced entity) are left in place — keeping the broken token in
// the output is more debuggable than silently emptying the link.
func rewriteTokens(html string, rc *tokenRewriteCtx) string {
	if html == "" || rc == nil {
		return html
	}

	// Canvas equation_images are domain-independent and don't depend on the
	// resolution context — handle them first so the result is consistent
	// across import targets.
	html = rewriteEquationImages(html)

	out := reFileBase.ReplaceAllStringFunc(html, func(match string) string {
		m := reFileBase.FindStringSubmatch(match)
		if len(m) < 2 {
			return match
		}
		raw := m[1]
		// Strip an optional anchor/query so /file.pdf?canvas_download=1 still resolves.
		bare := raw
		if i := strings.IndexAny(bare, "?#"); i >= 0 {
			bare = bare[:i]
		}
		// Try direct, URL-decoded, and trimmed-leading-slash variants.
		candidates := []string{bare}
		if dec, err := url.QueryUnescape(bare); err == nil && dec != bare {
			candidates = append(candidates, dec)
		}
		for _, c := range candidates {
			if u, ok := rc.fileURLByPath[c]; ok {
				return u
			}
			if u, ok := rc.fileURLByPath[strings.TrimPrefix(c, "/")]; ok {
				return u
			}
			// Canvas often emits paths under "web_resources/" without the
			// prefix — try prefixing.
			if u, ok := rc.fileURLByPath["web_resources/"+strings.TrimPrefix(c, "/")]; ok {
				return u
			}
		}
		return match
	})

	out = reWikiRef.ReplaceAllStringFunc(out, func(match string) string {
		m := reWikiRef.FindStringSubmatch(match)
		if len(m) < 2 {
			return match
		}
		slug := m[1]
		if i := strings.IndexAny(slug, "?#"); i >= 0 {
			slug = slug[:i]
		}
		if _, ok := rc.pageBySlug[slug]; ok {
			return fmt.Sprintf("/courses/%d/pages/%s", rc.courseID, slug)
		}
		return match
	})

	out = reObjRef.ReplaceAllStringFunc(out, func(match string) string {
		m := reObjRef.FindStringSubmatch(match)
		if len(m) < 3 {
			return match
		}
		objType := strings.ToLower(m[1]) // assignments, quizzes, discussion_topics, wiki_pages, modules, files
		migID := m[2]
		if i := strings.IndexAny(migID, "?#"); i >= 0 {
			migID = migID[:i]
		}

		ref, ok := rc.entityByMigID[migID]
		if !ok {
			return match
		}

		// Map Canvas object names to our route prefixes.
		switch objType {
		case "assignments":
			if ref.Type == "Assignment" {
				return fmt.Sprintf("/courses/%d/assignments/%d", rc.courseID, ref.ID)
			}
		case "quizzes":
			if ref.Type == "Quiz" {
				return fmt.Sprintf("/courses/%d/quizzes/%d", rc.courseID, ref.ID)
			}
		case "discussion_topics", "discussions":
			if ref.Type == "DiscussionTopic" {
				return fmt.Sprintf("/courses/%d/discussion_topics/%d", rc.courseID, ref.ID)
			}
		case "modules":
			if ref.Type == "ContextModule" {
				return fmt.Sprintf("/courses/%d/modules/%d", rc.courseID, ref.ID)
			}
		case "wiki_pages", "pages":
			// Wiki pages are referenced by slug elsewhere; fall through if
			// we have an ID-mapped one.
			if ref.Type == "WikiPage" {
				return fmt.Sprintf("/courses/%d/pages/%d", rc.courseID, ref.ID)
			}
		case "files", "attachments":
			if ref.Type == "Attachment" {
				return fmt.Sprintf("/api/v1/files/%d/download", ref.ID)
			}
		}
		return match
	})

	return out
}
