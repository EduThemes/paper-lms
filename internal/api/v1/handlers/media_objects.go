package handlers

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/settingsctx"
	"github.com/EduThemes/paper-lms/internal/storage"
)

// MediaObjectHandler serves rich-content audio/video: upload (file picker
// or browser recording), playback with HTTP range support, and caption
// tracks (Canvas media_objects + media_tracks, G002).
type MediaObjectHandler struct {
	svc   *service.MediaService
	authz *ResourceAuthorizer
}

// NewMediaObjectHandler wires the handler. authz supplies the DB-backed
// admin role and the course checks; without it only owners can edit.
func NewMediaObjectHandler(svc *service.MediaService, authz *ResourceAuthorizer) *MediaObjectHandler {
	return &MediaObjectHandler{svc: svc, authz: authz}
}

func mediaStreamURL(m *models.MediaObject) string {
	return fmt.Sprintf("/api/v1/media_objects/%s/stream", m.MediaID)
}

func mediaTrackJSON(m *models.MediaObject, t *models.MediaTrack) fiber.Map {
	return fiber.Map{
		"id":         t.ID,
		"locale":     t.Locale,
		"kind":       t.Kind,
		"label":      t.Label,
		"url":        fmt.Sprintf("/api/v1/media_objects/%s/media_tracks/%d/content", m.MediaID, t.ID),
		"created_at": t.CreatedAt,
		"updated_at": t.UpdatedAt,
	}
}

func mediaObjectJSON(m *models.MediaObject, tracks []models.MediaTrack, canEdit bool) fiber.Map {
	out := fiber.Map{
		"id":           m.ID,
		"media_id":     m.MediaID,
		"title":        m.Title,
		"media_type":   m.MediaType,
		"content_type": m.ContentType,
		"context_type": m.ContextType,
		"context_id":   m.ContextID,
		"user_id":      m.UserID,
		"created_at":   m.CreatedAt,
		"updated_at":   m.UpdatedAt,
		"stream_url":   mediaStreamURL(m),
		"can_edit":     canEdit,
	}
	if tracks != nil {
		list := make([]fiber.Map, len(tracks))
		for i := range tracks {
			list[i] = mediaTrackJSON(m, &tracks[i])
		}
		out["media_tracks"] = list
	}
	return out
}

// loadMedia loads :media_id in the caller's tenant and checks the caller
// may read it. It writes a 404 and returns wrote=true when the media is
// missing, deleted, in another tenant or not readable by the caller (404,
// not 403, so a media id does not reveal that the clip exists); the
// caller must return immediately in that case.
func (h *MediaObjectHandler) loadMedia(c *fiber.Ctx) (*models.MediaObject, bool, error) {
	mediaID := strings.TrimSpace(c.Params("media_id"))
	if mediaID == "" || len(mediaID) > 64 {
		return nil, true, responses.NotFound(c, "media object")
	}
	m, err := h.svc.Get(c.Context(), mediaID, callerAccountID(c))
	if err != nil || m == nil {
		return nil, true, responses.NotFound(c, "media object")
	}
	if !h.canRead(c, m) {
		return nil, true, responses.NotFound(c, "media object")
	}
	return m, false, nil
}

// maxSharedCourseChecks bounds the courses canRead tries for the
// owner-teaches-you rule (each check is a few queries).
const maxSharedCourseChecks = 10

// canRead is Canvas's context rule for media: the owner and tenant admins
// always; for media uploaded in a course, anyone who can read that course
// (an enrollment the course gate accepts, or a course admin). Media in a
// user's own context is the owner's alone, with one extension for both
// kinds: a clip is also readable in any course its owner teaches (teacher,
// TA or designer) that the caller can read. That covers a teacher reusing
// a clip from their library in another course they teach, and course copy
// carrying a clip's embed into the new term's course. The tenant was
// already checked by the load.
func (h *MediaObjectHandler) canRead(c *fiber.Ctx, m *models.MediaObject) bool {
	uid, _ := c.Locals("user_id").(uint)
	if uid == 0 {
		return false
	}
	if uid == m.UserID || callerIsAdmin(c, h.authz) {
		return true
	}
	if h.authz == nil {
		return false
	}
	if m.ContextType == "Course" && h.authz.RequireCourseEnrolled(c, m.ContextID) == nil {
		return true
	}
	courses, err := h.svc.CoursesTaughtToViewer(c.Context(), m, uid, maxSharedCourseChecks)
	if err != nil {
		return false
	}
	for _, courseID := range courses {
		if m.ContextType == "Course" && courseID == m.ContextID {
			continue // already refused above
		}
		if h.authz.RequireCourseEnrolled(c, courseID) == nil {
			return true
		}
	}
	return false
}

