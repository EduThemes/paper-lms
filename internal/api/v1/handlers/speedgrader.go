package handlers

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/gofiber/fiber/v2"
)

// SpeedGraderHandler handles SpeedGrader-related API endpoints.
type SpeedGraderHandler struct {
	speedGraderService *service.SpeedGraderService
	auditService       *service.AuditService

	// gradingUI (optional) adds anonymous ids, attempt history, groups,
	// moderated grading and statuses to the SpeedGrader payload.
	gradingUI *service.GradingUIService
}

// NewSpeedGraderHandler creates a new SpeedGraderHandler.
func NewSpeedGraderHandler(speedGraderService *service.SpeedGraderService, auditService *service.AuditService) *SpeedGraderHandler {
	return &SpeedGraderHandler{speedGraderService: speedGraderService, auditService: auditService}
}

// SetGradingUIService enables the SpeedGrader parity features.
func (h *SpeedGraderHandler) SetGradingUIService(g *service.GradingUIService) { h.gradingUI = g }

func speedGraderSubmissionJSON(sub *models.Submission) fiber.Map {
	if sub == nil {
		return nil
	}
	result := fiber.Map{
		"id":                     sub.ID,
		"assignment_id":          sub.AssignmentID,
		"user_id":                sub.UserID,
		"submission_type":        sub.SubmissionType,
		"body":                   sub.Body,
		"url":                    sub.URL,
		"score":                  sub.Score,
		"grade":                  sub.Grade,
		"entered_score":          sub.EnteredScore,
		"entered_grade":          sub.EnteredGrade,
		"points_deducted":        sub.PointsDeducted,
		"posted_at":              sub.PostedAt,
		"graded_at":              sub.GradedAt,
		"grader_id":              sub.GraderID,
		"submitted_at":           sub.SubmittedAt,
		"attempt":                sub.Attempt,
		"late":                   sub.Late,
		"missing":                sub.Missing,
		"excused":                sub.Excused,
		"workflow_state":         sub.WorkflowState,
		"custom_grade_status_id": sub.CustomGradeStatusID,
		"late_policy_status":     sub.LatePolicyStatus,
		"extra_attempts":         sub.ExtraAttempts,
	}
	if sub.Attachments != nil && *sub.Attachments != "" {
		var attachments []map[string]interface{}
		if err := json.Unmarshal([]byte(*sub.Attachments), &attachments); err == nil {
			result["attachments"] = attachments
		}
	}
	return result
}

// anonymizeSubmissionMap strips the student's identity from a rendered
// submission: no user id, and file links go through the anonymous route.
func anonymizeSubmissionMap(m fiber.Map, courseID, assignmentID uint, anonymousID string) fiber.Map {
	if m == nil {
		return nil
	}
	m["user_id"] = nil
	m["anonymous_id"] = anonymousID
	if atts, ok := m["attachments"].([]map[string]interface{}); ok {
		for _, a := range atts {
			if id, ok := a["id"].(float64); ok {
				a["url"] = anonymousFileURL(courseID, assignmentID, anonymousID, uint(id))
			}
		}
	}
	return m
}

func anonymousFileURL(courseID, assignmentID uint, anonymousID string, fileID uint) string {
	return fmt.Sprintf("/api/v1/courses/%d/assignments/%d/anonymous_submissions/%s/files/%d/download", courseID, assignmentID, anonymousID, fileID)
}

// versionJSON renders one recorded attempt.
func versionJSON(v *models.SubmissionVersion, courseID uint, anonymousID string) fiber.Map {
	m := fiber.Map{
		"attempt":         v.Attempt,
		"submission_type": v.SubmissionType,
		"body":            v.Body,
		"url":             v.URL,
		"submitted_at":    v.SubmittedAt,
		"late":            v.Late,
	}
	atts := make([]map[string]interface{}, 0)
	if v.Attachments != nil && *v.Attachments != "" {
		_ = json.Unmarshal([]byte(*v.Attachments), &atts)
	}
	// The URL stored with the attempt is the current-submission route,
	// which only serves files of the latest attempt; earlier attempts'
	// files go through the history (or anonymous) route.
	for _, a := range atts {
		if id, ok := a["id"].(float64); ok {
			if anonymousID != "" {
				a["url"] = anonymousFileURL(courseID, v.AssignmentID, anonymousID, uint(id))
			} else {
				a["url"] = fmt.Sprintf("/api/v1/courses/%d/assignments/%d/submissions/%d/history/files/%d/download", courseID, v.AssignmentID, v.UserID, uint(id))
			}
		}
	}
	m["attachments"] = atts
	return m
}

