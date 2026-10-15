package htmlsanitize

import (
	"strings"
	"testing"
)

// TestSanitize_InlineStylesSurvive: declarations the page designer and the
// editor emit come out byte for byte (bluemonday normalizes the separator to
// "; " and the property/value separator to ": ", which the inputs already
// use).
func TestSanitize_InlineStylesSurvive(t *testing.T) {
	cases := []struct{ name, style string }{
		{"var with fallback", `color: var(--pd-c1, #fff)`},
		{"nested grid maths", `grid-template-columns: repeat(auto-fill, minmax(min(100%, max(180px, calc((100% - 48px) / 4))), 1fr))`},
		{"flex layout", `display: flex; flex-wrap: wrap; gap: 16px 24px; align-items: center; justify-content: space-between`},
		{"grid areas", `display: grid; grid-template-areas: "hero hero" "side main"; grid-area: hero`},
		{"grid line names", `grid-template-columns: [full-start] minmax(1rem, 1fr) [content-start] minmax(0, 60rem) [content-end] minmax(1rem, 1fr) [full-end]`},
		{"absolute positioning", `position: absolute; inset: 0; top: -12px; left: calc(50% - 8px); z-index: 2`},
		{"relative with negative z", `position: relative; z-index: -10`},
		{"gradient background", `background: linear-gradient(135deg, rgb(18 53 91 / 90%) 0%, color-mix(in oklch, var(--pd-c2, #12355b) 60%, white) 100%)`},
		{"theme artwork", `background-image: url(/design-assets/themes/tide-pool/banner.webp); background-size: cover; background-position: center top; background-repeat: no-repeat`},
		{"retina artwork via image-set", `background-image: image-set(url(/design-assets/themes/tide-pool/banner.webp) 1x, url(/design-assets/themes/tide-pool/banner@2x.webp) 2x)`},
		{"image-set strings and type", `background-image: image-set("/design-assets/a.webp" type("image/webp"), "/design-assets/a.png" type("image/png"))`},
		{"file download background", `background: url(/api/v1/files/42/download) center / cover no-repeat`},
		{"icon background", `background-image: url(/api/v1/design/icons/solid-circle/star.svg?fg=ffffff&bg=12355b)`},
		{"https background", `background-image: url(https://images.example.org/photo.jpg)`},
		{"quoted https background", `background-image: url("https://images.example.org/photo.jpg")`},
		{"shadows and effects", `box-shadow: 0 1px 2px rgba(0, 0, 0, 0.1), 0 8px 24px -4px hsl(210 40% 20% / 0.25); text-shadow: 0 1px 0 #000; opacity: 0.9`},
		{"transform and transition", `transform: translate(-50%, -50%) rotate(3deg) scale(1.02); transition: transform 200ms cubic-bezier(0.2, 0.8, 0.2, 1), opacity 0.3s ease`},
		{"filters", `filter: drop-shadow(0 2px 4px rgb(0 0 0 / 20%)) saturate(1.2) hue-rotate(10deg); backdrop-filter: blur(8px)`},
		{"typography", `font: italic 600 1.125rem/1.5 "Atkinson Hyperlegible", system-ui, sans-serif; letter-spacing: 0.02em; text-transform: uppercase`},
		{"text decoration longhands", `text-decoration-line: underline; text-decoration-thickness: 2px; text-underline-offset: 0.2em; text-wrap: balance`},
		{"logical properties", `margin-inline: auto; padding-block: 2rem 1rem; inline-size: min(100%, 72ch); border-inline-start: 4px solid var(--pd-accent, #f59e0b)`},
		{"radius corners", `border-top-left-radius: 12px; border-bottom-right-radius: 50%`},
		{"aspect and object fit", `aspect-ratio: 16 / 9; object-fit: cover; object-position: 50% 30%`},
		{"clip path", `clip-path: polygon(0 0, 100% 0, 100% 85%, 0 100%)`},
		{"columns", `columns: 2 18rem; column-gap: 2rem; column-rule: 1px solid #ddd`},
		{"animation", `animation: pd-float 6s ease-in-out infinite alternate`},
		{"overflow and visibility", `overflow: hidden; visibility: visible; isolation: isolate`},
		{"clamp font size", `font-size: clamp(1.5rem, 1rem + 2vw, 3rem)`},
		{"oklch colour", `color: oklch(62% 0.19 255)`},
		{"steps easing", `animation-timing-function: steps(4, jump-end)`},
		{"list style", `list-style: disc outside`},
		{"background image", `background-image: url(/design-assets/themes/tide-pool/bullet.svg)`},
		{"cursor", `cursor: pointer`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := `<div style="` + escapeAttr(tc.style) + `">x</div>`
			want := `<div style="` + bluemondayEscapeAttr(tc.style) + `">x</div>`
			out := Sanitize(in)
			if out != want {
				t.Errorf("style changed\nin:   %s\nwant: %s\ngot:  %s", in, want, out)
			}
			if again := Sanitize(out); again != out {
				t.Errorf("not idempotent\nfirst:  %s\nsecond: %s", out, again)
			}
		})
	}
}

