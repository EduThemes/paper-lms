package handlers_test

// DB-backed tests for the learner-dashboard lane (planner, dashboard
// customization, missing work, activity stream, grade summary, observer
// alerts, profile). Each test runs on its own freshly migrated scratch
// database and skips without PARITY_DB_URL / DATABASE_URL.

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pgdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/db"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/storage"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

func learnerScratchDB(t *testing.T) *gorm.DB {
	t.Helper()
	base := os.Getenv("PARITY_DB_URL")
	if base == "" {
		base = os.Getenv("DATABASE_URL")
	}
	if base == "" {
		t.Skip("set PARITY_DB_URL (or DATABASE_URL) to run DB-backed handler tests")
	}
	swap := func(name string) string {
		u, err := url.Parse(base)
		require.NoError(t, err)
		u.Path = "/" + name
		return u.String()
	}
	admin, err := sql.Open("postgres", swap("postgres"))
	require.NoError(t, err)
	name := fmt.Sprintf("paper_lms_learner_%d", time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err = admin.ExecContext(ctx, fmt.Sprintf(`CREATE DATABASE %q`, name))
	require.NoError(t, err)
	bs, err := sql.Open("postgres", swap(name))
	require.NoError(t, err)
	_, err = bs.Exec(`CREATE EXTENSION IF NOT EXISTS vector`)
	_ = bs.Close()
	require.NoError(t, err)
	g, err := gorm.Open(pgdriver.Open(swap(name)), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.MigrateUp(g))
	t.Cleanup(func() {
		if raw, err := g.DB(); err == nil {
			_ = raw.Close()
		}
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer dropCancel()
		_, _ = admin.ExecContext(dropCtx, fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, name))
		_ = admin.Close()
	})
	return g
}

// Fixture ids (made-up data).
const (
	lnStudent   = uint(11)
	lnClassmate = uint(12)
	lnTeacher   = uint(13)
	lnParent    = uint(14)
	lnStranger  = uint(15)
	lnOtherTen  = uint(16)
	lnCourse    = uint(300)
	lnCourse2   = uint(301)
	lnOtherCrs  = uint(302)
)

type learnerFixture struct {
	t     *testing.T
	g     *gorm.DB
	app   *fiber.App
	store storage.Backend
	obs   *service.ObserverService
	alert *service.ObserverAlertService
}

