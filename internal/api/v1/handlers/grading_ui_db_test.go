package handlers_test

// DB-backed tests for the grading-UI lane: attempt limits and allowed
// extensions, attempt history, group grading, moderated grading,
// anonymous grading, custom grade statuses, comment drafts / group
// comments / editing, peer-review management and assignment settings.
// They run against a scratch, fully migrated database and are skipped
// when neither PARITY_DB_URL nor DATABASE_URL is set. All data is made up.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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

const (
	guTeacher  = uint(10)
	guTA       = uint(11)
	guStudentA = uint(20)
	guStudentB = uint(21)
	guStudentC = uint(22)
	guOutsider = uint(30) // account 2
	guCourse   = uint(100)
	guCourse2  = uint(200) // account 2
)

type guEnv struct {
	g      *gorm.DB
	subSvc *service.SubmissionService
	gu     *service.GradingUIService
	subH   *handlers.SubmissionHandler
	sgH    *handlers.SpeedGraderHandler
	gbH    *handlers.GradebookHandler
	guH    *handlers.GradingUIHandler
	prH    *handlers.PeerReviewHandler
	asgH   *handlers.AssignmentHandler
}

func newGUEnv(t *testing.T) *guEnv {
	t.Helper()
	g := coreScratchDB(t) // accounts 1 and 2, users 3 and 8
	ctx := context.Background()

	users := postgres.NewUserRepository(g)
	for _, u := range []*models.User{
		{ID: guTeacher, AccountID: 1, Name: "Pat Teacher", LoginID: "pat@example.test", Email: "pat@example.test", PasswordHash: "x", Role: "user"},
		{ID: guTA, AccountID: 1, Name: "Sam Assistant", LoginID: "sam@example.test", Email: "sam@example.test", PasswordHash: "x", Role: "user"},
		{ID: guStudentA, AccountID: 1, Name: "Ada Student", LoginID: "ada@example.test", Email: "ada@example.test", PasswordHash: "x", Role: "user"},
		{ID: guStudentB, AccountID: 1, Name: "Ben Student", LoginID: "ben@example.test", Email: "ben@example.test", PasswordHash: "x", Role: "user"},
		{ID: guStudentC, AccountID: 1, Name: "Cy Student", LoginID: "cy@example.test", Email: "cy@example.test", PasswordHash: "x", Role: "user"},
		{ID: guOutsider, AccountID: 2, Name: "Other Teacher", LoginID: "other@example.test", Email: "other@example.test", PasswordHash: "x", Role: "user"},
	} {
		require.NoError(t, users.Create(ctx, u))
	}
	courses := postgres.NewCourseRepository(g)
	require.NoError(t, courses.Create(ctx, &models.Course{ID: guCourse, AccountID: 1, Name: "Biology 101", CourseCode: "BIO101"}))
	require.NoError(t, courses.Create(ctx, &models.Course{ID: guCourse2, AccountID: 2, Name: "Other", CourseCode: "OTH"}))
	enrollments := postgres.NewEnrollmentRepository(g)
	for _, e := range []*models.Enrollment{
		{UserID: guTeacher, CourseID: guCourse, Type: "TeacherEnrollment", Role: "TeacherEnrollment"},
		{UserID: guTA, CourseID: guCourse, Type: "TaEnrollment", Role: "TaEnrollment"},
		{UserID: guStudentA, CourseID: guCourse, Type: "StudentEnrollment", Role: "StudentEnrollment"},
		{UserID: guStudentB, CourseID: guCourse, Type: "StudentEnrollment", Role: "StudentEnrollment"},
		{UserID: guStudentC, CourseID: guCourse, Type: "StudentEnrollment", Role: "StudentEnrollment"},
		{UserID: guOutsider, CourseID: guCourse2, Type: "TeacherEnrollment", Role: "TeacherEnrollment"},
	} {
		require.NoError(t, enrollments.Create(ctx, e))
	}

	submissionRepo := postgres.NewSubmissionRepository(g)
	assignmentRepo := postgres.NewAssignmentRepository(g)
	groupMembershipRepo := postgres.NewGroupMembershipRepository(g)
	commentRepo := postgres.NewSubmissionCommentRepository(g)
	attachmentRepo := postgres.NewAttachmentRepository(g)
	peerReviewRepo := postgres.NewPeerReviewRepository(g)
	subSvc := service.NewSubmissionService(submissionRepo, assignmentRepo, enrollments, postgres.NewLatePolicyRepository(g), courses,
		postgres.NewGradingPeriodGroupRepository(g), postgres.NewGradingPeriodRepository(g), groupMembershipRepo)
	guRepo := postgres.NewGradingUIRepository(g)
	subSvc.SetGradingPolicyRepository(postgres.NewGradingPolicyRepository(g))
	gu := service.NewGradingUIService(guRepo, submissionRepo, assignmentRepo, courses, groupMembershipRepo, peerReviewRepo, subSvc)
	peerSvc := service.NewPeerReviewService(peerReviewRepo, submissionRepo, enrollments)

	subH := handlers.NewSubmissionHandler(subSvc, commentRepo, attachmentRepo, users, assignmentRepo, nil, nil, nil, nil, nil)
	subH.SetPeerReviewService(peerSvc)
	subH.SetGradingUIService(gu)
	sgH := handlers.NewSpeedGraderHandler(service.NewSpeedGraderService(submissionRepo, commentRepo, assignmentRepo, enrollments, nil), nil)
	sgH.SetGradingUIService(gu)
	gradingSvc := service.NewGradingService(submissionRepo, assignmentRepo, postgres.NewAssignmentGroupRepository(g), enrollments, courses, postgres.NewGradingStandardRepository(g))
	gbH := handlers.NewGradebookHandler(gradingSvc, nil)
	gbH.SetGradingUIService(gu)
	prH := handlers.NewPeerReviewHandler(peerSvc, nil)
	prH.SetAssignmentService(service.NewAssignmentService(assignmentRepo))
	prH.SetUserRepository(users)
	asgH := handlers.NewAssignmentHandler(service.NewAssignmentService(assignmentRepo))
	asgH.SetGradingUIDeps(enrollments, attachmentRepo)
	asgH.SetGradesExistCheck(gu.GradesExist)
	guH := handlers.NewGradingUIHandler(gu, subH, users, peerSvc, nil)
	guH.SetRubricService(service.NewRubricService(postgres.NewRubricRepository(g), postgres.NewRubricAssociationRepository(g), postgres.NewRubricAssessmentRepository(g)))

	return &guEnv{g: g, subSvc: subSvc, gu: gu, subH: subH, sgH: sgH, gbH: gbH, guH: guH, prH: prH, asgH: asgH}
}

