package gamification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service/gamification/pseudonym"
)

// ErrCourseLeaderboardNotEnrolled is returned when a non-admin / non-teacher
// caller tries to access a course leaderboard without an active enrollment.
var ErrCourseLeaderboardNotEnrolled = errors.New("not enrolled in this course")

// ErrUnknownCurrency is returned when the requested currency code doesn't
// resolve at site scope.
var ErrUnknownCurrency = errors.New("unknown currency")

// ErrSnapshotNotFound is returned when a snapshot read misses for the
// requested window.
var ErrSnapshotNotFound = errors.New("snapshot not found")

// ErrPseudonymSwitchForbidden is returned when the render policy disallows a
// learner-side pseudonym switch (K-5/M68 tenant mode).
var ErrPseudonymSwitchForbidden = errors.New("pseudonym switching is not allowed in this course")

// ErrFirstNameNotAllowed is returned when first-name mode is disabled in the
// tenant render policy.
var ErrFirstNameNotAllowed = errors.New("first-name mode is not allowed in this course")

// LeaderboardService is the live + snapshot leaderboard renderer. It
// orchestrates the role resolution, opt-out filter, currency resolution,
// rendering policy (top-N gate / pseudonym layer / first-name eligibility)
// and the relative-window mechanic with filler peers.
type LeaderboardService struct {
	walletRepo     repository.GamificationWalletRepository
	currencyRepo   repository.GamificationCurrencyTypeRepository
	userRepo       repository.UserRepository
	enrollmentRepo repository.EnrollmentRepository
	accountRepo    repository.AccountRepository
	snapshotRepo   repository.GamificationLeaderboardSnapshotRepository
}

// NewLeaderboardService wires the service.
func NewLeaderboardService(
	walletRepo repository.GamificationWalletRepository,
	currencyRepo repository.GamificationCurrencyTypeRepository,
	userRepo repository.UserRepository,
	enrollmentRepo repository.EnrollmentRepository,
	accountRepo repository.AccountRepository,
	snapshotRepo repository.GamificationLeaderboardSnapshotRepository,
) *LeaderboardService {
	return &LeaderboardService{
		walletRepo:     walletRepo,
		currencyRepo:   currencyRepo,
		userRepo:       userRepo,
		enrollmentRepo: enrollmentRepo,
		accountRepo:    accountRepo,
		snapshotRepo:   snapshotRepo,
	}
}

// LeaderboardRow is the per-row payload the handler serializes. Fillers
// carry zero Rank/UserID; real rows carry the pseudonym-substituted Name.
type LeaderboardRow struct {
	Rank           int
	UserID         uint
	Name           string
	LifetimeEarned int64
	IsViewer       bool
}

// NextToBeatInfo wraps the optional motivational callout.
type NextToBeatInfo struct {
	Name           string
	LifetimeEarned int64
	Gap            int64
	CurrencyLabel  string
}

// CourseLeaderboard is the full live or snapshot leaderboard render output.
type CourseLeaderboard struct {
	CourseID        uint
	Currency        *models.GamificationCurrencyType
	TotalCandidates int
	Rows            []LeaderboardRow
	ViewerRank      int
	WindowKind      string // "top_n" | "full" | "relative" | "snapshot_weekly"
	UsedPseudonyms  bool
	NextToBeat      *NextToBeatInfo
	Limit           int
	Offset          int
}

// CourseLeaderboardInput packs the query knobs.
type CourseLeaderboardInput struct {
	CourseID     uint
	ViewerID     uint
	IsAdminFlag  bool // raw from c.Locals; the service does its own userRepo fallback
	TenantID     uint
	CurrencyCode string
	OffsetWeeks  int
	Limit        int
	Offset       int
}

// ViewerRoleResolution returns the resolved role.
type viewerRoleResolution struct {
	role ViewerRole
	// notEnrolled signals a 403 condition (caller-facing).
	notEnrolled bool
}

