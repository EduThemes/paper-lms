package service

import (
	"context"
	"errors"
	"strconv"

	"github.com/EduThemes/paper-lms/internal/repository"
)

// LTINRPSService implements the LTI Names and Role Provisioning Services
// (NRPS) specification. It provides course membership information in the
// format expected by LTI tools.
type LTINRPSService struct {
	enrollmentRepo repository.EnrollmentRepository
	userRepo       repository.UserRepository
}

// NewLTINRPSService creates a new LTINRPSService.
func NewLTINRPSService(enrollmentRepo repository.EnrollmentRepository, userRepo repository.UserRepository) *LTINRPSService {
	return &LTINRPSService{
		enrollmentRepo: enrollmentRepo,
		userRepo:       userRepo,
	}
}

// GetMemberships returns all course memberships formatted according to the
// LTI NRPS specification. Each enrollment is transformed into a member object
// with LTI role URIs, user identity fields, and status.
func (s *LTINRPSService) GetMemberships(ctx context.Context, courseID uint, params repository.PaginationParams) ([]map[string]interface{}, error) {
	// Fetch enrollments for the course
	enrollments, err := s.enrollmentRepo.ListByCourseID(ctx, courseID, 0, params)
	if err != nil {
		return nil, errors.New("failed to fetch course enrollments")
	}

	members := make([]map[string]interface{}, 0, len(enrollments.Items))

	for _, enrollment := range enrollments.Items {
		// Fetch the user details for each enrollment
		user, err := s.userRepo.FindByID(ctx, enrollment.UserID)
		if err != nil {
			// Skip enrollments where the user cannot be found
			continue
		}

		// Determine the LTI status from enrollment workflow state
		status := "Active"
		if enrollment.WorkflowState != "active" {
			status = "Inactive"
		}

		// Map the enrollment type to LTI role URIs
		roles := enrollmentTypeToNRPSRoles(enrollment.Type)

		// Build the member object
		member := map[string]interface{}{
			"status":      status,
			"name":        user.Name,
			"given_name":  extractGivenName(user.Name),
			"family_name": extractFamilyName(user.Name),
			"user_id":     strconv.FormatUint(uint64(user.ID), 10),
			"roles":       roles,
		}

		// Include SIS ID if available
		if user.SISUserID != nil && *user.SISUserID != "" {
			member["lis_person_sourcedid"] = *user.SISUserID
		}

		// Include email if available
		if user.Email != "" {
			member["email"] = user.Email
		}

		members = append(members, member)
	}

	return members, nil
}

// enrollmentTypeToNRPSRoles maps a Canvas enrollment type to one or more
// LTI NRPS role URIs.
func enrollmentTypeToNRPSRoles(enrollmentType string) []string {
	switch enrollmentType {
	case "StudentEnrollment":
		return []string{"http://purl.imsglobal.org/vocab/lis/v2/membership#Learner"}
	case "TeacherEnrollment":
		return []string{"http://purl.imsglobal.org/vocab/lis/v2/membership#Instructor"}
	case "TaEnrollment":
		return []string{"http://purl.imsglobal.org/vocab/lis/v2/membership/Instructor#TeachingAssistant"}
	case "ObserverEnrollment":
		return []string{"http://purl.imsglobal.org/vocab/lis/v2/membership#Mentor"}
	case "DesignerEnrollment":
		return []string{"http://purl.imsglobal.org/vocab/lis/v2/membership#ContentDeveloper"}
	default:
		return []string{}
	}
}

// extractGivenName returns the first name from a full name string.
// It splits on the first space; if no space is found, the full name is returned.
func extractGivenName(fullName string) string {
	for i, c := range fullName {
		if c == ' ' {
			return fullName[:i]
		}
	}
	return fullName
}

// extractFamilyName returns the last name from a full name string.
// It splits on the first space; if no space is found, an empty string is returned.
func extractFamilyName(fullName string) string {
	for i, c := range fullName {
		if c == ' ' {
			return fullName[i+1:]
		}
	}
	return ""
}
