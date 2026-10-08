package handlers_test

// F-013 parent tie for the rest of the outcome surface: reads, updates,
// outcome creation, result creation and alignments must refuse (404) an
// outcome group / outcome / assignment / alignment that belongs to another
// course than the URL's :course_id, and never reach the write.

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

type outcomeTieFixture struct {
	groupRepo      *mocks.MockLearningOutcomeGroupRepository
	outcomeRepo    *mocks.MockLearningOutcomeRepository
	alignmentRepo  *mocks.MockOutcomeAlignmentRepository
	assignmentRepo *mocks.MockAssignmentRepository
	app            *fiber.App
}

// Course 10 is the caller's; group 55 / outcome 77 belong to course 10,
// group 56 / outcome 78 to course 99 (same tenant 1).
func newOutcomeTieFixture(t *testing.T) *outcomeTieFixture {
	t.Helper()
	f := &outcomeTieFixture{
		groupRepo:      new(mocks.MockLearningOutcomeGroupRepository),
		outcomeRepo:    new(mocks.MockLearningOutcomeRepository),
		alignmentRepo:  new(mocks.MockOutcomeAlignmentRepository),
		assignmentRepo: new(mocks.MockAssignmentRepository),
	}
	f.groupRepo.On("FindByID", mock.Anything, uint(55), uint(1)).Return(&models.LearningOutcomeGroup{ID: 55, ContextType: "Course", ContextID: 10, Title: "Ours"}, nil).Maybe()
	f.groupRepo.On("FindByID", mock.Anything, uint(56), uint(1)).Return(&models.LearningOutcomeGroup{ID: 56, ContextType: "Course", ContextID: 99, Title: "Theirs"}, nil).Maybe()
	f.outcomeRepo.On("FindByID", mock.Anything, uint(77), uint(1)).Return(&models.LearningOutcome{ID: 77, ContextType: "Course", ContextID: 10, Title: "Ours", OutcomeGroupID: 55, PointsPossible: 5, MasteryPoints: 3}, nil).Maybe()
	f.outcomeRepo.On("FindByID", mock.Anything, uint(78), uint(1)).Return(&models.LearningOutcome{ID: 78, ContextType: "Course", ContextID: 99, Title: "Theirs", OutcomeGroupID: 56}, nil).Maybe()
	f.assignmentRepo.On("FindByID", mock.Anything, uint(300), uint(1)).Return(&models.Assignment{ID: 300, CourseID: 10}, nil).Maybe()
	f.assignmentRepo.On("FindByID", mock.Anything, uint(301), uint(1)).Return(&models.Assignment{ID: 301, CourseID: 99}, nil).Maybe()
	f.alignmentRepo.On("ListByCourseID", mock.Anything, uint(10), uint(1)).Return([]models.OutcomeAlignment{
		{ID: 5, CourseID: 10, LearningOutcomeID: 77, AssignmentID: 300},
	}, nil).Maybe()

	svc := service.NewLearningOutcomeService(f.groupRepo, f.outcomeRepo, &noopOutcomeResultRepo{})
	h := handlers.NewLearningOutcomeHandler(svc, f.alignmentRepo, nil)
	h.SetAssignmentRepo(f.assignmentRepo)

	app := testutil.SetupTestApp()
	app.Use(rubricOutcomeAuthStub(1))
	app.Get("/courses/:course_id/outcome_groups/:group_id", h.GetGroup)
	app.Put("/courses/:course_id/outcome_groups/:group_id", h.UpdateGroup)
	app.Get("/courses/:course_id/outcome_groups/:group_id/outcomes", h.ListOutcomes)
	app.Post("/courses/:course_id/outcome_groups/:group_id/outcomes", h.CreateOutcome)
	app.Get("/courses/:course_id/outcomes/:outcome_id", h.GetOutcome)
	app.Put("/courses/:course_id/outcomes/:outcome_id", h.UpdateOutcome)
	app.Post("/courses/:course_id/outcome_results", h.CreateResult)
	app.Post("/courses/:course_id/outcome_alignments", h.CreateAlignment)
	app.Delete("/courses/:course_id/outcome_alignments/:alignment_id", h.DeleteAlignment)
	f.app = app
	return f
}

