package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// Calendar item types returned by the aggregated calendar.
const (
	CalendarItemEvent           = "event"
	CalendarItemAssignment      = "assignment"
	CalendarItemQuiz            = "quiz"
	CalendarItemAppointment     = "appointment"      // the caller's own reservation
	CalendarItemAppointmentSlot = "appointment_slot" // a slot in a group the caller manages
)

// CalendarAggregateService builds the user calendar (G094/G146): every
// calendar the user can see (personal, each active course and its
// section-specific events, each group, the account), assignment and quiz
// due dates (with the user's own override dates for students), and
// scheduler appointments. It also backs the ICS feeds (G096).
type CalendarAggregateService struct {
	db *gorm.DB
}

func NewCalendarAggregateService(db *gorm.DB) *CalendarAggregateService {
	return &CalendarAggregateService{db: db}
}

// CalendarSection is a course section an instructor can give its own
// event times.
type CalendarSection struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// CalendarContext is one calendar the user can show or hide.
type CalendarContext struct {
	AssetString     string            `json:"asset_string"`
	Type            string            `json:"type"` // user, course, group, account
	ID              uint              `json:"id"`
	Name            string            `json:"name"`
	CanCreateEvents bool              `json:"can_create_calendar_events"`
	IsInstructor    bool              `json:"is_instructor,omitempty"`
	CourseID        *uint             `json:"course_id,omitempty"`
	Sections        []CalendarSection `json:"sections,omitempty"`

	sectionIDs map[uint]bool // the user's own sections (course contexts)
}

// CalendarItem is one entry on the aggregated calendar.
type CalendarItem struct {
	ID                    string     `json:"id"`
	Type                  string     `json:"type"`
	Title                 string     `json:"title"`
	Description           string     `json:"description,omitempty"`
	StartAt               time.Time  `json:"start_at"`
	EndAt                 *time.Time `json:"end_at"`
	AllDay                bool       `json:"all_day"`
	TimeZone              string     `json:"time_zone,omitempty"` // an all-day event's zone; "" = the viewer's
	ContextCode           string     `json:"context_code"`
	ContextName           string     `json:"context_name"`
	LocationName          string     `json:"location_name,omitempty"`
	LocationAddress       string     `json:"location_address,omitempty"`
	HTMLURL               string     `json:"html_url,omitempty"`
	CanEdit               bool       `json:"can_edit"`
	CalendarEventID       *uint      `json:"calendar_event_id,omitempty"`
	ParentCalendarEventID *uint      `json:"parent_calendar_event_id,omitempty"`
	SectionName           string     `json:"section_name,omitempty"`
	SeriesUUID            *string    `json:"series_uuid,omitempty"`
	RRule                 string     `json:"rrule,omitempty"`
	SeriesHead            bool       `json:"series_head,omitempty"`
	AssignmentID          *uint      `json:"assignment_id,omitempty"`
	QuizID                *uint      `json:"quiz_id,omitempty"`
	PointsPossible        *float64   `json:"points_possible,omitempty"`
	AppointmentGroupID    *uint      `json:"appointment_group_id,omitempty"`
	AppointmentSlotID     *uint      `json:"appointment_slot_id,omitempty"`
	ReservationID         *uint      `json:"reservation_id,omitempty"`
	ReservationCount      int        `json:"reservation_count,omitempty"`
	ParticipantsLimit     int        `json:"participants_limit,omitempty"`
	ReservedBy            []string   `json:"reserved_by,omitempty"`
	CreatedAt             time.Time  `json:"-"`
}

// CalendarQuery selects what ListItems returns.
type CalendarQuery struct {
	UserID    uint
	AccountID uint
	IsAdmin   bool
	Start     time.Time
	End       time.Time
	// ContextCodes limits the calendars ("user_5", "course_3",
	// "group_9", "account_1"). Empty means every calendar the user has.
	// Codes the user can't see are ignored.
	ContextCodes []string
	// Types limits the item kinds (event, assignment, appointment —
	// "assignment" covers quizzes, "appointment" covers slots). Empty
	// means all.
	Types map[string]bool
}

func (q CalendarQuery) wants(kind string) bool {
	return len(q.Types) == 0 || q.Types[kind]
}

// MaxCalendarRange bounds one aggregated request.
const MaxCalendarRange = 400 * 24 * time.Hour

type enrollmentRow struct {
	CourseID        uint
	CourseName      string
	CourseState     string
	Type            string
	CourseSectionID *uint
}

