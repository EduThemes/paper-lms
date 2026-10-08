package handlers

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

// discussionTopicGetter is satisfied by DiscussionService,
// DiscussionV2Service and DiscussionCheckpointService.
type discussionTopicGetter interface {
	GetTopic(ctx context.Context, id, accountID uint) (*models.DiscussionTopic, error)
}

// discussionEntryGetter is satisfied by DiscussionService and
// DiscussionV2Service.
type discussionEntryGetter interface {
	GetEntry(ctx context.Context, id, accountID uint) (*models.DiscussionEntry, error)
}

// discussionNow is swappable in tests.
var discussionNow = time.Now

// discussionCallerIsStaff reports whether the request's auth Locals mark
// the caller as course staff for discussion moderation: an admin
// (is_admin, set by the permission middleware) or a teacher/TA
// (enrollment_type, set by RequireEnrolled / RequireInstructor).
func discussionCallerIsStaff(c *fiber.Ctx) bool {
	if isAdmin, _ := c.Locals("is_admin").(bool); isAdmin {
		return true
	}
	et, _ := c.Locals("enrollment_type").(string)
	return et == "TeacherEnrollment" || et == "TaEnrollment"
}

// requireTopicInCourse resolves :topic_id under :course_id for the caller.
// It returns wrote=true after writing a response when the topic is
// missing, in another tenant, in another course (F-013 parent tie),
// soft-deleted, or not yet visible to a student (unpublished or
// delayed-post in the future). Every "not yours" case is the same 404
// so the caller can't probe for topics elsewhere. The caller MUST
// short-circuit (return nil) when wrote is true.
//
// The course middleware only guards :course_id, so without this tie a
// member of any course could address any topic by id.
func requireTopicInCourse(c *fiber.Ctx, svc discussionTopicGetter) (*models.DiscussionTopic, bool) {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		_ = responses.BadRequest(c, "Invalid course ID")
		return nil, true
	}
	topicID, err := c.ParamsInt("topic_id")
	if err != nil || topicID <= 0 {
		_ = responses.BadRequest(c, "Invalid topic ID")
		return nil, true
	}

	topic, err := svc.GetTopic(c.Context(), uint(topicID), callerAccountID(c))
	if err != nil || topic == nil ||
		topic.CourseID != uint(courseID) ||
		topic.WorkflowState == models.DiscussionTopicDeleted ||
		!discussionTopicVisibleToCaller(c, topic) ||
		!discussionAudienceAllows(c, svc, topic) {
		_ = responses.NotFound(c, "discussion topic")
		return nil, true
	}
	return topic, false
}

// discussionTopicVisibleToCaller mirrors Canvas: course staff see every
// topic; students don't see unpublished topics or topics whose
// delayed_post_at is still in the future.
func discussionTopicVisibleToCaller(c *fiber.Ctx, topic *models.DiscussionTopic) bool {
	if discussionCallerIsStaff(c) {
		return true
	}
	return service.DiscussionTopicVisibleToStudents(topic, discussionNow())
}

// discussionAudienceOf returns the section / group visibility rules
// installed on a discussion service, or nil.
func discussionAudienceOf(svc interface{}) service.DiscussionAudience {
	if p, ok := svc.(interface {
		Audience() service.DiscussionAudience
	}); ok {
		return p.Audience()
	}
	return nil
}

// discussionAudienceAllows applies the section-specific and group-topic
// rules to a non-staff caller (staff always pass).
func discussionAudienceAllows(c *fiber.Ctx, svc interface{}, topic *models.DiscussionTopic) bool {
	if discussionCallerIsStaff(c) {
		return true
	}
	aud := discussionAudienceOf(svc)
	if aud == nil {
		return true
	}
	userID, _ := c.Locals("user_id").(uint)
	return aud.TopicVisibleToUser(c.Context(), topic, userID)
}

// discussionInitialPostGate is implemented by the parity service.
type discussionInitialPostGate interface {
	InitialPostRequired(ctx context.Context, topic *models.DiscussionTopic, userID uint) bool
}

