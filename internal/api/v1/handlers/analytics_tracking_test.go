package handlers_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

type analyticsTestRig struct {
	app         *fiber.App
	pageViews   *mocks.MockPageViewRepository
	enrollments *mocks.MockEnrollmentRepository
	created     chan *models.PageView
}

func newAnalyticsTestRig(t *testing.T, callerID uint, masquerade bool) *analyticsTestRig {
	t.Helper()
	rig := &analyticsTestRig{
		pageViews:   new(mocks.MockPageViewRepository),
		enrollments: new(mocks.MockEnrollmentRepository),
		created:     make(chan *models.PageView, 10),
	}
	svc := service.NewAnalyticsService(rig.pageViews, new(mocks.MockSubmissionRepository), rig.enrollments, new(mocks.MockAssignmentRepository))
	h := handlers.NewAnalyticsHandler(svc, nil)

	rig.pageViews.On("Create", mock.Anything, mock.AnythingOfType("*models.PageView")).Run(func(args mock.Arguments) {
		rig.created <- args.Get(1).(*models.PageView)
	}).Return(nil).Maybe()
	rig.pageViews.On("TouchEnrollmentActivity", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	app := testutil.SetupTestApp()
	installAuthAndPagination(app, callerID)
	if masquerade {
		app.Use(func(c *fiber.Ctx) error {
			c.Locals("masquerade_by", uint(1))
			return c.Next()
		})
	}
	app.Use(h.ParticipationRecorder())
	app.Post("/api/v1/page_views", h.CreatePageView)
	app.Put("/api/v1/page_views/:id", h.UpdatePageView)
	app.Post("/api/v1/courses/:course_id/discussion_topics/:topic_id/entries", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"ok": true})
	})
	app.Post("/api/v1/courses/:course_id/assignments/:id/fail", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"ok": false})
	})
	rig.app = app
	return rig
}

func (r *analyticsTestRig) waitCreated(t *testing.T) *models.PageView {
	t.Helper()
	select {
	case pv := <-r.created:
		return pv
	case <-time.After(2 * time.Second):
		t.Fatal("expected a page view to be recorded")
		return nil
	}
}

func (r *analyticsTestRig) assertNoneCreated(t *testing.T) {
	t.Helper()
	select {
	case pv := <-r.created:
		t.Fatalf("expected no page view, got %+v", pv)
	case <-time.After(200 * time.Millisecond):
	}
}

// G154: a successful student write on a course route is a participation.
func TestParticipationRecorder_RecordsStudentCourseWrites(t *testing.T) {
	rig := newAnalyticsTestRig(t, 42, false)
	rig.enrollments.On("FindByUserAndCourse", mock.Anything, uint(42), uint(101), uint(1)).Return(&models.Enrollment{UserID: 42, CourseID: 101, Type: "StudentEnrollment"}, nil)

	resp := testutil.MakeRequest(rig.app, http.MethodPost, "/api/v1/courses/101/discussion_topics/7/entries", strings.NewReader(`{}`))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	pv := rig.waitCreated(t)
	assert.True(t, pv.Participated)
	assert.Equal(t, uint(101), pv.ContextID)
	assert.Equal(t, "discussions", pv.AssetCategory)

	// A failed write is not a participation.
	resp = testutil.MakeRequest(rig.app, http.MethodPost, "/api/v1/courses/101/assignments/3/fail", strings.NewReader(`{}`))
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	rig.assertNoneCreated(t)
}

func TestParticipationRecorder_SkipsMasquerade(t *testing.T) {
	rig := newAnalyticsTestRig(t, 42, true)
	resp := testutil.MakeRequest(rig.app, http.MethodPost, "/api/v1/courses/101/discussion_topics/7/entries", strings.NewReader(`{}`))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	rig.assertNoneCreated(t)
}

// G154: POST /page_views keeps only course views of enrolled users and
// answers 204 (nothing recorded) otherwise.
func TestCreatePageView_EnrolledOnly(t *testing.T) {
	rig := newAnalyticsTestRig(t, 42, false)
	rig.enrollments.On("FindByUserAndCourse", mock.Anything, uint(42), uint(101), uint(1)).Return(&models.Enrollment{UserID: 42, CourseID: 101, Type: "StudentEnrollment"}, nil)
	rig.enrollments.On("FindByUserAndCourse", mock.Anything, uint(42), uint(202), uint(1)).Return(nil, gorm.ErrRecordNotFound)

	resp := testutil.MakeRequest(rig.app, http.MethodPost, "/api/v1/page_views",
		strings.NewReader(`{"context_type":"Course","context_id":202,"url":"/courses/202"}`))
	assert.Equal(t, http.StatusNoContent, resp.StatusCode, "another course: not recorded, no existence leak")

	resp = testutil.MakeRequest(rig.app, http.MethodPost, "/api/v1/page_views",
		strings.NewReader(`{"context_type":"Course","context_id":101,"url":"https://evil.example/x"}`))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "url must be a same-site path")

	resp = testutil.MakeRequest(rig.app, http.MethodPost, "/api/v1/page_views",
		strings.NewReader(`{"context_type":"Course","context_id":101,"url":"/courses/101/pages/intro","participated":true}`))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	pv := rig.waitCreated(t)
	assert.False(t, pv.Participated, "participation is never taken from the client")
	assert.Equal(t, "pages", pv.AssetCategory)
}

func TestUpdatePageView_OwnerOnly(t *testing.T) {
	rig := newAnalyticsTestRig(t, 42, false)
	rig.pageViews.On("UpdateInteraction", mock.Anything, uint(5), uint(42), 30).Return(nil)
	rig.pageViews.On("UpdateInteraction", mock.Anything, uint(6), uint(42), 30).Return(gorm.ErrRecordNotFound)

	resp := testutil.MakeRequest(rig.app, http.MethodPut, "/api/v1/page_views/5", strings.NewReader(`{"interaction_seconds":30}`))
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp = testutil.MakeRequest(rig.app, http.MethodPut, "/api/v1/page_views/6", strings.NewReader(`{"interaction_seconds":30}`))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
