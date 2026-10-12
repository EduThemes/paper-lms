package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/service"
)

// ModuleProgressionHandler serves the module progression engine (G061 /
// G062): a student's progress through a course's modules, marking items
// viewed or done, the teacher's per-student progress grid, and the
// content gate that keeps locked module content closed.
type ModuleProgressionHandler struct {
	svc *service.ModuleProgressionService
}

func NewModuleProgressionHandler(svc *service.ModuleProgressionService) *ModuleProgressionHandler {
	return &ModuleProgressionHandler{svc: svc}
}

func moduleProgressToJSON(mp service.ModuleProgress, completedAt interface{}, namer func(uint) string) fiber.Map {
	items := mp.Items
	if items == nil {
		items = []service.ItemProgress{}
	}
	return fiber.Map{
		"module_id":                   mp.ModuleID,
		"name":                        mp.Name,
		"state":                       mp.State,
		"unlock_at":                   mp.UnlockAt,
		"locked_until_date":           mp.LockedUntilDate,
		"prerequisite_module_ids":     mp.PrerequisiteModuleIDs,
		"prerequisites_unmet":         mp.PrerequisitesUnmet,
		"require_sequential_progress": mp.RequireSequentialProgress,
		"requirement_count":           mp.RequirementCount,
		"current_position":            mp.CurrentPosition,
		"requirements_met":            mp.RequirementsMet,
		"completed_at":                completedAt,
		"lock_explanation":            mp.LockExplanation(namer),
		"items":                       items,
	}
}

// progressSubject resolves whose progress the caller asked for. Students
// read their own; course staff may read any student of the course
// (?user_id=); an observer may read a student they observe. Writes the
// response and returns wrote=true when the request is refused.
func (h *ModuleProgressionHandler) progressSubject(c *fiber.Ctx, courseID uint) (uint, bool, bool) {
	self := callerUserID(c)
	target := self
	if q := c.Query("user_id"); q != "" && q != "self" {
		n, err := strconv.ParseUint(q, 10, 64)
		if err != nil || n == 0 {
			_ = responses.BadRequest(c, "Invalid user_id")
			return 0, false, true
		}
		target = uint(n)
	}
	if target != self {
		allowed := canViewUnpublishedContent(c)
		if !allowed {
			obs, err := h.svc.ObservesStudent(c.Context(), self, target, courseID)
			if err != nil {
				_ = responses.InternalError(c, "Could not load module progress")
				return 0, false, true
			}
			allowed = obs
		}
		if !allowed {
			_ = responses.NotFound(c, "user")
			return 0, false, true
		}
	}
	learner, err := h.svc.IsLearner(c.Context(), target, courseID)
	if err != nil {
		_ = responses.InternalError(c, "Could not load module progress")
		return 0, false, true
	}
	if target != self && !learner {
		// Only students have module progress; a staff member or a user
		// outside the course looks the same as an unknown user.
		_ = responses.NotFound(c, "user")
		return 0, false, true
	}
	return target, learner, false
}

// GetProgress returns a student's progress through the course's modules.
// GET /courses/:course_id/module_progress[?user_id=]
func (h *ModuleProgressionHandler) GetProgress(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}
	if ok, err := h.svc.CourseInAccount(c.Context(), uint(courseID), callerAccountID(c)); err != nil || !ok {
		return responses.NotFound(c, "course")
	}
	userID, learner, wrote := h.progressSubject(c, uint(courseID))
	if wrote {
		return nil
	}
	if !learner {
		// Course staff have no progression: nothing is locked for them.
		return c.JSON(fiber.Map{"user_id": userID, "student": false, "modules": []fiber.Map{}})
	}
	progress, err := h.svc.Evaluate(c.Context(), uint(courseID), userID)
	if err != nil {
		return responses.InternalError(c, "Could not load module progress")
	}
	ids := make([]uint, len(progress.Modules))
	for i, m := range progress.Modules {
		ids[i] = m.ModuleID
	}
	completed, err := h.svc.CompletedAt(c.Context(), userID, ids)
	if err != nil {
		return responses.InternalError(c, "Could not load module progress")
	}
	namer := progress.ModuleNamer()
	mods := make([]fiber.Map, len(progress.Modules))
	for i, m := range progress.Modules {
		mods[i] = moduleProgressToJSON(m, completed[m.ModuleID], namer)
	}
	return c.JSON(fiber.Map{"user_id": userID, "student": true, "modules": mods})
}