// app mounts the grading routes behind a stub that authenticates as
// userID in accountID with the given course enrollment type.
func (e *guEnv) app(userID, accountID uint, enrollmentType string, isAdmin bool) *fiber.App {
	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		c.Locals("account_id", accountID)
		c.Locals("enrollment_type", enrollmentType)
		c.Locals("is_admin", isAdmin)
		return c.Next()
	})
	a := "/courses/:course_id/assignments/:assignment_id"
	app.Post("/courses/:course_id/assignments", e.asgH.CreateAssignment)
	app.Put("/courses/:course_id/assignments/:id", e.asgH.UpdateAssignment)
	app.Post(a+"/submissions", e.subH.CreateSubmission)
	app.Put(a+"/submissions/:user_id", e.subH.UpdateSubmission)
	app.Get(a+"/submissions/:user_id/comments", e.subH.ListSubmissionComments)
	app.Post(a+"/submissions/:user_id/comments", e.subH.CreateSubmissionComment)
	app.Put(a+"/submissions/:user_id/comments/:comment_id", e.guH.UpdateSubmissionComment)
	app.Delete(a+"/submissions/:user_id/comments/:comment_id", e.guH.DeleteSubmissionComment)
	app.Post("/courses/:course_id/assignments/:id/post_grades", e.subH.PostGrades)
	app.Get(a+"/speedgrader", e.sgH.GetSpeedGraderData)
	app.Get("/courses/:course_id/gradebook", e.gbH.GetGradebook)
	anon := a + "/anonymous_submissions/:anonymous_id"
	app.Put(anon, e.guH.UpdateAnonymousSubmission)
	app.Get(anon+"/comments", e.guH.ListAnonymousSubmissionComments)
	app.Put(anon+"/comments/:comment_id", e.guH.UpdateAnonymousSubmissionComment)
	app.Delete(anon+"/comments/:comment_id", e.guH.DeleteAnonymousSubmissionComment)
	app.Get(anon+"/rubric_assessment", e.guH.GetAnonymousRubricAssessment)
	app.Put(anon+"/rubric_assessment", e.guH.SaveAnonymousRubricAssessment)
	app.Get("/courses/:course_id/grading_periods", e.guH.ListCourseGradingPeriods)
	app.Post(anon+"/comments", e.guH.CreateAnonymousSubmissionComment)
	app.Put(a+"/provisional_grades", e.guH.SaveProvisionalGrade)
	app.Get(a+"/moderation", e.guH.GetModeration)
	app.Put(a+"/moderation/selections", e.guH.SelectProvisionalGrade)
	app.Post(a+"/moderation/publish", e.guH.PublishModeratedGrades)
	app.Get("/accounts/:account_id/custom_grade_statuses", e.guH.ListAccountCustomGradeStatuses)
	app.Post("/accounts/:account_id/custom_grade_statuses", e.guH.CreateCustomGradeStatus)
	app.Delete("/accounts/:account_id/custom_grade_statuses/:status_id", e.guH.DeleteCustomGradeStatus)
	app.Get("/courses/:course_id/custom_grade_statuses", e.guH.ListCourseCustomGradeStatuses)
	app.Post("/courses/:course_id/assignments/:id/peer_reviews/manual", e.guH.AssignPeerReview)
	app.Delete("/courses/:course_id/assignments/:id/peer_reviews/:review_id", e.guH.DeletePeerReview)
	app.Get("/courses/:course_id/assignments/:id/peer_reviews/received", e.guH.ListReceivedPeerReviews)
	app.Get("/courses/:course_id/assignments/:id/peer_reviews/mine", e.prH.ListMyPeerReviews)
	app.Get("/courses/:course_id/assignments/:id/peer_reviews", e.prH.ListPeerReviews)
	app.Put("/peer_reviews/:review_id", e.prH.SubmitPeerReview)
	return app
}