// currentEnrollmentRows lists the user's enrollments (in the tenant) that
// are current once the date windows apply: an "active" row whose
// enrollment / section / course / term dates have not started yet or
// have ended, or whose course is concluded, is not a calendar context
// (Canvas shows the calendars of current enrollments only).
func (s *CalendarAggregateService) currentEnrollmentRows(ctx context.Context, userID, accountID uint) ([]enrollmentRow, error) {
	db := s.db.WithContext(ctx)
	var enrollments []models.Enrollment
	if err := db.Joins("JOIN courses c ON c.id = enrollments.course_id").
		Where("enrollments.user_id = ? AND enrollments.workflow_state = ? AND c.account_id = ? AND c.workflow_state <> ?",
			userID, string(models.EnrollmentActive), accountID, string(models.CourseDeleted)).
		Find(&enrollments).Error; err != nil {
		return nil, err
	}
	if len(enrollments) == 0 {
		return nil, nil
	}
	courseIDs := make([]uint, 0, len(enrollments))
	var sectionIDs []uint
	for _, e := range enrollments {
		courseIDs = append(courseIDs, e.CourseID)
		if e.CourseSectionID != nil {
			sectionIDs = append(sectionIDs, *e.CourseSectionID)
		}
	}
	var courses []models.Course
	if err := db.Where("id IN ? AND account_id = ?", courseIDs, accountID).Find(&courses).Error; err != nil {
		return nil, err
	}
	byCourse := make(map[uint]*models.Course, len(courses))
	var termIDs []uint
	for i := range courses {
		byCourse[courses[i].ID] = &courses[i]
		if courses[i].EnrollmentTermID != nil {
			termIDs = append(termIDs, *courses[i].EnrollmentTermID)
		}
	}
	sections := map[uint]*models.CourseSection{}
	if len(sectionIDs) > 0 {
		var secs []models.CourseSection
		if err := db.Where("id IN ?", sectionIDs).Find(&secs).Error; err != nil {
			return nil, err
		}
		for i := range secs {
			sections[secs[i].ID] = &secs[i]
		}
	}
	terms := map[uint]*models.EnrollmentTerm{}
	if len(termIDs) > 0 {
		var ts []models.EnrollmentTerm
		if err := db.Where("id IN ?", termIDs).Find(&ts).Error; err != nil {
			return nil, err
		}
		for i := range ts {
			terms[ts[i].ID] = &ts[i]
		}
	}
	now := time.Now()
	rows := make([]enrollmentRow, 0, len(enrollments))
	for i := range enrollments {
		e := &enrollments[i]
		course := byCourse[e.CourseID]
		if course == nil {
			continue
		}
		var section *models.CourseSection
		if e.CourseSectionID != nil {
			section = sections[*e.CourseSectionID]
		}
		var term *models.EnrollmentTerm
		if course.EnrollmentTermID != nil {
			term = terms[*course.EnrollmentTermID]
		}
		if models.EffectiveEnrollmentState(e, section, course, term, now) != models.EffectiveActive {
			continue
		}
		rows = append(rows, enrollmentRow{CourseID: course.ID, CourseName: course.Name, CourseState: string(course.WorkflowState), Type: e.Type, CourseSectionID: e.CourseSectionID})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].CourseName != rows[j].CourseName {
			return rows[i].CourseName < rows[j].CourseName
		}
		return rows[i].CourseID < rows[j].CourseID
	})
	return rows, nil
}

