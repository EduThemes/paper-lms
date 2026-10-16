package designdoc

import (
	"bytes"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/EduThemes/paper-lms/internal/security/csssanitize"
)

// Limits ValidateDesign applies while it cleans a design.
const (
	// MaxDesignSections caps a design's sections; later ones are dropped.
	MaxDesignSections = 60
	// MaxDesignListItems caps every list in a section (quick links,
	// objectives, schedule and this-week items).
	MaxDesignListItems = 100
	// maxLabelRunes caps identifiers, headings and other short labels.
	maxLabelRunes = 200
	// maxTextRunes caps every other plain-text field.
	maxTextRunes = 2000
	// maxTextBytes bounds a URL field before it is checked.
	maxTextBytes = 4096
	// maxRichBytes caps one RichText field before it is sanitized.
	maxRichBytes = 64 * 1024
)

// fieldKind is how ValidateDesign treats one member of a section or item.
type fieldKind int

const (
	fieldLabel fieldKind = iota // short plain text (maxLabelRunes)
	fieldText                   // plain text (maxTextRunes)
	fieldRich                   // RichText: SanitizeRichText
	fieldHref                   // link target: kept only if SafeHref
	fieldImage                  // image address: kept only if SafeImageSrc
)

// fieldSet maps a member name to its kind.
type fieldSet map[string]fieldKind

// listSpec describes a list member of a section: plain strings (item nil)
// or objects with the given members.
type listSpec struct {
	key  string
	item fieldSet // nil: a list of plain-text strings
}

// sectionSpec is the engine's schema for one section type
// (web/src/designer/engine/types.ts, Section).
type sectionSpec struct {
	fields fieldSet
	list   *listSpec
}

var (
	quickLinkFields = fieldSet{
		"id": fieldLabel, "label": fieldLabel, "moduleId": fieldLabel, "href": fieldHref,
		"icon": fieldLabel, "description": fieldText, "slot": fieldLabel,
	}
	scheduleItemFields = fieldSet{
		"id": fieldLabel, "title": fieldText, "dates": fieldLabel, "summary": fieldText, "href": fieldHref,
	}
	thisWeekItemFields = fieldSet{
		"id": fieldLabel, "label": fieldText, "href": fieldHref, "due": fieldLabel,
	}
)

var sectionSpecs = map[string]sectionSpec{
	"title":   {fields: fieldSet{"text": fieldText, "subtitle": fieldText}},
	"banner":  {fields: fieldSet{"imageId": fieldLabel, "customSrc": fieldImage, "alt": fieldText}},
	"welcome": {fields: fieldSet{"heading": fieldLabel, "body": fieldRich}},
	"quick-links": {
		fields: fieldSet{"heading": fieldLabel},
		list:   &listSpec{key: "links", item: quickLinkFields},
	},
	"instructor": {fields: fieldSet{
		"heading": fieldLabel, "name": fieldLabel, "role": fieldLabel, "email": fieldLabel,
		"officeHours": fieldText, "photoSrc": fieldImage, "photoAlt": fieldText, "bio": fieldRich,
	}},
	"callout": {fields: fieldSet{"variant": fieldLabel, "heading": fieldLabel, "body": fieldRich}},
	"objectives": {
		fields: fieldSet{"heading": fieldLabel},
		list:   &listSpec{key: "items"},
	},
	"schedule": {
		fields: fieldSet{"heading": fieldLabel},
		list:   &listSpec{key: "items", item: scheduleItemFields},
	},
	"this-week": {
		fields: fieldSet{"heading": fieldLabel, "title": fieldText, "dates": fieldLabel, "moduleId": fieldLabel},
		list:   &listSpec{key: "items", item: thisWeekItemFields},
	},
	"text":    {fields: fieldSet{"heading": fieldLabel, "body": fieldRich}},
	"divider": {fields: fieldSet{}},
}

// topLevelLabels are the PageDoc members kept as short labels.
var topLevelLabels = []string{"themeId", "paletteId", "layout", "kind", "locale"}

// optionLabels are the members of PageDoc.options.
var optionLabels = []string{"density", "headingStyle"}

// cleanText removes control characters (keeping tab, newline, carriage
// return and form feed) and cuts s to at most maxRunes runes. A value that
// needs neither is returned unchanged.
func cleanText(s string, maxRunes int) string {
	clean := true
	for _, r := range s {
		if isStrippedControl(r) {
			clean = false
			break
		}
	}
	if !clean {
		s = strings.Map(func(r rune) rune {
			if isStrippedControl(r) {
				return -1
			}
			return r
		}, s)
	}
	if utf8.RuneCountInString(s) > maxRunes {
		s = string([]rune(s)[:maxRunes])
	}
	return s
}

func isStrippedControl(r rune) bool {
	switch r {
	case '\t', '\n', '\r', '\f':
		return false
	}
	return r < 0x20 || (r >= 0x7f && r <= 0x9f)
}

