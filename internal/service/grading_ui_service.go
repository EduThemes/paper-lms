package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// GradingUIService holds the grading-workflow features that sit on top of
// SubmissionService: anonymous grading identities, moderated grading
// (provisional grades, moderator selection, publishing), custom grade
// statuses, grader-set submission statuses, submission attempt history,
// comment editing and group comments, and peer-review housekeeping.
type GradingUIService struct {
	repo                repository.GradingUIRepository
	submissionRepo      repository.SubmissionRepository
	assignmentRepo      repository.AssignmentRepository
	courseRepo          repository.CourseRepository
	groupMembershipRepo repository.GroupMembershipRepository
	peerReviewRepo      repository.PeerReviewRepository
	submissions         *SubmissionService
}

// NewGradingUIService wires the service.
func NewGradingUIService(
	repo repository.GradingUIRepository,
	submissionRepo repository.SubmissionRepository,
	assignmentRepo repository.AssignmentRepository,
	courseRepo repository.CourseRepository,
	groupMembershipRepo repository.GroupMembershipRepository,
	peerReviewRepo repository.PeerReviewRepository,
	submissions *SubmissionService,
) *GradingUIService {
	return &GradingUIService{
		repo:                repo,
		submissionRepo:      submissionRepo,
		assignmentRepo:      assignmentRepo,
		courseRepo:          courseRepo,
		groupMembershipRepo: groupMembershipRepo,
		peerReviewRepo:      peerReviewRepo,
		submissions:         submissions,
	}
}

// Errors the handlers map to status codes.
var (
	ErrNotModerated            = errors.New("assignment is not moderated")
	ErrGradesAlreadyPublished  = errors.New("grades for this assignment have already been published")
	ErrGraderLimitReached      = errors.New("the maximum number of graders for this assignment has been reached")
	ErrNotModerator            = errors.New("only the final grader or an administrator can moderate this assignment")
	ErrProvisionalGradeInvalid = errors.New("provisional grade does not belong to this student")
	ErrCustomStatusLimit       = fmt.Errorf("an account can have at most %d custom grade statuses", models.MaxCustomGradeStatuses)
	ErrCustomStatusNotFound    = errors.New("custom grade status not found")
	ErrInvalidLatePolicyStatus = errors.New("late_policy_status must be one of late, missing, extended, none")
)

// ---------------------------------------------------------------------
// Anonymous grading
// ---------------------------------------------------------------------

const anonymousIDAlphabet = "abcdefghijkmnopqrstuvwxyz23456789"

// newAnonymousID returns a random 5-character id not in taken.
func newAnonymousID(taken map[string]bool) string {
	for {
		var b strings.Builder
		for i := 0; i < 5; i++ {
			n, err := rand.Int(rand.Reader, big.NewInt(int64(len(anonymousIDAlphabet))))
			if err != nil {
				// crypto/rand failing is fatal elsewhere too; fall back to
				// a time-derived character rather than looping forever.
				n = big.NewInt(time.Now().UnixNano() % int64(len(anonymousIDAlphabet)))
			}
			b.WriteByte(anonymousIDAlphabet[n.Int64()])
		}
		id := b.String()
		if !taken[id] {
			taken[id] = true
			return id
		}
	}
}

// AnonymizeStudents mirrors Canvas Assignment#anonymize_students?: an
// anonymously graded assignment hides student identities from graders
// until a moderated assignment's grades are published, or (unmoderated)
// until every student's submission is posted. studentIDs is the course's
// active student list; a student without a submission row counts as
// unposted.
func (s *GradingUIService) AnonymizeStudents(ctx context.Context, a *models.Assignment, studentIDs []uint) bool {
	if a == nil || !a.AnonymousGrading {
		return false
	}
	if a.ModeratedGrading {
		return !a.GradesPublished()
	}
	subs, err := s.repo.ListSubmissionsByAssignment(ctx, a.ID)
	if err != nil {
		return true
	}
	posted := make(map[uint]bool, len(subs))
	for _, sub := range subs {
		if sub.PostedAt != nil {
			posted[sub.UserID] = true
		}
	}
	if len(studentIDs) == 0 {
		if len(subs) == 0 {
			return true
		}
		for _, sub := range subs {
			if sub.PostedAt == nil {
				return true
			}
		}
		return false
	}
	for _, id := range studentIDs {
		if !posted[id] {
			return true
		}
	}
	return false
}

