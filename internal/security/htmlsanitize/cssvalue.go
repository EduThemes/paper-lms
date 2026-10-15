package htmlsanitize

import (
	"io"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
)

// This file is the CSS value validator shared by the inline style policy
// (this package) and the page stylesheet sanitizer (internal/security/
// csssanitize). Both call CSSValueOK on every declaration value they keep, so
// a value that is safe in a style="" attribute is exactly a value that is safe
// in a page stylesheet. Property-specific limits that only make sense inline
// (position, z-index) live in htmlsanitize.go.

const (
	// MaxCSSValueBytes is the longest declaration value CSSValueOK accepts.
	MaxCSSValueBytes = 4096
	// MaxCSSURLBytes is the longest url() / image-set() URL CSSURLOK accepts.
	MaxCSSURLBytes = 2048

	maxCSSValueTokens = 1024
	maxCSSValueDepth  = 32
)

// cssFunctions are the functions a declaration value may call. Anything else
// (expression(), element(), -moz-element(), src(), env(), attr(), paint(),
// anchor(), ...) makes the whole value invalid. url() is not listed: the
// tokenizer turns it into a URL token, which is checked by CSSURLOK.
var cssFunctions = map[string]bool{
	// Maths and variables.
	"calc": true, "min": true, "max": true, "clamp": true, "var": true,
	"round": true, "mod": true, "rem": true, "abs": true, "sign": true,
	"sin": true, "cos": true, "tan": true, "asin": true, "acos": true,
	"atan": true, "atan2": true, "pow": true, "sqrt": true, "hypot": true,
	"log": true, "exp": true,
	// Colour.
	"rgb": true, "rgba": true, "hsl": true, "hsla": true, "hwb": true,
	"lab": true, "lch": true, "oklab": true, "oklch": true, "color": true,
	"color-mix": true, "light-dark": true,
	// Images. image-set() arguments are URLs and are checked like url().
	"linear-gradient": true, "radial-gradient": true, "conic-gradient": true,
	"repeating-linear-gradient": true, "repeating-radial-gradient": true,
	"repeating-conic-gradient": true, "image-set": true, "type": true,
	// Transforms.
	"translate": true, "translatex": true, "translatey": true, "translatez": true,
	"translate3d": true, "rotate": true, "rotatex": true, "rotatey": true,
	"rotatez": true, "rotate3d": true, "scale": true, "scalex": true,
	"scaley": true, "scalez": true, "scale3d": true, "skew": true,
	"skewx": true, "skewy": true, "matrix": true, "matrix3d": true,
	"perspective": true,
	// Filters (opacity() doubles as the filter function).
	"blur": true, "brightness": true, "contrast": true, "drop-shadow": true,
	"grayscale": true, "hue-rotate": true, "invert": true, "opacity": true,
	"saturate": true, "sepia": true,
	// Easing.
	"cubic-bezier": true, "steps": true, "linear": true,
	// Grid.
	"repeat": true, "minmax": true, "fit-content": true,
	// Basic shapes (clip-path, shape-outside).
	"inset": true, "circle": true, "ellipse": true, "polygon": true,
	"rect": true, "xywh": true,
	// Generated content and list counters.
	"counter": true, "counters": true,
}

// cssVendorPrefixes are stripped from function names before the allowlist
// lookup (-webkit-linear-gradient, -webkit-image-set, ...). -moz-element is
// still rejected: element() is not on the list.
var cssVendorPrefixes = []string{"-webkit-", "-moz-", "-ms-", "-o-"}

// cssDeniedSubstrings make a value invalid wherever they appear (strings and
// URLs included), as a backstop to the token checks below.
var cssDeniedSubstrings = []string{
	"expression(", "javascript:", "vbscript:", "livescript:", "-moz-binding",
	"behavior", "/*", "*/", "<!--", "-->", "</",
}

