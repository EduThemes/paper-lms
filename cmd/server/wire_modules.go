package main

import (
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
)

// modulesLaneDeps are the already-built services and repositories the
// modules lane plugs into.
type modulesLaneDeps struct {
	ModuleService     *service.ModuleService
	ModuleHandler     *handlers.ModuleHandler
	ModuleItemHandler *handlers.ModuleItemHandler
	AssignmentService *service.AssignmentService
	MasteryPaths      *service.MasteryPathService
	CoursePaces       *service.CoursePaceService
	QuizRepo          repository.QuizRepository
	TopicRepo         repository.DiscussionTopicRepository
	AttachmentRepo    repository.AttachmentRepository
	ToolRepo          repository.ContextExternalToolRepository
	DevKeyRepo        repository.DeveloperKeyRepository
	CourseRepo        repository.CourseRepository
	IMSCCParser       *service.IMSCCParser
}

// wireModulesLane builds the module progression engine (G061/G062) and
// connects the modules-lane features: Mastery Paths visibility (G080),
// File / External Tool module items (G064) and pacing blackout dates,
// quizzes and discussions (G079). It returns the progression handler
// for the router (content gate + progress routes).
func wireModulesLane(database *gorm.DB, d modulesLaneDeps) *handlers.ModuleProgressionHandler {
	progression := service.NewModuleProgressionService(postgres.NewModuleProgressionRepository(database))
	if d.MasteryPaths != nil {
		progression.SetAssignmentVisibility(d.MasteryPaths)
		d.AssignmentService.SetStudentVisibility(d.MasteryPaths)
	}
	d.ModuleHandler.SetProgressionService(progression)
	d.ModuleItemHandler.SetProgressionService(progression)
	d.ModuleService.SetLinkRepos(d.AttachmentRepo, d.ToolRepo, d.DevKeyRepo, d.CourseRepo)
	d.CoursePaces.SetPacingRepos(postgres.NewBlackoutDateRepository(database), d.QuizRepo, d.TopicRepo)
	if d.IMSCCParser != nil {
		// G068: imported module prerequisites.
		d.IMSCCParser.SetPrerequisiteRepo(postgres.NewModulePrerequisiteRepository(database))
	}
	return handlers.NewModuleProgressionHandler(progression)
}
