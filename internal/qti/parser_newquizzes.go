package qti

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Canvas New Quizzes uses the IMS-QTI 2.2 element set. The relevant
// shapes are documented at https://www.imsglobal.org/spec/qti/v2p2/.
// We model only what NQ actually emits — full QTI 2.2 has dozens of
// interactions we will never see from Canvas.
//
// A NQ export ships one `<assessmentItem>` per question (in a separate
// file each), plus an `<assessmentTest>` that references them in order
// via `<assessmentItemRef>`. We follow the test → item references to
// rebuild the quiz.

type nqAssessmentTest struct {
	XMLName     xml.Name           `xml:"assessmentTest"`
	Identifier  string             `xml:"identifier,attr"`
	Title       string             `xml:"title,attr"`
	TestParts   []nqTestPart       `xml:"testPart"`
}

type nqTestPart struct {
	AssessmentSections []nqAssessmentSection `xml:"assessmentSection"`
}

type nqAssessmentSection struct {
	Identifier         string                `xml:"identifier,attr"`
	Title              string                `xml:"title,attr"`
	AssessmentItemRefs []nqAssessmentItemRef `xml:"assessmentItemRef"`
	// Nested sections (Canvas uses these for stimulus groups).
	Sections           []nqAssessmentSection  `xml:"assessmentSection"`
	RubricBlock        *nqRubricBlock         `xml:"rubricBlock"`
}

type nqAssessmentItemRef struct {
	Identifier string `xml:"identifier,attr"`
	Href       string `xml:"href,attr"`
}

type nqRubricBlock struct {
	View    string `xml:"view,attr"`
	Content string `xml:",innerxml"`
}

// nqAssessmentItem is one question. The crucial fields:
//   - <responseDeclaration> declares response variables and their correct values
//   - itemBody contains the prompt + interaction
//   - <modalFeedback> is per-outcome feedback
type nqAssessmentItem struct {
	XMLName               xml.Name              `xml:"assessmentItem"`
	Identifier            string                `xml:"identifier,attr"`
	Title                 string                `xml:"title,attr"`
	Adaptive              string                `xml:"adaptive,attr"`
	ResponseDeclarations  []nqResponseDeclaration `xml:"responseDeclaration"`
	OutcomeDeclarations   []nqOutcomeDeclaration  `xml:"outcomeDeclaration"`
	ItemBody              nqItemBody            `xml:"itemBody"`
	ModalFeedbacks        []nqModalFeedback     `xml:"modalFeedback"`
}

type nqResponseDeclaration struct {
	Identifier  string         `xml:"identifier,attr"`
	Cardinality string         `xml:"cardinality,attr"`
	BaseType    string         `xml:"baseType,attr"`
	CorrectResponse nqCorrectResponse `xml:"correctResponse"`
	Mapping     *nqMapping     `xml:"mapping"`
	// AreaMapping is used by hotspotInteraction's response decl.
	AreaMapping *nqAreaMapping `xml:"areaMapping"`
}

type nqCorrectResponse struct {
	Values []nqValue `xml:"value"`
}

type nqValue struct {
	FieldIdentifier string `xml:"fieldIdentifier,attr"`
	BaseType        string `xml:"baseType,attr"`
	Text            string `xml:",chardata"`
}

type nqMapping struct {
	LowerBound  string         `xml:"lowerBound,attr"`
	UpperBound  string         `xml:"upperBound,attr"`
	DefaultValue string        `xml:"defaultValue,attr"`
	MapEntries  []nqMapEntry   `xml:"mapEntry"`
}

type nqMapEntry struct {
	MapKey      string `xml:"mapKey,attr"`
	MappedValue string `xml:"mappedValue,attr"`
}

type nqAreaMapping struct {
	AreaMapEntries []nqAreaMapEntry `xml:"areaMapEntry"`
}

