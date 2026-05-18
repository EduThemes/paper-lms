package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
	"github.com/EduThemes/paper-lms/internal/service/gamification/pseudonym"
)

// leaderboardRowJSON is the per-row payload returned by GetCourseLeaderboard.
// `Name` carries either the learner's legal name (admin / teacher view, or
// HigherEd/Corp student view) or a curated pseudonym (K-5/M68/H912 student
// view) — the server decides per render policy; the frontend just renders.
//
// LifetimeEarned is the ranking column (gamification_wallet.go:9–14 — earned
// not held, so spendable currencies don't penalize learners who spend gems).
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
	ViewerRank      int                  `json:"viewer_rank,omitempty"`     // 0 if viewer isn't ranked (opted out or not enrolled)
	WindowKind      string               `json:"window_kind"`               // "top_n" | "full" | "relative"
	UsedPseudonyms  bool                 `json:"used_pseudonyms"`
	NextToBeat      *nextToBeatJSON      `json:"next_to_beat,omitempty"`
	Limit           int                  `json:"limit"`
	Offset          int                  `json:"offset"`
}

// GetCourseLeaderboard returns the per-course ranking by lifetime_earned in
// the named currency (defaults to `xp`). Access is open to admins, course
// teachers/TAs, AND enrolled students — the W2-C opt-out filter + the
// W3-B tenant-mode render policy together determine what each viewer sees.
//
// Privacy chain:
//
//  1. EnrollmentRepository.ListActiveStudentUserIDsByCourse → candidate
//     set bounded to active StudentEnrollment rows.
//  2. UserRepository.FilterPublicLeaderboardCandidates → drops opted-out
//     learners (W2-C).
//  3. GamificationWalletRepository.RankByCurrency → sorts by
//     lifetime_earned, ties by earliest most-recent positive transaction.
//  4. RenderPolicyFor(tenantMode, role, viewerRank) → decides
//     pseudonyms / top-N gate / first-name eligibility.
//  5. Pseudonym substitution for peers (viewer always sees own real name).
func (h *GamificationHandler) GetCourseLeaderboard(c *fiber.Ctx) error {
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

	role, wrote, fiberErr := h.resolveViewerRoleInCourse(c, viewerID, courseID)
	if wrote {
		// helper already wrote a 401/403/500 response; short-circuit.
		// fiberErr is whatever c.Status().JSON() returned (typically nil).
		return fiberErr
	}

	// Currency resolution. Default to "xp" — Wave 1 seed guarantees this
	// at every tenant. Site-scope currencies are stored with
	// scope_id = tenantID (per the SeedSystemCurrenciesForTenant
	// convention), not scope_id = 0.
	currencyCode := c.Query("currency", "xp")
	tenantID := callerAccountID(c)
	currency, err := h.currencyRepo.FindByCode(c.Context(), tenantID, models.ScopeSite, tenantID, currencyCode)
	if err != nil {
		return responses.InternalError(c, "failed to resolve currency")
	}
	if currency == nil {
		return responses.Error(c, fiber.StatusBadRequest, "unknown currency: "+currencyCode)
	}

	// Sprint 7-B — historical-window branch. ?offset_weeks=N (N>=1)
	// reads from a stored snapshot for the (course, currency, week)
	// tuple. The handler re-applies FilterPublicLeaderboardCandidates
	// at read time so a learner who opted out post-snapshot still
	// vanishes from peer views (FERPA decision documented in plan).
	if offsetWeeks := parseIntDefault(c.Query("offset_weeks"), 0); offsetWeeks >= 1 {
		return h.serveSnapshotLeaderboard(c, courseID, currency, role, viewerID, tenantID, offsetWeeks)
	}

	// Tenant mode drives the render policy. Falls back to higher_ed
	// (the model's SQL default) if the account row can't be loaded —
	// the safest default because it surfaces real names, not the
	// less-strict K-12 anonymity path.
	tenantMode := "higher_ed"
	if h.accountRepo != nil {
		acc, err := h.accountRepo.FindByID(c.Context(), tenantID)
		if err == nil && acc != nil && acc.TenantMode != "" {
			tenantMode = string(acc.TenantMode)
		}
	}

	// Candidate set + opt-out filter.
	enrollments, err := h.enrollmentRepo.ListActiveStudentEnrollmentsByCourse(c.Context(), courseID)
	if err != nil {
		return responses.InternalError(c, "failed to list enrollments")
	}
	candidates := make([]uint, 0, len(enrollments))
	enrollmentByUser := make(map[uint]models.Enrollment, len(enrollments))
	for _, e := range enrollments {
		candidates = append(candidates, e.UserID)
		enrollmentByUser[e.UserID] = e
	}
	visible, err := h.userRepo.FilterPublicLeaderboardCandidates(c.Context(), candidates)
	if err != nil {
		return responses.InternalError(c, "failed to apply opt-out filter")
	}

	// Rank.
	ranked, err := h.walletRepo.RankByCurrency(c.Context(), currency.ID, visible)
	if err != nil {
		return responses.InternalError(c, "failed to rank wallet balances")
	}

	// Viewer's rank (0 if not in `visible` — opted out, or a non-student
	// viewer like teacher / admin).
	viewerRank := 0
	for _, r := range ranked {
		if r.UserID == viewerID {
			viewerRank = r.Rank
			break
		}
	}

	policy := gamification.RenderPolicyFor(tenantMode, role, viewerRank)

	// Decide the response window.
	//   * Admin / teacher / top-N student → ShowTopN (or full list with
	//     pagination if asked for ?mode=full).
	//   * Student outside top N → relative window with fillers, "next
	//     to beat" callout. This is the W3-C motivational mechanic.
	windowKind := "full"
	var rows []leaderboardRowJSON
	var ntbJSON *nextToBeatJSON
	switch {
	case role == gamification.ViewerStudent && !policy.ShowTopN:
		windowKind = "relative"
		// Pull the viewer's enrollment so fillers come from the same
		// pool — they blend with the viewer's view of the world.
		viewerEnrollment := enrollmentByUser[viewerID]
		viewerPool := resolveViewerPool(viewerEnrollment)
		window := gamification.ComposeRelativeWindow(ranked, viewerID, viewerPool, viewerEnrollment.ID)

		// Single FindByIDs covers every row name needed: the window's
		// non-filler rows + the next-to-beat row if there is one.
		idsForName := make([]uint, 0, len(window.Rows)+1)
		for _, r := range window.Rows {
			if !r.IsFiller {
				idsForName = append(idsForName, r.UserID)
			}
		}
		if window.NextToBeat != nil {
			idsForName = append(idsForName, window.NextToBeat.UserID)
		}
		nameByID := h.loadNamesByID(c, idsForName)

		rows = h.buildRelativeRowsWithNames(window.Rows, enrollmentByUser, policy, viewerID, nameByID)
		if window.NextToBeat != nil {
			ntbName := h.renderName(window.NextToBeat.UserID, enrollmentByUser, policy, viewerID, nameByID)
			ntbJSON = &nextToBeatJSON{
				Name:           ntbName,
				LifetimeEarned: window.NextToBeat.LifetimeEarned,
				Gap:            window.NextToBeat.Gap,
				CurrencyLabel:  currency.DisplayLabel,
			}
		}
	case policy.ShowTopN:
		windowKind = "top_n"
		n := policy.TopNSize
		if n > len(ranked) {
			n = len(ranked)
		}
		rows = h.buildRows(c, ranked[:n], enrollmentByUser, policy, viewerID)
	default:
		// Admin / teacher full-list view, paginated.
		limit := parseIntDefault(c.Query("limit"), 50)
		if limit > 200 {
			limit = 200
		}
		if limit < 1 {
			limit = 50
		}
		offset := parseIntDefault(c.Query("offset"), 0)
		if offset < 0 {
			offset = 0
		}
		end := offset + limit
		if end > len(ranked) {
			end = len(ranked)
		}
		rows = h.buildRows(c, ranked[offset:end], enrollmentByUser, policy, viewerID)
	}

	return c.JSON(leaderboardResponse{
		CourseID:        courseID,
		CurrencyTypeID:  currency.ID,
		CurrencyCode:    currency.Code,
		CurrencyLabel:   currency.DisplayLabel,
		TotalCandidates: len(ranked),
		Rows:            rows,
		ViewerRank:      viewerRank,
		WindowKind:      windowKind,
		UsedPseudonyms:  policy.UsePseudonyms,
		NextToBeat:      ntbJSON,
		Limit:           parseIntDefault(c.Query("limit"), 50),
		Offset:          parseIntDefault(c.Query("offset"), 0),
	})
}

