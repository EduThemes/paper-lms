package qti

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Canvas Classic QTI 2.1 XML structures.
//
// Despite the "QTI 2.1" branding, Canvas Classic exports use the QTI 1.2-
// derived `<questestinterop>` envelope (with `<assessment>` → `<section>`
// → `<item>` nesting and `<qtimetadatafield>` for the item type). This
// has been stable since ~2010; every Canvas instance still produces it
// today. We hew exactly to what real exports look like — not to any
// formal QTI 2.1 spec — because reality wins.

type classicRoot struct {
	XMLName     xml.Name           `xml:"questestinterop"`
	Assessments []classicAssessment `xml:"assessment"`
	// <objectbank> appears in the assessment_question_banks/<id>.xml
	// files when an instructor has authored question banks.
	ObjectBank *classicObjectBank `xml:"objectbank"`
}

type classicAssessment struct {
	Ident          string                  `xml:"ident,attr"`
	Title          string                  `xml:"title,attr"`
	Metadata       classicQTIMetadata      `xml:"qtimetadata"`
	Sections       []classicSection        `xml:"section"`
}

type classicObjectBank struct {
	Ident    string             `xml:"ident,attr"`
	Metadata classicQTIMetadata `xml:"qtimetadata"`
	Items    []classicItem      `xml:"item"`
}

type classicSection struct {
	Ident         string              `xml:"ident,attr"`
	Title         string              `xml:"title,attr"`
	Items         []classicItem       `xml:"item"`
	// `<selection_ordering>` inside a section is how Canvas Classic
	// represents "draw N items from a bank". We capture the source
	// pool reference; the service layer turns this into a
	// QuizQuestionGroup or a one-off resolved question.
	SourceBank    *classicSourceBank  `xml:"sourcebank_ref"`
	SectionRefs   []classicSectionRef `xml:"selection_ordering>order>order_extension>ims_qti_express_object_bank"`
}

// classicSourceBank captures the assessmentRef-style pointer in
// `<sourcebank_ref>` (Canvas uses both forms in the wild).
type classicSourceBank struct {
	Ident string `xml:",chardata"`
}

type classicSectionRef struct {
	Ident string `xml:"sourcebank_ref,attr"`
}

type classicQTIMetadata struct {
	Fields []classicMetaField `xml:"qtimetadatafield"`
}

type classicMetaField struct {
	Label string `xml:"fieldlabel"`
	Entry string `xml:"fieldentry"`
}

type classicItem struct {
	Ident        string             `xml:"ident,attr"`
	Title        string             `xml:"title,attr"`
	Metadata     classicItemMeta    `xml:"itemmetadata"`
	Presentation classicPresentation `xml:"presentation"`
	// `<resprocessing>` holds the correct-response rules and feedback
	// linkage. Canvas always emits exactly one.
	RespProcessing classicRespProcessing `xml:"resprocessing"`
	Feedback       []classicFeedback     `xml:"itemfeedback"`
}

type classicItemMeta struct {
	Fields classicQTIMetadata `xml:"qtimetadata"`
}

type classicPresentation struct {
	// Question prompt is in `<material><mattext>`. Canvas wraps it in
	// CDATA or escaped HTML. We treat it as raw bytes — the renderer
	// downstream handles sanitization.
	Material   []classicMaterial   `xml:"material"`
	ResponseLid []classicResponseLid `xml:"response_lid"`
	ResponseStr []classicResponseStr `xml:"response_str"`
	// `<response_extension>` is Canvas's container for numerical
	// response_num declarations.
	ResponseNum []classicResponseNum `xml:"response_num"`
}

type classicMaterial struct {
	MatText []classicMatText `xml:"mattext"`
}

type classicMatText struct {
	TextType string `xml:"texttype,attr"`
	Value    string `xml:",chardata"`
}

type classicResponseLid struct {
	Ident        string              `xml:"ident,attr"`
	RCardinality string              `xml:"rcardinality,attr"`
	// Material carries the per-response prompt: the left-hand label of a
	// matching pair, or the blank name of a dropdown / FIMB blank.
	Material     []classicMaterial   `xml:"material"`
	RenderChoice classicRenderChoice `xml:"render_choice"`
}

