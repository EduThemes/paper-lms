package service

import (
	"encoding/json"
	"testing"
)

// QTI 1.2 sample (Common Cartridge) for a multiple-choice question with
// cc_profile metadata. Verifies the cc_profile → question_type fallback.
const ccMultipleChoiceQTI = `<?xml version="1.0" encoding="UTF-8"?>
<questestinterop xmlns="http://www.imsglobal.org/xsd/ims_qtiasiv1p2">
  <assessment ident="a1" title="Quiz">
    <section ident="root_section">
      <item ident="q1" title="Q1">
        <itemmetadata><qtimetadata>
          <qtimetadatafield><fieldlabel>cc_profile</fieldlabel><fieldentry>cc.multiple_choice.v0p1</fieldentry></qtimetadatafield>
        </qtimetadata></itemmetadata>
        <presentation>
          <material><mattext texttype="text/plain">2+2=?</mattext></material>
          <response_lid ident="response1" rcardinality="Single">
            <render_choice>
              <response_label ident="A"><material><mattext>3</mattext></material></response_label>
              <response_label ident="B"><material><mattext>4</mattext></material></response_label>
            </render_choice>
          </response_lid>
        </presentation>
        <resprocessing>
          <outcomes><decvar maxvalue="100" minvalue="0" varname="SCORE" vartype="Decimal"/></outcomes>
          <respcondition continue="No">
            <conditionvar><varequal respident="response1">B</varequal></conditionvar>
            <setvar action="Set" varname="SCORE">100</setvar>
          </respcondition>
        </resprocessing>
      </item>
    </section>
  </assessment>
</questestinterop>`

func TestParseQTI_CCProfileMultipleChoice(t *testing.T) {
	r, err := ParseQTIAssessment([]byte(ccMultipleChoiceQTI))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(r.Questions) != 1 {
		t.Fatalf("got %d questions, want 1", len(r.Questions))
	}
	q := r.Questions[0]
	if q.QuestionType != "multiple_choice" {
		t.Errorf("question_type = %q, want multiple_choice", q.QuestionType)
	}
	var answers []answerChoice
	if err := json.Unmarshal([]byte(q.Answers), &answers); err != nil {
		t.Fatalf("answers json: %v", err)
	}
	if len(answers) != 2 {
		t.Fatalf("got %d answers, want 2", len(answers))
	}
	// Verify B is the correct one (weight 100), A is incorrect (weight 0).
	correct := 0
	for _, a := range answers {
		if a.Weight == 100 {
			correct++
			if a.ID != "B" {
				t.Errorf("correct answer = %q, want B", a.ID)
			}
		}
	}
	if correct != 1 {
		t.Errorf("got %d correct answers, want 1", correct)
	}
}

const ccNumericalRangeQTI = `<?xml version="1.0" encoding="UTF-8"?>
<questestinterop>
  <assessment ident="a1" title="Quiz">
    <section ident="root_section">
      <item ident="q1" title="Q1">
        <itemmetadata><qtimetadata>
          <qtimetadatafield><fieldlabel>question_type</fieldlabel><fieldentry>numerical_question</fieldentry></qtimetadatafield>
        </qtimetadata></itemmetadata>
        <presentation>
          <material><mattext>Pi to 1 decimal?</mattext></material>
          <response_str ident="response1" rcardinality="Single"><render_fib/></response_str>
        </presentation>
        <resprocessing>
          <outcomes><decvar maxvalue="100" minvalue="0" varname="SCORE" vartype="Decimal"/></outcomes>
          <respcondition continue="No">
            <conditionvar>
              <vargte respident="response1">3.0</vargte>
              <varlte respident="response1">3.2</varlte>
            </conditionvar>
            <setvar action="Set" varname="SCORE">100</setvar>
          </respcondition>
        </resprocessing>
      </item>
    </section>
  </assessment>
</questestinterop>`

