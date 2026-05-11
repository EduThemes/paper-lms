package handlers

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

// AnnouncementHandler handles HTTP requests for announcements.
type AnnouncementHandler struct {
	announcementService *service.AnnouncementService
	authz               *ResourceAuthorizer
}

// NewAnnouncementHandler creates a new AnnouncementHandler.
func NewAnnouncementHandler(announcementService *service.AnnouncementService, authz *ResourceAuthorizer) *AnnouncementHandler {
	return &AnnouncementHandler{announcementService: announcementService, authz: authz}
}

func announcementToJSON(a *models.Announcement, isRead bool, isAcknowledged bool) fiber.Map {
	return fiber.Map{
		"id":                      a.ID,
		"course_id":               a.CourseID,
		"account_id":              a.AccountID,
		"user_id":                 a.UserID,
		"title":                   a.Title,
		"message":                 a.Message,
		"priority":                a.Priority,
		"require_acknowledgement": a.RequireAck,
		"target_audience":         a.TargetAudience,
		"posted_at":               a.PostedAt,
		"delayed_post_at":         a.DelayedPostAt,
		"workflow_state":          a.WorkflowState,
		"allow_comments":          a.AllowComments,
		"is_global":               a.IsGlobal,
		"is_read":                 isRead,
		"is_acknowledged":         isAcknowledged,
		"created_at":              a.CreatedAt,
		"updated_at":              a.UpdatedAt,
	}
}

