package service

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// ListActiveCourseRoles returns the active custom roles of accountID that
// can be given to a course enrollment (every base role but admin).
func (s *CustomRoleService) ListActiveCourseRoles(ctx context.Context, accountID uint) ([]models.CustomRole, error) {
	roles, err := s.roleRepo.ListActive(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]models.CustomRole, 0, len(roles))
	for _, r := range roles {
		if r.BaseRoleType != "admin" && r.AccountID == accountID {
			out = append(out, r)
		}
	}
	return out, nil
}

// CoursePermissions resolves the caller's permission map in a course and
// the custom role (if any) their enrollment carries.
func (s *CustomRoleService) CoursePermissions(ctx context.Context, userID, courseID, accountID uint) (map[string]bool, *uint, error) {
	enrollment, err := s.enrollmentRepo.FindByUserAndCourse(ctx, userID, courseID, accountID)
	if err != nil || enrollment == nil {
		return nil, nil, ErrNotEnrolled
	}
	perms, err := s.EnrollmentPermissions(ctx, enrollment, accountID)
	if err != nil {
		return nil, nil, err
	}
	return perms, enrollment.RoleID, nil
}

// BaseRoleEnrollmentType maps a custom role's base role onto the
// enrollment type it is given with.
func BaseRoleEnrollmentType(baseRoleType string) string {
	switch baseRoleType {
	case "teacher":
		return "TeacherEnrollment"
	case "ta":
		return "TaEnrollment"
	case "observer":
		return "ObserverEnrollment"
	case "student":
		return "StudentEnrollment"
	}
	return ""
}
