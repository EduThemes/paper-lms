package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

// assignmentSummaryJSON is the "assignment" object of a graded topic.
func assignmentSummaryJSON(a *models.Assignment) fiber.Map {
	return fiber.Map{
		"id":                  a.ID,
		"name":                a.Name,
		"points_possible":     a.PointsPossible,
		"grading_type":        a.GradingType,
		"due_at":              a.DueAt,
		"unlock_at":           a.UnlockAt,
		"lock_at":             a.LockAt,
		"assignment_group_id": a.AssignmentGroupID,
		"published":           a.IsPublished(),
	}
}

// addDiscussionTopicExtras adds the per-viewer and parity fields to a
// topic's JSON: whether the viewer may post, must post first, the graded
// assignment, the sections of a section-specific topic (staff), and for
// a group discussion either the group topics (staff) or the viewer's own
// group topic (students).
func addDiscussionTopicExtras(c *fiber.Ctx, svc interface{}, parity *service.DiscussionParityService, topic *models.DiscussionTopic, out fiber.Map) {
	staff := discussionCallerIsStaff(c)
	locked := discussionTopicLockedForCaller(c, topic)
	out["locked_for_user"] = locked
	out["initial_post_required"] = discussionInitialPostRequired(c, svc, topic)
	out["can_post"] = !locked && !discussionIsGroupRoot(topic)
	out["is_group_root"] = discussionIsGroupRoot(topic)

	if parity == nil {
		return
	}
	ctx := c.Context()

	gradedTopic := topic
	if topic.RootTopicID != nil {
		// accountID=0: internal hop; the child was resolved in the
		// caller's tenant and course, and the root shares that course.
		if root, err := parity.ChildRoot(ctx, topic); err == nil && root != nil {
			gradedTopic = root
			names := parity.GroupNames(ctx, root)
			if topic.GroupID != nil {
				out["group_name"] = names[*topic.GroupID]
			}
			out["root_title"] = root.Title
		}
	}
	if a := parity.GradedAssignment(ctx, gradedTopic); a != nil {
		out["assignment"] = assignmentSummaryJSON(a)
		out["graded"] = true
	} else {
		out["graded"] = false
	}

	if staff {
		// The editor locks the group set and anonymity once people post.
		posted, _ := parity.TopicHasPosts(ctx, topic)
		out["has_posts"] = posted
	}
	if staff && topic.IsSectionSpecific {
		out["section_ids"] = parity.TopicSectionIDs(ctx, []uint{topic.ID})[topic.ID]
	}

	if !discussionIsGroupRoot(topic) {
		return
	}
	userID, _ := c.Locals("user_id").(uint)
	if staff {
		children, err := parity.SyncGroupTopics(ctx, topic)
		if err != nil {
			return
		}
		names := parity.GroupNames(ctx, topic)
		stats := parity.TopicStatsFor(ctx, children, userID)
		list := make([]fiber.Map, 0, len(children))
		for _, ch := range children {
			name := ""
			if ch.GroupID != nil {
				name = names[*ch.GroupID]
			}
			st := stats[ch.ID]
			list = append(list, fiber.Map{
				"id":           ch.ID,
				"group_id":     ch.GroupID,
				"group_name":   name,
				"entry_count":  st.EntryCount,
				"unread_count": st.UnreadCount,
			})
		}
		out["group_topics"] = list
		return
	}
	child, err := parity.ChildTopicForUser(ctx, topic, userID)
	if err == nil && child != nil {
		out["group_topic_id"] = child.ID
	} else {
		out["group_topic_id"] = nil
	}
}
