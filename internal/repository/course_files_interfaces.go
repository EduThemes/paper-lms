package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// FolderScope selects which part of a context's file tree a listing
// covers.
type FolderScope int

const (
	// FolderScopeAll lists everything in the context (flat).
	FolderScopeAll FolderScope = iota
	// FolderScopeRoot lists what sits directly in the root folder
	// (files with no folder count as root files).
	FolderScopeRoot
	// FolderScopeFolder lists what sits directly in FolderID.
	FolderScopeFolder
)

// FileTreeQuery filters file and folder listings of one context.
type FileTreeQuery struct {
	ContextType string
	ContextID   uint
	Scope       FolderScope
	// FolderID is the folder for FolderScopeFolder.
	FolderID uint
	// RootFolderID is the context's root folder (0 if none yet); used by
	// FolderScopeRoot.
	RootFolderID uint
	// LearnerView hides what learners may not browse: locked
	// (unpublished) and hidden files, and anything under a locked or
	// hidden folder.
	LearnerView bool
}

// CourseFilesRepository holds the file-tree queries behind file
// management, visibility and storage quotas (G070-G072). Kept apart from
// AttachmentRepository / FolderRepository so their mocks don't change.
type CourseFilesRepository interface {
	ListFiles(ctx context.Context, q FileTreeQuery, params PaginationParams) (*PaginatedResult[models.Attachment], error)
	ListFolders(ctx context.Context, q FileTreeQuery, params PaginationParams) (*PaginatedResult[models.Folder], error)
	// FolderChain returns the folder followed by its ancestors, nearest
	// first. Deleted folders are included so callers can refuse them.
	FolderChain(ctx context.Context, folderID uint) ([]models.Folder, error)
	// ContextUsage sums the size of the context's live files, in bytes.
	ContextUsage(ctx context.Context, contextType string, contextID uint) (int64, error)
	// RefreshFolderPaths recomputes full_name for every folder below
	// folderID from the folder's own (already saved) full_name.
	RefreshFolderPaths(ctx context.Context, folderID uint) error
	// DeleteFolderTree soft-deletes the folder, its subfolders and every
	// file inside them.
	DeleteFolderTree(ctx context.Context, folderID uint) error
	// NameTaken reports whether a live file other than excludeID already
	// has displayName in the same folder (nil folder = root).
	NameTaken(ctx context.Context, contextType string, contextID uint, folderID *uint, displayName string, excludeID uint) (bool, error)
}
