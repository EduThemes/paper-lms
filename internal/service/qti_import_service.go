package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/qti"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// QTIImportService orchestrates Canvas-QTI -> Paper LMS persistence.
//
// Architecture: this service is intentionally a thin wrapper. The
// dialect-specific XML parsing lives in internal/qti; this service
// only:
//
//   1. Stages a multipart upload to disk.
//   2. Delegates parsing to qti.Importer (returns an in-memory result).
//   3. Persists the result via the existing item-bank and stimulus
//      services PLUS the QuizRepository / QuizQuestionRepository
//      directly. The QuizService doesn't expose CreateQuiz so we use
//      the repo it composes — we are NOT modifying QuizService itself
//      (per wave-B constraints) but we do depend on the same
//      repository abstraction it depends on.
//
// Transactional semantics (v1 / sync only):
//
//   We deliberately do NOT wrap the persistence loop in a database
//   transaction. Reasons:
//     - The existing services (QuizItemBankService, QuizStimulusService)
//       hold their own GORM scopes; they cannot enlist into an outer
//       transaction without modification, which the wave-B constraints
//       forbid.
//     - Practically, Canvas QTI imports are large but bounded; partial-
//       failure is reported in the ImportSummary's Errors/Warnings so
//       the instructor can re-import or delete the partial quiz via
//       existing UI.
//   A future v2 may swap in a transaction-aware persister; the qti
//   package itself is already transaction-agnostic (it returns an
//   in-memory ImportResult).
type QTIImportService struct {
	importer        qti.Importer
	quizRepo        repository.QuizRepository
	questionRepo    repository.QuizQuestionRepository
	bankService     *QuizItemBankService
	stimulusService *QuizStimulusService
	uploadsRoot     string
}

// NewQTIImportService wires the import pipeline.
//
//   - quizRepo / questionRepo come from the same factories that
//     QuizService uses (postgres.NewQuizRepository, etc.).
//   - bankService and stimulusService are the existing services.
//   - uploadsRoot is the directory where uploaded zips are staged.
func NewQTIImportService(
	quizRepo repository.QuizRepository,
	questionRepo repository.QuizQuestionRepository,
	bankService *QuizItemBankService,
	stimulusService *QuizStimulusService,
	uploadsRoot string,
) *QTIImportService {
	return &QTIImportService{
		importer:        qti.NewImporter(),
		quizRepo:        quizRepo,
		questionRepo:    questionRepo,
		bankService:     bankService,
		stimulusService: stimulusService,
		uploadsRoot:     uploadsRoot,
	}
}

// ImportSummary is the JSON-shaped result the HTTP handler returns.
type ImportSummary struct {
	Dialect          string              `json:"dialect"`
	QuizzesCreated   int                 `json:"quizzes_created"`
	QuestionsCreated int                 `json:"questions_created"`
	BanksCreated     int                 `json:"banks_created"`
	BankItemsCreated int                 `json:"bank_items_created"`
	StimuliCreated   int                 `json:"stimuli_created"`
	QuizIDs          []uint              `json:"quiz_ids"`
	Warnings         []qti.ImportWarning `json:"warnings,omitempty"`
	Errors           []qti.ImportError   `json:"errors,omitempty"`
}

