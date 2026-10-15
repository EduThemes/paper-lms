package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/settingsctx"
	"github.com/EduThemes/paper-lms/internal/storage"
	"github.com/gofiber/fiber/v2"
)

// UploadSubmissionFile stores a file a student is about to submit.
//
//	POST /courses/:course_id/assignments/:assignment_id/submissions/self/files
//
// B013: the course files route is instructor-only, so students had no way
// to upload work for an online_upload assignment. Like Canvas, the file
// lands in the student's personal (User) context; the submission later
// references it and the submission-scoped download route below governs
// who may read it.
func (h *SubmissionHandler) UploadSubmissionFile(c *fiber.Ctx) error {
	if h.fileService == nil {
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
	userID, _ := c.Locals("user_id").(uint)
	if userID == 0 {
		return responses.Unauthorized(c)
	}
	if et, _ := c.Locals("enrollment_type").(string); et != "StudentEnrollment" {
		return responses.Forbidden(c, "Only students can submit to assignments")
	}

	assignment, wrote := h.requireAssignmentInCourse(c, uint(assignmentID), uint(courseID))
	if wrote {
		return nil
	}
	types := "," + strings.ReplaceAll(assignment.SubmissionTypes, " ", "") + ","
	acceptsUpload := strings.Contains(types, ",online_upload,")
	acceptsMedia := strings.Contains(types, ",media_recording,")
	if !acceptsUpload && !acceptsMedia {
		return responses.BadRequest(c, "This assignment does not accept file uploads")
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return responses.BadRequest(c, "File is required")
	}
	// A media recording is checked by type below; any other file must
	// have one of the assignment's allowed extensions.
	claimedMedia := service.IsMediaContentType(fileHeader.Header.Get("Content-Type"))
	if !acceptsMedia || !claimedMedia {
		if !acceptsUpload {
			return responses.BadRequest(c, "This assignment accepts audio or video recordings only")
		}
		if !service.FileExtensionAllowed(assignment, fileHeader.Filename) {
			return responses.BadRequest(c, "This file type is not allowed for this assignment (allowed: "+assignment.AllowedExtensions+")")
		}
	}
	return h.storeOwnFile(c, fileHeader, func(att *models.Attachment) string {
		return submissionFileDownloadURL(uint(courseID), uint(assignmentID), userID, att.ID)
	})
}

// storeOwnFile stores an uploaded file in the caller's personal (User)
// context and answers 201 with its JSON (url from urlFor). Used for
// submission files and for comment attachments / recordings.
func (h *SubmissionHandler) storeOwnFile(c *fiber.Ctx, fileHeader *multipart.FileHeader, urlFor func(*models.Attachment) string) error {
	userID, _ := c.Locals("user_id").(uint)
	if userID == 0 {
		return responses.Unauthorized(c)
	}
	file, err := fileHeader.Open()
	if err != nil {
		return responses.InternalError(c, "Could not open uploaded file")
	}
	defer func() { _ = file.Close() }()

	// Same content-type hardening as the course upload path: never trust
	// a client claim of an inline-safe type the bytes contradict.
	contentType := fileHeader.Header.Get("Content-Type")
	sniffBuf := make([]byte, 512)
	n, _ := io.ReadFull(file, sniffBuf)
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return responses.InternalError(c, "Could not read uploaded file")
	}
	if n > 0 {
		sniffed := http.DetectContentType(sniffBuf[:n])
		if isInlineSafeMIME(contentType) && !isInlineSafeMIME(sniffed) &&
			!strings.HasPrefix(sniffed, "application/octet-stream") {
			contentType = sniffed
		}
	}

	attachment := &models.Attachment{
		ContextType: "User",
		ContextID:   userID,
		UserID:      userID,
		DisplayName: fileHeader.Filename,
		Filename:    fileHeader.Filename,
		ContentType: contentType,
		Size:        fileHeader.Size,
	}
	uploadCtx := settingsctx.WithAccountID(c.Context(), callerAccountID(c))
	if err := h.fileService.UploadFile(uploadCtx, attachment, file); err != nil {
		return responses.BadRequest(c, "Could not upload file: "+err.Error())
	}

	j := attachmentToJSON(attachment)
	j["url"] = urlFor(attachment)
	return c.Status(fiber.StatusCreated).JSON(j)
}

