package handlers

// Grading-policy endpoints (grading-policy lane): server-computed course
// grades, effective due dates, final grade overrides, course post
// policy, grading scheme selection and account-level schemes, late
// policy re-application, and submission history.

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

type GradingPolicyHandler struct {
	policy         *service.GradingPolicyService
	courseRepo     repository.CourseRepository
	assignmentRepo repository.AssignmentRepository
	submissionRepo repository.SubmissionRepository
	standards      repository.GradingStandardRepository
	observers      *service.ObserverService
}

func NewGradingPolicyHandler(policy *service.GradingPolicyService, courseRepo repository.CourseRepository, assignmentRepo repository.AssignmentRepository,
	submissionRepo repository.SubmissionRepository, standards repository.GradingStandardRepository, observers *service.ObserverService) *GradingPolicyHandler {
	return &GradingPolicyHandler{policy: policy, courseRepo: courseRepo, assignmentRepo: assignmentRepo,
		submissionRepo: submissionRepo, standards: standards, observers: observers}
}

// loadCourse loads :course_id under the caller's tenant. On a miss it
// writes a 404 and returns wrote=true.
func (h *GradingPolicyHandler) loadCourse(c *fiber.Ctx) (*models.Course, bool) {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		_ = responses.BadRequest(c, "Invalid course ID")
		return nil, true
	}
	course, err := h.courseRepo.FindByID(c.Context(), uint(courseID), callerAccountID(c))
	if err != nil || course == nil {
		_ = responses.NotFound(c, "course")
		return nil, true
	}
	return course, false
}

// callerMayViewStudent: the student themself or a linked observer.
func (h *GradingPolicyHandler) callerMayViewStudent(c *fiber.Ctx, studentID uint) bool {
	callerID, _ := c.Locals("user_id").(uint)
	if callerID != 0 && callerID == studentID {
		return true
	}
	if h.observers != nil {
		if ok, _ := h.observers.IsObserverOf(c.Context(), callerID, studentID); ok {
			return true
		}
	}
	return false
}

func parseUintList(raw string) []uint {
	var out []uint
	for _, part := range strings.Split(raw, ",") {
		if n, err := strconv.ParseUint(strings.TrimSpace(part), 10, 64); err == nil && n > 0 {
			out = append(out, uint(n))
		}
	}
	return out
}

// CourseGrades handles GET /courses/:course_id/grades. Staff get every
// student (or ?user_ids=1,2); a student gets their own posted grades; an
// observer passes ?user_id=<observee>.
func (h *GradingPolicyHandler) CourseGrades(c *fiber.Ctx) error {
	course, wrote := h.loadCourse(c)
	if wrote {
		return nil
	}
	opts := service.CourseGradesOptions{}
	if callerIsCourseStaff(c) {
		opts.UserIDs = parseUintList(c.Query("user_ids"))
		if uid := c.QueryInt("user_id"); uid > 0 {
			opts.UserIDs = []uint{uint(uid)}
		}
	} else {
		callerID, _ := c.Locals("user_id").(uint)
		target := callerID
		if uid := c.QueryInt("user_id"); uid > 0 {
			target = uint(uid)
		}
		if target == 0 || !h.callerMayViewStudent(c, target) {
			return responses.NotFound(c, "student")
		}
		opts.UserIDs = []uint{target}
		opts.PostedOnly = true
	}
	grades, err := h.policy.CourseGrades(c.Context(), course, opts)
	if err != nil {
		return responses.InternalError(c, "Could not calculate grades")
	}
	if opts.PostedOnly {
		// Final grade overrides are part of the grade a student sees;
		// per-period overrides too. Nothing else to strip.
		for i := range grades.Students {
			grades.Students[i].Unassigned = nil
		}
	}
	return c.JSON(grades)
}

// EffectiveDueDates handles GET /courses/:course_id/effective_due_dates
// (staff): assignment id -> user id -> dates and grading period.
func (h *GradingPolicyHandler) EffectiveDueDates(c *fiber.Ctx) error {
	course, wrote := h.loadCourse(c)
	if wrote {
		return nil
	}
	dd, err := h.policy.DueDates(c.Context(), course, parseUintList(c.Query("user_ids")))
	if err != nil {
		return responses.InternalError(c, "Could not resolve due dates")
	}
	only := map[uint]bool{}
	for _, id := range parseUintList(c.Query("assignment_ids")) {
		only[id] = true
	}
	out := map[string]map[string]service.EffectiveDueDate{}
	for aid, per := range dd {
		if len(only) > 0 && !only[aid] {
			continue
		}
		m := map[string]service.EffectiveDueDate{}
		for uid, d := range per {
			m[strconv.FormatUint(uint64(uid), 10)] = d
		}
		out[strconv.FormatUint(uint64(aid), 10)] = m
	}
	return c.JSON(out)
}

