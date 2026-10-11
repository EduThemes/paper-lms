package handlers_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// makeTinyIMSCC produces an in-memory .imscc bundle containing one
// trivial Canvas Classic assessment. This is a black-box integration
// fixture for the handler — the parser already has detailed coverage
// for parse edge cases.
func makeTinyIMSCC(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	manifest := `<?xml version="1.0"?>
<manifest xmlns="http://www.imsglobal.org/xsd/imsccv1p1/imscp_v1p1">
  <resources>
    <resource identifier="a" type="imsqti_xmlv1p2/imscc_xmlv1p1/assessment" href="a.xml.qti">
      <file href="a.xml.qti"/>
    </resource>
  </resources>
</manifest>`
	asmt := `<?xml version="1.0"?>
<questestinterop>
  <assessment ident="a" title="Tiny">
    <qtimetadata/>
    <section ident="s">
      <item ident="q1" title="MC">
        <itemmetadata><qtimetadata>
          <qtimetadatafield><fieldlabel>question_type</fieldlabel><fieldentry>multiple_choice_question</fieldentry></qtimetadatafield>
          <qtimetadatafield><fieldlabel>points_possible</fieldlabel><fieldentry>1</fieldentry></qtimetadatafield>
        </qtimetadata></itemmetadata>
        <presentation>
          <material><mattext>2+2?</mattext></material>
          <response_lid ident="r1" rcardinality="Single">
            <render_choice>
              <response_label ident="b"><material><mattext>4</mattext></material></response_label>
            </render_choice>
          </response_lid>
        </presentation>
        <resprocessing>
          <outcomes><decvar maxvalue="100" varname="SCORE"/></outcomes>
          <respcondition continue="No">
            <conditionvar><varequal respident="r1">b</varequal></conditionvar>
            <setvar varname="SCORE" action="Set">100</setvar>
          </respcondition>
        </resprocessing>
      </item>
    </section>
  </assessment>
</questestinterop>`

	for _, e := range []struct{ name, data string }{
		{"imsmanifest.xml", manifest},
		{"a.xml.qti", asmt},
	} {
		w, err := zw.Create(e.name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(e.data))
	}
	zw.Close()
	return buf.Bytes()
}

func TestQTIImportHandler_PostMultipartIMSCC(t *testing.T) {
	// Wire the full service with mocks for the repos.
	quizRepo := &mocks.MockQuizRepository{}
	questionRepo := &mocks.MockQuizQuestionRepository{}
	bankRepo := &mocks.MockQuizItemBankRepository{}
	bankItemRepo := &mocks.MockQuizItemBankItemRepository{}
	stimRepo := &mocks.MockQuizStimulusRepository{}

	bankSvc := service.NewQuizItemBankService(bankRepo, bankItemRepo, questionRepo)
	stimSvc := service.NewQuizStimulusService(stimRepo, questionRepo)
	svc := service.NewQTIImportService(quizRepo, questionRepo, bankSvc, stimSvc, t.TempDir())
	h := handlers.NewQTIImportHandler(svc)

	quizRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Quiz")).
		Run(func(args mock.Arguments) { args.Get(1).(*models.Quiz).ID = 1 }).
		Return(nil)
	questionRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.QuizQuestion")).
		Return(nil)

	app := testutil.SetupTestApp()
	api := app.Group("", func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(42))
		c.Locals("account_id", uint(1))
		return c.Next()
	})
	api.Post("/courses/:course_id/qti_import", h.Import)

	// Build multipart form body.
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, _ := mw.CreateFormFile("file", "tiny.imscc")
	io.Copy(part, bytes.NewReader(makeTinyIMSCC(t)))
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/courses/9/qti_import", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var out map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if got, _ := out["quizzes_created"].(float64); got != 1 {
		t.Errorf("want 1 quiz, got %v", out["quizzes_created"])
	}
	if got, _ := out["questions_created"].(float64); got != 1 {
		t.Errorf("want 1 question, got %v", out["questions_created"])
	}
}