// EnsureSubmissionRows makes sure every student in studentIDs has a
// submission row for the assignment (an unsubmitted placeholder when
// they never submitted, as Canvas creates for every student) and that
// every row carries an anonymous id. Returns the rows keyed by user id.
func (s *GradingUIService) EnsureSubmissionRows(ctx context.Context, a *models.Assignment, studentIDs []uint) (map[uint]*models.Submission, error) {
	subs, err := s.repo.ListSubmissionsByAssignment(ctx, a.ID)
	if err != nil {
		return nil, err
	}
	byUser := make(map[uint]*models.Submission, len(subs))
	taken := make(map[string]bool, len(subs))
	for i := range subs {
		byUser[subs[i].UserID] = &subs[i]
		if subs[i].AnonymousID != nil {
			taken[*subs[i].AnonymousID] = true
		}
	}
	for _, uid := range studentIDs {
		if _, ok := byUser[uid]; ok {
			continue
		}
		anon := newAnonymousID(taken)
		row := &models.Submission{
			AssignmentID:  a.ID,
			UserID:        uid,
			WorkflowState: models.SubmissionUnsubmitted,
			AnonymousID:   &anon,
		}
		if err := s.submissionRepo.Create(ctx, row); err != nil {
			// A concurrent request may have created it; re-read.
			if existing, fErr := s.submissionRepo.FindByAssignmentAndUser(ctx, a.ID, uid, 0); fErr == nil {
				byUser[uid] = existing
				continue
			}
			return nil, err
		}
		byUser[uid] = row
	}
	for _, sub := range byUser {
		if sub.AnonymousID != nil && *sub.AnonymousID != "" {
			continue
		}
		anon := newAnonymousID(taken)
		if err := s.repo.UpdateSubmissionColumns(ctx, sub.ID, map[string]interface{}{"anonymous_id": anon}); err != nil {
			return nil, err
		}
		sub.AnonymousID = &anon
	}
	return byUser, nil
}

// ResolveAnonymousID maps an anonymous id back to the submission (and
// so the student) inside the caller's tenant.
func (s *GradingUIService) ResolveAnonymousID(ctx context.Context, assignmentID uint, anonymousID string, accountID uint) (*models.Submission, error) {
	anonymousID = strings.TrimSpace(anonymousID)
	if anonymousID == "" {
		return nil, ErrSubmissionNotFound
	}
	sub, err := s.repo.FindSubmissionByAnonymousID(ctx, assignmentID, anonymousID, accountID)
	if err != nil || sub == nil {
		return nil, ErrSubmissionNotFound
	}
	return sub, nil
}

// ---------------------------------------------------------------------
// Submission attempt history
// ---------------------------------------------------------------------

// VersionsByAssignment returns every recorded attempt for the assignment
// keyed by submission id.
func (s *GradingUIService) VersionsByAssignment(ctx context.Context, assignmentID uint) (map[uint][]models.SubmissionVersion, error) {
	versions, err := s.repo.ListSubmissionVersionsByAssignment(ctx, assignmentID)
	if err != nil {
		return nil, err
	}
	out := make(map[uint][]models.SubmissionVersion)
	for _, v := range versions {
		out[v.SubmissionID] = append(out[v.SubmissionID], v)
	}
	return out, nil
}

// VersionsForSubmission returns one submission's recorded attempts.
func (s *GradingUIService) VersionsForSubmission(ctx context.Context, submissionID uint) ([]models.SubmissionVersion, error) {
	return s.repo.ListSubmissionVersions(ctx, submissionID)
}

// ---------------------------------------------------------------------
// Moderated grading
// ---------------------------------------------------------------------

// IsModerator reports whether userID may moderate the assignment: the
// final grader, or an administrator (Canvas "Select final grade").
func IsModerator(a *models.Assignment, userID uint, isAdmin bool) bool {
	if a == nil || !a.ModeratedGrading {
		return false
	}
	if isAdmin {
		return true
	}
	return a.FinalGraderID != nil && *a.FinalGraderID == userID
}

// CanViewOtherGraderIdentities mirrors Canvas
// Assignment#can_view_other_grader_identities? for a course grader:
// until a moderated assignment's grades are published, the final grader
// sees the other graders' names only when grader_names_visible_to_final_grader
// is on; an administrator always does; any other grader does only when
// grader comments are visible and graders are not anonymous to each
// other. Outside moderated grading identities are never hidden.
func CanViewOtherGraderIdentities(a *models.Assignment, userID uint, isAdmin bool) bool {
	if a == nil || !a.ModeratedGrading || a.GradesPublished() {
		return true
	}
	if a.FinalGraderID != nil && *a.FinalGraderID == userID {
		return a.GraderNamesVisibleToFinalGrader
	}
	if isAdmin {
		return true
	}
	if !a.GraderCommentsVisibleToGraders {
		return false
	}
	return !a.GradersAnonymousToGraders
}

