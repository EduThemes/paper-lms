package handlers_test

// DB-backed tests for the modules lane: the progression engine and
// content gate (G061/G062), File / External Tool items (G064) and
// Mastery Paths visibility (G080). Skipped without Postgres.

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

type mpFixture struct {
	g                     *gorm.DB
	course                *models.Course
	teacher, student, alt uint
	m1, m2, m3            *models.ContextModule
	p1, p2                *models.WikiPage
	a1, a2                *models.Assignment
	itemP1, itemA1        *models.ContentTag
	itemP2, itemA2        *models.ContentTag
	progression           *service.ModuleProgressionService
}

func mpSeedUser(t *testing.T, g *gorm.DB, id, account uint, name string) {
	t.Helper()
	email := fmt.Sprintf("user%d@example.test", id)
	require.NoError(t, g.Create(&models.User{ID: id, AccountID: account, Name: name, SortableName: name, LoginID: email, Email: email, PasswordHash: "x", Role: "user"}).Error)
}

func mpEnroll(t *testing.T, g *gorm.DB, userID, courseID uint, typ string) {
	t.Helper()
	require.NoError(t, g.Create(&models.Enrollment{UserID: userID, CourseID: courseID, Type: typ, Role: typ, WorkflowState: models.EnrollmentActive}).Error)
}

func mpItem(t *testing.T, g *gorm.DB, moduleID uint, pos int, ct string, contentID uint, req string, url string) *models.ContentTag {
	t.Helper()
	id := contentID
	tag := &models.ContentTag{ContextModuleID: moduleID, Position: pos, ContentType: ct, ContentID: &id, Title: fmt.Sprintf("%s %d", ct, contentID), URL: url, CompletionType: req, WorkflowState: models.ContentTagActive}
	require.NoError(t, g.Create(tag).Error)
	return tag
}

func newMPFixture(t *testing.T) *mpFixture {
	t.Helper()
	g := contentLaneDB(t)
	seedContentAccounts(t, g, 1, 2)
	f := &mpFixture{g: g, teacher: 501, student: 502, alt: 503}
	mpSeedUser(t, g, f.teacher, 1, "Terry Teacher")
	mpSeedUser(t, g, f.student, 1, "Sam Student")
	mpSeedUser(t, g, f.alt, 1, "Alex Learner")
	f.course = seedContentCourse(t, g, 1, "BIO-101")
	mpEnroll(t, g, f.teacher, f.course.ID, "TeacherEnrollment")
	mpEnroll(t, g, f.student, f.course.ID, "StudentEnrollment")
	mpEnroll(t, g, f.alt, f.course.ID, "StudentEnrollment")

	f.p1 = &models.WikiPage{CourseID: f.course.ID, Title: "Welcome", URL: "welcome", WorkflowState: "active"}
	f.p2 = &models.WikiPage{CourseID: f.course.ID, Title: "Reading", URL: "reading", WorkflowState: "active"}
	require.NoError(t, g.Create(f.p1).Error)
	require.NoError(t, g.Create(f.p2).Error)
	f.a1 = seedContentAssignment(t, g, f.course.ID, "Lab report")
	f.a2 = seedContentAssignment(t, g, f.course.ID, "Unit test")

	future := time.Now().Add(14 * 24 * time.Hour)
	f.m1 = &models.ContextModule{CourseID: f.course.ID, Name: "Week 1", Position: 1, RequireSequentialProgress: true, WorkflowState: "active"}
	f.m2 = &models.ContextModule{CourseID: f.course.ID, Name: "Week 2", Position: 2, WorkflowState: "active"}
	f.m3 = &models.ContextModule{CourseID: f.course.ID, Name: "Week 3", Position: 3, UnlockAt: &future, WorkflowState: "active"}
	for _, m := range []*models.ContextModule{f.m1, f.m2, f.m3} {
		require.NoError(t, g.Create(m).Error)
	}
	require.NoError(t, postgres.NewModulePrerequisiteRepository(g).SetPrerequisites(context.Background(), f.m2.ID, []uint{f.m1.ID}))

	f.itemP1 = mpItem(t, g, f.m1.ID, 1, "WikiPage", f.p1.ID, models.ModuleCompletionMustView, "welcome")
	f.itemA1 = mpItem(t, g, f.m1.ID, 2, "Assignment", f.a1.ID, models.ModuleCompletionMustSubmit, "")
	f.itemP2 = mpItem(t, g, f.m1.ID, 3, "WikiPage", f.p2.ID, models.ModuleCompletionMustMarkDone, "reading")
	f.itemA2 = mpItem(t, g, f.m2.ID, 1, "Assignment", f.a2.ID, models.ModuleCompletionMustSubmit, "")

	f.progression = service.NewModuleProgressionService(postgres.NewModuleProgressionRepository(g))
	return f
}

