package main

import (
	"context"

	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
)

// pageDesignerDeps is what the Page Designer (docs/page-designer) needs
// from main's dependency graph.
type pageDesignerDeps struct {
	database       *gorm.DB
	courseRepo     repository.CourseRepository
	attachmentRepo repository.AttachmentRepository
	pageHandler    *handlers.PageHandler
	courseHandler  *handlers.CourseHandler
	imsccParser    *service.IMSCCParser
	settingsLookup service.SettingsLookupFunc
	frontendURL    string
}

// wirePageDesigner attaches the designer's hooks to the core page and
// course handlers and the cartridge importer, and returns the saved
// colour scheme handler. (The cartridge exporter gets FRONTEND_URL in
// wireContentMigrationLane; the icon route is stateless.)
func wirePageDesigner(d pageDesignerDeps) *handlers.DesignColourSchemeHandler {
	d.pageHandler.SetSettingsLookup(d.settingsLookup)
	d.courseHandler.SetPageDesigner(d.attachmentRepo, d.settingsLookup)
	if d.imsccParser != nil {
		courses := d.courseRepo
		lookup := d.settingsLookup
		d.imsccParser.SetPageDesigner(d.frontendURL, func(ctx context.Context, courseID uint) bool {
			// accountID=0: auth-internal — the import target was tied to
			// the requesting user's tenant before the import started.
			course, err := courses.FindByID(ctx, courseID, 0)
			if err != nil || course == nil {
				return true
			}
			return service.CustomCSSEnabled(ctx, lookup, course.AccountID)
		})
	}
	return handlers.NewDesignColourSchemeHandler(postgres.NewDesignColourSchemeRepository(d.database))
}
