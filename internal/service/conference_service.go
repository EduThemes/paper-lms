package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	// lookup resolves the conferences.* settings per tenant. Nil means no
	// provider is configured.
	lookup SettingsLookupFunc
	// httpClient overrides the provider API client (tests). Nil uses an
	// SSRF-safe client.
	httpClient *http.Client
	// validateURL overrides the SSRF guard (tests point providers at a
	// loopback httptest server). Nil uses security.ValidateExternalURL.
	validateURL func(ctx context.Context, rawURL string) error
	// Zoom endpoints; overridable in tests.
	zoomAPIBase   string
	zoomOAuthBase string
	zoomTokens    *zoomTokenCache
}

// SetSettingsLookup wires the settings resolution chain used to find the
// conferencing providers for a tenant.
func (s *ConferenceService) SetSettingsLookup(lookup SettingsLookupFunc) {
	s.lookup = lookup
}

func NewConferenceService(confRepo repository.ConferenceRepository, participantRepo repository.ConferenceParticipantRepository) *ConferenceService {
	return &ConferenceService{
		confRepo:        confRepo,
		participantRepo: participantRepo,
		zoomAPIBase:     zoomDefaultAPIBase,
		zoomOAuthBase:   zoomDefaultOAuthBase,
		zoomTokens:      newZoomTokenCache(),
	}
}

// Conference types.
const (
	ConferenceTypeBigBlueButton = "BigBlueButton"
	ConferenceTypeZoom          = "Zoom"
)

// CRUD

