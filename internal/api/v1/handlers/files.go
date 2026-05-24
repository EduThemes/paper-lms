package handlers

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/settingsctx"
	"github.com/EduThemes/paper-lms/internal/storage"
	"github.com/gofiber/fiber/v2"
)

type FileHandler struct {
	fileService         *service.FileService
	enrollmentRepo      repository.EnrollmentRepository
	groupMembershipRepo repository.GroupMembershipRepository
	auditService        *service.AuditService
}

// NewFileHandler wires the file-handler dependencies. groupMembershipRepo
// was added in the F-041 fix to authz Group-context downloads.
func NewFileHandler(fileService *service.FileService, enrollmentRepo repository.EnrollmentRepository, groupMembershipRepo repository.GroupMembershipRepository, auditService *service.AuditService) *FileHandler {
	return &FileHandler{
		fileService:         fileService,
		enrollmentRepo:      enrollmentRepo,
		groupMembershipRepo: groupMembershipRepo,
		auditService:        auditService,
	}
}

// inlineSafeMIMEPrefixes is the allow-list of MIME prefixes that may be
// served with Content-Disposition: inline (rendered in the browser
// rather than downloaded). Anything not on the list defaults to
// Disposition: attachment so a malicious upload can't execute as
// script in the LMS origin.
//
// SECURITY (F-040): SVG is the canonical XSS vehicle (<script> inside
// <svg> runs same-origin), so image/svg+xml is intentionally NOT
// inline-safe. text/html, application/xhtml+xml, application/xml are
// likewise risky. PDF can embed scripts in some viewers — we err on
// the side of attachment.
var inlineSafeMIMEPrefixes = []string{
	"image/png",
	"image/jpeg",
	"image/gif",
	"image/webp",
	"image/avif",
	"image/heic",
}

// isInlineSafeMIME returns true when the given MIME type matches one
// of the inline-safe prefixes. Exact-match style — a stored MIME of
// `image/svg+xml` does NOT match the `image/png` allow-listed prefix.
func isInlineSafeMIME(contentType string) bool {
	lower := strings.ToLower(strings.TrimSpace(contentType))
	for _, p := range inlineSafeMIMEPrefixes {
		if lower == p || strings.HasPrefix(lower, p+";") {
			return true
		}
	}
	return false
}

func attachmentToJSON(a *models.Attachment) fiber.Map {
	return fiber.Map{
		"id":             a.ID,
		"context_type":   a.ContextType,
		"context_id":     a.ContextID,
		"folder_id":      a.FolderID,
		"user_id":        a.UserID,
		"display_name":   a.DisplayName,
		"filename":       a.Filename,
		"content_type":   a.ContentType,
		"size":           a.Size,
		"md5":            a.MD5,
		"workflow_state": a.WorkflowState,
		"file_state":     a.FileState,
		"upload_status":  a.UploadStatus,
		"created_at":     a.CreatedAt,
		"updated_at":     a.UpdatedAt,
		"url":            fmt.Sprintf("/api/v1/files/%d/download", a.ID),
	}
}

func (h *FileHandler) ListCourseFiles(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.fileService.ListFilesByContext(c.Context(), "Course", uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch files")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	files := make([]fiber.Map, len(result.Items))
	for i, a := range result.Items {
		files[i] = attachmentToJSON(&a)
	}

	return c.JSON(files)
}

func (h *FileHandler) UploadCourseFile(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
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

	uploaderID, err := getUserID(c)
	if err != nil {
		return err
	}

	attachment := &models.Attachment{
		ContextType: "Course",
		ContextID:   uint(courseID),
		UserID:      uploaderID,
		DisplayName: fileHeader.Filename,
		Filename:    fileHeader.Filename,
		ContentType: fileHeader.Header.Get("Content-Type"),
		Size:        fileHeader.Size,
	}

	// Stamp the caller's account onto ctx so the S3 backend (Wave 6 lookup
	// closure) resolves storage.s3.* at account scope first, falling back to
	// instance/env. Enables per-district S3 buckets on the upload path.
	uploadCtx := settingsctx.WithAccountID(c.Context(), callerAccountID(c))
	if err := h.fileService.UploadFile(uploadCtx, attachment, file); err != nil {
		return responses.InternalError(c, "Could not upload file")
	}

	return c.Status(fiber.StatusCreated).JSON(attachmentToJSON(attachment))
}

func (h *FileHandler) GetFile(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid file ID")
	}

	attachment, err := h.fileService.GetAttachment(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "file")
	}

	if callerID, _ := getUserID(c); callerID != 0 && h.auditService != nil && attachment.UserID != 0 && attachment.UserID != callerID {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, attachment.UserID, "read", "file_metadata", "attachments", attachment.ID, c.IP(), c.Get("User-Agent"))
	}

	return c.JSON(attachmentToJSON(attachment))
}

func (h *FileHandler) DeleteFile(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid file ID")
	}

	if err := h.fileService.DeleteAttachment(c.Context(), uint(id)); err != nil {
		return responses.InternalError(c, "Could not delete file")
	}

	return c.JSON(fiber.Map{"delete": true})
}

