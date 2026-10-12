package handlers

import "github.com/EduThemes/paper-lms/internal/domain/models"

// applyCourseDateAccessSettings copies the G106 course settings from an
// UpdateCourse body (nil = leave unchanged):
//
//   - restrict_enrollments_to_course_dates: the course's start/end dates
//     bound participation instead of the term's;
//   - restrict_student_future_view / restrict_student_past_view: students
//     and observers get no access at all (rather than read-only) before
//     their enrollment starts / after it concludes.
func applyCourseDateAccessSettings(course *models.Course, restrictToCourseDates, restrictFutureView, restrictPastView *bool) {
	if restrictToCourseDates != nil {
		course.RestrictEnrollmentsToCourseDates = *restrictToCourseDates
	}
	if restrictFutureView != nil {
		course.RestrictStudentFutureView = *restrictFutureView
	}
	if restrictPastView != nil {
		course.RestrictStudentPastView = *restrictPastView
	}
}
