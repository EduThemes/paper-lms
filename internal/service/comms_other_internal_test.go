package service

// Regression tests for the comms-other lane (announcements, notification
// delivery, conferences, collaborations, groups). Example data only.

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	repoPostgres "github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

var errCommsNotFound = errors.New("not found")

// ---------------------------------------------------------------------------
// fakes
// ---------------------------------------------------------------------------

type fakeDeliveryRepo struct {
	created     []models.NotificationDelivery
	listedTypes []string
}

func (f *fakeDeliveryRepo) Create(_ context.Context, d *models.NotificationDelivery) error {
	d.ID = uint(len(f.created) + 1)
	f.created = append(f.created, *d)
	return nil
}
func (f *fakeDeliveryRepo) FindByID(context.Context, uint) (*models.NotificationDelivery, error) {
	return nil, errCommsNotFound
}
func (f *fakeDeliveryRepo) Update(context.Context, *models.NotificationDelivery) error { return nil }
func (f *fakeDeliveryRepo) Delete(context.Context, uint, uint) error                  { return nil }
func (f *fakeDeliveryRepo) ListByUserID(context.Context, uint, repository.PaginationParams) (*repository.PaginatedResult[models.NotificationDelivery], error) {
	return &repository.PaginatedResult[models.NotificationDelivery]{}, nil
}
func (f *fakeDeliveryRepo) ListByUserIDAndStatus(context.Context, uint, string, repository.PaginationParams) (*repository.PaginatedResult[models.NotificationDelivery], error) {
	return &repository.PaginatedResult[models.NotificationDelivery]{}, nil
}
func (f *fakeDeliveryRepo) ListPending(context.Context, time.Time) ([]models.NotificationDelivery, error) {
	f.listedTypes = append(f.listedTypes, "*")
	return nil, nil
}
func (f *fakeDeliveryRepo) ListPendingByDigestType(_ context.Context, digestType string, _ time.Time) ([]models.NotificationDelivery, error) {
	f.listedTypes = append(f.listedTypes, digestType)
	return nil, nil
}
func (f *fakeDeliveryRepo) UpdateStatus(context.Context, uint, string) error    { return nil }
func (f *fakeDeliveryRepo) IncrementRetry(context.Context, uint, string) error  { return nil }
func (f *fakeDeliveryRepo) ListByNotificationID(context.Context, uint) ([]models.NotificationDelivery, error) {
	return nil, nil
}
func (f *fakeDeliveryRepo) CountByStatus(context.Context) (map[string]int64, error) { return nil, nil }
func (f *fakeDeliveryRepo) ListFailed(context.Context) ([]models.NotificationDelivery, error) {
	return nil, nil
}

var _ repoPostgres.NotificationDeliveryRepository = (*fakeDeliveryRepo)(nil)

type fakeChannelRepo struct {
	channels []models.CommunicationChannel
	updated  []models.CommunicationChannel
}

func (f *fakeChannelRepo) Create(context.Context, *models.CommunicationChannel) error { return nil }
func (f *fakeChannelRepo) FindByID(context.Context, uint) (*models.CommunicationChannel, error) {
	return nil, errCommsNotFound
}
func (f *fakeChannelRepo) Update(_ context.Context, ch *models.CommunicationChannel) error {
	f.updated = append(f.updated, *ch)
	return nil
}
func (f *fakeChannelRepo) Delete(context.Context, uint) error { return nil }
func (f *fakeChannelRepo) ListByUserID(context.Context, uint) ([]models.CommunicationChannel, error) {
	return f.channels, nil
}
func (f *fakeChannelRepo) FindByUserIDAndType(context.Context, uint, string) ([]models.CommunicationChannel, error) {
	return nil, nil
}
func (f *fakeChannelRepo) FindPrimaryByUserID(context.Context, uint) (*models.CommunicationChannel, error) {
	return nil, errCommsNotFound
}

var _ repoPostgres.CommunicationChannelRepository = (*fakeChannelRepo)(nil)

