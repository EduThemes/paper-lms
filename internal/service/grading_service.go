package service

import (
	"context"
	"encoding/json"
	"fmt"

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
	policy              *GradingPolicyService
}

// SetPolicyService routes student grades through the full grading-policy
// calculator (overrides, grading periods, final grade overrides).
func (s *GradingService) SetPolicyService(p *GradingPolicyService) { s.policy = p }

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
	// OverrideScore / OverrideGrade are the instructor's final grade
	// override, when set (G029).
	OverrideScore *float64 `json:"override_score,omitempty"`
	OverrideGrade string   `json:"override_grade,omitempty"`
	// NoCurrentScore / NoFinalScore: nothing counts yet, so the score
	// fields' zero means "no grade" (exports leave them blank).
	NoCurrentScore bool `json:"-"`
	NoFinalScore   bool `json:"-"`
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

	// Build student list (only students). A student enrolled in two
	// sections holds two StudentEnrollment rows; list them once.
	students := make([]GradebookStudent, 0)
	seenStudent := map[uint]bool{}
	for _, e := range enrollments.Items {
		if e.Type == "StudentEnrollment" && !seenStudent[e.UserID] {
			seenStudent[e.UserID] = true
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

	// The full calculator (overrides, grading periods, drop rules, final
	// grade override) when the grading-policy service is wired.
	if s.policy != nil {
		grades, err := s.policy.CourseGrades(ctx, course, CourseGradesOptions{UserIDs: []uint{studentID}})
		if err != nil {
			return nil, err
		}
		out := &StudentGrade{}
		if len(grades.Students) == 0 {
			return out, nil
		}
		g := grades.Students[0]
		if g.CurrentScore != nil {
			out.CurrentScore = *g.CurrentScore
		}
		if g.FinalScore != nil {
			out.FinalScore = *g.FinalScore
		}
		out.NoCurrentScore, out.NoFinalScore = g.CurrentScore == nil, g.FinalScore == nil
		out.CurrentGrade, out.FinalGrade = g.CurrentGrade, g.FinalGrade
		if g.OverrideScore != nil {
			out.OverrideScore = g.OverrideScore
			out.OverrideGrade = g.OverrideGrade
		}
		return out, nil
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

	// Groups carry weights and drop rules (G020); they are always read
	// when a policy service is wired, otherwise only for weighted courses.
	var groups []models.AssignmentGroup
	if course.ApplyGroupWeights || s.policy != nil {
		if res, err := s.groupRepo.ListByCourseID(ctx, courseID, repository.PaginationParams{Page: 1, PerPage: 100}); err == nil {
			groups = res.Items
		}
	}
	res := CalculateCourseGrade(CourseGradeInput{
		Assignments: assignments.Items, Groups: groups, Submissions: subByAssignment,
		ApplyGroupWeights: course.ApplyGroupWeights,
	})
	var currentScore, finalScore float64
	if res.CurrentScore != nil {
		currentScore = *res.CurrentScore
	}
	if res.FinalScore != nil {
		finalScore = *res.FinalScore
	}

	return &StudentGrade{
		CurrentGrade: scoreToLetterGradeWithScale(currentScore, scale),
		CurrentScore: currentScore,
		FinalGrade:   scoreToLetterGradeWithScale(finalScore, scale),
		FinalScore:   finalScore,
	}, nil
}

// CurrentCourseScore is the grade calculator's current score (0-100) for
// one student, the same number the Grades page and gradebook show: group
// weights when the course applies them, drop rules, extra credit and
// excused work, and only graded work counted. subByAssignment must
// hold only the submissions the viewer may see (e.g. posted grades for a
// student or observer). ok is false when nothing graded counts yet.
func CurrentCourseScore(applyGroupWeights bool, assignments []models.Assignment, groups []models.AssignmentGroup, subByAssignment map[uint]*models.Submission) (score float64, ok bool) {
	// Same calculator as the gradebook: drop rules apply too (G020).
	res := CalculateCourseGrade(CourseGradeInput{
		Assignments: assignments, Groups: groups, Submissions: subByAssignment,
		ApplyGroupWeights: applyGroupWeights,
	})
	if res.CurrentScore == nil {
		return 0, false
	}
	return *res.CurrentScore, true
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
	entries := parseGradingStandardData(standard.Data)
	// The lookup walks the scale top-down, so it must be descending.
	sortEntriesDesc(entries)
	return entries
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
