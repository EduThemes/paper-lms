// Package designdoc validates the Page Designer's JSON documents (a page's
// design, a course's look, a saved colour scheme) and holds the HTML
// helpers the Common Cartridge export uses on designed pages.
// See docs/page-designer/README.md.
package designdoc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Size limits from the contract.
const (
	MaxDesignBytes = 256 * 1024
	MaxLookBytes   = 16 * 1024
	// MaxSchemeNameRunes caps a saved colour scheme's name.
	MaxSchemeNameRunes = 60
)

var (
	reDesignID  = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)
	reThemeID   = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)
	rePaletteID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)
	reHexColor  = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	// Paths under /design-assets/: no "..", no empty segments, no query.
	reDesignAsset = regexp.MustCompile(`^/design-assets/[A-Za-z0-9._@-]+(?:/[A-Za-z0-9._@-]+)*$`)
	reFileURL     = regexp.MustCompile(`^/api/v1/files/([0-9]{1,18})/download$`)
)

// ValidationError is a client error (the API answers 422 / 400).
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

func invalid(format string, args ...interface{}) error {
	return &ValidationError{Msg: fmt.Sprintf(format, args...)}
}

// IsValidation reports whether err is a ValidationError.
func IsValidation(err error) bool {
	var v *ValidationError
	return errors.As(err, &v)
}

// decodeObject parses raw as one JSON object, keeping its members raw.
func decodeObject(raw []byte, what string) (map[string]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, invalid("%s must be a JSON object", what)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &obj); err != nil {
		return nil, invalid("%s is not valid JSON", what)
	}
	return obj, nil
}

// isVersion1 reports whether raw is the JSON number 1.
func isVersion1(raw json.RawMessage) bool {
	f, err := strconv.ParseFloat(string(bytes.TrimSpace(raw)), 64)
	return err == nil && f == 1
}

