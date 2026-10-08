package service_test

// F27: 000073 must apply on a database where AutoMigrate already made
// one_roster_sync_logs.errors a bigint (it used to abort with
// "operator does not exist: bigint ~ unknown" and leave the chain dirty),
// and still convert the legacy text column.

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/db"
)

func TestMigration000073_IdempotentOnBigintColumn(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()

	dataType := func() string {
		var dt string
		require.NoError(t, g.Raw(`SELECT data_type FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = 'one_roster_sync_logs' AND column_name = 'errors'`).Scan(&dt).Error)
		return dt
	}
	version := func() uint {
		v, dirty, err := db.MigrateVersion(g)
		require.NoError(t, err)
		require.False(t, dirty)
		return v
	}
	latest := version()

	// Roll back to 72: the column is text again.
	require.NoError(t, db.MigrateDown(g, migrationsAbove(t, 72)))
	require.Equal(t, uint(72), version())
	assert.Equal(t, "text", dataType())

	// AutoMigrate-style drift: the column is already bigint.
	require.NoError(t, g.Exec(`ALTER TABLE one_roster_sync_logs ALTER COLUMN errors TYPE bigint USING 0`).Error)
	require.NoError(t, db.MigrateUp(g))
	assert.Equal(t, latest, version())
	assert.Equal(t, "bigint", dataType())

	// The legacy text column still converts, with an oversized number -> 0.
	require.NoError(t, db.MigrateDown(g, migrationsAbove(t, 72)))
	require.NoError(t, g.Exec(`INSERT INTO one_roster_sync_logs (connection_id, sync_type, status, errors) VALUES (1, 'full', 'failed', '123456789012345678901234')`).Error)
	require.NoError(t, db.MigrateUp(g))
	assert.Equal(t, "bigint", dataType())
	var errs int64
	require.NoError(t, g.Raw(`SELECT errors FROM one_roster_sync_logs LIMIT 1`).Scan(&errs).Error)
	assert.Equal(t, int64(0), errs)
}

// migrationsAbove counts the up migrations numbered above v. Migration
// numbers are not contiguous (lanes are assigned number blocks), so the
// step count to roll back to v is not latest-v.
func migrationsAbove(t *testing.T, v uint) int {
	t.Helper()
	entries, err := os.ReadDir("../db/migrations")
	require.NoError(t, err)
	n := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") || len(name) < 6 {
			continue
		}
		if num, convErr := strconv.Atoi(name[:6]); convErr == nil && uint(num) > v {
			n++
		}
	}
	return n
}
