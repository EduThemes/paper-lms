package service_test

// 000099 fails content migrations that were queued before the worker
// existed: the old API let a caller queue a course_copy of any source
// course, and the new sweep would otherwise run it unauthorized.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/db"
)

func TestMigration000099_FailsLegacyQueuedMigrations(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()

	require.NoError(t, db.MigrateTo(g, 98))
	require.NoError(t, g.Exec(`INSERT INTO content_migrations (course_id, user_id, migration_type, workflow_state, source_course_id, created_at, updated_at)
		VALUES (1, 1, 'course_copy', 'queued', 2, now(), now()),
		       (1, 1, 'common_cartridge', 'running', NULL, now(), now()),
		       (1, 1, 'course_copy', 'completed', 2, now(), now())`).Error)
	require.NoError(t, db.MigrateUp(g))

	var states []string
	require.NoError(t, g.Raw(`SELECT workflow_state FROM content_migrations ORDER BY id`).Scan(&states).Error)
	assert.Equal(t, []string{"failed", "failed", "completed"}, states)
	var queued int64
	require.NoError(t, g.Raw(`SELECT count(*) FROM content_migrations WHERE workflow_state = 'queued'`).Scan(&queued).Error)
	assert.Zero(t, queued)
}
