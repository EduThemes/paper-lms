package qti

// Canvas-parity regressions for the Classic importer:
//   B124 — numeric exact±margin and range answers survive import.
//   B125 — multiple-dropdown blank ids match the "[blank]" tokens in the
//          question text (Canvas idents are "response_<blank>").
//   B035 — matching keeps left labels + the right-hand pool; FIMB blanks
//          exported as response_lid store accepted TEXT, not label ids.

import (
	"encoding/json"
	"strings"
	"testing"
)

func parseOneClassic(t *testing.T, itemXML string) QuestionImport {
	t.Helper()
	full := `<?xml version="1.0"?><questestinterop>
		<assessment ident="A1" title="T"><qtimetadata/>
			<section ident="root">` + itemXML + `</section>
		</assessment>
	</questestinterop>`
	quizzes, _, errs := parseClassicAssessmentFile("test.xml", []byte(full))
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	if len(quizzes) != 1 || len(quizzes[0].Questions) != 1 {
		t.Fatalf("want 1 question")
	}
	return quizzes[0].Questions[0]
}

func TestClassicNumericalMarginAndRange(t *testing.T) {
	q := parseOneClassic(t, classicItemFromTmpl("n1", "numerical_question", "1", `
        <presentation><material><mattext>X</mattext></material>
          <response_str ident="response1" rcardinality="Single"><render_fib fibtype="Decimal"/></response_str>
        </presentation>
        <resprocessing>
          <outcomes><decvar maxvalue="100" varname="SCORE"/></outcomes>
          <respcondition continue="No">
            <conditionvar><or>
              <varequal respident="response1">42.0</varequal>
              <and><vargte respident="response1">41.5</vargte><varlte respident="response1">42.5</varlte></and>
            </or></conditionvar>
            <setvar varname="SCORE" action="Set">100</setvar>
          </respcondition>
          <respcondition continue="No">
            <conditionvar><vargte respident="response1">100</vargte><varlte respident="response1">200</varlte></conditionvar>
            <setvar varname="SCORE" action="Set">100</setvar>
          </respcondition>
        </resprocessing>`))

	var answers []map[string]interface{}
	if err := json.Unmarshal([]byte(q.Answers), &answers); err != nil {
		t.Fatalf("answers: %v (%s)", err, q.Answers)
	}
	if len(answers) != 2 {
		t.Fatalf("want 2 numeric answers (exact±margin, range), got %s", q.Answers)
	}
	if answers[0]["text"] != "42.0" || answers[0]["margin"] != "0.5" {
		t.Errorf("exact answer lost its margin: %v", answers[0])
	}
	if answers[1]["numerical_answer_type"] != "range_answer" || answers[1]["start"] != "100" || answers[1]["end"] != "200" {
		t.Errorf("range answer not imported: %v", answers[1])
	}
}

func TestClassicDropdownBlankIDStripsResponsePrefix(t *testing.T) {
	q := parseOneClassic(t, classicItemFromTmpl("md", "multiple_dropdowns_question", "1", `
        <presentation>
          <material><mattext>Roses are [color]</mattext></material>
          <response_lid ident="response_color">
            <material><mattext>color</mattext></material>
            <render_choice>
              <response_label ident="21"><material><mattext>red</mattext></material></response_label>
              <response_label ident="22"><material><mattext>blue</mattext></material></response_label>
            </render_choice>
          </response_lid>
        </presentation>
        <resprocessing>
          <outcomes><decvar maxvalue="100" varname="SCORE"/></outcomes>
          <respcondition><conditionvar><varequal respident="response_color">21</varequal></conditionvar>
            <setvar varname="SCORE" action="Add">100.00</setvar></respcondition>
        </resprocessing>`))
	if !strings.Contains(q.Answers, `"blank_id":"color"`) || strings.Contains(q.Answers, "response_color") {
		t.Errorf("blank id must be the bare token name: %s", q.Answers)
	}
	if !strings.Contains(q.Answers, `"id":"21","text":"red","weight":100`) && !strings.Contains(q.Answers, `"weight":100`) {
		t.Errorf("correct option not weighted: %s", q.Answers)
	}
}

func TestClassicMatchingKeepsLabelsAndPool(t *testing.T) {
	q := parseOneClassic(t, classicItemFromTmpl("m", "matching_question", "2", `
        <presentation>
          <material><mattext>Match</mattext></material>
          <response_lid ident="response_31">
            <material><mattext>France</mattext></material>
            <render_choice>
              <response_label ident="901"><material><mattext>Paris</mattext></material></response_label>
              <response_label ident="902"><material><mattext>London</mattext></material></response_label>
            </render_choice>
          </response_lid>
        </presentation>
        <resprocessing>
          <outcomes><decvar maxvalue="100" varname="SCORE"/></outcomes>
          <respcondition><conditionvar><varequal respident="response_31">901</varequal></conditionvar>
            <setvar varname="SCORE" action="Add">100.00</setvar></respcondition>
        </resprocessing>`))
	for _, want := range []string{`"left":"France"`, `"right_id":"901"`, `"text":"Paris"`, `"blank_id":"_match_options_"`} {
		if !strings.Contains(q.Answers, want) {
			t.Errorf("missing %s in %s", want, q.Answers)
		}
	}
}

func TestClassicFIMBResponseLidStoresTexts(t *testing.T) {
	q := parseOneClassic(t, classicItemFromTmpl("f", "fill_in_multiple_blanks_question", "1", `
        <presentation>
          <material><mattext>The [color] sky</mattext></material>
          <response_lid ident="response_color">
            <material><mattext>color</mattext></material>
            <render_choice>
              <response_label ident="41"><material><mattext>orange</mattext></material></response_label>
              <response_label ident="42"><material><mattext>red</mattext></material></response_label>
            </render_choice>
          </response_lid>
        </presentation>
        <resprocessing>
          <outcomes><decvar maxvalue="100" varname="SCORE"/></outcomes>
          <respcondition><conditionvar><varequal respident="response_color">41</varequal></conditionvar>
            <setvar varname="SCORE" action="Add">100.00</setvar></respcondition>
        </resprocessing>`))
	var m map[string][]string
	if err := json.Unmarshal([]byte(q.Answers), &m); err != nil {
		t.Fatalf("answers: %v", err)
	}
	got := strings.Join(m["color"], ",")
	if got != "orange,red" {
		t.Errorf("color accepted = %q, want orange,red (texts, not label ids)", got)
	}
}
