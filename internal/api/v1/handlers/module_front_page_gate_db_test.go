package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

// The front page route names no page, but when the front page sits in a
// locked module it is as locked as through /pages/:url.
func TestModuleContentGate_FrontPage(t *testing.T) {
	f := newMPFixture(t)
	g := f.g
	cid := f.course.ID
	front := &models.WikiPage{CourseID: cid, Title: "Week 2 home", URL: "week-2-home", WorkflowState: "active", FrontPage: true}
	require.NoError(t, g.Create(front).Error)
	mpItem(t, g, f.m2.ID, 4, "WikiPage", front.ID, "", "week-2-home")

	ok := func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) }
	path := fmt.Sprintf("/courses/%d/front_page", cid)
	get := func(userID uint, enrollment, p string) int {
		app := f.app(userID, 1, enrollment)
		app.Get("/courses/:course_id/front_page", ok)
		return testutil.MakeRequest(app, http.MethodGet, p, nil).StatusCode
	}
	// Week 2 is behind Week 1's requirements.
	assert.Equal(t, http.StatusForbidden, get(f.student, "StudentEnrollment", path))
	assert.Equal(t, http.StatusForbidden, get(f.student, "StudentEnrollment", fmt.Sprintf("/Courses/%d/Front_Page/", cid)), "path variants are gated too")
	assert.Equal(t, http.StatusOK, get(f.teacher, "TeacherEnrollment", path), "staff are not module-locked")

	// A front page outside any module is never module-locked.
	require.NoError(t, g.Model(&models.ContentTag{}).Where("content_type = ? AND content_id = ?", "WikiPage", front.ID).Update("workflow_state", "deleted").Error)
	assert.Equal(t, http.StatusOK, get(f.student, "StudentEnrollment", path))
}