// GraderLabels maps each grader of a moderated assignment to the label
// another grader sees when identities are hidden ("Grader AB12C").
func (s *GradingUIService) GraderLabels(ctx context.Context, assignmentID uint) map[uint]string {
	out := map[uint]string{}
	graders, err := s.repo.ListModerationGraders(ctx, assignmentID)
	if err != nil {
		return out
	}
	for _, g := range graders {
		out[g.UserID] = "Grader " + strings.ToUpper(g.AnonymousID)
	}
	return out
}

// ensureModerationGrader registers graderID as a grader of the moderated
// assignment, taking one of the grader_count slots unless the grader is
// the moderator (Canvas ensure_grader_can_adjudicate).
func (s *GradingUIService) ensureModerationGrader(ctx context.Context, a *models.Assignment, graderID uint, isModerator bool) (*models.ModerationGrader, error) {
	occupy := !isModerator
	existing, err := s.repo.FindModerationGrader(ctx, a.ID, graderID)
	if err == nil && existing != nil {
		if occupy && !existing.SlotTaken {
			if err := s.checkGraderSlot(ctx, a, graderID); err != nil {
				return nil, err
			}
			existing.SlotTaken = true
			if err := s.repo.SaveModerationGrader(ctx, existing); err != nil {
				return nil, err
			}
		}
		return existing, nil
	}
	if occupy {
		if err := s.checkGraderSlot(ctx, a, graderID); err != nil {
			return nil, err
		}
	}
	graders, _ := s.repo.ListModerationGraders(ctx, a.ID)
	taken := make(map[string]bool, len(graders))
	for _, g := range graders {
		taken[g.AnonymousID] = true
	}
	g := &models.ModerationGrader{
		AssignmentID: a.ID,
		UserID:       graderID,
		AnonymousID:  newAnonymousID(taken),
		SlotTaken:    occupy,
	}
	if err := s.repo.SaveModerationGrader(ctx, g); err != nil {
		// Unique (assignment_id, user_id): a concurrent first grade by
		// the same grader created the row.
		if again, fErr := s.repo.FindModerationGrader(ctx, a.ID, graderID); fErr == nil {
			return again, nil
		}
		return nil, err
	}
	return g, nil
}

// checkGraderSlot refuses a new provisional grader once grader_count
// graders (not counting the final grader) hold a slot.
func (s *GradingUIService) checkGraderSlot(ctx context.Context, a *models.Assignment, graderID uint) error {
	limit := a.GraderCount
	if limit <= 0 {
		return nil
	}
	graders, err := s.repo.ListModerationGraders(ctx, a.ID)
	if err != nil {
		return err
	}
	used := 0
	for _, g := range graders {
		if !g.SlotTaken || g.UserID == graderID {
			continue
		}
		if a.FinalGraderID != nil && g.UserID == *a.FinalGraderID {
			continue
		}
		used++
	}
	if used >= limit {
		return ErrGraderLimitReached
	}
	return nil
}

// submissionRowFor returns the student's submission, creating an
// unsubmitted placeholder when there is none (a grade or comment can be
// left on a student who never submitted).
func (s *GradingUIService) submissionRowFor(ctx context.Context, a *models.Assignment, userID uint) (*models.Submission, error) {
	if sub, err := s.submissionRepo.FindByAssignmentAndUser(ctx, a.ID, userID, 0); err == nil && sub != nil {
		return sub, nil
	}
	rows, err := s.EnsureSubmissionRows(ctx, a, []uint{userID})
	if err != nil {
		return nil, err
	}
	if sub := rows[userID]; sub != nil {
		return sub, nil
	}
	return nil, ErrSubmissionNotFound
}

