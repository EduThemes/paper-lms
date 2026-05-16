package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	mathrand "math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// gradedViaAuto is the audit-trail value stamped on quiz_submission_answer
// rows that were scored by the auto-grader. Added in Wave A1 alongside the
// matching / fill_in_multiple_blanks cutover so legacy pending_review rows
// (which pre-date this constant) can be distinguished from newly-graded ones.
const gradedViaAuto = "auto"

// answerOption represents one answer choice in a quiz question's Answers JSON field.
// Extended in Wave A1: the optional Margin field is now read by the numerical
// grader, and several new fields are consumed by the 9 new item-type graders.
type answerOption struct {
	ID       string  `json:"id"`
	Text     string  `json:"text"`
	Comments string  `json:"comments"`
	Weight   float64 `json:"weight"`
	// Margin is the numerical tolerance for numerical_question / formula items.
	// Empty string preserves legacy string-equality behavior. A bare number is
	// absolute tolerance ("0.5"); a trailing "%" is percentage of the correct
	// value (e.g. "5%" of 200 = ±10).
	Margin string `json:"margin"`
	// BlankID identifies which blank an option belongs to (multiple_dropdown).
	BlankID string `json:"blank_id"`
	// Coordinates for hot_spot accepted rectangles.
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"w"`
	Height float64 `json:"h"`
	// RightID is the matching-pair right-hand option ID.
	RightID string `json:"right_id"`
	// Left is the matching-pair left-hand label (human readable).
	Left string `json:"left"`
}

// QuizCompletedCallback is invoked asynchronously after a quiz submission
// transitions to "complete" or "pending_review" via CompleteSubmission.
// Mirrors SubmissionGradedCallback: callbacks receive a fresh context and
// must be self-contained (no reliance on the request's context.Cancel).
// Used by the gamification engine to fire `verb=completed, object_type=Quiz`
// rules without coupling the quiz service to the rule engine.
type QuizCompletedCallback func(ctx context.Context, submissionID uint)

type QuizService struct {
	quizRepo             repository.QuizRepository
	questionRepo         repository.QuizQuestionRepository
	submissionRepo       repository.QuizSubmissionRepository
	answerRepo           repository.QuizSubmissionAnswerRepository
	groupRepo            repository.QuizQuestionGroupRepository
	bankEntryRepo        repository.QuestionBankEntryRepository
	accommodationService *AccommodationService

	// onCompletedCallbacks fire (in goroutines) after a successful
	// CompleteSubmission. Registered via OnCompleted; never invoked in
	// tests unless explicitly wired.
	onCompletedCallbacks []QuizCompletedCallback
}

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

func NewQuizService(
	quizRepo repository.QuizRepository,
	questionRepo repository.QuizQuestionRepository,
	submissionRepo repository.QuizSubmissionRepository,
	answerRepo repository.QuizSubmissionAnswerRepository,
	opts ...func(*QuizService),
) *QuizService {
	s := &QuizService{
		quizRepo:       quizRepo,
		questionRepo:   questionRepo,
		submissionRepo: submissionRepo,
		answerRepo:     answerRepo,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func WithQuestionGroupRepo(repo repository.QuizQuestionGroupRepository) func(*QuizService) {
	return func(s *QuizService) {
		s.groupRepo = repo
	}
}

func WithBankEntryRepo(repo repository.QuestionBankEntryRepository) func(*QuizService) {
	return func(s *QuizService) {
		s.bankEntryRepo = repo
	}
}

func WithAccommodationService(svc *AccommodationService) func(*QuizService) {
	return func(s *QuizService) {
		s.accommodationService = svc
	}
}

// ---------- Quiz Question Methods ----------

func (s *QuizService) CreateQuestion(ctx context.Context, question *models.QuizQuestion) error {
	if question.QuestionText == "" {
		return errors.New("question_text is required")
	}
	if question.QuestionType == "" {
		return errors.New("question_type is required")
	}

	validTypes := map[string]bool{
		"multiple_choice":         true,
		"true_false":              true,
		"short_answer":            true,
		"essay":                   true,
		"matching":                true,
		"fill_in_multiple_blanks": true,
		"numerical_question":      true,
		// Wave A1: 9 new item types added to the auto-grader.
		"multiple_answer":    true,
		"multiple_dropdown":  true,
		"fill_in_the_blank":  true,
		"formula":            true,
		"file_upload":        true,
		"ordering":           true,
		"categorization":     true,
		"hot_spot":           true,
		"text_only":          true,
	}
	if !validTypes[question.QuestionType] {
		return errors.New("invalid question_type")
	}

	if question.WorkflowState == "" {
		question.WorkflowState = "active"
	}

	if question.PointsPossible == nil {
		defaultPoints := 1.0
		question.PointsPossible = &defaultPoints
	}

	return s.questionRepo.Create(ctx, question)
}

func (s *QuizService) GetQuestion(ctx context.Context, id uint) (*models.QuizQuestion, error) {
	return s.questionRepo.FindByID(ctx, id)
}

func (s *QuizService) UpdateQuestion(ctx context.Context, question *models.QuizQuestion) error {
	return s.questionRepo.Update(ctx, question)
}

func (s *QuizService) DeleteQuestion(ctx context.Context, id uint) error {
	return s.questionRepo.Delete(ctx, id)
}

func (s *QuizService) ListQuestions(ctx context.Context, quizID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.QuizQuestion], error) {
	return s.questionRepo.ListByQuizID(ctx, quizID, params)
}

// ---------- Quiz Question Group Methods ----------

func (s *QuizService) CreateQuestionGroup(ctx context.Context, group *models.QuizQuestionGroup) error {
	if s.groupRepo == nil {
		return errors.New("question group repository not configured")
	}
	if group.PickCount < 1 {
		group.PickCount = 1
	}
	return s.groupRepo.Create(ctx, group)
}

func (s *QuizService) GetQuestionGroup(ctx context.Context, id uint) (*models.QuizQuestionGroup, error) {
	if s.groupRepo == nil {
		return nil, errors.New("question group repository not configured")
	}
	return s.groupRepo.FindByID(ctx, id)
}

func (s *QuizService) UpdateQuestionGroup(ctx context.Context, group *models.QuizQuestionGroup) error {
	if s.groupRepo == nil {
		return errors.New("question group repository not configured")
	}
	return s.groupRepo.Update(ctx, group)
}

func (s *QuizService) DeleteQuestionGroup(ctx context.Context, id uint) error {
	if s.groupRepo == nil {
		return errors.New("question group repository not configured")
	}
	return s.groupRepo.Delete(ctx, id)
}

func (s *QuizService) ListQuestionGroups(ctx context.Context, quizID uint) ([]models.QuizQuestionGroup, error) {
	if s.groupRepo == nil {
		return nil, errors.New("question group repository not configured")
	}
	return s.groupRepo.ListByQuizID(ctx, quizID)
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

// generateValidationToken creates a cryptographically random hex token.
func generateValidationToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

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

// ---------- Statistics Methods ----------

// GetQuiz returns a quiz by ID.
func (s *QuizService) GetQuiz(ctx context.Context, quizID uint) (*models.Quiz, error) {
	return s.quizRepo.FindByID(ctx, quizID, 0)
}

// ListAllQuestions returns all active questions for a quiz (no pagination).
func (s *QuizService) ListAllQuestions(ctx context.Context, quizID uint) ([]models.QuizQuestion, error) {
	result, err := s.questionRepo.ListByQuizID(ctx, quizID, repository.PaginationParams{Page: 1, PerPage: 10000})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// ListAllCompletedSubmissions returns all completed/pending_review submissions for a quiz.
func (s *QuizService) ListAllCompletedSubmissions(ctx context.Context, quizID uint) ([]models.QuizSubmission, error) {
	return s.submissionRepo.ListCompletedByQuizID(ctx, quizID)
}

// ListAnswersBySubmissionIDs returns all answers for the given submission IDs.
func (s *QuizService) ListAnswersBySubmissionIDs(ctx context.Context, submissionIDs []uint) ([]models.QuizSubmissionAnswer, error) {
	return s.answerRepo.ListBySubmissionIDs(ctx, submissionIDs)
}

// ---------- Auto-Grading Logic ----------

// autoGrade evaluates a student's answer against the question's correct answer(s).
// Returns (points, correct, gradable). gradable is false for types that require manual review.
//
// Wave A1 cutover: matching, fill_in_multiple_blanks, and numerical_question
// (with margin) are now auto-graded. Pre-Wave-A1 submissions for these types
// that were already routed to pending_review remain untouched — only newly-
// completed submissions exercise the new code paths. The graded_via column
// on quiz_submission_answer (migration 000014) provides the audit trail.
func (s *QuizService) autoGrade(question *models.QuizQuestion, submittedAnswer string) (float64, bool, bool) {
	qType := question.QuestionType
	pointsPossible := 1.0
	if question.PointsPossible != nil {
		pointsPossible = *question.PointsPossible
	}

	switch qType {
	case "multiple_choice", "true_false":
		return s.gradeMultipleChoice(question, submittedAnswer, pointsPossible)
	case "short_answer":
		return s.gradeShortAnswer(question, submittedAnswer, pointsPossible)
	case "numerical_question":
		return s.gradeNumerical(question, submittedAnswer, pointsPossible)
	case "essay":
		// Essays cannot be auto-graded
		return 0, false, false
	case "matching":
		// Wave A1 (bug 2B fix): per-pair scoring. Was previously routed to
		// pending_review; legacy already-pending rows stay there because this
		// path only runs on newly-completed submissions.
		return s.gradeMatching(question, submittedAnswer, pointsPossible)
	case "fill_in_multiple_blanks":
		// Wave A1 (bug 2B fix): per-blank case-insensitive scoring.
		return s.gradeFillInMultipleBlanks(question, submittedAnswer, pointsPossible)

	// ---- Wave A1: 9 new item types ----
	case "multiple_answer":
		return s.gradeMultipleAnswer(question, submittedAnswer, pointsPossible)
	case "multiple_dropdown":
		return s.gradeMultipleDropdown(question, submittedAnswer, pointsPossible)
	case "fill_in_the_blank":
		return s.gradeFillInTheBlank(question, submittedAnswer, pointsPossible)
	case "formula":
		return s.gradeFormula(question, submittedAnswer, pointsPossible)
	case "file_upload":
		// File uploads are never auto-graded; an instructor must review.
		return 0, false, false
	case "ordering":
		return s.gradeOrdering(question, submittedAnswer, pointsPossible)
	case "categorization":
		return s.gradeCategorization(question, submittedAnswer, pointsPossible)
	case "hot_spot":
		return s.gradeHotSpot(question, submittedAnswer, pointsPossible)
	case "text_only":
		// Informational items contribute 0 points but are considered
		// "gradable" (i.e. they never block a submission with pending_review).
		return 0, false, true

	default:
		return 0, false, false
	}
}

// gradeMultipleChoice checks if the submitted answer ID matches an answer with weight > 0.
func (s *QuizService) gradeMultipleChoice(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var options []answerOption
	if err := json.Unmarshal([]byte(question.Answers), &options); err != nil {
		return 0, false, false
	}

	submittedAnswer = strings.TrimSpace(submittedAnswer)

	for _, opt := range options {
		if opt.ID == submittedAnswer && opt.Weight > 0 {
			// Partial credit: score = pointsPossible * (weight / 100)
			score := pointsPossible * (opt.Weight / 100.0)
			score = math.Round(score*100) / 100
			return score, true, true
		}
	}

	return 0, false, true
}

// gradeShortAnswer checks if the submitted text matches any correct answer (case-insensitive).
func (s *QuizService) gradeShortAnswer(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var options []answerOption
	if err := json.Unmarshal([]byte(question.Answers), &options); err != nil {
		return 0, false, false
	}

	submittedAnswer = strings.TrimSpace(strings.ToLower(submittedAnswer))

	for _, opt := range options {
		if opt.Weight > 0 && strings.TrimSpace(strings.ToLower(opt.Text)) == submittedAnswer {
			score := pointsPossible * (opt.Weight / 100.0)
			score = math.Round(score*100) / 100
			return score, true, true
		}
	}

	return 0, false, true
}

// gradeNumerical checks if the submitted number matches a correct numerical answer.
// The Answers JSON for numerical questions has the format:
// [{"id":"a1","text":"42","weight":100,"margin":"0.5"}]
//
// Wave A1 (bug 1B fix): the `margin` field is now honored. Two modes:
//   - Empty margin: legacy string-equality match on opt.Text (backwards-compat
//     with quizzes authored before margin support existed).
//   - Non-empty margin: numeric parse + tolerance band. Margin is absolute
//     ("0.5" → ±0.5) or percentage of the correct value ("5%" → ±5% of opt.Text).
//
// Percent semantics: "5%" applied to a correct answer of 200 yields a band of
// ±10 (200 * 0.05). Sign-agnostic — `math.Abs` is used on the correct value.
func (s *QuizService) gradeNumerical(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var options []answerOption
	if err := json.Unmarshal([]byte(question.Answers), &options); err != nil {
		return 0, false, false
	}

	submittedAnswer = strings.TrimSpace(submittedAnswer)

	for _, opt := range options {
		if opt.Weight <= 0 {
			continue
		}
		if numericalMatch(submittedAnswer, opt) {
			score := pointsPossible * (opt.Weight / 100.0)
			score = math.Round(score*100) / 100
			return score, true, true
		}
	}

	return 0, false, true
}

// numericalMatch returns true if submitted answers the given option, respecting
// the option's optional margin tolerance. Used by both gradeNumerical and
// gradeFormula (Wave A1).
func numericalMatch(submitted string, opt answerOption) bool {
	optText := strings.TrimSpace(opt.Text)
	margin := strings.TrimSpace(opt.Margin)

	// Legacy path: no margin → exact string equality. Preserves backwards
	// compat for quizzes authored before margin support existed.
	if margin == "" {
		return optText == submitted
	}

	userVal, err := strconv.ParseFloat(submitted, 64)
	if err != nil {
		return false
	}
	correctVal, err := strconv.ParseFloat(optText, 64)
	if err != nil {
		return false
	}

	tolerance, ok := parseMargin(margin, correctVal)
	if !ok {
		return false
	}
	return math.Abs(userVal-correctVal) <= tolerance
}

// parseMargin parses a margin string into an absolute tolerance value.
// "0.5" → 0.5. "5%" → 5% of |correctVal|. Returns (tolerance, ok).
func parseMargin(margin string, correctVal float64) (float64, bool) {
	margin = strings.TrimSpace(margin)
	if strings.HasSuffix(margin, "%") {
		pct, err := strconv.ParseFloat(strings.TrimSuffix(margin, "%"), 64)
		if err != nil {
			return 0, false
		}
		return math.Abs(correctVal) * (pct / 100.0), true
	}
	tol, err := strconv.ParseFloat(margin, 64)
	if err != nil {
		return 0, false
	}
	return math.Abs(tol), true
}

// ---------- Wave A1: New auto-graders ----------

// gradeMultipleAnswer scores a multiple_answer (checkbox) question.
// Submission JSON is an array of selected option IDs. Each correct option ID
// selected awards +(points/|correct|); each incorrect ID selected deducts
// (points/|correct|). Final score floors at 0 (no negative points). Correct
// flag is true only when the selected set exactly equals the correct set.
func (s *QuizService) gradeMultipleAnswer(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var options []answerOption
	if err := json.Unmarshal([]byte(question.Answers), &options); err != nil {
		return 0, false, false
	}

	var selected []string
	submittedAnswer = strings.TrimSpace(submittedAnswer)
	if submittedAnswer == "" {
		// Empty array submission is treated as "no selections"; still gradable.
		selected = nil
	} else if err := json.Unmarshal([]byte(submittedAnswer), &selected); err != nil {
		return 0, false, false
	}

	correctIDs := map[string]bool{}
	knownIDs := map[string]bool{}
	for _, opt := range options {
		knownIDs[opt.ID] = true
		if opt.Weight > 0 {
			correctIDs[opt.ID] = true
		}
	}

	if len(correctIDs) == 0 {
		return 0, false, true
	}

	perOption := pointsPossible / float64(len(correctIDs))

	score := 0.0
	seen := map[string]bool{}
	exact := true
	for _, sel := range selected {
		if seen[sel] {
			continue
		}
		seen[sel] = true
		if correctIDs[sel] {
			score += perOption
		} else if knownIDs[sel] {
			score -= perOption
			exact = false
		} else {
			// Unknown option ID: treat as wrong selection.
			score -= perOption
			exact = false
		}
	}
	if score < 0 {
		score = 0
	}
	score = math.Round(score*100) / 100

	// Exact match only when every correct option was selected and nothing else.
	for id := range correctIDs {
		if !seen[id] {
			exact = false
			break
		}
	}
	return score, exact, true
}

// gradeMultipleDropdown scores a multiple_dropdown question. Submission JSON:
// {"blank_id": "option_id"}. Each blank's selected option ID is matched
// against options where opt.BlankID == blank and opt.Weight > 0. Score per
// blank is pointsPossible/|blanks|; partial credit allowed.
func (s *QuizService) gradeMultipleDropdown(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var options []answerOption
	if err := json.Unmarshal([]byte(question.Answers), &options); err != nil {
		return 0, false, false
	}

	var submitted map[string]string
	if strings.TrimSpace(submittedAnswer) == "" {
		submitted = map[string]string{}
	} else if err := json.Unmarshal([]byte(submittedAnswer), &submitted); err != nil {
		return 0, false, false
	}

	// Group correct option IDs by blank.
	correctByBlank := map[string]map[string]bool{}
	for _, opt := range options {
		if opt.BlankID == "" {
			continue
		}
		if _, ok := correctByBlank[opt.BlankID]; !ok {
			correctByBlank[opt.BlankID] = map[string]bool{}
		}
		if opt.Weight > 0 {
			correctByBlank[opt.BlankID][opt.ID] = true
		}
	}

	blankCount := len(correctByBlank)
	if blankCount == 0 {
		return 0, false, true
	}
	perBlank := pointsPossible / float64(blankCount)

	score := 0.0
	correctCount := 0
	for blank, accepted := range correctByBlank {
		if accepted[submitted[blank]] {
			score += perBlank
			correctCount++
		}
	}
	score = math.Round(score*100) / 100
	return score, correctCount == blankCount, true
}

// gradeFillInTheBlank scores a single-blank fill_in_the_blank. Like
// short_answer, but partial-credit via the per-option Weight field. The
// submission is the user's typed text; it is matched case-insensitively
// (TrimSpace + ToLower) against each option's Text.
func (s *QuizService) gradeFillInTheBlank(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var options []answerOption
	if err := json.Unmarshal([]byte(question.Answers), &options); err != nil {
		return 0, false, false
	}

	submittedAnswer = strings.TrimSpace(strings.ToLower(submittedAnswer))
	for _, opt := range options {
		if opt.Weight > 0 && strings.TrimSpace(strings.ToLower(opt.Text)) == submittedAnswer {
			score := pointsPossible * (opt.Weight / 100.0)
			score = math.Round(score*100) / 100
			return score, true, true
		}
	}
	return 0, false, true
}

// gradeFormula scores a formula item. Re-uses the numerical-tolerance logic.
// Submission is the user-computed value; each option carries the expected
// value (Text) and optional Margin (absolute or percent).
func (s *QuizService) gradeFormula(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var options []answerOption
	if err := json.Unmarshal([]byte(question.Answers), &options); err != nil {
		return 0, false, false
	}

	submittedAnswer = strings.TrimSpace(submittedAnswer)
	for _, opt := range options {
		if opt.Weight <= 0 {
			continue
		}
		if numericalMatch(submittedAnswer, opt) {
			score := pointsPossible * (opt.Weight / 100.0)
			score = math.Round(score*100) / 100
			return score, true, true
		}
	}
	return 0, false, true
}

// gradeOrdering scores an ordering item. Submission JSON: ["id1","id2",…]
// representing the user's order. Option list defines the canonical order
// (by Position-like array index of options where Weight > 0). Score is
// pointsPossible × (correctPositions / totalPositions).
func (s *QuizService) gradeOrdering(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var options []answerOption
	if err := json.Unmarshal([]byte(question.Answers), &options); err != nil {
		return 0, false, false
	}

	// Canonical order = order of options as authored (Weight > 0 only).
	var canonical []string
	for _, opt := range options {
		if opt.Weight > 0 {
			canonical = append(canonical, opt.ID)
		}
	}

	var submitted []string
	if strings.TrimSpace(submittedAnswer) == "" {
		submitted = nil
	} else if err := json.Unmarshal([]byte(submittedAnswer), &submitted); err != nil {
		return 0, false, false
	}

	total := len(canonical)
	if total == 0 {
		return 0, false, true
	}

	correct := 0
	for i, id := range canonical {
		if i < len(submitted) && submitted[i] == id {
			correct++
		}
	}

	score := pointsPossible * float64(correct) / float64(total)
	score = math.Round(score*100) / 100
	return score, correct == total, true
}

// gradeCategorization scores a categorization (bucket-drop) item. Submission
// JSON: {"item_id":"bucket_id"}. The question's Answers JSON encodes the
// correct bucket for each item via opt.ID = item_id and opt.RightID = bucket_id
// (only options with Weight > 0). Score per item is pointsPossible/|items|.
func (s *QuizService) gradeCategorization(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var options []answerOption
	if err := json.Unmarshal([]byte(question.Answers), &options); err != nil {
		return 0, false, false
	}

	correctBucket := map[string]string{}
	for _, opt := range options {
		if opt.Weight > 0 && opt.ID != "" {
			correctBucket[opt.ID] = opt.RightID
		}
	}

	var submitted map[string]string
	if strings.TrimSpace(submittedAnswer) == "" {
		submitted = map[string]string{}
	} else if err := json.Unmarshal([]byte(submittedAnswer), &submitted); err != nil {
		return 0, false, false
	}

	itemCount := len(correctBucket)
	if itemCount == 0 {
		return 0, false, true
	}
	perItem := pointsPossible / float64(itemCount)

	score := 0.0
	correctPlaced := 0
	for item, bucket := range correctBucket {
		if submitted[item] == bucket {
			score += perItem
			correctPlaced++
		}
	}
	score = math.Round(score*100) / 100
	return score, correctPlaced == itemCount, true
}

// gradeHotSpot scores a hot_spot item. Submission JSON: {"x":N,"y":M}.
// Each option (Weight > 0) defines an axis-aligned rectangle via X, Y, Width,
// Height (top-left origin). Binary scoring: the click is correct if it falls
// inside ANY of the accepted rectangles (boundary inclusive).
func (s *QuizService) gradeHotSpot(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var options []answerOption
	if err := json.Unmarshal([]byte(question.Answers), &options); err != nil {
		return 0, false, false
	}

	var click struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}
	if strings.TrimSpace(submittedAnswer) == "" {
		return 0, false, true
	}
	if err := json.Unmarshal([]byte(submittedAnswer), &click); err != nil {
		return 0, false, false
	}

	for _, opt := range options {
		if opt.Weight <= 0 {
			continue
		}
		if click.X >= opt.X && click.X <= opt.X+opt.Width &&
			click.Y >= opt.Y && click.Y <= opt.Y+opt.Height {
			score := math.Round(pointsPossible*100) / 100
			return score, true, true
		}
	}
	return 0, false, true
}

// gradeMatching scores a matching item. Question Answers JSON is an array of
// option objects, each with Left (label) and RightID (the correct right-hand
// option id). Submission JSON is the same shape: [{left, right_id}, …].
// Score = pointsPossible × (correctPairs / totalPairs).
//
// Wave A1 NOTE on backwards compatibility: prior to this commit, matching was
// hard-routed to pending_review (see internal/service/quiz_service_grading_regression_test.go).
// Existing submissions already in pending_review are NOT retroactively
// re-graded; only newly-completed submissions hit this code path. The
// graded_via column added in migration 000014 distinguishes them.
func (s *QuizService) gradeMatching(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var correctPairs []answerOption
	if err := json.Unmarshal([]byte(question.Answers), &correctPairs); err != nil {
		return 0, false, false
	}

	correctByLeft := map[string]string{}
	for _, p := range correctPairs {
		if p.Left != "" {
			correctByLeft[p.Left] = p.RightID
		}
	}

	type submittedPair struct {
		Left    string `json:"left"`
		RightID string `json:"right_id"`
	}
	var submitted []submittedPair
	if strings.TrimSpace(submittedAnswer) == "" {
		submitted = nil
	} else if err := json.Unmarshal([]byte(submittedAnswer), &submitted); err != nil {
		return 0, false, false
	}

	total := len(correctByLeft)
	if total == 0 {
		// No correctly-formed pairs in the question definition — route to
		// manual review rather than silently auto-completing a malformed item.
		return 0, false, false
	}

	correct := 0
	for _, sp := range submitted {
		if want, ok := correctByLeft[sp.Left]; ok && want == sp.RightID {
			correct++
		}
	}

	score := pointsPossible * float64(correct) / float64(total)
	score = math.Round(score*100) / 100
	return score, correct == total, true
}

// gradeFillInMultipleBlanks scores a fill_in_multiple_blanks item.
// Question Answers JSON: {"blank_id": ["accepted1", "accepted2", …]}.
// Submission JSON: {"blank_id": "user_text"}. Per-blank match is
// case-insensitive + TrimSpace'd. Score = pointsPossible × (correct/total).
//
// Wave A1 NOTE: same backwards-compat caveat as gradeMatching — legacy
// pending_review submissions are not retroactively scored.
func (s *QuizService) gradeFillInMultipleBlanks(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var accepted map[string][]string
	if err := json.Unmarshal([]byte(question.Answers), &accepted); err != nil {
		return 0, false, false
	}

	var submitted map[string]string
	if strings.TrimSpace(submittedAnswer) == "" {
		submitted = map[string]string{}
	} else if err := json.Unmarshal([]byte(submittedAnswer), &submitted); err != nil {
		return 0, false, false
	}

	total := len(accepted)
	if total == 0 {
		// No blanks defined — malformed item, route to manual review.
		return 0, false, false
	}

	correct := 0
	for blank, acceptables := range accepted {
		user := strings.TrimSpace(strings.ToLower(submitted[blank]))
		if user == "" {
			continue
		}
		for _, ans := range acceptables {
			if strings.TrimSpace(strings.ToLower(ans)) == user {
				correct++
				break
			}
		}
	}

	score := pointsPossible * float64(correct) / float64(total)
	score = math.Round(score*100) / 100
	return score, correct == total, true
}
