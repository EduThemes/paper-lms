package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// setupLeaderboardHandler wires the GamificationHandler for W3 endpoint
// tests. Mirrors setupGamificationHandler but bypasses the route-mount
// boilerplate from the W2 helper — we only need /courses/:id/leaderboard
// here. The auth stub injects user_id + is_admin Locals.
func setupLeaderboardHandler(callerID uint, isAdmin bool) (*fiber.App, *mockGamWalletRepo, *mocks.MockUserRepository, *mocks.MockEnrollmentRepository, *mocks.MockAccountRepository, *mockGamCurrencyRepo, *mocks.MockGamificationLeaderboardSnapshotRepository) {
	walletRepo := new(mockGamWalletRepo)
	currencyRepo := new(mockGamCurrencyRepo)
	userRepo := new(mocks.MockUserRepository)
	badgeRepo := new(mockGamBadgeRepo)
	badgeAwardRepo := new(mockGamBadgeAwardRepo)
	ruleRepo := new(mockGamRuleRepo)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)
	accountRepo := new(mocks.MockAccountRepository)
	snapshotRepo := new(mocks.MockGamificationLeaderboardSnapshotRepository)
	h := handlers.NewGamificationHandler(walletRepo, currencyRepo, userRepo, badgeRepo, badgeAwardRepo, ruleRepo, enrollmentRepo, accountRepo, snapshotRepo)

	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", callerID)
		c.Locals("is_admin", isAdmin)
		c.Locals("account_id", uint(1))
		return c.Next()
	})
	app.Get("/api/v1/courses/:course_id/leaderboard", h.GetCourseLeaderboard)
	return app, walletRepo, userRepo, enrollmentRepo, accountRepo, currencyRepo, snapshotRepo
}

// xpCurrency returns the default xp currency fixture.
func xpCurrency() *models.GamificationCurrencyType {
	return &models.GamificationCurrencyType{
		ID:           1,
		TenantID:     1,
		ScopeType:    models.ScopeSite,
		ScopeID:      1,
		Code:         "xp",
		DisplayLabel: "XP",
		SystemOwned:  true,
	}
}

// course1ActiveEnrollments returns six active student enrollments
// (ids 100-105), the standard fixture used across tests. Pseudonym
// fields left at zero values — generator fills them deterministically.
func course1ActiveEnrollments() []models.Enrollment {
	out := make([]models.Enrollment, 6)
	for i := range out {
		out[i] = models.Enrollment{
			ID:                uint(200 + i),
			UserID:            uint(100 + i),
			CourseID:          1,
			Type:              "StudentEnrollment",
			WorkflowState:     "active",
			PseudonymPoolCode: "animals_v1",
		}
	}
	return out
}

func TestGetCourseLeaderboard_AdminSeesRealNamesAndTopN(t *testing.T) {
	const adminID = 1
	app, walletRepo, userRepo, enrollmentRepo, accountRepo, currencyRepo, _ := setupLeaderboardHandler(adminID, true)

	currencyRepo.On("FindByCode", mock.Anything, uint(1), models.ScopeSite, uint(1), "xp").
		Return(xpCurrency(), nil)
	accountRepo.On("FindByID", mock.Anything, uint(1)).
		Return(&models.Account{ID: 1, TenantMode: "k5"}, nil)
	enrollmentRepo.On("ListActiveStudentEnrollmentsByCourse", mock.Anything, uint(1)).
		Return(course1ActiveEnrollments(), nil)
	// Admin doesn't have a row in the candidate set; userRepo lookup is
	// still required (no admin middleware on this route).
	userRepo.On("FindByID", mock.Anything, uint(adminID)).
		Return(&models.User{ID: adminID, Name: "Avery Admin", Role: "admin"}, nil).Maybe()

	candidateIDs := []uint{100, 101, 102, 103, 104, 105}
	userRepo.On("FilterPublicLeaderboardCandidates", mock.Anything, candidateIDs).
		Return(candidateIDs, nil)
	rankRows := []repository.RankRow{
		{UserID: 100, LifetimeEarned: 640, Rank: 1},
		{UserID: 101, LifetimeEarned: 520, Rank: 2},
		{UserID: 102, LifetimeEarned: 440, Rank: 3},
		{UserID: 103, LifetimeEarned: 360, Rank: 4},
		{UserID: 104, LifetimeEarned: 250, Rank: 5},
		{UserID: 105, LifetimeEarned: 140, Rank: 6},
	}
	walletRepo.On("RankByCurrency", mock.Anything, uint(1), candidateIDs).
		Return(rankRows, nil)
	userRepo.On("FindByIDs", mock.Anything, mock.Anything).
		Return([]models.User{
			{ID: 100, Name: "Sofia Alvarez"},
			{ID: 101, Name: "Ben Carter"},
			{ID: 102, Name: "Chen Wei"},
			{ID: 103, Name: "Diego Martinez"},
			{ID: 104, Name: "Emma Patel"},
		}, nil)

	resp := requestLeaderboard(t, app, "/api/v1/courses/1/leaderboard")
	requireStatus(t, resp, http.StatusOK)

	var body leaderboardJSON
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	require.Equal(t, "top_n", body.WindowKind, "admin should get top_n window")
	require.False(t, body.UsedPseudonyms, "admin should see real names regardless of tenant mode")
	require.Equal(t, 5, len(body.Rows), "top_n should trim to top 5")
	require.Equal(t, "Sofia Alvarez", body.Rows[0].Name)
	require.Equal(t, 6, body.TotalCandidates, "all 6 active enrollments ranked")
}

