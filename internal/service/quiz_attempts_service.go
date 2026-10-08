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
	liveGroups := make(map[uint]bool, len(groups))
	for _, g := range groups {
		liveGroups[g.ID] = true
	}

	rng := mathrand.New(mathrand.NewSource(time.Now().UnixNano()))

	// poolEntry is either a real quiz question (questionID != 0) or a
	// question-bank entry that still has to be materialized into this quiz
	// (bankEntry != nil). Bank entry IDs live in a DIFFERENT table
	// (question_bank_entries) and must never be mixed into quiz-question
	// IDs — resolving one through questionRepo would serve an unrelated
	// (possibly cross-tenant) quiz_questions row.
	type poolEntry struct {
		questionID uint
		bankEntry  *models.QuestionBankEntry
	}

	var quizCourseID uint
	quizLoaded := false

	for _, group := range groups {
		var pool []poolEntry

		groupQuestions, gqErr := s.questionRepo.ListByGroupID(ctx, group.ID)
		if gqErr != nil {
			groupQuestions = nil
		}
		// Already-materialized copies of bank entries, keyed by entry id.
		materialized := make(map[uint]uint)
		for _, q := range groupQuestions {
			groupedQuestionIDs[q.ID] = true
			if q.QuestionBankEntryID != nil {
				materialized[*q.QuestionBankEntryID] = q.ID
			}
		}

		// Option 1: Group pulls from a linked QuestionBank (same course only).
		if group.QuestionBankID != nil && s.bankEntryRepo != nil {
			bankOK := true
			if s.questionBankRepo != nil {
				if !quizLoaded {
					if quiz, qErr := s.quizRepo.FindByID(ctx, quizID, 0); qErr == nil && quiz != nil {
						quizCourseID = quiz.CourseID
					}
					quizLoaded = true
				}
				bank, bErr := s.questionBankRepo.FindByID(ctx, *group.QuestionBankID)
				bankOK = bErr == nil && bank != nil && bank.CourseID == quizCourseID && bank.WorkflowState != "deleted"
			}
			if bankOK {
				if entries, err := s.bankEntryRepo.ListByBankID(ctx, *group.QuestionBankID); err == nil {
					for i := range entries {
						entry := entries[i]
						if entry.QuestionBankID != *group.QuestionBankID {
							continue
						}
						if qid, ok := materialized[entry.ID]; ok {
							pool = append(pool, poolEntry{questionID: qid})
						} else {
							pool = append(pool, poolEntry{bankEntry: &entry})
						}
					}
				}
			}
		}

		// Option 2: questions directly assigned to the group. Materialized
		// bank copies were already added via option 1 (only while their
		// entry is still in the bank).
		for _, q := range groupQuestions {
			if q.QuestionBankEntryID != nil {
				continue
			}
			pool = append(pool, poolEntry{questionID: q.ID})
		}

		// Randomly pick PickCount questions from the pool
		if len(pool) > 0 {
			rng.Shuffle(len(pool), func(i, j int) {
				pool[i], pool[j] = pool[j], pool[i]
			})

			pickCount := group.PickCount
			if pickCount > len(pool) {
				pickCount = len(pool)
			}
			for _, pe := range pool[:pickCount] {
				if pe.questionID != 0 {
					selectedIDs = append(selectedIDs, pe.questionID)
					continue
				}
				qid, err := s.materializeBankEntry(ctx, quizID, group, pe.bankEntry)
				if err != nil {
					return nil, err
				}
				selectedIDs = append(selectedIDs, qid)
			}
		}
	}

	// Add all ungrouped questions. A question whose group was deleted (a
	// dangling quiz_question_group_id) is treated as ungrouped rather than
	// silently dropped from every attempt. Materialized bank copies are
	// never "ungrouped" — they only enter an attempt through their group.
	for _, q := range allQuestions.Items {
		if groupedQuestionIDs[q.ID] || q.QuestionBankEntryID != nil {
			continue
		}
		if q.QuizQuestionGroupID == nil || *q.QuizQuestionGroupID == 0 || !liveGroups[*q.QuizQuestionGroupID] {
			selectedIDs = append(selectedIDs, q.ID)
		}
	}

	return selectedIDs, nil
}

