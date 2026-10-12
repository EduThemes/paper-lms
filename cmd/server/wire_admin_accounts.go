package main

import (
	"context"

	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/auth"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

// adminAccountsWiring carries what the account-administration lane needs
// from main (kept in one struct so main.go grows by a single call).
type adminAccountsWiring struct {
	DB                *gorm.DB
	PermMiddleware    *middleware.PermissionMiddleware
	CustomRoleService *service.CustomRoleService
	EnrollmentService *service.EnrollmentService
	UserService       *service.UserService
	LoginPipeline     *auth.LoginPipeline
	ObserverService   *service.ObserverService
	GradingService    *service.GradingService
	FileService       *service.FileService
	SISImportService  *service.SISImportService
	AuditService      *service.AuditService
	CourseRepo        repository.CourseRepository
}

// wireAdminAccounts connects the lane's enforcement hooks to the shared
// services and builds its handler:
//   - G102: custom-role permission sets on the course gates, role_id on
//     enrollment create, course-context override validation.
//   - G115: extra local logins in password sign-in; SSO logins in the
//     LoginPipeline.
//   - G111: SIS batches run off the request path; user_observers.csv.
//   - G120: storage quotas on uploads.
func wireAdminAccounts(w adminAccountsWiring) *handlers.AdminAccountsHandler {
	w.CustomRoleService.WithCourseRepo(w.CourseRepo)
	w.PermMiddleware.SetPermissionResolver(w.CustomRoleService)
	w.EnrollmentService.SetRoleResolver(w.CustomRoleService)

	logins := service.NewUserLoginService(w.DB)
	w.UserService.SetExtraLogins(logins)
	w.LoginPipeline.SetProviderLogins(logins)

	w.SISImportService.SetObserverLinker(w.ObserverService).SetAsync(true)

	quotas := service.NewStorageQuotaService(w.DB)
	w.FileService.SetQuotaChecker(quotas)

	h := handlers.NewAdminAccountsHandler(handlers.AdminAccountsDeps{
		Users:     service.NewAdminUserService(w.DB),
		Logins:    logins,
		Observers: w.ObserverService,
		Reports:   service.NewAccountReportService(w.DB, w.GradingService),
		Brand:     service.NewBrandConfigService(w.DB),
		Tools:     service.NewAdminToolsService(w.DB),
		Quotas:    quotas,
		Roles:     w.CustomRoleService,
		Audit:     w.AuditService,
	})
	return h
}

// sisResumeJob processes SIS batches left waiting (e.g. across a restart).
func sisResumeJob(s *service.SISImportService) func(ctx context.Context) error {
	return func(ctx context.Context) error { return s.ResumePendingBatches(ctx) }
}
