package handlers_test

// Grading-lane parity regressions (Canvas comparison audit). Each test
// names the bug id it locks. All data is made up.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
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
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/storage"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// gradingFixture wires a SubmissionHandler over mocks. Course 1 / tenant 1;
// assignment 1 is a published assignment of course 1; assignment 5 lives
// in course 2 of the same tenant; assignment 9 is in another tenant (its
// lookup under tenant 1 misses).
type gradingFixture struct {
	app            *fiber.App
	handler        *handlers.SubmissionHandler
	submissionRepo *mocks.MockSubmissionRepository
	assignmentRepo *mocks.MockAssignmentRepository
	enrollmentRepo *mocks.MockEnrollmentRepository
	commentRepo    *mocks.MockSubmissionCommentRepository
	userRepo       *mocks.MockUserRepository
	attachmentRepo *mocks.MockAttachmentRepository
}

func newGradingFixture(t *testing.T, callerID uint, enrollmentType string, assignment1 *models.Assignment) *gradingFixture {
	t.Helper()
	f := &gradingFixture{
		submissionRepo: new(mocks.MockSubmissionRepository),
		assignmentRepo: new(mocks.MockAssignmentRepository),
		enrollmentRepo: new(mocks.MockEnrollmentRepository),
		commentRepo:    new(mocks.MockSubmissionCommentRepository),
		userRepo:       new(mocks.MockUserRepository),
		attachmentRepo: new(mocks.MockAttachmentRepository),
	}
	if assignment1 == nil {
		assignment1 = &models.Assignment{ID: 1, CourseID: 1, Name: "Essay", WorkflowState: models.AssignmentPublished,
			SubmissionTypes: "online_text_entry,online_upload", PostPolicy: "automatic"}
	}
	f.assignmentRepo.On("FindByID", mock.Anything, uint(1), uint(1)).Return(assignment1, nil).Maybe()
	f.assignmentRepo.On("FindByID", mock.Anything, uint(5), uint(1)).
		Return(&models.Assignment{ID: 5, CourseID: 2, Name: "Other course", WorkflowState: models.AssignmentPublished}, nil).Maybe()
	f.assignmentRepo.On("FindByID", mock.Anything, uint(9), uint(1)).Return(nil, errors.New("not found")).Maybe()
	f.enrollmentRepo.On("FindByUserAndCourse", mock.Anything, uint(2), uint(1), uint(1)).
		Return(&models.Enrollment{UserID: 2, CourseID: 1, Type: "StudentEnrollment", WorkflowState: "active"}, nil).Maybe()
	f.enrollmentRepo.On("FindByUserAndCourse", mock.Anything, uint(3), uint(1), uint(1)).
		Return(&models.Enrollment{UserID: 3, CourseID: 1, Type: "TeacherEnrollment", WorkflowState: "active"}, nil).Maybe()

	svc := service.NewSubmissionService(f.submissionRepo, f.assignmentRepo, f.enrollmentRepo,
		new(mocks.MockLatePolicyRepository), new(mocks.MockCourseRepository),
		new(mocks.MockGradingPeriodGroupRepository), new(mocks.MockGradingPeriodRepository), nil)
	f.handler = handlers.NewSubmissionHandler(svc, f.commentRepo, f.attachmentRepo, f.userRepo, f.assignmentRepo, nil, nil, nil, nil, nil)

	f.app = testutil.SetupTestApp()
	f.app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", callerID)
		c.Locals("account_id", uint(1))
		c.Locals("enrollment_type", enrollmentType)
		return c.Next()
	})
	f.app.Use(middleware.PaginationParams())
	base := "/api/v1/courses/:course_id"
	f.app.Get(base+"/submissions", f.handler.ListCourseSubmissions)
	f.app.Post(base+"/submissions/bulk_grade", f.handler.BulkGrade)
	g := base + "/assignments/:assignment_id"
	f.app.Get(g+"/submissions", f.handler.ListSubmissions)
	f.app.Post(g+"/submissions", f.handler.CreateSubmission)
	f.app.Post(g+"/submissions/self/files", f.handler.UploadSubmissionFile)
	f.app.Get(g+"/submissions/:user_id", f.handler.GetSubmission)
	f.app.Put(g+"/submissions/:user_id", f.handler.UpdateSubmission)
	f.app.Get(g+"/submissions/:user_id/comments", f.handler.ListSubmissionComments)
	f.app.Post(g+"/submissions/:user_id/comments", f.handler.CreateSubmissionComment)
	f.app.Get(g+"/submissions/:user_id/files/:file_id/download", f.handler.DownloadSubmissionFile)
	f.app.Post(base+"/assignments/:id/post_grades", f.handler.PostGrades)
	f.app.Post(base+"/assignments/:id/hide_grades", f.handler.HideGrades)
	return f
}

func fptr(v float64) *float64 { return &v }
func sptr(v string) *string   { return &v }

// B009: listing an assignment of another course (same tenant) or another
// tenant is a 404, and no submission is read.
func TestGradingParity_B009_ListSubmissionsRequiresCourseTie(t *testing.T) {
	for _, id := range []string{"5", "9"} {
		f := newGradingFixture(t, 7, "TeacherEnrollment", nil)
		resp := testutil.MakeRequest(f.app, http.MethodGet, "/api/v1/courses/1/assignments/"+id+"/submissions", nil)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "assignment %s", id)
		f.submissionRepo.AssertNotCalled(t, "ListByAssignmentID", mock.Anything, mock.Anything, mock.Anything)
	}
}

