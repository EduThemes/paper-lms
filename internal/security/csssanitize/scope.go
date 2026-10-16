package csssanitize

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"

	"github.com/tdewolff/parse/v2/css"
)

// rootPattern is the selector Scope accepts as the page root: one id or
// class, e.g. "#paper-page-42".
var rootPattern = regexp.MustCompile(`^[#.][A-Za-z_][A-Za-z0-9_-]{0,127}$`)

// scheme is the colour scheme a rule is limited to after rewriting
// prefers-color-scheme media queries.
type scheme int

const (
	schemeAny scheme = iota
	schemeDark
	schemeLight
)

// combine returns the scheme of a rule limited by both a and b, or false if
// they contradict each other (dark inside light).
func combine(a, b scheme) (scheme, bool) {
	switch {
	case a == schemeAny:
		return b, true
	case b == schemeAny || a == b:
		return a, true
	}
	return schemeAny, false
}

// rootSubjectProperties are the properties a rule may set on the page root
// itself (":root", "html" or "body" in the author's stylesheet). The root is
// the containment box that keeps the page inside Paper's layout, so a rule
// aimed at it may only set inherited, paint-inside-the-box properties and
// custom properties; never display, position, offsets, z-index, contain,
// isolation, overflow, margins, sizes, transforms, filters, shadows,
// outlines, animations or transitions, any of which could undo the
// containment or draw outside the box.
var rootSubjectProperties = map[string]bool{
	"color": true, "line-height": true, "letter-spacing": true, "word-spacing": true,
	"white-space": true, "word-break": true, "overflow-wrap": true, "word-wrap": true,
	"hyphens": true, "tab-size": true, "direction": true, "accent-color": true,
	"caret-color": true, "color-scheme": true, "cursor": true, "quotes": true,
	"counter-reset": true, "counter-set": true, "gap": true, "row-gap": true,
	"column-gap": true, "max-width": true, "max-height": true, "min-height": true,
	"text-align": true, "text-indent": true, "text-transform": true, "text-shadow": true,
	"text-decoration": true, "text-decoration-color": true, "text-decoration-line": true,
	"text-decoration-style": true, "text-decoration-thickness": true,
	"text-underline-offset": true, "text-wrap": true,
	"padding": true, "padding-top": true, "padding-right": true, "padding-bottom": true,
	"padding-left": true, "padding-inline": true, "padding-inline-start": true,
	"padding-inline-end": true, "padding-block": true, "padding-block-start": true,
	"padding-block-end": true,
}

// rootSubjectOK reports whether prop may be set on the page root.
func rootSubjectOK(prop string) bool {
	if strings.HasPrefix(prop, "--") {
		return true
	}
	p := vendorPrefix.ReplaceAllString(prop, "")
	if rootSubjectProperties[p] {
		return true
	}
	// Backgrounds and fonts paint inside the box; borders sit inside it
	// (border-image is excluded: its outset draws outside).
	return strings.HasPrefix(p, "background") || strings.HasPrefix(p, "font") ||
		(strings.HasPrefix(p, "border") && !strings.HasPrefix(p, "border-image")) ||
		strings.HasPrefix(p, "list-style")
}

type scoper struct {
	root      string
	keyframes map[string]string // author name -> per-page name
}

