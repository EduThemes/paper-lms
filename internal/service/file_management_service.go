package service

import (
	"context"
	"crypto/md5" // #nosec G501 -- content checksum, not a security control
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// SetTreeRepository enables the file-tree features (folder-scoped and
// learner-filtered listings, moves, quota usage).
func (s *FileService) SetTreeRepository(tree repository.CourseFilesRepository) *FileService {
	s.tree = tree
	return s
}

// HasTree reports whether the file-tree repository is wired.
func (s *FileService) HasTree() bool { return s.tree != nil }

// FindRootFolder returns the context's root folder without creating it.
func (s *FileService) FindRootFolder(ctx context.Context, contextType string, contextID uint) (*models.Folder, error) {
	return s.folderRepo.FindRootFolder(ctx, contextType, contextID)
}

func (s *FileService) ListTreeFiles(ctx context.Context, q repository.FileTreeQuery, params repository.PaginationParams) (*repository.PaginatedResult[models.Attachment], error) {
	return s.tree.ListFiles(ctx, q, params)
}

func (s *FileService) ListTreeFolders(ctx context.Context, q repository.FileTreeQuery, params repository.PaginationParams) (*repository.PaginatedResult[models.Folder], error) {
	return s.tree.ListFolders(ctx, q, params)
}

// FolderChain returns the folder and its ancestors, nearest first.
func (s *FileService) FolderChain(ctx context.Context, folderID uint) ([]models.Folder, error) {
	return s.tree.FolderChain(ctx, folderID)
}

// FolderBlockedForLearner reports whether a learner is kept out of a
// folder: it, or a folder above it, is deleted, locked or hidden. When
// lockedOnly is set, hidden folders don't count (a hidden folder's files
// stay readable by link).
func (s *FileService) FolderBlockedForLearner(ctx context.Context, folderID uint, lockedOnly bool) (bool, error) {
	chain, err := s.tree.FolderChain(ctx, folderID)
	if err != nil {
		return true, err
	}
	if len(chain) == 0 {
		return true, nil
	}
	for _, f := range chain {
		if f.WorkflowState == "deleted" || f.Locked || (!lockedOnly && f.Hidden) {
			return true, nil
		}
	}
	return false, nil
}

// FolderIsWithin reports whether folderID is ancestorID or sits below it.
func (s *FileService) FolderIsWithin(ctx context.Context, folderID, ancestorID uint) (bool, error) {
	chain, err := s.tree.FolderChain(ctx, folderID)
	if err != nil {
		return false, err
	}
	for _, f := range chain {
		if f.ID == ancestorID {
			return true, nil
		}
	}
	return false, nil
}

// SaveFolder saves a renamed or moved folder and refreshes the full_name
// of everything below it.
func (s *FileService) SaveFolder(ctx context.Context, folder *models.Folder) error {
	if err := s.folderRepo.Update(ctx, folder); err != nil {
		return err
	}
	if s.tree != nil {
		return s.tree.RefreshFolderPaths(ctx, folder.ID)
	}
	return nil
}

// DeleteFolderTree soft-deletes a folder with its subfolders and files.
func (s *FileService) DeleteFolderTree(ctx context.Context, folderID uint) error {
	if s.tree == nil {
		return s.folderRepo.Delete(ctx, folderID)
	}
	return s.tree.DeleteFolderTree(ctx, folderID)
}

// UpdateAttachment saves attachment metadata (name, folder, visibility,
// usage rights).
func (s *FileService) UpdateAttachment(ctx context.Context, attachment *models.Attachment) error {
	return s.attachmentRepo.Update(ctx, attachment)
}

// NameTaken reports a display-name clash in the target folder.
func (s *FileService) NameTaken(ctx context.Context, a *models.Attachment, folderID *uint, name string) (bool, error) {
	if s.tree == nil {
		return false, nil
	}
	return s.tree.NameTaken(ctx, a.ContextType, a.ContextID, folderID, name, a.ID)
}

// ContextUsage returns the bytes used by a context's live files.
func (s *FileService) ContextUsage(ctx context.Context, contextType string, contextID uint) (int64, error) {
	if s.tree == nil {
		return 0, nil
	}
	return s.tree.ContextUsage(ctx, contextType, contextID)
}

// Storage quotas (course G072, account-wide G120) live in one place,
// the storage_quotas table: see StorageQuotaService.

// ReplaceContent swaps the stored bytes of an existing attachment for a
// new upload (G071 "replace"). The attachment keeps its id, so every
// link and embed keeps working; its display name is kept too. The new
// upload goes through the same validation as any user upload.
func (s *FileService) ReplaceContent(ctx context.Context, attachment *models.Attachment, filename, contentType string, size int64, data io.Reader) error {
	filename = sanitizeFilename(filename)
	if err := s.ValidateUploadWithMIME(filename, size, contentType); err != nil {
		return err
	}
	contentType = RecordedContentType(contentType)
	key := filepath.Join(attachment.ContextType, fmt.Sprintf("%d", attachment.ContextID), uuid.New().String(), filename)
	hash := md5.New() // #nosec G401 -- content checksum, not a security control
	if err := s.storageBackend.Put(ctx, key, io.TeeReader(data, hash), contentType); err != nil {
		return fmt.Errorf("could not store file: %w", err)
	}
	oldKey := attachment.StoragePath
	attachment.Filename = filename
	attachment.ContentType = contentType
	attachment.Size = size
	attachment.MD5 = hex.EncodeToString(hash.Sum(nil))
	attachment.StoragePath = key
	if err := s.attachmentRepo.Update(ctx, attachment); err != nil {
		_ = s.storageBackend.Delete(ctx, key)
		return err
	}
	if oldKey != "" && oldKey != key {
		if err := s.storageBackend.Delete(ctx, oldKey); err != nil {
			slog.Warn("files: delete replaced blob", "attachment_id", attachment.ID, "error", err)
		}
	}
	return nil
}
