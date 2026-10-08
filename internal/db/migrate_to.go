package db

import (
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"gorm.io/gorm"
)

// MigrateTo migrates the schema up or down to exactly the given version.
// Unlike MigrateDown(steps) it does not assume migration numbers are
// contiguous, so tests that re-apply one historical migration keep
// working when later migrations use non-adjacent numbers.
func MigrateTo(db *gorm.DB, version uint) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}
	subFS, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create sub filesystem: %w", err)
	}
	source, err := iofs.New(subFS, ".")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}
	driver, err := postgres.WithInstance(sqlDB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}
	if err := m.Migrate(version); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}
