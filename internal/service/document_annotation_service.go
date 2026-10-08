package service

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type DocumentAnnotationService struct {
	annotationRepo repository.DocumentAnnotationRepository
	submissionRepo repository.SubmissionRepository
	attachmentRepo repository.AttachmentRepository
	enrollmentRepo repository.EnrollmentRepository
}

func NewDocumentAnnotationService(
	annotationRepo repository.DocumentAnnotationRepository,
	submissionRepo repository.SubmissionRepository,
	attachmentRepo repository.AttachmentRepository,
	enrollmentRepo repository.EnrollmentRepository,
) *DocumentAnnotationService {
	return &DocumentAnnotationService{
		annotationRepo: annotationRepo,
		submissionRepo: submissionRepo,
		attachmentRepo: attachmentRepo,
		enrollmentRepo: enrollmentRepo,
	}
}

// GetAnnotation returns a single annotation by ID with its replies preloaded.
func (s *DocumentAnnotationService) GetAnnotation(ctx context.Context, annotationID uint) (*models.DocumentAnnotation, error) {
	annotation, err := s.annotationRepo.FindByID(ctx, annotationID, 0)
	if err != nil {
		return nil, errors.New("annotation not found")
	}
	if annotation.WorkflowState == "deleted" {
		return nil, errors.New("annotation not found")
	}
	return annotation, nil
}

// CreateAnnotation creates a new annotation on a submission.
// Course staff can annotate any submission; everyone else only their own.
// callerIsStaff MUST be resolved by the caller against the course the
// submission belongs to (never a client-supplied course id).
func (s *DocumentAnnotationService) CreateAnnotation(ctx context.Context, annotation *models.DocumentAnnotation, callerIsStaff bool) error {
	// Validate submission exists (the handler already loaded it tenant-scoped).
	submission, err := s.submissionRepo.FindByID(ctx, annotation.SubmissionID, 0)
	if err != nil {
		return errors.New("submission not found")
	}

	if !callerIsStaff {
		// Students can only annotate their own submission
		if submission.UserID != annotation.UserID {
			return errors.New("you do not have permission to annotate this submission")
		}
	}

	// Validate annotation type
	validTypes := map[string]bool{
		"highlight":     true,
		"comment":       true,
		"strikethrough": true,
		"freehand":      true,
		"point":         true,
	}
	if !validTypes[annotation.AnnotationType] {
		return errors.New("invalid annotation type")
	}

	annotation.WorkflowState = "active"
	return s.annotationRepo.Create(ctx, annotation)
}

// UpdateAnnotation updates an existing annotation. Only the owner can edit their own annotation.
func (s *DocumentAnnotationService) UpdateAnnotation(ctx context.Context, annotationID uint, userID uint, updates map[string]interface{}) (*models.DocumentAnnotation, error) {
	annotation, err := s.annotationRepo.FindByID(ctx, annotationID, 0)
	if err != nil {
		return nil, errors.New("annotation not found")
	}

	if annotation.UserID != userID {
		return nil, errors.New("only the annotation owner can edit it")
	}

	if annotation.WorkflowState == "deleted" {
		return nil, errors.New("cannot update a deleted annotation")
	}

	// Apply allowed updates
	if content, ok := updates["content"].(string); ok {
		annotation.Content = content
	}
	if color, ok := updates["color"].(string); ok {
		annotation.Color = color
	}
	if selStart, ok := updates["selection_start"].(float64); ok {
		annotation.SelectionStart = int(selStart)
	}
	if selEnd, ok := updates["selection_end"].(float64); ok {
		annotation.SelectionEnd = int(selEnd)
	}
	if x, ok := updates["x"].(float64); ok {
		annotation.X = x
	}
	if y, ok := updates["y"].(float64); ok {
		annotation.Y = y
	}
	if width, ok := updates["width"].(float64); ok {
		annotation.Width = width
	}
	if height, ok := updates["height"].(float64); ok {
		annotation.Height = height
	}
	if pathData, ok := updates["path_data"].(string); ok {
		annotation.PathData = pathData
	}

	if err := s.annotationRepo.Update(ctx, annotation); err != nil {
		return nil, err
	}

	return annotation, nil
}

