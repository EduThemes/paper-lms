package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

// ---------------------------------------------------------------------------
// Account reports (G117)
// ---------------------------------------------------------------------------

func accountReportJSON(r *models.AccountReport) fiber.Map {
	var params map[string]interface{}
	_ = json.Unmarshal([]byte(r.Parameters), &params)
	out := fiber.Map{
		"id":          r.ID,
		"report":      r.ReportType,
		"status":      r.WorkflowState,
		"progress":    r.Progress,
		"message":     r.Message,
		"row_count":   r.RowCount,
		"parameters":  params,
		"user_id":     r.UserID,
		"created_at":  r.CreatedAt,
		"started_at":  r.StartedAt,
		"ended_at":    r.EndedAt,
		"file_name":   r.FileName,
		"downloadable": r.WorkflowState == models.AccountReportComplete,
	}
	return out
}

// ListReports handles GET /accounts/:account_id/reports: every report type
// with its most recent run.
func (h *AdminAccountsHandler) ListReports(c *fiber.Ctx) error {
	accountID, wrote := sisAccountScope(c)
	if wrote {
		return nil
	}
	runs, err := h.reports.List(c.Context(), accountID, "", 100)
	if err != nil {
		return responses.InternalError(c, "Could not load reports")
	}
	last := map[string]fiber.Map{}
	for i := range runs {
		if _, ok := last[runs[i].ReportType]; !ok {
			last[runs[i].ReportType] = accountReportJSON(&runs[i])
		}
	}
	defs := service.AccountReportDefinitions()
	out := make([]fiber.Map, len(defs))
	for i, d := range defs {
		out[i] = fiber.Map{"report": d.Report, "title": d.Title, "description": d.Description, "parameters": d.Parameters, "last_run": last[d.Report]}
	}
	return c.JSON(out)
}

// ListReportRuns handles GET /accounts/:account_id/reports/:report.
func (h *AdminAccountsHandler) ListReportRuns(c *fiber.Ctx) error {
	accountID, wrote := sisAccountScope(c)
	if wrote {
		return nil
	}
	runs, err := h.reports.List(c.Context(), accountID, c.Params("report"), c.QueryInt("limit", 25))
	if err != nil {
		return responses.InternalError(c, "Could not load reports")
	}
	out := make([]fiber.Map, len(runs))
	for i := range runs {
		out[i] = accountReportJSON(&runs[i])
	}
	return c.JSON(out)
}

// StartReport handles POST /accounts/:account_id/reports/:report.
func (h *AdminAccountsHandler) StartReport(c *fiber.Ctx) error {
	accountID, wrote := sisAccountScope(c)
	if wrote {
		return nil
	}
	var body struct {
		Parameters service.AccountReportParams `json:"parameters"`
	}
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&body); err != nil {
			return responses.BadRequest(c, "Invalid input")
		}
	}
	userID, _ := c.Locals("user_id").(uint)
	r, err := h.reports.Start(c.Context(), accountID, userID, c.Params("report"), body.Parameters)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUnknownReport):
			return responses.NotFound(c, "report")
		case errors.Is(err, service.ErrReportTermNotFound):
			return responses.BadRequest(c, err.Error())
		}
		return responses.InternalError(c, "Could not start the report")
	}
	return c.Status(fiber.StatusCreated).JSON(accountReportJSON(r))
}

func (h *AdminAccountsHandler) reportRun(c *fiber.Ctx) (*models.AccountReport, bool) {
	accountID, wrote := sisAccountScope(c)
	if wrote {
		return nil, true
	}
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		_ = responses.BadRequest(c, "Invalid report ID")
		return nil, true
	}
	r, err := h.reports.Get(c.Context(), uint(id), accountID)
	if err != nil || r.ReportType != c.Params("report") {
		_ = responses.NotFound(c, "report")
		return nil, true
	}
	return r, false
}

// GetReportRun handles GET /accounts/:account_id/reports/:report/:id.
func (h *AdminAccountsHandler) GetReportRun(c *fiber.Ctx) error {
	r, wrote := h.reportRun(c)
	if wrote {
		return nil
	}
	return c.JSON(accountReportJSON(r))
}

