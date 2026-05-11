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

// ListCheckpoints — GET /api/v1/courses/:course_id/discussion_topics/:topic_id/checkpoints
func (h *DiscussionCheckpointHandler) ListCheckpoints(c *fiber.Ctx) error {
	topicID, err := c.ParamsInt("topic_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid topic ID")
	}
	checkpoints, err := h.checkpointService.ListCheckpoints(c.Context(), uint(topicID))
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
	topicID, err := c.ParamsInt("topic_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid topic ID")
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

	created, err := h.checkpointService.CreateCheckpoints(c.Context(), uint(topicID), checkpoints)
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
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid checkpoint ID")
	}

	var in checkpointInput
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "Invalid request body")
	}

	cp := &models.DiscussionCheckpoint{
		ID:              uint(id),
		CheckpointType:  in.CheckpointType,
		DueAt:           in.DueAt,
		PointsPossible:  in.PointsPossible,
		RequiredReplies: in.RequiredReplies,
		WorkflowState:   "active",
	}
	if topicID, terr := c.ParamsInt("topic_id"); terr == nil {
		cp.DiscussionTopicID = uint(topicID)
	}

	if err := h.checkpointService.UpdateCheckpoint(c.Context(), cp); err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.JSON(checkpointToJSON(cp))
}

// DeleteCheckpoint — DELETE /api/v1/courses/:course_id/discussion_topics/:topic_id/checkpoints/:id
func (h *DiscussionCheckpointHandler) DeleteCheckpoint(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid checkpoint ID")
	}
	if err := h.checkpointService.DeleteCheckpoint(c.Context(), uint(id)); err != nil {
		return responses.InternalError(c, "Could not delete checkpoint")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// GetUserProgress — GET /api/v1/courses/:course_id/discussion_topics/:topic_id/checkpoints/progress?user_id=N
// If user_id is omitted, falls back to the authenticated user.
func (h *DiscussionCheckpointHandler) GetUserProgress(c *fiber.Ctx) error {
	topicID, err := c.ParamsInt("topic_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid topic ID")
	}

	userID, err := strconv.ParseUint(c.Query("user_id", ""), 10, 64)
	if err != nil || userID == 0 {
		// Fall back to the authenticated user if available.
		if v, ok := c.Locals("user_id").(uint); ok && v != 0 {
			userID = uint64(v)
		} else {
			return responses.BadRequest(c, "user_id is required")
		}
	}

	progress, err := h.checkpointService.EvaluateUserProgress(c.Context(), uint(topicID), uint(userID))
	if err != nil {
		return responses.InternalError(c, "Could not evaluate progress")
	}

	out := make([]fiber.Map, len(progress))
	for i, p := range progress {
		out[i] = progressToJSON(p)
	}
	return c.JSON(out)
}
