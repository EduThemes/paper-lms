package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/settingsctx"
)

// SetQuota wires course storage quotas (G072): the course's own quota
// or the account default, both in storage_quotas (G120).
func (h *FileHandler) SetQuota(quotas CourseStorageQuotas) *FileHandler {
	h.quotas = quotas
	return h
}

func usageRightsJSON(a *models.Attachment) fiber.Map {
	if a.UsageRightsJustification == "" {
		return nil
	}
	return fiber.Map{
		"use_justification": a.UsageRightsJustification,
		"legal_copyright":   a.UsageRightsLegalCopyright,
		"license":           a.UsageRightsLicense,
	}
}

// fileJSONForCaller adds locked_for_user for learners (Canvas): true while
// the file is outside its availability window.
func fileJSONForCaller(a *models.Attachment, learner bool) fiber.Map {
	out := attachmentToJSON(a)
	out["locked_for_user"] = learner && a.LockedForLearnerAt(time.Now())
	return out
}

// managesCourseFiles reports whether the caller may see every file of the
// course (teachers, TAs, designers, admins). On course routes the
// permission middleware has already set enrollment_type; elsewhere (e.g.
// /files/:id/download) the DB-backed role and enrollment are checked.
func (h *FileHandler) managesCourseFiles(c *fiber.Ctx, courseID uint) bool {
	if et, ok := c.Locals("enrollment_type").(string); ok && et != "" {
		return canViewUnpublishedContent(c)
	}
	if h.authz != nil {
		return courseContentManager(c, h.authz, courseID)
	}
	return canViewUnpublishedContent(c)
}

// courseContentManager is canViewUnpublishedContent for routes that carry
// no course middleware (/files/:id/download, /folders/:id): it asks the
// database whether the caller is an admin of the course's tenant or holds
// an active teacher, TA or designer enrollment. Designers count here as
// they do on course routes, or they would see an unpublished file in the
// course listing and then get a 404 downloading it.
func courseContentManager(c *fiber.Ctx, a *ResourceAuthorizer, courseID uint) bool {
	if a == nil || a.RequireCourseEnrolled(c, courseID) != nil {
		return false
	}
	if callerIsAdmin(c, a) {
		return true
	}
	userID, _ := c.Locals("user_id").(uint)
	e, err := a.enrollmentRepo.FindByUserAndCourse(c.Context(), userID, courseID, callerAccountID(c))
	if err != nil || e == nil || e.WorkflowState != "active" {
		return false
	}
	switch e.Type {
	case "TeacherEnrollment", "TaEnrollment", "DesignerEnrollment":
		return true
	}
	return false
}

// learnerCannotSeeFile: the file is unpublished, or sits in a deleted or
// unpublished folder. (Hidden files and folders stay readable by link.)
func (h *FileHandler) learnerCannotSeeFile(c *fiber.Ctx, a *models.Attachment) bool {
	if a.Locked {
		return true
	}
	if a.FolderID != nil && h.fileService.HasTree() {
		blocked, err := h.fileService.FolderBlockedForLearner(c.Context(), *a.FolderID, true)
		return err != nil || blocked
	}
	return false
}

// courseTreeQuery parses ?folder_id (root | <id> | empty = all) for a
// course listing. A folder id must name a live folder of the course; a
// learner may not list an unpublished or hidden one. Returns wrote=true
// after writing an error response.
func (h *FileHandler) courseTreeQuery(c *fiber.Ctx, courseID uint, raw string, learner bool) (repository.FileTreeQuery, bool, error) {
	q := repository.FileTreeQuery{ContextType: "Course", ContextID: courseID, LearnerView: learner}
	switch raw = strings.TrimSpace(raw); raw {
	case "":
		q.Scope = repository.FolderScopeAll
	case "root":
		q.Scope = repository.FolderScopeRoot
		if root, err := h.fileService.FindRootFolder(c.Context(), "Course", courseID); err == nil && root != nil {
			q.RootFolderID = root.ID
		}
	default:
		fid, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || fid == 0 {
			return q, true, responses.BadRequest(c, "Invalid folder_id")
		}
		folder, err := h.fileService.GetFolder(c.Context(), uint(fid), callerAccountID(c))
		if err != nil || folder == nil || folder.ContextType != "Course" || folder.ContextID != courseID || folder.WorkflowState == "deleted" {
			return q, true, responses.NotFound(c, "folder")
		}
		if learner {
			if blocked, berr := h.fileService.FolderBlockedForLearner(c.Context(), folder.ID, false); berr != nil || blocked {
				return q, true, responses.NotFound(c, "folder")
			}
		}
		q.Scope = repository.FolderScopeFolder
		q.FolderID = folder.ID
	}
	return q, false, nil
}

// quotaScope is the tenant a quota lookup is confined to (0 for a
// super_admin, whose role crosses tenants by design).
func quotaScope(c *fiber.Ctx) uint {
	if isSuper, _ := c.Locals("is_super_admin").(bool); isSuper {
		return 0
	}
	return callerAccountID(c)
}

