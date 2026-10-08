package main

import (
	"gorm.io/gorm"

	v1 "github.com/EduThemes/paper-lms/internal/api/v1"
	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
)

// pagesFilesDeps is what the pages / files / course-shell lane needs from
// main's dependency graph.
type pagesFilesDeps struct {
	database       *gorm.DB
	pageRepo       repository.PageRepository
	courseRepo     repository.CourseRepository
	pageHandler    *handlers.PageHandler
	fileService    *service.FileService
	fileHandler    *handlers.FileHandler
	settingsLookup service.SettingsLookupFunc
	userHandler    *handlers.UserHandler
	authz          *handlers.ResourceAuthorizer
	permMiddleware *middleware.PermissionMiddleware
	courseHandler  *handlers.CourseHandler
}

// wirePagesFiles builds the lane's services and handlers, attaches the
// optional hooks to the core handlers, and returns the Router block.
func wirePagesFiles(d pagesFilesDeps) *v1.PagesFilesRoutes {
	// Page history (G065).
	revisionRepo := postgres.NewWikiPageRevisionRepository(d.database)
	revisionService := service.NewWikiPageRevisionService(revisionRepo, d.pageRepo)
	d.pageHandler.SetRevisionService(revisionService)

	// File tree, visibility and quotas (G070-G072). Course quotas share
	// the storage_quotas table with the account-level quota admin (G120).
	d.fileService.SetTreeRepository(postgres.NewCourseFilesRepository(d.database))
	quotas := service.NewStorageQuotaService(d.database)
	d.fileHandler.SetQuota(quotas)
	if d.courseHandler != nil {
		d.courseHandler.SetStorageQuotas(quotas)
	}

	// Student View (G076). The course gates let a Student View session
	// into an unpublished course.
	studentView := handlers.NewStudentViewHandler(service.NewStudentViewService(d.database), d.courseRepo, d.userHandler)
	d.permMiddleware.SetStudentViewCheck(studentView.IsTestStudentOf)
	d.authz.SetStudentViewCheck(studentView.IsTestStudentOf)

	// Course publish gating (G077): a course with graded student work
	// can't be unpublished.
	if d.courseHandler != nil {
		d.courseHandler.SetUnpublishGuard(service.NewCoursePublishService(d.database).HasGradedStudentWork)
	}

	return &v1.PagesFilesRoutes{
		Revisions:      handlers.NewWikiPageRevisionHandler(d.pageHandler, revisionService),
		FileManagement: true,
		StudentView:    studentView,
	}
}
