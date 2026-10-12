package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/auth/initialpassword"
	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// sisRun carries the state of one batch run: counters, errors and the
// SIS objects the batch touched (for batch mode).
type sisRun struct {
	ctx       context.Context
	s         *SISImportService
	batch     *models.SISBatch
	accountID uint

	processed int
	errors    int
	counts    map[string]int
	fileNames []string

	// Touched objects, for batch mode.
	seenCourses     map[uint]bool
	seenSections    map[uint]bool
	seenEnrollments map[uint]bool
	// Batch mode only removes the kinds of object the batch supplied
	// (Canvas: supplied_batches), so a batch with just enrollments.csv
	// never deletes the term's courses.
	sawCourses     bool
	sawSections    bool
	sawEnrollments bool
}

func newSISRun(ctx context.Context, s *SISImportService, batch *models.SISBatch) *sisRun {
	return &sisRun{
		ctx: ctx, s: s, batch: batch, accountID: batch.AccountID,
		counts:          map[string]int{},
		seenCourses:     map[uint]bool{},
		seenSections:    map[uint]bool{},
		seenEnrollments: map[uint]bool{},
	}
}

func (r *sisRun) fail(line int, file, msg string) {
	r.errors++
	r.s.recordError(r.ctx, r.batch.ID, line, msg, file)
}

// done counts a processed row and periodically saves progress.
func (r *sisRun) done(importType string) {
	r.processed++
	r.counts[importType]++
	if r.processed%50 == 0 && r.batch.TotalRows > 0 {
		r.batch.ProcessedRows = r.processed
		r.batch.Progress = (r.processed * 99) / r.batch.TotalRows
		_ = r.s.batchRepo.Update(r.ctx, r.batch)
	}
}

func (r *sisRun) db() *gorm.DB { return r.s.db.WithContext(r.ctx) }

// importFile dispatches a parsed CSV to its importer.
func (r *sisRun) importFile(pf *sisParsedFile) {
	r.fileNames = append(r.fileNames, pf.name)
	var handle func(sisRow) error
	switch pf.importType {
	case "users":
		handle = r.importUser
	case "courses":
		r.sawCourses = true
		handle = r.importCourse
	case "sections":
		r.sawSections = true
		handle = r.importSection
	case "enrollments":
		r.sawEnrollments = true
		handle = r.importEnrollment
	case "terms":
		handle = r.importTerm
	case "accounts":
		handle = r.importAccount
	case "logins":
		handle = r.importLogin
	case "admins":
		handle = r.importAdmin
	case "change_sis_id":
		handle = r.importChangeSISID
	case "user_observers":
		handle = r.importUserObserver
	case "group_categories":
		handle = r.importGroupCategory
	case "groups":
		handle = r.importGroup
	case "group_memberships":
		handle = r.importGroupMembership
	case "xlists":
		handle = r.importXlist
	default:
		r.fail(0, pf.name, "unknown import type "+pf.importType)
		return
	}
	for _, row := range pf.rows {
		if msg, bad := row.values["__error"]; bad {
			r.fail(row.line, pf.name, "failed to read row: "+msg)
			continue
		}
		if err := handle(row); err != nil {
			r.fail(row.line, pf.name, err.Error())
			continue
		}
		r.done(pf.importType)
	}
}

func rowErr(format string, args ...interface{}) error { return fmt.Errorf(format, args...) }

// ---------------------------------------------------------------------------
// users.csv
// ---------------------------------------------------------------------------