// TestSanitize_InlineStylesBlocked: each dangerous declaration is removed and
// the safe declaration next to it is kept.
func TestSanitize_InlineStylesBlocked(t *testing.T) {
	cases := []struct {
		name     string
		bad      string
		mustDrop string
	}{
		{"position fixed", `position: fixed`, "fixed"},
		{"position sticky", `position: sticky`, "sticky"},
		{"position webkit sticky", `position: -webkit-sticky`, "sticky"},
		{"prefixed property still checked", `-webkit-position: fixed`, "fixed"},
		{"z-index too high", `z-index: 9999`, "9999"},
		{"z-index too low", `z-index: -11`, "-11"},
		{"z-index maths", `z-index: calc(1000 * 10)`, "1000"},
		{"javascript url", `background: url(javascript:alert(1))`, "javascript"},
		{"quoted javascript url", `background-image: url( "javascript:alert(1)" )`, "javascript"},
		{"data url", `background-image: url(data:image/png;base64,iVBORw0KGgo=)`, "data:"},
		{"http url", `background-image: url(http://example.com/a.png)`, "example.com"},
		{"protocol-relative url", `background:url(//evil.example/x.png)`, "evil"},
		{"fragment url filter", `filter: url(#steal)`, "steal"},
		{"other same-origin path", `background-image: url(/api/v1/users/self)`, "users"},
		{"path traversal", `background-image: url(/design-assets/../api/v1/users/self)`, "users"},
		{"credentials in url", `background-image: url(https://user@evil.example/x.png)`, "evil"},
		{"image-set bad string", `background-image: image-set("http://evil.example/a.png" 1x)`, "evil"},
		{"var inside image-set", `background-image: image-set(var(--x) 1x)`, "image-set"},
		{"expression", `width: expression(alert(1))`, "expression"},
		{"behavior", `behavior: url(x.htc)`, "htc"},
		{"behavior in a value", `font-family: behavior`, "behavior"},
		{"moz binding", `-moz-binding: url(https://evil.example/x.xml#x)`, "binding"},
		{"moz binding value", `background: -moz-binding`, "binding"},
		{"element()", `background: -moz-element(#password)`, "element"},
		{"unknown function", `width: attr(data-w)`, "attr"},
		{"env()", `padding-top: env(safe-area-inset-top)`, "env("},
		{"escape spelling url", `background: \75 rl(javascript:alert(1))`, "rl("},
		{"escape in value", `color: r\65 d`, `\65`},
		{"comment inside value", `width: calc(100% /* x */ - 1px)`, "calc"},
		{"unbalanced paren", `width: calc(100% - 1px`, "calc"},
		{"stray close paren", `width: 10px)`, "10px"},
		{"at-keyword", `font-family: @import`, "import"},
		{"colon in value", `color: progid:DXImageTransform`, "progid"},
		{"markup in string", `font-family: "</style><script>"`, "script"},
		{"bang delimiter", `color: red ! ie`, "ie"},
		{"bad var name", `color: var(-x)`, "var"},
		{"var without name", `color: var(, red)`, "var"},
		{"not allowed property content", `content: "x"`, "content"},
		{"not allowed property mask", `mask-image: url(/design-assets/a.svg)`, "mask"},
		{"not allowed property pointer-events", `pointer-events: none`, "pointer"},
		{"vbscript", `background: url(vbscript:msgbox)`, "vbscript"},
		// Only backgrounds may load a URL; cursor takes keywords only.
		{"cursor url", `cursor: url(https://cdn.example.com/c.png), auto`, "cdn"},
		{"cursor maths", `cursor: var(--c)`, "cursor"},
		{"filter url", `filter: url(https://cdn.example.com/f.svg#f)`, "cdn"},
		{"backdrop-filter url", `backdrop-filter: url(https://cdn.example.com/f.svg#f)`, "cdn"},
		{"border-image url", `border-image: url(/design-assets/a.png) 30 round`, "border-image"},
		{"border-image-source url", `border-image-source: url(https://cdn.example.com/b.png)`, "cdn"},
		{"list-style-image url", `list-style-image: url(/design-assets/bullet.svg)`, "bullet"},
		{"list-style url", `list-style: url(/design-assets/bullet.svg) inside`, "bullet"},
		{"clip-path url", `clip-path: url(https://cdn.example.com/c.svg#c)`, "cdn"},
		{"shape-outside url", `shape-outside: url(https://cdn.example.com/s.png)`, "cdn"},
		{"shape-outside image-set", `shape-outside: image-set("https://cdn.example.com/s.png" 1x)`, "cdn"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := `<div style="color: red; ` + escapeAttr(tc.bad) + `; margin: 0">x</div>`
			out := Sanitize(in)
			if strings.Contains(out, tc.mustDrop) {
				t.Errorf("output should not contain %q\nin:  %s\nout: %s", tc.mustDrop, in, out)
			}
			if !strings.Contains(out, `style="color: red; margin: 0"`) {
				t.Errorf("the safe declarations around the rejected one should be kept\nin:  %s\nout: %s", in, out)
			}
			if again := Sanitize(out); again != out {
				t.Errorf("not idempotent\nfirst:  %s\nsecond: %s", out, again)
			}
		})
	}
}

