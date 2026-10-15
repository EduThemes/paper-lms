package handlers_test

// DB-backed tests for the Page Designer API additions
// (docs/page-designer/README.md): page css/design, revisions, the
// custom-CSS setting, the course look and saved colour schemes. All data
// is made up.

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/security/csssanitize"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/service/settings"
	"github.com/EduThemes/paper-lms/internal/settingsctx"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

type designFixture struct {
	g           *gorm.DB
	pages       *handlers.PageHandler
	revs        *handlers.WikiPageRevisionHandler
	courses     *handlers.CourseHandler
	schemes     *handlers.DesignColourSchemeHandler
	cssSetting  string // value the settings lookup returns for pages.custom_css_enabled
	lookupAccts []uint // accounts the lookup was asked about
}

func newDesignFixture(t *testing.T) *designFixture {
	t.Helper()
	g := coreScratchDB(t)
	for _, c := range []*models.Course{
		{ID: 100, AccountID: 1, Name: "Biology", CourseCode: "BIO", WorkflowState: models.CourseAvailable},
		{ID: 101, AccountID: 1, Name: "Chemistry", CourseCode: "CHEM", WorkflowState: models.CourseAvailable},
		{ID: 200, AccountID: 2, Name: "Other Tenant", CourseCode: "OT", WorkflowState: models.CourseAvailable},
	} {
		require.NoError(t, g.Create(c).Error)
	}
	users := postgres.NewUserRepository(g)
	require.NoError(t, users.Create(context.Background(), &models.User{ID: 9, AccountID: 1, Name: "John Roe", LoginID: "john@example.test", Email: "john@example.test", PasswordHash: "x", Role: "user"}))

	f := &designFixture{g: g, cssSetting: "true"}
	lookup := func(ctx context.Context, key string) (string, error) {
		if key == settings.PagesCustomCSSEnabled {
			f.lookupAccts = append(f.lookupAccts, settingsctx.AccountIDFromContext(ctx))
			return f.cssSetting, nil
		}
		return "", nil
	}

	courseRepo := postgres.NewCourseRepository(g)
	policy := service.NewCourseCSSPolicy(courseRepo, lookup)
	pageRepo := postgres.NewPageRepository(g)
	revSvc := service.NewWikiPageRevisionService(postgres.NewWikiPageRevisionRepository(g), pageRepo).SetCourseCSSPolicy(policy)
	f.pages = handlers.NewPageHandler(service.NewPageService(pageRepo)).SetRevisionService(revSvc).SetCourseCSSPolicy(policy)
	f.revs = handlers.NewWikiPageRevisionHandler(f.pages, revSvc)

	enrollRepo := postgres.NewEnrollmentRepository(g)
	f.courses = handlers.NewCourseHandler(
		service.NewCourseService(courseRepo, enrollRepo, postgres.NewSectionRepository(g)),
		service.NewEnrollmentService(enrollRepo),
	).SetPageDesigner(postgres.NewAttachmentRepository(g), lookup)

	f.schemes = handlers.NewDesignColourSchemeHandler(postgres.NewDesignColourSchemeRepository(g))
	return f
}

func (f *designFixture) app(userID, accountID uint, enrollmentType string) *fiber.App {
	app := testutil.SetupTestApp()
	app.Use(pfStub(userID, accountID, enrollmentType))
	app.Get("/courses/:course_id/pages", f.pages.ListPages)
	app.Post("/courses/:course_id/pages", f.pages.CreatePage)
	app.Get("/courses/:course_id/pages/:url_or_id", f.pages.GetPage)
	app.Put("/courses/:course_id/pages/:url_or_id", f.pages.UpdatePage)
	app.Get("/courses/:course_id/front_page", f.pages.GetFrontPage)
	app.Get("/courses/:course_id/p/:slug", f.pages.GetPublicPage)
	app.Get("/courses/:course_id/pages/:url_or_id/revisions", f.revs.ListRevisions)
	app.Post("/courses/:course_id/pages/:url_or_id/revisions/:revision_id", f.revs.RevertToRevision)
	app.Get("/courses/:id", f.courses.GetCourse)
	app.Put("/courses/:id", f.courses.UpdateCourse)
	app.Get("/users/self/design_colour_schemes", f.schemes.List)
	app.Post("/users/self/design_colour_schemes", f.schemes.Create)
	app.Put("/users/self/design_colour_schemes/:id", f.schemes.Update)
	app.Delete("/users/self/design_colour_schemes/:id", f.schemes.Delete)
	return app
}

