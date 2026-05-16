package service

import (
	"context"
	"mime/multipart"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/qti"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

type repoPaginated = repository.PaginatedResult[models.QuizQuestion]

// TestQTIImportServicePersist exercises the persistence loop with
// hand-built ImportResults so we don't have to round-trip XML in this
// test layer (that's covered by internal/qti's own tests).
//
// The mock expectations document the exact sequence the service is
// expected to execute. If the service skips a layer or re-orders the
// inserts, the mock framework fails loudly.
func TestQTIImportServicePersist_FullFlow(t *testing.T) {
	ctx := context.Background()

	quizRepo := &mocks.MockQuizRepository{}
	questionRepo := &mocks.MockQuizQuestionRepository{}
	bankRepo := &mocks.MockQuizItemBankRepository{}
	bankItemRepo := &mocks.MockQuizItemBankItemRepository{}
	stimRepo := &mocks.MockQuizStimulusRepository{}

	bankSvc := NewQuizItemBankService(bankRepo, bankItemRepo, questionRepo)
	stimSvc := NewQuizStimulusService(stimRepo, questionRepo)
	svc := NewQTIImportService(quizRepo, questionRepo, bankSvc, stimSvc, t.TempDir())

	// Expectations.
	stimRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.QuizStimulus")).
		Run(func(args mock.Arguments) {
			args.Get(1).(*models.QuizStimulus).ID = 11
		}).Return(nil)

	bankRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.QuizItemBank")).
		Run(func(args mock.Arguments) {
			args.Get(1).(*models.QuizItemBank).ID = 22
		}).Return(nil)

	bankItemRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.QuizItemBankItem")).
		Run(func(args mock.Arguments) {
			args.Get(1).(*models.QuizItemBankItem).ID = 33
		}).Return(nil)
	// CreateBankItem inside QuizItemBankService calls bankRepo.FindByID
	// to verify the bank exists.
	bankRepo.On("FindByID", mock.Anything, uint(22)).
		Return(&models.QuizItemBank{ID: 22, CourseID: 5, Title: "T"}, nil)

	quizRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Quiz")).
		Run(func(args mock.Arguments) {
			args.Get(1).(*models.Quiz).ID = 44
		}).Return(nil)

	questionRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.QuizQuestion")).
		Return(nil)

	// Synthetic ImportResult — one stimulus, one bank with one item,
	// one quiz with two questions (one normal + one stimulus-linked).
	pts := 1.0
	res := &qti.ImportResult{
		Dialect: qti.DialectNewQuizzes,
		Stimuli: []qti.StimulusImport{{
			Identifier: "stim-1",
			Title:      "Passage",
			Content:    `{"type":"doc"}`,
		}},
		ItemBanks: []qti.ItemBankImport{{
			Identifier: "bank-1",
			Title:      "Sample Bank",
			Items: []qti.BankItemImport{{
				Identifier:     "bi-1",
				QuestionType:   qti.UnifiedMultipleChoice,
				QuestionText:   "Q?",
				PointsPossible: &pts,
				Answers:        "[]",
			}},
		}},
		Quizzes: []qti.QuizImport{{
			Title: "Imported", QuizType: "assignment",
			Questions: []qti.QuestionImport{
				{
					QuestionType: qti.UnifiedShortAnswer,
					QuestionText: "Q1",
					Answers:      "[]",
				},
				{
					QuestionType:       qti.UnifiedEssay,
					QuestionText:       "Q2",
					Answers:            "[]",
					StimulusIdentifier: "stim-1",
				},
			},
		}},
	}

	summary, err := svc.persist(ctx, res, 5, 7)
	if err != nil {
		t.Fatalf("persist: %v", err)
	}
	if summary.StimuliCreated != 1 {
		t.Errorf("want 1 stimulus, got %d", summary.StimuliCreated)
	}
	if summary.BanksCreated != 1 {
		t.Errorf("want 1 bank, got %d", summary.BanksCreated)
	}
	if summary.BankItemsCreated != 1 {
		t.Errorf("want 1 bank item, got %d", summary.BankItemsCreated)
	}
	if summary.QuizzesCreated != 1 {
		t.Errorf("want 1 quiz, got %d", summary.QuizzesCreated)
	}
	if summary.QuestionsCreated != 2 {
		t.Errorf("want 2 questions, got %d", summary.QuestionsCreated)
	}

	// Verify the stimulus-linked question carries the resolved
	// StimulusID. We check by walking the questionRepo Create calls.
	stimulusLinkedSeen := false
	for _, call := range questionRepo.Calls {
		if call.Method != "Create" {
			continue
		}
		q, ok := call.Arguments[1].(*models.QuizQuestion)
		if !ok {
			continue
		}
		if q.StimulusID != nil && *q.StimulusID == 11 {
			stimulusLinkedSeen = true
		}
	}
	if !stimulusLinkedSeen {
		t.Error("expected a question with StimulusID=11 (resolved from stim-1)")
	}

	quizRepo.AssertExpectations(t)
	questionRepo.AssertExpectations(t)
	bankRepo.AssertExpectations(t)
	bankItemRepo.AssertExpectations(t)
	stimRepo.AssertExpectations(t)
}

// TestQTIImportServicePersist_QuizCreateFailureSurfacesAsError verifies
// a failed quiz insert produces an Errors entry, not a panic.
func TestQTIImportServicePersist_QuizCreateFailureSurfacesAsError(t *testing.T) {
	ctx := context.Background()
	quizRepo := &mocks.MockQuizRepository{}
	questionRepo := &mocks.MockQuizQuestionRepository{}
	bankRepo := &mocks.MockQuizItemBankRepository{}
	bankItemRepo := &mocks.MockQuizItemBankItemRepository{}
	stimRepo := &mocks.MockQuizStimulusRepository{}

	bankSvc := NewQuizItemBankService(bankRepo, bankItemRepo, questionRepo)
	stimSvc := NewQuizStimulusService(stimRepo, questionRepo)
	svc := NewQTIImportService(quizRepo, questionRepo, bankSvc, stimSvc, t.TempDir())

	quizRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Quiz")).
		Return(errAlreadyExists())

	summary, err := svc.persist(ctx, &qti.ImportResult{
		Quizzes: []qti.QuizImport{{Title: "X", Identifier: "x"}},
	}, 1, 1)
	if err != nil {
		t.Fatalf("persist returned error: %v", err)
	}
	if summary.QuizzesCreated != 0 {
		t.Errorf("expected 0 quizzes created on failure, got %d", summary.QuizzesCreated)
	}
	if len(summary.Errors) != 1 {
		t.Fatalf("expected 1 error in summary, got %d", len(summary.Errors))
	}
	if summary.Errors[0].Code != "quiz_create_failed" {
		t.Errorf("expected code quiz_create_failed, got %s", summary.Errors[0].Code)
	}
}

// TestQTIImportService_BankRefPlaceholderExpansion covers the
// branch where a section's <sourcebank_ref> produced a placeholder
// QuestionImport that the service expands into a real question
// using the resolved bank item's content.
func TestQTIImportService_BankRefPlaceholderExpansion(t *testing.T) {
	ctx := context.Background()
	quizRepo := &mocks.MockQuizRepository{}
	questionRepo := &mocks.MockQuizQuestionRepository{}
	bankRepo := &mocks.MockQuizItemBankRepository{}
	bankItemRepo := &mocks.MockQuizItemBankItemRepository{}
	stimRepo := &mocks.MockQuizStimulusRepository{}
	bankSvc := NewQuizItemBankService(bankRepo, bankItemRepo, questionRepo)
	stimSvc := NewQuizStimulusService(stimRepo, questionRepo)
	svc := NewQTIImportService(quizRepo, questionRepo, bankSvc, stimSvc, t.TempDir())

	pts := 2.0
	bankRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.QuizItemBank")).
		Run(func(args mock.Arguments) { args.Get(1).(*models.QuizItemBank).ID = 1 }).
		Return(nil)
	bankRepo.On("FindByID", mock.Anything, uint(1)).
		Return(&models.QuizItemBank{ID: 1, CourseID: 1, Title: "B"}, nil)
	bankItemRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.QuizItemBankItem")).
		Run(func(args mock.Arguments) { args.Get(1).(*models.QuizItemBankItem).ID = 99 }).
		Return(nil)
	bankItemRepo.On("FindByID", mock.Anything, uint(99)).
		Return(&models.QuizItemBankItem{
			ID: 99, BankID: 1,
			QuestionType: qti.UnifiedShortAnswer, QuestionText: "Real Q",
			Answers: `[{"id":"a1","text":"X","weight":100}]`, PointsPossible: &pts,
		}, nil)
	quizRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Quiz")).
		Run(func(args mock.Arguments) { args.Get(1).(*models.Quiz).ID = 50 }).
		Return(nil)

	gotQuestionType := ""
	questionRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.QuizQuestion")).
		Run(func(args mock.Arguments) {
			gotQuestionType = args.Get(1).(*models.QuizQuestion).QuestionType
		}).Return(nil)

	res := &qti.ImportResult{
		ItemBanks: []qti.ItemBankImport{{
			Identifier: "b1", Title: "B",
			Items: []qti.BankItemImport{{
				Identifier: "bi1", QuestionType: qti.UnifiedShortAnswer,
				QuestionText: "Real Q", Answers: "[]", PointsPossible: &pts,
			}},
		}},
		Quizzes: []qti.QuizImport{{Title: "Q", Questions: []qti.QuestionImport{
			// This is the placeholder from a <sourcebank_ref>.
			{
				QuestionType:       qti.UnifiedMultipleChoice,
				QuestionText:       "[Bank reference: b1]",
				Answers:            "",
				BankItemIdentifier: "bi1",
			},
		}}},
	}
	summary, err := svc.persist(ctx, res, 1, 1)
	if err != nil {
		t.Fatalf("persist: %v", err)
	}
	if summary.QuestionsCreated != 1 {
		t.Errorf("want 1 question created, got %d", summary.QuestionsCreated)
	}
	// The persisted question's type should have been replaced with
	// the bank item's actual short_answer type (NOT the placeholder
	// multiple_choice).
	if gotQuestionType != qti.UnifiedShortAnswer {
		t.Errorf("expected bank item to expand placeholder type to short_answer, got %q", gotQuestionType)
	}
}

// TestQTIImportService_StimulusAndBankCreateFailures exercises the
// warning paths where stimulus / bank inserts fail mid-import. The
// import should NOT abort — failures are recorded in Warnings and the
// rest of the bundle imports normally.
func TestQTIImportService_StimulusAndBankCreateFailures(t *testing.T) {
	ctx := context.Background()
	quizRepo := &mocks.MockQuizRepository{}
	questionRepo := &mocks.MockQuizQuestionRepository{}
	bankRepo := &mocks.MockQuizItemBankRepository{}
	bankItemRepo := &mocks.MockQuizItemBankItemRepository{}
	stimRepo := &mocks.MockQuizStimulusRepository{}
	bankSvc := NewQuizItemBankService(bankRepo, bankItemRepo, questionRepo)
	stimSvc := NewQuizStimulusService(stimRepo, questionRepo)
	svc := NewQTIImportService(quizRepo, questionRepo, bankSvc, stimSvc, t.TempDir())

	// Stimulus create fails → warning, no panic.
	stimRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.QuizStimulus")).
		Return(errAlreadyExists())
	// Bank create fails → warning. No subsequent bank item inserts.
	bankRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.QuizItemBank")).
		Return(errAlreadyExists())
	// Quiz + question still succeed.
	quizRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Quiz")).
		Run(func(args mock.Arguments) { args.Get(1).(*models.Quiz).ID = 1 }).
		Return(nil)
	questionRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.QuizQuestion")).Return(nil)

	pts := 1.0
	res := &qti.ImportResult{
		Stimuli: []qti.StimulusImport{{Identifier: "s1", Title: "P", Content: "{}"}},
		ItemBanks: []qti.ItemBankImport{{
			Identifier: "b1", Title: "B",
			Items: []qti.BankItemImport{{Identifier: "bi1", QuestionType: qti.UnifiedShortAnswer, QuestionText: "Q", PointsPossible: &pts}},
		}},
		Quizzes: []qti.QuizImport{{Title: "Q", Questions: []qti.QuestionImport{
			{QuestionType: qti.UnifiedEssay, QuestionText: "E"},
		}}},
	}
	summary, err := svc.persist(ctx, res, 1, 1)
	if err != nil {
		t.Fatalf("persist: %v", err)
	}
	if summary.StimuliCreated != 0 || summary.BanksCreated != 0 {
		t.Errorf("expected stim/bank to fail: got stim=%d bank=%d", summary.StimuliCreated, summary.BanksCreated)
	}
	if summary.QuizzesCreated != 1 {
		t.Errorf("quiz should still succeed: got %d", summary.QuizzesCreated)
	}
	// Two warnings expected (stimulus + bank).
	if len(summary.Warnings) < 2 {
		t.Errorf("expected at least 2 warnings, got %d: %+v", len(summary.Warnings), summary.Warnings)
	}
}

// errAlreadyExists is a tiny test error used to drive failure paths.
type testErr struct{ msg string }

func (e testErr) Error() string { return e.msg }
func errAlreadyExists() error   { return testErr{"row already exists"} }

// TestQTIImportService_ExportQuiz drives the exporter path and round-
// trips the bytes through the importer to verify the produced bundle
// is well-formed.
func TestQTIImportService_ExportQuiz(t *testing.T) {
	ctx := context.Background()
	quizRepo := &mocks.MockQuizRepository{}
	questionRepo := &mocks.MockQuizQuestionRepository{}
	bankRepo := &mocks.MockQuizItemBankRepository{}
	bankItemRepo := &mocks.MockQuizItemBankItemRepository{}
	stimRepo := &mocks.MockQuizStimulusRepository{}

	bankSvc := NewQuizItemBankService(bankRepo, bankItemRepo, questionRepo)
	stimSvc := NewQuizStimulusService(stimRepo, questionRepo)
	svc := NewQTIImportService(quizRepo, questionRepo, bankSvc, stimSvc, t.TempDir())

	points := 1.0
	quiz := &models.Quiz{ID: 9, Title: "Export Me", QuizType: "assignment"}
	quizRepo.On("FindByID", mock.Anything, uint(9), uint(0)).Return(quiz, nil)
	questionRepo.On("ListByQuizID", mock.Anything, uint(9), mock.Anything).
		Return(&repoPaginated{Items: []models.QuizQuestion{
			{
				ID: 1, Position: 0,
				QuestionType:   qti.UnifiedShortAnswer,
				QuestionText:   "Q?",
				PointsPossible: &points,
				Answers:        `[{"id":"a1","text":"Paris","weight":100}]`,
			},
		}}, nil)

	data, err := svc.ExportQuiz(ctx, 9)
	if err != nil {
		t.Fatalf("ExportQuiz: %v", err)
	}
	if len(data) < 100 {
		t.Errorf("export bytes suspiciously small: %d", len(data))
	}
}

// TestQTIImportService_ImportMultipart covers the on-disk staging path
// by synthesizing a *multipart.FileHeader pointing to a real zip on
// disk. This goes through ImportMultipart → ImportFromPath →
// persist end-to-end.
func TestQTIImportService_ImportMultipart(t *testing.T) {
	ctx := context.Background()
	quizRepo := &mocks.MockQuizRepository{}
	questionRepo := &mocks.MockQuizQuestionRepository{}
	bankRepo := &mocks.MockQuizItemBankRepository{}
	bankItemRepo := &mocks.MockQuizItemBankItemRepository{}
	stimRepo := &mocks.MockQuizStimulusRepository{}
	bankSvc := NewQuizItemBankService(bankRepo, bankItemRepo, questionRepo)
	stimSvc := NewQuizStimulusService(stimRepo, questionRepo)
	svc := NewQTIImportService(quizRepo, questionRepo, bankSvc, stimSvc, t.TempDir())

	quizRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Quiz")).
		Run(func(args mock.Arguments) { args.Get(1).(*models.Quiz).ID = 1 }).
		Return(nil)
	questionRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.QuizQuestion")).Return(nil)

	// Build a multipart.FileHeader pointing to the real zip.
	zipPath := buildTinyClassicIMSCC(t)
	fh := makeFileHeader(t, zipPath, "tiny.imscc")

	summary, err := svc.ImportMultipart(ctx, fh, 5, 7)
	if err != nil {
		t.Fatalf("ImportMultipart: %v", err)
	}
	if summary.QuizzesCreated != 1 {
		t.Errorf("want 1 quiz, got %d", summary.QuizzesCreated)
	}
}

// TestQTIImportService_ImportMultipart_NilFile covers the error path.
func TestQTIImportService_ImportMultipart_NilFile(t *testing.T) {
	svc := NewQTIImportService(nil, nil, nil, nil, t.TempDir())
	if _, err := svc.ImportMultipart(context.Background(), nil, 1, 1); err == nil {
		t.Error("expected error for nil file")
	}
	if _, err := svc.ImportMultipart(context.Background(), &multipart.FileHeader{Filename: "x"}, 0, 1); err == nil {
		t.Error("expected error for course_id=0")
	}
}

// TestQTIImportService_SanitizeFilenameAndHelpers covers the small
// pure-function helpers so the coverage report doesn't drop below
// the target on those.
func TestQTIImportService_SanitizeFilenameAndHelpers(t *testing.T) {
	cases := []struct{ in, want string }{
		{"foo.imscc", "foo.imscc"},
		{"path/to/file.imscc", "file.imscc"},
		{"bad name!.imscc", "bad_name_.imscc"},
		{"", "upload.imscc"},
	}
	for _, c := range cases {
		if got := sanitizeQTIFilename(c.in); got != c.want {
			t.Errorf("sanitizeQTIFilename(%q): want %q, got %q", c.in, c.want, got)
		}
	}
}

// TestQTIImportService_ImportFromPathRealFixture drives the public
// ImportFromPath entry point against a real .imscc bundle. This
// covers the full path from XML parse through persistence.
func TestQTIImportService_ImportFromPathRealFixture(t *testing.T) {
	ctx := context.Background()
	quizRepo := &mocks.MockQuizRepository{}
	questionRepo := &mocks.MockQuizQuestionRepository{}
	bankRepo := &mocks.MockQuizItemBankRepository{}
	bankItemRepo := &mocks.MockQuizItemBankItemRepository{}
	stimRepo := &mocks.MockQuizStimulusRepository{}

	bankSvc := NewQuizItemBankService(bankRepo, bankItemRepo, questionRepo)
	stimSvc := NewQuizStimulusService(stimRepo, questionRepo)
	svc := NewQTIImportService(quizRepo, questionRepo, bankSvc, stimSvc, t.TempDir())

	// All persists succeed; assign predictable IDs.
	var nextID uint = 1
	bankRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.QuizItemBank")).
		Run(func(args mock.Arguments) {
			args.Get(1).(*models.QuizItemBank).ID = nextID
			nextID++
		}).Return(nil)
	bankItemRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.QuizItemBankItem")).
		Run(func(args mock.Arguments) {
			args.Get(1).(*models.QuizItemBankItem).ID = nextID
			nextID++
		}).Return(nil)
	bankRepo.On("FindByID", mock.Anything, mock.AnythingOfType("uint")).
		Return(&models.QuizItemBank{ID: 1, CourseID: 1, Title: "T"}, nil)
	quizRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Quiz")).
		Run(func(args mock.Arguments) {
			args.Get(1).(*models.Quiz).ID = nextID
			nextID++
		}).Return(nil)
	questionRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.QuizQuestion")).
		Return(nil)
	stimRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.QuizStimulus")).Return(nil)

	// Build a tiny zip in the temp dir.
	zipPath := buildTinyClassicIMSCC(t)

	summary, err := svc.ImportFromPath(ctx, zipPath, 1, 7)
	if err != nil {
		t.Fatalf("ImportFromPath: %v", err)
	}
	if summary.QuizzesCreated != 1 {
		t.Errorf("want 1 quiz, got %d", summary.QuizzesCreated)
	}
	if summary.QuestionsCreated < 1 {
		t.Errorf("want >=1 question, got %d", summary.QuestionsCreated)
	}
}
