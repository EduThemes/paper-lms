package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

type GroupHandler struct {
	groupService *service.GroupService
	authz        *ResourceAuthorizer
	// Wave 5.2: 13.5 LogPIIAccess sweep. ListGroupMemberships surfaces
	// names, emails, and login_ids of every member — that's the only
	// student-keyed read here. Group/category metadata reads are not
	// student PII.
	auditService *service.AuditService
}

func NewGroupHandler(groupService *service.GroupService, authz *ResourceAuthorizer, auditService *service.AuditService) *GroupHandler {
	return &GroupHandler{groupService: groupService, authz: authz, auditService: auditService}
}

// getCourseIDFromCategory fetches a group category and returns its CourseID.
// Returns 0 if the category has no associated course.
func (h *GroupHandler) getCourseIDFromCategory(c *fiber.Ctx, categoryID uint) (uint, error) {
	category, err := h.groupService.GetCategory(c.Context(), categoryID, callerAccountID(c))
	if err != nil {
		return 0, err
	}
	if category.CourseID != nil {
		return *category.CourseID, nil
	}
	return 0, nil
}

// getCourseIDFromGroup fetches a group, then its category, to resolve the CourseID.
// Returns 0 if the category has no associated course.
func (h *GroupHandler) getCourseIDFromGroup(c *fiber.Ctx, groupID uint) (uint, error) {
	group, err := h.groupService.GetGroup(c.Context(), groupID, callerAccountID(c))
	if err != nil {
		return 0, err
	}
	return h.getCourseIDFromCategory(c, group.GroupCategoryID)
}

// ---- JSON converters ----

func groupCategoryToJSON(c *models.GroupCategory) fiber.Map {
	return fiber.Map{
		"id":             c.ID,
		"course_id":      c.CourseID,
		"account_id":     c.AccountID,
		"name":           c.Name,
		"self_signup":    c.SelfSignup,
		"group_limit":    c.GroupLimit,
		"auto_leader":    c.AutoLeader,
		"role":           c.Role,
		"workflow_state": c.WorkflowState,
		"created_at":     c.CreatedAt,
		"updated_at":     c.UpdatedAt,
	}
}

func groupToJSON(g *models.Group) fiber.Map {
	return fiber.Map{
		"id":                g.ID,
		"group_category_id": g.GroupCategoryID,
		"name":              g.Name,
		"description":       g.Description,
		"max_membership":    g.MaxMembership,
		"is_public":         g.IsPublic,
		"join_level":        g.JoinLevel,
		"context_type":      g.ContextType,
		"context_id":        g.ContextID,
		"workflow_state":    g.WorkflowState,
		"created_at":        g.CreatedAt,
		"updated_at":        g.UpdatedAt,
	}
}

func groupMembershipToJSON(m *models.GroupMembership) fiber.Map {
	result := fiber.Map{
		"id":             m.ID,
		"group_id":       m.GroupID,
		"user_id":        m.UserID,
		"workflow_state": m.WorkflowState,
		"moderator":      m.Moderator,
		"created_at":     m.CreatedAt,
		"updated_at":     m.UpdatedAt,
	}
	if m.User != nil {
		result["user"] = fiber.Map{
			"id":            m.User.ID,
			"name":          m.User.Name,
			"sortable_name": m.User.SortableName,
			"short_name":    m.User.ShortName,
			"login_id":      m.User.LoginID,
			"email":         m.User.Email,
		}
	}
	return result
}

// ---- Group Category handlers ----

