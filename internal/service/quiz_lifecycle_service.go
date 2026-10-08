package service

// Quiz lifecycle (Canvas parity, quiz-engine lane): the pieces that tie a
// quiz attempt to the rest of the course.
//
//   - G041 a graded quiz is backed by an "online_quiz" assignment, and the
//     kept score is written to that assignment's submission through the
//     normal grading path (late policy, post policy, grade-change log).
//   - G044 the kept score follows the quiz scoring policy across attempts.
//   - G046 per-student due/unlock/lock dates come from the linked
//     assignment's overrides (plus accommodations).
//   - G047 per-student extensions (extra attempts / time, manual unlock).
//   - G057 the attempt event log.
//
// Everything here is optional: a QuizService built without
// WithQuizLifecycle behaves exactly as before (tests built on mocks rely
// on that), and production wires every dependency in cmd/server.

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// QuizGradebook is the slice of SubmissionService the quiz engine needs:
// per-student effective dates and writing a quiz score to the linked
// assignment's submission.
type QuizGradebook interface {
	EffectiveDates(ctx context.Context, assignment *models.Assignment, userID uint) (dueAt, unlockAt, lockAt *time.Time)
	RecordQuizScore(ctx context.Context, assignment *models.Assignment, rec QuizScoreRecord) (*models.Submission, error)
	// QuizScorePosted reports whether the student can see their grade on
	// the assignment (automatic post policy, or posted by an instructor).
	QuizScorePosted(ctx context.Context, assignment *models.Assignment, userID uint) bool
}

// QuizScoreRecord is what the quiz engine reports to the gradebook for one
// student: the kept score and when / which attempt it came from.
type QuizScoreRecord struct {
	UserID        uint
	Score         float64
	SubmittedAt   time.Time
	Attempt       int
	PendingReview bool
	// GraderID is the instructor for a manual score change; 0 for an
	// automatic (auto-graded / auto-submitted / regraded) write.
	GraderID uint
}

// QuizLifecycleDeps wires the lifecycle collaborators. Any field may be
// nil; the matching feature is then skipped.
type QuizLifecycleDeps struct {
	Assignments      repository.AssignmentRepository
	AssignmentGroups repository.AssignmentGroupRepository
	Gradebook        QuizGradebook
	Extensions       repository.QuizExtensionRepository
	Events           repository.QuizSubmissionEventRepository
	Regrades         repository.QuizQuestionRegradeRepository
	Overdue          repository.QuizOverdueSubmissionFinder
	Enrollments      repository.EnrollmentRepository
	// Unlinked finds graded quizzes that still need their assignment.
	Unlinked repository.QuizUnlinkedGradedFinder
}

// WithQuizLifecycle enables the quiz lifecycle features.
func WithQuizLifecycle(deps QuizLifecycleDeps) func(*QuizService) {
	return func(s *QuizService) {
		d := deps
		s.lifecycle = &d
	}
}

// ErrQuizNotGraded is returned by gradebook operations on a practice quiz
// or ungraded survey.
var ErrQuizNotGraded = errors.New("quiz is not graded")

// ---------- extensions / effective dates ----------

// quizExtensionFor returns the user's moderation row for the quiz, or an
// empty one.
func (s *QuizService) quizExtensionFor(ctx context.Context, quizID, userID uint) models.QuizExtension {
	if s.lifecycle == nil || s.lifecycle.Extensions == nil {
		return models.QuizExtension{QuizID: quizID, UserID: userID}
	}
	ext, err := s.lifecycle.Extensions.FindByQuizAndUser(ctx, quizID, userID)
	if err != nil || ext == nil {
		return models.QuizExtension{QuizID: quizID, UserID: userID}
	}
	return *ext
}

