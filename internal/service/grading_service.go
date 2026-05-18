package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// GradeEntry represents a single row in a grading scale: the grade name and
// the minimum percentage (as 0.0–1.0) required to earn it.
type GradeEntry struct {
	Name  string
	Value float64 // minimum percentage threshold (e.g., 0.93 for 93%)
}

type GradingService struct {
	submissionRepo      repository.SubmissionRepository
	assignmentRepo      repository.AssignmentRepository
	groupRepo           repository.AssignmentGroupRepository
	enrollmentRepo      repository.EnrollmentRepository
	courseRepo           repository.CourseRepository
	gradingStandardRepo repository.GradingStandardRepository
}

func NewGradingService(submissionRepo repository.SubmissionRepository, assignmentRepo repository.AssignmentRepository, groupRepo repository.AssignmentGroupRepository, enrollmentRepo repository.EnrollmentRepository, courseRepo repository.CourseRepository, gradingStandardRepo repository.GradingStandardRepository) *GradingService {
	return &GradingService{
		submissionRepo:      submissionRepo,
		assignmentRepo:      assignmentRepo,
		groupRepo:           groupRepo,
		enrollmentRepo:      enrollmentRepo,
		courseRepo:           courseRepo,
		gradingStandardRepo: gradingStandardRepo,
	}
}

type GradebookEntry struct {
	Students    []GradebookStudent                       `json:"students"`
	Assignments []GradebookAssignment                    `json:"assignments"`
	Submissions map[string]map[string]*models.Submission `json:"submissions"`
}

type GradebookStudent struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type GradebookAssignment struct {
	ID             uint     `json:"id"`
	Name           string   `json:"name"`
	PointsPossible *float64 `json:"points_possible"`
}

type StudentGrade struct {
	CurrentGrade string  `json:"current_grade"`
	CurrentScore float64 `json:"current_score"`
	FinalGrade   string  `json:"final_grade"`
	FinalScore   float64 `json:"final_score"`
}

func (s *GradingService) GetGradebook(ctx context.Context, courseID uint) (*GradebookEntry, error) {
	// Get all enrollments for the course (students)
	params := repository.PaginationParams{Page: 1, PerPage: 1000}
	enrollments, err := s.enrollmentRepo.ListByCourseID(ctx, courseID, 0, params)
	if err != nil {
		return nil, err
	}

	// Get all assignments for the course
	assignments, err := s.assignmentRepo.ListByCourseID(ctx, courseID, params)
	if err != nil {
		return nil, err
	}

	// Get all submissions for the course
	allSubmissions, err := s.submissionRepo.BulkListByCourse(ctx, courseID, repository.PaginationParams{Page: 1, PerPage: 10000})
	if err != nil {
		return nil, err
	}

	// Build student list (only students)
	students := make([]GradebookStudent, 0)
	for _, e := range enrollments.Items {
		if e.Type == "StudentEnrollment" {
			name := ""
			if e.User != nil {
				name = e.User.Name
			}
			students = append(students, GradebookStudent{
				ID:   e.UserID,
				Name: name,
			})
		}
	}

	// Build assignment list
	gradebookAssignments := make([]GradebookAssignment, len(assignments.Items))
	for i, a := range assignments.Items {
		gradebookAssignments[i] = GradebookAssignment{
			ID:             a.ID,
			Name:           a.Name,
			PointsPossible: a.PointsPossible,
		}
	}

	// Build submissions map: student_id -> assignment_id -> submission
	submissionsMap := make(map[string]map[string]*models.Submission)
	for i, sub := range allSubmissions.Items {
		studentKey := fmt.Sprintf("%d", sub.UserID)
		assignmentKey := fmt.Sprintf("%d", sub.AssignmentID)
		if submissionsMap[studentKey] == nil {
			submissionsMap[studentKey] = make(map[string]*models.Submission)
		}
		submissionsMap[studentKey][assignmentKey] = &allSubmissions.Items[i]
	}

	return &GradebookEntry{
		Students:    students,
		Assignments: gradebookAssignments,
		Submissions: submissionsMap,
	}, nil
}

