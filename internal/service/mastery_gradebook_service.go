package service

import (
	"context"

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
func (s *MasteryGradebookService) GetMasteryGradebook(ctx context.Context, courseID, accountID uint) (*MasteryGradebook, error) {
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
		res, err := s.enrollmentRepo.ListByCourseID(ctx, courseID, 0, repository.PaginationParams{Page: page, PerPage: 100})
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
		// Mastery gradebook: students were already tenant-scoped via
		// the enrollment-by-course lookup. Wave 2 passes 0 to match
		// pre-widening semantics; Sprint 2.3 leftover threads tenant
		// from the mastery handler.
		users, err := s.userRepo.FindByIDs(ctx, studentIDs, 0)
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
	outcomeByID := map[uint]models.LearningOutcome{}
	page = 1
	for {
		res, err := s.outcomeRepo.ListByContext(ctx, "Course", courseID, accountID, repository.PaginationParams{Page: page, PerPage: 100})
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
			outcomeByID[o.ID] = o
		}
		if int64(page*100) >= res.TotalCount || len(res.Items) == 0 {
			break
		}
		page++
	}

	// For each student, fetch their results in this course context and roll
	// them up per outcome (using the outcome's calculation method), on the
	// proficiency scale's point range.
	cells := []MasteryCell{}
	for _, st := range students {
		results, err := s.resultRepo.ListByUserAndContext(ctx, st.ID, "Course", courseID)
		if err != nil {
			continue
		}
		byOutcome := map[uint][]models.LearningOutcomeResult{}
		for _, r := range results {
			byOutcome[r.LearningOutcomeID] = append(byOutcome[r.LearningOutcomeID], r)
		}
		for _, oid := range outcomeIDs {
			cell := MasteryCell{UserID: st.ID, OutcomeID: oid}
			if rs := byOutcome[oid]; len(rs) > 0 {
				cell.Score, cell.Possible, cell.Rating = s.masteryCellScore(rs, outcomeByID[oid], scale)
			}
			// Empty cells are emitted too so the frontend can render the
			// full grid without extra logic.
			cells = append(cells, cell)
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

// masteryCellScore rolls a student's results for one outcome up into a
// single score on the proficiency scale's point range and rates it.
//
// B091: results store raw asset points (e.g. 12/100) while proficiency
// ratings are on the scale's own range (e.g. 0-4). Rating the raw score made
// any score above the top threshold "Exceeds Mastery". Like Canvas (whose
// account mastery scales define the outcome's points_possible and
// mastery_points), each result is converted to a fraction of its possible
// points and re-expressed on the scale before the calculation method runs.
func (s *MasteryGradebookService) masteryCellScore(results []models.LearningOutcomeResult, outcome models.LearningOutcome, scale *models.OutcomeProficiency) (*float64, *float64, *models.OutcomeProficiencyRating) {
	onScale := outcome
	var possible *float64
	if scale != nil && len(scale.Ratings) > 0 {
		var maxPts float64
		mastery := -1.0
		for _, r := range scale.Ratings {
			if r.Points > maxPts {
				maxPts = r.Points
			}
			if r.Mastery {
				mastery = r.Points
			}
		}
		if maxPts > 0 {
			onScale.PointsPossible = maxPts
			if mastery >= 0 {
				onScale.MasteryPoints = mastery
			}
			possible = &maxPts
		}
	}

	normalized := make([]models.LearningOutcomeResult, 0, len(results))
	for _, r := range results {
		if r.Score == nil {
			continue
		}
		// A result without score/possible is already on the outcome's own
		// scale; express it as a fraction of that so it re-scales too.
		if _, ok := resultPercent(&r); !ok && outcome.PointsPossible > 0 {
			pct := *r.Score / outcome.PointsPossible
			r.Percent = &pct
		}
		normalized = append(normalized, r)
	}

	score, ok := RollupOutcomeScore(normalized, &onScale)
	if !ok {
		return nil, possible, nil
	}
	if possible == nil && onScale.PointsPossible > 0 {
		pp := onScale.PointsPossible
		possible = &pp
	}
	// LevelFor reads only its arguments, so a nil proficiency service is safe.
	return &score, possible, s.proficiency.LevelFor(score, scale)
}
