package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// Result types for batch operations

type DateShiftResult struct {
	AssignmentsShifted int `json:"assignments_shifted"`
	EventsShifted      int `json:"events_shifted"`
	DayShift           int `json:"day_shift"`
}

type BulkMessageResult struct {
	MessagesSent int      `json:"messages_sent"`
	Errors       []string `json:"errors,omitempty"`
}

type BulkEnrollmentRequest struct {
	UserID    uint   `json:"user_id"`
	Type      string `json:"type"`
	SectionID *uint  `json:"section_id,omitempty"`
}

type BulkEnrollResult struct {
	Enrolled int      `json:"enrolled"`
	Errors   []string `json:"errors,omitempty"`
}

type AssignmentDateUpdate struct {
	AssignmentID uint       `json:"assignment_id"`
	DueAt        *time.Time `json:"due_at"`
	UnlockAt     *time.Time `json:"unlock_at"`
	LockAt       *time.Time `json:"lock_at"`
}

type BulkDateUpdateResult struct {
	Updated int      `json:"updated"`
	Errors  []string `json:"errors,omitempty"`
}

// BatchService handles bulk operations for course management.
type BatchService struct {
	courseRepo          repository.CourseRepository
	moduleRepo          repository.ModuleRepository
	moduleItemRepo      repository.ModuleItemRepository
	assignmentRepo      repository.AssignmentRepository
	quizRepo            repository.QuizRepository
	quizQuestionRepo    repository.QuizQuestionRepository
	pageRepo            repository.PageRepository
	discussionTopicRepo repository.DiscussionTopicRepository
	calendarEventRepo   repository.CalendarEventRepository
	enrollmentRepo      repository.EnrollmentRepository
	conversationRepo    repository.ConversationRepository
	convParticipantRepo repository.ConversationParticipantRepository
	convMessageRepo     repository.ConversationMessageRepository
	userRepo            repository.UserRepository
	sectionRepo         repository.SectionRepository
}

func NewBatchService(
	courseRepo repository.CourseRepository,
	moduleRepo repository.ModuleRepository,
	moduleItemRepo repository.ModuleItemRepository,
	assignmentRepo repository.AssignmentRepository,
	quizRepo repository.QuizRepository,
	quizQuestionRepo repository.QuizQuestionRepository,
	pageRepo repository.PageRepository,
	discussionTopicRepo repository.DiscussionTopicRepository,
	calendarEventRepo repository.CalendarEventRepository,
	enrollmentRepo repository.EnrollmentRepository,
	conversationRepo repository.ConversationRepository,
	convParticipantRepo repository.ConversationParticipantRepository,
	convMessageRepo repository.ConversationMessageRepository,
	userRepo repository.UserRepository,
	sectionRepo repository.SectionRepository,
) *BatchService {
	return &BatchService{
		courseRepo:          courseRepo,
		moduleRepo:          moduleRepo,
		moduleItemRepo:      moduleItemRepo,
		assignmentRepo:      assignmentRepo,
		quizRepo:            quizRepo,
		quizQuestionRepo:    quizQuestionRepo,
		pageRepo:            pageRepo,
		discussionTopicRepo: discussionTopicRepo,
		calendarEventRepo:   calendarEventRepo,
		enrollmentRepo:      enrollmentRepo,
		conversationRepo:    conversationRepo,
		convParticipantRepo: convParticipantRepo,
		convMessageRepo:     convMessageRepo,
		userRepo:            userRepo,
		sectionRepo:         sectionRepo,
	}
}