// resolveViewerRoleInCourse decides whether the viewer is admin, teacher/TA,
// or student in the course. Mirrors the handler's prior helper. Returns
// ErrCourseLeaderboardNotEnrolled when the viewer is neither admin nor an
// active enrolled member.
func (s *LeaderboardService) resolveViewerRoleInCourse(ctx context.Context, viewerID, courseID, tenantID uint, isAdminLocals bool) (ViewerRole, error) {
	isAdmin := isAdminLocals
	if !isAdmin && s.userRepo != nil {
		// AUTH-INTERNAL: viewer-role fallback. viewerID is the JWT
		// subject; role check is tenant-independent. accountID=0 is
		// correct (matches RequireAdmin middleware pattern).
		if user, err := s.userRepo.FindByID(ctx, viewerID, 0); err == nil && user != nil && user.Role == "admin" {
			isAdmin = true
		}
	}
	if isAdmin {
		return ViewerAdmin, nil
	}
	enrollment, err := s.enrollmentRepo.FindByUserAndCourse(ctx, viewerID, courseID, tenantID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	if enrollment == nil || enrollment.WorkflowState != "active" {
		return "", ErrCourseLeaderboardNotEnrolled
	}
	if enrollment.Type == "TeacherEnrollment" || enrollment.Type == "TaEnrollment" {
		return ViewerTeacher, nil
	}
	return ViewerStudent, nil
}

// tenantMode loads the account's tenant_mode for render-policy decisions.
// Falls back to "higher_ed" (the model SQL default) on any error — surfaces
// real names, the safest default.
func (s *LeaderboardService) tenantMode(ctx context.Context, tenantID uint) string {
	if s.accountRepo == nil {
		return "higher_ed"
	}
	acc, err := s.accountRepo.FindByID(ctx, tenantID)
	if err == nil && acc != nil && acc.TenantMode != "" {
		return string(acc.TenantMode)
	}
	return "higher_ed"
}

// GetCourseLeaderboard renders the live or snapshot leaderboard for a course.
func (s *LeaderboardService) GetCourseLeaderboard(ctx context.Context, in CourseLeaderboardInput) (*CourseLeaderboard, error) {
	role, err := s.resolveViewerRoleInCourse(ctx, in.ViewerID, in.CourseID, in.TenantID, in.IsAdminFlag)
	if err != nil {
		return nil, err
	}

	// Currency resolution. Default to "xp" — Wave 1 seed guarantees this.
	currencyCode := in.CurrencyCode
	if currencyCode == "" {
		currencyCode = "xp"
	}
	currency, err := s.currencyRepo.FindByCode(ctx, in.TenantID, models.ScopeSite, in.TenantID, currencyCode)
	if err != nil {
		return nil, err
	}
	if currency == nil {
		return nil, fmt.Errorf("%w: %s", ErrUnknownCurrency, currencyCode)
	}

	if in.OffsetWeeks >= 1 {
		return s.serveSnapshotLeaderboard(ctx, in.CourseID, currency, role, in.ViewerID, in.TenantID, in.OffsetWeeks)
	}

	tenantMode := s.tenantMode(ctx, in.TenantID)

	enrollments, err := s.enrollmentRepo.ListActiveStudentEnrollmentsByCourse(ctx, in.CourseID, in.TenantID)
	if err != nil {
		return nil, err
	}
	candidates := make([]uint, 0, len(enrollments))
	enrollmentByUser := make(map[uint]models.Enrollment, len(enrollments))
	for _, e := range enrollments {
		candidates = append(candidates, e.UserID)
		enrollmentByUser[e.UserID] = e
	}
	visible, err := s.userRepo.FilterPublicLeaderboardCandidates(ctx, candidates)
	if err != nil {
		return nil, err
	}
	ranked, err := s.walletRepo.RankByCurrency(ctx, currency.ID, visible)
	if err != nil {
		return nil, err
	}

	viewerRank := 0
	for _, r := range ranked {
		if r.UserID == in.ViewerID {
			viewerRank = r.Rank
			break
		}
	}

	policy := RenderPolicyFor(tenantMode, role, viewerRank)

	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := in.Offset
	if offset < 0 {
		offset = 0
	}

	out := &CourseLeaderboard{
		CourseID:        in.CourseID,
		Currency:        currency,
		TotalCandidates: len(ranked),
		ViewerRank:      viewerRank,
		UsedPseudonyms:  policy.UsePseudonyms,
		Limit:           limit,
		Offset:          offset,
	}

	switch {
	case role == ViewerStudent && !policy.ShowTopN:
		out.WindowKind = "relative"
		viewerEnrollment := enrollmentByUser[in.ViewerID]
		viewerPool := ResolveViewerPool(viewerEnrollment)
		window := ComposeRelativeWindow(ranked, in.ViewerID, viewerPool, viewerEnrollment.ID)

		idsForName := make([]uint, 0, len(window.Rows)+1)
		for _, r := range window.Rows {
			if !r.IsFiller {
				idsForName = append(idsForName, r.UserID)
			}
		}
		if window.NextToBeat != nil {
			idsForName = append(idsForName, window.NextToBeat.UserID)
		}
		nameByID := s.LoadNamesByID(ctx, idsForName, in.TenantID)

		out.Rows = s.BuildRelativeRowsWithNames(window.Rows, enrollmentByUser, policy, in.ViewerID, nameByID)
		if window.NextToBeat != nil {
			ntbName := renderNameFor(window.NextToBeat.UserID, enrollmentByUser, policy, in.ViewerID, nameByID)
			out.NextToBeat = &NextToBeatInfo{
				Name:           ntbName,
				LifetimeEarned: window.NextToBeat.LifetimeEarned,
				Gap:            window.NextToBeat.Gap,
				CurrencyLabel:  currency.DisplayLabel,
			}
		}
	case policy.ShowTopN:
		out.WindowKind = "top_n"
		n := policy.TopNSize
		if n > len(ranked) {
			n = len(ranked)
		}
		out.Rows = s.BuildRows(ctx, ranked[:n], enrollmentByUser, policy, in.ViewerID, in.TenantID)
	default:
		out.WindowKind = "full"
		end := offset + limit
		if end > len(ranked) {
			end = len(ranked)
		}
		page := ranked
		if offset <= len(ranked) {
			page = ranked[offset:end]
		} else {
			page = nil
		}
		out.Rows = s.BuildRows(ctx, page, enrollmentByUser, policy, in.ViewerID, in.TenantID)
	}
	return out, nil
}

// BuildRows materializes the leaderboard slice into rows, applying pseudonym
// substitution where policy demands it. Public so the handler tests and
// callers can compose the path.
func (s *LeaderboardService) BuildRows(ctx context.Context, page []repository.RankRow, enrollmentByUser map[uint]models.Enrollment, policy LeaderboardRenderPolicy, viewerID, tenantID uint) []LeaderboardRow {
	idsForName := make([]uint, 0, len(page))
	for _, r := range page {
		idsForName = append(idsForName, r.UserID)
	}
	nameByID := s.LoadNamesByID(ctx, idsForName, tenantID)
	out := make([]LeaderboardRow, 0, len(page))
	for _, r := range page {
		legal := nameByID[r.UserID]
		display := legal
		isViewer := r.UserID == viewerID
		if policy.UsePseudonyms && !isViewer {
			display = RenderPseudonym(enrollmentByUser[r.UserID], legal)
		}
		out = append(out, LeaderboardRow{
			Rank:           r.Rank,
			UserID:         r.UserID,
			Name:           display,
			LifetimeEarned: r.LifetimeEarned,
			IsViewer:       isViewer,
		})
	}
	return out
}

// BuildRelativeRowsWithNames materializes the relative-window slice using a
// pre-loaded name map. Fillers pass through with no user_id and no rank.
func (s *LeaderboardService) BuildRelativeRowsWithNames(window []RelativeRow, enrollmentByUser map[uint]models.Enrollment, policy LeaderboardRenderPolicy, viewerID uint, nameByID map[uint]string) []LeaderboardRow {
	out := make([]LeaderboardRow, 0, len(window))
	for _, r := range window {
		if r.IsFiller {
			out = append(out, LeaderboardRow{
				Name:           r.Pseudonym,
				LifetimeEarned: r.LifetimeEarned,
			})
			continue
		}
		legal := nameByID[r.UserID]
		display := legal
		isViewer := r.UserID == viewerID
		if policy.UsePseudonyms && !isViewer {
			display = RenderPseudonym(enrollmentByUser[r.UserID], legal)
		}
		out = append(out, LeaderboardRow{
			Rank:           r.Rank,
			UserID:         r.UserID,
			Name:           display,
			LifetimeEarned: r.LifetimeEarned,
			IsViewer:       isViewer,
		})
	}
	return out
}

// LoadNamesByID is the single FindByIDs entry point. Soft-fails to an empty
// map on error so a name-lookup outage degrades to "User #N" rather than a
// 500 from the caller's path.
func (s *LeaderboardService) LoadNamesByID(ctx context.Context, ids []uint, tenantID uint) map[uint]string {
	if len(ids) == 0 {
		return map[uint]string{}
	}
	users, err := s.userRepo.FindByIDs(ctx, ids, tenantID)
	if err != nil {
		return map[uint]string{}
	}
	out := make(map[uint]string, len(users))
	for _, u := range users {
		out[u.ID] = u.Name
	}
	return out
}

// renderNameFor returns the same display string BuildRelativeRowsWithNames
// produces for userID, given the active policy.
func renderNameFor(userID uint, enrollmentByUser map[uint]models.Enrollment, policy LeaderboardRenderPolicy, viewerID uint, nameByID map[uint]string) string {
	legal := nameByID[userID]
	if policy.UsePseudonyms && userID != viewerID {
		return RenderPseudonym(enrollmentByUser[userID], legal)
	}
	return legal
}

// serveSnapshotLeaderboard reads from gamification_leaderboard_snapshots and
// re-applies the opt-out filter at read time so a learner who opted out
// POST-snapshot still vanishes from peer views.
func (s *LeaderboardService) serveSnapshotLeaderboard(ctx context.Context, courseID uint, currency *models.GamificationCurrencyType, role ViewerRole, viewerID, tenantID uint, offsetWeeks int) (*CourseLeaderboard, error) {
	_, windowEnd := WeeklyWindowForOffset(time.Now(), offsetWeeks)

	row, err := s.snapshotRepo.FindByWindow(ctx,
		models.ScopeCourse, courseID, currency.ID,
		string(models.WindowKindWeekly), windowEnd)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, fmt.Errorf("%w: window ending %s", ErrSnapshotNotFound, windowEnd.Format(time.RFC3339))
	}

	var payload []models.SnapshotRow
	if err := json.Unmarshal([]byte(row.Payload), &payload); err != nil {
		return nil, err
	}

	candidateIDs := make([]uint, 0, len(payload))
	for _, r := range payload {
		candidateIDs = append(candidateIDs, r.UserID)
	}
	visible, err := s.userRepo.FilterPublicLeaderboardCandidates(ctx, candidateIDs)
	if err != nil {
		return nil, err
	}
	visibleSet := make(map[uint]struct{}, len(visible))
	for _, id := range visible {
		visibleSet[id] = struct{}{}
	}
	filtered := make([]models.SnapshotRow, 0, len(payload))
	for _, r := range payload {
		if _, ok := visibleSet[r.UserID]; ok {
			filtered = append(filtered, r)
		}
	}
	for i := range filtered {
		filtered[i].Rank = i + 1
	}

	viewerRank := 0
	for _, r := range filtered {
		if r.UserID == viewerID {
			viewerRank = r.Rank
			break
		}
	}

	tenantMode := s.tenantMode(ctx, tenantID)
	policy := RenderPolicyFor(tenantMode, role, viewerRank)

	enrollments, err := s.enrollmentRepo.ListActiveStudentEnrollmentsByCourse(ctx, courseID, tenantID)
	if err != nil {
		return nil, err
	}
	enrollmentByUser := make(map[uint]models.Enrollment, len(enrollments))
	for _, e := range enrollments {
		enrollmentByUser[e.UserID] = e
	}

	nameByID := s.LoadNamesByID(ctx, candidateIDs, tenantID)
	rows := make([]LeaderboardRow, 0, len(filtered))
	for _, r := range filtered {
		legal := nameByID[r.UserID]
		display := legal
		isViewer := r.UserID == viewerID
		if policy.UsePseudonyms && !isViewer {
			display = RenderPseudonym(enrollmentByUser[r.UserID], legal)
		}
		rows = append(rows, LeaderboardRow{
			Rank:           r.Rank,
			UserID:         r.UserID,
			Name:           display,
			LifetimeEarned: r.LifetimeEarned,
			IsViewer:       isViewer,
		})
	}

	return &CourseLeaderboard{
		CourseID:        courseID,
		Currency:        currency,
		TotalCandidates: len(filtered),
		Rows:            rows,
		ViewerRank:      viewerRank,
		WindowKind:      "snapshot_weekly",
		UsedPseudonyms:  policy.UsePseudonyms,
		Limit:           len(rows),
		Offset:          0,
	}, nil
}

