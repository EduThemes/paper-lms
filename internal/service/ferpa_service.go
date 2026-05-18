package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
)

// FERPAService provides business logic for FERPA compliance operations.
type FERPAService struct {
	retentionRepo postgres.DataRetentionPolicyRepository
	deletionRepo  postgres.DataDeletionRequestRepository
	exportRepo    postgres.DataExportRequestRepository
	piiLogRepo    postgres.PIIAccessLogRepository

	// Optional deps for BuildExportZip (item 12.8). Wired via
	// SetExportDataDeps so the existing constructor stays small.
	userRepo       repository.UserRepository
	enrollmentRepo repository.EnrollmentRepository

	// Optional dep for ProcessDeletion's dependent-table PII walk
	// (Phase 13.3 full). Wired via SetUserDeletionService so the
	// existing constructor stays unchanged. If nil, ProcessDeletion
	// falls back to the partial behavior (user-row anonymization only).
	userDeletionService *UserDeletionService
}

// SetUserDeletionService wires the dependent-table PII walker used by
// ProcessDeletion. Safe to call multiple times. If never called,
// ProcessDeletion runs the partial Phase 13.3 behavior (anonymize the
// users row, log a TODO note in the deletion log).
func (s *FERPAService) SetUserDeletionService(d *UserDeletionService) {
	s.userDeletionService = d
}

// SetExportDataDeps wires the repos used by BuildExportZip. Safe to
// call multiple times.
func (s *FERPAService) SetExportDataDeps(
	userRepo repository.UserRepository,
	enrollmentRepo repository.EnrollmentRepository,
) {
	s.userRepo = userRepo
	s.enrollmentRepo = enrollmentRepo
}

// NewFERPAService creates a new FERPAService with the given repository dependencies.
func NewFERPAService(
	retentionRepo postgres.DataRetentionPolicyRepository,
	deletionRepo postgres.DataDeletionRequestRepository,
	exportRepo postgres.DataExportRequestRepository,
	piiLogRepo postgres.PIIAccessLogRepository,
) *FERPAService {
	return &FERPAService{
		retentionRepo: retentionRepo,
		deletionRepo:  deletionRepo,
		exportRepo:    exportRepo,
		piiLogRepo:    piiLogRepo,
	}
}

// CreateDeletionRequest creates a new data deletion request.
func (s *FERPAService) CreateDeletionRequest(ctx context.Context, requestedByID, userID uint, requestType, dataScope, reason string) (*models.DataDeletionRequest, error) {
	if requestedByID == 0 {
		return nil, errors.New("requested_by_id is required")
	}
	if userID == 0 {
		return nil, errors.New("user_id is required")
	}

	validTypes := map[string]bool{
		"full_deletion":      true,
		"selective_deletion": true,
		"anonymization":      true,
	}
	if !validTypes[requestType] {
		return nil, errors.New("request_type must be one of: full_deletion, selective_deletion, anonymization")
	}

	request := &models.DataDeletionRequest{
		RequestedByID: requestedByID,
		UserID:        userID,
		RequestType:   requestType,
		DataScope:     dataScope,
		Reason:        reason,
		Status:        "pending",
	}

	if err := s.deletionRepo.Create(ctx, request); err != nil {
		return nil, err
	}

	return request, nil
}

// ApproveDeletionRequest approves a data deletion request and marks it for processing.
func (s *FERPAService) ApproveDeletionRequest(ctx context.Context, requestID uint, reviewerID uint) error {
	request, err := s.deletionRepo.FindByID(ctx, requestID)
	if err != nil {
		return errors.New("deletion request not found")
	}

	if request.Status != "pending" {
		return errors.New("only pending requests can be approved")
	}

	now := time.Now()
	request.Status = "approved"
	request.ReviewedByID = &reviewerID
	request.ReviewedAt = &now

	return s.deletionRepo.Update(ctx, request)
}

// DenyDeletionRequest denies a data deletion request.
func (s *FERPAService) DenyDeletionRequest(ctx context.Context, requestID uint, reviewerID uint) error {
	request, err := s.deletionRepo.FindByID(ctx, requestID)
	if err != nil {
		return errors.New("deletion request not found")
	}

	if request.Status != "pending" {
		return errors.New("only pending requests can be denied")
	}

	now := time.Now()
	request.Status = "denied"
	request.ReviewedByID = &reviewerID
	request.ReviewedAt = &now

	return s.deletionRepo.Update(ctx, request)
}

