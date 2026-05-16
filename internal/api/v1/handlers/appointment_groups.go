package handlers

import (
	"errors"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type AppointmentGroupHandler struct {
	svc   *service.AppointmentGroupService
	authz *ResourceAuthorizer
}

func NewAppointmentGroupHandler(svc *service.AppointmentGroupService, authz *ResourceAuthorizer) *AppointmentGroupHandler {
	return &AppointmentGroupHandler{svc: svc, authz: authz}
}

// ----- JSON serializers -----

func appointmentGroupToJSON(g *models.AppointmentGroup) fiber.Map {
	return fiber.Map{
		"id":                                g.ID,
		"course_id":                         g.CourseID,
		"title":                             g.Title,
		"description":                       g.Description,
		"location_name":                     g.LocationName,
		"location_address":                  g.LocationAddress,
		"min_appointments_per_participant":  g.MinAppointmentsPerParticipant,
		"max_appointments_per_participant":  g.MaxAppointmentsPerParticipant,
		"participants_per_appointment":      g.ParticipantsPerAppointment,
		"created_by_user_id":                g.CreatedByUserID,
		"workflow_state":                    g.WorkflowState,
		"created_at":                        g.CreatedAt,
		"updated_at":                        g.UpdatedAt,
	}
}

func appointmentSlotToJSON(s *models.AppointmentSlot) fiber.Map {
	return fiber.Map{
		"id":                 s.ID,
		"appointment_group_id": s.GroupID,
		"start_at":           s.StartAt,
		"end_at":             s.EndAt,
		"participants_limit": s.ParticipantsLimit,
		"created_at":         s.CreatedAt,
	}
}

func slotAvailabilityToJSON(a *service.SlotAvailability) fiber.Map {
	m := appointmentSlotToJSON(&a.Slot)
	m["reservation_count"] = a.ReservationCount
	m["effective_limit"] = a.EffectiveLimit
	m["available"] = a.Available
	return m
}

func reservationToJSON(r *models.AppointmentReservation) fiber.Map {
	return fiber.Map{
		"id":             r.ID,
		"slot_id":        r.SlotID,
		"group_id":       r.GroupID,
		"user_id":        r.UserID,
		"reserved_at":    r.ReservedAt,
		"canceled_at":    r.CanceledAt,
		"workflow_state": r.WorkflowState,
	}
}

// ----- Group handlers -----

// List requires a course context (via ?course_id=… or course-scoped route).
func (h *AppointmentGroupHandler) List(c *fiber.Ctx) error {
	courseID, _ := c.ParamsInt("course_id")
	if courseID == 0 {
		if q := c.Query("course_id"); q != "" {
			if v, err := strconv.Atoi(q); err == nil {
				courseID = v
			}
		}
	}
	if courseID <= 0 {
		return responses.BadRequest(c, "course_id is required")
	}
	if err := h.authz.RequireCourseEnrolled(c, uint(courseID)); err != nil {
		return err
	}

	params := middleware.GetPagination(c)
	result, err := h.svc.ListByCourse(c.Context(), uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch appointment groups")
	}
	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	items := make([]fiber.Map, len(result.Items))
	for i, g := range result.Items {
		items[i] = appointmentGroupToJSON(&g)
	}
	return c.JSON(items)
}

func (h *AppointmentGroupHandler) Get(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid appointment group ID")
	}
	g, err := h.svc.GetGroup(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "appointment group")
	}
	if err := h.authz.RequireCourseEnrolled(c, g.CourseID); err != nil {
		return err
	}
	return c.JSON(appointmentGroupToJSON(g))
}

type slotInputJSON struct {
	StartAt           time.Time `json:"start_at"`
	EndAt             time.Time `json:"end_at"`
	ParticipantsLimit *int      `json:"participants_limit"`
}

