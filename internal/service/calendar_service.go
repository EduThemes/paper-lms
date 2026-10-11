package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type CalendarService struct {
	repo repository.CalendarEventRepository
}

func NewCalendarService(repo repository.CalendarEventRepository) *CalendarService {
	return &CalendarService{repo: repo}
}

// Scopes for editing or deleting one occurrence of a recurring series
// (Canvas: which=one|all|following).
const (
	SeriesScopeOne       = "one"
	SeriesScopeAll       = "all"
	SeriesScopeFollowing = "following"
)

var (
	ErrCalendarEndBeforeStart   = errors.New("calendar event end_at must not be before start_at")
	ErrCalendarSeriesAndSection = errors.New("an event cannot both repeat and have section-specific times")
	ErrCalendarSectionsCourse   = errors.New("section-specific times are only allowed on course events")
	ErrCalendarBadScope         = errors.New("which must be one of: one, all, following")
	// ErrCalendarValidation matches (errors.Is) every input-validation
	// failure so handlers can answer 400 with the message.
	ErrCalendarValidation = errors.New("invalid calendar event")
)

type calendarValidationError string

func (e calendarValidationError) Error() string        { return string(e) }
func (e calendarValidationError) Is(target error) bool { return target == ErrCalendarValidation }

func calValidation(msg string) error { return calendarValidationError(msg) }

func (s *CalendarService) Create(ctx context.Context, event *models.CalendarEvent) error {
	if event.Title == "" {
		return calValidation("calendar event title is required")
	}
	if event.StartAt.IsZero() {
		return calValidation("calendar event start_at is required")
	}
	if event.EndAt != nil && event.EndAt.Before(event.StartAt) {
		return ErrCalendarEndBeforeStart
	}
	if event.WorkflowState == "" {
		event.WorkflowState = "active"
	}
	return s.repo.Create(ctx, event)
}

func (s *CalendarService) GetByID(ctx context.Context, id, accountID uint) (*models.CalendarEvent, error) {
	return s.repo.FindByID(ctx, id, accountID)
}

func (s *CalendarService) Update(ctx context.Context, event *models.CalendarEvent) error {
	return s.repo.Update(ctx, event)
}

func (s *CalendarService) Delete(ctx context.Context, id uint) error {
	return s.repo.Delete(ctx, id)
}

func (s *CalendarService) ListByContext(ctx context.Context, contextType string, contextID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.CalendarEvent], error) {
	return s.repo.ListByContext(ctx, contextType, contextID, params)
}

// ListChildren returns the section-specific children of a course event.
func (s *CalendarService) ListChildren(ctx context.Context, parentID uint) ([]models.CalendarEvent, error) {
	return s.repo.ListChildren(ctx, parentID)
}

// ListSeries returns every occurrence of a recurring series.
func (s *CalendarService) ListSeries(ctx context.Context, seriesUUID string) ([]models.CalendarEvent, error) {
	return s.repo.ListBySeries(ctx, seriesUUID)
}

// ChildEventInput is one section's times for a course event.
type ChildEventInput struct {
	SectionID uint
	StartAt   time.Time
	EndAt     *time.Time
}

// CreateOptions extends Create with recurrence and section times.
type CreateOptions struct {
	RRule    string
	Location *time.Location
	Children []ChildEventInput
}

// CreateWithOptions creates a plain event, a recurring series (one row
// per occurrence, the first one the series head) or a course event with
// section-specific child events. It returns every row it created; the
// first is the event the caller asked for (the head, or the parent).
func (s *CalendarService) CreateWithOptions(ctx context.Context, event *models.CalendarEvent, opts CreateOptions) ([]models.CalendarEvent, error) {
	rrule := strings.TrimSpace(opts.RRule)
	if rrule != "" && len(opts.Children) > 0 {
		return nil, ErrCalendarSeriesAndSection
	}
	if len(opts.Children) > 0 {
		return s.createWithChildren(ctx, event, opts.Children)
	}
	if rrule == "" {
		if err := s.Create(ctx, event); err != nil {
			return nil, err
		}
		return []models.CalendarEvent{*event}, nil
	}
	if event.Title == "" {
		return nil, calValidation("calendar event title is required")
	}
	if event.StartAt.IsZero() {
		return nil, calValidation("calendar event start_at is required")
	}
	if event.EndAt != nil && event.EndAt.Before(event.StartAt) {
		return nil, ErrCalendarEndBeforeStart
	}
	rule, err := ParseRRule(rrule)
	if err != nil {
		return nil, err
	}
	rows, err := buildSeries(event, rule, opts.Location, uuid.NewString())
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateMany(ctx, rows); err != nil {
		return nil, err
	}
	out := make([]models.CalendarEvent, len(rows))
	for i, r := range rows {
		out[i] = *r
	}
	*event = out[0]
	return out, nil
}

