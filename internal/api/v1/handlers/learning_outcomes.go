package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

type LearningOutcomeHandler struct {
	outcomeService *service.LearningOutcomeService
	alignmentRepo  repository.OutcomeAlignmentRepository
	// Wave 5.2: 13.5 LogPIIAccess sweep. Outcome groups/outcomes/
	// alignments are rubric metadata (not PII). The student-keyed
	// reads are ListResults (per-user outcome attainment) and
	// GetMasteryGradebook (per-student mastery rollups).
	auditService *service.AuditService
	// enrollmentRepo resolves an observer's linked students in a course
	// (B087). Optional: when nil, observers see no results.
	enrollmentRepo repository.EnrollmentRepository
	// assignmentRepo ties an alignment's assignment to the URL course.
	// Optional: when nil, CreateAlignment refuses (fail closed).
	assignmentRepo repository.AssignmentRepository
}

func NewLearningOutcomeHandler(outcomeService *service.LearningOutcomeService, alignmentRepo repository.OutcomeAlignmentRepository, auditService *service.AuditService) *LearningOutcomeHandler {
	return &LearningOutcomeHandler{outcomeService: outcomeService, alignmentRepo: alignmentRepo, auditService: auditService}
}

// SetEnrollmentRepo wires the enrollment lookup used to let an observer read
// their linked students' outcome results.
func (h *LearningOutcomeHandler) SetEnrollmentRepo(repo repository.EnrollmentRepository) {
	h.enrollmentRepo = repo
}

// SetAssignmentRepo wires the assignment lookup CreateAlignment uses to check
// that the aligned assignment belongs to the URL course.
func (h *LearningOutcomeHandler) SetAssignmentRepo(repo repository.AssignmentRepository) {
	h.assignmentRepo = repo
}

// courseGroup loads an outcome group under the caller's tenant and ties it
// to :course_id (F-013 parent tie). Any mismatch is reported as not found,
// so the caller answers 404 and never touches another course's group.
func (h *LearningOutcomeHandler) courseGroup(c *fiber.Ctx, courseID, groupID uint) (*models.LearningOutcomeGroup, bool) {
	group, err := h.outcomeService.GetGroup(c.Context(), groupID, callerAccountID(c))
	if err != nil || group == nil || group.ContextType != "Course" || group.ContextID != courseID {
		return nil, false
	}
	return group, true
}

// courseOutcome is courseGroup for outcomes.
func (h *LearningOutcomeHandler) courseOutcome(c *fiber.Ctx, courseID, outcomeID uint) (*models.LearningOutcome, bool) {
	outcome, err := h.outcomeService.GetOutcome(c.Context(), outcomeID, callerAccountID(c))
	if err != nil || outcome == nil || outcome.ContextType != "Course" || outcome.ContextID != courseID {
		return nil, false
	}
	return outcome, true
}

// outcomeResultAudience reports which students' outcome results the caller
// may read in the course. staff=true (teacher/TA/admin) means every student.
// Otherwise the caller sees only their own results, plus — for an observer —
// the students they are linked to in this course (Canvas outcome_results /
// outcome_rollups restrict students to themselves; B087).
func (h *LearningOutcomeHandler) outcomeResultAudience(c *fiber.Ctx, courseID uint) (allowed []uint, staff bool) {
	if callerIsCourseStaff(c) {
		return nil, true
	}
	callerID, _ := c.Locals("user_id").(uint)
	if callerID == 0 {
		return nil, false
	}
	allowed = []uint{callerID}
	if et, _ := c.Locals("enrollment_type").(string); et == "ObserverEnrollment" && h.enrollmentRepo != nil {
		enrollments, err := h.enrollmentRepo.ListByUserID(c.Context(), callerID, callerAccountID(c))
		if err == nil {
			seen := map[uint]bool{callerID: true}
			for _, e := range enrollments {
				if e.CourseID != courseID || e.Type != "ObserverEnrollment" || e.WorkflowState != models.EnrollmentActive || e.AssociatedUserID == nil {
					continue
				}
				if !seen[*e.AssociatedUserID] {
					seen[*e.AssociatedUserID] = true
					allowed = append(allowed, *e.AssociatedUserID)
				}
			}
		}
	}
	return allowed, false
}

