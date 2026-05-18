package handlers

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
)

// leaderboardRowJSON is the per-row payload returned by GetCourseLeaderboard.
type leaderboardRowJSON struct {
	Rank           int    `json:"rank"`
	UserID         uint   `json:"user_id"`
	Name           string `json:"name"`
	LifetimeEarned int64  `json:"lifetime_earned"`
	IsViewer       bool   `json:"is_viewer,omitempty"`
}

type nextToBeatJSON struct {
	Name           string `json:"name"`
	LifetimeEarned int64  `json:"lifetime_earned"`
	Gap            int64  `json:"gap"`
	CurrencyLabel  string `json:"currency_label"`
}

type leaderboardResponse struct {
	CourseID        uint                 `json:"course_id"`
	CurrencyTypeID  uint                 `json:"currency_type_id"`
	CurrencyCode    string               `json:"currency_code"`
	CurrencyLabel   string               `json:"currency_label"`
	TotalCandidates int                  `json:"total_candidates"`
	Rows            []leaderboardRowJSON `json:"rows"`
	ViewerRank      int                  `json:"viewer_rank,omitempty"`
	WindowKind      string               `json:"window_kind"`
	UsedPseudonyms  bool                 `json:"used_pseudonyms"`
	NextToBeat      *nextToBeatJSON      `json:"next_to_beat,omitempty"`
	Limit           int                  `json:"limit"`
	Offset          int                  `json:"offset"`
}

// GetCourseLeaderboard returns the per-course ranking by lifetime_earned in
// the named currency. Service drives role resolution, opt-out filter, render
// policy + relative-window mechanic. Handler is parse → call → serialize.
func (h *GamificationHandler) GetCourseLeaderboard(c *fiber.Ctx) error {
	courseID64, err := strconv.ParseUint(c.Params("course_id"), 10, 64)
	if err != nil || courseID64 == 0 {
		return responses.Error(c, fiber.StatusBadRequest, "invalid course_id")
	}
	viewerID, _ := c.Locals("user_id").(uint)
	if viewerID == 0 {
		return responses.Unauthorized(c)
	}
	isAdmin, _ := c.Locals("is_admin").(bool)

	out, err := h.leaderboardService.GetCourseLeaderboard(c.Context(), gamification.CourseLeaderboardInput{
		CourseID:     uint(courseID64),
		ViewerID:     viewerID,
		IsAdminFlag:  isAdmin,
		TenantID:     callerAccountID(c),
		CurrencyCode: c.Query("currency", "xp"),
		OffsetWeeks:  parseIntDefault(c.Query("offset_weeks"), 0),
		Limit:        parseIntDefault(c.Query("limit"), 50),
		Offset:       parseIntDefault(c.Query("offset"), 0),
	})
	if err != nil {
		return mapLeaderboardServiceError(c, err)
	}

	rows := make([]leaderboardRowJSON, 0, len(out.Rows))
	for _, r := range out.Rows {
		rows = append(rows, leaderboardRowJSON{
			Rank:           r.Rank,
			UserID:         r.UserID,
			Name:           r.Name,
			LifetimeEarned: r.LifetimeEarned,
			IsViewer:       r.IsViewer,
		})
	}
	var ntb *nextToBeatJSON
	if out.NextToBeat != nil {
		ntb = &nextToBeatJSON{
			Name:           out.NextToBeat.Name,
			LifetimeEarned: out.NextToBeat.LifetimeEarned,
			Gap:            out.NextToBeat.Gap,
			CurrencyLabel:  out.NextToBeat.CurrencyLabel,
		}
	}
	return c.JSON(leaderboardResponse{
		CourseID:        out.CourseID,
		CurrencyTypeID:  out.Currency.ID,
		CurrencyCode:    out.Currency.Code,
		CurrencyLabel:   out.Currency.DisplayLabel,
		TotalCandidates: out.TotalCandidates,
		Rows:            rows,
		ViewerRank:      out.ViewerRank,
		WindowKind:      out.WindowKind,
		UsedPseudonyms:  out.UsedPseudonyms,
		NextToBeat:      ntb,
		Limit:           out.Limit,
		Offset:          out.Offset,
	})
}

// mapLeaderboardServiceError translates leaderboard sentinels to Fiber
// responses. ErrUnknownCurrency is 400; ErrCourseLeaderboardNotEnrolled is
// 404 per 13.1.E (existence leak — a 403 confirms the course exists in
// this or another tenant to a non-enrolled viewer).
func mapLeaderboardServiceError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, gamification.ErrCourseLeaderboardNotEnrolled):
		// 13.1.E: existence leak — return 404 not 403 on non-enrolled.
		return responses.NotFound(c, "course")
	case errors.Is(err, gamification.ErrUnknownCurrency):
		return responses.Error(c, fiber.StatusBadRequest, err.Error())
	case errors.Is(err, gamification.ErrSnapshotNotFound):
		return responses.Error(c, fiber.StatusNotFound, err.Error())
	default:
		return responses.InternalError(c, "leaderboard render failed")
	}
}

// parseIntDefault parses a query-string int, returning fallback on empty /
// invalid input.
func parseIntDefault(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return v
}
