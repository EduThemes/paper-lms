package middleware

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// Course access resolution shared by the course gates (RequireCourseRole /
// RequireEnrolled) and handlers.ResourceAuthorizer. It turns a user's
// enrollments in a course into one decision (models.ResolveCourseAccess):
//
//   - G106 date-based access: enrollment / section / course / term dates
//     and a concluded course make an enrollment read-only (or, for a
//     future start or a course that restricts past/future viewing, give
//     no access).
//   - G110 multiple enrollments: permissions are the union of every
//     usable enrollment.
//   - G107 section-limited enrollments: the visible sections are stored
//     in Locals for handlers to filter rosters, gradebooks and
//     submission lists.

// Locals keys written by StoreCourseAccess.
const (
	localEnrollmentReadOnly = "enrollment_read_only"
	localVisibleSectionIDs  = "visible_section_ids"
	localEnrollmentTypes    = "enrollment_types"
	localVisibleUserIDs     = "visible_user_ids"
)

// nowFunc is swapped by tests.
var nowFunc = time.Now

// ResolveCourseEnrollment loads the user's enrollments in the course and
// resolves the request's access. It returns a non-empty denial message
// when the caller has no usable enrollment of an allowed type. course may
// be nil (date settings then come from the enrollment, section and term
// only).
//
// When repo implements repository.EnrollmentAccessReader (the Postgres
// repository does) every live enrollment, its section and the term are
// considered. Test doubles that only implement EnrollmentRepository fall
// back to the single most privileged active enrollment.
func ResolveCourseEnrollment(c *fiber.Ctx, repo repository.EnrollmentRepository, userID, courseID, accountID uint, course *models.Course, roleSet map[string]bool) (models.CourseAccessDecision, string) {
	var data *models.CourseAccessData
	if reader, ok := repo.(repository.EnrollmentAccessReader); ok {
		loaded, err := reader.LoadCourseAccess(c.Context(), userID, courseID, accountID)
		if err != nil {
			return models.CourseAccessDecision{}, "user is not enrolled in this course"
		}
		data = loaded
	} else {
		e, err := repo.FindByUserAndCourse(c.Context(), userID, courseID, accountID)
		if err != nil || e == nil {
			return models.CourseAccessDecision{}, "user is not enrolled in this course"
		}
		data = &models.CourseAccessData{Enrollments: []models.Enrollment{*e}}
	}
	if len(data.Enrollments) == 0 {
		return models.CourseAccessDecision{}, "user is not enrolled in this course"
	}

	dec := models.ResolveCourseAccess(data, course, roleSet, nowFunc())
	if dec.Enrollment != nil {
		return dec, ""
	}
	if dec.RoleMismatch {
		return dec, "insufficient permissions for this action"
	}
	switch dec.State {
	case models.EffectivePendingActive, models.EffectivePendingInvited:
		return dec, "this course has not started yet"
	case models.EffectiveCompleted:
		return dec, "your enrollment in this course has concluded"
	case "":
		return dec, "insufficient permissions for this action"
	}
	return dec, "enrollment is not active"
}

// safeMethod reports whether the request only reads.
func safeMethod(c *fiber.Ctx) bool {
	switch c.Method() {
	case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions:
		return true
	}
	return false
}

// ReadOnlyDenial returns the 403 message for a write made under a
// read-only (concluded or not-yet-started) enrollment, or "".
func ReadOnlyDenial(c *fiber.Ctx, dec models.CourseAccessDecision) string {
	return readOnlyDenial(c, dec)
}

func readOnlyDenial(c *fiber.Ctx, dec models.CourseAccessDecision) string {
	if dec.Access != models.EnrollmentAccessReadOnly || safeMethod(c) {
		return ""
	}
	if dec.State == models.EffectivePendingActive {
		return "this course has not started yet; you can view it but not participate"
	}
	return "your enrollment in this course has concluded; the course is read-only"
}

// StoreCourseAccess records the decision in Locals for handlers.
func StoreCourseAccess(c *fiber.Ctx, dec models.CourseAccessDecision) {
	if dec.Enrollment == nil {
		return
	}
	c.Locals("enrollment_type", dec.Enrollment.Type)
	c.Locals("enrollment_id", dec.Enrollment.ID)
	c.Locals(localEnrollmentReadOnly, dec.Access == models.EnrollmentAccessReadOnly)
	c.Locals(localEnrollmentTypes, dec.UsableTypes)
	if dec.SectionLimited {
		c.Locals(localVisibleSectionIDs, dec.VisibleSectionIDs)
	}
}

// VisibleSectionIDs returns the sections a section-limited caller may
// see (0 = the course's default, section-less enrollments) and true, or
// nil and false when the caller is not section-limited (admins never are).
func VisibleSectionIDs(c *fiber.Ctx) ([]uint, bool) {
	ids, ok := c.Locals(localVisibleSectionIDs).([]uint)
	if !ok {
		return nil, false
	}
	return ids, true
}

// EnrollmentReadOnly reports whether the request acts under a read-only
// (concluded / not yet started) enrollment.
func EnrollmentReadOnly(c *fiber.Ctx) bool {
	ro, _ := c.Locals(localEnrollmentReadOnly).(bool)
	return ro
}

// storeVisibleUsers resolves the user ids a section-limited caller may
// see (everyone with an enrollment in one of their sections) for the
// handlers that list people, submissions and grades.
func (pm *PermissionMiddleware) storeVisibleUsers(c *fiber.Ctx, dec models.CourseAccessDecision, courseID uint) error {
	reader, ok := pm.enrollmentRepo.(repository.EnrollmentAccessReader)
	if !ok {
		return nil
	}
	ids, err := reader.UserIDsInSections(c.Context(), courseID, dec.VisibleSectionIDs)
	if err != nil {
		return err
	}
	set := make(map[uint]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	c.Locals(localVisibleUserIDs, set)
	return nil
}

// SectionVisibleUsers returns the set of user ids a section-limited
// caller may see and true, or nil and false when the caller is not
// section-limited.
func SectionVisibleUsers(c *fiber.Ctx) (map[uint]bool, bool) {
	set, ok := c.Locals(localVisibleUserIDs).(map[uint]bool)
	return set, ok
}

// sectionLimitedTargetDenied refuses per-user course routes
// (/:user_id, /:student_id, ?user_id=) that name a user outside a
// section-limited caller's sections. A miss is a 404 so the route can't
// be used to probe other sections' rosters.
func (pm *PermissionMiddleware) sectionLimitedTargetDenied(c *fiber.Ctx, dec models.CourseAccessDecision, callerID, courseID uint) (bool, error) {
	reader, ok := pm.enrollmentRepo.(repository.EnrollmentAccessReader)
	if !ok {
		return false, nil
	}
	for _, raw := range []string{c.Params("user_id"), c.Params("student_id"), c.Query("user_id")} {
		if raw == "" || raw == "self" {
			continue
		}
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || n == 0 || uint(n) == callerID {
			continue
		}
		in, err := reader.UserInCourseSections(c.Context(), uint(n), courseID, dec.VisibleSectionIDs)
		if err != nil {
			return true, err
		}
		if !in {
			return true, nil
		}
	}
	return false, nil
}