// buildSeries expands template into one row per occurrence of rule.
func buildSeries(template *models.CalendarEvent, rule *RecurrenceRule, loc *time.Location, seriesUUID string) ([]*models.CalendarEvent, error) {
	starts, err := rule.Occurrences(template.StartAt, loc)
	if err != nil {
		return nil, err
	}
	var dur *time.Duration
	if template.EndAt != nil {
		d := template.EndAt.Sub(template.StartAt)
		dur = &d
	}
	ruleText := rule.String()
	rows := make([]*models.CalendarEvent, 0, len(starts))
	for i, st := range starts {
		e := *template
		e.ID = 0
		e.StartAt = st
		e.EndAt = nil
		if dur != nil {
			end := st.Add(*dur)
			e.EndAt = &end
		}
		id := seriesUUID
		e.SeriesUUID = &id
		e.RRule = ruleText
		e.SeriesHead = i == 0
		if e.WorkflowState == "" {
			e.WorkflowState = "active"
		}
		rows = append(rows, &e)
	}
	return rows, nil
}

func (s *CalendarService) createWithChildren(ctx context.Context, parent *models.CalendarEvent, children []ChildEventInput) ([]models.CalendarEvent, error) {
	if parent.ContextType != models.CalendarContextCourse {
		return nil, ErrCalendarSectionsCourse
	}
	if err := validateChildren(children); err != nil {
		return nil, err
	}
	setParentRange(parent, children)
	if err := s.Create(ctx, parent); err != nil {
		return nil, err
	}
	rows := make([]*models.CalendarEvent, 0, len(children))
	for _, ch := range children {
		rows = append(rows, childFromParent(parent, ch))
	}
	if err := s.repo.CreateMany(ctx, rows); err != nil {
		_ = s.repo.Delete(ctx, parent.ID)
		return nil, err
	}
	out := []models.CalendarEvent{*parent}
	for _, r := range rows {
		out = append(out, *r)
	}
	return out, nil
}

func validateChildren(children []ChildEventInput) error {
	seen := map[uint]bool{}
	for _, ch := range children {
		if ch.SectionID == 0 {
			return calValidation("each section time needs a section")
		}
		if seen[ch.SectionID] {
			return calValidation("a section can only have one time per event")
		}
		seen[ch.SectionID] = true
		if ch.StartAt.IsZero() {
			return calValidation("each section time needs a start_at")
		}
		if ch.EndAt != nil && ch.EndAt.Before(ch.StartAt) {
			return ErrCalendarEndBeforeStart
		}
	}
	return nil
}

// setParentRange makes the parent span its children (Canvas keeps the
// parent's range as the min start / max end of the section times).
func setParentRange(parent *models.CalendarEvent, children []ChildEventInput) {
	if len(children) == 0 {
		return
	}
	minStart := children[0].StartAt
	var maxEnd *time.Time
	for _, ch := range children {
		if ch.StartAt.Before(minStart) {
			minStart = ch.StartAt
		}
		end := ch.StartAt
		if ch.EndAt != nil {
			end = *ch.EndAt
		}
		if maxEnd == nil || end.After(*maxEnd) {
			e := end
			maxEnd = &e
		}
	}
	parent.StartAt = minStart
	parent.EndAt = maxEnd
}

