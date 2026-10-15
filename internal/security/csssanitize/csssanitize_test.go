package csssanitize

import (
	"errors"
	"strings"
	"testing"

	"github.com/tdewolff/parse/v2/css"
)

func mustSanitize(t *testing.T, in string) string {
	t.Helper()
	out, err := Sanitize(in)
	if err != nil {
		t.Fatalf("Sanitize(%q): %v", in, err)
	}
	if again, _ := Sanitize(out); again != out {
		t.Fatalf("not idempotent\ninput:  %q\nfirst:  %q\nsecond: %q", in, out, again)
	}
	checkInert(t, out)
	return out
}

func checkInert(t *testing.T, out string) {
	t.Helper()
	lower := strings.ToLower(out)
	// "javascript:" can legitimately appear in a selector ([href^="javascript:"])
	// where it is inert; values are checked for it by the per-case tests.
	for _, bad := range []string{"</style", "</", "<!--", "-->", "@import", "@font-face", "@charset", "@namespace", "\\"} {
		if strings.Contains(lower, bad) {
			t.Errorf("output contains %q:\n%s", bad, out)
		}
	}
}

func TestSanitize_Canonical(t *testing.T) {
	in := `/* brand */ .A > .b+.c ,  h2:is(.x,.y)   { COLOR : Red ; --Pd-X:1px; background: url( "https://cdn.example.com/a.png" ) no-repeat !IMPORTANT }`
	want := ".A > .b + .c,\nh2:is(.x, .y) {\n  color: Red;\n  --Pd-X: 1px;\n  background: url( \"https://cdn.example.com/a.png\" ) no-repeat !important;\n}\n"
	if got := mustSanitize(t, in); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestSanitize_KeepsAllowed(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"fixed and sticky allowed in a stylesheet", `.bar{position:fixed;z-index:99999} .h{position:sticky;top:0}`, []string{"position: fixed;", "z-index: 99999;", "position: sticky;"}},
		{"nested at-rules", `@supports (display: grid) and (not (display: inline-grid)) { @media screen and (min-width:600px) { .g { display: grid } } }`,
			[]string{"@supports (display: grid) and (not (display: inline-grid)) {", "  @media screen and (min-width: 600px) {", "      display: grid;"}},
		{"container query", `@container card (min-width: 400px) { .t { font-size: 2rem } }`, []string{"@container card (min-width: 400px) {"}},
		{"range media query", `@media (width>=600px) and (width<900px) { .a { color: red } }`, []string{"@media (width >= 600px) and (width < 900px) {"}},
		{"keyframes", `@keyframes Float { FROM { opacity: 0 } 50.5% { opacity: .5 } to { opacity: 1 } }`, []string{"@keyframes Float {", "  from {", "  50.5% {", "  to {"}},
		{"is/where/has with commas", `:is(h1, h2) a, :where(.a,.b) > p, li:has(> img, + p) {color:red}`, []string{":is(h1, h2) a,", ":where(.a, .b) > p,", "li:has(> img, + p) {"}},
		{"attribute selector with comma in quotes", `a[title="a,b"], img[data-pd-icon='3'] {color:red}`, []string{`a[title="a,b"],`, `img[data-pd-icon='3'] {`}},
		{"nth-child", `li:nth-child(2n+1), li:nth-of-type(odd) {color:red}`, []string{"li:nth-child(2n+1),"}},
		{"unterminated block closed at end", `.a { color: red`, []string{".a {\n  color: red;\n}"}},
		{"unterminated media closed at end", `@media screen { .a { color: red }`, []string{"@media screen {", "    color: red;"}},
		{"CDO and CDC ignored", `<!-- .a { color: red } -->`, []string{".a {"}},
		{"icon content url", `img[data-pd-icon="1"]{content:url("/api/v1/design/icons/outline/star.svg?fg=000000&bg=ffffff")}`, []string{`content: url("/api/v1/design/icons/outline/star.svg?fg=000000&bg=ffffff");`}},
		{"artwork with @2x", `.h{background-image:image-set(url(/design-assets/themes/t/banner.webp) 1x, url(/design-assets/themes/t/banner@2x.webp) 2x)}`, []string{"banner@2x.webp"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := mustSanitize(t, tc.in)
			for _, w := range tc.want {
				if !strings.Contains(out, w) {
					t.Errorf("output should contain %q\ninput: %s\noutput:\n%s", w, tc.in, out)
				}
			}
		})
	}
}

