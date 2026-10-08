package handlers_test

// DB-backed content-lane regression tests. Gated on PARITY_DB_URL /
// DATABASE_URL; each test runs against its own freshly migrated scratch
// database, so they no-op (skip) without Postgres and are safe to run in
// parallel with other suites.

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pgdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/db"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

func contentLaneDB(t *testing.T) *gorm.DB {
	t.Helper()
	base := os.Getenv("PARITY_DB_URL")
	if base == "" {
		base = os.Getenv("DATABASE_URL")
	}
	if base == "" {
		t.Skip("set PARITY_DB_URL (or DATABASE_URL) to run DB-backed handler tests")
	}
	swap := func(name string) string {
		u, err := url.Parse(base)
		require.NoError(t, err)
		u.Path = "/" + name
		return u.String()
	}
	admin, err := sql.Open("postgres", swap("postgres"))
	require.NoError(t, err)
	name := fmt.Sprintf("paper_lms_contentlane_%d", time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err = admin.ExecContext(ctx, fmt.Sprintf(`CREATE DATABASE %q`, name))
	require.NoError(t, err)

	bs, err := sql.Open("postgres", swap(name))
	require.NoError(t, err)
	_, err = bs.Exec(`CREATE EXTENSION IF NOT EXISTS vector`)
	_ = bs.Close()
	require.NoError(t, err)

	g, err := gorm.Open(pgdriver.Open(swap(name)), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.MigrateUp(g))

	t.Cleanup(func() {
		if raw, err := g.DB(); err == nil {
			_ = raw.Close()
		}
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer dropCancel()
		_, _ = admin.ExecContext(dropCtx, fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, name))
		_ = admin.Close()
	})
	return g
}

func seedContentAccounts(t *testing.T, g *gorm.DB, ids ...uint) {
	t.Helper()
	for _, id := range ids {
		require.NoError(t, g.Exec(
			`INSERT INTO accounts (id, name, workflow_state, mfa_policy, default_locale, tenant_mode, max_upload_size_mb)
			 VALUES (?, ?, 'active', 'off', 'en', 'higher_ed', 500) ON CONFLICT (id) DO NOTHING`,
			id, fmt.Sprintf("District %d", id)).Error)
	}
}

func seedContentCourse(t *testing.T, g *gorm.DB, accountID uint, name string) *models.Course {
	t.Helper()
	c := &models.Course{AccountID: accountID, Name: name, CourseCode: name, WorkflowState: models.CourseAvailable}
	require.NoError(t, postgres.NewCourseRepository(g).Create(context.Background(), c))
	return c
}

func seedContentAssignment(t *testing.T, g *gorm.DB, courseID uint, name string) *models.Assignment {
	t.Helper()
	a := &models.Assignment{CourseID: courseID, Name: name, WorkflowState: models.AssignmentPublished, Published: true}
	require.NoError(t, postgres.NewAssignmentRepository(g).Create(context.Background(), a))
	return a
}

// B141: replacing or deleting a conditional-release rule is tied to the
// URL's course — a rule id from another course (or tenant) is a 404 and
// the rule survives.
func TestMasteryPathRule_OtherCourse_404(t *testing.T) {
	g := contentLaneDB(t)
	seedContentAccounts(t, g, 1, 2)
	mine := seedContentCourse(t, g, 1, "BIO-1")
	theirs := seedContentCourse(t, g, 2, "CHEM-1")
	trigger := seedContentAssignment(t, g, theirs.ID, "Unit test")
	a1 := seedContentAssignment(t, g, theirs.ID, "Remedial")
	a2 := seedContentAssignment(t, g, theirs.ID, "Enrichment")

	repo := postgres.NewMasteryPathRepository(g)
	svc := service.NewMasteryPathService(repo, postgres.NewSubmissionRepository(g), postgres.NewAssignmentRepository(g))
	rule, err := svc.CreateRule(context.Background(), theirs.ID, trigger.ID, []service.RangeInput{
		{LowerBound: 0, UpperBound: 70, AssignmentIDs: []uint{a1.ID}},
		{LowerBound: 70, UpperBound: 100, AssignmentIDs: []uint{a2.ID}},
	})
	require.NoError(t, err)

	h := handlers.NewMasteryPathHandler(svc)
	app := testutil.SetupTestApp()
	stub := contentAuthStub(1, "TeacherEnrollment", false)
	app.Put("/courses/:course_id/mastery_paths/rules/:rule_id", stub, h.ReplaceRule)
	app.Delete("/courses/:course_id/mastery_paths/rules/:rule_id", stub, h.DeleteRule)

	path := fmt.Sprintf("/courses/%d/mastery_paths/rules/%d", mine.ID, rule.ID)
	resp := testutil.MakeRequest(app, http.MethodDelete, path, nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp = testutil.MakeRequest(app, http.MethodPut, path, testutil.JSONBody(map[string]any{
		"scoring_ranges": []map[string]any{{"lower_bound": 0, "upper_bound": 50}, {"lower_bound": 50, "upper_bound": 100}},
	}))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	still, err := svc.GetRule(context.Background(), rule.ID)
	require.NoError(t, err)
	assert.Equal(t, "active", still.WorkflowState)
	assert.Len(t, still.ScoringRanges, 2)

	// A rule may not release (or trigger on) another course's assignments.
	mineTrigger := seedContentAssignment(t, g, mine.ID, "Quiz 1")
	_, err = svc.CreateRule(context.Background(), mine.ID, mineTrigger.ID, []service.RangeInput{
		{LowerBound: 0, UpperBound: 70, AssignmentIDs: []uint{a1.ID}},
		{LowerBound: 70, UpperBound: 100, AssignmentIDs: []uint{a2.ID}},
	})
	assert.ErrorIs(t, err, service.ErrMasteryPathForeignAssignment)
}
