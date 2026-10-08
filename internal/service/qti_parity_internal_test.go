package service

// B035: questions imported from a Canvas .imscc (QTI 1.2) must come out in
// a type and answer shape Paper can grade. Each case parses real-shaped
// Canvas XML (lib/cc/qti/qti_items.rb) and grades a submission end to end.

import (
	"testing"
)

func canvasItem(qType, body string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<questestinterop>
  <assessment ident="a1" title="Quiz">
    <section ident="root_section">
      <item ident="q1" title="Q1">
        <itemmetadata><qtimetadata>
          <qtimetadatafield><fieldlabel>question_type</fieldlabel><fieldentry>` + qType + `</fieldentry></qtimetadatafield>
          <qtimetadatafield><fieldlabel>points_possible</fieldlabel><fieldentry>2</fieldentry></qtimetadatafield>
        </qtimetadata></itemmetadata>
` + body + `
      </item>
    </section>
  </assessment>
</questestinterop>`
}

func gradeImported(t *testing.T, xml, submitted string) (string, float64, bool) {
	t.Helper()
	r, err := ParseQTIAssessment([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(r.Questions) != 1 {
		t.Fatalf("want 1 question, got %d", len(r.Questions))
	}
	q := r.Questions[0]
	pts, _, gradable := (&QuizService{}).autoGrade(&q, submitted)
	return q.QuestionType, pts, gradable
}

func TestCanvasImport_MultipleAnswersGradable(t *testing.T) {
	xml := canvasItem("multiple_answers_question", `
        <presentation>
          <material><mattext>Pick primes</mattext></material>
          <response_lid ident="response1" rcardinality="Multiple">
            <render_choice>
              <response_label ident="11"><material><mattext>2</mattext></material></response_label>
              <response_label ident="12"><material><mattext>3</mattext></material></response_label>
              <response_label ident="13"><material><mattext>4</mattext></material></response_label>
            </render_choice>
          </response_lid>
        </presentation>
        <resprocessing>
          <outcomes><decvar maxvalue="100" minvalue="0" varname="SCORE" vartype="Decimal"/></outcomes>
          <respcondition continue="No">
            <conditionvar><and>
              <varequal respident="response1">11</varequal>
              <varequal respident="response1">12</varequal>
              <not><varequal respident="response1">13</varequal></not>
            </and></conditionvar>
            <setvar action="Set" varname="SCORE">100</setvar>
          </respcondition>
        </resprocessing>`)
	qType, pts, gradable := gradeImported(t, xml, `["11","12"]`)
	if qType != "multiple_answer" {
		t.Fatalf("type = %q, want multiple_answer", qType)
	}
	if !gradable || pts != 2 {
		t.Fatalf("graded (%v, %v), want full credit", pts, gradable)
	}
}

func TestCanvasImport_MultipleDropdownsGradable(t *testing.T) {
	xml := canvasItem("multiple_dropdowns_question", `
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
          <outcomes><decvar maxvalue="100" minvalue="0" varname="SCORE" vartype="Decimal"/></outcomes>
          <respcondition>
            <conditionvar><varequal respident="response_color">21</varequal></conditionvar>
            <setvar varname="SCORE" action="Add">100.00</setvar>
          </respcondition>
        </resprocessing>`)
	qType, pts, gradable := gradeImported(t, xml, `{"color":"21"}`)
	if qType != "multiple_dropdown" {
		t.Fatalf("type = %q, want multiple_dropdown", qType)
	}
	if !gradable || pts != 2 {
		t.Fatalf("graded (%v, %v), want full credit", pts, gradable)
	}
}

func TestCanvasImport_NumericalMarginGradable(t *testing.T) {
	xml := canvasItem("numerical_question", `
        <presentation>
          <material><mattext>Answer?</mattext></material>
          <response_str ident="response1" rcardinality="Single"><render_fib fibtype="Decimal"><response_label ident="answer1"/></render_fib></response_str>
        </presentation>
        <resprocessing>
          <outcomes><decvar maxvalue="100" minvalue="0" varname="SCORE" vartype="Decimal"/></outcomes>
          <respcondition continue="No">
            <conditionvar><or>
              <varequal respident="response1">42.0</varequal>
              <and><vargte respident="response1">41.5</vargte><varlte respident="response1">42.5</varlte></and>
            </or></conditionvar>
            <setvar action="Set" varname="SCORE">100</setvar>
          </respcondition>
        </resprocessing>`)
	for _, tc := range []struct {
		in   string
		want float64
	}{{"42", 2}, {"42.4", 2}, {"42.6", 0}} {
		qType, pts, gradable := gradeImported(t, xml, tc.in)
		if qType != "numerical_question" || !gradable || pts != tc.want {
			t.Errorf("%s: graded (%q, %v, %v), want %v", tc.in, qType, pts, gradable, tc.want)
		}
	}
}

func TestCanvasImport_MatchingGradable(t *testing.T) {
	xml := canvasItem("matching_question", `
        <presentation>
          <material><mattext>Match capitals</mattext></material>
          <response_lid ident="response_31">
            <material><mattext>France</mattext></material>
            <render_choice>
              <response_label ident="901"><material><mattext>Paris</mattext></material></response_label>
              <response_label ident="902"><material><mattext>London</mattext></material></response_label>
            </render_choice>
          </response_lid>
          <response_lid ident="response_32">
            <material><mattext>UK</mattext></material>
            <render_choice>
              <response_label ident="901"><material><mattext>Paris</mattext></material></response_label>
              <response_label ident="902"><material><mattext>London</mattext></material></response_label>
            </render_choice>
          </response_lid>
        </presentation>
        <resprocessing>
          <outcomes><decvar maxvalue="100" minvalue="0" varname="SCORE" vartype="Decimal"/></outcomes>
          <respcondition><conditionvar><varequal respident="response_31">901</varequal></conditionvar><setvar varname="SCORE" action="Add">50.00</setvar></respcondition>
          <respcondition><conditionvar><varequal respident="response_32">902</varequal></conditionvar><setvar varname="SCORE" action="Add">50.00</setvar></respcondition>
        </resprocessing>`)
	qType, pts, gradable := gradeImported(t, xml, `[{"left":"31","right_id":"901"},{"left":"32","right_id":"902"}]`)
	if qType != "matching" || !gradable || pts != 2 {
		t.Fatalf("graded (%q, %v, %v), want matching full credit", qType, pts, gradable)
	}
}

func TestCanvasImport_FillInMultipleBlanksResponseLidGradable(t *testing.T) {
	// Canvas exports FIMB blanks as response_lid with every accepted answer
	// listed in render_choice; resprocessing names one by label ident.
	xml := canvasItem("fill_in_multiple_blanks_question", `
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
          <outcomes><decvar maxvalue="100" minvalue="0" varname="SCORE" vartype="Decimal"/></outcomes>
          <respcondition><conditionvar><varequal respident="response_color">41</varequal></conditionvar><setvar varname="SCORE" action="Add">100.00</setvar></respcondition>
        </resprocessing>`)
	for _, tc := range []struct {
		in   string
		want float64
	}{{`{"color":"Red"}`, 2}, {`{"color":"orange"}`, 2}, {`{"color":"41"}`, 0}} {
		qType, pts, gradable := gradeImported(t, xml, tc.in)
		if qType != "fill_in_multiple_blanks" || !gradable || pts != tc.want {
			t.Errorf("%s: graded (%q, %v, %v), want %v", tc.in, qType, pts, gradable, tc.want)
		}
	}
}
