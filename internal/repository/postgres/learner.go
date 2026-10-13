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

type learnerRepo struct {
	db *gorm.DB
}

// NewLearnerRepository builds the read model behind the student / observer
// experience (see repository.LearnerRepository).
func NewLearnerRepository(db *gorm.DB) repository.LearnerRepository {
	return &learnerRepo{db: db}
}

func notFoundOr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return repository.ErrLearnerNotFound
	}
	return err
}

// --- Courses ---

func (r *learnerRepo) CoursesByIDs(ctx context.Context, ids []uint, accountID uint) ([]models.Course, error) {
	var out []models.Course
	if len(ids) == 0 {
		return out, nil
	}
	q := r.db.WithContext(ctx).Where("id IN ? AND workflow_state <> ?", ids, models.CourseDeleted)
	if accountID != 0 {
		q = q.Where("account_id = ?", accountID)
	}
	err := q.Order("id").Find(&out).Error
	return out, err
}

func (r *learnerRepo) EnrollmentTermsByIDs(ctx context.Context, ids []uint) ([]models.EnrollmentTerm, error) {
	var out []models.EnrollmentTerm
	if len(ids) == 0 {
		return out, nil
	}
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&out).Error
	return out, err
}

func (r *learnerRepo) GradingPeriodsForGroup(ctx context.Context, groupID uint) ([]models.GradingPeriod, error) {
	var out []models.GradingPeriod
	err := r.db.WithContext(ctx).
		Where("grading_period_group_id = ? AND workflow_state = ?", groupID, "active").
		Order("start_date ASC, id ASC").Find(&out).Error
	return out, err
}

func (r *learnerRepo) GradingPeriodGroup(ctx context.Context, groupID uint) (*models.GradingPeriodGroup, error) {
	var g models.GradingPeriodGroup
	if err := r.db.WithContext(ctx).Where("id = ? AND workflow_state = ?", groupID, "active").First(&g).Error; err != nil {
		return nil, notFoundOr(err)
	}
	return &g, nil
}

// --- Dashboard course preferences ---

func (r *learnerRepo) ListCoursePreferences(ctx context.Context, userID uint) ([]models.UserCoursePreference, error) {
	var out []models.UserCoursePreference
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("id").Find(&out).Error
	return out, err
}

func (r *learnerRepo) GetCoursePreference(ctx context.Context, userID, courseID uint) (*models.UserCoursePreference, error) {
	var p models.UserCoursePreference
	if err := r.db.WithContext(ctx).Where("user_id = ? AND course_id = ?", userID, courseID).First(&p).Error; err != nil {
		return nil, notFoundOr(err)
	}
	return &p, nil
}

// SaveCoursePreference upserts on (user_id, course_id). Every column is
// written explicitly so false / NULL values replace earlier ones.
func (r *learnerRepo) SaveCoursePreference(ctx context.Context, p *models.UserCoursePreference) error {
	now := time.Now()
	return r.db.WithContext(ctx).Raw(`
		INSERT INTO user_course_preferences (user_id, course_id, favorite, nickname, color, position, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (user_id, course_id) DO UPDATE
		   SET favorite = EXCLUDED.favorite,
		       nickname = EXCLUDED.nickname,
		       color = EXCLUDED.color,
		       position = EXCLUDED.position,
		       updated_at = EXCLUDED.updated_at
		RETURNING id, created_at`,
		p.UserID, p.CourseID, p.Favorite, p.Nickname, p.Color, p.Position, now, now).
		Row().Scan(&p.ID, &p.CreatedAt)
}

// --- Plannables ---

func (r *learnerRepo) PublishedAssignments(ctx context.Context, courseIDs []uint) ([]models.Assignment, error) {
	var out []models.Assignment
	if len(courseIDs) == 0 {
		return out, nil
	}
	err := r.db.WithContext(ctx).
		Where("course_id IN ?", courseIDs).
		Where("(workflow_state = ? OR ((workflow_state = '' OR workflow_state IS NULL) AND published = TRUE))", models.AssignmentPublished).
		Order("position ASC, id ASC").Find(&out).Error
	return out, err
}

