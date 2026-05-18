package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// errFakeNotFound is the sentinel returned by every in-test fake repo in
// this file's FindByID / FindByX methods when the requested row was not
// seeded. The handler layer translates this to 404 — exact value doesn't
// matter as long as it's non-nil.
var errFakeNotFound = errors.New("fake repo: not found")

// Wave 5.2 / Phase 13.5 follow-up: LogPIIAccess sweep across student-keyed
// read handlers. Every test below locks one (accessor, studentID, dataField,
// resource) tuple — regressions here re-open the "LogPIIAccess defined,
// never called" audit finding from May 2026.
//
// Per the project memory preference for in-test fakes over testify Mock
// for surfaces that are shaped by a single assertion (a single Create call
// on the PIIAccessLog repo), the fake `piiSink` below records every Create
// call so tests inspect it inline. Heavy repository interfaces still use
// the existing testify mocks in internal/testutil/mocks.

// ----- piiSink: in-test PIIAccessLogRepository fake -----

// piiSink captures every PII access log Create call so tests can assert
// the (accessor, student, data field, resource) tuple emitted by a
// handler. It satisfies postgres.PIIAccessLogRepository.
type piiSink struct {
	mu      sync.Mutex
	entries []models.PIIAccessLog
}

func newPIISink() *piiSink {
	return &piiSink{}
}

func (p *piiSink) Create(ctx context.Context, log *models.PIIAccessLog) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if log != nil {
		p.entries = append(p.entries, *log)
	}
	return nil
}

func (p *piiSink) ListByStudentID(ctx context.Context, studentID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PIIAccessLog], error) {
	return &repository.PaginatedResult[models.PIIAccessLog]{}, nil
}

func (p *piiSink) ListByAccessorID(ctx context.Context, accessorID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PIIAccessLog], error) {
	return &repository.PaginatedResult[models.PIIAccessLog]{}, nil
}

func (p *piiSink) snapshot() []models.PIIAccessLog {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]models.PIIAccessLog, len(p.entries))
	copy(out, p.entries)
	return out
}

// assertOneEntry is the assertion helper every test in this file uses:
// exactly one PII access log row was emitted, with the expected tuple.
func assertOneEntry(t *testing.T, sink *piiSink, accessor, student uint, dataField, resource string) {
	t.Helper()
	entries := sink.snapshot()
	require.Len(t, entries, 1, "expected exactly one PII access log row")
	e := entries[0]
	assert.Equal(t, accessor, e.AccessorID, "accessor mismatch")
	assert.Equal(t, student, e.StudentID, "student mismatch")
	assert.Equal(t, "read", e.AccessType, "access type mismatch")
	assert.Equal(t, dataField, e.DataField, "data field mismatch")
	assert.Equal(t, resource, e.Resource, "resource mismatch")
}

// ----- helpers shared across tests -----

// installAuthAndPagination wires the user/account locals + pagination
// middleware that every handler in this file expects.
func installAuthAndPagination(app *fiber.App, callerID uint) {
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", callerID)
		c.Locals("account_id", uint(1))
		return c.Next()
	})
	app.Use(middleware.PaginationParams())
}

// ===========================================================================
// accommodations.go
// ===========================================================================

// fakeAccommodationRepo: in-test implementation of
// postgres.StudentAccommodationRepository. Each test seeds the fake with
// the rows it needs and asserts on the recorded handler audit row.
type fakeAccommodationRepo struct {
	byID  map[uint]*models.StudentAccommodation
	byUID map[uint][]models.StudentAccommodation
}

func newFakeAccommodationRepo() *fakeAccommodationRepo {
	return &fakeAccommodationRepo{
		byID:  map[uint]*models.StudentAccommodation{},
		byUID: map[uint][]models.StudentAccommodation{},
	}
}

