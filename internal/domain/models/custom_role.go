package models

import "time"

// CustomRole defines a custom role with granular permissions that extends the base Canvas role types.
// Permissions are stored as a JSON object: {"permission_name": {"enabled": true, "locked": false}}
type CustomRole struct {
	ID              uint      `json:"id" gorm:"column:id;primaryKey"`
	AccountID       uint      `json:"account_id" gorm:"not null;index"`
	Name            string    `json:"name" gorm:"not null"`
	BaseRoleType    string    `json:"base_role_type" gorm:"not null;index"` // teacher, ta, student, observer, admin
	Label           string    `json:"label"`
	WorkflowState   string    `json:"workflow_state" gorm:"not null;default:'active'"` // active, inactive, deleted
	Permissions     string    `json:"permissions" gorm:"type:jsonb;default:'{}'"`      // {"permission_name": {"enabled": true, "locked": false}}
	CreatedByUserID uint      `json:"created_by_user_id" gorm:"index"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Permission category constants for organizing the UI.
const (
	PermCategoryCourse     = "Course Management"
	PermCategoryUser       = "User Management"
	PermCategoryAdmin      = "Administration"
	PermCategorySubmission = "Grading & Submissions"
)

// Course Management permissions
const (
	PermManageContent        = "manage_content"
	PermManageAssignments    = "manage_assignments"
	PermManageGrades         = "manage_grades"
	PermViewAllGrades        = "view_all_grades"
	PermManageSections       = "manage_sections"
	PermManageEnrollments    = "manage_enrollments"
	PermManageCalendar       = "manage_calendar"
	PermManageAnnouncements  = "manage_announcements"
	PermManageDiscussions    = "manage_discussions"
	PermManageFiles          = "manage_files"
	PermManagePages          = "manage_pages"
	PermManageModules        = "manage_modules"
	PermManageQuizzes        = "manage_quizzes"
	PermManageRubrics        = "manage_rubrics"
	PermManageOutcomes       = "manage_outcomes"
	PermManageGroups         = "manage_groups"
	PermManageConferences    = "manage_conferences"
	PermManageCollaborations = "manage_collaborations"
)

// User Management permissions
const (
	PermSendMessages    = "send_messages"
	PermViewAnalytics   = "view_analytics"
	PermViewUserEmail   = "view_user_email"
	PermManageUserNotes = "manage_user_notes"
	PermReadRoster      = "read_roster"
)

// Administration permissions
const (
	PermManageCourses         = "manage_courses"
	PermManageAccountSettings = "manage_account_settings"
	PermManageDeveloperKeys   = "manage_developer_keys"
	PermManageSIS             = "manage_sis"
	PermManageAuthProviders   = "manage_auth_providers"
	PermManageUsers           = "manage_users"
	PermViewAuditLog          = "view_audit_log"
	PermManageEnrollmentTerms = "manage_enrollment_terms"
	PermManageBlueprint       = "manage_blueprint"
	PermManagePacing          = "manage_pacing"
)

// Grading & Submission permissions
const (
	PermGradeSubmissions      = "grade_submissions"
	PermCommentOnSubmissions  = "comment_on_submissions"
	PermViewSubmissionDetails = "view_submission_details"
	PermModerateGrades        = "moderate_grades"
)

// PermissionDefinition describes a single permission with its metadata.
type PermissionDefinition struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// AllPermissions returns all available permission definitions organized by category.
func AllPermissions() []PermissionDefinition {
	return []PermissionDefinition{
		// Course Management
		{Name: PermManageContent, Label: "Manage Content", Description: "Create, edit, and delete course content items", Category: PermCategoryCourse},
		{Name: PermManageAssignments, Label: "Manage Assignments", Description: "Create, edit, and delete assignments", Category: PermCategoryCourse},
		{Name: PermManageGrades, Label: "Manage Grades", Description: "Edit and manage the gradebook", Category: PermCategoryCourse},
		{Name: PermViewAllGrades, Label: "View All Grades", Description: "View grades for all students in the course", Category: PermCategoryCourse},
		{Name: PermManageSections, Label: "Manage Sections", Description: "Create, edit, and delete course sections", Category: PermCategoryCourse},
		{Name: PermManageEnrollments, Label: "Manage Enrollments", Description: "Add, remove, and modify student enrollments", Category: PermCategoryCourse},
		{Name: PermManageCalendar, Label: "Manage Calendar", Description: "Create and edit calendar events for the course", Category: PermCategoryCourse},
		{Name: PermManageAnnouncements, Label: "Manage Announcements", Description: "Create, edit, and delete course announcements", Category: PermCategoryCourse},
		{Name: PermManageDiscussions, Label: "Manage Discussions", Description: "Create, edit, and moderate discussion topics", Category: PermCategoryCourse},
		{Name: PermManageFiles, Label: "Manage Files", Description: "Upload, organize, and delete course files", Category: PermCategoryCourse},
		{Name: PermManagePages, Label: "Manage Pages", Description: "Create, edit, and delete wiki pages", Category: PermCategoryCourse},
		{Name: PermManageModules, Label: "Manage Modules", Description: "Create, edit, and organize course modules", Category: PermCategoryCourse},
		{Name: PermManageQuizzes, Label: "Manage Quizzes", Description: "Create, edit, and publish quizzes", Category: PermCategoryCourse},
		{Name: PermManageRubrics, Label: "Manage Rubrics", Description: "Create, edit, and associate rubrics", Category: PermCategoryCourse},
		{Name: PermManageOutcomes, Label: "Manage Outcomes", Description: "Create and manage learning outcomes", Category: PermCategoryCourse},
		{Name: PermManageGroups, Label: "Manage Groups", Description: "Create and manage student groups", Category: PermCategoryCourse},
		{Name: PermManageConferences, Label: "Manage Conferences", Description: "Create and manage web conferences", Category: PermCategoryCourse},
		{Name: PermManageCollaborations, Label: "Manage Collaborations", Description: "Create and manage collaborative documents", Category: PermCategoryCourse},
		// User Management
		{Name: PermSendMessages, Label: "Send Messages", Description: "Send messages to course participants via inbox", Category: PermCategoryUser},
		{Name: PermViewAnalytics, Label: "View Analytics", Description: "Access course and student analytics dashboards", Category: PermCategoryUser},
		{Name: PermViewUserEmail, Label: "View User Email", Description: "View email addresses of enrolled users", Category: PermCategoryUser},
		{Name: PermManageUserNotes, Label: "Manage User Notes", Description: "Create and view faculty journal notes about students", Category: PermCategoryUser},
		{Name: PermReadRoster, Label: "Read Roster", Description: "View the list of enrolled students in the course", Category: PermCategoryUser},
		// Administration
		{Name: PermManageCourses, Label: "Manage Courses", Description: "Create, edit, and delete courses at the account level", Category: PermCategoryAdmin},
		{Name: PermManageAccountSettings, Label: "Manage Account Settings", Description: "Modify account-level settings and configurations", Category: PermCategoryAdmin},
		{Name: PermManageDeveloperKeys, Label: "Manage Developer Keys", Description: "Create and manage OAuth2 developer keys and API tokens", Category: PermCategoryAdmin},
		{Name: PermManageSIS, Label: "Manage SIS", Description: "Import and export SIS data for the account", Category: PermCategoryAdmin},
		{Name: PermManageAuthProviders, Label: "Manage Auth Providers", Description: "Configure SSO and authentication providers (SAML, LDAP, CAS)", Category: PermCategoryAdmin},
		{Name: PermManageUsers, Label: "Manage Users", Description: "Create, edit, and deactivate user accounts", Category: PermCategoryAdmin},
		{Name: PermViewAuditLog, Label: "View Audit Log", Description: "Access the system audit log for compliance tracking", Category: PermCategoryAdmin},
		{Name: PermManageEnrollmentTerms, Label: "Manage Enrollment Terms", Description: "Create and edit academic terms and enrollment periods", Category: PermCategoryAdmin},
		{Name: PermManageBlueprint, Label: "Manage Blueprint Courses", Description: "Create and sync blueprint course templates", Category: PermCategoryAdmin},
		{Name: PermManagePacing, Label: "Manage Course Pacing", Description: "Configure and manage course pacing plans", Category: PermCategoryAdmin},
		// Grading & Submissions
		{Name: PermGradeSubmissions, Label: "Grade Submissions", Description: "Assign grades and scores to student submissions", Category: PermCategorySubmission},
		{Name: PermCommentOnSubmissions, Label: "Comment on Submissions", Description: "Add feedback comments to student submissions", Category: PermCategorySubmission},
		{Name: PermViewSubmissionDetails, Label: "View Submission Details", Description: "View full submission content and metadata", Category: PermCategorySubmission},
		{Name: PermModerateGrades, Label: "Moderate Grades", Description: "Review and approve grades from multiple graders", Category: PermCategorySubmission},
	}
}

// AllPermissionNames returns a flat list of all permission name strings.
func AllPermissionNames() []string {
	perms := AllPermissions()
	names := make([]string, len(perms))
	for i, p := range perms {
		names[i] = p.Name
	}
	return names
}

// ValidBaseRoleTypes returns the allowed base role type values.
func ValidBaseRoleTypes() []string {
	return []string{"teacher", "ta", "student", "observer", "admin"}
}

// IsValidBaseRoleType checks whether the given string is a valid base role type.
func IsValidBaseRoleType(roleType string) bool {
	for _, v := range ValidBaseRoleTypes() {
		if v == roleType {
			return true
		}
	}
	return false
}

// IsValidPermission checks whether the given string is a valid permission name.
func IsValidPermission(perm string) bool {
	for _, p := range AllPermissions() {
		if p.Name == perm {
			return true
		}
	}
	return false
}

// PermissionPreset represents a named set of permissions that can be applied to a role.
type PermissionPreset struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}