type nqAreaMapEntry struct {
	Shape       string `xml:"shape,attr"`
	Coords      string `xml:"coords,attr"`
	MappedValue string `xml:"mappedValue,attr"`
}

type nqOutcomeDeclaration struct {
	Identifier   string `xml:"identifier,attr"`
	Cardinality  string `xml:"cardinality,attr"`
	BaseType     string `xml:"baseType,attr"`
}

// nqItemBody contains the prompt + interaction. We capture the raw
// inner XML to preserve the prompt HTML (Canvas embeds rich content),
// plus each interaction type separately.
type nqItemBody struct {
	// InnerXML holds the prompt HTML which can include paragraphs,
	// images, and tables. The dialect parser uses this as the
	// question_text.
	InnerXML string `xml:",innerxml"`
	// Strongly-typed interactions follow. Only one is non-nil per item.
	ChoiceInteractions       []nqChoiceInteraction       `xml:"choiceInteraction"`
	TextEntryInteractions    []nqTextEntryInteraction    `xml:"textEntryInteraction"`
	ExtendedTextInteractions []nqExtendedTextInteraction `xml:"extendedTextInteraction"`
	MatchInteractions        []nqMatchInteraction        `xml:"matchInteraction"`
	InlineChoiceInteractions []nqInlineChoiceInteraction `xml:"inlineChoiceInteraction"`
	UploadInteractions       []nqUploadInteraction       `xml:"uploadInteraction"`
	OrderInteractions        []nqOrderInteraction        `xml:"orderInteraction"`
	GapMatchInteractions     []nqGapMatchInteraction     `xml:"gapMatchInteraction"`
	HotspotInteractions      []nqHotspotInteraction      `xml:"hotspotInteraction"`
}

type nqChoiceInteraction struct {
	ResponseIdentifier string         `xml:"responseIdentifier,attr"`
	MaxChoices         int            `xml:"maxChoices,attr"`
	Prompt             string         `xml:"prompt"`
	SimpleChoices      []nqSimpleChoice `xml:"simpleChoice"`
}

type nqSimpleChoice struct {
	Identifier string `xml:"identifier,attr"`
	Content    string `xml:",innerxml"`
}

type nqTextEntryInteraction struct {
	ResponseIdentifier string `xml:"responseIdentifier,attr"`
	ExpectedLength     int    `xml:"expectedLength,attr"`
}

type nqExtendedTextInteraction struct {
	ResponseIdentifier string `xml:"responseIdentifier,attr"`
	Prompt             string `xml:"prompt"`
}

type nqMatchInteraction struct {
	ResponseIdentifier string         `xml:"responseIdentifier,attr"`
	MaxAssociations    int            `xml:"maxAssociations,attr"`
	SimpleMatchSets    []nqMatchSet   `xml:"simpleMatchSet"`
}

type nqMatchSet struct {
	SimpleAssociableChoices []nqSimpleAssociableChoice `xml:"simpleAssociableChoice"`
}

type nqSimpleAssociableChoice struct {
	Identifier  string `xml:"identifier,attr"`
	MatchMax    int    `xml:"matchMax,attr"`
	Content     string `xml:",innerxml"`
}

type nqInlineChoiceInteraction struct {
	ResponseIdentifier string             `xml:"responseIdentifier,attr"`
	InlineChoices      []nqInlineChoice   `xml:"inlineChoice"`
}

type nqInlineChoice struct {
	Identifier string `xml:"identifier,attr"`
	Content    string `xml:",innerxml"`
}

type nqUploadInteraction struct {
	ResponseIdentifier string `xml:"responseIdentifier,attr"`
}

type nqOrderInteraction struct {
	ResponseIdentifier string             `xml:"responseIdentifier,attr"`
	SimpleChoices      []nqSimpleChoice   `xml:"simpleChoice"`
}

