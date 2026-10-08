package qti

import (
	"encoding/json"
	"testing"
)

// G059: a Canvas calculated_question imported through the standalone QTI
// importer keeps its formula, variables and answer sets.
func TestClassicCalculatedQuestion(t *testing.T) {
	item := classicItemFromTmpl("c1", "calculated_question", "3", `
		<presentation>
			<material><mattext>What is [a] times [b]?</mattext></material>
			<response_str ident="response1" rcardinality="Single"><render_fib fibtype="Decimal"/></response_str>
		</presentation>
		<resprocessing>
			<outcomes><decvar maxvalue="100" varname="SCORE"/></outcomes>
			<respcondition title="correct"><conditionvar><other/></conditionvar><setvar varname="SCORE" action="Set">100</setvar></respcondition>
		</resprocessing>
		<itemproc_extension><calculated>
			<answer_tolerance>5%</answer_tolerance>
			<formulas decimal_places="0"><formula>a*b</formula></formulas>
			<vars><var name="a" scale="0"><min>2</min><max>9</max></var><var name="b" scale="0"><min>2</min><max>9</max></var></vars>
			<var_sets><var_set ident="1"><var name="a">3</var><var name="b">4</var><answer>12</answer></var_set></var_sets>
		</calculated></itemproc_extension>`)
	full := `<?xml version="1.0"?><questestinterop><assessment ident="A1" title="T"><qtimetadata/><section ident="root">` + item + `</section></assessment></questestinterop>`
	quizzes, _, errs := parseClassicAssessmentFile("test.xml", []byte(full))
	if len(errs) > 0 || len(quizzes) != 1 || len(quizzes[0].Questions) != 1 {
		t.Fatalf("parse: quizzes=%d errs=%v", len(quizzes), errs)
	}
	q := quizzes[0].Questions[0]
	if q.QuestionType != UnifiedFormula {
		t.Fatalf("type = %s", q.QuestionType)
	}
	var cfg []formulaConfig
	if err := json.Unmarshal([]byte(q.Answers), &cfg); err != nil || len(cfg) != 1 {
		t.Fatalf("answers %s: %v", q.Answers, err)
	}
	c := cfg[0]
	if c.Formula != "a*b" || c.Tolerance != "5%" || len(c.Variables) != 2 || len(c.AnswerSets) != 1 || c.AnswerSets[0].Answer != 12 {
		t.Fatalf("config = %+v", c)
	}
	if c.AnswerPrecision == nil || *c.AnswerPrecision != 0 {
		t.Fatalf("decimal places = %v", c.AnswerPrecision)
	}
}
