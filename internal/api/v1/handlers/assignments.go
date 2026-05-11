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

type AssignmentHandler struct {
	assignmentService *service.AssignmentService
}

func NewAssignmentHandler(assignmentService *service.AssignmentService) *AssignmentHandler {
	return &AssignmentHandler{assignmentService: assignmentService}
}

func assignmentToJSON(a *models.Assignment) fiber.Map {
	return fiber.Map{
		"id":                   a.ID,
		"course_id":            a.CourseID,
		"assignment_group_id":  a.AssignmentGroupID,
		"name":                 a.Name,
		"description":          a.Description,
		"due_at":               a.DueAt,
		"unlock_at":            a.UnlockAt,
		"lock_at":              a.LockAt,
		"points_possible":      a.PointsPossible,
		"grading_type":         a.GradingType,
		"submission_types":     strings.Split(a.SubmissionTypes, ","),
		"position":             a.Position,
		"workflow_state":       a.WorkflowState,
		"published":            a.Published,
		"anonymous_grading":    a.AnonymousGrading,
		"post_policy":          a.PostPolicy,
		"peer_reviews_enabled": a.PeerReviewsEnabled,
		"peer_review_count":    a.PeerReviewCount,
		"group_category_id":    a.GroupCategoryID,
		"is_group_assignment":  a.GroupCategoryID != nil && *a.GroupCategoryID > 0,
		"created_at":           a.CreatedAt,
		"updated_at":           a.UpdatedAt,
	}
}

func (h *AssignmentHandler) ListAssignments(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.assignmentService.ListByCourse(c.Context(), uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch assignments")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	assignments := make([]fiber.Map, len(result.Items))
	for i, a := range result.Items {
		assignments[i] = assignmentToJSON(&a)
	}

	return c.JSON(assignments)
}

func (h *AssignmentHandler) GetAssignment(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	assignment, err := h.assignmentService.GetByID(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "assignment")
	}

	return c.JSON(assignmentToJSON(assignment))
}

