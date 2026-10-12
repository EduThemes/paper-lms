package handlers_test

// DB-backed tests for the grade write paths and grading notifications
// (post-merge audit, lane B): machine scores (LTI passback, gradebook
// import) are points, provisional grades read typed grades, the typed
// grade survives re-formatting, quiz score sync honours manual statuses
// and late-policy re-application, moderation gates on excuse / status,
// hiding grades switches to manual posting, and comment / grade
// notifications. Skipped without a database. All data is made up.

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

// wpEnv is the grading-UI env plus handlers wired with real notification
// delivery and the grading-policy service.
type wpEnv struct {
	*guEnv
	subH *handlers.SubmissionHandler
	guH  *handlers.GradingUIHandler
	gp   *service.GradingPolicyService
}

func newWPEnv(t *testing.T) *wpEnv {
	t.Helper()
	e := newGUEnv(t)
	g := e.g
	users := postgres.NewUserRepository(g)
	delivery := service.NewNotificationDeliveryService(postgres.NewNotificationDeliveryRepository(g), postgres.NewCommunicationChannelRepository(g),
		postgres.NewNotificationPreferenceRepository(g), postgres.NewNotificationRepository(g), users, nil)
	e.subSvc.SetGradeNotifier(func(ctx context.Context, a *models.Assignment, sub *models.Submission) {
		service.NotifyGradePosted(ctx, delivery, a, sub)
	})
	gp := service.NewGradingPolicyService(postgres.NewGradingPolicyRepository(g), postgres.NewGradingStandardRepository(g), e.subSvc)
	subH := handlers.NewSubmissionHandler(e.subSvc, postgres.NewSubmissionCommentRepository(g), postgres.NewAttachmentRepository(g), users,
		postgres.NewAssignmentRepository(g), delivery, nil, nil, nil, nil)
	subH.SetGradingUIService(e.gu)
	subH.SetGradingPolicyService(gp)
	guH := handlers.NewGradingUIHandler(e.gu, subH, users, nil, nil)
	return &wpEnv{guEnv: e, subH: subH, guH: guH, gp: gp}
}

func (e *wpEnv) wpApp(userID uint, enrollmentType string) *fiber.App {
	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		c.Locals("account_id", uint(1))
		c.Locals("enrollment_type", enrollmentType)
		c.Locals("is_admin", false)
		return c.Next()
	})
	a := "/courses/:course_id/assignments/:assignment_id"
	app.Post("/courses/:course_id/assignments", e.asgH.CreateAssignment)
	app.Put("/courses/:course_id/assignments/:id", e.asgH.UpdateAssignment)
	app.Post(a+"/submissions", e.subH.CreateSubmission)
	app.Put(a+"/submissions/:user_id", e.subH.UpdateSubmission)
	app.Post(a+"/submissions/:user_id/comments", e.subH.CreateSubmissionComment)
	app.Put(a+"/submissions/:user_id/comments/:comment_id", e.guH.UpdateSubmissionComment)
	app.Post("/courses/:course_id/assignments/:id/post_grades", e.subH.PostGrades)
	app.Post("/courses/:course_id/assignments/:id/hide_grades", e.subH.HideGrades)
	app.Post("/courses/:course_id/submissions/bulk_grade", e.subH.BulkGrade)
	return app
}

func (e *wpEnv) wpTeacher() *fiber.App { return e.wpApp(guTeacher, "TeacherEnrollment") }

// setPoints changes an assignment's points_possible (the shared helper
// always creates 10-point assignments).
func (e *wpEnv) setPoints(t *testing.T, a *models.Assignment, pts float64) {
	t.Helper()
	require.NoError(t, e.g.Model(&models.Assignment{}).Where("id = ?", a.ID).Update("points_possible", pts).Error)
	a.PointsPossible = &pts
}

func (e *wpEnv) sub(t *testing.T, a *models.Assignment, userID uint) *models.Submission {
	t.Helper()
	s, err := e.subSvc.GetByAssignmentAndUser(context.Background(), a.ID, userID, 1)
	require.NoError(t, err)
	return s
}

func (e *wpEnv) notifications(t *testing.T, userID uint, typ string) []models.Notification {
	t.Helper()
	var out []models.Notification
	require.NoError(t, e.g.Where("user_id = ? AND notification_type = ?", userID, typ).Order("id").Find(&out).Error)
	return out
}

func wpStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Finding 1: an LTI / import score is points, never a typed grade.
func TestGradeScore_MachineScoresArePoints(t *testing.T) {
	e := newWPEnv(t)
	ctx := context.Background()

	pct := e.assignment(t, &models.Assignment{Name: "Percent tool", GradingType: "percent"})
	sub, err := e.subSvc.GradeScore(ctx, pct.ID, guStudentA, guTeacher, 1, 7)
	require.NoError(t, err)
	require.NotNil(t, sub.Score)
	assert.Equal(t, 7.0, *sub.Score, "7/10 from a tool is 7 points, not 0.7")
	assert.Equal(t, "70%", wpStr(sub.Grade))

	pf := e.assignment(t, &models.Assignment{Name: "Pass fail tool", GradingType: "pass_fail"})
	sub, err = e.subSvc.GradeScore(ctx, pf.ID, guStudentA, guTeacher, 1, 7)
	require.NoError(t, err, "a partial score on a pass/fail assignment is accepted")
	assert.Equal(t, 7.0, *sub.Score)
	assert.Equal(t, "complete", wpStr(sub.Grade))

	// Typed grades still use the grading type.
	sub, err = e.subSvc.Grade(ctx, pct.ID, guStudentB, guTeacher, 1, "70")
	require.NoError(t, err)
	assert.Equal(t, 7.0, *sub.Score)

	// Cross-tenant callers are refused.
	_, err = e.subSvc.GradeScore(ctx, pct.ID, guStudentA, guTeacher, 2, 7)
	assert.ErrorIs(t, err, service.ErrGradeCrossTenant)
}

// Finding 1 + 7d: GradeScore notifies when the visible grade changes, and
// re-sending the same score stays quiet.
func TestGradeScore_NotifiesOnChangeOnly(t *testing.T) {
	e := newWPEnv(t)
	ctx := context.Background()
	a := e.assignment(t, &models.Assignment{Name: "Tool"})
	_, err := e.subSvc.GradeScore(ctx, a.ID, guStudentA, 0, 1, 7)
	require.NoError(t, err)
	_, err = e.subSvc.GradeScore(ctx, a.ID, guStudentA, 0, 1, 7)
	require.NoError(t, err)
	assert.Len(t, e.notifications(t, guStudentA, "submission_grade"), 1)
	_, err = e.subSvc.GradeScore(ctx, a.ID, guStudentA, 0, 1, 8)
	require.NoError(t, err)
	assert.Len(t, e.notifications(t, guStudentA, "submission_grade"), 2)

	// A manual post policy keeps the grade (and the notice) hidden.
	hidden := e.assignment(t, &models.Assignment{Name: "Hidden tool", PostPolicy: "manual"})
	_, err = e.subSvc.GradeScore(ctx, hidden.ID, guStudentB, 0, 1, 5)
	require.NoError(t, err)
	assert.Empty(t, e.notifications(t, guStudentB, "submission_grade"))
}

// Finding 1 + 7d: the gradebook CSV import / curve send points as
// "score"; the API stores them as points and notifies.
func TestBulkGrade_ScoreEntriesArePoints(t *testing.T) {
	e := newWPEnv(t)
	a := e.assignment(t, &models.Assignment{Name: "Percent", GradingType: "percent"})
	code, body := guDo(t, e.wpTeacher(), http.MethodPost, fmt.Sprintf("/courses/%d/submissions/bulk_grade", guCourse),
		map[string]interface{}{"grade_data": []map[string]interface{}{
			{"assignment_id": a.ID, "user_id": guStudentA, "score": 8.5},
			{"assignment_id": a.ID, "user_id": guStudentB, "posted_grade": "85"},
		}})
	require.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, 8.5, *e.sub(t, a, guStudentA).Score)
	assert.Equal(t, "85%", wpStr(e.sub(t, a, guStudentA).Grade))
	assert.Equal(t, 8.5, *e.sub(t, a, guStudentB).Score, "posted_grade keeps typed-grade semantics")
	assert.Len(t, e.notifications(t, guStudentA, "submission_grade"), 1)
	assert.Len(t, e.notifications(t, guStudentB, "submission_grade"), 1)
}

