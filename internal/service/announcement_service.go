package service

import (
	"context"
	"errors"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	repoPostgres "github.com/EduThemes/paper-lms/internal/repository/postgres"
)

// AnnouncementStats holds read/acknowledged counts for an announcement.
type AnnouncementStats struct {
	ReadCount         int64 `json:"read_count"`
	AcknowledgedCount int64 `json:"acknowledged_count"`
	TotalAudience     int64 `json:"total_audience"`
}

// AnnouncementService provides business logic for announcements.
type AnnouncementService struct {
	announcementRepo repoPostgres.AnnouncementRepository
	receiptRepo      repoPostgres.AnnouncementReadReceiptRepository
	enrollmentRepo   repository.EnrollmentRepository
}

// NewAnnouncementService creates a new AnnouncementService.
func NewAnnouncementService(
	announcementRepo repoPostgres.AnnouncementRepository,
	receiptRepo repoPostgres.AnnouncementReadReceiptRepository,
	enrollmentRepo repository.EnrollmentRepository,
) *AnnouncementService {
	return &AnnouncementService{
		announcementRepo: announcementRepo,
		receiptRepo:      receiptRepo,
		enrollmentRepo:   enrollmentRepo,
	}
}

// CreateAnnouncement validates and creates an announcement.
func (s *AnnouncementService) CreateAnnouncement(ctx context.Context, announcement *models.Announcement) error {
	if announcement.Title == "" {
		return errors.New("announcement title is required")
	}
	if announcement.Message == "" {
		return errors.New("announcement message is required")
	}

	// Validate priority
	if announcement.Priority == "" {
		announcement.Priority = "normal"
	}
	if announcement.Priority != "normal" && announcement.Priority != "urgent" {
		return errors.New("priority must be 'normal' or 'urgent'")
	}

	// Validate target audience
	if announcement.TargetAudience == "" {
		announcement.TargetAudience = "all"
	}

	// State machine: determine workflow state
	if announcement.WorkflowState == "published" || announcement.WorkflowState == "" {
		if announcement.DelayedPostAt != nil && announcement.DelayedPostAt.After(time.Now()) {
			announcement.WorkflowState = "scheduled"
		} else {
			announcement.WorkflowState = "published"
			now := time.Now()
			announcement.PostedAt = &now
		}
	}

	return s.announcementRepo.Create(ctx, announcement)
}

// UpdateAnnouncement updates an existing announcement.
func (s *AnnouncementService) UpdateAnnouncement(ctx context.Context, announcement *models.Announcement) error {
	if announcement.Title == "" {
		return errors.New("announcement title is required")
	}

	// If transitioning to published and no posted_at, set it now
	if announcement.WorkflowState == "published" && announcement.PostedAt == nil {
		now := time.Now()
		announcement.PostedAt = &now
	}

	// If delayed_post_at is set in the future and state is not draft, schedule it
	if announcement.DelayedPostAt != nil && announcement.DelayedPostAt.After(time.Now()) && announcement.WorkflowState != "draft" {
		announcement.WorkflowState = "scheduled"
	}

	return s.announcementRepo.Update(ctx, announcement)
}

// DeleteAnnouncement soft-deletes an announcement.
func (s *AnnouncementService) DeleteAnnouncement(ctx context.Context, id uint) error {
	return s.announcementRepo.Delete(ctx, id)
}

// ListCourseAnnouncements returns announcements for a course, filtering by audience if applicable.
func (s *AnnouncementService) ListCourseAnnouncements(ctx context.Context, courseID uint, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Announcement], error) {
	result, err := s.announcementRepo.ListByCourseID(ctx, courseID, params)
	if err != nil {
		return nil, err
	}

	// Determine user enrollment type for audience filtering.
	// accountID=0: service doesn't yet thread tenant context.
	enrollment, enrollErr := s.enrollmentRepo.FindByUserAndCourse(ctx, userID, courseID, 0)

	// If user has no enrollment, return all (admin or global context)
	if enrollErr != nil || enrollment == nil {
		return result, nil
	}

	// Filter by target audience
	filtered := make([]models.Announcement, 0, len(result.Items))
	for _, a := range result.Items {
		if matchesAudience(a.TargetAudience, enrollment.Type) {
			filtered = append(filtered, a)
		}
	}

	result.Items = filtered
	result.TotalCount = int64(len(filtered))
	return result, nil
}

// matchesAudience checks if a user's enrollment type matches the announcement target.
func matchesAudience(target, enrollmentType string) bool {
	switch target {
	case "all":
		return true
	case "students":
		return enrollmentType == "StudentEnrollment"
	case "teachers":
		return enrollmentType == "TeacherEnrollment" || enrollmentType == "TaEnrollment"
	case "observers":
		return enrollmentType == "ObserverEnrollment"
	default:
		// section:<id> targeting — show to all for now (section filtering done at repo level)
		return true
	}
}

