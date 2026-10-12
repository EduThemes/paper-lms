package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha1" // #nosec G505 -- the BigBlueButton API checksum is specified as SHA-1
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/security"
	"github.com/EduThemes/paper-lms/internal/settingsctx"
)

type ConferenceService struct {
	confRepo        repository.ConferenceRepository
	participantRepo repository.ConferenceParticipantRepository
	// lookup resolves the conferences.bbb.* settings per tenant. Nil
	// means BigBlueButton is not configured.
	lookup SettingsLookupFunc
	// httpClient overrides the BBB API client (tests). Nil uses an
	// SSRF-safe client.
	httpClient *http.Client
}

// SetSettingsLookup wires the settings resolution chain used to find the
// BigBlueButton server for a tenant.
func (s *ConferenceService) SetSettingsLookup(lookup SettingsLookupFunc) {
	s.lookup = lookup
}

func NewConferenceService(confRepo repository.ConferenceRepository, participantRepo repository.ConferenceParticipantRepository) *ConferenceService {
	return &ConferenceService{
		confRepo:        confRepo,
		participantRepo: participantRepo,
	}
}

// CRUD

func (s *ConferenceService) Create(ctx context.Context, conference *models.Conference) error {
	if conference.Title == "" {
		return errors.New("conference title is required")
	}
	if conference.ConferenceType == "" {
		return errors.New("conference_type is required")
	}
	if conference.ConferenceType != "BigBlueButton" && conference.ConferenceType != "Zoom" {
		return errors.New("conference_type must be 'BigBlueButton' or 'Zoom'")
	}
	if conference.ContextType == "" || conference.ContextID == 0 {
		return errors.New("context_type and context_id are required")
	}
	if err := ValidateMeetingURL(conference.JoinURL); err != nil {
		return err
	}
	if conference.WorkflowState == "" {
		conference.WorkflowState = "active"
	}
	if conference.Recordings == "" {
		conference.Recordings = "[]"
	}
	if conference.Settings == "" {
		conference.Settings = "{}"
	}

	return s.confRepo.Create(ctx, conference)
}

// GetByID is the public read path. Sprint 2.3 threaded accountID
// through from the handler; pass 0 from internal callers.
func (s *ConferenceService) GetByID(ctx context.Context, id, accountID uint) (*models.Conference, error) {
	conference, err := s.confRepo.FindByID(ctx, id, accountID)
	if err != nil {
		return nil, errors.New("conference not found")
	}
	return conference, nil
}

func (s *ConferenceService) Update(ctx context.Context, conference *models.Conference) error {
	_, err := s.confRepo.FindByID(ctx, conference.ID, 0)
	if err != nil {
		return errors.New("conference not found")
	}

	return s.confRepo.Update(ctx, conference)
}

func (s *ConferenceService) Delete(ctx context.Context, id uint) error {
	return s.confRepo.Delete(ctx, id)
}

func (s *ConferenceService) ListByContext(ctx context.Context, contextType string, contextID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Conference], error) {
	return s.confRepo.ListByContext(ctx, contextType, contextID, 0, params)
}

// Conference lifecycle
//
// Paper supports two ways to actually hold a conference:
//
//   - BigBlueButton: when conferences.bbb.url + conferences.bbb.secret are
//     configured (Settings → Conferencing, or BBB_URL / BBB_SECRET), Start
//     creates the meeting through the BBB API and Join returns a signed
//     BBB join URL (moderator for instructors, viewer for everyone else).
//   - External meeting link: any conference may carry a teacher-supplied
//     meeting URL (e.g. a Zoom link) in join_url; Join returns it.
//
// With neither, Start refuses with ErrConferenceNotConfigured instead of
// handing out a URL that goes nowhere.

// ErrConferenceNotConfigured is returned when a conference has no way to
// be held: no BigBlueButton server and no external meeting link.
var ErrConferenceNotConfigured = errors.New("no conferencing provider is configured: ask an administrator to configure BigBlueButton, or edit the conference and add a meeting link")

// ErrConferenceNotStarted is returned when a non-moderator tries to join a
// conference the instructor has not started yet.
var ErrConferenceNotStarted = errors.New("conference has not been started")

// ErrConferenceEnded is returned when joining or starting an ended
// conference.
var ErrConferenceEnded = errors.New("conference has already ended")

// bbbConfig is the resolved BigBlueButton server configuration.
type bbbConfig struct {
	BaseURL string // normalized, ends in "/api/"
	Secret  string
}