// Finding 2: provisional grades are read per the grading type, and the
// published grade matches what the grader typed.
func TestProvisionalGrade_TypedGrades(t *testing.T) {
	e := newWPEnv(t)
	ctx := context.Background()
	final := guTeacher
	a := e.assignment(t, &models.Assignment{Name: "Moderated percent", GradingType: "percent", ModeratedGrading: true, GraderCount: 1, FinalGraderID: &final})
	e.setPoints(t, a, 50)
	code, body := submitText(t, e.guEnv, a, guStudentA)
	require.Equal(t, http.StatusCreated, code, body)

	pg, err := e.gu.SaveProvisionalGrade(ctx, a, guStudentA, guTA, false, false, "85")
	require.NoError(t, err)
	require.NotNil(t, pg.Score)
	assert.Equal(t, 42.5, *pg.Score, "85 on a percent assignment is 85%")
	assert.Equal(t, "85%", wpStr(pg.Grade))
	pg, err = e.gu.SaveProvisionalGrade(ctx, a, guStudentA, guTA, false, false, "85%")
	require.NoError(t, err)
	assert.Equal(t, 42.5, *pg.Score)
	_, err = e.gu.SaveProvisionalGrade(ctx, a, guStudentA, guTA, false, false, "nonsense")
	assert.Error(t, err)

	res, err := e.gu.PublishGrades(ctx, a, guTeacher, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Published)
	sub := e.sub(t, a, guStudentA)
	assert.Equal(t, 42.5, *sub.Score)
	assert.Equal(t, "85%", wpStr(sub.Grade))

	letter := e.assignment(t, &models.Assignment{Name: "Moderated letter", GradingType: "letter_grade", ModeratedGrading: true, GraderCount: 1, FinalGraderID: &final})
	pg, err = e.gu.SaveProvisionalGrade(ctx, letter, guStudentA, guTA, false, false, "B+")
	require.NoError(t, err)
	assert.Equal(t, "B+", wpStr(pg.Grade))
	require.NotNil(t, pg.Score)
	assert.Greater(t, *pg.Score, 0.0)

	pf := e.assignment(t, &models.Assignment{Name: "Moderated pass fail", GradingType: "pass_fail", ModeratedGrading: true, GraderCount: 1, FinalGraderID: &final})
	pg, err = e.gu.SaveProvisionalGrade(ctx, pf, guStudentA, guTA, false, false, "complete")
	require.NoError(t, err)
	assert.Equal(t, 10.0, *pg.Score)
}

// Finding 3: a point-less pass/fail grade stays "complete" through
// moderated publishing and a late-status re-derivation.
func TestTypedGrade_SurvivesReformatting(t *testing.T) {
	e := newWPEnv(t)
	ctx := context.Background()
	final := guTeacher
	m := e.assignment(t, &models.Assignment{Name: "Zero pf moderated", GradingType: "pass_fail", ModeratedGrading: true, GraderCount: 1, FinalGraderID: &final})
	e.setPoints(t, m, 0)
	_, err := e.gu.SaveProvisionalGrade(ctx, m, guStudentA, guTA, false, false, "complete")
	require.NoError(t, err)
	_, err = e.gu.PublishGrades(ctx, m, guTeacher, 1)
	require.NoError(t, err)
	assert.Equal(t, "complete", wpStr(e.sub(t, m, guStudentA).Grade))

	a := e.assignment(t, &models.Assignment{Name: "Zero pf", GradingType: "pass_fail"})
	e.setPoints(t, a, 0)
	_, err = e.subSvc.Grade(ctx, a.ID, guStudentA, guTeacher, 1, "complete")
	require.NoError(t, err)
	sub, err := e.subSvc.SetLatePolicyStatus(ctx, a.ID, guStudentA, guTeacher, 1, models.LatePolicyStatusLate, nil)
	require.NoError(t, err)
	assert.Equal(t, "complete", wpStr(sub.Grade))
}

