package handlers_test

// More DB-backed grading-UI tests: rubric assessment by anonymous id,
// omit_from_final_grade, the course grading-period list and the LTI
// external-tool submission hook. Made-up data only.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
)

// anonymousIDFor reads studentID's anonymous id straight from the row.
func anonymousIDFor(t *testing.T, e *guEnv, assignmentID, studentID uint) string {
	t.Helper()
	var sub models.Submission
	require.NoError(t, e.g.Where("assignment_id = ? AND user_id = ?", assignmentID, studentID).First(&sub).Error)
	require.NotNil(t, sub.AnonymousID)
	return *sub.AnonymousID
}

// G031/G034: SpeedGrader saves a rubric assessment by anonymous id
// without ever learning the student's user id.
func TestGradingUI_AnonymousRubricAssessment(t *testing.T) {
	e := newGUEnv(t)
	ctx := context.Background()
	a := e.assignment(t, &models.Assignment{AnonymousGrading: true, PostPolicy: "manual"})
	rubric := &models.Rubric{ContextType: "Course", ContextID: guCourse, Title: "Lab rubric", PointsPossible: 10,
		Data: `[{"id":"c1","description":"Method","points":5},{"id":"c2","description":"Results","points":5}]`}
	require.NoError(t, postgres.NewRubricRepository(e.g).Create(ctx, rubric))
	require.NoError(t, postgres.NewRubricAssociationRepository(e.g).Create(ctx, &models.RubricAssociation{
		RubricID: rubric.ID, AssociationID: a.ID, AssociationType: "Assignment", ContextType: "Course", ContextID: guCourse, Purpose: "grading", UseForGrading: true,
	}))
	code, body := submitText(t, e, a, guStudentA)
	require.Equal(t, http.StatusCreated, code, body)
	// SpeedGrader assigns the anonymous ids.
	code, body = guDo(t, e.teacher(), http.MethodGet, fmt.Sprintf("/courses/%d/assignments/%d/speedgrader", guCourse, a.ID), nil)
	require.Equal(t, http.StatusOK, code, body)
	anon := anonymousIDFor(t, e, a.ID, guStudentA)
	path := fmt.Sprintf("/courses/%d/assignments/%d/anonymous_submissions/%s/rubric_assessment", guCourse, a.ID, anon)

	code, body = guDo(t, e.teacher(), http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, code, body)
	assert.JSONEq(t, `{"rubric_assessment":null}`, body)

	data := `{"c1":{"points":4,"comments":"clear"},"c2":{"points":3}}`
	code, body = guDo(t, e.teacher(), http.MethodPut, path, map[string]interface{}{"rubric_assessment": map[string]interface{}{"data": data}})
	require.Equal(t, http.StatusCreated, code, body)
	assert.NotContains(t, body, "user_id")
	var out struct {
		RubricAssessment struct {
			ID    uint     `json:"id"`
			Score *float64 `json:"score"`
		} `json:"rubric_assessment"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &out))
	require.NotNil(t, out.RubricAssessment.Score)
	assert.Equal(t, 7.0, *out.RubricAssessment.Score)

	// A second save updates the same assessment.
	code, body = guDo(t, e.teacher(), http.MethodPut, path, map[string]interface{}{"rubric_assessment": map[string]interface{}{"data": `{"c1":{"points":5},"c2":{"points":5}}`}})
	require.Equal(t, http.StatusOK, code, body)
	assert.Contains(t, body, fmt.Sprintf(`"id":%d`, out.RubricAssessment.ID))
	assert.Contains(t, body, `"score":10`)

	// Another tenant's teacher gets 404.
	code, _ = guDo(t, e.app(guOutsider, 2, "TeacherEnrollment", false), http.MethodGet, path, nil)
	assert.Equal(t, http.StatusNotFound, code)

	// The rubric-assessment API hides the student while anonymized.
	rubricSvc := service.NewRubricService(postgres.NewRubricRepository(e.g), postgres.NewRubricAssociationRepository(e.g), postgres.NewRubricAssessmentRepository(e.g))
	raH := handlers.NewRubricAssessmentHandler(rubricSvc)
	raH.SetAnonymityCheck(func(ctx context.Context, courseID, assignmentID, accountID uint) bool {
		asg, err := postgres.NewAssignmentRepository(e.g).FindByID(ctx, assignmentID, accountID)
		if err != nil || asg.CourseID != courseID {
			return false
		}
		return e.gu.AnonymizeStudents(ctx, asg, []uint{guStudentA, guStudentB, guStudentC})
	})
	_, assoc, err := rubricSvc.GetRubricForAssignment(ctx, a.ID, 1)
	require.NoError(t, err)
	listApp := e.teacher()
	listApp.Get("/courses/:course_id/rubric_associations/:association_id/rubric_assessments", raH.ListAssessments)
	listPath := fmt.Sprintf("/courses/%d/rubric_associations/%d/rubric_assessments", guCourse, assoc.ID)
	code, body = guDo(t, listApp, http.MethodGet, listPath, nil)
	require.Equal(t, http.StatusOK, code, body)
	assert.Contains(t, body, `"user_id":null`)
	assert.NotContains(t, body, fmt.Sprintf(`"user_id":%d`, guStudentA))

	// Once every grade is posted, anonymity lifts.
	for _, s := range []uint{guStudentA, guStudentB, guStudentC} {
		_, err := e.subSvc.Grade(ctx, a.ID, s, guTeacher, 1, "5")
		require.NoError(t, err)
	}
	code, _ = guDo(t, e.teacher(), http.MethodPost, fmt.Sprintf("/courses/%d/assignments/%d/post_grades", guCourse, a.ID), nil)
	require.Equal(t, http.StatusOK, code)
	_, body = guDo(t, listApp, http.MethodGet, listPath, nil)
	assert.Contains(t, body, fmt.Sprintf(`"user_id":%d`, guStudentA))
}

// G035: an assignment omitted from the final grade still holds its
// grade but does not move the course total.
func TestGradingUI_OmitFromFinalGrade(t *testing.T) {
	e := newGUEnv(t)
	ctx := context.Background()
	counted := e.assignment(t, &models.Assignment{Name: "Counted"})
	omitted := e.assignment(t, &models.Assignment{Name: "Practice", OmitFromFinalGrade: true})
	_, err := e.subSvc.Grade(ctx, counted.ID, guStudentA, guTeacher, 1, "8")
	require.NoError(t, err)
	_, err = e.subSvc.Grade(ctx, omitted.ID, guStudentA, guTeacher, 1, "1")
	require.NoError(t, err)

	gradingSvc := service.NewGradingService(postgres.NewSubmissionRepository(e.g), postgres.NewAssignmentRepository(e.g), postgres.NewAssignmentGroupRepository(e.g),
		postgres.NewEnrollmentRepository(e.g), postgres.NewCourseRepository(e.g), postgres.NewGradingStandardRepository(e.g))
	grade, err := gradingSvc.GetStudentGrade(ctx, guCourse, guStudentA)
	require.NoError(t, err)
	assert.Equal(t, 80.0, grade.CurrentScore, "the omitted 1/10 must not pull the total down")

	// The flag round-trips through the assignment API.
	code, body := guDo(t, e.teacher(), http.MethodPut, fmt.Sprintf("/courses/%d/assignments/%d", guCourse, counted.ID),
		map[string]interface{}{"assignment": map[string]interface{}{"omit_from_final_grade": true}})
	require.Equal(t, http.StatusOK, code, body)
	assert.Contains(t, body, `"omit_from_final_grade":true`)
}

// G035: the gradebook's grading-period filter lists the periods of the
// course's account; other tenants get 404.
func TestGradingUI_CourseGradingPeriods(t *testing.T) {
	e := newGUEnv(t)
	ctx := context.Background()
	group := &models.GradingPeriodGroup{AccountID: 1, Title: "2026-27", WorkflowState: "active"}
	require.NoError(t, e.g.WithContext(ctx).Create(group).Error)
	start := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	for i, title := range []string{"Fall", "Spring"} {
		p := &models.GradingPeriod{GradingPeriodGroupID: group.ID, Title: title, WorkflowState: "active",
			StartDate: start.AddDate(0, 5*i, 0), EndDate: start.AddDate(0, 5*i+4, 0)}
		require.NoError(t, e.g.WithContext(ctx).Create(p).Error)
	}
	code, body := guDo(t, e.teacher(), http.MethodGet, fmt.Sprintf("/courses/%d/grading_periods", guCourse), nil)
	require.Equal(t, http.StatusOK, code, body)
	var periods []struct {
		Title string `json:"title"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &periods))
	require.Len(t, periods, 2)
	assert.Equal(t, "Fall", periods[0].Title)

	code, _ = guDo(t, e.app(guOutsider, 2, "TeacherEnrollment", false), http.MethodGet, fmt.Sprintf("/courses/%d/grading_periods", guCourse), nil)
	assert.Equal(t, http.StatusNotFound, code)
}

