package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
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

func (s *LearningOutcomeService) GetGroup(ctx context.Context, id, accountID uint) (*models.LearningOutcomeGroup, error) {
	return s.groupRepo.FindByID(ctx, id, accountID)
}

func (s *LearningOutcomeService) UpdateGroup(ctx context.Context, group *models.LearningOutcomeGroup) error {
	return s.groupRepo.Update(ctx, group)
}

// DeleteGroup soft-deletes the outcome group. F-012 — accountID is
// the caller's tenant; when 0 the underlying repo skips the tenant
// filter (auth-internal contract per internal/repository/postgres/user.go).
func (s *LearningOutcomeService) DeleteGroup(ctx context.Context, id, accountID uint) error {
	return s.groupRepo.Delete(ctx, id, accountID)
}

func (s *LearningOutcomeService) ListGroups(ctx context.Context, contextType string, contextID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LearningOutcomeGroup], error) {
	return s.groupRepo.ListByContext(ctx, contextType, contextID, accountID, params)
}

func (s *LearningOutcomeService) GetRootGroup(ctx context.Context, contextType string, contextID, accountID uint) (*models.LearningOutcomeGroup, error) {
	return s.groupRepo.FindRootGroup(ctx, contextType, contextID, accountID)
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
	defaultPointsPossibleFromRatings(outcome)
	return s.outcomeRepo.Create(ctx, outcome)
}

// defaultPointsPossibleFromRatings mirrors Canvas, where an outcome's
// points_possible is its highest rating. Results are scaled onto
// points_possible (B090), so leaving it 0 would collapse every result onto
// the mastery threshold.
func defaultPointsPossibleFromRatings(outcome *models.LearningOutcome) {
	if outcome.PointsPossible > 0 || outcome.RatingsData == "" {
		return
	}
	var ratings []struct {
		Points float64 `json:"points"`
	}
	if err := json.Unmarshal([]byte(outcome.RatingsData), &ratings); err != nil {
		return
	}
	for _, r := range ratings {
		if r.Points > outcome.PointsPossible {
			outcome.PointsPossible = r.Points
		}
	}
}

func (s *LearningOutcomeService) GetOutcome(ctx context.Context, id, accountID uint) (*models.LearningOutcome, error) {
	return s.outcomeRepo.FindByID(ctx, id, accountID)
}

func (s *LearningOutcomeService) UpdateOutcome(ctx context.Context, outcome *models.LearningOutcome) error {
	defaultPointsPossibleFromRatings(outcome)
	return s.outcomeRepo.Update(ctx, outcome)
}

// DeleteOutcome soft-deletes the outcome. F-012 — accountID is the
// caller's tenant; when 0 the underlying repo skips the tenant filter
// (auth-internal contract per internal/repository/postgres/user.go).
func (s *LearningOutcomeService) DeleteOutcome(ctx context.Context, id, accountID uint) error {
	return s.outcomeRepo.Delete(ctx, id, accountID)
}

func (s *LearningOutcomeService) ListOutcomes(ctx context.Context, groupID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LearningOutcome], error) {
	return s.outcomeRepo.ListByGroupID(ctx, groupID, accountID, params)
}

