package handlers

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/gofiber/fiber/v2"
)

// Anonymous grading on the user-addressed submission routes.
//
// SpeedGrader addresses students by anonymous id while an assignment
// anonymizes them (Canvas Assignment#anonymize_students?). The ordinary
// user-addressed routes (list, show, file download, history) must not
// undo that for graders: a grader who could fetch
// /submissions/:user_id would learn which anonymous id belongs to whom.
// The student themself (and a linked observer) is unaffected — they are
// not graders.

// staffAnonymized reports whether the caller is course staff and the
// assignment currently hides student identities from graders. Without
// the grading-UI service the anonymous_grading flag alone decides (the
// legacy SpeedGrader behavior); a lookup failure keeps the students
// hidden.
func (h *SubmissionHandler) staffAnonymized(c *fiber.Ctx, courseID uint, a *models.Assignment) bool {
	if a == nil || !a.AnonymousGrading || !callerIsCourseStaff(c) {
		return false
	}
	return h.assignmentAnonymized(c.Context(), courseID, a, callerAccountID(c))
}

// assignmentAnonymized is staffAnonymized without the caller check.
func (h *SubmissionHandler) assignmentAnonymized(ctx context.Context, courseID uint, a *models.Assignment, accountID uint) bool {
	if a == nil || !a.AnonymousGrading {
		return false
	}
	if h.gradingUI == nil {
		return true
	}
	students, err := h.submissionService.CourseStudents(ctx, courseID, accountID)
	if err != nil {
		return true // fail closed
	}
	ids := make([]uint, 0, len(students))
	for _, s := range students {
		ids = append(ids, s.UserID)
	}
	return h.gradingUI.AnonymizeStudents(ctx, a, ids)
}

// ensureAnonymousIDs gives every listed row an anonymous id (SpeedGrader
// assigns them lazily) so the grader can address it. Rows keep their
// place; no placeholder rows are created.
func (h *SubmissionHandler) ensureAnonymousIDs(c *fiber.Ctx, a *models.Assignment, items []models.Submission) {
	if h.gradingUI == nil || len(items) == 0 {
		return
	}
	ids := make([]uint, 0, len(items))
	for i := range items {
		if items[i].AnonymousID == nil || *items[i].AnonymousID == "" {
			ids = append(ids, items[i].UserID)
		}
	}
	if len(ids) == 0 {
		return
	}
	rows, err := h.gradingUI.EnsureSubmissionRows(c.Context(), a, ids)
	if err != nil {
		return
	}
	for i := range items {
		if r := rows[items[i].UserID]; r != nil && r.AnonymousID != nil {
			items[i].AnonymousID = r.AnonymousID
		}
	}
}

// anonymizedStaffSubmissionJSON renders a submission for a grader of an
// anonymized assignment: no user id, files through the anonymous route
// (or no link at all when the row has no anonymous id yet).
func anonymizedStaffSubmissionJSON(courseID uint, s *models.Submission) fiber.Map {
	m := submissionToJSON(s)
	if s.AnonymousID != nil && *s.AnonymousID != "" {
		return anonymizeSubmissionMap(m, courseID, s.AssignmentID, *s.AnonymousID)
	}
	m["user_id"] = nil
	m["anonymous_id"] = nil
	if atts, ok := m["attachments"].([]map[string]interface{}); ok {
		for _, a := range atts {
			delete(a, "url")
		}
	}
	return m
}
