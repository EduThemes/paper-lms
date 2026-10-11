package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// Scheduler parity (G097): appointment groups spanning several courses,
// limited to sections, or reserved by student groups; adding and
// removing slots after creation; a participant's own appointments; and
// the participant roster an instructor can message.
//
// These methods need the *gorm.DB the service was built with; with a nil
// db (unit tests on mocks) they degrade to the single-course behavior.

var (
	ErrNotInParticipantGroup = errors.New("you must be in a group of this group set to sign up")
	ErrAppointmentContexts   = errors.New("invalid appointment group contexts")
)

// ReserveOptions carries the group a reservation is made for (Group
// appointment groups) and the participant's note.
type ReserveOptions struct {
	ParticipantGroupID *uint
	Comments           string
}

// HasDB reports whether the service can run the multi-context queries.
func (s *AppointmentGroupService) HasDB() bool { return s.db != nil }

// GroupContexts returns the extra courses and section limits of a group.
func (s *AppointmentGroupService) GroupContexts(ctx context.Context, groupID uint) ([]models.AppointmentGroupContext, error) {
	if s.db == nil {
		return nil, nil
	}
	var rows []models.AppointmentGroupContext
	err := s.db.WithContext(ctx).Where("appointment_group_id = ?", groupID).Order("id ASC").Find(&rows).Error
	return rows, err
}

// CourseIDs lists every course a group belongs to, the primary first.
func CourseIDs(g *models.AppointmentGroup, contexts []models.AppointmentGroupContext) []uint {
	out := []uint{g.CourseID}
	for _, c := range contexts {
		if c.ContextType == models.CalendarContextCourse && c.ContextID != g.CourseID {
			out = append(out, c.ContextID)
		}
	}
	return out
}