// itemParams parses :course_id, :module_id and :item_id. Writes a 400 and
// returns wrote=true on a malformed id.
func itemParams(c *fiber.Ctx) (courseID, moduleID, itemID uint, wrote bool) {
	cid, err1 := c.ParamsInt("course_id")
	mid, err2 := c.ParamsInt("module_id")
	iid, err3 := c.ParamsInt("item_id")
	if err1 != nil || err2 != nil || err3 != nil || cid <= 0 || mid <= 0 || iid <= 0 {
		_ = responses.BadRequest(c, "Invalid course, module or item ID")
		return 0, 0, 0, true
	}
	return uint(cid), uint(mid), uint(iid), false
}

// requireItemInModule verifies the item belongs to :module_id of the
// course, as the student sees it (F-013 parent tie). Writes a 404 and
// returns wrote=true on any miss.
func (h *ModuleProgressionHandler) requireItemInModule(c *fiber.Ctx, courseID, moduleID, itemID uint) bool {
	if ok, err := h.svc.CourseInAccount(c.Context(), courseID, callerAccountID(c)); err != nil || !ok {
		_ = responses.NotFound(c, "module item")
		return true
	}
	progress, err := h.svc.Evaluate(c.Context(), courseID, callerUserID(c))
	if err != nil {
		_ = responses.InternalError(c, "Could not load module progress")
		return true
	}
	mp, ip := progress.Item(itemID)
	if ip == nil || mp.ModuleID != moduleID {
		_ = responses.NotFound(c, "module item")
		return true
	}
	return false
}

// writeItemError maps a progression write error to a response.
func writeItemError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, service.ErrModuleItemNotFound):
		return responses.NotFound(c, "module item")
	case errors.Is(err, service.ErrModuleItemLocked):
		return responses.Forbidden(c, "This module item is locked")
	case errors.Is(err, service.ErrModuleItemNotMarkable):
		return responses.BadRequest(c, "This module item does not have a mark-as-done requirement")
	default:
		return responses.InternalError(c, "Could not update module progress")
	}
}