func childFromParent(parent *models.CalendarEvent, ch ChildEventInput) *models.CalendarEvent {
	pid := parent.ID
	return &models.CalendarEvent{
		ContextType:           models.CalendarContextCourseSection,
		ContextID:             ch.SectionID,
		Title:                 parent.Title,
		Description:           parent.Description,
		StartAt:               ch.StartAt,
		EndAt:                 ch.EndAt,
		LocationName:          parent.LocationName,
		LocationAddress:       parent.LocationAddress,
		AllDay:                parent.AllDay,
		CreatedByUserID:       parent.CreatedByUserID,
		WorkflowState:         "active",
		ParentCalendarEventID: &pid,
	}
}

// EventChanges is a partial update. Nil fields are left alone; EndAtSet
// distinguishes "clear end_at" from "leave it".
type EventChanges struct {
	Title           *string
	Description     *string
	StartAt         *time.Time
	EndAt           *time.Time
	EndAtSet        bool
	LocationName    *string
	LocationAddress *string
	AllDay          *bool
	RRule           *string
	Location        *time.Location
	// Children replaces the section times when non-nil; RemoveChildren
	// turns a sectioned event back into a plain course event.
	Children       *[]ChildEventInput
	RemoveChildren bool
}

func (ch *EventChanges) applyAttrs(e *models.CalendarEvent) {
	if ch.Title != nil {
		e.Title = *ch.Title
	}
	if ch.Description != nil {
		e.Description = *ch.Description
	}
	if ch.LocationName != nil {
		e.LocationName = *ch.LocationName
	}
	if ch.LocationAddress != nil {
		e.LocationAddress = *ch.LocationAddress
	}
	if ch.AllDay != nil {
		e.AllDay = *ch.AllDay
	}
}

// UpdateWithScope applies changes to event and, for a recurring series,
// to the occurrences named by which. It returns the rows that now make up
// the edited range (the edited event first).
func (s *CalendarService) UpdateWithScope(ctx context.Context, event *models.CalendarEvent, ch EventChanges, which string) ([]models.CalendarEvent, error) {
	if which == "" {
		which = SeriesScopeOne
	}
	if which != SeriesScopeOne && which != SeriesScopeAll && which != SeriesScopeFollowing {
		return nil, ErrCalendarBadScope
	}
	if ch.Title != nil && strings.TrimSpace(*ch.Title) == "" {
		return nil, calValidation("calendar event title is required")
	}
	newRule := ""
	ruleChanged := false
	if ch.RRule != nil {
		newRule = strings.TrimSpace(*ch.RRule)
		ruleChanged = newRule != "" && newRule != event.RRule
	}
	hasChildren := ch.Children != nil && len(*ch.Children) > 0
	if hasChildren && (event.SeriesUUID != nil || newRule != "") {
		return nil, ErrCalendarSeriesAndSection
	}

	// Section times on a course event.
	if ch.Children != nil || ch.RemoveChildren {
		return s.updateChildren(ctx, event, ch)
	}

	// A plain event that gains a rule becomes the head of a new series.
	if event.SeriesUUID == nil {
		if newRule != "" {
			existing, err := s.repo.ListChildren(ctx, event.ID)
			if err != nil {
				return nil, err
			}
			if len(existing) > 0 {
				return nil, ErrCalendarSeriesAndSection
			}
			return s.convertToSeries(ctx, event, ch, newRule)
		}
		if err := applyTimes(event, ch); err != nil {
			return nil, err
		}
		ch.applyAttrs(event)
		if err := s.repo.Update(ctx, event); err != nil {
			return nil, err
		}
		if err := s.syncChildren(ctx, event); err != nil {
			return nil, err
		}
		return []models.CalendarEvent{*event}, nil
	}

	if which == SeriesScopeOne {
		if ruleChanged {
			return nil, calValidation("to change how an event repeats, edit all events or this and following events")
		}
		if err := applyTimes(event, ch); err != nil {
			return nil, err
		}
		ch.applyAttrs(event)
		if err := s.repo.Update(ctx, event); err != nil {
			return nil, err
		}
		return []models.CalendarEvent{*event}, nil
	}

	series, err := s.repo.ListBySeries(ctx, *event.SeriesUUID)
	if err != nil {
		return nil, err
	}
	targets, earlier := splitSeries(series, event, which)
	if len(targets) == 0 {
		targets = []models.CalendarEvent{*event}
	}

	if ruleChanged {
		return s.regenerateSeries(ctx, event, ch, which, newRule, targets, earlier)
	}

	// Same rule: shift every target by the edited event's start delta and
	// give each the new duration when end_at changed.
	edited := *event
	if err := applyTimes(&edited, ch); err != nil {
		return nil, err
	}
	delta := edited.StartAt.Sub(event.StartAt)
	var newDur *time.Duration
	if ch.EndAtSet && edited.EndAt != nil {
		d := edited.EndAt.Sub(edited.StartAt)
		newDur = &d
	}
	rows := make([]*models.CalendarEvent, 0, len(targets))
	for i := range targets {
		t := targets[i]
		oldDur := time.Duration(-1)
		if t.EndAt != nil {
			oldDur = t.EndAt.Sub(t.StartAt)
		}
		t.StartAt = t.StartAt.Add(delta)
		switch {
		case ch.EndAtSet && ch.EndAt == nil:
			t.EndAt = nil
		case newDur != nil:
			end := t.StartAt.Add(*newDur)
			t.EndAt = &end
		case oldDur >= 0:
			end := t.StartAt.Add(oldDur)
			t.EndAt = &end
		}
		ch.applyAttrs(&t)
		rows = append(rows, &t)
	}
	if err := s.repo.SaveMany(ctx, rows); err != nil {
		return nil, err
	}
	out := make([]models.CalendarEvent, 0, len(rows))
	for _, r := range rows {
		if r.ID == event.ID {
			*event = *r
			out = append([]models.CalendarEvent{*r}, out...)
			continue
		}
		out = append(out, *r)
	}
	return out, nil
}