// buildRelativeRowsWithNames materializes the relative-window slice
// using a pre-loaded nameByID map. Fillers pass through with no user_id
// and no rank — the response shape is identical to a real row from the
// wire side (network-tab inspection can't distinguish a filler from a
// real peer).
//
// Split from `buildRows` because the caller pre-fetches names once for
// both the window rows AND the next-to-beat row, avoiding the
// double-FindByIDs round-trip the original W3-C implementation had.
func (h *GamificationHandler) buildRelativeRowsWithNames(window []gamification.RelativeRow, enrollmentByUser map[uint]models.Enrollment, policy gamification.LeaderboardRenderPolicy, viewerID uint, nameByID map[uint]string) []leaderboardRowJSON {
	out := make([]leaderboardRowJSON, 0, len(window))
	for _, r := range window {
		if r.IsFiller {
			out = append(out, leaderboardRowJSON{
				// rank/user_id deliberately zero — fillers don't carry
				// a real identity. Frontend renders rows in array
				// order, not by row.Rank.
				Name:           r.Pseudonym,
				LifetimeEarned: r.LifetimeEarned,
			})
			continue
		}
		legal := nameByID[r.UserID]
		display := legal
		isViewer := r.UserID == viewerID
		if policy.UsePseudonyms && !isViewer {
			display = renderPseudonym(enrollmentByUser[r.UserID], legal)
		}
		out = append(out, leaderboardRowJSON{
			Rank:           r.Rank,
			UserID:         r.UserID,
			Name:           display,
			LifetimeEarned: r.LifetimeEarned,
			IsViewer:       isViewer,
		})
	}
	return out
}