// ResolveViewerPool returns the pseudonym pool the viewer's enrollment uses.
func ResolveViewerPool(e models.Enrollment) pseudonym.Pool {
	code := pseudonym.PoolCode(e.PseudonymPoolCode)
	if code == "" || code == pseudonym.PoolFirstName {
		code = pseudonym.PoolAnimals
	}
	p, err := pseudonym.PoolByCode(code)
	if err != nil || p == nil {
		return pseudonym.Pool{Code: pseudonym.PoolAnimals}
	}
	return *p
}

// RenderPseudonym resolves a peer row's display name. Persisted alias wins;
// otherwise the deterministic FNV generator picks one from the enrollment's
// chosen pool at attempt 0. First-name mode special-cases to the legal-name
// first token. Empty fallback returns the bare legal name so a render never
// throws.
func RenderPseudonym(e models.Enrollment, legalName string) string {
	pool := pseudonym.PoolCode(e.PseudonymPoolCode)
	if pool == "" {
		pool = pseudonym.PoolAnimals
	}
	if pool == pseudonym.PoolFirstName {
		if first := pseudonym.FirstNameOf(legalName); first != "" {
			return first
		}
		return legalName
	}
	if e.PseudonymName != nil && *e.PseudonymName != "" {
		return *e.PseudonymName
	}
	p, err := pseudonym.PoolByCode(pool)
	if err != nil || p == nil {
		return legalName
	}
	return pseudonym.GenerateForEnrollment(*p, e.ID, 0)
}