func newLearnerFixture(t *testing.T) *learnerFixture {
	t.Helper()
	g := learnerScratchDB(t)
	ctx := context.Background()
	for _, id := range []uint{1, 2} {
		require.NoError(t, g.Exec(
			`INSERT INTO accounts (id, name, workflow_state, mfa_policy, default_locale, tenant_mode, max_upload_size_mb)
			 VALUES (?, ?, 'active', 'off', 'en', 'higher_ed', 500) ON CONFLICT (id) DO NOTHING`,
			id, fmt.Sprintf("District %d", id)).Error)
	}
	users := postgres.NewUserRepository(g)
	for _, u := range []*models.User{
		{ID: lnStudent, AccountID: 1, Name: "Jane Doe", LoginID: "jane@example.test", Email: "jane@example.test", PasswordHash: "x", Role: "user"},
		{ID: lnClassmate, AccountID: 1, Name: "John Roe", LoginID: "john@example.test", Email: "john@example.test", PasswordHash: "x", Role: "user"},
		{ID: lnTeacher, AccountID: 1, Name: "Terry Teacher", LoginID: "terry@example.test", Email: "terry@example.test", PasswordHash: "x", Role: "user"},
		{ID: lnParent, AccountID: 1, Name: "Pat Parent", LoginID: "pat@example.test", Email: "pat@example.test", PasswordHash: "x", Role: "user"},
		{ID: lnStranger, AccountID: 1, Name: "Sam Stranger", LoginID: "sam@example.test", Email: "sam@example.test", PasswordHash: "x", Role: "user"},
		{ID: lnOtherTen, AccountID: 2, Name: "Olly Other", LoginID: "olly@example.test", Email: "olly@example.test", PasswordHash: "x", Role: "user"},
	} {
		require.NoError(t, users.Create(ctx, u))
	}
	for _, c := range []*models.Course{
		{ID: lnCourse, AccountID: 1, Name: "Biology", CourseCode: "BIO", WorkflowState: models.CourseAvailable},
		{ID: lnCourse2, AccountID: 1, Name: "Chemistry", CourseCode: "CHEM", WorkflowState: models.CourseAvailable},
		{ID: lnOtherCrs, AccountID: 2, Name: "History", CourseCode: "HIST", WorkflowState: models.CourseAvailable},
	} {
		require.NoError(t, g.Create(c).Error)
	}
	for _, e := range []*models.Enrollment{
		{UserID: lnStudent, CourseID: lnCourse, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: models.EnrollmentActive},
		{UserID: lnStudent, CourseID: lnCourse2, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: models.EnrollmentActive},
		{UserID: lnClassmate, CourseID: lnCourse, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: models.EnrollmentActive},
		{UserID: lnTeacher, CourseID: lnCourse, Type: "TeacherEnrollment", Role: "TeacherEnrollment", WorkflowState: models.EnrollmentActive},
		{UserID: lnOtherTen, CourseID: lnOtherCrs, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: models.EnrollmentActive},
	} {
		require.NoError(t, g.Create(e).Error)
	}

	enrollmentRepo := postgres.NewEnrollmentRepository(g)
	courseRepo := postgres.NewCourseRepository(g)
	learner := postgres.NewLearnerRepository(g)
	obs := service.NewObserverService(enrollmentRepo, courseRepo, users).SetObserverLinkStore(postgres.NewObserverLinkRepository(g))
	require.NoError(t, obs.LinkObserverToStudent(ctx, lnParent, lnStudent))

	planner := service.NewPlannerService(postgres.NewPlannerNoteRepository(g), postgres.NewPlannerOverrideRepository(g),
		enrollmentRepo, postgres.NewAssignmentRepository(g), postgres.NewQuizRepository(g), postgres.NewCalendarEventRepository(g)).
		SetLearnerRepository(learner)
	dash := service.NewLearnerDashboardService(learner, enrollmentRepo)
	grades := service.NewGradeSummaryService(learner, enrollmentRepo, postgres.NewGradingStandardRepository(g))
	alerts := service.NewObserverAlertService(learner, obs, dash, grades)
	store := storage.NewLocalBackend(t.TempDir())
	profiles := service.NewUserProfileService(users, learner, store)
	h := handlers.NewLearnerHandler(planner, dash, grades, alerts, profiles, obs, users)

	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		if v := c.Get("X-Test-User"); v != "" {
			n, _ := strconv.Atoi(v)
			c.Locals("user_id", uint(n))
		}
		if v := c.Get("X-Test-Account"); v != "" {
			n, _ := strconv.Atoi(v)
			c.Locals("account_id", uint(n))
		}
		if v := c.Get("X-Test-Enrollment"); v != "" {
			c.Locals("enrollment_type", v)
		}
		return c.Next()
	})
	app.Get("/planner/items", h.PlannerItems)
	app.Post("/planner_notes", h.CreatePlannerNote)
	app.Put("/planner_notes/:id", h.UpdatePlannerNote)
	app.Delete("/planner_notes/:id", h.DeletePlannerNote)
	app.Post("/planner/overrides", h.CreatePlannerOverride)
	app.Put("/planner/overrides/:id", h.UpdatePlannerOverride)
	app.Get("/dashboard/dashboard_cards", h.DashboardCards)
	app.Post("/users/self/favorites/courses/:course_id", h.AddFavoriteCourse)
	app.Put("/users/self/course_nicknames/:course_id", h.SetCourseNickname)
	app.Put("/users/self/colors/:asset_string", h.SetCustomColor)
	app.Get("/users/self/colors", h.GetCustomColors)
	app.Get("/users/self/activity_stream", h.ActivityStream)
	app.Get("/users/:user_id/missing_submissions", h.MissingSubmissions)
	app.Get("/courses/:course_id/grade_summary", h.GradeSummary)
	app.Post("/observer_alert_thresholds", h.CreateAlertThreshold)
	app.Get("/users/self/observer_alerts", h.ListObserverAlerts)
	app.Get("/users/self/observer_alerts/unread_count", h.ObserverAlertsUnreadCount)
	app.Put("/observer_alerts/:id/:workflow_state", h.UpdateObserverAlert)
	app.Put("/users/self/profile", h.UpdateProfile)
	app.Post("/users/self/avatar", h.UploadAvatar)
	app.Get("/users/:id/profile_card", h.GetProfileCard)
	app.Get("/users/:id/avatar", h.GetAvatar)
	return &learnerFixture{t: t, g: g, app: app, store: store, obs: obs, alert: alerts}
}

func (f *learnerFixture) do(method, path string, user, account uint, body any, enrollment string) (*http.Response, []byte) {
	f.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User", strconv.Itoa(int(user)))
	req.Header.Set("X-Test-Account", strconv.Itoa(int(account)))
	if enrollment != "" {
		req.Header.Set("X-Test-Enrollment", enrollment)
	}
	resp, err := f.app.Test(req, -1)
	require.NoError(f.t, err)
	data, _ := io.ReadAll(resp.Body)
	return resp, data
}

