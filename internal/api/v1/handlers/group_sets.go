package handlers

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

// Group set tools (G100): the unassigned-student roster, auto-assign,
// bulk group creation, and CSV import / export. Every route is for the
// instructors of the set's course.

// maxGroupImportBytes bounds an uploaded group CSV.
const maxGroupImportBytes = 5 << 20

// loadInstructorCourseCategory loads :group_category_id in the caller's
// tenant and requires a course set the caller teaches. It follows the
// (result, wrote, err) convention: on wrote=true the caller returns err.
func (h *GroupHandler) loadInstructorCourseCategory(c *fiber.Ctx) (*models.GroupCategory, bool, error) {
	id, err := c.ParamsInt("group_category_id")
	if err != nil || id <= 0 {
		return nil, true, responses.BadRequest(c, "Invalid category ID")
	}
	category, err := h.groupService.GetCategory(c.Context(), uint(id), callerAccountID(c))
	if err != nil || category == nil || category.WorkflowState == "deleted" {
		return nil, true, responses.NotFound(c, "group category")
	}
	if category.CourseID == nil || *category.CourseID == 0 {
		return nil, true, responses.NotFound(c, "group category")
	}
	if err := h.authz.RequireCourseInstructor(c, *category.CourseID); err != nil {
		return nil, true, err
	}
	return category, false, nil
}

// groupSetError maps group-set tool errors onto HTTP statuses.
func groupSetError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, service.ErrNoGroupsInSet),
		errors.Is(err, service.ErrGroupsNotEmptyBySection),
		errors.Is(err, service.ErrGroupsLimitedBySection),
		errors.Is(err, service.ErrTooFewGroupsBySection),
		errors.Is(err, service.ErrImportEmpty),
		errors.Is(err, service.ErrImportColumns),
		errors.Is(err, service.ErrImportTooLarge),
		errors.Is(err, service.ErrGroupSetNotInCourse),
		errors.Is(err, service.ErrAutoAssignStudentGroups):
		return responses.BadRequest(c, err.Error())
	default:
		return responses.InternalError(c, "Could not update the group set")
	}
}

// ListGroupCategoryUsers lists the course's students for a group set;
// with ?unassigned=true only those in no group of the set (Canvas
// GET /group_categories/:id/users?unassigned=true).
// GET /api/v1/group_categories/:group_category_id/users
func (h *GroupHandler) ListGroupCategoryUsers(c *fiber.Ctx) error {
	category, wrote, err := h.loadInstructorCourseCategory(c)
	if wrote {
		return err
	}
	var students []service.GroupSetStudent
	if c.Query("unassigned") == "true" {
		students, err = h.groupService.UnassignedStudents(c.Context(), category, callerAccountID(c))
	} else {
		students, err = h.groupService.CourseStudentsForSet(c.Context(), category, callerAccountID(c))
	}
	if err != nil {
		return groupSetError(c, err)
	}
	// FERPA audit: the roster carries login ids, emails and SIS ids
	// (bulk read keyed on the group set, like ListGroupMemberships).
	if callerID, _ := c.Locals("user_id").(uint); callerID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, 0, "read", "bulk_group_set_roster", "group_categories", category.ID, c.IP(), c.Get("User-Agent"))
	}
	return c.JSON(students)
}

// AssignUnassignedMembers spreads the set's unassigned students over its
// groups (Canvas POST /group_categories/:id/assign_unassigned_members).
// Body/query: group_by_section=true keeps each group to one section.
// POST /api/v1/group_categories/:group_category_id/assign_unassigned_members
func (h *GroupHandler) AssignUnassignedMembers(c *fiber.Ctx) error {
	category, wrote, err := h.loadInstructorCourseCategory(c)
	if wrote {
		return err
	}
	var input struct {
		GroupBySection bool `json:"group_by_section"`
	}
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&input); err != nil {
			return responses.BadRequest(c, "Invalid input")
		}
	}
	bySection := input.GroupBySection || c.Query("group_by_section") == "true"
	created, err := h.groupService.AssignUnassigned(c.Context(), category, callerAccountID(c), bySection)
	if err != nil {
		return groupSetError(c, err)
	}
	out := make([]fiber.Map, len(created))
	for i := range created {
		out[i] = groupMembershipToJSON(&created[i], false)
	}
	return c.JSON(fiber.Map{
		"assigned":    len(created),
		"memberships": out,
	})
}