// truncateBytes cuts s to at most n bytes without splitting a rune.
func truncateBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// rawString decodes raw as a JSON string.
func rawString(raw json.RawMessage) (string, bool) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	return s, true
}

// cleanField returns the cleaned value of one member, or false to drop it.
func cleanField(kind fieldKind, raw json.RawMessage) (string, bool) {
	s, ok := rawString(raw)
	if !ok {
		return "", false
	}
	switch kind {
	case fieldLabel:
		return cleanText(s, maxLabelRunes), true
	case fieldText:
		return cleanText(s, maxTextRunes), true
	case fieldRich:
		return SanitizeRichText(truncateBytes(s, maxRichBytes)), true
	case fieldHref:
		if len(s) > maxTextBytes || !SafeHref(s) {
			return "", false
		}
		return cleanText(s, maxTextRunes), true
	case fieldImage:
		if !SafeImageSrc(s) {
			return "", false
		}
		return s, true
	}
	return "", false
}

// cleanObject keeps the members of obj that fields lists, cleaned.
func cleanObject(obj map[string]json.RawMessage, fields fieldSet) map[string]any {
	out := make(map[string]any, len(fields))
	for key, kind := range fields {
		raw, present := obj[key]
		if !present {
			continue
		}
		if v, ok := cleanField(kind, raw); ok {
			out[key] = v
		}
	}
	return out
}

// cleanList cleans a section's list member: at most MaxDesignListItems
// entries, each a plain-text string or an object of spec.item. Entries of
// the wrong shape are dropped.
func cleanList(raw json.RawMessage, spec *listSpec) []any {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return []any{}
	}
	out := make([]any, 0, min(len(items), MaxDesignListItems))
	for _, it := range items {
		if len(out) == MaxDesignListItems {
			break
		}
		if spec.item == nil {
			if s, ok := rawString(it); ok {
				out = append(out, cleanText(s, maxTextRunes))
			}
			continue
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(it, &obj); err != nil || obj == nil {
			continue
		}
		out = append(out, cleanObject(obj, spec.item))
	}
	return out
}

// cleanSection returns the section cleaned against its type's schema, or
// false when the section is not an object or its type is unknown.
func cleanSection(raw json.RawMessage) (map[string]any, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return nil, false
	}
	typ, ok := stringMember(obj, "type")
	if !ok {
		return nil, false
	}
	spec, known := sectionSpecs[typ]
	if !known {
		return nil, false
	}
	out := cleanObject(obj, spec.fields)
	out["type"] = typ
	if id, ok := stringMember(obj, "id"); ok {
		out["id"] = cleanText(id, maxLabelRunes)
	}
	if spec.list != nil {
		if raw, present := obj[spec.list.key]; present {
			out[spec.list.key] = cleanList(raw, spec.list)
		}
	}
	return out, true
}

// normalizeDesign rebuilds a design (already checked for version and id)
// from the members the engine's PageDoc defines, cleaned. Unknown members
// are dropped.
func normalizeDesign(obj map[string]json.RawMessage, id string) ([]byte, error) {
	out := map[string]any{"version": 1, "id": id}
	for _, key := range topLevelLabels {
		if raw, present := obj[key]; present {
			if v, ok := cleanField(fieldLabel, raw); ok {
				out[key] = v
			}
		}
	}
	if raw, present := obj["palette"]; present && !isJSONNull(raw) {
		if _, err := ValidatePalette(raw); err == nil {
			// Keep the palette as written (ValidatePalette's copy lower-cases
			// the colours, which would change the design's hash).
			var palette map[string]string
			if json.Unmarshal(raw, &palette) == nil {
				out["palette"] = palette
			}
		}
	}
	if raw, present := obj["sections"]; present {
		sections := []any{}
		var list []json.RawMessage
		if json.Unmarshal(raw, &list) == nil {
			for _, s := range list {
				if len(sections) == MaxDesignSections {
					break
				}
				if sec, ok := cleanSection(s); ok {
					sections = append(sections, sec)
				}
			}
		}
		out["sections"] = sections
	}
	if raw, present := obj["options"]; present {
		var opts map[string]json.RawMessage
		if json.Unmarshal(raw, &opts) == nil && opts != nil {
			clean := map[string]any{}
			for _, key := range optionLabels {
				if raw, present := opts[key]; present {
					if v, ok := cleanField(fieldLabel, raw); ok {
						clean[key] = v
					}
				}
			}
			out["options"] = clean
		}
	}
	if raw, present := obj["customCss"]; present {
		if css, ok := rawString(raw); ok {
			// Kept as text: it is sanitized when it becomes the page
			// stylesheet. Only the size is bounded here.
			out["customCss"] = truncateBytes(cleanText(css, len(css)), csssanitize.MaxBytes)
		}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(out); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