func TestSanitize_DropsDangerous(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		mustNot []string
	}{
		{"import", `@import url(https://evil.example/x.css); @import "x.css";`, []string{"import", "evil"}},
		{"font-face", `@font-face { font-family: x; src: url(https://evil.example/x.woff) }`, []string{"font", "evil"}},
		{"namespace charset page layer property", `@charset "utf-8"; @namespace svg url(http://www.w3.org/2000/svg); @page { margin: 0 } @layer base { .a { color: red } } @layer x; @property --x { syntax: '<color>'; inherits: false; initial-value: red }`, []string{"@", "margin", "color", "syntax"}},
		{"unknown and vendor at-rules", `@-webkit-keyframes x { from { color: red } } @scope (.a) { .b { color: red } } @starting-style { .a { opacity: 0 } } @foo bar { .a { color: red } }`, []string{"@", "color", "opacity"}},
		{"javascript url", `.a { background: url( "javascript:alert(1)" ) }`, []string{"javascript", ".a"}},
		{"unquoted javascript url", `.a { background-image: url(javascript:alert(1)) }`, []string{"javascript"}},
		{"data url", `.a { background-image: url(data:image/svg+xml;base64,PHN2Zz4=) }`, []string{"data:"}},
		{"http url", `.a { background-image: url(http://evil.example/a.png) }`, []string{"evil"}},
		{"protocol-relative url", `.a { background:url(//evil.example/x) }`, []string{"evil"}},
		{"expression", `.a { width: expression(alert(1)) }`, []string{"expression"}},
		{"behavior", `.a { behavior: url(x.htc); -ms-behavior: url(x.htc) }`, []string{"behavior", "htc"}},
		{"moz-binding", `.a { -moz-binding: url(https://evil.example/x.xml#x) }`, []string{"binding", "evil"}},
		{"escaped url", `.a { background: \75 rl(javascript:alert(1)) }`, []string{"rl(", "javascript"}},
		{"escaped property", `.a { \63 olor: red }`, []string{"olor"}},
		{"escape in selector", `.a\:b { color: red }`, []string{"a"}},
		{"close style tag in selector", `</style><script>alert(1)</script> .a { color: red }`, []string{"script", "alert"}},
		{"close style tag in string", `.a { font-family: "</style><script>alert(1)</script>" }`, []string{"script"}},
		{"close style tag in attribute selector", `a[title="</style>"] { color: red }`, []string{"style"}},
		{"comment in value", `.a { width: calc(1px /* x */ + 2px) }`, []string{"/*", "*/"}},
		{"nested rules", `.a { .b { color: red } &:hover { color: blue } }`, []string{"color", ".b", "hover"}},
		{"at-rule inside a style rule", `.a { @media screen { color: red } }`, []string{"@media", "color"}},
		{"IE star hack", `.a { *zoom: 1 }`, []string{"zoom"}},
		{"sibling of the root", `+ p { color: red } ~ p { color: red }`, []string{"color"}},
		{"unknown selector function", `:host(.a) p, ::part(x) { color: red }`, []string{"host", "part"}},
		{"selector url", `a[href=url(x)] { color: red }`, []string{"url"}},
		{"unicode escape in value", `.a { color: \72 ed }`, []string{"ed"}},
		{"braces in a string value", `.a { content: "}" }`, []string{`"}"`}},
		{"keyframes reserved name", `@keyframes none { from { color: red } }`, []string{"keyframes"}},
		{"keyframes bad selector", `@keyframes x { .a { color: red } 120% { color: red } }`, []string{"keyframes"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := mustSanitize(t, tc.in)
			for _, s := range tc.mustNot {
				if strings.Contains(out, s) {
					t.Errorf("output should not contain %q\ninput: %s\noutput:\n%s", s, tc.in, out)
				}
			}
		})
	}
}

// TestSanitize_RecoversAfterBadInput: a broken rule never takes the next
// good rule with it, except where the browser would also read them as one.
func TestSanitize_RecoversAfterBadInput(t *testing.T) {
	cases := []struct{ name, in string }{
		{"brace inside a string", `.a { content: "}" } .b { color: blue }`},
		{"escaped brace", `.a { color: red\} } .b { color: blue }`},
		{"bad declaration", `.a { color: ; : x; ;; width } .b { color: blue }`},
		{"bad url", `.a { background: url(a b) } .b { color: blue }`},
		{"dropped at-rule block", `@font-face { src: url(x) } .b { color: blue }`},
		{"statement at-rule", `@import "x.css"; .b { color: blue }`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := mustSanitize(t, tc.in)
			if !strings.Contains(out, ".b {\n  color: blue;\n}") {
				t.Errorf("the rule after the bad one should survive\ninput: %s\noutput:\n%s", tc.in, out)
			}
		})
	}
	// An unclosed parenthesis swallows the rest of the sheet in a browser
	// too, so nothing after it may be emitted as a separate rule.
	out := mustSanitize(t, `.a { color: rgb( } .b { color: blue }`)
	if strings.Contains(out, ".b") {
		t.Errorf("rule inside an unclosed parenthesis was emitted:\n%s", out)
	}
}

