package v1

import (
	"github.com/gofiber/fiber/v2"
)

// registerAdminAccountsRoutes mounts the account-administration lane:
// admin user management (G114), additional logins (G115), account-level
// observer links (G121), user logs and admin tools (G119), account reports
// (G117), theme (G118), storage quotas (G120), custom-role helpers (G102)
// and the SIS batch file listing (G111).
//
// Every /accounts/:account_id/* route is behind `admin` (RequireAdmin),
// which ties :account_id to the caller's tenant (404 otherwise; a
// super_admin crosses tenants by design).
func (r *Router) registerAdminAccountsRoutes(protected fiber.Router, admin, enrolled, instructor fiber.Handler) {
	h := r.AdminAccountsHandler
	if h == nil {
		return
	}

	// Users (G114) + logins (G115) + observer links (G121) + logs (G119)
	protected.Post("/accounts/:account_id/users", admin, h.CreateUser)
	protected.Get("/accounts/:account_id/users/:user_id", admin, h.GetUserDetail)
	protected.Put("/accounts/:account_id/users/:user_id", admin, h.UpdateUser)
	protected.Get("/accounts/:account_id/users/:user_id/history", admin, h.GetUserHistory)
	protected.Get("/accounts/:account_id/users/:user_id/logins", admin, h.ListLogins)
	protected.Post("/accounts/:account_id/users/:user_id/logins", admin, h.CreateLogin)
	protected.Put("/accounts/:account_id/users/:user_id/logins/:login_id", admin, h.UpdateLogin)
	protected.Delete("/accounts/:account_id/users/:user_id/logins/:login_id", admin, h.DeleteLogin)
	protected.Put("/accounts/:account_id/users/:user_id/observees/:observee_id", admin, h.LinkObservee)
	protected.Delete("/accounts/:account_id/users/:user_id/observees/:observee_id", admin, h.UnlinkObservee)

	// Account reports (G117)
	protected.Get("/accounts/:account_id/reports", admin, h.ListReports)
	protected.Get("/accounts/:account_id/reports/:report", admin, h.ListReportRuns)
	protected.Post("/accounts/:account_id/reports/:report", admin, h.StartReport)
	protected.Get("/accounts/:account_id/reports/:report/:id", admin, h.GetReportRun)
	protected.Get("/accounts/:account_id/reports/:report/:id/download", admin, h.DownloadReport)
	protected.Delete("/accounts/:account_id/reports/:report/:id", admin, h.DeleteReportRun)

	// Theme (G118). /brand_config is the caller's own account and is read
	// by every signed-in page.
	protected.Get("/brand_config", h.GetMyBrandConfig)
	protected.Get("/brand_config/logo", h.GetMyBrandLogo)
	protected.Get("/accounts/:account_id/brand_config", admin, h.GetBrandConfig)
	protected.Put("/accounts/:account_id/brand_config", admin, h.UpdateBrandConfig)
	protected.Delete("/accounts/:account_id/brand_config", admin, h.ResetBrandConfig)
	protected.Post("/accounts/:account_id/brand_config/logo", admin, h.UploadBrandLogo)
	protected.Delete("/accounts/:account_id/brand_config/logo", admin, h.DeleteBrandLogo)

	// Admin tools (G119)
	protected.Get("/accounts/:account_id/deleted_courses", admin, h.ListDeletedCourses)
	protected.Post("/accounts/:account_id/deleted_courses/:course_id/restore", admin, h.RestoreCourse)

	// Storage quotas (G120)
	protected.Get("/accounts/:account_id/storage_quotas", admin, h.GetQuotaDefaults)
	protected.Put("/accounts/:account_id/storage_quotas", admin, h.UpdateQuotaDefaults)
	protected.Get("/accounts/:account_id/storage_quotas/:context_type/:context_id", admin, h.GetContextQuota)
	protected.Put("/accounts/:account_id/storage_quotas/:context_type/:context_id", admin, h.SetContextQuota)
	protected.Get("/courses/:course_id/storage_quota", instructor, h.GetCourseQuota)

	// Custom roles (G102)
	protected.Get("/accounts/:account_id/role_base_permissions", admin, h.GetBaseRolePermissions)
	protected.Get("/courses/:course_id/assignable_roles", instructor, h.ListAssignableRoles)

	// SIS batch files (G111)
	if r.SISImportHandler != nil {
		protected.Get("/accounts/:account_id/sis_imports/:id/files", admin, r.SISImportHandler.ListSISImportFiles)
	}
}
