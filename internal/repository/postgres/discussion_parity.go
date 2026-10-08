package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type discussionParityRepo struct {
	db *gorm.DB
}

// NewDiscussionParityRepository returns the Postgres implementation of
// repository.DiscussionParityRepository.
func NewDiscussionParityRepository(db *gorm.DB) repository.DiscussionParityRepository {
	return &discussionParityRepo{db: db}
}

// discussionEscapeLike escapes the LIKE wildcards in a user-supplied search term.
func discussionEscapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// unreadEntryExists is the "has an entry this user hasn't read" test for
// one topic id expression (own posts never count).
const unreadEntryExists = `EXISTS (
	SELECT 1 FROM discussion_entries ue
	WHERE ue.discussion_topic_id = %s
	  AND ue.workflow_state <> 'deleted'
	  AND ue.user_id <> @uid
	  AND NOT EXISTS (
		SELECT 1 FROM discussion_entry_participants up
		WHERE up.discussion_entry_id = ue.id AND up.user_id = @uid AND up.read_at IS NOT NULL
	  )
)`

func (r *discussionParityRepo) ListTopicsIndex(ctx context.Context, q repository.DiscussionIndexQuery) (*repository.PaginatedResult[models.DiscussionTopic], error) {
	query := r.db.WithContext(ctx).Model(&models.DiscussionTopic{}).
		Where("course_id = ? AND workflow_state <> ? AND root_topic_id IS NULL", q.CourseID, "deleted")
	if q.AccountID != 0 {
		query = query.Where("course_id IN (SELECT id FROM courses WHERE account_id = ?)", q.AccountID)
	}
	if q.StudentView {
		query = query.
			Where("workflow_state <> ?", "unpublished").
			Where("(workflow_state <> ? OR delayed_post_at IS NOT NULL)", "post_delayed").
			Where("(delayed_post_at IS NULL OR delayed_post_at <= ?)", q.Now).
			Where(`(is_section_specific IS NOT TRUE OR EXISTS (
				SELECT 1 FROM discussion_topic_section_visibilities v
				JOIN enrollments en ON en.course_section_id = v.course_section_id
				WHERE v.discussion_topic_id = discussion_topics.id
				  AND en.user_id = ? AND en.course_id = discussion_topics.course_id
				  AND en.workflow_state = 'active'))`, q.UserID)
	}
	if term := strings.TrimSpace(q.Search); term != "" {
		like := "%" + discussionEscapeLike(term) + "%"
		query = query.Where("(title ILIKE ? OR message ILIKE ?)", like, like)
	}
	if q.UnreadOnly {
		own := strings.Replace(unreadEntryExists, "%s", "discussion_topics.id", 1)
		// For a group discussion the posts live on the child topics; a
		// student only counts the topics of their own groups.
		childFilter := ""
		if q.StudentView {
			childFilter = ` AND c.group_id IN (SELECT gm.group_id FROM group_memberships gm WHERE gm.user_id = @uid AND gm.workflow_state = 'accepted')`
		}
		child := `EXISTS (SELECT 1 FROM discussion_topics c WHERE c.root_topic_id = discussion_topics.id AND c.workflow_state <> 'deleted'` + childFilter +
			` AND ` + strings.Replace(unreadEntryExists, "%s", "c.id", 1) + `)`
		query = query.Where("("+own+" OR "+child+")", map[string]interface{}{"uid": q.UserID})
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, err
	}

	var topics []models.DiscussionTopic
	offset := (q.Params.Page - 1) * q.Params.PerPage
	if err := query.Offset(offset).Limit(q.Params.PerPage).Order("pinned DESC, created_at DESC, id DESC").Find(&topics).Error; err != nil {
		return nil, err
	}
	return &repository.PaginatedResult[models.DiscussionTopic]{
		Items:      topics,
		TotalCount: count,
		Page:       q.Params.Page,
		PerPage:    q.Params.PerPage,
	}, nil
}

