package service

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// validMigrationTypes lists the accepted migration_type values.
var validMigrationTypes = map[string]bool{
	"common_cartridge":  true,
	"canvas_cartridge":  true,
	"course_copy":       true,
	"qti_converter":     true,
	"zip_file_importer": true,
}

type ContentMigrationService struct {
	migrationRepo repository.ContentMigrationRepository
}

func NewContentMigrationService(migrationRepo repository.ContentMigrationRepository) *ContentMigrationService {
	return &ContentMigrationService{migrationRepo: migrationRepo}
}

func (s *ContentMigrationService) CreateMigration(ctx context.Context, migration *models.ContentMigration) error {
	if migration.CourseID == 0 {
		return errors.New("course_id is required")
	}
	if migration.MigrationType == "" {
		return errors.New("migration_type is required")
	}
	if !validMigrationTypes[migration.MigrationType] {
		return errors.New("invalid migration_type")
	}
	if migration.WorkflowState == "" {
		migration.WorkflowState = "created"
	}
	migration.Progress = 0
	return s.migrationRepo.Create(ctx, migration)
}

func (s *ContentMigrationService) GetMigration(ctx context.Context, id uint) (*models.ContentMigration, error) {
	return s.migrationRepo.FindByID(ctx, id)
}

func (s *ContentMigrationService) UpdateMigration(ctx context.Context, migration *models.ContentMigration) error {
	return s.migrationRepo.Update(ctx, migration)
}

func (s *ContentMigrationService) ListMigrations(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.ContentMigration], error) {
	return s.migrationRepo.ListByCourseID(ctx, courseID, params)
}
