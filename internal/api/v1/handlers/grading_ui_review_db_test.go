package handlers_test

// DB-backed tests for the grading-UI review fixes: comment draft rules,
// editing / publishing comments by anonymous id, grader identities under
// moderated grading, and the moderated-grading switch rules. Same scratch
// database and made-up data as grading_ui_db_test.go.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

func guCommentID(t *testing.T, body string) uint {
	t.Helper()
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(body), &m), body)
	return uint(m["id"].(float64))
}

// G033: a sent comment never becomes a draft again, so a student cannot
// hide their comment from the graders and a grader cannot pull back
// feedback the student may have read.
func TestGradingUI_CommentDraftRules(t *testing.T) {
	e := newGUEnv(t)
	a := e.assignment(t, &models.Assignment{})
	code, body := submitText(t, e, a, guStudentA)
	require.Equal(t, http.StatusCreated, code, body)
	base := fmt.Sprintf("/courses/%d/assignments/%d/submissions/%d/comments", guCourse, a.ID, guStudentA)

	code, body = guDo(t, e.student(guStudentA), http.MethodPost, base, map[string]interface{}{"comment": map[string]interface{}{"text_comment": "question for you", "draft": true}})
	require.Equal(t, http.StatusCreated, code, body)
	assert.Contains(t, body, `"draft":false`, "students cannot create drafts")
	studentComment := guCommentID(t, body)
	code, body = guDo(t, e.student(guStudentA), http.MethodPut, fmt.Sprintf("%s/%d", base, studentComment),
		map[string]interface{}{"comment": map[string]interface{}{"draft": true}})
	assert.Equal(t, http.StatusBadRequest, code, body)
	_, body = guDo(t, e.teacher(), http.MethodGet, base, nil)
	assert.Contains(t, body, "question for you", "the teacher still sees the student's comment")

	code, body = guDo(t, e.teacher(), http.MethodPost, base, map[string]interface{}{"comment": map[string]interface{}{"text_comment": "sent feedback"}})
	require.Equal(t, http.StatusCreated, code, body)
	sent := guCommentID(t, body)
	code, body = guDo(t, e.teacher(), http.MethodPut, fmt.Sprintf("%s/%d", base, sent),
		map[string]interface{}{"comment": map[string]interface{}{"draft": true}})
	assert.Equal(t, http.StatusBadRequest, code, body)
	_, body = guDo(t, e.student(guStudentA), http.MethodGet, base, nil)
	assert.Contains(t, body, "sent feedback")
}

// G031 + G033: under anonymous grading a grader edits, publishes and
// deletes comments by anonymous id (SpeedGrader has no user id).
func TestGradingUI_AnonymousCommentEditPublishDelete(t *testing.T) {
	e := newGUEnv(t)
	a := e.assignment(t, &models.Assignment{AnonymousGrading: true})
	code, body := submitText(t, e, a, guStudentA)
	require.Equal(t, http.StatusCreated, code, body)

	rows, err := e.gu.EnsureSubmissionRows(context.Background(), a, []uint{guStudentA, guStudentB, guStudentC})
	require.NoError(t, err)
	anonPath := fmt.Sprintf("/courses/%d/assignments/%d/anonymous_submissions/%s", guCourse, a.ID, *rows[guStudentA].AnonymousID)

	code, body = guDo(t, e.teacher(), http.MethodPost, anonPath+"/comments", map[string]interface{}{"comment": map[string]interface{}{"text_comment": "draft by anon id", "draft": true}})
	require.Equal(t, http.StatusCreated, code, body)
	id := guCommentID(t, body)
	studentBase := fmt.Sprintf("/courses/%d/assignments/%d/submissions/%d/comments", guCourse, a.ID, guStudentA)
	_, body = guDo(t, e.student(guStudentA), http.MethodGet, studentBase, nil)
	assert.NotContains(t, body, "draft by anon id")

	// Another tenant's teacher and a wrong anonymous id get 404.
	code, _ = guDo(t, e.app(guOutsider, 2, "TeacherEnrollment", false), http.MethodPut, fmt.Sprintf("%s/comments/%d", anonPath, id),
		map[string]interface{}{"comment": map[string]interface{}{"draft": false}})
	assert.Equal(t, http.StatusNotFound, code)
	otherAnon := fmt.Sprintf("/courses/%d/assignments/%d/anonymous_submissions/%s", guCourse, a.ID, *rows[guStudentB].AnonymousID)
	code, _ = guDo(t, e.teacher(), http.MethodPut, fmt.Sprintf("%s/comments/%d", otherAnon, id),
		map[string]interface{}{"comment": map[string]interface{}{"draft": false}})
	assert.Equal(t, http.StatusNotFound, code, "the comment belongs to another submission")

	code, body = guDo(t, e.teacher(), http.MethodPut, fmt.Sprintf("%s/comments/%d", anonPath, id),
		map[string]interface{}{"comment": map[string]interface{}{"text_comment": "published by anon id", "draft": false}})
	require.Equal(t, http.StatusOK, code, body)
	assert.NotContains(t, body, fmt.Sprintf("submissions/%d/", guStudentA), "file links stay anonymous")
	_, body = guDo(t, e.student(guStudentA), http.MethodGet, studentBase, nil)
	assert.Contains(t, body, "published by anon id", "publishing the draft posts it to the student")

	code, body = guDo(t, e.teacher(), http.MethodDelete, fmt.Sprintf("%s/comments/%d", anonPath, id), nil)
	require.Equal(t, http.StatusOK, code, body)
	_, body = guDo(t, e.student(guStudentA), http.MethodGet, studentBase, nil)
	assert.NotContains(t, body, "published by anon id")
}