type nqGapMatchInteraction struct {
	ResponseIdentifier string                `xml:"responseIdentifier,attr"`
	GapTexts           []nqGapText           `xml:"gapText"`
	Gaps               []nqGap               `xml:"gap"`
	// The body in between holds the prompt with embedded <gap> markers.
	InnerXML string `xml:",innerxml"`
}

type nqGapText struct {
	Identifier string `xml:"identifier,attr"`
	Content    string `xml:",chardata"`
}

type nqGap struct {
	Identifier string `xml:"identifier,attr"`
}

type nqHotspotInteraction struct {
	ResponseIdentifier string          `xml:"responseIdentifier,attr"`
	MaxChoices         int             `xml:"maxChoices,attr"`
	Object             nqHotspotObject `xml:"object"`
	HotspotChoices     []nqHotspotChoice `xml:"hotspotChoice"`
}

type nqHotspotObject struct {
	Data   string `xml:"data,attr"`
	Type   string `xml:"type,attr"`
	Width  string `xml:"width,attr"`
	Height string `xml:"height,attr"`
}

type nqHotspotChoice struct {
	Identifier string `xml:"identifier,attr"`
	Shape      string `xml:"shape,attr"`
	Coords     string `xml:"coords,attr"`
}

type nqModalFeedback struct {
	Identifier string `xml:"identifier,attr"`
	OutcomeIdentifier string `xml:"outcomeIdentifier,attr"`
	ShowHide   string `xml:"showHide,attr"`
	Content    string `xml:",innerxml"`
}

// parseNewQuizzesItem parses one NQ <assessmentItem> XML blob and
// returns the Paper LMS QuestionImport. Errors are returned per-item;
// the caller decides whether to surface them as ImportError.
func parseNewQuizzesItem(filename string, data []byte, position int) (QuestionImport, []ImportWarning, *ImportError) {
	var item nqAssessmentItem
	if err := xml.Unmarshal(data, &item); err != nil {
		return QuestionImport{}, nil, &ImportError{
			Source:  filename,
			Code:    "xml_parse_error",
			Message: err.Error(),
		}
	}
	return nqItemToQuestion(item, filename, position)
}

