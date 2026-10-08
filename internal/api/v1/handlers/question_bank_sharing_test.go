package handlers_test

// G060: one question-bank system with account sharing — handler access
// rules (read vs. own) and payloads.

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

// fakeSharedBankRepo: course 7 owns bank 1 (shared) and bank 2 (private);
// course 8 (same account) owns bank 3. Bank 1 is usable from course 8.
type fakeSharedBankRepo struct {
	banks   map[uint]*models.QuestionBank
	updated *models.QuestionBank
}

func newFakeSharedBankRepo() *fakeSharedBankRepo {
	return &fakeSharedBankRepo{banks: map[uint]*models.QuestionBank{
		1: {ID: 1, CourseID: 7, Title: "Shared", WorkflowState: "active", SharedWithAccount: true},
		2: {ID: 2, CourseID: 7, Title: "Private", WorkflowState: "active"},
		3: {ID: 3, CourseID: 8, Title: "Course 8 bank", WorkflowState: "active"},
	}}
}

func (f *fakeSharedBankRepo) Create(_ context.Context, b *models.QuestionBank) error { return nil }
func (f *fakeSharedBankRepo) FindByID(_ context.Context, id uint) (*models.QuestionBank, error) {
	if b, ok := f.banks[id]; ok {
		cp := *b
		return &cp, nil
	}
	return nil, errors.New("not found")
}
func (f *fakeSharedBankRepo) Update(_ context.Context, b *models.QuestionBank) error {
	f.updated = b
	return nil
}
func (f *fakeSharedBankRepo) Delete(_ context.Context, _ uint) error { return nil }
func (f *fakeSharedBankRepo) ListByCourse(_ context.Context, courseID uint, _ repository.PaginationParams) (*repository.PaginatedResult[models.QuestionBank], error) {
	return &repository.PaginatedResult[models.QuestionBank]{}, nil
}
func (f *fakeSharedBankRepo) ListUsableByCourse(_ context.Context, courseID uint) ([]models.QuestionBank, error) {
	var out []models.QuestionBank
	for _, id := range []uint{1, 2, 3} {
		b := f.banks[id]
		if b.CourseID == courseID || (b.SharedWithAccount && courseID == 8) {
			out = append(out, *b)
		}
	}
	return out, nil
}
func (f *fakeSharedBankRepo) UsableInCourse(_ context.Context, bankID, courseID uint) (bool, error) {
	b := f.banks[bankID]
	return b != nil && (b.CourseID == courseID || (b.SharedWithAccount && courseID == 8)), nil
}

type fakeQBEntryRepo struct{ created *models.QuestionBankEntry }

func (f *fakeQBEntryRepo) Create(_ context.Context, e *models.QuestionBankEntry) error {
	e.ID = 77
	f.created = e
	return nil
}
func (f *fakeQBEntryRepo) FindByID(_ context.Context, _ uint) (*models.QuestionBankEntry, error) {
	return nil, errors.New("not found")
}
func (f *fakeQBEntryRepo) Update(_ context.Context, _ *models.QuestionBankEntry) error { return nil }
func (f *fakeQBEntryRepo) Delete(_ context.Context, _ uint) error                      { return nil }
func (f *fakeQBEntryRepo) ListByBankID(_ context.Context, _ uint) ([]models.QuestionBankEntry, error) {
	return nil, nil
}

