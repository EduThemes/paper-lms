package service

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type LatePolicyService struct {
	latePolicyRepo repository.LatePolicyRepository
}

func NewLatePolicyService(latePolicyRepo repository.LatePolicyRepository) *LatePolicyService {
	return &LatePolicyService{latePolicyRepo: latePolicyRepo}
}

func (s *LatePolicyService) Create(ctx context.Context, policy *models.LatePolicy) error {
	if policy.CourseID == 0 {
		return errors.New("course_id is required")
	}
	if policy.LateSubmissionInterval == "" {
		policy.LateSubmissionInterval = "day"
	}
	if policy.LateSubmissionInterval != "day" && policy.LateSubmissionInterval != "hour" {
		return errors.New("late_submission_interval must be 'day' or 'hour'")
	}
	return s.latePolicyRepo.Create(ctx, policy)
}

func (s *LatePolicyService) GetByCourse(ctx context.Context, courseID uint) (*models.LatePolicy, error) {
	return s.latePolicyRepo.FindByCourseID(ctx, courseID)
}

func (s *LatePolicyService) Update(ctx context.Context, policy *models.LatePolicy) error {
	if policy.LateSubmissionInterval != "" && policy.LateSubmissionInterval != "day" && policy.LateSubmissionInterval != "hour" {
		return errors.New("late_submission_interval must be 'day' or 'hour'")
	}
	return s.latePolicyRepo.Update(ctx, policy)
}

func (s *LatePolicyService) Delete(ctx context.Context, courseID uint) error {
	return s.latePolicyRepo.Delete(ctx, courseID)
}
