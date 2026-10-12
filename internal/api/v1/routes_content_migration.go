package v1

import (
	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
)

// registerContentMigrationRoutes mounts the content-migration lane:
// course content exports (IMS Common Cartridge), the course-copy source
// picker and selective-copy data, and blueprint item locking.
//
// It is called BEFORE the core /content_migrations/:id routes so the
// literal /content_migrations/selectable and /copy_sources paths are not
// swallowed by the :id route.
func (r *Router) registerContentMigrationRoutes(protected fiber.Router, instructor fiber.Handler) {
	if r.ContentExportHandler != nil {
		protected.Get("/courses/:course_id/content_exports", instructor, r.ContentExportHandler.ListExports)
		protected.Post("/courses/:course_id/content_exports", middleware.ExpensiveOpRateLimit(), instructor, r.ContentExportHandler.ExportCourse)
		protected.Get("/courses/:course_id/content_exports/:id", instructor, r.ContentExportHandler.GetExport)
		protected.Get("/courses/:course_id/content_exports/:id/download", instructor, r.ContentExportHandler.DownloadExport)
	}

	if r.ContentMigrationHandler != nil {
		protected.Get("/courses/:course_id/content_migrations/selectable", instructor, r.ContentMigrationHandler.SelectableContent)
		protected.Get("/courses/:course_id/content_migrations/copy_sources", instructor, r.ContentMigrationHandler.CopySources)
	}

	if r.BlueprintHandler != nil {
		protected.Get("/courses/:course_id/blueprint_templates/default/content", instructor, r.BlueprintHandler.ListLockableContent)
		protected.Put("/courses/:course_id/blueprint_templates/default/restrict_item", instructor, r.BlueprintHandler.RestrictItem)
		protected.Get("/courses/:course_id/blueprint_subscriptions/locked_content", instructor, r.BlueprintHandler.ListChildLockedContent)
	}
}