func TestGetCourseLeaderboard_K5StudentSeesPseudonymsAndRelativeWindow(t *testing.T) {
	const viewerID = 105 // last-place student, rank 6
	app, walletRepo, userRepo, enrollmentRepo, accountRepo, currencyRepo, _ := setupLeaderboardHandler(viewerID, false)

	currencyRepo.On("FindByCode", mock.Anything, uint(1), models.ScopeSite, uint(1), "xp").
		Return(xpCurrency(), nil)
	accountRepo.On("FindByID", mock.Anything, uint(1)).
		Return(&models.Account{ID: 1, TenantMode: "k5"}, nil)
	enrollmentRepo.On("ListActiveStudentEnrollmentsByCourse", mock.Anything, uint(1)).
		Return(course1ActiveEnrollments(), nil)
	// Viewer's own enrollment lookup (role resolution).
	enrollmentRepo.On("FindByUserAndCourse", mock.Anything, uint(viewerID), uint(1)).
		Return(&models.Enrollment{
			ID: 205, UserID: viewerID, CourseID: 1,
			Type: "StudentEnrollment", WorkflowState: "active",
			PseudonymPoolCode: "animals_v1",
		}, nil)
	userRepo.On("FindByID", mock.Anything, uint(viewerID)).
		Return(&models.User{ID: viewerID, Name: "Gabriel O'Donnell", Role: "user"}, nil).Maybe()

	candidateIDs := []uint{100, 101, 102, 103, 104, 105}
	userRepo.On("FilterPublicLeaderboardCandidates", mock.Anything, candidateIDs).
		Return(candidateIDs, nil)
	rankRows := []repository.RankRow{
		{UserID: 100, LifetimeEarned: 640, Rank: 1},
		{UserID: 101, LifetimeEarned: 520, Rank: 2},
		{UserID: 102, LifetimeEarned: 440, Rank: 3},
		{UserID: 103, LifetimeEarned: 360, Rank: 4},
		{UserID: 104, LifetimeEarned: 250, Rank: 5},
		{UserID: 105, LifetimeEarned: 40, Rank: 6},
	}
	walletRepo.On("RankByCurrency", mock.Anything, uint(1), candidateIDs).
		Return(rankRows, nil)
	userRepo.On("FindByIDs", mock.Anything, mock.Anything).
		Return([]models.User{
			{ID: 103, Name: "Diego Martinez"},
			{ID: 104, Name: "Emma Patel"},
			{ID: viewerID, Name: "Gabriel O'Donnell"},
		}, nil)

	resp := requestLeaderboard(t, app, "/api/v1/courses/1/leaderboard")
	requireStatus(t, resp, http.StatusOK)

	var body leaderboardJSON
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	require.Equal(t, "relative", body.WindowKind, "K-5 student outside top-N gets relative window")
	require.True(t, body.UsedPseudonyms, "K-5 student must see pseudonyms")
	require.Equal(t, 5, len(body.Rows), "relative window is always 5 rows")
	require.Equal(t, 6, body.ViewerRank, "viewer should be at rank 6")

	// Viewer's own row must show real name + is_viewer flag.
	var viewerRow *leaderboardRowJSON
	for i, r := range body.Rows {
		if r.IsViewer {
			viewerRow = &body.Rows[i]
		}
	}
	require.NotNil(t, viewerRow, "viewer row should be present in the window")
	require.Equal(t, "Gabriel O'Donnell", viewerRow.Name, "viewer always sees own legal name")
	require.Equal(t, 6, viewerRow.Rank)

	// Peer rows must NOT contain the legal name "Emma Patel" or "Diego Martinez".
	for _, r := range body.Rows {
		if r.IsViewer {
			continue
		}
		require.NotEqual(t, "Emma Patel", r.Name, "peer rows must be pseudonymized")
		require.NotEqual(t, "Diego Martinez", r.Name)
	}

	// Next-to-beat should reference rank-5's row (Emma at 250 XP),
	// with its pseudonym substituted.
	require.NotNil(t, body.NextToBeat, "non-rank-1 viewer should have a next-to-beat row")
	require.Equal(t, int64(250-40+1), body.NextToBeat.Gap, "gap should be (above - viewer) + 1")
	require.NotEqual(t, "Emma Patel", body.NextToBeat.Name, "next-to-beat name should be pseudonymized")
}

