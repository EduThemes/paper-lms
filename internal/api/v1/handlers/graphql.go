package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/graphql"
)

// GraphQLHandler handles GraphQL HTTP requests.
type GraphQLHandler struct {
	resolver *graphql.Resolver
}

// NewGraphQLHandler creates a new GraphQL handler.
func NewGraphQLHandler(resolver *graphql.Resolver) *GraphQLHandler {
	return &GraphQLHandler{resolver: resolver}
}

// HandleQuery handles POST requests to the GraphQL endpoint.
// It expects a JSON body with query, variables, and operationName fields.
func (h *GraphQLHandler) HandleQuery(c *fiber.Ctx) error {
	// Get authenticated user ID from context
	userID, ok := c.Locals("user_id").(uint)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(graphql.Response{
			Errors: []graphql.GraphQLError{
				{Message: "authentication required"},
			},
		})
	}

	// Parse the GraphQL request
	var req graphql.Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(graphql.Response{
			Errors: []graphql.GraphQLError{
				{Message: "invalid request body: " + err.Error()},
			},
		})
	}

	if req.Query == "" {
		return c.Status(fiber.StatusBadRequest).JSON(graphql.Response{
			Errors: []graphql.GraphQLError{
				{Message: "query is required"},
			},
		})
	}

	// Execute the query
	resp := h.resolver.Resolve(c.Context(), userID, req.Query, req.Variables)

	// Return the response with appropriate status code
	if resp.Data == nil && len(resp.Errors) > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(resp)
	}

	return c.JSON(resp)
}
