package handlers

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/auth"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

// StudentViewHandler implements Canvas Student View (G076):
//
//	POST   /courses/:course_id/student_view   enter (course staff)
//	DELETE /courses/:course_id/student_view   reset the test student
//	GET    /users/self/student_view            is this session Student View?
//
// Entering switches the session to the course's test student with a
// masquerade token (masquerade_by = the real user), so leaving is the
// usual DELETE /masquerade. The test student is a plain StudentEnrollment
// in one course, and EffectiveRole caps the session at the real user's
// role, so Student View can never grant more than the teacher has.
type StudentViewHandler struct {
	svc        *service.StudentViewService
	courseRepo repository.CourseRepository
	users      *UserHandler
}

func NewStudentViewHandler(svc *service.StudentViewService, courseRepo repository.CourseRepository, users *UserHandler) *StudentViewHandler {
	return &StudentViewHandler{svc: svc, courseRepo: courseRepo, users: users}
}

func studentViewUserJSON(u *models.User) fiber.Map {
	return fiber.Map{
		"id":            u.ID,
		"name":          u.Name,
		"sortable_name": u.SortableName,
		"short_name":    u.ShortName,
		"role":          u.Role,
		"account_id":    u.AccountID,
	}
}

// startSession issues the Student View session cookie for testUser on
// behalf of realUserID / realAccountID.
func (h *StudentViewHandler) startSession(c *fiber.Ctx, testUser *models.User, realUserID, realAccountID uint) error {
	token, err := auth.GenerateMasqueradeToken(testUser, realUserID, realAccountID, h.users.jwtSecret)
	if err != nil {
		return err
	}
	h.users.setAuthCookie(c, token)
	return nil
}

func (h *StudentViewHandler) audit(c *fiber.Ctx, event string, realUserID, courseID, accountID, testUserID uint) {
	if h.users.auditService == nil {
		return
	}
	payload, _ := json.Marshal(fiber.Map{"real_user_id": realUserID, "test_student_id": testUserID, "course_id": courseID})
	cid := courseID
	_ = h.users.auditService.LogEvent(c.Context(), event, realUserID, &cid, auditAccountPtr(accountID),
		"Course", courseID, fmt.Sprintf("User %d %s for course %d", realUserID, event, courseID),
		string(payload), c.IP(), c.Get("User-Agent"))
}

// inStudentView returns the course whose Student View this session is,
// or 0.
func (h *StudentViewHandler) inStudentView(c *fiber.Ctx) uint {
	realID, _ := c.Locals("masquerade_by").(uint)
	userID, _ := c.Locals("user_id").(uint)
	if realID == 0 || userID == 0 {
		return 0
	}
	courseID, err := h.svc.TestStudentCourse(c.Context(), userID)
	if err != nil {
		return 0
	}
	return courseID
}

// Enter handles POST /courses/:course_id/student_view (course staff).
func (h *StudentViewHandler) Enter(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	if realID, _ := c.Locals("masquerade_by").(uint); realID != 0 {
		return responses.BadRequest(c, "Leave the current masquerade or Student View first")
	}
	if !canViewUnpublishedContent(c) {
		return responses.Forbidden(c, "Only course staff can use Student View")
	}

	course, err := h.courseRepo.FindByID(c.Context(), uint(courseID), callerAccountID(c))
	if err != nil || course == nil {
		return responses.NotFound(c, "course")
	}
	testUser, err := h.svc.Ensure(c.Context(), course)
	if err != nil {
		return responses.InternalError(c, "Could not prepare the test student")
	}
	if err := h.startSession(c, testUser, userID, callerAccountID(c)); err != nil {
		return responses.InternalError(c, "Could not start Student View")
	}
	h.audit(c, "student_view_start", userID, course.ID, callerAccountID(c), testUser.ID)

	return c.JSON(fiber.Map{"student_view": true, "course_id": course.ID, "user": studentViewUserJSON(testUser)})
}

// Reset handles DELETE /courses/:course_id/student_view. Course staff
// reset from outside Student View; from inside it, the test student is
// replaced by a fresh one and the session moves to it.
func (h *StudentViewHandler) Reset(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	viewing := h.inStudentView(c)
	realID, _ := c.Locals("masquerade_by").(uint)
	switch {
	case viewing == uint(courseID):
		// Inside Student View: the real user is acting.
	case realID == 0 && canViewUnpublishedContent(c):
		// Course staff outside Student View.
	default:
		return responses.Forbidden(c, "Only course staff can reset the test student")
	}

	course, err := h.courseRepo.FindByID(c.Context(), uint(courseID), callerAccountID(c))
	if err != nil || course == nil {
		return responses.NotFound(c, "course")
	}
	if err := h.svc.Reset(c.Context(), course.ID); err != nil {
		return responses.InternalError(c, "Could not reset the test student")
	}

	actor := userID
	if viewing == 0 {
		h.audit(c, "student_view_reset", actor, course.ID, callerAccountID(c), 0)
		return c.JSON(fiber.Map{"reset": true, "student_view": false})
	}

	// In Student View: continue as a fresh test student and retire the
	// old session token.
	realAccount, _ := c.Locals("admin_account_id").(uint)
	fresh, err := h.svc.Ensure(c.Context(), course)
	if err != nil {
		return responses.InternalError(c, "Could not prepare the test student")
	}
	if h.users.tokenBlacklist != nil {
		if old := c.Cookies("paper_session"); old != "" {
			h.users.tokenBlacklist.Revoke(old, time.Now().Add(24*time.Hour))
		}
	}
	if err := h.startSession(c, fresh, realID, realAccount); err != nil {
		return responses.InternalError(c, "Could not restart Student View")
	}
	h.audit(c, "student_view_reset", realID, course.ID, realAccount, fresh.ID)
	return c.JSON(fiber.Map{"reset": true, "student_view": true, "course_id": course.ID, "user": studentViewUserJSON(fresh)})
}

// Status handles GET /users/self/student_view: {course_id} while the
// session is Student View, else {course_id: null}.
func (h *StudentViewHandler) Status(c *fiber.Ctx) error {
	if courseID := h.inStudentView(c); courseID != 0 {
		return c.JSON(fiber.Map{"student_view": true, "course_id": courseID})
	}
	return c.JSON(fiber.Map{"student_view": false, "course_id": nil})
}

// IsTestStudentOf reports whether userID is courseID's test student. The
// course gates use it to let Student View into an unpublished course,
// which Canvas allows so teachers can preview before publishing.
func (h *StudentViewHandler) IsTestStudentOf(c *fiber.Ctx, userID, courseID uint) bool {
	got, err := h.svc.TestStudentCourse(c.Context(), userID)
	return err == nil && got == courseID
}
