package handlers

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/service"
)

// CalendarHandler serves the aggregated user calendar (G094/G146) and the
// subscribable ICS feed (G096).
type CalendarHandler struct {
	agg   *service.CalendarAggregateService
	feeds *service.CalendarFeedService
	authz *ResourceAuthorizer
}

func NewCalendarHandler(agg *service.CalendarAggregateService, feeds *service.CalendarFeedService, authz *ResourceAuthorizer) *CalendarHandler {
	return &CalendarHandler{agg: agg, feeds: feeds, authz: authz}
}

// ListContexts returns every calendar the caller can see.
// GET /api/v1/calendar/contexts
func (h *CalendarHandler) ListContexts(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	ctxs, err := h.agg.ListContexts(c.Context(), userID, callerAccountID(c), callerIsAdmin(c, h.authz))
	if err != nil {
		return responses.InternalError(c, "Could not load calendars")
	}
	return c.JSON(ctxs)
}

// ListItems returns events, due dates and appointments for a date range.
// GET /api/v1/calendar/items?start_date=&end_date=&context_codes[]=&types[]=
func (h *CalendarHandler) ListItems(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	q, err := parseCalendarQuery(c, userID, callerAccountID(c), callerIsAdmin(c, h.authz))
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	items, err := h.agg.ListItems(c.Context(), q)
	if err != nil {
		return calendarListError(c, err)
	}
	return c.JSON(items)
}

const calendarFeedPathPrefix = "/api/v1/feeds/calendars/"

func feedStatusJSON(c *fiber.Ctx, st *service.FeedTokenStatus) fiber.Map {
	out := fiber.Map{"exists": st.Exists, "url_available": st.Token != ""}
	if st.Exists {
		out["created_at"] = st.CreatedAt
	}
	if st.Token != "" {
		path := calendarFeedPathPrefix + st.Token + ".ics"
		url := c.BaseURL() + path
		out["feed_path"] = path
		out["feed_url"] = url
		out["webcal_url"] = "webcal://" + strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "http://")
	}
	return out
}

// GetFeed shows the caller's feed subscription URL (when one exists).
// GET /api/v1/calendar/feed
func (h *CalendarHandler) GetFeed(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	st, err := h.feeds.Get(c.Context(), userID, false)
	if err != nil {
		return responses.InternalError(c, "Could not load calendar feed")
	}
	return c.JSON(feedStatusJSON(c, st))
}

// RotateFeed creates the feed URL, or replaces it (revoking the old one).
// POST /api/v1/calendar/feed
func (h *CalendarHandler) RotateFeed(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	st, err := h.feeds.Rotate(c.Context(), userID)
	if err != nil {
		return responses.InternalError(c, "Could not create calendar feed")
	}
	return c.Status(fiber.StatusCreated).JSON(feedStatusJSON(c, st))
}

// RevokeFeed turns the feed off.
// DELETE /api/v1/calendar/feed
func (h *CalendarHandler) RevokeFeed(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	if err := h.feeds.Revoke(c.Context(), userID); err != nil {
		return responses.InternalError(c, "Could not revoke calendar feed")
	}
	return c.JSON(fiber.Map{"exists": false})
}

// PublicFeed serves a user's calendar as ICS to a calendar client
// (Google, Outlook, Apple) that cannot carry a session. The token in the
// URL is the only credential and grants nothing but this read; it never
// creates a session. Every failure is the same 404.
// GET /api/v1/feeds/calendars/:token(.ics)
func (h *CalendarHandler) PublicFeed(c *fiber.Ctx) error {
	token := strings.TrimSuffix(c.Params("token"), ".ics")
	user, err := h.feeds.Resolve(c.Context(), token)
	if err != nil {
		// Unknown token, revoked token, suspended owner: all one 404.
		return responses.NotFound(c, "calendar feed")
	}
	now := time.Now()
	items, err := h.agg.ListItems(c.Context(), service.CalendarQuery{
		UserID:    user.ID,
		AccountID: user.AccountID,
		IsAdmin:   false,
		Start:     now.AddDate(0, 0, -30),
		End:       now.AddDate(0, 0, 366),
	})
	if err != nil {
		return responses.InternalError(c, "Could not build calendar feed")
	}
	c.Set("Content-Type", "text/calendar; charset=utf-8")
	c.Set("Cache-Control", "private, max-age=900")
	c.Set("Content-Disposition", "inline; filename=\"calendar.ics\"")
	return c.Send(service.RenderICSIn(items, user.Name+" (Paper LMS)", c.BaseURL(), now, service.LoadCalendarLocation(user.TimeZone)))
}
