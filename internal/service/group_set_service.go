package service

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"sort"
	"strconv"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// Group set tooling (G100): group leaders, assigning unassigned students,
// creating groups in bulk, and CSV import / export of group memberships.
// Semantics follow Canvas (GroupLeadership, GroupCategory
// #assign_unassigned_members, GroupAndMembershipImporter) with original
// code.

// SetUserRepository wires the user lookups the roster tools need (names,
// login ids and SIS ids of the course's students).
func (s *GroupService) SetUserRepository(userRepo repository.UserRepository) {
	s.userRepo = userRepo
}

// Group set errors.
var (
	ErrLeaderNotMember         = errors.New("the group leader must be a member of the group")
	ErrNoGroupsInSet           = errors.New("this group set has no groups to assign students to")
	ErrGroupsNotEmptyBySection = errors.New("groups must be empty to assign students by section")
	ErrGroupsLimitedBySection  = errors.New("groups cannot have size limits to assign students by section")
	ErrTooFewGroupsBySection   = errors.New("there must be at least as many groups as sections to assign students by section")
	ErrGroupSetNotInCourse     = errors.New("this tool is only available for course group sets")
	ErrAutoAssignStudentGroups = errors.New("students organize their own groups in this group set, so it cannot be auto-assigned")
	ErrImportEmpty             = errors.New("the CSV file is empty")
	ErrImportColumns           = errors.New("the CSV needs a user column (canvas_user_id, user_id, login_id or email) and a group column (canvas_group_id or group_name)")
	ErrImportTooLarge          = fmt.Errorf("the CSV has more than %d rows", maxGroupImportRows)
)

const (
	// maxGroupsPerSet caps bulk group creation (Canvas's
	// max_groups_in_new_category default).
	maxGroupsPerSet = 200
	// maxGroupImportRows bounds one CSV import.
	maxGroupImportRows = 10000
	// maxImportSkips caps how many skipped rows are reported back.
	maxImportSkips = 200
)

// ---- Leadership ----

// SetLeader makes userID the group's leader, or clears the leader when
// userID is nil. The leader must be an accepted member.
func (s *GroupService) SetLeader(ctx context.Context, group *models.Group, userID *uint) error {
	if userID != nil {
		m, err := s.membershipRepo.FindByGroupAndUser(ctx, group.ID, *userID)
		if err != nil || m == nil || m.WorkflowState != "accepted" {
			return ErrLeaderNotMember
		}
	}
	if err := s.groupRepo.SetLeader(ctx, group.ID, userID); err != nil {
		return err
	}
	group.LeaderID = userID
	return nil
}

// syncLeadership runs after a group's membership changes (Canvas
// GroupLeadership#member_changed_event): a leader who is no longer a
// member loses the role, and an empty leadership spot is filled per the
// group set's auto_leader ("first" member or a "random" member).
func (s *GroupService) syncLeadership(ctx context.Context, group *models.Group, category *models.GroupCategory) {
	if group == nil {
		return
	}
	if group.LeaderID != nil {
		m, err := s.membershipRepo.FindByGroupAndUser(ctx, group.ID, *group.LeaderID)
		if err != nil || m == nil || m.WorkflowState != "accepted" {
			if s.groupRepo.SetLeader(ctx, group.ID, nil) == nil {
				group.LeaderID = nil
			}
		}
	}
	if group.LeaderID != nil || category == nil || (category.AutoLeader != "first" && category.AutoLeader != "random") {
		return
	}
	members, err := s.membershipRepo.ListByGroupID(ctx, group.ID, repository.PaginationParams{Page: 1, PerPage: 1000})
	if err != nil || members == nil {
		return
	}
	var candidates []uint
	for _, m := range members.Items { // ordered by membership id: first joiner first
		if m.WorkflowState == "accepted" {
			candidates = append(candidates, m.UserID)
		}
	}
	if len(candidates) == 0 {
		return
	}
	pick := candidates[0]
	if category.AutoLeader == "random" {
		pick = candidates[rand.IntN(len(candidates))] //nolint:gosec // not security sensitive
	}
	if s.groupRepo.SetLeader(ctx, group.ID, &pick) == nil {
		group.LeaderID = &pick
	}
}

