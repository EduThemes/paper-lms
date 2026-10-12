package main

import (
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/auth"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/security"
	"github.com/EduThemes/paper-lms/internal/service"
)

// wireLTIAdvantage connects the LTI 1.3 / Advantage collaborators (LTI
// parity lane, G123-G134) to the LTI, AGS and NRPS services. Kept out of
// main.go so the lane adds a single call there.
func wireLTIAdvantage(
	database *gorm.DB,
	ltiService *service.LTIService,
	agsService *service.LTIAGSService,
	nrpsService *service.LTINRPSService,
	devKeyService *service.DeveloperKeyService,
	submissions *service.SubmissionService,
	userRepo repository.UserRepository,
	courseRepo repository.CourseRepository,
	assignmentRepo repository.AssignmentRepository,
	moduleRepo repository.ModuleRepository,
	moduleItemRepo repository.ModuleItemRepository,
	lineItemRepo repository.LTILineItemRepository,
	resultRepo repository.LTIResultRepository,
) {
	store := postgres.NewLTIAdvantageRepository(database)
	ltiService.SetAdvantageDeps(service.LTIAdvantageDeps{
		Store:          store,
		UserRepo:       userRepo,
		AssignmentRepo: assignmentRepo,
		LineItemRepo:   lineItemRepo,
		ResultRepo:     resultRepo,
		ModuleRepo:     moduleRepo,
		ModuleItemRepo: moduleItemRepo,
		OpenSecret:     auth.OpenString,
		SealSecret:     auth.SealString,
		Grader:         submissions,
		Submitter:      submissions,
		ValidateURL:    security.ValidateExternalURL,
		// Tool JWKS fetches go through the SSRF-safe dialer.
		HTTPClient: &http.Client{Transport: security.SafeTransport(), Timeout: 15 * time.Second},
	})
	ltiService.SetDeveloperKeyCreator(devKeyService)
	agsService.SetAdvantageStore(store)
	nrpsService.SetCourseRepo(courseRepo)
}
