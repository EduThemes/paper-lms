package service

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"strconv"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// QTIResult contains the parsed quiz metadata and questions from a QTI XML assessment.
type QTIResult struct {
	Title          string
	Description    string
	QuizType       string
	TimeLimit      *int
	PointsPossible float64
	Questions      []models.QuizQuestion
	// IsQuestionBank is true when the QTI envelope identifies the resource as
	// an item bank (cc.itembank.v0p1 cc_profile, or quiz_type
	// "assessment_question_bank") rather than a delivered assessment.
	IsQuestionBank bool
}

// --- QTI 1.2 XML structures (Canvas primary format) ---

type qtiQuestestinterop struct {
	XMLName     xml.Name        `xml:"questestinterop"`
	Assessments []qtiAssessment `xml:"assessment"`
}

type qtiAssessment struct {
	XMLName  xml.Name      `xml:"assessment"`
	Ident    string        `xml:"ident,attr"`
	Title    string        `xml:"title,attr"`
	MetaData qtiMetaData   `xml:"qtimetadata"`
	Sections []qtiSection  `xml:"section"`
}

type qtiMetaData struct {
	Fields []qtiMetaDataField `xml:"qtimetadatafield"`
}

type qtiMetaDataField struct {
	Label string `xml:"fieldlabel"`
	Entry string `xml:"fieldentry"`
}

type qtiSection struct {
	XMLName  xml.Name   `xml:"section"`
	Ident    string     `xml:"ident,attr"`
	Title    string     `xml:"title,attr"`
	Items    []qtiItem  `xml:"item"`
	Sections []qtiSection `xml:"section"`
}

type qtiItem struct {
	XMLName          xml.Name             `xml:"item"`
	Ident            string               `xml:"ident,attr"`
	Title            string               `xml:"title,attr"`
	MetaData         qtiItemMetaData      `xml:"itemmetadata"`
	Presentation     qtiPresentation      `xml:"presentation"`
	ResponseProcessing qtiResProcessing   `xml:"resprocessing"`
	Feedbacks        []qtiItemFeedback    `xml:"itemfeedback"`
}

type qtiItemMetaData struct {
	Fields []qtiMetaDataField `xml:"qtimetadata>qtimetadatafield"`
}

type qtiPresentation struct {
	Material   qtiMaterial    `xml:"material"`
	Responses  []qtiResponse  `xml:"response_lid"`
	ResponseStr []qtiResponseStr `xml:"response_str"`
}

type qtiMaterial struct {
	MatText qtiMatText `xml:"mattext"`
}

type qtiMatText struct {
	TextType string `xml:"texttype,attr"`
	Text     string `xml:",chardata"`
}

type qtiResponse struct {
	Ident        string          `xml:"ident,attr"`
	RCardinality string          `xml:"rcardinality,attr"`
	// Material is the prompt text on this response (used by matching and
	// multi-dropdown questions, where each prompt or blank is wrapped in its
	// own <response_lid> with the prompt copy in the material).
	Material     qtiMaterial     `xml:"material"`
	RenderChoice qtiRenderChoice `xml:"render_choice"`
}

type qtiResponseStr struct {
	Ident        string       `xml:"ident,attr"`
	RCardinality string       `xml:"rcardinality,attr"`
	Material     qtiMaterial  `xml:"material"`
	RenderFib    qtiRenderFib `xml:"render_fib"`
}

type qtiRenderFib struct {
	Rows    int    `xml:"rows,attr"`
	Columns int    `xml:"columns,attr"`
}

type qtiRenderChoice struct {
	Labels []qtiResponseLabel `xml:"response_label"`
}

type qtiResponseLabel struct {
	Ident    string      `xml:"ident,attr"`
	Material qtiMaterial `xml:"material"`
}

type qtiResProcessing struct {
	Outcomes   qtiOutcomes      `xml:"outcomes"`
	Conditions []qtiResCondition `xml:"respcondition"`
}

type qtiOutcomes struct {
	DecVars []qtiDecVar `xml:"decvar"`
}

type qtiDecVar struct {
	MaxValue string `xml:"maxvalue,attr"`
	MinValue string `xml:"minvalue,attr"`
	VarName  string `xml:"varname,attr"`
	VarType  string `xml:"vartype,attr"`
}

