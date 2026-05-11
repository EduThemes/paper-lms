package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	mathrand "math/rand"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// answerOption represents one answer choice in a quiz question's Answers JSON field.
type answerOption struct {
	ID       string  `json:"id"`
	Text     string  `json:"text"`
	Comments string  `json:"comments"`
	Weight   float64 `json:"weight"`
}

type QuizService struct {
	quizRepo             repository.QuizRepository
	questionRepo         repository.QuizQuestionRepository
	submissionRepo       repository.QuizSubmissionRepository
	answerRepo           repository.QuizSubmissionAnswerRepository
	groupRepo            repository.QuizQuestionGroupRepository
	bankEntryRepo        repository.QuestionBankEntryRepository
	accommodationService *AccommodationService
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
		"multiple_choice":          true,
		"true_false":               true,
		"short_answer":             true,
		"essay":                    true,
		"matching":                 true,
		"fill_in_multiple_blanks":  true,
		"numerical_question":       true,
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
	quiz, err := s.quizRepo.FindByID(ctx, quizID)
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
			totalScore += points
		} else {
			// Essay and other non-auto-gradable types require manual review
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
	return s.quizRepo.FindByID(ctx, quizID)
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
	case "matching", "fill_in_multiple_blanks":
		// These complex types require manual review for now
		return 0, false, false
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

// gradeNumerical checks if the submitted number matches a correct numerical answer within a margin.
// The Answers JSON for numerical questions has the format:
// [{"id":"a1","text":"42","weight":100,"margin":"0.5"}]
// For simplicity, we do an exact string match on the text field.
func (s *QuizService) gradeNumerical(question *models.QuizQuestion, submittedAnswer string, pointsPossible float64) (float64, bool, bool) {
	var options []answerOption
	if err := json.Unmarshal([]byte(question.Answers), &options); err != nil {
		return 0, false, false
	}

	submittedAnswer = strings.TrimSpace(submittedAnswer)

	for _, opt := range options {
		if opt.Weight > 0 && strings.TrimSpace(opt.Text) == submittedAnswer {
			score := pointsPossible * (opt.Weight / 100.0)
			score = math.Round(score*100) / 100
			return score, true, true
		}
	}

	return 0, false, true
}