func (h *GroupHandler) ListGroupCategories(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.groupService.ListCategoriesByCourse(c.Context(), uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch group categories")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	categories := make([]fiber.Map, len(result.Items))
	for i, cat := range result.Items {
		categories[i] = groupCategoryToJSON(&cat)
	}

	return c.JSON(categories)
}

func (h *GroupHandler) CreateGroupCategory(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		GroupCategory struct {
			Name       string `json:"name"`
			SelfSignup string `json:"self_signup"`
			GroupLimit *int   `json:"group_limit"`
			AutoLeader string `json:"auto_leader"`
			Role       string `json:"role"`
		} `json:"group_category"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	cid := uint(courseID)
	category := &models.GroupCategory{
		CourseID:   &cid,
		Name:       input.GroupCategory.Name,
		SelfSignup: input.GroupCategory.SelfSignup,
		GroupLimit: input.GroupCategory.GroupLimit,
		AutoLeader: input.GroupCategory.AutoLeader,
		Role:       input.GroupCategory.Role,
	}

	if err := h.groupService.CreateCategory(c.Context(), category); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(groupCategoryToJSON(category))
}

func (h *GroupHandler) GetGroupCategory(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid category ID")
	}

	category, err := h.groupService.GetCategory(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "group category")
	}

	// Authorization: require enrollment for course-scoped categories
	if category.CourseID != nil {
		if err := h.authz.RequireCourseEnrolled(c, *category.CourseID); err != nil {
			return err
		}
	}

	return c.JSON(groupCategoryToJSON(category))
}

func (h *GroupHandler) UpdateGroupCategory(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid category ID")
	}

	category, err := h.groupService.GetCategory(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "group category")
	}

	// Authorization: require instructor for course-scoped categories
	if category.CourseID != nil {
		if err := h.authz.RequireCourseInstructor(c, *category.CourseID); err != nil {
			return err
		}
	}

	var input struct {
		GroupCategory struct {
			Name       *string `json:"name"`
			SelfSignup *string `json:"self_signup"`
			GroupLimit *int    `json:"group_limit"`
			AutoLeader *string `json:"auto_leader"`
			Role       *string `json:"role"`
		} `json:"group_category"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.GroupCategory.Name != nil {
		category.Name = *input.GroupCategory.Name
	}
	if input.GroupCategory.SelfSignup != nil {
		category.SelfSignup = *input.GroupCategory.SelfSignup
	}
	if input.GroupCategory.GroupLimit != nil {
		category.GroupLimit = input.GroupCategory.GroupLimit
	}
	if input.GroupCategory.AutoLeader != nil {
		category.AutoLeader = *input.GroupCategory.AutoLeader
	}
	if input.GroupCategory.Role != nil {
		category.Role = *input.GroupCategory.Role
	}

	if err := h.groupService.UpdateCategory(c.Context(), category); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(groupCategoryToJSON(category))
}

func (h *GroupHandler) DeleteGroupCategory(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid category ID")
	}

	// Fetch first to check authorization
	category, err := h.groupService.GetCategory(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "group category")
	}

	// Authorization: require instructor for course-scoped categories
	if category.CourseID != nil {
		if err := h.authz.RequireCourseInstructor(c, *category.CourseID); err != nil {
			return err
		}
	}

	if err := h.groupService.DeleteCategory(c.Context(), category.ID); err != nil {
		return responses.InternalError(c, "Could not delete group category")
	}

	return c.JSON(fiber.Map{"delete": true})
}

// ---- Group handlers ----