// ListCourseAnnouncements handles GET /courses/:course_id/announcements
func (h *AnnouncementHandler) ListCourseAnnouncements(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	userID, _ := c.Locals("user_id").(uint)
	params := middleware.GetPagination(c)

	result, err := h.announcementService.ListCourseAnnouncements(c.Context(), uint(courseID), userID, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch announcements")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	// Batch-fetch read/ack status in one query instead of N+1
	announcementIDs := make([]uint, len(result.Items))
	for i, a := range result.Items {
		announcementIDs[i] = a.ID
	}
	statusMap := h.announcementService.GetBulkReadStatus(c.Context(), announcementIDs, userID)

	announcements := make([]fiber.Map, len(result.Items))
	for i, a := range result.Items {
		s := statusMap[a.ID]
		announcements[i] = announcementToJSON(&a, s.IsRead, s.IsAcknowledged)
	}

	return c.JSON(announcements)
}

// CreateCourseAnnouncement handles POST /courses/:course_id/announcements
func (h *AnnouncementHandler) CreateCourseAnnouncement(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		Title          string     `json:"title"`
		Message        string     `json:"message"`
		Priority       string     `json:"priority"`
		RequireAck     bool       `json:"require_acknowledgement"`
		TargetAudience string     `json:"target_audience"`
		DelayedPostAt  *time.Time `json:"delayed_post_at"`
		WorkflowState  string     `json:"workflow_state"`
		AllowComments  bool       `json:"allow_comments"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if strings.TrimSpace(input.Title) == "" {
		return responses.BadRequest(c, "Announcement title is required")
	}

	userID, _ := c.Locals("user_id").(uint)
	cid := uint(courseID)

	announcement := &models.Announcement{
		CourseID:       &cid,
		UserID:         userID,
		Title:          input.Title,
		Message:        input.Message,
		Priority:       input.Priority,
		RequireAck:     input.RequireAck,
		TargetAudience: input.TargetAudience,
		DelayedPostAt:  input.DelayedPostAt,
		WorkflowState:  input.WorkflowState,
		AllowComments:  input.AllowComments,
	}

	if err := h.announcementService.CreateAnnouncement(c.Context(), announcement); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(announcementToJSON(announcement, false, false))
}

// GetAnnouncement handles GET /announcements/:id (also marks as read)
func (h *AnnouncementHandler) GetAnnouncement(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid announcement ID")
	}

	userID, _ := c.Locals("user_id").(uint)

	announcement, err := h.announcementService.GetAnnouncement(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "announcement")
	}

	if announcement.WorkflowState == "deleted" {
		return responses.NotFound(c, "announcement")
	}

	// Authorize: user must be enrolled in the announcement's course
	if announcement.CourseID != nil {
		if err := h.authz.RequireCourseEnrolled(c, *announcement.CourseID); err != nil {
			return err
		}
	}

	// Auto-mark as read on view
	_ = h.announcementService.MarkAsRead(c.Context(), announcement.ID, userID)

	isRead := true
	isAck := h.announcementService.IsAcknowledged(c.Context(), announcement.ID, userID)

	return c.JSON(announcementToJSON(announcement, isRead, isAck))
}

// UpdateAnnouncement handles PUT /announcements/:id
func (h *AnnouncementHandler) UpdateAnnouncement(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid announcement ID")
	}

	announcement, err := h.announcementService.GetAnnouncement(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "announcement")
	}

	// Authorize: user must be an instructor in the announcement's course
	if announcement.CourseID != nil {
		if err := h.authz.RequireCourseInstructor(c, *announcement.CourseID); err != nil {
			return err
		}
	}

	var input struct {
		Title          *string    `json:"title"`
		Message        *string    `json:"message"`
		Priority       *string    `json:"priority"`
		RequireAck     *bool      `json:"require_acknowledgement"`
		TargetAudience *string    `json:"target_audience"`
		DelayedPostAt  *time.Time `json:"delayed_post_at"`
		WorkflowState  *string    `json:"workflow_state"`
		AllowComments  *bool      `json:"allow_comments"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Title != nil {
		announcement.Title = *input.Title
	}
	if input.Message != nil {
		announcement.Message = *input.Message
	}
	if input.Priority != nil {
		announcement.Priority = *input.Priority
	}
	if input.RequireAck != nil {
		announcement.RequireAck = *input.RequireAck
	}
	if input.TargetAudience != nil {
		announcement.TargetAudience = *input.TargetAudience
	}
	if input.DelayedPostAt != nil {
		announcement.DelayedPostAt = input.DelayedPostAt
	}
	if input.WorkflowState != nil {
		announcement.WorkflowState = *input.WorkflowState
	}
	if input.AllowComments != nil {
		announcement.AllowComments = *input.AllowComments
	}

	if err := h.announcementService.UpdateAnnouncement(c.Context(), announcement); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	userID, _ := c.Locals("user_id").(uint)
	isRead := h.announcementService.IsRead(c.Context(), announcement.ID, userID)
	isAck := h.announcementService.IsAcknowledged(c.Context(), announcement.ID, userID)

	return c.JSON(announcementToJSON(announcement, isRead, isAck))
}

// DeleteAnnouncement handles DELETE /announcements/:id
func (h *AnnouncementHandler) DeleteAnnouncement(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid announcement ID")
	}

	// Fetch announcement to check authorization
	announcement, err := h.announcementService.GetAnnouncement(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "announcement")
	}

	// Authorize: user must be an instructor in the announcement's course
	if announcement.CourseID != nil {
		if err := h.authz.RequireCourseInstructor(c, *announcement.CourseID); err != nil {
			return err
		}
	}

	if err := h.announcementService.DeleteAnnouncement(c.Context(), uint(id)); err != nil {
		return responses.InternalError(c, "Could not delete announcement")
	}

	return c.JSON(fiber.Map{"delete": true})
}

// MarkAsRead handles POST /announcements/:id/read
func (h *AnnouncementHandler) MarkAsRead(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid announcement ID")
	}

	// Fetch announcement to check authorization
	announcement, err := h.announcementService.GetAnnouncement(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "announcement")
	}

	// Authorize: user must be enrolled in the announcement's course
	if announcement.CourseID != nil {
		if err := h.authz.RequireCourseEnrolled(c, *announcement.CourseID); err != nil {
			return err
		}
	}

	userID, _ := c.Locals("user_id").(uint)

	if err := h.announcementService.MarkAsRead(c.Context(), uint(id), userID); err != nil {
		return responses.InternalError(c, "Could not mark announcement as read")
	}

	return c.JSON(fiber.Map{"marked_as_read": true})
}

