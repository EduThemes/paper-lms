package service

// QuizService is the public facade for all quiz operations (authoring,
// attempts, grading, statistics). The implementation is split across four
// sibling files for navigability — see quiz_authoring_service.go,
// quiz_attempts_service.go, quiz_grading_service.go, and
// quiz_statistics_service.go. All exported methods continue to hang off
// this single struct so the handler call sites in
// internal/api/v1/handlers/quiz_*.go remain untouched.
//
// This file owns ONLY:
//   - the QuizService struct + its repository / dependency fields
//   - the NewQuizService constructor + functional options
//
// All behavior lives in the sibling files. The split was made in Wave 5
// (chore/wave5-split-quiz-blueprint) because quiz_service.go had grown
// past 1100 LOC across four very different concerns.

import (
	"github.com/EduThemes/paper-lms/internal/repository"
)

type QuizService struct {
	quizRepo             repository.QuizRepository
	questionRepo         repository.QuizQuestionRepository
	submissionRepo       repository.QuizSubmissionRepository
	answerRepo           repository.QuizSubmissionAnswerRepository
	groupRepo            repository.QuizQuestionGroupRepository
	bankEntryRepo        repository.QuestionBankEntryRepository
	accommodationService *AccommodationService

	// onCompletedCallbacks fire (in goroutines) after a successful
	// CompleteSubmission. Registered via OnCompleted; never invoked in
	// tests unless explicitly wired.
	onCompletedCallbacks []QuizCompletedCallback
}

func NewQuizService(
	quizRepo repository.QuizRepository,
	questionRepo repository.QuizQuestionRepository,
	submissionRepo repository.QuizSubmissionRepository,
	answerRepo repository.QuizSubmissionAnswerRepository,
	opts ...func(*QuizService),
) *QuizService {
	s := &QuizService{
		quizRepo:       quizRepo,
		questionRepo:   questionRepo,
		submissionRepo: submissionRepo,
		answerRepo:     answerRepo,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func WithQuestionGroupRepo(repo repository.QuizQuestionGroupRepository) func(*QuizService) {
	return func(s *QuizService) {
		s.groupRepo = repo
	}
}

func WithBankEntryRepo(repo repository.QuestionBankEntryRepository) func(*QuizService) {
	return func(s *QuizService) {
		s.bankEntryRepo = repo
	}
}

func WithAccommodationService(svc *AccommodationService) func(*QuizService) {
	return func(s *QuizService) {
		s.accommodationService = svc
	}
}
