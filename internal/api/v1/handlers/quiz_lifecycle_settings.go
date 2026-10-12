package handlers

// Quiz settings added by the quiz-lifecycle lane: scoring policy (G044),
// answer-key / results release (G043) and the gradebook assignment group
// (G041). Shared by CreateQuiz and UpdateQuiz.

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

var validQuizTypes = map[string]bool{
	models.QuizTypeAssignment:   true,
	models.QuizTypePractice:     true,
	models.QuizTypeGradedSurvey: true,
	models.QuizTypeSurvey:       true,
}

var validScoringPolicies = map[string]bool{
	models.QuizScoringKeepHighest: true,
	models.QuizScoringKeepLatest:  true,
	models.QuizScoringKeepAverage: true,
}

var validHideResults = map[string]bool{
	"":                         true,
	"always":                   true,
	"until_after_last_attempt": true,
}

// quizLifecycleInput is embedded in the create / update quiz bodies. Every
// field is optional; absent keys leave the quiz unchanged, and the two
// dates accept an explicit null to clear them.
type quizLifecycleInput struct {
	ScoringPolicy                 *string                  `json:"scoring_policy"`
	ShowCorrectAnswers            *bool                    `json:"show_correct_answers"`
	ShowCorrectAnswersAt          optionalField[time.Time] `json:"show_correct_answers_at"`
	HideCorrectAnswersAt          optionalField[time.Time] `json:"hide_correct_answers_at"`
	ShowCorrectAnswersLastAttempt *bool                    `json:"show_correct_answers_last_attempt"`
	HideResults                   *string                  `json:"hide_results"`
	OneTimeResults                *bool                    `json:"one_time_results"`
	AssignmentGroupID             *uint                    `json:"assignment_group_id"`
}

// apply validates and copies the settings onto quiz. Returns a non-empty
// message for a 400.
func (in quizLifecycleInput) apply(quiz *models.Quiz) string {
	if in.ScoringPolicy != nil {
		if !validScoringPolicies[*in.ScoringPolicy] {
			return "Invalid scoring_policy"
		}
		quiz.ScoringPolicy = *in.ScoringPolicy
	}
	if in.HideResults != nil {
		if !validHideResults[*in.HideResults] {
			return "Invalid hide_results"
		}
		quiz.HideResults = *in.HideResults
	}
	if in.ShowCorrectAnswers != nil {
		quiz.ShowCorrectAnswers = *in.ShowCorrectAnswers
	}
	applyOptionalTime(&quiz.ShowCorrectAnswersAt, in.ShowCorrectAnswersAt)
	applyOptionalTime(&quiz.HideCorrectAnswersAt, in.HideCorrectAnswersAt)
	if in.ShowCorrectAnswersLastAttempt != nil {
		quiz.ShowCorrectAnswersLastAttempt = *in.ShowCorrectAnswersLastAttempt
	}
	if in.OneTimeResults != nil {
		quiz.OneTimeResults = *in.OneTimeResults
	}
	if quiz.ShowCorrectAnswersAt != nil && quiz.HideCorrectAnswersAt != nil &&
		!quiz.HideCorrectAnswersAt.After(*quiz.ShowCorrectAnswersAt) {
		return "hide_correct_answers_at must be after show_correct_answers_at"
	}
	if quiz.UnlockAt != nil && quiz.LockAt != nil && quiz.LockAt.Before(*quiz.UnlockAt) {
		return "lock_at must not be before unlock_at"
	}
	return ""
}

// assignmentOptions validates an assignment_group_id from the body: it
// must be a live group of the URL's course in the caller's tenant (404
// otherwise, never a cross-tenant existence hint). wrote=true means a
// response was written and the caller must return.
func (h *QuizHandler) assignmentOptions(c *fiber.Ctx, groupID *uint, courseID uint) (*service.QuizAssignmentOptions, bool) {
	if groupID == nil || h.quizService == nil {
		return nil, false
	}
	if !h.quizService.AssignmentGroupInCourse(c.Context(), *groupID, courseID, callerAccountID(c)) {
		_ = responses.NotFound(c, "assignment group")
		return nil, true
	}
	gid := *groupID
	return &service.QuizAssignmentOptions{AssignmentGroupID: &gid}, false
}

// syncAssignment keeps the quiz's gradebook assignment in step after a
// save. Reports false on failure (logged).
func (h *QuizHandler) syncAssignment(c *fiber.Ctx, quiz *models.Quiz, opts *service.QuizAssignmentOptions) bool {
	if h.quizService == nil {
		return true
	}
	if _, err := h.quizService.SyncQuizAssignment(c.Context(), quiz, opts); err != nil {
		slog.Error("quiz assignment sync", "quiz_id", quiz.ID, "err", err)
		return false
	}
	return true
}
