package handlers

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/gofiber/fiber/v2"
)

type FolderHandler struct {
	fileService *service.FileService
	authz       *ResourceAuthorizer
}

func NewFolderHandler(fileService *service.FileService, authz *ResourceAuthorizer) *FolderHandler {
	return &FolderHandler{fileService: fileService, authz: authz}
}

func folderToJSON(f *models.Folder) fiber.Map {
	return fiber.Map{
		"id":               f.ID,
		"context_type":     f.ContextType,
		"context_id":       f.ContextID,
		"parent_folder_id": f.ParentFolderID,
		"name":             f.Name,
		"full_name":        f.FullName,
		"position":         f.Position,
		"workflow_state":   f.WorkflowState,
		"locked":           f.Locked,
		"hidden":           f.Hidden,
		"created_at":       f.CreatedAt,
		"updated_at":       f.UpdatedAt,
	}
}

// validFolderName trims and checks a folder name: required, no path
// separators, at most 255 characters. Plain text (SanitizePlainText).
func validFolderName(raw string) (string, bool) {
	name := strings.TrimSpace(service.SanitizePlainText(raw))
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") || len(name) > 255 {
		return "", false
	}
	return name, true
}

// managesFolderCourse reports whether the caller sees every folder of the
// course. Course routes carry enrollment_type from the permission
// middleware; /folders/:id routes fall back to the DB-backed checks.
func (h *FolderHandler) managesFolderCourse(c *fiber.Ctx, courseID uint) bool {
	if et, ok := c.Locals("enrollment_type").(string); ok && et != "" {
		return canViewUnpublishedContent(c)
	}
	if callerIsAdmin(c, h.authz) {
		return true
	}
	if h.authz != nil {
		return h.authz.RequireCourseInstructor(c, courseID) == nil
	}
	return false
}

// learnerBlockedFolder: a learner may not open a deleted, unpublished or
// hidden folder (or one below such a folder).
func (h *FolderHandler) learnerBlockedFolder(c *fiber.Ctx, folder *models.Folder) bool {
	if folder.WorkflowState == "deleted" || folder.Locked || folder.Hidden {
		return true
	}
	if h.fileService.HasTree() {
		blocked, err := h.fileService.FolderBlockedForLearner(c.Context(), folder.ID, false)
		return err != nil || blocked
	}
	return false
}

// ListCourseFolders handles GET /courses/:course_id/folders. With no
// parameter it lists every folder of the course (flat; the client builds
// the tree). ?parent_folder_id=root|<id> lists one level (G071).
// Learners never see unpublished or hidden folders (G070).
func (h *FolderHandler) ListCourseFolders(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)
	learner := !canViewUnpublishedContent(c)

	var result *repository.PaginatedResult[models.Folder]
	if h.fileService.HasTree() {
		q := repository.FileTreeQuery{ContextType: "Course", ContextID: uint(courseID), LearnerView: learner}
		switch raw := strings.TrimSpace(c.Query("parent_folder_id")); raw {
		case "":
			q.Scope = repository.FolderScopeAll
		case "root":
			q.Scope = repository.FolderScopeRoot
			if root, rerr := h.fileService.FindRootFolder(c.Context(), "Course", uint(courseID)); rerr == nil && root != nil {
				q.RootFolderID = root.ID
			}
		default:
			fid, perr := strconv.ParseUint(raw, 10, 64)
			if perr != nil || fid == 0 {
				return responses.BadRequest(c, "Invalid parent_folder_id")
			}
			parent, ferr := h.fileService.GetFolder(c.Context(), uint(fid), callerAccountID(c))
			if ferr != nil || parent == nil || parent.ContextType != "Course" || parent.ContextID != uint(courseID) || parent.WorkflowState == "deleted" {
				return responses.NotFound(c, "folder")
			}
			if learner && h.learnerBlockedFolder(c, parent) {
				return responses.NotFound(c, "folder")
			}
			q.Scope = repository.FolderScopeFolder
			q.FolderID = parent.ID
		}
		result, err = h.fileService.ListTreeFolders(c.Context(), q, params)
	} else {
		result, err = h.fileService.ListFolders(c.Context(), "Course", uint(courseID), nil, params)
	}
	if err != nil {
		return responses.InternalError(c, "Could not fetch folders")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	folders := make([]fiber.Map, len(result.Items))
	for i, f := range result.Items {
		folders[i] = folderToJSON(&f)
	}

	return c.JSON(folders)
}