// DownloadReport handles GET /accounts/:account_id/reports/:report/:id/download.
func (h *AdminAccountsHandler) DownloadReport(c *fiber.Ctx) error {
	r, wrote := h.reportRun(c)
	if wrote {
		return nil
	}
	if r.WorkflowState != models.AccountReportComplete || len(r.FileData) == 0 {
		return responses.NotFound(c, "report file")
	}
	ct := "text/csv; charset=utf-8"
	if strings.HasSuffix(r.FileName, ".zip") {
		ct = "application/zip"
	}
	c.Set("Content-Type", ct)
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", r.FileName))
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("Cache-Control", "private, no-store")
	return c.Send(r.FileData)
}

// DeleteReportRun handles DELETE /accounts/:account_id/reports/:report/:id.
func (h *AdminAccountsHandler) DeleteReportRun(c *fiber.Ctx) error {
	r, wrote := h.reportRun(c)
	if wrote {
		return nil
	}
	if err := h.reports.Delete(c.Context(), r.ID, r.AccountID); err != nil {
		return responses.NotFound(c, "report")
	}
	return c.JSON(fiber.Map{"id": r.ID, "deleted": true})
}

// ---------------------------------------------------------------------------
// Theme / brand config (G118)
// ---------------------------------------------------------------------------

// GetMyBrandConfig handles GET /brand_config: the signed-in user's account
// theme (every user needs it to render the UI).
func (h *AdminAccountsHandler) GetMyBrandConfig(c *fiber.Ctx) error {
	v, err := h.brand.Get(c.Context(), callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not load the theme")
	}
	return c.JSON(v)
}

// GetMyBrandLogo handles GET /brand_config/logo.
func (h *AdminAccountsHandler) GetMyBrandLogo(c *fiber.Ctx) error {
	data, ct, err := h.brand.Logo(c.Context(), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "logo")
	}
	c.Set("Content-Type", ct)
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("Cache-Control", "private, max-age=3600")
	c.Set("Content-Security-Policy", "default-src 'none'")
	return c.Send(data)
}

// GetBrandConfig handles GET /accounts/:account_id/brand_config: the theme
// plus the variable definitions for the Theme Editor.
func (h *AdminAccountsHandler) GetBrandConfig(c *fiber.Ctx) error {
	accountID, wrote := sisAccountScope(c)
	if wrote {
		return nil
	}
	v, err := h.brand.Get(c.Context(), accountID)
	if err != nil {
		return responses.InternalError(c, "Could not load the theme")
	}
	return c.JSON(fiber.Map{"config": v, "variables": service.BrandVariables()})
}

// UpdateBrandConfig handles PUT /accounts/:account_id/brand_config.
func (h *AdminAccountsHandler) UpdateBrandConfig(c *fiber.Ctx) error {
	accountID, wrote := sisAccountScope(c)
	if wrote {
		return nil
	}
	var body struct {
		Variables map[string]string `json:"variables"`
	}
	if err := c.BodyParser(&body); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	userID, _ := c.Locals("user_id").(uint)
	v, err := h.brand.UpdateVariables(c.Context(), accountID, userID, body.Variables)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.JSON(v)
}

// UploadBrandLogo handles POST /accounts/:account_id/brand_config/logo
// (multipart "logo").
func (h *AdminAccountsHandler) UploadBrandLogo(c *fiber.Ctx) error {
	accountID, wrote := sisAccountScope(c)
	if wrote {
		return nil
	}
	fh, err := c.FormFile("logo")
	if err != nil {
		return responses.BadRequest(c, "logo file is required")
	}
	if fh.Size > service.MaxBrandLogoBytes {
		return responses.BadRequest(c, service.ErrBrandLogoTooBig.Error())
	}
	f, err := fh.Open()
	if err != nil {
		return responses.InternalError(c, "Could not read the logo")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, service.MaxBrandLogoBytes+1))
	if err != nil {
		return responses.InternalError(c, "Could not read the logo")
	}
	userID, _ := c.Locals("user_id").(uint)
	v, err := h.brand.SetLogo(c.Context(), accountID, userID, data)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.JSON(v)
}

// DeleteBrandLogo handles DELETE /accounts/:account_id/brand_config/logo.
func (h *AdminAccountsHandler) DeleteBrandLogo(c *fiber.Ctx) error {
	accountID, wrote := sisAccountScope(c)
	if wrote {
		return nil
	}
	userID, _ := c.Locals("user_id").(uint)
	v, err := h.brand.DeleteLogo(c.Context(), accountID, userID)
	if err != nil {
		return responses.InternalError(c, "Could not remove the logo")
	}
	return c.JSON(v)
}