func (e *guEnv) teacher() *fiber.App { return e.app(guTeacher, 1, "TeacherEnrollment", false) }
func (e *guEnv) student(id uint) *fiber.App {
	return e.app(id, 1, "StudentEnrollment", false)
}

func (e *guEnv) assignment(t *testing.T, a *models.Assignment) *models.Assignment {
	t.Helper()
	a.CourseID = guCourse
	if a.Name == "" {
		a.Name = "Essay"
	}
	if a.SubmissionTypes == "" {
		a.SubmissionTypes = "online_text_entry"
	}
	a.WorkflowState = models.AssignmentPublished
	a.Published = true
	pts := 10.0
	a.PointsPossible = &pts
	if a.PostPolicy == "" {
		a.PostPolicy = "automatic"
	}
	require.NoError(t, postgres.NewAssignmentRepository(e.g).Create(context.Background(), a))
	return a
}

func guDo(t *testing.T, app *fiber.App, method, path string, body interface{}) (int, string) {
	t.Helper()
	var r io.Reader
	if body != nil {
		r = testutil.JSONBody(body)
	}
	resp := testutil.MakeRequest(app, method, path, r)
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

func submitText(t *testing.T, e *guEnv, a *models.Assignment, student uint) (int, string) {
	return guDo(t, e.student(student), http.MethodPost, fmt.Sprintf("/courses/%d/assignments/%d/submissions", guCourse, a.ID),
		map[string]interface{}{"submission": map[string]interface{}{"submission_type": "online_text_entry", "body": "<p>my answer</p>"}})
}

// G027: allowed_attempts caps resubmission; extra_attempts lifts the cap.
func TestGradingUI_AttemptLimit(t *testing.T) {
	e := newGUEnv(t)
	one := 1
	a := e.assignment(t, &models.Assignment{AllowedAttempts: &one})

	code, body := submitText(t, e, a, guStudentA)
	require.Equal(t, http.StatusCreated, code, body)
	code, body = submitText(t, e, a, guStudentA)
	assert.Equal(t, http.StatusConflict, code, body)
	assert.Contains(t, body, "attempts")

	// The teacher grants one extra attempt.
	code, body = guDo(t, e.teacher(), http.MethodPut, fmt.Sprintf("/courses/%d/assignments/%d/submissions/%d", guCourse, a.ID, guStudentA),
		map[string]interface{}{"submission": map[string]interface{}{"extra_attempts": 1}})
	require.Equal(t, http.StatusOK, code, body)
	code, body = submitText(t, e, a, guStudentA)
	assert.Equal(t, http.StatusCreated, code, body)

	// G034: every attempt is kept for SpeedGrader.
	sub, err := e.subSvc.GetByAssignmentAndUser(context.Background(), a.ID, guStudentA, 1)
	require.NoError(t, err)
	versions, err := e.gu.VersionsForSubmission(context.Background(), sub.ID)
	require.NoError(t, err)
	require.Len(t, versions, 2)
	assert.Equal(t, 1, versions[0].Attempt)
	assert.Equal(t, 2, versions[1].Attempt)
}

// G027: allowed_extensions is enforced on submit.
func TestGradingUI_AllowedExtensions(t *testing.T) {
	e := newGUEnv(t)
	a := e.assignment(t, &models.Assignment{SubmissionTypes: "online_upload", AllowedExtensions: "pdf,docx"})
	ctx := context.Background()
	mk := func(name string) *models.Submission {
		typ := "online_upload"
		att := fmt.Sprintf(`[{"id":1,"filename":%q,"content_type":"application/octet-stream"}]`, name)
		return &models.Submission{AssignmentID: a.ID, UserID: guStudentA, SubmissionType: &typ, Attachments: &att}
	}
	err := e.subSvc.Submit(ctx, mk("notes.txt"), guCourse, 1)
	assert.ErrorIs(t, err, service.ErrFileExtensionNotAllowed)
	assert.NoError(t, e.subSvc.Submit(ctx, mk("Report.PDF"), guCourse, 1))
}

// G026: media submissions need an audio/video file; annotation
// submissions need the assignment's document.
func TestGradingUI_MediaAndAnnotationSubmissions(t *testing.T) {
	e := newGUEnv(t)
	ctx := context.Background()
	media := e.assignment(t, &models.Assignment{SubmissionTypes: "media_recording"})
	typ := "media_recording"
	bad := `[{"id":1,"filename":"x.pdf","content_type":"application/pdf"}]`
	err := e.subSvc.Submit(ctx, &models.Submission{AssignmentID: media.ID, UserID: guStudentA, SubmissionType: &typ, Attachments: &bad}, guCourse, 1)
	assert.Error(t, err)
	good := `[{"id":1,"filename":"clip.webm","content_type":"video/webm"}]`
	assert.NoError(t, e.subSvc.Submit(ctx, &models.Submission{AssignmentID: media.ID, UserID: guStudentA, SubmissionType: &typ, Attachments: &good}, guCourse, 1))

	ann := e.assignment(t, &models.Assignment{SubmissionTypes: "student_annotation"})
	at := "student_annotation"
	err = e.subSvc.Submit(ctx, &models.Submission{AssignmentID: ann.ID, UserID: guStudentA, SubmissionType: &at}, guCourse, 1)
	assert.Error(t, err, "no annotatable document configured")

	// LTI hook: an external tool records a basic_lti_launch submission.
	lti := e.assignment(t, &models.Assignment{SubmissionTypes: "external_tool"})
	sub, err := e.subSvc.RecordExternalToolSubmission(ctx, lti, guStudentB, "https://tool.example.test/return")
	require.NoError(t, err)
	assert.Equal(t, "basic_lti_launch", *sub.SubmissionType)
	_, err = e.subSvc.RecordExternalToolSubmission(ctx, lti, guStudentB, "javascript:alert(1)")
	assert.Error(t, err)
}

func guSeedGroup(t *testing.T, e *guEnv, members ...uint) uint {
	t.Helper()
	course := guCourse
	cat := &models.GroupCategory{CourseID: &course, Name: "Project teams"}
	require.NoError(t, e.g.Create(cat).Error)
	grp := &models.Group{GroupCategoryID: cat.ID, Name: "Team Rocket", ContextType: "Course", ContextID: guCourse}
	require.NoError(t, e.g.Create(grp).Error)
	for _, m := range members {
		require.NoError(t, e.g.Create(&models.GroupMembership{GroupID: grp.ID, UserID: m, WorkflowState: "accepted"}).Error)
	}
	return cat.ID
}

// G037: a group assignment copies the grade to the group, unless it is
// graded individually. G033: a group comment reaches every member.
func TestGradingUI_GroupGradingAndGroupComments(t *testing.T) {
	e := newGUEnv(t)
	ctx := context.Background()
	cat := guSeedGroup(t, e, guStudentA, guStudentB)

	together := e.assignment(t, &models.Assignment{GroupCategoryID: &cat})
	_, err := e.subSvc.Grade(ctx, together.ID, guStudentA, guTeacher, 1, "8")
	require.NoError(t, err)
	b, err := e.subSvc.GetByAssignmentAndUser(ctx, together.ID, guStudentB, 1)
	require.NoError(t, err)
	require.NotNil(t, b.Score)
	assert.Equal(t, 8.0, *b.Score)

	individual := e.assignment(t, &models.Assignment{GroupCategoryID: &cat, GradeGroupStudentsIndividually: true})
	_, err = e.subSvc.Grade(ctx, individual.ID, guStudentA, guTeacher, 1, "9")
	require.NoError(t, err)
	_, err = e.subSvc.GetByAssignmentAndUser(ctx, individual.ID, guStudentB, 1)
	assert.Error(t, err, "B must not receive A's individual grade")

	// Group comment from the teacher lands on both members' submissions.
	code, body := guDo(t, e.teacher(), http.MethodPost, fmt.Sprintf("/courses/%d/assignments/%d/submissions/%d/comments", guCourse, together.ID, guStudentA),
		map[string]interface{}{"comment": map[string]interface{}{"text_comment": "Nice teamwork", "group_comment": true}})
	require.Equal(t, http.StatusCreated, code, body)
	code, body = guDo(t, e.student(guStudentB), http.MethodGet, fmt.Sprintf("/courses/%d/assignments/%d/submissions/%d/comments", guCourse, together.ID, guStudentB), nil)
	require.Equal(t, http.StatusOK, code, body)
	assert.Contains(t, body, "Nice teamwork")

	// SpeedGrader lists each student's group.
	code, body = guDo(t, e.teacher(), http.MethodGet, fmt.Sprintf("/courses/%d/assignments/%d/speedgrader", guCourse, together.ID), nil)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, "Team Rocket")
}

