package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
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
		"created_at":       f.CreatedAt,
		"updated_at":       f.UpdatedAt,
	}
}

func (h *FolderHandler) ListCourseFolders(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.fileService.ListFolders(c.Context(), "Course", uint(courseID), nil, params)
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
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	parentFolderID := input.ParentFolderID
	if parentFolderID == nil {
		root, err := h.fileService.GetOrCreateRootFolder(c.Context(), "Course", uint(courseID))
		if err != nil {
			return responses.InternalError(c, "Could not get root folder")
		}
		parentFolderID = &root.ID
	}

	parent, err := h.fileService.GetFolder(c.Context(), *parentFolderID)
	if err != nil {
		return responses.NotFound(c, "parent folder")
	}

	fullName := parent.FullName + "/" + input.Name

	folder := &models.Folder{
		ContextType:    "Course",
		ContextID:      uint(courseID),
		ParentFolderID: parentFolderID,
		Name:           input.Name,
		FullName:       fullName,
		WorkflowState:  "visible",
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

	folder, err := h.fileService.GetFolder(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "folder")
	}

	if folder.ContextType == "Course" {
		if err := h.authz.RequireCourseEnrolled(c, folder.ContextID); err != nil {
			return err
		}
	}

	return c.JSON(folderToJSON(folder))
}

func (h *FolderHandler) UpdateFolder(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid folder ID")
	}

	folder, err := h.fileService.GetFolder(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "folder")
	}

	if folder.ContextType == "Course" {
		if err := h.authz.RequireCourseInstructor(c, folder.ContextID); err != nil {
			return err
		}
	}

	var input struct {
		Name     *string `json:"name"`
		Position *int    `json:"position"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Name != nil {
		folder.Name = *input.Name
	}
	if input.Position != nil {
		folder.Position = *input.Position
	}

	if err := h.fileService.UpdateFolder(c.Context(), folder); err != nil {
		return responses.InternalError(c, "Could not update folder")
	}

	return c.JSON(folderToJSON(folder))
}

func (h *FolderHandler) DeleteFolder(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid folder ID")
	}

	folder, err := h.fileService.GetFolder(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "folder")
	}

	if folder.ContextType == "Course" {
		if err := h.authz.RequireCourseInstructor(c, folder.ContextID); err != nil {
			return err
		}
	}

	if err := h.fileService.DeleteFolder(c.Context(), uint(id)); err != nil {
		return responses.InternalError(c, "Could not delete folder")
	}

	return c.JSON(fiber.Map{"delete": true})
}

func (h *FolderHandler) ListSubfolders(c *fiber.Ctx) error {
	folderID, err := c.ParamsInt("folder_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid folder ID")
	}

	parentFolder, err := h.fileService.GetFolder(c.Context(), uint(folderID))
	if err != nil {
		return responses.NotFound(c, "folder")
	}

	if parentFolder.ContextType == "Course" {
		if err := h.authz.RequireCourseEnrolled(c, parentFolder.ContextID); err != nil {
			return err
		}
	}

	params := middleware.GetPagination(c)

	result, err := h.fileService.ListSubfolders(c.Context(), uint(folderID), params)
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
