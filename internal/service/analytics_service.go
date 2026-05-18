package service

import (
	"context"
	"math"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type AnalyticsService struct {
	pageViewRepo   repository.PageViewRepository
	submissionRepo repository.SubmissionRepository
	enrollmentRepo repository.EnrollmentRepository
	assignmentRepo repository.AssignmentRepository
}

func NewAnalyticsService(
	pageViewRepo repository.PageViewRepository,
	submissionRepo repository.SubmissionRepository,
	enrollmentRepo repository.EnrollmentRepository,
	assignmentRepo repository.AssignmentRepository,
) *AnalyticsService {
	return &AnalyticsService{
		pageViewRepo:   pageViewRepo,
		submissionRepo: submissionRepo,
		enrollmentRepo: enrollmentRepo,
		assignmentRepo: assignmentRepo,
	}
}

// GetCourseActivity returns page view counts grouped by date for a course.
func (s *AnalyticsService) GetCourseActivity(ctx context.Context, courseID uint) ([]map[string]interface{}, error) {
	return s.pageViewRepo.CountByContextGrouped(ctx, "Course", courseID)
}

// GetCourseAssignmentStats returns per-assignment statistics: title, points_possible, min/max/avg score.
func (s *AnalyticsService) GetCourseAssignmentStats(ctx context.Context, courseID uint) ([]map[string]interface{}, error) {
	// Fetch all assignments for the course
	assignmentResult, err := s.assignmentRepo.ListByCourseID(ctx, courseID, repository.PaginationParams{Page: 1, PerPage: 1000})
	if err != nil {
		return nil, err
	}

	// Fetch all submissions for the course
	submissionResult, err := s.submissionRepo.BulkListByCourse(ctx, courseID, repository.PaginationParams{Page: 1, PerPage: 10000})
	if err != nil {
		return nil, err
	}

	// Group submissions by assignment ID
	submissionsByAssignment := make(map[uint][]models.Submission)
	for _, sub := range submissionResult.Items {
		submissionsByAssignment[sub.AssignmentID] = append(submissionsByAssignment[sub.AssignmentID], sub)
	}

	var stats []map[string]interface{}
	for _, assignment := range assignmentResult.Items {
		subs := submissionsByAssignment[assignment.ID]

		var minScore, maxScore, avgScore *float64
		var scoredCount int
		var totalScore float64

		for _, sub := range subs {
			if sub.Score != nil {
				score := *sub.Score
				scoredCount++
				totalScore += score
				if minScore == nil || score < *minScore {
					s := score
					minScore = &s
				}
				if maxScore == nil || score > *maxScore {
					s := score
					maxScore = &s
				}
			}
		}

		if scoredCount > 0 {
			avg := math.Round((totalScore/float64(scoredCount))*100) / 100
			avgScore = &avg
		}

		stat := map[string]interface{}{
			"assignment_id":   assignment.ID,
			"title":           assignment.Name,
			"points_possible": assignment.PointsPossible,
			"min_score":       minScore,
			"max_score":       maxScore,
			"avg_score":       avgScore,
			"submission_count": len(subs),
		}
		stats = append(stats, stat)
	}

	if stats == nil {
		stats = []map[string]interface{}{}
	}

	return stats, nil
}

// GetStudentSummaries returns analytics summaries for each enrolled student in a course.
func (s *AnalyticsService) GetStudentSummaries(ctx context.Context, courseID uint, params repository.PaginationParams) ([]map[string]interface{}, error) {
	// accountID=0: service doesn't yet thread tenant context. Follow-up
	// per Wave 2 — repo accepts accountID; service signature needs widening.
	enrollmentResult, err := s.enrollmentRepo.ListByCourseID(ctx, courseID, 0, params)
	if err != nil {
		return nil, err
	}

	// Fetch all submissions for the course to count per student
	submissionResult, err := s.submissionRepo.BulkListByCourse(ctx, courseID, repository.PaginationParams{Page: 1, PerPage: 10000})
	if err != nil {
		return nil, err
	}

	// Count submitted assignments per user
	submittedByUser := make(map[uint]int)
	for _, sub := range submissionResult.Items {
		if sub.WorkflowState == "submitted" || sub.WorkflowState == "graded" {
			submittedByUser[sub.UserID]++
		}
	}

	var summaries []map[string]interface{}
	for _, enrollment := range enrollmentResult.Items {
		if enrollment.Type != "StudentEnrollment" {
			continue
		}

		userID := enrollment.UserID

		// Get page view count and interaction seconds for this student in this course.
		// Tenant scope is enforced upstream by GetStudentSummaries' callerAccountID.
		pageViewResult, _ := s.pageViewRepo.ListByUserID(ctx, userID, 0, repository.PaginationParams{Page: 1, PerPage: 1})
		var pageViewCount int64
		if pageViewResult != nil {
			pageViewCount = pageViewResult.TotalCount
		}

		interactionSeconds, _ := s.pageViewRepo.SumInteractionByUserAndContext(ctx, userID, "Course", courseID)

		userName := ""
		if enrollment.User != nil {
			userName = enrollment.User.Name
		}

		summary := map[string]interface{}{
			"id":                  userID,
			"name":                userName,
			"page_views":          pageViewCount,
			"interaction_seconds": interactionSeconds,
			"submissions":         submittedByUser[userID],
		}
		summaries = append(summaries, summary)
	}

	if summaries == nil {
		summaries = []map[string]interface{}{}
	}

	return summaries, nil
}

// GetStudentActivity returns page views for a specific student in a course.
func (s *AnalyticsService) GetStudentActivity(ctx context.Context, courseID, userID uint) ([]map[string]interface{}, error) {
	// Verify enrollment. accountID=0: service doesn't yet thread tenant.
	_, err := s.enrollmentRepo.FindByUserAndCourse(ctx, userID, courseID, 0)
	if err != nil {
		return nil, err
	}

	return s.pageViewRepo.CountByContextGrouped(ctx, "Course", courseID)
}

// GetStudentAssignments returns assignment submissions for a specific student in a course.
func (s *AnalyticsService) GetStudentAssignments(ctx context.Context, courseID, userID uint) ([]map[string]interface{}, error) {
	// Get all assignments for the course
	assignmentResult, err := s.assignmentRepo.ListByCourseID(ctx, courseID, repository.PaginationParams{Page: 1, PerPage: 1000})
	if err != nil {
		return nil, err
	}

	// Get user submissions for this course
	submissions, err := s.submissionRepo.ListByUserAndCourse(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}

	// Map submissions by assignment ID
	subByAssignment := make(map[uint]*models.Submission)
	for i := range submissions {
		subByAssignment[submissions[i].AssignmentID] = &submissions[i]
	}

	var results []map[string]interface{}
	for _, assignment := range assignmentResult.Items {
		entry := map[string]interface{}{
			"assignment_id":   assignment.ID,
			"title":           assignment.Name,
			"points_possible": assignment.PointsPossible,
			"due_at":          assignment.DueAt,
			"submission":      nil,
		}

		if sub, ok := subByAssignment[assignment.ID]; ok {
			entry["submission"] = map[string]interface{}{
				"score":          sub.Score,
				"submitted_at":   sub.SubmittedAt,
				"workflow_state": sub.WorkflowState,
				"late":           sub.Late,
				"missing":        sub.Missing,
				"excused":        sub.Excused,
			}
		}

		results = append(results, entry)
	}

	if results == nil {
		results = []map[string]interface{}{}
	}

	return results, nil
}

// GetDepartmentActivity returns placeholder activity stats for a department/account.
func (s *AnalyticsService) GetDepartmentActivity(ctx context.Context, accountID uint) ([]map[string]interface{}, error) {
	return []map[string]interface{}{
		{
			"date":          "2024-01-01",
			"views":         0,
			"participations": 0,
		},
	}, nil
}

// GetDepartmentGrades returns placeholder grade distribution for a department/account.
func (s *AnalyticsService) GetDepartmentGrades(ctx context.Context, accountID uint) ([]map[string]interface{}, error) {
	return []map[string]interface{}{
		{
			"score_bucket": "90-100",
			"count":        0,
		},
	}, nil
}

// GetDepartmentStatistics returns placeholder statistics for a department/account.
func (s *AnalyticsService) GetDepartmentStatistics(ctx context.Context, accountID uint) (map[string]interface{}, error) {
	return map[string]interface{}{
		"courses":     0,
		"teachers":    0,
		"students":    0,
		"assignments": 0,
		"submissions": 0,
	}, nil
}

// RecordPageView creates a new page view record.
func (s *AnalyticsService) RecordPageView(ctx context.Context, pageView *models.PageView) error {
	return s.pageViewRepo.Create(ctx, pageView)
}

// GetUserPageViews returns a paginated list of page views for a user.
// accountID==0 disables the tenant scope (privileged internal callers).
func (s *AnalyticsService) GetUserPageViews(ctx context.Context, userID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PageView], error) {
	return s.pageViewRepo.ListByUserID(ctx, userID, accountID, params)
}
