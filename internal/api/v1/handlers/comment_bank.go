package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type CommentBankHandler struct {
	service *service.CommentBankService
}

func NewCommentBankHandler(service *service.CommentBankService) *CommentBankHandler {
	return &CommentBankHandler{service: service}
}

func commentBankItemToJSON(item *models.CommentBankItem) fiber.Map {
	result := fiber.Map{
		"id":         item.ID,
		"user_id":    item.UserID,
		"comment":    item.Comment,
		"created_at": item.CreatedAt,
		"updated_at": item.UpdatedAt,
	}
	if item.CourseID != nil {
		result["course_id"] = *item.CourseID
	}
	return result
}

func (h *CommentBankHandler) ListItems(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uint)
	if !ok || userID == 0 {
		return responses.Unauthorized(c)
	}

	params := middleware.GetPagination(c)
	result, err := h.service.List(c.Context(), userID, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch comment bank items")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	items := make([]fiber.Map, len(result.Items))
	for i, item := range result.Items {
		items[i] = commentBankItemToJSON(&item)
	}
	return c.JSON(items)
}

func (h *CommentBankHandler) CreateItem(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uint)
	if !ok || userID == 0 {
		return responses.Unauthorized(c)
	}

	var input struct {
		Comment  string `json:"comment"`
		CourseID *uint  `json:"course_id"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	item := &models.CommentBankItem{
		Comment:  input.Comment,
		CourseID: input.CourseID,
	}

	if err := h.service.Create(c.Context(), userID, item); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(commentBankItemToJSON(item))
}

func (h *CommentBankHandler) UpdateItem(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uint)
	if !ok || userID == 0 {
		return responses.Unauthorized(c)
	}

	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid comment bank item ID")
	}

	var input struct {
		Comment string `json:"comment"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	item, err := h.service.Update(c.Context(), userID, uint(id), input.Comment)
	if err != nil {
		if err.Error() == "unauthorized" {
			return responses.Error(c, fiber.StatusForbidden, "You do not own this comment bank item")
		}
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(commentBankItemToJSON(item))
}

func (h *CommentBankHandler) DeleteItem(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uint)
	if !ok || userID == 0 {
		return responses.Unauthorized(c)
	}

	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid comment bank item ID")
	}

	if err := h.service.Delete(c.Context(), userID, uint(id)); err != nil {
		if err.Error() == "unauthorized" {
			return responses.Error(c, fiber.StatusForbidden, "You do not own this comment bank item")
		}
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{"delete": true})
}

func (h *CommentBankHandler) SearchItems(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uint)
	if !ok || userID == 0 {
		return responses.Unauthorized(c)
	}

	searchTerm := c.Query("search_term")
	if searchTerm == "" {
		return responses.BadRequest(c, "search_term query parameter is required")
	}

	items, err := h.service.Search(c.Context(), userID, searchTerm)
	if err != nil {
		return responses.InternalError(c, "Could not search comment bank items")
	}

	result := make([]fiber.Map, len(items))
	for i, item := range items {
		result[i] = commentBankItemToJSON(&item)
	}
	return c.JSON(result)
}