func (h *AssignmentHandler) CreateAssignment(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		Assignment struct {
			Name               string     `json:"name"`
			Description        string     `json:"description"`
			DueAt              *time.Time `json:"due_at"`
			UnlockAt           *time.Time `json:"unlock_at"`
			LockAt             *time.Time `json:"lock_at"`
			PointsPossible     *float64   `json:"points_possible"`
			GradingType        string     `json:"grading_type"`
			SubmissionTypes    []string   `json:"submission_types"`
			Position           int        `json:"position"`
			Published          bool       `json:"published"`
			AssignmentGroupID  *uint      `json:"assignment_group_id"`
			AnonymousGrading   bool       `json:"anonymous_grading"`
			PostPolicy         string     `json:"post_policy"`
			PeerReviewsEnabled bool       `json:"peer_reviews_enabled"`
			PeerReviewCount    int        `json:"peer_review_count"`
			GroupCategoryID    *uint      `json:"group_category_id"`
		} `json:"assignment"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if strings.TrimSpace(input.Assignment.Name) == "" {
		return responses.BadRequest(c, "Assignment name is required")
	}

	state := "unpublished"
	if input.Assignment.Published {
		state = "published"
	}

	submissionTypes := strings.Join(input.Assignment.SubmissionTypes, ",")

	postPolicy := input.Assignment.PostPolicy
	if postPolicy == "" {
		postPolicy = "automatic"
	}

	assignment := &models.Assignment{
		CourseID:           uint(courseID),
		AssignmentGroupID:  input.Assignment.AssignmentGroupID,
		Name:               input.Assignment.Name,
		Description:        input.Assignment.Description,
		DueAt:              input.Assignment.DueAt,
		UnlockAt:           input.Assignment.UnlockAt,
		LockAt:             input.Assignment.LockAt,
		PointsPossible:     input.Assignment.PointsPossible,
		GradingType:        input.Assignment.GradingType,
		SubmissionTypes:    submissionTypes,
		Position:           input.Assignment.Position,
		Published:          input.Assignment.Published,
		WorkflowState:      state,
		AnonymousGrading:   input.Assignment.AnonymousGrading,
		PostPolicy:         postPolicy,
		PeerReviewsEnabled: input.Assignment.PeerReviewsEnabled,
		PeerReviewCount:    input.Assignment.PeerReviewCount,
		GroupCategoryID:    input.Assignment.GroupCategoryID,
	}

	if assignment.GradingType == "" {
		assignment.GradingType = "points"
	}
	if assignment.SubmissionTypes == "" {
		assignment.SubmissionTypes = "online_text_entry"
	}

	if err := h.assignmentService.Create(c.Context(), assignment); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(assignmentToJSON(assignment))
}

func (h *AssignmentHandler) UpdateAssignment(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	assignment, err := h.assignmentService.GetByID(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "assignment")
	}

	var input struct {
		Assignment struct {
			Name               *string    `json:"name"`
			Description        *string    `json:"description"`
			DueAt              *time.Time `json:"due_at"`
			UnlockAt           *time.Time `json:"unlock_at"`
			LockAt             *time.Time `json:"lock_at"`
			PointsPossible     *float64   `json:"points_possible"`
			GradingType        *string    `json:"grading_type"`
			SubmissionTypes    []string   `json:"submission_types"`
			Position           *int       `json:"position"`
			Published          *bool      `json:"published"`
			AssignmentGroupID  *uint      `json:"assignment_group_id"`
			AnonymousGrading   *bool      `json:"anonymous_grading"`
			PostPolicy         *string    `json:"post_policy"`
			PeerReviewsEnabled *bool      `json:"peer_reviews_enabled"`
			PeerReviewCount    *int       `json:"peer_review_count"`
			GroupCategoryID    *uint      `json:"group_category_id"`
		} `json:"assignment"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Assignment.Name != nil {
		assignment.Name = *input.Assignment.Name
	}
	if input.Assignment.Description != nil {
		assignment.Description = *input.Assignment.Description
	}
	if input.Assignment.DueAt != nil {
		assignment.DueAt = input.Assignment.DueAt
	}
	if input.Assignment.UnlockAt != nil {
		assignment.UnlockAt = input.Assignment.UnlockAt
	}
	if input.Assignment.LockAt != nil {
		assignment.LockAt = input.Assignment.LockAt
	}
	if input.Assignment.PointsPossible != nil {
		assignment.PointsPossible = input.Assignment.PointsPossible
	}
	if input.Assignment.GradingType != nil {
		assignment.GradingType = *input.Assignment.GradingType
	}
	if len(input.Assignment.SubmissionTypes) > 0 {
		assignment.SubmissionTypes = strings.Join(input.Assignment.SubmissionTypes, ",")
	}
	if input.Assignment.Position != nil {
		assignment.Position = *input.Assignment.Position
	}
	if input.Assignment.Published != nil {
		assignment.Published = *input.Assignment.Published
		if *input.Assignment.Published {
			assignment.WorkflowState = "published"
		} else {
			assignment.WorkflowState = "unpublished"
		}
	}
	if input.Assignment.AssignmentGroupID != nil {
		assignment.AssignmentGroupID = input.Assignment.AssignmentGroupID
	}
	if input.Assignment.AnonymousGrading != nil {
		assignment.AnonymousGrading = *input.Assignment.AnonymousGrading
	}
	if input.Assignment.PostPolicy != nil {
		assignment.PostPolicy = *input.Assignment.PostPolicy
	}
	if input.Assignment.PeerReviewsEnabled != nil {
		assignment.PeerReviewsEnabled = *input.Assignment.PeerReviewsEnabled
	}
	if input.Assignment.PeerReviewCount != nil {
		assignment.PeerReviewCount = *input.Assignment.PeerReviewCount
	}
	if input.Assignment.GroupCategoryID != nil {
		assignment.GroupCategoryID = input.Assignment.GroupCategoryID
	}

	if err := h.assignmentService.Update(c.Context(), assignment); err != nil {
		return responses.InternalError(c, "Could not update assignment")
	}

	return c.JSON(assignmentToJSON(assignment))
}

func (h *AssignmentHandler) DeleteAssignment(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	if err := h.assignmentService.Delete(c.Context(), uint(id)); err != nil {
		return responses.InternalError(c, "Could not delete assignment")
	}

	return c.JSON(fiber.Map{"delete": true})
}
