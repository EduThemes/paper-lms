package handlers_test

// Content-lane parity regression tests (modules, pages, files,
// blueprint, Commons, course pacing, content migrations). Each test pins
// one audited defect: the cross-course / cross-tenant request is now a
// 404 (or 400) and the destructive or exfiltrating repo call never runs.

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

var errNotFound = errors.New("record not found")

// contentAuthStub stamps the Locals the auth + course middleware set.
func contentAuthStub(accountID uint, enrollmentType string, isAdmin bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(7))
		c.Locals("account_id", accountID)
		if enrollmentType != "" {
			c.Locals("enrollment_type", enrollmentType)
		}
		c.Locals("is_admin", isAdmin)
		return c.Next()
	}
}

// ---------------------------------------------------------------- Commons

func newCommonsFixture() (*handlers.CommonsHandler, *mocks.MockSharedContentRepository, *mocks.MockCourseRepository, *mocks.MockAssignmentRepository, *mocks.MockQuizRepository, *mocks.MockQuizQuestionRepository) {
	shared := new(mocks.MockSharedContentRepository)
	courses := new(mocks.MockCourseRepository)
	assignments := new(mocks.MockAssignmentRepository)
	pages := new(mocks.MockPageRepository)
	quizzes := new(mocks.MockQuizRepository)
	modules := new(mocks.MockModuleRepository)
	discussions := new(mocks.MockDiscussionTopicRepository)
	questions := new(mocks.MockQuizQuestionRepository)
	items := new(mocks.MockModuleItemRepository)
	svc := service.NewCommonsService(shared, courses, assignments, pages, quizzes, modules, discussions)
	svc.SetContentDeps(questions, items)
	return handlers.NewCommonsHandler(svc, courses), shared, courses, assignments, quizzes, questions
}

// B036: the import target is the URL's :course_id, resolved under the
// caller's tenant. A course in another tenant is a 404 and nothing is
// written.
func TestCommonsImport_TargetCourseInOtherTenant_404(t *testing.T) {
	h, shared, courses, assignments, _, _ := newCommonsFixture()
	shared.On("FindByID", mock.Anything, uint(5), uint(1)).Return(&models.SharedContent{
		ID: 5, AccountID: 1, ResourceType: "assignment", Visibility: "public",
		ContentSnapshot: `{"assignment":{"id":3,"name":"Essay 1","course_id":42}}`,
	}, nil)
	courses.On("FindByID", mock.Anything, uint(10), uint(1)).Return(nil, errNotFound)

	app := testutil.SetupTestApp()
	app.Post("/courses/:course_id/commons/:id/import", contentAuthStub(1, "TeacherEnrollment", false), h.Import)

	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/10/commons/5/import", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assignments.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// B036: the target comes from the URL, never from a query/body
// course_id (which the course middleware never saw).
func TestCommonsImport_IgnoresQueryCourseID(t *testing.T) {
	h, shared, courses, assignments, _, _ := newCommonsFixture()
	shared.On("FindByID", mock.Anything, uint(5), uint(1)).Return(&models.SharedContent{
		ID: 5, AccountID: 1, ResourceType: "assignment",
		ContentSnapshot: `{"assignment":{"id":3,"name":"Essay 1","course_id":42}}`,
	}, nil)
	courses.On("FindByID", mock.Anything, uint(10), uint(1)).Return(&models.Course{ID: 10, AccountID: 1}, nil)
	assignments.On("Create", mock.Anything, mock.MatchedBy(func(a *models.Assignment) bool { return a.CourseID == 10 })).Return(nil)
	shared.On("IncrementDownloadCount", mock.Anything, uint(5)).Return(nil)

	app := testutil.SetupTestApp()
	app.Post("/courses/:course_id/commons/:id/import", contentAuthStub(1, "TeacherEnrollment", false), h.Import)

	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/10/commons/5/import?course_id=77", nil)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	courses.AssertNotCalled(t, "FindByID", mock.Anything, uint(77), mock.Anything)
	assignments.AssertExpectations(t)
}

// B038: publishing a resource id from another course is a 404 and no
// snapshot of it is stored.
func TestCommonsPublish_ResourceFromOtherCourse_404(t *testing.T) {
	h, shared, courses, assignments, _, _ := newCommonsFixture()
	courses.On("FindByID", mock.Anything, uint(10), uint(1)).Return(&models.Course{ID: 10, AccountID: 1}, nil)
	assignments.On("FindByID", mock.Anything, uint(500), uint(1)).Return(&models.Assignment{ID: 500, CourseID: 99, Name: "Secret"}, nil)

	app := testutil.SetupTestApp()
	app.Post("/courses/:course_id/commons/publish", contentAuthStub(1, "TeacherEnrollment", false), h.Publish)

	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/10/commons/publish", testutil.JSONBody(map[string]any{
		"resource_type": "assignment", "resource_id": 500, "title": "Copy",
	}))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	shared.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// ---------------------------------------------------------------- Blueprint

