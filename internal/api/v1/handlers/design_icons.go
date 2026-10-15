package handlers

import (
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/designassets"
)

// DesignIcon serves GET /api/v1/design/icons/:style/:file, file being
// "<name>.svg", with ?fg=rrggbb&bg=rrggbb. Public: designed pages, and
// cartridges exported from them, load these as <img>. The body is a pure
// function of the URL, so it is cached for a year. Unknown names draw
// circle-dashed; an unknown style or a file that isn't .svg is a 404.
//
// The SVG is fully determined by the server (colours are validated hex,
// the geometry is embedded), and the response CSP forbids scripts anyway.
func DesignIcon(c *fiber.Ctx) error {
	style := c.Params("style")
	file := c.Params("file")
	if decoded, err := url.PathUnescape(file); err == nil {
		file = decoded
	}
	if !designassets.IsStyle(style) || !strings.HasSuffix(file, ".svg") {
		return responses.NotFound(c, "icon")
	}
	name := strings.TrimSuffix(file, ".svg")
	svg := designassets.RenderIconSVG(name, style, c.Query("fg"), c.Query("bg"))

	c.Set(fiber.HeaderContentType, "image/svg+xml")
	c.Set(fiber.HeaderCacheControl, "public, max-age=31536000, immutable")
	c.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	c.Set("X-Content-Type-Options", "nosniff")
	return c.SendString(svg)
}