func TestGetCourseLeaderboard_OptedOutStudentDroppedFromRanking(t *testing.T) {
	const viewerID = 100
	app, walletRepo, userRepo, enrollmentRepo, accountRepo, currencyRepo, _ := setupLeaderboardHandler(viewerID, false)

	currencyRepo.On("FindByCode", mock.Anything, uint(1), models.ScopeSite, uint(1), "xp").
		Return(xpCurrency(), nil)
	accountRepo.On("FindByID", mock.Anything, uint(1)).
		Return(&models.Account{ID: 1, TenantMode: "higher_ed"}, nil)
	enrollmentRepo.On("ListActiveStudentEnrollmentsByCourse", mock.Anything, uint(1)).
		Return(course1ActiveEnrollments(), nil)
	enrollmentRepo.On("FindByUserAndCourse", mock.Anything, uint(viewerID), uint(1)).
		Return(&models.Enrollment{
			ID: 200, UserID: viewerID, CourseID: 1,
			Type: "StudentEnrollment", WorkflowState: "active",
			PseudonymPoolCode: "animals_v1",
		}, nil)
	userRepo.On("FindByID", mock.Anything, uint(viewerID)).
		Return(&models.User{ID: viewerID, Name: "Sofia Alvarez", Role: "user"}, nil).Maybe()

	candidateIDs := []uint{100, 101, 102, 103, 104, 105}
	// FilterPublicLeaderboardCandidates drops user 103 (Diego, opted out).
	filtered := []uint{100, 101, 102, 104, 105}
	userRepo.On("FilterPublicLeaderboardCandidates", mock.Anything, candidateIDs).
		Return(filtered, nil)
	rankRows := []repository.RankRow{
		{UserID: 100, LifetimeEarned: 640, Rank: 1},
		{UserID: 101, LifetimeEarned: 520, Rank: 2},
		{UserID: 102, LifetimeEarned: 440, Rank: 3},
		{UserID: 104, LifetimeEarned: 250, Rank: 4},
		{UserID: 105, LifetimeEarned: 40, Rank: 5},
	}
	walletRepo.On("RankByCurrency", mock.Anything, uint(1), filtered).
		Return(rankRows, nil)
	userRepo.On("FindByIDs", mock.Anything, mock.Anything).
		Return([]models.User{
			{ID: 100, Name: "Sofia Alvarez"},
			{ID: 101, Name: "Ben Carter"},
			{ID: 102, Name: "Chen Wei"},
			{ID: 104, Name: "Emma Patel"},
			{ID: 105, Name: "Gabriel O'Donnell"},
		}, nil)

	resp := requestLeaderboard(t, app, "/api/v1/courses/1/leaderboard")
	requireStatus(t, resp, http.StatusOK)

	var body leaderboardJSON
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	require.Equal(t, 5, body.TotalCandidates, "opted-out student excluded → 5 of 6 candidates remain")
	for _, r := range body.Rows {
		require.NotEqual(t, uint(103), r.UserID, "user 103 (opted out) must not appear in any row")
		require.NotEqual(t, "Diego Martinez", r.Name)
	}
}

