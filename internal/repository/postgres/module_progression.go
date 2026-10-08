package postgres

import (
	"context"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// ModuleProgressionRepository holds the queries the module progression
// engine runs: the published module structure of a course, the
// per-student facts requirements are evaluated against, the
// view/mark-done actions, and the cached progression rows.
type ModuleProgressionRepository struct {
	db *gorm.DB
}

func NewModuleProgressionRepository(db *gorm.DB) *ModuleProgressionRepository {
	return &ModuleProgressionRepository{db: db}
}

// ListPublishedModules returns the course's published modules, each with
// only its published items, ordered by position.
func (r *ModuleProgressionRepository) ListPublishedModules(ctx context.Context, courseID uint) ([]models.ContextModule, error) {
	var mods []models.ContextModule
	err := r.db.WithContext(ctx).
		Where("course_id = ? AND workflow_state = ?", courseID, "active").
		Preload("Items", func(db *gorm.DB) *gorm.DB {
			return db.Where("workflow_state = ?", models.ContentTagActive).Order("position ASC, id ASC")
		}).
		Order("position ASC, id ASC").
		Find(&mods).Error
	return mods, err
}

// ListPrerequisites maps each module id to its prerequisite module ids.
func (r *ModuleProgressionRepository) ListPrerequisites(ctx context.Context, moduleIDs []uint) (map[uint][]uint, error) {
	out := map[uint][]uint{}
	if len(moduleIDs) == 0 {
		return out, nil
	}
	var rows []models.ModulePrerequisite
	if err := r.db.WithContext(ctx).Where("module_id IN ?", moduleIDs).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, p := range rows {
		out[p.ModuleID] = append(out[p.ModuleID], p.PrerequisiteModuleID)
	}
	return out, nil
}

// ListItemActions returns the user's view/mark-done rows for the items.
func (r *ModuleProgressionRepository) ListItemActions(ctx context.Context, userID uint, itemIDs []uint) (map[uint]models.ModuleItemUserAction, error) {
	out := map[uint]models.ModuleItemUserAction{}
	if len(itemIDs) == 0 {
		return out, nil
	}
	var rows []models.ModuleItemUserAction
	if err := r.db.WithContext(ctx).Where("user_id = ? AND content_tag_id IN ?", userID, itemIDs).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, a := range rows {
		out[a.ContentTagID] = a
	}
	return out, nil
}

// RecordViews stamps viewed_at on each item for the user, keeping the
// first view time.
func (r *ModuleProgressionRepository) RecordViews(ctx context.Context, userID uint, itemIDs []uint, at time.Time) error {
	if len(itemIDs) == 0 {
		return nil
	}
	rows := make([]models.ModuleItemUserAction, 0, len(itemIDs))
	for _, id := range itemIDs {
		t := at
		rows = append(rows, models.ModuleItemUserAction{ContentTagID: id, UserID: userID, ViewedAt: &t})
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "content_tag_id"}, {Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"viewed_at":  gorm.Expr("COALESCE(module_item_user_actions.viewed_at, EXCLUDED.viewed_at)"),
			"updated_at": gorm.Expr("EXCLUDED.updated_at"),
		}),
	}).Create(&rows).Error
}

// SetMarkedDone sets (done=true) or clears the user's mark-done on item.
func (r *ModuleProgressionRepository) SetMarkedDone(ctx context.Context, userID, itemID uint, done bool, at time.Time) error {
	var doneAt *time.Time
	if done {
		t := at
		doneAt = &t
	}
	row := models.ModuleItemUserAction{ContentTagID: itemID, UserID: userID, MarkedDoneAt: doneAt}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "content_tag_id"}, {Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"marked_done_at": gorm.Expr("EXCLUDED.marked_done_at"),
			"updated_at":     gorm.Expr("EXCLUDED.updated_at"),
		}),
	}).Create(&row).Error
}