// ListGlobalAnnouncements returns global/account-level announcements.
func (s *AnnouncementService) ListGlobalAnnouncements(ctx context.Context, params repository.PaginationParams) (*repository.PaginatedResult[models.Announcement], error) {
	return s.announcementRepo.ListGlobal(ctx, params)
}

// ListAccountAnnouncements returns announcements for a specific account.
func (s *AnnouncementService) ListAccountAnnouncements(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Announcement], error) {
	return s.announcementRepo.ListByAccountID(ctx, accountID, params)
}

// GetAnnouncement returns a single announcement by ID.
func (s *AnnouncementService) GetAnnouncement(ctx context.Context, id uint) (*models.Announcement, error) {
	return s.announcementRepo.FindByID(ctx, id)
}

// PublishScheduledAnnouncements transitions scheduled announcements to published
// when their DelayedPostAt time has passed.
func (s *AnnouncementService) PublishScheduledAnnouncements(ctx context.Context) (int, error) {
	ready, err := s.announcementRepo.ListScheduledReady(ctx)
	if err != nil {
		return 0, err
	}

	count := 0
	for i := range ready {
		ready[i].WorkflowState = "published"
		now := time.Now()
		ready[i].PostedAt = &now
		if err := s.announcementRepo.Update(ctx, &ready[i]); err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}

// MarkAsRead upserts a read receipt for the given user and announcement.
func (s *AnnouncementService) MarkAsRead(ctx context.Context, announcementID, userID uint) error {
	return s.receiptRepo.MarkRead(ctx, announcementID, userID)
}

// AcknowledgeAnnouncement marks an announcement as acknowledged by the user.
func (s *AnnouncementService) AcknowledgeAnnouncement(ctx context.Context, announcementID, userID uint) error {
	// Verify the announcement requires acknowledgement
	announcement, err := s.announcementRepo.FindByID(ctx, announcementID)
	if err != nil {
		return err
	}
	if !announcement.RequireAck {
		return errors.New("this announcement does not require acknowledgement")
	}

	return s.receiptRepo.MarkAcknowledged(ctx, announcementID, userID)
}

// GetReadReceipts returns paginated read receipts for an announcement (instructor view).
func (s *AnnouncementService) GetReadReceipts(ctx context.Context, announcementID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AnnouncementReadReceipt], error) {
	return s.receiptRepo.ListByAnnouncementID(ctx, announcementID, params)
}

// GetAnnouncementStats returns read/acknowledged counts and total audience.
func (s *AnnouncementService) GetAnnouncementStats(ctx context.Context, announcementID uint) (*AnnouncementStats, error) {
	announcement, err := s.announcementRepo.FindByID(ctx, announcementID)
	if err != nil {
		return nil, err
	}

	readCount, err := s.receiptRepo.CountReadByAnnouncementID(ctx, announcementID)
	if err != nil {
		return nil, err
	}

	ackCount, err := s.receiptRepo.CountAcknowledgedByAnnouncementID(ctx, announcementID)
	if err != nil {
		return nil, err
	}

	// Estimate total audience from course enrollments
	var totalAudience int64
	if announcement.CourseID != nil {
		enrollParams := repository.PaginationParams{Page: 1, PerPage: 1}
		enrollResult, err := s.enrollmentRepo.ListByCourseID(ctx, *announcement.CourseID, 0, enrollParams)
		if err == nil {
			totalAudience = enrollResult.TotalCount
		}
	}

	return &AnnouncementStats{
		ReadCount:         readCount,
		AcknowledgedCount: ackCount,
		TotalAudience:     totalAudience,
	}, nil
}

// ReadAckStatus holds the read/acknowledged status for a single announcement.
type ReadAckStatus struct {
	IsRead         bool
	IsAcknowledged bool
}

// GetBulkReadStatus returns read/acknowledged status for multiple announcements in a single query.
func (s *AnnouncementService) GetBulkReadStatus(ctx context.Context, announcementIDs []uint, userID uint) map[uint]ReadAckStatus {
	result := make(map[uint]ReadAckStatus, len(announcementIDs))
	if len(announcementIDs) == 0 || userID == 0 {
		return result
	}

	receipts, err := s.receiptRepo.FindByAnnouncementIDsAndUser(ctx, announcementIDs, userID)
	if err != nil {
		return result
	}

	for _, r := range receipts {
		result[r.AnnouncementID] = ReadAckStatus{
			IsRead:         true,
			IsAcknowledged: r.Acknowledged,
		}
	}
	return result
}

// IsRead checks if a user has read a specific announcement.
func (s *AnnouncementService) IsRead(ctx context.Context, announcementID, userID uint) bool {
	receipt, err := s.receiptRepo.FindByAnnouncementAndUser(ctx, announcementID, userID)
	return err == nil && receipt != nil
}

// IsAcknowledged checks if a user has acknowledged a specific announcement.
func (s *AnnouncementService) IsAcknowledged(ctx context.Context, announcementID, userID uint) bool {
	receipt, err := s.receiptRepo.FindByAnnouncementAndUser(ctx, announcementID, userID)
	return err == nil && receipt != nil && receipt.Acknowledged
}
