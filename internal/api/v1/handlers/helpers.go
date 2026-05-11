package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
)

// getUserID safely extracts the authenticated user's ID from Fiber context.
// Returns 401 Unauthorized if user_id is not set (should never happen behind AuthMiddleware).
func getUserID(c *fiber.Ctx) (uint, error) {
	userID, ok := c.Locals("user_id").(uint)
	if !ok {
		return 0, responses.Unauthorized(c)
	}
	return userID, nil
}
