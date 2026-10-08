package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

// Announcement replies (G090): readers comment on a course announcement
// that allows comments; the course's instructors can always reply and
// moderate (delete) replies; authors edit and delete their own.

func announcementReplyToJSON(r *models.AnnouncementReply, userName string) fiber.Map {
	return fiber.Map{
		"id":              r.ID,
		"announcement_id": r.AnnouncementID,
		"user_id":         r.UserID,
		"user_name":       userName,
		"parent_id":       r.ParentID,
		"message":         r.Message,
		"created_at":      r.CreatedAt,
		"updated_at":      r.UpdatedAt,
	}
}

// loadReplyAnnouncement loads the :id announcement and checks the caller
// can read it. (result, wrote, err): wrote=true means a response was
// written and the caller must return err.
func (h *AnnouncementHandler) loadReplyAnnouncement(c *fiber.Ctx) (*models.Announcement, bool, error) {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return nil, true, responses.BadRequest(c, "Invalid announcement ID")
	}
	a, err := h.announcementService.GetAnnouncement(c.Context(), uint(id))
	if err != nil || a == nil {
		return nil, true, responses.NotFound(c, "announcement")
	}
	if wrote, err := h.authorizeAnnouncement(c, a, false); wrote {
		return nil, true, err
	}
	if a.CourseID == nil {
		return nil, true, responses.BadRequest(c, "Replies are only available on course announcements")
	}
	if !h.announcementService.RepliesEnabled() {
		return nil, true, responses.Error(c, fiber.StatusServiceUnavailable, "Announcement replies are not available")
	}
	return a, false, nil
}

// canManageAnnouncement reports (without writing) whether the caller
// manages the announcement's course.
func (h *AnnouncementHandler) canManageAnnouncement(c *fiber.Ctx, a *models.Announcement) bool {
	if a.CourseID == nil {
		return callerIsAdmin(c, h.authz)
	}
	return callerIsCourseInstructor(c, h.authz, *a.CourseID)
}

// ListReplies handles GET /announcements/:id/replies.
func (h *AnnouncementHandler) ListReplies(c *fiber.Ctx) error {
	a, wrote, err := h.loadReplyAnnouncement(c)
	if wrote {
		return err
	}
	params := middleware.GetPagination(c)
	res, err := h.announcementService.ListReplies(c.Context(), a.ID, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch replies")
	}
	responses.SetPaginationHeaders(c, res.TotalCount, res.Page, res.PerPage)
	names := map[uint]string{}
	out := make([]fiber.Map, len(res.Items))
	for i := range res.Items {
		r := &res.Items[i]
		out[i] = announcementReplyToJSON(r, h.userName(c, names, r.UserID))
	}
	return c.JSON(out)
}

func (h *AnnouncementHandler) userName(c *fiber.Ctx, cache map[uint]string, id uint) string {
	if n, ok := cache[id]; ok {
		return n
	}
	name := ""
	if h.authz != nil && h.authz.userRepo != nil {
		if u, err := h.authz.userRepo.FindByID(c.Context(), id, callerAccountID(c)); err == nil && u != nil {
			name = u.Name
		}
	}
	cache[id] = name
	return name
}

// CreateReply handles POST /announcements/:id/replies {message, parent_id}.
func (h *AnnouncementHandler) CreateReply(c *fiber.Ctx) error {
	a, wrote, err := h.loadReplyAnnouncement(c)
	if wrote {
		return err
	}
	if err := h.announcementService.CanReply(a, h.canManageAnnouncement(c, a)); err != nil {
		if errors.Is(err, service.ErrRepliesDisabled) {
			return responses.Forbidden(c, err.Error())
		}
		return responses.BadRequest(c, err.Error())
	}
	var input struct {
		Message  string `json:"message"`
		ParentID *uint  `json:"parent_id"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	userID, _ := c.Locals("user_id").(uint)
	reply, err := h.announcementService.CreateReply(c.Context(), a, userID, service.SanitizeHTML(input.Message), input.ParentID)
	if err != nil {
		if errors.Is(err, service.ErrReplyNotFound) {
			return responses.NotFound(c, "reply")
		}
		return responses.BadRequest(c, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(announcementReplyToJSON(reply, h.userName(c, map[uint]string{}, userID)))
}

// loadReply loads :reply_id of the announcement. (result, wrote, err).
func (h *AnnouncementHandler) loadReply(c *fiber.Ctx, a *models.Announcement) (*models.AnnouncementReply, bool, error) {
	rid, err := c.ParamsInt("reply_id")
	if err != nil || rid <= 0 {
		return nil, true, responses.BadRequest(c, "Invalid reply ID")
	}
	r, err := h.announcementService.GetReply(c.Context(), a.ID, uint(rid))
	if err != nil {
		return nil, true, responses.NotFound(c, "reply")
	}
	return r, false, nil
}

// UpdateReply handles PUT /announcements/:id/replies/:reply_id (author only).
func (h *AnnouncementHandler) UpdateReply(c *fiber.Ctx) error {
	a, wrote, err := h.loadReplyAnnouncement(c)
	if wrote {
		return err
	}
	r, wrote, err := h.loadReply(c, a)
	if wrote {
		return err
	}
	userID, _ := c.Locals("user_id").(uint)
	if r.UserID != userID {
		return responses.Forbidden(c, "You can only edit your own reply")
	}
	var input struct {
		Message string `json:"message"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	if err := h.announcementService.UpdateReply(c.Context(), r, service.SanitizeHTML(input.Message)); err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.JSON(announcementReplyToJSON(r, h.userName(c, map[uint]string{}, r.UserID)))
}

// DeleteReply handles DELETE /announcements/:id/replies/:reply_id (author
// or a course instructor).
func (h *AnnouncementHandler) DeleteReply(c *fiber.Ctx) error {
	a, wrote, err := h.loadReplyAnnouncement(c)
	if wrote {
		return err
	}
	r, wrote, err := h.loadReply(c, a)
	if wrote {
		return err
	}
	userID, _ := c.Locals("user_id").(uint)
	if r.UserID != userID && !h.canManageAnnouncement(c, a) {
		return responses.Forbidden(c, "You can only delete your own reply")
	}
	if err := h.announcementService.DeleteReply(c.Context(), r); err != nil {
		return responses.InternalError(c, "Could not delete reply")
	}
	return c.JSON(fiber.Map{"deleted": true})
}