// PseudonymPoolSample is the public-facing pool descriptor for the picker UI.
type PseudonymPoolSample struct {
	Code           string
	Label          string
	Description    string
	CandidateCount int
	Samples        []string
}

// PseudonymCatalogForViewer returns the pool catalog the picker UI consumes,
// gated by the tenant render policy. Returns ErrPseudonymSwitchForbidden when
// the policy disallows the switcher.
func (s *LeaderboardService) PseudonymCatalogForViewer(ctx context.Context, viewerID, courseID, tenantID uint, isAdminLocals bool) (catalog []PseudonymPoolSample, firstNameAvailable bool, current models.Enrollment, err error) {
	enrollment, err := s.enrollmentRepo.FindByUserAndCourse(ctx, viewerID, courseID, tenantID)
	if err != nil {
		return nil, false, models.Enrollment{}, err
	}
	if enrollment == nil || enrollment.WorkflowState != "active" {
		return nil, false, models.Enrollment{}, ErrCourseLeaderboardNotEnrolled
	}

	policy := s.policyForViewerInCourse(ctx, viewerID, tenantID, enrollment, isAdminLocals)
	if !policy.LearnerCanSwitch {
		return nil, false, *enrollment, ErrPseudonymSwitchForbidden
	}

	pools := make([]PseudonymPoolSample, 0, len(pseudonym.Catalog()))
	for _, p := range pseudonym.Catalog() {
		samples := make([]string, 0, 5)
		for i := 0; i < 5; i++ {
			samples = append(samples, pseudonym.GenerateForEnrollment(p, enrollment.ID, i))
		}
		pools = append(pools, PseudonymPoolSample{
			Code:           string(p.Code),
			Label:          p.Label,
			Description:    p.Description,
			CandidateCount: pseudonym.CandidateCount(p),
			Samples:        samples,
		})
	}
	return pools, policy.AllowFirstName, *enrollment, nil
}

