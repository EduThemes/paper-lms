package handlers

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
	"github.com/EduThemes/paper-lms/internal/service/gamification/pseudonym"
)

// pseudonymPoolJSON is the public-facing pool descriptor. Word lists
// are NOT returned — the picker UI offers a small set of generated
// suggestions per pool instead. Exposing the full vocabulary would let
// a curious learner script the combinatorial space and engineer a
// specific name; not catastrophic but unnecessary.
type pseudonymPoolJSON struct {
	Code           string   `json:"code"`
	Label          string   `json:"label"`
	Description    string   `json:"description"`
	CandidateCount int      `json:"candidate_count"`
	Samples        []string `json:"samples"`
}

type pseudonymPoolsResponse struct {
	Pools        []pseudonymPoolJSON `json:"pools"`
	FirstNameAvailable bool            `json:"first_name_available"`
	CurrentPoolCode    string          `json:"current_pool_code,omitempty"`
	CurrentName        string          `json:"current_name,omitempty"`
}

// GetPseudonymPools returns the catalog of selectable pseudonym pools
// for the requesting learner in the given course. The returned set is
// gated by the tenant-mode render policy (e.g. K-5 doesn't expose the
// switch surface; FirstNameAvailable is false unless H912+).
//
// Includes a small set of sample names per pool so the picker UI can
// show "here's what your name could look like" without round-tripping
// per click.
func (h *GamificationHandler) GetPseudonymPools(c *fiber.Ctx) error {
	courseIDParam := c.Params("course_id")
	courseID64, err := strconv.ParseUint(courseIDParam, 10, 64)
	if err != nil || courseID64 == 0 {
		return responses.Error(c, fiber.StatusBadRequest, "invalid course_id")
	}
	courseID := uint(courseID64)

	viewerID, _ := c.Locals("user_id").(uint)
	if viewerID == 0 {
		return responses.Unauthorized(c)
	}

	enrollment, err := h.enrollmentRepo.FindByUserAndCourse(c.Context(), viewerID, courseID)
	if err != nil {
		return responses.InternalError(c, "failed to resolve enrollment")
	}
	if enrollment == nil || enrollment.WorkflowState != "active" {
		// 13.1.E: existence leak — return 404 not 403. A 403 reveals
		// the course exists (potentially in another tenant) to a non-
		// enrolled viewer.
		return responses.NotFound(c, "course")
	}

	policy := h.policyForViewerInCourse(c, enrollment)
	if !policy.LearnerCanSwitch {
		// True authorization failure: viewer IS enrolled in the course
		// but the tenant-mode render policy disables pseudonym
		// switching. 403 is correct here — no existence leak.
		return responses.Error(c, fiber.StatusForbidden, "pseudonym switching is not allowed in this course")
	}

	pools := make([]pseudonymPoolJSON, 0, len(pseudonym.Catalog()))
	for _, p := range pseudonym.Catalog() {
		samples := make([]string, 0, 5)
		for i := 0; i < 5; i++ {
			samples = append(samples, pseudonym.GenerateForEnrollment(p, enrollment.ID, i))
		}
		pools = append(pools, pseudonymPoolJSON{
			Code:           string(p.Code),
			Label:          p.Label,
			Description:    p.Description,
			CandidateCount: pseudonym.CandidateCount(p),
			Samples:        samples,
		})
	}

	return c.JSON(pseudonymPoolsResponse{
		Pools:              pools,
		FirstNameAvailable: policy.AllowFirstName,
		CurrentPoolCode:    enrollment.PseudonymPoolCode,
		CurrentName:        ptrToString(enrollment.PseudonymName),
	})
}

// pseudonymUpdateBody is the PUT payload. Either:
//
//   - {pool_code, name}: explicit selection from the pool's combinatorial
//     space. Server validates name ∈ pool.
//   - {pool_code, regenerate: true}: server rolls a fresh deterministic
//     name within the pool.
//   - {pool_code: "first_name"}: special-case, no `name` needed.
type pseudonymUpdateBody struct {
	PoolCode   string `json:"pool_code"`
	Name       string `json:"name"`
	Regenerate bool   `json:"regenerate"`
}