// TestSanitize_StyleAttributeDroppedWhenEmpty: when every declaration is
// rejected the attribute disappears instead of being left empty.
func TestSanitize_StyleAttributeDroppedWhenEmpty(t *testing.T) {
	out := Sanitize(`<p style="position: fixed; background: url(javascript:alert(1))">x</p>`)
	if out != `<p>x</p>` {
		t.Errorf("got %q", out)
	}
	// An unterminated string swallows the rest of the attribute, so the
	// whole style fails to parse and is dropped.
	out = Sanitize(`<p style="color: red; font-family: &quot;Open Sans; margin: 0">x</p>`)
	if out != `<p>x</p>` {
		t.Errorf("got %q", out)
	}
}

// TestSanitize_ImportantDropped documents bluemonday's style parser behaviour:
// !important is removed and the declaration kept.
func TestSanitize_ImportantDropped(t *testing.T) {
	out := Sanitize(`<p style="color: red !important">x</p>`)
	if out != `<p style="color: red">x</p>` {
		t.Errorf("got %q", out)
	}
}

// TestSanitize_EscapedDeclarationsRemovedBeforeBluemonday covers the
// pre-pass: escaped declarations are removed, the rest of the attribute and
// document stay byte for byte, and entity-encoded backslashes count.
func TestSanitize_EscapedDeclarationsRemovedBeforeBluemonday(t *testing.T) {
	cases := []struct{ in, want string }{
		{`<p style="color: \72 ed; width: 10px">x</p>`, `<p style="width: 10px">x</p>`},
		{`<p style="color: &#92;72 ed; width: 10px">x</p>`, `<p style="width: 10px">x</p>`},
		{`<p style="\63 olor: red">x</p>`, `<p>x</p>`},
		{`<p title="a\b" style="color: red">x\y</p>`, `<p title="a\b" style="color: red">x\y</p>`},
		{`<p style="font-family: '\5FAE\8F6F\96C5\9ED1', sans-serif; color: red">x</p>`, `<p style="color: red">x</p>`},
	}
	for _, tc := range cases {
		if got := Sanitize(tc.in); got != tc.want {
			t.Errorf("Sanitize(%q)\n got: %q\nwant: %q", tc.in, got, tc.want)
		}
	}
}