func TestGetCourseLeaderboard_UnknownCurrencyReturns400(t *testing.T) {
	app, _, userRepo, enrollmentRepo, accountRepo, currencyRepo, _ := setupLeaderboardHandler(100, false)

	currencyRepo.On("FindByCode", mock.Anything, uint(1), models.ScopeSite, uint(1), "doesnotexist").
		Return((*models.GamificationCurrencyType)(nil), nil)
	accountRepo.On("FindByID", mock.Anything, uint(1)).
		Return(&models.Account{ID: 1, TenantMode: "k5"}, nil).Maybe()
	enrollmentRepo.On("FindByUserAndCourse", mock.Anything, uint(100), uint(1)).
		Return(&models.Enrollment{
			ID: 200, UserID: 100, CourseID: 1,
			Type: "StudentEnrollment", WorkflowState: "active",
		}, nil)
	userRepo.On("FindByID", mock.Anything, uint(100)).
		Return(&models.User{ID: 100, Name: "Sofia Alvarez", Role: "user"}, nil).Maybe()

	resp := requestLeaderboard(t, app, "/api/v1/courses/1/leaderboard?currency=doesnotexist")
	requireStatus(t, resp, http.StatusBadRequest)
}

func TestGetCourseLeaderboard_SnapshotMissingReturns404(t *testing.T) {
	const adminID = 1
	app, _, userRepo, _, _, currencyRepo, snapshotRepo := setupLeaderboardHandler(adminID, true)

	currencyRepo.On("FindByCode", mock.Anything, uint(1), models.ScopeSite, uint(1), "xp").
		Return(xpCurrency(), nil)
	userRepo.On("FindByID", mock.Anything, uint(adminID)).
		Return(&models.User{ID: adminID, Name: "Avery Admin", Role: "admin"}, nil).Maybe()
	// Snapshot lookup returns nil → handler emits 404.
	snapshotRepo.On("FindByWindow", mock.Anything, models.ScopeCourse, uint(1), uint(1), "weekly", mock.Anything).
		Return((*models.GamificationLeaderboardSnapshot)(nil), nil)

	resp := requestLeaderboard(t, app, "/api/v1/courses/1/leaderboard?offset_weeks=1")
	requireStatus(t, resp, http.StatusNotFound)
}