func newDeliveryFixture(t *testing.T, prefs *models.NotificationPreference, channels []models.CommunicationChannel) (*NotificationDeliveryService, *fakeDeliveryRepo, *mocks.MockNotificationRepository) {
	t.Helper()
	deliveries := &fakeDeliveryRepo{}
	prefRepo := new(mocks.MockNotificationPreferenceRepository)
	if prefs == nil {
		prefRepo.On("FindByUserID", mock.Anything, uint(50)).Return(nil, errCommsNotFound)
	} else {
		prefRepo.On("FindByUserID", mock.Anything, uint(50)).Return(prefs, nil)
	}
	notifRepo := new(mocks.MockNotificationRepository)
	notifRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Notification")).Run(func(args mock.Arguments) {
		args.Get(1).(*models.Notification).ID = 900
	}).Return(nil)
	userRepo := new(mocks.MockUserRepository)
	userRepo.On("FindByID", mock.Anything, uint(50), mock.Anything).Return(&models.User{ID: 50, Email: "jane.doe@example.com"}, nil)
	svc := NewNotificationDeliveryService(deliveries, &fakeChannelRepo{channels: channels}, prefRepo, notifRepo, userRepo, nil)
	return svc, deliveries, notifRepo
}

// ---------------------------------------------------------------------------
// Notification delivery (B055, B153, B168)
// ---------------------------------------------------------------------------

// B153: a user who never saved preferences gets the defaults, under which
// grade notifications are delivered (to their account email).
func TestQueueNotification_NoStoredPreferencesUsesDefaults(t *testing.T) {
	svc, deliveries, _ := newDeliveryFixture(t, nil, nil)
	require.NoError(t, svc.QueueNotification(context.Background(), 50, "submission_grade", "Grade posted", "body", "Assignment", 3))
	require.Len(t, deliveries.created, 1, "default preferences must deliver grade notifications")
	assert.Equal(t, "jane.doe@example.com", deliveries.created[0].Address)
	assert.Equal(t, "daily", deliveries.created[0].DigestType)
}

// B055: every queued notification also lands in the in-app feed, and the
// deliveries are linked to it.
func TestQueueNotification_CreatesInAppNotification(t *testing.T) {
	svc, deliveries, notifRepo := newDeliveryFixture(t, &models.NotificationPreference{UserID: 50, Policy: "immediately", NotifyNewAnnouncement: true}, nil)
	require.NoError(t, svc.QueueNotification(context.Background(), 50, "new_announcement", "New announcement: Quiz moved", "body", "Course", 5))
	notifRepo.AssertCalled(t, "Create", mock.Anything, mock.MatchedBy(func(n *models.Notification) bool {
		return n.UserID == 50 && n.NotificationType == "new_announcement" && n.Title == "New announcement: Quiz moved" && n.ContextID == 5
	}))
	require.Len(t, deliveries.created, 1)
	assert.Equal(t, uint(900), deliveries.created[0].NotificationID)
	assert.Equal(t, "immediate", deliveries.created[0].DigestType)
}

// B055: even with delivery turned off, the in-app notification is kept.
func TestQueueNotification_NeverPolicyStillRecordsInApp(t *testing.T) {
	svc, deliveries, notifRepo := newDeliveryFixture(t, &models.NotificationPreference{UserID: 50, Policy: "never"}, nil)
	require.NoError(t, svc.QueueNotification(context.Background(), 50, "new_message", "New message", "body", "Conversation", 4))
	notifRepo.AssertNumberOfCalls(t, "Create", 1)
	assert.Empty(t, deliveries.created)
}

// B168: unconfirmed addresses and push channels never receive deliveries;
// confirmed channels do, alongside the account email.
func TestQueueNotification_OnlyConfirmedDeliverableChannels(t *testing.T) {
	channels := []models.CommunicationChannel{
		{ID: 1, UserID: 50, ChannelType: "email", Address: "unverified@example.com", Confirmed: false, WorkflowState: "active"},
		{ID: 2, UserID: 50, ChannelType: "push", Address: "device-token", Confirmed: true, WorkflowState: "active"},
		{ID: 3, UserID: 50, ChannelType: "email", Address: "verified@example.com", Confirmed: true, WorkflowState: "active"},
		{ID: 4, UserID: 50, ChannelType: "email", Address: "Jane.Doe@example.com", Confirmed: true, WorkflowState: "active"},
	}
	svc, deliveries, _ := newDeliveryFixture(t, nil, channels)
	require.NoError(t, svc.QueueNotification(context.Background(), 50, "submission_grade", "Grade posted", "body", "Assignment", 3))
	var addrs []string
	for _, d := range deliveries.created {
		addrs = append(addrs, d.Address)
		assert.NotEqual(t, "push", d.ChannelType)
	}
	assert.ElementsMatch(t, []string{"jane.doe@example.com", "verified@example.com"}, addrs)
}

