package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// Zoom client (G098). Paper talks to Zoom as a Server-to-Server OAuth app:
// an operator creates the app in the Zoom marketplace and configures its
// account id, client id and client secret (Settings → Conferencing, or
// ZOOM_ACCOUNT_ID / ZOOM_CLIENT_ID / ZOOM_CLIENT_SECRET), plus the Zoom
// user that hosts the meetings. Paper then creates one Zoom meeting per
// conference, sends instructors through a fresh host start link and
// everyone else through the attendee link, ends the meeting on End, and
// lists / deletes its cloud recordings.

const (
	zoomDefaultAPIBase   = "https://api.zoom.us/v2"
	zoomDefaultOAuthBase = "https://zoom.us"
)

// zoomConfig is the resolved Zoom app configuration.
type zoomConfig struct {
	AccountID    string
	ClientID     string
	ClientSecret string
	HostUser     string
}

func (c zoomConfig) cacheKey() string { return c.AccountID + "\x00" + c.ClientID }

// resolveZoom returns the Zoom config for the caller's tenant, or
// ok=false when Zoom is not configured.
func (s *ConferenceService) resolveZoom(ctx context.Context, accountID uint) (zoomConfig, bool) {
	cfg := zoomConfig{
		AccountID:    s.resolveSetting(ctx, accountID, "conferences.zoom.account_id"),
		ClientID:     s.resolveSetting(ctx, accountID, "conferences.zoom.client_id"),
		ClientSecret: s.resolveSetting(ctx, accountID, "conferences.zoom.client_secret"),
		HostUser:     s.resolveSetting(ctx, accountID, "conferences.zoom.host_user"),
	}
	if cfg.AccountID == "" || cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.HostUser == "" {
		return zoomConfig{}, false
	}
	return cfg, true
}

// usesZoom reports whether this conference is held on the configured Zoom
// app (a Zoom conference without a teacher-supplied link).
func (s *ConferenceService) usesZoom(ctx context.Context, conference *models.Conference, accountID uint) (zoomConfig, bool) {
	if conference.ConferenceType != ConferenceTypeZoom || isExternalJoinURL(conference.JoinURL) {
		return zoomConfig{}, false
	}
	return s.resolveZoom(ctx, accountID)
}

// zoomTokenCache keeps Server-to-Server access tokens until shortly before
// they expire (Zoom issues them for an hour).
type zoomTokenCache struct {
	mu     sync.Mutex
	tokens map[string]zoomCachedToken
}

type zoomCachedToken struct {
	token   string
	expires time.Time
}

func newZoomTokenCache() *zoomTokenCache {
	return &zoomTokenCache{tokens: map[string]zoomCachedToken{}}
}

// zoomAPIError is an error response from Zoom.
type zoomAPIError struct {
	Status  int
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *zoomAPIError) Error() string {
	return fmt.Sprintf("zoom refused the request (HTTP %d, code %d): %s", e.Status, e.Code, e.Message)
}

// zoomNotFound reports a 404 from Zoom (meeting or recording gone).
func zoomNotFound(err error) bool {
	var ze *zoomAPIError
	return errors.As(err, &ze) && ze.Status == http.StatusNotFound
}

