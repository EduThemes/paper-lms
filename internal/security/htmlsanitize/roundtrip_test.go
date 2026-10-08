package htmlsanitize

import (
	"strings"
	"testing"
)

// TestSanitize_CanvasRoundTripMarkup covers the rce-editor parity gaps: the
// markup the editor and imported Canvas content rely on survives a save.
func TestSanitize_CanvasRoundTripMarkup(t *testing.T) {
	cases := []struct {
		name        string
		in          string
		mustContain []string
	}{
		{
			name:        "G000 colour, size and highlight on spans",
			in:          `<p><span style="color: #e03e2d; font-size: 18pt; background-color: rgb(255, 255, 0)">x</span></p>`,
			mustContain: []string{"color: #e03e2d", "font-size: 18pt", "background-color: rgb(255, 255, 0)"},
		},
		{
			name:        "G000 legacy font element",
			in:          `<font color="#ff0000" size="4" face="Arial, sans-serif">old</font>`,
			mustContain: []string{"<font", `color="#ff0000"`, `size="4"`, `face="Arial, sans-serif"`},
		},
		{
			name:        "G013 role and aria attributes",
			in:          `<div role="region" aria-label="Unit intro" aria-describedby="d1"><span aria-hidden="true">*</span></div>`,
			mustContain: []string{`role="region"`, `aria-label="Unit intro"`, `aria-describedby="d1"`, `aria-hidden="true"`},
		},
		{
			name:        "G006/G013 decorative image",
			in:          `<img src="/courses/1/files/2/download" alt="" role="presentation" data-decorative="true">`,
			mustContain: []string{`role="presentation"`, `data-decorative="true"`, `alt=""`},
		},
		{
			name:        "G014 tel link",
			in:          `<a href="tel:+15555550100">Call the office</a>`,
			mustContain: []string{`href="tel:+15555550100"`},
		},
		{
			name:        "G015 table caption, scope and resized column widths",
			in:          `<table><caption>Grades</caption><colgroup><col style="width: 120px" width="120"></colgroup><tbody><tr><th scope="col" colwidth="120">Name</th><td colwidth="120,80">x</td></tr></tbody></table>`,
			mustContain: []string{"<caption>Grades</caption>", `scope="col"`, `colwidth="120"`, `colwidth="120,80"`, `width="120"`},
		},
		{
			name:        "G003 direction, sub/sup, details, ordered list start",
			in:          `<p dir="rtl" lang="ar">x</p><p>H<sub>2</sub>O x<sup>2</sup></p><details open><summary>More</summary><p>y</p></details><ol start="3"><li>z</li></ol>`,
			mustContain: []string{`dir="rtl"`, "<sub>2</sub>", "<sup>2</sup>", "<details open", "<summary>More</summary>", `start="3"`},
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
			if again := Sanitize(out); again != out {
				t.Errorf("not idempotent\nfirst:  %q\nsecond: %q", out, again)
			}
		})
	}
}

// TestSanitize_AccessibilityAndLegacyAttrsStayInert makes sure the widened
// attribute set did not open a script or styling channel.
func TestSanitize_AccessibilityAndLegacyAttrsStayInert(t *testing.T) {
	cases := []struct {
		name           string
		in             string
		mustNotContain []string
	}{
		{"aria value cannot break out of the attribute", `<span aria-label='x" onmouseover="alert(1)'>y</span>`, []string{`onmouseover="`}},
		{"role with markup rejected", `<div role="button onclick=alert(1)">x</div>`, []string{"onclick"}},
		{"font color with css expression rejected", `<font color="expression(alert(1))">x</font>`, []string{"expression"}},
		{"font face with markup rejected", `<font face="x<script>">x</font>`, []string{"<script"}},
		{"colwidth with junk rejected", `<table><tr><td colwidth="1;background:url(x)">x</td></tr></table>`, []string{"background", "url("}},
		{"javascript link still stripped", `<a href="javascript:alert(1)">x</a>`, []string{"javascript:"}},
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
