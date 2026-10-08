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
	// Wave 5.2: 13.5 LogPIIAccess sweep. Announcement body/list reads
	// are course-keyed content (not student PII). The student-keyed
	// read is GetReadReceipts, which returns per-student read/ack
	// state for an announcement (instructor view).
	auditService *service.AuditService
}

// NewAnnouncementHandler creates a new AnnouncementHandler.
func NewAnnouncementHandler(announcementService *service.AnnouncementService, authz *ResourceAuthorizer, auditService *service.AuditService) *AnnouncementHandler {
	return &AnnouncementHandler{announcementService: announcementService, authz: authz, auditService: auditService}
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
	h.decorate(c, result.Items, announcements)

	return c.JSON(announcements)
}

// decorate adds the section targets and reply counts (G090) to rendered
// announcements.
func (h *AnnouncementHandler) decorate(c *fiber.Ctx, items []models.Announcement, out []fiber.Map) {
	if len(items) == 0 {
		return
	}
	ids := make([]uint, len(items))
	for i, a := range items {
		ids[i] = a.ID
	}
	sections := h.announcementService.SectionsFor(c.Context(), ids)
	counts := h.announcementService.ReplyCounts(c.Context(), ids)
	for i, a := range items {
		secs := sections[a.ID]
		if secs == nil {
			secs = []uint{}
		}
		out[i]["section_ids"] = secs
		out[i]["reply_count"] = counts[a.ID]
	}
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
		SectionIDs     []uint     `json:"section_ids"`
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
		Message:        service.SanitizeHTML(input.Message),
		Priority:       input.Priority,
		RequireAck:     input.RequireAck,
		TargetAudience: input.TargetAudience,
		DelayedPostAt:  input.DelayedPostAt,
		WorkflowState:  input.WorkflowState,
		AllowComments:  input.AllowComments,
	}

	if err := h.announcementService.CreateAnnouncementWithSections(c.Context(), announcement, input.SectionIDs); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	out := []fiber.Map{announcementToJSON(announcement, false, false)}
	h.decorate(c, []models.Announcement{*announcement}, out)
	return c.Status(fiber.StatusCreated).JSON(out[0])
}