func containsUint(ids []uint, id uint) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func outcomeGroupToJSON(g *models.LearningOutcomeGroup) fiber.Map {
	return fiber.Map{
		"id":                      g.ID,
		"context_type":            g.ContextType,
		"context_id":              g.ContextID,
		"parent_outcome_group_id": g.ParentGroupID,
		"title":                   g.Title,
		"description":             g.Description,
		"workflow_state":          g.WorkflowState,
		"created_at":              g.CreatedAt,
		"updated_at":              g.UpdatedAt,
	}
}

func outcomeToJSON(o *models.LearningOutcome) fiber.Map {
	return fiber.Map{
		"id":                 o.ID,
		"context_type":       o.ContextType,
		"context_id":         o.ContextID,
		"outcome_group_id":   o.OutcomeGroupID,
		"title":              o.Title,
		"display_name":       o.DisplayName,
		"description":        o.Description,
		"calculation_method": o.CalculationMethod,
		"calculation_int":    o.CalculationInt,
		"mastery_points":     o.MasteryPoints,
		"points_possible":    o.PointsPossible,
		"ratings":            o.RatingsData,
		"workflow_state":     o.WorkflowState,
		"created_at":         o.CreatedAt,
		"updated_at":         o.UpdatedAt,
	}
}

func outcomeResultToJSON(r *models.LearningOutcomeResult) fiber.Map {
	return fiber.Map{
		"id":                    r.ID,
		"user_id":               r.UserID,
		"learning_outcome_id":   r.LearningOutcomeID,
		"context_type":          r.ContextType,
		"context_id":            r.ContextID,
		"associated_asset_type": r.AssociatedAssetType,
		"associated_asset_id":   r.AssociatedAssetID,
		"score":                 r.Score,
		"possible":              r.Possible,
		"mastery":               r.Mastery,
		"percent":               r.Percent,
		"attempt":               r.Attempt,
		"assessed_at":           r.AssessedAt,
		"submitted_at":          r.SubmittedAt,
		"title":                 r.Title,
		"created_at":            r.CreatedAt,
		"updated_at":            r.UpdatedAt,
	}
}

// Outcome Group endpoints

func (h *LearningOutcomeHandler) ListGroups(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.outcomeService.ListGroups(c.Context(), "Course", uint(courseID), callerAccountID(c), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch outcome groups")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	groups := make([]fiber.Map, len(result.Items))
	for i, g := range result.Items {
		groups[i] = outcomeGroupToJSON(&g)
	}

	return c.JSON(groups)
}

func (h *LearningOutcomeHandler) CreateGroup(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		Title         string `json:"title"`
		Description   string `json:"description"`
		ParentGroupID *uint  `json:"parent_outcome_group_id"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	group := &models.LearningOutcomeGroup{
		ContextType:   "Course",
		ContextID:     uint(courseID),
		Title:         input.Title,
		Description:   service.SanitizePlainText(input.Description),
		ParentGroupID: input.ParentGroupID,
	}

	if err := h.outcomeService.CreateGroup(c.Context(), group); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(outcomeGroupToJSON(group))
}

func (h *LearningOutcomeHandler) GetGroup(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	groupID, err := c.ParamsInt("group_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid group ID")
	}

	group, ok := h.courseGroup(c, uint(courseID), uint(groupID))
	if !ok {
		return responses.NotFound(c, "outcome group")
	}

	return c.JSON(outcomeGroupToJSON(group))
}

func (h *LearningOutcomeHandler) UpdateGroup(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	groupID, err := c.ParamsInt("group_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid group ID")
	}

	// F-013 parent tie: a teacher of this course must not edit another
	// course's outcome group by putting its id in the URL.
	group, ok := h.courseGroup(c, uint(courseID), uint(groupID))
	if !ok {
		return responses.NotFound(c, "outcome group")
	}

	var input struct {
		Title         *string `json:"title"`
		Description   *string `json:"description"`
		ParentGroupID *uint   `json:"parent_outcome_group_id"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Title != nil {
		group.Title = *input.Title
	}
	if input.Description != nil {
		group.Description = service.SanitizePlainText(*input.Description)
	}
	if input.ParentGroupID != nil {
		if *input.ParentGroupID != 0 {
			if _, ok := h.courseGroup(c, uint(courseID), *input.ParentGroupID); !ok || *input.ParentGroupID == group.ID {
				return responses.BadRequest(c, "parent_outcome_group_id must be another group in this course")
			}
		}
		group.ParentGroupID = input.ParentGroupID
	}

	if err := h.outcomeService.UpdateGroup(c.Context(), group); err != nil {
		return responses.InternalError(c, "Could not update outcome group")
	}

	return c.JSON(outcomeGroupToJSON(group))
}

func (h *LearningOutcomeHandler) DeleteGroup(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	groupID, err := c.ParamsInt("group_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid group ID")
	}

	// SECURITY (F-011 / F-012): load the group under the caller's
	// tenant AND verify it belongs to the URL's course. Pre-fix path
	// called DeleteGroup(id) with no scope, so a teacher in course 10
	// could DELETE /courses/10/outcome_groups/<id from any other
	// course/tenant>. Account-context groups intentionally cannot be
	// deleted through a course-scoped route.
	group, err := h.outcomeService.GetGroup(c.Context(), uint(groupID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "outcome group")
	}
	if group.ContextType != "Course" || group.ContextID != uint(courseID) {
		return responses.NotFound(c, "outcome group")
	}

	if err := h.outcomeService.DeleteGroup(c.Context(), uint(groupID), callerAccountID(c)); err != nil {
		return responses.InternalError(c, "Could not delete outcome group")
	}

	return c.JSON(fiber.Map{"delete": true})
}