// syncLeadershipByID reloads a group and its set, then syncs leadership.
func (s *GroupService) syncLeadershipByID(ctx context.Context, groupID uint) {
	group, err := s.groupRepo.FindByID(ctx, groupID, 0)
	if err != nil || group == nil {
		return
	}
	category, _ := s.catRepo.FindByID(ctx, group.GroupCategoryID, 0)
	s.syncLeadership(ctx, group, category)
}

// ---- Roster ----

// GroupSetStudent is a course student as the group-set tools see them.
type GroupSetStudent struct {
	ID           uint    `json:"id"`
	Name         string  `json:"name"`
	SortableName string  `json:"sortable_name"`
	LoginID      string  `json:"login_id,omitempty"`
	SISUserID    *string `json:"sis_user_id,omitempty"`
	Email        string  `json:"email,omitempty"`
	SectionIDs   []uint  `json:"section_ids"`
}

// courseStudents returns the course's active students (one entry per
// user, with every section they are in), sorted by sortable name.
// accountID scopes the lookups to the caller's tenant.
func (s *GroupService) courseStudents(ctx context.Context, courseID, accountID uint) ([]GroupSetStudent, error) {
	if s.enrollmentRepo == nil {
		return nil, errors.New("enrollments are not available")
	}
	enrollments, err := s.enrollmentRepo.ListActiveStudentEnrollmentsByCourse(ctx, courseID, accountID)
	if err != nil {
		return nil, err
	}
	sections := map[uint][]uint{}
	var ids []uint
	for _, e := range enrollments {
		if _, seen := sections[e.UserID]; !seen {
			ids = append(ids, e.UserID)
			sections[e.UserID] = []uint{}
		}
		if e.CourseSectionID != nil {
			sections[e.UserID] = append(sections[e.UserID], *e.CourseSectionID)
		}
	}
	byID := map[uint]models.User{}
	if s.userRepo != nil && len(ids) > 0 {
		users, err := s.userRepo.FindByIDs(ctx, ids, accountID)
		if err != nil {
			return nil, err
		}
		for _, u := range users {
			byID[u.ID] = u
		}
	}
	out := make([]GroupSetStudent, 0, len(ids))
	for _, id := range ids {
		st := GroupSetStudent{ID: id, SectionIDs: sections[id], Name: fmt.Sprintf("User %d", id)}
		if u, ok := byID[id]; ok {
			st.Name, st.SortableName, st.LoginID, st.SISUserID, st.Email = u.Name, u.SortableName, u.LoginID, u.SISUserID, u.Email
		} else if s.userRepo != nil {
			continue // not in the caller's tenant (or deleted)
		}
		out = append(out, st)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := sortKey(out[i]), sortKey(out[j])
		if a != b {
			return a < b
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func sortKey(s GroupSetStudent) string {
	if s.SortableName != "" {
		return strings.ToLower(s.SortableName)
	}
	return strings.ToLower(s.Name)
}

// UnassignedStudents lists the course's students who are in no group of
// the set. In student-organized sets (several groups per student allowed)
// every student counts as unassigned, as in Canvas.
func (s *GroupService) UnassignedStudents(ctx context.Context, category *models.GroupCategory, accountID uint) ([]GroupSetStudent, error) {
	courseID, err := categoryCourseID(category)
	if err != nil {
		return nil, err
	}
	students, err := s.courseStudents(ctx, courseID, accountID)
	if err != nil {
		return nil, err
	}
	if allowsMultipleMemberships(category) {
		return students, nil
	}
	assigned, err := s.membershipRepo.ListUserIDsInCategory(ctx, category.ID)
	if err != nil {
		return nil, err
	}
	in := make(map[uint]bool, len(assigned))
	for _, id := range assigned {
		in[id] = true
	}
	out := make([]GroupSetStudent, 0, len(students))
	for _, st := range students {
		if !in[st.ID] {
			out = append(out, st)
		}
	}
	return out, nil
}

// CourseStudentsForSet lists every active student of the set's course.
func (s *GroupService) CourseStudentsForSet(ctx context.Context, category *models.GroupCategory, accountID uint) ([]GroupSetStudent, error) {
	courseID, err := categoryCourseID(category)
	if err != nil {
		return nil, err
	}
	return s.courseStudents(ctx, courseID, accountID)
}

func categoryCourseID(category *models.GroupCategory) (uint, error) {
	if category == nil || category.CourseID == nil || *category.CourseID == 0 {
		return 0, ErrGroupSetNotInCourse
	}
	return *category.CourseID, nil
}

// activeGroups lists the set's non-deleted groups.
func (s *GroupService) activeGroups(ctx context.Context, categoryID uint) ([]models.Group, error) {
	res, err := s.groupRepo.ListByCategoryID(ctx, categoryID, repository.PaginationParams{Page: 1, PerPage: 1000})
	if err != nil {
		return nil, err
	}
	out := make([]models.Group, 0, len(res.Items))
	for _, g := range res.Items {
		if g.WorkflowState != "deleted" {
			out = append(out, g)
		}
	}
	return out, nil
}

// ---- Bulk group creation ----

// groupBaseName is the per-group name stem: the set name, singularized
// the simple English way ("Project teams" → "Project team").
func groupBaseName(setName string) string {
	n := strings.TrimSpace(setName)
	if len(n) > 3 && strings.HasSuffix(n, "s") && !strings.HasSuffix(n, "ss") {
		n = n[:len(n)-1]
	}
	if n == "" {
		n = "Group"
	}
	return n
}

// CreateGroupsInSet creates count groups named "<set> N" in a course set,
// numbering after the groups already there.
func (s *GroupService) CreateGroupsInSet(ctx context.Context, category *models.GroupCategory, count int) ([]models.Group, error) {
	courseID, err := categoryCourseID(category)
	if err != nil {
		return nil, err
	}
	if count <= 0 {
		return []models.Group{}, nil
	}
	existing, err := s.activeGroups(ctx, category.ID)
	if err != nil {
		return nil, err
	}
	if count > maxGroupsPerSet-len(existing) {
		count = maxGroupsPerSet - len(existing)
	}
	base := groupBaseName(category.Name)
	taken := map[string]bool{}
	for _, g := range existing {
		taken[strings.ToLower(g.Name)] = true
	}
	created := make([]models.Group, 0, max(count, 0))
	n := len(existing)
	for len(created) < count {
		n++
		name := fmt.Sprintf("%s %d", base, n)
		if taken[strings.ToLower(name)] {
			continue
		}
		g := &models.Group{
			GroupCategoryID: category.ID,
			Name:            name,
			ContextType:     "Course",
			ContextID:       courseID,
			WorkflowState:   "available",
			JoinLevel:       "invitation_only",
		}
		if err := s.groupRepo.Create(ctx, g); err != nil {
			return created, err
		}
		taken[strings.ToLower(name)] = true
		created = append(created, *g)
	}
	return created, nil
}

// GroupCountForSize is how many groups of size members the set's
// unassigned students need (Canvas create_group_member_count).
func (s *GroupService) GroupCountForSize(ctx context.Context, category *models.GroupCategory, accountID uint, size int) (int, error) {
	if size <= 0 {
		return 0, nil
	}
	unassigned, err := s.UnassignedStudents(ctx, category, accountID)
	if err != nil {
		return 0, err
	}
	return (len(unassigned) + size - 1) / size, nil
}

// ---- Assign unassigned members ----

// AssignUnassigned spreads the set's unassigned students over its groups
// as evenly as possible, filling the smallest groups first and honoring
// group size limits (Canvas #assign_unassigned_members). With bySection,
// every group is given to one section and only that section's students
// go in it; as in Canvas, the groups must be empty and unlimited, and
// there must be at least one group per section.
func (s *GroupService) AssignUnassigned(ctx context.Context, category *models.GroupCategory, accountID uint, bySection bool) ([]models.GroupMembership, error) {
	if err := CheckAutoAssign(category); err != nil {
		return nil, err
	}
	groups, err := s.activeGroups(ctx, category.ID)
	if err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return nil, ErrNoGroupsInSet
	}
	unassigned, err := s.UnassignedStudents(ctx, category, accountID)
	if err != nil {
		return nil, err
	}
	if len(unassigned) == 0 {
		return []models.GroupMembership{}, nil
	}
	ids := make([]uint, len(groups))
	for i, g := range groups {
		ids[i] = g.ID
	}
	counts, err := s.membershipRepo.CountByGroupIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	var plan map[uint][]uint // group id → users to add
	if bySection {
		plan, err = planBySection(groups, counts, category, unassigned)
	} else {
		plan = planEvenly(groups, counts, category, unassigned)
	}
	if err != nil {
		return nil, err
	}

	created := []models.GroupMembership{}
	for i := range groups {
		g := &groups[i]
		users := plan[g.ID]
		if len(users) == 0 {
			continue
		}
		for _, uid := range users {
			m := models.GroupMembership{GroupID: g.ID, UserID: uid, WorkflowState: "accepted"}
			if err := s.membershipRepo.Create(ctx, &m); err != nil {
				return created, err
			}
			created = append(created, m)
		}
		s.syncLeadership(ctx, g, category)
	}
	return created, nil
}

// CheckAutoAssign reports whether the set's unassigned students can be
// assigned automatically. A student-organized set allows several groups
// per student, so every student counts as unassigned there: assigning
// them would re-add students to groups they are already in.
func CheckAutoAssign(category *models.GroupCategory) error {
	if _, err := categoryCourseID(category); err != nil {
		return err
	}
	if allowsMultipleMemberships(category) {
		return ErrAutoAssignStudentGroups
	}
	return nil
}

// planEvenly gives each student (in random order) to the group with the
// fewest members that still has room.
func planEvenly(groups []models.Group, counts map[uint]int64, category *models.GroupCategory, students []GroupSetStudent) map[uint][]uint {
	order := make([]GroupSetStudent, len(students))
	copy(order, students)
	rand.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] }) //nolint:gosec // not security sensitive

	size := make(map[uint]int64, len(groups))
	for _, g := range groups {
		size[g.ID] = counts[g.ID]
	}
	plan := map[uint][]uint{}
	for _, st := range order {
		best := -1
		for i := range groups {
			g := &groups[i]
			if limit := groupCapacity(g, category); limit > 0 && size[g.ID] >= int64(limit) {
				continue
			}
			if best < 0 || size[g.ID] < size[groups[best].ID] {
				best = i
			}
		}
		if best < 0 {
			break // every group is full
		}
		id := groups[best].ID
		plan[id] = append(plan[id], st.ID)
		size[id]++
	}
	return plan
}

