package service_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/db"
)

// migrateDownTo rolls the schema back one migration at a time until it is
// at target. Migration numbers are not contiguous (parallel work reserves
// numbered blocks), so "latest - target" is not a step count.
func migrateDownTo(t *testing.T, g *gorm.DB, target uint) {
	t.Helper()
	for i := 0; i < 1000; i++ {
		v, dirty, err := db.MigrateVersion(g)
		require.NoError(t, err)
		require.False(t, dirty)
		if v <= target {
			require.Equal(t, target, v, "no migration %d to stop at", target)
			return
		}
		require.NoError(t, db.MigrateDown(g, 1))
	}
	t.Fatalf("could not roll back to migration %d", target)
}