// UpdatePseudonymForSelf is the learner-facing switcher. Self-only —
// reads viewerID from Locals, never accepts another learner's id on
// the URL. Pool restriction enforced by the render policy
// (tenant-mode gate), name validated against the pool's vocabulary.
func (h *GamificationHandler) UpdatePseudonymForSelf(c *fiber.Ctx) error {
	courseIDParam := c.Params("course_id")
	courseID64, err := strconv.ParseUint(courseIDParam, 10, 64)
	if err != nil || courseID64 == 0 {
		return responses.Error(c, fiber.StatusBadRequest, "invalid course_id")
	}
	courseID := uint(courseID64)

	viewerID, _ := c.Locals("user_id").(uint)
	if viewerID == 0 {
		return responses.Unauthorized(c)
	}

	enrollment, err := h.enrollmentRepo.FindByUserAndCourse(c.Context(), viewerID, courseID)
	if err != nil {
		return responses.InternalError(c, "failed to resolve enrollment")
	}
	if enrollment == nil || enrollment.WorkflowState != "active" {
		// 13.1.E: existence leak — return 404 not 403. Same rationale
		// as GetPseudonymPools above: don't confirm course existence
		// to a non-enrolled viewer (potentially cross-tenant).
		return responses.NotFound(c, "course")
	}

	policy := h.policyForViewerInCourse(c, enrollment)
	if !policy.LearnerCanSwitch {
		// True authorization failure: viewer IS enrolled but the
		// tenant-mode policy gates pseudonym switching. 403 is correct.
		return responses.Error(c, fiber.StatusForbidden, "pseudonym switching is not allowed in this course")
	}

	var body pseudonymUpdateBody
	if err := c.BodyParser(&body); err != nil {
		return responses.BadRequest(c, "invalid request body")
	}
	body.PoolCode = strings.TrimSpace(body.PoolCode)
	body.Name = strings.TrimSpace(body.Name)
	if body.PoolCode == "" {
		return responses.BadRequest(c, "pool_code is required")
	}

	poolCode := pseudonym.PoolCode(body.PoolCode)

	// First-name short-circuit. No `name` needed; we store
	// pool_code='first_name' with an empty name and the renderer
	// special-cases to legal-name's first token.
	if poolCode == pseudonym.PoolFirstName {
		if !policy.AllowFirstName {
			return responses.Error(c, fiber.StatusForbidden, "first-name mode is not allowed in this course")
		}
		if err := h.enrollmentRepo.UpdatePseudonymForSelf(c.Context(), viewerID, courseID, string(poolCode), ""); err != nil {
			return responses.InternalError(c, "failed to set pseudonym")
		}
		return c.JSON(fiber.Map{"pool_code": string(poolCode), "name": ""})
	}

	// Resolve the pool.
	pool, err := pseudonym.PoolByCode(poolCode)
	if err != nil || pool == nil {
		return responses.BadRequest(c, "unknown pool_code: "+body.PoolCode)
	}

	// Either regenerate deterministically, or accept a learner-chosen
	// name after validation.
	var newName string
	if body.Regenerate {
		// Walk the attempt counter until we land in a free slot. The
		// pseudonym package's Generator does this via a callback that
		// returns (created, error); the repo translates UNIQUE
		// violations to ErrPseudonymTaken so we can iterate.
		gen := pseudonym.NewGenerator()
		name, err := gen.Generate(c.Context(), *pool, enrollment.ID, func(name string, attempt int) (bool, error) {
			err := h.enrollmentRepo.UpdatePseudonymForSelf(c.Context(), viewerID, courseID, string(poolCode), name)
			if errors.Is(err, repository.ErrPseudonymTaken) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			return true, nil
		})
		if err != nil {
			return responses.InternalError(c, "failed to allocate pseudonym")
		}
		newName = name
	} else {
		if body.Name == "" {
			return responses.BadRequest(c, "name is required when regenerate is false")
		}
		if err := pseudonym.Validate(*pool, body.Name); err != nil {
			return responses.BadRequest(c, err.Error())
		}
		if err := h.enrollmentRepo.UpdatePseudonymForSelf(c.Context(), viewerID, courseID, string(poolCode), body.Name); err != nil {
			if errors.Is(err, repository.ErrPseudonymTaken) {
				return responses.Error(c, fiber.StatusConflict, "another learner is already using that pseudonym in this course; pick another or regenerate")
			}
			return responses.InternalError(c, "failed to set pseudonym")
		}
		newName = body.Name
	}

	return c.JSON(fiber.Map{"pool_code": string(poolCode), "name": newName})
}

// ptrToString safely dereferences a *string, returning "" for nil.
func ptrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// policyForViewerInCourse condenses the (role + tenantMode) lookup the
// pseudonym handlers share with the leaderboard handler. Failures
// fall back to higher_ed defaults — see GetCourseLeaderboard for
// rationale.
//
// F3.3 / F3.4 closeout: previously this had its own inline role
// derivation that, unlike the leaderboard handler's
// resolveViewerRoleInCourse, did NOT fall back to the userRepo on a
// missing `is_admin` Locals. After F4.3's auth-middleware change the
// Locals is now always populated on a JWT login, but the userRepo
// fallback survives as defense-in-depth (handler tests + access-token
// path coverage). Both endpoints now route role decisions through
// the same helper.
func (h *GamificationHandler) policyForViewerInCourse(c *fiber.Ctx, viewerEnrollment *models.Enrollment) gamification.LeaderboardRenderPolicy {
	role := h.roleFromAdminOrEnrollment(c, viewerEnrollment)
	tenantMode := "higher_ed"
	if h.accountRepo != nil {
		if acc, err := h.accountRepo.FindByID(c.Context(), callerAccountID(c)); err == nil && acc != nil && acc.TenantMode != "" {
			tenantMode = string(acc.TenantMode)
		}
	}
	return gamification.RenderPolicyFor(tenantMode, role, 0)
}

// roleFromAdminOrEnrollment mirrors resolveViewerRoleInCourse's
// admin-detection logic for callers that have already fetched the
// enrollment row. Reads the is_admin Locals first; falls back to a
// userRepo lookup when the Locals is unset (which can happen if the
// route mounts without the permissions middleware chain).
func (h *GamificationHandler) roleFromAdminOrEnrollment(c *fiber.Ctx, viewerEnrollment *models.Enrollment) gamification.ViewerRole {
	isAdmin, _ := c.Locals("is_admin").(bool)
	if !isAdmin && h.userRepo != nil {
		if viewerID, ok := c.Locals("user_id").(uint); ok && viewerID > 0 {
			if user, err := h.userRepo.FindByID(c.Context(), viewerID); err == nil && user != nil && user.Role == "admin" {
				isAdmin = true
			}
		}
	}
	if isAdmin {
		return gamification.ViewerAdmin
	}
	if viewerEnrollment != nil && (viewerEnrollment.Type == "TeacherEnrollment" || viewerEnrollment.Type == "TaEnrollment") {
		return gamification.ViewerTeacher
	}
	return gamification.ViewerStudent
}
