package handlers

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

type DiscussionHandler struct {
	discussionService *service.DiscussionService
}

func NewDiscussionHandler(discussionService *service.DiscussionService) *DiscussionHandler {
	return &DiscussionHandler{discussionService: discussionService}
}

func topicToJSON(t *models.DiscussionTopic) fiber.Map {
	return fiber.Map{
		"id":                   t.ID,
		"course_id":            t.CourseID,
		"user_id":              t.UserID,
		"title":                t.Title,
		"message":              t.Message,
		"discussion_type":      t.DiscussionType,
		"posted_at":            t.PostedAt,
		"delayed_post_at":      t.DelayedPostAt,
		"lock_at":              t.LockAt,
		"pinned":               t.Pinned,
		"locked":               t.Locked,
		"allow_rating":         t.AllowRating,
		"only_graders_can_rate": t.OnlyGradersCanRate,
		"sort_by_rating":       t.SortByRating,
		"require_initial_post": t.RequireInitialPost,
		"assignment_id":        t.AssignmentID,
		"workflow_state":       t.WorkflowState,
		"published":            t.WorkflowState != models.DiscussionTopicUnpublished,
		"created_at":           t.CreatedAt,
		"updated_at":           t.UpdatedAt,
	}
}

func (h *DiscussionHandler) ListTopics(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	// Students only see topics they could open (published, not delayed);
	// staff see every topic in the course.
	result, err := h.discussionService.ListTopicsForViewer(c.Context(), uint(courseID), callerAccountID(c), !discussionCallerIsStaff(c), params)
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

func (h *DiscussionHandler) GetTopic(c *fiber.Ctx) error {
	topic, wrote := requireTopicInCourse(c, h.discussionService)
	if wrote {
		return nil
	}

	return c.JSON(topicToJSON(topic))
}

func (h *DiscussionHandler) CreateTopic(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		DiscussionTopic struct {
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
		} `json:"discussion_topic"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if strings.TrimSpace(input.DiscussionTopic.Title) == "" {
		return responses.BadRequest(c, "Discussion title is required")
	}

	userID, _ := c.Locals("user_id").(uint)

	topic := &models.DiscussionTopic{
		CourseID:           uint(courseID),
		UserID:             userID,
		Title:              input.DiscussionTopic.Title,
		Message:            service.SanitizeHTML(input.DiscussionTopic.Message),
		DiscussionType:     input.DiscussionTopic.DiscussionType,
		PostedAt:           input.DiscussionTopic.PostedAt,
		DelayedPostAt:      input.DiscussionTopic.DelayedPostAt,
		LockAt:             input.DiscussionTopic.LockAt,
		Pinned:             input.DiscussionTopic.Pinned,
		Locked:             input.DiscussionTopic.Locked,
		AllowRating:        input.DiscussionTopic.AllowRating,
		OnlyGradersCanRate: input.DiscussionTopic.OnlyGradersCanRate,
		SortByRating:       input.DiscussionTopic.SortByRating,
		RequireInitialPost: input.DiscussionTopic.RequireInitialPost,
		AssignmentID:       input.DiscussionTopic.AssignmentID,
	}

	if err := h.discussionService.CreateTopic(c.Context(), topic); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(topicToJSON(topic))
}

func (h *DiscussionHandler) UpdateTopic(c *fiber.Ctx) error {
	// Tenant + F-013 course parent tie: a teacher of course A must not be
	// able to rewrite a topic in course B by addressing it by id.
	topic, wrote := requireTopicInCourse(c, h.discussionService)
	if wrote {
		return nil
	}

	var input struct {
		DiscussionTopic struct {
			Title              *string    `json:"title"`
			Message            *string    `json:"message"`
			DiscussionType     *string    `json:"discussion_type"`
			PostedAt           *time.Time `json:"posted_at"`
			DelayedPostAt      *time.Time `json:"delayed_post_at"`
			LockAt             *time.Time `json:"lock_at"`
			Pinned             *bool      `json:"pinned"`
			Locked             *bool      `json:"locked"`
			AllowRating        *bool      `json:"allow_rating"`
			OnlyGradersCanRate *bool      `json:"only_graders_can_rate"`
			SortByRating       *bool      `json:"sort_by_rating"`
			RequireInitialPost *bool      `json:"require_initial_post"`
			AssignmentID       *uint      `json:"assignment_id"`
			// Published (Canvas `published`) publishes or unpublishes the
			// topic through the DiscussionTopic state machine. Students
			// can't open unpublished topics, so this is how a teacher
			// releases a copied or imported (unpublished) topic.
			Published *bool `json:"published"`
		} `json:"discussion_topic"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if p := input.DiscussionTopic.Published; p != nil {
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
				if existing.TotalCount > 0 {
					return responses.BadRequest(c, "A discussion with replies can't be unpublished")
				}
			}
			topic.WorkflowState = target
		}
	}

	if input.DiscussionTopic.Title != nil {
		topic.Title = *input.DiscussionTopic.Title
	}
	if input.DiscussionTopic.Message != nil {
		topic.Message = service.SanitizeHTML(*input.DiscussionTopic.Message)
	}
	if input.DiscussionTopic.DiscussionType != nil {
		topic.DiscussionType = *input.DiscussionTopic.DiscussionType
	}
	if input.DiscussionTopic.PostedAt != nil {
		topic.PostedAt = input.DiscussionTopic.PostedAt
	}
	if input.DiscussionTopic.DelayedPostAt != nil {
		topic.DelayedPostAt = input.DiscussionTopic.DelayedPostAt
	}
	if input.DiscussionTopic.LockAt != nil {
		topic.LockAt = input.DiscussionTopic.LockAt
	}
	if input.DiscussionTopic.Pinned != nil {
		topic.Pinned = *input.DiscussionTopic.Pinned
	}
	if input.DiscussionTopic.Locked != nil {
		topic.Locked = *input.DiscussionTopic.Locked
	}
	if input.DiscussionTopic.AllowRating != nil {
		topic.AllowRating = *input.DiscussionTopic.AllowRating
	}
	if input.DiscussionTopic.OnlyGradersCanRate != nil {
		topic.OnlyGradersCanRate = *input.DiscussionTopic.OnlyGradersCanRate
	}
	if input.DiscussionTopic.SortByRating != nil {
		topic.SortByRating = *input.DiscussionTopic.SortByRating
	}
	if input.DiscussionTopic.RequireInitialPost != nil {
		topic.RequireInitialPost = *input.DiscussionTopic.RequireInitialPost
	}
	if input.DiscussionTopic.AssignmentID != nil {
		topic.AssignmentID = input.DiscussionTopic.AssignmentID
	}

	if err := h.discussionService.UpdateTopic(c.Context(), topic); err != nil {
		return responses.InternalError(c, "Could not update discussion topic")
	}

	return c.JSON(topicToJSON(topic))
}

func (h *DiscussionHandler) DeleteTopic(c *fiber.Ctx) error {
	// The repo soft-delete is keyed on id only, so the topic MUST be
	// resolved inside the caller's tenant and this course first.
	topic, wrote := requireTopicInCourse(c, h.discussionService)
	if wrote {
		return nil
	}

	if err := h.discussionService.DeleteTopic(c.Context(), topic.ID); err != nil {
		return responses.InternalError(c, "Could not delete discussion topic")
	}

	return c.JSON(fiber.Map{"delete": true})
}

func (h *DiscussionHandler) GetFullView(c *fiber.Ctx) error {
	topic, wrote := requireTopicInCourse(c, h.discussionService)
	if wrote {
		return nil
	}

	view, err := h.discussionService.GetFullView(c.Context(), topic, callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not load discussion")
	}

	return c.JSON(fiber.Map{
		"topic":   topicToJSON(view.Topic),
		"entries": view.Entries,
	})
}
