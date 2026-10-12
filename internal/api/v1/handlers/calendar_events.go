package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/gofiber/fiber/v2"
)

type CalendarEventHandler struct {
	calendarService *service.CalendarService
	authz           *ResourceAuthorizer
	// agg backs the aggregated /calendar_events query, section and group
	// lookups, and the user's time zone. Optional: without it the handler
	// keeps the single-context behavior.
	agg *service.CalendarAggregateService
}

func NewCalendarEventHandler(calendarService *service.CalendarService, authz *ResourceAuthorizer) *CalendarEventHandler {
	return &CalendarEventHandler{calendarService: calendarService, authz: authz}
}

// SetAggregator wires the aggregated calendar (G094) into the event
// handler.
func (h *CalendarEventHandler) SetAggregator(agg *service.CalendarAggregateService) {
	h.agg = agg
}

func calendarEventToJSON(e *models.CalendarEvent) fiber.Map {
	return fiber.Map{
		"id":                       e.ID,
		"context_type":             e.ContextType,
		"context_id":               e.ContextID,
		"context_code":             calendarContextCode(e.ContextType, e.ContextID),
		"title":                    e.Title,
		"description":              e.Description,
		"start_at":                 e.StartAt,
		"end_at":                   e.EndAt,
		"location_name":            e.LocationName,
		"location_address":         e.LocationAddress,
		"all_day":                  e.AllDay,
		"created_by_user_id":       e.CreatedByUserID,
		"workflow_state":           e.WorkflowState,
		"parent_calendar_event_id": e.ParentCalendarEventID,
		"series_uuid":              e.SeriesUUID,
		"rrule":                    e.RRule,
		"series_head":              e.SeriesHead,
		"created_at":               e.CreatedAt,
		"updated_at":               e.UpdatedAt,
	}
}

func calendarContextCode(contextType string, id uint) string {
	switch contextType {
	case models.CalendarContextCourseSection:
		return fmt.Sprintf("course_section_%d", id)
	default:
		return fmt.Sprintf("%s_%d", strings.ToLower(contextType), id)
	}
}

