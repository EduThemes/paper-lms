// Package csssanitize cleans and scopes page stylesheets (wiki_pages.css).
// See docs/page-designer/README.md, "Page stylesheet rules".
//
// Sanitize parses author CSS with a CSS Syntax Level 3 tokenizer and the
// spec's rule-parsing algorithm (parse.go), keeps only what the policy allows
// and writes it back out in a canonical, readable form:
//
//   - Style rules, plus @media, @supports, @container and @keyframes (nested
//     as deep as maxDepth). Every other at-rule is dropped: @import,
//     @font-face, @namespace, @charset, @page, @layer, @property, vendor
//     @-webkit-keyframes, and unknown ones. CSS nesting (rules inside a style
//     rule's block) is dropped too.
//   - Declarations whose property is a standard-looking name
//     (^-?[a-z][a-z0-9-]*$, not behavior/binding in any vendor form) or a
//     custom property (--name), and whose value passes
//     htmlsanitize.CSSValueOK, the same validator inline styles use: listed
//     functions only, url()/image-set() limited to https: and Paper's file,
//     icon and artwork paths, no escapes, comments or at-keywords. A value
//     may load a URL only in background / background-image
//     (htmlsanitize.URLStyleProperties) and in content on a rule whose
//     selectors all target img (the designer's dark-mode icon swap,
//     img[data-pd-icon]{content:url(...)}); not in filter, border-image,
//     list-style, cursor, mask, clip-path or a custom property (which var()
//     could carry anywhere). Unlike inline styles, position: fixed and
//     sticky and any z-index are allowed: the page root is a containment
//     box. "!important" is kept.
//   - Selectors made of ordinary selector tokens (no backslash escapes,
//     strings with markup, at-keywords or braces), with whitespace and
//     combinators normalized.
//
// Comments are removed. The output never contains "</", "<!--" or "-->".
//
// Scope rewrites sanitized CSS for one page (scope.go).
package csssanitize

import (
	"errors"
	"regexp"
	"strings"

	"github.com/tdewolff/parse/v2/css"

	"github.com/EduThemes/paper-lms/internal/security/htmlsanitize"
)

// MaxBytes is the largest stylesheet Sanitize accepts.
const MaxBytes = 200 * 1024

// MaxScopeBytes is the largest sanitized stylesheet: the input limit of
// Scope and Resanitize, and the most Sanitize may return. Sanitize's output
// is reformatted (indentation, one declaration per line), typically ~40%
// larger than its input, so a sheet within MaxBytes can sanitize to more
// than MaxBytes; with MaxBytes as Scope's limit such a sheet would silently
// scope to nothing.
const MaxScopeBytes = 2 * MaxBytes

// ErrTooLarge is returned by Sanitize when the input exceeds MaxBytes.
var ErrTooLarge = errors.New("stylesheet is larger than 200 KB")

const (
	// maxDepth bounds at-rule nesting (@supports { @media { ... } }).
	maxDepth = 8
	// maxSelectorBytes bounds one selector.
	maxSelectorBytes = 1024
	// maxPreludeBytes bounds an @media / @supports / @container prelude.
	maxPreludeBytes = 1024
)

var (
	propertyName       = regexp.MustCompile(`^-?[a-z][a-z0-9-]*$`)
	customPropertyName = regexp.MustCompile(`^--[a-zA-Z0-9_-]+$`)
	vendorPrefix       = regexp.MustCompile(`^-[a-z]+-`)
	keyframesName      = regexp.MustCompile(`^-?[a-zA-Z_][a-zA-Z0-9_-]{0,99}$`)
	keyframeSelector   = regexp.MustCompile(`^(from|to|100(\.0{1,4})?%|[0-9]{1,2}(\.[0-9]{1,4})?%)$`)
	hashSelector       = regexp.MustCompile(`^#[a-zA-Z0-9_-]+$`)
)

// deniedProperties are refused in any vendor form (-ms-behavior,
// -moz-binding, ...): both attach script or behaviour to elements in old
// engines.
var deniedProperties = map[string]bool{"behavior": true, "binding": true}

// deniedInPropertyName rejects made-up property names that would spell a
// script scheme or legacy hook when followed by ":" ("xjavascript: 0"). No
// real property contains these words.
func deniedInPropertyName(prop string) bool {
	for _, w := range []string{"script", "expression", "behavior", "binding"} {
		if strings.Contains(prop, w) {
			return true
		}
	}
	return false
}