// AcknowledgeAnnouncement handles POST /announcements/:id/acknowledge
func (h *AnnouncementHandler) AcknowledgeAnnouncement(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid announcement ID")
	}

	// Fetch announcement to check authorization
	announcement, err := h.announcementService.GetAnnouncement(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "announcement")
	}

	// Authorize: user must be enrolled in the announcement's course
	if announcement.CourseID != nil {
		if err := h.authz.RequireCourseEnrolled(c, *announcement.CourseID); err != nil {
			return err
		}
	}

	userID, _ := c.Locals("user_id").(uint)

	if err := h.announcementService.AcknowledgeAnnouncement(c.Context(), uint(id), userID); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{"acknowledged": true})
}

// GetReadReceipts handles GET /announcements/:id/read_receipts
func (h *AnnouncementHandler) GetReadReceipts(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid announcement ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.announcementService.GetReadReceipts(c.Context(), uint(id), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch read receipts")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	receipts := make([]fiber.Map, len(result.Items))
	for i, r := range result.Items {
		receipts[i] = fiber.Map{
			"id":              r.ID,
			"announcement_id": r.AnnouncementID,
			"user_id":         r.UserID,
			"read_at":         r.ReadAt,
			"acknowledged":    r.Acknowledged,
			"acknowledged_at": r.AcknowledgedAt,
		}
	}

	// Also include stats
	stats, _ := h.announcementService.GetAnnouncementStats(c.Context(), uint(id))

	return c.JSON(fiber.Map{
		"receipts": receipts,
		"stats": fiber.Map{
			"read_count":         stats.ReadCount,
			"acknowledged_count": stats.AcknowledgedCount,
			"total_audience":     stats.TotalAudience,
		},
	})
}

// ListAccountAnnouncements handles GET /accounts/:account_id/announcements
func (h *AnnouncementHandler) ListAccountAnnouncements(c *fiber.Ctx) error {
	accountID, err := c.ParamsInt("account_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.announcementService.ListAccountAnnouncements(c.Context(), uint(accountID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch account announcements")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	userID, _ := c.Locals("user_id").(uint)

	// Batch-fetch read/ack status in one query instead of N+1
	announcementIDs := make([]uint, len(result.Items))
	for i, a := range result.Items {
		announcementIDs[i] = a.ID
	}
	statusMap := h.announcementService.GetBulkReadStatus(c.Context(), announcementIDs, userID)

	announcements := make([]fiber.Map, len(result.Items))
	for i, a := range result.Items {
		s := statusMap[a.ID]
		announcements[i] = announcementToJSON(&a, s.IsRead, s.IsAcknowledged)
	}

	return c.JSON(announcements)
}

// CreateAccountAnnouncement handles POST /accounts/:account_id/announcements
func (h *AnnouncementHandler) CreateAccountAnnouncement(c *fiber.Ctx) error {
	accountID, err := c.ParamsInt("account_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	var input struct {
		Title          string     `json:"title"`
		Message        string     `json:"message"`
		Priority       string     `json:"priority"`
		RequireAck     bool       `json:"require_acknowledgement"`
		TargetAudience string     `json:"target_audience"`
		DelayedPostAt  *time.Time `json:"delayed_post_at"`
		WorkflowState  string     `json:"workflow_state"`
		AllowComments  bool       `json:"allow_comments"`
		IsGlobal       bool       `json:"is_global"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	userID, _ := c.Locals("user_id").(uint)
	aid := uint(accountID)

	announcement := &models.Announcement{
		AccountID:      &aid,
		UserID:         userID,
		Title:          input.Title,
		Message:        input.Message,
		Priority:       input.Priority,
		RequireAck:     input.RequireAck,
		TargetAudience: input.TargetAudience,
		DelayedPostAt:  input.DelayedPostAt,
		WorkflowState:  input.WorkflowState,
		AllowComments:  input.AllowComments,
		IsGlobal:       input.IsGlobal,
	}

	if err := h.announcementService.CreateAnnouncement(c.Context(), announcement); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(announcementToJSON(announcement, false, false))
}