// SetGroupContexts replaces a group's extra courses and section limits.
func (s *AppointmentGroupService) SetGroupContexts(ctx context.Context, groupID uint, contexts []models.AppointmentGroupContext) error {
	if s.db == nil {
		return nil
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("appointment_group_id = ?", groupID).Delete(&models.AppointmentGroupContext{}).Error; err != nil {
			return err
		}
		for i := range contexts {
			row := contexts[i]
			row.ID = 0
			row.AppointmentGroupID = groupID
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ValidateContexts checks extra courses and sections all sit in the
// tenant, sections belong to one of the group's courses and the group
// category belongs to one of them. It returns the normalized rows.
func (s *AppointmentGroupService) ValidateContexts(ctx context.Context, accountID, primaryCourseID uint, courseIDs, sectionIDs []uint, groupCategoryID *uint) ([]models.AppointmentGroupContext, error) {
	if s.db == nil {
		return nil, nil
	}
	db := s.db.WithContext(ctx)
	all := map[uint]bool{primaryCourseID: true}
	var rows []models.AppointmentGroupContext
	for _, id := range courseIDs {
		if all[id] {
			continue
		}
		var n int64
		db.Model(&models.Course{}).Where("id = ? AND account_id = ? AND workflow_state <> ?", id, accountID, string(models.CourseDeleted)).Count(&n)
		if n == 0 {
			return nil, fmt.Errorf("%w: course %d not found", ErrAppointmentContexts, id)
		}
		all[id] = true
		rows = append(rows, models.AppointmentGroupContext{ContextType: models.CalendarContextCourse, ContextID: id})
	}
	courseList := make([]uint, 0, len(all))
	for id := range all {
		courseList = append(courseList, id)
	}
	seen := map[uint]bool{}
	for _, id := range sectionIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		var n int64
		db.Model(&models.CourseSection{}).Where("id = ? AND course_id IN ? AND workflow_state <> ?", id, courseList, "deleted").Count(&n)
		if n == 0 {
			return nil, fmt.Errorf("%w: section %d is not in this appointment group's courses", ErrAppointmentContexts, id)
		}
		rows = append(rows, models.AppointmentGroupContext{ContextType: models.CalendarContextCourseSection, ContextID: id})
	}
	if groupCategoryID != nil {
		var n int64
		db.Model(&models.GroupCategory{}).Where("id = ? AND course_id IN ? AND workflow_state <> ?", *groupCategoryID, courseList, "deleted").Count(&n)
		if n == 0 {
			return nil, fmt.Errorf("%w: group set %d is not in this appointment group's courses", ErrAppointmentContexts, *groupCategoryID)
		}
	}
	return rows, nil
}

// SectionAllowed reports whether a user enrolled in courseID may sign up
// given the group's section limits for that course.
func (s *AppointmentGroupService) SectionAllowed(ctx context.Context, contexts []models.AppointmentGroupContext, courseID, userID uint) bool {
	if s.db == nil {
		return true
	}
	var limited []uint
	for _, c := range contexts {
		if c.ContextType == models.CalendarContextCourseSection {
			limited = append(limited, c.ContextID)
		}
	}
	if len(limited) == 0 {
		return true
	}
	db := s.db.WithContext(ctx)
	// Only the limits that belong to this course apply to it.
	var inCourse []uint
	db.Model(&models.CourseSection{}).Where("id IN ? AND course_id = ?", limited, courseID).Pluck("id", &inCourse)
	if len(inCourse) == 0 {
		return true
	}
	var n int64
	db.Model(&models.Enrollment{}).
		Where("user_id = ? AND course_id = ? AND workflow_state = ? AND course_section_id IN ?", userID, courseID, string(models.EnrollmentActive), inCourse).
		Count(&n)
	return n > 0
}

// CanParticipate reports whether the user is an active student of one of
// the group's courses and inside that course's section limits.
func (s *AppointmentGroupService) CanParticipate(ctx context.Context, g *models.AppointmentGroup, contexts []models.AppointmentGroupContext, userID uint) bool {
	if s.db == nil {
		return true
	}
	var courses []uint
	s.db.WithContext(ctx).Model(&models.Enrollment{}).
		Where("user_id = ? AND course_id IN ? AND type = ? AND workflow_state = ?", userID, CourseIDs(g, contexts), "StudentEnrollment", string(models.EnrollmentActive)).
		Distinct().Pluck("course_id", &courses)
	for _, cid := range courses {
		if s.SectionAllowed(ctx, contexts, cid, userID) {
			return true
		}
	}
	return false
}

// ParticipantGroupFor returns the user's group in the category (nil when
// they have none).
func (s *AppointmentGroupService) ParticipantGroupFor(ctx context.Context, groupCategoryID, userID uint) *uint {
	if s.db == nil {
		return nil
	}
	var ids []uint
	s.db.WithContext(ctx).Table("group_memberships gm").
		Joins("JOIN groups g ON g.id = gm.group_id").
		Where("g.group_category_id = ? AND gm.user_id = ? AND gm.workflow_state = ? AND g.workflow_state <> ?", groupCategoryID, userID, "accepted", "deleted").
		Order("g.id ASC").
		Limit(1).
		Pluck("g.id", &ids)
	if len(ids) == 0 {
		return nil
	}
	return &ids[0]
}

// IsGroupMember reports whether the user is an accepted member of group.
func (s *AppointmentGroupService) IsGroupMember(ctx context.Context, groupID, userID uint) bool {
	if s.db == nil {
		return false
	}
	var n int64
	s.db.WithContext(ctx).Table("group_memberships").
		Where("group_id = ? AND user_id = ? AND workflow_state = ?", groupID, userID, "accepted").
		Count(&n)
	return n > 0
}

// AddSlots appends bookable slots to an existing group.
func (s *AppointmentGroupService) AddSlots(ctx context.Context, groupID uint, slots []SlotInput) ([]models.AppointmentSlot, error) {
	for _, in := range slots {
		if !in.EndAt.After(in.StartAt) {
			return nil, ErrInvalidSlotTimes
		}
	}
	out := make([]models.AppointmentSlot, 0, len(slots))
	for _, in := range slots {
		slot := models.AppointmentSlot{GroupID: groupID, StartAt: in.StartAt, EndAt: in.EndAt, ParticipantsLimit: in.ParticipantsLimit}
		if err := s.slotRepo.Create(ctx, &slot); err != nil {
			return nil, err
		}
		out = append(out, slot)
	}
	return out, nil
}

// DeleteSlot removes a slot; its reservations go with it.
func (s *AppointmentGroupService) DeleteSlot(ctx context.Context, slotID uint) error {
	return s.slotRepo.Delete(ctx, slotID)
}

// ReserveWith is Reserve for a Group appointment group (or with a note):
// for a group reservation the duplicate, per-participant maximum and
// capacity checks count the student group, not the member who booked.
func (s *AppointmentGroupService) ReserveWith(ctx context.Context, groupID, slotID, userID uint, opts ReserveOptions) (*models.AppointmentReservation, error) {
	if opts.ParticipantGroupID == nil {
		res, err := s.Reserve(ctx, groupID, slotID, userID)
		if err == nil && opts.Comments != "" {
			res.Comments = opts.Comments
			err = s.reservationRepo.Update(ctx, res)
		}
		return res, err
	}
	pg := *opts.ParticipantGroupID
	var reservation models.AppointmentReservation
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var group models.AppointmentGroup
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&group, groupID).Error; err != nil {
			return ErrAppointmentGroupGone
		}
		if group.WorkflowState == "deleted" {
			return ErrAppointmentGroupGone
		}
		var slot models.AppointmentSlot
		if err := tx.First(&slot, slotID).Error; err != nil {
			return err
		}
		if slot.GroupID != groupID {
			return ErrSlotMismatch
		}
		if !slot.StartAt.After(time.Now()) {
			return ErrSlotInPast
		}
		var existing int64
		if err := tx.Model(&models.AppointmentReservation{}).
			Where("slot_id = ? AND participant_group_id = ? AND workflow_state = ?", slotID, pg, "reserved").
			Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return ErrAlreadyReserved
		}
		if group.MaxAppointmentsPerParticipant > 0 {
			var n int64
			if err := tx.Model(&models.AppointmentReservation{}).
				Where("group_id = ? AND participant_group_id = ? AND workflow_state = ?", groupID, pg, "reserved").
				Count(&n).Error; err != nil {
				return err
			}
			if int(n) >= group.MaxAppointmentsPerParticipant {
				return ErrMaxReservationsHit
			}
		}
		limit := group.ParticipantsPerAppointment
		if slot.ParticipantsLimit != nil {
			limit = *slot.ParticipantsLimit
		}
		var count int64
		if err := tx.Model(&models.AppointmentReservation{}).
			Where("slot_id = ? AND workflow_state = ?", slotID, "reserved").
			Count(&count).Error; err != nil {
			return err
		}
		if int(count) >= limit {
			return ErrSlotFull
		}
		reservation = models.AppointmentReservation{
			SlotID:             slotID,
			GroupID:            groupID,
			UserID:             userID,
			ParticipantGroupID: &pg,
			Comments:           opts.Comments,
			ReservedAt:         time.Now(),
			WorkflowState:      "reserved",
		}
		return tx.Create(&reservation).Error
	})
	if err != nil {
		return nil, err
	}
	return &reservation, nil
}

