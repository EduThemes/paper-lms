package handlers

// Learner-experience endpoints (learner-dashboard lane): the planner /
// to-do list, dashboard course cards with favorites / nicknames / colors,
// the recent activity stream, missing submissions, the student grade
// summary, observer alerts and the user profile. Routes are mounted in
// internal/api/v1/routes_learner.go.

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

type LearnerHandler struct {
	planner   *service.PlannerService
	dashboard *service.LearnerDashboardService
	grades    *service.GradeSummaryService
	alerts    *service.ObserverAlertService
	profiles  *service.UserProfileService
	observers *service.ObserverService
	users     repository.UserRepository
}

func NewLearnerHandler(
	planner *service.PlannerService,
	dashboard *service.LearnerDashboardService,
	grades *service.GradeSummaryService,
	alerts *service.ObserverAlertService,
	profiles *service.UserProfileService,
	observers *service.ObserverService,
	users repository.UserRepository,
) *LearnerHandler {
	return &LearnerHandler{
		planner: planner, dashboard: dashboard, grades: grades, alerts: alerts,
		profiles: profiles, observers: observers, users: users,
	}
}

// parsePlannerTime accepts an RFC 3339 timestamp or a YYYY-MM-DD date. A
// bare end date is inclusive (end of that day, UTC).
func parsePlannerTime(v string, endOfDay bool) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, nil
	}
	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		return time.Time{}, err
	}
	if endOfDay {
		t = t.Add(24*time.Hour - time.Nanosecond)
	}
	return t, nil
}

// --- Planner ---

// PlannerItems handles GET /planner/items?start_date=&end_date=&filter=
func (h *LearnerHandler) PlannerItems(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	now := time.Now()
	q := service.PlannerQuery{Start: now.AddDate(0, 0, -14), End: now.AddDate(0, 0, 14)}
	if sd := c.Query("start_date"); sd != "" {
		t, err := parsePlannerTime(sd, false)
		if err != nil {
			return responses.BadRequest(c, "Invalid start_date (use YYYY-MM-DD or RFC 3339)")
		}
		q.Start = t
	}
	if ed := c.Query("end_date"); ed != "" {
		t, err := parsePlannerTime(ed, true)
		if err != nil {
			return responses.BadRequest(c, "Invalid end_date (use YYYY-MM-DD or RFC 3339)")
		}
		q.End = t
	}
	if q.End.Before(q.Start) {
		return responses.BadRequest(c, "end_date must not be before start_date")
	}
	if q.End.Sub(q.Start) > 400*24*time.Hour {
		return responses.BadRequest(c, "The date range may span at most 400 days")
	}
	q.IncompleteOnly = c.Query("filter") == "incomplete_items"
	items, err := h.planner.Items(c.Context(), userID, callerAccountID(c), q)
	if err != nil {
		return responses.InternalError(c, "Could not fetch planner items")
	}
	return c.JSON(items)
}

type plannerNoteBody struct {
	Title    *string    `json:"title"`
	Details  *string    `json:"details"`
	TodoDate *time.Time `json:"todo_date"`
	CourseID *uint      `json:"course_id"`
}

func plannerNoteJSON(note *models.PlannerNote) fiber.Map {
	return fiber.Map{
		"id":             note.ID,
		"user_id":        note.UserID,
		"title":          note.Title,
		"details":        note.Details,
		"todo_date":      note.TodoDate,
		"course_id":      note.CourseID,
		"workflow_state": note.WorkflowState,
		"created_at":     note.CreatedAt,
		"updated_at":     note.UpdatedAt,
	}
}

func plannerOverrideJSON(o *models.PlannerOverride) fiber.Map {
	return fiber.Map{
		"id":              o.ID,
		"user_id":         o.UserID,
		"plannable_type":  o.PlannableType,
		"plannable_id":    o.PlannableID,
		"marked_complete": o.MarkedComplete,
		"dismissed":       o.Dismissed,
		"created_at":      o.CreatedAt,
		"updated_at":      o.UpdatedAt,
	}
}