// canEdit: the owner, a tenant admin, or an instructor of the course the
// media was uploaded in.
func (h *MediaObjectHandler) canEdit(c *fiber.Ctx, m *models.MediaObject) bool {
	if uid, _ := c.Locals("user_id").(uint); uid != 0 && uid == m.UserID {
		return true
	}
	if callerIsAdmin(c, h.authz) {
		return true
	}
	if m.ContextType == "Course" && h.authz != nil {
		return h.authz.RequireCourseInstructor(c, m.ContextID) == nil
	}
	return false
}

func mediaForbidden() error {
	return fiber.NewError(fiber.StatusForbidden, "you cannot change this media")
}

// Upload handles POST /media_objects (multipart: file, title,
// context_type=Course + context_id to attach it to a course).
func (h *MediaObjectHandler) Upload(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return responses.BadRequest(c, "file is required")
	}

	ctxType, ctxID := "User", userID
	if strings.EqualFold(strings.TrimSpace(c.FormValue("context_type")), "Course") {
		id, convErr := strconv.ParseUint(strings.TrimSpace(c.FormValue("context_id")), 10, 64)
		if convErr != nil || id == 0 {
			return responses.BadRequest(c, "Invalid context_id")
		}
		if h.authz == nil {
			return responses.NotFound(c, "course")
		}
		if err := h.authz.RequireCourseEnrolled(c, uint(id)); err != nil {
			return err
		}
		ctxType, ctxID = "Course", uint(id)
	}

	file, err := fileHeader.Open()
	if err != nil {
		return responses.InternalError(c, "Could not open uploaded file")
	}
	defer func() { _ = file.Close() }()

	accountID := callerAccountID(c)
	m, err := h.svc.Upload(settingsctx.WithAccountID(c.Context(), accountID), service.UploadMediaInput{
		AccountID:   accountID,
		UserID:      userID,
		ContextType: ctxType,
		ContextID:   ctxID,
		Title:       c.FormValue("title"),
		Filename:    fileHeader.Filename,
		ContentType: fileHeader.Header.Get("Content-Type"),
		Size:        fileHeader.Size,
		Data:        file,
	})
	if err != nil {
		msg := err.Error()
		if errors.Is(err, service.ErrMediaUnsupported) {
			return responses.BadRequest(c, service.ErrMediaUnsupported.Error())
		}
		if strings.Contains(msg, "exceeds maximum") || strings.Contains(msg, "not allowed") || strings.Contains(msg, "does not match") {
			return responses.BadRequest(c, strings.TrimPrefix(msg, "store media: "))
		}
		return responses.InternalError(c, "Could not store media")
	}
	return c.Status(fiber.StatusCreated).JSON(mediaObjectJSON(m, []models.MediaTrack{}, true))
}

// ListMine handles GET /users/self/media_objects.
func (h *MediaObjectHandler) ListMine(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	params := middleware.GetPagination(c)
	res, err := h.svc.ListByUser(c.Context(), userID, callerAccountID(c), params)
	if err != nil {
		return responses.InternalError(c, "Could not list media")
	}
	responses.SetPaginationHeaders(c, res.TotalCount, res.Page, res.PerPage)
	out := make([]fiber.Map, len(res.Items))
	for i := range res.Items {
		out[i] = mediaObjectJSON(&res.Items[i], nil, true)
	}
	return c.JSON(out)
}

// Get handles GET /media_objects/:media_id (with its caption tracks).
func (h *MediaObjectHandler) Get(c *fiber.Ctx) error {
	m, wrote, err := h.loadMedia(c)
	if wrote || err != nil {
		return err
	}
	tracks, err := h.svc.ListTracks(c.Context(), m)
	if err != nil {
		return responses.InternalError(c, "Could not load captions")
	}
	if tracks == nil {
		tracks = []models.MediaTrack{}
	}
	return c.JSON(mediaObjectJSON(m, tracks, h.canEdit(c, m)))
}

// Update handles PUT /media_objects/:media_id ({"title": "..."}).
func (h *MediaObjectHandler) Update(c *fiber.Ctx) error {
	m, wrote, err := h.loadMedia(c)
	if wrote || err != nil {
		return err
	}
	if !h.canEdit(c, m) {
		return mediaForbidden()
	}
	var body struct {
		Title string `json:"title"`
	}
	if err := c.BodyParser(&body); err != nil {
		return responses.BadRequest(c, "Invalid request body")
	}
	if err := h.svc.Rename(c.Context(), m, body.Title); err != nil {
		return responses.InternalError(c, "Could not update media")
	}
	return c.JSON(mediaObjectJSON(m, nil, true))
}

