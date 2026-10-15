package htmlsanitize

import (
	"strings"
	"testing"
)

func TestCSSValueOK(t *testing.T) {
	good := []string{
		"red", "#fff", "#12355bcc", "0", "-1px", "1.5em", "50%", "auto",
		"var(--pd-c1, #fff)",
		"var(--pd-c1)",
		"var(--A_b-9, var(--b, rgb(0 0 0 / 50%)))",
		"repeat(auto-fill, minmax(min(100%, max(180px, calc((100% - 48px) / 4))), 1fr))",
		"calc(100% - 2 * var(--gap, 8px))",
		"clamp(1rem, 0.5rem + 2vw, 2rem)",
		`"Open Sans", 'Noto Sans', sans-serif`,
		`"hero hero" "side main"`,
		"[full-start] 1fr [full-end]",
		"linear-gradient(to right, #000 0%, transparent 100%)",
		"-webkit-linear-gradient(top, #000, #fff)",
		"repeating-conic-gradient(from 0deg at 50% 50%, red 0 10deg, blue 10deg 20deg)",
		"url(/design-assets/themes/tide-pool/banner@2x.webp)",
		`url("/api/v1/files/7/download")`,
		"url( 'https://cdn.example.com/a/b.png?w=100&h=100' )",
		"url(/api/v1/design/icons/outline/book-open.svg?fg=000000&bg=ffffff)",
		`image-set("/design-assets/a.webp" 1x, url(/design-assets/a@2x.webp) 2x)`,
		`-webkit-image-set(url(/design-assets/a.webp) 1x)`,
		`image-set("/design-assets/a.avif" type("image/avif"))`,
		"translate(-50%, -50%) rotate(45deg) scale3d(1, 1, 1)",
		"cubic-bezier(0.2, 0.8, 0.2, 1)",
		"steps(4, jump-end)",
		"linear(0, 0.25 25%, 1)",
		"drop-shadow(0 0 2px rgba(0, 0, 0, .5)) blur(2px)",
		"polygon(50% 0, 100% 50%, 50% 100%, 0 50%)",
		"circle(40% at 50% 50%)",
		"inset(10px round 8px)",
		"color-mix(in srgb, red 50%, blue)",
		"light-dark(#000, #fff)",
		"oklch(70% 0.1 200 / 0.5)",
		"counter(item) \". \"",
		"16 / 9",
		"1fr 2fr",
		"微软雅黑",
		"italic bold 12px/30px Georgia, serif",
	}
	for _, v := range good {
		if !CSSValueOK(v) {
			t.Errorf("CSSValueOK(%q) = false, want true", v)
		}
	}

	bad := []string{
		"",
		"red;",
		"red; position: fixed",
		"red}",
		"{",
		"red !important",
		"@import",
		"a@b",
		`"a@b"`,
		"<b>",
		`"</style>"`,
		`"a;b"`,
		"a > b",
		"\\72 ed",
		"r\\65 d",
		"red /* comment */",
		"red */",
		"expression(alert(1))",
		"EXPRESSION(alert(1))",
		"url(javascript:alert(1))",
		`url("javascript:alert(1)")`,
		"url( 'javascript:alert(1)' )",
		"url(JAVASCRIPT:alert(1))",
		"url(vbscript:x)",
		"url(data:image/png;base64,AAAA)",
		"url(data:image/svg+xml,<svg/onload=alert(1)>)",
		"url(http://example.com/a.png)",
		"url(//example.com/a.png)",
		"url(#frag)",
		"url(a.png)",
		"url(/courses/1/files/2/download)",
		"url(/api/v1/files/abc/download)",
		"url(/api/v1/users/self)",
		"url(/design-assets/../../etc/passwd)",
		"url(https://user:pw@example.com/a.png)",
		"url(https://example.com/a b.png)",
		"url(https://)",
		"url(https:///path)",
		"url()",
		"url(",
		`image-set("http://example.com/a.png" 1x)`,
		`image-set("javascript:alert(1)" 1x)`,
		`image-set(var(--x) 1x)`,
		`image-set("/design-assets/a.png" type("image/png;x"))`,
		"src(\"https://example.com/a.png\")",
		"element(#login)",
		"-moz-element(#login)",
		"attr(data-x)",
		"env(safe-area-inset-top)",
		"paint(worklet)",
		"anchor(--a top)",
		"-moz-binding",
		"behavior",
		"var(-x)",
		"var(x)",
		"var(--x y)",
		"var()",
		"var(--)",
		"calc(1px",
		"calc(1px))",
		"1px)",
		"[a",
		"a]",
		"(a]",
		`"unterminated`,
		"'a\nb'",
		"a:b",
		"progid:DXImageTransform.Microsoft.Alpha(opacity=50)",
		"a = b",
		"a | b",
		"a ~ b",
		"a & b",
		"U+0025-00FF",
		"<!--",
		"-->",
		"a\x00b",
		"a\x7fb",
		"a\u0085b",
		"\xff",
		strings.Repeat("a", MaxCSSValueBytes+1),
		strings.Repeat("calc(", 40) + "1" + strings.Repeat(")", 40),
		strings.Repeat("1 ", maxCSSValueTokens+1),
	}
	for _, v := range bad {
		if CSSValueOK(v) {
			t.Errorf("CSSValueOK(%q) = true, want false", v)
		}
	}
}

