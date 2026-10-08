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
