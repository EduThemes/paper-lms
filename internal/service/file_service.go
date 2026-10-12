package service

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/settingsctx"
	"github.com/EduThemes/paper-lms/internal/storage"
	"github.com/google/uuid"
)

// MaxUploadSize is the default maximum file upload size (100 MB).
const MaxUploadSize int64 = 100 * 1024 * 1024

// allowedMIMETypes maps file extensions to permitted MIME type prefixes.
// Files whose detected MIME type does not match will be rejected.
var allowedMIMETypes = map[string][]string{
	".pdf":  {"application/pdf"},
	".doc":  {"application/msword", "application/vnd.openxmlformats"},
	".docx": {"application/vnd.openxmlformats", "application/msword"},
	".xls":  {"application/vnd.ms-excel", "application/vnd.openxmlformats"},
	".xlsx": {"application/vnd.openxmlformats", "application/vnd.ms-excel"},
	".ppt":  {"application/vnd.ms-powerpoint", "application/vnd.openxmlformats"},
	".pptx": {"application/vnd.openxmlformats", "application/vnd.ms-powerpoint"},
	".txt":  {"text/plain"},
	".csv":  {"text/csv", "text/plain", "application/csv"},
	".html": {"text/html"},
	".htm":  {"text/html"},
	".png":  {"image/png"},
	".jpg":  {"image/jpeg"},
	".jpeg": {"image/jpeg"},
	".gif":  {"image/gif"},
	".svg":  {"image/svg+xml"},
	".webp": {"image/webp"},
	".mp4":  {"video/mp4", "audio/mp4"},
	".webm": {"video/webm", "audio/webm"},
	".mp3":  {"audio/mpeg"},
	".wav":  {"audio/wav", "audio/x-wav"},
	".ogg":  {"audio/ogg", "video/ogg"},
	// Media recorded or uploaded through the rich content editor (G002).
	".m4a": {"audio/mp4", "audio/x-m4a", "audio/m4a"},
	".m4v": {"video/mp4", "video/x-m4v"},
	".mov": {"video/quicktime"},
	".ogv": {"video/ogg"},
	".oga": {"audio/ogg"},
	".aac": {"audio/aac", "audio/x-aac"},
	".zip":  {"application/zip", "application/x-zip"},
	".json": {"application/json", "text/plain"},
	".xml":  {"application/xml", "text/xml"},
	".md":   {"text/markdown", "text/plain"},
}

// blockedExtensions are never allowed, regardless of MIME type.
var blockedExtensions = map[string]bool{
	".exe": true, ".bat": true, ".cmd": true, ".com": true,
	".msi": true, ".scr": true, ".pif": true, ".vbs": true,
	".js": true, ".wsh": true, ".wsf": true, ".ps1": true,
	".sh": true, ".cgi": true, ".dll": true, ".sys": true,
	".svg": true, ".html": true, ".htm": true, // XSS vectors: SVG can embed scripts, HTML executes arbitrary JS
}

type FileService struct {
	folderRepo     repository.FolderRepository
	attachmentRepo repository.AttachmentRepository
	storageBackend storage.Backend
	storagePath    string // kept for backward compat (local path prefix)
	maxUploadSize  int64
	// tree backs file management, visibility and usage (G070-G071);
	// set with SetTreeRepository. Nil keeps the older flat behaviour.
	tree repository.CourseFilesRepository
	// quota enforces storage quotas on user uploads (G120). Optional.
	quota UploadQuotaChecker
}

// UploadQuotaChecker refuses uploads that would exceed a context's storage
// quota (*StorageQuotaService).
type UploadQuotaChecker interface {
	CheckQuota(ctx context.Context, contextType string, contextID uint, addBytes int64) error
}

// SetQuotaChecker enables storage-quota enforcement on user uploads.
func (s *FileService) SetQuotaChecker(q UploadQuotaChecker) *FileService {
	s.quota = q
	return s
}

func NewFileService(folderRepo repository.FolderRepository, attachmentRepo repository.AttachmentRepository, storagePath string) *FileService {
	return &FileService{
		folderRepo:     folderRepo,
		attachmentRepo: attachmentRepo,
		storageBackend: storage.NewLocalBackend(storagePath),
		storagePath:    storagePath,
		maxUploadSize:  MaxUploadSize,
	}
}

// NewFileServiceWithBackend creates a FileService with a custom storage backend.
func NewFileServiceWithBackend(folderRepo repository.FolderRepository, attachmentRepo repository.AttachmentRepository, backend storage.Backend) *FileService {
	return &FileService{
		folderRepo:     folderRepo,
		attachmentRepo: attachmentRepo,
		storageBackend: backend,
		maxUploadSize:  MaxUploadSize,
	}
}