// splitSeries returns the rows `which` targets and, for "following", the
// earlier rows left behind.
func splitSeries(series []models.CalendarEvent, event *models.CalendarEvent, which string) (targets, earlier []models.CalendarEvent) {
	if which == SeriesScopeAll {
		return series, nil
	}
	for _, e := range series {
		if e.ID == event.ID || !e.StartAt.Before(event.StartAt) {
			targets = append(targets, e)
		} else {
			earlier = append(earlier, e)
		}
	}
	return targets, earlier
}

func applyTimes(e *models.CalendarEvent, ch EventChanges) error {
	if ch.StartAt != nil {
		e.StartAt = *ch.StartAt
	}
	if ch.EndAtSet {
		e.EndAt = ch.EndAt
	}
	if e.EndAt != nil && e.EndAt.Before(e.StartAt) {
		return ErrCalendarEndBeforeStart
	}
	return nil
}

func (s *CalendarService) convertToSeries(ctx context.Context, event *models.CalendarEvent, ch EventChanges, rrule string) ([]models.CalendarEvent, error) {
	rule, err := ParseRRule(rrule)
	if err != nil {
		return nil, err
	}
	template := *event
	if err := applyTimes(&template, ch); err != nil {
		return nil, err
	}
	ch.applyAttrs(&template)
	rows, err := buildSeries(&template, rule, ch.Location, uuid.NewString())
	if err != nil {
		return nil, err
	}
	// The existing row becomes the head (it keeps its id); the rest are new.
	head := rows[0]
	head.ID = event.ID
	head.CreatedAt = event.CreatedAt
	if err := s.repo.Update(ctx, head); err != nil {
		return nil, err
	}
	if err := s.repo.CreateMany(ctx, rows[1:]); err != nil {
		return nil, err
	}
	*event = *head
	out := make([]models.CalendarEvent, len(rows))
	for i, r := range rows {
		out[i] = *r
	}
	return out, nil
}

