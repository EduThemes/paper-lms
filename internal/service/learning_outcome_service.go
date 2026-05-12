package service

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

// OutcomeMasteryCrossedCallback fires (asynchronously) when a
// LearningOutcomeResult row transitions from Mastery=nil/false to
// Mastery=true on the same (user_id, learning_outcome_id, asset_type,
// asset_id) composite key. Per-row transition only — rollup-level mastery
// across multiple assets is left to the OutcomeMastery predicate. Receives
// the user, outcome, and persisted result IDs. Same contract as
// SubmissionGradedCallback: detached context, no panic, no error return.
type OutcomeMasteryCrossedCallback func(ctx context.Context, userID, outcomeID, resultID uint)

type LearningOutcomeService struct {
	groupRepo   repository.LearningOutcomeGroupRepository
	outcomeRepo repository.LearningOutcomeRepository
	resultRepo  repository.LearningOutcomeResultRepository

	onMasteryCrossedCallbacks []OutcomeMasteryCrossedCallback
}

func NewLearningOutcomeService(
	groupRepo repository.LearningOutcomeGroupRepository,
	outcomeRepo repository.LearningOutcomeRepository,
	resultRepo repository.LearningOutcomeResultRepository,
) *LearningOutcomeService {
	return &LearningOutcomeService{
		groupRepo:   groupRepo,
		outcomeRepo: outcomeRepo,
		resultRepo:  resultRepo,
	}
}

// OnMasteryCrossed registers a callback to fire after a
// LearningOutcomeResult's Mastery flag flips false/nil → true on the same
// (user, outcome, asset) row. Callbacks run in fresh goroutines with a
// detached context; panics are recovered.
func (s *LearningOutcomeService) OnMasteryCrossed(cb OutcomeMasteryCrossedCallback) {
	s.onMasteryCrossedCallbacks = append(s.onMasteryCrossedCallbacks, cb)
}

func (s *LearningOutcomeService) fireOnMasteryCrossed(userID, outcomeID, resultID uint) {
	for _, cb := range s.onMasteryCrossedCallbacks {
		go func(cb OutcomeMasteryCrossedCallback) {
			defer recoverFromPanic("learning outcome OnMasteryCrossed callback")
			cb(context.Background(), userID, outcomeID, resultID)
		}(cb)
	}
}

// Group methods

func (s *LearningOutcomeService) CreateGroup(ctx context.Context, group *models.LearningOutcomeGroup) error {
	if group.Title == "" {
		return errors.New("outcome group title is required")
	}
	if group.ContextType == "" {
		return errors.New("outcome group context_type is required")
	}
	if group.ContextID == 0 {
		return errors.New("outcome group context_id is required")
	}
	if group.WorkflowState == "" {
		group.WorkflowState = "active"
	}
	return s.groupRepo.Create(ctx, group)
}

func (s *LearningOutcomeService) GetGroup(ctx context.Context, id uint) (*models.LearningOutcomeGroup, error) {
	return s.groupRepo.FindByID(ctx, id)
}

func (s *LearningOutcomeService) UpdateGroup(ctx context.Context, group *models.LearningOutcomeGroup) error {
	return s.groupRepo.Update(ctx, group)
}

func (s *LearningOutcomeService) DeleteGroup(ctx context.Context, id uint) error {
	return s.groupRepo.Delete(ctx, id)
}

func (s *LearningOutcomeService) ListGroups(ctx context.Context, contextType string, contextID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LearningOutcomeGroup], error) {
	return s.groupRepo.ListByContext(ctx, contextType, contextID, params)
}

func (s *LearningOutcomeService) GetRootGroup(ctx context.Context, contextType string, contextID uint) (*models.LearningOutcomeGroup, error) {
	return s.groupRepo.FindRootGroup(ctx, contextType, contextID)
}

// Outcome methods

func (s *LearningOutcomeService) CreateOutcome(ctx context.Context, outcome *models.LearningOutcome) error {
	if outcome.Title == "" {
		return errors.New("outcome title is required")
	}
	if outcome.ContextType == "" {
		return errors.New("outcome context_type is required")
	}
	if outcome.ContextID == 0 {
		return errors.New("outcome context_id is required")
	}
	if outcome.OutcomeGroupID == 0 {
		return errors.New("outcome outcome_group_id is required")
	}
	if outcome.WorkflowState == "" {
		outcome.WorkflowState = "active"
	}
	if outcome.CalculationMethod == "" {
		outcome.CalculationMethod = "decaying_average"
	}
	return s.outcomeRepo.Create(ctx, outcome)
}

