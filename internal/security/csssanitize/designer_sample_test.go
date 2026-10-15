package csssanitize

import (
	"strings"
	"testing"
)

// designerCSS is hand-written to match the stylesheet the page designer
// engine emits: custom properties on :root, a dark colourway in
// prefers-color-scheme, icon images through content:url(), hover and focus
// states, reduced-motion handling and the reader font variable.
const designerCSS = `:root {
  --pd-c1: #12355b;
  --pd-c2: #ffffff;
  --pd-c3: #f59e0b;
}
@media (prefers-color-scheme: dark) {
  :root {
    --pd-c1: #9cc3ff;
    --pd-c2: #0d1117;
  }
}
img[data-pd-icon="3"] {
  content: url("/api/v1/design/icons/solid-circle/book-open.svg?fg=ffffff&bg=12355b");
}
.pd-card {
  transition: transform 160ms ease, box-shadow 160ms ease;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.08);
}
.pd-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.16);
  filter: saturate(1.1);
}
.pd-card a:focus-visible {
  outline: 3px solid var(--pd-c3, #f59e0b);
  outline-offset: 2px;
}
.pd-hero h1,
.pd-hero h2 {
  font-family: var(--reading-font-family, "Fraunces", Georgia, serif);
}
@media (prefers-reduced-motion: reduce) {
  .pd-card,
  .pd-card:hover {
    transition: none;
    transform: none;
  }
}
`

func TestDesignerStylesheet(t *testing.T) {
	out := mustSanitize(t, designerCSS)
	// Sanitize separates top-level rules with a blank line; otherwise the
	// designer's output is already in canonical form.
	if strings.ReplaceAll(out, "}\n\n", "}\n") != designerCSS {
		t.Errorf("designer stylesheet changed by Sanitize\n got:\n%s\nwant:\n%s", out, designerCSS)
	}
	const root = "#paper-page-7"
	scoped := Scope(out, root)
	checkScoped(t, scoped, root)
	for _, want := range []string{
		`#paper-page-7{--pd-c1:#12355b;--pd-c2:#ffffff;--pd-c3:#f59e0b}`,
		`html.dark #paper-page-7{--pd-c1:#9cc3ff;--pd-c2:#0d1117}`,
		`#paper-page-7 img[data-pd-icon="3"]{content:url("/api/v1/design/icons/solid-circle/book-open.svg?fg=ffffff&bg=12355b")}`,
		`#paper-page-7 .pd-card a:focus-visible{outline:3px solid var(--pd-c3, #f59e0b);outline-offset:2px}`,
		`#paper-page-7 .pd-hero h1,#paper-page-7 .pd-hero h2{font-family:var(--reading-font-family, "Fraunces", Georgia, serif)}`,
		`@media (prefers-reduced-motion: reduce){#paper-page-7 .pd-card,#paper-page-7 .pd-card:hover{transition:none;transform:none}}`,
	} {
		if !strings.Contains(scoped, want) {
			t.Errorf("scoped stylesheet should contain\n%s\ngot:\n%s", want, scoped)
		}
	}
}