func (r *sisRun) importUser(row sisRow) error {
	sisUserID := row.get("user_id")
	loginID := row.get("login_id")
	password := row.values["password"]
	firstName := row.get("first_name")
	lastName := row.get("last_name")
	fullName := row.get("full_name")
	sortableName := row.get("sortable_name")
	shortName := row.get("short_name")
	email := row.get("email")
	status := strings.ToLower(row.get("status"))

	if sisUserID == "" {
		return rowErr("user_id is required")
	}
	switch status {
	case "", "active", "deleted", "suspended":
	default:
		return rowErr("invalid status '%s' (expected active, deleted or suspended)", status)
	}
	name := fullName
	if name == "" {
		name = strings.TrimSpace(firstName + " " + lastName)
	}
	name = SanitizePlainText(name)

	existing, findErr := r.s.findUserBySISID(r.ctx, sisUserID, r.accountID)
	if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return rowErr("error looking up user: %v", findErr)
	}

	if status == "deleted" || status == "suspended" {
		if existing == nil {
			return nil // nothing to remove
		}
		if existing.Role == models.RoleSuperUser {
			return rowErr("user_id '%s' is a platform administrator and can't be changed by SIS", sisUserID)
		}
		if !existing.Suspended {
			existing.Suspended = true
			existing.SuspendedBySIS = true
		}
		if err := r.s.userRepo.Update(r.ctx, existing); err != nil {
			return rowErr("failed to update user: %v", err)
		}
		if status == "deleted" {
			// Canvas removes a deleted SIS user's enrollments in the
			// account. Saved one by one through the repo so
			// Enrollment.AfterSave drops linked observers' access too.
			var es []models.Enrollment
			r.db().Where("user_id = ? AND workflow_state <> ? AND course_id IN (SELECT id FROM courses WHERE account_id = ?)",
				existing.ID, models.EnrollmentDeleted, r.accountID).Find(&es)
			for i := range es {
				es[i].WorkflowState = models.EnrollmentDeleted
				if err := r.s.enrollmentRepo.Update(r.ctx, &es[i]); err != nil {
					return rowErr("failed to remove enrollment: %v", err)
				}
			}
		}
		return nil
	}

	if existing != nil {
		// The roster can't rewrite a platform super_admin's login or
		// email (an email change is a password-reset takeover); the
		// admin user API refuses it the same way.
		if existing.Role == models.RoleSuperUser {
			return rowErr("user_id '%s' is a platform administrator and can't be changed by SIS", sisUserID)
		}
		if name != "" {
			existing.Name = name
		}
		if loginID != "" && loginID != existing.LoginID {
			// Unique across primary and additional logins (G115).
			taken, err := NewAdminUserService(r.s.db).loginTaken(r.ctx, loginID, existing.ID)
			if err != nil || taken {
				return rowErr("login_id '%s' is already in use", loginID)
			}
			existing.LoginID = loginID
		}
		if email != "" {
			existing.Email = email
		}
		if sortableName != "" {
			existing.SortableName = SanitizePlainText(sortableName)
		} else if firstName != "" && lastName != "" {
			existing.SortableName = SanitizePlainText(lastName + ", " + firstName)
		}
		if shortName != "" {
			existing.ShortName = SanitizePlainText(shortName)
		} else if firstName != "" {
			existing.ShortName = SanitizePlainText(firstName)
		}
		// Reactivate only what the roster itself suspended.
		if existing.Suspended && existing.SuspendedBySIS {
			existing.Suspended = false
			existing.SuspendedBySIS = false
		}
		if err := r.s.userRepo.Update(r.ctx, existing); err != nil {
			return rowErr("failed to update user: %v", err)
		}
		return nil
	}

	// SIS user IDs are globally unique (idx_users_sis_user_id); a
	// collision with another tenant's user must not be resolved by
	// touching that user. Refuse the row instead.
	if r.s.sisIDTaken(r.ctx, "users", "sis_user_id", sisUserID) {
		return rowErr("user_id '%s' is already in use", sisUserID)
	}
	if loginID == "" {
		return rowErr("login_id is required for a new user")
	}
	if taken, err := NewAdminUserService(r.s.db).loginTaken(r.ctx, loginID, 0); err != nil || taken {
		return rowErr("login_id '%s' is already in use", loginID)
	}
	sortable := name
	if sortableName != "" {
		sortable = SanitizePlainText(sortableName)
	} else if firstName != "" && lastName != "" {
		sortable = SanitizePlainText(lastName + ", " + firstName)
	}
	short := SanitizePlainText(firstName)
	if shortName != "" {
		short = SanitizePlainText(shortName)
	}
	user := &models.User{
		AccountID:    r.accountID,
		Name:         name,
		SortableName: sortable,
		ShortName:    short,
		LoginID:      loginID,
		Email:        email,
		SISUserID:    &sisUserID,
	}
	if password != "" {
		if err := user.HashPassword(password); err != nil {
			return rowErr("failed to hash password: %v", err)
		}
	} else {
		// No password supplied in the CSV — generate a
		// cryptographically random initial password. The plaintext is
		// irrecoverable; the LoginPipeline forces the user to set one
		// (RequiresPasswordReset) or they sign in through SSO.
		initialPW, err := initialpassword.GenerateInitialPassword()
		if err != nil {
			return rowErr("failed to generate initial password: %v", err)
		}
		if err := user.HashPassword(initialPW); err != nil {
			return rowErr("failed to hash default password: %v", err)
		}
		user.RequiresPasswordReset = true
	}
	if err := r.s.userRepo.Create(r.ctx, user); err != nil {
		return rowErr("failed to create user: %v", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// courses.csv
// ---------------------------------------------------------------------------

func (r *sisRun) importCourse(row sisRow) error {
	sisCourseID := row.get("course_id")
	shortName := row.get("short_name")
	longName := row.get("long_name")
	accountRef := row.get("account_id")
	termRef := row.get("term_id")
	status := strings.ToLower(row.get("status"))

	if sisCourseID == "" {
		return rowErr("course_id is required")
	}
	switch status {
	case "", "active", "deleted", "completed", "unpublished", "published":
	default:
		return rowErr("invalid status '%s' (expected active, deleted, completed, unpublished or published)", status)
	}

	// Courses always land in the batch's account. Canvas's courses.csv
	// account_id names a (sub-)account by SIS id; Paper tenants are
	// single-level, so the column is accepted only when it names the
	// batch's own account (by sis_account_id or numeric id).
	if accountRef != "" && !r.s.accountRefMatches(r.ctx, accountRef, r.accountID) {
		return rowErr("account_id '%s' does not match the importing account", accountRef)
	}

	var termID *uint
	if termRef != "" {
		term, err := r.s.findTermBySISID(r.ctx, termRef, r.accountID)
		if err != nil {
			return rowErr("term_id '%s' not found", termRef)
		}
		termID = &term.ID
	}
	startAt, err := parseSISDate(row.get("start_date"))
	if err != nil {
		return err
	}
	endAt, err := parseSISDate(row.get("end_date"))
	if err != nil {
		return err
	}

	existing, findErr := r.s.findCourseBySISID(r.ctx, sisCourseID, r.accountID)
	if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return rowErr("error looking up course: %v", findErr)
	}

	if existing != nil {
		if longName != "" {
			existing.Name = longName
		}
		if shortName != "" {
			existing.CourseCode = shortName
		}
		switch status {
		case "deleted":
			existing.WorkflowState = models.CourseDeleted
		case "completed":
			existing.WorkflowState = models.CourseCompleted
		case "unpublished":
			existing.WorkflowState = models.CourseClaimed
		case "published":
			existing.WorkflowState = models.CourseAvailable
		default: // active: undelete, but keep a teacher's publish choice
			if existing.WorkflowState == models.CourseDeleted {
				existing.WorkflowState = models.CourseAvailable
			}
		}
		if termRef != "" {
			existing.EnrollmentTermID = termID
		}
		if row.get("start_date") != "" {
			existing.StartAt = startAt
		}
		if row.get("end_date") != "" {
			existing.EndAt = endAt
		}
		if err := r.s.courseRepo.Update(r.ctx, existing); err != nil {
			return rowErr("failed to update course: %v", err)
		}
		r.seenCourses[existing.ID] = true
		return nil
	}

	if r.s.sisIDTaken(r.ctx, "courses", "sis_course_id", sisCourseID) {
		return rowErr("course_id '%s' is already in use", sisCourseID)
	}
	state := models.CourseAvailable
	switch status {
	case "deleted":
		state = models.CourseDeleted
	case "completed":
		state = models.CourseCompleted
	case "unpublished":
		state = models.CourseClaimed
	}
	if longName == "" {
		longName = shortName
	}
	if shortName == "" {
		shortName = longName
	}
	course := &models.Course{
		Name:             longName,
		CourseCode:       shortName,
		SISCourseID:      &sisCourseID,
		AccountID:        r.accountID,
		WorkflowState:    state,
		EnrollmentTermID: termID,
		StartAt:          startAt,
		EndAt:            endAt,
	}
	if err := r.s.courseRepo.Create(r.ctx, course); err != nil {
		return rowErr("failed to create course: %v", err)
	}
	r.seenCourses[course.ID] = true
	return nil
}

// ---------------------------------------------------------------------------
// sections.csv
// ---------------------------------------------------------------------------

func (r *sisRun) importSection(row sisRow) error {
	sisSectionID := row.get("section_id")
	sisCourseID := row.get("course_id")
	name := row.get("name")
	status := strings.ToLower(row.get("status"))

	if sisSectionID == "" {
		return rowErr("section_id is required")
	}
	if sisCourseID == "" {
		return rowErr("course_id is required")
	}
	switch status {
	case "", "active", "deleted":
	default:
		return rowErr("invalid status '%s' (expected active or deleted)", status)
	}
	course, err := r.s.findCourseBySISID(r.ctx, sisCourseID, r.accountID)
	if err != nil {
		return rowErr("course with sis_course_id '%s' not found", sisCourseID)
	}
	startAt, endAt, err := sisRowDates(row)
	if err != nil {
		return err
	}
	workflowState := "active"
	if status == "deleted" {
		workflowState = "deleted"
	}

	existing, findErr := r.s.findSectionBySISID(r.ctx, sisSectionID, r.accountID)
	if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return rowErr("error looking up section: %v", findErr)
	}
	if existing != nil {
		if name != "" {
			existing.Name = name
		}
		if existing.NonxlistCourseID != nil {
			// Cross-listed (xlists.csv / UI): the SIS course is the home
			// course; the section stays where it was cross-listed (G108).
			existing.NonxlistCourseID = &course.ID
			if existing.CourseID == course.ID {
				existing.NonxlistCourseID = nil
			}
		} else {
			existing.CourseID = course.ID
		}
		existing.WorkflowState = workflowState
		if row.get("start_date") != "" {
			existing.StartAt = startAt
		}
		if row.get("end_date") != "" {
			existing.EndAt = endAt
		}
		if startAt != nil || endAt != nil {
			// Dates on a section only bound enrollments when restricted,
			// as in Canvas's SIS import (G106).
			existing.RestrictEnrollmentsToSectionDates = true
		}
		if err := r.db().Save(existing).Error; err != nil {
			return rowErr("failed to update section: %v", err)
		}
		r.seenSections[existing.ID] = true
		return nil
	}
	if r.s.sisIDTaken(r.ctx, "course_sections", "sis_section_id", sisSectionID) {
		return rowErr("section_id '%s' is already in use", sisSectionID)
	}
	section := &models.CourseSection{
		CourseID:      course.ID,
		Name:          name,
		SISSectionID:  &sisSectionID,
		WorkflowState: workflowState,
	}
	applySISSectionDates(section, startAt, endAt)
	if err := r.s.sectionRepo.Create(r.ctx, section); err != nil {
		return rowErr("failed to create section: %v", err)
	}
	r.seenSections[section.ID] = true
	return nil
}