// SubmissionFact is the slice of a submission row the engine needs.
type SubmissionFact struct {
	AssignmentID  uint
	WorkflowState string
	Score         *float64
	PostedAt      *time.Time
	Excused       bool
	Attempt       int
	SubmittedAt   *time.Time
}

// ListSubmissionFacts returns the user's submissions keyed by assignment.
func (r *ModuleProgressionRepository) ListSubmissionFacts(ctx context.Context, userID uint, assignmentIDs []uint) (map[uint]SubmissionFact, error) {
	out := map[uint]SubmissionFact{}
	if len(assignmentIDs) == 0 {
		return out, nil
	}
	var rows []SubmissionFact
	err := r.db.WithContext(ctx).Table("submissions").
		Select("assignment_id, workflow_state, score, posted_at, excused, attempt, submitted_at").
		Where("user_id = ? AND assignment_id IN ?", userID, assignmentIDs).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, s := range rows {
		out[s.AssignmentID] = s
	}
	return out, nil
}

// QuizFact summarises the user's quiz submissions for one quiz.
type QuizFact struct {
	QuizID uint
	// Finished: at least one attempt was turned in (complete or
	// pending_review).
	Finished bool
	// BestScore is the highest kept/raw score among graded attempts
	// whose linked assignment submission (if any) is posted.
	BestScore *float64
	Excused   bool
}

// ListQuizFacts returns the user's quiz results keyed by quiz.
func (r *ModuleProgressionRepository) ListQuizFacts(ctx context.Context, userID uint, quizIDs []uint) (map[uint]QuizFact, error) {
	out := map[uint]QuizFact{}
	if len(quizIDs) == 0 {
		return out, nil
	}
	type row struct {
		QuizID        uint
		WorkflowState string
		Score         *float64
		KeptScore     *float64
		SubmissionID  *uint
		PostedAt      *time.Time
		Excused       *bool
	}
	var rows []row
	err := r.db.WithContext(ctx).Table("quiz_submissions AS qs").
		Select("qs.quiz_id, qs.workflow_state, qs.score, qs.kept_score, qs.submission_id, s.posted_at, s.excused").
		Joins("LEFT JOIN submissions s ON s.id = qs.submission_id").
		Where("qs.user_id = ? AND qs.quiz_id IN ?", userID, quizIDs).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, q := range rows {
		f := out[q.QuizID]
		f.QuizID = q.QuizID
		if q.Excused != nil && *q.Excused {
			f.Excused = true
		}
		finished := q.WorkflowState == "complete" || q.WorkflowState == "pending_review"
		if finished {
			f.Finished = true
		}
		// A linked assignment submission that is not posted hides the
		// grade: it may not satisfy a min_score yet (Canvas).
		posted := q.SubmissionID == nil || q.PostedAt != nil
		if q.WorkflowState == "complete" && posted {
			sc := q.KeptScore
			if sc == nil {
				sc = q.Score
			}
			if sc != nil && (f.BestScore == nil || *sc > *f.BestScore) {
				v := *sc
				f.BestScore = &v
			}
		}
		out[q.QuizID] = f
	}
	return out, nil
}

// DiscussionFact is the user's participation in one topic.
type DiscussionFact struct {
	TopicID      uint
	AssignmentID *uint
	Contributed  bool
}