type qtiResCondition struct {
	Continue      string           `xml:"continue,attr"`
	ConditionVar  qtiConditionVar  `xml:"conditionvar"`
	SetVars       []qtiSetVar      `xml:"setvar"`
	DisplayFeedback []qtiDisplayFeedback `xml:"displayfeedback"`
}

type qtiConditionVar struct {
	VarEqual []qtiVarEqual `xml:"varequal"`
	VarGte   []qtiVarRange `xml:"vargte"`
	VarLte   []qtiVarRange `xml:"varlte"`
	And      *qtiCondAnd   `xml:"and"`
	Or       *qtiCondOr    `xml:"or"`
	Other    *struct{}     `xml:"other"`
}

type qtiCondAnd struct {
	VarEqual []qtiVarEqual `xml:"varequal"`
	VarGte   []qtiVarRange `xml:"vargte"`
	VarLte   []qtiVarRange `xml:"varlte"`
}

type qtiCondOr struct {
	VarEqual []qtiVarEqual `xml:"varequal"`
	And      []qtiCondAnd  `xml:"and"`
}

type qtiVarEqual struct {
	RespIdent string `xml:"respident,attr"`
	Case      string `xml:"case,attr"`
	Value     string `xml:",chardata"`
}

type qtiVarRange struct {
	RespIdent string `xml:"respident,attr"`
	Value     string `xml:",chardata"`
}

type qtiSetVar struct {
	VarName string `xml:"varname,attr"`
	Action  string `xml:"action,attr"`
	Value   string `xml:",chardata"`
}

type qtiDisplayFeedback struct {
	FeedbackType string `xml:"feedbacktype,attr"`
	LinkRefID    string `xml:"linkrefid,attr"`
}

type qtiItemFeedback struct {
	Ident    string      `xml:"ident,attr"`
	Material qtiMaterial `xml:"flow_mat>material"`
}

// ParseQTIAssessment parses QTI 1.2/2.1 XML data and returns quiz metadata with questions.
func ParseQTIAssessment(data []byte) (*QTIResult, error) {
	// Try QTI 1.2 first (Canvas primary format)
	result, err := parseQTI12(data)
	if err == nil && result != nil {
		return result, nil
	}

	// Fall back to a simpler parse for variant XML structures
	return parseQTIFallback(data)
}

func parseQTI12(data []byte) (*QTIResult, error) {
	var interop qtiQuestestinterop
	if err := xml.Unmarshal(data, &interop); err != nil {
		return nil, fmt.Errorf("failed to parse QTI XML: %w", err)
	}

	if len(interop.Assessments) == 0 {
		return nil, fmt.Errorf("no assessments found in QTI XML")
	}

	assessment := interop.Assessments[0]
	result := &QTIResult{
		Title:    assessment.Title,
		QuizType: "assignment",
	}

	// Parse assessment-level metadata
	for _, field := range assessment.MetaData.Fields {
		switch field.Label {
		case "cc_maxattempts":
			// Max attempts
		case "qmd_timelimit":
			if tl, err := strconv.Atoi(field.Entry); err == nil {
				result.TimeLimit = &tl
			}
		case "quiz_type":
			result.QuizType = field.Entry
			if field.Entry == "assessment_question_bank" {
				result.IsQuestionBank = true
			}
		case "cc_profile":
			if field.Entry == "cc.itembank.v0p1" {
				result.IsQuestionBank = true
			}
		}
	}

	// Parse items from all sections
	position := 1
	var totalPoints float64
	for _, section := range assessment.Sections {
		items := collectItems(section)
		for _, item := range items {
			question := parseQTIItem(item, position)
			if question.PointsPossible != nil {
				totalPoints += *question.PointsPossible
			}
			result.Questions = append(result.Questions, question)
			position++
		}
	}

	result.PointsPossible = totalPoints

	return result, nil
}

// collectItems recursively collects all items from sections and nested sections.
func collectItems(section qtiSection) []qtiItem {
	items := make([]qtiItem, 0, len(section.Items))
	items = append(items, section.Items...)
	for _, sub := range section.Sections {
		items = append(items, collectItems(sub)...)
	}
	return items
}

func parseQTIItem(item qtiItem, position int) models.QuizQuestion {
	questionType := detectQuestionType(item)
	questionText := extractQuestionText(item)
	points := extractPointsPossible(item)
	answers := extractAnswers(item, questionType)
	correctComments, incorrectComments := extractFeedback(item)

	answersJSON, _ := json.Marshal(answers)

	q := models.QuizQuestion{
		Position:          position,
		QuestionType:      questionType,
		QuestionText:      questionText,
		PointsPossible:    points,
		Answers:           string(answersJSON),
		CorrectComments:   correctComments,
		IncorrectComments: incorrectComments,
		WorkflowState:     "active",
	}

	return q
}

