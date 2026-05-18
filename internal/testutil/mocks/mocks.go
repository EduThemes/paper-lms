package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// FnSubmissionRepository implements repository.SubmissionRepository using function fields for testing.
type FnSubmissionRepository struct {
	CreateFn                  func(ctx context.Context, submission *models.Submission) error
	FindByIDFn                func(ctx context.Context, id uint) (*models.Submission, error)
	FindByAssignmentAndUserFn func(ctx context.Context, assignmentID, userID uint) (*models.Submission, error)
	UpdateFn                  func(ctx context.Context, submission *models.Submission) error
	ListByAssignmentIDFn      func(ctx context.Context, assignmentID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Submission], error)
	ListByUserAndCourseFn     func(ctx context.Context, userID, courseID uint) ([]models.Submission, error)
	BulkListByCourseFn        func(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Submission], error)
}

func (m *FnSubmissionRepository) Create(ctx context.Context, submission *models.Submission) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, submission)
	}
	return nil
}

func (m *FnSubmissionRepository) FindByID(ctx context.Context, id uint) (*models.Submission, error) {
	if m.FindByIDFn != nil {
		return m.FindByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *FnSubmissionRepository) FindByAssignmentAndUser(ctx context.Context, assignmentID, userID uint) (*models.Submission, error) {
	if m.FindByAssignmentAndUserFn != nil {
		return m.FindByAssignmentAndUserFn(ctx, assignmentID, userID)
	}
	return nil, nil
}

func (m *FnSubmissionRepository) Update(ctx context.Context, submission *models.Submission) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, submission)
	}
	return nil
}

func (m *FnSubmissionRepository) ListByAssignmentID(ctx context.Context, assignmentID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Submission], error) {
	if m.ListByAssignmentIDFn != nil {
		return m.ListByAssignmentIDFn(ctx, assignmentID, params)
	}
	return &repository.PaginatedResult[models.Submission]{}, nil
}

func (m *FnSubmissionRepository) ListByUserAndCourse(ctx context.Context, userID, courseID uint) ([]models.Submission, error) {
	if m.ListByUserAndCourseFn != nil {
		return m.ListByUserAndCourseFn(ctx, userID, courseID)
	}
	return nil, nil
}

func (m *FnSubmissionRepository) BulkListByCourse(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Submission], error) {
	if m.BulkListByCourseFn != nil {
		return m.BulkListByCourseFn(ctx, courseID, params)
	}
	return &repository.PaginatedResult[models.Submission]{}, nil
}

// FnAssignmentRepository implements repository.AssignmentRepository using function fields for testing.
type FnAssignmentRepository struct {
	CreateFn         func(ctx context.Context, assignment *models.Assignment) error
	FindByIDFn       func(ctx context.Context, id uint) (*models.Assignment, error)
	UpdateFn         func(ctx context.Context, assignment *models.Assignment) error
	DeleteFn         func(ctx context.Context, id uint) error
	ListByCourseIDFn func(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Assignment], error)
}

func (m *FnAssignmentRepository) Create(ctx context.Context, assignment *models.Assignment) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, assignment)
	}
	return nil
}

func (m *FnAssignmentRepository) FindByID(ctx context.Context, id uint) (*models.Assignment, error) {
	if m.FindByIDFn != nil {
		return m.FindByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *FnAssignmentRepository) Update(ctx context.Context, assignment *models.Assignment) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, assignment)
	}
	return nil
}

func (m *FnAssignmentRepository) Delete(ctx context.Context, id uint) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, id)
	}
	return nil
}

func (m *FnAssignmentRepository) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Assignment], error) {
	if m.ListByCourseIDFn != nil {
		return m.ListByCourseIDFn(ctx, courseID, params)
	}
	return &repository.PaginatedResult[models.Assignment]{}, nil
}

// FnAssignmentGroupRepository implements repository.AssignmentGroupRepository using function fields for testing.
type FnAssignmentGroupRepository struct {
	CreateFn         func(ctx context.Context, group *models.AssignmentGroup) error
	FindByIDFn       func(ctx context.Context, id, accountID uint) (*models.AssignmentGroup, error)
	UpdateFn         func(ctx context.Context, group *models.AssignmentGroup) error
	DeleteFn         func(ctx context.Context, id uint) error
	ListByCourseIDFn func(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AssignmentGroup], error)
}

func (m *FnAssignmentGroupRepository) Create(ctx context.Context, group *models.AssignmentGroup) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, group)
	}
	return nil
}

func (m *FnAssignmentGroupRepository) FindByID(ctx context.Context, id, accountID uint) (*models.AssignmentGroup, error) {
	if m.FindByIDFn != nil {
		return m.FindByIDFn(ctx, id, accountID)
	}
	return nil, nil
}

