package handlers

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

type SubmissionHandler struct {
	submissionService           *service.SubmissionService
	commentRepo                 repository.SubmissionCommentRepository
	attachmentRepo              repository.AttachmentRepository
	userRepo                    repository.UserRepository
	assignmentRepo              repository.AssignmentRepository
	notificationDeliveryService *service.NotificationDeliveryService
	observerService             *service.ObserverService
	outcomeAlignmentRepo        repository.OutcomeAlignmentRepository
	outcomeService              *service.LearningOutcomeService
}

func NewSubmissionHandler(submissionService *service.SubmissionService, commentRepo repository.SubmissionCommentRepository, attachmentRepo repository.AttachmentRepository, userRepo repository.UserRepository, assignmentRepo repository.AssignmentRepository, notificationDeliveryService *service.NotificationDeliveryService, observerService *service.ObserverService, outcomeAlignmentRepo repository.OutcomeAlignmentRepository, outcomeService *service.LearningOutcomeService) *SubmissionHandler {
	return &SubmissionHandler{
		submissionService:           submissionService,
		commentRepo:                 commentRepo,
		attachmentRepo:              attachmentRepo,
		userRepo:                    userRepo,
		assignmentRepo:              assignmentRepo,
		notificationDeliveryService: notificationDeliveryService,
		observerService:             observerService,
		outcomeAlignmentRepo:        outcomeAlignmentRepo,
		outcomeService:              outcomeService,
	}
}

func submissionToJSON(s *models.Submission) fiber.Map {
	result := fiber.Map{
		"id":              s.ID,
		"assignment_id":   s.AssignmentID,
		"user_id":         s.UserID,
		"submission_type": s.SubmissionType,
		"body":            s.Body,
		"url":             s.URL,
		"score":           s.Score,
		"grade":           s.Grade,
		"graded_at":       s.GradedAt,
		"grader_id":       s.GraderID,
		"submitted_at":    s.SubmittedAt,
		"attempt":         s.Attempt,
		"late":            s.Late,
		"missing":         s.Missing,
		"excused":         s.Excused,
		"workflow_state":  s.WorkflowState,
		"preview_url":     nil,
	}
	if s.Attachments != nil && *s.Attachments != "" {
		var attachments []map[string]interface{}
		if err := json.Unmarshal([]byte(*s.Attachments), &attachments); err == nil {
			result["attachments"] = attachments
		}
	}
	return result
}

func submissionCommentToJSON(sc *models.SubmissionComment) fiber.Map {
	return fiber.Map{
		"id":            sc.ID,
		"submission_id": sc.SubmissionID,
		"author_id":     sc.AuthorID,
		"comment":       sc.Comment,
		"draft":         sc.Draft,
		"created_at":    sc.CreatedAt,
		"updated_at":    sc.UpdatedAt,
	}
}

func (h *SubmissionHandler) ListCourseSubmissions(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)
	if params.PerPage > 10000 {
		params.PerPage = 10000
	}

	result, err := h.submissionService.BulkListByCourse(c.Context(), uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch submissions")
	}

	// Filter to requesting user's submissions if user_id query param is present
	// This allows students (enrolled but not instructor) to fetch their own submissions
	// Observers can pass their observee's user_id to see that student's submissions
	requestingUserID, _ := c.Locals("user_id").(uint)
	filterUserID := uint(0)
	if uid := c.Query("user_id"); uid != "" {
		if uid == "self" {
			filterUserID = requestingUserID
		} else {
			if parsed, err := strconv.Atoi(uid); err == nil && parsed > 0 {
				targetUserID := uint(parsed)
				// If requesting someone else's submissions, verify observer link
				if targetUserID != requestingUserID && h.observerService != nil {
					isObserver, _ := h.observerService.IsObserverOf(c.Context(), requestingUserID, targetUserID)
					if !isObserver {
						return responses.BadRequest(c, "Not authorized to view this student's submissions")
					}
				}
				filterUserID = targetUserID
			}
		}
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	submissions := make([]fiber.Map, 0, len(result.Items))
	for _, s := range result.Items {
		if filterUserID > 0 && s.UserID != filterUserID {
			continue
		}
		submissions = append(submissions, submissionToJSON(&s))
	}

	return c.JSON(submissions)
}