func (s *LearningOutcomeService) GetOutcome(ctx context.Context, id uint) (*models.LearningOutcome, error) {
	return s.outcomeRepo.FindByID(ctx, id)
}

func (s *LearningOutcomeService) UpdateOutcome(ctx context.Context, outcome *models.LearningOutcome) error {
	return s.outcomeRepo.Update(ctx, outcome)
}

func (s *LearningOutcomeService) DeleteOutcome(ctx context.Context, id uint) error {
	return s.outcomeRepo.Delete(ctx, id)
}

func (s *LearningOutcomeService) ListOutcomes(ctx context.Context, groupID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LearningOutcome], error) {
	return s.outcomeRepo.ListByGroupID(ctx, groupID, params)
}

func (s *LearningOutcomeService) ListOutcomesByContext(ctx context.Context, contextType string, contextID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LearningOutcome], error) {
	return s.outcomeRepo.ListByContext(ctx, contextType, contextID, params)
}

// Result methods

func (s *LearningOutcomeService) CreateResult(ctx context.Context, result *models.LearningOutcomeResult) error {
	if result.UserID == 0 {
		return errors.New("user_id is required")
	}
	if result.LearningOutcomeID == 0 {
		return errors.New("learning_outcome_id is required")
	}

	// Calculate percent if score and possible are provided
	if result.Score != nil && result.Possible != nil && *result.Possible > 0 {
		pct := *result.Score / *result.Possible
		result.Percent = &pct
	}

	// Determine mastery if mastery_points are available and score is provided
	if result.Score != nil {
		outcome, err := s.outcomeRepo.FindByID(ctx, result.LearningOutcomeID)
		if err == nil {
			mastery := *result.Score >= outcome.MasteryPoints
			result.Mastery = &mastery
		}
	}

	// Capture the prior mastery state on the same composite key the Upsert
	// uses, so the OnMasteryCrossed callback only fires on a false/nil→true
	// transition. Lookup errors other than NotFound are ignored — we'd
	// rather miss one transition emit than fail the result write.
	wasMastered := false
	prior, err := s.resultRepo.FindByUserOutcomeAsset(
		ctx, result.UserID, result.LearningOutcomeID,
		result.AssociatedAssetType, result.AssociatedAssetID,
	)
	if err == nil && prior != nil && prior.Mastery != nil {
		wasMastered = *prior.Mastery
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		// Non-fatal: log via the gamification callback's own error path
		// instead of returning here — the result write must succeed.
		_ = err
	}

	if err := s.resultRepo.Upsert(ctx, result); err != nil {
		return err
	}

	nowMastered := result.Mastery != nil && *result.Mastery
	if nowMastered && !wasMastered {
		s.fireOnMasteryCrossed(result.UserID, result.LearningOutcomeID, result.ID)
	}
	return nil
}

func (s *LearningOutcomeService) GetResult(ctx context.Context, id uint) (*models.LearningOutcomeResult, error) {
	return s.resultRepo.FindByID(ctx, id)
}

func (s *LearningOutcomeService) ListResultsByOutcome(ctx context.Context, outcomeID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LearningOutcomeResult], error) {
	return s.resultRepo.ListByOutcomeID(ctx, outcomeID, params)
}

func (s *LearningOutcomeService) ListResultsByUserAndContext(ctx context.Context, userID uint, contextType string, contextID uint) ([]models.LearningOutcomeResult, error) {
	return s.resultRepo.ListByUserAndContext(ctx, userID, contextType, contextID)
}

// MasteryRollup represents the rollup data for a single student's outcome results.
type MasteryRollup struct {
	StudentID uint                 `json:"student_id"`
	Scores    []OutcomeRollupScore `json:"scores"`
}

// OutcomeRollupScore represents the rollup score for a single outcome.
type OutcomeRollupScore struct {
	OutcomeID uint     `json:"outcome_id"`
	Score     *float64 `json:"score"`
	Count     int      `json:"count"`
	Mastery   *bool    `json:"mastery"`
	Title     string   `json:"title"`
}

