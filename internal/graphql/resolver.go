package graphql

import (
	"context"
	"fmt"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

// Resolver executes parsed GraphQL queries against the service layer.
type Resolver struct {
	courseService     *service.CourseService
	assignmentService *service.AssignmentService
	userService       *service.UserService
	enrollmentService *service.EnrollmentService
	moduleService     *service.ModuleService
	submissionService *service.SubmissionService
}

// NewResolver creates a new GraphQL resolver wired to the given services.
func NewResolver(
	courseService *service.CourseService,
	assignmentService *service.AssignmentService,
	userService *service.UserService,
	enrollmentService *service.EnrollmentService,
	moduleService *service.ModuleService,
	submissionService *service.SubmissionService,
) *Resolver {
	return &Resolver{
		courseService:     courseService,
		assignmentService: assignmentService,
		userService:       userService,
		enrollmentService: enrollmentService,
		moduleService:     moduleService,
		submissionService: submissionService,
	}
}

// Resolve parses and executes a GraphQL query, returning a Response.
func (r *Resolver) Resolve(ctx context.Context, userID uint, query string, variables map[string]interface{}) *Response {
	fields, err := ParseQuery(query)
	if err != nil {
		return &Response{
			Errors: []GraphQLError{{Message: fmt.Sprintf("parse error: %s", err.Error())}},
		}
	}

	data := make(map[string]interface{})
	var errors []GraphQLError

	for _, field := range fields {
		// Resolve variable references in arguments
		resolvedArgs := resolveVariables(field.Arguments, variables)

		result, fieldErr := r.resolveRootField(ctx, userID, field.Name, resolvedArgs, field.Fields, variables)
		if fieldErr != nil {
			errors = append(errors, GraphQLError{
				Message: fieldErr.Error(),
				Path:    []string{field.Name},
			})
			data[field.Name] = nil
		} else {
			data[field.Name] = result
		}
	}

	resp := &Response{Data: data}
	if len(errors) > 0 {
		resp.Errors = errors
	}
	return resp
}

// resolveVariables replaces $variable references in arguments with values from the variables map.
func resolveVariables(args map[string]interface{}, variables map[string]interface{}) map[string]interface{} {
	if variables == nil || args == nil {
		return args
	}
	resolved := make(map[string]interface{}, len(args))
	for k, v := range args {
		if s, ok := v.(string); ok && len(s) > 1 && s[0] == '$' {
			varName := s[1:]
			if val, exists := variables[varName]; exists {
				resolved[k] = val
				continue
			}
		}
		resolved[k] = v
	}
	return resolved
}

func (r *Resolver) resolveRootField(ctx context.Context, userID uint, name string, args map[string]interface{}, subFields []Field, variables map[string]interface{}) (interface{}, error) {
	switch name {
	case "course":
		return r.resolveCourse(ctx, args, subFields, variables)
	case "allCourses":
		return r.resolveAllCourses(ctx, args, subFields, variables)
	case "assignment":
		return r.resolveAssignment(ctx, args, subFields)
	case "self":
		return r.resolveSelf(ctx, userID, subFields)
	case "user":
		return r.resolveUser(ctx, args, subFields)
	default:
		return nil, fmt.Errorf("unknown field: %s", name)
	}
}

// --- Root resolvers ---

func (r *Resolver) resolveCourse(ctx context.Context, args map[string]interface{}, subFields []Field, variables map[string]interface{}) (interface{}, error) {
	id, err := getUintArg(args, "id")
	if err != nil {
		return nil, fmt.Errorf("course requires 'id' argument: %w", err)
	}

	course, err := r.courseService.GetByID(ctx, id, 0)
	if err != nil {
		return nil, fmt.Errorf("course not found: %w", err)
	}

	return r.buildCourseMap(ctx, course, subFields, variables)
}

func (r *Resolver) resolveAllCourses(ctx context.Context, args map[string]interface{}, subFields []Field, variables map[string]interface{}) (interface{}, error) {
	page := getIntArgOr(args, "page", 1)
	perPage := getIntArgOr(args, "perPage", 10)

	params := repository.PaginationParams{
		Page:    page,
		PerPage: perPage,
	}

	result, err := r.courseService.List(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list courses: %w", err)
	}

	var courses []interface{}
	for i := range result.Items {
		c, err := r.buildCourseMap(ctx, &result.Items[i], subFields, variables)
		if err != nil {
			return nil, err
		}
		courses = append(courses, c)
	}

	return courses, nil
}

func (r *Resolver) resolveAssignment(ctx context.Context, args map[string]interface{}, subFields []Field) (interface{}, error) {
	id, err := getUintArg(args, "id")
	if err != nil {
		return nil, fmt.Errorf("assignment requires 'id' argument: %w", err)
	}

	assignment, err := r.assignmentService.GetByID(ctx, id, 0)
	if err != nil {
		return nil, fmt.Errorf("assignment not found: %w", err)
	}

	return buildAssignmentMap(assignment, subFields), nil
}

func (r *Resolver) resolveSelf(ctx context.Context, userID uint, subFields []Field) (interface{}, error) {
	user, err := r.userService.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("current user not found: %w", err)
	}

	return buildUserMap(user, subFields), nil
}