func (s *GradingService) GetStudentGrade(ctx context.Context, courseID, studentID uint) (*StudentGrade, error) {
	// Get course to check if weighted grading is enabled
	course, err := s.courseRepo.FindByID(ctx, courseID, 0)
	if err != nil {
		return nil, err
	}

	// Look up the course's custom grading scale (if any)
	scale := s.getCourseGradingScale(ctx, courseID)

	// Get all assignments for the course
	params := repository.PaginationParams{Page: 1, PerPage: 1000}
	assignments, err := s.assignmentRepo.ListByCourseID(ctx, courseID, params)
	if err != nil {
		return nil, err
	}

	// Get student submissions
	submissions, err := s.submissionRepo.ListByUserAndCourse(ctx, studentID, courseID)
	if err != nil {
		return nil, err
	}

	// Build submission lookup
	subByAssignment := make(map[uint]*models.Submission)
	for i, sub := range submissions {
		subByAssignment[sub.AssignmentID] = &submissions[i]
	}

	// Check if weighted grading is enabled and groups have weights
	if course.ApplyGroupWeights {
		groups, err := s.groupRepo.ListByCourseID(ctx, courseID, repository.PaginationParams{Page: 1, PerPage: 100})
		if err == nil && len(groups.Items) > 0 {
			currentScore, finalScore := s.calculateWeightedGrade(assignments.Items, groups.Items, subByAssignment)
			return &StudentGrade{
				CurrentGrade: scoreToLetterGradeWithScale(currentScore, scale),
				CurrentScore: currentScore,
				FinalGrade:   scoreToLetterGradeWithScale(finalScore, scale),
				FinalScore:   finalScore,
			}, nil
		}
	}

	// Unweighted: simple point totals
	var currentEarned, currentPossible float64
	var finalEarned, finalPossible float64

	for _, a := range assignments.Items {
		if a.PointsPossible == nil || *a.PointsPossible <= 0 {
			continue
		}
		points := *a.PointsPossible
		finalPossible += points

		sub, exists := subByAssignment[a.ID]
		if exists && sub.Score != nil {
			currentEarned += *sub.Score
			currentPossible += points
			finalEarned += *sub.Score
		}
	}

	var currentScore, finalScore float64
	if currentPossible > 0 {
		currentScore = math.Round(currentEarned/currentPossible*10000) / 100
	}
	if finalPossible > 0 {
		finalScore = math.Round(finalEarned/finalPossible*10000) / 100
	}

	return &StudentGrade{
		CurrentGrade: scoreToLetterGradeWithScale(currentScore, scale),
		CurrentScore: currentScore,
		FinalGrade:   scoreToLetterGradeWithScale(finalScore, scale),
		FinalScore:   finalScore,
	}, nil
}

// calculateWeightedGrade computes Canvas-compatible weighted grades.
// Current score: only considers groups where the student has graded submissions.
// Final score: treats unsubmitted assignments as 0.
func (s *GradingService) calculateWeightedGrade(assignments []models.Assignment, groups []models.AssignmentGroup, subByAssignment map[uint]*models.Submission) (currentScore, finalScore float64) {
	// Build group lookup
	groupMap := make(map[uint]*models.AssignmentGroup)
	for i := range groups {
		groupMap[groups[i].ID] = &groups[i]
	}

	// Per-group earned/possible totals
	type groupTotals struct {
		currentEarned, currentPossible float64
		finalEarned, finalPossible     float64
		weight                         float64
	}
	totals := make(map[uint]*groupTotals)

	for _, a := range assignments {
		if a.PointsPossible == nil || *a.PointsPossible <= 0 {
			continue
		}
		groupID := uint(0)
		if a.AssignmentGroupID != nil {
			groupID = *a.AssignmentGroupID
		}
		g, ok := groupMap[groupID]
		if !ok || g.GroupWeight <= 0 {
			continue // Skip assignments in groups with no weight
		}

		gt, exists := totals[groupID]
		if !exists {
			gt = &groupTotals{weight: g.GroupWeight}
			totals[groupID] = gt
		}

		points := *a.PointsPossible
		gt.finalPossible += points

		sub, hasSub := subByAssignment[a.ID]
		if hasSub && sub.Score != nil {
			gt.currentEarned += *sub.Score
			gt.currentPossible += points
			gt.finalEarned += *sub.Score
		}
	}

	// Calculate weighted averages
	var currentWeightedSum, currentWeightTotal float64
	var finalWeightedSum, finalWeightTotal float64

	for _, gt := range totals {
		if gt.currentPossible > 0 {
			groupPct := gt.currentEarned / gt.currentPossible * 100
			currentWeightedSum += groupPct * gt.weight
			currentWeightTotal += gt.weight
		}
		if gt.finalPossible > 0 {
			groupPct := gt.finalEarned / gt.finalPossible * 100
			finalWeightedSum += groupPct * gt.weight
			finalWeightTotal += gt.weight
		}
	}

	if currentWeightTotal > 0 {
		currentScore = math.Round(currentWeightedSum/currentWeightTotal*100) / 100
	}
	if finalWeightTotal > 0 {
		finalScore = math.Round(finalWeightedSum/finalWeightTotal*100) / 100
	}
	return
}