// ListPlannerNotes handles GET /planner_notes?start_date=&end_date=
func (h *LearnerHandler) ListPlannerNotes(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	var start, end *time.Time
	if sd := c.Query("start_date"); sd != "" {
		t, err := parsePlannerTime(sd, false)
		if err != nil {
			return responses.BadRequest(c, "Invalid start_date")
		}
		start = &t
	}
	if ed := c.Query("end_date"); ed != "" {
		t, err := parsePlannerTime(ed, true)
		if err != nil {
			return responses.BadRequest(c, "Invalid end_date")
		}
		end = &t
	}
	notes, err := h.planner.ListNotes(c.Context(), userID, start, end)
	if err != nil {
		return responses.InternalError(c, "Could not fetch planner notes")
	}
	out := make([]fiber.Map, len(notes))
	for i := range notes {
		out[i] = plannerNoteJSON(&notes[i])
	}
	return c.JSON(out)
}

// CreatePlannerNote handles POST /planner_notes
func (h *LearnerHandler) CreatePlannerNote(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	var in plannerNoteBody
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	note, err := h.planner.CreateNoteFor(c.Context(), userID, callerAccountID(c), service.NoteInput{
		Title: in.Title, Details: in.Details, TodoDate: in.TodoDate, CourseID: in.CourseID,
	})
	if errors.Is(err, service.ErrPlannerNotFound) {
		return responses.NotFound(c, "course")
	}
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(plannerNoteJSON(note))
}

// UpdatePlannerNote handles PUT /planner_notes/:id
func (h *LearnerHandler) UpdatePlannerNote(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return responses.BadRequest(c, "Invalid planner note ID")
	}
	var raw map[string]any
	_ = c.BodyParser(&raw)
	var in plannerNoteBody
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	clear := false
	if v, ok := raw["course_id"]; ok && v == nil {
		clear = true
	}
	note, err := h.planner.UpdateNoteFor(c.Context(), userID, callerAccountID(c), uint(id), service.NoteInput{
		Title: in.Title, Details: in.Details, TodoDate: in.TodoDate, CourseID: in.CourseID, ClearCourse: clear,
	})
	if errors.Is(err, service.ErrPlannerNotFound) {
		return responses.NotFound(c, "planner note")
	}
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.JSON(plannerNoteJSON(note))
}

// DeletePlannerNote handles DELETE /planner_notes/:id
func (h *LearnerHandler) DeletePlannerNote(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return responses.BadRequest(c, "Invalid planner note ID")
	}
	if err := h.planner.DeleteNote(c.Context(), uint(id), userID); err != nil {
		if errors.Is(err, service.ErrPlannerNotFound) {
			return responses.NotFound(c, "planner note")
		}
		return responses.InternalError(c, "Could not delete planner note")
	}
	return c.JSON(fiber.Map{"deleted": true})
}

// ListPlannerOverrides handles GET /planner/overrides
func (h *LearnerHandler) ListPlannerOverrides(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	list, err := h.planner.ListOverrides(c.Context(), userID)
	if err != nil {
		return responses.InternalError(c, "Could not fetch planner overrides")
	}
	out := make([]fiber.Map, len(list))
	for i := range list {
		out[i] = plannerOverrideJSON(&list[i])
	}
	return c.JSON(out)
}

// CreatePlannerOverride handles POST /planner/overrides (upsert by
// plannable). Flags left out of the body keep their stored value.
func (h *LearnerHandler) CreatePlannerOverride(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	var in struct {
		PlannableType  string `json:"plannable_type"`
		PlannableID    uint   `json:"plannable_id"`
		MarkedComplete *bool  `json:"marked_complete"`
		Dismissed      *bool  `json:"dismissed"`
	}
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	o, err := h.planner.UpsertOverride(c.Context(), userID, in.PlannableType, in.PlannableID, in.MarkedComplete, in.Dismissed)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(plannerOverrideJSON(o))
}