// MyAppointment is one of the caller's reservations with its slot and
// group details (GET /users/self/appointments).
type MyAppointment struct {
	ID                 uint       `json:"id"`
	SlotID             uint       `json:"slot_id"`
	GroupID            uint       `json:"group_id"`
	UserID             uint       `json:"user_id"`
	ParticipantGroupID *uint      `json:"participant_group_id"`
	ParticipantName    string     `json:"participant_name,omitempty"`
	Comments           string     `json:"comments"`
	ReservedAt         time.Time  `json:"reserved_at"`
	CanceledAt         *time.Time `json:"canceled_at"`
	WorkflowState      string     `json:"workflow_state"`
	StartAt            time.Time  `json:"start_at"`
	EndAt              time.Time  `json:"end_at"`
	Title              string     `json:"title"`
	Description        string     `json:"description"`
	LocationName       string     `json:"location_name"`
	LocationAddress    string     `json:"location_address"`
	CourseID           uint       `json:"course_id"`
	CourseName         string     `json:"course_name"`
}

// ListMyAppointments returns the user's live reservations (and their
// student groups'), soonest first.
func (s *AppointmentGroupService) ListMyAppointments(ctx context.Context, userID, accountID uint) ([]MyAppointment, error) {
	var out []MyAppointment
	err := s.db.WithContext(ctx).Table("appointment_reservations r").
		Select(`r.id, r.slot_id, r.group_id, r.user_id, r.participant_group_id, COALESCE(pg.name, '') AS participant_name,
			COALESCE(r.comments, '') AS comments, r.reserved_at, r.canceled_at, r.workflow_state,
			s.start_at, s.end_at, g.title, COALESCE(g.description, '') AS description,
			COALESCE(g.location_name, '') AS location_name, COALESCE(g.location_address, '') AS location_address,
			g.course_id, c.name AS course_name`).
		Joins("JOIN appointment_slots s ON s.id = r.slot_id").
		Joins("JOIN appointment_groups g ON g.id = r.group_id").
		Joins("JOIN courses c ON c.id = g.course_id").
		Joins("LEFT JOIN groups pg ON pg.id = r.participant_group_id").
		Where("r.workflow_state = ? AND g.workflow_state <> ? AND c.account_id = ?", "reserved", "deleted", accountID).
		Where(`r.user_id = ? OR r.participant_group_id IN (
			SELECT group_id FROM group_memberships WHERE user_id = ? AND workflow_state = 'accepted')`, userID, userID).
		Order("s.start_at ASC, r.id ASC").
		Scan(&out).Error
	return out, err
}