// SaveProvisionalGrade records graderID's grade for the student on a
// moderated assignment whose grades are not yet published. An empty
// postedGrade clears the grader's score. final=true stores the
// moderator's own custom grade (moderators only).
func (s *GradingUIService) SaveProvisionalGrade(ctx context.Context, a *models.Assignment, studentID, graderID uint, isModerator, final bool, postedGrade string) (*models.ProvisionalGrade, error) {
	if a == nil || !a.ModeratedGrading {
		return nil, ErrNotModerated
	}
	if a.GradesPublished() {
		return nil, ErrGradesAlreadyPublished
	}
	if final && !isModerator {
		return nil, ErrNotModerator
	}
	var score *float64
	var grade string
	postedGrade = strings.TrimSpace(postedGrade)
	if postedGrade != "" {
		// G023: a provisional grade is read like a final one, per the
		// assignment's grading type ("85%", "B+", "complete", points), so
		// the published grade matches what the grader typed.
		scheme := DefaultGradingScheme()
		if s.submissions != nil {
			scheme = s.submissions.resolveScheme(ctx, a)
		}
		v, err := InterpretGrade(a, scheme, postedGrade)
		if err != nil {
			return nil, err
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, fmt.Errorf("invalid grade value: %s", postedGrade)
		}
		score = &v
		grade = FormatGrade(a, scheme, v, postedGrade)
	}
	if _, err := s.ensureModerationGrader(ctx, a, graderID, isModerator); err != nil {
		return nil, err
	}
	sub, err := s.submissionRowFor(ctx, a, studentID)
	if err != nil {
		return nil, err
	}
	pg, err := s.repo.FindProvisionalGrade(ctx, sub.ID, graderID, final)
	if err != nil || pg == nil {
		pg = &models.ProvisionalGrade{SubmissionID: sub.ID, ScorerID: graderID, Final: final}
	}
	now := time.Now()
	pg.Score = score
	if score != nil {
		g := grade
		pg.Grade = &g
		pg.GradedAt = &now
	} else {
		pg.Grade = nil
		pg.GradedAt = nil
	}
	if err := s.repo.SaveProvisionalGrade(ctx, pg); err != nil {
		return nil, err
	}
	// The moderator's custom grade is selected for the student at once.
	if final && score != nil {
		if err := s.selectGrade(ctx, a.ID, studentID, &pg.ID); err != nil {
			return nil, err
		}
	}
	return pg, nil
}

// ProvisionalGradesForStudents returns the provisional grades of the
// given submissions keyed by submission id.
func (s *GradingUIService) ProvisionalGradesForStudents(ctx context.Context, submissionIDs []uint) (map[uint][]models.ProvisionalGrade, error) {
	pgs, err := s.repo.ListProvisionalGradesBySubmissions(ctx, submissionIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[uint][]models.ProvisionalGrade)
	for _, pg := range pgs {
		out[pg.SubmissionID] = append(out[pg.SubmissionID], pg)
	}
	return out, nil
}

// ModerationGraders lists the graders who took part.
func (s *GradingUIService) ModerationGraders(ctx context.Context, assignmentID uint) ([]models.ModerationGrader, error) {
	return s.repo.ListModerationGraders(ctx, assignmentID)
}

// ModerationSelections lists the moderator's selections keyed by student.
func (s *GradingUIService) ModerationSelections(ctx context.Context, assignmentID uint) (map[uint]*uint, error) {
	sels, err := s.repo.ListModerationSelections(ctx, assignmentID)
	if err != nil {
		return nil, err
	}
	out := make(map[uint]*uint, len(sels))
	for _, sel := range sels {
		out[sel.StudentID] = sel.SelectedProvisionalGradeID
	}
	return out, nil
}

func (s *GradingUIService) selectGrade(ctx context.Context, assignmentID, studentID uint, pgID *uint) error {
	sel, err := s.repo.FindModerationSelection(ctx, assignmentID, studentID)
	if err != nil || sel == nil {
		sel = &models.ModerationSelection{AssignmentID: assignmentID, StudentID: studentID}
	}
	sel.SelectedProvisionalGradeID = pgID
	return s.repo.SaveModerationSelection(ctx, sel)
}

// SelectProvisionalGrade records the moderator's choice of grade for a
// student. provisionalGradeID 0 clears the selection.
func (s *GradingUIService) SelectProvisionalGrade(ctx context.Context, a *models.Assignment, studentID, provisionalGradeID uint) error {
	if a == nil || !a.ModeratedGrading {
		return ErrNotModerated
	}
	if a.GradesPublished() {
		return ErrGradesAlreadyPublished
	}
	if provisionalGradeID == 0 {
		return s.selectGrade(ctx, a.ID, studentID, nil)
	}
	pg, err := s.repo.FindProvisionalGradeByID(ctx, provisionalGradeID)
	if err != nil || pg == nil {
		return ErrProvisionalGradeInvalid
	}
	sub, err := s.submissionRepo.FindByAssignmentAndUser(ctx, a.ID, studentID, 0)
	if err != nil || sub == nil || sub.ID != pg.SubmissionID {
		return ErrProvisionalGradeInvalid
	}
	id := pg.ID
	return s.selectGrade(ctx, a.ID, studentID, &id)
}

// GradesExist mirrors Canvas graded_submissions_exist?: some student of
// the assignment has a grade (or is excused), or a grader has entered a
// provisional grade. The caller has already loaded the assignment under
// its tenant. Errors count as "graded" so the moderation switch fails
// closed.
func (s *GradingUIService) GradesExist(ctx context.Context, assignmentID uint) bool {
	subs, err := s.repo.ListSubmissionsByAssignment(ctx, assignmentID)
	if err != nil {
		return true
	}
	ids := make([]uint, 0, len(subs))
	for _, sub := range subs {
		if sub.Score != nil || sub.Grade != nil || sub.Excused || sub.WorkflowState == models.SubmissionGraded {
			return true
		}
		ids = append(ids, sub.ID)
	}
	pgs, err := s.repo.ListProvisionalGradesBySubmissions(ctx, ids)
	if err != nil {
		return true
	}
	for _, pg := range pgs {
		if pg.Score != nil {
			return true
		}
	}
	return false
}