// Outcome endpoints

func (h *LearningOutcomeHandler) ListOutcomes(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	groupID, err := c.ParamsInt("group_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid group ID")
	}
	if _, ok := h.courseGroup(c, uint(courseID), uint(groupID)); !ok {
		return responses.NotFound(c, "outcome group")
	}

	params := middleware.GetPagination(c)

	result, err := h.outcomeService.ListOutcomes(c.Context(), uint(groupID), callerAccountID(c), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch outcomes")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	outcomes := make([]fiber.Map, len(result.Items))
	for i, o := range result.Items {
		outcomes[i] = outcomeToJSON(&o)
	}

	return c.JSON(outcomes)
}

func (h *LearningOutcomeHandler) CreateOutcome(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	groupID, err := c.ParamsInt("group_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid group ID")
	}
	// F-013 parent tie: the new outcome may only go into a group of this
	// course, not into another course's group.
	if _, ok := h.courseGroup(c, uint(courseID), uint(groupID)); !ok {
		return responses.NotFound(c, "outcome group")
	}

	var input struct {
		Title             string  `json:"title"`
		DisplayName       string  `json:"display_name"`
		Description       string  `json:"description"`
		CalculationMethod string  `json:"calculation_method"`
		CalculationInt    int     `json:"calculation_int"`
		MasteryPoints     float64 `json:"mastery_points"`
		PointsPossible    float64 `json:"points_possible"`
		Ratings           string  `json:"ratings"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	outcome := &models.LearningOutcome{
		ContextType:       "Course",
		ContextID:         uint(courseID),
		OutcomeGroupID:    uint(groupID),
		Title:             input.Title,
		DisplayName:       input.DisplayName,
		Description:       service.SanitizePlainText(input.Description),
		CalculationMethod: input.CalculationMethod,
		CalculationInt:    input.CalculationInt,
		MasteryPoints:     input.MasteryPoints,
		PointsPossible:    input.PointsPossible,
		RatingsData:       input.Ratings,
	}

	if err := h.outcomeService.CreateOutcome(c.Context(), outcome); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(outcomeToJSON(outcome))
}

func (h *LearningOutcomeHandler) GetOutcome(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	outcomeID, err := c.ParamsInt("outcome_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid outcome ID")
	}

	outcome, ok := h.courseOutcome(c, uint(courseID), uint(outcomeID))
	if !ok {
		return responses.NotFound(c, "outcome")
	}

	return c.JSON(outcomeToJSON(outcome))
}

func (h *LearningOutcomeHandler) UpdateOutcome(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	outcomeID, err := c.ParamsInt("outcome_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid outcome ID")
	}

	// F-013 parent tie: a teacher of this course must not rewrite another
	// course's outcome (its mastery threshold, scale or calculation).
	outcome, ok := h.courseOutcome(c, uint(courseID), uint(outcomeID))
	if !ok {
		return responses.NotFound(c, "outcome")
	}

	var input struct {
		Title             *string  `json:"title"`
		DisplayName       *string  `json:"display_name"`
		Description       *string  `json:"description"`
		CalculationMethod *string  `json:"calculation_method"`
		CalculationInt    *int     `json:"calculation_int"`
		MasteryPoints     *float64 `json:"mastery_points"`
		PointsPossible    *float64 `json:"points_possible"`
		Ratings           *string  `json:"ratings"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Title != nil {
		outcome.Title = *input.Title
	}
	if input.DisplayName != nil {
		outcome.DisplayName = *input.DisplayName
	}
	if input.Description != nil {
		outcome.Description = service.SanitizePlainText(*input.Description)
	}
	if input.CalculationMethod != nil {
		outcome.CalculationMethod = *input.CalculationMethod
	}
	if input.CalculationInt != nil {
		outcome.CalculationInt = *input.CalculationInt
	}
	if input.MasteryPoints != nil {
		outcome.MasteryPoints = *input.MasteryPoints
	}
	if input.PointsPossible != nil {
		outcome.PointsPossible = *input.PointsPossible
	}
	if input.Ratings != nil {
		outcome.RatingsData = *input.Ratings
		if input.PointsPossible == nil {
			// Like Canvas, points_possible follows the highest rating:
			// re-derive it from the new ratings (B090 scales results onto
			// it, so a stale value would mis-scale every result).
			outcome.PointsPossible = 0
		}
	}

	if err := h.outcomeService.UpdateOutcome(c.Context(), outcome); err != nil {
		return responses.InternalError(c, "Could not update outcome")
	}

	return c.JSON(outcomeToJSON(outcome))
}