func (f *learnerFixture) assignment(courseID uint, name string, due *time.Time, points float64, groupID *uint) *models.Assignment {
	a := &models.Assignment{CourseID: courseID, Name: name, WorkflowState: models.AssignmentPublished, Published: true,
		DueAt: due, PointsPossible: &points, AssignmentGroupID: groupID, SubmissionTypes: "online_text_entry", GradingType: "points"}
	require.NoError(f.t, f.g.Create(a).Error)
	return a
}

func (f *learnerFixture) grade(assignmentID, userID uint, score float64, posted bool) *models.Submission {
	now := time.Now()
	sub := &models.Submission{AssignmentID: assignmentID, UserID: userID, Score: &score, GradedAt: &now,
		SubmittedAt: &now, WorkflowState: models.SubmissionGraded}
	if posted {
		sub.PostedAt = &now
	}
	require.NoError(f.t, f.g.Create(sub).Error)
	return sub
}

func ptrTime(t time.Time) *time.Time { return &t }

func decodeList(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	var out []map[string]any
	require.NoError(t, json.Unmarshal(data, &out), string(data))
	return out
}

func decodeMap(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out), string(data))
	return out
}

// G144: the planner lists the student's assignments (with the effective
// due date from a student override), notes, and honors mark-as-done.
func TestLearnerPlanner_ItemsNotesAndOverrides(t *testing.T) {
	f := newLearnerFixture(t)
	now := time.Now()
	essay := f.assignment(lnCourse, "Essay", ptrTime(now.Add(48*time.Hour)), 10, nil)
	moved := f.assignment(lnCourse2, "Lab report", ptrTime(now.Add(-30*24*time.Hour)), 10, nil)
	// Moved into range for this student only.
	ov := &models.AssignmentOverride{AssignmentID: moved.ID, Title: "Extension", DueAt: ptrTime(now.Add(72 * time.Hour)), WorkflowState: "active"}
	require.NoError(t, f.g.Create(ov).Error)
	require.NoError(t, f.g.Create(&models.AssignmentOverrideStudent{AssignmentOverrideID: ov.ID, UserID: lnStudent, AssignmentID: moved.ID}).Error)
	// Another tenant's assignment never shows.
	f.assignment(lnOtherCrs, "Foreign", ptrTime(now.Add(24*time.Hour)), 10, nil)

	resp, data := f.do("POST", "/planner_notes", lnStudent, 1, map[string]any{
		"title": "Study <b>group</b>", "todo_date": now.Add(24 * time.Hour).Format(time.RFC3339), "course_id": lnCourse,
	}, "")
	require.Equal(t, 201, resp.StatusCode, string(data))
	note := decodeMap(t, data)
	assert.Equal(t, "Study group", note["title"], "plain-text sanitized, not entity-encoded")

	// A note in a course the student is not in is a 404.
	resp, _ = f.do("POST", "/planner_notes", lnStudent, 1, map[string]any{"title": "x", "course_id": lnOtherCrs}, "")
	assert.Equal(t, 404, resp.StatusCode)

	q := fmt.Sprintf("/planner/items?start_date=%s&end_date=%s", url.QueryEscape(now.Format(time.RFC3339)), url.QueryEscape(now.Add(7*24*time.Hour).Format(time.RFC3339)))
	resp, data = f.do("GET", q, lnStudent, 1, nil, "")
	require.Equal(t, 200, resp.StatusCode, string(data))
	items := decodeList(t, data)
	byID := map[string]map[string]any{}
	for _, it := range items {
		byID[fmt.Sprintf("%s:%v", it["plannable_type"], it["plannable_id"])] = it
	}
	require.Contains(t, byID, fmt.Sprintf("assignment:%d", essay.ID))
	require.Contains(t, byID, fmt.Sprintf("assignment:%d", moved.ID), "student override moves the due date into range")
	require.Contains(t, byID, fmt.Sprintf("planner_note:%v", note["id"]))
	assert.Len(t, items, 3)

	// Mark the essay done; filter=incomplete_items drops it.
	resp, data = f.do("POST", "/planner/overrides", lnStudent, 1, map[string]any{
		"plannable_type": "assignment", "plannable_id": essay.ID, "marked_complete": true,
	}, "")
	require.Equal(t, 201, resp.StatusCode, string(data))
	resp, data = f.do("GET", q+"&filter=incomplete_items", lnStudent, 1, nil, "")
	require.Equal(t, 200, resp.StatusCode)
	for _, it := range decodeList(t, data) {
		if it["plannable_type"] == "assignment" {
			assert.NotEqual(t, float64(essay.ID), it["plannable_id"], "completed item filtered out")
		}
	}

	// Another user can't touch the note (404, not 403).
	noteID := uint(note["id"].(float64))
	resp, _ = f.do("PUT", fmt.Sprintf("/planner_notes/%d", noteID), lnClassmate, 1, map[string]any{"title": "hijack"}, "")
	assert.Equal(t, 404, resp.StatusCode)
	resp, _ = f.do("DELETE", fmt.Sprintf("/planner_notes/%d", noteID), lnClassmate, 1, nil, "")
	assert.Equal(t, 404, resp.StatusCode)
	resp, _ = f.do("DELETE", fmt.Sprintf("/planner_notes/%d", noteID), lnStudent, 1, nil, "")
	assert.Equal(t, 200, resp.StatusCode)

	// Unsupported plannable types are rejected.
	resp, _ = f.do("POST", "/planner/overrides", lnStudent, 1, map[string]any{"plannable_type": "users", "plannable_id": 1}, "")
	assert.Equal(t, 400, resp.StatusCode)
}

