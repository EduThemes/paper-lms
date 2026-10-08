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
	observerService   *service.ObserverService
}

// SetObserverService lets linked observers read annotations on their
// student's submissions (same rule as SubmissionHandler.callerCanViewSubmission).
func (h *DocumentAnnotationHandler) SetObserverService(s *service.ObserverService) *DocumentAnnotationHandler {
	h.observerService = s
	return h
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

// annotationScope is the resolved authorization context of a submission's
// annotations: the submission, the course it belongs to, and the caller's
// relationship to it.
type annotationScope struct {
	submission *models.Submission
	courseID   uint
	callerID   uint
	isStaff    bool // teacher/TA of the course, or an admin of its tenant
	isOwner    bool // the caller is the submission's author
}

// canRead: the submission owner, course staff, or a linked observer of the
// owner (SubmissionHandler.callerCanViewSubmission rule).
func (h *DocumentAnnotationHandler) canRead(c *fiber.Ctx, sc *annotationScope) bool {
	if sc.isOwner || sc.isStaff {
		return true
	}
	if h.observerService != nil && sc.callerID != 0 {
		if ok, _ := h.observerService.IsObserverOf(c.Context(), sc.callerID, sc.submission.UserID); ok {
			return true
		}
	}
	return false
}

// scopeForSubmission builds the scope for a loaded submission. The course is
// always derived from the submission's assignment (tenant-scoped), never from
// client input, and the caller must be enrolled in that course (or an admin
// of its tenant). Returns a non-nil error to return directly on refusal.
func (h *DocumentAnnotationHandler) scopeForSubmission(c *fiber.Ctx, submission *models.Submission, assignment *models.Assignment) (*annotationScope, error) {
	callerID, _ := c.Locals("user_id").(uint)
	if callerID == 0 {
		return nil, fiber.NewError(fiber.StatusUnauthorized, "authentication required")
	}
	if err := h.authz.RequireCourseEnrolled(c, assignment.CourseID); err != nil {
		return nil, err
	}
	return &annotationScope{
		submission: submission,
		courseID:   assignment.CourseID,
		callerID:   callerID,
		isStaff:    callerIsCourseInstructor(c, h.authz, assignment.CourseID),
		isOwner:    submission.UserID == callerID,
	}, nil
}

// scopeFromURL resolves /courses/:course_id/assignments/:assignment_id/
// submissions/:user_id: the assignment must be in the caller's tenant AND in
// :course_id (F-013 parent-tie). Any miss is a 404.
func (h *DocumentAnnotationHandler) scopeFromURL(c *fiber.Ctx) (*annotationScope, error) {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		return nil, fiber.NewError(fiber.StatusBadRequest, "Invalid course ID")
	}
	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil || assignmentID <= 0 {
		return nil, fiber.NewError(fiber.StatusBadRequest, "Invalid assignment ID")
	}
	userID, err := c.ParamsInt("user_id")
	if err != nil || userID <= 0 {
		return nil, fiber.NewError(fiber.StatusBadRequest, "Invalid user ID")
	}
	acct := callerAccountID(c)
	assignment, err := h.assignmentRepo.FindByID(c.Context(), uint(assignmentID), acct)
	if err != nil || assignment == nil || assignment.CourseID != uint(courseID) {
		return nil, authzNotFound("submission")
	}
	submission, err := h.submissionService.GetByAssignmentAndUser(c.Context(), uint(assignmentID), uint(userID), acct)
	if err != nil || submission == nil {
		return nil, authzNotFound("submission")
	}
	return h.scopeForSubmission(c, submission, assignment)
}

