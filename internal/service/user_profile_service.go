package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/storage"
)

// UserProfileService manages the public-facing profile: pronouns, a short
// bio, profile links and an uploaded avatar image.
type UserProfileService struct {
	users   repository.UserRepository
	learner repository.LearnerRepository
	store   storage.Backend
}

func NewUserProfileService(users repository.UserRepository, learner repository.LearnerRepository, store storage.Backend) *UserProfileService {
	return &UserProfileService{users: users, learner: learner, store: store}
}

const (
	maxPronounsLen  = 50
	maxBioLen       = 2000
	maxProfileLinks = 10
	maxLinkTitleLen = 255
	maxLinkURLLen   = 2000
	MaxAvatarBytes  = 2 << 20 // 2 MiB
	avatarKeyPrefix = "avatars/"
)

var (
	ErrProfileNotFound = errors.New("user not found")
	ErrInvalidProfile  = errors.New("invalid profile")
	ErrInvalidAvatar   = errors.New("avatar must be a PNG, JPEG, GIF or WebP image of at most 2 MB")
)

// avatarTypes are the image types accepted as avatars, keyed by sniffed
// MIME type. SVG is deliberately absent (it can carry script).
var avatarTypes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// UserProfile is the profile card other people see.
type UserProfile struct {
	ID        uint                     `json:"id"`
	Name      string                   `json:"name"`
	ShortName string                   `json:"short_name"`
	Pronouns  string                   `json:"pronouns"`
	Bio       string                   `json:"bio"`
	AvatarURL string                   `json:"avatar_url"`
	Links     []models.UserProfileLink `json:"links"`
}

// ProfileLinkInput is one link in an update request.
type ProfileLinkInput struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

// ProfileUpdate is the editable part of a profile; nil fields are left as is.
type ProfileUpdate struct {
	Pronouns *string
	Bio      *string
	Links    *[]ProfileLinkInput
}

// normalizeProfileURL accepts only absolute http(s) URLs (no javascript:,
// data: or relative links). Paper never fetches it; it is only rendered as
// a link with rel=noopener, so no SSRF guard is needed.
func normalizeProfileURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > maxLinkURLLen {
		return "", fmt.Errorf("%w: link url is required", ErrInvalidProfile)
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("%w: link url must be an http(s) address", ErrInvalidProfile)
	}
	return u.String(), nil
}

// Get returns userID's profile card. The viewer must be the user, share an
// active course with them, or be an admin (canSeeAll); anyone else gets
// ErrProfileNotFound, as does a user in another tenant.
func (s *UserProfileService) Get(ctx context.Context, viewerID, userID, accountID uint, canSeeAll bool) (*UserProfile, error) {
	u, err := s.users.FindByID(ctx, userID, accountID)
	if err != nil || u == nil {
		return nil, ErrProfileNotFound
	}
	if viewerID != userID && !canSeeAll {
		shared, err := s.learner.UsersShareCourse(ctx, viewerID, userID)
		if err != nil || !shared {
			return nil, ErrProfileNotFound
		}
	}
	links, err := s.learner.ListProfileLinks(ctx, userID)
	if err != nil {
		return nil, err
	}
	if links == nil {
		links = []models.UserProfileLink{}
	}
	return &UserProfile{
		ID: u.ID, Name: u.Name, ShortName: u.ShortName, Pronouns: u.Pronouns,
		Bio: u.Bio, AvatarURL: u.AvatarURL, Links: links,
	}, nil
}

// Update edits the caller's own profile.
func (s *UserProfileService) Update(ctx context.Context, userID, accountID uint, in ProfileUpdate) (*UserProfile, error) {
	u, err := s.users.FindByID(ctx, userID, accountID)
	if err != nil || u == nil {
		return nil, ErrProfileNotFound
	}
	if in.Pronouns != nil {
		p := strings.TrimSpace(SanitizePlainText(*in.Pronouns))
		if len([]rune(p)) > maxPronounsLen {
			return nil, fmt.Errorf("%w: pronouns must be %d characters or fewer", ErrInvalidProfile, maxPronounsLen)
		}
		u.Pronouns = p
	}
	if in.Bio != nil {
		b := strings.TrimSpace(SanitizePlainText(*in.Bio))
		if len([]rune(b)) > maxBioLen {
			return nil, fmt.Errorf("%w: bio must be %d characters or fewer", ErrInvalidProfile, maxBioLen)
		}
		u.Bio = b
	}
	var links []models.UserProfileLink
	if in.Links != nil {
		if len(*in.Links) > maxProfileLinks {
			return nil, fmt.Errorf("%w: at most %d links", ErrInvalidProfile, maxProfileLinks)
		}
		for _, l := range *in.Links {
			if strings.TrimSpace(l.Title) == "" && strings.TrimSpace(l.URL) == "" {
				continue
			}
			link, err := normalizeProfileURL(l.URL)
			if err != nil {
				return nil, err
			}
			title := strings.TrimSpace(SanitizePlainText(l.Title))
			if title == "" {
				title = link
			}
			if len([]rune(title)) > maxLinkTitleLen {
				return nil, fmt.Errorf("%w: link title is too long", ErrInvalidProfile)
			}
			links = append(links, models.UserProfileLink{Title: title, URL: link})
		}
	}
	if in.Pronouns != nil || in.Bio != nil {
		if err := s.users.Update(ctx, u); err != nil {
			return nil, err
		}
	}
	if in.Links != nil {
		if err := s.learner.ReplaceProfileLinks(ctx, userID, links); err != nil {
			return nil, err
		}
	}
	return s.Get(ctx, userID, userID, accountID, true)
}

