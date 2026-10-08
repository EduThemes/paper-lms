package db

import (
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"gorm.io/gorm"
)

// MigrateTo moves the schema up or down to exactly the given migration
// version. Migration numbers are not contiguous (parallel work reserves
// number blocks), so "N steps down" is not "version minus N"; tests that
// re-apply an old migration roll back to a version with this instead.
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
