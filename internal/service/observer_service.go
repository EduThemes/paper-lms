package service

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// ObserverAnnouncementRepository is a consumer-side interface so this file
// can use the announcement repo (defined in postgres pkg) without import cycles.
type ObserverAnnouncementRepository interface {
	ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Announcement], error)
}

// ChildOverview aggregates the per-child data shown on the parent dashboard.
type ChildOverview struct {
	Courses          []ChildCourseSummary `json:"courses"`
	UpcomingThisWeek []UpcomingItem       `json:"upcoming_this_week"`
	RecentGrades     []RecentGradeItem    `json:"recent_grades"`
	RecentActivity   []ActivityItem       `json:"recent_activity"`
}

// ChildCourseSummary represents a course the child is enrolled in plus
// summary stats (current grade %, count of assignments still pending).
type ChildCourseSummary struct {
	CourseID     uint     `json:"course_id"`
	Name         string   `json:"name"`
	CourseCode   string   `json:"course_code"`
	CurrentGrade *float64 `json:"current_grade,omitempty"`
	PendingCount int      `json:"pending_count"`
}

// UpcomingItem is an assignment or quiz due within the next 7 days.
type UpcomingItem struct {
	ID         uint      `json:"id"`
	Type       string    `json:"type"` // "assignment" or "quiz"
	Title      string    `json:"title"`
	CourseID   uint      `json:"course_id"`
	CourseName string    `json:"course_name"`
	DueAt      time.Time `json:"due_at"`
}

// RecentGradeItem is one of the last graded submissions for the child.
type RecentGradeItem struct {
	SubmissionID   uint      `json:"submission_id"`
	AssignmentID   uint      `json:"assignment_id"`
	AssignmentName string    `json:"assignment_name"`
	CourseID       uint      `json:"course_id"`
	CourseName     string    `json:"course_name"`
	Score          *float64  `json:"score,omitempty"`
	PointsPossible *float64  `json:"points_possible,omitempty"`
	GradedAt       time.Time `json:"graded_at"`
}

// ActivityItem is an announcement or page-update from a course the child
// is enrolled in.
type ActivityItem struct {
	Type       string    `json:"type"` // "announcement" or "page"
	ID         uint      `json:"id"`
	Title      string    `json:"title"`
	CourseID   uint      `json:"course_id"`
	CourseName string    `json:"course_name"`
	OccurredAt time.Time `json:"occurred_at"`
}

// ObserverEnrollmentRepository defines the enrollment repo methods needed by the
// observer service. The main EnrollmentRepository interface will be extended with
// an Update method by the main thread; this consumer-side interface ensures
// compile-time safety without modifying shared interface files.
type ObserverEnrollmentRepository interface {
	Create(ctx context.Context, enrollment *models.Enrollment) error
	FindByID(ctx context.Context, id uint) (*models.Enrollment, error)
	ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Enrollment], error)
	ListByUserID(ctx context.Context, userID uint) ([]models.Enrollment, error)
	FindByUserAndCourse(ctx context.Context, userID, courseID uint) (*models.Enrollment, error)
	Update(ctx context.Context, enrollment *models.Enrollment) error
}

type ObserverService struct {
	enrollmentRepo ObserverEnrollmentRepository
	courseRepo     repository.CourseRepository
	userRepo       repository.UserRepository

	// Optional dependencies — only required for GetChildOverview. They are
	// configured via SetOverviewDeps so the existing constructor signature
	// (called from main.go) does not have to change. If nil, GetChildOverview
	// returns an error.
	assignmentRepo   repository.AssignmentRepository
	submissionRepo   repository.SubmissionRepository
	quizRepo         repository.QuizRepository
	announcementRepo ObserverAnnouncementRepository
	pageRepo         repository.PageRepository
}

func NewObserverService(
	enrollmentRepo ObserverEnrollmentRepository,
	courseRepo repository.CourseRepository,
	userRepo repository.UserRepository,
) *ObserverService {
	return &ObserverService{
		enrollmentRepo: enrollmentRepo,
		courseRepo:     courseRepo,
		userRepo:       userRepo,
	}
}

// SetOverviewDeps wires the additional repositories needed to power
// GetChildOverview. Call this from cmd/server/main.go after constructing
// the service. Safe to call multiple times.
func (s *ObserverService) SetOverviewDeps(
	assignmentRepo repository.AssignmentRepository,
	submissionRepo repository.SubmissionRepository,
	quizRepo repository.QuizRepository,
	announcementRepo ObserverAnnouncementRepository,
	pageRepo repository.PageRepository,
) {
	s.assignmentRepo = assignmentRepo
	s.submissionRepo = submissionRepo
	s.quizRepo = quizRepo
	s.announcementRepo = announcementRepo
	s.pageRepo = pageRepo
}