// MarkItemRead records that the student viewed a module item (Canvas
// mark_read). Course staff calls are a no-op.
// POST /courses/:course_id/modules/:module_id/items/:item_id/mark_read
func (h *ModuleProgressionHandler) MarkItemRead(c *fiber.Ctx) error {
	courseID, moduleID, itemID, wrote := itemParams(c)
	if wrote {
		return nil
	}
	learner, err := h.svc.IsLearner(c.Context(), callerUserID(c), courseID)
	if err != nil {
		return responses.InternalError(c, "Could not update module progress")
	}
	if !learner {
		return c.SendStatus(fiber.StatusNoContent)
	}
	if h.requireItemInModule(c, courseID, moduleID, itemID) {
		return nil
	}
	if err := h.svc.RecordItemView(c.Context(), courseID, callerUserID(c), itemID); err != nil {
		return writeItemError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// MarkItemDone / UnmarkItemDone toggle a must_mark_done requirement.
// PUT|DELETE /courses/:course_id/modules/:module_id/items/:item_id/done
func (h *ModuleProgressionHandler) MarkItemDone(c *fiber.Ctx) error   { return h.setDone(c, true) }
func (h *ModuleProgressionHandler) UnmarkItemDone(c *fiber.Ctx) error { return h.setDone(c, false) }

func (h *ModuleProgressionHandler) setDone(c *fiber.Ctx, done bool) error {
	courseID, moduleID, itemID, wrote := itemParams(c)
	if wrote {
		return nil
	}
	learner, err := h.svc.IsLearner(c.Context(), callerUserID(c), courseID)
	if err != nil {
		return responses.InternalError(c, "Could not update module progress")
	}
	if !learner {
		return responses.Forbidden(c, "Only students can mark module items done")
	}
	if h.requireItemInModule(c, courseID, moduleID, itemID) {
		return nil
	}
	if err := h.svc.SetItemDone(c.Context(), courseID, callerUserID(c), itemID, done); err != nil {
		return writeItemError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// Roster returns every student's state on every module (the teacher's
// "student progress" view).
// GET /courses/:course_id/module_progressions
func (h *ModuleProgressionHandler) Roster(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}
	if ok, err := h.svc.CourseInAccount(c.Context(), uint(courseID), callerAccountID(c)); err != nil || !ok {
		return responses.NotFound(c, "course")
	}
	roster, err := h.svc.CourseRoster(c.Context(), uint(courseID))
	if err != nil {
		return responses.InternalError(c, "Could not load module progress")
	}
	return c.JSON(roster)
}

// ---------------------------------------------------------------------
// Content gate
// ---------------------------------------------------------------------

type gatedRoute struct {
	method string
	re     *regexp.Regexp
	kind   string
	// view: a successful request counts as viewing the content.
	view bool
	// file: the second capture is an attachment id and the route has no
	// course id (the course comes from the attachment).
	file bool
	// frontPage: the route names no page; the course's front page is
	// the content.
	frontPage bool
}

// gatedRoutes are the content reads/writes a locked module item must
// close for a student: opening the content, and submitting/posting to
// it. Paths are matched with or without the /api/v1 prefix.
//
// The patterns must accept every spelling Fiber routes to the content
// handler: the app uses Fiber's default (non-strict, case-insensitive)
// routing, so a trailing slash and any letter case reach the same
// handler (matchGatedRoute trims the slash, (?i) handles case), and id
// segments are captured loosely and parsed with strconv.Atoi exactly as
// c.ParamsInt does ("+5" is 5).
var gatedRoutes = []gatedRoute{
	{method: fiber.MethodGet, re: regexp.MustCompile(`(?i)^(?:/api/v1)?/courses/([^/]+)/assignments/([^/]+)$`), kind: service.GatedAssignment, view: true},
	{method: fiber.MethodPost, re: regexp.MustCompile(`(?i)^(?:/api/v1)?/courses/([^/]+)/assignments/([^/]+)/submissions(?:/self/files)?$`), kind: service.GatedAssignment},
	{method: fiber.MethodGet, re: regexp.MustCompile(`(?i)^(?:/api/v1)?/courses/([^/]+)/pages/([^/]+)$`), kind: service.GatedPage, view: true},
	// The front page is a page too: a locked one stays locked here.
	{method: fiber.MethodGet, re: regexp.MustCompile(`(?i)^(?:/api/v1)?/courses/([^/]+)/front_page$`), kind: service.GatedPage, view: true, frontPage: true},
	{method: fiber.MethodGet, re: regexp.MustCompile(`(?i)^(?:/api/v1)?/courses/([^/]+)/quizzes/([^/]+)$`), kind: service.GatedQuiz, view: true},
	{method: fiber.MethodPost, re: regexp.MustCompile(`(?i)^(?:/api/v1)?/courses/([^/]+)/quizzes/([^/]+)/submissions$`), kind: service.GatedQuiz},
	// The topic and its full views (/view, /view_v2 — the discussion
	// page reads view_v2) count as opening it.
	{method: fiber.MethodGet, re: regexp.MustCompile(`(?i)^(?:/api/v1)?/courses/([^/]+)/discussion_topics/([^/]+)(?:/view|/view_v2)?$`), kind: service.GatedDiscussion, view: true},
	// Every other read under the topic (entries, replies, versions,
	// unread counts) exposes its content too.
	{method: fiber.MethodGet, re: regexp.MustCompile(`(?i)^(?:/api/v1)?/courses/([^/]+)/discussion_topics/([^/]+)/.+$`), kind: service.GatedDiscussion},
	{method: fiber.MethodPost, re: regexp.MustCompile(`(?i)^(?:/api/v1)?/courses/([^/]+)/discussion_topics/([^/]+)/entries(?:/[^/]+/replies)?$`), kind: service.GatedDiscussion},
	{method: fiber.MethodGet, re: regexp.MustCompile(`(?i)^(?:/api/v1)?/courses/([^/]+)/files/([^/]+)$`), kind: service.GatedFile, view: true},
	{method: fiber.MethodGet, re: regexp.MustCompile(`(?i)^(?:/api/v1)?/files/([^/]+)/download$`), kind: service.GatedFile, view: true, file: true},
}

// gateParamID parses a captured id segment the way c.ParamsInt does. ok
// is false for anything the content handler would itself reject.
func gateParamID(s string) (uint, bool) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, false
	}
	return uint(n), true
}

// ContentGate enforces module locks server-side (G061): a student may
// not open — or submit to — content whose every module item is locked
// (unlock date, prerequisites, sequential progress), and an unreleased
// Mastery Paths assignment does not exist for them (G080). Opening
// content that sits in a module records the must_view requirement.
//
// It runs on the protected group before the route handlers, so it
// resolves the caller's course role itself; anyone who is not a student
// of the course passes straight through, and the route's own guards
// still apply afterwards.
func (h *ModuleProgressionHandler) ContentGate() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if h == nil || h.svc == nil {
			return c.Next()
		}
		if idx, m := matchGatedIndex(c.Method(), c.Path()); idx != nil {
			return h.redactLockedIndex(c, idx, m)
		}
		route, m := matchGatedRoute(c.Method(), c.Path())
		if route == nil {
			return c.Next()
		}
		userID := callerUserID(c)
		if userID == 0 {
			return c.Next()
		}
		ctx := c.Context()

		// An id the handler cannot parse is rejected by the handler, and
		// content that does not exist has nothing to protect: both pass.
		var courseID, contentID uint
		if route.file {
			id, ok := gateParamID(m[1])
			if !ok {
				return c.Next()
			}
			cid, found, err := h.svc.AttachmentCourseID(ctx, id)
			if err != nil {
				slog.Warn("module content gate: attachment lookup failed", "attachment_id", id, "err", err)
				return responses.InternalError(c, "Could not check module access")
			}
			if !found {
				return c.Next()
			}
			courseID, contentID = cid, id
		} else {
			cid, ok := gateParamID(m[1])
			if !ok {
				return c.Next()
			}
			courseID = cid
			if route.frontPage {
				pid, found, err := h.svc.ResolveFrontPageID(ctx, courseID)
				if err != nil {
					slog.Warn("module content gate: front page lookup failed", "course_id", courseID, "err", err)
					return responses.InternalError(c, "Could not check module access")
				}
				if !found {
					return c.Next()
				}
				contentID = pid
			} else if route.kind == service.GatedPage {
				pid, found, err := h.svc.ResolvePageID(ctx, courseID, m[2])
				if err != nil {
					slog.Warn("module content gate: page lookup failed", "course_id", courseID, "err", err)
					return responses.InternalError(c, "Could not check module access")
				}
				if !found {
					return c.Next()
				}
				contentID = pid
			} else {
				id, ok := gateParamID(m[2])
				if !ok {
					return c.Next()
				}
				contentID = id
			}
		}

		learner, err := h.svc.IsLearner(ctx, userID, courseID)
		if err != nil {
			slog.Warn("module content gate: enrollment lookup failed", "course_id", courseID, "err", err)
			return responses.InternalError(c, "Could not check module access")
		}
		if !learner {
			return c.Next()
		}

		access, err := h.svc.CheckContentAccess(ctx, courseID, userID, route.kind, contentID)
		if err != nil {
			slog.Warn("module content gate: evaluation failed", "course_id", courseID, "err", err)
			return responses.InternalError(c, "Could not check module access")
		}
		if access.Hidden {
			return responses.NotFound(c, "assignment")
		}
		if access.Locked {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"errors": []fiber.Map{{"message": access.Explanation, "code": "module_locked"}},
				"lock_info": fiber.Map{
					"context_module_id": access.ModuleID,
					"unlock_at":         access.UnlockAt,
				},
				"lock_explanation": access.Explanation,
			})
		}

		if err := c.Next(); err != nil {
			return err
		}
		if route.view && len(access.ItemIDs) > 0 && c.Response().StatusCode() < 300 {
			if err := h.svc.RecordContentView(ctx, userID, access.ItemIDs); err != nil {
				slog.Warn("module content gate: could not record view", "course_id", courseID, "err", err)
			}
		}
		return nil
	}
}

