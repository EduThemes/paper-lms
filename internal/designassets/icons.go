// Package designassets renders the Page Designer's icon SVGs for
// GET /api/v1/design/icons/:style/:name.svg.
//
// RenderIconSVG is a port of renderIconSvg in
// web/src/designer/engine/icons.ts and must stay byte-identical to it: the
// designer previews icons with the TypeScript engine, and saved pages and
// exported cartridges load them from this route. icon-nodes.json (Lucide
// geometry, ISC licence, from lucide-static as pinned in web/package.json)
// and icon-aliases.json are copies of what the engine imports; refresh
// them, and testdata/golden.json, with `make designer-icons`.
package designassets

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

//go:embed icon-nodes.json
var iconNodesJSON []byte

//go:embed icon-aliases.json
var iconAliasesJSON []byte

// Styles are the four icon styles, in the engine's order.
var Styles = []string{"solid-circle", "soft-square", "outline", "duotone"}

// IsStyle reports whether s is one of Styles.
func IsStyle(s string) bool {
	for _, st := range Styles {
		if st == s {
			return true
		}
	}
	return false
}

// Fallback is drawn for unknown names (the engine's ICON_FALLBACK).
const Fallback = "circle-dashed"

// DefaultSize is the engine's default icon size in px.
const DefaultSize = 96

type iconSet struct {
	// inner[name] is the icon's child elements, already serialized the
	// way the engine serializes them (attributes in source order, "key"
	// dropped, values escaped).
	inner   map[string]string
	aliases map[string]string
}

var loadIcons = sync.OnceValues(func() (*iconSet, error) {
	inner, err := parseIconNodes(iconNodesJSON)
	if err != nil {
		return nil, fmt.Errorf("designassets: icon-nodes.json: %w", err)
	}
	aliases := map[string]string{}
	if err := json.Unmarshal(iconAliasesJSON, &aliases); err != nil {
		return nil, fmt.Errorf("designassets: icon-aliases.json: %w", err)
	}
	return &iconSet{inner: inner, aliases: aliases}, nil
})

func icons() *iconSet {
	set, err := loadIcons()
	if err != nil {
		// The files are embedded at build time; a parse failure is a
		// broken build, caught by the package tests.
		panic(err)
	}
	return set
}

// parseIconNodes reads {"name": [["tag", {"attr": "value", …}], …], …}
// keeping attribute order (encoding/json maps would lose it, and the
// engine writes attributes in source order).
func parseIconNodes(data []byte) (map[string]string, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	expect := func(want json.Delim) error {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		if d, ok := tok.(json.Delim); !ok || d != want {
			return fmt.Errorf("expected %q, got %v", want, tok)
		}
		return nil
	}
	str := func() (string, error) {
		tok, err := dec.Token()
		if err != nil {
			return "", err
		}
		switch v := tok.(type) {
		case string:
			return v, nil
		case json.Number:
			return jsNumber(v), nil
		case bool:
			return strconv.FormatBool(v), nil
		}
		return "", fmt.Errorf("expected a string, got %v", tok)
	}

	out := map[string]string{}
	if err := expect('{'); err != nil {
		return nil, err
	}
	for dec.More() {
		name, err := str()
		if err != nil {
			return nil, err
		}
		var b strings.Builder
		if err := expect('['); err != nil {
			return nil, err
		}
		for dec.More() {
			if err := expect('['); err != nil {
				return nil, err
			}
			tag, err := str()
			if err != nil {
				return nil, err
			}
			b.WriteString("<")
			b.WriteString(tag)
			if err := expect('{'); err != nil {
				return nil, err
			}
			for dec.More() {
				key, err := str()
				if err != nil {
					return nil, err
				}
				val, err := str()
				if err != nil {
					return nil, err
				}
				if key == "key" {
					continue
				}
				b.WriteString(" ")
				b.WriteString(key)
				b.WriteString(`="`)
				b.WriteString(esc(val))
				b.WriteString(`"`)
			}
			if err := expect('}'); err != nil {
				return nil, err
			}
			if err := expect(']'); err != nil {
				return nil, err
			}
			b.WriteString("/>")
		}
		if err := expect(']'); err != nil {
			return nil, err
		}
		out[name] = b.String()
	}
	if err := expect('}'); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing data")
	}
	return out, nil
}