// ResetBrandConfig handles DELETE /accounts/:account_id/brand_config.
func (h *AdminAccountsHandler) ResetBrandConfig(c *fiber.Ctx) error {
	accountID, wrote := sisAccountScope(c)
	if wrote {
		return nil
	}
	if err := h.brand.Reset(c.Context(), accountID); err != nil {
		return responses.InternalError(c, "Could not reset the theme")
	}
	return c.JSON(fiber.Map{"reset": true})
}

// ---------------------------------------------------------------------------
// Admin tools: restore deleted courses (G119)
// ---------------------------------------------------------------------------

// ListDeletedCourses handles GET /accounts/:account_id/deleted_courses.
func (h *AdminAccountsHandler) ListDeletedCourses(c *fiber.Ctx) error {
	accountID, wrote := sisAccountScope(c)
	if wrote {
		return nil
	}
	cs, err := h.tools.ListDeletedCourses(c.Context(), accountID, c.Query("search_term"), c.QueryInt("limit", 50))
	if err != nil {
		return responses.InternalError(c, "Could not load deleted courses")
	}
	out := make([]fiber.Map, len(cs))
	for i, co := range cs {
		out[i] = fiber.Map{
			"id": co.ID, "name": co.Name, "course_code": co.CourseCode, "sis_course_id": co.SISCourseID,
			"enrollment_term_id": co.EnrollmentTermID, "workflow_state": co.WorkflowState, "deleted_at": co.UpdatedAt,
		}
	}
	return c.JSON(out)
}

// RestoreCourse handles POST /accounts/:account_id/deleted_courses/:course_id/restore.
func (h *AdminAccountsHandler) RestoreCourse(c *fiber.Ctx) error {
	accountID, wrote := sisAccountScope(c)
	if wrote {
		return nil
	}
	id, err := c.ParamsInt("course_id")
	if err != nil || id <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}
	co, err := h.tools.RestoreCourse(c.Context(), uint(id), accountID)
	if err != nil {
		return responses.NotFound(c, "course")
	}
	if h.audit != nil {
		userID, _ := c.Locals("user_id").(uint)
		acct := accountID
		cid := co.ID
		payload, _ := json.Marshal(fiber.Map{"course_id": co.ID, "workflow_state": co.WorkflowState})
		_ = h.audit.LogEvent(c.Context(), "course_restored", userID, &cid, &acct, "Course", co.ID,
			"restored", string(payload), c.IP(), c.Get("User-Agent"))
	}
	return c.JSON(fiber.Map{"id": co.ID, "name": co.Name, "workflow_state": co.WorkflowState})
}

// ---------------------------------------------------------------------------
// Storage quotas (G120)
// ---------------------------------------------------------------------------

// GetQuotaDefaults handles GET /accounts/:account_id/storage_quotas.
func (h *AdminAccountsHandler) GetQuotaDefaults(c *fiber.Ctx) error {
	accountID, wrote := sisAccountScope(c)
	if wrote {
		return nil
	}
	d, err := h.quotas.AccountDefaults(c.Context(), accountID)
	if err != nil {
		return responses.InternalError(c, "Could not load quotas")
	}
	return c.JSON(fiber.Map{"default_course_quota_mb": d["Course"], "default_user_quota_mb": d["User"], "default_group_quota_mb": d["Group"]})
}

// UpdateQuotaDefaults handles PUT /accounts/:account_id/storage_quotas.
func (h *AdminAccountsHandler) UpdateQuotaDefaults(c *fiber.Ctx) error {
	accountID, wrote := sisAccountScope(c)
	if wrote {
		return nil
	}
	var body struct {
		Course *int64 `json:"default_course_quota_mb"`
		User   *int64 `json:"default_user_quota_mb"`
		Group  *int64 `json:"default_group_quota_mb"`
	}
	if err := c.BodyParser(&body); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	set := map[string]int64{}
	if body.Course != nil {
		set["Course"] = *body.Course
	}
	if body.User != nil {
		set["User"] = *body.User
	}
	if body.Group != nil {
		set["Group"] = *body.Group
	}
	if err := h.quotas.SetAccountDefaults(c.Context(), accountID, set); err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return h.GetQuotaDefaults(c)
}

