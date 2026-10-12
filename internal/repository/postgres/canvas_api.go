package postgres

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type canvasAPIRepo struct {
	db *gorm.DB
}

// NewCanvasAPIRepository builds the repository behind the Canvas-compatible
// collection endpoints and the SIS id resolver (G137/G138).
func NewCanvasAPIRepository(db *gorm.DB) repository.CanvasAPIRepository {
	return &canvasAPIRepo{db: db}
}

// defaultEnrollmentStates is Canvas's default enrollment_state filter.
var defaultEnrollmentStates = []string{string(models.EnrollmentActive), string(models.EnrollmentInvited)}

func canvasPaginate[T any](q *gorm.DB, params repository.PaginationParams, order string) (*repository.PaginatedResult[T], error) {
	var count int64
	if err := q.Session(&gorm.Session{}).Count(&count).Error; err != nil {
		return nil, err
	}
	if params.PerPage < 1 {
		params.PerPage = 10
	}
	if params.Page < 1 {
		params.Page = 1
	}
	var items []T
	if err := q.Order(order).Offset((params.Page - 1) * params.PerPage).Limit(params.PerPage).Find(&items).Error; err != nil {
		return nil, err
	}
	return &repository.PaginatedResult[T]{Items: items, TotalCount: count, Page: params.Page, PerPage: params.PerPage}, nil
}

// likePattern escapes LIKE metacharacters in a user-supplied search term.
func likePattern(term string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(strings.ToLower(strings.TrimSpace(term))) + "%"
}

func (r *canvasAPIRepo) ResolveCanvasID(ctx context.Context, kind, value string, accountID uint) (uint, error) {
	var id uint
	q := r.db.WithContext(ctx)
	var err error
	switch kind {
	case "sis_course_id":
		q = q.Model(&models.Course{}).Where("sis_course_id = ? AND workflow_state <> ?", value, models.CourseDeleted)
		if accountID != 0 {
			q = q.Where("account_id = ?", accountID)
		}
		err = q.Select("id").Limit(1).Scan(&id).Error
	case "sis_user_id", "sis_login_id":
		col := "sis_user_id"
		if kind == "sis_login_id" {
			col = "login_id"
		}
		q = q.Model(&models.User{}).Where(col+" = ?", value)
		if accountID != 0 {
			q = q.Where("account_id = ?", accountID)
		}
		err = q.Select("id").Limit(1).Scan(&id).Error
	case "sis_section_id":
		q = q.Table("course_sections").Joins("JOIN courses ON courses.id = course_sections.course_id").
			Where("course_sections.sis_section_id = ? AND course_sections.workflow_state <> ?", value, "deleted")
		if accountID != 0 {
			q = q.Where("courses.account_id = ?", accountID)
		}
		err = q.Select("course_sections.id").Limit(1).Scan(&id).Error
	case "sis_account_id":
		q = q.Model(&models.Account{}).Where("sis_account_id = ?", value)
		if accountID != 0 {
			q = q.Where("id = ?", accountID)
		}
		err = q.Select("id").Limit(1).Scan(&id).Error
	case "sis_term_id":
		q = q.Model(&models.EnrollmentTerm{}).Where("sis_term_id = ? AND workflow_state <> ?", value, "deleted")
		if accountID != 0 {
			q = q.Where("account_id = ?", accountID)
		}
		err = q.Select("id").Limit(1).Scan(&id).Error
	default:
		return 0, gorm.ErrRecordNotFound
	}
	if err != nil {
		return 0, err
	}
	if id == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	return id, nil
}

func statesOrDefault(states []string) []string {
	if len(states) == 0 {
		return defaultEnrollmentStates
	}
	return states
}

// courseInTenant restricts a query on a course-child table to courses of
// accountID (no-op when 0).
func courseInTenant(q *gorm.DB, courseCol string, accountID uint) *gorm.DB {
	if accountID == 0 {
		return q
	}
	return q.Where(courseCol+" IN (SELECT id FROM courses WHERE account_id = ?)", accountID)
}

