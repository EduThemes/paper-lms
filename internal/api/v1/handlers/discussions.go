package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/gofiber/fiber/v2"
)

type DiscussionHandler struct {
	discussionService *service.DiscussionService
	// parity adds graded, group, section-specific and anonymous topics
	// and the index counters. Optional (nil in some tests).
	parity *service.DiscussionParityService
}

func NewDiscussionHandler(discussionService *service.DiscussionService) *DiscussionHandler {
	return &DiscussionHandler{discussionService: discussionService}
}

// SetParity enables the Canvas-parity discussion features on topics.
func (h *DiscussionHandler) SetParity(p *service.DiscussionParityService) *DiscussionHandler {
	h.parity = p
	return h
}

func topicToJSON(t *models.DiscussionTopic) fiber.Map {
	return fiber.Map{
		"id":                    t.ID,
		"course_id":             t.CourseID,
		"user_id":               t.UserID,
		"title":                 t.Title,
		"message":               t.Message,
		"discussion_type":       t.DiscussionType,
		"posted_at":             t.PostedAt,
		"delayed_post_at":       t.DelayedPostAt,
		"lock_at":               t.LockAt,
		"pinned":                t.Pinned,
		"locked":                t.Locked,
		"allow_rating":          t.AllowRating,
		"only_graders_can_rate": t.OnlyGradersCanRate,
		"sort_by_rating":        t.SortByRating,
		"require_initial_post":  t.RequireInitialPost,
		"assignment_id":         t.AssignmentID,
		"group_category_id":     t.GroupCategoryID,
		"root_topic_id":         t.RootTopicID,
		"group_id":              t.GroupID,
		"anonymous_state":       t.AnonymousState,
		"is_section_specific":   t.IsSectionSpecific,
		"workflow_state":        t.WorkflowState,
		"published":             t.WorkflowState != models.DiscussionTopicUnpublished,
		"created_at":            t.CreatedAt,
		"updated_at":            t.UpdatedAt,
	}
}

// discussionParityErr maps the parity service's validation errors to a
// 400; anything else is a 500. Returns the written response.
func discussionParityErr(c *fiber.Ctx, err error, fallback string) error {
	for _, known := range []error{
		service.ErrDiscussionInvalidSections, service.ErrDiscussionSectionsRequired,
		service.ErrDiscussionInvalidGroupSet, service.ErrDiscussionGroupSetLocked,
		service.ErrDiscussionInvalidAnonymity, service.ErrDiscussionAnonymousConflict,
		service.ErrDiscussionSectionConflict, service.ErrDiscussionGradedHasSubmission,
		service.ErrDiscussionInvalidAssignment, service.ErrDiscussionInvalidPoints,
	} {
		if errors.Is(err, known) {
			return responses.BadRequest(c, err.Error())
		}
	}
	if err != nil && strings.Contains(err.Error(), "grading_type") {
		return responses.BadRequest(c, err.Error())
	}
	return responses.InternalError(c, fallback)
}

// ListTopics — GET /courses/:course_id/discussion_topics
//
// Query: page, per_page, search_term (title / message), filter_by
// (all | unread). Each topic carries the viewer's reply and unread
// counts (Canvas discussion_subentry_count / unread_count).
func (h *DiscussionHandler) ListTopics(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)
	staff := discussionCallerIsStaff(c)

	if h.parity == nil {
		// Students only see topics they could open (published, not
		// delayed); staff see every topic in the course.
		result, err := h.discussionService.ListTopicsForViewer(c.Context(), uint(courseID), callerAccountID(c), !staff, params)
		if err != nil {
			return responses.InternalError(c, "Could not fetch discussion topics")
		}
		responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)
		topics := make([]fiber.Map, len(result.Items))
		for i, t := range result.Items {
			topics[i] = topicToJSON(&t)
		}
		return c.JSON(topics)
	}

	userID, _ := c.Locals("user_id").(uint)
	items, total, err := h.parity.ListIndex(c.Context(), repository.DiscussionIndexQuery{
		CourseID:    uint(courseID),
		AccountID:   callerAccountID(c),
		StudentView: !staff,
		UserID:      userID,
		Now:         discussionNow(),
		Search:      c.Query("search_term"),
		UnreadOnly:  c.Query("filter_by") == "unread",
		Params:      params,
	})
	if err != nil {
		return responses.InternalError(c, "Could not fetch discussion topics")
	}
	responses.SetPaginationHeaders(c, total, params.Page, params.PerPage)

	topics := make([]fiber.Map, len(items))
	for i := range items {
		it := &items[i]
		m := topicToJSON(&it.Topic)
		m["discussion_subentry_count"] = it.EntryCount
		m["unread_count"] = it.UnreadCount
		m["last_reply_at"] = it.LastReplyAt
		if it.UnreadCount > 0 {
			m["read_state"] = "unread"
		} else {
			m["read_state"] = "read"
		}
		m["locked_for_user"] = discussionTopicLockedForCaller(c, &it.Topic)
		if it.Topic.GroupCategoryID != nil {
			m["group_topic_count"] = it.GroupTopics
		}
		if staff && it.Topic.IsSectionSpecific {
			m["section_ids"] = it.SectionIDs
		}
		if a := h.parity.GradedAssignment(c.Context(), &it.Topic); a != nil {
			m["assignment"] = assignmentSummaryJSON(a)
		}
		topics[i] = m
	}
	return c.JSON(topics)
}