// Scope rewrites sanitized CSS so it applies only inside root
// (e.g. "#paper-page-42").
//
// The input is sanitized again first, so Scope is safe on any input. Then:
//
//   - Every selector gets root as an ancestor: "h2" -> "#r h2". A leading
//     ":root", "html" or "body" (and "html body") becomes root itself:
//     ":root{--x:1}" -> "#r{--x:1}", "body h2" -> "#r h2". If that leading
//     compound has qualifiers they stay on the real element, so the author's
//     condition keeps its meaning: "html.dark p" -> "html.dark #r p",
//     "body.rtl p" -> "body.rtl #r p". Those qualifiers may only be classes
//     and [attr] / [attr=value] tests; a selector with any other (a
//     pseudo-class such as :has(), a function) is dropped, as are selectors
//     that would reach a sibling of the root (":root + p", "~ p"). Sanitize
//     already drops combinators inside :is() / :where() / :not() and any
//     html, body, :root or :scope inside a selector function.
//   - A rule aimed at the root itself may only set rootSubjectProperties
//     and custom properties; other declarations are dropped from it.
//   - @media (prefers-color-scheme: dark) rules are re-emitted with an
//     "html.dark " prefix and the feature removed from the query (light:
//     "html:not(.dark) "), so they follow Paper's theme toggle. Other
//     conditions in the same query stay in an @media; queries in a list are
//     handled one by one. Queries that combine the feature with "or" or
//     with "not" plus other conditions are kept as written (they follow the
//     operating system), as are features nested in parentheses.
//   - @keyframes x becomes @keyframes pd-<hash(root)>-x, and animation /
//     animation-name values in the sheet that name it are rewritten
//     (names reached through var() are not). Keyframe selectors (from, to,
//     percentages) are not prefixed.
//   - @supports and @container blocks and other @media rules are kept, with
//     their contents scoped the same way.
//
// The output is compact (no comments or optional whitespace). Scope returns
// "" if root is not a single "#id" or ".class" selector or css is larger
// than MaxScopeBytes.
func Scope(css, root string) string {
	if !rootPattern.MatchString(root) || len(css) > MaxScopeBytes {
		return ""
	}
	nodes := sanitizeSheet(css)
	h := fnv.New32a()
	_, _ = h.Write([]byte(root))
	s := &scoper{root: root, keyframes: map[string]string{}}
	s.collectKeyframes(nodes, fmt.Sprintf("pd-%08x-", h.Sum32()))
	var b strings.Builder
	s.emit(&b, nodes, schemeAny)
	out := b.String()
	if !safeText(out) {
		return ""
	}
	return out
}

func (s *scoper) collectKeyframes(nodes []*node, prefix string) {
	for _, n := range nodes {
		if n.at == "keyframes" {
			s.keyframes[n.prelude] = prefix + n.prelude
		}
		s.collectKeyframes(n.children, prefix)
	}
}

func (s *scoper) emit(b *strings.Builder, nodes []*node, sch scheme) {
	for _, n := range nodes {
		switch n.at {
		case "":
			s.emitStyleRule(b, n, sch)
		case "keyframes":
			b.WriteString("@keyframes " + s.keyframes[n.prelude] + "{")
			for _, c := range n.children {
				writeRule(b, c.selectors, c.decls)
			}
			b.WriteString("}")
		case "media":
			s.emitMedia(b, n, sch)
		default: // supports, container
			s.wrap(b, "@"+n.at+" "+n.prelude, n.children, sch)
		}
	}
}

// wrap writes children inside an at-rule block, or nothing when no child
// survives scoping.
func (s *scoper) wrap(b *strings.Builder, head string, children []*node, sch scheme) {
	var inner strings.Builder
	s.emit(&inner, children, sch)
	if inner.Len() == 0 {
		return
	}
	b.WriteString(head + "{" + inner.String() + "}")
}

func (s *scoper) emitStyleRule(b *strings.Builder, n *node, sch scheme) {
	var onRoot, inside []string
	for _, sel := range n.selectors {
		scoped, subjectIsRoot, ok := s.scopeSelector(sel, sch)
		if !ok {
			continue
		}
		if subjectIsRoot {
			onRoot = append(onRoot, scoped)
		} else {
			inside = append(inside, scoped)
		}
	}
	decls := make([]decl, 0, len(n.decls))
	for _, d := range n.decls {
		decls = append(decls, s.renameAnimations(d))
	}
	if len(onRoot) > 0 {
		var rootDecls []decl
		for _, d := range decls {
			if rootSubjectOK(d.prop) {
				rootDecls = append(rootDecls, d)
			}
		}
		writeRule(b, onRoot, rootDecls)
	}
	writeRule(b, inside, decls)
}

func writeRule(b *strings.Builder, selectors []string, decls []decl) {
	if len(selectors) == 0 || len(decls) == 0 {
		return
	}
	b.WriteString(strings.Join(selectors, ","))
	b.WriteByte('{')
	for i, d := range decls {
		if i > 0 {
			b.WriteByte(';')
		}
		b.WriteString(d.prop + ":" + d.value)
		if d.important {
			b.WriteString("!important")
		}
	}
	b.WriteByte('}')
}

// renameAnimations points animation and animation-name at the per-page
// keyframes names.
func (s *scoper) renameAnimations(d decl) decl {
	p := vendorPrefix.ReplaceAllString(d.prop, "")
	if (p != "animation" && p != "animation-name") || len(s.keyframes) == 0 {
		return d
	}
	toks := tokenize(d.value)
	for i, t := range toks {
		if t.tt == css.IdentToken {
			if name, ok := s.keyframes[t.data]; ok {
				toks[i].data = name
			}
		}
	}
	d.value = joinTokens(toks)
	return d
}

