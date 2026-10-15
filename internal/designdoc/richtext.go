package designdoc

import (
	"html"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	nethtml "golang.org/x/net/html"
)

// This file holds the server-side twins of the engine's link, image and
// rich-text rules (web/src/designer/engine/sanitize.ts sanitizeRich and
// safeHref, render/context.ts safeImageSrc). The engine applies them when it
// renders; the server applies them when it stores a design, so a field the
// designer UI writes into the DOM directly is safe too.

// richTextPolicy is the engine's RichText element set: p, strong, em,
// a[href], ul, ol, li, br. Every other tag is removed (script, style and the
// like with their contents). href values are left as written here and checked
// by SafeHref afterwards: bluemonday's own URL check re-encodes the URL, which
// would turn the engine's {course} token into %7Bcourse%7D.
var richTextPolicy = func() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("p", "strong", "em", "ul", "ol", "li", "br")
	p.AllowAttrs("href").OnElements("a")
	return p
}()

// controlRunes matches the characters the engine strips before it looks for
// a URL scheme: C0 controls, space, DEL and C1 controls.
var controlRunes = regexp.MustCompile("[\u0000- \u007f-\u009f]")

var (
	hrefScheme = regexp.MustCompile(`^([a-z][a-z0-9+.-]*):`)
	hrefSep    = regexp.MustCompile(`[/?#]`)
)

// hrefProbeOK is the engine's safeHref decision on one spelling of a link
// target: https, http (the engine upgrades it), mailto, protocol-relative
// (upgraded to https) and relative URLs, including the {course} token. Any
// other scheme, and any colon before the first / ? or # (a scheme that did
// not parse cleanly), is refused.
func hrefProbeOK(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	probe := strings.ToLower(controlRunes.ReplaceAllString(s, ""))
	if strings.HasPrefix(probe, "//") {
		return true
	}
	if m := hrefScheme.FindStringSubmatch(probe); m != nil {
		switch m[1] {
		case "https", "http", "mailto":
			return true
		}
		return false
	}
	colon := strings.IndexByte(probe, ':')
	if colon == -1 {
		return true
	}
	sep := hrefSep.FindStringIndex(probe)
	return sep != nil && sep[0] < colon
}

// SafeHref reports whether a link target in a design is one the engine's
// safeHref keeps. It checks the value both as written and with HTML entities
// decoded (the engine decodes them first), so an encoded scheme
// ("&#106;avascript:") is refused either way.
func SafeHref(raw string) bool {
	return hrefProbeOK(raw) && hrefProbeOK(html.UnescapeString(raw))
}

var (
	reImageFile  = regexp.MustCompile(`^/api/v1/files/[0-9]{1,20}/download([?#][^\s"'<>\\]*)?$`)
	reImageAsset = regexp.MustCompile(`^/design-assets/[A-Za-z0-9._@-]+(/[A-Za-z0-9._@-]+)*$`)
	reImageHTTPS = regexp.MustCompile(`(?i)^https://[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*(:[0-9]{1,5})?([/?#][^\s"'<>\\]*)?$`)
)

// SafeImageSrc reports whether an image address in a design (banner
// customSrc, instructor photoSrc) is allowed: an https:// URL with a plain
// host, a Paper file download (/api/v1/files/<id>/download...) or theme
// artwork (/design-assets/...). The engine's upgrades of http:// and
// protocol-relative addresses are not accepted here: such a value is dropped.
func SafeImageSrc(src string) bool {
	if src == "" || len(src) > maxTextBytes || strings.Contains(src, "..") {
		return false
	}
	return reImageHTTPS.MatchString(src) || reImageFile.MatchString(src) || reImageAsset.MatchString(src)
}

// SanitizeRichText cleans one RichText field of a design: only p, strong,
// em, a[href], ul, ol, li and br survive, and a link whose href fails
// SafeHref loses its tag (its text stays). When the input already parses to
// exactly the tokens of the cleaned output it is returned unchanged, so a
// body the engine produced keeps its bytes (and the page's data-pd-hash keeps
// matching its design).
func SanitizeRichText(in string) string {
	if in == "" {
		return ""
	}
	out := filterLinks(richTextPolicy.Sanitize(in))
	if sameTokens(in, out) {
		return in
	}
	return out
}

// filterLinks rewrites bluemonday's output so every <a> carries exactly one
// href that passes SafeHref; other anchors are unwrapped.
func filterLinks(s string) string {
	z := nethtml.NewTokenizer(strings.NewReader(s))
	var b strings.Builder
	var open []bool // per open <a>: was its tag kept?
	for {
		tt := z.Next()
		if tt == nethtml.ErrorToken {
			break
		}
		raw := string(z.Raw())
		switch tt {
		case nethtml.StartTagToken, nethtml.SelfClosingTagToken:
			tok := z.Token()
			if tok.Data != "a" {
				b.WriteString(raw)
				continue
			}
			href, ok := "", false
			for _, a := range tok.Attr {
				if a.Key == "href" && a.Namespace == "" {
					href, ok = a.Val, true
					break
				}
			}
			keep := ok && SafeHref(href) && tt == nethtml.StartTagToken
			if keep {
				b.WriteString(`<a href="` + html.EscapeString(href) + `">`)
			}
			if tt == nethtml.StartTagToken {
				open = append(open, keep)
			}
		case nethtml.EndTagToken:
			if z.Token().Data != "a" {
				b.WriteString(raw)
				continue
			}
			if n := len(open); n > 0 {
				if open[n-1] {
					b.WriteString("</a>")
				}
				open = open[:n-1]
			}
		default:
			b.WriteString(raw)
		}
	}
	return b.String()
}

// sameTokens reports whether a and b tokenize to the same sequence of tags,
// attributes and (decoded) text. <x/> and <x> count as the same tag.
func sameTokens(a, b string) bool {
	za := nethtml.NewTokenizer(strings.NewReader(a))
	zb := nethtml.NewTokenizer(strings.NewReader(b))
	for {
		ta, tb := za.Next(), zb.Next()
		if ta == nethtml.SelfClosingTagToken {
			ta = nethtml.StartTagToken
		}
		if tb == nethtml.SelfClosingTagToken {
			tb = nethtml.StartTagToken
		}
		if ta != tb {
			return false
		}
		if ta == nethtml.ErrorToken {
			return true
		}
		x, y := za.Token(), zb.Token()
		if x.Data != y.Data || len(x.Attr) != len(y.Attr) {
			return false
		}
		for i := range x.Attr {
			if x.Attr[i] != y.Attr[i] {
				return false
			}
		}
	}
}