// defaultGradingScale is the standard US grading scale used when no custom
// grading standard is configured for a course.
var defaultGradingScale = []GradeEntry{
	{"A", 0.93},
	{"A-", 0.90},
	{"B+", 0.87},
	{"B", 0.83},
	{"B-", 0.80},
	{"C+", 0.77},
	{"C", 0.73},
	{"C-", 0.70},
	{"D+", 0.67},
	{"D", 0.63},
	{"D-", 0.60},
	{"F", 0.0},
}

// scoreToLetterGrade converts a percentage (0–100) to a letter grade using the
// default grading scale. Kept for backward compatibility with tests.
func scoreToLetterGrade(score float64) string {
	return scoreToLetterGradeWithScale(score, nil)
}

// scoreToLetterGradeWithScale converts a percentage (0–100) to a letter grade.
// If scale is nil, the default grading scale is used.
// The scale entries must be sorted descending by Value.
func scoreToLetterGradeWithScale(score float64, scale []GradeEntry) string {
	if scale == nil {
		scale = defaultGradingScale
	}
	// Convert score from 0–100 to 0.0–1.0 for comparison
	pct := score / 100.0
	for _, entry := range scale {
		if pct >= entry.Value {
			return entry.Name
		}
	}
	// If we fall through (shouldn't happen with a well-formed scale), return the last entry
	if len(scale) > 0 {
		return scale[len(scale)-1].Name
	}
	return "F"
}

// getCourseGradingScale looks up the active grading standard for a course and
// parses it into a []GradeEntry. Returns nil if no custom standard exists.
// The Data field stores a Canvas-format JSON array of [name, value] pairs,
// e.g. [["A", 0.94], ["A-", 0.90], ...]
func (s *GradingService) getCourseGradingScale(ctx context.Context, courseID uint) []GradeEntry {
	if s.gradingStandardRepo == nil {
		return nil
	}
	standard, err := s.gradingStandardRepo.FindActiveByCourse(ctx, courseID)
	if err != nil || standard == nil {
		return nil
	}
	return parseGradingStandardData(standard.Data)
}

// parseGradingStandardData parses the JSONB data from a GradingStandard.
// Supports Canvas format: [["A", 0.94], ["A-", 0.90], ...]
func parseGradingStandardData(data string) []GradeEntry {
	var raw [][]interface{}
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		return nil
	}
	entries := make([]GradeEntry, 0, len(raw))
	for _, pair := range raw {
		if len(pair) < 2 {
			continue
		}
		name, ok := pair[0].(string)
		if !ok {
			continue
		}
		var value float64
		switch v := pair[1].(type) {
		case float64:
			value = v
		case json.Number:
			val, err := v.Float64()
			if err != nil {
				continue
			}
			value = val
		default:
			continue
		}
		entries = append(entries, GradeEntry{Name: name, Value: value})
	}
	if len(entries) == 0 {
		return nil
	}
	return entries
}
