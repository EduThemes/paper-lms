package service

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

// Media errors the handler maps to 400s.
var (
	ErrMediaUnsupported  = errors.New("unsupported media format: upload an audio or video file")
	ErrMediaTrackInvalid = errors.New("invalid caption track")
)

// MaxMediaTrackBytes caps an uploaded caption file (Canvas has no explicit
// cap; a feature-length SRT is well under 1 MB).
const MaxMediaTrackBytes = 1 << 20

// mediaExtensions maps the audio/video types the editor accepts to the file
// extension the attachment is stored under (the file service checks the
// extension against the type).
var mediaExtensions = map[string]string{
	"video/mp4":       ".mp4",
	"video/webm":      ".webm",
	"video/ogg":       ".ogv",
	"video/quicktime": ".mov",
	"video/x-m4v":     ".m4v",
	"audio/mpeg":      ".mp3",
	"audio/mp4":       ".m4a",
	"audio/x-m4a":     ".m4a",
	"audio/m4a":       ".m4a",
	"audio/webm":      ".webm",
	"audio/ogg":       ".ogg",
	"audio/wav":       ".wav",
	"audio/wave":      ".wav",
	"audio/x-wav":     ".wav",
	"audio/aac":       ".aac",
	"audio/x-aac":     ".aac",
}

// MediaTrackKinds are the <track kind> values Canvas accepts.
var MediaTrackKinds = map[string]bool{
	"subtitles": true, "captions": true, "descriptions": true, "chapters": true, "metadata": true,
}

var (
	mediaTrackLocale  = regexp.MustCompile(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)
	looksLikeTTML     = regexp.MustCompile(`(?i)<tt\s+xml`)
	srtShortHour      = regexp.MustCompile(`(?m)(^|\s)(\d):(\d{2}):(\d{2})`)
	srtCommaMillis    = regexp.MustCompile(`(\d{2}:\d{2}:\d{2}),(\d{3})`)
	vttCueTimingArrow = regexp.MustCompile(`\d{2}:\d{2}(:\d{2})?\.\d{3}\s+-->\s+`)
)

// MediaService stores rich-content audio/video and its caption tracks.
type MediaService struct {
	files  *FileService
	media  repository.MediaObjectRepository
	tracks repository.MediaTrackRepository
}

// NewMediaService wires the media service.
func NewMediaService(files *FileService, media repository.MediaObjectRepository, tracks repository.MediaTrackRepository) *MediaService {
	return &MediaService{files: files, media: media, tracks: tracks}
}

// Files exposes the file service (the stream handler needs the backend).
func (s *MediaService) Files() *FileService { return s.files }

// UploadMediaInput is one upload from the editor (file picker or recorder).
type UploadMediaInput struct {
	AccountID   uint
	UserID      uint
	ContextType string // "Course" or "User"
	ContextID   uint
	Title       string
	Filename    string
	ContentType string // as claimed by the client
	Size        int64
	Data        io.Reader
}

// baseMediaType lowercases a Content-Type and drops its parameters
// ("audio/webm;codecs=opus" -> "audio/webm").
func baseMediaType(ct string) string {
	if mt, _, err := mime.ParseMediaType(ct); err == nil {
		return strings.ToLower(mt)
	}
	return strings.ToLower(strings.TrimSpace(strings.SplitN(ct, ";", 2)[0]))
}

// resolveMediaType decides the stored content type from the client's claim
// and the sniffed bytes. The bytes must not positively identify as anything
// but audio/video (an HTML or image file renamed .mp4 is refused); bytes the
// sniffer cannot classify keep the claim (QuickTime, AAC, M4A variants).
func resolveMediaType(claimed string, head []byte) (string, error) {
	claim := baseMediaType(claimed)
	sniffed := baseMediaType(http.DetectContentType(head))
	switch {
	case strings.HasPrefix(sniffed, "audio/"), strings.HasPrefix(sniffed, "video/"), sniffed == "application/ogg":
		if _, ok := mediaExtensions[claim]; ok {
			return claim, nil
		}
		if sniffed == "application/ogg" {
			return "audio/ogg", nil
		}
		if _, ok := mediaExtensions[sniffed]; ok {
			return sniffed, nil
		}
		return "", ErrMediaUnsupported
	case sniffed == "application/octet-stream":
		if _, ok := mediaExtensions[claim]; ok {
			return claim, nil
		}
	}
	return "", ErrMediaUnsupported
}