func detectQuestionType(item qtiItem) string {
	// Check metadata for explicit question type (Canvas extension)
	for _, field := range item.MetaData.Fields {
		switch field.Label {
		case "question_type":
			return normalizeQuestionType(field.Entry)
		case "cc_profile":
			// Common Cartridge declares the question shape via cc_profile
			// rather than a Canvas-specific question_type field. Map the
			// well-known profiles back to Canvas question_type strings.
			if t := ccProfileToQuestionType(field.Entry); t != "" {
				return t
			}
		}
	}

	// Infer from item structure
	hasResponseLid := len(item.Presentation.Responses) > 0
	hasResponseStr := len(item.Presentation.ResponseStr) > 0

	if hasResponseStr {
		// Multiple <response_str> elements = fill-in-multiple-blanks
		if len(item.Presentation.ResponseStr) > 1 {
			return "fill_in_multiple_blanks"
		}
		rs := item.Presentation.ResponseStr[0]
		if rs.RenderFib.Rows > 1 {
			return "essay"
		}
		// A single response_str could be either short_answer or numerical;
		// check for vargte/varlte in the conditions to disambiguate.
		if hasNumericalRange(item) {
			return "numerical_question"
		}
		return "short_answer"
	}

	if hasResponseLid {
		resp := item.Presentation.Responses[0]
		labels := resp.RenderChoice.Labels

		// True/false: exactly 2 choices with true/false values
		if len(labels) == 2 {
			texts := make([]string, 2)
			for i, l := range labels {
				texts[i] = strings.ToLower(strings.TrimSpace(extractMatText(l.Material)))
			}
			if (texts[0] == "true" && texts[1] == "false") || (texts[0] == "false" && texts[1] == "true") {
				return "true_false"
			}
		}

		if resp.RCardinality == "Multiple" {
			return "multiple_answers"
		}

		// Multiple response_lid groups: matching by default. Canvas's
		// multiple_dropdowns_question shares the structure but is identified
		// via cc_profile/question_type metadata above, so by the time we
		// fall through here we've ruled it out.
		if len(item.Presentation.Responses) > 1 {
			return "matching"
		}

		return "multiple_choice"
	}

	return "essay"
}

// ccProfileToQuestionType maps a Common Cartridge cc_profile value to the
// Canvas question_type string used in our domain model. Returns "" if the
// profile is unknown.
func ccProfileToQuestionType(profile string) string {
	switch profile {
	case "cc.multiple_choice.v0p1":
		return "multiple_choice"
	case "cc.multiple_response.v0p1":
		return "multiple_answers"
	case "cc.true_false.v0p1":
		return "true_false"
	case "cc.fib.v0p1":
		return "short_answer"
	case "cc.essay.v0p1":
		return "essay"
	case "cc.pattern_match.v0p1":
		return "short_answer"
	}
	return ""
}

// normalizeQuestionType maps the various spellings Canvas uses (with or
// without the "_question" suffix) to the bare form our domain model and
// quiz_service expect (e.g. "multiple_choice_question" → "multiple_choice").
// numerical_question is the one canonical exception that keeps the suffix.
func normalizeQuestionType(t string) string {
	switch t {
	case "multiple_choice_question":
		return "multiple_choice"
	case "true_false_question":
		return "true_false"
	case "short_answer_question":
		return "short_answer"
	case "essay_question":
		return "essay"
	case "matching_question":
		return "matching"
	case "multiple_answers_question":
		return "multiple_answers"
	case "fill_in_multiple_blanks_question":
		return "fill_in_multiple_blanks"
	case "multiple_dropdowns_question":
		return "multiple_dropdowns"
	}
	return t
}

// hasNumericalRange reports whether the item has a vargte/varlte condition,
// which Canvas uses to encode numerical_question range answers in QTI 1.2.
func hasNumericalRange(item qtiItem) bool {
	for _, cond := range item.ResponseProcessing.Conditions {
		if len(cond.ConditionVar.VarGte) > 0 || len(cond.ConditionVar.VarLte) > 0 {
			return true
		}
		if cond.ConditionVar.And != nil &&
			(len(cond.ConditionVar.And.VarGte) > 0 || len(cond.ConditionVar.And.VarLte) > 0) {
			return true
		}
	}
	return false
}