func designDoc(id string) map[string]any {
	return map[string]any{
		"version": 1, "id": id, "themeId": "tide-pool", "paletteId": "default", "layout": "classic-band",
		"kind": "home", "locale": "en", "sections": []any{},
		"options": map[string]any{"density": "comfortable", "headingStyle": "band"},
	}
}

func jsonMap(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	m, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	return m
}

func TestPageDesign_CreateShowListUpdateAndRevisions(t *testing.T) {
	f := newDesignFixture(t)
	teacher := f.app(8, 1, "TeacherEnrollment")

	resp := testutil.MakeRequest(teacher, http.MethodPost, "/courses/100/pages", pageBody(map[string]any{
		"title": "Home", "body": "<p>Welcome</p>", "published": true,
		"css": "p{color:red}", "design": designDoc("doc-1"),
	}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	created := jsonMap(t, resp)
	assert.Equal(t, true, created["has_design"])
	require.IsType(t, map[string]any{}, created["design"])
	assert.Equal(t, "doc-1", created["design"].(map[string]any)["id"])
	pageID := created["page_id"]

	resp = testutil.MakeRequest(teacher, http.MethodGet, "/courses/100/pages/home", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	shown := jsonMap(t, resp)
	assert.Equal(t, "doc-1", shown["design"].(map[string]any)["id"])
	assert.Contains(t, shown, "css")
	assert.Contains(t, shown, "scoped_css")
	assert.NotEmpty(t, shown["css"])
	assert.Contains(t, shown["scoped_css"], fmt.Sprintf("#paper-page-%v", pageID))

	// The list says which pages are designed but carries neither field.
	resp = testutil.MakeRequest(teacher, http.MethodPost, "/courses/100/pages", pageBody(map[string]any{"title": "Plain", "published": true}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp = testutil.MakeRequest(teacher, http.MethodGet, "/courses/100/pages", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	list, err := testutil.ParseJSONArray(resp)
	require.NoError(t, err)
	require.Len(t, list, 2)
	byTitle := map[string]map[string]any{}
	for _, p := range list {
		byTitle[p["title"].(string)] = p
		assert.NotContains(t, p, "css")
		assert.NotContains(t, p, "design")
		assert.NotContains(t, p, "scoped_css")
	}
	assert.Equal(t, true, byTitle["Home"]["has_design"])
	assert.Equal(t, false, byTitle["Plain"]["has_design"])

	// The stored design is a real jsonb document, and an ordinary page
	// keeps SQL NULL (not the JSON literal null).
	var nulls int64
	require.NoError(t, f.g.Raw(`SELECT count(*) FROM wiki_pages WHERE course_id = 100 AND design IS NULL`).Scan(&nulls).Error)
	assert.EqualValues(t, 1, nulls)
	var docID string
	require.NoError(t, f.g.Raw(`SELECT design->>'id' FROM wiki_pages WHERE course_id = 100 AND url = 'home'`).Scan(&docID).Error)
	assert.Equal(t, "doc-1", docID)

	// A new design is a new revision; an unchanged one is not.
	resp = testutil.MakeRequest(teacher, http.MethodPut, "/courses/100/pages/home", pageBody(map[string]any{"design": designDoc("doc-2")}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp = testutil.MakeRequest(teacher, http.MethodPut, "/courses/100/pages/home", pageBody(map[string]any{"design": designDoc("doc-2")}))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// design: null turns it back into an ordinary page.
	resp = testutil.MakeRequest(teacher, http.MethodPut, "/courses/100/pages/home", pageBody(map[string]any{"design": nil}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	cleared := jsonMap(t, resp)
	assert.Nil(t, cleared["design"])
	assert.Equal(t, false, cleared["has_design"])

	resp = testutil.MakeRequest(teacher, http.MethodGet, "/courses/100/pages/home/revisions", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	revs, err := testutil.ParseJSONArray(resp)
	require.NoError(t, err)
	require.Len(t, revs, 3) // create, doc-2, cleared
	assert.Nil(t, revs[0]["design"])
	assert.Equal(t, "doc-2", revs[1]["design"].(map[string]any)["id"])
	assert.Equal(t, "doc-1", revs[2]["design"].(map[string]any)["id"])

	// Restoring revision 1 brings the design (and stylesheet) back.
	resp = testutil.MakeRequest(teacher, http.MethodPost, "/courses/100/pages/home/revisions/1", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp = testutil.MakeRequest(teacher, http.MethodGet, "/courses/100/pages/home", nil)
	restored := jsonMap(t, resp)
	require.IsType(t, map[string]any{}, restored["design"])
	assert.Equal(t, "doc-1", restored["design"].(map[string]any)["id"])
	assert.NotEmpty(t, restored["css"])
}

// The editor gets the stylesheet back as the teacher typed it (comments and
// formatting), readers never do, and a source that no longer sanitizes to
// the stored CSS is not shown at all.
func TestPageDesign_CSSSourceKeptForStaff(t *testing.T) {
	f := newDesignFixture(t)
	teacher := f.app(8, 1, "TeacherEnrollment")
	student := f.app(9, 1, "StudentEnrollment")
	typed := "/* brand colours */\np {\n  color: red;\n}\n"

	resp := testutil.MakeRequest(teacher, http.MethodPost, "/courses/100/pages", pageBody(map[string]any{
		"title": "Styled", "body": "<p>Hi</p>", "published": true, "css": typed,
	}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	created := jsonMap(t, resp)
	assert.Equal(t, typed, created["css_source"])
	assert.NotContains(t, created["css"], "brand colours")

	resp = testutil.MakeRequest(student, http.MethodGet, "/courses/100/pages/styled", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotContains(t, jsonMap(t, resp), "css_source")

	// A copy or sync that rewrote css without the source: fall back to css.
	require.NoError(t, f.g.Exec("UPDATE wiki_pages SET css = 'p{color:blue}' WHERE url = 'styled'").Error)
	resp = testutil.MakeRequest(teacher, http.MethodGet, "/courses/100/pages/styled", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotContains(t, jsonMap(t, resp), "css_source")

	// Clearing the stylesheet clears the source too.
	resp = testutil.MakeRequest(teacher, http.MethodPut, "/courses/100/pages/styled", pageBody(map[string]any{"css": ""}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var source string
	require.NoError(t, f.g.Raw("SELECT css_source FROM wiki_pages WHERE url = 'styled'").Scan(&source).Error)
	assert.Empty(t, source)
}

// The front page and the public page render pages too, so they share the
// show shape: staff get the design, readers (students, anonymous visitors
// of a public page) only has_design and what they render.
func TestPageDesign_FrontAndPublicPagesCarryDesign(t *testing.T) {
	f := newDesignFixture(t)
	teacher := f.app(8, 1, "TeacherEnrollment")
	student := f.app(9, 1, "StudentEnrollment")
	anonymous := f.app(0, 0, "")
	resp := testutil.MakeRequest(teacher, http.MethodPost, "/courses/100/pages", pageBody(map[string]any{
		"title": "Home", "published": true, "front_page": true, "public": true, "css": "p{color:red}", "design": designDoc("doc-f"),
	}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	for _, path := range []string{"/courses/100/front_page", "/courses/100/p/home", "/courses/100/pages/home"} {
		resp = testutil.MakeRequest(teacher, http.MethodGet, path, nil)
		require.Equal(t, http.StatusOK, resp.StatusCode, path)
		got := jsonMap(t, resp)
		assert.Contains(t, got, "css", path)
		assert.Contains(t, got, "scoped_css", path)
		require.IsType(t, map[string]any{}, got["design"], path)
		assert.Equal(t, "doc-f", got["design"].(map[string]any)["id"], path)

		resp = testutil.MakeRequest(student, http.MethodGet, path, nil)
		require.Equal(t, http.StatusOK, resp.StatusCode, path)
		got = jsonMap(t, resp)
		assert.Equal(t, true, got["has_design"], path)
		assert.NotEmpty(t, got["scoped_css"], path)
		assert.NotContains(t, got, "design", path)
		assert.NotContains(t, got, "css_source", path)
	}
	resp = testutil.MakeRequest(anonymous, http.MethodGet, "/courses/100/p/home", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	got := jsonMap(t, resp)
	assert.Equal(t, true, got["has_design"])
	assert.NotContains(t, got, "design")
	assert.NotContains(t, got, "css_source")
}

// The audit repro through the API: a design's rich text, links and image
// addresses are cleaned before they are stored.
func TestPageDesign_DesignIsCleanedOnSave(t *testing.T) {
	f := newDesignFixture(t)
	teacher := f.app(8, 1, "TeacherEnrollment")
	doc := designDoc("doc-x")
	doc["sections"] = []any{
		map[string]any{"id": "w", "type": "welcome", "heading": "Hi",
			"body": `<p>Hello</p><img src=x onerror=alert(1)><iframe srcdoc="<script>alert(1)</script>"></iframe>`},
		map[string]any{"id": "q", "type": "quick-links", "heading": "Go", "links": []any{
			map[string]any{"id": "l", "label": "Bad", "href": "javascript:alert(1)", "icon": "x"}}},
		map[string]any{"id": "b", "type": "banner", "customSrc": "data:image/png;base64,AAAA", "alt": ""},
		map[string]any{"id": "z", "type": "marquee", "text": "unknown"},
	}
	resp := testutil.MakeRequest(teacher, http.MethodPost, "/courses/100/pages", pageBody(map[string]any{
		"title": "Clean", "published": true, "design": doc,
	}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var stored string
	require.NoError(t, f.g.Raw(`SELECT design::text FROM wiki_pages WHERE url = 'clean'`).Scan(&stored).Error)
	for _, bad := range []string{"<img", "onerror", "<iframe", "srcdoc", "javascript:", "data:", "marquee"} {
		assert.NotContains(t, stored, bad)
	}
	assert.Contains(t, stored, "<p>Hello</p>")
}

func TestPageDesign_Validation(t *testing.T) {
	f := newDesignFixture(t)
	teacher := f.app(8, 1, "TeacherEnrollment")

	for name, design := range map[string]any{
		"array":       []any{1},
		"no version":  map[string]any{"id": "x"},
		"version 2":   map[string]any{"version": 2, "id": "x"},
		"bad id":      map[string]any{"version": 1, "id": "x y"},
		"numeric id":  map[string]any{"version": 1, "id": 5},
		"string body": "not a doc",
		"too large":   map[string]any{"version": 1, "id": "x", "pad": strings.Repeat("a", 256*1024)},
	} {
		resp := testutil.MakeRequest(teacher, http.MethodPost, "/courses/100/pages",
			pageBody(map[string]any{"title": "Bad " + name, "design": design}))
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, name)
	}

	resp := testutil.MakeRequest(teacher, http.MethodPost, "/courses/100/pages",
		pageBody(map[string]any{"title": "Huge CSS", "css": strings.Repeat("a", csssanitize.MaxBytes+1)}))
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	var n int64
	require.NoError(t, f.g.Model(&models.WikiPage{}).Count(&n).Error)
	assert.Zero(t, n, "rejected pages must not be stored")
}

func TestPageDesign_StudentEditorsCannotSetStyleOrDesign(t *testing.T) {
	f := newDesignFixture(t)
	teacher := f.app(8, 1, "TeacherEnrollment")
	student := f.app(9, 1, "StudentEnrollment")

	resp := testutil.MakeRequest(teacher, http.MethodPost, "/courses/100/pages", pageBody(map[string]any{
		"title": "Class Notes", "published": true, "editing_roles": "teachers,students", "design": designDoc("doc-n"),
	}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	for name, fields := range map[string]map[string]any{
		"design":       {"design": designDoc("doc-s")},
		"clear design": {"design": nil},
		"css":          {"css": "p{color:red}"},
		"empty css":    {"css": ""},
		"with body":    {"body": "<p>x</p>", "css": "p{color:red}"},
	} {
		resp := testutil.MakeRequest(student, http.MethodPut, "/courses/100/pages/class-notes", pageBody(fields))
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, name)
	}
	// Content edits are still fine, and the design is untouched (the
	// student sees only that the page is designed).
	resp = testutil.MakeRequest(student, http.MethodPut, "/courses/100/pages/class-notes", pageBody(map[string]any{"body": "<p>notes</p>"}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	got := jsonMap(t, resp)
	assert.Equal(t, true, got["has_design"])
	assert.NotContains(t, got, "design")
	var docID string
	require.NoError(t, f.g.Raw(`SELECT design->>'id' FROM wiki_pages WHERE url = 'class-notes'`).Scan(&docID).Error)
	assert.Equal(t, "doc-n", docID)

	// A student editor may read the page's history, but not the design.
	resp = testutil.MakeRequest(student, http.MethodGet, "/courses/100/pages/class-notes/revisions", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	revs, err := testutil.ParseJSONArray(resp)
	require.NoError(t, err)
	require.NotEmpty(t, revs)
	for _, r := range revs {
		assert.Equal(t, true, r["has_design"])
		assert.NotContains(t, r, "design")
		assert.NotContains(t, r, "css_source")
		assert.Contains(t, r, "body")
	}
}

func TestPageDesign_CustomCSSSetting(t *testing.T) {
	f := newDesignFixture(t)
	teacher := f.app(8, 1, "TeacherEnrollment")
	f.cssSetting = "false"

	// Hand-written CSS on an ordinary page is refused...
	resp := testutil.MakeRequest(teacher, http.MethodPost, "/courses/100/pages",
		pageBody(map[string]any{"title": "Plain", "css": "p{color:red}"}))
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	resp = testutil.MakeRequest(teacher, http.MethodPost, "/courses/100/pages",
		pageBody(map[string]any{"title": "Plain", "css": "p{color:red}", "design": nil}))
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	assert.Contains(t, f.lookupAccts, uint(1), "the setting is resolved for the course's account")

	// ...but designer CSS travels with a design, and an empty css is fine.
	resp = testutil.MakeRequest(teacher, http.MethodPost, "/courses/100/pages",
		pageBody(map[string]any{"title": "Designed", "css": "p{color:red}", "design": designDoc("doc-d")}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp = testutil.MakeRequest(teacher, http.MethodPost, "/courses/100/pages",
		pageBody(map[string]any{"title": "Plain", "css": ""}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	// A css-only update of the designed page needs the design alongside.
	resp = testutil.MakeRequest(teacher, http.MethodPut, "/courses/100/pages/designed",
		pageBody(map[string]any{"css": "p{color:blue}"}))
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	// Detaching the design with the setting off takes the stylesheet (and
	// its source) with it: without a design the page may not carry CSS.
	require.NoError(t, f.g.Model(&models.WikiPage{}).Where("course_id = 100 AND url = 'designed'").
		Updates(map[string]any{"css": "p{color:green}", "css_source": "p { color: green }"}).Error)
	resp = testutil.MakeRequest(teacher, http.MethodPut, "/courses/100/pages/designed", pageBody(map[string]any{"design": nil}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	detached := jsonMap(t, resp)
	assert.Empty(t, detached["css"])
	assert.Empty(t, detached["scoped_css"])
	assert.Nil(t, detached["design"])
	var source string
	require.NoError(t, f.g.Raw("SELECT css_source FROM wiki_pages WHERE url = 'designed'").Scan(&source).Error)
	assert.Empty(t, source)

	// With the setting on, CSS alone is accepted, and a detach keeps it
	// ("Edit as HTML" keeps the page's look).
	f.cssSetting = "true"
	resp = testutil.MakeRequest(teacher, http.MethodPut, "/courses/100/pages/plain", pageBody(map[string]any{"css": "p{color:red}"}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp = testutil.MakeRequest(teacher, http.MethodPut, "/courses/100/pages/designed",
		pageBody(map[string]any{"css": "p{color:green}", "design": designDoc("doc-d")}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp = testutil.MakeRequest(teacher, http.MethodPut, "/courses/100/pages/designed", pageBody(map[string]any{"design": nil}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotEmpty(t, jsonMap(t, resp)["css"])
}

// The setting is the course's account's, not the caller's (an admin of
// another account sees the course's rule, as course show reports it).
func TestPageDesign_CustomCSSSettingFollowsCourseAccount(t *testing.T) {
	f := newDesignFixture(t)
	f.cssSetting = "false"
	admin := f.app(8, 7, "TeacherEnrollment") // caller's account 7, course 100's is 1
	resp := testutil.MakeRequest(admin, http.MethodPost, "/courses/100/pages",
		pageBody(map[string]any{"title": "Plain", "css": "p{color:red}"}))
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	assert.Contains(t, f.lookupAccts, uint(1))
	assert.NotContains(t, f.lookupAccts, uint(7))
}

// Restoring a revision with the setting off: a revision without a design
// comes back without its hand-written stylesheet; one with a design keeps
// the designer's CSS.
func TestPageDesign_RevisionRestoreHonoursCustomCSSSetting(t *testing.T) {
	f := newDesignFixture(t)
	teacher := f.app(8, 1, "TeacherEnrollment")
	resp := testutil.MakeRequest(teacher, http.MethodPost, "/courses/100/pages", pageBody(map[string]any{
		"title": "Notes", "published": true, "css": "p{color:red}",
	}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp = testutil.MakeRequest(teacher, http.MethodPut, "/courses/100/pages/notes",
		pageBody(map[string]any{"css": "p{color:blue}", "design": designDoc("doc-r")}))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	f.cssSetting = "false"
	resp = testutil.MakeRequest(teacher, http.MethodPost, "/courses/100/pages/notes/revisions/1", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var page models.WikiPage
	require.NoError(t, f.g.Where("url = 'notes'").First(&page).Error)
	assert.Empty(t, page.CSS, "hand-written CSS must not come back with the setting off")
	assert.Empty(t, page.CSSSource)
	assert.True(t, page.Design.IsNull())

	resp = testutil.MakeRequest(teacher, http.MethodPost, "/courses/100/pages/notes/revisions/2", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, f.g.Where("url = 'notes'").First(&page).Error)
	assert.NotEmpty(t, page.CSS, "designer CSS travels with its design")
	assert.False(t, page.Design.IsNull())
}

func TestPageDesign_CrossTenantPageIsNotFound(t *testing.T) {
	f := newDesignFixture(t)
	page := &models.WikiPage{CourseID: 200, Title: "Other", Body: "<p>x</p>", WorkflowState: "active", EditingRoles: "teachers",
		Design: models.NullJSON(`{"version":1,"id":"other"}`)}
	require.NoError(t, postgres.NewPageRepository(f.g).Create(context.Background(), page))

	teacher := f.app(8, 1, "TeacherEnrollment")
	path := fmt.Sprintf("/courses/200/pages/%d", page.ID)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(teacher, http.MethodGet, path, nil).StatusCode)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(teacher, http.MethodPut, path,
		pageBody(map[string]any{"design": nil})).StatusCode)

	var stored models.WikiPage
	require.NoError(t, f.g.First(&stored, page.ID).Error)
	assert.False(t, stored.Design.IsNull(), "a cross-tenant request must not clear the design")
}

func TestCourseLook(t *testing.T) {
	f := newDesignFixture(t)
	att := func(course uint) uint {
		a := &models.Attachment{ContextType: "Course", ContextID: course, UserID: 8, DisplayName: "banner.webp",
			Filename: "banner.webp", ContentType: "image/webp", Size: 10, StoragePath: fmt.Sprintf("x/%d", course)}
		require.NoError(t, f.g.Create(a).Error)
		return a.ID
	}
	own, sibling, foreign := att(100), att(101), att(200)

	teacher := f.app(8, 1, "TeacherEnrollment")
	student := f.app(9, 1, "StudentEnrollment")
	look := func(src string) map[string]any {
		return map[string]any{
			"version": 1, "theme_id": "tide-pool", "palette_id": "default",
			"colors":      map[string]any{"primary": "#12355b", "on_primary": "#ffffff", "link": "#0b5cad"},
			"dark_colors": map[string]any{"background": "#0d1117", "text": "#e6edf3"},
			"banner":      map[string]any{"src": src, "alt": ""},
		}
	}
	courseBody := func(v any) map[string]any { return map[string]any{"course": map[string]any{"look": v}} }

	// Show: no look yet, and the resolved setting.
	resp := testutil.MakeRequest(teacher, http.MethodGet, "/courses/100", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	shown := jsonMap(t, resp)
	assert.Nil(t, shown["look"])
	assert.Equal(t, true, shown["custom_css_enabled"])
	f.cssSetting = "false"
	resp = testutil.MakeRequest(teacher, http.MethodGet, "/courses/100", nil)
	assert.Equal(t, false, jsonMap(t, resp)["custom_css_enabled"])
	f.cssSetting = "true"

	resp = testutil.MakeRequest(teacher, http.MethodPut, "/courses/100", testutil.JSONBody(courseBody(look("/design-assets/themes/tide-pool/banner.webp"))))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	updated := jsonMap(t, resp)
	require.IsType(t, map[string]any{}, updated["look"])
	assert.Equal(t, "tide-pool", updated["look"].(map[string]any)["theme_id"])

	resp = testutil.MakeRequest(teacher, http.MethodPut, "/courses/100", testutil.JSONBody(courseBody(look(fmt.Sprintf("/api/v1/files/%d/download", own)))))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	for name, v := range map[string]any{
		"sibling course file": look(fmt.Sprintf("/api/v1/files/%d/download", sibling)),
		"other tenant file":   look(fmt.Sprintf("/api/v1/files/%d/download", foreign)),
		"missing file":        look("/api/v1/files/999999/download"),
		"external banner":     look("https://evil.example/banner.webp"),
		"bad theme":           map[string]any{"theme_id": "Tide Pool"},
		"bad colour":          map[string]any{"theme_id": "x", "colors": map[string]any{"primary": "red"}},
		"too large":           map[string]any{"theme_id": "x", "banner": map[string]any{"src": "/design-assets/a.webp", "alt": strings.Repeat("a", 17*1024)}},
		"not an object":       "tide-pool",
	} {
		resp := testutil.MakeRequest(teacher, http.MethodPut, "/courses/100", testutil.JSONBody(courseBody(v)))
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, name)
	}

	// Only course staff change the look.
	resp = testutil.MakeRequest(student, http.MethodPut, "/courses/100", testutil.JSONBody(courseBody(nil)))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	// The file banner survived the refused updates; null clears it.
	resp = testutil.MakeRequest(teacher, http.MethodGet, "/courses/100", nil)
	banner := jsonMap(t, resp)["look"].(map[string]any)["banner"].(map[string]any)
	assert.Equal(t, fmt.Sprintf("/api/v1/files/%d/download", own), banner["src"])
	resp = testutil.MakeRequest(teacher, http.MethodPut, "/courses/100", testutil.JSONBody(courseBody(nil)))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Nil(t, jsonMap(t, resp)["look"])
	var nulls int64
	require.NoError(t, f.g.Raw(`SELECT count(*) FROM courses WHERE id = 100 AND look IS NULL`).Scan(&nulls).Error)
	assert.EqualValues(t, 1, nulls)

	// Another tenant's course is a 404.
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(teacher, http.MethodPut, "/courses/200",
		testutil.JSONBody(courseBody(look("/design-assets/a.webp")))).StatusCode)
}

func testPalette(name string) map[string]any {
	return map[string]any{
		"id": "custom-1", "name": name, "primary": "#12355b", "onPrimary": "#ffffff",
		"secondary": "#2a6f97", "onSecondary": "#ffffff", "accent": "#f5c518", "surface": "#eef6fb",
		"onSurface": "#1f2328", "background": "#ffffff", "text": "#1f2328", "link": "#0b5cad",
	}
}

func TestDesignColourSchemes_CRUDAndIsolation(t *testing.T) {
	f := newDesignFixture(t)
	jane := f.app(8, 1, "")
	john := f.app(9, 1, "")
	admin2 := f.app(3, 2, "")

	resp := testutil.MakeRequest(jane, http.MethodPost, "/users/self/design_colour_schemes",
		testutil.JSONBody(map[string]any{"name": " Ocean ", "palette": testPalette("Ocean")}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	created := jsonMap(t, resp)
	assert.Equal(t, "Ocean", created["name"])
	id := created["id"]
	path := fmt.Sprintf("/users/self/design_colour_schemes/%v", id)

	resp = testutil.MakeRequest(jane, http.MethodGet, "/users/self/design_colour_schemes", nil)
	list, err := testutil.ParseJSONArray(resp)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "#12355b", list[0]["palette"].(map[string]any)["primary"])

	// Other users — same tenant or not — neither see nor touch it.
	for _, other := range []*fiber.App{john, admin2} {
		resp = testutil.MakeRequest(other, http.MethodGet, "/users/self/design_colour_schemes", nil)
		l, err := testutil.ParseJSONArray(resp)
		require.NoError(t, err)
		assert.Empty(t, l)
		assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(other, http.MethodPut, path,
			testutil.JSONBody(map[string]any{"name": "Mine now"})).StatusCode)
		assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(other, http.MethodDelete, path, nil).StatusCode)
	}

	resp = testutil.MakeRequest(jane, http.MethodPut, path, testutil.JSONBody(map[string]any{"name": "Deep Ocean"}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "Deep Ocean", jsonMap(t, resp)["name"])

	// Validation.
	bad := testPalette("x")
	bad["accent"] = "javascript:alert(1)"
	for name, body := range map[string]map[string]any{
		"bad colour":    {"name": "x", "palette": bad},
		"no palette":    {"name": "x"},
		"no name":       {"palette": testPalette("x")},
		"long name":     {"name": strings.Repeat("n", 61), "palette": testPalette("x")},
		"blank name":    {"name": "   ", "palette": testPalette("x")},
		"palette array": {"name": "x", "palette": []any{}},
	} {
		resp := testutil.MakeRequest(jane, http.MethodPost, "/users/self/design_colour_schemes", testutil.JSONBody(body))
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, name)
	}

	// At most 50 per user.
	for i := 1; i < handlers.MaxDesignColourSchemes; i++ {
		resp := testutil.MakeRequest(jane, http.MethodPost, "/users/self/design_colour_schemes",
			testutil.JSONBody(map[string]any{"name": fmt.Sprintf("Scheme %d", i), "palette": testPalette("p")}))
		require.Equal(t, http.StatusCreated, resp.StatusCode)
	}
	resp = testutil.MakeRequest(jane, http.MethodPost, "/users/self/design_colour_schemes",
		testutil.JSONBody(map[string]any{"name": "One too many", "palette": testPalette("p")}))
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	// Another user still has room.
	resp = testutil.MakeRequest(john, http.MethodPost, "/users/self/design_colour_schemes",
		testutil.JSONBody(map[string]any{"name": "Forest", "palette": testPalette("Forest")}))
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	assert.Equal(t, http.StatusOK, testutil.MakeRequest(jane, http.MethodDelete, path, nil).StatusCode)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(jane, http.MethodDelete, path, nil).StatusCode)
}

func TestPagesCustomCSSKeyMatchesCatalog(t *testing.T) {
	def, ok := settings.Find(service.PagesCustomCSSEnabledKey)
	require.True(t, ok)
	assert.Equal(t, settings.PagesCustomCSSEnabled, def.Key)
	assert.Equal(t, settings.TypeBool, def.ValueType)
	assert.Equal(t, "true", def.Default)
	assert.True(t, def.AllowsScope(settings.ScopeAccount))
	assert.True(t, def.AllowsScope(settings.ScopeInstance))
}
