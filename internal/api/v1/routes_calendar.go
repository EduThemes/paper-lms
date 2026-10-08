package v1

import (
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/gofiber/fiber/v2"
)

// registerCalendarPublicRoutes mounts the subscribable ICS feed (G096).
// It must run before the protected group is created: a calendar client
// (Google, Outlook, Apple) has no session, and the unguessable token in
// the URL is the only credential. The token grants this read and
// nothing else — it never creates a session. authLimit caps token
// guessing per IP.
func (r *Router) registerCalendarPublicRoutes(api fiber.Router, authLimit fiber.Handler) {
	if r.CalendarHandler == nil {
		return
	}
	api.Get("/feeds/calendars/:token", authLimit, r.CalendarHandler.PublicFeed)
}

// registerCalendarRoutes mounts the aggregated calendar (G094/G146), the
// feed management endpoints (G096) and the scheduler additions (G097).
func (r *Router) registerCalendarRoutes(protected fiber.Router) {
	if r.CalendarHandler != nil {
		protected.Get("/calendar/contexts", r.CalendarHandler.ListContexts)
		protected.Get("/calendar/items", r.CalendarHandler.ListItems)
		protected.Get("/calendar/feed", r.CalendarHandler.GetFeed)
		protected.Post("/calendar/feed", r.CalendarHandler.RotateFeed)
		protected.Delete("/calendar/feed", r.CalendarHandler.RevokeFeed)
	}

	if r.AppointmentGroupHandler != nil {
		protected.Delete("/appointment_groups/:id/appointments/:slot_id", r.AppointmentGroupHandler.DeleteSlot)
		protected.Get("/appointment_groups/:id/users", r.AppointmentGroupHandler.ListParticipants)
		protected.Get("/appointment_groups/:id/groups", r.AppointmentGroupHandler.ListParticipants)
		protected.Post("/appointment_groups/:id/messages", middleware.ExpensiveOpRateLimit(), r.AppointmentGroupHandler.MessageParticipants)
	}
}