// LinkObserverToStudent creates an ObserverEnrollment for each active course the
// student is enrolled in. The AssociatedUserID on each enrollment is set to the
// student's user ID so the observer is linked to that student.
func (s *ObserverService) LinkObserverToStudent(ctx context.Context, observerUserID, studentUserID uint) error {
	// Validate that the observer user exists.
	_, err := s.userRepo.FindByID(ctx, observerUserID)
	if err != nil {
		return errors.New("observer user not found")
	}

	// Validate that the student user exists.
	_, err = s.userRepo.FindByID(ctx, studentUserID)
	if err != nil {
		return errors.New("student user not found")
	}

	// Check if the observer is already linked to this student.
	already, err := s.IsObserverOf(ctx, observerUserID, studentUserID)
	if err != nil {
		return err
	}
	if already {
		return errors.New("observer is already linked to this student")
	}

	// Get all active enrollments for the student.
	studentEnrollments, err := s.enrollmentRepo.ListByUserID(ctx, studentUserID)
	if err != nil {
		return errors.New("could not fetch student enrollments")
	}

	if len(studentEnrollments) == 0 {
		return errors.New("student has no active enrollments")
	}

	// Create an ObserverEnrollment in each course the student is enrolled in.
	for _, se := range studentEnrollments {
		if se.WorkflowState != models.EnrollmentActive {
			continue
		}

		enrollment := &models.Enrollment{
			UserID:           observerUserID,
			CourseID:         se.CourseID,
			CourseSectionID:  se.CourseSectionID,
			Type:             "ObserverEnrollment",
			Role:             "ObserverEnrollment",
			WorkflowState:    models.EnrollmentActive,
			AssociatedUserID: &studentUserID,
		}

		if err := s.enrollmentRepo.Create(ctx, enrollment); err != nil {
			return err
		}
	}

	return nil
}

// UnlinkObserver removes observer enrollments for the given student by setting
// the workflow_state to "deleted" on each matching ObserverEnrollment.
func (s *ObserverService) UnlinkObserver(ctx context.Context, observerUserID, studentUserID uint) error {
	enrollments, err := s.enrollmentRepo.ListByUserID(ctx, observerUserID)
	if err != nil {
		return errors.New("could not fetch observer enrollments")
	}

	found := false
	for _, e := range enrollments {
		if e.Type != "ObserverEnrollment" {
			continue
		}
		if e.AssociatedUserID == nil || *e.AssociatedUserID != studentUserID {
			continue
		}

		found = true
		e.WorkflowState = "deleted"
		if err := s.enrollmentRepo.Update(ctx, &e); err != nil {
			return err
		}
	}

	if !found {
		return errors.New("observer is not linked to this student")
	}

	return nil
}

// ListObservedStudents returns a list of unique student user IDs the observer is
// linked to, derived from active ObserverEnrollment records where
// associated_user_id is not null.
func (s *ObserverService) ListObservedStudents(ctx context.Context, observerUserID uint) ([]uint, error) {
	enrollments, err := s.enrollmentRepo.ListByUserID(ctx, observerUserID)
	if err != nil {
		return nil, errors.New("could not fetch observer enrollments")
	}

	seen := make(map[uint]bool)
	var studentIDs []uint

	for _, e := range enrollments {
		if e.Type != "ObserverEnrollment" {
			continue
		}
		if e.AssociatedUserID == nil {
			continue
		}
		if e.WorkflowState != "active" {
			continue
		}
		if !seen[*e.AssociatedUserID] {
			seen[*e.AssociatedUserID] = true
			studentIDs = append(studentIDs, *e.AssociatedUserID)
		}
	}

	return studentIDs, nil
}

// IsObserverOf checks whether the observer has an active ObserverEnrollment
// linked to the given student.
func (s *ObserverService) IsObserverOf(ctx context.Context, observerUserID, studentUserID uint) (bool, error) {
	enrollments, err := s.enrollmentRepo.ListByUserID(ctx, observerUserID)
	if err != nil {
		return false, errors.New("could not fetch observer enrollments")
	}

	for _, e := range enrollments {
		if e.Type != "ObserverEnrollment" {
			continue
		}
		if e.AssociatedUserID == nil {
			continue
		}
		if e.WorkflowState != "active" {
			continue
		}
		if *e.AssociatedUserID == studentUserID {
			return true, nil
		}
	}

	return false, nil
}