func (f *fakeAccommodationRepo) Create(ctx context.Context, a *models.StudentAccommodation) error {
	f.byID[a.ID] = a
	return nil
}
func (f *fakeAccommodationRepo) FindByID(ctx context.Context, id uint) (*models.StudentAccommodation, error) {
	a, ok := f.byID[id]
	if !ok {
		return nil, errFakeNotFound
	}
	return a, nil
}
func (f *fakeAccommodationRepo) Update(ctx context.Context, a *models.StudentAccommodation) error {
	f.byID[a.ID] = a
	return nil
}
func (f *fakeAccommodationRepo) Delete(ctx context.Context, id uint) error {
	delete(f.byID, id)
	return nil
}
func (f *fakeAccommodationRepo) ListByUserID(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.StudentAccommodation], error) {
	items := f.byUID[userID]
	if userID == 0 {
		// Bulk list returns everything (matches the live repo's behavior
		// for the ListCourseAccommodations entrypoint).
		for _, v := range f.byID {
			items = append(items, *v)
		}
	}
	return &repository.PaginatedResult[models.StudentAccommodation]{
		Items:      items,
		TotalCount: int64(len(items)),
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
func (f *fakeAccommodationRepo) ListByUserAndCourse(ctx context.Context, userID, courseID uint) ([]models.StudentAccommodation, error) {
	return nil, nil
}
func (f *fakeAccommodationRepo) ListActiveByUserID(ctx context.Context, userID uint) ([]models.StudentAccommodation, error) {
	return nil, nil
}

type fakeAccommodationAppRepo struct{}

func (f *fakeAccommodationAppRepo) Create(ctx context.Context, a *models.AccommodationApplication) error {
	return nil
}
func (f *fakeAccommodationAppRepo) FindByResourceAndUser(ctx context.Context, resourceType string, resourceID, userID uint) (*models.AccommodationApplication, error) {
	return nil, errFakeNotFound
}
func (f *fakeAccommodationAppRepo) ListByAccommodationID(ctx context.Context, accommodationID uint) ([]models.AccommodationApplication, error) {
	return nil, nil
}

// satisfy postgres interfaces at compile time
var _ postgres.StudentAccommodationRepository = (*fakeAccommodationRepo)(nil)
var _ postgres.AccommodationApplicationRepository = (*fakeAccommodationAppRepo)(nil)

func TestListUserAccommodations_FiresLogPIIAccess(t *testing.T) {
	accomRepo := newFakeAccommodationRepo()
	studentID := uint(42)
	accomRepo.byUID[studentID] = []models.StudentAccommodation{
		{ID: 1, UserID: studentID, AccommodationType: "extended_time"},
	}

	sink := newPIISink()
	accomService := service.NewAccommodationService(accomRepo, &fakeAccommodationAppRepo{})
	assignmentSvc := service.NewAssignmentService(nil)
	authz := handlers.NewResourceAuthorizer(new(mocks.MockEnrollmentRepository), new(mocks.MockUserRepository))
	auditService := service.NewAuditService(nil, nil, sink)
	h := handlers.NewAccommodationHandler(accomService, assignmentSvc, authz, auditService)

	app := testutil.SetupTestApp()
	installAuthAndPagination(app, 7)
	app.Get("/api/v1/users/:user_id/accommodations", h.ListUserAccommodations)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/users/42/accommodations", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assertOneEntry(t, sink, 7, 42, "accommodations_list", "student_accommodations")
}

func TestListUserAccommodations_SelfReadDoesNotLog(t *testing.T) {
	// Self-read (student viewing own accommodations) is not a FERPA
	// disclosure — the handler skips LogPIIAccess in that case.
	accomRepo := newFakeAccommodationRepo()
	accomRepo.byUID[7] = []models.StudentAccommodation{
		{ID: 1, UserID: 7, AccommodationType: "extended_time"},
	}
	sink := newPIISink()
	accomService := service.NewAccommodationService(accomRepo, &fakeAccommodationAppRepo{})
	assignmentSvc := service.NewAssignmentService(nil)
	authz := handlers.NewResourceAuthorizer(new(mocks.MockEnrollmentRepository), new(mocks.MockUserRepository))
	auditService := service.NewAuditService(nil, nil, sink)
	h := handlers.NewAccommodationHandler(accomService, assignmentSvc, authz, auditService)

	app := testutil.SetupTestApp()
	installAuthAndPagination(app, 7) // caller is the student
	app.Get("/api/v1/users/:user_id/accommodations", h.ListUserAccommodations)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/users/7/accommodations", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Empty(t, sink.snapshot(), "self-read should not emit a PII access log row")
}

func TestGetAccommodation_FiresLogPIIAccess(t *testing.T) {
	accomRepo := newFakeAccommodationRepo()
	studentID := uint(42)
	accomRepo.byID[99] = &models.StudentAccommodation{
		ID:                99,
		UserID:            studentID,
		AccommodationType: "extended_time",
	}

	sink := newPIISink()
	accomService := service.NewAccommodationService(accomRepo, &fakeAccommodationAppRepo{})
	assignmentSvc := service.NewAssignmentService(nil)
	userMock := new(mocks.MockUserRepository)
	// caller 7 is an admin so RequireOwnerOrAdmin passes (caller != student).
	userMock.On("FindByID", mock.Anything, uint(7)).Return(&models.User{ID: 7, Role: "admin"}, nil)
	authz := handlers.NewResourceAuthorizer(new(mocks.MockEnrollmentRepository), userMock)
	auditService := service.NewAuditService(nil, nil, sink)
	h := handlers.NewAccommodationHandler(accomService, assignmentSvc, authz, auditService)

	app := testutil.SetupTestApp()
	installAuthAndPagination(app, 7)
	app.Get("/api/v1/accommodations/:id", h.GetAccommodation)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/accommodations/99", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assertOneEntry(t, sink, 7, 42, "accommodation_detail", "student_accommodations")
}

func TestListCourseAccommodations_FiresBulkLogPIIAccess(t *testing.T) {
	accomRepo := newFakeAccommodationRepo()
	courseID := uint(101)
	accomRepo.byID[1] = &models.StudentAccommodation{ID: 1, UserID: 42, CourseID: &courseID, AccommodationType: "extended_time"}
	accomRepo.byID[2] = &models.StudentAccommodation{ID: 2, UserID: 88, CourseID: &courseID, AccommodationType: "modified_due_dates"}

	sink := newPIISink()
	accomService := service.NewAccommodationService(accomRepo, &fakeAccommodationAppRepo{})
	assignmentSvc := service.NewAssignmentService(nil)
	authz := handlers.NewResourceAuthorizer(new(mocks.MockEnrollmentRepository), new(mocks.MockUserRepository))
	auditService := service.NewAuditService(nil, nil, sink)
	h := handlers.NewAccommodationHandler(accomService, assignmentSvc, authz, auditService)

	app := testutil.SetupTestApp()
	installAuthAndPagination(app, 7)
	app.Get("/api/v1/courses/:course_id/accommodations", h.ListCourseAccommodations)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/courses/101/accommodations", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	// Bulk read: studentID == 0 per the convention, resource is "courses"
	// keyed by course_id.
	assertOneEntry(t, sink, 7, 0, "bulk_accommodations_read", "courses")
	assert.Equal(t, uint(101), sink.snapshot()[0].ResourceID)
}

// ===========================================================================
// announcements.go — GetReadReceipts
// ===========================================================================

// fakeReceiptRepo: in-test AnnouncementReadReceiptRepository.
type fakeReceiptRepo struct {
	byAnn map[uint][]models.AnnouncementReadReceipt
}

func newFakeReceiptRepo() *fakeReceiptRepo {
	return &fakeReceiptRepo{byAnn: map[uint][]models.AnnouncementReadReceipt{}}
}

func (f *fakeReceiptRepo) Create(ctx context.Context, r *models.AnnouncementReadReceipt) error {
	return nil
}
func (f *fakeReceiptRepo) FindByAnnouncementAndUser(ctx context.Context, announcementID, userID uint) (*models.AnnouncementReadReceipt, error) {
	return nil, errFakeNotFound
}
func (f *fakeReceiptRepo) FindByAnnouncementIDsAndUser(ctx context.Context, ids []uint, userID uint) ([]models.AnnouncementReadReceipt, error) {
	return nil, nil
}
func (f *fakeReceiptRepo) ListByAnnouncementID(ctx context.Context, announcementID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AnnouncementReadReceipt], error) {
	items := f.byAnn[announcementID]
	return &repository.PaginatedResult[models.AnnouncementReadReceipt]{
		Items:      items,
		TotalCount: int64(len(items)),
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
func (f *fakeReceiptRepo) CountReadByAnnouncementID(ctx context.Context, announcementID uint) (int64, error) {
	return int64(len(f.byAnn[announcementID])), nil
}
func (f *fakeReceiptRepo) CountAcknowledgedByAnnouncementID(ctx context.Context, announcementID uint) (int64, error) {
	return 0, nil
}
func (f *fakeReceiptRepo) MarkRead(ctx context.Context, announcementID, userID uint) error {
	return nil
}
func (f *fakeReceiptRepo) MarkAcknowledged(ctx context.Context, announcementID, userID uint) error {
	return nil
}
func (f *fakeReceiptRepo) BulkMarkRead(ctx context.Context, announcementID uint, userIDs []uint) error {
	return nil
}

// fakeAnnouncementRepo: minimal in-test AnnouncementRepository for the
// receipts test (only FindByID is exercised by GetAnnouncementStats).
type fakeAnnouncementRepo struct {
	byID map[uint]*models.Announcement
}

func newFakeAnnouncementRepo() *fakeAnnouncementRepo {
	return &fakeAnnouncementRepo{byID: map[uint]*models.Announcement{}}
}

func (f *fakeAnnouncementRepo) Create(ctx context.Context, a *models.Announcement) error {
	f.byID[a.ID] = a
	return nil
}
func (f *fakeAnnouncementRepo) FindByID(ctx context.Context, id uint) (*models.Announcement, error) {
	a, ok := f.byID[id]
	if !ok {
		return nil, errFakeNotFound
	}
	return a, nil
}
func (f *fakeAnnouncementRepo) Update(ctx context.Context, a *models.Announcement) error {
	f.byID[a.ID] = a
	return nil
}
func (f *fakeAnnouncementRepo) Delete(ctx context.Context, id uint) error {
	delete(f.byID, id)
	return nil
}
func (f *fakeAnnouncementRepo) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Announcement], error) {
	return &repository.PaginatedResult[models.Announcement]{}, nil
}
func (f *fakeAnnouncementRepo) ListByAccountID(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Announcement], error) {
	return &repository.PaginatedResult[models.Announcement]{}, nil
}
func (f *fakeAnnouncementRepo) ListGlobal(ctx context.Context, params repository.PaginationParams) (*repository.PaginatedResult[models.Announcement], error) {
	return &repository.PaginatedResult[models.Announcement]{}, nil
}
func (f *fakeAnnouncementRepo) ListScheduledReady(ctx context.Context) ([]models.Announcement, error) {
	return nil, nil
}