// ListDiscussionFacts returns, per topic, its graded assignment (if any)
// and whether the user has posted a live entry.
func (r *ModuleProgressionRepository) ListDiscussionFacts(ctx context.Context, userID uint, topicIDs []uint) (map[uint]DiscussionFact, error) {
	out := map[uint]DiscussionFact{}
	if len(topicIDs) == 0 {
		return out, nil
	}
	var topics []models.DiscussionTopic
	if err := r.db.WithContext(ctx).Select("id, assignment_id").Where("id IN ?", topicIDs).Find(&topics).Error; err != nil {
		return nil, err
	}
	for _, t := range topics {
		out[t.ID] = DiscussionFact{TopicID: t.ID, AssignmentID: t.AssignmentID}
	}
	var posted []uint
	if err := r.db.WithContext(ctx).Table("discussion_entries").
		Where("user_id = ? AND discussion_topic_id IN ? AND workflow_state <> ?", userID, topicIDs, "deleted").
		Distinct().Pluck("discussion_topic_id", &posted).Error; err != nil {
		return nil, err
	}
	for _, id := range posted {
		f := out[id]
		f.TopicID = id
		f.Contributed = true
		out[id] = f
	}
	return out, nil
}

// ListProgressions returns the cached progression rows for the modules.
func (r *ModuleProgressionRepository) ListProgressions(ctx context.Context, userID uint, moduleIDs []uint) (map[uint]models.ContextModuleProgression, error) {
	out := map[uint]models.ContextModuleProgression{}
	if len(moduleIDs) == 0 {
		return out, nil
	}
	var rows []models.ContextModuleProgression
	if err := r.db.WithContext(ctx).Where("user_id = ? AND context_module_id IN ?", userID, moduleIDs).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, p := range rows {
		out[p.ContextModuleID] = p
	}
	return out, nil
}

// UpsertProgressions writes the evaluated progression rows.
func (r *ModuleProgressionRepository) UpsertProgressions(ctx context.Context, rows []models.ContextModuleProgression) error {
	if len(rows) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "context_module_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"workflow_state", "requirements_met", "current_position", "completed_at", "evaluated_at", "updated_at",
		}),
	}).Create(&rows).Error
}

// CourseStudent is one active student of a course.
type CourseStudent struct {
	ID           uint
	Name         string
	SortableName string
}

// ListActiveStudents returns the course's active students, by name.
func (r *ModuleProgressionRepository) ListActiveStudents(ctx context.Context, courseID uint) ([]CourseStudent, error) {
	var rows []CourseStudent
	err := r.db.WithContext(ctx).Table("users AS u").
		Select("DISTINCT u.id, u.name, u.sortable_name").
		Joins("JOIN enrollments e ON e.user_id = u.id").
		Where("e.course_id = ? AND e.type = ? AND e.workflow_state = ?", courseID, "StudentEnrollment", "active").
		Order("u.sortable_name ASC, u.name ASC, u.id ASC").
		Scan(&rows).Error
	return rows, err
}

// ActiveEnrollmentTypes returns the user's active enrollment types in
// the course.
func (r *ModuleProgressionRepository) ActiveEnrollmentTypes(ctx context.Context, userID, courseID uint) ([]string, error) {
	var types []string
	err := r.db.WithContext(ctx).Table("enrollments").
		Where("user_id = ? AND course_id = ? AND workflow_state = ?", userID, courseID, "active").
		Distinct().Pluck("type", &types).Error
	return types, err
}

// ObservedStudentIDs returns the students the observer observes in the
// course (active ObserverEnrollments with an associated user).
func (r *ModuleProgressionRepository) ObservedStudentIDs(ctx context.Context, observerID, courseID uint) ([]uint, error) {
	var ids []uint
	err := r.db.WithContext(ctx).Table("enrollments").
		Where("user_id = ? AND course_id = ? AND type = ? AND workflow_state = ? AND associated_user_id IS NOT NULL",
			observerID, courseID, "ObserverEnrollment", "active").
		Distinct().Pluck("associated_user_id", &ids).Error
	return ids, err
}