// StorageBackend returns the underlying storage backend (for handlers that need
// to stream files directly, e.g., download handler).
func (s *FileService) StorageBackend() storage.Backend {
	return s.storageBackend
}

// ValidateUpload checks that the file meets size, extension, and MIME type
// requirements before any data is written to disk.
func (s *FileService) ValidateUpload(filename string, sizeBytes int64) error {
	if sizeBytes > s.maxUploadSize {
		return fmt.Errorf("file size %d bytes exceeds maximum of %d bytes", sizeBytes, s.maxUploadSize)
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if blockedExtensions[ext] {
		return fmt.Errorf("file type %s is not allowed", ext)
	}
	// Unknown extensions are allowed through unless blocked above (e.g.,
	// custom file types for assignments). Known extensions have their
	// Content-Type checked against allowedMIMETypes in ValidateUploadWithMIME.
	return nil
}

// ValidateUploadWithMIME checks extension, size, and verifies the Content-Type
// matches the expected MIME type for the file extension.
func (s *FileService) ValidateUploadWithMIME(filename string, sizeBytes int64, contentType string) error {
	if err := s.ValidateUpload(filename, sizeBytes); err != nil {
		return err
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if allowedTypes, ok := allowedMIMETypes[ext]; ok && contentType != "" {
		matched := false
		for _, prefix := range allowedTypes {
			if strings.HasPrefix(contentType, prefix) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("content type %s does not match expected type for %s files", contentType, ext)
		}
	}
	return nil
}

// Folder operations

func (s *FileService) CreateFolder(ctx context.Context, folder *models.Folder) error {
	if folder.Name == "" {
		return errors.New("folder name is required")
	}
	if folder.WorkflowState == "" {
		folder.WorkflowState = "visible"
	}
	return s.folderRepo.Create(ctx, folder)
}

// GetFolder retrieves a folder, scoped to the caller's tenant.
// accountID==0 disables the tenant scope (background jobs, IMSCC import).
func (s *FileService) GetFolder(ctx context.Context, id, accountID uint) (*models.Folder, error) {
	return s.folderRepo.FindByID(ctx, id, accountID)
}

func (s *FileService) UpdateFolder(ctx context.Context, folder *models.Folder) error {
	return s.folderRepo.Update(ctx, folder)
}

func (s *FileService) DeleteFolder(ctx context.Context, id uint) error {
	return s.folderRepo.Delete(ctx, id)
}

func (s *FileService) ListFolders(ctx context.Context, contextType string, contextID uint, parentFolderID *uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Folder], error) {
	return s.folderRepo.ListByContext(ctx, contextType, contextID, parentFolderID, params)
}

func (s *FileService) ListSubfolders(ctx context.Context, folderID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Folder], error) {
	folder, err := s.folderRepo.FindByID(ctx, folderID, accountID)
	if err != nil {
		return nil, err
	}
	return s.folderRepo.ListByContext(ctx, folder.ContextType, folder.ContextID, &folderID, params)
}

func (s *FileService) GetOrCreateRootFolder(ctx context.Context, contextType string, contextID uint) (*models.Folder, error) {
	folder, err := s.folderRepo.FindRootFolder(ctx, contextType, contextID)
	if err == nil {
		return folder, nil
	}

	name := "course files"
	if contextType == "User" {
		name = "my files"
	}

	root := &models.Folder{
		ContextType:   contextType,
		ContextID:     contextID,
		Name:          name,
		FullName:      name,
		Position:      0,
		WorkflowState: "visible",
	}

	if err := s.folderRepo.Create(ctx, root); err != nil {
		return nil, err
	}

	return root, nil
}

// Attachment operations

// sanitizeFilename removes path traversal components and dangerous characters from a filename.
func sanitizeFilename(name string) string {
	// Extract only the base filename, stripping any directory components
	name = filepath.Base(name)
	// Reject hidden files and directory traversal
	if name == "." || name == ".." || name == "" {
		name = "upload"
	}
	// Replace null bytes and path separators that might have survived
	cleaned := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		if name[i] != 0 && name[i] != '/' && name[i] != '\\' {
			cleaned = append(cleaned, name[i])
		}
	}
	if len(cleaned) == 0 {
		return "upload"
	}
	return string(cleaned)
}

func (s *FileService) UploadFile(ctx context.Context, attachment *models.Attachment, fileData io.Reader) error {
	return s.uploadFile(ctx, attachment, fileData, false)
}

// UploadFileTrusted uploads a file from a trusted source (currently: IMSCC
// content imports), bypassing the user-upload extension blocklist (which
// rejects SVG/HTML for XSS reasons). Course content imports legitimately
// contain SVG diagrams and stay scoped to instructors, so the blocklist
// would silently strip half of a math-heavy cartridge.
func (s *FileService) UploadFileTrusted(ctx context.Context, attachment *models.Attachment, fileData io.Reader) error {
	return s.uploadFile(ctx, attachment, fileData, true)
}

