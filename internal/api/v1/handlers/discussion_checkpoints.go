package handlers

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type DiscussionCheckpointHandler struct {
	checkpointService *service.DiscussionCheckpointService
}

func NewDiscussionCheckpointHandler(checkpointService *service.DiscussionCheckpointService) *DiscussionCheckpointHandler {
	return &DiscussionCheckpointHandler{checkpointService: checkpointService}
}

func checkpointToJSON(cp *models.DiscussionCheckpoint) fiber.Map {
	return fiber.Map{
		"id":                  cp.ID,
		"discussion_topic_id": cp.DiscussionTopicID,
		"checkpoint_type":     cp.CheckpointType,
		"due_at":              cp.DueAt,
		"points_possible":     cp.PointsPossible,
		"required_replies":    cp.RequiredReplies,
		"workflow_state":      cp.WorkflowState,
		"created_at":          cp.CreatedAt,
		"updated_at":          cp.UpdatedAt,
	}
}

func progressToJSON(p service.UserCheckpointProgress) fiber.Map {
	return fiber.Map{
		"checkpoint":   checkpointToJSON(&p.Checkpoint),
		"reply_count":  p.ReplyCount,
		"required":     p.Required,
		"status":       p.Status,
		"completed_at": p.CompletedAt,
	}
}

// checkpointInput is the JSON shape for create/update bodies.
type checkpointInput struct {
	CheckpointType  string     `json:"checkpoint_type"`
	DueAt           *time.Time `json:"due_at"`
	PointsPossible  float64    `json:"points_possible"`
	RequiredReplies int        `json:"required_replies"`
}

// requireCheckpointInTopic resolves :id as a live checkpoint on the
// already-resolved topic. Writes a 404 and returns wrote=true otherwise.
func (h *DiscussionCheckpointHandler) requireCheckpointInTopic(c *fiber.Ctx, topicID uint) (*models.DiscussionCheckpoint, bool) {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		_ = responses.BadRequest(c, "Invalid checkpoint ID")
		return nil, true
	}
	cp, err := h.checkpointService.GetCheckpoint(c.Context(), uint(id), callerAccountID(c))
	if err != nil || cp == nil || cp.DiscussionTopicID != topicID || cp.WorkflowState == "deleted" {
		_ = responses.NotFound(c, "checkpoint")
		return nil, true
	}
	return cp, false
}

// ListCheckpoints — GET /api/v1/courses/:course_id/discussion_topics/:topic_id/checkpoints
func (h *DiscussionCheckpointHandler) ListCheckpoints(c *fiber.Ctx) error {
	topic, wrote := requireTopicInCourse(c, h.checkpointService)
	if wrote {
		return nil
	}
	checkpoints, err := h.checkpointService.ListCheckpoints(c.Context(), topic.ID)
	if err != nil {
		return responses.InternalError(c, "Could not fetch checkpoints")
	}
	out := make([]fiber.Map, len(checkpoints))
	for i := range checkpoints {
		out[i] = checkpointToJSON(&checkpoints[i])
	}
	return c.JSON(out)
}

// CreateCheckpoints — POST /api/v1/courses/:course_id/discussion_topics/:topic_id/checkpoints
// Body: { "checkpoints": [ { ... }, { ... } ] }  (replaces the existing set)
func (h *DiscussionCheckpointHandler) CreateCheckpoints(c *fiber.Ctx) error {
	topic, wrote := requireTopicInCourse(c, h.checkpointService)
	if wrote {
		return nil
	}

	var body struct {
		Checkpoints []checkpointInput `json:"checkpoints"`
	}
	if err := c.BodyParser(&body); err != nil {
		return responses.BadRequest(c, "Invalid request body")
	}
	if len(body.Checkpoints) == 0 {
		return responses.BadRequest(c, "At least one checkpoint is required")
	}

	checkpoints := make([]*models.DiscussionCheckpoint, 0, len(body.Checkpoints))
	for _, in := range body.Checkpoints {
		checkpoints = append(checkpoints, &models.DiscussionCheckpoint{
			CheckpointType:  in.CheckpointType,
			DueAt:           in.DueAt,
			PointsPossible:  in.PointsPossible,
			RequiredReplies: in.RequiredReplies,
		})
	}

	created, err := h.checkpointService.CreateCheckpoints(c.Context(), topic.ID, checkpoints)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	out := make([]fiber.Map, len(created))
	for i := range created {
		out[i] = checkpointToJSON(&created[i])
	}
	return c.Status(fiber.StatusCreated).JSON(out)
}

// UpdateCheckpoint — PUT /api/v1/courses/:course_id/discussion_topics/:topic_id/checkpoints/:id
func (h *DiscussionCheckpointHandler) UpdateCheckpoint(c *fiber.Ctx) error {
	topic, wrote := requireTopicInCourse(c, h.checkpointService)
	if wrote {
		return nil
	}
	cp, wrote := h.requireCheckpointInTopic(c, topic.ID)
	if wrote {
		return nil
	}

	var in checkpointInput
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "Invalid request body")
	}

	cp.CheckpointType = in.CheckpointType
	cp.DueAt = in.DueAt
	cp.PointsPossible = in.PointsPossible
	cp.RequiredReplies = in.RequiredReplies

	if err := h.checkpointService.UpdateCheckpoint(c.Context(), cp); err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.JSON(checkpointToJSON(cp))
}

// DeleteCheckpoint — DELETE /api/v1/courses/:course_id/discussion_topics/:topic_id/checkpoints/:id
func (h *DiscussionCheckpointHandler) DeleteCheckpoint(c *fiber.Ctx) error {
	topic, wrote := requireTopicInCourse(c, h.checkpointService)
	if wrote {
		return nil
	}
	cp, wrote := h.requireCheckpointInTopic(c, topic.ID)
	if wrote {
		return nil
	}
	if err := h.checkpointService.DeleteCheckpoint(c.Context(), cp.ID); err != nil {
		return responses.InternalError(c, "Could not delete checkpoint")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// GetUserProgress — GET /api/v1/courses/:course_id/discussion_topics/:topic_id/checkpoints/progress?user_id=N
// If user_id is omitted, falls back to the authenticated user. Only
// course staff may ask about another user; a student asking for a
// classmate's progress gets 403.
func (h *DiscussionCheckpointHandler) GetUserProgress(c *fiber.Ctx) error {
	topic, wrote := requireTopicInCourse(c, h.checkpointService)
	if wrote {
		return nil
	}

	callerID, _ := c.Locals("user_id").(uint)

	var userID uint
	raw := c.Query("user_id", "")
	if raw == "" || raw == "self" {
		userID = callerID
	} else {
		parsed, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return responses.BadRequest(c, "Invalid user_id")
		}
		userID = uint(parsed)
	}
	if userID == 0 {
		return responses.BadRequest(c, "user_id is required")
	}
	if userID != callerID && !discussionCallerIsStaff(c) {
		return responses.Forbidden(c, "You can only view your own checkpoint progress")
	}

	progress, err := h.checkpointService.EvaluateUserProgress(c.Context(), topic.ID, userID)
	if err != nil {
		return responses.InternalError(c, "Could not evaluate progress")
	}

	out := make([]fiber.Map, len(progress))
	for i, p := range progress {
		out[i] = progressToJSON(p)
	}
	return c.JSON(out)
}
