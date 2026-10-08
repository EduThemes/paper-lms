package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// UserLoginService manages a user's additional logins (G115; Canvas
// pseudonyms). Every method that takes accountID scopes the user (and
// so the login) to that tenant; a user of another tenant is
// ErrUserLoginNotFound.
type UserLoginService struct {
	db *gorm.DB
}

// NewUserLoginService builds the service.
func NewUserLoginService(db *gorm.DB) *UserLoginService {
	return &UserLoginService{db: db}
}

// Errors surfaced to handlers.
var (
	ErrUserLoginNotFound   = errors.New("login not found")
	ErrUserLoginTaken      = errors.New("that login is already in use")
	ErrUserLoginInvalid    = errors.New("unique_id is required")
	ErrUserLoginProvider   = errors.New("authentication provider not found")
	ErrUserLoginSISTaken   = errors.New("that SIS id is already in use")
	ErrUserLoginPassword   = errors.New("password must be at least 8 characters")
	ErrUserLoginProviderPW = errors.New("an SSO login has no password")
)

// UserLoginInput is the editable part of a login. nil fields are left
// unchanged on update.
type UserLoginInput struct {
	UniqueID                 *string
	Password                 *string
	SISUserID                *string
	AuthenticationProviderID *uint // 0 = make it a local login
	WorkflowState            *string
}

// userInAccount loads a user of accountID.
func (s *UserLoginService) userInAccount(ctx context.Context, userID, accountID uint) (*models.User, error) {
	var u models.User
	q := s.db.WithContext(ctx).Where("id = ?", userID)
	if accountID != 0 {
		q = q.Where("account_id = ?", accountID)
	}
	if err := q.First(&u).Error; err != nil {
		return nil, ErrUserLoginNotFound
	}
	return &u, nil
}

// List returns the user's non-deleted logins.
func (s *UserLoginService) List(ctx context.Context, userID, accountID uint) ([]models.UserLogin, error) {
	if _, err := s.userInAccount(ctx, userID, accountID); err != nil {
		return nil, err
	}
	var out []models.UserLogin
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND workflow_state <> ?", userID, models.UserLoginDeleted).
		Order("id ASC").Find(&out).Error
	return out, err
}

