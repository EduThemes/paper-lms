package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

type DocumentAnnotationHandler struct {
	annotationService *service.DocumentAnnotationService
	submissionService *service.SubmissionService
	assignmentRepo    repository.AssignmentRepository
	submissionRepo    repository.SubmissionRepository
	authz             *ResourceAuthorizer
}

func NewDocumentAnnotationHandler(
	annotationService *service.DocumentAnnotationService,
	submissionService *service.SubmissionService,
	assignmentRepo repository.AssignmentRepository,
	submissionRepo repository.SubmissionRepository,
	authz *ResourceAuthorizer,
) *DocumentAnnotationHandler {
	return &DocumentAnnotationHandler{
		annotationService: annotationService,
		submissionService: submissionService,
		assignmentRepo:    assignmentRepo,
		submissionRepo:    submissionRepo,
		authz:             authz,
	}
}

// getCourseIDForAnnotation resolves the course ID from annotation -> submission -> assignment chain.
func (h *DocumentAnnotationHandler) getCourseIDForAnnotation(c *fiber.Ctx, annotationID uint) (uint, error) {
	annotation, err := h.annotationService.GetAnnotation(c.Context(), annotationID)
	if err != nil {
		return 0, err
	}
	submission, err := h.submissionRepo.FindByID(c.Context(), annotation.SubmissionID)
	if err != nil {
		return 0, err
	}
	assignment, err := h.assignmentRepo.FindByID(c.Context(), submission.AssignmentID)
	if err != nil {
		return 0, err
	}
	return assignment.CourseID, nil
}

func annotationToJSON(a *models.DocumentAnnotation) fiber.Map {
	result := fiber.Map{
		"id":                  a.ID,
		"submission_id":       a.SubmissionID,
		"user_id":             a.UserID,
		"annotation_type":     a.AnnotationType,
		"color":               a.Color,
		"content":             a.Content,
		"page_number":         a.PageNumber,
		"selection_start":     a.SelectionStart,
		"selection_end":       a.SelectionEnd,
		"x":                   a.X,
		"y":                   a.Y,
		"width":               a.Width,
		"height":              a.Height,
		"path_data":           a.PathData,
		"parent_annotation_id": a.ParentAnnotationID,
		"resolved_at":         a.ResolvedAt,
		"resolved_by_user_id": a.ResolvedByUserID,
		"workflow_state":      a.WorkflowState,
		"created_at":          a.CreatedAt,
		"updated_at":          a.UpdatedAt,
	}

	if a.User != nil {
		result["user"] = fiber.Map{
			"id":         a.User.ID,
			"name":       a.User.Name,
			"avatar_url": a.User.AvatarURL,
		}
	}

	if len(a.Replies) > 0 {
		repliesJSON := make([]fiber.Map, len(a.Replies))
		for i, reply := range a.Replies {
			repliesJSON[i] = annotationToJSON(&reply)
		}
		result["replies"] = repliesJSON
	}

	return result
}