// B055: the per-minute job sends only "immediate" deliveries; daily and
// weekly rows wait for their digest.
func TestProcessPendingDeliveries_OnlyImmediate(t *testing.T) {
	svc, deliveries, _ := newDeliveryFixture(t, nil, nil)
	_, err := svc.ProcessPendingDeliveries(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"immediate"}, deliveries.listedTypes)
}

func TestChannelConfirmation(t *testing.T) {
	channelRepo := &fakeChannelRepo{}
	svc := NewNotificationDeliveryService(&fakeDeliveryRepo{}, channelRepo, nil, nil, nil, nil)

	webhook := &models.CommunicationChannel{UserID: 50, ChannelType: "webhook", Address: "https://hooks.example.com/x"}
	code, err := svc.BeginChannelConfirmation(webhook)
	require.NoError(t, err)
	assert.Empty(t, code)
	assert.True(t, webhook.Confirmed, "owner-supplied webhooks need no emailed code")

	ch := &models.CommunicationChannel{UserID: 50, ChannelType: "email", Address: "second@example.com"}
	code, err = svc.BeginChannelConfirmation(ch)
	require.NoError(t, err)
	require.Len(t, code, 10)
	assert.False(t, ch.Confirmed)
	assert.NotContains(t, ch.ConfirmCode, code, "only a hash of the code is stored")

	assert.ErrorIs(t, svc.ConfirmChannel(context.Background(), ch, "WRONGCODE1"), ErrInvalidConfirmationCode)
	assert.False(t, ch.Confirmed)

	require.NoError(t, svc.ConfirmChannel(context.Background(), ch, strings.ToLower(code)))
	assert.True(t, ch.Confirmed)
	assert.Empty(t, ch.ConfirmCode)
	require.Len(t, channelRepo.updated, 1)
}

func TestDefaultNotificationPreferenceMatchesGetOrCreate(t *testing.T) {
	prefRepo := new(mocks.MockNotificationPreferenceRepository)
	prefRepo.On("FindByUserID", mock.Anything, uint(50)).Return(nil, errCommsNotFound)
	prefRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	got, err := NewNotificationService(prefRepo, nil).GetOrCreatePreferences(context.Background(), 50)
	require.NoError(t, err)
	assert.Equal(t, DefaultNotificationPreference(50), got)
}

// ---------------------------------------------------------------------------
// Announcements (B053, B151)
// ---------------------------------------------------------------------------

func TestResolveWorkflowState(t *testing.T) {
	now := time.Now()
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)

	a := &models.Announcement{WorkflowState: "scheduled", DelayedPostAt: &future}
	require.NoError(t, resolveWorkflowState(a, now))
	assert.Equal(t, AnnouncementStateScheduled, a.WorkflowState)
	assert.Nil(t, a.PostedAt)

	a = &models.Announcement{WorkflowState: "scheduled", DelayedPostAt: &past}
	require.NoError(t, resolveWorkflowState(a, now))
	assert.Equal(t, AnnouncementStatePublished, a.WorkflowState, "a delay that already passed posts immediately")
	assert.NotNil(t, a.PostedAt)

	a = &models.Announcement{WorkflowState: "scheduled"}
	require.NoError(t, resolveWorkflowState(a, now))
	assert.Equal(t, AnnouncementStatePublished, a.WorkflowState)

	a = &models.Announcement{WorkflowState: "draft", DelayedPostAt: &future}
	require.NoError(t, resolveWorkflowState(a, now))
	assert.Equal(t, AnnouncementStateDraft, a.WorkflowState)

	assert.Error(t, resolveWorkflowState(&models.Announcement{WorkflowState: "bogus"}, now))
}

type commsRecordingNotifier struct{ users []uint }

func (r *commsRecordingNotifier) QueueNotification(_ context.Context, userID uint, _, _, _, _ string, _ uint) error {
	r.users = append(r.users, userID)
	return nil
}