// CloneCourse deep-clones a course with selected content types. All cloned content
// is set to "unpublished" workflow state. Returns the newly created course.
func (s *BatchService) CloneCourse(
	ctx context.Context,
	sourceCourseID uint,
	destName string,
	accountID uint,
	includeModules bool,
	includeAssignments bool,
	includePages bool,
	includeQuizzes bool,
	includeDiscussions bool,
) (*models.Course, error) {
	// Fetch the source course
	sourceCourse, err := s.courseRepo.FindByID(ctx, sourceCourseID, 0)
	if err != nil {
		return nil, fmt.Errorf("source course not found: %w", err)
	}

	// Create the new course
	newCourse := &models.Course{
		AccountID:     accountID,
		Name:          destName,
		CourseCode:    sourceCourse.CourseCode + "-copy",
		WorkflowState: "unpublished",
		DefaultView:   sourceCourse.DefaultView,
		SyllabusBody:  sourceCourse.SyllabusBody,
		License:       sourceCourse.License,
		IsPublic:      false,
	}

	if err := s.courseRepo.Create(ctx, newCourse); err != nil {
		return nil, fmt.Errorf("failed to create destination course: %w", err)
	}

	// Create a default section for the new course
	section := &models.CourseSection{
		CourseID:      newCourse.ID,
		Name:          destName,
		WorkflowState: "active",
	}
	if err := s.sectionRepo.Create(ctx, section); err != nil {
		return nil, fmt.Errorf("failed to create default section: %w", err)
	}

	// Track old assignment ID -> new assignment ID for module item remapping
	assignmentIDMap := make(map[uint]uint)
	pageIDMap := make(map[uint]uint)
	quizIDMap := make(map[uint]uint)
	discussionIDMap := make(map[uint]uint)

	// Clone assignments
	if includeAssignments {
		if err := s.cloneAssignments(ctx, sourceCourseID, newCourse.ID, assignmentIDMap); err != nil {
			return nil, fmt.Errorf("failed to clone assignments: %w", err)
		}
	}

	// Clone pages
	if includePages {
		if err := s.clonePages(ctx, sourceCourseID, newCourse.ID, pageIDMap); err != nil {
			return nil, fmt.Errorf("failed to clone pages: %w", err)
		}
	}

	// Clone quizzes
	if includeQuizzes {
		if err := s.cloneQuizzes(ctx, sourceCourseID, newCourse.ID, quizIDMap); err != nil {
			return nil, fmt.Errorf("failed to clone quizzes: %w", err)
		}
	}

	// Clone discussions
	if includeDiscussions {
		if err := s.cloneDiscussions(ctx, sourceCourseID, newCourse.ID, discussionIDMap); err != nil {
			return nil, fmt.Errorf("failed to clone discussions: %w", err)
		}
	}

	// Clone modules (and their items, remapping content IDs)
	if includeModules {
		if err := s.cloneModules(ctx, sourceCourseID, newCourse.ID, assignmentIDMap, pageIDMap, quizIDMap, discussionIDMap); err != nil {
			return nil, fmt.Errorf("failed to clone modules: %w", err)
		}
	}

	return newCourse, nil
}

func (s *BatchService) cloneAssignments(ctx context.Context, sourceCourseID, destCourseID uint, idMap map[uint]uint) error {
	allParams := repository.PaginationParams{Page: 1, PerPage: 1000}
	result, err := s.assignmentRepo.ListByCourseID(ctx, sourceCourseID, allParams)
	if err != nil {
		return err
	}

	for _, a := range result.Items {
		oldID := a.ID
		newAssignment := &models.Assignment{
			CourseID:        destCourseID,
			Name:            a.Name,
			Description:     a.Description,
			DueAt:           a.DueAt,
			UnlockAt:        a.UnlockAt,
			LockAt:          a.LockAt,
			PointsPossible:  a.PointsPossible,
			GradingType:     a.GradingType,
			SubmissionTypes: a.SubmissionTypes,
			Position:        a.Position,
			WorkflowState:   "unpublished",
			Published:       false,
		}
		if err := s.assignmentRepo.Create(ctx, newAssignment); err != nil {
			return fmt.Errorf("failed to clone assignment %d: %w", oldID, err)
		}
		idMap[oldID] = newAssignment.ID
	}
	return nil
}