// UpdatePlannerOverride handles PUT /planner/overrides/:id
func (h *LearnerHandler) UpdatePlannerOverride(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return responses.BadRequest(c, "Invalid planner override ID")
	}
	var in struct {
		MarkedComplete *bool `json:"marked_complete"`
		Dismissed      *bool `json:"dismissed"`
	}
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	o, err := h.planner.UpdateOverride(c.Context(), uint(id), userID, in.MarkedComplete, in.Dismissed)
	if errors.Is(err, service.ErrPlannerNotFound) {
		return responses.NotFound(c, "planner override")
	}
	if err != nil {
		return responses.InternalError(c, "Could not update planner override")
	}
	return c.JSON(plannerOverrideJSON(o))
}

// DeletePlannerOverride handles DELETE /planner/overrides/:id
func (h *LearnerHandler) DeletePlannerOverride(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return responses.BadRequest(c, "Invalid planner override ID")
	}
	if err := h.planner.DeleteOverride(c.Context(), uint(id), userID); err != nil {
		if errors.Is(err, service.ErrPlannerNotFound) {
			return responses.NotFound(c, "planner override")
		}
		return responses.InternalError(c, "Could not delete planner override")
	}
	return c.JSON(fiber.Map{"deleted": true})
}

// --- Dashboard ---

// DashboardCards handles GET /dashboard/dashboard_cards?all=true
func (h *LearnerHandler) DashboardCards(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	cards, err := h.dashboard.Cards(c.Context(), userID, callerAccountID(c), c.QueryBool("all", false))
	if err != nil {
		return responses.InternalError(c, "Could not fetch dashboard cards")
	}
	return c.JSON(cards)
}

// SetDashboardPositions handles PUT /dashboard/positions {order: [course ids]}
func (h *LearnerHandler) SetDashboardPositions(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	var in struct {
		Order []uint `json:"order"`
	}
	if err := c.BodyParser(&in); err != nil || len(in.Order) > 500 {
		return responses.BadRequest(c, "order must be a list of course ids")
	}
	if err := h.dashboard.SetPositions(c.Context(), userID, callerAccountID(c), in.Order); err != nil {
		return responses.InternalError(c, "Could not save the course order")
	}
	return c.JSON(fiber.Map{"saved": true})
}

// courseIDParam reads :course_id; wrote=true after answering 400.
func courseIDParam(c *fiber.Ctx) (uint, bool) {
	id, err := c.ParamsInt("course_id")
	if err != nil || id <= 0 {
		_ = responses.BadRequest(c, "Invalid course ID")
		return 0, true
	}
	return uint(id), false
}

// prefError answers a dashboard-preference error; wrote is always true.
func prefError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, service.ErrDashboardCourseNotFound):
		return responses.NotFound(c, "course")
	case errors.Is(err, service.ErrInvalidColor):
		return responses.BadRequest(c, err.Error())
	}
	if strings.Contains(err.Error(), "characters") {
		return responses.BadRequest(c, err.Error())
	}
	return responses.InternalError(c, "Could not save the course preference")
}

// ListFavoriteCourses handles GET /users/self/favorites/courses
func (h *LearnerHandler) ListFavoriteCourses(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	cards, err := h.dashboard.Cards(c.Context(), userID, callerAccountID(c), true)
	if err != nil {
		return responses.InternalError(c, "Could not fetch favorites")
	}
	out := []fiber.Map{}
	for _, card := range cards {
		if card.IsFavorited {
			out = append(out, fiber.Map{
				"id": card.ID, "name": card.ShortName, "original_name": card.OriginalName,
				"course_code": card.CourseCode, "is_favorite": true,
			})
		}
	}
	return c.JSON(out)
}

// AddFavoriteCourse handles POST /users/self/favorites/courses/:course_id
func (h *LearnerHandler) AddFavoriteCourse(c *fiber.Ctx) error {
	return h.setFavorite(c, true)
}

// RemoveFavoriteCourse handles DELETE /users/self/favorites/courses/:course_id
func (h *LearnerHandler) RemoveFavoriteCourse(c *fiber.Ctx) error {
	return h.setFavorite(c, false)
}

