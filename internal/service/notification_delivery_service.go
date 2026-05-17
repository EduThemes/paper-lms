package service

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
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

// QueueNotification looks up the user's notification preference for the given type,
// determines frequency and channel, and creates a NotificationDelivery record with the
// appropriate scheduledFor time.
func (s *NotificationDeliveryService) QueueNotification(
	ctx context.Context,
	userID uint,
	notificationType string,
	subject string,
	body string,
	contextType string,
	contextID uint,
) error {
	// Look up user preferences for digest frequency
	prefs, err := s.prefRepo.FindByUserID(ctx, userID)
	if err != nil {
		// Default to daily if no preference is set
		prefs = &models.NotificationPreference{
			UserID: userID,
			Policy: "daily",
		}
	}

	// If user has opted out of notifications entirely, skip
	if prefs.Policy == "never" {
		return nil
	}

	// Check if the specific notification type is enabled
	if !s.isNotificationTypeEnabled(prefs, notificationType) {
		return nil
	}

	// Map the policy to a digest type
	digestType := s.policyToDigestType(prefs.Policy)

	// Calculate when this delivery should be sent
	scheduledFor := s.calculateScheduledFor(digestType)

	// Look up the user's communication channels
	channels, err := s.channelRepo.ListByUserID(ctx, userID)
	if err != nil || len(channels) == 0 {
		// Fall back to user email if no explicit communication channels exist
		user, userErr := s.userRepo.FindByID(ctx, userID)
		if userErr != nil {
			return fmt.Errorf("failed to find user %d: %w", userID, userErr)
		}

		delivery := &models.NotificationDelivery{
			NotificationID: 0, // Will be linked after notification creation
			UserID:         userID,
			ChannelType:    "email",
			Address:        user.Email,
			Subject:        subject,
			Body:           body,
			DeliveryStatus: "pending",
			DigestType:     digestType,
			MaxRetries:     3,
			ScheduledFor:   &scheduledFor,
		}
		return s.deliveryRepo.Create(ctx, delivery)
	}

	// Create a delivery record for each active communication channel
	for _, ch := range channels {
		delivery := &models.NotificationDelivery{
			NotificationID: 0,
			UserID:         userID,
			ChannelType:    ch.ChannelType,
			Address:        ch.Address,
			Subject:        subject,
			Body:           body,
			DeliveryStatus: "pending",
			DigestType:     digestType,
			MaxRetries:     3,
			ScheduledFor:   &scheduledFor,
		}
		if err := s.deliveryRepo.Create(ctx, delivery); err != nil {
			return fmt.Errorf("failed to create delivery for channel %d: %w", ch.ID, err)
		}
	}

	return nil
}

// ProcessPendingDeliveries fetches all deliveries where scheduledFor <= now AND status = pending,
// and sends them via the appropriate channel. Updates status accordingly.
func (s *NotificationDeliveryService) ProcessPendingDeliveries(ctx context.Context) (int, error) {
	now := time.Now()
	deliveries, err := s.deliveryRepo.ListPending(ctx, now)
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

		var sendErr error
		switch d.ChannelType {
		case "email":
			sendErr = s.SendEmail(ctx, d.Address, d.Subject, d.Body)
		case "webhook":
			sendErr = s.sendWebhook(d.Address, d.Subject, d.Body)
		default:
			sendErr = fmt.Errorf("unsupported channel type: %s", d.ChannelType)
		}

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

			var sendErr error
			switch key.ChannelType {
			case "email":
				sendErr = s.SendEmail(ctx, key.Address, d.Subject, d.Body)
			case "webhook":
				sendErr = s.sendWebhook(key.Address, d.Subject, d.Body)
			}
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
		var bodyParts []string
		bodyParts = append(bodyParts, fmt.Sprintf("<h2>Your %s Notification Digest</h2>", digestType))
		bodyParts = append(bodyParts, fmt.Sprintf("<p>You have %d notifications:</p><ul>", len(group)))
		for _, d := range group {
			bodyParts = append(bodyParts, fmt.Sprintf("<li><strong>%s</strong><br/>%s</li>", d.Subject, d.Body))
		}
		bodyParts = append(bodyParts, "</ul>")
		digestBody := strings.Join(bodyParts, "\n")

		var sendErr error
		switch key.ChannelType {
		case "email":
			sendErr = s.SendEmail(ctx, key.Address, subject, digestBody)
		case "webhook":
			sendErr = s.sendWebhook(key.Address, subject, digestBody)
		}

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
	case "daily":
		// Schedule for the next day at 8:00 AM UTC
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 8, 0, 0, 0, time.UTC)
		return next
	case "weekly":
		// Schedule for next Monday at 8:00 AM UTC
		daysUntilMonday := (8 - int(now.Weekday())) % 7
		if daysUntilMonday == 0 {
			daysUntilMonday = 7
		}
		next := time.Date(now.Year(), now.Month(), now.Day()+daysUntilMonday, 8, 0, 0, 0, time.UTC)
		return next
	default:
		return now
	}
}

// sendWebhook sends a notification payload via HTTP POST to the webhook URL.
func (s *NotificationDeliveryService) sendWebhook(url, subject, body string) error {
	payload := fmt.Sprintf(`{"subject":%q,"body":%q,"timestamp":%q}`, subject, body, time.Now().Format(time.RFC3339))
	req, err := http.NewRequest("POST", url, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "PaperLMS-Notification/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

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