// GetMasteryGradebook returns all outcomes for a course with rollup results for all students
// who have outcome results in the course.
func (s *LearningOutcomeService) GetMasteryGradebook(ctx context.Context, courseID uint, params repository.PaginationParams) ([]MasteryRollup, []models.LearningOutcome, error) {
	// Get all outcomes for the course
	allOutcomes, err := s.outcomeRepo.ListByContext(ctx, "Course", courseID, repository.PaginationParams{Page: 1, PerPage: 1000})
	if err != nil {
		return nil, nil, err
	}

	// Collect all results across all outcomes and discover unique student IDs
	allResultsByUser := make(map[uint][]models.LearningOutcomeResult)
	for _, outcome := range allOutcomes.Items {
		results, err := s.resultRepo.ListByOutcomeID(ctx, outcome.ID, repository.PaginationParams{Page: 1, PerPage: 10000})
		if err != nil {
			continue
		}
		for _, r := range results.Items {
			allResultsByUser[r.UserID] = append(allResultsByUser[r.UserID], r)
		}
	}

	// Build rollups for each student, paginated
	var userIDs []uint
	for uid := range allResultsByUser {
		userIDs = append(userIDs, uid)
	}

	// Apply pagination to user list
	start := (params.Page - 1) * params.PerPage
	if start > len(userIDs) {
		start = len(userIDs)
	}
	end := start + params.PerPage
	if end > len(userIDs) {
		end = len(userIDs)
	}
	pagedUserIDs := userIDs[start:end]

	var rollups []MasteryRollup
	for _, userID := range pagedUserIDs {
		userResults := allResultsByUser[userID]

		// Build a map of outcome_id -> results for quick lookup
		resultsByOutcome := make(map[uint][]models.LearningOutcomeResult)
		for _, r := range userResults {
			resultsByOutcome[r.LearningOutcomeID] = append(resultsByOutcome[r.LearningOutcomeID], r)
		}

		var scores []OutcomeRollupScore
		for _, outcome := range allOutcomes.Items {
			outcomeResults := resultsByOutcome[outcome.ID]
			score := OutcomeRollupScore{
				OutcomeID: outcome.ID,
				Count:     len(outcomeResults),
				Title:     outcome.Title,
			}

			if len(outcomeResults) > 0 {
				// Calculate rollup score based on the outcome's calculation method
				rollupScore := calculateRollupScore(outcomeResults, outcome.CalculationMethod, outcome.CalculationInt)
				score.Score = &rollupScore
				mastery := rollupScore >= outcome.MasteryPoints
				score.Mastery = &mastery
			}

			scores = append(scores, score)
		}

		rollups = append(rollups, MasteryRollup{
			StudentID: userID,
			Scores:    scores,
		})
	}

	return rollups, allOutcomes.Items, nil
}

// calculateRollupScore computes the aggregated score based on the calculation method.
func calculateRollupScore(results []models.LearningOutcomeResult, method string, calcInt int) float64 {
	if len(results) == 0 {
		return 0
	}

	switch method {
	case "latest":
		// Return the most recent score
		latest := results[0]
		for _, r := range results[1:] {
			if r.CreatedAt.After(latest.CreatedAt) {
				latest = r
			}
		}
		if latest.Score != nil {
			return *latest.Score
		}
		return 0

	case "highest":
		// Return the highest score
		var highest float64
		for _, r := range results {
			if r.Score != nil && *r.Score > highest {
				highest = *r.Score
			}
		}
		return highest

	case "n_mastery":
		// Return the average of the n most recent scores where n = calcInt
		n := calcInt
		if n <= 0 {
			n = 3
		}
		// Results are ordered by created_at DESC from repo
		count := n
		if count > len(results) {
			count = len(results)
		}
		var sum float64
		var validCount int
		for i := 0; i < count; i++ {
			if results[i].Score != nil {
				sum += *results[i].Score
				validCount++
			}
		}
		if validCount == 0 {
			return 0
		}
		return sum / float64(validCount)

	case "decaying_average":
		// Decaying average: weight most recent at calcInt%, rest at (100-calcInt)%
		weight := float64(calcInt) / 100.0
		if weight <= 0 || weight >= 1 {
			weight = 0.65
		}

		// Need at least 2 results for decaying average
		if len(results) == 1 {
			if results[0].Score != nil {
				return *results[0].Score
			}
			return 0
		}

		// Results are ordered most recent first
		// Calculate average of all previous results (excluding most recent)
		var prevSum float64
		var prevCount int
		for i := 1; i < len(results); i++ {
			if results[i].Score != nil {
				prevSum += *results[i].Score
				prevCount++
			}
		}

		if prevCount == 0 {
			if results[0].Score != nil {
				return *results[0].Score
			}
			return 0
		}

		prevAvg := prevSum / float64(prevCount)
		mostRecent := 0.0
		if results[0].Score != nil {
			mostRecent = *results[0].Score
		}

		return (weight * mostRecent) + ((1 - weight) * prevAvg)

	default:
		// Default to returning the latest score
		if results[0].Score != nil {
			return *results[0].Score
		}
		return 0
	}
}