type appointmentGroupInput struct {
	CourseID                      uint            `json:"course_id"`
	Title                         string          `json:"title"`
	Description                   string          `json:"description"`
	LocationName                  string          `json:"location_name"`
	LocationAddress               string          `json:"location_address"`
	MinAppointmentsPerParticipant int             `json:"min_appointments_per_participant"`
	MaxAppointmentsPerParticipant int             `json:"max_appointments_per_participant"`
	ParticipantsPerAppointment    int             `json:"participants_per_appointment"`
	WorkflowState                 string          `json:"workflow_state"`
	NewAppointments               []slotInputJSON `json:"new_appointments"`
}

func (h *AppointmentGroupHandler) Create(c *fiber.Ctx) error {
	var body struct {
		AppointmentGroup appointmentGroupInput `json:"appointment_group"`
	}
	if err := c.BodyParser(&body); err != nil {
		// Allow flat input as well.
		var flat appointmentGroupInput
		if err2 := c.BodyParser(&flat); err2 != nil {
			return responses.BadRequest(c, "Invalid input")
		}
		body.AppointmentGroup = flat
	}
	in := body.AppointmentGroup
	if in.CourseID == 0 {
		return responses.BadRequest(c, "course_id is required")
	}
	if err := h.authz.RequireCourseInstructor(c, in.CourseID); err != nil {
		return err
	}
	userID, _ := c.Locals("user_id").(uint)

	group := &models.AppointmentGroup{
		CourseID:                      in.CourseID,
		Title:                         in.Title,
		Description:                   in.Description,
		LocationName:                  in.LocationName,
		LocationAddress:               in.LocationAddress,
		MinAppointmentsPerParticipant: in.MinAppointmentsPerParticipant,
		MaxAppointmentsPerParticipant: in.MaxAppointmentsPerParticipant,
		ParticipantsPerAppointment:    in.ParticipantsPerAppointment,
		CreatedByUserID:               userID,
		WorkflowState:                 in.WorkflowState,
	}
	slots := make([]service.SlotInput, 0, len(in.NewAppointments))
	for _, s := range in.NewAppointments {
		slots = append(slots, service.SlotInput{
			StartAt:           s.StartAt,
			EndAt:             s.EndAt,
			ParticipantsLimit: s.ParticipantsLimit,
		})
	}

	created, createdSlots, err := h.svc.CreateGroup(c.Context(), group, slots)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	out := appointmentGroupToJSON(created)
	slotJSON := make([]fiber.Map, len(createdSlots))
	for i, s := range createdSlots {
		slotJSON[i] = appointmentSlotToJSON(&s)
	}
	out["appointments"] = slotJSON
	return c.Status(fiber.StatusCreated).JSON(out)
}

func (h *AppointmentGroupHandler) Update(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid appointment group ID")
	}
	g, err := h.svc.GetGroup(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "appointment group")
	}
	if err := h.authz.RequireCourseInstructor(c, g.CourseID); err != nil {
		return err
	}

	var body struct {
		AppointmentGroup appointmentGroupInput `json:"appointment_group"`
	}
	if err := c.BodyParser(&body); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	in := body.AppointmentGroup
	if in.Title != "" {
		g.Title = in.Title
	}
	g.Description = in.Description
	g.LocationName = in.LocationName
	g.LocationAddress = in.LocationAddress
	if in.MinAppointmentsPerParticipant >= 0 {
		g.MinAppointmentsPerParticipant = in.MinAppointmentsPerParticipant
	}
	if in.MaxAppointmentsPerParticipant > 0 {
		g.MaxAppointmentsPerParticipant = in.MaxAppointmentsPerParticipant
	}
	if in.ParticipantsPerAppointment > 0 {
		g.ParticipantsPerAppointment = in.ParticipantsPerAppointment
	}
	if in.WorkflowState != "" {
		g.WorkflowState = in.WorkflowState
	}
	if err := h.svc.UpdateGroup(c.Context(), g); err != nil {
		return responses.InternalError(c, "Could not update appointment group")
	}
	return c.JSON(appointmentGroupToJSON(g))
}

