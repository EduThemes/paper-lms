package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha1" // #nosec G505 -- the BigBlueButton API checksum is specified as SHA-1
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// BigBlueButton API client (G098). Every call is a GET whose query carries
// checksum = sha1(callName + queryString + sharedSecret), per the BBB API
// spec. The server URL is operator-configured; each request still passes
// the SSRF guard before it is built.

// bbbConfig is the resolved BigBlueButton server configuration.
type bbbConfig struct {
	BaseURL string // normalized, ends in "/api/"
	Secret  string
}

// resolveBBB returns the BigBlueButton config for the caller's tenant, or
// ok=false when BBB is not configured.
func (s *ConferenceService) resolveBBB(ctx context.Context, accountID uint) (bbbConfig, bool) {
	rawURL := s.resolveSetting(ctx, accountID, "conferences.bbb.url")
	secret := s.resolveSetting(ctx, accountID, "conferences.bbb.secret")
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
	if conference.ConferenceType != ConferenceTypeBigBlueButton || isExternalJoinURL(conference.JoinURL) {
		return bbbConfig{}, false
	}
	return s.resolveBBB(ctx, accountID)
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
	sum := sha1.Sum([]byte(call + query + cfg.Secret)) // #nosec G401 -- BBB API checksum is SHA-1 by spec
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

// bbbGet performs a server-side BBB API call and decodes the XML response
// into out (a struct embedding bbbResponse). A FAILED returncode is not an
// error here, so callers can branch on messageKey.
func (s *ConferenceService) bbbGet(ctx context.Context, apiURL string, out any) error {
	if err := s.checkOutboundURL(ctx, apiURL); err != nil {
		return fmt.Errorf("the BigBlueButton server: URL rejected: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return err
	}
	resp, err := s.client().Do(req)
	if err != nil {
		return fmt.Errorf("the BigBlueButton server: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if err := xml.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(out); err != nil {
		return fmt.Errorf("the BigBlueButton server: returned an unreadable response (HTTP %d)", resp.StatusCode)
	}
	return nil
}

// bbbCall performs a BBB API call that must succeed.
func (s *ConferenceService) bbbCall(ctx context.Context, apiURL string) error {
	var out bbbResponse
	if err := s.bbbGet(ctx, apiURL, &out); err != nil {
		return err
	}
	if out.ReturnCode != "SUCCESS" {
		return fmt.Errorf("the BigBlueButton server: refused the request: %s %s", out.MessageKey, out.Message)
	}
	return nil
}

// bbbCreateParams are the create-call parameters for a conference.
// Recording is opt-in per conference: the moderator starts and stops it
// from inside the meeting (no auto-start), matching Canvas.
func (s *ConferenceService) bbbCreateParams(ctx context.Context, cfg bbbConfig, conference *models.Conference, accountID uint) url.Values {
	meetingID := bbbMeetingID(conference.ID)
	params := url.Values{}
	params.Set("name", conference.Title)
	params.Set("meetingID", meetingID)
	params.Set("attendeePW", bbbPassword(cfg, meetingID, "attendee"))
	params.Set("moderatorPW", bbbPassword(cfg, meetingID, "moderator"))
	if conference.Duration > 0 {
		params.Set("duration", strconv.Itoa(conference.Duration))
	}
	if conference.RecordingEnabled {
		params.Set("record", "true")
		params.Set("autoStartRecording", "false")
		params.Set("allowStartStopRecording", "true")
	} else {
		params.Set("record", "false")
	}
	if logout := s.logoutURL(ctx, conference, accountID); logout != "" {
		params.Set("logoutURL", logout)
	}
	// Tag the meeting so recordings can be traced back to Paper.
	params.Set("meta_paper-conference-id", strconv.FormatUint(uint64(conference.ID), 10))
	return params
}

// bbbCreateMeeting creates (or re-opens) the conference's meeting. BBB's
// create is idempotent for a meeting that is still running.
func (s *ConferenceService) bbbCreateMeeting(ctx context.Context, cfg bbbConfig, conference *models.Conference, accountID uint) error {
	return s.bbbCall(ctx, bbbURL(cfg, "create", s.bbbCreateParams(ctx, cfg, conference, accountID)))
}

// bbbEndMeeting ends the meeting, best effort.
func (s *ConferenceService) bbbEndMeeting(ctx context.Context, cfg bbbConfig, conference *models.Conference) {
	meetingID := bbbMeetingID(conference.ID)
	params := url.Values{}
	params.Set("meetingID", meetingID)
	params.Set("password", bbbPassword(cfg, meetingID, "moderator"))
	_ = s.bbbCall(ctx, bbbURL(cfg, "end", params))
}

// bbbMeetingOpen reports whether the meeting exists on the BBB server
// (created and not yet ended).
func (s *ConferenceService) bbbMeetingOpen(ctx context.Context, cfg bbbConfig, conference *models.Conference) (bool, error) {
	params := url.Values{}
	params.Set("meetingID", bbbMeetingID(conference.ID))
	var out bbbResponse
	if err := s.bbbGet(ctx, bbbURL(cfg, "getMeetingInfo", params), &out); err != nil {
		return false, err
	}
	if out.ReturnCode == "SUCCESS" {
		return true, nil
	}
	if out.MessageKey == "notFound" {
		return false, nil
	}
	return false, fmt.Errorf("the BigBlueButton server: refused the request: %s %s", out.MessageKey, out.Message)
}

// bbbJoinURL returns the signed join link. A moderator re-opens the
// meeting if the server closed it (everyone left); an attendee is told to
// wait instead of being sent to a BBB error page.
func (s *ConferenceService) bbbJoinURL(ctx context.Context, cfg bbbConfig, conference *models.Conference, userID uint, userName string, moderator bool, accountID uint) (string, error) {
	if moderator {
		if err := s.bbbCreateMeeting(ctx, cfg, conference, accountID); err != nil {
			return "", err
		}
	} else {
		open, err := s.bbbMeetingOpen(ctx, cfg, conference)
		if err != nil {
			return "", err
		}
		if !open {
			return "", ErrConferenceNotRunning
		}
	}

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
	params.Set("userID", strconv.FormatUint(uint64(userID), 10))
	params.Set("redirect", "true")
	return bbbURL(cfg, "join", params), nil
}

// BBB getRecordings response.
type bbbRecordingsResponse struct {
	bbbResponse
	Recordings []bbbRecording `xml:"recordings>recording"`
}

type bbbRecording struct {
	RecordID  string `xml:"recordID"`
	MeetingID string `xml:"meetingID"`
	Name      string `xml:"name"`
	Published string `xml:"published"`
	StartTime string `xml:"startTime"`
	EndTime   string `xml:"endTime"`
	Formats   []struct {
		Type   string `xml:"type"`
		URL    string `xml:"url"`
		Length string `xml:"length"`
	} `xml:"playback>format"`
}

// bbbMillis parses a BBB epoch-milliseconds timestamp.
func bbbMillis(v string) *time.Time {
	ms, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil || ms <= 0 {
		return nil
	}
	t := time.UnixMilli(ms).UTC()
	return &t
}

// bbbRecordings lists the recordings of the conference's meeting.
func (s *ConferenceService) bbbRecordings(ctx context.Context, cfg bbbConfig, conference *models.Conference) ([]ConferenceRecording, error) {
	meetingID := bbbMeetingID(conference.ID)
	params := url.Values{}
	params.Set("meetingID", meetingID)
	var out bbbRecordingsResponse
	if err := s.bbbGet(ctx, bbbURL(cfg, "getRecordings", params), &out); err != nil {
		return nil, err
	}
	if out.ReturnCode != "SUCCESS" {
		return nil, fmt.Errorf("the BigBlueButton server: refused the request: %s %s", out.MessageKey, out.Message)
	}
	recs := make([]ConferenceRecording, 0, len(out.Recordings))
	for _, r := range out.Recordings {
		// getRecordings filters by meetingID already; double-check so a
		// misbehaving server can't surface another meeting's recording.
		if r.MeetingID != "" && r.MeetingID != meetingID {
			continue
		}
		rec := ConferenceRecording{
			ID:        r.RecordID,
			Title:     r.Name,
			StartedAt: bbbMillis(r.StartTime),
			EndedAt:   bbbMillis(r.EndTime),
			Published: r.Published == "true",
			Formats:   []ConferenceRecordingFormat{},
		}
		if rec.Title == "" {
			rec.Title = conference.Title
		}
		for _, f := range r.Formats {
			if !isExternalJoinURL(f.URL) {
				continue
			}
			length, _ := strconv.Atoi(strings.TrimSpace(f.Length))
			rec.Formats = append(rec.Formats, ConferenceRecordingFormat{Type: f.Type, URL: f.URL, LengthMinutes: length})
			if rec.PlaybackURL == "" || f.Type == "presentation" {
				rec.PlaybackURL = f.URL
			}
			if length > rec.DurationMinutes {
				rec.DurationMinutes = length
			}
		}
		recs = append(recs, rec)
	}
	return recs, nil
}

// bbbDeleteRecording deletes one recording.
func (s *ConferenceService) bbbDeleteRecording(ctx context.Context, cfg bbbConfig, recordID string) error {
	params := url.Values{}
	params.Set("recordID", recordID)
	return s.bbbCall(ctx, bbbURL(cfg, "deleteRecordings", params))
}