// truncateUTF8 cuts s to at most maxBytes bytes without splitting a
// multi-byte character: a byte slice that ends mid-rune is invalid UTF-8,
// which Postgres refuses, so a long non-Latin title failed the upload.
func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	i := maxBytes
	for i > 0 && !utf8.RuneStart(s[i]) {
		i--
	}
	return s[:i]
}

func newMediaID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "m-" + hex.EncodeToString(b), nil
}

// CoursesTaughtToViewer lists courses (up to limit) where the media's
// owner teaches and viewerID is enrolled; see the repository method.
func (s *MediaService) CoursesTaughtToViewer(ctx context.Context, mo *models.MediaObject, viewerID uint, limit int) ([]uint, error) {
	return s.media.CoursesTaughtToViewer(ctx, mo.UserID, viewerID, mo.AccountID, limit)
}

// Upload stores the file and creates its media object.
func (s *MediaService) Upload(ctx context.Context, in UploadMediaInput) (*models.MediaObject, error) {
	br := bufio.NewReaderSize(in.Data, 512)
	head, _ := br.Peek(512)
	contentType, err := resolveMediaType(in.ContentType, head)
	if err != nil {
		return nil, err
	}
	ext := mediaExtensions[contentType]
	base := strings.TrimSuffix(filepath.Base(in.Filename), filepath.Ext(in.Filename))
	if base == "" || base == "." || base == "/" {
		base = "media"
	}
	filename := base + ext

	title := strings.TrimSpace(SanitizePlainText(in.Title))
	if title == "" {
		title = base
	}
	title = truncateUTF8(title, 255)

	attachment := &models.Attachment{
		ContextType: "User",
		ContextID:   in.UserID,
		UserID:      in.UserID,
		DisplayName: filename,
		Filename:    filename,
		ContentType: contentType,
		Size:        in.Size,
	}
	if err := s.files.UploadFile(ctx, attachment, br); err != nil {
		return nil, fmt.Errorf("store media: %w", err)
	}

	mediaID, err := newMediaID()
	if err != nil {
		return nil, err
	}
	mediaType := "video"
	if strings.HasPrefix(contentType, "audio/") {
		mediaType = "audio"
	}
	ctxType, ctxID := in.ContextType, in.ContextID
	if ctxType != "Course" {
		ctxType, ctxID = "User", in.UserID
	}
	mo := &models.MediaObject{
		AccountID:     in.AccountID,
		UserID:        in.UserID,
		AttachmentID:  attachment.ID,
		MediaID:       mediaID,
		ContextType:   ctxType,
		ContextID:     ctxID,
		MediaType:     mediaType,
		ContentType:   contentType,
		Title:         title,
		WorkflowState: models.MediaObjectActive,
	}
	if err := s.media.Create(ctx, mo); err != nil {
		return nil, err
	}
	return mo, nil
}

// Get loads an active media object of the tenant.
func (s *MediaService) Get(ctx context.Context, mediaID string, accountID uint) (*models.MediaObject, error) {
	return s.media.FindByMediaID(ctx, mediaID, accountID)
}

// ListByUser lists the user's own media.
func (s *MediaService) ListByUser(ctx context.Context, userID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.MediaObject], error) {
	return s.media.ListByUser(ctx, userID, accountID, params)
}

// Rename sets the media title (plain text).
func (s *MediaService) Rename(ctx context.Context, mo *models.MediaObject, title string) error {
	title = strings.TrimSpace(SanitizePlainText(title))
	if title == "" {
		return nil
	}
	title = truncateUTF8(title, 255)
	mo.Title = title
	return s.media.Update(ctx, mo)
}

// Delete soft-deletes the media object; embeds stop playing.
func (s *MediaService) Delete(ctx context.Context, mo *models.MediaObject) error {
	mo.WorkflowState = models.MediaObjectDeleted
	return s.media.Update(ctx, mo)
}