func (r *Resolver) resolveUser(ctx context.Context, args map[string]interface{}, subFields []Field) (interface{}, error) {
	id, err := getUintArg(args, "id")
	if err != nil {
		return nil, fmt.Errorf("user requires 'id' argument: %w", err)
	}

	user, err := r.userService.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return buildUserMap(user, subFields), nil
}

// --- Map builders ---

func (r *Resolver) buildCourseMap(ctx context.Context, course *models.Course, subFields []Field, variables map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for _, f := range subFields {
		switch f.Name {
		case "id":
			result["id"] = course.ID
		case "name":
			result["name"] = course.Name
		case "course_code":
			result["course_code"] = course.CourseCode
		case "workflow_state":
			result["workflow_state"] = course.WorkflowState
		case "account_id":
			result["account_id"] = course.AccountID
		case "start_at":
			result["start_at"] = course.StartAt
		case "end_at":
			result["end_at"] = course.EndAt
		case "default_view":
			result["default_view"] = course.DefaultView
		case "is_public":
			result["is_public"] = course.IsPublic
		case "created_at":
			result["created_at"] = formatTime(course.CreatedAt)
		case "updated_at":
			result["updated_at"] = formatTime(course.UpdatedAt)
		case "assignments":
			resolvedArgs := resolveVariables(f.Arguments, variables)
			assignments, err := r.resolveCourseAssignments(ctx, course.ID, resolvedArgs, f.Fields)
			if err != nil {
				return nil, err
			}
			result["assignments"] = assignments
		case "enrollments":
			resolvedArgs := resolveVariables(f.Arguments, variables)
			enrollments, err := r.resolveCourseEnrollments(ctx, course.ID, resolvedArgs, f.Fields)
			if err != nil {
				return nil, err
			}
			result["enrollments"] = enrollments
		case "modules":
			resolvedArgs := resolveVariables(f.Arguments, variables)
			modules, err := r.resolveCourseModules(ctx, course.ID, resolvedArgs, f.Fields)
			if err != nil {
				return nil, err
			}
			result["modules"] = modules
		}
	}

	// If no sub-fields specified, return default scalar fields
	if len(subFields) == 0 {
		result["id"] = course.ID
		result["name"] = course.Name
		result["course_code"] = course.CourseCode
		result["workflow_state"] = course.WorkflowState
		result["created_at"] = formatTime(course.CreatedAt)
	}

	return result, nil
}

func buildAssignmentMap(a *models.Assignment, subFields []Field) map[string]interface{} {
	result := make(map[string]interface{})

	for _, f := range subFields {
		switch f.Name {
		case "id":
			result["id"] = a.ID
		case "name":
			result["name"] = a.Name
		case "description":
			result["description"] = a.Description
		case "points_possible":
			result["points_possible"] = a.PointsPossible
		case "due_at":
			result["due_at"] = a.DueAt
		case "unlock_at":
			result["unlock_at"] = a.UnlockAt
		case "lock_at":
			result["lock_at"] = a.LockAt
		case "course_id":
			result["course_id"] = a.CourseID
		case "grading_type":
			result["grading_type"] = a.GradingType
		case "submission_types":
			result["submission_types"] = a.SubmissionTypes
		case "workflow_state":
			result["workflow_state"] = a.WorkflowState
		case "published":
			result["published"] = a.Published
		case "position":
			result["position"] = a.Position
		case "created_at":
			result["created_at"] = formatTime(a.CreatedAt)
		case "updated_at":
			result["updated_at"] = formatTime(a.UpdatedAt)
		}
	}

	if len(subFields) == 0 {
		result["id"] = a.ID
		result["name"] = a.Name
		result["description"] = a.Description
		result["points_possible"] = a.PointsPossible
		result["due_at"] = a.DueAt
		result["course_id"] = a.CourseID
	}

	return result
}

func buildUserMap(u *models.User, subFields []Field) map[string]interface{} {
	result := make(map[string]interface{})

	for _, f := range subFields {
		switch f.Name {
		case "id":
			result["id"] = u.ID
		case "name":
			result["name"] = u.Name
		case "sortable_name":
			result["sortable_name"] = u.SortableName
		case "short_name":
			result["short_name"] = u.ShortName
		case "email":
			result["email"] = u.Email
		case "login_id":
			result["login_id"] = u.LoginID
		case "avatar_url":
			result["avatar_url"] = u.AvatarURL
		case "locale":
			result["locale"] = u.Locale
		case "time_zone":
			result["time_zone"] = u.TimeZone
		case "created_at":
			result["created_at"] = formatTime(u.CreatedAt)
		case "updated_at":
			result["updated_at"] = formatTime(u.UpdatedAt)
		}
	}

	if len(subFields) == 0 {
		result["id"] = u.ID
		result["name"] = u.Name
		result["email"] = u.Email
		result["created_at"] = formatTime(u.CreatedAt)
	}

	return result
}

