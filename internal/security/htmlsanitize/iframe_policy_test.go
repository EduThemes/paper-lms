package htmlsanitize

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// G001: common education embeds keep their src (and their allow list).
func TestSanitize_KeepsEducationEmbeds(t *testing.T) {
	srcs := []string{
		"https://www.youtube.com/embed/abc123XYZ",
		"https://docs.google.com/presentation/d/e/2PACX-1/embed?start=false",
		"https://docs.google.com/document/d/1abc/preview",
		"https://docs.google.com/forms/d/e/1FAIpQ/viewform?embedded=true",
		"https://drive.google.com/file/d/1abc/preview",
		"https://school.hosted.panopto.com/Panopto/Pages/Embed.aspx?id=1234",
		"https://cdnapisec.kaltura.com/p/1/sp/100/embedIframeJs/uiconf_id/2/partner_id/1",
		"https://www.desmos.com/calculator/abcd1234",
		"https://phet.colorado.edu/sims/html/energy-skate-park/latest/energy-skate-park_en.html",
		"https://www.geogebra.org/material/iframe/id/abc",
		"https://padlet.com/embed/xyz",
		"https://quizlet.com/123/flashcards/embed",
		"https://district.h5p.com/content/1290/embed",
		"https://onedrive.live.com/embed?resid=1",
		"https://view.genial.ly/123",
		"/courses/12/external_tools/retrieve?url=https%3A%2F%2Ftool.example%2Flaunch",
	}
	for _, src := range srcs {
		in := `<iframe src="` + src + `" title="Lesson" allow="autoplay; fullscreen; encrypted-media" loading="lazy" referrerpolicy="strict-origin-when-cross-origin"></iframe>`
		out := Sanitize(in)
		if !strings.Contains(out, `src="`+strings.ReplaceAll(src, "&", "&amp;")+`"`) {
			t.Errorf("src stripped for %s\nout: %s", src, out)
		}
		for _, attr := range []string{`allow="autoplay; fullscreen; encrypted-media"`, `loading="lazy"`, `referrerpolicy="strict-origin-when-cross-origin"`} {
			if !strings.Contains(out, attr) {
				t.Errorf("%s dropped for %s\nout: %s", attr, src, out)
			}
		}
	}
}

func TestSanitize_RejectsLookalikeIframeHosts(t *testing.T) {
	srcs := []string{
		"https://attacker.example/login",
		"https://docs.google.com.attacker.example/document/",
		"https://www.youtube.com.attacker.example/embed/x",
		"https://attacker.example/?https://www.youtube.com/embed/x",
		"http://www.youtube.com/embed/abc", // https only
		"https://evilpanopto.com/Panopto/Pages/Embed.aspx",
		"https://kaltura.com.attacker.example/",
		"javascript:alert(1)//https://www.youtube.com/embed/",
		"/api/v1/users/self",
		"//attacker.example/courses/1/external_tools/retrieve?x",
		"/courses/1/external_tools/retrieve",
	}
	for _, src := range srcs {
		out := Sanitize(`<iframe src="` + src + `"></iframe>`)
		if strings.Contains(out, "src=") {
			t.Errorf("iframe src %q should have been stripped: %s", src, out)
		}
	}
}

func TestSanitize_IframeAllowAttributeIsValidated(t *testing.T) {
	keep := []string{
		"accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share",
		"fullscreen",
		"camera 'src'; microphone 'src'",
		"autoplay *;",
	}
	for _, v := range keep {
		out := Sanitize(`<iframe src="https://www.youtube.com/embed/x" allow="` + v + `"></iframe>`)
		if !strings.Contains(out, `allow="`) {
			t.Errorf("allow=%q should be kept: %s", v, out)
		}
	}
	drop := []string{
		"geolocation",
		"camera https://attacker.example",
		"autoplay\" onload=\"alert(1)",
		"fullscreen; payment",
	}
	for _, v := range drop {
		out := Sanitize(`<iframe src="https://www.youtube.com/embed/x" allow='` + v + `'></iframe>`)
		if strings.Contains(out, "allow=") || strings.Contains(out, "onload") {
			t.Errorf("allow=%q should be dropped: %s", v, out)
		}
	}
}

// The web client mirrors the allowlist (DOMPurify hook + editor iframe node).
// A host added on one side only would be stripped by the other.
func TestTrustedIframeHosts_MatchesWebClient(t *testing.T) {
	path := filepath.Join("..", "..", "..", "web", "src", "utils", "trustedIframeHosts.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("web client not present: %v", err)
	}
	var mirror struct {
		Hosts          []string `json:"hosts"`
		SameOriginPath string   `json:"sameOriginPath"`
	}
	if err := json.Unmarshal(raw, &mirror); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if strings.Join(mirror.Hosts, "\n") != strings.Join(TrustedIframeHosts, "\n") {
		t.Errorf("web/src/utils/trustedIframeHosts.json hosts differ from TrustedIframeHosts")
	}
	if mirror.SameOriginPath != sameOriginIframePath {
		t.Errorf("sameOriginPath differs: %q vs %q", mirror.SameOriginPath, sameOriginIframePath)
	}
}

// Every fragment must pin the end of the host with a "/" so a lookalike
// domain cannot match.
func TestTrustedIframeHosts_EndHostWithSlash(t *testing.T) {
	for _, h := range TrustedIframeHosts {
		if !strings.Contains(h, "/") {
			t.Errorf("host fragment %q does not end the host with /", h)
		}
		if TrustedIframeSrc.MatchString("https://" + strings.ReplaceAll(strings.Split(h, "/")[0], `\.`, ".") + ".attacker.example/") {
			t.Errorf("fragment %q matches a lookalike domain", h)
		}
	}
}

// The editor's media and equation nodes (G002, G007) survive the server
// sanitizer as written, so the player, its media id and the equation's
// LaTeX reach every display surface.
func TestSanitize_KeepsEditorMediaAndEquations(t *testing.T) {
	in := `<video src="/api/v1/media_objects/m-0123456789abcdef0123456789abcdef/stream" controls="controls" preload="metadata" data-media-id="m-0123456789abcdef0123456789abcdef" title="Lab" style="max-width: 100%"></video>` +
		`<audio src="https://cdn.example/a.mp3" controls="controls" preload="metadata"><track kind="captions" srclang="en" label="English" src="https://cdn.example/a.vtt"></audio>` +
		`<p><span class="math-tex" data-latex="\pi r^2" data-display="block">\[\pi r^2\]</span></p>`
	out := Sanitize(in)
	for _, want := range []string{
		`src="/api/v1/media_objects/m-0123456789abcdef0123456789abcdef/stream"`,
		`data-media-id="m-0123456789abcdef0123456789abcdef"`,
		`controls="controls"`, `preload="metadata"`, `max-width: 100%`,
		`kind="captions"`, `srclang="en"`, `label="English"`, `src="https://cdn.example/a.vtt"`,
		`class="math-tex"`, `data-latex="\pi r^2"`, `data-display="block"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("sanitized output lost %q\nout: %s", want, out)
		}
	}
}
