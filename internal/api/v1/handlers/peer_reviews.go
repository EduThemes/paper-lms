package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/service"
)

type PeerReviewHandler struct {
	peerReviewService *service.PeerReviewService
	auditService      *service.AuditService
}

func NewPeerReviewHandler(peerReviewService *service.PeerReviewService, auditService *service.AuditService) *PeerReviewHandler {
	return &PeerReviewHandler{peerReviewService: peerReviewService, auditService: auditService}
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

	var input struct {
		Count int `json:"count"`
	}
	if err := c.BodyParser(&input); err != nil || input.Count <= 0 {
		input.Count = 1
	}

	reviews, err := h.peerReviewService.AssignPeerReviews(c.Context(), uint(courseID), uint(assignmentID), input.Count)
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

	reviews, err := h.peerReviewService.ListByAssignment(c.Context(), uint(assignmentID))
	if err != nil {
		return responses.InternalError(c, "Could not list peer reviews")
	}

	if callerID, _ := getUserID(c); callerID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, 0, "read", "peer_reviews_bulk", "assignments", uint(assignmentID), c.IP(), c.Get("User-Agent"))
	}

	return c.JSON(reviews)
}

func (h *PeerReviewHandler) ListMyPeerReviews(c *fiber.Ctx) error {
	assignmentID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	userID := c.Locals("user_id").(uint)

	reviews, err := h.peerReviewService.ListByReviewer(c.Context(), uint(assignmentID), userID)
	if err != nil {
		return responses.InternalError(c, "Could not list peer reviews")
	}

	return c.JSON(reviews)
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

	review, err := h.peerReviewService.SubmitReview(c.Context(), uint(reviewID), input.Score, input.Comments)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(review)
}
