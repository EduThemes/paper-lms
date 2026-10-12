package service_test

// Real-Postgres tests for the quiz-items lane: can't-go-back locks persist
// on the attempt (G049), formula answers are graded against the attempt's
// own variables (G051), surveys never wait for review (G058), and item
// analysis reads the latest finished attempt per student (G054).
// Skipped when PARITY_DB_URL / DATABASE_URL is unset (freshDB).

import (
	"context"
	"errors"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

func TestQuizItemsDB_CantGoBackPersists(t *testing.T) {
	f := newQuizDBFixture(t)
	ctx := context.Background()
	quiz := f.quiz(t, true)
	quiz.OneQuestionAtATime, quiz.CantGoBack = true, true
	if err := f.g.Save(quiz).Error; err != nil {
		t.Fatal(err)
	}
	q1 := &models.QuizQuestion{QuizID: quiz.ID, Position: 1, QuestionType: "multiple_choice", QuestionText: "One", Answers: `[{"id":"a","text":"A","weight":100}]`, WorkflowState: "active"}
	q2 := &models.QuizQuestion{QuizID: quiz.ID, Position: 2, QuestionType: "short_answer", QuestionText: "Two", Answers: `[{"id":"s","text":"x","weight":100}]`, WorkflowState: "active"}
	for _, q := range []*models.QuizQuestion{q1, q2} {
		if err := f.svc.CreateQuestion(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	sub, err := f.svc.StartSubmission(ctx, quiz.ID, f.userID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.AnswerQuestion(ctx, sub.ID, q1.ID, "a"); err != nil {
		t.Fatalf("answer before moving on: %v", err)
	}
	if _, err := f.svc.MarkQuestionRead(ctx, sub, q1.ID); err != nil {
		t.Fatal(err)
	}
	reloaded, _ := f.svc.GetSubmission(ctx, sub.ID)
	if !service.ReadQuestionSet(reloaded)[q1.ID] {
		t.Fatalf("read marker not persisted: %q", reloaded.ReadQuestions)
	}
	if _, err := f.svc.AnswerQuestion(ctx, sub.ID, q1.ID, "b"); !errors.Is(err, service.ErrQuestionLocked) {
		t.Fatalf("answer after moving on: want ErrQuestionLocked, got %v", err)
	}
	if _, err := f.svc.AnswerQuestion(ctx, sub.ID, q2.ID, "x"); err != nil {
		t.Fatalf("unlocked question: %v", err)
	}
	done, err := f.svc.CompleteSubmission(ctx, sub.ID, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	if done.Score == nil || *done.Score != 2 {
		t.Fatalf("the locked first answer still counts: score=%v", done.Score)
	}
}

func TestQuizItemsDB_FormulaGradedPerAttempt(t *testing.T) {
	f := newQuizDBFixture(t)
	ctx := context.Background()
	quiz := f.quiz(t, true)
	pts := 3.0
	q := &models.QuizQuestion{QuizID: quiz.ID, Position: 1, QuestionType: "formula", QuestionText: "Triple [n]", PointsPossible: &pts,
		Answers: `[{"id":"f","weight":100,"formula":"n*3","variables":[{"name":"n","min":2,"max":900,"precision":0}],"tolerance":"0"}]`, WorkflowState: "active"}
	if err := f.svc.CreateQuestion(ctx, q); err != nil {
		t.Fatal(err)
	}
	sub, err := f.svc.StartSubmission(ctx, quiz.ID, f.userID, nil)
	if err != nil {
		t.Fatal(err)
	}
	qs, err := f.svc.GetSubmissionQuestions(ctx, sub.ID)
	if err != nil || len(qs) != 1 {
		t.Fatalf("questions: %v %d", err, len(qs))
	}
	personal, inst := service.PersonalizeFormulaQuestion(qs[0], sub.ID)
	if inst == nil || personal.QuestionText == "Triple [n]" {
		t.Fatalf("question not personalized: %q", personal.QuestionText)
	}
	if _, err := f.svc.AnswerQuestion(ctx, sub.ID, q.ID, service.FormatFormulaNumber(inst.Expected)); err != nil {
		t.Fatal(err)
	}
	done, err := f.svc.CompleteSubmission(ctx, sub.ID, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	if done.WorkflowState != "complete" || done.Score == nil || *done.Score != 3 {
		t.Fatalf("formula attempt: state=%s score=%v", done.WorkflowState, done.Score)
	}
}

func TestQuizItemsDB_SurveyCompletesWithoutReview(t *testing.T) {
	f := newQuizDBFixture(t)
	ctx := context.Background()
	quiz := f.quiz(t, true)
	five := 5.0
	quiz.QuizType, quiz.PointsPossible, quiz.AnonymousSubmissions = "graded_survey", &five, true
	if err := f.g.Save(quiz).Error; err != nil {
		t.Fatal(err)
	}
	var reloadedQuiz models.Quiz
	if err := f.g.First(&reloadedQuiz, quiz.ID).Error; err != nil || !reloadedQuiz.AnonymousSubmissions {
		t.Fatalf("anonymous_submissions not persisted: %v", err)
	}
	essay := &models.QuizQuestion{QuizID: quiz.ID, Position: 1, QuestionType: "essay", QuestionText: "Thoughts?", Answers: `[]`, WorkflowState: "active"}
	if err := f.svc.CreateQuestion(ctx, essay); err != nil {
		t.Fatal(err)
	}
	sub, err := f.svc.StartSubmission(ctx, quiz.ID, f.userID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.AnswerQuestion(ctx, sub.ID, essay.ID, "Loved it"); err != nil {
		t.Fatal(err)
	}
	done, err := f.svc.CompleteSubmission(ctx, sub.ID, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	if done.WorkflowState != "complete" {
		t.Fatalf("survey essays must not wait for review, got %s", done.WorkflowState)
	}
	if done.Score == nil || *done.Score != 5 {
		t.Fatalf("graded survey awards full points, got %v", done.Score)
	}

	report, respondents, _, err := f.svc.BuildItemAnalysis(ctx, &reloadedQuiz)
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Respondents != 1 || len(respondents) != 1 || !report.Anonymous {
		t.Fatalf("report: %+v", report.Summary)
	}
	if report.Items[0].Difficulty != nil {
		t.Fatal("surveys have no difficulty index")
	}
}