func extractQuestionText(item qtiItem) string {
	text := extractMatText(item.Presentation.Material)
	if text == "" {
		text = item.Title
	}
	return text
}

func extractMatText(mat qtiMaterial) string {
	text := mat.MatText.Text
	if mat.MatText.TextType == "text/html" {
		// Keep HTML as-is for rich text support
		return text
	}
	return html.UnescapeString(text)
}

func extractPointsPossible(item qtiItem) *float64 {
	// Check metadata first
	for _, field := range item.MetaData.Fields {
		if field.Label == "points_possible" {
			if pts, err := strconv.ParseFloat(field.Entry, 64); err == nil {
				return &pts
			}
		}
	}

	// Check outcomes decvar
	for _, dv := range item.ResponseProcessing.Outcomes.DecVars {
		if dv.MaxValue != "" {
			if pts, err := strconv.ParseFloat(dv.MaxValue, 64); err == nil {
				return &pts
			}
		}
	}

	// Default to 1 point
	defaultPts := 1.0
	return &defaultPts
}

// answerChoice represents a single answer option for a quiz question.
//
// The shape matches Canvas's quiz_question.answers JSON column. Most question
// types only set ID/Text/Weight; matching, multi-blank/dropdown, and numerical
// questions populate the additional fields below.
type answerChoice struct {
	ID       string  `json:"id"`
	Text     string  `json:"text"`
	Comments string  `json:"comments,omitempty"`
	Weight   float64 `json:"weight"` // 100 = correct, 0 = incorrect

	// Matching: id of the correct match in the question's `matches` list.
	MatchID string `json:"match_id,omitempty"`

	// fill_in_multiple_blanks / multiple_dropdowns: the named blank this
	// answer is associated with.
	BlankID string `json:"blank_id,omitempty"`

	// Numerical: one of "exact_answer", "range_answer".
	NumericalAnswerType string   `json:"numerical_answer_type,omitempty"`
	Exact               *float64 `json:"exact,omitempty"`
	Margin              *float64 `json:"margin,omitempty"`
	Start               *float64 `json:"start,omitempty"`
	End                 *float64 `json:"end,omitempty"`
}

// matchOption is a single right-hand-side option in a matching question.
// Stored on QuizQuestion alongside `answers` as the question's `matches` JSON.
type matchOption struct {
	MatchID string `json:"match_id"`
	Text    string `json:"text"`
}

func extractAnswers(item qtiItem, questionType string) []answerChoice {
	switch questionType {
	case "multiple_choice", "true_false", "multiple_answers":
		return extractMultipleChoiceAnswers(item)
	case "short_answer":
		return extractShortAnswers(item)
	case "fill_in_multiple_blanks":
		return extractFillInMultipleBlanksAnswers(item)
	case "multiple_dropdowns":
		return extractMultipleDropdownsAnswers(item)
	case "matching":
		return extractMatchingAnswers(item)
	case "numerical_question":
		return extractNumericalAnswers(item)
	case "essay":
		return nil
	default:
		return extractMultipleChoiceAnswers(item)
	}
}

func extractMultipleChoiceAnswers(item qtiItem) []answerChoice {
	if len(item.Presentation.Responses) == 0 {
		return nil
	}

	resp := item.Presentation.Responses[0]
	labels := resp.RenderChoice.Labels

	// Build a map of label ident -> correct weight
	correctMap := buildCorrectMap(item)

	answers := make([]answerChoice, 0, len(labels))
	for _, label := range labels {
		weight := 0.0
		if w, ok := correctMap[label.Ident]; ok {
			weight = w
		}

		answers = append(answers, answerChoice{
			ID:     label.Ident,
			Text:   extractMatText(label.Material),
			Weight: weight,
		})
	}

	return answers
}

// extractShortAnswers walks each respcondition that scores points and emits
// every accepted text answer. Each varequal under a scoring condition becomes
// one accepted answer string.
func extractShortAnswers(item qtiItem) []answerChoice {
	var answers []answerChoice
	for _, cond := range item.ResponseProcessing.Conditions {
		if !conditionScores(cond) {
			continue
		}
		for _, ve := range cond.ConditionVar.VarEqual {
			answers = append(answers, answerChoice{
				ID:     ve.Value,
				Text:   ve.Value,
				Weight: 100,
			})
		}
		if cond.ConditionVar.And != nil {
			for _, ve := range cond.ConditionVar.And.VarEqual {
				answers = append(answers, answerChoice{
					ID:     ve.Value,
					Text:   ve.Value,
					Weight: 100,
				})
			}
		}
	}
	return answers
}