func (h *DiscussionHandler) GetTopic(c *fiber.Ctx) error {
	topic, wrote := requireTopicInCourse(c, h.discussionService)
	if wrote {
		return nil
	}

	out := topicToJSON(topic)
	addDiscussionTopicExtras(c, h.discussionService, h.parity, topic, out)
	return c.JSON(out)
}

// optionalJSON distinguishes an absent field from an explicit null.
type optionalJSON struct {
	Set  bool
	Null bool
	Raw  json.RawMessage
}

func (o *optionalJSON) UnmarshalJSON(b []byte) error {
	o.Set = true
	o.Null = bytes.Equal(bytes.TrimSpace(b), []byte("null"))
	o.Raw = append(o.Raw[:0], b...)
	return nil
}

func (o optionalJSON) timeValue() (*time.Time, error) {
	if !o.Set || o.Null {
		return nil, nil
	}
	var s string
	if err := json.Unmarshal(o.Raw, &s); err != nil {
		return nil, err
	}
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (o optionalJSON) uintValue() (*uint, error) {
	if !o.Set || o.Null {
		return nil, nil
	}
	var v uint
	if err := json.Unmarshal(o.Raw, &v); err != nil {
		return nil, err
	}
	if v == 0 {
		return nil, nil
	}
	return &v, nil
}

func (o optionalJSON) stringValue() (*string, error) {
	if !o.Set || o.Null {
		return nil, nil
	}
	var v string
	if err := json.Unmarshal(o.Raw, &v); err != nil {
		return nil, err
	}
	return service.NormalizeAnonymousState(&v), nil
}

func (o optionalJSON) gradedValue() (*service.DiscussionGradedInput, error) {
	if !o.Set || o.Null {
		return nil, nil
	}
	var v service.DiscussionGradedInput
	if err := json.Unmarshal(o.Raw, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// discussionTopicCreateInput is the create body (Canvas
// discussion_topic[...] parameters).
type discussionTopicCreateInput struct {
	Title              string     `json:"title"`
	Message            string     `json:"message"`
	DiscussionType     string     `json:"discussion_type"`
	PostedAt           *time.Time `json:"posted_at"`
	DelayedPostAt      *time.Time `json:"delayed_post_at"`
	LockAt             *time.Time `json:"lock_at"`
	Pinned             bool       `json:"pinned"`
	Locked             bool       `json:"locked"`
	AllowRating        bool       `json:"allow_rating"`
	OnlyGradersCanRate bool       `json:"only_graders_can_rate"`
	SortByRating       bool       `json:"sort_by_rating"`
	RequireInitialPost bool       `json:"require_initial_post"`
	AssignmentID       *uint      `json:"assignment_id"`
	// Published defaults to true; false creates a draft.
	Published         *bool                          `json:"published"`
	GroupCategoryID   *uint                          `json:"group_category_id"`
	AnonymousState    *string                        `json:"anonymous_state"`
	IsSectionSpecific bool                           `json:"is_section_specific"`
	SectionIDs        []uint                         `json:"section_ids"`
	Assignment        *service.DiscussionGradedInput `json:"assignment"`
}

func (h *DiscussionHandler) CreateTopic(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		DiscussionTopic discussionTopicCreateInput `json:"discussion_topic"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	in := input.DiscussionTopic

	if strings.TrimSpace(in.Title) == "" {
		return responses.BadRequest(c, "Discussion title is required")
	}

	userID, _ := c.Locals("user_id").(uint)

	topic := &models.DiscussionTopic{
		CourseID:           uint(courseID),
		UserID:             userID,
		Title:              in.Title,
		Message:            service.SanitizeHTML(in.Message),
		DiscussionType:     in.DiscussionType,
		PostedAt:           in.PostedAt,
		DelayedPostAt:      in.DelayedPostAt,
		LockAt:             in.LockAt,
		Pinned:             in.Pinned,
		Locked:             in.Locked,
		AllowRating:        in.AllowRating,
		OnlyGradersCanRate: in.OnlyGradersCanRate,
		SortByRating:       in.SortByRating,
		RequireInitialPost: in.RequireInitialPost,
		AssignmentID:       in.AssignmentID,
		WorkflowState:      models.DiscussionTopicActive,
	}
	if in.Published != nil && !*in.Published {
		topic.WorkflowState = models.DiscussionTopicUnpublished
	}
	topic.WorkflowState = service.DiscussionStateForSchedule(topic, discussionNow())

	usesParity := in.AssignmentID != nil || in.GroupCategoryID != nil || in.AnonymousState != nil ||
		in.IsSectionSpecific || in.Assignment != nil
	if usesParity && h.parity == nil {
		return responses.BadRequest(c, "These discussion options are not enabled")
	}
	if h.parity != nil {
		ctx := c.Context()
		if in.AssignmentID != nil {
			if err := h.parity.ValidateLinkedAssignment(ctx, topic.ID, *in.AssignmentID, topic.CourseID, callerAccountID(c)); err != nil {
				return responses.BadRequest(c, err.Error())
			}
		}
		if in.GroupCategoryID != nil && *in.GroupCategoryID != 0 {
			if err := h.parity.ValidateGroupCategory(ctx, topic.CourseID, *in.GroupCategoryID); err != nil {
				return discussionParityErr(c, err, "Could not create discussion topic")
			}
			gid := *in.GroupCategoryID
			topic.GroupCategoryID = &gid
		}
		topic.AnonymousState = service.NormalizeAnonymousState(in.AnonymousState)
		topic.IsSectionSpecific = in.IsSectionSpecific
		if err := service.ValidateTopicOptions(topic, in.Assignment != nil); err != nil {
			return discussionParityErr(c, err, "Could not create discussion topic")
		}
		if topic.IsSectionSpecific {
			if err := h.parity.ValidateSections(ctx, topic.CourseID, in.SectionIDs); err != nil {
				return discussionParityErr(c, err, "Could not create discussion topic")
			}
		}
		if err := h.parity.ValidateGradedInput(ctx, topic.CourseID, in.Assignment); err != nil {
			return discussionParityErr(c, err, "Could not create discussion topic")
		}
	}

	if err := h.discussionService.CreateTopic(c.Context(), topic); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	if h.parity != nil {
		ctx := c.Context()
		if in.Assignment != nil {
			if err := h.parity.ApplyGraded(ctx, topic, in.Assignment, callerAccountID(c)); err != nil {
				return discussionParityErr(c, err, "Could not create the graded assignment")
			}
			if err := h.discussionService.UpdateTopic(ctx, topic); err != nil {
				return responses.InternalError(c, "Could not save discussion topic")
			}
		}
		if topic.IsSectionSpecific {
			if err := h.parity.SetTopicSections(ctx, topic, in.SectionIDs); err != nil {
				return discussionParityErr(c, err, "Could not save discussion sections")
			}
		}
		if topic.GroupCategoryID != nil {
			if _, err := h.parity.SyncGroupTopics(ctx, topic); err != nil {
				return responses.InternalError(c, "Could not create the group topics")
			}
		}
		h.notifyNewTopicAsync(topic)
	}

	out := topicToJSON(topic)
	addDiscussionTopicExtras(c, h.discussionService, h.parity, topic, out)
	return c.Status(fiber.StatusCreated).JSON(out)
}

// notifyNewTopicAsync announces a newly available topic in the
// background (the request context ends with the response).
func (h *DiscussionHandler) notifyNewTopicAsync(topic *models.DiscussionTopic) {
	if h.parity == nil {
		return
	}
	cp := *topic
	go func() {
		defer func() { _ = recover() }()
		h.parity.NotifyNewTopic(context.Background(), &cp)
	}()
}

func (h *DiscussionHandler) UpdateTopic(c *fiber.Ctx) error {
	// Tenant + F-013 course parent tie: a teacher of course A must not be
	// able to rewrite a topic in course B by addressing it by id.
	topic, wrote := requireTopicInCourse(c, h.discussionService)
	if wrote {
		return nil
	}
	if topic.RootTopicID != nil {
		return responses.BadRequest(c, "Edit the group discussion itself, not a group's copy")
	}
	wasVisible := service.DiscussionTopicVisibleToStudents(topic, discussionNow())

	var input struct {
		DiscussionTopic struct {
			Title              *string      `json:"title"`
			Message            *string      `json:"message"`
			DiscussionType     *string      `json:"discussion_type"`
			PostedAt           *time.Time   `json:"posted_at"`
			DelayedPostAt      optionalJSON `json:"delayed_post_at"`
			LockAt             optionalJSON `json:"lock_at"`
			Pinned             *bool        `json:"pinned"`
			Locked             *bool        `json:"locked"`
			AllowRating        *bool        `json:"allow_rating"`
			OnlyGradersCanRate *bool        `json:"only_graders_can_rate"`
			SortByRating       *bool        `json:"sort_by_rating"`
			RequireInitialPost *bool        `json:"require_initial_post"`
			AssignmentID       *uint        `json:"assignment_id"`
			// Published (Canvas `published`) publishes or unpublishes the
			// topic through the DiscussionTopic state machine. Students
			// can't open unpublished topics, so this is how a teacher
			// releases a copied or imported (unpublished) topic.
			Published *bool `json:"published"`
			// Parity options; null clears group_category_id /
			// anonymous_state, and "assignment": null makes the topic
			// ungraded.
			GroupCategoryID   optionalJSON `json:"group_category_id"`
			AnonymousState    optionalJSON `json:"anonymous_state"`
			IsSectionSpecific *bool        `json:"is_section_specific"`
			SectionIDs        []uint       `json:"section_ids"`
			Assignment        optionalJSON `json:"assignment"`
		} `json:"discussion_topic"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	in := &input.DiscussionTopic

	usesParity := in.GroupCategoryID.Set || in.AnonymousState.Set || in.IsSectionSpecific != nil ||
		in.SectionIDs != nil || in.Assignment.Set || in.AssignmentID != nil
	if usesParity && h.parity == nil {
		return responses.BadRequest(c, "These discussion options are not enabled")
	}

	// reschedule re-derives active / post_delayed from delayed_post_at
	// when this request publishes the topic or moves its date.
	reschedule := in.DelayedPostAt.Set
	if p := in.Published; p != nil {
		// Only the unpublished <-> active edge is driven by this flag; a
		// locked or post_delayed topic already counts as published.
		target := topic.WorkflowState
		if *p && topic.WorkflowState == models.DiscussionTopicUnpublished {
			target = models.DiscussionTopicActive
		}
		if !*p {
			target = models.DiscussionTopicUnpublished
		}
		if target != topic.WorkflowState {
			role := callerRoleForStateMachine(c)
			if role == models.RoleTA {
				// TAs manage topics alongside teachers (this route is
				// behind the instructor middleware), as in Canvas.
				role = models.RoleTeacher
			}
			if err := models.TransitionDiscussionTopicWorkflowState(topic.WorkflowState, target, role); err != nil {
				return responses.BadRequest(c, err.Error())
			}
			// Canvas can_unpublish?: a topic people have already posted
			// in can't go back to draft.
			if target == models.DiscussionTopicUnpublished {
				existing, err := h.discussionService.ListEntries(c.Context(), topic.ID, callerAccountID(c), repository.PaginationParams{Page: 1, PerPage: 1})
				if err != nil {
					return responses.InternalError(c, "Could not update discussion topic")
				}
				hasPosts := existing.TotalCount > 0
				if !hasPosts && h.parity != nil && discussionIsGroupRoot(topic) {
					hasPosts, _ = h.parity.TopicHasPosts(c.Context(), topic)
				}
				if hasPosts {
					return responses.BadRequest(c, "A discussion with replies can't be unpublished")
				}
			}
			topic.WorkflowState = target
			reschedule = true
		}
	}

	if in.Title != nil {
		if strings.TrimSpace(*in.Title) == "" {
			return responses.BadRequest(c, "Discussion title is required")
		}
		topic.Title = *in.Title
	}
	if in.Message != nil {
		topic.Message = service.SanitizeHTML(*in.Message)
	}
	if in.DiscussionType != nil {
		topic.DiscussionType = *in.DiscussionType
	}
	if in.PostedAt != nil {
		topic.PostedAt = in.PostedAt
	}
	if in.DelayedPostAt.Set {
		v, err := in.DelayedPostAt.timeValue()
		if err != nil {
			return responses.BadRequest(c, "delayed_post_at must be an ISO 8601 time")
		}
		topic.DelayedPostAt = v
	}
	if in.LockAt.Set {
		v, err := in.LockAt.timeValue()
		if err != nil {
			return responses.BadRequest(c, "lock_at must be an ISO 8601 time")
		}
		topic.LockAt = v
	}
	if in.Pinned != nil {
		topic.Pinned = *in.Pinned
	}
	if in.Locked != nil {
		topic.Locked = *in.Locked
	}
	if in.AllowRating != nil {
		topic.AllowRating = *in.AllowRating
	}
	if in.OnlyGradersCanRate != nil {
		topic.OnlyGradersCanRate = *in.OnlyGradersCanRate
	}
	if in.SortByRating != nil {
		topic.SortByRating = *in.SortByRating
	}
	if in.RequireInitialPost != nil {
		topic.RequireInitialPost = *in.RequireInitialPost
	}

	var graded *service.DiscussionGradedInput
	removeGraded := false
	if h.parity != nil {
		ctx := c.Context()
		if in.AssignmentID != nil {
			if err := h.parity.ValidateLinkedAssignment(ctx, topic.ID, *in.AssignmentID, topic.CourseID, callerAccountID(c)); err != nil {
				return responses.BadRequest(c, err.Error())
			}
			topic.AssignmentID = in.AssignmentID
		}
		if in.GroupCategoryID.Set {
			v, err := in.GroupCategoryID.uintValue()
			if err != nil {
				return responses.BadRequest(c, "group_category_id must be a number or null")
			}
			changed := (v == nil) != (topic.GroupCategoryID == nil) ||
				(v != nil && topic.GroupCategoryID != nil && *v != *topic.GroupCategoryID)
			if changed {
				if err := h.parity.GroupCategoryChangeAllowed(ctx, topic); err != nil {
					return discussionParityErr(c, err, "Could not update discussion topic")
				}
				if v != nil {
					if err := h.parity.ValidateGroupCategory(ctx, topic.CourseID, *v); err != nil {
						return discussionParityErr(c, err, "Could not update discussion topic")
					}
				}
				topic.GroupCategoryID = v
			}
		}
		if in.AnonymousState.Set {
			v, err := in.AnonymousState.stringValue()
			if err != nil {
				return responses.BadRequest(c, "anonymous_state must be a string or null")
			}
			changed := (v == nil) != (topic.AnonymousState == nil) ||
				(v != nil && topic.AnonymousState != nil && *v != *topic.AnonymousState)
			if changed {
				// Canvas: anonymity is fixed once people have posted, so
				// a post written under an alias is never unmasked.
				if posted, err := h.parity.TopicHasPosts(ctx, topic); err != nil || posted {
					return responses.BadRequest(c, "Anonymity can't change after people have posted")
				}
				topic.AnonymousState = v
			}
		}
		if in.IsSectionSpecific != nil {
			topic.IsSectionSpecific = *in.IsSectionSpecific
		}
		if in.Assignment.Set {
			if in.Assignment.Null {
				removeGraded = true
			} else {
				g, err := in.Assignment.gradedValue()
				if err != nil {
					return responses.BadRequest(c, "assignment must be an object or null")
				}
				graded = g
			}
		}
		willBeGraded := graded != nil || (topic.AssignmentID != nil && !removeGraded)
		probe := *topic
		if removeGraded {
			probe.AssignmentID = nil
		}
		if err := service.ValidateTopicOptions(&probe, willBeGraded); err != nil {
			return discussionParityErr(c, err, "Could not update discussion topic")
		}
		if topic.IsSectionSpecific && (in.SectionIDs != nil || in.IsSectionSpecific != nil) {
			ids := in.SectionIDs
			if ids == nil {
				ids = h.parity.TopicSectionIDs(ctx, []uint{topic.ID})[topic.ID]
			}
			if err := h.parity.ValidateSections(ctx, topic.CourseID, ids); err != nil {
				return discussionParityErr(c, err, "Could not update discussion topic")
			}
			in.SectionIDs = ids
		}
		if err := h.parity.ValidateGradedInput(ctx, topic.CourseID, graded); err != nil {
			return discussionParityErr(c, err, "Could not update discussion topic")
		}
		if removeGraded {
			if err := h.parity.RemoveGraded(ctx, topic); err != nil {
				return discussionParityErr(c, err, "Could not update discussion topic")
			}
		}
	}

	if reschedule {
		topic.WorkflowState = service.DiscussionStateForSchedule(topic, discussionNow())
	}

	if h.parity != nil && graded != nil {
		if err := h.parity.ApplyGraded(c.Context(), topic, graded, callerAccountID(c)); err != nil {
			return discussionParityErr(c, err, "Could not update the graded assignment")
		}
	}

	if err := h.discussionService.UpdateTopic(c.Context(), topic); err != nil {
		return responses.InternalError(c, "Could not update discussion topic")
	}

	if h.parity != nil {
		ctx := c.Context()
		if in.IsSectionSpecific != nil || in.SectionIDs != nil {
			if err := h.parity.SetTopicSections(ctx, topic, in.SectionIDs); err != nil {
				return discussionParityErr(c, err, "Could not save discussion sections")
			}
		}
		if _, err := h.parity.SyncGroupTopics(ctx, topic); err != nil {
			return responses.InternalError(c, "Could not update the group topics")
		}
		if graded == nil {
			h.parity.SyncAssignmentWithTopic(ctx, topic)
		}
		if !wasVisible {
			h.notifyNewTopicAsync(topic)
		}
	}

	out := topicToJSON(topic)
	addDiscussionTopicExtras(c, h.discussionService, h.parity, topic, out)
	return c.JSON(out)
}

func (h *DiscussionHandler) DeleteTopic(c *fiber.Ctx) error {
	// The repo soft-delete is keyed on id only, so the topic MUST be
	// resolved inside the caller's tenant and this course first.
	topic, wrote := requireTopicInCourse(c, h.discussionService)
	if wrote {
		return nil
	}
	if topic.RootTopicID != nil {
		return responses.BadRequest(c, "Delete the group discussion itself, not a group's copy")
	}

	if err := h.discussionService.DeleteTopic(c.Context(), topic.ID); err != nil {
		return responses.InternalError(c, "Could not delete discussion topic")
	}
	if h.parity != nil {
		// The group topics and the graded assignment go with the topic.
		topic.WorkflowState = models.DiscussionTopicDeleted
		_, _ = h.parity.SyncGroupTopics(c.Context(), topic)
		h.parity.SyncAssignmentWithTopic(c.Context(), topic)
	}

	return c.JSON(fiber.Map{"delete": true})
}

func (h *DiscussionHandler) GetFullView(c *fiber.Ctx) error {
	topic, wrote := requireTopicInCourse(c, h.discussionService)
	if wrote {
		return nil
	}

	topicJSON := topicToJSON(topic)
	addDiscussionTopicExtras(c, h.discussionService, h.parity, topic, topicJSON)

	// Someone who must post first, and the root of a group discussion
	// (posts live on the group topics), get no entries.
	if discussionInitialPostRequired(c, h.discussionService, topic) || discussionIsGroupRoot(topic) {
		return c.JSON(fiber.Map{"topic": topicJSON, "entries": []fiber.Map{}})
	}

	view, err := h.discussionService.GetFullView(c.Context(), topic, callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not load discussion")
	}

	var refs []discussionEntryRef
	collectV1Refs(view.Entries, &refs)
	p := newDiscussionPresenter(c, h.parity, topic, refs)
	entries := make([]fiber.Map, len(view.Entries))
	for i := range view.Entries {
		entries[i] = entryViewV1ToJSON(&view.Entries[i], p)
	}

	return c.JSON(fiber.Map{
		"topic":   topicJSON,
		"entries": entries,
	})
}
