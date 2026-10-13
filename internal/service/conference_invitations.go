package service

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// Conference invitations (Canvas "Web Conference Invitation"): when a
// course conference is created or edited with invitees — specific course
// members, or every member of the course — each newly invited member
// becomes an "invitee" participant and is notified.

// SetInvitations wires conference invitations: the course roster (who may
// be invited), the course name for the message, and the notification
// queue. Without them invitations are ignored.
func (s *ConferenceService) SetInvitations(enrollments repository.EnrollmentRepository, courses repository.CourseRepository, notifier Notifier) *ConferenceService {
	s.enrollments = enrollments
	s.courses = courses
	s.notifier = notifier
	return s
}

// ConferenceInvite says who to invite: specific users, or every active
// member of the conference's course.
type ConferenceInvite struct {
	UserIDs []uint
	All     bool
}

// Empty reports whether the invite names nobody.
func (in ConferenceInvite) Empty() bool { return !in.All && len(in.UserIDs) == 0 }

// InviteAsync invites course members to a course conference and notifies
// each one not already a participant. Only active members of the
// conference's course can be invited (ids outside the roster, including
// other tenants' users, are ignored, as Canvas restricts invitees to the
// context's users); the inviter is never invited. The roster read, the
// participant rows and the notifications run off the request path on a
// copy of the conference with a detached context.
func (s *ConferenceService) InviteAsync(conference *models.Conference, in ConferenceInvite, inviterID uint) {
	if s.enrollments == nil || conference == nil || conference.ContextType != "Course" || in.Empty() {
		return
	}
	snapshot := *conference
	invite := ConferenceInvite{UserIDs: append([]uint(nil), in.UserIDs...), All: in.All}
	go func() {
		defer recoverFromPanic("conference invite")
		if _, err := s.Invite(context.Background(), &snapshot, invite, inviterID); err != nil {
			slog.Warn("conference invitations failed", "conference_id", snapshot.ID, "error", err)
		}
	}()
}

// Invite is the synchronous body of InviteAsync. It returns the users newly
// invited.
func (s *ConferenceService) Invite(ctx context.Context, conference *models.Conference, in ConferenceInvite, inviterID uint) ([]uint, error) {
	if s.enrollments == nil || conference.ContextType != "Course" || in.Empty() {
		return nil, nil
	}
	members, err := s.activeCourseMembers(ctx, conference.ContextID)
	if err != nil {
		return nil, err
	}
	var targets []uint
	if in.All {
		targets = members
	} else {
		roster := make(map[uint]bool, len(members))
		for _, id := range members {
			roster[id] = true
		}
		seen := map[uint]bool{}
		for _, id := range in.UserIDs {
			if roster[id] && !seen[id] {
				seen[id] = true
				targets = append(targets, id)
			}
		}
	}

	var invited []uint
	for _, uid := range targets {
		if uid == inviterID || uid == conference.UserID {
			continue
		}
		// accountID=0: the conference row was loaded tenant-scoped by the
		// handler; the user ids come from its course roster.
		if existing, _ := s.participantRepo.FindByConferenceAndUser(ctx, conference.ID, uid, 0); existing != nil {
			continue
		}
		if err := s.participantRepo.Create(ctx, &models.ConferenceParticipant{
			ConferenceID: conference.ID, UserID: uid, ParticipationType: "invitee",
		}); err != nil {
			continue
		}
		invited = append(invited, uid)
	}
	s.notifyInvited(ctx, conference, invited)
	return invited, nil
}

// activeCourseMembers lists the user ids of a course's active enrollments.
func (s *ConferenceService) activeCourseMembers(ctx context.Context, courseID uint) ([]uint, error) {
	const perPage = 200
	seen := map[uint]bool{}
	var out []uint
	for page := 1; ; page++ {
		// accountID=0: the course id is the stored conference's context,
		// which the handler already tenant-checked.
		res, err := s.enrollments.ListByCourseID(ctx, courseID, 0, repository.PaginationParams{Page: page, PerPage: perPage})
		if err != nil {
			return nil, err
		}
		if res == nil {
			return out, nil
		}
		for _, e := range res.Items {
			if e.WorkflowState != models.EnrollmentActive || seen[e.UserID] {
				continue
			}
			seen[e.UserID] = true
			out = append(out, e.UserID)
		}
		if len(res.Items) < perPage || int64(page*perPage) >= res.TotalCount {
			return out, nil
		}
	}
}

func (s *ConferenceService) notifyInvited(ctx context.Context, conference *models.Conference, userIDs []uint) {
	if s.notifier == nil || len(userIDs) == 0 {
		return
	}
	courseName := ""
	if s.courses != nil {
		if c, err := s.courses.FindByID(ctx, conference.ContextID, 0); err == nil && c != nil {
			courseName = c.Name
		}
	}
	subject := fmt.Sprintf("Web conference invitation: %s", conference.Title)
	where := ""
	if courseName != "" {
		where = fmt.Sprintf(" in <strong>%s</strong>", html.EscapeString(courseName))
	}
	body := fmt.Sprintf("<p>You have been invited to the web conference <strong>%s</strong>%s.</p>", html.EscapeString(conference.Title), where)
	if d := strings.TrimSpace(conference.Description); d != "" {
		// Description is stored as sanitized plain text; escape it for the
		// HTML body.
		body += fmt.Sprintf("<p>%s</p>", html.EscapeString(d))
	}
	body += "<p>Join it from the course's Conferences page.</p>"
	for _, uid := range userIDs {
		_ = s.notifier.QueueNotification(ctx, uid, NotificationWebConferenceInvitation, subject, body, "Course", conference.ContextID)
	}
}