// fakeSubRepo is an in-memory BlueprintSubscriptionRepository.
type fakeSubRepo struct {
	rows    []models.BlueprintSubscription
	deletes []uint
	creates []uint
}

func (f *fakeSubRepo) Create(_ context.Context, s *models.BlueprintSubscription) error {
	for i := range f.rows {
		if f.rows[i].BlueprintTemplateID == s.BlueprintTemplateID && f.rows[i].ChildCourseID == s.ChildCourseID {
			f.rows[i].WorkflowState = "active"
			s.ID = f.rows[i].ID
			f.creates = append(f.creates, s.ChildCourseID)
			return nil
		}
	}
	s.ID = uint(len(f.rows) + 1)
	f.rows = append(f.rows, *s)
	f.creates = append(f.creates, s.ChildCourseID)
	return nil
}
func (f *fakeSubRepo) FindByID(_ context.Context, id uint) (*models.BlueprintSubscription, error) {
	for i := range f.rows {
		if f.rows[i].ID == id && f.rows[i].WorkflowState != "deleted" {
			r := f.rows[i]
			return &r, nil
		}
	}
	return nil, errNotFound
}
func (f *fakeSubRepo) FindByTemplateAndChild(_ context.Context, templateID, childCourseID uint) (*models.BlueprintSubscription, error) {
	return nil, errNotFound
}
func (f *fakeSubRepo) Update(_ context.Context, _ *models.BlueprintSubscription) error { return nil }
func (f *fakeSubRepo) Delete(_ context.Context, id uint) error {
	for i := range f.rows {
		if f.rows[i].ID == id {
			f.rows[i].WorkflowState = "deleted"
		}
	}
	f.deletes = append(f.deletes, id)
	return nil
}
func (f *fakeSubRepo) list(pred func(models.BlueprintSubscription) bool, params repository.PaginationParams) *repository.PaginatedResult[models.BlueprintSubscription] {
	var all []models.BlueprintSubscription
	for _, r := range f.rows {
		if r.WorkflowState != "deleted" && pred(r) {
			all = append(all, r)
		}
	}
	start := (params.Page - 1) * params.PerPage
	end := start + params.PerPage
	if start > len(all) {
		start = len(all)
	}
	if end > len(all) {
		end = len(all)
	}
	return &repository.PaginatedResult[models.BlueprintSubscription]{Items: all[start:end], TotalCount: int64(len(all)), Page: params.Page, PerPage: params.PerPage}
}
func (f *fakeSubRepo) ListByTemplateID(_ context.Context, templateID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.BlueprintSubscription], error) {
	return f.list(func(r models.BlueprintSubscription) bool { return r.BlueprintTemplateID == templateID }, params), nil
}
func (f *fakeSubRepo) ListByChildCourseID(_ context.Context, childCourseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.BlueprintSubscription], error) {
	return f.list(func(r models.BlueprintSubscription) bool { return r.ChildCourseID == childCourseID }, params), nil
}