func (h *SubmissionHandler) ListSubmissions(c *fiber.Ctx) error {
	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.submissionService.ListByAssignment(c.Context(), uint(assignmentID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch submissions")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	submissions := make([]fiber.Map, len(result.Items))
	for i, s := range result.Items {
		submissions[i] = submissionToJSON(&s)
	}

	return c.JSON(submissions)
}

func (h *SubmissionHandler) GetSubmission(c *fiber.Ctx) error {
	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	userID, err := c.ParamsInt("user_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	submission, err := h.submissionService.GetByAssignmentAndUser(c.Context(), uint(assignmentID), uint(userID))
	if err != nil {
		return responses.NotFound(c, "submission")
	}

	return c.JSON(submissionToJSON(submission))
}

func (h *SubmissionHandler) CreateSubmission(c *fiber.Ctx) error {
	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	var input struct {
		Submission struct {
			SubmissionType string `json:"submission_type"`
			Body           string `json:"body"`
			URL            string `json:"url"`
			FileIDs        []uint `json:"file_ids"`
		} `json:"submission"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	submission := &models.Submission{
		AssignmentID:   uint(assignmentID),
		UserID:         userID,
		SubmissionType: &input.Submission.SubmissionType,
		Body:           &input.Submission.Body,
	}

	if input.Submission.URL != "" {
		submission.URL = &input.Submission.URL
	}

	// Resolve file attachments
	if len(input.Submission.FileIDs) > 0 {
		var attachments []map[string]interface{}
		for _, fid := range input.Submission.FileIDs {
			att, err := h.attachmentRepo.FindByID(c.Context(), fid)
			if err != nil {
				return responses.BadRequest(c, fmt.Sprintf("File ID %d not found", fid))
			}
			attachments = append(attachments, map[string]interface{}{
				"id":           att.ID,
				"display_name": att.DisplayName,
				"filename":     att.Filename,
				"content_type": att.ContentType,
				"size":         att.Size,
				"url":          fmt.Sprintf("/api/v1/files/%d/download", att.ID),
			})
		}
		if len(attachments) > 0 {
			attJSON, _ := json.Marshal(attachments)
			attStr := string(attJSON)
			submission.Attachments = &attStr
		}
	}

	if err := h.submissionService.Create(c.Context(), submission); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(submissionToJSON(submission))
}

func (h *SubmissionHandler) UpdateSubmission(c *fiber.Ctx) error {
	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	userID, err := c.ParamsInt("user_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	graderID, err := getUserID(c)
	if err != nil {
		return err
	}

	var input struct {
		Submission struct {
			PostedGrade string `json:"posted_grade"`
			Excused     *bool  `json:"excused"`
		} `json:"submission"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Submission.PostedGrade != "" {
		submission, err := h.submissionService.Grade(c.Context(), uint(assignmentID), uint(userID), graderID, input.Submission.PostedGrade)
		if err != nil {
			return responses.BadRequest(c, err.Error())
		}

		// Queue grade-posted notification for the student
		if h.notificationDeliveryService != nil {
			assignmentName := fmt.Sprintf("Assignment #%d", assignmentID)
			if assignment, aErr := h.assignmentRepo.FindByID(c.Context(), uint(assignmentID), callerAccountID(c)); aErr == nil {
				assignmentName = assignment.Name
			}
			gradeDisplay := input.Submission.PostedGrade
			if submission.Grade != nil {
				gradeDisplay = *submission.Grade
			}
			subject := fmt.Sprintf("Grade posted: %s", assignmentName)
			body := fmt.Sprintf("Your submission for <strong>%s</strong> has been graded. You received a score of <strong>%s</strong>.", assignmentName, gradeDisplay)
			_ = h.notificationDeliveryService.QueueNotification(c.Context(), uint(userID), "submission_grade", subject, body, "Assignment", uint(assignmentID))
		}

		// Auto-create outcome results for aligned outcomes
		if h.outcomeAlignmentRepo != nil && h.outcomeService != nil && submission.Score != nil {
			if alignments, aErr := h.outcomeAlignmentRepo.ListByAssignmentID(c.Context(), uint(assignmentID)); aErr == nil {
				for _, alignment := range alignments {
					possible := 100.0
					if assignment, aErr := h.assignmentRepo.FindByID(c.Context(), uint(assignmentID), callerAccountID(c)); aErr == nil && assignment.PointsPossible != nil {
						possible = *assignment.PointsPossible
					}
					result := &models.LearningOutcomeResult{
						UserID:              uint(userID),
						LearningOutcomeID:   alignment.LearningOutcomeID,
						ContextType:         "Course",
						ContextID:           alignment.CourseID,
						AssociatedAssetType: "Assignment",
						AssociatedAssetID:   uint(assignmentID),
						Score:               submission.Score,
						Possible:            &possible,
						Attempt:             submission.Attempt,
					}
					_ = h.outcomeService.CreateResult(c.Context(), result)
				}
			}
		}

		return c.JSON(submissionToJSON(submission))
	}

	// Handle excused update
	if input.Submission.Excused != nil {
		submission, err := h.submissionService.GetByAssignmentAndUser(c.Context(), uint(assignmentID), uint(userID))
		if err != nil {
			return responses.NotFound(c, "submission")
		}
		submission.Excused = *input.Submission.Excused
		return c.JSON(submissionToJSON(submission))
	}

	return responses.BadRequest(c, "No valid update fields provided")
}

func (h *SubmissionHandler) CreateSubmissionComment(c *fiber.Ctx) error {
	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	userID, err := c.ParamsInt("user_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	authorID, err := getUserID(c)
	if err != nil {
		return err
	}

	var input struct {
		Comment struct {
			TextComment string `json:"text_comment"`
		} `json:"comment"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Comment.TextComment == "" {
		return responses.BadRequest(c, "Comment text is required")
	}

	// Find the submission to get its ID
	submission, err := h.submissionService.GetByAssignmentAndUser(c.Context(), uint(assignmentID), uint(userID))
	if err != nil {
		return responses.NotFound(c, "submission")
	}

	comment := &models.SubmissionComment{
		SubmissionID: submission.ID,
		AuthorID:     authorID,
		Comment:      input.Comment.TextComment,
	}

	if err := h.commentRepo.Create(c.Context(), comment); err != nil {
		return responses.InternalError(c, "Could not create comment")
	}

	j := submissionCommentToJSON(comment)
	if author, err := h.userRepo.FindByID(c.Context(), authorID); err == nil {
		j["author_name"] = author.Name
	}
	return c.Status(fiber.StatusCreated).JSON(j)
}

func (h *SubmissionHandler) ListSubmissionComments(c *fiber.Ctx) error {
	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	userID, err := c.ParamsInt("user_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	// Find the submission to get its ID
	submission, err := h.submissionService.GetByAssignmentAndUser(c.Context(), uint(assignmentID), uint(userID))
	if err != nil {
		return responses.NotFound(c, "submission")
	}

	comments, err := h.commentRepo.ListBySubmissionID(c.Context(), submission.ID, callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not fetch comments")
	}

	// Build author name cache to avoid N+1
	authorNames := make(map[uint]string)
	for _, sc := range comments {
		if _, ok := authorNames[sc.AuthorID]; !ok {
			if author, err := h.userRepo.FindByID(c.Context(), sc.AuthorID); err == nil {
				authorNames[sc.AuthorID] = author.Name
			}
		}
	}

	result := make([]fiber.Map, len(comments))
	for i, sc := range comments {
		j := submissionCommentToJSON(&sc)
		if name, ok := authorNames[sc.AuthorID]; ok {
			j["author_name"] = name
		}
		result[i] = j
	}

	return c.JSON(result)
}

func (h *SubmissionHandler) BulkGrade(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	graderID, err := getUserID(c)
	if err != nil {
		return err
	}

	var input struct {
		GradeData []struct {
			AssignmentID uint   `json:"assignment_id"`
			UserID       uint   `json:"user_id"`
			PostedGrade  string `json:"posted_grade"`
		} `json:"grade_data"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if len(input.GradeData) == 0 {
		return responses.BadRequest(c, "grade_data must not be empty")
	}
	if len(input.GradeData) > 10000 {
		return responses.BadRequest(c, "grade_data exceeds maximum of 10000 entries")
	}

	type gradeResult struct {
		AssignmentID uint   `json:"assignment_id"`
		UserID       uint   `json:"user_id"`
		Score        *float64 `json:"score,omitempty"`
		Grade        *string  `json:"grade,omitempty"`
		Error        string `json:"error,omitempty"`
	}

	results := make([]gradeResult, 0, len(input.GradeData))
	_ = courseID // validated above

	for _, entry := range input.GradeData {
		sub, err := h.submissionService.Grade(c.Context(), entry.AssignmentID, entry.UserID, graderID, entry.PostedGrade)
		if err != nil {
			results = append(results, gradeResult{
				AssignmentID: entry.AssignmentID,
				UserID:       entry.UserID,
				Error:        err.Error(),
			})
		} else {
			results = append(results, gradeResult{
				AssignmentID: entry.AssignmentID,
				UserID:       entry.UserID,
				Score:        sub.Score,
				Grade:        sub.Grade,
			})
		}
	}

	return c.JSON(fiber.Map{"results": results})
}

// PostGrades sets posted_at on all graded submissions for an assignment,
// making grades visible to students.
func (h *SubmissionHandler) PostGrades(c *fiber.Ctx) error {
	assignmentID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	now := time.Now()
	if err := h.submissionService.PostGradesByAssignment(c.Context(), uint(assignmentID), &now); err != nil {
		return responses.InternalError(c, "Could not post grades")
	}

	return c.JSON(fiber.Map{"posted": true, "posted_at": now})
}

// HideGrades clears posted_at on all submissions for an assignment,
// hiding grades from students.
func (h *SubmissionHandler) HideGrades(c *fiber.Ctx) error {
	assignmentID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	if err := h.submissionService.PostGradesByAssignment(c.Context(), uint(assignmentID), nil); err != nil {
		return responses.InternalError(c, "Could not hide grades")
	}

	return c.JSON(fiber.Map{"hidden": true})
}
