package service

import (
	"context"
	"fmt"
	"html"
	"log/slog"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// Observer alert delivery: every new alert is also queued as an
// "observer_alert" notification to the observer (in-app feed plus their
// delivery channels, per their Observer Alerts preference), and grade
// alerts are evaluated the moment a grade becomes visible instead of
// waiting for the hourly scan.

// SetNotifier wires observer alert notifications. Optional.
func (s *ObserverAlertService) SetNotifier(n Notifier) *ObserverAlertService {
	s.notifier = n
	return s
}

// gradeAlertTypes are the threshold types a newly visible grade can trip.
var gradeAlertTypes = []string{
	models.ObserverAlertAssignmentGradeLow, models.ObserverAlertAssignmentGradeHigh,
	models.ObserverAlertCourseGradeLow, models.ObserverAlertCourseGradeHigh,
}

// GradedCallback evaluates the grade alert thresholds about the graded
// student. Register it with SubmissionService.OnGraded, which runs it in a
// goroutine with a detached context once the grade is visible to the
// student (posted), so a hidden grade never alerts a parent early.
func (s *ObserverAlertService) GradedCallback(submissions repository.SubmissionRepository) SubmissionGradedCallback {
	return func(ctx context.Context, submissionID uint) {
		// AUTH-INTERNAL: accountID=0 — the id comes from a grade write that
		// was already tenant-checked, not from request input.
		sub, err := submissions.FindByID(ctx, submissionID, 0)
		if err != nil || sub == nil || sub.UserID == 0 || sub.PostedAt == nil {
			return
		}
		s.EvaluateGradeAlerts(ctx, sub.UserID)
	}
}

// EvaluateGradeAlerts runs the grade alert thresholds every observer has
// set for one student.
func (s *ObserverAlertService) EvaluateGradeAlerts(ctx context.Context, studentID uint) {
	ths, err := s.learner.ListAlertThresholdsForStudent(ctx, studentID, gradeAlertTypes)
	if err != nil {
		slog.Warn("observer grade alerts: list thresholds failed", "student_id", studentID, "error", err)
		return
	}
	for i := range ths {
		if err := s.Evaluate(ctx, &ths[i]); err != nil {
			slog.Warn("observer alert evaluation failed", "threshold_id", ths[i].ID, "error", err)
		}
	}
}

// notifyAlertsAsync queues one notification per new alert off the caller's
// path: Evaluate also runs inside an observer's GET of their alerts, and the
// request context must not be used once the handler returns.
func (s *ObserverAlertService) notifyAlertsAsync(studentName string, alerts []models.ObserverAlert) {
	if s.notifier == nil || len(alerts) == 0 {
		return
	}
	list := append([]models.ObserverAlert(nil), alerts...)
	go func() {
		defer recoverFromPanic("observer alert notify")
		ctx := context.Background()
		for i := range list {
			s.notifyAlert(ctx, studentName, &list[i])
		}
	}()
}

func (s *ObserverAlertService) notifyAlert(ctx context.Context, studentName string, a *models.ObserverAlert) {
	contextType, contextID := "User", a.StudentID
	if a.CourseID != nil {
		contextType, contextID = "Course", *a.CourseID
	}
	subject := "Observer alert: " + a.Title
	who := ""
	if studentName != "" {
		who = fmt.Sprintf(" about <strong>%s</strong>", html.EscapeString(studentName))
	}
	body := fmt.Sprintf("<p>New alert%s:</p><p>%s</p><p>See all alerts on your observer dashboard.</p>", who, html.EscapeString(a.Title))
	_ = s.notifier.QueueNotification(ctx, a.ObserverID, NotificationObserverAlert, subject, body, contextType, contextID)
}
