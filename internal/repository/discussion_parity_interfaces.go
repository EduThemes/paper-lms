package repository

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// DiscussionIndexQuery describes one page of a course's discussion index.
type DiscussionIndexQuery struct {
	CourseID  uint
	AccountID uint
	// StudentView restricts the list to topics a student may open:
	// published, past their delayed_post_at, and (for section-specific
	// topics) assigned to one of UserID's sections.
	StudentView bool
	UserID      uint
	Now         time.Time
	// Search matches the title or message (case-insensitive).
	Search string
	// UnreadOnly keeps topics with at least one entry UserID hasn't read.
	UnreadOnly bool
	Params     PaginationParams
}

// DiscussionTopicStats are the per-viewer counters shown on the index.
type DiscussionTopicStats struct {
	EntryCount  int64
	UnreadCount int64
	LastReplyAt *time.Time
}

// DiscussionUserBrief is the minimum shown in the @mention picker.
type DiscussionUserBrief struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// DiscussionParityRepository holds the queries behind group, section,
// anonymous, graded and index features of discussions. Every method that
// takes a topic id expects the caller to have resolved that topic in the
// caller's tenant and course first.
type DiscussionParityRepository interface {
	// Index
	ListTopicsIndex(ctx context.Context, q DiscussionIndexQuery) (*PaginatedResult[models.DiscussionTopic], error)
	// TopicStats counts the live entries of each topic and those userID
	// hasn't read (their own posts never count as unread).
	TopicStats(ctx context.Context, topicIDs []uint, userID uint) (map[uint]DiscussionTopicStats, error)

	// Sections
	ReplaceTopicSections(ctx context.Context, topicID uint, sectionIDs []uint) error
	ListTopicSectionIDs(ctx context.Context, topicIDs []uint) (map[uint][]uint, error)
	// CountCourseSections returns how many of sectionIDs are active
	// sections of courseID.
	CountCourseSections(ctx context.Context, courseID uint, sectionIDs []uint) (int64, error)
	// UserSectionIDs lists the sections of the user's active enrollments
	// in the course.
	UserSectionIDs(ctx context.Context, courseID, userID uint) ([]uint, error)

	// Groups
	GroupCategoryInCourse(ctx context.Context, categoryID, courseID uint) (bool, error)
	ListActiveGroups(ctx context.Context, categoryID uint) ([]models.Group, error)
	ListChildTopics(ctx context.Context, rootTopicID uint) ([]models.DiscussionTopic, error)
	IsGroupMember(ctx context.Context, groupID, userID uint) (bool, error)
	UserGroupIDs(ctx context.Context, categoryID, userID uint) ([]uint, error)
	ListGroupMemberIDs(ctx context.Context, groupID uint) ([]uint, error)
	CreateChildTopic(ctx context.Context, topic *models.DiscussionTopic) error

	// Entries
	UserHasPosted(ctx context.Context, topicID, userID uint) (bool, error)
	// ListUserEntries returns userID's live entries on the given topics,
	// oldest first.
	ListUserEntries(ctx context.Context, topicIDs []uint, userID uint) ([]models.DiscussionEntry, error)
	ListPosterIDs(ctx context.Context, topicID uint) ([]uint, error)

	// Participants
	// AnonymousIDs returns (creating where missing) the alias of each
	// user in the topic.
	AnonymousIDs(ctx context.Context, topicID uint, userIDs []uint) (map[uint]string, error)

	// Mentions
	ListMentionedUserIDs(ctx context.Context, entryID uint) ([]uint, error)
	AddMentions(ctx context.Context, entryID uint, userIDs []uint) error

	// Course membership
	// ActiveCourseUserIDs filters userIDs to those with an active
	// enrollment in the course.
	ActiveCourseUserIDs(ctx context.Context, courseID uint, userIDs []uint) ([]uint, error)
	// ActiveCourseStudentIDs lists the users with an active student
	// enrollment in the course.
	IsActiveStudent(ctx context.Context, courseID, userID uint) (bool, error)
	CourseStaffIDs(ctx context.Context, courseID uint) ([]uint, error)
	// ObservedStudentIDs lists the students an observer observes in the
	// course (active ObserverEnrollment rows).
	ObservedStudentIDs(ctx context.Context, courseID, observerID uint) ([]uint, error)
	// ListActiveCourseUserIDs lists every user with an active enrollment.
	ListActiveCourseUserIDs(ctx context.Context, courseID uint) ([]uint, error)
	// SearchCourseUsers lists active members of the course whose name
	// matches search (all when empty), up to limit.
	SearchCourseUsers(ctx context.Context, courseID uint, search string, limit int) ([]DiscussionUserBrief, error)
	// FindUsersByIDs returns the named users that exist.
	UserNames(ctx context.Context, userIDs []uint) (map[uint]string, error)

	// ListDueDelayedTopics lists root topics still post_delayed whose
	// delayed_post_at has passed (across all courses; background job).
	ListDueDelayedTopics(ctx context.Context, now time.Time, limit int) ([]models.DiscussionTopic, error)

	// Graded discussions
	FindTopicByAssignment(ctx context.Context, assignmentID, courseID uint) (*models.DiscussionTopic, error)
	CountActiveSubmissions(ctx context.Context, assignmentID uint) (int64, error)
	AssignmentGroupInCourse(ctx context.Context, groupID, courseID uint) (bool, error)
}