func stringMember(obj map[string]json.RawMessage, key string) (string, bool) {
	raw, ok := obj[key]
	if !ok {
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	return s, true
}

// compact returns raw as compact JSON (jsonb normalizes it anyway; this
// keeps the stored bytes small and the size check honest).
func compact(raw []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ValidateDesign checks a page design (the engine's PageDoc) and returns it
// cleaned. It must be a JSON object, at most MaxDesignBytes, with
// "version": 1 and a string "id" matching ^[A-Za-z0-9_-]{1,100}$; anything
// else is a ValidationError. The rest is cleaned rather than refused, against
// the engine's schema (web/src/designer/engine/types.ts), because the
// designer writes some fields straight into the page:
//
//   - Sections of an unknown type are dropped, as are members a section type
//     does not define; at most MaxDesignSections sections and
//     MaxDesignListItems entries per list are kept.
//   - RichText fields (welcome/callout/text body, instructor bio) go through
//     SanitizeRichText: p, strong, em, a[href], ul, ol, li, br.
//   - Link targets (quick links, schedule and this-week items) are kept only
//     if SafeHref accepts them; image addresses (banner customSrc, instructor
//     photoSrc) only if SafeImageSrc does.
//   - "palette" is kept only if it passes ValidatePalette.
//   - Plain strings lose control characters and are cut to 200 (labels) or
//     2,000 characters; "customCss" stays text (it is sanitized when it
//     becomes the page stylesheet), cut to csssanitize.MaxBytes.
//   - Unknown top-level members are dropped.
//
// A design the engine produced comes back with the same values, so the
// page's data-pd-hash still matches it. Cleaning is idempotent.
func ValidateDesign(raw []byte) ([]byte, error) {
	if len(raw) > MaxDesignBytes {
		return nil, invalid("design is larger than 256 KB")
	}
	obj, err := decodeObject(raw, "design")
	if err != nil {
		return nil, err
	}
	if v, ok := obj["version"]; !ok || !isVersion1(v) {
		return nil, invalid("design.version must be 1")
	}
	id, ok := stringMember(obj, "id")
	if !ok || !reDesignID.MatchString(id) {
		return nil, invalid("design.id must be 1-100 letters, digits, '-' or '_'")
	}
	out, err := normalizeDesign(obj, id)
	if err != nil {
		return nil, invalid("design could not be read")
	}
	return out, nil
}

// LookColorKeys are the keys of courses.look colors / dark_colors.
var LookColorKeys = []string{
	"primary", "on_primary", "secondary", "on_secondary", "accent",
	"surface", "on_surface", "background", "text", "link",
}

var lookColorKeySet = func() map[string]bool {
	m := map[string]bool{}
	for _, k := range LookColorKeys {
		m[k] = true
	}
	return m
}()

var lookTopKeys = map[string]bool{
	"version": true, "theme_id": true, "palette_id": true,
	"colors": true, "dark_colors": true, "banner": true,
}

// FileInCourse reports whether a file id belongs to the course being
// edited (tenant- and course-scoped by the caller).
type FileInCourse func(fileID uint) bool

// ValidateLook checks a course look (courses.look): at most MaxLookBytes;
// "version" (when present) is 1; "theme_id" matches ^[a-z0-9-]{1,64}$;
// "palette_id" (optional) is a short identifier; "colors" / "dark_colors"
// map the LookColorKeys to #rrggbb; "banner" (optional, may be null) has a
// "src" and optional "src_2x" under /design-assets/ or
// /api/v1/files/<id>/download for a file fileOK accepts, and an optional
// string "alt". Unknown keys are refused. Returns the compacted document.
func ValidateLook(raw []byte, fileOK FileInCourse) ([]byte, error) {
	if len(raw) > MaxLookBytes {
		return nil, invalid("look is larger than 16 KB")
	}
	obj, err := decodeObject(raw, "look")
	if err != nil {
		return nil, err
	}
	for k := range obj {
		if !lookTopKeys[k] {
			return nil, invalid("look.%s is not a known field", k)
		}
	}
	if v, ok := obj["version"]; ok && !isVersion1(v) {
		return nil, invalid("look.version must be 1")
	}
	theme, ok := stringMember(obj, "theme_id")
	if !ok || !reThemeID.MatchString(theme) {
		return nil, invalid("look.theme_id must match ^[a-z0-9-]{1,64}$")
	}
	if _, present := obj["palette_id"]; present {
		if p, ok := stringMember(obj, "palette_id"); !ok || !rePaletteID.MatchString(p) {
			return nil, invalid("look.palette_id must be a short identifier")
		}
	}
	for _, key := range []string{"colors", "dark_colors"} {
		raw, present := obj[key]
		if !present || isJSONNull(raw) {
			continue
		}
		var colors map[string]json.RawMessage
		if err := json.Unmarshal(raw, &colors); err != nil {
			return nil, invalid("look.%s must be an object", key)
		}
		for name, v := range colors {
			if !lookColorKeySet[name] {
				return nil, invalid("look.%s.%s is not a known colour", key, name)
			}
			var c string
			if err := json.Unmarshal(v, &c); err != nil || !reHexColor.MatchString(c) {
				return nil, invalid("look.%s.%s must be a #rrggbb colour", key, name)
			}
		}
	}
	if raw, present := obj["banner"]; present && !isJSONNull(raw) {
		var banner map[string]json.RawMessage
		if err := json.Unmarshal(raw, &banner); err != nil {
			return nil, invalid("look.banner must be an object")
		}
		for k := range banner {
			switch k {
			case "src", "src_2x", "alt":
			default:
				return nil, invalid("look.banner.%s is not a known field", k)
			}
		}
		src, ok := stringMember(banner, "src")
		if !ok {
			return nil, invalid("look.banner.src is required")
		}
		if err := validateBannerPath(src, fileOK); err != nil {
			return nil, err
		}
		if _, present := banner["src_2x"]; present && !isJSONNull(banner["src_2x"]) {
			src2, ok := stringMember(banner, "src_2x")
			if !ok {
				return nil, invalid("look.banner.src_2x must be a string")
			}
			if err := validateBannerPath(src2, fileOK); err != nil {
				return nil, err
			}
		}
		if _, present := banner["alt"]; present {
			alt, ok := stringMember(banner, "alt")
			if !ok || utf8.RuneCountInString(alt) > 1000 {
				return nil, invalid("look.banner.alt must be a string of at most 1000 characters")
			}
		}
	}
	return compact(raw)
}

func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func validateBannerPath(p string, fileOK FileInCourse) error {
	if reDesignAsset.MatchString(p) && !strings.Contains(p, "..") {
		return nil
	}
	if m := reFileURL.FindStringSubmatch(p); m != nil {
		id, err := strconv.ParseUint(m[1], 10, 64)
		if err == nil && id > 0 && fileOK != nil && fileOK(uint(id)) {
			return nil
		}
		return invalid("look.banner file %s is not a file in this course", m[1])
	}
	return invalid("look.banner paths must be under /design-assets/ or /api/v1/files/<id>/download")
}

// LookFileIDs returns the file ids a look's banner points at.
func LookFileIDs(raw []byte) []uint {
	var look struct {
		Banner *struct {
			Src   string `json:"src"`
			Src2x string `json:"src_2x"`
		} `json:"banner"`
	}
	if json.Unmarshal(raw, &look) != nil || look.Banner == nil {
		return nil
	}
	var out []uint
	for _, p := range []string{look.Banner.Src, look.Banner.Src2x} {
		if m := reFileURL.FindStringSubmatch(p); m != nil {
			if id, err := strconv.ParseUint(m[1], 10, 64); err == nil {
				out = append(out, uint(id))
			}
		}
	}
	return out
}

// PaletteKeys are the members of the engine's Palette
// (web/src/designer/engine/types.ts).
var PaletteKeys = []string{
	"id", "name", "primary", "onPrimary", "secondary", "onSecondary",
	"accent", "surface", "onSurface", "background", "text", "link",
}

// ValidatePalette checks a saved palette: exactly the PaletteKeys, "id" a
// short identifier, "name" a string of at most 60 characters, every other
// member a #rrggbb colour. Returns the palette re-encoded with lower-case
// colours.
func ValidatePalette(raw []byte) ([]byte, error) {
	if len(raw) > 8*1024 {
		return nil, invalid("palette is too large")
	}
	obj, err := decodeObject(raw, "palette")
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(PaletteKeys))
	for k := range obj {
		known := false
		for _, pk := range PaletteKeys {
			if k == pk {
				known = true
				break
			}
		}
		if !known {
			return nil, invalid("palette.%s is not a known field", k)
		}
	}
	for _, k := range PaletteKeys {
		v, ok := stringMember(obj, k)
		if !ok {
			return nil, invalid("palette.%s is required and must be a string", k)
		}
		switch k {
		case "id":
			if !rePaletteID.MatchString(v) {
				return nil, invalid("palette.id must be 1-100 letters, digits, '-' or '_'")
			}
		case "name":
			if utf8.RuneCountInString(v) > MaxSchemeNameRunes {
				return nil, invalid("palette.name must be at most %d characters", MaxSchemeNameRunes)
			}
		default:
			if !reHexColor.MatchString(v) {
				return nil, invalid("palette.%s must be a #rrggbb colour", k)
			}
			v = strings.ToLower(v)
		}
		out[k] = v
	}
	return json.Marshal(out)
}
