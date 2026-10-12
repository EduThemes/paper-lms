package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	repoPostgres "github.com/EduThemes/paper-lms/internal/repository/postgres"
)

// Announcement workflow states.
const (
	AnnouncementStateDraft     = "draft"
	AnnouncementStateScheduled = "scheduled"
	AnnouncementStatePublished = "published"
	AnnouncementStateDeleted   = "deleted"
)

// AnnouncementStats holds read/acknowledged counts for an announcement.
type AnnouncementStats struct {
	ReadCount         int64 `json:"read_count"`
	AcknowledgedCount int64 `json:"acknowledged_count"`
	TotalAudience     int64 `json:"total_audience"`
}

// Notifier queues a per-user notification (in-app plus the
// user's delivery channels). NotificationDeliveryService satisfies it.
type Notifier interface {
	QueueNotification(ctx context.Context, userID uint, notificationType, subject, body, contextType string, contextID uint) error
}

// AnnouncementService provides business logic for announcements.
type AnnouncementService struct {
	announcementRepo repoPostgres.AnnouncementRepository
	receiptRepo      repoPostgres.AnnouncementReadReceiptRepository
	enrollmentRepo   repository.EnrollmentRepository
	notifier         Notifier
	// extras stores replies and section targeting (G090). Nil disables
	// both (unit tests).
	extras AnnouncementExtrasStore
}

// AnnouncementExtrasStore is the persistence for announcement replies and
// section targeting (postgres.AnnouncementExtrasRepository).
type AnnouncementExtrasStore interface {
	CreateReply(ctx context.Context, reply *models.AnnouncementReply) error
	FindReply(ctx context.Context, announcementID, replyID uint) (*models.AnnouncementReply, error)
	UpdateReply(ctx context.Context, reply *models.AnnouncementReply) error
	ListReplies(ctx context.Context, announcementID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AnnouncementReply], error)
	ReplyCounts(ctx context.Context, announcementIDs []uint) (map[uint]int64, error)
	SetSections(ctx context.Context, announcementID uint, sectionIDs []uint) error
	SectionsFor(ctx context.Context, announcementIDs []uint) (map[uint][]uint, error)
	SectionsInCourse(ctx context.Context, courseID uint, sectionIDs []uint) ([]uint, error)
}

