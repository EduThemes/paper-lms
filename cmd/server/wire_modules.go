package main

import (
	"context"

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
	// LTIService gets the module-lock launch gate; GradingPolicy supplies
	// the per-student (override-aware) assignment dates for it.
	LTIService    *service.LTIService
	GradingPolicy *service.GradingPolicyService
}

// assignmentDatesFunc adapts the grading-policy service to the LTI
// launch gate's per-student date lookup.
func assignmentDatesFunc(courses repository.CourseRepository, policy *service.GradingPolicyService) service.AssignmentDatesFunc {
	if courses == nil || policy == nil {
		return nil
	}
	return func(ctx context.Context, courseID, userID, accountID, assignmentID uint) (*service.EffectiveDueDate, bool, error) {
		course, err := courses.FindByID(ctx, courseID, accountID)
		if err != nil || course == nil {
			return nil, false, err
		}
		dates, err := policy.StudentAssignmentDates(ctx, course, userID)
		if err != nil {
			return nil, false, err
		}
		d, ok := dates[assignmentID]
		if !ok {
			return nil, false, nil
		}
		return &d, true, nil
	}
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
	if d.LTIService != nil {
		// The course tool launch route honours module locks (the module
		// item's launch_path points at it).
		d.LTIService.SetLaunchGate(service.NewModuleLaunchGate(progression, assignmentDatesFunc(d.CourseRepo, d.GradingPolicy)))
	}
	d.ModuleService.SetLinkRepos(d.AttachmentRepo, d.ToolRepo, d.DevKeyRepo, d.CourseRepo)
	d.CoursePaces.SetPacingRepos(postgres.NewBlackoutDateRepository(database), d.QuizRepo, d.TopicRepo)
	if d.IMSCCParser != nil {
		// G068: imported module prerequisites.
		d.IMSCCParser.SetPrerequisiteRepo(postgres.NewModulePrerequisiteRepository(database))
	}
	return handlers.NewModuleProgressionHandler(progression)
}