// jsNumber formats a JSON number the way JavaScript's String(n) does for
// the plain magnitudes icon geometry uses.
func jsNumber(n json.Number) string {
	f, err := n.Float64()
	if err != nil {
		return n.String()
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// esc is the engine's esc(): & < > " escaped.
var escReplacer = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

func esc(s string) string { return escReplacer.Replace(s) }

var reNonName = regexp.MustCompile(`[^a-z0-9-]`)

// cleanName is the engine's String(name).toLowerCase().replace(/[^a-z0-9-]/g, ”).
func cleanName(name string) string {
	return reNonName.ReplaceAllString(strings.ToLower(name), "")
}

// ResolveIcon returns the canonical Lucide name for a name or alias, and
// false when there is none (the engine's resolveIcon).
func ResolveIcon(name string) (string, bool) {
	set := icons()
	k := cleanName(name)
	if _, ok := set.inner[k]; ok {
		return k, true
	}
	if a, ok := set.aliases[k]; ok {
		return a, true
	}
	return "", false
}

// IconNames lists every icon name, sorted.
func IconNames() []string {
	set := icons()
	names := make([]string, 0, len(set.inner))
	for n := range set.inner {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// jsTrim strips what JavaScript's String.prototype.trim strips.
func jsTrim(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		switch r {
		case '\t', '\n', '\v', '\f', '\r', ' ', '\u00a0', '\u1680', '\u2028', '\u2029',
			'\u202f', '\u205f', '\u3000', '\ufeff':
			return true
		}
		return r >= '\u2000' && r <= '\u200a'
	})
}

var (
	reHex3 = regexp.MustCompile(`^[0-9a-f]{3}$`)
	reHex6 = regexp.MustCompile(`^[0-9a-f]{6}$`)
)

// NormHex is the engine's normHex: "#abc", "abc", "#AABBCC" → "#aabbcc";
// "" when the input is not a hex colour.
func NormHex(c string) string {
	if c == "" {
		return ""
	}
	h := strings.ToLower(strings.TrimPrefix(jsTrim(c), "#"))
	if reHex3.MatchString(h) {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	if !reHex6.MatchString(h) {
		return ""
	}
	return "#" + h
}

func hexToRGB(c string) [3]float64 {
	h := NormHex(c)
	if h == "" {
		h = "#000000"
	}
	var out [3]float64
	for i := 0; i < 3; i++ {
		v, _ := strconv.ParseUint(h[1+2*i:3+2*i], 16, 8)
		out[i] = float64(v)
	}
	return out
}

// jsRound is JavaScript's Math.round (halves round up).
func jsRound(x float64) float64 {
	return math.Floor(x + 0.5)
}

// mix is the engine's linear sRGB mix: t=0 → a, t=1 → b.
func mix(a, b string, t float64) string {
	x, y := hexToRGB(a), hexToRGB(b)
	var sb strings.Builder
	sb.WriteByte('#')
	for i := 0; i < 3; i++ {
		v := jsRound(x[i] + (y[i]-x[i])*t)
		v = math.Max(0, math.Min(255, v))
		fmt.Fprintf(&sb, "%02x", int(v))
	}
	return sb.String()
}

// fmtNum is the engine's fmt: rounded to 3 decimals, printed like
// JavaScript's String(n).
func fmtNum(n float64) string {
	return strconv.FormatFloat(jsRound(n*1000)/1000, 'f', -1, 64)
}

// RenderIconSVG renders one icon at DefaultSize. fg and bg are colours as
// NormHex accepts them; invalid ones fall back to #1f2328 / #ffffff. An
// unknown style renders solid-circle and an unknown name Fallback, as in
// the engine.
func RenderIconSVG(name, style, fg, bg string) string {
	return RenderIconSVGSize(name, style, fg, bg, DefaultSize)
}

// RenderIconSVGSize is RenderIconSVG at a given size (clamped to 8..1024).
func RenderIconSVGSize(name, style, fg, bg string, size float64) string {
	S := int(math.Max(8, math.Min(1024, jsRound(size))))
	if !IsStyle(style) {
		style = "solid-circle"
	}
	if fg = NormHex(fg); fg == "" {
		fg = "#1f2328"
	}
	if bg = NormHex(bg); bg == "" {
		bg = "#ffffff"
	}
	set := icons()
	key, ok := ResolveIcon(name)
	if !ok {
		key = Fallback
	}
	inner := set.inner[key] // a dangling alias renders an empty glyph, as in the engine

	s := float64(S)
	ss := strconv.Itoa(S)
	back := ""
	glyph := s * 0.5
	switch style {
	case "solid-circle":
		back = `<circle cx="` + fmtNum(s/2) + `" cy="` + fmtNum(s/2) + `" r="` + fmtNum(s/2) + `" fill="` + bg + `"/>`
	case "soft-square":
		back = `<rect width="` + ss + `" height="` + ss + `" rx="` + fmtNum(s*0.24) + `" fill="` + bg + `"/>`
	case "outline":
		sw := math.Max(1.5, s*0.045)
		back = `<circle cx="` + fmtNum(s/2) + `" cy="` + fmtNum(s/2) + `" r="` + fmtNum(s/2-sw/2) + `" fill="` + bg +
			`" stroke="` + fg + `" stroke-width="` + fmtNum(sw) + `"/>`
		glyph = s * 0.46
	case "duotone":
		back = `<rect width="` + ss + `" height="` + ss + `" rx="` + fmtNum(s*0.3) + `" fill="` + bg + `"/>` +
			`<circle cx="` + fmtNum(s*0.6) + `" cy="` + fmtNum(s*0.42) + `" r="` + fmtNum(s*0.22) + `" fill="` + mix(fg, bg, 0.72) + `"/>`
	}
	off := (s - glyph) / 2
	k := glyph / 24
	return `<svg xmlns="http://www.w3.org/2000/svg" width="` + ss + `" height="` + ss + `" viewBox="0 0 ` + ss + ` ` + ss + `">` +
		back +
		`<g transform="translate(` + fmtNum(off) + ` ` + fmtNum(off) + `) scale(` + fmtNum(k) + `)" fill="none" stroke="` + fg +
		`" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` + inner + `</g>` +
		`</svg>`
}
