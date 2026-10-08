package service

// Tests for the notify-inbox lane: per-category x per-channel notification
// policies and stub channels (G091), announcement section targeting and
// replies (G090). Example data only.

import (
	"context"
	"errors"
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

// ---------------------------------------------------------------------------
// fakes
// ---------------------------------------------------------------------------

type fakePolicyStore struct {
	rows    []models.NotificationPolicy
	deleted []string
}

func (f *fakePolicyStore) ListByUser(_ context.Context, userID uint) ([]models.NotificationPolicy, error) {
	var out []models.NotificationPolicy
	for _, r := range f.rows {
		if r.UserID == userID {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakePolicyStore) Upsert(_ context.Context, p *models.NotificationPolicy) error {
	for i, r := range f.rows {
		if r.UserID == p.UserID && r.CommunicationChannelID == p.CommunicationChannelID && r.Category == p.Category {
			f.rows[i].Frequency = p.Frequency
			return nil
		}
	}
	f.rows = append(f.rows, *p)
	return nil
}
func (f *fakePolicyStore) Delete(_ context.Context, userID, channelID uint, category string) error {
	kept := f.rows[:0]
	for _, r := range f.rows {
		if r.UserID == userID && r.CommunicationChannelID == channelID && r.Category == category {
			f.deleted = append(f.deleted, category)
			continue
		}
		kept = append(kept, r)
	}
	f.rows = kept
	return nil
}
func (f *fakePolicyStore) DeleteByChannel(context.Context, uint, uint) error { return nil }

type fakeSender struct{ sent []string }

func (f *fakeSender) Send(_ context.Context, address, _, _ string) error {
	f.sent = append(f.sent, address)
	return nil
}

func policy(channelID uint, category, frequency string) models.NotificationPolicy {
	return models.NotificationPolicy{UserID: 50, CommunicationChannelID: channelID, Category: category, Frequency: frequency}
}

// ---------------------------------------------------------------------------
// G091 — notification engine
// ---------------------------------------------------------------------------

func TestNotificationCategoryFor(t *testing.T) {
	assert.Equal(t, "grading", NotificationCategoryFor("submission_grade"))
	assert.Equal(t, "announcement", NotificationCategoryFor("new_announcement"))
	assert.Equal(t, "conversation_message", NotificationCategoryFor("new_message"))
	assert.Equal(t, "announcement_created_by_you", NotificationCategoryFor("announcement_reply"))
	assert.Equal(t, "discussion_entry", NotificationCategoryFor("discussion_reply"))
	assert.Equal(t, NotificationCategoryOther, NotificationCategoryFor("something_new"))
}

// Each channel follows its own frequency for the category: the account
// email can be "never" while a second address gets the same category
// immediately.
func TestQueueNotification_PerChannelCategoryPolicy(t *testing.T) {
	channels := []models.CommunicationChannel{
		{ID: 3, UserID: 50, ChannelType: "email", Address: "verified@example.com", Confirmed: true, WorkflowState: "active"},
	}
	svc, deliveries, _ := newDeliveryFixture(t, &models.NotificationPreference{UserID: 50, Policy: "daily", NotifySubmissionGrade: true, NotifyNewMessage: true}, channels)
	svc.SetPolicyStore(&fakePolicyStore{rows: []models.NotificationPolicy{
		policy(models.PrimaryEmailChannelID, "grading", "never"),
		policy(3, "grading", "immediately"),
	}})

	require.NoError(t, svc.QueueNotification(context.Background(), 50, "submission_grade", "Grade posted", "body", "Assignment", 3))
	require.Len(t, deliveries.created, 1)
	assert.Equal(t, "verified@example.com", deliveries.created[0].Address)
	assert.Equal(t, "immediate", deliveries.created[0].DigestType)

	// Another category still follows the global preference on both.
	deliveries.created = nil
	require.NoError(t, svc.QueueNotification(context.Background(), 50, "new_message", "New message", "body", "Conversation", 4))
	require.Len(t, deliveries.created, 2)
	for _, d := range deliveries.created {
		assert.Equal(t, "daily", d.DigestType)
	}
}

// An explicit category cell wins over the legacy per-type toggle; without
// a cell the toggle still applies.
func TestQueueNotification_ExplicitCellBeatsLegacyToggle(t *testing.T) {
	prefs := &models.NotificationPreference{UserID: 50, Policy: "daily", NotifySubmissionGrade: false, NotifyNewMessage: true}
	svc, deliveries, _ := newDeliveryFixture(t, prefs, nil)
	store := &fakePolicyStore{}
	svc.SetPolicyStore(store)

	require.NoError(t, svc.QueueNotification(context.Background(), 50, "submission_grade", "Grade posted", "body", "Assignment", 3))
	assert.Empty(t, deliveries.created, "legacy toggle off and no explicit cell: nothing is delivered")

	store.rows = []models.NotificationPolicy{policy(models.PrimaryEmailChannelID, "grading", "weekly")}
	require.NoError(t, svc.QueueNotification(context.Background(), 50, "submission_grade", "Grade posted", "body", "Assignment", 3))
	require.Len(t, deliveries.created, 1)
	assert.Equal(t, "weekly", deliveries.created[0].DigestType)
}

// Daily/weekly deliveries are due when queued, so the next digest run (at
// the digest hour / day) picks them up instead of the one after.
func TestCalculateScheduledFor_DigestsDueImmediately(t *testing.T) {
	svc := &NotificationDeliveryService{}
	before := time.Now()
	for _, dt := range []string{"daily", "weekly", "immediate"} {
		got := svc.calculateScheduledFor(dt)
		assert.False(t, got.After(time.Now()), "%s must not be scheduled in the future", dt)
		assert.False(t, got.Before(before.Add(-time.Second)), "%s", dt)
	}
}

func TestEffectivePolicies_GridAndExplicitFlags(t *testing.T) {
	channels := []models.CommunicationChannel{
		{ID: 3, UserID: 50, ChannelType: "email", Address: "verified@example.com", Confirmed: true, WorkflowState: "active"},
		{ID: 4, UserID: 50, ChannelType: "email", Address: "pending@example.com", Confirmed: false, WorkflowState: "active"},
	}
	svc, _, _ := newDeliveryFixture(t, &models.NotificationPreference{UserID: 50, Policy: "weekly", NotifyNewMessage: false, NotifySubmissionGrade: true, NotifyNewAnnouncement: true}, channels)
	svc.SetPolicyStore(&fakePolicyStore{rows: []models.NotificationPolicy{policy(3, "announcement", "immediately")}})

	grid, err := svc.EffectivePolicies(context.Background(), 50)
	require.NoError(t, err)
	require.Len(t, grid.Channels, 3)
	assert.True(t, grid.Channels[0].Primary)
	assert.False(t, grid.Channels[2].Deliverable, "an unconfirmed address receives nothing")
	assert.Len(t, grid.Policies, 3*len(NotificationCategories))

	cell := func(ch uint, cat string) PolicyCell {
		for _, p := range grid.Policies {
			if p.ChannelID == ch && p.Category == cat {
				return p
			}
		}
		t.Fatalf("no cell %d/%s", ch, cat)
		return PolicyCell{}
	}
	assert.Equal(t, PolicyCell{ChannelID: 3, Category: "announcement", Frequency: "immediately", Explicit: true}, cell(3, "announcement"))
	assert.Equal(t, "weekly", cell(0, "announcement").Frequency)
	assert.Equal(t, "never", cell(0, "conversation_message").Frequency, "legacy new-message toggle off")
	assert.False(t, cell(0, "conversation_message").Explicit)
}

func TestSetPolicy_Validation(t *testing.T) {
	channels := []models.CommunicationChannel{{ID: 3, UserID: 50, ChannelType: "email", Address: "verified@example.com", Confirmed: true, WorkflowState: "active"}}
	svc, _, _ := newDeliveryFixture(t, nil, channels)
	store := &fakePolicyStore{}
	svc.SetPolicyStore(store)
	ctx := context.Background()

	require.NoError(t, svc.SetPolicy(ctx, 50, 3, "grading", "immediately"))
	require.NoError(t, svc.SetPolicy(ctx, 50, 0, "grading", "never"))
	assert.Len(t, store.rows, 2)

	assert.ErrorIs(t, svc.SetPolicy(ctx, 50, 99, "grading", "daily"), ErrUnknownPolicyChannel, "another user's channel id is refused")
	assert.Error(t, svc.SetPolicy(ctx, 50, 3, "no_such_category", "daily"))
	assert.Error(t, svc.SetPolicy(ctx, 50, 3, "grading", "hourly"))

	require.NoError(t, svc.SetPolicy(ctx, 50, 3, "grading", "default"))
	assert.Equal(t, []string{"grading"}, store.deleted)
	assert.Len(t, store.rows, 1)
}

// SMS and push are stub channel types: listed as unavailable and never
// delivered to until a provider is registered.
func TestStubChannels_SMSNeedsProvider(t *testing.T) {
	channels := []models.CommunicationChannel{
		{ID: 5, UserID: 50, ChannelType: "sms", Address: "+15555550100", Confirmed: true, WorkflowState: "active"},
	}
	svc, deliveries, _ := newDeliveryFixture(t, &models.NotificationPreference{UserID: 50, Policy: "immediately", NotifyNewAnnouncement: true}, channels)

	types := map[string]ChannelTypeInfo{}
	for _, ct := range svc.ChannelTypes() {
		types[ct.Type] = ct
	}
	assert.False(t, types["sms"].Available)
	assert.NotEmpty(t, types["sms"].Reason)
	assert.False(t, types["push"].Available)
	assert.True(t, types["email"].Available)

	require.NoError(t, svc.QueueNotification(context.Background(), 50, "new_announcement", "New announcement", "body", "Course", 5))
	for _, d := range deliveries.created {
		assert.NotEqual(t, "sms", d.ChannelType, "no provider: the SMS channel receives nothing")
	}

	sender := &fakeSender{}
	svc.RegisterChannelSender("sms", sender)
	assert.True(t, svc.IsDeliverableChannelType("sms"))
	deliveries.created = nil
	require.NoError(t, svc.QueueNotification(context.Background(), 50, "new_announcement", "New announcement", "<p>body</p>", "Course", 5))
	var sms *models.NotificationDelivery
	for i := range deliveries.created {
		if deliveries.created[i].ChannelType == "sms" {
			sms = &deliveries.created[i]
		}
	}
	require.NotNil(t, sms)
	require.NoError(t, svc.dispatch(context.Background(), 50, "sms", sms.Address, sms.Subject, sms.Body))
	assert.Equal(t, []string{"+15555550100"}, sender.sent)
}

// ---------------------------------------------------------------------------
// G090 — announcements
// ---------------------------------------------------------------------------

type fakeAnnouncementExtras struct {
	sections map[uint][]uint
	inCourse map[uint][]uint // courseID -> section ids
	replies  []models.AnnouncementReply
	setCalls map[uint][]uint
}

func newFakeExtras() *fakeAnnouncementExtras {
	return &fakeAnnouncementExtras{sections: map[uint][]uint{}, inCourse: map[uint][]uint{}, setCalls: map[uint][]uint{}}
}

func (f *fakeAnnouncementExtras) CreateReply(_ context.Context, r *models.AnnouncementReply) error {
	r.ID = uint(len(f.replies) + 1)
	f.replies = append(f.replies, *r)
	return nil
}
func (f *fakeAnnouncementExtras) FindReply(_ context.Context, announcementID, replyID uint) (*models.AnnouncementReply, error) {
	for _, r := range f.replies {
		if r.ID == replyID && r.AnnouncementID == announcementID && r.WorkflowState == models.AnnouncementReplyActive {
			cp := r
			return &cp, nil
		}
	}
	return nil, errors.New("not found")
}
func (f *fakeAnnouncementExtras) UpdateReply(_ context.Context, r *models.AnnouncementReply) error {
	for i := range f.replies {
		if f.replies[i].ID == r.ID {
			f.replies[i] = *r
		}
	}
	return nil
}
func (f *fakeAnnouncementExtras) ListReplies(context.Context, uint, repository.PaginationParams) (*repository.PaginatedResult[models.AnnouncementReply], error) {
	return &repository.PaginatedResult[models.AnnouncementReply]{Items: f.replies}, nil
}
func (f *fakeAnnouncementExtras) ReplyCounts(context.Context, []uint) (map[uint]int64, error) {
	return map[uint]int64{}, nil
}
func (f *fakeAnnouncementExtras) SetSections(_ context.Context, id uint, s []uint) error {
	f.setCalls[id] = s
	f.sections[id] = s
	return nil
}
func (f *fakeAnnouncementExtras) SectionsFor(_ context.Context, ids []uint) (map[uint][]uint, error) {
	out := map[uint][]uint{}
	for _, id := range ids {
		if s, ok := f.sections[id]; ok {
			out[id] = s
		}
	}
	return out, nil
}
func (f *fakeAnnouncementExtras) SectionsInCourse(_ context.Context, courseID uint, ids []uint) ([]uint, error) {
	allowed := map[uint]bool{}
	for _, s := range f.inCourse[courseID] {
		allowed[s] = true
	}
	var out []uint
	for _, id := range ids {
		if allowed[id] {
			out = append(out, id)
		}
	}
	return out, nil
}

func TestAnnouncementNotifyPublished_MultiSectionTarget(t *testing.T) {
	sec7, sec8, sec9 := uint(7), uint(8), uint(9)
	enrollRepo := new(mocks.MockEnrollmentRepository)
	enrollRepo.On("ListByCourseID", mock.Anything, uint(5), uint(0), mock.Anything).Return(&repository.PaginatedResult[models.Enrollment]{
		Items: []models.Enrollment{
			{UserID: 40, CourseID: 5, Type: "TeacherEnrollment", WorkflowState: "active"},
			{UserID: 41, CourseID: 5, Type: "TaEnrollment", WorkflowState: "active", CourseSectionID: &sec7},
			{UserID: 50, CourseID: 5, Type: "StudentEnrollment", WorkflowState: "active", CourseSectionID: &sec7},
			{UserID: 51, CourseID: 5, Type: "StudentEnrollment", WorkflowState: "active", CourseSectionID: &sec8},
			{UserID: 52, CourseID: 5, Type: "StudentEnrollment", WorkflowState: "active", CourseSectionID: &sec9},
		},
		TotalCount: 5,
	}, nil)
	svc := NewAnnouncementService(nil, nil, enrollRepo)
	extras := newFakeExtras()
	extras.sections[10] = []uint{7, 8}
	svc.SetExtras(extras)
	n := &commsRecordingNotifier{}
	svc.SetNotifier(n)
	cid := uint(5)
	svc.notifyPublished(context.Background(), &models.Announcement{ID: 10, CourseID: &cid, UserID: 40, Title: "t", TargetAudience: "sections"})
	assert.ElementsMatch(t, []uint{41, 50, 51}, n.users, "only members of sections 7 and 8 are notified")
}

func TestAnnouncementValidateTarget(t *testing.T) {
	svc := NewAnnouncementService(nil, nil, nil)
	extras := newFakeExtras()
	extras.inCourse[5] = []uint{7, 8}
	svc.SetExtras(extras)
	cid := uint(5)
	ctx := context.Background()

	got, err := svc.validateTarget(ctx, &models.Announcement{CourseID: &cid, TargetAudience: "sections"}, []uint{7, 8, 7}, true)
	require.NoError(t, err)
	assert.Equal(t, []uint{7, 8}, got)

	_, err = svc.validateTarget(ctx, &models.Announcement{CourseID: &cid, TargetAudience: "sections"}, []uint{7, 99}, true)
	assert.Error(t, err, "a section of another course is refused")
	_, err = svc.validateTarget(ctx, &models.Announcement{CourseID: &cid, TargetAudience: "sections"}, nil, true)
	assert.Error(t, err, "section targeting needs sections")
	_, err = svc.validateTarget(ctx, &models.Announcement{CourseID: &cid, TargetAudience: "section:99"}, nil, true)
	assert.Error(t, err, "legacy single-section target is parent-tied too")
	_, err = svc.validateTarget(ctx, &models.Announcement{CourseID: &cid, TargetAudience: "everyone"}, nil, true)
	assert.Error(t, err)
	aid := uint(1)
	_, err = svc.validateTarget(ctx, &models.Announcement{AccountID: &aid, TargetAudience: "sections"}, []uint{7}, true)
	assert.Error(t, err, "account announcements have no sections")
}

func TestAnnouncementViewer_SectionTargetVisibility(t *testing.T) {
	extrasSections := []uint{7, 8}
	student7 := repoPostgres.AnnouncementViewer{Audiences: []string{"students"}, SectionIDs: []uint{7}}
	student9 := repoPostgres.AnnouncementViewer{Audiences: []string{"students"}, SectionIDs: []uint{9}}
	assert.True(t, viewerMatchesAudience(student7, "sections", extrasSections))
	assert.False(t, viewerMatchesAudience(student9, "sections", extrasSections))
}

func TestAnnouncementReplies(t *testing.T) {
	svc := NewAnnouncementService(nil, nil, nil)
	extras := newFakeExtras()
	svc.SetExtras(extras)
	n := &commsRecordingNotifier{}
	svc.SetNotifier(n)
	cid := uint(5)
	ctx := context.Background()
	a := &models.Announcement{ID: 10, CourseID: &cid, UserID: 40, Title: "Quiz moved", AllowComments: false}

	assert.ErrorIs(t, svc.CanReply(a, false), ErrRepliesDisabled)
	assert.NoError(t, svc.CanReply(a, true), "instructors can always reply")
	a.AllowComments = true
	assert.NoError(t, svc.CanReply(a, false))

	r, err := svc.CreateReply(ctx, a, 50, "<p>Thanks</p>", nil)
	require.NoError(t, err)
	assert.Equal(t, []uint{40}, n.users, "the author is notified of a reply")

	_, err = svc.CreateReply(ctx, a, 40, "<p>You're welcome</p>", &r.ID)
	require.NoError(t, err)
	assert.Equal(t, []uint{40}, n.users, "the author is not notified of their own reply")

	other := &models.Announcement{ID: 11, CourseID: &cid, UserID: 40, Title: "Other", AllowComments: true}
	_, err = svc.CreateReply(ctx, other, 50, "x", &r.ID)
	assert.ErrorIs(t, err, ErrReplyNotFound, "a parent from another announcement is refused")
	_, err = svc.CreateReply(ctx, a, 50, "   ", nil)
	assert.Error(t, err)

	got, err := svc.GetReply(ctx, 10, r.ID)
	require.NoError(t, err)
	require.NoError(t, svc.DeleteReply(ctx, got))
	_, err = svc.GetReply(ctx, 10, r.ID)
	assert.ErrorIs(t, err, ErrReplyNotFound)
}
