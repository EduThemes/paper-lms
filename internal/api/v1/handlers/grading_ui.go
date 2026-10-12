package handlers

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/gofiber/fiber/v2"
)

// GradingUIHandler serves the grading-workflow endpoints added for Canvas
// parity: anonymous-submission addressing, moderated grading, custom grade
// statuses, comment editing and comment files, the student-annotation
// document, and peer-review management. It reuses SubmissionHandler's
// grading and comment flows so both addressing schemes behave the same.
type GradingUIHandler struct {
	svc               *service.GradingUIService
	subs              *SubmissionHandler
	userRepo          repository.UserRepository
	peerReviewService *service.PeerReviewService
	fileService       *service.FileService
	rubricService     *service.RubricService // optional (SetRubricService)
}

// NewGradingUIHandler wires the handler.
func NewGradingUIHandler(svc *service.GradingUIService, subs *SubmissionHandler, userRepo repository.UserRepository, peerReviewService *service.PeerReviewService, fileService *service.FileService) *GradingUIHandler {
	return &GradingUIHandler{svc: svc, subs: subs, userRepo: userRepo, peerReviewService: peerReviewService, fileService: fileService}
}

// courseAndAssignment parses :course_id + :assignment_id and loads the
// assignment under the caller's tenant tied to the course (404 on any
// miss). (result, wrote).
func (h *GradingUIHandler) courseAndAssignment(c *fiber.Ctx, param string) (uint, *models.Assignment, bool) {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		_ = responses.BadRequest(c, "Invalid course ID")
		return 0, nil, true
	}
	assignmentID, err := c.ParamsInt(param)
	if err != nil || assignmentID <= 0 {
		_ = responses.BadRequest(c, "Invalid assignment ID")
		return 0, nil, true
	}
	assignment, wrote := h.subs.requireAssignmentInCourse(c, uint(assignmentID), uint(courseID))
	if wrote {
		return 0, nil, true
	}
	return uint(courseID), assignment, false
}

// anonymousSubmission resolves :anonymous_id to its submission. (result, wrote).
func (h *GradingUIHandler) anonymousSubmission(c *fiber.Ctx) (uint, *models.Assignment, *models.Submission, bool) {
	courseID, assignment, wrote := h.courseAndAssignment(c, "assignment_id")
	if wrote {
		return 0, nil, nil, true
	}
	sub, err := h.svc.ResolveAnonymousID(c.Context(), assignment.ID, c.Params("anonymous_id"), callerAccountID(c))
	if err != nil {
		_ = responses.NotFound(c, "submission")
		return 0, nil, nil, true
	}
	return courseID, assignment, sub, false
}

// anonymousSubmissionJSON is the staff view of a submission with the
// student's identity removed.
func anonymousSubmissionJSON(courseID uint, sub *models.Submission) fiber.Map {
	anon := ""
	if sub.AnonymousID != nil {
		anon = *sub.AnonymousID
	}
	m := speedGraderSubmissionJSON(sub)
	return anonymizeSubmissionMap(m, courseID, sub.AssignmentID, anon)
}

// ---------------------------------------------------------------------
// Anonymous submissions (SpeedGrader under anonymous grading)
// ---------------------------------------------------------------------

// UpdateAnonymousSubmission grades / excuses / sets the status of the
// submission behind an anonymous id (Canvas
// PUT .../anonymous_submissions/:anonymous_id).
func (h *GradingUIHandler) UpdateAnonymousSubmission(c *fiber.Ctx) error {
	courseID, assignment, sub, wrote := h.anonymousSubmission(c)
	if wrote {
		return nil
	}
	updated, wrote, err := h.subs.updateSubmissionFor(c, courseID, assignment.ID, sub.UserID)
	if wrote || err != nil {
		return err
	}
	return c.JSON(anonymousSubmissionJSON(courseID, updated))
}

// ListAnonymousSubmissionComments lists comments by anonymous id.
func (h *GradingUIHandler) ListAnonymousSubmissionComments(c *fiber.Ctx) error {
	_, assignment, sub, wrote := h.anonymousSubmission(c)
	if wrote {
		return nil
	}
	out, wrote, err := h.subs.listCommentsFor(c, assignment, sub, true)
	if wrote || err != nil {
		return err
	}
	return c.JSON(out)
}

// CreateAnonymousSubmissionComment comments by anonymous id.
func (h *GradingUIHandler) CreateAnonymousSubmissionComment(c *fiber.Ctx) error {
	_, assignment, sub, wrote := h.anonymousSubmission(c)
	if wrote {
		return nil
	}
	out, wrote, err := h.subs.createCommentFor(c, assignment, sub, true)
	if wrote || err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(out)
}

// DownloadAnonymousSubmissionFile streams a submitted file by anonymous id.
func (h *GradingUIHandler) DownloadAnonymousSubmissionFile(c *fiber.Ctx) error {
	if h.fileService == nil {
		return responses.NotFound(c, "file")
	}
	_, assignment, sub, wrote := h.anonymousSubmission(c)
	if wrote {
		return nil
	}
	fileID, err := c.ParamsInt("file_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid file ID")
	}
	if !submissionHasAttachment(sub, uint(fileID)) && !h.fileInSubmissionHistory(c, sub, uint(fileID)) {
		return responses.NotFound(c, "file")
	}
	_ = assignment
	return h.subs.streamSubmissionAttachment(c, uint(fileID), sub.UserID)
}

