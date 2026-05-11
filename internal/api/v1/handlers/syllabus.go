package handlers

import (
	"sort"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

// groupColors is a palette of 8 colors for assignment groups, cycled through.
var groupColors = []string{
	"#3b82f6", // blue
	"#10b981", // emerald
	"#f59e0b", // amber
	"#f43f5e", // rose
	"#8b5cf6", // purple
	"#14b8a6", // teal
	"#f97316", // orange
	"#6366f1", // indigo
}

// SyllabusHandler aggregates data from multiple services to produce a
// composite syllabus view that is a structured improvement over Canvas's
// static HTML blob.
type SyllabusHandler struct {
	courseService           *service.CourseService
	assignmentService      *service.AssignmentService
	assignmentGroupService *service.AssignmentGroupService
	calendarService        *service.CalendarService
	gradingService         *service.GradingService
	enrollmentService      *service.EnrollmentService
	submissionService      *service.SubmissionService
}

// NewSyllabusHandler creates a new SyllabusHandler with all required service
// dependencies injected.
func NewSyllabusHandler(
	courseService *service.CourseService,
	assignmentService *service.AssignmentService,
	assignmentGroupService *service.AssignmentGroupService,
	calendarService *service.CalendarService,
	gradingService *service.GradingService,
	enrollmentService *service.EnrollmentService,
	submissionService *service.SubmissionService,
) *SyllabusHandler {
	return &SyllabusHandler{
		courseService:           courseService,
		assignmentService:      assignmentService,
		assignmentGroupService: assignmentGroupService,
		calendarService:        calendarService,
		gradingService:         gradingService,
		enrollmentService:      enrollmentService,
		submissionService:      submissionService,
	}
}

// syllabusGradingBreakdown represents one assignment group's weight info.
type syllabusGradingBreakdown struct {
	GroupName       string  `json:"group_name"`
	GroupWeight     float64 `json:"group_weight"`
	AssignmentCount int     `json:"assignment_count"`
	GroupColor      string  `json:"group_color"`
}

// syllabusTimelineItem represents a single item in the chronological timeline.
type syllabusTimelineItem struct {
	ID             uint       `json:"id"`
	Type           string     `json:"type"`
	Title          string     `json:"title"`
	DueAt          *time.Time `json:"due_at,omitempty"`
	StartAt        *time.Time `json:"start_at,omitempty"`
	PointsPossible *float64   `json:"points_possible,omitempty"`
	GroupName      string     `json:"group_name,omitempty"`
	GroupColor     string     `json:"group_color,omitempty"`
	Status         string     `json:"status,omitempty"`
}

// GetSyllabus handles GET /courses/:course_id/syllabus
// It aggregates course info, assignment groups, assignments, calendar events,
// and (for students) submission status into a single structured response.
func (h *SyllabusHandler) GetSyllabus(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	userID, _ := c.Locals("user_id").(uint)

	// 1. Fetch the course
	course, err := h.courseService.GetByID(c.Context(), uint(courseID))
	if err != nil {
		return responses.NotFound(c, "course")
	}

	// 2. Determine the user's role in this course
	userRole := ""
	if userID > 0 {
		role, roleErr := h.enrollmentService.GetUserRole(c.Context(), userID, uint(courseID))
		if roleErr == nil {
			userRole = role
		}
	}
	isStudent := userRole == "StudentEnrollment"

	// 3. Fetch all assignment groups for the course
	bigPage := repository.PaginationParams{Page: 1, PerPage: 1000}
	groupsResult, err := h.assignmentGroupService.ListByCourse(c.Context(), uint(courseID), bigPage)
	if err != nil {
		return responses.InternalError(c, "Could not fetch assignment groups")
	}

	// Build a lookup: group ID -> group info + assigned color
	type groupInfo struct {
		Name   string
		Weight float64
		Color  string
	}
	groupMap := make(map[uint]groupInfo, len(groupsResult.Items))
	for i, g := range groupsResult.Items {
		color := groupColors[i%len(groupColors)]
		groupMap[g.ID] = groupInfo{
			Name:   g.Name,
			Weight: g.GroupWeight,
			Color:  color,
		}
	}

	// 4. Fetch all assignments for the course
	assignmentsResult, err := h.assignmentService.ListByCourse(c.Context(), uint(courseID), bigPage)
	if err != nil {
		return responses.InternalError(c, "Could not fetch assignments")
	}

	// Count assignments per group for the grading breakdown
	groupAssignmentCount := make(map[uint]int)
	for _, a := range assignmentsResult.Items {
		if a.AssignmentGroupID != nil {
			groupAssignmentCount[*a.AssignmentGroupID]++
		}
	}

	// 5. Build grading breakdown
	gradingBreakdown := make([]syllabusGradingBreakdown, 0, len(groupsResult.Items))
	for _, g := range groupsResult.Items {
		info := groupMap[g.ID]
		gradingBreakdown = append(gradingBreakdown, syllabusGradingBreakdown{
			GroupName:       info.Name,
			GroupWeight:     info.Weight,
			AssignmentCount: groupAssignmentCount[g.ID],
			GroupColor:      info.Color,
		})
	}

	// 6. If the user is a student, fetch their submissions for status determination
	submissionByAssignment := make(map[uint]string) // assignment_id -> status
	if isStudent && userID > 0 {
		for _, a := range assignmentsResult.Items {
			sub, subErr := h.submissionService.GetByAssignmentAndUser(c.Context(), a.ID, userID)
			if subErr != nil {
				// No submission found
				now := time.Now()
				if a.DueAt != nil && now.After(*a.DueAt) {
					submissionByAssignment[a.ID] = "missing"
				} else {
					submissionByAssignment[a.ID] = "upcoming"
				}
				continue
			}
			switch sub.WorkflowState {
			case "graded":
				submissionByAssignment[a.ID] = "graded"
			case "submitted":
				submissionByAssignment[a.ID] = "submitted"
			default:
				now := time.Now()
				if a.DueAt != nil && now.After(*a.DueAt) {
					submissionByAssignment[a.ID] = "missing"
				} else {
					submissionByAssignment[a.ID] = "upcoming"
				}
			}
		}
	}

	// 7. Build timeline items from assignments
	timeline := make([]syllabusTimelineItem, 0, len(assignmentsResult.Items)+10)
	for _, a := range assignmentsResult.Items {
		if a.WorkflowState == "deleted" {
			continue
		}
		item := syllabusTimelineItem{
			ID:             a.ID,
			Type:           "assignment",
			Title:          a.Name,
			DueAt:          a.DueAt,
			PointsPossible: a.PointsPossible,
		}
		if a.AssignmentGroupID != nil {
			if info, ok := groupMap[*a.AssignmentGroupID]; ok {
				item.GroupName = info.Name
				item.GroupColor = info.Color
			}
		}
		if isStudent {
			item.Status = submissionByAssignment[a.ID]
		}
		timeline = append(timeline, item)
	}

	// 8. Fetch calendar events for the course and merge into timeline
	eventsResult, err := h.calendarService.ListByContext(c.Context(), "Course", uint(courseID), bigPage)
	if err == nil {
		for _, e := range eventsResult.Items {
			if e.WorkflowState == "deleted" {
				continue
			}
			startAt := e.StartAt
			timeline = append(timeline, syllabusTimelineItem{
				ID:      e.ID,
				Type:    "event",
				Title:   e.Title,
				StartAt: &startAt,
			})
		}
	}

	// 9. Sort timeline chronologically by due_at or start_at
	sort.Slice(timeline, func(i, j int) bool {
		ti := timelineItemSortKey(timeline[i])
		tj := timelineItemSortKey(timeline[j])
		if ti == nil && tj == nil {
			return timeline[i].Title < timeline[j].Title
		}
		if ti == nil {
			return false
		}
		if tj == nil {
			return true
		}
		return ti.Before(*tj)
	})

	// 10. Build composite response
	courseJSON := fiber.Map{
		"id":            course.ID,
		"name":          course.Name,
		"course_code":   course.CourseCode,
		"syllabus_body": course.SyllabusBody,
		"start_at":      course.StartAt,
		"end_at":        course.EndAt,
	}

	return c.JSON(fiber.Map{
		"course":            courseJSON,
		"grading_breakdown": gradingBreakdown,
		"timeline":          timeline,
		"user_role":         userRole,
	})
}

// timelineItemSortKey returns the relevant time for sorting a timeline item.
func timelineItemSortKey(item syllabusTimelineItem) *time.Time {
	if item.DueAt != nil {
		return item.DueAt
	}
	return item.StartAt
}
