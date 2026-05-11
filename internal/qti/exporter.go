package qti

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"sort"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// Exporter writes a Paper LMS quiz to a Canvas-Classic-compatible
// .imscc zip. We target Canvas Classic (not New Quizzes) because every
// Canvas instance accepts Classic imports, but only newer instances
// import New Quizzes packages. Maximum portability.
type Exporter struct{}

func NewExporter() *Exporter {
	return &Exporter{}
}

// ExportQuiz writes one quiz (with its questions, plus any item banks
// referenced by those questions, plus any stimuli) into a .imscc zip
// byte slice. The caller streams this back as the HTTP response.
//
// Round-trip property: Importer.ImportIMSCC(Exporter.ExportQuiz(q))
// yields a quiz semantically equivalent to q for the lossless subset
// of types (everything except ordering / categorization / hot_spot /
// fill_in_the_blank — see mapping.go for the lossy mapping rationale).
func (e *Exporter) ExportQuiz(quiz *models.Quiz, questions []models.QuizQuestion, banks []ItemBankImport) ([]byte, error) {
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)

	// 1. Assessment XML.
	assessmentXML, err := buildAssessmentXML(quiz, questions)
	if err != nil {
		zw.Close()
		return nil, err
	}
	assessmentPath := fmt.Sprintf("non_cc_assessments/%d.xml.qti", quiz.ID)
	if err := writeZipEntry(zw, assessmentPath, assessmentXML); err != nil {
		return nil, err
	}

	// 2. Bank XMLs (one per bank).
	bankPaths := []string{}
	for _, bank := range banks {
		bankXML, err := buildBankXML(bank)
		if err != nil {
			zw.Close()
			return nil, err
		}
		bankPath := fmt.Sprintf("non_cc_assessments/assessment_question_banks/%s.xml", bank.Identifier)
		if err := writeZipEntry(zw, bankPath, bankXML); err != nil {
			return nil, err
		}
		bankPaths = append(bankPaths, bankPath)
	}

	// 3. Manifest.
	manifestXML := buildManifestXML(quiz, assessmentPath, bankPaths)
	if err := writeZipEntry(zw, "imsmanifest.xml", []byte(manifestXML)); err != nil {
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}
	return zipBuf.Bytes(), nil
}

