package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/auth/initialpassword"
	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// AdminUserService backs account-level user management (G114): creating
// users, editing another user's profile and primary login, and the admin
// user detail page (enrollments, logins, observer links, sign-in history,
// grade changes). Every call is scoped to one tenant (accountID).
type AdminUserService struct {
	db *gorm.DB
}

// NewAdminUserService builds the service.
func NewAdminUserService(db *gorm.DB) *AdminUserService {
	return &AdminUserService{db: db}
}

// Errors surfaced to handlers.
var (
	ErrAdminUserNotFound   = errors.New("user not found")
	ErrAdminUserLoginTaken = errors.New("that login is already in use")
	ErrAdminUserSISTaken   = errors.New("that SIS user id is already in use")
	ErrAdminUserInvalid    = errors.New("name, email and login are required")
	ErrAdminUserPassword   = errors.New("password must be at least 8 characters")
)

// AdminUserInput is the editable profile of a user. nil = unchanged.
type AdminUserInput struct {
	Name         *string
	ShortName    *string
	SortableName *string
	Email        *string
	LoginID      *string
	SISUserID    *string
	TimeZone     *string
	Locale       *string
	Password     *string
	// RequirePasswordReset forces a new password at next sign-in.
	RequirePasswordReset *bool
}

func trimPtr(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

// loginTaken reports whether login is used as any user's primary login
// (other than exceptUserID) or as an active additional login.
func (s *AdminUserService) loginTaken(ctx context.Context, login string, exceptUserID uint) (bool, error) {
	norm := models.NormalizeLoginID(login)
	var n int64
	if err := s.db.WithContext(ctx).Model(&models.User{}).
		Where("LOWER(login_id) = ? AND id <> ?", norm, exceptUserID).Count(&n).Error; err != nil {
		return false, err
	}
	if n > 0 {
		return true, nil
	}
	if err := s.db.WithContext(ctx).Model(&models.UserLogin{}).
		Where("unique_id_normalized = ? AND workflow_state = ?", norm, models.UserLoginActive).
		Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *AdminUserService) sisTaken(ctx context.Context, sis string, exceptUserID uint) (bool, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&models.User{}).
		Where("sis_user_id = ? AND id <> ?", sis, exceptUserID).Count(&n).Error
	return n > 0, err
}

// applyProfile validates and copies input onto u.
func (s *AdminUserService) applyProfile(ctx context.Context, u *models.User, in AdminUserInput) error {
	if in.Name != nil {
		name := trimPtr(in.Name)
		if name == "" {
			return ErrAdminUserInvalid
		}
		u.Name = SanitizePlainText(name)
		if in.SortableName == nil {
			u.SortableName = sortableNameFor(u.Name)
		}
		if in.ShortName == nil && u.ShortName == "" {
			u.ShortName = strings.Fields(u.Name)[0]
		}
	}
	if in.ShortName != nil {
		u.ShortName = SanitizePlainText(trimPtr(in.ShortName))
	}
	if in.SortableName != nil {
		u.SortableName = SanitizePlainText(trimPtr(in.SortableName))
	}
	if in.Email != nil {
		email := trimPtr(in.Email)
		if email == "" || !strings.Contains(email, "@") {
			return errors.New("a valid email is required")
		}
		u.Email = email
	}
	if in.LoginID != nil {
		login := trimPtr(in.LoginID)
		if login == "" {
			return ErrAdminUserInvalid
		}
		if !strings.EqualFold(login, u.LoginID) {
			taken, err := s.loginTaken(ctx, login, u.ID)
			if err != nil {
				return err
			}
			if taken {
				return ErrAdminUserLoginTaken
			}
		}
		u.LoginID = login
	}
	if in.SISUserID != nil {
		sis := trimPtr(in.SISUserID)
		if sis == "" {
			u.SISUserID = nil
		} else {
			taken, err := s.sisTaken(ctx, sis, u.ID)
			if err != nil {
				return err
			}
			if taken {
				return ErrAdminUserSISTaken
			}
			u.SISUserID = &sis
		}
	}
	if in.TimeZone != nil {
		if tz := trimPtr(in.TimeZone); tz != "" {
			if _, err := time.LoadLocation(tz); err != nil {
				return errors.New("unknown time zone")
			}
			u.TimeZone = tz
		}
	}
	if in.Locale != nil {
		u.Locale = trimPtr(in.Locale)
	}
	if in.Password != nil && *in.Password != "" {
		if len(*in.Password) < 8 {
			return ErrAdminUserPassword
		}
		if err := u.HashPassword(*in.Password); err != nil {
			return err
		}
	}
	if in.RequirePasswordReset != nil {
		u.RequiresPasswordReset = *in.RequirePasswordReset
	}
	return nil
}