// materializeBankEntry copies a question-bank entry into the quiz as a
// QuizQuestion owned by the group (QuestionBankEntryID records the
// provenance so later attempts reuse the same row). Returns the quiz
// question id.
func (s *QuizService) materializeBankEntry(ctx context.Context, quizID uint, group models.QuizQuestionGroup, entry *models.QuestionBankEntry) (uint, error) {
	groupID := group.ID
	entryID := entry.ID
	pts := entry.PointsPossible
	q := &models.QuizQuestion{
		QuizID:              quizID,
		QuizQuestionGroupID: &groupID,
		QuestionBankEntryID: &entryID,
		Position:            entry.Position,
		QuestionType:        NormalizeQuizQuestionType(entry.QuestionType),
		QuestionText:        entry.QuestionText,
		PointsPossible:      &pts,
		Answers:             entry.Answers,
		WorkflowState:       "active",
	}
	if q.QuestionText == "" {
		q.QuestionText = entry.QuestionName
	}
	if q.Answers == "" {
		q.Answers = "[]"
	}
	if err := s.questionRepo.Create(ctx, q); err != nil {
		return 0, err
	}
	return q.ID, nil
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
// If timeLimit is provided (in minutes), the EndAt field is set
// accordingly. timeLimit is server-controlled (handler does NOT
// expose it to the request body — see F-046).
//
// SECURITY (F-048): enforces quiz.UnlockAt / LockAt windows. The
// pre-fix path let a student start a quiz before unlock_at (get
// early access to questions) or after lock_at (sneak in late).
//
// SECURITY (F-049): the find-create race is now closed via the
// (quiz_id, user_id, workflow_state='untaken') partial UNIQUE index
// added in migration 000062. A parallel-fire attempt that would
// otherwise create N concurrent untaken submissions trips the
// constraint and the second-writer reload-the-existing path runs.
// The legacy serial path remains for the non-racing case.
func (s *QuizService) StartSubmission(ctx context.Context, quizID, userID uint, timeLimit *int) (*models.QuizSubmission, error) {
	// Load the quiz first so we can validate windows before churning
	// the submissions table.
	quiz, err := s.quizRepo.FindByID(ctx, quizID, 0)
	if err != nil {
		return nil, errors.New("quiz not found")
	}
	// A soft-deleted quiz is gone; it can never be taken.
	if quiz.WorkflowState == "deleted" {
		return nil, errors.New("quiz not found")
	}

	now := time.Now()

	// F-048: unlock_at / lock_at windows.
	if quiz.UnlockAt != nil && now.Before(*quiz.UnlockAt) {
		return nil, errors.New("quiz is not yet available")
	}
	if quiz.LockAt != nil && now.After(*quiz.LockAt) {
		return nil, errors.New("quiz is locked and no longer accepts submissions")
	}

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
	if quiz.AllowedAttempts > 0 && attempt > quiz.AllowedAttempts {
		return nil, errors.New("maximum number of attempts reached")
	}

	token, err := generateValidationToken()
	if err != nil {
		return nil, errors.New("could not generate validation token")
	}

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
		// F-049 race recovery: migration 000062 adds a partial UNIQUE
		// on (quiz_id, user_id) WHERE workflow_state='untaken'. When
		// two requests race to start the same quiz, the loser's
		// Create fails with the UNIQUE violation. Re-run the
		// existing-untaken find so the loser sees the winner's row
		// instead of bubbling a raw GORM error to the handler (which
		// pre-fix surfaced as 400).
		if winner, findErr := s.submissionRepo.FindByQuizAndUser(ctx, quizID, userID); findErr == nil &&
			winner != nil && winner.WorkflowState == "untaken" {
			return winner, nil
		}
		return nil, err
	}

	return submission, nil
}

func (s *QuizService) GetSubmission(ctx context.Context, id uint) (*models.QuizSubmission, error) {
	return s.submissionRepo.FindByID(ctx, id)
}

