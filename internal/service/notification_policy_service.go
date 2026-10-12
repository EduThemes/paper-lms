package service

import (
	"context"
	"errors"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// policyKey identifies one cell of the category x channel grid.
type policyKey struct {
	channelID uint
	category  string
}

// explicitPolicies loads the user's explicit grid cells. A lookup failure
// degrades to "no overrides" (the global preference applies) rather than
// dropping the notification.
func (s *NotificationDeliveryService) explicitPolicies(ctx context.Context, userID uint) map[policyKey]string {
	out := map[policyKey]string{}
	if s.policies == nil {
		return out
	}
	rows, err := s.policies.ListByUser(ctx, userID)
	if err != nil {
		return out
	}
	for _, r := range rows {
		out[policyKey{r.CommunicationChannelID, r.Category}] = r.Frequency
	}
	return out
}

// legacyFrequency is the frequency the global NotificationPreference gives
// a category: its policy, unless the policy is "never" or the category's
// (or the type's) per-type toggle is off.
func (s *NotificationDeliveryService) legacyFrequency(prefs *models.NotificationPreference, category, notificationType string) string {
	if prefs == nil {
		prefs = DefaultNotificationPreference(0)
	}
	if prefs.Policy == models.NotificationFrequencyNever {
		return models.NotificationFrequencyNever
	}
	if c, ok := notificationCategoryByKey(category); ok && c.legacyToggle != "" && !s.isNotificationTypeEnabled(prefs, c.legacyToggle) {
		return models.NotificationFrequencyNever
	}
	if notificationType != "" && !s.isNotificationTypeEnabled(prefs, notificationType) {
		return models.NotificationFrequencyNever
	}
	if IsNotificationFrequency(prefs.Policy) {
		return prefs.Policy
	}
	return models.NotificationFrequencyDaily
}

// frequencyFor resolves how often a notification of category reaches the
// user on one channel: the explicit cell when set, else the global
// preference.
func (s *NotificationDeliveryService) frequencyFor(prefs *models.NotificationPreference, overrides map[policyKey]string, channelID uint, category, notificationType string) string {
	if f, ok := overrides[policyKey{channelID, category}]; ok && IsNotificationFrequency(f) {
		return f
	}
	return s.legacyFrequency(prefs, category, notificationType)
}

// PolicyChannel is one column of the preference grid.
type PolicyChannel struct {
	ID          uint   `json:"id"`
	ChannelType string `json:"channel_type"`
	Address     string `json:"address"`
	Primary     bool   `json:"primary"`
	Confirmed   bool   `json:"confirmed"`
	// Deliverable is false for a channel that receives nothing right now
	// (unconfirmed, or a stub type without a provider).
	Deliverable bool `json:"deliverable"`
}

// PolicyCell is one cell of the grid. Explicit is false when the value is
// inherited from the global preference.
type PolicyCell struct {
	ChannelID uint   `json:"channel_id"`
	Category  string `json:"category"`
	Frequency string `json:"frequency"`
	Explicit  bool   `json:"explicit"`
}

// PolicyGrid is the full per-category x per-channel preference view.
type PolicyGrid struct {
	Categories   []NotificationCategory `json:"categories"`
	Channels     []PolicyChannel        `json:"channels"`
	Policies     []PolicyCell           `json:"policies"`
	Frequencies  []string               `json:"frequencies"`
	ChannelTypes []ChannelTypeInfo      `json:"channel_types"`
}

// PolicyChannels lists the user's grid columns: the account email first,
// then every active communication channel.
//
// A channel whose address repeats the account email (or an earlier
// deliverable channel) is left out: deliveryTargets folds such duplicates
// into the first target, so a column for one would hold settings that no
// delivery ever reads.
func (s *NotificationDeliveryService) PolicyChannels(ctx context.Context, userID uint) []PolicyChannel {
	var out []PolicyChannel
	seen := map[string]bool{}
	key := func(channelType, address string) string {
		return channelType + ":" + strings.ToLower(strings.TrimSpace(address))
	}
	if s.userRepo != nil {
		// Self lookup: userID is the authenticated caller.
		if u, err := s.userRepo.FindByID(ctx, userID, 0); err == nil && u != nil && strings.TrimSpace(u.Email) != "" {
			out = append(out, PolicyChannel{ID: models.PrimaryEmailChannelID, ChannelType: "email", Address: u.Email, Primary: true, Confirmed: true, Deliverable: true})
			seen[key("email", u.Email)] = true
		}
	}
	if s.channelRepo != nil {
		if chans, err := s.channelRepo.ListByUserID(ctx, userID); err == nil {
			for _, ch := range chans {
				if ch.WorkflowState != "active" {
					continue
				}
				k := key(ch.ChannelType, ch.Address)
				if seen[k] {
					continue
				}
				deliverable := ch.Confirmed && s.IsDeliverableChannelType(ch.ChannelType)
				if deliverable {
					seen[k] = true
				}
				out = append(out, PolicyChannel{
					ID: ch.ID, ChannelType: ch.ChannelType, Address: ch.Address, Confirmed: ch.Confirmed,
					Deliverable: deliverable,
				})
			}
		}
	}
	return out
}

// EffectivePolicies builds the grid: every category x every channel with
// the frequency that would apply today.
func (s *NotificationDeliveryService) EffectivePolicies(ctx context.Context, userID uint) (*PolicyGrid, error) {
	prefs, err := s.prefRepo.FindByUserID(ctx, userID)
	if err != nil || prefs == nil {
		prefs = DefaultNotificationPreference(userID)
	}
	overrides := s.explicitPolicies(ctx, userID)
	channels := s.PolicyChannels(ctx, userID)
	grid := &PolicyGrid{
		Categories:   NotificationCategories,
		Channels:     channels,
		Frequencies:  []string{"immediately", "daily", "weekly", "never"},
		ChannelTypes: s.ChannelTypes(),
	}
	for _, ch := range channels {
		for _, c := range NotificationCategories {
			cell := PolicyCell{ChannelID: ch.ID, Category: c.Key}
			if f, ok := overrides[policyKey{ch.ID, c.Key}]; ok && IsNotificationFrequency(f) {
				cell.Frequency, cell.Explicit = f, true
			} else {
				cell.Frequency = s.legacyFrequency(prefs, c.Key, "")
			}
			grid.Policies = append(grid.Policies, cell)
		}
	}
	return grid, nil
}

// ErrUnknownPolicyChannel is returned when the channel is not one of the
// caller's own active channels.
var ErrUnknownPolicyChannel = errors.New("communication channel not found")

// SetPolicy writes one grid cell for the caller. frequency "" (or
// "default") removes the explicit cell so it inherits again.
func (s *NotificationDeliveryService) SetPolicy(ctx context.Context, userID, channelID uint, category, frequency string) error {
	if s.policies == nil {
		return errors.New("notification policies are not available")
	}
	if !IsNotificationCategory(category) {
		return errors.New("unknown notification category")
	}
	owned := false
	for _, ch := range s.PolicyChannels(ctx, userID) {
		if ch.ID == channelID {
			owned = true
			break
		}
	}
	if !owned {
		return ErrUnknownPolicyChannel
	}
	if frequency == "" || frequency == "default" {
		return s.policies.Delete(ctx, userID, channelID, category)
	}
	if !IsNotificationFrequency(frequency) {
		return errors.New("frequency must be one of: immediately, daily, weekly, never")
	}
	return s.policies.Upsert(ctx, &models.NotificationPolicy{
		UserID: userID, CommunicationChannelID: channelID, Category: category, Frequency: frequency,
	})
}

// ForgetChannelPolicies drops a removed channel's grid cells.
func (s *NotificationDeliveryService) ForgetChannelPolicies(ctx context.Context, userID, channelID uint) {
	if s.policies == nil || channelID == models.PrimaryEmailChannelID {
		return
	}
	_ = s.policies.DeleteByChannel(ctx, userID, channelID)
}