func (h *AppointmentGroupHandler) Delete(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid appointment group ID")
	}
	g, err := h.svc.GetGroup(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "appointment group")
	}
	if err := h.authz.RequireCourseInstructor(c, g.CourseID); err != nil {
		return err
	}
	if err := h.svc.DeleteGroup(c.Context(), uint(id)); err != nil {
		return responses.InternalError(c, "Could not delete appointment group")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ----- Slot handlers -----

func (h *AppointmentGroupHandler) ListSlots(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid appointment group ID")
	}
	g, err := h.svc.GetGroup(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "appointment group")
	}
	if err := h.authz.RequireCourseEnrolled(c, g.CourseID); err != nil {
		return err
	}
	avail, err := h.svc.ListSlotsWithAvailability(c.Context(), g)
	if err != nil {
		return responses.InternalError(c, "Could not fetch slots")
	}
	includeFull := c.Query("include_full") == "true"
	items := make([]fiber.Map, 0, len(avail))
	for _, a := range avail {
		if !includeFull && !a.Available {
			continue
		}
		items = append(items, slotAvailabilityToJSON(&a))
	}
	return c.JSON(items)
}

// ----- Reservation handlers -----

func (h *AppointmentGroupHandler) ListReservations(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid appointment group ID")
	}
	slotID, err := c.ParamsInt("slot_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid slot ID")
	}
	g, err := h.svc.GetGroup(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "appointment group")
	}
	// Only instructors see the full list of reservations.
	if err := h.authz.RequireCourseInstructor(c, g.CourseID); err != nil {
		return err
	}
	slot, err := h.svc.GetSlot(c.Context(), uint(slotID), callerAccountID(c))
	if err != nil || slot.GroupID != uint(id) {
		return responses.NotFound(c, "slot")
	}
	items, err := h.svc.ListReservations(c.Context(), uint(slotID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch reservations")
	}
	out := make([]fiber.Map, len(items))
	for i, r := range items {
		out[i] = reservationToJSON(&r)
	}
	return c.JSON(out)
}

func (h *AppointmentGroupHandler) Reserve(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid appointment group ID")
	}
	slotID, err := c.ParamsInt("slot_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid slot ID")
	}
	g, err := h.svc.GetGroup(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "appointment group")
	}
	if err := h.authz.RequireCourseEnrolled(c, g.CourseID); err != nil {
		return err
	}
	userID, _ := c.Locals("user_id").(uint)
	if userID == 0 {
		return responses.Unauthorized(c)
	}

	res, err := h.svc.Reserve(c.Context(), uint(id), uint(slotID), userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSlotFull),
			errors.Is(err, service.ErrAlreadyReserved),
			errors.Is(err, service.ErrMaxReservationsHit),
			errors.Is(err, service.ErrSlotInPast),
			errors.Is(err, service.ErrSlotMismatch):
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"errors": []fiber.Map{{"message": err.Error()}},
			})
		default:
			return responses.InternalError(c, "Could not reserve slot")
		}
	}
	return c.Status(fiber.StatusCreated).JSON(reservationToJSON(res))
}

func (h *AppointmentGroupHandler) CancelReservation(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid appointment group ID")
	}
	slotID, err := c.ParamsInt("slot_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid slot ID")
	}
	resID, err := c.ParamsInt("reservation_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid reservation ID")
	}
	g, err := h.svc.GetGroup(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "appointment group")
	}
	res, err := h.svc.GetReservation(c.Context(), uint(resID), callerAccountID(c))
	if err != nil || res.SlotID != uint(slotID) || res.GroupID != uint(id) {
		return responses.NotFound(c, "reservation")
	}
	userID, _ := c.Locals("user_id").(uint)
	// Owner can cancel; instructors can cancel any reservation in their course.
	if res.UserID != userID {
		if err := h.authz.RequireCourseInstructor(c, g.CourseID); err != nil {
			return err
		}
	}
	if err := h.svc.Cancel(c.Context(), res); err != nil {
		return responses.InternalError(c, "Could not cancel reservation")
	}
	return c.JSON(reservationToJSON(res))
}
