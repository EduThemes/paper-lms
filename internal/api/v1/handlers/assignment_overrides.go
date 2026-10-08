package handlers

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type AssignmentOverrideHandler struct {
	overrideService   *service.OverrideService
	assignmentService *service.AssignmentService
	// gradingPolicy (optional) refreshes late / missing statuses when an
	// override changes a student's dates (G019).
	gradingPolicy *service.GradingPolicyService
}

// SetGradingPolicyService wires the late/missing refresh.
func (h *AssignmentOverrideHandler) SetGradingPolicyService(p *service.GradingPolicyService) {
	h.gradingPolicy = p
}

// refreshDueStatus recomputes the assignment's late / missing statuses.
func (h *AssignmentOverrideHandler) refreshDueStatus(c *fiber.Ctx, courseID, assignmentID uint) {
	if h.gradingPolicy != nil {
		_ = h.gradingPolicy.RefreshDueStatus(c.Context(), courseID, []uint{assignmentID})
	}
}

func NewAssignmentOverrideHandler(overrideService *service.OverrideService, assignmentService *service.AssignmentService) *AssignmentOverrideHandler {
	return &AssignmentOverrideHandler{
		overrideService:   overrideService,
		assignmentService: assignmentService,
	}
}

// overrideInCourse returns true if (a) the override's AssignmentID
// matches the URL's :assignment_id and (b) that assignment's CourseID
// matches the URL's :course_id (loaded under the caller's tenant).
// Returns false on any mismatch — the caller MUST emit
// responses.NotFound when this returns false. PENTEST F-013 closure
// for the assignment-override surface.
//
// This is a value-returning helper (NOT a wrote-the-response helper)
// to avoid the (result, wrote, err) pitfall: responses.NotFound
// returns nil after writing, so a helper that called it would
// silently let the caller fall through to the next step.
func (h *AssignmentOverrideHandler) overrideInCourse(c *fiber.Ctx, override *models.AssignmentOverride, urlAssignmentID, urlCourseID uint) bool {
	if override.AssignmentID != urlAssignmentID {
		return false
	}
	assignment, err := h.assignmentService.GetByID(c.Context(), override.AssignmentID, callerAccountID(c))
	if err != nil {
		return false
	}
	return assignment.CourseID == urlCourseID
}

// requireAssignmentInCourse loads :assignment_id under the caller's
// tenant and verifies it belongs to :course_id. Writes a 404 and returns
// wrote=true on any miss; the caller MUST `return nil`.
func (h *AssignmentOverrideHandler) requireAssignmentInCourse(c *fiber.Ctx, assignmentID, courseID uint) (*models.Assignment, bool) {
	assignment, err := h.assignmentService.GetByID(c.Context(), assignmentID, callerAccountID(c))
	if err != nil || assignment == nil || assignment.CourseID != courseID || assignment.WorkflowState == models.AssignmentDeleted {
		_ = responses.NotFound(c, "assignment")
		return nil, true
	}
	return assignment, false
}

func assignmentOverrideToJSON(o *models.AssignmentOverride) fiber.Map {
	return fiber.Map{
		"id":                o.ID,
		"assignment_id":     o.AssignmentID,
		"title":             o.Title,
		"due_at":            o.DueAt,
		"unlock_at":         o.UnlockAt,
		"lock_at":           o.LockAt,
		"all_day":           o.AllDay,
		"all_day_date":      o.AllDayDate,
		"course_section_id": o.CourseSectionID,
		"workflow_state":    o.WorkflowState,
		"created_at":        o.CreatedAt,
		"updated_at":        o.UpdatedAt,
	}
}

func (h *AssignmentOverrideHandler) ListOverrides(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}
	// B019: F-013 parent-tie — the assignment must be in :course_id and
	// the caller's tenant before its overrides (and student ids) are read.
	if _, wrote := h.requireAssignmentInCourse(c, uint(assignmentID), uint(courseID)); wrote {
		return nil
	}

	overrides, err := h.overrideService.ListOverrides(c.Context(), uint(assignmentID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch assignment overrides")
	}

	result := make([]fiber.Map, len(overrides))
	for i, o := range overrides {
		// Fetch student IDs for each override
		students, _ := h.overrideService.ListStudents(c.Context(), o.ID)
		studentIDs := make([]uint, len(students))
		for j, s := range students {
			studentIDs[j] = s.UserID
		}

		overrideJSON := assignmentOverrideToJSON(&o)
		overrideJSON["student_ids"] = studentIDs
		result[i] = overrideJSON
	}

	return c.JSON(result)
}

