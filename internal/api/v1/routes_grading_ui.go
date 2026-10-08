package v1

import (
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/gofiber/fiber/v2"
)

// registerGradingUIRoutes mounts the grading-workflow surfaces: grading
// and commenting by anonymous id (anonymous grading), moderated grading
// (provisional grades, moderation summary, selection, publish), custom
// grade statuses, comment edit/delete and comment files, the
// student-annotation document, prior-attempt files, and peer-review
// management. Every handler re-checks the course parent-tie and tenant.
func (r *Router) registerGradingUIRoutes(protected fiber.Router, admin, enrolled, instructor fiber.Handler) {
	h := r.GradingUIHandler
	if h == nil {
		return
	}
	upload := []fiber.Handler{middleware.UploadRateLimit(), middleware.EnforceUploadSize(r.SettingsLookup)}

	// Anonymous grading: SpeedGrader addresses students by anonymous id.
	anon := "/courses/:course_id/assignments/:assignment_id/anonymous_submissions/:anonymous_id"
	protected.Put(anon, instructor, h.UpdateAnonymousSubmission)
	protected.Get(anon+"/comments", instructor, h.ListAnonymousSubmissionComments)
	protected.Post(anon+"/comments", instructor, h.CreateAnonymousSubmissionComment)
	protected.Post(anon+"/comment_files", append(upload, instructor, h.UploadAnonymousCommentFile)...)
	protected.Get(anon+"/files/:file_id/download", instructor, h.DownloadAnonymousSubmissionFile)
	protected.Get(anon+"/comments/:comment_id/files/:file_id/download", instructor, h.DownloadAnonymousCommentFile)
	protected.Get(anon+"/rubric_assessment", instructor, h.GetAnonymousRubricAssessment)
	protected.Put(anon+"/rubric_assessment", instructor, h.SaveAnonymousRubricAssessment)

	// Comment management and comment files (submission owner or staff).
	sub := "/courses/:course_id/assignments/:assignment_id/submissions/:user_id"
	protected.Put(sub+"/comments/:comment_id", enrolled, h.UpdateSubmissionComment)
	protected.Delete(sub+"/comments/:comment_id", enrolled, h.DeleteSubmissionComment)
	protected.Get(sub+"/comments/:comment_id/files/:file_id/download", enrolled, h.DownloadCommentFile)
	protected.Post(sub+"/comment_files", append(upload, enrolled, h.UploadCommentFile)...)
	protected.Get(sub+"/history/files/:file_id/download", enrolled, h.DownloadPriorAttemptFile)

	// Student annotation.
	protected.Post("/courses/:course_id/assignments/:assignment_id/annotation_session", enrolled, h.StartAnnotation)
	protected.Get("/courses/:course_id/assignments/:assignment_id/annotatable_document", enrolled, h.GetAnnotatableDocument)

	// Moderated grading.
	protected.Put("/courses/:course_id/assignments/:assignment_id/provisional_grades", instructor, h.SaveProvisionalGrade)
	protected.Get("/courses/:course_id/assignments/:assignment_id/moderation", instructor, h.GetModeration)
	protected.Put("/courses/:course_id/assignments/:assignment_id/moderation/selections", instructor, h.SelectProvisionalGrade)
	protected.Post("/courses/:course_id/assignments/:assignment_id/moderation/publish", instructor, h.PublishModeratedGrades)

	// Custom grade statuses.
	protected.Get("/accounts/:account_id/custom_grade_statuses", admin, h.ListAccountCustomGradeStatuses)
	protected.Post("/accounts/:account_id/custom_grade_statuses", admin, h.CreateCustomGradeStatus)
	protected.Put("/accounts/:account_id/custom_grade_statuses/:status_id", admin, h.UpdateCustomGradeStatus)
	protected.Delete("/accounts/:account_id/custom_grade_statuses/:status_id", admin, h.DeleteCustomGradeStatus)
	protected.Get("/courses/:course_id/custom_grade_statuses", instructor, h.ListCourseCustomGradeStatuses)
	protected.Get("/courses/:course_id/grading_periods", instructor, h.ListCourseGradingPeriods)

	// Peer reviews.
	protected.Post("/courses/:course_id/assignments/:id/peer_reviews/manual", instructor, h.AssignPeerReview)
	protected.Delete("/courses/:course_id/assignments/:id/peer_reviews/:review_id", instructor, h.DeletePeerReview)
	protected.Get("/courses/:course_id/assignments/:id/peer_reviews/received", enrolled, h.ListReceivedPeerReviews)
	protected.Get("/peer_reviews/:review_id/files/:file_id/download", h.DownloadPeerReviewFile)
}