// regenerateSeries rebuilds the targeted occurrences from a new rule.
// "all" keeps the series id and re-anchors on the first occurrence;
// "following" starts a new series at the edited event and closes the
// earlier part with an UNTIL just before it.
func (s *CalendarService) regenerateSeries(ctx context.Context, event *models.CalendarEvent, ch EventChanges, which, rrule string, targets, earlier []models.CalendarEvent) ([]models.CalendarEvent, error) {
	rule, err := ParseRRule(rrule)
	if err != nil {
		return nil, err
	}
	anchor := targets[0]
	if which == SeriesScopeFollowing {
		anchor = *event
	}
	edited := *event
	if err := applyTimes(&edited, ch); err != nil {
		return nil, err
	}
	template := anchor
	template.StartAt = anchor.StartAt.Add(edited.StartAt.Sub(event.StartAt))
	template.EndAt = nil
	if edited.EndAt != nil {
		end := template.StartAt.Add(edited.EndAt.Sub(edited.StartAt))
		template.EndAt = &end
	}
	ch.applyAttrs(&template)

	seriesID := *event.SeriesUUID
	if which == SeriesScopeFollowing && len(earlier) > 0 {
		seriesID = uuid.NewString()
	}
	rows, err := buildSeries(&template, rule, ch.Location, seriesID)
	if err != nil {
		return nil, err
	}

	ids := make([]uint, 0, len(targets))
	for _, t := range targets {
		ids = append(ids, t.ID)
	}
	if err := s.repo.DeleteMany(ctx, ids); err != nil {
		return nil, err
	}
	if err := s.repo.CreateMany(ctx, rows); err != nil {
		return nil, err
	}
	if len(earlier) > 0 {
		if err := s.closeSeriesBefore(ctx, earlier, event.StartAt); err != nil {
			return nil, err
		}
	}
	*event = *rows[0]
	out := make([]models.CalendarEvent, len(rows))
	for i, r := range rows {
		out[i] = *r
	}
	return out, nil
}

// closeSeriesBefore bounds the remaining (earlier) occurrences' rule so
// it ends just before cut.
func (s *CalendarService) closeSeriesBefore(ctx context.Context, earlier []models.CalendarEvent, cut time.Time) error {
	rows := make([]*models.CalendarEvent, 0, len(earlier))
	for i := range earlier {
		e := earlier[i]
		if rule, err := ParseRRule(e.RRule); err == nil {
			e.RRule = rule.WithUntil(cut.Add(-time.Second)).String()
		}
		rows = append(rows, &e)
	}
	return s.repo.SaveMany(ctx, rows)
}

// updateChildren replaces a course event's section times (or removes
// them) and keeps the parent spanning its children.
func (s *CalendarService) updateChildren(ctx context.Context, parent *models.CalendarEvent, ch EventChanges) ([]models.CalendarEvent, error) {
	if parent.ContextType != models.CalendarContextCourse {
		return nil, ErrCalendarSectionsCourse
	}
	existing, err := s.repo.ListChildren(ctx, parent.ID)
	if err != nil {
		return nil, err
	}
	var want []ChildEventInput
	if ch.Children != nil && !ch.RemoveChildren {
		want = *ch.Children
	}
	if err := validateChildren(want); err != nil {
		return nil, err
	}

	ch.applyAttrs(parent)
	if len(want) > 0 {
		setParentRange(parent, want)
	} else if err := applyTimes(parent, ch); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, parent); err != nil {
		return nil, err
	}

	bySection := map[uint]models.CalendarEvent{}
	for _, e := range existing {
		bySection[e.ContextID] = e
	}
	var creates, saves []*models.CalendarEvent
	keep := map[uint]bool{}
	for _, w := range want {
		keep[w.SectionID] = true
		if cur, ok := bySection[w.SectionID]; ok {
			cur.StartAt = w.StartAt
			cur.EndAt = w.EndAt
			copyParentAttrs(parent, &cur)
			c := cur
			saves = append(saves, &c)
			continue
		}
		creates = append(creates, childFromParent(parent, w))
	}
	var drop []uint
	for _, e := range existing {
		if !keep[e.ContextID] {
			drop = append(drop, e.ID)
		}
	}
	if err := s.repo.DeleteMany(ctx, drop); err != nil {
		return nil, err
	}
	if err := s.repo.SaveMany(ctx, saves); err != nil {
		return nil, err
	}
	if err := s.repo.CreateMany(ctx, creates); err != nil {
		return nil, err
	}
	out := []models.CalendarEvent{*parent}
	for _, r := range append(saves, creates...) {
		out = append(out, *r)
	}
	return out, nil
}

func copyParentAttrs(parent, child *models.CalendarEvent) {
	child.Title = parent.Title
	child.Description = parent.Description
	child.LocationName = parent.LocationName
	child.LocationAddress = parent.LocationAddress
	child.AllDay = parent.AllDay
}