// DeleteAnnotation soft-deletes an annotation. Authorization (owner, or an
// instructor of the annotated submission's course) is the handler's job: it
// resolves the course from the annotation itself, tenant-scoped.
func (s *DocumentAnnotationService) DeleteAnnotation(ctx context.Context, annotationID uint) error {
	if _, err := s.annotationRepo.FindByID(ctx, annotationID, 0); err != nil {
		return errors.New("annotation not found")
	}

	return s.annotationRepo.Delete(ctx, annotationID)
}

// ListAnnotations returns paginated annotations for a submission (top-level only, with replies preloaded).
func (s *DocumentAnnotationService) ListAnnotations(ctx context.Context, submissionID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DocumentAnnotation], error) {
	return s.annotationRepo.ListBySubmissionID(ctx, submissionID, params)
}

// ListAnnotationsByPage returns annotations for a specific page of a submission.
func (s *DocumentAnnotationService) ListAnnotationsByPage(ctx context.Context, submissionID uint, pageNumber int) ([]models.DocumentAnnotation, error) {
	return s.annotationRepo.ListBySubmissionAndPage(ctx, submissionID, pageNumber)
}

// ResolveAnnotation marks an annotation as resolved.
func (s *DocumentAnnotationService) ResolveAnnotation(ctx context.Context, annotationID uint, resolvedByUserID uint) error {
	annotation, err := s.annotationRepo.FindByID(ctx, annotationID, 0)
	if err != nil {
		return errors.New("annotation not found")
	}

	if annotation.WorkflowState == "deleted" {
		return errors.New("cannot resolve a deleted annotation")
	}

	return s.annotationRepo.Resolve(ctx, annotationID, resolvedByUserID)
}

// UnresolveAnnotation removes the resolved status from an annotation.
func (s *DocumentAnnotationService) UnresolveAnnotation(ctx context.Context, annotationID uint) error {
	annotation, err := s.annotationRepo.FindByID(ctx, annotationID, 0)
	if err != nil {
		return errors.New("annotation not found")
	}

	if annotation.WorkflowState != "resolved" {
		return errors.New("annotation is not resolved")
	}

	return s.annotationRepo.Unresolve(ctx, annotationID)
}

// AnnotationSummary holds counts by annotation type and resolved status.
type AnnotationSummary struct {
	TotalCount    int64            `json:"total_count"`
	ByType        map[string]int64 `json:"by_type"`
	ResolvedCount int64            `json:"resolved_count"`
	ActiveCount   int64            `json:"active_count"`
}

// GetAnnotationSummary returns counts by type and resolved status for a submission.
func (s *DocumentAnnotationService) GetAnnotationSummary(ctx context.Context, submissionID uint) (*AnnotationSummary, error) {
	total, err := s.annotationRepo.CountBySubmissionID(ctx, submissionID)
	if err != nil {
		return nil, err
	}

	// Get all annotations to compute summary (using a large page)
	result, err := s.annotationRepo.ListBySubmissionID(ctx, submissionID, repository.PaginationParams{Page: 1, PerPage: 10000})
	if err != nil {
		return nil, err
	}

	summary := &AnnotationSummary{
		TotalCount: total,
		ByType:     make(map[string]int64),
	}

	for _, a := range result.Items {
		summary.ByType[a.AnnotationType]++
		if a.WorkflowState == "resolved" {
			summary.ResolvedCount++
		} else {
			summary.ActiveCount++
		}
		// Count replies
		for range a.Replies {
			summary.TotalCount++
		}
	}

	return summary, nil
}

// ReplyToAnnotation creates an annotation as a reply to an existing annotation.
// Authorization (course staff or the submission's author, on the course
// derived from the annotation) is the handler's job.
func (s *DocumentAnnotationService) ReplyToAnnotation(ctx context.Context, parentAnnotationID uint, reply *models.DocumentAnnotation) error {
	parent, err := s.annotationRepo.FindByID(ctx, parentAnnotationID, 0)
	if err != nil {
		return errors.New("parent annotation not found")
	}

	if parent.WorkflowState == "deleted" {
		return errors.New("cannot reply to a deleted annotation")
	}

	// Inherit submission context from parent
	reply.SubmissionID = parent.SubmissionID
	reply.PageNumber = parent.PageNumber
	reply.ParentAnnotationID = &parentAnnotationID
	reply.AnnotationType = "comment" // replies are always comments
	reply.WorkflowState = "active"

	return s.annotationRepo.Create(ctx, reply)
}
