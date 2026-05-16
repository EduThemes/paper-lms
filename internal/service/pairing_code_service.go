package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"gorm.io/gorm"
)

// DefaultPairingCodeTTL is the default lifetime for a freshly generated
// pairing code if no explicit TTL is provided.
const DefaultPairingCodeTTL = 24 * time.Hour

// PairingCodeObserverLinker is the slice of ObserverService used by this
// service. Defining it locally keeps us decoupled from the concrete service
// type and avoids touching any shared interface file.
type PairingCodeObserverLinker interface {
	LinkObserverToStudent(ctx context.Context, observerUserID, studentUserID uint) error
}

// PairingCodeService implements the parent/observer pairing-code workflow.
type PairingCodeService struct {
	repo            postgres.PairingCodeRepository
	observerService PairingCodeObserverLinker
	maxAttempts     int

	// Optional dependencies needed for the teacher-or-self mint flow
	// added in Phase 12 / item 12.6. Wired via SetAuthzDeps; without
	// them GenerateForStudent returns an error.
	enrollmentRepo repository.EnrollmentRepository
	courseRepo     repository.CourseRepository
	accountRepo    repository.AccountRepository
}

func NewPairingCodeService(repo postgres.PairingCodeRepository, observerService PairingCodeObserverLinker) *PairingCodeService {
	return &PairingCodeService{
		repo:            repo,
		observerService: observerService,
		maxAttempts:     5,
	}
}

// SetAuthzDeps wires the repositories needed by GenerateForStudent's
// consent check. Safe to call multiple times (last-write-wins).
func (s *PairingCodeService) SetAuthzDeps(
	enrollmentRepo repository.EnrollmentRepository,
	courseRepo repository.CourseRepository,
	accountRepo repository.AccountRepository,
) {
	s.enrollmentRepo = enrollmentRepo
	s.courseRepo = courseRepo
	s.accountRepo = accountRepo
}

// adultTenantModes are the tenant modes in which a student may mint
// their own pairing code. K-12 modes (k5, m68, h912) require a teacher
// to mint as a consent surrogate per item 12.6.
var adultTenantModes = map[string]bool{
	"higher_ed": true,
	"corp":      true,
	"pro":       true,
}

// ErrPairingMintForbidden is returned by GenerateForStudent when the
// caller is neither a teacher in any of the student's courses nor the
// student themselves in an adult-mode tenant.
var ErrPairingMintForbidden = errors.New("not authorized to mint pairing code for this student")

// GenerateForStudent mints a pairing code on behalf of a student after
// applying the 12.6 consent rule:
//
//   - A teacher (TeacherEnrollment / TaEnrollment) in any of the student's
//     active courses may mint.
//   - The student themselves may mint IF every course they are enrolled
//     in is hosted by an account whose tenant_mode is adult-mode
//     (higher_ed / corp / pro). A student in even one K-12 course must
//     get a teacher to mint.
//
// Returns ErrPairingMintForbidden when neither condition holds.
func (s *PairingCodeService) GenerateForStudent(ctx context.Context, callerID, studentID uint, ttl time.Duration) (*models.PairingCode, error) {
	if studentID == 0 {
		return nil, errors.New("student id is required")
	}
	if callerID == 0 {
		return nil, errors.New("caller id is required")
	}
	if s.enrollmentRepo == nil || s.courseRepo == nil || s.accountRepo == nil {
		return nil, errors.New("pairing-code mint authz not configured")
	}

	authorized, err := s.callerMayMintForStudent(ctx, callerID, studentID)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, ErrPairingMintForbidden
	}
	return s.Generate(ctx, studentID, ttl)
}

// callerMayMintForStudent encodes the 12.6 consent rule. See
// GenerateForStudent for the full description.
func (s *PairingCodeService) callerMayMintForStudent(ctx context.Context, callerID, studentID uint) (bool, error) {
	studentEnrollments, err := s.enrollmentRepo.ListByUserID(ctx, studentID)
	if err != nil {
		return false, errors.New("could not fetch student enrollments")
	}
	// A student with no active enrollments has no course context to
	// consent against — fail closed.
	hasActiveCourse := false
	allCoursesAdultMode := true
	courseIDs := make([]uint, 0, len(studentEnrollments))
	for _, e := range studentEnrollments {
		if e.WorkflowState != "active" || e.Type != "StudentEnrollment" {
			continue
		}
		hasActiveCourse = true
		courseIDs = append(courseIDs, e.CourseID)
		course, err := s.courseRepo.FindByID(ctx, e.CourseID, 0)
		if err != nil {
			continue
		}
		account, err := s.accountRepo.FindByID(ctx, course.AccountID)
		if err != nil {
			continue
		}
		if !adultTenantModes[string(account.TenantMode)] {
			allCoursesAdultMode = false
		}
	}
	if !hasActiveCourse {
		return false, nil
	}

	// Teacher path: caller has a TeacherEnrollment / TaEnrollment in any
	// of the student's courses.
	callerEnrollments, err := s.enrollmentRepo.ListByUserID(ctx, callerID)
	if err != nil {
		return false, errors.New("could not fetch caller enrollments")
	}
	courseIDSet := make(map[uint]bool, len(courseIDs))
	for _, id := range courseIDs {
		courseIDSet[id] = true
	}
	for _, e := range callerEnrollments {
		if e.WorkflowState != "active" {
			continue
		}
		if e.Type != "TeacherEnrollment" && e.Type != "TaEnrollment" {
			continue
		}
		if courseIDSet[e.CourseID] {
			return true, nil
		}
	}

	// Self-mint path: only when EVERY course is adult-mode.
	if callerID == studentID && allCoursesAdultMode {
		return true, nil
	}
	return false, nil
}