// B011: post/hide grades on a foreign assignment is refused before any write.
func TestGradingParity_B011_PostHideGradesRequireCourseTie(t *testing.T) {
	for _, path := range []string{"/api/v1/courses/1/assignments/5/post_grades", "/api/v1/courses/1/assignments/9/hide_grades"} {
		f := newGradingFixture(t, 7, "TeacherEnrollment", nil)
		resp := testutil.MakeRequest(f.app, http.MethodPost, path, nil)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, path)
		f.submissionRepo.AssertNotCalled(t, "PostGradesByAssignment", mock.Anything, mock.Anything, mock.Anything)
	}
}

// B015: grading another course's assignment, or a user who isn't a
// student of the course, is a 404 with no write.
func TestGradingParity_B015_UpdateSubmissionRequiresCourseAndStudent(t *testing.T) {
	body := func() io.Reader {
		return testutil.JSONBody(map[string]interface{}{"submission": map[string]interface{}{"posted_grade": "10"}})
	}
	f := newGradingFixture(t, 7, "TeacherEnrollment", nil)
	resp := testutil.MakeRequest(f.app, http.MethodPut, "/api/v1/courses/1/assignments/5/submissions/2", body())
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "cross-course assignment")

	// user 3 is a teacher of the course, user 4 has no enrollment.
	f.enrollmentRepo.On("FindByUserAndCourse", mock.Anything, uint(4), uint(1), uint(1)).Return(nil, errors.New("not found"))
	for _, uid := range []string{"3", "4"} {
		resp = testutil.MakeRequest(f.app, http.MethodPut, "/api/v1/courses/1/assignments/1/submissions/"+uid, body())
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "user %s", uid)
	}
	f.submissionRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	f.submissionRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestGradingParity_B015_BulkGradeRejectsForeignEntries(t *testing.T) {
	f := newGradingFixture(t, 7, "TeacherEnrollment", nil)
	resp := testutil.MakeRequest(f.app, http.MethodPost, "/api/v1/courses/1/submissions/bulk_grade", testutil.JSONBody(map[string]interface{}{
		"grade_data": []map[string]interface{}{
			{"assignment_id": 5, "user_id": 2, "posted_grade": "10"},
			{"assignment_id": 9, "user_id": 2, "posted_grade": "10"},
		},
	}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var out struct {
		Results []struct {
			Error string `json:"error"`
		} `json:"results"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.Len(t, out.Results, 2)
	for _, r := range out.Results {
		assert.NotEmpty(t, r.Error)
	}
	f.submissionRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	f.submissionRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

// B016: a classmate can neither read nor post comments on another
// student's submission.
func TestGradingParity_B016_ClassmateCannotUseComments(t *testing.T) {
	f := newGradingFixture(t, 8, "StudentEnrollment", nil)
	f.submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(2), uint(1)).
		Return(&models.Submission{ID: 11, AssignmentID: 1, UserID: 2}, nil)

	resp := testutil.MakeRequest(f.app, http.MethodGet, "/api/v1/courses/1/assignments/1/submissions/2/comments", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp = testutil.MakeRequest(f.app, http.MethodPost, "/api/v1/courses/1/assignments/1/submissions/2/comments",
		testutil.JSONBody(map[string]interface{}{"comment": map[string]interface{}{"text_comment": "hi"}}))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	f.commentRepo.AssertNotCalled(t, "ListBySubmissionID", mock.Anything, mock.Anything, mock.Anything)
	f.commentRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// B016 (owner side): before posting, the student sees only their own
// comments, never the grader's feedback.
func TestGradingParity_B016_OwnerSeesOnlyOwnCommentsUntilPosted(t *testing.T) {
	f := newGradingFixture(t, 2, "StudentEnrollment", nil)
	f.submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(2), uint(1)).
		Return(&models.Submission{ID: 11, AssignmentID: 1, UserID: 2, Score: fptr(7)}, nil)
	f.commentRepo.On("ListBySubmissionID", mock.Anything, uint(11), uint(1)).Return([]models.SubmissionComment{
		{ID: 1, SubmissionID: 11, AuthorID: 2, Comment: "my question"},
		{ID: 2, SubmissionID: 11, AuthorID: 7, Comment: "teacher feedback"},
	}, nil)
	f.userRepo.On("FindByID", mock.Anything, mock.Anything, uint(1)).Return(&models.User{Name: "Jane Doe"}, nil)

	resp := testutil.MakeRequest(f.app, http.MethodGet, "/api/v1/courses/1/assignments/1/submissions/2/comments", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	items, err := testutil.ParseJSONArray(resp)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "my question", items[0]["comment"])
}

// B012 / B088 / B109: an unposted grade is withheld from the student and
// posted_at is serialized so the UI can tell.
func TestGradingParity_B012_StudentGradeHiddenUntilPosted(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name      string
		postedAt  *time.Time
		wantScore interface{}
	}{
		{"unposted", nil, nil},
		{"posted", &now, float64(7)},
	}
	for _, tc := range cases {
		f := newGradingFixture(t, 2, "StudentEnrollment", nil)
		f.submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(2), uint(1)).Return(&models.Submission{
			ID: 11, AssignmentID: 1, UserID: 2, Score: fptr(7), Grade: sptr("7"), SubmittedAt: &now,
			WorkflowState: models.SubmissionGraded, PostedAt: tc.postedAt,
		}, nil)
		resp := testutil.MakeRequest(f.app, http.MethodGet, "/api/v1/courses/1/assignments/1/submissions/2", nil)
		require.Equal(t, http.StatusOK, resp.StatusCode, tc.name)
		m, err := testutil.ParseJSONMap(resp)
		require.NoError(t, err)
		assert.Equal(t, tc.wantScore, m["score"], tc.name)
		_, hasPostedAt := m["posted_at"]
		assert.True(t, hasPostedAt, "posted_at must be serialized (%s)", tc.name)
		if tc.postedAt == nil {
			assert.Nil(t, m["grade"])
			assert.Equal(t, "submitted", m["workflow_state"], "graded state must not leak")
		}
	}

	// Excusing is part of the grade: hidden until posted as well.
	for _, tc := range []struct {
		name        string
		postedAt    *time.Time
		wantExcused bool
	}{{"unposted excused", nil, false}, {"posted excused", &now, true}} {
		f := newGradingFixture(t, 2, "StudentEnrollment", nil)
		f.submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(2), uint(1)).Return(&models.Submission{
			ID: 11, AssignmentID: 1, UserID: 2, Excused: true, WorkflowState: models.SubmissionUnsubmitted, PostedAt: tc.postedAt,
		}, nil)
		resp := testutil.MakeRequest(f.app, http.MethodGet, "/api/v1/courses/1/assignments/1/submissions/2", nil)
		require.Equal(t, http.StatusOK, resp.StatusCode, tc.name)
		m, err := testutil.ParseJSONMap(resp)
		require.NoError(t, err)
		assert.Equal(t, tc.wantExcused, m["excused"], tc.name)
	}
}

// B012: under a manual post policy grading neither posts the submission
// nor (therefore) notifies; under automatic it posts immediately.
func TestGradingParity_B012_GradePostsOnlyUnderAutomaticPolicy(t *testing.T) {
	for _, policy := range []string{"automatic", "manual"} {
		f := newGradingFixture(t, 7, "TeacherEnrollment", &models.Assignment{
			ID: 1, CourseID: 1, WorkflowState: models.AssignmentPublished, PostPolicy: policy, PointsPossible: fptr(10),
		})
		f.submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(2), uint(1)).
			Return(&models.Submission{ID: 11, AssignmentID: 1, UserID: 2}, nil)
		var saved *models.Submission
		f.submissionRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.Submission")).
			Run(func(args mock.Arguments) { saved = args.Get(1).(*models.Submission) }).Return(nil)

		resp := testutil.MakeRequest(f.app, http.MethodPut, "/api/v1/courses/1/assignments/1/submissions/2",
			testutil.JSONBody(map[string]interface{}{"submission": map[string]interface{}{"posted_grade": "7"}}))
		require.Equal(t, http.StatusOK, resp.StatusCode, policy)
		require.NotNil(t, saved)
		if policy == "manual" {
			assert.Nil(t, saved.PostedAt, "manual policy keeps the grade hidden")
		} else {
			assert.NotNil(t, saved.PostedAt, "automatic policy posts on grade")
		}
	}
}

// B113: an empty posted_grade clears the grade.
func TestGradingParity_B113_EmptyPostedGradeClears(t *testing.T) {
	f := newGradingFixture(t, 7, "TeacherEnrollment", nil)
	now := time.Now()
	f.submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(2), uint(1)).Return(&models.Submission{
		ID: 11, AssignmentID: 1, UserID: 2, Score: fptr(7), Grade: sptr("7"), EnteredScore: fptr(7),
		SubmittedAt: &now, WorkflowState: models.SubmissionGraded,
	}, nil)
	f.submissionRepo.On("Update", mock.Anything, mock.MatchedBy(func(s *models.Submission) bool {
		return s.Score == nil && s.Grade == nil && s.EnteredScore == nil && s.WorkflowState == models.SubmissionSubmitted
	})).Return(nil).Once()

	resp := testutil.MakeRequest(f.app, http.MethodPut, "/api/v1/courses/1/assignments/1/submissions/2",
		testutil.JSONBody(map[string]interface{}{"submission": map[string]interface{}{"posted_grade": ""}}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	m, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	assert.Nil(t, m["score"])
	f.submissionRepo.AssertExpectations(t)
}

// B089: a student's own rows are filtered first, then paginated, and the
// header total is the student's count, not the course's.
func TestGradingParity_B089_StudentCourseSubmissionsPaginateAfterFilter(t *testing.T) {
	f := newGradingFixture(t, 2, "StudentEnrollment", nil)
	own := make([]models.Submission, 30)
	for i := range own {
		own[i] = models.Submission{ID: uint(100 + i), AssignmentID: uint(i + 1), UserID: 2}
	}
	f.submissionRepo.On("ListByUserAndCourse", mock.Anything, uint(2), uint(1)).Return(own, nil)

	resp := testutil.MakeRequest(f.app, http.MethodGet, "/api/v1/courses/1/submissions?user_id=self&page=2&per_page=25", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	items, err := testutil.ParseJSONArray(resp)
	require.NoError(t, err)
	assert.Len(t, items, 5)
	f.submissionRepo.AssertNotCalled(t, "BulkListByCourse", mock.Anything, mock.Anything, mock.Anything)
}

// B111: observers can't submit; unpublished, foreign and wrong-type
// submissions are refused.
func TestGradingParity_B111_SubmitGuards(t *testing.T) {
	text := func() io.Reader {
		return testutil.JSONBody(map[string]interface{}{"submission": map[string]interface{}{
			"submission_type": "online_text_entry", "body": "work"}})
	}

	f := newGradingFixture(t, 2, "ObserverEnrollment", nil)
	resp := testutil.MakeRequest(f.app, http.MethodPost, "/api/v1/courses/1/assignments/1/submissions", text())
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "observer")

	f = newGradingFixture(t, 2, "StudentEnrollment", &models.Assignment{ID: 1, CourseID: 1, WorkflowState: models.AssignmentUnpublished, SubmissionTypes: "online_text_entry"})
	resp = testutil.MakeRequest(f.app, http.MethodPost, "/api/v1/courses/1/assignments/1/submissions", text())
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "unpublished")

	for _, id := range []string{"5", "9"} {
		f = newGradingFixture(t, 2, "StudentEnrollment", nil)
		resp = testutil.MakeRequest(f.app, http.MethodPost, "/api/v1/courses/1/assignments/"+id+"/submissions", text())
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "foreign assignment %s", id)
	}

	f = newGradingFixture(t, 2, "StudentEnrollment", &models.Assignment{ID: 1, CourseID: 1, WorkflowState: models.AssignmentPublished, SubmissionTypes: "on_paper"})
	resp = testutil.MakeRequest(f.app, http.MethodPost, "/api/v1/courses/1/assignments/1/submissions", text())
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "type not accepted")
	f.submissionRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)

	// A website-URL submission must be an http(s) link: a javascript: URL
	// would run in the grader's session when clicked in SpeedGrader.
	for _, bad := range []string{"javascript:alert(document.cookie)", "data:text/html,<script>x</script>", "example.test/no-scheme"} {
		f = newGradingFixture(t, 2, "StudentEnrollment", &models.Assignment{ID: 1, CourseID: 1, WorkflowState: models.AssignmentPublished, SubmissionTypes: "online_url"})
		resp = testutil.MakeRequest(f.app, http.MethodPost, "/api/v1/courses/1/assignments/1/submissions", testutil.JSONBody(map[string]interface{}{
			"submission": map[string]interface{}{"submission_type": "online_url", "url": bad}}))
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, bad)
		f.submissionRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	}
}

// B013: a student may only attach files they uploaded themselves.
func TestGradingParity_B013_CannotAttachSomeoneElsesFile(t *testing.T) {
	f := newGradingFixture(t, 2, "StudentEnrollment", nil)
	f.attachmentRepo.On("FindByID", mock.Anything, uint(40), uint(1)).
		Return(&models.Attachment{ID: 40, UserID: 8, ContextType: "User", ContextID: 8}, nil)
	resp := testutil.MakeRequest(f.app, http.MethodPost, "/api/v1/courses/1/assignments/1/submissions",
		testutil.JSONBody(map[string]interface{}{"submission": map[string]interface{}{
			"submission_type": "online_upload", "file_ids": []uint{40}}}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	f.submissionRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// B013: a student can upload a file for a submission (personal context),
// and only the owner/staff can download it through the submission.
func TestGradingParity_B013_StudentUploadAndSubmissionScopedDownload(t *testing.T) {
	f := newGradingFixture(t, 2, "StudentEnrollment", nil)
	fileService := service.NewFileServiceWithBackend(new(mocks.MockFolderRepository), f.attachmentRepo, storage.NewLocalBackend(t.TempDir()))
	f.handler.SetFileService(fileService)
	var created *models.Attachment
	f.attachmentRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Attachment")).
		Run(func(args mock.Arguments) {
			created = args.Get(1).(*models.Attachment)
			created.ID = 77
		}).Return(nil)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Disposition", `form-data; name="file"; filename="essay.txt"`)
	hdr.Set("Content-Type", "text/plain")
	part, err := w.CreatePart(hdr)
	require.NoError(t, err)
	_, _ = part.Write([]byte("my essay text"))
	require.NoError(t, w.Close())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/courses/1/assignments/1/submissions/self/files", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := f.app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.NotNil(t, created)
	assert.Equal(t, "User", created.ContextType)
	assert.Equal(t, uint(2), created.ContextID)

	// A classmate (user 8) cannot download it through the submission route.
	other := newGradingFixture(t, 8, "StudentEnrollment", nil)
	other.handler.SetFileService(fileService)
	attJSON := `[{"id":77}]`
	other.submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(2), uint(1)).
		Return(&models.Submission{ID: 11, AssignmentID: 1, UserID: 2, Attachments: &attJSON}, nil)
	other.submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(8), uint(1)).
		Return(&models.Submission{ID: 12, AssignmentID: 1, UserID: 8}, nil)
	resp = testutil.MakeRequest(other.app, http.MethodGet, "/api/v1/courses/1/assignments/1/submissions/2/files/77/download", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// The teacher can.
	teacher := newGradingFixture(t, 7, "TeacherEnrollment", nil)
	teacher.handler.SetFileService(fileService)
	teacher.submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(2), uint(1)).
		Return(&models.Submission{ID: 11, AssignmentID: 1, UserID: 2, Attachments: &attJSON}, nil)
	f.attachmentRepo.On("FindByID", mock.Anything, uint(77), uint(1)).Return(created, nil)
	teacher.attachmentRepo = f.attachmentRepo
	resp = testutil.MakeRequest(teacher.app, http.MethodGet, "/api/v1/courses/1/assignments/1/submissions/2/files/77/download", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "my essay text", string(body))

	// A group member whose own (group) submission carries the same file
	// can download it, although the URL names the uploader.
	member := newGradingFixture(t, 9, "StudentEnrollment", nil)
	member.handler.SetFileService(fileService)
	member.submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(2), uint(1)).
		Return(&models.Submission{ID: 11, AssignmentID: 1, UserID: 2, Attachments: &attJSON}, nil)
	member.submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(9), uint(1)).
		Return(&models.Submission{ID: 13, AssignmentID: 1, UserID: 9, Attachments: &attJSON}, nil)
	resp = testutil.MakeRequest(member.app, http.MethodGet, "/api/v1/courses/1/assignments/1/submissions/2/files/77/download", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// SpeedGrader
// ---------------------------------------------------------------------------

func newSpeedGraderApp(t *testing.T, assignmentRepo *mocks.MockAssignmentRepository, submissionRepo *mocks.MockSubmissionRepository, commentRepo *mocks.MockSubmissionCommentRepository, enrollmentRepo *mocks.MockEnrollmentRepository) *fiber.App {
	t.Helper()
	svc := service.NewSpeedGraderService(submissionRepo, commentRepo, assignmentRepo, enrollmentRepo, nil)
	h := handlers.NewSpeedGraderHandler(svc, nil)
	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(7))
		c.Locals("account_id", uint(1))
		c.Locals("enrollment_type", "TeacherEnrollment")
		return c.Next()
	})
	app.Get("/courses/:course_id/assignments/:assignment_id/speedgrader", h.GetSpeedGraderData)
	app.Get("/courses/:course_id/assignments/:assignment_id/speedgrader/submissions/:user_id", h.GetStudentSubmission)
	return app
}

// B010: the single-submission SpeedGrader read is tenant- and course-scoped.
func TestGradingParity_B010_SpeedGraderStudentSubmissionScoped(t *testing.T) {
	assignmentRepo := new(mocks.MockAssignmentRepository)
	submissionRepo := new(mocks.MockSubmissionRepository)
	commentRepo := new(mocks.MockSubmissionCommentRepository)
	assignmentRepo.On("FindByID", mock.Anything, uint(9), uint(1)).Return(nil, errors.New("not found"))
	assignmentRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(&models.Assignment{ID: 5, CourseID: 2}, nil)
	app := newSpeedGraderApp(t, assignmentRepo, submissionRepo, commentRepo, new(mocks.MockEnrollmentRepository))

	for _, id := range []string{"9", "5"} {
		resp := testutil.MakeRequest(app, http.MethodGet, "/courses/1/assignments/"+id+"/speedgrader/submissions/2", nil)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "assignment %s", id)
	}
	submissionRepo.AssertNotCalled(t, "FindByAssignmentAndUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	commentRepo.AssertNotCalled(t, "ListBySubmissionID", mock.Anything, mock.Anything, mock.Anything)
}

// B119: on an anonymous assignment a student's own comment carries the
// anonymous label, never the real name or user id.
func TestGradingParity_B119_AnonymousCommentAuthorMasked(t *testing.T) {
	assignmentRepo := new(mocks.MockAssignmentRepository)
	submissionRepo := new(mocks.MockSubmissionRepository)
	commentRepo := new(mocks.MockSubmissionCommentRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)
	assignmentRepo.On("FindByID", mock.Anything, uint(1), uint(1)).
		Return(&models.Assignment{ID: 1, CourseID: 1, AnonymousGrading: true, WorkflowState: models.AssignmentPublished}, nil)
	enrollmentRepo.On("ListByCourseID", mock.Anything, uint(1), uint(0), mock.Anything).Return(&repository.PaginatedResult[models.Enrollment]{
		Items: []models.Enrollment{
			{UserID: 2, CourseID: 1, Type: "StudentEnrollment", User: &models.User{ID: 2, Name: "Jane Doe"}},
			{UserID: 7, CourseID: 1, Type: "TeacherEnrollment", User: &models.User{ID: 7, Name: "Pat Teacher"}},
		},
	}, nil)
	submissionRepo.On("ListByAssignmentID", mock.Anything, uint(1), mock.Anything).Return(&repository.PaginatedResult[models.Submission]{
		Items: []models.Submission{{ID: 11, AssignmentID: 1, UserID: 2}},
	}, nil)
	commentRepo.On("ListBySubmissionID", mock.Anything, uint(11), uint(1)).Return([]models.SubmissionComment{
		{ID: 1, SubmissionID: 11, AuthorID: 2, Comment: "please regrade"},
		{ID: 2, SubmissionID: 11, AuthorID: 7, Comment: "sure"},
	}, nil)
	app := newSpeedGraderApp(t, assignmentRepo, submissionRepo, commentRepo, enrollmentRepo)

	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/1/assignments/1/speedgrader", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	raw, _ := io.ReadAll(resp.Body)
	assert.NotContains(t, string(raw), "Jane Doe")
	var out struct {
		Students []struct {
			UserName string                   `json:"user_name"`
			Comments []map[string]interface{} `json:"comments"`
		} `json:"students"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	require.Len(t, out.Students, 1)
	require.Len(t, out.Students[0].Comments, 2)
	assert.Equal(t, out.Students[0].UserName, out.Students[0].Comments[0]["author_name"])
	assert.Nil(t, out.Students[0].Comments[0]["author_id"])
	assert.Equal(t, "Pat Teacher", out.Students[0].Comments[1]["author_name"])
}

// ---------------------------------------------------------------------------
// Peer reviews
// ---------------------------------------------------------------------------

// B018: only the assigned reviewer can complete a review.
func TestGradingParity_B018_OnlyAssignedReviewerCanSubmit(t *testing.T) {
	prRepo := new(mocks.MockPeerReviewRepository)
	prRepo.On("FindByID", mock.Anything, uint(42), uint(1)).
		Return(&models.PeerReview{ID: 42, AssignmentID: 1, ReviewerID: 10, RevieweeID: 11, WorkflowState: "assigned"}, nil)
	svc := service.NewPeerReviewService(prRepo, new(mocks.MockSubmissionRepository), new(mocks.MockEnrollmentRepository))
	h := handlers.NewPeerReviewHandler(svc, nil)

	run := func(caller uint) int {
		app := testutil.SetupTestApp()
		app.Use(authStub(caller, 1))
		app.Put("/peer_reviews/:review_id", h.SubmitPeerReview)
		resp := testutil.MakeRequest(app, http.MethodPut, "/peer_reviews/42",
			testutil.JSONBody(map[string]interface{}{"score": 3, "comments": "ok"}))
		return resp.StatusCode
	}
	assert.Equal(t, http.StatusNotFound, run(99), "a non-reviewer must be refused")
	prRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)

	prRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.PeerReview")).Return(nil)
	assert.Equal(t, http.StatusOK, run(10))
}

// B117: the reviewer's list carries the work to review (no grade).
func TestGradingParity_B117_MyPeerReviewsIncludeSubmission(t *testing.T) {
	prRepo := new(mocks.MockPeerReviewRepository)
	subRepo := new(mocks.MockSubmissionRepository)
	assignmentRepo := new(mocks.MockAssignmentRepository)
	prRepo.On("ListByReviewer", mock.Anything, uint(1), uint(10), uint(1)).
		Return([]models.PeerReview{{ID: 42, AssignmentID: 1, SubmissionID: 11, ReviewerID: 10, RevieweeID: 11}}, nil)
	subRepo.On("FindByID", mock.Anything, uint(11), uint(1)).
		Return(&models.Submission{ID: 11, AssignmentID: 1, UserID: 11, Body: sptr("<p>the essay</p>"), Score: fptr(9)}, nil)
	assignmentRepo.On("FindByID", mock.Anything, uint(1), uint(1)).Return(&models.Assignment{ID: 1, CourseID: 1}, nil)
	svc := service.NewPeerReviewService(prRepo, subRepo, new(mocks.MockEnrollmentRepository))
	h := handlers.NewPeerReviewHandler(svc, nil)
	h.SetAssignmentService(service.NewAssignmentService(assignmentRepo))

	app := testutil.SetupTestApp()
	app.Use(authStub(10, 1))
	app.Get("/courses/:course_id/assignments/:id/peer_reviews/mine", h.ListMyPeerReviews)
	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/1/assignments/1/peer_reviews/mine", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	items, err := testutil.ParseJSONArray(resp)
	require.NoError(t, err)
	require.Len(t, items, 1)
	sub, ok := items[0]["submission"].(map[string]interface{})
	require.True(t, ok, "submission must be embedded")
	assert.Equal(t, "<p>the essay</p>", sub["body"])
	_, hasScore := sub["score"]
	assert.False(t, hasScore, "the reviewee's grade must not be exposed")

	// Wrong course → 404.
	resp = testutil.MakeRequest(app, http.MethodGet, "/courses/2/assignments/1/peer_reviews/mine", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// Rubric assessments
// ---------------------------------------------------------------------------

// B017: a student sees only their own (posted) assessment, not
// classmates'.
func TestGradingParity_B017_StudentSeesOnlyOwnRubricAssessments(t *testing.T) {
	assocRepo := new(mocks.MockRubricAssociationRepository)
	assessRepo := new(mocks.MockRubricAssessmentRepository)
	assocRepo.On("FindByID", mock.Anything, uint(3)).Return(&models.RubricAssociation{
		ID: 3, RubricID: 1, AssociationID: 1, AssociationType: "Assignment", ContextType: "Course", ContextID: 1,
	}, nil)
	assessRepo.On("ListByAssociationID", mock.Anything, uint(3), mock.Anything).Return(&repository.PaginatedResult[models.RubricAssessment]{
		Items: []models.RubricAssessment{
			{ID: 1, RubricAssociationID: 3, UserID: 2, AssessorID: 7, AssessmentType: "grading"},
			{ID: 2, RubricAssociationID: 3, UserID: 8, AssessorID: 7, AssessmentType: "grading"},
		},
		TotalCount: 2, Page: 1, PerPage: 10,
	}, nil)
	subRepo := new(mocks.MockSubmissionRepository)
	now := time.Now()
	subRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(2), uint(1)).
		Return(&models.Submission{ID: 11, AssignmentID: 1, UserID: 2, PostedAt: &now}, nil)
	subSvc := service.NewSubmissionService(subRepo, new(mocks.MockAssignmentRepository), new(mocks.MockEnrollmentRepository),
		new(mocks.MockLatePolicyRepository), new(mocks.MockCourseRepository), new(mocks.MockGradingPeriodGroupRepository), new(mocks.MockGradingPeriodRepository), nil)

	h := handlers.NewRubricAssessmentHandler(service.NewRubricService(new(mocks.MockRubricRepository), assocRepo, assessRepo))
	h.SetVisibilityDeps(subSvc, nil)
	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(2))
		c.Locals("account_id", uint(1))
		c.Locals("enrollment_type", "StudentEnrollment")
		return c.Next()
	})
	app.Use(middleware.PaginationParams())
	app.Get("/courses/:course_id/rubric_associations/:association_id/rubric_assessments", h.ListAssessments)

	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/1/rubric_associations/3/rubric_assessments", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	items, err := testutil.ParseJSONArray(resp)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.EqualValues(t, 2, items[0]["user_id"])
}