func (s *BatchService) clonePages(ctx context.Context, sourceCourseID, destCourseID uint, idMap map[uint]uint) error {
	allParams := repository.PaginationParams{Page: 1, PerPage: 1000}
	result, err := s.pageRepo.ListByCourseID(ctx, sourceCourseID, allParams)
	if err != nil {
		return err
	}

	for _, p := range result.Items {
		oldID := p.ID
		newPage := &models.WikiPage{
			CourseID:      destCourseID,
			Title:         p.Title,
			URL:           p.URL,
			Body:          p.Body,
			WorkflowState: "unpublished",
			EditingRoles:  p.EditingRoles,
			FrontPage:     p.FrontPage,
		}
		if err := s.pageRepo.Create(ctx, newPage); err != nil {
			return fmt.Errorf("failed to clone page %d: %w", oldID, err)
		}
		idMap[oldID] = newPage.ID
	}
	return nil
}

func (s *BatchService) cloneQuizzes(ctx context.Context, sourceCourseID, destCourseID uint, idMap map[uint]uint) error {
	allParams := repository.PaginationParams{Page: 1, PerPage: 1000}
	result, err := s.quizRepo.ListByCourseID(ctx, sourceCourseID, allParams)
	if err != nil {
		return err
	}

	for _, q := range result.Items {
		oldID := q.ID
		newQuiz := &models.Quiz{
			CourseID:        destCourseID,
			Title:           q.Title,
			Description:     q.Description,
			QuizType:        q.QuizType,
			TimeLimit:       q.TimeLimit,
			AllowedAttempts: q.AllowedAttempts,
			DueAt:           q.DueAt,
			UnlockAt:        q.UnlockAt,
			LockAt:          q.LockAt,
			PointsPossible:  q.PointsPossible,
			Published:       false,
			WorkflowState:   "unpublished",
		}
		if err := s.quizRepo.Create(ctx, newQuiz); err != nil {
			return fmt.Errorf("failed to clone quiz %d: %w", oldID, err)
		}
		idMap[oldID] = newQuiz.ID

		// Clone quiz questions
		qParams := repository.PaginationParams{Page: 1, PerPage: 1000}
		questions, err := s.quizQuestionRepo.ListByQuizID(ctx, oldID, qParams)
		if err != nil {
			continue // skip question cloning if fetch fails
		}
		for _, question := range questions.Items {
			newQuestion := &models.QuizQuestion{
				QuizID:            newQuiz.ID,
				Position:          question.Position,
				QuestionType:      question.QuestionType,
				QuestionText:      question.QuestionText,
				PointsPossible:    question.PointsPossible,
				Answers:           question.Answers,
				CorrectComments:   question.CorrectComments,
				IncorrectComments: question.IncorrectComments,
				NeutralComments:   question.NeutralComments,
				WorkflowState:     question.WorkflowState,
			}
			_ = s.quizQuestionRepo.Create(ctx, newQuestion)
		}
	}
	return nil
}

func (s *BatchService) cloneDiscussions(ctx context.Context, sourceCourseID, destCourseID uint, idMap map[uint]uint) error {
	allParams := repository.PaginationParams{Page: 1, PerPage: 1000}
	result, err := s.discussionTopicRepo.ListByCourseID(ctx, sourceCourseID, allParams)
	if err != nil {
		return err
	}

	for _, d := range result.Items {
		oldID := d.ID
		newDiscussion := &models.DiscussionTopic{
			CourseID:           destCourseID,
			UserID:             d.UserID,
			Title:              d.Title,
			Message:            d.Message,
			DiscussionType:     d.DiscussionType,
			Pinned:             d.Pinned,
			AllowRating:        d.AllowRating,
			OnlyGradersCanRate: d.OnlyGradersCanRate,
			SortByRating:       d.SortByRating,
			RequireInitialPost: d.RequireInitialPost,
			WorkflowState:      "unpublished",
		}
		if err := s.discussionTopicRepo.Create(ctx, newDiscussion); err != nil {
			return fmt.Errorf("failed to clone discussion %d: %w", oldID, err)
		}
		idMap[oldID] = newDiscussion.ID
	}
	return nil
}

