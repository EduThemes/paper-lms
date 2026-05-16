package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

// PairingCodeHandler exposes the parent/observer pairing-code endpoints.
type PairingCodeHandler struct {
	pairingService *service.PairingCodeService
}

func NewPairingCodeHandler(pairingService *service.PairingCodeService) *PairingCodeHandler {
	return &PairingCodeHandler{pairingService: pairingService}
}

func pairingCodeToJSON(pc *models.PairingCode) fiber.Map {
	return fiber.Map{
		"id":          pc.ID,
		"code":        pc.Code,
		"user_id":     pc.UserID,
		"created_at":  pc.CreatedAt,
		"expires_at":  pc.ExpiresAt,
		"redeemed_at": pc.RedeemedAt,
	}
}

// Generate handles POST /users/self/pairing_codes
// The authenticated student creates a new pairing code for themselves.
func (h *PairingCodeHandler) Generate(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	pc, gerr := h.pairingService.Generate(c.Context(), userID, 0)
	if gerr != nil {
		return responses.BadRequest(c, gerr.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(pairingCodeToJSON(pc))
}

// Redeem handles POST /users/self/pairing_codes/redeem
// Body: { "code": "ABC-123-XYZ" }
// The authenticated user (the prospective observer) redeems the code.
func (h *PairingCodeHandler) Redeem(c *fiber.Ctx) error {
	observerID, err := getUserID(c)
	if err != nil {
		return err
	}

	var input struct {
		Code string `json:"code"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	if input.Code == "" {
		return responses.BadRequest(c, "code is required")
	}

	pc, rerr := h.pairingService.Redeem(c.Context(), input.Code, observerID)
	if rerr != nil {
		return responses.BadRequest(c, rerr.Error())
	}

	return c.JSON(fiber.Map{
		"redeemed":    true,
		"observer_id": observerID,
		"observee_id": pc.UserID,
		"code":        pc.Code,
	})
}

// List handles GET /users/self/pairing_codes
// Returns the authenticated student's active (unredeemed, unexpired) codes.
func (h *PairingCodeHandler) List(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	codes, lerr := h.pairingService.ListActiveForStudent(c.Context(), userID)
	if lerr != nil {
		return responses.InternalError(c, "Could not fetch pairing codes")
	}
	out := make([]fiber.Map, len(codes))
	for i := range codes {
		out[i] = pairingCodeToJSON(&codes[i])
	}
	return c.JSON(out)
}

// MintForStudent handles POST /users/:student_id/observer-pairing-codes.
//
// Authorization (item 12.6 consent rule, enforced inside the service):
//   - A teacher in any of the student's active courses may mint.
//   - The student themselves may mint only when EVERY course they're
//     enrolled in lives under an account whose tenant_mode is adult-mode
//     (higher_ed / corp / pro). K-12 students must use the teacher path.
//
// Pre-12.6 the parent-link path (POST /users/:user_id/observees) took
// observee_id directly with no verification. This route is the
// teacher-mediated counterpart of POST /users/self/pairing_codes.
func (h *PairingCodeHandler) MintForStudent(c *fiber.Ctx) error {
	studentID, perr := c.ParamsInt("student_id")
	if perr != nil || studentID <= 0 {
		return responses.BadRequest(c, "Invalid student ID")
	}
	callerID, err := getUserID(c)
	if err != nil {
		return err
	}

	pc, gerr := h.pairingService.GenerateForStudent(c.Context(), callerID, uint(studentID), 0)
	if gerr != nil {
		if errors.Is(gerr, service.ErrPairingMintForbidden) {
			return responses.Forbidden(c, "not authorized to mint a pairing code for this student")
		}
		return responses.BadRequest(c, gerr.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(pairingCodeToJSON(pc))
}

// Revoke handles DELETE /users/self/pairing_codes/:id
func (h *PairingCodeHandler) Revoke(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	id, perr := c.ParamsInt("id")
	if perr != nil || id <= 0 {
		return responses.BadRequest(c, "Invalid pairing code id")
	}
	if rerr := h.pairingService.Revoke(c.Context(), userID, uint(id)); rerr != nil {
		return responses.BadRequest(c, rerr.Error())
	}
	return c.JSON(fiber.Map{"id": id, "deleted": true})
}
