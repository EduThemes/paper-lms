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
	attachmentRepo repository.AttachmentRepository
	pageHandler    *handlers.PageHandler
	courseHandler  *handlers.CourseHandler
	imsccParser    *service.IMSCCParser
	commons        *service.CommonsService
	blueprints     *service.BlueprintService
	batch          *service.BatchService
	settingsLookup service.SettingsLookupFunc
	// cssPolicy resolves pages.custom_css_enabled for a course's account
	// (service.NewCourseCSSPolicy). The course copier and page history get
	// it in their own lanes.
	cssPolicy   service.CourseCSSPolicy
	frontendURL string
}

// wirePageDesigner attaches the designer's hooks to the core page and
// course handlers, the cartridge importer and the other page write paths
// (Commons import, blueprint sync, batch clone), and returns the saved
// colour scheme handler. (The cartridge exporter gets FRONTEND_URL in
// wireContentMigrationLane; the icon route is stateless.)
func wirePageDesigner(d pageDesignerDeps) *handlers.DesignColourSchemeHandler {
	d.pageHandler.SetCourseCSSPolicy(d.cssPolicy)
	d.courseHandler.SetPageDesigner(d.attachmentRepo, d.settingsLookup)
	if d.imsccParser != nil {
		policy := d.cssPolicy
		d.imsccParser.SetPageDesigner(d.frontendURL, func(ctx context.Context, courseID uint) bool {
			return policy.Allowed(ctx, courseID)
		})
	}
	if d.commons != nil {
		d.commons.SetCourseCSSPolicy(d.cssPolicy)
	}
	if d.blueprints != nil {
		d.blueprints.SetCourseCSSPolicy(d.cssPolicy)
	}
	if d.batch != nil {
		d.batch.SetCourseCSSPolicy(d.cssPolicy)
	}
	return handlers.NewDesignColourSchemeHandler(postgres.NewDesignColourSchemeRepository(d.database))
}
