package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"time"
)

type EnrollmentTermHandler struct {
	termService *service.EnrollmentTermService
}

func NewEnrollmentTermHandler(termService *service.EnrollmentTermService) *EnrollmentTermHandler {
	return &EnrollmentTermHandler{termService: termService}
}

func enrollmentTermToJSON(term *models.EnrollmentTerm) fiber.Map {
	return fiber.Map{
		"id":                      term.ID,
		"account_id":              term.AccountID,
		"name":                    term.Name,
		"sis_term_id":             term.SISTermID,
		"start_at":                term.StartAt,
		"end_at":                  term.EndAt,
		"grading_period_group_id": term.GradingPeriodGroupID,
		"workflow_state":          term.WorkflowState,
		"created_at":              term.CreatedAt,
		"updated_at":              term.UpdatedAt,
	}
}

func (h *EnrollmentTermHandler) ListTerms(c *fiber.Ctx) error {
	accountID, err := c.ParamsInt("account_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.termService.ListTerms(c.Context(), uint(accountID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch enrollment terms")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	terms := make([]fiber.Map, len(result.Items))
	for i, t := range result.Items {
		terms[i] = enrollmentTermToJSON(&t)
	}

	return c.JSON(fiber.Map{"enrollment_terms": terms})
}

func (h *EnrollmentTermHandler) CreateTerm(c *fiber.Ctx) error {
	accountID, err := c.ParamsInt("account_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	// user_id verified by auth middleware

	var input struct {
		EnrollmentTerm struct {
			Name                 string  `json:"name"`
			SISTermID            string  `json:"sis_term_id"`
			StartAt              *string `json:"start_at"`
			EndAt                *string `json:"end_at"`
			GradingPeriodGroupID *uint   `json:"grading_period_group_id"`
		} `json:"enrollment_term"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	term := &models.EnrollmentTerm{
		AccountID:            uint(accountID),
		Name:                 input.EnrollmentTerm.Name,
		SISTermID:            input.EnrollmentTerm.SISTermID,
		GradingPeriodGroupID: input.EnrollmentTerm.GradingPeriodGroupID,
	}

	if input.EnrollmentTerm.StartAt != nil {
		t, err := time.Parse(time.RFC3339, *input.EnrollmentTerm.StartAt)
		if err != nil {
			return responses.BadRequest(c, "Invalid start_at date format, use RFC3339")
		}
		term.StartAt = &t
	}

	if input.EnrollmentTerm.EndAt != nil {
		t, err := time.Parse(time.RFC3339, *input.EnrollmentTerm.EndAt)
		if err != nil {
			return responses.BadRequest(c, "Invalid end_at date format, use RFC3339")
		}
		term.EndAt = &t
	}

	if err := h.termService.CreateTerm(c.Context(), term); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(enrollmentTermToJSON(term))
}

// loadTermInAccount loads the :id term and ties it to the URL :account_id
// and the caller's tenant. GetTerm loads by bare id, so without this tie an
// admin of tenant B could read, edit or delete tenant A's terms via
// /accounts/B/terms/<A term id>. Every failure is the same 404
// (existence-leak contract). Returns wrote=true after writing a response;
// the caller MUST return nil in that case.
func (h *EnrollmentTermHandler) loadTermInAccount(c *fiber.Ctx) (*models.EnrollmentTerm, bool) {
	accountID, err := c.ParamsInt("account_id")
	if err != nil || accountID <= 0 {
		_ = responses.BadRequest(c, "Invalid account ID")
		return nil, true
	}
	termID, err := c.ParamsInt("id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid term ID")
		return nil, true
	}
	term, err := h.termService.GetTerm(c.Context(), uint(termID))
	if err != nil || term == nil || term.AccountID != uint(accountID) {
		_ = responses.NotFound(c, "enrollment term")
		return nil, true
	}
	if assertSameTenant(c, term.AccountID) {
		return nil, true
	}
	return term, false
}

func (h *EnrollmentTermHandler) GetTerm(c *fiber.Ctx) error {
	term, wrote := h.loadTermInAccount(c)
	if wrote {
		return nil
	}

	return c.JSON(enrollmentTermToJSON(term))
}

func (h *EnrollmentTermHandler) UpdateTerm(c *fiber.Ctx) error {
	// user_id verified by auth middleware
	term, wrote := h.loadTermInAccount(c)
	if wrote {
		return nil
	}

	var input struct {
		EnrollmentTerm struct {
			Name                 *string `json:"name"`
			SISTermID            *string `json:"sis_term_id"`
			StartAt              *string `json:"start_at"`
			EndAt                *string `json:"end_at"`
			GradingPeriodGroupID *uint   `json:"grading_period_group_id"`
		} `json:"enrollment_term"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.EnrollmentTerm.Name != nil {
		term.Name = *input.EnrollmentTerm.Name
	}
	if input.EnrollmentTerm.SISTermID != nil {
		term.SISTermID = *input.EnrollmentTerm.SISTermID
	}
	if input.EnrollmentTerm.GradingPeriodGroupID != nil {
		term.GradingPeriodGroupID = input.EnrollmentTerm.GradingPeriodGroupID
	}
	if input.EnrollmentTerm.StartAt != nil {
		t, err := time.Parse(time.RFC3339, *input.EnrollmentTerm.StartAt)
		if err != nil {
			return responses.BadRequest(c, "Invalid start_at date format, use RFC3339")
		}
		term.StartAt = &t
	}
	if input.EnrollmentTerm.EndAt != nil {
		t, err := time.Parse(time.RFC3339, *input.EnrollmentTerm.EndAt)
		if err != nil {
			return responses.BadRequest(c, "Invalid end_at date format, use RFC3339")
		}
		term.EndAt = &t
	}

	if err := h.termService.UpdateTerm(c.Context(), term); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(enrollmentTermToJSON(term))
}

func (h *EnrollmentTermHandler) DeleteTerm(c *fiber.Ctx) error {
	// user_id verified by auth middleware
	term, wrote := h.loadTermInAccount(c)
	if wrote {
		return nil
	}

	if err := h.termService.DeleteTerm(c.Context(), term.ID); err != nil {
		return responses.InternalError(c, "Could not delete enrollment term")
	}

	return c.JSON(fiber.Map{"delete": true})
}

func (h *EnrollmentTermHandler) GetCurrentTerm(c *fiber.Ctx) error {
	accountID, err := c.ParamsInt("account_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	term, err := h.termService.GetCurrentTerm(c.Context(), uint(accountID))
	if err != nil {
		return responses.NotFound(c, "current enrollment term")
	}

	return c.JSON(enrollmentTermToJSON(term))
}
