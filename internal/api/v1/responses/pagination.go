package responses

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// SetPaginationHeaders writes a Canvas-style Link header (current, first,
// last, next, prev). Every link keeps the request's original query string —
// filters such as type[]=StudentEnrollment, include[], search_term — and only
// replaces page / per_page, like Canvas's Api.build_links. Rebuilding the
// URL from page + per_page alone made clients that follow rel="next" (the
// documented way to page a Canvas API) get unfiltered data from page 2 on.
func SetPaginationHeaders(c *fiber.Ctx, totalCount int64, page, perPage int) {
	if perPage < 1 {
		perPage = 1
	}
	totalPages := int(math.Ceil(float64(totalCount) / float64(perPage)))
	if totalPages < 1 {
		totalPages = 1
	}

	baseURL := c.BaseURL() + c.Path()

	// Preserve the original query parameters, in order, minus page/per_page.
	var kept []string
	c.Request().URI().QueryArgs().VisitAll(func(key, value []byte) {
		k := string(key)
		if k == "page" || k == "per_page" {
			return
		}
		kept = append(kept, url.QueryEscape(k)+"="+url.QueryEscape(string(value)))
	})
	prefix := ""
	if len(kept) > 0 {
		prefix = strings.Join(kept, "&") + "&"
	}

	link := func(p int, rel string) string {
		return fmt.Sprintf("<%s?%spage=%s&per_page=%s>; rel=\"%s\"", baseURL, prefix, strconv.Itoa(p), strconv.Itoa(perPage), rel)
	}

	links := []string{
		link(page, "current"),
		link(1, "first"),
		link(totalPages, "last"),
	}
	if page < totalPages {
		links = append(links, link(page+1, "next"))
	}
	if page > 1 {
		links = append(links, link(page-1, "prev"))
	}

	c.Set("Link", strings.Join(links, ", "))
}