func (h *LearningOutcomeHandler) DeleteOutcome(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	outcomeID, err := c.ParamsInt("outcome_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid outcome ID")
	}

	// SECURITY (F-011 / F-012): load the outcome under the caller's
	// tenant AND verify it belongs to the URL's course. Pre-fix path
	// called DeleteOutcome(id) with no scope, so a teacher in course
	// 10 could DELETE /courses/10/outcomes/<id from any other course/
	// tenant>. Account-context outcomes intentionally cannot be deleted
	// through a course-scoped route.
	outcome, err := h.outcomeService.GetOutcome(c.Context(), uint(outcomeID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "outcome")
	}
	if outcome.ContextType != "Course" || outcome.ContextID != uint(courseID) {
		return responses.NotFound(c, "outcome")
	}

	if err := h.outcomeService.DeleteOutcome(c.Context(), uint(outcomeID), callerAccountID(c)); err != nil {
		return responses.InternalError(c, "Could not delete outcome")
	}

	return c.JSON(fiber.Map{"delete": true})
}

// Result endpoints

func (h *LearningOutcomeHandler) ListResults(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	// SECURITY (B087): the route is `enrolled`, so a student could read
	// every classmate's outcome scores (no user_id) or any one classmate's
	// (user_id=<classmate>). Only course staff see other students; students
	// see themselves and observers their linked students. A disallowed
	// user_id is a 404, matching the existence-leak contract.
	allowed, staff := h.outcomeResultAudience(c, uint(courseID))

	userID := c.QueryInt("user_id", 0)
	if userID > 0 && !staff && !containsUint(allowed, uint(userID)) {
		return responses.NotFound(c, "outcome results")
	}
	if userID == 0 && !staff {
		items := []fiber.Map{}
		for _, uid := range allowed {
			results, err := h.outcomeService.ListResultsByUserAndContext(c.Context(), uid, "Course", uint(courseID))
			if err != nil {
				return responses.InternalError(c, "Could not fetch outcome results")
			}
			for i := range results {
				items = append(items, outcomeResultToJSON(&results[i]))
			}
		}
		return c.JSON(fiber.Map{"outcome_results": items})
	}
	if userID > 0 {
		results, err := h.outcomeService.ListResultsByUserAndContext(c.Context(), uint(userID), "Course", uint(courseID))
		if err != nil {
			return responses.InternalError(c, "Could not fetch outcome results")
		}

		items := make([]fiber.Map, len(results))
		for i, r := range results {
			items[i] = outcomeResultToJSON(&r)
		}

		// Wave 5.2 FERPA audit: reading another student's outcome
		// mastery results is a per-student academic-record read.
		if callerID, _ := getUserID(c); callerID != 0 && uint(userID) != callerID && h.auditService != nil {
			_ = h.auditService.LogPIIAccess(c.Context(), callerID, uint(userID), "read", "outcome_results", "courses", uint(courseID), c.IP(), c.Get("User-Agent"))
		}

		return c.JSON(fiber.Map{
			"outcome_results": items,
		})
	}

	// If no user_id, list all results for all outcomes in this course
	params := middleware.GetPagination(c)

	// Get all outcomes for the course first
	outcomes, err := h.outcomeService.ListOutcomesByContext(c.Context(), "Course", uint(courseID), callerAccountID(c), repository.PaginationParams{Page: 1, PerPage: 1000})
	if err != nil {
		return responses.InternalError(c, "Could not fetch outcomes")
	}

	var allResults []fiber.Map
	for _, outcome := range outcomes.Items {
		results, err := h.outcomeService.ListResultsByOutcome(c.Context(), outcome.ID, params)
		if err != nil {
			continue
		}
		for _, r := range results.Items {
			allResults = append(allResults, outcomeResultToJSON(&r))
		}
	}

	// Wave 5.2 FERPA audit: bulk read across every student's outcome
	// results in this course (instructor view). studentID=0 follows
	// the bulk-read convention.
	if callerID, _ := getUserID(c); callerID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, 0, "read", "bulk_outcome_results", "courses", uint(courseID), c.IP(), c.Get("User-Agent"))
	}

	return c.JSON(fiber.Map{
		"outcome_results": allResults,
	})
}