// syncChildren copies the parent's shared fields onto its children after
// a plain edit of the parent.
func (s *CalendarService) syncChildren(ctx context.Context, parent *models.CalendarEvent) error {
	children, err := s.repo.ListChildren(ctx, parent.ID)
	if err != nil || len(children) == 0 {
		return err
	}
	rows := make([]*models.CalendarEvent, 0, len(children))
	for i := range children {
		c := children[i]
		copyParentAttrs(parent, &c)
		rows = append(rows, &c)
	}
	return s.repo.SaveMany(ctx, rows)
}

// DeleteWithScope soft-deletes the event (with its section children) or,
// for a recurring series, the occurrences named by which. It returns the
// ids it deleted.
func (s *CalendarService) DeleteWithScope(ctx context.Context, event *models.CalendarEvent, which string) ([]uint, error) {
	if which == "" {
		which = SeriesScopeOne
	}
	if which != SeriesScopeOne && which != SeriesScopeAll && which != SeriesScopeFollowing {
		return nil, ErrCalendarBadScope
	}
	ids := []uint{event.ID}
	if children, err := s.repo.ListChildren(ctx, event.ID); err == nil {
		for _, c := range children {
			ids = append(ids, c.ID)
		}
	}
	if event.SeriesUUID == nil {
		return ids, s.repo.DeleteMany(ctx, ids)
	}
	series, err := s.repo.ListBySeries(ctx, *event.SeriesUUID)
	if err != nil {
		return nil, err
	}
	switch which {
	case SeriesScopeOne:
		if err := s.repo.DeleteMany(ctx, ids); err != nil {
			return nil, err
		}
		// Promote the next occurrence when the head goes.
		if event.SeriesHead {
			for i := range series {
				if series[i].ID != event.ID {
					next := series[i]
					next.SeriesHead = true
					if err := s.repo.Update(ctx, &next); err != nil {
						return nil, err
					}
					break
				}
			}
		}
		return ids, nil
	default:
		targets, earlier := splitSeries(series, event, which)
		ids = ids[:0]
		for _, t := range targets {
			ids = append(ids, t.ID)
		}
		if err := s.repo.DeleteMany(ctx, ids); err != nil {
			return nil, err
		}
		if len(earlier) > 0 {
			if err := s.closeSeriesBefore(ctx, earlier, event.StartAt); err != nil {
				return nil, err
			}
		}
		return ids, nil
	}
}

// ExportAsICalendar renders one calendar's events as ICS, dating all-day
// events in loc (the reader's zone; nil means UTC).
func (s *CalendarService) ExportAsICalendar(ctx context.Context, contextType string, contextID uint, startAt, endAt time.Time, loc *time.Location) ([]byte, error) {
	events, err := s.repo.ListByContextAndDateRange(ctx, contextType, contextID, startAt, endAt)
	if err != nil {
		return nil, err
	}
	items := make([]CalendarItem, 0, len(events))
	for i := range events {
		e := events[i]
		items = append(items, CalendarItem{
			ID:              fmt.Sprintf("event_%d", e.ID),
			Type:            CalendarItemEvent,
			Title:           e.Title,
			Description:     e.Description,
			StartAt:         e.StartAt,
			EndAt:           e.EndAt,
			AllDay:          e.AllDay,
			LocationName:    e.LocationName,
			LocationAddress: e.LocationAddress,
			CreatedAt:       e.CreatedAt,
		})
	}
	return RenderICSIn(items, "Paper LMS", "", time.Now(), loc), nil
}

// RenderICS renders calendar items as an RFC 5545 VCALENDAR. baseURL,
// when set, turns each item's HTMLURL into an absolute URL property.
func RenderICS(items []CalendarItem, calName, baseURL string, now time.Time) []byte {
	return RenderICSIn(items, calName, baseURL, now, time.UTC)
}

