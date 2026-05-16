package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// permissionState represents the enabled/locked state of a single permission in the JSON blob.
type permissionState struct {
	Enabled bool `json:"enabled"`
	Locked  bool `json:"locked"`
}

// CustomRoleService provides business logic for custom roles and granular permissions.
type CustomRoleService struct {
	roleRepo       repository.CustomRoleRepository
	overrideRepo   repository.RoleOverrideRepository
	enrollmentRepo repository.EnrollmentRepository
}

// NewCustomRoleService creates a new CustomRoleService.
func NewCustomRoleService(
	roleRepo repository.CustomRoleRepository,
	overrideRepo repository.RoleOverrideRepository,
	enrollmentRepo repository.EnrollmentRepository,
) *CustomRoleService {
	return &CustomRoleService{
		roleRepo:       roleRepo,
		overrideRepo:   overrideRepo,
		enrollmentRepo: enrollmentRepo,
	}
}

// CreateRole creates a new custom role after validation.
func (s *CustomRoleService) CreateRole(ctx context.Context, role *models.CustomRole) error {
	if role.Name == "" {
		return errors.New("role name is required")
	}
	if !models.IsValidBaseRoleType(role.BaseRoleType) {
		return errors.New("invalid base role type")
	}

	// Check for duplicate name within account
	existing, _ := s.roleRepo.FindByAccountAndName(ctx, role.AccountID, role.Name)
	if existing != nil {
		return errors.New("a role with this name already exists in this account")
	}

	if role.WorkflowState == "" {
		role.WorkflowState = "active"
	}
	if role.Permissions == "" {
		role.Permissions = "{}"
	}
	if role.Label == "" {
		role.Label = role.Name
	}

	return s.roleRepo.Create(ctx, role)
}

// UpdateRole updates an existing custom role.
func (s *CustomRoleService) UpdateRole(ctx context.Context, role *models.CustomRole) error {
	if role.Name == "" {
		return errors.New("role name is required")
	}
	if !models.IsValidBaseRoleType(role.BaseRoleType) {
		return errors.New("invalid base role type")
	}
	return s.roleRepo.Update(ctx, role)
}

// DeleteRole soft-deletes a custom role by setting workflow_state to "deleted".
func (s *CustomRoleService) DeleteRole(ctx context.Context, id uint) error {
	return s.roleRepo.Delete(ctx, id)
}

// ListRoles returns paginated roles for an account.
func (s *CustomRoleService) ListRoles(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.CustomRole], error) {
	return s.roleRepo.ListByAccountID(ctx, accountID, params)
}

// GetRole retrieves a single role by ID. accountID scopes the read to a
// single tenant (13.1.D); pass 0 for unscoped reads from privileged
// internal callers.
func (s *CustomRoleService) GetRole(ctx context.Context, id, accountID uint) (*models.CustomRole, error) {
	return s.roleRepo.FindByID(ctx, id, accountID)
}

// CloneRole duplicates an existing role with a new name.
func (s *CustomRoleService) CloneRole(ctx context.Context, sourceID, accountID uint, newName string, createdByUserID uint) (*models.CustomRole, error) {
	source, err := s.roleRepo.FindByID(ctx, sourceID, accountID)
	if err != nil {
		return nil, errors.New("source role not found")
	}

	if newName == "" {
		newName = source.Name + " (Copy)"
	}

	// Check for duplicate name
	existing, _ := s.roleRepo.FindByAccountAndName(ctx, source.AccountID, newName)
	if existing != nil {
		return nil, errors.New("a role with this name already exists in this account")
	}

	clone := &models.CustomRole{
		AccountID:       source.AccountID,
		Name:            newName,
		BaseRoleType:    source.BaseRoleType,
		Label:           newName,
		WorkflowState:   "active",
		Permissions:     source.Permissions,
		CreatedByUserID: createdByUserID,
	}

	if err := s.roleRepo.Create(ctx, clone); err != nil {
		return nil, err
	}

	// Clone role overrides from source
	overrides, err := s.overrideRepo.ListByRoleID(ctx, sourceID)
	if err == nil && len(overrides) > 0 {
		clonedOverrides := make([]models.RoleOverride, len(overrides))
		for i, o := range overrides {
			clonedOverrides[i] = models.RoleOverride{
				AccountID:   o.AccountID,
				RoleID:      clone.ID,
				Permission:  o.Permission,
				Enabled:     o.Enabled,
				Locked:      o.Locked,
				ContextType: o.ContextType,
				ContextID:   o.ContextID,
			}
		}
		_ = s.overrideRepo.BulkUpsert(ctx, clonedOverrides)
	}

	return clone, nil
}