// speedGraderCommentVisible applies the comment visibility rules to the
// grader's view: drafts are private to their author, and on an
// unpublished moderated assignment other graders' comments show only
// when the assignment allows it (or to the moderator).
func speedGraderCommentVisible(a *models.Assignment, sc *models.SubmissionComment, studentID, callerID uint, isModerator bool) bool {
	if sc.Draft && sc.AuthorID != callerID {
		return false
	}
	if a.ModeratedGrading && !a.GradesPublished() && !a.GraderCommentsVisibleToGraders &&
		sc.AuthorID != callerID && sc.AuthorID != studentID {
		return isModerator
	}
	return true
}

// GetSpeedGraderData returns the full SpeedGrader data set for an assignment,
// including the assignment, all enrolled students, their submissions, and comments.
// GET /courses/:course_id/assignments/:assignment_id/speedgrader
func (h *SpeedGraderHandler) GetSpeedGraderData(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	var anonymize func(*models.Assignment, []uint) bool
	if h.gradingUI != nil {
		anonymize = func(a *models.Assignment, ids []uint) bool { return h.gradingUI.AnonymizeStudents(c.Context(), a, ids) }
	}
	data, err := h.speedGraderService.GetSpeedGraderDataAnonymized(c.Context(), uint(courseID), uint(assignmentID), callerAccountID(c), anonymize)
	if err != nil {
		if err == service.ErrSpeedGraderAssignmentNotFound {
			return responses.NotFound(c, "assignment")
		}
		return responses.InternalError(c, "Could not fetch SpeedGrader data")
	}

	callerID, _ := getUserID(c)
	if callerID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, 0, "read", "speedgrader_bulk", "assignments", uint(assignmentID), c.IP(), c.Get("User-Agent"))
	}

	assignment := data.Assignment
	isAdmin, _ := c.Locals("is_admin").(bool)
	isModerator := service.IsModerator(assignment, callerID, isAdmin)

	studentIDList := make([]uint, 0, len(data.Students))
	studentIDs := make(map[uint]bool, len(data.Students))
	for _, student := range data.Students {
		studentIDs[student.UserID] = true
		studentIDList = append(studentIDList, student.UserID)
	}

	// Anonymity: the legacy flag-only behavior (labels, masked comment
	// authors) applies whenever anonymous grading is on; with the
	// grading-UI service wired, identities lift automatically once the
	// grades are posted (or, moderated, published) and every student is
	// addressed by an anonymous id instead of a user id.
	anonymous := data.Anonymized
	var rows map[uint]*models.Submission
	var versions map[uint][]models.SubmissionVersion
	var groups map[uint]service.StudentGroup
	var provisional map[uint][]models.ProvisionalGrade
	var selections map[uint]*uint
	graderLabels := map[uint]string{}
	hideGraderNames := false // other graders shown by label (moderated grading)
	var statuses []models.CustomGradeStatus
	if h.gradingUI != nil {
		if anonymous || assignment.ModeratedGrading {
			if r, rErr := h.gradingUI.EnsureSubmissionRows(c.Context(), assignment, studentIDList); rErr == nil {
				rows = r
			}
		}
		versions, _ = h.gradingUI.VersionsByAssignment(c.Context(), assignment.ID)
		if !anonymous {
			groups = h.gradingUI.GroupsForStudents(c.Context(), assignment, studentIDList)
		}
		if assignment.ModeratedGrading {
			subIDs := make([]uint, 0, len(rows))
			for _, r := range rows {
				subIDs = append(subIDs, r.ID)
			}
			provisional, _ = h.gradingUI.ProvisionalGradesForStudents(c.Context(), subIDs)
			selections, _ = h.gradingUI.ModerationSelections(c.Context(), assignment.ID)
			graders, _ := h.gradingUI.ModerationGraders(c.Context(), assignment.ID)
			hideGraderNames = !service.CanViewOtherGraderIdentities(assignment, callerID, isAdmin)
			for _, g := range graders {
				if hideGraderNames && g.UserID != callerID {
					graderLabels[g.UserID] = "Grader " + strings.ToUpper(g.AnonymousID)
				} else if name := data.UserNameMap[g.UserID]; name != "" {
					graderLabels[g.UserID] = name
				} else {
					graderLabels[g.UserID] = fmt.Sprintf("User %d", g.UserID)
				}
			}
		}
		if acct, aErr := h.gradingUI.CourseAccountID(c.Context(), uint(courseID), callerAccountID(c)); aErr == nil {
			statuses, _ = h.gradingUI.ListCustomGradeStatuses(c.Context(), acct)
		}
	}

	// Use the full user name map (includes teachers, TAs, and students)
	userNameLookup := data.UserNameMap

	studentsJSON := make([]fiber.Map, len(data.Students))
	for i, student := range data.Students {
		sub := student.Submission
		if r := rows[student.UserID]; r != nil {
			sub = r
		}
		anonID := ""
		if sub != nil && sub.AnonymousID != nil {
			anonID = *sub.AnonymousID
		}
		addressAnonymously := anonymous && h.gradingUI != nil && anonID != ""

		commentsJSON := make([]fiber.Map, 0, len(student.Comments))
		for j := range student.Comments {
			comment := &student.Comments[j]
			if !speedGraderCommentVisible(assignment, comment, student.UserID, callerID, isModerator) {
				continue
			}
			authorName := userNameLookup[comment.AuthorID]
			if authorName == "" {
				authorName = fmt.Sprintf("User %d", comment.AuthorID)
			}
			// B119: under anonymous grading a student author is identified
			// only by their anonymous label — no real name, no user id.
			var authorID interface{} = comment.AuthorID
			if anonymous && studentIDs[comment.AuthorID] {
				authorID = nil
				if comment.AuthorID == student.UserID {
					authorName = student.UserName
				}
			}
			// Moderated grading: another grader's comment carries only
			// their grader label while identities are hidden from the
			// caller (graders_anonymous_to_graders /
			// grader_names_visible_to_final_grader).
			if hideGraderNames && !studentIDs[comment.AuthorID] && comment.AuthorID != callerID {
				authorID = nil
				authorName = graderLabels[comment.AuthorID]
				if authorName == "" {
					authorName = "Grader"
				}
			}
			pathKey := fmt.Sprintf("submissions/%d", student.UserID)
			if addressAnonymously {
				pathKey = "anonymous_submissions/" + anonID
			}
			cj := commentJSON(uint(courseID), assignment.ID, pathKey, comment)
			cj["author_id"] = authorID
			cj["author_name"] = authorName
			commentsJSON = append(commentsJSON, cj)
		}

		subJSON := speedGraderSubmissionJSON(sub)
		var userIDField interface{} = student.UserID
		if addressAnonymously {
			subJSON = anonymizeSubmissionMap(subJSON, uint(courseID), assignment.ID, anonID)
			userIDField = nil
		}

		entry := fiber.Map{
			"user_id":    userIDField,
			"user_name":  student.UserName,
			"submission": subJSON,
			"comments":   commentsJSON,
		}
		if addressAnonymously {
			entry["anonymous_id"] = anonID
		}
		if h.gradingUI != nil {
			entry["attempts_remaining"] = service.AttemptsRemaining(assignment, sub)
			history := make([]fiber.Map, 0)
			if sub != nil {
				for k := range versions[sub.ID] {
					va := ""
					if addressAnonymously {
						va = anonID
					}
					history = append(history, versionJSON(&versions[sub.ID][k], uint(courseID), va))
				}
			}
			entry["submission_history"] = history
			if g, ok := groups[student.UserID]; ok {
				entry["group_id"] = g.GroupID
				entry["group_name"] = g.GroupName
			}
			if assignment.ModeratedGrading && sub != nil {
				pgs := make([]fiber.Map, 0)
				for _, pg := range provisional[sub.ID] {
					mine := pg.ScorerID == callerID
					if !mine && !isModerator {
						continue
					}
					pgs = append(pgs, fiber.Map{
						"id":          pg.ID,
						"score":       pg.Score,
						"grade":       pg.Grade,
						"final":       pg.Final,
						"mine":        mine,
						"scorer_name": graderLabels[pg.ScorerID],
						"graded_at":   pg.GradedAt,
						"selected":    selections[student.UserID] != nil && *selections[student.UserID] == pg.ID,
					})
				}
				entry["provisional_grades"] = pgs
			}
		}
		studentsJSON[i] = entry
	}

	assignmentJSON := fiber.Map{
		"id":                                assignment.ID,
		"course_id":                         assignment.CourseID,
		"name":                              assignment.Name,
		"description":                       assignment.Description,
		"due_at":                            assignment.DueAt,
		"points_possible":                   assignment.PointsPossible,
		"grading_type":                      assignment.GradingType,
		"submission_types":                  assignment.SubmissionTypes,
		"published":                         assignment.Published,
		"anonymous_grading":                 assignment.AnonymousGrading,
		"anonymize_students":                anonymous,
		"post_policy":                       assignment.PostPolicy,
		"moderated_grading":                 assignment.ModeratedGrading,
		"grades_published":                  assignment.GradesPublished(),
		"is_moderator":                      isModerator,
		"grader_count":                      assignment.GraderCount,
		"is_group_assignment":               assignment.IsGroupAssignment(),
		"grade_group_students_individually": assignment.GradeGroupStudentsIndividually,
		"allowed_attempts":                  assignment.AllowedAttempts,
		"annotatable_attachment_id":         assignment.AnnotatableAttachmentID,
	}
	statusJSON := make([]fiber.Map, 0, len(statuses))
	for _, st := range statuses {
		if st.AppliesToSubmissions {
			statusJSON = append(statusJSON, fiber.Map{"id": st.ID, "name": st.Name, "color": st.Color})
		}
	}

	return c.JSON(fiber.Map{
		"assignment":            assignmentJSON,
		"students":              studentsJSON,
		"custom_grade_statuses": statusJSON,
	})
}