func (r *learnerRepo) PublishedQuizzes(ctx context.Context, courseIDs []uint) ([]models.Quiz, error) {
	var out []models.Quiz
	if len(courseIDs) == 0 {
		return out, nil
	}
	err := r.db.WithContext(ctx).
		Where("course_id IN ? AND published = TRUE AND workflow_state <> ?", courseIDs, "deleted").
		Order("id ASC").Find(&out).Error
	return out, err
}

func (r *learnerRepo) AssignmentGroups(ctx context.Context, courseID uint) ([]models.AssignmentGroup, error) {
	var out []models.AssignmentGroup
	err := r.db.WithContext(ctx).
		Where("course_id = ? AND workflow_state <> ?", courseID, "deleted").
		Order("position ASC, id ASC").Find(&out).Error
	return out, err
}

func (r *learnerRepo) OverrideDueDates(ctx context.Context, userID uint, sectionIDs, assignmentIDs []uint) (map[uint][]*time.Time, error) {
	out := map[uint][]*time.Time{}
	if len(assignmentIDs) == 0 {
		return out, nil
	}
	type row struct {
		AssignmentID uint
		DueAt        *time.Time
	}
	var rows []row
	q := r.db.WithContext(ctx).Table("assignment_overrides o").
		Select("o.assignment_id, o.due_at").
		Where("o.assignment_id IN ? AND o.workflow_state = ?", assignmentIDs, "active")
	if len(sectionIDs) > 0 {
		q = q.Where("(o.course_section_id IN ? OR EXISTS (SELECT 1 FROM assignment_override_students s WHERE s.assignment_override_id = o.id AND s.user_id = ?))", sectionIDs, userID)
	} else {
		q = q.Where("EXISTS (SELECT 1 FROM assignment_override_students s WHERE s.assignment_override_id = o.id AND s.user_id = ?)", userID)
	}
	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, rw := range rows {
		out[rw.AssignmentID] = append(out[rw.AssignmentID], rw.DueAt)
	}
	return out, nil
}

func (r *learnerRepo) SubmissionsForUser(ctx context.Context, userID uint, assignmentIDs []uint) ([]models.Submission, error) {
	var out []models.Submission
	if len(assignmentIDs) == 0 {
		return out, nil
	}
	err := r.db.WithContext(ctx).Where("user_id = ? AND assignment_id IN ?", userID, assignmentIDs).Find(&out).Error
	return out, err
}

func (r *learnerRepo) LatestQuizSubmissions(ctx context.Context, userID uint, quizIDs []uint) (map[uint]models.QuizSubmission, error) {
	out := map[uint]models.QuizSubmission{}
	if len(quizIDs) == 0 {
		return out, nil
	}
	var rows []models.QuizSubmission
	if err := r.db.WithContext(ctx).Where("user_id = ? AND quiz_id IN ?", userID, quizIDs).
		Order("attempt ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, qs := range rows {
		out[qs.QuizID] = qs // ascending order: the last write is the latest attempt
	}
	return out, nil
}

func (r *learnerRepo) DiscussionTopicsForAssignments(ctx context.Context, assignmentIDs []uint) (map[uint]uint, error) {
	out := map[uint]uint{}
	if len(assignmentIDs) == 0 {
		return out, nil
	}
	var topics []models.DiscussionTopic
	if err := r.db.WithContext(ctx).Select("id, assignment_id").
		Where("assignment_id IN ? AND workflow_state <> ?", assignmentIDs, models.DiscussionTopicDeleted).
		Find(&topics).Error; err != nil {
		return nil, err
	}
	for _, t := range topics {
		if t.AssignmentID != nil {
			out[*t.AssignmentID] = t.ID
		}
	}
	return out, nil
}

// --- Activity ---