func TestParseQTI_NumericalRange(t *testing.T) {
	r, err := ParseQTIAssessment([]byte(ccNumericalRangeQTI))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(r.Questions) != 1 {
		t.Fatalf("got %d questions, want 1", len(r.Questions))
	}
	q := r.Questions[0]
	if q.QuestionType != "numerical_question" {
		t.Errorf("question_type = %q, want numerical_question", q.QuestionType)
	}
	var answers []answerChoice
	if err := json.Unmarshal([]byte(q.Answers), &answers); err != nil {
		t.Fatalf("answers json: %v", err)
	}
	if len(answers) != 1 {
		t.Fatalf("got %d answers, want 1", len(answers))
	}
	a := answers[0]
	if a.NumericalAnswerType != "range_answer" {
		t.Errorf("numerical_answer_type = %q, want range_answer", a.NumericalAnswerType)
	}
	if a.Start == nil || *a.Start != 3.0 {
		t.Errorf("start = %v, want 3.0", a.Start)
	}
	if a.End == nil || *a.End != 3.2 {
		t.Errorf("end = %v, want 3.2", a.End)
	}
}

const ccNumericalExactQTI = `<?xml version="1.0" encoding="UTF-8"?>
<questestinterop>
  <assessment ident="a1" title="Quiz">
    <section ident="root_section">
      <item ident="q1" title="Q1">
        <itemmetadata><qtimetadata>
          <qtimetadatafield><fieldlabel>question_type</fieldlabel><fieldentry>numerical_question</fieldentry></qtimetadatafield>
        </qtimetadata></itemmetadata>
        <presentation>
          <material><mattext>2+2?</mattext></material>
          <response_str ident="response1" rcardinality="Single"><render_fib/></response_str>
        </presentation>
        <resprocessing>
          <outcomes><decvar maxvalue="100" minvalue="0" varname="SCORE" vartype="Decimal"/></outcomes>
          <respcondition continue="No">
            <conditionvar>
              <varequal respident="response1">4</varequal>
            </conditionvar>
            <setvar action="Set" varname="SCORE">100</setvar>
          </respcondition>
        </resprocessing>
      </item>
    </section>
  </assessment>
</questestinterop>`

func TestParseQTI_NumericalExact(t *testing.T) {
	r, err := ParseQTIAssessment([]byte(ccNumericalExactQTI))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	q := r.Questions[0]
	if q.QuestionType != "numerical_question" {
		t.Fatalf("question_type = %q, want numerical_question", q.QuestionType)
	}
	var answers []answerChoice
	_ = json.Unmarshal([]byte(q.Answers), &answers)
	if len(answers) != 1 || answers[0].NumericalAnswerType != "exact_answer" {
		t.Fatalf("answers = %+v, want one exact_answer", answers)
	}
	if answers[0].Exact == nil || *answers[0].Exact != 4 {
		t.Errorf("exact = %v, want 4", answers[0].Exact)
	}
}

// Matching: two prompts, three match candidates (one distractor).
const ccMatchingQTI = `<?xml version="1.0" encoding="UTF-8"?>
<questestinterop>
  <assessment ident="a1" title="Quiz">
    <section ident="root_section">
      <item ident="q1" title="Q1">
        <itemmetadata><qtimetadata>
          <qtimetadatafield><fieldlabel>question_type</fieldlabel><fieldentry>matching_question</fieldentry></qtimetadatafield>
        </qtimetadata></itemmetadata>
        <presentation>
          <material><mattext>Match the capitals.</mattext></material>
          <response_lid ident="response_p1" rcardinality="Single">
            <material><mattext>France</mattext></material>
            <render_choice>
              <response_label ident="m_paris"><material><mattext>Paris</mattext></material></response_label>
              <response_label ident="m_madrid"><material><mattext>Madrid</mattext></material></response_label>
              <response_label ident="m_oslo"><material><mattext>Oslo</mattext></material></response_label>
            </render_choice>
          </response_lid>
          <response_lid ident="response_p2" rcardinality="Single">
            <material><mattext>Spain</mattext></material>
            <render_choice>
              <response_label ident="m_paris"><material><mattext>Paris</mattext></material></response_label>
              <response_label ident="m_madrid"><material><mattext>Madrid</mattext></material></response_label>
              <response_label ident="m_oslo"><material><mattext>Oslo</mattext></material></response_label>
            </render_choice>
          </response_lid>
        </presentation>
        <resprocessing>
          <outcomes><decvar maxvalue="100" minvalue="0" varname="SCORE" vartype="Decimal"/></outcomes>
          <respcondition>
            <conditionvar><varequal respident="response_p1">m_paris</varequal></conditionvar>
            <setvar action="Add" varname="SCORE">50</setvar>
          </respcondition>
          <respcondition>
            <conditionvar><varequal respident="response_p2">m_madrid</varequal></conditionvar>
            <setvar action="Add" varname="SCORE">50</setvar>
          </respcondition>
        </resprocessing>
      </item>
    </section>
  </assessment>
</questestinterop>`

