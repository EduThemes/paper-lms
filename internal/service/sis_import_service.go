package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

// SISImportService imports Canvas-format SIS CSVs (G111). A batch is one
// upload: a single CSV or a zip of CSVs. The files are stored with the
// batch and processed off the request path in Canvas's dependency order
// (change_sis_id, accounts, terms, users, logins, courses, sections,
// enrollments, admins, group_categories, groups, group_memberships,
// user_observers). Every row is confined to the batch's account.
type SISImportService struct {
	batchRepo      repository.SISBatchRepository
	errorRepo      repository.SISBatchErrorRepository
	userRepo       repository.UserRepository
	courseRepo     repository.CourseRepository
	sectionRepo    repository.SectionRepository
	enrollmentRepo repository.EnrollmentRepository
	db             *gorm.DB
	// sectionService (SetSectionService) applies xlists.csv rows.
	sectionService *SectionService

	// observers links observer → student for user_observers.csv. Wired
	// via SetObserverLinker; without it those rows are refused.
	observers SISObserverLinker
	// async processes a batch in a goroutine (production); tests run it
	// inline.
	async bool
}

// SISObserverLinker pairs and unpairs observers (*ObserverService).
type SISObserverLinker interface {
	LinkObserverToStudent(ctx context.Context, observerUserID, studentUserID uint) error
	UnlinkObserver(ctx context.Context, observerUserID, studentUserID uint) error
}

func NewSISImportService(
	batchRepo repository.SISBatchRepository,
	errorRepo repository.SISBatchErrorRepository,
	userRepo repository.UserRepository,
	courseRepo repository.CourseRepository,
	sectionRepo repository.SectionRepository,
	enrollmentRepo repository.EnrollmentRepository,
	db *gorm.DB,
) *SISImportService {
	return &SISImportService{
		batchRepo:      batchRepo,
		errorRepo:      errorRepo,
		userRepo:       userRepo,
		courseRepo:     courseRepo,
		sectionRepo:    sectionRepo,
		enrollmentRepo: enrollmentRepo,
		db:             db,
	}
}

// SetObserverLinker wires user_observers.csv support.
func (s *SISImportService) SetObserverLinker(l SISObserverLinker) *SISImportService {
	s.observers = l
	return s
}

// SetAsync makes StartImport process batches in the background.
func (s *SISImportService) SetAsync(async bool) *SISImportService {
	s.async = async
	return s
}

func (s *SISImportService) CreateBatch(ctx context.Context, accountID uint) (*models.SISBatch, error) {
	batch := &models.SISBatch{
		AccountID:     accountID,
		WorkflowState: "created",
	}
	if err := s.batchRepo.Create(ctx, batch); err != nil {
		return nil, err
	}
	return batch, nil
}

// ErrSISBatchNotFound is returned when a batch does not exist or lives in
// another account than the caller is scoped to (indistinguishable on
// purpose: cross-tenant reads 404).
var ErrSISBatchNotFound = errors.New("sis batch not found")

// GetBatch loads a batch by id within accountID. accountID==0 is the
// internal escape hatch (no tenant filter) used by ProcessImport, which
// only ever receives a batch id it just created.
func (s *SISImportService) GetBatch(ctx context.Context, id, accountID uint) (*models.SISBatch, error) {
	batch, err := s.batchRepo.FindByID(ctx, id)
	if err != nil || batch == nil {
		return nil, ErrSISBatchNotFound
	}
	if accountID != 0 && batch.AccountID != accountID {
		return nil, ErrSISBatchNotFound
	}
	return batch, nil
}

func (s *SISImportService) ListBatches(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.SISBatch], error) {
	return s.batchRepo.ListByAccountID(ctx, accountID, params)
}

// GetBatchErrors lists a batch's row errors after tying the batch to
// accountID (see GetBatch).
func (s *SISImportService) GetBatchErrors(ctx context.Context, batchID, accountID uint) ([]models.SISBatchError, error) {
	if _, err := s.GetBatch(ctx, batchID, accountID); err != nil {
		return nil, err
	}
	return s.errorRepo.ListByBatchID(ctx, batchID)
}