// G030: while a moderated assignment's grades are unpublished, graders
// anonymous to each other see other graders' comments by label only; the
// final grader sees names only when the assignment allows it; an
// administrator always does.
func TestGradingUI_GraderIdentitiesHiddenInComments(t *testing.T) {
	e := newGUEnv(t)
	final := guTeacher
	a := e.assignment(t, &models.Assignment{
		ModeratedGrading: true, GraderCount: 2, FinalGraderID: &final,
		GraderCommentsVisibleToGraders: true, GradersAnonymousToGraders: true,
		GraderNamesVisibleToFinalGrader: false,
	})
	code, body := submitText(t, e, a, guStudentA)
	require.Equal(t, http.StatusCreated, code, body)
	ta := e.app(guTA, 1, "TaEnrollment", false)
	pgPath := fmt.Sprintf("/courses/%d/assignments/%d/provisional_grades", guCourse, a.ID)
	code, body = guDo(t, ta, http.MethodPut, pgPath, map[string]interface{}{"user_id": guStudentA, "posted_grade": "6"})
	require.Equal(t, http.StatusOK, code, body)

	comments := fmt.Sprintf("/courses/%d/assignments/%d/submissions/%d/comments", guCourse, a.ID, guStudentA)
	code, body = guDo(t, ta, http.MethodPost, comments, map[string]interface{}{"comment": map[string]interface{}{"text_comment": "TA thoughts"}})
	require.Equal(t, http.StatusCreated, code, body)
	code, body = guDo(t, e.teacher(), http.MethodPost, comments, map[string]interface{}{"comment": map[string]interface{}{"text_comment": "Moderator thoughts"}})
	require.Equal(t, http.StatusCreated, code, body)

	speedgrader := fmt.Sprintf("/courses/%d/assignments/%d/speedgrader", guCourse, a.ID)

	// The TA sees the moderator's comment, unnamed.
	_, body = guDo(t, ta, http.MethodGet, speedgrader, nil)
	assert.Contains(t, body, "Moderator thoughts")
	assert.NotContains(t, body, "Pat Teacher")
	assert.NotContains(t, body, fmt.Sprintf(`"author_id":%d`, guTeacher))
	_, body = guDo(t, ta, http.MethodGet, comments, nil)
	assert.Contains(t, body, "Moderator thoughts")
	assert.NotContains(t, body, "Pat Teacher")

	// The final grader sees the TA by grader label only.
	_, body = guDo(t, e.teacher(), http.MethodGet, speedgrader, nil)
	assert.Contains(t, body, "TA thoughts")
	assert.NotContains(t, body, "Sam Assistant")
	assert.Contains(t, body, "Grader ")
	_, body = guDo(t, e.teacher(), http.MethodGet, comments, nil)
	assert.NotContains(t, body, "Sam Assistant")
	assert.Contains(t, body, "Grader ")

	// An administrator sees names.
	_, body = guDo(t, e.app(3, 1, "TeacherEnrollment", true), http.MethodGet, comments, nil)
	assert.Contains(t, body, "Sam Assistant")
}