func (h *LearnerHandler) setFavorite(c *fiber.Ctx, fav bool) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	courseID, wrote := courseIDParam(c)
	if wrote {
		return nil
	}
	if _, err := h.dashboard.SetFavorite(c.Context(), userID, callerAccountID(c), courseID, fav); err != nil {
		return prefError(c, err)
	}
	return c.JSON(fiber.Map{"context_type": "course", "context_id": courseID, "is_favorite": fav})
}

func nicknameJSON(courseID uint, name string, nick *string) fiber.Map {
	return fiber.Map{"course_id": courseID, "name": name, "nickname": nick}
}

// ListCourseNicknames handles GET /users/self/course_nicknames
func (h *LearnerHandler) ListCourseNicknames(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	prefs, courses, err := h.dashboard.Preferences(c.Context(), userID, callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not fetch nicknames")
	}
	out := []fiber.Map{}
	for _, p := range prefs {
		if p.Nickname != nil {
			out = append(out, nicknameJSON(p.CourseID, courses[p.CourseID].Name, p.Nickname))
		}
	}
	return c.JSON(out)
}

// SetCourseNickname handles PUT /users/self/course_nicknames/:course_id {nickname}
func (h *LearnerHandler) SetCourseNickname(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	courseID, wrote := courseIDParam(c)
	if wrote {
		return nil
	}
	var in struct {
		Nickname string `json:"nickname"`
	}
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	p, err := h.dashboard.SetNickname(c.Context(), userID, callerAccountID(c), courseID, in.Nickname)
	if err != nil {
		return prefError(c, err)
	}
	return c.JSON(nicknameJSON(courseID, "", p.Nickname))
}

// DeleteCourseNickname handles DELETE /users/self/course_nicknames/:course_id
func (h *LearnerHandler) DeleteCourseNickname(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	courseID, wrote := courseIDParam(c)
	if wrote {
		return nil
	}
	if _, err := h.dashboard.SetNickname(c.Context(), userID, callerAccountID(c), courseID, ""); err != nil {
		return prefError(c, err)
	}
	return c.JSON(nicknameJSON(courseID, "", nil))
}

// GetCustomColors handles GET /users/self/colors
func (h *LearnerHandler) GetCustomColors(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	prefs, _, err := h.dashboard.Preferences(c.Context(), userID, callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not fetch colors")
	}
	colors := fiber.Map{}
	for _, p := range prefs {
		if p.Color != nil {
			colors[fmt.Sprintf("course_%d", p.CourseID)] = *p.Color
		}
	}
	return c.JSON(fiber.Map{"custom_colors": colors})
}

// SetCustomColor handles PUT /users/self/colors/:asset_string {hexcode}.
// Only course_<id> asset strings are supported.
func (h *LearnerHandler) SetCustomColor(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	asset := c.Params("asset_string")
	if !strings.HasPrefix(asset, "course_") {
		return responses.BadRequest(c, "Only course colors are supported")
	}
	id, err := strconv.ParseUint(strings.TrimPrefix(asset, "course_"), 10, 64)
	if err != nil || id == 0 {
		return responses.BadRequest(c, "Invalid asset string")
	}
	var in struct {
		Hexcode string `json:"hexcode"`
	}
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	p, err := h.dashboard.SetColor(c.Context(), userID, callerAccountID(c), uint(id), in.Hexcode)
	if err != nil {
		return prefError(c, err)
	}
	return c.JSON(fiber.Map{"hexcode": p.Color})
}

// ActivityStream handles GET /users/self/activity_stream?days=&per_page=
func (h *LearnerHandler) ActivityStream(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	items, err := h.dashboard.ActivityStream(c.Context(), userID, callerAccountID(c), c.QueryInt("days", 28), c.QueryInt("per_page", 50))
	if err != nil {
		return responses.InternalError(c, "Could not fetch the activity stream")
	}
	return c.JSON(items)
}