// PseudonymUpdateRequest is the parsed PUT body.
type PseudonymUpdateRequest struct {
	PoolCode   string
	Name       string
	Regenerate bool
}

// PseudonymUpdateResult is the persisted outcome.
type PseudonymUpdateResult struct {
	PoolCode string
	Name     string
}

// UpdatePseudonymForSelf is the learner-facing switcher (W3-B).
func (s *LeaderboardService) UpdatePseudonymForSelf(ctx context.Context, viewerID, courseID, tenantID uint, isAdminLocals bool, req PseudonymUpdateRequest) (*PseudonymUpdateResult, error) {
	enrollment, err := s.enrollmentRepo.FindByUserAndCourse(ctx, viewerID, courseID, tenantID)
	if err != nil {
		return nil, err
	}
	if enrollment == nil || enrollment.WorkflowState != "active" {
		return nil, ErrCourseLeaderboardNotEnrolled
	}
	policy := s.policyForViewerInCourse(ctx, viewerID, tenantID, enrollment, isAdminLocals)
	if !policy.LearnerCanSwitch {
		return nil, ErrPseudonymSwitchForbidden
	}

	poolCode := pseudonym.PoolCode(req.PoolCode)
	if poolCode == pseudonym.PoolFirstName {
		if !policy.AllowFirstName {
			return nil, ErrFirstNameNotAllowed
		}
		if err := s.enrollmentRepo.UpdatePseudonymForSelf(ctx, viewerID, courseID, tenantID, string(poolCode), ""); err != nil {
			return nil, err
		}
		return &PseudonymUpdateResult{PoolCode: string(poolCode), Name: ""}, nil
	}

	pool, err := pseudonym.PoolByCode(poolCode)
	if err != nil || pool == nil {
		return nil, fmt.Errorf("unknown pool_code: %s", req.PoolCode)
	}

	var newName string
	if req.Regenerate {
		gen := pseudonym.NewGenerator()
		name, err := gen.Generate(ctx, *pool, enrollment.ID, func(name string, attempt int) (bool, error) {
			perr := s.enrollmentRepo.UpdatePseudonymForSelf(ctx, viewerID, courseID, tenantID, string(poolCode), name)
			if errors.Is(perr, repository.ErrPseudonymTaken) {
				return false, nil
			}
			if perr != nil {
				return false, perr
			}
			return true, nil
		})
		if err != nil {
			return nil, err
		}
		newName = name
	} else {
		if req.Name == "" {
			return nil, errors.New("name is required when regenerate is false")
		}
		if err := pseudonym.Validate(*pool, req.Name); err != nil {
			return nil, err
		}
		if err := s.enrollmentRepo.UpdatePseudonymForSelf(ctx, viewerID, courseID, tenantID, string(poolCode), req.Name); err != nil {
			return nil, err
		}
		newName = req.Name
	}
	return &PseudonymUpdateResult{PoolCode: string(poolCode), Name: newName}, nil
}