func TestAnnouncementNotifyPublished_TargetsAudience(t *testing.T) {
	sec7 := uint(7)
	enrollRepo := new(mocks.MockEnrollmentRepository)
	enrollRepo.On("ListByCourseID", mock.Anything, uint(5), uint(0), mock.Anything).Return(&repository.PaginatedResult[models.Enrollment]{
		Items: []models.Enrollment{
			{UserID: 40, CourseID: 5, Type: "TeacherEnrollment", WorkflowState: "active"},
			{UserID: 50, CourseID: 5, Type: "StudentEnrollment", WorkflowState: "active", CourseSectionID: &sec7},
			{UserID: 51, CourseID: 5, Type: "StudentEnrollment", WorkflowState: "active"},
			{UserID: 52, CourseID: 5, Type: "StudentEnrollment", WorkflowState: "completed"},
			{UserID: 60, CourseID: 5, Type: "ObserverEnrollment", WorkflowState: "active"},
		},
		TotalCount: 5,
	}, nil)
	svc := NewAnnouncementService(nil, nil, enrollRepo)
	cid := uint(5)

	for _, tc := range []struct {
		audience string
		want     []uint
	}{
		{"all", []uint{50, 51, 60}},
		{"students", []uint{50, 51}},
		{"observers", []uint{60}},
		{"section:7", []uint{50}},
	} {
		n := &commsRecordingNotifier{}
		svc.SetNotifier(n)
		svc.notifyPublished(context.Background(), &models.Announcement{CourseID: &cid, UserID: 40, Title: "t", TargetAudience: tc.audience})
		assert.ElementsMatch(t, tc.want, n.users, "audience %s", tc.audience)
	}
}

// createOnlyAnnouncementRepo implements Create; any other call panics.
type createOnlyAnnouncementRepo struct {
	repoPostgres.AnnouncementRepository
}

func (createOnlyAnnouncementRepo) Create(_ context.Context, a *models.Announcement) error {
	a.ID = 99
	return nil
}

// blockingNotifier blocks every QueueNotification until release is closed.
type blockingNotifier struct {
	release chan struct{}
	calls   chan uint
}

func (b *blockingNotifier) QueueNotification(_ context.Context, userID uint, _, _, _, _ string, _ uint) error {
	<-b.release
	b.calls <- userID
	return nil
}

// Publishing must not wait for the per-member fan-out: CreateAnnouncement
// returns while notifications are still being queued, and they still
// arrive afterwards.
func TestCreateAnnouncement_NotifiesOffTheRequestPath(t *testing.T) {
	enrollRepo := new(mocks.MockEnrollmentRepository)
	enrollRepo.On("ListByCourseID", mock.Anything, uint(5), uint(0), mock.Anything).Return(&repository.PaginatedResult[models.Enrollment]{
		Items:      []models.Enrollment{{UserID: 50, CourseID: 5, Type: "StudentEnrollment", WorkflowState: "active"}},
		TotalCount: 1,
	}, nil)
	n := &blockingNotifier{release: make(chan struct{}), calls: make(chan uint, 4)}
	svc := NewAnnouncementService(createOnlyAnnouncementRepo{}, nil, enrollRepo)
	svc.SetNotifier(n)
	cid := uint(5)

	done := make(chan error, 1)
	go func() {
		done <- svc.CreateAnnouncement(context.Background(), &models.Announcement{CourseID: &cid, UserID: 40, Title: "Quiz moved", Message: "m"})
	}()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		close(n.release)
		t.Fatal("CreateAnnouncement blocked on the notification fan-out")
	}

	close(n.release)
	select {
	case uid := <-n.calls:
		assert.Equal(t, uint(50), uid)
	case <-time.After(3 * time.Second):
		t.Fatal("the student was never notified")
	}
}

func TestAnnouncementViewer_TeacherSeesEverything(t *testing.T) {
	enrollRepo := new(mocks.MockEnrollmentRepository)
	enrollRepo.On("ListByUserID", mock.Anything, uint(40), uint(0)).Return([]models.Enrollment{{UserID: 40, CourseID: 5, Type: "TeacherEnrollment", WorkflowState: "active"}}, nil)
	sec := uint(9)
	enrollRepo.On("ListByUserID", mock.Anything, uint(50), uint(0)).Return([]models.Enrollment{{UserID: 50, CourseID: 5, Type: "StudentEnrollment", WorkflowState: "active", CourseSectionID: &sec}}, nil)
	svc := NewAnnouncementService(nil, nil, enrollRepo)

	assert.True(t, svc.viewerFor(context.Background(), 5, 40).Instructor)
	v := svc.viewerFor(context.Background(), 5, 50)
	assert.False(t, v.Instructor)
	assert.Equal(t, []string{"students"}, v.Audiences)
	assert.Equal(t, []uint{9}, v.SectionIDs)

	cid := uint(5)
	teachersOnly := &models.Announcement{CourseID: &cid, WorkflowState: "published", TargetAudience: "students"}
	assert.True(t, svc.CanView(context.Background(), teachersOnly, 40), "the author's own students-only post stays visible to the teacher")
	otherSection := &models.Announcement{CourseID: &cid, WorkflowState: "published", TargetAudience: "section:8"}
	assert.False(t, svc.CanView(context.Background(), otherSection, 50))
	mySection := &models.Announcement{CourseID: &cid, WorkflowState: "published", TargetAudience: "section:9"}
	assert.True(t, svc.CanView(context.Background(), mySection, 50))
}