// loadNamesByID is the single FindByIDs entry point for the relative-
// window path. Soft-fails to an empty map on error so a name-lookup
// outage degrades to "User #N" display rather than a 500.
func (h *GamificationHandler) loadNamesByID(c *fiber.Ctx, ids []uint) map[uint]string {
	if len(ids) == 0 {
		return map[uint]string{}
	}
	users, err := h.userRepo.FindByIDs(c.Context(), ids)
	if err != nil {
		return map[uint]string{}
	}
	out := make(map[uint]string, len(users))
	for _, u := range users {
		out[u.ID] = u.Name
	}
	return out
}

// renderName returns the same display string buildRelativeRowsWithNames
// would produce for `userID`, given the active policy. Used by the
// next-to-beat composer.
func (h *GamificationHandler) renderName(userID uint, enrollmentByUser map[uint]models.Enrollment, policy gamification.LeaderboardRenderPolicy, viewerID uint, nameByID map[uint]string) string {
	legal := nameByID[userID]
	if policy.UsePseudonyms && userID != viewerID {
		return renderPseudonym(enrollmentByUser[userID], legal)
	}
	return legal
}

// resolveViewerRoleInCourse decides whether the request's viewer is an
// admin, a course teacher / TA, or a student in this course.
//
// Returns (role, wrote, err). When `wrote=true`, the helper already
// emitted an HTTP response (401/403/500) and the caller MUST return the
// `err` immediately without further processing. The wrote-flag is the
// abort signal because `responses.Error()` returns nil after writing,
// which the previous (err != nil) check couldn't distinguish from a
// happy path.
//
// Extracted from W3-A's inline check + W3-B's `policyForViewerInCourse`
// (gamification_pseudonyms.go) to keep the two endpoints consistent on
// the userRepo admin fallback. The leaderboard route mounts without an
// admin middleware (it's accessible to students), so `is_admin` Locals
// may be unset for an admin user; the userRepo lookup catches that.
func (h *GamificationHandler) resolveViewerRoleInCourse(c *fiber.Ctx, viewerID, courseID uint) (gamification.ViewerRole, bool, error) {
	isAdmin, _ := c.Locals("is_admin").(bool)
	if !isAdmin {
		if user, err := h.userRepo.FindByID(c.Context(), viewerID); err == nil && user != nil && user.Role == "admin" {
			isAdmin = true
		}
	}
	if isAdmin {
		return gamification.ViewerAdmin, false, nil
	}
	viewerEnrollment, err := h.enrollmentRepo.FindByUserAndCourse(c.Context(), viewerID, courseID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", true, responses.InternalError(c, "failed to resolve enrollment")
	}
	if viewerEnrollment == nil || viewerEnrollment.WorkflowState != "active" {
		// 13.1.E: existence leak — return 404 not 403. A 403 confirms
		// the course exists (in this or another tenant) to a non-
		// enrolled viewer; 404 keeps that signal silent.
		return "", true, responses.NotFound(c, "course")
	}
	if viewerEnrollment.Type == "TeacherEnrollment" || viewerEnrollment.Type == "TaEnrollment" {
		return gamification.ViewerTeacher, false, nil
	}
	return gamification.ViewerStudent, false, nil
}