func (r *learnerRepo) RecentlyPostedSubmissions(ctx context.Context, userID uint, courseIDs []uint, since time.Time) ([]models.Submission, error) {
	var out []models.Submission
	if len(courseIDs) == 0 {
		return out, nil
	}
	// The student learns of a grade when it is posted, so the later of
	// graded_at / posted_at is the event time (GREATEST skips NULLs). A
	// grade entered under a manual post policy and posted hours later must
	// still count as new when it is posted.
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND posted_at IS NOT NULL AND (score IS NOT NULL OR excused = TRUE)", userID).
		Where("GREATEST(graded_at, posted_at) >= ?", since).
		Where("assignment_id IN (SELECT id FROM assignments WHERE course_id IN ? AND workflow_state <> ?)", courseIDs, models.AssignmentDeleted).
		Order("GREATEST(graded_at, posted_at) DESC").Limit(100).Find(&out).Error
	return out, err
}

func (r *learnerRepo) RecentCommentsOnSubmissions(ctx context.Context, userID uint, courseIDs []uint, since time.Time) ([]repository.SubmissionCommentActivity, error) {
	var out []repository.SubmissionCommentActivity
	if len(courseIDs) == 0 {
		return out, nil
	}
	// Only comments the student can see: not drafts, and — for comments by
	// anyone else — only once the submission is posted (post policy).
	err := r.db.WithContext(ctx).Table("submission_comments sc").
		Select(`sc.id AS comment_id, sc.submission_id, s.assignment_id, a.course_id,
		        sc.author_id, sc.comment, sc.created_at`).
		Joins("JOIN submissions s ON s.id = sc.submission_id").
		Joins("JOIN assignments a ON a.id = s.assignment_id").
		Where("s.user_id = ? AND sc.author_id <> ? AND sc.draft = FALSE", userID, userID).
		Where("s.posted_at IS NOT NULL AND a.course_id IN ? AND a.workflow_state <> ?", courseIDs, models.AssignmentDeleted).
		Where("sc.created_at >= ?", since).
		Order("sc.created_at DESC").Limit(100).Scan(&out).Error
	return out, err
}

func (r *learnerRepo) RecentDiscussionTopics(ctx context.Context, courseIDs []uint, since time.Time) ([]models.DiscussionTopic, error) {
	var out []models.DiscussionTopic
	if len(courseIDs) == 0 {
		return out, nil
	}
	now := time.Now()
	err := r.db.WithContext(ctx).
		Where("course_id IN ? AND workflow_state IN ?", courseIDs,
			[]models.DiscussionTopicWorkflow{models.DiscussionTopicActive, models.DiscussionTopicLocked}).
		Where("(delayed_post_at IS NULL OR delayed_post_at <= ?)", now).
		Where("COALESCE(posted_at, created_at) >= ?", since).
		Order("COALESCE(posted_at, created_at) DESC").Limit(100).Find(&out).Error
	return out, err
}

func (r *learnerRepo) RecentAnnouncements(ctx context.Context, courseIDs []uint, since time.Time) ([]models.Announcement, error) {
	var out []models.Announcement
	if len(courseIDs) == 0 {
		return out, nil
	}
	now := time.Now()
	err := r.db.WithContext(ctx).
		Where("course_id IN ? AND workflow_state = ?", courseIDs, "published").
		Where("(posted_at IS NULL OR posted_at <= ?)", now).
		Where("COALESCE(posted_at, created_at) >= ?", since).
		Order("COALESCE(posted_at, created_at) DESC").Limit(100).Find(&out).Error
	return out, err
}

// --- Grade summary ---

func (r *learnerRepo) SubmissionComments(ctx context.Context, submissionIDs []uint) ([]models.SubmissionComment, error) {
	var out []models.SubmissionComment
	if len(submissionIDs) == 0 {
		return out, nil
	}
	err := r.db.WithContext(ctx).Where("submission_id IN ? AND draft = FALSE", submissionIDs).
		Order("created_at ASC, id ASC").Find(&out).Error
	return out, err
}