// G026: an LTI tool reporting the student's work as Submitted creates a
// basic_lti_launch submission on an external_tool assignment.
func TestGradingUI_ExternalToolSubmissionHook(t *testing.T) {
	e := newGUEnv(t)
	ctx := context.Background()
	a := e.assignment(t, &models.Assignment{SubmissionTypes: "external_tool"})
	item := &models.LTILineItem{AssignmentID: &a.ID, CourseID: guCourse, Label: "Tool quiz", ScoreMaximum: 10}
	lineItems := postgres.NewLTILineItemRepository(e.g)
	require.NoError(t, lineItems.Create(ctx, item))
	ags := service.NewLTIAGSService(lineItems, postgres.NewLTIResultRepository(e.g), postgres.NewAssignmentRepository(e.g), postgres.NewEnrollmentRepository(e.g), e.subSvc)

	score, max := 6.0, 10.0
	require.NoError(t, ags.PostScore(ctx, item, 1, guTeacher, &models.LTIResult{
		UserID: guStudentA, ResultScore: &score, ResultMaximum: &max, ActivityProgress: "Submitted", GradingProgress: "FullyGraded",
	}))
	sub, err := e.subSvc.GetByAssignmentAndUser(ctx, a.ID, guStudentA, 1)
	require.NoError(t, err)
	require.NotNil(t, sub.SubmissionType)
	assert.Equal(t, "basic_lti_launch", *sub.SubmissionType)
	assert.NotNil(t, sub.SubmittedAt)
	require.NotNil(t, sub.Score)
	assert.Equal(t, 6.0, *sub.Score)

	// A score that is not a submission (InProgress) on a fresh student
	// does not invent one.
	require.NoError(t, ags.PostScore(ctx, item, 1, guTeacher, &models.LTIResult{
		UserID: guStudentB, ActivityProgress: "InProgress", GradingProgress: "NotReady",
	}))
	other, err := e.subSvc.GetByAssignmentAndUser(ctx, a.ID, guStudentB, 1)
	if err == nil && other != nil {
		assert.Nil(t, other.SubmittedAt)
	}
}
