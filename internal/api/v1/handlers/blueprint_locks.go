package handlers

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/service"
)

// blueprintContentTypeAliases maps Canvas's restrict_item content_type
// values (and Paper's own) onto Paper's lockable content types.
var blueprintContentTypeAliases = map[string]string{
	"assignment":       "Assignment",
	"quiz":             "Quiz",
	"quizzes::quiz":    "Quiz",
	"wiki_page":        "WikiPage",
	"wikipage":         "WikiPage",
	"page":             "WikiPage",
	"discussion_topic": "DiscussionTopic",
	"discussiontopic":  "DiscussionTopic",
	"discussion":       "DiscussionTopic",
}

// ListLockableContent lists the blueprint course's lockable content with
// each item's effective lock settings.
// GET /api/v1/courses/:course_id/blueprint_templates/default/content
func (h *BlueprintHandler) ListLockableContent(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}
	template, err := h.blueprintService.GetOrCreateTemplate(c.Context(), uint(courseID))
	if err != nil {
		return responses.NotFound(c, "blueprint template")
	}
	items, err := h.blueprintService.ListLockableContent(c.Context(), template)
	if err != nil {
		return responses.InternalError(c, "Could not list blueprint content")
	}
	if items == nil {
		items = []service.BlueprintContentItem{}
	}
	return c.JSON(fiber.Map{
		"default_restrictions":     service.ParseBlueprintRestrictions(template.DefaultRestrictions),
		"use_default_restrictions": template.UseDefaultRestrictions,
		"items":                    items,
	})
}

// RestrictItem locks or unlocks one piece of blueprint content (Canvas
// restrict_item).
// PUT /api/v1/courses/:course_id/blueprint_templates/default/restrict_item
//
// Body: {"content_type": "assignment", "content_id": 12,
// "restricted": true} — restricted=true locks with the template's
// default restrictions (content when none are set), false unlocks it.
// Paper extensions: "restrictions": {"content": true, "points": false,
// ...} sets them explicitly; "inherit": true drops the item's own
// setting so it follows the template defaults.
func (h *BlueprintHandler) RestrictItem(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}
	var input struct {
		ContentType  string                         `json:"content_type"`
		ContentID    uint                           `json:"content_id"`
		Restricted   *bool                          `json:"restricted"`
		Restrictions *service.BlueprintRestrictions `json:"restrictions"`
		Inherit      bool                           `json:"inherit"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	contentType, ok := blueprintContentTypeAliases[strings.ToLower(strings.TrimSpace(input.ContentType))]
	if !ok || input.ContentID == 0 {
		return responses.BadRequest(c, "content_type must be one of assignment, quiz, wiki_page, discussion_topic, with a content_id")
	}
	template, err := h.blueprintService.GetOrCreateTemplate(c.Context(), uint(courseID))
	if err != nil {
		return responses.NotFound(c, "blueprint template")
	}

	var r *service.BlueprintRestrictions
	switch {
	case input.Inherit:
		r = nil
	case input.Restrictions != nil:
		r = input.Restrictions
	case input.Restricted != nil && *input.Restricted:
		d := service.ParseBlueprintRestrictions(template.DefaultRestrictions)
		if !d.Any() {
			d = service.BlueprintRestrictions{Content: true}
		}
		r = &d
	case input.Restricted != nil:
		r = &service.BlueprintRestrictions{}
	default:
		return responses.BadRequest(c, "Give restricted, restrictions or inherit")
	}

	if err := h.blueprintService.SetItemRestrictions(c.Context(), template, callerAccountID(c), contentType, input.ContentID, r); err != nil {
		if errors.Is(err, service.ErrBlueprintLockInvalidContent) {
			return responses.NotFound(c, "content")
		}
		return responses.InternalError(c, "Could not update the lock")
	}
	return c.JSON(fiber.Map{"success": true})
}

// ListChildLockedContent lists the locked blueprint copies in a child
// (associated) course.
// GET /api/v1/courses/:course_id/blueprint_subscriptions/locked_content
func (h *BlueprintHandler) ListChildLockedContent(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}
	items, err := h.blueprintService.ListChildLockedContent(c.Context(), uint(courseID))
	if err != nil {
		return responses.InternalError(c, "Could not list locked content")
	}
	if items == nil {
		items = []service.BlueprintLockedChildItem{}
	}
	return c.JSON(items)
}