// ListGroupsForUser lists appointment groups across the user's courses.
// scope "manageable" returns groups in courses the user teaches (every
// state but deleted); "reservable" (default) returns active groups in
// courses where the user is a student.
func (s *AppointmentGroupService) ListGroupsForUser(ctx context.Context, userID, accountID uint, scope string) ([]models.AppointmentGroup, error) {
	if s.db == nil {
		return nil, nil
	}
	types := []string{"StudentEnrollment"}
	states := []string{"active"}
	if scope == "manageable" {
		types = []string{"TeacherEnrollment", "TaEnrollment"}
		states = []string{"active", "pending"}
	}
	courses := s.db.Table("enrollments e").Select("e.course_id").
		Joins("JOIN courses c ON c.id = e.course_id").
		Where("e.user_id = ? AND e.workflow_state = ? AND e.type IN ? AND c.account_id = ? AND c.workflow_state <> ?",
			userID, string(models.EnrollmentActive), types, accountID, string(models.CourseDeleted))
	var out []models.AppointmentGroup
	err := s.db.WithContext(ctx).
		Where("workflow_state IN ?", states).
		Where(`course_id IN (?) OR id IN (
			SELECT appointment_group_id FROM appointment_group_contexts WHERE context_type = 'Course' AND context_id IN (?))`, courses, courses).
		Order("created_at DESC, id DESC").
		Find(&out).Error
	return out, err
}

// AppointmentParticipant is one row of the participant roster.
type AppointmentParticipant struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	Registered bool   `json:"registered"`
	// MemberIDs lists the members of a student group participant.
	MemberIDs []uint `json:"member_ids,omitempty"`
}