type classicRenderChoice struct {
	ResponseLabels []classicResponseLabel `xml:"response_label"`
}

type classicResponseLabel struct {
	Ident    string            `xml:"ident,attr"`
	Material []classicMaterial `xml:"material"`
}

type classicResponseStr struct {
	Ident       string `xml:"ident,attr"`
	RenderFib   classicRenderFIB `xml:"render_fib"`
}

type classicRenderFIB struct {
	// fill-in-blank typically has nothing structured here; the
	// blank's accepted answers live in resprocessing.
	Rows string `xml:"rows,attr"`
}

type classicResponseNum struct {
	Ident      string           `xml:"ident,attr"`
	RenderFib  classicRenderFIB `xml:"render_fib"`
}

type classicRespProcessing struct {
	RespConditions []classicRespCondition `xml:"respcondition"`
	Outcomes       *classicOutcomes       `xml:"outcomes"`
}

type classicOutcomes struct {
	DecVar []classicDecVar `xml:"decvar"`
}

type classicDecVar struct {
	MaxValue string `xml:"maxvalue,attr"`
	VarName  string `xml:"varname,attr"`
}

type classicRespCondition struct {
	Title       string              `xml:"title,attr"`
	Continue    string              `xml:"continue,attr"`
	ConditionVar classicConditionVar `xml:"conditionvar"`
	SetVar      []classicSetVar     `xml:"setvar"`
	DisplayFeedback []classicDisplayFeedback `xml:"displayfeedback"`
}

type classicConditionVar struct {
	// Possible children: <varequal>, <varsubset>, <and>, <or>.
	// We capture each by raw inner XML and then re-parse only the
	// ones we recognize. This is more robust than trying to nest
	// every shape — the Canvas exports include lots of irrelevant
	// matchers we don't care about.
	VarEquals []classicVarEqual `xml:"varequal"`
	// Numeric bounds: Canvas encodes exact±margin as
	// <or><varequal/><and><vargte/><varlte/></and></or>, a range answer as
	// bare <vargte/><varlte/>, and a precision answer with <vargt/>.
	VarGte []classicVarEqual    `xml:"vargte"`
	VarGt  []classicVarEqual    `xml:"vargt"`
	VarLte []classicVarEqual    `xml:"varlte"`
	And    *classicConditionVar `xml:"and"`
	Or     *classicConditionVar `xml:"or"`
}

type classicVarEqual struct {
	Respident string `xml:"respident,attr"`
	Case      string `xml:"case,attr"`
	Value     string `xml:",chardata"`
}

type classicSetVar struct {
	VarName string `xml:"varname,attr"`
	Action  string `xml:"action,attr"`
	Value   string `xml:",chardata"`
}

type classicDisplayFeedback struct {
	LinkRefID string `xml:"linkrefid,attr"`
	FeedbackType string `xml:"feedbacktype,attr"`
}

type classicFeedback struct {
	Ident     string            `xml:"ident,attr"`
	View      string            `xml:"view,attr"`
	Material  []classicMaterial `xml:"flow_mat>material"`
	Material2 []classicMaterial `xml:"material"`
}

// parseClassicXML parses one Canvas Classic XML file. Returns assessments,
// banks, and warnings. The caller (parser.go) drives this from the bundle
// walker.
func parseClassicXML(filename string, data []byte) (*classicRoot, error) {
	var root classicRoot
	if err := xml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse %s: %w", filename, err)
	}
	return &root, nil
}