// nqItemToQuestion is the workhorse — dispatches on which interaction
// element is present and builds the Paper LMS Answers JSON. Each branch
// mirrors the corresponding builder in parser_classic.go in spirit:
// the output JSON shape must match what quiz_service.go's graders
// expect.
func nqItemToQuestion(item nqAssessmentItem, source string, position int) (QuestionImport, []ImportWarning, *ImportError) {
	warnings := []ImportWarning{}
	body := item.ItemBody

	// Some NQ interactions (notably inlineChoiceInteraction) are
	// embedded inside paragraph elements, so encoding/xml's direct-
	// child unmarshal doesn't catch them. Re-extract via a permissive
	// inner-XML scan when the direct-child slice is empty.
	if len(body.InlineChoiceInteractions) == 0 {
		body.InlineChoiceInteractions = extractInlineChoiceInteractions(body.InnerXML)
	}

	// Question prompt: NQ encloses prompt HTML in <itemBody> with the
	// interaction element nested inside. We pull the prompt by
	// stripping the recognized interactions from the innerXML — see
	// extractNQPrompt for the rationale.
	prompt := extractNQPrompt(body)

	q := QuestionImport{
		Position:         position,
		QuestionText:     prompt,
		SourceIdentifier: item.Identifier,
	}

	// Points: NQ encodes via outcomeDeclaration's defaultValue or via
	// the mapping table; defaults to 1.0.
	points := nqExtractPoints(item)
	q.PointsPossible = &points

	// Modal feedback → correct/incorrect/neutral comments. NQ uses
	// outcomeIdentifier="FEEDBACK_correct" / "FEEDBACK_incorrect" /
	// "FEEDBACK_general" by convention.
	for _, mf := range item.ModalFeedbacks {
		text := strings.TrimSpace(mf.Content)
		if text == "" {
			continue
		}
		ident := strings.ToLower(mf.Identifier + " " + mf.OutcomeIdentifier)
		switch {
		case strings.Contains(ident, "correct") && !strings.Contains(ident, "incorrect"):
			q.CorrectComments = text
		case strings.Contains(ident, "incorrect"):
			q.IncorrectComments = text
		case strings.Contains(ident, "general"), strings.Contains(ident, "neutral"):
			q.NeutralComments = text
		}
	}

	// Dispatch on which interaction is present (exactly one).
	switch {
	case len(body.ChoiceInteractions) > 0:
		ci := body.ChoiceInteractions[0]
		labels := make([]string, len(ci.SimpleChoices))
		for i, sc := range ci.SimpleChoices {
			labels[i] = stripHTMLForLabel(sc.Content)
		}
		unified := ClassifyNewQuizzesChoice(labels, ci.MaxChoices)
		q.QuestionType = unified
		correctIDs := nqCorrectIDsForResponse(item.ResponseDeclarations, ci.ResponseIdentifier)
		answers := make([]map[string]interface{}, 0, len(ci.SimpleChoices))
		for _, sc := range ci.SimpleChoices {
			weight := 0.0
			if correctIDs[sc.Identifier] {
				weight = 100.0
			}
			answers = append(answers, map[string]interface{}{
				"id":     sc.Identifier,
				"text":   stripHTMLForLabel(sc.Content),
				"weight": weight,
			})
		}
		j, err := json.Marshal(answers)
		if err != nil {
			return QuestionImport{}, nil, &ImportError{Source: source, Code: "marshal_error", Message: err.Error()}
		}
		q.Answers = string(j)

	case len(body.TextEntryInteractions) > 0:
		te := body.TextEntryInteractions[0]
		hasTolerance := false
		for _, rd := range item.ResponseDeclarations {
			if rd.Identifier == te.ResponseIdentifier && rd.BaseType == "float" {
				hasTolerance = true
				break
			}
		}
		q.QuestionType = ClassifyNewQuizzesTextEntry(hasTolerance)
		// Accepted values from correctResponse.values.
		values := nqCorrectValuesForResponse(item.ResponseDeclarations, te.ResponseIdentifier)
		answers := make([]map[string]interface{}, 0, len(values))
		for i, v := range values {
			entry := map[string]interface{}{
				"id":     fmt.Sprintf("a%d", i+1),
				"text":   v,
				"weight": 100.0,
			}
			if hasTolerance {
				// Look for a mapping with bounds; pull margin from
				// (upperBound - lowerBound) / 2 if available.
				if margin := nqExtractTolerance(item.ResponseDeclarations, te.ResponseIdentifier); margin != "" {
					entry["margin"] = margin
				}
			}
			answers = append(answers, entry)
		}
		j, _ := json.Marshal(answers)
		q.Answers = string(j)

	case len(body.ExtendedTextInteractions) > 0:
		q.QuestionType = UnifiedEssay
		q.Answers = "[]"

	case len(body.UploadInteractions) > 0:
		q.QuestionType = UnifiedFileUpload
		q.Answers = "[]"

	case len(body.MatchInteractions) > 0:
		mi := body.MatchInteractions[0]
		q.QuestionType = UnifiedMatching
		// First simpleMatchSet = left items, second = right items.
		var lefts, rights []nqSimpleAssociableChoice
		if len(mi.SimpleMatchSets) >= 2 {
			lefts = mi.SimpleMatchSets[0].SimpleAssociableChoices
			rights = mi.SimpleMatchSets[1].SimpleAssociableChoices
		}
		_ = rights
		// Correct response is a directedPair: "leftID rightID".
		pairs := nqDirectedPairs(item.ResponseDeclarations, mi.ResponseIdentifier)
		answers := []map[string]interface{}{}
		for _, l := range lefts {
			leftLabel := stripHTMLForLabel(l.Content)
			rightID := pairs[l.Identifier]
			answers = append(answers, map[string]interface{}{
				"id":       l.Identifier,
				"left":     leftLabel,
				"right_id": rightID,
				"weight":   100.0,
			})
		}
		j, _ := json.Marshal(answers)
		q.Answers = string(j)

	case len(body.InlineChoiceInteractions) > 0:
		// Multiple inlineChoice interactions inside one item =
		// multiple_dropdown. A single one = also multiple_dropdown
		// with one blank — Paper LMS doesn't have a special
		// single-dropdown type.
		q.QuestionType = UnifiedMultipleDropdown
		answers := []map[string]interface{}{}
		for _, ici := range body.InlineChoiceInteractions {
			correctIDs := nqCorrectIDsForResponse(item.ResponseDeclarations, ici.ResponseIdentifier)
			for _, ic := range ici.InlineChoices {
				weight := 0.0
				if correctIDs[ic.Identifier] {
					weight = 100.0
				}
				answers = append(answers, map[string]interface{}{
					"id":       ic.Identifier,
					"text":     stripHTMLForLabel(ic.Content),
					"weight":   weight,
					"blank_id": ici.ResponseIdentifier,
				})
			}
		}
		j, _ := json.Marshal(answers)
		q.Answers = string(j)

	case len(body.OrderInteractions) > 0:
		oi := body.OrderInteractions[0]
		q.QuestionType = UnifiedOrdering
		// Correct order is the chardata of correctResponse <value>s
		// in the response declaration, in order.
		correctOrder := nqCorrectValuesForResponse(item.ResponseDeclarations, oi.ResponseIdentifier)
		// Build {id, text, weight=100 for each in canonical order}.
		// Choices are presented in shuffled order by Canvas; the
		// ordering grader keys on the canonical (correctOrder)
		// sequence.
		choiceText := map[string]string{}
		for _, sc := range oi.SimpleChoices {
			choiceText[sc.Identifier] = stripHTMLForLabel(sc.Content)
		}
		answers := make([]map[string]interface{}, 0, len(correctOrder))
		for _, id := range correctOrder {
			answers = append(answers, map[string]interface{}{
				"id":     id,
				"text":   choiceText[id],
				"weight": 100.0,
			})
		}
		j, _ := json.Marshal(answers)
		q.Answers = string(j)

	case len(body.GapMatchInteractions) > 0:
		// Treated as categorization: gapText IDs are the "items",
		// gap IDs are the "buckets". Correct response is
		// directedPair "itemID bucketID".
		gmi := body.GapMatchInteractions[0]
		_ = gmi
		q.QuestionType = UnifiedCategorization
		pairs := nqDirectedPairs(item.ResponseDeclarations, gmi.ResponseIdentifier)
		answers := []map[string]interface{}{}
		// Look up each gap text by id so we can carry the label.
		gapTextByID := map[string]string{}
		for _, gt := range gmi.GapTexts {
			gapTextByID[gt.Identifier] = strings.TrimSpace(gt.Content)
		}
		// Sort for determinism.
		keys := make([]string, 0, len(pairs))
		for k := range pairs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, itemID := range keys {
			answers = append(answers, map[string]interface{}{
				"id":       itemID,
				"text":     gapTextByID[itemID],
				"right_id": pairs[itemID],
				"weight":   100.0,
			})
		}
		j, _ := json.Marshal(answers)
		q.Answers = string(j)

	case len(body.HotspotInteractions) > 0:
		hi := body.HotspotInteractions[0]
		q.QuestionType = UnifiedHotSpot
		// Hotspot correct ids come from correctResponse.values.
		correctIDs := nqCorrectIDsForResponse(item.ResponseDeclarations, hi.ResponseIdentifier)
		answers := []map[string]interface{}{}
		for _, hc := range hi.HotspotChoices {
			if !correctIDs[hc.Identifier] {
				continue
			}
			// shape="rect" coords="x1,y1,x2,y2" (per QTI 2.2 spec).
			x, y, w, h, ok := parseRectCoords(hc.Coords)
			if !ok {
				warnings = append(warnings, ImportWarning{
					Source: source, Code: "hotspot_coords_unparseable",
					Message: fmt.Sprintf("could not parse coords %q", hc.Coords),
				})
				continue
			}
			answers = append(answers, map[string]interface{}{
				"id":     hc.Identifier,
				"x":      x,
				"y":      y,
				"w":      w,
				"h":      h,
				"weight": 100.0,
			})
		}
		j, _ := json.Marshal(answers)
		q.Answers = string(j)

	default:
		// No recognized interaction. Could be a rubricBlock-only
		// item (stimulus passage holder) — handled separately by
		// the bundle walker. Treat the rest as warnings.
		return QuestionImport{}, []ImportWarning{{
			Source: source, Code: "no_interaction",
			Message: "assessmentItem has no recognized interaction element",
		}}, nil
	}

	return q, warnings, nil
}