// ListContexts returns every calendar the user can see, the personal
// calendar first.
func (s *CalendarAggregateService) ListContexts(ctx context.Context, userID, accountID uint, isAdmin bool) ([]CalendarContext, error) {
	db := s.db.WithContext(ctx)
	var user models.User
	if err := db.Select("id", "name", "account_id").Where("id = ? AND account_id = ?", userID, accountID).First(&user).Error; err != nil {
		return nil, err
	}
	out := []CalendarContext{{
		AssetString:     fmt.Sprintf("user_%d", userID),
		Type:            "user",
		ID:              userID,
		Name:            user.Name,
		CanCreateEvents: true,
	}}

	rows, err := s.currentEnrollmentRows(ctx, userID, accountID)
	if err != nil {
		return nil, err
	}
	byCourse := map[uint]*CalendarContext{}
	var order []uint
	for _, r := range rows {
		instructor := r.Type == "TeacherEnrollment" || r.Type == "TaEnrollment" || r.Type == "DesignerEnrollment"
		unpublished := r.CourseState == string(models.CourseClaimed) || r.CourseState == string(models.CourseCreated) || r.CourseState == string(models.CourseUnpublished)
		if unpublished && !instructor {
			continue // students don't see an unpublished course
		}
		c, ok := byCourse[r.CourseID]
		if !ok {
			c = &CalendarContext{
				AssetString: fmt.Sprintf("course_%d", r.CourseID),
				Type:        "course",
				ID:          r.CourseID,
				Name:        r.CourseName,
				sectionIDs:  map[uint]bool{},
			}
			byCourse[r.CourseID] = c
			order = append(order, r.CourseID)
		}
		if instructor && r.Type != "DesignerEnrollment" {
			c.IsInstructor = true
			c.CanCreateEvents = true
		}
		if r.CourseSectionID != nil {
			c.sectionIDs[*r.CourseSectionID] = true
		}
	}
	var instructorCourses []uint
	for _, id := range order {
		if byCourse[id].IsInstructor {
			instructorCourses = append(instructorCourses, id)
		}
	}
	if len(instructorCourses) > 0 {
		var secs []models.CourseSection
		if err := db.Select("id", "name", "course_id").
			Where("course_id IN ? AND workflow_state <> ?", instructorCourses, "deleted").
			Order("name ASC, id ASC").Find(&secs).Error; err != nil {
			return nil, err
		}
		for _, sec := range secs {
			c := byCourse[sec.CourseID]
			c.Sections = append(c.Sections, CalendarSection{ID: sec.ID, Name: sec.Name})
		}
	}
	for _, id := range order {
		out = append(out, *byCourse[id])
	}

	type groupRow struct {
		ID          uint
		Name        string
		ContextType string
		ContextID   uint
	}
	var groups []groupRow
	if err := db.Table("group_memberships gm").
		Select("g.id, g.name, g.context_type, g.context_id").
		Joins("JOIN groups g ON g.id = gm.group_id").
		Where("gm.user_id = ? AND gm.workflow_state = ? AND g.workflow_state <> ?", userID, "accepted", "deleted").
		Where(`(g.context_type = 'Account' AND g.context_id = ?)
			OR (g.context_type = 'Course' AND g.context_id IN (SELECT id FROM courses WHERE account_id = ?))`, accountID, accountID).
		Order("g.name ASC, g.id ASC").
		Scan(&groups).Error; err != nil {
		return nil, err
	}
	for _, g := range groups {
		gc := CalendarContext{
			AssetString:     fmt.Sprintf("group_%d", g.ID),
			Type:            "group",
			ID:              g.ID,
			Name:            g.Name,
			CanCreateEvents: true,
		}
		if g.ContextType == "Course" {
			cid := g.ContextID
			gc.CourseID = &cid
		}
		out = append(out, gc)
	}

	var acct models.Account
	if err := db.Select("id", "name").Where("id = ?", accountID).First(&acct).Error; err == nil {
		out = append(out, CalendarContext{
			AssetString:     fmt.Sprintf("account_%d", accountID),
			Type:            "account",
			ID:              accountID,
			Name:            acct.Name,
			CanCreateEvents: isAdmin,
		})
	}
	return out, nil
}

// selectContexts narrows contexts to the requested codes.
func selectContexts(all []CalendarContext, codes []string) []CalendarContext {
	if len(codes) == 0 {
		return all
	}
	want := map[string]bool{}
	for _, c := range codes {
		want[strings.TrimSpace(c)] = true
	}
	var out []CalendarContext
	for _, c := range all {
		if want[c.AssetString] {
			out = append(out, c)
		}
	}
	return out
}