// AvatarPath is the URL stored in users.avatar_url for an uploaded avatar.
// The version query busts caches when the image changes.
func AvatarPath(userID uint, version string) string {
	return fmt.Sprintf("/api/v1/users/%d/avatar?v=%s", userID, version)
}

// SetAvatar stores an uploaded image as the caller's avatar. The content
// type is sniffed from the bytes, never trusted from the client.
func (s *UserProfileService) SetAvatar(ctx context.Context, userID, accountID uint, r io.Reader) (*models.User, error) {
	if s.store == nil {
		return nil, errors.New("file storage is not configured")
	}
	u, err := s.users.FindByID(ctx, userID, accountID)
	if err != nil || u == nil {
		return nil, ErrProfileNotFound
	}
	data, err := io.ReadAll(io.LimitReader(r, MaxAvatarBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > MaxAvatarBytes {
		return nil, ErrInvalidAvatar
	}
	mime := http.DetectContentType(data)
	ext, ok := avatarTypes[mime]
	if !ok {
		return nil, ErrInvalidAvatar
	}
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	version := hex.EncodeToString(buf)
	key := fmt.Sprintf("%s%d/%d/%s%s", avatarKeyPrefix, u.AccountID, u.ID, version, ext)
	if err := s.store.Put(ctx, key, bytes.NewReader(data), mime); err != nil {
		return nil, err
	}
	old := u.AvatarKey
	u.AvatarKey = key
	u.AvatarURL = AvatarPath(u.ID, version)
	if err := s.users.Update(ctx, u); err != nil {
		_ = s.store.Delete(ctx, key)
		return nil, err
	}
	if old != "" && strings.HasPrefix(old, avatarKeyPrefix) {
		_ = s.store.Delete(ctx, old)
	}
	return u, nil
}

// RemoveAvatar clears the caller's uploaded avatar.
func (s *UserProfileService) RemoveAvatar(ctx context.Context, userID, accountID uint) error {
	u, err := s.users.FindByID(ctx, userID, accountID)
	if err != nil || u == nil {
		return ErrProfileNotFound
	}
	old := u.AvatarKey
	u.AvatarKey = ""
	if strings.HasPrefix(u.AvatarURL, fmt.Sprintf("/api/v1/users/%d/avatar", u.ID)) {
		u.AvatarURL = ""
	}
	if err := s.users.Update(ctx, u); err != nil {
		return err
	}
	if old != "" && s.store != nil && strings.HasPrefix(old, avatarKeyPrefix) {
		_ = s.store.Delete(ctx, old)
	}
	return nil
}

// Avatar is an avatar image ready to stream.
type Avatar struct {
	Body        io.ReadCloser
	ContentType string
	ModTime     time.Time
}

// OpenAvatar opens userID's uploaded avatar. Any signed-in user of the same
// tenant may load it (avatars appear next to names everywhere); another
// tenant's user, or a user without an uploaded avatar, is ErrProfileNotFound.
func (s *UserProfileService) OpenAvatar(ctx context.Context, userID, accountID uint) (*Avatar, error) {
	if s.store == nil {
		return nil, ErrProfileNotFound
	}
	u, err := s.users.FindByID(ctx, userID, accountID)
	if err != nil || u == nil || u.AvatarKey == "" || !strings.HasPrefix(u.AvatarKey, avatarKeyPrefix) {
		return nil, ErrProfileNotFound
	}
	body, err := s.store.Get(ctx, u.AvatarKey)
	if err != nil {
		return nil, ErrProfileNotFound
	}
	ct := "application/octet-stream"
	for mime, ext := range avatarTypes {
		if strings.HasSuffix(u.AvatarKey, ext) {
			ct = mime
		}
	}
	return &Avatar{Body: body, ContentType: ct, ModTime: u.UpdatedAt}, nil
}