// sortableNameFor renders "Last, First" for a two-word-or-longer name.
func sortableNameFor(name string) string {
	parts := strings.Fields(name)
	if len(parts) < 2 {
		return name
	}
	return parts[len(parts)-1] + ", " + strings.Join(parts[:len(parts)-1], " ")
}

// CreateUser creates a user in accountID (Canvas POST
// /accounts/:id/users). Without a password the user gets a random one and
// must set their own at first sign-in (the LoginPipeline gate).
func (s *AdminUserService) CreateUser(ctx context.Context, accountID uint, in AdminUserInput) (*models.User, error) {
	if accountID == 0 || trimPtr(in.Name) == "" || trimPtr(in.Email) == "" {
		return nil, ErrAdminUserInvalid
	}
	if trimPtr(in.LoginID) == "" {
		email := trimPtr(in.Email)
		in.LoginID = &email
	}
	u := &models.User{AccountID: accountID, Role: "user"}
	if err := s.applyProfile(ctx, u, in); err != nil {
		return nil, err
	}
	if u.PasswordHash == "" {
		pw, err := initialpassword.GenerateInitialPassword()
		if err != nil {
			return nil, err
		}
		if err := u.HashPassword(pw); err != nil {
			return nil, err
		}
		u.RequiresPasswordReset = true
	}
	if u.TimeZone == "" {
		u.TimeZone = "America/New_York"
	}
	if err := s.db.WithContext(ctx).Create(u).Error; err != nil {
		if isUniqueViolation(err) {
			return nil, ErrAdminUserLoginTaken
		}
		return nil, err
	}
	return u, nil
}

// GetUser loads a user of accountID.
func (s *AdminUserService) GetUser(ctx context.Context, userID, accountID uint) (*models.User, error) {
	var u models.User
	if err := s.db.WithContext(ctx).Where("id = ? AND account_id = ?", userID, accountID).First(&u).Error; err != nil {
		return nil, ErrAdminUserNotFound
	}
	return &u, nil
}

// UpdateUser edits a user of accountID.
func (s *AdminUserService) UpdateUser(ctx context.Context, u *models.User, in AdminUserInput) error {
	if err := s.applyProfile(ctx, u, in); err != nil {
		return err
	}
	if err := s.db.WithContext(ctx).Save(u).Error; err != nil {
		if isUniqueViolation(err) {
			return ErrAdminUserLoginTaken
		}
		return err
	}
	return nil
}

