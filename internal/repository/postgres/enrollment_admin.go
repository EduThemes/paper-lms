package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

// Enrollment lifecycle, date-based access and section-limit queries
// (admin-enrollments lane). Same struct as the core enrollment repo; the
// extra constructor hands callers the wider interface.

// NewEnrollmentAdminRepository returns the enrollment repository typed as
// the lifecycle/admin surface.
func NewEnrollmentAdminRepository(db *gorm.DB) repository.EnrollmentAdminRepository {
	return &enrollmentRepo{db: db}
}

// accessStates are the workflow states that can still grant some access
// (or become active again): everything but deleted and rejected.
var accessStates = []string{
	string(models.EnrollmentActive),
	string(models.EnrollmentInvited),
	string(models.EnrollmentCompleted),
	string(models.EnrollmentInactive),
}

func (r *enrollmentRepo) LoadCourseAccess(ctx context.Context, userID, courseID, accountID uint) (*models.CourseAccessData, error) {
	data := &models.CourseAccessData{Sections: map[uint]*models.CourseSection{}}
	q := r.db.WithContext(ctx).
		Where("user_id = ? AND course_id = ? AND workflow_state IN ?", userID, courseID, accessStates)
	q = tenantScopedByCourse(q, accountID)
	if err := q.Order(enrollmentPrivilegeOrder).Order("id ASC").Find(&data.Enrollments).Error; err != nil {
		return nil, err
	}
	if len(data.Enrollments) == 0 {
		return data, nil
	}

	var sectionIDs []uint
	for _, e := range data.Enrollments {
		if e.CourseSectionID != nil {
			sectionIDs = append(sectionIDs, *e.CourseSectionID)
		}
	}
	if len(sectionIDs) > 0 {
		var sections []models.CourseSection
		if err := r.db.WithContext(ctx).Where("id IN ?", sectionIDs).Find(&sections).Error; err != nil {
			return nil, err
		}
		for i := range sections {
			data.Sections[sections[i].ID] = &sections[i]
		}
	}

	var terms []models.EnrollmentTerm
	if err := r.db.WithContext(ctx).
		Where("id = (SELECT enrollment_term_id FROM courses WHERE id = ?)", courseID).
		Limit(1).Find(&terms).Error; err != nil {
		return nil, err
	}
	if len(terms) == 1 {
		data.Term = &terms[0]
	}
	return data, nil
}

// sectionMatch builds "course_section_id IN (...)" with 0 meaning
// "no section" (NULL).
func sectionMatch(q *gorm.DB, sectionIDs []uint) *gorm.DB {
	var ids []uint
	includeNull := false
	for _, id := range sectionIDs {
		if id == 0 {
			includeNull = true
		} else {
			ids = append(ids, id)
		}
	}
	switch {
	case len(ids) > 0 && includeNull:
		return q.Where("(course_section_id IN ? OR course_section_id IS NULL)", ids)
	case len(ids) > 0:
		return q.Where("course_section_id IN ?", ids)
	case includeNull:
		return q.Where("course_section_id IS NULL")
	}
	return q.Where("1 = 0")
}

// staffTypes are always visible to a section-limited user (a limited
// student still sees the course's teachers; a limited TA can see, but by
// the add-role rules not manage, the teacher).
var staffTypes = []string{models.TeacherEnrollmentType, models.TaEnrollmentType, models.DesignerEnrollmentType}

// inSectionsOrStaff matches enrollments in sectionIDs or of a staff type.
func (r *enrollmentRepo) inSectionsOrStaff(sectionIDs []uint) *gorm.DB {
	return sectionMatch(r.db.Session(&gorm.Session{NewDB: true}), sectionIDs).Or("type IN ?", staffTypes)
}

