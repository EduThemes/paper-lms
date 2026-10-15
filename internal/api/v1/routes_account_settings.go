package v1

import "github.com/gofiber/fiber/v2"

// registerAccountSettingsRoutes mounts the account-admin settings API: the
// Settings Engine keys a tenant admin may set for their own account
// (settings.Definition.AccountAdminEditable). `admin` (RequireAdmin) ties
// :account_id to the caller's tenant (404 otherwise; a super_admin crosses
// tenants by design) and refuses non-admins (403).
func (r *Router) registerAccountSettingsRoutes(protected fiber.Router, admin fiber.Handler) {
	h := r.AccountSettingsHandler
	if h == nil {
		return
	}
	protected.Get("/accounts/:account_id/settings", admin, h.List)
	protected.Put("/accounts/:account_id/settings/:key", admin, h.Update)
}