func (h *FileHandler) DownloadFile(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid file ID")
	}

	attachment, err := h.fileService.GetAttachment(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "file")
	}

	// SECURITY (F-041): authz default-denies for any context type the
	// switch doesn't recognize. The pre-fix path only checked enrollment
	// for ContextType=="Course"; every other context (User, Group,
	// Account, etc.) fell through to "allow." User-context attachments
	// (personal files, draft uploads) and Group-context attachments
	// (often private to members) were readable by anyone with a JWT.
	userID, _ := c.Locals("user_id").(uint)
	switch attachment.ContextType {
	case "Course":
		enrollment, _ := h.enrollmentRepo.FindByUserAndCourse(c.Context(), userID, attachment.ContextID, callerAccountID(c))
		if enrollment == nil || enrollment.WorkflowState != "active" {
			return responses.NotFound(c, "file")
		}
	case "User":
		// Personal files: only the owner can download. Admins of the
		// same tenant retain access via the GetAttachment tenant gate
		// + is_admin Locals; that combination is intentionally
		// permissive for FERPA / data-export workflows.
		isAdmin, _ := c.Locals("is_admin").(bool)
		if !isAdmin && attachment.ContextID != userID {
			return responses.NotFound(c, "file")
		}
	case "Group":
		// Group files: visible only to members of the group + admins.
		isMember := false
		if h.groupMembershipRepo != nil {
			memberships, _ := h.groupMembershipRepo.ListByGroupID(c.Context(), attachment.ContextID, repository.PaginationParams{Page: 1, PerPage: 10000})
			if memberships != nil {
				for _, m := range memberships.Items {
					if m.UserID == userID && m.WorkflowState == "accepted" {
						isMember = true
						break
					}
				}
			}
		}
		isAdmin, _ := c.Locals("is_admin").(bool)
		if !isMember && !isAdmin {
			return responses.NotFound(c, "file")
		}
	case "Account":
		// Account-context attachments are admin-only by convention.
		isAdmin, _ := c.Locals("is_admin").(bool)
		if !isAdmin {
			return responses.NotFound(c, "file")
		}
	default:
		// Unknown context type — fail closed. Adding a new context
		// type now requires explicit authz wiring here.
		return responses.NotFound(c, "file")
	}

	if callerID, _ := getUserID(c); callerID != 0 && h.auditService != nil && attachment.UserID != 0 && attachment.UserID != callerID {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, attachment.UserID, "download", "file_download", "attachments", attachment.ID, c.IP(), c.Get("User-Agent"))
	}

	// Sanitize filename for Content-Disposition to prevent header injection
	safeName := filepath.Base(attachment.Filename)
	safeName = strings.ReplaceAll(safeName, "\"", "")
	safeName = strings.ReplaceAll(safeName, "\r", "")
	safeName = strings.ReplaceAll(safeName, "\n", "")

	// SECURITY (F-040): serve inline ONLY for an explicit allow-list of
	// MIME prefixes that are NOT executable in a browser. The pre-fix
	// path served any `image/*` inline — and the upload-time
	// `Content-Type` header is attacker-controlled, so an attacker
	// could upload `payload.bogus` with header `Content-Type:
	// image/svg+xml` (unknown extension skips MIME validation) and
	// trigger script execution in the LMS origin on download. SVG is
	// the canonical XSS vehicle here; PDFs can also embed scripts in
	// some viewers, so neither is allow-listed for inline.
	disposition := "attachment"
	if isInlineSafeMIME(attachment.ContentType) {
		disposition = "inline"
	}
	c.Set("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"; filename*=UTF-8''%s", disposition, safeName, url.PathEscape(safeName)))

	backend := h.fileService.StorageBackend()

	// For S3 backend, redirect to presigned URL instead of proxying
	if _, isS3 := backend.(*storage.S3Backend); isS3 {
		downloadURL, err := h.fileService.GetFileURL(c.Context(), uint(id), callerAccountID(c))
		if err != nil {
			return responses.InternalError(c, "Could not generate download URL")
		}
		return c.Redirect(downloadURL, fiber.StatusTemporaryRedirect)
	}

	// For local backend, stream the file directly. Stamp account on ctx so
	// the storage backend's per-tenant config lookup (Wave 6) sees the
	// caller; the local backend ignores ctx, S3 would resolve per-tenant.
	downloadCtx := settingsctx.WithAccountID(c.Context(), callerAccountID(c))
	reader, err := backend.Get(downloadCtx, attachment.StoragePath)
	if err != nil {
		return responses.InternalError(c, "Could not locate file")
	}
	defer reader.Close()

	c.Set("Content-Type", attachment.ContentType)
	return c.SendStream(reader)
}

func (h *FileHandler) ListFolderFiles(c *fiber.Ctx) error {
	folderID, err := c.ParamsInt("folder_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid folder ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.fileService.ListFilesByFolder(c.Context(), uint(folderID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch files")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	files := make([]fiber.Map, len(result.Items))
	for i, a := range result.Items {
		files[i] = attachmentToJSON(&a)
	}

	return c.JSON(files)
}
