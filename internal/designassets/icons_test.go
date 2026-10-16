package designassets

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type golden struct {
	IconCount     int               `json:"icon_count"`
	DigestColours [2]string         `json:"digest_colours"`
	Digests       map[string]string `json:"digests"`
	Samples       []struct {
		Name  string `json:"name"`
		Style string `json:"style"`
		FG    string `json:"fg"`
		BG    string `json:"bg"`
		SVG   string `json:"svg"`
	} `json:"samples"`
}

// testdata/golden.json is the TypeScript engine's output (make
// designer-icons); the Go port must match it byte for byte.
func loadGolden(t *testing.T) golden {
	t.Helper()
	data, err := os.ReadFile("testdata/golden.json")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var g golden
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatalf("parse golden: %v", err)
	}
	return g
}

func TestRenderIconSVG_MatchesEngineSamples(t *testing.T) {
	g := loadGolden(t)
	if len(g.Samples) == 0 {
		t.Fatal("golden has no samples")
	}
	for _, s := range g.Samples {
		got := RenderIconSVG(s.Name, s.Style, s.FG, s.BG)
		if got != s.SVG {
			t.Errorf("RenderIconSVG(%q, %q, %q, %q) differs from the engine\n got: %s\nwant: %s", s.Name, s.Style, s.FG, s.BG, got, s.SVG)
		}
	}
}

func TestRenderIconSVG_MatchesEngineForEveryIcon(t *testing.T) {
	g := loadGolden(t)
	names := IconNames()
	if len(names) != g.IconCount {
		t.Fatalf("icon count = %d, engine has %d (run make designer-icons)", len(names), g.IconCount)
	}
	for _, style := range Styles {
		h := sha256.New()
		for _, n := range names {
			h.Write([]byte(RenderIconSVG(n, style, g.DigestColours[0], g.DigestColours[1]) + "\n"))
		}
		if got := hex.EncodeToString(h.Sum(nil)); got != g.Digests[style] {
			t.Errorf("style %s: digest of all icons differs from the engine", style)
		}
	}
}

func TestResolveIcon(t *testing.T) {
	if n, ok := ResolveIcon("Star"); !ok || n != "star" {
		t.Errorf("Star → %q %v", n, ok)
	}
	if n, ok := ResolveIcon("smile"); !ok || n == "smile" && !strings.Contains(RenderIconSVG("smile", "outline", "000", "fff"), "<circle") {
		t.Errorf("alias smile → %q %v", n, ok)
	}
	if _, ok := ResolveIcon("definitely-not-an-icon"); ok {
		t.Error("unknown name resolved")
	}
	// Unknown names draw the fallback.
	if RenderIconSVG("definitely-not-an-icon", "outline", "000", "fff") != RenderIconSVG(Fallback, "outline", "000", "fff") {
		t.Error("unknown name does not render the fallback")
	}
}

func TestNormHex(t *testing.T) {
	for in, want := range map[string]string{
		"abc": "#aabbcc", "#ABC": "#aabbcc", " 12AB9f ": "#12ab9f", "\uFEFF#000000": "#000000",
		"": "", "zzz": "", "#12345": "", "\u0085abc": "",
	} {
		if got := NormHex(in); got != want {
			t.Errorf("NormHex(%q) = %q, want %q", in, got, want)
		}
	}
}
