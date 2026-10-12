package postgres_test

// Integration tests for the notify-inbox lane: inbox state and restricted
// replies (G092), the address book (G093), notification policies (G091)
// and announcement replies / section targeting (G090). Gated on
// PARITY_DB_URL / DATABASE_URL via freshDB. Example data only.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
)

func seedInboxUser(t *testing.T, g *gorm.DB, id, accountID uint, name string) {
	t.Helper()
	u := &models.User{ID: id, AccountID: accountID, Name: name, SortableName: name,
		LoginID: fmt.Sprintf("user%d@example.test", id), Email: fmt.Sprintf("user%d@example.test", id), PasswordHash: "x", Role: "user"}
	require.NoError(t, postgres.NewUserRepository(g).Create(context.Background(), u))
}

func page(n int) repository.PaginationParams { return repository.PaginationParams{Page: 1, PerPage: n} }

func TestInboxRepository_StateRepliesAndVisibility(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	seedCommsAccount(t, g, 1)
	seedCommsAccount(t, g, 2)
	for _, u := range []struct {
		id, acct uint
		name     string
	}{{101, 1, "Ada Teacher"}, {102, 1, "Ben Student"}, {103, 1, "Cy Student"}} {
		seedInboxUser(t, g, u.id, u.acct, u.name)
	}
	course := seedCommsCourse(t, g, 1, "INBOX-1")

	convSvc := service.NewConversationService(postgres.NewConversationRepository(g), postgres.NewConversationParticipantRepository(g), postgres.NewConversationMessageRepository(g))
	inbox := postgres.NewInboxRepository(g)

	conv := &models.Conversation{Subject: "Project groups", CreatedByUserID: 101, ContextType: "Course", ContextID: course.ID}
	require.NoError(t, convSvc.CreateConversation(ctx, conv, []uint{102, 103}))

	post := func(sender uint, body string, onlyTo []uint) *models.ConversationMessage {
		m := &models.ConversationMessage{ConversationID: conv.ID, UserID: sender, Body: body}
		require.NoError(t, convSvc.CreateMessage(ctx, m))
		require.NoError(t, inbox.AddMessageRecipients(ctx, m.ID, onlyTo))
		require.NoError(t, inbox.OnNewMessage(ctx, conv.ID, sender, onlyTo))
		return m
	}
	m1 := post(101, "Groups are posted", nil)

	n, err := inbox.UnreadCount(ctx, 102, 1)
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)
	n, _ = inbox.UnreadCount(ctx, 101, 1)
	assert.EqualValues(t, 0, n, "the sender's own copy is read")

	rows, total, err := inbox.ListForUser(ctx, 102, 1, postgres.InboxFilter{Scope: postgres.InboxScopeUnread}, page(10))
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	assert.Equal(t, "unread", rows[0].ParticipantState)
	assert.EqualValues(t, 1, rows[0].MessageCount)
	assert.Equal(t, "Groups are posted", rows[0].LastMessage)
	assert.Equal(t, "Course", rows[0].ContextType)
	assert.Equal(t, "Course INBOX-1", rows[0].ContextName)

	// Ben replies to Ada only: Cy must not see it.
	m2 := post(102, "Can I switch groups?", []uint{101})
	vis, err := inbox.ListVisibleMessages(ctx, conv.ID, 103, page(50))
	require.NoError(t, err)
	assert.EqualValues(t, 1, vis.TotalCount, "a restricted reply is hidden from other participants")
	vis, _ = inbox.ListVisibleMessages(ctx, conv.ID, 101, page(50))
	assert.EqualValues(t, 2, vis.TotalCount)
	recips, err := inbox.RecipientsForMessages(ctx, []uint{m1.ID, m2.ID})
	require.NoError(t, err)
	assert.Equal(t, []uint{101}, recips[m2.ID])
	_, restricted := recips[m1.ID]
	assert.False(t, restricted)

	// Attachments follow message visibility.
	att := &models.Attachment{ContextType: "User", ContextID: 102, UserID: 102, DisplayName: "notes.pdf", Filename: "notes.pdf", ContentType: "application/pdf", Size: 10, StoragePath: "User/102/x/notes.pdf"}
	require.NoError(t, g.Create(att).Error)
	require.NoError(t, inbox.LinkAttachments(ctx, m2.ID, []uint{att.ID}))
	ok, err := inbox.CanSeeAttachment(ctx, conv.ID, 101, att.ID)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, _ = inbox.CanSeeAttachment(ctx, conv.ID, 103, att.ID)
	assert.False(t, ok, "the attachment of a reply addressed to someone else is not downloadable")
	atts, err := inbox.AttachmentsForMessages(ctx, []uint{m2.ID})
	require.NoError(t, err)
	require.Len(t, atts[m2.ID], 1)
	assert.Equal(t, "notes.pdf", atts[m2.ID][0].DisplayName)

	// Read / star / archive affect only the caller.
	_, err = inbox.UpdateParticipants(ctx, 103, []uint{conv.ID}, postgres.InboxEventMarkRead)
	require.NoError(t, err)
	n, _ = inbox.UnreadCount(ctx, 103, 1)
	assert.EqualValues(t, 0, n)
	_, _ = inbox.UpdateParticipants(ctx, 103, []uint{conv.ID}, postgres.InboxEventStar)
	_, total, _ = inbox.ListForUser(ctx, 103, 1, postgres.InboxFilter{Scope: postgres.InboxScopeStarred}, page(10))
	assert.EqualValues(t, 1, total)
	_, total, _ = inbox.ListForUser(ctx, 102, 1, postgres.InboxFilter{Scope: postgres.InboxScopeStarred}, page(10))
	assert.EqualValues(t, 0, total, "starring is per participant")
	_, _ = inbox.UpdateParticipants(ctx, 103, []uint{conv.ID}, postgres.InboxEventArchive)
	_, total, _ = inbox.ListForUser(ctx, 103, 1, postgres.InboxFilter{}, page(10))
	assert.EqualValues(t, 0, total, "archived conversations leave the inbox")
	_, total, _ = inbox.ListForUser(ctx, 103, 1, postgres.InboxFilter{Scope: postgres.InboxScopeArchived}, page(10))
	assert.EqualValues(t, 1, total)
	// Updating a conversation you are not in changes nothing.
	changed, err := inbox.UpdateParticipants(ctx, 999, []uint{conv.ID}, postgres.InboxEventStar)
	require.NoError(t, err)
	assert.EqualValues(t, 0, changed)

	// Sent scope.
	_, total, _ = inbox.ListForUser(ctx, 102, 1, postgres.InboxFilter{Scope: postgres.InboxScopeSent}, page(10))
	assert.EqualValues(t, 1, total)
	_, total, _ = inbox.ListForUser(ctx, 103, 1, postgres.InboxFilter{Scope: postgres.InboxScopeSent}, page(10))
	assert.EqualValues(t, 0, total)

	// Context filter and tenant scope.
	_, total, _ = inbox.ListForUser(ctx, 101, 1, postgres.InboxFilter{ContextType: "Course", ContextID: course.ID}, page(10))
	assert.EqualValues(t, 1, total)
	_, total, _ = inbox.ListForUser(ctx, 101, 1, postgres.InboxFilter{ContextType: "Course", ContextID: course.ID + 1000}, page(10))
	assert.EqualValues(t, 0, total)
	_, total, _ = inbox.ListForUser(ctx, 101, 2, postgres.InboxFilter{}, page(10))
	assert.EqualValues(t, 0, total, "another tenant's scope sees nothing")

	// Delete for me: hidden until a new message arrives, which then shows
	// alone and unread (and leaves the archive).
	_, err = inbox.UpdateParticipants(ctx, 103, []uint{conv.ID}, postgres.InboxEventDestroy)
	require.NoError(t, err)
	_, total, _ = inbox.ListForUser(ctx, 103, 1, postgres.InboxFilter{Scope: postgres.InboxScopeArchived}, page(10))
	assert.EqualValues(t, 0, total)
	_, err = inbox.GetForUser(ctx, conv.ID, 103, 1)
	assert.Error(t, err, "a deleted conversation is gone for that participant")
	time.Sleep(10 * time.Millisecond)
	post(101, "Final groups attached", nil)
	rows, total, _ = inbox.ListForUser(ctx, 103, 1, postgres.InboxFilter{}, page(10))
	require.EqualValues(t, 1, total)
	assert.Equal(t, "unread", rows[0].ParticipantState)
	assert.EqualValues(t, 1, rows[0].MessageCount, "only messages after the deletion are visible")
	assert.False(t, rows[0].Starred)

	// Search on subject.
	_, total, _ = inbox.ListForUser(ctx, 101, 1, postgres.InboxFilter{Search: "project"}, page(10))
	assert.EqualValues(t, 1, total)
	_, total, _ = inbox.ListForUser(ctx, 101, 1, postgres.InboxFilter{Search: "100%"}, page(10))
	assert.EqualValues(t, 0, total)
}

