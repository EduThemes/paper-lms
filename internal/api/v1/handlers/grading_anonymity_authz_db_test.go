package handlers_test

// Post-merge authorization seams around anonymous grading and peer-review
// files: the user-addressed submission routes must not undo anonymous
// grading for graders, and the course-less peer-review file route must
// still respect the assignment's publish state and the reviewer's
// enrollment. All data is made up.

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

type anonRoutesEnv struct {
	*guEnv
	files *service.FileService
	guH   *handlers.GradingUIHandler
	gpH   *handlers.GradingPolicyHandler
}

func newAnonRoutesEnv(t *testing.T) *anonRoutesEnv {
	t.Helper()
	e := newGUEnv(t)
	g := e.g
	require.NoError(t, g.Model(&models.Course{}).Where("id = ?", guCourse).Update("workflow_state", models.CourseAvailable).Error)
	users := postgres.NewUserRepository(g)
	courses := postgres.NewCourseRepository(g)
	enrollments := postgres.NewEnrollmentRepository(g)
	submissionRepo := postgres.NewSubmissionRepository(g)
	assignmentRepo := postgres.NewAssignmentRepository(g)
	standards := postgres.NewGradingStandardRepository(g)

	files := service.NewFileService(postgres.NewFolderRepository(g), postgres.NewAttachmentRepository(g), t.TempDir())
	e.subH.SetFileService(files)
	peerSvc := service.NewPeerReviewService(postgres.NewPeerReviewRepository(g), submissionRepo, enrollments)
	guH := handlers.NewGradingUIHandler(e.gu, e.subH, users, peerSvc, files)
	guH.SetAuthorizer(handlers.NewResourceAuthorizer(enrollments, users, courses))

	anonymity := handlers.AnonymityCheck(func(ctx context.Context, courseID, assignmentID, accountID uint) bool {
		a, err := assignmentRepo.FindByID(ctx, assignmentID, accountID)
		if err != nil || a == nil || a.CourseID != courseID || !a.AnonymousGrading {
			return false
		}
		students, err := e.subSvc.CourseStudents(ctx, courseID, accountID)
		if err != nil {
			return true
		}
		ids := make([]uint, 0, len(students))
		for _, s := range students {
			ids = append(ids, s.UserID)
		}
		return e.gu.AnonymizeStudents(ctx, a, ids)
	})
	e.sgH.SetAnonymityCheck(anonymity)
	gpH := handlers.NewGradingPolicyHandler(service.NewGradingPolicyService(postgres.NewGradingPolicyRepository(g), standards, e.subSvc),
		courses, assignmentRepo, submissionRepo, standards, nil)
	gpH.SetAnonymityCheck(anonymity)
	return &anonRoutesEnv{guEnv: e, files: files, guH: guH, gpH: gpH}
}

func (e *anonRoutesEnv) routes(userID, accountID uint, enrollmentType string) *fiber.App {
	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		c.Locals("account_id", accountID)
		c.Locals("enrollment_type", enrollmentType)
		c.Locals("is_admin", false)
		return c.Next()
	})
	a := "/courses/:course_id/assignments/:assignment_id"
	app.Get(a+"/submissions", e.subH.ListSubmissions)
	app.Get(a+"/submissions/:user_id/files/:file_id/download", e.subH.DownloadSubmissionFile)
	app.Get(a+"/submissions/:user_id/history/files/:file_id/download", e.guH.DownloadPriorAttemptFile)
	app.Get(a+"/submissions/:user_id/history", e.gpH.SubmissionHistory)
	app.Get(a+"/submissions/:user_id", e.subH.GetSubmission)
	app.Get(a+"/speedgrader/submissions/:user_id", e.sgH.GetStudentSubmission)
	app.Get(a+"/anonymous_submissions/:anonymous_id/files/:file_id/download", e.guH.DownloadAnonymousSubmissionFile)
	app.Post("/courses/:course_id/assignments/:id/post_grades", e.subH.PostGrades)
	app.Get("/peer_reviews/:review_id/files/:file_id/download", e.guH.DownloadPeerReviewFile)
	return app
}

// uploadSubmissionFile stores a small file owned by student and submits
// it to the assignment.
func (e *anonRoutesEnv) uploadSubmissionFile(t *testing.T, a *models.Assignment, student uint) uint {
	t.Helper()
	ctx := context.Background()
	att := &models.Attachment{ContextType: "User", ContextID: student, UserID: student, DisplayName: "essay.txt", Filename: "essay.txt", ContentType: "text/plain"}
	require.NoError(t, e.files.UploadFile(ctx, att, strings.NewReader("my essay")))
	upload := "online_upload"
	atts := fmt.Sprintf(`[{"id":%d,"filename":"essay.txt","content_type":"text/plain"}]`, att.ID)
	require.NoError(t, e.subSvc.Submit(ctx, &models.Submission{AssignmentID: a.ID, UserID: student, SubmissionType: &upload, Attachments: &atts}, guCourse, 1))
	return att.ID
}