// enforceCourseQuota writes a 400 when adding addBytes (and freeing
// freeBytes, for a replacement) would exceed the course quota. Returns
// wrote=true then.
func (h *FileHandler) enforceCourseQuota(c *fiber.Ctx, ctx context.Context, courseID uint, addBytes, freeBytes int64) (bool, error) {
	if h.quotas == nil {
		return false, nil
	}
	err := h.quotas.CheckQuota(ctx, "Course", courseID, addBytes-freeBytes)
	if errors.Is(err, service.ErrStorageQuotaExceeded) {
		return true, responses.BadRequest(c, "Course storage quota exceeded")
	}
	if err != nil {
		return true, responses.InternalError(c, "Could not check storage quota")
	}
	return false, nil
}

// GetCourseQuota handles GET /courses/:course_id/files/quota (Canvas
// files/quota): {quota, quota_used} in bytes.
func (h *FileHandler) GetCourseQuota(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	if h.quotas == nil {
		used, err := h.fileService.ContextUsage(c.Context(), "Course", uint(courseID))
		if err != nil {
			return responses.InternalError(c, "Could not compute storage usage")
		}
		return c.JSON(fiber.Map{"quota": 0, "quota_used": used})
	}
	info, err := h.quotas.Info(c.Context(), "Course", uint(courseID), quotaScope(c))
	if err != nil {
		return responses.NotFound(c, "course")
	}
	// Canvas files/quota: {quota, quota_used} in bytes, plus quota_mb and
	// whether the course has its own quota.
	return c.JSON(info)
}

// loadCourseFile loads a live attachment of the course (tenant-scoped,
// course-tied). Returns wrote=true after writing a 404.
func (h *FileHandler) loadCourseFile(c *fiber.Ctx, courseID uint) (*models.Attachment, bool, error) {
	id, err := c.ParamsInt("id")
	if err != nil {
		return nil, true, responses.BadRequest(c, "Invalid file ID")
	}
	a, err := h.fileService.GetAttachment(c.Context(), uint(id), callerAccountID(c))
	if err != nil || a == nil || a.ContextType != "Course" || a.ContextID != courseID {
		return nil, true, responses.NotFound(c, "file")
	}
	return a, false, nil
}

