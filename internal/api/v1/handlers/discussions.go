package handlers

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
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

	result, err := h.discussionService.ListTopics(c.Context(), uint(courseID), params)
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
	id, err := c.ParamsInt("topic_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid topic ID")
	}

	topic, err := h.discussionService.GetTopic(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "discussion topic")
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
		Message:            input.DiscussionTopic.Message,
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
	id, err := c.ParamsInt("topic_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid topic ID")
	}

	topic, err := h.discussionService.GetTopic(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "discussion topic")
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
		} `json:"discussion_topic"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.DiscussionTopic.Title != nil {
		topic.Title = *input.DiscussionTopic.Title
	}
	if input.DiscussionTopic.Message != nil {
		topic.Message = *input.DiscussionTopic.Message
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
	id, err := c.ParamsInt("topic_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid topic ID")
	}

	if err := h.discussionService.DeleteTopic(c.Context(), uint(id)); err != nil {
		return responses.InternalError(c, "Could not delete discussion topic")
	}

	return c.JSON(fiber.Map{"delete": true})
}

func (h *DiscussionHandler) GetFullView(c *fiber.Ctx) error {
	id, err := c.ParamsInt("topic_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid topic ID")
	}

	view, err := h.discussionService.GetFullView(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "discussion topic")
	}

	return c.JSON(fiber.Map{
		"topic":   topicToJSON(view.Topic),
		"entries": view.Entries,
	})
}
