package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

// Section-limited enrollments (G107). The course gate stores the user ids
// a section-limited caller may see (middleware.SectionVisibleUsers); the
// helpers below trim course-wide listings to them. A caller who is not
// section-limited (including every admin) is unaffected.

// sectionVisibleUser reports whether the caller may see userID.
func sectionVisibleUser(c *fiber.Ctx, userID uint) bool {
	set, limited := middleware.SectionVisibleUsers(c)
	if !limited {
		return true
	}
	if caller, _ := c.Locals("user_id").(uint); caller == userID {
		return true
	}
	return set[userID]
}

// filterGradebookForSections drops students outside a section-limited
// grader's sections from the gradebook.
func filterGradebookForSections(c *fiber.Ctx, gb *service.GradebookEntry) {
	if gb == nil {
		return
	}
	if _, limited := middleware.SectionVisibleUsers(c); !limited {
		return
	}
	students := gb.Students[:0]
	for _, s := range gb.Students {
		if sectionVisibleUser(c, s.ID) {
			students = append(students, s)
		}
	}
	gb.Students = students
	for key := range gb.Submissions {
		id, err := strconv.ParseUint(key, 10, 64)
		if err != nil || !sectionVisibleUser(c, uint(id)) {
			delete(gb.Submissions, key)
		}
	}
}

// clampPostSections narrows a post / hide grades request to a
// section-limited caller's sections: no section_ids means "my sections"
// (not "every section"), and naming a section outside theirs is refused
// (ok=false; answer 404). Callers who are not section-limited pass
// through unchanged.
func clampPostSections(c *fiber.Ctx, ids []uint) ([]uint, bool) {
	visible, limited := middleware.VisibleSectionIDs(c)
	if !limited {
		return ids, true
	}
	if len(visible) == 0 {
		return nil, false
	}
	if len(ids) == 0 {
		return append([]uint(nil), visible...), true
	}
	allowed := make(map[uint]bool, len(visible))
	for _, v := range visible {
		allowed[v] = true
	}
	for _, id := range ids {
		if !allowed[id] {
			return nil, false
		}
	}
	return ids, true
}

// filterSubmissionsForSections keeps only the submissions of users a
// section-limited caller may see.
func filterSubmissionsForSections(c *fiber.Ctx, subs []models.Submission) []models.Submission {
	if _, limited := middleware.SectionVisibleUsers(c); !limited {
		return subs
	}
	out := subs[:0]
	for _, s := range subs {
		if sectionVisibleUser(c, s.UserID) {
			out = append(out, s)
		}
	}
	return out
}
