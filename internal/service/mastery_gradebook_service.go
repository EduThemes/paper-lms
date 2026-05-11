package service

import (
	"context"
	"sort"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// MasteryGradebookService computes the Learning Mastery Gradebook for a course:
// students × outcomes, with the current proficiency level for each cell.
type MasteryGradebookService struct {
	enrollmentRepo repository.EnrollmentRepository
	outcomeRepo    repository.LearningOutcomeRepository
	resultRepo     repository.LearningOutcomeResultRepository
	userRepo       repository.UserRepository
	proficiency    *OutcomeProficiencyService
}

func NewMasteryGradebookService(
	enrollmentRepo repository.EnrollmentRepository,
	outcomeRepo repository.LearningOutcomeRepository,
	resultRepo repository.LearningOutcomeResultRepository,
	userRepo repository.UserRepository,
	proficiency *OutcomeProficiencyService,
) *MasteryGradebookService {
	return &MasteryGradebookService{
		enrollmentRepo: enrollmentRepo,
		outcomeRepo:    outcomeRepo,
		resultRepo:     resultRepo,
		userRepo:       userRepo,
		proficiency:    proficiency,
	}
}

// MasteryCell is a single student × outcome data point.
type MasteryCell struct {
	UserID    uint                              `json:"user_id"`
	OutcomeID uint                              `json:"outcome_id"`
	Score     *float64                          `json:"score,omitempty"`
	Possible  *float64                          `json:"possible,omitempty"`
	Rating    *models.OutcomeProficiencyRating  `json:"rating,omitempty"`
}

// MasteryStudent is a student row in the gradebook.
type MasteryStudent struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// MasteryOutcome is an outcome column in the gradebook.
type MasteryOutcome struct {
	ID            uint    `json:"id"`
	Title         string  `json:"title"`
	DisplayName   string  `json:"display_name"`
	MasteryPoints float64 `json:"mastery_points"`
	PointsPossible float64 `json:"points_possible"`
}

// MasteryGradebook is the response shape returned by GetMasteryGradebook.
type MasteryGradebook struct {
	CourseID    uint                       `json:"course_id"`
	Proficiency *models.OutcomeProficiency `json:"proficiency"`
	Students    []MasteryStudent           `json:"students"`
	Outcomes    []MasteryOutcome           `json:"outcomes"`
	Cells       []MasteryCell              `json:"cells"`
}

// GetMasteryGradebook builds the Learning Mastery Gradebook for a course. It
// computes one cell per (student, outcome) using the most recent
// LearningOutcomeResult for that pair and resolves the proficiency level using
// the course's proficiency scale.
func (s *MasteryGradebookService) GetMasteryGradebook(ctx context.Context, courseID uint) (*MasteryGradebook, error) {
	// Resolve scale.
	scale, err := s.proficiency.Get(ctx, "Course", courseID)
	if err != nil {
		return nil, err
	}

	// Load all student enrollments for the course (paginate through pages).
	students := []MasteryStudent{}
	studentIDs := []uint{}
	page := 1
	for {
		res, err := s.enrollmentRepo.ListByCourseID(ctx, courseID, repository.PaginationParams{Page: page, PerPage: 100})
		if err != nil {
			return nil, err
		}
		for _, e := range res.Items {
			if e.Type != "StudentEnrollment" {
				continue
			}
			studentIDs = append(studentIDs, e.UserID)
		}
		if int64(page*100) >= res.TotalCount || len(res.Items) == 0 {
			break
		}
		page++
	}

	if len(studentIDs) > 0 {
		users, err := s.userRepo.FindByIDs(ctx, studentIDs)
		if err != nil {
			return nil, err
		}
		for _, u := range users {
			students = append(students, MasteryStudent{ID: u.ID, Name: u.Name, Email: u.Email})
		}
	}

	// Load all outcomes for the course.
	outcomes := []MasteryOutcome{}
	outcomeIDs := []uint{}
	page = 1
	for {
		res, err := s.outcomeRepo.ListByContext(ctx, "Course", courseID, repository.PaginationParams{Page: page, PerPage: 100})
		if err != nil {
			return nil, err
		}
		for _, o := range res.Items {
			outcomes = append(outcomes, MasteryOutcome{
				ID:             o.ID,
				Title:          o.Title,
				DisplayName:    o.DisplayName,
				MasteryPoints:  o.MasteryPoints,
				PointsPossible: o.PointsPossible,
			})
			outcomeIDs = append(outcomeIDs, o.ID)
		}
		if int64(page*100) >= res.TotalCount || len(res.Items) == 0 {
			break
		}
		page++
	}

	// For each student, fetch their results in this course context and pick the
	// latest score per outcome.
	cells := []MasteryCell{}
	for _, st := range students {
		results, err := s.resultRepo.ListByUserAndContext(ctx, st.ID, "Course", courseID)
		if err != nil {
			continue
		}
		// pick latest result per outcome
		sort.Slice(results, func(i, j int) bool {
			ai := results[i].AssessedAt
			aj := results[j].AssessedAt
			if ai == nil && aj == nil {
				return results[i].UpdatedAt.After(results[j].UpdatedAt)
			}
			if ai == nil {
				return false
			}
			if aj == nil {
				return true
			}
			return ai.After(*aj)
		})
		seen := map[uint]bool{}
		for _, r := range results {
			if seen[r.LearningOutcomeID] {
				continue
			}
			seen[r.LearningOutcomeID] = true
			cell := MasteryCell{UserID: st.ID, OutcomeID: r.LearningOutcomeID, Score: r.Score, Possible: r.Possible}
			if r.Score != nil {
				cell.Rating = s.proficiency.LevelFor(*r.Score, scale)
			}
			cells = append(cells, cell)
		}
		// also emit empty cells for outcomes the student has no result for, so
		// the frontend can render the full grid without extra logic.
		for _, oid := range outcomeIDs {
			if !seen[oid] {
				cells = append(cells, MasteryCell{UserID: st.ID, OutcomeID: oid})
			}
		}
	}

	return &MasteryGradebook{
		CourseID:    courseID,
		Proficiency: scale,
		Students:    students,
		Outcomes:    outcomes,
		Cells:       cells,
	}, nil
}