// loginNameTaken reports whether a login name is in use by any user's
// primary login or by another active login (deployment-wide: local
// sign-in resolves the name before a tenant is known).
func (s *UserLoginService) loginNameTaken(ctx context.Context, name string, exceptLoginID uint) (bool, error) {
	norm := models.NormalizeLoginID(name)
	var n int64
	if err := s.db.WithContext(ctx).Model(&models.User{}).
		Where("LOWER(login_id) = ?", norm).Count(&n).Error; err != nil {
		return false, err
	}
	if n > 0 {
		return true, nil
	}
	q := s.db.WithContext(ctx).Model(&models.UserLogin{}).
		Where("unique_id_normalized = ? AND workflow_state = ?", norm, models.UserLoginActive)
	if exceptLoginID != 0 {
		q = q.Where("id <> ?", exceptLoginID)
	}
	if err := q.Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// sisIDTaken reports whether a SIS user id is already carried by another
// user (primary SIS id, deployment-wide unique) or by a login of another
// user in the tenant.
func (s *UserLoginService) sisIDTaken(ctx context.Context, sisID string, userID, accountID, exceptLoginID uint) (bool, error) {
	var n int64
	if err := s.db.WithContext(ctx).Model(&models.User{}).
		Where("sis_user_id = ? AND id <> ?", sisID, userID).Count(&n).Error; err != nil {
		return false, err
	}
	if n > 0 {
		return true, nil
	}
	q := s.db.WithContext(ctx).Model(&models.UserLogin{}).
		Where("account_id = ? AND sis_user_id = ? AND user_id <> ? AND workflow_state <> ?", accountID, sisID, userID, models.UserLoginDeleted)
	if exceptLoginID != 0 {
		q = q.Where("id <> ?", exceptLoginID)
	}
	if err := q.Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// providerInAccount checks an authentication provider belongs to the
// account.
func (s *UserLoginService) providerInAccount(ctx context.Context, providerID, accountID uint) bool {
	var n int64
	s.db.WithContext(ctx).Model(&models.AuthenticationProvider{}).
		Where("id = ? AND account_id = ?", providerID, accountID).Count(&n)
	return n > 0
}

// apply validates and copies input onto login.
func (s *UserLoginService) apply(ctx context.Context, login *models.UserLogin, user *models.User, in UserLoginInput) error {
	if in.UniqueID != nil {
		name := strings.TrimSpace(*in.UniqueID)
		if name == "" {
			return ErrUserLoginInvalid
		}
		if models.NormalizeLoginID(name) != login.UniqueIDNormalized {
			taken, err := s.loginNameTaken(ctx, name, login.ID)
			if err != nil {
				return err
			}
			if taken {
				return ErrUserLoginTaken
			}
		}
		login.UniqueID = name
	}
	if in.AuthenticationProviderID != nil {
		if *in.AuthenticationProviderID == 0 {
			login.AuthenticationProviderID = nil
		} else {
			if !s.providerInAccount(ctx, *in.AuthenticationProviderID, user.AccountID) {
				return ErrUserLoginProvider
			}
			id := *in.AuthenticationProviderID
			login.AuthenticationProviderID = &id
			login.PasswordHash = "" // an SSO login authenticates at the IdP
		}
	}
	if in.SISUserID != nil {
		sis := strings.TrimSpace(*in.SISUserID)
		if sis == "" {
			login.SISUserID = nil
		} else {
			taken, err := s.sisIDTaken(ctx, sis, user.ID, user.AccountID, login.ID)
			if err != nil {
				return err
			}
			if taken {
				return ErrUserLoginSISTaken
			}
			login.SISUserID = &sis
		}
	}
	if in.Password != nil && *in.Password != "" {
		if login.AuthenticationProviderID != nil {
			return ErrUserLoginProviderPW
		}
		if len(*in.Password) < 8 {
			return ErrUserLoginPassword
		}
		if err := login.SetPassword(*in.Password); err != nil {
			return err
		}
	}
	if in.WorkflowState != nil {
		switch *in.WorkflowState {
		case models.UserLoginActive, models.UserLoginSuspended:
			if *in.WorkflowState == models.UserLoginActive && login.WorkflowState != models.UserLoginActive {
				taken, err := s.loginNameTaken(ctx, login.UniqueID, login.ID)
				if err != nil {
					return err
				}
				if taken {
					return ErrUserLoginTaken
				}
			}
			login.WorkflowState = *in.WorkflowState
		default:
			return errors.New("workflow_state must be active or suspended")
		}
	}
	return nil
}

// Create adds a login to a user of accountID.
func (s *UserLoginService) Create(ctx context.Context, userID, accountID uint, in UserLoginInput) (*models.UserLogin, error) {
	user, err := s.userInAccount(ctx, userID, accountID)
	if err != nil {
		return nil, err
	}
	if in.UniqueID == nil || strings.TrimSpace(*in.UniqueID) == "" {
		return nil, ErrUserLoginInvalid
	}
	login := &models.UserLogin{UserID: user.ID, AccountID: user.AccountID, WorkflowState: models.UserLoginActive}
	if err := s.apply(ctx, login, user, in); err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Create(login).Error; err != nil {
		if isUniqueViolation(err) {
			return nil, ErrUserLoginTaken
		}
		return nil, err
	}
	return login, nil
}

// Get loads one login of a user of accountID.
func (s *UserLoginService) Get(ctx context.Context, userID, loginID, accountID uint) (*models.UserLogin, *models.User, error) {
	user, err := s.userInAccount(ctx, userID, accountID)
	if err != nil {
		return nil, nil, err
	}
	var login models.UserLogin
	if err := s.db.WithContext(ctx).
		Where("id = ? AND user_id = ? AND workflow_state <> ?", loginID, user.ID, models.UserLoginDeleted).
		First(&login).Error; err != nil {
		return nil, nil, ErrUserLoginNotFound
	}
	return &login, user, nil
}

// Update edits a login.
func (s *UserLoginService) Update(ctx context.Context, userID, loginID, accountID uint, in UserLoginInput) (*models.UserLogin, error) {
	login, user, err := s.Get(ctx, userID, loginID, accountID)
	if err != nil {
		return nil, err
	}
	if err := s.apply(ctx, login, user, in); err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Save(login).Error; err != nil {
		if isUniqueViolation(err) {
			return nil, ErrUserLoginTaken
		}
		return nil, err
	}
	return login, nil
}

// Delete soft-deletes a login.
func (s *UserLoginService) Delete(ctx context.Context, userID, loginID, accountID uint) error {
	login, _, err := s.Get(ctx, userID, loginID, accountID)
	if err != nil {
		return err
	}
	login.WorkflowState = models.UserLoginDeleted
	return s.db.WithContext(ctx).Save(login).Error
}

// FindActiveLocal resolves an active local (non-SSO) login by name.
// AUTH-INTERNAL: runs before authentication, so no tenant scope.
func (s *UserLoginService) FindActiveLocal(ctx context.Context, name string) (*models.UserLogin, error) {
	var login models.UserLogin
	err := s.db.WithContext(ctx).
		Where("unique_id_normalized = ? AND workflow_state = ? AND authentication_provider_id IS NULL",
			models.NormalizeLoginID(name), models.UserLoginActive).
		First(&login).Error
	if err != nil {
		return nil, err
	}
	return &login, nil
}

// FindForProvider resolves the login an SSO provider's subject names,
// inside the provider's account. AUTH-INTERNAL (LoginPipeline).
func (s *UserLoginService) FindForProvider(ctx context.Context, providerID, accountID uint, subject string) (*models.UserLogin, error) {
	if subject == "" || accountID == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var login models.UserLogin
	err := s.db.WithContext(ctx).
		Where("account_id = ? AND authentication_provider_id = ? AND unique_id_normalized = ? AND workflow_state = ?",
			accountID, providerID, models.NormalizeLoginID(subject), models.UserLoginActive).
		First(&login).Error
	if err != nil {
		return nil, err
	}
	return &login, nil
}

// TouchLastLogin records a successful sign-in through a login.
func (s *UserLoginService) TouchLastLogin(ctx context.Context, loginID uint) {
	now := time.Now()
	s.db.WithContext(ctx).Model(&models.UserLogin{}).Where("id = ?", loginID).Update("last_login_at", now)
}

// AuthenticateLocal verifies a password against an extra local login and
// returns its user. A login without its own password uses the user's.
// AUTH-INTERNAL: pre-authentication.
func (s *UserLoginService) AuthenticateLocal(ctx context.Context, name, password string) (*models.User, error) {
	login, err := s.FindActiveLocal(ctx, name)
	if err != nil {
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(password))
		return nil, errors.New("invalid credentials")
	}
	var user models.User
	if err := s.db.WithContext(ctx).Where("id = ?", login.UserID).First(&user).Error; err != nil {
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(password))
		return nil, errors.New("invalid credentials")
	}
	if login.HasPassword() {
		if err := login.CheckPassword(password); err != nil {
			return nil, errors.New("invalid credentials")
		}
	} else if err := user.CheckPassword(password); err != nil {
		return nil, errors.New("invalid credentials")
	}
	s.TouchLastLogin(ctx, login.ID)
	return &user, nil
}

// isUniqueViolation reports a Postgres unique-constraint error.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "23505")
}