// CreateGroupsInCategory creates groups in bulk: either count groups, or
// as many groups of member_count students as the unassigned students
// need; assign_unassigned_members=true then fills them.
// POST /api/v1/group_categories/:group_category_id/create_groups
func (h *GroupHandler) CreateGroupsInCategory(c *fiber.Ctx) error {
	category, wrote, err := h.loadInstructorCourseCategory(c)
	if wrote {
		return err
	}
	var input struct {
		Count                   int  `json:"count"`
		MemberCount             int  `json:"member_count"`
		AssignUnassignedMembers bool `json:"assign_unassigned_members"`
		GroupBySection          bool `json:"group_by_section"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	groups, assigned, wrote, err := h.createAndFillGroups(c, category, input.Count, input.MemberCount, input.AssignUnassignedMembers, input.GroupBySection)
	if wrote {
		return err
	}
	out := make([]fiber.Map, len(groups))
	for i := range groups {
		out[i] = groupToJSON(&groups[i])
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"groups": out, "assigned": assigned})
}

// createAndFillGroups is the shared bulk-create path (also used by
// CreateGroupCategory's create_group_count). (result, wrote, err).
func (h *GroupHandler) createAndFillGroups(c *fiber.Ctx, category *models.GroupCategory, count, memberCount int, assign, bySection bool) ([]models.Group, int, bool, error) {
	if count < 0 || memberCount < 0 {
		return nil, 0, true, responses.BadRequest(c, "count must not be negative")
	}
	if count == 0 && memberCount > 0 {
		n, err := h.groupService.GroupCountForSize(c.Context(), category, callerAccountID(c), memberCount)
		if err != nil {
			return nil, 0, true, groupSetError(c, err)
		}
		count = n
		assign = true
	}
	// Refuse an assignment that can't run before any group is created.
	if assign {
		if err := service.CheckAutoAssign(category); err != nil {
			return nil, 0, true, groupSetError(c, err)
		}
	}
	groups, err := h.groupService.CreateGroupsInSet(c.Context(), category, count)
	if err != nil {
		return nil, 0, true, groupSetError(c, err)
	}
	assigned := 0
	if assign && len(groups) > 0 {
		created, err := h.groupService.AssignUnassigned(c.Context(), category, callerAccountID(c), bySection)
		if err != nil {
			return nil, 0, true, groupSetError(c, err)
		}
		assigned = len(created)
	}
	return groups, assigned, false, nil
}

// ImportGroupCategoryCSV assigns students to groups from an uploaded CSV
// (multipart field "attachment", or a raw text/csv body).
// POST /api/v1/group_categories/:group_category_id/import
func (h *GroupHandler) ImportGroupCategoryCSV(c *fiber.Ctx) error {
	category, wrote, err := h.loadInstructorCourseCategory(c)
	if wrote {
		return err
	}
	var data []byte
	if fh, ferr := c.FormFile("attachment"); ferr == nil {
		if fh.Size > maxGroupImportBytes {
			return responses.BadRequest(c, "the CSV file is too large (5 MB maximum)")
		}
		f, err := fh.Open()
		if err != nil {
			return responses.BadRequest(c, "could not read the uploaded file")
		}
		defer func() { _ = f.Close() }()
		data, err = io.ReadAll(io.LimitReader(f, maxGroupImportBytes+1))
		if err != nil {
			return responses.BadRequest(c, "could not read the uploaded file")
		}
	} else {
		data = c.Body()
	}
	if len(data) > maxGroupImportBytes {
		return responses.BadRequest(c, "the CSV file is too large (5 MB maximum)")
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return responses.BadRequest(c, service.ErrImportEmpty.Error())
	}
	result, err := h.groupService.ImportGroupsCSV(c.Context(), category, callerAccountID(c), bytes.NewReader(data))
	if err != nil {
		var msg string
		if result != nil {
			msg = fmt.Sprintf(" (stopped after %d rows)", result.RowsProcessed)
		}
		if errors.Is(err, service.ErrImportColumns) || errors.Is(err, service.ErrImportEmpty) || errors.Is(err, service.ErrImportTooLarge) {
			return groupSetError(c, err)
		}
		if result == nil {
			return responses.BadRequest(c, err.Error())
		}
		return responses.InternalError(c, "Could not finish the import"+msg)
	}
	return c.JSON(result)
}

var unsafeFilenameChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// ExportGroupCategoryCSV downloads the set's memberships as CSV.
// GET /api/v1/group_categories/:group_category_id/export
func (h *GroupHandler) ExportGroupCategoryCSV(c *fiber.Ctx) error {
	category, wrote, err := h.loadInstructorCourseCategory(c)
	if wrote {
		return err
	}
	var buf bytes.Buffer
	if err := h.groupService.ExportGroupsCSV(c.Context(), category, callerAccountID(c), &buf); err != nil {
		return groupSetError(c, err)
	}
	// FERPA audit: the export carries the same roster PII (login ids,
	// emails, SIS ids) as the roster read.
	if callerID, _ := c.Locals("user_id").(uint); callerID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, 0, "export", "bulk_group_set_roster", "group_categories", category.ID, c.IP(), c.Get("User-Agent"))
	}
	name := unsafeFilenameChars.ReplaceAllString(category.Name, "_")
	if name == "" {
		name = "groups"
	}
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s.csv"`, name))
	c.Set("X-Content-Type-Options", "nosniff")
	return c.Send(buf.Bytes())
}