// SetExtras wires replies and section targeting.
func (s *AnnouncementService) SetExtras(e AnnouncementExtrasStore) {
	s.extras = e
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

// SetNotifier wires the notification fan-out used when an announcement
// becomes visible (published immediately or by the delayed-post job).
// Nil disables notifications (tests, tools).
func (s *AnnouncementService) SetNotifier(n Notifier) {
	s.notifier = n
}

// resolveWorkflowState maps the requested state + delayed_post_at onto
// the stored state. "scheduled" and "published" both mean "post it": a
// future delayed_post_at keeps the row scheduled until the delayed-post
// job publishes it, otherwise it is published now. "draft" stays draft.
func resolveWorkflowState(a *models.Announcement, now time.Time) error {
	switch a.WorkflowState {
	case "", AnnouncementStatePublished, AnnouncementStateScheduled:
		if a.DelayedPostAt != nil && a.DelayedPostAt.After(now) {
			a.WorkflowState = AnnouncementStateScheduled
			a.PostedAt = nil
			return nil
		}
		a.WorkflowState = AnnouncementStatePublished
		if a.PostedAt == nil {
			t := now
			a.PostedAt = &t
		}
		return nil
	case AnnouncementStateDraft:
		return nil
	default:
		return errors.New("workflow_state must be 'draft', 'scheduled', or 'published'")
	}
}

// CreateAnnouncement validates and creates an announcement.
func (s *AnnouncementService) CreateAnnouncement(ctx context.Context, announcement *models.Announcement) error {
	return s.CreateAnnouncementWithSections(ctx, announcement, nil)
}

// CreateAnnouncementWithSections creates an announcement; with
// target_audience "sections" it is visible only to sectionIDs (sections of
// the announcement's course).
func (s *AnnouncementService) CreateAnnouncementWithSections(ctx context.Context, announcement *models.Announcement, sectionIDs []uint) error {
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
	sections, err := s.validateTarget(ctx, announcement, sectionIDs, true)
	if err != nil {
		return err
	}

	if err := resolveWorkflowState(announcement, time.Now()); err != nil {
		return err
	}

	if err := s.announcementRepo.Create(ctx, announcement); err != nil {
		return err
	}
	if sections != nil && s.extras != nil {
		if err := s.extras.SetSections(ctx, announcement.ID, sections); err != nil {
			return err
		}
	}
	if announcement.WorkflowState == AnnouncementStatePublished {
		s.notifyPublishedAsync(announcement)
	}
	return nil
}

// UpdateAnnouncement updates an existing announcement. previousState is
// the stored state before the caller applied its changes; a transition
// into "published" fans out notifications.
func (s *AnnouncementService) UpdateAnnouncement(ctx context.Context, announcement *models.Announcement, previousState string) error {
	return s.UpdateAnnouncementWithSections(ctx, announcement, previousState, nil)
}

// UpdateAnnouncementWithSections updates an announcement. sectionIDs nil
// keeps the stored section list; non-nil replaces it (target_audience
// "sections" requires at least one).
func (s *AnnouncementService) UpdateAnnouncementWithSections(ctx context.Context, announcement *models.Announcement, previousState string, sectionIDs []uint) error {
	if announcement.Title == "" {
		return errors.New("announcement title is required")
	}
	sections, err := s.validateTarget(ctx, announcement, sectionIDs, false)
	if err != nil {
		return err
	}

	if err := resolveWorkflowState(announcement, time.Now()); err != nil {
		return err
	}

	if err := s.announcementRepo.Update(ctx, announcement); err != nil {
		return err
	}
	if sections != nil && s.extras != nil {
		if err := s.extras.SetSections(ctx, announcement.ID, sections); err != nil {
			return err
		}
	}
	if announcement.WorkflowState == AnnouncementStatePublished && previousState != AnnouncementStatePublished {
		s.notifyPublishedAsync(announcement)
	}
	return nil
}

// notifyPublishedAsync runs the audience fan-out off the request path.
// It queues several writes per course member, so a large course would
// otherwise hold the teacher's POST open for seconds (and invite a
// duplicate-posting retry). It works on a copy with a detached context,
// like the discussion OnEntryCreated callbacks; panics are recovered.
func (s *AnnouncementService) notifyPublishedAsync(a *models.Announcement) {
	if s.notifier == nil || a.CourseID == nil || s.enrollmentRepo == nil {
		return
	}
	snapshot := *a
	go func() {
		defer recoverFromPanic("announcement notifyPublished")
		s.notifyPublished(context.Background(), &snapshot)
	}()
}

// DeleteAnnouncement soft-deletes an announcement.
func (s *AnnouncementService) DeleteAnnouncement(ctx context.Context, id uint) error {
	return s.announcementRepo.Delete(ctx, id)
}

func isInstructorEnrollmentType(t string) bool {
	return t == "TeacherEnrollment" || t == "TaEnrollment" || t == "DesignerEnrollment"
}

// courseEnrollments returns all of the user's enrollments (any state) in
// the course.
func (s *AnnouncementService) courseEnrollments(ctx context.Context, courseID, userID uint) []models.Enrollment {
	if s.enrollmentRepo == nil || userID == 0 {
		return nil
	}
	// accountID=0: the course id was already tenant-checked by the
	// enrolled route middleware / handler authz before this runs.
	all, err := s.enrollmentRepo.ListByUserID(ctx, userID, 0)
	if err != nil {
		return nil
	}
	out := make([]models.Enrollment, 0, 1)
	for _, e := range all {
		if e.CourseID == courseID {
			out = append(out, e)
		}
	}
	return out
}

// viewerFor resolves how a user sees a course's announcements.
//
//   - An ACTIVE teacher/TA/designer enrollment → instructor view.
//   - No enrollment at all → instructor view: the enrolled route
//     middleware and the handler authz only let a non-member this far
//     when they are an admin.
//   - Anyone else (students, observers, members with only inactive
//     enrollments) gets the audience-filtered, posted-only view.
func (s *AnnouncementService) viewerFor(ctx context.Context, courseID, userID uint) repoPostgres.AnnouncementViewer {
	enrollments := s.courseEnrollments(ctx, courseID, userID)
	if len(enrollments) == 0 {
		return repoPostgres.AnnouncementViewer{Instructor: true}
	}
	v := repoPostgres.AnnouncementViewer{}
	seen := map[string]bool{}
	for _, e := range enrollments {
		if e.WorkflowState != "active" {
			continue
		}
		if isInstructorEnrollmentType(e.Type) {
			return repoPostgres.AnnouncementViewer{Instructor: true}
		}
		if aud := audienceForEnrollmentType(e.Type); aud != "" && !seen[aud] {
			seen[aud] = true
			v.Audiences = append(v.Audiences, aud)
		}
		if e.CourseSectionID != nil {
			v.SectionIDs = append(v.SectionIDs, *e.CourseSectionID)
		}
	}
	return v
}

// ListCourseAnnouncements returns the announcements of a course visible
// to the user. Instructors (and admins) see drafts, delayed posts and
// every audience; other users see posted rows targeted at them, by role
// (students / observers / teachers) or by one of their sections. The
// filtering happens in SQL so pagination counts are exact.
func (s *AnnouncementService) ListCourseAnnouncements(ctx context.Context, courseID uint, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Announcement], error) {
	return s.announcementRepo.ListForViewer(ctx, courseID, s.viewerFor(ctx, courseID, userID), params)
}

