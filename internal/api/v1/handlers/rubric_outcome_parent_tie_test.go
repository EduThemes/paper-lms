package handlers_test

// F-012 (rubric/outcome family): Course-child Delete handlers for
// rubrics, outcome groups, and outcomes must refuse when the resource's
// parent course doesn't match the URL :course_id. Mirrors the canonical
// shape locked by parent_tie_test.go (assignment family).
//
// Each test asserts:
//   1. Cross-course request returns 404 (NOT 200, NOT 500).
//   2. The destructive Delete is NOT called on the repo.

import (
	"context"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

func rubricOutcomeAuthStub(callerAccountID uint) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(7))
		c.Locals("account_id", callerAccountID)
		c.Locals("enrollment_type", "TeacherEnrollment")
		return c.Next()
	}
}

// noopOutcomeResultRepo is a minimal stub. The Delete handlers under
// test never hit the result repo, but LearningOutcomeService's
// constructor requires one.
type noopOutcomeResultRepo struct{}

func (n *noopOutcomeResultRepo) Create(ctx context.Context, r *models.LearningOutcomeResult) error {
	return nil
}
func (n *noopOutcomeResultRepo) FindByID(ctx context.Context, id uint) (*models.LearningOutcomeResult, error) {
	return nil, nil
}
func (n *noopOutcomeResultRepo) Update(ctx context.Context, r *models.LearningOutcomeResult) error {
	return nil
}
func (n *noopOutcomeResultRepo) Upsert(ctx context.Context, r *models.LearningOutcomeResult) (*bool, error) {
	return nil, nil
}
func (n *noopOutcomeResultRepo) ListByOutcomeID(ctx context.Context, outcomeID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LearningOutcomeResult], error) {
	return &repository.PaginatedResult[models.LearningOutcomeResult]{}, nil
}
func (n *noopOutcomeResultRepo) ListByUserAndContext(ctx context.Context, userID uint, contextType string, contextID uint) ([]models.LearningOutcomeResult, error) {
	return nil, nil
}
func (n *noopOutcomeResultRepo) ListByUserAndOutcomeIDs(ctx context.Context, userID uint, outcomeIDs []uint) ([]models.LearningOutcomeResult, error) {
	return nil, nil
}

var _ repository.LearningOutcomeResultRepository = (*noopOutcomeResultRepo)(nil)

// TestDeleteRubric_RejectsCrossCourseID — F-011 / F-012 repro. Teacher
// in course 10 (tenant 1) attempts DELETE /courses/10/rubrics/<id whose
// rubric.ContextID = 99>. Mock returns that rubric; handler MUST 404
// before the destructive Delete fires.
func TestDeleteRubric_RejectsCrossCourseID(t *testing.T) {
	rubricRepo := new(mocks.MockRubricRepository)
	assocRepo := new(mocks.MockRubricAssociationRepository)
	assessRepo := new(mocks.MockRubricAssessmentRepository)

	// FindByID under the caller's tenant returns a rubric belonging
	// to a different course; the handler must NOT proceed to Delete.
	rubricRepo.On("FindByID", mock.Anything, uint(42), uint(1)).Return(&models.Rubric{
		ID:          42,
		ContextType: "Course",
		ContextID:   99, // ← NOT the URL's :course_id (10)
	}, nil)

	rubricSvc := service.NewRubricService(rubricRepo, assocRepo, assessRepo)
	h := handlers.NewRubricHandler(rubricSvc)
	app := testutil.SetupTestApp()
	app.Delete("/courses/:course_id/rubrics/:rubric_id",
		rubricOutcomeAuthStub(1),
		h.DeleteRubric)

	resp := testutil.MakeRequest(app, http.MethodDelete, "/courses/10/rubrics/42", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	// Critical: Delete MUST NOT have been called on the cross-course
	// row. Mock assertion locks the contract.
	rubricRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything, mock.Anything)
}