// authorizeAnnouncement enforces access to a single announcement. It
// returns wrote=true after writing the refusal; the caller MUST return
// err immediately in that case (the (result, wrote, err) convention).
//
//   - Course announcements: manage = course instructor (or admin); read
//     = enrolled AND the announcement is visible to the caller (drafts and
//     not-yet-posted delayed posts are instructor-only; audience/section
//     targeting applies). An invisible row is a 404, not a 403.
//   - Account announcements: the row must belong to the caller's tenant
//     (cross-tenant is a 404); manage additionally requires an admin.
func (h *AnnouncementHandler) authorizeAnnouncement(c *fiber.Ctx, a *models.Announcement, manage bool) (bool, error) {
	if a.WorkflowState == service.AnnouncementStateDeleted {
		return true, responses.NotFound(c, "announcement")
	}
	userID, _ := c.Locals("user_id").(uint)

	if a.CourseID != nil {
		var aerr error
		if manage {
			aerr = h.authz.RequireCourseInstructor(c, *a.CourseID)
		} else {
			aerr = h.authz.RequireCourseEnrolled(c, *a.CourseID)
		}
		if authzDenied(c, aerr) {
			return true, aerr
		}
		if !manage && !h.announcementService.CanView(c.Context(), a, userID) {
			return true, responses.NotFound(c, "announcement")
		}
		return false, nil
	}

	var accountID uint
	if a.AccountID != nil {
		accountID = *a.AccountID
	}
	if assertSameTenant(c, accountID) {
		return true, nil
	}
	if manage {
		if !callerIsAdmin(c, h.authz) {
			return true, responses.Forbidden(c, "admin access required")
		}
		return false, nil
	}
	if !h.announcementService.CanView(c.Context(), a, userID) && !callerIsAdmin(c, h.authz) {
		return true, responses.NotFound(c, "announcement")
	}
	return false, nil
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

	if wrote, err := h.authorizeAnnouncement(c, announcement, false); wrote {
		return err
	}

	// Auto-mark as read on view
	_ = h.announcementService.MarkAsRead(c.Context(), announcement.ID, userID)

	isRead := true
	isAck := h.announcementService.IsAcknowledged(c.Context(), announcement.ID, userID)

	out := []fiber.Map{announcementToJSON(announcement, isRead, isAck)}
	h.decorate(c, []models.Announcement{*announcement}, out)
	return c.JSON(out[0])
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

	// Course announcements: course instructor. Account announcements:
	// admin of the owning tenant.
	if wrote, err := h.authorizeAnnouncement(c, announcement, true); wrote {
		return err
	}
	previousState := announcement.WorkflowState

	var input struct {
		Title          *string    `json:"title"`
		Message        *string    `json:"message"`
		Priority       *string    `json:"priority"`
		RequireAck     *bool      `json:"require_acknowledgement"`
		TargetAudience *string    `json:"target_audience"`
		DelayedPostAt  *time.Time `json:"delayed_post_at"`
		WorkflowState  *string    `json:"workflow_state"`
		AllowComments  *bool      `json:"allow_comments"`
		SectionIDs     *[]uint    `json:"section_ids"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Title != nil {
		announcement.Title = *input.Title
	}
	if input.Message != nil {
		announcement.Message = service.SanitizeHTML(*input.Message)
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
		// "published" without a delayed_post_at means "post it now":
		// drop any stale delay so the row isn't silently re-scheduled.
		if *input.WorkflowState == service.AnnouncementStatePublished && input.DelayedPostAt == nil {
			announcement.DelayedPostAt = nil
		}
	}
	if input.AllowComments != nil {
		announcement.AllowComments = *input.AllowComments
	}

	var sectionIDs []uint
	if input.SectionIDs != nil {
		sectionIDs = *input.SectionIDs
		if sectionIDs == nil {
			sectionIDs = []uint{}
		}
	}
	if err := h.announcementService.UpdateAnnouncementWithSections(c.Context(), announcement, previousState, sectionIDs); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	userID, _ := c.Locals("user_id").(uint)
	isRead := h.announcementService.IsRead(c.Context(), announcement.ID, userID)
	isAck := h.announcementService.IsAcknowledged(c.Context(), announcement.ID, userID)

	out := []fiber.Map{announcementToJSON(announcement, isRead, isAck)}
	h.decorate(c, []models.Announcement{*announcement}, out)
	return c.JSON(out[0])
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

	if wrote, err := h.authorizeAnnouncement(c, announcement, true); wrote {
		return err
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

	if wrote, err := h.authorizeAnnouncement(c, announcement, false); wrote {
		return err
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

	if wrote, err := h.authorizeAnnouncement(c, announcement, false); wrote {
		return err
	}

	userID, _ := c.Locals("user_id").(uint)

	if err := h.announcementService.AcknowledgeAnnouncement(c.Context(), uint(id), userID); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{"acknowledged": true})
}

// GetReadReceipts handles GET /announcements/:id/read_receipts
//
// The route carries no :course_id, so course-role middleware cannot
// resolve the course; authorization happens here against the
// announcement's own course (instructor) or account (tenant admin).
func (h *AnnouncementHandler) GetReadReceipts(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid announcement ID")
	}

	announcement, err := h.announcementService.GetAnnouncement(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "announcement")
	}
	if wrote, err := h.authorizeAnnouncement(c, announcement, true); wrote {
		return err
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
	stats, err := h.announcementService.GetAnnouncementStats(c.Context(), uint(id))
	if err != nil || stats == nil {
		stats = &service.AnnouncementStats{}
	}

	// Wave 5.2 FERPA audit: read-receipts expose which specific
	// students have read/acknowledged an announcement. Bulk read
	// (studentID=0) keyed on the announcement resource.
	if callerID, _ := getUserID(c); callerID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, 0, "read", "announcement_read_receipts", "announcements", uint(id), c.IP(), c.Get("User-Agent"))
	}

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
//
// Account announcements are visible to every user of that account, and
// only to them: a :account_id other than the caller's tenant is a 404.
func (h *AnnouncementHandler) ListAccountAnnouncements(c *fiber.Ctx) error {
	accountID, err := c.ParamsInt("account_id")
	if err != nil || accountID <= 0 {
		return responses.BadRequest(c, "Invalid account ID")
	}
	if assertSameTenant(c, uint(accountID)) {
		return nil
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
	h.decorate(c, result.Items, announcements)

	return c.JSON(announcements)
}

// CreateAccountAnnouncement handles POST /accounts/:account_id/announcements
func (h *AnnouncementHandler) CreateAccountAnnouncement(c *fiber.Ctx) error {
	accountID, err := c.ParamsInt("account_id")
	if err != nil || accountID <= 0 {
		return responses.BadRequest(c, "Invalid account ID")
	}
	// Defense in depth: an admin may only post to their own account.
	if assertSameTenant(c, uint(accountID)) {
		return nil
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

	// A site-wide (cross-tenant) announcement is a platform-operator
	// action, never a tenant admin's.
	if input.IsGlobal {
		if isSuper, _ := c.Locals("is_super_admin").(bool); !isSuper {
			return responses.Forbidden(c, "only a platform administrator can post a global announcement")
		}
	}

	userID, _ := c.Locals("user_id").(uint)
	aid := uint(accountID)

	announcement := &models.Announcement{
		AccountID:      &aid,
		UserID:         userID,
		Title:          input.Title,
		Message:        service.SanitizeHTML(input.Message),
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