// extractMatchingAnswers turns Canvas's matching-question QTI shape into an
// ordered list of {prompt, correct match} pairs plus the full pool of match
// options (so distractors aren't lost). Match options are appended with
// BlankID set to the sentinel "_match_options_" so consumers can split the
// two halves cleanly.
//
// Canvas emits one <response_lid ident="response_<answer_id>"> per prompt,
// each carrying the prompt text in its material element and the same
// render_choice list of match candidates. Correct pairings live in
// respconditions: <varequal respident="response_<answer_id>"><match_id></varequal>.
func extractMatchingAnswers(item qtiItem) []answerChoice {
	// Map respident → correct match_id from the resprocessing conditions.
	correctByRespident := make(map[string]string)
	for _, cond := range item.ResponseProcessing.Conditions {
		if !conditionScores(cond) {
			continue
		}
		for _, ve := range cond.ConditionVar.VarEqual {
			correctByRespident[ve.RespIdent] = ve.Value
		}
		if cond.ConditionVar.And != nil {
			for _, ve := range cond.ConditionVar.And.VarEqual {
				correctByRespident[ve.RespIdent] = ve.Value
			}
		}
	}

	answers := make([]answerChoice, 0, len(item.Presentation.Responses))
	matchSet := make(map[string]string) // match_id → text
	matchOrder := make([]string, 0)

	for _, resp := range item.Presentation.Responses {
		// Strip the conventional "response_" prefix so the answer ID matches
		// what's referenced from elsewhere (e.g. exporter migration_ids).
		answerID := strings.TrimPrefix(resp.Ident, "response_")
		promptText := extractMatText(resp.Material)
		matchID := correctByRespident[resp.Ident]

		answers = append(answers, answerChoice{
			ID:      answerID,
			Text:    promptText,
			MatchID: matchID,
			Weight:  100,
		})

		for _, label := range resp.RenderChoice.Labels {
			if _, seen := matchSet[label.Ident]; seen {
				continue
			}
			matchSet[label.Ident] = extractMatText(label.Material)
			matchOrder = append(matchOrder, label.Ident)
		}
	}

	// Append the full match pool as separate entries flagged with the
	// "_match_options_" blank_id sentinel.
	for _, mid := range matchOrder {
		answers = append(answers, answerChoice{
			ID:      mid,
			Text:    matchSet[mid],
			BlankID: "_match_options_",
		})
	}

	return answers
}

// extractFillInMultipleBlanksAnswers handles the Canvas QTI 1.2 shape where
// every blank is its own <response_lid> (or <response_str>) with the blank's
// label in the material. Correct answers come from varequal in scoring
// respconditions, keyed by respident → blank_id.
func extractFillInMultipleBlanksAnswers(item qtiItem) []answerChoice {
	// blank_id by respident, derived from the response material.
	blankByRespident := make(map[string]string)
	respidentOrder := make([]string, 0)
	for _, resp := range item.Presentation.Responses {
		blankByRespident[resp.Ident] = textOrIdent(extractMatText(resp.Material), resp.Ident)
		respidentOrder = append(respidentOrder, resp.Ident)
	}
	for _, rs := range item.Presentation.ResponseStr {
		blankByRespident[rs.Ident] = textOrIdent(extractMatText(rs.Material), rs.Ident)
		respidentOrder = append(respidentOrder, rs.Ident)
	}

	var answers []answerChoice
	for _, cond := range item.ResponseProcessing.Conditions {
		if !conditionScores(cond) {
			continue
		}
		varEquals := append([]qtiVarEqual{}, cond.ConditionVar.VarEqual...)
		if cond.ConditionVar.And != nil {
			varEquals = append(varEquals, cond.ConditionVar.And.VarEqual...)
		}
		for _, ve := range varEquals {
			blankID := blankByRespident[ve.RespIdent]
			if blankID == "" {
				blankID = ve.RespIdent
			}
			answers = append(answers, answerChoice{
				ID:      ve.Value,
				Text:    ve.Value,
				BlankID: blankID,
				Weight:  100,
			})
		}
	}

	return answers
}

