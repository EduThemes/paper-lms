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

// isInstructor checks whether a user has an instructor or TA role in the given course.
func (s *DocumentAnnotationService) isInstructor(ctx context.Context, userID, courseID uint) (bool, error) {
	enrollment, err := s.enrollmentRepo.FindByUserAndCourse(ctx, userID, courseID)
	if err != nil {
		return false, nil
	}
	return enrollment.Type == "TeacherEnrollment" || enrollment.Type == "TaEnrollment", nil
}

// GetAnnotation returns a single annotation by ID with its replies preloaded.
func (s *DocumentAnnotationService) GetAnnotation(ctx context.Context, annotationID uint) (*models.DocumentAnnotation, error) {
	annotation, err := s.annotationRepo.FindByID(ctx, annotationID)
	if err != nil {
		return nil, errors.New("annotation not found")
	}
	if annotation.WorkflowState == "deleted" {
		return nil, errors.New("annotation not found")
	}
	return annotation, nil
}

// CreateAnnotation creates a new annotation on a submission.
// Instructors can annotate any submission. Students can only annotate their own submissions.
func (s *DocumentAnnotationService) CreateAnnotation(ctx context.Context, annotation *models.DocumentAnnotation, courseID uint) error {
	// Validate submission exists
	submission, err := s.submissionRepo.FindByID(ctx, annotation.SubmissionID)
	if err != nil {
		return errors.New("submission not found")
	}

	// Permission check
	instructor, err := s.isInstructor(ctx, annotation.UserID, courseID)
	if err != nil {
		return err
	}

	if !instructor {
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
	annotation, err := s.annotationRepo.FindByID(ctx, annotationID)
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

// DeleteAnnotation soft-deletes an annotation. Owner or instructor can delete.
func (s *DocumentAnnotationService) DeleteAnnotation(ctx context.Context, annotationID uint, userID uint, courseID uint) error {
	annotation, err := s.annotationRepo.FindByID(ctx, annotationID)
	if err != nil {
		return errors.New("annotation not found")
	}

	if annotation.UserID != userID {
		// Check if the user is an instructor
		instructor, err := s.isInstructor(ctx, userID, courseID)
		if err != nil {
			return err
		}
		if !instructor {
			return errors.New("only the annotation owner or an instructor can delete this annotation")
		}
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
	annotation, err := s.annotationRepo.FindByID(ctx, annotationID)
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
	annotation, err := s.annotationRepo.FindByID(ctx, annotationID)
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
func (s *DocumentAnnotationService) ReplyToAnnotation(ctx context.Context, parentAnnotationID uint, reply *models.DocumentAnnotation, courseID uint) error {
	parent, err := s.annotationRepo.FindByID(ctx, parentAnnotationID)
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

	// Permission check: instructors can reply to any annotation, students to their own submission's annotations
	submission, err := s.submissionRepo.FindByID(ctx, reply.SubmissionID)
	if err != nil {
		return errors.New("submission not found")
	}

	instructor, err := s.isInstructor(ctx, reply.UserID, courseID)
	if err != nil {
		return err
	}

	if !instructor && submission.UserID != reply.UserID {
		return errors.New("you do not have permission to reply to this annotation")
	}

	return s.annotationRepo.Create(ctx, reply)
}
