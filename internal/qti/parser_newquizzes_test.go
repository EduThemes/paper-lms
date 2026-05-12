package qti

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestNewQuizzesItemTypes is the NQ equivalent of TestClassicItemTypes.
// Each row provides a minimal <assessmentItem> XML, the expected unified
// type, and an assertion on the answers JSON.
func TestNewQuizzesItemTypes(t *testing.T) {
	cases := []struct {
		name          string
		xml           string
		wantType      string
		assertAnswers func(t *testing.T, answers string)
	}{
		{
			name:     "choice_single_to_multiple_choice",
			wantType: UnifiedMultipleChoice,
			xml: `<assessmentItem identifier="i1">
				<responseDeclaration identifier="RESPONSE" cardinality="single" baseType="identifier">
					<correctResponse><value>b</value></correctResponse>
				</responseDeclaration>
				<itemBody>
					<p>Q?</p>
					<choiceInteraction responseIdentifier="RESPONSE" maxChoices="1">
						<simpleChoice identifier="a">A</simpleChoice>
						<simpleChoice identifier="b">B</simpleChoice>
					</choiceInteraction>
				</itemBody>
			</assessmentItem>`,
			assertAnswers: func(t *testing.T, a string) {
				if !strings.Contains(a, `"id":"b"`) || !strings.Contains(a, `"weight":100`) {
					t.Errorf("expected b correct in %s", a)
				}
			},
		},
		{
			name:     "choice_two_options_TF",
			wantType: UnifiedTrueFalse,
			xml: `<assessmentItem identifier="i2">
				<responseDeclaration identifier="RESPONSE" cardinality="single" baseType="identifier">
					<correctResponse><value>t</value></correctResponse>
				</responseDeclaration>
				<itemBody>
					<p>Sky is blue.</p>
					<choiceInteraction responseIdentifier="RESPONSE" maxChoices="1">
						<simpleChoice identifier="t">True</simpleChoice>
						<simpleChoice identifier="f">False</simpleChoice>
					</choiceInteraction>
				</itemBody>
			</assessmentItem>`,
		},
		{
			name:     "choice_multi_to_multiple_answer",
			wantType: UnifiedMultipleAnswer,
			xml: `<assessmentItem identifier="i3">
				<responseDeclaration identifier="RESPONSE" cardinality="multiple" baseType="identifier">
					<correctResponse><value>a</value><value>b</value></correctResponse>
				</responseDeclaration>
				<itemBody>
					<choiceInteraction responseIdentifier="RESPONSE" maxChoices="0">
						<simpleChoice identifier="a">A</simpleChoice>
						<simpleChoice identifier="b">B</simpleChoice>
						<simpleChoice identifier="c">C</simpleChoice>
					</choiceInteraction>
				</itemBody>
			</assessmentItem>`,
			assertAnswers: func(t *testing.T, a string) {
				var opts []map[string]interface{}
				_ = json.Unmarshal([]byte(a), &opts)
				correct := 0
				for _, o := range opts {
					if w, _ := o["weight"].(float64); w > 0 {
						correct++
					}
				}
				if correct != 2 {
					t.Errorf("want 2 correct opts, got %d", correct)
				}
			},
		},
		{
			name:     "textentry_to_fill_in_the_blank",
			wantType: UnifiedFillInTheBlank,
			xml: `<assessmentItem identifier="i4">
				<responseDeclaration identifier="RESPONSE" cardinality="single" baseType="string">
					<correctResponse><value>Mars</value></correctResponse>
				</responseDeclaration>
				<itemBody>
					<p>Planet?</p>
					<textEntryInteraction responseIdentifier="RESPONSE"/>
				</itemBody>
			</assessmentItem>`,
			assertAnswers: func(t *testing.T, a string) {
				if !strings.Contains(a, `"text":"Mars"`) {
					t.Errorf("want Mars in answers, got %s", a)
				}
			},
		},
		{
			name:     "textentry_float_to_numerical",
			wantType: UnifiedNumerical,
			xml: `<assessmentItem identifier="i5">
				<responseDeclaration identifier="RESPONSE" cardinality="single" baseType="float">
					<correctResponse><value>42</value></correctResponse>
					<mapping lowerBound="41.5" upperBound="42.5" defaultValue="0"/>
				</responseDeclaration>
				<itemBody>
					<textEntryInteraction responseIdentifier="RESPONSE"/>
				</itemBody>
			</assessmentItem>`,
			assertAnswers: func(t *testing.T, a string) {
				if !strings.Contains(a, `"margin":"0.5"`) {
					t.Errorf("want margin 0.5 in %s", a)
				}
			},
		},
		{
			name:     "extendedtext_to_essay",
			wantType: UnifiedEssay,
			xml: `<assessmentItem identifier="i6">
				<responseDeclaration identifier="RESPONSE" cardinality="single" baseType="string"/>
				<itemBody>
					<extendedTextInteraction responseIdentifier="RESPONSE" expectedLength="200"/>
				</itemBody>
			</assessmentItem>`,
		},
		{
			name:     "upload_to_file_upload",
			wantType: UnifiedFileUpload,
			xml: `<assessmentItem identifier="i7">
				<responseDeclaration identifier="RESPONSE" cardinality="single" baseType="file"/>
				<itemBody>
					<uploadInteraction responseIdentifier="RESPONSE"/>
				</itemBody>
			</assessmentItem>`,
		},
		{
			name:     "match_to_matching",
			wantType: UnifiedMatching,
			xml: `<assessmentItem identifier="i8">
				<responseDeclaration identifier="RESPONSE" cardinality="multiple" baseType="directedPair">
					<correctResponse>
						<value>france paris</value>
						<value>uk london</value>
					</correctResponse>
				</responseDeclaration>
				<itemBody>
					<matchInteraction responseIdentifier="RESPONSE" maxAssociations="2">
						<simpleMatchSet>
							<simpleAssociableChoice identifier="france" matchMax="1">France</simpleAssociableChoice>
							<simpleAssociableChoice identifier="uk" matchMax="1">UK</simpleAssociableChoice>
						</simpleMatchSet>
						<simpleMatchSet>
							<simpleAssociableChoice identifier="paris" matchMax="1">Paris</simpleAssociableChoice>
							<simpleAssociableChoice identifier="london" matchMax="1">London</simpleAssociableChoice>
						</simpleMatchSet>
					</matchInteraction>
				</itemBody>
			</assessmentItem>`,
			assertAnswers: func(t *testing.T, a string) {
				if !strings.Contains(a, `"right_id":"paris"`) ||
					!strings.Contains(a, `"right_id":"london"`) {
					t.Errorf("expected both pairs in %s", a)
				}
			},
		},
		{
			name:     "inline_to_dropdown",
			wantType: UnifiedMultipleDropdown,
			xml: `<assessmentItem identifier="i9">
				<responseDeclaration identifier="b1" cardinality="single" baseType="identifier">
					<correctResponse><value>sky</value></correctResponse>
				</responseDeclaration>
				<itemBody>
					<p>The
						<inlineChoiceInteraction responseIdentifier="b1">
							<inlineChoice identifier="sky">sky</inlineChoice>
							<inlineChoice identifier="grass">grass</inlineChoice>
						</inlineChoiceInteraction>
					</p>
				</itemBody>
			</assessmentItem>`,
			assertAnswers: func(t *testing.T, a string) {
				if !strings.Contains(a, `"blank_id":"b1"`) {
					t.Errorf("want blank_id b1 in %s", a)
				}
			},
		},
		{
			name:     "order_to_ordering",
			wantType: UnifiedOrdering,
			xml: `<assessmentItem identifier="i10">
				<responseDeclaration identifier="RESPONSE" cardinality="ordered" baseType="identifier">
					<correctResponse><value>s1</value><value>s2</value></correctResponse>
				</responseDeclaration>
				<itemBody>
					<orderInteraction responseIdentifier="RESPONSE">
						<simpleChoice identifier="s1">First</simpleChoice>
						<simpleChoice identifier="s2">Second</simpleChoice>
					</orderInteraction>
				</itemBody>
			</assessmentItem>`,
			assertAnswers: func(t *testing.T, a string) {
				var opts []map[string]interface{}
				_ = json.Unmarshal([]byte(a), &opts)
				if len(opts) != 2 || opts[0]["id"] != "s1" || opts[1]["id"] != "s2" {
					t.Errorf("canonical order not preserved: %s", a)
				}
			},
		},
		{
			name:     "gapmatch_to_categorization",
			wantType: UnifiedCategorization,
			xml: `<assessmentItem identifier="i11">
				<responseDeclaration identifier="RESPONSE" cardinality="multiple" baseType="directedPair">
					<correctResponse>
						<value>apple fruit</value>
						<value>carrot veg</value>
					</correctResponse>
				</responseDeclaration>
				<itemBody>
					<gapMatchInteraction responseIdentifier="RESPONSE">
						<gapText identifier="apple" matchMax="1">Apple</gapText>
						<gapText identifier="carrot" matchMax="1">Carrot</gapText>
						<p>Fruits: <gap identifier="fruit"/></p>
					</gapMatchInteraction>
				</itemBody>
			</assessmentItem>`,
			assertAnswers: func(t *testing.T, a string) {
				if !strings.Contains(a, `"right_id":"fruit"`) {
					t.Errorf("expected fruit bucket in %s", a)
				}
			},
		},
		{
			name:     "hotspot_to_hotspot",
			wantType: UnifiedHotSpot,
			xml: `<assessmentItem identifier="i12">
				<responseDeclaration identifier="RESPONSE" cardinality="single" baseType="identifier">
					<correctResponse><value>spot1</value></correctResponse>
				</responseDeclaration>
				<itemBody>
					<hotspotInteraction responseIdentifier="RESPONSE" maxChoices="1">
						<object data="x.png" type="image/png" width="100" height="100"/>
						<hotspotChoice identifier="spot1" shape="rect" coords="10,10,50,50"/>
					</hotspotInteraction>
				</itemBody>
			</assessmentItem>`,
			assertAnswers: func(t *testing.T, a string) {
				if !strings.Contains(a, `"w":40`) || !strings.Contains(a, `"h":40`) {
					t.Errorf("expected width/height 40 in %s", a)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q, _, perr := parseNewQuizzesItem("test.xml", []byte(tc.xml), 0)
			if perr != nil {
				t.Fatalf("parse error: %+v", *perr)
			}
			if q.QuestionType != tc.wantType {
				t.Errorf("want %s, got %s", tc.wantType, q.QuestionType)
			}
			if tc.assertAnswers != nil {
				tc.assertAnswers(t, q.Answers)
			}
		})
	}
}

// TestNewQuizzesClassifierEdgeCases hardens the classifier helpers
// against tricky inputs (whitespace, mixed case, etc).
func TestNewQuizzesClassifierEdgeCases(t *testing.T) {
	if got := ClassifyNewQuizzesChoice([]string{" TRUE ", " false "}, 1); got != UnifiedTrueFalse {
		t.Errorf("whitespace+case TF: got %s", got)
	}
	if got := ClassifyNewQuizzesChoice([]string{"A", "B"}, 1); got != UnifiedMultipleChoice {
		t.Errorf("non-TF 2 options: got %s", got)
	}
	if got := ClassifyNewQuizzesChoice([]string{"A", "B", "C"}, 0); got != UnifiedMultipleAnswer {
		t.Errorf("maxChoices=0 (unlimited): got %s", got)
	}
	if got := ClassifyNewQuizzesTextEntry(true); got != UnifiedNumerical {
		t.Errorf("textentry+tolerance: got %s", got)
	}
	if got := ClassifyNewQuizzesTextEntry(false); got != UnifiedFillInTheBlank {
		t.Errorf("textentry no tolerance: got %s", got)
	}
}