func (s *LearningOutcomeService) ListOutcomesByContext(ctx context.Context, contextType string, contextID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LearningOutcome], error) {
	return s.outcomeRepo.ListByContext(ctx, contextType, contextID, accountID, params)
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

	// Determine mastery. B090: the raw score is in the ASSET's units
	// (e.g. 10/100 assignment points) while mastery_points is on the
	// OUTCOME's scale (e.g. 3 of 5). Canvas scales the result onto the
	// outcome scale (percent * points_possible) before comparing, so a
	// 10/100 must not count as mastering a 3/5 outcome.
	if result.Score != nil {
		outcome, err := s.outcomeRepo.FindByID(ctx, result.LearningOutcomeID, 0)
		if err == nil {
			mastery := ScaledOutcomeScore(result, outcome) >= outcome.MasteryPoints
			result.Mastery = &mastery
		}
	}
	if result.AssessedAt == nil {
		now := time.Now()
		result.AssessedAt = &now
	}

	// Upsert returns the atomic pre-write Mastery value (captured inside
	// the same row-locked transaction that performs the write). priorMastery
	// nil means either no prior row existed or the prior row's Mastery was
	// nil — either way "not yet mastered" for transition-detection purposes.
	priorMastery, err := s.resultRepo.Upsert(ctx, result)
	if err != nil {
		return err
	}

	wasMastered := priorMastery != nil && *priorMastery
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
func (s *LearningOutcomeService) GetMasteryGradebook(ctx context.Context, courseID, accountID uint, params repository.PaginationParams) ([]MasteryRollup, []models.LearningOutcome, error) {
	return s.getMasteryGradebook(ctx, courseID, accountID, nil, params)
}

// GetMasteryGradebookForUsers is GetMasteryGradebook restricted to the given
// students (B087: a student sees only their own rollups, an observer only
// their linked students'). An empty userIDs slice yields no rollups.
func (s *LearningOutcomeService) GetMasteryGradebookForUsers(ctx context.Context, courseID, accountID uint, userIDs []uint, params repository.PaginationParams) ([]MasteryRollup, []models.LearningOutcome, error) {
	only := make(map[uint]bool, len(userIDs))
	for _, id := range userIDs {
		only[id] = true
	}
	return s.getMasteryGradebook(ctx, courseID, accountID, only, params)
}

// getMasteryGradebook builds the rollups; onlyUsers==nil means every student.
func (s *LearningOutcomeService) getMasteryGradebook(ctx context.Context, courseID, accountID uint, onlyUsers map[uint]bool, params repository.PaginationParams) ([]MasteryRollup, []models.LearningOutcome, error) {
	// Get all outcomes for the course
	allOutcomes, err := s.outcomeRepo.ListByContext(ctx, "Course", courseID, accountID, repository.PaginationParams{Page: 1, PerPage: 1000})
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
			if onlyUsers != nil && !onlyUsers[r.UserID] {
				continue
			}
			allResultsByUser[r.UserID] = append(allResultsByUser[r.UserID], r)
		}
	}

	// Build rollups for each student, paginated
	// B195: map iteration order is randomized, so slicing an unsorted key
	// list into pages returned a different random subset on every request
	// (duplicating some students and skipping others). Sort for a stable
	// page order.
	userIDs := make([]uint, 0, len(allResultsByUser))
	for uid := range allResultsByUser {
		userIDs = append(userIDs, uid)
	}
	sort.Slice(userIDs, func(i, j int) bool { return userIDs[i] < userIDs[j] })

	// Apply pagination to user list
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PerPage < 1 {
		params.PerPage = 10
	}
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
				// Calculate rollup score based on the outcome's calculation
				// method, on the outcome's own point scale (B090).
				if rollupScore, ok := RollupOutcomeScore(outcomeResults, &outcome); ok {
					score.Score = &rollupScore
					mastery := rollupScore >= outcome.MasteryPoints
					score.Mastery = &mastery
				} else {
					// e.g. n_mastery with fewer than n mastering results:
					// Canvas reports no score; the outcome is not mastered.
					mastery := false
					score.Mastery = &mastery
				}
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

// ScaledOutcomeScore returns a result's score on the outcome's own point
// scale, mirroring Canvas RollupScoreAggregatorHelper#result_score: when the
// result carries a percent (score / possible) it is multiplied by the
// outcome's points_possible (falling back to mastery_points); otherwise the
// raw score is assumed to already be on the outcome scale.
func ScaledOutcomeScore(r *models.LearningOutcomeResult, outcome *models.LearningOutcome) float64 {
	if r == nil || r.Score == nil {
		return 0
	}
	pct, ok := resultPercent(r)
	if !ok || outcome == nil {
		return *r.Score
	}
	switch {
	case outcome.PointsPossible > 0:
		return pct * outcome.PointsPossible
	case outcome.MasteryPoints > 0:
		return pct * outcome.MasteryPoints
	default:
		return *r.Score
	}
}