// GetPermissionPresets returns built-in permission preset templates.
func (s *CustomRoleService) GetPermissionPresets() []models.PermissionPreset {
	return []models.PermissionPreset{
		{
			Name:        "full_teacher",
			Label:       "Full Teacher",
			Description: "All course management and submission permissions - equivalent to the default teacher role",
			Permissions: []string{
				models.PermManageContent, models.PermManageAssignments, models.PermManageGrades,
				models.PermViewAllGrades, models.PermManageSections, models.PermManageEnrollments,
				models.PermManageCalendar, models.PermManageAnnouncements, models.PermManageDiscussions,
				models.PermManageFiles, models.PermManagePages, models.PermManageModules,
				models.PermManageQuizzes, models.PermManageRubrics, models.PermManageOutcomes,
				models.PermManageGroups, models.PermManageConferences, models.PermManageCollaborations,
				models.PermSendMessages, models.PermViewAnalytics, models.PermViewUserEmail,
				models.PermManageUserNotes, models.PermReadRoster,
				models.PermGradeSubmissions, models.PermCommentOnSubmissions,
				models.PermViewSubmissionDetails, models.PermModerateGrades,
			},
		},
		{
			Name:        "grading_assistant",
			Label:       "Grading Assistant",
			Description: "Focused on grading and viewing submissions - ideal for TAs who only grade",
			Permissions: []string{
				models.PermGradeSubmissions, models.PermCommentOnSubmissions,
				models.PermViewAllGrades, models.PermViewSubmissionDetails,
				models.PermReadRoster,
			},
		},
		{
			Name:        "content_manager",
			Label:       "Content Manager",
			Description: "Manages course content, pages, files, and modules - no grading access",
			Permissions: []string{
				models.PermManageContent, models.PermManagePages,
				models.PermManageFiles, models.PermManageModules,
			},
		},
		{
			Name:        "department_chair",
			Label:       "Department Chair",
			Description: "Full teacher permissions plus analytics, enrollment management, and section oversight",
			Permissions: []string{
				// All teacher permissions
				models.PermManageContent, models.PermManageAssignments, models.PermManageGrades,
				models.PermViewAllGrades, models.PermManageSections, models.PermManageEnrollments,
				models.PermManageCalendar, models.PermManageAnnouncements, models.PermManageDiscussions,
				models.PermManageFiles, models.PermManagePages, models.PermManageModules,
				models.PermManageQuizzes, models.PermManageRubrics, models.PermManageOutcomes,
				models.PermManageGroups, models.PermManageConferences, models.PermManageCollaborations,
				models.PermSendMessages, models.PermViewUserEmail,
				models.PermManageUserNotes, models.PermReadRoster,
				models.PermGradeSubmissions, models.PermCommentOnSubmissions,
				models.PermViewSubmissionDetails, models.PermModerateGrades,
				// Additional chair permissions
				models.PermViewAnalytics, models.PermManageEnrollments, models.PermManageSections,
			},
		},
	}
}

// baseRolePermissions returns the default permissions for a given base role type.
func baseRolePermissions(baseRoleType string) map[string]bool {
	perms := make(map[string]bool)

	switch baseRoleType {
	case "admin":
		// Admin gets all permissions
		for _, p := range models.AllPermissionNames() {
			perms[p] = true
		}
	case "teacher":
		// Teachers get all course, user, and submission permissions
		coursePerms := []string{
			models.PermManageContent, models.PermManageAssignments, models.PermManageGrades,
			models.PermViewAllGrades, models.PermManageSections, models.PermManageEnrollments,
			models.PermManageCalendar, models.PermManageAnnouncements, models.PermManageDiscussions,
			models.PermManageFiles, models.PermManagePages, models.PermManageModules,
			models.PermManageQuizzes, models.PermManageRubrics, models.PermManageOutcomes,
			models.PermManageGroups, models.PermManageConferences, models.PermManageCollaborations,
		}
		userPerms := []string{
			models.PermSendMessages, models.PermViewAnalytics, models.PermViewUserEmail,
			models.PermManageUserNotes, models.PermReadRoster,
		}
		subPerms := []string{
			models.PermGradeSubmissions, models.PermCommentOnSubmissions,
			models.PermViewSubmissionDetails, models.PermModerateGrades,
		}
		for _, p := range coursePerms {
			perms[p] = true
		}
		for _, p := range userPerms {
			perms[p] = true
		}
		for _, p := range subPerms {
			perms[p] = true
		}
	case "ta":
		// TAs get a subset of course and submission permissions
		taPerms := []string{
			models.PermManageDiscussions, models.PermManageFiles,
			models.PermViewAllGrades, models.PermReadRoster,
			models.PermSendMessages, models.PermViewUserEmail,
			models.PermGradeSubmissions, models.PermCommentOnSubmissions,
			models.PermViewSubmissionDetails,
		}
		for _, p := range taPerms {
			perms[p] = true
		}
	case "student":
		// Students get minimal permissions
		studentPerms := []string{
			models.PermSendMessages, models.PermReadRoster,
		}
		for _, p := range studentPerms {
			perms[p] = true
		}
	case "observer":
		// Observers can view roster and read
		observerPerms := []string{
			models.PermReadRoster,
		}
		for _, p := range observerPerms {
			perms[p] = true
		}
	}

	return perms
}