func (h *GroupHandler) ListGroupsByCategory(c *fiber.Ctx) error {
	categoryID, err := c.ParamsInt("group_category_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid category ID")
	}

	// Authorization: require enrollment for course-scoped categories
	courseID, err := h.getCourseIDFromCategory(c, uint(categoryID))
	if err != nil {
		return responses.NotFound(c, "group category")
	}
	if courseID != 0 {
		if err := h.authz.RequireCourseEnrolled(c, courseID); err != nil {
			return err
		}
	}

	params := middleware.GetPagination(c)

	result, err := h.groupService.ListGroupsByCategory(c.Context(), uint(categoryID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch groups")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	groups := make([]fiber.Map, len(result.Items))
	for i, g := range result.Items {
		groups[i] = groupToJSON(&g)
	}

	return c.JSON(groups)
}

func (h *GroupHandler) CreateGroup(c *fiber.Ctx) error {
	categoryID, err := c.ParamsInt("group_category_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid category ID")
	}

	// Authorization: require instructor for course-scoped categories
	courseID, err := h.getCourseIDFromCategory(c, uint(categoryID))
	if err != nil {
		return responses.NotFound(c, "group category")
	}
	if courseID != 0 {
		if err := h.authz.RequireCourseInstructor(c, courseID); err != nil {
			return err
		}
	}

	var input struct {
		Group struct {
			Name          string `json:"name"`
			Description   string `json:"description"`
			MaxMembership *int   `json:"max_membership"`
			IsPublic      bool   `json:"is_public"`
			JoinLevel     string `json:"join_level"`
			ContextType   string `json:"context_type"`
			ContextID     uint   `json:"context_id"`
		} `json:"group"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	group := &models.Group{
		GroupCategoryID: uint(categoryID),
		Name:            input.Group.Name,
		Description:     service.SanitizeHTML(input.Group.Description),
		MaxMembership:   input.Group.MaxMembership,
		IsPublic:        input.Group.IsPublic,
		JoinLevel:       input.Group.JoinLevel,
		ContextType:     input.Group.ContextType,
		ContextID:       input.Group.ContextID,
	}

	if err := h.groupService.CreateGroup(c.Context(), group); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(groupToJSON(group))
}

func (h *GroupHandler) GetGroup(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid group ID")
	}

	group, err := h.groupService.GetGroup(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "group")
	}

	// Authorization: require enrollment for course-scoped groups
	courseID, err := h.getCourseIDFromCategory(c, group.GroupCategoryID)
	if err == nil && courseID != 0 {
		if err := h.authz.RequireCourseEnrolled(c, courseID); err != nil {
			return err
		}
	}

	return c.JSON(groupToJSON(group))
}

func (h *GroupHandler) UpdateGroup(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid group ID")
	}

	group, err := h.groupService.GetGroup(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "group")
	}

	// Authorization: require instructor for course-scoped groups
	courseID, err := h.getCourseIDFromCategory(c, group.GroupCategoryID)
	if err == nil && courseID != 0 {
		if err := h.authz.RequireCourseInstructor(c, courseID); err != nil {
			return err
		}
	}

	var input struct {
		Group struct {
			Name          *string `json:"name"`
			Description   *string `json:"description"`
			MaxMembership *int    `json:"max_membership"`
			IsPublic      *bool   `json:"is_public"`
			JoinLevel     *string `json:"join_level"`
		} `json:"group"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Group.Name != nil {
		group.Name = *input.Group.Name
	}
	if input.Group.Description != nil {
		group.Description = service.SanitizeHTML(*input.Group.Description)
	}
	if input.Group.MaxMembership != nil {
		group.MaxMembership = input.Group.MaxMembership
	}
	if input.Group.IsPublic != nil {
		group.IsPublic = *input.Group.IsPublic
	}
	if input.Group.JoinLevel != nil {
		group.JoinLevel = *input.Group.JoinLevel
	}

	if err := h.groupService.UpdateGroup(c.Context(), group); err != nil {
		return responses.InternalError(c, "Could not update group")
	}

	return c.JSON(groupToJSON(group))
}

func (h *GroupHandler) DeleteGroup(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid group ID")
	}

	// Fetch first to check authorization
	group, err := h.groupService.GetGroup(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "group")
	}

	// Authorization: require instructor for course-scoped groups
	courseID, err := h.getCourseIDFromCategory(c, group.GroupCategoryID)
	if err == nil && courseID != 0 {
		if err := h.authz.RequireCourseInstructor(c, courseID); err != nil {
			return err
		}
	}

	if err := h.groupService.DeleteGroup(c.Context(), group.ID); err != nil {
		return responses.InternalError(c, "Could not delete group")
	}

	return c.JSON(fiber.Map{"delete": true})
}

// ---- Group Membership handlers ----