// classicItemToQuestion converts one Canvas Classic <item> to a Paper
// LMS QuestionImport. Returns the question plus any warnings raised
// during conversion (e.g. malformed answer JSON, unknown item type).
//
// Why this is long: each Canvas item type has its own quirks in how
// it encodes the correct answer. The transformations are explicitly
// shaped to match what the existing quiz_service.go graders expect —
// see the comment on each `case` branch for which grader consumes the
// resulting JSON.
func classicItemToQuestion(item classicItem, position int, source string) (QuestionImport, []ImportWarning, *ImportError) {
	warnings := []ImportWarning{}

	// 1. Item-type metadata field (the load-bearing dispatch).
	canvasType := ""
	pointsPossible := 1.0
	for _, f := range item.Metadata.Fields.Fields {
		switch f.Label {
		case "question_type":
			canvasType = f.Entry
		case "points_possible":
			if v, err := strconv.ParseFloat(f.Entry, 64); err == nil {
				pointsPossible = v
			}
		}
	}

	unifiedType, ok := MapCanvasClassicType(canvasType)
	if !ok {
		return QuestionImport{}, nil, &ImportError{
			Source:  fmt.Sprintf("%s#%s", source, item.Ident),
			Code:    "unknown_item_type",
			Message: fmt.Sprintf("Canvas Classic question_type %q has no Paper LMS mapping", canvasType),
		}
	}

	// 2. Question prompt — first non-empty <mattext> in presentation.
	prompt := firstMatText(item.Presentation.Material)

	// 3. Per-type-feedback (correct / incorrect / neutral). Canvas
	// emits these as separate `<itemfeedback>` blocks, identified by
	// ident: "correct_fb", "incorrect_fb", "general_fb" (or any name
	// referenced by a respcondition's displayfeedback). We index them
	// here and resolve by linkrefid below.
	feedbackByIdent := map[string]string{}
	for _, fb := range item.Feedback {
		text := firstMatText(fb.Material)
		if text == "" {
			text = firstMatText(fb.Material2)
		}
		if fb.Ident != "" && text != "" {
			feedbackByIdent[fb.Ident] = text
		}
	}

	q := QuestionImport{
		Position:         position,
		QuestionType:     unifiedType,
		QuestionText:     prompt,
		PointsPossible:   &pointsPossible,
		SourceIdentifier: item.Ident,
	}

	// Resolve feedback by the linkrefid in any respcondition that
	// references it. Canvas convention:
	//   "correct_fb" / "correct"       → correct_comments
	//   "incorrect_fb" / "general_incorrect" → incorrect_comments
	//   "general_fb" / "neutral"       → neutral_comments
	for _, rc := range item.RespProcessing.RespConditions {
		for _, df := range rc.DisplayFeedback {
			if text, ok := feedbackByIdent[df.LinkRefID]; ok {
				switch normalizeFeedbackIdent(df.LinkRefID) {
				case "correct":
					q.CorrectComments = text
				case "incorrect":
					q.IncorrectComments = text
				case "general", "neutral":
					q.NeutralComments = text
				}
			}
		}
	}
	// Also pick up feedback blocks by ident even when no respcondition
	// links to them — exporters sometimes omit the <displayfeedback>
	// pointer but still emit the feedback block with a canonical ident.
	for ident, text := range feedbackByIdent {
		switch normalizeFeedbackIdent(ident) {
		case "correct":
			if q.CorrectComments == "" {
				q.CorrectComments = text
			}
		case "incorrect":
			if q.IncorrectComments == "" {
				q.IncorrectComments = text
			}
		case "general", "neutral":
			if q.NeutralComments == "" {
				q.NeutralComments = text
			}
		}
	}

	// 4. Per-type answer JSON. The transformation MUST yield the JSON
	// shape consumed by the corresponding grader in quiz_service.go.
	answersJSON, w, err := buildClassicAnswers(unifiedType, item, pointsPossible)
	if err != nil {
		return QuestionImport{}, nil, &ImportError{
			Source:  fmt.Sprintf("%s#%s", source, item.Ident),
			Code:    "answer_parse_error",
			Message: err.Error(),
		}
	}
	q.Answers = answersJSON
	warnings = append(warnings, w...)

	return q, warnings, nil
}

