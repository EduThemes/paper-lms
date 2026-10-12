package service

// Manual grading of a finished quiz attempt (G042) — Canvas
// QuizSubmission#update_scores: per-question scores and comments, and
// whole-attempt fudge points. An attempt stays pending_review while any
// essay / file-upload answer in it has not been scored by a grader.

import (
	"context"
	"errors"
	"math"
	"strconv"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// QuestionScoreUpdate is a grader's change to one question of an attempt.
// nil fields are left unchanged.
type QuestionScoreUpdate struct {
	Score   *float64
	Comment *string
}

// QuizScoreUpdate is a grader's change to a finished attempt.
type QuizScoreUpdate struct {
	// FudgePoints replaces the attempt's fudge points (nil: unchanged).
	FudgePoints *float64
	Questions   map[uint]QuestionScoreUpdate
}

// Manual-grading errors (handlers map them to 400 / 404).
var (
	ErrQuizAttemptNotFinished = errors.New("only a finished quiz attempt can be graded")
	ErrInvalidQuizScore       = errors.New("scores must be finite numbers")
)

const gradedViaManual = "manual"

// attemptQuestions returns the questions that make up an attempt, keyed
// by id (the personalized set, or every authored question).
func (s *QuizService) attemptQuestions(ctx context.Context, sub *models.QuizSubmission) (map[uint]*models.QuizQuestion, []uint, error) {
	qs, err := s.GetSubmissionQuestions(ctx, sub.ID)
	if err != nil {
		return nil, nil, err
	}
	byID := make(map[uint]*models.QuizQuestion, len(qs))
	order := make([]uint, 0, len(qs))
	for i := range qs {
		q := qs[i]
		if q.QuizID != sub.QuizID {
			continue
		}
		byID[q.ID] = &q
		order = append(order, q.ID)
	}
	return byID, order, nil
}

// needsManualScore reports whether a question can only be scored by a
// grader (essay, file upload, and anything the auto-grader does not know).
func (s *QuizService) needsManualScore(q *models.QuizQuestion) bool {
	_, _, gradable := s.autoGradeWithPoints(q, "", 0)
	return !gradable
}

// UpdateQuizScores applies a grader's scores, comments and fudge points to
// a finished attempt, recomputes its score and state, and pushes the kept
// score to the gradebook. Comments are plain text.
func (s *QuizService) UpdateQuizScores(ctx context.Context, quiz *models.Quiz, sub *models.QuizSubmission, upd QuizScoreUpdate, graderID uint) (*models.QuizSubmission, []models.QuizSubmissionAnswer, error) {
	if !sub.IsCompleted() {
		return nil, nil, ErrQuizAttemptNotFinished
	}
	if upd.FudgePoints != nil && !finite(*upd.FudgePoints) {
		return nil, nil, ErrInvalidQuizScore
	}
	for _, q := range upd.Questions {
		if q.Score != nil && !finite(*q.Score) {
			return nil, nil, ErrInvalidQuizScore
		}
	}
	// A closed grading period locks the gradebook grade: refuse before
	// saving question scores the gradebook sync would then drop.
	if err := s.quizGradeWritable(ctx, quiz, sub.UserID); err != nil {
		return nil, nil, err
	}

	questions, _, err := s.attemptQuestions(ctx, sub)
	if err != nil {
		return nil, nil, err
	}
	for qid := range upd.Questions {
		if questions[qid] == nil {
			return nil, nil, ErrQuestionNotInSubmission
		}
	}

	answers, err := s.answerRepo.ListBySubmissionID(ctx, sub.ID)
	if err != nil {
		return nil, nil, err
	}
	byQuestion := make(map[uint]*models.QuizSubmissionAnswer, len(answers))
	for i := range answers {
		if _, dup := byQuestion[answers[i].QuestionID]; !dup {
			byQuestion[answers[i].QuestionID] = &answers[i]
		}
	}

	groupCache := map[uint]*models.QuizQuestionGroup{}
	changed := map[string]interface{}{}
	for qid, u := range upd.Questions {
		q := questions[qid]
		ans := byQuestion[qid]
		if ans == nil {
			// The student skipped this question; a grader can still score
			// or comment on it (Canvas keeps a blank answer row).
			ans = &models.QuizSubmissionAnswer{QuizSubmissionID: sub.ID, QuestionID: qid}
			if err := s.answerRepo.Create(ctx, ans); err != nil {
				return nil, nil, err
			}
			byQuestion[qid] = ans
		}
		if u.Score != nil {
			pts := roundScore(*u.Score)
			possible := s.effectiveQuestionPoints(ctx, q, groupCache)
			correct := possible > 0 && pts >= possible
			via := gradedViaManual
			ans.Points = &pts
			ans.Correct = &correct
			ans.GradedVia = &via
		}
		if u.Comment != nil {
			ans.Comment = SanitizePlainText(*u.Comment)
		}
		if err := s.answerRepo.Update(ctx, ans); err != nil {
			return nil, nil, err
		}
		changed[uintKey(qid)] = ans.Points
	}

	if upd.FudgePoints != nil {
		fp := roundScore(*upd.FudgePoints)
		if fp == 0 {
			sub.FudgePoints = nil
		} else {
			sub.FudgePoints = &fp
		}
	}

	s.rescoreAttempt(ctx, quiz, sub, questions, byQuestion, groupCache)
	if err := s.submissionRepo.Update(ctx, sub); err != nil {
		return nil, nil, err
	}

	s.recordQuizEvent(ctx, sub, models.QuizEventManuallyGraded, map[string]interface{}{
		"grader_id":    graderID,
		"questions":    changed,
		"fudge_points": sub.FudgePoints,
		"score":        sub.Score,
	})
	s.refreshKeptScore(ctx, quiz, sub.UserID, graderID)

	out := make([]models.QuizSubmissionAnswer, 0, len(byQuestion))
	for _, a := range byQuestion {
		out = append(out, *a)
	}
	return sub, out, nil
}

// quizGradeWriteChecker is implemented by *SubmissionService.
type quizGradeWriteChecker interface {
	QuizGradeWritable(ctx context.Context, assignment *models.Assignment, userID uint) error
}

// quizGradeWritable refuses a manual score change on a graded quiz whose
// linked assignment's grade is locked (closed grading period).
func (s *QuizService) quizGradeWritable(ctx context.Context, quiz *models.Quiz, userID uint) error {
	if s.lifecycle == nil || quiz == nil || !quiz.IsGraded() {
		return nil
	}
	checker, ok := s.lifecycle.Gradebook.(quizGradeWriteChecker)
	if !ok {
		return nil
	}
	a := s.linkedAssignment(ctx, quiz)
	if a == nil {
		return nil
	}
	return checker.QuizGradeWritable(ctx, a, userID)
}

// rescoreAttempt recomputes an attempt's score (question points + fudge
// points; a graded survey always earns its full points) and its state:
// pending_review while any manually-scored question is still ungraded.
func (s *QuizService) rescoreAttempt(ctx context.Context, quiz *models.Quiz, sub *models.QuizSubmission, questions map[uint]*models.QuizQuestion, byQuestion map[uint]*models.QuizSubmissionAnswer, groupCache map[uint]*models.QuizQuestionGroup) {
	total := 0.0
	pending := false
	for qid, q := range questions {
		ans := byQuestion[qid]
		if ans != nil && ans.Points != nil {
			total += *ans.Points
		}
		// An essay / upload the student answered that no grader has
		// scored yet keeps the attempt in review (same rule as
		// CompleteSubmission).
		if ans != nil && s.needsManualScore(q) && (ans.GradedVia == nil || *ans.GradedVia != gradedViaManual) {
			pending = true
		}
	}
	if sub.FudgePoints != nil {
		total += *sub.FudgePoints
	}
	total = roundScore(total)
	if quiz != nil && quiz.QuizType == models.QuizTypeGradedSurvey && quiz.PointsPossible != nil {
		total = *quiz.PointsPossible
	}
	sub.Score = &total
	if pending && !quiz.IsSurvey() {
		sub.WorkflowState = models.QuizSubmissionPendingReview
	} else {
		sub.WorkflowState = models.QuizSubmissionComplete
	}
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func uintKey(v uint) string { return strconv.FormatUint(uint64(v), 10) }