// GetSubmissionAnswer returns the stored answer for one question of an
// attempt (callers must already have authorized access to the attempt).
func (s *QuizService) GetSubmissionAnswer(ctx context.Context, submissionID, questionID uint) (*models.QuizSubmissionAnswer, error) {
	return s.answerRepo.FindBySubmissionAndQuestion(ctx, submissionID, questionID)
}

// FileUploadQuestionForSubmission loads a question and verifies it is a
// file_upload question that belongs to the attempt.
func (s *QuizService) FileUploadQuestionForSubmission(ctx context.Context, submission *models.QuizSubmission, questionID uint) (*models.QuizQuestion, error) {
	question, err := s.questionRepo.FindByID(ctx, questionID)
	if err != nil || question == nil {
		return nil, ErrQuestionNotInSubmission
	}
	selected, hasSelected := selectedQuestionSet(submission)
	if !questionBelongsToSubmission(submission, question, selected, hasSelected) ||
		NormalizeQuizQuestionType(question.QuestionType) != "file_upload" {
		return nil, ErrQuestionNotInSubmission
	}
	return question, nil
}

// ErrQuestionNotInSubmission is returned when an answer targets a question
// that is not part of the submission's quiz / personalized question set.
var ErrQuestionNotInSubmission = errors.New("question is not part of this quiz submission")