func (h *CalendarEventHandler) ListEvents(c *fiber.Ctx) error {
	var contextType string
	var contextID uint

	courseID, err := c.ParamsInt("course_id")
	if err == nil && courseID > 0 {
		contextType = "Course"
		contextID = uint(courseID)
	} else {
		userID, _ := c.Locals("user_id").(uint)
		if userID == 0 {
			return responses.Unauthorized(c)
		}
		// Canvas-style aggregated query: context_codes[] / type /
		// start_date ask for every calendar the user can see.
		if h.agg != nil && wantsAggregatedCalendar(c) {
			return h.listAggregated(c, userID)
		}
		contextType = "User"
		contextID = userID
	}

	params := middleware.GetPagination(c)

	result, err := h.calendarService.ListByContext(c.Context(), contextType, contextID, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch calendar events")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	events := make([]fiber.Map, len(result.Items))
	for i, e := range result.Items {
		events[i] = calendarEventToJSON(&e)
	}

	return c.JSON(events)
}

func wantsAggregatedCalendar(c *fiber.Ctx) bool {
	args := c.Context().QueryArgs()
	for _, k := range []string{"context_codes[]", "context_codes", "type", "start_date", "end_date"} {
		if args.Has(k) {
			return true
		}
	}
	return false
}

// listAggregated answers GET /calendar_events in its Canvas shape:
// type=event (default; events and the caller's appointments) or
// type=assignment (assignment and quiz due dates).
func (h *CalendarEventHandler) listAggregated(c *fiber.Ctx, userID uint) error {
	q, err := parseCalendarQuery(c, userID, callerAccountID(c), callerIsAdmin(c, h.authz), h.eventLocation(c, ""))
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	if len(q.Types) == 0 {
		switch c.Query("type", "event") {
		case "assignment":
			q.Types = map[string]bool{service.CalendarItemAssignment: true}
		default:
			q.Types = map[string]bool{service.CalendarItemEvent: true}
		}
	}
	items, err := h.agg.ListItems(c.Context(), q)
	if err != nil {
		return calendarListError(c, err)
	}
	return c.JSON(items)
}

// calendarListError answers an aggregated-calendar failure: a bad date
// range is a 400 with its message; anything else is a 500 that does not
// echo database text.
func calendarListError(c *fiber.Ctx, err error) error {
	if errors.Is(err, service.ErrCalendarValidation) {
		return responses.BadRequest(c, err.Error())
	}
	return responses.InternalError(c, "Could not load the calendar")
}

// parseCalendarQuery reads start_date/end_date (YYYY-MM-DD or RFC 3339;
// a bare end date is inclusive), context_codes[] and types[]. A bare
// date, and the default "today", are days in loc — the caller's profile
// time zone (G157) — not UTC days.
func parseCalendarQuery(c *fiber.Ctx, userID, accountID uint, isAdmin bool, loc *time.Location) (service.CalendarQuery, error) {
	if loc == nil {
		loc = time.UTC
	}
	q := service.CalendarQuery{UserID: userID, AccountID: accountID, IsAdmin: isAdmin}
	now := time.Now().In(loc)
	q.Start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	q.End = endOfCalendarDay(q.Start)
	if v := c.Query("start_date"); v != "" {
		t, dateOnly, err := parseCalendarDate(v, loc)
		if err != nil {
			return q, errors.New("start_date must be YYYY-MM-DD or an ISO 8601 time")
		}
		q.Start = t
		if c.Query("end_date") == "" {
			if dateOnly {
				q.End = endOfCalendarDay(t)
			} else {
				q.End = t.Add(24*time.Hour - time.Second)
			}
		}
	}
	if v := c.Query("end_date"); v != "" {
		t, dateOnly, err := parseCalendarDate(v, loc)
		if err != nil {
			return q, errors.New("end_date must be YYYY-MM-DD or an ISO 8601 time")
		}
		if dateOnly {
			t = endOfCalendarDay(t)
		}
		q.End = t
	}
	q.ContextCodes = queryList(c, "context_codes")
	if types := queryList(c, "types"); len(types) > 0 {
		q.Types = map[string]bool{}
		for _, t := range types {
			q.Types[t] = true
		}
	}
	return q, nil
}

// parseCalendarDate reads YYYY-MM-DD as midnight in loc, or an RFC 3339
// instant; the bool reports a bare date.
func parseCalendarDate(v string, loc *time.Location) (time.Time, bool, error) {
	if loc == nil {
		loc = time.UTC
	}
	if t, err := time.ParseInLocation("2006-01-02", v, loc); err == nil {
		return t, true, nil
	}
	t, err := time.Parse(time.RFC3339, v)
	return t, false, err
}

// endOfCalendarDay is the last second of the day that starts at
// dayStart (a local midnight), in dayStart's zone. AddDate, not 24h, so a
// DST change that day doesn't shift it.
func endOfCalendarDay(dayStart time.Time) time.Time {
	return dayStart.AddDate(0, 0, 1).Add(-time.Second)
}

// sectionCourse returns the course a section event belongs to (0 when
// the section is not in the caller's tenant).
func (h *CalendarEventHandler) sectionCourse(c *fiber.Ctx, sectionID uint) uint {
	if h.agg == nil {
		return 0
	}
	return h.agg.SectionCourseID(c.Context(), sectionID, callerAccountID(c))
}

func (h *CalendarEventHandler) isGroupMember(c *fiber.Ctx, groupID uint) bool {
	if h.agg == nil {
		return false
	}
	userID, _ := c.Locals("user_id").(uint)
	return h.agg.IsGroupMember(c.Context(), groupID, userID, callerAccountID(c))
}

// authorizeEventRead enforces who may read a single calendar event (the
// repo lookup has already applied the tenant filter). It returns
// wrote=true after writing the refusal; the caller MUST return err.
//
//   - User context: a personal calendar — the owner (or an admin) only.
//     Anyone else gets the same 404 as a missing event.
//   - Course context: course members (enrolled, or admin).
//   - CourseSection context: members of the section's course.
//   - Account context: everyone in the tenant.
//   - Group: members of the group, the creator, or an admin.
func (h *CalendarEventHandler) authorizeEventRead(c *fiber.Ctx, event *models.CalendarEvent) (bool, error) {
	if !h.eventReadable(c, event) {
		// Not visible to the caller: the same 404 as a missing event, so
		// event ids can't be probed (course events included).
		return true, responses.NotFound(c, "calendar event")
	}
	return false, nil
}

// eventReadable reports whether the caller may see the event. It never
// writes a response.
func (h *CalendarEventHandler) eventReadable(c *fiber.Ctx, event *models.CalendarEvent) bool {
	userID, _ := c.Locals("user_id").(uint)
	switch event.ContextType {
	case "User":
		return event.ContextID == userID || event.CreatedByUserID == userID || callerIsAdmin(c, h.authz)
	case "Course":
		return h.authz.RequireCourseEnrolled(c, event.ContextID) == nil
	case models.CalendarContextCourseSection:
		courseID := h.sectionCourse(c, event.ContextID)
		return courseID != 0 && h.authz.RequireCourseEnrolled(c, courseID) == nil
	case "Account":
		return true
	case models.CalendarContextGroup:
		return event.CreatedByUserID == userID || h.isGroupMember(c, event.ContextID) || callerIsAdmin(c, h.authz)
	default:
		return event.CreatedByUserID == userID || callerIsAdmin(c, h.authz)
	}
}

func (h *CalendarEventHandler) GetEvent(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid calendar event ID")
	}

	event, err := h.calendarService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "calendar event")
	}
	if wrote, err := h.authorizeEventRead(c, event); wrote {
		return err
	}

	out := calendarEventToJSON(event)
	if event.ContextType == models.CalendarContextCourse {
		if children, err := h.calendarService.ListChildren(c.Context(), event.ID); err == nil {
			out["child_events"] = eventsJSON(children)
			out["child_events_count"] = len(children)
		}
	}
	return c.JSON(out)
}