// compoundEnd returns the index of the first top-level whitespace or
// combinator at or after i.
func compoundEnd(toks []token, i int) int {
	depth := 0
	for ; i < len(toks); i++ {
		t := toks[i]
		switch {
		case isOpener(t.tt):
			depth++
		case t.tt == css.RightParenthesisToken || t.tt == css.RightBracketToken:
			if depth > 0 {
				depth--
			}
		case depth == 0 && (t.tt == css.WhitespaceToken || t.delim('>') || t.delim('+') || t.delim('~')):
			return i
		}
	}
	return i
}

func skipWS(toks []token, i int) int {
	for i < len(toks) && toks[i].tt == css.WhitespaceToken {
		i++
	}
	return i
}

// scopeSelector puts root in front of a sanitized selector. subjectIsRoot
// reports that the selector matches the root itself; ok is false for a
// selector that would reach outside the root.
func (s *scoper) scopeSelector(sel string, sch scheme) (scoped string, subjectIsRoot, ok bool) {
	toks := trimWS(tokenize(sel))
	if len(toks) == 0 {
		return "", false, false
	}
	var htmlQ, bodyQ []token
	i := 0
	htmlDone := false
	switch {
	case toks[0].ident("html"):
		end := compoundEnd(toks, 0)
		htmlQ, i, htmlDone = toks[1:end], end, true
	case len(toks) > 1 && toks[0].is(css.ColonToken) && toks[1].ident("root"):
		end := compoundEnd(toks, 0)
		htmlQ, i, htmlDone = toks[2:end], end, true
	case toks[0].ident("body"):
		end := compoundEnd(toks, 0)
		bodyQ, i = toks[1:end], end
	}
	if htmlDone {
		// "html body", "html > body", ":root body".
		j := skipWS(toks, i)
		separated := j > i
		if j < len(toks) && toks[j].delim('>') {
			j = skipWS(toks, j+1)
			separated = true
		}
		if separated && j < len(toks) && toks[j].ident("body") {
			end := compoundEnd(toks, j)
			bodyQ, i = toks[j+1:end], end
		}
	}
	// Qualifiers stay on the real <html> / <body>, outside the page, so
	// they may only be classes and attribute presence / equality tests:
	// "html:has(input[value^=a]) p" would let the page test the rest of
	// Paper's document, and a :has() on the root's own compound could look
	// at its siblings.
	if !simpleQualifiers(htmlQ) || !simpleQualifiers(bodyQ) {
		return "", false, false
	}
	rest := trimWS(toks[i:])
	if len(rest) > 0 && (rest[0].delim('+') || rest[0].delim('~')) {
		return "", false, false
	}

	var b strings.Builder
	q := ""
	switch sch {
	case schemeDark:
		q = ".dark"
	case schemeLight:
		q = ":not(.dark)"
	}
	q += joinTokens(htmlQ)
	if q != "" {
		b.WriteString("html" + q + " ")
	}
	if len(bodyQ) > 0 {
		b.WriteString("body" + joinTokens(bodyQ) + " ")
	}
	b.WriteString(s.root)
	if len(rest) == 0 {
		return b.String(), true, true
	}
	b.WriteString(" " + joinTokens(rest))
	return b.String(), false, true
}

// simpleQualifiers reports whether toks (a compound's qualifiers after its
// type selector) are only ".class", "[attr]" and "[attr=value]" selectors.
func simpleQualifiers(toks []token) bool {
	for i := 0; i < len(toks); {
		switch {
		case toks[i].delim('.') && i+1 < len(toks) && toks[i+1].is(css.IdentToken):
			i += 2
		case toks[i].is(css.LeftBracketToken):
			j := i + 1
			if j >= len(toks) || !toks[j].is(css.IdentToken) {
				return false
			}
			j++
			if j < len(toks) && toks[j].delim('=') {
				if j+1 >= len(toks) || (!toks[j+1].is(css.IdentToken) && !toks[j+1].is(css.StringToken)) {
					return false
				}
				j += 2
			}
			if j >= len(toks) || !toks[j].is(css.RightBracketToken) {
				return false
			}
			i = j + 1
		default:
			return false
		}
	}
	return true
}

// mediaQuery is one query of an @media prelude after looking for a
// prefers-color-scheme feature.
type mediaQuery struct {
	text   string // the query with the feature removed ("" = all)
	scheme scheme
}

