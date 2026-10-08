package postgres

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type gradingPolicyRepo struct {
	db *gorm.DB
}

// NewGradingPolicyRepository returns the grading-policy store.
func NewGradingPolicyRepository(db *gorm.DB) repository.GradingPolicyRepository {
	return &gradingPolicyRepo{db: db}
}

func (r *gradingPolicyRepo) ListCourseAssignments(ctx context.Context, courseID uint) ([]models.Assignment, error) {
	var out []models.Assignment
	err := r.db.WithContext(ctx).
		Where("course_id = ? AND workflow_state <> ?", courseID, models.AssignmentDeleted).
		Order("position ASC, id ASC").Find(&out).Error
	return out, err
}

func (r *gradingPolicyRepo) ListCourseGroups(ctx context.Context, courseID uint) ([]models.AssignmentGroup, error) {
	var out []models.AssignmentGroup
	err := r.db.WithContext(ctx).
		Where("course_id = ? AND workflow_state <> ?", courseID, "deleted").
		Order("position ASC, id ASC").Find(&out).Error
	return out, err
}

func (r *gradingPolicyRepo) ListCourseSubmissions(ctx context.Context, courseID uint, userIDs []uint) ([]models.Submission, error) {
	var out []models.Submission
	q := r.db.WithContext(ctx).
		Where("assignment_id IN (SELECT id FROM assignments WHERE course_id = ?)", courseID)
	if len(userIDs) > 0 {
		q = q.Where("user_id IN ?", userIDs)
	}
	err := q.Order("id ASC").Find(&out).Error
	return out, err
}

func (r *gradingPolicyRepo) ListAssignmentSubmissions(ctx context.Context, assignmentID uint) ([]models.Submission, error) {
	var out []models.Submission
	err := r.db.WithContext(ctx).Where("assignment_id = ?", assignmentID).Order("id ASC").Find(&out).Error
	return out, err
}

func (r *gradingPolicyRepo) ListCourseOverrides(ctx context.Context, courseID uint) ([]models.AssignmentOverride, error) {
	var out []models.AssignmentOverride
	err := r.db.WithContext(ctx).
		Where("workflow_state <> ? AND assignment_id IN (SELECT id FROM assignments WHERE course_id = ?)", "deleted", courseID).
		Order("id ASC").Find(&out).Error
	return out, err
}

func (r *gradingPolicyRepo) ListCourseOverrideStudents(ctx context.Context, courseID uint) ([]models.AssignmentOverrideStudent, error) {
	var out []models.AssignmentOverrideStudent
	err := r.db.WithContext(ctx).
		Where("assignment_id IN (SELECT id FROM assignments WHERE course_id = ?)", courseID).
		Where("assignment_override_id IN (SELECT id FROM assignment_overrides WHERE workflow_state <> ?)", "deleted").
		Order("id ASC").Find(&out).Error
	return out, err
}

func (r *gradingPolicyRepo) ListCourseStudents(ctx context.Context, courseID uint) ([]models.Enrollment, error) {
	var out []models.Enrollment
	err := r.db.WithContext(ctx).Preload("User").
		Where("course_id = ? AND type = ? AND workflow_state NOT IN ?", courseID, "StudentEnrollment",
			[]string{string(models.EnrollmentDeleted), string(models.EnrollmentRejected)}).
		Order("id ASC").Find(&out).Error
	return out, err
}

func (r *gradingPolicyRepo) ListCourseAccommodations(ctx context.Context, courseID uint, userIDs []uint) ([]models.StudentAccommodation, error) {
	var out []models.StudentAccommodation
	if len(userIDs) == 0 {
		return out, nil
	}
	now := time.Now()
	err := r.db.WithContext(ctx).
		Where("user_id IN ? AND (course_id = ? OR course_id IS NULL) AND status = ? AND effective_from <= ? AND (effective_until IS NULL OR effective_until >= ?)",
			userIDs, courseID, "active", now, now).
		Find(&out).Error
	return out, err
}

func (r *gradingPolicyRepo) CourseGradingPeriods(ctx context.Context, course *models.Course) (*models.GradingPeriodGroup, []models.GradingPeriod, error) {
	if course == nil || course.EnrollmentTermID == nil {
		return nil, nil, nil
	}
	var term models.EnrollmentTerm
	if err := r.db.WithContext(ctx).
		Where("id = ? AND account_id = ?", *course.EnrollmentTermID, course.AccountID).
		First(&term).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	if term.GradingPeriodGroupID == nil {
		return nil, nil, nil
	}
	var group models.GradingPeriodGroup
	if err := r.db.WithContext(ctx).
		Where("id = ? AND account_id = ? AND workflow_state = ?", *term.GradingPeriodGroupID, course.AccountID, "active").
		First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	var periods []models.GradingPeriod
	if err := r.db.WithContext(ctx).
		Where("grading_period_group_id = ? AND workflow_state = ?", group.ID, "active").
		Order("start_date ASC, id ASC").Find(&periods).Error; err != nil {
		return nil, nil, err
	}
	return &group, periods, nil
}

func (r *gradingPolicyRepo) ListScoreOverrides(ctx context.Context, courseID uint) ([]models.CourseScoreOverride, error) {
	var out []models.CourseScoreOverride
	err := r.db.WithContext(ctx).Where("course_id = ?", courseID).Order("id ASC").Find(&out).Error
	return out, err
}

