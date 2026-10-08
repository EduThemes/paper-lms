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
	"log/slog"
	"sort"
	"strconv"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/storage"
)

// SISCSVExporter is the provisioning export surface the account reports
// reuse (satisfied by *SISImportService).
type SISCSVExporter interface {
	ExportUsersCSV(ctx context.Context, accountID uint) ([]byte, error)
	ExportCoursesCSV(ctx context.Context, accountID uint) ([]byte, error)
	ExportSectionsCSV(ctx context.Context, accountID uint) ([]byte, error)
	ExportEnrollmentsCSV(ctx context.Context, accountID uint) ([]byte, error)
}

// StudentGrader computes a student's course grade (satisfied by
// *GradingService).
type StudentGrader interface {
	GetStudentGrade(ctx context.Context, courseID, studentID uint) (*StudentGrade, error)
}

// AccountReportParam describes one parameter a report accepts.
type AccountReportParam struct {
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

// AccountReportDefinition is one entry of the report catalog.
type AccountReportDefinition struct {
	Report      string                        `json:"report"`
	Title       string                        `json:"title"`
	Description string                        `json:"description"`
	Parameters  map[string]AccountReportParam `json:"parameters"`
}

var (
	// ErrUnknownAccountReport is returned for a report name not in the
	// catalog.
	ErrUnknownAccountReport = errors.New("unknown report")
	// ErrAccountReportNotFound is returned for a run that does not exist
	// in the caller's account (or is not of the named report).
	ErrAccountReportNotFound = errors.New("report not found")
	// ErrAccountReportParams is returned when a run's parameters are
	// invalid.
	ErrAccountReportParams = errors.New("invalid report parameters")
)

var provisioningParams = map[string]AccountReportParam{
	"enrollment_term_id": {Description: "Not applied: the export covers every term."},
	"users":              {Description: "Include users"},
	"courses":            {Description: "Include courses"},
	"sections":           {Description: "Include sections"},
	"enrollments":        {Description: "Include enrollments"},
}

// accountReportCatalog lists the reports this instance can run, using
// Canvas's report names so existing scripts find them.
var accountReportCatalog = []AccountReportDefinition{
	{
		Report:      "provisioning_csv",
		Title:       "Provisioning",
		Description: "Users, courses, sections and enrollments in SIS import format. One CSV, or a ZIP when several are chosen.",
		Parameters:  provisioningParams,
	},
	{
		Report:      "sis_export_csv",
		Title:       "SIS Export",
		Description: "The same files as Provisioning, for re-importing into a student information system.",
		Parameters:  provisioningParams,
	},
	{
		Report:      "grade_export_csv",
		Title:       "Grade Export",
		Description: "Current and final scores for every student enrollment.",
		Parameters: map[string]AccountReportParam{
			"enrollment_term_id": {Description: "Only courses in this term"},
		},
	},
	{
		Report:      "last_user_access_csv",
		Title:       "Last User Access",
		Description: "When each user last used the site.",
		Parameters:  map[string]AccountReportParam{},
	},
	{
		Report:      "unpublished_courses_csv",
		Title:       "Unpublished Courses",
		Description: "Courses that have not been published.",
		Parameters: map[string]AccountReportParam{
			"enrollment_term_id": {Description: "Only courses in this term"},
		},
	},
}

// AccountReportCatalog returns the report definitions.
func AccountReportCatalog() []AccountReportDefinition {
	return accountReportCatalog
}

func findReportDefinition(name string) (AccountReportDefinition, bool) {
	for _, d := range accountReportCatalog {
		if d.Report == name {
			return d, true
		}
	}
	return AccountReportDefinition{}, false
}

// accountReportStaleAfter is how long a run may stay "running" before it is
// reported as failed (the process that ran it must have stopped).
const accountReportStaleAfter = time.Hour

// AccountReportService runs Canvas-style account reports in the
// background and stores their output in the storage backend (G138).
type AccountReportService struct {
	repo    repository.AccountReportRepository
	storage storage.Backend
	sis     SISCSVExporter
	grader  StudentGrader
	// run launches a report job; tests replace it to run synchronously.
	run     func(func())
	timeout time.Duration
}

// NewAccountReportService wires the service. grader may be nil (the grade
// export then leaves scores blank).
func NewAccountReportService(repo repository.AccountReportRepository, backend storage.Backend, sis SISCSVExporter, grader StudentGrader) *AccountReportService {
	return &AccountReportService{
		repo:    repo,
		storage: backend,
		sis:     sis,
		grader:  grader,
		run:     func(f func()) { go f() },
		timeout: 15 * time.Minute,
	}
}

// SetRunner replaces how jobs are launched (tests run them inline).
func (s *AccountReportService) SetRunner(run func(func())) {
	if run != nil {
		s.run = run
	}
}

// AvailableReport is a catalog entry plus its newest run.
type AvailableReport struct {
	AccountReportDefinition
	LastRun *models.AccountReport
}

// Available returns the report catalog with each report's last run.
func (s *AccountReportService) Available(ctx context.Context, accountID uint) ([]AvailableReport, error) {
	last, err := s.repo.LastRuns(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]AvailableReport, len(accountReportCatalog))
	for i, d := range accountReportCatalog {
		out[i] = AvailableReport{AccountReportDefinition: d}
		if r, ok := last[d.Report]; ok {
			r := r
			markStale(&r)
			out[i].LastRun = &r
		}
	}
	return out, nil
}

