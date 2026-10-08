package handlers_test

// DB-backed tests for the groups-conferences parity lane: group leaders,
// auto-assign, bulk creation, CSV import / export and the self-leave rule
// (G100); tenant-scoped conference reads, providers and recording
// management authz (G098). Fixture ids come from newFollowupFixture
// (audit_followup_db_test.go). All data is made up.

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

const (
	gcStudent3 = uint(23)
	gcStudent4 = uint(24)
)

// groupSetApp mounts every group route the lane touches.
func groupSetApp(f *followupFixture, userID, accountID uint) *fiber.App {
	g := f.g
	svc := service.NewGroupService(postgres.NewGroupCategoryRepository(g), postgres.NewGroupRepository(g), postgres.NewGroupMembershipRepository(g), postgres.NewEnrollmentRepository(g))
	svc.SetUserRepository(postgres.NewUserRepository(g))
	h := handlers.NewGroupHandler(svc, f.authz, nil)
	app := testutil.SetupTestApp()
	app.Use(callerStub(userID, accountID))
	app.Post("/courses/:course_id/group_categories", h.CreateGroupCategory)
	app.Get("/groups/:id", h.GetGroup)
	app.Put("/groups/:id", h.UpdateGroup)
	app.Post("/groups/:group_id/memberships", h.CreateGroupMembership)
	app.Delete("/groups/:group_id/memberships/:membership_id", h.DeleteGroupMembership)
	app.Get("/group_categories/:group_category_id/users", h.ListGroupCategoryUsers)
	app.Post("/group_categories/:group_category_id/assign_unassigned_members", h.AssignUnassignedMembers)
	app.Post("/group_categories/:group_category_id/create_groups", h.CreateGroupsInCategory)
	app.Post("/group_categories/:group_category_id/import", h.ImportGroupCategoryCSV)
	app.Get("/group_categories/:group_category_id/export", h.ExportGroupCategoryCSV)
	return app
}

// seedMoreStudents adds two more students to course 100 (four in all).
func seedMoreStudents(t *testing.T, f *followupFixture) {
	t.Helper()
	users := postgres.NewUserRepository(f.g)
	for _, u := range []*models.User{
		{ID: gcStudent3, AccountID: 1, Name: "Jo Example", SortableName: "Example, Jo", LoginID: "jo@example.test", Email: "jo@example.test", PasswordHash: "x", Role: "user"},
		{ID: gcStudent4, AccountID: 1, Name: "Lee Sample", SortableName: "Sample, Lee", LoginID: "lee@example.test", Email: "lee@example.test", PasswordHash: "x", Role: "user"},
	} {
		require.NoError(t, users.Create(context.Background(), u))
		require.NoError(t, f.g.Create(&models.Enrollment{UserID: u.ID, CourseID: fxCourseA, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: models.EnrollmentActive}).Error)
	}
}

func emptySet(t *testing.T, f *followupFixture, autoLeader, selfSignup string) *models.GroupCategory {
	t.Helper()
	cid := fxCourseA
	cat := &models.GroupCategory{CourseID: &cid, Name: "Project teams", AutoLeader: autoLeader, SelfSignup: selfSignup, WorkflowState: "active"}
	require.NoError(t, f.g.Create(cat).Error)
	return cat
}

func decodeJSON(t *testing.T, resp *http.Response, out any) {
	t.Helper()
	require.NoError(t, testutil.ParseJSONResponse(resp, out))
}

func membershipsOf(t *testing.T, f *followupFixture, categoryID uint) map[uint]uint {
	t.Helper()
	var rows []struct {
		UserID  uint
		GroupID uint
	}
	require.NoError(t, f.g.Raw(`SELECT gm.user_id, gm.group_id FROM group_memberships gm JOIN groups g ON g.id = gm.group_id
		WHERE g.group_category_id = ? AND g.workflow_state <> 'deleted'`, categoryID).Scan(&rows).Error)
	out := map[uint]uint{}
	for _, r := range rows {
		out[r.UserID] = r.GroupID
	}
	return out
}

