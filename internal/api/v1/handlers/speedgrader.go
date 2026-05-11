package handlers

import (
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

// SpeedGraderHandler handles SpeedGrader-related API endpoints.
type SpeedGraderHandler struct {
	speedGraderService *service.SpeedGraderService
}

// NewSpeedGraderHandler creates a new SpeedGraderHandler.
func NewSpeedGraderHandler(speedGraderService *service.SpeedGraderService) *SpeedGraderHandler {
	return &SpeedGraderHandler{speedGraderService: speedGraderService}
}

func speedGraderSubmissionJSON(sub *models.Submission) fiber.Map {
	if sub == nil {
		return nil
	}
	result := fiber.Map{
		"id":              sub.ID,
		"assignment_id":   sub.AssignmentID,
		"user_id":         sub.UserID,
		"submission_type": sub.SubmissionType,
		"body":            sub.Body,
		"url":             sub.URL,
		"score":           sub.Score,
		"grade":           sub.Grade,
		"graded_at":       sub.GradedAt,
		"grader_id":       sub.GraderID,
		"submitted_at":    sub.SubmittedAt,
		"attempt":         sub.Attempt,
		"late":            sub.Late,
		"missing":         sub.Missing,
		"excused":         sub.Excused,
		"workflow_state":  sub.WorkflowState,
	}
	if sub.Attachments != nil && *sub.Attachments != "" {
		var attachments []map[string]interface{}
		if err := json.Unmarshal([]byte(*sub.Attachments), &attachments); err == nil {
			result["attachments"] = attachments
		}
	}
	return result
}

// GetSpeedGraderData returns the full SpeedGrader data set for an assignment,
// including the assignment, all enrolled students, their submissions, and comments.
// GET /courses/:course_id/assignments/:assignment_id/speedgrader
func (h *SpeedGraderHandler) GetSpeedGraderData(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	data, err := h.speedGraderService.GetSpeedGraderData(c.Context(), uint(courseID), uint(assignmentID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch SpeedGrader data")
	}

	// Use the full user name map (includes teachers, TAs, and students)
	userNameLookup := data.UserNameMap

	// Build the response with assignment details and students
	studentsJSON := make([]fiber.Map, len(data.Students))
	for i, student := range data.Students {
		commentsJSON := make([]fiber.Map, len(student.Comments))
		for j, comment := range student.Comments {
			authorName := userNameLookup[comment.AuthorID]
			if authorName == "" {
				authorName = fmt.Sprintf("User %d", comment.AuthorID)
			}
			commentsJSON[j] = fiber.Map{
				"id":            comment.ID,
				"submission_id": comment.SubmissionID,
				"author_id":     comment.AuthorID,
				"author_name":   authorName,
				"comment":       comment.Comment,
				"draft":         comment.Draft,
				"created_at":    comment.CreatedAt,
				"updated_at":    comment.UpdatedAt,
			}
		}

		studentsJSON[i] = fiber.Map{
			"user_id":    student.UserID,
			"user_name":  student.UserName,
			"submission": speedGraderSubmissionJSON(student.Submission),
			"comments":   commentsJSON,
		}
	}

	return c.JSON(fiber.Map{
		"assignment": fiber.Map{
			"id":               data.Assignment.ID,
			"course_id":        data.Assignment.CourseID,
			"name":             data.Assignment.Name,
			"description":      data.Assignment.Description,
			"due_at":           data.Assignment.DueAt,
			"points_possible":  data.Assignment.PointsPossible,
			"grading_type":     data.Assignment.GradingType,
			"submission_types": data.Assignment.SubmissionTypes,
			"published":        data.Assignment.Published,
		},
		"students": studentsJSON,
	})
}

// GetStudentSubmission returns a single student's submission with comments
// for the given assignment.
// GET /courses/:course_id/assignments/:assignment_id/speedgrader/submissions/:user_id
func (h *SpeedGraderHandler) GetStudentSubmission(c *fiber.Ctx) error {
	_, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	userID, err := c.ParamsInt("user_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	data, err := h.speedGraderService.GetStudentSubmission(c.Context(), uint(assignmentID), uint(userID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch student submission")
	}

	commentsJSON := make([]fiber.Map, len(data.Comments))
	for i, comment := range data.Comments {
		commentsJSON[i] = fiber.Map{
			"id":            comment.ID,
			"submission_id": comment.SubmissionID,
			"author_id":     comment.AuthorID,
			"comment":       comment.Comment,
			"draft":         comment.Draft,
			"created_at":    comment.CreatedAt,
			"updated_at":    comment.UpdatedAt,
		}
	}

	return c.JSON(fiber.Map{
		"submission": speedGraderSubmissionJSON(data.Submission),
		"comments":   commentsJSON,
	})
}