// ListItems returns the aggregated calendar for the query's date range.
func (s *CalendarAggregateService) ListItems(ctx context.Context, q CalendarQuery) ([]CalendarItem, error) {
	if q.End.Before(q.Start) {
		return nil, calValidation("end_date must not be before start_date")
	}
	if q.End.Sub(q.Start) > MaxCalendarRange {
		return nil, calValidation("the date range is limited to 400 days")
	}
	all, err := s.ListContexts(ctx, q.UserID, q.AccountID, q.IsAdmin)
	if err != nil {
		return nil, err
	}
	sel := selectContexts(all, q.ContextCodes)

	var (
		userSelected    bool
		accountSelected bool
		courses         []CalendarContext
		groupIDs        []uint
		groupNames      = map[uint]string{}
	)
	for _, c := range sel {
		switch c.Type {
		case "user":
			userSelected = true
		case "course":
			courses = append(courses, c)
		case "group":
			groupIDs = append(groupIDs, c.ID)
			groupNames[c.ID] = c.Name
		case "account":
			accountSelected = true
		}
	}
	userName := all[0].Name
	var items []CalendarItem

	if q.wants(CalendarItemEvent) {
		evs, err := s.eventItems(ctx, q, userSelected, accountSelected, courses, groupIDs, groupNames, userName, all)
		if err != nil {
			return nil, err
		}
		items = append(items, evs...)
	}
	if q.wants(CalendarItemAssignment) && len(courses) > 0 {
		as, err := s.assignmentItems(ctx, q, courses)
		if err != nil {
			return nil, err
		}
		items = append(items, as...)
		qs, err := s.quizItems(ctx, q, courses)
		if err != nil {
			return nil, err
		}
		items = append(items, qs...)
	}
	if q.wants(CalendarItemAppointment) || q.wants(CalendarItemEvent) {
		ap, err := s.appointmentItems(ctx, q, userSelected, courses, all)
		if err != nil {
			return nil, err
		}
		items = append(items, ap...)
	}

	for i := range items {
		items[i].StartAt = items[i].StartAt.UTC()
		if items[i].EndAt != nil {
			e := items[i].EndAt.UTC()
			items[i].EndAt = &e
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if !items[i].StartAt.Equal(items[j].StartAt) {
			return items[i].StartAt.Before(items[j].StartAt)
		}
		return items[i].ID < items[j].ID
	})
	return items, nil
}

// overlap is the date-range predicate for rows with start_at/end_at.
const calendarOverlapSQL = "start_at <= ? AND COALESCE(end_at, start_at) >= ?"

func (s *CalendarAggregateService) eventItems(ctx context.Context, q CalendarQuery, userSelected, accountSelected bool,
	courses []CalendarContext, groupIDs []uint, groupNames map[uint]string, userName string, all []CalendarContext) ([]CalendarItem, error) {
	db := s.db.WithContext(ctx)
	var items []CalendarItem
	base := func() *gorm.DB {
		return db.Model(&models.CalendarEvent{}).
			Where("workflow_state <> ?", "deleted").
			Where(calendarOverlapSQL, q.End, q.Start)
	}
	add := func(e models.CalendarEvent, code, name string, canEdit bool) {
		id := e.ID
		it := CalendarItem{
			ID:                    fmt.Sprintf("event_%d", e.ID),
			Type:                  CalendarItemEvent,
			Title:                 e.Title,
			Description:           e.Description,
			StartAt:               e.StartAt,
			EndAt:                 e.EndAt,
			AllDay:                e.AllDay,
			TimeZone:              derefString(e.TimeZone),
			ContextCode:           code,
			ContextName:           name,
			LocationName:          e.LocationName,
			LocationAddress:       e.LocationAddress,
			CanEdit:               canEdit,
			CalendarEventID:       &id,
			ParentCalendarEventID: e.ParentCalendarEventID,
			SeriesUUID:            e.SeriesUUID,
			RRule:                 e.RRule,
			SeriesHead:            e.SeriesHead,
			CreatedAt:             e.CreatedAt,
		}
		items = append(items, it)
	}

	if userSelected {
		var evs []models.CalendarEvent
		if err := base().Where("context_type = ? AND context_id = ?", models.CalendarContextUser, q.UserID).Find(&evs).Error; err != nil {
			return nil, err
		}
		for _, e := range evs {
			add(e, fmt.Sprintf("user_%d", q.UserID), userName, true)
		}
	}

	if len(courses) > 0 {
		courseIDs := make([]uint, 0, len(courses))
		byID := map[uint]CalendarContext{}
		for _, c := range courses {
			courseIDs = append(courseIDs, c.ID)
			byID[c.ID] = c
		}
		// Course events, minus parents that hand out section times.
		var evs []models.CalendarEvent
		if err := base().Where("context_type = ? AND context_id IN ?", models.CalendarContextCourse, courseIDs).
			Where(`NOT EXISTS (SELECT 1 FROM calendar_events ch
				WHERE ch.parent_calendar_event_id = calendar_events.id AND ch.workflow_state <> 'deleted')`).
			Find(&evs).Error; err != nil {
			return nil, err
		}
		for _, e := range evs {
			c := byID[e.ContextID]
			add(e, c.AssetString, c.Name, c.IsInstructor)
		}

		// Section-specific children: instructors see every section of
		// their course; everyone else sees only their own sections.
		type childRow struct {
			models.CalendarEvent
			SectionCourseID uint
			SectionName     string
		}
		var children []childRow
		if err := db.Table("calendar_events").
			Select("calendar_events.*, cs.course_id AS section_course_id, cs.name AS section_name").
			Joins("JOIN course_sections cs ON cs.id = calendar_events.context_id").
			Where("calendar_events.context_type = ? AND calendar_events.workflow_state <> ?", models.CalendarContextCourseSection, "deleted").
			Where("cs.course_id IN ?", courseIDs).
			Where("calendar_events.start_at <= ? AND COALESCE(calendar_events.end_at, calendar_events.start_at) >= ?", q.End, q.Start).
			Scan(&children).Error; err != nil {
			return nil, err
		}
		for _, ch := range children {
			c, ok := byID[ch.SectionCourseID]
			if !ok {
				continue
			}
			if !c.IsInstructor && !c.sectionIDs[ch.ContextID] {
				continue
			}
			add(ch.CalendarEvent, c.AssetString, c.Name, c.IsInstructor)
			items[len(items)-1].SectionName = ch.SectionName
		}
	}

	if len(groupIDs) > 0 {
		var evs []models.CalendarEvent
		if err := base().Where("context_type = ? AND context_id IN ?", models.CalendarContextGroup, groupIDs).Find(&evs).Error; err != nil {
			return nil, err
		}
		for _, e := range evs {
			add(e, fmt.Sprintf("group_%d", e.ContextID), groupNames[e.ContextID], e.CreatedByUserID == q.UserID || q.IsAdmin)
		}
	}

	if accountSelected {
		var evs []models.CalendarEvent
		if err := base().Where("context_type = ? AND context_id = ?", models.CalendarContextAccount, q.AccountID).Find(&evs).Error; err != nil {
			return nil, err
		}
		name := ""
		for _, c := range all {
			if c.Type == "account" {
				name = c.Name
			}
		}
		for _, e := range evs {
			add(e, fmt.Sprintf("account_%d", q.AccountID), name, q.IsAdmin)
		}
	}
	return items, nil
}

