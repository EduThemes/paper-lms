package service_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/db"
)

// migrateDownTo rolls the migration chain back one step at a time until
// the applied version is at or below target. Migration numbers are not
// contiguous (parallel work reserves number blocks), so "latest - N"
// steps does not land on a given version.
func migrateDownTo(t *testing.T, g *gorm.DB, target uint) {
	t.Helper()
	for i := 0; i < 1000; i++ {
		v, dirty, err := db.MigrateVersion(g)
		require.NoError(t, err)
		require.False(t, dirty)
		if v <= target {
			return
		}
		require.NoError(t, db.MigrateDown(g, 1))
	}
	t.Fatalf("could not migrate down to %d", target)
}
