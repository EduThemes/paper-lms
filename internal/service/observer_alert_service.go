package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// ObserverLinkChecker is the slice of ObserverService the alert engine
// needs: is the pairing live, and which of the student's courses may the
// observer see.
type ObserverLinkChecker interface {
	IsObserverOf(ctx context.Context, observerUserID, studentUserID uint) (bool, error)
	GetObserveeCourses(ctx context.Context, observerUserID, studentUserID uint) ([]models.Course, error)
}

// ObserverAlertService manages observer alert thresholds and raises
// alerts (Canvas observer alerts): a low / high course grade, a low / high
// assignment grade, missing work and new course announcements.
//
// Alerts are raised by Evaluate, which the hourly scheduler runs for every
// threshold and which also runs (throttled) when an observer opens their
// alerts, so a parent never waits an hour to see a new alert.
type ObserverAlertService struct {
	learner       repository.LearnerRepository
	links         ObserverLinkChecker
	dashboard     *LearnerDashboardService
	grades        *GradeSummaryService
	announcements PlannerAnnouncementVisibility
	now           func() time.Time
}

func NewObserverAlertService(learner repository.LearnerRepository, links ObserverLinkChecker, dashboard *LearnerDashboardService, grades *GradeSummaryService) *ObserverAlertService {
	return &ObserverAlertService{learner: learner, links: links, dashboard: dashboard, grades: grades, now: time.Now}
}

// SetAnnouncementVisibility enables course_announcement alerts.
func (s *ObserverAlertService) SetAnnouncementVisibility(v PlannerAnnouncementVisibility) *ObserverAlertService {
	s.announcements = v
	return s
}

var (
	// ErrObserverAlertNotFound: unknown id, someone else's, or a student
	// the caller does not observe (handlers answer 404).
	ErrObserverAlertNotFound = errors.New("not found")
	// ErrInvalidAlertThreshold: bad alert type or threshold value.
	ErrInvalidAlertThreshold = errors.New("invalid alert threshold")
	// ErrInvalidAlertTransition: workflow_state change not allowed.
	ErrInvalidAlertTransition = errors.New("invalid alert state change")
)

// requireObservee checks the caller observes studentID within their tenant.
func (s *ObserverAlertService) requireObservee(ctx context.Context, observerID, accountID, studentID uint) error {
	users, err := s.learner.UsersByIDs(ctx, []uint{studentID})
	if err != nil || len(users) != 1 || (accountID != 0 && users[0].AccountID != accountID) {
		return ErrObserverAlertNotFound
	}
	ok, err := s.links.IsObserverOf(ctx, observerID, studentID)
	if err != nil || !ok {
		return ErrObserverAlertNotFound
	}
	return nil
}

// ThresholdInput is a create / update request.
type ThresholdInput struct {
	StudentID uint
	AlertType string
	Threshold *float64
}

func validateThreshold(alertType string, threshold *float64) (*float64, error) {
	if !models.IsObserverAlertType(alertType) {
		return nil, fmt.Errorf("%w: unknown alert_type %q", ErrInvalidAlertThreshold, alertType)
	}
	if !models.ObserverAlertTypeNeedsThreshold(alertType) {
		return nil, nil
	}
	if threshold == nil {
		return nil, fmt.Errorf("%w: %s needs a threshold percentage", ErrInvalidAlertThreshold, alertType)
	}
	if *threshold < 0 || *threshold > 1000 {
		return nil, fmt.Errorf("%w: threshold must be between 0 and 1000", ErrInvalidAlertThreshold)
	}
	v := *threshold
	return &v, nil
}

// ListThresholds returns the caller's thresholds (optionally for one student).
func (s *ObserverAlertService) ListThresholds(ctx context.Context, observerID, accountID uint, studentID *uint) ([]models.ObserverAlertThreshold, error) {
	if studentID != nil {
		if err := s.requireObservee(ctx, observerID, accountID, *studentID); err != nil {
			return nil, err
		}
	}
	return s.learner.ListAlertThresholds(ctx, observerID, studentID)
}

