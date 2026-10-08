package handlers_test

// G002: rich-content media uploads, playback (with byte ranges) and caption
// tracks, against a freshly migrated scratch database. Skipped without
// PARITY_DB_URL / DATABASE_URL. All data is made up.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
)

// fakeMP4 is a minimal ISO-BMFF header the content sniffer identifies as
// video/mp4, padded so range requests have something to cut.
func fakeMP4() []byte {
	head := []byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p', 'm', 'p', '4', '2', 0, 0, 0, 0, 'm', 'p', '4', '2', 'i', 's', 'o', 'm'}
	body := bytes.Repeat([]byte("0123456789"), 100)
	return append(head, body...)
}

type mediaFixture struct {
	g       *gorm.DB
	app     *fiber.App
	course  *models.Course
	teacher uint
	student uint
	other   uint // a student of another course in the same tenant
	foreign uint // a user of another tenant
}

func newMediaFixture(t *testing.T) *mediaFixture {
	t.Helper()
	g := contentLaneDB(t)
	seedContentAccounts(t, g, 1, 2)
	course := seedContentCourse(t, g, 1, "MEDIA-101")
	otherCourse := seedContentCourse(t, g, 1, "MEDIA-201")

	users := postgres.NewUserRepository(g)
	for _, u := range []*models.User{
		{ID: 11, AccountID: 1, Name: "Teacher Example", LoginID: "teacher@example.test", Email: "teacher@example.test", PasswordHash: "x", Role: "user"},
		{ID: 12, AccountID: 1, Name: "Jane Doe", LoginID: "jane@example.test", Email: "jane@example.test", PasswordHash: "x", Role: "user"},
		{ID: 13, AccountID: 1, Name: "John Doe", LoginID: "john@example.test", Email: "john@example.test", PasswordHash: "x", Role: "user"},
		{ID: 14, AccountID: 2, Name: "Other Tenant", LoginID: "other@example.test", Email: "other@example.test", PasswordHash: "x", Role: "user"},
	} {
		require.NoError(t, users.Create(context.Background(), u))
	}
	enrollments := postgres.NewEnrollmentRepository(g)
	for _, e := range []*models.Enrollment{
		{UserID: 11, CourseID: course.ID, Type: "TeacherEnrollment", Role: "TeacherEnrollment", WorkflowState: "active", PseudonymPoolCode: "animals_v1"},
		{UserID: 12, CourseID: course.ID, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: "active", PseudonymPoolCode: "animals_v1"},
		{UserID: 13, CourseID: otherCourse.ID, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: "active", PseudonymPoolCode: "animals_v1"},
	} {
		require.NoError(t, enrollments.Create(context.Background(), e))
	}

	files := service.NewFileService(postgres.NewFolderRepository(g), postgres.NewAttachmentRepository(g), t.TempDir())
	svc := service.NewMediaService(files, postgres.NewMediaObjectRepository(g), postgres.NewMediaTrackRepository(g))
	authz := handlers.NewResourceAuthorizer(enrollments, users, postgres.NewCourseRepository(g))
	h := handlers.NewMediaObjectHandler(svc, authz)

	app := fiber.New()
	// The caller is chosen per request with the X-Test-User header.
	app.Use(func(c *fiber.Ctx) error {
		var uid uint
		fmt.Sscan(c.Get("X-Test-User"), &uid)
		acct := uint(1)
		if uid == 14 {
			acct = 2
		}
		c.Locals("user_id", uid)
		c.Locals("account_id", acct)
		return c.Next()
	})
	app.Post("/media_objects", h.Upload)
	app.Get("/users/self/media_objects", h.ListMine)
	app.Get("/media_objects/:media_id", h.Get)
	app.Put("/media_objects/:media_id", h.Update)
	app.Delete("/media_objects/:media_id", h.Delete)
	app.Get("/media_objects/:media_id/stream", h.Stream)
	app.Get("/media_objects/:media_id/media_tracks", h.ListTracks)
	app.Post("/media_objects/:media_id/media_tracks", h.SaveTrack)
	app.Get("/media_objects/:media_id/media_tracks/:track_id/content", h.TrackContent)
	app.Delete("/media_objects/:media_id/media_tracks/:track_id", h.DeleteTrack)

	return &mediaFixture{g: g, app: app, course: course, teacher: 11, student: 12, other: 13, foreign: 14}
}

func (f *mediaFixture) do(t *testing.T, user uint, method, path string, body io.Reader, contentType string, headers ...string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("X-Test-User", fmt.Sprint(user))
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := f.app.Test(req, -1)
	require.NoError(t, err)
	return resp
}

