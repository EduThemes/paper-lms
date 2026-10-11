package service

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// MaxRecurringEvents caps how many occurrences one recurring event may
// expand to (Canvas uses the same limit).
const MaxRecurringEvents = 200

// RecurrenceRule is the subset of an RFC 5545 RRULE the calendar
// supports: FREQ (DAILY, WEEKLY, MONTHLY, YEARLY), INTERVAL, COUNT,
// UNTIL, BYDAY (with ordinals for MONTHLY/YEARLY, e.g. 2TU or -1FR),
// BYMONTHDAY and BYMONTH. A rule must be bounded by COUNT or UNTIL.
type RecurrenceRule struct {
	Freq       string
	Interval   int
	Count      int
	Until      *time.Time
	untilDate  string // UNTIL given as a bare date (YYYYMMDD): inclusive through that local day
	ByDay      []RecurrenceWeekday
	ByMonthDay []int
	ByMonth    []int
}

// RecurrenceWeekday is one BYDAY entry. N is the ordinal (0 = every such
// weekday in the period, 2 = the second, -1 = the last).
type RecurrenceWeekday struct {
	Day time.Weekday
	N   int
}

var rruleWeekdays = map[string]time.Weekday{
	"SU": time.Sunday, "MO": time.Monday, "TU": time.Tuesday, "WE": time.Wednesday,
	"TH": time.Thursday, "FR": time.Friday, "SA": time.Saturday,
}

// ErrInvalidRRule is wrapped by every ParseRRule error.
var ErrInvalidRRule = errors.New("invalid recurrence rule")

func rruleErr(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidRRule, fmt.Sprintf(format, a...))
}

// ParseRRule parses an RRULE value ("FREQ=WEEKLY;BYDAY=MO,WE;COUNT=10",
// with or without the "RRULE:" prefix).
func ParseRRule(s string) (*RecurrenceRule, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "RRULE:"), "rrule:")
	if s == "" {
		return nil, rruleErr("empty rule")
	}
	r := &RecurrenceRule{Interval: 1}
	for _, part := range strings.Split(s, ";") {
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return nil, rruleErr("malformed part %q", part)
		}
		key, val := strings.ToUpper(strings.TrimSpace(kv[0])), strings.ToUpper(strings.TrimSpace(kv[1]))
		switch key {
		case "FREQ":
			switch val {
			case "DAILY", "WEEKLY", "MONTHLY", "YEARLY":
				r.Freq = val
			default:
				return nil, rruleErr("unsupported FREQ %q", val)
			}
		case "INTERVAL":
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 || n > 365 {
				return nil, rruleErr("INTERVAL must be between 1 and 365")
			}
			r.Interval = n
		case "COUNT":
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 {
				return nil, rruleErr("COUNT must be a positive number")
			}
			r.Count = n
		case "UNTIL":
			if t, err := time.Parse("20060102T150405Z", val); err == nil {
				r.Until = &t
			} else if _, err := time.Parse("20060102", val); err == nil {
				r.untilDate = val
			} else {
				return nil, rruleErr("UNTIL must be YYYYMMDD or YYYYMMDDTHHMMSSZ")
			}
		case "BYDAY":
			for _, d := range strings.Split(val, ",") {
				d = strings.TrimSpace(d)
				if len(d) < 2 {
					return nil, rruleErr("bad BYDAY %q", d)
				}
				wd, ok := rruleWeekdays[d[len(d)-2:]]
				if !ok {
					return nil, rruleErr("bad BYDAY %q", d)
				}
				n := 0
				if prefix := d[:len(d)-2]; prefix != "" {
					v, err := strconv.Atoi(prefix)
					if err != nil || v == 0 || v < -5 || v > 5 {
						return nil, rruleErr("bad BYDAY ordinal %q", d)
					}
					n = v
				}
				r.ByDay = append(r.ByDay, RecurrenceWeekday{Day: wd, N: n})
			}
		case "BYMONTHDAY":
			for _, d := range strings.Split(val, ",") {
				v, err := strconv.Atoi(strings.TrimSpace(d))
				if err != nil || v == 0 || v < -31 || v > 31 {
					return nil, rruleErr("bad BYMONTHDAY %q", d)
				}
				r.ByMonthDay = append(r.ByMonthDay, v)
			}
		case "BYMONTH":
			for _, d := range strings.Split(val, ",") {
				v, err := strconv.Atoi(strings.TrimSpace(d))
				if err != nil || v < 1 || v > 12 {
					return nil, rruleErr("bad BYMONTH %q", d)
				}
				r.ByMonth = append(r.ByMonth, v)
			}
		case "WKST":
			// Weeks always start on Monday here (the RFC default).
		default:
			return nil, rruleErr("unsupported part %q", key)
		}
	}
	if r.Freq == "" {
		return nil, rruleErr("FREQ is required")
	}
	if r.Count == 0 && r.Until == nil && r.untilDate == "" {
		return nil, rruleErr("COUNT or UNTIL is required")
	}
	if r.Count > MaxRecurringEvents {
		return nil, rruleErr("a series is limited to %d events", MaxRecurringEvents)
	}
	for _, d := range r.ByDay {
		if d.N != 0 && r.Freq != "MONTHLY" && r.Freq != "YEARLY" {
			return nil, rruleErr("BYDAY ordinals are only allowed with MONTHLY or YEARLY")
		}
	}
	return r, nil
}