// extractMultipleDropdownsAnswers parses the QTI 1.2 multi-dropdown shape:
// one <response_lid> per blank carrying the blank's name in the material and
// the dropdown choices in render_choice. Each render_choice option becomes an
// answer entry; correct ones are flagged via scoring respconditions.
func extractMultipleDropdownsAnswers(item qtiItem) []answerChoice {
	correct := make(map[string]map[string]bool) // respident → choice ident → correct
	for _, cond := range item.ResponseProcessing.Conditions {
		if !conditionScores(cond) {
			continue
		}
		varEquals := append([]qtiVarEqual{}, cond.ConditionVar.VarEqual...)
		if cond.ConditionVar.And != nil {
			varEquals = append(varEquals, cond.ConditionVar.And.VarEqual...)
		}
		for _, ve := range varEquals {
			if correct[ve.RespIdent] == nil {
				correct[ve.RespIdent] = map[string]bool{}
			}
			correct[ve.RespIdent][ve.Value] = true
		}
	}

	var answers []answerChoice
	for _, resp := range item.Presentation.Responses {
		blankID := textOrIdent(extractMatText(resp.Material), resp.Ident)
		for _, label := range resp.RenderChoice.Labels {
			weight := 0.0
			if correct[resp.Ident][label.Ident] {
				weight = 100
			}
			answers = append(answers, answerChoice{
				ID:      label.Ident,
				Text:    extractMatText(label.Material),
				BlankID: blankID,
				Weight:  weight,
			})
		}
	}
	return answers
}

// extractNumericalAnswers parses both exact and range numerical answers per
// QTI 1.2 conventions. Canvas encodes:
//   - exact match: <varequal>3.14</varequal>
//   - exact with margin: <or><varequal>v</varequal><and><vargte>v-m</vargte><varlte>v+m</varlte></and></or>
//   - range: <vargte>lo</vargte><varlte>hi</varlte> (bare or wrapped in <and>)
func extractNumericalAnswers(item qtiItem) []answerChoice {
	var answers []answerChoice
	for _, cond := range item.ResponseProcessing.Conditions {
		if !conditionScores(cond) {
			continue
		}

		// Range answer: vargte+varlte siblings (bare or under <and>).
		gte, lte := collectRangeBounds(cond)
		if gte != nil && lte != nil {
			ans := answerChoice{
				ID:                  fmt.Sprintf("range_%g_%g", *gte, *lte),
				Text:                fmt.Sprintf("%g..%g", *gte, *lte),
				Weight:              100,
				NumericalAnswerType: "range_answer",
				Start:               gte,
				End:                 lte,
			}
			answers = append(answers, ans)
			continue
		}

		// Exact answer (optionally wrapped in <or> with a margin <and> branch).
		exacts := cond.ConditionVar.VarEqual
		var orMargin *float64
		if cond.ConditionVar.Or != nil {
			exacts = append(exacts, cond.ConditionVar.Or.VarEqual...)
			for _, sub := range cond.ConditionVar.Or.And {
				if len(sub.VarGte) > 0 && len(sub.VarLte) > 0 {
					if lo, errLo := strconv.ParseFloat(sub.VarGte[0].Value, 64); errLo == nil {
						if hi, errHi := strconv.ParseFloat(sub.VarLte[0].Value, 64); errHi == nil {
							m := (hi - lo) / 2
							orMargin = &m
						}
					}
				}
			}
		}
		for _, ve := range exacts {
			val, err := strconv.ParseFloat(ve.Value, 64)
			if err != nil {
				continue
			}
			v := val
			ans := answerChoice{
				ID:                  fmt.Sprintf("exact_%g", val),
				Text:                ve.Value,
				Weight:              100,
				NumericalAnswerType: "exact_answer",
				Exact:               &v,
			}
			if orMargin != nil {
				m := *orMargin
				ans.Margin = &m
			}
			answers = append(answers, ans)
		}
	}
	return answers
}