var _ postgres.AnnouncementRepository = (*fakeAnnouncementRepo)(nil)
var _ postgres.AnnouncementReadReceiptRepository = (*fakeReceiptRepo)(nil)

func TestGetReadReceipts_FiresLogPIIAccess(t *testing.T) {
	annRepo := newFakeAnnouncementRepo()
	annRepo.byID[10] = &models.Announcement{ID: 10, Title: "t", Message: "m"}

	receiptRepo := newFakeReceiptRepo()
	receiptRepo.byAnn[10] = []models.AnnouncementReadReceipt{
		{ID: 1, AnnouncementID: 10, UserID: 50},
		{ID: 2, AnnouncementID: 10, UserID: 51},
	}

	sink := newPIISink()
	announcementService := service.NewAnnouncementService(annRepo, receiptRepo, new(mocks.MockEnrollmentRepository))
	authz := handlers.NewResourceAuthorizer(new(mocks.MockEnrollmentRepository), new(mocks.MockUserRepository))
	auditService := service.NewAuditService(nil, nil, sink)
	h := handlers.NewAnnouncementHandler(announcementService, authz, auditService)

	app := testutil.SetupTestApp()
	installAuthAndPagination(app, 7)
	app.Get("/api/v1/announcements/:id/read_receipts", h.GetReadReceipts)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/announcements/10/read_receipts", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	// Bulk read across N students; studentID=0, resource keyed on the
	// announcement.
	assertOneEntry(t, sink, 7, 0, "announcement_read_receipts", "announcements")
	assert.Equal(t, uint(10), sink.snapshot()[0].ResourceID)
}