// ListBatchFiles returns the files of a batch of accountID (without their
// content).
func (s *SISImportService) ListBatchFiles(ctx context.Context, batchID, accountID uint) ([]models.SISBatchFile, error) {
	if _, err := s.GetBatch(ctx, batchID, accountID); err != nil {
		return nil, err
	}
	var files []models.SISBatchFile
	err := s.db.WithContext(ctx).Omit("content").Where("sis_batch_id = ?", batchID).Order("id").Find(&files).Error
	return files, err
}

// ---------------------------------------------------------------------------
// Import types and detection
// ---------------------------------------------------------------------------

// sisImportOrder is Canvas's processing order: a row may only reference
// objects from its own file or one earlier in the list.
var sisImportOrder = []string{
	"change_sis_id", "accounts", "terms", "users", "logins", "courses",
	"sections", "xlists", "enrollments", "admins", "group_categories",
	"groups", "group_memberships", "user_observers",
}

// sisTypeSignatures identifies a CSV by its header (Canvas detects the
// importer from the columns). Checked in order; every listed column must
// be present.
var sisTypeSignatures = []struct {
	importType string
	columns    []string
}{
	{"change_sis_id", []string{"old_id", "new_id", "type"}},
	{"user_observers", []string{"observer_id", "student_id"}},
	{"group_memberships", []string{"group_id", "user_id"}},
	{"groups", []string{"group_id", "name"}},
	{"group_categories", []string{"group_category_id", "category_name"}},
	{"xlists", []string{"xlist_course_id", "section_id"}},
	{"enrollments", []string{"course_id", "user_id", "role"}},
	{"enrollments", []string{"section_id", "user_id", "role"}},
	{"admins", []string{"user_id", "account_id", "role"}},
	{"sections", []string{"section_id", "course_id", "name"}},
	{"courses", []string{"course_id", "short_name", "long_name"}},
	{"terms", []string{"term_id", "name"}},
	{"logins", []string{"user_id", "login_id", "existing_user_id"}},
	{"logins", []string{"user_id", "login_id", "existing_paper_user_id"}},
	{"logins", []string{"user_id", "login_id", "existing_canvas_user_id"}},
	{"users", []string{"user_id", "login_id"}},
	{"users", []string{"user_id", "first_name"}},
	{"users", []string{"user_id", "full_name"}},
	{"accounts", []string{"account_id", "parent_account_id", "name"}},
}

// DetectSISImportType returns the import type of a CSV header, or "".
func DetectSISImportType(header []string) string {
	cols := map[string]bool{}
	for _, h := range header {
		cols[strings.TrimSpace(strings.ToLower(strings.TrimPrefix(h, "\ufeff")))] = true
	}
	for _, sig := range sisTypeSignatures {
		ok := true
		for _, c := range sig.columns {
			if !cols[c] {
				ok = false
				break
			}
		}
		if ok {
			return sig.importType
		}
	}
	return ""
}

