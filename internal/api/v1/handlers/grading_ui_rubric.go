package handlers

import (
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/gofiber/fiber/v2"
)

// Rubric assessment by anonymous id: while an assignment anonymizes its
// students SpeedGrader has no user id to pass to the rubric assessment
// API, so it reads and saves the grading assessment through the
// anonymous submission instead (Canvas assesses by anonymous_id too).

// SetRubricService enables the anonymous rubric assessment routes.
func (h *GradingUIHandler) SetRubricService(r *service.RubricService) { h.rubricService = r }

// anonymousRubricAssessmentJSON renders an assessment without its
// student's user id.
func anonymousRubricAssessmentJSON(a *models.RubricAssessment, anonymousID string) fiber.Map {
	return fiber.Map{
		"id":                    a.ID,
		"rubric_id":             a.RubricID,
		"rubric_association_id": a.RubricAssociationID,
		"anonymous_id":          anonymousID,
		"score":                 a.Score,
		"data":                  a.Data,
		"assessment_type":       a.AssessmentType,
		"updated_at":            a.UpdatedAt,
	}
}

// anonymousGradingAssessment loads the assignment's rubric association
// and the student's grading assessment (nil when none yet). (result,
// wrote).
func (h *GradingUIHandler) anonymousGradingAssessment(c *fiber.Ctx, assignment *models.Assignment, sub *models.Submission) (*models.RubricAssociation, *models.RubricAssessment, bool) {
	if h.rubricService == nil {
		_ = responses.NotFound(c, "rubric")
		return nil, nil, true
	}
	_, assoc, err := h.rubricService.GetRubricForAssignment(c.Context(), assignment.ID, callerAccountID(c))
	if err != nil || assoc == nil {
		_ = responses.NotFound(c, "rubric")
		return nil, nil, true
	}
	all, err := h.rubricService.ListAssessmentsByAssociation(c.Context(), assoc.ID, repository.PaginationParams{Page: 1, PerPage: 10000})
	if err != nil {
		_ = responses.InternalError(c, "Could not fetch rubric assessments")
		return nil, nil, true
	}
	for i := range all.Items {
		a := &all.Items[i]
		if a.UserID == sub.UserID && (a.AssessmentType == "" || a.AssessmentType == "grading") {
			return assoc, a, false
		}
	}
	return assoc, nil, false
}

// GetAnonymousRubricAssessment returns the grading rubric assessment of
// the student behind :anonymous_id (null when not assessed yet).
//
//	GET /courses/:course_id/assignments/:assignment_id/anonymous_submissions/:anonymous_id/rubric_assessment
func (h *GradingUIHandler) GetAnonymousRubricAssessment(c *fiber.Ctx) error {
	_, assignment, sub, wrote := h.anonymousSubmission(c)
	if wrote {
		return nil
	}
	_, existing, wrote := h.anonymousGradingAssessment(c, assignment, sub)
	if wrote {
		return nil
	}
	if existing == nil {
		return c.JSON(fiber.Map{"rubric_assessment": nil})
	}
	return c.JSON(fiber.Map{"rubric_assessment": anonymousRubricAssessmentJSON(existing, c.Params("anonymous_id"))})
}

// SaveAnonymousRubricAssessment creates or replaces the grading rubric
// assessment of the student behind :anonymous_id.
//
//	PUT /courses/:course_id/assignments/:assignment_id/anonymous_submissions/:anonymous_id/rubric_assessment
//	{ "rubric_assessment": { "data": "{\"crit\":{\"points\":3}}" } }
func (h *GradingUIHandler) SaveAnonymousRubricAssessment(c *fiber.Ctx) error {
	_, assignment, sub, wrote := h.anonymousSubmission(c)
	if wrote {
		return nil
	}
	var input struct {
		RubricAssessment struct {
			Data string `json:"data"`
		} `json:"rubric_assessment"`
	}
	if err := c.BodyParser(&input); err != nil || input.RubricAssessment.Data == "" {
		return responses.BadRequest(c, "rubric_assessment.data is required")
	}
	assoc, existing, wrote := h.anonymousGradingAssessment(c, assignment, sub)
	if wrote {
		return nil
	}
	assessorID, _ := c.Locals("user_id").(uint)
	if existing != nil {
		existing.Data = input.RubricAssessment.Data
		existing.AssessorID = assessorID
		if err := h.rubricService.UpdateAssessment(c.Context(), existing); err != nil {
			return responses.BadRequest(c, err.Error())
		}
		return c.JSON(fiber.Map{"rubric_assessment": anonymousRubricAssessmentJSON(existing, c.Params("anonymous_id"))})
	}
	created := &models.RubricAssessment{
		RubricID:            assoc.RubricID,
		RubricAssociationID: assoc.ID,
		UserID:              sub.UserID,
		AssessorID:          assessorID,
		Data:                input.RubricAssessment.Data,
		AssessmentType:      "grading",
	}
	if err := h.rubricService.CreateAssessment(c.Context(), created); err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"rubric_assessment": anonymousRubricAssessmentJSON(created, c.Params("anonymous_id"))})
}
