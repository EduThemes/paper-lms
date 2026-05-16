package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// ----------------------------------------------------------------------
// mockGamRuleRepo — local stub for the rule repository surface.
// Mirrors the W2-B/W2-D mock pattern.
// ----------------------------------------------------------------------

type mockGamRuleRepo struct{ mock.Mock }

func (m *mockGamRuleRepo) Create(ctx context.Context, r *models.GamificationRule) error {
	args := m.Called(ctx, r)
	return args.Error(0)
}
func (m *mockGamRuleRepo) FindByID(ctx context.Context, id uint) (*models.GamificationRule, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GamificationRule), args.Error(1)
}
func (m *mockGamRuleRepo) Update(ctx context.Context, r *models.GamificationRule) error {
	args := m.Called(ctx, r)
	return args.Error(0)
}
func (m *mockGamRuleRepo) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *mockGamRuleRepo) ListEnabledByScope(ctx context.Context, scopeType models.GamificationScopeType, scopeID uint) ([]models.GamificationRule, error) {
	args := m.Called(ctx, scopeType, scopeID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.GamificationRule), args.Error(1)
}
func (m *mockGamRuleRepo) ListByTenantID(ctx context.Context, tenantID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GamificationRule], error) {
	args := m.Called(ctx, tenantID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.GamificationRule]), args.Error(1)
}
func (m *mockGamRuleRepo) ListByScope(ctx context.Context, tenantID uint, scopeType models.GamificationScopeType, scopeID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GamificationRule], error) {
	args := m.Called(ctx, tenantID, scopeType, scopeID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.GamificationRule]), args.Error(1)
}
func (m *mockGamRuleRepo) RecordEvaluation(ctx context.Context, e *models.GamificationRuleEvaluation) error {
	args := m.Called(ctx, e)
	return args.Error(0)
}
func (m *mockGamRuleRepo) ListEvaluationsForUserRule(ctx context.Context, userID, ruleID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GamificationRuleEvaluation], error) {
	args := m.Called(ctx, userID, ruleID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.GamificationRuleEvaluation]), args.Error(1)
}
func (m *mockGamRuleRepo) LastFiringForUserRule(ctx context.Context, userID, ruleID uint) (*models.GamificationRuleEvaluation, error) {
	args := m.Called(ctx, userID, ruleID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GamificationRuleEvaluation), args.Error(1)
}
func (m *mockGamRuleRepo) CountFiringsInWindow(ctx context.Context, userID, ruleID uint, since time.Time) (int64, error) {
	args := m.Called(ctx, userID, ruleID, since)
	return args.Get(0).(int64), args.Error(1)
}

var _ repository.GamificationRuleRepository = (*mockGamRuleRepo)(nil)

// ----------------------------------------------------------------------
// Rule-handler harness.
// Mounts only the rule routes + vocabulary; existing W2-A..D tests
// keep their own setup helper unchanged.
// ----------------------------------------------------------------------

func setupRuleHandler(callerID uint, isAdmin bool) (*fiber.App, *mockGamRuleRepo) {
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

	app.Get("/api/v1/gamification/vocabulary", h.GetVocabulary)
	app.Get("/api/v1/gamification/rules", h.ListRules)
	app.Get("/api/v1/gamification/rules/:id", h.GetRule)
	app.Post("/api/v1/gamification/rules", h.CreateRule)
	app.Patch("/api/v1/gamification/rules/:id", h.PatchRule)
	app.Delete("/api/v1/gamification/rules/:id", h.DeleteRule)
	// Course-scope mirror.
	app.Get("/api/v1/courses/:course_id/gamification/rules", h.ListRules)
	app.Get("/api/v1/courses/:course_id/gamification/rules/:id", h.GetRule)
	app.Post("/api/v1/courses/:course_id/gamification/rules", h.CreateRule)
	app.Patch("/api/v1/courses/:course_id/gamification/rules/:id", h.PatchRule)
	app.Delete("/api/v1/courses/:course_id/gamification/rules/:id", h.DeleteRule)

	return app, ruleRepo
}

