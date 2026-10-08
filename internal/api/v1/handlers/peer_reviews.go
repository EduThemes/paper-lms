package handlers

import (
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/gofiber/fiber/v2"
)

type PeerReviewHandler struct {
	peerReviewService *service.PeerReviewService
	auditService      *service.AuditService
	assignmentService *service.AssignmentService
}

func NewPeerReviewHandler(peerReviewService *service.PeerReviewService, auditService *service.AuditService) *PeerReviewHandler {
	return &PeerReviewHandler{peerReviewService: peerReviewService, auditService: auditService}
}

// SetAssignmentService enables the F-013 course parent-tie on the
// course-scoped peer review routes.
func (h *PeerReviewHandler) SetAssignmentService(as *service.AssignmentService) {
	h.assignmentService = as
}

// requireAssignmentInCourse ties :id (assignment) to :course_id under the
// caller's tenant. Writes a 404 and returns wrote=true on a miss.
func (h *PeerReviewHandler) requireAssignmentInCourse(c *fiber.Ctx, assignmentID uint) (*models.Assignment, bool) {
	if h.assignmentService == nil {
		return nil, false
	}
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid course ID")
		return nil, true
	}
	assignment, err := h.assignmentService.GetByID(c.Context(), assignmentID, callerAccountID(c))
	if err != nil || assignment == nil || assignment.CourseID != uint(courseID) || assignment.WorkflowState == models.AssignmentDeleted {
		_ = responses.NotFound(c, "assignment")
		return nil, true
	}
	return assignment, false
}

func (h *PeerReviewHandler) AssignPeerReviews(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	assignmentID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}
	if _, wrote := h.requireAssignmentInCourse(c, uint(assignmentID)); wrote {
		return nil
	}

	var input struct {
		Count int `json:"count"`
	}
	if err := c.BodyParser(&input); err != nil || input.Count <= 0 {
		input.Count = 1
	}

	reviews, err := h.peerReviewService.AssignPeerReviews(c.Context(), uint(courseID), uint(assignmentID), input.Count, callerAccountID(c))
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(reviews)
}

func (h *PeerReviewHandler) ListPeerReviews(c *fiber.Ctx) error {
	assignmentID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}
	if _, wrote := h.requireAssignmentInCourse(c, uint(assignmentID)); wrote {
		return nil
	}

	reviews, err := h.peerReviewService.ListByAssignment(c.Context(), uint(assignmentID), callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not list peer reviews")
	}

	if callerID, _ := c.Locals("user_id").(uint); callerID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, 0, "read", "peer_reviews_bulk", "assignments", uint(assignmentID), c.IP(), c.Get("User-Agent"))
	}

	return c.JSON(reviews)
}

// ListMyPeerReviews returns the caller's assigned reviews together with
// the work to review (B117: the reviewer UI renders review.submission,
// which the API never returned, and the reviewer cannot read the
// classmate's submission any other way). The grade is never included.
func (h *PeerReviewHandler) ListMyPeerReviews(c *fiber.Ctx) error {
	assignmentID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}
	if _, wrote := h.requireAssignmentInCourse(c, uint(assignmentID)); wrote {
		return nil
	}

	userID, _ := c.Locals("user_id").(uint)
	if userID == 0 {
		return responses.Unauthorized(c)
	}

	reviews, err := h.peerReviewService.ListByReviewer(c.Context(), uint(assignmentID), userID, callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not list peer reviews")
	}

	out := make([]fiber.Map, 0, len(reviews))
	for i := range reviews {
		r := &reviews[i]
		j := fiber.Map{
			"id":             r.ID,
			"assignment_id":  r.AssignmentID,
			"submission_id":  r.SubmissionID,
			"reviewer_id":    r.ReviewerID,
			"reviewee_id":    r.RevieweeID,
			"workflow_state": r.WorkflowState,
			"score":          r.Score,
			"comments":       r.Comments,
			"created_at":     r.CreatedAt,
			"updated_at":     r.UpdatedAt,
		}
		if sub, err := h.peerReviewService.SubmissionForReview(c.Context(), r, callerAccountID(c)); err == nil && sub != nil {
			j["submission"] = peerReviewSubmissionJSON(sub)
		}
		out = append(out, j)
	}

	return c.JSON(out)
}

func (h *PeerReviewHandler) SubmitPeerReview(c *fiber.Ctx) error {
	reviewID, err := c.ParamsInt("review_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid review ID")
	}

	var input struct {
		Score    float64 `json:"score"`
		Comments string  `json:"comments"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	// B018: only the assigned reviewer may complete a review; anyone else
	// gets the same 404 as a missing review.
	callerID, _ := c.Locals("user_id").(uint)
	review, err := h.peerReviewService.SubmitReview(c.Context(), uint(reviewID), callerID, input.Score, input.Comments, callerAccountID(c))
	if err != nil {
		if err == service.ErrPeerReviewNotFound {
			return responses.NotFound(c, "peer review")
		}
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(review)
}
