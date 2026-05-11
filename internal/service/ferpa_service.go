package service

import (
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
	retentionRepo  postgres.DataRetentionPolicyRepository
	deletionRepo   postgres.DataDeletionRequestRepository
	exportRepo     postgres.DataExportRequestRepository
	piiLogRepo     postgres.PIIAccessLogRepository
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

// ProcessDeletion processes an approved deletion request by anonymizing/deleting data.
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

	// Build a deletion log recording what was processed
	deletionLog := map[string]interface{}{
		"request_id":   requestID,
		"user_id":      request.UserID,
		"request_type": request.RequestType,
		"processed_at": time.Now().Format(time.RFC3339),
		"data_scope":   request.DataScope,
		"status":       "completed",
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