// G030: moderated grading follows Canvas's switch rules: not for group,
// peer-reviewed or ungraded assignments, and frozen once anything is
// graded (provisionally or not).
func TestGradingUI_ModeratedGradingSwitchRules(t *testing.T) {
	e := newGUEnv(t)
	create := fmt.Sprintf("/courses/%d/assignments", guCourse)
	cat := guSeedGroup(t, e, guStudentA, guStudentB)
	for name, extra := range map[string]map[string]interface{}{
		"group":      {"group_category_id": cat},
		"peer":       {"peer_reviews_enabled": true},
		"not graded": {"grading_type": "not_graded"},
	} {
		in := map[string]interface{}{"name": "Moderated " + name, "moderated_grading": true}
		for k, v := range extra {
			in[k] = v
		}
		code, body := guDo(t, e.teacher(), http.MethodPost, create, map[string]interface{}{"assignment": in})
		assert.Equal(t, http.StatusBadRequest, code, "%s: %s", name, body)
	}

	// Graded work freezes the switch.
	plain := e.assignment(t, &models.Assignment{})
	_, err := e.subSvc.Grade(context.Background(), plain.ID, guStudentA, guTeacher, 1, "8")
	require.NoError(t, err)
	update := fmt.Sprintf("/courses/%d/assignments/%d", guCourse, plain.ID)
	code, body := guDo(t, e.teacher(), http.MethodPut, update, map[string]interface{}{"assignment": map[string]interface{}{"moderated_grading": true}})
	assert.Equal(t, http.StatusBadRequest, code, body)

	// A provisional grade freezes it too; with none it can still change.
	final := guTeacher
	mod := e.assignment(t, &models.Assignment{ModeratedGrading: true, GraderCount: 2, FinalGraderID: &final})
	modUpdate := fmt.Sprintf("/courses/%d/assignments/%d", guCourse, mod.ID)
	code, body = guDo(t, e.app(guTA, 1, "TaEnrollment", false), http.MethodPut, fmt.Sprintf("/courses/%d/assignments/%d/provisional_grades", guCourse, mod.ID),
		map[string]interface{}{"user_id": guStudentA, "posted_grade": "4"})
	require.Equal(t, http.StatusOK, code, body)
	code, body = guDo(t, e.teacher(), http.MethodPut, modUpdate, map[string]interface{}{"assignment": map[string]interface{}{"moderated_grading": false}})
	assert.Equal(t, http.StatusBadRequest, code, body)

	fresh := e.assignment(t, &models.Assignment{})
	code, body = guDo(t, e.teacher(), http.MethodPut, fmt.Sprintf("/courses/%d/assignments/%d", guCourse, fresh.ID),
		map[string]interface{}{"assignment": map[string]interface{}{"moderated_grading": true}})
	require.Equal(t, http.StatusOK, code, body)
	assert.Contains(t, body, `"moderated_grading":true`)
}

// G034: an earlier attempt's files link to the history route (the
// current-submission route serves only the latest attempt's files).
func TestGradingUI_PriorAttemptFileLinks(t *testing.T) {
	e := newGUEnv(t)
	a := e.assignment(t, &models.Assignment{SubmissionTypes: "online_upload,online_text_entry"})
	ctx := context.Background()
	upload := "online_upload"
	atts := `[{"id":777,"filename":"draft.pdf","content_type":"application/pdf","url":"/api/v1/old"}]`
	require.NoError(t, e.subSvc.Submit(ctx, &models.Submission{AssignmentID: a.ID, UserID: guStudentA, SubmissionType: &upload, Attachments: &atts}, guCourse, 1))
	code, body := submitText(t, e, a, guStudentA)
	require.Equal(t, http.StatusCreated, code, body)

	code, body = guDo(t, e.teacher(), http.MethodGet, fmt.Sprintf("/courses/%d/assignments/%d/speedgrader", guCourse, a.ID), nil)
	require.Equal(t, http.StatusOK, code, body)
	assert.Contains(t, body, fmt.Sprintf("/api/v1/courses/%d/assignments/%d/submissions/%d/history/files/777/download", guCourse, a.ID, guStudentA))
	assert.NotContains(t, body, "/api/v1/old")
}

// A form-encoded assignment create (no JSON body) is not rejected by the
// JSON-only grading settings.
func TestGradingUI_FormEncodedAssignmentCreate(t *testing.T) {
	e := newGUEnv(t)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/courses/%d/assignments", guCourse), strings.NewReader("assignment.name=Reading+log"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := e.teacher().Test(req, -1)
	require.NoError(t, err)
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusCreated, resp.StatusCode, string(raw))
	assert.Contains(t, string(raw), "Reading log")
	assert.Contains(t, string(raw), `"grader_comments_visible_to_graders":true`, "create defaults still apply")
}
