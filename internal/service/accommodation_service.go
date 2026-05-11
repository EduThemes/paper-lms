package service

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
)

// AccommodationService provides business logic for student accommodation management.
type AccommodationService struct {
	accommodationRepo postgres.StudentAccommodationRepository
	applicationRepo   postgres.AccommodationApplicationRepository
}

// NewAccommodationService creates a new AccommodationService with the given repository dependencies.
func NewAccommodationService(
	accommodationRepo postgres.StudentAccommodationRepository,
	applicationRepo postgres.AccommodationApplicationRepository,
) *AccommodationService {
	return &AccommodationService{
		accommodationRepo: accommodationRepo,
		applicationRepo:   applicationRepo,
	}
}

// CreateAccommodation validates and creates a new student accommodation.
func (s *AccommodationService) CreateAccommodation(ctx context.Context, accommodation *models.StudentAccommodation) error {
	if accommodation.UserID == 0 {
		return errors.New("user_id is required")
	}
	if accommodation.AccommodationType == "" {
		return errors.New("accommodation_type is required")
	}

	validTypes := map[string]bool{
		"extended_time":         true,
		"modified_due_dates":    true,
		"alternative_format":    true,
		"reduced_assignments":   true,
		"preferential_seating":  true,
		"assistive_tech":        true,
		"other":                 true,
	}
	if !validTypes[accommodation.AccommodationType] {
		return errors.New("invalid accommodation_type")
	}

	if accommodation.Status == "" {
		accommodation.Status = "active"
	}
	if accommodation.EffectiveFrom.IsZero() {
		accommodation.EffectiveFrom = time.Now()
	}
	if accommodation.CreatedByID == 0 {
		return errors.New("created_by_id is required")
	}

	return s.accommodationRepo.Create(ctx, accommodation)
}

// GetAccommodation retrieves a single accommodation by ID.
func (s *AccommodationService) GetAccommodation(ctx context.Context, id uint) (*models.StudentAccommodation, error) {
	return s.accommodationRepo.FindByID(ctx, id)
}

// UpdateAccommodation updates an existing accommodation record.
func (s *AccommodationService) UpdateAccommodation(ctx context.Context, accommodation *models.StudentAccommodation) error {
	return s.accommodationRepo.Update(ctx, accommodation)
}

// DeactivateAccommodation soft-deletes an accommodation by setting status to inactive.
func (s *AccommodationService) DeactivateAccommodation(ctx context.Context, id uint) error {
	return s.accommodationRepo.Delete(ctx, id)
}

// ListStudentAccommodations lists accommodations for a user with pagination.
func (s *AccommodationService) ListStudentAccommodations(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.StudentAccommodation], error) {
	return s.accommodationRepo.ListByUserID(ctx, userID, params)
}

// GetActiveAccommodations returns all active, non-expired accommodations for a user.
func (s *AccommodationService) GetActiveAccommodations(ctx context.Context, userID uint) ([]models.StudentAccommodation, error) {
	return s.accommodationRepo.ListActiveByUserID(ctx, userID)
}

// AssignmentAdjustment holds the result of applying accommodations to an assignment.
type AssignmentAdjustment struct {
	AccommodationID uint       `json:"accommodation_id"`
	OriginalDueAt   *time.Time `json:"original_due_at"`
	AdjustedDueAt   *time.Time `json:"adjusted_due_at"`
	ExtraDays       int        `json:"extra_days"`
}