func TestAnonymousGrading_UserAddressedRoutesHideStudents(t *testing.T) {
	e := newAnonRoutesEnv(t)
	a := e.assignment(t, &models.Assignment{AnonymousGrading: true, PostPolicy: "manual", SubmissionTypes: "online_upload,online_text_entry"})
	fileID := e.uploadSubmissionFile(t, a, guStudentA)

	ta := e.routes(guTA, 1, "TaEnrollment")
	teacher := e.routes(guTeacher, 1, "TeacherEnrollment")
	student := e.routes(guStudentA, 1, "StudentEnrollment")
	base := fmt.Sprintf("/courses/%d/assignments/%d", guCourse, a.ID)

	// The list keeps the rows but drops user ids; files go through the
	// anonymous route. Listing assigns the anonymous id if SpeedGrader
	// has not yet.
	anonID := ""
	for _, app := range []*fiber.App{ta, teacher} {
		code, body := guDo(t, app, http.MethodGet, base+"/submissions", nil)
		require.Equal(t, http.StatusOK, code, body)
		if anonID == "" {
			anonID = anonymousIDFor(t, e.guEnv, a.ID, guStudentA)
		}
		assert.NotContains(t, body, fmt.Sprintf(`"user_id":%d`, guStudentA))
		assert.Contains(t, body, `"anonymous_id":"`+anonID+`"`)
		assert.Contains(t, body, fmt.Sprintf("/anonymous_submissions/%s/files/%d/download", anonID, fileID))
	}

	userPaths := []string{
		fmt.Sprintf("%s/submissions/%d", base, guStudentA),
		fmt.Sprintf("%s/submissions/%d/files/%d/download", base, guStudentA, fileID),
		fmt.Sprintf("%s/submissions/%d/history", base, guStudentA),
		fmt.Sprintf("%s/speedgrader/submissions/%d", base, guStudentA),
	}
	for _, p := range userPaths {
		code, body := guDo(t, ta, http.MethodGet, p, nil)
		assert.Equal(t, http.StatusNotFound, code, "TA must not address an anonymized student by user id: %s %s", p, body)
	}
	// The anonymous route still serves the grader.
	code, body := guDo(t, ta, http.MethodGet, fmt.Sprintf("%s/anonymous_submissions/%s/files/%d/download", base, anonID, fileID), nil)
	assert.Equal(t, http.StatusOK, code, body)
	// The student keeps their own view.
	for _, p := range userPaths[:3] {
		code, body := guDo(t, student, http.MethodGet, p, nil)
		assert.Equal(t, http.StatusOK, code, "own submission: %s %s", p, body)
	}
	// Another tenant's teacher sees nothing.
	code, _ = guDo(t, e.routes(guOutsider, 2, "TeacherEnrollment"), http.MethodGet, base+"/submissions", nil)
	assert.Equal(t, http.StatusNotFound, code)

	// Posting every student's grade lifts anonymity: the user routes open.
	for _, s := range []uint{guStudentA, guStudentB, guStudentC} {
		_, err := e.subSvc.Grade(context.Background(), a.ID, s, guTeacher, 1, "5")
		require.NoError(t, err)
	}
	code, _ = guDo(t, teacher, http.MethodPost, fmt.Sprintf("/courses/%d/assignments/%d/post_grades", guCourse, a.ID), nil)
	require.Equal(t, http.StatusOK, code)
	for _, p := range userPaths {
		code, body := guDo(t, ta, http.MethodGet, p, nil)
		assert.Equal(t, http.StatusOK, code, "%s %s", p, body)
	}
	_, body = guDo(t, ta, http.MethodGet, base+"/submissions", nil)
	assert.Contains(t, body, fmt.Sprintf(`"user_id":%d`, guStudentA))
}

func TestPeerReviewFileDownload_RequiresPublishedAssignmentAndEnrollment(t *testing.T) {
	e := newAnonRoutesEnv(t)
	a := e.assignment(t, &models.Assignment{SubmissionTypes: "online_upload", PeerReviewsEnabled: true})
	fileID := e.uploadSubmissionFile(t, a, guStudentA)
	pr := &models.PeerReview{AssignmentID: a.ID, ReviewerID: guStudentB, RevieweeID: guStudentA, WorkflowState: "assigned"}
	require.NoError(t, e.g.Create(pr).Error)
	path := fmt.Sprintf("/peer_reviews/%d/files/%d/download", pr.ID, fileID)
	reviewer := e.routes(guStudentB, 1, "")

	code, body := guDo(t, reviewer, http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, code, body)

	// Someone else (same tenant or not) gets nothing.
	code, _ = guDo(t, e.routes(guStudentC, 1, ""), http.MethodGet, path, nil)
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = guDo(t, e.routes(guOutsider, 2, ""), http.MethodGet, path, nil)
	assert.Equal(t, http.StatusNotFound, code)

	// Unpublished assignment: hidden.
	require.NoError(t, e.g.Model(&models.Assignment{}).Where("id = ?", a.ID).Updates(map[string]interface{}{"workflow_state": models.AssignmentUnpublished, "published": false}).Error)
	code, _ = guDo(t, reviewer, http.MethodGet, path, nil)
	assert.Equal(t, http.StatusNotFound, code, "unpublished assignment")
	require.NoError(t, e.g.Model(&models.Assignment{}).Where("id = ?", a.ID).Updates(map[string]interface{}{"workflow_state": models.AssignmentPublished, "published": true}).Error)

	// The reviewer's enrollment ended: no more access.
	require.NoError(t, e.g.Model(&models.Enrollment{}).Where("user_id = ? AND course_id = ?", guStudentB, guCourse).Update("workflow_state", models.EnrollmentDeleted).Error)
	code, _ = guDo(t, reviewer, http.MethodGet, path, nil)
	assert.Equal(t, http.StatusNotFound, code, "dropped reviewer")
}