// gatedIndex is a course content index whose rows carry the content's
// body. A locked row keeps its title and dates but loses the body, as
// Canvas withholds a locked item's description from the student.
type gatedIndex struct {
	re      *regexp.Regexp
	kind    string
	idField string
	body    string
}

var gatedIndexes = []gatedIndex{
	{re: regexp.MustCompile(`(?i)^(?:/api/v1)?/courses/([^/]+)/assignments$`), kind: service.GatedAssignment, idField: "id", body: "description"},
	{re: regexp.MustCompile(`(?i)^(?:/api/v1)?/courses/([^/]+)/quizzes$`), kind: service.GatedQuiz, idField: "id", body: "description"},
	{re: regexp.MustCompile(`(?i)^(?:/api/v1)?/courses/([^/]+)/discussion_topics$`), kind: service.GatedDiscussion, idField: "id", body: "message"},
	{re: regexp.MustCompile(`(?i)^(?:/api/v1)?/courses/([^/]+)/pages$`), kind: service.GatedPage, idField: "page_id", body: "body"},
}

func matchGatedIndex(method, path string) (*gatedIndex, []string) {
	if method != fiber.MethodGet {
		return nil, nil
	}
	path = strings.TrimRight(path, "/")
	for i := range gatedIndexes {
		if m := gatedIndexes[i].re.FindStringSubmatch(path); m != nil {
			return &gatedIndexes[i], m
		}
	}
	return nil, nil
}