// discussionInitialPostRequired reports whether a require_initial_post
// topic still hides other people's posts from the caller (Canvas
// initial_post_required?). Staff are never gated.
func discussionInitialPostRequired(c *fiber.Ctx, svc interface{}, topic *models.DiscussionTopic) bool {
	if !topic.RequireInitialPost || discussionCallerIsStaff(c) {
		return false
	}
	gate, ok := discussionAudienceOf(svc).(discussionInitialPostGate)
	if !ok {
		return false
	}
	userID, _ := c.Locals("user_id").(uint)
	return gate.InitialPostRequired(c.Context(), topic, userID)
}

// discussionIsGroupRoot reports whether the topic is the root of a group
// discussion: its posts live on the per-group child topics.
func discussionIsGroupRoot(topic *models.DiscussionTopic) bool {
	return topic.GroupCategoryID != nil && topic.RootTopicID == nil
}

// requireTopicAcceptsPosts writes a 400 and returns wrote=true when the
// topic is a group discussion's root (members post in their group's
// topic instead).
func requireTopicAcceptsPosts(c *fiber.Ctx, topic *models.DiscussionTopic) bool {
	if discussionIsGroupRoot(topic) {
		_ = responses.BadRequest(c, "This is a group discussion: post in your group's topic")
		return true
	}
	return false
}

// discussionTopicLockedForCaller reports whether the caller may no longer
// post, reply, edit or delete in the topic. Course staff are never
// locked out (Canvas lets moderators keep working in locked topics).
func discussionTopicLockedForCaller(c *fiber.Ctx, topic *models.DiscussionTopic) bool {
	if discussionCallerIsStaff(c) {
		return false
	}
	if topic.Locked || topic.WorkflowState == models.DiscussionTopicLocked {
		return true
	}
	return topic.LockAt != nil && !topic.LockAt.After(discussionNow())
}

// requireTopicOpenForCaller writes a 403 and returns wrote=true when the
// topic is locked for the caller.
func requireTopicOpenForCaller(c *fiber.Ctx, topic *models.DiscussionTopic) bool {
	if discussionTopicLockedForCaller(c, topic) {
		_ = responses.Forbidden(c, "This discussion is locked")
		return true
	}
	return false
}

// requireEntryInTopic resolves the entry named by the route param under
// an already-resolved topic. It returns wrote=true after writing a 404
// when the entry is missing, in another tenant, on a different topic,
// or soft-deleted. The caller MUST short-circuit when wrote is true.
func requireEntryInTopic(c *fiber.Ctx, svc discussionEntryGetter, topicID uint, param string) (*models.DiscussionEntry, bool) {
	entryID, err := c.ParamsInt(param)
	if err != nil || entryID <= 0 {
		_ = responses.BadRequest(c, "Invalid entry ID")
		return nil, true
	}
	entry, err := svc.GetEntry(c.Context(), uint(entryID), callerAccountID(c))
	if err != nil || entry == nil ||
		entry.DiscussionTopicID != topicID ||
		entry.WorkflowState == discussionEntryDeletedState {
		_ = responses.NotFound(c, "discussion entry")
		return nil, true
	}
	return entry, false
}

// discussionEntryDeletedState is the soft-delete workflow_state on entries.
const discussionEntryDeletedState = "deleted"

// requireEntryModerator enforces who may edit or delete an entry: its
// author (while the topic is open for them) or course staff. Anyone
// else gets 403; the entry is already known to be visible to them, so
// this leaks nothing. Returns wrote=true when a response was written.
func requireEntryModerator(c *fiber.Ctx, topic *models.DiscussionTopic, entry *models.DiscussionEntry) bool {
	if discussionCallerIsStaff(c) {
		return false
	}
	userID, _ := c.Locals("user_id").(uint)
	if userID == 0 || entry.UserID != userID {
		_ = responses.Forbidden(c, "You can only change your own posts")
		return true
	}
	return requireTopicOpenForCaller(c, topic)
}

// discussionCallerMayRate mirrors Canvas DiscussionTopic :rate — the
// topic must allow rating, and when only graders may rate the caller
// must be course staff.
func discussionCallerMayRate(c *fiber.Ctx, topic *models.DiscussionTopic) bool {
	if !topic.AllowRating {
		return false
	}
	if topic.OnlyGradersCanRate && !discussionCallerIsStaff(c) {
		return false
	}
	return true
}
