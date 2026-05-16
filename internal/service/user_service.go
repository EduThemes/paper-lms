package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type UserService struct {
	repo repository.UserRepository
}

func NewUserService(repo repository.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) Register(ctx context.Context, name, email, password string) (*models.User, error) {
	if name == "" || email == "" || password == "" {
		return nil, errors.New("name, email, and password are required")
	}
	if len(password) < 8 {
		return nil, errors.New("password must be at least 8 characters")
	}

	existing, _ := s.repo.FindByEmail(ctx, email)
	if existing != nil {
		return nil, errors.New("user already exists")
	}

	sortableName := name
	if parts := strings.SplitN(name, " ", 2); len(parts) == 2 {
		sortableName = parts[1] + ", " + parts[0]
	}

	user := &models.User{
		Name:         name,
		SortableName: sortableName,
		ShortName:    name,
		LoginID:      email,
		Email:        email,
	}

	if err := user.HashPassword(password); err != nil {
		return nil, err
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *UserService) Authenticate(ctx context.Context, loginID, password string) (*models.User, error) {
	user, err := s.repo.FindByLoginID(ctx, loginID)
	if err != nil {
		// Fallback to email lookup
		user, err = s.repo.FindByEmail(ctx, loginID)
		if err != nil {
			return nil, errors.New("invalid credentials")
		}
	}

	if err := user.CheckPassword(password); err != nil {
		return nil, errors.New("invalid credentials")
	}

	return user, nil
}

func (s *UserService) GetByID(ctx context.Context, id uint) (*models.User, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *UserService) Update(ctx context.Context, user *models.User) error {
	return s.repo.Update(ctx, user)
}

func (s *UserService) List(ctx context.Context, params repository.PaginationParams) (*repository.PaginatedResult[models.User], error) {
	return s.repo.List(ctx, params)
}

func (s *UserService) Search(ctx context.Context, searchTerm string, params repository.PaginationParams) (*repository.PaginatedResult[models.User], error) {
	return s.repo.Search(ctx, searchTerm, params)
}

func (s *UserService) RequestPasswordReset(ctx context.Context, email string) (string, error) {
	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		// Don't reveal whether the email exists
		return "", nil
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", errors.New("could not generate reset token")
	}
	token := hex.EncodeToString(b)
	expires := time.Now().Add(1 * time.Hour)

	user.ResetToken = token
	user.ResetTokenExpiresAt = &expires
	if err := s.repo.Update(ctx, user); err != nil {
		return "", errors.New("could not save reset token")
	}

	return token, nil
}

func (s *UserService) ResetPassword(ctx context.Context, token, newPassword string) error {
	if token == "" {
		return errors.New("reset token is required")
	}
	if len(newPassword) < 8 {
		return errors.New("password must be at least 8 characters")
	}

	user, err := s.repo.FindByResetToken(ctx, token)
	if err != nil {
		return errors.New("invalid or expired reset token")
	}

	if user.ResetTokenExpiresAt == nil || user.ResetTokenExpiresAt.Before(time.Now()) {
		return errors.New("reset token has expired")
	}

	if err := user.HashPassword(newPassword); err != nil {
		return err
	}

	user.ResetToken = ""
	user.ResetTokenExpiresAt = nil
	return s.repo.Update(ctx, user)
}

// ChangePassword verifies the current password and sets a new one for a
// logged-in user. The current-password check defends against an attacker
// with a stolen session: a session alone shouldn't be enough to pivot to
// permanent account takeover via password change.
func (s *UserService) ChangePassword(ctx context.Context, userID uint, currentPassword, newPassword string) error {
	if len(newPassword) < 8 {
		return errors.New("password must be at least 8 characters")
	}

	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return errors.New("user not found")
	}

	if err := user.CheckPassword(currentPassword); err != nil {
		return errors.New("current password is incorrect")
	}

	if currentPassword == newPassword {
		return errors.New("new password must differ from current password")
	}

	if err := user.HashPassword(newPassword); err != nil {
		return err
	}
	return s.repo.Update(ctx, user)
}