func (s *BatchService) cloneModules(
	ctx context.Context,
	sourceCourseID, destCourseID uint,
	assignmentIDMap, pageIDMap, quizIDMap, discussionIDMap map[uint]uint,
) error {
	allParams := repository.PaginationParams{Page: 1, PerPage: 1000}
	result, err := s.moduleRepo.ListByCourseID(ctx, sourceCourseID, allParams)
	if err != nil {
		return err
	}

	for _, m := range result.Items {
		newModule := &models.ContextModule{
			CourseID:                  destCourseID,
			Name:                      m.Name,
			Position:                  m.Position,
			UnlockAt:                  m.UnlockAt,
			RequireSequentialProgress: m.RequireSequentialProgress,
			WorkflowState:             "unpublished",
		}
		if err := s.moduleRepo.Create(ctx, newModule); err != nil {
			return fmt.Errorf("failed to clone module %d: %w", m.ID, err)
		}

		// Clone module items
		itemResult, err := s.moduleItemRepo.ListByModuleID(ctx, m.ID, allParams)
		if err != nil {
			return fmt.Errorf("failed to list items for module %d: %w", m.ID, err)
		}

		for _, item := range itemResult.Items {
			newItem := &models.ContentTag{
				ContextModuleID: newModule.ID,
				ContentType:     item.ContentType,
				Title:           item.Title,
				Position:        item.Position,
				URL:             item.URL,
				Indent:          item.Indent,
				NewTab:          item.NewTab,
				WorkflowState:   "unpublished",
			}

			// Remap content IDs to cloned resources
			if item.ContentID != nil {
				oldContentID := *item.ContentID
				var newContentID uint
				var found bool

				switch item.ContentType {
				case "Assignment":
					newContentID, found = assignmentIDMap[oldContentID]
				case "WikiPage":
					newContentID, found = pageIDMap[oldContentID]
				case "Quiz":
					newContentID, found = quizIDMap[oldContentID]
				case "DiscussionTopic":
					newContentID, found = discussionIDMap[oldContentID]
				default:
					// For ExternalUrl, ContextModuleSubHeader, etc., keep the original
					newContentID = oldContentID
					found = true
				}

				if found {
					newItem.ContentID = &newContentID
				}
				// If not found (content type wasn't included in clone), omit the content ID
			}

			if err := s.moduleItemRepo.Create(ctx, newItem); err != nil {
				return fmt.Errorf("failed to clone module item %d: %w", item.ID, err)
			}
		}
	}
	return nil
}

// BulkDateShift shifts all dates in a course by a number of days. It calculates the
// day shift from the difference between oldStartDate and newStartDate, then applies
// that shift to all assignment dates and calendar event dates.
func (s *BatchService) BulkDateShift(
	ctx context.Context,
	courseID uint,
	oldStartDate time.Time,
	newStartDate time.Time,
	dayShift int,
) (*DateShiftResult, error) {
	// If dayShift is not explicitly provided, calculate from the date difference
	if dayShift == 0 {
		dayShift = int(newStartDate.Sub(oldStartDate).Hours() / 24)
	}

	if dayShift == 0 {
		return &DateShiftResult{DayShift: 0}, nil
	}

	result := &DateShiftResult{DayShift: dayShift}
	shiftDuration := time.Duration(dayShift) * 24 * time.Hour

	// Shift assignment dates
	allParams := repository.PaginationParams{Page: 1, PerPage: 1000}
	assignments, err := s.assignmentRepo.ListByCourseID(ctx, courseID, allParams)
	if err != nil {
		return nil, fmt.Errorf("failed to list assignments: %w", err)
	}

	for _, a := range assignments.Items {
		shifted := false
		if a.DueAt != nil {
			t := a.DueAt.Add(shiftDuration)
			a.DueAt = &t
			shifted = true
		}
		if a.UnlockAt != nil {
			t := a.UnlockAt.Add(shiftDuration)
			a.UnlockAt = &t
			shifted = true
		}
		if a.LockAt != nil {
			t := a.LockAt.Add(shiftDuration)
			a.LockAt = &t
			shifted = true
		}
		if shifted {
			if err := s.assignmentRepo.Update(ctx, &a); err != nil {
				return nil, fmt.Errorf("failed to update assignment %d: %w", a.ID, err)
			}
			result.AssignmentsShifted++
		}
	}

	// Shift calendar event dates
	events, err := s.calendarEventRepo.ListByContext(ctx, "Course", courseID, allParams)
	if err != nil {
		return nil, fmt.Errorf("failed to list calendar events: %w", err)
	}

	for _, e := range events.Items {
		e.StartAt = e.StartAt.Add(shiftDuration)
		if e.EndAt != nil {
			t := e.EndAt.Add(shiftDuration)
			e.EndAt = &t
		}
		if err := s.calendarEventRepo.Update(ctx, &e); err != nil {
			return nil, fmt.Errorf("failed to update calendar event %d: %w", e.ID, err)
		}
		result.EventsShifted++
	}

	return result, nil
}

