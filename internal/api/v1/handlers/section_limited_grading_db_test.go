package handlers_test

// G107 seams: a section-limited grader must not reach other sections'
// students through bulk grading routes, anonymous ids, request bodies or
// a section-less post / hide. Runs the real course gate in front of the
// handlers. All data is made up.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

const (
	slSectionA = uint(501) // TA's section: students A and C
	slSectionB = uint(502) // student B
)

func newSectionLimitedEnv(t *testing.T) *anonRoutesEnv {
	t.Helper()
	e := newAnonRoutesEnv(t)
	g := e.g
	for _, s := range []*models.CourseSection{
		{ID: slSectionA, CourseID: guCourse, Name: "Section A", WorkflowState: "active"},
		{ID: slSectionB, CourseID: guCourse, Name: "Section B", WorkflowState: "active"},
	} {
		require.NoError(t, g.Create(s).Error)
	}
	set := func(user, section uint, limited bool) {
		require.NoError(t, g.Model(&models.Enrollment{}).Where("user_id = ? AND course_id = ?", user, guCourse).
			Updates(map[string]interface{}{"course_section_id": section, "limit_privileges_to_course_section": limited}).Error)
	}
	set(guTA, slSectionA, true)
	set(guStudentA, slSectionA, false)
	set(guStudentC, slSectionA, false)
	set(guStudentB, slSectionB, false)
	e.subH.SetGradingPolicyService(service.NewGradingPolicyService(postgres.NewGradingPolicyRepository(g), postgres.NewGradingStandardRepository(g), e.subSvc))
	return e
}

// gated mounts the routes behind the real course gate as userID.
func (e *anonRoutesEnv) gated(userID, accountID uint) *fiber.App {
	g := e.g
	pm := middleware.NewPermissionMiddleware(postgres.NewEnrollmentRepository(g), postgres.NewUserRepository(g), postgres.NewCourseRepository(g))
	enrolled, instructor := pm.RequireEnrolled(), pm.RequireInstructor()
	app := testutil.SetupTestApp()
	app.Use(callerStub(userID, accountID))
	a := "/courses/:course_id/assignments/:assignment_id"
	app.Get("/courses/:course_id/grades", enrolled, e.gpH.CourseGrades)
	app.Get("/courses/:course_id/effective_due_dates", instructor, e.gpH.EffectiveDueDates)
	app.Get(a+"/speedgrader", instructor, e.sgH.GetSpeedGraderData)
	app.Get(a+"/anonymous_submissions/:anonymous_id/comments", instructor, e.guH.ListAnonymousSubmissionComments)
	app.Put(a+"/provisional_grades", instructor, e.guH.SaveProvisionalGrade)
	app.Get(a+"/moderation", instructor, e.guH.GetModeration)
	app.Post("/courses/:course_id/assignments/:id/peer_reviews/manual", instructor, e.guH.AssignPeerReview)
	app.Get("/courses/:course_id/assignments/:id/peer_reviews", instructor, e.prH.ListPeerReviews)
	app.Post("/courses/:course_id/assignments/:id/post_grades", instructor, e.subH.PostGrades)
	app.Post("/courses/:course_id/assignments/:id/hide_grades", instructor, e.subH.HideGrades)
	return app
}