func TestSanitize_DesignerMarkup(t *testing.T) {
	cases := []struct {
		name        string
		in          string
		mustContain []string
	}{
		{
			name:        "sectioning elements",
			in:          `<header class="pd-hero"><nav aria-label="Units"><a href="/courses/1/pages/a">A</a></nav></header><footer class="pd-foot">f</footer>`,
			mustContain: []string{`<header class="pd-hero">`, `<nav aria-label="Units">`, `<footer class="pd-foot">`},
		},
		{
			name:        "class on any element",
			in:          `<section class="pd-band"><h2 class="pd-title">x</h2><figure class="pd-fig"><figcaption class="c">c</figcaption></figure><strong class="s">s</strong></section>`,
			mustContain: []string{`<section class="pd-band">`, `<figcaption class="c">`, `<strong class="s">`},
		},
		{
			name: "picture with sources",
			in: `<picture><source srcset="/design-assets/themes/t/banner.webp 1x, /design-assets/themes/t/banner@2x.webp 2x" type="image/webp" media="(min-width: 600px)">` +
				`<img src="/design-assets/themes/t/banner.png" srcset="/design-assets/themes/t/banner.png 960w, /design-assets/themes/t/banner@2x.png 1920w" sizes="(max-width: 960px) 100vw, 960px" alt="" loading="lazy" decoding="async"></picture>`,
			mustContain: []string{
				"<picture>",
				`srcset="/design-assets/themes/t/banner.webp 1x, /design-assets/themes/t/banner@2x.webp 2x"`,
				`media="(min-width: 600px)"`,
				`srcset="/design-assets/themes/t/banner.png 960w, /design-assets/themes/t/banner@2x.png 1920w"`,
				`sizes="(max-width: 960px) 100vw, 960px"`,
				`loading="lazy"`, `decoding="async"`,
			},
		},
		{
			name:        "alt text with punctuation survives",
			in:          `<img src="/a.png" alt="Figure 2: Sales vs. costs — 2024 & 2025? &quot;Up&quot;; mostly.">`,
			mustContain: []string{`alt="Figure 2: Sales vs. costs — 2024 &amp; 2025? &#34;Up&#34;; mostly."`},
		},
		{
			name:        "title with punctuation survives",
			in:          `<abbr title="Hypertext Markup Language: the web's language?">HTML</abbr>`,
			mustContain: []string{`title="Hypertext Markup Language: the web&#39;s language?"`},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := Sanitize(tc.in)
			for _, s := range tc.mustContain {
				if !strings.Contains(out, s) {
					t.Errorf("output should contain %q\nin:  %s\nout: %s", s, tc.in, out)
				}
			}
			if again := Sanitize(out); again != out {
				t.Errorf("not idempotent\nfirst:  %s\nsecond: %s", out, again)
			}
		})
	}
}

func TestSanitize_DesignerMarkupStaysInert(t *testing.T) {
	cases := []struct {
		name           string
		in             string
		mustNotContain []string
	}{
		{"main is not allowed", `<main>x</main>`, []string{"<main"}},
		{"srcset with a javascript entry", `<img src="/a.png" srcset="/a.png 1x, javascript:alert(1) 2x">`, []string{"srcset", "javascript"}},
		{"srcset with a data entry", `<img src="/a.png" srcset="data:image/png;base64,AAAA 1x">`, []string{"srcset", "data:"}},
		{"srcset with markup", `<img src="/a.png" srcset="/a.png&quot; onerror=&quot;alert(1) 1x">`, []string{"srcset", "onerror"}},
		{"source srcset bad url", `<picture><source srcset="vbscript:x 1x"></picture>`, []string{"vbscript"}},
		{"loading other value", `<img src="/a.png" loading="auto">`, []string{"loading"}},
		{"decoding other value", `<img src="/a.png" decoding="x">`, []string{"decoding"}},
		{"alt with a control character", "<img src=\"/a.png\" alt=\"a\x01b\">", []string{"alt="}},
		{"title cannot break the attribute", `<p title='x" onmouseover="alert(1)'>y</p>`, []string{`onmouseover="`}},
		{"class cannot break the attribute", `<p class='x" onclick="alert(1)'>y</p>`, []string{`onclick="`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := Sanitize(tc.in)
			for _, s := range tc.mustNotContain {
				if strings.Contains(out, s) {
					t.Errorf("output should NOT contain %q\nin:  %s\nout: %s", s, tc.in, out)
				}
			}
		})
	}
}

func TestZIndexAndPositionHandlers(t *testing.T) {
	for _, v := range []string{"auto", "0", "-10", "100", "42", " 7 "} {
		if !zIndexValueOK(v) {
			t.Errorf("z-index %q should be allowed", v)
		}
	}
	for _, v := range []string{"", "101", "-11", "1e2", "+1000", "99999", "calc(1)", "inherit"} {
		if zIndexValueOK(v) {
			t.Errorf("z-index %q should be rejected", v)
		}
	}
	for _, v := range []string{"static", "relative", "absolute", "ABSOLUTE"} {
		if !positionValueOK(v) {
			t.Errorf("position %q should be allowed", v)
		}
	}
	for _, v := range []string{"fixed", "sticky", "-webkit-sticky", "inherit", ""} {
		if positionValueOK(v) {
			t.Errorf("position %q should be rejected", v)
		}
	}
}

// escapeAttr writes a double-quoted attribute value.
func escapeAttr(s string) string {
	return strings.ReplaceAll(s, `"`, "&quot;")
}

// bluemondayEscapeAttr is how bluemonday writes a double quote back out.
func bluemondayEscapeAttr(s string) string {
	return strings.NewReplacer(`&`, "&amp;", `"`, "&#34;").Replace(s)
}