func (h *FolderHandler) CreateCourseFolder(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		Name           string `json:"name"`
		ParentFolderID *uint  `json:"parent_folder_id"`
		Locked         bool   `json:"locked"`
		Hidden         bool   `json:"hidden"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	name, ok := validFolderName(input.Name)
	if !ok {
		return responses.BadRequest(c, "Invalid folder name")
	}

	parentFolderID := input.ParentFolderID
	if parentFolderID == nil {
		root, err := h.fileService.GetOrCreateRootFolder(c.Context(), "Course", uint(courseID))
		if err != nil {
			return responses.InternalError(c, "Could not get root folder")
		}
		parentFolderID = &root.ID
	}

	parent, err := h.fileService.GetFolder(c.Context(), *parentFolderID, callerAccountID(c))
	// The parent must be a live folder of THIS course. The tenant-scoped
	// load alone let a teacher of course A hang a folder under course B's
	// tree (it then showed up in B's subfolder listing).
	if err != nil || parent == nil || parent.ContextType != "Course" || parent.ContextID != uint(courseID) || parent.WorkflowState == "deleted" {
		return responses.NotFound(c, "parent folder")
	}

	folder := &models.Folder{
		ContextType:    "Course",
		ContextID:      uint(courseID),
		ParentFolderID: parentFolderID,
		Name:           name,
		FullName:       parent.FullName + "/" + name,
		WorkflowState:  "visible",
		Locked:         input.Locked,
		Hidden:         input.Hidden,
	}

	if err := h.fileService.CreateFolder(c.Context(), folder); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(folderToJSON(folder))
}

func (h *FolderHandler) GetFolder(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid folder ID")
	}

	folder, err := h.fileService.GetFolder(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "folder")
	}

	if folder.ContextType == "Course" {
		if err := h.authz.RequireCourseEnrolled(c, folder.ContextID); err != nil {
			return err
		}
		if !h.managesFolderCourse(c, folder.ContextID) && h.learnerBlockedFolder(c, folder) {
			return responses.NotFound(c, "folder")
		}
	}

	return c.JSON(folderToJSON(folder))
}

// UpdateFolder handles PUT /folders/:id: rename (name), move
// (parent_folder_id), publish state (locked) and hidden (G070 / G071).
// A rename or move refreshes full_name for the whole subtree. The course
// root folder can't be moved, and a folder can't move into itself or
// below itself.
func (h *FolderHandler) UpdateFolder(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid folder ID")
	}

	folder, err := h.fileService.GetFolder(c.Context(), uint(id), callerAccountID(c))
	if err != nil || folder.WorkflowState == "deleted" {
		return responses.NotFound(c, "folder")
	}

	if folder.ContextType == "Course" {
		if err := h.authz.RequireCourseInstructor(c, folder.ContextID); err != nil {
			return err
		}
	} else if !h.canManageNonCourseFolder(c, folder) {
		return responses.NotFound(c, "folder")
	}

	var input struct {
		Name           *string         `json:"name"`
		Position       *int            `json:"position"`
		ParentFolderID json.RawMessage `json:"parent_folder_id"`
		Locked         *bool           `json:"locked"`
		Hidden         *bool           `json:"hidden"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	pathChanged := false
	if input.Name != nil {
		name, ok := validFolderName(*input.Name)
		if !ok {
			return responses.BadRequest(c, "Invalid folder name")
		}
		pathChanged = pathChanged || name != folder.Name
		folder.Name = name
	}
	if len(input.ParentFolderID) > 0 {
		newParent, wrote, perr := h.resolveMoveTarget(c, folder, input.ParentFolderID)
		if wrote {
			return perr
		}
		if newParent != nil && (folder.ParentFolderID == nil || *folder.ParentFolderID != *newParent) {
			folder.ParentFolderID = newParent
			pathChanged = true
		}
	}
	if input.Position != nil {
		folder.Position = *input.Position
	}
	if input.Locked != nil {
		folder.Locked = *input.Locked
	}
	if input.Hidden != nil {
		folder.Hidden = *input.Hidden
	}

	if pathChanged {
		folder.FullName = folder.Name
		if folder.ParentFolderID != nil {
			parent, perr := h.fileService.GetFolder(c.Context(), *folder.ParentFolderID, callerAccountID(c))
			if perr != nil || parent == nil {
				return responses.NotFound(c, "parent folder")
			}
			folder.FullName = parent.FullName + "/" + folder.Name
		}
	}

	if err := h.fileService.SaveFolder(c.Context(), folder); err != nil {
		return responses.InternalError(c, "Could not update folder")
	}

	return c.JSON(folderToJSON(folder))
}

