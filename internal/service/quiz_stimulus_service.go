package service

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// QuizStimulusService manages reusable stimulus passages (TipTap content)
// shared across multiple QuizQuestions.
type QuizStimulusService struct {
	stimulusRepo repository.QuizStimulusRepository
	questionRepo repository.QuizQuestionRepository
}

func NewQuizStimulusService(
	stimulusRepo repository.QuizStimulusRepository,
	questionRepo repository.QuizQuestionRepository,
) *QuizStimulusService {
	return &QuizStimulusService{
		stimulusRepo: stimulusRepo,
		questionRepo: questionRepo,
	}
}

func (s *QuizStimulusService) CreateStimulus(ctx context.Context, stim *models.QuizStimulus) error {
	if stim == nil {
		return errors.New("stimulus is required")
	}
	if stim.CourseID == 0 {
		return errors.New("course_id is required")
	}
	if stim.Title == "" {
		return errors.New("title is required")
	}
	if stim.Content == "" {
		stim.Content = "{}"
	}
	return s.stimulusRepo.Create(ctx, stim)
}

func (s *QuizStimulusService) GetStimulus(ctx context.Context, courseID, id uint) (*models.QuizStimulus, error) {
	stim, err := s.stimulusRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("stimulus not found")
	}
	if courseID != 0 && stim.CourseID != courseID {
		return nil, errors.New("stimulus does not belong to course")
	}
	return stim, nil
}

func (s *QuizStimulusService) ListStimuli(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.QuizStimulus], error) {
	if courseID == 0 {
		return nil, errors.New("course_id is required")
	}
	return s.stimulusRepo.ListByCourseID(ctx, courseID, params)
}

func (s *QuizStimulusService) UpdateStimulus(ctx context.Context, courseID uint, stim *models.QuizStimulus) error {
	if stim == nil || stim.ID == 0 {
		return errors.New("stimulus id is required")
	}
	existing, err := s.stimulusRepo.FindByID(ctx, stim.ID)
	if err != nil {
		return errors.New("stimulus not found")
	}
	if courseID != 0 && existing.CourseID != courseID {
		return errors.New("stimulus does not belong to course")
	}
	stim.CourseID = existing.CourseID
	stim.CreatedAt = existing.CreatedAt
	return s.stimulusRepo.Update(ctx, stim)
}

func (s *QuizStimulusService) DeleteStimulus(ctx context.Context, courseID, id uint) error {
	existing, err := s.stimulusRepo.FindByID(ctx, id)
	if err != nil {
		return errors.New("stimulus not found")
	}
	if courseID != 0 && existing.CourseID != courseID {
		return errors.New("stimulus does not belong to course")
	}
	return s.stimulusRepo.Delete(ctx, id)
}

// LinkQuestionToStimulus sets the QuizQuestion's stimulus_id pointer.
func (s *QuizStimulusService) LinkQuestionToStimulus(ctx context.Context, questionID, stimulusID uint) error {
	if questionID == 0 {
		return errors.New("question_id is required")
	}
	if stimulusID == 0 {
		return errors.New("stimulus_id is required")
	}
	if _, err := s.questionRepo.FindByID(ctx, questionID); err != nil {
		return errors.New("quiz question not found")
	}
	if _, err := s.stimulusRepo.FindByID(ctx, stimulusID); err != nil {
		return errors.New("stimulus not found")
	}
	sid := stimulusID
	return s.stimulusRepo.SetQuestionStimulus(ctx, questionID, &sid)
}

// UnlinkQuestionFromStimulus clears the stimulus FK on a quiz question.
func (s *QuizStimulusService) UnlinkQuestionFromStimulus(ctx context.Context, questionID uint) error {
	if questionID == 0 {
		return errors.New("question_id is required")
	}
	if _, err := s.questionRepo.FindByID(ctx, questionID); err != nil {
		return errors.New("quiz question not found")
	}
	return s.stimulusRepo.SetQuestionStimulus(ctx, questionID, nil)
}

func (s *QuizStimulusService) ListQuestionsForStimulus(ctx context.Context, stimulusID uint) ([]models.QuizQuestion, error) {
	if stimulusID == 0 {
		return nil, errors.New("stimulus_id is required")
	}
	if _, err := s.stimulusRepo.FindByID(ctx, stimulusID); err != nil {
		return nil, errors.New("stimulus not found")
	}
	return s.stimulusRepo.ListQuestionsForStimulus(ctx, stimulusID)
}