type overrideRow struct {
	ID              uint
	AssignmentID    uint
	DueAt           *time.Time
	CourseSectionID *uint
	Title           string
	SectionName     string
}

func (s *CalendarAggregateService) assignmentItems(ctx context.Context, q CalendarQuery, courses []CalendarContext) ([]CalendarItem, error) {
	db := s.db.WithContext(ctx)
	var items []CalendarItem

	var instructorIDs, studentIDs []uint
	byID := map[uint]CalendarContext{}
	for _, c := range courses {
		byID[c.ID] = c
		if c.IsInstructor {
			instructorIDs = append(instructorIDs, c.ID)
		} else {
			studentIDs = append(studentIDs, c.ID)
		}
	}
	// Any assignment whose base date or an active override's date falls
	// in range is a candidate; the effective date is worked out below.
	inRange := `(assignments.due_at BETWEEN ? AND ?) OR EXISTS (
		SELECT 1 FROM assignment_overrides o WHERE o.assignment_id = assignments.id
		AND o.workflow_state = 'active' AND o.due_at BETWEEN ? AND ?)`
	load := func(courseIDs []uint, publishedOnly bool) ([]models.Assignment, error) {
		if len(courseIDs) == 0 {
			return nil, nil
		}
		var as []models.Assignment
		tx := db.Where("course_id IN ? AND workflow_state <> ?", courseIDs, string(models.AssignmentDeleted)).
			Where(inRange, q.Start, q.End, q.Start, q.End)
		if publishedOnly {
			tx = tx.Where("(workflow_state = ? OR (workflow_state = '' AND published = true))", string(models.AssignmentPublished))
		}
		if err := tx.Order("due_at ASC, id ASC").Find(&as).Error; err != nil {
			return nil, err
		}
		return as, nil
	}
	loadOverrides := func(as []models.Assignment) (map[uint][]overrideRow, error) {
		out := map[uint][]overrideRow{}
		if len(as) == 0 {
			return out, nil
		}
		ids := make([]uint, len(as))
		for i, a := range as {
			ids[i] = a.ID
		}
		var rows []overrideRow
		if err := db.Table("assignment_overrides o").
			Select("o.id, o.assignment_id, o.due_at, o.course_section_id, o.title, COALESCE(cs.name, '') AS section_name").
			Joins("LEFT JOIN course_sections cs ON cs.id = o.course_section_id").
			Where("o.assignment_id IN ? AND o.workflow_state = ?", ids, "active").
			Order("o.id ASC").
			Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			out[r.AssignmentID] = append(out[r.AssignmentID], r)
		}
		return out, nil
	}
	item := func(a models.Assignment, due time.Time, suffix string, c CalendarContext) CalendarItem {
		id := a.ID
		title := a.Name
		if suffix != "" {
			title = fmt.Sprintf("%s (%s)", a.Name, suffix)
		}
		key := fmt.Sprintf("assignment_%d", a.ID)
		if suffix != "" {
			key = fmt.Sprintf("assignment_%d_%s", a.ID, strings.ReplaceAll(strings.ToLower(suffix), " ", "_"))
		}
		return CalendarItem{
			ID:             key,
			Type:           CalendarItemAssignment,
			Title:          title,
			StartAt:        due,
			EndAt:          &due,
			ContextCode:    c.AssetString,
			ContextName:    c.Name,
			HTMLURL:        fmt.Sprintf("/courses/%d/assignments/%d", a.CourseID, a.ID),
			CanEdit:        c.IsInstructor,
			AssignmentID:   &id,
			PointsPossible: a.PointsPossible,
			CreatedAt:      a.CreatedAt,
		}
	}
	inWindow := func(t *time.Time) bool {
		return t != nil && !t.Before(q.Start) && !t.After(q.End)
	}

	// Instructors: the base date plus one entry per dated override.
	teach, err := load(instructorIDs, false)
	if err != nil {
		return nil, err
	}
	teachOv, err := loadOverrides(teach)
	if err != nil {
		return nil, err
	}
	for _, a := range teach {
		c := byID[a.CourseID]
		if inWindow(a.DueAt) {
			items = append(items, item(a, *a.DueAt, "", c))
		}
		for _, o := range teachOv[a.ID] {
			if !inWindow(o.DueAt) {
				continue
			}
			label := o.SectionName
			if label == "" {
				label = o.Title
			}
			if label == "" {
				label = "override " + strconv.FormatUint(uint64(o.ID), 10)
			}
			it := item(a, *o.DueAt, label, c)
			it.ID = fmt.Sprintf("assignment_%d_override_%d", a.ID, o.ID)
			items = append(items, it)
		}
	}

	// Students / observers: one entry at the effective due date. An
	// override that applies to the user (their own, or one of their
	// sections) replaces the base date; when several apply the latest
	// wins, and an applicable override with no date means no due date.
	learn, err := load(studentIDs, true)
	if err != nil {
		return nil, err
	}
	learnOv, err := loadOverrides(learn)
	if err != nil {
		return nil, err
	}
	mine := map[uint]bool{}
	if len(learn) > 0 {
		var ids []uint
		if err := db.Table("assignment_override_students").
			Where("user_id = ?", q.UserID).
			Pluck("assignment_override_id", &ids).Error; err != nil {
			return nil, err
		}
		for _, id := range ids {
			mine[id] = true
		}
	}
	for _, a := range learn {
		c := byID[a.CourseID]
		due := a.DueAt
		applied := false
		var latest *time.Time
		noDate := false
		for _, o := range learnOv[a.ID] {
			applies := mine[o.ID] || (o.CourseSectionID != nil && c.sectionIDs[*o.CourseSectionID])
			if !applies {
				continue
			}
			applied = true
			if o.DueAt == nil {
				noDate = true
				continue
			}
			if latest == nil || o.DueAt.After(*latest) {
				d := *o.DueAt
				latest = &d
			}
		}
		if applied {
			due = latest
			if noDate {
				due = nil
			}
		} else if a.OnlyVisibleToOverrides {
			// Assigned only to its overrides' students and sections, none
			// of which is this user: it is not on their calendar (or ICS
			// feed) at the base date — it does not exist for them.
			continue
		}
		if inWindow(due) {
			items = append(items, item(a, *due, "", c))
		}
	}
	return items, nil
}

