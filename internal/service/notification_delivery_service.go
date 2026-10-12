package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/security"
	"github.com/EduThemes/paper-lms/internal/settingsctx"
)

// SMTPConfig holds the configuration for outbound email delivery.
// Kept as a struct so the resolved-effective view can be passed
// around internally. Construction now happens per-send (see
// resolveSMTPConfig) so cred rotation via the super-admin panel
// takes effect without a restart.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	Enabled  bool
}

// SettingsLookupFunc is the type NotificationDeliveryService accepts
// for resolving live config values. Function-typed rather than
// interface-typed for two reasons:
//
//  1. It breaks an import cycle. internal/auth imports internal/service
//     (via auth_audit.go). If this package imported
//     internal/service/settings (which imports internal/auth for
//     secretbox), Go rejects the cycle. A bare function type lets
//     the caller (cmd/server/main.go) wire the closure that holds
//     the settings.Service reference.
//  2. Tests can stub trivially with a literal map lookup.
//
// Empty string + nil error means "no value in the resolution chain";
// non-nil error means "the lookup itself failed transiently."
type SettingsLookupFunc func(ctx context.Context, key string) (string, error)

// NotificationDeliveryService is the core notification dispatch engine.
type NotificationDeliveryService struct {
	deliveryRepo postgres.NotificationDeliveryRepository
	channelRepo  postgres.CommunicationChannelRepository
	prefRepo     repository.NotificationPreferenceRepository
	notifRepo    repository.NotificationRepository
	userRepo     repository.UserRepository
	lookup       SettingsLookupFunc
	// policies holds the per-category x per-channel overrides (G091).
	// Nil means every delivery follows the global NotificationPreference.
	policies NotificationPolicyStore
	// senders are pluggable transports for channel types beyond the
	// built-in email and webhook (SMS, push). A type without a registered
	// sender is a stub: it is listed as unavailable and never queued.
	senders map[string]ChannelSender
}

// NotificationPolicyStore is the persistence the per-category x
// per-channel preferences need (postgres.NotificationPolicyRepository).
type NotificationPolicyStore interface {
	ListByUser(ctx context.Context, userID uint) ([]models.NotificationPolicy, error)
	Upsert(ctx context.Context, p *models.NotificationPolicy) error
	Delete(ctx context.Context, userID, channelID uint, category string) error
	DeleteByChannel(ctx context.Context, userID, channelID uint) error
}

// ChannelSender delivers one notification to one address over a transport
// the engine does not implement itself (an SMS gateway, a push service).
type ChannelSender interface {
	Send(ctx context.Context, address, subject, body string) error
}

// SetPolicyStore wires the per-category x per-channel preferences.
func (s *NotificationDeliveryService) SetPolicyStore(p NotificationPolicyStore) {
	s.policies = p
}

// RegisterChannelSender makes channelType ("sms", "push") deliverable via
// sender. Without a registration that channel type stays a stub.
func (s *NotificationDeliveryService) RegisterChannelSender(channelType string, sender ChannelSender) {
	if s.senders == nil {
		s.senders = map[string]ChannelSender{}
	}
	s.senders[channelType] = sender
}

