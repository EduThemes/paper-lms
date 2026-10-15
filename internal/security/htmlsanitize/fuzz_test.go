package htmlsanitize

import (
	"regexp"
	"strings"
	"testing"
)

var fixedPosition = regexp.MustCompile(`position:\s*(-[a-z]+-)?(fixed|sticky)`)

// FuzzInlineStyle checks that any style attribute value sanitizes to a
// stable result with no escapes, fixed positioning, or url() outside the
// URL rule.
func FuzzInlineStyle(f *testing.F) {
	for _, s := range []string{
		"color: var(--pd-c1, #fff)",
		"position: fixed; top: 0",
		`background: url("javascript:alert(1)")`,
		`background: \75 rl(x)`,
		"z-index: 100; z-index: 101",
		"font: position: fixed",
		`font-family: "a;b"; color: red`,
		"background-image: image-set(url(/design-assets/a@2x.webp) 2x)",
		"width: calc(1px /* x */ + 2px)",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, style string) {
		if len(style) > 2048 {
			return
		}
		in := `<div style="` + strings.ReplaceAll(style, `"`, "&quot;") + `">x</div>`
		out := Sanitize(in)
		if again := Sanitize(out); again != out {
			t.Fatalf("not idempotent\nin:     %q\nfirst:  %q\nsecond: %q", in, out, again)
		}
		lower := strings.ToLower(out)
		for _, bad := range []string{`\`, "&#92;", "expression(", "javascript:", "/*"} {
			if strings.Contains(lower, bad) {
				t.Fatalf("output contains %q\nin:  %q\nout: %q", bad, in, out)
			}
		}
		if fixedPosition.MatchString(lower) {
			t.Fatalf("fixed/sticky position kept\nin:  %q\nout: %q", in, out)
		}
	})
}