// RenderICSIn is RenderICS with all-day events dated in loc (the
// calendar owner's zone). An all-day event is stored as the instant of
// its local midnight, so reading its date in UTC puts it on the previous
// day for anyone east of UTC.
func RenderICSIn(items []CalendarItem, calName, baseURL string, now time.Time, loc *time.Location) []byte {
	if loc == nil {
		loc = time.UTC
	}
	sorted := make([]CalendarItem, len(items))
	copy(sorted, items)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].StartAt.Before(sorted[j].StartAt) })

	var buf bytes.Buffer
	line := func(s string) { writeICSLine(&buf, s) }
	line("BEGIN:VCALENDAR")
	line("VERSION:2.0")
	line("PRODID:-//Paper LMS//Calendar//EN")
	line("CALSCALE:GREGORIAN")
	line("METHOD:PUBLISH")
	if calName != "" {
		line("X-WR-CALNAME:" + escapeICalText(calName))
	}
	stamp := now.UTC().Format("20060102T150405Z")
	for _, it := range sorted {
		line("BEGIN:VEVENT")
		line(fmt.Sprintf("UID:%s@paper-lms", it.ID))
		line("DTSTAMP:" + stamp)
		title := it.Title
		if it.ContextName != "" && it.Type != CalendarItemEvent {
			title = fmt.Sprintf("%s [%s]", it.Title, it.ContextName)
		}
		line("SUMMARY:" + escapeICalText(title))
		if it.Description != "" {
			// Descriptions are rich content; the feed carries plain text.
			line("DESCRIPTION:" + escapeICalText(RichTextToPlain(it.Description)))
		}
		if it.AllDay {
			startDay, endDay := allDayRange(it.StartAt, it.EndAt, loc)
			line("DTSTART;VALUE=DATE:" + startDay.Format("20060102"))
			line("DTEND;VALUE=DATE:" + endDay.Format("20060102"))
		} else {
			line("DTSTART:" + it.StartAt.UTC().Format("20060102T150405Z"))
			if it.EndAt != nil {
				line("DTEND:" + it.EndAt.UTC().Format("20060102T150405Z"))
			}
		}
		location := it.LocationName
		if it.LocationAddress != "" {
			if location != "" {
				location += ", "
			}
			location += it.LocationAddress
		}
		if location != "" {
			line("LOCATION:" + escapeICalText(location))
		}
		if baseURL != "" && it.HTMLURL != "" {
			line("URL:" + strings.TrimRight(baseURL, "/") + it.HTMLURL)
		}
		line("END:VEVENT")
	}
	line("END:VCALENDAR")
	return buf.Bytes()
}

// allDayRange returns an all-day item's first day and its exclusive end
// day (RFC 5545: a DATE event's DTEND is the day after its last day),
// read in loc. An end at exactly local midnight is itself exclusive; any
// other end time still covers its own day. The range is always at least
// one day long.
func allDayRange(start time.Time, end *time.Time, loc *time.Location) (time.Time, time.Time) {
	day := func(t time.Time) time.Time {
		l := t.In(loc)
		return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, time.UTC)
	}
	startDay := day(start)
	endDay := startDay.AddDate(0, 0, 1)
	if end != nil && end.After(start) {
		last := day(*end)
		if e := end.In(loc); e.Hour() == 0 && e.Minute() == 0 && e.Second() == 0 && e.Nanosecond() == 0 {
			last = last.AddDate(0, 0, -1)
		}
		if cand := last.AddDate(0, 0, 1); cand.After(endDay) {
			endDay = cand
		}
	}
	return startDay, endDay
}

// writeICSLine writes one content line folded at 75 octets (RFC 5545
// 3.1) without splitting a UTF-8 sequence.
func writeICSLine(buf *bytes.Buffer, s string) {
	const limit = 75
	first := true
	for len(s) > 0 {
		max := limit
		if !first {
			max = limit - 1 // the leading space counts
		}
		if len(s) <= max {
			if !first {
				buf.WriteByte(' ')
			}
			buf.WriteString(s)
			break
		}
		cut := max
		for cut > 0 && (s[cut]&0xC0) == 0x80 {
			cut--
		}
		if !first {
			buf.WriteByte(' ')
		}
		buf.WriteString(s[:cut])
		buf.WriteString("\r\n")
		s = s[cut:]
		first = false
	}
	buf.WriteString("\r\n")
}

func escapeICalText(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, ";", "\\;")
	s = strings.ReplaceAll(s, "\r\n", "\\n")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}
