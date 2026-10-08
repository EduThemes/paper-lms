package handlers

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

// ePortfolio additions (G155): private sharing links, comment moderation and
// a picker of the owner's course submissions.

// CreateShareLink handles POST /portfolios/:id/share_link (owner/admin):
// issues a new private sharing link (any previous link stops working).
func (h *PortfolioHandler) CreateShareLink(c *fiber.Ctx) error {
	portfolio, wrote, err := h.loadOwnedPortfolio(c, "id")
	if wrote {
		return err
	}
	token, err := h.portfolioService.CreateShareLink(c.Context(), portfolio)
	if err != nil {
		return responses.InternalError(c, "Could not create sharing link")
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"share_token": token,
		"share_url":   "/portfolios/shared/" + url.PathEscape(token),
	})
}

// RevokeShareLink handles DELETE /portfolios/:id/share_link.
func (h *PortfolioHandler) RevokeShareLink(c *fiber.Ctx) error {
	portfolio, wrote, err := h.loadOwnedPortfolio(c, "id")
	if wrote {
		return err
	}
	if err := h.portfolioService.RevokeShareLink(c.Context(), portfolio); err != nil {
		return responses.InternalError(c, "Could not remove sharing link")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// loadSharedPortfolio resolves :token for the unauthenticated shared-link
// routes. Unknown or revoked tokens are not found.
func (h *PortfolioHandler) loadSharedPortfolio(c *fiber.Ctx) (*models.Portfolio, bool) {
	token := strings.TrimSpace(c.Params("token"))
	if token == "" {
		return nil, false
	}
	portfolio, err := h.portfolioService.GetBySharedLink(c.Context(), token)
	if err != nil || portfolio == nil || portfolio.WorkflowState == models.PortfolioDeleted {
		return nil, false
	}
	return portfolio, true
}

// GetSharedPortfolio handles GET /portfolios/shared/:token (no auth): the
// portfolio as a visitor sees it (hidden sections removed), whether or not
// it is public — holding the link is the permission.
func (h *PortfolioHandler) GetSharedPortfolio(c *fiber.Ctx) error {
	portfolio, ok := h.loadSharedPortfolio(c)
	if !ok {
		return responses.NotFound(c, "portfolio")
	}
	_ = h.portfolioService.RecordView(c.Context(), portfolio.ID)
	base := fmt.Sprintf("/api/v1/portfolios/shared/%s/files", url.PathEscape(*portfolio.ShareToken))
	out := h.portfolioDetailJSONWithFiles(c, portfolio, true, base)
	out["shared_via_link"] = true
	return c.JSON(out)
}

// DownloadSharedArtifactFile handles GET /portfolios/shared/:token/files/:file_id
// (no auth): a file of a visible artifact of the shared portfolio.
func (h *PortfolioHandler) DownloadSharedArtifactFile(c *fiber.Ctx) error {
	portfolio, ok := h.loadSharedPortfolio(c)
	if !ok {
		return responses.NotFound(c, "file")
	}
	// accountID=0: anonymous route with no tenant Locals. The attachment is
	// served only when it belongs to this portfolio's owner and backs one
	// of its visible artifacts (checked in streamPortfolioFile).
	return h.streamPortfolioFile(c, portfolio, 0, true)
}

// DeleteComment handles DELETE /portfolios/:id/comments/:comment_id. The
// portfolio owner (or an admin) can remove any comment on it; a viewer can
// remove their own. A comment of another portfolio is a 404.
func (h *PortfolioHandler) DeleteComment(c *fiber.Ctx) error {
	portfolio, wrote, err := h.loadViewablePortfolio(c, "id")
	if wrote {
		return err
	}
	commentID, err := c.ParamsInt("comment_id")
	if err != nil || commentID <= 0 {
		return responses.BadRequest(c, "Invalid comment ID")
	}
	comment, err := h.portfolioService.GetComment(c.Context(), portfolio.ID, uint(commentID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "comment")
	}
	callerID, _ := c.Locals("user_id").(uint)
	if comment.UserID != callerID {
		if h.authz == nil || h.authz.RequireOwnerOrAdmin(c, portfolio.UserID) != nil {
			return responses.NotFound(c, "comment")
		}
	}
	if err := h.portfolioService.DeleteComment(c.Context(), portfolio.ID, uint(commentID), callerAccountID(c)); err != nil {
		if errors.Is(err, service.ErrPortfolioChildNotFound) {
			return responses.NotFound(c, "comment")
		}
		return responses.InternalError(c, "Could not delete comment")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ListImportableSubmissions handles
// GET /portfolios/:id/importable_submissions?course_id= (owner/admin): the
// owner's turned-in submissions in that course, for "Add course work".
func (h *PortfolioHandler) ListImportableSubmissions(c *fiber.Ctx) error {
	portfolio, wrote, err := h.loadOwnedPortfolio(c, "id")
	if wrote {
		return err
	}
	courseID := c.QueryInt("course_id", 0)
	if courseID <= 0 {
		return responses.BadRequest(c, "course_id is required")
	}
	items, err := h.portfolioService.ListImportableSubmissions(c.Context(), portfolio, uint(courseID), callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not list submissions")
	}
	return c.JSON(items)
}
