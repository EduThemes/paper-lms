package handlers

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
)

// pseudonymPoolJSON is the public-facing pool descriptor. Word lists are NOT
// returned — the picker UI offers a small set of generated suggestions per
// pool instead.
type pseudonymPoolJSON struct {
	Code           string   `json:"code"`
	Label          string   `json:"label"`
	Description    string   `json:"description"`
	CandidateCount int      `json:"candidate_count"`
	Samples        []string `json:"samples"`
}

type pseudonymPoolsResponse struct {
	Pools              []pseudonymPoolJSON `json:"pools"`
	FirstNameAvailable bool                `json:"first_name_available"`
	CurrentPoolCode    string              `json:"current_pool_code,omitempty"`
	CurrentName        string              `json:"current_name,omitempty"`
}

// GetPseudonymPools returns the catalog of selectable pseudonym pools.
func (h *GamificationHandler) GetPseudonymPools(c *fiber.Ctx) error {
	courseID64, err := strconv.ParseUint(c.Params("course_id"), 10, 64)
	if err != nil || courseID64 == 0 {
		return responses.Error(c, fiber.StatusBadRequest, "invalid course_id")
	}
	viewerID, _ := c.Locals("user_id").(uint)
	if viewerID == 0 {
		return responses.Unauthorized(c)
	}
	isAdmin, _ := c.Locals("is_admin").(bool)

	pools, firstNameAvail, enrollment, err := h.leaderboardService.PseudonymCatalogForViewer(c.Context(), viewerID, uint(courseID64), callerAccountID(c), isAdmin)
	if err != nil {
		return mapPseudonymServiceError(c, err)
	}

	jsonPools := make([]pseudonymPoolJSON, 0, len(pools))
	for _, p := range pools {
		jsonPools = append(jsonPools, pseudonymPoolJSON{
			Code:           p.Code,
			Label:          p.Label,
			Description:    p.Description,
			CandidateCount: p.CandidateCount,
			Samples:        p.Samples,
		})
	}
	return c.JSON(pseudonymPoolsResponse{
		Pools:              jsonPools,
		FirstNameAvailable: firstNameAvail,
		CurrentPoolCode:    enrollment.PseudonymPoolCode,
		CurrentName:        ptrToString(enrollment.PseudonymName),
	})
}

type pseudonymUpdateBody struct {
	PoolCode   string `json:"pool_code"`
	Name       string `json:"name"`
	Regenerate bool   `json:"regenerate"`
}

// UpdatePseudonymForSelf is the learner-facing switcher.
func (h *GamificationHandler) UpdatePseudonymForSelf(c *fiber.Ctx) error {
	courseID64, err := strconv.ParseUint(c.Params("course_id"), 10, 64)
	if err != nil || courseID64 == 0 {
		return responses.Error(c, fiber.StatusBadRequest, "invalid course_id")
	}
	viewerID, _ := c.Locals("user_id").(uint)
	if viewerID == 0 {
		return responses.Unauthorized(c)
	}
	isAdmin, _ := c.Locals("is_admin").(bool)

	var body pseudonymUpdateBody
	if err := c.BodyParser(&body); err != nil {
		return responses.BadRequest(c, "invalid request body")
	}
	body.PoolCode = strings.TrimSpace(body.PoolCode)
	body.Name = strings.TrimSpace(body.Name)
	if body.PoolCode == "" {
		return responses.BadRequest(c, "pool_code is required")
	}

	result, err := h.leaderboardService.UpdatePseudonymForSelf(c.Context(), viewerID, uint(courseID64), callerAccountID(c), isAdmin, gamification.PseudonymUpdateRequest{
		PoolCode:   body.PoolCode,
		Name:       body.Name,
		Regenerate: body.Regenerate,
	})
	if err != nil {
		return mapPseudonymServiceError(c, err)
	}
	return c.JSON(fiber.Map{"pool_code": result.PoolCode, "name": result.Name})
}

func mapPseudonymServiceError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, gamification.ErrCourseLeaderboardNotEnrolled):
		// 13.1.E: existence leak — return 404 not 403 on non-enrolled.
		// A 403 confirms the course exists (potentially cross-tenant)
		// to a non-enrolled viewer.
		return responses.NotFound(c, "course")
	case errors.Is(err, gamification.ErrPseudonymSwitchForbidden):
		return responses.Error(c, fiber.StatusForbidden, "pseudonym switching is not allowed in this course")
	case errors.Is(err, gamification.ErrFirstNameNotAllowed):
		return responses.Error(c, fiber.StatusForbidden, "first-name mode is not allowed in this course")
	case errors.Is(err, repository.ErrPseudonymTaken):
		return responses.Error(c, fiber.StatusConflict, "another learner is already using that pseudonym in this course; pick another or regenerate")
	default:
		if err == nil {
			return responses.InternalError(c, "pseudonym operation failed")
		}
		// Unknown pool_code, validation, etc. → 400
		return responses.BadRequest(c, err.Error())
	}
}

// ptrToString safely dereferences a *string, returning "" for nil.
func ptrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