// Delete handles DELETE /media_objects/:media_id.
func (h *MediaObjectHandler) Delete(c *fiber.Ctx) error {
	m, wrote, err := h.loadMedia(c)
	if wrote || err != nil {
		return err
	}
	// Deleting breaks every embed of the clip (a student's recorded answer
	// included), so course instructors, who may fix captions, may not.
	if uid, _ := c.Locals("user_id").(uint); uid != m.UserID && !callerIsAdmin(c, h.authz) {
		return mediaForbidden()
	}
	if err := h.svc.Delete(c.Context(), m); err != nil {
		return responses.InternalError(c, "Could not delete media")
	}
	return c.JSON(fiber.Map{"delete": true})
}

// readCloser pairs a range-limited reader with the underlying file so
// fasthttp closes the file once the body is sent.
type readCloser struct {
	io.Reader
	io.Closer
}

// parseByteRange parses a single "bytes=start-end" range against size.
// Multi-range requests are not supported (players never send them).
func parseByteRange(header string, size int64) (start, end int64, ok bool) {
	if size <= 0 || !strings.HasPrefix(header, "bytes=") {
		return 0, 0, false
	}
	spec := strings.TrimSpace(strings.TrimPrefix(header, "bytes="))
	if strings.Contains(spec, ",") {
		return 0, 0, false
	}
	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	first, last := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	switch {
	case first == "" && last != "":
		// Suffix range: the last N bytes.
		n, err := strconv.ParseInt(last, 10, 64)
		if err != nil || n <= 0 {
			return 0, 0, false
		}
		if n > size {
			n = size
		}
		return size - n, size - 1, true
	case first != "":
		s, err := strconv.ParseInt(first, 10, 64)
		if err != nil || s < 0 || s >= size {
			return 0, 0, false
		}
		e := size - 1
		if last != "" {
			v, err := strconv.ParseInt(last, 10, 64)
			if err != nil || v < s {
				return 0, 0, false
			}
			if v < e {
				e = v
			}
		}
		return s, e, true
	}
	return 0, 0, false
}

// Stream handles GET /media_objects/:media_id/stream. It honours byte
// ranges (Safari will not play a video without them, and seeking needs
// them everywhere); S3 storage redirects to a presigned URL, which supports
// ranges natively.
func (h *MediaObjectHandler) Stream(c *fiber.Ctx) error {
	m, wrote, err := h.loadMedia(c)
	if wrote || err != nil {
		return err
	}
	att, err := h.svc.Attachment(c.Context(), m)
	if err != nil || att == nil {
		return responses.NotFound(c, "media object")
	}
	files := h.svc.Files()
	ctx := settingsctx.WithAccountID(c.Context(), m.AccountID)
	backend := files.StorageBackend()
	if _, isS3 := backend.(*storage.S3Backend); isS3 {
		u, err := files.GetFileURL(ctx, att.ID, m.AccountID)
		if err != nil {
			return responses.InternalError(c, "Could not generate media URL")
		}
		return c.Redirect(u, fiber.StatusTemporaryRedirect)
	}
	reader, err := backend.Get(ctx, att.StoragePath)
	if err != nil {
		return responses.InternalError(c, "Could not locate media")
	}

	// The stored type was checked against the file's bytes at upload and is
	// always audio/* or video/*, so it is safe to play inline.
	c.Set("Content-Type", m.ContentType)
	c.Set("Content-Disposition", "inline")
	c.Set("Accept-Ranges", "bytes")
	c.Set("Cache-Control", "private, max-age=3600")

	rs, seekable := reader.(io.ReadSeeker)
	if !seekable {
		return c.SendStream(reader)
	}
	size, err := rs.Seek(0, io.SeekEnd)
	if err != nil {
		_ = reader.Close()
		return responses.InternalError(c, "Could not read media")
	}
	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		_ = reader.Close()
		return responses.InternalError(c, "Could not read media")
	}

	rangeHeader := c.Get(fiber.HeaderRange)
	if rangeHeader == "" {
		return c.SendStream(reader, int(size))
	}
	start, end, ok := parseByteRange(rangeHeader, size)
	if !ok {
		_ = reader.Close()
		c.Set(fiber.HeaderContentRange, fmt.Sprintf("bytes */%d", size))
		return c.SendStatus(fiber.StatusRequestedRangeNotSatisfiable)
	}
	if _, err := rs.Seek(start, io.SeekStart); err != nil {
		_ = reader.Close()
		return responses.InternalError(c, "Could not read media")
	}
	length := end - start + 1
	c.Status(fiber.StatusPartialContent)
	c.Set(fiber.HeaderContentRange, fmt.Sprintf("bytes %d-%d/%d", start, end, size))
	return c.SendStream(readCloser{Reader: io.LimitReader(rs, length), Closer: reader}, int(length))
}