func quotaContextType(raw string) (string, bool) {
	switch strings.ToLower(raw) {
	case "courses", "course":
		return "Course", true
	case "users", "user":
		return "User", true
	case "groups", "group":
		return "Group", true
	}
	return "", false
}

// GetContextQuota handles GET /accounts/:account_id/storage_quotas/:context_type/:context_id.
func (h *AdminAccountsHandler) GetContextQuota(c *fiber.Ctx) error {
	accountID, wrote := sisAccountScope(c)
	if wrote {
		return nil
	}
	ct, ok := quotaContextType(c.Params("context_type"))
	id, err := strconv.ParseUint(c.Params("context_id"), 10, 64)
	if !ok || err != nil || id == 0 {
		return responses.NotFound(c, "context")
	}
	info, err := h.quotas.Info(c.Context(), ct, uint(id), accountID)
	if err != nil {
		return responses.NotFound(c, strings.ToLower(ct))
	}
	return c.JSON(info)
}

// SetContextQuota handles PUT /accounts/:account_id/storage_quotas/:context_type/:context_id
// with {"quota_mb": n}; null or a negative value clears the override.
func (h *AdminAccountsHandler) SetContextQuota(c *fiber.Ctx) error {
	accountID, wrote := sisAccountScope(c)
	if wrote {
		return nil
	}
	ct, ok := quotaContextType(c.Params("context_type"))
	id, err := strconv.ParseUint(c.Params("context_id"), 10, 64)
	if !ok || err != nil || id == 0 {
		return responses.NotFound(c, "context")
	}
	var body struct {
		QuotaMB *int64 `json:"quota_mb"`
	}
	if err := c.BodyParser(&body); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	mb := int64(-1)
	if body.QuotaMB != nil {
		mb = *body.QuotaMB
	}
	info, err := h.quotas.SetContextQuota(c.Context(), ct, uint(id), accountID, mb)
	if err != nil {
		if errors.Is(err, service.ErrQuotaContextNotFound) {
			return responses.NotFound(c, strings.ToLower(ct))
		}
		return responses.BadRequest(c, err.Error())
	}
	return c.JSON(info)
}

// GetCourseQuota handles GET /courses/:course_id/storage_quota (course
// staff see their course's usage on the Files page).
func (h *AdminAccountsHandler) GetCourseQuota(c *fiber.Ctx) error {
	id, err := c.ParamsInt("course_id")
	if err != nil || id <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}
	scope := callerAccountID(c)
	if isSuper, _ := c.Locals("is_super_admin").(bool); isSuper {
		scope = 0
	}
	info, err := h.quotas.Info(c.Context(), "Course", uint(id), scope)
	if err != nil {
		return responses.NotFound(c, "course")
	}
	return c.JSON(info)
}

// ---------------------------------------------------------------------------
// Custom role helpers (G102)
// ---------------------------------------------------------------------------

// GetBaseRolePermissions handles GET /accounts/:account_id/role_base_permissions?base_role_type=ta:
// a base role's defaults, used to prefill a new custom role.
func (h *AdminAccountsHandler) GetBaseRolePermissions(c *fiber.Ctx) error {
	if _, wrote := sisAccountScope(c); wrote {
		return nil
	}
	base := c.Query("base_role_type")
	if !models.IsValidBaseRoleType(base) {
		return responses.BadRequest(c, "invalid base_role_type")
	}
	perms := h.roles.BaseRolePermissions(base)
	enabled := make([]string, 0, len(perms))
	for _, p := range models.AllPermissionNames() {
		if perms[p] {
			enabled = append(enabled, p)
		}
	}
	return c.JSON(fiber.Map{"base_role_type": base, "permissions": enabled})
}

// ListAssignableRoles handles GET /courses/:course_id/assignable_roles:
// the active custom course roles of the course's tenant, for the Add
// People role picker.
func (h *AdminAccountsHandler) ListAssignableRoles(c *fiber.Ctx) error {
	roles, err := h.roles.ListActiveCourseRoles(c.Context(), callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not load roles")
	}
	out := make([]fiber.Map, len(roles))
	for i, r := range roles {
		out[i] = fiber.Map{"id": r.ID, "name": r.Name, "label": r.Label, "base_role_type": r.BaseRoleType, "enrollment_type": service.BaseRoleEnrollmentType(r.BaseRoleType)}
	}
	return c.JSON(out)
}