func (h *GroupHandler) ListGroupMemberships(c *fiber.Ctx) error {
	groupID, err := c.ParamsInt("group_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid group ID")
	}

	// Authorization: require enrollment for course-scoped groups
	courseID, err := h.getCourseIDFromGroup(c, uint(groupID))
	if err != nil {
		return responses.NotFound(c, "group")
	}
	if courseID != 0 {
		if err := h.authz.RequireCourseEnrolled(c, courseID); err != nil {
			return err
		}
	}

	params := middleware.GetPagination(c)

	result, err := h.groupService.ListMembers(c.Context(), uint(groupID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch group memberships")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	memberships := make([]fiber.Map, len(result.Items))
	for i, m := range result.Items {
		memberships[i] = groupMembershipToJSON(&m)
	}

	// Wave 5.2 FERPA audit: membership list embeds user.email +
	// user.login_id for every group member — directory-level PII.
	// Bulk read (studentID=0) keyed on the group resource.
	if callerID, _ := getUserID(c); callerID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, 0, "read", "bulk_group_memberships", "groups", uint(groupID), c.IP(), c.Get("User-Agent"))
	}

	return c.JSON(memberships)
}

// CreateGroupMembership adds a member to a group.
//
// SECURITY (F-017): the pre-fix path only enforced authz when the
// group was course-scoped (RequireCourseInstructor). For non-course
// groups (account-level / personal), ANY authenticated user could
// add ANY other user with `moderator: true` and `workflow_state:
// "accepted"`. Fix:
//   - course-scoped groups: instructor of the course (preserved).
//   - non-course groups: caller MUST be either an existing group
//     moderator OR an admin, OR be adding themselves with
//     moderator=false (self-signup path).
//   - moderator=true on self-signup is silently downgraded; only an
//     existing moderator / admin can promote.
//   - workflow_state restricted to "accepted" / "requested" for
//     self-signup; only mod/admin can flip to anything else.
func (h *GroupHandler) CreateGroupMembership(c *fiber.Ctx) error {
	groupID, err := c.ParamsInt("group_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid group ID")
	}

	// Course-scoped groups: same instructor gate as before.
	courseID, err := h.getCourseIDFromGroup(c, uint(groupID))
	if err != nil {
		return responses.NotFound(c, "group")
	}

	var input struct {
		Membership struct {
			UserID        uint   `json:"user_id"`
			WorkflowState string `json:"workflow_state"`
			Moderator     bool   `json:"moderator"`
		} `json:"membership"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	callerID, _ := c.Locals("user_id").(uint)
	isAdmin, _ := c.Locals("is_admin").(bool)

	// If user_id not provided, use the authenticated user (self-signup)
	userID := input.Membership.UserID
	if userID == 0 {
		userID = callerID
	}
	if userID == 0 {
		return responses.BadRequest(c, "user_id is required")
	}

	if courseID != 0 {
		if err := h.authz.RequireCourseInstructor(c, courseID); err != nil {
			return err
		}
	} else {
		// Non-course group. Self-signup is OK with constrained fields;
		// adding others or setting moderator requires an existing
		// moderator role OR admin.
		isCallerMod := h.callerIsGroupModerator(c, uint(groupID), callerID)
		if !isAdmin && !isCallerMod {
			if userID != callerID {
				return responses.Forbidden(c, "only group moderators or admins can add other members")
			}
			if input.Membership.Moderator {
				return responses.Forbidden(c, "cannot self-grant moderator status")
			}
			switch input.Membership.WorkflowState {
			case "", "accepted", "requested":
				// allowed for self-signup
			default:
				return responses.Forbidden(c, "workflow_state restricted on self-signup")
			}
		}
	}

	membership := &models.GroupMembership{
		GroupID:       uint(groupID),
		UserID:        userID,
		WorkflowState: input.Membership.WorkflowState,
		Moderator:     input.Membership.Moderator,
	}

	if err := h.groupService.AddMember(c.Context(), membership); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(groupMembershipToJSON(membership))
}

// callerIsGroupModerator returns true if the caller has an accepted
// moderator membership in the given group. Cheap helper used by the
// non-course CreateGroupMembership path; on the (rare) miss we fall
// back to admin-or-self rules.
func (h *GroupHandler) callerIsGroupModerator(c *fiber.Ctx, groupID, callerID uint) bool {
	if callerID == 0 {
		return false
	}
	members, err := h.groupService.ListMembers(c.Context(), groupID, repository.PaginationParams{Page: 1, PerPage: 10000})
	if err != nil || members == nil {
		return false
	}
	for _, m := range members.Items {
		if m.UserID == callerID && m.Moderator && m.WorkflowState == "accepted" {
			return true
		}
	}
	return false
}

// UpdateGroupMembership updates an existing membership (workflow_state /
// moderator).
//
// SECURITY (F-017): non-course groups previously allowed ANY
// authenticated user to flip workflow_state and moderator on any
// membership. Now: course groups still gated by instructor; non-
// course groups require admin OR an existing moderator in that group.
func (h *GroupHandler) UpdateGroupMembership(c *fiber.Ctx) error {
	membershipID, err := c.ParamsInt("membership_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid membership ID")
	}

	membership, err := h.groupService.GetMembership(c.Context(), uint(membershipID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "group membership")
	}

	// Authorization: course-scoped groups require instructor; account-
	// scoped (non-course) groups require admin. Before the 2026-05-22
	// audit fix, the account-scope branch had NO authz gate, which let
	// a pending member self-accept by writing workflow_state=accepted.
	courseID, err := h.getCourseIDFromGroup(c, membership.GroupID)
	if err == nil && courseID != 0 {
		if err := h.authz.RequireCourseInstructor(c, courseID); err != nil {
			return err
		}
	} else {
		callerID, _ := c.Locals("user_id").(uint)
		isAdmin, _ := c.Locals("is_admin").(bool)
		if !isAdmin && !h.callerIsGroupModerator(c, membership.GroupID, callerID) {
			return responses.Forbidden(c, "only group moderators or admins can modify memberships")
		}
	}

	var input struct {
		Membership struct {
			WorkflowState *string `json:"workflow_state"`
			Moderator     *bool   `json:"moderator"`
		} `json:"membership"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Membership.WorkflowState != nil {
		membership.WorkflowState = *input.Membership.WorkflowState
	}
	if input.Membership.Moderator != nil {
		membership.Moderator = *input.Membership.Moderator
	}

	if err := h.groupService.UpdateMembership(c.Context(), membership); err != nil {
		return responses.InternalError(c, "Could not update group membership")
	}

	return c.JSON(groupMembershipToJSON(membership))
}