// PublishResult reports what PublishGrades did.
type PublishResult struct {
	Published   int      `json:"published"`
	Unresolved  []uint   `json:"unresolved_student_ids"`
	Errors      []string `json:"errors,omitempty"`
	PublishedAt string   `json:"published_at"`
}

// PublishGrades releases a moderated assignment's grades: each student's
// selected provisional grade (or the only one, when a single grader
// scored them) becomes their grade, and the assignment is marked
// published so ordinary grading and posting take over. Students whose
// grades are ambiguous (several provisional grades, none selected) are
// reported and left ungraded, as Canvas does.
func (s *GradingUIService) PublishGrades(ctx context.Context, a *models.Assignment, moderatorID, accountID uint) (*PublishResult, error) {
	if a == nil || !a.ModeratedGrading {
		return nil, ErrNotModerated
	}
	if a.GradesPublished() {
		return nil, ErrGradesAlreadyPublished
	}
	subs, err := s.repo.ListSubmissionsByAssignment(ctx, a.ID)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(subs))
	for _, sub := range subs {
		ids = append(ids, sub.ID)
	}
	pgsBySub, err := s.ProvisionalGradesForStudents(ctx, ids)
	if err != nil {
		return nil, err
	}
	selections, err := s.ModerationSelections(ctx, a.ID)
	if err != nil {
		return nil, err
	}

	// Mark published first so the regular grade path accepts the writes.
	if err := s.repo.SetAssignmentGradesPublished(ctx, a.ID); err != nil {
		return nil, err
	}
	now := time.Now()
	a.GradesPublishedAt = &now

	res := &PublishResult{Unresolved: []uint{}, PublishedAt: now.Format(time.RFC3339)}
	for _, sub := range subs {
		pgs := pgsBySub[sub.ID]
		var chosen *models.ProvisionalGrade
		if sel, ok := selections[sub.UserID]; ok && sel != nil {
			for i := range pgs {
				if pgs[i].ID == *sel {
					chosen = &pgs[i]
				}
			}
		}
		if chosen == nil {
			scored := make([]models.ProvisionalGrade, 0, len(pgs))
			for _, pg := range pgs {
				if pg.Score != nil {
					scored = append(scored, pg)
				}
			}
			if len(scored) == 1 {
				chosen = &scored[0]
			} else if len(scored) > 1 {
				res.Unresolved = append(res.Unresolved, sub.UserID)
			}
		}
		if chosen == nil || chosen.Score == nil {
			continue
		}
		given := ""
		if chosen.Grade != nil {
			given = *chosen.Grade
		}
		if _, err := s.submissions.PublishModeratedGrade(ctx, a, sub.UserID, moderatorID, accountID, *chosen.Score, given); err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("student %d: %v", sub.UserID, err))
			continue
		}
		res.Published++
	}
	return res, nil
}

// ---------------------------------------------------------------------
// Custom grade statuses
// ---------------------------------------------------------------------

// ListCustomGradeStatuses returns the account's active statuses.
func (s *GradingUIService) ListCustomGradeStatuses(ctx context.Context, accountID uint) ([]models.CustomGradeStatus, error) {
	return s.repo.ListCustomGradeStatuses(ctx, accountID, false)
}

// CourseAccountID returns the account a course belongs to (tenant-scoped).
func (s *GradingUIService) CourseAccountID(ctx context.Context, courseID, accountID uint) (uint, error) {
	course, err := s.courseRepo.FindByID(ctx, courseID, accountID)
	if err != nil || course == nil {
		return 0, ErrAssignmentUnavailable
	}
	return course.AccountID, nil
}

// CustomGradeStatusInput is the writable part of a status.
type CustomGradeStatusInput struct {
	Name                 *string `json:"name"`
	Color                *string `json:"color"`
	AppliesToSubmissions *bool   `json:"applies_to_submissions"`
	AppliesToFinalGrade  *bool   `json:"applies_to_final_grade"`
}

func validStatusColor(c string) bool {
	if len(c) != 7 || c[0] != '#' {
		return false
	}
	for _, r := range c[1:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return false
		}
	}
	return true
}

