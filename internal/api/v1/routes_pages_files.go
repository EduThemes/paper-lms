package v1

import (
	"context"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
)

// PagesFilesRoutes carries the pages / files / course-shell handlers that
// are not part of the core Router fields (Canvas-parity lane
// "pages-files"). Nil fields simply leave their routes unmounted.
type PagesFilesRoutes struct {
	// Page history (G065).
	Revisions *handlers.WikiPageRevisionHandler

	// File management, visibility and quota routes live on the core
	// FileHandler (G070-G072); this flag mounts them.
	FileManagement bool

	// Student View (G076).
	StudentView *handlers.StudentViewHandler

	// FolderCourseID maps a folder to its course (ok=false for user and
	// group folders) so the hidden Files tab also closes folder listings.
	FolderCourseID func(ctx context.Context, folderID, accountID uint) (uint, bool)
}

// registerPagesFilesRoutes mounts the pages-files lane routes. Core page,
// file and folder CRUD stays in router.go; everything here layers on it.
func (r *Router) registerPagesFilesRoutes(protected fiber.Router, enrolled, instructor fiber.Handler) {
	pf := r.PagesFiles
	if pf == nil {
		return
	}

	// Hidden course tabs (G078): each gate is its own route ahead of the
	// core list route and continues to it unless the course hides the tab
	// from this learner. Canvas gates the same index endpoints; single
	// items stay reachable (e.g. a page linked from a module).
	if r.PermMiddleware != nil {
		protected.Get("/courses/:course_id/pages", r.PermMiddleware.RequireCourseTab(middleware.CourseTabPages))
		protected.Get("/courses/:course_id/files", r.PermMiddleware.RequireCourseTab(middleware.CourseTabFiles))
		protected.Get("/courses/:course_id/folders", r.PermMiddleware.RequireCourseTab(middleware.CourseTabFiles))
		protected.Get("/courses/:course_id/quizzes", r.PermMiddleware.RequireCourseTab(middleware.CourseTabQuizzes))
		// A course folder's file and subfolder listings are the Files
		// index too; the folder names the course.
		if pf.FolderCourseID != nil {
			folderCourse := func(c *fiber.Ctx) (uint, bool) {
				id, err := c.ParamsInt("folder_id")
				if err != nil || id <= 0 {
					return 0, false
				}
				account, _ := c.Locals("account_id").(uint)
				return pf.FolderCourseID(c.Context(), uint(id), account)
			}
			protected.Get("/folders/:folder_id/files", r.PermMiddleware.RequireCourseTabFor(middleware.CourseTabFiles, folderCourse))
			protected.Get("/folders/:folder_id/folders", r.PermMiddleware.RequireCourseTabFor(middleware.CourseTabFiles, folderCourse))
		}
	}

	// Page revisions: the handler requires edit rights on the page to read
	// history and course-manager rights to revert.
	if pf.Revisions != nil {
		protected.Get("/courses/:course_id/pages/:url_or_id/revisions", enrolled, pf.Revisions.ListRevisions)
		protected.Get("/courses/:course_id/pages/:url_or_id/revisions/:revision_id", enrolled, pf.Revisions.GetRevision)
		protected.Post("/courses/:course_id/pages/:url_or_id/revisions/:revision_id", enrolled, pf.Revisions.RevertToRevision)
	}

	// Files: quota (registered ahead of GET /files/:id, which would
	// otherwise swallow "quota"), rename / move / visibility / usage
	// rights, and replace-in-place.
	if pf.FileManagement && r.FileHandler != nil {
		protected.Get("/courses/:course_id/files/quota", instructor, r.FileHandler.GetCourseQuota)
		protected.Put("/courses/:course_id/files/:id", instructor, r.FileHandler.UpdateCourseFile)
		protected.Post("/courses/:course_id/files/:id/replace", middleware.UploadRateLimit(), middleware.EnforceUploadSize(r.SettingsLookup), instructor, r.FileHandler.ReplaceCourseFile)
	}

	// Student View: staff enter; reset works from outside (staff) or from
	// inside Student View (the handler checks which).
	if pf.StudentView != nil {
		protected.Post("/courses/:course_id/student_view", instructor, pf.StudentView.Enter)
		protected.Delete("/courses/:course_id/student_view", enrolled, pf.StudentView.Reset)
		protected.Get("/users/self/student_view", pf.StudentView.Status)
	}
}
