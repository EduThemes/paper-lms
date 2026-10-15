package htmlsanitize

import (
	"regexp"
	"strings"
	"testing"
)

// designerSample is hand-written to match what the page designer engine
// (web/src/designer/engine renderDesign) emits: every inline property and
// attribute it uses, var(--pd-cN, #hex) colours, the nested grid maths, the
// visually-hidden clip pattern, srcset with @2x artwork and data-* hooks.
const designerSample = `<div id="pd-doc-1" class="paper-design" data-pd-doc="doc-1" data-pd-v="1">` +
	`<header class="pd-hero" style="position: relative; display: grid; grid-template-columns: repeat(auto-fill, minmax(min(100%, max(180px, calc((100% - 48px) / 4))), 1fr)); gap: 24px; row-gap: 16px; column-gap: 24px; align-items: center; padding: clamp(24px, 4vw, 48px); background: var(--pd-c1, #12355b); color: var(--pd-c2, #ffffff); border-radius: 16px; overflow: hidden; min-height: 240px">` +
	`<picture><img src="/design-assets/themes/tide-pool/banner.webp" srcset="/design-assets/themes/tide-pool/banner.webp 1x, /design-assets/themes/tide-pool/banner@2x.webp 2x" alt="" loading="lazy" style="width: 100%; height: auto; aspect-ratio: 16 / 9; object-fit: cover; max-width: 100%"></picture>` +
	`<h1 style="font-size: clamp(2rem, 1.2rem + 3vw, 3.5rem); font-weight: 800; line-height: 1.1; margin: 0; text-align: left">Welcome</h1>` +
	`<span style="position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; border: 0">Course home</span>` +
	`</header>` +
	`<nav aria-label="Modules"><ul style="list-style: none; display: flex; flex-direction: row; flex-wrap: wrap; gap: 12px; margin-left: 0; padding: 0">` +
	`<li style="flex: 1 1 200px; min-width: 0; border-top: 4px solid var(--pd-c3, #f59e0b); border-left: 1px solid var(--pd-c4, #d0d7de); border-bottom: 1px solid var(--pd-c4, #d0d7de)">` +
	`<a href="/courses/1/pages/module-1" style="color: var(--pd-c5, #0b5cad); text-decoration: none; font-weight: 600">Module 1</a>` +
	`<img data-pd-icon="3" src="/api/v1/design/icons/solid-circle/book-open.svg?fg=ffffff&bg=12355b" alt="" style="top: 8px; left: 8px; position: absolute; height: 40px; width: 40px">` +
	`</li></ul></nav>` +
	`<footer class="pd-foot" style="max-width: 72ch; margin: 0 auto; font-size: 0.875rem">Footer</footer>` +
	`</div>`

// TestSanitize_DesignerSampleSurvives: the sample comes out byte for byte
// apart from bluemonday's entity encoding of "&" in URLs and the
// rel="nofollow" it adds to links.
func TestSanitize_DesignerSampleSurvives(t *testing.T) {
	out := Sanitize(designerSample)
	want := strings.NewReplacer(
		"&bg=", "&amp;bg=",
		`font-weight: 600">Module`, `font-weight: 600" rel="nofollow">Module`,
	).Replace(designerSample)
	if out != want {
		t.Errorf("designer markup changed\nwant: %s\ngot:  %s", want, out)
	}
	if Sanitize(out) != out {
		t.Error("not idempotent")
	}
	// Every inline property the engine uses must be on the allowlist.
	used := regexp.MustCompile(`([a-z-]+): `).FindAllStringSubmatch(designerSample, -1)
	allowed := map[string]bool{"position": true, "z-index": true}
	for _, p := range allowedStyleProperties {
		allowed[p] = true
	}
	for _, m := range used {
		if !allowed[m[1]] {
			t.Errorf("designer property %q is not allowed", m[1])
		}
	}
}