// DeleteGroupMembership removes a member from a group.
//
// SECURITY (F-017): same authz shape as UpdateGroupMembership. A
// member may always remove themselves; otherwise the caller must
// be a group moderator or an admin (and for course groups, a course
// instructor).
func (h *GroupHandler) DeleteGroupMembership(c *fiber.Ctx) error {
	membershipID, err := c.ParamsInt("membership_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid membership ID")
	}

	// Fetch first to check authorization
	membership, err := h.groupService.GetMembership(c.Context(), uint(membershipID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "group membership")
	}

	callerID, _ := c.Locals("user_id").(uint)
	isAdmin, _ := c.Locals("is_admin").(bool)

	// Self-removal is always allowed (leave-the-group UX).
	if membership.UserID != callerID {
		// Authorization: require instructor for course-scoped groups
		courseID, err := h.getCourseIDFromGroup(c, membership.GroupID)
		if err == nil && courseID != 0 {
			if err := h.authz.RequireCourseInstructor(c, courseID); err != nil {
				return err
			}
		} else if !isAdmin && !h.callerIsGroupModerator(c, membership.GroupID, callerID) {
			return responses.Forbidden(c, "only group moderators or admins can remove other members")
		}
	}

	if err := h.groupService.RemoveMember(c.Context(), membership.ID); err != nil {
		return responses.InternalError(c, "Could not remove group membership")
	}

	return c.JSON(fiber.Map{"delete": true})
}

// ---- User Groups ----

func (h *GroupHandler) ListUserGroups(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(uint)
	if userID == 0 {
		return responses.Unauthorized(c)
	}

	params := middleware.GetPagination(c)

	result, err := h.groupService.ListUserGroups(c.Context(), userID, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch user groups")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	groups := make([]fiber.Map, len(result.Items))
	for i, g := range result.Items {
		groups[i] = groupToJSON(&g)
	}

	return c.JSON(groups)
}
