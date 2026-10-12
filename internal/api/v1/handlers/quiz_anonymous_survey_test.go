package handlers_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

// On an anonymous survey the staff event log (question_answered events
// carry the answers) and the manual-scoring route (it returns the
// answers) stay closed: the respondent's responses are theirs alone.
func TestQuizAnonymousSurvey_EventsAndScoresHiddenFromStaff(t *testing.T) {
	survey := func(anonymous bool) *models.Quiz {
		q := publishedQuiz(5, 7)
		q.QuizType = "graded_survey"
		q.AnonymousSubmissions = anonymous
		return q
	}
	run := func(anonymous bool) (events, scores int, d *lifecycleDeps) {
		app, d := lifecycleApp("TeacherEnrollment", 2)
		lh := handlers.NewQuizLifecycleHandler(service.NewQuizService(d.quizRepo, d.qRepo, d.sRepo, d.aRepo,
			service.WithQuizLifecycle(service.QuizLifecycleDeps{Events: d.events})))
		app.Get("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/events", lh.ListEvents)
		d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(survey(anonymous), nil)
		d.sRepo.On("FindByID", mock.Anything, uint(30)).Return(&models.QuizSubmission{ID: 30, QuizID: 5, UserID: 10, Attempt: 1, WorkflowState: "complete"}, nil)
		d.events.rows = []models.QuizSubmissionEvent{{QuizSubmissionID: 30, EventType: "question_answered", EventData: `{"answer":"made-up"}`}}
		resp := testutil.MakeRequest(app, http.MethodGet, "/courses/7/quizzes/5/submissions/30/events", nil)
		events = resp.StatusCode
		if anonymous {
			resp = testutil.MakeRequest(app, http.MethodPut, "/courses/7/quizzes/5/submissions/30/scores", strings.NewReader(`{"fudge_points":1}`))
			scores = resp.StatusCode
		}
		return events, scores, d
	}

	events, scores, d := run(true)
	assert.Equal(t, http.StatusNotFound, events)
	assert.Equal(t, http.StatusNotFound, scores)
	d.aRepo.AssertNotCalled(t, "ListBySubmissionID", mock.Anything, mock.Anything)

	// A non-anonymous survey's event log stays readable to staff.
	events, _, _ = run(false)
	assert.Equal(t, http.StatusOK, events)
}