// BulkSendMessage sends a conversation message to all users matching the enrollment
// types in a given course. One conversation is created per recipient.
func (s *BatchService) BulkSendMessage(
	ctx context.Context,
	senderID uint,
	recipientCourseID uint,
	enrollmentTypes []string,
	subject string,
	body string,
) (*BulkMessageResult, error) {
	if subject == "" {
		return nil, errors.New("subject is required")
	}
	if body == "" {
		return nil, errors.New("message body is required")
	}
	if len(enrollmentTypes) == 0 {
		return nil, errors.New("at least one enrollment type is required")
	}

	result := &BulkMessageResult{}

	// Get all enrollments for the course
	allParams := repository.PaginationParams{Page: 1, PerPage: 1000}
	enrollments, err := s.enrollmentRepo.ListByCourseID(ctx, recipientCourseID, allParams)
	if err != nil {
		return nil, fmt.Errorf("failed to list enrollments: %w", err)
	}

	// Build a set of desired enrollment types for fast lookup
	typeSet := make(map[string]bool, len(enrollmentTypes))
	for _, t := range enrollmentTypes {
		typeSet[t] = true
	}

	// Collect unique user IDs matching the enrollment types (excluding the sender)
	recipientIDs := make(map[uint]bool)
	for _, e := range enrollments.Items {
		if e.WorkflowState != "active" {
			continue
		}
		if !typeSet[e.Type] {
			continue
		}
		if e.UserID == senderID {
			continue
		}
		recipientIDs[e.UserID] = true
	}

	now := time.Now()

	// Create one conversation per recipient
	for recipientID := range recipientIDs {
		conv := &models.Conversation{
			Subject:         subject,
			CreatedByUserID: senderID,
			LastMessageAt:   now,
			WorkflowState:   "active",
		}

		if err := s.conversationRepo.Create(ctx, conv); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to create conversation for user %d: %s", recipientID, err.Error()))
			continue
		}

		// Add sender as participant
		senderParticipant := &models.ConversationParticipant{
			ConversationID: conv.ID,
			UserID:         senderID,
			WorkflowState:  "active",
		}
		if err := s.convParticipantRepo.Create(ctx, senderParticipant); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to add sender participant for user %d: %s", recipientID, err.Error()))
			continue
		}

		// Add recipient as participant
		recipientParticipant := &models.ConversationParticipant{
			ConversationID: conv.ID,
			UserID:         recipientID,
			WorkflowState:  "active",
		}
		if err := s.convParticipantRepo.Create(ctx, recipientParticipant); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to add recipient participant for user %d: %s", recipientID, err.Error()))
			continue
		}

		// Create the message
		msg := &models.ConversationMessage{
			ConversationID: conv.ID,
			UserID:         senderID,
			Body:           body,
			WorkflowState:  "active",
		}
		if err := s.convMessageRepo.Create(ctx, msg); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to create message for user %d: %s", recipientID, err.Error()))
			continue
		}

		result.MessagesSent++
	}

	return result, nil
}

