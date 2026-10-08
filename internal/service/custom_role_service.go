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
	// courseRepo ties course-context overrides to the role's tenant. Wired
	// via WithCourseRepo; without it course-context overrides are refused.
	courseRepo repository.CourseRepository
}

// WithCourseRepo wires the course repository used to validate
// course-context role overrides. Returns the service for chaining.
func (s *CustomRoleService) WithCourseRepo(courseRepo repository.CourseRepository) *CustomRoleService {
	s.courseRepo = courseRepo
	return s
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

// DeleteRole soft-deletes a custom role by setting workflow_state to
// "deleted". accountID scopes the write to a single tenant (F-012);
// handler callers MUST pass callerAccountID(c). Pass 0 only from
// privileged internal callers.
func (s *CustomRoleService) DeleteRole(ctx context.Context, id, accountID uint) error {
	return s.roleRepo.Delete(ctx, id, accountID)
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

// CheckPermission resolves whether a user has a specific permission in a
// course context. accountID is the caller's tenant (0 = unscoped,
// privileged internal callers only).
func (s *CustomRoleService) CheckPermission(ctx context.Context, userID, courseID, accountID uint, permission string) (bool, error) {
	perms, err := s.GetEffectivePermissions(ctx, userID, courseID, accountID)
	if err != nil {
		return false, err
	}
	return perms[permission], nil
}

// ErrNotEnrolled is returned when the user has no active enrollment in the
// course (or the course is outside accountID).
var ErrNotEnrolled = errors.New("user is not enrolled in this course")

// GetEffectivePermissions returns the full permission map for a user in a
// course. accountID scopes the enrollment (and so the course) to a tenant.
func (s *CustomRoleService) GetEffectivePermissions(ctx context.Context, userID, courseID, accountID uint) (map[string]bool, error) {
	enrollment, err := s.enrollmentRepo.FindByUserAndCourse(ctx, userID, courseID, accountID)
	if err != nil || enrollment == nil {
		return nil, ErrNotEnrolled
	}
	return s.EnrollmentPermissions(ctx, enrollment, accountID)
}

// EnrollmentPermissions resolves the permission map an enrollment carries
// (G102). Canvas semantics, adapted to Paper's single-level tenants:
//
//   - No custom role: the base role's defaults.
//   - A custom role: exactly the permissions enabled in the role's
//     permission set (what the Custom Roles editor shows), then the role's
//     account-level overrides, then the role's overrides for this course.
//     A permission marked locked in the role (or by a locked account
//     override) cannot be changed further down the chain.
//
// accountID is the course's tenant; the custom role must belong to it, so
// a role id copied from another tenant never grants anything. A role that
// is missing, deleted or from another tenant falls back to the base role.
func (s *CustomRoleService) EnrollmentPermissions(ctx context.Context, enrollment *models.Enrollment, accountID uint) (map[string]bool, error) {
	baseRole := enrollmentTypeToBaseRole(enrollment.Type)
	if enrollment.RoleID == nil || *enrollment.RoleID == 0 {
		return baseRolePermissions(baseRole), nil
	}
	role, err := s.roleRepo.FindByID(ctx, *enrollment.RoleID, accountID)
	if err != nil || role == nil || role.WorkflowState != "active" {
		return baseRolePermissions(baseRole), nil
	}

	perms := make(map[string]bool, len(models.AllPermissionNames()))
	for _, p := range models.AllPermissionNames() {
		perms[p] = false
	}
	locked := map[string]bool{}
	for name, state := range parseRolePermissions(role.Permissions) {
		if !models.IsValidPermission(name) {
			continue
		}
		perms[name] = state.Enabled
		if state.Locked {
			locked[name] = true
		}
	}

	overrides, err := s.overrideRepo.ListByRoleID(ctx, role.ID)
	if err != nil {
		return nil, err
	}
	// Account-level overrides first, then this course's.
	for _, o := range overrides {
		if o.ContextType != "Account" || !models.IsValidPermission(o.Permission) || locked[o.Permission] {
			continue
		}
		perms[o.Permission] = o.Enabled
		if o.Locked {
			locked[o.Permission] = true
		}
	}
	for _, o := range overrides {
		if o.ContextType != "Course" || o.ContextID != enrollment.CourseID ||
			!models.IsValidPermission(o.Permission) || locked[o.Permission] {
			continue
		}
		perms[o.Permission] = o.Enabled
	}
	return perms, nil
}

// BaseRolePermissions exposes a base role's default permission set (used to
// prefill the Custom Roles editor so a new role starts from its base).
func (s *CustomRoleService) BaseRolePermissions(baseRoleType string) map[string]bool {
	return baseRolePermissions(baseRoleType)
}

// ErrRoleNotAssignable is returned when a custom role can't be given to an
// enrollment: missing, inactive, another tenant's, or based on a different
// role than the enrollment type.
var ErrRoleNotAssignable = errors.New("custom role not found or not valid for this enrollment type")

// ResolveEnrollmentRole validates a custom role for an enrollment of
// enrollmentType in a course of accountID and returns it. The role's base
// role must match the enrollment type (a TA-based role can only be given
// to a TaEnrollment), as in Canvas.
func (s *CustomRoleService) ResolveEnrollmentRole(ctx context.Context, roleID, accountID uint, enrollmentType string) (*models.CustomRole, error) {
	if roleID == 0 || accountID == 0 {
		return nil, ErrRoleNotAssignable
	}
	role, err := s.roleRepo.FindByID(ctx, roleID, accountID)
	if err != nil || role == nil || role.WorkflowState != "active" || role.AccountID != accountID {
		return nil, ErrRoleNotAssignable
	}
	if role.BaseRoleType == "admin" || role.BaseRoleType != enrollmentTypeToBaseRole(enrollmentType) {
		return nil, ErrRoleNotAssignable
	}
	return role, nil
}

// parseRolePermissions decodes a CustomRole.Permissions JSON blob. It
// accepts both {"perm": {"enabled": true, "locked": false}} and the
// shorthand {"perm": true}.
func parseRolePermissions(raw string) map[string]permissionState {
	out := map[string]permissionState{}
	if raw == "" || raw == "{}" {
		return out
	}
	var full map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &full); err != nil {
		return out
	}
	for name, v := range full {
		var st permissionState
		if err := json.Unmarshal(v, &st); err == nil {
			out[name] = st
			continue
		}
		var b bool
		if err := json.Unmarshal(v, &b); err == nil {
			out[name] = permissionState{Enabled: b}
		}
	}
	return out
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

// SetRoleOverride creates or updates a single permission override for a
// role (keyed by role, permission and context).
func (s *CustomRoleService) SetRoleOverride(ctx context.Context, override *models.RoleOverride) error {
	if !models.IsValidPermission(override.Permission) {
		return errors.New("invalid permission name")
	}
	role, err := s.roleRepo.FindByID(ctx, override.RoleID, override.AccountID)
	if err != nil || role == nil {
		return errors.New("role not found")
	}
	override.AccountID = role.AccountID
	if err := s.validateOverrideContext(ctx, role, override); err != nil {
		return err
	}
	return s.overrideRepo.BulkUpsert(ctx, []models.RoleOverride{*override})
}

// validateOverrideContext normalises and checks an override's context:
// "Account" (context_id 0) or a "Course" of the role's own tenant.
func (s *CustomRoleService) validateOverrideContext(ctx context.Context, role *models.CustomRole, o *models.RoleOverride) error {
	switch o.ContextType {
	case "", "Account":
		o.ContextType = "Account"
		o.ContextID = 0
		return nil
	case "Course":
		if o.ContextID == 0 || s.courseRepo == nil {
			return errors.New("invalid course context")
		}
		course, err := s.courseRepo.FindByID(ctx, o.ContextID, role.AccountID)
		if err != nil || course == nil || course.AccountID != role.AccountID {
			return errors.New("invalid course context")
		}
		return nil
	default:
		return errors.New("context_type must be Account or Course")
	}
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

	// Validate all permissions and contexts
	for i := range overrides {
		if !models.IsValidPermission(overrides[i].Permission) {
			return errors.New("invalid permission name: " + overrides[i].Permission)
		}
		overrides[i].RoleID = roleID
		overrides[i].AccountID = role.AccountID
		if err := s.validateOverrideContext(ctx, role, &overrides[i]); err != nil {
			return err
		}
	}

	return s.overrideRepo.BulkUpsert(ctx, overrides)
}

// GetRoleOverrides returns all overrides for a specific role.
func (s *CustomRoleService) GetRoleOverrides(ctx context.Context, roleID uint) ([]models.RoleOverride, error) {
	return s.overrideRepo.ListByRoleID(ctx, roleID)
}
