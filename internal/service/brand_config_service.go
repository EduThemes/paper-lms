package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"

	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// BrandConfigService manages an account's theme (G118; Canvas Theme
// Editor). Only allow-listed variables with plain colour values are
// stored, so a theme can never inject CSS: the frontend turns each value
// into a CSS custom property itself.
type BrandConfigService struct {
	db *gorm.DB
}

// NewBrandConfigService builds the service.
func NewBrandConfigService(db *gorm.DB) *BrandConfigService {
	return &BrandConfigService{db: db}
}

// BrandVariable describes one themeable variable.
type BrandVariable struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Default     string `json:"default"`
}

// BrandVariables is the allow-list of theme variables.
func BrandVariables() []BrandVariable {
	return []BrandVariable{
		{"brand_primary", "Primary brand color", "Links, highlights, active navigation and the brand color scale.", "#1B6EB1"},
		{"button_primary", "Primary button", "Background of primary buttons.", "#1B6EB1"},
		{"button_primary_text", "Primary button text", "Text on primary buttons.", "#FFFFFF"},
		{"nav_background", "Navigation background", "Background of the left navigation.", ""},
		{"nav_text", "Navigation text", "Text and icons in the left navigation.", ""},
	}
}

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// MaxBrandLogoBytes caps the logo upload.
const MaxBrandLogoBytes = 512 * 1024

// Errors surfaced to handlers.
var (
	ErrBrandVariable    = errors.New("unknown theme variable")
	ErrBrandColor       = errors.New("theme colors must be #RRGGBB")
	ErrBrandLogoType    = errors.New("the logo must be a PNG, JPEG, GIF or WebP image")
	ErrBrandLogoTooBig  = errors.New("the logo must be 512 KB or smaller")
	ErrBrandLogoMissing = errors.New("no logo")
)

// BrandConfigView is the public shape of a theme.
type BrandConfigView struct {
	AccountID uint              `json:"account_id"`
	Variables map[string]string `json:"variables"`
	HasLogo   bool              `json:"has_logo"`
	Version   int64             `json:"version"` // changes whenever the theme changes (cache-buster)
}

func (s *BrandConfigService) load(ctx context.Context, accountID uint) (*models.BrandConfig, error) {
	var b models.BrandConfig
	err := s.db.WithContext(ctx).Where("account_id = ?", accountID).First(&b).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &b, err
}

func viewOf(accountID uint, b *models.BrandConfig) *BrandConfigView {
	v := &BrandConfigView{AccountID: accountID, Variables: map[string]string{}}
	if b == nil {
		return v
	}
	_ = json.Unmarshal([]byte(b.Variables), &v.Variables)
	v.HasLogo = len(b.LogoData) > 0
	v.Version = b.UpdatedAt.Unix()
	return v
}

// Get returns an account's theme (empty when none is set).
func (s *BrandConfigService) Get(ctx context.Context, accountID uint) (*BrandConfigView, error) {
	b, err := s.load(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return viewOf(accountID, b), nil
}

// ValidateBrandVariables checks keys against the allow-list and values as
// #RRGGBB (an empty value clears the variable).
func ValidateBrandVariables(vars map[string]string) (map[string]string, error) {
	allowed := map[string]bool{}
	for _, v := range BrandVariables() {
		allowed[v.Name] = true
	}
	out := map[string]string{}
	for k, v := range vars {
		if !allowed[k] {
			return nil, ErrBrandVariable
		}
		if v == "" {
			continue
		}
		if !hexColor.MatchString(v) {
			return nil, ErrBrandColor
		}
		out[k] = v
	}
	return out, nil
}

func (s *BrandConfigService) upsert(ctx context.Context, accountID, userID uint, mutate func(b *models.BrandConfig)) (*BrandConfigView, error) {
	b, err := s.load(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if b == nil {
		b = &models.BrandConfig{AccountID: accountID}
	}
	mutate(b)
	b.UpdatedByUserID = userID
	if err := s.db.WithContext(ctx).Save(b).Error; err != nil {
		return nil, err
	}
	return viewOf(accountID, b), nil
}

// UpdateVariables replaces the theme's variables.
func (s *BrandConfigService) UpdateVariables(ctx context.Context, accountID, userID uint, vars map[string]string) (*BrandConfigView, error) {
	clean, err := ValidateBrandVariables(vars)
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(clean)
	return s.upsert(ctx, accountID, userID, func(b *models.BrandConfig) { b.Variables = string(raw) })
}

// SetLogo stores a logo after sniffing its type from the bytes (the
// client-supplied content type is ignored).
func (s *BrandConfigService) SetLogo(ctx context.Context, accountID, userID uint, data []byte) (*BrandConfigView, error) {
	if len(data) == 0 {
		return nil, ErrBrandLogoType
	}
	if len(data) > MaxBrandLogoBytes {
		return nil, ErrBrandLogoTooBig
	}
	ct := http.DetectContentType(data)
	switch ct {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
	default:
		return nil, ErrBrandLogoType
	}
	return s.upsert(ctx, accountID, userID, func(b *models.BrandConfig) {
		b.LogoData = data
		b.LogoContentType = ct
	})
}

// DeleteLogo removes the logo.
func (s *BrandConfigService) DeleteLogo(ctx context.Context, accountID, userID uint) (*BrandConfigView, error) {
	return s.upsert(ctx, accountID, userID, func(b *models.BrandConfig) {
		b.LogoData = nil
		b.LogoContentType = ""
	})
}

// Reset removes the account's theme entirely.
func (s *BrandConfigService) Reset(ctx context.Context, accountID uint) error {
	return s.db.WithContext(ctx).Where("account_id = ?", accountID).Delete(&models.BrandConfig{}).Error
}

// Logo returns the stored logo.
func (s *BrandConfigService) Logo(ctx context.Context, accountID uint) ([]byte, string, error) {
	b, err := s.load(ctx, accountID)
	if err != nil {
		return nil, "", err
	}
	if b == nil || len(b.LogoData) == 0 {
		return nil, "", ErrBrandLogoMissing
	}
	return b.LogoData, b.LogoContentType, nil
}