func (s *FileService) uploadFile(ctx context.Context, attachment *models.Attachment, fileData io.Reader, trusted bool) error {
	// Sanitize filename to prevent path traversal attacks
	attachment.Filename = sanitizeFilename(attachment.Filename)
	attachment.DisplayName = sanitizeFilename(attachment.DisplayName)

	// Validate file extension, size, and MIME type. For trusted imports we
	// still enforce the size cap and MIME alignment — only the blocklist is
	// skipped.
	if trusted {
		if attachment.Size > s.maxUploadSize {
			return fmt.Errorf("file size %d bytes exceeds maximum of %d bytes", attachment.Size, s.maxUploadSize)
		}
	} else if err := s.ValidateUploadWithMIME(attachment.Filename, attachment.Size, attachment.ContentType); err != nil {
		return err
	}
	// Storage quota (G120): user uploads only; trusted course imports are
	// an instructor's own content migration and are not refused midway.
	if !trusted && s.quota != nil {
		if err := s.quota.CheckQuota(ctx, attachment.ContextType, attachment.ContextID, attachment.Size); err != nil {
			return err
		}
	}

	// Build storage key: ContextType/ContextID/UUID/filename
	fileUUID := uuid.New().String()
	storageKey := filepath.Join(attachment.ContextType, fmt.Sprintf("%d", attachment.ContextID), fileUUID, attachment.Filename)

	// Hash the file while uploading
	hash := md5.New()
	tee := io.TeeReader(fileData, hash)

	// Wave 9: per-tenant S3 config resolution. Callers that have a caller
	// accountID in scope (e.g. UploadCourseFile handler) stamp it via
	// settingsctx.WithAccountID before calling here so the S3 backend's
	// Wave 6 lookup walks account → parent → instance → env. Trusted
	// imports (IMSCC parser) currently don't have a clear caller account —
	// FOLLOW-UP (Wave 10): widen UploadFileTrusted to accept accountID and
	// thread from imscc_parser caller context.
	if err := s.storageBackend.Put(ctx, storageKey, tee, attachment.ContentType); err != nil {
		return fmt.Errorf("could not store file: %w", err)
	}

	attachment.MD5 = hex.EncodeToString(hash.Sum(nil))
	attachment.StoragePath = storageKey

	if attachment.WorkflowState == "" {
		attachment.WorkflowState = "active"
	}
	if attachment.FileState == "" {
		attachment.FileState = "available"
	}
	if attachment.UploadStatus == "" {
		attachment.UploadStatus = "success"
	}

	return s.attachmentRepo.Create(ctx, attachment)
}

// GetAttachment retrieves an attachment, scoped to the caller's tenant.
// accountID==0 disables the tenant scope (background jobs, IMSCC import).
func (s *FileService) GetAttachment(ctx context.Context, id, accountID uint) (*models.Attachment, error) {
	return s.attachmentRepo.FindByID(ctx, id, accountID)
}

func (s *FileService) DeleteAttachment(ctx context.Context, id uint) error {
	return s.attachmentRepo.Delete(ctx, id)
}

func (s *FileService) ListFilesByContext(ctx context.Context, contextType string, contextID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Attachment], error) {
	return s.attachmentRepo.ListByContext(ctx, contextType, contextID, params)
}

func (s *FileService) ListFilesByFolder(ctx context.Context, folderID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Attachment], error) {
	return s.attachmentRepo.ListByFolderID(ctx, folderID, params)
}

// GetFilePath returns the storage key for the given attachment.
// For local storage, this resolves to a filesystem path.
// For S3, this is the object key used with StorageBackend().Get().
func (s *FileService) GetFilePath(ctx context.Context, id, accountID uint) (string, error) {
	attachment, err := s.attachmentRepo.FindByID(ctx, id, accountID)
	if err != nil {
		return "", err
	}
	return attachment.StoragePath, nil
}

// GetFileURL returns a download URL for the given attachment.
// For local storage, returns the file path. For S3, returns a presigned URL.
func (s *FileService) GetFileURL(ctx context.Context, id, accountID uint) (string, error) {
	attachment, err := s.attachmentRepo.FindByID(ctx, id, accountID)
	if err != nil {
		return "", err
	}
	// Stamp the caller's account onto ctx so the S3 backend resolves
	// storage.s3.* at account scope (Wave 6 lookup closure walks account →
	// parent → instance → env). Enables per-district S3 buckets.
	scopedCtx := settingsctx.WithAccountID(ctx, accountID)
	return s.storageBackend.URL(scopedCtx, attachment.StoragePath)
}