var (
	// cssCustomPropertyName is a custom property name as var() takes it.
	cssCustomPropertyName = regexp.MustCompile(`^--[a-zA-Z0-9_-]+$`)
	cssHashToken          = regexp.MustCompile(`^#[a-zA-Z0-9_-]+$`)
	cssMIMEType           = regexp.MustCompile(`^[a-zA-Z0-9.+/-]{1,100}$`)
	cssURLHost            = regexp.MustCompile(`(?i)^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$`)

	// cssSameOriginURL lists the Paper paths a stylesheet or inline style may
	// load: a course file download, a designer icon, and theme artwork.
	cssSameOriginURL = regexp.MustCompile(`(?i)^/(` +
		`api/v1/files/[0-9]{1,20}/download(\?[a-z0-9=&_.%-]*)?` +
		`|api/v1/design/icons/[a-z0-9_-]+/[a-z0-9_.-]+\.svg(\?[a-z0-9=&_.%-]*)?` +
		`|design-assets/([a-z0-9_@.-]+/)*[a-z0-9_@.-]+(#[a-z0-9_-]+)?` +
		`)$`)
)

// CSSURLOK reports whether u (the unquoted argument of url() or an
// image-set() string) may be loaded by content CSS: an https:// URL with a
// plain host (no credentials), or one of Paper's own paths
// (/api/v1/files/<id>/download, /api/v1/design/icons/..., /design-assets/...).
// data:, http:, protocol-relative (//host), fragment-only (#id) and every
// other scheme are rejected, as is anything containing whitespace, quotes,
// parentheses, backslashes or markup characters.
func CSSURLOK(u string) bool {
	if u == "" || len(u) > MaxCSSURLBytes || !utf8.ValidString(u) {
		return false
	}
	for _, r := range u {
		if r <= 0x20 || r >= 0x7f || strings.ContainsRune("\"'()\\<>{}^`|;", r) {
			return false
		}
	}
	if strings.Contains(u, "..") {
		return false
	}
	if len(u) > 8 && strings.EqualFold(u[:8], "https://") {
		pu, err := url.Parse(u)
		if err != nil || !strings.EqualFold(pu.Scheme, "https") || pu.Opaque != "" || pu.User != nil {
			return false
		}
		return pu.Host != "" && cssURLHost.MatchString(pu.Hostname())
	}
	return cssSameOriginURL.MatchString(u)
}