// Finding 4: quiz score sync honours a manual late status, and the late
// policy is re-applied to auto-graded quiz scores after a due-date change.
func TestQuizScoreSync_LateStatusAndReapply(t *testing.T) {
	e := newWPEnv(t)
	ctx := context.Background()
	require.NoError(t, postgres.NewLatePolicyRepository(e.g).Create(ctx, &models.LatePolicy{
		CourseID: guCourse, LateSubmissionDeductionEnabled: true, LateSubmissionDeduction: 10, LateSubmissionInterval: "day",
	}))
	due := time.Now().Add(-36 * time.Hour)
	a := e.assignment(t, &models.Assignment{Name: "Quiz", SubmissionTypes: "online_quiz", DueAt: &due})
	finished := time.Now().Add(-time.Minute)

	sub, err := e.subSvc.RecordQuizScore(ctx, a, service.QuizScoreRecord{UserID: guStudentA, Score: 8, SubmittedAt: finished, Attempt: 1})
	require.NoError(t, err)
	assert.True(t, sub.Late)
	assert.Equal(t, 6.0, *sub.Score, "two days late at 10%/day")

	// An extension (new due date) re-applies the late policy to the
	// auto-graded score, which has no grader.
	later := time.Now().Add(48 * time.Hour)
	require.NoError(t, e.g.Model(&models.Assignment{}).Where("id = ?", a.ID).Update("due_at", later).Error)
	require.NoError(t, e.gp.RefreshDueStatus(ctx, guCourse, []uint{a.ID}))
	sub = e.sub(t, a, guStudentA)
	assert.False(t, sub.Late)
	assert.Equal(t, 8.0, *sub.Score)

	// Back to past due, with the instructor marking the work not late:
	// a regrade keeps the manual status.
	require.NoError(t, e.g.Model(&models.Assignment{}).Where("id = ?", a.ID).Update("due_at", due).Error)
	a.DueAt = &due
	_, err = e.subSvc.SetLatePolicyStatus(ctx, a.ID, guStudentA, guTeacher, 1, models.LatePolicyStatusNone, nil)
	require.NoError(t, err)
	sub, err = e.subSvc.RecordQuizScore(ctx, a, service.QuizScoreRecord{UserID: guStudentA, Score: 9, SubmittedAt: finished, Attempt: 1})
	require.NoError(t, err)
	assert.False(t, sub.Late, "a regrade must not reset late_policy_status=none")
	assert.Equal(t, 9.0, *sub.Score)
	require.NotNil(t, sub.LatePolicyStatus)
	assert.Equal(t, models.LatePolicyStatusNone, *sub.LatePolicyStatus)
}

// Finding 7d: quiz scores notify on a regrade / manual score change, not
// on the student's own finished attempt.
func TestQuizScoreSync_Notifications(t *testing.T) {
	e := newWPEnv(t)
	ctx := context.Background()
	a := e.assignment(t, &models.Assignment{Name: "Quiz", SubmissionTypes: "online_quiz"})
	finished := time.Now().Add(-time.Minute)
	_, err := e.subSvc.RecordQuizScore(ctx, a, service.QuizScoreRecord{UserID: guStudentA, Score: 6, SubmittedAt: finished, Attempt: 1})
	require.NoError(t, err)
	assert.Empty(t, e.notifications(t, guStudentA, "submission_grade"))
	_, err = e.subSvc.RecordQuizScore(ctx, a, service.QuizScoreRecord{UserID: guStudentA, Score: 7, SubmittedAt: finished, Attempt: 1})
	require.NoError(t, err)
	assert.Len(t, e.notifications(t, guStudentA, "submission_grade"), 1, "a regrade changed the visible grade")
	_, err = e.subSvc.RecordQuizScore(ctx, a, service.QuizScoreRecord{UserID: guStudentA, Score: 9, SubmittedAt: finished, Attempt: 1, GraderID: guTeacher})
	require.NoError(t, err)
	assert.Len(t, e.notifications(t, guStudentA, "submission_grade"), 2)
}

// Finding 5: excusing, "EX" and late/missing statuses wait for the
// moderator to publish.
func TestModeration_GatesExcuseAndStatus(t *testing.T) {
	e := newWPEnv(t)
	ctx := context.Background()
	require.NoError(t, postgres.NewLatePolicyRepository(e.g).Create(ctx, &models.LatePolicy{
		CourseID: guCourse, MissingSubmissionDeductionEnabled: true, MissingSubmissionDeduction: 50,
	}))
	final := guTeacher
	a := e.assignment(t, &models.Assignment{Name: "Moderated", ModeratedGrading: true, GraderCount: 1, FinalGraderID: &final})

	_, err := e.subSvc.SetExcused(ctx, a.ID, guStudentA, guTeacher, 1, true)
	assert.ErrorIs(t, err, service.ErrModerationInProgress)
	_, err = e.subSvc.Grade(ctx, a.ID, guStudentA, guTeacher, 1, "EX")
	assert.ErrorIs(t, err, service.ErrModerationInProgress)
	_, err = e.subSvc.SetLatePolicyStatus(ctx, a.ID, guStudentA, guTeacher, 1, models.LatePolicyStatusMissing, nil)
	assert.ErrorIs(t, err, service.ErrModerationInProgress)

	path := fmt.Sprintf("/courses/%d/assignments/%d/submissions/%d", guCourse, a.ID, guStudentA)
	code, body := guDo(t, e.wpTeacher(), http.MethodPut, path, map[string]interface{}{"submission": map[string]interface{}{"excused": true}})
	assert.Equal(t, http.StatusConflict, code, body)
	code, body = guDo(t, e.wpTeacher(), http.MethodPut, path, map[string]interface{}{"submission": map[string]interface{}{"late_policy_status": "missing"}})
	assert.Equal(t, http.StatusConflict, code, body)

	var n int64
	require.NoError(t, e.g.Model(&models.Submission{}).Where("assignment_id = ? AND score IS NOT NULL", a.ID).Count(&n).Error)
	assert.Zero(t, n, "no missing-policy score was written")
}

