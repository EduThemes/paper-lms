package service

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// Scheduler notifications (Canvas "Appointment Reserved By User",
// "Appointment Canceled By User" and "Appointment Deleted For User").
//
// Each Notify* method returns at once: the recipient lookup and the
// per-recipient queue writes run in a goroutine on copies of the rows with
// a detached context (the request's fasthttp context is recycled once the
// handler returns), and a panic there is recovered. Without a notifier or
// a *gorm.DB (unit tests on mocks) they do nothing.

// SetNotifier wires scheduler notifications. Optional.
func (s *AppointmentGroupService) SetNotifier(n Notifier) *AppointmentGroupService {
	s.notifier = n
	return s
}

func (s *AppointmentGroupService) canNotify() bool { return s.notifier != nil && s.db != nil }

// NotifyReserved tells the group's instructors that a participant signed
// up for a slot.
func (s *AppointmentGroupService) NotifyReserved(g *models.AppointmentGroup, res *models.AppointmentReservation) {
	if !s.canNotify() || g == nil || res == nil {
		return
	}
	group, reservation := *g, *res
	go func() {
		defer recoverFromPanic("appointment notifyReserved")
		s.notifyInstructors(context.Background(), &group, &reservation, NotificationAppointmentReservedByUser)
	}()
}

// NotifyCanceled reports a canceled reservation. byParticipant is true when
// the booker (or a member of the student group it was booked for) canceled
// it: the instructors hear about it. Otherwise an instructor canceled it and
// the participant(s) are told their appointment was deleted.
func (s *AppointmentGroupService) NotifyCanceled(g *models.AppointmentGroup, res *models.AppointmentReservation, actorID uint, byParticipant bool) {
	if !s.canNotify() || g == nil || res == nil {
		return
	}
	group, reservation := *g, *res
	go func() {
		defer recoverFromPanic("appointment notifyCanceled")
		ctx := context.Background()
		if byParticipant {
			s.notifyInstructors(ctx, &group, &reservation, NotificationAppointmentCanceledByUser)
			return
		}
		var start *time.Time
		if slot := s.slotFor(ctx, reservation.SlotID); slot != nil {
			start = &slot.StartAt
		}
		s.notifyDeletedForParticipants(ctx, &group, []models.AppointmentReservation{reservation}, actorID, start)
	}()
}

// LiveReservations returns a slot's current (not canceled) reservations.
func (s *AppointmentGroupService) LiveReservations(ctx context.Context, slotID uint) []models.AppointmentReservation {
	all, err := s.reservationRepo.ListBySlot(ctx, slotID)
	if err != nil {
		return nil
	}
	live := make([]models.AppointmentReservation, 0, len(all))
	for _, r := range all {
		if r.WorkflowState == "reserved" {
			live = append(live, r)
		}
	}
	return live
}

// NotifySlotDeleted tells everyone who held a reservation on a slot an
// instructor removed that their appointment was deleted. reservations are
// the slot's live reservations, read before the slot was deleted.
func (s *AppointmentGroupService) NotifySlotDeleted(g *models.AppointmentGroup, slot *models.AppointmentSlot, reservations []models.AppointmentReservation, actorID uint) {
	if !s.canNotify() || g == nil || slot == nil || len(reservations) == 0 {
		return
	}
	group, start := *g, slot.StartAt
	list := append([]models.AppointmentReservation(nil), reservations...)
	go func() {
		defer recoverFromPanic("appointment notifySlotDeleted")
		s.notifyDeletedForParticipants(context.Background(), &group, list, actorID, &start)
	}()
}

// apptRecipient is the slice of a user a scheduler notification needs.
type apptRecipient struct {
	ID       uint
	Name     string
	TimeZone string
}

func (s *AppointmentGroupService) usersByID(ctx context.Context, ids []uint) map[uint]apptRecipient {
	out := map[uint]apptRecipient{}
	if len(ids) == 0 {
		return out
	}
	var rows []apptRecipient
	s.db.WithContext(ctx).Table("users").Select("id, name, COALESCE(time_zone, '') AS time_zone").
		Where("id IN ?", ids).Scan(&rows)
	for _, r := range rows {
		out[r.ID] = r
	}
	return out
}

// instructorIDs lists the active teachers and TAs of every course the group
// belongs to; the group's creator when it has none.
func (s *AppointmentGroupService) instructorIDs(ctx context.Context, g *models.AppointmentGroup) []uint {
	contexts, _ := s.GroupContexts(ctx, g.ID)
	var ids []uint
	s.db.WithContext(ctx).Model(&models.Enrollment{}).
		Where("course_id IN ? AND type IN ? AND workflow_state = ?", CourseIDs(g, contexts),
			[]string{models.TeacherEnrollmentType, models.TaEnrollmentType}, string(models.EnrollmentActive)).
		Distinct().Order("user_id").Pluck("user_id", &ids)
	if len(ids) == 0 && g.CreatedByUserID != 0 {
		ids = []uint{g.CreatedByUserID}
	}
	return ids
}