// G033: drafts are private to their author until published; the author
// can edit and delete; files must be the author's own.
func TestGradingUI_CommentDraftsEditDelete(t *testing.T) {
	e := newGUEnv(t)
	a := e.assignment(t, &models.Assignment{})
	code, body := submitText(t, e, a, guStudentA)
	require.Equal(t, http.StatusCreated, code, body)
	base := fmt.Sprintf("/courses/%d/assignments/%d/submissions/%d/comments", guCourse, a.ID, guStudentA)

	code, body = guDo(t, e.teacher(), http.MethodPost, base, map[string]interface{}{"comment": map[string]interface{}{"text_comment": "draft feedback", "draft": true}})
	require.Equal(t, http.StatusCreated, code, body)
	var created map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(body), &created))
	id := uint(created["id"].(float64))

	_, body = guDo(t, e.student(guStudentA), http.MethodGet, base, nil)
	assert.NotContains(t, body, "draft feedback", "students never see a draft")

	// Someone else cannot edit it.
	code, _ = guDo(t, e.app(guTA, 1, "TaEnrollment", false), http.MethodPut, fmt.Sprintf("%s/%d", base, id),
		map[string]interface{}{"comment": map[string]interface{}{"draft": false}})
	assert.NotEqual(t, http.StatusOK, code)

	code, body = guDo(t, e.teacher(), http.MethodPut, fmt.Sprintf("%s/%d", base, id),
		map[string]interface{}{"comment": map[string]interface{}{"text_comment": "final feedback", "draft": false}})
	require.Equal(t, http.StatusOK, code, body)
	_, body = guDo(t, e.student(guStudentA), http.MethodGet, base, nil)
	assert.Contains(t, body, "final feedback")

	// Unknown / foreign file ids are refused.
	code, _ = guDo(t, e.teacher(), http.MethodPost, base, map[string]interface{}{"comment": map[string]interface{}{"text_comment": "see file", "file_ids": []uint{424242}}})
	assert.Equal(t, http.StatusBadRequest, code)

	// A classmate cannot delete it; the author can.
	code, _ = guDo(t, e.student(guStudentB), http.MethodDelete, fmt.Sprintf("%s/%d", base, id), nil)
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = guDo(t, e.teacher(), http.MethodDelete, fmt.Sprintf("%s/%d", base, id), nil)
	assert.Equal(t, http.StatusOK, code)
	_, body = guDo(t, e.student(guStudentA), http.MethodGet, base, nil)
	assert.NotContains(t, body, "final feedback")
}