// selectorFunctions are the functional pseudo-classes a selector may use.
var selectorFunctions = map[string]bool{
	"is(": true, "where(": true, "not(": true, "has(": true,
	"nth-child(": true, "nth-last-child(": true, "nth-of-type(": true,
	"nth-last-of-type(": true, "lang(": true, "dir(": true,
}

// matchesAnyFunctions are the functional pseudo-classes whose argument is
// matched against the element itself. Inside them a selector may not use a
// combinator, and nothing inside any selector function may name html, body,
// :root or :scope: "p:is(html:has(a[href*=x]) *)" would let a page's
// stylesheet test the rest of Paper's document from inside the scoped page.
// (matches and the -any forms are not in selectorFunctions; they are listed
// so the rule holds if they are ever added.)
var matchesAnyFunctions = map[string]bool{
	"is": true, "where": true, "not": true, "matches": true, "-webkit-any": true, "-moz-any": true,
}

// keyframesReserved cannot be keyframes names.
var keyframesReserved = map[string]bool{
	"none": true, "initial": true, "inherit": true, "unset": true,
	"revert": true, "revert-layer": true, "default": true,
}

// node is a sanitized rule.
type node struct {
	at        string   // "", "media", "supports", "container" or "keyframes"
	prelude   string   // normalized prelude; the name for @keyframes
	selectors []string // style rule (or keyframe selectors inside @keyframes)
	decls     []decl
	children  []*node
}

type decl struct {
	prop      string
	value     string
	important bool
}

// Sanitize parses author CSS and returns it normalized, keeping only what is
// allowed. It is idempotent (see Resanitize for sanitized input larger than
// MaxBytes). ErrTooLarge when the input exceeds MaxBytes, or when the
// normalized sheet would exceed MaxScopeBytes.
func Sanitize(css string) (string, error) {
	if len(css) > MaxBytes {
		return "", ErrTooLarge
	}
	return sanitizeBounded(css)
}

// Resanitize re-applies Sanitize's rules to a stylesheet Sanitize produced
// earlier (a stored page stylesheet, a copied or imported one). Its input
// limit is MaxScopeBytes, because Sanitize's output can be larger than
// MaxBytes.
func Resanitize(css string) (string, error) {
	if len(css) > MaxScopeBytes {
		return "", ErrTooLarge
	}
	return sanitizeBounded(css)
}

func sanitizeBounded(css string) (string, error) {
	out := render(sanitizeSheet(css))
	if len(out) > MaxScopeBytes {
		return "", ErrTooLarge
	}
	return out, nil
}

// sanitizeSheet parses and filters a stylesheet.
func sanitizeSheet(s string) []*node {
	p := newParser(s)
	return p.sanitizeRules(span{0, len(p.toks)}, 0, false)
}

func (p *parser) sanitizeRules(s span, depth int, inKeyframes bool) []*node {
	var out []*node
	for _, r := range p.rules(s.lo, s.hi) {
		if !r.hasBlock {
			continue // @import, @charset, @namespace, @layer a; ...
		}
		if r.at == "" {
			if n := p.sanitizeStyleRule(r, inKeyframes); n != nil {
				out = append(out, n)
			}
			continue
		}
		if inKeyframes || depth >= maxDepth {
			continue
		}
		var n *node
		switch r.at {
		case "media", "container":
			prelude, ok := conditionPrelude(p.tokensIn(r.prelude))
			if !ok {
				continue
			}
			n = &node{at: r.at, prelude: prelude, children: p.sanitizeRules(r.block, depth+1, false)}
		case "supports":
			prelude, ok := supportsPrelude(p.tokensIn(r.prelude))
			if !ok {
				continue
			}
			n = &node{at: r.at, prelude: prelude, children: p.sanitizeRules(r.block, depth+1, false)}
		case "keyframes":
			toks := p.tokensIn(r.prelude)
			if len(toks) != 1 || toks[0].tt != css.IdentToken || !keyframesName.MatchString(toks[0].data) ||
				keyframesReserved[strings.ToLower(toks[0].data)] {
				continue
			}
			n = &node{at: r.at, prelude: toks[0].data, children: p.sanitizeRules(r.block, depth+1, true)}
		default:
			continue // @font-face, @page, @layer, @property, @-webkit-keyframes, unknown
		}
		if len(n.children) > 0 && safeText(render([]*node{n})) {
			out = append(out, n)
		}
	}
	return out
}

