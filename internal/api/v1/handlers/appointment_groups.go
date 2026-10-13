package handlers

import (
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

type AppointmentGroupHandler struct {
	svc   *service.AppointmentGroupService
	authz *ResourceAuthorizer
	// Wave 5.2: 13.5 LogPIIAccess sweep. ListReservations is the only
	// student-keyed read here — it surfaces every student who booked a
	// given slot. Group/Slot detail reads are course-keyed metadata and
	// don't fire LogPIIAccess.
	auditService *service.AuditService
	// conversations sends "message participants" (G097). Optional.
	conversations *service.ConversationService
}

func NewAppointmentGroupHandler(svc *service.AppointmentGroupService, authz *ResourceAuthorizer, auditService *service.AuditService) *AppointmentGroupHandler {
	return &AppointmentGroupHandler{svc: svc, authz: authz, auditService: auditService}
}

// SetMessenger wires the conversation service used to message an
// appointment group's participants.
func (h *AppointmentGroupHandler) SetMessenger(conv *service.ConversationService) {
	h.conversations = conv
}

// ----- JSON serializers -----

func appointmentGroupToJSON(g *models.AppointmentGroup) fiber.Map {
	participantType := g.ParticipantType
	if participantType == "" {
		participantType = models.AppointmentParticipantUser
	}
	return fiber.Map{
		"id":                               g.ID,
		"course_id":                        g.CourseID,
		"title":                            g.Title,
		"description":                      g.Description,
		"location_name":                    g.LocationName,
		"location_address":                 g.LocationAddress,
		"min_appointments_per_participant": g.MinAppointmentsPerParticipant,
		"max_appointments_per_participant": g.MaxAppointmentsPerParticipant,
		"participants_per_appointment":     g.ParticipantsPerAppointment,
		"participant_type":                 participantType,
		"group_category_id":                g.GroupCategoryID,
		"created_by_user_id":               g.CreatedByUserID,
		"workflow_state":                   g.WorkflowState,
		"created_at":                       g.CreatedAt,
		"updated_at":                       g.UpdatedAt,
	}
}

// withContexts adds the Canvas context_codes / sub_context_codes.
func withContexts(m fiber.Map, g *models.AppointmentGroup, contexts []models.AppointmentGroupContext) fiber.Map {
	codes := []string{}
	for _, id := range service.CourseIDs(g, contexts) {
		codes = append(codes, fmt.Sprintf("course_%d", id))
	}
	sub := []string{}
	for _, c := range contexts {
		if c.ContextType == models.CalendarContextCourseSection {
			sub = append(sub, fmt.Sprintf("course_section_%d", c.ContextID))
		}
	}
	if g.GroupCategoryID != nil {
		sub = append(sub, fmt.Sprintf("group_category_%d", *g.GroupCategoryID))
	}
	m["context_codes"] = codes
	m["sub_context_codes"] = sub
	return m
}

func appointmentSlotToJSON(s *models.AppointmentSlot) fiber.Map {
	return fiber.Map{
		"id":                   s.ID,
		"appointment_group_id": s.GroupID,
		"start_at":             s.StartAt,
		"end_at":               s.EndAt,
		"participants_limit":   s.ParticipantsLimit,
		"created_at":           s.CreatedAt,
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
		"id":                   r.ID,
		"slot_id":              r.SlotID,
		"group_id":             r.GroupID,
		"user_id":              r.UserID,
		"participant_group_id": r.ParticipantGroupID,
		"comments":             r.Comments,
		"reserved_at":          r.ReservedAt,
		"canceled_at":          r.CanceledAt,
		"workflow_state":       r.WorkflowState,
	}
}

// ----- access -----

// groupAccess decides how the caller relates to an appointment group:
// manager (instructor of any of its courses, or admin) or participant
// (enrolled in one of its courses, inside any section limit). It never
// writes the response; a non-nil error is the refusal to return. An
// unpublished (pending) group is invisible to participants.
func (h *AppointmentGroupHandler) groupAccess(c *fiber.Ctx, g *models.AppointmentGroup) (bool, []models.AppointmentGroupContext, error) {
	contexts, err := h.svc.GroupContexts(c.Context(), g.ID)
	if err != nil {
		return false, nil, fiber.NewError(fiber.StatusInternalServerError, "Could not load appointment group")
	}
	courses := service.CourseIDs(g, contexts)
	for _, cid := range courses {
		if callerIsCourseInstructor(c, h.authz, cid) {
			return true, contexts, nil
		}
	}
	userID, _ := c.Locals("user_id").(uint)
	for _, cid := range courses {
		if h.authz.RequireCourseEnrolled(c, cid) != nil || !h.svc.SectionAllowed(c.Context(), contexts, cid, userID) {
			continue
		}
		if g.WorkflowState != "active" {
			return false, nil, authzNotFound("appointment group")
		}
		return false, contexts, nil
	}
	// Not enrolled in any of its courses, or enrolled only in sections
	// the group is not for: the group does not exist for the caller (404,
	// not 403, so group ids can't be probed).
	return false, nil, authzNotFound("appointment group")
}

// ----- Group handlers -----

// List returns a course's appointment groups (course_id in the path or
// query), or — with no course — the caller's groups across courses:
// scope=reservable (default, active groups they can sign up for) or
// scope=manageable (groups in courses they teach).
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
		if !h.svc.HasDB() {
			return responses.BadRequest(c, "course_id is required")
		}
		userID, err := getUserID(c)
		if err != nil {
			return err
		}
		groups, err := h.svc.ListGroupsForUser(c.Context(), userID, callerAccountID(c), c.Query("scope", "reservable"))
		if err != nil {
			return responses.InternalError(c, "Could not fetch appointment groups")
		}
		items := make([]fiber.Map, 0, len(groups))
		for i := range groups {
			g := groups[i]
			ctxs, _ := h.svc.GroupContexts(c.Context(), g.ID)
			if c.Query("scope") != "manageable" && !h.svc.CanParticipate(c.Context(), &g, ctxs, userID) {
				continue
			}
			items = append(items, withContexts(appointmentGroupToJSON(&g), &g, ctxs))
		}
		return c.JSON(items)
	}
	if err := h.authz.RequireCourseEnrolled(c, uint(courseID)); err != nil {
		return err
	}
	manager := callerIsCourseInstructor(c, h.authz, uint(courseID))

	params := middleware.GetPagination(c)
	result, err := h.svc.ListByCourse(c.Context(), uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch appointment groups")
	}
	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	userID, _ := c.Locals("user_id").(uint)
	items := make([]fiber.Map, 0, len(result.Items))
	for i := range result.Items {
		g := result.Items[i]
		ctxs, _ := h.svc.GroupContexts(c.Context(), g.ID)
		if !manager {
			// Students only see published groups meant for their section.
			if g.WorkflowState != "active" || !h.svc.SectionAllowed(c.Context(), ctxs, uint(courseID), userID) {
				continue
			}
		}
		items = append(items, withContexts(appointmentGroupToJSON(&g), &g, ctxs))
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
	manager, ctxs, err := h.groupAccess(c, g)
	if err != nil {
		return err
	}
	out := withContexts(appointmentGroupToJSON(g), g, ctxs)
	out["can_manage"] = manager
	return c.JSON(out)
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
	ContextCodes                  []string        `json:"context_codes"`
	SubContextCodes               []string        `json:"sub_context_codes"`
	ParticipantType               string          `json:"participant_type"`
}

