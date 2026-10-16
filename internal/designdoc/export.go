package designdoc

import (
	"regexp"
	"strings"
)

// Helpers for exporting a designed page's HTML to Common Cartridge, where
// it must render on its own (Canvas has no Paper stylesheet and no
// /design-assets on its origin).

var (
	// var(--pd-<name>, #hex): how the engine writes colours that differ
	// between the light and dark colourways, with the light colour as the
	// fallback. Same pattern as the engine's flattenDesignVars
	// (web/src/designer/engine/render/design.ts), so export output matches
	// what the designer previews.
	rePDVar = regexp.MustCompile(`var\(--pd-[a-z0-9-]+,\s*(#[0-9a-fA-F]{3,8})\)`)
	// A root-relative designer URL at the start of a URL token: after a
	// quote, "(", "=", ",", whitespace, ";" (end of &quot;) or the start.
	reDesignURL = regexp.MustCompile(`(^|["'(=,\s;])(/design-assets/|/api/v1/design/icons/)`)
)

// FlattenPDVars rewrites var(--pd-…, #hex) to #hex, so the page renders
// its light colourway without the designer stylesheet (Canvas has none).
// It is a port of the engine's flattenDesignVars: only the --pd- prefix
// with a hex fallback is touched; other var() references, and --pd- vars
// with any other fallback, are left alone.
func FlattenPDVars(html string) string {
	if !strings.Contains(html, "var(--pd-") {
		return html
	}
	return rePDVar.ReplaceAllString(html, "$1")
}

// AbsolutizeDesignURLs prefixes root-relative /design-assets/… and
// /api/v1/design/icons/… URLs with base (the public frontend URL, e.g.
// "https://paper.example.edu"). An empty or non-http(s) base leaves the
// HTML unchanged.
func AbsolutizeDesignURLs(html, base string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" || (!strings.HasPrefix(base, "https://") && !strings.HasPrefix(base, "http://")) {
		return html
	}
	if !strings.Contains(html, "/design-assets/") && !strings.Contains(html, "/api/v1/design/icons/") {
		return html
	}
	return reDesignURL.ReplaceAllStringFunc(html, func(m string) string {
		g := reDesignURL.FindStringSubmatch(m)
		return g[1] + base + g[2]
	})
}

// RelativizeDesignURLs is the inverse of AbsolutizeDesignURLs for one
// base: base + "/design-assets/…" becomes "/design-assets/…" again, so a
// cartridge exported from this Paper instance imports with local URLs.
func RelativizeDesignURLs(html, base string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" || !strings.Contains(html, base) {
		return html
	}
	r := strings.NewReplacer(
		base+"/design-assets/", "/design-assets/",
		base+"/api/v1/design/icons/", "/api/v1/design/icons/",
	)
	return r.Replace(html)
}

// ExportPageHTML applies both export rewrites to a page body.
func ExportPageHTML(html, base string) string {
	return AbsolutizeDesignURLs(FlattenPDVars(html), base)
}
