package handlers

// Item analysis endpoints (quiz-items lane, G054): JSON report plus the
// item-analysis and student-analysis CSV downloads.

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

// SetUserRepo lets the student-analysis CSV resolve respondent names.
func (h *QuizStatisticsHandler) SetUserRepo(repo repository.UserRepository) {
	h.userRepo = repo
}

// loadAnalysisQuiz parses :course_id / :quiz_id and loads the quiz scoped to
// the course and tenant. wrote=true means a response was written.
func (h *QuizStatisticsHandler) loadAnalysisQuiz(c *fiber.Ctx) (*models.Quiz, bool) {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid course ID")
		return nil, true
	}
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid quiz ID")
		return nil, true
	}
	return loadQuizForCaller(c, h.quizService, uint(quizID), uint(courseID))
}

// GetItemAnalysis handles GET /courses/:course_id/quizzes/:quiz_id/item_analysis.
func (h *QuizStatisticsHandler) GetItemAnalysis(c *fiber.Ctx) error {
	quiz, wrote := h.loadAnalysisQuiz(c)
	if wrote {
		return nil
	}
	report, _, _, err := h.quizService.BuildItemAnalysis(c.Context(), quiz)
	if err != nil {
		return responses.InternalError(c, "Could not compute item analysis")
	}
	return c.JSON(report)
}

// ItemAnalysisCSV handles GET /courses/:course_id/quizzes/:quiz_id/item_analysis.csv.
func (h *QuizStatisticsHandler) ItemAnalysisCSV(c *fiber.Ctx) error {
	quiz, wrote := h.loadAnalysisQuiz(c)
	if wrote {
		return nil
	}
	report, _, _, err := h.quizService.BuildItemAnalysis(c.Context(), quiz)
	if err != nil {
		return responses.InternalError(c, "Could not compute item analysis")
	}
	var buf bytes.Buffer
	if err := service.WriteItemAnalysisCSV(&buf, report); err != nil {
		return responses.InternalError(c, "Could not write item analysis")
	}
	return sendQuizCSV(c, buf.Bytes(), fmt.Sprintf("quiz-%d-item-analysis.csv", quiz.ID))
}

// StudentAnalysisCSV handles GET /courses/:course_id/quizzes/:quiz_id/student_analysis.csv.
// Anonymous surveys never carry names or user ids.
func (h *QuizStatisticsHandler) StudentAnalysisCSV(c *fiber.Ctx) error {
	quiz, wrote := h.loadAnalysisQuiz(c)
	if wrote {
		return nil
	}
	report, respondents, questions, err := h.quizService.BuildItemAnalysis(c.Context(), quiz)
	if err != nil {
		return responses.InternalError(c, "Could not compute student analysis")
	}
	names := service.StudentAnalysisNames{}
	if !quiz.AnonymousSubmissions && h.userRepo != nil && len(respondents) > 0 {
		ids := make([]uint, 0, len(respondents))
		for _, r := range respondents {
			ids = append(ids, r.UserID())
		}
		if users, uErr := h.userRepo.FindByIDs(c.Context(), ids, callerAccountID(c)); uErr == nil {
			for _, u := range users {
				names[u.ID] = [2]string{u.Name, u.LoginID}
			}
		}
	}
	var buf bytes.Buffer
	if err := service.WriteStudentAnalysisCSV(&buf, report, respondents, questions, names); err != nil {
		return responses.InternalError(c, "Could not write student analysis")
	}
	// PII audit: a bulk read of every student's answers.
	if userID, _ := c.Locals("user_id").(uint); userID != 0 && h.auditService != nil && !quiz.AnonymousSubmissions {
		_ = h.auditService.LogPIIAccess(c.Context(), userID, 0, "read", "bulk_quiz_student_analysis_export", "quizzes", quiz.ID, c.IP(), c.Get("User-Agent"))
	}
	return sendQuizCSV(c, buf.Bytes(), fmt.Sprintf("quiz-%d-student-analysis.csv", quiz.ID))
}

// SetAuditService enables PII audit logging for the student-analysis export.
func (h *QuizStatisticsHandler) SetAuditService(a *service.AuditService) {
	h.auditService = a
}

func sendQuizCSV(c *fiber.Ctx, body []byte, filename string) error {
	filename = strings.NewReplacer(`"`, "", "\r", "", "\n", "").Replace(filename)
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="`+filename+`"`)
	c.Set("Cache-Control", "no-store")
	return c.Send(body)
}