// parsedContexts is context_codes / sub_context_codes split by kind.
type parsedContexts struct {
	courses         []uint
	sections        []uint
	groupCategoryID *uint
}

func parseAppointmentCodes(contextCodes, subCodes []string) (parsedContexts, error) {
	var p parsedContexts
	num := func(code, prefix string) (uint, error) {
		v, err := strconv.ParseUint(strings.TrimPrefix(code, prefix), 10, 64)
		if err != nil || v == 0 {
			return 0, fmt.Errorf("invalid context code %q", code)
		}
		return uint(v), nil
	}
	for _, code := range contextCodes {
		code = strings.TrimSpace(code)
		if !strings.HasPrefix(code, "course_") || strings.HasPrefix(code, "course_section_") {
			return p, fmt.Errorf("context_codes must be course_<id>, got %q", code)
		}
		id, err := num(code, "course_")
		if err != nil {
			return p, err
		}
		p.courses = append(p.courses, id)
	}
	for _, code := range subCodes {
		code = strings.TrimSpace(code)
		switch {
		case strings.HasPrefix(code, "course_section_"):
			id, err := num(code, "course_section_")
			if err != nil {
				return p, err
			}
			p.sections = append(p.sections, id)
		case strings.HasPrefix(code, "group_category_"):
			id, err := num(code, "group_category_")
			if err != nil {
				return p, err
			}
			if p.groupCategoryID != nil && *p.groupCategoryID != id {
				return p, errors.New("only one group set can sign up for an appointment group")
			}
			p.groupCategoryID = &id
		default:
			return p, fmt.Errorf("sub_context_codes must be course_section_<id> or group_category_<id>, got %q", code)
		}
	}
	if p.groupCategoryID != nil && len(p.sections) > 0 {
		return p, errors.New("an appointment group is limited either to sections or to a group set, not both")
	}
	return p, nil
}

