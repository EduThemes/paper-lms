package handlers_test

// Content-migration lane handler tests (G066/G067/G069/G142), against a
// migrated scratch Postgres (skipped without one).

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/storage"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

// seedLaneTeacher creates user 7 (the contentAuthStub caller).
func seedLaneTeacher(t *testing.T, g *gorm.DB) {
	t.Helper()
	require.NoError(t, g.Create(&models.User{ID: 7, AccountID: 1, Name: "Terry Teacher", LoginID: "terry.lane@example.test", Email: "terry.lane@example.test"}).Error)
}

func enrollLane(t *testing.T, g *gorm.DB, userID, courseID uint, typ string) {
	t.Helper()
	require.NoError(t, postgres.NewEnrollmentRepository(g).Create(context.Background(), &models.Enrollment{
		UserID: userID, CourseID: courseID, Type: typ, Role: typ, WorkflowState: "active", PseudonymPoolCode: "animals_v1",
	}))
}

func laneFiles(t *testing.T, g *gorm.DB) (*service.FileService, storage.Backend) {
	backend := storage.NewLocalBackend(t.TempDir())
	return service.NewFileServiceWithBackend(postgres.NewFolderRepository(g), postgres.NewAttachmentRepository(g), backend), backend
}

func TestContentExportAPI_CreatePollDownload(t *testing.T) {
	g := contentLaneDB(t)
	seedContentAccounts(t, g, 1, 2)
	course := seedContentCourse(t, g, 1, "ENG-1")
	other := seedContentCourse(t, g, 1, "ENG-2")
	seedContentAssignment(t, g, course.ID, "Essay")
	files, backend := laneFiles(t, g)

	exporter := service.NewIMSCCExporter(
		postgres.NewCourseRepository(g), postgres.NewModuleRepository(g), postgres.NewModuleItemRepository(g),
		postgres.NewPageRepository(g), postgres.NewAssignmentRepository(g), postgres.NewQuizRepository(g),
		postgres.NewQuizQuestionRepository(g), postgres.NewDiscussionTopicRepository(g), postgres.NewAssignmentGroupRepository(g),
		postgres.NewRubricRepository(g), postgres.NewRubricAssociationRepository(g),
		postgres.NewLearningOutcomeGroupRepository(g), postgres.NewLearningOutcomeRepository(g),
		postgres.NewCalendarEventRepository(g), postgres.NewAttachmentRepository(g), files,
	)
	exports := service.NewContentExportService(postgres.NewContentExportRepository(g), exporter, backend, t.TempDir())
	exports.SetDispatcher(func(fn func()) { fn() })
	h := handlers.NewContentExportHandler(exports)

	mount := func(account uint) *fiber.App {
		app := testutil.SetupTestApp()
		stub := contentAuthStub(account, "TeacherEnrollment", false)
		app.Get("/courses/:course_id/content_exports", stub, h.ListExports)
		app.Post("/courses/:course_id/content_exports", stub, h.ExportCourse)
		app.Get("/courses/:course_id/content_exports/:id", stub, h.GetExport)
		app.Get("/courses/:course_id/content_exports/:id/download", stub, h.DownloadExport)
		return app
	}
	app := mount(1)

	resp := testutil.MakeRequest(app, http.MethodPost, fmt.Sprintf("/courses/%d/content_exports", course.ID), testutil.JSONBody(map[string]any{"export_type": "common_cartridge"}))
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	created, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	id := uint(created["id"].(float64))

	resp = testutil.MakeRequest(app, http.MethodGet, fmt.Sprintf("/courses/%d/content_exports/%d", course.ID, id), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	got, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	assert.Equal(t, "exported", got["workflow_state"])
	att, ok := got["attachment"].(map[string]any)
	require.True(t, ok, "an exported export carries its download")
	assert.Equal(t, fmt.Sprintf("/api/v1/courses/%d/content_exports/%d/download", course.ID, id), att["url"])

	resp = testutil.MakeRequest(app, http.MethodGet, fmt.Sprintf("/courses/%d/content_exports/%d/download", course.ID, id), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/zip", resp.Header.Get("Content-Type"))
	assert.Contains(t, resp.Header.Get("Content-Disposition"), ".imscc")
	body, _ := io.ReadAll(resp.Body)
	require.Greater(t, len(body), 4)
	assert.Equal(t, "PK", string(body[:2]))

	resp = testutil.MakeRequest(app, http.MethodGet, fmt.Sprintf("/courses/%d/content_exports", course.ID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	list, err := testutil.ParseJSONArray(resp)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	// Parent tie and tenant: another course's URL or another tenant is a
	// 404, never the package.
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(app, http.MethodGet, fmt.Sprintf("/courses/%d/content_exports/%d", other.ID, id), nil).StatusCode)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(app, http.MethodGet, fmt.Sprintf("/courses/%d/content_exports/%d/download", other.ID, id), nil).StatusCode)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(mount(2), http.MethodGet, fmt.Sprintf("/courses/%d/content_exports/%d/download", course.ID, id), nil).StatusCode)

	resp = testutil.MakeRequest(app, http.MethodPost, fmt.Sprintf("/courses/%d/content_exports", course.ID), testutil.JSONBody(map[string]any{"export_type": "zip"}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCourseCopyAPI_AuthorizesSourceQueuesAndRuns(t *testing.T) {
	g := contentLaneDB(t)
	seedContentAccounts(t, g, 1, 2)
	seedLaneTeacher(t, g)
	target := seedContentCourse(t, g, 1, "PHYS-F26")
	source := seedContentCourse(t, g, 1, "PHYS-F25")
	stranger := seedContentCourse(t, g, 1, "PHYS-OTHER")
	foreign := seedContentCourse(t, g, 2, "PHYS-X")
	enrollLane(t, g, 7, target.ID, "TeacherEnrollment")
	enrollLane(t, g, 7, source.ID, "TeacherEnrollment")
	enrollLane(t, g, 7, foreign.ID, "TeacherEnrollment")
	seedContentAssignment(t, g, source.ID, "Kinematics lab")
	seedContentAssignment(t, g, stranger.ID, "Not yours")
	files, _ := laneFiles(t, g)

	courses := postgres.NewCourseRepository(g)
	copier := service.NewCourseCopyService(service.CourseCopyDeps{
		Courses: courses, AssignmentGroups: postgres.NewAssignmentGroupRepository(g), Assignments: postgres.NewAssignmentRepository(g),
		Quizzes: postgres.NewQuizRepository(g), Questions: postgres.NewQuizQuestionRepository(g), QuestionGroups: postgres.NewQuizQuestionGroupRepository(g),
		Pages: postgres.NewPageRepository(g), Discussions: postgres.NewDiscussionTopicRepository(g), Modules: postgres.NewModuleRepository(g),
		ModuleItems: postgres.NewModuleItemRepository(g), Events: postgres.NewCalendarEventRepository(g), Rubrics: postgres.NewRubricRepository(g),
		RubricAssocs: postgres.NewRubricAssociationRepository(g), Folders: postgres.NewFolderRepository(g), Attachments: postgres.NewAttachmentRepository(g), Files: files,
	})
	migRepo := postgres.NewContentMigrationRepository(g)
	runner := service.NewContentMigrationRunner(migRepo, postgres.NewContentMigrationQueue(g), courses, copier, nil, service.ContentDateRepos{})
	var queued []func()
	runner.SetDispatcher(func(fn func()) { queued = append(queued, fn) })
	h := handlers.NewContentMigrationHandler(service.NewContentMigrationService(migRepo))
	h.SetCourseCopy(handlers.ContentMigrationDeps{Runner: runner, Copier: copier, Courses: courses, Enrollments: postgres.NewEnrollmentRepository(g), StoragePath: t.TempDir()})

	mount := func(isAdmin bool) *fiber.App {
		app := testutil.SetupTestApp()
		stub := contentAuthStub(1, "TeacherEnrollment", isAdmin)
		app.Get("/courses/:course_id/content_migrations/selectable", stub, h.SelectableContent)
		app.Get("/courses/:course_id/content_migrations/copy_sources", stub, h.CopySources)
		app.Post("/courses/:course_id/content_migrations", stub, h.CreateMigration)
		app.Get("/courses/:course_id/content_migrations/:id", stub, h.GetMigration)
		return app
	}
	app := mount(false)
	post := func(app *fiber.App, src uint) *http.Response {
		return testutil.MakeRequest(app, http.MethodPost, fmt.Sprintf("/courses/%d/content_migrations", target.ID), testutil.JSONBody(map[string]any{
			"migration_type":     "course_copy",
			"settings":           map[string]any{"source_course_id": src},
			"date_shift_options": map[string]any{"shift_dates": true, "old_start_date": "2025-09-01", "new_start_date": "2026-08-31"},
		}))
	}

	// Not a teacher of it / other tenant / itself: refused without a row.
	assert.Equal(t, http.StatusNotFound, post(app, stranger.ID).StatusCode)
	assert.Equal(t, http.StatusNotFound, post(app, foreign.ID).StatusCode)
	assert.Equal(t, http.StatusBadRequest, post(app, target.ID).StatusCode)
	assert.Empty(t, queued)

	resp := post(app, source.ID)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	created, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	assert.Equal(t, "queued", created["workflow_state"])
	require.Len(t, queued, 1)
	queued[0]()

	resp = testutil.MakeRequest(app, http.MethodGet, fmt.Sprintf("/courses/%d/content_migrations/%v", target.ID, created["id"]), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	got, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	assert.Equal(t, "completed", got["workflow_state"])
	results := got["results"].(map[string]any)
	assert.Equal(t, float64(1), results["counts"].(map[string]any)["assignments"])
	as, err := postgres.NewAssignmentRepository(g).ListByCourseID(context.Background(), target.ID, repository.PaginationParams{Page: 1, PerPage: 10})
	require.NoError(t, err)
	require.Len(t, as.Items, 1)
	assert.Equal(t, "Kinematics lab", as.Items[0].Name)

	// Selective data: the source's content, only for an allowed source.
	resp = testutil.MakeRequest(app, http.MethodGet, fmt.Sprintf("/courses/%d/content_migrations/selectable?source_course_id=%d", target.ID, source.ID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	types, err := testutil.ParseJSONArray(resp)
	require.NoError(t, err)
	foundAssignments := false
	for _, ty := range types {
		if ty["type"] == "assignments" {
			foundAssignments = true
			assert.Equal(t, float64(1), ty["count"])
		}
	}
	assert.True(t, foundAssignments)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(app, http.MethodGet, fmt.Sprintf("/courses/%d/content_migrations/selectable?source_course_id=%d", target.ID, stranger.ID), nil).StatusCode)

	// Copy sources: courses the caller teaches in this tenant, minus the
	// target itself.
	resp = testutil.MakeRequest(app, http.MethodGet, fmt.Sprintf("/courses/%d/content_migrations/copy_sources", target.ID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	sources, err := testutil.ParseJSONArray(resp)
	require.NoError(t, err)
	require.Len(t, sources, 1)
	assert.Equal(t, float64(source.ID), sources[0]["id"])

	// An admin of the tenant may copy from any of its courses.
	admin := mount(true)
	assert.Equal(t, http.StatusCreated, post(admin, stranger.ID).StatusCode)
	assert.Equal(t, http.StatusNotFound, post(admin, foreign.ID).StatusCode)
	resp = testutil.MakeRequest(admin, http.MethodGet, fmt.Sprintf("/courses/%d/content_migrations/copy_sources?search=other", target.ID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	sources, err = testutil.ParseJSONArray(resp)
	require.NoError(t, err)
	require.Len(t, sources, 1)
	assert.Equal(t, float64(stranger.ID), sources[0]["id"])
}

func TestBlueprintLockGuard_RefusesLockedEditsInChildCourses(t *testing.T) {
	g := contentLaneDB(t)
	seedContentAccounts(t, g, 1)
	seedLaneTeacher(t, g)
	ctx := context.Background()
	bp := seedContentCourse(t, g, 1, "BP-GUARD")
	child := seedContentCourse(t, g, 1, "SEC-GUARD")
	enrollLane(t, g, 7, child.ID, "TeacherEnrollment")
	essay := seedContentAssignment(t, g, bp.ID, "Essay")
	quiz := &models.Quiz{CourseID: bp.ID, Title: "Quiz 1", WorkflowState: "available"}
	require.NoError(t, postgres.NewQuizRepository(g).Create(ctx, quiz))
	page := &models.WikiPage{CourseID: bp.ID, Title: "Welcome", Body: "<p>Hi</p>", WorkflowState: "active"}
	require.NoError(t, postgres.NewPageRepository(g).Create(ctx, page))

	svc := service.NewBlueprintService(
		postgres.NewBlueprintTemplateRepository(g), postgres.NewBlueprintSubscriptionRepository(g), postgres.NewBlueprintMigrationRepository(g),
		postgres.NewModuleRepository(g), postgres.NewModuleItemRepository(g), postgres.NewAssignmentRepository(g), postgres.NewPageRepository(g),
		postgres.NewQuizRepository(g), postgres.NewQuizQuestionRepository(g), postgres.NewDiscussionTopicRepository(g),
	)
	svc.SetDeps(postgres.NewCourseRepository(g), postgres.NewEnrollmentRepository(g), postgres.NewBlueprintContentTagRepository(g))
	svc.SetLockRepo(postgres.NewBlueprintContentLockRepository(g))
	tmpl, err := svc.GetOrCreateTemplate(ctx, bp.ID)
	require.NoError(t, err)
	require.NoError(t, svc.ModifyAssociations(ctx, tmpl, service.BlueprintCaller{UserID: 1, AccountID: 1, IsAdmin: true}, []uint{child.ID}, nil))

	// Lock through the API handler (Canvas restrict_item).
	bh := handlers.NewBlueprintHandler(svc)
	api := testutil.SetupTestApp()
	api.Put("/courses/:course_id/blueprint_templates/default/restrict_item", contentAuthStub(1, "TeacherEnrollment", false), bh.RestrictItem)
	api.Get("/courses/:course_id/blueprint_templates/default/content", contentAuthStub(1, "TeacherEnrollment", false), bh.ListLockableContent)
	api.Get("/courses/:course_id/blueprint_subscriptions/locked_content", contentAuthStub(1, "TeacherEnrollment", false), bh.ListChildLockedContent)
	restrict := func(body map[string]any) int {
		return testutil.MakeRequest(api, http.MethodPut, fmt.Sprintf("/courses/%d/blueprint_templates/default/restrict_item", bp.ID), testutil.JSONBody(body)).StatusCode
	}
	assert.Equal(t, http.StatusOK, restrict(map[string]any{"content_type": "assignment", "content_id": essay.ID, "restricted": true}))
	assert.Equal(t, http.StatusOK, restrict(map[string]any{"content_type": "quiz", "content_id": quiz.ID, "restrictions": map[string]bool{"content": true}}))
	assert.Equal(t, http.StatusNotFound, restrict(map[string]any{"content_type": "assignment", "content_id": essay.ID + 999, "restricted": true}))
	assert.Equal(t, http.StatusBadRequest, restrict(map[string]any{"content_type": "module", "content_id": 1, "restricted": true}))

	resp := testutil.MakeRequest(api, http.MethodGet, fmt.Sprintf("/courses/%d/blueprint_templates/default/content", bp.ID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	listing, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	assert.Len(t, listing["items"], 3)

	_, err = svc.TriggerSync(ctx, tmpl.ID, 1, "initial")
	require.NoError(t, err)
	childA, err := postgres.NewAssignmentRepository(g).ListByCourseID(ctx, child.ID, repository.PaginationParams{Page: 1, PerPage: 10})
	require.NoError(t, err)
	require.Len(t, childA.Items, 1)
	copyA := childA.Items[0]
	childQ, err := postgres.NewQuizRepository(g).ListByCourseID(ctx, child.ID, repository.PaginationParams{Page: 1, PerPage: 10})
	require.NoError(t, err)
	require.Len(t, childQ.Items, 1)
	childP, err := postgres.NewPageRepository(g).ListByCourseID(ctx, child.ID, repository.PaginationParams{Page: 1, PerPage: 10})
	require.NoError(t, err)
	require.Len(t, childP.Items, 1)

	resp = testutil.MakeRequest(api, http.MethodGet, fmt.Sprintf("/courses/%d/blueprint_subscriptions/locked_content", child.ID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	lockedList, err := testutil.ParseJSONArray(resp)
	require.NoError(t, err)
	assert.Len(t, lockedList, 2)

	guard := handlers.NewBlueprintLockGuard(handlers.BlueprintLockGuardDeps{
		Blueprints: svc, Pages: postgres.NewPageRepository(g), Courses: postgres.NewCourseRepository(g), Enrollments: postgres.NewEnrollmentRepository(g),
	})
	ok := func(c *fiber.Ctx) error { return c.SendString("handler ran") }
	mount := func(stub fiber.Handler) *fiber.App {
		app := testutil.SetupTestApp()
		app.Use(stub)
		app.Use(guard)
		app.Put("/api/v1/courses/:course_id/assignments/:id", ok)
		app.Delete("/api/v1/courses/:course_id/assignments/:id", ok)
		app.Put("/api/v1/courses/:course_id/pages/:url_or_id", ok)
		app.Post("/api/v1/courses/:course_id/quizzes/:quiz_id/questions", ok)
		app.Post("/api/v1/courses/:course_id/assignments/bulk_update_dates", ok)
		app.Post("/api/v1/courses/:course_id/date_shift", ok)
		app.Get("/api/v1/courses/:course_id/assignments/:id", ok)
		app.Post("/api/v1/courses/:course_id/pages/:url_or_id/revisions/:revision_id", ok)
		app.Put("/api/v1/courses/:course_id/assignments/:assignment_id/external_tool", ok)
		app.Delete("/api/v1/courses/:course_id/assignments/:assignment_id/external_tool", ok)
		return app
	}
	app := mount(contentAuthStub(1, "TeacherEnrollment", false))
	aPath := fmt.Sprintf("/api/v1/courses/%d/assignments/%d", child.ID, copyA.ID)

	resp = testutil.MakeRequest(app, http.MethodPut, aPath, testutil.JSONBody(map[string]any{"assignment": map[string]any{"name": "Renamed"}}))
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	m, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	assert.Equal(t, true, m["blueprint_locked"])
	assert.Equal(t, []any{"content"}, m["locked_attributes"])

	// Unchanged locked fields and unlocked fields pass.
	resp = testutil.MakeRequest(app, http.MethodPut, aPath, testutil.JSONBody(map[string]any{"assignment": map[string]any{"name": "Essay", "due_at": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)}}))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, http.StatusForbidden, testutil.MakeRequest(app, http.MethodDelete, aPath, nil).StatusCode)
	assert.Equal(t, http.StatusOK, testutil.MakeRequest(app, http.MethodGet, aPath, nil).StatusCode)

	// The router matches paths case-insensitively; so does the guard.
	upper := fmt.Sprintf("/API/V1/Courses/%d/Assignments/%d", child.ID, copyA.ID)
	resp = testutil.MakeRequest(app, http.MethodPut, upper, testutil.JSONBody(map[string]any{"assignment": map[string]any{"name": "Renamed"}}))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "a mixed-case path must not slip past the lock")
	assert.Equal(t, http.StatusForbidden, testutil.MakeRequest(app, http.MethodDelete, upper, nil).StatusCode)

	// encoding/json binds keys case-insensitively; so does the guard.
	for _, body := range []map[string]any{
		{"Assignment": map[string]any{"NAME": "Renamed"}},
		{"assignment": map[string]any{"Name": "Renamed"}},
		{"assignment": map[string]any{"name": "Essay"}, "ASSIGNMENT": map[string]any{"name": "Renamed"}},
	} {
		resp = testutil.MakeRequest(app, http.MethodPut, aPath, testutil.JSONBody(body))
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "body %v", body)
	}
	resp = testutil.MakeRequest(app, http.MethodPut, aPath, strings.NewReader(`{"assignment":{"name":"Essay","NAME":"Renamed"}}`))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "a differing duplicate key must not hide behind an unchanged one")

	// The write handlers bind form bodies too: they are checked the same way.
	form := func(contentType, body string) int {
		req := httptest.NewRequest(http.MethodPut, aPath, strings.NewReader(body))
		req.Header.Set("Content-Type", contentType)
		r, err := app.Test(req, -1)
		require.NoError(t, err)
		return r.StatusCode
	}
	assert.Equal(t, http.StatusForbidden, form("application/x-www-form-urlencoded", "assignment%5Bname%5D=Renamed"))
	assert.Equal(t, http.StatusForbidden, form("application/x-www-form-urlencoded", "Assignment.Name=Renamed"))
	assert.Equal(t, http.StatusOK, form("application/x-www-form-urlencoded", "assignment%5Bname%5D=Essay"))
	assert.Equal(t, http.StatusForbidden, form("application/x-www-form-urlencoded", "assignment%5Bname%5D=Essay&assignment%5Bname%5D=Renamed"))
	// BodyParser binds XML too; the guard cannot read it, so it is refused.
	assert.Equal(t, http.StatusForbidden, form("application/xml", "<x><Assignment><Name>Renamed</Name></Assignment></x>"))
	var mp bytes.Buffer
	mw := multipart.NewWriter(&mp)
	require.NoError(t, mw.WriteField("assignment[name]", "Renamed"))
	require.NoError(t, mw.Close())
	assert.Equal(t, http.StatusForbidden, form(mw.FormDataContentType(), mp.String()))

	// A full-form save re-sends submission_types as a list while the row
	// stores a comma-separated string: unchanged, so it passes.
	resp = testutil.MakeRequest(app, http.MethodPut, aPath, testutil.JSONBody(map[string]any{"assignment": map[string]any{
		"name": "Essay", "submission_types": strings.Split(copyA.SubmissionTypes, ","),
	}}))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp = testutil.MakeRequest(app, http.MethodPut, aPath, testutil.JSONBody(map[string]any{"assignment": map[string]any{
		"submission_types": []string{"external_tool"},
	}}))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	// The unlocked page (addressed by slug) is editable; a locked quiz's
	// questions are not.
	resp = testutil.MakeRequest(app, http.MethodPut, fmt.Sprintf("/api/v1/courses/%d/pages/%s", child.ID, childP.Items[0].URL), testutil.JSONBody(map[string]any{"wiki_page": map[string]any{"body": "<p>Local</p>"}}))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp = testutil.MakeRequest(app, http.MethodPost, fmt.Sprintf("/api/v1/courses/%d/quizzes/%d/questions", child.ID, childQ.Items[0].ID), testutil.JSONBody(map[string]any{"question": map[string]any{"question_text": "New?"}}))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	// Attaching or detaching the locked assignment's external tool
	// rewrites its content.
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		resp = testutil.MakeRequest(app, method, aPath+"/external_tool", testutil.JSONBody(map[string]any{"tool_id": 1}))
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "%s external_tool", method)
	}
	// Reverting the unlocked page passes; once its content is locked, a
	// revert (which replaces the body) is refused.
	revert := fmt.Sprintf("/api/v1/courses/%d/pages/%s/revisions/1", child.ID, childP.Items[0].URL)
	assert.Equal(t, http.StatusOK, testutil.MakeRequest(app, http.MethodPost, revert, nil).StatusCode)
	require.NoError(t, svc.SetItemRestrictions(ctx, tmpl, 1, "WikiPage", page.ID, &service.BlueprintRestrictions{Content: true}))
	assert.Equal(t, http.StatusForbidden, testutil.MakeRequest(app, http.MethodPost, revert, nil).StatusCode, "locked page revert")
	assert.Equal(t, http.StatusForbidden, testutil.MakeRequest(app, http.MethodPost, fmt.Sprintf("/api/v1/courses/%d/pages/%d/revisions/1", child.ID, childP.Items[0].ID), nil).StatusCode, "by page id")
	require.NoError(t, svc.SetItemRestrictions(ctx, tmpl, 1, "WikiPage", page.ID, &service.BlueprintRestrictions{}))

	// No date lock yet: a whole-course date shift passes.
	resp = testutil.MakeRequest(app, http.MethodPost, fmt.Sprintf("/api/v1/courses/%d/date_shift", child.ID), testutil.JSONBody(map[string]any{"day_shift": 7}))
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Bulk date edits honor date locks.
	require.NoError(t, svc.SetItemRestrictions(ctx, tmpl, 1, "Assignment", essay.ID, &service.BlueprintRestrictions{DueDates: true}))
	bulk := fmt.Sprintf("/api/v1/courses/%d/assignments/bulk_update_dates", child.ID)
	resp = testutil.MakeRequest(app, http.MethodPost, bulk, testutil.JSONBody(map[string]any{"updates": []map[string]any{{"assignment_id": copyA.ID, "due_at": "2026-12-01T23:59:00Z"}}}))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp = testutil.MakeRequest(app, http.MethodPost, bulk, testutil.JSONBody(map[string]any{"UPDATES": []map[string]any{{"Assignment_ID": copyA.ID, "DUE_AT": "2026-12-01T23:59:00Z"}}}))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "mixed-case keys bind in the handler")
	bulkReq := httptest.NewRequest(http.MethodPost, bulk, strings.NewReader("<x/>"))
	bulkReq.Header.Set("Content-Type", "application/xml")
	bulkResp, err := app.Test(bulkReq, -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, bulkResp.StatusCode, "a body the guard cannot read is refused while a date lock exists")
	// Shifting the whole course would move the locked dates as well.
	resp = testutil.MakeRequest(app, http.MethodPost, fmt.Sprintf("/api/v1/courses/%d/date_shift", child.ID), testutil.JSONBody(map[string]any{"day_shift": 7}))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp = testutil.MakeRequest(app, http.MethodPost, bulk, testutil.JSONBody(map[string]any{"updates": []map[string]any{{"assignment_id": copyA.ID, "due_at": nil}}}))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	// Content is no longer locked for it: a rename passes.
	resp = testutil.MakeRequest(app, http.MethodPut, aPath, testutil.JSONBody(map[string]any{"assignment": map[string]any{"name": "Renamed"}}))
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Someone who is not staff of the child course is passed through to
	// the route's own gate: the guard never reveals a lock.
	outsider := mount(func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(99))
		c.Locals("account_id", uint(1))
		c.Locals("is_admin", false)
		return c.Next()
	})
	resp = testutil.MakeRequest(outsider, http.MethodDelete, aPath, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