// app mounts the progression routes behind an auth stub for userID and a
// stand-in for each gated content route.
func (f *mpFixture) app(userID, accountID uint, enrollmentType string) *fiber.App {
	h := handlers.NewModuleProgressionHandler(f.progression)
	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		c.Locals("account_id", accountID)
		c.Locals("enrollment_type", enrollmentType)
		c.Locals("is_admin", false)
		return c.Next()
	})
	app.Use(h.ContentGate())
	ok := func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) }
	app.Get("/courses/:course_id/assignments/:id", ok)
	app.Post("/courses/:course_id/assignments/:id/submissions", ok)
	app.Get("/courses/:course_id/pages/:url_or_id", ok)
	app.Get("/courses/:course_id/module_progress", h.GetProgress)
	app.Get("/courses/:course_id/module_progressions", h.Roster)
	app.Post("/courses/:course_id/modules/:module_id/items/:item_id/mark_read", h.MarkItemRead)
	app.Put("/courses/:course_id/modules/:module_id/items/:item_id/done", h.MarkItemDone)
	app.Delete("/courses/:course_id/modules/:module_id/items/:item_id/done", h.UnmarkItemDone)
	return app
}

type mpProgress struct {
	UserID  uint `json:"user_id"`
	Student bool `json:"student"`
	Modules []struct {
		ModuleID           uint       `json:"module_id"`
		State              string     `json:"state"`
		PrerequisitesUnmet []uint     `json:"prerequisites_unmet"`
		CurrentPosition    *int       `json:"current_position"`
		CompletedAt        *time.Time `json:"completed_at"`
		LockExplanation    string     `json:"lock_explanation"`
		Items              []struct {
			ItemID    uint   `json:"item_id"`
			Completed bool   `json:"completed"`
			Locked    bool   `json:"locked"`
			Type      string `json:"requirement_type"`
		} `json:"items"`
	} `json:"modules"`
}