// planBySection allots groups to sections in proportion to each section's
// student count (every section gets at least one), then spreads each
// section's students evenly over its groups. A student in several
// sections is placed by their first section. The groups must be
// unlimited: neither a group max_membership nor the set's group_limit,
// since the plan does not stop at a capacity.
func planBySection(groups []models.Group, counts map[uint]int64, category *models.GroupCategory, students []GroupSetStudent) (map[uint][]uint, error) {
	for i := range groups {
		g := &groups[i]
		if counts[g.ID] > 0 {
			return nil, ErrGroupsNotEmptyBySection
		}
		if groupCapacity(g, category) > 0 {
			return nil, ErrGroupsLimitedBySection
		}
	}
	bySec := map[uint][]GroupSetStudent{}
	var secOrder []uint
	for _, st := range students {
		var sec uint
		if len(st.SectionIDs) > 0 {
			sec = st.SectionIDs[0]
		}
		if _, ok := bySec[sec]; !ok {
			secOrder = append(secOrder, sec)
		}
		bySec[sec] = append(bySec[sec], st)
	}
	sort.Slice(secOrder, func(i, j int) bool { return secOrder[i] < secOrder[j] })
	if len(groups) < len(secOrder) {
		return nil, ErrTooFewGroupsBySection
	}

	// Largest-remainder allotment with a floor of one group per section.
	allot := map[uint]int{}
	remaining := len(groups) - len(secOrder)
	total := len(students)
	type rem struct {
		sec  uint
		frac float64
	}
	var rems []rem
	given := 0
	for _, sec := range secOrder {
		allot[sec] = 1
		share := float64(remaining) * float64(len(bySec[sec])) / float64(total)
		whole := int(share)
		allot[sec] += whole
		given += whole
		rems = append(rems, rem{sec, share - float64(whole)})
	}
	sort.SliceStable(rems, func(i, j int) bool { return rems[i].frac > rems[j].frac })
	for i := 0; given < remaining; i++ {
		allot[rems[i%len(rems)].sec]++
		given++
	}

	plan := map[uint][]uint{}
	next := 0
	for _, sec := range secOrder {
		mine := groups[next : next+allot[sec]]
		next += allot[sec]
		members := bySec[sec]
		rand.Shuffle(len(members), func(i, j int) { members[i], members[j] = members[j], members[i] }) //nolint:gosec // not security sensitive
		for i, st := range members {
			id := mine[i%len(mine)].ID
			plan[id] = append(plan[id], st.ID)
		}
	}
	return plan, nil
}

