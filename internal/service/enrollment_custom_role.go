package service

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// EnrollmentRoleResolver validates a custom role for an enrollment.
// *CustomRoleService implements it (ResolveEnrollmentRole).
type EnrollmentRoleResolver interface {
	ResolveEnrollmentRole(ctx context.Context, roleID, accountID uint, enrollmentType string) (*models.CustomRole, error)
}

// SetRoleResolver wires custom-role validation into Create (G102).
// Returns the service for chaining.
func (s *EnrollmentService) SetRoleResolver(r EnrollmentRoleResolver) *EnrollmentService {
	s.roleResolver = r
	return s
}

// applyCustomRole sets enrollment.Role from the custom role named by
// enrollment.RoleID, or from the base type when there is none. The role
// must be an active role of the course's tenant whose base role matches
// the enrollment type; anything else is ErrRoleNotAssignable.
//
// Called after validateTarget, so the course is already known to be in
// the caller's tenant.
func (s *EnrollmentService) applyCustomRole(ctx context.Context, enrollment *models.Enrollment) error {
	if enrollment.RoleID == nil || *enrollment.RoleID == 0 {
		enrollment.RoleID = nil
		enrollment.Role = enrollment.Type
		return nil
	}
	if s.roleResolver == nil || s.courseRepo == nil {
		return ErrRoleNotAssignable
	}
	// Unscoped read is safe: validateTarget already tied the course to
	// the caller's tenant; this only reads its account id.
	course, err := s.courseRepo.FindByID(ctx, enrollment.CourseID, 0)
	if err != nil || course == nil {
		return ErrRoleNotAssignable
	}
	role, err := s.roleResolver.ResolveEnrollmentRole(ctx, *enrollment.RoleID, course.AccountID, enrollment.Type)
	if err != nil {
		return ErrRoleNotAssignable
	}
	enrollment.Role = role.Name
	return nil
}