// Generate creates a new active pairing code for the given student. Multiple
// active codes per student are allowed. If ttl is zero or negative, the
// default TTL is used.
func (s *PairingCodeService) Generate(ctx context.Context, studentID uint, ttl time.Duration) (*models.PairingCode, error) {
	if studentID == 0 {
		return nil, errors.New("student id is required")
	}
	if ttl <= 0 {
		ttl = DefaultPairingCodeTTL
	}

	now := time.Now().UTC()
	pc := &models.PairingCode{
		UserID:    studentID,
		ExpiresAt: now.Add(ttl),
	}

	// Retry on the (extremely unlikely) chance of a uniqueness collision.
	var lastErr error
	for attempt := 0; attempt < s.maxAttempts; attempt++ {
		code, err := models.GeneratePairingCodeString()
		if err != nil {
			return nil, errors.New("could not generate pairing code")
		}
		pc.Code = code
		if err := s.repo.Create(ctx, pc); err != nil {
			lastErr = err
			continue
		}
		return pc, nil
	}
	if lastErr != nil {
		return nil, errors.New("could not create pairing code")
	}
	return nil, errors.New("could not create pairing code")
}

// Redeem atomically validates the code and links the observer to the student.
// The pairing code is marked redeemed inside the same transaction. The
// observer link itself is performed via the injected observer linker after
// the code is reserved.
func (s *PairingCodeService) Redeem(ctx context.Context, code string, observerID uint) (*models.PairingCode, error) {
	if observerID == 0 {
		return nil, errors.New("observer id is required")
	}
	cleaned := normalizePairingCode(code)
	if cleaned == "" {
		return nil, errors.New("pairing code is required")
	}

	now := time.Now().UTC()
	var redeemed *models.PairingCode

	err := s.repo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txRepo := s.repo.WithTx(tx)
		pc, err := txRepo.FindByCode(ctx, cleaned)
		if err != nil {
			return errors.New("invalid pairing code")
		}
		if pc.IsRedeemed() {
			return errors.New("pairing code has already been used")
		}
		if pc.IsExpired(now) {
			return errors.New("pairing code has expired")
		}
		if pc.UserID == observerID {
			return errors.New("you cannot pair yourself as your own observer")
		}

		// Reserve the code inside the transaction. The conditional update
		// (redeemed_at IS NULL) protects against a concurrent redeem racing
		// with this one.
		if err := txRepo.MarkRedeemed(ctx, pc.ID, now); err != nil {
			return errors.New("could not redeem pairing code")
		}

		// Re-read to confirm we won the race and to get the canonical row.
		fresh, err := txRepo.FindByID(ctx, pc.ID)
		if err != nil || fresh.RedeemedAt == nil || !fresh.RedeemedAt.Equal(now) {
			return errors.New("pairing code has already been used")
		}

		// Create the observer relationship. If this fails the transaction
		// rolls back, leaving the code reusable.
		if err := s.observerService.LinkObserverToStudent(ctx, observerID, pc.UserID); err != nil {
			return err
		}

		redeemed = fresh
		return nil
	})
	if err != nil {
		return nil, err
	}
	return redeemed, nil
}

// ListActiveForStudent returns all unredeemed, unexpired codes belonging to
// the student, newest first.
func (s *PairingCodeService) ListActiveForStudent(ctx context.Context, studentID uint) ([]models.PairingCode, error) {
	if studentID == 0 {
		return nil, errors.New("student id is required")
	}
	return s.repo.ListActiveByUserID(ctx, studentID, time.Now().UTC())
}

// Revoke deletes a code that belongs to the given student. Codes belonging to
// other users are not deletable through this method.
func (s *PairingCodeService) Revoke(ctx context.Context, studentID, codeID uint) error {
	pc, err := s.repo.FindByID(ctx, codeID)
	if err != nil {
		return errors.New("pairing code not found")
	}
	if pc.UserID != studentID {
		return errors.New("pairing code not found")
	}
	return s.repo.Delete(ctx, codeID)
}

// normalizePairingCode strips whitespace and uppercases the input so that
// users can type codes leniently (lower-case, with or without hyphens).
func normalizePairingCode(in string) string {
	s := strings.ToUpper(strings.TrimSpace(in))
	// Allow users to enter the code without hyphens; collapse anything that
	// is not alphanumeric or hyphen.
	var b strings.Builder
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	cleaned := b.String()
	// If they entered 9 chars without hyphens, format with hyphens.
	if len(cleaned) == 9 && !strings.Contains(cleaned, "-") {
		cleaned = cleaned[0:3] + "-" + cleaned[3:6] + "-" + cleaned[6:9]
	}
	return cleaned
}