// ---------------------------------------------------------------------------
// Conferences (B058)
// ---------------------------------------------------------------------------

func TestConferenceJoin_BigBlueButtonSignedURL(t *testing.T) {
	partRepo := new(mocks.MockConferenceParticipantRepository)
	partRepo.On("FindByConferenceAndUser", mock.Anything, uint(9), mock.Anything, mock.Anything).Return(nil, errCommsNotFound)
	partRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	svc := NewConferenceService(nil, partRepo)
	settings := map[string]string{"conferences.bbb.url": "https://bbb.example.com/bigbluebutton/", "conferences.bbb.secret": "example-secret"}
	svc.SetSettingsLookup(func(_ context.Context, key string) (string, error) { return settings[key], nil })
	// G098: join now checks the meeting on the server; answer from a fake.
	useFakeBBB(t, svc, "example-secret").open = true

	started := time.Now()
	conf := &models.Conference{ID: 9, ConferenceType: "BigBlueButton", Title: "Office hours", StartedAt: &started}
	joinURL, err := svc.JoinConference(context.Background(), conf, 50, "Jane Doe", false, 1)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(joinURL, "https://bbb.example.com/bigbluebutton/api/join?"), joinURL)

	u, err := url.Parse(joinURL)
	require.NoError(t, err)
	q := u.Query()
	assert.Equal(t, "VIEWER", q.Get("role"))
	assert.Equal(t, "paper-conference-9", q.Get("meetingID"))
	// The checksum must verify the way a BBB server checks it.
	checksum := q.Get("checksum")
	raw := u.RawQuery[:strings.LastIndex(u.RawQuery, "&checksum=")]
	cfg := bbbConfig{BaseURL: "https://bbb.example.com/bigbluebutton/api/", Secret: "example-secret"}
	want := bbbURL(cfg, "join", mustParseQuery(t, raw))
	assert.Contains(t, want, "checksum="+checksum)

	modURL, err := svc.JoinConference(context.Background(), conf, 40, "Pat Teacher", true, 1)
	require.NoError(t, err)
	assert.Contains(t, modURL, "role=MODERATOR")
	assert.NotEqual(t, q.Get("password"), mustParseURL(t, modURL).Query().Get("password"), "moderators and viewers get different passwords")
}

func TestConferenceStart_NoProvider(t *testing.T) {
	svc := NewConferenceService(nil, nil)
	_, err := svc.StartConference(context.Background(), &models.Conference{ID: 9, ConferenceType: "BigBlueButton"}, 1)
	assert.ErrorIs(t, err, ErrConferenceNotConfigured)

	_, err = svc.JoinConference(context.Background(), &models.Conference{ID: 9, ConferenceType: "Zoom"}, 50, "Jane Doe", false, 1)
	assert.ErrorIs(t, err, ErrConferenceNotStarted)
}

func TestValidateMeetingURL(t *testing.T) {
	assert.NoError(t, ValidateMeetingURL(""))
	assert.NoError(t, ValidateMeetingURL("https://zoom.example.com/j/1"))
	assert.Error(t, ValidateMeetingURL("javascript:alert(1)"))
	assert.Error(t, ValidateMeetingURL("/conferences/1/join"))
}

func mustParseURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	require.NoError(t, err)
	return u
}

func mustParseQuery(t *testing.T, s string) url.Values {
	t.Helper()
	v, err := url.ParseQuery(s)
	require.NoError(t, err)
	return v
}

// ---------------------------------------------------------------------------
// Collaborations (B154)
// ---------------------------------------------------------------------------

