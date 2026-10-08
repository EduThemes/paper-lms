package service

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/EduThemes/paper-lms/internal/repository"
)

// LTINRPSService implements the LTI Names and Role Provisioning Services
// (NRPS) specification. It provides course membership information in the
// format expected by LTI tools.
type LTINRPSService struct {
	enrollmentRepo repository.EnrollmentRepository
	userRepo       repository.UserRepository
	courseRepo     repository.CourseRepository
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
func (s *LTINRPSService) GetMemberships(ctx context.Context, courseID, accountID uint, params repository.PaginationParams) ([]map[string]interface{}, int64, error) {
	// Fetch enrollments for the course (tenant-scoped)
	enrollments, err := s.enrollmentRepo.ListByCourseID(ctx, courseID, accountID, params)
	if err != nil {
		return nil, 0, errors.New("failed to fetch course enrollments")
	}

	members := make([]map[string]interface{}, 0, len(enrollments.Items))

	for _, enrollment := range enrollments.Items {
		// Fetch the user details for each enrollment, scoped to the caller's
		// tenant (Gap-2: previously accountID=0, i.e. unscoped).
		user, err := s.userRepo.FindByID(ctx, enrollment.UserID, accountID)
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

	return members, enrollments.TotalCount, nil
}

// SetCourseRepo wires the course lookup used for the membership
// container's context label and title (G129).
func (s *LTINRPSService) SetCourseRepo(r repository.CourseRepository) { s.courseRepo = r }

// CourseContext returns the course's label (course code) and title, or
// empty strings when the course cannot be read.
func (s *LTINRPSService) CourseContext(ctx context.Context, courseID, accountID uint) (string, string) {
	if s.courseRepo == nil {
		return "", ""
	}
	course, err := s.courseRepo.FindByID(ctx, courseID, accountID)
	if err != nil || course == nil {
		return "", ""
	}
	return course.CourseCode, course.Name
}

// NRPSFilter narrows an NRPS membership listing (G129).
type NRPSFilter struct {
	// Role is an LIS role URI or its short name ("Learner",
	// "Instructor", ...); members holding it are returned.
	Role string
	// Privacy, when set, limits the PII in each member to what a tool
	// with that privacy level may see. "" returns full members (course
	// staff callers).
	Privacy string
}

func nrpsRoleMatches(roles []string, want string) bool {
	want = strings.TrimSpace(want)
	if want == "" {
		return true
	}
	for _, r := range roles {
		if r == want {
			return true
		}
		if i := strings.LastIndexAny(r, "#/"); i >= 0 && r[i+1:] == want {
			return true
		}
	}
	return false
}

const nrpsMaxScan = 10000

// GetMembershipsFiltered is GetMemberships with the NRPS role filter and
// privacy-level gating of member PII. The role filter needs the whole
// roster, so the course is scanned (capped) and paged in memory.
func (s *LTINRPSService) GetMembershipsFiltered(ctx context.Context, courseID, accountID uint, f NRPSFilter, params repository.PaginationParams) ([]map[string]interface{}, int64, error) {
	if f.Role == "" && f.Privacy == "" {
		return s.GetMemberships(ctx, courseID, accountID, params)
	}
	var all []map[string]interface{}
	for page := 1; len(all) < nrpsMaxScan; page++ {
		batch, total, err := s.GetMemberships(ctx, courseID, accountID, repository.PaginationParams{Page: page, PerPage: 100})
		if err != nil {
			return nil, 0, err
		}
		for _, m := range batch {
			roles, _ := m["roles"].([]string)
			if !nrpsRoleMatches(roles, f.Role) {
				continue
			}
			if f.Privacy != "" {
				if !privacyAllowsName(f.Privacy) {
					delete(m, "name")
					delete(m, "given_name")
					delete(m, "family_name")
				}
				if !privacyAllowsEmail(f.Privacy) {
					delete(m, "email")
				}
				if !privacyAllowsSIS(f.Privacy) {
					delete(m, "lis_person_sourcedid")
				}
			}
			all = append(all, m)
		}
		if int64(page*100) >= total || len(batch) == 0 {
			break
		}
	}
	start := (params.Page - 1) * params.PerPage
	if start < 0 {
		start = 0
	}
	if start > len(all) {
		start = len(all)
	}
	end := start + params.PerPage
	if end > len(all) || params.PerPage <= 0 {
		end = len(all)
	}
	return all[start:end], int64(len(all)), nil
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
