package handlers

import (
	"context"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// UnpublishGuard reports whether a published course has graded student
// work, which keeps it from being unpublished (G077).
type UnpublishGuard func(ctx context.Context, courseID uint) (hasGradedWork bool, err error)

// SetUnpublishGuard wires the Canvas "unpublishable?" rule: a published
// course whose real students already have scored submissions can't go
// back to unpublished (it would take their grades away mid-term).
func (h *CourseHandler) SetUnpublishGuard(g UnpublishGuard) *CourseHandler {
	h.unpublishGuard = g
	return h
}

// unpublishBlockedMessage matches Canvas's wording for the refusal.
const unpublishBlockedMessage = "This course can't be unpublished because students have graded submissions in it"

// checkUnpublishable answers the available -> unpublished transition. It
// writes the error response itself, so it follows the (wrote, err)
// pattern: when wrote is true the caller must return err at once.
func (h *CourseHandler) checkUnpublishable(c *fiber.Ctx, course *models.Course, next models.CourseWorkflow) (bool, error) {
	if h.unpublishGuard == nil || course.WorkflowState != models.CourseAvailable || next != models.CourseUnpublished {
		return false, nil
	}
	graded, err := h.unpublishGuard(c.Context(), course.ID)
	if err != nil {
		return true, responses.InternalError(c, "Could not check the course's submissions")
	}
	if graded {
		return true, responses.BadRequest(c, unpublishBlockedMessage)
	}
	return false, nil
}