// CSSValueOK reports whether value is safe as the value of one CSS
// declaration, inline or in a page stylesheet. The caller strips a trailing
// "!important" first; the property name is checked separately.
//
// The value is tokenized with a CSS Syntax Level 3 tokenizer and every token
// must be one a value can legitimately need: identifiers, numbers,
// dimensions, percentages, colours (#hash), strings, commas, the arithmetic
// delimiters + - * /, balanced ( ) [ ], and calls to the functions in
// cssFunctions. Rejected outright: backslash escapes, control characters,
// comments, at-keywords, ; { } : ! @ < > and the other delimiters, unknown
// functions, unbalanced brackets or quotes, var() with a malformed name or
// inside image-set(), and url()/image-set() URLs that fail CSSURLOK.
func CSSValueOK(value string) bool {
	if value == "" || len(value) > MaxCSSValueBytes || !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		switch {
		case r == '\\':
			return false
		case r < 0x20 && r != '\t' && r != '\n' && r != '\r' && r != '\f':
			return false
		case r == 0x7f || (r >= 0x80 && r <= 0x9f):
			return false
		}
	}
	lower := strings.ToLower(value)
	for _, bad := range cssDeniedSubstrings {
		if strings.Contains(lower, bad) {
			return false
		}
	}

	const (
		varNone = iota
		varWantName
		varWantCommaOrClose
	)
	var (
		stack    []string // open function names, "(" and "["
		varState = varNone
		tokens   int
	)
	inside := func(name string) bool {
		for _, s := range stack {
			if s == name {
				return true
			}
		}
		return false
	}
	top := func() string {
		if len(stack) == 0 {
			return ""
		}
		return stack[len(stack)-1]
	}

	l := css.NewLexer(parse.NewInputString(value))
	for {
		tt, data := l.Next()
		if tt == css.ErrorToken {
			if l.Err() != io.EOF {
				return false
			}
			break
		}
		tokens++
		if tokens > maxCSSValueTokens {
			return false
		}
		if tt == css.WhitespaceToken {
			continue
		}
		switch varState {
		case varWantName:
			if tt != css.CustomPropertyNameToken || !cssCustomPropertyName.Match(data) {
				return false
			}
			varState = varWantCommaOrClose
			continue
		case varWantCommaOrClose:
			if tt != css.CommaToken && tt != css.RightParenthesisToken {
				return false
			}
			varState = varNone
		}

		switch tt {
		case css.IdentToken, css.NumberToken, css.PercentageToken, css.DimensionToken, css.CommaToken:
		case css.CustomPropertyNameToken:
			if !cssCustomPropertyName.Match(data) {
				return false
			}
		case css.HashToken:
			if !cssHashToken.Match(data) {
				return false
			}
		case css.StringToken:
			s, ok := unquoteCSSString(string(data))
			if !ok {
				return false
			}
			switch {
			case top() == "type":
				if !cssMIMEType.MatchString(s) {
					return false
				}
			case top() == "image-set":
				if !CSSURLOK(s) {
					return false
				}
			case strings.ContainsAny(s, ";{}<>@\n\r\f"):
				return false
			}
		case css.URLToken:
			if !CSSURLOK(urlTokenArgument(string(data))) {
				return false
			}
		case css.FunctionToken:
			name := strings.ToLower(string(data[:len(data)-1]))
			for _, prefix := range cssVendorPrefixes {
				name = strings.TrimPrefix(name, prefix)
			}
			if !cssFunctions[name] {
				return false
			}
			if name == "var" {
				if inside("image-set") {
					return false
				}
				varState = varWantName
			}
			stack = append(stack, name)
		case css.LeftParenthesisToken:
			stack = append(stack, "(")
		case css.LeftBracketToken:
			stack = append(stack, "[")
		case css.RightParenthesisToken:
			if len(stack) == 0 || top() == "[" {
				return false
			}
			stack = stack[:len(stack)-1]
		case css.RightBracketToken:
			if top() != "[" {
				return false
			}
			stack = stack[:len(stack)-1]
		case css.DelimToken:
			if len(data) != 1 || !strings.ContainsRune("+-*/", rune(data[0])) {
				return false
			}
		default:
			// At-keywords, bad strings/URLs, ; : { }, <!-- -->, comments,
			// unicode ranges, attribute matchers, ||.
			return false
		}
		if len(stack) > maxCSSValueDepth {
			return false
		}
	}
	return len(stack) == 0 && varState == varNone
}

// unquoteCSSString strips the quotes from a string token. Strings with
// escapes were already rejected; an unterminated string (the tokenizer
// returns one at EOF) is rejected here.
func unquoteCSSString(tok string) (string, bool) {
	if len(tok) < 2 || tok[0] != tok[len(tok)-1] || (tok[0] != '"' && tok[0] != '\'') {
		return "", false
	}
	return tok[1 : len(tok)-1], true
}

// urlTokenArgument returns the URL inside a url(...) token, quoted or not.
// It returns "" (which CSSURLOK rejects) for anything malformed.
func urlTokenArgument(tok string) string {
	if len(tok) < 5 || !strings.EqualFold(tok[:4], "url(") || tok[len(tok)-1] != ')' {
		return ""
	}
	arg := strings.Trim(tok[4:len(tok)-1], " \t\n\r\f")
	if arg == "" {
		return ""
	}
	if arg[0] == '"' || arg[0] == '\'' {
		s, ok := unquoteCSSString(arg)
		if !ok {
			return ""
		}
		return s
	}
	return arg
}