func (r *learnerRepo) RubricAssessmentsForUser(ctx context.Context, userID uint, assignmentIDs []uint) ([]repository.RubricAssessmentWithRubric, error) {
	var out []repository.RubricAssessmentWithRubric
	if len(assignmentIDs) == 0 {
		return out, nil
	}
	type row struct {
		AssignmentID   uint
		AssessmentID   uint
		RubricTitle    string
		RubricData     string
		PointsPossible float64
		HidePoints     bool
		HideScoreTotal bool
	}
	var rows []row
	err := r.db.WithContext(ctx).Table("rubric_assessments ra").
		Select(`ras.association_id AS assignment_id, ra.id AS assessment_id, rb.title AS rubric_title,
		        COALESCE(rb.data::text, '[]') AS rubric_data, rb.points_possible,
		        (ras.hide_points OR rb.hide_points) AS hide_points,
		        (ras.hide_score_total OR rb.hide_score_total) AS hide_score_total`).
		Joins("JOIN rubric_associations ras ON ras.id = ra.rubric_association_id").
		Joins("JOIN rubrics rb ON rb.id = ra.rubric_id").
		Where("ra.user_id = ? AND ra.assessment_type = ? AND ra.workflow_state <> ?", userID, "grading", "deleted").
		Where("ras.association_type = ? AND ras.association_id IN ?", "Assignment", assignmentIDs).
		Order("ra.updated_at ASC").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return out, nil
	}
	ids := make([]uint, len(rows))
	for i, rw := range rows {
		ids[i] = rw.AssessmentID
	}
	var assessments []models.RubricAssessment
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&assessments).Error; err != nil {
		return nil, err
	}
	byID := map[uint]models.RubricAssessment{}
	for _, a := range assessments {
		byID[a.ID] = a
	}
	for _, rw := range rows {
		out = append(out, repository.RubricAssessmentWithRubric{
			AssignmentID:   rw.AssignmentID,
			Assessment:     byID[rw.AssessmentID],
			RubricTitle:    rw.RubricTitle,
			RubricData:     rw.RubricData,
			PointsPossible: rw.PointsPossible,
			HidePoints:     rw.HidePoints,
			HideScoreTotal: rw.HideScoreTotal,
		})
	}
	return out, nil
}

func (r *learnerRepo) UsersByIDs(ctx context.Context, ids []uint) ([]models.User, error) {
	var out []models.User
	if len(ids) == 0 {
		return out, nil
	}
	err := r.db.WithContext(ctx).Select("id, account_id, name, short_name, sortable_name, avatar_url, pronouns").
		Where("id IN ?", ids).Find(&out).Error
	return out, err
}

// --- Observer alerts ---

func (r *learnerRepo) ListAlertThresholds(ctx context.Context, observerID uint, studentID *uint) ([]models.ObserverAlertThreshold, error) {
	var out []models.ObserverAlertThreshold
	q := r.db.WithContext(ctx).Where("observer_id = ?", observerID)
	if studentID != nil {
		q = q.Where("student_id = ?", *studentID)
	}
	err := q.Order("id").Find(&out).Error
	return out, err
}

func (r *learnerRepo) ListAllAlertThresholds(ctx context.Context) ([]models.ObserverAlertThreshold, error) {
	var out []models.ObserverAlertThreshold
	err := r.db.WithContext(ctx).Order("observer_id, student_id, id").Find(&out).Error
	return out, err
}

func (r *learnerRepo) ListAlertThresholdsForStudent(ctx context.Context, studentID uint, alertTypes []string) ([]models.ObserverAlertThreshold, error) {
	var out []models.ObserverAlertThreshold
	q := r.db.WithContext(ctx).Where("student_id = ?", studentID)
	if len(alertTypes) > 0 {
		q = q.Where("alert_type IN ?", alertTypes)
	}
	err := q.Order("observer_id, id").Find(&out).Error
	return out, err
}

func (r *learnerRepo) GetAlertThreshold(ctx context.Context, id, observerID uint) (*models.ObserverAlertThreshold, error) {
	var t models.ObserverAlertThreshold
	if err := r.db.WithContext(ctx).Where("id = ? AND observer_id = ?", id, observerID).First(&t).Error; err != nil {
		return nil, notFoundOr(err)
	}
	return &t, nil
}