// ImportMultipart accepts a Fiber-style multipart file header, stages
// it to a temp file, parses, and persists.
func (s *QTIImportService) ImportMultipart(ctx context.Context, fh *multipart.FileHeader, courseID, userID uint) (*ImportSummary, error) {
	if fh == nil {
		return nil, errors.New("file is required")
	}
	if courseID == 0 {
		return nil, errors.New("course_id is required")
	}

	dir := filepath.Join(s.uploadsRoot, "qti_imports", fmt.Sprintf("%d", courseID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("staging dir: %w", err)
	}
	zipPath := filepath.Join(dir, sanitizeQTIFilename(fh.Filename))
	if err := saveMultipart(fh, zipPath); err != nil {
		return nil, fmt.Errorf("save upload: %w", err)
	}
	defer os.Remove(zipPath)

	return s.ImportFromPath(ctx, zipPath, courseID, userID)
}

// ImportFromPath imports an already-staged zip — useful for tests
// and CLI re-imports.
func (s *QTIImportService) ImportFromPath(ctx context.Context, zipPath string, courseID, userID uint) (*ImportSummary, error) {
	result, err := s.importer.ImportIMSCC(ctx, zipPath, courseID)
	if err != nil {
		return nil, fmt.Errorf("parse imscc: %w", err)
	}
	return s.persist(ctx, result, courseID, userID)
}

// persist writes an in-memory ImportResult to the database via the
// configured services + repos. Visible to tests so callers can inject
// a hand-built ImportResult without round-tripping through XML.
func (s *QTIImportService) persist(ctx context.Context, result *qti.ImportResult, courseID, userID uint) (*ImportSummary, error) {
	summary := &ImportSummary{
		Dialect:  string(result.Dialect),
		Warnings: result.Warnings,
		Errors:   result.Errors,
	}

	// 1. Stimuli first — questions may reference them.
	stimulusByIdent := map[string]uint{}
	for _, stim := range result.Stimuli {
		row := &models.QuizStimulus{
			CourseID: courseID,
			Title:    stim.Title,
			Content:  stim.Content,
		}
		if row.Title == "" {
			row.Title = "Imported Passage " + stim.Identifier
		}
		if err := s.stimulusService.CreateStimulus(ctx, row); err != nil {
			summary.Warnings = append(summary.Warnings, qti.ImportWarning{
				Source: stim.Identifier, Code: "stimulus_create_failed",
				Message: err.Error(),
			})
			continue
		}
		stimulusByIdent[stim.Identifier] = row.ID
		summary.StimuliCreated++
	}

	// 2. Banks + bank items. Track items by source identifier so
	// question-level <assessmentRef>s can resolve to bank_item_id.
	bankItemByIdent := map[string]uint{}
	for _, bank := range result.ItemBanks {
		row := &models.QuizItemBank{
			CourseID:        courseID,
			Title:           bank.Title,
			Description:     bank.Description,
			CreatedByUserID: userID,
		}
		if err := s.bankService.CreateBank(ctx, row); err != nil {
			summary.Warnings = append(summary.Warnings, qti.ImportWarning{
				Source: bank.Identifier, Code: "bank_create_failed",
				Message: err.Error(),
			})
			continue
		}
		summary.BanksCreated++
		for _, bi := range bank.Items {
			itemRow := &models.QuizItemBankItem{
				BankID:            row.ID,
				Position:          bi.Position,
				QuestionType:      bi.QuestionType,
				QuestionText:      bi.QuestionText,
				PointsPossible:    bi.PointsPossible,
				Answers:           bi.Answers,
				CorrectComments:   bi.CorrectComments,
				IncorrectComments: bi.IncorrectComments,
				NeutralComments:   bi.NeutralComments,
			}
			if err := s.bankService.CreateBankItem(ctx, itemRow); err != nil {
				summary.Warnings = append(summary.Warnings, qti.ImportWarning{
					Source: bi.Identifier, Code: "bank_item_create_failed",
					Message: err.Error(),
				})
				continue
			}
			bankItemByIdent[bi.Identifier] = itemRow.ID
			summary.BankItemsCreated++
		}
	}

	// 3. Quizzes + questions. Resolve stimulus and bank refs as we go.
	for _, qz := range result.Quizzes {
		quizRow := &models.Quiz{
			CourseID:       courseID,
			Title:          qz.Title,
			Description:    qz.Description,
			QuizType:       qz.QuizType,
			TimeLimit:      qz.TimeLimit,
			PointsPossible: qz.PointsPossible,
			ShuffleAnswers: qz.ShuffleAnswers,
			Published:      qz.Published,
			WorkflowState:  "unpublished",
		}
		if quizRow.QuizType == "" {
			quizRow.QuizType = "assignment"
		}
		if quizRow.Title == "" {
			quizRow.Title = "Imported Quiz"
		}
		if err := s.quizRepo.Create(ctx, quizRow); err != nil {
			summary.Errors = append(summary.Errors, qti.ImportError{
				Source: qz.Identifier, Code: "quiz_create_failed",
				Message: err.Error(),
			})
			continue
		}
		summary.QuizzesCreated++
		summary.QuizIDs = append(summary.QuizIDs, quizRow.ID)

		for _, qq := range qz.Questions {
			questionRow := &models.QuizQuestion{
				QuizID:            quizRow.ID,
				Position:          qq.Position,
				QuestionType:      qq.QuestionType,
				QuestionText:      qq.QuestionText,
				PointsPossible:    qq.PointsPossible,
				Answers:           qq.Answers,
				CorrectComments:   qq.CorrectComments,
				IncorrectComments: qq.IncorrectComments,
				NeutralComments:   qq.NeutralComments,
				WorkflowState:     "active",
			}
			if qq.StimulusIdentifier != "" {
				if sid, ok := stimulusByIdent[qq.StimulusIdentifier]; ok {
					questionRow.StimulusID = &sid
				}
			}
			if qq.BankItemIdentifier != "" {
				if bid, ok := bankItemByIdent[qq.BankItemIdentifier]; ok {
					questionRow.BankItemID = &bid
				}
			}
			if questionRow.Answers == "" {
				questionRow.Answers = "[]"
			}
			// Skip bank-reference placeholder questions if the bank
			// item resolved successfully (the bank item is the real
			// question — we don't need a separate placeholder row).
			if qq.BankItemIdentifier != "" && questionRow.BankItemID != nil &&
				questionRow.QuestionType == qti.UnifiedMultipleChoice &&
				len(questionRow.Answers) <= 2 {
				// Bank-ref placeholder; emit a real question that
				// references the bank item but with the bank item's
				// content. The caller can later "expand" it.
				bankItem, err := s.bankService.GetBankItem(ctx, *questionRow.BankItemID)
				if err == nil && bankItem != nil {
					questionRow.QuestionType = bankItem.QuestionType
					questionRow.QuestionText = bankItem.QuestionText
					questionRow.Answers = bankItem.Answers
					if bankItem.PointsPossible != nil {
						questionRow.PointsPossible = bankItem.PointsPossible
					}
				}
			}
			if err := s.questionRepo.Create(ctx, questionRow); err != nil {
				summary.Warnings = append(summary.Warnings, qti.ImportWarning{
					Source: qq.SourceIdentifier, Code: "question_create_failed",
					Message: err.Error(),
				})
				continue
			}
			summary.QuestionsCreated++
		}
	}

	return summary, nil
}

// ExportQuiz emits a Canvas-Classic-compatible .imscc zip for the given
// quiz. Optionally includes referenced item banks.
func (s *QTIImportService) ExportQuiz(ctx context.Context, quizID uint) ([]byte, error) {
	quiz, err := s.quizRepo.FindByID(ctx, quizID, 0)
	if err != nil {
		return nil, fmt.Errorf("quiz not found: %w", err)
	}
	page, err := s.questionRepo.ListByQuizID(ctx, quizID, repository.PaginationParams{Page: 1, PerPage: 10000})
	if err != nil {
		return nil, err
	}
	// Collect referenced banks for self-contained export.
	bankSeen := map[uint]bool{}
	banks := []qti.ItemBankImport{}
	for _, q := range page.Items {
		if q.BankItemID == nil || bankSeen[*q.BankItemID] {
			continue
		}
		bankSeen[*q.BankItemID] = true
		bi, err := s.bankService.GetBankItem(ctx, *q.BankItemID)
		if err != nil {
			continue
		}
		banks = append(banks, qti.ItemBankImport{
			Identifier: fmt.Sprintf("bank-%d", bi.BankID),
			Title:      "Exported Bank",
			Items: []qti.BankItemImport{{
				Identifier:        fmt.Sprintf("bi-%d", bi.ID),
				Position:          bi.Position,
				QuestionType:      bi.QuestionType,
				QuestionText:      bi.QuestionText,
				PointsPossible:    bi.PointsPossible,
				Answers:           bi.Answers,
				CorrectComments:   bi.CorrectComments,
				IncorrectComments: bi.IncorrectComments,
				NeutralComments:   bi.NeutralComments,
			}},
		})
	}
	return qti.NewExporter().ExportQuiz(quiz, page.Items, banks)
}

// --- small helpers ---

func sanitizeQTIFilename(s string) string {
	if s == "" {
		return "upload.imscc"
	}
	// Strip directory components; keep extension.
	base := filepath.Base(s)
	out := []rune{}
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '.', r == '-', r == '_':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}

func saveMultipart(fh *multipart.FileHeader, dst string) error {
	src, err := fh.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, src)
	return err
}
