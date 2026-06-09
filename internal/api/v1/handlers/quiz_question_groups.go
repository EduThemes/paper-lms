package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type QuizQuestionGroupHandler struct {
	quizService *service.QuizService
}

func NewQuizQuestionGroupHandler(quizService *service.QuizService) *QuizQuestionGroupHandler {
	return &QuizQuestionGroupHandler{quizService: quizService}
}

func quizQuestionGroupToJSON(g *models.QuizQuestionGroup) fiber.Map {
	return fiber.Map{
		"id":               g.ID,
		"quiz_id":          g.QuizID,
		"name":             g.Name,
		"pick_count":       g.PickCount,
		"points_per_item":  g.PointsPerItem,
		"question_bank_id": g.QuestionBankID,
		"position":         g.Position,
		"created_at":       g.CreatedAt,
		"updated_at":       g.UpdatedAt,
	}
}

// scopedQuestionGroup loads the :group_id group and verifies it belongs
// to :quiz_id, which must itself live in :course_id and the caller's
// tenant. Returns wrote=true (a 404 has been written) on any miss so the
// caller short-circuits with `return nil`. Closes the nested-route
// cross-tenant / cross-quiz IDOR — the route middleware only guards
// :course_id.
func (h *QuizQuestionGroupHandler) scopedQuestionGroup(c *fiber.Ctx) (*models.QuizQuestionGroup, bool) {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid course ID")
		return nil, true
	}
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid quiz ID")
		return nil, true
	}
	groupID, err := c.ParamsInt("group_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid group ID")
		return nil, true
	}
	if requireQuizInCourse(c, h.quizService, uint(quizID), uint(courseID)) {
		return nil, true
	}
	group, err := h.quizService.GetQuestionGroup(c.Context(), uint(groupID))
	if err != nil || group.QuizID != uint(quizID) {
		_ = responses.NotFound(c, "quiz question group")
		return nil, true
	}
	return group, false
}

// ListGroups handles GET /courses/:course_id/quizzes/:quiz_id/groups
func (h *QuizQuestionGroupHandler) ListGroups(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}
	if requireQuizInCourse(c, h.quizService, uint(quizID), uint(courseID)) {
		return nil
	}

	groups, err := h.quizService.ListQuestionGroups(c.Context(), uint(quizID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch question groups")
	}

	result := make([]fiber.Map, len(groups))
	for i, g := range groups {
		result[i] = quizQuestionGroupToJSON(&g)
	}

	return c.JSON(result)
}

// CreateGroup handles POST /courses/:course_id/quizzes/:quiz_id/groups
func (h *QuizQuestionGroupHandler) CreateGroup(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}
	if requireQuizInCourse(c, h.quizService, uint(quizID), uint(courseID)) {
		return nil
	}

	var input struct {
		Name           string   `json:"name"`
		PickCount      int      `json:"pick_count"`
		PointsPerItem  *float64 `json:"points_per_item"`
		QuestionBankID *uint    `json:"question_bank_id"`
		Position       int      `json:"position"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	group := &models.QuizQuestionGroup{
		QuizID:         uint(quizID),
		Name:           input.Name,
		PickCount:      input.PickCount,
		PointsPerItem:  input.PointsPerItem,
		QuestionBankID: input.QuestionBankID,
		Position:       input.Position,
	}

	if err := h.quizService.CreateQuestionGroup(c.Context(), group); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(quizQuestionGroupToJSON(group))
}

// GetGroup handles GET /courses/:course_id/quizzes/:quiz_id/groups/:group_id
func (h *QuizQuestionGroupHandler) GetGroup(c *fiber.Ctx) error {
	group, wrote := h.scopedQuestionGroup(c)
	if wrote {
		return nil
	}

	return c.JSON(quizQuestionGroupToJSON(group))
}

// UpdateGroup handles PUT /courses/:course_id/quizzes/:quiz_id/groups/:group_id
func (h *QuizQuestionGroupHandler) UpdateGroup(c *fiber.Ctx) error {
	group, wrote := h.scopedQuestionGroup(c)
	if wrote {
		return nil
	}

	var input struct {
		Name           *string  `json:"name"`
		PickCount      *int     `json:"pick_count"`
		PointsPerItem  *float64 `json:"points_per_item"`
		QuestionBankID *uint    `json:"question_bank_id"`
		Position       *int     `json:"position"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Name != nil {
		group.Name = *input.Name
	}
	if input.PickCount != nil {
		group.PickCount = *input.PickCount
	}
	if input.PointsPerItem != nil {
		group.PointsPerItem = input.PointsPerItem
	}
	if input.QuestionBankID != nil {
		group.QuestionBankID = input.QuestionBankID
	}
	if input.Position != nil {
		group.Position = *input.Position
	}

	if err := h.quizService.UpdateQuestionGroup(c.Context(), group); err != nil {
		return responses.InternalError(c, "Could not update question group")
	}

	return c.JSON(quizQuestionGroupToJSON(group))
}

// DeleteGroup handles DELETE /courses/:course_id/quizzes/:quiz_id/groups/:group_id
func (h *QuizQuestionGroupHandler) DeleteGroup(c *fiber.Ctx) error {
	group, wrote := h.scopedQuestionGroup(c)
	if wrote {
		return nil
	}

	if err := h.quizService.DeleteQuestionGroup(c.Context(), group.ID); err != nil {
		return responses.InternalError(c, "Could not delete question group")
	}

	return c.JSON(fiber.Map{"deleted": true})
}