func validSISImportType(t string) bool {
	for _, k := range sisImportOrder {
		if k == t {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Starting an import
// ---------------------------------------------------------------------------

// SISUpload is one uploaded file (a CSV or a zip of CSVs).
type SISUpload struct {
	FileName string
	Data     []byte
	// ImportType forces the type of a single CSV (legacy import_type
	// form field); "" = detect from the header.
	ImportType string
}

// SISImportOptions are Canvas's sis_imports options.
type SISImportOptions struct {
	BatchMode                bool
	BatchModeTermID          *uint
	DiffingDataSetIdentifier string
	ChangeThreshold          *int
}

// Limits on what one upload may expand to.
const (
	sisMaxFiles          = 100
	sisMaxUncompressedMB = 200
)

// Errors surfaced to handlers.
var (
	ErrSISNoFiles         = errors.New("no CSV files found in the upload")
	ErrSISUnknownCSV      = errors.New("could not tell which SIS import a CSV is from its header")
	ErrSISTooLarge        = errors.New("the upload expands past the import size limit")
	ErrSISBatchModeTerm   = errors.New("batch mode requires batch_mode_term_id naming a term of this account")
	ErrSISChangeThreshold = errors.New("change_threshold must be between 1 and 100")
)

// expandUpload turns uploads into stored batch files: CSVs are kept, zips
// are unpacked (CSV entries only), and each CSV's type is detected.
func expandUpload(uploads []SISUpload) ([]models.SISBatchFile, error) {
	var out []models.SISBatchFile
	var total int64
	addCSV := func(name string, data []byte, forced string) error {
		total += int64(len(data))
		if total > sisMaxUncompressedMB*1024*1024 {
			return ErrSISTooLarge
		}
		t := forced
		if t == "" {
			r := csv.NewReader(bytes.NewReader(data))
			header, err := r.Read()
			if err != nil {
				return fmt.Errorf("%s: %w", name, ErrSISUnknownCSV)
			}
			t = DetectSISImportType(header)
		}
		if !validSISImportType(t) {
			return fmt.Errorf("%s: %w", name, ErrSISUnknownCSV)
		}
		out = append(out, models.SISBatchFile{FileName: name, ImportType: t, Content: data})
		if len(out) > sisMaxFiles {
			return ErrSISTooLarge
		}
		return nil
	}
	for _, u := range uploads {
		isZip := strings.HasSuffix(strings.ToLower(u.FileName), ".zip") || bytes.HasPrefix(u.Data, []byte("PK\x03\x04"))
		if !isZip {
			if err := addCSV(path.Base(u.FileName), u.Data, u.ImportType); err != nil {
				return nil, err
			}
			continue
		}
		zr, err := zip.NewReader(bytes.NewReader(u.Data), int64(len(u.Data)))
		if err != nil {
			return nil, fmt.Errorf("%s: not a valid zip file", u.FileName)
		}
		for _, f := range zr.File {
			if f.FileInfo().IsDir() || !strings.HasSuffix(strings.ToLower(f.Name), ".csv") ||
				strings.HasPrefix(path.Base(f.Name), ".") || strings.Contains(f.Name, "__MACOSX") {
				continue
			}
			if f.UncompressedSize64 > sisMaxUncompressedMB*1024*1024 {
				return nil, ErrSISTooLarge
			}
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("%s: %w", f.Name, err)
			}
			data, err := io.ReadAll(io.LimitReader(rc, sisMaxUncompressedMB*1024*1024+1))
			_ = rc.Close()
			if err != nil {
				return nil, fmt.Errorf("%s: %w", f.Name, err)
			}
			if err := addCSV(path.Base(f.Name), data, ""); err != nil {
				return nil, err
			}
		}
	}
	if len(out) == 0 {
		return nil, ErrSISNoFiles
	}
	return out, nil
}

// StartImport validates and stores an upload as a new batch of accountID
// and processes it (in the background when async is on).
func (s *SISImportService) StartImport(ctx context.Context, accountID, userID uint, opts SISImportOptions, uploads []SISUpload) (*models.SISBatch, error) {
	files, err := expandUpload(uploads)
	if err != nil {
		return nil, err
	}
	if opts.ChangeThreshold != nil && (*opts.ChangeThreshold < 1 || *opts.ChangeThreshold > 100) {
		return nil, ErrSISChangeThreshold
	}
	if opts.BatchMode {
		if opts.BatchModeTermID == nil {
			return nil, ErrSISBatchModeTerm
		}
		var n int64
		s.db.WithContext(ctx).Model(&models.EnrollmentTerm{}).
			Where("id = ? AND account_id = ?", *opts.BatchModeTermID, accountID).Count(&n)
		if n == 0 {
			return nil, ErrSISBatchModeTerm
		}
	}
	batch := &models.SISBatch{
		AccountID:       accountID,
		UserID:          userID,
		WorkflowState:   "created",
		BatchMode:       opts.BatchMode,
		BatchModeTermID: opts.BatchModeTermID,
		ChangeThreshold: opts.ChangeThreshold,
	}
	if id := strings.TrimSpace(opts.DiffingDataSetIdentifier); id != "" {
		batch.DiffingDataSetIdentifier = &id
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(batch).Error; err != nil {
			return err
		}
		for i := range files {
			files[i].SISBatchID = batch.ID
			if err := tx.Create(&files[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if s.async {
		id := batch.ID
		go func() { // #nosec G118 -- the import job outlives the request that queued it
			defer recoverFromPanic("sis import")
			_ = s.RunBatch(context.Background(), id)
		}()
		return batch, nil
	}
	if err := s.RunBatch(ctx, batch.ID); err != nil {
		return nil, err
	}
	return s.GetBatch(ctx, batch.ID, accountID)
}

// ResumePendingBatches processes batches left in "created" (e.g. the
// server restarted before their goroutine ran). Called by the scheduler.
func (s *SISImportService) ResumePendingBatches(ctx context.Context) error {
	var ids []uint
	cutoff := time.Now().Add(-2 * time.Minute)
	if err := s.db.WithContext(ctx).Model(&models.SISBatch{}).
		Where("workflow_state = ? AND created_at < ? AND id IN (SELECT sis_batch_id FROM sis_batch_files)", "created", cutoff).
		Order("id").Limit(20).Pluck("id", &ids).Error; err != nil {
		return err
	}
	for _, id := range ids {
		_ = s.RunBatch(ctx, id)
	}
	return nil
}

// ProcessImport runs one CSV of a known type against an existing batch,
// synchronously (the original single-file entry point, kept for internal
// callers and tests).
func (s *SISImportService) ProcessImport(ctx context.Context, batchID uint, csvType string, reader io.Reader) error {
	batch, err := s.batchRepo.FindByID(ctx, batchID)
	if err != nil {
		return fmt.Errorf("batch not found: %w", err)
	}
	if batch.AccountID == 0 {
		return errors.New("sis batch has no account")
	}
	if !validSISImportType(csvType) {
		return fmt.Errorf("unknown import type: %s", csvType)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	return s.runFiles(ctx, batch, []models.SISBatchFile{{FileName: csvType + ".csv", ImportType: csvType, Content: data}})
}

// RunBatch processes a stored batch.
func (s *SISImportService) RunBatch(ctx context.Context, batchID uint) error {
	batch, err := s.batchRepo.FindByID(ctx, batchID)
	if err != nil || batch == nil {
		return ErrSISBatchNotFound
	}
	if batch.WorkflowState != "created" {
		return nil // already picked up
	}
	// Claim the batch so a concurrent resume can't run it twice.
	res := s.db.WithContext(ctx).Model(&models.SISBatch{}).
		Where("id = ? AND workflow_state = ?", batchID, "created").
		Update("workflow_state", "importing")
	if res.Error != nil || res.RowsAffected == 0 {
		return res.Error
	}
	batch.WorkflowState = "importing"
	var files []models.SISBatchFile
	if err := s.db.WithContext(ctx).Where("sis_batch_id = ?", batchID).Order("id").Find(&files).Error; err != nil {
		return err
	}
	return s.runFiles(ctx, batch, files)
}

// runFiles processes files against batch in dependency order and records
// the outcome on the batch.
func (s *SISImportService) runFiles(ctx context.Context, batch *models.SISBatch, files []models.SISBatchFile) error {
	now := time.Now()
	batch.WorkflowState = "importing"
	batch.StartedAt = &now
	_ = s.batchRepo.Update(ctx, batch)

	run := newSISRun(ctx, s, batch)

	sort.SliceStable(files, func(i, j int) bool {
		return sisOrderIndex(files[i].ImportType) < sisOrderIndex(files[j].ImportType)
	})

	// Parse every file up front so the progress bar has a total.
	parsed := make([]*sisParsedFile, 0, len(files))
	for _, f := range files {
		pf, err := parseSISFile(f)
		if err != nil {
			run.fail(0, f.FileName, err.Error())
			continue
		}
		parsed = append(parsed, pf)
	}

	// Diffing: apply only what changed since the previous batch with the
	// same data set identifier.
	if batch.DiffingDataSetIdentifier != nil {
		if err := run.applyDiffing(parsed); err != nil {
			return s.finish(ctx, run, err)
		}
	}
	for _, pf := range parsed {
		batch.TotalRows += len(pf.rows)
	}

	for _, pf := range parsed {
		run.importFile(pf)
	}

	var runErr error
	if batch.BatchMode && batch.BatchModeTermID != nil {
		runErr = run.applyBatchMode()
	}
	return s.finish(ctx, run, runErr)
}

// finish records the final state of a run.
func (s *SISImportService) finish(ctx context.Context, run *sisRun, runErr error) error {
	batch := run.batch
	ended := time.Now()
	batch.EndedAt = &ended
	batch.Progress = 100
	batch.ProcessedRows = run.processed
	counts, _ := json.Marshal(map[string]interface{}{"counts": run.counts, "files": run.fileNames})
	batch.Data = string(counts)
	switch {
	case errors.Is(runErr, errSISAborted):
		batch.WorkflowState = "aborted"
	case runErr != nil:
		batch.WorkflowState = "failed"
	case run.errors > 0:
		batch.WorkflowState = "imported_with_messages"
	default:
		batch.WorkflowState = "imported"
	}
	// The stored CSVs outlive the run (diffing baseline, file listing):
	// drop the plaintext passwords users.csv / logins.csv carried.
	s.scrubBatchPasswords(ctx, batch.ID)
	if err := s.batchRepo.Update(ctx, batch); err != nil {
		return err
	}
	if errors.Is(runErr, errSISAborted) {
		return nil
	}
	return runErr
}

func sisOrderIndex(t string) int {
	for i, k := range sisImportOrder {
		if k == t {
			return i
		}
	}
	return len(sisImportOrder)
}

// sisParsedFile is a CSV parsed into rows keyed by lower-cased column.
type sisParsedFile struct {
	name       string
	importType string
	header     []string
	rows       []sisRow
}

type sisRow struct {
	line   int // 1-based CSV line (header = 1)
	values map[string]string
}

func (r sisRow) get(col string) string { return strings.TrimSpace(r.values[col]) }

func parseSISFile(f models.SISBatchFile) (*sisParsedFile, error) {
	reader := csv.NewReader(bytes.NewReader(f.Content))
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %v", err)
	}
	cols := make([]string, len(header))
	for i, h := range header {
		cols[i] = strings.TrimSpace(strings.ToLower(strings.TrimPrefix(h, "\ufeff")))
	}
	pf := &sisParsedFile{name: f.FileName, importType: f.ImportType, header: cols}
	line := 1
	for {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}
		line++
		if err != nil {
			pf.rows = append(pf.rows, sisRow{line: line, values: map[string]string{"__error": err.Error()}})
			continue
		}
		vals := make(map[string]string, len(cols))
		for i, c := range cols {
			if i < len(rec) {
				vals[c] = rec[i]
			}
		}
		pf.rows = append(pf.rows, sisRow{line: line, values: vals})
	}
	return pf, nil
}

// ---------------------------------------------------------------------------
// Export methods
// ---------------------------------------------------------------------------

func (s *SISImportService) ExportUsersCSV(ctx context.Context, accountID uint) ([]byte, error) {
	var users []models.User
	if err := s.db.WithContext(ctx).Where("account_id = ?", accountID).Find(&users).Error; err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	_ = writer.Write([]string{"user_id", "login_id", "first_name", "last_name", "email", "status"})

	for _, u := range users {
		sisID := ""
		if u.SISUserID != nil {
			sisID = *u.SISUserID
		}

		firstName, lastName := splitName(u.Name)
		status := "active"
		if u.Suspended {
			status = "suspended"
		}

		_ = writeCSVRow(writer, []string{
			sisID,
			u.LoginID,
			firstName,
			lastName,
			u.Email,
			status,
		})
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (s *SISImportService) ExportCoursesCSV(ctx context.Context, accountID uint) ([]byte, error) {
	var courses []models.Course
	if err := s.db.WithContext(ctx).Where("account_id = ?", accountID).Find(&courses).Error; err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	_ = writer.Write([]string{"course_id", "short_name", "long_name", "account_id", "status"})

	for _, c := range courses {
		sisID := ""
		if c.SISCourseID != nil {
			sisID = *c.SISCourseID
		}

		_ = writeCSVRow(writer, []string{
			sisID,
			c.CourseCode,
			c.Name,
			strconv.FormatUint(uint64(c.AccountID), 10),
			sisCourseStatus(string(c.WorkflowState)),
		})
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (s *SISImportService) ExportSectionsCSV(ctx context.Context, accountID uint) ([]byte, error) {
	var sections []models.CourseSection
	if err := s.db.WithContext(ctx).
		Where("course_id IN (SELECT id FROM courses WHERE account_id = ?)", accountID).
		Find(&sections).Error; err != nil {
		return nil, err
	}

	// Build a map of course ID to SIS course ID for lookups (tenant-scoped)
	var courses []models.Course
	s.db.WithContext(ctx).Where("account_id = ?", accountID).Find(&courses)
	courseMap := make(map[uint]string)
	for _, c := range courses {
		if c.SISCourseID != nil {
			courseMap[c.ID] = *c.SISCourseID
		}
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	_ = writer.Write([]string{"section_id", "course_id", "name", "status"})

	for _, sec := range sections {
		sisSectionID := ""
		if sec.SISSectionID != nil {
			sisSectionID = *sec.SISSectionID
		}

		sisCourseID := courseMap[sec.CourseID]

		status := "active"
		if sec.WorkflowState == "deleted" {
			status = "deleted"
		}

		_ = writeCSVRow(writer, []string{
			sisSectionID,
			sisCourseID,
			sec.Name,
			status,
		})
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (s *SISImportService) ExportEnrollmentsCSV(ctx context.Context, accountID uint) ([]byte, error) {
	var enrollments []models.Enrollment
	if err := s.db.WithContext(ctx).
		Where("course_id IN (SELECT id FROM courses WHERE account_id = ?)", accountID).
		Find(&enrollments).Error; err != nil {
		return nil, err
	}

	// Build lookup maps for SIS IDs (tenant-scoped)
	var courses []models.Course
	s.db.WithContext(ctx).Where("account_id = ?", accountID).Find(&courses)
	courseMap := make(map[uint]string)
	for _, c := range courses {
		if c.SISCourseID != nil {
			courseMap[c.ID] = *c.SISCourseID
		}
	}

	var users []models.User
	s.db.WithContext(ctx).Where("account_id = ?", accountID).Find(&users)
	userMap := make(map[uint]string)
	for _, u := range users {
		if u.SISUserID != nil {
			userMap[u.ID] = *u.SISUserID
		}
	}

	var sections []models.CourseSection
	s.db.WithContext(ctx).
		Where("course_id IN (SELECT id FROM courses WHERE account_id = ?)", accountID).
		Find(&sections)
	sectionMap := make(map[uint]string)
	for _, sec := range sections {
		if sec.SISSectionID != nil {
			sectionMap[sec.ID] = *sec.SISSectionID
		}
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	_ = writer.Write([]string{"course_id", "user_id", "role", "section_id", "status"})

	for _, e := range enrollments {
		sisCourseID := courseMap[e.CourseID]
		sisUserID := userMap[e.UserID]

		sisSectionID := ""
		if e.CourseSectionID != nil {
			sisSectionID = sectionMap[*e.CourseSectionID]
		}

		role := sisRoleForType[e.Type]
		if e.RoleID != nil && e.Role != "" && e.Role != e.Type {
			role = e.Role
		}

		_ = writeCSVRow(writer, []string{
			sisCourseID,
			sisUserID,
			role,
			sisSectionID,
			sisEnrollmentStatus(string(e.WorkflowState)),
		})
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// ---------------------------------------------------------------------------
// Tenant-scoped SIS-id lookups
// ---------------------------------------------------------------------------

// Every SIS-ID lookup is tenant-scoped: a CSV uploaded by one tenant
// must never resolve (and then overwrite) another tenant's row.

func (s *SISImportService) findUserBySISID(ctx context.Context, sisID string, accountID uint) (*models.User, error) {
	var user models.User
	if err := s.db.WithContext(ctx).Where("sis_user_id = ? AND account_id = ?", sisID, accountID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *SISImportService) findCourseBySISID(ctx context.Context, sisID string, accountID uint) (*models.Course, error) {
	var course models.Course
	if err := s.db.WithContext(ctx).Where("sis_course_id = ? AND account_id = ?", sisID, accountID).First(&course).Error; err != nil {
		return nil, err
	}
	return &course, nil
}

func (s *SISImportService) findSectionBySISID(ctx context.Context, sisID string, accountID uint) (*models.CourseSection, error) {
	var section models.CourseSection
	if err := s.db.WithContext(ctx).
		Where("sis_section_id = ? AND course_id IN (SELECT id FROM courses WHERE account_id = ?)", sisID, accountID).
		First(&section).Error; err != nil {
		return nil, err
	}
	return &section, nil
}

func (s *SISImportService) findTermBySISID(ctx context.Context, sisID string, accountID uint) (*models.EnrollmentTerm, error) {
	var term models.EnrollmentTerm
	if err := s.db.WithContext(ctx).Where("sis_term_id = ? AND account_id = ?", sisID, accountID).First(&term).Error; err != nil {
		return nil, err
	}
	return &term, nil
}

// sisIDTaken reports whether a SIS id is already used anywhere (the
// sis_*_id unique indexes are deployment-wide). Called only after the
// tenant-scoped lookup missed, so a hit means another tenant owns it.
// table/column are compile-time constants, never user input.
func (s *SISImportService) sisIDTaken(ctx context.Context, table, column, sisID string) bool {
	var n int64
	if err := s.db.WithContext(ctx).Table(table).Where(column+" = ?", sisID).Count(&n).Error; err != nil {
		return false
	}
	return n > 0
}

// accountRefMatches reports whether a courses.csv account_id value names
// accountID, either by its sis_account_id or by its numeric id.
func (s *SISImportService) accountRefMatches(ctx context.Context, ref string, accountID uint) bool {
	var acct models.Account
	if err := s.db.WithContext(ctx).Where("id = ?", accountID).First(&acct).Error; err != nil {
		return false
	}
	if acct.SISAccountID != nil && *acct.SISAccountID == ref {
		return true
	}
	if parsed, err := strconv.ParseUint(ref, 10, 64); err == nil && uint(parsed) == accountID {
		return true
	}
	return false
}

func (s *SISImportService) recordError(ctx context.Context, batchID uint, row int, message string, file string) {
	batchError := &models.SISBatchError{
		SISBatchID: batchID,
		Row:        row,
		Message:    message,
		File:       file,
	}
	_ = s.errorRepo.Create(ctx, batchError)
}

// Utility functions

func splitName(name string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(name), " ", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return name, ""
}

// parseSISDate accepts ISO 8601 timestamps and plain dates.
func parseSISDate(v string) (*time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02T15:04Z07:00", "2006-01-02"} {
		if t, err := time.Parse(layout, v); err == nil {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("invalid date %q (use ISO 8601)", v)
}