// String renders the rule back to RRULE text (without the prefix).
func (r *RecurrenceRule) String() string {
	parts := []string{"FREQ=" + r.Freq}
	if r.Interval > 1 {
		parts = append(parts, "INTERVAL="+strconv.Itoa(r.Interval))
	}
	if len(r.ByDay) > 0 {
		days := make([]string, len(r.ByDay))
		for i, d := range r.ByDay {
			code := weekdayCode(d.Day)
			if d.N != 0 {
				code = strconv.Itoa(d.N) + code
			}
			days[i] = code
		}
		parts = append(parts, "BYDAY="+strings.Join(days, ","))
	}
	if len(r.ByMonthDay) > 0 {
		parts = append(parts, "BYMONTHDAY="+joinInts(r.ByMonthDay))
	}
	if len(r.ByMonth) > 0 {
		parts = append(parts, "BYMONTH="+joinInts(r.ByMonth))
	}
	if r.Count > 0 {
		parts = append(parts, "COUNT="+strconv.Itoa(r.Count))
	}
	if r.Until != nil {
		parts = append(parts, "UNTIL="+r.Until.UTC().Format("20060102T150405Z"))
	} else if r.untilDate != "" {
		parts = append(parts, "UNTIL="+r.untilDate)
	}
	return strings.Join(parts, ";")
}

// WithUntil returns a copy of the rule bounded to end at until (COUNT
// dropped). Used to close the old part of a series split by an edit to
// "this and following events".
func (r *RecurrenceRule) WithUntil(until time.Time) *RecurrenceRule {
	cp := *r
	cp.Count = 0
	u := until.UTC()
	cp.Until = &u
	cp.untilDate = ""
	return &cp
}

func weekdayCode(d time.Weekday) string {
	for k, v := range rruleWeekdays {
		if v == d {
			return k
		}
	}
	return ""
}

func joinInts(vs []int) string {
	s := make([]string, len(vs))
	for i, v := range vs {
		s[i] = strconv.Itoa(v)
	}
	return strings.Join(s, ",")
}

// Occurrences expands the rule from dtstart. Dates are generated on the
// wall clock of loc (so a weekly 9:00 class stays at 9:00 across a DST
// change) and returned in UTC. The first occurrence is the first
// matching time at or after dtstart; at most MaxRecurringEvents are
// returned and an error is raised when the rule would produce more.
func (r *RecurrenceRule) Occurrences(dtstart time.Time, loc *time.Location) ([]time.Time, error) {
	if loc == nil {
		loc = time.UTC
	}
	start := dtstart.In(loc)
	until := r.Until
	if r.untilDate != "" {
		d, _ := time.ParseInLocation("20060102", r.untilDate, loc)
		end := d.AddDate(0, 0, 1).Add(-time.Second)
		until = &end
	}

	var out []time.Time
	// Each period yields its candidates in order; stop once COUNT or
	// UNTIL is reached. The period cap only guards against a rule that
	// never matches (e.g. BYMONTHDAY=31 with BYMONTH=2).
	const maxPeriods = 5000
	for k := 0; k < maxPeriods; k++ {
		cands := r.periodCandidates(start, k, loc)
		for _, c := range cands {
			if c.Before(start) {
				continue
			}
			if until != nil && c.After(*until) {
				return finishOccurrences(out)
			}
			out = append(out, c.UTC())
			if r.Count > 0 && len(out) >= r.Count {
				return finishOccurrences(out)
			}
			if len(out) > MaxRecurringEvents {
				return nil, rruleErr("a series is limited to %d events", MaxRecurringEvents)
			}
		}
	}
	return finishOccurrences(out)
}

func finishOccurrences(out []time.Time) ([]time.Time, error) {
	if len(out) == 0 {
		return nil, rruleErr("the rule does not produce any events")
	}
	return out, nil
}