// GetObserveeCourses returns the courses a specific observed student is enrolled
// in where the observer also has an active ObserverEnrollment linked to that student.
func (s *ObserverService) GetObserveeCourses(ctx context.Context, observerUserID, studentUserID uint) ([]models.Course, error) {
	// Verify observer is actually linked to this student
	linked, err := s.IsObserverOf(ctx, observerUserID, studentUserID)
	if err != nil {
		return nil, err
	}
	if !linked {
		return nil, errors.New("observer is not linked to this student")
	}

	// Get the student's active enrollments to find their courses
	studentEnrollments, err := s.enrollmentRepo.ListByUserID(ctx, studentUserID)
	if err != nil {
		return nil, errors.New("could not fetch student enrollments")
	}

	seen := make(map[uint]bool)
	var courses []models.Course

	for _, e := range studentEnrollments {
		if e.WorkflowState != "active" {
			continue
		}
		if seen[e.CourseID] {
			continue
		}
		seen[e.CourseID] = true

		course, err := s.courseRepo.FindByID(ctx, e.CourseID, 0)
		if err != nil {
			continue
		}
		courses = append(courses, *course)
	}

	return courses, nil
}

// GetChildOverview returns aggregated dashboard data for a single child
// (current courses + grades, upcoming work this week, recent grades, and
// recent activity). The caller must be linked to childID via an active
// ObserverEnrollment, otherwise an error is returned.
func (s *ObserverService) GetChildOverview(ctx context.Context, parentID, childID uint) (*ChildOverview, error) {
	if s.assignmentRepo == nil || s.submissionRepo == nil || s.quizRepo == nil || s.announcementRepo == nil || s.pageRepo == nil {
		return nil, errors.New("observer overview dependencies not configured")
	}

	linked, err := s.IsObserverOf(ctx, parentID, childID)
	if err != nil {
		return nil, err
	}
	if !linked {
		return nil, errors.New("observer is not linked to this student")
	}

	studentEnrollments, err := s.enrollmentRepo.ListByUserID(ctx, childID)
	if err != nil {
		return nil, errors.New("could not fetch student enrollments")
	}

	// Build the unique active course set for the child.
	courseIDs := []uint{}
	courseByID := map[uint]*models.Course{}
	for _, e := range studentEnrollments {
		if e.WorkflowState != "active" {
			continue
		}
		if _, ok := courseByID[e.CourseID]; ok {
			continue
		}
		course, cerr := s.courseRepo.FindByID(ctx, e.CourseID, 0)
		if cerr != nil {
			continue
		}
		courseByID[e.CourseID] = course
		courseIDs = append(courseIDs, e.CourseID)
	}

	now := time.Now()
	weekFromNow := now.Add(7 * 24 * time.Hour)

	courseSummaries := make([]ChildCourseSummary, 0, len(courseIDs))
	upcoming := []UpcomingItem{}
	recentGrades := []RecentGradeItem{}
	activity := []ActivityItem{}

	listParams := repository.PaginationParams{Page: 1, PerPage: 100}

	for _, courseID := range courseIDs {
		course := courseByID[courseID]

		// Per-course assignments (used for upcoming + grade calculation).
		assignments, _ := s.assignmentRepo.ListByCourseID(ctx, courseID, listParams)
		assignmentByID := map[uint]models.Assignment{}
		if assignments != nil {
			for _, a := range assignments.Items {
				assignmentByID[a.ID] = a
				if a.WorkflowState == "deleted" || !a.Published {
					continue
				}
				if a.DueAt != nil && a.DueAt.After(now) && a.DueAt.Before(weekFromNow) {
					upcoming = append(upcoming, UpcomingItem{
						ID:         a.ID,
						Type:       "assignment",
						Title:      a.Name,
						CourseID:   courseID,
						CourseName: course.Name,
						DueAt:      *a.DueAt,
					})
				}
			}
		}

		// Quizzes — also count toward "due this week".
		quizzes, _ := s.quizRepo.ListByCourseID(ctx, courseID, listParams)
		if quizzes != nil {
			for _, q := range quizzes.Items {
				if q.WorkflowState == "deleted" || !q.Published {
					continue
				}
				if q.DueAt != nil && q.DueAt.After(now) && q.DueAt.Before(weekFromNow) {
					upcoming = append(upcoming, UpcomingItem{
						ID:         q.ID,
						Type:       "quiz",
						Title:      q.Title,
						CourseID:   courseID,
						CourseName: course.Name,
						DueAt:      *q.DueAt,
					})
				}
			}
		}

		// Submissions for this child in this course (drives grade % + recent + pending).
		subs, serr := s.submissionRepo.ListByUserAndCourse(ctx, childID, courseID)
		var earned, possible float64
		pending := 0
		if serr == nil {
			for _, sub := range subs {
				assn, ok := assignmentByID[sub.AssignmentID]
				if !ok {
					continue
				}
				// Pending = published assignment with a future or past due date that
				// the student has not submitted yet (and is not graded/excused).
				if assn.Published && !sub.Excused && sub.WorkflowState != "graded" {
					if sub.SubmittedAt == nil {
						pending++
					}
				}
				// Grade contribution: only graded, posted submissions with score.
				if sub.Score != nil && assn.PointsPossible != nil && *assn.PointsPossible > 0 && sub.PostedAt != nil {
					earned += *sub.Score
					possible += *assn.PointsPossible
				}
				// Recent grades feed.
				if sub.GradedAt != nil && sub.Score != nil {
					recentGrades = append(recentGrades, RecentGradeItem{
						SubmissionID:   sub.ID,
						AssignmentID:   sub.AssignmentID,
						AssignmentName: assn.Name,
						CourseID:       courseID,
						CourseName:     course.Name,
						Score:          sub.Score,
						PointsPossible: assn.PointsPossible,
						GradedAt:       *sub.GradedAt,
					})
				}
			}
		}

		var grade *float64
		if possible > 0 {
			pct := (earned / possible) * 100.0
			grade = &pct
		}
		courseSummaries = append(courseSummaries, ChildCourseSummary{
			CourseID:     courseID,
			Name:         course.Name,
			CourseCode:   course.CourseCode,
			CurrentGrade: grade,
			PendingCount: pending,
		})

		// Recent activity: announcements + page updates.
		anns, _ := s.announcementRepo.ListByCourseID(ctx, courseID, listParams)
		if anns != nil {
			for _, a := range anns.Items {
				occurred := a.CreatedAt
				if a.PostedAt != nil {
					occurred = *a.PostedAt
				}
				activity = append(activity, ActivityItem{
					Type:       "announcement",
					ID:         a.ID,
					Title:      a.Title,
					CourseID:   courseID,
					CourseName: course.Name,
					OccurredAt: occurred,
				})
			}
		}

		pages, _ := s.pageRepo.ListByCourseID(ctx, courseID, listParams)
		if pages != nil {
			for _, p := range pages.Items {
				activity = append(activity, ActivityItem{
					Type:       "page",
					ID:         p.ID,
					Title:      p.Title,
					CourseID:   courseID,
					CourseName: course.Name,
					OccurredAt: p.UpdatedAt,
				})
			}
		}
	}

	// Sort + truncate the feeds.
	sort.Slice(upcoming, func(i, j int) bool { return upcoming[i].DueAt.Before(upcoming[j].DueAt) })
	sort.Slice(recentGrades, func(i, j int) bool { return recentGrades[i].GradedAt.After(recentGrades[j].GradedAt) })
	if len(recentGrades) > 10 {
		recentGrades = recentGrades[:10]
	}
	sort.Slice(activity, func(i, j int) bool { return activity[i].OccurredAt.After(activity[j].OccurredAt) })
	if len(activity) > 10 {
		activity = activity[:10]
	}

	return &ChildOverview{
		Courses:          courseSummaries,
		UpcomingThisWeek: upcoming,
		RecentGrades:     recentGrades,
		RecentActivity:   activity,
	}, nil
}

// GetObserverDashboard returns the courses where the observer has an active
// ObserverEnrollment.
func (s *ObserverService) GetObserverDashboard(ctx context.Context, observerUserID uint) ([]models.Course, error) {
	enrollments, err := s.enrollmentRepo.ListByUserID(ctx, observerUserID)
	if err != nil {
		return nil, errors.New("could not fetch observer enrollments")
	}

	seen := make(map[uint]bool)
	var courses []models.Course

	for _, e := range enrollments {
		if e.Type != "ObserverEnrollment" {
			continue
		}
		if e.WorkflowState != "active" {
			continue
		}
		if seen[e.CourseID] {
			continue
		}
		seen[e.CourseID] = true

		course, err := s.courseRepo.FindByID(ctx, e.CourseID, 0)
		if err != nil {
			continue
		}
		courses = append(courses, *course)
	}

	return courses, nil
}