func eventsJSON(events []models.CalendarEvent) []fiber.Map {
	out := make([]fiber.Map, len(events))
	for i := range events {
		out[i] = calendarEventToJSON(&events[i])
	}
	return out
}

// authorizeEventCreate enforces where the caller may create an event,
// mirroring Canvas's manage_calendar rules. It returns wrote=true after
// writing the refusal; the caller MUST return err.
//
//   - User: only the caller's own calendar.
//   - Course: an instructor (teacher/TA) of the course, or an admin.
//   - Group: a member of the group (or an admin).
//   - Account: an admin of that account (the caller's tenant).
//   - Anything else is refused.
func (h *CalendarEventHandler) authorizeEventCreate(c *fiber.Ctx, event *models.CalendarEvent) (bool, error) {
	userID, _ := c.Locals("user_id").(uint)
	switch event.ContextType {
	case "User":
		if event.ContextID != userID {
			return true, responses.Forbidden(c, "you can only add events to your own calendar")
		}
		return false, nil
	case "Course":
		if event.ContextID == 0 {
			return true, responses.BadRequest(c, "context_id is required")
		}
		if err := h.authz.RequireCourseInstructor(c, event.ContextID); authzDenied(c, err) {
			return true, err
		}
		return false, nil
	case models.CalendarContextGroup:
		// Group members manage the group calendar. Anyone else (an admin
		// included) gets the same 404 as a group that doesn't exist.
		if h.isGroupMember(c, event.ContextID) {
			return false, nil
		}
		return true, responses.NotFound(c, "group")
	case "Account":
		if event.ContextID != callerAccountID(c) {
			return true, responses.NotFound(c, "account")
		}
		if !callerIsAdmin(c, h.authz) {
			return true, responses.Forbidden(c, "admin access required")
		}
		return false, nil
	default:
		return true, responses.BadRequest(c, "context_type must be one of: User, Course, Group, Account")
	}
}

// authorizeEventManage enforces who may edit or delete an event: the
// creator, an admin, an instructor of the event's course, or (for group
// events) a group member. It returns wrote=true after writing the
// refusal; the caller MUST return err.
func (h *CalendarEventHandler) authorizeEventManage(c *fiber.Ctx, event *models.CalendarEvent) (bool, error) {
	userID, _ := c.Locals("user_id").(uint)
	// An event the caller can't even see is a 404, as on read; only a
	// visible event the caller may not change is a 403.
	if !h.eventReadable(c, event) {
		return true, responses.NotFound(c, "calendar event")
	}
	if event.ContextType == models.CalendarContextCourseSection {
		return true, responses.BadRequest(c, "edit the course event this section time belongs to")
	}
	switch event.ContextType {
	case models.CalendarContextCourse:
		if callerIsCourseInstructor(c, h.authz, event.ContextID) {
			return false, nil
		}
	case models.CalendarContextGroup:
		if h.isGroupMember(c, event.ContextID) {
			return false, nil
		}
	case models.CalendarContextAccount:
		if callerIsAdmin(c, h.authz) {
			return false, nil
		}
		if event.CreatedByUserID != userID {
			return true, responses.Forbidden(c, "admin access required")
		}
	}
	if err := h.authz.RequireOwnerOrAdmin(c, event.CreatedByUserID); authzDenied(c, err) {
		return true, err
	}
	return false, nil
}

