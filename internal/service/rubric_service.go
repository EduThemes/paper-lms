package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// CriterionAssessment represents the assessment data for a single criterion.
// Data format: {"criterion_1": {"points": 5, "comments": "Good"}, "criterion_2": {"points": 3, "comments": ""}}
type CriterionAssessment struct {
	Points   float64 `json:"points"`
	Comments string  `json:"comments"`
}

// RubricAssessmentCreatedCallback fires (asynchronously) after a rubric
// assessment is successfully created. Receives the new assessment's ID.
// Same contract as SubmissionGradedCallback: detached context, no panic,
// no error return.
type RubricAssessmentCreatedCallback func(ctx context.Context, assessmentID uint)

type RubricService struct {
	rubricRepo repository.RubricRepository
	assocRepo  repository.RubricAssociationRepository
	assessRepo repository.RubricAssessmentRepository

	// onAssessmentCreatedCallbacks fire (in goroutines) after a
	// successful CreateAssessment.
	onAssessmentCreatedCallbacks []RubricAssessmentCreatedCallback
}

func NewRubricService(
	rubricRepo repository.RubricRepository,
	assocRepo repository.RubricAssociationRepository,
	assessRepo repository.RubricAssessmentRepository,
) *RubricService {
	return &RubricService{
		rubricRepo: rubricRepo,
		assocRepo:  assocRepo,
		assessRepo: assessRepo,
	}
}

// OnAssessmentCreated registers a callback to fire after a successful
// rubric-assessment write. Callbacks run in fresh goroutines with a
// detached context; panics are recovered.
func (s *RubricService) OnAssessmentCreated(cb RubricAssessmentCreatedCallback) {
	s.onAssessmentCreatedCallbacks = append(s.onAssessmentCreatedCallbacks, cb)
}

func (s *RubricService) fireOnAssessmentCreated(assessmentID uint) {
	for _, cb := range s.onAssessmentCreatedCallbacks {
		go func(cb RubricAssessmentCreatedCallback) {
			defer recoverFromPanic("rubric OnAssessmentCreated callback")
			cb(context.Background(), assessmentID)
		}(cb)
	}
}

// Rubric methods

func (s *RubricService) CreateRubric(ctx context.Context, rubric *models.Rubric) error {
	if rubric.Title == "" {
		return errors.New("rubric title is required")
	}
	if rubric.ContextType == "" {
		return errors.New("rubric context_type is required")
	}
	if rubric.ContextID == 0 {
		return errors.New("rubric context_id is required")
	}
	if rubric.WorkflowState == "" {
		rubric.WorkflowState = "active"
	}
	return s.rubricRepo.Create(ctx, rubric)
}

func (s *RubricService) GetRubric(ctx context.Context, id, accountID uint) (*models.Rubric, error) {
	return s.rubricRepo.FindByID(ctx, id, accountID)
}

func (s *RubricService) UpdateRubric(ctx context.Context, rubric *models.Rubric) error {
	return s.rubricRepo.Update(ctx, rubric)
}

func (s *RubricService) DeleteRubric(ctx context.Context, id uint) error {
	return s.rubricRepo.Delete(ctx, id)
}

func (s *RubricService) ListRubricsByContext(ctx context.Context, contextType string, contextID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Rubric], error) {
	return s.rubricRepo.ListByContext(ctx, contextType, contextID, accountID, params)
}

// Association methods

func (s *RubricService) CreateAssociation(ctx context.Context, rubricID, associationID uint, associationType string, useForGrading bool, accountID uint) (*models.RubricAssociation, error) {
	rubric, err := s.rubricRepo.FindByID(ctx, rubricID, accountID)
	if err != nil {
		return nil, errors.New("rubric not found")
	}

	assoc := &models.RubricAssociation{
		RubricID:        rubricID,
		AssociationID:   associationID,
		AssociationType: associationType,
		ContextType:     rubric.ContextType,
		ContextID:       rubric.ContextID,
		Purpose:         "grading",
		UseForGrading:   useForGrading,
	}

	if err := s.assocRepo.Create(ctx, assoc); err != nil {
		return nil, err
	}

	return assoc, nil
}

func (s *RubricService) GetAssociation(ctx context.Context, id uint) (*models.RubricAssociation, error) {
	return s.assocRepo.FindByID(ctx, id)
}

func (s *RubricService) DeleteAssociation(ctx context.Context, id uint) error {
	return s.assocRepo.Delete(ctx, id)
}

func (s *RubricService) GetRubricForAssignment(ctx context.Context, assignmentID, accountID uint) (*models.Rubric, *models.RubricAssociation, error) {
	assoc, err := s.assocRepo.FindByAssociation(ctx, assignmentID, "Assignment")
	if err != nil {
		return nil, nil, err
	}

	rubric, err := s.rubricRepo.FindByID(ctx, assoc.RubricID, accountID)
	if err != nil {
		return nil, nil, err
	}

	return rubric, assoc, nil
}

// Assessment methods

func (s *RubricService) CreateAssessment(ctx context.Context, assessment *models.RubricAssessment) error {
	if assessment.RubricID == 0 {
		return errors.New("rubric_id is required")
	}
	if assessment.RubricAssociationID == 0 {
		return errors.New("rubric_association_id is required")
	}
	if assessment.UserID == 0 {
		return errors.New("user_id is required")
	}
	if assessment.AssessorID == 0 {
		return errors.New("assessor_id is required")
	}
	if assessment.AssessmentType == "" {
		assessment.AssessmentType = "grading"
	}
	if assessment.WorkflowState == "" {
		assessment.WorkflowState = "active"
	}

	// Calculate total score from data JSON
	if assessment.Data != "" {
		score, err := calculateRubricScore(assessment.Data)
		if err == nil {
			assessment.Score = &score
		}
	}

	if err := s.assessRepo.Create(ctx, assessment); err != nil {
		return err
	}
	s.fireOnAssessmentCreated(assessment.ID)
	return nil
}

func (s *RubricService) GetAssessment(ctx context.Context, id uint) (*models.RubricAssessment, error) {
	return s.assessRepo.FindByID(ctx, id)
}

func (s *RubricService) UpdateAssessment(ctx context.Context, assessment *models.RubricAssessment) error {
	// Recalculate total score from data JSON if data is present
	if assessment.Data != "" {
		score, err := calculateRubricScore(assessment.Data)
		if err == nil {
			assessment.Score = &score
		}
	}

	return s.assessRepo.Update(ctx, assessment)
}

func (s *RubricService) ListAssessmentsByAssociation(ctx context.Context, rubricAssocID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.RubricAssessment], error) {
	return s.assessRepo.ListByAssociationID(ctx, rubricAssocID, params)
}

// calculateRubricScore parses the assessment data JSON and sums all criterion points.
// Data format: {"criterion_1": {"points": 5, "comments": "Good"}, "criterion_2": {"points": 3, "comments": ""}}
func calculateRubricScore(data string) (float64, error) {
	var criteria map[string]CriterionAssessment
	if err := json.Unmarshal([]byte(data), &criteria); err != nil {
		return 0, err
	}

	var total float64
	for _, c := range criteria {
		total += c.Points
	}
	return total, nil
}