// scopeForAnnotation resolves /annotations/:id: annotation -> submission ->
// assignment, all tenant-scoped. The course comes from that chain only; any
// client-supplied course_id is ignored.
func (h *DocumentAnnotationHandler) scopeForAnnotation(c *fiber.Ctx, annotationID uint) (*models.DocumentAnnotation, *annotationScope, error) {
	annotation, err := h.annotationService.GetAnnotation(c.Context(), annotationID)
	if err != nil {
		return nil, nil, authzNotFound("annotation")
	}
	acct := callerAccountID(c)
	submission, err := h.submissionRepo.FindByID(c.Context(), annotation.SubmissionID, acct)
	if err != nil || submission == nil {
		return nil, nil, authzNotFound("annotation")
	}
	assignment, err := h.assignmentRepo.FindByID(c.Context(), submission.AssignmentID, acct)
	if err != nil || assignment == nil {
		return nil, nil, authzNotFound("annotation")
	}
	sc, err := h.scopeForSubmission(c, submission, assignment)
	if err != nil {
		return nil, nil, err
	}
	if !h.canRead(c, sc) {
		return nil, nil, authzNotFound("annotation")
	}
	return annotation, sc, nil
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

// ListAnnotations returns annotations for a submission. The caller must be
// the submission owner, course staff, or a linked observer.
// GET /courses/:course_id/assignments/:assignment_id/submissions/:user_id/annotations
func (h *DocumentAnnotationHandler) ListAnnotations(c *fiber.Ctx) error {
	sc, err := h.scopeFromURL(c)
	if err != nil {
		return err
	}
	if !h.canRead(c, sc) {
		return authzNotFound("submission")
	}

	params := middleware.GetPagination(c)

	result, err := h.annotationService.ListAnnotations(c.Context(), sc.submission.ID, params)
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

// CreateAnnotation creates a new annotation on a submission. Course staff can
// annotate any submission of the course; students only their own.
// POST /courses/:course_id/assignments/:assignment_id/submissions/:user_id/annotations
func (h *DocumentAnnotationHandler) CreateAnnotation(c *fiber.Ctx) error {
	sc, err := h.scopeFromURL(c)
	if err != nil {
		return err
	}
	if !sc.isOwner && !sc.isStaff {
		if h.canRead(c, sc) {
			return authzForbidden("you do not have permission to annotate this submission")
		}
		return authzNotFound("submission")
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
		SubmissionID:   sc.submission.ID,
		UserID:         sc.callerID,
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

	if err := h.annotationService.CreateAnnotation(c.Context(), annotation, sc.isStaff); err != nil {
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

	annotation, _, err := h.scopeForAnnotation(c, uint(annotationID))
	if err != nil {
		return err
	}

	return c.JSON(annotationToJSON(annotation))
}

// UpdateAnnotation updates an existing annotation. Only its author can edit it.
// PUT /annotations/:id
func (h *DocumentAnnotationHandler) UpdateAnnotation(c *fiber.Ctx) error {
	annotationID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid annotation ID")
	}

	if _, _, err := h.scopeForAnnotation(c, uint(annotationID)); err != nil {
		return err
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

// DeleteAnnotation soft-deletes an annotation: its author, or an instructor
// of the course the annotated submission belongs to. Any course_id query
// parameter is ignored — the course is derived from the annotation.
// DELETE /annotations/:id
func (h *DocumentAnnotationHandler) DeleteAnnotation(c *fiber.Ctx) error {
	annotationID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid annotation ID")
	}

	annotation, sc, err := h.scopeForAnnotation(c, uint(annotationID))
	if err != nil {
		return err
	}
	if annotation.UserID != sc.callerID && !sc.isStaff {
		return authzForbidden("only the annotation owner or an instructor can delete this annotation")
	}

	if err := h.annotationService.DeleteAnnotation(c.Context(), uint(annotationID)); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{"status": "deleted"})
}

// ResolveAnnotation marks an annotation as resolved: course staff, or the
// author of the annotated submission (marking feedback as addressed).
// POST /annotations/:id/resolve
func (h *DocumentAnnotationHandler) ResolveAnnotation(c *fiber.Ctx) error {
	annotationID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid annotation ID")
	}

	_, sc, err := h.scopeForAnnotation(c, uint(annotationID))
	if err != nil {
		return err
	}
	if !sc.isStaff && !sc.isOwner {
		return authzForbidden("you do not have permission to resolve this annotation")
	}

	if err := h.annotationService.ResolveAnnotation(c.Context(), uint(annotationID), sc.callerID); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{"status": "resolved"})
}

// UnresolveAnnotation removes the resolved status (same rule as resolve).
// DELETE /annotations/:id/resolve
func (h *DocumentAnnotationHandler) UnresolveAnnotation(c *fiber.Ctx) error {
	annotationID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid annotation ID")
	}

	_, sc, err := h.scopeForAnnotation(c, uint(annotationID))
	if err != nil {
		return err
	}
	if !sc.isStaff && !sc.isOwner {
		return authzForbidden("you do not have permission to unresolve this annotation")
	}

	if err := h.annotationService.UnresolveAnnotation(c.Context(), uint(annotationID)); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{"status": "active"})
}

// ReplyToAnnotation creates a reply to an existing annotation: course staff,
// or the author of the annotated submission. Any course_id query parameter
// is ignored — the course is derived from the annotation.
// POST /annotations/:id/replies
func (h *DocumentAnnotationHandler) ReplyToAnnotation(c *fiber.Ctx) error {
	annotationID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid annotation ID")
	}

	_, sc, err := h.scopeForAnnotation(c, uint(annotationID))
	if err != nil {
		return err
	}
	if !sc.isStaff && !sc.isOwner {
		return authzForbidden("you do not have permission to reply to this annotation")
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
		UserID:  sc.callerID,
		Content: input.Annotation.Content,
	}

	if err := h.annotationService.ReplyToAnnotation(c.Context(), uint(annotationID), reply); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(annotationToJSON(reply))
}

// GetAnnotationSummary returns annotation counts by type and resolved status.
// GET /courses/:course_id/assignments/:assignment_id/submissions/:user_id/annotation_summary
func (h *DocumentAnnotationHandler) GetAnnotationSummary(c *fiber.Ctx) error {
	sc, err := h.scopeFromURL(c)
	if err != nil {
		return err
	}
	if !h.canRead(c, sc) {
		return authzNotFound("submission")
	}

	summary, err := h.annotationService.GetAnnotationSummary(c.Context(), sc.submission.ID)
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