// ---------------------------------------------------------------------------
// enrollments.csv
// ---------------------------------------------------------------------------

func (r *sisRun) importEnrollment(row sisRow) error {
	sisCourseID := row.get("course_id")
	sisUserID := row.get("user_id")
	role := row.get("role")
	sisSectionID := row.get("section_id")
	status := row.get("status")
	sisAssociatedUserID := row.get("associated_user_id")

	if sisUserID == "" {
		return rowErr("user_id is required")
	}

	var course *models.Course
	var section *models.CourseSection
	if sisSectionID != "" {
		sec, err := r.s.findSectionBySISID(r.ctx, sisSectionID, r.accountID)
		if err != nil {
			if sisCourseID != "" {
				return rowErr("section with sis_section_id '%s' not found in course '%s'", sisSectionID, sisCourseID)
			}
			return rowErr("section with sis_section_id '%s' not found", sisSectionID)
		}
		section = sec
	}
	if sisCourseID != "" {
		c, err := r.s.findCourseBySISID(r.ctx, sisCourseID, r.accountID)
		if err != nil {
			return rowErr("course with sis_course_id '%s' not found", sisCourseID)
		}
		course = c
	} else if section != nil {
		var c models.Course
		if err := r.db().Where("id = ? AND account_id = ?", section.CourseID, r.accountID).First(&c).Error; err != nil {
			return rowErr("course for section '%s' not found", sisSectionID)
		}
		course = &c
	} else {
		return rowErr("course_id is required")
	}
	if section != nil && section.CourseID != course.ID {
		// A section cross-listed out of the named course (G108) is
		// accepted: the enrollment lives in the course the section now
		// belongs to, as in Canvas. Without this every nightly
		// enrollments.csv row for a cross-listed section failed.
		crosslistedOut := section.NonxlistCourseID != nil && *section.NonxlistCourseID == course.ID
		if !crosslistedOut {
			return rowErr("section with sis_section_id '%s' not found in course '%s'", sisSectionID, sisCourseID)
		}
		var c models.Course
		if err := r.db().Where("id = ? AND account_id = ?", section.CourseID, r.accountID).First(&c).Error; err != nil {
			return rowErr("course for section '%s' not found", sisSectionID)
		}
		course = &c
	}
	startAt, endAt, err := sisRowDates(row)
	if err != nil {
		return err
	}
	limitSection := strings.EqualFold(row.get("limit_section_privileges"), "true")

	user, err := r.s.findUserBySISID(r.ctx, sisUserID, r.accountID)
	if err != nil {
		return rowErr("user with sis_user_id '%s' not found", sisUserID)
	}

	var workflowState models.EnrollmentWorkflow
	switch strings.ToLower(status) {
	case "", "active":
		workflowState = models.EnrollmentActive
	case "deleted":
		workflowState = models.EnrollmentDeleted
	case "inactive":
		workflowState = models.EnrollmentInactive
	case "completed", "concluded":
		workflowState = models.EnrollmentCompleted
	default:
		return rowErr("invalid status '%s' (expected active, deleted, inactive or completed)", status)
	}

	// Unknown roles are refused rather than stored verbatim: a Type
	// outside the five base types fails every course-role check while
	// still passing "is enrolled".
	enrollmentType, roleName, roleID, ok := r.s.mapSISEnrollmentRole(r.ctx, role, r.accountID)
	if !ok {
		return rowErr("invalid role '%s' (expected student, teacher, ta, observer, designer or a custom role name)", role)
	}

	enrollment := &models.Enrollment{
		UserID:        user.ID,
		CourseID:      course.ID,
		Type:          enrollmentType,
		Role:          roleName,
		RoleID:        roleID,
		WorkflowState: workflowState,
		StartAt:       startAt,
		EndAt:         endAt,

		LimitPrivilegesToCourseSection: limitSection,
	}
	if section != nil {
		enrollment.CourseSectionID = &section.ID
	}
	// Observers may name the observed student (a SIS user id).
	if sisAssociatedUserID != "" && enrollmentType == "ObserverEnrollment" {
		observee, err := r.s.findUserBySISID(r.ctx, sisAssociatedUserID, r.accountID)
		if err != nil {
			return rowErr("associated user with sis_user_id '%s' not found", sisAssociatedUserID)
		}
		enrollment.AssociatedUserID = &observee.ID
	}

	// Deduplicate on re-import (user, course, type, associated user) in
	// ANY state so a nightly feed updates rows in place.
	existing, lookupErr := r.s.findSISEnrollment(r.ctx, enrollment)
	if lookupErr != nil && !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return rowErr("error looking up enrollment: %v", lookupErr)
	}
	if existing != nil {
		existing.WorkflowState = workflowState
		existing.Role = roleName
		existing.RoleID = roleID
		if enrollment.CourseSectionID != nil {
			existing.CourseSectionID = enrollment.CourseSectionID
		}
		if _, ok := row.values["start_date"]; ok {
			existing.StartAt = startAt
		}
		if _, ok := row.values["end_date"]; ok {
			existing.EndAt = endAt
		}
		if _, ok := row.values["limit_section_privileges"]; ok {
			existing.LimitPrivilegesToCourseSection = limitSection
		}
		if err := r.s.enrollmentRepo.Update(r.ctx, existing); err != nil {
			return rowErr("failed to update enrollment: %v", err)
		}
		r.seenEnrollments[existing.ID] = true
		return nil
	}
	if err := r.s.enrollmentRepo.Create(r.ctx, enrollment); err != nil {
		return rowErr("failed to create enrollment: %v", err)
	}
	r.seenEnrollments[enrollment.ID] = true
	return nil
}

