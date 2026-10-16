package designdoc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateDesign(t *testing.T) {
	ok := []string{
		`{"version":1,"id":"a1b2-c3_d4","themeId":"tide-pool","sections":[]}`,
		` {"version": 1.0, "id": "x"} `,
	}
	for _, in := range ok {
		out, err := ValidateDesign([]byte(in))
		if err != nil {
			t.Errorf("ValidateDesign(%s): %v", in, err)
			continue
		}
		if !json.Valid(out) || strings.Contains(string(out), " ") {
			t.Errorf("ValidateDesign(%s) = %s, want compact JSON", in, out)
		}
	}
	bad := []string{
		``, `null`, `[]`, `"x"`, `{`,
		`{"id":"x"}`,               // no version
		`{"version":2,"id":"x"}`,   // wrong version
		`{"version":"1","id":"x"}`, // version must be a number
		`{"version":1}`,            // no id
		`{"version":1,"id":7}`,     // id must be a string
		`{"version":1,"id":"a b"}`, // id charset
		`{"version":1,"id":"<x>"}`, // id charset
		`{"version":1,"id":"` + strings.Repeat("a", 101) + `"}`,
	}
	for _, in := range bad {
		if _, err := ValidateDesign([]byte(in)); err == nil || !IsValidation(err) {
			t.Errorf("ValidateDesign(%q) accepted (err=%v)", in, err)
		}
	}
	big := `{"version":1,"id":"x","pad":"` + strings.Repeat("a", MaxDesignBytes) + `"}`
	if _, err := ValidateDesign([]byte(big)); err == nil {
		t.Error("oversized design accepted")
	}
}

func TestValidateLook(t *testing.T) {
	inCourse := func(id uint) bool { return id == 42 }
	ok := []string{
		`{"version":1,"theme_id":"tide-pool","palette_id":"default",
		  "colors":{"primary":"#12355b","on_primary":"#FFFFFF","link":"#0a0b0c"},
		  "dark_colors":{"background":"#000000"},
		  "banner":{"src":"/design-assets/themes/tide-pool/banner.webp","src_2x":"/design-assets/themes/tide-pool/banner@2x.webp","alt":""}}`,
		`{"theme_id":"x","banner":null}`,
		`{"theme_id":"x","banner":{"src":"/api/v1/files/42/download"}}`,
	}
	for _, in := range ok {
		if _, err := ValidateLook([]byte(in), inCourse); err != nil {
			t.Errorf("ValidateLook(%s): %v", in, err)
		}
	}
	bad := map[string]string{
		"not object":        `[]`,
		"version":           `{"version":2,"theme_id":"x"}`,
		"no theme":          `{"colors":{}}`,
		"theme charset":     `{"theme_id":"Tide Pool"}`,
		"unknown key":       `{"theme_id":"x","script":"alert(1)"}`,
		"colour format":     `{"theme_id":"x","colors":{"primary":"red"}}`,
		"colour short":      `{"theme_id":"x","colors":{"primary":"#fff"}}`,
		"colour key":        `{"theme_id":"x","colors":{"evil":"#ffffff"}}`,
		"colour css":        `{"theme_id":"x","dark_colors":{"text":"#fff;background:url(x)"}}`,
		"banner external":   `{"theme_id":"x","banner":{"src":"https://evil.example/x.webp"}}`,
		"banner traversal":  `{"theme_id":"x","banner":{"src":"/design-assets/../api/v1/users"}}`,
		"banner other file": `{"theme_id":"x","banner":{"src":"/api/v1/files/43/download"}}`,
		"banner 2x":         `{"theme_id":"x","banner":{"src":"/design-assets/a.webp","src_2x":"javascript:alert(1)"}}`,
		"banner no src":     `{"theme_id":"x","banner":{"alt":"x"}}`,
		"banner key":        `{"theme_id":"x","banner":{"src":"/design-assets/a.webp","onload":"x"}}`,
		"palette id":        `{"theme_id":"x","palette_id":"a b"}`,
	}
	for name, in := range bad {
		if _, err := ValidateLook([]byte(in), inCourse); err == nil {
			t.Errorf("%s: ValidateLook(%s) accepted", name, in)
		}
	}
	big := `{"theme_id":"x","banner":{"src":"/design-assets/a.webp","alt":"` + strings.Repeat("a", MaxLookBytes) + `"}}`
	if _, err := ValidateLook([]byte(big), inCourse); err == nil {
		t.Error("oversized look accepted")
	}
	// No file checker: file banners are refused.
	if _, err := ValidateLook([]byte(`{"theme_id":"x","banner":{"src":"/api/v1/files/42/download"}}`), nil); err == nil {
		t.Error("file banner accepted without a checker")
	}
}

func TestLookFileIDs(t *testing.T) {
	ids := LookFileIDs([]byte(`{"theme_id":"x","banner":{"src":"/api/v1/files/7/download","src_2x":"/design-assets/a.webp"}}`))
	if len(ids) != 1 || ids[0] != 7 {
		t.Errorf("LookFileIDs = %v", ids)
	}
}

func validPalette() map[string]string {
	return map[string]string{
		"id": "custom-1", "name": "Ocean", "primary": "#12355B", "onPrimary": "#ffffff",
		"secondary": "#2a6f97", "onSecondary": "#ffffff", "accent": "#f5c518", "surface": "#eef6fb",
		"onSurface": "#1f2328", "background": "#ffffff", "text": "#1f2328", "link": "#0b5cad",
	}
}