// Participants lists who may sign up (students of the group's courses,
// within any section limits — or the student groups of the group set)
// and whether each has a live reservation. status filters to
// "registered" or "unregistered"; anything else returns everyone.
func (s *AppointmentGroupService) Participants(ctx context.Context, g *models.AppointmentGroup, contexts []models.AppointmentGroupContext, status string) ([]AppointmentParticipant, error) {
	if s.db == nil {
		return nil, nil
	}
	db := s.db.WithContext(ctx)
	var out []AppointmentParticipant
	keep := func(registered bool) bool {
		switch status {
		case "registered":
			return registered
		case "unregistered":
			return !registered
		}
		return true
	}

	if g.ParticipantType == models.AppointmentParticipantGroup && g.GroupCategoryID != nil {
		var groups []models.Group
		if err := db.Where("group_category_id = ? AND workflow_state <> ?", *g.GroupCategoryID, "deleted").Order("name ASC, id ASC").Find(&groups).Error; err != nil {
			return nil, err
		}
		var booked []uint
		db.Model(&models.AppointmentReservation{}).
			Where("group_id = ? AND workflow_state = ? AND participant_group_id IS NOT NULL", g.ID, "reserved").
			Distinct().Pluck("participant_group_id", &booked)
		isBooked := map[uint]bool{}
		for _, id := range booked {
			isBooked[id] = true
		}
		for _, gr := range groups {
			if !keep(isBooked[gr.ID]) {
				continue
			}
			var members []uint
			db.Model(&models.GroupMembership{}).Where("group_id = ? AND workflow_state = ?", gr.ID, "accepted").Pluck("user_id", &members)
			out = append(out, AppointmentParticipant{ID: gr.ID, Name: gr.Name, Registered: isBooked[gr.ID], MemberIDs: members})
		}
		return out, nil
	}

	courseIDs := CourseIDs(g, contexts)
	var limited []uint
	for _, c := range contexts {
		if c.ContextType == models.CalendarContextCourseSection {
			limited = append(limited, c.ContextID)
		}
	}
	type row struct {
		ID   uint
		Name string
	}
	q := db.Table("enrollments e").
		Select("DISTINCT u.id, u.name").
		Joins("JOIN users u ON u.id = e.user_id").
		Where("e.course_id IN ? AND e.type = ? AND e.workflow_state = ?", courseIDs, "StudentEnrollment", string(models.EnrollmentActive))
	if len(limited) > 0 {
		// A course with section limits contributes only those sections;
		// a course without any keeps all its students.
		q = q.Where(`e.course_section_id IN ? OR e.course_id NOT IN (
			SELECT course_id FROM course_sections WHERE id IN ?)`, limited, limited)
	}
	var rows []row
	if err := q.Order("u.name ASC, u.id ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	var booked []uint
	db.Model(&models.AppointmentReservation{}).
		Where("group_id = ? AND workflow_state = ?", g.ID, "reserved").
		Distinct().Pluck("user_id", &booked)
	isBooked := map[uint]bool{}
	for _, id := range booked {
		isBooked[id] = true
	}
	for _, r := range rows {
		if !keep(isBooked[r.ID]) {
			continue
		}
		out = append(out, AppointmentParticipant{ID: r.ID, Name: r.Name, Registered: isBooked[r.ID]})
	}
	return out, nil
}

// SlotParticipants returns the names on each live reservation of a slot
// (student group names for a Group appointment group).
func (s *AppointmentGroupService) SlotParticipants(ctx context.Context, slotID uint) (map[uint]string, error) {
	out := map[uint]string{}
	if s.db == nil {
		return out, nil
	}
	type row struct {
		ID   uint
		Name string
	}
	var rows []row
	err := s.db.WithContext(ctx).Table("appointment_reservations r").
		Select("r.id, COALESCE(pg.name, u.name) AS name").
		Joins("JOIN users u ON u.id = r.user_id").
		Joins("LEFT JOIN groups pg ON pg.id = r.participant_group_id").
		Where("r.slot_id = ? AND r.workflow_state = ?", slotID, "reserved").
		Scan(&rows).Error
	for _, r := range rows {
		out[r.ID] = r.Name
	}
	return out, err
}