func TestOutcomes_CrossCourseIDsReturn404(t *testing.T) {
	f := newOutcomeTieFixture(t)
	cases := []struct {
		method, path, body string
	}{
		{http.MethodGet, "/courses/10/outcome_groups/56", ""},
		{http.MethodPut, "/courses/10/outcome_groups/56", `{"title":"pwned"}`},
		{http.MethodGet, "/courses/10/outcome_groups/56/outcomes", ""},
		{http.MethodPost, "/courses/10/outcome_groups/56/outcomes", `{"title":"x"}`},
		{http.MethodGet, "/courses/10/outcomes/78", ""},
		{http.MethodPut, "/courses/10/outcomes/78", `{"mastery_points":0}`},
		{http.MethodPost, "/courses/10/outcome_results", `{"user_id":42,"learning_outcome_id":78,"score":5}`},
		{http.MethodPost, "/courses/10/outcome_alignments", `{"learning_outcome_id":78,"assignment_id":300}`},
		{http.MethodPost, "/courses/10/outcome_alignments", `{"learning_outcome_id":77,"assignment_id":301}`},
		{http.MethodDelete, "/courses/10/outcome_alignments/6", ""},
	}
	for _, tc := range cases {
		resp := testutil.MakeRequest(f.app, tc.method, tc.path, body(tc.body))
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "%s %s %s", tc.method, tc.path, tc.body)
	}
	f.groupRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	f.outcomeRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	f.outcomeRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	f.alignmentRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	f.alignmentRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}

func TestOutcomes_SameCourseIDsStillWork(t *testing.T) {
	f := newOutcomeTieFixture(t)
	f.groupRepo.On("Update", mock.Anything, mock.Anything).Return(nil)
	f.outcomeRepo.On("Update", mock.Anything, mock.Anything).Return(nil)
	f.alignmentRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	f.alignmentRepo.On("Delete", mock.Anything, uint(5)).Return(nil)

	resp := testutil.MakeRequest(f.app, http.MethodGet, "/courses/10/outcome_groups/55", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp = testutil.MakeRequest(f.app, http.MethodPut, "/courses/10/outcome_groups/55", body(`{"title":"Renamed"}`))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp = testutil.MakeRequest(f.app, http.MethodPut, "/courses/10/outcome_groups/55", body(`{"parent_outcome_group_id":56}`))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "re-parenting under another course's group")
	resp = testutil.MakeRequest(f.app, http.MethodGet, "/courses/10/outcomes/77", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp = testutil.MakeRequest(f.app, http.MethodPost, "/courses/10/outcome_alignments", body(`{"learning_outcome_id":77,"assignment_id":300}`))
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	resp = testutil.MakeRequest(f.app, http.MethodDelete, "/courses/10/outcome_alignments/5", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	f.alignmentRepo.AssertCalled(t, "Delete", mock.Anything, uint(5))
}

// Changing an outcome's ratings without points_possible re-derives
// points_possible from the highest rating (results are scaled onto it);
// outcome 77 starts at points_possible 5, the new scale tops out at 4.
func TestUpdateOutcome_NewRatingsRederivePointsPossible(t *testing.T) {
	f := newOutcomeTieFixture(t)
	var saved *models.LearningOutcome
	f.outcomeRepo.On("Update", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		saved = args.Get(1).(*models.LearningOutcome)
	}).Return(nil)

	resp := testutil.MakeRequest(f.app, http.MethodPut, "/courses/10/outcomes/77",
		body(`{"ratings":"[{\"description\":\"Exceeds\",\"points\":4},{\"description\":\"Meets\",\"points\":3}]"}`))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	if assert.NotNil(t, saved) {
		assert.Equal(t, 4.0, saved.PointsPossible)
	}
}
