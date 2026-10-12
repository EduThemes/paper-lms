package service

// G059: Canvas course-import fidelity for quiz items — calculated
// questions keep their formula/variables/answer sets and grade per
// attempt, precision numerical answers keep their tolerance window, and
// quiz delivery restrictions survive the import.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

const canvasCalculatedBody = `
        <presentation>
          <material><mattext texttype="text/html">&lt;p&gt;What is [x] plus [y]?&lt;/p&gt;</mattext></material>
          <response_str ident="response1" rcardinality="Single"><render_fib fibtype="Decimal"><response_label ident="answer1"/></render_fib></response_str>
        </presentation>
        <resprocessing>
          <outcomes><decvar maxvalue="100" minvalue="0" varname="SCORE" vartype="Decimal"/></outcomes>
          <respcondition title="correct"><conditionvar><other/></conditionvar><setvar varname="SCORE" action="Set">100</setvar></respcondition>
          <respcondition title="incorrect"><conditionvar><not><other/></not></conditionvar><setvar varname="SCORE" action="Set">0</setvar></respcondition>
        </resprocessing>
        <itemproc_extension>
          <calculated>
            <answer_tolerance>0.1</answer_tolerance>
            <formulas decimal_places="1"><formula>x+y</formula></formulas>
            <vars>
              <var name="x" scale="0"><min>1</min><max>10</max></var>
              <var name="y" scale="1"><min>0</min><max>1</max></var>
            </vars>
            <var_sets>
              <var_set ident="s1"><var name="x">3</var><var name="y">0.5</var><answer>3.5</answer></var_set>
              <var_set ident="s2"><var name="x">7</var><var name="y">0.2</var><answer>7.2</answer></var_set>
            </var_sets>
          </calculated>
        </itemproc_extension>`

func TestCanvasImport_CalculatedQuestionKeepsAnswerSets(t *testing.T) {
	r, err := ParseQTIAssessment([]byte(canvasItem("calculated_question", canvasCalculatedBody)))
	if err != nil {
		t.Fatal(err)
	}
	q := r.Questions[0]
	if q.QuestionType != "formula" {
		t.Fatalf("type = %s", q.QuestionType)
	}
	cfg, ok := ParseFormulaConfig(q.Answers)
	if !ok || cfg.Formula != "x+y" || len(cfg.Variables) != 2 || len(cfg.AnswerSets) != 2 {
		t.Fatalf("config = %+v (ok=%v) from %s", cfg, ok, q.Answers)
	}
	if cfg.AnswerPrecision == nil || *cfg.AnswerPrecision != 1 || cfg.Variables[1].Precision != 1 {
		t.Fatalf("decimal places lost: %+v", cfg)
	}

	// Each attempt draws one set; the attempt's own answer is correct, the
	// personalized text shows the set's values.
	q.ID = 42
	s := &QuizService{}
	for sub := uint(1); sub <= 6; sub++ {
		personal, inst := PersonalizeFormulaQuestion(q, sub)
		if inst == nil {
			t.Fatal("expected an instance")
		}
		if strings.Contains(personal.QuestionText, "[x]") {
			t.Fatalf("tokens left in %q", personal.QuestionText)
		}
		if _, correct, gradable := s.gradeAnswerForSubmission(sub, &q, FormatFormulaNumber(inst.Expected), 2); !correct || !gradable {
			t.Fatalf("attempt %d: own answer %v rejected", sub, inst.Expected)
		}
		if _, correct, _ := s.gradeAnswerForSubmission(sub, &q, FormatFormulaNumber(inst.Expected+0.5), 2); correct {
			t.Fatal("answer outside the 0.1 tolerance accepted")
		}
	}
}

func TestCanvasImport_PrecisionAnswerKeepsWindow(t *testing.T) {
	// Canvas numerical precision_answer (approximate 13.4, precision 4).
	body := `
        <presentation>
          <material><mattext>Approximately?</mattext></material>
          <response_str ident="response1" rcardinality="Single"><render_fib fibtype="Decimal"><response_label ident="answer1"/></render_fib></response_str>
        </presentation>
        <resprocessing>
          <outcomes><decvar maxvalue="100" minvalue="0" varname="SCORE" vartype="Decimal"/></outcomes>
          <respcondition continue="No">
            <conditionvar><or>
              <varequal respident="response1">13.4</varequal>
              <and><vargt respident="response1">13.395</vargt><varlte respident="response1">13.405</varlte></and>
            </or></conditionvar>
            <setvar action="Set" varname="SCORE">100</setvar>
          </respcondition>
        </resprocessing>`
	xml := canvasItem("numerical_question", body)
	for _, tc := range []struct {
		in   string
		want bool
	}{{"13.4", true}, {"13.402", true}, {"13.396", true}, {"13.41", false}, {"13.3", false}} {
		_, pts, gradable := gradeImported(t, xml, tc.in)
		if !gradable || (pts > 0) != tc.want {
			t.Errorf("%s: pts=%v gradable=%v, want correct=%v", tc.in, pts, gradable, tc.want)
		}
	}
}

func TestImportQuizMeta_DeliveryRestrictions(t *testing.T) {
	prev := quizSecretSealer
	t.Cleanup(func() { quizSecretSealer = prev })
	SetQuizSecretSealer(func(b []byte) ([]byte, error) { return append([]byte("sealed:"), b...), nil })

	q := &models.Quiz{}
	applyQuizMeta(q, &canvasQuizMeta{
		OneQuestionAtATime:   "true",
		CantGoBack:           "true",
		AccessCode:           "lab-42",
		IPFilter:             "10.0.0.0/8, 192.168.0.5",
		AnonymousSubmissions: "true",
	})
	if !q.OneQuestionAtATime || !q.CantGoBack || !q.AnonymousSubmissions {
		t.Fatalf("flags lost: %+v", q)
	}
	if q.IPFilter != "10.0.0.0/8,192.168.0.5" {
		t.Fatalf("ip filter = %q", q.IPFilter)
	}
	if !bytes.Equal(q.AccessCodeEncrypted, []byte("sealed:lab-42")) {
		t.Fatalf("access code not sealed: %q", q.AccessCodeEncrypted)
	}

	// An unparseable filter is dropped rather than locking everyone out;
	// without a sealer the code is not stored in plaintext.
	SetQuizSecretSealer(nil)
	q2 := &models.Quiz{}
	applyQuizMeta(q2, &canvasQuizMeta{AccessCode: "x", IPFilter: "lab-network"})
	if q2.IPFilter != "" || len(q2.AccessCodeEncrypted) != 0 {
		t.Fatalf("unexpected restrictions: %+v", q2)
	}
}
