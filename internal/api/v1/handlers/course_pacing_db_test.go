package handlers_test

// G079: blackout dates (CRUD + course tie) and a published course pace
// that skips them and dates quizzes and graded discussions too.

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

func TestCoursePacing_BlackoutDatesQuizzesAndDiscussions(t *testing.T) {
	g := contentLaneDB(t)
	seedContentAccounts(t, g, 1)
	ctx := context.Background()
	course := seedContentCourse(t, g, 1, "PACE-1")
	other := seedContentCourse(t, g, 1, "PACE-2")
	// Friday 2026-09-04.
	start := time.Date(2026, 9, 4, 9, 0, 0, 0, time.UTC)
	require.NoError(t, g.Model(course).Update("start_at", start).Error)
	mpSeedUser(t, g, 601, 1, "Pat Poster")

	assignment := seedContentAssignment(t, g, course.ID, "Essay")
	quiz := &models.Quiz{CourseID: course.ID, Title: "Checkpoint", WorkflowState: "available", Published: true}
	require.NoError(t, g.Create(quiz).Error)
	graded := seedContentAssignment(t, g, course.ID, "Debate (graded)")
	topic := &models.DiscussionTopic{CourseID: course.ID, UserID: 601, Title: "Debate", AssignmentID: &graded.ID, WorkflowState: models.DiscussionTopicWorkflow("active")}
	require.NoError(t, g.Create(topic).Error)

	mod := &models.ContextModule{CourseID: course.ID, Name: "Unit 1", Position: 1, WorkflowState: "active"}
	require.NoError(t, g.Create(mod).Error)
	iA := mpItem(t, g, mod.ID, 1, "Assignment", assignment.ID, "", "")
	iQ := mpItem(t, g, mod.ID, 2, "Quizzes::Quiz", quiz.ID, "", "")
	iD := mpItem(t, g, mod.ID, 3, "DiscussionTopic", topic.ID, "", "")

	paceSvc := service.NewCoursePaceService(postgres.NewCoursePaceRepository(g), postgres.NewCoursePaceModuleItemRepository(g), postgres.NewModuleItemRepository(g), postgres.NewAssignmentRepository(g))
	paceSvc.SetDeps(service.CoursePaceDeps{ModuleRepo: postgres.NewModuleRepository(g), CourseRepo: postgres.NewCourseRepository(g)})
	paceSvc.SetPacingRepos(postgres.NewBlackoutDateRepository(g), postgres.NewQuizRepository(g), postgres.NewDiscussionTopicRepository(g))
	h := handlers.NewCoursePaceHandler(paceSvc)

	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(7))
		c.Locals("account_id", uint(1))
		c.Locals("enrollment_type", "TeacherEnrollment")
		return c.Next()
	})
	app.Get("/courses/:course_id/blackout_dates", h.ListBlackoutDates)
	app.Post("/courses/:course_id/blackout_dates", h.CreateBlackoutDate)
	app.Delete("/courses/:course_id/blackout_dates/:id", h.DeleteBlackoutDate)
	app.Get("/courses/:course_id/course_pacing/:id/timeline", h.GetTimeline)

	// Validation.
	resp := testutil.MakeRequest(app, http.MethodPost, fmt.Sprintf("/courses/%d/blackout_dates", course.ID), testutil.JSONBody(map[string]any{
		"blackout_date": map[string]any{"event_title": "Backwards", "start_date": "2026-09-10", "end_date": "2026-09-08"},
	}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// Labor Day week: Mon 7th and Tue 8th are blacked out.
	resp = testutil.MakeRequest(app, http.MethodPost, fmt.Sprintf("/courses/%d/blackout_dates", course.ID), testutil.JSONBody(map[string]any{
		"blackout_date": map[string]any{"event_title": "Labor Day break", "start_date": "2026-09-07", "end_date": "2026-09-08"},
	}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	created, _ := testutil.ParseJSONMap(resp)
	blackoutID := uint(created["id"].(float64))
	assert.Equal(t, "2026-09-07", created["start_date"])

	resp = testutil.MakeRequest(app, http.MethodGet, fmt.Sprintf("/courses/%d/blackout_dates", course.ID), nil)
	rows, _ := testutil.ParseJSONArray(resp)
	assert.Len(t, rows, 1)
	// Another course's id is a 404 and leaves the row alone.
	resp = testutil.MakeRequest(app, http.MethodDelete, fmt.Sprintf("/courses/%d/blackout_dates/%d", other.ID, blackoutID), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	pace := &models.CoursePace{CourseID: course.ID, ExcludeWeekends: true}
	require.NoError(t, paceSvc.Create(ctx, pace))
	_, err := paceSvc.UpdatePaceItems(ctx, pace, []models.CoursePaceModuleItem{
		{ModuleItemID: iA.ID, Duration: 1},
		{ModuleItemID: iQ.ID, Duration: 1},
		{ModuleItemID: iD.ID, Duration: 1},
	})
	require.NoError(t, err)

	resp = testutil.MakeRequest(app, http.MethodGet, fmt.Sprintf("/courses/%d/course_pacing/%d/timeline", course.ID, pace.ID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	timeline, _ := testutil.ParseJSONArray(resp)
	require.Len(t, timeline, 3)
	assert.Contains(t, timeline[0]["projected_date"], "2026-09-09", "weekend and blackout days are skipped")

	_, err = paceSvc.PublishPace(ctx, pace.ID, 1)
	require.NoError(t, err)

	day := func(tm *time.Time) string {
		require.NotNil(t, tm)
		return tm.UTC().Format("2006-01-02")
	}
	var a models.Assignment
	require.NoError(t, g.First(&a, assignment.ID).Error)
	assert.Equal(t, "2026-09-09", day(a.DueAt))
	var q models.Quiz
	require.NoError(t, g.First(&q, quiz.ID).Error)
	assert.Equal(t, "2026-09-10", day(q.DueAt), "quizzes are paced")
	var ga models.Assignment
	require.NoError(t, g.First(&ga, graded.ID).Error)
	assert.Equal(t, "2026-09-11", day(ga.DueAt), "graded discussions are paced through their assignment")

	resp = testutil.MakeRequest(app, http.MethodDelete, fmt.Sprintf("/courses/%d/blackout_dates/%d", course.ID, blackoutID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
