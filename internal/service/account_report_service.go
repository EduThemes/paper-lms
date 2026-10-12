package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// AccountReportService runs account-level reports (G117; Canvas: the
// account_reports plugin). Each run is an AccountReport row; the CSV (or a
// zip of CSVs) is generated off the request path and stored on the row.
// Every query is confined to the report's account.
type AccountReportService struct {
	db     *gorm.DB
	grades StudentGradeCalculator
	// async runs reports in a goroutine; tests turn it off to run inline.
	async bool
}

// StudentGradeCalculator computes a student's course grade
// (*GradingService).
type StudentGradeCalculator interface {
	GetStudentGrade(ctx context.Context, courseID, studentID uint) (*StudentGrade, error)
}

// NewAccountReportService builds the service. grades may be nil (the
// grade export then leaves scores blank).
func NewAccountReportService(db *gorm.DB, grades StudentGradeCalculator) *AccountReportService {
	return &AccountReportService{db: db, grades: grades, async: true}
}

// SetAsync toggles background execution (tests run reports inline).
func (s *AccountReportService) SetAsync(async bool) *AccountReportService {
	s.async = async
	return s
}

// AccountReportDefinition describes one report type for the UI.
type AccountReportDefinition struct {
	Report      string   `json:"report"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Parameters  []string `json:"parameters"`
}

// Report type keys (Canvas names).
const (
	ReportProvisioning      = "provisioning_csv"
	ReportSISExport         = "sis_export_csv"
	ReportGradeExport       = "grade_export_csv"
	ReportLastUserAccess    = "last_user_access_csv"
	ReportUnpublishedCourse = "unpublished_courses_csv"
	ReportDeletedCourses    = "recently_deleted_courses_csv"
	ReportZeroActivity      = "zero_activity_csv"
)

// provisioningEntities are the CSVs the provisioning / SIS export reports
// can produce, in SIS import order.
var provisioningEntities = []string{"users", "logins", "terms", "courses", "sections", "enrollments", "groups", "group_memberships", "user_observers"}

// AccountReportDefinitions lists the available reports.
func AccountReportDefinitions() []AccountReportDefinition {
	return []AccountReportDefinition{
		{ReportProvisioning, "Provisioning", "Every user, login, term, course, section, enrollment, group and observer link in the account (SIS and non-SIS), with Paper and SIS ids.", []string{"entities", "enrollment_term_id", "include_deleted"}},
		{ReportSISExport, "SIS Export", "The SIS-managed rows in SIS import format, ready to edit and re-import.", []string{"entities", "enrollment_term_id", "include_deleted"}},
		{ReportGradeExport, "Grade Export", "Current and final scores for every student enrollment.", []string{"enrollment_term_id", "include_deleted"}},
		{ReportLastUserAccess, "Last User Access", "Each user's most recent sign-in and course activity.", []string{"enrollment_term_id"}},
		{ReportUnpublishedCourse, "Unpublished Courses", "Courses that have not been published.", []string{"enrollment_term_id"}},
		{ReportDeletedCourses, "Recently Deleted Courses", "Courses deleted in the last 30 days (restorable from Admin Tools).", nil},
		{ReportZeroActivity, "Zero Activity", "Active student enrollments with no recorded activity.", []string{"enrollment_term_id"}},
	}
}

func knownReport(report string) bool {
	for _, d := range AccountReportDefinitions() {
		if d.Report == report {
			return true
		}
	}
	return false
}

// AccountReportParams are a run's options.
type AccountReportParams struct {
	Entities         []string `json:"entities,omitempty"`
	EnrollmentTermID *uint    `json:"enrollment_term_id,omitempty"`
	IncludeDeleted   bool     `json:"include_deleted,omitempty"`
}

// Errors surfaced to handlers.
var (
	ErrUnknownReport        = errors.New("unknown report type")
	ErrAccountReportMissing = errors.New("report not found")
	ErrReportTermNotFound   = errors.New("enrollment term not found")
)

// Start creates a report run and generates it (in the background unless
// async is off).
func (s *AccountReportService) Start(ctx context.Context, accountID, userID uint, report string, params AccountReportParams) (*models.AccountReport, error) {
	if !knownReport(report) {
		return nil, ErrUnknownReport
	}
	if params.EnrollmentTermID != nil {
		var n int64
		s.db.WithContext(ctx).Model(&models.EnrollmentTerm{}).
			Where("id = ? AND account_id = ?", *params.EnrollmentTermID, accountID).Count(&n)
		if n == 0 {
			return nil, ErrReportTermNotFound
		}
	}
	valid := params.Entities[:0]
	for _, e := range params.Entities {
		for _, known := range provisioningEntities {
			if e == known {
				valid = append(valid, e)
				break
			}
		}
	}
	params.Entities = valid
	raw, _ := json.Marshal(params)
	r := &models.AccountReport{AccountID: accountID, UserID: userID, ReportType: report, Parameters: string(raw)}
	if err := s.db.WithContext(ctx).Create(r).Error; err != nil {
		return nil, err
	}
	if s.async {
		id := r.ID
		go func() { // #nosec G118 -- the report job outlives the request that queued it
			defer recoverFromPanic("account report")
			s.run(context.Background(), id)
		}()
	} else {
		s.run(ctx, r.ID)
		_ = s.db.WithContext(ctx).First(r, r.ID).Error
	}
	return r, nil
}

// List returns an account's report runs, newest first (without file data).
func (s *AccountReportService) List(ctx context.Context, accountID uint, report string, limit int) ([]models.AccountReport, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q := s.db.WithContext(ctx).Omit("file_data").Where("account_id = ?", accountID)
	if report != "" {
		q = q.Where("report_type = ?", report)
	}
	var out []models.AccountReport
	err := q.Order("created_at DESC, id DESC").Limit(limit).Find(&out).Error
	return out, err
}

// Get loads one run of accountID (with file data).
func (s *AccountReportService) Get(ctx context.Context, id, accountID uint) (*models.AccountReport, error) {
	var r models.AccountReport
	if err := s.db.WithContext(ctx).Where("id = ? AND account_id = ?", id, accountID).First(&r).Error; err != nil {
		return nil, ErrAccountReportMissing
	}
	return &r, nil
}

// Delete removes a run of accountID.
func (s *AccountReportService) Delete(ctx context.Context, id, accountID uint) error {
	res := s.db.WithContext(ctx).Where("id = ? AND account_id = ?", id, accountID).Delete(&models.AccountReport{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrAccountReportMissing
	}
	return nil
}

// Abort stops a run of accountID that has not finished (Canvas PUT
// .../reports/:report/:id/abort). A finished run is returned unchanged.
func (s *AccountReportService) Abort(ctx context.Context, id, accountID uint) (*models.AccountReport, error) {
	if err := s.db.WithContext(ctx).Model(&models.AccountReport{}).
		Where("id = ? AND account_id = ? AND workflow_state IN ?", id, accountID,
			[]string{models.AccountReportCreated, models.AccountReportRunning}).
		Updates(map[string]interface{}{
			"workflow_state": models.AccountReportAborted, "ended_at": time.Now(),
			"message": "The report was aborted.",
		}).Error; err != nil {
		return nil, err
	}
	return s.Get(ctx, id, accountID)
}

// ParseAccountReportParams reads report parameters in either shape: this
// API's {"entities": [...]} or Canvas's per-file flags
// ({"users": true, "courses": "true", ...}, as JSON or form
// parameters[users]=true), plus enrollment_term_id and include_deleted.
func ParseAccountReportParams(raw map[string]any) (AccountReportParams, error) {
	var p AccountReportParams
	truthy := func(v any) bool {
		switch t := v.(type) {
		case bool:
			return t
		case string:
			return strings.EqualFold(t, "true") || t == "1"
		case float64:
			return t != 0
		}
		return false
	}
	seen := map[string]bool{}
	if list, ok := raw["entities"].([]any); ok {
		for _, e := range list {
			if name, ok := e.(string); ok && !seen[name] {
				seen[name] = true
				p.Entities = append(p.Entities, name)
			}
		}
	}
	for _, name := range provisioningEntities {
		if v, ok := raw[name]; ok && truthy(v) && !seen[name] {
			seen[name] = true
			p.Entities = append(p.Entities, name)
		}
	}
	switch t := raw["enrollment_term_id"].(type) {
	case float64:
		if t > 0 {
			id := uint(t)
			p.EnrollmentTermID = &id
		}
	case string:
		if t != "" {
			n, err := strconv.ParseUint(t, 10, 64)
			if err != nil || n == 0 {
				return p, ErrReportTermNotFound
			}
			id := uint(n)
			p.EnrollmentTermID = &id
		}
	}
	p.IncludeDeleted = truthy(raw["include_deleted"])
	return p, nil
}

// run generates a report and stores the result on its row.
func (s *AccountReportService) run(ctx context.Context, id uint) {
	var r models.AccountReport
	if err := s.db.WithContext(ctx).First(&r, id).Error; err != nil {
		return
	}
	now := time.Now()
	// Only a run still waiting starts (an aborted one stays aborted).
	if res := s.db.WithContext(ctx).Model(&models.AccountReport{}).
		Where("id = ? AND workflow_state = ?", r.ID, models.AccountReportCreated).
		Updates(map[string]interface{}{
			"workflow_state": models.AccountReportRunning, "started_at": now, "progress": 5,
		}); res.Error != nil || res.RowsAffected == 0 {
		return
	}
	// finish records the outcome unless the run was aborted meanwhile.
	finish := func(fields map[string]interface{}) {
		s.db.WithContext(ctx).Model(&models.AccountReport{}).
			Where("id = ? AND workflow_state = ?", r.ID, models.AccountReportRunning).
			Updates(fields)
	}
	var params AccountReportParams
	_ = json.Unmarshal([]byte(r.Parameters), &params)

	files, rows, err := s.generate(ctx, &r, params)
	ended := time.Now()
	if err != nil {
		slog.Error("account report failed", "report_id", r.ID, "report", r.ReportType, "error", err)
		finish(map[string]interface{}{
			"workflow_state": models.AccountReportError, "ended_at": ended, "progress": 100,
			"message": "The report could not be generated.",
		})
		return
	}
	name, data, err := packReportFiles(r.ReportType, r.ID, files)
	if err != nil {
		finish(map[string]interface{}{
			"workflow_state": models.AccountReportError, "ended_at": ended, "progress": 100,
			"message": "The report could not be packaged.",
		})
		return
	}
	finish(map[string]interface{}{
		"workflow_state": models.AccountReportComplete, "ended_at": ended, "progress": 100,
		"row_count": rows, "file_name": name, "file_data": data,
		"message": fmt.Sprintf("%d row(s)", rows),
	})
}

// reportFile is one CSV of a report.
type reportFile struct {
	name string
	rows [][]string // first row is the header
}

// packReportFiles returns a single CSV, or a zip when there are several.
func packReportFiles(report string, id uint, files []reportFile) (string, []byte, error) {
	encode := func(f reportFile) ([]byte, error) {
		var buf bytes.Buffer
		w := csv.NewWriter(&buf)
		for _, row := range f.rows {
			if err := writeCSVRow(w, row); err != nil {
				return nil, err
			}
		}
		w.Flush()
		return buf.Bytes(), w.Error()
	}
	if len(files) == 1 {
		b, err := encode(files[0])
		return files[0].name, b, err
	}
	var zbuf bytes.Buffer
	zw := zip.NewWriter(&zbuf)
	for _, f := range files {
		b, err := encode(f)
		if err != nil {
			return "", nil, err
		}
		w, err := zw.Create(f.name)
		if err != nil {
			return "", nil, err
		}
		if _, err := w.Write(b); err != nil {
			return "", nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return "", nil, err
	}
	return fmt.Sprintf("%s_%d.zip", strings.TrimSuffix(report, "_csv"), id), zbuf.Bytes(), nil
}

func (s *AccountReportService) generate(ctx context.Context, r *models.AccountReport, p AccountReportParams) ([]reportFile, int, error) {
	switch r.ReportType {
	case ReportProvisioning, ReportSISExport:
		return s.provisioning(ctx, r.AccountID, p, r.ReportType == ReportSISExport)
	case ReportGradeExport:
		f, err := s.gradeExport(ctx, r.AccountID, p)
		return []reportFile{f}, len(f.rows) - 1, err
	case ReportLastUserAccess:
		f, err := s.lastUserAccess(ctx, r.AccountID, p)
		return []reportFile{f}, len(f.rows) - 1, err
	case ReportUnpublishedCourse:
		f, err := s.unpublishedCourses(ctx, r.AccountID, p)
		return []reportFile{f}, len(f.rows) - 1, err
	case ReportDeletedCourses:
		f, err := s.deletedCourses(ctx, r.AccountID)
		return []reportFile{f}, len(f.rows) - 1, err
	case ReportZeroActivity:
		f, err := s.zeroActivity(ctx, r.AccountID, p)
		return []reportFile{f}, len(f.rows) - 1, err
	}
	return nil, 0, ErrUnknownReport
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func strOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func uintStr(v uint) string { return strconv.FormatUint(uint64(v), 10) }

func uintPtrStr(p *uint) string {
	if p == nil {
		return ""
	}
	return uintStr(*p)
}

func timeStr(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func scoreStr(f *float64) string {
	if f == nil {
		return ""
	}
	return strconv.FormatFloat(*f, 'f', 2, 64)
}

// courseScope returns the course filter for a report (tenant + optional
// term + deleted handling) as a gorm scope on the courses table alias c.
func courseFilterSQL(p AccountReportParams) (string, []interface{}) {
	where := "c.account_id = ?"
	var args []interface{}
	if p.EnrollmentTermID != nil {
		where += " AND c.enrollment_term_id = ?"
		args = append(args, *p.EnrollmentTermID)
	}
	if !p.IncludeDeleted {
		where += " AND c.workflow_state <> 'deleted'"
	}
	return where, args
}

// sisCourseStatus maps a course workflow state to the SIS CSV status.
func sisCourseStatus(state string) string {
	switch state {
	case "deleted":
		return "deleted"
	case "completed":
		return "completed"
	case "available":
		return "active"
	default:
		return "unpublished"
	}
}

func sisEnrollmentStatus(state string) string {
	switch state {
	case "completed":
		return "completed"
	case "inactive":
		return "inactive"
	case "deleted":
		return "deleted"
	case "invited", "creation_pending":
		return "active"
	default:
		return state
	}
}

var sisRoleForType = map[string]string{
	"StudentEnrollment":  "student",
	"TeacherEnrollment":  "teacher",
	"TaEnrollment":       "ta",
	"ObserverEnrollment": "observer",
	"DesignerEnrollment": "designer",
}

// ---------------------------------------------------------------------------
// provisioning / SIS export
// ---------------------------------------------------------------------------

func (s *AccountReportService) provisioning(ctx context.Context, accountID uint, p AccountReportParams, sisOnly bool) ([]reportFile, int, error) {
	entities := p.Entities
	if len(entities) == 0 {
		entities = []string{"users", "courses", "sections", "enrollments"}
	}
	want := map[string]bool{}
	for _, e := range entities {
		want[e] = true
	}
	var files []reportFile
	total := 0
	add := func(f reportFile, err error) error {
		if err != nil {
			return err
		}
		files = append(files, f)
		total += len(f.rows) - 1
		return nil
	}
	for _, e := range provisioningEntities {
		if !want[e] {
			continue
		}
		var err error
		switch e {
		case "users":
			err = add(s.provUsers(ctx, accountID, p, sisOnly))
		case "logins":
			err = add(s.provLogins(ctx, accountID, sisOnly))
		case "terms":
			err = add(s.provTerms(ctx, accountID, p, sisOnly))
		case "courses":
			err = add(s.provCourses(ctx, accountID, p, sisOnly))
		case "sections":
			err = add(s.provSections(ctx, accountID, p, sisOnly))
		case "enrollments":
			err = add(s.provEnrollments(ctx, accountID, p, sisOnly))
		case "groups":
			err = add(s.provGroups(ctx, accountID, p, sisOnly))
		case "group_memberships":
			err = add(s.provGroupMemberships(ctx, accountID, sisOnly))
		case "user_observers":
			err = add(s.provObservers(ctx, accountID, sisOnly))
		}
		if err != nil {
			return nil, 0, err
		}
	}
	return files, total, nil
}

func (s *AccountReportService) provUsers(ctx context.Context, accountID uint, p AccountReportParams, sisOnly bool) (reportFile, error) {
	var users []models.User
	q := s.db.WithContext(ctx).Where("account_id = ?", accountID)
	if sisOnly {
		q = q.Where("sis_user_id IS NOT NULL")
	}
	if err := q.Order("id").Find(&users).Error; err != nil {
		return reportFile{}, err
	}
	var rows [][]string
	if sisOnly {
		rows = append(rows, []string{"user_id", "login_id", "first_name", "last_name", "short_name", "sortable_name", "email", "status"})
	} else {
		rows = append(rows, []string{"paper_user_id", "user_id", "login_id", "first_name", "last_name", "full_name", "sortable_name", "short_name", "email", "status", "created_at"})
	}
	for _, u := range users {
		status := "active"
		if u.Suspended {
			status = "suspended"
		}
		if !p.IncludeDeleted && status != "active" && sisOnly {
			continue
		}
		first, last := splitName(u.Name)
		if sisOnly {
			rows = append(rows, []string{strOrEmpty(u.SISUserID), u.LoginID, first, last, u.ShortName, u.SortableName, u.Email, status})
		} else {
			rows = append(rows, []string{uintStr(u.ID), strOrEmpty(u.SISUserID), u.LoginID, first, last, u.Name, u.SortableName, u.ShortName, u.Email, status, u.CreatedAt.UTC().Format(time.RFC3339)})
		}
	}
	return reportFile{name: "users.csv", rows: rows}, nil
}

func (s *AccountReportService) provLogins(ctx context.Context, accountID uint, sisOnly bool) (reportFile, error) {
	type row struct {
		ID                       uint
		UserID                   uint
		UniqueID                 string
		SISUserID                *string
		AuthenticationProviderID *uint
		WorkflowState            string
		UserSIS                  *string
	}
	var rs []row
	q := `SELECT l.id, l.user_id, l.unique_id, l.sis_user_id, l.authentication_provider_id, l.workflow_state, u.sis_user_id AS user_sis
	      FROM user_logins l JOIN users u ON u.id = l.user_id
	      WHERE l.account_id = ? AND u.account_id = ? AND l.workflow_state <> 'deleted'`
	if sisOnly {
		q += " AND u.sis_user_id IS NOT NULL"
	}
	if err := s.db.WithContext(ctx).Raw(q+" ORDER BY l.id", accountID, accountID).Scan(&rs).Error; err != nil {
		return reportFile{}, err
	}
	rows := [][]string{{"user_id", "login_id", "authentication_provider_id", "existing_user_id", "existing_paper_user_id", "status"}}
	for _, r := range rs {
		rows = append(rows, []string{strOrEmpty(r.SISUserID), r.UniqueID, uintPtrStr(r.AuthenticationProviderID), strOrEmpty(r.UserSIS), uintStr(r.UserID), r.WorkflowState})
	}
	return reportFile{name: "logins.csv", rows: rows}, nil
}

func (s *AccountReportService) provTerms(ctx context.Context, accountID uint, p AccountReportParams, sisOnly bool) (reportFile, error) {
	var terms []models.EnrollmentTerm
	q := s.db.WithContext(ctx).Where("account_id = ?", accountID)
	if sisOnly {
		q = q.Where("sis_term_id IS NOT NULL")
	}
	if !p.IncludeDeleted {
		q = q.Where("COALESCE(workflow_state, 'active') <> 'deleted'")
	}
	if err := q.Order("id").Find(&terms).Error; err != nil {
		return reportFile{}, err
	}
	rows := [][]string{{"paper_term_id", "term_id", "name", "status", "start_date", "end_date"}}
	if sisOnly {
		rows[0] = []string{"term_id", "name", "status", "start_date", "end_date"}
	}
	for _, t := range terms {
		status := t.WorkflowState
		if status == "" {
			status = "active"
		}
		if sisOnly {
			rows = append(rows, []string{strOrEmpty(t.SISTermID), t.Name, status, timeStr(t.StartAt), timeStr(t.EndAt)})
		} else {
			rows = append(rows, []string{uintStr(t.ID), strOrEmpty(t.SISTermID), t.Name, status, timeStr(t.StartAt), timeStr(t.EndAt)})
		}
	}
	return reportFile{name: "terms.csv", rows: rows}, nil
}

func (s *AccountReportService) provCourses(ctx context.Context, accountID uint, p AccountReportParams, sisOnly bool) (reportFile, error) {
	type row struct {
		ID            uint
		SISCourseID   *string
		CourseCode    string
		Name          string
		WorkflowState string
		TermID        *uint
		TermSIS       *string
		StartAt       *time.Time
		EndAt         *time.Time
	}
	where, args := courseFilterSQL(p)
	q := `SELECT c.id, c.sis_course_id, c.course_code, c.name, c.workflow_state, c.enrollment_term_id AS term_id,
	             t.sis_term_id AS term_sis, c.start_at, c.end_at
	      FROM courses c LEFT JOIN enrollment_terms t ON t.id = c.enrollment_term_id
	      WHERE ` + where
	if sisOnly {
		q += " AND c.sis_course_id IS NOT NULL"
	}
	var rs []row
	if err := s.db.WithContext(ctx).Raw(q+" ORDER BY c.id", append([]interface{}{accountID}, args...)...).Scan(&rs).Error; err != nil {
		return reportFile{}, err
	}
	var rows [][]string
	if sisOnly {
		rows = append(rows, []string{"course_id", "short_name", "long_name", "account_id", "term_id", "status", "start_date", "end_date"})
	} else {
		rows = append(rows, []string{"paper_course_id", "course_id", "short_name", "long_name", "paper_term_id", "term_id", "status", "start_date", "end_date"})
	}
	for _, r := range rs {
		if sisOnly {
			rows = append(rows, []string{strOrEmpty(r.SISCourseID), r.CourseCode, r.Name, "", strOrEmpty(r.TermSIS), sisCourseStatus(r.WorkflowState), timeStr(r.StartAt), timeStr(r.EndAt)})
		} else {
			rows = append(rows, []string{uintStr(r.ID), strOrEmpty(r.SISCourseID), r.CourseCode, r.Name, uintPtrStr(r.TermID), strOrEmpty(r.TermSIS), sisCourseStatus(r.WorkflowState), timeStr(r.StartAt), timeStr(r.EndAt)})
		}
	}
	return reportFile{name: "courses.csv", rows: rows}, nil
}

func (s *AccountReportService) provSections(ctx context.Context, accountID uint, p AccountReportParams, sisOnly bool) (reportFile, error) {
	type row struct {
		ID            uint
		SISSectionID  *string
		CourseID      uint
		CourseSIS     *string
		Name          string
		WorkflowState string
	}
	where, args := courseFilterSQL(p)
	q := `SELECT s.id, s.sis_section_id, s.course_id, c.sis_course_id AS course_sis, s.name, s.workflow_state
	      FROM course_sections s JOIN courses c ON c.id = s.course_id WHERE ` + where
	if sisOnly {
		q += " AND s.sis_section_id IS NOT NULL"
	}
	if !p.IncludeDeleted {
		q += " AND s.workflow_state <> 'deleted'"
	}
	var rs []row
	if err := s.db.WithContext(ctx).Raw(q+" ORDER BY s.id", append([]interface{}{accountID}, args...)...).Scan(&rs).Error; err != nil {
		return reportFile{}, err
	}
	var rows [][]string
	if sisOnly {
		rows = append(rows, []string{"section_id", "course_id", "name", "status"})
	} else {
		rows = append(rows, []string{"paper_section_id", "section_id", "paper_course_id", "course_id", "name", "status"})
	}
	for _, r := range rs {
		status := "active"
		if r.WorkflowState == "deleted" {
			status = "deleted"
		}
		if sisOnly {
			rows = append(rows, []string{strOrEmpty(r.SISSectionID), strOrEmpty(r.CourseSIS), r.Name, status})
		} else {
			rows = append(rows, []string{uintStr(r.ID), strOrEmpty(r.SISSectionID), uintStr(r.CourseID), strOrEmpty(r.CourseSIS), r.Name, status})
		}
	}
	return reportFile{name: "sections.csv", rows: rows}, nil
}

func (s *AccountReportService) provEnrollments(ctx context.Context, accountID uint, p AccountReportParams, sisOnly bool) (reportFile, error) {
	type row struct {
		ID            uint
		CourseID      uint
		CourseSIS     *string
		UserID        uint
		UserSIS       *string
		Type          string
		Role          string
		RoleID        *uint
		SectionID     *uint
		SectionSIS    *string
		WorkflowState string
		AssocID       *uint
		AssocSIS      *string
	}
	where, args := courseFilterSQL(p)
	q := `SELECT e.id, e.course_id, c.sis_course_id AS course_sis, e.user_id, u.sis_user_id AS user_sis,
	             e.type, e.role, e.role_id, e.course_section_id AS section_id, s.sis_section_id AS section_sis,
	             e.workflow_state, e.associated_user_id AS assoc_id, au.sis_user_id AS assoc_sis
	      FROM enrollments e
	      JOIN courses c ON c.id = e.course_id
	      JOIN users u ON u.id = e.user_id
	      LEFT JOIN course_sections s ON s.id = e.course_section_id
	      LEFT JOIN users au ON au.id = e.associated_user_id
	      WHERE ` + where
	if sisOnly {
		q += " AND c.sis_course_id IS NOT NULL AND u.sis_user_id IS NOT NULL"
	}
	if !p.IncludeDeleted {
		q += " AND e.workflow_state <> 'deleted'"
	}
	var rs []row
	if err := s.db.WithContext(ctx).Raw(q+" ORDER BY e.id", append([]interface{}{accountID}, args...)...).Scan(&rs).Error; err != nil {
		return reportFile{}, err
	}
	var rows [][]string
	if sisOnly {
		rows = append(rows, []string{"course_id", "user_id", "role", "section_id", "status", "associated_user_id"})
	} else {
		rows = append(rows, []string{"paper_enrollment_id", "paper_course_id", "course_id", "paper_user_id", "user_id", "role", "role_id", "paper_section_id", "section_id", "status", "associated_user_id"})
	}
	for _, r := range rs {
		role := sisRoleForType[r.Type]
		if r.RoleID != nil && r.Role != "" && r.Role != r.Type {
			role = r.Role // custom role, by name (enrollments.csv semantics)
		}
		if sisOnly {
			rows = append(rows, []string{strOrEmpty(r.CourseSIS), strOrEmpty(r.UserSIS), role, strOrEmpty(r.SectionSIS), sisEnrollmentStatus(r.WorkflowState), strOrEmpty(r.AssocSIS)})
		} else {
			rows = append(rows, []string{uintStr(r.ID), uintStr(r.CourseID), strOrEmpty(r.CourseSIS), uintStr(r.UserID), strOrEmpty(r.UserSIS), role, uintPtrStr(r.RoleID), uintPtrStr(r.SectionID), strOrEmpty(r.SectionSIS), sisEnrollmentStatus(r.WorkflowState), strOrEmpty(r.AssocSIS)})
		}
	}
	return reportFile{name: "enrollments.csv", rows: rows}, nil
}

func (s *AccountReportService) provGroups(ctx context.Context, accountID uint, p AccountReportParams, sisOnly bool) (reportFile, error) {
	type row struct {
		ID            uint
		SISSourceID   *string
		CategorySIS   *string
		CourseSIS     *string
		Name          string
		WorkflowState string
	}
	q := `SELECT g.id, g.sis_source_id, gc.sis_source_id AS category_sis, c.sis_course_id AS course_sis, g.name, g.workflow_state
	      FROM groups g
	      JOIN group_categories gc ON gc.id = g.group_category_id
	      JOIN courses c ON g.context_type = 'Course' AND c.id = g.context_id
	      WHERE c.account_id = ?`
	if sisOnly {
		q += " AND g.sis_source_id IS NOT NULL"
	}
	if !p.IncludeDeleted {
		q += " AND g.workflow_state <> 'deleted'"
	}
	var rs []row
	if err := s.db.WithContext(ctx).Raw(q+" ORDER BY g.id", accountID).Scan(&rs).Error; err != nil {
		return reportFile{}, err
	}
	rows := [][]string{{"paper_group_id", "group_id", "group_category_id", "course_id", "name", "status"}}
	for _, r := range rs {
		status := "available"
		if r.WorkflowState == "deleted" {
			status = "deleted"
		}
		rows = append(rows, []string{uintStr(r.ID), strOrEmpty(r.SISSourceID), strOrEmpty(r.CategorySIS), strOrEmpty(r.CourseSIS), r.Name, status})
	}
	return reportFile{name: "groups.csv", rows: rows}, nil
}

func (s *AccountReportService) provGroupMemberships(ctx context.Context, accountID uint, sisOnly bool) (reportFile, error) {
	type row struct {
		GroupID  uint
		GroupSIS *string
		UserID   uint
		UserSIS  *string
		State    string
	}
	q := `SELECT gm.group_id, g.sis_source_id AS group_sis, gm.user_id, u.sis_user_id AS user_sis, gm.workflow_state AS state
	      FROM group_memberships gm
	      JOIN groups g ON g.id = gm.group_id
	      JOIN users u ON u.id = gm.user_id
	      JOIN courses c ON g.context_type = 'Course' AND c.id = g.context_id
	      WHERE c.account_id = ? AND u.account_id = ?`
	if sisOnly {
		q += " AND g.sis_source_id IS NOT NULL AND u.sis_user_id IS NOT NULL"
	}
	var rs []row
	if err := s.db.WithContext(ctx).Raw(q+" ORDER BY gm.id", accountID, accountID).Scan(&rs).Error; err != nil {
		return reportFile{}, err
	}
	rows := [][]string{{"paper_group_id", "group_id", "paper_user_id", "user_id", "status"}}
	for _, r := range rs {
		status := "accepted"
		if r.State == "deleted" {
			status = "deleted"
		}
		rows = append(rows, []string{uintStr(r.GroupID), strOrEmpty(r.GroupSIS), uintStr(r.UserID), strOrEmpty(r.UserSIS), status})
	}
	return reportFile{name: "group_memberships.csv", rows: rows}, nil
}

func (s *AccountReportService) provObservers(ctx context.Context, accountID uint, sisOnly bool) (reportFile, error) {
	type row struct {
		ObserverID  uint
		ObserverSIS *string
		StudentID   uint
		StudentSIS  *string
	}
	q := `SELECT l.observer_id, o.sis_user_id AS observer_sis, l.student_id, st.sis_user_id AS student_sis
	      FROM observer_links l
	      JOIN users o ON o.id = l.observer_id
	      JOIN users st ON st.id = l.student_id
	      WHERE l.account_id = ? AND l.workflow_state = 'active' AND o.account_id = ? AND st.account_id = ?`
	if sisOnly {
		q += " AND o.sis_user_id IS NOT NULL AND st.sis_user_id IS NOT NULL"
	}
	var rs []row
	if err := s.db.WithContext(ctx).Raw(q+" ORDER BY l.id", accountID, accountID, accountID).Scan(&rs).Error; err != nil {
		return reportFile{}, err
	}
	rows := [][]string{{"paper_observer_id", "observer_id", "paper_student_id", "student_id", "status"}}
	for _, r := range rs {
		rows = append(rows, []string{uintStr(r.ObserverID), strOrEmpty(r.ObserverSIS), uintStr(r.StudentID), strOrEmpty(r.StudentSIS), "active"})
	}
	return reportFile{name: "user_observers.csv", rows: rows}, nil
}

// ---------------------------------------------------------------------------
// grade export
// ---------------------------------------------------------------------------

func (s *AccountReportService) gradeExport(ctx context.Context, accountID uint, p AccountReportParams) (reportFile, error) {
	type row struct {
		EnrollmentID  uint
		UserID        uint
		UserName      string
		UserSIS       *string
		CourseID      uint
		CourseName    string
		CourseSIS     *string
		SectionID     *uint
		SectionName   *string
		SectionSIS    *string
		TermID        *uint
		TermName      *string
		TermSIS       *string
		WorkflowState string
	}
	where, args := courseFilterSQL(p)
	q := `SELECT e.id AS enrollment_id, e.user_id, u.name AS user_name, u.sis_user_id AS user_sis,
	             e.course_id, c.name AS course_name, c.sis_course_id AS course_sis,
	             e.course_section_id AS section_id, s.name AS section_name, s.sis_section_id AS section_sis,
	             c.enrollment_term_id AS term_id, t.name AS term_name, t.sis_term_id AS term_sis,
	             e.workflow_state
	      FROM enrollments e
	      JOIN courses c ON c.id = e.course_id
	      JOIN users u ON u.id = e.user_id
	      LEFT JOIN course_sections s ON s.id = e.course_section_id
	      LEFT JOIN enrollment_terms t ON t.id = c.enrollment_term_id
	      WHERE e.type = 'StudentEnrollment' AND ` + where
	if !p.IncludeDeleted {
		q += " AND e.workflow_state <> 'deleted'"
	}
	var rs []row
	if err := s.db.WithContext(ctx).Raw(q+" ORDER BY c.id, u.sortable_name, e.id", append([]interface{}{accountID}, args...)...).Scan(&rs).Error; err != nil {
		return reportFile{}, err
	}
	rows := [][]string{{"student name", "student id", "student sis", "course", "course id", "course sis", "section", "section id", "section sis", "term", "term id", "term sis", "current score", "final score", "current grade", "final grade", "enrollment state", "override score", "override grade"}}
	for _, r := range rs {
		var cur, fin, override *float64
		var curGrade, finGrade, overrideGrade string
		if s.grades != nil {
			if g, err := s.grades.GetStudentGrade(ctx, r.CourseID, r.UserID); err == nil && g != nil {
				// Nothing graded yet: blank, not 0.
				if !g.NoCurrentScore {
					c := g.CurrentScore
					cur = &c
					curGrade = g.CurrentGrade
				}
				if !g.NoFinalScore {
					f := g.FinalScore
					fin = &f
					finGrade = g.FinalGrade
				}
				override, overrideGrade = g.OverrideScore, g.OverrideGrade
			}
		}
		rows = append(rows, []string{
			r.UserName, uintStr(r.UserID), strOrEmpty(r.UserSIS),
			r.CourseName, uintStr(r.CourseID), strOrEmpty(r.CourseSIS),
			strOrEmpty(r.SectionName), uintPtrStr(r.SectionID), strOrEmpty(r.SectionSIS),
			strOrEmpty(r.TermName), uintPtrStr(r.TermID), strOrEmpty(r.TermSIS),
			scoreStr(cur), scoreStr(fin), curGrade, finGrade, sisEnrollmentStatus(r.WorkflowState),
			scoreStr(override), overrideGrade,
		})
	}
	return reportFile{name: "grade_export.csv", rows: rows}, nil
}

// ---------------------------------------------------------------------------
// activity / course-state reports
// ---------------------------------------------------------------------------

func (s *AccountReportService) lastUserAccess(ctx context.Context, accountID uint, p AccountReportParams) (reportFile, error) {
	type row struct {
		ID           uint
		Name         string
		SISUserID    *string
		LoginID      string
		Email        string
		LastLogin    *time.Time
		LastActivity *time.Time
	}
	q := `SELECT u.id, u.name, u.sis_user_id, u.login_id, u.email,
	             (SELECT MAX(a.created_at) FROM audit_logs a WHERE a.user_id = u.id AND a.event_type = 'auth.login_succeeded') AS last_login,
	             (SELECT MAX(e.last_activity_at) FROM enrollments e JOIN courses c ON c.id = e.course_id
	                WHERE e.user_id = u.id AND c.account_id = ?` + termClause(p) + `) AS last_activity
	      FROM users u WHERE u.account_id = ?`
	args := []interface{}{accountID}
	if p.EnrollmentTermID != nil {
		args = append(args, *p.EnrollmentTermID)
		q += ` AND EXISTS (SELECT 1 FROM enrollments e2 JOIN courses c2 ON c2.id = e2.course_id
		                   WHERE e2.user_id = u.id AND c2.enrollment_term_id = ?)`
		args = append(args, accountID, *p.EnrollmentTermID)
	} else {
		args = append(args, accountID)
	}
	var rs []row
	if err := s.db.WithContext(ctx).Raw(q+" ORDER BY u.sortable_name, u.id", args...).Scan(&rs).Error; err != nil {
		return reportFile{}, err
	}
	rows := [][]string{{"user id", "user sis id", "user name", "login id", "email", "last sign-in", "last course activity", "last access"}}
	for _, r := range rs {
		last := r.LastLogin
		if r.LastActivity != nil && (last == nil || r.LastActivity.After(*last)) {
			last = r.LastActivity
		}
		rows = append(rows, []string{uintStr(r.ID), strOrEmpty(r.SISUserID), r.Name, r.LoginID, r.Email, timeStr(r.LastLogin), timeStr(r.LastActivity), timeStr(last)})
	}
	return reportFile{name: "last_user_access.csv", rows: rows}, nil
}

func termClause(p AccountReportParams) string {
	if p.EnrollmentTermID != nil {
		return " AND c.enrollment_term_id = ?"
	}
	return ""
}

func (s *AccountReportService) coursesByState(ctx context.Context, accountID uint, p AccountReportParams, states []string, since *time.Time) ([]models.Course, error) {
	q := s.db.WithContext(ctx).Where("account_id = ? AND workflow_state IN ?", accountID, states)
	if p.EnrollmentTermID != nil {
		q = q.Where("enrollment_term_id = ?", *p.EnrollmentTermID)
	}
	if since != nil {
		q = q.Where("updated_at >= ?", *since)
	}
	var cs []models.Course
	err := q.Order("id").Find(&cs).Error
	return cs, err
}

func (s *AccountReportService) unpublishedCourses(ctx context.Context, accountID uint, p AccountReportParams) (reportFile, error) {
	cs, err := s.coursesByState(ctx, accountID, p, []string{"claimed", "created", "unpublished"}, nil)
	if err != nil {
		return reportFile{}, err
	}
	rows := [][]string{{"course id", "course sis id", "short name", "long name", "term id", "created at"}}
	for _, c := range cs {
		rows = append(rows, []string{uintStr(c.ID), strOrEmpty(c.SISCourseID), c.CourseCode, c.Name, uintPtrStr(c.EnrollmentTermID), c.CreatedAt.UTC().Format(time.RFC3339)})
	}
	return reportFile{name: "unpublished_courses.csv", rows: rows}, nil
}

func (s *AccountReportService) deletedCourses(ctx context.Context, accountID uint) (reportFile, error) {
	since := time.Now().AddDate(0, 0, -30)
	cs, err := s.coursesByState(ctx, accountID, AccountReportParams{}, []string{"deleted"}, &since)
	if err != nil {
		return reportFile{}, err
	}
	rows := [][]string{{"course id", "course sis id", "short name", "long name", "term id", "deleted at"}}
	for _, c := range cs {
		rows = append(rows, []string{uintStr(c.ID), strOrEmpty(c.SISCourseID), c.CourseCode, c.Name, uintPtrStr(c.EnrollmentTermID), c.UpdatedAt.UTC().Format(time.RFC3339)})
	}
	return reportFile{name: "recently_deleted_courses.csv", rows: rows}, nil
}

func (s *AccountReportService) zeroActivity(ctx context.Context, accountID uint, p AccountReportParams) (reportFile, error) {
	type row struct {
		UserID     uint
		UserName   string
		UserSIS    *string
		CourseID   uint
		CourseName string
		CourseSIS  *string
		SectionID  *uint
	}
	where, args := courseFilterSQL(AccountReportParams{EnrollmentTermID: p.EnrollmentTermID})
	q := `SELECT e.user_id, u.name AS user_name, u.sis_user_id AS user_sis, e.course_id, c.name AS course_name,
	             c.sis_course_id AS course_sis, e.course_section_id AS section_id
	      FROM enrollments e JOIN courses c ON c.id = e.course_id JOIN users u ON u.id = e.user_id
	      WHERE e.type = 'StudentEnrollment' AND e.workflow_state = 'active' AND e.last_activity_at IS NULL AND ` + where
	var rs []row
	if err := s.db.WithContext(ctx).Raw(q+" ORDER BY c.id, u.sortable_name", append([]interface{}{accountID}, args...)...).Scan(&rs).Error; err != nil {
		return reportFile{}, err
	}
	rows := [][]string{{"user id", "user sis id", "user name", "course id", "course sis id", "course name", "section id"}}
	for _, r := range rs {
		rows = append(rows, []string{uintStr(r.UserID), strOrEmpty(r.UserSIS), r.UserName, uintStr(r.CourseID), strOrEmpty(r.CourseSIS), r.CourseName, uintPtrStr(r.SectionID)})
	}
	return reportFile{name: "zero_activity.csv", rows: rows}, nil
}