func TestGroupSet_CreateGroupsAssignUnassignedAndAutoLeader(t *testing.T) {
	f := newFollowupFixture(t)
	seedMoreStudents(t, f)
	teacher := groupSetApp(f, fxTeacherA, 1)

	// Create a set with two groups and fill them in one call.
	resp := testutil.MakeRequest(teacher, http.MethodPost, fmt.Sprintf("/courses/%d/group_categories", fxCourseA), testutil.JSONBody(map[string]any{
		"group_category": map[string]any{"name": "Project teams", "auto_leader": "first", "create_group_count": 2, "assign_unassigned_members": true},
	}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created map[string]any
	decodeJSON(t, resp, &created)
	assert.EqualValues(t, 2, created["groups_count"])
	assert.EqualValues(t, 4, created["assigned"])
	catID := uint(created["id"].(float64))

	members := membershipsOf(t, f, catID)
	assert.Len(t, members, 4, "every student is in exactly one group")
	perGroup := map[uint]int{}
	for _, gid := range members {
		perGroup[gid]++
	}
	assert.Len(t, perGroup, 2)
	for gid, n := range perGroup {
		assert.Equal(t, 2, n, "group %d is balanced", gid)
	}

	// auto_leader=first gave every group a leader who is a member.
	var groups []models.Group
	require.NoError(t, f.g.Where("group_category_id = ?", catID).Find(&groups).Error)
	for _, g := range groups {
		require.NotNil(t, g.LeaderID, "group %s has a leader", g.Name)
		assert.Equal(t, g.ID, members[*g.LeaderID])
		assert.True(t, strings.HasPrefix(g.Name, "Project team "), g.Name)
	}

	// Nobody is left unassigned.
	resp = testutil.MakeRequest(teacher, http.MethodGet, fmt.Sprintf("/group_categories/%d/users?unassigned=true", catID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var unassigned []map[string]any
	decodeJSON(t, resp, &unassigned)
	assert.Empty(t, unassigned)

	// The leader leaving hands leadership to the remaining member.
	leader := *groups[0].LeaderID
	var m models.GroupMembership
	require.NoError(t, f.g.Where("group_id = ? AND user_id = ?", groups[0].ID, leader).First(&m).Error)
	resp = testutil.MakeRequest(teacher, http.MethodDelete, fmt.Sprintf("/groups/%d/memberships/%d", groups[0].ID, m.ID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var after models.Group
	require.NoError(t, f.g.First(&after, groups[0].ID).Error)
	require.NotNil(t, after.LeaderID)
	assert.NotEqual(t, leader, *after.LeaderID)
}

func TestGroupSet_UnassignedListAndStudentRefused(t *testing.T) {
	f := newFollowupFixture(t)
	seedMoreStudents(t, f)
	cat := emptySet(t, f, "", "")
	grp := &models.Group{GroupCategoryID: cat.ID, Name: "Team 1", ContextType: "Course", ContextID: fxCourseA}
	require.NoError(t, f.g.Create(grp).Error)
	require.NoError(t, f.g.Create(&models.GroupMembership{GroupID: grp.ID, UserID: fxStudent, WorkflowState: "accepted"}).Error)

	resp := testutil.MakeRequest(groupSetApp(f, fxTeacherA, 1), http.MethodGet, fmt.Sprintf("/group_categories/%d/users?unassigned=true", cat.ID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var unassigned []map[string]any
	decodeJSON(t, resp, &unassigned)
	ids := []uint{}
	for _, u := range unassigned {
		ids = append(ids, uint(u["id"].(float64)))
	}
	assert.ElementsMatch(t, []uint{fxClassmate, gcStudent3, gcStudent4}, ids)

	// Students and other tenants can't use the tools.
	student := groupSetApp(f, fxStudent, 1)
	assert.Equal(t, http.StatusForbidden, testutil.MakeRequest(student, http.MethodPost, fmt.Sprintf("/group_categories/%d/assign_unassigned_members", cat.ID), nil).StatusCode)
	foreign := groupSetApp(f, fxAdminB, 2)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(foreign, http.MethodGet, fmt.Sprintf("/group_categories/%d/export", cat.ID), nil).StatusCode)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(foreign, http.MethodPost, fmt.Sprintf("/group_categories/%d/assign_unassigned_members", cat.ID), nil).StatusCode)
}

func TestGroupSet_AssignWithNoGroupsIsABadRequest(t *testing.T) {
	f := newFollowupFixture(t)
	cat := emptySet(t, f, "", "")
	resp := testutil.MakeRequest(groupSetApp(f, fxTeacherA, 1), http.MethodPost, fmt.Sprintf("/group_categories/%d/assign_unassigned_members", cat.ID), nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGroupLeader_SetClearAndLeaderMayRename(t *testing.T) {
	f := newFollowupFixture(t)
	_, grp := seedCourseGroup(t, f.g) // students 21 + 22
	teacher := groupSetApp(f, fxTeacherA, 1)
	path := fmt.Sprintf("/groups/%d", grp.ID)

	// A non-member can't lead.
	resp := testutil.MakeRequest(teacher, http.MethodPut, path, testutil.JSONBody(map[string]any{"group": map[string]any{"leader_id": fxTeacherB}}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// Canvas shape: leader: {id}.
	resp = testutil.MakeRequest(teacher, http.MethodPut, path, testutil.JSONBody(map[string]any{"group": map[string]any{"leader": map[string]any{"id": fxStudent}}}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]any
	decodeJSON(t, resp, &body)
	assert.EqualValues(t, fxStudent, body["leader_id"])

	// The leader may rename the group, but not change its limits or
	// hand leadership around.
	leader := groupSetApp(f, fxStudent, 1)
	resp = testutil.MakeRequest(leader, http.MethodPut, path, testutil.JSONBody(map[string]any{"group": map[string]any{"name": "The Owls"}}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp = testutil.MakeRequest(leader, http.MethodPut, path, testutil.JSONBody(map[string]any{"group": map[string]any{"max_membership": 9}}))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp = testutil.MakeRequest(leader, http.MethodPut, path, testutil.JSONBody(map[string]any{"group": map[string]any{"leader_id": fxClassmate}}))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	// A plain member may not rename.
	resp = testutil.MakeRequest(groupSetApp(f, fxClassmate, 1), http.MethodPut, path, testutil.JSONBody(map[string]any{"group": map[string]any{"name": "Mine"}}))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	// Clear.
	resp = testutil.MakeRequest(teacher, http.MethodPut, path, testutil.JSONBody(map[string]any{"group": map[string]any{"leader_id": nil}}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var g models.Group
	require.NoError(t, f.g.First(&g, grp.ID).Error)
	assert.Nil(t, g.LeaderID)
	assert.Equal(t, "The Owls", g.Name)
}

func TestGroupMembership_StudentLeavesOnlySelfSignupGroups(t *testing.T) {
	f := newFollowupFixture(t)
	cat, grp := seedCourseGroup(t, f.g) // no self sign-up
	var m models.GroupMembership
	require.NoError(t, f.g.Where("group_id = ? AND user_id = ?", grp.ID, fxStudent).First(&m).Error)
	student := groupSetApp(f, fxStudent, 1)
	path := fmt.Sprintf("/groups/%d/memberships/%d", grp.ID, m.ID)

	assert.Equal(t, http.StatusForbidden, testutil.MakeRequest(student, http.MethodDelete, path, nil).StatusCode,
		"an instructor-assigned group can't be left")

	require.NoError(t, f.g.Model(&models.GroupCategory{}).Where("id = ?", cat.ID).Update("self_signup", "enabled").Error)
	assert.Equal(t, http.StatusOK, testutil.MakeRequest(student, http.MethodDelete, path, nil).StatusCode)
}

func multipartCSV(t *testing.T, csv string) (io.Reader, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("attachment", "groups.csv")
	require.NoError(t, err)
	_, _ = part.Write([]byte(csv))
	require.NoError(t, w.Close())
	return &buf, w.FormDataContentType()
}

func TestGroupSet_CSVImportAndExportRoundTrip(t *testing.T) {
	f := newFollowupFixture(t)
	seedMoreStudents(t, f)
	cat := emptySet(t, f, "", "")
	existing := &models.Group{GroupCategoryID: cat.ID, Name: "Team Red", ContextType: "Course", ContextID: fxCourseA}
	require.NoError(t, f.g.Create(existing).Error)
	teacher := groupSetApp(f, fxTeacherA, 1)

	csv := "login_id,canvas_user_id,group_name\n" +
		"sam@example.test,,Team Red\n" +
		",22,Team Blue\n" +
		"JO@example.test,,team blue\n" + // case-insensitive login + group name
		"nobody@example.test,,Team Red\n" + // not a student
		"terry@example.test,,Team Red\n" + // the teacher is not a student
		"sam@example.test,,Team Blue\n" // listed twice
	body, ctype := multipartCSV(t, csv)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/group_categories/%d/import", cat.ID), body)
	req.Header.Set("Content-Type", ctype)
	resp, err := teacher.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var result service.GroupImportResult
	decodeJSON(t, resp, &result)
	assert.Equal(t, 6, result.RowsProcessed)
	assert.Equal(t, 1, result.GroupsCreated)
	assert.Equal(t, 3, result.MembershipsCreated)
	assert.Equal(t, 3, result.SkippedCount)

	members := membershipsOf(t, f, cat.ID)
	assert.Equal(t, existing.ID, members[fxStudent])
	assert.Equal(t, members[fxClassmate], members[gcStudent3])
	assert.NotEqual(t, existing.ID, members[fxClassmate])
	_, has4 := members[gcStudent4]
	assert.False(t, has4)

	// Export lists every student, the unassigned one with blank group.
	resp = testutil.MakeRequest(teacher, http.MethodGet, fmt.Sprintf("/group_categories/%d/export", cat.ID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/csv")
	raw, _ := io.ReadAll(resp.Body)
	out := string(raw)
	assert.True(t, strings.HasPrefix(out, "name,canvas_user_id,user_id,login_id,email,group_name,canvas_group_id\n"), out)
	assert.Contains(t, out, "Sam Student,21,,sam@example.test,sam@example.test,Team Red,")
	assert.Contains(t, out, "Lee Sample,24,,lee@example.test,lee@example.test,,\n")
	assert.NotContains(t, out, "terry@example.test", "staff are not in the roster")

	// Bad header.
	resp = testutil.MakeRequest(teacher, http.MethodPost, fmt.Sprintf("/group_categories/%d/import", cat.ID), strings.NewReader("name,section\nx,y\n"))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ---- Conferences (G098) ----

func conferenceApp(f *followupFixture, userID, accountID uint) *fiber.App {
	svc := service.NewConferenceService(postgres.NewConferenceRepository(f.g), postgres.NewConferenceParticipantRepository(f.g))
	h := handlers.NewConferenceHandler(svc, f.authz)
	app := testutil.SetupTestApp()
	app.Use(callerStub(userID, accountID))
	app.Get("/courses/:course_id/conferences", h.ListConferences)
	app.Get("/courses/:course_id/conferences/providers", h.ListProviders)
	app.Get("/conferences/:id", h.GetConference)
	app.Post("/conferences/:id/start", h.StartConference)
	app.Post("/conferences/:id/join", h.JoinConference)
	app.Get("/conferences/:id/recordings", h.GetRecordings)
	app.Delete("/conferences/:id/recordings/:recording_id", h.DeleteRecording)
	return app
}

func TestConferences_TenantScopedReadsAndExternalLinkLifecycle(t *testing.T) {
	f := newFollowupFixture(t)
	conf := &models.Conference{ContextType: "Course", ContextID: fxCourseA, ConferenceType: "Zoom", Title: "Office hours",
		UserID: fxTeacherA, JoinURL: "https://meet.example.com/room-1", WorkflowState: "active", Recordings: "[]", Settings: "{}"}
	require.NoError(t, f.g.Create(conf).Error)

	student := conferenceApp(f, fxStudent, 1)
	resp := testutil.MakeRequest(student, http.MethodGet, fmt.Sprintf("/conferences/%d", conf.ID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode, "an enrolled student reads a conference in their tenant")
	resp = testutil.MakeRequest(student, http.MethodGet, fmt.Sprintf("/courses/%d/conferences", fxCourseA), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var list []map[string]any
	decodeJSON(t, resp, &list)
	require.Len(t, list, 1)

	foreign := conferenceApp(f, fxAdminB, 2)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(foreign, http.MethodGet, fmt.Sprintf("/conferences/%d", conf.ID), nil).StatusCode)

	// Students can't start; the external link is handed out once started.
	assert.Equal(t, http.StatusConflict, testutil.MakeRequest(student, http.MethodPost, fmt.Sprintf("/conferences/%d/join", conf.ID), nil).StatusCode)
	assert.Equal(t, http.StatusForbidden, testutil.MakeRequest(student, http.MethodPost, fmt.Sprintf("/conferences/%d/start", conf.ID), nil).StatusCode)
	teacher := conferenceApp(f, fxTeacherA, 1)
	require.Equal(t, http.StatusOK, testutil.MakeRequest(teacher, http.MethodPost, fmt.Sprintf("/conferences/%d/start", conf.ID), nil).StatusCode)
	resp = testutil.MakeRequest(student, http.MethodPost, fmt.Sprintf("/conferences/%d/join", conf.ID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var joined map[string]any
	decodeJSON(t, resp, &joined)
	assert.Equal(t, "https://meet.example.com/room-1", joined["join_url"])

	// No provider: recordings are an empty list; deleting is a conflict
	// for the teacher and forbidden for the student.
	resp = testutil.MakeRequest(student, http.MethodGet, fmt.Sprintf("/conferences/%d/recordings", conf.ID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var recs map[string]any
	decodeJSON(t, resp, &recs)
	assert.Equal(t, []any{}, recs["recordings"])
	assert.Equal(t, http.StatusForbidden, testutil.MakeRequest(student, http.MethodDelete, fmt.Sprintf("/conferences/%d/recordings/rec-1", conf.ID), nil).StatusCode)
	assert.Equal(t, http.StatusConflict, testutil.MakeRequest(teacher, http.MethodDelete, fmt.Sprintf("/conferences/%d/recordings/rec-1", conf.ID), nil).StatusCode)

	// The same tenant subquery (groups have no course_id column) guards
	// participants and collaborations.
	parts, err := postgres.NewConferenceParticipantRepository(f.g).ListByConferenceID(context.Background(), conf.ID, 1)
	require.NoError(t, err)
	assert.Len(t, parts, 1, "the student who joined is a participant")
	collab := &models.Collaboration{ContextType: "Course", ContextID: fxCourseA, Title: "Shared notes", CollaborationType: "etherpad", UserID: fxTeacherA, WorkflowState: "active"}
	require.NoError(t, f.g.Create(collab).Error)
	_, err = postgres.NewCollaborationRepository(f.g).FindByID(context.Background(), collab.ID, 1)
	require.NoError(t, err)

	resp = testutil.MakeRequest(student, http.MethodGet, fmt.Sprintf("/courses/%d/conferences/providers", fxCourseA), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var providers map[string]any
	decodeJSON(t, resp, &providers)
	assert.Len(t, providers["providers"], 2)
}