// extractNQPrompt strips the recognized interaction tags from the
// item body's innerXML to leave only the prompt HTML.
//
// Why this approach: NQ embeds the prompt directly in <itemBody>
// alongside the interaction (e.g. <itemBody><p>What is 2+2?</p>
// <choiceInteraction>…</choiceInteraction></itemBody>). The interaction
// tags also have their own <prompt> children sometimes. We take the
// "outer" prompt (everything outside the interaction) as the question
// text and ignore the interaction-internal prompt as a redundant copy.
func extractNQPrompt(body nqItemBody) string {
	raw := body.InnerXML
	// Cheap removal of known interaction blocks.
	tags := []string{
		"choiceInteraction", "textEntryInteraction", "extendedTextInteraction",
		"matchInteraction", "inlineChoiceInteraction", "uploadInteraction",
		"orderInteraction", "gapMatchInteraction", "hotspotInteraction",
	}
	for _, t := range tags {
		raw = stripBlock(raw, t)
	}
	return strings.TrimSpace(raw)
}

// stripBlock removes all `<tag …>…</tag>` blocks from s. Naive — does
// not handle nested same-tag blocks (which NQ never produces).
func stripBlock(s, tag string) string {
	openTag := "<" + tag
	closeTag := "</" + tag + ">"
	for {
		i := strings.Index(s, openTag)
		if i < 0 {
			return s
		}
		j := strings.Index(s[i:], closeTag)
		if j < 0 {
			// Unterminated — chop from i to end.
			return s[:i]
		}
		s = s[:i] + s[i+j+len(closeTag):]
	}
}