// requireInstructorOfAll refuses unless the caller teaches every course.
func (h *AppointmentGroupHandler) requireInstructorOfAll(c *fiber.Ctx, courseIDs []uint) error {
	for _, id := range courseIDs {
		if err := h.authz.RequireCourseInstructor(c, id); err != nil {
			return err
		}
	}
	return nil
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
	switch in.WorkflowState {
	case "", "active", "pending":
	default:
		// Same rule as Update: a group can't be created already deleted
		// (or in a state participants' visibility checks don't know).
		return responses.BadRequest(c, "workflow_state must be 'active' or 'pending'")
	}
	parsed, err := parseAppointmentCodes(in.ContextCodes, in.SubContextCodes)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	// The primary course: the URL's, the body's, else the first context.
	if pathCourse, _ := c.ParamsInt("course_id"); pathCourse > 0 {
		if in.CourseID != 0 && in.CourseID != uint(pathCourse) {
			return responses.BadRequest(c, "course_id does not match the course in the URL")
		}
		in.CourseID = uint(pathCourse)
	}
	if in.CourseID == 0 && len(parsed.courses) > 0 {
		in.CourseID = parsed.courses[0]
	}
	if in.CourseID == 0 {
		return responses.BadRequest(c, "course_id is required")
	}
	if err := h.requireInstructorOfAll(c, append([]uint{in.CourseID}, parsed.courses...)); err != nil {
		return err
	}
	userID, _ := c.Locals("user_id").(uint)

	participantType := models.AppointmentParticipantUser
	if parsed.groupCategoryID != nil || in.ParticipantType == models.AppointmentParticipantGroup {
		if parsed.groupCategoryID == nil {
			return responses.BadRequest(c, "a group appointment group needs a group set (sub_context_codes group_category_<id>)")
		}
		participantType = models.AppointmentParticipantGroup
	}
	contexts, err := h.svc.ValidateContexts(c.Context(), callerAccountID(c), in.CourseID, parsed.courses, parsed.sections, parsed.groupCategoryID)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	group := &models.AppointmentGroup{
		CourseID:                      in.CourseID,
		Title:                         in.Title,
		Description:                   service.SanitizePlainText(in.Description),
		LocationName:                  in.LocationName,
		LocationAddress:               in.LocationAddress,
		MinAppointmentsPerParticipant: in.MinAppointmentsPerParticipant,
		MaxAppointmentsPerParticipant: in.MaxAppointmentsPerParticipant,
		ParticipantsPerAppointment:    in.ParticipantsPerAppointment,
		CreatedByUserID:               userID,
		WorkflowState:                 in.WorkflowState,
		ParticipantType:               participantType,
		GroupCategoryID:               parsed.groupCategoryID,
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
	if len(contexts) > 0 {
		if err := h.svc.SetGroupContexts(c.Context(), created.ID, contexts); err != nil {
			return responses.InternalError(c, "Could not save appointment group courses")
		}
	}

	out := withContexts(appointmentGroupToJSON(created), created, contexts)
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

	// Partial update: only fields present in the body change. Pointer
	// fields distinguish "omitted" from "set to empty/zero", so a
	// title-only PUT no longer wipes description and location.
	var body struct {
		AppointmentGroup struct {
			Title                         *string         `json:"title"`
			Description                   *string         `json:"description"`
			LocationName                  *string         `json:"location_name"`
			LocationAddress               *string         `json:"location_address"`
			MinAppointmentsPerParticipant *int            `json:"min_appointments_per_participant"`
			MaxAppointmentsPerParticipant *int            `json:"max_appointments_per_participant"`
			ParticipantsPerAppointment    *int            `json:"participants_per_appointment"`
			WorkflowState                 *string         `json:"workflow_state"`
			NewAppointments               []slotInputJSON `json:"new_appointments"`
			ContextCodes                  []string        `json:"context_codes"`
			SubContextCodes               []string        `json:"sub_context_codes"`
		} `json:"appointment_group"`
	}
	if err := c.BodyParser(&body); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	in := body.AppointmentGroup
	if in.Title != nil {
		if *in.Title == "" {
			return responses.BadRequest(c, "title cannot be empty")
		}
		g.Title = *in.Title
	}
	if in.Description != nil {
		g.Description = service.SanitizePlainText(*in.Description)
	}
	if in.LocationName != nil {
		g.LocationName = *in.LocationName
	}
	if in.LocationAddress != nil {
		g.LocationAddress = *in.LocationAddress
	}
	if in.MinAppointmentsPerParticipant != nil && *in.MinAppointmentsPerParticipant >= 0 {
		g.MinAppointmentsPerParticipant = *in.MinAppointmentsPerParticipant
	}
	if in.MaxAppointmentsPerParticipant != nil && *in.MaxAppointmentsPerParticipant > 0 {
		g.MaxAppointmentsPerParticipant = *in.MaxAppointmentsPerParticipant
	}
	if in.ParticipantsPerAppointment != nil && *in.ParticipantsPerAppointment > 0 {
		g.ParticipantsPerAppointment = *in.ParticipantsPerAppointment
	}
	if in.WorkflowState != nil && *in.WorkflowState != "" {
		switch *in.WorkflowState {
		case "active", "pending":
			g.WorkflowState = *in.WorkflowState
		default:
			return responses.BadRequest(c, "workflow_state must be 'active' or 'pending'")
		}
	}

	// Courses and section limits are replaced when either list is sent.
	var contexts []models.AppointmentGroupContext
	contextsChanged := in.ContextCodes != nil || in.SubContextCodes != nil
	if contextsChanged {
		parsed, err := parseAppointmentCodes(in.ContextCodes, in.SubContextCodes)
		if err != nil {
			return responses.BadRequest(c, err.Error())
		}
		if parsed.groupCategoryID != nil && (g.GroupCategoryID == nil || *g.GroupCategoryID != *parsed.groupCategoryID) {
			return responses.BadRequest(c, "the group set of an appointment group cannot be changed")
		}
		if g.GroupCategoryID != nil && len(parsed.sections) > 0 {
			return responses.BadRequest(c, "an appointment group is limited either to sections or to a group set, not both")
		}
		if err := h.requireInstructorOfAll(c, parsed.courses); err != nil {
			return err
		}
		contexts, err = h.svc.ValidateContexts(c.Context(), callerAccountID(c), g.CourseID, parsed.courses, parsed.sections, g.GroupCategoryID)
		if err != nil {
			return responses.BadRequest(c, err.Error())
		}
	}
	newSlots := make([]service.SlotInput, 0, len(in.NewAppointments))
	for _, s := range in.NewAppointments {
		if !s.EndAt.After(s.StartAt) {
			return responses.BadRequest(c, service.ErrInvalidSlotTimes.Error())
		}
		newSlots = append(newSlots, service.SlotInput{StartAt: s.StartAt, EndAt: s.EndAt, ParticipantsLimit: s.ParticipantsLimit})
	}

	if err := h.svc.UpdateGroup(c.Context(), g); err != nil {
		return responses.InternalError(c, "Could not update appointment group")
	}
	if contextsChanged {
		if err := h.svc.SetGroupContexts(c.Context(), g.ID, contexts); err != nil {
			return responses.InternalError(c, "Could not save appointment group courses")
		}
	} else {
		contexts, _ = h.svc.GroupContexts(c.Context(), g.ID)
	}
	out := withContexts(appointmentGroupToJSON(g), g, contexts)
	if len(newSlots) > 0 {
		created, err := h.svc.AddSlots(c.Context(), g.ID, newSlots)
		if err != nil {
			return responses.BadRequest(c, err.Error())
		}
		slotJSON := make([]fiber.Map, len(created))
		for i := range created {
			slotJSON[i] = appointmentSlotToJSON(&created[i])
		}
		out["new_appointments"] = slotJSON
	}
	return c.JSON(out)
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
	manager, _, err := h.groupAccess(c, g)
	if err != nil {
		return err
	}
	avail, err := h.svc.ListSlotsWithAvailability(c.Context(), g)
	if err != nil {
		return responses.InternalError(c, "Could not fetch slots")
	}
	includeFull := c.Query("include_full") == "true" || manager
	items := make([]fiber.Map, 0, len(avail))
	for _, a := range avail {
		if !includeFull && !a.Available {
			continue
		}
		m := slotAvailabilityToJSON(&a)
		if manager && a.ReservationCount > 0 {
			// Instructors see who booked each slot.
			names, _ := h.svc.SlotParticipants(c.Context(), a.Slot.ID)
			list := make([]fiber.Map, 0, len(names))
			for rid, name := range names {
				list = append(list, fiber.Map{"reservation_id": rid, "name": name})
			}
			m["participants"] = list
		}
		items = append(items, m)
	}
	return c.JSON(items)
}

// DeleteSlot removes one slot (and its reservations) from a group.
// DELETE /api/v1/appointment_groups/:id/appointments/:slot_id
func (h *AppointmentGroupHandler) DeleteSlot(c *fiber.Ctx) error {
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
	if err := h.authz.RequireCourseInstructor(c, g.CourseID); err != nil {
		return err
	}
	slot, err := h.svc.GetSlot(c.Context(), uint(slotID), callerAccountID(c))
	if err != nil || slot.GroupID != g.ID {
		return responses.NotFound(c, "slot")
	}
	// Read who held the slot before its reservations go with it.
	live := h.svc.LiveReservations(c.Context(), slot.ID)
	if err := h.svc.DeleteSlot(c.Context(), slot.ID); err != nil {
		return responses.InternalError(c, "Could not delete slot")
	}
	actorID, _ := c.Locals("user_id").(uint)
	h.svc.NotifySlotDeleted(g, slot, live, actorID)
	return c.SendStatus(fiber.StatusNoContent)
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

	// Wave 5.2 FERPA audit: instructor surfacing every student who
	// booked a given slot. Bulk read (studentID=0) keyed on the
	// appointment_slot resource per the convention.
	if callerID, _ := getUserID(c); callerID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, 0, "read", "bulk_appointment_reservations", "appointment_slots", uint(slotID), c.IP(), c.Get("User-Agent"))
	}

	return c.JSON(out)
}