// ---------------------------------------------------------------------------
// Assignments, overrides, accommodations
// ---------------------------------------------------------------------------

func assignmentApp(t *testing.T, repo *mocks.MockAssignmentRepository, enrollmentType string) *fiber.App {
	t.Helper()
	h := handlers.NewAssignmentHandler(service.NewAssignmentService(repo))
	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(2))
		c.Locals("account_id", uint(1))
		c.Locals("enrollment_type", enrollmentType)
		return c.Next()
	})
	app.Use(middleware.PaginationParams())
	app.Get("/courses/:course_id/assignments", h.ListAssignments)
	app.Get("/courses/:course_id/assignments/:id", h.GetAssignment)
	app.Put("/courses/:course_id/assignments/:id", h.UpdateAssignment)
	return app
}

// B116: students never see unpublished assignments.
func TestGradingParity_B116_StudentsDontSeeUnpublishedAssignments(t *testing.T) {
	repo := new(mocks.MockAssignmentRepository)
	repo.On("ListByCourseID", mock.Anything, uint(1), mock.Anything).Return(&repository.PaginatedResult[models.Assignment]{
		Items: []models.Assignment{
			{ID: 1, CourseID: 1, Name: "Live", WorkflowState: models.AssignmentPublished},
			{ID: 2, CourseID: 1, Name: "Draft", WorkflowState: models.AssignmentUnpublished},
		},
		TotalCount: 2, Page: 1, PerPage: 100000,
	}, nil)
	repo.On("FindByID", mock.Anything, uint(2), uint(1)).
		Return(&models.Assignment{ID: 2, CourseID: 1, Name: "Draft", WorkflowState: models.AssignmentUnpublished}, nil)

	student := assignmentApp(t, repo, "StudentEnrollment")
	resp := testutil.MakeRequest(student, http.MethodGet, "/courses/1/assignments", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	items, err := testutil.ParseJSONArray(resp)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Live", items[0]["name"])
	resp = testutil.MakeRequest(student, http.MethodGet, "/courses/1/assignments/2", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	teacher := assignmentApp(t, repo, "TeacherEnrollment")
	resp = testutil.MakeRequest(teacher, http.MethodGet, "/courses/1/assignments/2", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// B114: an explicit null clears due_at and turns off group assignment.
func TestGradingParity_B114_ExplicitNullClearsFields(t *testing.T) {
	repo := new(mocks.MockAssignmentRepository)
	due := time.Now().Add(48 * time.Hour)
	gc := uint(4)
	repo.On("FindByID", mock.Anything, uint(1), uint(1)).Return(&models.Assignment{
		ID: 1, CourseID: 1, Name: "Essay", WorkflowState: models.AssignmentPublished, DueAt: &due, GroupCategoryID: &gc,
	}, nil)
	repo.On("Update", mock.Anything, mock.MatchedBy(func(a *models.Assignment) bool {
		return a.DueAt == nil && a.GroupCategoryID == nil && a.Name == "Essay v2"
	})).Return(nil).Once()

	app := assignmentApp(t, repo, "TeacherEnrollment")
	resp := testutil.MakeRequest(app, http.MethodPut, "/courses/1/assignments/1", testutil.JSONBody(map[string]interface{}{
		"assignment": map[string]interface{}{"name": "Essay v2", "due_at": nil, "group_category_id": nil},
	}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	repo.AssertExpectations(t)
}

func overrideApp(t *testing.T) (*fiber.App, *mocks.MockAssignmentOverrideRepository, *mocks.MockAssignmentOverrideStudentRepository, *mocks.MockEnrollmentRepository) {
	t.Helper()
	overrideRepo := new(mocks.MockAssignmentOverrideRepository)
	studentRepo := new(mocks.MockAssignmentOverrideStudentRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)
	assignmentRepo := new(mocks.MockAssignmentRepository)
	assignmentRepo.On("FindByID", mock.Anything, uint(1), uint(1)).Return(&models.Assignment{ID: 1, CourseID: 1}, nil).Maybe()
	assignmentRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(&models.Assignment{ID: 5, CourseID: 2}, nil).Maybe()
	h := handlers.NewAssignmentOverrideHandler(
		service.NewOverrideService(overrideRepo, studentRepo, enrollmentRepo, new(mocks.MockSectionRepository)),
		service.NewAssignmentService(assignmentRepo))
	app := testutil.SetupTestApp()
	app.Use(overrideAuthStub(1, "TeacherEnrollment"))
	app.Get("/courses/:course_id/assignments/:assignment_id/overrides", h.ListOverrides)
	app.Post("/courses/:course_id/assignments/:assignment_id/overrides", h.CreateOverride)
	return app, overrideRepo, studentRepo, enrollmentRepo
}

// B019: list/create overrides require the course parent-tie.
func TestGradingParity_B019_OverrideListCreateRequireCourseTie(t *testing.T) {
	app, overrideRepo, _, _ := overrideApp(t)
	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/1/assignments/5/overrides", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp = testutil.MakeRequest(app, http.MethodPost, "/courses/1/assignments/5/overrides",
		testutil.JSONBody(map[string]interface{}{"assignment_override": map[string]interface{}{"title": "x"}}))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	overrideRepo.AssertNotCalled(t, "ListByAssignmentID", mock.Anything, mock.Anything)
	overrideRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// B112: a failed student insert is reported (and rolled back), never
// echoed as saved.
func TestGradingParity_B112_OverrideStudentConflictReported(t *testing.T) {
	app, overrideRepo, studentRepo, enrollmentRepo := overrideApp(t)
	enrollmentRepo.On("FindByUserAndCourse", mock.Anything, uint(2), uint(1), uint(1)).
		Return(&models.Enrollment{UserID: 2, CourseID: 1, Type: "StudentEnrollment"}, nil)
	overrideRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.AssignmentOverride")).
		Run(func(args mock.Arguments) { args.Get(1).(*models.AssignmentOverride).ID = 30 }).Return(nil)
	studentRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.AssignmentOverrideStudent")).
		Return(errors.New("duplicate key value violates unique constraint"))
	overrideRepo.On("Delete", mock.Anything, uint(30)).Return(nil).Once()
	studentRepo.On("ListByUserAndAssignment", mock.Anything, uint(2), uint(1)).Return([]models.AssignmentOverrideStudent{}, nil)
	studentRepo.On("ListByOverrideID", mock.Anything, uint(30)).Return([]models.AssignmentOverrideStudent{}, nil)

	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/1/assignments/1/overrides",
		testutil.JSONBody(map[string]interface{}{"assignment_override": map[string]interface{}{"title": "x", "student_ids": []uint{2}}}))
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	overrideRepo.AssertExpectations(t)
}

// courseAccommodationRepo is a minimal in-memory StudentAccommodationRepository.
type courseAccommodationRepo struct {
	byUser map[uint][]models.StudentAccommodation
}

func (f *courseAccommodationRepo) Create(context.Context, *models.StudentAccommodation) error {
	return nil
}
func (f *courseAccommodationRepo) FindByID(context.Context, uint) (*models.StudentAccommodation, error) {
	return nil, errors.New("not found")
}
func (f *courseAccommodationRepo) Update(context.Context, *models.StudentAccommodation) error {
	return nil
}
func (f *courseAccommodationRepo) Delete(context.Context, uint) error { return nil }
func (f *courseAccommodationRepo) ListByUserID(context.Context, uint, repository.PaginationParams) (*repository.PaginatedResult[models.StudentAccommodation], error) {
	return &repository.PaginatedResult[models.StudentAccommodation]{}, nil
}
func (f *courseAccommodationRepo) ListByUserAndCourse(_ context.Context, userID, _ uint) ([]models.StudentAccommodation, error) {
	return f.byUser[userID], nil
}
func (f *courseAccommodationRepo) ListActiveByUserID(_ context.Context, userID uint) ([]models.StudentAccommodation, error) {
	return f.byUser[userID], nil
}

// B122: the apply-accommodations route reads :assignment_id.
func TestGradingParity_B122_ApplyAccommodationsReadsAssignmentID(t *testing.T) {
	due := time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC)
	extra := 3
	assignmentRepo := new(mocks.MockAssignmentRepository)
	assignmentRepo.On("FindByID", mock.Anything, uint(1), uint(1)).Return(&models.Assignment{ID: 1, CourseID: 1, DueAt: &due}, nil)
	accSvc := service.NewAccommodationService(&courseAccommodationRepo{byUser: map[uint][]models.StudentAccommodation{
		2: {{ID: 5, UserID: 2, ExtraDays: &extra}},
	}}, nil)
	h := handlers.NewAccommodationHandler(accSvc, service.NewAssignmentService(assignmentRepo), nil, nil)
	app := testutil.SetupTestApp()
	app.Use(overrideAuthStub(1, "TeacherEnrollment"))
	app.Post("/courses/:course_id/assignments/:assignment_id/apply_accommodations", h.ApplyAccommodationsToAssignment)

	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/1/assignments/1/apply_accommodations",
		testutil.JSONBody(map[string]interface{}{"user_ids": []uint{2}}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	m, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	adj, _ := m["adjustments"].([]interface{})
	assert.Len(t, adj, 1)

	// Another course's URL is a 404.
	resp = testutil.MakeRequest(app, http.MethodPost, "/courses/2/assignments/1/apply_accommodations",
		testutil.JSONBody(map[string]interface{}{"user_ids": []uint{2}}))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
