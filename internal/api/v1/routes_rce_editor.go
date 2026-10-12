package v1

import "github.com/gofiber/fiber/v2"

// registerRCEEditorRoutes mounts the rich content editor's own endpoints.
// Today: the per-user key the editor encrypts its local autosave drafts
// with (G012). Self-only by construction (it derives from the caller's
// session), so no permission middleware beyond authentication applies.
func (r *Router) registerRCEEditorRoutes(protected fiber.Router) {
	if r.RCEDraftKeyHandler != nil {
		protected.Get("/users/self/rce_draft_key", r.RCEDraftKeyHandler.GetDraftKey)
	}
}