func (r *enrollmentRepo) UserInCourseSections(ctx context.Context, userID, courseID uint, sectionIDs []uint) (bool, error) {
	var n int64
	q := r.db.WithContext(ctx).Model(&models.Enrollment{}).
		Where("user_id = ? AND course_id = ? AND workflow_state <> ?", userID, courseID, string(models.EnrollmentDeleted)).
		Where(r.inSectionsOrStaff(sectionIDs))
	if err := q.Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

func (r *enrollmentRepo) FindInCourse(ctx context.Context, id, courseID, accountID uint) (*models.Enrollment, error) {
	var e models.Enrollment
	q := r.db.WithContext(ctx).Where("id = ? AND course_id = ?", id, courseID)
	q = tenantScopedByCourse(q, accountID)
	if err := q.Preload("User").Take(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *enrollmentRepo) ListForUserInCourse(ctx context.Context, userID, courseID, accountID uint) ([]models.Enrollment, error) {
	var out []models.Enrollment
	q := r.db.WithContext(ctx).
		Where("user_id = ? AND course_id = ? AND workflow_state <> ?", userID, courseID, string(models.EnrollmentDeleted))
	q = tenantScopedByCourse(q, accountID)
	err := q.Order("id ASC").Find(&out).Error
	return out, err
}

func (r *enrollmentRepo) ListByCourseFiltered(ctx context.Context, courseID, accountID uint, f repository.EnrollmentListFilter, params repository.PaginationParams) (*repository.PaginatedResult[models.Enrollment], error) {
	var items []models.Enrollment
	var count int64

	q := r.db.WithContext(ctx).Model(&models.Enrollment{}).Where("course_id = ?", courseID)
	if len(f.States) > 0 {
		q = q.Where("workflow_state IN ?", f.States)
	} else {
		q = q.Where("workflow_state <> ?", string(models.EnrollmentDeleted))
	}
	if f.SectionIDs != nil {
		inSections := sectionMatch(r.db.Session(&gorm.Session{NewDB: true}), f.SectionIDs)
		if len(f.AlwaysIncludeTypes) > 0 {
			q = q.Where(inSections.Or("type IN ?", f.AlwaysIncludeTypes))
		} else {
			q = q.Where(inSections)
		}
	}
	q = tenantScopedByCourse(q, accountID)
	if err := q.Count(&count).Error; err != nil {
		return nil, err
	}
	offset := (params.Page - 1) * params.PerPage
	if err := q.Preload("User").Offset(offset).Limit(params.PerPage).Order("id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return &repository.PaginatedResult[models.Enrollment]{
		Items:      items,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *enrollmentRepo) ListInvitedForUser(ctx context.Context, userID, accountID uint) ([]models.Enrollment, error) {
	var out []models.Enrollment
	q := r.db.WithContext(ctx).
		Where("user_id = ? AND workflow_state = ?", userID, string(models.EnrollmentInvited)).
		Where("course_id IN (SELECT id FROM courses WHERE workflow_state <> ?)", string(models.CourseDeleted))
	q = tenantScopedByCourse(q, accountID)
	err := q.Preload("Course").Order("id ASC").Find(&out).Error
	return out, err
}

func (r *enrollmentRepo) UserIDsInSections(ctx context.Context, courseID uint, sectionIDs []uint) ([]uint, error) {
	var ids []uint
	q := r.db.WithContext(ctx).Model(&models.Enrollment{}).
		Where("course_id = ? AND workflow_state <> ?", courseID, string(models.EnrollmentDeleted)).
		Where(r.inSectionsOrStaff(sectionIDs))
	err := q.Distinct("user_id").Pluck("user_id", &ids).Error
	return ids, err
}

func (r *enrollmentRepo) CountLiveInSection(ctx context.Context, sectionID uint) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.Enrollment{}).
		Where("course_section_id = ? AND workflow_state <> ?", sectionID, string(models.EnrollmentDeleted)).
		Count(&n).Error
	return n, err
}

func (r *enrollmentRepo) Save(ctx context.Context, e *models.Enrollment) error {
	// Omit the preloaded associations: Save would otherwise upsert them.
	return r.db.WithContext(ctx).Omit("User", "Course").Save(e).Error
}

// NewSectionAdminRepository returns the section repository typed as the
// section-management surface.
func NewSectionAdminRepository(db *gorm.DB) repository.SectionAdminRepository {
	return &sectionRepo{db: db}
}

func (r *sectionRepo) FindInAccount(ctx context.Context, id, accountID uint) (*models.CourseSection, error) {
	var s models.CourseSection
	q := r.db.WithContext(ctx).Where("id = ? AND workflow_state <> ?", id, "deleted")
	if accountID != 0 {
		q = q.Where("course_id IN (SELECT id FROM courses WHERE account_id = ?)", accountID)
	}
	if err := q.Take(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *sectionRepo) ListByCourse(ctx context.Context, courseID uint) ([]models.CourseSection, error) {
	var out []models.CourseSection
	err := r.db.WithContext(ctx).
		Where("course_id = ? AND workflow_state <> ?", courseID, "deleted").
		Order("id ASC").Find(&out).Error
	return out, err
}

func (r *sectionRepo) Save(ctx context.Context, section *models.CourseSection) error {
	return r.db.WithContext(ctx).Save(section).Error
}

func (r *sectionRepo) MoveToCourse(ctx context.Context, section *models.CourseSection, toCourseID uint, nonxlistCourseID *uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.CourseSection{}).Where("id = ?", section.ID).
			Updates(map[string]interface{}{
				"course_id":          toCourseID,
				"nonxlist_course_id": nonxlistCourseID,
				"updated_at":         gorm.Expr("NOW()"),
			}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Enrollment{}).Where("course_section_id = ?", section.ID).
			Updates(map[string]interface{}{
				"course_id":  toCourseID,
				"updated_at": gorm.Expr("NOW()"),
			}).Error; err != nil {
			return err
		}
		section.CourseID = toCourseID
		section.NonxlistCourseID = nonxlistCourseID
		return nil
	})
}