func (f *mediaFixture) upload(t *testing.T, user uint, filename, claimedType string, data []byte, fields map[string]string) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		require.NoError(t, w.WriteField(k, v))
	}
	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	hdr.Set("Content-Type", claimedType)
	part, err := w.CreatePart(hdr)
	require.NoError(t, err)
	_, _ = part.Write(data)
	require.NoError(t, w.Close())
	return f.do(t, user, http.MethodPost, "/media_objects", &buf, w.FormDataContentType())
}

func decodeMap(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
	return m
}

func TestMediaObjects_UploadStreamAndCaptions(t *testing.T) {
	f := newMediaFixture(t)
	video := fakeMP4()

	// A student records a clip from an assignment in their course.
	resp := f.upload(t, f.student, "recording.mp4", "video/mp4", video, map[string]string{
		"title": "My answer", "context_type": "Course", "context_id": fmt.Sprint(f.course.ID),
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	created := decodeMap(t, resp)
	mediaID := created["media_id"].(string)
	assert.True(t, strings.HasPrefix(mediaID, "m-"))
	assert.Equal(t, "video", created["media_type"])
	assert.Equal(t, "Course", created["context_type"])
	streamURL := created["stream_url"].(string)
	assert.Equal(t, "/api/v1/media_objects/"+mediaID+"/stream", streamURL)

	// The teacher plays it in full ...
	resp = f.do(t, f.teacher, http.MethodGet, "/media_objects/"+mediaID+"/stream", nil, "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "video/mp4", resp.Header.Get("Content-Type"))
	assert.Equal(t, "bytes", resp.Header.Get("Accept-Ranges"))
	got, _ := io.ReadAll(resp.Body)
	assert.Equal(t, video, got)

	// ... and seeks (a byte range).
	resp = f.do(t, f.teacher, http.MethodGet, "/media_objects/"+mediaID+"/stream", nil, "", "Range", "bytes=4-11")
	require.Equal(t, http.StatusPartialContent, resp.StatusCode)
	assert.Equal(t, fmt.Sprintf("bytes 4-11/%d", len(video)), resp.Header.Get("Content-Range"))
	got, _ = io.ReadAll(resp.Body)
	assert.Equal(t, "ftypmp42", string(got))

	resp = f.do(t, f.teacher, http.MethodGet, "/media_objects/"+mediaID+"/stream", nil, "", "Range", "bytes=-5")
	require.Equal(t, http.StatusPartialContent, resp.StatusCode)
	got, _ = io.ReadAll(resp.Body)
	assert.Equal(t, "56789", string(got))

	resp = f.do(t, f.teacher, http.MethodGet, "/media_objects/"+mediaID+"/stream", nil, "", "Range", fmt.Sprintf("bytes=%d-", len(video)+10))
	assert.Equal(t, http.StatusRequestedRangeNotSatisfiable, resp.StatusCode)

	// The course teacher adds SRT captions; they are served as WebVTT.
	srt := "1\r\n0:00:01,000 --> 0:00:04,500\r\nHello class\r\n"
	resp = f.do(t, f.teacher, http.MethodPost, "/media_objects/"+mediaID+"/media_tracks", strings.NewReader(
		`{"locale":"en","kind":"captions","label":"English","content":`+jsonString(srt)+`}`), "application/json")
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	track := decodeMap(t, resp)
	trackURL := track["url"].(string)
	trackPath := strings.TrimPrefix(trackURL, "/api/v1")

	resp = f.do(t, f.student, http.MethodGet, trackPath, nil, "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/vtt")
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "WEBVTT\n\n1\n00:00:01.000 --> 00:00:04.500\nHello class\n", string(body))

	// Re-uploading the same locale replaces the track (one per locale).
	vtt := "WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nHi everyone\n"
	resp = f.do(t, f.student, http.MethodPost, "/media_objects/"+mediaID+"/media_tracks", strings.NewReader(
		`{"locale":"en","kind":"subtitles","content":`+jsonString(vtt)+`}`), "application/json")
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp = f.do(t, f.student, http.MethodGet, "/media_objects/"+mediaID, nil, "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	detail := decodeMap(t, resp)
	tracks := detail["media_tracks"].([]any)
	require.Len(t, tracks, 1)
	assert.Equal(t, "subtitles", tracks[0].(map[string]any)["kind"])
	assert.Equal(t, true, detail["can_edit"])

	// A student of another course can play the clip (they were shown the
	// link) but cannot change its captions.
	resp = f.do(t, f.other, http.MethodGet, "/media_objects/"+mediaID+"/stream", nil, "")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp = f.do(t, f.other, http.MethodGet, "/media_objects/"+mediaID, nil, "")
	assert.Equal(t, false, decodeMap(t, resp)["can_edit"])
	resp = f.do(t, f.other, http.MethodPost, "/media_objects/"+mediaID+"/media_tracks", strings.NewReader(
		`{"locale":"fr","content":`+jsonString(vtt)+`}`), "application/json")
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp = f.do(t, f.other, http.MethodDelete, "/media_objects/"+mediaID, nil, "")
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	// Another tenant gets 404 for the media, its stream and its captions.
	for _, p := range []string{"/media_objects/" + mediaID, "/media_objects/" + mediaID + "/stream", trackPath} {
		resp = f.do(t, f.foreign, http.MethodGet, p, nil, "")
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, p)
	}

	// The student's library lists it; the owner deletes it and playback stops.
	resp = f.do(t, f.student, http.MethodGet, "/users/self/media_objects", nil, "")
	var mine []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&mine))
	require.Len(t, mine, 1)
	// The teacher may fix captions but not delete a student's recording.
	resp = f.do(t, f.teacher, http.MethodDelete, "/media_objects/"+mediaID, nil, "")
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp = f.do(t, f.student, http.MethodDelete, "/media_objects/"+mediaID, nil, "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp = f.do(t, f.teacher, http.MethodGet, "/media_objects/"+mediaID+"/stream", nil, "")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestMediaObjects_UploadRejectsNonMediaAndForeignCourse(t *testing.T) {
	f := newMediaFixture(t)

	// HTML renamed to .mp4 is refused by content sniffing.
	resp := f.upload(t, f.student, "clip.mp4", "video/mp4", []byte("<!DOCTYPE html><html><script>alert(1)</script></html>"), nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// A PDF is not media.
	resp = f.upload(t, f.student, "notes.pdf", "application/pdf", []byte("%PDF-1.4 fake"), nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// Uploading into a course you are not enrolled in is refused.
	resp = f.upload(t, f.other, "clip.mp4", "video/mp4", fakeMP4(), map[string]string{
		"context_type": "Course", "context_id": fmt.Sprint(f.course.ID),
	})
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	// Recorded audio (no sniffable header) keeps its claimed type.
	resp = f.upload(t, f.student, "blob", "audio/webm;codecs=opus", []byte{0x1a, 0x45, 0xdf, 0xa3, 0, 0, 0, 0}, nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	created := decodeMap(t, resp)
	assert.Equal(t, "audio", created["media_type"])
	assert.Equal(t, "audio/webm", created["content_type"])
	assert.Equal(t, "User", created["context_type"])
}

func TestMediaObjects_InvalidCaptionsAreRejected(t *testing.T) {
	f := newMediaFixture(t)
	resp := f.upload(t, f.teacher, "lecture.mp4", "video/mp4", fakeMP4(), nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	mediaID := decodeMap(t, resp)["media_id"].(string)

	for name, body := range map[string]string{
		"ttml":       `{"locale":"en","content":"<tt xmlns=\"http://www.w3.org/ns/ttml\" xml:lang=\"en\"></tt>"}`,
		"no cues":    `{"locale":"en","content":"just some words"}`,
		"bad locale": `{"locale":"english!","content":"WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nx\n"}`,
		"bad kind":   `{"locale":"en","kind":"karaoke","content":"WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nx\n"}`,
	} {
		resp = f.do(t, f.teacher, http.MethodPost, "/media_objects/"+mediaID+"/media_tracks", strings.NewReader(body), "application/json")
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, name)
	}

	// A track id of another media object is a 404.
	resp = f.upload(t, f.teacher, "other.mp4", "video/mp4", fakeMP4(), nil)
	otherID := decodeMap(t, resp)["media_id"].(string)
	resp = f.do(t, f.teacher, http.MethodPost, "/media_objects/"+otherID+"/media_tracks", strings.NewReader(
		`{"locale":"es","content":"WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nHola\n"}`), "application/json")
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	trackID := decodeMap(t, resp)["id"]
	resp = f.do(t, f.teacher, http.MethodGet, fmt.Sprintf("/media_objects/%s/media_tracks/%v/content", mediaID, trackID), nil, "")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp = f.do(t, f.teacher, http.MethodDelete, fmt.Sprintf("/media_objects/%s/media_tracks/%v", mediaID, trackID), nil, "")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp = f.do(t, f.teacher, http.MethodDelete, fmt.Sprintf("/media_objects/%s/media_tracks/%v", otherID, trackID), nil, "")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
