package service

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// BlackoutDateStore persists course pacing blackout dates
// (postgres.BlackoutDateRepository).
type BlackoutDateStore interface {
	ListByCourse(ctx context.Context, courseID uint) ([]models.BlackoutDate, error)
	Create(ctx context.Context, b *models.BlackoutDate) error
	Delete(ctx context.Context, id, courseID uint) (bool, error)
}

// SetPacingRepos wires blackout dates and the quiz / discussion repos so
// a published pace also dates quizzes and graded discussions (G079).
func (s *CoursePaceService) SetPacingRepos(blackouts BlackoutDateStore, quizRepo repository.QuizRepository, topicRepo repository.DiscussionTopicRepository) {
	s.blackouts = blackouts
	s.quizRepo = quizRepo
	s.topicRepo = topicRepo
}

// ErrBlackoutDateInvalid is returned for a blackout date with no title or
// an end before its start.
var ErrBlackoutDateInvalid = errors.New("a blackout date needs a title and an end date on or after its start date")

// ListBlackoutDates returns the course's blackout dates.
func (s *CoursePaceService) ListBlackoutDates(ctx context.Context, courseID uint) ([]models.BlackoutDate, error) {
	if s.blackouts == nil {
		return []models.BlackoutDate{}, nil
	}
	return s.blackouts.ListByCourse(ctx, courseID)
}

// CreateBlackoutDate validates and stores a blackout date.
func (s *CoursePaceService) CreateBlackoutDate(ctx context.Context, b *models.BlackoutDate) error {
	if s.blackouts == nil {
		return errors.New("blackout dates are not configured")
	}
	b.EventTitle = strings.TrimSpace(SanitizePlainText(b.EventTitle))
	if b.EventTitle == "" || b.CourseID == 0 || b.StartDate.IsZero() || b.EndDate.IsZero() || dayOf(b.EndDate).Before(dayOf(b.StartDate)) {
		return ErrBlackoutDateInvalid
	}
	b.StartDate = dayOf(b.StartDate)
	b.EndDate = dayOf(b.EndDate)
	return s.blackouts.Create(ctx, b)
}

// DeleteBlackoutDate removes a blackout date of the course; found is
// false when id is not one of the course's blackout dates.
func (s *CoursePaceService) DeleteBlackoutDate(ctx context.Context, id, courseID uint) (bool, error) {
	if s.blackouts == nil {
		return false, nil
	}
	return s.blackouts.Delete(ctx, id, courseID)
}

func dayOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// isPaceDay reports whether work may be scheduled on day: not a weekend
// day when the pace excludes weekends, and not inside a blackout date.
func isPaceDay(day time.Time, excludeWeekends bool, blackouts []models.BlackoutDate) bool {
	if excludeWeekends {
		if wd := day.Weekday(); wd == time.Saturday || wd == time.Sunday {
			return false
		}
	}
	for _, b := range blackouts {
		if b.Covers(day) {
			return false
		}
	}
	return true
}

// advancePaceDays moves n schedulable days forward from start.
func advancePaceDays(start time.Time, n int, excludeWeekends bool, blackouts []models.BlackoutDate) time.Time {
	current := start
	for guard := 0; n > 0 && guard < 100000; guard++ {
		current = current.AddDate(0, 0, 1)
		if isPaceDay(current, excludeWeekends, blackouts) {
			n--
		}
	}
	return current
}

// paceDaysBetween counts the schedulable days in (start, end].
func paceDaysBetween(start, end time.Time, excludeWeekends bool, blackouts []models.BlackoutDate) int {
	n := 0
	last := dayOf(end)
	for d := start.AddDate(0, 0, 1); !dayOf(d).After(last); d = d.AddDate(0, 0, 1) {
		if isPaceDay(d, excludeWeekends, blackouts) {
			n++
		}
	}
	return n
}

// PaceSchedule returns the due date of each item given the item
// durations (days of work), counting only schedulable days from start.
// With hardEnd set (a pace with hard end dates) a plan that would run
// past the end date is compressed proportionally so the last item lands
// on it (Canvas compresses the same way).
func PaceSchedule(start time.Time, durations []int, excludeWeekends bool, blackouts []models.BlackoutDate, hardEnd *time.Time) []time.Time {
	out := make([]time.Time, len(durations))
	total := 0
	for _, d := range durations {
		if d > 0 {
			total += d
		}
	}
	if hardEnd != nil && total > 0 {
		avail := paceDaysBetween(start, *hardEnd, excludeWeekends, blackouts)
		if avail < total {
			if avail < 0 {
				avail = 0
			}
			cum := 0
			for i, d := range durations {
				if d > 0 {
					cum += d
				}
				target := int(math.Floor(float64(cum)*float64(avail)/float64(total) + 1e-9))
				out[i] = advancePaceDays(start, target, excludeWeekends, blackouts)
			}
			return out
		}
	}
	current := start
	for i, d := range durations {
		if d > 0 {
			current = advancePaceDays(current, d, excludeWeekends, blackouts)
		}
		out[i] = current
	}
	return out
}

// paceSchedule computes the due date of each pace item.
func (s *CoursePaceService) paceSchedule(ctx context.Context, pace *models.CoursePace, items []models.CoursePaceModuleItem) []time.Time {
	durations := make([]int, len(items))
	for i, it := range items {
		durations[i] = it.Duration
	}
	var blackouts []models.BlackoutDate
	if s.blackouts != nil {
		blackouts, _ = s.blackouts.ListByCourse(ctx, pace.CourseID)
	}
	var hardEnd *time.Time
	if pace.HardEndDates && pace.EndDate != nil {
		hardEnd = pace.EndDate
	}
	return PaceSchedule(s.paceStartDate(ctx, pace), durations, pace.ExcludeWeekends, blackouts, hardEnd)
}

// pacedAssignmentID maps a pace item's module item to the assignment
// whose dates the pace writes. Quizzes (which carry their own due date)
// are dated here directly for the course-wide pace and reported as
// handled; graded discussions resolve to their assignment. Content of
// another course is never touched (B039).
func (s *CoursePaceService) pacedAssignmentID(ctx context.Context, pace *models.CoursePace, item *models.ContentTag, due time.Time) (uint, bool) {
	if item.ContentID == nil {
		return 0, true
	}
	id := *item.ContentID
	switch item.ContentType {
	case "Assignment":
		return id, false
	case "DiscussionTopic":
		if s.topicRepo == nil {
			return 0, true
		}
		topic, err := s.topicRepo.FindByID(ctx, id, 0)
		if err != nil || topic == nil || topic.CourseID != pace.CourseID || topic.AssignmentID == nil {
			return 0, true
		}
		return *topic.AssignmentID, false
	case "Quizzes::Quiz", "Quiz":
		// Paper quizzes have no per-student overrides, so only the
		// course-wide pace dates them.
		if s.quizRepo == nil || pace.UserID != nil || pace.CourseSectionID != nil {
			return 0, true
		}
		quiz, err := s.quizRepo.FindByID(ctx, id, 0)
		if err != nil || quiz == nil || quiz.CourseID != pace.CourseID {
			return 0, true
		}
		d := due
		quiz.DueAt = &d
		_ = s.quizRepo.Update(ctx, quiz)
		return 0, true
	}
	return 0, true
}
