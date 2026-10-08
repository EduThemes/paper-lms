package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
	// authz supplies the DB-backed admin role and the course gate used by
	// canReadContextFiles. Nil means no admin bypass.
	authz *ResourceAuthorizer
	// quotas backs course storage quotas (G072 / G120); see SetQuota.
	// Nil leaves quotas unenforced here.
	quotas CourseStorageQuotas
}

// SetAuthorizer wires the DB-backed role / course checks.
func (h *FileHandler) SetAuthorizer(a *ResourceAuthorizer) *FileHandler {
	h.authz = a
	return h
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
		// G070 visibility. "locked" is Canvas's unpublished flag.
		"locked":       a.Locked,
		"hidden":       a.Hidden,
		"lock_at":      a.LockAt,
		"unlock_at":    a.UnlockAt,
		"usage_rights": usageRightsJSON(a),
	}
}

func (h *FileHandler) ListCourseFiles(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	var result *repository.PaginatedResult[models.Attachment]
	learner := !canViewUnpublishedContent(c)
	if h.fileService.HasTree() {
		// G070/G071: ?folder_id=root|<id> lists one folder; learners
		// never see unpublished or hidden files, or anything under an
		// unpublished or hidden folder.
		q, wrote, qerr := h.courseTreeQuery(c, uint(courseID), c.Query("folder_id"), learner)
		if wrote {
			return qerr
		}
		result, err = h.fileService.ListTreeFiles(c.Context(), q, params)
	} else {
		result, err = h.fileService.ListFilesByContext(c.Context(), "Course", uint(courseID), params)
	}
	if err != nil {
		return responses.InternalError(c, "Could not fetch files")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	files := make([]fiber.Map, len(result.Items))
	for i, a := range result.Items {
		files[i] = fileJSONForCaller(&a, learner)
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

	// SECURITY: don't trust the client Content-Type for the inline-render
	// decision. Sniff the magic bytes; if the file CLAIMS an inline-safe
	// image type but its actual bytes positively identify as something else
	// (e.g. an SVG/HTML document uploaded as a .png with
	// Content-Type: image/png), store the sniffed type so the download path
	// serves it as an attachment, never inline. Unrecognized types
	// (application/octet-stream — covers AVIF/HEIC the stdlib can't sniff)
	// keep the client claim so legitimate modern images still render inline.
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
		ContextType: "Course",
		ContextID:   uint(courseID),
		UserID:      uploaderID,
		DisplayName: fileHeader.Filename,
		Filename:    fileHeader.Filename,
		ContentType: contentType,
		Size:        fileHeader.Size,
	}

	// B138: honor the destination folder. The folder must be a live
	// folder of THIS course (tenant-scoped load + context tie), otherwise
	// 404 — a foreign folder id must not let a file land in another
	// course's tree.
	if raw := strings.TrimSpace(c.FormValue("folder_id")); raw != "" {
		fid, convErr := strconv.ParseUint(raw, 10, 64)
		if convErr != nil || fid == 0 {
			return responses.BadRequest(c, "Invalid folder_id")
		}
		folder, ferr := h.fileService.GetFolder(c.Context(), uint(fid), callerAccountID(c))
		if ferr != nil || folder == nil || folder.ContextType != "Course" || folder.ContextID != uint(courseID) || folder.WorkflowState == "deleted" {
			return responses.NotFound(c, "folder")
		}
		folderID := folder.ID
		attachment.FolderID = &folderID
	}

	// Stamp the caller's account onto ctx so the S3 backend (Wave 6 lookup
	// closure) resolves storage.s3.* at account scope first, falling back to
	// instance/env. Enables per-district S3 buckets on the upload path.
	uploadCtx := settingsctx.WithAccountID(c.Context(), callerAccountID(c))

	// G072: refuse uploads that would take the course over its quota.
	if wrote, qerr := h.enforceCourseQuota(c, uploadCtx, uint(courseID), fileHeader.Size, 0); wrote {
		return qerr
	}

	if err := h.fileService.UploadFile(uploadCtx, attachment, file); err != nil {
		if errors.Is(err, service.ErrStorageQuotaExceeded) {
			return responses.BadRequest(c, "Course storage quota exceeded")
		}
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

	// G070: an unpublished file (or one in an unpublished folder) is
	// invisible to learners; a date-locked one is described but marked
	// locked_for_user.
	learner := false
	if attachment.ContextType == "Course" {
		// The route carries :course_id; tie the file to it (F-013).
		if courseID, perr := c.ParamsInt("course_id"); perr == nil && attachment.ContextID != uint(courseID) {
			return responses.NotFound(c, "file")
		}
		learner = !h.managesCourseFiles(c, attachment.ContextID)
		if learner && h.learnerCannotSeeFile(c, attachment) {
			return responses.NotFound(c, "file")
		}
	}

	if callerID, _ := getUserID(c); callerID != 0 && h.auditService != nil && attachment.UserID != 0 && attachment.UserID != callerID {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, attachment.UserID, "read", "file_metadata", "attachments", attachment.ID, c.IP(), c.Get("User-Agent"))
	}

	return c.JSON(fileJSONForCaller(attachment, learner))
}

func (h *FileHandler) DeleteFile(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid file ID")
	}

	// F-043: load under the caller's tenant + verify the attachment is
	// in the URL's :course_id BEFORE delete. Pre-fix:
	//   DELETE /courses/10/files/<id from any course in any tenant>
	// soft-deleted the cross-context attachment because the service
	// hit a raw `WHERE id = ?` with no scope.
	attachment, err := h.fileService.GetAttachment(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "file")
	}
	if attachment.ContextType != "Course" || attachment.ContextID != uint(courseID) {
		return responses.NotFound(c, "file")
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
	if !h.canReadContextFiles(c, attachment.ContextType, attachment.ContextID) {
		return responses.NotFound(c, "file")
	}

	// G070: learners can't fetch an unpublished file (or one in an
	// unpublished folder), nor one outside its availability window.
	// Hidden files stay downloadable by link.
	if attachment.ContextType == "Course" && !h.managesCourseFiles(c, attachment.ContextID) {
		if h.learnerCannotSeeFile(c, attachment) {
			return responses.NotFound(c, "file")
		}
		if attachment.LockedForLearnerAt(time.Now()) {
			return responses.Forbidden(c, "This file is locked")
		}
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
	// No defer Close: fasthttp streams the body after the handler returns
	// and closes the reader itself (io.Closer). Closing it here sent an
	// empty/failed download from local storage.

	c.Set("Content-Type", attachment.ContentType)
	return c.SendStream(reader)
}

// canReadContextFiles reports whether the caller may read files that
// live in the (contextType, contextID) context. The tenant check is the
// caller's job (load the attachment/folder with callerAccountID first);
// this adds the per-context membership rule and default-denies unknown
// context types.
func (h *FileHandler) canReadContextFiles(c *fiber.Ctx, contextType string, contextID uint) bool {
	userID, _ := c.Locals("user_id").(uint)
	// DB-backed role (masquerade-capped). The JWT is_admin Locals is only
	// a hint: these routes have no permission gate to refresh it, so a
	// demoted admin's unexpired token would still carry it.
	isAdmin := callerIsAdmin(c, h.authz)
	switch contextType {
	case "Course":
		// Same rule as the course routes: same-tenant admins, or an active
		// enrollment — and students/observers are refused while the course
		// is unpublished.
		if h.authz != nil {
			return h.authz.RequireCourseEnrolled(c, contextID) == nil
		}
		enrollment, _ := h.enrollmentRepo.FindByUserAndCourse(c.Context(), userID, contextID, callerAccountID(c))
		return enrollment != nil && enrollment.WorkflowState == "active"
	case "User":
		// Personal files: only the owner. Admins of the same tenant
		// retain access via the tenant gate + the DB admin role; that
		// combination is intentionally permissive for FERPA /
		// data-export workflows.
		return isAdmin || contextID == userID
	case "Group":
		// Group files: visible only to members of the group + admins.
		if isAdmin {
			return true
		}
		if h.groupMembershipRepo != nil {
			memberships, _ := h.groupMembershipRepo.ListByGroupID(c.Context(), contextID, repository.PaginationParams{Page: 1, PerPage: 10000})
			if memberships != nil {
				for _, m := range memberships.Items {
					if m.UserID == userID && m.WorkflowState == "accepted" {
						return true
					}
				}
			}
		}
		return false
	case "Account":
		// Account-context attachments are admin-only by convention.
		return isAdmin
	default:
		// Unknown context type — fail closed. Adding a new context
		// type now requires explicit authz wiring here.
		return false
	}
}

func (h *FileHandler) ListFolderFiles(c *fiber.Ctx) error {
	folderID, err := c.ParamsInt("folder_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid folder ID")
	}

	// B046: the route has no course middleware, so authorize here. Load
	// the folder under the caller's tenant (cross-tenant → 404), then
	// apply the same per-context read rule as file download. Pre-fix any
	// authenticated user could enumerate any folder's file metadata.
	folder, err := h.fileService.GetFolder(c.Context(), uint(folderID), callerAccountID(c))
	if err != nil || folder == nil || folder.WorkflowState == "deleted" {
		return responses.NotFound(c, "folder")
	}
	if !h.canReadContextFiles(c, folder.ContextType, folder.ContextID) {
		return responses.NotFound(c, "folder")
	}

	params := middleware.GetPagination(c)

	var result *repository.PaginatedResult[models.Attachment]
	learner := folder.ContextType == "Course" && !h.managesCourseFiles(c, folder.ContextID)
	if h.fileService.HasTree() && folder.ContextType == "Course" {
		// G070: a learner can't browse into an unpublished or hidden
		// folder, and sees only the published, unhidden files in it.
		if learner {
			if blocked, berr := h.fileService.FolderBlockedForLearner(c.Context(), folder.ID, false); berr != nil || blocked {
				return responses.NotFound(c, "folder")
			}
		}
		result, err = h.fileService.ListTreeFiles(c.Context(), repository.FileTreeQuery{
			ContextType: folder.ContextType, ContextID: folder.ContextID,
			Scope: repository.FolderScopeFolder, FolderID: folder.ID, LearnerView: learner,
		}, params)
	} else {
		result, err = h.fileService.ListFilesByFolder(c.Context(), uint(folderID), params)
	}
	if err != nil {
		return responses.InternalError(c, "Could not fetch files")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	files := make([]fiber.Map, len(result.Items))
	for i, a := range result.Items {
		files[i] = fileJSONForCaller(&a, learner)
	}

	return c.JSON(files)
}