func (s *CalendarAggregateService) quizItems(ctx context.Context, q CalendarQuery, courses []CalendarContext) ([]CalendarItem, error) {
	var items []CalendarItem
	byID := map[uint]CalendarContext{}
	var ids []uint
	for _, c := range courses {
		byID[c.ID] = c
		ids = append(ids, c.ID)
	}
	var quizzes []models.Quiz
	if err := s.db.WithContext(ctx).
		Where("course_id IN ? AND workflow_state <> ? AND due_at BETWEEN ? AND ?", ids, "deleted", q.Start, q.End).
		Order("due_at ASC, id ASC").
		Find(&quizzes).Error; err != nil {
		return nil, err
	}
	for _, qz := range quizzes {
		c := byID[qz.CourseID]
		// Same published test as the quiz pages (quiz_access.go).
		if !c.IsInstructor && !qz.Published && qz.WorkflowState != "published" {
			continue
		}
		id := qz.ID
		due := *qz.DueAt
		items = append(items, CalendarItem{
			ID:             fmt.Sprintf("quiz_%d", qz.ID),
			Type:           CalendarItemQuiz,
			Title:          qz.Title,
			StartAt:        due,
			EndAt:          &due,
			ContextCode:    c.AssetString,
			ContextName:    c.Name,
			HTMLURL:        fmt.Sprintf("/courses/%d/quizzes/%d", qz.CourseID, qz.ID),
			CanEdit:        c.IsInstructor,
			QuizID:         &id,
			PointsPossible: qz.PointsPossible,
			CreatedAt:      qz.CreatedAt,
		})
	}
	return items, nil
}