// linkedAssignment loads the quiz's gradebook assignment (nil when the
// quiz is ungraded or the link is broken). accountID=0: internal read —
// every caller has already loaded the quiz under the caller's tenant, and
// the assignment id comes from that row, never from the request.
func (s *QuizService) linkedAssignment(ctx context.Context, quiz *models.Quiz) *models.Assignment {
	if s.lifecycle == nil || s.lifecycle.Assignments == nil || quiz == nil || quiz.AssignmentID == nil {
		return nil
	}
	a, err := s.lifecycle.Assignments.FindByID(ctx, *quiz.AssignmentID, 0)
	if err != nil || a == nil || a.WorkflowState == models.AssignmentDeleted || a.CourseID != quiz.CourseID {
		return nil
	}
	return a
}

// EffectiveQuizDates returns the due / unlock / lock dates that apply to
// userID: the linked assignment's dates with its section / student
// overrides and extra-day accommodations applied, else the quiz's own.
func (s *QuizService) EffectiveQuizDates(ctx context.Context, quiz *models.Quiz, userID uint) (dueAt, unlockAt, lockAt *time.Time) {
	if quiz == nil {
		return nil, nil, nil
	}
	if s.lifecycle != nil && s.lifecycle.Gradebook != nil {
		if a := s.linkedAssignment(ctx, quiz); a != nil {
			return s.lifecycle.Gradebook.EffectiveDates(ctx, a, userID)
		}
	}
	return quiz.DueAt, quiz.UnlockAt, quiz.LockAt
}

// AttemptsAllowedFor is the user's attempt limit (-1 unlimited): the quiz's
// allowed_attempts plus the user's extra attempts. allowed_attempts <= 0
// means unlimited (the engine's long-standing reading of 0 and -1).
func AttemptsAllowedFor(quiz *models.Quiz, ext models.QuizExtension) int {
	if quiz == nil || quiz.AllowedAttempts <= 0 {
		return -1
	}
	return quiz.AllowedAttempts + ext.ExtraAttempts
}

// AttemptsLeft is how many more attempts the user may start (-1 unlimited).
func AttemptsLeft(quiz *models.Quiz, ext models.QuizExtension, attemptsUsed int) int {
	allowed := AttemptsAllowedFor(quiz, ext)
	if allowed < 0 {
		return -1
	}
	if left := allowed - attemptsUsed; left > 0 {
		return left
	}
	return 0
}

// ---------- kept score (G044) ----------

// KeptScore applies the quiz scoring policy to a user's finished attempts:
// keep_highest (default), keep_latest or keep_average. ok=false when the
// user has no finished attempt.
func KeptScore(policy string, attempts []models.QuizSubmission) (float64, bool) {
	var scores []float64
	latestAttempt := -1
	latest := 0.0
	for i := range attempts {
		a := &attempts[i]
		if !a.IsCompleted() {
			continue
		}
		v := 0.0
		if a.Score != nil {
			v = *a.Score
		}
		scores = append(scores, v)
		if a.Attempt > latestAttempt {
			latestAttempt = a.Attempt
			latest = v
		}
	}
	if len(scores) == 0 {
		return 0, false
	}
	switch policy {
	case models.QuizScoringKeepLatest:
		return latest, true
	case models.QuizScoringKeepAverage:
		sum := 0.0
		for _, v := range scores {
			sum += v
		}
		return roundScore(sum / float64(len(scores))), true
	default:
		best := scores[0]
		for _, v := range scores[1:] {
			if v > best {
				best = v
			}
		}
		return best, true
	}
}

// userAttempts returns every attempt the user made on the quiz.
func (s *QuizService) userAttempts(ctx context.Context, quizID, userID uint) ([]models.QuizSubmission, error) {
	return s.submissionRepo.ListByQuizAndUserIDs(ctx, quizID, []uint{userID})
}

