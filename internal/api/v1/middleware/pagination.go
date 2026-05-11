package middleware

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/repository"
)

const (
	DefaultPerPage = 10
	MaxPerPage     = 100
)

func PaginationParams() fiber.Handler {
	return func(c *fiber.Ctx) error {
		page, _ := strconv.Atoi(c.Query("page", "1"))
		if page < 1 {
			page = 1
		}

		perPage, _ := strconv.Atoi(c.Query("per_page", strconv.Itoa(DefaultPerPage)))
		if perPage < 1 {
			perPage = DefaultPerPage
		}
		if perPage > MaxPerPage {
			perPage = MaxPerPage
		}

		c.Locals("pagination", repository.PaginationParams{
			Page:    page,
			PerPage: perPage,
		})

		return c.Next()
	}
}

func GetPagination(c *fiber.Ctx) repository.PaginationParams {
	if p, ok := c.Locals("pagination").(repository.PaginationParams); ok {
		return p
	}
	return repository.PaginationParams{Page: 1, PerPage: DefaultPerPage}
}