// ListAnnotations returns annotations for a submission.
// GET /courses/:course_id/assignments/:assignment_id/submissions/:user_id/annotations
func (h *DocumentAnnotationHandler) ListAnnotations(c *fiber.Ctx) error {
	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	userID, err := c.ParamsInt("user_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	// Find the submission
	submission, err := h.submissionService.GetByAssignmentAndUser(c.Context(), uint(assignmentID), uint(userID))
	if err != nil {
		return responses.NotFound(c, "submission")
	}

	params := middleware.GetPagination(c)

	result, err := h.annotationService.ListAnnotations(c.Context(), submission.ID, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch annotations")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	annotations := make([]fiber.Map, len(result.Items))
	for i, a := range result.Items {
		annotations[i] = annotationToJSON(&a)
	}

	return c.JSON(annotations)
}

// CreateAnnotation creates a new annotation on a submission.
// POST /courses/:course_id/assignments/:assignment_id/submissions/:user_id/annotations
func (h *DocumentAnnotationHandler) CreateAnnotation(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	submissionUserID, err := c.ParamsInt("user_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	currentUserID, err := getUserID(c)
	if err != nil {
		return err
	}

	// Find the submission
	submission, err := h.submissionService.GetByAssignmentAndUser(c.Context(), uint(assignmentID), uint(submissionUserID))
	if err != nil {
		return responses.NotFound(c, "submission")
	}

	var input struct {
		Annotation struct {
			AnnotationType string  `json:"annotation_type"`
			Color          string  `json:"color"`
			Content        string  `json:"content"`
			PageNumber     int     `json:"page_number"`
			SelectionStart int     `json:"selection_start"`
			SelectionEnd   int     `json:"selection_end"`
			X              float64 `json:"x"`
			Y              float64 `json:"y"`
			Width          float64 `json:"width"`
			Height         float64 `json:"height"`
			PathData       string  `json:"path_data"`
		} `json:"annotation"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	annotation := &models.DocumentAnnotation{
		SubmissionID:   submission.ID,
		UserID:         currentUserID,
		AnnotationType: input.Annotation.AnnotationType,
		Color:          input.Annotation.Color,
		Content:        input.Annotation.Content,
		PageNumber:     input.Annotation.PageNumber,
		SelectionStart: input.Annotation.SelectionStart,
		SelectionEnd:   input.Annotation.SelectionEnd,
		X:              input.Annotation.X,
		Y:              input.Annotation.Y,
		Width:          input.Annotation.Width,
		Height:         input.Annotation.Height,
		PathData:       input.Annotation.PathData,
	}

	if annotation.PageNumber == 0 {
		annotation.PageNumber = 1
	}

	if annotation.Color == "" {
		annotation.Color = "#FFFF00"
	}

	if err := h.annotationService.CreateAnnotation(c.Context(), annotation, uint(courseID)); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(annotationToJSON(annotation))
}

// GetAnnotation returns a single annotation with replies.
// GET /annotations/:id
func (h *DocumentAnnotationHandler) GetAnnotation(c *fiber.Ctx) error {
	annotationID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid annotation ID")
	}

	courseID, err := h.getCourseIDForAnnotation(c, uint(annotationID))
	if err != nil {
		return responses.NotFound(c, "annotation")
	}
	if authErr := h.authz.RequireCourseEnrolled(c, courseID); authErr != nil {
		return authErr
	}

	annotation, err := h.annotationService.GetAnnotation(c.Context(), uint(annotationID))
	if err != nil {
		return responses.NotFound(c, "annotation")
	}

	return c.JSON(annotationToJSON(annotation))
}

// UpdateAnnotation updates an existing annotation.
// PUT /annotations/:id
func (h *DocumentAnnotationHandler) UpdateAnnotation(c *fiber.Ctx) error {
	annotationID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid annotation ID")
	}

	currentUserID, err := getUserID(c)
	if err != nil {
		return err
	}

	var input struct {
		Annotation map[string]interface{} `json:"annotation"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	annotation, err := h.annotationService.UpdateAnnotation(c.Context(), uint(annotationID), currentUserID, input.Annotation)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(annotationToJSON(annotation))
}

// DeleteAnnotation soft-deletes an annotation.
// DELETE /annotations/:id
func (h *DocumentAnnotationHandler) DeleteAnnotation(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		// For routes without course_id in path, try query param
		courseID = c.QueryInt("course_id", 0)
		if courseID == 0 {
			return responses.BadRequest(c, "course_id is required")
		}
	}

	annotationID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid annotation ID")
	}

	currentUserID, err := getUserID(c)
	if err != nil {
		return err
	}

	if err := h.annotationService.DeleteAnnotation(c.Context(), uint(annotationID), currentUserID, uint(courseID)); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{"status": "deleted"})
}

// ResolveAnnotation marks an annotation as resolved.
// POST /annotations/:id/resolve
func (h *DocumentAnnotationHandler) ResolveAnnotation(c *fiber.Ctx) error {
	annotationID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid annotation ID")
	}

	courseID, err := h.getCourseIDForAnnotation(c, uint(annotationID))
	if err != nil {
		return responses.NotFound(c, "annotation")
	}
	if authErr := h.authz.RequireCourseEnrolled(c, courseID); authErr != nil {
		return authErr
	}

	currentUserID, err := getUserID(c)
	if err != nil {
		return err
	}

	if err := h.annotationService.ResolveAnnotation(c.Context(), uint(annotationID), currentUserID); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{"status": "resolved"})
}

// UnresolveAnnotation removes the resolved status.
// DELETE /annotations/:id/resolve
func (h *DocumentAnnotationHandler) UnresolveAnnotation(c *fiber.Ctx) error {
	annotationID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid annotation ID")
	}

	courseID, err := h.getCourseIDForAnnotation(c, uint(annotationID))
	if err != nil {
		return responses.NotFound(c, "annotation")
	}
	if authErr := h.authz.RequireCourseEnrolled(c, courseID); authErr != nil {
		return authErr
	}

	if err := h.annotationService.UnresolveAnnotation(c.Context(), uint(annotationID)); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{"status": "active"})
}

// ReplyToAnnotation creates a reply to an existing annotation.
// POST /annotations/:id/replies
func (h *DocumentAnnotationHandler) ReplyToAnnotation(c *fiber.Ctx) error {
	courseID := c.QueryInt("course_id", 0)
	if courseID == 0 {
		// Try to get from params if route includes it
		var err error
		courseID, err = c.ParamsInt("course_id")
		if err != nil || courseID == 0 {
			return responses.BadRequest(c, "course_id is required")
		}
	}

	annotationID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid annotation ID")
	}

	currentUserID, err := getUserID(c)
	if err != nil {
		return err
	}

	var input struct {
		Annotation struct {
			Content string `json:"content"`
		} `json:"annotation"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Annotation.Content == "" {
		return responses.BadRequest(c, "Reply content is required")
	}

	reply := &models.DocumentAnnotation{
		UserID:  currentUserID,
		Content: input.Annotation.Content,
	}

	if err := h.annotationService.ReplyToAnnotation(c.Context(), uint(annotationID), reply, uint(courseID)); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(annotationToJSON(reply))
}

// GetAnnotationSummary returns annotation counts by type and resolved status.
// GET /courses/:course_id/assignments/:assignment_id/submissions/:user_id/annotation_summary
func (h *DocumentAnnotationHandler) GetAnnotationSummary(c *fiber.Ctx) error {
	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	userID, err := c.ParamsInt("user_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	// Find the submission
	submission, err := h.submissionService.GetByAssignmentAndUser(c.Context(), uint(assignmentID), uint(userID))
	if err != nil {
		return responses.NotFound(c, "submission")
	}

	summary, err := h.annotationService.GetAnnotationSummary(c.Context(), submission.ID)
	if err != nil {
		return responses.InternalError(c, "Could not fetch annotation summary")
	}

	return c.JSON(fiber.Map{
		"total_count":    summary.TotalCount,
		"by_type":        summary.ByType,
		"resolved_count": summary.ResolvedCount,
		"active_count":   summary.ActiveCount,
	})
}