// refreshKeptScore recomputes the user's kept score, stores it on their
// latest finished attempt and pushes it to the gradebook. graderID is the
// instructor behind a manual change (0 for automatic writes).
func (s *QuizService) refreshKeptScore(ctx context.Context, quiz *models.Quiz, userID, graderID uint) {
	if s.lifecycle == nil || quiz == nil {
		return
	}
	attempts, err := s.userAttempts(ctx, quiz.ID, userID)
	if err != nil {
		slog.Warn("quiz kept score: list attempts", "quiz_id", quiz.ID, "user_id", userID, "err", err)
		return
	}
	kept, ok := KeptScore(quiz.ScoringPolicy, attempts)
	if !ok {
		return
	}
	var latest *models.QuizSubmission
	for i := range attempts {
		if attempts[i].IsCompleted() && (latest == nil || attempts[i].Attempt > latest.Attempt) {
			latest = &attempts[i]
		}
	}
	if latest == nil {
		return
	}
	if latest.KeptScore == nil || *latest.KeptScore != kept {
		latest.KeptScore = &kept
		if err := s.submissionRepo.Update(ctx, latest); err != nil {
			slog.Warn("quiz kept score: save", "quiz_submission_id", latest.ID, "err", err)
		}
	}
	s.pushGradebookScore(ctx, quiz, latest, kept, graderID)
}

// pushGradebookScore writes the kept score to the linked assignment.
func (s *QuizService) pushGradebookScore(ctx context.Context, quiz *models.Quiz, latest *models.QuizSubmission, kept float64, graderID uint) {
	if s.lifecycle == nil || s.lifecycle.Gradebook == nil || !quiz.IsGraded() {
		return
	}
	a := s.linkedAssignment(ctx, quiz)
	if a == nil {
		if quiz.AssignmentID != nil {
			return // link exists but the assignment is gone; never recurse
		}
		// A graded quiz copied or imported without its assignment gets
		// one on first use; SyncQuizAssignment then pushes every score.
		if _, err := s.SyncQuizAssignment(ctx, quiz, nil); err != nil {
			slog.Warn("quiz gradebook: create assignment", "quiz_id", quiz.ID, "err", err)
		}
		return
	}
	submittedAt := time.Now()
	if latest.FinishedAt != nil {
		submittedAt = *latest.FinishedAt
	}
	gbSub, err := s.lifecycle.Gradebook.RecordQuizScore(ctx, a, QuizScoreRecord{
		UserID:        latest.UserID,
		Score:         kept,
		SubmittedAt:   submittedAt,
		Attempt:       latest.Attempt,
		PendingReview: latest.WorkflowState == models.QuizSubmissionPendingReview,
		GraderID:      graderID,
	})
	if err != nil {
		slog.Warn("quiz gradebook: record score", "quiz_id", quiz.ID, "user_id", latest.UserID, "err", err)
		return
	}
	// Link the attempt to the gradebook submission it fed (Canvas
	// quiz_submissions.submission_id).
	if gbSub != nil && gbSub.ID != 0 && (latest.SubmissionID == nil || *latest.SubmissionID != gbSub.ID) {
		id := gbSub.ID
		latest.SubmissionID = &id
		if err := s.submissionRepo.Update(ctx, latest); err != nil {
			slog.Warn("quiz gradebook: link attempt", "quiz_submission_id", latest.ID, "err", err)
		}
	}
}

// resyncAllGradebookScores pushes every student's kept score (used when a
// graded quiz gets its assignment, or a regrade touches many attempts).
func (s *QuizService) resyncAllGradebookScores(ctx context.Context, quiz *models.Quiz) {
	subs, err := s.submissionRepo.ListCompletedByQuizID(ctx, quiz.ID)
	if err != nil {
		return
	}
	seen := map[uint]bool{}
	for _, sub := range subs {
		if seen[sub.UserID] {
			continue
		}
		seen[sub.UserID] = true
		s.refreshKeptScore(ctx, quiz, sub.UserID, 0)
	}
}

// ---------- assignment link (G041 / G046) ----------

// QuizAssignmentOptions carries editor-only assignment settings.
type QuizAssignmentOptions struct {
	// AssignmentGroupID moves the assignment into this group (already
	// verified to be in the quiz's course). nil leaves it unchanged.
	AssignmentGroupID *uint
}