// Finding 6: hiding grades switches the assignment to manual posting, so
// a later comment or grade does not re-post them.
func TestHideGrades_SwitchesToManual(t *testing.T) {
	e := newWPEnv(t)
	ctx := context.Background()
	a := e.assignment(t, &models.Assignment{Name: "Hide me"})
	_, err := e.subSvc.Grade(ctx, a.ID, guStudentA, guTeacher, 1, "8")
	require.NoError(t, err)
	require.NotNil(t, e.sub(t, a, guStudentA).PostedAt)

	code, body := guDo(t, e.wpTeacher(), http.MethodPost, fmt.Sprintf("/courses/%d/assignments/%d/hide_grades", guCourse, a.ID), nil)
	require.Equal(t, http.StatusOK, code, body)
	assert.Contains(t, body, `"post_policy":"manual"`)
	var reloaded models.Assignment
	require.NoError(t, e.g.First(&reloaded, a.ID).Error)
	assert.Equal(t, "manual", reloaded.PostPolicy)
	assert.Nil(t, e.sub(t, a, guStudentA).PostedAt)

	code, body = guDo(t, e.wpTeacher(), http.MethodPost, fmt.Sprintf("/courses/%d/assignments/%d/submissions/%d/comments", guCourse, a.ID, guStudentA),
		map[string]interface{}{"comment": map[string]interface{}{"text_comment": "See me"}})
	require.Equal(t, http.StatusCreated, code, body)
	_, err = e.subSvc.Grade(ctx, a.ID, guStudentA, guTeacher, 1, "9")
	require.NoError(t, err)
	assert.Nil(t, e.sub(t, a, guStudentA).PostedAt, "the hidden grade stays hidden")
	assert.Empty(t, e.notifications(t, guStudentA, "submission_comment"))
}

// Finding 7a: moderated assignments always post manually, and a grader's
// comment before publishing is not sent to the student.
func TestModerated_ManualPostPolicyAndQuietComments(t *testing.T) {
	e := newWPEnv(t)
	code, body := guDo(t, e.wpTeacher(), http.MethodPost, fmt.Sprintf("/courses/%d/assignments", guCourse), map[string]interface{}{
		"assignment": map[string]interface{}{"name": "Moderated essay", "points_possible": 10, "published": true,
			"post_policy": "automatic", "moderated_grading": true, "final_grader_id": guTeacher},
	})
	require.Equal(t, http.StatusCreated, code, body)
	assert.Contains(t, body, `"post_policy":"manual"`)
	var created models.Assignment
	require.NoError(t, e.g.Where("name = ?", "Moderated essay").First(&created).Error)
	code, body = guDo(t, e.wpTeacher(), http.MethodPut, fmt.Sprintf("/courses/%d/assignments/%d", guCourse, created.ID),
		map[string]interface{}{"assignment": map[string]interface{}{"post_policy": "automatic"}})
	require.Equal(t, http.StatusOK, code, body)
	assert.Contains(t, body, `"post_policy":"manual"`)

	// Even on a legacy automatic moderated row with a posted submission,
	// the comment notice waits for publishing.
	final := guTeacher
	legacy := e.assignment(t, &models.Assignment{Name: "Legacy moderated", ModeratedGrading: true, GraderCount: 1, FinalGraderID: &final})
	now := time.Now()
	sub := &models.Submission{AssignmentID: legacy.ID, UserID: guStudentA, WorkflowState: models.SubmissionSubmitted, PostedAt: &now}
	e.subH.NotifySubmissionCommentForTest(context.Background(), legacy, sub, guTA, "Sam Assistant", "provisional note")
	assert.Empty(t, e.notifications(t, guStudentA, "submission_comment"))
}