// policyForViewerInCourse condenses (role + tenantMode) lookup shared between
// the pseudonym + leaderboard surfaces.
func (s *LeaderboardService) policyForViewerInCourse(ctx context.Context, viewerID, tenantID uint, enrollment *models.Enrollment, isAdminLocals bool) LeaderboardRenderPolicy {
	role := s.roleFromAdminOrEnrollment(ctx, viewerID, enrollment, isAdminLocals)
	tenantMode := s.tenantMode(ctx, tenantID)
	return RenderPolicyFor(tenantMode, role, 0)
}

func (s *LeaderboardService) roleFromAdminOrEnrollment(ctx context.Context, viewerID uint, enrollment *models.Enrollment, isAdminLocals bool) ViewerRole {
	isAdmin := isAdminLocals
	if !isAdmin && s.userRepo != nil && viewerID > 0 {
		// AUTH-INTERNAL: viewer-role fallback. viewerID is the JWT
		// subject; role check is tenant-independent. accountID=0 is
		// correct (matches RequireAdmin middleware pattern).
		if user, err := s.userRepo.FindByID(ctx, viewerID, 0); err == nil && user != nil && user.Role == "admin" {
			isAdmin = true
		}
	}
	if isAdmin {
		return ViewerAdmin
	}
	if enrollment != nil && (enrollment.Type == "TeacherEnrollment" || enrollment.Type == "TaEnrollment") {
		return ViewerTeacher
	}
	return ViewerStudent
}