// participantIDs is who a reservation belongs to: the booker, or every
// accepted member of the student group it was made for.
func (s *AppointmentGroupService) participantIDs(ctx context.Context, res *models.AppointmentReservation) []uint {
	if res.ParticipantGroupID == nil {
		return []uint{res.UserID}
	}
	var ids []uint
	s.db.WithContext(ctx).Table("group_memberships").
		Where("group_id = ? AND workflow_state = ?", *res.ParticipantGroupID, "accepted").
		Order("user_id").Pluck("user_id", &ids)
	if len(ids) == 0 {
		ids = []uint{res.UserID}
	}
	return ids
}

// participantLabel names who holds a reservation: the student group's name
// for a group booking, else the booker's name.
func (s *AppointmentGroupService) participantLabel(ctx context.Context, res *models.AppointmentReservation, users map[uint]apptRecipient) string {
	if res.ParticipantGroupID != nil {
		var names []string
		s.db.WithContext(ctx).Table("groups").Where("id = ?", *res.ParticipantGroupID).Limit(1).Pluck("name", &names)
		if len(names) == 1 && names[0] != "" {
			return names[0]
		}
	}
	if u, ok := users[res.UserID]; ok && u.Name != "" {
		return u.Name
	}
	return "A participant"
}

func (s *AppointmentGroupService) slotFor(ctx context.Context, slotID uint) *models.AppointmentSlot {
	var slot models.AppointmentSlot
	if err := s.db.WithContext(ctx).First(&slot, slotID).Error; err != nil {
		return nil
	}
	return &slot
}

// formatNotificationTime renders t in the recipient's time zone (UTC when
// it is unset or unknown), e.g. "Mon, Jan 2, 2006 3:04 PM EST".
func formatNotificationTime(t time.Time, tz string) string {
	loc := time.UTC
	if tz = strings.TrimSpace(tz); tz != "" {
		if l, err := time.LoadLocation(tz); err == nil {
			loc = l
		}
	}
	return t.In(loc).Format("Mon, Jan 2, 2006 3:04 PM MST")
}

func (s *AppointmentGroupService) notifyInstructors(ctx context.Context, g *models.AppointmentGroup, res *models.AppointmentReservation, kind string) {
	recipients := s.instructorIDs(ctx, g)
	users := s.usersByID(ctx, append(append([]uint(nil), recipients...), res.UserID))
	who := s.participantLabel(ctx, res, users)
	slot := s.slotFor(ctx, res.SlotID)
	title := html.EscapeString(g.Title)
	for _, uid := range recipients {
		if uid == res.UserID {
			continue
		}
		when := ""
		if slot != nil {
			when = " on " + formatNotificationTime(slot.StartAt, users[uid].TimeZone)
		}
		var subject, body string
		if kind == NotificationAppointmentReservedByUser {
			subject = fmt.Sprintf("Appointment reserved: %s", g.Title)
			body = fmt.Sprintf("<p><strong>%s</strong> signed up for <strong>%s</strong>%s.</p>", html.EscapeString(who), title, html.EscapeString(when))
			if c := strings.TrimSpace(res.Comments); c != "" {
				body += fmt.Sprintf("<p>Comments: %s</p>", html.EscapeString(c))
			}
		} else {
			subject = fmt.Sprintf("Appointment canceled: %s", g.Title)
			body = fmt.Sprintf("<p><strong>%s</strong> canceled their appointment for <strong>%s</strong>%s.</p>", html.EscapeString(who), title, html.EscapeString(when))
		}
		_ = s.notifier.QueueNotification(ctx, uid, kind, subject, body, "Course", g.CourseID)
	}
}

// notifyDeletedForParticipants tells the holders of reservations an
// instructor (actorID) canceled; start is the slot's start time (nil when
// unknown).
func (s *AppointmentGroupService) notifyDeletedForParticipants(ctx context.Context, g *models.AppointmentGroup, reservations []models.AppointmentReservation, actorID uint, start *time.Time) {
	title := html.EscapeString(g.Title)
	seen := map[uint]bool{actorID: true}
	for i := range reservations {
		res := &reservations[i]
		recipients := s.participantIDs(ctx, res)
		users := s.usersByID(ctx, append(append([]uint(nil), recipients...), actorID))
		by := ""
		if u, ok := users[actorID]; ok && u.Name != "" {
			by = " by " + u.Name
		}
		for _, uid := range recipients {
			if seen[uid] {
				continue
			}
			seen[uid] = true
			when := ""
			if start != nil {
				when = " on " + formatNotificationTime(*start, users[uid].TimeZone)
			}
			subject := fmt.Sprintf("Appointment canceled: %s", g.Title)
			body := fmt.Sprintf("<p>Your appointment for <strong>%s</strong>%s was canceled%s.</p>", title, html.EscapeString(when), html.EscapeString(by))
			_ = s.notifier.QueueNotification(ctx, uid, NotificationAppointmentDeletedForUser, subject, body, "Course", g.CourseID)
		}
	}
}