func TestQTIImportHandler_MissingFile(t *testing.T) {
	svc := service.NewQTIImportService(
		&mocks.MockQuizRepository{},
		&mocks.MockQuizQuestionRepository{},
		nil, nil, t.TempDir(),
	)
	h := handlers.NewQTIImportHandler(svc)

	app := testutil.SetupTestApp()
	app.Post("/courses/:course_id/qti_import", h.Import)

	req := httptest.NewRequest(http.MethodPost, "/courses/1/qti_import", nil)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=---empty")
	resp, _ := app.Test(req, -1)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestQTIImportHandler_BadFileExt(t *testing.T) {
	svc := service.NewQTIImportService(
		&mocks.MockQuizRepository{},
		&mocks.MockQuizQuestionRepository{},
		nil, nil, t.TempDir(),
	)
	h := handlers.NewQTIImportHandler(svc)

	app := testutil.SetupTestApp()
	app.Post("/courses/:course_id/qti_import", h.Import)

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, _ := mw.CreateFormFile("file", "wrong.txt")
	part.Write([]byte("not a zip"))
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/courses/1/qti_import", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, _ := app.Test(req, -1)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// The .imscc export carries the full answer key, so it is course-scoped:
// a quiz from another course (or, via the tenant-scoped lookup, another
// tenant) is a 404, and bank-draw copies are never exported as questions.
func setupQTIExport(t *testing.T) (*fiber.App, *mocks.MockQuizRepository, *mocks.MockQuizQuestionRepository) {
	t.Helper()
	quizRepo := &mocks.MockQuizRepository{}
	questionRepo := &mocks.MockQuizQuestionRepository{}
	bankSvc := service.NewQuizItemBankService(&mocks.MockQuizItemBankRepository{}, &mocks.MockQuizItemBankItemRepository{}, questionRepo)
	stimSvc := service.NewQuizStimulusService(&mocks.MockQuizStimulusRepository{}, questionRepo)
	svc := service.NewQTIImportService(quizRepo, questionRepo, bankSvc, stimSvc, t.TempDir())
	h := handlers.NewQTIImportHandler(svc)

	app := testutil.SetupTestApp()
	api := app.Group("", func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(42))
		c.Locals("account_id", uint(1))
		return c.Next()
	})
	api.Get("/courses/:course_id/quizzes/:quiz_id/export.imscc", h.Export)
	return app, quizRepo, questionRepo
}

func TestQTIExport_QuizFromOtherCourse_404(t *testing.T) {
	app, quizRepo, _ := setupQTIExport(t)
	// Quiz 5 exists in the caller's tenant but in course 99, not course 7.
	quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(&models.Quiz{ID: 5, CourseID: 99}, nil)

	req := httptest.NewRequest(http.MethodGet, "/courses/7/quizzes/5/export.imscc", nil)
	resp, _ := app.Test(req, -1)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestQTIExport_OtherTenantQuiz_404(t *testing.T) {
	app, quizRepo, _ := setupQTIExport(t)
	// The tenant-scoped lookup (caller account 1) does not find the quiz.
	quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(nil, assert.AnError)

	req := httptest.NewRequest(http.MethodGet, "/courses/7/quizzes/5/export.imscc", nil)
	resp, _ := app.Test(req, -1)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestQTIExport_SameCourse_SkipsBankDrawCopies(t *testing.T) {
	app, quizRepo, questionRepo := setupQTIExport(t)
	quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(&models.Quiz{ID: 5, CourseID: 7, Title: "Export"}, nil)
	pts := 1.0
	entryID := uint(77)
	questionRepo.On("ListByQuizID", mock.Anything, uint(5), mock.Anything).Return(&repository.PaginatedResult[models.QuizQuestion]{Items: []models.QuizQuestion{
		{ID: 1, QuizID: 5, QuestionType: "short_answer", QuestionText: "Authored question", PointsPossible: &pts, Answers: `[{"id":"a","text":"Paris","weight":100}]`},
		{ID: 2, QuizID: 5, QuestionType: "short_answer", QuestionText: "Drawn bank copy", PointsPossible: &pts, Answers: `[{"id":"b","text":"Rome","weight":100}]`, QuestionBankEntryID: &entryID},
	}}, nil)

	req := httptest.NewRequest(http.MethodGet, "/courses/7/quizzes/5/export.imscc", nil)
	resp, _ := app.Test(req, -1)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	raw, _ := io.ReadAll(resp.Body)
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("zip: %v", err)
	}
	var all bytes.Buffer
	for _, f := range zr.File {
		rc, _ := f.Open()
		_, _ = io.Copy(&all, rc)
		_ = rc.Close()
	}
	assert.Contains(t, all.String(), "Authored question")
	assert.NotContains(t, all.String(), "Drawn bank copy")
}