// markStale reports a run left "running" by a stopped process as failed.
func markStale(r *models.AccountReport) {
	if r.WorkflowState != models.AccountReportRunning && r.WorkflowState != models.AccountReportCreated {
		return
	}
	since := r.CreatedAt
	if r.StartedAt != nil {
		since = *r.StartedAt
	}
	if time.Since(since) > accountReportStaleAfter {
		r.WorkflowState = models.AccountReportError
		r.Message = "The report stopped before it finished. Run it again."
	}
}

// normalizeReportParams validates and normalizes a run's parameters.
func normalizeReportParams(def AccountReportDefinition, raw map[string]any) (map[string]any, error) {
	out := map[string]any{}
	for k, v := range raw {
		if _, ok := def.Parameters[k]; !ok {
			continue
		}
		switch k {
		case "enrollment_term_id":
			id, err := paramUint(v)
			if err != nil {
				return nil, fmt.Errorf("%w: enrollment_term_id must be a term id", ErrAccountReportParams)
			}
			if id != 0 {
				out[k] = id
			}
		default:
			out[k] = paramTruthy(v)
		}
	}
	if def.Report == "provisioning_csv" || def.Report == "sis_export_csv" {
		chosen := false
		for _, k := range []string{"users", "courses", "sections", "enrollments"} {
			if b, _ := out[k].(bool); b {
				chosen = true
			}
		}
		if !chosen {
			return nil, fmt.Errorf("%w: choose at least one of users, courses, sections or enrollments", ErrAccountReportParams)
		}
	}
	return out, nil
}

func paramTruthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		b, err := strconv.ParseBool(t)
		return err == nil && b || t == "1" || t == "on"
	case float64:
		return t != 0
	}
	return false
}

func paramUint(v any) (uint, error) {
	switch t := v.(type) {
	case nil:
		return 0, nil
	case float64:
		if t < 0 || t != float64(uint(t)) {
			return 0, errors.New("not an id")
		}
		return uint(t), nil
	case string:
		if t == "" {
			return 0, nil
		}
		n, err := strconv.ParseUint(t, 10, 64)
		return uint(n), err
	}
	return 0, errors.New("not an id")
}

// Start records a new run of reportType and launches it in the background.
func (s *AccountReportService) Start(ctx context.Context, reportType string, params map[string]any, userID, accountID uint) (*models.AccountReport, error) {
	def, ok := findReportDefinition(reportType)
	if !ok {
		return nil, ErrUnknownAccountReport
	}
	norm, err := normalizeReportParams(def, params)
	if err != nil {
		return nil, err
	}
	encoded, _ := json.Marshal(norm)
	rep := &models.AccountReport{
		AccountID:     accountID,
		UserID:        userID,
		ReportType:    reportType,
		WorkflowState: models.AccountReportCreated,
		Parameters:    string(encoded),
	}
	if err := s.repo.Create(ctx, rep); err != nil {
		return nil, err
	}
	id := rep.ID
	s.run(func() { s.execute(id, accountID) })
	return rep, nil
}

// execute generates the report off the request path.
func (s *AccountReportService) execute(id, accountID uint) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	rep, err := s.repo.FindByID(ctx, id, accountID)
	if err != nil {
		return
	}
	if rep.WorkflowState != models.AccountReportCreated {
		return // aborted or deleted before it started
	}
	now := time.Now()
	rep.WorkflowState = models.AccountReportRunning
	rep.StartedAt = &now
	rep.Progress = 10
	if err := s.repo.Update(ctx, rep); err != nil {
		return
	}

	var params map[string]any
	_ = json.Unmarshal([]byte(rep.Parameters), &params)

	data, name, contentType, genErr := s.generate(ctx, rep.ReportType, params, accountID)

	// Re-read: the run may have been aborted or deleted meanwhile.
	current, err := s.repo.FindByID(ctx, id, accountID)
	if err != nil || current.WorkflowState != models.AccountReportRunning {
		return
	}
	ended := time.Now()
	current.EndedAt = &ended
	if genErr != nil {
		slog.Error("account report failed", "report_id", id, "report", rep.ReportType, "err", genErr)
		current.WorkflowState = models.AccountReportError
		current.Message = "The report could not be generated."
		_ = s.repo.Update(ctx, current)
		return
	}
	key := fmt.Sprintf("AccountReports/%d/%d/%s", accountID, id, name)
	if s.storage == nil {
		current.WorkflowState = models.AccountReportError
		current.Message = "No file storage is configured."
		_ = s.repo.Update(ctx, current)
		return
	}
	if err := s.storage.Put(ctx, key, bytes.NewReader(data), contentType); err != nil {
		slog.Error("account report upload failed", "report_id", id, "err", err)
		current.WorkflowState = models.AccountReportError
		current.Message = "The report file could not be saved."
		_ = s.repo.Update(ctx, current)
		return
	}
	current.FileKey = &key
	current.FileName = &name
	current.ContentType = &contentType
	current.FileSize = int64(len(data))
	current.Progress = 100
	current.WorkflowState = models.AccountReportComplete
	_ = s.repo.Update(ctx, current)
}