func newBlueprintFixture(subs *fakeSubRepo) (*handlers.BlueprintHandler, *mocks.MockCourseRepository, *mocks.MockEnrollmentRepository) {
	tmpl := new(mocks.MockBlueprintTemplateRepository)
	tmpl.On("FindByCourseID", mock.Anything, uint(10)).Return(&models.BlueprintTemplate{ID: 3, CourseID: 10, WorkflowState: "active"}, nil)
	tmpl.On("FindByCourseID", mock.Anything, mock.Anything).Return(nil, errNotFound)
	courses := new(mocks.MockCourseRepository)
	enrollments := new(mocks.MockEnrollmentRepository)
	svc := service.NewBlueprintService(tmpl, subs, nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetDeps(courses, enrollments, nil)
	return handlers.NewBlueprintHandler(svc), courses, enrollments
}

// B037: a course in another tenant cannot be associated (and so can
// never receive a sync).
func TestBlueprintAssociations_CrossTenantCourseRejected(t *testing.T) {
	subs := &fakeSubRepo{}
	h, courses, _ := newBlueprintFixture(subs)
	courses.On("FindByID", mock.Anything, uint(10), uint(1)).Return(&models.Course{ID: 10, AccountID: 1}, nil)
	courses.On("FindByID", mock.Anything, uint(900), uint(1)).Return(nil, errNotFound) // tenant 2's course

	app := testutil.SetupTestApp()
	app.Put("/courses/:course_id/blueprint_templates/default/associated_courses", contentAuthStub(1, "TeacherEnrollment", false), h.UpdateAssociations)

	resp := testutil.MakeRequest(app, http.MethodPut, "/courses/10/blueprint_templates/default/associated_courses",
		testutil.JSONBody(map[string]any{"course_ids_to_add": []uint{900}}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Empty(t, subs.creates)
}

// B037: a same-tenant course the caller does not teach is rejected too.
func TestBlueprintAssociations_CourseCallerDoesNotTeachRejected(t *testing.T) {
	subs := &fakeSubRepo{}
	h, courses, enrollments := newBlueprintFixture(subs)
	courses.On("FindByID", mock.Anything, uint(10), uint(1)).Return(&models.Course{ID: 10, AccountID: 1}, nil)
	courses.On("FindByID", mock.Anything, uint(20), uint(1)).Return(&models.Course{ID: 20, AccountID: 1}, nil)
	enrollments.On("FindByUserAndCourse", mock.Anything, uint(7), uint(20), uint(1)).Return(nil, errNotFound)

	app := testutil.SetupTestApp()
	app.Put("/courses/:course_id/blueprint_templates/default/associated_courses", contentAuthStub(1, "TeacherEnrollment", false), h.UpdateAssociations)

	resp := testutil.MakeRequest(app, http.MethodPut, "/courses/10/blueprint_templates/default/associated_courses",
		testutil.JSONBody(map[string]any{"course_ids_to_add": []uint{20}}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Empty(t, subs.creates)
}

// B040: adding a 13th course keeps all 12 existing associations (the
// handler used to rebuild the set from a 10-row page and delete the
// rest).
func TestBlueprintAssociations_AddKeepsAssociationsBeyondFirstPage(t *testing.T) {
	subs := &fakeSubRepo{}
	for i := 0; i < 12; i++ {
		subs.rows = append(subs.rows, models.BlueprintSubscription{ID: uint(i + 1), BlueprintTemplateID: 3, ChildCourseID: uint(100 + i), WorkflowState: "active"})
	}
	h, courses, _ := newBlueprintFixture(subs)
	courses.On("FindByID", mock.Anything, uint(10), uint(1)).Return(&models.Course{ID: 10, AccountID: 1}, nil)
	courses.On("FindByID", mock.Anything, uint(50), uint(1)).Return(&models.Course{ID: 50, AccountID: 1}, nil)

	app := testutil.SetupTestApp()
	app.Put("/courses/:course_id/blueprint_templates/default/associated_courses", contentAuthStub(1, "", true), h.UpdateAssociations)

	resp := testutil.MakeRequest(app, http.MethodPut, "/courses/10/blueprint_templates/default/associated_courses",
		testutil.JSONBody(map[string]any{"course_ids_to_add": []uint{50}}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Empty(t, subs.deletes, "no existing association may be removed")
	assert.Equal(t, []uint{50}, subs.creates)
}

// ---------------------------------------------------------------- Course pacing

func newPaceFixture() (*handlers.CoursePaceHandler, *mocks.MockCoursePaceRepository, *mocks.MockCoursePaceModuleItemRepository, *mocks.MockModuleItemRepository, *mocks.MockModuleRepository) {
	paces := new(mocks.MockCoursePaceRepository)
	paceItems := new(mocks.MockCoursePaceModuleItemRepository)
	items := new(mocks.MockModuleItemRepository)
	modules := new(mocks.MockModuleRepository)
	assignments := new(mocks.MockAssignmentRepository)
	svc := service.NewCoursePaceService(paces, paceItems, items, assignments)
	svc.SetDeps(service.CoursePaceDeps{ModuleRepo: modules})
	return handlers.NewCoursePaceHandler(svc), paces, paceItems, items, modules
}

// B039: a pace id from another course is a 404 on every :id route.
func TestCoursePace_PaceFromOtherCourse_404(t *testing.T) {
	h, paces, paceItems, _, _ := newPaceFixture()
	paces.On("FindByID", mock.Anything, uint(7), uint(1)).Return(&models.CoursePace{ID: 7, CourseID: 99}, nil)

	app := testutil.SetupTestApp()
	stub := contentAuthStub(1, "TeacherEnrollment", false)
	app.Get("/courses/:course_id/course_pacing/:id/module_items", stub, h.GetPaceModuleItems)
	app.Put("/courses/:course_id/course_pacing/:id/module_items", stub, h.UpdatePaceModuleItems)
	app.Post("/courses/:course_id/course_pacing/:id/publish", stub, h.PublishCoursePace)
	app.Delete("/courses/:course_id/course_pacing/:id", stub, h.DeleteCoursePace)

	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/courses/10/course_pacing/7/module_items"},
		{http.MethodPut, "/courses/10/course_pacing/7/module_items"},
		{http.MethodPost, "/courses/10/course_pacing/7/publish"},
		{http.MethodDelete, "/courses/10/course_pacing/7"},
	} {
		resp := testutil.MakeRequest(app, tc.method, tc.path, testutil.JSONBody(map[string]any{"module_items": []any{}}))
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, tc.method+" "+tc.path)
	}
	paceItems.AssertNotCalled(t, "BulkUpsert", mock.Anything, mock.Anything)
	paces.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
	paces.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

// B039: a module item of another course cannot be put on a pace (publish
// would follow it to that course's assignment).
func TestCoursePace_ForeignModuleItemRejected(t *testing.T) {
	h, paces, paceItems, items, modules := newPaceFixture()
	paces.On("FindByID", mock.Anything, uint(7), uint(1)).Return(&models.CoursePace{ID: 7, CourseID: 10}, nil)
	items.On("FindByID", mock.Anything, uint(555)).Return(&models.ContentTag{ID: 555, ContextModuleID: 40, ContentType: "Assignment", WorkflowState: "active"}, nil)
	modules.On("FindByID", mock.Anything, uint(40), uint(0)).Return(&models.ContextModule{ID: 40, CourseID: 99, WorkflowState: "active"}, nil)

	app := testutil.SetupTestApp()
	app.Put("/courses/:course_id/course_pacing/:id/module_items", contentAuthStub(1, "TeacherEnrollment", false), h.UpdatePaceModuleItems)

	resp := testutil.MakeRequest(app, http.MethodPut, "/courses/10/course_pacing/7/module_items",
		testutil.JSONBody(map[string]any{"module_items": []map[string]any{{"module_item_id": 555, "duration": 2}}}))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	paceItems.AssertNotCalled(t, "BulkUpsert", mock.Anything, mock.Anything)
}

// ---------------------------------------------------------------- Modules

func newModuleFixture() (*handlers.ModuleHandler, *handlers.ModuleItemHandler, *mocks.MockModuleRepository, *mocks.MockModuleItemRepository, *mocks.MockAssignmentRepository) {
	modules := new(mocks.MockModuleRepository)
	items := new(mocks.MockModuleItemRepository)
	prereqs := &fakePrereqRepo{}
	assignments := new(mocks.MockAssignmentRepository)
	svc := service.NewModuleService(modules, items, service.WithPrerequisiteRepo(prereqs))
	svc.SetContentRepos(assignments, new(mocks.MockQuizRepository), new(mocks.MockDiscussionTopicRepository), new(mocks.MockPageRepository))
	pageSvc := service.NewPageService(new(mocks.MockPageRepository))
	return handlers.NewModuleHandler(svc), handlers.NewModuleItemHandler(svc, pageSvc), modules, items, assignments
}

type fakePrereqRepo struct {
	set map[uint][]uint
}

func (f *fakePrereqRepo) SetPrerequisites(_ context.Context, moduleID uint, ids []uint) error {
	if f.set == nil {
		f.set = map[uint][]uint{}
	}
	f.set[moduleID] = ids
	return nil
}
func (f *fakePrereqRepo) GetPrerequisites(_ context.Context, moduleID uint) ([]uint, error) {
	return f.set[moduleID], nil
}
func (f *fakePrereqRepo) GetModulesWithPrerequisite(_ context.Context, _ uint) ([]uint, error) {
	return nil, nil
}

// B140: GET module by id is tied to :course_id.
func TestGetModule_OtherCourse_404(t *testing.T) {
	mh, _, modules, _, _ := newModuleFixture()
	modules.On("FindByID", mock.Anything, uint(40), uint(1)).Return(&models.ContextModule{ID: 40, CourseID: 99, WorkflowState: "active"}, nil)

	app := testutil.SetupTestApp()
	app.Get("/courses/:course_id/modules/:id", contentAuthStub(1, "StudentEnrollment", false), mh.GetModule)

	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/10/modules/40", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// B140 + B045: a student's GET hides deleted and unpublished items, and
// an unpublished module is a 404.
func TestGetModule_StudentSeesOnlyPublished(t *testing.T) {
	mh, _, modules, _, _ := newModuleFixture()
	modules.On("FindByID", mock.Anything, uint(40), uint(1)).Return(&models.ContextModule{ID: 40, CourseID: 10, WorkflowState: "active", Items: []models.ContentTag{
		{ID: 1, Title: "Visible", WorkflowState: "active"},
		{ID: 2, Title: "Draft", WorkflowState: "unpublished"},
		{ID: 3, Title: "Gone", WorkflowState: "deleted"},
	}}, nil)
	modules.On("FindByID", mock.Anything, uint(41), uint(1)).Return(&models.ContextModule{ID: 41, CourseID: 10, WorkflowState: "unpublished"}, nil)

	app := testutil.SetupTestApp()
	app.Get("/courses/:course_id/modules/:id", contentAuthStub(1, "StudentEnrollment", false), mh.GetModule)

	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/10/modules/40", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := testutil.ParseJSONMap(resp)
	gotItems, _ := body["items"].([]any)
	require.Len(t, gotItems, 1)
	assert.Equal(t, "Visible", gotItems[0].(map[string]any)["title"])

	resp = testutil.MakeRequest(app, http.MethodGet, "/courses/10/modules/41", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// B045: a student's module list goes through the published-only query.
func TestListModules_StudentUsesPublishedQuery(t *testing.T) {
	mh, _, modules, _, _ := newModuleFixture()
	modules.On("ListPublishedByCourseID", mock.Anything, uint(10), mock.Anything).Return(&repository.PaginatedResult[models.ContextModule]{Page: 1, PerPage: 10}, nil)

	app := testutil.SetupTestApp()
	app.Get("/courses/:course_id/modules", contentAuthStub(1, "StudentEnrollment", false), mh.ListModules)

	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/10/modules", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	modules.AssertNotCalled(t, "ListByCourseID", mock.Anything, mock.Anything, mock.Anything)
}

// B140: prerequisites are tied to the course — a module of another
// course can neither be read/written nor used as a prerequisite.
func TestModulePrerequisites_CourseTied(t *testing.T) {
	mh, _, modules, _, _ := newModuleFixture()
	modules.On("FindByID", mock.Anything, uint(40), uint(1)).Return(&models.ContextModule{ID: 40, CourseID: 10, WorkflowState: "active"}, nil)
	modules.On("FindByID", mock.Anything, uint(77), uint(1)).Return(&models.ContextModule{ID: 77, CourseID: 99, WorkflowState: "active"}, nil)

	app := testutil.SetupTestApp()
	stub := contentAuthStub(1, "TeacherEnrollment", false)
	app.Put("/courses/:course_id/modules/:id/prerequisites", stub, mh.SetPrerequisites)
	app.Get("/courses/:course_id/modules/:id/prerequisites", stub, mh.GetPrerequisites)

	// Foreign module as the target.
	resp := testutil.MakeRequest(app, http.MethodPut, "/courses/10/modules/77/prerequisites", testutil.JSONBody(map[string]any{"prerequisite_module_ids": []uint{40}}))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp = testutil.MakeRequest(app, http.MethodGet, "/courses/10/modules/77/prerequisites", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Foreign module as a prerequisite.
	resp = testutil.MakeRequest(app, http.MethodPut, "/courses/10/modules/40/prerequisites", testutil.JSONBody(map[string]any{"prerequisite_module_ids": []uint{77}}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// B042: an Assignment item needs a content_id naming an assignment of
// this course.
func TestCreateModuleItem_AssignmentNeedsCourseContent(t *testing.T) {
	_, ih, modules, items, assignments := newModuleFixture()
	modules.On("FindByID", mock.Anything, uint(40), uint(1)).Return(&models.ContextModule{ID: 40, CourseID: 10, WorkflowState: "active"}, nil)
	assignments.On("FindByID", mock.Anything, uint(500), uint(1)).Return(&models.Assignment{ID: 500, CourseID: 99, Name: "Other"}, nil)
	assignments.On("FindByID", mock.Anything, uint(501), uint(1)).Return(&models.Assignment{ID: 501, CourseID: 10, Name: "Essay 1"}, nil)
	items.On("Create", mock.Anything, mock.Anything).Return(nil)

	app := testutil.SetupTestApp()
	app.Post("/courses/:course_id/modules/:module_id/items", contentAuthStub(1, "TeacherEnrollment", false), ih.CreateModuleItem)

	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/10/modules/40/items",
		testutil.JSONBody(map[string]any{"module_item": map[string]any{"title": "Essay", "type": "Assignment"}}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "missing content_id")

	resp = testutil.MakeRequest(app, http.MethodPost, "/courses/10/modules/40/items",
		testutil.JSONBody(map[string]any{"module_item": map[string]any{"type": "Assignment", "content_id": 500}}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "foreign content_id")
	items.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)

	resp = testutil.MakeRequest(app, http.MethodPost, "/courses/10/modules/40/items",
		testutil.JSONBody(map[string]any{"module_item": map[string]any{"type": "Assignment", "content_id": 501}}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	body, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, "Essay 1", body["title"], "blank title defaults to the content's name")
	assert.EqualValues(t, 501, body["content_id"])
}

// B043: completion requirements are validated and persisted.
func TestUpdateModuleItem_PersistsCompletionRequirement(t *testing.T) {
	_, ih, modules, items, _ := newModuleFixture()
	modules.On("FindByID", mock.Anything, uint(40), uint(1)).Return(&models.ContextModule{ID: 40, CourseID: 10, WorkflowState: "active"}, nil)
	items.On("FindByID", mock.Anything, uint(9)).Return(&models.ContentTag{ID: 9, ContextModuleID: 40, ContentType: "Assignment", Title: "Essay", WorkflowState: "active"}, nil)
	var saved *models.ContentTag
	items.On("Update", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		saved = args.Get(1).(*models.ContentTag)
	}).Return(nil)

	app := testutil.SetupTestApp()
	app.Put("/courses/:course_id/modules/:module_id/items/:item_id", contentAuthStub(1, "TeacherEnrollment", false), ih.UpdateModuleItem)

	resp := testutil.MakeRequest(app, http.MethodPut, "/courses/10/modules/40/items/9",
		testutil.JSONBody(map[string]any{"module_item": map[string]any{"completion_type": "min_score", "min_score": 80}}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotNil(t, saved)
	assert.Equal(t, "min_score", saved.CompletionType)
	require.NotNil(t, saved.MinScore)
	assert.Equal(t, 80.0, *saved.MinScore)
	body, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, "min_score", body["completion_type"])

	// must_contribute is not a valid requirement for an assignment.
	resp = testutil.MakeRequest(app, http.MethodPut, "/courses/10/modules/40/items/9",
		testutil.JSONBody(map[string]any{"module_item": map[string]any{"completion_type": "must_contribute"}}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ---------------------------------------------------------------- Pages

func newPageFixture() (*handlers.PageHandler, *mocks.MockPageRepository) {
	pages := new(mocks.MockPageRepository)
	return handlers.NewPageHandler(service.NewPageService(pages)), pages
}

// B137: numeric page id from another course (same tenant) is a 404.
func TestGetPage_NumericIDOtherCourse_404(t *testing.T) {
	h, pages := newPageFixture()
	pages.On("FindByID", mock.Anything, uint(300), uint(1)).Return(&models.WikiPage{ID: 300, CourseID: 99, WorkflowState: "active", Title: "B"}, nil)
	pages.On("FindByID", mock.Anything, uint(301), uint(1)).Return(&models.WikiPage{ID: 301, CourseID: 10, WorkflowState: "deleted", Title: "Gone"}, nil)

	app := testutil.SetupTestApp()
	app.Get("/courses/:course_id/pages/:url_or_id", contentAuthStub(1, "StudentEnrollment", false), h.GetPage)

	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(app, http.MethodGet, "/courses/10/pages/300", nil).StatusCode)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(app, http.MethodGet, "/courses/10/pages/301", nil).StatusCode)
}

// B045: students cannot read or list draft pages; teachers can.
func TestPages_DraftsHiddenFromStudents(t *testing.T) {
	h, pages := newPageFixture()
	pages.On("FindByCourseAndURL", mock.Anything, uint(10), "unit-3-answers").Return(&models.WikiPage{ID: 5, CourseID: 10, URL: "unit-3-answers", WorkflowState: "unpublished"}, nil)
	pages.On("ListPublishedByCourseID", mock.Anything, uint(10), mock.Anything).Return(&repository.PaginatedResult[models.WikiPage]{Page: 1, PerPage: 10}, nil)

	student := testutil.SetupTestApp()
	student.Get("/courses/:course_id/pages", contentAuthStub(1, "StudentEnrollment", false), h.ListPages)
	student.Get("/courses/:course_id/pages/:url_or_id", contentAuthStub(1, "StudentEnrollment", false), h.GetPage)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(student, http.MethodGet, "/courses/10/pages/unit-3-answers", nil).StatusCode)
	assert.Equal(t, http.StatusOK, testutil.MakeRequest(student, http.MethodGet, "/courses/10/pages", nil).StatusCode)
	pages.AssertNotCalled(t, "ListByCourseID", mock.Anything, mock.Anything, mock.Anything)

	teacher := testutil.SetupTestApp()
	teacher.Get("/courses/:course_id/pages/:url_or_id", contentAuthStub(1, "TeacherEnrollment", false), h.GetPage)
	assert.Equal(t, http.StatusOK, testutil.MakeRequest(teacher, http.MethodGet, "/courses/10/pages/unit-3-answers", nil).StatusCode)
}

// ---------------------------------------------------------------- Files

func newFileFixture(t *testing.T) (*handlers.FileHandler, *mocks.MockFolderRepository, *mocks.MockAttachmentRepository, *mocks.MockEnrollmentRepository) {
	folders := new(mocks.MockFolderRepository)
	attachments := new(mocks.MockAttachmentRepository)
	enrollments := new(mocks.MockEnrollmentRepository)
	svc := service.NewFileService(folders, attachments, t.TempDir())
	return handlers.NewFileHandler(svc, enrollments, nil, nil), folders, attachments, enrollments
}

// B046: folder file listing is tenant-scoped and requires course
// membership.
func TestListFolderFiles_RequiresTenantAndEnrollment(t *testing.T) {
	h, folders, attachments, enrollments := newFileFixture(t)
	folders.On("FindByID", mock.Anything, uint(60), uint(1)).Return(nil, errNotFound) // other tenant
	folders.On("FindByID", mock.Anything, uint(61), uint(1)).Return(&models.Folder{ID: 61, ContextType: "Course", ContextID: 99, WorkflowState: "visible"}, nil)
	enrollments.On("FindByUserAndCourse", mock.Anything, uint(7), uint(99), uint(1)).Return(nil, errNotFound)

	app := testutil.SetupTestApp()
	app.Get("/folders/:folder_id/files", contentAuthStub(1, "", false), h.ListFolderFiles)

	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(app, http.MethodGet, "/folders/60/files", nil).StatusCode)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(app, http.MethodGet, "/folders/61/files", nil).StatusCode)
	attachments.AssertNotCalled(t, "ListByFolderID", mock.Anything, mock.Anything, mock.Anything)
}

func uploadRequest(t *testing.T, path string, fields map[string]string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		require.NoError(t, w.WriteField(k, v))
	}
	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Disposition", `form-data; name="file"; filename="handout.txt"`)
	hdr.Set("Content-Type", "text/plain")
	fw, err := w.CreatePart(hdr)
	require.NoError(t, err)
	_, _ = fw.Write([]byte("hello class"))
	require.NoError(t, w.Close())
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

// B138: an upload lands in the requested folder of this course; a folder
// of another course is a 404.
func TestUploadCourseFile_HonorsFolderOfThisCourse(t *testing.T) {
	h, folders, attachments, _ := newFileFixture(t)
	folders.On("FindByID", mock.Anything, uint(61), uint(1)).Return(&models.Folder{ID: 61, ContextType: "Course", ContextID: 10, WorkflowState: "visible"}, nil)
	folders.On("FindByID", mock.Anything, uint(62), uint(1)).Return(&models.Folder{ID: 62, ContextType: "Course", ContextID: 99, WorkflowState: "visible"}, nil)
	var created *models.Attachment
	attachments.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		created = args.Get(1).(*models.Attachment)
	}).Return(nil)
	attachments.On("Update", mock.Anything, mock.Anything).Return(nil)

	app := testutil.SetupTestApp()
	app.Post("/courses/:course_id/files", contentAuthStub(1, "TeacherEnrollment", false), h.UploadCourseFile)

	resp, err := app.Test(uploadRequest(t, "/courses/10/files", map[string]string{"folder_id": "62"}), -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Nil(t, created)

	resp, err = app.Test(uploadRequest(t, "/courses/10/files", map[string]string{"folder_id": "61"}), -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.NotNil(t, created)
	require.NotNil(t, created.FolderID)
	assert.Equal(t, uint(61), *created.FolderID)
}

// ---------------------------------------------------------------- Content migrations

type fakeMigrationRepo struct {
	row     *models.ContentMigration
	updated bool
}

func (f *fakeMigrationRepo) Create(_ context.Context, _ *models.ContentMigration) error { return nil }
func (f *fakeMigrationRepo) FindByID(_ context.Context, id uint) (*models.ContentMigration, error) {
	if f.row != nil && f.row.ID == id {
		r := *f.row
		return &r, nil
	}
	return nil, errNotFound
}
func (f *fakeMigrationRepo) Update(_ context.Context, _ *models.ContentMigration) error {
	f.updated = true
	return nil
}
func (f *fakeMigrationRepo) Delete(_ context.Context, _ uint) error { return nil }
func (f *fakeMigrationRepo) ListByCourseID(_ context.Context, _ uint, _ repository.PaginationParams) (*repository.PaginatedResult[models.ContentMigration], error) {
	return &repository.PaginatedResult[models.ContentMigration]{}, nil
}

// B142: GET/PUT of another course's migration is a 404 and PUT writes
// nothing.
func TestContentMigration_OtherCourse_404(t *testing.T) {
	repo := &fakeMigrationRepo{row: &models.ContentMigration{ID: 8, CourseID: 99, MigrationType: "common_cartridge", WorkflowState: "running", CreatedAt: time.Now()}}
	h := handlers.NewContentMigrationHandler(service.NewContentMigrationService(repo))

	app := testutil.SetupTestApp()
	stub := contentAuthStub(1, "TeacherEnrollment", false)
	app.Get("/courses/:course_id/content_migrations/:id", stub, h.GetMigration)
	app.Put("/courses/:course_id/content_migrations/:id", stub, h.UpdateMigration)

	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(app, http.MethodGet, "/courses/10/content_migrations/8", nil).StatusCode)
	resp := testutil.MakeRequest(app, http.MethodPut, "/courses/10/content_migrations/8",
		testutil.JSONBody(map[string]any{"content_migration": map[string]any{"workflow_state": "completed"}}))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.False(t, repo.updated)

	// Same course: readable, and only known states may be written.
	assert.Equal(t, http.StatusOK, testutil.MakeRequest(app, http.MethodGet, "/courses/99/content_migrations/8", nil).StatusCode)
	resp = testutil.MakeRequest(app, http.MethodPut, "/courses/99/content_migrations/8",
		testutil.JSONBody(map[string]any{"content_migration": map[string]any{"workflow_state": "pwned"}}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ---------------------------------------------------------------- Syllabus

// B136: students never see deleted or unpublished assignments on the
// syllabus; course managers see drafts.
func TestVisibleSyllabusAssignments(t *testing.T) {
	items := []models.Assignment{
		{ID: 1, Name: "Live", WorkflowState: models.AssignmentPublished, Published: true},
		{ID: 2, Name: "Draft", WorkflowState: models.AssignmentUnpublished},
		{ID: 3, Name: "Gone", WorkflowState: models.AssignmentDeleted},
	}
	student := handlers.VisibleSyllabusAssignments(items, false)
	require.Len(t, student, 1)
	assert.Equal(t, uint(1), student[0].ID)

	teacher := handlers.VisibleSyllabusAssignments(items, true)
	assert.Len(t, teacher, 2)
}
