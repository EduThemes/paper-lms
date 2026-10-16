package v1

import (
	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
)

// registerDesignPublicRoutes mounts the Page Designer's public icon route
// (docs/page-designer/README.md, "API"). No auth, no CSRF: designed pages
// and Common Cartridge exports load these as <img> from any origin. The
// response is a pure function of the URL and cached for a year.
func (r *Router) registerDesignPublicRoutes(api fiber.Router) {
	api.Get("/design/icons/:style/:file", handlers.DesignIcon)
}

// registerDesignRoutes mounts the authenticated Page Designer routes: a
// teacher's saved colour schemes.
func (r *Router) registerDesignRoutes(protected fiber.Router) {
	h := r.DesignColourSchemeHandler
	if h == nil {
		return
	}
	protected.Get("/users/self/design_colour_schemes", h.List)
	protected.Post("/users/self/design_colour_schemes", h.Create)
	protected.Put("/users/self/design_colour_schemes/:id", h.Update)
	protected.Delete("/users/self/design_colour_schemes/:id", h.Delete)
}