// CreateThreshold opts the caller in to an alert type for a student they
// observe. Re-creating an existing type updates its threshold.
func (s *ObserverAlertService) CreateThreshold(ctx context.Context, observerID, accountID uint, in ThresholdInput) (*models.ObserverAlertThreshold, error) {
	if err := s.requireObservee(ctx, observerID, accountID, in.StudentID); err != nil {
		return nil, err
	}
	th, err := validateThreshold(in.AlertType, in.Threshold)
	if err != nil {
		return nil, err
	}
	now := s.now()
	t := &models.ObserverAlertThreshold{
		AccountID: accountID, ObserverID: observerID, StudentID: in.StudentID,
		AlertType: in.AlertType, Threshold: th, LastEvaluatedAt: &now,
	}
	if err := s.learner.SaveAlertThreshold(ctx, t); err != nil {
		return nil, err
	}
	// The upsert may have hit an existing row; return the stored one.
	all, err := s.learner.ListAlertThresholds(ctx, observerID, &in.StudentID)
	if err == nil {
		for i := range all {
			if all[i].AlertType == in.AlertType {
				return &all[i], nil
			}
		}
	}
	return t, nil
}

// UpdateThreshold changes the threshold value of the caller's own row.
func (s *ObserverAlertService) UpdateThreshold(ctx context.Context, observerID, id uint, threshold *float64) (*models.ObserverAlertThreshold, error) {
	t, err := s.learner.GetAlertThreshold(ctx, id, observerID)
	if err != nil {
		return nil, ErrObserverAlertNotFound
	}
	th, err := validateThreshold(t.AlertType, threshold)
	if err != nil {
		return nil, err
	}
	t.Threshold = th
	if err := s.learner.SaveAlertThreshold(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// DeleteThreshold removes the caller's own row (and its alerts).
func (s *ObserverAlertService) DeleteThreshold(ctx context.Context, observerID, id uint) error {
	if err := s.learner.DeleteAlertThreshold(ctx, id, observerID); err != nil {
		if errors.Is(err, repository.ErrLearnerNotFound) {
			return ErrObserverAlertNotFound
		}
		return err
	}
	return nil
}

// ListAlerts evaluates the caller's thresholds (throttled) and returns their
// undismissed alerts, newest first.
func (s *ObserverAlertService) ListAlerts(ctx context.Context, observerID, accountID uint, studentID *uint) ([]models.ObserverAlert, error) {
	if studentID != nil {
		if err := s.requireObservee(ctx, observerID, accountID, *studentID); err != nil {
			return nil, err
		}
	}
	s.EvaluateObserver(ctx, observerID, 5*time.Minute)
	list, err := s.learner.ListAlerts(ctx, observerID, studentID, 200)
	if err != nil {
		return nil, err
	}
	return s.keepLinked(ctx, observerID, list), nil
}

// keepLinked drops alerts about students the observer no longer observes:
// once the pairing is removed the parent loses the alert history too, as
// in Canvas (unlinking destroys the observer's alert thresholds).
func (s *ObserverAlertService) keepLinked(ctx context.Context, observerID uint, list []models.ObserverAlert) []models.ObserverAlert {
	linked := map[uint]bool{}
	kept := list[:0]
	for _, a := range list {
		ok, seen := linked[a.StudentID]
		if !seen {
			isLinked, err := s.links.IsObserverOf(ctx, observerID, a.StudentID)
			ok = err == nil && isLinked
			linked[a.StudentID] = ok
		}
		if ok {
			kept = append(kept, a)
		}
	}
	return kept
}

// UnreadCount evaluates (throttled) and counts the caller's unread alerts
// about students they still observe.
func (s *ObserverAlertService) UnreadCount(ctx context.Context, observerID uint) (int64, error) {
	s.EvaluateObserver(ctx, observerID, 5*time.Minute)
	list, err := s.learner.ListAlerts(ctx, observerID, nil, 1000)
	if err != nil {
		return 0, err
	}
	var n int64
	for _, a := range s.keepLinked(ctx, observerID, list) {
		if a.WorkflowState == models.ObserverAlertUnread {
			n++
		}
	}
	return n, nil
}

// SetAlertState marks the caller's alert read or dismissed.
func (s *ObserverAlertService) SetAlertState(ctx context.Context, observerID, id uint, state models.ObserverAlertWorkflow) (*models.ObserverAlert, error) {
	a, err := s.learner.GetAlert(ctx, id, observerID)
	if err != nil {
		return nil, ErrObserverAlertNotFound
	}
	if !a.WorkflowState.CanTransitionTo(state) {
		return nil, ErrInvalidAlertTransition
	}
	if err := s.learner.UpdateAlertState(ctx, a.ID, state); err != nil {
		return nil, err
	}
	a.WorkflowState = state
	return a, nil
}

// EvaluateObserver runs the scan for every threshold of one observer that
// was not evaluated within minAge.
func (s *ObserverAlertService) EvaluateObserver(ctx context.Context, observerID uint, minAge time.Duration) {
	ths, err := s.learner.ListAlertThresholds(ctx, observerID, nil)
	if err != nil {
		return
	}
	now := s.now()
	for i := range ths {
		if ths[i].LastEvaluatedAt != nil && now.Sub(*ths[i].LastEvaluatedAt) < minAge {
			continue
		}
		if err := s.Evaluate(ctx, &ths[i]); err != nil {
			slog.Warn("observer alert evaluation failed", "threshold_id", ths[i].ID, "error", err)
		}
	}
}

// EvaluateAll is the scheduler job: scan every threshold.
func (s *ObserverAlertService) EvaluateAll(ctx context.Context) error {
	ths, err := s.learner.ListAllAlertThresholds(ctx)
	if err != nil {
		return err
	}
	for i := range ths {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := s.Evaluate(ctx, &ths[i]); err != nil {
			slog.Warn("observer alert evaluation failed", "threshold_id", ths[i].ID, "error", err)
		}
	}
	return nil
}

// Evaluate raises any new alerts for one threshold. Events (grades,
// announcements, due dates) are only considered from the moment the
// observer opted in, so turning an alert on never floods them with history.
func (s *ObserverAlertService) Evaluate(ctx context.Context, t *models.ObserverAlertThreshold) error {
	now := s.now()
	since := t.CreatedAt
	if t.LastEvaluatedAt != nil && t.LastEvaluatedAt.After(since) {
		since = *t.LastEvaluatedAt
	}
	// Look back a little past the high-water mark so an event stamped just
	// before the previous scan finished is not lost; the unique index keeps
	// it from alerting twice.
	since = since.Add(-10 * time.Minute)
	if since.Before(t.CreatedAt) {
		since = t.CreatedAt
	}
	ok, err := s.links.IsObserverOf(ctx, t.ObserverID, t.StudentID)
	if err != nil {
		return err
	}
	if !ok {
		return s.learner.TouchAlertThreshold(ctx, t.ID, now)
	}
	courses, err := s.links.GetObserveeCourses(ctx, t.ObserverID, t.StudentID)
	if err != nil {
		return err
	}
	courseIDs := make([]uint, 0, len(courses))
	visible := map[uint]bool{}
	names := map[uint]string{}
	for _, c := range courses {
		if c.AccountID != t.AccountID || c.WorkflowState != models.CourseAvailable {
			continue
		}
		courseIDs = append(courseIDs, c.ID)
		visible[c.ID] = true
		names[c.ID] = c.Name
	}
	studentName := ""
	if users, _ := s.learner.UsersByIDs(ctx, []uint{t.StudentID}); len(users) == 1 {
		studentName = users[0].Name
	}

	raise := func(contextType string, contextID uint, courseID *uint, title, link string, when time.Time) error {
		_, err := s.learner.CreateAlertIfAbsent(ctx, &models.ObserverAlert{
			AccountID: t.AccountID, ThresholdID: t.ID, ObserverID: t.ObserverID, StudentID: t.StudentID,
			AlertType: t.AlertType, ContextType: contextType, ContextID: contextID, CourseID: courseID,
			Title: title, Link: link, ActionDate: when,
		})
		return err
	}

	switch t.AlertType {
	case models.ObserverAlertAssignmentMissing:
		missing, err := s.dashboard.MissingSubmissions(ctx, t.StudentID, t.AccountID, visible, nil)
		if err != nil {
			return err
		}
		for _, m := range missing {
			if m.DueAt == nil || m.DueAt.Before(t.CreatedAt) {
				continue
			}
			cid := m.CourseID
			if err := raise("Assignment", m.ID, &cid,
				fmt.Sprintf("Assignment missing: %s in %s", m.Name, m.CourseName), m.HTMLURL, *m.DueAt); err != nil {
				return err
			}
		}

	case models.ObserverAlertAssignmentGradeLow, models.ObserverAlertAssignmentGradeHigh:
		if t.Threshold == nil {
			break
		}
		subs, err := s.learner.RecentlyPostedSubmissions(ctx, t.StudentID, courseIDs, since)
		if err != nil {
			return err
		}
		assignments, err := s.learner.PublishedAssignments(ctx, courseIDs)
		if err != nil {
			return err
		}
		byID := map[uint]models.Assignment{}
		for _, a := range assignments {
			byID[a.ID] = a
		}
		for _, sub := range subs {
			a, ok := byID[sub.AssignmentID]
			if !ok || sub.Score == nil || sub.Excused || a.PointsPossible == nil || *a.PointsPossible <= 0 {
				continue
			}
			pct := *sub.Score / *a.PointsPossible * 100
			low := t.AlertType == models.ObserverAlertAssignmentGradeLow
			if (low && pct >= *t.Threshold) || (!low && pct <= *t.Threshold) {
				continue
			}
			word := "high"
			if low {
				word = "low"
			}
			cid := a.CourseID
			when := *sub.PostedAt
			if err := raise("Submission", sub.ID, &cid,
				fmt.Sprintf("Assignment grade %s: %s scored %.1f%% on %s", word, studentName, pct, a.Name),
				fmt.Sprintf("/courses/%d/grades?observee_id=%d", a.CourseID, t.StudentID), when); err != nil {
				return err
			}
		}

	case models.ObserverAlertCourseGradeLow, models.ObserverAlertCourseGradeHigh:
		if t.Threshold == nil || s.grades == nil {
			break
		}
		low := t.AlertType == models.ObserverAlertCourseGradeLow
		for _, cid := range courseIDs {
			// nil period: the current grading period when the course has
			// periods (what the parent sees by default), else the whole course.
			sum, err := s.grades.Summary(ctx, cid, t.StudentID, t.AccountID, nil, GradeSummaryViewer{})
			if err != nil || sum.Totals == nil {
				continue
			}
			// The grade the student has: the instructor's final grade
			// override when set, else the computed current score.
			current := sum.Totals.CurrentScore
			if sum.Totals.OverrideScore != nil {
				current = sum.Totals.OverrideScore
			}
			if current == nil {
				continue
			}
			score := *current
			crossed := (low && score < *t.Threshold) || (!low && score > *t.Threshold)
			if !crossed {
				// Re-arm: the grade recovered, so a later crossing alerts again.
				if err := s.learner.ClearHandledAlert(ctx, t.ID, "Course", cid); err != nil {
					return err
				}
				continue
			}
			word := "high"
			if low {
				word = "low"
			}
			c := cid
			if err := raise("Course", cid, &c,
				fmt.Sprintf("Course grade %s: %s has %.1f%% in %s", word, studentName, score, names[cid]),
				fmt.Sprintf("/courses/%d/grades?observee_id=%d", cid, t.StudentID), now); err != nil {
				return err
			}
		}

	case models.ObserverAlertCourseAnnouncement:
		if s.announcements == nil {
			break
		}
		anns, err := s.learner.RecentAnnouncements(ctx, courseIDs, since)
		if err != nil {
			return err
		}
		for i := range anns {
			a := &anns[i]
			if a.CourseID == nil || !visible[*a.CourseID] || !s.announcements.CanView(ctx, a, t.StudentID) {
				continue
			}
			when := a.CreatedAt
			if a.PostedAt != nil {
				when = *a.PostedAt
			}
			if when.Before(t.CreatedAt) {
				continue
			}
			cid := *a.CourseID
			if err := raise("Announcement", a.ID, &cid,
				fmt.Sprintf("Announcement in %s: %s", names[cid], a.Title),
				fmt.Sprintf("/courses/%d/announcements", cid), when); err != nil {
				return err
			}
		}
	}
	return s.learner.TouchAlertThreshold(ctx, t.ID, now)
}