func TestValidatePalette(t *testing.T) {
	raw, _ := json.Marshal(validPalette())
	out, err := ValidatePalette(raw)
	if err != nil {
		t.Fatalf("ValidatePalette: %v", err)
	}
	var got map[string]string
	_ = json.Unmarshal(out, &got)
	if got["primary"] != "#12355b" {
		t.Errorf("primary not lower-cased: %q", got["primary"])
	}

	mutate := map[string]func(m map[string]string){
		"missing key":  func(m map[string]string) { delete(m, "link") },
		"extra key":    func(m map[string]string) { m["shadow"] = "#000000" },
		"bad colour":   func(m map[string]string) { m["accent"] = "rgb(0,0,0)" },
		"long name":    func(m map[string]string) { m["name"] = strings.Repeat("n", 61) },
		"bad id":       func(m map[string]string) { m["id"] = "<x>" },
		"empty colour": func(m map[string]string) { m["text"] = "" },
	}
	for name, f := range mutate {
		m := validPalette()
		f(m)
		raw, _ := json.Marshal(m)
		if _, err := ValidatePalette(raw); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := ValidatePalette([]byte(`{"id":1}`)); err == nil {
		t.Error("non-string member accepted")
	}
}

func TestFlattenPDVars(t *testing.T) {
	// Mirrors the engine's flattenDesignVars
	// (web/src/designer/engine/render/design.ts).
	cases := map[string]string{
		// The engine's form.
		`<div style="background:var(--pd-c1, #12355b);color:var(--pd-c2,#fff)">x</div>`: `<div style="background:#12355b;color:#fff">x</div>`,
		`<p style='border:1px solid var(--pd-c3, #abcdef80)'>y</p>`:                     `<p style='border:1px solid #abcdef80'>y</p>`,
		// Other vars stay.
		`<p style="font-family:var(--reading-font-family, Georgia)">z</p>`: `<p style="font-family:var(--reading-font-family, Georgia)">z</p>`,
		// Non-hex fallbacks are left alone rather than broken.
		`<p style="color:var(--pd-c1, rgb(1,2,3))">w</p>`: `<p style="color:var(--pd-c1, rgb(1,2,3))">w</p>`,
		`<p style="color:var(--pd-c1, red)">w</p>`:        `<p style="color:var(--pd-c1, red)">w</p>`,
		// Upper-case names are not the engine's.
		`<p style="color:var(--pd-C1, #010203)">v</p>`: `<p style="color:var(--pd-C1, #010203)">v</p>`,
	}
	for in, want := range cases {
		if got := FlattenPDVars(in); got != want {
			t.Errorf("FlattenPDVars(%s)\n got %s\nwant %s", in, got, want)
		}
	}
}

func TestAbsolutizeDesignURLs(t *testing.T) {
	base := "https://paper.example.edu/"
	in := `<img src="/design-assets/themes/a/banner.webp" srcset="/design-assets/themes/a/b.webp 1x, /design-assets/themes/a/b@2x.webp 2x">` +
		`<img src='/api/v1/design/icons/outline/star.svg?fg=000000&amp;bg=ffffff'>` +
		`<div style="background:url(/design-assets/x.webp)"></div>` +
		`<div style="background:url(&quot;/design-assets/y.webp&quot;)"></div>` +
		`<a href="/courses/1/pages/x">keep</a><img src="https://cdn.example.com/design-assets/z.webp">` +
		`<p>/design-assets/ in text stays put? no: only after a delimiter</p>`
	got := AbsolutizeDesignURLs(in, base)
	for _, want := range []string{
		`src="https://paper.example.edu/design-assets/themes/a/banner.webp"`,
		`srcset="https://paper.example.edu/design-assets/themes/a/b.webp 1x, https://paper.example.edu/design-assets/themes/a/b@2x.webp 2x"`,
		`src='https://paper.example.edu/api/v1/design/icons/outline/star.svg?fg=000000&amp;bg=ffffff'`,
		`url(https://paper.example.edu/design-assets/x.webp)`,
		`url(&quot;https://paper.example.edu/design-assets/y.webp&quot;)`,
		`href="/courses/1/pages/x"`,
		`src="https://cdn.example.com/design-assets/z.webp"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in\n%s", want, got)
		}
	}
	if AbsolutizeDesignURLs(in, "") != in || AbsolutizeDesignURLs(in, "javascript:alert(1)") != in {
		t.Error("an empty or non-http base must leave the HTML alone")
	}
	// Round trip.
	if back := RelativizeDesignURLs(got, base); !strings.Contains(back, `src="/design-assets/themes/a/banner.webp"`) ||
		!strings.Contains(back, `src='/api/v1/design/icons/outline/star.svg`) ||
		!strings.Contains(back, `https://cdn.example.com/design-assets/z.webp`) {
		t.Errorf("RelativizeDesignURLs did not undo AbsolutizeDesignURLs:\n%s", back)
	}
}

func TestExportPageHTML(t *testing.T) {
	in := `<div id="pd-x" style="background:var(--pd-c1, #12355b)"><img src="/api/v1/design/icons/duotone/star.svg?fg=ffffff&amp;bg=12355b" alt=""></div>`
	want := `<div id="pd-x" style="background:#12355b"><img src="https://lms.example.org/api/v1/design/icons/duotone/star.svg?fg=ffffff&amp;bg=12355b" alt=""></div>`
	if got := ExportPageHTML(in, "https://lms.example.org"); got != want {
		t.Errorf("ExportPageHTML\n got %s\nwant %s", got, want)
	}
}