func TestAddressBook_SharedContexts(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	seedCommsAccount(t, g, 1)
	seedCommsAccount(t, g, 2)
	seedInboxUser(t, g, 201, 1, "Ada Teacher")
	seedInboxUser(t, g, 202, 1, "Ben Student")
	seedInboxUser(t, g, 203, 1, "Cy Stranger")   // same tenant, no shared context
	seedInboxUser(t, g, 204, 2, "Dee Elsewhere") // other tenant
	seedInboxUser(t, g, 205, 1, "Eve Groupmate") // shares only a group with Ben
	course := seedCommsCourse(t, g, 1, "AB-1")
	other := seedCommsCourse(t, g, 1, "AB-2")
	section := &models.CourseSection{CourseID: course.ID, Name: "Section A", WorkflowState: "active"}
	require.NoError(t, g.Create(section).Error)
	seedCommsEnrollment(t, g, 201, course.ID, "TeacherEnrollment", nil)
	seedCommsEnrollment(t, g, 202, course.ID, "StudentEnrollment", &section.ID)
	seedCommsEnrollment(t, g, 205, other.ID, "StudentEnrollment", nil)

	otherID := other.ID
	cat := &models.GroupCategory{Name: "Projects", CourseID: &otherID, WorkflowState: "active"}
	require.NoError(t, g.Create(cat).Error)
	grp := &models.Group{GroupCategoryID: cat.ID, Name: "Team Rocket", ContextType: "Course", ContextID: other.ID, WorkflowState: "available"}
	require.NoError(t, g.Create(grp).Error)
	require.NoError(t, g.Create(&models.GroupMembership{GroupID: grp.ID, UserID: 202, WorkflowState: "accepted"}).Error)
	require.NoError(t, g.Create(&models.GroupMembership{GroupID: grp.ID, UserID: 205, WorkflowState: "accepted"}).Error)

	ab := postgres.NewRecipientSearchRepository(g)
	bad, err := ab.Unaddressable(ctx, 202, 1, []uint{201, 203, 204, 205, 202}, false)
	require.NoError(t, err)
	assert.ElementsMatch(t, []uint{203, 204}, bad, "only course peers and group peers of the same tenant are addressable")

	bad, err = ab.Unaddressable(ctx, 201, 1, []uint{203, 204}, true)
	require.NoError(t, err)
	assert.Equal(t, []uint{204}, bad, "admins reach anyone in their own account only")

	found, err := ab.SearchRecipients(ctx, 202, 1, "eve", false, 10)
	require.NoError(t, err)
	require.Len(t, found, 1, "recipient search includes group peers")
	assert.Equal(t, uint(205), found[0].ID)

	ctxs, err := ab.ListContexts(ctx, 202, 1)
	require.NoError(t, err)
	require.Len(t, ctxs, 2)
	assert.Equal(t, "Course", ctxs[0].ContextType)
	assert.False(t, ctxs[0].Instructor)
	assert.Equal(t, "Group", ctxs[1].ContextType)
	assert.Equal(t, "Team Rocket", ctxs[1].Name)

	member, instructor, err := ab.CourseRole(ctx, 201, course.ID, 1)
	require.NoError(t, err)
	assert.True(t, member)
	assert.True(t, instructor)
	member, _, _ = ab.CourseRole(ctx, 201, course.ID, 2)
	assert.False(t, member, "course of another tenant")

	f, m, cid, err := ab.GroupInfo(ctx, 202, grp.ID, 1)
	require.NoError(t, err)
	assert.True(t, f)
	assert.True(t, m)
	assert.Equal(t, other.ID, cid)
	f, _, _, _ = ab.GroupInfo(ctx, 202, grp.ID, 2)
	assert.False(t, f)

	sc, err := ab.SectionCourse(ctx, section.ID, 1)
	require.NoError(t, err)
	assert.Equal(t, course.ID, sc)

	students, err := ab.ContextMembers(ctx, 1, postgres.ContextMemberQuery{ContextType: "Course", ContextID: course.ID, EnrollmentTypes: []string{"StudentEnrollment"}})
	require.NoError(t, err)
	require.Len(t, students, 1)
	assert.Equal(t, uint(202), students[0].ID)
	inSection, _ := ab.ContextMembers(ctx, 1, postgres.ContextMemberQuery{ContextType: "Section", ContextID: section.ID})
	require.Len(t, inSection, 1)
	groupMembers, _ := ab.ContextMembers(ctx, 1, postgres.ContextMemberQuery{ContextType: "Group", ContextID: grp.ID, ExcludeUserID: 202})
	require.Len(t, groupMembers, 1)
	assert.Equal(t, uint(205), groupMembers[0].ID)
}