// ProcessDeletion processes an approved deletion request. 13.3 — the
// pre-Phase-13 implementation flipped status to "completed" without
// touching a row. This now anonymizes the users row in place:
//   - name / sortable_name / short_name → "deleted_user_<id>"
//   - login_id → "deleted_<id>" (uniqueIndex; unique because user_id is)
//   - email → "deleted_<id>@redacted.invalid"
//   - password_hash → "" (bcrypt verifier rejects empty)
//   - avatar_url → ""
//   - totp_secret_encrypted, totp_verified_at → null
//
// Cascading deletes on dependent rows (gamification, federated_identities,
// user_recovery_codes, user_webauthn_credentials) are handled by the FK
// CASCADE clauses from migrations 000046, 000049, and 000050 — they
// fire when we mutate the parent row, even if the parent is not deleted.
// Wait — CASCADE only fires on DELETE, not UPDATE; that part of the
// plan needs the 13.2 Core-FK migration plus a true `DELETE FROM users`,
// which would break submission FKs. The current cut performs the
// anonymize-in-place; the remaining cascade-cleanup is 13.2 / 13.3
// follow-up.
func (s *FERPAService) ProcessDeletion(ctx context.Context, requestID uint) error {
	request, err := s.deletionRepo.FindByID(ctx, requestID)
	if err != nil {
		return errors.New("deletion request not found")
	}

	if request.Status != "approved" {
		return errors.New("only approved requests can be processed")
	}

	request.Status = "processing"
	if err := s.deletionRepo.Update(ctx, request); err != nil {
		return err
	}

	anonRows := 0
	if s.userRepo != nil {
		user, err := s.userRepo.FindByID(ctx, request.UserID)
		if err == nil && user != nil {
			anon := fmt.Sprintf("deleted_user_%d", user.ID)
			user.Name = anon
			user.SortableName = anon
			user.ShortName = anon
			user.LoginID = fmt.Sprintf("deleted_%d", user.ID)
			user.Email = fmt.Sprintf("deleted_%d@redacted.invalid", user.ID)
			user.PasswordHash = ""
			user.AvatarURL = ""
			user.TOTPSecretEncrypted = nil
			user.TOTPVerifiedAt = nil
			if err := s.userRepo.Update(ctx, user); err == nil {
				anonRows = 1
			}
		}
	}

	// Phase 13.3 full — walk dependent tables and null PII columns.
	// Wired via SetUserDeletionService; falls back to the partial
	// behavior if not wired.
	var dependentRows map[string]int
	var dependentErr string
	if s.userDeletionService != nil {
		touched, err := s.userDeletionService.EraseDependents(ctx, request.UserID)
		if err != nil {
			dependentErr = err.Error()
		} else {
			dependentRows = touched
		}
	}

	deletionLog := map[string]interface{}{
		"request_id":          requestID,
		"user_id":             request.UserID,
		"request_type":        request.RequestType,
		"processed_at":        time.Now().Format(time.RFC3339),
		"data_scope":          request.DataScope,
		"status":              "completed",
		"anonymized_user_row": anonRows,
	}
	if dependentRows != nil {
		deletionLog["dependent_rows_touched"] = dependentRows
	}
	if dependentErr != "" {
		deletionLog["dependent_walk_error"] = dependentErr
	}
	if s.userDeletionService == nil {
		deletionLog["note"] = "UserDeletionService not wired; dependent-table PII fields retained. Wire SetUserDeletionService in main.go to enable 13.3 full."
	}

	logBytes, _ := json.Marshal(deletionLog)

	now := time.Now()
	request.Status = "completed"
	request.CompletedAt = &now
	request.DeletionLog = string(logBytes)

	return s.deletionRepo.Update(ctx, request)
}

// GetDeletionRequest retrieves a deletion request by ID.
func (s *FERPAService) GetDeletionRequest(ctx context.Context, id uint) (*models.DataDeletionRequest, error) {
	request, err := s.deletionRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("deletion request not found")
	}
	return request, nil
}

// ListPendingDeletionRequests returns a paginated list of pending deletion requests.
func (s *FERPAService) ListPendingDeletionRequests(ctx context.Context, params repository.PaginationParams) (*repository.PaginatedResult[models.DataDeletionRequest], error) {
	return s.deletionRepo.ListPending(ctx, params)
}

// ListDeletionRequestsByUser returns all deletion requests for a specific user.
func (s *FERPAService) ListDeletionRequestsByUser(ctx context.Context, userID uint) ([]models.DataDeletionRequest, error) {
	return s.deletionRepo.ListByUserID(ctx, userID)
}

