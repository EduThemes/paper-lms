package main

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
)

// contentMigrationLane is the content-migration lane's wiring: course
// content exports, the course-copy worker, blueprint locking and the
// scheduler sweeps that pick up queued work after a restart.
type contentMigrationLane struct {
	exportHandler *handlers.ContentExportHandler
	lockGuard     fiber.Handler
	// Minute-scheduler jobs.
	migrationSweep func(ctx context.Context) error
	exportSweep    func(ctx context.Context) error
}

// wireContentMigrationLane builds the lane's services over the shared
// database and hooks them into the existing handlers. Repositories are
// stateless wrappers over the *gorm.DB, so the lane builds its own.
func wireContentMigrationLane(
	database *gorm.DB,
	fileService *service.FileService,
	fileStoragePath string,
	parser *service.IMSCCParser,
	blueprints *service.BlueprintService,
	commons *service.CommonsService,
	migrationHandler *handlers.ContentMigrationHandler,
	importHandler *handlers.ContentImportHandler,
) contentMigrationLane {
	courses := postgres.NewCourseRepository(database)
	enrollments := postgres.NewEnrollmentRepository(database)
	assignments := postgres.NewAssignmentRepository(database)
	groups := postgres.NewAssignmentGroupRepository(database)
	quizzes := postgres.NewQuizRepository(database)
	questions := postgres.NewQuizQuestionRepository(database)
	pages := postgres.NewPageRepository(database)
	discussions := postgres.NewDiscussionTopicRepository(database)
	modules := postgres.NewModuleRepository(database)
	items := postgres.NewModuleItemRepository(database)
	events := postgres.NewCalendarEventRepository(database)
	rubrics := postgres.NewRubricRepository(database)
	rubricAssocs := postgres.NewRubricAssociationRepository(database)
	attachments := postgres.NewAttachmentRepository(database)

	exporter := service.NewIMSCCExporter(
		courses, modules, items, pages, assignments, quizzes, questions, discussions,
		groups, rubrics, rubricAssocs,
		postgres.NewLearningOutcomeGroupRepository(database), postgres.NewLearningOutcomeRepository(database),
		events, attachments, fileService,
	)
	exports := service.NewContentExportService(postgres.NewContentExportRepository(database), exporter, fileService.StorageBackend(), "")

	copier := service.NewCourseCopyService(service.CourseCopyDeps{
		Courses: courses, AssignmentGroups: groups, Assignments: assignments,
		Quizzes: quizzes, Questions: questions, QuestionGroups: postgres.NewQuizQuestionGroupRepository(database),
		Pages: pages, Discussions: discussions, Modules: modules, ModuleItems: items,
		Events: events, Rubrics: rubrics, RubricAssocs: rubricAssocs,
		Folders: postgres.NewFolderRepository(database), Attachments: attachments, Files: fileService,
	})
	runner := service.NewContentMigrationRunner(
		postgres.NewContentMigrationRepository(database), postgres.NewContentMigrationQueue(database),
		courses, copier, parser,
		service.ContentDateRepos{Assignments: assignments, Quizzes: quizzes, Discussions: discussions, Modules: modules, Events: events},
	)
	migrationHandler.SetCourseCopy(handlers.ContentMigrationDeps{
		Runner: runner, Copier: copier, Courses: courses, Enrollments: enrollments, StoragePath: fileStoragePath,
	})
	importHandler.SetPostProcessor(runner)

	blueprints.SetLockRepo(postgres.NewBlueprintContentLockRepository(database))
	blueprints.SetSyncDeps(groups, attachments, fileService)
	commons.SetFileDeps(postgres.NewQuizQuestionGroupRepository(database), attachments, fileService)

	return contentMigrationLane{
		exportHandler: handlers.NewContentExportHandler(exports),
		lockGuard: handlers.NewBlueprintLockGuard(handlers.BlueprintLockGuardDeps{
			Blueprints: blueprints, Pages: pages, Courses: courses, Enrollments: enrollments,
		}),
		migrationSweep: runner.ProcessPending,
		exportSweep:    exports.ProcessPending,
	}
}