// ChannelTypeInfo describes one communication channel type for the UI.
type ChannelTypeInfo struct {
	Type      string `json:"type"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

// stubChannelTypes are the channel types Canvas offers that this
// deployment can only deliver to once a provider is registered.
var stubChannelTypes = []string{"sms", "push"}

// ChannelTypes lists every channel type and whether it can deliver here.
func (s *NotificationDeliveryService) ChannelTypes() []ChannelTypeInfo {
	out := []ChannelTypeInfo{{Type: "email", Available: true}, {Type: "webhook", Available: true}}
	for _, t := range stubChannelTypes {
		if s.senders[t] != nil {
			out = append(out, ChannelTypeInfo{Type: t, Available: true})
			continue
		}
		label := map[string]string{"sms": "SMS", "push": "push notification"}[t]
		out = append(out, ChannelTypeInfo{Type: t, Available: false, Reason: "No " + label + " provider is configured for this site."})
	}
	return out
}

// IsDeliverableChannelType reports whether notifications can be sent to a
// channel of this type (built-in email/webhook, or a registered sender).
func (s *NotificationDeliveryService) IsDeliverableChannelType(channelType string) bool {
	if channelType == "email" || channelType == "webhook" {
		return true
	}
	return s.senders[channelType] != nil
}

// NewNotificationDeliveryService creates a new NotificationDeliveryService
// with all required dependencies. The lookup function resolves SMTP
// settings per-send so cred rotation via the super-admin UI takes
// effect immediately. Pass nil only in tests that don't exercise the
// email path.
//
// Wave 5: this signature dropped the SMTPConfig parameter. The
// catalog (internal/service/settings/catalog.go) carries the env
// fallback names (SMTP_HOST, SMTP_PORT, …) so deployments that have
// never touched the settings UI keep working unchanged — the
// resolution chain returns the env value with source="env".
func NewNotificationDeliveryService(
	deliveryRepo postgres.NotificationDeliveryRepository,
	channelRepo postgres.CommunicationChannelRepository,
	prefRepo repository.NotificationPreferenceRepository,
	notifRepo repository.NotificationRepository,
	userRepo repository.UserRepository,
	lookup SettingsLookupFunc,
) *NotificationDeliveryService {
	return &NotificationDeliveryService{
		deliveryRepo: deliveryRepo,
		channelRepo:  channelRepo,
		prefRepo:     prefRepo,
		notifRepo:    notifRepo,
		userRepo:     userRepo,
		lookup:       lookup,
	}
}

// scopeCtxFor stamps the per-tenant account hint onto ctx so the
// SMTP resolution chain walks the recipient's account scope (and
// the account's parent chain) before falling through to instance/
// env/default. Wave 8 — unlocks per-district SMTP creds.
//
// A failed user lookup or a user with AccountID=0 falls through to
// the unscoped ctx — back-compat with single-tenant deployments
// where every recipient resolves the same SMTP.
func (s *NotificationDeliveryService) scopeCtxFor(ctx context.Context, userID uint) context.Context {
	if userID == 0 {
		return ctx
	}
	// Settings context derivation: the whole point of this lookup is
	// to discover the user's account_id. accountID=0 is correct
	// (we're learning the tenant, not enforcing it).
	u, err := s.userRepo.FindByID(ctx, userID, 0)
	if err != nil || u == nil || u.AccountID == 0 {
		return ctx
	}
	return settingsctx.WithAccountID(ctx, u.AccountID)
}

// resolveSMTPConfig fetches the live SMTP configuration via the
// settings lookup. Called on every Send so changes to SMTP creds
// take effect immediately — the whole point of the Settings Engine.
//
// The lookup function (wired in main.go) walks the resolution chain
// (account → instance → env → default) internally; this function
// just reads each catalog key and assembles the struct. A missing or
// unparseable port falls back to 587 — the same default the catalog
// declares.
func (s *NotificationDeliveryService) resolveSMTPConfig(ctx context.Context) (SMTPConfig, error) {
	if s.lookup == nil {
		return SMTPConfig{}, fmt.Errorf("notification delivery: settings lookup not wired")
	}

	read := func(key string) (string, error) {
		v, err := s.lookup(ctx, key)
		if err != nil {
			return "", fmt.Errorf("settings %s: %w", key, err)
		}
		return v, nil
	}

	host, err := read("smtp.host")
	if err != nil {
		return SMTPConfig{}, err
	}
	portStr, err := read("smtp.port")
	if err != nil {
		return SMTPConfig{}, err
	}
	username, err := read("smtp.username")
	if err != nil {
		return SMTPConfig{}, err
	}
	password, err := read("smtp.password")
	if err != nil {
		return SMTPConfig{}, err
	}
	from, err := read("smtp.from")
	if err != nil {
		return SMTPConfig{}, err
	}
	enabledStr, err := read("smtp.enabled")
	if err != nil {
		return SMTPConfig{}, err
	}

	port, _ := strconv.Atoi(strings.TrimSpace(portStr))
	if port == 0 {
		port = 587 // catalog default; explicit so a bad parse doesn't yield port 0
	}

	// Enabled parsing matches the pre-Wave-5 strict semantics from
	// internal/config/config.go: only the literal lowercase string
	// "true" counts. Operators using non-canonical casing (e.g.
	// SMTP_ENABLED=True) as a soft-disable keep working — Wave 5
	// audit H1 caught the widened parse before it shipped. Values
	// written through the super-admin UI are normalized to
	// lowercase by the catalog's validator at write time, so this
	// only affects env-var-driven deployments.
	return SMTPConfig{
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		From:     from,
		Enabled:  enabledStr == "true",
	}, nil
}

// DefaultNotificationPreference is the preference set a user has before
// they ever save the notification-preferences page. It is the single
// source of truth for both NotificationService.GetOrCreatePreferences and
// QueueNotification, so a user without a stored row is treated exactly as
// if the defaults had been saved.
func DefaultNotificationPreference(userID uint) *models.NotificationPreference {
	return &models.NotificationPreference{
		UserID:                userID,
		Policy:                "daily",
		NotifyNewMessage:      true,
		NotifyEventStart:      false,
		NotifySubmissionGrade: true,
		NotifyNewAnnouncement: true,
	}
}

// deliveryTarget is one address a notification is sent to. ChannelID is
// the communication_channels row, or models.PrimaryEmailChannelID for the
// account email.
type deliveryTarget struct {
	ChannelID   uint
	ChannelType string
	Address     string
}

// deliveryTargets resolves where a user's notifications go: the user's
// account email (the confirmed login identity) plus every additional
// channel that is active, confirmed and of a deliverable type. An
// unconfirmed address never receives mail — the owner must prove they
// control it first (see ConfirmChannel).
func (s *NotificationDeliveryService) deliveryTargets(ctx context.Context, userID uint) ([]deliveryTarget, error) {
	var targets []deliveryTarget
	seen := map[string]bool{}

	// Background job: the notification was raised for this user by a
	// tenant-checked request. accountID=0 is correct here.
	user, userErr := s.userRepo.FindByID(ctx, userID, 0)
	if userErr == nil && user != nil && strings.TrimSpace(user.Email) != "" {
		targets = append(targets, deliveryTarget{ChannelID: models.PrimaryEmailChannelID, ChannelType: "email", Address: user.Email})
		seen["email:"+strings.ToLower(strings.TrimSpace(user.Email))] = true
	}

	if s.channelRepo != nil {
		channels, err := s.channelRepo.ListByUserID(ctx, userID)
		if err == nil {
			for _, ch := range channels {
				if !ch.Confirmed || ch.WorkflowState != "active" || !s.IsDeliverableChannelType(ch.ChannelType) {
					continue
				}
				key := ch.ChannelType + ":" + strings.ToLower(strings.TrimSpace(ch.Address))
				if seen[key] {
					continue
				}
				seen[key] = true
				targets = append(targets, deliveryTarget{ChannelID: ch.ID, ChannelType: ch.ChannelType, Address: ch.Address})
			}
		}
	}

	if len(targets) == 0 && userErr != nil {
		return nil, fmt.Errorf("failed to find user %d: %w", userID, userErr)
	}
	return targets, nil
}

// QueueNotification records a notification for the user and queues its
// deliveries.
//
//  1. An in-app notification row is always created (the /notifications
//     feed), like Canvas's stream items — preferences govern delivery
//     channels, not the in-app feed.
//  2. For each deliverable target (account email plus confirmed channels)
//     the frequency is the user's explicit policy for that channel and the
//     type's category (G091), falling back to the global preference (or
//     DefaultNotificationPreference when none is stored). "immediately"
//     deliveries are sent by the per-minute job, daily/weekly ones are
//     batched into digests, and "never" queues nothing on that channel.
func (s *NotificationDeliveryService) QueueNotification(
	ctx context.Context,
	userID uint,
	notificationType string,
	subject string,
	body string,
	contextType string,
	contextID uint,
) error {
	var notificationID uint
	if s.notifRepo != nil {
		now := time.Now()
		n := &models.Notification{
			UserID:           userID,
			NotificationType: notificationType,
			Title:            subject,
			Message:          notificationPreview(body),
			ContextType:      contextType,
			ContextID:        contextID,
			SentAt:           &now,
		}
		if err := s.notifRepo.Create(ctx, n); err == nil {
			notificationID = n.ID
		}
	}

	prefs, err := s.prefRepo.FindByUserID(ctx, userID)
	if err != nil || prefs == nil {
		prefs = DefaultNotificationPreference(userID)
	}
	category := NotificationCategoryFor(notificationType)
	overrides := s.explicitPolicies(ctx, userID)

	targets, err := s.deliveryTargets(ctx, userID)
	if err != nil {
		return err
	}

	for _, t := range targets {
		frequency := s.frequencyFor(prefs, overrides, t.ChannelID, category, notificationType)
		if frequency == models.NotificationFrequencyNever {
			continue
		}
		digestType := s.policyToDigestType(frequency)
		scheduledFor := s.calculateScheduledFor(digestType)
		delivery := &models.NotificationDelivery{
			NotificationID: notificationID,
			UserID:         userID,
			ChannelType:    t.ChannelType,
			Address:        t.Address,
			Subject:        subject,
			Body:           body,
			DeliveryStatus: "pending",
			DigestType:     digestType,
			MaxRetries:     3,
			ScheduledFor:   &scheduledFor,
		}
		if err := s.deliveryRepo.Create(ctx, delivery); err != nil {
			return fmt.Errorf("failed to create %s delivery: %w", t.ChannelType, err)
		}
	}

	return nil
}

// LatestNotificationAt returns when the user was last sent a notification
// of notificationType about (contextType, contextID), looking at their
// most recent in-app notifications; nil when none is found. Used to avoid
// re-sending notifications that were already delivered.
func (s *NotificationDeliveryService) LatestNotificationAt(ctx context.Context, userID uint, notificationType, contextType string, contextID uint) *time.Time {
	if s == nil || s.notifRepo == nil {
		return nil
	}
	// Background lookup for a user a tenant-checked request already
	// resolved: accountID=0.
	page, err := s.notifRepo.ListByUserID(ctx, userID, 0, repository.PaginationParams{Page: 1, PerPage: 200})
	if err != nil || page == nil {
		return nil
	}
	for _, n := range page.Items { // newest first
		if n.NotificationType == notificationType && n.ContextType == contextType && n.ContextID == contextID {
			at := n.CreatedAt
			return &at
		}
	}
	return nil
}

// notificationPreview turns an (HTML) notification body into the short
// plain-text preview shown in the in-app feed, which renders text.
func notificationPreview(body string) string {
	text := html.UnescapeString(stripHTML(body))
	const maxLen = 280
	if r := []rune(text); len(r) > maxLen {
		text = string(r[:maxLen-1]) + "…"
	}
	return text
}

// channelConfirmCodeHash is what is stored in communication_channels.
// confirm_code: the SHA-256 of the code, never the code itself.
func channelConfirmCodeHash(code string) string {
	sum := sha256.Sum256([]byte(strings.ToUpper(strings.TrimSpace(code))))
	return hex.EncodeToString(sum[:])
}

// newChannelConfirmCode returns a 10-character code from an unambiguous
// alphabet (no 0/O, 1/I) that the user types back in.
func newChannelConfirmCode() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, len(buf))
	for i, b := range buf {
		out[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(out), nil
}

// BeginChannelConfirmation prepares a newly added channel for delivery.
// Email channels get a one-time code (stored hashed) that is mailed to
// the address; the channel receives no notifications until the owner
// enters the code (ConfirmChannel). Webhook channels are owner-supplied
// URLs behind the SSRF guard and are confirmed immediately. SMS channels
// (only accepted once an SMS provider is registered) get a code by text,
// like email. The caller persists the channel after this returns.
func (s *NotificationDeliveryService) BeginChannelConfirmation(ch *models.CommunicationChannel) (code string, err error) {
	if ch.ChannelType != "email" && ch.ChannelType != "sms" {
		now := time.Now()
		ch.Confirmed = true
		ch.ConfirmedAt = &now
		ch.ConfirmCode = ""
		return "", nil
	}
	code, err = newChannelConfirmCode()
	if err != nil {
		return "", err
	}
	ch.Confirmed = false
	ch.ConfirmedAt = nil
	ch.ConfirmCode = channelConfirmCodeHash(code)
	return code, nil
}

// SendChannelConfirmation mails the confirmation code to a new email
// channel. Best-effort: a failure leaves the channel unconfirmed.
func (s *NotificationDeliveryService) SendChannelConfirmation(ctx context.Context, ch *models.CommunicationChannel, code string) error {
	if code == "" {
		return nil
	}
	if ch.ChannelType == "sms" {
		if sender := s.senders["sms"]; sender != nil {
			return sender.Send(ctx, ch.Address, "Paper LMS", "Your Paper LMS confirmation code is "+code)
		}
		return nil
	}
	if ch.ChannelType != "email" {
		return nil
	}
	body := fmt.Sprintf("<p>Enter this code in Paper LMS to start receiving notifications at this address:</p><p><strong>%s</strong></p><p>If you did not add this address, you can ignore this email.</p>", code)
	return s.SendEmail(s.scopeCtxFor(ctx, ch.UserID), ch.Address, "Confirm your notification email", body)
}

// ErrInvalidConfirmationCode is returned by ConfirmChannel on a wrong code.
var ErrInvalidConfirmationCode = errors.New("invalid confirmation code")

// ConfirmChannel marks the channel confirmed when code matches the one
// issued by BeginChannelConfirmation, and persists it.
func (s *NotificationDeliveryService) ConfirmChannel(ctx context.Context, ch *models.CommunicationChannel, code string) error {
	if ch.Confirmed {
		return nil
	}
	if ch.ConfirmCode == "" || strings.TrimSpace(code) == "" ||
		subtle.ConstantTimeCompare([]byte(ch.ConfirmCode), []byte(channelConfirmCodeHash(code))) != 1 {
		return ErrInvalidConfirmationCode
	}
	now := time.Now()
	ch.Confirmed = true
	ch.ConfirmedAt = &now
	ch.ConfirmCode = ""
	return s.channelRepo.Update(ctx, ch)
}

// ProcessPendingDeliveries sends the "immediate" deliveries that are due
// (scheduledFor <= now, status pending/queued) via the appropriate
// channel and updates their status. Daily/weekly rows are left for
// ProcessDigests so they are batched, not sent one by one. Run every
// minute by the background scheduler.
func (s *NotificationDeliveryService) ProcessPendingDeliveries(ctx context.Context) (int, error) {
	now := time.Now()
	deliveries, err := s.deliveryRepo.ListPendingByDigestType(ctx, "immediate", now)
	if err != nil {
		return 0, fmt.Errorf("failed to list pending deliveries: %w", err)
	}

	processed := 0
	for i := range deliveries {
		d := &deliveries[i]

		// Mark as queued during processing
		if err := s.deliveryRepo.UpdateStatus(ctx, d.ID, "queued"); err != nil {
			continue
		}

		sendErr := s.dispatch(ctx, d.UserID, d.ChannelType, d.Address, d.Subject, d.Body)

		if sendErr != nil {
			if err := s.deliveryRepo.IncrementRetry(ctx, d.ID, sendErr.Error()); err != nil {
				continue
			}
		} else {
			sentAt := time.Now()
			d.DeliveryStatus = "sent"
			d.SentAt = &sentAt
			if err := s.deliveryRepo.Update(ctx, d); err != nil {
				continue
			}
			processed++
		}
	}

	return processed, nil
}

// dispatch sends one message over the delivery's channel.
//
// Email uses per-tenant SMTP (Wave 8): the recipient's account_id is
// stamped on the lookup context so SMTP creds resolve at the account
// scope (walking the parent chain to the district). Webhooks go through
// the SSRF guard. Any other type needs a registered ChannelSender.
func (s *NotificationDeliveryService) dispatch(ctx context.Context, userID uint, channelType, address, subject, body string) error {
	switch channelType {
	case "email":
		return s.SendEmail(s.scopeCtxFor(ctx, userID), address, subject, body)
	case "webhook":
		return s.sendWebhook(address, subject, body)
	}
	if sender := s.senders[channelType]; sender != nil {
		return sender.Send(ctx, address, subject, notificationPreview(body))
	}
	return fmt.Errorf("unsupported channel type: %s", channelType)
}

// SendEmail sends an email via SMTP with the body wrapped in a simple
// HTML template. Resolves live SMTP config via the settings service on
// every call — a super-admin who rotates the password through the
// /superadmin/settings UI takes effect immediately on the next send,
// no restart.
//
// Wave 5: SendEmail now takes ctx so the resolution call can be
// cancellable + propagate request scope. Callers internal to this
// package already had ctx in scope; the public-API change is
// limited to this file and main.go's wiring.
func (s *NotificationDeliveryService) SendEmail(ctx context.Context, to, subject, body string) error {
	cfg, err := s.resolveSMTPConfig(ctx)
	if err != nil {
		return err
	}
	if !cfg.Enabled {
		return fmt.Errorf("SMTP is not enabled")
	}

	// Subjects now carry user-controlled text (conversation subjects,
	// announcement and topic titles). A CR/LF in a header value would let
	// the sender inject headers or a whole forged body, so header values
	// are flattened to one line before the message is assembled.
	to = sanitizeEmailHeaderValue(to)
	subject = sanitizeEmailHeaderValue(subject)

	htmlBody, err := s.wrapHTMLTemplate(subject, body)
	if err != nil {
		return fmt.Errorf("failed to render email template: %w", err)
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=\"utf-8\"\r\n\r\n%s",
		cfg.From, to, subject, htmlBody)

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}

	if err := smtp.SendMail(addr, auth, cfg.From, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("SMTP send failed: %w", err)
	}

	return nil
}

// buildDigestBody renders the HTML body of a digest email. Each
// delivery's Subject is plain text (it can hold a user-written
// conversation subject or title), so it is escaped; Body is already
// sanitized/escaped HTML when it is queued.
func buildDigestBody(digestType string, group []models.NotificationDelivery) string {
	var bodyParts []string
	bodyParts = append(bodyParts, fmt.Sprintf("<h2>Your %s Notification Digest</h2>", html.EscapeString(digestType)))
	bodyParts = append(bodyParts, fmt.Sprintf("<p>You have %d notifications:</p><ul>", len(group)))
	for _, d := range group {
		bodyParts = append(bodyParts, fmt.Sprintf("<li><strong>%s</strong><br/>%s</li>", html.EscapeString(d.Subject), d.Body))
	}
	bodyParts = append(bodyParts, "</ul>")
	return strings.Join(bodyParts, "\n")
}

// sanitizeEmailHeaderValue collapses CR, LF and other control characters
// to spaces so a value cannot terminate its header line (header / body
// injection).
func sanitizeEmailHeaderValue(v string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || (r < 0x20 && r != '\t') || r == 0x7f {
			return ' '
		}
		return r
	}, v))
}

// ProcessDigests aggregates pending deliveries for the same user+channel with the specified
// digest type, creates a single summary email, and marks the originals as sent.
func (s *NotificationDeliveryService) ProcessDigests(ctx context.Context, digestType string) (int, error) {
	now := time.Now()
	deliveries, err := s.deliveryRepo.ListPendingByDigestType(ctx, digestType, now)
	if err != nil {
		return 0, fmt.Errorf("failed to list digest deliveries: %w", err)
	}

	if len(deliveries) == 0 {
		return 0, nil
	}

	// Group deliveries by user_id + channel_type + address
	type groupKey struct {
		UserID      uint
		ChannelType string
		Address     string
	}
	groups := make(map[groupKey][]models.NotificationDelivery)
	for _, d := range deliveries {
		key := groupKey{UserID: d.UserID, ChannelType: d.ChannelType, Address: d.Address}
		groups[key] = append(groups[key], d)
	}

	processed := 0
	for key, group := range groups {
		if len(group) == 1 {
			// Single notification -- send as-is, no digest needed
			d := &group[0]
			if err := s.deliveryRepo.UpdateStatus(ctx, d.ID, "queued"); err != nil {
				continue
			}

			sendErr := s.dispatch(ctx, key.UserID, key.ChannelType, key.Address, d.Subject, d.Body)
			if sendErr != nil {
				_ = s.deliveryRepo.IncrementRetry(ctx, d.ID, sendErr.Error())
			} else {
				sentAt := time.Now()
				d.DeliveryStatus = "sent"
				d.SentAt = &sentAt
				_ = s.deliveryRepo.Update(ctx, d)
				processed++
			}
			continue
		}

		// Multiple notifications -- create a digest summary
		digestLabel := strings.ToUpper(digestType[:1]) + digestType[1:]
		subject := fmt.Sprintf("Paper LMS %s Digest — %d notifications", digestLabel, len(group))
		digestBody := buildDigestBody(digestType, group)

		sendErr := s.dispatch(ctx, key.UserID, key.ChannelType, key.Address, subject, digestBody)

		sentAt := time.Now()
		for i := range group {
			d := &group[i]
			if sendErr != nil {
				_ = s.deliveryRepo.IncrementRetry(ctx, d.ID, sendErr.Error())
			} else {
				d.DeliveryStatus = "sent"
				d.SentAt = &sentAt
				_ = s.deliveryRepo.Update(ctx, d)
				processed++
			}
		}
	}

	return processed, nil
}

// RetryFailedDeliveries finds deliveries where status = failed AND retry_count < max_retries,
// and resets them to pending for reprocessing.
func (s *NotificationDeliveryService) RetryFailedDeliveries(ctx context.Context) (int, error) {
	failed, err := s.deliveryRepo.ListFailed(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to list failed deliveries: %w", err)
	}

	retried := 0
	now := time.Now()
	for i := range failed {
		d := &failed[i]
		d.DeliveryStatus = "pending"
		d.ScheduledFor = &now
		if err := s.deliveryRepo.Update(ctx, d); err != nil {
			continue
		}
		retried++
	}

	return retried, nil
}

// GetDeliveryLog returns a paginated delivery history for a user.
func (s *NotificationDeliveryService) GetDeliveryLog(ctx context.Context, userID uint, page, perPage int) (*repository.PaginatedResult[models.NotificationDelivery], error) {
	params := repository.PaginationParams{Page: page, PerPage: perPage}
	return s.deliveryRepo.ListByUserID(ctx, userID, params)
}

// GetDeliveryLogByStatus returns a paginated and filtered delivery history for a user.
func (s *NotificationDeliveryService) GetDeliveryLogByStatus(ctx context.Context, userID uint, status string, page, perPage int) (*repository.PaginatedResult[models.NotificationDelivery], error) {
	params := repository.PaginationParams{Page: page, PerPage: perPage}
	return s.deliveryRepo.ListByUserIDAndStatus(ctx, userID, status, params)
}

// GetDeliveryStats returns admin-level delivery statistics (counts by status).
func (s *NotificationDeliveryService) GetDeliveryStats(ctx context.Context) (map[string]int64, error) {
	return s.deliveryRepo.CountByStatus(ctx)
}

// isNotificationTypeEnabled checks if the user has enabled the specific notification type.
func (s *NotificationDeliveryService) isNotificationTypeEnabled(prefs *models.NotificationPreference, notificationType string) bool {
	switch notificationType {
	case "new_message":
		return prefs.NotifyNewMessage
	case "event_start":
		return prefs.NotifyEventStart
	case "submission_grade":
		return prefs.NotifySubmissionGrade
	case "new_announcement":
		return prefs.NotifyNewAnnouncement
	default:
		// Unknown type -- deliver by default
		return true
	}
}

// policyToDigestType maps a preference policy to a digest type.
func (s *NotificationDeliveryService) policyToDigestType(policy string) string {
	switch policy {
	case "immediately":
		return "immediate"
	case "daily":
		return "daily"
	case "weekly":
		return "weekly"
	default:
		return "daily"
	}
}

// calculateScheduledFor determines when a delivery should be sent based on digest type.
func (s *NotificationDeliveryService) calculateScheduledFor(digestType string) time.Time {
	now := time.Now()
	switch digestType {
	case "immediate":
		return now
	case "hourly":
		return now.Truncate(time.Hour).Add(time.Hour)
	case "daily", "weekly":
		// Digest rows are due as soon as they are queued: the digest job
		// itself only runs at the digest hour (daily) or on the digest day
		// (weekly) and batches everything pending up to that run. A future
		// timestamp here used to land just after the job's hour, so a
		// daily digest went out a day late and a weekly one a week late.
		return now
	default:
		return now
	}
}

// sendWebhook sends a notification payload via HTTP POST to the webhook URL.
//
// SECURITY (F-021): user-controlled communication-channel addresses
// were a viable SSRF primitive — an attacker registered a webhook
// channel pointing at internal admin panels (or the cloud-metadata
// service) and every notification fired POSTs from the server's
// outbound IP, exfiltrating the notification body and probing the
// internal network. ValidateExternalURL refuses RFC1918 / link-local
// / loopback / port mismatches before the request is built.
func (s *NotificationDeliveryService) sendWebhook(url, subject, body string) error {
	if err := security.ValidateExternalURL(context.Background(), url); err != nil {
		return fmt.Errorf("webhook URL rejected by SSRF guard: %w", err)
	}
	payload := fmt.Sprintf(`{"subject":%q,"body":%q,"timestamp":%q}`, subject, body, time.Now().Format(time.RFC3339))
	req, err := http.NewRequest("POST", url, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "PaperLMS-Notification/1.0")

	// SafeTransport re-validates the resolved IP at connect time
	// (DNS-rebinding defense) on top of the up-front ValidateExternalURL.
	client := &http.Client{Timeout: 10 * time.Second, Transport: security.SafeTransport()}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// emailHTMLTemplate is a simple HTML wrapper for notification emails.
const emailHTMLTemplate = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{.Subject}}</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 0; padding: 0; background-color: #f5f5f5; }
    .container { max-width: 600px; margin: 0 auto; padding: 20px; }
    .card { background: #ffffff; border-radius: 8px; padding: 24px; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
    .header { border-bottom: 2px solid #3b82f6; padding-bottom: 16px; margin-bottom: 16px; }
    .header h1 { margin: 0; font-size: 20px; color: #1e293b; }
    .body { color: #334155; line-height: 1.6; }
    .footer { margin-top: 24px; padding-top: 16px; border-top: 1px solid #e2e8f0; color: #94a3b8; font-size: 12px; text-align: center; }
  </style>
</head>
<body>
  <div class="container">
    <div class="card">
      <div class="header">
        <h1>{{.Subject}}</h1>
      </div>
      <div class="body">
        {{.Body}}
      </div>
      <div class="footer">
        <p>This notification was sent by Paper LMS. You can manage your notification preferences in your account settings.</p>
      </div>
    </div>
  </div>
</body>
</html>`

// wrapHTMLTemplate renders the email body inside a styled HTML template.
func (s *NotificationDeliveryService) wrapHTMLTemplate(subject, body string) (string, error) {
	tmpl, err := template.New("email").Parse(emailHTMLTemplate)
	if err != nil {
		return "", err
	}
	data := struct {
		Subject string
		Body    template.HTML
	}{
		Subject: subject,
		Body:    template.HTML(body),
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