// serveSnapshotLeaderboard handles ?offset_weeks=N (N>=1) by reading
// from gamification_leaderboard_snapshots instead of live-computing.
// FERPA contract: re-applies FilterPublicLeaderboardCandidates at read
// time so a learner who opted out POST-snapshot still vanishes from
// peer views. Same render-policy / pseudonym / top-N branching as the
// live path so the response shape stays consistent across windows.
func (h *GamificationHandler) serveSnapshotLeaderboard(
	c *fiber.Ctx,
	courseID uint,
	currency *models.GamificationCurrencyType,
	role gamification.ViewerRole,
	viewerID, tenantID uint,
	offsetWeeks int,
) error {
	// Resolve the window-end for this offset.
	_, windowEnd := gamification.WeeklyWindowForOffset(time.Now(), offsetWeeks)

	row, err := h.snapshotRepo.FindByWindow(c.Context(),
		models.ScopeCourse, courseID, currency.ID,
		string(models.WindowKindWeekly), windowEnd)
	if err != nil {
		return responses.InternalError(c, "failed to load snapshot")
	}
	if row == nil {
		return responses.Error(c, fiber.StatusNotFound,
			fmt.Sprintf("no snapshot for window ending %s", windowEnd.Format(time.RFC3339)))
	}

	// Decode payload.
	var payload []models.SnapshotRow
	if err := json.Unmarshal([]byte(row.Payload), &payload); err != nil {
		return responses.InternalError(c, "failed to decode snapshot payload")
	}

	// Re-apply opt-out filter at READ time. A learner who opted out
	// AFTER snapshot-write still disappears from peer views.
	candidateIDs := make([]uint, 0, len(payload))
	for _, r := range payload {
		candidateIDs = append(candidateIDs, r.UserID)
	}
	visible, err := h.userRepo.FilterPublicLeaderboardCandidates(c.Context(), candidateIDs)
	if err != nil {
		return responses.InternalError(c, "failed to apply opt-out filter")
	}
	visibleSet := make(map[uint]struct{}, len(visible))
	for _, id := range visible {
		visibleSet[id] = struct{}{}
	}

	// Drop payload rows for users who opted out post-snapshot, and
	// re-number ranks 1..N so the response doesn't show gaps in the
	// rank column.
	filtered := make([]models.SnapshotRow, 0, len(payload))
	for _, r := range payload {
		if _, ok := visibleSet[r.UserID]; ok {
			filtered = append(filtered, r)
		}
	}
	for i := range filtered {
		filtered[i].Rank = i + 1
	}

	// Find viewer rank for the render-policy decision.
	viewerRank := 0
	for _, r := range filtered {
		if r.UserID == viewerID {
			viewerRank = r.Rank
			break
		}
	}

	// Render policy is the same as the live path.
	tenantMode := "higher_ed"
	if h.accountRepo != nil {
		if acc, err := h.accountRepo.FindByID(c.Context(), tenantID); err == nil && acc != nil && acc.TenantMode != "" {
			tenantMode = string(acc.TenantMode)
		}
	}
	policy := gamification.RenderPolicyFor(tenantMode, role, viewerRank)

	// Pseudonym substitution needs the enrollment context for each
	// payload user. Pull them once.
	enrollments, err := h.enrollmentRepo.ListActiveStudentEnrollmentsByCourse(c.Context(), courseID)
	if err != nil {
		return responses.InternalError(c, "failed to list enrollments")
	}
	enrollmentByUser := make(map[uint]models.Enrollment, len(enrollments))
	for _, e := range enrollments {
		enrollmentByUser[e.UserID] = e
	}

	// Materialize rows. Snapshot reads ALWAYS return the full
	// historical list — the relative-window + filler mechanic is a
	// motivational tool tied to the CURRENT cohort; historical
	// snapshots show what actually happened that week. Pseudonym
	// substitution still applies.
	nameByID := h.loadNamesByID(c, candidateIDs)
	rows := make([]leaderboardRowJSON, 0, len(filtered))
	for _, r := range filtered {
		legal := nameByID[r.UserID]
		display := legal
		isViewer := r.UserID == viewerID
		if policy.UsePseudonyms && !isViewer {
			display = renderPseudonym(enrollmentByUser[r.UserID], legal)
		}
		rows = append(rows, leaderboardRowJSON{
			Rank:           r.Rank,
			UserID:         r.UserID,
			Name:           display,
			LifetimeEarned: r.LifetimeEarned,
			IsViewer:       isViewer,
		})
	}

	return c.JSON(leaderboardResponse{
		CourseID:        courseID,
		CurrencyTypeID:  currency.ID,
		CurrencyCode:    currency.Code,
		CurrencyLabel:   currency.DisplayLabel,
		TotalCandidates: len(filtered),
		Rows:            rows,
		ViewerRank:      viewerRank,
		WindowKind:      "snapshot_weekly",
		UsedPseudonyms:  policy.UsePseudonyms,
		Limit:           len(rows),
		Offset:          0,
	})
}