// Finding 7b: publishing a draft comment notifies the student.
func TestDraftComment_PublishNotifies(t *testing.T) {
	e := newWPEnv(t)
	a := e.assignment(t, &models.Assignment{Name: "Drafts"})
	code, body := submitText(t, e.guEnv, a, guStudentA)
	require.Equal(t, http.StatusCreated, code, body)
	base := fmt.Sprintf("/courses/%d/assignments/%d/submissions/%d/comments", guCourse, a.ID, guStudentA)
	code, body = guDo(t, e.wpTeacher(), http.MethodPost, base, map[string]interface{}{"comment": map[string]interface{}{"text_comment": "Nice start", "draft": true}})
	require.Equal(t, http.StatusCreated, code, body)
	assert.Empty(t, e.notifications(t, guStudentA, "submission_comment"))

	var cm models.SubmissionComment
	require.NoError(t, e.g.Where("comment = ?", "Nice start").First(&cm).Error)
	code, body = guDo(t, e.wpTeacher(), http.MethodPut, fmt.Sprintf("%s/%d", base, cm.ID), map[string]interface{}{"comment": map[string]interface{}{"draft": false}})
	require.Equal(t, http.StatusOK, code, body)
	notes := e.notifications(t, guStudentA, "submission_comment")
	require.Len(t, notes, 1)
	assert.Contains(t, notes[0].Message, "Nice start")
}

// Finding 7c: comments held back by a manual post policy are sent when
// grades are posted, once; hiding and re-posting sends only new ones.
func TestPostGrades_SendsHeldComments(t *testing.T) {
	e := newWPEnv(t)
	a := e.assignment(t, &models.Assignment{Name: "Manual", PostPolicy: "manual"})
	code, body := submitText(t, e.guEnv, a, guStudentA)
	require.Equal(t, http.StatusCreated, code, body)
	comments := fmt.Sprintf("/courses/%d/assignments/%d/submissions/%d/comments", guCourse, a.ID, guStudentA)
	for _, text := range []string{"First note", "Second note"} {
		code, body = guDo(t, e.wpTeacher(), http.MethodPost, comments, map[string]interface{}{"comment": map[string]interface{}{"text_comment": text}})
		require.Equal(t, http.StatusCreated, code, body)
	}
	assert.Empty(t, e.notifications(t, guStudentA, "submission_comment"))

	post := fmt.Sprintf("/courses/%d/assignments/%d/post_grades", guCourse, a.ID)
	hide := fmt.Sprintf("/courses/%d/assignments/%d/hide_grades", guCourse, a.ID)
	code, body = guDo(t, e.wpTeacher(), http.MethodPost, post, nil)
	require.Equal(t, http.StatusOK, code, body)
	notes := e.notifications(t, guStudentA, "submission_comment")
	require.Len(t, notes, 1, "one notice for the held comments")
	assert.Contains(t, notes[0].Message, "First note")
	assert.Contains(t, notes[0].Message, "Second note")

	code, body = guDo(t, e.wpTeacher(), http.MethodPost, hide, nil)
	require.Equal(t, http.StatusOK, code, body)
	time.Sleep(5 * time.Millisecond)
	code, body = guDo(t, e.wpTeacher(), http.MethodPost, comments, map[string]interface{}{"comment": map[string]interface{}{"text_comment": "Third note"}})
	require.Equal(t, http.StatusCreated, code, body)
	code, body = guDo(t, e.wpTeacher(), http.MethodPost, post, nil)
	require.Equal(t, http.StatusOK, code, body)
	notes = e.notifications(t, guStudentA, "submission_comment")
	require.Len(t, notes, 2)
	assert.Contains(t, notes[1].Message, "Third note")
	assert.NotContains(t, notes[1].Message, "First note", "already-sent comments are not repeated")

	// Posting again with nothing new sends nothing.
	code, body = guDo(t, e.wpTeacher(), http.MethodPost, post, nil)
	require.Equal(t, http.StatusOK, code, body)
	assert.Len(t, e.notifications(t, guStudentA, "submission_comment"), 2)
}

