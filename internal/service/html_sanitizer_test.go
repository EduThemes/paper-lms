package service

import (
	"strings"
	"testing"
)

// TestSanitizeHTML covers the defense-in-depth HTML sanitization policy.
// Each case is a small assertion against well-known XSS / injection vectors
// and a small set of "must preserve" cases that LMS content actually needs
// (KaTeX math, syntax highlighting classes, embedded YouTube/Vimeo, etc.).
func TestSanitizeHTML(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		mustNotContain   []string
		mustContain      []string
		exactEmpty       bool
	}{
		{
			name:           "script tag stripped",
			input:          `<p>hello</p><script>alert(1)</script>`,
			mustNotContain: []string{"<script", "alert(1)"},
			mustContain:    []string{"<p>hello</p>"},
		},
		{
			name:           "img onerror stripped",
			input:          `<img src="x" onerror="alert(1)">`,
			mustNotContain: []string{"onerror", "alert"},
		},
		{
			name:           "javascript: URL blocked on anchor",
			input:          `<a href="javascript:alert(1)">click</a>`,
			mustNotContain: []string{"javascript:", "alert(1)"},
		},
		{
			name:           "untrusted iframe src blocked",
			input:          `<iframe src="https://evil.example.com/x"></iframe>`,
			mustNotContain: []string{"evil.example.com"},
		},
		{
			name:        "youtube iframe preserved",
			input:       `<iframe src="https://www.youtube.com/embed/dQw4w9WgXcQ" width="560" height="315" allowfullscreen title="Demo"></iframe>`,
			mustContain: []string{"<iframe", `src="https://www.youtube.com/embed/dQw4w9WgXcQ"`},
		},
		{
			name:        "vimeo iframe preserved",
			input:       `<iframe src="https://player.vimeo.com/video/12345" width="640" height="360"></iframe>`,
			mustContain: []string{"<iframe", "player.vimeo.com/video/12345"},
		},
		{
			name:        "katex span class preserved",
			input:       `<span class="math-tex">$x^2$</span>`,
			mustContain: []string{`class="math-tex"`, "$x^2$"},
		},
		{
			name:        "data-* attribute preserved",
			input:       `<span data-mathml="<math>x</math>">x</span>`,
			mustContain: []string{"data-mathml"},
		},
		{
			name:        "plain paragraph preserved",
			input:       `<p>This is a paragraph.</p>`,
			mustContain: []string{"<p>This is a paragraph.</p>"},
		},
		{
			name:        "https anchor preserved",
			input:       `<a href="https://example.com">example</a>`,
			mustContain: []string{`href="https://example.com"`, ">example</a>"},
		},
		{
			name:           "style tag stripped",
			input:          `<style>body{display:none}</style><p>ok</p>`,
			mustNotContain: []string{"<style", "display:none"},
			mustContain:    []string{"<p>ok</p>"},
		},
		{
			name:           "onclick attribute stripped",
			input:          `<button onclick="steal()">x</button>`,
			mustNotContain: []string{"onclick", "steal()"},
		},
		{
			name:       "empty input returns empty",
			input:      "",
			exactEmpty: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := SanitizeHTML(tc.input)
			if tc.exactEmpty {
				if out != "" {
					t.Fatalf("expected empty output, got %q", out)
				}
				return
			}
			for _, s := range tc.mustNotContain {
				if strings.Contains(out, s) {
					t.Errorf("output should NOT contain %q\ninput:  %q\noutput: %q", s, tc.input, out)
				}
			}
			for _, s := range tc.mustContain {
				if !strings.Contains(out, s) {
					t.Errorf("output should contain %q\ninput:  %q\noutput: %q", s, tc.input, out)
				}
			}
		})
	}
}

// TestSanitizeHTML_Idempotent ensures a second pass through the sanitizer
// does not alter already-clean output.
func TestSanitizeHTML_Idempotent(t *testing.T) {
	in := `<p>hello <a href="https://example.com" rel="nofollow">link</a></p>`
	first := SanitizeHTML(in)
	second := SanitizeHTML(first)
	if first != second {
		t.Errorf("sanitizer not idempotent\nfirst:  %q\nsecond: %q", first, second)
	}
}