func TestCSSURLOK(t *testing.T) {
	good := []string{
		"https://example.com/a.png",
		"HTTPS://Example.com/a.png",
		"https://cdn.example.com:8443/a/b.png?x=1&y=2#z",
		"https://images.example.org/w_100,h_100/photo.jpg",
		"/api/v1/files/1/download",
		"/api/v1/files/123456/download?verifier=abc",
		"/api/v1/design/icons/solid-circle/star.svg",
		"/api/v1/design/icons/duotone/book-open.svg?fg=12355b&bg=ffffff",
		"/design-assets/themes/tide-pool/banner.webp",
		"/design-assets/themes/tide-pool/banner@2x.webp",
		"/design-assets/sprite.svg#icon-1",
	}
	for _, u := range good {
		if !CSSURLOK(u) {
			t.Errorf("CSSURLOK(%q) = false, want true", u)
		}
	}
	bad := []string{
		"", "http://example.com/a.png", "//example.com/a.png", "javascript:alert(1)",
		"data:image/png;base64,AAAA", "ftp://example.com/a", "https:example.com",
		"https://user@example.com/a", "https://exa mple.com/", "https://example.com/a'b",
		"https://example.com/a(b)", "https://example.com/a\\b", "https://example.com/<x>",
		"https://-bad-.example.com/", "https://example.com/é", "/api/v1/files/1",
		"/api/v1/files/1/download/../../users", "/api/v1/users/self", "/design-assets/",
		"/design-assets/a/../b.png", "/design-assets/a%2e%2e/b.png", "/api/v1/design/icons/x.svg",
		"#frag", "a.png", "./a.png", "/courses/1/files/2/download",
		"https://example.com/" + strings.Repeat("a", MaxCSSURLBytes),
	}
	for _, u := range bad {
		if CSSURLOK(u) {
			t.Errorf("CSSURLOK(%q) = true, want false", u)
		}
	}
}

func TestSrcsetValue(t *testing.T) {
	good := []string{
		"/a.png", "/a.png 1x", "/a.png 1x, /b.png 2x", "a.png 480w, b.png 960w",
		"https://example.com/a.png 1.5x", "/design-assets/t/banner@2x.webp 2x",
		"/a.png 1x,/b.png 2x", " /a.png 1x , /b.png 2x ", "http://example.com/a.png",
	}
	for _, v := range good {
		if !SrcsetValue.MatchString(v) {
			t.Errorf("srcset %q should match", v)
		}
	}
	bad := []string{
		"", "javascript:alert(1)", "/a.png 1x, javascript:alert(1) 2x", "data:image/png;base64,AA 1x",
		"//evil.example/a.png", "/a.png 1y", "/a.png 1x 2x", `/a.png" onerror="x`, "/a.png 1x,, /b.png",
		"vbscript:x", "/a\\b.png",
	}
	for _, v := range bad {
		if SrcsetValue.MatchString(v) {
			t.Errorf("srcset %q should not match", v)
		}
	}
}
