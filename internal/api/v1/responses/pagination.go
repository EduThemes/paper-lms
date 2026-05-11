package responses

import (
	"fmt"
	"math"

	"github.com/gofiber/fiber/v2"
)

func SetPaginationHeaders(c *fiber.Ctx, totalCount int64, page, perPage int) {
	totalPages := int(math.Ceil(float64(totalCount) / float64(perPage)))
	if totalPages < 1 {
		totalPages = 1
	}

	baseURL := c.BaseURL() + c.Path()
	var links []string

	links = append(links, fmt.Sprintf("<%s?page=%d&per_page=%d>; rel=\"current\"", baseURL, page, perPage))
	links = append(links, fmt.Sprintf("<%s?page=%d&per_page=%d>; rel=\"first\"", baseURL, 1, perPage))
	links = append(links, fmt.Sprintf("<%s?page=%d&per_page=%d>; rel=\"last\"", baseURL, totalPages, perPage))

	if page < totalPages {
		links = append(links, fmt.Sprintf("<%s?page=%d&per_page=%d>; rel=\"next\"", baseURL, page+1, perPage))
	}
	if page > 1 {
		links = append(links, fmt.Sprintf("<%s?page=%d&per_page=%d>; rel=\"prev\"", baseURL, page-1, perPage))
	}

	linkHeader := ""
	for i, link := range links {
		if i > 0 {
			linkHeader += ", "
		}
		linkHeader += link
	}
	c.Set("Link", linkHeader)
}
