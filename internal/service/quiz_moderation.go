package service

// Moderate quiz (G047) and server-side auto-submit of overdue attempts
// (G048).

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// QuizExtensionInput is a moderation change for one student. nil fields
// keep the current value. ExtendFromNow / ExtendFromEndAt (minutes) also
// extend an attempt that is in progress right now.
type QuizExtensionInput struct {
	UserID           uint
	ExtraAttempts    *int
	ExtraTime        *int
	ManuallyUnlocked *bool
	ExtendFromNow    *int
	ExtendFromEndAt  *int
}

// Limits mirror Canvas (QuizExtension: at most 1000 extra attempts and
// 10080 extra minutes).
const (
	maxQuizExtraAttempts = 1000
	maxQuizExtraMinutes  = 10080
)

// ErrInvalidQuizExtension is returned for out-of-range moderation values.
var ErrInvalidQuizExtension = errors.New("invalid quiz extension")

func boundedNonNegative(v *int, max int) bool {
	return v == nil || (*v >= 0 && *v <= max)
}

// SetQuizExtension saves a student's moderation settings on the quiz and,
// when asked, extends the attempt they have in progress. The caller has
// verified the user is a student of the quiz's course.
func (s *QuizService) SetQuizExtension(ctx context.Context, quiz *models.Quiz, in QuizExtensionInput, actorID uint) (*models.QuizExtension, *models.QuizSubmission, error) {
	if s.lifecycle == nil || s.lifecycle.Extensions == nil {
		return nil, nil, errors.New("quiz moderation is not available")
	}
	if !boundedNonNegative(in.ExtraAttempts, maxQuizExtraAttempts) ||
		!boundedNonNegative(in.ExtraTime, maxQuizExtraMinutes) ||
		(in.ExtendFromNow != nil && (*in.ExtendFromNow <= 0 || *in.ExtendFromNow > maxQuizExtraMinutes)) ||
		(in.ExtendFromEndAt != nil && (*in.ExtendFromEndAt <= 0 || *in.ExtendFromEndAt > maxQuizExtraMinutes)) {
		return nil, nil, ErrInvalidQuizExtension
	}

	ext := s.quizExtensionFor(ctx, quiz.ID, in.UserID)
	if in.ExtraAttempts != nil {
		ext.ExtraAttempts = *in.ExtraAttempts
	}
	if in.ExtraTime != nil {
		ext.ExtraTime = *in.ExtraTime
	}
	if in.ManuallyUnlocked != nil {
		ext.ManuallyUnlocked = *in.ManuallyUnlocked
	}
	ext.QuizID, ext.UserID = quiz.ID, in.UserID
	if err := s.lifecycle.Extensions.Upsert(ctx, &ext); err != nil {
		return nil, nil, err
	}

	if in.ExtendFromNow == nil && in.ExtendFromEndAt == nil {
		return &ext, nil, nil
	}
	latest, err := s.submissionRepo.FindByQuizAndUser(ctx, quiz.ID, in.UserID)
	if err != nil || latest == nil || latest.WorkflowState != models.QuizSubmissionUntaken || latest.EndAt == nil {
		// Nothing in progress to extend; an attempt with no deadline (no
		// time limit, no lock date) must not be given one here.
		return &ext, nil, nil
	}
	now := time.Now()
	var newEnd time.Time
	if in.ExtendFromNow != nil {
		newEnd = now.Add(time.Duration(*in.ExtendFromNow) * time.Minute)
	} else {
		base := now
		if latest.EndAt != nil && latest.EndAt.After(now) {
			base = *latest.EndAt
		}
		newEnd = base.Add(time.Duration(*in.ExtendFromEndAt) * time.Minute)
	}
	old := latest.EndAt
	latest.EndAt = &newEnd
	if err := s.submissionRepo.Update(ctx, latest); err != nil {
		return nil, nil, err
	}
	s.recordQuizEvent(ctx, latest, models.QuizEventAttemptExtended, map[string]interface{}{
		"by":         actorID,
		"old_end_at": old,
		"new_end_at": newEnd,
	})
	return &ext, latest, nil
}

// IsQuizCourseStudent reports whether userID is an active student of the
// course in the caller's tenant (moderation targets must be).
func (s *QuizService) IsQuizCourseStudent(ctx context.Context, courseID, userID, accountID uint) bool {
	if s.lifecycle == nil || s.lifecycle.Enrollments == nil {
		return false
	}
	ids, err := s.lifecycle.Enrollments.ListActiveStudentUserIDsByCourse(ctx, courseID, accountID)
	if err != nil {
		return false
	}
	for _, id := range ids {
		if id == userID {
			return true
		}
	}
	return false
}

// QuizModerationRow is one student's line on the Moderate quiz page.
type QuizModerationRow struct {
	UserID           uint
	AttemptsUsed     int
	AttemptsLeft     int // -1 unlimited
	KeptScore        *float64
	LatestState      string
	InProgressID     uint
	InProgressEndAt  *time.Time
	ExtraAttempts    int
	ExtraTime        int
	ManuallyUnlocked bool
}