// resolveViewerPool returns the pseudonym pool the viewer's enrollment
// uses, defaulting to the animals pool when missing (e.g. first read of
// a fresh enrollment row before any switch has happened).
func resolveViewerPool(e models.Enrollment) pseudonym.Pool {
	code := pseudonym.PoolCode(e.PseudonymPoolCode)
	if code == "" || code == pseudonym.PoolFirstName {
		code = pseudonym.PoolAnimals
	}
	p, err := pseudonym.PoolByCode(code)
	if err != nil || p == nil {
		// Should never happen — Catalog has animals_v1. Defensive
		// fallback returns an empty pool that the filler generator
		// handles with "Anonymous Wanderer".
		return pseudonym.Pool{Code: pseudonym.PoolAnimals}
	}
	return *p
}

// buildRows materializes the leaderboard slice into JSON-shaped rows,
// applying the pseudonym substitution where policy demands it. The
// viewer always sees their own legal name so they know who they are
// on the board.
func (h *GamificationHandler) buildRows(c *fiber.Ctx, page []repository.RankRow, enrollmentByUser map[uint]models.Enrollment, policy gamification.LeaderboardRenderPolicy, viewerID uint) []leaderboardRowJSON {
	idsForName := make([]uint, 0, len(page))
	for _, r := range page {
		idsForName = append(idsForName, r.UserID)
	}
	users, err := h.userRepo.FindByIDs(c.Context(), idsForName)
	if err != nil {
		// Soft-fail: render rows with empty names rather than 500.
		users = nil
	}
	nameByID := make(map[uint]string, len(users))
	for _, u := range users {
		nameByID[u.ID] = u.Name
	}

	out := make([]leaderboardRowJSON, 0, len(page))
	for _, r := range page {
		legal := nameByID[r.UserID]
		display := legal
		isViewer := r.UserID == viewerID

		if policy.UsePseudonyms && !isViewer {
			display = renderPseudonym(enrollmentByUser[r.UserID], legal)
		}

		out = append(out, leaderboardRowJSON{
			Rank:           r.Rank,
			UserID:         r.UserID,
			Name:           display,
			LifetimeEarned: r.LifetimeEarned,
			IsViewer:       isViewer,
		})
	}
	return out
}

// renderPseudonym resolves a peer row's display name. Persisted alias
// wins; otherwise the deterministic FNV generator picks one from the
// enrollment's chosen pool at attempt 0. First-name mode special-cases
// to the legal-name first token. Empty fallback returns the bare legal
// name so a render never throws.
func renderPseudonym(e models.Enrollment, legalName string) string {
	pool := pseudonym.PoolCode(e.PseudonymPoolCode)
	if pool == "" {
		pool = pseudonym.PoolAnimals
	}
	if pool == pseudonym.PoolFirstName {
		if first := pseudonym.FirstNameOf(legalName); first != "" {
			return first
		}
		return legalName
	}
	if e.PseudonymName != nil && *e.PseudonymName != "" {
		return *e.PseudonymName
	}
	p, err := pseudonym.PoolByCode(pool)
	if err != nil || p == nil {
		return legalName
	}
	return pseudonym.GenerateForEnrollment(*p, e.ID, 0)
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