// BulkEnrollUsers enrolls multiple users into a course at once. Each enrollment
// request specifies a user ID, enrollment type, and optional section ID.
func (s *BatchService) BulkEnrollUsers(
	ctx context.Context,
	courseID uint,
	enrollments []BulkEnrollmentRequest,
) (*BulkEnrollResult, error) {
	if len(enrollments) == 0 {
		return nil, errors.New("at least one enrollment is required")
	}

	// Verify the course exists
	_, err := s.courseRepo.FindByID(ctx, courseID, 0)
	if err != nil {
		return nil, fmt.Errorf("course not found: %w", err)
	}

	validTypes := map[string]bool{
		"StudentEnrollment":  true,
		"TeacherEnrollment":  true,
		"TaEnrollment":       true,
		"ObserverEnrollment": true,
		"DesignerEnrollment": true,
	}

	result := &BulkEnrollResult{}

	for i, req := range enrollments {
		if req.UserID == 0 {
			result.Errors = append(result.Errors, fmt.Sprintf("enrollment[%d]: user_id is required", i))
			continue
		}

		if req.Type == "" {
			req.Type = "StudentEnrollment"
		}

		if !validTypes[req.Type] {
			result.Errors = append(result.Errors, fmt.Sprintf("enrollment[%d]: invalid enrollment type %q", i, req.Type))
			continue
		}

		// Check for existing enrollment
		existing, _ := s.enrollmentRepo.FindByUserAndCourse(ctx, req.UserID, courseID)
		if existing != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("enrollment[%d]: user %d is already enrolled in this course", i, req.UserID))
			continue
		}

		enrollment := &models.Enrollment{
			UserID:          req.UserID,
			CourseID:        courseID,
			CourseSectionID: req.SectionID,
			Type:            req.Type,
			Role:            req.Type,
			WorkflowState:   "active",
		}

		if err := s.enrollmentRepo.Create(ctx, enrollment); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("enrollment[%d]: %s", i, err.Error()))
			continue
		}

		result.Enrolled++
	}

	return result, nil
}

// BulkUpdateAssignmentDates updates due dates for multiple assignments at once.
// This is designed for use in the gradebook date editor.
func (s *BatchService) BulkUpdateAssignmentDates(
	ctx context.Context,
	courseID uint,
	updates []AssignmentDateUpdate,
) (*BulkDateUpdateResult, error) {
	if len(updates) == 0 {
		return nil, errors.New("at least one update is required")
	}

	result := &BulkDateUpdateResult{}

	for i, u := range updates {
		if u.AssignmentID == 0 {
			result.Errors = append(result.Errors, fmt.Sprintf("update[%d]: assignment_id is required", i))
			continue
		}

		assignment, err := s.assignmentRepo.FindByID(ctx, u.AssignmentID, 0)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("update[%d]: assignment %d not found", i, u.AssignmentID))
			continue
		}

		// Verify the assignment belongs to the specified course
		if assignment.CourseID != courseID {
			result.Errors = append(result.Errors, fmt.Sprintf("update[%d]: assignment %d does not belong to course %d", i, u.AssignmentID, courseID))
			continue
		}

		if u.DueAt != nil {
			assignment.DueAt = u.DueAt
		}
		if u.UnlockAt != nil {
			assignment.UnlockAt = u.UnlockAt
		}
		if u.LockAt != nil {
			assignment.LockAt = u.LockAt
		}

		if err := s.assignmentRepo.Update(ctx, assignment); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("update[%d]: failed to update assignment %d: %s", i, u.AssignmentID, err.Error()))
			continue
		}

		result.Updated++
	}

	return result, nil
}