func TestSectionLimitedGrader_BulkRoutesStayInSection(t *testing.T) {
	e := newSectionLimitedEnv(t)
	a := e.assignment(t, &models.Assignment{PostPolicy: "manual"})
	for _, s := range []uint{guStudentA, guStudentB, guStudentC} {
		code, body := submitText(t, e.guEnv, a, s)
		require.Equal(t, http.StatusCreated, code, body)
		_, err := e.subSvc.Grade(context.Background(), a.ID, s, guTeacher, 1, "8")
		require.NoError(t, err)
	}
	ta := e.gated(guTA, 1)
	teacher := e.gated(guTeacher, 1)
	userKey := func(id uint) string { return fmt.Sprintf(`"user_id":%d`, id) }

	// Course grades, with and without ?user_ids naming the other section.
	for _, q := range []string{"", fmt.Sprintf("?user_ids=%d,%d", guStudentA, guStudentB)} {
		code, body := guDo(t, ta, http.MethodGet, fmt.Sprintf("/courses/%d/grades%s", guCourse, q), nil)
		require.Equal(t, http.StatusOK, code, body)
		assert.NotContains(t, body, userKey(guStudentB), "grades%s", q)
	}
	// ?user_id= naming another section is stopped by the course gate.
	code, _ := guDo(t, ta, http.MethodGet, fmt.Sprintf("/courses/%d/grades?user_id=%d", guCourse, guStudentB), nil)
	assert.Equal(t, http.StatusNotFound, code)
	_, body := guDo(t, ta, http.MethodGet, fmt.Sprintf("/courses/%d/grades", guCourse), nil)
	assert.Contains(t, body, userKey(guStudentA))
	_, body = guDo(t, teacher, http.MethodGet, fmt.Sprintf("/courses/%d/grades", guCourse), nil)
	assert.Contains(t, body, userKey(guStudentB), "unlimited teacher sees everyone")

	// Effective due dates.
	code, body = guDo(t, ta, http.MethodGet, fmt.Sprintf("/courses/%d/effective_due_dates?user_ids=%d,%d", guCourse, guStudentA, guStudentB), nil)
	require.Equal(t, http.StatusOK, code, body)
	var dd map[string]map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(body), &dd))
	per := dd[fmt.Sprint(a.ID)]
	assert.Contains(t, per, fmt.Sprint(guStudentA))
	assert.NotContains(t, per, fmt.Sprint(guStudentB))

	// SpeedGrader.
	code, body = guDo(t, ta, http.MethodGet, fmt.Sprintf("/courses/%d/assignments/%d/speedgrader", guCourse, a.ID), nil)
	require.Equal(t, http.StatusOK, code, body)
	assert.Contains(t, body, "Ada Student")
	assert.NotContains(t, body, "Ben Student")

	// Peer reviews by body ids.
	prPath := fmt.Sprintf("/courses/%d/assignments/%d/peer_reviews/manual", guCourse, a.ID)
	code, _ = guDo(t, ta, http.MethodPost, prPath, map[string]interface{}{"reviewer_id": guStudentA, "reviewee_id": guStudentB})
	assert.Equal(t, http.StatusNotFound, code)
	code, body = guDo(t, ta, http.MethodPost, prPath, map[string]interface{}{"reviewer_id": guStudentA, "reviewee_id": guStudentC})
	assert.Equal(t, http.StatusCreated, code, body)
	code, body = guDo(t, teacher, http.MethodPost, prPath, map[string]interface{}{"reviewer_id": guStudentB, "reviewee_id": guStudentA})
	require.Equal(t, http.StatusCreated, code, body)
	_, body = guDo(t, ta, http.MethodGet, fmt.Sprintf("/courses/%d/assignments/%d/peer_reviews", guCourse, a.ID), nil)
	assert.NotContains(t, body, fmt.Sprintf(`"reviewer_id":%d`, guStudentB))

	// Post grades: naming another section is refused; no section_ids
	// posts only the TA's own sections.
	postPath := fmt.Sprintf("/courses/%d/assignments/%d/post_grades", guCourse, a.ID)
	code, _ = guDo(t, ta, http.MethodPost, postPath, map[string]interface{}{"section_ids": []uint{slSectionB}})
	assert.Equal(t, http.StatusNotFound, code)
	code, body = guDo(t, ta, http.MethodPost, postPath, map[string]interface{}{})
	require.Equal(t, http.StatusOK, code, body)
	posted := func(user uint) bool {
		var sub models.Submission
		require.NoError(t, e.g.Where("assignment_id = ? AND user_id = ?", a.ID, user).First(&sub).Error)
		return sub.PostedAt != nil
	}
	assert.True(t, posted(guStudentA))
	assert.True(t, posted(guStudentC))
	assert.False(t, posted(guStudentB), "other section untouched")

	// Hide the same way, then the teacher can post everyone.
	code, _ = guDo(t, ta, http.MethodPost, fmt.Sprintf("/courses/%d/assignments/%d/hide_grades", guCourse, a.ID), nil)
	require.Equal(t, http.StatusOK, code)
	assert.False(t, posted(guStudentA))
	code, _ = guDo(t, teacher, http.MethodPost, postPath, nil)
	require.Equal(t, http.StatusOK, code)
	assert.True(t, posted(guStudentB))

	// Another tenant's teacher is stopped at the gate.
	code, _ = guDo(t, e.gated(guOutsider, 2), http.MethodGet, fmt.Sprintf("/courses/%d/grades", guCourse), nil)
	assert.Equal(t, http.StatusNotFound, code)
}

func TestSectionLimitedGrader_AnonymousAndModerationRoutes(t *testing.T) {
	e := newSectionLimitedEnv(t)
	ta := guTA
	a := e.assignment(t, &models.Assignment{AnonymousGrading: true, ModeratedGrading: true, GraderCount: 2, FinalGraderID: &ta, PostPolicy: "manual"})
	for _, s := range []uint{guStudentA, guStudentB} {
		code, body := submitText(t, e.guEnv, a, s)
		require.Equal(t, http.StatusCreated, code, body)
	}
	// SpeedGrader (as the teacher) hands out anonymous ids.
	code, body := guDo(t, e.gated(guTeacher, 1), http.MethodGet, fmt.Sprintf("/courses/%d/assignments/%d/speedgrader", guCourse, a.ID), nil)
	require.Equal(t, http.StatusOK, code, body)
	anonA := anonymousIDFor(t, e.guEnv, a.ID, guStudentA)
	anonB := anonymousIDFor(t, e.guEnv, a.ID, guStudentB)

	app := e.gated(guTA, 1)
	base := fmt.Sprintf("/courses/%d/assignments/%d", guCourse, a.ID)
	code, _ = guDo(t, app, http.MethodGet, base+"/anonymous_submissions/"+anonA+"/comments", nil)
	assert.Equal(t, http.StatusOK, code)
	code, _ = guDo(t, app, http.MethodGet, base+"/anonymous_submissions/"+anonB+"/comments", nil)
	assert.Equal(t, http.StatusNotFound, code, "anonymous id of another section")

	for _, in := range []map[string]interface{}{
		{"user_id": guStudentB, "posted_grade": "5"},
		{"anonymous_id": anonB, "posted_grade": "5"},
	} {
		code, body := guDo(t, app, http.MethodPut, base+"/provisional_grades", in)
		assert.Equal(t, http.StatusNotFound, code, "%v %s", in, body)
	}
	code, body = guDo(t, app, http.MethodPut, base+"/provisional_grades", map[string]interface{}{"anonymous_id": anonA, "posted_grade": "6"})
	assert.Equal(t, http.StatusOK, code, body)

	code, body = guDo(t, app, http.MethodGet, base+"/moderation", nil)
	require.Equal(t, http.StatusOK, code, body)
	assert.Contains(t, body, anonA)
	assert.NotContains(t, body, anonB)
}
