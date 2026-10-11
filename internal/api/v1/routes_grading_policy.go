package v1

import "github.com/gofiber/fiber/v2"

// registerGradingPolicyRoutes mounts the grading-policy lane: server
// computed course grades, effective due dates, final grade overrides,
// course post policy, grading scheme selection, account grading schemes,
// manual late / missing statuses, the missing-submission policy, and
// submission history. Every course route ties to :course_id under the
// caller's tenant inside the handler.
func (r *Router) registerGradingPolicyRoutes(protected fiber.Router, admin, enrolled, instructor fiber.Handler) {
	h := r.GradingPolicyHandler
	if h == nil {
		return
	}
	// Grades: staff see every student; a student sees their own posted
	// grades, an observer their observee's (?user_id=).
	protected.Get("/courses/:course_id/grades", enrolled, h.CourseGrades)
	protected.Get("/courses/:course_id/assignment_dates", enrolled, h.MyAssignmentDates)
	protected.Get("/courses/:course_id/effective_due_dates", instructor, h.EffectiveDueDates)

	// Final grade override (course or one grading period).
	protected.Put("/courses/:course_id/students/:user_id/score_override", instructor, h.SetScoreOverride)

	// Course post policy (default for its assignments).
	protected.Get("/courses/:course_id/post_policy", instructor, h.GetPostPolicy)
	protected.Put("/courses/:course_id/post_policy", instructor, h.SetPostPolicy)

	// Grading scheme selection (course + account schemes).
	protected.Get("/courses/:course_id/grading_schemes", enrolled, h.ListCourseSchemes)
	protected.Put("/courses/:course_id/grading_scheme", instructor, h.SetCourseScheme)

	// Late / missing: manual statuses and the missing-submission policy.
	protected.Put("/courses/:course_id/assignments/:assignment_id/submissions/:user_id/late_policy_status", instructor, h.SetLateStatus)
	protected.Post("/courses/:course_id/late_policy/apply", instructor, h.ApplyLatePolicy)

	// Submission history (every attempt).
	protected.Get("/courses/:course_id/assignments/:assignment_id/submissions/:user_id/history", enrolled, h.SubmissionHistory)

	// Account-level grading schemes shared by the account's courses.
	if gs := r.GradingStandardHandler; gs != nil {
		protected.Get("/accounts/:account_id/grading_standards", admin, gs.ListAccountGradingStandards)
		protected.Post("/accounts/:account_id/grading_standards", admin, gs.CreateAccountGradingStandard)
		protected.Put("/accounts/:account_id/grading_standards/:id", admin, gs.UpdateAccountGradingStandard)
		protected.Delete("/accounts/:account_id/grading_standards/:id", admin, gs.DeleteAccountGradingStandard)
	}
}