// UpdateCourseFile handles PUT /courses/:course_id/files/:id (instructor):
// rename (display_name), move (parent_folder_id / folder_id), visibility
// (locked, hidden, lock_at, unlock_at) and usage rights (G070 / G071).
// on_duplicate=rename appends a counter when the name is taken in the
// target folder; otherwise a clash is a 409.
func (h *FileHandler) UpdateCourseFile(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	a, wrote, err := h.loadCourseFile(c, uint(courseID))
	if wrote {
		return err
	}

	var input struct {
		DisplayName    *string         `json:"display_name"`
		Name           *string         `json:"name"`
		ParentFolderID json.RawMessage `json:"parent_folder_id"`
		FolderID       json.RawMessage `json:"folder_id"`
		Locked         *bool           `json:"locked"`
		Hidden         *bool           `json:"hidden"`
		LockAt         json.RawMessage `json:"lock_at"`
		UnlockAt       json.RawMessage `json:"unlock_at"`
		OnDuplicate    string          `json:"on_duplicate"`
		UsageRights    *struct {
			UseJustification string `json:"use_justification"`
			LegalCopyright   string `json:"legal_copyright"`
			License          string `json:"license"`
		} `json:"usage_rights"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	name := a.DisplayName
	if input.Name != nil && input.DisplayName == nil {
		input.DisplayName = input.Name
	}
	if input.DisplayName != nil {
		name = strings.TrimSpace(service.SanitizePlainText(*input.DisplayName))
		if name == "" || strings.ContainsAny(name, "/\\") || name == "." || name == ".." {
			return responses.BadRequest(c, "Invalid file name")
		}
		if len(name) > 255 {
			return responses.BadRequest(c, "File name is too long")
		}
	}

	folderID := a.FolderID
	rawFolder := input.ParentFolderID
	if len(rawFolder) == 0 {
		rawFolder = input.FolderID
	}
	if len(rawFolder) > 0 {
		target, wrote, ferr := h.resolveTargetFolder(c, uint(courseID), rawFolder)
		if wrote {
			return ferr
		}
		folderID = target
	}

	if input.DisplayName != nil || len(rawFolder) > 0 {
		taken, terr := h.fileService.NameTaken(c.Context(), a, folderID, name)
		if terr != nil {
			return responses.InternalError(c, "Could not check file name")
		}
		if taken {
			if input.OnDuplicate != "rename" {
				return responses.Error(c, http.StatusConflict, "A file named \""+name+"\" already exists in that folder")
			}
			name, terr = h.uniqueFileName(c, a, folderID, name)
			if terr != nil {
				return responses.InternalError(c, "Could not check file name")
			}
		}
	}

	if input.Locked != nil {
		a.Locked = *input.Locked
	}
	if input.Hidden != nil {
		a.Hidden = *input.Hidden
	}
	if set, t, perr := parseOptionalTimeJSON(input.LockAt); perr != nil {
		return responses.BadRequest(c, "Invalid lock_at")
	} else if set {
		a.LockAt = t
	}
	if set, t, perr := parseOptionalTimeJSON(input.UnlockAt); perr != nil {
		return responses.BadRequest(c, "Invalid unlock_at")
	} else if set {
		a.UnlockAt = t
	}
	if a.LockAt != nil && a.UnlockAt != nil && !a.UnlockAt.Before(*a.LockAt) {
		return responses.BadRequest(c, "unlock_at must be before lock_at")
	}
	if input.UsageRights != nil {
		ur := input.UsageRights
		if !models.ValidUsageRightsJustification(ur.UseJustification) {
			return responses.BadRequest(c, "Invalid use_justification")
		}
		a.UsageRightsJustification = ur.UseJustification
		a.UsageRightsLegalCopyright = strings.TrimSpace(service.SanitizePlainText(ur.LegalCopyright))
		a.UsageRightsLicense = strings.TrimSpace(service.SanitizePlainText(ur.License))
		if ur.UseJustification == "" {
			a.UsageRightsLegalCopyright, a.UsageRightsLicense = "", ""
		}
	}

	a.DisplayName = name
	a.FolderID = folderID
	if err := h.fileService.UpdateAttachment(c.Context(), a); err != nil {
		return responses.InternalError(c, "Could not update file")
	}
	return c.JSON(attachmentToJSON(a))
}

// resolveTargetFolder parses a folder reference for a move: null or
// "root" means the course root, a number must be a live folder of the
// course.
func (h *FileHandler) resolveTargetFolder(c *fiber.Ctx, courseID uint, raw json.RawMessage) (*uint, bool, error) {
	s := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	if s == "null" || s == "" || s == "root" {
		return nil, false, nil
	}
	fid, err := strconv.ParseUint(s, 10, 64)
	if err != nil || fid == 0 {
		return nil, true, responses.BadRequest(c, "Invalid folder")
	}
	folder, err := h.fileService.GetFolder(c.Context(), uint(fid), callerAccountID(c))
	if err != nil || folder == nil || folder.ContextType != "Course" || folder.ContextID != courseID || folder.WorkflowState == "deleted" {
		return nil, true, responses.NotFound(c, "folder")
	}
	id := folder.ID
	return &id, false, nil
}

// uniqueFileName appends " (n)" before the extension until the name is
// free in the folder.
func (h *FileHandler) uniqueFileName(c *fiber.Ctx, a *models.Attachment, folderID *uint, name string) (string, error) {
	base, ext := name, ""
	if dot := strings.LastIndex(name, "."); dot > 0 {
		base, ext = name[:dot], name[dot:]
	}
	for n := 1; n < 1000; n++ {
		candidate := base + " (" + strconv.Itoa(n) + ")" + ext
		taken, err := h.fileService.NameTaken(c.Context(), a, folderID, candidate)
		if err != nil {
			return "", err
		}
		if !taken {
			return candidate, nil
		}
	}
	return "", errors.New("no free name")
}

// ReplaceCourseFile handles POST /courses/:course_id/files/:id/replace
// (instructor, multipart "file"): new content under the same file id, so
// links, embeds and module items keep pointing at it (G071).
func (h *FileHandler) ReplaceCourseFile(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	a, wrote, err := h.loadCourseFile(c, uint(courseID))
	if wrote {
		return err
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return responses.BadRequest(c, "File is required")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return responses.InternalError(c, "Could not open uploaded file")
	}
	defer func() { _ = file.Close() }()

	// Same content-type rule as a fresh upload: a file that claims an
	// inline-safe image type but sniffs as something else is stored with
	// the sniffed type, so it is never served inline.
	contentType := fileHeader.Header.Get("Content-Type")
	sniffBuf := make([]byte, 512)
	n, _ := io.ReadFull(file, sniffBuf)
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return responses.InternalError(c, "Could not read uploaded file")
	}
	if n > 0 {
		sniffed := http.DetectContentType(sniffBuf[:n])
		if isInlineSafeMIME(contentType) && !isInlineSafeMIME(sniffed) && !strings.HasPrefix(sniffed, "application/octet-stream") {
			contentType = sniffed
		}
	}

	ctx := settingsctx.WithAccountID(c.Context(), callerAccountID(c))
	if wrote, qerr := h.enforceCourseQuota(c, ctx, uint(courseID), fileHeader.Size, a.Size); wrote {
		return qerr
	}
	if err := h.fileService.ReplaceContent(ctx, a, fileHeader.Filename, contentType, fileHeader.Size, file); err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.JSON(attachmentToJSON(a))
}