func (r *canvasAPIRepo) ListCourseUsers(ctx context.Context, courseID uint, f repository.CourseUserFilter, params repository.PaginationParams, accountID uint) (*repository.PaginatedResult[models.User], error) {
	enr := r.db.Model(&models.Enrollment{}).Select("user_id").
		Where("course_id = ? AND workflow_state IN ?", courseID, statesOrDefault(f.EnrollmentStates))
	enr = courseInTenant(enr, "course_id", accountID)
	if len(f.EnrollmentTypes) > 0 {
		enr = enr.Where("type IN ?", f.EnrollmentTypes)
	}

	q := r.db.WithContext(ctx).Model(&models.User{}).Where("id IN (?)", enr)
	if accountID != 0 {
		q = q.Where("account_id = ?", accountID)
	}
	if len(f.UserIDs) > 0 {
		q = q.Where("id IN ?", f.UserIDs)
	}
	if term := strings.TrimSpace(f.SearchTerm); term != "" {
		p := likePattern(term)
		if f.NameSearchOnly {
			q = q.Where(`(LOWER(name) LIKE ? ESCAPE '\' OR LOWER(sortable_name) LIKE ? ESCAPE '\' OR LOWER(COALESCE(short_name, '')) LIKE ? ESCAPE '\')`, p, p, p)
		} else {
			q = q.Where(`(LOWER(name) LIKE ? ESCAPE '\' OR LOWER(sortable_name) LIKE ? ESCAPE '\' OR LOWER(login_id) LIKE ? ESCAPE '\' OR LOWER(email) LIKE ? ESCAPE '\' OR LOWER(COALESCE(sis_user_id, '')) LIKE ? ESCAPE '\')`, p, p, p, p, p)
		}
	}
	return canvasPaginate[models.User](q, params, userOrder(f.Sort, "asc"))
}

// userOrder maps a Canvas user sort key to an ORDER BY clause with a
// stable id tiebreak.
func userOrder(sort, order string) string {
	dir := "ASC"
	if strings.EqualFold(order, "desc") {
		dir = "DESC"
	}
	switch sort {
	case "email":
		return "LOWER(email) " + dir + ", id ASC"
	case "sis_id":
		return "sis_user_id " + dir + " NULLS LAST, id ASC"
	case "id":
		return "id " + dir
	case "last_login":
		return "(SELECT MAX(created_at) FROM page_views WHERE page_views.user_id = users.id) " + dir + " NULLS LAST, id ASC"
	default:
		return "LOWER(COALESCE(NULLIF(sortable_name, ''), name)) " + dir + ", id ASC"
	}
}

func (r *canvasAPIRepo) CourseEnrollmentsForUsers(ctx context.Context, courseID uint, userIDs []uint, states []string, accountID uint) ([]models.Enrollment, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	q := r.db.WithContext(ctx).Where("course_id = ? AND user_id IN ? AND workflow_state IN ?", courseID, userIDs, statesOrDefault(states))
	q = courseInTenant(q, "course_id", accountID)
	var out []models.Enrollment
	err := q.Order("id ASC").Find(&out).Error
	return out, err
}

func (r *canvasAPIRepo) ListAccountCourses(ctx context.Context, f repository.AccountCourseFilter, params repository.PaginationParams, accountID uint) (*repository.PaginatedResult[models.Course], error) {
	q := r.db.WithContext(ctx).Model(&models.Course{})
	if accountID != 0 {
		q = q.Where("courses.account_id = ?", accountID)
	}
	if len(f.States) > 0 {
		q = q.Where("courses.workflow_state IN ?", f.States)
	} else {
		q = q.Where("courses.workflow_state <> ?", models.CourseDeleted)
	}
	if f.Published != nil {
		published := []models.CourseWorkflow{models.CourseAvailable, models.CourseCompleted}
		if *f.Published {
			q = q.Where("courses.workflow_state IN ?", published)
		} else {
			q = q.Where("courses.workflow_state NOT IN ?", published)
		}
	}
	if f.Completed != nil {
		if *f.Completed {
			q = q.Where("courses.workflow_state = ?", models.CourseCompleted)
		} else {
			q = q.Where("courses.workflow_state <> ?", models.CourseCompleted)
		}
	}
	if f.EnrollmentTermID != nil {
		q = q.Where("courses.enrollment_term_id = ?", *f.EnrollmentTermID)
	}
	if f.WithEnrollments != nil {
		sub := "SELECT 1 FROM enrollments e WHERE e.course_id = courses.id AND e.workflow_state = 'active'"
		if *f.WithEnrollments {
			q = q.Where("EXISTS (" + sub + ")")
		} else {
			q = q.Where("NOT EXISTS (" + sub + ")")
		}
	}
	if len(f.ByTeachers) > 0 {
		q = q.Where("EXISTS (SELECT 1 FROM enrollments e WHERE e.course_id = courses.id AND e.type = 'TeacherEnrollment' AND e.workflow_state IN ? AND e.user_id IN ?)", defaultEnrollmentStates, f.ByTeachers)
	}
	if term := strings.TrimSpace(f.SearchTerm); term != "" {
		p := likePattern(term)
		q = q.Where(`(LOWER(courses.name) LIKE ? ESCAPE '\' OR LOWER(courses.course_code) LIKE ? ESCAPE '\' OR LOWER(COALESCE(courses.sis_course_id, '')) LIKE ? ESCAPE '\')`, p, p, p)
	}

	dir := "ASC"
	if strings.EqualFold(f.Order, "desc") {
		dir = "DESC"
	}
	var order string
	switch f.Sort {
	case "sis_course_id":
		order = "courses.sis_course_id " + dir + " NULLS LAST, courses.id ASC"
	case "term":
		order = "(SELECT name FROM enrollment_terms t WHERE t.id = courses.enrollment_term_id) " + dir + " NULLS LAST, courses.id ASC"
	case "teacher":
		order = "(SELECT MIN(LOWER(u.sortable_name)) FROM enrollments e JOIN users u ON u.id = e.user_id WHERE e.course_id = courses.id AND e.type = 'TeacherEnrollment' AND e.workflow_state = 'active') " + dir + " NULLS LAST, courses.id ASC"
	case "created_at":
		order = "courses.created_at " + dir + ", courses.id ASC"
	case "id":
		order = "courses.id " + dir
	default:
		order = "LOWER(courses.name) " + dir + ", courses.id ASC"
	}
	return canvasPaginate[models.Course](q, params, order)
}