// ---- CSV import / export ----

// GroupImportSkip is one CSV row that was not imported.
type GroupImportSkip struct {
	Row    int    `json:"row"`
	Reason string `json:"reason"`
}

// GroupImportResult summarizes a CSV import.
type GroupImportResult struct {
	RowsProcessed      int               `json:"rows_processed"`
	GroupsCreated      int               `json:"groups_created"`
	MembershipsCreated int               `json:"memberships_created"`
	Skipped            []GroupImportSkip `json:"skipped"`
	SkippedCount       int               `json:"skipped_count"`
}

func (r *GroupImportResult) skip(row int, reason string) {
	r.SkippedCount++
	if len(r.Skipped) < maxImportSkips {
		r.Skipped = append(r.Skipped, GroupImportSkip{Row: row, Reason: reason})
	}
}

// ImportGroupsCSV assigns students to groups from a CSV with a header
// row. A student is matched by canvas_user_id (Paper user id), user_id
// (SIS id), login_id or email, and must be an active student of the
// course; a group by canvas_group_id or group_name, and a group_name that
// does not exist yet is created. Importing a student moves them out of
// their other groups in the set (unless the set allows several). This is
// the format ExportGroupsCSV writes, so an export can be edited and
// re-imported.
func (s *GroupService) ImportGroupsCSV(ctx context.Context, category *models.GroupCategory, accountID uint, r io.Reader) (*GroupImportResult, error) {
	courseID, err := categoryCourseID(category)
	if err != nil {
		return nil, err
	}
	reader := csv.NewReader(io.LimitReader(r, 5<<20))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	header, err := reader.Read()
	if err == io.EOF {
		return nil, ErrImportEmpty
	}
	if err != nil {
		return nil, fmt.Errorf("malformed CSV: %w", err)
	}
	col := map[string]int{}
	for i, h := range header {
		h = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\xef\xbb\xbf")))
		if _, dup := col[h]; !dup {
			col[h] = i
		}
	}
	has := func(name string) bool { _, ok := col[name]; return ok }
	if !(has("canvas_user_id") || has("user_id") || has("login_id") || has("email")) ||
		!(has("canvas_group_id") || has("group_name")) {
		return nil, ErrImportColumns
	}
	cell := func(rec []string, name string) string {
		i, ok := col[name]
		if !ok || i >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[i])
	}

	students, err := s.courseStudents(ctx, courseID, accountID)
	if err != nil {
		return nil, err
	}
	byID := map[uint]uint{}
	bySIS, byLogin, byEmail := map[string]uint{}, map[string]uint{}, map[string]uint{}
	for _, st := range students {
		byID[st.ID] = st.ID
		if st.SISUserID != nil && *st.SISUserID != "" {
			bySIS[*st.SISUserID] = st.ID
		}
		if st.LoginID != "" {
			byLogin[strings.ToLower(st.LoginID)] = st.ID
		}
		if st.Email != "" {
			byEmail[strings.ToLower(st.Email)] = st.ID
		}
	}

	groups, err := s.activeGroups(ctx, category.ID)
	if err != nil {
		return nil, err
	}
	groupByID := map[uint]*models.Group{}
	groupByName := map[string]*models.Group{}
	for i := range groups {
		g := &groups[i]
		groupByID[g.ID] = g
		groupByName[strings.ToLower(g.Name)] = g
	}

	result := &GroupImportResult{Skipped: []GroupImportSkip{}}
	seenUsers := map[uint]bool{}
	row := 1 // the header
	for {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}
		row++
		if err != nil {
			result.skip(row, "malformed row")
			continue
		}
		if row-1 > maxGroupImportRows {
			return nil, ErrImportTooLarge
		}
		blank := true
		for _, v := range rec {
			if strings.TrimSpace(v) != "" {
				blank = false
				break
			}
		}
		if blank {
			continue
		}
		result.RowsProcessed++

		// Resolve the student.
		var userID uint
		if v := cell(rec, "canvas_user_id"); v != "" {
			if n, perr := strconv.ParseUint(v, 10, 64); perr == nil {
				userID = byID[uint(n)]
			}
		}
		if userID == 0 {
			if v := cell(rec, "user_id"); v != "" {
				userID = bySIS[v]
			}
		}
		if userID == 0 {
			if v := cell(rec, "login_id"); v != "" {
				userID = byLogin[strings.ToLower(v)]
			}
		}
		if userID == 0 {
			if v := cell(rec, "email"); v != "" {
				userID = byEmail[strings.ToLower(v)]
			}
		}
		if userID == 0 {
			result.skip(row, "no active student of this course matches the row")
			continue
		}

		// Resolve (or create) the group.
		var group *models.Group
		if v := cell(rec, "canvas_group_id"); v != "" {
			if n, perr := strconv.ParseUint(v, 10, 64); perr == nil {
				group = groupByID[uint(n)]
			}
			if group == nil && cell(rec, "group_name") == "" {
				result.skip(row, "canvas_group_id is not a group of this set")
				continue
			}
		}
		if group == nil {
			name := cell(rec, "group_name")
			if name == "" {
				// A student with no group: leave them unassigned.
				continue
			}
			if len([]rune(name)) > 255 {
				result.skip(row, "group_name is too long")
				continue
			}
			group = groupByName[strings.ToLower(name)]
			if group == nil {
				if len(groupByID) >= maxGroupsPerSet {
					result.skip(row, "the group set already has the maximum number of groups")
					continue
				}
				g := &models.Group{
					GroupCategoryID: category.ID,
					Name:            name,
					ContextType:     "Course",
					ContextID:       courseID,
					WorkflowState:   "available",
					JoinLevel:       "invitation_only",
				}
				if err := s.groupRepo.Create(ctx, g); err != nil {
					return result, err
				}
				result.GroupsCreated++
				groupByID[g.ID] = g
				groupByName[strings.ToLower(g.Name)] = g
				group = g
			}
		}

		if seenUsers[userID] && !allowsMultipleMemberships(category) {
			result.skip(row, "the student is listed more than once; the first row was used")
			continue
		}
		seenUsers[userID] = true

		existing, _ := s.membershipRepo.FindByGroupAndUser(ctx, group.ID, userID)
		if existing != nil {
			continue // already in that group
		}
		if err := s.AddMember(ctx, &models.GroupMembership{GroupID: group.ID, UserID: userID, WorkflowState: "accepted"}); err != nil {
			if errors.Is(err, ErrGroupFull) {
				result.skip(row, fmt.Sprintf("group %q is full", group.Name))
				continue
			}
			return result, err
		}
		result.MembershipsCreated++
	}
	return result, nil
}