func (p *parser) sanitizeStyleRule(r rawRule, inKeyframes bool) *node {
	var selectors []string
	for _, part := range splitTopLevel(p.tokensIn(r.prelude)) {
		part = trimWS(part)
		var sel string
		var ok bool
		if inKeyframes {
			sel, ok = keyframeSelectorText(part)
		} else {
			sel, ok = selectorText(part)
		}
		if ok {
			selectors = append(selectors, sel)
		}
	}
	if len(selectors) == 0 {
		return nil
	}
	decls := p.sanitizeDecls(r.block, !inKeyframes && allSubjectsAreImg(selectors))
	if len(decls) == 0 {
		return nil
	}
	n := &node{selectors: selectors, decls: decls}
	if !safeText(render([]*node{n})) {
		return nil
	}
	return n
}

// urlValueAllowed reports whether prop may have a value that loads a URL:
// the background properties, and content when every selector of the rule
// targets img.
func urlValueAllowed(prop string, imgSubject bool) bool {
	if strings.HasPrefix(prop, "--") {
		return false
	}
	p := vendorPrefix.ReplaceAllString(prop, "")
	return htmlsanitize.URLStyleProperties[p] || (p == "content" && imgSubject)
}

// allSubjectsAreImg reports whether every selector's subject (its last
// compound) starts with the type selector img.
func allSubjectsAreImg(selectors []string) bool {
	if len(selectors) == 0 {
		return false
	}
	for _, sel := range selectors {
		toks := trimWS(tokenize(sel))
		start, depth := 0, 0
		for i, t := range toks {
			switch {
			case isOpener(t.tt):
				depth++
			case t.tt == css.RightParenthesisToken || t.tt == css.RightBracketToken:
				if depth > 0 {
					depth--
				}
			case depth == 0 && (t.tt == css.WhitespaceToken || t.delim('>') || t.delim('+') || t.delim('~')):
				start = i + 1
			}
		}
		if start >= len(toks) || !toks[start].ident("img") {
			return false
		}
	}
	return true
}

func (p *parser) sanitizeDecls(s span, imgSubject bool) []decl {
	var out []decl
	for _, d := range p.declarations(s.lo, s.hi) {
		var prop string
		switch d.name.tt {
		case css.CustomPropertyNameToken:
			if !customPropertyName.MatchString(d.name.data) {
				continue
			}
			prop = d.name.data
		case css.IdentToken:
			prop = strings.ToLower(d.name.data)
			if !propertyName.MatchString(prop) || strings.HasPrefix(prop, "--") {
				continue
			}
			if deniedProperties[vendorPrefix.ReplaceAllString(prop, "")] || deniedProperties[prop] || deniedInPropertyName(prop) {
				continue
			}
		default:
			continue
		}
		toks := p.tokensIn(d.value)
		important := false
		if n := len(toks); n >= 2 && toks[n-1].ident("important") {
			k := n - 2
			for k >= 0 && toks[k].tt == css.WhitespaceToken {
				k--
			}
			if k >= 0 && toks[k].delim('!') {
				important = true
				toks = trimWS(toks[:k])
			}
		}
		value := joinTokens(toks)
		if !htmlsanitize.CSSValueOK(value) {
			continue
		}
		if htmlsanitize.CSSValueLoadsURL(value) && !urlValueAllowed(prop, imgSubject) {
			continue
		}
		out = append(out, decl{prop: prop, value: value, important: important})
	}
	return out
}