// resolveMoveTarget validates a folder move. Returns the new parent id
// (nil = unchanged) or wrote=true after writing an error.
func (h *FolderHandler) resolveMoveTarget(c *fiber.Ctx, folder *models.Folder, raw json.RawMessage) (*uint, bool, error) {
	if folder.ParentFolderID == nil {
		return nil, true, responses.BadRequest(c, "The root folder can't be moved")
	}
	s := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	var target *models.Folder
	if s == "root" || s == "null" || s == "" {
		root, err := h.fileService.FindRootFolder(c.Context(), folder.ContextType, folder.ContextID)
		if err != nil || root == nil {
			return nil, true, responses.NotFound(c, "folder")
		}
		target = root
	} else {
		pid, err := strconv.ParseUint(s, 10, 64)
		if err != nil || pid == 0 {
			return nil, true, responses.BadRequest(c, "Invalid parent_folder_id")
		}
		target, err = h.fileService.GetFolder(c.Context(), uint(pid), callerAccountID(c))
		if err != nil || target == nil {
			return nil, true, responses.NotFound(c, "folder")
		}
	}
	if target.ContextType != folder.ContextType || target.ContextID != folder.ContextID || target.WorkflowState == "deleted" {
		return nil, true, responses.NotFound(c, "folder")
	}
	if target.ID == folder.ID {
		return nil, true, responses.BadRequest(c, "A folder can't be moved into itself")
	}
	if h.fileService.HasTree() {
		within, err := h.fileService.FolderIsWithin(c.Context(), target.ID, folder.ID)
		if err != nil {
			return nil, true, responses.InternalError(c, "Could not check folder")
		}
		if within {
			return nil, true, responses.BadRequest(c, "A folder can't be moved into one of its own subfolders")
		}
	}
	id := target.ID
	return &id, false, nil
}

// DeleteFolder soft-deletes the folder with everything inside it (Canvas
// deletes a folder's contents too). The course root can't be deleted.
func (h *FolderHandler) DeleteFolder(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid folder ID")
	}

	folder, err := h.fileService.GetFolder(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "folder")
	}

	if folder.ContextType == "Course" {
		if err := h.authz.RequireCourseInstructor(c, folder.ContextID); err != nil {
			return err
		}
		if folder.ParentFolderID == nil {
			return responses.BadRequest(c, "The root folder can't be deleted")
		}
	} else if !h.canManageNonCourseFolder(c, folder) {
		return responses.NotFound(c, "folder")
	}

	if err := h.fileService.DeleteFolderTree(c.Context(), uint(id)); err != nil {
		return responses.InternalError(c, "Could not delete folder")
	}

	return c.JSON(fiber.Map{"delete": true})
}

func (h *FolderHandler) ListSubfolders(c *fiber.Ctx) error {
	folderID, err := c.ParamsInt("folder_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid folder ID")
	}

	parentFolder, err := h.fileService.GetFolder(c.Context(), uint(folderID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "folder")
	}

	learner := false
	if parentFolder.ContextType == "Course" {
		if err := h.authz.RequireCourseEnrolled(c, parentFolder.ContextID); err != nil {
			return err
		}
		learner = !h.managesFolderCourse(c, parentFolder.ContextID)
		if learner && h.learnerBlockedFolder(c, parentFolder) {
			return responses.NotFound(c, "folder")
		}
	}

	params := middleware.GetPagination(c)

	var result *repository.PaginatedResult[models.Folder]
	if h.fileService.HasTree() {
		result, err = h.fileService.ListTreeFolders(c.Context(), repository.FileTreeQuery{
			ContextType: parentFolder.ContextType, ContextID: parentFolder.ContextID,
			Scope: repository.FolderScopeFolder, FolderID: parentFolder.ID, LearnerView: learner,
		}, params)
	} else {
		result, err = h.fileService.ListSubfolders(c.Context(), uint(folderID), callerAccountID(c), params)
	}
	if err != nil {
		return responses.InternalError(c, "Could not fetch subfolders")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	folders := make([]fiber.Map, len(result.Items))
	for i, f := range result.Items {
		folders[i] = folderToJSON(&f)
	}

	return c.JSON(folders)
}

// canManageNonCourseFolder is the write rule for folders outside a
// course: a personal (User) folder may be changed by its owner, anything
// else (Group, Account, unknown) only by a same-tenant admin (the folder
// was already loaded under the caller's tenant). Pre-fix UpdateFolder and
// DeleteFolder checked only Course folders, so any authenticated user in
// the tenant could rename or delete another user's or a group's folder.
func (h *FolderHandler) canManageNonCourseFolder(c *fiber.Ctx, folder *models.Folder) bool {
	// DB-backed role (masquerade-capped), not the JWT is_admin hint.
	if callerIsAdmin(c, h.authz) {
		return true
	}
	userID, _ := c.Locals("user_id").(uint)
	return folder.ContextType == "User" && userID != 0 && folder.ContextID == userID
}