// redactLockedIndex runs the index handler, then, for a student of the
// course, blanks the body of every row whose content is module-locked
// and marks it locked_for_user with the lock explanation. The opening
// routes refuse such content outright (ContentGate); without this the
// index would hand the same body over.
func (h *ModuleProgressionHandler) redactLockedIndex(c *fiber.Ctx, idx *gatedIndex, m []string) error {
	if err := c.Next(); err != nil {
		return err
	}
	userID := callerUserID(c)
	courseID, ok := gateParamID(m[1])
	if userID == 0 || !ok || c.Response().StatusCode() != fiber.StatusOK {
		return nil
	}
	ctx := c.Context()
	learner, err := h.svc.IsLearner(ctx, userID, courseID)
	if err != nil || !learner {
		if err != nil {
			slog.Warn("module content gate: enrollment lookup failed", "course_id", courseID, "err", err)
			return responses.InternalError(c, "Could not check module access")
		}
		return nil
	}
	var rows []map[string]interface{}
	dec := json.NewDecoder(bytes.NewReader(c.Response().Body()))
	dec.UseNumber()
	if err := dec.Decode(&rows); err != nil || len(rows) == 0 {
		// Not a JSON array of rows (an error body, an empty page).
		return nil
	}
	locked, err := h.svc.LockedContent(ctx, courseID, userID)
	if err != nil {
		slog.Warn("module content gate: evaluation failed", "course_id", courseID, "err", err)
		return responses.InternalError(c, "Could not check module access")
	}
	byID := locked[idx.kind]
	if len(byID) == 0 {
		return nil
	}
	changed := false
	for _, row := range rows {
		n, ok := row[idx.idField].(json.Number)
		if !ok {
			continue
		}
		id, err := strconv.ParseUint(n.String(), 10, 64)
		if err != nil {
			continue
		}
		explanation, isLocked := byID[uint(id)]
		if !isLocked {
			continue
		}
		row[idx.body] = ""
		row["locked_for_user"] = true
		row["lock_explanation"] = explanation
		changed = true
	}
	if !changed {
		return nil
	}
	return c.JSON(rows)
}

func matchGatedRoute(method, path string) (*gatedRoute, []string) {
	// Non-strict routing sends "/x/" to the "/x" handler.
	path = strings.TrimRight(path, "/")
	for i := range gatedRoutes {
		r := &gatedRoutes[i]
		if r.method != method {
			continue
		}
		if m := r.re.FindStringSubmatch(path); m != nil {
			return r, m
		}
	}
	return nil, nil
}
