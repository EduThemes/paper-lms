package csssanitize

import (
	"strings"
	"testing"
)

// Only backgrounds may load a URL in a page stylesheet, plus content on img
// (the designer's dark-mode icon swap); custom properties may not carry one
// (var() would take it anywhere).
func TestSanitize_URLsOnlyInBackgroundsAndImgContent(t *testing.T) {
	const u = `url("/design-assets/themes/a/x.webp")`
	out := mustSanitize(t, `
.a { background: `+u+` no-repeat; background-image: `+u+`; -webkit-background-image: `+u+` }
img[data-pd-icon="3"], :root .card > img.icon { content: `+u+` }
.b { filter: `+u+`; backdrop-filter: `+u+`; border-image: `+u+` 30; border-image-source: `+u+`;
     list-style: `+u+` inside; list-style-image: `+u+`; cursor: `+u+`, auto; mask-image: `+u+`;
     -webkit-mask-image: `+u+`; clip-path: `+u+`; shape-outside: image-set("/design-assets/a.webp" 1x);
     --icon: `+u+`; content: `+u+`; color: red }
img, p { content: `+u+`; color: blue }
`)
	for _, want := range []string{
		"background: " + u + " no-repeat;", "background-image: " + u + ";", "-webkit-background-image: " + u + ";",
		"img[data-pd-icon=\"3\"],\n:root .card > img.icon {\n  content: " + u + ";",
		"color: red;", "color: blue;",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	for _, bad := range []string{"filter", "border-image", "list-style", "cursor", "mask", "clip-path", "shape-outside", "--icon"} {
		if strings.Contains(out, bad) {
			t.Errorf("%s with a URL kept:\n%s", bad, out)
		}
	}
	if strings.Count(out, "content:") != 1 {
		t.Errorf("content:url() kept outside an img-only rule:\n%s", out)
	}

	// Without a URL those properties are fine.
	out = mustSanitize(t, `.c { filter: blur(2px); list-style: disc inside; cursor: pointer; --icon: 1px; content: "x" }`)
	for _, want := range []string{"filter: blur(2px);", "list-style: disc inside;", "cursor: pointer;", "--icon: 1px;", `content: "x";`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}

	// The scoped designer icon swap keeps its image.
	scoped := Scope(`img[data-pd-icon="2"]{content:`+u+`}`, "#paper-page-3")
	if want := `#paper-page-3 img[data-pd-icon="2"]{content:` + u + `}`; scoped != want {
		t.Errorf("Scope = %s, want %s", scoped, want)
	}
}