// ActivityStreamSummary handles GET /users/self/activity_stream/summary
func (h *LearnerHandler) ActivityStreamSummary(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	items, err := h.dashboard.ActivityStream(c.Context(), userID, callerAccountID(c), 28, 100)
	if err != nil {
		return responses.InternalError(c, "Could not fetch the activity stream")
	}
	return c.JSON(service.ActivitySummary(items))
}

// resolveStudentParam reads a "self" or numeric user id for endpoints a
// student may call for themselves and an observer for a linked student.
// It returns the student id and, for an observer, the observed courses.
// wrote=true after it answered (400 / 404).
func (h *LearnerHandler) resolveStudentParam(c *fiber.Ctx, raw string, callerID uint) (studentID uint, courseFilter map[uint]bool, wrote bool) {
	if raw == "" || raw == "self" {
		return callerID, nil, false
	}
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		_ = responses.BadRequest(c, "Invalid user ID")
		return 0, nil, true
	}
	if uint(id) == callerID {
		return callerID, nil, false
	}
	if h.observers == nil {
		_ = responses.NotFound(c, "user")
		return 0, nil, true
	}
	courses, err := h.observers.GetObserveeCourses(c.Context(), callerID, uint(id))
	if err != nil {
		_ = responses.NotFound(c, "user")
		return 0, nil, true
	}
	acct := callerAccountID(c)
	filter := map[uint]bool{}
	for _, course := range courses {
		if course.AccountID == acct {
			filter[course.ID] = true
		}
	}
	return uint(id), filter, false
}

// MissingSubmissions handles GET /users/:user_id/missing_submissions
// (self, or a student the caller observes).
func (h *LearnerHandler) MissingSubmissions(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	studentID, filter, wrote := h.resolveStudentParam(c, c.Params("user_id"), userID)
	if wrote {
		return nil
	}
	var overrides []models.PlannerOverride
	if studentID == userID {
		overrides, _ = h.planner.ListOverrides(c.Context(), userID)
	}
	list, err := h.dashboard.MissingSubmissions(c.Context(), studentID, callerAccountID(c), filter, overrides)
	if err != nil {
		return responses.InternalError(c, "Could not fetch missing submissions")
	}
	if !c.QueryBool("include_dismissed", true) {
		kept := list[:0]
		for _, m := range list {
			if !m.Dismissed {
				kept = append(kept, m)
			}
		}
		list = kept
	}
	return c.JSON(list)
}

// --- Grade summary ---

// GradeSummary handles GET /courses/:course_id/grade_summary?student_id=&grading_period_id=
// A student reads their own; an observer a student they observe in this
// course; course staff any student.
func (h *LearnerHandler) GradeSummary(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	courseID, wrote := courseIDParam(c)
	if wrote {
		return nil
	}
	staff := callerIsCourseStaff(c)
	studentID := userID
	if raw := c.Query("student_id"); raw != "" && raw != "self" {
		id, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || id == 0 {
			return responses.BadRequest(c, "Invalid student_id")
		}
		studentID = uint(id)
	}
	if studentID != userID && !staff {
		allowed := false
		if h.observers != nil {
			if courses, err := h.observers.GetObserveeCourses(c.Context(), userID, studentID); err == nil {
				for _, course := range courses {
					if course.ID == courseID {
						allowed = true
					}
				}
			}
		}
		if !allowed {
			return responses.NotFound(c, "student")
		}
	}
	var periodID *uint
	if raw := c.Query("grading_period_id"); raw != "" {
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return responses.BadRequest(c, "Invalid grading_period_id")
		}
		v := uint(n)
		periodID = &v
	}
	sum, err := h.grades.Summary(c.Context(), courseID, studentID, callerAccountID(c), periodID, service.GradeSummaryViewer{IsStaff: staff})
	if errors.Is(err, service.ErrGradeSummaryNotFound) {
		return responses.NotFound(c, "student")
	}
	if err != nil {
		return responses.InternalError(c, "Could not build the grade summary")
	}
	return c.JSON(sum)
}

// --- Observer alerts ---