func TestValidateCollaborationURL(t *testing.T) {
	for _, ok := range []string{"", "https://docs.example.com/d/1", "http://pad.example.com/p/x"} {
		assert.NoError(t, ValidateCollaborationURL(ok), ok)
	}
	for _, bad := range []string{"javascript:alert(1)", "JaVaScRiPt:alert(1)", "data:text/html,<script>x</script>", "vbscript:x", "//no-scheme.example.com", "docs.example.com"} {
		assert.ErrorIs(t, ValidateCollaborationURL(bad), ErrInvalidCollaborationURL, bad)
	}
}

// ---------------------------------------------------------------------------
// Groups (B163)
// ---------------------------------------------------------------------------

func TestAddMember_GroupLimitFromCategory(t *testing.T) {
	limit := 2
	catRepo := new(mocks.MockGroupCategoryRepository)
	groupRepo := new(mocks.MockGroupRepository)
	memRepo := new(mocks.MockGroupMembershipRepository)
	groupRepo.On("FindByID", mock.Anything, uint(21), uint(0)).Return(&models.Group{ID: 21, GroupCategoryID: 7}, nil)
	catRepo.On("FindByID", mock.Anything, uint(7), uint(0)).Return(&models.GroupCategory{ID: 7, GroupLimit: &limit}, nil)
	memRepo.On("FindByGroupAndUser", mock.Anything, uint(21), uint(50)).Return(nil, errCommsNotFound)
	memRepo.On("ListByGroupID", mock.Anything, uint(21), mock.Anything).Return(&repository.PaginatedResult[models.GroupMembership]{TotalCount: 2}, nil)

	err := NewGroupService(catRepo, groupRepo, memRepo, nil).AddMember(context.Background(), &models.GroupMembership{GroupID: 21, UserID: 50})
	assert.ErrorIs(t, err, ErrGroupFull)
	memRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestAddMember_MovesUserOutOfOtherGroupInSet(t *testing.T) {
	catRepo := new(mocks.MockGroupCategoryRepository)
	groupRepo := new(mocks.MockGroupRepository)
	memRepo := new(mocks.MockGroupMembershipRepository)
	groupRepo.On("FindByID", mock.Anything, uint(21), uint(0)).Return(&models.Group{ID: 21, GroupCategoryID: 7}, nil)
	catRepo.On("FindByID", mock.Anything, uint(7), uint(0)).Return(&models.GroupCategory{ID: 7}, nil)
	memRepo.On("FindByGroupAndUser", mock.Anything, uint(21), uint(50)).Return(nil, errCommsNotFound)
	// Already in group 22 of the same set; after the move, in none.
	memRepo.On("FindUserGroupInCategory", mock.Anything, uint(50), uint(7)).Return(&models.Group{ID: 22, GroupCategoryID: 7}, nil).Once()
	memRepo.On("FindUserGroupInCategory", mock.Anything, uint(50), uint(7)).Return(nil, errCommsNotFound)
	memRepo.On("FindByGroupAndUser", mock.Anything, uint(22), uint(50)).Return(&models.GroupMembership{ID: 300, GroupID: 22, UserID: 50}, nil)
	memRepo.On("Delete", mock.Anything, uint(300)).Return(nil)
	memRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.GroupMembership")).Return(nil)

	require.NoError(t, NewGroupService(catRepo, groupRepo, memRepo, nil).AddMember(context.Background(), &models.GroupMembership{GroupID: 21, UserID: 50}))
	memRepo.AssertCalled(t, "Delete", mock.Anything, uint(300))
	memRepo.AssertCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestCheckSelfSignup_RestrictedRequiresCommonSection(t *testing.T) {
	sec1, sec2 := uint(1), uint(2)
	catRepo := new(mocks.MockGroupCategoryRepository)
	memRepo := new(mocks.MockGroupMembershipRepository)
	enrollRepo := new(mocks.MockEnrollmentRepository)
	catRepo.On("FindByID", mock.Anything, uint(7), uint(0)).Return(&models.GroupCategory{ID: 7, SelfSignup: "restricted"}, nil)
	memRepo.On("ListByGroupID", mock.Anything, uint(21), mock.Anything).Return(&repository.PaginatedResult[models.GroupMembership]{
		Items: []models.GroupMembership{{GroupID: 21, UserID: 51, WorkflowState: "accepted"}},
	}, nil)
	enrollRepo.On("ListByUserID", mock.Anything, uint(50), uint(0)).Return([]models.Enrollment{{CourseID: 5, Type: "StudentEnrollment", WorkflowState: "active", CourseSectionID: &sec1}}, nil)
	enrollRepo.On("ListByUserID", mock.Anything, uint(51), uint(0)).Return([]models.Enrollment{{CourseID: 5, Type: "StudentEnrollment", WorkflowState: "active", CourseSectionID: &sec2}}, nil)
	enrollRepo.On("ListByUserID", mock.Anything, uint(52), uint(0)).Return([]models.Enrollment{{CourseID: 5, Type: "StudentEnrollment", WorkflowState: "active", CourseSectionID: &sec2}}, nil)

	svc := NewGroupService(catRepo, nil, memRepo, enrollRepo)
	group := &models.Group{ID: 21, GroupCategoryID: 7}
	assert.ErrorIs(t, svc.CheckSelfSignup(context.Background(), group, 50, 5), ErrSelfSignupNoCommonSection)
	assert.NoError(t, svc.CheckSelfSignup(context.Background(), group, 52, 5))
}

func TestCheckSelfSignup_DeletedGroupOrSetRefused(t *testing.T) {
	catRepo := new(mocks.MockGroupCategoryRepository)
	enrollRepo := new(mocks.MockEnrollmentRepository)
	catRepo.On("FindByID", mock.Anything, uint(7), uint(0)).Return(&models.GroupCategory{ID: 7, SelfSignup: "enabled"}, nil)
	catRepo.On("FindByID", mock.Anything, uint(8), uint(0)).Return(&models.GroupCategory{ID: 8, SelfSignup: "enabled", WorkflowState: "deleted"}, nil)
	enrollRepo.On("ListByUserID", mock.Anything, uint(50), uint(0)).Return([]models.Enrollment{{CourseID: 5, Type: "StudentEnrollment", WorkflowState: "active"}}, nil)
	svc := NewGroupService(catRepo, nil, nil, enrollRepo)

	assert.NoError(t, svc.CheckSelfSignup(context.Background(), &models.Group{ID: 21, GroupCategoryID: 7, WorkflowState: "available"}, 50, 5))
	assert.ErrorIs(t, svc.CheckSelfSignup(context.Background(), &models.Group{ID: 22, GroupCategoryID: 7, WorkflowState: "deleted"}, 50, 5), ErrSelfSignupDisabled,
		"a deleted group cannot be joined")
	assert.ErrorIs(t, svc.CheckSelfSignup(context.Background(), &models.Group{ID: 23, GroupCategoryID: 8, WorkflowState: "available"}, 50, 5), ErrSelfSignupDisabled,
		"a group in a deleted group set cannot be joined")
}

// ---------------------------------------------------------------------------
// Discussion reply notifications (B055)
// ---------------------------------------------------------------------------

func TestDiscussionReplyNotificationCallback(t *testing.T) {
	entryRepo := new(mocks.MockDiscussionEntryRepository)
	topicRepo := new(mocks.MockDiscussionTopicRepository)
	parentID := uint(70)
	entryRepo.On("FindByID", mock.Anything, uint(71), uint(0)).Return(&models.DiscussionEntry{ID: 71, DiscussionTopicID: 9, UserID: 50, ParentID: &parentID, Message: "I agree"}, nil)
	entryRepo.On("FindByID", mock.Anything, uint(70), uint(0)).Return(&models.DiscussionEntry{ID: 70, DiscussionTopicID: 9, UserID: 51, Message: "First post"}, nil)
	entryRepo.On("FindByID", mock.Anything, uint(72), uint(0)).Return(&models.DiscussionEntry{ID: 72, DiscussionTopicID: 9, UserID: 52, Message: "Top level"}, nil)
	entryRepo.On("FindByID", mock.Anything, uint(73), uint(0)).Return(&models.DiscussionEntry{ID: 73, DiscussionTopicID: 9, UserID: 40, Message: "Own topic"}, nil)
	topicRepo.On("FindByID", mock.Anything, uint(9), uint(0)).Return(&models.DiscussionTopic{ID: 9, CourseID: 5, UserID: 40, Title: "Week 1"}, nil)

	n := &commsRecordingNotifier{}
	cb := DiscussionReplyNotificationCallback(entryRepo, topicRepo, n)
	cb(context.Background(), 71) // reply → parent author 51
	cb(context.Background(), 72) // top-level → topic author 40
	cb(context.Background(), 73) // the topic author posting in their own topic → nobody
	assert.Equal(t, []uint{51, 40}, n.users)
}