// childEventInput is one entry of child_event_data: a section's times.
type childEventInput struct {
	StartAt     *time.Time `json:"start_at"`
	EndAt       *time.Time `json:"end_at"`
	ContextCode string     `json:"context_code"`
}

// parseChildEventData accepts child_event_data as an array or as the
// Canvas object form ({"0": {...}, "1": {...}}).
func parseChildEventData(raw json.RawMessage) ([]childEventInput, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var list []childEventInput
	if err := json.Unmarshal(raw, &list); err == nil {
		return list, nil
	}
	var m map[string]childEventInput
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, errors.New("child_event_data must be a list of {start_at, end_at, context_code}")
	}
	keys := make([]int, 0, len(m))
	byKey := map[int]childEventInput{}
	for k, v := range m {
		n, err := strconv.Atoi(k)
		if err != nil {
			return nil, errors.New("child_event_data keys must be numbers")
		}
		keys = append(keys, n)
		byKey[n] = v
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	for _, k := range keys {
		list = append(list, byKey[k])
	}
	return list, nil
}

// resolveChildren turns child_event_data into section inputs, checking
// every section belongs to courseID in the caller's tenant. It returns
// wrote=true after writing the refusal.
func (h *CalendarEventHandler) resolveChildren(c *fiber.Ctx, courseID uint, data []childEventInput) ([]service.ChildEventInput, bool, error) {
	out := make([]service.ChildEventInput, 0, len(data))
	for _, d := range data {
		code := strings.TrimSpace(d.ContextCode)
		if !strings.HasPrefix(code, "course_section_") {
			return nil, true, responses.BadRequest(c, "child_event_data context_code must be course_section_<id>")
		}
		sid, err := strconv.ParseUint(strings.TrimPrefix(code, "course_section_"), 10, 64)
		if err != nil || sid == 0 {
			return nil, true, responses.BadRequest(c, "child_event_data context_code must be course_section_<id>")
		}
		if h.sectionCourse(c, uint(sid)) != courseID {
			return nil, true, responses.NotFound(c, "course section")
		}
		if d.StartAt == nil {
			return nil, true, responses.BadRequest(c, "each section time needs a start_at")
		}
		out = append(out, service.ChildEventInput{SectionID: uint(sid), StartAt: *d.StartAt, EndAt: d.EndAt})
	}
	return out, false, nil
}

// eventLocation picks the zone a recurrence expands in: the request's
// time_zone, else the user's profile zone.
func (h *CalendarEventHandler) eventLocation(c *fiber.Ctx, tz string) *time.Location {
	if tz != "" {
		return service.LoadCalendarLocation(tz)
	}
	if h.agg != nil {
		userID, _ := c.Locals("user_id").(uint)
		return h.agg.UserTimeZone(c.Context(), userID)
	}
	return time.UTC
}

// calendarServiceError maps a create/update failure: validation errors
// are 400s with their message; anything else is a 500 that does not
// echo database text.
func calendarServiceError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidRRule),
		errors.Is(err, service.ErrCalendarEndBeforeStart),
		errors.Is(err, service.ErrCalendarSeriesAndSection),
		errors.Is(err, service.ErrCalendarSectionsCourse),
		errors.Is(err, service.ErrCalendarBadScope),
		errors.Is(err, service.ErrCalendarValidation):
		return responses.BadRequest(c, err.Error())
	}
	return responses.InternalError(c, "Could not save calendar event")
}