func alertError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, service.ErrObserverAlertNotFound):
		return responses.NotFound(c, "observer alert")
	case errors.Is(err, service.ErrInvalidAlertThreshold), errors.Is(err, service.ErrInvalidAlertTransition):
		return responses.BadRequest(c, err.Error())
	}
	return responses.InternalError(c, "Could not process the observer alert request")
}

func optionalUintQuery(c *fiber.Ctx, key string) (*uint, bool) {
	raw := c.Query(key)
	if raw == "" {
		return nil, false
	}
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || n == 0 {
		_ = responses.BadRequest(c, "Invalid "+key)
		return nil, true
	}
	v := uint(n)
	return &v, false
}

// ListAlertThresholds handles GET /observer_alert_thresholds?student_id=
func (h *LearnerHandler) ListAlertThresholds(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	studentID, wrote := optionalUintQuery(c, "student_id")
	if wrote {
		return nil
	}
	list, err := h.alerts.ListThresholds(c.Context(), userID, callerAccountID(c), studentID)
	if err != nil {
		return alertError(c, err)
	}
	return c.JSON(list)
}

type thresholdBody struct {
	Threshold struct {
		UserID    uint     `json:"user_id"`
		AlertType string   `json:"alert_type"`
		Threshold *float64 `json:"threshold"`
	} `json:"threshold"`
}

// CreateAlertThreshold handles POST /observer_alert_thresholds
func (h *LearnerHandler) CreateAlertThreshold(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	var in thresholdBody
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	t, err := h.alerts.CreateThreshold(c.Context(), userID, callerAccountID(c), service.ThresholdInput{
		StudentID: in.Threshold.UserID, AlertType: in.Threshold.AlertType, Threshold: in.Threshold.Threshold,
	})
	if err != nil {
		return alertError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(t)
}

// UpdateAlertThreshold handles PUT /observer_alert_thresholds/:id
func (h *LearnerHandler) UpdateAlertThreshold(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return responses.BadRequest(c, "Invalid threshold ID")
	}
	var in thresholdBody
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	t, err := h.alerts.UpdateThreshold(c.Context(), userID, uint(id), in.Threshold.Threshold)
	if err != nil {
		return alertError(c, err)
	}
	return c.JSON(t)
}

// DeleteAlertThreshold handles DELETE /observer_alert_thresholds/:id
func (h *LearnerHandler) DeleteAlertThreshold(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return responses.BadRequest(c, "Invalid threshold ID")
	}
	if err := h.alerts.DeleteThreshold(c.Context(), userID, uint(id)); err != nil {
		return alertError(c, err)
	}
	return c.JSON(fiber.Map{"deleted": true})
}

// ListObserverAlerts handles GET /users/self/observer_alerts?student_id=
func (h *LearnerHandler) ListObserverAlerts(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	studentID, wrote := optionalUintQuery(c, "student_id")
	if wrote {
		return nil
	}
	list, err := h.alerts.ListAlerts(c.Context(), userID, callerAccountID(c), studentID)
	if err != nil {
		return alertError(c, err)
	}
	return c.JSON(list)
}

// ObserverAlertsUnreadCount handles GET /users/self/observer_alerts/unread_count
func (h *LearnerHandler) ObserverAlertsUnreadCount(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	n, err := h.alerts.UnreadCount(c.Context(), userID)
	if err != nil {
		return responses.InternalError(c, "Could not count alerts")
	}
	return c.JSON(fiber.Map{"unread_count": n})
}

// UpdateObserverAlert handles PUT /observer_alerts/:id/:workflow_state
// (read | dismissed).
func (h *LearnerHandler) UpdateObserverAlert(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return responses.BadRequest(c, "Invalid alert ID")
	}
	state := models.ObserverAlertWorkflow(c.Params("workflow_state"))
	if state != models.ObserverAlertRead && state != models.ObserverAlertDismissed {
		return responses.BadRequest(c, "workflow_state must be read or dismissed")
	}
	a, err := h.alerts.SetAlertState(c.Context(), userID, uint(id), state)
	if err != nil {
		return alertError(c, err)
	}
	return c.JSON(a)
}

