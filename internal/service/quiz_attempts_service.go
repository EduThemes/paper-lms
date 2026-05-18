package service

// Quiz submission/attempt methods — StartSubmission, AnswerQuestion,
// CompleteSubmission, GetSubmission, ListSubmissions, ListSubmissionAnswers,
// GetSubmissionQuestions, plus the private generators (validation token,
// personalized question set) and the QuizCompletedCallback wiring.
//
// Wave 5 split (chore/wave5-split-quiz-blueprint): methods hang off
// *QuizService so the handler surface is unchanged. Tests are split into
// quiz_attempts_test.go.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	mathrand "math/rand"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// QuizCompletedCallback is invoked asynchronously after a quiz submission
// transitions to "complete" or "pending_review" via CompleteSubmission.
// Mirrors SubmissionGradedCallback: callbacks receive a fresh context and
// must be self-contained (no reliance on the request's context.Cancel).
// Used by the gamification engine to fire `verb=completed, object_type=Quiz`
// rules without coupling the quiz service to the rule engine.
type QuizCompletedCallback func(ctx context.Context, submissionID uint)

// OnCompleted registers a callback to fire after CompleteSubmission
// successfully writes the terminal workflow state. The callback runs in
// a fresh goroutine with a detached context.Background(); panics are
// recovered and logged. Multiple registrations stack; order is
// registration order.
func (s *QuizService) OnCompleted(cb QuizCompletedCallback) {
	s.onCompletedCallbacks = append(s.onCompletedCallbacks, cb)
}

// fireOnCompleted runs all registered callbacks in goroutines with a
// detached context. Panics are recovered. Errors are the callback's
// responsibility — the signature returns nothing.
func (s *QuizService) fireOnCompleted(submissionID uint) {
	for _, cb := range s.onCompletedCallbacks {
		go func(cb QuizCompletedCallback) {
			defer recoverFromPanic("quiz OnCompleted callback")
			cb(context.Background(), submissionID)
		}(cb)
	}
}

// generateValidationToken creates a cryptographically random hex token.
func generateValidationToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// generateSelectedQuestions builds the personalized question ID list for a quiz submission.
// For questions not in any group, all are included.
// For each QuizQuestionGroup, PickCount questions are randomly selected from the pool.
func (s *QuizService) generateSelectedQuestions(ctx context.Context, quizID uint) ([]uint, error) {
	// Get all questions for this quiz
	allQuestions, err := s.questionRepo.ListByQuizID(ctx, quizID, repository.PaginationParams{Page: 1, PerPage: 10000})
	if err != nil {
		return nil, err
	}

	// If no group repo is configured, return all question IDs
	if s.groupRepo == nil {
		ids := make([]uint, len(allQuestions.Items))
		for i, q := range allQuestions.Items {
			ids[i] = q.ID
		}
		return ids, nil
	}

	// Get all question groups for this quiz
	groups, err := s.groupRepo.ListByQuizID(ctx, quizID)
	if err != nil {
		return nil, err
	}

	// If no groups exist, return all question IDs (no randomization needed)
	if len(groups) == 0 {
		ids := make([]uint, len(allQuestions.Items))
		for i, q := range allQuestions.Items {
			ids[i] = q.ID
		}
		return ids, nil
	}

	var selectedIDs []uint

	// Track which questions belong to a group so we can add ungrouped ones
	groupedQuestionIDs := make(map[uint]bool)

	rng := mathrand.New(mathrand.NewSource(time.Now().UnixNano()))

	for _, group := range groups {
		var pool []uint

		// Option 1: Group pulls from a linked QuestionBank
		if group.QuestionBankID != nil && s.bankEntryRepo != nil {
			entries, err := s.bankEntryRepo.ListByBankID(ctx, *group.QuestionBankID)
			if err == nil {
				for _, entry := range entries {
					pool = append(pool, entry.ID)
				}
			}
		}

		// Option 2: Group uses questions directly assigned to it via QuizQuestionGroupID
		groupQuestions, err := s.questionRepo.ListByGroupID(ctx, group.ID)
		if err == nil {
			for _, q := range groupQuestions {
				pool = append(pool, q.ID)
				groupedQuestionIDs[q.ID] = true
			}
		}

		// Randomly pick PickCount questions from the pool
		if len(pool) > 0 {
			// Shuffle the pool
			rng.Shuffle(len(pool), func(i, j int) {
				pool[i], pool[j] = pool[j], pool[i]
			})

			pickCount := group.PickCount
			if pickCount > len(pool) {
				pickCount = len(pool)
			}
			selectedIDs = append(selectedIDs, pool[:pickCount]...)
		}
	}

	// Add all ungrouped questions (questions not assigned to any group)
	for _, q := range allQuestions.Items {
		if !groupedQuestionIDs[q.ID] && (q.QuizQuestionGroupID == nil || *q.QuizQuestionGroupID == 0) {
			selectedIDs = append(selectedIDs, q.ID)
		}
	}

	return selectedIDs, nil
}