func TestParseQTI_MatchingPairs(t *testing.T) {
	r, err := ParseQTIAssessment([]byte(ccMatchingQTI))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	q := r.Questions[0]
	if q.QuestionType != "matching" {
		t.Fatalf("question_type = %q, want matching", q.QuestionType)
	}

	var answers []answerChoice
	if err := json.Unmarshal([]byte(q.Answers), &answers); err != nil {
		t.Fatalf("answers json: %v", err)
	}

	// Expected: 2 prompt entries + 3 match-pool entries = 5 total.
	if len(answers) != 5 {
		t.Fatalf("got %d answers, want 5 (2 prompts + 3 match options)", len(answers))
	}

	prompts := []answerChoice{}
	matchPool := []answerChoice{}
	for _, a := range answers {
		if a.BlankID == "_match_options_" {
			matchPool = append(matchPool, a)
		} else {
			prompts = append(prompts, a)
		}
	}
	if len(prompts) != 2 {
		t.Fatalf("got %d prompts, want 2", len(prompts))
	}
	if len(matchPool) != 3 {
		t.Fatalf("got %d match options, want 3", len(matchPool))
	}

	got := map[string]string{}
	for _, p := range prompts {
		got[p.Text] = p.MatchID
	}
	if got["France"] != "m_paris" {
		t.Errorf("France → %q, want m_paris", got["France"])
	}
	if got["Spain"] != "m_madrid" {
		t.Errorf("Spain → %q, want m_madrid", got["Spain"])
	}
}

const ccFillInMultipleBlanksQTI = `<?xml version="1.0" encoding="UTF-8"?>
<questestinterop>
  <assessment ident="a1" title="Quiz">
    <section ident="root_section">
      <item ident="q1" title="Q1">
        <itemmetadata><qtimetadata>
          <qtimetadatafield><fieldlabel>question_type</fieldlabel><fieldentry>fill_in_multiple_blanks_question</fieldentry></qtimetadatafield>
        </qtimetadata></itemmetadata>
        <presentation>
          <material><mattext>The [color1] sky over [city] at sunset.</mattext></material>
          <response_str ident="response_color1" rcardinality="Single">
            <material><mattext>color1</mattext></material>
            <render_fib/>
          </response_str>
          <response_str ident="response_city" rcardinality="Single">
            <material><mattext>city</mattext></material>
            <render_fib/>
          </response_str>
        </presentation>
        <resprocessing>
          <outcomes><decvar maxvalue="100" minvalue="0" varname="SCORE" vartype="Decimal"/></outcomes>
          <respcondition>
            <conditionvar><varequal respident="response_color1">orange</varequal></conditionvar>
            <setvar action="Add" varname="SCORE">50</setvar>
          </respcondition>
          <respcondition>
            <conditionvar><varequal respident="response_color1">red</varequal></conditionvar>
            <setvar action="Add" varname="SCORE">50</setvar>
          </respcondition>
          <respcondition>
            <conditionvar><varequal respident="response_city">Phoenix</varequal></conditionvar>
            <setvar action="Add" varname="SCORE">50</setvar>
          </respcondition>
        </resprocessing>
      </item>
    </section>
  </assessment>
</questestinterop>`

func TestParseQTI_FillInMultipleBlanks(t *testing.T) {
	r, err := ParseQTIAssessment([]byte(ccFillInMultipleBlanksQTI))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	q := r.Questions[0]
	if q.QuestionType != "fill_in_multiple_blanks" {
		t.Fatalf("question_type = %q, want fill_in_multiple_blanks", q.QuestionType)
	}
	var answers []answerChoice
	_ = json.Unmarshal([]byte(q.Answers), &answers)
	byBlank := map[string][]string{}
	for _, a := range answers {
		byBlank[a.BlankID] = append(byBlank[a.BlankID], a.Text)
	}
	if len(byBlank["color1"]) != 2 {
		t.Errorf("color1 answers = %v, want 2 (orange,red)", byBlank["color1"])
	}
	if len(byBlank["city"]) != 1 || byBlank["city"][0] != "Phoenix" {
		t.Errorf("city answers = %v, want [Phoenix]", byBlank["city"])
	}
}
