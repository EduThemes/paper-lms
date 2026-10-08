package service

import (
	"errors"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// The SIS CSV formats beyond users / courses / sections / enrollments
// (G111). Canvas reference: lib/sis/csv/*_importer.rb.

// ---------------------------------------------------------------------------
// terms.csv: term_id, name, status, start_date, end_date
// ---------------------------------------------------------------------------

func (r *sisRun) importTerm(row sisRow) error {
	sisTermID := row.get("term_id")
	name := SanitizePlainText(row.get("name"))
	status := strings.ToLower(row.get("status"))
	if sisTermID == "" {
		return rowErr("term_id is required")
	}
	switch status {
	case "", "active", "deleted":
	default:
		return rowErr("invalid status '%s' (expected active or deleted)", status)
	}
	startAt, err := parseSISDate(row.get("start_date"))
	if err != nil {
		return err
	}
	endAt, err := parseSISDate(row.get("end_date"))
	if err != nil {
		return err
	}
	if startAt != nil && endAt != nil && endAt.Before(*startAt) {
		return rowErr("end_date is before start_date")
	}
	state := "active"
	if status == "deleted" {
		state = "deleted"
	}
	existing, err := r.s.findTermBySISID(r.ctx, sisTermID, r.accountID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return rowErr("error looking up term: %v", err)
	}
	if existing != nil {
		if name != "" {
			existing.Name = name
		}
		existing.WorkflowState = state
		if row.get("start_date") != "" {
			existing.StartAt = startAt
		}
		if row.get("end_date") != "" {
			existing.EndAt = endAt
		}
		if err := r.db().Save(existing).Error; err != nil {
			return rowErr("failed to update term: %v", err)
		}
		return nil
	}
	if name == "" {
		return rowErr("name is required for a new term")
	}
	id := sisTermID
	term := &models.EnrollmentTerm{AccountID: r.accountID, Name: name, SISTermID: &id, StartAt: startAt, EndAt: endAt, WorkflowState: state}
	if err := r.db().Create(term).Error; err != nil {
		return rowErr("failed to create term: %v", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// accounts.csv: account_id, parent_account_id, name, status
//
// Sub-accounts are not part of Paper (single-level tenants), so the only
// row accepted is one naming the importing account itself, which may
// rename it.
// ---------------------------------------------------------------------------

func (r *sisRun) importAccount(row sisRow) error {
	ref := row.get("account_id")
	if ref == "" {
		return rowErr("account_id is required")
	}
	if row.get("parent_account_id") != "" || !r.s.accountRefMatches(r.ctx, ref, r.accountID) {
		return rowErr("sub-accounts are not supported: account_id '%s' must name the importing account", ref)
	}
	if strings.EqualFold(row.get("status"), "deleted") {
		return rowErr("the importing account can't be deleted through SIS")
	}
	if name := SanitizePlainText(row.get("name")); name != "" {
		if err := r.db().Model(&models.Account{}).Where("id = ?", r.accountID).Update("name", name).Error; err != nil {
			return rowErr("failed to update account: %v", err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// logins.csv: user_id, login_id, authentication_provider_id, password,
// existing_user_id | existing_paper_user_id | existing_canvas_user_id
//
// Adds (or updates) an additional login for an existing user (G115).
// user_id is the SIS id the new login carries.
// ---------------------------------------------------------------------------

func (r *sisRun) importLogin(row sisRow) error {
	loginName := row.get("login_id")
	loginSIS := row.get("user_id")
	if loginName == "" {
		return rowErr("login_id is required")
	}
	user, err := r.existingUser(row)
	if err != nil {
		return err
	}
	// A login with a password signs in as its user: an account admin's
	// roster must never be able to add one to a platform super_admin (the
	// logins API refuses it too, callerMayManageUser).
	if user.Role == models.RoleSuperUser {
		return rowErr("user '%s' is a platform administrator and can't be changed by SIS", loginName)
	}
	logins := NewUserLoginService(r.s.db)
	in := UserLoginInput{UniqueID: &loginName}
	if loginSIS != "" {
		in.SISUserID = &loginSIS
	}
	if pw := row.values["password"]; pw != "" {
		in.Password = &pw
	}
	if ap := row.get("authentication_provider_id"); ap != "" {
		id, perr := strconv.ParseUint(ap, 10, 64)
		if perr != nil {
			return rowErr("authentication_provider_id '%s' is not a provider id", ap)
		}
		pid := uint(id)
		in.AuthenticationProviderID = &pid
	}
	// Update the user's login with this name if it already exists.
	var existing models.UserLogin
	err = r.db().Where("user_id = ? AND unique_id_normalized = ? AND workflow_state <> ?",
		user.ID, models.NormalizeLoginID(loginName), models.UserLoginDeleted).First(&existing).Error
	if err == nil {
		if _, uerr := logins.Update(r.ctx, user.ID, existing.ID, r.accountID, in); uerr != nil {
			return rowErr("login_id '%s': %v", loginName, uerr)
		}
		return nil
	}
	if strings.EqualFold(user.LoginID, loginName) {
		return nil // the user's primary login already
	}
	if _, cerr := logins.Create(r.ctx, user.ID, r.accountID, in); cerr != nil {
		return rowErr("login_id '%s': %v", loginName, cerr)
	}
	return nil
}

// existingUser resolves logins.csv's existing_* columns inside the tenant.
func (r *sisRun) existingUser(row sisRow) (*models.User, error) {
	if sis := row.get("existing_user_id"); sis != "" {
		u, err := r.s.findUserBySISID(r.ctx, sis, r.accountID)
		if err != nil {
			return nil, rowErr("existing user with sis_user_id '%s' not found", sis)
		}
		return u, nil
	}
	raw := row.get("existing_paper_user_id")
	if raw == "" {
		raw = row.get("existing_canvas_user_id")
	}
	if raw != "" {
		id, err := strconv.ParseUint(raw, 10, 64)
		if err == nil {
			var u models.User
			if err := r.db().Where("id = ? AND account_id = ?", id, r.accountID).First(&u).Error; err == nil {
				return &u, nil
			}
		}
		return nil, rowErr("existing user '%s' not found", raw)
	}
	return nil, rowErr("existing_user_id or existing_paper_user_id is required")
}

// ---------------------------------------------------------------------------
// admins.csv: user_id, account_id, role, status
//
// Paper has one account-admin role, so role must be AccountAdmin (or an
// admin-based custom role name). A platform super_admin is never changed.
// ---------------------------------------------------------------------------

func (r *sisRun) importAdmin(row sisRow) error {
	sisUserID := row.get("user_id")
	ref := row.get("account_id")
	role := row.get("role")
	status := strings.ToLower(row.get("status"))
	if sisUserID == "" {
		return rowErr("user_id is required")
	}
	if ref != "" && !r.s.accountRefMatches(r.ctx, ref, r.accountID) {
		return rowErr("account_id '%s' does not match the importing account", ref)
	}
	if !strings.EqualFold(role, "AccountAdmin") && !r.adminCustomRole(role) {
		return rowErr("invalid role '%s' (expected AccountAdmin)", role)
	}
	switch status {
	case "", "active", "deleted":
	default:
		return rowErr("invalid status '%s' (expected active or deleted)", status)
	}
	user, err := r.s.findUserBySISID(r.ctx, sisUserID, r.accountID)
	if err != nil {
		return rowErr("user with sis_user_id '%s' not found", sisUserID)
	}
	if user.Role == models.RoleSuperUser {
		return rowErr("user_id '%s' is a platform administrator and can't be changed by SIS", sisUserID)
	}
	switch {
	case status == "deleted" && user.Role == "admin":
		user.Role = "user"
	case status != "deleted":
		user.Role = "admin"
	default:
		return nil
	}
	if err := r.s.userRepo.Update(r.ctx, user); err != nil {
		return rowErr("failed to update user: %v", err)
	}
	return nil
}

func (r *sisRun) adminCustomRole(name string) bool {
	if name == "" {
		return false
	}
	var n int64
	r.db().Model(&models.CustomRole{}).
		Where("account_id = ? AND LOWER(name) = ? AND base_role_type = ? AND workflow_state = ?", r.accountID, strings.ToLower(name), "admin", "active").
		Count(&n)
	return n > 0
}

// ---------------------------------------------------------------------------
// change_sis_id.csv: old_id, new_id, type
// ---------------------------------------------------------------------------

// sisIDTables names the table and SIS-id column per change_sis_id type;
// tenantSQL confines the row to the batch's account. Compile-time
// constants only.
var sisIDTables = map[string]struct{ table, column, tenantSQL string }{
	"user":           {"users", "sis_user_id", "account_id = ?"},
	"course":         {"courses", "sis_course_id", "account_id = ?"},
	"section":        {"course_sections", "sis_section_id", "course_id IN (SELECT id FROM courses WHERE account_id = ?)"},
	"term":           {"enrollment_terms", "sis_term_id", "account_id = ?"},
	"group_category": {"group_categories", "sis_source_id", "(account_id = ? OR course_id IN (SELECT id FROM courses WHERE account_id = ?))"},
	"group":          {"groups", "sis_source_id", "context_type = 'Course' AND context_id IN (SELECT id FROM courses WHERE account_id = ?)"},
}

func (r *sisRun) importChangeSISID(row sisRow) error {
	oldID, newID := row.get("old_id"), row.get("new_id")
	typ := strings.ToLower(row.get("type"))
	if oldID == "" || newID == "" {
		return rowErr("old_id and new_id are required")
	}
	if typ == "account" {
		var acct models.Account
		if err := r.db().First(&acct, r.accountID).Error; err != nil || acct.SISAccountID == nil || *acct.SISAccountID != oldID {
			return rowErr("account with sis id '%s' not found", oldID)
		}
		var n int64
		r.db().Model(&models.Account{}).Where("sis_account_id = ?", newID).Count(&n)
		if n > 0 {
			return rowErr("new_id '%s' is already in use", newID)
		}
		return r.db().Model(&models.Account{}).Where("id = ?", r.accountID).Update("sis_account_id", newID).Error
	}
	spec, ok := sisIDTables[typ]
	if !ok {
		return rowErr("invalid type '%s' (expected user, course, section, term, group_category, group or account)", typ)
	}
	args := []interface{}{r.accountID}
	if strings.Count(spec.tenantSQL, "?") == 2 {
		args = append(args, r.accountID)
	}
	q := func() *gorm.DB {
		return r.db().Table(spec.table).Where(spec.tenantSQL, args...)
	}
	var n int64
	if err := q().Where(spec.column+" = ?", oldID).Count(&n).Error; err != nil || n == 0 {
		return rowErr("%s with sis id '%s' not found", typ, oldID)
	}
	// Uniqueness: users/courses/sections are unique deployment-wide;
	// the rest per tenant.
	var taken int64
	switch typ {
	case "user", "course", "section":
		r.db().Table(spec.table).Where(spec.column+" = ?", newID).Count(&taken)
	default:
		q().Where(spec.column+" = ?", newID).Count(&taken)
	}
	if taken > 0 {
		return rowErr("new_id '%s' is already in use", newID)
	}
	return q().Where(spec.column+" = ?", oldID).Update(spec.column, newID).Error
}

// ---------------------------------------------------------------------------
// user_observers.csv: observer_id, student_id, status
// ---------------------------------------------------------------------------

func (r *sisRun) importUserObserver(row sisRow) error {
	obsSIS, stuSIS := row.get("observer_id"), row.get("student_id")
	status := strings.ToLower(row.get("status"))
	if obsSIS == "" || stuSIS == "" {
		return rowErr("observer_id and student_id are required")
	}
	switch status {
	case "", "active", "deleted":
	default:
		return rowErr("invalid status '%s' (expected active or deleted)", status)
	}
	if r.s.observers == nil {
		return rowErr("observer links are not available")
	}
	observer, err := r.s.findUserBySISID(r.ctx, obsSIS, r.accountID)
	if err != nil {
		return rowErr("observer with sis_user_id '%s' not found", obsSIS)
	}
	student, err := r.s.findUserBySISID(r.ctx, stuSIS, r.accountID)
	if err != nil {
		return rowErr("student with sis_user_id '%s' not found", stuSIS)
	}
	if observer.ID == student.ID {
		return rowErr("a user can't observe themselves")
	}
	if status == "deleted" {
		if err := r.s.observers.UnlinkObserver(r.ctx, observer.ID, student.ID); err != nil && !strings.Contains(err.Error(), "not linked") {
			return rowErr("failed to unlink observer: %v", err)
		}
		return nil
	}
	if err := r.s.observers.LinkObserverToStudent(r.ctx, observer.ID, student.ID); err != nil && !strings.Contains(err.Error(), "already linked") {
		return rowErr("failed to link observer: %v", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// group_categories.csv: group_category_id, account_id, course_id,
// category_name, status
// ---------------------------------------------------------------------------

func (r *sisRun) importGroupCategory(row sisRow) error {
	sisID := row.get("group_category_id")
	name := SanitizePlainText(row.get("category_name"))
	status := strings.ToLower(row.get("status"))
	if sisID == "" {
		return rowErr("group_category_id is required")
	}
	switch status {
	case "", "active", "deleted":
	default:
		return rowErr("invalid status '%s' (expected active or deleted)", status)
	}
	if ref := row.get("account_id"); ref != "" && !r.s.accountRefMatches(r.ctx, ref, r.accountID) {
		return rowErr("account_id '%s' does not match the importing account", ref)
	}
	var courseID *uint
	if sisCourse := row.get("course_id"); sisCourse != "" {
		c, err := r.s.findCourseBySISID(r.ctx, sisCourse, r.accountID)
		if err != nil {
			return rowErr("course with sis_course_id '%s' not found", sisCourse)
		}
		courseID = &c.ID
	}
	state := "active"
	if status == "deleted" {
		state = "deleted"
	}
	existing, err := r.findGroupCategory(sisID)
	if err != nil {
		return err
	}
	if existing != nil {
		if name != "" {
			existing.Name = name
		}
		existing.WorkflowState = state
		return r.db().Save(existing).Error
	}
	if name == "" {
		return rowErr("category_name is required for a new group category")
	}
	cat := &models.GroupCategory{Name: name, WorkflowState: state, SISSourceID: &sisID}
	if courseID != nil {
		cat.CourseID = courseID
	} else {
		acct := r.accountID
		cat.AccountID = &acct
	}
	return r.db().Create(cat).Error
}

func (r *sisRun) findGroupCategory(sisID string) (*models.GroupCategory, error) {
	var cat models.GroupCategory
	err := r.db().Where("sis_source_id = ? AND (account_id = ? OR course_id IN (SELECT id FROM courses WHERE account_id = ?))",
		sisID, r.accountID, r.accountID).First(&cat).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, rowErr("error looking up group category: %v", err)
	}
	return &cat, nil
}

// ---------------------------------------------------------------------------
// groups.csv: group_id, group_category_id, account_id, course_id, name,
// status
//
// Paper groups live in a course; a group without group_category_id goes
// into the course's "Student Groups" category (Canvas's default).
// ---------------------------------------------------------------------------

func (r *sisRun) importGroup(row sisRow) error {
	sisID := row.get("group_id")
	name := SanitizePlainText(row.get("name"))
	status := strings.ToLower(row.get("status"))
	if sisID == "" {
		return rowErr("group_id is required")
	}
	switch status {
	case "", "available", "active", "deleted":
	default:
		return rowErr("invalid status '%s' (expected available or deleted)", status)
	}
	if ref := row.get("account_id"); ref != "" && !r.s.accountRefMatches(r.ctx, ref, r.accountID) {
		return rowErr("account_id '%s' does not match the importing account", ref)
	}
	var course *models.Course
	if sisCourse := row.get("course_id"); sisCourse != "" {
		c, err := r.s.findCourseBySISID(r.ctx, sisCourse, r.accountID)
		if err != nil {
			return rowErr("course with sis_course_id '%s' not found", sisCourse)
		}
		course = c
	}
	var cat *models.GroupCategory
	if catSIS := row.get("group_category_id"); catSIS != "" {
		c, err := r.findGroupCategory(catSIS)
		if err != nil {
			return err
		}
		if c == nil {
			return rowErr("group category '%s' not found", catSIS)
		}
		cat = c
		if course == nil && c.CourseID != nil {
			var cc models.Course
			if err := r.db().Where("id = ? AND account_id = ?", *c.CourseID, r.accountID).First(&cc).Error; err == nil {
				course = &cc
			}
		}
		if course != nil && (c.CourseID == nil || *c.CourseID != course.ID) {
			return rowErr("group category '%s' is not in course '%s'", catSIS, row.get("course_id"))
		}
	}
	state := "available"
	if status == "deleted" {
		state = "deleted"
	}

	var existing models.Group
	err := r.db().Where("sis_source_id = ? AND context_type = 'Course' AND context_id IN (SELECT id FROM courses WHERE account_id = ?)",
		sisID, r.accountID).First(&existing).Error
	if err == nil {
		if name != "" {
			existing.Name = name
		}
		existing.WorkflowState = state
		return r.db().Save(&existing).Error
	}
	if course == nil {
		return rowErr("course_id is required (account-level groups are not supported)")
	}
	if name == "" {
		return rowErr("name is required for a new group")
	}
	if cat == nil {
		c, err := r.defaultStudentGroupsCategory(course.ID)
		if err != nil {
			return err
		}
		cat = c
	}
	g := &models.Group{
		GroupCategoryID: cat.ID, Name: name, ContextType: "Course", ContextID: course.ID,
		WorkflowState: state, SISSourceID: &sisID, JoinLevel: "invitation_only",
	}
	return r.db().Create(g).Error
}

func (r *sisRun) defaultStudentGroupsCategory(courseID uint) (*models.GroupCategory, error) {
	var cat models.GroupCategory
	err := r.db().Where("course_id = ? AND name = ? AND workflow_state = ?", courseID, "Student Groups", "active").First(&cat).Error
	if err == nil {
		return &cat, nil
	}
	cat = models.GroupCategory{CourseID: &courseID, Name: "Student Groups", WorkflowState: "active"}
	if err := r.db().Create(&cat).Error; err != nil {
		return nil, rowErr("failed to create the Student Groups category: %v", err)
	}
	return &cat, nil
}

// ---------------------------------------------------------------------------
// group_memberships.csv: group_id, user_id, status
// ---------------------------------------------------------------------------

func (r *sisRun) importGroupMembership(row sisRow) error {
	groupSIS, userSIS := row.get("group_id"), row.get("user_id")
	status := strings.ToLower(row.get("status"))
	if groupSIS == "" || userSIS == "" {
		return rowErr("group_id and user_id are required")
	}
	switch status {
	case "", "accepted", "active", "deleted":
	default:
		return rowErr("invalid status '%s' (expected accepted or deleted)", status)
	}
	var group models.Group
	if err := r.db().Where("sis_source_id = ? AND context_type = 'Course' AND context_id IN (SELECT id FROM courses WHERE account_id = ?)",
		groupSIS, r.accountID).First(&group).Error; err != nil {
		return rowErr("group '%s' not found", groupSIS)
	}
	user, err := r.s.findUserBySISID(r.ctx, userSIS, r.accountID)
	if err != nil {
		return rowErr("user with sis_user_id '%s' not found", userSIS)
	}
	var m models.GroupMembership
	err = r.db().Where("group_id = ? AND user_id = ?", group.ID, user.ID).First(&m).Error
	if status == "deleted" {
		if err == nil {
			m.WorkflowState = "deleted"
			return r.db().Save(&m).Error
		}
		return nil
	}
	if err == nil {
		m.WorkflowState = "accepted"
		return r.db().Save(&m).Error
	}
	return r.db().Create(&models.GroupMembership{GroupID: group.ID, UserID: user.ID, WorkflowState: "accepted"}).Error
}