// buildClassicAnswers extracts the correct-answer set for a single
// Canvas Classic item and returns it as the Paper LMS Answers JSON
// string. Each branch documents the grader function (in
// quiz_service.go) that will consume the output.
func buildClassicAnswers(unifiedType string, item classicItem, points float64) (string, []ImportWarning, error) {
	switch unifiedType {

	case UnifiedMultipleChoice, UnifiedTrueFalse:
		// gradeMultipleChoice expects an array of {id, text, weight}.
		// The correct option has weight=100, others have weight=0.
		// We resolve the correct ident from the first <respcondition>
		// whose <setvar varname="SCORE" action="Set"> value > 0.
		return buildMultipleChoiceAnswers(item)

	case UnifiedMultipleAnswer:
		// gradeMultipleAnswer also expects {id, text, weight} but
		// multiple options have weight=100. Canvas encodes this via
		// an <and> of <varequal> matchers (one per correct id) inside
		// a single respcondition.
		return buildMultipleAnswerAnswers(item)

	case UnifiedShortAnswer, UnifiedFillInTheBlank:
		// gradeShortAnswer / gradeFillInTheBlank both expect
		// [{id, text, weight}] where text is the accepted string.
		// Canvas emits multiple <varequal case="No"> entries, one
		// per accepted spelling.
		return buildShortAnswerAnswers(item)

	case UnifiedNumerical:
		// gradeNumerical expects [{id, text, weight, margin}].
		// Canvas emits <varequal> for an exact-match or a <vargte>
		// + <varlte> pair for a tolerance range. We collapse the
		// tolerance pair into a single answer with the midpoint and
		// margin.
		return buildNumericalAnswers(item)

	case UnifiedFormula:
		// gradeFormula re-uses the numerical grader. Same shape.
		return buildNumericalAnswers(item)

	case UnifiedEssay, UnifiedFileUpload, UnifiedTextOnly:
		// These three are not auto-graded — answers JSON is just a
		// stub so the column isn't null.
		return "[]", nil, nil

	case UnifiedMatching:
		// gradeMatching expects [{left, right_id}, …] where Left is
		// the human label of the left-hand item and RightID is the
		// id of the correct right-hand option. Canvas encodes this
		// with one <response_lid> per left item; the respconditions
		// hold the correct mapping.
		return buildMatchingAnswers(item)

	case UnifiedFillInMultipleBlanks:
		// gradeFillInMultipleBlanks expects {"blank_id":["a","b"]}.
		// Canvas emits one <response_lid> per blank with respident
		// matching the blank id; respconditions hold the accepted
		// strings.
		return buildFillInMultipleBlanksAnswers(item)

	case UnifiedMultipleDropdown:
		// gradeMultipleDropdown expects [{id, text, weight, blank_id}].
		// Canvas encodes each dropdown as a <response_lid> with the
		// blank id as respident.
		return buildMultipleDropdownAnswers(item)
	}

	// Unhandled-but-mapped types (shouldn't happen — coverage test).
	return "[]", []ImportWarning{{
		Code:    "unhandled_classic_type",
		Message: fmt.Sprintf("Mapped to %s but no answer builder", unifiedType),
	}}, nil
}

// --- per-type answer builders ---

func buildMultipleChoiceAnswers(item classicItem) (string, []ImportWarning, error) {
	if len(item.Presentation.ResponseLid) == 0 {
		return "[]", []ImportWarning{{Code: "missing_response_lid"}}, nil
	}
	lid := item.Presentation.ResponseLid[0]
	correctIDs := classicCorrectIDs(item.RespProcessing.RespConditions, lid.Ident)

	answers := make([]map[string]interface{}, 0, len(lid.RenderChoice.ResponseLabels))
	for _, rl := range lid.RenderChoice.ResponseLabels {
		text := firstMatText(rl.Material)
		weight := 0.0
		if correctIDs[rl.Ident] {
			weight = 100.0
		}
		answers = append(answers, map[string]interface{}{
			"id":     rl.Ident,
			"text":   text,
			"weight": weight,
		})
	}
	b, err := json.Marshal(answers)
	if err != nil {
		return "", nil, err
	}
	return string(b), nil, nil
}