// CheckPermission resolves whether a user has a specific permission in a course context.
// It merges the base role permissions with any custom role overrides.
func (s *CustomRoleService) CheckPermission(ctx context.Context, userID, courseID uint, permission string) (bool, error) {
	perms, err := s.GetEffectivePermissions(ctx, userID, courseID)
	if err != nil {
		return false, err
	}
	return perms[permission], nil
}

// GetEffectivePermissions returns the full permission map for a user in a course context.
// It starts with the base role defaults and then applies any custom role overrides.
func (s *CustomRoleService) GetEffectivePermissions(ctx context.Context, userID, courseID uint) (map[string]bool, error) {
	enrollment, err := s.enrollmentRepo.FindByUserAndCourse(ctx, userID, courseID)
	if err != nil {
		return nil, errors.New("user is not enrolled in this course")
	}

	// Map enrollment type to base role type
	baseRole := enrollmentTypeToBaseRole(enrollment.Type)
	perms := baseRolePermissions(baseRole)

	// If the enrollment has a custom role reference, look up overrides
	if enrollment.Role != "" && enrollment.Role != enrollment.Type {
		// Try to find a custom role matching the enrollment role name
		// Search across all active roles for the account
		roles, err := s.roleRepo.ListActive(ctx, 1) // default account
		if err == nil {
			for _, role := range roles {
				if role.Name == enrollment.Role || role.Label == enrollment.Role {
					// Apply permissions from the custom role JSON
					s.applyRolePermissions(perms, &role)

					// Apply specific overrides
					overrides, oErr := s.overrideRepo.ListByRoleID(ctx, role.ID)
					if oErr == nil {
						for _, o := range overrides {
							if !o.Locked {
								perms[o.Permission] = o.Enabled
							}
						}
					}
					break
				}
			}
		}
	}

	return perms, nil
}

// applyRolePermissions merges the permissions JSON blob from a CustomRole into the permission map.
func (s *CustomRoleService) applyRolePermissions(perms map[string]bool, role *models.CustomRole) {
	if role.Permissions == "" || role.Permissions == "{}" {
		return
	}

	var permMap map[string]permissionState
	if err := json.Unmarshal([]byte(role.Permissions), &permMap); err != nil {
		return
	}

	for name, state := range permMap {
		if models.IsValidPermission(name) {
			perms[name] = state.Enabled
		}
	}
}

// enrollmentTypeToBaseRole maps Canvas enrollment types to base role types.
func enrollmentTypeToBaseRole(enrollmentType string) string {
	switch enrollmentType {
	case "TeacherEnrollment":
		return "teacher"
	case "TaEnrollment":
		return "ta"
	case "StudentEnrollment":
		return "student"
	case "ObserverEnrollment":
		return "observer"
	case "DesignerEnrollment":
		return "teacher" // designers get teacher-level by default
	default:
		return "student"
	}
}

// SetRoleOverride creates or updates a single permission override for a role.
func (s *CustomRoleService) SetRoleOverride(ctx context.Context, override *models.RoleOverride) error {
	if !models.IsValidPermission(override.Permission) {
		return errors.New("invalid permission name")
	}

	// Check that the role exists
	_, err := s.roleRepo.FindByID(ctx, override.RoleID, 0)
	if err != nil {
		return errors.New("role not found")
	}

	if override.ContextType == "" {
		override.ContextType = "Account"
	}

	existing, err := s.overrideRepo.FindByRoleAndPermission(ctx, override.RoleID, override.Permission)
	if err == nil && existing != nil {
		existing.Enabled = override.Enabled
		existing.Locked = override.Locked
		return s.overrideRepo.Update(ctx, existing)
	}

	return s.overrideRepo.Create(ctx, override)
}

// RemoveRoleOverride deletes a specific override.
func (s *CustomRoleService) RemoveRoleOverride(ctx context.Context, overrideID uint) error {
	return s.overrideRepo.Delete(ctx, overrideID)
}

// BulkSetOverrides creates or updates multiple permission overrides for a role at once.
func (s *CustomRoleService) BulkSetOverrides(ctx context.Context, roleID, accountID uint, overrides []models.RoleOverride) error {
	// Validate the role exists
	role, err := s.roleRepo.FindByID(ctx, roleID, accountID)
	if err != nil {
		return errors.New("role not found")
	}

	// Validate all permissions
	for i := range overrides {
		if !models.IsValidPermission(overrides[i].Permission) {
			return errors.New("invalid permission name: " + overrides[i].Permission)
		}
		overrides[i].RoleID = roleID
		overrides[i].AccountID = role.AccountID
		if overrides[i].ContextType == "" {
			overrides[i].ContextType = "Account"
		}
	}

	return s.overrideRepo.BulkUpsert(ctx, overrides)
}

// GetRoleOverrides returns all overrides for a specific role.
func (s *CustomRoleService) GetRoleOverrides(ctx context.Context, roleID uint) ([]models.RoleOverride, error) {
	return s.overrideRepo.ListByRoleID(ctx, roleID)
}