// Attachment loads the media's stored file.
func (s *MediaService) Attachment(ctx context.Context, mo *models.MediaObject) (*models.Attachment, error) {
	return s.files.GetAttachment(ctx, mo.AttachmentID, 0) // tenant already checked on the media object
}

// ListTracks lists the media's caption tracks.
func (s *MediaService) ListTracks(ctx context.Context, mo *models.MediaObject) ([]models.MediaTrack, error) {
	return s.tracks.ListByMediaObject(ctx, mo.ID, mo.AccountID)
}

// GetTrack loads one track of the media object (404 when it belongs to a
// different media object).
func (s *MediaService) GetTrack(ctx context.Context, mo *models.MediaObject, trackID uint) (*models.MediaTrack, error) {
	t, err := s.tracks.FindByID(ctx, trackID, mo.AccountID)
	if err != nil {
		return nil, err
	}
	if t.MediaObjectID != mo.ID {
		return nil, gorm.ErrRecordNotFound
	}
	return t, nil
}

// SaveTrackInput is one caption upload.
type SaveTrackInput struct {
	Locale  string
	Kind    string
	Label   string
	Content string
}

// SaveTrack validates and stores a caption track, replacing the media's
// existing track for the same locale.
func (s *MediaService) SaveTrack(ctx context.Context, mo *models.MediaObject, userID uint, in SaveTrackInput) (*models.MediaTrack, error) {
	locale := strings.TrimSpace(in.Locale)
	if !mediaTrackLocale.MatchString(locale) {
		return nil, fmt.Errorf("%w: locale must be a language code such as en or es-MX", ErrMediaTrackInvalid)
	}
	kind := strings.TrimSpace(in.Kind)
	if kind == "" {
		kind = "subtitles"
	}
	if !MediaTrackKinds[kind] {
		return nil, fmt.Errorf("%w: kind must be subtitles, captions, descriptions, chapters or metadata", ErrMediaTrackInvalid)
	}
	webvtt, err := ToWebVTT(in.Content)
	if err != nil {
		return nil, err
	}
	label := strings.TrimSpace(SanitizePlainText(in.Label))
	if label == "" {
		label = locale
	}
	label = truncateUTF8(label, 100)
	t := &models.MediaTrack{
		MediaObjectID: mo.ID,
		AccountID:     mo.AccountID,
		UserID:        userID,
		Kind:          kind,
		Locale:        locale,
		Label:         label,
		Content:       in.Content,
		WebVTTContent: webvtt,
		UpdatedAt:     time.Now(),
	}
	if err := s.tracks.Upsert(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// DeleteTrack removes one track of the media object.
func (s *MediaService) DeleteTrack(ctx context.Context, mo *models.MediaObject, trackID uint) error {
	if _, err := s.GetTrack(ctx, mo, trackID); err != nil {
		return err
	}
	return s.tracks.Delete(ctx, trackID, mo.AccountID)
}

// ToWebVTT validates caption content and returns it as WebVTT, converting
// SubRip (.srt) the way Canvas does: zero-pad single-digit hours, use a dot
// before the milliseconds, and prepend the WEBVTT header. TTML is refused
// (Canvas refuses it as an XSS vector). The result must contain at least one
// cue timing line.
func ToWebVTT(content string) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("%w: the caption file is empty", ErrMediaTrackInvalid)
	}
	if len(content) > MaxMediaTrackBytes {
		return "", fmt.Errorf("%w: caption files are limited to 1 MB", ErrMediaTrackInvalid)
	}
	if looksLikeTTML.MatchString(content) {
		return "", fmt.Errorf("%w: TTML caption files are not supported; upload WebVTT or SRT", ErrMediaTrackInvalid)
	}
	text := strings.TrimPrefix(content, "\ufeff")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if !strings.HasPrefix(strings.TrimSpace(text), "WEBVTT") {
		text = srtShortHour.ReplaceAllString(text, "${1}0$2:$3:$4")
		text = srtCommaMillis.ReplaceAllString(text, "$1.$2")
		text = "WEBVTT\n\n" + strings.TrimSpace(text) + "\n"
	}
	if !vttCueTimingArrow.MatchString(text) {
		return "", fmt.Errorf("%w: no cues found; upload a WebVTT (.vtt) or SubRip (.srt) file", ErrMediaTrackInvalid)
	}
	return text, nil
}