func applyStatusInput(st *models.CustomGradeStatus, in CustomGradeStatusInput) error {
	if in.Name != nil {
		name := strings.TrimSpace(SanitizePlainText(*in.Name))
		if name == "" || len([]rune(name)) > 14 {
			return errors.New("name is required and must be at most 14 characters")
		}
		st.Name = name
	}
	if in.Color != nil {
		if !validStatusColor(*in.Color) {
			return errors.New("color must be a hex color such as #E0E0E0")
		}
		st.Color = strings.ToUpper(*in.Color)
	}
	if in.AppliesToSubmissions != nil {
		st.AppliesToSubmissions = *in.AppliesToSubmissions
	}
	if in.AppliesToFinalGrade != nil {
		st.AppliesToFinalGrade = *in.AppliesToFinalGrade
	}
	if !st.AppliesToSubmissions && !st.AppliesToFinalGrade {
		return errors.New("a status must apply to submissions, final grades, or both")
	}
	return nil
}

// CreateCustomGradeStatus adds a status to the account (max 3 active).
func (s *GradingUIService) CreateCustomGradeStatus(ctx context.Context, accountID, creatorID uint, in CustomGradeStatusInput) (*models.CustomGradeStatus, error) {
	existing, err := s.repo.ListCustomGradeStatuses(ctx, accountID, false)
	if err != nil {
		return nil, err
	}
	if len(existing) >= models.MaxCustomGradeStatuses {
		return nil, ErrCustomStatusLimit
	}
	if in.AppliesToSubmissions == nil && in.AppliesToFinalGrade == nil {
		t := true
		in.AppliesToSubmissions = &t
	}
	if in.Name == nil {
		return nil, errors.New("name is required and must be at most 14 characters")
	}
	creator := creatorID
	st := &models.CustomGradeStatus{AccountID: accountID, CreatedByID: &creator, WorkflowState: "active"}
	if err := applyStatusInput(st, in); err != nil {
		return nil, err
	}
	if err := s.repo.CreateCustomGradeStatus(ctx, st); err != nil {
		return nil, err
	}
	return st, nil
}

// UpdateCustomGradeStatus edits a status in the account.
func (s *GradingUIService) UpdateCustomGradeStatus(ctx context.Context, accountID, id uint, in CustomGradeStatusInput) (*models.CustomGradeStatus, error) {
	st, err := s.repo.FindCustomGradeStatus(ctx, id, accountID)
	if err != nil || st == nil || st.WorkflowState != "active" {
		return nil, ErrCustomStatusNotFound
	}
	if err := applyStatusInput(st, in); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateCustomGradeStatus(ctx, st); err != nil {
		return nil, err
	}
	return st, nil
}

// DeleteCustomGradeStatus soft-deletes a status and clears it from the
// submissions that carried it.
func (s *GradingUIService) DeleteCustomGradeStatus(ctx context.Context, accountID, id uint) error {
	st, err := s.repo.FindCustomGradeStatus(ctx, id, accountID)
	if err != nil || st == nil || st.WorkflowState != "active" {
		return ErrCustomStatusNotFound
	}
	st.WorkflowState = "deleted"
	if err := s.repo.UpdateCustomGradeStatus(ctx, st); err != nil {
		return err
	}
	return s.repo.ClearCustomGradeStatus(ctx, st.ID)
}

// SubmissionStatusUpdate is a grader's status change on one submission.
// Nil fields are left alone. CustomGradeStatusID 0 clears the custom
// status; LatePolicyStatus "none" clears late/missing.
type SubmissionStatusUpdate struct {
	CustomGradeStatusID *uint
	LatePolicyStatus    *string
	ExtraAttempts       *int
	// GraderID is recorded on any grade change a late/missing status
	// causes (late deduction, missing policy).
	GraderID uint
}

// Empty reports whether the update changes nothing.
func (u SubmissionStatusUpdate) Empty() bool {
	return u.CustomGradeStatusID == nil && u.LatePolicyStatus == nil && u.ExtraAttempts == nil
}

