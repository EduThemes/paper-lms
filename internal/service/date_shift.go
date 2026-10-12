package service

// Date shifting for course copy and package import (G067), following
// the semantics of Canvas's "Adjust events and due dates" import option:
//
//   - remove_dates clears every optional date on the imported content;
//   - shift_dates maps each date from the old course window
//     [old_start, old_end] onto the new window [new_start, new_end]
//     proportionally, by calendar day, keeping the wall-clock time;
//   - a missing new_end is inferred so the course keeps its length, and a
//     missing old/new start falls back to the source/target course dates;
//   - the shifted date is then moved onto the weekday its original date
//     fell on (or the substituted weekday, e.g. Mon→Tue), preferring the
//     earlier occurrence unless that lands before new_start.
//
// Beyond Canvas: when only the two start dates are known (no old end),
// the content moves by the whole-day difference between them instead of
// being left unshifted.

import (
	"context"
	"math"
	"strconv"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// DateShifter maps content dates from a source course window onto a
// target window. A nil *DateShifter leaves every date unchanged.
type DateShifter struct {
	remove   bool
	loc      *time.Location
	scale    bool // proportional (both windows known) vs. day offset
	oldStart time.Time
	oldEnd   time.Time
	newStart time.Time
	newEnd   time.Time
	subs     map[time.Weekday]time.Weekday
}

// NewDateShifter resolves the options against the source and target
// courses. It returns nil when nothing should change.
func NewDateShifter(opts *models.DateShiftOptions, source, target *models.Course) *DateShifter {
	if opts == nil {
		return nil
	}
	if opts.RemoveDates {
		return &DateShifter{remove: true}
	}
	if !opts.ShiftDates {
		return nil
	}
	loc := time.UTC
	if opts.TimeZone != "" {
		if l, err := time.LoadLocation(opts.TimeZone); err == nil {
			loc = l
		}
	}
	d := &DateShifter{loc: loc, subs: map[time.Weekday]time.Weekday{}}

	// Explicit option dates are calendar dates (a date picker's value or
	// midnight UTC): read their date in UTC. Course dates are instants:
	// read their date in the shift zone.
	optDate := func(t *time.Time) (time.Time, bool) {
		if t == nil || t.IsZero() {
			return time.Time{}, false
		}
		u := t.UTC()
		return civilDate(u.Year(), u.Month(), u.Day()), true
	}
	courseDate := func(t *time.Time) (time.Time, bool) {
		if t == nil || t.IsZero() {
			return time.Time{}, false
		}
		l := t.In(loc)
		return civilDate(l.Year(), l.Month(), l.Day()), true
	}

	var hasOldStart, hasOldEnd, hasNewStart, hasNewEnd bool
	if d.oldStart, hasOldStart = optDate(opts.OldStartDate); !hasOldStart && source != nil {
		d.oldStart, hasOldStart = courseDate(source.StartAt)
	}
	if d.oldEnd, hasOldEnd = optDate(opts.OldEndDate); !hasOldEnd && source != nil {
		d.oldEnd, hasOldEnd = courseDate(source.EndAt)
	}
	if d.newStart, hasNewStart = optDate(opts.NewStartDate); !hasNewStart && target != nil {
		d.newStart, hasNewStart = courseDate(target.StartAt)
	}
	d.newEnd, hasNewEnd = optDate(opts.NewEndDate)

	// An end before its start is a typo, not a window.
	if hasOldStart && hasOldEnd && d.oldEnd.Before(d.oldStart) {
		hasOldEnd = false
	}
	if hasNewStart && hasNewEnd && d.newEnd.Before(d.newStart) {
		hasNewEnd = false
	}
	if !hasNewEnd && hasNewStart && hasOldStart && hasOldEnd {
		d.newEnd = d.newStart.Add(d.oldEnd.Sub(d.oldStart))
		hasNewEnd = true
	}

	for from, to := range opts.DaySubstitutions {
		f, err1 := strconv.Atoi(from)
		t, err2 := strconv.Atoi(to)
		if err1 == nil && err2 == nil && f >= 0 && f <= 6 && t >= 0 && t <= 6 {
			d.subs[time.Weekday(f)] = time.Weekday(t)
		}
	}

	switch {
	case hasOldStart && hasOldEnd && hasNewStart && hasNewEnd:
		d.scale = true
	case hasOldStart && hasNewStart:
		d.scale = false
	default:
		return nil
	}
	return d
}

func civilDate(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func daysBetween(a, b time.Time) int {
	return int(math.Round(b.Sub(a).Hours() / 24))
}

// Shift maps one instant. It returns nil when dates are being removed.
func (d *DateShifter) Shift(t time.Time) *time.Time {
	if d == nil {
		return &t
	}
	if d.remove {
		return nil
	}
	local := t.In(d.loc)
	oldDate := civilDate(local.Year(), local.Month(), local.Day())

	var newDate time.Time
	if d.scale {
		oldFull := daysBetween(d.oldStart, d.oldEnd)
		pct := 0.0
		if oldFull > 0 {
			pct = float64(daysBetween(d.oldStart, oldDate)) / float64(oldFull)
		}
		newFull := daysBetween(d.newStart, d.newEnd)
		newDate = d.newStart.AddDate(0, 0, int(math.Round(float64(newFull)*pct)))
	} else {
		newDate = oldDate.AddDate(0, 0, daysBetween(d.oldStart, d.newStart))
	}

	want, ok := d.subs[oldDate.Weekday()]
	if !ok {
		want = oldDate.Weekday()
	}
	if newDate.Weekday() != want {
		newDate = newDate.AddDate(0, 0, (int(want)-int(newDate.Weekday())+7)%7)
		if !newDate.AddDate(0, 0, -7).Before(d.newStart) {
			newDate = newDate.AddDate(0, 0, -7)
		}
	}

	out := time.Date(newDate.Year(), newDate.Month(), newDate.Day(),
		local.Hour(), local.Minute(), local.Second(), 0, d.loc)
	return &out
}

// ShiftPtr maps an optional instant; nil stays nil.
func (d *DateShifter) ShiftPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	return d.Shift(*t)
}

// ShiftRequired maps an instant that cannot be cleared (a calendar
// event's start): removal leaves it unchanged.
func (d *DateShifter) ShiftRequired(t time.Time) time.Time {
	if d != nil && d.remove {
		return t
	}
	if s := d.Shift(t); s != nil {
		return *s
	}
	return t
}

// ContentDateRepos is the set of repositories ShiftEntityDates writes
// through. A nil repo skips its content type.
type ContentDateRepos struct {
	Assignments interface {
		FindByID(ctx context.Context, id, accountID uint) (*models.Assignment, error)
		Update(ctx context.Context, a *models.Assignment) error
	}
	Quizzes interface {
		FindByID(ctx context.Context, id, accountID uint) (*models.Quiz, error)
		Update(ctx context.Context, q *models.Quiz) error
	}
	Discussions interface {
		FindByID(ctx context.Context, id, accountID uint) (*models.DiscussionTopic, error)
		Update(ctx context.Context, t *models.DiscussionTopic) error
	}
	Modules interface {
		FindByID(ctx context.Context, id, accountID uint) (*models.ContextModule, error)
		Update(ctx context.Context, m *models.ContextModule) error
	}
	Events interface {
		FindByID(ctx context.Context, id, accountID uint) (*models.CalendarEvent, error)
		Update(ctx context.Context, e *models.CalendarEvent) error
	}
}

// ShiftEntityDates applies the shifter to every dated entity in refs
// (rows a copy or import just created). It returns how many rows changed.
//
// The lookups pass accountID=0: auth-internal — every ref names a row
// this migration created in the (already tenant-checked) target course.
func ShiftEntityDates(ctx context.Context, repos ContentDateRepos, d *DateShifter, refs []EntityRef) (int, error) {
	if d == nil {
		return 0, nil
	}
	shifted := 0
	for _, ref := range refs {
		switch ref.Type {
		case "Assignment":
			if repos.Assignments == nil {
				continue
			}
			a, err := repos.Assignments.FindByID(ctx, ref.ID, 0)
			if err != nil || a == nil {
				continue
			}
			if a.DueAt == nil && a.UnlockAt == nil && a.LockAt == nil {
				continue
			}
			a.DueAt, a.UnlockAt, a.LockAt = d.ShiftPtr(a.DueAt), d.ShiftPtr(a.UnlockAt), d.ShiftPtr(a.LockAt)
			if err := repos.Assignments.Update(ctx, a); err != nil {
				return shifted, err
			}
			shifted++
		case "Quiz":
			if repos.Quizzes == nil {
				continue
			}
			q, err := repos.Quizzes.FindByID(ctx, ref.ID, 0)
			if err != nil || q == nil {
				continue
			}
			if q.DueAt == nil && q.UnlockAt == nil && q.LockAt == nil {
				continue
			}
			q.DueAt, q.UnlockAt, q.LockAt = d.ShiftPtr(q.DueAt), d.ShiftPtr(q.UnlockAt), d.ShiftPtr(q.LockAt)
			if err := repos.Quizzes.Update(ctx, q); err != nil {
				return shifted, err
			}
			shifted++
		case "DiscussionTopic":
			if repos.Discussions == nil {
				continue
			}
			t, err := repos.Discussions.FindByID(ctx, ref.ID, 0)
			if err != nil || t == nil {
				continue
			}
			if t.DelayedPostAt == nil && t.LockAt == nil {
				continue
			}
			t.DelayedPostAt, t.LockAt = d.ShiftPtr(t.DelayedPostAt), d.ShiftPtr(t.LockAt)
			if err := repos.Discussions.Update(ctx, t); err != nil {
				return shifted, err
			}
			shifted++
		case "ContextModule":
			if repos.Modules == nil {
				continue
			}
			m, err := repos.Modules.FindByID(ctx, ref.ID, 0)
			if err != nil || m == nil {
				continue
			}
			if m.UnlockAt == nil && m.EndAt == nil {
				continue
			}
			m.UnlockAt, m.EndAt = d.ShiftPtr(m.UnlockAt), d.ShiftPtr(m.EndAt)
			m.Items = nil
			if err := repos.Modules.Update(ctx, m); err != nil {
				return shifted, err
			}
			shifted++
		case "CalendarEvent":
			if repos.Events == nil {
				continue
			}
			e, err := repos.Events.FindByID(ctx, ref.ID, 0)
			if err != nil || e == nil {
				continue
			}
			e.StartAt = d.ShiftRequired(e.StartAt)
			e.EndAt = d.ShiftPtr(e.EndAt)
			if err := repos.Events.Update(ctx, e); err != nil {
				return shifted, err
			}
			shifted++
		}
	}
	return shifted, nil
}