// ListMyReservations returns the caller's active reservations (and their
// student groups') with the slot times and group details, so the
// scheduler and "My appointments" can show and cancel them.
// GET /api/v1/users/self/appointments
func (h *AppointmentGroupHandler) ListMyReservations(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	if h.svc.HasDB() {
		items, err := h.svc.ListMyAppointments(c.Context(), userID, callerAccountID(c))
		if err != nil {
			return responses.InternalError(c, "Could not fetch reservations")
		}
		if items == nil {
			items = []service.MyAppointment{}
		}
		return c.JSON(items)
	}
	items, err := h.svc.ListMyReservations(c.Context(), userID)
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
	manager, ctxs, err := h.groupAccess(c, g)
	if err != nil {
		return err
	}
	userID, _ := c.Locals("user_id").(uint)
	if userID == 0 {
		return responses.Unauthorized(c)
	}
	// Signing up is for students (Canvas participant_for): an observer or
	// designer who can read the group must not take a student's slot.
	if !manager && g.ParticipantType != models.AppointmentParticipantGroup &&
		h.svc.HasDB() && !h.svc.CanParticipate(c.Context(), g, ctxs, userID) {
		return responses.Forbidden(c, "only students can sign up for this appointment group")
	}
	var body struct {
		Comments string `json:"comments"`
	}
	_ = c.BodyParser(&body)

	opts := service.ReserveOptions{Comments: service.SanitizePlainText(body.Comments)}
	if g.ParticipantType == models.AppointmentParticipantGroup && g.GroupCategoryID != nil {
		pg := h.svc.ParticipantGroupFor(c.Context(), *g.GroupCategoryID, userID)
		if pg == nil {
			return responses.Forbidden(c, service.ErrNotInParticipantGroup.Error())
		}
		opts.ParticipantGroupID = pg
	}

	res, err := h.svc.ReserveWith(c.Context(), uint(id), uint(slotID), userID, opts)
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
	h.svc.NotifyReserved(g, res)
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
	// The booker, a member of the student group it was booked for, or an
	// instructor of the course may cancel.
	own := res.UserID == userID ||
		(res.ParticipantGroupID != nil && h.svc.IsGroupMember(c.Context(), *res.ParticipantGroupID, userID))
	if !own {
		if err := h.authz.RequireCourseInstructor(c, g.CourseID); err != nil {
			return err
		}
	}
	wasLive := res.WorkflowState == "reserved"
	if err := h.svc.Cancel(c.Context(), res); err != nil {
		return responses.InternalError(c, "Could not cancel reservation")
	}
	if wasLive {
		// The participant canceling tells the instructors; an instructor
		// canceling tells the participant (Canvas "Appointment Deleted For
		// User"). Queued off the request path.
		h.svc.NotifyCanceled(g, res, userID, own)
	}
	return c.JSON(reservationToJSON(res))
}