// resolveBBB returns the BigBlueButton config for the caller's tenant, or
// ok=false when BBB is not configured.
func (s *ConferenceService) resolveBBB(ctx context.Context, accountID uint) (bbbConfig, bool) {
	if s.lookup == nil {
		return bbbConfig{}, false
	}
	ctx = settingsctx.WithAccountID(ctx, accountID)
	rawURL, err := s.lookup(ctx, "conferences.bbb.url")
	if err != nil {
		return bbbConfig{}, false
	}
	secret, err := s.lookup(ctx, "conferences.bbb.secret")
	if err != nil {
		return bbbConfig{}, false
	}
	rawURL, secret = strings.TrimSpace(rawURL), strings.TrimSpace(secret)
	if rawURL == "" || secret == "" {
		return bbbConfig{}, false
	}
	base := strings.TrimRight(rawURL, "/")
	if !strings.HasSuffix(base, "/api") {
		base += "/api"
	}
	return bbbConfig{BaseURL: base + "/", Secret: secret}, true
}

// usesBBB reports whether this conference is held on the configured BBB
// server (a BigBlueButton conference without an external link).
func (s *ConferenceService) usesBBB(ctx context.Context, conference *models.Conference, accountID uint) (bbbConfig, bool) {
	if conference.ConferenceType != "BigBlueButton" || isExternalJoinURL(conference.JoinURL) {
		return bbbConfig{}, false
	}
	return s.resolveBBB(ctx, accountID)
}

// isExternalJoinURL reports whether the stored join URL is a real,
// teacher-supplied meeting link (as opposed to empty).
func isExternalJoinURL(u string) bool {
	return strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://")
}

// ValidateMeetingURL checks a teacher-supplied external meeting link. It
// must be an absolute http(s) URL; it is only ever handed to browsers,
// never fetched server-side.
func ValidateMeetingURL(raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return errors.New("join_url must be an absolute http(s) URL")
	}
	return nil
}

// bbbMeetingID is the BBB meeting identifier for a Paper conference.
func bbbMeetingID(conferenceID uint) string {
	return fmt.Sprintf("paper-conference-%d", conferenceID)
}

// bbbPassword derives a stable per-meeting password from the shared
// secret, so nothing extra has to be stored.
func bbbPassword(cfg bbbConfig, meetingID, role string) string {
	mac := hmac.New(sha256.New, []byte(cfg.Secret))
	mac.Write([]byte(meetingID + ":" + role))
	return hex.EncodeToString(mac.Sum(nil))[:24]
}

// bbbURL builds a signed BBB API URL: checksum = sha1(call + query +
// secret), per the BigBlueButton API spec.
func bbbURL(cfg bbbConfig, call string, params url.Values) string {
	query := params.Encode()
	sum := sha1.Sum([]byte(call + query + cfg.Secret)) // #nosec G401 -- BigBlueButton API checksum is specified as SHA-1
	if query != "" {
		query += "&"
	}
	return cfg.BaseURL + call + "?" + query + "checksum=" + hex.EncodeToString(sum[:])
}

type bbbResponse struct {
	ReturnCode string `xml:"returncode"`
	MessageKey string `xml:"messageKey"`
	Message    string `xml:"message"`
}

// bbbCall performs a server-side BBB API call. The server URL is
// operator-configured but still passes the SSRF guard before the request
// is built.
func (s *ConferenceService) bbbCall(ctx context.Context, apiURL string) error {
	if err := security.ValidateExternalURL(ctx, apiURL); err != nil {
		return fmt.Errorf("BigBlueButton URL rejected: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return err
	}
	client := s.httpClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second, Transport: security.SafeTransport()}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("BigBlueButton request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out bbbResponse
	if err := xml.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return fmt.Errorf("BigBlueButton returned an unreadable response (HTTP %d)", resp.StatusCode)
	}
	if out.ReturnCode != "SUCCESS" {
		return fmt.Errorf("BigBlueButton refused the request: %s %s", out.MessageKey, out.Message)
	}
	return nil
}

// StartConference starts a conference so participants can join. On a
// configured BBB server it creates the meeting; a conference with an
// external meeting link needs nothing created. accountID scopes the
// settings lookup to the caller's tenant.
func (s *ConferenceService) StartConference(ctx context.Context, conference *models.Conference, accountID uint) (*models.Conference, error) {
	if conference.StartedAt != nil && conference.EndedAt == nil {
		return conference, nil // already running: starting is idempotent
	}

	if cfg, ok := s.usesBBB(ctx, conference, accountID); ok {
		meetingID := bbbMeetingID(conference.ID)
		params := url.Values{}
		params.Set("name", conference.Title)
		params.Set("meetingID", meetingID)
		params.Set("attendeePW", bbbPassword(cfg, meetingID, "attendee"))
		params.Set("moderatorPW", bbbPassword(cfg, meetingID, "moderator"))
		if conference.Duration > 0 {
			params.Set("duration", fmt.Sprintf("%d", conference.Duration))
		}
		if err := s.bbbCall(ctx, bbbURL(cfg, "create", params)); err != nil {
			return nil, err
		}
	} else if !isExternalJoinURL(conference.JoinURL) {
		return nil, ErrConferenceNotConfigured
	}

	now := time.Now()
	conference.StartedAt = &now
	conference.EndedAt = nil
	conference.WorkflowState = "active"

	if err := s.confRepo.Update(ctx, conference); err != nil {
		return nil, err
	}

	return conference, nil
}