func (h *CalendarEventHandler) CreateEvent(c *fiber.Ctx) error {
	var input struct {
		CalendarEvent struct {
			ContextType     string          `json:"context_type"`
			ContextID       uint            `json:"context_id"`
			ContextCode     string          `json:"context_code"`
			Title           string          `json:"title"`
			Description     string          `json:"description"`
			StartAt         time.Time       `json:"start_at"`
			EndAt           *time.Time      `json:"end_at"`
			LocationName    string          `json:"location_name"`
			LocationAddress string          `json:"location_address"`
			AllDay          bool            `json:"all_day"`
			RRule           string          `json:"rrule"`
			TimeZone        string          `json:"time_zone"`
			ChildEventData  json.RawMessage `json:"child_event_data"`
		} `json:"calendar_event"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	userID, _ := c.Locals("user_id").(uint)
	in := input.CalendarEvent

	// No context means the caller's personal calendar. context_code
	// ("course_5") is accepted as in Canvas.
	contextType := in.ContextType
	contextID := in.ContextID
	if contextType == "" && in.ContextCode != "" {
		contextType, contextID = parseContextCode(in.ContextCode)
		if contextType == "" {
			return responses.BadRequest(c, "context_code must be user_<id>, course_<id>, group_<id> or account_<id>")
		}
	}
	if contextType == "" {
		contextType = "User"
		if contextID == 0 {
			contextID = userID
		}
	}

	event := &models.CalendarEvent{
		ContextType:     contextType,
		ContextID:       contextID,
		Title:           in.Title,
		Description:     service.SanitizeHTML(in.Description), // rich content (RCE), G010
		StartAt:         in.StartAt,
		EndAt:           in.EndAt,
		LocationName:    in.LocationName,
		LocationAddress: in.LocationAddress,
		AllDay:          in.AllDay,
		CreatedByUserID: userID,
	}

	if wrote, err := h.authorizeEventCreate(c, event); wrote {
		return err
	}

	childData, err := parseChildEventData(in.ChildEventData)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	var children []service.ChildEventInput
	if len(childData) > 0 {
		if contextType != models.CalendarContextCourse {
			return responses.BadRequest(c, service.ErrCalendarSectionsCourse.Error())
		}
		var wrote bool
		children, wrote, err = h.resolveChildren(c, contextID, childData)
		if wrote {
			return err
		}
		if event.StartAt.IsZero() {
			event.StartAt = children[0].StartAt
		}
	}

	created, err := h.calendarService.CreateWithOptions(c.Context(), event, service.CreateOptions{
		RRule:    in.RRule,
		Location: h.eventLocation(c, in.TimeZone),
		Children: children,
	})
	if err != nil {
		return calendarServiceError(c, err)
	}

	out := calendarEventToJSON(event)
	if len(children) > 0 {
		out["child_events"] = eventsJSON(created[1:])
		out["child_events_count"] = len(created) - 1
	}
	if event.SeriesUUID != nil {
		out["duplicates"] = eventsJSON(created[1:])
		out["series_count"] = len(created)
	}
	return c.Status(fiber.StatusCreated).JSON(out)
}

// parseContextCode splits "course_5" into ("Course", 5).
func parseContextCode(code string) (string, uint) {
	idx := strings.LastIndex(code, "_")
	if idx <= 0 {
		return "", 0
	}
	id, err := strconv.ParseUint(code[idx+1:], 10, 64)
	if err != nil || id == 0 {
		return "", 0
	}
	switch code[:idx] {
	case "user":
		return models.CalendarContextUser, uint(id)
	case "course":
		return models.CalendarContextCourse, uint(id)
	case "group":
		return models.CalendarContextGroup, uint(id)
	case "account":
		return models.CalendarContextAccount, uint(id)
	}
	return "", 0
}

func (h *CalendarEventHandler) UpdateEvent(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid calendar event ID")
	}

	event, err := h.calendarService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil || event.WorkflowState == "deleted" {
		return responses.NotFound(c, "calendar event")
	}

	if wrote, err := h.authorizeEventManage(c, event); wrote {
		return err
	}

	var input struct {
		CalendarEvent struct {
			Title             *string         `json:"title"`
			Description       *string         `json:"description"`
			StartAt           *time.Time      `json:"start_at"`
			EndAt             *time.Time      `json:"end_at"`
			LocationName      *string         `json:"location_name"`
			LocationAddress   *string         `json:"location_address"`
			AllDay            *bool           `json:"all_day"`
			RRule             *string         `json:"rrule"`
			TimeZone          string          `json:"time_zone"`
			ChildEventData    json.RawMessage `json:"child_event_data"`
			RemoveChildEvents bool            `json:"remove_child_events"`
		} `json:"calendar_event"`
		Which string `json:"which"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	// end_at: null clears it; an absent key leaves it alone.
	var presence struct {
		CalendarEvent map[string]json.RawMessage `json:"calendar_event"`
	}
	_ = json.Unmarshal(c.Body(), &presence)
	_, endAtSet := presence.CalendarEvent["end_at"]

	in := input.CalendarEvent
	ch := service.EventChanges{
		StartAt:        in.StartAt,
		EndAt:          in.EndAt,
		EndAtSet:       endAtSet,
		AllDay:         in.AllDay,
		RRule:          in.RRule,
		Location:       h.eventLocation(c, in.TimeZone),
		RemoveChildren: in.RemoveChildEvents,
	}
	ch.Title = in.Title
	ch.LocationName = in.LocationName
	ch.LocationAddress = in.LocationAddress
	if in.Description != nil {
		v := service.SanitizeHTML(*in.Description)
		ch.Description = &v
	}
	childData, err := parseChildEventData(in.ChildEventData)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	if len(childData) > 0 {
		if event.ContextType != models.CalendarContextCourse {
			return responses.BadRequest(c, service.ErrCalendarSectionsCourse.Error())
		}
		children, wrote, err := h.resolveChildren(c, event.ContextID, childData)
		if wrote {
			return err
		}
		ch.Children = &children
	}

	which := input.Which
	if which == "" {
		which = c.Query("which")
	}
	rows, err := h.calendarService.UpdateWithScope(c.Context(), event, ch, which)
	if err != nil {
		return calendarServiceError(c, err)
	}

	out := calendarEventToJSON(event)
	if len(rows) > 1 {
		if event.SeriesUUID != nil {
			out["duplicates"] = eventsJSON(rows[1:])
		} else {
			out["child_events"] = eventsJSON(rows[1:])
		}
	}
	return c.JSON(out)
}

func (h *CalendarEventHandler) DeleteEvent(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid calendar event ID")
	}

	event, err := h.calendarService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil || event.WorkflowState == "deleted" {
		return responses.NotFound(c, "calendar event")
	}
	if wrote, err := h.authorizeEventManage(c, event); wrote {
		return err
	}

	ids, err := h.calendarService.DeleteWithScope(c.Context(), event, c.Query("which"))
	if err != nil {
		if errors.Is(err, service.ErrCalendarBadScope) {
			return responses.BadRequest(c, err.Error())
		}
		return responses.InternalError(c, "Could not delete calendar event")
	}

	return c.JSON(fiber.Map{"delete": true, "deleted_ids": ids})
}