// CanView reports whether the user may read this announcement through a
// direct GET / mark-read / acknowledge. Mirrors ListCourseAnnouncements:
// drafts and not-yet-posted delayed announcements are instructor-only,
// and audience / section targeting applies.
func (s *AnnouncementService) CanView(ctx context.Context, a *models.Announcement, userID uint) bool {
	if a.WorkflowState == AnnouncementStateDeleted {
		return false
	}
	if a.CourseID == nil {
		return isPosted(a)
	}
	v := s.viewerFor(ctx, *a.CourseID, userID)
	if v.Instructor {
		return true
	}
	return isPosted(a) && viewerMatchesAudience(v, a.TargetAudience, s.targetSections(ctx, a))
}

// targetSections is the section list of a "sections"-targeted
// announcement (nil for any other audience).
func (s *AnnouncementService) targetSections(ctx context.Context, a *models.Announcement) []uint {
	if a.TargetAudience != AnnouncementTargetSections || s.extras == nil {
		return nil
	}
	m, err := s.extras.SectionsFor(ctx, []uint{a.ID})
	if err != nil {
		return nil
	}
	return m[a.ID]
}

// AnnouncementTargetSections is the target_audience of an announcement
// restricted to the sections in announcement_section_visibilities.
const AnnouncementTargetSections = "sections"

// validateTarget checks target_audience and, for "sections", that every
// section belongs to the announcement's course. It returns the section
// list to store (nil = leave the stored list alone).
func (s *AnnouncementService) validateTarget(ctx context.Context, a *models.Announcement, sectionIDs []uint, creating bool) ([]uint, error) {
	switch {
	case a.TargetAudience == "all", a.TargetAudience == "students", a.TargetAudience == "teachers", a.TargetAudience == "observers":
		if sectionIDs != nil || !creating {
			// Leaving section targeting clears the stored list.
			if s.extras != nil && a.ID != 0 {
				return []uint{}, nil
			}
		}
		return nil, nil
	case strings.HasPrefix(a.TargetAudience, "section:"):
		if a.CourseID == nil {
			return nil, errors.New("section targeting is only available for course announcements")
		}
		id, err := strconv.ParseUint(strings.TrimPrefix(a.TargetAudience, "section:"), 10, 64)
		if err != nil || id == 0 {
			return nil, errors.New("invalid section target")
		}
		if s.extras != nil {
			ok, err := s.extras.SectionsInCourse(ctx, *a.CourseID, []uint{uint(id)})
			if err != nil {
				return nil, err
			}
			if len(ok) != 1 {
				return nil, errors.New("section not found in this course")
			}
		}
		return nil, nil
	case a.TargetAudience == AnnouncementTargetSections:
		if a.CourseID == nil {
			return nil, errors.New("section targeting is only available for course announcements")
		}
		if s.extras == nil {
			return nil, errors.New("section targeting is not available")
		}
		if sectionIDs == nil {
			if creating {
				return nil, errors.New("section_ids is required when targeting sections")
			}
			existing := s.targetSections(ctx, a)
			if len(existing) == 0 {
				return nil, errors.New("section_ids is required when targeting sections")
			}
			return nil, nil
		}
		uniq := map[uint]bool{}
		var ids []uint
		for _, id := range sectionIDs {
			if id != 0 && !uniq[id] {
				uniq[id] = true
				ids = append(ids, id)
			}
		}
		if len(ids) == 0 {
			return nil, errors.New("section_ids is required when targeting sections")
		}
		ok, err := s.extras.SectionsInCourse(ctx, *a.CourseID, ids)
		if err != nil {
			return nil, err
		}
		if len(ok) != len(ids) {
			return nil, errors.New("section not found in this course")
		}
		return ids, nil
	default:
		return nil, errors.New("target_audience must be all, students, teachers, observers or sections")
	}
}