// --- Profile ---

func profileError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, service.ErrProfileNotFound):
		return responses.NotFound(c, "user")
	case errors.Is(err, service.ErrInvalidProfile), errors.Is(err, service.ErrInvalidAvatar):
		return responses.BadRequest(c, err.Error())
	}
	return responses.InternalError(c, "Could not process the profile request")
}

// userIDParam reads :id accepting "self"; wrote=true after a 400.
func userIDParam(c *fiber.Ctx, callerID uint) (uint, bool) {
	raw := c.Params("id")
	if raw == "self" {
		return callerID, false
	}
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || n == 0 {
		_ = responses.BadRequest(c, "Invalid user ID")
		return 0, true
	}
	return uint(n), false
}

// GetProfileCard handles GET /users/:id/profile_card. The user themself,
// anyone sharing an active course with them, or an admin of their tenant.
func (h *LearnerHandler) GetProfileCard(c *fiber.Ctx) error {
	callerID, err := getUserID(c)
	if err != nil {
		return err
	}
	id, wrote := userIDParam(c, callerID)
	if wrote {
		return nil
	}
	isAdmin := middleware.RoleRank(middleware.EffectiveRole(c, h.users)) >= 1
	p, err := h.profiles.Get(c.Context(), callerID, id, callerAccountID(c), isAdmin)
	if err != nil {
		return profileError(c, err)
	}
	return c.JSON(p)
}

// UpdateProfile handles PUT /users/self/profile {pronouns, bio, links}
func (h *LearnerHandler) UpdateProfile(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	var in struct {
		Pronouns *string                     `json:"pronouns"`
		Bio      *string                     `json:"bio"`
		Links    *[]service.ProfileLinkInput `json:"links"`
	}
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	p, err := h.profiles.Update(c.Context(), userID, callerAccountID(c), service.ProfileUpdate{
		Pronouns: in.Pronouns, Bio: in.Bio, Links: in.Links,
	})
	if err != nil {
		return profileError(c, err)
	}
	return c.JSON(p)
}

// UploadAvatar handles POST /users/self/avatar (multipart field "file").
func (h *LearnerHandler) UploadAvatar(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	fh, err := c.FormFile("file")
	if err != nil {
		return responses.BadRequest(c, "file is required")
	}
	if fh.Size > service.MaxAvatarBytes {
		return responses.BadRequest(c, service.ErrInvalidAvatar.Error())
	}
	f, err := fh.Open()
	if err != nil {
		return responses.BadRequest(c, "Could not read the upload")
	}
	defer func() { _ = f.Close() }()
	u, err := h.profiles.SetAvatar(c.Context(), userID, callerAccountID(c), f)
	if err != nil {
		return profileError(c, err)
	}
	return c.JSON(fiber.Map{"avatar_url": u.AvatarURL})
}

// DeleteAvatar handles DELETE /users/self/avatar
func (h *LearnerHandler) DeleteAvatar(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	if err := h.profiles.RemoveAvatar(c.Context(), userID, callerAccountID(c)); err != nil {
		return profileError(c, err)
	}
	return c.JSON(fiber.Map{"avatar_url": ""})
}

// GetAvatar handles GET /users/:id/avatar — streams an uploaded avatar to
// any signed-in user of the same tenant.
func (h *LearnerHandler) GetAvatar(c *fiber.Ctx) error {
	callerID, err := getUserID(c)
	if err != nil {
		return err
	}
	id, wrote := userIDParam(c, callerID)
	if wrote {
		return nil
	}
	av, err := h.profiles.OpenAvatar(c.Context(), id, callerAccountID(c))
	if err != nil {
		return profileError(c, err)
	}
	c.Set(fiber.HeaderContentType, av.ContentType)
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set(fiber.HeaderCacheControl, "private, max-age=86400")
	c.Set(fiber.HeaderContentDisposition, "inline")
	return c.SendStream(av.Body)
}