// resultPercent is the result's fraction of its own possible points.
func resultPercent(r *models.LearningOutcomeResult) (float64, bool) {
	if r.Percent != nil {
		return *r.Percent, true
	}
	if r.Score != nil && r.Possible != nil && *r.Possible > 0 {
		return *r.Score / *r.Possible, true
	}
	return 0, false
}

// resultTime orders results the way Canvas does (submitted_at, then
// assessed_at), falling back to the row timestamps.
func resultTime(r *models.LearningOutcomeResult) time.Time {
	if r.SubmittedAt != nil {
		return *r.SubmittedAt
	}
	if r.AssessedAt != nil {
		return *r.AssessedAt
	}
	if !r.UpdatedAt.IsZero() {
		return r.UpdatedAt
	}
	return r.CreatedAt
}

// RollupOutcomeScore aggregates one student's results for one outcome using
// the outcome's calculation method, with every score scaled onto the
// outcome's point scale first. ok=false means the method yields no score
// (Canvas returns nil), e.g. n_mastery with fewer than n mastering results.
func RollupOutcomeScore(results []models.LearningOutcomeResult, outcome *models.LearningOutcome) (float64, bool) {
	scaled := make([]models.LearningOutcomeResult, 0, len(results))
	for _, r := range results {
		if r.Score == nil {
			continue
		}
		v := ScaledOutcomeScore(&r, outcome)
		r.Score = &v
		scaled = append(scaled, r)
	}
	return calculateRollupScore(scaled, outcome.CalculationMethod, outcome.CalculationInt, outcome.MasteryPoints)
}

// calculateRollupScore computes the aggregated score for one outcome,
// matching Canvas app/models/rollup_score.rb. Scores must already be on the
// outcome's scale. Results may be passed in any order; they are ordered
// newest first here. ok=false means "no score" (Canvas nil).
func calculateRollupScore(results []models.LearningOutcomeResult, method string, calcInt int, masteryPoints float64) (float64, bool) {
	scored := make([]models.LearningOutcomeResult, 0, len(results))
	for _, r := range results {
		if r.Score != nil {
			scored = append(scored, r)
		}
	}
	if len(scored) == 0 {
		return 0, false
	}
	sort.SliceStable(scored, func(i, j int) bool {
		return resultTime(&scored[i]).After(resultTime(&scored[j]))
	})
	scores := make([]float64, len(scored)) // newest first
	for i, r := range scored {
		scores[i] = *r.Score
	}

	switch method {
	case "latest":
		return round2(scores[0]), true

	case "highest":
		highest := scores[0]
		for _, s := range scores[1:] {
			if s > highest {
				highest = s
			}
		}
		return round2(highest), true

	case "average":
		return round2(mean(scores)), true

	case "n_mastery":
		// Canvas n_mastery_set: only scores at or above mastery_points
		// count toward mastery; at least n of them are required and the
		// rollup is the average of those mastering scores (B194).
		n := calcInt
		if n <= 0 {
			n = 3
		}
		var mastering []float64
		for _, s := range scores {
			if s >= masteryPoints {
				mastering = append(mastering, s)
			}
		}
		if len(mastering) < n {
			return 0, false
		}
		return round2(mean(mastering)), true

	case "standard_decaying_average":
		// Canvas standard_decaying_average: fold oldest -> newest, each
		// newer score weighted at decay_rate%.
		rate := float64(calcInt)
		if rate <= 0 || rate >= 100 {
			rate = 65
		}
		decay := scores[len(scores)-1]
		for i := len(scores) - 2; i >= 0; i-- {
			decay = decay*((100-rate)/100) + scores[i]*(rate/100)
		}
		return round2(decay), true

	default:
		// "decaying_average" (Paper's default) / "weighted_average":
		// Canvas decaying_average_set weights the newest score at
		// calcInt% and the mean of all older scores at (100-calcInt)%.
		weight := float64(calcInt) / 100.0
		if weight <= 0 || weight >= 1 {
			weight = 0.65
		}
		if len(scores) == 1 {
			return round2(scores[0]), true
		}
		return round2(weight*scores[0] + (1-weight)*mean(scores[1:])), true
	}
}

func mean(xs []float64) float64 {
	var sum float64
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}