// collectRangeBounds extracts the (gte, lte) pair from a respcondition that
// represents a numerical range answer. Returns (nil, nil) if either bound is
// missing. Looks at both bare conditionvar children and <and>-wrapped ones.
func collectRangeBounds(cond qtiResCondition) (gte, lte *float64) {
	gtes := append([]qtiVarRange{}, cond.ConditionVar.VarGte...)
	ltes := append([]qtiVarRange{}, cond.ConditionVar.VarLte...)
	if cond.ConditionVar.And != nil {
		gtes = append(gtes, cond.ConditionVar.And.VarGte...)
		ltes = append(ltes, cond.ConditionVar.And.VarLte...)
	}
	if len(gtes) > 0 {
		if v, err := strconv.ParseFloat(gtes[0].Value, 64); err == nil {
			gte = &v
		}
	}
	if len(ltes) > 0 {
		if v, err := strconv.ParseFloat(ltes[0].Value, 64); err == nil {
			lte = &v
		}
	}
	return gte, lte
}

// conditionScores reports whether a respcondition awards points (i.e. its
// setvar sets a positive SCORE / numeric outcome value).
func conditionScores(cond qtiResCondition) bool {
	for _, sv := range cond.SetVars {
		if v, err := strconv.ParseFloat(sv.Value, 64); err == nil && v > 0 {
			return true
		}
	}
	return false
}

func textOrIdent(text, ident string) string {
	t := strings.TrimSpace(text)
	if t != "" {
		return t
	}
	return ident
}

func buildCorrectMap(item qtiItem) map[string]float64 {
	correctMap := make(map[string]float64)
	for _, cond := range item.ResponseProcessing.Conditions {
		// Determine the score for this condition
		score := 0.0
		for _, sv := range cond.SetVars {
			if v, err := strconv.ParseFloat(sv.Value, 64); err == nil && v > 0 {
				score = 100.0
				break
			}
		}

		// Map each varequal ident to the score
		for _, ve := range cond.ConditionVar.VarEqual {
			if score > 0 {
				correctMap[ve.Value] = score
			}
		}

		// Check inside <and> block
		if cond.ConditionVar.And != nil {
			for _, ve := range cond.ConditionVar.And.VarEqual {
				if score > 0 {
					correctMap[ve.Value] = score
				}
			}
		}
	}
	return correctMap
}

func extractFeedback(item qtiItem) (correctComments string, incorrectComments string) {
	feedbackMap := make(map[string]string)
	for _, fb := range item.Feedbacks {
		text := extractMatText(fb.Material)
		feedbackMap[fb.Ident] = text
	}

	// Scan respconditions for feedback references
	for _, cond := range item.ResponseProcessing.Conditions {
		isCorrect := false
		for _, sv := range cond.SetVars {
			if v, err := strconv.ParseFloat(sv.Value, 64); err == nil && v > 0 {
				isCorrect = true
				break
			}
		}

		for _, df := range cond.DisplayFeedback {
			text := feedbackMap[df.LinkRefID]
			if text == "" {
				continue
			}
			if isCorrect {
				correctComments = text
			} else {
				incorrectComments = text
			}
		}
	}

	// Also check for well-known Canvas feedback idents
	if text, ok := feedbackMap["correct_fb"]; ok && correctComments == "" {
		correctComments = text
	}
	if text, ok := feedbackMap["general_correct_fb"]; ok && correctComments == "" {
		correctComments = text
	}
	if text, ok := feedbackMap["incorrect_fb"]; ok && incorrectComments == "" {
		incorrectComments = text
	}
	if text, ok := feedbackMap["general_incorrect_fb"]; ok && incorrectComments == "" {
		incorrectComments = text
	}

	return correctComments, incorrectComments
}

// parseQTIFallback attempts a simpler parse for non-standard or QTI 2.1 XML.
func parseQTIFallback(data []byte) (*QTIResult, error) {
	// Try parsing as a single assessment element (no questestinterop wrapper)
	type simpleAssessment struct {
		XMLName  xml.Name     `xml:"assessment"`
		Title    string       `xml:"title,attr"`
		Sections []qtiSection `xml:"section"`
	}

	var sa simpleAssessment
	if err := xml.Unmarshal(data, &sa); err != nil {
		return nil, fmt.Errorf("failed to parse QTI XML in fallback mode: %w", err)
	}

	result := &QTIResult{
		Title:    sa.Title,
		QuizType: "assignment",
	}

	position := 1
	var totalPoints float64
	for _, section := range sa.Sections {
		items := collectItems(section)
		for _, item := range items {
			question := parseQTIItem(item, position)
			if question.PointsPossible != nil {
				totalPoints += *question.PointsPossible
			}
			result.Questions = append(result.Questions, question)
			position++
		}
	}
	result.PointsPossible = totalPoints

	return result, nil
}