func optUint(params map[string]any, key string) *uint {
	if v, ok := params[key]; ok {
		if id, err := paramUint(v); err == nil && id != 0 {
			return &id
		}
	}
	return nil
}

// generate builds the report file.
func (s *AccountReportService) generate(ctx context.Context, reportType string, params map[string]any, accountID uint) ([]byte, string, string, error) {
	stamp := time.Now().UTC().Format("20060102-150405")
	switch reportType {
	case "provisioning_csv", "sis_export_csv":
		return s.provisioning(ctx, reportType, params, accountID, stamp)
	case "grade_export_csv":
		data, err := s.gradeExport(ctx, optUint(params, "enrollment_term_id"), accountID)
		return data, "grade_export_" + stamp + ".csv", "text/csv", err
	case "last_user_access_csv":
		data, err := s.lastUserAccess(ctx, accountID)
		return data, "last_user_access_" + stamp + ".csv", "text/csv", err
	case "unpublished_courses_csv":
		data, err := s.unpublishedCourses(ctx, optUint(params, "enrollment_term_id"), accountID)
		return data, "unpublished_courses_" + stamp + ".csv", "text/csv", err
	}
	return nil, "", "", ErrUnknownAccountReport
}

func (s *AccountReportService) provisioning(ctx context.Context, reportType string, params map[string]any, accountID uint, stamp string) ([]byte, string, string, error) {
	if s.sis == nil {
		return nil, "", "", errors.New("provisioning export unavailable")
	}
	type part struct {
		name string
		fn   func(context.Context, uint) ([]byte, error)
	}
	var parts []part
	for _, p := range []part{
		{"users", s.sis.ExportUsersCSV},
		{"courses", s.sis.ExportCoursesCSV},
		{"sections", s.sis.ExportSectionsCSV},
		{"enrollments", s.sis.ExportEnrollmentsCSV},
	} {
		if b, _ := params[p.name].(bool); b {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return nil, "", "", ErrAccountReportParams
	}
	if len(parts) == 1 {
		data, err := parts[0].fn(ctx, accountID)
		return data, parts[0].name + ".csv", "text/csv", err
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, p := range parts {
		data, err := p.fn(ctx, accountID)
		if err != nil {
			return nil, "", "", err
		}
		w, err := zw.Create(p.name + ".csv")
		if err != nil {
			return nil, "", "", err
		}
		if _, err := w.Write(data); err != nil {
			return nil, "", "", err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, "", "", err
	}
	return buf.Bytes(), reportType + "_" + stamp + ".zip", "application/zip", nil
}

func strOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func uintOrEmpty(p *uint) string {
	if p == nil {
		return ""
	}
	return strconv.FormatUint(uint64(*p), 10)
}

func formatScore(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func (s *AccountReportService) gradeExport(ctx context.Context, termID *uint, accountID uint) ([]byte, error) {
	rows, err := s.repo.GradeExportRows(ctx, termID, accountID)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"student name", "student id", "student sis", "course", "course id", "course sis",
		"section", "section id", "section sis", "term", "term id", "term sis", "current score", "final score", "enrollment state"})
	type key struct{ course, user uint }
	cache := map[key]*StudentGrade{}
	for _, r := range rows {
		current, final := "", ""
		if s.grader != nil {
			k := key{r.CourseID, r.UserID}
			g, seen := cache[k]
			if !seen {
				g, _ = s.grader.GetStudentGrade(ctx, r.CourseID, r.UserID)
				cache[k] = g
			}
			if g != nil {
				current, final = formatScore(g.CurrentScore), formatScore(g.FinalScore)
			}
		}
		_ = w.Write([]string{
			csvSafe(r.UserName), strconv.FormatUint(uint64(r.UserID), 10), csvSafe(strOrEmpty(r.SISUserID)),
			csvSafe(r.CourseName), strconv.FormatUint(uint64(r.CourseID), 10), csvSafe(strOrEmpty(r.SISCourseID)),
			csvSafe(strOrEmpty(r.SectionName)), uintOrEmpty(r.SectionID), csvSafe(strOrEmpty(r.SISSectionID)),
			csvSafe(strOrEmpty(r.TermName)), uintOrEmpty(r.TermID), csvSafe(strOrEmpty(r.SISTermID)),
			current, final, r.EnrollmentState,
		})
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

func (s *AccountReportService) lastUserAccess(ctx context.Context, accountID uint) ([]byte, error) {
	rows, err := s.repo.LastAccessRows(ctx, accountID)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"user id", "user sis id", "user name", "last access at"})
	for _, r := range rows {
		last := ""
		if r.LastAccessAt != nil {
			last = r.LastAccessAt.UTC().Format(time.RFC3339)
		}
		_ = w.Write([]string{strconv.FormatUint(uint64(r.UserID), 10), csvSafe(strOrEmpty(r.SISUserID)), csvSafe(r.Name), last})
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

func (s *AccountReportService) unpublishedCourses(ctx context.Context, termID *uint, accountID uint) ([]byte, error) {
	courses, err := s.repo.UnpublishedCourses(ctx, termID, accountID)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(courses, func(i, j int) bool { return courses[i].ID < courses[j].ID })
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"course id", "course sis id", "short name", "long name", "start date", "end date"})
	for _, c := range courses {
		start, end := "", ""
		if c.StartAt != nil {
			start = c.StartAt.UTC().Format(time.RFC3339)
		}
		if c.EndAt != nil {
			end = c.EndAt.UTC().Format(time.RFC3339)
		}
		_ = w.Write([]string{strconv.FormatUint(uint64(c.ID), 10), csvSafe(strOrEmpty(c.SISCourseID)), csvSafe(c.CourseCode), csvSafe(c.Name), start, end})
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

// csvSafe neutralises spreadsheet formula injection: a cell starting with
// = + - @ (or a tab / carriage return) is prefixed with a quote so Excel
// and Sheets show it as text instead of evaluating it.
func csvSafe(v string) string {
	if v == "" {
		return v
	}
	switch v[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + v
	}
	return v
}

// List returns runs of reportType ("" = all) in the account.
func (s *AccountReportService) List(ctx context.Context, reportType string, params repository.PaginationParams, accountID uint) (*repository.PaginatedResult[models.AccountReport], error) {
	if reportType != "" {
		if _, ok := findReportDefinition(reportType); !ok {
			return nil, ErrUnknownAccountReport
		}
	}
	page, err := s.repo.List(ctx, reportType, params, accountID)
	if err != nil {
		return nil, err
	}
	for i := range page.Items {
		markStale(&page.Items[i])
	}
	return page, nil
}

// Get loads one run, which must be of reportType.
func (s *AccountReportService) Get(ctx context.Context, reportType string, id, accountID uint) (*models.AccountReport, error) {
	rep, err := s.repo.FindByID(ctx, id, accountID)
	if err != nil || rep == nil || rep.ReportType != reportType {
		return nil, ErrAccountReportNotFound
	}
	markStale(rep)
	return rep, nil
}

// Abort stops a run that has not finished.
func (s *AccountReportService) Abort(ctx context.Context, reportType string, id, accountID uint) (*models.AccountReport, error) {
	rep, err := s.Get(ctx, reportType, id, accountID)
	if err != nil {
		return nil, err
	}
	if rep.WorkflowState == models.AccountReportCreated || rep.WorkflowState == models.AccountReportRunning {
		now := time.Now()
		rep.WorkflowState = models.AccountReportAborted
		rep.EndedAt = &now
		if err := s.repo.Update(ctx, rep); err != nil {
			return nil, err
		}
	}
	return rep, nil
}

// Delete removes a run and its file.
func (s *AccountReportService) Delete(ctx context.Context, reportType string, id, accountID uint) (*models.AccountReport, error) {
	rep, err := s.Get(ctx, reportType, id, accountID)
	if err != nil {
		return nil, err
	}
	if rep.FileKey != nil && s.storage != nil {
		_ = s.storage.Delete(ctx, *rep.FileKey)
	}
	rep.WorkflowState = models.AccountReportDeleted
	rep.FileKey = nil
	if err := s.repo.Update(ctx, rep); err != nil {
		return nil, err
	}
	return rep, nil
}

// OpenFile opens a complete run's output.
func (s *AccountReportService) OpenFile(ctx context.Context, rep *models.AccountReport) (io.ReadCloser, error) {
	if rep.WorkflowState != models.AccountReportComplete || rep.FileKey == nil || s.storage == nil {
		return nil, ErrAccountReportNotFound
	}
	return s.storage.Get(ctx, *rep.FileKey)
}
