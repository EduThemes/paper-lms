package qti

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestClassicItemTypes is a table-driven sweep covering every Canvas
// Classic item type. Each row provides a minimal `<item>` XML snippet,
// the expected unified question_type, and assertions about the
// resulting Answers JSON / points / comments.
//
// The snippets are deliberately small — full assessment context is
// covered by the imscc end-to-end test.
func TestClassicItemTypes(t *testing.T) {
	cases := []struct {
		name         string
		xml          string
		wantType     string
		wantPoints   float64
		assertAnswers func(t *testing.T, answers string)
	}{
		{
			name:       "multiple_choice",
			wantType:   UnifiedMultipleChoice,
			wantPoints: 2,
			xml: classicItemFromTmpl("mc1", "multiple_choice_question", "2", `
        <presentation>
          <material><mattext>Q?</mattext></material>
          <response_lid ident="response1" rcardinality="Single">
            <render_choice>
              <response_label ident="a"><material><mattext>A</mattext></material></response_label>
              <response_label ident="b"><material><mattext>B</mattext></material></response_label>
            </render_choice>
          </response_lid>
        </presentation>
        <resprocessing>
          <outcomes><decvar maxvalue="100" varname="SCORE"/></outcomes>
          <respcondition continue="No">
            <conditionvar><varequal respident="response1">b</varequal></conditionvar>
            <setvar varname="SCORE" action="Set">100</setvar>
          </respcondition>
        </resprocessing>`),
			assertAnswers: func(t *testing.T, a string) {
				var opts []map[string]interface{}
				if err := json.Unmarshal([]byte(a), &opts); err != nil {
					t.Fatalf("unmarshal answers: %v", err)
				}
				if len(opts) != 2 {
					t.Fatalf("want 2 opts, got %d", len(opts))
				}
				// b is correct → weight 100, a → 0.
				for _, o := range opts {
					id := o["id"].(string)
					weight, _ := o["weight"].(float64)
					if id == "b" && weight != 100 {
						t.Errorf("expected b weight=100, got %v", weight)
					}
					if id == "a" && weight != 0 {
						t.Errorf("expected a weight=0, got %v", weight)
					}
				}
			},
		},
		{
			name:       "true_false",
			wantType:   UnifiedTrueFalse,
			wantPoints: 1,
			xml: classicItemFromTmpl("tf1", "true_false_question", "1", `
        <presentation>
          <material><mattext>X</mattext></material>
          <response_lid ident="response1" rcardinality="Single">
            <render_choice>
              <response_label ident="t"><material><mattext>True</mattext></material></response_label>
              <response_label ident="f"><material><mattext>False</mattext></material></response_label>
            </render_choice>
          </response_lid>
        </presentation>
        <resprocessing>
          <outcomes><decvar maxvalue="100" varname="SCORE"/></outcomes>
          <respcondition continue="No">
            <conditionvar><varequal respident="response1">t</varequal></conditionvar>
            <setvar varname="SCORE" action="Set">100</setvar>
          </respcondition>
        </resprocessing>`),
			assertAnswers: func(t *testing.T, a string) {
				if !strings.Contains(a, `"id":"t"`) || !strings.Contains(a, `"weight":100`) {
					t.Errorf("expected t weighted 100 in %s", a)
				}
			},
		},
		{
			name:       "multiple_answer",
			wantType:   UnifiedMultipleAnswer,
			wantPoints: 3,
			xml: classicItemFromTmpl("ma1", "multiple_answers_question", "3", `
        <presentation>
          <material><mattext>Q</mattext></material>
          <response_lid ident="response1" rcardinality="Multiple">
            <render_choice>
              <response_label ident="x"><material><mattext>x</mattext></material></response_label>
              <response_label ident="y"><material><mattext>y</mattext></material></response_label>
              <response_label ident="z"><material><mattext>z</mattext></material></response_label>
            </render_choice>
          </response_lid>
        </presentation>
        <resprocessing>
          <outcomes><decvar maxvalue="100" varname="SCORE"/></outcomes>
          <respcondition continue="No">
            <conditionvar><and>
              <varequal respident="response1">x</varequal>
              <varequal respident="response1">y</varequal>
            </and></conditionvar>
            <setvar varname="SCORE" action="Set">100</setvar>
          </respcondition>
        </resprocessing>`),
			assertAnswers: func(t *testing.T, a string) {
				var opts []map[string]interface{}
				_ = json.Unmarshal([]byte(a), &opts)
				correctCount := 0
				for _, o := range opts {
					if w, _ := o["weight"].(float64); w > 0 {
						correctCount++
					}
				}
				if correctCount != 2 {
					t.Errorf("expected 2 correct opts (x,y), got %d", correctCount)
				}
			},
		},
		{
			name:       "short_answer",
			wantType:   UnifiedShortAnswer,
			wantPoints: 1,
			xml: classicItemFromTmpl("sa1", "short_answer_question", "1", `
        <presentation>
          <material><mattext>Q</mattext></material>
          <response_str ident="response1"><render_fib rows="1"/></response_str>
        </presentation>
        <resprocessing>
          <outcomes><decvar maxvalue="100" varname="SCORE"/></outcomes>
          <respcondition continue="No">
            <conditionvar><varequal respident="response1">Paris</varequal></conditionvar>
            <setvar varname="SCORE" action="Set">100</setvar>
          </respcondition>
        </resprocessing>`),
			assertAnswers: func(t *testing.T, a string) {
				if !strings.Contains(a, `"text":"Paris"`) {
					t.Errorf("expected accepted text Paris in %s", a)
				}
			},
		},
		{
			name:       "fill_in_multiple_blanks",
			wantType:   UnifiedFillInMultipleBlanks,
			wantPoints: 2,
			xml: classicItemFromTmpl("fimb1", "fill_in_multiple_blanks_question", "2", `
        <presentation>
          <material><mattext>The [c1] [c2] fox</mattext></material>
          <response_str ident="c1"><render_fib rows="1"/></response_str>
          <response_str ident="c2"><render_fib rows="1"/></response_str>
        </presentation>
        <resprocessing>
          <outcomes><decvar maxvalue="100" varname="SCORE"/></outcomes>
          <respcondition continue="No">
            <conditionvar><varequal respident="c1">quick</varequal></conditionvar>
            <setvar varname="SCORE" action="Set">100</setvar>
          </respcondition>
          <respcondition continue="No">
            <conditionvar><varequal respident="c2">brown</varequal></conditionvar>
            <setvar varname="SCORE" action="Set">100</setvar>
          </respcondition>
        </resprocessing>`),
			assertAnswers: func(t *testing.T, a string) {
				var m map[string][]string
				if err := json.Unmarshal([]byte(a), &m); err != nil {
					t.Fatalf("unmarshal blanks: %v", err)
				}
				if len(m["c1"]) == 0 || m["c1"][0] != "quick" {
					t.Errorf("expected c1=[quick], got %v", m["c1"])
				}
				if len(m["c2"]) == 0 || m["c2"][0] != "brown" {
					t.Errorf("expected c2=[brown], got %v", m["c2"])
				}
			},
		},
		{
			name:       "multiple_dropdown",
			wantType:   UnifiedMultipleDropdown,
			wantPoints: 2,
			xml: classicItemFromTmpl("md1", "multiple_dropdowns_question", "2", `
        <presentation>
          <material><mattext>X</mattext></material>
          <response_lid ident="b1" rcardinality="Single">
            <render_choice>
              <response_label ident="b1_yes"><material><mattext>yes</mattext></material></response_label>
              <response_label ident="b1_no"><material><mattext>no</mattext></material></response_label>
            </render_choice>
          </response_lid>
        </presentation>
        <resprocessing>
          <outcomes><decvar maxvalue="100" varname="SCORE"/></outcomes>
          <respcondition continue="No">
            <conditionvar><varequal respident="b1">b1_yes</varequal></conditionvar>
            <setvar varname="SCORE" action="Set">100</setvar>
          </respcondition>
        </resprocessing>`),
			assertAnswers: func(t *testing.T, a string) {
				if !strings.Contains(a, `"blank_id":"b1"`) {
					t.Errorf("expected blank_id b1 in %s", a)
				}
			},
		},
		{
			name:       "matching",
			wantType:   UnifiedMatching,
			wantPoints: 2,
			xml: classicItemFromTmpl("m1", "matching_question", "2", `
        <presentation>
          <material><mattext>X</mattext></material>
          <response_lid ident="left1" rcardinality="Single">
            <render_choice>
              <response_label ident="r1"><material><mattext>R1</mattext></material></response_label>
              <response_label ident="r2"><material><mattext>R2</mattext></material></response_label>
            </render_choice>
          </response_lid>
        </presentation>
        <resprocessing>
          <outcomes><decvar maxvalue="100" varname="SCORE"/></outcomes>
          <respcondition continue="No">
            <conditionvar><varequal respident="left1">r1</varequal></conditionvar>
            <setvar varname="SCORE" action="Set">50</setvar>
          </respcondition>
        </resprocessing>`),
			assertAnswers: func(t *testing.T, a string) {
				if !strings.Contains(a, `"right_id":"r1"`) {
					t.Errorf("expected right_id r1 in %s", a)
				}
			},
		},
		{
			name:       "numerical",
			wantType:   UnifiedNumerical,
			wantPoints: 1,
			xml: classicItemFromTmplWithExtraMeta("n1", "numerical_question", "1",
				`<qtimetadatafield><fieldlabel>answer_exact</fieldlabel><fieldentry>42</fieldentry></qtimetadatafield>
				<qtimetadatafield><fieldlabel>answer_error_margin</fieldlabel><fieldentry>0.5</fieldentry></qtimetadatafield>`, `
        <presentation>
          <material><mattext>X</mattext></material>
          <response_str ident="response1"><render_fib rows="1"/></response_str>
        </presentation>
        <resprocessing>
          <outcomes><decvar maxvalue="100" varname="SCORE"/></outcomes>
        </resprocessing>`),
			assertAnswers: func(t *testing.T, a string) {
				if !strings.Contains(a, `"text":"42"`) {
					t.Errorf("expected text=42 in %s", a)
				}
				if !strings.Contains(a, `"margin":"0.5"`) {
					t.Errorf("expected margin=0.5 in %s", a)
				}
			},
		},
		{
			name:       "formula",
			wantType:   UnifiedFormula,
			wantPoints: 2,
			xml: classicItemFromTmplWithExtraMeta("f1", "calculated_question", "2",
				`<qtimetadatafield><fieldlabel>answer_exact</fieldlabel><fieldentry>100</fieldentry></qtimetadatafield>
				<qtimetadatafield><fieldlabel>answer_error_margin</fieldlabel><fieldentry>5%</fieldentry></qtimetadatafield>`, `
        <presentation>
          <material><mattext>X</mattext></material>
          <response_str ident="response1"><render_fib rows="1"/></response_str>
        </presentation>
        <resprocessing>
          <outcomes><decvar maxvalue="100" varname="SCORE"/></outcomes>
        </resprocessing>`),
			assertAnswers: func(t *testing.T, a string) {
				if !strings.Contains(a, `"margin":"5%"`) {
					t.Errorf("expected margin=5%% in %s", a)
				}
			},
		},
		{
			name:       "essay",
			wantType:   UnifiedEssay,
			wantPoints: 5,
			xml: classicItemFromTmpl("e1", "essay_question", "5", `
        <presentation>
          <material><mattext>Explain</mattext></material>
          <response_str ident="response1"><render_fib rows="5"/></response_str>
        </presentation>
        <resprocessing><outcomes><decvar maxvalue="100" varname="SCORE"/></outcomes></resprocessing>`),
			assertAnswers: func(t *testing.T, a string) {
				if a != "[]" {
					t.Errorf("expected empty answers, got %s", a)
				}
			},
		},
		{
			name:       "file_upload",
			wantType:   UnifiedFileUpload,
			wantPoints: 3,
			xml: classicItemFromTmpl("up1", "file_upload_question", "3", `
        <presentation><material><mattext>Upload</mattext></material></presentation>
        <resprocessing><outcomes><decvar maxvalue="100" varname="SCORE"/></outcomes></resprocessing>`),
			assertAnswers: func(t *testing.T, a string) {
				if a != "[]" {
					t.Errorf("expected empty, got %s", a)
				}
			},
		},
		{
			name:       "text_only",
			wantType:   UnifiedTextOnly,
			wantPoints: 1,
			xml: classicItemFromTmpl("t1", "text_only_question", "1", `
        <presentation><material><mattext>Read carefully</mattext></material></presentation>
        <resprocessing><outcomes><decvar maxvalue="100" varname="SCORE"/></outcomes></resprocessing>`),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Wrap in a minimal assessment so parseClassicAssessmentFile accepts it.
			full := `<?xml version="1.0"?><questestinterop>
				<assessment ident="A1" title="T"><qtimetadata/>
					<section ident="root">` + tc.xml + `</section>
				</assessment>
			</questestinterop>`
			quizzes, warnings, errs := parseClassicAssessmentFile("test.xml", []byte(full))
			if len(errs) > 0 {
				t.Fatalf("unexpected errors: %v", errs)
			}
			if len(quizzes) != 1 || len(quizzes[0].Questions) != 1 {
				t.Fatalf("expected 1 quiz / 1 question, got %d quizzes (warnings=%v)", len(quizzes), warnings)
			}
			q := quizzes[0].Questions[0]
			if q.QuestionType != tc.wantType {
				t.Errorf("want type %s, got %s", tc.wantType, q.QuestionType)
			}
			if q.PointsPossible == nil || *q.PointsPossible != tc.wantPoints {
				t.Errorf("want points %v, got %v", tc.wantPoints, q.PointsPossible)
			}
			if tc.assertAnswers != nil {
				tc.assertAnswers(t, q.Answers)
			}
		})
	}
}