func (s *CalendarAggregateService) appointmentItems(ctx context.Context, q CalendarQuery, userSelected bool, courses []CalendarContext, all []CalendarContext) ([]CalendarItem, error) {
	db := s.db.WithContext(ctx)
	var items []CalendarItem
	names := map[uint]string{}
	for _, c := range all {
		if c.Type == "course" {
			names[c.ID] = c.Name
		}
	}
	codeFor := func(courseID uint) (string, string) {
		if n, ok := names[courseID]; ok {
			return fmt.Sprintf("course_%d", courseID), n
		}
		return fmt.Sprintf("user_%d", q.UserID), all[0].Name
	}

	// The caller's own reservations (or their student group's) live on
	// the personal calendar.
	if userSelected {
		type resRow struct {
			ReservationID   uint
			SlotID          uint
			GroupID         uint
			StartAt         time.Time
			EndAt           time.Time
			Title           string
			Description     string
			CourseID        uint
			LocationName    string
			LocationAddress string
		}
		var rows []resRow
		if err := db.Table("appointment_reservations r").
			Select(`r.id AS reservation_id, r.slot_id, r.group_id, s.start_at, s.end_at,
				g.title, g.description, g.course_id, g.location_name, g.location_address`).
			Joins("JOIN appointment_slots s ON s.id = r.slot_id").
			Joins("JOIN appointment_groups g ON g.id = r.group_id").
			Where("r.workflow_state = ? AND g.workflow_state <> ?", "reserved", "deleted").
			Where("g.course_id IN (SELECT id FROM courses WHERE account_id = ?)", q.AccountID).
			Where(`r.user_id = ? OR r.participant_group_id IN (
				SELECT group_id FROM group_memberships WHERE user_id = ? AND workflow_state = 'accepted')`, q.UserID, q.UserID).
			Where("s.start_at <= ? AND s.end_at >= ?", q.End, q.Start).
			Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			code, name := codeFor(r.CourseID)
			gid, sid, rid := r.GroupID, r.SlotID, r.ReservationID
			end := r.EndAt
			items = append(items, CalendarItem{
				ID:                 fmt.Sprintf("reservation_%d", r.ReservationID),
				Type:               CalendarItemAppointment,
				Title:              r.Title,
				Description:        r.Description,
				StartAt:            r.StartAt,
				EndAt:              &end,
				ContextCode:        code,
				ContextName:        name,
				LocationName:       r.LocationName,
				LocationAddress:    r.LocationAddress,
				HTMLURL:            fmt.Sprintf("/courses/%d/appointment_groups", r.CourseID),
				AppointmentGroupID: &gid,
				AppointmentSlotID:  &sid,
				ReservationID:      &rid,
			})
		}
	}

	// Slots of appointment groups in courses the caller teaches.
	var teach []uint
	byID := map[uint]CalendarContext{}
	for _, c := range courses {
		if c.IsInstructor {
			teach = append(teach, c.ID)
			byID[c.ID] = c
		}
	}
	if len(teach) == 0 {
		return items, nil
	}
	type slotRow struct {
		SlotID            uint
		GroupID           uint
		StartAt           time.Time
		EndAt             time.Time
		ParticipantsLimit *int
		Title             string
		CourseID          uint
		PerAppointment    int
		LocationName      string
		LocationAddress   string
	}
	var slots []slotRow
	if err := db.Table("appointment_slots s").
		Select(`s.id AS slot_id, s.group_id, s.start_at, s.end_at, s.participants_limit,
			g.title, g.course_id, g.participants_per_appointment AS per_appointment, g.location_name, g.location_address`).
		Joins("JOIN appointment_groups g ON g.id = s.group_id").
		Where("g.workflow_state <> ? AND g.course_id IN ?", "deleted", teach).
		Where("s.start_at <= ? AND s.end_at >= ?", q.End, q.Start).
		Order("s.start_at ASC, s.id ASC").
		Scan(&slots).Error; err != nil {
		return nil, err
	}
	if len(slots) == 0 {
		return items, nil
	}
	slotIDs := make([]uint, len(slots))
	for i, s := range slots {
		slotIDs[i] = s.SlotID
	}
	type whoRow struct {
		SlotID    uint
		UserName  string
		GroupName string
	}
	var who []whoRow
	if err := db.Table("appointment_reservations r").
		Select("r.slot_id, u.name AS user_name, COALESCE(pg.name, '') AS group_name").
		Joins("JOIN users u ON u.id = r.user_id").
		Joins("LEFT JOIN groups pg ON pg.id = r.participant_group_id").
		Where("r.slot_id IN ? AND r.workflow_state = ?", slotIDs, "reserved").
		Order("r.reserved_at ASC").
		Scan(&who).Error; err != nil {
		return nil, err
	}
	bySlot := map[uint][]string{}
	for _, w := range who {
		n := w.UserName
		if w.GroupName != "" {
			n = w.GroupName
		}
		bySlot[w.SlotID] = append(bySlot[w.SlotID], n)
	}
	for _, sl := range slots {
		c := byID[sl.CourseID]
		limit := sl.PerAppointment
		if sl.ParticipantsLimit != nil {
			limit = *sl.ParticipantsLimit
		}
		gid, sid := sl.GroupID, sl.SlotID
		end := sl.EndAt
		items = append(items, CalendarItem{
			ID:                 fmt.Sprintf("slot_%d", sl.SlotID),
			Type:               CalendarItemAppointmentSlot,
			Title:              sl.Title,
			StartAt:            sl.StartAt,
			EndAt:              &end,
			ContextCode:        c.AssetString,
			ContextName:        c.Name,
			LocationName:       sl.LocationName,
			LocationAddress:    sl.LocationAddress,
			HTMLURL:            fmt.Sprintf("/courses/%d/appointment_groups", sl.CourseID),
			CanEdit:            true,
			AppointmentGroupID: &gid,
			AppointmentSlotID:  &sid,
			ReservationCount:   len(bySlot[sl.SlotID]),
			ParticipantsLimit:  limit,
			ReservedBy:         bySlot[sl.SlotID],
		})
	}
	return items, nil
}