// fileInSubmissionHistory reports whether fileID was attached to an
// earlier attempt of sub (SpeedGrader shows prior attempts).
func (h *GradingUIHandler) fileInSubmissionHistory(c *fiber.Ctx, sub *models.Submission, fileID uint) bool {
	versions, err := h.svc.VersionsForSubmission(c.Context(), sub.ID)
	if err != nil {
		return false
	}
	for i := range versions {
		v := versions[i]
		if submissionHasAttachment(&models.Submission{Attachments: v.Attachments}, fileID) {
			return true
		}
	}
	return false
}

// DownloadPriorAttemptFile streams a file from an earlier attempt of a
// student's submission (user-addressed). Staff, the owner or a linked
// observer only.
func (h *GradingUIHandler) DownloadPriorAttemptFile(c *fiber.Ctx) error {
	if h.fileService == nil {
		return responses.NotFound(c, "file")
	}
	_, assignment, wrote := h.courseAndAssignment(c, "assignment_id")
	if wrote {
		return nil
	}
	userID, err := c.ParamsInt("user_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}
	fileID, err := c.ParamsInt("file_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid file ID")
	}
	sub, err := h.subs.submissionService.GetByAssignmentAndUser(c.Context(), assignment.ID, uint(userID), callerAccountID(c))
	if err != nil || sub == nil || !h.subs.callerCanViewSubmission(c, sub.UserID) || !h.fileInSubmissionHistory(c, sub, uint(fileID)) {
		return responses.NotFound(c, "file")
	}
	return h.subs.streamSubmissionAttachment(c, uint(fileID), sub.UserID)
}

// ---------------------------------------------------------------------
// Comment editing, deletion and files
// ---------------------------------------------------------------------

// loadComment resolves :comment_id on the :user_id submission and checks
// the caller may see it. (result, wrote).
func (h *GradingUIHandler) loadComment(c *fiber.Ctx, sub *models.Submission, assignment *models.Assignment) (*models.SubmissionComment, bool) {
	commentID, err := c.ParamsInt("comment_id")
	if err != nil || commentID <= 0 {
		_ = responses.BadRequest(c, "Invalid comment ID")
		return nil, true
	}
	cm, err := h.svc.FindComment(c.Context(), uint(commentID), callerAccountID(c))
	if err != nil || cm == nil || cm.SubmissionID != sub.ID {
		_ = responses.NotFound(c, "comment")
		return nil, true
	}
	callerID, _ := c.Locals("user_id").(uint)
	if !h.subs.commentVisibleTo(c, assignment, sub, cm, callerID, callerIsCourseStaff(c)) {
		_ = responses.NotFound(c, "comment")
		return nil, true
	}
	return cm, false
}

// commentTarget resolves the submission for comment routes addressed by
// user id (owner, staff or observer may read). (result, wrote).
func (h *GradingUIHandler) commentTarget(c *fiber.Ctx) (uint, *models.Assignment, *models.Submission, bool) {
	assignment, sub, wrote := h.subs.loadCommentableSubmission(c)
	if wrote {
		return 0, nil, nil, true
	}
	if !h.subs.callerCanViewSubmission(c, sub.UserID) {
		_ = responses.NotFound(c, "submission")
		return 0, nil, nil, true
	}
	courseID, _ := c.ParamsInt("course_id")
	return uint(courseID), assignment, sub, false
}

// UpdateSubmissionComment edits a comment's text or publishes a draft.
// Only the author may edit.
//
//	PUT /courses/:course_id/assignments/:assignment_id/submissions/:user_id/comments/:comment_id
func (h *GradingUIHandler) UpdateSubmissionComment(c *fiber.Ctx) error {
	courseID, assignment, sub, wrote := h.commentTarget(c)
	if wrote {
		return nil
	}
	return h.updateComment(c, courseID, assignment, sub, false)
}

// UpdateAnonymousSubmissionComment is UpdateSubmissionComment by
// anonymous id, so a grader can edit a comment or publish a draft while
// the assignment anonymizes students (the route is instructor-gated).
//
//	PUT /courses/:course_id/assignments/:assignment_id/anonymous_submissions/:anonymous_id/comments/:comment_id
func (h *GradingUIHandler) UpdateAnonymousSubmissionComment(c *fiber.Ctx) error {
	courseID, assignment, sub, wrote := h.anonymousSubmission(c)
	if wrote {
		return nil
	}
	return h.updateComment(c, courseID, assignment, sub, true)
}