// csvSafe neutralizes spreadsheet formula injection in an exported cell.
func csvSafe(v string) string {
	if v != "" && strings.ContainsAny(v[:1], "=+-@\t\r") {
		return "'" + v
	}
	return v
}

// ExportGroupsCSV writes every student of the course with their group in
// the set (blank for unassigned students), in the format ImportGroupsCSV
// reads.
func (s *GroupService) ExportGroupsCSV(ctx context.Context, category *models.GroupCategory, accountID uint, w io.Writer) error {
	courseID, err := categoryCourseID(category)
	if err != nil {
		return err
	}
	students, err := s.courseStudents(ctx, courseID, accountID)
	if err != nil {
		return err
	}
	groups, err := s.activeGroups(ctx, category.ID)
	if err != nil {
		return err
	}
	memberOf := map[uint][]*models.Group{}
	for i := range groups {
		g := &groups[i]
		res, err := s.membershipRepo.ListByGroupID(ctx, g.ID, repository.PaginationParams{Page: 1, PerPage: 10000})
		if err != nil {
			return err
		}
		for _, m := range res.Items {
			if m.WorkflowState == "accepted" {
				memberOf[m.UserID] = append(memberOf[m.UserID], g)
			}
		}
	}

	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"name", "canvas_user_id", "user_id", "login_id", "email", "group_name", "canvas_group_id"}); err != nil {
		return err
	}
	for _, st := range students {
		sis := ""
		if st.SISUserID != nil {
			sis = *st.SISUserID
		}
		base := []string{csvSafe(st.Name), strconv.FormatUint(uint64(st.ID), 10), csvSafe(sis), csvSafe(st.LoginID), csvSafe(st.Email)}
		gs := memberOf[st.ID]
		if len(gs) == 0 {
			if err := cw.Write(append(base, "", "")); err != nil {
				return err
			}
			continue
		}
		for _, g := range gs {
			if err := cw.Write(append(append([]string{}, base...), csvSafe(g.Name), strconv.FormatUint(uint64(g.ID), 10))); err != nil {
				return err
			}
		}
	}
	cw.Flush()
	return cw.Error()
}