// zoomToken returns a valid access token for the app.
func (s *ConferenceService) zoomToken(ctx context.Context, cfg zoomConfig) (string, error) {
	cache := s.zoomTokens
	if cache == nil {
		cache = newZoomTokenCache()
		s.zoomTokens = cache
	}
	cache.mu.Lock()
	if t, ok := cache.tokens[cfg.cacheKey()]; ok && time.Now().Before(t.expires) {
		cache.mu.Unlock()
		return t.token, nil
	}
	cache.mu.Unlock()

	q := url.Values{}
	q.Set("grant_type", "account_credentials")
	q.Set("account_id", cfg.AccountID)
	tokenURL := strings.TrimRight(s.zoomOAuthBase, "/") + "/oauth/token?" + q.Encode()
	if err := s.checkOutboundURL(ctx, tokenURL); err != nil {
		return "", fmt.Errorf("zoom: URL rejected: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(cfg.ClientID, cfg.ClientSecret)
	resp, err := s.client().Do(req)
	if err != nil {
		return "", fmt.Errorf("zoom: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("zoom: rejected the app credentials (HTTP %d)", resp.StatusCode)
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.AccessToken == "" {
		return "", errors.New("zoom: returned an unreadable token response")
	}
	ttl := time.Duration(out.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = time.Hour
	}
	cache.mu.Lock()
	cache.tokens[cfg.cacheKey()] = zoomCachedToken{token: out.AccessToken, expires: time.Now().Add(ttl - time.Minute)}
	cache.mu.Unlock()
	return out.AccessToken, nil
}

// zoomDo performs an authenticated Zoom API call. out may be nil.
func (s *ConferenceService) zoomDo(ctx context.Context, cfg zoomConfig, method, path string, body, out any) error {
	token, err := s.zoomToken(ctx, cfg)
	if err != nil {
		return err
	}
	apiURL := strings.TrimRight(s.zoomAPIBase, "/") + path
	if err := s.checkOutboundURL(ctx, apiURL); err != nil {
		return fmt.Errorf("zoom: URL rejected: %w", err)
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, apiURL, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.client().Do(req)
	if err != nil {
		return fmt.Errorf("zoom: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 300 {
		apiErr := &zoomAPIError{Status: resp.StatusCode}
		_ = json.Unmarshal(raw, apiErr)
		return apiErr
	}
	if out != nil && len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return errors.New("zoom: returned an unreadable response")
		}
	}
	return nil
}

type zoomMeeting struct {
	ID       json.Number `json:"id"`
	JoinURL  string      `json:"join_url"`
	StartURL string      `json:"start_url"`
}

// zoomCreateMeeting creates the conference's Zoom meeting and records its
// id and attendee link on the conference (persisted by the caller).
func (s *ConferenceService) zoomCreateMeeting(ctx context.Context, cfg zoomConfig, conference *models.Conference) error {
	duration := conference.Duration
	if duration <= 0 {
		duration = 60
	}
	autoRecording := "none"
	if conference.RecordingEnabled {
		autoRecording = "cloud"
	}
	payload := map[string]any{
		"topic":      conference.Title,
		"type":       2, // scheduled: can be restarted by the host
		"start_time": time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"duration":   duration,
		"agenda":     truncateRunes(conference.Description, 2000),
		"settings": map[string]any{
			"join_before_host": false,
			"waiting_room":     false,
			"approval_type":    2,
			"auto_recording":   autoRecording,
		},
	}
	var m zoomMeeting
	path := "/users/" + url.PathEscape(cfg.HostUser) + "/meetings"
	if err := s.zoomDo(ctx, cfg, http.MethodPost, path, payload, &m); err != nil {
		return err
	}
	if m.ID.String() == "" || !isExternalJoinURL(m.JoinURL) {
		return errors.New("zoom: did not return a meeting")
	}
	conference.ProviderMeetingID = m.ID.String()
	conference.ProviderJoinURL = m.JoinURL
	return nil
}

// zoomEnsureMeeting creates the meeting on first start; a restarted
// conference reuses its meeting.
func (s *ConferenceService) zoomEnsureMeeting(ctx context.Context, cfg zoomConfig, conference *models.Conference) error {
	if conference.ProviderMeetingID != "" {
		return nil
	}
	return s.zoomCreateMeeting(ctx, cfg, conference)
}

// zoomJoinURL returns the host start link (fetched fresh: it embeds a
// short-lived host token) for moderators, the attendee link otherwise.
func (s *ConferenceService) zoomJoinURL(ctx context.Context, cfg zoomConfig, conference *models.Conference, moderator bool) (string, error) {
	if conference.ProviderMeetingID == "" {
		if !moderator {
			return "", ErrConferenceNotRunning
		}
		if err := s.zoomCreateMeeting(ctx, cfg, conference); err != nil {
			return "", err
		}
		if err := s.confRepo.Update(ctx, conference); err != nil {
			return "", err
		}
	}
	if !moderator {
		if isExternalJoinURL(conference.ProviderJoinURL) {
			return conference.ProviderJoinURL, nil
		}
	}
	var m zoomMeeting
	err := s.zoomDo(ctx, cfg, http.MethodGet, "/meetings/"+url.PathEscape(conference.ProviderMeetingID), nil, &m)
	if zoomNotFound(err) && moderator {
		// The meeting was deleted on the Zoom side: make a new one.
		if err := s.zoomCreateMeeting(ctx, cfg, conference); err != nil {
			return "", err
		}
		if err := s.confRepo.Update(ctx, conference); err != nil {
			return "", err
		}
		err = s.zoomDo(ctx, cfg, http.MethodGet, "/meetings/"+url.PathEscape(conference.ProviderMeetingID), nil, &m)
	}
	if err != nil {
		if zoomNotFound(err) {
			return "", ErrConferenceNotRunning
		}
		return "", err
	}
	if moderator {
		if !isExternalJoinURL(m.StartURL) {
			return "", errors.New("zoom: did not return a host link")
		}
		return m.StartURL, nil
	}
	if !isExternalJoinURL(m.JoinURL) {
		return "", ErrConferenceNotRunning
	}
	return m.JoinURL, nil
}

// zoomEndMeeting ends a live meeting, best effort.
func (s *ConferenceService) zoomEndMeeting(ctx context.Context, cfg zoomConfig, meetingID string) {
	_ = s.zoomDo(ctx, cfg, http.MethodPut, "/meetings/"+url.PathEscape(meetingID)+"/status", map[string]string{"action": "end"}, nil)
}

type zoomRecordingList struct {
	UUID           string `json:"uuid"`
	Topic          string `json:"topic"`
	StartTime      string `json:"start_time"`
	Duration       int    `json:"duration"`
	ShareURL       string `json:"share_url"`
	RecordingFiles []struct {
		ID             string `json:"id"`
		RecordingStart string `json:"recording_start"`
		RecordingEnd   string `json:"recording_end"`
		FileType       string `json:"file_type"`
		RecordingType  string `json:"recording_type"`
		PlayURL        string `json:"play_url"`
		Status         string `json:"status"`
	} `json:"recording_files"`
}

func zoomTime(v string) *time.Time {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(v))
	if err != nil {
		return nil
	}
	return &t
}

// zoomRecordings lists the cloud recordings of the meeting's latest
// instance as one recording whose formats are the playable files. Its id
// is the meeting instance UUID.
func (s *ConferenceService) zoomRecordings(ctx context.Context, cfg zoomConfig, meetingID string) ([]ConferenceRecording, error) {
	var out zoomRecordingList
	err := s.zoomDo(ctx, cfg, http.MethodGet, "/meetings/"+url.PathEscape(meetingID)+"/recordings", nil, &out)
	if zoomNotFound(err) {
		return []ConferenceRecording{}, nil // no recordings yet
	}
	if err != nil {
		return nil, err
	}
	if out.UUID == "" || len(out.RecordingFiles) == 0 {
		return []ConferenceRecording{}, nil
	}
	rec := ConferenceRecording{
		ID:              out.UUID,
		Title:           out.Topic,
		StartedAt:       zoomTime(out.StartTime),
		DurationMinutes: out.Duration,
		Published:       true,
		Formats:         []ConferenceRecordingFormat{},
	}
	for _, f := range out.RecordingFiles {
		if t := zoomTime(f.RecordingEnd); t != nil && (rec.EndedAt == nil || t.After(*rec.EndedAt)) {
			rec.EndedAt = t
		}
		if !isExternalJoinURL(f.PlayURL) || (f.Status != "" && f.Status != "completed") {
			continue
		}
		label := f.RecordingType
		if label == "" {
			label = strings.ToLower(f.FileType)
		}
		rec.Formats = append(rec.Formats, ConferenceRecordingFormat{Type: label, URL: f.PlayURL})
		if rec.PlaybackURL == "" && f.FileType == "MP4" {
			rec.PlaybackURL = f.PlayURL
		}
	}
	if rec.PlaybackURL == "" {
		if isExternalJoinURL(out.ShareURL) {
			rec.PlaybackURL = out.ShareURL
		} else if len(rec.Formats) > 0 {
			rec.PlaybackURL = rec.Formats[0].URL
		}
	}
	return []ConferenceRecording{rec}, nil
}

// zoomMeetingInstancePath escapes a meeting instance UUID the way Zoom
// requires: UUIDs that begin with "/" or contain "//" are double-encoded.
func zoomMeetingInstancePath(uuid string) string {
	escaped := url.PathEscape(uuid)
	if strings.HasPrefix(uuid, "/") || strings.Contains(uuid, "//") {
		escaped = url.PathEscape(escaped)
	}
	return escaped
}

// zoomDeleteRecordings moves a meeting instance's recordings to the Zoom
// trash.
func (s *ConferenceService) zoomDeleteRecordings(ctx context.Context, cfg zoomConfig, instanceUUID string) error {
	err := s.zoomDo(ctx, cfg, http.MethodDelete, "/meetings/"+zoomMeetingInstancePath(instanceUUID)+"/recordings?action=trash", nil, nil)
	if zoomNotFound(err) {
		return ErrRecordingNotFound
	}
	return err
}

// truncateRunes caps s at n runes.
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