// G031: under anonymous grading SpeedGrader exposes neither user ids nor
// names; grading and comments go through the anonymous id; the
// gradebook hides the column until grades are posted, then names return.
func TestGradingUI_AnonymousGrading(t *testing.T) {
	e := newGUEnv(t)
	a := e.assignment(t, &models.Assignment{AnonymousGrading: true, PostPolicy: "manual"})
	code, body := submitText(t, e, a, guStudentA)
	require.Equal(t, http.StatusCreated, code, body)

	code, body = guDo(t, e.teacher(), http.MethodGet, fmt.Sprintf("/courses/%d/assignments/%d/speedgrader", guCourse, a.ID), nil)
	require.Equal(t, http.StatusOK, code, body)
	assert.NotContains(t, body, "Ada Student")
	assert.NotContains(t, body, fmt.Sprintf(`"user_id":%d`, guStudentA))
	var sg struct {
		Assignment struct {
			AnonymizeStudents bool `json:"anonymize_students"`
		} `json:"assignment"`
		Students []struct {
			AnonymousID string                 `json:"anonymous_id"`
			UserID      *uint                  `json:"user_id"`
			Submission  map[string]interface{} `json:"submission"`
		} `json:"students"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &sg))
	assert.True(t, sg.Assignment.AnonymizeStudents)
	require.Len(t, sg.Students, 3, "every student gets an anonymous row")
	var adaAnon string
	for _, s := range sg.Students {
		assert.Nil(t, s.UserID)
		require.Len(t, s.AnonymousID, 5)
		if s.Submission["workflow_state"] == "submitted" {
			adaAnon = s.AnonymousID
		}
	}
	require.NotEmpty(t, adaAnon)

	anonPath := fmt.Sprintf("/courses/%d/assignments/%d/anonymous_submissions/%s", guCourse, a.ID, adaAnon)
	code, body = guDo(t, e.teacher(), http.MethodPut, anonPath, map[string]interface{}{"submission": map[string]interface{}{"posted_grade": "7"}})
	require.Equal(t, http.StatusOK, code, body)
	assert.NotContains(t, body, fmt.Sprintf(`"user_id":%d`, guStudentA))
	code, body = guDo(t, e.teacher(), http.MethodPost, anonPath+"/comments", map[string]interface{}{"comment": map[string]interface{}{"text_comment": "good start"}})
	require.Equal(t, http.StatusCreated, code, body)

	// Tenant + course tie: another tenant's teacher gets 404.
	code, _ = guDo(t, e.app(guOutsider, 2, "TeacherEnrollment", false), http.MethodPut, anonPath, map[string]interface{}{"submission": map[string]interface{}{"posted_grade": "1"}})
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = guDo(t, e.teacher(), http.MethodPut, fmt.Sprintf("/courses/%d/assignments/%d/anonymous_submissions/zzzzz", guCourse, a.ID), map[string]interface{}{"submission": map[string]interface{}{"posted_grade": "1"}})
	assert.Equal(t, http.StatusNotFound, code)

	// The gradebook masks the anonymized column.
	code, body = guDo(t, e.teacher(), http.MethodGet, fmt.Sprintf("/courses/%d/gradebook", guCourse), nil)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, fmt.Sprintf(`"anonymized_assignment_ids":[%d]`, a.ID))
	assert.NotContains(t, body, `"score":7`)

	// Posting grades to every student lifts anonymity.
	for _, s := range []uint{guStudentB, guStudentC} {
		_, err := e.subSvc.Grade(context.Background(), a.ID, s, guTeacher, 1, "5")
		require.NoError(t, err)
	}
	code, _ = guDo(t, e.teacher(), http.MethodPost, fmt.Sprintf("/courses/%d/assignments/%d/post_grades", guCourse, a.ID), nil)
	require.Equal(t, http.StatusOK, code)
	_, body = guDo(t, e.teacher(), http.MethodGet, fmt.Sprintf("/courses/%d/assignments/%d/speedgrader", guCourse, a.ID), nil)
	assert.Contains(t, body, "Ada Student")
}

// G030: moderated grading — provisional grades only, a grader-count
// limit, moderator selection, publish, then normal posting.
func TestGradingUI_ModeratedGrading(t *testing.T) {
	e := newGUEnv(t)
	final := guTeacher
	a := e.assignment(t, &models.Assignment{ModeratedGrading: true, GraderCount: 1, FinalGraderID: &final, GraderNamesVisibleToFinalGrader: true})
	ctx := context.Background()
	code, body := submitText(t, e, a, guStudentA)
	require.Equal(t, http.StatusCreated, code, body)

	// A direct grade is refused until grades are published.
	_, err := e.subSvc.Grade(ctx, a.ID, guStudentA, guTA, 1, "5")
	assert.ErrorIs(t, err, service.ErrModerationInProgress)
	code, _ = guDo(t, e.teacher(), http.MethodPost, fmt.Sprintf("/courses/%d/assignments/%d/post_grades", guCourse, a.ID), nil)
	assert.Equal(t, http.StatusConflict, code)

	ta := e.app(guTA, 1, "TaEnrollment", false)
	pgPath := fmt.Sprintf("/courses/%d/assignments/%d/provisional_grades", guCourse, a.ID)
	code, body = guDo(t, ta, http.MethodPut, pgPath, map[string]interface{}{"user_id": guStudentA, "posted_grade": "6"})
	require.Equal(t, http.StatusOK, code, body)
	var taPG map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(body), &taPG))

	// grader_count=1: a second provisional grader is refused, the final
	// grader (moderator) is not.
	require.NoError(t, e.g.Create(&models.User{ID: 12, AccountID: 1, Name: "Lee Grader", LoginID: "lee@example.test", Email: "lee@example.test", PasswordHash: "x", Role: "user"}).Error)
	code, _ = guDo(t, e.app(12, 1, "TaEnrollment", false), http.MethodPut, pgPath, map[string]interface{}{"user_id": guStudentA, "posted_grade": "3"})
	assert.Equal(t, http.StatusConflict, code)
	code, body = guDo(t, e.teacher(), http.MethodPut, pgPath, map[string]interface{}{"user_id": guStudentA, "posted_grade": "9"})
	require.Equal(t, http.StatusOK, code, body)

	// Only the moderator sees the summary.
	code, _ = guDo(t, ta, http.MethodGet, fmt.Sprintf("/courses/%d/assignments/%d/moderation", guCourse, a.ID), nil)
	assert.Equal(t, http.StatusForbidden, code)
	code, body = guDo(t, e.teacher(), http.MethodGet, fmt.Sprintf("/courses/%d/assignments/%d/moderation", guCourse, a.ID), nil)
	require.Equal(t, http.StatusOK, code, body)
	assert.Contains(t, body, "Sam Assistant")

	// Choose the TA's grade, publish, and the student has it.
	code, body = guDo(t, e.teacher(), http.MethodPut, fmt.Sprintf("/courses/%d/assignments/%d/moderation/selections", guCourse, a.ID),
		map[string]interface{}{"user_id": guStudentA, "provisional_grade_id": uint(taPG["id"].(float64))})
	require.Equal(t, http.StatusOK, code, body)
	code, body = guDo(t, e.teacher(), http.MethodPost, fmt.Sprintf("/courses/%d/assignments/%d/moderation/publish", guCourse, a.ID), nil)
	require.Equal(t, http.StatusOK, code, body)
	assert.Contains(t, body, `"published":1`)
	sub, err := e.subSvc.GetByAssignmentAndUser(ctx, a.ID, guStudentA, 1)
	require.NoError(t, err)
	require.NotNil(t, sub.Score)
	assert.Equal(t, 6.0, *sub.Score)

	// Published: a second publish conflicts, and ordinary grading works.
	code, _ = guDo(t, e.teacher(), http.MethodPost, fmt.Sprintf("/courses/%d/assignments/%d/moderation/publish", guCourse, a.ID), nil)
	assert.Equal(t, http.StatusConflict, code)
	_, err = e.subSvc.Grade(ctx, a.ID, guStudentA, guTeacher, 1, "7")
	assert.NoError(t, err)
}

// G032: custom grade statuses — per-account, max 3, settable on a
// submission, never across tenants.
func TestGradingUI_CustomGradeStatuses(t *testing.T) {
	e := newGUEnv(t)
	admin := e.app(3, 1, "", true)
	var firstID uint
	for i, name := range []string{"Incomplete", "Redo", "Pending"} {
		code, body := guDo(t, admin, http.MethodPost, "/accounts/1/custom_grade_statuses",
			map[string]interface{}{"custom_grade_status": map[string]interface{}{"name": name, "color": "#FFCC00"}})
		require.Equal(t, http.StatusCreated, code, body)
		if i == 0 {
			var m map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(body), &m))
			firstID = uint(m["id"].(float64))
		}
	}
	code, _ := guDo(t, admin, http.MethodPost, "/accounts/1/custom_grade_statuses",
		map[string]interface{}{"custom_grade_status": map[string]interface{}{"name": "Fourth", "color": "#FFCC00"}})
	assert.Equal(t, http.StatusConflict, code)
	code, _ = guDo(t, admin, http.MethodPost, "/accounts/1/custom_grade_statuses",
		map[string]interface{}{"custom_grade_status": map[string]interface{}{"name": "Bad", "color": "red"}})
	assert.NotEqual(t, http.StatusCreated, code)
	// An admin of another account cannot list account 1's statuses.
	code, _ = guDo(t, e.app(3, 2, "", true), http.MethodGet, "/accounts/1/custom_grade_statuses", nil)
	assert.Equal(t, http.StatusNotFound, code)

	code, body := guDo(t, e.teacher(), http.MethodGet, fmt.Sprintf("/courses/%d/custom_grade_statuses", guCourse), nil)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, "Incomplete")

	a := e.assignment(t, &models.Assignment{})
	path := fmt.Sprintf("/courses/%d/assignments/%d/submissions/%d", guCourse, a.ID, guStudentA)
	code, body = guDo(t, e.teacher(), http.MethodPut, path, map[string]interface{}{"submission": map[string]interface{}{"custom_grade_status_id": firstID, "late_policy_status": "missing"}})
	require.Equal(t, http.StatusOK, code, body)
	assert.Contains(t, body, fmt.Sprintf(`"custom_grade_status_id":%d`, firstID))
	assert.Contains(t, body, `"missing":true`)

	// A status from another account is not found.
	foreign := &models.CustomGradeStatus{AccountID: 2, Name: "Theirs", Color: "#000000", AppliesToSubmissions: true}
	require.NoError(t, e.g.Create(foreign).Error)
	code, _ = guDo(t, e.teacher(), http.MethodPut, path, map[string]interface{}{"submission": map[string]interface{}{"custom_grade_status_id": foreign.ID}})
	assert.Equal(t, http.StatusNotFound, code)

	// Deleting a status clears it from submissions.
	code, _ = guDo(t, admin, http.MethodDelete, fmt.Sprintf("/accounts/1/custom_grade_statuses/%d", firstID), nil)
	require.Equal(t, http.StatusOK, code)
	sub, err := e.subSvc.GetByAssignmentAndUser(context.Background(), a.ID, guStudentA, 1)
	require.NoError(t, err)
	assert.Nil(t, sub.CustomGradeStatusID)
}

// G036: manual peer review assignment, reviewer names for the teacher,
// anonymous reviews for students, rubric-based review, feedback received.
func TestGradingUI_PeerReviews(t *testing.T) {
	e := newGUEnv(t)
	a := e.assignment(t, &models.Assignment{PeerReviewsEnabled: true, AnonymousPeerReviews: true})
	for _, s := range []uint{guStudentA, guStudentB} {
		code, body := submitText(t, e, a, s)
		require.Equal(t, http.StatusCreated, code, body)
	}
	manual := fmt.Sprintf("/courses/%d/assignments/%d/peer_reviews/manual", guCourse, a.ID)
	code, body := guDo(t, e.teacher(), http.MethodPost, manual, map[string]interface{}{"reviewer_id": guStudentA, "reviewee_id": guStudentB})
	require.Equal(t, http.StatusCreated, code, body)
	var pr map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(body), &pr))
	prID := uint(pr["id"].(float64))
	code, _ = guDo(t, e.teacher(), http.MethodPost, manual, map[string]interface{}{"reviewer_id": guStudentA, "reviewee_id": guStudentB})
	assert.Equal(t, http.StatusConflict, code, "duplicate")
	code, _ = guDo(t, e.teacher(), http.MethodPost, manual, map[string]interface{}{"reviewer_id": guStudentA, "reviewee_id": guStudentC})
	assert.Equal(t, http.StatusBadRequest, code, "C has not submitted")

	_, body = guDo(t, e.teacher(), http.MethodGet, fmt.Sprintf("/courses/%d/assignments/%d/peer_reviews", guCourse, a.ID), nil)
	assert.Contains(t, body, "Ada Student")
	assert.Contains(t, body, "Ben Student")

	// The anonymous reviewer does not learn whose work it is.
	_, body = guDo(t, e.student(guStudentA), http.MethodGet, fmt.Sprintf("/courses/%d/assignments/%d/peer_reviews/mine", guCourse, a.ID), nil)
	assert.NotContains(t, body, "Ben Student")
	assert.Contains(t, body, "Anonymous student")

	// Rubric-based review: the score is the sum of criteria.
	code, body = guDo(t, e.student(guStudentA), http.MethodPut, fmt.Sprintf("/peer_reviews/%d", prID),
		map[string]interface{}{"comments": "Clear <b>thesis</b>", "rubric_data": map[string]interface{}{"c1": map[string]interface{}{"points": 3}, "c2": map[string]interface{}{"points": 4.5}}})
	require.Equal(t, http.StatusOK, code, body)
	assert.Contains(t, body, `"score":7.5`)

	// The reviewee reads it, without the reviewer's name.
	code, body = guDo(t, e.student(guStudentB), http.MethodGet, fmt.Sprintf("/courses/%d/assignments/%d/peer_reviews/received", guCourse, a.ID), nil)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, "Clear thesis")
	assert.NotContains(t, body, "Ada Student")
	assert.Contains(t, body, "Anonymous reviewer")

	// Deleting from another course's URL is a 404.
	code, _ = guDo(t, e.teacher(), http.MethodDelete, fmt.Sprintf("/courses/%d/assignments/%d/peer_reviews/%d", guCourse2, a.ID, prID), nil)
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = guDo(t, e.teacher(), http.MethodDelete, fmt.Sprintf("/courses/%d/assignments/%d/peer_reviews/%d", guCourse, a.ID, prID), nil)
	assert.Equal(t, http.StatusOK, code)
}

// G027/G030/G036/G038: assignment authoring accepts and validates the
// new settings.
func TestGradingUI_AssignmentSettings(t *testing.T) {
	e := newGUEnv(t)
	due := time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339)
	code, body := guDo(t, e.teacher(), http.MethodPost, fmt.Sprintf("/courses/%d/assignments", guCourse), map[string]interface{}{
		"assignment": map[string]interface{}{
			"name": "Lab report", "submission_types": []string{"online_upload"},
			"allowed_attempts": 3, "allowed_extensions": []string{".PDF", "docx"},
			"moderated_grading": true, "final_grader_id": guTeacher,
			"anonymous_peer_reviews": true, "peer_reviews_due_at": due,
			"unlock_at": time.Now().UTC().Format(time.RFC3339), "lock_at": due,
		},
	})
	require.Equal(t, http.StatusCreated, code, body)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(body), &m))
	assert.Equal(t, float64(3), m["allowed_attempts"])
	assert.Equal(t, []interface{}{"pdf", "docx"}, m["allowed_extensions"])
	assert.Equal(t, float64(2), m["grader_count"], "Canvas default grader count")
	assert.Equal(t, true, m["grader_comments_visible_to_graders"])
	assert.Equal(t, true, m["anonymous_peer_reviews"])
	assert.NotNil(t, m["unlock_at"])
	id := uint(m["id"].(float64))

	// A student cannot be the final grader.
	code, _ = guDo(t, e.teacher(), http.MethodPut, fmt.Sprintf("/courses/%d/assignments/%d", guCourse, id),
		map[string]interface{}{"assignment": map[string]interface{}{"final_grader_id": guStudentA}})
	assert.Equal(t, http.StatusBadRequest, code)
	// A student annotation assignment needs a document of this course.
	code, body = guDo(t, e.teacher(), http.MethodPut, fmt.Sprintf("/courses/%d/assignments/%d", guCourse, id),
		map[string]interface{}{"assignment": map[string]interface{}{"submission_types": []string{"student_annotation"}}})
	assert.Equal(t, http.StatusBadRequest, code, body)
	// Clearing the attempt limit.
	code, body = guDo(t, e.teacher(), http.MethodPut, fmt.Sprintf("/courses/%d/assignments/%d", guCourse, id),
		map[string]interface{}{"assignment": map[string]interface{}{"allowed_attempts": -1}})
	require.Equal(t, http.StatusOK, code, body)
	assert.True(t, strings.Contains(body, `"allowed_attempts":null`), body)
}