func (r *discussionParityRepo) TopicStats(ctx context.Context, topicIDs []uint, userID uint) (map[uint]repository.DiscussionTopicStats, error) {
	out := make(map[uint]repository.DiscussionTopicStats, len(topicIDs))
	if len(topicIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		TopicID     uint
		EntryCount  int64
		UnreadCount int64
		LastReplyAt *time.Time
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT e.discussion_topic_id AS topic_id,
		       COUNT(*) AS entry_count,
		       COUNT(*) FILTER (WHERE e.user_id <> ? AND NOT EXISTS (
		           SELECT 1 FROM discussion_entry_participants p
		           WHERE p.discussion_entry_id = e.id AND p.user_id = ? AND p.read_at IS NOT NULL
		       )) AS unread_count,
		       MAX(e.created_at) AS last_reply_at
		  FROM discussion_entries e
		 WHERE e.discussion_topic_id IN ? AND e.workflow_state <> 'deleted'
		 GROUP BY e.discussion_topic_id`, userID, userID, topicIDs).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.TopicID] = repository.DiscussionTopicStats{
			EntryCount:  row.EntryCount,
			UnreadCount: row.UnreadCount,
			LastReplyAt: row.LastReplyAt,
		}
	}
	return out, nil
}

func (r *discussionParityRepo) ReplaceTopicSections(ctx context.Context, topicID uint, sectionIDs []uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("discussion_topic_id = ?", topicID).Delete(&models.DiscussionTopicSectionVisibility{}).Error; err != nil {
			return err
		}
		seen := map[uint]bool{}
		for _, sid := range sectionIDs {
			if seen[sid] {
				continue
			}
			seen[sid] = true
			row := &models.DiscussionTopicSectionVisibility{DiscussionTopicID: topicID, CourseSectionID: sid}
			if err := tx.Create(row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *discussionParityRepo) ListTopicSectionIDs(ctx context.Context, topicIDs []uint) (map[uint][]uint, error) {
	out := map[uint][]uint{}
	if len(topicIDs) == 0 {
		return out, nil
	}
	var rows []models.DiscussionTopicSectionVisibility
	if err := r.db.WithContext(ctx).Where("discussion_topic_id IN ?", topicIDs).Order("course_section_id").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.DiscussionTopicID] = append(out[row.DiscussionTopicID], row.CourseSectionID)
	}
	return out, nil
}

func (r *discussionParityRepo) CountCourseSections(ctx context.Context, courseID uint, sectionIDs []uint) (int64, error) {
	if len(sectionIDs) == 0 {
		return 0, nil
	}
	var n int64
	err := r.db.WithContext(ctx).Model(&models.CourseSection{}).
		Where("course_id = ? AND id IN ? AND workflow_state <> ?", courseID, sectionIDs, "deleted").
		Count(&n).Error
	return n, err
}

func (r *discussionParityRepo) UserSectionIDs(ctx context.Context, courseID, userID uint) ([]uint, error) {
	var ids []uint
	err := r.db.WithContext(ctx).Model(&models.Enrollment{}).
		Where("course_id = ? AND user_id = ? AND workflow_state = ? AND course_section_id IS NOT NULL", courseID, userID, "active").
		Distinct().Pluck("course_section_id", &ids).Error
	return ids, err
}

func (r *discussionParityRepo) GroupCategoryInCourse(ctx context.Context, categoryID, courseID uint) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.GroupCategory{}).
		Where("id = ? AND course_id = ? AND workflow_state <> ?", categoryID, courseID, "deleted").
		Count(&n).Error
	return n > 0, err
}

func (r *discussionParityRepo) ListActiveGroups(ctx context.Context, categoryID uint) ([]models.Group, error) {
	var groups []models.Group
	err := r.db.WithContext(ctx).
		Where("group_category_id = ? AND workflow_state <> ?", categoryID, "deleted").
		Order("name ASC, id ASC").Find(&groups).Error
	return groups, err
}

func (r *discussionParityRepo) ListChildTopics(ctx context.Context, rootTopicID uint) ([]models.DiscussionTopic, error) {
	var topics []models.DiscussionTopic
	err := r.db.WithContext(ctx).
		Where("root_topic_id = ? AND workflow_state <> ?", rootTopicID, "deleted").
		Order("id ASC").Find(&topics).Error
	return topics, err
}

func (r *discussionParityRepo) IsGroupMember(ctx context.Context, groupID, userID uint) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.GroupMembership{}).
		Where("group_id = ? AND user_id = ? AND workflow_state = ?", groupID, userID, "accepted").
		Count(&n).Error
	return n > 0, err
}

func (r *discussionParityRepo) UserGroupIDs(ctx context.Context, categoryID, userID uint) ([]uint, error) {
	var ids []uint
	err := r.db.WithContext(ctx).Model(&models.GroupMembership{}).
		Joins("JOIN groups ON groups.id = group_memberships.group_id").
		Where("groups.group_category_id = ? AND groups.workflow_state <> ? AND group_memberships.user_id = ? AND group_memberships.workflow_state = ?",
			categoryID, "deleted", userID, "accepted").
		Order("groups.id").
		Pluck("groups.id", &ids).Error
	return ids, err
}

func (r *discussionParityRepo) ListGroupMemberIDs(ctx context.Context, groupID uint) ([]uint, error) {
	var ids []uint
	err := r.db.WithContext(ctx).Model(&models.GroupMembership{}).
		Where("group_id = ? AND workflow_state = ?", groupID, "accepted").
		Pluck("user_id", &ids).Error
	return ids, err
}

// CreateChildTopic creates a group's child topic. When a concurrent
// request created the same (root, group) child first, the unique index
// idx_discussion_topics_root_group makes this a no-op and topic is
// loaded from the existing row instead, so a group never gets two copies.
func (r *discussionParityRepo) CreateChildTopic(ctx context.Context, topic *models.DiscussionTopic) error {
	res := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(topic)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 || topic.RootTopicID == nil || topic.GroupID == nil {
		return nil
	}
	var existing models.DiscussionTopic
	if err := r.db.WithContext(ctx).
		Where("root_topic_id = ? AND group_id = ? AND workflow_state <> ?", *topic.RootTopicID, *topic.GroupID, "deleted").
		First(&existing).Error; err != nil {
		return err
	}
	*topic = existing
	return nil
}

func (r *discussionParityRepo) UserHasPosted(ctx context.Context, topicID, userID uint) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.DiscussionEntry{}).
		Where("discussion_topic_id = ? AND user_id = ? AND workflow_state <> ?", topicID, userID, "deleted").
		Limit(1).Count(&n).Error
	return n > 0, err
}

func (r *discussionParityRepo) ListUserEntries(ctx context.Context, topicIDs []uint, userID uint) ([]models.DiscussionEntry, error) {
	var entries []models.DiscussionEntry
	if len(topicIDs) == 0 {
		return entries, nil
	}
	err := r.db.WithContext(ctx).
		Where("discussion_topic_id IN ? AND user_id = ? AND workflow_state <> ?", topicIDs, userID, "deleted").
		Order("created_at ASC, id ASC").Find(&entries).Error
	return entries, err
}

func (r *discussionParityRepo) ListPosterIDs(ctx context.Context, topicID uint) ([]uint, error) {
	var ids []uint
	err := r.db.WithContext(ctx).Model(&models.DiscussionEntry{}).
		Where("discussion_topic_id = ? AND workflow_state <> ?", topicID, "deleted").
		Distinct().Pluck("user_id", &ids).Error
	return ids, err
}

// newAnonymousID returns a short random alias. Random (not derived from
// the user id) so it can't be reversed by hashing candidate ids.
func newAnonymousID() (string, error) {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b)[:5], nil
}

func (r *discussionParityRepo) AnonymousIDs(ctx context.Context, topicID uint, userIDs []uint) (map[uint]string, error) {
	out := map[uint]string{}
	if len(userIDs) == 0 {
		return out, nil
	}
	var rows []models.DiscussionTopicParticipant
	if err := r.db.WithContext(ctx).Where("discussion_topic_id = ?", topicID).Find(&rows).Error; err != nil {
		return nil, err
	}
	used := map[string]bool{}
	for _, p := range rows {
		if p.AnonymousID != nil && *p.AnonymousID != "" {
			out[p.UserID] = *p.AnonymousID
			used[*p.AnonymousID] = true
		}
	}
	for _, uid := range userIDs {
		if _, ok := out[uid]; ok {
			continue
		}
		var alias string
		for attempt := 0; attempt < 20; attempt++ {
			a, err := newAnonymousID()
			if err != nil {
				return nil, err
			}
			if !used[a] {
				alias = a
				break
			}
		}
		if alias == "" {
			continue
		}
		now := time.Now()
		// An existing row keeps its subscription choice. A new row is
		// seeded with the Canvas default (the topic author and anyone
		// who has posted are subscribed), since a participant row with
		// an explicit subscribed value overrides that default.
		res := r.db.WithContext(ctx).Exec(`
			INSERT INTO discussion_topic_participants (discussion_topic_id, user_id, subscribed, anonymous_id, created_at, updated_at)
			SELECT t.id, ?, (t.user_id = ? OR EXISTS (
			           SELECT 1 FROM discussion_entries e
			           WHERE e.discussion_topic_id = t.id AND e.user_id = ? AND e.workflow_state <> 'deleted')),
			       ?, ?, ?
			  FROM discussion_topics t WHERE t.id = ?
			ON CONFLICT (discussion_topic_id, user_id)
			DO UPDATE SET anonymous_id = COALESCE(discussion_topic_participants.anonymous_id, EXCLUDED.anonymous_id), updated_at = EXCLUDED.updated_at`,
			uid, uid, uid, alias, now, now, topicID)
		if res.Error != nil {
			return nil, res.Error
		}
		var stored models.DiscussionTopicParticipant
		if err := r.db.WithContext(ctx).Where("discussion_topic_id = ? AND user_id = ?", topicID, uid).First(&stored).Error; err != nil {
			return nil, err
		}
		if stored.AnonymousID != nil {
			out[uid] = *stored.AnonymousID
			used[*stored.AnonymousID] = true
		}
	}
	return out, nil
}

func (r *discussionParityRepo) ListMentionedUserIDs(ctx context.Context, entryID uint) ([]uint, error) {
	var ids []uint
	err := r.db.WithContext(ctx).Model(&models.DiscussionEntryMention{}).
		Where("discussion_entry_id = ?", entryID).Pluck("user_id", &ids).Error
	return ids, err
}

func (r *discussionParityRepo) AddMentions(ctx context.Context, entryID uint, userIDs []uint) error {
	now := time.Now()
	for _, uid := range userIDs {
		if err := r.db.WithContext(ctx).Exec(
			`INSERT INTO discussion_entry_mentions (discussion_entry_id, user_id, created_at) VALUES (?, ?, ?)
			 ON CONFLICT (discussion_entry_id, user_id) DO NOTHING`, entryID, uid, now).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *discussionParityRepo) ActiveCourseUserIDs(ctx context.Context, courseID uint, userIDs []uint) ([]uint, error) {
	var ids []uint
	if len(userIDs) == 0 {
		return ids, nil
	}
	err := r.db.WithContext(ctx).Model(&models.Enrollment{}).
		Where("course_id = ? AND user_id IN ? AND workflow_state = ?", courseID, userIDs, "active").
		Distinct().Pluck("user_id", &ids).Error
	return ids, err
}

func (r *discussionParityRepo) IsActiveStudent(ctx context.Context, courseID, userID uint) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.Enrollment{}).
		Where("course_id = ? AND user_id = ? AND type = ? AND workflow_state = ?", courseID, userID, "StudentEnrollment", "active").
		Count(&n).Error
	return n > 0, err
}

func (r *discussionParityRepo) CourseStaffIDs(ctx context.Context, courseID uint) ([]uint, error) {
	var ids []uint
	err := r.db.WithContext(ctx).Model(&models.Enrollment{}).
		Where("course_id = ? AND type IN ? AND workflow_state = ?", courseID, []string{"TeacherEnrollment", "TaEnrollment"}, "active").
		Distinct().Pluck("user_id", &ids).Error
	return ids, err
}

func (r *discussionParityRepo) ObservedStudentIDs(ctx context.Context, courseID, observerID uint) ([]uint, error) {
	var ids []uint
	err := r.db.WithContext(ctx).Model(&models.Enrollment{}).
		Where("course_id = ? AND user_id = ? AND type = ? AND workflow_state = ? AND associated_user_id IS NOT NULL",
			courseID, observerID, "ObserverEnrollment", "active").
		Distinct().Pluck("associated_user_id", &ids).Error
	return ids, err
}

func (r *discussionParityRepo) SearchCourseUsers(ctx context.Context, courseID uint, search string, limit int) ([]repository.DiscussionUserBrief, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	q := r.db.WithContext(ctx).Table("users").
		Select("DISTINCT users.id AS id, users.name AS name").
		Joins("JOIN enrollments en ON en.user_id = users.id").
		Where("en.course_id = ? AND en.workflow_state = ?", courseID, "active")
	if term := strings.TrimSpace(search); term != "" {
		q = q.Where("users.name ILIKE ?", "%"+discussionEscapeLike(term)+"%")
	}
	var out []repository.DiscussionUserBrief
	err := q.Order("users.name ASC").Limit(limit).Scan(&out).Error
	return out, err
}

func (r *discussionParityRepo) UserNames(ctx context.Context, userIDs []uint) (map[uint]string, error) {
	out := map[uint]string{}
	if len(userIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		ID   uint
		Name string
	}
	if err := r.db.WithContext(ctx).Table("users").Select("id, name").Where("id IN ?", userIDs).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = row.Name
	}
	return out, nil
}

func (r *discussionParityRepo) FindTopicByAssignment(ctx context.Context, assignmentID, courseID uint) (*models.DiscussionTopic, error) {
	var topic models.DiscussionTopic
	err := r.db.WithContext(ctx).
		Where("assignment_id = ? AND course_id = ? AND root_topic_id IS NULL AND workflow_state <> ?", assignmentID, courseID, "deleted").
		Order("id ASC").First(&topic).Error
	if err != nil {
		return nil, err
	}
	return &topic, nil
}

func (r *discussionParityRepo) CountActiveSubmissions(ctx context.Context, assignmentID uint) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.Submission{}).
		Where("assignment_id = ? AND (submitted_at IS NOT NULL OR score IS NOT NULL)", assignmentID).
		Count(&n).Error
	return n, err
}

func (r *discussionParityRepo) AssignmentGroupInCourse(ctx context.Context, groupID, courseID uint) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.AssignmentGroup{}).
		Where("id = ? AND course_id = ? AND workflow_state <> ?", groupID, courseID, "deleted").
		Count(&n).Error
	return n > 0, err
}

func (r *discussionParityRepo) ListActiveCourseUserIDs(ctx context.Context, courseID uint) ([]uint, error) {
	var ids []uint
	err := r.db.WithContext(ctx).Model(&models.Enrollment{}).
		Where("course_id = ? AND workflow_state = ?", courseID, "active").
		Distinct().Pluck("user_id", &ids).Error
	return ids, err
}

func (r *discussionParityRepo) ListDueDelayedTopics(ctx context.Context, now time.Time, limit int) ([]models.DiscussionTopic, error) {
	if limit <= 0 {
		limit = 200
	}
	var topics []models.DiscussionTopic
	err := r.db.WithContext(ctx).
		Where("workflow_state = ? AND root_topic_id IS NULL AND delayed_post_at IS NOT NULL AND delayed_post_at <= ?", "post_delayed", now).
		Order("delayed_post_at ASC, id ASC").Limit(limit).Find(&topics).Error
	return topics, err
}