// UpdateSubmissionStatus applies a grader's status change. The
// assignment was loaded under the caller's tenant; courseAccountID is
// the account that owns the course (custom statuses belong to it).
func (s *GradingUIService) UpdateSubmissionStatus(ctx context.Context, a *models.Assignment, studentID, courseAccountID uint, u SubmissionStatusUpdate) (*models.Submission, error) {
	cols := map[string]interface{}{}
	if u.CustomGradeStatusID != nil {
		if *u.CustomGradeStatusID == 0 {
			cols["custom_grade_status_id"] = nil
		} else {
			st, err := s.repo.FindCustomGradeStatus(ctx, *u.CustomGradeStatusID, courseAccountID)
			if err != nil || st == nil || st.WorkflowState != "active" || !st.AppliesToSubmissions {
				return nil, ErrCustomStatusNotFound
			}
			cols["custom_grade_status_id"] = st.ID
		}
	}
	lateStatus := ""
	if u.LatePolicyStatus != nil {
		switch *u.LatePolicyStatus {
		case "late", "missing", "extended", "none":
			lateStatus = *u.LatePolicyStatus
		case "":
			lateStatus = "none"
		default:
			return nil, ErrInvalidLatePolicyStatus
		}
	}
	if u.ExtraAttempts != nil {
		if *u.ExtraAttempts < 0 || *u.ExtraAttempts > 1000 {
			return nil, errors.New("extra_attempts must be between 0 and 1000")
		}
		cols["extra_attempts"] = *u.ExtraAttempts
	}
	// A late/missing status goes through the grading-policy rules
	// (closed periods, effective due date, late deduction, missing
	// policy) rather than writing the flags directly.
	if lateStatus != "" {
		if _, err := s.submissions.SetLatePolicyStatus(ctx, a.ID, studentID, u.GraderID, courseAccountID, lateStatus, nil); err != nil {
			return nil, err
		}
	}
	sub, err := s.submissionRowFor(ctx, a, studentID)
	if err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return sub, nil
	}
	if err := s.repo.UpdateSubmissionColumns(ctx, sub.ID, cols); err != nil {
		return nil, err
	}
	return s.submissionRepo.FindByID(ctx, sub.ID, 0)
}

// ---------------------------------------------------------------------
// Comments
// ---------------------------------------------------------------------

// FindComment loads a comment under the caller's tenant.
func (s *GradingUIService) FindComment(ctx context.Context, id, accountID uint) (*models.SubmissionComment, error) {
	return s.repo.FindSubmissionComment(ctx, id, accountID)
}

// UpdateComment edits a comment's text and/or publishes a draft. Group
// comments are edited on every member's copy.
func (s *GradingUIService) UpdateComment(ctx context.Context, c *models.SubmissionComment, text *string, draft *bool) error {
	targets := []models.SubmissionComment{*c}
	if c.GroupCommentID != nil && *c.GroupCommentID != "" {
		if copies, err := s.repo.ListSubmissionCommentsByGroupCommentID(ctx, *c.GroupCommentID); err == nil && len(copies) > 0 {
			targets = copies
		}
	}
	for i := range targets {
		t := &targets[i]
		if text != nil {
			t.Comment = *text
		}
		if draft != nil {
			t.Draft = *draft
		}
		if err := s.repo.UpdateSubmissionComment(ctx, t); err != nil {
			return err
		}
		if t.ID == c.ID {
			*c = *t
		}
	}
	return nil
}

// CommentCopySubmissions returns the submissions carrying comment c: sub
// alone, or, for a group comment, every member's submission that holds a
// copy (sub first).
func (s *GradingUIService) CommentCopySubmissions(ctx context.Context, c *models.SubmissionComment, sub *models.Submission) []*models.Submission {
	out := []*models.Submission{sub}
	if c == nil || c.GroupCommentID == nil || *c.GroupCommentID == "" {
		return out
	}
	copies, err := s.repo.ListSubmissionCommentsByGroupCommentID(ctx, *c.GroupCommentID)
	if err != nil {
		return out
	}
	for _, cp := range copies {
		if cp.SubmissionID == sub.ID {
			continue
		}
		// The copies were written by this lane for members of the same
		// assignment; the id comes from our own row, not the request.
		if other, err := s.submissionRepo.FindByID(ctx, cp.SubmissionID, 0); err == nil && other != nil && other.AssignmentID == sub.AssignmentID {
			out = append(out, other)
		}
	}
	return out
}

// DeleteComment removes a comment (every copy of a group comment).
func (s *GradingUIService) DeleteComment(ctx context.Context, c *models.SubmissionComment) error {
	if c.GroupCommentID != nil && *c.GroupCommentID != "" {
		if copies, err := s.repo.ListSubmissionCommentsByGroupCommentID(ctx, *c.GroupCommentID); err == nil {
			for _, cp := range copies {
				if err := s.repo.DeleteSubmissionComment(ctx, cp.ID); err != nil {
					return err
				}
			}
			return nil
		}
	}
	return s.repo.DeleteSubmissionComment(ctx, c.ID)
}

// GroupSubmissions returns the submission rows of every accepted member
// of userID's group for a group assignment (creating placeholders), the
// student's own row first. For a non-group assignment, or a student in
// no group, it is just the student's row.
func (s *GradingUIService) GroupSubmissions(ctx context.Context, a *models.Assignment, userID uint) ([]*models.Submission, error) {
	own, err := s.submissionRowFor(ctx, a, userID)
	if err != nil {
		return nil, err
	}
	out := []*models.Submission{own}
	if !a.IsGroupAssignment() || s.submissions == nil {
		return out, nil
	}
	members := s.submissions.getGroupMemberIDs(ctx, userID, *a.GroupCategoryID)
	if len(members) == 0 {
		return out, nil
	}
	rows, err := s.EnsureSubmissionRows(ctx, a, members)
	if err != nil {
		return out, nil
	}
	for _, m := range members {
		if r := rows[m]; r != nil {
			out = append(out, r)
		}
	}
	return out, nil
}