// QuizModerationRoster lists every active student in the quiz's course
// with their attempts, kept score and extension.
func (s *QuizService) QuizModerationRoster(ctx context.Context, quiz *models.Quiz, accountID uint) ([]QuizModerationRow, error) {
	if s.lifecycle == nil || s.lifecycle.Enrollments == nil {
		return nil, errors.New("quiz moderation is not available")
	}
	studentIDs, err := s.lifecycle.Enrollments.ListActiveStudentUserIDsByCourse(ctx, quiz.CourseID, accountID)
	if err != nil {
		return nil, err
	}
	exts := map[uint]models.QuizExtension{}
	if s.lifecycle.Extensions != nil {
		if list, err := s.lifecycle.Extensions.ListByQuizID(ctx, quiz.ID); err == nil {
			for _, e := range list {
				exts[e.UserID] = e
			}
		}
	}
	attempts, err := s.submissionRepo.ListByQuizAndUserIDs(ctx, quiz.ID, studentIDs)
	if err != nil {
		return nil, err
	}
	byUser := map[uint][]models.QuizSubmission{}
	for _, a := range attempts {
		byUser[a.UserID] = append(byUser[a.UserID], a)
	}

	rows := make([]QuizModerationRow, 0, len(studentIDs))
	for _, uid := range studentIDs {
		ext := exts[uid]
		ext.QuizID, ext.UserID = quiz.ID, uid
		row := QuizModerationRow{
			UserID:           uid,
			ExtraAttempts:    ext.ExtraAttempts,
			ExtraTime:        ext.ExtraTime,
			ManuallyUnlocked: ext.ManuallyUnlocked,
		}
		mine := byUser[uid]
		used := 0
		var latest *models.QuizSubmission
		for i := range mine {
			a := &mine[i]
			if a.IsCompleted() {
				used++
			}
			if latest == nil || a.Attempt > latest.Attempt {
				latest = a
			}
			if a.WorkflowState == models.QuizSubmissionUntaken {
				row.InProgressID = a.ID
				row.InProgressEndAt = a.EndAt
			}
		}
		if kept, ok := KeptScore(quiz.ScoringPolicy, mine); ok {
			k := kept
			row.KeptScore = &k
		}
		if latest != nil {
			row.LatestState = latest.WorkflowState
		}
		row.AttemptsUsed = used
		row.AttemptsLeft = AttemptsLeft(quiz, ext, used)
		rows = append(rows, row)
	}
	return rows, nil
}

// QuizAttemptStatus is a student's own view of where they stand on a quiz.
type QuizAttemptStatus struct {
	AttemptsUsed     int
	AllowedAttempts  int // -1 unlimited, includes extra attempts
	AttemptsLeft     int // -1 unlimited
	ExtraTime        int
	ManuallyUnlocked bool
	KeptScore        *float64
	DueAt            *time.Time
	UnlockAt         *time.Time
	LockAt           *time.Time
}

// AttemptStatusFor returns the user's attempt count, limits and effective
// dates on the quiz (their overrides and extensions applied).
func (s *QuizService) AttemptStatusFor(ctx context.Context, quiz *models.Quiz, userID uint, includeScore bool) (*QuizAttemptStatus, error) {
	ext := s.quizExtensionFor(ctx, quiz.ID, userID)
	attempts, err := s.userAttempts(ctx, quiz.ID, userID)
	if err != nil {
		return nil, err
	}
	used := 0
	for i := range attempts {
		if attempts[i].IsCompleted() {
			used++
		}
	}
	st := &QuizAttemptStatus{
		AttemptsUsed:     used,
		AllowedAttempts:  AttemptsAllowedFor(quiz, ext),
		AttemptsLeft:     AttemptsLeft(quiz, ext, used),
		ExtraTime:        ext.ExtraTime,
		ManuallyUnlocked: ext.ManuallyUnlocked,
	}
	st.DueAt, st.UnlockAt, st.LockAt = s.EffectiveQuizDates(ctx, quiz, userID)
	if includeScore {
		if kept, ok := KeptScore(quiz.ScoringPolicy, attempts); ok {
			st.KeptScore = &kept
		}
	}
	return st, nil
}

// ---------- G048: server-side auto-submit ----------

// quizAutoSubmitGrace is how long past end_at an attempt is left alone
// before the server turns it in (covers an answer save or /complete call
// still in flight when the timer hits zero).
const quizAutoSubmitGrace = time.Minute

// AutoSubmitOverdueAttempts turns in every in-progress attempt whose
// deadline (time limit, extra time and lock date) has passed, grading it
// exactly as a student submission and stamping it finished at its
// deadline (Canvas OutstandingQuizSubmissionManager). Returns how many
// attempts were submitted.
func (s *QuizService) AutoSubmitOverdueAttempts(ctx context.Context, now time.Time) (int, error) {
	if s.lifecycle == nil || s.lifecycle.Overdue == nil {
		return 0, nil
	}
	done := 0
	for batch := 0; batch < 50; batch++ {
		subs, err := s.lifecycle.Overdue.ListOverdueUntaken(ctx, now.Add(-quizAutoSubmitGrace), 200)
		if err != nil {
			return done, err
		}
		if len(subs) == 0 {
			return done, nil
		}
		progressed := 0
		for i := range subs {
			sub := subs[i]
			if sub.EndAt == nil {
				continue
			}
			if _, err := s.finalizeAttempt(ctx, &sub, *sub.EndAt, models.QuizEventAutoSubmitted); err != nil {
				// Another request (the student's own submit) may have won
				// the race; anything else is logged and retried next tick.
				slog.Warn("quiz auto-submit", "quiz_submission_id", sub.ID, "err", err)
				continue
			}
			done++
			progressed++
		}
		if progressed == 0 {
			return done, nil
		}
	}
	return done, nil
}