// ApplyAccommodationsToAssignment calculates adjusted due dates based on active accommodations.
// It finds the most generous applicable accommodation (most extra days) and returns the adjustment.
func (s *AccommodationService) ApplyAccommodationsToAssignment(ctx context.Context, userID uint, courseID *uint, originalDueAt *time.Time) (*AssignmentAdjustment, error) {
	if originalDueAt == nil {
		return nil, nil // no due date to adjust
	}

	var accommodations []models.StudentAccommodation
	var err error

	if courseID != nil {
		accommodations, err = s.accommodationRepo.ListByUserAndCourse(ctx, userID, *courseID)
	} else {
		accommodations, err = s.accommodationRepo.ListActiveByUserID(ctx, userID)
	}
	if err != nil {
		return nil, err
	}

	// Find the most generous accommodation with extra_days
	var bestAccommodation *models.StudentAccommodation
	bestExtraDays := 0

	for i := range accommodations {
		a := &accommodations[i]
		if a.ExtraDays != nil && *a.ExtraDays > bestExtraDays {
			bestExtraDays = *a.ExtraDays
			bestAccommodation = a
		}
	}

	if bestAccommodation == nil {
		return nil, nil // no applicable accommodation
	}

	adjustedDue := originalDueAt.AddDate(0, 0, bestExtraDays)

	return &AssignmentAdjustment{
		AccommodationID: bestAccommodation.ID,
		OriginalDueAt:   originalDueAt,
		AdjustedDueAt:   &adjustedDue,
		ExtraDays:       bestExtraDays,
	}, nil
}

// QuizAdjustment holds the result of applying accommodations to a quiz.
type QuizAdjustment struct {
	AccommodationID   uint    `json:"accommodation_id"`
	OriginalTimeLimit int     `json:"original_time_limit"` // minutes
	AdjustedTimeLimit int     `json:"adjusted_time_limit"` // minutes
	TimeMultiplier    float64 `json:"time_multiplier"`
}

// ApplyAccommodationsToQuiz calculates adjusted time limits based on active accommodations.
// It finds the most generous applicable accommodation (highest multiplier) and returns the adjustment.
func (s *AccommodationService) ApplyAccommodationsToQuiz(ctx context.Context, userID uint, courseID *uint, originalTimeLimit *int) (*QuizAdjustment, error) {
	if originalTimeLimit == nil || *originalTimeLimit <= 0 {
		return nil, nil // no time limit to adjust
	}

	var accommodations []models.StudentAccommodation
	var err error

	if courseID != nil {
		accommodations, err = s.accommodationRepo.ListByUserAndCourse(ctx, userID, *courseID)
	} else {
		accommodations, err = s.accommodationRepo.ListActiveByUserID(ctx, userID)
	}
	if err != nil {
		return nil, err
	}

	// Find the most generous accommodation with time_multiplier
	var bestAccommodation *models.StudentAccommodation
	bestMultiplier := 1.0

	for i := range accommodations {
		a := &accommodations[i]
		if a.TimeMultiplier != nil && *a.TimeMultiplier > bestMultiplier {
			bestMultiplier = *a.TimeMultiplier
			bestAccommodation = a
		}
	}

	if bestAccommodation == nil {
		return nil, nil // no applicable accommodation
	}

	adjustedTime := int(math.Ceil(float64(*originalTimeLimit) * bestMultiplier))

	return &QuizAdjustment{
		AccommodationID:   bestAccommodation.ID,
		OriginalTimeLimit: *originalTimeLimit,
		AdjustedTimeLimit: adjustedTime,
		TimeMultiplier:    bestMultiplier,
	}, nil
}

// RecordApplication records that an accommodation was applied to a specific resource.
func (s *AccommodationService) RecordApplication(
	ctx context.Context,
	accommodationID uint,
	resourceType string,
	resourceID uint,
	userID uint,
	originalDueAt *time.Time,
	adjustedDueAt *time.Time,
	originalTimeLimit *int,
	adjustedTimeLimit *int,
) error {
	application := &models.AccommodationApplication{
		AccommodationID:   accommodationID,
		ResourceType:      resourceType,
		ResourceID:        resourceID,
		UserID:            userID,
		OriginalDueAt:     originalDueAt,
		AdjustedDueAt:     adjustedDueAt,
		OriginalTimeLimit: originalTimeLimit,
		AdjustedTimeLimit: adjustedTimeLimit,
		AppliedAt:         time.Now(),
	}
	return s.applicationRepo.Create(ctx, application)
}