// stripHTMLForLabel produces a plain-text label from an HTML snippet.
// Used for option labels (true/false detection, matching keys). Not a
// general-purpose HTML sanitizer.
func stripHTMLForLabel(s string) string {
	out := []rune{}
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			out = append(out, r)
		}
	}
	return strings.TrimSpace(string(out))
}

// nqCorrectIDsForResponse returns the set of identifier values declared
// correct for a given response variable. Used by choice / dropdown /
// hotspot interactions.
func nqCorrectIDsForResponse(rds []nqResponseDeclaration, respID string) map[string]bool {
	out := map[string]bool{}
	for _, rd := range rds {
		if rd.Identifier != respID {
			continue
		}
		for _, v := range rd.CorrectResponse.Values {
			val := strings.TrimSpace(v.Text)
			if val != "" {
				out[val] = true
			}
		}
	}
	return out
}

// nqCorrectValuesForResponse returns the chardata of each correctResponse
// <value>, preserving source order. Used by text-entry & order.
func nqCorrectValuesForResponse(rds []nqResponseDeclaration, respID string) []string {
	out := []string{}
	for _, rd := range rds {
		if rd.Identifier != respID {
			continue
		}
		for _, v := range rd.CorrectResponse.Values {
			out = append(out, strings.TrimSpace(v.Text))
		}
	}
	return out
}