// EndConference ends a running conference (and the BBB meeting, best
// effort).
func (s *ConferenceService) EndConference(ctx context.Context, conference *models.Conference, accountID uint) (*models.Conference, error) {
	if conference.StartedAt == nil {
		return nil, ErrConferenceNotStarted
	}

	if conference.EndedAt != nil {
		return nil, ErrConferenceEnded
	}

	if cfg, ok := s.usesBBB(ctx, conference, accountID); ok {
		meetingID := bbbMeetingID(conference.ID)
		params := url.Values{}
		params.Set("meetingID", meetingID)
		params.Set("password", bbbPassword(cfg, meetingID, "moderator"))
		_ = s.bbbCall(ctx, bbbURL(cfg, "end", params))
	}

	now := time.Now()
	conference.EndedAt = &now
	conference.WorkflowState = "ended"

	if err := s.confRepo.Update(ctx, conference); err != nil {
		return nil, err
	}

	return conference, nil
}

// JoinConference returns the URL the user opens to join a running
// conference and records them as a participant. Moderators (course
// instructors) join BBB meetings as MODERATOR, everyone else as VIEWER.
func (s *ConferenceService) JoinConference(ctx context.Context, conference *models.Conference, userID uint, userName string, moderator bool, accountID uint) (string, error) {
	if conference.EndedAt != nil {
		return "", ErrConferenceEnded
	}
	if conference.StartedAt == nil {
		return "", ErrConferenceNotStarted
	}

	var joinURL string
	if cfg, ok := s.usesBBB(ctx, conference, accountID); ok {
		meetingID := bbbMeetingID(conference.ID)
		role, pwRole := "VIEWER", "attendee"
		if moderator {
			role, pwRole = "MODERATOR", "moderator"
		}
		if strings.TrimSpace(userName) == "" {
			userName = fmt.Sprintf("User %d", userID)
		}
		params := url.Values{}
		params.Set("fullName", userName)
		params.Set("meetingID", meetingID)
		params.Set("password", bbbPassword(cfg, meetingID, pwRole))
		params.Set("role", role)
		params.Set("userID", fmt.Sprintf("%d", userID))
		params.Set("redirect", "true")
		joinURL = bbbURL(cfg, "join", params)
	} else if isExternalJoinURL(conference.JoinURL) {
		joinURL = conference.JoinURL
	} else {
		return "", ErrConferenceNotConfigured
	}

	participationType := "invitee"
	if moderator {
		participationType = "initiator"
	}
	if existing, _ := s.participantRepo.FindByConferenceAndUser(ctx, conference.ID, userID, 0); existing == nil {
		_ = s.participantRepo.Create(ctx, &models.ConferenceParticipant{
			ConferenceID:      conference.ID,
			UserID:            userID,
			ParticipationType: participationType,
		})
	}

	return joinURL, nil
}

func (s *ConferenceService) GetRecordings(ctx context.Context, id uint) (string, error) {
	conference, err := s.confRepo.FindByID(ctx, id, 0)
	if err != nil {
		return "", errors.New("conference not found")
	}

	return conference.Recordings, nil
}

// Participant management

func (s *ConferenceService) AddParticipant(ctx context.Context, conferenceID, userID uint, participationType string) error {
	// Check conference exists
	_, err := s.confRepo.FindByID(ctx, conferenceID, 0)
	if err != nil {
		return errors.New("conference not found")
	}

	if participationType == "" {
		participationType = "invitee"
	}
	if participationType != "initiator" && participationType != "invitee" && participationType != "observer" {
		return errors.New("participation_type must be 'initiator', 'invitee', or 'observer'")
	}

	// Check if participant already exists
	existing, _ := s.participantRepo.FindByConferenceAndUser(ctx, conferenceID, userID, 0)
	if existing != nil {
		return errors.New("user is already a participant in this conference")
	}

	participant := &models.ConferenceParticipant{
		ConferenceID:      conferenceID,
		UserID:            userID,
		ParticipationType: participationType,
	}

	return s.participantRepo.Create(ctx, participant)
}

func (s *ConferenceService) RemoveParticipant(ctx context.Context, conferenceID, userID uint) error {
	participant, err := s.participantRepo.FindByConferenceAndUser(ctx, conferenceID, userID, 0)
	if err != nil {
		return errors.New("participant not found")
	}

	return s.participantRepo.Delete(ctx, participant.ID)
}

func (s *ConferenceService) ListParticipants(ctx context.Context, conferenceID uint) ([]models.ConferenceParticipant, error) {
	return s.participantRepo.ListByConferenceID(ctx, conferenceID, 0)
}