// ExportAsICal downloads the caller's calendar as an .ics file: every
// calendar they can see (when the aggregator is wired), else their
// personal events.
func (h *CalendarEventHandler) ExportAsICal(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(uint)
	if userID == 0 {
		return responses.Unauthorized(c)
	}

	now := time.Now()
	startDate := now.AddDate(0, 0, -30)
	endDate := now.AddDate(0, 0, 365)
	// Bare dates and all-day events are read in the caller's profile
	// zone (G157).
	loc := h.eventLocation(c, "")

	if sd := c.Query("start_date"); sd != "" {
		parsed, err := time.ParseInLocation("2006-01-02", sd, loc)
		if err == nil {
			startDate = parsed
		}
	}
	if ed := c.Query("end_date"); ed != "" {
		parsed, err := time.ParseInLocation("2006-01-02", ed, loc)
		if err == nil {
			endDate = parsed
		}
	}

	var data []byte
	if h.agg != nil {
		items, err := h.agg.ListItems(c.Context(), service.CalendarQuery{
			UserID:    userID,
			AccountID: callerAccountID(c),
			IsAdmin:   callerIsAdmin(c, h.authz),
			Start:     startDate,
			End:       endDate,
		})
		if err != nil {
			return calendarListError(c, err)
		}
		data = service.RenderICSIn(items, "Paper LMS calendar", c.BaseURL(), now, loc)
	} else {
		var err error
		data, err = h.calendarService.ExportAsICalendar(c.Context(), "User", userID, startDate, endDate, loc)
		if err != nil {
			return responses.InternalError(c, "Could not export calendar")
		}
	}

	c.Set("Content-Type", "text/calendar; charset=utf-8")
	c.Set("Content-Disposition", "attachment; filename=\"calendar.ics\"")
	return c.Send(data)
}