func buildMultipleAnswerAnswers(item classicItem) (string, []ImportWarning, error) {
	if len(item.Presentation.ResponseLid) == 0 {
		return "[]", []ImportWarning{{Code: "missing_response_lid"}}, nil
	}
	lid := item.Presentation.ResponseLid[0]
	correctIDs := classicCorrectIDs(item.RespProcessing.RespConditions, lid.Ident)

	answers := make([]map[string]interface{}, 0, len(lid.RenderChoice.ResponseLabels))
	for _, rl := range lid.RenderChoice.ResponseLabels {
		text := firstMatText(rl.Material)
		weight := 0.0
		if correctIDs[rl.Ident] {
			weight = 100.0
		}
		answers = append(answers, map[string]interface{}{
			"id":     rl.Ident,
			"text":   text,
			"weight": weight,
		})
	}
	b, err := json.Marshal(answers)
	return string(b), nil, err
}

func buildShortAnswerAnswers(item classicItem) (string, []ImportWarning, error) {
	// Each <varequal> child of a positive-score respcondition is one
	// accepted spelling.
	accepted := classicVarEqualValues(item.RespProcessing.RespConditions)
	answers := make([]map[string]interface{}, 0, len(accepted))
	for i, text := range accepted {
		answers = append(answers, map[string]interface{}{
			"id":     fmt.Sprintf("a%d", i+1),
			"text":   text,
			"weight": 100.0,
		})
	}
	b, err := json.Marshal(answers)
	return string(b), nil, err
}


func buildNumericalAnswers(item classicItem) (string, []ImportWarning, error) {
	// Paper's own exporter (and some Canvas exports) carry an explicit
	// answer_exact / answer_error_margin pair in item metadata — prefer it.
	answers := []map[string]interface{}{}
	exact := ""
	margin := ""
	for _, f := range item.Metadata.Fields.Fields {
		switch f.Label {
		case "answer_exact":
			exact = f.Entry
		case "answer_error_margin":
			margin = f.Entry
		}
	}
	if exact != "" {
		entry := map[string]interface{}{
			"id":     "a1",
			"text":   exact,
			"weight": 100.0,
		}
		if margin != "" {
			entry["margin"] = margin
		}
		answers = append(answers, entry)
		b, err := json.Marshal(answers)
		return string(b), nil, err
	}

	// Otherwise decode Canvas's resprocessing (lib/cc/qti/qti_items.rb
	// numerical_resprocessing), one scoring respcondition per answer:
	//   exact ± margin: <or><varequal>v</varequal><and><vargte>v-m</vargte><varlte>v+m</varlte></and></or>
	//   precision:      <or><varequal>v</varequal><and><vargt>lo</vargt><varlte>hi</varlte></and></or>
	//   range:          <vargte>lo</vargte><varlte>hi</varlte>
	//   bare exact:     <varequal>v</varequal>
	var warnings []ImportWarning
	n := 0
	for _, rc := range item.RespProcessing.RespConditions {
		if classicRespConditionScore(rc) <= 0 {
			continue
		}
		cv := rc.ConditionVar
		if entry, ok := numericalAnswerFromCondition(cv); ok {
			n++
			entry["id"] = fmt.Sprintf("a%d", n)
			entry["weight"] = 100.0
			answers = append(answers, entry)
			continue
		}
		warnings = append(warnings, ImportWarning{
			Source:  item.Ident,
			Code:    "numerical_condition_unrecognized",
			Message: "a scoring numerical condition could not be decoded",
		})
	}
	b, err := json.Marshal(answers)
	return string(b), warnings, err
}