func (r *canvasAPIRepo) TeachersForCourses(ctx context.Context, courseIDs []uint, accountID uint) (map[uint][]models.User, error) {
	out := map[uint][]models.User{}
	if len(courseIDs) == 0 {
		return out, nil
	}
	type row struct {
		CourseID uint
		models.User
	}
	var rows []row
	q := r.db.WithContext(ctx).Table("enrollments").
		Select("enrollments.course_id AS course_id, users.*").
		Joins("JOIN users ON users.id = enrollments.user_id").
		Where("enrollments.course_id IN ? AND enrollments.type = ? AND enrollments.workflow_state = ?", courseIDs, "TeacherEnrollment", models.EnrollmentActive)
	q = courseInTenant(q, "enrollments.course_id", accountID)
	if err := q.Order("LOWER(users.name) ASC, users.id ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	seen := map[[2]uint]bool{}
	for _, rw := range rows {
		k := [2]uint{rw.CourseID, rw.ID}
		if seen[k] {
			continue
		}
		seen[k] = true
		out[rw.CourseID] = append(out[rw.CourseID], rw.User)
	}
	return out, nil
}

func (r *canvasAPIRepo) StudentCountsForCourses(ctx context.Context, courseIDs []uint, accountID uint) (map[uint]int64, error) {
	out := map[uint]int64{}
	if len(courseIDs) == 0 {
		return out, nil
	}
	type row struct {
		CourseID uint
		N        int64
	}
	var rows []row
	q := r.db.WithContext(ctx).Model(&models.Enrollment{}).
		Select("course_id, COUNT(DISTINCT user_id) AS n").
		Where("course_id IN ? AND type = ? AND workflow_state = ?", courseIDs, "StudentEnrollment", models.EnrollmentActive)
	q = courseInTenant(q, "course_id", accountID)
	if err := q.Group("course_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, rw := range rows {
		out[rw.CourseID] = rw.N
	}
	return out, nil
}

func (r *canvasAPIRepo) TermsByID(ctx context.Context, ids []uint, accountID uint) (map[uint]models.EnrollmentTerm, error) {
	out := map[uint]models.EnrollmentTerm{}
	if len(ids) == 0 {
		return out, nil
	}
	q := r.db.WithContext(ctx).Where("id IN ?", ids)
	if accountID != 0 {
		q = q.Where("account_id = ?", accountID)
	}
	var terms []models.EnrollmentTerm
	if err := q.Find(&terms).Error; err != nil {
		return nil, err
	}
	for _, t := range terms {
		out[t.ID] = t
	}
	return out, nil
}

func (r *canvasAPIRepo) UserEnrollmentsInCourses(ctx context.Context, userID uint, courseIDs []uint, accountID uint) (map[uint][]models.Enrollment, error) {
	out := map[uint][]models.Enrollment{}
	if len(courseIDs) == 0 {
		return out, nil
	}
	q := r.db.WithContext(ctx).Where("user_id = ? AND course_id IN ? AND workflow_state NOT IN ?", userID, courseIDs,
		[]string{string(models.EnrollmentDeleted), string(models.EnrollmentRejected)})
	q = courseInTenant(q, "course_id", accountID)
	var rows []models.Enrollment
	if err := q.Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, e := range rows {
		out[e.CourseID] = append(out[e.CourseID], e)
	}
	return out, nil
}

func (r *canvasAPIRepo) ListAccountUsers(ctx context.Context, f repository.AccountUserFilter, params repository.PaginationParams, accountID uint) (*repository.PaginatedResult[models.User], error) {
	q := r.db.WithContext(ctx).Model(&models.User{})
	if accountID != 0 {
		q = q.Where("account_id = ?", accountID)
	}
	if term := strings.TrimSpace(f.SearchTerm); term != "" {
		p := likePattern(term)
		q = q.Where(`(LOWER(name) LIKE ? ESCAPE '\' OR LOWER(sortable_name) LIKE ? ESCAPE '\' OR LOWER(login_id) LIKE ? ESCAPE '\' OR LOWER(email) LIKE ? ESCAPE '\' OR LOWER(COALESCE(sis_user_id, '')) LIKE ? ESCAPE '\')`, p, p, p, p, p)
	}
	return canvasPaginate[models.User](q, params, userOrder(f.Sort, f.Order))
}

func (r *canvasAPIRepo) ListStudentSubmissions(ctx context.Context, courseID uint, f repository.StudentSubmissionFilter, params repository.PaginationParams, accountID uint) (*repository.PaginatedResult[models.Submission], error) {
	assignments := r.db.Model(&models.Assignment{}).Select("id").
		Where("course_id = ? AND workflow_state <> ?", courseID, models.AssignmentDeleted)
	assignments = courseInTenant(assignments, "course_id", accountID)
	if !f.IncludeUnpublished {
		assignments = assignments.Where("(workflow_state = ? OR (workflow_state = '' AND published))", models.AssignmentPublished)
	}
	if len(f.AssignmentIDs) > 0 {
		assignments = assignments.Where("id IN ?", f.AssignmentIDs)
	}

	students := r.db.Model(&models.Enrollment{}).Select("user_id").
		Where("course_id = ? AND type = ? AND workflow_state IN ?", courseID, "StudentEnrollment",
			[]string{string(models.EnrollmentActive), string(models.EnrollmentInvited), string(models.EnrollmentCompleted), string(models.EnrollmentInactive)})

	q := r.db.WithContext(ctx).Model(&models.Submission{}).
		Where("assignment_id IN (?)", assignments).
		Where("user_id IN (?)", students)
	if len(f.StudentIDs) > 0 {
		q = q.Where("user_id IN ?", f.StudentIDs)
	}
	if f.WorkflowState != "" {
		q = q.Where("workflow_state = ?", f.WorkflowState)
	}
	if f.SubmittedSince != nil {
		q = q.Where("submitted_at >= ?", *f.SubmittedSince)
	}
	if f.GradedSince != nil {
		q = q.Where("graded_at >= ?", *f.GradedSince)
	}
	dir := "ASC"
	if f.OrderDirection == "descending" {
		dir = "DESC"
	}
	order := "id " + dir
	if f.Order == "graded_at" {
		order = "graded_at " + dir + " NULLS LAST, id ASC"
	}
	return canvasPaginate[models.Submission](q, params, order)
}

func (r *canvasAPIRepo) AssignmentsByID(ctx context.Context, courseID uint, ids []uint, accountID uint) (map[uint]models.Assignment, error) {
	out := map[uint]models.Assignment{}
	if len(ids) == 0 {
		return out, nil
	}
	q := r.db.WithContext(ctx).Where("course_id = ? AND id IN ?", courseID, ids)
	q = courseInTenant(q, "course_id", accountID)
	var rows []models.Assignment
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, a := range rows {
		out[a.ID] = a
	}
	return out, nil
}

func (r *canvasAPIRepo) UsersByID(ctx context.Context, ids []uint, accountID uint) (map[uint]models.User, error) {
	out := map[uint]models.User{}
	if len(ids) == 0 {
		return out, nil
	}
	q := r.db.WithContext(ctx).Where("id IN ?", ids)
	if accountID != 0 {
		q = q.Where("account_id = ?", accountID)
	}
	var rows []models.User
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, u := range rows {
		out[u.ID] = u
	}
	return out, nil
}