// TestDeleteRubric_RejectsAccountContextOnCourseRoute — even within
// the caller's tenant, the course-scoped DELETE route MUST NOT delete
// an Account-context rubric. Account-level deletes belong on a
// separate /accounts/:account_id/rubrics route (not yet implemented).
func TestDeleteRubric_RejectsAccountContextOnCourseRoute(t *testing.T) {
	rubricRepo := new(mocks.MockRubricRepository)
	assocRepo := new(mocks.MockRubricAssociationRepository)
	assessRepo := new(mocks.MockRubricAssessmentRepository)

	rubricRepo.On("FindByID", mock.Anything, uint(42), uint(1)).Return(&models.Rubric{
		ID:          42,
		ContextType: "Account",
		ContextID:   1, // tenant-scope match, but wrong context_type
	}, nil)

	rubricSvc := service.NewRubricService(rubricRepo, assocRepo, assessRepo)
	h := handlers.NewRubricHandler(rubricSvc)
	app := testutil.SetupTestApp()
	app.Delete("/courses/:course_id/rubrics/:rubric_id",
		rubricOutcomeAuthStub(1),
		h.DeleteRubric)

	resp := testutil.MakeRequest(app, http.MethodDelete, "/courses/10/rubrics/42", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	rubricRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything, mock.Anything)
}