func (h *GradingUIHandler) updateComment(c *fiber.Ctx, courseID uint, assignment *models.Assignment, sub *models.Submission, anonymize bool) error {
	cm, wrote := h.loadComment(c, sub, assignment)
	if wrote {
		return nil
	}
	callerID, _ := c.Locals("user_id").(uint)
	if cm.AuthorID != callerID {
		return responses.Forbidden(c, "Only the author can edit a comment")
	}
	var input struct {
		Comment struct {
			TextComment *string `json:"text_comment"`
			Draft       *bool   `json:"draft"`
		} `json:"comment"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	if input.Comment.TextComment != nil && strings.TrimSpace(*input.Comment.TextComment) == "" && (cm.Attachments == nil || *cm.Attachments == "" || *cm.Attachments == "[]") {
		return responses.BadRequest(c, "Comment text is required")
	}
	// A draft can be published, but a sent comment (which the student
	// may already have read) never goes back to being a draft. This also
	// keeps a student from hiding a comment from the graders: drafts are
	// a grader tool (Canvas), and comment creation already refuses them
	// to students.
	if input.Comment.Draft != nil && *input.Comment.Draft && !cm.Draft {
		return responses.BadRequest(c, "A sent comment cannot be turned back into a draft")
	}
	wasDraft := cm.Draft
	if err := h.svc.UpdateComment(c.Context(), cm, input.Comment.TextComment, input.Comment.Draft); err != nil {
		return responses.InternalError(c, "Could not update comment")
	}
	// Publishing a grader's draft on an automatically-posted assignment
	// posts the submission (every member's, for a group comment), as
	// sending the comment directly would.
	if wasDraft && !cm.Draft {
		targets := h.svc.CommentCopySubmissions(c.Context(), cm, sub)
		if callerIsCourseStaff(c) && !assignment.UsesManualPostPolicy() && assignment.GradesPublished() {
			for _, target := range targets {
				_ = h.subs.submissionService.MarkPosted(c.Context(), target)
			}
		}
		// The comment is sent now: notify each recipient, with the same
		// gates as a comment sent directly (hidden / moderated stay quiet).
		authorName := ""
		if author, err := h.subs.userRepo.FindByID(c.Context(), callerID, callerAccountID(c)); err == nil && author != nil {
			authorName = author.Name
		}
		for _, target := range targets {
			h.subs.notifySubmissionComment(c.Context(), assignment, target, callerID, authorName, cm.Comment)
		}
	}
	return c.JSON(commentJSON(courseID, assignment.ID, commentPathKey(sub, anonymize), cm))
}

// DeleteSubmissionComment removes a comment: its author, or course staff.
//
//	DELETE /courses/:course_id/assignments/:assignment_id/submissions/:user_id/comments/:comment_id
func (h *GradingUIHandler) DeleteSubmissionComment(c *fiber.Ctx) error {
	_, assignment, sub, wrote := h.commentTarget(c)
	if wrote {
		return nil
	}
	return h.deleteComment(c, assignment, sub)
}

// DeleteAnonymousSubmissionComment is DeleteSubmissionComment by
// anonymous id (instructor-gated route).
//
//	DELETE /courses/:course_id/assignments/:assignment_id/anonymous_submissions/:anonymous_id/comments/:comment_id
func (h *GradingUIHandler) DeleteAnonymousSubmissionComment(c *fiber.Ctx) error {
	_, assignment, sub, wrote := h.anonymousSubmission(c)
	if wrote {
		return nil
	}
	return h.deleteComment(c, assignment, sub)
}

func (h *GradingUIHandler) deleteComment(c *fiber.Ctx, assignment *models.Assignment, sub *models.Submission) error {
	cm, wrote := h.loadComment(c, sub, assignment)
	if wrote {
		return nil
	}
	callerID, _ := c.Locals("user_id").(uint)
	if cm.AuthorID != callerID && !callerIsCourseStaff(c) {
		return responses.Forbidden(c, "Only the author or an instructor can delete a comment")
	}
	if err := h.svc.DeleteComment(c.Context(), cm); err != nil {
		return responses.InternalError(c, "Could not delete comment")
	}
	return c.JSON(fiber.Map{"deleted": true})
}

// UploadCommentFile stores a file (or a recorded audio/video clip) the
// caller will attach to a comment on the :user_id submission. Allowed
// for the submission's student and course staff; the file lands in the
// caller's own space and the comment-scoped download route governs who
// may read it.
//
//	POST /courses/:course_id/assignments/:assignment_id/submissions/:user_id/comment_files
func (h *GradingUIHandler) UploadCommentFile(c *fiber.Ctx) error {
	if h.subs.fileService == nil {
		return responses.InternalError(c, "File uploads are not configured")
	}
	courseID, err := c.ParamsInt("course_id")
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
	if _, wrote := h.subs.requireAssignmentInCourse(c, uint(assignmentID), uint(courseID)); wrote {
		return nil
	}
	callerID, _ := c.Locals("user_id").(uint)
	if !callerIsCourseStaff(c) && callerID != uint(userID) {
		return responses.NotFound(c, "submission")
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return responses.BadRequest(c, "File is required")
	}
	return h.subs.storeOwnFile(c, fileHeader, func(*models.Attachment) string { return "" })
}

// UploadAnonymousCommentFile is UploadCommentFile for SpeedGrader under
// anonymous grading (staff only; the route is instructor-gated).
func (h *GradingUIHandler) UploadAnonymousCommentFile(c *fiber.Ctx) error {
	if h.subs.fileService == nil {
		return responses.InternalError(c, "File uploads are not configured")
	}
	if _, _, _, wrote := h.anonymousSubmission(c); wrote {
		return nil
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return responses.BadRequest(c, "File is required")
	}
	return h.subs.storeOwnFile(c, fileHeader, func(*models.Attachment) string { return "" })
}

// commentHasFile reports whether fileID is attached to comment cm.
func commentHasFile(cm *models.SubmissionComment, fileID uint) bool {
	if cm.Attachments == nil || *cm.Attachments == "" {
		return false
	}
	var files []struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal([]byte(*cm.Attachments), &files); err != nil {
		return false
	}
	for _, f := range files {
		if f.ID == fileID {
			return true
		}
	}
	return false
}

// DownloadCommentFile streams a file attached to a comment the caller
// can see.
//
//	GET /courses/:course_id/assignments/:assignment_id/submissions/:user_id/comments/:comment_id/files/:file_id/download
func (h *GradingUIHandler) DownloadCommentFile(c *fiber.Ctx) error {
	if h.fileService == nil {
		return responses.NotFound(c, "file")
	}
	_, assignment, sub, wrote := h.commentTarget(c)
	if wrote {
		return nil
	}
	return h.streamCommentFile(c, assignment, sub)
}

// DownloadAnonymousCommentFile is DownloadCommentFile by anonymous id.
func (h *GradingUIHandler) DownloadAnonymousCommentFile(c *fiber.Ctx) error {
	if h.fileService == nil {
		return responses.NotFound(c, "file")
	}
	_, assignment, sub, wrote := h.anonymousSubmission(c)
	if wrote {
		return nil
	}
	return h.streamCommentFile(c, assignment, sub)
}

func (h *GradingUIHandler) streamCommentFile(c *fiber.Ctx, assignment *models.Assignment, sub *models.Submission) error {
	cm, wrote := h.loadComment(c, sub, assignment)
	if wrote {
		return nil
	}
	fileID, err := c.ParamsInt("file_id")
	if err != nil || !commentHasFile(cm, uint(fileID)) {
		return responses.NotFound(c, "file")
	}
	return h.subs.streamSubmissionAttachment(c, uint(fileID), sub.UserID)
}

// ---------------------------------------------------------------------
// Student annotation
// ---------------------------------------------------------------------

func hasSubmissionType(a *models.Assignment, t string) bool {
	return strings.Contains(","+strings.ReplaceAll(a.SubmissionTypes, " ", "")+",", ","+t+",")
}

// annotatableAttachment loads the assignment's annotatable document: a
// file of the same course in the caller's tenant. (result, wrote).
func (h *GradingUIHandler) annotatableAttachment(c *fiber.Ctx, courseID uint, a *models.Assignment) (*models.Attachment, bool) {
	if h.fileService == nil || a.AnnotatableAttachmentID == nil || *a.AnnotatableAttachmentID == 0 {
		_ = responses.NotFound(c, "document")
		return nil, true
	}
	att, err := h.fileService.GetAttachment(c.Context(), *a.AnnotatableAttachmentID, callerAccountID(c))
	if err != nil || att == nil || att.ContextType != "Course" || att.ContextID != courseID {
		_ = responses.NotFound(c, "document")
		return nil, true
	}
	return att, false
}

// StartAnnotation prepares a student_annotation assignment for the
// calling student: their (unsubmitted) submission row, which the
// document annotations hang off, and the document to annotate.
//
//	POST /courses/:course_id/assignments/:assignment_id/annotation_session
func (h *GradingUIHandler) StartAnnotation(c *fiber.Ctx) error {
	courseID, assignment, wrote := h.courseAndAssignment(c, "assignment_id")
	if wrote {
		return nil
	}
	if et, _ := c.Locals("enrollment_type").(string); et != "StudentEnrollment" {
		return responses.Forbidden(c, "Only students can submit to assignments")
	}
	if !hasSubmissionType(assignment, "student_annotation") {
		return responses.BadRequest(c, "This assignment is not a student annotation assignment")
	}
	att, wrote := h.annotatableAttachment(c, courseID, assignment)
	if wrote {
		return nil
	}
	callerID, _ := c.Locals("user_id").(uint)
	rows, err := h.svc.EnsureSubmissionRows(c.Context(), assignment, []uint{callerID})
	if err != nil || rows[callerID] == nil {
		return responses.InternalError(c, "Could not start annotating")
	}
	sub := rows[callerID]
	return c.JSON(fiber.Map{
		"submission": studentSubmissionToJSON(sub),
		"document":   annotatableDocumentJSON(courseID, assignment.ID, att),
	})
}

func annotatableDocumentJSON(courseID, assignmentID uint, att *models.Attachment) fiber.Map {
	return fiber.Map{
		"id":           att.ID,
		"display_name": att.DisplayName,
		"content_type": att.ContentType,
		"size":         att.Size,
		"url":          fmt.Sprintf("/api/v1/courses/%d/assignments/%d/annotatable_document", courseID, assignmentID),
	}
}

// GetAnnotatableDocument streams the document students annotate, to
// anyone who can see the assignment.
//
//	GET /courses/:course_id/assignments/:assignment_id/annotatable_document
func (h *GradingUIHandler) GetAnnotatableDocument(c *fiber.Ctx) error {
	courseID, assignment, wrote := h.courseAndAssignment(c, "assignment_id")
	if wrote {
		return nil
	}
	att, wrote := h.annotatableAttachment(c, courseID, assignment)
	if wrote {
		return nil
	}
	return h.subs.streamSubmissionAttachment(c, att.ID, 0)
}

// ---------------------------------------------------------------------
// Moderated grading
// ---------------------------------------------------------------------

// studentFromBody resolves {user_id} or {anonymous_id} in a moderation
// request to a student of the assignment's course. (result, wrote).
func (h *GradingUIHandler) studentFromBody(c *fiber.Ctx, courseID uint, assignment *models.Assignment, userID uint, anonymousID string) (uint, bool) {
	if anonymousID != "" {
		sub, err := h.svc.ResolveAnonymousID(c.Context(), assignment.ID, anonymousID, callerAccountID(c))
		if err != nil {
			_ = responses.NotFound(c, "submission")
			return 0, true
		}
		return sub.UserID, false
	}
	if userID == 0 || !h.subs.submissionService.IsCourseStudent(c.Context(), courseID, userID, callerAccountID(c)) {
		_ = responses.NotFound(c, "user")
		return 0, true
	}
	return userID, false
}

func (h *GradingUIHandler) moderationError(c *fiber.Ctx, err error) error {
	switch err {
	case service.ErrNotModerated:
		return responses.BadRequest(c, err.Error())
	case service.ErrGradesAlreadyPublished, service.ErrGraderLimitReached:
		return responses.Error(c, fiber.StatusConflict, err.Error())
	case service.ErrNotModerator:
		return responses.Forbidden(c, err.Error())
	case service.ErrProvisionalGradeInvalid:
		return responses.NotFound(c, "provisional grade")
	}
	return responses.BadRequest(c, err.Error())
}

// SaveProvisionalGrade records the caller's provisional grade.
//
//	PUT /courses/:course_id/assignments/:assignment_id/provisional_grades
//	{ "user_id" | "anonymous_id", "posted_grade", "final" }
func (h *GradingUIHandler) SaveProvisionalGrade(c *fiber.Ctx) error {
	courseID, assignment, wrote := h.courseAndAssignment(c, "assignment_id")
	if wrote {
		return nil
	}
	var input struct {
		UserID      uint   `json:"user_id"`
		AnonymousID string `json:"anonymous_id"`
		PostedGrade string `json:"posted_grade"`
		Final       bool   `json:"final"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	studentID, wrote := h.studentFromBody(c, courseID, assignment, input.UserID, input.AnonymousID)
	if wrote {
		return nil
	}
	callerID, _ := c.Locals("user_id").(uint)
	isAdmin, _ := c.Locals("is_admin").(bool)
	pg, err := h.svc.SaveProvisionalGrade(c.Context(), assignment, studentID, callerID, service.IsModerator(assignment, callerID, isAdmin), input.Final, input.PostedGrade)
	if err != nil {
		return h.moderationError(c, err)
	}
	return c.JSON(fiber.Map{
		"id":        pg.ID,
		"score":     pg.Score,
		"grade":     pg.Grade,
		"final":     pg.Final,
		"graded_at": pg.GradedAt,
		"mine":      true,
	})
}

// requireModerator answers 403 unless the caller moderates the assignment.
func (h *GradingUIHandler) requireModerator(c *fiber.Ctx, a *models.Assignment) bool {
	callerID, _ := c.Locals("user_id").(uint)
	isAdmin, _ := c.Locals("is_admin").(bool)
	if !a.ModeratedGrading {
		_ = responses.BadRequest(c, service.ErrNotModerated.Error())
		return true
	}
	if !service.IsModerator(a, callerID, isAdmin) {
		_ = responses.Forbidden(c, service.ErrNotModerator.Error())
		return true
	}
	return false
}

// GetModeration is the moderator's grade summary: every student, every
// grader's provisional grade, and the current selection.
//
//	GET /courses/:course_id/assignments/:assignment_id/moderation
func (h *GradingUIHandler) GetModeration(c *fiber.Ctx) error {
	courseID, assignment, wrote := h.courseAndAssignment(c, "assignment_id")
	if wrote {
		return nil
	}
	if h.requireModerator(c, assignment) {
		return nil
	}
	callerID, _ := c.Locals("user_id").(uint)
	isAdmin, _ := c.Locals("is_admin").(bool)

	students, err := h.subs.submissionService.CourseStudents(c.Context(), courseID, callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not load students")
	}
	ids := make([]uint, 0, len(students))
	for _, s := range students {
		ids = append(ids, s.UserID)
	}
	rows, err := h.svc.EnsureSubmissionRows(c.Context(), assignment, ids)
	if err != nil {
		return responses.InternalError(c, "Could not load submissions")
	}
	subIDs := make([]uint, 0, len(rows))
	for _, r := range rows {
		subIDs = append(subIDs, r.ID)
	}
	pgs, _ := h.svc.ProvisionalGradesForStudents(c.Context(), subIDs)
	selections, _ := h.svc.ModerationSelections(c.Context(), assignment.ID)
	graders, _ := h.svc.ModerationGraders(c.Context(), assignment.ID)

	anonymizeStudents := h.svc.AnonymizeStudents(c.Context(), assignment, ids)
	if anonymizeStudents {
		service.SortForAnonymity(assignment.ID, students)
	}
	hideGraderNames := !service.CanViewOtherGraderIdentities(assignment, callerID, isAdmin)
	graderJSON := make([]fiber.Map, 0, len(graders))
	graderName := map[uint]string{}
	for _, g := range graders {
		name := "Grader " + strings.ToUpper(g.AnonymousID)
		if !hideGraderNames || g.UserID == callerID {
			if u, err := h.userRepo.FindByID(c.Context(), g.UserID, callerAccountID(c)); err == nil && u != nil {
				name = u.Name
			}
		}
		graderName[g.UserID] = name
		graderJSON = append(graderJSON, fiber.Map{
			"grader_id":    g.ID,
			"name":         name,
			"anonymous_id": g.AnonymousID,
			"slot_taken":   g.SlotTaken,
			"is_me":        g.UserID == callerID,
		})
	}

	studentJSON := make([]fiber.Map, 0, len(students))
	for i, s := range students {
		row := rows[s.UserID]
		if row == nil {
			continue
		}
		entry := fiber.Map{"submission_id": row.ID}
		if anonymizeStudents {
			entry["anonymous_id"] = row.AnonymousID
			entry["user_id"] = nil
			entry["name"] = fmt.Sprintf("Student %d", i+1)
		} else {
			entry["user_id"] = s.UserID
			entry["name"] = s.Name
		}
		list := make([]fiber.Map, 0)
		for _, pg := range pgs[row.ID] {
			list = append(list, fiber.Map{
				"id":          pg.ID,
				"score":       pg.Score,
				"grade":       pg.Grade,
				"final":       pg.Final,
				"scorer_name": graderName[pg.ScorerID],
				"mine":        pg.ScorerID == callerID,
			})
		}
		entry["provisional_grades"] = list
		entry["selected_provisional_grade_id"] = selections[s.UserID]
		entry["current_score"] = row.Score
		studentJSON = append(studentJSON, entry)
	}

	return c.JSON(fiber.Map{
		"assignment": fiber.Map{
			"id":                 assignment.ID,
			"name":               assignment.Name,
			"points_possible":    assignment.PointsPossible,
			"grader_count":       assignment.GraderCount,
			"grades_published":   assignment.GradesPublished(),
			"anonymize_students": anonymizeStudents,
		},
		"graders":  graderJSON,
		"students": studentJSON,
	})
}

// SelectProvisionalGrade records the moderator's choice for a student.
//
//	PUT /courses/:course_id/assignments/:assignment_id/moderation/selections
//	{ "user_id" | "anonymous_id", "provisional_grade_id" }
func (h *GradingUIHandler) SelectProvisionalGrade(c *fiber.Ctx) error {
	courseID, assignment, wrote := h.courseAndAssignment(c, "assignment_id")
	if wrote {
		return nil
	}
	if h.requireModerator(c, assignment) {
		return nil
	}
	var input struct {
		UserID             uint   `json:"user_id"`
		AnonymousID        string `json:"anonymous_id"`
		ProvisionalGradeID uint   `json:"provisional_grade_id"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	studentID, wrote := h.studentFromBody(c, courseID, assignment, input.UserID, input.AnonymousID)
	if wrote {
		return nil
	}
	if err := h.svc.SelectProvisionalGrade(c.Context(), assignment, studentID, input.ProvisionalGradeID); err != nil {
		return h.moderationError(c, err)
	}
	return c.JSON(fiber.Map{"selected_provisional_grade_id": input.ProvisionalGradeID})
}

// PublishModeratedGrades releases the selected grades.
//
//	POST /courses/:course_id/assignments/:assignment_id/moderation/publish
func (h *GradingUIHandler) PublishModeratedGrades(c *fiber.Ctx) error {
	_, assignment, wrote := h.courseAndAssignment(c, "assignment_id")
	if wrote {
		return nil
	}
	if h.requireModerator(c, assignment) {
		return nil
	}
	callerID, _ := c.Locals("user_id").(uint)
	res, err := h.svc.PublishGrades(c.Context(), assignment, callerID, callerAccountID(c))
	if err != nil {
		return h.moderationError(c, err)
	}
	return c.JSON(res)
}

// ---------------------------------------------------------------------
// Custom grade statuses
// ---------------------------------------------------------------------

func customStatusJSON(s *models.CustomGradeStatus) fiber.Map {
	return fiber.Map{
		"id":                     s.ID,
		"name":                   s.Name,
		"color":                  s.Color,
		"applies_to_submissions": s.AppliesToSubmissions,
		"applies_to_final_grade": s.AppliesToFinalGrade,
		"workflow_state":         s.WorkflowState,
	}
}

// accountParam parses :account_id and enforces the caller's tenant.
// (result, wrote).
func accountParam(c *fiber.Ctx) (uint, bool) {
	id, err := c.ParamsInt("account_id")
	if err != nil || id <= 0 {
		_ = responses.BadRequest(c, "Invalid account ID")
		return 0, true
	}
	if assertSameTenant(c, uint(id)) {
		return 0, true
	}
	return uint(id), false
}

// ListAccountCustomGradeStatuses lists an account's statuses (admin).
func (h *GradingUIHandler) ListAccountCustomGradeStatuses(c *fiber.Ctx) error {
	accountID, wrote := accountParam(c)
	if wrote {
		return nil
	}
	return h.renderStatuses(c, accountID)
}

func (h *GradingUIHandler) renderStatuses(c *fiber.Ctx, accountID uint) error {
	list, err := h.svc.ListCustomGradeStatuses(c.Context(), accountID)
	if err != nil {
		return responses.InternalError(c, "Could not load grade statuses")
	}
	out := make([]fiber.Map, 0, len(list))
	for i := range list {
		out = append(out, customStatusJSON(&list[i]))
	}
	return c.JSON(out)
}

// ListCourseCustomGradeStatuses lists the statuses usable in a course
// (its account's), for graders.
func (h *GradingUIHandler) ListCourseCustomGradeStatuses(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	acct, err := h.svc.CourseAccountID(c.Context(), uint(courseID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "course")
	}
	return h.renderStatuses(c, acct)
}

// CreateCustomGradeStatus adds an account status (admin).
func (h *GradingUIHandler) CreateCustomGradeStatus(c *fiber.Ctx) error {
	accountID, wrote := accountParam(c)
	if wrote {
		return nil
	}
	var input struct {
		CustomGradeStatus service.CustomGradeStatusInput `json:"custom_grade_status"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	callerID, _ := c.Locals("user_id").(uint)
	st, err := h.svc.CreateCustomGradeStatus(c.Context(), accountID, callerID, input.CustomGradeStatus)
	if err != nil {
		if err == service.ErrCustomStatusLimit {
			return responses.Error(c, fiber.StatusConflict, err.Error())
		}
		return responses.BadRequest(c, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(customStatusJSON(st))
}

// UpdateCustomGradeStatus edits an account status (admin).
func (h *GradingUIHandler) UpdateCustomGradeStatus(c *fiber.Ctx) error {
	accountID, wrote := accountParam(c)
	if wrote {
		return nil
	}
	id, err := c.ParamsInt("status_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid status ID")
	}
	var input struct {
		CustomGradeStatus service.CustomGradeStatusInput `json:"custom_grade_status"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	st, err := h.svc.UpdateCustomGradeStatus(c.Context(), accountID, uint(id), input.CustomGradeStatus)
	if err != nil {
		if err == service.ErrCustomStatusNotFound {
			return responses.NotFound(c, "custom grade status")
		}
		return responses.BadRequest(c, err.Error())
	}
	return c.JSON(customStatusJSON(st))
}

// DeleteCustomGradeStatus removes an account status (admin).
func (h *GradingUIHandler) DeleteCustomGradeStatus(c *fiber.Ctx) error {
	accountID, wrote := accountParam(c)
	if wrote {
		return nil
	}
	id, err := c.ParamsInt("status_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid status ID")
	}
	if err := h.svc.DeleteCustomGradeStatus(c.Context(), accountID, uint(id)); err != nil {
		if err == service.ErrCustomStatusNotFound {
			return responses.NotFound(c, "custom grade status")
		}
		return responses.InternalError(c, "Could not delete grade status")
	}
	return c.JSON(fiber.Map{"deleted": true})
}

// ---------------------------------------------------------------------
// Peer reviews
// ---------------------------------------------------------------------

// AssignPeerReview manually assigns one review (instructor).
//
//	POST /courses/:course_id/assignments/:id/peer_reviews/manual { reviewer_id, reviewee_id }
func (h *GradingUIHandler) AssignPeerReview(c *fiber.Ctx) error {
	courseID, assignment, wrote := h.courseAndAssignment(c, "id")
	if wrote {
		return nil
	}
	var input struct {
		ReviewerID uint `json:"reviewer_id"`
		RevieweeID uint `json:"reviewee_id"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	acct := callerAccountID(c)
	if !h.subs.submissionService.IsCourseStudent(c.Context(), courseID, input.ReviewerID, acct) ||
		!h.subs.submissionService.IsCourseStudent(c.Context(), courseID, input.RevieweeID, acct) {
		return responses.NotFound(c, "user")
	}
	pr, err := h.svc.AssignPeerReview(c.Context(), assignment, input.ReviewerID, input.RevieweeID)
	if err != nil {
		if err == service.ErrPeerReviewConflict {
			return responses.Error(c, fiber.StatusConflict, err.Error())
		}
		return responses.BadRequest(c, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(pr)
}

// DeletePeerReview removes an assigned review (instructor).
//
//	DELETE /courses/:course_id/assignments/:id/peer_reviews/:review_id
func (h *GradingUIHandler) DeletePeerReview(c *fiber.Ctx) error {
	_, assignment, wrote := h.courseAndAssignment(c, "id")
	if wrote {
		return nil
	}
	reviewID, err := c.ParamsInt("review_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid review ID")
	}
	if err := h.svc.DeletePeerReview(c.Context(), assignment, uint(reviewID), callerAccountID(c)); err != nil {
		return responses.NotFound(c, "peer review")
	}
	return c.JSON(fiber.Map{"deleted": true})
}

// ListReceivedPeerReviews returns the completed reviews of the caller's
// own work. Reviewer identities are hidden for anonymous peer reviews.
//
//	GET /courses/:course_id/assignments/:id/peer_reviews/received
func (h *GradingUIHandler) ListReceivedPeerReviews(c *fiber.Ctx) error {
	_, assignment, wrote := h.courseAndAssignment(c, "id")
	if wrote {
		return nil
	}
	callerID, _ := c.Locals("user_id").(uint)
	reviews, err := h.svc.ReceivedPeerReviews(c.Context(), assignment.ID, callerID, callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not load peer reviews")
	}
	out := make([]fiber.Map, 0, len(reviews))
	for i, r := range reviews {
		j := fiber.Map{
			"id":          r.ID,
			"score":       r.Score,
			"comments":    r.Comments,
			"rubric_data": jsonOrNil(r.RubricData),
			"updated_at":  r.UpdatedAt,
		}
		if assignment.AnonymousPeerReviews {
			j["reviewer_id"] = nil
			j["reviewer_name"] = fmt.Sprintf("Anonymous reviewer %d", i+1)
		} else {
			j["reviewer_id"] = r.ReviewerID
			if u, err := h.userRepo.FindByID(c.Context(), r.ReviewerID, callerAccountID(c)); err == nil && u != nil {
				j["reviewer_name"] = u.Name
			}
		}
		out = append(out, j)
	}
	return c.JSON(out)
}

// DownloadPeerReviewFile streams a file of the submission an assigned
// reviewer is reviewing, without naming the reviewee in the URL (so
// anonymous peer reviews stay anonymous).
//
//	GET /peer_reviews/:review_id/files/:file_id/download
func (h *GradingUIHandler) DownloadPeerReviewFile(c *fiber.Ctx) error {
	if h.fileService == nil || h.peerReviewService == nil {
		return responses.NotFound(c, "file")
	}
	reviewID, err := c.ParamsInt("review_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid review ID")
	}
	fileID, err := c.ParamsInt("file_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid file ID")
	}
	callerID, _ := c.Locals("user_id").(uint)
	pr, err := h.peerReviewService.GetByID(c.Context(), uint(reviewID), callerAccountID(c))
	if err != nil || pr == nil || callerID == 0 || pr.ReviewerID != callerID {
		return responses.NotFound(c, "file")
	}
	sub, err := h.peerReviewService.SubmissionForReview(c.Context(), pr, callerAccountID(c))
	if err != nil || sub == nil || !submissionHasAttachment(sub, uint(fileID)) {
		return responses.NotFound(c, "file")
	}
	return h.subs.streamSubmissionAttachment(c, uint(fileID), sub.UserID)
}

// jsonOrNil decodes a JSON text column for a response.
func jsonOrNil(raw *string) interface{} {
	if raw == nil || *raw == "" {
		return nil
	}
	var v interface{}
	if err := json.Unmarshal([]byte(*raw), &v); err != nil {
		return nil
	}
	return v
}

// maskAnonymizedGradebook hides the grades of assignments whose students
// are anonymized for graders: matching a gradebook score to SpeedGrader
// would otherwise reveal who is who. Returns the masked assignment ids.
func maskAnonymizedGradebook(c *fiber.Ctx, g *service.GradingUIService, courseID uint, gb *service.GradebookEntry) []uint {
	out := []uint{}
	if g == nil || gb == nil {
		return out
	}
	studentIDs := make([]uint, 0, len(gb.Students))
	for _, s := range gb.Students {
		studentIDs = append(studentIDs, s.ID)
	}
	anon := g.AnonymizedAssignmentIDs(c.Context(), courseID, studentIDs)
	if len(anon) == 0 {
		return out
	}
	for id := range anon {
		out = append(out, id)
	}
	for student, subs := range gb.Submissions {
		for key, sub := range subs {
			if sub == nil || !anon[sub.AssignmentID] {
				continue
			}
			masked := *sub
			masked.Score, masked.Grade, masked.EnteredScore, masked.EnteredGrade, masked.PointsDeducted = nil, nil, nil, nil, nil
			masked.Excused = false
			if masked.WorkflowState == models.SubmissionGraded {
				masked.WorkflowState = models.SubmissionSubmitted
			}
			gb.Submissions[student][key] = &masked
		}
	}
	return out
}

// ListCourseGradingPeriods lists the grading periods that apply to the
// course (its term's period set; the account-level period routes are
// admin-only), for the gradebook's grading-period filter. is_closed
// honours the close date.
//
//	GET /courses/:course_id/grading_periods
func (h *GradingUIHandler) ListCourseGradingPeriods(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	periods, err := h.subs.submissionService.CourseGradingPeriods(c.Context(), uint(courseID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "course")
	}
	now := time.Now()
	out := make([]fiber.Map, 0, len(periods))
	for i := range periods {
		p := &periods[i]
		out = append(out, fiber.Map{
			"id":         p.ID,
			"title":      p.Title,
			"start_date": p.StartDate,
			"end_date":   p.EndDate,
			"close_date": p.CloseDate,
			"weight":     p.Weight,
			"is_closed":  service.GradingPeriodClosed(p, now),
		})
	}
	return c.JSON(out)
}
