package handlers

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/gofiber/fiber/v2"
)

// AnonymityCheck reports whether an assignment currently anonymizes its
// students (anonymous grading not yet lifted by posting grades).
type AnonymityCheck func(ctx context.Context, courseID, assignmentID, accountID uint) bool

// SetAnonymityCheck makes the staff rubric-assessment listing hide the
// assessed student's user id while the assignment anonymizes students,
// so graders cannot map SpeedGrader's anonymous ids back to students
// through the rubric API (grading-ui lane, G031). nil-safe.
func (h *RubricAssessmentHandler) SetAnonymityCheck(fn AnonymityCheck) { h.anonymityCheck = fn }

// anonymizedAssociation reports whether assoc's assignment anonymizes
// students right now.
func (h *RubricAssessmentHandler) anonymizedAssociation(c *fiber.Ctx, assoc *models.RubricAssociation, courseID uint) bool {
	if h.anonymityCheck == nil || assoc == nil || assoc.AssociationType != "Assignment" {
		return false
	}
	return h.anonymityCheck(c.Context(), courseID, assoc.AssociationID, callerAccountID(c))
}

// maskAssessmentStudent removes the assessed student's identity from a
// rendered assessment.
func maskAssessmentStudent(m fiber.Map) fiber.Map {
	m["user_id"] = nil
	return m
}