// selectorText validates one complex selector and returns it normalized:
// whitespace runs become one space, combinators get one space either side,
// and there is no space just inside parentheses or before a comma.
func selectorText(toks []token) (string, bool) {
	if len(toks) == 0 {
		return "", false
	}
	var b strings.Builder
	space := false
	bracket := 0
	write := func(s string) {
		if space && b.Len() > 0 {
			last := b.String()[b.Len()-1]
			if last != '(' && s != ")" && s != "," {
				b.WriteByte(' ')
			}
		}
		space = false
		b.WriteString(s)
	}
	var funcs []string // open functional pseudo-classes, innermost last
	inner := func() string {
		if len(funcs) == 0 {
			return ""
		}
		return funcs[len(funcs)-1]
	}
	prevColon := false
	for _, t := range toks {
		if strings.Contains(t.data, `\`) {
			return "", false
		}
		if t.tt == css.WhitespaceToken {
			if bracket == 0 {
				space = true
			}
			continue
		}
		colon := prevColon
		prevColon = t.tt == css.ColonToken
		if len(funcs) > 0 && bracket == 0 {
			// Inside :is() / :where() / :not() a combinator would let the
			// argument look at the page root's ancestors ("p:is(html:has(..) *)").
			if matchesAnyFunctions[inner()] && space && b.Len() > 0 {
				last := b.String()[b.Len()-1]
				if last != '(' && last != ',' && t.tt != css.RightParenthesisToken && t.tt != css.CommaToken {
					return "", false // descendant combinator
				}
			}
			if t.tt == css.IdentToken {
				name := strings.ToLower(t.data)
				switch {
				case colon && (name == "root" || name == "scope"):
					return "", false
				case !colon && (name == "html" || name == "body"):
					return "", false
				case name == "of" && strings.HasPrefix(inner(), "nth-"):
					// :nth-child(An+B of S): S is a selector list too.
					return "", false
				}
			}
		}
		switch t.tt {
		case css.IdentToken, css.NumberToken, css.DimensionToken, css.ColonToken:
		case css.CustomPropertyNameToken:
			if !customPropertyName.MatchString(t.data) {
				return "", false
			}
		case css.HashToken:
			if !hashSelector.MatchString(t.data) {
				return "", false
			}
		case css.StringToken:
			if !selectorStringOK(t.data) {
				return "", false
			}
		case css.IncludeMatchToken, css.DashMatchToken, css.PrefixMatchToken, css.SuffixMatchToken, css.SubstringMatchToken:
			if bracket == 0 {
				return "", false
			}
		case css.LeftBracketToken:
			bracket++
		case css.RightBracketToken:
			if bracket == 0 {
				return "", false
			}
			bracket--
		case css.FunctionToken:
			name := strings.ToLower(t.data)
			if !selectorFunctions[name] {
				return "", false
			}
			funcs = append(funcs, strings.TrimSuffix(name, "("))
		case css.RightParenthesisToken:
			if len(funcs) == 0 {
				return "", false
			}
			funcs = funcs[:len(funcs)-1]
		case css.CommaToken:
			if len(funcs) == 0 {
				return "", false
			}
			write(",")
			space = true
			continue
		case css.DelimToken:
			switch t.data {
			case ".", "*":
			case ">", "+", "~":
				if b.Len() == 0 && t.data != ">" {
					// A leading sibling combinator would reach a sibling
					// of the page root once scoped.
					return "", false
				}
				if matchesAnyFunctions[inner()] {
					return "", false // combinator inside :is() / :where() / :not()
				}
				if bracket == 0 {
					space = true
					write(t.data)
					space = true
					continue
				}
				return "", false
			case "=":
				if bracket == 0 {
					return "", false
				}
			default:
				return "", false
			}
		default:
			// ( without a function, { }, ;, @, url(), bad strings, <!-- -->,
			// unicode ranges, ||.
			return "", false
		}
		write(t.data)
	}
	if bracket != 0 || len(funcs) != 0 {
		return "", false
	}
	out := b.String()
	if out == "" || len(out) > maxSelectorBytes {
		return "", false
	}
	return out, true
}

// selectorStringOK accepts a quoted attribute value without escapes,
// newlines or markup.
func selectorStringOK(s string) bool {
	if len(s) < 2 || s[0] != s[len(s)-1] || (s[0] != '"' && s[0] != '\'') {
		return false
	}
	return !strings.ContainsAny(s[1:len(s)-1], "\\\n\r\f<>{};@")
}

func keyframeSelectorText(toks []token) (string, bool) {
	if len(toks) != 1 {
		return "", false
	}
	s := strings.ToLower(toks[0].data)
	if !keyframeSelector.MatchString(s) {
		return "", false
	}
	return s, true
}

// conditionPrelude validates an @media or @container prelude: identifiers,
// numbers, dimensions, ratios, comparisons and parenthesized features. The
// comparison operators are written with a space either side, so a "<" is
// never followed by anything that could start markup.
func conditionPrelude(toks []token) (string, bool) {
	if len(toks) == 0 {
		return "", false
	}
	var b strings.Builder
	space := false
	parens := 0
	emit := func(s string, spaced bool) {
		if (space || spaced) && b.Len() > 0 {
			last := b.String()[b.Len()-1]
			if last != '(' && last != ' ' && s != ")" && s != "," && s != ":" {
				b.WriteByte(' ')
			}
		}
		space = false
		b.WriteString(s)
		if spaced {
			space = true
		}
	}
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if strings.Contains(t.data, `\`) {
			return "", false
		}
		switch t.tt {
		case css.WhitespaceToken:
			space = true
		case css.IdentToken, css.NumberToken, css.DimensionToken, css.PercentageToken:
			emit(t.data, false)
		case css.ColonToken:
			emit(":", false)
			space = true
		case css.CommaToken:
			emit(",", false)
			space = true
		case css.LeftParenthesisToken:
			parens++
			emit("(", false)
		case css.RightParenthesisToken:
			if parens == 0 {
				return "", false
			}
			parens--
			emit(")", false)
		case css.DelimToken:
			switch t.data {
			case "/":
				emit("/", false)
			case "<", ">":
				op := t.data
				if i+1 < len(toks) && toks[i+1].delim('=') {
					op += "="
					i++
				}
				emit(op, true)
			case "=":
				emit("=", true)
			default:
				return "", false
			}
		default:
			return "", false
		}
	}
	out := b.String()
	if parens != 0 || out == "" || len(out) > maxPreludeBytes {
		return "", false
	}
	return out, true
}

// supportsPrelude validates an @supports condition: declarations in
// parentheses, not/and/or, and selector(...) tests. URLs, strings with
// markup, at-keywords, braces and "<" are refused.
func supportsPrelude(toks []token) (string, bool) {
	if len(toks) == 0 {
		return "", false
	}
	var b strings.Builder
	space := false
	depth := 0
	for _, t := range toks {
		if strings.Contains(t.data, `\`) {
			return "", false
		}
		switch t.tt {
		case css.WhitespaceToken:
			space = true
			continue
		case css.IdentToken, css.NumberToken, css.DimensionToken, css.PercentageToken,
			css.ColonToken, css.CommaToken, css.IncludeMatchToken, css.DashMatchToken,
			css.PrefixMatchToken, css.SuffixMatchToken, css.SubstringMatchToken:
		case css.CustomPropertyNameToken:
			if !customPropertyName.MatchString(t.data) {
				return "", false
			}
		case css.HashToken:
			if !hashSelector.MatchString(t.data) {
				return "", false
			}
		case css.StringToken:
			if !selectorStringOK(t.data) {
				return "", false
			}
		case css.FunctionToken:
			if !propertyName.MatchString(strings.ToLower(strings.TrimSuffix(t.data, "("))) {
				return "", false
			}
			depth++
		case css.LeftParenthesisToken, css.LeftBracketToken:
			depth++
		case css.RightParenthesisToken, css.RightBracketToken:
			if depth == 0 {
				return "", false
			}
			depth--
		case css.DelimToken:
			switch t.data {
			case ".", "*", "/", "+", "-", "%", "=", "~":
			case ">":
				// Written with spaces so it can never join a preceding "--".
				space = true
				if b.Len() > 0 {
					b.WriteByte(' ')
				}
				b.WriteString(">")
				continue
			default:
				return "", false
			}
		default:
			return "", false
		}
		if space && b.Len() > 0 {
			last := b.String()[b.Len()-1]
			if last != '(' && t.tt != css.RightParenthesisToken {
				b.WriteByte(' ')
			}
		}
		space = false
		b.WriteString(t.data)
	}
	out := b.String()
	if depth != 0 || out == "" || len(out) > maxPreludeBytes {
		return "", false
	}
	return out, true
}

// safeText is the last check on every rule written out: nothing that could
// end a <style> element or open/close an HTML comment.
func safeText(s string) bool {
	return !strings.Contains(s, "</") && !strings.Contains(s, "<!") && !strings.Contains(s, "-->")
}

// render writes sanitized rules in the canonical, readable form Sanitize
// returns.
func render(nodes []*node) string {
	var b strings.Builder
	renderNodes(&b, nodes, "")
	return b.String()
}

func renderNodes(b *strings.Builder, nodes []*node, indent string) {
	for i, n := range nodes {
		if i > 0 && indent == "" {
			b.WriteByte('\n')
		}
		if n.at != "" {
			b.WriteString(indent + "@" + n.at + " " + n.prelude + " {\n")
			renderNodes(b, n.children, indent+"  ")
			b.WriteString(indent + "}\n")
			continue
		}
		b.WriteString(indent + strings.Join(n.selectors, ",\n"+indent) + " {\n")
		for _, d := range n.decls {
			b.WriteString(indent + "  " + d.prop + ": " + d.value)
			if d.important {
				b.WriteString(" !important")
			}
			b.WriteString(";\n")
		}
		b.WriteString(indent + "}\n")
	}
}