// sisBaseRoleTypes maps the Canvas enrollments.csv base role keywords
// (and the full type names) onto Paper's enrollment Type.
var sisBaseRoleTypes = map[string]string{
	"student":            "StudentEnrollment",
	"teacher":            "TeacherEnrollment",
	"ta":                 "TaEnrollment",
	"observer":           "ObserverEnrollment",
	"designer":           "DesignerEnrollment",
	"studentenrollment":  "StudentEnrollment",
	"teacherenrollment":  "TeacherEnrollment",
	"taenrollment":       "TaEnrollment",
	"observerenrollment": "ObserverEnrollment",
	"designerenrollment": "DesignerEnrollment",
}

// mapSISEnrollmentRole resolves an enrollments.csv role value into
// (Type, Role, RoleID). Base roles match case-insensitively; anything else
// must name an active custom role of the tenant whose base type is a
// course role: Type is the base type, Role the custom role's name and
// RoleID its id (G102 enforcement). ok=false means the value is invalid.
func (s *SISImportService) mapSISEnrollmentRole(ctx context.Context, role string, accountID uint) (string, string, *uint, bool) {
	key := strings.ToLower(strings.TrimSpace(role))
	if key == "" {
		return "", "", nil, false
	}
	if t, ok := sisBaseRoleTypes[key]; ok {
		return t, t, nil, true
	}
	if s.db == nil {
		return "", "", nil, false
	}
	var custom models.CustomRole
	err := s.db.WithContext(ctx).
		Where("account_id = ? AND LOWER(name) = ? AND workflow_state = ?", accountID, key, "active").
		First(&custom).Error
	if err != nil {
		return "", "", nil, false
	}
	t, ok := sisBaseRoleTypes[strings.ToLower(custom.BaseRoleType)]
	if !ok {
		// e.g. an "admin"-based custom role is not a course enrollment.
		return "", "", nil, false
	}
	id := custom.ID
	return t, custom.Name, &id, true
}

// findSISEnrollment returns the enrollment an enrollments.csv row
// refers to, in any workflow state, preferring a non-deleted row.
func (s *SISImportService) findSISEnrollment(ctx context.Context, e *models.Enrollment) (*models.Enrollment, error) {
	q := s.db.WithContext(ctx).
		Where("user_id = ? AND course_id = ? AND type = ?", e.UserID, e.CourseID, e.Type)
	if e.AssociatedUserID != nil {
		q = q.Where("associated_user_id = ?", *e.AssociatedUserID)
	} else {
		q = q.Where("associated_user_id IS NULL")
	}
	var existing models.Enrollment
	if err := q.Order("(workflow_state = 'deleted') ASC, id ASC").First(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}
