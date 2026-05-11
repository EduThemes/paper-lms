package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

// COPPAHandler handles HTTP requests for COPPA 2025 compliance endpoints.
type COPPAHandler struct {
	coppaService *service.COPPAService
}

// NewCOPPAHandler creates a new COPPAHandler.
func NewCOPPAHandler(coppaService *service.COPPAService) *COPPAHandler {
	return &COPPAHandler{coppaService: coppaService}
}

func parentalConsentToJSON(consent *models.ParentalConsent) fiber.Map {
	return fiber.Map{
		"id":             consent.ID,
		"student_id":     consent.StudentID,
		"parent_user_id": consent.ParentUserID,
		"parent_name":    consent.ParentName,
		"parent_email":   consent.ParentEmail,
		"consent_type":   consent.ConsentType,
		"status":         consent.Status,
		"consent_method": consent.ConsentMethod,
		"consented_at":   consent.ConsentedAt,
		"revoked_at":     consent.RevokedAt,
		"expires_at":     consent.ExpiresAt,
		"notes":          consent.Notes,
		"created_at":     consent.CreatedAt,
		"updated_at":     consent.UpdatedAt,
	}
}

func dpaToJSON(dpa *models.DataProcessingAgreement) fiber.Map {
	return fiber.Map{
		"id":               dpa.ID,
		"account_id":       dpa.AccountID,
		"vendor_name":      dpa.VendorName,
		"vendor_contact":   dpa.VendorContact,
		"purpose":          dpa.Purpose,
		"data_categories":  dpa.DataCategories,
		"retention_period": dpa.RetentionPeriod,
		"status":           dpa.Status,
		"signed_at":        dpa.SignedAt,
		"expires_at":       dpa.ExpiresAt,
		"document_url":     dpa.DocumentURL,
		"created_at":       dpa.CreatedAt,
		"updated_at":       dpa.UpdatedAt,
	}
}

// RequestConsent handles POST /api/v1/users/:user_id/parental_consent
func (h *COPPAHandler) RequestConsent(c *fiber.Ctx) error {
	userID, err := strconv.Atoi(c.Params("user_id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	var input struct {
		ParentEmail string `json:"parent_email"`
		ConsentType string `json:"consent_type"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	consent, err := h.coppaService.RequestParentalConsent(c.Context(), uint(userID), input.ParentEmail, input.ConsentType)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(parentalConsentToJSON(consent))
}

// ListConsents handles GET /api/v1/users/:user_id/parental_consent
func (h *COPPAHandler) ListConsents(c *fiber.Ctx) error {
	userID, err := strconv.Atoi(c.Params("user_id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	consents, err := h.coppaService.ListConsentsForStudent(c.Context(), uint(userID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch consent records")
	}

	result := make([]fiber.Map, len(consents))
	for i, consent := range consents {
		result[i] = parentalConsentToJSON(&consent)
	}

	return c.JSON(result)
}

// VerifyConsent handles POST /api/v1/parental_consent/verify/:token
func (h *COPPAHandler) VerifyConsent(c *fiber.Ctx) error {
	token := c.Params("token")
	if token == "" {
		return responses.BadRequest(c, "Verification token is required")
	}

	consent, err := h.coppaService.VerifyConsent(c.Context(), token)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(parentalConsentToJSON(consent))
}

// RevokeConsent handles DELETE /api/v1/parental_consent/:id
func (h *COPPAHandler) RevokeConsent(c *fiber.Ctx) error {
	consentID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid consent ID")
	}

	userID, _ := c.Locals("user_id").(uint)

	if err := h.coppaService.RevokeConsent(c.Context(), uint(consentID), userID); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{"revoked": true})
}

// ListDPAs handles GET /api/v1/accounts/:account_id/data_processing_agreements
func (h *COPPAHandler) ListDPAs(c *fiber.Ctx) error {
	accountID, err := strconv.Atoi(c.Params("account_id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.coppaService.ListDPAs(c.Context(), uint(accountID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch data processing agreements")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	dpas := make([]fiber.Map, len(result.Items))
	for i, dpa := range result.Items {
		dpas[i] = dpaToJSON(&dpa)
	}

	return c.JSON(dpas)
}

// CreateDPA handles POST /api/v1/accounts/:account_id/data_processing_agreements
func (h *COPPAHandler) CreateDPA(c *fiber.Ctx) error {
	accountID, err := strconv.Atoi(c.Params("account_id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	var input struct {
		VendorName      string `json:"vendor_name"`
		VendorContact   string `json:"vendor_contact"`
		Purpose         string `json:"purpose"`
		DataCategories  string `json:"data_categories"`
		RetentionPeriod string `json:"retention_period"`
		DocumentURL     string `json:"document_url"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	dpa := &models.DataProcessingAgreement{
		AccountID:       uint(accountID),
		VendorName:      input.VendorName,
		VendorContact:   input.VendorContact,
		Purpose:         input.Purpose,
		DataCategories:  input.DataCategories,
		RetentionPeriod: input.RetentionPeriod,
		DocumentURL:     input.DocumentURL,
	}

	if err := h.coppaService.CreateDPA(c.Context(), dpa); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(dpaToJSON(dpa))
}

// UpdateDPA handles PUT /api/v1/data_processing_agreements/:id
func (h *COPPAHandler) UpdateDPA(c *fiber.Ctx) error {
	dpaID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid DPA ID")
	}

	dpa, err := h.coppaService.GetDPA(c.Context(), uint(dpaID))
	if err != nil {
		return responses.NotFound(c, "data processing agreement")
	}

	var input struct {
		VendorName      *string `json:"vendor_name"`
		VendorContact   *string `json:"vendor_contact"`
		Purpose         *string `json:"purpose"`
		DataCategories  *string `json:"data_categories"`
		RetentionPeriod *string `json:"retention_period"`
		Status          *string `json:"status"`
		DocumentURL     *string `json:"document_url"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.VendorName != nil {
		dpa.VendorName = *input.VendorName
	}
	if input.VendorContact != nil {
		dpa.VendorContact = *input.VendorContact
	}
	if input.Purpose != nil {
		dpa.Purpose = *input.Purpose
	}
	if input.DataCategories != nil {
		dpa.DataCategories = *input.DataCategories
	}
	if input.RetentionPeriod != nil {
		dpa.RetentionPeriod = *input.RetentionPeriod
	}
	if input.Status != nil {
		dpa.Status = *input.Status
	}
	if input.DocumentURL != nil {
		dpa.DocumentURL = *input.DocumentURL
	}

	if err := h.coppaService.UpdateDPA(c.Context(), dpa); err != nil {
		return responses.InternalError(c, "Could not update data processing agreement")
	}

	return c.JSON(dpaToJSON(dpa))
}