// StudentGroup describes the group a student submitted with.
type StudentGroup struct {
	GroupID   uint   `json:"group_id"`
	GroupName string `json:"group_name"`
}

// GroupsForStudents maps each student to their group in the assignment's
// group category (empty map for a non-group assignment).
func (s *GradingUIService) GroupsForStudents(ctx context.Context, a *models.Assignment, studentIDs []uint) map[uint]StudentGroup {
	out := map[uint]StudentGroup{}
	if !a.IsGroupAssignment() || s.groupMembershipRepo == nil {
		return out
	}
	for _, uid := range studentIDs {
		g, err := s.groupMembershipRepo.FindUserGroupInCategory(ctx, uid, *a.GroupCategoryID)
		if err != nil || g == nil {
			continue
		}
		out[uid] = StudentGroup{GroupID: g.ID, GroupName: g.Name}
	}
	return out
}

// ---------------------------------------------------------------------
// Peer reviews
// ---------------------------------------------------------------------

// ErrPeerReviewConflict is returned for an invalid manual assignment.
var ErrPeerReviewConflict = errors.New("that peer review cannot be assigned")

// AssignPeerReview manually assigns reviewerID to review revieweeID's
// submission (Canvas "Assign peer review"). Both must be students; the
// reviewee must have submitted.
func (s *GradingUIService) AssignPeerReview(ctx context.Context, a *models.Assignment, reviewerID, revieweeID uint) (*models.PeerReview, error) {
	if reviewerID == 0 || revieweeID == 0 || reviewerID == revieweeID {
		return nil, ErrPeerReviewConflict
	}
	sub, err := s.submissionRepo.FindByAssignmentAndUser(ctx, a.ID, revieweeID, 0)
	if err != nil || !hasSubmitted(sub) {
		return nil, errors.New("the student being reviewed has not submitted")
	}
	existing, err := s.peerReviewRepo.ListByAssignment(ctx, a.ID, 0)
	if err != nil {
		return nil, err
	}
	for _, pr := range existing {
		if pr.ReviewerID == reviewerID && pr.RevieweeID == revieweeID {
			return nil, ErrPeerReviewConflict
		}
	}
	pr := &models.PeerReview{
		AssignmentID:  a.ID,
		SubmissionID:  sub.ID,
		ReviewerID:    reviewerID,
		RevieweeID:    revieweeID,
		WorkflowState: "assigned",
	}
	if err := s.peerReviewRepo.Create(ctx, pr); err != nil {
		return nil, err
	}
	return pr, nil
}

// DeletePeerReview removes an assigned review of the assignment
// (tenant-scoped through accountID).
func (s *GradingUIService) DeletePeerReview(ctx context.Context, a *models.Assignment, reviewID, accountID uint) error {
	pr, err := s.peerReviewRepo.FindByID(ctx, reviewID, accountID)
	if err != nil || pr == nil || pr.AssignmentID != a.ID {
		return ErrPeerReviewNotFound
	}
	return s.repo.DeletePeerReview(ctx, pr.ID)
}

// ReceivedPeerReviews returns the completed reviews of revieweeID's work.
func (s *GradingUIService) ReceivedPeerReviews(ctx context.Context, assignmentID, revieweeID, accountID uint) ([]models.PeerReview, error) {
	all, err := s.peerReviewRepo.ListByAssignment(ctx, assignmentID, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]models.PeerReview, 0)
	for _, pr := range all {
		if pr.RevieweeID == revieweeID && pr.WorkflowState == "completed" {
			out = append(out, pr)
		}
	}
	return out, nil
}

// AnonymizedAssignmentIDs returns the course's assignments whose
// students are currently anonymized for graders (see AnonymizeStudents).
// The gradebook hides those columns' grades, as Canvas does.
func (s *GradingUIService) AnonymizedAssignmentIDs(ctx context.Context, courseID uint, studentIDs []uint) map[uint]bool {
	out := map[uint]bool{}
	res, err := s.assignmentRepo.ListByCourseID(ctx, courseID, repository.PaginationParams{Page: 1, PerPage: 1000})
	if err != nil || res == nil {
		return out
	}
	for i := range res.Items {
		a := &res.Items[i]
		if a.AnonymousGrading && s.AnonymizeStudents(ctx, a, studentIDs) {
			out[a.ID] = true
		}
	}
	return out
}