// ListTracks handles GET /media_objects/:media_id/media_tracks.
func (h *MediaObjectHandler) ListTracks(c *fiber.Ctx) error {
	m, wrote, err := h.loadMedia(c)
	if wrote || err != nil {
		return err
	}
	tracks, err := h.svc.ListTracks(c.Context(), m)
	if err != nil {
		return responses.InternalError(c, "Could not load captions")
	}
	out := make([]fiber.Map, len(tracks))
	for i := range tracks {
		out[i] = mediaTrackJSON(m, &tracks[i])
	}
	return c.JSON(out)
}

// SaveTrack handles POST /media_objects/:media_id/media_tracks. The body is
// JSON {locale, kind, label, content} or multipart with a caption "file"
// plus locale/kind/label fields. A track for an existing locale replaces it.
func (h *MediaObjectHandler) SaveTrack(c *fiber.Ctx) error {
	m, wrote, err := h.loadMedia(c)
	if wrote || err != nil {
		return err
	}
	if !h.canEdit(c, m) {
		return mediaForbidden()
	}
	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	var in service.SaveTrackInput
	if fh, ferr := c.FormFile("file"); ferr == nil {
		if fh.Size > service.MaxMediaTrackBytes {
			return responses.BadRequest(c, "caption files are limited to 1 MB")
		}
		f, err := fh.Open()
		if err != nil {
			return responses.BadRequest(c, "Could not read caption file")
		}
		raw, err := io.ReadAll(io.LimitReader(f, service.MaxMediaTrackBytes+1))
		_ = f.Close()
		if err != nil {
			return responses.BadRequest(c, "Could not read caption file")
		}
		in = service.SaveTrackInput{Locale: c.FormValue("locale"), Kind: c.FormValue("kind"), Label: c.FormValue("label"), Content: string(raw)}
	} else {
		var body struct {
			Locale  string `json:"locale"`
			Kind    string `json:"kind"`
			Label   string `json:"label"`
			Content string `json:"content"`
		}
		if err := c.BodyParser(&body); err != nil {
			return responses.BadRequest(c, "Invalid request body")
		}
		in = service.SaveTrackInput{Locale: body.Locale, Kind: body.Kind, Label: body.Label, Content: body.Content}
	}

	t, err := h.svc.SaveTrack(c.Context(), m, userID, in)
	if err != nil {
		if errors.Is(err, service.ErrMediaTrackInvalid) {
			return responses.BadRequest(c, err.Error())
		}
		return responses.InternalError(c, "Could not save captions")
	}
	return c.Status(fiber.StatusCreated).JSON(mediaTrackJSON(m, t))
}

// TrackContent handles GET /media_objects/:media_id/media_tracks/:track_id/content
// — the WebVTT file a <track> element loads.
func (h *MediaObjectHandler) TrackContent(c *fiber.Ctx) error {
	m, wrote, err := h.loadMedia(c)
	if wrote || err != nil {
		return err
	}
	trackID, err := c.ParamsInt("track_id")
	if err != nil || trackID <= 0 {
		return responses.NotFound(c, "media track")
	}
	t, err := h.svc.GetTrack(c.Context(), m, uint(trackID))
	if err != nil {
		return responses.NotFound(c, "media track")
	}
	c.Set("Content-Type", "text/vtt; charset=utf-8")
	c.Set("Cache-Control", "private, no-cache")
	return c.SendString(t.WebVTTContent)
}

// DeleteTrack handles DELETE /media_objects/:media_id/media_tracks/:track_id.
func (h *MediaObjectHandler) DeleteTrack(c *fiber.Ctx) error {
	m, wrote, err := h.loadMedia(c)
	if wrote || err != nil {
		return err
	}
	if !h.canEdit(c, m) {
		return mediaForbidden()
	}
	trackID, err := c.ParamsInt("track_id")
	if err != nil || trackID <= 0 {
		return responses.NotFound(c, "media track")
	}
	if err := h.svc.DeleteTrack(c.Context(), m, uint(trackID)); err != nil {
		return responses.NotFound(c, "media track")
	}
	return c.JSON(fiber.Map{"delete": true})
}