// GetSubmissionQuestions returns the personalized list of questions for a specific submission.
// If the submission has a SelectedQuestions field, only those questions are returned.
// Otherwise, all quiz questions are returned.
func (s *QuizService) GetSubmissionQuestions(ctx context.Context, submissionID uint) ([]models.QuizQuestion, error) {
	submission, err := s.submissionRepo.FindByID(ctx, submissionID)
	if err != nil {
		return nil, errors.New("quiz submission not found")
	}

	// If there are selected questions, parse and return only those
	if submission.SelectedQuestions != "" {
		var questionIDs []uint
		if err := json.Unmarshal([]byte(submission.SelectedQuestions), &questionIDs); err != nil {
			return nil, errors.New("could not parse selected questions")
		}

		questions := make([]models.QuizQuestion, 0, len(questionIDs))
		for _, qID := range questionIDs {
			q, err := s.questionRepo.FindByID(ctx, qID)
			if err != nil {
				continue // skip questions that no longer exist
			}
			questions = append(questions, *q)
		}
		return questions, nil
	}

	// No selected questions stored - return all quiz questions
	result, err := s.questionRepo.ListByQuizID(ctx, submission.QuizID, repository.PaginationParams{Page: 1, PerPage: 10000})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// ---------- Quiz Submission Methods ----------

// StartSubmission creates a new quiz submission for the given user.
// If timeLimit is provided (in minutes), the EndAt field is set accordingly.
func (s *QuizService) StartSubmission(ctx context.Context, quizID, userID uint, timeLimit *int) (*models.QuizSubmission, error) {
	// Check for an existing in-progress submission
	existing, _ := s.submissionRepo.FindByQuizAndUser(ctx, quizID, userID)
	if existing != nil && existing.WorkflowState == "untaken" {
		// Return the existing untaken submission so the user can resume
		return existing, nil
	}

	// Calculate attempt number
	attempt := 1
	if existing != nil {
		attempt = existing.Attempt + 1
	}

	// Enforce attempt limits
	quiz, err := s.quizRepo.FindByID(ctx, quizID, 0)
	if err != nil {
		return nil, errors.New("quiz not found")
	}
	if quiz.AllowedAttempts > 0 && attempt > quiz.AllowedAttempts {
		return nil, errors.New("maximum number of attempts reached")
	}

	token, err := generateValidationToken()
	if err != nil {
		return nil, errors.New("could not generate validation token")
	}

	now := time.Now()

	submission := &models.QuizSubmission{
		QuizID:          quizID,
		UserID:          userID,
		Attempt:         attempt,
		StartedAt:       &now,
		ValidationToken: token,
		WorkflowState:   "untaken",
	}

	// Use quiz time limit if no override provided
	effectiveTimeLimit := timeLimit
	if effectiveTimeLimit == nil && quiz.TimeLimit != nil && *quiz.TimeLimit > 0 {
		effectiveTimeLimit = quiz.TimeLimit
	}

	// Apply student accommodations (IEP/504 time extensions)
	if s.accommodationService != nil && effectiveTimeLimit != nil && *effectiveTimeLimit > 0 {
		courseID := quiz.CourseID
		adjustment, err := s.accommodationService.ApplyAccommodationsToQuiz(ctx, userID, &courseID, effectiveTimeLimit)
		if err == nil && adjustment != nil {
			adjusted := adjustment.AdjustedTimeLimit
			effectiveTimeLimit = &adjusted
		}
	}

	if effectiveTimeLimit != nil && *effectiveTimeLimit > 0 {
		endAt := now.Add(time.Duration(*effectiveTimeLimit) * time.Minute)
		submission.EndAt = &endAt
	}

	// Generate personalized question set if question groups exist
	selectedIDs, err := s.generateSelectedQuestions(ctx, quizID)
	if err == nil && len(selectedIDs) > 0 {
		selectedJSON, err := json.Marshal(selectedIDs)
		if err == nil {
			submission.SelectedQuestions = string(selectedJSON)
		}
	}

	if err := s.submissionRepo.Create(ctx, submission); err != nil {
		return nil, err
	}

	return submission, nil
}

func (s *QuizService) GetSubmission(ctx context.Context, id uint) (*models.QuizSubmission, error) {
	return s.submissionRepo.FindByID(ctx, id)
}

// AnswerQuestion upserts an answer for a specific question in a quiz submission.
func (s *QuizService) AnswerQuestion(ctx context.Context, submissionID, questionID uint, answer string) (*models.QuizSubmissionAnswer, error) {
	// Verify the submission exists and is still in progress
	submission, err := s.submissionRepo.FindByID(ctx, submissionID)
	if err != nil {
		return nil, errors.New("quiz submission not found")
	}

	if submission.WorkflowState != "untaken" {
		return nil, errors.New("quiz submission is not in progress")
	}

	// Check if time has expired
	if submission.EndAt != nil && time.Now().After(*submission.EndAt) {
		return nil, errors.New("quiz time has expired")
	}

	// Attempt to find an existing answer for this question in this submission
	existing, _ := s.answerRepo.FindBySubmissionAndQuestion(ctx, submissionID, questionID)
	if existing != nil {
		existing.Answer = answer
		// Reset grading fields — they will be recalculated on completion
		existing.Correct = nil
		existing.Points = nil
		if err := s.answerRepo.Update(ctx, existing); err != nil {
			return nil, err
		}
		return existing, nil
	}

	// Create new answer
	newAnswer := &models.QuizSubmissionAnswer{
		QuizSubmissionID: submissionID,
		QuestionID:       questionID,
		Answer:           answer,
	}

	if err := s.answerRepo.Create(ctx, newAnswer); err != nil {
		return nil, err
	}

	return newAnswer, nil
}

// CompleteSubmission finalizes a quiz submission, triggers auto-grading, and calculates the score.
func (s *QuizService) CompleteSubmission(ctx context.Context, submissionID, userID uint) (*models.QuizSubmission, error) {
	submission, err := s.submissionRepo.FindByID(ctx, submissionID)
	if err != nil {
		return nil, errors.New("quiz submission not found")
	}

	if submission.UserID != userID {
		return nil, errors.New("unauthorized: submission does not belong to this user")
	}

	if submission.WorkflowState != "untaken" {
		return nil, errors.New("quiz submission is not in progress")
	}

	// Get all answers for this submission
	answers, err := s.answerRepo.ListBySubmissionID(ctx, submissionID)
	if err != nil {
		return nil, errors.New("could not fetch submission answers")
	}

	totalScore := 0.0
	needsReview := false

	for i := range answers {
		question, qErr := s.questionRepo.FindByID(ctx, answers[i].QuestionID)
		if qErr != nil {
			continue
		}

		points, correct, gradable := s.autoGrade(question, answers[i].Answer)

		if gradable {
			answers[i].Correct = &correct
			answers[i].Points = &points
			// Wave A1: stamp the audit trail so we can distinguish auto-graded
			// rows from manual SpeedGrader rows and pre-cutover legacy rows.
			via := gradedViaAuto
			answers[i].GradedVia = &via
			totalScore += points
		} else {
			// Essay, file_upload, and other non-auto-gradable types require
			// manual review. We deliberately do NOT stamp GradedVia here —
			// it will be set to "manual" when the instructor scores it.
			needsReview = true
			zero := 0.0
			answers[i].Points = &zero
		}

		_ = s.answerRepo.Update(ctx, &answers[i])
	}

	now := time.Now()
	submission.FinishedAt = &now
	submission.Score = &totalScore
	submission.KeptScore = &totalScore

	if submission.StartedAt != nil {
		spent := int(now.Sub(*submission.StartedAt).Seconds())
		submission.TimeSpent = spent
	}

	if needsReview {
		submission.WorkflowState = "pending_review"
	} else {
		submission.WorkflowState = "complete"
	}

	if err := s.submissionRepo.Update(ctx, submission); err != nil {
		return nil, err
	}

	// Fire post-completion callbacks (gamification, notifications, etc.)
	// asynchronously. Failures in callbacks must never block completion
	// or surface as errors here.
	s.fireOnCompleted(submission.ID)

	return submission, nil
}

func (s *QuizService) ListSubmissions(ctx context.Context, quizID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.QuizSubmission], error) {
	return s.submissionRepo.ListByQuizID(ctx, quizID, params)
}

// ListSubmissionAnswers returns all answers for a completed quiz submission.
func (s *QuizService) ListSubmissionAnswers(ctx context.Context, submissionID, userID uint) ([]models.QuizSubmissionAnswer, error) {
	submission, err := s.submissionRepo.FindByID(ctx, submissionID)
	if err != nil {
		return nil, errors.New("quiz submission not found")
	}
	// Only the submitting user (or via instructor route) can view answers
	if submission.UserID != userID {
		return nil, errors.New("unauthorized: submission does not belong to this user")
	}
	if submission.WorkflowState == "untaken" {
		return nil, errors.New("quiz submission is still in progress")
	}
	return s.answerRepo.ListBySubmissionID(ctx, submissionID)
}