func (r *learnerRepo) SaveAlertThreshold(ctx context.Context, t *models.ObserverAlertThreshold) error {
	if t.ID == 0 {
		return r.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "observer_id"}, {Name: "student_id"}, {Name: "alert_type"}},
			DoUpdates: clause.AssignmentColumns([]string{"threshold", "updated_at"}),
		}).Create(t).Error
	}
	return r.db.WithContext(ctx).Save(t).Error
}

func (r *learnerRepo) DeleteAlertThreshold(ctx context.Context, id, observerID uint) error {
	res := r.db.WithContext(ctx).Where("id = ? AND observer_id = ?", id, observerID).Delete(&models.ObserverAlertThreshold{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return repository.ErrLearnerNotFound
	}
	return nil
}

func (r *learnerRepo) TouchAlertThreshold(ctx context.Context, id uint, at time.Time) error {
	return r.db.WithContext(ctx).Model(&models.ObserverAlertThreshold{}).Where("id = ?", id).
		UpdateColumn("last_evaluated_at", at).Error
}

func (r *learnerRepo) CreateAlertIfAbsent(ctx context.Context, a *models.ObserverAlert) (bool, error) {
	res := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(a)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *learnerRepo) ClearHandledAlert(ctx context.Context, thresholdID uint, contextType string, contextID uint) error {
	return r.db.WithContext(ctx).
		Where("threshold_id = ? AND context_type = ? AND context_id = ? AND workflow_state <> ?",
			thresholdID, contextType, contextID, models.ObserverAlertUnread).
		Delete(&models.ObserverAlert{}).Error
}

func (r *learnerRepo) ListAlerts(ctx context.Context, observerID uint, studentID *uint, limit int) ([]models.ObserverAlert, error) {
	var out []models.ObserverAlert
	q := r.db.WithContext(ctx).Where("observer_id = ? AND workflow_state <> ?", observerID, models.ObserverAlertDismissed)
	if studentID != nil {
		q = q.Where("student_id = ?", *studentID)
	}
	if limit <= 0 {
		limit = 100
	}
	err := q.Order("action_date DESC, id DESC").Limit(limit).Find(&out).Error
	return out, err
}

func (r *learnerRepo) CountUnreadAlerts(ctx context.Context, observerID uint) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.ObserverAlert{}).
		Where("observer_id = ? AND workflow_state = ?", observerID, models.ObserverAlertUnread).Count(&n).Error
	return n, err
}

func (r *learnerRepo) GetAlert(ctx context.Context, id, observerID uint) (*models.ObserverAlert, error) {
	var a models.ObserverAlert
	if err := r.db.WithContext(ctx).Where("id = ? AND observer_id = ?", id, observerID).First(&a).Error; err != nil {
		return nil, notFoundOr(err)
	}
	return &a, nil
}

func (r *learnerRepo) UpdateAlertState(ctx context.Context, id uint, state models.ObserverAlertWorkflow) error {
	return r.db.WithContext(ctx).Model(&models.ObserverAlert{}).Where("id = ?", id).
		Updates(map[string]any{"workflow_state": state, "updated_at": time.Now()}).Error
}

// --- Profile links ---

func (r *learnerRepo) ListProfileLinks(ctx context.Context, userID uint) ([]models.UserProfileLink, error) {
	var out []models.UserProfileLink
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("position ASC, id ASC").Find(&out).Error
	return out, err
}

func (r *learnerRepo) ReplaceProfileLinks(ctx context.Context, userID uint, links []models.UserProfileLink) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&models.UserProfileLink{}).Error; err != nil {
			return err
		}
		for i := range links {
			links[i].ID = 0
			links[i].UserID = userID
			links[i].Position = i
			if err := tx.Create(&links[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *learnerRepo) UsersShareCourse(ctx context.Context, a, b uint) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("enrollments e1").
		Joins("JOIN enrollments e2 ON e2.course_id = e1.course_id").
		Where("e1.user_id = ? AND e2.user_id = ? AND e1.workflow_state = ? AND e2.workflow_state = ?",
			a, b, models.EnrollmentActive, models.EnrollmentActive).
		Limit(1).Count(&n).Error
	return n > 0, err
}
