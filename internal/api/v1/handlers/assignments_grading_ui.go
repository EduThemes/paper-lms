package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/gofiber/fiber/v2"
)

// Grading-UI assignment settings (allowed attempts and extensions, the
// annotatable document, group grading, moderated grading, peer-review
// options). Kept apart from assignments.go so the create/update handlers
// only call applyGradingUISettings / addGradingUIFields.

// SetGradingUIDeps enables validation of the final grader (must be a
// teacher or TA of the course) and of the annotatable document (must be
// a file of the course).
func (h *AssignmentHandler) SetGradingUIDeps(enrollmentRepo repository.EnrollmentRepository, attachmentRepo repository.AttachmentRepository) {
	h.enrollmentRepo = enrollmentRepo
	h.attachmentRepo = attachmentRepo
}

// gradesExistFunc reports whether an assignment has graded submissions
// or provisional grades.
type gradesExistFunc func(ctx context.Context, assignmentID uint) bool

// SetGradesExistCheck enables Canvas's rule that moderated grading
// cannot be switched on or off once work has been graded (nil-safe).
func (h *AssignmentHandler) SetGradesExistCheck(fn func(ctx context.Context, assignmentID uint) bool) {
	h.gradesExist = fn
}

// addGradingUIFields adds the settings to an assignment's JSON.
func addGradingUIFields(m fiber.Map, a *models.Assignment) {
	m["allowed_attempts"] = a.AllowedAttempts
	exts := []string{}
	if strings.TrimSpace(a.AllowedExtensions) != "" {
		exts = strings.Split(a.AllowedExtensions, ",")
	}
	m["allowed_extensions"] = exts
	m["annotatable_attachment_id"] = a.AnnotatableAttachmentID
	m["grade_group_students_individually"] = a.GradeGroupStudentsIndividually
	m["moderated_grading"] = a.ModeratedGrading
	m["grader_count"] = a.GraderCount
	m["final_grader_id"] = a.FinalGraderID
	m["grader_comments_visible_to_graders"] = a.GraderCommentsVisibleToGraders
	m["graders_anonymous_to_graders"] = a.GradersAnonymousToGraders
	m["grader_names_visible_to_final_grader"] = a.GraderNamesVisibleToFinalGrader
	m["grades_published"] = a.GradesPublished()
	m["grades_published_at"] = a.GradesPublishedAt
	m["anonymous_peer_reviews"] = a.AnonymousPeerReviews
	m["peer_reviews_due_at"] = a.PeerReviewsDueAt
	m["omit_from_final_grade"] = a.OmitFromFinalGrade
}

// gradingUISettingsInput is the writable part (all optional).
type gradingUISettingsInput struct {
	AllowedAttempts                 *int            `json:"allowed_attempts"`
	AllowedExtensions               json.RawMessage `json:"allowed_extensions"` // "pdf,docx" or ["pdf","docx"]
	AnnotatableAttachmentID         *uint           `json:"annotatable_attachment_id"`
	GradeGroupStudentsIndividually  *bool           `json:"grade_group_students_individually"`
	ModeratedGrading                *bool           `json:"moderated_grading"`
	GraderCount                     *int            `json:"grader_count"`
	FinalGraderID                   *uint           `json:"final_grader_id"`
	GraderCommentsVisibleToGraders  *bool           `json:"grader_comments_visible_to_graders"`
	GradersAnonymousToGraders       *bool           `json:"graders_anonymous_to_graders"`
	GraderNamesVisibleToFinalGrader *bool           `json:"grader_names_visible_to_final_grader"`
	AnonymousPeerReviews            *bool           `json:"anonymous_peer_reviews"`
	PeerReviewsDueAt                *time.Time      `json:"peer_reviews_due_at"`
	OmitFromFinalGrade              *bool           `json:"omit_from_final_grade"`
}