// G144: work in an unpublished course is not on a student's planner.
func TestLearnerPlanner_UnpublishedCourseHidden(t *testing.T) {
	f := newLearnerFixture(t)
	now := time.Now()
	require.NoError(t, f.g.Model(&models.Course{}).Where("id = ?", lnCourse2).Update("workflow_state", models.CourseUnpublished).Error)
	hidden := f.assignment(lnCourse2, "Draft lab", ptrTime(now.Add(24*time.Hour)), 10, nil)
	shown := f.assignment(lnCourse, "Essay", ptrTime(now.Add(24*time.Hour)), 10, nil)
	resp, data := f.do("GET", "/planner/items", lnStudent, 1, nil, "")
	require.Equal(t, 200, resp.StatusCode, string(data))
	ids := map[float64]bool{}
	for _, it := range decodeList(t, data) {
		ids[it["plannable_id"].(float64)] = true
	}
	assert.True(t, ids[float64(shown.ID)])
	assert.False(t, ids[float64(hidden.ID)])
}

// G145: favorites, nicknames and colors are per user, only for enrolled
// courses, and the dashboard shows favorites once any exist.
func TestLearnerDashboard_FavoritesNicknamesColors(t *testing.T) {
	f := newLearnerFixture(t)
	resp, data := f.do("GET", "/dashboard/dashboard_cards", lnStudent, 1, nil, "")
	require.Equal(t, 200, resp.StatusCode, string(data))
	assert.Len(t, decodeList(t, data), 2)

	resp, _ = f.do("POST", fmt.Sprintf("/users/self/favorites/courses/%d", lnCourse2), lnStudent, 1, nil, "")
	require.Equal(t, 200, resp.StatusCode)
	resp, data = f.do("PUT", fmt.Sprintf("/users/self/course_nicknames/%d", lnCourse2), lnStudent, 1, map[string]any{"nickname": "Chem <i>fun</i>"}, "")
	require.Equal(t, 200, resp.StatusCode, string(data))
	resp, _ = f.do("PUT", fmt.Sprintf("/users/self/colors/course_%d", lnCourse2), lnStudent, 1, map[string]any{"hexcode": "1A2B3C"}, "")
	require.Equal(t, 200, resp.StatusCode)

	resp, data = f.do("GET", "/dashboard/dashboard_cards", lnStudent, 1, nil, "")
	require.Equal(t, 200, resp.StatusCode)
	cards := decodeList(t, data)
	require.Len(t, cards, 1, "favorites only once one exists")
	assert.Equal(t, "Chem fun", cards[0]["shortName"])
	assert.Equal(t, "Chemistry", cards[0]["originalName"])
	assert.Equal(t, "#1a2b3c", cards[0]["color"])
	resp, data = f.do("GET", "/dashboard/dashboard_cards?all=true", lnStudent, 1, nil, "")
	require.Equal(t, 200, resp.StatusCode)
	assert.Len(t, decodeList(t, data), 2)

	// The classmate's dashboard is untouched.
	_, data = f.do("GET", "/dashboard/dashboard_cards", lnClassmate, 1, nil, "")
	cards = decodeList(t, data)
	require.Len(t, cards, 1)
	assert.Nil(t, cards[0]["color"])

	// Not enrolled / other tenant: 404. Bad color: 400.
	resp, _ = f.do("POST", fmt.Sprintf("/users/self/favorites/courses/%d", lnOtherCrs), lnStudent, 1, nil, "")
	assert.Equal(t, 404, resp.StatusCode)
	resp, _ = f.do("PUT", fmt.Sprintf("/users/self/colors/course_%d", lnCourse), lnStudent, 1, map[string]any{"hexcode": "red; x"}, "")
	assert.Equal(t, 400, resp.StatusCode)
	_, data = f.do("GET", "/users/self/colors", lnStudent, 1, nil, "")
	assert.Equal(t, map[string]any{fmt.Sprintf("course_%d", lnCourse2): "#1a2b3c"}, decodeMap(t, data)["custom_colors"])
}