// TestDeleteRubric_AcceptsSameCourseID — happy path: course 10's own
// rubric 42 deletes cleanly and Delete is invoked WITH the caller's
// accountID.
func TestDeleteRubric_AcceptsSameCourseID(t *testing.T) {
	rubricRepo := new(mocks.MockRubricRepository)
	assocRepo := new(mocks.MockRubricAssociationRepository)
	assessRepo := new(mocks.MockRubricAssessmentRepository)

	rubricRepo.On("FindByID", mock.Anything, uint(42), uint(1)).Return(&models.Rubric{
		ID:          42,
		ContextType: "Course",
		ContextID:   10, // matches URL :course_id
	}, nil)
	rubricRepo.On("Delete", mock.Anything, uint(42), uint(1)).Return(nil)

	rubricSvc := service.NewRubricService(rubricRepo, assocRepo, assessRepo)
	h := handlers.NewRubricHandler(rubricSvc)
	app := testutil.SetupTestApp()
	app.Delete("/courses/:course_id/rubrics/:rubric_id",
		rubricOutcomeAuthStub(1),
		h.DeleteRubric)

	resp := testutil.MakeRequest(app, http.MethodDelete, "/courses/10/rubrics/42", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	rubricRepo.AssertCalled(t, "Delete", mock.Anything, uint(42), uint(1))
}

// TestDeleteOutcomeGroup_RejectsCrossCourseID — F-011 / F-012 repro
// for the outcome_group surface.
func TestDeleteOutcomeGroup_RejectsCrossCourseID(t *testing.T) {
	groupRepo := new(mocks.MockLearningOutcomeGroupRepository)
	outcomeRepo := new(mocks.MockLearningOutcomeRepository)

	groupRepo.On("FindByID", mock.Anything, uint(55), uint(1)).Return(&models.LearningOutcomeGroup{
		ID:          55,
		ContextType: "Course",
		ContextID:   99, // ← NOT the URL's :course_id (10)
	}, nil)

	outcomeSvc := service.NewLearningOutcomeService(groupRepo, outcomeRepo, &noopOutcomeResultRepo{})
	h := handlers.NewLearningOutcomeHandler(outcomeSvc, new(mocks.MockOutcomeAlignmentRepository), nil)
	app := testutil.SetupTestApp()
	app.Delete("/courses/:course_id/outcome_groups/:group_id",
		rubricOutcomeAuthStub(1),
		h.DeleteGroup)

	resp := testutil.MakeRequest(app, http.MethodDelete, "/courses/10/outcome_groups/55", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	groupRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything, mock.Anything)
}

// TestDeleteOutcomeGroup_AcceptsSameCourseID — happy path. Verifies
// the caller's accountID is threaded into the Delete call.
func TestDeleteOutcomeGroup_AcceptsSameCourseID(t *testing.T) {
	groupRepo := new(mocks.MockLearningOutcomeGroupRepository)
	outcomeRepo := new(mocks.MockLearningOutcomeRepository)

	groupRepo.On("FindByID", mock.Anything, uint(55), uint(1)).Return(&models.LearningOutcomeGroup{
		ID:          55,
		ContextType: "Course",
		ContextID:   10,
	}, nil)
	groupRepo.On("Delete", mock.Anything, uint(55), uint(1)).Return(nil)

	outcomeSvc := service.NewLearningOutcomeService(groupRepo, outcomeRepo, &noopOutcomeResultRepo{})
	h := handlers.NewLearningOutcomeHandler(outcomeSvc, new(mocks.MockOutcomeAlignmentRepository), nil)
	app := testutil.SetupTestApp()
	app.Delete("/courses/:course_id/outcome_groups/:group_id",
		rubricOutcomeAuthStub(1),
		h.DeleteGroup)

	resp := testutil.MakeRequest(app, http.MethodDelete, "/courses/10/outcome_groups/55", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	groupRepo.AssertCalled(t, "Delete", mock.Anything, uint(55), uint(1))
}

// TestDeleteOutcome_RejectsCrossCourseID — F-011 / F-012 repro for the
// learning-outcome surface.
func TestDeleteOutcome_RejectsCrossCourseID(t *testing.T) {
	groupRepo := new(mocks.MockLearningOutcomeGroupRepository)
	outcomeRepo := new(mocks.MockLearningOutcomeRepository)

	outcomeRepo.On("FindByID", mock.Anything, uint(77), uint(1)).Return(&models.LearningOutcome{
		ID:          77,
		ContextType: "Course",
		ContextID:   99, // ← NOT the URL's :course_id (10)
	}, nil)

	outcomeSvc := service.NewLearningOutcomeService(groupRepo, outcomeRepo, &noopOutcomeResultRepo{})
	h := handlers.NewLearningOutcomeHandler(outcomeSvc, new(mocks.MockOutcomeAlignmentRepository), nil)
	app := testutil.SetupTestApp()
	app.Delete("/courses/:course_id/outcomes/:outcome_id",
		rubricOutcomeAuthStub(1),
		h.DeleteOutcome)

	resp := testutil.MakeRequest(app, http.MethodDelete, "/courses/10/outcomes/77", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	outcomeRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything, mock.Anything)
}

// TestDeleteOutcome_AcceptsSameCourseID — happy path. Verifies the
// caller's accountID is threaded into the Delete call.
func TestDeleteOutcome_AcceptsSameCourseID(t *testing.T) {
	groupRepo := new(mocks.MockLearningOutcomeGroupRepository)
	outcomeRepo := new(mocks.MockLearningOutcomeRepository)

	outcomeRepo.On("FindByID", mock.Anything, uint(77), uint(1)).Return(&models.LearningOutcome{
		ID:          77,
		ContextType: "Course",
		ContextID:   10,
	}, nil)
	outcomeRepo.On("Delete", mock.Anything, uint(77), uint(1)).Return(nil)

	outcomeSvc := service.NewLearningOutcomeService(groupRepo, outcomeRepo, &noopOutcomeResultRepo{})
	h := handlers.NewLearningOutcomeHandler(outcomeSvc, new(mocks.MockOutcomeAlignmentRepository), nil)
	app := testutil.SetupTestApp()
	app.Delete("/courses/:course_id/outcomes/:outcome_id",
		rubricOutcomeAuthStub(1),
		h.DeleteOutcome)

	resp := testutil.MakeRequest(app, http.MethodDelete, "/courses/10/outcomes/77", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	outcomeRepo.AssertCalled(t, "Delete", mock.Anything, uint(77), uint(1))
}