// classicItemFromTmpl wraps an inner item body with item-level metadata.
func classicItemFromTmpl(ident, qtype, points, body string) string {
	return `<item ident="` + ident + `" title="T">
		<itemmetadata><qtimetadata>
			<qtimetadatafield><fieldlabel>question_type</fieldlabel><fieldentry>` + qtype + `</fieldentry></qtimetadatafield>
			<qtimetadatafield><fieldlabel>points_possible</fieldlabel><fieldentry>` + points + `</fieldentry></qtimetadatafield>
		</qtimetadata></itemmetadata>
		` + body + `
	</item>`
}

func classicItemFromTmplWithExtraMeta(ident, qtype, points, extraMeta, body string) string {
	return `<item ident="` + ident + `" title="T">
		<itemmetadata><qtimetadata>
			<qtimetadatafield><fieldlabel>question_type</fieldlabel><fieldentry>` + qtype + `</fieldentry></qtimetadatafield>
			<qtimetadatafield><fieldlabel>points_possible</fieldlabel><fieldentry>` + points + `</fieldentry></qtimetadatafield>
			` + extraMeta + `
		</qtimetadata></itemmetadata>
		` + body + `
	</item>`
}

// TestClassicUnknownTypeIsError ensures an unknown question_type
// produces a per-item ImportError (not a warning) so the importer can
// surface it loudly.
func TestClassicUnknownTypeIsError(t *testing.T) {
	xml := classicItemFromTmpl("x1", "unicorn_question", "1", `
		<presentation><material><mattext>?</mattext></material></presentation>
		<resprocessing><outcomes><decvar maxvalue="100" varname="SCORE"/></outcomes></resprocessing>`)
	full := `<?xml version="1.0"?><questestinterop>
		<assessment ident="A1" title="T"><qtimetadata/>
			<section ident="root">` + xml + `</section>
		</assessment>
	</questestinterop>`
	_, _, errs := parseClassicAssessmentFile("test.xml", []byte(full))
	if len(errs) == 0 {
		t.Fatal("expected an error for unknown item type")
	}
	if errs[0].Code != "unknown_item_type" {
		t.Errorf("want code unknown_item_type, got %s", errs[0].Code)
	}
}