// G145: missing submissions for the student and their observer; activity
// stream shows posted grades only.
func TestLearnerMissingAndActivity(t *testing.T) {
	f := newLearnerFixture(t)
	now := time.Now()
	missing := f.assignment(lnCourse, "Overdue worksheet", ptrTime(now.Add(-48*time.Hour)), 10, nil)
	turnedIn := f.assignment(lnCourse, "Turned in", ptrTime(now.Add(-48*time.Hour)), 10, nil)
	f.assignment(lnCourse, "Not due yet", ptrTime(now.Add(48*time.Hour)), 10, nil)
	f.grade(turnedIn.ID, lnStudent, 9, true)
	hidden := f.assignment(lnCourse2, "Manual post", nil, 10, nil)
	f.grade(hidden.ID, lnStudent, 4, false)

	resp, data := f.do("GET", "/users/self/missing_submissions", lnStudent, 1, nil, "")
	require.Equal(t, 200, resp.StatusCode, string(data))
	list := decodeList(t, data)
	require.Len(t, list, 1)
	assert.Equal(t, float64(missing.ID), list[0]["id"])

	// The parent sees the child's missing work; a stranger gets 404.
	resp, data = f.do("GET", fmt.Sprintf("/users/%d/missing_submissions", lnStudent), lnParent, 1, nil, "")
	require.Equal(t, 200, resp.StatusCode, string(data))
	assert.Len(t, decodeList(t, data), 1)
	resp, _ = f.do("GET", fmt.Sprintf("/users/%d/missing_submissions", lnStudent), lnStranger, 1, nil, "")
	assert.Equal(t, 404, resp.StatusCode)

	resp, data = f.do("GET", "/users/self/activity_stream", lnStudent, 1, nil, "")
	require.Equal(t, 200, resp.StatusCode, string(data))
	stream := decodeList(t, data)
	var titles []string
	for _, it := range stream {
		titles = append(titles, it["title"].(string))
	}
	assert.Contains(t, titles, "Turned in")
	assert.NotContains(t, titles, "Manual post", "unposted grade never streams")
}

// G147: the grade summary drops the lowest score, withholds unposted
// grades, honors hide-totals for students, and lets only the student, their
// observer and staff read it.
func TestLearnerGradeSummary(t *testing.T) {
	f := newLearnerFixture(t)
	grp := &models.AssignmentGroup{CourseID: lnCourse, Name: "Quizzes", Rules: `{"drop_lowest": 1}`, WorkflowState: "available"}
	require.NoError(t, f.g.Create(grp).Error)
	a1 := f.assignment(lnCourse, "Q1", nil, 10, &grp.ID)
	a2 := f.assignment(lnCourse, "Q2", nil, 10, &grp.ID)
	a3 := f.assignment(lnCourse, "Q3", nil, 10, &grp.ID)
	unposted := f.assignment(lnCourse, "Q4", nil, 10, &grp.ID)
	f.grade(a1.ID, lnStudent, 10, true)
	f.grade(a2.ID, lnStudent, 8, true)
	f.grade(a3.ID, lnStudent, 2, true)
	f.grade(unposted.ID, lnStudent, 0, false)

	path := fmt.Sprintf("/courses/%d/grade_summary", lnCourse)
	resp, data := f.do("GET", path, lnStudent, 1, nil, "StudentEnrollment")
	require.Equal(t, 200, resp.StatusCode, string(data))
	sum := decodeMap(t, data)
	totals := sum["totals"].(map[string]any)
	assert.Equal(t, 90.0, totals["current_score"], "Q3 dropped, Q4 unposted: (10+8)/20")
	byName := map[string]map[string]any{}
	for _, a := range sum["assignments"].([]any) {
		m := a.(map[string]any)
		byName[m["name"].(string)] = m
	}
	assert.Equal(t, true, byName["Q3"]["dropped"])
	assert.Nil(t, byName["Q4"]["score"], "unposted score withheld")
	assert.Equal(t, "not_posted", byName["Q4"]["status"])

	// Observer reads the child's; stranger and classmate cannot.
	resp, _ = f.do("GET", fmt.Sprintf("%s?student_id=%d", path, lnStudent), lnParent, 1, nil, "ObserverEnrollment")
	assert.Equal(t, 200, resp.StatusCode)
	resp, _ = f.do("GET", fmt.Sprintf("%s?student_id=%d", path, lnStudent), lnClassmate, 1, nil, "StudentEnrollment")
	assert.Equal(t, 404, resp.StatusCode)
	// Staff see unposted scores.
	resp, data = f.do("GET", fmt.Sprintf("%s?student_id=%d", path, lnStudent), lnTeacher, 1, nil, "TeacherEnrollment")
	require.Equal(t, 200, resp.StatusCode)
	for _, a := range decodeMap(t, data)["assignments"].([]any) {
		m := a.(map[string]any)
		if m["name"] == "Q4" {
			assert.Equal(t, 0.0, m["score"])
		}
	}

	// Hide totals from students (staff still see them).
	require.NoError(t, f.g.Model(&models.Course{}).Where("id = ?", lnCourse).Update("hide_final_grades", true).Error)
	_, data = f.do("GET", path, lnStudent, 1, nil, "StudentEnrollment")
	sum = decodeMap(t, data)
	assert.Nil(t, sum["totals"])
	assert.Equal(t, true, sum["hide_totals"])
	_, data = f.do("GET", fmt.Sprintf("%s?student_id=%d", path, lnStudent), lnTeacher, 1, nil, "TeacherEnrollment")
	assert.NotNil(t, decodeMap(t, data)["totals"])
}