// The bank page sends answers as a JSON array; the quiz editor sends a
// JSON string. Both are stored as JSON text (the array used to be a 400).
func TestQuestionBanks_AddQuestionAcceptsArrayOrStringAnswers(t *testing.T) {
	for _, body := range []string{
		`{"question_name":"Q","question_type":"matching_question","question_text":"<p>Match</p><script>x</script>","points_possible":2,"answers":[{"id":"m","left":"A","right":"1","right_id":"r"}]}`,
		`{"question_name":"Q","question_type":"matching","question_text":"<p>Match</p>","points_possible":2,"answers":"[{\"id\":\"m\",\"left\":\"A\",\"right\":\"1\",\"right_id\":\"r\"}]"}`,
	} {
		entries := &fakeQBEntryRepo{}
		h := handlers.NewQuestionBankHandler(service.NewQuestionBankService(newFakeSharedBankRepo(), entries, nil, nil))
		app := testutil.SetupTestApp()
		app.Use(func(c *fiber.Ctx) error {
			c.Locals("user_id", uint(2))
			c.Locals("account_id", uint(1))
			c.Locals("enrollment_type", "TeacherEnrollment")
			return c.Next()
		})
		app.Post("/courses/:course_id/question_banks/:bank_id/questions", h.AddQuestion)
		resp := testutil.MakeRequest(app, http.MethodPost, "/courses/7/question_banks/2/questions", strings.NewReader(body))
		require.Equal(t, http.StatusCreated, resp.StatusCode, body)
		require.NotNil(t, entries.created)
		assert.Equal(t, `[{"id":"m","left":"A","right":"1","right_id":"r"}]`, entries.created.Answers)
		assert.Equal(t, "matching", entries.created.QuestionType)
		assert.Equal(t, uint(2), entries.created.QuestionBankID)
		assert.NotContains(t, entries.created.QuestionText, "<script>")
	}
}

func bankApp(role string, repo *fakeSharedBankRepo) *fiber.App {
	h := handlers.NewQuestionBankHandler(service.NewQuestionBankService(repo, nil, nil, nil))
	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(2))
		c.Locals("account_id", uint(1))
		c.Locals("enrollment_type", role)
		return c.Next()
	})
	app.Get("/courses/:course_id/question_banks", h.ListBanks)
	app.Get("/courses/:course_id/question_banks/:bank_id", h.GetBank)
	app.Put("/courses/:course_id/question_banks/:bank_id", h.UpdateBank)
	return app
}

func TestQuestionBanks_ListIncludesAccountSharedBanksForStaff(t *testing.T) {
	repo := newFakeSharedBankRepo()
	resp := testutil.MakeRequest(bankApp("TeacherEnrollment", repo), http.MethodGet, "/courses/8/question_banks", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	arr, _ := testutil.ParseJSONArray(resp)
	require.Len(t, arr, 2)
	byTitle := map[string]map[string]interface{}{}
	for _, b := range arr {
		byTitle[b["title"].(string)] = b
	}
	assert.Equal(t, true, byTitle["Course 8 bank"]["owned"])
	assert.Equal(t, false, byTitle["Shared"]["owned"])
	assert.NotContains(t, byTitle, "Private")

	// Students only see their own course's banks.
	resp = testutil.MakeRequest(bankApp("StudentEnrollment", repo), http.MethodGet, "/courses/8/question_banks", nil)
	arr, _ = testutil.ParseJSONArray(resp)
	require.Len(t, arr, 1)
	assert.Equal(t, "Course 8 bank", arr[0]["title"])
}

func TestQuestionBanks_SharedBankReadableNotEditable(t *testing.T) {
	repo := newFakeSharedBankRepo()
	app := bankApp("TeacherEnrollment", repo)

	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/8/question_banks/1", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	m, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, false, m["owned"])

	resp = testutil.MakeRequest(app, http.MethodGet, "/courses/8/question_banks/2", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "an unshared bank of another course is not found")

	resp = testutil.MakeRequest(app, http.MethodPut, "/courses/8/question_banks/1", testutil.JSONBody(map[string]interface{}{"title": "Hijacked"}))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "only the owning course edits a shared bank")
	assert.Nil(t, repo.updated)
}

func TestQuestionBanks_OwnerSharesBank(t *testing.T) {
	repo := newFakeSharedBankRepo()
	resp := testutil.MakeRequest(bankApp("TeacherEnrollment", repo), http.MethodPut, "/courses/7/question_banks/2",
		testutil.JSONBody(map[string]interface{}{"shared_with_account": true, "description": "<b>Unit 2</b> items"}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotNil(t, repo.updated)
	assert.True(t, repo.updated.SharedWithAccount)
	assert.Equal(t, "Private", repo.updated.Title, "absent title is unchanged")
	assert.NotContains(t, repo.updated.Description, "<b>", "description is plain text")
	m, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, true, m["shared_with_account"])
	assert.Equal(t, true, m["owned"])
}