func (r *gradingPolicyRepo) UpsertScoreOverride(ctx context.Context, o *models.CourseScoreOverride) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "course_id"}, {Name: "user_id"}, {Name: "grading_period_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"override_score", "updated_by_id", "updated_at"}),
	}).Create(o).Error
}

func (r *gradingPolicyRepo) DeleteScoreOverride(ctx context.Context, courseID, userID, gradingPeriodID uint) error {
	return r.db.WithContext(ctx).
		Where("course_id = ? AND user_id = ? AND grading_period_id = ?", courseID, userID, gradingPeriodID).
		Delete(&models.CourseScoreOverride{}).Error
}

func (r *gradingPolicyRepo) UpsertSubmissionVersion(ctx context.Context, v *models.SubmissionVersion) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "submission_id"}, {Name: "attempt"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"submission_type", "body", "url", "attachments", "submitted_at", "late",
			"score", "grade", "entered_score", "grader_id", "graded_at", "updated_at",
		}),
	}).Create(v).Error
}

func (r *gradingPolicyRepo) UpdateSubmissionVersionGrade(ctx context.Context, sub *models.Submission) error {
	if sub == nil || sub.ID == 0 || sub.Attempt < 1 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&models.SubmissionVersion{}).
		Where("submission_id = ? AND attempt = ?", sub.ID, sub.Attempt).
		Updates(map[string]interface{}{
			"score":         sub.Score,
			"grade":         sub.Grade,
			"entered_score": sub.EnteredScore,
			"grader_id":     sub.GraderID,
			"graded_at":     sub.GradedAt,
			"updated_at":    time.Now(),
		}).Error
}

func (r *gradingPolicyRepo) ListSubmissionVersions(ctx context.Context, submissionID uint) ([]models.SubmissionVersion, error) {
	var out []models.SubmissionVersion
	err := r.db.WithContext(ctx).Where("submission_id = ?", submissionID).Order("attempt ASC").Find(&out).Error
	return out, err
}

func (r *gradingPolicyRepo) SetAssignmentsPostPolicy(ctx context.Context, courseID uint, policy string) error {
	return r.db.WithContext(ctx).Model(&models.Assignment{}).
		Where("course_id = ? AND workflow_state <> ? AND anonymous_grading = ?", courseID, models.AssignmentDeleted, false).
		Update("post_policy", policy).Error
}

func (r *gradingPolicyRepo) SetCoursePostManually(ctx context.Context, courseID uint, postManually bool) error {
	return r.db.WithContext(ctx).Model(&models.Course{}).Where("id = ?", courseID).
		Update("post_manually", postManually).Error
}

func (r *gradingPolicyRepo) SetCourseGradingScheme(ctx context.Context, courseID uint, standardID *uint, disabled bool) error {
	return r.db.WithContext(ctx).Model(&models.Course{}).Where("id = ?", courseID).
		Updates(map[string]interface{}{"grading_standard_id": standardID, "grading_standard_disabled": disabled}).Error
}

func (r *gradingPolicyRepo) SetSubmissionsPostedAt(ctx context.Context, assignmentID uint, submissionIDs []uint, postedAt *time.Time) error {
	if len(submissionIDs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&models.Submission{}).
		Where("assignment_id = ? AND id IN ?", assignmentID, submissionIDs).
		Update("posted_at", postedAt).Error
}

func (r *gradingPolicyRepo) UpdateSubmissionStatus(ctx context.Context, sub *models.Submission) error {
	return r.db.WithContext(ctx).Model(&models.Submission{}).Where("id = ?", sub.ID).
		Updates(map[string]interface{}{
			"cached_due_date": sub.CachedDueDate,
			"late":            sub.Late,
			"missing":         sub.Missing,
		}).Error
}

func (r *gradingPolicyRepo) ListAssignmentsDueBetween(ctx context.Context, from, to time.Time) ([]models.Assignment, error) {
	var out []models.Assignment
	err := r.db.WithContext(ctx).
		Where("workflow_state = ?", models.AssignmentPublished).
		Where("course_id IN (SELECT id FROM courses WHERE workflow_state NOT IN ?)",
			[]string{string(models.CourseDeleted), string(models.CourseCompleted)}).
		Where("(due_at BETWEEN ? AND ?) OR id IN (SELECT assignment_id FROM assignment_overrides WHERE workflow_state <> ? AND due_at BETWEEN ? AND ?)",
			from, to, "deleted", from, to).
		Order("id ASC").Find(&out).Error
	return out, err
}

func (r *gradingPolicyRepo) FindCourse(ctx context.Context, courseID uint) (*models.Course, error) {
	var c models.Course
	if err := r.db.WithContext(ctx).First(&c, courseID).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *gradingPolicyRepo) FindLatePolicy(ctx context.Context, courseID uint) (*models.LatePolicy, error) {
	var p models.LatePolicy
	if err := r.db.WithContext(ctx).Where("course_id = ?", courseID).First(&p).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (r *gradingPolicyRepo) CreateSubmission(ctx context.Context, sub *models.Submission) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(sub).Error
}

func (r *gradingPolicyRepo) ListUserStudentCourseIDs(ctx context.Context, userID uint) ([]uint, error) {
	var ids []uint
	err := r.db.WithContext(ctx).Model(&models.Enrollment{}).
		Where("user_id = ? AND type = ? AND workflow_state NOT IN ?", userID, "StudentEnrollment",
			[]string{string(models.EnrollmentDeleted), string(models.EnrollmentRejected)}).
		Distinct().Pluck("course_id", &ids).Error
	return ids, err
}
