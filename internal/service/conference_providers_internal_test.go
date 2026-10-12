package service

// G098: BigBlueButton and Zoom provider clients, exercised against fake
// provider servers (an http.RoundTripper, so no sockets). Example data only.

import (
	"context"
	"crypto/sha1" //nolint:gosec // BBB checksum under test
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func textResponse(status int, contentType, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// fakeBBB is a minimal BigBlueButton API server that verifies checksums.
type fakeBBB struct {
	t          *testing.T
	secret     string
	mu         sync.Mutex
	open       bool
	calls      []string
	lastCreate url.Values
	recordings string // <recording> elements for getRecordings
	deleted    []string
}

func useFakeBBB(t *testing.T, svc *ConferenceService, secret string) *fakeBBB {
	f := &fakeBBB{t: t, secret: secret}
	svc.validateURL = func(context.Context, string) error { return nil }
	svc.httpClient = &http.Client{Transport: roundTripFunc(f.serve)}
	return f
}

func bbbXML(inner string) *http.Response {
	return textResponse(http.StatusOK, "text/xml", "<response>"+inner+"</response>")
}

func (f *fakeBBB) serve(r *http.Request) (*http.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	call := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	raw := r.URL.RawQuery
	i := strings.LastIndex(raw, "checksum=")
	if i < 0 {
		return bbbXML("<returncode>FAILED</returncode><messageKey>checksumError</messageKey>"), nil
	}
	got := raw[i+len("checksum="):]
	query := strings.TrimSuffix(raw[:i], "&")
	sum := sha1.Sum([]byte(call + query + f.secret)) //nolint:gosec // BBB checksum
	if hex.EncodeToString(sum[:]) != got {
		return bbbXML("<returncode>FAILED</returncode><messageKey>checksumError</messageKey>"), nil
	}
	f.calls = append(f.calls, call)
	q := r.URL.Query()
	switch call {
	case "create":
		f.open = true
		f.lastCreate = q
		return bbbXML("<returncode>SUCCESS</returncode><meetingID>" + q.Get("meetingID") + "</meetingID>"), nil
	case "getMeetingInfo":
		if f.open {
			return bbbXML("<returncode>SUCCESS</returncode><running>true</running>"), nil
		}
		return bbbXML("<returncode>FAILED</returncode><messageKey>notFound</messageKey>"), nil
	case "end":
		f.open = false
		return bbbXML("<returncode>SUCCESS</returncode>"), nil
	case "getRecordings":
		return bbbXML("<returncode>SUCCESS</returncode><recordings>" + f.recordings + "</recordings>"), nil
	case "deleteRecordings":
		f.deleted = append(f.deleted, q.Get("recordID"))
		return bbbXML("<returncode>SUCCESS</returncode><deleted>true</deleted>"), nil
	}
	return bbbXML("<returncode>FAILED</returncode><messageKey>unsupported</messageKey>"), nil
}

func bbbSettings(extra map[string]string) SettingsLookupFunc {
	settings := map[string]string{
		"conferences.bbb.url":    "https://bbb.example.com/bigbluebutton/",
		"conferences.bbb.secret": "example-secret",
	}
	for k, v := range extra {
		settings[k] = v
	}
	return func(_ context.Context, key string) (string, error) { return settings[key], nil }
}

func conferenceRepoAcceptingUpdates() *mocks.MockConferenceRepository {
	repo := new(mocks.MockConferenceRepository)
	repo.On("Update", mock.Anything, mock.Anything).Return(nil)
	return repo
}

func TestBBBStart_CreatesRecordableMeetingWithValidChecksum(t *testing.T) {
	svc := NewConferenceService(conferenceRepoAcceptingUpdates(), nil)
	svc.SetSettingsLookup(bbbSettings(map[string]string{"branding.frontend_url": "https://lms.example.com/"}))
	fake := useFakeBBB(t, svc, "example-secret")

	conf := &models.Conference{ID: 9, ContextType: "Course", ContextID: 5, ConferenceType: "BigBlueButton", Title: "Office hours", Duration: 45, RecordingEnabled: true}
	started, err := svc.StartConference(context.Background(), conf, 1)
	require.NoError(t, err)
	require.NotNil(t, started.StartedAt)
	assert.Equal(t, []string{"create"}, fake.calls)
	assert.Equal(t, "paper-conference-9", fake.lastCreate.Get("meetingID"))
	assert.Equal(t, "true", fake.lastCreate.Get("record"))
	assert.Equal(t, "false", fake.lastCreate.Get("autoStartRecording"))
	assert.Equal(t, "45", fake.lastCreate.Get("duration"))
	assert.Equal(t, "https://lms.example.com/courses/5/conferences", fake.lastCreate.Get("logoutURL"))
	assert.NotEqual(t, fake.lastCreate.Get("attendeePW"), fake.lastCreate.Get("moderatorPW"))
}

func TestBBBJoin_AttendeeWaitsWhenMeetingClosedModeratorReopens(t *testing.T) {
	partRepo := new(mocks.MockConferenceParticipantRepository)
	partRepo.On("FindByConferenceAndUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errCommsNotFound)
	partRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	svc := NewConferenceService(nil, partRepo)
	svc.SetSettingsLookup(bbbSettings(nil))
	fake := useFakeBBB(t, svc, "example-secret")

	started := time.Now()
	conf := &models.Conference{ID: 9, ContextType: "Course", ContextID: 5, ConferenceType: "BigBlueButton", Title: "Office hours", StartedAt: &started}

	_, err := svc.JoinConference(context.Background(), conf, 50, "Jane Doe", false, 1)
	assert.ErrorIs(t, err, ErrConferenceNotRunning, "the meeting closed on the server: the student waits")

	modURL, err := svc.JoinConference(context.Background(), conf, 40, "Pat Teacher", true, 1)
	require.NoError(t, err)
	assert.Contains(t, modURL, "role=MODERATOR")
	assert.Contains(t, fake.calls, "create", "the moderator re-opens the meeting")

	viewerURL, err := svc.JoinConference(context.Background(), conf, 50, "Jane Doe", false, 1)
	require.NoError(t, err)
	assert.Contains(t, viewerURL, "role=VIEWER")
}

func TestBBBRecordings_ListAndDeleteOnlyOwnRecording(t *testing.T) {
	svc := NewConferenceService(nil, nil)
	svc.SetSettingsLookup(bbbSettings(nil))
	fake := useFakeBBB(t, svc, "example-secret")
	fake.recordings = `<recording><recordID>rec-1</recordID><meetingID>paper-conference-9</meetingID><name>Office hours</name>` +
		`<published>true</published><startTime>1767225600000</startTime><endTime>1767229200000</endTime>` +
		`<playback><format><type>presentation</type><url>https://bbb.example.com/playback/rec-1</url><length>60</length></format></playback></recording>` +
		// A recording of a different meeting must never surface.
		`<recording><recordID>rec-other</recordID><meetingID>paper-conference-10</meetingID><name>Other</name></recording>`

	conf := &models.Conference{ID: 9, ConferenceType: "BigBlueButton", Title: "Office hours"}
	recs, err := svc.GetRecordings(context.Background(), conf, 1)
	require.NoError(t, err)
	require.Len(t, recs, 1)
	assert.Equal(t, "rec-1", recs[0].ID)
	assert.Equal(t, "https://bbb.example.com/playback/rec-1", recs[0].PlaybackURL)
	assert.Equal(t, 60, recs[0].DurationMinutes)
	require.NotNil(t, recs[0].StartedAt)
	assert.True(t, recs[0].Published)

	assert.ErrorIs(t, svc.DeleteRecording(context.Background(), conf, "rec-other", 1), ErrRecordingNotFound)
	assert.Empty(t, fake.deleted, "a recording of another meeting is never deleted")
	require.NoError(t, svc.DeleteRecording(context.Background(), conf, "rec-1", 1))
	assert.Equal(t, []string{"rec-1"}, fake.deleted)
}

func TestConferenceRecordings_ExternalLinkHasNone(t *testing.T) {
	svc := NewConferenceService(nil, nil)
	svc.SetSettingsLookup(bbbSettings(nil))
	conf := &models.Conference{ID: 9, ConferenceType: "BigBlueButton", JoinURL: "https://meet.example.com/room"}
	recs, err := svc.GetRecordings(context.Background(), conf, 1)
	require.NoError(t, err)
	assert.Empty(t, recs)
	assert.ErrorIs(t, svc.DeleteRecording(context.Background(), conf, "x", 1), ErrRecordingsUnsupported)
}

func TestBBB_SSRFGuardRejectsInternalServer(t *testing.T) {
	svc := NewConferenceService(conferenceRepoAcceptingUpdates(), nil)
	svc.SetSettingsLookup(func(_ context.Context, key string) (string, error) {
		return map[string]string{"conferences.bbb.url": "https://127.0.0.1/bigbluebutton/", "conferences.bbb.secret": "s"}[key], nil
	})
	_, err := svc.StartConference(context.Background(), &models.Conference{ID: 9, ConferenceType: "BigBlueButton", Title: "x"}, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rejected")
}

// fakeZoom is a minimal Zoom OAuth + REST API.
type fakeZoom struct {
	mu          sync.Mutex
	tokenCalls  int
	created     []map[string]any
	ended       []string
	deleted     []string
	recordings  string
	meetingGone bool
}

func useFakeZoom(svc *ConferenceService) *fakeZoom {
	f := &fakeZoom{}
	svc.validateURL = func(context.Context, string) error { return nil }
	svc.httpClient = &http.Client{Transport: roundTripFunc(f.serve)}
	return f
}

func (f *fakeZoom) serve(r *http.Request) (*http.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Host == "zoom.us" && r.URL.Path == "/oauth/token" {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "client-id" || pass != "client-secret" || r.URL.Query().Get("account_id") != "acct-1" ||
			r.URL.Query().Get("grant_type") != "account_credentials" {
			return textResponse(http.StatusUnauthorized, "application/json", `{"reason":"Invalid client"}`), nil
		}
		f.tokenCalls++
		return textResponse(http.StatusOK, "application/json", `{"access_token":"tok-1","expires_in":3600}`), nil
	}
	if r.URL.Host != "api.zoom.us" || r.Header.Get("Authorization") != "Bearer tok-1" {
		return textResponse(http.StatusUnauthorized, "application/json", `{"code":124,"message":"Invalid access token"}`), nil
	}
	path := strings.TrimPrefix(r.URL.EscapedPath(), "/v2")
	switch {
	case r.Method == http.MethodPost && path == "/users/host@example.com/meetings":
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.created = append(f.created, body)
		return textResponse(http.StatusCreated, "application/json",
			`{"id":85746065432,"join_url":"https://zoom.example.com/j/85746065432","start_url":"https://zoom.example.com/s/85746065432?zak=one"}`), nil
	case r.Method == http.MethodGet && path == "/meetings/85746065432":
		if f.meetingGone {
			return textResponse(http.StatusNotFound, "application/json", `{"code":3001,"message":"Meeting does not exist"}`), nil
		}
		return textResponse(http.StatusOK, "application/json",
			`{"id":85746065432,"join_url":"https://zoom.example.com/j/85746065432","start_url":"https://zoom.example.com/s/85746065432?zak=fresh"}`), nil
	case r.Method == http.MethodPut && path == "/meetings/85746065432/status":
		f.ended = append(f.ended, "85746065432")
		return textResponse(http.StatusNoContent, "application/json", ""), nil
	case r.Method == http.MethodGet && path == "/meetings/85746065432/recordings":
		if f.recordings == "" {
			return textResponse(http.StatusNotFound, "application/json", `{"code":3301,"message":"This recording does not exist."}`), nil
		}
		return textResponse(http.StatusOK, "application/json", f.recordings), nil
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/meetings/") && strings.HasSuffix(path, "/recordings"):
		f.deleted = append(f.deleted, strings.TrimSuffix(strings.TrimPrefix(path, "/meetings/"), "/recordings")+"?"+r.URL.RawQuery)
		return textResponse(http.StatusNoContent, "application/json", ""), nil
	}
	return textResponse(http.StatusNotFound, "application/json", fmt.Sprintf(`{"code":404,"message":"no route %s %s"}`, r.Method, path)), nil
}

func zoomSettings() SettingsLookupFunc {
	settings := map[string]string{
		"conferences.zoom.account_id":    "acct-1",
		"conferences.zoom.client_id":     "client-id",
		"conferences.zoom.client_secret": "client-secret",
		"conferences.zoom.host_user":     "host@example.com",
	}
	return func(_ context.Context, key string) (string, error) { return settings[key], nil }
}

func TestZoom_StartJoinEndLifecycle(t *testing.T) {
	partRepo := new(mocks.MockConferenceParticipantRepository)
	partRepo.On("FindByConferenceAndUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errCommsNotFound)
	partRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	svc := NewConferenceService(conferenceRepoAcceptingUpdates(), partRepo)
	svc.SetSettingsLookup(zoomSettings())
	fake := useFakeZoom(svc)

	conf := &models.Conference{ID: 12, ContextType: "Course", ContextID: 5, ConferenceType: "Zoom", Title: "Review session", Duration: 30, RecordingEnabled: true}
	_, err := svc.StartConference(context.Background(), conf, 1)
	require.NoError(t, err)
	require.Len(t, fake.created, 1)
	assert.Equal(t, "Review session", fake.created[0]["topic"])
	assert.Equal(t, "cloud", fake.created[0]["settings"].(map[string]any)["auto_recording"])
	assert.Equal(t, "85746065432", conf.ProviderMeetingID)

	hostURL, err := svc.JoinConference(context.Background(), conf, 40, "Pat Teacher", true, 1)
	require.NoError(t, err)
	assert.Equal(t, "https://zoom.example.com/s/85746065432?zak=fresh", hostURL, "the host link is fetched fresh, never stored")

	studentURL, err := svc.JoinConference(context.Background(), conf, 50, "Jane Doe", false, 1)
	require.NoError(t, err)
	assert.Equal(t, "https://zoom.example.com/j/85746065432", studentURL)

	_, err = svc.EndConference(context.Background(), conf, 1)
	require.NoError(t, err)
	assert.Equal(t, []string{"85746065432"}, fake.ended)
	assert.Equal(t, 1, fake.tokenCalls, "the access token is cached")
}

func TestZoom_Recordings(t *testing.T) {
	svc := NewConferenceService(nil, nil)
	svc.SetSettingsLookup(zoomSettings())
	fake := useFakeZoom(svc)
	conf := &models.Conference{ID: 12, ConferenceType: "Zoom", ProviderMeetingID: "85746065432"}

	recs, err := svc.GetRecordings(context.Background(), conf, 1)
	require.NoError(t, err)
	assert.Empty(t, recs, "no cloud recording yet")

	fake.recordings = `{"uuid":"/abc//def==","topic":"Review session","start_time":"2026-01-01T15:00:00Z","duration":42,
		"recording_files":[{"id":"f1","file_type":"MP4","recording_type":"shared_screen_with_speaker_view","play_url":"https://zoom.example.com/rec/play/f1","status":"completed","recording_end":"2026-01-01T15:42:00Z"},
		{"id":"f2","file_type":"M4A","recording_type":"audio_only","play_url":"https://zoom.example.com/rec/play/f2","status":"completed"}]}`
	recs, err = svc.GetRecordings(context.Background(), conf, 1)
	require.NoError(t, err)
	require.Len(t, recs, 1)
	assert.Equal(t, "/abc//def==", recs[0].ID)
	assert.Equal(t, "https://zoom.example.com/rec/play/f1", recs[0].PlaybackURL)
	assert.Len(t, recs[0].Formats, 2)
	assert.Equal(t, 42, recs[0].DurationMinutes)

	assert.ErrorIs(t, svc.DeleteRecording(context.Background(), conf, "some-other-uuid", 1), ErrRecordingNotFound)
	require.NoError(t, svc.DeleteRecording(context.Background(), conf, "/abc//def==", 1))
	require.Len(t, fake.deleted, 1)
	// A UUID with "/" is double-encoded, and recordings go to the trash.
	assert.Equal(t, "%252Fabc%252F%252Fdef==?action=trash", fake.deleted[0])
}

func TestConferenceProviders(t *testing.T) {
	svc := NewConferenceService(nil, nil)
	assert.Equal(t, []ConferenceProviderInfo{
		{Type: "BigBlueButton", Name: "BigBlueButton"},
		{Type: "Zoom", Name: "Zoom"},
	}, svc.Providers(context.Background(), 1))

	svc.SetSettingsLookup(zoomSettings())
	got := svc.Providers(context.Background(), 1)
	assert.False(t, got[0].Configured)
	assert.True(t, got[1].Configured)
	assert.True(t, got[1].SupportsRecording)
}

// A tenant that configures another tenant's Zoom account and client ids
// with a wrong secret must not reuse the token cached for the real
// credentials: the cache is keyed on the secret too.
func TestZoom_TokenCacheIsKeyedOnTheSecret(t *testing.T) {
	svc := NewConferenceService(nil, nil)
	fake := useFakeZoom(svc)
	good := zoomConfig{AccountID: "acct-1", ClientID: "client-id", ClientSecret: "client-secret", HostUser: "host@example.com"}
	tok, err := svc.zoomToken(context.Background(), good)
	require.NoError(t, err)
	assert.Equal(t, "tok-1", tok)

	forged := good
	forged.ClientSecret = "guessed"
	_, err = svc.zoomToken(context.Background(), forged)
	require.Error(t, err, "a wrong secret goes to Zoom and is refused")
	assert.Equal(t, 1, fake.tokenCalls)

	tok, err = svc.zoomToken(context.Background(), good)
	require.NoError(t, err)
	assert.Equal(t, "tok-1", tok)
	assert.Equal(t, 1, fake.tokenCalls, "the real credentials still hit the cache")
}