func (h *LearningOutcomeHandler) CreateResult(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		UserID              uint     `json:"user_id"`
		LearningOutcomeID   uint     `json:"learning_outcome_id"`
		AssociatedAssetType string   `json:"associated_asset_type"`
		AssociatedAssetID   uint     `json:"associated_asset_id"`
		Score               *float64 `json:"score"`
		Possible            *float64 `json:"possible"`
		Attempt             int      `json:"attempt"`
		Title               string   `json:"title"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	// F-013 parent tie: results may only be recorded against an outcome of
	// this course (otherwise a teacher could set mastery, and fire mastery
	// events, on another course's outcome).
	if input.LearningOutcomeID != 0 {
		if _, ok := h.courseOutcome(c, uint(courseID), input.LearningOutcomeID); !ok {
			return responses.NotFound(c, "outcome")
		}
	}

	result := &models.LearningOutcomeResult{
		UserID:              input.UserID,
		LearningOutcomeID:   input.LearningOutcomeID,
		ContextType:         "Course",
		ContextID:           uint(courseID),
		AssociatedAssetType: input.AssociatedAssetType,
		AssociatedAssetID:   input.AssociatedAssetID,
		Score:               input.Score,
		Possible:            input.Possible,
		Attempt:             input.Attempt,
		Title:               input.Title,
	}

	if err := h.outcomeService.CreateResult(c.Context(), result); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(outcomeResultToJSON(result))
}

// Mastery Gradebook endpoint

func (h *LearningOutcomeHandler) GetMasteryGradebook(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	// SECURITY (B087): non-staff callers get only their own rollups (or,
	// for observers, their linked students'), never the whole class.
	allowed, staff := h.outcomeResultAudience(c, uint(courseID))
	var (
		rollups  []service.MasteryRollup
		outcomes []models.LearningOutcome
	)
	if staff {
		rollups, outcomes, err = h.outcomeService.GetMasteryGradebook(c.Context(), uint(courseID), callerAccountID(c), params)
	} else {
		rollups, outcomes, err = h.outcomeService.GetMasteryGradebookForUsers(c.Context(), uint(courseID), callerAccountID(c), allowed, params)
	}
	if err != nil {
		return responses.InternalError(c, "Could not fetch mastery gradebook")
	}

	// Format outcomes for the response
	outcomeList := make([]fiber.Map, len(outcomes))
	for i, o := range outcomes {
		outcomeList[i] = outcomeToJSON(&o)
	}

	// Format rollups for the response
	rollupList := make([]fiber.Map, len(rollups))
	for i, r := range rollups {
		scores := make([]fiber.Map, len(r.Scores))
		for j, s := range r.Scores {
			scores[j] = fiber.Map{
				"outcome_id": s.OutcomeID,
				"score":      s.Score,
				"count":      s.Count,
				"mastery":    s.Mastery,
				"title":      s.Title,
			}
		}
		rollupList[i] = fiber.Map{
			"links": fiber.Map{
				"user": r.StudentID,
			},
			"scores": scores,
		}
	}

	// Wave 5.2 FERPA audit: mastery gradebook returns per-student
	// rollups across every outcome — bulk read of academic-record
	// data. studentID=0 per the bulk convention.
	if callerID, _ := getUserID(c); callerID != 0 && h.auditService != nil {
		if staff {
			_ = h.auditService.LogPIIAccess(c.Context(), callerID, 0, "read", "mastery_gradebook", "courses", uint(courseID), c.IP(), c.Get("User-Agent"))
		} else {
			// Observer reading a linked student's rollups.
			for _, uid := range allowed {
				if uid != callerID {
					_ = h.auditService.LogPIIAccess(c.Context(), callerID, uid, "read", "mastery_gradebook", "courses", uint(courseID), c.IP(), c.Get("User-Agent"))
				}
			}
		}
	}

	return c.JSON(fiber.Map{
		"rollups":         rollupList,
		"linked": fiber.Map{
			"outcomes": outcomeList,
		},
	})
}

// Alignment endpoints

func (h *LearningOutcomeHandler) CreateAlignment(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		LearningOutcomeID uint `json:"learning_outcome_id"`
		AssignmentID      uint `json:"assignment_id"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.LearningOutcomeID == 0 || input.AssignmentID == 0 {
		return responses.BadRequest(c, "learning_outcome_id and assignment_id are required")
	}

	// F-013 parent tie: both the outcome and the assignment must belong to
	// this course. Grading an aligned assignment copies the grade into an
	// outcome result in the alignment's course, so aligning another
	// course's assignment would copy that course's students' grades here.
	if _, ok := h.courseOutcome(c, uint(courseID), input.LearningOutcomeID); !ok {
		return responses.NotFound(c, "outcome")
	}
	if h.assignmentRepo == nil {
		return responses.InternalError(c, "Could not verify the assignment")
	}
	if a, err := h.assignmentRepo.FindByID(c.Context(), input.AssignmentID, callerAccountID(c)); err != nil || a == nil || a.CourseID != uint(courseID) {
		return responses.NotFound(c, "assignment")
	}

	alignment := &models.OutcomeAlignment{
		LearningOutcomeID: input.LearningOutcomeID,
		AssignmentID:      input.AssignmentID,
		CourseID:          uint(courseID),
	}

	if err := h.alignmentRepo.Create(c.Context(), alignment); err != nil {
		return responses.InternalError(c, "Could not create alignment")
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":                  alignment.ID,
		"learning_outcome_id": alignment.LearningOutcomeID,
		"assignment_id":       alignment.AssignmentID,
		"course_id":           alignment.CourseID,
	})
}