func buildEnrollmentMap(e *models.Enrollment, subFields []Field) map[string]interface{} {
	result := make(map[string]interface{})

	for _, f := range subFields {
		switch f.Name {
		case "id":
			result["id"] = e.ID
		case "user_id":
			result["user_id"] = e.UserID
		case "course_id":
			result["course_id"] = e.CourseID
		case "type":
			result["type"] = e.Type
		case "role":
			result["role"] = e.Role
		case "workflow_state":
			result["workflow_state"] = e.WorkflowState
		case "created_at":
			result["created_at"] = formatTime(e.CreatedAt)
		case "updated_at":
			result["updated_at"] = formatTime(e.UpdatedAt)
		}
	}

	if len(subFields) == 0 {
		result["id"] = e.ID
		result["user_id"] = e.UserID
		result["type"] = e.Type
		result["role"] = e.Role
		result["workflow_state"] = e.WorkflowState
	}

	return result
}

func buildModuleMap(mod *models.ContextModule, subFields []Field) map[string]interface{} {
	result := make(map[string]interface{})

	for _, f := range subFields {
		switch f.Name {
		case "id":
			result["id"] = mod.ID
		case "name":
			result["name"] = mod.Name
		case "position":
			result["position"] = mod.Position
		case "course_id":
			result["course_id"] = mod.CourseID
		case "unlock_at":
			result["unlock_at"] = mod.UnlockAt
		case "require_sequential_progress":
			result["require_sequential_progress"] = mod.RequireSequentialProgress
		case "workflow_state":
			result["workflow_state"] = mod.WorkflowState
		case "created_at":
			result["created_at"] = formatTime(mod.CreatedAt)
		case "updated_at":
			result["updated_at"] = formatTime(mod.UpdatedAt)
		}
	}

	if len(subFields) == 0 {
		result["id"] = mod.ID
		result["name"] = mod.Name
		result["position"] = mod.Position
		result["workflow_state"] = mod.WorkflowState
	}

	return result
}

// --- Nested resolvers ---

func (r *Resolver) resolveCourseAssignments(ctx context.Context, courseID uint, args map[string]interface{}, subFields []Field) (interface{}, error) {
	page := getIntArgOr(args, "page", 1)
	perPage := getIntArgOr(args, "perPage", 100)

	params := repository.PaginationParams{Page: page, PerPage: perPage}
	result, err := r.assignmentService.ListByCourse(ctx, courseID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list assignments for course %d: %w", courseID, err)
	}

	var items []interface{}
	for i := range result.Items {
		items = append(items, buildAssignmentMap(&result.Items[i], subFields))
	}

	return items, nil
}

func (r *Resolver) resolveCourseEnrollments(ctx context.Context, courseID uint, args map[string]interface{}, subFields []Field) (interface{}, error) {
	page := getIntArgOr(args, "page", 1)
	perPage := getIntArgOr(args, "perPage", 100)

	params := repository.PaginationParams{Page: page, PerPage: perPage}
	result, err := r.enrollmentService.ListByCourse(ctx, courseID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list enrollments for course %d: %w", courseID, err)
	}

	var items []interface{}
	for i := range result.Items {
		items = append(items, buildEnrollmentMap(&result.Items[i], subFields))
	}

	return items, nil
}

func (r *Resolver) resolveCourseModules(ctx context.Context, courseID uint, args map[string]interface{}, subFields []Field) (interface{}, error) {
	page := getIntArgOr(args, "page", 1)
	perPage := getIntArgOr(args, "perPage", 100)

	params := repository.PaginationParams{Page: page, PerPage: perPage}
	result, err := r.moduleService.ListByCourse(ctx, courseID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list modules for course %d: %w", courseID, err)
	}

	var items []interface{}
	for i := range result.Items {
		items = append(items, buildModuleMap(&result.Items[i], subFields))
	}

	return items, nil
}

// --- Argument helpers ---

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func getUintArg(args map[string]interface{}, name string) (uint, error) {
	v, ok := args[name]
	if !ok {
		return 0, fmt.Errorf("missing argument '%s'", name)
	}

	switch val := v.(type) {
	case int:
		if val < 0 {
			return 0, fmt.Errorf("argument '%s' must be non-negative", name)
		}
		return uint(val), nil
	case float64:
		if val < 0 {
			return 0, fmt.Errorf("argument '%s' must be non-negative", name)
		}
		return uint(val), nil
	case string:
		var n int
		_, err := fmt.Sscanf(val, "%d", &n)
		if err != nil {
			return 0, fmt.Errorf("argument '%s' must be an integer, got '%s'", name, val)
		}
		if n < 0 {
			return 0, fmt.Errorf("argument '%s' must be non-negative", name)
		}
		return uint(n), nil
	default:
		return 0, fmt.Errorf("argument '%s' has unsupported type %T", name, v)
	}
}

func getIntArgOr(args map[string]interface{}, name string, defaultVal int) int {
	v, ok := args[name]
	if !ok {
		return defaultVal
	}

	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	case string:
		var n int
		_, err := fmt.Sscanf(val, "%d", &n)
		if err != nil {
			return defaultVal
		}
		return n
	default:
		return defaultVal
	}
}