// periodCandidates lists the sorted local times the rule matches in the
// k-th period (day, week, month or year) counted from start.
func (r *RecurrenceRule) periodCandidates(start time.Time, k int, loc *time.Location) []time.Time {
	h, m, s := start.Clock()
	at := func(y int, mo time.Month, d int) time.Time {
		return time.Date(y, mo, d, h, m, s, 0, loc)
	}
	var cands []time.Time
	switch r.Freq {
	case "DAILY":
		d := start.AddDate(0, 0, k*r.Interval)
		c := at(d.Year(), d.Month(), d.Day())
		if r.matchesFilters(c) {
			cands = append(cands, c)
		}
	case "WEEKLY":
		// Monday of start's week, then step whole weeks.
		offset := (int(start.Weekday()) + 6) % 7
		monday := at(start.Year(), start.Month(), start.Day()-offset).AddDate(0, 0, 7*k*r.Interval)
		days := r.ByDay
		if len(days) == 0 {
			days = []RecurrenceWeekday{{Day: start.Weekday()}}
		}
		for _, wd := range days {
			idx := (int(wd.Day) + 6) % 7
			c := at(monday.Year(), monday.Month(), monday.Day()+idx)
			if r.monthAllowed(c) {
				cands = append(cands, c)
			}
		}
	case "MONTHLY":
		first := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, loc).AddDate(0, k*r.Interval, 0)
		if !r.monthAllowed(first) {
			break
		}
		cands = r.daysInMonth(first.Year(), first.Month(), start.Day(), at)
	case "YEARLY":
		year := start.Year() + k*r.Interval
		months := r.ByMonth
		if len(months) == 0 {
			months = []int{int(start.Month())}
		}
		for _, mo := range months {
			if len(r.ByDay) == 0 && len(r.ByMonthDay) == 0 {
				if c := at(year, time.Month(mo), start.Day()); c.Day() == start.Day() {
					cands = append(cands, c)
				}
				continue
			}
			cands = append(cands, r.daysInMonth(year, time.Month(mo), start.Day(), at)...)
		}
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].Before(cands[j]) })
	return dedupeTimes(cands)
}

// daysInMonth applies BYMONTHDAY / BYDAY inside one month. With neither
// it falls back to defaultDay, skipping months that lack it (the 31st).
func (r *RecurrenceRule) daysInMonth(year int, month time.Month, defaultDay int, at func(int, time.Month, int) time.Time) []time.Time {
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	var days []int
	switch {
	case len(r.ByMonthDay) > 0:
		for _, d := range r.ByMonthDay {
			if d < 0 {
				d = last + d + 1
			}
			if d >= 1 && d <= last {
				days = append(days, d)
			}
		}
		if len(r.ByDay) > 0 {
			days = filterDays(days, func(d int) bool {
				return r.byDayMatches(year, month, d, last)
			})
		}
	case len(r.ByDay) > 0:
		for d := 1; d <= last; d++ {
			if r.byDayMatches(year, month, d, last) {
				days = append(days, d)
			}
		}
	default:
		if defaultDay <= last {
			days = []int{defaultDay}
		}
	}
	out := make([]time.Time, 0, len(days))
	for _, d := range days {
		out = append(out, at(year, month, d))
	}
	return out
}

func (r *RecurrenceRule) byDayMatches(year int, month time.Month, day, last int) bool {
	wd := time.Date(year, month, day, 12, 0, 0, 0, time.UTC).Weekday()
	for _, b := range r.ByDay {
		if b.Day != wd {
			continue
		}
		switch {
		case b.N == 0:
			return true
		case b.N > 0 && (day-1)/7+1 == b.N:
			return true
		case b.N < 0 && (last-day)/7+1 == -b.N:
			return true
		}
	}
	return false
}

// matchesFilters applies BYMONTH / BYMONTHDAY / BYDAY as filters (DAILY).
func (r *RecurrenceRule) matchesFilters(t time.Time) bool {
	if !r.monthAllowed(t) {
		return false
	}
	if len(r.ByMonthDay) > 0 {
		last := time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
		ok := false
		for _, d := range r.ByMonthDay {
			if d < 0 {
				d = last + d + 1
			}
			if d == t.Day() {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if len(r.ByDay) > 0 {
		ok := false
		for _, b := range r.ByDay {
			if b.Day == t.Weekday() {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

func (r *RecurrenceRule) monthAllowed(t time.Time) bool {
	if len(r.ByMonth) == 0 || r.Freq == "YEARLY" {
		return true
	}
	for _, m := range r.ByMonth {
		if int(t.Month()) == m {
			return true
		}
	}
	return false
}

func filterDays(days []int, keep func(int) bool) []int {
	out := days[:0]
	for _, d := range days {
		if keep(d) {
			out = append(out, d)
		}
	}
	return out
}

func dedupeTimes(ts []time.Time) []time.Time {
	if len(ts) < 2 {
		return ts
	}
	out := ts[:1]
	for _, t := range ts[1:] {
		if !t.Equal(out[len(out)-1]) {
			out = append(out, t)
		}
	}
	return out
}

// LoadCalendarLocation resolves an IANA zone name, falling back to UTC
// for an empty or unknown name.
func LoadCalendarLocation(name string) *time.Location {
	if name == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}
