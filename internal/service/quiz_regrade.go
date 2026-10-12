package service

// Regrade a question after its answer key changed (G053) — Canvas
// QuizRegrader. Options:
//
//   - current_correct_only          rescore every finished attempt with the
//                                   current key
//   - current_and_previous_correct  award points for both: keep whichever
//                                   of the stored and the rescored points
//                                   is higher
//   - full_credit                   every attempt that contains the
//                                   question gets its full points
//   - no_regrade                    record the change, keep scores
//
// Manually scored answers (essays, uploads, grader overrides) are only
// touched by full_credit.

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// ErrInvalidRegradeOption is returned for an unknown regrade option.
var ErrInvalidRegradeOption = errors.New("invalid regrade option")

var validRegradeOptions = map[string]bool{
	models.QuizRegradeCurrentCorrectOnly: true,
	models.QuizRegradeAwardBoth:          true,
	models.QuizRegradeFullCredit:         true,
	models.QuizRegradeNoRegrade:          true,
}

// RegradeQuizQuestion rescoring every finished attempt that contains the
// question, then refreshes kept scores and the gradebook for the students
// whose scores moved. The question must belong to the quiz.
func (s *QuizService) RegradeQuizQuestion(ctx context.Context, quiz *models.Quiz, question *models.QuizQuestion, option string, actorID uint) (*models.QuizQuestionRegrade, error) {
	if !validRegradeOptions[option] {
		return nil, ErrInvalidRegradeOption
	}
	if question == nil || question.QuizID != quiz.ID {
		return nil, ErrQuestionNotInSubmission
	}

	rec := &models.QuizQuestionRegrade{
		QuizID:          quiz.ID,
		QuizQuestionID:  question.ID,
		RegradeOption:   option,
		AnswersSnapshot: question.Answers,
		RegradedByID:    actorID,
	}

	if option != models.QuizRegradeNoRegrade {
		subs, err := s.submissionRepo.ListCompletedByQuizID(ctx, quiz.ID)
		if err != nil {
			return nil, err
		}
		groupCache := map[uint]*models.QuizQuestionGroup{}
		possible := s.effectiveQuestionPoints(ctx, question, groupCache)
		manualOnly := s.needsManualScore(question)
		touchedUsers := map[uint]bool{}

		for i := range subs {
			sub := &subs[i]
			selected, hasSelected := selectedQuestionSet(sub)
			if !questionBelongsToSubmission(sub, question, selected, hasSelected) {
				continue
			}
			rec.SubmissionsRegraded++

			ans, _ := s.answerRepo.FindBySubmissionAndQuestion(ctx, sub.ID, question.ID)
			old := 0.0
			if ans != nil && ans.Points != nil {
				old = *ans.Points
			}

			var newPts float64
			var correct bool
			switch option {
			case models.QuizRegradeFullCredit:
				newPts, correct = possible, true
			default:
				if manualOnly || (ans != nil && ans.GradedVia != nil && *ans.GradedVia == gradedViaManual) {
					continue // grader-scored: only full credit overrides it
				}
				answerText := ""
				if ans != nil {
					answerText = ans.Answer
				}
				pts, ok, gradable := s.autoGradeWithPoints(question, answerText, possible)
				if !gradable {
					continue
				}
				newPts, correct = pts, ok
				if option == models.QuizRegradeAwardBoth && old > newPts {
					newPts = old
					correct = ans != nil && ans.Correct != nil && *ans.Correct
				}
			}
			newPts = roundScore(newPts)
			if ans != nil && ans.Points != nil && *ans.Points == newPts {
				continue
			}

			if ans == nil {
				ans = &models.QuizSubmissionAnswer{QuizSubmissionID: sub.ID, QuestionID: question.ID}
				if err := s.answerRepo.Create(ctx, ans); err != nil {
					return nil, err
				}
			}
			via := gradedViaAuto
			if manualOnly || (ans.GradedVia != nil && *ans.GradedVia == gradedViaManual) {
				// Full credit on a grader-scored question counts as
				// graded, so it no longer holds the attempt in review.
				via = gradedViaManual
			}
			ans.Points = &newPts
			ans.Correct = &correct
			ans.GradedVia = &via
			if err := s.answerRepo.Update(ctx, ans); err != nil {
				return nil, err
			}

			before := 0.0
			if sub.Score != nil {
				before = *sub.Score
			}
			if sub.ScoreBeforeRegrade == nil {
				b := before
				sub.ScoreBeforeRegrade = &b
			}
			if err := s.rescoreFromStore(ctx, quiz, sub, groupCache); err != nil {
				return nil, err
			}
			if err := s.submissionRepo.Update(ctx, sub); err != nil {
				return nil, err
			}
			rec.SubmissionsChanged++
			touchedUsers[sub.UserID] = true
			s.recordQuizEvent(ctx, sub, models.QuizEventRegraded, map[string]interface{}{
				"question_id":    question.ID,
				"regrade_option": option,
				"old_points":     old,
				"new_points":     newPts,
				"by":             actorID,
			})
		}
		for uid := range touchedUsers {
			s.refreshKeptScore(ctx, quiz, uid, 0)
		}
	}

	if s.lifecycle != nil && s.lifecycle.Regrades != nil {
		if err := s.lifecycle.Regrades.Create(ctx, rec); err != nil {
			return nil, err
		}
	}
	return rec, nil
}

// rescoreFromStore reloads an attempt's questions and answers and
// recomputes its score and review state.
func (s *QuizService) rescoreFromStore(ctx context.Context, quiz *models.Quiz, sub *models.QuizSubmission, groupCache map[uint]*models.QuizQuestionGroup) error {
	questions, _, err := s.attemptQuestions(ctx, sub)
	if err != nil {
		return err
	}
	answers, err := s.answerRepo.ListBySubmissionID(ctx, sub.ID)
	if err != nil {
		return err
	}
	byQuestion := make(map[uint]*models.QuizSubmissionAnswer, len(answers))
	for i := range answers {
		if _, dup := byQuestion[answers[i].QuestionID]; !dup {
			byQuestion[answers[i].QuestionID] = &answers[i]
		}
	}
	s.rescoreAttempt(ctx, quiz, sub, questions, byQuestion, groupCache)
	return nil
}

// ListQuizRegrades returns the quiz's regrade history, newest first.
func (s *QuizService) ListQuizRegrades(ctx context.Context, quizID uint) ([]models.QuizQuestionRegrade, error) {
	if s.lifecycle == nil || s.lifecycle.Regrades == nil {
		return []models.QuizQuestionRegrade{}, nil
	}
	return s.lifecycle.Regrades.ListByQuizID(ctx, quizID)
}