// numericalAnswerFromCondition decodes one Canvas numerical conditionvar.
func numericalAnswerFromCondition(cv classicConditionVar) (map[string]interface{}, bool) {
	// Exact (± margin / precision band) wrapped in <or>.
	if cv.Or != nil && len(cv.Or.VarEquals) > 0 {
		exact := strings.TrimSpace(cv.Or.VarEquals[0].Value)
		entry := map[string]interface{}{"text": exact, "numerical_answer_type": "exact_answer", "exact": exact}
		if band := cv.Or.And; band != nil {
			lo, okLo := firstBound(band.VarGte, band.VarGt)
			hi, okHi := firstBound(band.VarLte, nil)
			if okLo && okHi {
				if len(band.VarGte) == 0 && len(band.VarGt) > 0 {
					// precision band: exclusive lower bound — import as a
					// range so the accepted interval is preserved.
					return map[string]interface{}{
						"text":                  exact,
						"numerical_answer_type": "range_answer",
						"start":                 formatNum(lo),
						"end":                   formatNum(hi),
					}, true
				}
				m := (hi - lo) / 2
				if m < 0 {
					m = -m
				}
				entry["margin"] = formatNum(m)
			}
		}
		return entry, exact != ""
	}
	// Range: bare or <and>-wrapped vargte + varlte.
	gte, lte := cv.VarGte, cv.VarLte
	if cv.And != nil {
		gte = append(append([]classicVarEqual{}, gte...), cv.And.VarGte...)
		lte = append(append([]classicVarEqual{}, lte...), cv.And.VarLte...)
	}
	if lo, okLo := firstBound(gte, nil); okLo {
		if hi, okHi := firstBound(lte, nil); okHi {
			return map[string]interface{}{
				"text":                  formatNum(lo) + ".." + formatNum(hi),
				"numerical_answer_type": "range_answer",
				"start":                 formatNum(lo),
				"end":                   formatNum(hi),
			}, true
		}
	}
	// Bare exact.
	if len(cv.VarEquals) > 0 {
		exact := strings.TrimSpace(cv.VarEquals[0].Value)
		return map[string]interface{}{"text": exact, "numerical_answer_type": "exact_answer", "exact": exact}, exact != ""
	}
	return nil, false
}

func firstBound(primary, secondary []classicVarEqual) (float64, bool) {
	for _, list := range [][]classicVarEqual{primary, secondary} {
		for _, v := range list {
			if f, err := strconv.ParseFloat(strings.TrimSpace(v.Value), 64); err == nil {
				return f, true
			}
		}
	}
	return 0, false
}

