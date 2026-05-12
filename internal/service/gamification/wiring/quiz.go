package wiring

// quiz.go — the "QuizCompletedCallback" emit adapter. Translates a quiz
// service callback (which only knows a submission ID) into a fully
// populated `verb=completed, object_type=Quiz` GamificationEvent and
// hands it to gamification.Emitter.
//
// Errors are logged and swallowed. The quiz service fires this callback
// in a detached goroutine after the terminal workflow write, so
// propagating an error would only crash the goroutine — there's no
// caller to surface it to. Wave 1 keeps the contract narrow: callbacks
// return nothing, log on every load/emit failure with the submission ID
// so a grep on "completed quiz emit" finds every miss.

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
)

// CompletedQuizEmitCallback returns a service.QuizCompletedCallback that
// loads the quiz submission + its quiz + the quiz's course, builds a
// `verb=completed, object_type=Quiz` event, and calls emitter.Emit.
//
// All errors (load failures, emit failures) are logged via slog.Error
// and swallowed. The callback returns nothing because the quiz
// service's fan-out goroutine has no caller to receive an error.
func CompletedQuizEmitCallback(
	emitter *gamification.Emitter,
	quizSubmissionRepo repository.QuizSubmissionRepository,
	quizRepo repository.QuizRepository,
	courseRepo repository.CourseRepository,
) service.QuizCompletedCallback {
	return func(ctx context.Context, submissionID uint) {
		quizSubmission, err := quizSubmissionRepo.FindByID(ctx, submissionID)
		if err != nil {
			slog.Error("completed quiz emit: load submission",
				"submission_id", submissionID, "error", err)
			return
		}

		quiz, err := quizRepo.FindByID(ctx, quizSubmission.QuizID)
		if err != nil {
			slog.Error("completed quiz emit: load quiz",
				"submission_id", submissionID, "error", err)
			return
		}

		course, err := courseRepo.FindByID(ctx, quiz.CourseID)
		if err != nil {
			slog.Error("completed quiz emit: load course",
				"submission_id", submissionID, "error", err)
			return
		}

		// OccurredAt: prefer the actual finish time; fall back to now()
		// only if the submission row has no FinishedAt (callers that
		// reach the "complete" workflow state without setting it are
		// already in a degraded path — the emitter still needs a
		// non-zero xAPI timestamp).
		occurredAt := time.Now()
		if quizSubmission.FinishedAt != nil {
			occurredAt = *quizSubmission.FinishedAt
		}

		result, err := json.Marshal(map[string]any{
			"score":              quizSubmission.Score,
			"workflow_state":     quizSubmission.WorkflowState,
			"time_spent_seconds": quizSubmission.TimeSpent,
			"attempt":            quizSubmission.Attempt,
		})
		if err != nil {
			slog.Error("completed quiz emit: marshal result",
				"submission_id", submissionID, "error", err)
			return
		}

		contextJSON, err := json.Marshal(map[string]any{
			"course_id":          quiz.CourseID,
			"quiz_submission_id": quizSubmission.ID,
		})
		if err != nil {
			slog.Error("completed quiz emit: marshal context",
				"submission_id", submissionID, "error", err)
			return
		}

		event := &models.GamificationEvent{
			OccurredAt: occurredAt,
			TenantID:   course.AccountID,
			ActorID:    quizSubmission.UserID,
			Verb:       gamification.VerbCompleted,
			ObjectType: gamification.ObjectQuiz,
			ObjectID:   &quizSubmission.QuizID,
			Result:     result,
			Context:    contextJSON,
			Source:     gamification.EmitterSource,
		}

		if _, err := emitter.Emit(ctx, event); err != nil {
			slog.Error("completed quiz emit: emit failed",
				"submission_id", submissionID, "error", err)
			return
		}
	}
}