// G147: grading periods filter the summary by (effective) due date.
func TestLearnerGradeSummary_GradingPeriods(t *testing.T) {
	f := newLearnerFixture(t)
	now := time.Now()
	gpg := &models.GradingPeriodGroup{AccountID: 1, Title: "Year", WorkflowState: "active"}
	require.NoError(t, f.g.Create(gpg).Error)
	p1 := &models.GradingPeriod{GradingPeriodGroupID: gpg.ID, Title: "Q1", StartDate: now.AddDate(0, -2, 0), EndDate: now.AddDate(0, -1, 0), WorkflowState: "active"}
	p2 := &models.GradingPeriod{GradingPeriodGroupID: gpg.ID, Title: "Q2", StartDate: now.AddDate(0, -1, 0).Add(time.Second), EndDate: now.AddDate(0, 1, 0), WorkflowState: "active"}
	require.NoError(t, f.g.Create(p1).Error)
	require.NoError(t, f.g.Create(p2).Error)
	term := &models.EnrollmentTerm{AccountID: 1, Name: "2026", GradingPeriodGroupID: &gpg.ID, WorkflowState: "active"}
	require.NoError(t, f.g.Create(term).Error)
	require.NoError(t, f.g.Model(&models.Course{}).Where("id = ?", lnCourse).Update("enrollment_term_id", term.ID).Error)
	old := f.assignment(lnCourse, "Old", ptrTime(now.AddDate(0, 0, -45)), 10, nil)
	cur := f.assignment(lnCourse, "Current", ptrTime(now.AddDate(0, 0, -1)), 10, nil)
	f.grade(old.ID, lnStudent, 2, true)
	f.grade(cur.ID, lnStudent, 10, true)

	path := fmt.Sprintf("/courses/%d/grade_summary", lnCourse)
	_, data := f.do("GET", path, lnStudent, 1, nil, "StudentEnrollment")
	sum := decodeMap(t, data)
	assert.Equal(t, float64(p2.ID), sum["grading_period_id"], "defaults to the current period")
	assert.Len(t, sum["assignments"], 1)
	assert.Equal(t, 100.0, sum["totals"].(map[string]any)["current_score"])

	_, data = f.do("GET", fmt.Sprintf("%s?grading_period_id=%d", path, p1.ID), lnStudent, 1, nil, "StudentEnrollment")
	sum = decodeMap(t, data)
	assert.Equal(t, 20.0, sum["totals"].(map[string]any)["current_score"])
	resp, _ := f.do("GET", path+"?grading_period_id=999999", lnStudent, 1, nil, "StudentEnrollment")
	assert.Equal(t, 404, resp.StatusCode)
}