// CourseInAccount reports whether the course belongs to the tenant
// (accountID 0 disables the check).
func (r *ModuleProgressionRepository) CourseInAccount(ctx context.Context, courseID, accountID uint) (bool, error) {
	q := r.db.WithContext(ctx).Table("courses").Where("id = ?", courseID)
	if accountID != 0 {
		q = q.Where("account_id = ?", accountID)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// ListItemsForContent returns the published items of the course's
// published modules that link the given content.
func (r *ModuleProgressionRepository) ListItemsForContent(ctx context.Context, courseID uint, contentTypes []string, contentID uint) ([]models.ContentTag, error) {
	var items []models.ContentTag
	err := r.db.WithContext(ctx).Table("content_tags AS ct").
		Select("ct.*").
		Joins("JOIN context_modules cm ON cm.id = ct.context_module_id").
		Where("cm.course_id = ? AND cm.workflow_state = ? AND ct.workflow_state = ?", courseID, "active", models.ContentTagActive).
		Where("ct.content_type IN ? AND ct.content_id = ?", contentTypes, contentID).
		Scan(&items).Error
	return items, err
}

// ResolvePageID maps a page key from a URL (slug or numeric id) to the
// id of a live page of the course.
func (r *ModuleProgressionRepository) ResolvePageID(ctx context.Context, courseID uint, key string) (uint, bool, error) {
	q := r.db.WithContext(ctx).Table("wiki_pages").
		Where("course_id = ? AND workflow_state <> ?", courseID, "deleted")
	if n, err := strconv.ParseUint(key, 10, 64); err == nil {
		q = q.Where("url = ? OR id = ?", key, n)
	} else {
		q = q.Where("url = ?", key)
	}
	var ids []uint
	if err := q.Order("id ASC").Limit(1).Pluck("id", &ids).Error; err != nil {
		return 0, false, err
	}
	if len(ids) == 0 {
		return 0, false, nil
	}
	return ids[0], true, nil
}

// AttachmentCourseID returns the course an attachment belongs to (ok is
// false for user/group files and unknown ids).
func (r *ModuleProgressionRepository) AttachmentCourseID(ctx context.Context, attachmentID uint) (uint, bool, error) {
	var att models.Attachment
	err := r.db.WithContext(ctx).Select("id, context_type, context_id").Where("id = ?", attachmentID).Limit(1).Find(&att).Error
	if err != nil {
		return 0, false, err
	}
	if att.ID == 0 || att.ContextType != "Course" {
		return 0, false, nil
	}
	return att.ContextID, true, nil
}

// UnpublishedContent lists the course content a student may not open
// because it is a draft or deleted, keyed by kind ("Assignment", "Quiz",
// "DiscussionTopic", "WikiPage", "Attachment") then content id (G063).
// A module item linking such content is hidden from students, as Canvas
// keeps a module item's published state in step with its content.
func (r *ModuleProgressionRepository) UnpublishedContent(ctx context.Context, courseID uint) (map[string]map[uint]bool, error) {
	queries := []struct {
		kind, table, where string
	}{
		// Mirrors Assignment.IsPublished: 'published', or a legacy blank
		// state with the published flag set.
		{"Assignment", "assignments", "course_id = ? AND NOT (workflow_state = 'published' OR (workflow_state = '' AND published))"},
		// Mirrors quizVisibleToStudents.
		{"Quiz", "quizzes", "course_id = ? AND (workflow_state = 'deleted' OR NOT (published OR workflow_state = 'published'))"},
		{"DiscussionTopic", "discussion_topics", "course_id = ? AND workflow_state IN ('unpublished', 'deleted')"},
		{"WikiPage", "wiki_pages", "course_id = ? AND workflow_state <> 'active'"},
		{"Attachment", "attachments", "context_type = 'Course' AND context_id = ? AND workflow_state = 'deleted'"},
	}
	out := make(map[string]map[uint]bool, len(queries))
	for _, q := range queries {
		var ids []uint
		if err := r.db.WithContext(ctx).Table(q.table).Where(q.where, courseID).Pluck("id", &ids).Error; err != nil {
			return nil, err
		}
		set := make(map[uint]bool, len(ids))
		for _, id := range ids {
			set[id] = true
		}
		out[q.kind] = set
	}
	return out, nil
}