func formatNum(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// canvasBlankID turns a Canvas response ident ("response_<blank>") or the
// response's material text into the blank name used in the question text
// ("[blank]"). Canvas puts the bare blank name in the material.
func canvasBlankID(lid classicResponseLid) string {
	if t := firstMatText(lid.Material); t != "" {
		return t
	}
	return strings.TrimPrefix(lid.Ident, "response_")
}

func buildMatchingAnswers(item classicItem) (string, []ImportWarning, error) {
	// Canvas emits one <response_lid ident="response_<answer_id>"> per left
	// item, carrying the left prompt in its material and the full pool of
	// right-hand matches in render_choice. The correct match_id for each
	// left is a varequal (respident = the lid ident) in resprocessing —
	// scored with action="Add" partial points, so any positive score counts.
	answers := []interface{}{}
	poolSeen := map[string]bool{}
	pool := []map[string]interface{}{}
	for _, lid := range item.Presentation.ResponseLid {
		answerID := strings.TrimPrefix(lid.Ident, "response_")
		leftLabel := firstMatText(lid.Material)
		if leftLabel == "" {
			leftLabel = answerID
		}

		correctRight := ""
		for _, rc := range item.RespProcessing.RespConditions {
			if classicRespConditionScore(rc) <= 0 {
				continue
			}
			for _, ve := range rc.ConditionVar.VarEquals {
				if ve.Respident == lid.Ident {
					correctRight = strings.TrimSpace(ve.Value)
					break
				}
			}
			if correctRight != "" {
				break
			}
		}
		answers = append(answers, map[string]interface{}{
			"id":       answerID,
			"left":     leftLabel,
			"text":     leftLabel,
			"right_id": correctRight,
			"weight":   100.0,
		})
		for _, rl := range lid.RenderChoice.ResponseLabels {
			text := firstMatText(rl.Material)
			if rl.Ident == "" || text == "" || poolSeen[rl.Ident] {
				continue
			}
			poolSeen[rl.Ident] = true
			pool = append(pool, map[string]interface{}{
				"id":       rl.Ident,
				"text":     text,
				"blank_id": "_match_options_",
			})
		}
	}
	for _, p := range pool {
		answers = append(answers, p)
	}
	b, err := json.Marshal(answers)
	return string(b), nil, err
}

func buildFillInMultipleBlanksAnswers(item classicItem) (string, []ImportWarning, error) {
	// Result shape: map[blank_id] -> []accepted_string.
	//
	// Canvas emits one <response_lid ident="response_<blank>"> per blank
	// whose render_choice lists EVERY accepted answer for that blank (all
	// fill-in-multiple-blanks answers are correct alternatives); the
	// resprocessing varequal references one of those answers by its label
	// ident, not by its text. Paper's own exporter instead emits
	// <response_str ident="<blank>"> and varequal values that ARE the
	// accepted text. Both are handled here.
	accepted := map[string][]string{}
	seen := map[string]map[string]bool{}
	add := func(blank, text string) {
		text = strings.TrimSpace(text)
		if blank == "" || text == "" {
			return
		}
		if seen[blank] == nil {
			seen[blank] = map[string]bool{}
		}
		if seen[blank][text] {
			return
		}
		seen[blank][text] = true
		accepted[blank] = append(accepted[blank], text)
	}

	blankByRespident := map[string]string{}
	labelText := map[string]map[string]string{} // respident → label ident → text
	for _, lid := range item.Presentation.ResponseLid {
		blank := canvasBlankID(lid)
		blankByRespident[lid.Ident] = blank
		labelText[lid.Ident] = map[string]string{}
		for _, rl := range lid.RenderChoice.ResponseLabels {
			text := firstMatText(rl.Material)
			labelText[lid.Ident][rl.Ident] = text
			add(blank, text)
		}
	}
	for _, rs := range item.Presentation.ResponseStr {
		blankByRespident[rs.Ident] = strings.TrimPrefix(rs.Ident, "response_")
	}

	collect := func(ves []classicVarEqual) {
		for _, ve := range ves {
			if ve.Respident == "" {
				continue
			}
			blank, ok := blankByRespident[ve.Respident]
			if !ok {
				blank = strings.TrimPrefix(ve.Respident, "response_")
			}
			value := ve.Value
			if t, isLabel := labelText[ve.Respident][strings.TrimSpace(ve.Value)]; isLabel && t != "" {
				value = t
			}
			add(blank, value)
		}
	}
	for _, rc := range item.RespProcessing.RespConditions {
		if classicRespConditionScore(rc) <= 0 {
			continue
		}
		collect(rc.ConditionVar.VarEquals)
		if rc.ConditionVar.And != nil {
			collect(rc.ConditionVar.And.VarEquals)
		}
		if rc.ConditionVar.Or != nil {
			collect(rc.ConditionVar.Or.VarEquals)
		}
	}
	// Sort blank keys for deterministic output (test stability).
	keys := make([]string, 0, len(accepted))
	for k := range accepted {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	ordered := map[string][]string{}
	for _, k := range keys {
		ordered[k] = accepted[k]
	}
	b, err := json.Marshal(ordered)
	return string(b), nil, err
}

func buildMultipleDropdownAnswers(item classicItem) (string, []ImportWarning, error) {
	// Each <response_lid> corresponds to one dropdown / blank. Canvas uses
	// ident "response_<blank>" with the bare blank name in the material;
	// the question text references "[blank]", so the stored blank_id must
	// be the bare name. The correct option id for each blank is in a
	// respcondition with respident matching the lid ident.
	answers := []map[string]interface{}{}
	for _, lid := range item.Presentation.ResponseLid {
		blankID := canvasBlankID(lid)
		correct := map[string]bool{}
		for _, rc := range item.RespProcessing.RespConditions {
			if classicRespConditionScore(rc) <= 0 {
				continue
			}
			for _, ve := range rc.ConditionVar.VarEquals {
				if ve.Respident == lid.Ident {
					correct[strings.TrimSpace(ve.Value)] = true
				}
			}
		}
		for _, rl := range lid.RenderChoice.ResponseLabels {
			text := firstMatText(rl.Material)
			weight := 0.0
			if correct[rl.Ident] {
				weight = 100.0
			}
			answers = append(answers, map[string]interface{}{
				"id":       rl.Ident,
				"text":     text,
				"weight":   weight,
				"blank_id": blankID,
			})
		}
	}
	b, err := json.Marshal(answers)
	return string(b), nil, err
}

// --- helpers ---

// classicCorrectIDs walks all respconditions for a given response ident
// and returns the set of option idents that have a positive setvar
// score. Used by MC / multi-answer.
func classicCorrectIDs(rcs []classicRespCondition, respident string) map[string]bool {
	correct := map[string]bool{}
	for _, rc := range rcs {
		score := classicRespConditionScore(rc)
		if score <= 0 {
			continue
		}
		for _, ve := range rc.ConditionVar.VarEquals {
			if ve.Respident == "" || ve.Respident == respident {
				correct[ve.Value] = true
			}
		}
		if rc.ConditionVar.And != nil {
			for _, ve := range rc.ConditionVar.And.VarEquals {
				if ve.Respident == "" || ve.Respident == respident {
					correct[ve.Value] = true
				}
			}
		}
		if rc.ConditionVar.Or != nil {
			for _, ve := range rc.ConditionVar.Or.VarEquals {
				if ve.Respident == "" || ve.Respident == respident {
					correct[ve.Value] = true
				}
			}
		}
	}
	return correct
}

// classicVarEqualValues returns all <varequal> chardata from
// respconditions with positive score. Used by short_answer & numerical.
func classicVarEqualValues(rcs []classicRespCondition) []string {
	out := []string{}
	for _, rc := range rcs {
		if classicRespConditionScore(rc) <= 0 {
			continue
		}
		for _, ve := range rc.ConditionVar.VarEquals {
			out = append(out, ve.Value)
		}
		if rc.ConditionVar.And != nil {
			for _, ve := range rc.ConditionVar.And.VarEquals {
				out = append(out, ve.Value)
			}
		}
		if rc.ConditionVar.Or != nil {
			for _, ve := range rc.ConditionVar.Or.VarEquals {
				out = append(out, ve.Value)
			}
		}
	}
	return out
}

// classicRespConditionScore returns the score set by a respcondition.
// Returns 0 for no scoring setvar (assume incorrect) or negative score.
// Score > 0 means "this branch represents a correct answer".
func classicRespConditionScore(rc classicRespCondition) float64 {
	for _, sv := range rc.SetVar {
		if !strings.EqualFold(sv.VarName, "SCORE") && sv.VarName != "" && !strings.EqualFold(sv.VarName, "que_score") {
			continue
		}
		if v, err := strconv.ParseFloat(strings.TrimSpace(sv.Value), 64); err == nil {
			return v
		}
	}
	return 0
}

// firstMatText returns the first non-empty <mattext> chardata across a
// list of <material> blocks.
func firstMatText(materials []classicMaterial) string {
	for _, m := range materials {
		for _, mt := range m.MatText {
			if s := strings.TrimSpace(mt.Value); s != "" {
				return s
			}
		}
	}
	return ""
}

// normalizeFeedbackIdent collapses Canvas's various feedback-ident
// conventions to one of {"correct","incorrect","general","neutral"}
// or returns "" for unrecognized.
func normalizeFeedbackIdent(id string) string {
	l := strings.ToLower(id)
	switch {
	case strings.Contains(l, "correct_fb"), strings.HasPrefix(l, "correct"), strings.Contains(l, "correct_comment"):
		// Order matters: "incorrect" contains "correct", so check
		// "incorrect" first when prefixed.
		if strings.HasPrefix(l, "incorrect") || strings.Contains(l, "incorrect_") {
			return "incorrect"
		}
		return "correct"
	case strings.HasPrefix(l, "incorrect"), strings.Contains(l, "incorrect_"):
		return "incorrect"
	case strings.Contains(l, "general"), strings.Contains(l, "neutral"):
		return "general"
	}
	return ""
}