// G149: observer alert thresholds are limited to observed students and a
// low assignment grade raises one alert that can be read and dismissed.
func TestLearnerObserverAlerts(t *testing.T) {
	f := newLearnerFixture(t)
	resp, _ := f.do("POST", "/observer_alert_thresholds", lnParent, 1, map[string]any{
		"threshold": map[string]any{"user_id": lnClassmate, "alert_type": "assignment_grade_low", "threshold": 70},
	}, "")
	assert.Equal(t, 404, resp.StatusCode, "not their child")
	resp, _ = f.do("POST", "/observer_alert_thresholds", lnParent, 1, map[string]any{
		"threshold": map[string]any{"user_id": lnStudent, "alert_type": "assignment_grade_low"},
	}, "")
	assert.Equal(t, 400, resp.StatusCode, "grade alert needs a threshold")
	resp, data := f.do("POST", "/observer_alert_thresholds", lnParent, 1, map[string]any{
		"threshold": map[string]any{"user_id": lnStudent, "alert_type": "assignment_grade_low", "threshold": 70},
	}, "")
	require.Equal(t, 201, resp.StatusCode, string(data))

	// Grades posted after opting in: one low, one fine.
	time.Sleep(10 * time.Millisecond)
	low := f.assignment(lnCourse, "Unit test", nil, 10, nil)
	ok := f.assignment(lnCourse, "Homework", nil, 10, nil)
	f.grade(low.ID, lnStudent, 5, true)
	f.grade(ok.ID, lnStudent, 9, true)
	require.NoError(t, f.alert.EvaluateAll(context.Background()))
	require.NoError(t, f.alert.EvaluateAll(context.Background()), "rescans never duplicate")

	resp, data = f.do("GET", "/users/self/observer_alerts", lnParent, 1, nil, "")
	require.Equal(t, 200, resp.StatusCode, string(data))
	alerts := decodeList(t, data)
	require.Len(t, alerts, 1)
	assert.Equal(t, "assignment_grade_low", alerts[0]["alert_type"])
	_, data = f.do("GET", "/users/self/observer_alerts/unread_count", lnParent, 1, nil, "")
	assert.Equal(t, 1.0, decodeMap(t, data)["unread_count"])

	id := uint(alerts[0]["id"].(float64))
	resp, _ = f.do("PUT", fmt.Sprintf("/observer_alerts/%d/read", id), lnStranger, 1, nil, "")
	assert.Equal(t, 404, resp.StatusCode)
	resp, _ = f.do("PUT", fmt.Sprintf("/observer_alerts/%d/unread", id), lnParent, 1, nil, "")
	assert.Equal(t, 400, resp.StatusCode)
	resp, _ = f.do("PUT", fmt.Sprintf("/observer_alerts/%d/dismissed", id), lnParent, 1, nil, "")
	assert.Equal(t, 200, resp.StatusCode)
	resp, _ = f.do("PUT", fmt.Sprintf("/observer_alerts/%d/read", id), lnParent, 1, nil, "")
	assert.Equal(t, 400, resp.StatusCode, "dismissed is final")
	_, data = f.do("GET", "/users/self/observer_alerts", lnParent, 1, nil, "")
	assert.Len(t, decodeList(t, data), 0)
}

// G149: a grade entered under a manual post policy alerts when it is
// posted, even though it was graded long before the scan's high-water mark.
func TestLearnerObserverAlerts_ManualPostAlertsWhenPosted(t *testing.T) {
	f := newLearnerFixture(t)
	resp, data := f.do("POST", "/observer_alert_thresholds", lnParent, 1, map[string]any{
		"threshold": map[string]any{"user_id": lnStudent, "alert_type": "assignment_grade_low", "threshold": 70},
	}, "")
	require.Equal(t, 201, resp.StatusCode, string(data))

	quiz := f.assignment(lnCourse, "Unit quiz", nil, 10, nil)
	sub := f.grade(quiz.ID, lnStudent, 3, false)
	require.NoError(t, f.g.Model(sub).Update("graded_at", time.Now().Add(-2*time.Hour)).Error)
	require.NoError(t, f.alert.EvaluateAll(context.Background()))
	_, data = f.do("GET", "/users/self/observer_alerts", lnParent, 1, nil, "")
	assert.Len(t, decodeList(t, data), 0, "hidden grade: no alert yet")

	time.Sleep(10 * time.Millisecond)
	require.NoError(t, f.g.Model(sub).Update("posted_at", time.Now()).Error)
	require.NoError(t, f.alert.EvaluateAll(context.Background()))
	_, data = f.do("GET", "/users/self/observer_alerts", lnParent, 1, nil, "")
	alerts := decodeList(t, data)
	require.Len(t, alerts, 1, "posting the grade raises the alert")
	assert.Equal(t, "assignment_grade_low", alerts[0]["alert_type"])
}