// selectedQuestionSet parses submission.SelectedQuestions. ok=false means
// the submission has no personalized set (every quiz question applies).
func selectedQuestionSet(submission *models.QuizSubmission) (map[uint]bool, bool) {
	if submission.SelectedQuestions == "" {
		return nil, false
	}
	var ids []uint
	if err := json.Unmarshal([]byte(submission.SelectedQuestions), &ids); err != nil {
		return map[uint]bool{}, true
	}
	set := make(map[uint]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set, true
}

// questionBelongsToSubmission reports whether the question is gradable in
// this attempt: it must belong to the submission's quiz and, when the
// attempt has a personalized question set, be one of the selected ones.
func questionBelongsToSubmission(submission *models.QuizSubmission, question *models.QuizQuestion, selected map[uint]bool, hasSelected bool) bool {
	if question == nil || question.QuizID != submission.QuizID {
		return false
	}
	if hasSelected && !selected[question.ID] {
		return false
	}
	return true
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

	// The question must belong to this attempt — otherwise a student could
	// store answers to questions from other quizzes and have them graded
	// into this submission's score.
	question, qErr := s.questionRepo.FindByID(ctx, questionID)
	if qErr != nil || question == nil || question.WorkflowState == "deleted" {
		return nil, ErrQuestionNotInSubmission
	}
	selected, hasSelected := selectedQuestionSet(submission)
	if !questionBelongsToSubmission(submission, question, selected, hasSelected) {
		return nil, ErrQuestionNotInSubmission
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

	selected, hasSelected := selectedQuestionSet(submission)
	groupCache := map[uint]*models.QuizQuestionGroup{}
	seenQuestion := map[uint]bool{}

	for i := range answers {
		question, qErr := s.questionRepo.FindByID(ctx, answers[i].QuestionID)
		if qErr != nil {
			continue
		}
		// Only questions that belong to this attempt count toward the
		// score (B025: stray answers to other quizzes' questions are
		// ignored), and each question is scored at most once.
		if !questionBelongsToSubmission(submission, question, selected, hasSelected) || seenQuestion[question.ID] {
			continue
		}
		seenQuestion[question.ID] = true

		pointsPossible := s.effectiveQuestionPoints(ctx, question, groupCache)
		points, correct, gradable := s.autoGradeWithPoints(question, answers[i].Answer, pointsPossible)

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

	// F-050: cap FinishedAt at submission.EndAt + 5-minute grace when
	// the student delayed the explicit /complete call past their
	// time-limit. The pre-fix path stamped FinishedAt = time.Now()
	// unconditionally, which inflated TimeSpent and the gradebook
	// "submitted at" column on a delayed POST. (F-046 already blocks
	// new ANSWERS past EndAt — this only tightens the recorded
	// timestamps.)
	now := time.Now()
	finishAt := now
	if submission.EndAt != nil && now.After(*submission.EndAt) {
		finishAt = submission.EndAt.Add(5 * time.Minute)
		if now.Before(finishAt) {
			// Still inside grace window — use the actual now.
			finishAt = now
		}
	}
	submission.FinishedAt = &finishAt
	submission.Score = &totalScore
	submission.KeptScore = &totalScore

	if submission.StartedAt != nil {
		spent := int(finishAt.Sub(*submission.StartedAt).Seconds())
		submission.TimeSpent = spent
	}

	if needsReview {
		submission.WorkflowState = "pending_review"
	} else {
		submission.WorkflowState = "complete"
	}

	// B134: two concurrent /complete calls (double click, or the timer
	// firing as the student clicks Submit) both read 'untaken'. The real
	// repository finalizes with a conditional UPDATE … WHERE
	// workflow_state='untaken' so only one caller wins and only the winner
	// fires the completion callbacks.
	if fin, ok := s.submissionRepo.(quizSubmissionFinalizer); ok {
		won, err := fin.FinalizeIfInProgress(ctx, submission)
		if err != nil {
			return nil, err
		}
		if !won {
			return nil, errors.New("quiz submission is not in progress")
		}
	} else if err := s.submissionRepo.Update(ctx, submission); err != nil {
		return nil, err
	}

	// Fire post-completion callbacks (gamification, notifications, etc.)
	// asynchronously. Failures in callbacks must never block completion
	// or surface as errors here.
	s.fireOnCompleted(submission.ID)

	return submission, nil
}

// quizSubmissionFinalizer is implemented by the Postgres repository: it
// writes the terminal state only if the row is still 'untaken' and reports
// whether this caller won.
type quizSubmissionFinalizer interface {
	FinalizeIfInProgress(ctx context.Context, submission *models.QuizSubmission) (bool, error)
}

// effectiveQuestionPoints returns the points a question is worth in an
// attempt: the owning group's points_per_item override when set (Canvas
// question-group "points per question"), else the question's own points.
func (s *QuizService) effectiveQuestionPoints(ctx context.Context, question *models.QuizQuestion, cache map[uint]*models.QuizQuestionGroup) float64 {
	pts := 1.0
	if question.PointsPossible != nil {
		pts = *question.PointsPossible
	}
	if s.groupRepo == nil || question.QuizQuestionGroupID == nil || *question.QuizQuestionGroupID == 0 {
		return pts
	}
	gid := *question.QuizQuestionGroupID
	group, ok := cache[gid]
	if !ok {
		g, err := s.groupRepo.FindByID(ctx, gid)
		if err != nil || g == nil || g.QuizID != question.QuizID {
			g = nil
		}
		cache[gid] = g
		group = g
	}
	if group != nil && group.PointsPerItem != nil {
		return *group.PointsPerItem
	}
	return pts
}

func (s *QuizService) ListSubmissions(ctx context.Context, quizID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.QuizSubmission], error) {
	return s.submissionRepo.ListByQuizID(ctx, quizID, params)
}

// ListSubmissionsForUsers returns every attempt the given users made on the
// quiz (a student's own history, or an observer's linked students). Used
// for non-staff callers, who must never see the course-wide roster.
func (s *QuizService) ListSubmissionsForUsers(ctx context.Context, quizID uint, userIDs []uint) ([]models.QuizSubmission, error) {
	if len(userIDs) == 0 {
		return []models.QuizSubmission{}, nil
	}
	return s.submissionRepo.ListByQuizAndUserIDs(ctx, quizID, userIDs)
}

// ListSubmissionAnswers returns all answers for a completed quiz submission.
// Authorization (owner / course staff / linked observer) is the handler's
// job; this method only refuses to expose an attempt that is still in
// progress to anyone but staff.
func (s *QuizService) ListSubmissionAnswers(ctx context.Context, submissionID uint, viewerIsStaff bool) ([]models.QuizSubmissionAnswer, error) {
	submission, err := s.submissionRepo.FindByID(ctx, submissionID)
	if err != nil {
		return nil, errors.New("quiz submission not found")
	}
	if submission.WorkflowState == "untaken" && !viewerIsStaff {
		return nil, errors.New("quiz submission is still in progress")
	}
	return s.answerRepo.ListBySubmissionID(ctx, submissionID)
}
