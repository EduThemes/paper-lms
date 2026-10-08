package handlers_test

// B069 (account-scoped admin endpoints that load rows by bare id): grading
// period groups and periods. The URL :account_id is now pinned to the
// caller's tenant by RequireAdmin, but the handlers loaded the group and
// period by bare id, so /accounts/<own>/grading_period_groups/<other
// tenant's group> still reached another tenant's rows. All data is made up.

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

func gradingPeriodTenantApp(groupRepo *mocks.MockGradingPeriodGroupRepository, periodRepo *mocks.MockGradingPeriodRepository) *handlers.GradingPeriodHandler {
	return handlers.NewGradingPeriodHandler(service.NewGradingPeriodService(groupRepo, periodRepo))
}

func TestGradingPeriods_OtherTenantsGroupIsNotFound(t *testing.T) {
	groupRepo := new(mocks.MockGradingPeriodGroupRepository)
	periodRepo := new(mocks.MockGradingPeriodRepository)
	// Group 40 belongs to tenant 1; the caller is an admin of tenant 2.
	groupRepo.On("FindByID", mock.Anything, uint(40), uint(0)).
		Return(&models.GradingPeriodGroup{ID: 40, AccountID: 1, Title: "Tenant 1 year"}, nil)
	periodRepo.On("FindByID", mock.Anything, uint(41), uint(0)).
		Return(&models.GradingPeriod{ID: 41, GradingPeriodGroupID: 40, Title: "Q1"}, nil)
	h := gradingPeriodTenantApp(groupRepo, periodRepo)

	app := testutil.SetupTestApp()
	app.Use(callerStub(9, 2))
	base := "/api/v1/accounts/:account_id/grading_period_groups/:group_id"
	app.Get(base, h.GetGroup)
	app.Put(base, h.UpdateGroup)
	app.Delete(base, h.DeleteGroup)
	app.Get(base+"/grading_periods", h.ListPeriods)
	app.Post(base+"/grading_periods", h.CreatePeriod)
	app.Get(base+"/grading_periods/:period_id", h.GetPeriod)
	app.Put(base+"/grading_periods/:period_id", h.UpdatePeriod)
	app.Delete(base+"/grading_periods/:period_id", h.DeletePeriod)

	reqs := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/accounts/2/grading_period_groups/40"},
		{http.MethodPut, "/api/v1/accounts/2/grading_period_groups/40"},
		{http.MethodDelete, "/api/v1/accounts/2/grading_period_groups/40"},
		{http.MethodGet, "/api/v1/accounts/2/grading_period_groups/40/grading_periods"},
		{http.MethodPost, "/api/v1/accounts/2/grading_period_groups/40/grading_periods"},
		{http.MethodGet, "/api/v1/accounts/2/grading_period_groups/40/grading_periods/41"},
		{http.MethodPut, "/api/v1/accounts/2/grading_period_groups/40/grading_periods/41"},
		{http.MethodDelete, "/api/v1/accounts/2/grading_period_groups/40/grading_periods/41"},
	}
	for _, r := range reqs {
		resp := testutil.MakeRequest(app, r.method, r.path, testutil.JSONBody(map[string]interface{}{
			"grading_period_group": map[string]interface{}{"title": "hijacked"},
			"grading_period":       map[string]interface{}{"title": "hijacked", "is_closed": true},
		}))
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "%s %s", r.method, r.path)
	}
	groupRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	groupRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
	periodRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	periodRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	periodRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
	periodRepo.AssertNotCalled(t, "ListByGroupID", mock.Anything, mock.Anything, mock.Anything)
}

// A period addressed under the wrong (own-tenant) group is also a 404: the
// period must belong to the group named in the URL.
func TestGradingPeriods_PeriodMustBelongToURLGroup(t *testing.T) {
	groupRepo := new(mocks.MockGradingPeriodGroupRepository)
	periodRepo := new(mocks.MockGradingPeriodRepository)
	groupRepo.On("FindByID", mock.Anything, uint(50), uint(0)).
		Return(&models.GradingPeriodGroup{ID: 50, AccountID: 2, Title: "Own year"}, nil)
	periodRepo.On("FindByID", mock.Anything, uint(41), uint(0)).
		Return(&models.GradingPeriod{ID: 41, GradingPeriodGroupID: 40, Title: "Other tenant Q1"}, nil)
	h := gradingPeriodTenantApp(groupRepo, periodRepo)

	app := testutil.SetupTestApp()
	app.Use(callerStub(9, 2))
	app.Delete("/api/v1/accounts/:account_id/grading_period_groups/:group_id/grading_periods/:period_id", h.DeletePeriod)

	resp := testutil.MakeRequest(app, http.MethodDelete, "/api/v1/accounts/2/grading_period_groups/50/grading_periods/41", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	periodRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}

// Own-tenant access still works.
func TestGradingPeriods_OwnTenantGroupReadable(t *testing.T) {
	groupRepo := new(mocks.MockGradingPeriodGroupRepository)
	periodRepo := new(mocks.MockGradingPeriodRepository)
	groupRepo.On("FindByID", mock.Anything, uint(50), uint(0)).
		Return(&models.GradingPeriodGroup{ID: 50, AccountID: 2, Title: "Own year"}, nil)
	periodRepo.On("ListByGroupID", mock.Anything, uint(50), uint(0)).
		Return([]models.GradingPeriod{{ID: 51, GradingPeriodGroupID: 50, Title: "Q1"}}, nil)
	h := gradingPeriodTenantApp(groupRepo, periodRepo)

	app := testutil.SetupTestApp()
	app.Use(callerStub(9, 2))
	app.Get("/api/v1/accounts/:account_id/grading_period_groups/:group_id", h.GetGroup)
	app.Get("/api/v1/accounts/:account_id/grading_period_groups/:group_id/grading_periods", h.ListPeriods)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/accounts/2/grading_period_groups/50", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp = testutil.MakeRequest(app, http.MethodGet, "/api/v1/accounts/2/grading_period_groups/50/grading_periods", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