// G144: an override only changes the flags it is given, and work turned in
// after the student unchecked or dismissed it counts as done again.
func TestLearnerPlanner_OverrideFlagsAndLaterSubmission(t *testing.T) {
	f := newLearnerFixture(t)
	now := time.Now()
	essay := f.assignment(lnCourse, "Essay", ptrTime(now.Add(24*time.Hour)), 10, nil)

	resp, data := f.do("POST", "/planner/overrides", lnStudent, 1, map[string]any{
		"plannable_type": "assignment", "plannable_id": essay.ID, "marked_complete": true,
	}, "")
	require.Equal(t, 201, resp.StatusCode, string(data))
	resp, data = f.do("POST", "/planner/overrides", lnStudent, 1, map[string]any{
		"plannable_type": "assignment", "plannable_id": essay.ID, "dismissed": true,
	}, "")
	require.Equal(t, 201, resp.StatusCode, string(data))
	o := decodeMap(t, data)
	assert.Equal(t, true, o["marked_complete"], "dismissing keeps the done mark")
	assert.Equal(t, true, o["dismissed"])

	resp, data = f.do("POST", "/planner/overrides", lnStudent, 1, map[string]any{
		"plannable_type": "assignment", "plannable_id": essay.ID, "marked_complete": false,
	}, "")
	require.Equal(t, 201, resp.StatusCode, string(data))
	o = decodeMap(t, data)
	assert.Equal(t, false, o["marked_complete"])
	assert.Equal(t, true, o["dismissed"], "unchecking keeps the dismissal")

	itemFor := func() map[string]any {
		_, data := f.do("GET", "/planner/items", lnStudent, 1, nil, "")
		for _, it := range decodeList(t, data) {
			if it["plannable_type"] == "assignment" && it["plannable_id"] == float64(essay.ID) {
				return it
			}
		}
		t.Fatalf("essay not on the planner: %s", data)
		return nil
	}
	assert.Equal(t, false, itemFor()["completed"], "explicitly not done")

	time.Sleep(10 * time.Millisecond)
	submitted := time.Now()
	require.NoError(t, f.g.Create(&models.Submission{AssignmentID: essay.ID, UserID: lnStudent, SubmittedAt: &submitted,
		WorkflowState: models.SubmissionSubmitted}).Error)
	assert.Equal(t, true, itemFor()["completed"], "turned in after the override: done")
}

// G161: profile fields, link validation, visibility and avatar upload.
func TestLearnerProfile(t *testing.T) {
	f := newLearnerFixture(t)
	resp, data := f.do("PUT", "/users/self/profile", lnStudent, 1, map[string]any{
		"pronouns": "she/her", "bio": "I like <script>x</script>cells",
		"links": []map[string]string{{"title": "Portfolio", "url": "example.test/jane"}},
	}, "")
	require.Equal(t, 200, resp.StatusCode, string(data))
	p := decodeMap(t, data)
	assert.Equal(t, "she/her", p["pronouns"])
	assert.NotContains(t, p["bio"], "<script>")
	links := p["links"].([]any)
	require.Len(t, links, 1)
	assert.Equal(t, "https://example.test/jane", links[0].(map[string]any)["url"])

	resp, _ = f.do("PUT", "/users/self/profile", lnStudent, 1, map[string]any{
		"links": []map[string]string{{"title": "x", "url": "javascript:alert(1)"}},
	}, "")
	assert.Equal(t, 400, resp.StatusCode)

	// Classmate (shared course) sees it; a stranger and another tenant don't.
	resp, _ = f.do("GET", fmt.Sprintf("/users/%d/profile_card", lnStudent), lnClassmate, 1, nil, "")
	assert.Equal(t, 200, resp.StatusCode)
	resp, _ = f.do("GET", fmt.Sprintf("/users/%d/profile_card", lnStudent), lnStranger, 1, nil, "")
	assert.Equal(t, 404, resp.StatusCode)
	resp, _ = f.do("GET", fmt.Sprintf("/users/%d/profile_card", lnStudent), lnOtherTen, 2, nil, "")
	assert.Equal(t, 404, resp.StatusCode)

	upload := func(content []byte) (*http.Response, []byte) {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		part, _ := w.CreateFormFile("file", "me.png")
		_, _ = part.Write(content)
		_ = w.Close()
		req := httptest.NewRequest("POST", "/users/self/avatar", &buf)
		req.Header.Set("Content-Type", w.FormDataContentType())
		req.Header.Set("X-Test-User", strconv.Itoa(int(lnStudent)))
		req.Header.Set("X-Test-Account", "1")
		resp, err := f.app.Test(req, -1)
		require.NoError(t, err)
		body, _ := io.ReadAll(resp.Body)
		return resp, body
	}
	resp, _ = upload([]byte("<svg onload=alert(1)></svg>"))
	assert.Equal(t, 400, resp.StatusCode, "not an allowed image type")
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 13, 'I', 'H', 'D', 'R', 0, 0, 0, 1, 0, 0, 0, 1, 8, 6, 0, 0, 0}
	resp, data = upload(png)
	require.Equal(t, 200, resp.StatusCode, string(data))
	assert.Contains(t, decodeMap(t, data)["avatar_url"], fmt.Sprintf("/api/v1/users/%d/avatar", lnStudent))

	resp, data = f.do("GET", fmt.Sprintf("/users/%d/avatar", lnStudent), lnClassmate, 1, nil, "")
	require.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "image/png", resp.Header.Get("Content-Type"))
	assert.Equal(t, png, data)
	resp, _ = f.do("GET", fmt.Sprintf("/users/%d/avatar", lnStudent), lnOtherTen, 2, nil, "")
	assert.Equal(t, 404, resp.StatusCode, "another tenant")
}
