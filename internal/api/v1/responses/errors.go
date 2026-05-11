package responses

import "github.com/gofiber/fiber/v2"

func Error(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(fiber.Map{
		"errors": []fiber.Map{
			{"message": message},
		},
	})
}

func NotFound(c *fiber.Ctx, resource string) error {
	return Error(c, fiber.StatusNotFound, "The specified "+resource+" was not found")
}

func BadRequest(c *fiber.Ctx, message string) error {
	return Error(c, fiber.StatusBadRequest, message)
}

func InternalError(c *fiber.Ctx, message string) error {
	return Error(c, fiber.StatusInternalServerError, message)
}

func Unauthorized(c *fiber.Ctx) error {
	return Error(c, fiber.StatusUnauthorized, "Unauthorized")
}