// applyGradingUISettings reads the settings from the request body onto
// a. creating=true applies Canvas defaults for settings not sent.
func (h *AssignmentHandler) applyGradingUISettings(c *fiber.Ctx, a *models.Assignment, creating bool) error {
	var body struct {
		Assignment gradingUISettingsInput `json:"assignment"`
	}
	var raw struct {
		Assignment map[string]json.RawMessage `json:"assignment"`
	}
	// The create/update handlers already parsed the body with BodyParser,
	// which also accepts form-encoded input. Those settings are JSON-only;
	// a body that is not a JSON object carries none of them (the Canvas
	// defaults below still apply on create).
	if err := json.Unmarshal(c.Body(), &body); err == nil {
		_ = json.Unmarshal(c.Body(), &raw)
	} else if strings.Contains(strings.ToLower(c.Get(fiber.HeaderContentType)), "json") {
		return errors.New("invalid input")
	}
	isNull := func(key string) bool {
		v, ok := raw.Assignment[key]
		return ok && strings.TrimSpace(string(v)) == "null"
	}
	in := body.Assignment
	wasModerated := a.ModeratedGrading

	if isNull("allowed_attempts") {
		a.AllowedAttempts = nil
	} else if in.AllowedAttempts != nil {
		if *in.AllowedAttempts <= 0 {
			a.AllowedAttempts = nil // -1 / 0: unlimited (Canvas -1)
		} else {
			n := *in.AllowedAttempts
			a.AllowedAttempts = &n
		}
	}
	if len(in.AllowedExtensions) > 0 && !isNull("allowed_extensions") {
		var list []string
		var str string
		switch {
		case json.Unmarshal(in.AllowedExtensions, &list) == nil:
			a.AllowedExtensions = service.NormalizeAllowedExtensions(strings.Join(list, ","))
		case json.Unmarshal(in.AllowedExtensions, &str) == nil:
			a.AllowedExtensions = service.NormalizeAllowedExtensions(str)
		default:
			return errors.New("allowed_extensions must be a list of file extensions")
		}
	} else if isNull("allowed_extensions") {
		a.AllowedExtensions = ""
	}

	if isNull("annotatable_attachment_id") {
		a.AnnotatableAttachmentID = nil
	} else if in.AnnotatableAttachmentID != nil {
		if *in.AnnotatableAttachmentID == 0 {
			a.AnnotatableAttachmentID = nil
		} else {
			if h.attachmentRepo != nil {
				att, err := h.attachmentRepo.FindByID(c.Context(), *in.AnnotatableAttachmentID, callerAccountID(c))
				if err != nil || att == nil || att.ContextType != "Course" || att.ContextID != a.CourseID {
					return errors.New("annotatable_attachment_id must be a file in this course")
				}
			}
			id := *in.AnnotatableAttachmentID
			a.AnnotatableAttachmentID = &id
		}
	}
	if hasSubmissionType(a, "student_annotation") && (a.AnnotatableAttachmentID == nil || *a.AnnotatableAttachmentID == 0) {
		return errors.New("a student annotation assignment needs a document to annotate (annotatable_attachment_id)")
	}

	if in.GradeGroupStudentsIndividually != nil {
		a.GradeGroupStudentsIndividually = *in.GradeGroupStudentsIndividually
	}
	if in.OmitFromFinalGrade != nil {
		a.OmitFromFinalGrade = *in.OmitFromFinalGrade
	}
	if in.AnonymousPeerReviews != nil {
		a.AnonymousPeerReviews = *in.AnonymousPeerReviews
	}
	if isNull("peer_reviews_due_at") {
		a.PeerReviewsDueAt = nil
	} else if in.PeerReviewsDueAt != nil {
		a.PeerReviewsDueAt = in.PeerReviewsDueAt
	}

	// Moderated grading.
	if in.ModeratedGrading != nil && *in.ModeratedGrading != wasModerated && a.GradesPublishedAt != nil {
		return errors.New("moderated grading cannot be changed after grades have been published")
	}
	if in.ModeratedGrading != nil && *in.ModeratedGrading != wasModerated && !creating &&
		h.gradesExist != nil && h.gradesExist(c.Context(), a.ID) {
		// Canvas moderation_setting_ok?: switching after grading would
		// either expose grades a moderator never selected or strand the
		// provisional grades.
		return errors.New("moderated grading cannot be changed once submissions have been graded")
	}
	if in.ModeratedGrading != nil {
		a.ModeratedGrading = *in.ModeratedGrading
	}
	if a.ModeratedGrading && (creating || !wasModerated) {
		switch {
		case a.GradingType == "not_graded":
			return errors.New("moderated grading cannot be enabled for ungraded assignments")
		case a.IsGroupAssignment():
			return errors.New("moderated grading cannot be enabled for group assignments")
		case a.PeerReviewsEnabled:
			return errors.New("moderated grading cannot be enabled for peer reviewed assignments")
		}
	}
	if in.GraderCount != nil {
		if *in.GraderCount < 1 || *in.GraderCount > 10 {
			return errors.New("grader_count must be between 1 and 10")
		}
		a.GraderCount = *in.GraderCount
	}
	if a.ModeratedGrading && a.GraderCount < 1 {
		a.GraderCount = 2 // Canvas default
	}
	if a.ModeratedGrading {
		// Canvas: moderated assignments always post manually, so neither
		// a grader's comment nor the publish step shows grades or
		// feedback to students before the instructor posts them.
		a.PostPolicy = "manual"
	}
	if isNull("final_grader_id") {
		a.FinalGraderID = nil
	} else if in.FinalGraderID != nil {
		if *in.FinalGraderID == 0 {
			a.FinalGraderID = nil
		} else {
			if h.enrollmentRepo != nil {
				e, err := h.enrollmentRepo.FindByUserAndCourse(c.Context(), *in.FinalGraderID, a.CourseID, callerAccountID(c))
				if err != nil || e == nil || (e.Type != "TeacherEnrollment" && e.Type != "TaEnrollment") {
					return errors.New("final_grader_id must be a teacher or TA of this course")
				}
			}
			id := *in.FinalGraderID
			a.FinalGraderID = &id
		}
	}
	if in.GraderCommentsVisibleToGraders != nil {
		a.GraderCommentsVisibleToGraders = *in.GraderCommentsVisibleToGraders
	} else if creating {
		a.GraderCommentsVisibleToGraders = true
	}
	if in.GradersAnonymousToGraders != nil {
		a.GradersAnonymousToGraders = *in.GradersAnonymousToGraders
	}
	if in.GraderNamesVisibleToFinalGrader != nil {
		a.GraderNamesVisibleToFinalGrader = *in.GraderNamesVisibleToFinalGrader
	} else if creating {
		a.GraderNamesVisibleToFinalGrader = true
	}
	return nil
}
