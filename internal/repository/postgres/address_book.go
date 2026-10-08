package postgres

import (
	"context"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// Address book (G093). Canvas scopes who a user may message to the people
// they share a context with: an active enrollment in the same course, or
// an accepted membership in the same group, inside the user's account.
// Admins address anyone in their account. These methods hang off
// RecipientSearchRepository so the inbox has one source of truth.

// coursePeersSQL selects the users who share an active enrollment in a
// course of the account with the caller. Params: callerID, accountID.
const coursePeersSQL = `SELECT peer.user_id
	FROM enrollments mine
	JOIN enrollments peer ON peer.course_id = mine.course_id
	JOIN courses ON courses.id = mine.course_id
	WHERE mine.user_id = ?
	  AND mine.workflow_state = 'active'
	  AND peer.workflow_state = 'active'
	  AND courses.account_id = ?
	  AND courses.workflow_state <> 'deleted'`

// groupPeersSQL selects the users who share an accepted membership in a
// live group of the account with the caller. Params: callerID, accountID,
// accountID.
const groupPeersSQL = `SELECT peer.user_id
	FROM group_memberships mine
	JOIN group_memberships peer ON peer.group_id = mine.group_id
	JOIN groups g ON g.id = mine.group_id
	WHERE mine.user_id = ?
	  AND mine.workflow_state = 'accepted'
	  AND peer.workflow_state = 'accepted'
	  AND g.workflow_state <> 'deleted'
	  AND ((g.context_type = 'Course' AND g.context_id IN (SELECT id FROM courses WHERE account_id = ?))
	       OR (g.context_type = 'Account' AND g.context_id = ?))`

// Unaddressable returns the ids in recipientIDs the caller may NOT message
// (unknown, other tenant, suspended, or sharing no course or group). The
// caller's own id is always addressable. allInAccount (admins) only
// requires a live user of the same account.
func (r *RecipientSearchRepository) Unaddressable(ctx context.Context, callerID, accountID uint, recipientIDs []uint, allInAccount bool) ([]uint, error) {
	want := map[uint]bool{}
	for _, id := range recipientIDs {
		if id != callerID {
			want[id] = true
		}
	}
	if len(want) == 0 {
		return nil, nil
	}
	ids := make([]uint, 0, len(want))
	for id := range want {
		ids = append(ids, id)
	}
	q := r.db.WithContext(ctx).Model(&models.User{}).
		Where("users.id IN ? AND users.account_id = ? AND users.suspended = ?", ids, accountID, false)
	if !allInAccount {
		q = q.Where("(users.id IN ("+coursePeersSQL+") OR users.id IN ("+groupPeersSQL+"))", callerID, accountID, callerID, accountID, accountID)
	}
	var ok []uint
	if err := q.Pluck("users.id", &ok).Error; err != nil {
		return nil, err
	}
	for _, id := range ok {
		delete(want, id)
	}
	out := make([]uint, 0, len(want))
	for id := range want {
		out = append(out, id)
	}
	return out, nil
}

// AddressBookContext is one course or group the caller can message in.
type AddressBookContext struct {
	ContextType string `json:"context_type"` // Course / Group
	ContextID   uint   `json:"context_id"`
	Name        string `json:"name"`
	// Instructor is true when the caller teaches the course (or the
	// group's course): they may message the whole context or a role.
	Instructor bool `json:"instructor"`
}

// ListContexts returns the caller's live courses (active enrollment) and
// groups (accepted membership) in the account, courses first.
func (r *RecipientSearchRepository) ListContexts(ctx context.Context, callerID, accountID uint) ([]AddressBookContext, error) {
	var courses []struct {
		ID         uint
		Name       string
		Instructor bool
	}
	err := r.db.WithContext(ctx).Table("enrollments e").
		Select(`c.id, c.name, BOOL_OR(e.type IN ('TeacherEnrollment','TaEnrollment','DesignerEnrollment')) AS instructor`).
		Joins("JOIN courses c ON c.id = e.course_id").
		Where("e.user_id = ? AND e.workflow_state = 'active' AND c.account_id = ? AND c.workflow_state <> 'deleted'", callerID, accountID).
		Group("c.id, c.name").Order("c.name ASC, c.id ASC").
		Scan(&courses).Error
	if err != nil {
		return nil, err
	}
	out := make([]AddressBookContext, 0, len(courses))
	teaches := map[uint]bool{}
	for _, c := range courses {
		out = append(out, AddressBookContext{ContextType: "Course", ContextID: c.ID, Name: c.Name, Instructor: c.Instructor})
		teaches[c.ID] = c.Instructor
	}
	var groups []struct {
		ID          uint
		Name        string
		ContextType string
		ContextID   uint
	}
	err = r.db.WithContext(ctx).Table("group_memberships gm").
		Select("g.id, g.name, g.context_type, g.context_id").
		Joins("JOIN groups g ON g.id = gm.group_id").
		Where("gm.user_id = ? AND gm.workflow_state = 'accepted' AND g.workflow_state <> 'deleted'", callerID).
		Where("((g.context_type = 'Course' AND g.context_id IN (SELECT id FROM courses WHERE account_id = ?)) OR (g.context_type = 'Account' AND g.context_id = ?))", accountID, accountID).
		Order("g.name ASC, g.id ASC").
		Scan(&groups).Error
	if err != nil {
		return nil, err
	}
	for _, g := range groups {
		out = append(out, AddressBookContext{ContextType: "Group", ContextID: g.ID, Name: g.Name, Instructor: g.ContextType == "Course" && teaches[g.ContextID]})
	}
	return out, nil
}

// CourseRole reports the caller's standing in a course of the account:
// member = any active enrollment, instructor = active teacher/TA/designer.
func (r *RecipientSearchRepository) CourseRole(ctx context.Context, callerID, courseID, accountID uint) (member, instructor bool, err error) {
	var types []string
	err = r.db.WithContext(ctx).Table("enrollments e").
		Joins("JOIN courses c ON c.id = e.course_id").
		Where("e.user_id = ? AND e.course_id = ? AND e.workflow_state = 'active' AND c.account_id = ? AND c.workflow_state <> 'deleted'", callerID, courseID, accountID).
		Pluck("e.type", &types).Error
	if err != nil {
		return false, false, err
	}
	for _, t := range types {
		member = true
		if t == "TeacherEnrollment" || t == "TaEnrollment" || t == "DesignerEnrollment" {
			instructor = true
		}
	}
	return member, instructor, nil
}

// GroupInfo resolves a live group of the account: whether the caller is
// an accepted member, and the course it belongs to (0 for an account
// group). found=false means no such group in the tenant.
func (r *RecipientSearchRepository) GroupInfo(ctx context.Context, callerID, groupID, accountID uint) (found, member bool, courseID uint, err error) {
	var g models.Group
	err = r.db.WithContext(ctx).
		Where("id = ? AND workflow_state <> 'deleted'", groupID).
		Where("((context_type = 'Course' AND context_id IN (SELECT id FROM courses WHERE account_id = ?)) OR (context_type = 'Account' AND context_id = ?))", accountID, accountID).
		Limit(1).Find(&g).Error
	if err != nil || g.ID == 0 {
		return false, false, 0, err
	}
	var n int64
	if err = r.db.WithContext(ctx).Model(&models.GroupMembership{}).
		Where("group_id = ? AND user_id = ? AND workflow_state = 'accepted'", groupID, callerID).
		Count(&n).Error; err != nil {
		return true, false, 0, err
	}
	if g.ContextType == "Course" {
		courseID = g.ContextID
	}
	return true, n > 0, courseID, nil
}

// SectionCourse returns the course of a live section in the account (0
// when there is none).
func (r *RecipientSearchRepository) SectionCourse(ctx context.Context, sectionID, accountID uint) (uint, error) {
	var ids []uint
	err := r.db.WithContext(ctx).Table("course_sections s").
		Joins("JOIN courses c ON c.id = s.course_id").
		Where("s.id = ? AND c.account_id = ? AND COALESCE(s.workflow_state, 'active') <> 'deleted'", sectionID, accountID).
		Pluck("s.course_id", &ids).Error
	if err != nil || len(ids) == 0 {
		return 0, err
	}
	return ids[0], nil
}

// ContextMemberQuery selects members of one course, section or group.
type ContextMemberQuery struct {
	ContextType string // Course, Section, Group
	ContextID   uint
	// EnrollmentTypes limits a course/section to these enrollment types
	// (e.g. StudentEnrollment); empty = every type.
	EnrollmentTypes []string
	Search          string
	ExcludeUserID   uint
	Limit           int // 0 = no limit
}

// ContextMembers lists the live users of a context in the account. The
// caller has already been authorized for the context.
func (r *RecipientSearchRepository) ContextMembers(ctx context.Context, accountID uint, q ContextMemberQuery) ([]models.User, error) {
	db := r.db.WithContext(ctx).Model(&models.User{}).
		Where("users.account_id = ? AND users.suspended = ?", accountID, false)
	switch q.ContextType {
	case "Course", "Section":
		col := "e.course_id"
		if q.ContextType == "Section" {
			col = "e.course_section_id"
		}
		sub := r.db.Table("enrollments e").Select("e.user_id").
			Joins("JOIN courses c ON c.id = e.course_id").
			Where(col+" = ? AND e.workflow_state = 'active' AND c.account_id = ?", q.ContextID, accountID)
		if len(q.EnrollmentTypes) > 0 {
			sub = sub.Where("e.type IN ?", q.EnrollmentTypes)
		}
		db = db.Where("users.id IN (?)", sub)
	case "Group":
		sub := r.db.Table("group_memberships gm").Select("gm.user_id").
			Where("gm.group_id = ? AND gm.workflow_state = 'accepted'", q.ContextID)
		db = db.Where("users.id IN (?)", sub)
	default:
		return nil, nil
	}
	if q.ExcludeUserID != 0 {
		db = db.Where("users.id <> ?", q.ExcludeUserID)
	}
	if s := strings.TrimSpace(q.Search); s != "" {
		pattern := "%" + escapeLike(strings.ToLower(s)) + "%"
		db = db.Where(`(LOWER(users.name) LIKE ? ESCAPE '\' OR LOWER(COALESCE(users.sortable_name, '')) LIKE ? ESCAPE '\')`, pattern, pattern)
	}
	if q.Limit > 0 {
		db = db.Limit(q.Limit)
	}
	var users []models.User
	err := db.Order("users.sortable_name ASC, users.name ASC, users.id ASC").Find(&users).Error
	return users, err
}
