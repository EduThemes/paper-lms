package service

// Course content exports (G066/G142): Canvas's ContentExport for the
// common_cartridge type. A request creates a content_exports row; the
// worker builds the .imscc package with IMSCCExporter, stores it through
// the file storage backend (local disk or S3) and records its key, and
// the download endpoint streams it back.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/storage"
)

// ErrContentExportNotReady is returned when a download is requested for
// an export that has not finished.
var ErrContentExportNotReady = errors.New("content export is not ready")

// ErrInvalidExportType is returned for an unsupported export_type.
var ErrInvalidExportType = errors.New("invalid export_type")

// staleJobAfter is how long a claimed export/migration may run before the
// sweeper treats its worker as dead.
const staleJobAfter = 2 * time.Hour

// ContentExportService creates, runs and serves course content exports.
type ContentExportService struct {
	repo     repository.ContentExportRepository
	exporter *IMSCCExporter
	backend  storage.Backend
	tmpDir   string
	dispatch func(func())
}

// NewContentExportService wires the export pipeline. tmpDir is where the
// package is assembled before it is handed to the storage backend ("" =
// the OS temp dir).
func NewContentExportService(repo repository.ContentExportRepository, exporter *IMSCCExporter, backend storage.Backend, tmpDir string) *ContentExportService {
	return &ContentExportService{
		repo:     repo,
		exporter: exporter,
		backend:  backend,
		tmpDir:   tmpDir,
		dispatch: func(f func()) { go f() },
	}
}

// SetDispatcher replaces how a new export is started (tests run it
// inline). The default runs it on its own goroutine.
func (s *ContentExportService) SetDispatcher(fn func(func())) {
	if fn != nil {
		s.dispatch = fn
	}
}

// Create records an export request for the course and starts it.
func (s *ContentExportService) Create(ctx context.Context, courseID, userID uint, exportType string, settings *models.MigrationSettings) (*models.ContentExport, error) {
	if courseID == 0 {
		return nil, errors.New("course_id is required")
	}
	if exportType == "" {
		exportType = models.ContentExportTypeCommonCartridge
	}
	if exportType != models.ContentExportTypeCommonCartridge {
		return nil, ErrInvalidExportType
	}
	selection := "{}"
	if settings != nil {
		b, err := json.Marshal(settings)
		if err != nil {
			return nil, err
		}
		selection = string(b)
	}
	export := &models.ContentExport{
		CourseID:      courseID,
		UserID:        userID,
		ExportType:    exportType,
		WorkflowState: models.ContentExportCreated,
		Selection:     selection,
	}
	if err := s.repo.Create(ctx, export); err != nil {
		return nil, err
	}
	id := export.ID
	s.dispatch(func() {
		if err := s.Run(context.Background(), id); err != nil {
			slog.Warn("content export failed", "content_export_id", id, "err", err)
		}
	})
	return export, nil
}

// Get returns one export, scoped to the caller's tenant.
func (s *ContentExportService) Get(ctx context.Context, id, accountID uint) (*models.ContentExport, error) {
	return s.repo.FindByID(ctx, id, accountID)
}

// List returns a course's exports, newest first.
func (s *ContentExportService) List(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.ContentExport], error) {
	return s.repo.ListByCourseID(ctx, courseID, params)
}

// Run claims a created export and builds it. Running an export another
// worker already claimed is a no-op.
func (s *ContentExportService) Run(ctx context.Context, id uint) error {
	ok, err := s.repo.Claim(ctx, id)
	if err != nil || !ok {
		return err
	}
	// accountID=0: auth-internal — the worker runs an export the request
	// handler already tied to the caller's course and tenant.
	export, err := s.repo.FindByID(ctx, id, 0)
	if err != nil {
		return err
	}
	export.Progress = 10
	_ = s.repo.Update(ctx, export)

	runErr := s.build(ctx, export)
	now := time.Now()
	export.FinishedAt = &now
	if runErr != nil {
		export.WorkflowState = models.ContentExportFailed
		export.ErrorMessage = runErr.Error()
	} else {
		export.WorkflowState = models.ContentExportExported
		export.Progress = 100
	}
	if err := s.repo.Update(ctx, export); err != nil {
		return err
	}
	return runErr
}

func (s *ContentExportService) build(ctx context.Context, export *models.ContentExport) error {
	if s.exporter == nil || s.backend == nil {
		return errors.New("content export is not configured")
	}
	var settings *models.MigrationSettings
	if export.Selection != "" && export.Selection != "{}" {
		var ms models.MigrationSettings
		if err := json.Unmarshal([]byte(export.Selection), &ms); err == nil {
			settings = &ms
		}
	}

	workDir, err := os.MkdirTemp(s.tmpDir, "content-export-")
	if err != nil {
		return fmt.Errorf("could not create work directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(workDir) }()

	zipPath, result, err := s.exporter.ExportCourseSelected(ctx, export.CourseID, workDir, settings)
	if err != nil {
		return fmt.Errorf("export failed: %w", err)
	}
	if b, err := json.Marshal(result); err == nil {
		export.Results = string(b)
	}
	export.Progress = 80
	_ = s.repo.Update(ctx, export)

	f, err := os.Open(zipPath)
	if err != nil {
		return fmt.Errorf("could not open package: %w", err)
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	name := filepath.Base(zipPath)
	key := path.Join("content_exports", fmt.Sprintf("%d", export.CourseID), uuid.New().String(), name)
	if err := s.backend.Put(ctx, key, f, "application/zip"); err != nil {
		return fmt.Errorf("could not store package: %w", err)
	}
	export.StorageKey = key
	export.FileName = name
	export.FileSize = info.Size()
	return nil
}

// Open returns a reader over an exported package.
func (s *ContentExportService) Open(ctx context.Context, export *models.ContentExport) (io.ReadCloser, error) {
	if export == nil || export.WorkflowState != models.ContentExportExported || export.StorageKey == "" {
		return nil, ErrContentExportNotReady
	}
	if s.backend == nil {
		return nil, errors.New("content export is not configured")
	}
	return s.backend.Get(ctx, export.StorageKey)
}

// ProcessPending is the scheduler sweep: it runs exports nobody started
// (e.g. the server restarted between request and run) and fails exports
// whose worker died.
func (s *ContentExportService) ProcessPending(ctx context.Context) error {
	if _, err := s.repo.FailStale(ctx, time.Now().Add(-staleJobAfter)); err != nil {
		return err
	}
	ids, err := s.repo.ListPendingIDs(ctx, time.Now().Add(-time.Minute), 20)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.Run(ctx, id); err != nil {
			slog.Warn("content export failed", "content_export_id", id, "err", err)
		}
	}
	return nil
}