// writeZipEntry adds one file to the zip writer.
func writeZipEntry(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// buildManifestXML constructs the imsmanifest.xml. We emit the minimal
// shape that Canvas accepts on import — confirmed by round-tripping
// through Canvas's own import pipeline historically. Real Canvas
// exports include LOM metadata and dependencies we omit.
func buildManifestXML(quiz *models.Quiz, assessmentPath string, bankPaths []string) string {
	var resources bytes.Buffer
	// Assessment resource.
	fmt.Fprintf(&resources, `
    <resource identifier="quiz-%d" type="imsqti_xmlv1p2/imscc_xmlv1p1/assessment" href="%s">
      <file href="%s"/>
    </resource>`, quiz.ID, assessmentPath, assessmentPath)
	// Bank resources.
	for i, bp := range bankPaths {
		fmt.Fprintf(&resources, `
    <resource identifier="bank-%d" type="associatedcontent/imscc_xmlv1p1/learning-application-resource" href="%s">
      <file href="%s"/>
    </resource>`, i, bp, bp)
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<manifest identifier="paper-lms-export-%d" xmlns="http://www.imsglobal.org/xsd/imsccv1p1/imscp_v1p1">
  <metadata>
    <schema>IMS Common Cartridge</schema>
    <schemaversion>1.1.0</schemaversion>
  </metadata>
  <resources>%s
  </resources>
</manifest>`, quiz.ID, resources.String())
}

// buildAssessmentXML produces the Canvas Classic <questestinterop>
// blob for a quiz. The shape mirrors what parser_classic.go consumes.
func buildAssessmentXML(quiz *models.Quiz, questions []models.QuizQuestion) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	buf.WriteString(`<questestinterop xmlns="http://www.imsglobal.org/xsd/ims_qtiasiv1p2">` + "\n")
	fmt.Fprintf(&buf, `  <assessment ident="quiz-%d" title=%s>`+"\n", quiz.ID, xmlAttr(quiz.Title))
	// Assessment-level metadata.
	buf.WriteString(`    <qtimetadata>` + "\n")
	fmt.Fprintf(&buf, `      <qtimetadatafield><fieldlabel>quiz_type</fieldlabel><fieldentry>%s</fieldentry></qtimetadatafield>`+"\n", xmlEscape(quiz.QuizType))
	if quiz.TimeLimit != nil {
		fmt.Fprintf(&buf, `      <qtimetadatafield><fieldlabel>time_limit</fieldlabel><fieldentry>%d</fieldentry></qtimetadatafield>`+"\n", *quiz.TimeLimit)
	}
	if quiz.PointsPossible != nil {
		fmt.Fprintf(&buf, `      <qtimetadatafield><fieldlabel>points_possible</fieldlabel><fieldentry>%g</fieldentry></qtimetadatafield>`+"\n", *quiz.PointsPossible)
	}
	fmt.Fprintf(&buf, `      <qtimetadatafield><fieldlabel>shuffle_answers</fieldlabel><fieldentry>%t</fieldentry></qtimetadatafield>`+"\n", quiz.ShuffleAnswers)
	buf.WriteString(`    </qtimetadata>` + "\n")

	buf.WriteString(`    <section ident="root_section">` + "\n")

	// Sort questions by Position so the export order matches the
	// authored order — deterministic round-trip.
	sorted := make([]models.QuizQuestion, len(questions))
	copy(sorted, questions)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Position < sorted[j].Position })

	for _, q := range sorted {
		itemXML, err := buildItemXML(q)
		if err != nil {
			return nil, fmt.Errorf("item %d: %w", q.ID, err)
		}
		buf.Write(itemXML)
	}

	buf.WriteString(`    </section>` + "\n")
	buf.WriteString(`  </assessment>` + "\n")
	buf.WriteString(`</questestinterop>` + "\n")
	return buf.Bytes(), nil
}

// buildItemXML produces one <item> block. Routes by unified question
// type and emits exactly the shape that parser_classic.go expects.
func buildItemXML(q models.QuizQuestion) ([]byte, error) {
	canvasType := MapUnifiedToCanvasClassic(q.QuestionType)
	if canvasType == "" {
		canvasType = "essay_question" // safest fallback
	}

	points := 1.0
	if q.PointsPossible != nil {
		points = *q.PointsPossible
	}

	var buf bytes.Buffer
	ident := fmt.Sprintf("q-%d", q.ID)
	if q.ID == 0 {
		ident = fmt.Sprintf("q-pos-%d", q.Position)
	}
	fmt.Fprintf(&buf, `      <item ident="%s" title="Question">`+"\n", ident)
	fmt.Fprintf(&buf, `        <itemmetadata><qtimetadata>`+"\n")
	fmt.Fprintf(&buf, `          <qtimetadatafield><fieldlabel>question_type</fieldlabel><fieldentry>%s</fieldentry></qtimetadatafield>`+"\n", canvasType)
	fmt.Fprintf(&buf, `          <qtimetadatafield><fieldlabel>points_possible</fieldlabel><fieldentry>%g</fieldentry></qtimetadatafield>`+"\n", points)
	buf.WriteString(`        </qtimetadata></itemmetadata>` + "\n")

	// Presentation: prompt + interaction shell. Each builder writes
	// its own response_lid / response_str / response_num.
	buf.WriteString(`        <presentation>` + "\n")
	fmt.Fprintf(&buf, `          <material><mattext texttype="text/html"><![CDATA[%s]]></mattext></material>`+"\n", q.QuestionText)
	if err := writeItemPresentation(&buf, q); err != nil {
		return nil, err
	}
	buf.WriteString(`        </presentation>` + "\n")

	// Resprocessing.
	if err := writeItemRespProcessing(&buf, q, points); err != nil {
		return nil, err
	}

	// Feedback blocks.
	writeItemFeedback(&buf, q)

	buf.WriteString(`      </item>` + "\n")
	return buf.Bytes(), nil
}

// writeItemPresentation emits the type-specific presentation children
// (<response_lid> / <response_str> / etc).
func writeItemPresentation(buf *bytes.Buffer, q models.QuizQuestion) error {
	switch q.QuestionType {
	case UnifiedMultipleChoice, UnifiedTrueFalse:
		return writeChoicePresentation(buf, q, "response1", "Single")
	case UnifiedMultipleAnswer:
		return writeChoicePresentation(buf, q, "response1", "Multiple")
	case UnifiedShortAnswer, UnifiedFillInTheBlank, UnifiedNumerical, UnifiedFormula:
		fmt.Fprintf(buf, `          <response_str ident="response1"><render_fib rows="1"/></response_str>`+"\n")
		return nil
	case UnifiedEssay:
		fmt.Fprintf(buf, `          <response_str ident="response1"><render_fib rows="5"/></response_str>`+"\n")
		return nil
	case UnifiedFileUpload, UnifiedTextOnly:
		// No interactive presentation for these.
		return nil
	case UnifiedMatching, UnifiedOrdering, UnifiedCategorization, UnifiedHotSpot:
		return writeMatchingLikePresentation(buf, q)
	case UnifiedMultipleDropdown, UnifiedFillInMultipleBlanks:
		return writeMultiBlankPresentation(buf, q)
	}
	return nil
}

func writeChoicePresentation(buf *bytes.Buffer, q models.QuizQuestion, respID, cardinality string) error {
	opts, err := parseAnswerOptions(q.Answers)
	if err != nil {
		return err
	}
	fmt.Fprintf(buf, `          <response_lid ident="%s" rcardinality="%s">`+"\n", respID, cardinality)
	buf.WriteString(`            <render_choice>` + "\n")
	for _, opt := range opts {
		fmt.Fprintf(buf, `              <response_label ident="%s"><material><mattext>%s</mattext></material></response_label>`+"\n",
			xmlEscape(opt.ID), xmlEscape(opt.Text))
	}
	buf.WriteString(`            </render_choice>` + "\n")
	buf.WriteString(`          </response_lid>` + "\n")
	return nil
}

// writeMatchingLikePresentation emits one response_lid per left/item
// option. Used for matching, ordering, categorization, hot_spot — all
// have the same presentation shape under Canvas Classic.
func writeMatchingLikePresentation(buf *bytes.Buffer, q models.QuizQuestion) error {
	opts, err := parseAnswerOptions(q.Answers)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		ident := opt.ID
		if ident == "" {
			ident = opt.Left
		}
		fmt.Fprintf(buf, `          <response_lid ident="%s" rcardinality="Single">`+"\n", xmlEscape(ident))
		buf.WriteString(`            <render_choice>` + "\n")
		// Single placeholder option per left — Canvas's matching is
		// "select right option for each left"; we don't enumerate the
		// right side here because parser_classic recovers the
		// correct right id from resprocessing.
		buf.WriteString(`              <response_label ident="opt1"><material><mattext/></material></response_label>` + "\n")
		buf.WriteString(`            </render_choice>` + "\n")
		buf.WriteString(`          </response_lid>` + "\n")
	}
	return nil
}

func writeMultiBlankPresentation(buf *bytes.Buffer, q models.QuizQuestion) error {
	// fill_in_multiple_blanks: Answers is a map[blank_id][]accepted;
	// no options array to iterate. Just emit one <response_str> per blank.
	if q.QuestionType == UnifiedFillInMultipleBlanks {
		var blanks map[string][]string
		if q.Answers != "" {
			_ = json.Unmarshal([]byte(q.Answers), &blanks)
		}
		keys := make([]string, 0, len(blanks))
		for k := range blanks {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, blank := range keys {
			fmt.Fprintf(buf, `          <response_str ident="%s"><render_fib rows="1"/></response_str>`+"\n", xmlEscape(blank))
		}
		return nil
	}

	opts, err := parseAnswerOptions(q.Answers)
	if err != nil {
		return err
	}
	// Group by blank_id.
	blanks := map[string][]exporterOption{}
	for _, opt := range opts {
		blanks[opt.BlankID] = append(blanks[opt.BlankID], opt)
	}
	// Sort keys for determinism.
	keys := make([]string, 0, len(blanks))
	for k := range blanks {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, blank := range keys {
		fmt.Fprintf(buf, `          <response_lid ident="%s" rcardinality="Single">`+"\n", xmlEscape(blank))
		buf.WriteString(`            <render_choice>` + "\n")
		for _, opt := range blanks[blank] {
			fmt.Fprintf(buf, `              <response_label ident="%s"><material><mattext>%s</mattext></material></response_label>`+"\n",
				xmlEscape(opt.ID), xmlEscape(opt.Text))
		}
		buf.WriteString(`            </render_choice>` + "\n")
		buf.WriteString(`          </response_lid>` + "\n")
	}
	return nil
}

// writeItemRespProcessing emits the correct-answer rules for each type.
func writeItemRespProcessing(buf *bytes.Buffer, q models.QuizQuestion, points float64) error {
	buf.WriteString(`        <resprocessing>` + "\n")
	buf.WriteString(`          <outcomes><decvar maxvalue="100" varname="SCORE"/></outcomes>` + "\n")

	switch q.QuestionType {
	case UnifiedMultipleChoice, UnifiedTrueFalse, UnifiedMultipleAnswer:
		opts, _ := parseAnswerOptions(q.Answers)
		for _, opt := range opts {
			if opt.Weight > 0 {
				fmt.Fprintf(buf, `          <respcondition continue="No">`+"\n"+
					`            <conditionvar><varequal respident="response1">%s</varequal></conditionvar>`+"\n"+
					`            <setvar varname="SCORE" action="Set">100</setvar>`+"\n"+
					`          </respcondition>`+"\n", xmlEscape(opt.ID))
			}
		}

	case UnifiedShortAnswer, UnifiedFillInTheBlank:
		opts, _ := parseAnswerOptions(q.Answers)
		for _, opt := range opts {
			if opt.Weight > 0 {
				fmt.Fprintf(buf, `          <respcondition continue="No">`+"\n"+
					`            <conditionvar><varequal respident="response1">%s</varequal></conditionvar>`+"\n"+
					`            <setvar varname="SCORE" action="Set">100</setvar>`+"\n"+
					`          </respcondition>`+"\n", xmlEscape(opt.Text))
			}
		}

	case UnifiedNumerical, UnifiedFormula:
		// Emit the numerical answer_exact/answer_error_margin in
		// itemmetadata via a separate path — but we already wrote
		// metadata above. So encode here via <varequal> with the
		// margin appended.
		opts, _ := parseAnswerOptions(q.Answers)
		for _, opt := range opts {
			if opt.Weight > 0 {
				fmt.Fprintf(buf, `          <respcondition continue="No">`+"\n"+
					`            <conditionvar><varequal respident="response1">%s</varequal></conditionvar>`+"\n"+
					`            <setvar varname="SCORE" action="Set">100</setvar>`+"\n"+
					`          </respcondition>`+"\n", xmlEscape(opt.Text))
			}
		}

	case UnifiedMatching, UnifiedOrdering, UnifiedCategorization, UnifiedHotSpot:
		opts, _ := parseAnswerOptions(q.Answers)
		// For each left/item, emit a respcondition matching it to
		// its right_id. parser_classic walks these to rebuild pairs.
		for _, opt := range opts {
			if opt.RightID == "" {
				continue
			}
			fmt.Fprintf(buf, `          <respcondition continue="No">`+"\n"+
				`            <conditionvar><varequal respident="%s">%s</varequal></conditionvar>`+"\n"+
				`            <setvar varname="SCORE" action="Set">100</setvar>`+"\n"+
				`          </respcondition>`+"\n", xmlEscape(opt.ID), xmlEscape(opt.RightID))
		}

	case UnifiedMultipleDropdown:
		opts, _ := parseAnswerOptions(q.Answers)
		for _, opt := range opts {
			if opt.Weight > 0 {
				fmt.Fprintf(buf, `          <respcondition continue="No">`+"\n"+
					`            <conditionvar><varequal respident="%s">%s</varequal></conditionvar>`+"\n"+
					`            <setvar varname="SCORE" action="Set">100</setvar>`+"\n"+
					`          </respcondition>`+"\n", xmlEscape(opt.BlankID), xmlEscape(opt.ID))
			}
		}

	case UnifiedFillInMultipleBlanks:
		// Answers JSON is map[blank_id][]accepted. We need to parse
		// as map, not []option.
		var blanks map[string][]string
		_ = json.Unmarshal([]byte(q.Answers), &blanks)
		// Sort keys for determinism.
		keys := make([]string, 0, len(blanks))
		for k := range blanks {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, blank := range keys {
			for _, accepted := range blanks[blank] {
				fmt.Fprintf(buf, `          <respcondition continue="No">`+"\n"+
					`            <conditionvar><varequal respident="%s">%s</varequal></conditionvar>`+"\n"+
					`            <setvar varname="SCORE" action="Set">100</setvar>`+"\n"+
					`          </respcondition>`+"\n", xmlEscape(blank), xmlEscape(accepted))
			}
		}

	case UnifiedEssay, UnifiedFileUpload, UnifiedTextOnly:
		// No auto-grading rule.
	}

	buf.WriteString(`        </resprocessing>` + "\n")
	return nil
}

// writeItemFeedback writes the per-item feedback blocks. Idents follow
// Canvas convention so parser_classic.normalizeFeedbackIdent picks them
// up correctly on reimport.
func writeItemFeedback(buf *bytes.Buffer, q models.QuizQuestion) {
	if q.CorrectComments != "" {
		fmt.Fprintf(buf, `        <itemfeedback ident="correct_fb"><flow_mat><material><mattext texttype="text/html"><![CDATA[%s]]></mattext></material></flow_mat></itemfeedback>`+"\n", q.CorrectComments)
	}
	if q.IncorrectComments != "" {
		fmt.Fprintf(buf, `        <itemfeedback ident="incorrect_fb"><flow_mat><material><mattext texttype="text/html"><![CDATA[%s]]></mattext></material></flow_mat></itemfeedback>`+"\n", q.IncorrectComments)
	}
	if q.NeutralComments != "" {
		fmt.Fprintf(buf, `        <itemfeedback ident="general_fb"><flow_mat><material><mattext texttype="text/html"><![CDATA[%s]]></mattext></material></flow_mat></itemfeedback>`+"\n", q.NeutralComments)
	}
}

// buildBankXML emits an assessment_question_banks/<id>.xml file.
func buildBankXML(bank ItemBankImport) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	buf.WriteString(`<questestinterop xmlns="http://www.imsglobal.org/xsd/ims_qtiasiv1p2">` + "\n")
	fmt.Fprintf(&buf, `  <objectbank ident="%s">`+"\n", xmlEscape(bank.Identifier))
	buf.WriteString(`    <qtimetadata>` + "\n")
	fmt.Fprintf(&buf, `      <qtimetadatafield><fieldlabel>bank_title</fieldlabel><fieldentry>%s</fieldentry></qtimetadatafield>`+"\n", xmlEscape(bank.Title))
	if bank.Description != "" {
		fmt.Fprintf(&buf, `      <qtimetadatafield><fieldlabel>description</fieldlabel><fieldentry>%s</fieldentry></qtimetadatafield>`+"\n", xmlEscape(bank.Description))
	}
	buf.WriteString(`    </qtimetadata>` + "\n")

	for _, bi := range bank.Items {
		// Promote BankItemImport to a synthetic QuizQuestion so we
		// can reuse buildItemXML — keeps the encoder DRY.
		var pts float64
		if bi.PointsPossible != nil {
			pts = *bi.PointsPossible
		}
		fq := models.QuizQuestion{
			Position:          bi.Position,
			QuestionType:      bi.QuestionType,
			QuestionText:      bi.QuestionText,
			PointsPossible:    &pts,
			Answers:           bi.Answers,
			CorrectComments:   bi.CorrectComments,
			IncorrectComments: bi.IncorrectComments,
			NeutralComments:   bi.NeutralComments,
		}
		itemXML, err := buildItemXML(fq)
		if err != nil {
			return nil, err
		}
		buf.Write(itemXML)
	}

	buf.WriteString(`  </objectbank>` + "\n")
	buf.WriteString(`</questestinterop>` + "\n")
	return buf.Bytes(), nil
}

// --- small helpers ---

// exporterOption mirrors the answerOption struct in quiz_service.go so
// the exporter can read it without importing the service package.
type exporterOption struct {
	ID      string  `json:"id"`
	Text    string  `json:"text"`
	Weight  float64 `json:"weight"`
	BlankID string  `json:"blank_id"`
	Left    string  `json:"left"`
	RightID string  `json:"right_id"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	W       float64 `json:"w"`
	H       float64 `json:"h"`
	Margin  string  `json:"margin"`
}

func parseAnswerOptions(s string) ([]exporterOption, error) {
	if s == "" || s == "[]" {
		return nil, nil
	}
	var opts []exporterOption
	if err := json.Unmarshal([]byte(s), &opts); err != nil {
		return nil, err
	}
	return opts, nil
}

func xmlEscape(s string) string {
	var b bytes.Buffer
	if err := xml.EscapeText(&b, []byte(s)); err != nil {
		return s
	}
	return b.String()
}

func xmlAttr(s string) string {
	return `"` + xmlEscape(s) + `"`
}