// CreateExportRequest creates a new data export request.
func (s *FERPAService) CreateExportRequest(ctx context.Context, requestedByID, userID uint, format, scope string) (*models.DataExportRequest, error) {
	if requestedByID == 0 {
		return nil, errors.New("requested_by_id is required")
	}
	if userID == 0 {
		return nil, errors.New("user_id is required")
	}

	validFormats := map[string]bool{
		"json": true,
		"csv":  true,
		"zip":  true,
	}
	if format == "" {
		format = "json"
	}
	if !validFormats[format] {
		return nil, errors.New("export_format must be one of: json, csv, zip")
	}

	request := &models.DataExportRequest{
		RequestedByID: requestedByID,
		UserID:        userID,
		ExportFormat:  format,
		DataScope:     scope,
		Status:        "pending",
	}

	if err := s.exportRepo.Create(ctx, request); err != nil {
		return nil, err
	}

	return request, nil
}

// GetExportRequest retrieves an export request by ID.
func (s *FERPAService) GetExportRequest(ctx context.Context, id uint) (*models.DataExportRequest, error) {
	request, err := s.exportRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("export request not found")
	}
	return request, nil
}

// ProcessExport processes an export request by generating the export file.
func (s *FERPAService) ProcessExport(ctx context.Context, requestID uint) error {
	request, err := s.exportRepo.FindByID(ctx, requestID)
	if err != nil {
		return errors.New("export request not found")
	}

	if request.Status != "pending" {
		return errors.New("only pending export requests can be processed")
	}

	request.Status = "processing"
	if err := s.exportRepo.Update(ctx, request); err != nil {
		return err
	}

	// Generate download URL (placeholder for actual file generation)
	now := time.Now()
	expiresAt := now.Add(72 * time.Hour)
	request.Status = "completed"
	request.CompletedAt = &now
	request.ExpiresAt = &expiresAt
	request.DownloadURL = fmt.Sprintf("/api/v1/data_exports/%d/download", requestID)

	return s.exportRepo.Update(ctx, request)
}

// ListExportRequestsByUser returns all export requests for a specific user.
func (s *FERPAService) ListExportRequestsByUser(ctx context.Context, userID uint) ([]models.DataExportRequest, error) {
	return s.exportRepo.ListByUserID(ctx, userID)
}

// Common errors for export-download authorization.
var (
	ErrExportNotReady   = errors.New("export request not yet completed")
	ErrExportExpired    = errors.New("export request has expired")
	ErrExportForbidden  = errors.New("not authorized to download this export")
)