// ===========================================================================
// appointment_groups.go — ListReservations
// ===========================================================================

func TestListReservations_FiresBulkLogPIIAccess(t *testing.T) {
	groupRepo := new(mocks.MockAppointmentGroupRepository)
	slotRepo := new(mocks.MockAppointmentSlotRepository)
	resRepo := new(mocks.MockAppointmentReservationRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)
	userRepo := new(mocks.MockUserRepository)

	groupRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(&models.AppointmentGroup{ID: 5, CourseID: 101}, nil)
	slotRepo.On("FindByID", mock.Anything, uint(20), uint(1)).Return(&models.AppointmentSlot{ID: 20, GroupID: 5}, nil)
	resRepo.On("ListBySlot", mock.Anything, uint(20)).Return([]models.AppointmentReservation{
		{ID: 1, SlotID: 20, GroupID: 5, UserID: 42},
		{ID: 2, SlotID: 20, GroupID: 5, UserID: 88},
	}, nil)

	// Caller is an admin so RequireCourseInstructor passes without an
	// enrollment lookup.
	userRepo.On("FindByID", mock.Anything, uint(7)).Return(&models.User{ID: 7, Role: "admin"}, nil)

	appointmentService := service.NewAppointmentGroupService(groupRepo, slotRepo, resRepo, nil)
	authz := handlers.NewResourceAuthorizer(enrollmentRepo, userRepo)
	sink := newPIISink()
	auditService := service.NewAuditService(nil, nil, sink)
	h := handlers.NewAppointmentGroupHandler(appointmentService, authz, auditService)

	app := testutil.SetupTestApp()
	installAuthAndPagination(app, 7)
	app.Get("/api/v1/appointment_groups/:id/slots/:slot_id/reservations", h.ListReservations)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/appointment_groups/5/slots/20/reservations", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assertOneEntry(t, sink, 7, 0, "bulk_appointment_reservations", "appointment_slots")
	assert.Equal(t, uint(20), sink.snapshot()[0].ResourceID)
}