func (h *AssignmentOverrideHandler) CreateOverride(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}
	// B019: F-013 parent-tie before attaching anything to the assignment.
	if _, wrote := h.requireAssignmentInCourse(c, uint(assignmentID), uint(courseID)); wrote {
		return nil
	}

	var input struct {
		AssignmentOverride struct {
			Title           string     `json:"title"`
			DueAt           *time.Time `json:"due_at"`
			UnlockAt        *time.Time `json:"unlock_at"`
			LockAt          *time.Time `json:"lock_at"`
			AllDay          bool       `json:"all_day"`
			AllDayDate      *time.Time `json:"all_day_date"`
			CourseSectionID *uint      `json:"course_section_id"`
			StudentIDs      []uint     `json:"student_ids"`
		} `json:"assignment_override"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	override := &models.AssignmentOverride{
		AssignmentID:    uint(assignmentID),
		Title:           input.AssignmentOverride.Title,
		DueAt:           input.AssignmentOverride.DueAt,
		UnlockAt:        input.AssignmentOverride.UnlockAt,
		LockAt:          input.AssignmentOverride.LockAt,
		AllDay:          input.AssignmentOverride.AllDay,
		AllDayDate:      input.AssignmentOverride.AllDayDate,
		CourseSectionID: input.AssignmentOverride.CourseSectionID,
	}

	// The section must be one of this course's (F-013 parent-tie).
	if sid := input.AssignmentOverride.CourseSectionID; sid != nil && *sid != 0 {
		if !h.overrideService.SectionInCourse(c.Context(), uint(courseID), *sid) {
			return responses.NotFound(c, "section")
		}
	} else {
		override.CourseSectionID = nil
	}
	if override.CourseSectionID == nil && len(input.AssignmentOverride.StudentIDs) == 0 {
		return responses.BadRequest(c, "An override needs a section or at least one student")
	}

	// Every listed student must be a student of this course.
	studentIDs := dedupeUints(input.AssignmentOverride.StudentIDs)
	for _, studentID := range studentIDs {
		if !h.overrideService.IsCourseStudent(c.Context(), uint(courseID), studentID, callerAccountID(c)) {
			return responses.BadRequest(c, fmt.Sprintf("User %d is not a student in this course", studentID))
		}
	}

	if err := h.overrideService.CreateOverride(c.Context(), override); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	// B112: a failed student insert (e.g. the student already has an
	// override on this assignment) used to be swallowed while the
	// response still echoed the id as saved. Now the override is rolled
	// back and the conflict is reported.
	for _, studentID := range studentIDs {
		if err := h.overrideService.AddStudent(c.Context(), override.ID, studentID, uint(assignmentID)); err != nil {
			_ = h.overrideService.DeleteOverride(c.Context(), override.ID)
			return responses.Error(c, fiber.StatusConflict, fmt.Sprintf("User %d already has an override for this assignment", studentID))
		}
	}
	input.AssignmentOverride.StudentIDs = studentIDs
	h.refreshDueStatus(c, uint(courseID), uint(assignmentID))

	// Build response with student IDs
	overrideJSON := assignmentOverrideToJSON(override)
	overrideJSON["student_ids"] = input.AssignmentOverride.StudentIDs

	return c.Status(fiber.StatusCreated).JSON(overrideJSON)
}

func (h *AssignmentOverrideHandler) GetOverride(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	overrideID, err := c.ParamsInt("override_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid override ID")
	}

	override, err := h.overrideService.GetOverride(c.Context(), uint(overrideID))
	if err != nil {
		return responses.NotFound(c, "assignment override")
	}

	if !h.overrideInCourse(c, override, uint(assignmentID), uint(courseID)) {
		return responses.NotFound(c, "assignment override")
	}

	// Fetch student IDs
	students, _ := h.overrideService.ListStudents(c.Context(), override.ID)
	studentIDs := make([]uint, len(students))
	for i, s := range students {
		studentIDs[i] = s.UserID
	}

	overrideJSON := assignmentOverrideToJSON(override)
	overrideJSON["student_ids"] = studentIDs

	return c.JSON(overrideJSON)
}

func (h *AssignmentOverrideHandler) UpdateOverride(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	overrideID, err := c.ParamsInt("override_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid override ID")
	}

	override, err := h.overrideService.GetOverride(c.Context(), uint(overrideID))
	if err != nil {
		return responses.NotFound(c, "assignment override")
	}

	if !h.overrideInCourse(c, override, uint(assignmentID), uint(courseID)) {
		return responses.NotFound(c, "assignment override")
	}

	var input struct {
		AssignmentOverride struct {
			Title           *string    `json:"title"`
			DueAt           *time.Time `json:"due_at"`
			UnlockAt        *time.Time `json:"unlock_at"`
			LockAt          *time.Time `json:"lock_at"`
			AllDay          *bool      `json:"all_day"`
			AllDayDate      *time.Time `json:"all_day_date"`
			CourseSectionID *uint      `json:"course_section_id"`
			StudentIDs      *[]uint    `json:"student_ids"`
		} `json:"assignment_override"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	if sid := input.AssignmentOverride.CourseSectionID; sid != nil && *sid != 0 &&
		!h.overrideService.SectionInCourse(c.Context(), uint(courseID), *sid) {
		return responses.NotFound(c, "section")
	}
	var newStudents []uint
	if input.AssignmentOverride.StudentIDs != nil {
		newStudents = dedupeUints(*input.AssignmentOverride.StudentIDs)
		for _, studentID := range newStudents {
			if !h.overrideService.IsCourseStudent(c.Context(), uint(courseID), studentID, callerAccountID(c)) {
				return responses.BadRequest(c, fmt.Sprintf("User %d is not a student in this course", studentID))
			}
		}
	}

	if input.AssignmentOverride.Title != nil {
		override.Title = *input.AssignmentOverride.Title
	}
	if input.AssignmentOverride.DueAt != nil {
		override.DueAt = input.AssignmentOverride.DueAt
	}
	if input.AssignmentOverride.UnlockAt != nil {
		override.UnlockAt = input.AssignmentOverride.UnlockAt
	}
	if input.AssignmentOverride.LockAt != nil {
		override.LockAt = input.AssignmentOverride.LockAt
	}
	if input.AssignmentOverride.AllDay != nil {
		override.AllDay = *input.AssignmentOverride.AllDay
	}
	if input.AssignmentOverride.AllDayDate != nil {
		override.AllDayDate = input.AssignmentOverride.AllDayDate
	}
	if input.AssignmentOverride.CourseSectionID != nil {
		override.CourseSectionID = nonZeroUint(input.AssignmentOverride.CourseSectionID)
	}
	// An explicit null clears a date or the section (G019).
	var raw struct {
		AssignmentOverride map[string]json.RawMessage `json:"assignment_override"`
	}
	_ = json.Unmarshal(c.Body(), &raw)
	isNull := func(k string) bool {
		v, ok := raw.AssignmentOverride[k]
		return ok && strings.TrimSpace(string(v)) == "null"
	}
	if isNull("due_at") {
		override.DueAt = nil
	}
	if isNull("unlock_at") {
		override.UnlockAt = nil
	}
	if isNull("lock_at") {
		override.LockAt = nil
	}
	if isNull("course_section_id") {
		override.CourseSectionID = nil
	}

	if err := h.overrideService.UpdateOverride(c.Context(), override); err != nil {
		return responses.InternalError(c, "Could not update assignment override")
	}

	// G019: replace the ad-hoc student list when one was sent.
	if input.AssignmentOverride.StudentIDs != nil {
		current, _ := h.overrideService.ListStudents(c.Context(), override.ID)
		keep := map[uint]bool{}
		for _, id := range newStudents {
			keep[id] = true
		}
		have := map[uint]bool{}
		for _, st := range current {
			have[st.UserID] = true
			if !keep[st.UserID] {
				_ = h.overrideService.RemoveStudent(c.Context(), override.ID, st.UserID)
			}
		}
		for _, id := range newStudents {
			if have[id] {
				continue
			}
			if err := h.overrideService.AddStudent(c.Context(), override.ID, id, override.AssignmentID); err != nil {
				return responses.Error(c, fiber.StatusConflict, fmt.Sprintf("User %d already has an override for this assignment", id))
			}
		}
	}
	h.refreshDueStatus(c, uint(courseID), uint(assignmentID))

	// Fetch student IDs
	students, _ := h.overrideService.ListStudents(c.Context(), override.ID)
	studentIDs := make([]uint, len(students))
	for i, s := range students {
		studentIDs[i] = s.UserID
	}

	overrideJSON := assignmentOverrideToJSON(override)
	overrideJSON["student_ids"] = studentIDs

	return c.JSON(overrideJSON)
}

func (h *AssignmentOverrideHandler) DeleteOverride(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	overrideID, err := c.ParamsInt("override_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid override ID")
	}

	override, err := h.overrideService.GetOverride(c.Context(), uint(overrideID))
	if err != nil {
		return responses.NotFound(c, "assignment override")
	}

	if !h.overrideInCourse(c, override, uint(assignmentID), uint(courseID)) {
		return responses.NotFound(c, "assignment override")
	}

	if err := h.overrideService.DeleteOverride(c.Context(), uint(overrideID)); err != nil {
		return responses.InternalError(c, "Could not delete assignment override")
	}
	h.refreshDueStatus(c, uint(courseID), uint(assignmentID))

	return c.JSON(fiber.Map{"delete": true})
}

// dedupeUints returns ids with duplicates (and zeros) removed, preserving order.
func dedupeUints(ids []uint) []uint {
	seen := make(map[uint]bool, len(ids))
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
