package htmlsanitize

import (
	"strings"
	"testing"
)

// The audit repro: an overlay positioned over the mail client's own UI.
// Sanitize keeps positioning (the in-app viewers contain it); the email
// policy must not.
func TestSanitizeEmail_StripsPositioning(t *testing.T) {
	in := `<div style="position:absolute;top:0;left:0;width:100%;height:100%;z-index:100;background:#fff;color:#c00">` +
		`Your password expired. <a href="https://example.test/login">Sign in</a></div>`
	if got := Sanitize(in); !strings.Contains(got, "position: absolute") {
		t.Fatalf("precondition: the content policy keeps positioning, got %s", got)
	}
	got := SanitizeEmail(in)
	for _, bad := range []string{"position", "top: ", "left: ", "z-index", "background: "} {
		if strings.Contains(got, bad) {
			t.Errorf("email body keeps %q: %s", bad, got)
		}
	}
	for _, keep := range []string{"width: 100%", "height: 100%", "color: #c00", `href="https://example.test/login"`, "Sign in"} {
		if !strings.Contains(got, keep) {
			t.Errorf("email body lost %q: %s", keep, got)
		}
	}
}

func TestSanitizeEmail_ConservativeStyles(t *testing.T) {
	cases := map[string]string{
		// Typography, colour, box model and alignment survive.
		`<p style="text-align:center;color:#123456;font-weight:bold;margin:0 auto;padding:4px;border:1px solid #ccc">x</p>`: `<p style="text-align: center; color: #123456; font-weight: bold; margin: 0 auto; padding: 4px; border: 1px solid #ccc">x</p>`,
		`<td style="background-color:#eee;vertical-align:top;width:50%">x</td>`:                                             `<td style="background-color: #eee; vertical-align: top; width: 50%">x</td>`,
		// Everything that could escape or overlay is dropped.
		`<p style="transform:translate(0,-500px)">x</p>`:                    `<p>x</p>`,
		`<p style="position:fixed;inset:0">x</p>`:                           `<p>x</p>`,
		`<p style="margin-top:-9999px;display:none">x</p>`:                  `<p style="margin-top: -9999px">x</p>`,
		`<p style="background-image:url(https://example.test/t.png)">x</p>`: `<p>x</p>`,
		`<p style="background-color:url(https://example.test/t.png)">x</p>`: `<p>x</p>`,
		`<p style="list-style-image:url(https://example.test/t.png)">x</p>`: `<p>x</p>`,
		`<p style="color:expression(alert(1))">x</p>`:                       `<p>x</p>`,
		`<p style="font-family:'a\\62 c'">x</p>`:                            `<p>x</p>`,
		`<img src="https://example.test/a.png" alt="A" style="opacity:0">`:  `<img src="https://example.test/a.png" alt="A">`,
		`<p onclick="alert(1)">x</p><script>alert(1)</script>`:              `<p>x</p>`,
	}
	for in, want := range cases {
		if got := SanitizeEmail(in); got != want {
			t.Errorf("SanitizeEmail(%s)\n got %s\nwant %s", in, got, want)
		}
	}
	if SanitizeEmail("") != "" {
		t.Error(`SanitizeEmail("") must be ""`)
	}
}

func TestCSSValueLoadsURL(t *testing.T) {
	for v, want := range map[string]bool{
		`url(https://example.test/a.png)`:       true,
		`url("/design-assets/a.webp")`:          true,
		`no-repeat center / cover url(/x)`:      true,
		`image-set("/a.webp" 1x, "/b.webp" 2x)`: true,
		`-webkit-image-set("/a.webp" 1x)`:       true,
		`linear-gradient(red, blue)`:            false,
		`drop-shadow(0 0 2px red)`:              false,
		`"url(x)"`:                              false,
		`var(--x)`:                              false,
		`pointer`:                               false,
	} {
		if got := CSSValueLoadsURL(v); got != want {
			t.Errorf("CSSValueLoadsURL(%q) = %v, want %v", v, got, want)
		}
	}
}
