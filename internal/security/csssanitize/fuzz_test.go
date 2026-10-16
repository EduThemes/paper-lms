package csssanitize

import (
	"strings"
	"testing"
)

var fuzzSeeds = []string{
	"",
	`:root{--x:1} body h2{color:red}`,
	`/* c */ .a { color: red } /* unterminated`,
	`.a { content: "}" } .b { color: blue }`,
	`.a { color: red\} } .b { color: blue }`,
	`@supports (display:grid) { @media (prefers-color-scheme: dark) { html.dark p { color: red } } }`,
	`.a { color: red`,
	`@media screen { .a {`,
	`:is(.a, .b) :where(h1,h2) :has(> img, + p) { color: red }`,
	`a[title="a,b}"] { color: red }`,
	`</style><script>alert(1)</script>`,
	`@import url(x); @font-face { src: url(x) }`,
	`.a { background: url( "javascript:alert(1)" ) }`,
	`.a { background: \75 rl(x) } .b\:c { co\lor: red }`,
	`@keyframes x { from { opacity: 0 } to { opacity: 1 } } .a { animation: x 1s }`,
	`:root + p, ~ p, html ~ p, body + p { color: red }`,
	`<!-- .a { color: red } -->`,
	`@media (width<600px) and (width>=1px) { a { color: red } }`,
	`@media not (prefers-color-scheme: dark), print and (prefers-color-scheme: light) { :root { position: fixed } }`,
	`.a { .b { color: red } } .c { @media x { color: red } }`,
	`.x-->p{color:red} .y--{color:red}`,
	"\x00\xff .a{color:red}",
	// Audit regressions: a page stylesheet querying the rest of the document,
	// and URLs outside backgrounds.
	`html:has(input[name="q"][value^="se"]) p{color:red}`,
	`p:is(html:has(a[href*="/users/77"]) *){color:red}`,
	`:root:has(~ div){color:red} body:not(.x) p{color:red} li:nth-child(1 of html *){color:red}`,
	`p:where(.a > .b), p:not(:scope *), p:is([rel=html]){color:red}`,
	`.b{filter:url(https://cdn.example.com/f.svg#f);--i:url(/design-assets/a.png);cursor:url(/design-assets/c.png),auto} img.i{content:url(/design-assets/a.png)}`,
}

// FuzzSanitize checks the properties that must hold for any input:
// Sanitize is idempotent, nothing it or Scope writes can end the <style>
// element or import anything, and every Scope selector is anchored at the
// root.
func FuzzSanitize(f *testing.F) {
	for _, s := range fuzzSeeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		if len(in) > 4096 {
			return
		}
		out, err := Sanitize(in)
		if err != nil {
			t.Fatalf("Sanitize(%q): %v", in, err)
		}
		again, _ := Sanitize(out)
		if again != out {
			t.Fatalf("not idempotent\ninput:  %q\nfirst:  %q\nsecond: %q", in, out, again)
		}
		for _, s := range []string{out, Scope(in, "#paper-page-1")} {
			lower := strings.ToLower(s)
			if strings.Contains(lower, "</style") || strings.Contains(lower, "@import") ||
				strings.Contains(s, "<!--") || strings.Contains(s, "-->") || strings.Contains(s, `\`) {
				t.Fatalf("unsafe output for %q: %q", in, s)
			}
		}
		scoped := Scope(in, "#paper-page-1")
		if Scope(out, "#paper-page-1") != scoped {
			t.Fatalf("Scope(Sanitize(x)) != Scope(x) for %q", in)
		}
		checkScoped(t, scoped, "#paper-page-1")
	})
}

// TestFuzzSeeds runs the seed corpus as a plain test too, so CI covers it
// without -fuzz.
func TestFuzzSeeds(t *testing.T) {
	for _, s := range fuzzSeeds {
		out, err := Sanitize(s)
		if err != nil {
			t.Fatal(err)
		}
		if again, _ := Sanitize(out); again != out {
			t.Errorf("not idempotent for %q", s)
		}
		checkScoped(t, Scope(s, "#paper-page-1"), "#paper-page-1")
	}
}