func isPosted(a *models.Announcement) bool {
	if a.WorkflowState != AnnouncementStatePublished {
		return false
	}
	return a.PostedAt == nil || !a.PostedAt.After(time.Now())
}

func audienceForEnrollmentType(t string) string {
	switch t {
	case "StudentEnrollment":
		return "students"
	case "ObserverEnrollment":
		return "observers"
	case "TeacherEnrollment", "TaEnrollment", "DesignerEnrollment":
		return "teachers"
	}
	return ""
}

func viewerMatchesAudience(v repoPostgres.AnnouncementViewer, target string, targetSections []uint) bool {
	if v.Instructor || target == "" || target == "all" {
		return true
	}
	if target == AnnouncementTargetSections {
		for _, mine := range v.SectionIDs {
			for _, t := range targetSections {
				if mine == t {
					return true
				}
			}
		}
		return false
	}
	for _, a := range v.Audiences {
		if a == target {
			return true
		}
	}
	for _, id := range v.SectionIDs {
		if target == fmt.Sprintf("section:%d", id) {
			return true
		}
	}
	return false
}

// matchesAudience checks whether a member with the given enrollment type
// (and section) is targeted by the announcement. Instructors are always
// in the audience of their own course's announcements.
func matchesAudience(target, enrollmentType string, sectionID *uint, targetSections []uint) bool {
	v := repoPostgres.AnnouncementViewer{Instructor: isInstructorEnrollmentType(enrollmentType)}
	if aud := audienceForEnrollmentType(enrollmentType); aud != "" {
		v.Audiences = []string{aud}
	}
	if sectionID != nil {
		v.SectionIDs = []uint{*sectionID}
	}
	if target == "teachers" {
		// "Teachers only" is a real restriction even though instructors
		// see everything when listing.
		return v.Instructor
	}
	if target == AnnouncementTargetSections && v.Instructor {
		// Instructors always see their course's announcements, but a
		// section-targeted post only notifies the sections' members.
		v.Instructor = false
	}
	return viewerMatchesAudience(v, target, targetSections)
}

// notifyPublished fans a "new_announcement" notification out to the
// course members the announcement targets (never the author). Failures
// are swallowed: notification delivery must not fail the write.
func (s *AnnouncementService) notifyPublished(ctx context.Context, a *models.Announcement) {
	if s.notifier == nil || a.CourseID == nil || s.enrollmentRepo == nil {
		return
	}
	const perPage = 200
	subject := "New announcement: " + a.Title
	seen := map[uint]bool{a.UserID: true}
	targeted := s.targetSections(ctx, a)
	for page := 1; ; page++ {
		// accountID=0: background fan-out; the course id comes from a
		// stored announcement row, not from caller input.
		res, err := s.enrollmentRepo.ListByCourseID(ctx, *a.CourseID, 0, repository.PaginationParams{Page: page, PerPage: perPage})
		if err != nil || res == nil {
			return
		}
		for _, e := range res.Items {
			if seen[e.UserID] || e.WorkflowState != "active" {
				continue
			}
			if !matchesAudience(a.TargetAudience, e.Type, e.CourseSectionID, targeted) {
				continue
			}
			seen[e.UserID] = true
			_ = s.notifier.QueueNotification(ctx, e.UserID, "new_announcement", subject, a.Message, "Course", *a.CourseID)
		}
		if len(res.Items) < perPage || int64(page*perPage) >= res.TotalCount {
			return
		}
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

// PublishScheduledAnnouncements transitions scheduled announcements to
// published once their DelayedPostAt has passed, and notifies their
// audience. Run every minute by the background scheduler.
func (s *AnnouncementService) PublishScheduledAnnouncements(ctx context.Context) (int, error) {
	ready, err := s.announcementRepo.ListScheduledReady(ctx)
	if err != nil {
		return 0, err
	}

	count := 0
	for i := range ready {
		ready[i].WorkflowState = AnnouncementStatePublished
		now := time.Now()
		ready[i].PostedAt = &now
		if err := s.announcementRepo.Update(ctx, &ready[i]); err != nil {
			return count, err
		}
		s.notifyPublished(ctx, &ready[i])
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