func (s *scoper) emitMedia(b *strings.Builder, n *node, sch scheme) {
	queries, hasScheme := analyzeMedia(n.prelude)
	if !hasScheme {
		s.wrap(b, "@media "+n.prelude, n.children, sch)
		return
	}
	// Group the queries by the scheme they end up limited to, keeping the
	// order in which each scheme first appears.
	var order []scheme
	groups := map[scheme][]string{}
	all := map[scheme]bool{}
	for _, q := range queries {
		eff, ok := combine(sch, q.scheme)
		if !ok {
			continue
		}
		if _, seen := groups[eff]; !seen {
			order = append(order, eff)
			groups[eff] = nil
		}
		if q.text == "" {
			all[eff] = true
		} else {
			groups[eff] = append(groups[eff], q.text)
		}
	}
	for _, eff := range order {
		if all[eff] {
			s.emit(b, n.children, eff)
			continue
		}
		s.wrap(b, "@media "+strings.Join(groups[eff], ","), n.children, eff)
	}
}

// analyzeMedia splits a media query list and takes a top-level
// (prefers-color-scheme: dark|light) feature out of each query.
func analyzeMedia(prelude string) ([]mediaQuery, bool) {
	var out []mediaQuery
	found := false
	for _, part := range splitTopLevel(tokenize(prelude)) {
		q := analyzeQuery(trimWS(part))
		if q.scheme != schemeAny {
			found = true
		}
		out = append(out, q)
	}
	return out, found
}

func analyzeQuery(toks []token) mediaQuery {
	keep := mediaQuery{text: joinTokens(toks)}
	// Top-level items: identifiers and parenthesized groups.
	type item struct {
		toks []token
	}
	var items []item
	for i := 0; i < len(toks); {
		if toks[i].tt == css.WhitespaceToken {
			i++
			continue
		}
		end := i + 1
		if isOpener(toks[i].tt) {
			depth := 0
			for end = i; end < len(toks); end++ {
				if isOpener(toks[end].tt) {
					depth++
				} else if toks[end].tt == css.RightParenthesisToken || toks[end].tt == css.RightBracketToken {
					depth--
					if depth == 0 {
						end++
						break
					}
				}
			}
		}
		items = append(items, item{toks: toks[i:end]})
		i = end
	}
	featureAt := -1
	var sch scheme
	for k, it := range items {
		if len(it.toks) == 1 && it.toks[0].ident("or") {
			return keep
		}
		if f, ok := colorSchemeFeature(it.toks); ok {
			if featureAt >= 0 {
				return keep
			}
			featureAt, sch = k, f
		}
	}
	if featureAt < 0 {
		return keep
	}
	isAnd := func(k int) bool {
		return k >= 0 && k < len(items) && len(items[k].toks) == 1 && items[k].toks[0].ident("and")
	}
	if len(items[0].toks) == 1 && items[0].toks[0].ident("not") {
		if len(items) != 2 || featureAt != 1 {
			return keep
		}
		if sch == schemeDark {
			return mediaQuery{scheme: schemeLight}
		}
		return mediaQuery{scheme: schemeDark}
	}
	drop := map[int]bool{featureAt: true}
	switch {
	case isAnd(featureAt - 1):
		drop[featureAt-1] = true
	case isAnd(featureAt + 1):
		drop[featureAt+1] = true
	}
	var rest []string
	for k, it := range items {
		if !drop[k] {
			rest = append(rest, joinTokens(it.toks))
		}
	}
	text := strings.Join(rest, " ")
	switch strings.ToLower(text) {
	case "all", "only all", "only":
		text = ""
	}
	return mediaQuery{text: text, scheme: sch}
}

// colorSchemeFeature recognizes "(prefers-color-scheme: dark|light)".
func colorSchemeFeature(toks []token) (scheme, bool) {
	var inner []token
	if len(toks) < 2 || !toks[0].is(css.LeftParenthesisToken) || !toks[len(toks)-1].is(css.RightParenthesisToken) {
		return schemeAny, false
	}
	for _, t := range toks[1 : len(toks)-1] {
		if t.tt != css.WhitespaceToken {
			inner = append(inner, t)
		}
	}
	if len(inner) != 3 || !inner[0].ident("prefers-color-scheme") || !inner[1].is(css.ColonToken) {
		return schemeAny, false
	}
	switch {
	case inner[2].ident("dark"):
		return schemeDark, true
	case inner[2].ident("light"):
		return schemeLight, true
	}
	return schemeAny, false
}