// nqDirectedPairs parses correctResponse values formatted as
// "leftID rightID" (space-separated). Returns map[leftID]=rightID.
// Used by matching & gapMatch.
func nqDirectedPairs(rds []nqResponseDeclaration, respID string) map[string]string {
	pairs := map[string]string{}
	for _, rd := range rds {
		if rd.Identifier != respID {
			continue
		}
		for _, v := range rd.CorrectResponse.Values {
			parts := strings.Fields(v.Text)
			if len(parts) == 2 {
				pairs[parts[0]] = parts[1]
			}
		}
	}
	return pairs
}

// nqExtractPoints walks the outcome declarations and returns the
// point value. NQ uses an outcomeDeclaration with identifier="SCORE"
// or "MAXSCORE" and a defaultValue.
func nqExtractPoints(item nqAssessmentItem) float64 {
	// Look for an outcomeDeclaration with a defaultValue.
	// (The defaultValue element is more nested than we modeled; we
	// can iterate response declaration mappings instead as a fallback.)
	for _, rd := range item.ResponseDeclarations {
		if rd.Mapping != nil && rd.Mapping.UpperBound != "" {
			if v, err := strconv.ParseFloat(rd.Mapping.UpperBound, 64); err == nil {
				return v
			}
		}
	}
	return 1.0
}

// nqExtractTolerance returns the tolerance string (in Paper LMS margin
// format) for a numerical response declaration. Returns "" if none.
func nqExtractTolerance(rds []nqResponseDeclaration, respID string) string {
	for _, rd := range rds {
		if rd.Identifier != respID || rd.Mapping == nil {
			continue
		}
		// If we have a mapping with two bounds and a single mapEntry
		// for the correct value, the tolerance is (upper-lower)/2.
		lo, err1 := strconv.ParseFloat(rd.Mapping.LowerBound, 64)
		hi, err2 := strconv.ParseFloat(rd.Mapping.UpperBound, 64)
		if err1 == nil && err2 == nil && hi > lo {
			return strconv.FormatFloat((hi-lo)/2, 'f', -1, 64)
		}
	}
	return ""
}

// extractInlineChoiceInteractions scans inner XML for
// <inlineChoiceInteraction>…</inlineChoiceInteraction> blocks anywhere
// in the body (NQ embeds these inside <p> elements, which encoding/xml's
// direct-child path won't catch). Returns parsed interactions.
func extractInlineChoiceInteractions(innerXML string) []nqInlineChoiceInteraction {
	out := []nqInlineChoiceInteraction{}
	const openTag = "<inlineChoiceInteraction"
	const closeTag = "</inlineChoiceInteraction>"
	rest := innerXML
	for {
		i := strings.Index(rest, openTag)
		if i < 0 {
			break
		}
		j := strings.Index(rest[i:], closeTag)
		if j < 0 {
			break
		}
		block := rest[i : i+j+len(closeTag)]
		var ici nqInlineChoiceInteraction
		if err := xml.Unmarshal([]byte(block), &ici); err == nil {
			out = append(out, ici)
		}
		rest = rest[i+j+len(closeTag):]
	}
	return out
}

// parseRectCoords parses QTI 2.2 hotspot rect coords "x1,y1,x2,y2"
// into (x, y, width, height) for Paper LMS hot_spot answers.
func parseRectCoords(coords string) (x, y, w, h float64, ok bool) {
	parts := strings.Split(coords, ",")
	if len(parts) != 4 {
		return
	}
	vals := make([]float64, 4)
	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return 0, 0, 0, 0, false
		}
		vals[i] = v
	}
	x = vals[0]
	y = vals[1]
	w = vals[2] - vals[0]
	h = vals[3] - vals[1]
	return x, y, w, h, true
}