func (f *mpFixture) progress(t *testing.T, app *fiber.App, query string) mpProgress {
	t.Helper()
	resp := testutil.MakeRequest(app, http.MethodGet, fmt.Sprintf("/courses/%d/module_progress%s", f.course.ID, query), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var p mpProgress
	require.NoError(t, testutil.ParseJSONResponse(resp, &p))
	return p
}

func moduleState(p mpProgress, id uint) (string, bool) {
	for _, m := range p.Modules {
		if m.ModuleID == id {
			return m.State, true
		}
	}
	return "", false
}

func itemState(p mpProgress, id uint) (completed, locked bool) {
	for _, m := range p.Modules {
		for _, it := range m.Items {
			if it.ItemID == id {
				return it.Completed, it.Locked
			}
		}
	}
	return false, false
}

func TestModuleProgression_PrerequisitesSequentialAndRequirements(t *testing.T) {
	f := newMPFixture(t)
	student := f.app(f.student, 1, "StudentEnrollment")
	cid := f.course.ID

	// Fresh student: Week 1 open (only its first item), Week 2 waits on
	// Week 1, Week 3 waits on its unlock date.
	p := f.progress(t, student, "")
	assert.True(t, p.Student)
	st, _ := moduleState(p, f.m1.ID)
	assert.Equal(t, "unlocked", st)
	st, _ = moduleState(p, f.m2.ID)
	assert.Equal(t, "locked", st)
	st, _ = moduleState(p, f.m3.ID)
	assert.Equal(t, "locked", st)
	_, locked := itemState(p, f.itemP1.ID)
	assert.False(t, locked)
	_, locked = itemState(p, f.itemA1.ID)
	assert.True(t, locked, "sequential progress locks items after the first unmet requirement")

	// The gate keeps locked content closed server-side.
	resp := testutil.MakeRequest(student, http.MethodGet, fmt.Sprintf("/courses/%d/assignments/%d", cid, f.a1.ID), nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	body, _ := testutil.ParseJSONMap(resp)
	assert.Contains(t, fmt.Sprint(body["lock_explanation"]), "Week 1")
	resp = testutil.MakeRequest(student, http.MethodPost, fmt.Sprintf("/courses/%d/assignments/%d/submissions", cid, f.a2.ID), nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "no submitting to a locked module's assignment")

	// Opening the page (by slug) satisfies must_view and opens the next item.
	resp = testutil.MakeRequest(student, http.MethodGet, fmt.Sprintf("/courses/%d/pages/welcome", cid), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	p = f.progress(t, student, "")
	done, _ := itemState(p, f.itemP1.ID)
	assert.True(t, done)
	_, locked = itemState(p, f.itemA1.ID)
	assert.False(t, locked)
	_, locked = itemState(p, f.itemP2.ID)
	assert.True(t, locked)
	st, _ = moduleState(p, f.m1.ID)
	assert.Equal(t, "started", st)

	// Mark-done is refused on an item without that requirement, and on
	// an item that is still locked.
	resp = testutil.MakeRequest(student, http.MethodPut, fmt.Sprintf("/courses/%d/modules/%d/items/%d/done", cid, f.m1.ID, f.itemA1.ID), nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp = testutil.MakeRequest(student, http.MethodPut, fmt.Sprintf("/courses/%d/modules/%d/items/%d/done", cid, f.m1.ID, f.itemP2.ID), nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	// Wrong module in the URL: 404 (parent tie).
	resp = testutil.MakeRequest(student, http.MethodPut, fmt.Sprintf("/courses/%d/modules/%d/items/%d/done", cid, f.m2.ID, f.itemP2.ID), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Submitting the assignment meets must_submit.
	now := time.Now()
	require.NoError(t, f.g.Create(&models.Submission{AssignmentID: f.a1.ID, UserID: f.student, WorkflowState: models.SubmissionSubmitted, SubmittedAt: &now, Attempt: 1}).Error)
	resp = testutil.MakeRequest(student, http.MethodPut, fmt.Sprintf("/courses/%d/modules/%d/items/%d/done", cid, f.m1.ID, f.itemP2.ID), nil)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	p = f.progress(t, student, "")
	st, _ = moduleState(p, f.m1.ID)
	assert.Equal(t, "completed", st)
	for _, m := range p.Modules {
		if m.ModuleID == f.m1.ID {
			assert.NotNil(t, m.CompletedAt)
		}
	}
	st, _ = moduleState(p, f.m2.ID)
	assert.Equal(t, "unlocked", st, "completing the prerequisite unlocks Week 2")
	resp = testutil.MakeRequest(student, http.MethodGet, fmt.Sprintf("/courses/%d/assignments/%d", cid, f.a2.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Un-marking re-opens the requirement.
	resp = testutil.MakeRequest(student, http.MethodDelete, fmt.Sprintf("/courses/%d/modules/%d/items/%d/done", cid, f.m1.ID, f.itemP2.ID), nil)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	p = f.progress(t, student, "")
	st, _ = moduleState(p, f.m1.ID)
	assert.Equal(t, "started", st)

	// The progression is persisted for teachers.
	var row models.ContextModuleProgression
	require.NoError(t, f.g.Where("context_module_id = ? AND user_id = ?", f.m1.ID, f.student).First(&row).Error)
	assert.Equal(t, models.ModuleProgressionStarted, row.WorkflowState)
}

func TestModuleProgression_StaffAccessAndScoping(t *testing.T) {
	f := newMPFixture(t)
	cid := f.course.ID
	teacher := f.app(f.teacher, 1, "TeacherEnrollment")

	// Locks never apply to staff.
	resp := testutil.MakeRequest(teacher, http.MethodGet, fmt.Sprintf("/courses/%d/assignments/%d", cid, f.a2.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	p := f.progress(t, teacher, "")
	assert.False(t, p.Student)
	assert.Empty(t, p.Modules)

	// A teacher reads a student's progress and the roster.
	p = f.progress(t, teacher, fmt.Sprintf("?user_id=%d", f.student))
	assert.Equal(t, f.student, p.UserID)
	assert.Len(t, p.Modules, 3)
	resp = testutil.MakeRequest(teacher, http.MethodGet, fmt.Sprintf("/courses/%d/module_progressions", cid), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var roster service.ProgressRoster
	require.NoError(t, testutil.ParseJSONResponse(resp, &roster))
	assert.Len(t, roster.Modules, 3)
	require.Len(t, roster.Students, 2)
	assert.Equal(t, models.ModuleProgressionUnlocked, roster.Students[0].States[f.m1.ID])

	// A student may not read a classmate's progress; a non-member is unknown.
	alt := f.app(f.alt, 1, "StudentEnrollment")
	resp = testutil.MakeRequest(alt, http.MethodGet, fmt.Sprintf("/courses/%d/module_progress?user_id=%d", cid, f.student), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp = testutil.MakeRequest(teacher, http.MethodGet, fmt.Sprintf("/courses/%d/module_progress?user_id=%d", cid, 9999), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Another tenant: the course does not exist.
	other := f.app(f.student, 2, "StudentEnrollment")
	resp = testutil.MakeRequest(other, http.MethodGet, fmt.Sprintf("/courses/%d/module_progress", cid), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Staff mark_read is a harmless no-op; staff cannot mark done.
	resp = testutil.MakeRequest(teacher, http.MethodPost, fmt.Sprintf("/courses/%d/modules/%d/items/%d/mark_read", cid, f.m1.ID, f.itemP1.ID), nil)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp = testutil.MakeRequest(teacher, http.MethodPut, fmt.Sprintf("/courses/%d/modules/%d/items/%d/done", cid, f.m1.ID, f.itemP2.ID), nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	// A student's mark_read on a locked item is refused.
	student := f.app(f.student, 1, "StudentEnrollment")
	resp = testutil.MakeRequest(student, http.MethodPost, fmt.Sprintf("/courses/%d/modules/%d/items/%d/mark_read", cid, f.m2.ID, f.itemA2.ID), nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp = testutil.MakeRequest(student, http.MethodPost, fmt.Sprintf("/courses/%d/modules/%d/items/%d/mark_read", cid, f.m1.ID, f.itemP1.ID), nil)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestModuleItems_FileAndExternalTool(t *testing.T) {
	f := newMPFixture(t)
	g := f.g
	ctx := context.Background()
	other := seedContentCourse(t, g, 1, "CHEM-1")

	att := &models.Attachment{ContextType: "Course", ContextID: f.course.ID, UserID: f.teacher, DisplayName: "syllabus.pdf", Filename: "syllabus.pdf", ContentType: "application/pdf", Size: 10, StoragePath: "x"}
	foreign := &models.Attachment{ContextType: "Course", ContextID: other.ID, UserID: f.teacher, DisplayName: "secret.pdf", Filename: "secret.pdf", ContentType: "application/pdf", Size: 10, StoragePath: "y"}
	require.NoError(t, g.Create(att).Error)
	require.NoError(t, g.Create(foreign).Error)

	key := &models.DeveloperKey{AccountID: 1, Name: "Vendor", ClientID: "vendor-client-1", IsLTIKey: true, WorkflowState: "active"}
	require.NoError(t, g.Create(key).Error)
	tool := &models.ContextExternalTool{ContextType: "Account", ContextID: 1, DeveloperKeyID: key.ID, Name: "Lab Sim", URL: "https://labsim.example.org/launch", Domain: "example.org", WorkflowState: "active"}
	foreignTool := &models.ContextExternalTool{ContextType: "Course", ContextID: other.ID, DeveloperKeyID: key.ID, Name: "Other", URL: "https://other.example.org", WorkflowState: "active"}
	require.NoError(t, g.Create(tool).Error)
	require.NoError(t, g.Create(foreignTool).Error)

	moduleRepo := postgres.NewModuleRepository(g)
	moduleSvc := service.NewModuleService(moduleRepo, postgres.NewModuleItemRepository(g))
	moduleSvc.SetLinkRepos(postgres.NewAttachmentRepository(g), postgres.NewContextExternalToolRepository(g), postgres.NewDeveloperKeyRepository(g), postgres.NewCourseRepository(g))
	h := handlers.NewModuleItemHandler(moduleSvc, nil)
	h.SetProgressionService(f.progression)

	mkApp := func(userID uint, et string) *fiber.App {
		app := testutil.SetupTestApp()
		stub := func(c *fiber.Ctx) error {
			c.Locals("user_id", userID)
			c.Locals("account_id", uint(1))
			c.Locals("enrollment_type", et)
			return c.Next()
		}
		app.Post("/courses/:course_id/modules/:module_id/items", stub, h.CreateModuleItem)
		app.Get("/courses/:course_id/modules/:module_id/items/:item_id/launch", stub, h.LaunchParams)
		return app
	}
	teacher := mkApp(f.teacher, "TeacherEnrollment")
	path := fmt.Sprintf("/courses/%d/modules/%d/items", f.course.ID, f.m3.ID)

	resp := testutil.MakeRequest(teacher, http.MethodPost, path, testutil.JSONBody(map[string]any{"module_item": map[string]any{"type": "File", "content_id": att.ID}}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	created, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, "File", created["type"])
	assert.Equal(t, "syllabus.pdf", created["title"])

	resp = testutil.MakeRequest(teacher, http.MethodPost, path, testutil.JSONBody(map[string]any{"module_item": map[string]any{"type": "File", "content_id": foreign.ID}}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "another course's file cannot be linked")

	resp = testutil.MakeRequest(teacher, http.MethodPost, path, testutil.JSONBody(map[string]any{"module_item": map[string]any{"type": "ExternalTool", "content_id": foreignTool.ID}}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "a tool installed in another course is not available")
	resp = testutil.MakeRequest(teacher, http.MethodPost, path, testutil.JSONBody(map[string]any{"module_item": map[string]any{"type": "ExternalTool", "content_id": tool.ID, "external_url": "https://evil.example.com/x"}}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "launch URL must stay on the tool's domain")
	resp = testutil.MakeRequest(teacher, http.MethodPost, path, testutil.JSONBody(map[string]any{"module_item": map[string]any{"type": "ExternalUrl", "title": "x", "external_url": "javascript:alert(1)"}}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	resp = testutil.MakeRequest(teacher, http.MethodPost, path, testutil.JSONBody(map[string]any{"module_item": map[string]any{"type": "ExternalTool", "content_id": tool.ID, "external_url": "https://labsim.example.org/unit/4"}}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	toolItem, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, "ExternalTool", toolItem["type"])
	assert.Equal(t, "Lab Sim", toolItem["title"])
	toolItemID := uint(toolItem["id"].(float64))

	launchPath := fmt.Sprintf("/courses/%d/modules/%d/items/%d/launch", f.course.ID, f.m3.ID, toolItemID)
	resp = testutil.MakeRequest(teacher, http.MethodGet, launchPath, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var launch service.ToolLaunch
	require.NoError(t, testutil.ParseJSONResponse(resp, &launch))
	assert.Equal(t, "vendor-client-1", launch.ClientID)
	assert.Equal(t, "https://labsim.example.org/unit/4", launch.TargetLinkURI)
	assert.Equal(t, fmt.Sprintf("%d:module_item_%d:%d", f.course.ID, toolItemID, key.ID), launch.LTIMessageHint)
	_, _, _, err := service.ParseLTIMessageHint(launch.LTIMessageHint)
	require.NoError(t, err)

	// Week 3 is date-locked: a student cannot launch its tool.
	student := mkApp(f.student, "StudentEnrollment")
	resp = testutil.MakeRequest(student, http.MethodGet, launchPath, nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	_ = ctx
}

func TestMasteryPaths_GateAssignmentVisibility(t *testing.T) {
	f := newMPFixture(t)
	g := f.g
	ctx := context.Background()
	cid := f.course.ID
	pts := 10.0
	trigger := &models.Assignment{CourseID: cid, Name: "Diagnostic", PointsPossible: &pts, WorkflowState: models.AssignmentPublished, Published: true}
	require.NoError(t, g.Create(trigger).Error)
	low := seedContentAssignment(t, g, cid, "Review set")
	high := seedContentAssignment(t, g, cid, "Challenge set")

	submissions := postgres.NewSubmissionRepository(g)
	assignments := postgres.NewAssignmentRepository(g)
	mastery := service.NewMasteryPathService(postgres.NewMasteryPathRepository(g), submissions, assignments)
	_, err := mastery.CreateRule(ctx, cid, trigger.ID, []service.RangeInput{
		{LowerBound: 0, UpperBound: 70, AssignmentIDs: []uint{low.ID}},
		{LowerBound: 70, UpperBound: 100, AssignmentIDs: []uint{high.ID}},
	})
	require.NoError(t, err)
	f.progression.SetAssignmentVisibility(mastery)
	assignmentSvc := service.NewAssignmentService(assignments)
	assignmentSvc.SetStudentVisibility(mastery)

	// Path assignments in a module.
	pathModule := &models.ContextModule{CourseID: cid, Name: "Paths", Position: 4, WorkflowState: "active"}
	require.NoError(t, g.Create(pathModule).Error)
	mpItem(t, g, pathModule.ID, 1, "Assignment", low.ID, "", "")
	mpItem(t, g, pathModule.ID, 2, "Assignment", high.ID, "", "")

	moduleHandler := handlers.NewModuleHandler(service.NewModuleService(postgres.NewModuleRepository(g), postgres.NewModuleItemRepository(g)))
	moduleHandler.SetProgressionService(f.progression)
	assignmentHandler := handlers.NewAssignmentHandler(assignmentSvc)

	app := f.app(f.student, 1, "StudentEnrollment")
	app.Get("/courses/:course_id/assignments", assignmentHandler.ListAssignments)
	app.Get("/courses/:course_id/modules", moduleHandler.ListModules)

	listed := func() map[uint]bool {
		resp := testutil.MakeRequest(app, http.MethodGet, fmt.Sprintf("/courses/%d/assignments?per_page=100", cid), nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		rows, err := testutil.ParseJSONArray(resp)
		require.NoError(t, err)
		out := map[uint]bool{}
		for _, r := range rows {
			out[uint(r["id"].(float64))] = true
		}
		return out
	}
	moduleItems := func() map[uint]bool {
		resp := testutil.MakeRequest(app, http.MethodGet, fmt.Sprintf("/courses/%d/modules?include[]=items", cid), nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		rows, err := testutil.ParseJSONArray(resp)
		require.NoError(t, err)
		out := map[uint]bool{}
		for _, r := range rows {
			items, _ := r["items"].([]interface{})
			for _, it := range items {
				m := it.(map[string]interface{})
				if cidv, ok := m["content_id"].(float64); ok {
					out[uint(cidv)] = true
				}
			}
		}
		return out
	}
	get := func(id uint) int {
		return testutil.MakeRequest(app, http.MethodGet, fmt.Sprintf("/courses/%d/assignments/%d", cid, id), nil).StatusCode
	}

	// Before grading: neither path is visible anywhere.
	seen := listed()
	assert.True(t, seen[trigger.ID])
	assert.False(t, seen[low.ID])
	assert.False(t, seen[high.ID])
	assert.Equal(t, http.StatusNotFound, get(high.ID))
	items := moduleItems()
	assert.False(t, items[low.ID])
	assert.False(t, items[high.ID])

	// Graded 9/10 and posted: the challenge path is released.
	score := 9.0
	posted := time.Now()
	sub := &models.Submission{AssignmentID: trigger.ID, UserID: f.student, Score: &score, PostedAt: &posted, WorkflowState: models.SubmissionGraded, Attempt: 1, SubmittedAt: &posted}
	require.NoError(t, g.Create(sub).Error)
	require.NoError(t, mastery.EvaluateForStudent(ctx, sub.ID))
	seen = listed()
	assert.True(t, seen[high.ID])
	assert.False(t, seen[low.ID])
	assert.Equal(t, http.StatusOK, get(high.ID))
	assert.Equal(t, http.StatusNotFound, get(low.ID))
	items = moduleItems()
	assert.True(t, items[high.ID])
	assert.False(t, items[low.ID])

	// Regraded to 5/10: the student moves to the review path.
	score = 5.0
	require.NoError(t, g.Model(sub).Update("score", score).Error)
	require.NoError(t, mastery.EvaluateForStudent(ctx, sub.ID))
	seen = listed()
	assert.True(t, seen[low.ID])
	assert.False(t, seen[high.ID])

	// Editing the rule keeps the release (sets are recreated).
	_, err = mastery.CreateRule(ctx, cid, trigger.ID, []service.RangeInput{
		{LowerBound: 0, UpperBound: 60, AssignmentIDs: []uint{low.ID}},
		{LowerBound: 60, UpperBound: 100, AssignmentIDs: []uint{high.ID}},
	})
	require.NoError(t, err)
	seen = listed()
	assert.True(t, seen[low.ID])
	assert.False(t, seen[high.ID])

	// Another student was never graded: nothing released.
	altApp := f.app(f.alt, 1, "StudentEnrollment")
	altApp.Get("/courses/:course_id/assignments", assignmentHandler.ListAssignments)
	resp := testutil.MakeRequest(altApp, http.MethodGet, fmt.Sprintf("/courses/%d/assignments/%d", cid, low.ID), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// G063: a module item that links draft or deleted content is hidden from
// students (module lists and progress requirements) while staff still
// see it; publishing the content brings the item back.
func TestModuleItems_DraftContentHiddenFromStudents(t *testing.T) {
	f := newMPFixture(t)
	g := f.g
	cid := f.course.ID

	draftPage := &models.WikiPage{CourseID: cid, Title: "Draft notes", URL: "draft-notes", WorkflowState: "unpublished"}
	require.NoError(t, g.Create(draftPage).Error)
	draftAssignment := &models.Assignment{CourseID: cid, Name: "Draft essay", WorkflowState: models.AssignmentUnpublished}
	require.NoError(t, g.Create(draftAssignment).Error)
	mod := &models.ContextModule{CourseID: cid, Name: "Extras", Position: 5, WorkflowState: "active"}
	require.NoError(t, g.Create(mod).Error)
	pageItem := mpItem(t, g, mod.ID, 1, "WikiPage", draftPage.ID, models.ModuleCompletionMustView, "draft-notes")
	assignmentItem := mpItem(t, g, mod.ID, 2, "Assignment", draftAssignment.ID, "", "")
	urlItem := &models.ContentTag{ContextModuleID: mod.ID, Position: 3, ContentType: "ExternalUrl", Title: "Reference", URL: "https://example.org/ref", WorkflowState: models.ContentTagActive}
	require.NoError(t, g.Create(urlItem).Error)

	moduleHandler := handlers.NewModuleHandler(service.NewModuleService(postgres.NewModuleRepository(g), postgres.NewModuleItemRepository(g)))
	moduleHandler.SetProgressionService(f.progression)
	itemsOf := func(userID uint, enrollment string) map[uint]bool {
		app := f.app(userID, 1, enrollment)
		app.Get("/courses/:course_id/modules", moduleHandler.ListModules)
		resp := testutil.MakeRequest(app, http.MethodGet, fmt.Sprintf("/courses/%d/modules?include[]=items&per_page=50", cid), nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		rows, err := testutil.ParseJSONArray(resp)
		require.NoError(t, err)
		out := map[uint]bool{}
		for _, r := range rows {
			items, _ := r["items"].([]interface{})
			for _, it := range items {
				out[uint(it.(map[string]interface{})["id"].(float64))] = true
			}
		}
		return out
	}

	student := itemsOf(f.student, "StudentEnrollment")
	assert.False(t, student[pageItem.ID], "draft page item hidden from students")
	assert.False(t, student[assignmentItem.ID], "draft assignment item hidden from students")
	assert.True(t, student[urlItem.ID], "items without publishable content stay visible")
	teacher := itemsOf(f.teacher, "TeacherEnrollment")
	assert.True(t, teacher[pageItem.ID])
	assert.True(t, teacher[assignmentItem.ID])

	// The hidden must_view requirement does not count toward the module.
	p := f.progress(t, f.app(f.student, 1, "StudentEnrollment"), "")
	_, locked := itemState(p, pageItem.ID)
	assert.False(t, locked)
	for _, m := range p.Modules {
		if m.ModuleID != mod.ID {
			continue
		}
		for _, it := range m.Items {
			assert.NotEqual(t, pageItem.ID, it.ItemID, "a hidden item is not in the student's progress")
		}
	}

	// Publishing the page brings its item back.
	require.NoError(t, g.Model(draftPage).Update("workflow_state", "active").Error)
	student = itemsOf(f.student, "StudentEnrollment")
	assert.True(t, student[pageItem.ID])
	assert.False(t, student[assignmentItem.ID])
}

// The content gate matches request paths the way Fiber routes them:
// Fiber's default routing ignores a trailing slash and letter case, and
// ParamsInt accepts a leading "+", so each of those spellings reaches
// the content handler and must be gated too. The discussion page the UI
// renders reads /view_v2 and the entry lists, which are gated as well.
func TestModuleContentGate_PathVariantsAndDiscussionReads(t *testing.T) {
	f := newMPFixture(t)
	g := f.g
	cid := f.course.ID

	topic := &models.DiscussionTopic{CourseID: cid, Title: "Week 2 debate", Message: "prompt", WorkflowState: "active"}
	require.NoError(t, g.Create(topic).Error)
	mpItem(t, g, f.m2.ID, 2, "DiscussionTopic", topic.ID, "", "")

	student := f.app(f.student, 1, "StudentEnrollment")
	ok := func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) }
	student.Get("/courses/:course_id/discussion_topics/:topic_id", ok)
	student.Get("/courses/:course_id/discussion_topics/:topic_id/view_v2", ok)
	student.Get("/courses/:course_id/discussion_topics/:topic_id/entries", ok)
	student.Get("/courses/:course_id/discussion_topics/:topic_id/entries/:entry_id/replies", ok)

	// Week 2 is locked behind Week 1, so every spelling is refused.
	for _, path := range []string{
		fmt.Sprintf("/courses/%d/assignments/%d", cid, f.a2.ID),
		fmt.Sprintf("/courses/%d/assignments/%d/", cid, f.a2.ID),
		fmt.Sprintf("/Courses/%d/Assignments/%d", cid, f.a2.ID),
		fmt.Sprintf("/courses/%d/assignments/+%d", cid, f.a2.ID),
		fmt.Sprintf("/courses/%d/discussion_topics/%d", cid, topic.ID),
		fmt.Sprintf("/courses/%d/discussion_topics/%d/view_v2", cid, topic.ID),
		fmt.Sprintf("/courses/%d/discussion_topics/%d/entries", cid, topic.ID),
		fmt.Sprintf("/courses/%d/discussion_topics/%d/entries/1/replies", cid, topic.ID),
	} {
		resp := testutil.MakeRequest(student, http.MethodGet, path, nil)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "GET %s must be gated", path)
	}
	resp := testutil.MakeRequest(student, http.MethodPost, fmt.Sprintf("/courses/%d/assignments/%d/submissions/", cid, f.a2.ID), nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	// A numeric page key is the page id, exactly as the page handler
	// reads it ("+id" included), even when another page's slug is that
	// number.
	locked := &models.WikiPage{CourseID: cid, Title: "Week 2 notes", URL: "week-2-notes", WorkflowState: "active"}
	require.NoError(t, g.Create(locked).Error)
	mpItem(t, g, f.m2.ID, 3, "WikiPage", locked.ID, "", "week-2-notes")
	decoy := &models.WikiPage{CourseID: cid, Title: "Decoy", URL: fmt.Sprint(locked.ID), WorkflowState: "active"}
	require.NoError(t, g.Create(decoy).Error)
	for _, key := range []string{fmt.Sprint(locked.ID), fmt.Sprintf("+%d", locked.ID), "week-2-notes"} {
		resp = testutil.MakeRequest(student, http.MethodGet, fmt.Sprintf("/courses/%d/pages/%s", cid, key), nil)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "page key %q must be gated", key)
	}

	// The indexes keep a locked row's title but not its body.
	require.NoError(t, g.Model(f.a2).Update("description", "<p>Week 2 instructions</p>").Error)
	require.NoError(t, g.Model(f.a1).Update("description", "<p>Lab steps</p>").Error)
	assignmentHandler := handlers.NewAssignmentHandler(service.NewAssignmentService(postgres.NewAssignmentRepository(g)))
	listApp := func(userID uint, enrollment string) *fiber.App {
		app := f.app(userID, 1, enrollment)
		app.Get("/courses/:course_id/assignments", assignmentHandler.ListAssignments)
		app.Get("/courses/:course_id/pages", func(c *fiber.Ctx) error {
			return c.JSON([]fiber.Map{{"page_id": locked.ID, "title": locked.Title, "body": "<p>notes</p>"}, {"page_id": f.p1.ID, "title": f.p1.Title, "body": "<p>hi</p>"}})
		})
		return app
	}
	rowsOf := func(app *fiber.App, path, idField string) map[uint]map[string]interface{} {
		resp := testutil.MakeRequest(app, http.MethodGet, path, nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		rows, err := testutil.ParseJSONArray(resp)
		require.NoError(t, err)
		out := map[uint]map[string]interface{}{}
		for _, r := range rows {
			out[uint(r[idField].(float64))] = r
		}
		return out
	}
	rows := rowsOf(listApp(f.student, "StudentEnrollment"), fmt.Sprintf("/courses/%d/assignments?per_page=100", cid), "id")
	require.Contains(t, rows, f.a2.ID)
	assert.Equal(t, f.a2.Name, rows[f.a2.ID]["name"])
	assert.Equal(t, "", rows[f.a2.ID]["description"], "a locked assignment's description is withheld")
	assert.Equal(t, true, rows[f.a2.ID]["locked_for_user"])
	assert.Contains(t, fmt.Sprint(rows[f.a2.ID]["lock_explanation"]), "Week 2")
	assert.Equal(t, "", rows[f.a1.ID]["description"], "sequential progress locks the lab too")
	pages := rowsOf(listApp(f.student, "StudentEnrollment"), fmt.Sprintf("/courses/%d/pages/", cid), "page_id")
	assert.Equal(t, "", pages[locked.ID]["body"])
	assert.Equal(t, "<p>hi</p>", pages[f.p1.ID]["body"], "open content keeps its body")
	assert.Nil(t, pages[f.p1.ID]["locked_for_user"])
	rows = rowsOf(listApp(f.teacher, "TeacherEnrollment"), fmt.Sprintf("/courses/%d/assignments?per_page=100", cid), "id")
	assert.Equal(t, "<p>Week 2 instructions</p>", rows[f.a2.ID]["description"], "staff see everything")

	// Staff are never gated.
	teacher := f.app(f.teacher, 1, "TeacherEnrollment")
	teacher.Get("/courses/:course_id/discussion_topics/:topic_id/view_v2", ok)
	resp = testutil.MakeRequest(teacher, http.MethodGet, fmt.Sprintf("/courses/%d/discussion_topics/%d/view_v2", cid, topic.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