// validRuleBody returns a CreateRule payload that's well-formed end to
// end: trigger validates, condition_set decodes via predicates.Decode,
// effects decodes via effects.Decode.
func validRuleBody() []byte {
	body := map[string]any{
		"name":           "Award XP on quiz completion",
		"description":    "",
		"audience_level": "higher_ed",
		"trigger_event": map[string]any{
			"kind":        "OnEvent",
			"verb":        "completed",
			"object_type": "Quiz",
		},
		"condition_set": map[string]any{
			"kind":     "ConditionSet",
			"op":       "AND",
			"children": []any{},
		},
		"effects": []any{
			map[string]any{"kind": "AwardCurrency", "code": "xp", "amount": 10},
		},
	}
	b, _ := json.Marshal(body)
	return b
}

// ----------------------------------------------------------------------
// Vocabulary endpoint.
// ----------------------------------------------------------------------

func TestGetVocabulary_ServesCatalog(t *testing.T) {
	app, _ := setupRuleHandler(1, true)
	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/gamification/vocabulary", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)

	for _, k := range []string{"triggers", "predicates", "effects", "set_ops", "audiences", "scopes", "windows", "mastery_levels"} {
		_, ok := body[k]
		assert.Truef(t, ok, "expected key %q in vocabulary response", k)
	}

	triggers, _ := body["triggers"].([]any)
	require.NotEmpty(t, triggers, "TriggerCatalog must not be empty")
	predicates, _ := body["predicates"].([]any)
	require.GreaterOrEqual(t, len(predicates), 7, "PredicateCatalog must contain the 7 atoms shipped on main")
}

// ----------------------------------------------------------------------
// CreateRule.
// ----------------------------------------------------------------------