// SectionCourseID returns the course a section belongs to, scoped to the
// tenant (0 when it isn't there).
func (s *CalendarAggregateService) SectionCourseID(ctx context.Context, sectionID, accountID uint) uint {
	var sec models.CourseSection
	if err := s.db.WithContext(ctx).Select("id", "course_id").
		Where("id = ? AND workflow_state <> ? AND course_id IN (SELECT id FROM courses WHERE account_id = ?)", sectionID, "deleted", accountID).
		First(&sec).Error; err != nil {
		return 0
	}
	return sec.CourseID
}

// IsGroupMember reports whether the user is an accepted member of a group
// in the tenant.
func (s *CalendarAggregateService) IsGroupMember(ctx context.Context, groupID, userID, accountID uint) bool {
	var n int64
	s.db.WithContext(ctx).Table("group_memberships gm").
		Joins("JOIN groups g ON g.id = gm.group_id").
		Where("gm.group_id = ? AND gm.user_id = ? AND gm.workflow_state = ? AND g.workflow_state <> ?", groupID, userID, "accepted", "deleted").
		Where(`(g.context_type = 'Account' AND g.context_id = ?)
			OR (g.context_type = 'Course' AND g.context_id IN (SELECT id FROM courses WHERE account_id = ?))`, accountID, accountID).
		Count(&n)
	return n > 0
}

// UserTimeZoneName returns the user's profile zone name when it is a
// valid IANA zone, else "".
func (s *CalendarAggregateService) UserTimeZoneName(ctx context.Context, userID uint) string {
	var u models.User
	if err := s.db.WithContext(ctx).Select("id", "time_zone").Where("id = ?", userID).First(&u).Error; err != nil {
		return ""
	}
	if !ValidCalendarTimeZone(u.TimeZone) {
		return ""
	}
	return u.TimeZone
}

// UserTimeZone returns the user's configured time zone (UTC fallback).
func (s *CalendarAggregateService) UserTimeZone(ctx context.Context, userID uint) *time.Location {
	var u models.User
	if err := s.db.WithContext(ctx).Select("id", "time_zone").Where("id = ?", userID).First(&u).Error; err != nil {
		return time.UTC
	}
	return LoadCalendarLocation(u.TimeZone)
}
