package handlers

// File-upload quiz questions (B127).
//
// A student's upload is stored as a User-context attachment owned by the
// student (so the generic /files/:id/download only serves it to its owner),
// and the quiz answer records {"attachment_id", "filename"}. Course staff
// (and linked observers) fetch it through the attempt-scoped download
// below, which only serves the attachment actually recorded on that
// attempt's answer — never an arbitrary attachment id.

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/settingsctx"
)

// SetFileService wires the file service used by file-upload questions.
func (h *QuizSubmissionHandler) SetFileService(fs *service.FileService) {
	h.fileService = fs
}

type quizFileAnswer struct {
	AttachmentID uint   `json:"attachment_id"`
	Filename     string `json:"filename"`
	ContentType  string `json:"content_type,omitempty"`
	Size         int64  `json:"size,omitempty"`
}

// UploadAnswerFile handles
// POST /courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/questions/:question_id/file
func (h *QuizSubmissionHandler) UploadAnswerFile(c *fiber.Ctx) error {
	if h.fileService == nil {
		return responses.InternalError(c, "File uploads are not configured")
	}
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}
	questionID, err := c.ParamsInt("question_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid question ID")
	}
	if _, wrote := h.requireQuizInCourse(c, uint(quizID), uint(courseID)); wrote {
		return nil
	}
	submission, wrote := h.loadSubmission(c, uint(quizID))
	if wrote {
		return nil
	}
	userID, _ := c.Locals("user_id").(uint)
	if userID == 0 || submission.UserID != userID {
		return responses.NotFound(c, "quiz submission")
	}
	if submission.WorkflowState != "untaken" {
		return responses.BadRequest(c, "quiz submission is not in progress")
	}
	if _, err := h.quizService.FileUploadQuestionForSubmission(c.Context(), submission, uint(questionID)); err != nil {
		return responses.NotFound(c, "quiz question")
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return responses.BadRequest(c, "File is required")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return responses.InternalError(c, "Could not open uploaded file")
	}
	defer file.Close()

	// Browsers send "" or application/octet-stream for types they don't
	// know; derive from the extension so the MIME/extension check applies
	// consistently. (Downloads are always served as attachments anyway.)
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" || strings.HasPrefix(contentType, "application/octet-stream") {
		contentType = mime.TypeByExtension(strings.ToLower(filepath.Ext(fileHeader.Filename)))
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

	payload, _ := json.Marshal(quizFileAnswer{
		AttachmentID: attachment.ID,
		Filename:     attachment.DisplayName,
		ContentType:  attachment.ContentType,
		Size:         attachment.Size,
	})
	answer, err := h.quizService.AnswerQuestion(c.Context(), submission.ID, uint(questionID), string(payload))
	if err != nil {
		if errors.Is(err, service.ErrQuestionNotInSubmission) {
			return responses.NotFound(c, "quiz question")
		}
		return responses.BadRequest(c, err.Error())
	}

	out := quizSubmissionAnswerToJSON(answer)
	out["attachment_id"] = attachment.ID
	out["filename"] = attachment.DisplayName
	return c.Status(fiber.StatusCreated).JSON(out)
}

// DownloadAnswerFile handles
// GET /courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/questions/:question_id/file
// for the attempt owner, course staff and linked observers.
func (h *QuizSubmissionHandler) DownloadAnswerFile(c *fiber.Ctx) error {
	if h.fileService == nil {
		return responses.NotFound(c, "file")
	}
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}
	questionID, err := c.ParamsInt("question_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid question ID")
	}
	if _, wrote := h.requireQuizInCourse(c, uint(quizID), uint(courseID)); wrote {
		return nil
	}
	submission, wrote := h.loadSubmission(c, uint(quizID))
	if wrote {
		return nil
	}
	if !h.callerCanViewAttempt(c, submission.UserID) {
		return responses.NotFound(c, "quiz submission")
	}

	answer, err := h.quizService.GetSubmissionAnswer(c.Context(), submission.ID, uint(questionID))
	if err != nil || answer == nil {
		return responses.NotFound(c, "file")
	}
	var fa quizFileAnswer
	if err := json.Unmarshal([]byte(answer.Answer), &fa); err != nil || fa.AttachmentID == 0 {
		return responses.NotFound(c, "file")
	}
	attachment, err := h.fileService.GetAttachment(c.Context(), fa.AttachmentID, callerAccountID(c))
	// The attachment must be the attempt owner's own upload.
	if err != nil || attachment == nil || attachment.ContextType != "User" || attachment.ContextID != submission.UserID {
		return responses.NotFound(c, "file")
	}

	if viewer, _ := c.Locals("user_id").(uint); viewer != 0 && viewer != submission.UserID && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), viewer, submission.UserID, "download", "quiz_file_answer", "attachments", attachment.ID, c.IP(), c.Get("User-Agent"))
	}

	safeName := filepath.Base(attachment.Filename)
	safeName = strings.NewReplacer("\"", "", "\r", "", "\n", "").Replace(safeName)
	// Always a download — student uploads are untrusted content.
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"; filename*=UTF-8''%s", safeName, url.PathEscape(safeName)))
	c.Set("X-Content-Type-Options", "nosniff")

	downloadCtx := settingsctx.WithAccountID(c.Context(), callerAccountID(c))
	reader, err := h.fileService.StorageBackend().Get(downloadCtx, attachment.StoragePath)
	if err != nil {
		return responses.InternalError(c, "Could not locate file")
	}
	defer reader.Close()
	c.Set("Content-Type", "application/octet-stream")
	return c.SendStream(reader)
}