func (s *ConferenceService) Create(ctx context.Context, conference *models.Conference) error {
	if conference.Title == "" {
		return errors.New("conference title is required")
	}
	if conference.ConferenceType == "" {
		return errors.New("conference_type is required")
	}
	if conference.ConferenceType != ConferenceTypeBigBlueButton && conference.ConferenceType != ConferenceTypeZoom {
		return errors.New("conference_type must be 'BigBlueButton' or 'Zoom'")
	}
	if conference.ContextType == "" || conference.ContextID == 0 {
		return errors.New("context_type and context_id are required")
	}
	if conference.Duration < 0 {
		return errors.New("duration must not be negative")
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

// ListByContext lists a context's conferences, tenant-scoped by accountID
// (0 = internal caller, no scope).
func (s *ConferenceService) ListByContext(ctx context.Context, contextType string, contextID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Conference], error) {
	return s.confRepo.ListByContext(ctx, contextType, contextID, accountID, params)
}

// Conference lifecycle
//
// Paper holds a conference one of three ways:
//
//   - BigBlueButton: when conferences.bbb.url + conferences.bbb.secret are
//     configured (Settings → Conferencing, or BBB_URL / BBB_SECRET), Start
//     creates the meeting through the BBB API and Join returns a signed
//     BBB join URL (moderator for instructors, viewer for everyone else).
//   - Zoom: when the conferences.zoom.* Server-to-Server OAuth app is
//     configured, Start creates a Zoom meeting; instructors join through a
//     fresh host start link, everyone else through the attendee link.
//   - External meeting link: any conference may carry a teacher-supplied
//     meeting URL (e.g. a personal Zoom room) in join_url; Join returns it.
//
// With none of these, Start refuses with ErrConferenceNotConfigured instead
// of handing out a URL that goes nowhere.

// ErrConferenceNotConfigured is returned when a conference has no way to
// be held: no provider and no external meeting link.
var ErrConferenceNotConfigured = errors.New("no conferencing provider is configured: ask an administrator to configure BigBlueButton or Zoom, or edit the conference and add a meeting link")

// ErrConferenceNotStarted is returned when a non-moderator tries to join a
// conference the instructor has not started yet.
var ErrConferenceNotStarted = errors.New("conference has not been started")

// ErrConferenceNotRunning is returned when a non-moderator tries to join a
// started conference whose meeting is no longer open on the provider
// (everyone left and the provider closed it). The instructor restarts it
// by joining.
var ErrConferenceNotRunning = errors.New("the conference is not running right now: wait for the instructor to rejoin it")

// ErrConferenceEnded is returned when joining or starting an ended
// conference.
var ErrConferenceEnded = errors.New("conference has already ended")

// ErrRecordingNotFound is returned when a recording id does not belong to
// the conference.
var ErrRecordingNotFound = errors.New("recording not found")

// ErrRecordingsUnsupported is returned when a conference is not held on a
// provider that keeps recordings (an external meeting link).
var ErrRecordingsUnsupported = errors.New("recordings are only available for conferences held on BigBlueButton or Zoom")

// ConferenceProviderInfo describes one conference type for the UI.
type ConferenceProviderInfo struct {
	Type              string `json:"type"`
	Name              string `json:"name"`
	Configured        bool   `json:"configured"`
	SupportsRecording bool   `json:"supports_recording"`
}

// Providers reports which conference types the tenant can hold natively.
// An unconfigured type still works with an external meeting link.
func (s *ConferenceService) Providers(ctx context.Context, accountID uint) []ConferenceProviderInfo {
	_, bbb := s.resolveBBB(ctx, accountID)
	_, zoom := s.resolveZoom(ctx, accountID)
	return []ConferenceProviderInfo{
		{Type: ConferenceTypeBigBlueButton, Name: "BigBlueButton", Configured: bbb, SupportsRecording: bbb},
		{Type: ConferenceTypeZoom, Name: "Zoom", Configured: zoom, SupportsRecording: zoom},
	}
}

// ConferenceRecording is a provider recording in a provider-neutral shape.
type ConferenceRecording struct {
	ID              string                      `json:"id"`
	Title           string                      `json:"title"`
	StartedAt       *time.Time                  `json:"started_at"`
	EndedAt         *time.Time                  `json:"ended_at"`
	DurationMinutes int                         `json:"duration_minutes"`
	PlaybackURL     string                      `json:"playback_url"`
	Published       bool                        `json:"published"`
	Formats         []ConferenceRecordingFormat `json:"formats"`
}

// ConferenceRecordingFormat is one playable rendition of a recording.
type ConferenceRecordingFormat struct {
	Type          string `json:"type"`
	URL           string `json:"url"`
	LengthMinutes int    `json:"length_minutes"`
}

// resolveSetting reads a trimmed setting for the tenant ("" on error).
func (s *ConferenceService) resolveSetting(ctx context.Context, accountID uint, key string) string {
	if s.lookup == nil {
		return ""
	}
	v, err := s.lookup(settingsctx.WithAccountID(ctx, accountID), key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(v)
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

// checkOutboundURL runs the SSRF guard on a provider URL before any
// request is built.
func (s *ConferenceService) checkOutboundURL(ctx context.Context, rawURL string) error {
	if s.validateURL != nil {
		return s.validateURL(ctx, rawURL)
	}
	return security.ValidateExternalURL(ctx, rawURL)
}

func (s *ConferenceService) client() *http.Client {
	if s.httpClient != nil {
		return s.httpClient
	}
	return &http.Client{Timeout: 10 * time.Second, Transport: security.SafeTransport()}
}

// logoutURL is where the provider sends people when they leave: the
// course's conference page, when the frontend URL is configured.
func (s *ConferenceService) logoutURL(ctx context.Context, conference *models.Conference, accountID uint) string {
	base := strings.TrimRight(s.resolveSetting(ctx, accountID, "branding.frontend_url"), "/")
	if base == "" || conference.ContextType != "Course" || !isExternalJoinURL(base) {
		return ""
	}
	return fmt.Sprintf("%s/courses/%d/conferences", base, conference.ContextID)
}

// StartConference starts a conference so participants can join. On a
// configured provider it creates the meeting; a conference with an
// external meeting link needs nothing created. accountID scopes the
// settings lookup to the caller's tenant.
func (s *ConferenceService) StartConference(ctx context.Context, conference *models.Conference, accountID uint) (*models.Conference, error) {
	if conference.StartedAt != nil && conference.EndedAt == nil {
		return conference, nil // already running: starting is idempotent
	}

	if cfg, ok := s.usesBBB(ctx, conference, accountID); ok {
		if err := s.bbbCreateMeeting(ctx, cfg, conference, accountID); err != nil {
			return nil, err
		}
	} else if cfg, ok := s.usesZoom(ctx, conference, accountID); ok {
		if err := s.zoomEnsureMeeting(ctx, cfg, conference); err != nil {
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

// EndConference ends a running conference (and the provider meeting, best
// effort).
func (s *ConferenceService) EndConference(ctx context.Context, conference *models.Conference, accountID uint) (*models.Conference, error) {
	if conference.StartedAt == nil {
		return nil, ErrConferenceNotStarted
	}

	if conference.EndedAt != nil {
		return nil, ErrConferenceEnded
	}

	if cfg, ok := s.usesBBB(ctx, conference, accountID); ok {
		s.bbbEndMeeting(ctx, cfg, conference)
	} else if cfg, ok := s.usesZoom(ctx, conference, accountID); ok && conference.ProviderMeetingID != "" {
		s.zoomEndMeeting(ctx, cfg, conference.ProviderMeetingID)
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
// instructors) join BBB meetings as MODERATOR and Zoom meetings as host;
// everyone else as an attendee.
func (s *ConferenceService) JoinConference(ctx context.Context, conference *models.Conference, userID uint, userName string, moderator bool, accountID uint) (string, error) {
	if conference.EndedAt != nil {
		return "", ErrConferenceEnded
	}
	if conference.StartedAt == nil {
		return "", ErrConferenceNotStarted
	}

	var joinURL string
	if cfg, ok := s.usesBBB(ctx, conference, accountID); ok {
		u, err := s.bbbJoinURL(ctx, cfg, conference, userID, userName, moderator, accountID)
		if err != nil {
			return "", err
		}
		joinURL = u
	} else if cfg, ok := s.usesZoom(ctx, conference, accountID); ok {
		u, err := s.zoomJoinURL(ctx, cfg, conference, moderator)
		if err != nil {
			return "", err
		}
		joinURL = u
	} else if isExternalJoinURL(conference.JoinURL) {
		joinURL = conference.JoinURL
	} else {
		return "", ErrConferenceNotConfigured
	}

	participationType := "invitee"
	if moderator {
		participationType = "initiator"
	}
	if s.participantRepo != nil {
		if existing, _ := s.participantRepo.FindByConferenceAndUser(ctx, conference.ID, userID, 0); existing == nil {
			_ = s.participantRepo.Create(ctx, &models.ConferenceParticipant{
				ConferenceID:      conference.ID,
				UserID:            userID,
				ParticipationType: participationType,
			})
		}
	}

	return joinURL, nil
}

// GetRecordings returns the conference's recordings from its provider.
// Conferences on an external link have none (an empty list); recordings
// stored on the row by an earlier import are returned as-is.
func (s *ConferenceService) GetRecordings(ctx context.Context, conference *models.Conference, accountID uint) ([]ConferenceRecording, error) {
	if cfg, ok := s.usesBBB(ctx, conference, accountID); ok {
		return s.bbbRecordings(ctx, cfg, conference)
	}
	if cfg, ok := s.usesZoom(ctx, conference, accountID); ok {
		if conference.ProviderMeetingID == "" {
			return []ConferenceRecording{}, nil
		}
		return s.zoomRecordings(ctx, cfg, conference.ProviderMeetingID)
	}
	return storedRecordings(conference.Recordings), nil
}

// DeleteRecording deletes one of the conference's recordings on the
// provider. The id must belong to this conference's meeting: provider
// recording ids are global, so an unchecked id could delete another
// course's (or another tenant's, on a shared server) recording.
func (s *ConferenceService) DeleteRecording(ctx context.Context, conference *models.Conference, recordingID string, accountID uint) error {
	recordingID = strings.TrimSpace(recordingID)
	if recordingID == "" {
		return ErrRecordingNotFound
	}
	var (
		recs []ConferenceRecording
		err  error
	)
	bbbCfg, isBBB := s.usesBBB(ctx, conference, accountID)
	zoomCfg, isZoom := s.usesZoom(ctx, conference, accountID)
	switch {
	case isBBB:
		recs, err = s.bbbRecordings(ctx, bbbCfg, conference)
	case isZoom && conference.ProviderMeetingID != "":
		recs, err = s.zoomRecordings(ctx, zoomCfg, conference.ProviderMeetingID)
	case isZoom:
		return ErrRecordingNotFound
	default:
		return ErrRecordingsUnsupported
	}
	if err != nil {
		return err
	}
	found := false
	for _, r := range recs {
		if r.ID == recordingID {
			found = true
			break
		}
	}
	if !found {
		return ErrRecordingNotFound
	}
	if isBBB {
		return s.bbbDeleteRecording(ctx, bbbCfg, recordingID)
	}
	return s.zoomDeleteRecordings(ctx, zoomCfg, recordingID)
}

// storedRecordings parses the legacy recordings JSON column. Anything
// unreadable is treated as "no recordings".
func storedRecordings(raw string) []ConferenceRecording {
	out := []ConferenceRecording{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return out
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return []ConferenceRecording{}
	}
	return out
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
