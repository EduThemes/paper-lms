package main

import (
	"gorm.io/gorm"

	v1 "github.com/EduThemes/paper-lms/internal/api/v1"
	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
)

// pagesFilesDeps is what the pages / files / course-shell lane needs from
// main's dependency graph.
type pagesFilesDeps struct {
	database    *gorm.DB
	pageRepo    repository.PageRepository
	pageHandler *handlers.PageHandler
}

// wirePagesFiles builds the lane's services and handlers, attaches the
// optional hooks to the core handlers, and returns the Router block.
func wirePagesFiles(d pagesFilesDeps) *v1.PagesFilesRoutes {
	revisionRepo := postgres.NewWikiPageRevisionRepository(d.database)
	revisionService := service.NewWikiPageRevisionService(revisionRepo, d.pageRepo)
	d.pageHandler.SetRevisionService(revisionService)

	return &v1.PagesFilesRoutes{
		Revisions: handlers.NewWikiPageRevisionHandler(d.pageHandler, revisionService),
	}
}