// SyncQuizAssignment keeps the quiz's online_quiz assignment in step with
// the quiz: a graded, live quiz has one (created on demand, mirroring the
// title, description, dates, points and publish state); a practice quiz,
// ungraded survey or deleted quiz has none (its assignment is deleted).
// Returns the linked assignment, or nil.
func (s *QuizService) SyncQuizAssignment(ctx context.Context, quiz *models.Quiz, opts *QuizAssignmentOptions) (*models.Assignment, error) {
	if s.lifecycle == nil || s.lifecycle.Assignments == nil || quiz == nil {
		return nil, nil
	}
	repo := s.lifecycle.Assignments
	live := quiz.IsGraded() && quiz.WorkflowState != "deleted"

	if !live {
		if quiz.AssignmentID != nil {
			if err := repo.Delete(ctx, *quiz.AssignmentID); err != nil {
				return nil, err
			}
			quiz.AssignmentID = nil
			if err := s.quizRepo.Update(ctx, quiz); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}

	a := s.linkedAssignment(ctx, quiz)
	created, adopted := false, false
	if a == nil {
		if orphan := s.orphanQuizAssignment(ctx, quiz); orphan != nil {
			a = orphan
			adopted = true
		} else {
			a = &models.Assignment{CourseID: quiz.CourseID, GradingType: "points"}
			created = true
			if opts == nil || opts.AssignmentGroupID == nil {
				a.AssignmentGroupID = s.defaultAssignmentGroup(ctx, quiz.CourseID)
			}
		}
	}
	a.Name = quiz.Title
	a.Description = quiz.Description
	a.DueAt, a.UnlockAt, a.LockAt = quiz.DueAt, quiz.UnlockAt, quiz.LockAt
	a.PointsPossible = quiz.PointsPossible
	a.SubmissionTypes = "online_quiz"
	if opts != nil && opts.AssignmentGroupID != nil {
		gid := *opts.AssignmentGroupID
		a.AssignmentGroupID = &gid
	}
	if quiz.Published || quiz.WorkflowState == "published" {
		a.WorkflowState = models.AssignmentPublished
		a.Published = true
	} else {
		a.WorkflowState = models.AssignmentUnpublished
		a.Published = false
	}

	if created || adopted {
		if created {
			if err := repo.Create(ctx, a); err != nil {
				return nil, err
			}
		} else if err := repo.Update(ctx, a); err != nil {
			return nil, err
		}
		id := a.ID
		quiz.AssignmentID = &id
		if err := s.quizRepo.Update(ctx, quiz); err != nil {
			return nil, err
		}
		// Students who already took the quiz get their scores now.
		s.resyncAllGradebookScores(ctx, quiz)
		return a, nil
	}
	if err := repo.Update(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// orphanQuizAssignment finds a live online_quiz assignment in the quiz's
// course, titled like the quiz, that no quiz links to: the gradebook side
// of a quiz that a course copy or blueprint sync brought over without its
// link. Reusing it keeps one gradebook column per quiz (and any overrides
// or grades already on it).
func (s *QuizService) orphanQuizAssignment(ctx context.Context, quiz *models.Quiz) *models.Assignment {
	all := repository.PaginationParams{Page: 1, PerPage: 10000}
	assignments, err := s.lifecycle.Assignments.ListByCourseID(ctx, quiz.CourseID, all)
	if err != nil || assignments == nil {
		return nil
	}
	var candidates []models.Assignment
	for _, a := range assignments.Items {
		if a.CourseID == quiz.CourseID && a.Name == quiz.Title &&
			a.WorkflowState != models.AssignmentDeleted && hasSubmissionType(a.SubmissionTypes, quizSubmissionType) {
			candidates = append(candidates, a)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	quizzes, err := s.quizRepo.ListByCourseID(ctx, quiz.CourseID, all)
	if err != nil || quizzes == nil {
		return nil
	}
	linked := map[uint]bool{}
	for _, q := range quizzes.Items {
		if q.ID != quiz.ID && q.AssignmentID != nil {
			linked[*q.AssignmentID] = true
		}
	}
	for i := range candidates {
		if !linked[candidates[i].ID] {
			a := candidates[i]
			return &a
		}
	}
	return nil
}

// hasSubmissionType reports whether a comma-separated submission_types
// value contains want.
func hasSubmissionType(types, want string) bool {
	for _, t := range strings.Split(types, ",") {
		if strings.TrimSpace(t) == want {
			return true
		}
	}
	return false
}

// LinkGradedQuizzes gives every live graded quiz without a gradebook
// assignment its online_quiz assignment (and pushes the scores of
// students who already took it). Quizzes from before the gradebook link,
// and quizzes copied or imported without one, are picked up here rather
// than waiting for their next edit or attempt. Returns how many were
// linked.
func (s *QuizService) LinkGradedQuizzes(ctx context.Context) (int, error) {
	if s.lifecycle == nil || s.lifecycle.Unlinked == nil || s.lifecycle.Assignments == nil {
		return 0, nil
	}
	linked := 0
	for batch := 0; batch < 50; batch++ {
		quizzes, err := s.lifecycle.Unlinked.ListGradedWithoutAssignment(ctx, 100)
		if err != nil {
			return linked, err
		}
		progressed := 0
		for i := range quizzes {
			q := quizzes[i]
			a, err := s.SyncQuizAssignment(ctx, &q, nil)
			if err != nil || a == nil {
				slog.Warn("quiz gradebook: link graded quiz", "quiz_id", q.ID, "err", err)
				continue
			}
			linked++
			progressed++
		}
		// Stop when nothing is left, or only quizzes that keep failing.
		if progressed == 0 {
			return linked, nil
		}
	}
	return linked, nil
}

// defaultAssignmentGroup is the course's first assignment group (Canvas
// puts a new quiz's assignment there), or nil.
func (s *QuizService) defaultAssignmentGroup(ctx context.Context, courseID uint) *uint {
	if s.lifecycle == nil || s.lifecycle.AssignmentGroups == nil {
		return nil
	}
	res, err := s.lifecycle.AssignmentGroups.ListByCourseID(ctx, courseID, repository.PaginationParams{Page: 1, PerPage: 1})
	if err != nil || res == nil || len(res.Items) == 0 {
		return nil
	}
	id := res.Items[0].ID
	return &id
}

// LinkedAssignment exposes the quiz's live gradebook assignment (nil when
// none) for the quiz editor.
func (s *QuizService) LinkedAssignment(ctx context.Context, quiz *models.Quiz) *models.Assignment {
	return s.linkedAssignment(ctx, quiz)
}

// AssignmentGroupInCourse verifies a group id names a live assignment
// group of the course, under the caller's tenant.
func (s *QuizService) AssignmentGroupInCourse(ctx context.Context, groupID, courseID, accountID uint) bool {
	if s.lifecycle == nil || s.lifecycle.AssignmentGroups == nil {
		return false
	}
	g, err := s.lifecycle.AssignmentGroups.FindByID(ctx, groupID, accountID)
	return err == nil && g != nil && g.CourseID == courseID && g.WorkflowState != "deleted"
}

// QuizForAssignment finds the quiz backing an online_quiz assignment in
// the course (nil when there is none).
func (s *QuizService) QuizForAssignment(ctx context.Context, courseID, assignmentID uint) (*models.Quiz, error) {
	res, err := s.quizRepo.ListByCourseID(ctx, courseID, repository.PaginationParams{Page: 1, PerPage: 10000})
	if err != nil {
		return nil, err
	}
	for i := range res.Items {
		q := res.Items[i]
		if q.AssignmentID != nil && *q.AssignmentID == assignmentID {
			return &q, nil
		}
	}
	return nil, nil
}

// ---------- event log (G057) ----------

// recordQuizEvent appends a server-side event to an attempt's log.
// Failures are logged, never surfaced: the log must not block taking or
// grading a quiz.
func (s *QuizService) recordQuizEvent(ctx context.Context, sub *models.QuizSubmission, eventType string, data map[string]interface{}) {
	if s.lifecycle == nil || s.lifecycle.Events == nil || sub == nil || sub.ID == 0 {
		return
	}
	payload := ""
	if len(data) > 0 {
		if b, err := json.Marshal(data); err == nil {
			payload = string(b)
		}
	}
	ev := models.QuizSubmissionEvent{
		QuizSubmissionID: sub.ID,
		Attempt:          sub.Attempt,
		EventType:        eventType,
		EventData:        payload,
		Source:           "server",
	}
	if err := s.lifecycle.Events.BulkCreate(ctx, []models.QuizSubmissionEvent{ev}); err != nil {
		slog.Warn("quiz event log: write", "quiz_submission_id", sub.ID, "event", eventType, "err", err)
	}
}

// clientQuizEventTypes are the events the take page may report.
var clientQuizEventTypes = map[string]bool{
	models.QuizEventPageBlurred:        true,
	models.QuizEventPageFocused:        true,
	models.QuizEventQuestionViewed:     true,
	models.QuizEventQuestionFlagged:    true,
	models.QuizEventSessionResumed:     true,
	models.QuizEventQuestionUnanswered: true,
}

// ClientQuizEvent is one event reported by the take page.
type ClientQuizEvent struct {
	EventType       string
	EventData       json.RawMessage
	ClientTimestamp *time.Time
}

// Limits on client-reported events.
const (
	maxClientQuizEventsPerBatch = 100
	maxClientQuizEventDataBytes = 2048
)

// ErrInvalidQuizEvent is returned for a malformed client event batch.
var ErrInvalidQuizEvent = errors.New("invalid quiz submission event")

// RecordClientQuizEvents stores events reported by the student's take page
// for an attempt that is still in progress.
func (s *QuizService) RecordClientQuizEvents(ctx context.Context, sub *models.QuizSubmission, events []ClientQuizEvent) (int, error) {
	if s.lifecycle == nil || s.lifecycle.Events == nil {
		return 0, errors.New("quiz event log is not available")
	}
	if sub.WorkflowState != models.QuizSubmissionUntaken {
		return 0, errors.New("quiz submission is not in progress")
	}
	if len(events) == 0 {
		return 0, nil
	}
	if len(events) > maxClientQuizEventsPerBatch {
		return 0, ErrInvalidQuizEvent
	}
	rows := make([]models.QuizSubmissionEvent, 0, len(events))
	for _, e := range events {
		if !clientQuizEventTypes[e.EventType] {
			return 0, ErrInvalidQuizEvent
		}
		data := ""
		if raw := e.EventData; len(raw) > 0 && string(raw) != "null" {
			if len(raw) > maxClientQuizEventDataBytes {
				return 0, ErrInvalidQuizEvent
			}
			var obj map[string]interface{}
			if err := json.Unmarshal(raw, &obj); err != nil {
				return 0, ErrInvalidQuizEvent
			}
			b, _ := json.Marshal(obj)
			data = string(b)
		}
		rows = append(rows, models.QuizSubmissionEvent{
			QuizSubmissionID: sub.ID,
			Attempt:          sub.Attempt,
			EventType:        e.EventType,
			EventData:        data,
			Source:           "client",
			ClientTimestamp:  e.ClientTimestamp,
		})
	}
	if err := s.lifecycle.Events.BulkCreate(ctx, rows); err != nil {
		return 0, err
	}
	return len(rows), nil
}

// ListQuizEvents returns an attempt's event log, oldest first.
func (s *QuizService) ListQuizEvents(ctx context.Context, submissionID uint) ([]models.QuizSubmissionEvent, error) {
	if s.lifecycle == nil || s.lifecycle.Events == nil {
		return []models.QuizSubmissionEvent{}, nil
	}
	return s.lifecycle.Events.ListBySubmissionID(ctx, submissionID)
}
