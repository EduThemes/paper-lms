package handlers

import (
	"github.com/gofiber/fiber/v2"
)

// getUserID safely extracts the authenticated user's ID from Fiber context.
// Returns a 401 error if user_id is not set (should never happen behind
// AuthMiddleware).
//
// The error is a *fiber.Error that the app ErrorHandler renders; the helper
// does NOT write the response itself. It used to call responses.Unauthorized,
// which returns nil after writing, so `userID, err := getUserID(c); if err
// != nil { return err }` carried on with userID == 0 (the
// (result, wrote, err) pitfall).
func getUserID(c *fiber.Ctx) (uint, error) {
	userID, ok := c.Locals("user_id").(uint)
	if !ok || userID == 0 {
		return 0, fiber.NewError(fiber.StatusUnauthorized, "Unauthorized")
	}
	return userID, nil
}