func (m *FnAssignmentGroupRepository) Update(ctx context.Context, group *models.AssignmentGroup) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, group)
	}
	return nil
}

func (m *FnAssignmentGroupRepository) Delete(ctx context.Context, id uint) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, id)
	}
	return nil
}

func (m *FnAssignmentGroupRepository) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AssignmentGroup], error) {
	if m.ListByCourseIDFn != nil {
		return m.ListByCourseIDFn(ctx, courseID, params)
	}
	return &repository.PaginatedResult[models.AssignmentGroup]{}, nil
}

// FnEnrollmentRepository implements repository.EnrollmentRepository using function fields for testing.
type FnEnrollmentRepository struct {
	CreateFn                               func(ctx context.Context, enrollment *models.Enrollment) error
	FindByIDFn                             func(ctx context.Context, id, accountID uint) (*models.Enrollment, error)
	UpdateFn                               func(ctx context.Context, enrollment *models.Enrollment) error
	ListByCourseIDFn                       func(ctx context.Context, courseID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Enrollment], error)
	ListByUserIDFn                         func(ctx context.Context, userID, accountID uint) ([]models.Enrollment, error)
	FindByUserAndCourseFn                  func(ctx context.Context, userID, courseID, accountID uint) (*models.Enrollment, error)
	CountByCourseIDsFn                     func(ctx context.Context, courseIDs []uint, accountID uint) (map[uint]int64, error)
	ListActiveStudentUserIDsByCourseFn     func(ctx context.Context, courseID, accountID uint) ([]uint, error)
	ListActiveStudentEnrollmentsByCourseFn func(ctx context.Context, courseID, accountID uint) ([]models.Enrollment, error)
	UpdatePseudonymForSelfFn               func(ctx context.Context, userID, courseID, accountID uint, poolCode, name string) error
}

func (m *FnEnrollmentRepository) Create(ctx context.Context, enrollment *models.Enrollment) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, enrollment)
	}
	return nil
}

func (m *FnEnrollmentRepository) FindByID(ctx context.Context, id, accountID uint) (*models.Enrollment, error) {
	if m.FindByIDFn != nil {
		return m.FindByIDFn(ctx, id, accountID)
	}
	return nil, nil
}

func (m *FnEnrollmentRepository) Update(ctx context.Context, enrollment *models.Enrollment) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, enrollment)
	}
	return nil
}

func (m *FnEnrollmentRepository) ListByCourseID(ctx context.Context, courseID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Enrollment], error) {
	if m.ListByCourseIDFn != nil {
		return m.ListByCourseIDFn(ctx, courseID, accountID, params)
	}
	return &repository.PaginatedResult[models.Enrollment]{}, nil
}

func (m *FnEnrollmentRepository) ListByUserID(ctx context.Context, userID, accountID uint) ([]models.Enrollment, error) {
	if m.ListByUserIDFn != nil {
		return m.ListByUserIDFn(ctx, userID, accountID)
	}
	return nil, nil
}

func (m *FnEnrollmentRepository) FindByUserAndCourse(ctx context.Context, userID, courseID, accountID uint) (*models.Enrollment, error) {
	if m.FindByUserAndCourseFn != nil {
		return m.FindByUserAndCourseFn(ctx, userID, courseID, accountID)
	}
	return nil, nil
}

func (m *FnEnrollmentRepository) CountByCourseIDs(ctx context.Context, courseIDs []uint, accountID uint) (map[uint]int64, error) {
	if m.CountByCourseIDsFn != nil {
		return m.CountByCourseIDsFn(ctx, courseIDs, accountID)
	}
	return nil, nil
}

func (m *FnEnrollmentRepository) ListActiveStudentUserIDsByCourse(ctx context.Context, courseID, accountID uint) ([]uint, error) {
	if m.ListActiveStudentUserIDsByCourseFn != nil {
		return m.ListActiveStudentUserIDsByCourseFn(ctx, courseID, accountID)
	}
	return nil, nil
}

func (m *FnEnrollmentRepository) ListActiveStudentEnrollmentsByCourse(ctx context.Context, courseID, accountID uint) ([]models.Enrollment, error) {
	if m.ListActiveStudentEnrollmentsByCourseFn != nil {
		return m.ListActiveStudentEnrollmentsByCourseFn(ctx, courseID, accountID)
	}
	return nil, nil
}

func (m *FnEnrollmentRepository) UpdatePseudonymForSelf(ctx context.Context, userID, courseID, accountID uint, poolCode, name string) error {
	if m.UpdatePseudonymForSelfFn != nil {
		return m.UpdatePseudonymForSelfFn(ctx, userID, courseID, accountID, poolCode, name)
	}
	return nil
}