// MyAssignmentDates handles GET /courses/:course_id/assignment_dates: the
// caller's (or an observee's, ?user_id=) own effective dates per
// assignment, so the student UI shows override / accommodation dates.
// Assignments not assigned to the student are absent.
func (h *GradingPolicyHandler) MyAssignmentDates(c *fiber.Ctx) error {
	course, wrote := h.loadCourse(c)
	if wrote {
		return nil
	}
	callerID, _ := c.Locals("user_id").(uint)
	target := callerID
	if uid := c.QueryInt("user_id"); uid > 0 {
		target = uint(uid)
	}
	if target == 0 || (!callerIsCourseStaff(c) && !h.callerMayViewStudent(c, target)) {
		return responses.NotFound(c, "student")
	}
	dates, err := h.policy.StudentAssignmentDates(c.Context(), course, target)
	if err != nil {
		return responses.InternalError(c, "Could not resolve due dates")
	}
	out := map[string]service.EffectiveDueDate{}
	for aid, d := range dates {
		out[strconv.FormatUint(uint64(aid), 10)] = d
	}
	return c.JSON(fiber.Map{"user_id": target, "assignments": out})
}

// SetScoreOverride handles PUT /courses/:course_id/students/:user_id/score_override.
// Body: {"override_score": 91.5} (percent) or {"override_grade": "A-"};
// null / "" clears. "grading_period_id" scopes it to one period.
func (h *GradingPolicyHandler) SetScoreOverride(c *fiber.Ctx) error {
	course, wrote := h.loadCourse(c)
	if wrote {
		return nil
	}
	userID, err := c.ParamsInt("user_id")
	if err != nil || userID <= 0 {
		return responses.BadRequest(c, "Invalid user ID")
	}
	actorID, err := getUserID(c)
	if err != nil {
		return err
	}
	var input struct {
		OverrideScore   json.RawMessage `json:"override_score"`
		OverrideGrade   *string         `json:"override_grade"`
		GradingPeriodID uint            `json:"grading_period_id"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	var score *float64
	switch {
	case input.OverrideGrade != nil && strings.TrimSpace(*input.OverrideGrade) != "":
		g := strings.TrimSpace(*input.OverrideGrade)
		if v, perr := strconv.ParseFloat(strings.TrimSuffix(g, "%"), 64); perr == nil {
			score = &v
		} else {
			scheme := h.policy.CourseScheme(c.Context(), course)
			if scheme == nil {
				scheme = service.DefaultGradingScheme()
			}
			p, ok := scheme.GradeToPercent(g)
			if !ok {
				return responses.BadRequest(c, "override_grade is not a grade in the course grading scheme")
			}
			score = &p
		}
	case len(input.OverrideScore) > 0 && string(input.OverrideScore) != "null":
		var v float64
		if err := json.Unmarshal(input.OverrideScore, &v); err != nil {
			var s string
			if json.Unmarshal(input.OverrideScore, &s) != nil {
				return responses.BadRequest(c, "override_score must be a number")
			}
			s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "%"))
			if s != "" {
				if v, err = strconv.ParseFloat(s, 64); err != nil {
					return responses.BadRequest(c, "override_score must be a number")
				}
				score = &v
			}
		} else {
			score = &v
		}
	}
	o, err := h.policy.SetScoreOverride(c.Context(), course, uint(userID), input.GradingPeriodID, actorID, score)
	switch {
	case errors.Is(err, service.ErrNotCourseStudent):
		return responses.NotFound(c, "student")
	case errors.Is(err, service.ErrUnknownGradingPeriod):
		return responses.NotFound(c, "grading period")
	case err != nil:
		return responses.BadRequest(c, err.Error())
	}
	resp := fiber.Map{"course_id": course.ID, "user_id": userID, "grading_period_id": input.GradingPeriodID, "override_score": nil, "override_grade": nil}
	if o != nil {
		resp["override_score"] = o.OverrideScore
		if scheme := h.policy.CourseScheme(c.Context(), course); scheme != nil {
			resp["override_grade"] = scheme.ScoreToGrade(o.OverrideScore)
		}
	}
	return c.JSON(resp)
}

// GetPostPolicy handles GET /courses/:course_id/post_policy.
func (h *GradingPolicyHandler) GetPostPolicy(c *fiber.Ctx) error {
	course, wrote := h.loadCourse(c)
	if wrote {
		return nil
	}
	return c.JSON(fiber.Map{"course_id": course.ID, "post_manually": course.PostManually})
}

// SetPostPolicy handles PUT /courses/:course_id/post_policy
// {"post_manually": true}: the course default, applied to its
// assignments (anonymous assignments always post manually).
func (h *GradingPolicyHandler) SetPostPolicy(c *fiber.Ctx) error {
	course, wrote := h.loadCourse(c)
	if wrote {
		return nil
	}
	var input struct {
		PostManually *bool `json:"post_manually"`
	}
	if err := c.BodyParser(&input); err != nil || input.PostManually == nil {
		return responses.BadRequest(c, "post_manually is required")
	}
	if err := h.policy.SetCoursePostPolicy(c.Context(), course, *input.PostManually); err != nil {
		return responses.InternalError(c, "Could not update the post policy")
	}
	return c.JSON(fiber.Map{"course_id": course.ID, "post_manually": *input.PostManually})
}

func gradingStandardJSON(gs *models.GradingStandard) fiber.Map {
	return gradingStandardToJSON(gs)
}

// ListCourseSchemes handles GET /courses/:course_id/grading_schemes: the
// schemes the course may use (its own and its account's), the selected
// one and whether letter grades are on.
func (h *GradingPolicyHandler) ListCourseSchemes(c *fiber.Ctx) error {
	course, wrote := h.loadCourse(c)
	if wrote {
		return nil
	}
	courseSchemes, err := h.standards.ListByCourse(c.Context(), course.ID)
	if err != nil {
		return responses.InternalError(c, "Could not fetch grading schemes")
	}
	accountSchemes, err := h.standards.ListByAccount(c.Context(), course.AccountID)
	if err != nil {
		return responses.InternalError(c, "Could not fetch grading schemes")
	}
	schemes := make([]fiber.Map, 0, len(courseSchemes)+len(accountSchemes))
	for i := range courseSchemes {
		schemes = append(schemes, gradingStandardJSON(&courseSchemes[i]))
	}
	for i := range accountSchemes {
		schemes = append(schemes, gradingStandardJSON(&accountSchemes[i]))
	}
	resolved := service.ResolveGradingScheme(c.Context(), h.standards, course, nil)
	return c.JSON(fiber.Map{
		"grading_standard_id":           course.GradingStandardID,
		"grading_standard_enabled":      !course.GradingStandardDisabled,
		"effective_grading_scheme":      service.SchemeInfoFor(resolved),
		"effective_grading_standard_id": resolved.ID,
		"grading_schemes":               schemes,
	})
}

// SetCourseScheme handles PUT /courses/:course_id/grading_scheme
// {"grading_standard_id": 5 | null, "grading_standard_enabled": true}.
func (h *GradingPolicyHandler) SetCourseScheme(c *fiber.Ctx) error {
	course, wrote := h.loadCourse(c)
	if wrote {
		return nil
	}
	var input struct {
		GradingStandardID json.RawMessage `json:"grading_standard_id"`
		Enabled           *bool           `json:"grading_standard_enabled"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	if len(input.GradingStandardID) > 0 {
		raw := strings.TrimSpace(string(input.GradingStandardID))
		if raw == "null" || raw == `""` || raw == "0" {
			course.GradingStandardID = nil
		} else {
			var id uint
			if err := json.Unmarshal(input.GradingStandardID, &id); err != nil || id == 0 {
				return responses.BadRequest(c, "grading_standard_id must be a scheme id or null")
			}
			gs, err := h.standards.FindByID(c.Context(), id)
			if err != nil || gs == nil || gs.WorkflowState == "deleted" || !service.StandardVisibleToCourse(gs, course) {
				return responses.NotFound(c, "grading scheme")
			}
			course.GradingStandardID = &id
		}
	}
	if input.Enabled != nil {
		course.GradingStandardDisabled = !*input.Enabled
	}
	if err := h.policy.Repo().SetCourseGradingScheme(c.Context(), course.ID, course.GradingStandardID, course.GradingStandardDisabled); err != nil {
		return responses.InternalError(c, "Could not update the grading scheme")
	}
	return c.JSON(fiber.Map{
		"grading_standard_id":      course.GradingStandardID,
		"grading_standard_enabled": !course.GradingStandardDisabled,
	})
}

// ApplyLatePolicy handles POST /courses/:course_id/late_policy/apply:
// re-evaluates late / missing statuses and applies the missing-submission
// policy to every past-due assignment now (the background job does the
// same periodically).
func (h *GradingPolicyHandler) ApplyLatePolicy(c *fiber.Ctx) error {
	course, wrote := h.loadCourse(c)
	if wrote {
		return nil
	}
	n, err := h.policy.ApplyCourseMissingPolicy(c.Context(), course)
	if err != nil {
		return responses.InternalError(c, "Could not apply the late policy")
	}
	return c.JSON(fiber.Map{"updated": n})
}

// SubmissionHistory handles
// GET /courses/:course_id/assignments/:assignment_id/submissions/:user_id/history:
// every attempt of the submission. Students and observers see grades of
// posted submissions only.
func (h *GradingPolicyHandler) SubmissionHistory(c *fiber.Ctx) error {
	course, wrote := h.loadCourse(c)
	if wrote {
		return nil
	}
	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}
	userID, err := c.ParamsInt("user_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}
	staff := callerIsCourseStaff(c)
	assignment, err := h.assignmentRepo.FindByID(c.Context(), uint(assignmentID), callerAccountID(c))
	if err != nil || assignment == nil || assignment.CourseID != course.ID || assignment.WorkflowState == models.AssignmentDeleted ||
		(!staff && !assignment.IsPublished()) {
		return responses.NotFound(c, "assignment")
	}
	if !staff && !h.callerMayViewStudent(c, uint(userID)) {
		return responses.NotFound(c, "submission")
	}
	sub, err := h.submissionRepo.FindByAssignmentAndUser(c.Context(), assignment.ID, uint(userID), callerAccountID(c))
	if err != nil || sub == nil {
		return c.JSON([]fiber.Map{})
	}
	versions, err := h.policy.SubmissionHistory(c.Context(), sub.ID)
	if err != nil {
		return responses.InternalError(c, "Could not fetch submission history")
	}
	out := make([]fiber.Map, 0, len(versions))
	for _, v := range versions {
		m := fiber.Map{
			"id": v.ID, "submission_id": v.SubmissionID, "attempt": v.Attempt, "assignment_id": v.AssignmentID,
			"user_id": v.UserID, "submission_type": v.SubmissionType, "body": v.Body, "url": v.URL,
			"submitted_at": v.SubmittedAt, "late": v.Late, "score": v.Score, "grade": v.Grade,
			"entered_score": v.EnteredScore, "graded_at": v.GradedAt, "grader_id": v.GraderID,
			"current": v.Attempt == sub.Attempt,
		}
		if v.Attachments != nil && *v.Attachments != "" {
			var atts []map[string]interface{}
			if json.Unmarshal([]byte(*v.Attachments), &atts) == nil {
				m["attachments"] = atts
			}
		}
		if !staff && sub.PostedAt == nil {
			for _, k := range []string{"score", "grade", "entered_score", "graded_at", "grader_id"} {
				m[k] = nil
			}
		}
		out = append(out, m)
	}
	return c.JSON(out)
}

// SetLateStatus handles
// PUT /courses/:course_id/assignments/:assignment_id/submissions/:user_id/late_policy_status
// {"late_policy_status": "late" | "missing" | "extended" | "none" | null,
// "seconds_late_override": 86400}: the instructor's manual status (null
// goes back to the computed one).
func (h *GradingPolicyHandler) SetLateStatus(c *fiber.Ctx) error {
	course, wrote := h.loadCourse(c)
	if wrote {
		return nil
	}
	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil || assignmentID <= 0 {
		return responses.BadRequest(c, "Invalid assignment ID")
	}
	userID, err := c.ParamsInt("user_id")
	if err != nil || userID <= 0 {
		return responses.BadRequest(c, "Invalid user ID")
	}
	actorID, err := getUserID(c)
	if err != nil {
		return err
	}
	acct := callerAccountID(c)
	assignment, err := h.assignmentRepo.FindByID(c.Context(), uint(assignmentID), acct)
	if err != nil || assignment == nil || assignment.CourseID != course.ID || assignment.WorkflowState == models.AssignmentDeleted {
		return responses.NotFound(c, "assignment")
	}
	if ok, _ := h.policy.IsCourseStudent(c.Context(), course.ID, uint(userID)); !ok {
		return responses.NotFound(c, "student")
	}
	var input struct {
		LatePolicyStatus    *string `json:"late_policy_status"`
		SecondsLateOverride *int64  `json:"seconds_late_override"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	status := ""
	if input.LatePolicyStatus != nil {
		status = strings.TrimSpace(*input.LatePolicyStatus)
	}
	if input.SecondsLateOverride != nil && *input.SecondsLateOverride < 0 {
		return responses.BadRequest(c, "seconds_late_override cannot be negative")
	}
	subs := h.policy.Submissions()
	if subs == nil {
		return responses.InternalError(c, "Grading is not available")
	}
	sub, err := subs.SetLatePolicyStatus(c.Context(), assignment.ID, uint(userID), actorID, acct, status, input.SecondsLateOverride)
	switch {
	case errors.Is(err, service.ErrInvalidLateStatus):
		return responses.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrGradeCrossTenant):
		return responses.NotFound(c, "assignment")
	case errors.Is(err, service.ErrModerationInProgress):
		return responses.Error(c, fiber.StatusConflict, err.Error())
	case err != nil:
		return responses.BadRequest(c, err.Error())
	}
	return c.JSON(submissionToJSON(sub))
}