func TestGetCourseLeaderboard_SnapshotReadAppliesOptOutAtReadTime(t *testing.T) {
	const adminID = 1
	app, _, userRepo, enrollmentRepo, accountRepo, currencyRepo, snapshotRepo := setupLeaderboardHandler(adminID, true)

	currencyRepo.On("FindByCode", mock.Anything, uint(1), models.ScopeSite, uint(1), "xp").
		Return(xpCurrency(), nil)
	accountRepo.On("FindByID", mock.Anything, uint(1)).
		Return(&models.Account{ID: 1, TenantMode: "higher_ed"}, nil)
	userRepo.On("FindByID", mock.Anything, uint(adminID)).
		Return(&models.User{ID: adminID, Name: "Avery Admin", Role: "admin"}, nil).Maybe()

	// Snapshot payload was written before user 103 opted out. The
	// stored payload still contains them; the handler must drop them
	// when re-applying FilterPublicLeaderboardCandidates at read time.
	payload := []byte(`[
		{"user_id":100,"rank":1,"lifetime_earned":640},
		{"user_id":101,"rank":2,"lifetime_earned":520},
		{"user_id":102,"rank":3,"lifetime_earned":440},
		{"user_id":103,"rank":4,"lifetime_earned":360},
		{"user_id":104,"rank":5,"lifetime_earned":250},
		{"user_id":105,"rank":6,"lifetime_earned":140}
	]`)
	snap := &models.GamificationLeaderboardSnapshot{
		ID: 42, ScopeType: models.ScopeCourse, ScopeID: 1, CurrencyTypeID: 1,
		WindowKind:  "weekly",
		WindowStart: time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC),
		Payload:     payload,
	}
	snapshotRepo.On("FindByWindow", mock.Anything, models.ScopeCourse, uint(1), uint(1), "weekly", mock.Anything).
		Return(snap, nil)

	// User 103 opted out post-snapshot; FilterPublicLeaderboardCandidates
	// drops them at read time.
	allIDs := []uint{100, 101, 102, 103, 104, 105}
	filtered := []uint{100, 101, 102, 104, 105}
	userRepo.On("FilterPublicLeaderboardCandidates", mock.Anything, allIDs).
		Return(filtered, nil)

	enrollmentRepo.On("ListActiveStudentEnrollmentsByCourse", mock.Anything, uint(1)).
		Return(course1ActiveEnrollments(), nil)
	userRepo.On("FindByIDs", mock.Anything, mock.Anything).
		Return([]models.User{
			{ID: 100, Name: "Sofia Alvarez"},
			{ID: 101, Name: "Ben Carter"},
			{ID: 102, Name: "Chen Wei"},
			{ID: 104, Name: "Emma Patel"},
			{ID: 105, Name: "Gabriel O'Donnell"},
		}, nil)

	resp := requestLeaderboard(t, app, "/api/v1/courses/1/leaderboard?offset_weeks=1")
	requireStatus(t, resp, http.StatusOK)

	var body leaderboardJSON
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	require.Equal(t, "snapshot_weekly", body.WindowKind)
	require.Equal(t, 5, body.TotalCandidates, "user 103 dropped at read time → 5 of 6 remain")
	require.Equal(t, 5, len(body.Rows))
	// Ranks must be renumbered 1..5 (no gap at where rank 4 was).
	for i, r := range body.Rows {
		require.Equal(t, i+1, r.Rank, "rank should be re-numbered after opt-out drop")
		require.NotEqual(t, uint(103), r.UserID, "opted-out user must not appear")
	}
}

func TestGetCourseLeaderboard_NotEnrolledReturns403(t *testing.T) {
	const viewerID = 999
	app, _, userRepo, enrollmentRepo, _, _, _ := setupLeaderboardHandler(viewerID, false)

	enrollmentRepo.On("FindByUserAndCourse", mock.Anything, uint(viewerID), uint(1)).
		Return((*models.Enrollment)(nil), nil)
	userRepo.On("FindByID", mock.Anything, uint(viewerID)).
		Return(&models.User{ID: viewerID, Name: "Stranger", Role: "user"}, nil).Maybe()

	resp := requestLeaderboard(t, app, "/api/v1/courses/1/leaderboard")
	requireStatus(t, resp, http.StatusForbidden)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

type leaderboardJSON struct {
	CourseID        uint                 `json:"course_id"`
	CurrencyTypeID  uint                 `json:"currency_type_id"`
	CurrencyCode    string               `json:"currency_code"`
	CurrencyLabel   string               `json:"currency_label"`
	TotalCandidates int                  `json:"total_candidates"`
	Rows            []leaderboardRowJSON `json:"rows"`
	ViewerRank      int                  `json:"viewer_rank"`
	WindowKind      string               `json:"window_kind"`
	UsedPseudonyms  bool                 `json:"used_pseudonyms"`
	NextToBeat      *nextToBeatJSON      `json:"next_to_beat,omitempty"`
}

type leaderboardRowJSON struct {
	Rank           int    `json:"rank"`
	UserID         uint   `json:"user_id"`
	Name           string `json:"name"`
	LifetimeEarned int64  `json:"lifetime_earned"`
	IsViewer       bool   `json:"is_viewer"`
}

type nextToBeatJSON struct {
	Name           string `json:"name"`
	LifetimeEarned int64  `json:"lifetime_earned"`
	Gap            int64  `json:"gap"`
	CurrencyLabel  string `json:"currency_label"`
}

func requestLeaderboard(t *testing.T, app *fiber.App, path string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	resp, err := app.Test(req, int(5*time.Second/time.Millisecond))
	require.NoError(t, err)
	return resp
}

func requireStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		body := make([]byte, 1024)
		n, _ := resp.Body.Read(body)
		t.Fatalf("status: want %d got %d, body: %s", want, resp.StatusCode, strings.TrimSpace(string(body[:n])))
	}
}