func (h *LearningOutcomeHandler) DeleteAlignment(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	alignmentID, err := c.ParamsInt("alignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid alignment ID")
	}

	// F-013 parent tie: Delete(id) has no tenant or course scope, so only
	// delete an alignment that is listed for this course in the caller's
	// tenant; anything else is a 404.
	alignments, err := h.alignmentRepo.ListByCourseID(c.Context(), uint(courseID), callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not fetch alignments")
	}
	found := false
	for _, a := range alignments {
		if a.ID == uint(alignmentID) && a.CourseID == uint(courseID) {
			found = true
			break
		}
	}
	if !found {
		return responses.NotFound(c, "alignment")
	}

	if err := h.alignmentRepo.Delete(c.Context(), uint(alignmentID)); err != nil {
		return responses.InternalError(c, "Could not delete alignment")
	}

	return c.JSON(fiber.Map{"delete": true})
}

func (h *LearningOutcomeHandler) ListAlignments(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	assignmentID := c.QueryInt("assignment_id", 0)
	var alignments []models.OutcomeAlignment

	if assignmentID > 0 {
		alignments, err = h.alignmentRepo.ListByAssignmentID(c.Context(), uint(assignmentID), callerAccountID(c))
	} else {
		alignments, err = h.alignmentRepo.ListByCourseID(c.Context(), uint(courseID), callerAccountID(c))
	}
	if err != nil {
		return responses.InternalError(c, "Could not fetch alignments")
	}

	result := make([]fiber.Map, len(alignments))
	for i, a := range alignments {
		result[i] = fiber.Map{
			"id":                  a.ID,
			"learning_outcome_id": a.LearningOutcomeID,
			"assignment_id":       a.AssignmentID,
			"course_id":           a.CourseID,
		}
	}

	return c.JSON(result)
}