// DownloadSubmissionFile streams a file attached to a submission.
//
//	GET /courses/:course_id/assignments/:assignment_id/submissions/:user_id/files/:file_id/download
//
// Access follows the submission: its owner, course staff, a linked
// observer, or an assigned peer reviewer — and only for a file id that
// is actually attached to that submission. Every miss is a 404.
func (h *SubmissionHandler) DownloadSubmissionFile(c *fiber.Ctx) error {
	if h.fileService == nil {
		return responses.NotFound(c, "file")
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
	fileID, err := c.ParamsInt("file_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid file ID")
	}

	assignment, wrote := h.requireAssignmentInCourse(c, uint(assignmentID), uint(courseID))
	if wrote {
		return nil
	}
	submission, err := h.submissionService.GetByAssignmentAndUser(c.Context(), uint(assignmentID), uint(userID), callerAccountID(c))
	if err != nil || submission == nil {
		return responses.NotFound(c, "file")
	}
	// Anonymous grading: graders download through the anonymous route.
	if callerID, _ := c.Locals("user_id").(uint); callerID != submission.UserID && h.staffAnonymized(c, uint(courseID), assignment) {
		return responses.NotFound(c, "file")
	}
	if !submissionHasAttachment(submission, uint(fileID)) {
		return responses.NotFound(c, "file")
	}
	if !h.callerCanViewSubmission(c, submission.UserID) &&
		!h.callerIsAssignedPeerReviewer(c, uint(assignmentID), submission.UserID) &&
		!h.callerOwnSubmissionHasFile(c, uint(assignmentID), uint(fileID)) {
		return responses.NotFound(c, "file")
	}

	return h.streamSubmissionAttachment(c, uint(fileID), submission.UserID)
}

// streamSubmissionAttachment streams attachment fileID (already
// authorized by the caller) and audits a download of a student's file.
func (h *SubmissionHandler) streamSubmissionAttachment(c *fiber.Ctx, fileID, studentID uint) error {
	attachment, err := h.fileService.GetAttachment(c.Context(), fileID, callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "file")
	}

	if callerID, _ := c.Locals("user_id").(uint); callerID != 0 && studentID != 0 && h.auditService != nil && attachment.UserID != callerID {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, studentID, "download", "submission_file_download", "attachments", attachment.ID, c.IP(), c.Get("User-Agent"))
	}

	safeName := filepath.Base(attachment.Filename)
	safeName = strings.ReplaceAll(safeName, "\"", "")
	safeName = strings.ReplaceAll(safeName, "\r", "")
	safeName = strings.ReplaceAll(safeName, "\n", "")
	disposition := "attachment"
	if isInlineSafeMIME(attachment.ContentType) {
		disposition = "inline"
	}
	c.Set("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"; filename*=UTF-8''%s", disposition, safeName, url.PathEscape(safeName)))

	backend := h.fileService.StorageBackend()
	if _, isS3 := backend.(*storage.S3Backend); isS3 {
		downloadURL, err := h.fileService.GetFileURL(c.Context(), attachment.ID, callerAccountID(c))
		if err != nil {
			return responses.InternalError(c, "Could not generate download URL")
		}
		return c.Redirect(downloadURL, fiber.StatusTemporaryRedirect)
	}

	downloadCtx := settingsctx.WithAccountID(c.Context(), callerAccountID(c))
	reader, err := backend.Get(downloadCtx, attachment.StoragePath)
	if err != nil {
		return responses.InternalError(c, "Could not locate file")
	}
	// No defer Close: fasthttp streams the body after the handler returns
	// and closes the reader itself (it implements io.Closer). Closing it
	// here would truncate the download.

	c.Set("Content-Type", service.ServedContentType(attachment.ContentType))
	return c.SendStream(reader)
}

// callerOwnSubmissionHasFile covers group assignments: a group
// submission is copied onto every member's row with the submitter's
// download URLs, so a member may download a file that is attached to
// their OWN submission of this assignment even though the URL names the
// classmate who uploaded it.
func (h *SubmissionHandler) callerOwnSubmissionHasFile(c *fiber.Ctx, assignmentID, fileID uint) bool {
	callerID, _ := c.Locals("user_id").(uint)
	if callerID == 0 {
		return false
	}
	own, err := h.submissionService.GetByAssignmentAndUser(c.Context(), assignmentID, callerID, callerAccountID(c))
	return err == nil && submissionHasAttachment(own, fileID)
}

// submissionHasAttachment reports whether fileID is one of the files
// recorded on the submission.
func submissionHasAttachment(s *models.Submission, fileID uint) bool {
	if s == nil || s.Attachments == nil || *s.Attachments == "" {
		return false
	}
	var attachments []struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal([]byte(*s.Attachments), &attachments); err != nil {
		return false
	}
	for _, a := range attachments {
		if a.ID == fileID {
			return true
		}
	}
	return false
}
