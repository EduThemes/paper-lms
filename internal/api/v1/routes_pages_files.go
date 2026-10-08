package v1

import (
	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
)

// PagesFilesRoutes carries the pages / files / course-shell handlers that
// are not part of the core Router fields (Canvas-parity lane
// "pages-files"). Nil fields simply leave their routes unmounted.
type PagesFilesRoutes struct {
	// Page history (G065).
	Revisions *handlers.WikiPageRevisionHandler
}

// registerPagesFilesRoutes mounts the pages-files lane routes. Core page,
// file and folder CRUD stays in router.go; everything here layers on it.
func (r *Router) registerPagesFilesRoutes(protected fiber.Router, enrolled, instructor fiber.Handler) {
	pf := r.PagesFiles
	if pf == nil {
		return
	}

	// Page revisions: the handler requires edit rights on the page to read
	// history and course-manager rights to revert.
	if pf.Revisions != nil {
		protected.Get("/courses/:course_id/pages/:url_or_id/revisions", enrolled, pf.Revisions.ListRevisions)
		protected.Get("/courses/:course_id/pages/:url_or_id/revisions/:revision_id", enrolled, pf.Revisions.GetRevision)
		protected.Post("/courses/:course_id/pages/:url_or_id/revisions/:revision_id", enrolled, pf.Revisions.RevertToRevision)
	}
	_ = instructor
}