// ----- Participants & messaging -----

// ListParticipants lists who may sign up and whether they have.
// GET /api/v1/appointment_groups/:id/users (or /groups)
// ?registration_status=registered|unregistered|all
func (h *AppointmentGroupHandler) ListParticipants(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid appointment group ID")
	}
	g, err := h.svc.GetGroup(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "appointment group")
	}
	manager, ctxs, err := h.groupAccess(c, g)
	if err != nil {
		return err
	}
	if !manager {
		return authzForbidden("instructor access required")
	}
	list, err := h.svc.Participants(c.Context(), g, ctxs, c.Query("registration_status"))
	if err != nil {
		return responses.InternalError(c, "Could not fetch participants")
	}
	if list == nil {
		list = []service.AppointmentParticipant{}
	}
	if callerID, _ := getUserID(c); callerID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, 0, "read", "bulk_appointment_participants", "appointment_groups", g.ID, c.IP(), c.Get("User-Agent"))
	}
	return c.JSON(list)
}

// MessageParticipants sends each participant (every member, for a group
// appointment group) a private message.
// POST /api/v1/appointment_groups/:id/messages
// {subject, body, registration_status: registered|unregistered|all}
func (h *AppointmentGroupHandler) MessageParticipants(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid appointment group ID")
	}
	g, err := h.svc.GetGroup(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "appointment group")
	}
	manager, ctxs, err := h.groupAccess(c, g)
	if err != nil {
		return err
	}
	if !manager {
		return authzForbidden("instructor access required")
	}
	if h.conversations == nil {
		return responses.Error(c, fiber.StatusServiceUnavailable, "messaging is not available")
	}
	var body struct {
		Subject            string `json:"subject"`
		Body               string `json:"body"`
		RegistrationStatus string `json:"registration_status"`
	}
	if err := c.BodyParser(&body); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	body.Subject = strings.TrimSpace(service.SanitizePlainText(body.Subject))
	if body.Subject == "" {
		body.Subject = g.Title
	}
	if strings.TrimSpace(body.Body) == "" {
		return responses.BadRequest(c, "body is required")
	}
	list, err := h.svc.Participants(c.Context(), g, ctxs, body.RegistrationStatus)
	if err != nil {
		return responses.InternalError(c, "Could not fetch participants")
	}
	senderID, _ := c.Locals("user_id").(uint)
	recipients := map[uint]bool{}
	for _, p := range list {
		if len(p.MemberIDs) > 0 {
			for _, m := range p.MemberIDs {
				recipients[m] = true
			}
			continue
		}
		if g.ParticipantType != models.AppointmentParticipantGroup {
			recipients[p.ID] = true
		}
	}
	delete(recipients, senderID)

	sent := 0
	now := time.Now()
	for rid := range recipients {
		conv := &models.Conversation{Subject: body.Subject, CreatedByUserID: senderID, LastMessageAt: now}
		if err := h.conversations.CreateConversation(c.Context(), conv, []uint{rid}); err != nil {
			continue
		}
		msg := &models.ConversationMessage{ConversationID: conv.ID, UserID: senderID, Body: body.Body, CreatedAt: now}
		if err := h.conversations.CreateMessage(c.Context(), msg); err != nil {
			continue
		}
		sent++
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"sent": sent, "recipients": len(recipients)})
}