func TestSanitize_EmptyAndLimits(t *testing.T) {
	if out := mustSanitize(t, ""); out != "" {
		t.Errorf("empty input gave %q", out)
	}
	if out := mustSanitize(t, "/* only a comment */ \n "); out != "" {
		t.Errorf("comment-only input gave %q", out)
	}
	if _, err := Sanitize(strings.Repeat("a", MaxBytes+1)); !errors.Is(err, ErrTooLarge) {
		t.Errorf("want ErrTooLarge, got %v", err)
	}
	if _, err := Sanitize(strings.Repeat("a", MaxBytes)); err != nil {
		t.Errorf("MaxBytes input: %v", err)
	}
	deep := strings.Repeat("@media screen {", maxDepth+2) + ".a{color:red}" + strings.Repeat("}", maxDepth+2)
	if out := mustSanitize(t, deep); strings.Contains(out, "color") {
		t.Errorf("rules nested deeper than maxDepth should be dropped:\n%s", out)
	}
}

func TestScope(t *testing.T) {
	const root = "#paper-page-42"
	cases := []struct {
		name, in, want string
	}{
		{"root custom properties", `:root{--x:1}`, `#paper-page-42{--x:1}`},
		{"html and body map to the root", `html{--a:1} body{--b:2} html body{--c:3} html > body p{color:red}`,
			`#paper-page-42{--a:1}#paper-page-42{--b:2}#paper-page-42{--c:3}#paper-page-42 p{color:red}`},
		{"descendants", `h2, .a > p{color:red}`, `#paper-page-42 h2,#paper-page-42 .a > p{color:red}`},
		{"body descendant", `body h2{color:red}`, `#paper-page-42 h2{color:red}`},
		{"qualified html keeps its condition", `html.dark p{color:red} :root[lang="es"] q{quotes:none}`,
			`html.dark #paper-page-42 p{color:red}html[lang="es"] #paper-page-42 q{quotes:none}`},
		{"qualified body", `body.rtl p{color:red}`, `body.rtl #paper-page-42 p{color:red}`},
		{"child of root", `> p{color:red}`, `#paper-page-42 > p{color:red}`},
		{"siblings of the root dropped", `:root + p{color:red} html ~ p{color:red} body+p{color:red} p{color:blue}`, `#paper-page-42 p{color:blue}`},
		{"dark mode", `@media (prefers-color-scheme: dark){:root{--x:2} p{color:red}}`,
			`html.dark #paper-page-42{--x:2}html.dark #paper-page-42 p{color:red}`},
		{"light mode", `@media (prefers-color-scheme: light){p{color:red}}`, `html:not(.dark) #paper-page-42 p{color:red}`},
		{"not dark", `@media not (prefers-color-scheme: dark){p{color:red}}`, `html:not(.dark) #paper-page-42 p{color:red}`},
		{"dark with another condition", `@media screen and (prefers-color-scheme: dark){p{color:red}}`,
			`@media screen{html.dark #paper-page-42 p{color:red}}`},
		{"dark first then condition", `@media (prefers-color-scheme: dark) and (min-width: 600px){p{color:red}}`,
			`@media (min-width: 600px){html.dark #paper-page-42 p{color:red}}`},
		{"query list", `@media print, (prefers-color-scheme: dark){p{color:red}}`,
			`@media print{#paper-page-42 p{color:red}}html.dark #paper-page-42 p{color:red}`},
		{"or kept as written", `@media (prefers-color-scheme: dark) or (min-width: 1px){p{color:red}}`,
			`@media (prefers-color-scheme: dark) or (min-width: 1px){#paper-page-42 p{color:red}}`},
		{"dark inside light dropped", `@media (prefers-color-scheme: light){@media (prefers-color-scheme: dark){p{color:red}} a{color:blue}}`,
			`html:not(.dark) #paper-page-42 a{color:blue}`},
		{"dark in supports", `@supports (display: grid){@media (prefers-color-scheme: dark){.g{display:grid}}}`,
			`@supports (display: grid){html.dark #paper-page-42 .g{display:grid}}`},
		{"dark with html qualifier", `@media (prefers-color-scheme: dark){html[lang="es"] p{color:red}}`,
			`html.dark[lang="es"] #paper-page-42 p{color:red}`},
		{"other media kept", `@media (prefers-reduced-motion: reduce){*{animation:none!important}}`,
			`@media (prefers-reduced-motion: reduce){#paper-page-42 *{animation:none!important}}`},
		{"keyframes renamed", `@keyframes float{from{transform:none}50%{transform:translateY(-4px)}} .a{animation:float 2s infinite} .b{animation-name:float, other}`,
			`@keyframes pd-fd14c8b1-float{from{transform:none}50%{transform:translateY(-4px)}}#paper-page-42 .a{animation:pd-fd14c8b1-float 2s infinite}#paper-page-42 .b{animation-name:pd-fd14c8b1-float, other}`},
		{"root subject limited", `:root{position:fixed;inset:0;z-index:999;display:contents;contain:none;margin:-100px;width:200vw;transform:scale(9);animation:x 1s;outline:9px solid;box-shadow:0 0 0 9999px red;color:red;--x:1;background:#fff;font-size:20px;padding:1rem;border-top:1px solid}`,
			`#paper-page-42{color:red;--x:1;background:#fff;font-size:20px;padding:1rem;border-top:1px solid}`},
		{"root subject split from the rest", `:root, p{position:fixed;color:red}`, `#paper-page-42{color:red}#paper-page-42 p{position:fixed;color:red}`},
		{"dark root subject limited", `@media (prefers-color-scheme: dark){html.x{position:fixed;color:red}}`, `html.dark.x #paper-page-42{color:red}`},
		{"root inside is dropped", `:is(:root) p{position:fixed} p{color:blue}`, `#paper-page-42 p{color:blue}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Scope(tc.in, root)
			if got != tc.want {
				t.Errorf("Scope(%q)\n got: %s\nwant: %s", tc.in, got, tc.want)
			}
			checkScoped(t, got, root)
		})
	}
}

func TestScope_BadRoot(t *testing.T) {
	for _, root := range []string{"", "body", "#a b", "#a,#b", "#a{", "#1a", "#a</style>", "#" + strings.Repeat("a", 200)} {
		if got := Scope(`p{color:red}`, root); got != "" {
			t.Errorf("Scope with root %q = %q, want empty", root, got)
		}
	}
	if got := Scope(strings.Repeat(" ", MaxScopeBytes+1), "#r"); got != "" {
		t.Errorf("oversized input scoped to %q", got)
	}
}

// TestScope_SanitizesItsInput: Scope is safe on CSS that never went
// through Sanitize.
func TestScope_SanitizesItsInput(t *testing.T) {
	got := Scope(`@import "x"; p{background:url(javascript:alert(1));color:red} </style><script>x</script>{}`, "#r")
	if got != `#r p{color:red}` {
		t.Errorf("got %q", got)
	}
}

// checkScoped parses Scope output back and checks that every style rule
// outside @keyframes applies only to the root or its descendants: each
// selector is [html<q> ][body<q> ]<root>, followed by nothing, a
// descendant/child combinator, or a qualifier of the root itself.
func checkScoped(t *testing.T, out, root string) {
	t.Helper()
	checkInert(t, out)
	if !safeText(out) {
		t.Errorf("unsafe text in %q", out)
	}
	p := newParser(out)
	var walk func(s span, inKeyframes bool)
	walk = func(s span, inKeyframes bool) {
		for _, r := range p.rules(s.lo, s.hi) {
			switch {
			case r.at == "keyframes":
				walk(r.block, true)
			case r.at != "":
				if r.at != "media" && r.at != "supports" && r.at != "container" {
					t.Errorf("unexpected @%s in %q", r.at, out)
				}
				walk(r.block, false)
			case inKeyframes:
				for _, sel := range splitTopLevel(p.tokensIn(r.prelude)) {
					if _, ok := keyframeSelectorText(trimWS(sel)); !ok {
						t.Errorf("bad keyframe selector %q in %q", joinTokens(sel), out)
					}
				}
			default:
				for _, sel := range splitTopLevel(p.tokensIn(r.prelude)) {
					if !rootScoped(trimWS(sel), root) {
						t.Errorf("selector %q is not scoped to %s in %q", joinTokens(sel), root, out)
					}
				}
			}
		}
	}
	walk(span{0, len(p.toks)}, false)
}

// rootScoped also checks that qualifiers left on html / body are only
// classes and [attr] / [attr=value] tests (after the scoper's own
// ":not(.dark)" for light-mode rules).
func rootScoped(toks []token, root string) bool {
	i := 0
	if i < len(toks) && toks[i].ident("html") {
		end := compoundEnd(toks, i)
		q := toks[i+1 : end]
		if len(q) >= 5 && q[0].is(css.ColonToken) && q[1].is(css.FunctionToken) && strings.EqualFold(q[1].data, "not(") &&
			q[2].delim('.') && q[3].ident("dark") && q[4].is(css.RightParenthesisToken) {
			q = q[5:]
		}
		if !simpleQualifiers(q) {
			return false
		}
		i = skipWS(toks, end)
	}
	if i < len(toks) && toks[i].ident("body") {
		end := compoundEnd(toks, i)
		if !simpleQualifiers(toks[i+1 : end]) {
			return false
		}
		i = skipWS(toks, end)
	}
	if i >= len(toks) || toks[i].tt != css.HashToken || toks[i].data != root {
		return false
	}
	j := skipWS(toks, i+1)
	return j >= len(toks) || (!toks[j].delim('+') && !toks[j].delim('~'))
}