// TestClassicBankParse covers the assessment_question_banks <objectbank>
// file path.
func TestClassicBankParse(t *testing.T) {
	xmlDoc := `<?xml version="1.0"?>
<questestinterop>
  <objectbank ident="bk-1">
    <qtimetadata>
      <qtimetadatafield><fieldlabel>bank_title</fieldlabel><fieldentry>Test Bank</fieldentry></qtimetadatafield>
    </qtimetadata>
    ` + classicItemFromTmpl("bi1", "multiple_choice_question", "1", `
        <presentation>
          <material><mattext>Q</mattext></material>
          <response_lid ident="response1" rcardinality="Single">
            <render_choice>
              <response_label ident="a"><material><mattext>A</mattext></material></response_label>
            </render_choice>
          </response_lid>
        </presentation>
        <resprocessing>
          <outcomes><decvar maxvalue="100" varname="SCORE"/></outcomes>
          <respcondition continue="No">
            <conditionvar><varequal respident="response1">a</varequal></conditionvar>
            <setvar varname="SCORE" action="Set">100</setvar>
          </respcondition>
        </resprocessing>`) + `
  </objectbank>
</questestinterop>`

	bank, warnings, err := parseClassicBank("bank.xml", []byte(xmlDoc))
	if err != nil {
		t.Fatalf("parseClassicBank: %v", err)
	}
	_ = warnings
	if bank == nil {
		t.Fatal("nil bank")
	}
	if bank.Title != "Test Bank" {
		t.Errorf("want title Test Bank, got %q", bank.Title)
	}
	if len(bank.Items) != 1 {
		t.Fatalf("want 1 item, got %d", len(bank.Items))
	}
	if bank.Items[0].QuestionType != UnifiedMultipleChoice {
		t.Errorf("want MC, got %s", bank.Items[0].QuestionType)
	}
}