// AdminUserEnrollment is one enrollment row on the user detail page.
type AdminUserEnrollment struct {
	ID              uint       `json:"id"`
	CourseID        uint       `json:"course_id"`
	CourseName      string     `json:"course_name"`
	CourseCode      string     `json:"course_code"`
	CourseState     string     `json:"course_workflow_state"`
	SectionID       *uint      `json:"course_section_id"`
	SectionName     string     `json:"section_name"`
	Type            string     `json:"type"`
	Role            string     `json:"role"`
	RoleID          *uint      `json:"role_id"`
	WorkflowState   string     `json:"enrollment_state"`
	AssociatedUser  *uint      `json:"associated_user_id"`
	LastActivityAt  *time.Time `json:"last_activity_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

// ListEnrollments returns every enrollment of a user of accountID, in any
// state, with course and section names.
func (s *AdminUserService) ListEnrollments(ctx context.Context, userID, accountID uint) ([]AdminUserEnrollment, error) {
	var rows []AdminUserEnrollment
	err := s.db.WithContext(ctx).Raw(`
		SELECT e.id, e.course_id, c.name AS course_name, c.course_code, c.workflow_state AS course_state,
		       e.course_section_id AS section_id, COALESCE(cs.name, '') AS section_name,
		       e.type, e.role, e.role_id, e.workflow_state, e.associated_user_id AS associated_user,
		       e.last_activity_at, e.created_at
		FROM enrollments e
		JOIN courses c ON c.id = e.course_id
		LEFT JOIN course_sections cs ON cs.id = e.course_section_id
		WHERE e.user_id = ? AND c.account_id = ?
		ORDER BY (e.workflow_state = 'active') DESC, c.name ASC, e.id ASC`, userID, accountID).
		Scan(&rows).Error
	return rows, err
}

// ObserverLinkRow is one observer pairing on the user detail page.
type ObserverLinkRow struct {
	UserID    uint      `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	LinkedAt  time.Time `json:"linked_at"`
}

// ListObservees returns the students a user observes (active links).
func (s *AdminUserService) ListObservees(ctx context.Context, observerID, accountID uint) ([]ObserverLinkRow, error) {
	var rows []ObserverLinkRow
	err := s.db.WithContext(ctx).Raw(`
		SELECT u.id AS user_id, u.name, u.email, l.created_at AS linked_at
		FROM observer_links l JOIN users u ON u.id = l.student_id
		WHERE l.observer_id = ? AND l.workflow_state = 'active' AND u.account_id = ?
		ORDER BY u.sortable_name, u.id`, observerID, accountID).Scan(&rows).Error
	return rows, err
}

// ListObservers returns the users observing a student (active links).
func (s *AdminUserService) ListObservers(ctx context.Context, studentID, accountID uint) ([]ObserverLinkRow, error) {
	var rows []ObserverLinkRow
	err := s.db.WithContext(ctx).Raw(`
		SELECT u.id AS user_id, u.name, u.email, l.created_at AS linked_at
		FROM observer_links l JOIN users u ON u.id = l.observer_id
		WHERE l.student_id = ? AND l.workflow_state = 'active' AND u.account_id = ?
		ORDER BY u.sortable_name, u.id`, studentID, accountID).Scan(&rows).Error
	return rows, err
}

// ListAuthEvents returns a user's recent sign-in events (auth.* audit
// rows), newest first — the per-user login history (G119).
func (s *AdminUserService) ListAuthEvents(ctx context.Context, userID uint, limit int) ([]models.AuditLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows []models.AuditLog
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND event_type LIKE ?", userID, "auth.%").
		Order("created_at DESC, id DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

// ListUserActivity returns the user's recent audited actions (non-auth
// audit rows, including those taken while someone masqueraded as them).
func (s *AdminUserService) ListUserActivity(ctx context.Context, userID, accountID uint, limit int) ([]models.AuditLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows []models.AuditLog
	err := s.db.WithContext(ctx).
		Where("(user_id = ? OR real_user_id = ?) AND event_type NOT LIKE ? AND (account_id = ? OR account_id IS NULL)",
			userID, userID, "auth.%", accountID).
		Order("created_at DESC, id DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

// ListGradeChanges returns grade changes where the user was the grader or
// the student, limited to courses of accountID.
func (s *AdminUserService) ListGradeChanges(ctx context.Context, userID, accountID uint, limit int) ([]models.GradeChangeLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows []models.GradeChangeLog
	err := s.db.WithContext(ctx).
		Where("(grader_id = ? OR student_id = ?) AND course_id IN (SELECT id FROM courses WHERE account_id = ?)", userID, userID, accountID).
		Order("created_at DESC, id DESC").Limit(limit).Find(&rows).Error
	return rows, err
}