func TestNotificationPolicyRepository(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	seedCommsAccount(t, g, 1)
	seedInboxUser(t, g, 301, 1, "Pat Doe")
	seedInboxUser(t, g, 302, 1, "Sam Doe")
	repo := postgres.NewNotificationPolicyRepository(g)

	require.NoError(t, repo.Upsert(ctx, &models.NotificationPolicy{UserID: 301, CommunicationChannelID: 0, Category: "grading", Frequency: "immediately"}))
	require.NoError(t, repo.Upsert(ctx, &models.NotificationPolicy{UserID: 301, CommunicationChannelID: 0, Category: "grading", Frequency: "weekly"}))
	require.NoError(t, repo.Upsert(ctx, &models.NotificationPolicy{UserID: 301, CommunicationChannelID: 7, Category: "grading"}))
	require.NoError(t, repo.Upsert(ctx, &models.NotificationPolicy{UserID: 302, CommunicationChannelID: 0, Category: "grading", Frequency: "never"}))

	rows, err := repo.ListByUser(ctx, 301)
	require.NoError(t, err)
	require.Len(t, rows, 2, "upsert updates the existing cell")
	assert.Equal(t, "weekly", rows[0].Frequency)
	assert.Equal(t, "daily", rows[1].Frequency, "BeforeCreate mirrors the SQL default")

	err = g.Exec(`INSERT INTO notification_policies (user_id, communication_channel_id, category, frequency) VALUES (301, 0, 'x', 'hourly')`).Error
	assert.Error(t, err, "frequency CHECK")

	require.NoError(t, repo.Delete(ctx, 301, 0, "grading"))
	require.NoError(t, repo.DeleteByChannel(ctx, 301, 7))
	rows, _ = repo.ListByUser(ctx, 301)
	assert.Empty(t, rows)
	rows, _ = repo.ListByUser(ctx, 302)
	assert.Len(t, rows, 1, "another user's cells are untouched")
}

