package handlers

import (
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/designdoc"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// MaxDesignColourSchemes is how many palettes one user may save.
const MaxDesignColourSchemes = 50

// DesignColourSchemeHandler serves a teacher's saved Page Designer
// palettes ("My colour schemes"):
//
//	GET    /users/self/design_colour_schemes
//	POST   /users/self/design_colour_schemes        {name, palette}
//	PUT    /users/self/design_colour_schemes/:id    {name?, palette?}
//	DELETE /users/self/design_colour_schemes/:id
//
// Rows belong to the caller: another user's (or tenant's) scheme id is a
// 404, never a 403.
type DesignColourSchemeHandler struct {
	repo repository.DesignColourSchemeRepository
}

func NewDesignColourSchemeHandler(repo repository.DesignColourSchemeRepository) *DesignColourSchemeHandler {
	return &DesignColourSchemeHandler{repo: repo}
}

func colourSchemeJSON(s *models.DesignColourScheme) fiber.Map {
	return fiber.Map{
		"id":         s.ID,
		"name":       s.Name,
		"palette":    s.Palette,
		"created_at": s.CreatedAt,
		"updated_at": s.UpdatedAt,
	}
}

type colourSchemeInput struct {
	Name    *string         `json:"name"`
	Palette json.RawMessage `json:"palette"`
}

// validateSchemeName trims and checks a scheme name.
func validateSchemeName(raw string) (string, bool) {
	name := strings.TrimSpace(raw)
	return name, name != "" && utf8.RuneCountInString(name) <= designdoc.MaxSchemeNameRunes
}

func (h *DesignColourSchemeHandler) List(c *fiber.Ctx) error {
	userID := actingUserID(c)
	if userID == 0 {
		return responses.Unauthorized(c)
	}
	rows, err := h.repo.ListByUser(c.Context(), userID, callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not fetch colour schemes")
	}
	out := make([]fiber.Map, len(rows))
	for i := range rows {
		out[i] = colourSchemeJSON(&rows[i])
	}
	return c.JSON(out)
}

func (h *DesignColourSchemeHandler) Create(c *fiber.Ctx) error {
	userID := actingUserID(c)
	if userID == 0 {
		return responses.Unauthorized(c)
	}
	var in colourSchemeInput
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	if in.Name == nil {
		return unprocessable(c, "name is required")
	}
	name, ok := validateSchemeName(*in.Name)
	if !ok {
		return unprocessable(c, "name must be 1-60 characters")
	}
	if len(in.Palette) == 0 {
		return unprocessable(c, "palette is required")
	}
	palette, err := designdoc.ValidatePalette(in.Palette)
	if err != nil {
		return unprocessable(c, err.Error())
	}
	scheme := &models.DesignColourScheme{
		AccountID: callerAccountID(c),
		UserID:    userID,
		Name:      name,
		Palette:   palette,
	}
	if err := h.repo.Create(c.Context(), scheme, MaxDesignColourSchemes); err != nil {
		if errors.Is(err, repository.ErrColourSchemeLimit) {
			return unprocessable(c, "You can save at most 50 colour schemes; delete one first")
		}
		return responses.InternalError(c, "Could not save colour scheme")
	}
	return c.Status(fiber.StatusCreated).JSON(colourSchemeJSON(scheme))
}

// load returns the caller's scheme :id, or writes a 404 (wrote=true).
func (h *DesignColourSchemeHandler) load(c *fiber.Ctx) (*models.DesignColourScheme, bool, error) {
	userID := actingUserID(c)
	if userID == 0 {
		return nil, true, responses.Unauthorized(c)
	}
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return nil, true, responses.NotFound(c, "colour scheme")
	}
	s, err := h.repo.FindByID(c.Context(), uint(id), userID, callerAccountID(c))
	if err != nil || s == nil {
		return nil, true, responses.NotFound(c, "colour scheme")
	}
	return s, false, nil
}

func (h *DesignColourSchemeHandler) Update(c *fiber.Ctx) error {
	scheme, wrote, err := h.load(c)
	if wrote {
		return err
	}
	var in colourSchemeInput
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	if in.Name != nil {
		name, ok := validateSchemeName(*in.Name)
		if !ok {
			return unprocessable(c, "name must be 1-60 characters")
		}
		scheme.Name = name
	}
	if len(in.Palette) > 0 {
		palette, err := designdoc.ValidatePalette(in.Palette)
		if err != nil {
			return unprocessable(c, err.Error())
		}
		scheme.Palette = palette
	}
	if err := h.repo.Update(c.Context(), scheme); err != nil {
		return responses.InternalError(c, "Could not save colour scheme")
	}
	return c.JSON(colourSchemeJSON(scheme))
}

func (h *DesignColourSchemeHandler) Delete(c *fiber.Ctx) error {
	scheme, wrote, err := h.load(c)
	if wrote {
		return err
	}
	if err := h.repo.Delete(c.Context(), scheme.ID, scheme.UserID, scheme.AccountID); err != nil {
		return responses.NotFound(c, "colour scheme")
	}
	return c.JSON(colourSchemeJSON(scheme))
}
