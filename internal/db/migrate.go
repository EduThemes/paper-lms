package db

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"gorm.io/gorm"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// RunMigrations applies all pending SQL migrations from the embedded migrations directory.
// Returns the number of migrations applied, or an error.
func RunMigrations(db *gorm.DB) (int, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return 0, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	return runMigrationsWithDB(sqlDB)
}

func runMigrationsWithDB(sqlDB *sql.DB) (int, error) {
	subFS, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return 0, fmt.Errorf("failed to create sub filesystem: %w", err)
	}

	source, err := iofs.New(subFS, ".")
	if err != nil {
		return 0, fmt.Errorf("failed to create migration source: %w", err)
	}

	driver, err := postgres.WithInstance(sqlDB, &postgres.Config{})
	if err != nil {
		return 0, fmt.Errorf("failed to create migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return 0, fmt.Errorf("failed to create migrator: %w", err)
	}

	// Get current version before migration
	versionBefore, _, _ := m.Version()

	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return 0, fmt.Errorf("migration failed: %w", err)
	}

	versionAfter, _, _ := m.Version()

	applied := int(versionAfter - versionBefore)
	if err == migrate.ErrNoChange {
		applied = 0
	}

	return applied, nil
}

// MigrateDown rolls back the last N migrations. If steps is 0, rolls back all.
func MigrateDown(db *gorm.DB, steps int) error {
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

	if steps == 0 {
		return m.Down()
	}
	return m.Steps(-steps)
}

// MigrateVersion returns the current migration version.
func MigrateVersion(db *gorm.DB) (uint, bool, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return 0, false, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	subFS, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return 0, false, fmt.Errorf("failed to create sub filesystem: %w", err)
	}

	source, err := iofs.New(subFS, ".")
	if err != nil {
		return 0, false, fmt.Errorf("failed to create migration source: %w", err)
	}

	driver, err := postgres.WithInstance(sqlDB, &postgres.Config{})
	if err != nil {
		return 0, false, fmt.Errorf("failed to create migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return 0, false, fmt.Errorf("failed to create migrator: %w", err)
	}

	version, dirty, err := m.Version()
	return version, dirty, err
}

// MigrateForce forces the migration version without running migrations.
// Used for baselining existing databases.
func MigrateForce(db *gorm.DB, version int) error {
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

	return m.Force(version)
}

// MigrateUp runs all pending migrations and logs the result.
func MigrateUp(db *gorm.DB) error {
	applied, err := RunMigrations(db)
	if err != nil {
		return err
	}
	if applied > 0 {
		log.Printf("Applied %d migration(s)", applied)
	} else {
		log.Println("Database schema is up to date")
	}
	return nil
}
