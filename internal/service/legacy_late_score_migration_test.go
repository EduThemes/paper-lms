package service_test

// F21: rows backfilled by 000066 (entered_score = already-deducted score)
// are repaired by 000077, so re-saving a legacy late grade does not take
// the late penalty a second time. Runs against a migrated Postgres.

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/db"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
)

func TestMigration000077_RecoversEnteredScoreOfLegacyLateGrades(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	course := laneSeedCourse(t, g, 1, "LATE-1")
	require.NoError(t, g.Create(&models.LatePolicy{CourseID: course.ID, LateSubmissionDeductionEnabled: true, LateSubmissionDeduction: 20, LateSubmissionInterval: "day"}).Error)

	due := time.Now().Add(-72 * time.Hour).UTC().Truncate(time.Second)
	submitted := due.Add(20 * time.Hour) // one day late: 20%
	pts := 10.0
	a := &models.Assignment{CourseID: course.ID, Name: "Essay", PointsPossible: &pts, DueAt: &due, GradingType: "points", WorkflowState: models.AssignmentPublished, Published: true}
	require.NoError(t, g.Create(a).Error)
	student := &models.User{AccountID: 1, Name: "Sam Student", LoginID: "sam.late@example.test", Email: "sam.late@example.test"}
	require.NoError(t, g.Create(student).Error)

	// Legacy shape: the grader typed 10, the old code stored 8 and nothing
	// else; 000066 then copied 8 into entered_score.
	legacyScore, legacyGrade := 8.0, "8"
	graded := time.Now().Add(-time.Hour)
	sub := &models.Submission{AssignmentID: a.ID, UserID: student.ID, SubmittedAt: &submitted, Late: true,
		Score: &legacyScore, Grade: &legacyGrade, EnteredScore: &legacyScore, EnteredGrade: &legacyGrade,
		GradedAt: &graded, PostedAt: &graded, WorkflowState: models.SubmissionGraded}
	require.NoError(t, g.Create(sub).Error)

	// Re-apply 000077 on top of the row.
	migrateDownTo(t, g, 76)
	require.NoError(t, db.MigrateUp(g))

	var got models.Submission
	require.NoError(t, g.First(&got, sub.ID).Error)
	require.NotNil(t, got.EnteredScore)
	assert.InDelta(t, 10.0, *got.EnteredScore, 1e-9)
	require.NotNil(t, got.EnteredGrade)
	assert.Equal(t, "10", *got.EnteredGrade)
	require.NotNil(t, got.PointsDeducted)
	assert.InDelta(t, 2.0, *got.PointsDeducted, 1e-9)
	assert.InDelta(t, 8.0, *got.Score, 1e-9)

	// SpeedGrader re-posts the pre-filled entered value: the score stays 8.
	svc := service.NewSubmissionService(postgres.NewSubmissionRepository(g), postgres.NewAssignmentRepository(g),
		postgres.NewEnrollmentRepository(g), postgres.NewLatePolicyRepository(g), postgres.NewCourseRepository(g),
		postgres.NewGradingPeriodGroupRepository(g), postgres.NewGradingPeriodRepository(g), postgres.NewGroupMembershipRepository(g))
	regraded, err := svc.Grade(ctx, a.ID, student.ID, 1, 1, "10")
	require.NoError(t, err)
	assert.InDelta(t, 8.0, *regraded.Score, 1e-9, "re-saving a legacy late grade must not deduct twice")
}