// GetStudentSubmission returns a single student's submission with comments
// for the given assignment.
// GET /courses/:course_id/assignments/:assignment_id/speedgrader/submissions/:user_id
func (h *SpeedGraderHandler) GetStudentSubmission(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	userID, err := c.ParamsInt("user_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	data, err := h.speedGraderService.GetStudentSubmission(c.Context(), uint(courseID), uint(assignmentID), uint(userID), callerAccountID(c))
	if err != nil {
		if err == service.ErrSpeedGraderAssignmentNotFound {
			return responses.NotFound(c, "assignment")
		}
		return responses.InternalError(c, "Could not fetch student submission")
	}
	anonymous := data.Assignment != nil && data.Assignment.AnonymousGrading

	if callerID, _ := getUserID(c); callerID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, uint(userID), "read", "speedgrader_submission", "assignments", uint(assignmentID), c.IP(), c.Get("User-Agent"))
	}

	callerID, _ := getUserID(c)
	isAdmin, _ := c.Locals("is_admin").(bool)
	isModerator := service.IsModerator(data.Assignment, callerID, isAdmin)
	hideGraders := !service.CanViewOtherGraderIdentities(data.Assignment, callerID, isAdmin)
	commentsJSON := make([]fiber.Map, 0, len(data.Comments))
	for i := range data.Comments {
		comment := &data.Comments[i]
		if data.Assignment != nil && !speedGraderCommentVisible(data.Assignment, comment, uint(userID), callerID, isModerator) {
			continue
		}
		var authorID interface{} = comment.AuthorID
		if anonymous && comment.AuthorID == uint(userID) {
			authorID = nil
		}
		// Moderated grading: another grader stays unnamed while grader
		// identities are hidden from the caller.
		if hideGraders && comment.AuthorID != uint(userID) && comment.AuthorID != callerID {
			authorID = nil
		}
		cj := commentJSON(uint(courseID), uint(assignmentID), fmt.Sprintf("submissions/%d", userID), comment)
		cj["author_id"] = authorID
		commentsJSON = append(commentsJSON, cj)
	}

	return c.JSON(fiber.Map{
		"submission": speedGraderSubmissionJSON(data.Submission),
		"comments":   commentsJSON,
	})
}