func TestCreateRule_HappyPath_SiteScope(t *testing.T) {
	app, ruleRepo := setupRuleHandler(7, true)
	// callerAccountID defaults to 1 when no account_id locals is set —
	// matches the real router's behavior in a single-tenant setup.
	ruleRepo.On("Create", mock.Anything, mock.MatchedBy(func(r *models.GamificationRule) bool {
		return r.ScopeType == models.ScopeSite &&
			r.TenantID == 1 &&
			r.Name == "Award XP on quiz completion" &&
			r.AudienceLevel == models.AudienceHigherEd &&
			r.Enabled
	})).Return(nil).Run(func(args mock.Arguments) {
		r := args.Get(1).(*models.GamificationRule)
		r.ID = 101
		r.CreatedAt = time.Now()
		r.UpdatedAt = r.CreatedAt
	})

	resp := testutil.MakeRequest(app, http.MethodPost, "/api/v1/gamification/rules", bytes.NewReader(validRuleBody()))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	body, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	assert.Equal(t, float64(101), body["id"])
	assert.Equal(t, "site", body["scope_type"])
	assert.Equal(t, true, body["enabled"])

	ruleRepo.AssertCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestCreateRule_CourseScope_ResolvedFromURL(t *testing.T) {
	app, ruleRepo := setupRuleHandler(7, false)
	ruleRepo.On("Create", mock.Anything, mock.MatchedBy(func(r *models.GamificationRule) bool {
		return r.ScopeType == models.ScopeCourse && r.ScopeID == 42 && r.TenantID == 1
	})).Return(nil)

	resp := testutil.MakeRequest(app, http.MethodPost, "/api/v1/courses/42/gamification/rules", bytes.NewReader(validRuleBody()))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestCreateRule_HonorsExplicitEnabledFalse(t *testing.T) {
	// The bool-default class fix: a rule posted with enabled:false must
	// be persisted as enabled=false, not silently flipped back to the
	// schema DEFAULT TRUE. The repo-level raw INSERT guarantees this;
	// the handler must not pre-default it away first.
	app, ruleRepo := setupRuleHandler(7, true)

	body := map[string]any{}
	require.NoError(t, json.Unmarshal(validRuleBody(), &body))
	body["enabled"] = false
	payload, _ := json.Marshal(body)

	ruleRepo.On("Create", mock.Anything, mock.MatchedBy(func(r *models.GamificationRule) bool {
		return r.Enabled == false
	})).Return(nil)

	resp := testutil.MakeRequest(app, http.MethodPost, "/api/v1/gamification/rules", bytes.NewReader(payload))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestCreateRule_RejectsBadAudience(t *testing.T) {
	app, _ := setupRuleHandler(7, true)
	body := map[string]any{}
	require.NoError(t, json.Unmarshal(validRuleBody(), &body))
	body["audience_level"] = "preschool"
	payload, _ := json.Marshal(body)

	resp := testutil.MakeRequest(app, http.MethodPost, "/api/v1/gamification/rules", bytes.NewReader(payload))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateRule_RejectsUnknownVerb(t *testing.T) {
	app, _ := setupRuleHandler(7, true)
	body := map[string]any{}
	require.NoError(t, json.Unmarshal(validRuleBody(), &body))
	body["trigger_event"] = map[string]any{"kind": "OnEvent", "verb": "wiggled", "object_type": "Quiz"}
	payload, _ := json.Marshal(body)

	resp := testutil.MakeRequest(app, http.MethodPost, "/api/v1/gamification/rules", bytes.NewReader(payload))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateRule_RejectsUnknownPredicate(t *testing.T) {
	app, _ := setupRuleHandler(7, true)
	body := map[string]any{}
	require.NoError(t, json.Unmarshal(validRuleBody(), &body))
	body["condition_set"] = map[string]any{
		"kind": "ConditionSet", "op": "AND", "children": []any{
			map[string]any{"kind": "Imagined", "x": 1},
		},
	}
	payload, _ := json.Marshal(body)

	resp := testutil.MakeRequest(app, http.MethodPost, "/api/v1/gamification/rules", bytes.NewReader(payload))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateRule_RejectsUnknownEffect(t *testing.T) {
	app, _ := setupRuleHandler(7, true)
	body := map[string]any{}
	require.NoError(t, json.Unmarshal(validRuleBody(), &body))
	body["effects"] = []any{map[string]any{"kind": "TeleportUser"}}
	payload, _ := json.Marshal(body)

	resp := testutil.MakeRequest(app, http.MethodPost, "/api/v1/gamification/rules", bytes.NewReader(payload))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateRule_RejectsNegativeCooldown(t *testing.T) {
	app, _ := setupRuleHandler(7, true)
	body := map[string]any{}
	require.NoError(t, json.Unmarshal(validRuleBody(), &body))
	body["cooldown_seconds"] = -5
	payload, _ := json.Marshal(body)

	resp := testutil.MakeRequest(app, http.MethodPost, "/api/v1/gamification/rules", bytes.NewReader(payload))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateRule_RejectsBadMaxPerWindow(t *testing.T) {
	app, _ := setupRuleHandler(7, true)
	body := map[string]any{}
	require.NoError(t, json.Unmarshal(validRuleBody(), &body))
	body["max_per_window"] = map[string]any{"window": "fortnight", "count": 3}
	payload, _ := json.Marshal(body)

	resp := testutil.MakeRequest(app, http.MethodPost, "/api/v1/gamification/rules", bytes.NewReader(payload))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ----------------------------------------------------------------------
// PatchRule.
// ----------------------------------------------------------------------

func TestPatchRule_TogglesEnabled(t *testing.T) {
	app, ruleRepo := setupRuleHandler(7, true)
	existing := &models.GamificationRule{
		ID:            42,
		TenantID:      1,
		ScopeType:     models.ScopeSite,
		ScopeID:       1,
		AudienceLevel: models.AudienceHigherEd,
		Name:          "x",
		Enabled:       true,
		TriggerEvent:  datatypes.JSON(`{"kind":"OnEvent","verb":"completed","object_type":"Quiz"}`),
		ConditionSet:  datatypes.JSON(`{"kind":"ConditionSet","op":"AND","children":[]}`),
		Effects:       datatypes.JSON(`[{"kind":"AwardCurrency","code":"xp","amount":10}]`),
	}
	ruleRepo.On("FindByID", mock.Anything, uint(42)).Return(existing, nil)
	ruleRepo.On("Update", mock.Anything, mock.MatchedBy(func(r *models.GamificationRule) bool {
		return r.ID == 42 && r.Enabled == false
	})).Return(nil)

	body := []byte(`{"enabled": false}`)
	resp := testutil.MakeRequest(app, http.MethodPatch, "/api/v1/gamification/rules/42", bytes.NewReader(body))
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestPatchRule_ScopeMismatch_403(t *testing.T) {
	app, ruleRepo := setupRuleHandler(7, false) // course path will resolve to course scope
	existing := &models.GamificationRule{
		ID: 42, TenantID: 1, ScopeType: models.ScopeSite, ScopeID: 1,
		AudienceLevel: models.AudienceHigherEd, Name: "x", Enabled: true,
		TriggerEvent: datatypes.JSON(`{"kind":"OnEvent","verb":"completed","object_type":"Quiz"}`),
		ConditionSet: datatypes.JSON(`{"kind":"ConditionSet","op":"AND","children":[]}`),
		Effects:      datatypes.JSON(`[{"kind":"AwardCurrency","code":"xp","amount":10}]`),
	}
	ruleRepo.On("FindByID", mock.Anything, uint(42)).Return(existing, nil)

	resp := testutil.MakeRequest(app, http.MethodPatch, "/api/v1/courses/99/gamification/rules/42", bytes.NewReader([]byte(`{"enabled":false}`)))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestPatchRule_RejectsInvalidMergedState(t *testing.T) {
	// Removing required fields via patch must surface as 400, not 200
	// with a broken stored rule.
	app, ruleRepo := setupRuleHandler(7, true)
	existing := &models.GamificationRule{
		ID: 42, TenantID: 1, ScopeType: models.ScopeSite, ScopeID: 1,
		AudienceLevel: models.AudienceHigherEd, Name: "x", Enabled: true,
		TriggerEvent: datatypes.JSON(`{"kind":"OnEvent","verb":"completed","object_type":"Quiz"}`),
		ConditionSet: datatypes.JSON(`{"kind":"ConditionSet","op":"AND","children":[]}`),
		Effects:      datatypes.JSON(`[{"kind":"AwardCurrency","code":"xp","amount":10}]`),
	}
	ruleRepo.On("FindByID", mock.Anything, uint(42)).Return(existing, nil)

	// Patch attempts to swap in a malformed effect.
	resp := testutil.MakeRequest(app, http.MethodPatch, "/api/v1/gamification/rules/42",
		bytes.NewReader([]byte(`{"effects":[{"kind":"Mystery"}]}`)))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ----------------------------------------------------------------------
// DeleteRule + ListRules + GetRule basics.
// ----------------------------------------------------------------------

func TestDeleteRule_NoContent(t *testing.T) {
	app, ruleRepo := setupRuleHandler(7, true)
	existing := &models.GamificationRule{
		ID: 42, TenantID: 1, ScopeType: models.ScopeSite, ScopeID: 1,
		AudienceLevel: models.AudienceHigherEd,
	}
	ruleRepo.On("FindByID", mock.Anything, uint(42)).Return(existing, nil)
	ruleRepo.On("Delete", mock.Anything, uint(42)).Return(nil)

	resp := testutil.MakeRequest(app, http.MethodDelete, "/api/v1/gamification/rules/42", nil)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestListRules_FiltersByRouteScope(t *testing.T) {
	app, ruleRepo := setupRuleHandler(7, false)
	ruleRepo.On("ListByScope", mock.Anything, uint(1), models.ScopeCourse, uint(42), mock.Anything).
		Return(&repository.PaginatedResult[models.GamificationRule]{
			Items: []models.GamificationRule{
				{ID: 1, TenantID: 1, ScopeType: models.ScopeCourse, ScopeID: 42,
					AudienceLevel: models.AudienceHigherEd, Name: "course rule"},
			},
			TotalCount: 1, Page: 1, PerPage: 50,
		}, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/courses/42/gamification/rules", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestGetRule_RoundTripsJSONB(t *testing.T) {
	app, ruleRepo := setupRuleHandler(7, true)
	existing := &models.GamificationRule{
		ID: 42, TenantID: 1, ScopeType: models.ScopeSite, ScopeID: 1,
		AudienceLevel: models.AudienceHigherEd, Name: "x", Enabled: true,
		TriggerEvent: datatypes.JSON(`{"kind":"OnEvent","verb":"completed","object_type":"Quiz"}`),
		ConditionSet: datatypes.JSON(`{"kind":"ConditionSet","op":"AND","children":[]}`),
		Effects:      datatypes.JSON(`[{"kind":"AwardCurrency","code":"xp","amount":10}]`),
	}
	ruleRepo.On("FindByID", mock.Anything, uint(42)).Return(existing, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/gamification/rules/42", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	te, ok := body["trigger_event"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "OnEvent", te["kind"])
	assert.Equal(t, "completed", te["verb"])
}

// ----------------------------------------------------------------------
// Sanity check: server-imported vocabulary catalog non-empty.
// ----------------------------------------------------------------------

func TestCatalog_NonEmptySanityCheck(t *testing.T) {
	assert.NotEmpty(t, gamification.TriggerCatalog)
	assert.NotEmpty(t, gamification.PredicateCatalog)
	assert.NotEmpty(t, gamification.EffectCatalog)
	assert.NotEmpty(t, gamification.VerbCatalog)
	assert.NotEmpty(t, gamification.ObjectCatalog)
}