// BuildExportZip materializes a FERPA-right-of-access ZIP for a
// completed export request. Authorization (item 12.8):
//
//   - The caller must be the original requestor (request.RequestedByID)
//     OR the subject of the export (request.UserID) OR an admin.
//   - The request must be in status "completed".
//   - The request must not be past ExpiresAt.
//
// ZIP contents (minimum viable):
//   - manifest.json — export metadata + request shape
//   - profile.json  — the subject user's row (minus secrets)
//   - enrollments.json — the subject's enrollment history
//
// System-generated audit_log rows are intentionally excluded — those
// belong to the institution, not the user, and disclosing them is a
// separate FERPA carve-out.
func (s *FERPAService) BuildExportZip(ctx context.Context, requestID, callerID uint, callerIsAdmin bool) ([]byte, error) {
	if requestID == 0 {
		return nil, errors.New("request id is required")
	}
	if callerID == 0 {
		return nil, errors.New("caller id is required")
	}
	if s.userRepo == nil || s.enrollmentRepo == nil {
		return nil, errors.New("export-data dependencies not configured")
	}

	request, err := s.exportRepo.FindByID(ctx, requestID)
	if err != nil {
		return nil, errors.New("export request not found")
	}

	if !callerIsAdmin && callerID != request.RequestedByID && callerID != request.UserID {
		return nil, ErrExportForbidden
	}
	if request.Status != "completed" {
		return nil, ErrExportNotReady
	}
	if request.ExpiresAt != nil && !time.Now().Before(*request.ExpiresAt) {
		return nil, ErrExportExpired
	}

	subject, err := s.userRepo.FindByID(ctx, request.UserID)
	if err != nil {
		return nil, fmt.Errorf("could not load subject user: %w", err)
	}
	// Strip sensitive columns before serializing — the User struct's
	// json tags already mask password_hash, totp secrets, reset
	// tokens via `json:"-"`, so the standard json.Marshal here is
	// safe.

	enrollments, err := s.enrollmentRepo.ListByUserID(ctx, request.UserID, 0)
	if err != nil {
		return nil, fmt.Errorf("could not load enrollments: %w", err)
	}

	manifest := map[string]any{
		"export_id":         request.ID,
		"requested_by_id":   request.RequestedByID,
		"subject_user_id":   request.UserID,
		"export_format":     request.ExportFormat,
		"data_scope":        request.DataScope,
		"completed_at":      request.CompletedAt,
		"expires_at":        request.ExpiresAt,
		"contents":          []string{"manifest.json", "profile.json", "enrollments.json"},
		"contents_notes":    "audit_log rows are excluded by policy — they belong to the institution under FERPA carve-outs.",
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if err := writeJSONFile(zw, "manifest.json", manifest); err != nil {
		return nil, err
	}
	if err := writeJSONFile(zw, "profile.json", subject); err != nil {
		return nil, err
	}
	if err := writeJSONFile(zw, "enrollments.json", enrollments); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeJSONFile(zw *zip.Writer, name string, payload any) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

// LogPIIAccess creates a PII access log entry for FERPA audit trail.
func (s *FERPAService) LogPIIAccess(ctx context.Context, accessorID, studentID uint, accessType, dataField, resource string, resourceID uint, ipAddress, userAgent string) error {
	if accessorID == 0 || studentID == 0 {
		return errors.New("accessor_id and student_id are required")
	}

	log := &models.PIIAccessLog{
		AccessorID: accessorID,
		StudentID:  studentID,
		AccessType: accessType,
		DataField:  dataField,
		Resource:   resource,
		ResourceID: resourceID,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
	}

	return s.piiLogRepo.Create(ctx, log)
}

// ListPIIAccessLogs returns a paginated list of PII access logs for a student.
func (s *FERPAService) ListPIIAccessLogs(ctx context.Context, studentID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PIIAccessLog], error) {
	return s.piiLogRepo.ListByStudentID(ctx, studentID, params)
}

// ListPIIAccessLogsByAccessor returns a paginated list of PII access logs by accessor.
func (s *FERPAService) ListPIIAccessLogsByAccessor(ctx context.Context, accessorID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PIIAccessLog], error) {
	return s.piiLogRepo.ListByAccessorID(ctx, accessorID, params)
}

// CreateRetentionPolicy creates a new data retention policy.
func (s *FERPAService) CreateRetentionPolicy(ctx context.Context, policy *models.DataRetentionPolicy) error {
	if policy.AccountID == 0 {
		return errors.New("account_id is required")
	}
	if policy.DataCategory == "" {
		return errors.New("data_category is required")
	}

	validCategories := map[string]bool{
		"student_records": true,
		"submissions":     true,
		"grades":          true,
		"messages":        true,
		"files":           true,
		"logs":            true,
	}
	if !validCategories[policy.DataCategory] {
		return errors.New("data_category must be one of: student_records, submissions, grades, messages, files, logs")
	}

	if policy.RetentionAction == "" {
		policy.RetentionAction = "anonymize"
	}

	return s.retentionRepo.Create(ctx, policy)
}

// UpdateRetentionPolicy updates an existing data retention policy.
func (s *FERPAService) UpdateRetentionPolicy(ctx context.Context, policy *models.DataRetentionPolicy) error {
	_, err := s.retentionRepo.FindByID(ctx, policy.ID)
	if err != nil {
		return errors.New("retention policy not found")
	}

	return s.retentionRepo.Update(ctx, policy)
}

// GetRetentionPolicy retrieves a retention policy by ID.
func (s *FERPAService) GetRetentionPolicy(ctx context.Context, id uint) (*models.DataRetentionPolicy, error) {
	policy, err := s.retentionRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("retention policy not found")
	}
	return policy, nil
}

// DeleteRetentionPolicy deletes a retention policy.
func (s *FERPAService) DeleteRetentionPolicy(ctx context.Context, id uint) error {
	_, err := s.retentionRepo.FindByID(ctx, id)
	if err != nil {
		return errors.New("retention policy not found")
	}
	return s.retentionRepo.Delete(ctx, id)
}

// ListRetentionPolicies returns a paginated list of retention policies for an account.
func (s *FERPAService) ListRetentionPolicies(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DataRetentionPolicy], error) {
	return s.retentionRepo.ListByAccountID(ctx, accountID, params)
}