// ===========================================================================
// learning_outcomes.go — ListResults (per-user) + GetMasteryGradebook (bulk)
// ===========================================================================

// fakeLearningOutcomeResultRepo: minimal in-test
// LearningOutcomeResultRepository for the two student-keyed read paths.
type fakeLearningOutcomeResultRepo struct {
	byOutcome map[uint][]models.LearningOutcomeResult
	byUser    map[uint][]models.LearningOutcomeResult
}

func newFakeLearningOutcomeResultRepo() *fakeLearningOutcomeResultRepo {
	return &fakeLearningOutcomeResultRepo{
		byOutcome: map[uint][]models.LearningOutcomeResult{},
		byUser:    map[uint][]models.LearningOutcomeResult{},
	}
}

func (f *fakeLearningOutcomeResultRepo) Create(ctx context.Context, r *models.LearningOutcomeResult) error {
	return nil
}
func (f *fakeLearningOutcomeResultRepo) FindByID(ctx context.Context, id uint) (*models.LearningOutcomeResult, error) {
	return nil, errFakeNotFound
}
func (f *fakeLearningOutcomeResultRepo) Update(ctx context.Context, r *models.LearningOutcomeResult) error {
	return nil
}
func (f *fakeLearningOutcomeResultRepo) Upsert(ctx context.Context, r *models.LearningOutcomeResult) (*bool, error) {
	return nil, nil
}
func (f *fakeLearningOutcomeResultRepo) ListByOutcomeID(ctx context.Context, outcomeID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LearningOutcomeResult], error) {
	items := f.byOutcome[outcomeID]
	return &repository.PaginatedResult[models.LearningOutcomeResult]{
		Items:      items,
		TotalCount: int64(len(items)),
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
func (f *fakeLearningOutcomeResultRepo) ListByUserAndContext(ctx context.Context, userID uint, contextType string, contextID uint) ([]models.LearningOutcomeResult, error) {
	return f.byUser[userID], nil
}
func (f *fakeLearningOutcomeResultRepo) ListByUserAndOutcomeIDs(ctx context.Context, userID uint, outcomeIDs []uint) ([]models.LearningOutcomeResult, error) {
	return nil, nil
}

// fakeLearningOutcomeRepo: minimal stub. We only need ListByContext to
// return a paginated list for GetMasteryGradebook.
type fakeLearningOutcomeRepo struct {
	byCourse map[uint][]models.LearningOutcome
}

func (f *fakeLearningOutcomeRepo) Create(ctx context.Context, o *models.LearningOutcome) error {
	return nil
}
func (f *fakeLearningOutcomeRepo) FindByID(ctx context.Context, id, accountID uint) (*models.LearningOutcome, error) {
	return nil, errFakeNotFound
}
func (f *fakeLearningOutcomeRepo) Update(ctx context.Context, o *models.LearningOutcome) error {
	return nil
}
func (f *fakeLearningOutcomeRepo) Delete(ctx context.Context, id uint) error {
	return nil
}
func (f *fakeLearningOutcomeRepo) ListByGroupID(ctx context.Context, groupID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LearningOutcome], error) {
	return &repository.PaginatedResult[models.LearningOutcome]{}, nil
}
func (f *fakeLearningOutcomeRepo) ListByContext(ctx context.Context, contextType string, contextID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LearningOutcome], error) {
	items := f.byCourse[contextID]
	return &repository.PaginatedResult[models.LearningOutcome]{
		Items:      items,
		TotalCount: int64(len(items)),
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

// fakeLearningOutcomeGroupRepo: trivial stub (unused by the two test paths
// but required by the service constructor).
type fakeLearningOutcomeGroupRepo struct{}

func (f *fakeLearningOutcomeGroupRepo) Create(ctx context.Context, g *models.LearningOutcomeGroup) error {
	return nil
}
func (f *fakeLearningOutcomeGroupRepo) FindByID(ctx context.Context, id, accountID uint) (*models.LearningOutcomeGroup, error) {
	return nil, errFakeNotFound
}
func (f *fakeLearningOutcomeGroupRepo) Update(ctx context.Context, g *models.LearningOutcomeGroup) error {
	return nil
}
func (f *fakeLearningOutcomeGroupRepo) Delete(ctx context.Context, id uint) error {
	return nil
}
func (f *fakeLearningOutcomeGroupRepo) ListByContext(ctx context.Context, contextType string, contextID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LearningOutcomeGroup], error) {
	return &repository.PaginatedResult[models.LearningOutcomeGroup]{}, nil
}
func (f *fakeLearningOutcomeGroupRepo) FindRootGroup(ctx context.Context, contextType string, contextID, accountID uint) (*models.LearningOutcomeGroup, error) {
	return nil, errFakeNotFound
}

var _ repository.LearningOutcomeResultRepository = (*fakeLearningOutcomeResultRepo)(nil)
var _ repository.LearningOutcomeRepository = (*fakeLearningOutcomeRepo)(nil)
var _ repository.LearningOutcomeGroupRepository = (*fakeLearningOutcomeGroupRepo)(nil)

func TestListOutcomeResults_PerUser_FiresLogPIIAccess(t *testing.T) {
	resultRepo := newFakeLearningOutcomeResultRepo()
	resultRepo.byUser[42] = []models.LearningOutcomeResult{
		{ID: 1, UserID: 42, LearningOutcomeID: 5, ContextType: "Course", ContextID: 101},
	}

	outcomeSvc := service.NewLearningOutcomeService(&fakeLearningOutcomeGroupRepo{}, &fakeLearningOutcomeRepo{byCourse: map[uint][]models.LearningOutcome{}}, resultRepo)
	sink := newPIISink()
	auditService := service.NewAuditService(nil, nil, sink)
	h := handlers.NewLearningOutcomeHandler(outcomeSvc, new(mocks.MockOutcomeAlignmentRepository), auditService)

	app := testutil.SetupTestApp()
	installAuthAndPagination(app, 7)
	app.Get("/api/v1/courses/:course_id/outcome_results", h.ListResults)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/courses/101/outcome_results?user_id=42", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assertOneEntry(t, sink, 7, 42, "outcome_results", "courses")
}

func TestGetMasteryGradebook_FiresBulkLogPIIAccess(t *testing.T) {
	resultRepo := newFakeLearningOutcomeResultRepo()
	outcomeRepo := &fakeLearningOutcomeRepo{byCourse: map[uint][]models.LearningOutcome{
		101: {{ID: 5, ContextType: "Course", ContextID: 101}},
	}}
	outcomeSvc := service.NewLearningOutcomeService(&fakeLearningOutcomeGroupRepo{}, outcomeRepo, resultRepo)
	sink := newPIISink()
	auditService := service.NewAuditService(nil, nil, sink)
	h := handlers.NewLearningOutcomeHandler(outcomeSvc, new(mocks.MockOutcomeAlignmentRepository), auditService)

	app := testutil.SetupTestApp()
	installAuthAndPagination(app, 7)
	app.Get("/api/v1/courses/:course_id/outcome_rollups", h.GetMasteryGradebook)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/courses/101/outcome_rollups", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assertOneEntry(t, sink, 7, 0, "mastery_gradebook", "courses")
	assert.Equal(t, uint(101), sink.snapshot()[0].ResourceID)
}

// ===========================================================================
// groups.go — ListGroupMemberships
// ===========================================================================

func TestListGroupMemberships_FiresBulkLogPIIAccess(t *testing.T) {
	catRepo := new(mocks.MockGroupCategoryRepository)
	groupRepo := new(mocks.MockGroupRepository)
	membershipRepo := new(mocks.MockGroupMembershipRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)
	userRepo := new(mocks.MockUserRepository)

	// Group lookup → category lookup → courseID resolution path. The
	// handler resolves course context to authorize; the category has no
	// course (account-scoped) so the authz path is short-circuited.
	groupRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(&models.Group{ID: 5, GroupCategoryID: 7}, nil)
	catRepo.On("FindByID", mock.Anything, uint(7), uint(1)).Return(&models.GroupCategory{ID: 7, CourseID: nil}, nil)

	// Memberships returned by ListByGroupID — the data that emits the
	// bulk PII access row.
	membershipRepo.On("ListByGroupID", mock.Anything, uint(5), mock.Anything).Return(&repository.PaginatedResult[models.GroupMembership]{
		Items:      []models.GroupMembership{{ID: 1, GroupID: 5, UserID: 42}, {ID: 2, GroupID: 5, UserID: 88}},
		TotalCount: 2,
		Page:       1,
		PerPage:    25,
	}, nil)

	groupSvc := service.NewGroupService(catRepo, groupRepo, membershipRepo, enrollmentRepo)
	authz := handlers.NewResourceAuthorizer(enrollmentRepo, userRepo)
	sink := newPIISink()
	auditService := service.NewAuditService(nil, nil, sink)
	h := handlers.NewGroupHandler(groupSvc, authz, auditService)

	app := testutil.SetupTestApp()
	installAuthAndPagination(app, 7)
	app.Get("/api/v1/groups/:group_id/memberships", h.ListGroupMemberships)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/groups/5/memberships", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assertOneEntry(t, sink, 7, 0, "bulk_group_memberships", "groups")
	assert.Equal(t, uint(5), sink.snapshot()[0].ResourceID)
}
