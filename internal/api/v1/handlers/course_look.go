package handlers

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/designdoc"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

// Page Designer additions to the course API (docs/page-designer/README.md):
// the course look (courses.look) and the resolved pages.custom_css_enabled.

// SetPageDesigner wires what the course look needs: the attachment
// repository (a banner may be a file of this course) and the Settings
// Engine lookup for custom_css_enabled. Either may be nil.
func (h *CourseHandler) SetPageDesigner(files repository.AttachmentRepository, lookup service.SettingsLookupFunc) *CourseHandler {
	h.lookFiles = files
	h.settingsLookup = lookup
	return h
}

// withDesignerFields adds custom_css_enabled (resolved for the course's
// account) to a single-course response. look is already in courseToJSON.
func (h *CourseHandler) withDesignerFields(c *fiber.Ctx, course *models.Course, out fiber.Map) fiber.Map {
	out["custom_css_enabled"] = customCSSEnabled(c.Context(), h.settingsLookup, course.AccountID)
	return out
}

// courseFileChecker reports whether a file id is a live file of course,
// looked up inside the caller's tenant.
func (h *CourseHandler) courseFileChecker(c *fiber.Ctx, course *models.Course) designdoc.FileInCourse {
	return func(fileID uint) bool {
		if h.lookFiles == nil {
			return false
		}
		att, err := h.lookFiles.FindByID(c.Context(), fileID, callerAccountID(c))
		return err == nil && att != nil &&
			att.ContextType == "Course" && att.ContextID == course.ID &&
			att.WorkflowState != models.AttachmentDeleted
	}
}

// applyCourseLook validates course[look] and sets it on course (JSON null
// clears it). Only course managers may change it. Returns wrote=true
// after writing an error response.
func (h *CourseHandler) applyCourseLook(c *fiber.Ctx, course *models.Course, raw json.RawMessage) (bool, error) {
	if len(raw) == 0 {
		return false, nil
	}
	if !canViewUnpublishedContent(c) {
		return true, responses.Forbidden(c, "Only course staff can change the course look")
	}
	if models.NullJSON(raw).IsNull() {
		course.Look = nil
		return false, nil
	}
	look, err := designdoc.ValidateLook(raw, h.courseFileChecker(c, course))
	if err != nil {
		return true, unprocessable(c, err.Error())
	}
	course.Look = look
	return false, nil
}
