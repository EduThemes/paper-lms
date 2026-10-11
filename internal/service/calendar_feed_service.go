package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// CalendarFeedService manages the per-user token behind the subscribable
// ICS feed (G096). The token is a 256-bit random value; the database
// keeps its SHA-256 digest for lookup and a secretbox-sealed copy so the
// owner can see the URL again. When no secretbox key is configured the
// sealed copy is skipped and the URL is only shown when it is (re)made.
type CalendarFeedService struct {
	db   *gorm.DB
	seal func([]byte) ([]byte, error)
	open func([]byte) ([]byte, error)
}

// NewCalendarFeedService takes the secretbox seal/open functions
// (auth.Encrypt / auth.Decrypt — injected because internal/auth imports
// this package). Either may be nil, which disables recovering the URL.
func NewCalendarFeedService(db *gorm.DB, seal, open func([]byte) ([]byte, error)) *CalendarFeedService {
	return &CalendarFeedService{db: db, seal: seal, open: open}
}

// ErrCalendarFeedNotFound covers an unknown token, a deleted or suspended
// owner — all of which the public feed answers with the same 404.
var ErrCalendarFeedNotFound = errors.New("calendar feed not found")

// calendarFeedTokenPrefix keeps feed tokens recognisable in logs and
// secret scanners.
const calendarFeedTokenPrefix = "pcal_"

func digestFeedToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newFeedToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return calendarFeedTokenPrefix + base64.RawURLEncoding.EncodeToString(b), nil
}

// FeedTokenStatus is what the owner sees: whether a feed exists and, when
// the token can be recovered, the token itself.
type FeedTokenStatus struct {
	Exists    bool
	Token     string // empty when not recoverable
	CreatedAt time.Time
}

// Get returns the user's feed status, creating the feed on first use
// when create is true.
func (s *CalendarFeedService) Get(ctx context.Context, userID uint, create bool) (*FeedTokenStatus, error) {
	var row models.CalendarFeedToken
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if !create {
			return &FeedTokenStatus{}, nil
		}
		return s.Rotate(ctx, userID)
	}
	if err != nil {
		return nil, err
	}
	st := &FeedTokenStatus{Exists: true, CreatedAt: row.CreatedAt}
	if len(row.TokenCiphertext) > 0 && s.open != nil {
		if pt, err := s.open(row.TokenCiphertext); err == nil {
			st.Token = string(pt)
		}
	}
	return st, nil
}

// Rotate issues a new token for the user, revoking the old URL.
func (s *CalendarFeedService) Rotate(ctx context.Context, userID uint) (*FeedTokenStatus, error) {
	token, err := newFeedToken()
	if err != nil {
		return nil, err
	}
	var sealed []byte
	if s.seal != nil {
		if ct, err := s.seal([]byte(token)); err == nil {
			sealed = ct
		}
	}
	now := time.Now()
	row := models.CalendarFeedToken{
		UserID:          userID,
		TokenDigest:     digestFeedToken(token),
		TokenCiphertext: sealed,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"token_digest", "token_ciphertext", "created_at", "updated_at", "last_accessed_at"}),
	}).Create(&row).Error; err != nil {
		return nil, err
	}
	return &FeedTokenStatus{Exists: true, Token: token, CreatedAt: now}, nil
}

// Revoke deletes the user's feed token.
func (s *CalendarFeedService) Revoke(ctx context.Context, userID uint) error {
	return s.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&models.CalendarFeedToken{}).Error
}

// Resolve maps a feed token to its owner. Deleted owners and suspended
// owners resolve to ErrCalendarFeedNotFound.
func (s *CalendarFeedService) Resolve(ctx context.Context, token string) (*models.User, error) {
	token = strings.TrimSpace(token)
	if !strings.HasPrefix(token, calendarFeedTokenPrefix) || len(token) < len(calendarFeedTokenPrefix)+40 {
		return nil, ErrCalendarFeedNotFound
	}
	var row models.CalendarFeedToken
	if err := s.db.WithContext(ctx).Where("token_digest = ?", digestFeedToken(token)).First(&row).Error; err != nil {
		return nil, ErrCalendarFeedNotFound
	}
	var user models.User
	if err := s.db.WithContext(ctx).Where("id = ?", row.UserID).First(&user).Error; err != nil {
		return nil, ErrCalendarFeedNotFound
	}
	if user.Suspended {
		return nil, ErrCalendarFeedNotFound
	}
	now := time.Now()
	_ = s.db.WithContext(ctx).Model(&models.CalendarFeedToken{}).Where("id = ?", row.ID).Update("last_accessed_at", now).Error
	return &user, nil
}