func TestAnnouncementExtras_RepliesAndSections(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	seedCommsAccount(t, g, 1)
	seedInboxUser(t, g, 401, 1, "Ada Teacher")
	seedInboxUser(t, g, 402, 1, "Ben Student")
	course := seedCommsCourse(t, g, 1, "ANN-X")
	other := seedCommsCourse(t, g, 1, "ANN-Y")
	s1 := &models.CourseSection{CourseID: course.ID, Name: "S1", WorkflowState: "active"}
	s2 := &models.CourseSection{CourseID: course.ID, Name: "S2", WorkflowState: "active"}
	s3 := &models.CourseSection{CourseID: other.ID, Name: "S3", WorkflowState: "active"}
	require.NoError(t, g.Create(s1).Error)
	require.NoError(t, g.Create(s2).Error)
	require.NoError(t, g.Create(s3).Error)

	annRepo := postgres.NewAnnouncementRepository(g)
	extras := postgres.NewAnnouncementExtrasRepository(g)
	svc := service.NewAnnouncementService(annRepo, postgres.NewAnnouncementReadReceiptRepository(g), postgres.NewEnrollmentRepository(g))
	svc.SetExtras(extras)

	cid := course.ID
	a := &models.Announcement{CourseID: &cid, UserID: 401, Title: "Lab moved", Message: "<p>m</p>", TargetAudience: "sections", AllowComments: true}
	assert.Error(t, svc.CreateAnnouncementWithSections(ctx, a, []uint{s1.ID, s3.ID}), "a section of another course is refused")
	a.ID = 0
	require.NoError(t, svc.CreateAnnouncementWithSections(ctx, a, []uint{s1.ID}))
	secs, err := extras.SectionsFor(ctx, []uint{a.ID})
	require.NoError(t, err)
	assert.Equal(t, []uint{s1.ID}, secs[a.ID])

	in1 := postgres.AnnouncementViewer{Audiences: []string{"students"}, SectionIDs: []uint{s1.ID}}
	in2 := postgres.AnnouncementViewer{Audiences: []string{"students"}, SectionIDs: []uint{s2.ID}}
	res, err := annRepo.ListForViewer(ctx, course.ID, in1, page(10))
	require.NoError(t, err)
	assert.EqualValues(t, 1, res.TotalCount)
	res, _ = annRepo.ListForViewer(ctx, course.ID, in2, page(10))
	assert.EqualValues(t, 0, res.TotalCount, "a student of another section does not see it")

	// Re-target to S2 on update.
	prev := a.WorkflowState
	require.NoError(t, svc.UpdateAnnouncementWithSections(ctx, a, prev, []uint{s2.ID}))
	res, _ = annRepo.ListForViewer(ctx, course.ID, in2, page(10))
	assert.EqualValues(t, 1, res.TotalCount)
	// Back to everyone clears the section list.
	a.TargetAudience = "all"
	require.NoError(t, svc.UpdateAnnouncementWithSections(ctx, a, prev, nil))
	secs, _ = extras.SectionsFor(ctx, []uint{a.ID})
	assert.Empty(t, secs[a.ID])

	r1, err := svc.CreateReply(ctx, a, 402, "<p>Which room?</p>", nil)
	require.NoError(t, err)
	_, err = svc.CreateReply(ctx, a, 401, "<p>Room 4</p>", &r1.ID)
	require.NoError(t, err)
	list, err := svc.ListReplies(ctx, a.ID, page(10))
	require.NoError(t, err)
	assert.EqualValues(t, 2, list.TotalCount)
	counts := svc.ReplyCounts(ctx, []uint{a.ID})
	assert.EqualValues(t, 2, counts[a.ID])
	require.NoError(t, svc.DeleteReply(ctx, r1))
	_, err = svc.GetReply(ctx, a.ID, r1.ID)
	assert.ErrorIs(t, err, service.ErrReplyNotFound)
}
