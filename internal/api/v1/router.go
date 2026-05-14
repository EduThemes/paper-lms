package v1

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/auth"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type Router struct {
	accountRepo            repository.AccountRepository
	userHandler            *handlers.UserHandler
	accountHandler         *handlers.AccountHandler
	courseHandler           *handlers.CourseHandler
	sectionHandler         *handlers.SectionHandler
	enrollmentHandler      *handlers.EnrollmentHandler
	moduleHandler          *handlers.ModuleHandler
	moduleItemHandler      *handlers.ModuleItemHandler
	pageHandler            *handlers.PageHandler
	assignmentHandler      *handlers.AssignmentHandler
	assignmentGroupHandler *handlers.AssignmentGroupHandler
	submissionHandler      *handlers.SubmissionHandler
	gradebookHandler       *handlers.GradebookHandler
	gradingStandardHandler *handlers.GradingStandardHandler
	developerKeyHandler    *handlers.DeveloperKeyHandler
	accessTokenHandler     *handlers.AccessTokenHandler
	oauth2Handler          *handlers.OAuth2Handler
	externalToolHandler    *handlers.ExternalToolHandler
	ltiHandler             *handlers.LTIHandler
	discussionHandler      *handlers.DiscussionHandler
	discussionEntryHandler *handlers.DiscussionEntryHandler
	fileHandler            *handlers.FileHandler
	folderHandler          *handlers.FolderHandler
	sisImportHandler       *handlers.SISImportHandler
	quizHandler               *handlers.QuizHandler
	quizQuestionHandler       *handlers.QuizQuestionHandler
	quizSubmissionHandler     *handlers.QuizSubmissionHandler
	rubricHandler             *handlers.RubricHandler
	rubricAssessmentHandler   *handlers.RubricAssessmentHandler
	gradingPeriodHandler      *handlers.GradingPeriodHandler
	assignmentOverrideHandler *handlers.AssignmentOverrideHandler
	latePolicyHandler         *handlers.LatePolicyHandler
	calendarEventHandler  *handlers.CalendarEventHandler
	conversationHandler   *handlers.ConversationHandler
	notificationHandler   *handlers.NotificationHandler
	contentMigrationHandler *handlers.ContentMigrationHandler
	learningOutcomeHandler  *handlers.LearningOutcomeHandler
	speedGraderHandler      *handlers.SpeedGraderHandler
	groupHandler         *handlers.GroupHandler
	blueprintHandler     *handlers.BlueprintHandler
	coursePaceHandler    *handlers.CoursePaceHandler
	collaborationHandler *handlers.CollaborationHandler
	conferenceHandler    *handlers.ConferenceHandler
	analyticsHandler     *handlers.AnalyticsHandler
	observerHandler      *handlers.ObserverHandler
	graphqlHandler      *handlers.GraphQLHandler
	authProviderHandler *handlers.AuthProviderHandler
	discussionV2Handler  *handlers.DiscussionV2Handler
	contentImportHandler *handlers.ContentImportHandler
	batchHandler         *handlers.BatchHandler
	ssoHandler           *auth.SSOHandler
	announcementHandler    *handlers.AnnouncementHandler
	enrollmentTermHandler  *handlers.EnrollmentTermHandler
	syllabusHandler        *handlers.SyllabusHandler
	notificationDeliveryHandler *handlers.NotificationDeliveryHandler
	auditHandler                *handlers.AuditHandler
	customRoleHandler          *handlers.CustomRoleHandler
	onerosterHandler           *handlers.OneRosterHandler
	documentAnnotationHandler  *handlers.DocumentAnnotationHandler
	coppaHandler          *handlers.COPPAHandler
	ferpaHandler          *handlers.FERPAHandler
	accommodationHandler  *handlers.AccommodationHandler
	attendanceHandler     *handlers.AttendanceHandler
	portfolioHandler      *handlers.PortfolioHandler
	// Course Home Engine
	courseHomeHandler     *handlers.CourseHomeHandler
	// Peer Reviews, Question Banks
	peerReviewHandler          *handlers.PeerReviewHandler
	questionBankHandler        *handlers.QuestionBankHandler
	// Quiz Question Groups
	quizQuestionGroupHandler   *handlers.QuizQuestionGroupHandler
	// Quiz Statistics
	quizStatisticsHandler      *handlers.QuizStatisticsHandler
	// Setup
	setupHandler               *handlers.SetupHandler
	// P3 Features
	featureFlagHandler           *handlers.FeatureFlagHandler
	customGradebookColumnHandler *handlers.CustomGradebookColumnHandler
	masteryPathHandler           *handlers.MasteryPathHandler
	appointmentGroupHandler      *handlers.AppointmentGroupHandler
	outcomeProficiencyHandler    *handlers.OutcomeProficiencyHandler
	// Pairing codes
	pairingCodeHandler         *handlers.PairingCodeHandler
	// Discussion Checkpoints, Smart Search, Commons, AI Assist
	discussionCheckpointHandler *handlers.DiscussionCheckpointHandler
	smartSearchHandler          *handlers.SmartSearchHandler
	commonsHandler              *handlers.CommonsHandler
	aiAssistHandler             *handlers.AIAssistHandler
	// Wave A2: Quiz Item Banks, Stimuli, per-question Outcome Alignments
	quizItemBankHandler         *handlers.QuizItemBankHandler
	quizStimulusHandler         *handlers.QuizStimulusHandler
	quizOutcomeAlignmentHandler *handlers.QuizOutcomeAlignmentHandler
	// Wave B: QTI / IMSCC import + export.
	qtiImportHandler *handlers.QTIImportHandler
	// Phase 6 Wave 1: gamification read API (wallet + currencies).
	gamificationHandler *handlers.GamificationHandler
	authMiddleware             *middleware.AuthMiddleware
	permMiddleware             *middleware.PermissionMiddleware
}

func NewRouter(
	userHandler *handlers.UserHandler,
	accountHandler *handlers.AccountHandler,
	courseHandler *handlers.CourseHandler,
	sectionHandler *handlers.SectionHandler,
	enrollmentHandler *handlers.EnrollmentHandler,
	moduleHandler *handlers.ModuleHandler,
	moduleItemHandler *handlers.ModuleItemHandler,
	pageHandler *handlers.PageHandler,
	assignmentHandler *handlers.AssignmentHandler,
	assignmentGroupHandler *handlers.AssignmentGroupHandler,
	submissionHandler *handlers.SubmissionHandler,
	gradebookHandler *handlers.GradebookHandler,
	gradingStandardHandler *handlers.GradingStandardHandler,
	developerKeyHandler *handlers.DeveloperKeyHandler,
	accessTokenHandler *handlers.AccessTokenHandler,
	oauth2Handler *handlers.OAuth2Handler,
	externalToolHandler *handlers.ExternalToolHandler,
	ltiHandler *handlers.LTIHandler,
	discussionHandler *handlers.DiscussionHandler,
	discussionEntryHandler *handlers.DiscussionEntryHandler,
	fileHandler *handlers.FileHandler,
	folderHandler *handlers.FolderHandler,
	sisImportHandler *handlers.SISImportHandler,
	quizHandler *handlers.QuizHandler,
	quizQuestionHandler *handlers.QuizQuestionHandler,
	quizSubmissionHandler *handlers.QuizSubmissionHandler,
	rubricHandler *handlers.RubricHandler,
	rubricAssessmentHandler *handlers.RubricAssessmentHandler,
	gradingPeriodHandler *handlers.GradingPeriodHandler,
	assignmentOverrideHandler *handlers.AssignmentOverrideHandler,
	latePolicyHandler *handlers.LatePolicyHandler,
	calendarEventHandler *handlers.CalendarEventHandler,
	conversationHandler *handlers.ConversationHandler,
	notificationHandler *handlers.NotificationHandler,
	contentMigrationHandler *handlers.ContentMigrationHandler,
	learningOutcomeHandler *handlers.LearningOutcomeHandler,
	speedGraderHandler *handlers.SpeedGraderHandler,
	groupHandler *handlers.GroupHandler,
	blueprintHandler *handlers.BlueprintHandler,
	coursePaceHandler *handlers.CoursePaceHandler,
	collaborationHandler *handlers.CollaborationHandler,
	conferenceHandler *handlers.ConferenceHandler,
	analyticsHandler *handlers.AnalyticsHandler,
	observerHandler *handlers.ObserverHandler,
	graphqlHandler *handlers.GraphQLHandler,
	authProviderHandler *handlers.AuthProviderHandler,
	discussionV2Handler *handlers.DiscussionV2Handler,
	contentImportHandler *handlers.ContentImportHandler,
	batchHandler *handlers.BatchHandler,
	ssoHandler *auth.SSOHandler,
	announcementHandler *handlers.AnnouncementHandler,
	enrollmentTermHandler *handlers.EnrollmentTermHandler,
	syllabusHandler *handlers.SyllabusHandler,
	notificationDeliveryHandler *handlers.NotificationDeliveryHandler,
	auditHandler *handlers.AuditHandler,
	customRoleHandler *handlers.CustomRoleHandler,
	onerosterHandler *handlers.OneRosterHandler,
	documentAnnotationHandler *handlers.DocumentAnnotationHandler,
	coppaHandler *handlers.COPPAHandler,
	ferpaHandler *handlers.FERPAHandler,
	accommodationHandler *handlers.AccommodationHandler,
	attendanceHandler *handlers.AttendanceHandler,
	portfolioHandler *handlers.PortfolioHandler,
	// Course Home Engine
	courseHomeHandler *handlers.CourseHomeHandler,
	// Peer Reviews, Question Banks
	peerReviewHandler *handlers.PeerReviewHandler,
	questionBankHandler *handlers.QuestionBankHandler,
	// Quiz Question Groups
	quizQuestionGroupHandler *handlers.QuizQuestionGroupHandler,
	// Quiz Statistics
	quizStatisticsHandler *handlers.QuizStatisticsHandler,
	// Setup
	setupHandler *handlers.SetupHandler,
	// P3 Features
	featureFlagHandler *handlers.FeatureFlagHandler,
	customGradebookColumnHandler *handlers.CustomGradebookColumnHandler,
	masteryPathHandler *handlers.MasteryPathHandler,
	appointmentGroupHandler *handlers.AppointmentGroupHandler,
	outcomeProficiencyHandler *handlers.OutcomeProficiencyHandler,
	// Pairing codes
	pairingCodeHandler *handlers.PairingCodeHandler,
	// Discussion Checkpoints, Smart Search, Commons, AI Assist
	discussionCheckpointHandler *handlers.DiscussionCheckpointHandler,
	smartSearchHandler *handlers.SmartSearchHandler,
	commonsHandler *handlers.CommonsHandler,
	aiAssistHandler *handlers.AIAssistHandler,
	// Wave A2: Quiz Item Banks, Stimuli, per-question Outcome Alignments
	quizItemBankHandler *handlers.QuizItemBankHandler,
	quizStimulusHandler *handlers.QuizStimulusHandler,
	quizOutcomeAlignmentHandler *handlers.QuizOutcomeAlignmentHandler,
	// Wave B: QTI / IMSCC import + export.
	qtiImportHandler *handlers.QTIImportHandler,
	// Phase 6 Wave 1: gamification read API.
	gamificationHandler *handlers.GamificationHandler,
	authMiddleware *middleware.AuthMiddleware,
	permMiddleware *middleware.PermissionMiddleware,
	accountRepo repository.AccountRepository,
) *Router {
	return &Router{
		accountRepo: accountRepo,
		userHandler:            userHandler,
		accountHandler:         accountHandler,
		courseHandler:           courseHandler,
		sectionHandler:         sectionHandler,
		enrollmentHandler:      enrollmentHandler,
		moduleHandler:          moduleHandler,
		moduleItemHandler:      moduleItemHandler,
		pageHandler:            pageHandler,
		assignmentHandler:      assignmentHandler,
		assignmentGroupHandler: assignmentGroupHandler,
		submissionHandler:      submissionHandler,
		gradebookHandler:       gradebookHandler,
		gradingStandardHandler: gradingStandardHandler,
		developerKeyHandler:    developerKeyHandler,
		accessTokenHandler:     accessTokenHandler,
		oauth2Handler:          oauth2Handler,
		externalToolHandler:    externalToolHandler,
		ltiHandler:             ltiHandler,
		discussionHandler:      discussionHandler,
		discussionEntryHandler: discussionEntryHandler,
		fileHandler:            fileHandler,
		folderHandler:          folderHandler,
		sisImportHandler:          sisImportHandler,
		quizHandler:               quizHandler,
		quizQuestionHandler:       quizQuestionHandler,
		quizSubmissionHandler:     quizSubmissionHandler,
		rubricHandler:             rubricHandler,
		rubricAssessmentHandler:   rubricAssessmentHandler,
		gradingPeriodHandler:      gradingPeriodHandler,
		assignmentOverrideHandler: assignmentOverrideHandler,
		latePolicyHandler:         latePolicyHandler,
		calendarEventHandler:       calendarEventHandler,
		conversationHandler:        conversationHandler,
		notificationHandler:        notificationHandler,
		contentMigrationHandler:    contentMigrationHandler,
		learningOutcomeHandler:     learningOutcomeHandler,
		speedGraderHandler:         speedGraderHandler,
		groupHandler:               groupHandler,
		blueprintHandler:           blueprintHandler,
		coursePaceHandler:          coursePaceHandler,
		collaborationHandler:       collaborationHandler,
		conferenceHandler:          conferenceHandler,
		analyticsHandler:           analyticsHandler,
		observerHandler:            observerHandler,
		graphqlHandler:             graphqlHandler,
		authProviderHandler:        authProviderHandler,
		discussionV2Handler:        discussionV2Handler,
		contentImportHandler:       contentImportHandler,
		batchHandler:               batchHandler,
		ssoHandler:                 ssoHandler,
		announcementHandler:         announcementHandler,
		enrollmentTermHandler:       enrollmentTermHandler,
		syllabusHandler:             syllabusHandler,
		notificationDeliveryHandler:  notificationDeliveryHandler,
		auditHandler:                auditHandler,
		customRoleHandler:           customRoleHandler,
		onerosterHandler:            onerosterHandler,
		documentAnnotationHandler:   documentAnnotationHandler,
		coppaHandler:                coppaHandler,
		ferpaHandler:                ferpaHandler,
		accommodationHandler:        accommodationHandler,
		attendanceHandler:           attendanceHandler,
		portfolioHandler:            portfolioHandler,
		courseHomeHandler:           courseHomeHandler,
		peerReviewHandler:          peerReviewHandler,
		questionBankHandler:        questionBankHandler,
		quizQuestionGroupHandler:   quizQuestionGroupHandler,
		quizStatisticsHandler:      quizStatisticsHandler,
		setupHandler:               setupHandler,
		featureFlagHandler:           featureFlagHandler,
		customGradebookColumnHandler: customGradebookColumnHandler,
		masteryPathHandler:           masteryPathHandler,
		appointmentGroupHandler:      appointmentGroupHandler,
		outcomeProficiencyHandler:    outcomeProficiencyHandler,
		pairingCodeHandler:           pairingCodeHandler,
		discussionCheckpointHandler: discussionCheckpointHandler,
		smartSearchHandler:          smartSearchHandler,
		commonsHandler:              commonsHandler,
		aiAssistHandler:             aiAssistHandler,
		quizItemBankHandler:         quizItemBankHandler,
		quizStimulusHandler:         quizStimulusHandler,
		quizOutcomeAlignmentHandler: quizOutcomeAlignmentHandler,
		qtiImportHandler:            qtiImportHandler,
		gamificationHandler:         gamificationHandler,
		authMiddleware:              authMiddleware,
		permMiddleware:              permMiddleware,
	}
}

func (r *Router) Register(app *fiber.App) {
	api := app.Group("/api/v1", middleware.PaginationParams())

	// Permission middleware aliases for readability
	admin := r.permMiddleware.RequireAdmin()
	enrolled := r.permMiddleware.RequireEnrolled()
	instructor := r.permMiddleware.RequireInstructor()
	selfOrAdmin := r.permMiddleware.RequireSelfOrAdmin()

	// Setup wizard (public, no auth required)
	api.Get("/setup/status", r.setupHandler.GetStatus)
	api.Post("/setup/complete", middleware.AuthRateLimit(), r.setupHandler.CompleteSetup)

	// Public auth routes (rate-limited to prevent brute-force)
	authLimit := middleware.AuthRateLimit()
	api.Post("/login", authLimit, r.userHandler.Login)
	api.Post("/register", authLimit, r.userHandler.Register)
	api.Post("/logout", r.userHandler.Logout)
	api.Post("/password/reset", authLimit, r.userHandler.RequestPasswordReset)
	api.Post("/password/reset/confirm", authLimit, r.userHandler.ResetPassword)

	// Public OAuth2 token endpoint (no auth required)
	api.Post("/login/oauth2/token", r.oauth2Handler.Token)

	// Public LTI endpoints (no auth required)
	api.Get("/lti/jwks", r.ltiHandler.JWKS)
	api.Post("/lti/oidc/login", r.ltiHandler.OIDCLogin)
	api.Post("/lti/launch", r.ltiHandler.LaunchDirect)

	// Public SSO endpoints (no auth required)
	api.Get("/auth/saml/login", r.ssoHandler.HandleSAMLLogin)
	api.Post("/auth/saml/acs", r.ssoHandler.HandleSAMLACS)
	api.Get("/auth/saml/metadata", r.ssoHandler.HandleSAMLMetadata)
	api.Get("/auth/cas/login", r.ssoHandler.HandleCASLogin)
	api.Get("/auth/cas/callback", r.ssoHandler.HandleCASCallback)
	api.Post("/auth/ldap/login", r.ssoHandler.HandleLDAPLogin)

	// Public page endpoint (no auth required)
	api.Get("/courses/:course_id/p/:slug", r.pageHandler.GetPublicPage)

	// Protected routes (authentication required)
	protected := api.Group("", r.authMiddleware.Protected(), middleware.CSRFProtection())

	// Users (self access or admin)
	protected.Get("/users/self", r.userHandler.GetSelf)
	protected.Get("/users", admin, r.userHandler.ListUsers)
	protected.Get("/users/:id", selfOrAdmin, r.userHandler.GetUser)
	protected.Get("/users/:id/profile", selfOrAdmin, r.userHandler.GetUserProfile)
	protected.Put("/users/:id", selfOrAdmin, r.userHandler.UpdateUser)
	protected.Put("/users/:id/role", admin, r.userHandler.UpdateUserRole)

	// Masquerade (admin only)
	protected.Post("/users/:id/masquerade", admin, r.userHandler.StartMasquerade)
	protected.Delete("/masquerade", r.userHandler.EndMasquerade)

	// Personal Access Tokens (self or admin)
	protected.Get("/users/:user_id/tokens", selfOrAdmin, r.accessTokenHandler.ListAccessTokens)
	protected.Post("/users/:user_id/tokens", selfOrAdmin, r.accessTokenHandler.CreateAccessToken)
	protected.Delete("/users/:user_id/tokens/:id", selfOrAdmin, r.accessTokenHandler.DeleteAccessToken)

	// Accounts (admin only)
	protected.Get("/accounts", admin, r.accountHandler.ListAccounts)
	protected.Get("/accounts/:id", admin, r.accountHandler.GetAccount)
	protected.Put("/accounts/:id", admin, r.accountHandler.UpdateAccount)

	// Developer Keys (admin only)
	protected.Get("/accounts/:account_id/developer_keys", admin, r.developerKeyHandler.ListDeveloperKeys)
	protected.Post("/accounts/:account_id/developer_keys", admin, r.developerKeyHandler.CreateDeveloperKey)
	protected.Get("/accounts/:account_id/developer_keys/:id", admin, r.developerKeyHandler.GetDeveloperKey)
	protected.Put("/accounts/:account_id/developer_keys/:id", admin, r.developerKeyHandler.UpdateDeveloperKey)
	protected.Delete("/accounts/:account_id/developer_keys/:id", admin, r.developerKeyHandler.DeleteDeveloperKey)

	// OAuth2 Authorization (requires auth for consent)
	protected.Get("/login/oauth2/auth", r.oauth2Handler.Authorize)
	protected.Post("/login/oauth2/auth", r.oauth2Handler.AuthorizePost)

	// Courses (list: any user sees their own; create: admin; manage: instructor)
	protected.Get("/courses", r.courseHandler.ListCourses)
	protected.Post("/courses", admin, r.courseHandler.CreateCourse)
	protected.Get("/courses/:id", enrolled, r.courseHandler.GetCourse)
	protected.Put("/courses/:id", instructor, r.courseHandler.UpdateCourse)
	protected.Delete("/courses/:id", instructor, r.courseHandler.DeleteCourse)

	// External Tools (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/external_tools", enrolled, r.externalToolHandler.ListExternalTools)
	protected.Post("/courses/:course_id/external_tools", instructor, r.externalToolHandler.CreateExternalTool)
	protected.Get("/courses/:course_id/external_tools/:id", enrolled, r.externalToolHandler.GetExternalTool)
	protected.Put("/courses/:course_id/external_tools/:id", instructor, r.externalToolHandler.UpdateExternalTool)
	protected.Delete("/courses/:course_id/external_tools/:id", instructor, r.externalToolHandler.DeleteExternalTool)

	// Sections (view: enrolled; create: instructor)
	protected.Get("/courses/:course_id/sections", enrolled, r.sectionHandler.ListSections)
	protected.Post("/courses/:course_id/sections", instructor, r.sectionHandler.CreateSection)
	protected.Get("/sections/:id", r.sectionHandler.GetSection)

	// Enrollments (view: enrolled; create: instructor)
	protected.Get("/courses/:course_id/enrollments", enrolled, r.enrollmentHandler.ListEnrollments)
	protected.Post("/courses/:course_id/enrollments", instructor, r.enrollmentHandler.CreateEnrollment)

	// Modules (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/modules", enrolled, r.moduleHandler.ListModules)
	protected.Post("/courses/:course_id/modules", instructor, r.moduleHandler.CreateModule)
	protected.Get("/courses/:course_id/modules/:id", enrolled, r.moduleHandler.GetModule)
	protected.Put("/courses/:course_id/modules/:id", instructor, r.moduleHandler.UpdateModule)
	protected.Delete("/courses/:course_id/modules/:id", instructor, r.moduleHandler.DeleteModule)
	protected.Post("/courses/:course_id/modules/reorder", instructor, r.moduleHandler.ReorderModules)

	// Course Home Engine (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/home", enrolled, r.courseHomeHandler.GetHomeData)
	protected.Post("/courses/:course_id/home/visit", enrolled, r.courseHomeHandler.RecordVisit)
	protected.Get("/courses/:course_id/home/buttons", enrolled, r.courseHomeHandler.ListButtons)
	protected.Post("/courses/:course_id/home/buttons", instructor, r.courseHomeHandler.CreateButton)
	protected.Put("/courses/:course_id/home/buttons/reorder", instructor, r.courseHomeHandler.ReorderButtons)
	protected.Put("/courses/:course_id/home/buttons/:id", instructor, r.courseHomeHandler.UpdateButton)
	protected.Delete("/courses/:course_id/home/buttons/:id", instructor, r.courseHomeHandler.DeleteButton)
	protected.Get("/courses/:course_id/home/overrides", instructor, r.courseHomeHandler.ListOverrides)
	protected.Post("/courses/:course_id/home/overrides", instructor, r.courseHomeHandler.CreateOverride)
	protected.Put("/courses/:course_id/home/overrides/:id", instructor, r.courseHomeHandler.UpdateOverride)
	protected.Delete("/courses/:course_id/home/overrides/:id", instructor, r.courseHomeHandler.DeleteOverride)

	// Module Items (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/modules/:module_id/items", enrolled, r.moduleItemHandler.ListModuleItems)
	protected.Post("/courses/:course_id/modules/:module_id/items", instructor, r.moduleItemHandler.CreateModuleItem)
	protected.Get("/courses/:course_id/modules/:module_id/items/:item_id", enrolled, r.moduleItemHandler.GetModuleItem)
	protected.Put("/courses/:course_id/modules/:module_id/items/:item_id", instructor, r.moduleItemHandler.UpdateModuleItem)
	protected.Delete("/courses/:course_id/modules/:module_id/items/:item_id", instructor, r.moduleItemHandler.DeleteModuleItem)
	protected.Post("/courses/:course_id/modules/:module_id/items/reorder", instructor, r.moduleItemHandler.ReorderItems)
	protected.Post("/courses/:course_id/modules/:module_id/items/:item_id/move", instructor, r.moduleItemHandler.MoveItem)

	// Pages (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/pages", enrolled, r.pageHandler.ListPages)
	protected.Post("/courses/:course_id/pages", instructor, r.pageHandler.CreatePage)
	protected.Get("/courses/:course_id/pages/:url_or_id", enrolled, r.pageHandler.GetPage)
	protected.Put("/courses/:course_id/pages/:url_or_id", instructor, r.pageHandler.UpdatePage)
	protected.Delete("/courses/:course_id/pages/:url_or_id", instructor, r.pageHandler.DeletePage)

	// Assignments (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/assignments", enrolled, r.assignmentHandler.ListAssignments)
	protected.Post("/courses/:course_id/assignments", instructor, r.assignmentHandler.CreateAssignment)
	protected.Get("/courses/:course_id/assignments/:id", enrolled, r.assignmentHandler.GetAssignment)
	protected.Put("/courses/:course_id/assignments/:id", instructor, r.assignmentHandler.UpdateAssignment)
	protected.Delete("/courses/:course_id/assignments/:id", instructor, r.assignmentHandler.DeleteAssignment)

	// Assignment Groups (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/assignment_groups", enrolled, r.assignmentGroupHandler.ListAssignmentGroups)
	protected.Post("/courses/:course_id/assignment_groups", instructor, r.assignmentGroupHandler.CreateAssignmentGroup)
	protected.Get("/courses/:course_id/assignment_groups/:id", enrolled, r.assignmentGroupHandler.GetAssignmentGroup)
	protected.Put("/courses/:course_id/assignment_groups/:id", instructor, r.assignmentGroupHandler.UpdateAssignmentGroup)
	protected.Delete("/courses/:course_id/assignment_groups/:id", instructor, r.assignmentGroupHandler.DeleteAssignmentGroup)

	// Course-wide submissions (enrolled users; students see only their own)
	protected.Get("/courses/:course_id/submissions", enrolled, r.submissionHandler.ListCourseSubmissions)
	protected.Post("/courses/:course_id/submissions/bulk_grade", instructor, r.submissionHandler.BulkGrade)

	// Submissions (view: enrolled; create: enrolled; grade: instructor)
	protected.Get("/courses/:course_id/assignments/:assignment_id/submissions", enrolled, r.submissionHandler.ListSubmissions)
	protected.Post("/courses/:course_id/assignments/:assignment_id/submissions", enrolled, r.submissionHandler.CreateSubmission)
	protected.Get("/courses/:course_id/assignments/:assignment_id/submissions/:user_id", enrolled, r.submissionHandler.GetSubmission)
	protected.Put("/courses/:course_id/assignments/:assignment_id/submissions/:user_id", instructor, r.submissionHandler.UpdateSubmission)

	// Submission Comments (view/create: enrolled)
	protected.Get("/courses/:course_id/assignments/:assignment_id/submissions/:user_id/comments", enrolled, r.submissionHandler.ListSubmissionComments)
	protected.Post("/courses/:course_id/assignments/:assignment_id/submissions/:user_id/comments", enrolled, r.submissionHandler.CreateSubmissionComment)

	// Gradebook (instructor only)
	protected.Get("/courses/:course_id/gradebook", instructor, r.gradebookHandler.GetGradebook)
	protected.Get("/courses/:course_id/students/:student_id/grade", instructor, r.gradebookHandler.GetStudentGrade)

	// Grading Standards (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/grading_standards", enrolled, r.gradingStandardHandler.ListGradingStandards)
	protected.Post("/courses/:course_id/grading_standards", instructor, r.gradingStandardHandler.CreateGradingStandard)
	protected.Put("/courses/:course_id/grading_standards/:id", instructor, r.gradingStandardHandler.UpdateGradingStandard)
	protected.Delete("/courses/:course_id/grading_standards/:id", instructor, r.gradingStandardHandler.DeleteGradingStandard)

	// LTI AGS (Assignment and Grade Services) - protected via OAuth2 token + enrollment
	protected.Get("/lti/courses/:course_id/line_items", enrolled, r.ltiHandler.ListLineItems)
	protected.Post("/lti/courses/:course_id/line_items", instructor, r.ltiHandler.CreateLineItem)
	protected.Get("/lti/courses/:course_id/line_items/:id", enrolled, r.ltiHandler.GetLineItem)
	protected.Put("/lti/courses/:course_id/line_items/:id", instructor, r.ltiHandler.UpdateLineItem)
	protected.Delete("/lti/courses/:course_id/line_items/:id", instructor, r.ltiHandler.DeleteLineItem)
	protected.Post("/lti/courses/:course_id/line_items/:id/scores", instructor, r.ltiHandler.PostScore)
	protected.Get("/lti/courses/:course_id/line_items/:id/results", enrolled, r.ltiHandler.GetResults)

	// LTI NRPS (Names and Role Provisioning Services)
	protected.Get("/lti/courses/:course_id/memberships", enrolled, r.ltiHandler.GetMemberships)

	// Discussion Topics (view: enrolled; manage: instructor; post: enrolled)
	protected.Get("/courses/:course_id/discussion_topics", enrolled, r.discussionHandler.ListTopics)
	protected.Post("/courses/:course_id/discussion_topics", instructor, r.discussionHandler.CreateTopic)
	protected.Get("/courses/:course_id/discussion_topics/:topic_id", enrolled, r.discussionHandler.GetTopic)
	protected.Put("/courses/:course_id/discussion_topics/:topic_id", instructor, r.discussionHandler.UpdateTopic)
	protected.Delete("/courses/:course_id/discussion_topics/:topic_id", instructor, r.discussionHandler.DeleteTopic)
	protected.Get("/courses/:course_id/discussion_topics/:topic_id/view", enrolled, r.discussionHandler.GetFullView)

	// Discussion Entries (view/post: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/discussion_topics/:topic_id/entries", enrolled, r.discussionEntryHandler.ListEntries)
	protected.Post("/courses/:course_id/discussion_topics/:topic_id/entries", enrolled, r.discussionEntryHandler.CreateEntry)
	protected.Put("/courses/:course_id/discussion_topics/:topic_id/entries/:id", enrolled, r.discussionEntryHandler.UpdateEntry)
	protected.Delete("/courses/:course_id/discussion_topics/:topic_id/entries/:id", instructor, r.discussionEntryHandler.DeleteEntry)
	protected.Get("/courses/:course_id/discussion_topics/:topic_id/entries/:entry_id/replies", enrolled, r.discussionEntryHandler.ListReplies)
	protected.Post("/courses/:course_id/discussion_topics/:topic_id/entries/:entry_id/replies", enrolled, r.discussionEntryHandler.CreateReply)
	protected.Post("/courses/:course_id/discussion_topics/:topic_id/entries/:entry_id/rating", enrolled, r.discussionEntryHandler.RateEntry)

	// Files (view: enrolled; upload/delete: instructor)
	protected.Get("/courses/:course_id/files", enrolled, r.fileHandler.ListCourseFiles)
	protected.Post("/courses/:course_id/files", middleware.UploadRateLimit(), middleware.EnforceUploadSize(r.accountRepo), instructor, r.fileHandler.UploadCourseFile)
	protected.Get("/courses/:course_id/files/:id", enrolled, r.fileHandler.GetFile)
	protected.Delete("/courses/:course_id/files/:id", instructor, r.fileHandler.DeleteFile)
	protected.Get("/files/:id/download", r.fileHandler.DownloadFile)
	protected.Get("/folders/:folder_id/files", r.fileHandler.ListFolderFiles)

	// Folders (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/folders", enrolled, r.folderHandler.ListCourseFolders)
	protected.Post("/courses/:course_id/folders", instructor, r.folderHandler.CreateCourseFolder)
	protected.Get("/folders/:id", r.folderHandler.GetFolder)
	protected.Put("/folders/:id", r.folderHandler.UpdateFolder)
	protected.Delete("/folders/:id", r.folderHandler.DeleteFolder)
	protected.Get("/folders/:folder_id/folders", r.folderHandler.ListSubfolders)

	// SIS Import/Export (admin only)
	protected.Post("/accounts/:account_id/sis_imports", middleware.UploadRateLimit(), admin, r.sisImportHandler.CreateSISImport)
	protected.Get("/accounts/:account_id/sis_imports", admin, r.sisImportHandler.ListSISImports)
	protected.Get("/accounts/:account_id/sis_imports/:id", admin, r.sisImportHandler.GetSISImport)
	protected.Get("/accounts/:account_id/sis_imports/:id/errors", admin, r.sisImportHandler.GetSISImportErrors)
	protected.Get("/accounts/:account_id/sis_exports/users.csv", admin, r.sisImportHandler.ExportUsersCSV)
	protected.Get("/accounts/:account_id/sis_exports/courses.csv", admin, r.sisImportHandler.ExportCoursesCSV)
	protected.Get("/accounts/:account_id/sis_exports/sections.csv", admin, r.sisImportHandler.ExportSectionsCSV)
	protected.Get("/accounts/:account_id/sis_exports/enrollments.csv", admin, r.sisImportHandler.ExportEnrollmentsCSV)

	// Quizzes (view: enrolled; manage: instructor; take: enrolled)
	protected.Get("/courses/:course_id/quizzes", enrolled, r.quizHandler.ListQuizzes)
	protected.Post("/courses/:course_id/quizzes", instructor, r.quizHandler.CreateQuiz)
	protected.Get("/courses/:course_id/quizzes/:id", enrolled, r.quizHandler.GetQuiz)
	protected.Put("/courses/:course_id/quizzes/:id", instructor, r.quizHandler.UpdateQuiz)
	protected.Delete("/courses/:course_id/quizzes/:id", instructor, r.quizHandler.DeleteQuiz)

	// Quiz Questions (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/quizzes/:quiz_id/questions", enrolled, r.quizQuestionHandler.ListQuestions)
	protected.Post("/courses/:course_id/quizzes/:quiz_id/questions", instructor, r.quizQuestionHandler.CreateQuestion)
	protected.Get("/courses/:course_id/quizzes/:quiz_id/questions/:question_id", enrolled, r.quizQuestionHandler.GetQuestion)
	protected.Put("/courses/:course_id/quizzes/:quiz_id/questions/:question_id", instructor, r.quizQuestionHandler.UpdateQuestion)
	protected.Delete("/courses/:course_id/quizzes/:quiz_id/questions/:question_id", instructor, r.quizQuestionHandler.DeleteQuestion)

	// Quiz Submissions (take: enrolled; view: enrolled)
	protected.Post("/courses/:course_id/quizzes/:quiz_id/submissions", enrolled, r.quizSubmissionHandler.StartSubmission)
	protected.Get("/courses/:course_id/quizzes/:quiz_id/submissions", enrolled, r.quizSubmissionHandler.ListSubmissions)
	protected.Get("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id", enrolled, r.quizSubmissionHandler.GetSubmission)
	protected.Put("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/questions/:question_id", enrolled, r.quizSubmissionHandler.AnswerQuestion)
	protected.Post("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/complete", enrolled, r.quizSubmissionHandler.CompleteSubmission)
	protected.Get("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/answers", enrolled, r.quizSubmissionHandler.GetSubmissionAnswers)
	protected.Get("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/questions", enrolled, r.quizSubmissionHandler.GetSubmissionQuestions)

	// Quiz Statistics (instructor only)
	protected.Get("/courses/:course_id/quizzes/:quiz_id/statistics", instructor, r.quizStatisticsHandler.GetQuizStatistics)

	// Quiz Question Groups (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/quizzes/:quiz_id/groups", enrolled, r.quizQuestionGroupHandler.ListGroups)
	protected.Post("/courses/:course_id/quizzes/:quiz_id/groups", instructor, r.quizQuestionGroupHandler.CreateGroup)
	protected.Get("/courses/:course_id/quizzes/:quiz_id/groups/:group_id", enrolled, r.quizQuestionGroupHandler.GetGroup)
	protected.Put("/courses/:course_id/quizzes/:quiz_id/groups/:group_id", instructor, r.quizQuestionGroupHandler.UpdateGroup)
	protected.Delete("/courses/:course_id/quizzes/:quiz_id/groups/:group_id", instructor, r.quizQuestionGroupHandler.DeleteGroup)

	// Rubrics (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/rubrics", enrolled, r.rubricHandler.ListCourseRubrics)
	protected.Post("/courses/:course_id/rubrics", instructor, r.rubricHandler.CreateCourseRubric)
	protected.Get("/courses/:course_id/rubrics/:rubric_id", enrolled, r.rubricHandler.GetRubric)
	protected.Put("/courses/:course_id/rubrics/:rubric_id", instructor, r.rubricHandler.UpdateRubric)
	protected.Delete("/courses/:course_id/rubrics/:rubric_id", instructor, r.rubricHandler.DeleteRubric)
	protected.Post("/courses/:course_id/rubrics/:rubric_id/associations", instructor, r.rubricHandler.AssociateRubric)

	// Rubric Assessments (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/rubric_associations/:association_id/rubric_assessments", enrolled, r.rubricAssessmentHandler.ListAssessments)
	protected.Post("/courses/:course_id/rubric_associations/:association_id/rubric_assessments", instructor, r.rubricAssessmentHandler.CreateAssessment)
	protected.Get("/courses/:course_id/rubric_associations/:association_id/rubric_assessments/:assessment_id", enrolled, r.rubricAssessmentHandler.GetAssessment)
	protected.Put("/courses/:course_id/rubric_associations/:association_id/rubric_assessments/:assessment_id", instructor, r.rubricAssessmentHandler.UpdateAssessment)

	// Grading Periods (admin only)
	protected.Get("/accounts/:account_id/grading_period_groups", admin, r.gradingPeriodHandler.ListGroups)
	protected.Post("/accounts/:account_id/grading_period_groups", admin, r.gradingPeriodHandler.CreateGroup)
	protected.Get("/accounts/:account_id/grading_period_groups/:group_id", admin, r.gradingPeriodHandler.GetGroup)
	protected.Put("/accounts/:account_id/grading_period_groups/:group_id", admin, r.gradingPeriodHandler.UpdateGroup)
	protected.Delete("/accounts/:account_id/grading_period_groups/:group_id", admin, r.gradingPeriodHandler.DeleteGroup)
	protected.Get("/accounts/:account_id/grading_period_groups/:group_id/grading_periods", admin, r.gradingPeriodHandler.ListPeriods)
	protected.Post("/accounts/:account_id/grading_period_groups/:group_id/grading_periods", admin, r.gradingPeriodHandler.CreatePeriod)
	protected.Get("/accounts/:account_id/grading_period_groups/:group_id/grading_periods/:period_id", admin, r.gradingPeriodHandler.GetPeriod)
	protected.Put("/accounts/:account_id/grading_period_groups/:group_id/grading_periods/:period_id", admin, r.gradingPeriodHandler.UpdatePeriod)
	protected.Delete("/accounts/:account_id/grading_period_groups/:group_id/grading_periods/:period_id", admin, r.gradingPeriodHandler.DeletePeriod)

	// Assignment Rubric (view: enrolled)
	protected.Get("/courses/:course_id/assignments/:assignment_id/rubric", enrolled, r.rubricHandler.GetAssignmentRubric)

	// Assignment Overrides (instructor only)
	protected.Get("/courses/:course_id/assignments/:assignment_id/overrides", instructor, r.assignmentOverrideHandler.ListOverrides)
	protected.Post("/courses/:course_id/assignments/:assignment_id/overrides", instructor, r.assignmentOverrideHandler.CreateOverride)
	protected.Get("/courses/:course_id/assignments/:assignment_id/overrides/:override_id", instructor, r.assignmentOverrideHandler.GetOverride)
	protected.Put("/courses/:course_id/assignments/:assignment_id/overrides/:override_id", instructor, r.assignmentOverrideHandler.UpdateOverride)
	protected.Delete("/courses/:course_id/assignments/:assignment_id/overrides/:override_id", instructor, r.assignmentOverrideHandler.DeleteOverride)

	// Late Policy (instructor only)
	protected.Get("/courses/:course_id/late_policy", instructor, r.latePolicyHandler.GetLatePolicy)
	protected.Post("/courses/:course_id/late_policy", instructor, r.latePolicyHandler.CreateLatePolicy)
	protected.Put("/courses/:course_id/late_policy", instructor, r.latePolicyHandler.UpdateLatePolicy)
	protected.Delete("/courses/:course_id/late_policy", instructor, r.latePolicyHandler.DeleteLatePolicy)

	// Calendar Events (any authenticated user)
	protected.Get("/calendar_events", r.calendarEventHandler.ListEvents)
	protected.Get("/calendar_events.ics", r.calendarEventHandler.ExportAsICal)
	protected.Post("/calendar_events", r.calendarEventHandler.CreateEvent)
	protected.Get("/calendar_events/:id", r.calendarEventHandler.GetEvent)
	protected.Put("/calendar_events/:id", r.calendarEventHandler.UpdateEvent)
	protected.Delete("/calendar_events/:id", r.calendarEventHandler.DeleteEvent)
	protected.Get("/courses/:course_id/calendar_events", enrolled, r.calendarEventHandler.ListEvents)

	// Conversations (any authenticated user)
	protected.Get("/conversations", r.conversationHandler.ListConversations)
	protected.Post("/conversations", r.conversationHandler.CreateConversation)
	protected.Get("/conversations/:id", r.conversationHandler.GetConversation)
	protected.Put("/conversations/:id", r.conversationHandler.UpdateConversation)
	protected.Get("/conversations/:id/messages", r.conversationHandler.ListMessages)
	protected.Post("/conversations/:id/messages", r.conversationHandler.CreateMessage)
	protected.Put("/conversations/:id/mark_as_read", r.conversationHandler.MarkAsRead)

	// Notifications (any authenticated user)
	protected.Get("/notifications", r.notificationHandler.ListNotifications)
	protected.Put("/notifications/mark_all_as_read", r.notificationHandler.MarkAllAsRead)
	protected.Put("/notifications/:id/mark_as_read", r.notificationHandler.MarkAsRead)
	protected.Get("/users/self/notification_preferences", r.notificationHandler.GetPreferences)
	protected.Put("/users/self/notification_preferences", r.notificationHandler.UpdatePreferences)

	// Content Migrations (instructor only)
	protected.Get("/courses/:course_id/content_migrations", instructor, r.contentMigrationHandler.ListMigrations)
	protected.Post("/courses/:course_id/content_migrations", middleware.ExpensiveOpRateLimit(), instructor, r.contentMigrationHandler.CreateMigration)
	protected.Get("/courses/:course_id/content_migrations/:id", instructor, r.contentMigrationHandler.GetMigration)
	protected.Put("/courses/:course_id/content_migrations/:id", instructor, r.contentMigrationHandler.UpdateMigration)

	// Learning Outcomes (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/outcome_groups", enrolled, r.learningOutcomeHandler.ListGroups)
	protected.Post("/courses/:course_id/outcome_groups", instructor, r.learningOutcomeHandler.CreateGroup)
	protected.Get("/courses/:course_id/outcome_groups/:group_id", enrolled, r.learningOutcomeHandler.GetGroup)
	protected.Put("/courses/:course_id/outcome_groups/:group_id", instructor, r.learningOutcomeHandler.UpdateGroup)
	protected.Delete("/courses/:course_id/outcome_groups/:group_id", instructor, r.learningOutcomeHandler.DeleteGroup)
	protected.Get("/courses/:course_id/outcome_groups/:group_id/outcomes", enrolled, r.learningOutcomeHandler.ListOutcomes)
	protected.Post("/courses/:course_id/outcome_groups/:group_id/outcomes", instructor, r.learningOutcomeHandler.CreateOutcome)
	protected.Get("/courses/:course_id/outcomes/:outcome_id", enrolled, r.learningOutcomeHandler.GetOutcome)
	protected.Put("/courses/:course_id/outcomes/:outcome_id", instructor, r.learningOutcomeHandler.UpdateOutcome)
	protected.Delete("/courses/:course_id/outcomes/:outcome_id", instructor, r.learningOutcomeHandler.DeleteOutcome)
	protected.Get("/courses/:course_id/outcome_results", enrolled, r.learningOutcomeHandler.ListResults)
	protected.Post("/courses/:course_id/outcome_results", instructor, r.learningOutcomeHandler.CreateResult)
	protected.Get("/courses/:course_id/outcome_rollups", enrolled, r.learningOutcomeHandler.GetMasteryGradebook)
	protected.Get("/courses/:course_id/outcome_alignments", enrolled, r.learningOutcomeHandler.ListAlignments)
	protected.Post("/courses/:course_id/outcome_alignments", instructor, r.learningOutcomeHandler.CreateAlignment)
	protected.Delete("/courses/:course_id/outcome_alignments/:alignment_id", instructor, r.learningOutcomeHandler.DeleteAlignment)

	// SpeedGrader (instructor only)
	protected.Get("/courses/:course_id/assignments/:assignment_id/speedgrader", instructor, r.speedGraderHandler.GetSpeedGraderData)
	protected.Get("/courses/:course_id/assignments/:assignment_id/speedgrader/submissions/:user_id", instructor, r.speedGraderHandler.GetStudentSubmission)

	// Grade posting (instructor only)
	protected.Post("/courses/:course_id/assignments/:id/post_grades", instructor, r.submissionHandler.PostGrades)
	protected.Post("/courses/:course_id/assignments/:id/hide_grades", instructor, r.submissionHandler.HideGrades)

	// Groups (view: enrolled; manage categories: instructor; join: enrolled)
	protected.Get("/courses/:course_id/group_categories", enrolled, r.groupHandler.ListGroupCategories)
	protected.Post("/courses/:course_id/group_categories", instructor, r.groupHandler.CreateGroupCategory)
	protected.Get("/group_categories/:id", r.groupHandler.GetGroupCategory)
	protected.Put("/group_categories/:id", r.groupHandler.UpdateGroupCategory)
	protected.Delete("/group_categories/:id", r.groupHandler.DeleteGroupCategory)
	protected.Get("/group_categories/:group_category_id/groups", r.groupHandler.ListGroupsByCategory)
	protected.Post("/group_categories/:group_category_id/groups", r.groupHandler.CreateGroup)
	protected.Get("/groups/:id", r.groupHandler.GetGroup)
	protected.Put("/groups/:id", r.groupHandler.UpdateGroup)
	protected.Delete("/groups/:id", r.groupHandler.DeleteGroup)
	protected.Get("/groups/:group_id/memberships", r.groupHandler.ListGroupMemberships)
	protected.Post("/groups/:group_id/memberships", r.groupHandler.CreateGroupMembership)
	protected.Put("/groups/:group_id/memberships/:membership_id", r.groupHandler.UpdateGroupMembership)
	protected.Delete("/groups/:group_id/memberships/:membership_id", r.groupHandler.DeleteGroupMembership)
	protected.Get("/users/self/groups", r.groupHandler.ListUserGroups)

	// Blueprint Courses (instructor only)
	protected.Get("/courses/:course_id/blueprint_templates", instructor, r.blueprintHandler.ListTemplates)
	protected.Post("/courses/:course_id/blueprint_templates", instructor, r.blueprintHandler.CreateTemplate)
	protected.Get("/courses/:course_id/blueprint_templates/default", instructor, r.blueprintHandler.GetDefaultTemplate)
	protected.Put("/courses/:course_id/blueprint_templates/default", instructor, r.blueprintHandler.UpdateDefaultTemplate)
	protected.Get("/courses/:course_id/blueprint_templates/default/associated_courses", instructor, r.blueprintHandler.GetAssociatedCourses)
	protected.Put("/courses/:course_id/blueprint_templates/default/associated_courses", instructor, r.blueprintHandler.UpdateAssociations)
	protected.Get("/courses/:course_id/blueprint_templates/default/migrations", instructor, r.blueprintHandler.ListMigrations)
	protected.Post("/courses/:course_id/blueprint_templates/default/migrations", instructor, r.blueprintHandler.CreateMigration)
	protected.Get("/courses/:course_id/blueprint_templates/default/migrations/:migration_id", instructor, r.blueprintHandler.GetMigration)
	protected.Get("/courses/:course_id/blueprint_templates/default/unsynced_changes", instructor, r.blueprintHandler.GetUnsyncedChanges)
	protected.Get("/courses/:course_id/blueprint_subscriptions", enrolled, r.blueprintHandler.ListSubscriptions)
	protected.Get("/courses/:course_id/blueprint_subscriptions/:subscription_id/migrations", enrolled, r.blueprintHandler.GetSubscriptionMigrations)
	protected.Get("/courses/:course_id/blueprint_subscriptions/:subscription_id/migrations/:migration_id", enrolled, r.blueprintHandler.GetSubscriptionMigration)

	// Course Pacing (instructor only)
	protected.Get("/courses/:course_id/course_pacing", instructor, r.coursePaceHandler.ListCoursePaces)
	protected.Post("/courses/:course_id/course_pacing", instructor, r.coursePaceHandler.CreateCoursePace)
	protected.Get("/courses/:course_id/course_pacing/:id", instructor, r.coursePaceHandler.GetCoursePace)
	protected.Put("/courses/:course_id/course_pacing/:id", instructor, r.coursePaceHandler.UpdateCoursePace)
	protected.Delete("/courses/:course_id/course_pacing/:id", instructor, r.coursePaceHandler.DeleteCoursePace)
	protected.Post("/courses/:course_id/course_pacing/:id/publish", instructor, r.coursePaceHandler.PublishCoursePace)
	protected.Get("/courses/:course_id/course_pacing/:id/module_items", instructor, r.coursePaceHandler.GetPaceModuleItems)
	protected.Put("/courses/:course_id/course_pacing/:id/module_items", instructor, r.coursePaceHandler.UpdatePaceModuleItems)

	// Collaborations (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/collaborations", enrolled, r.collaborationHandler.ListCollaborations)
	protected.Post("/courses/:course_id/collaborations", instructor, r.collaborationHandler.CreateCollaboration)
	protected.Get("/collaborations/:id", r.collaborationHandler.GetCollaboration)
	protected.Put("/collaborations/:id", r.collaborationHandler.UpdateCollaboration)
	protected.Delete("/collaborations/:id", r.collaborationHandler.DeleteCollaboration)

	// Conferences (view: enrolled; manage: instructor; join: enrolled)
	protected.Get("/courses/:course_id/conferences", enrolled, r.conferenceHandler.ListConferences)
	protected.Post("/courses/:course_id/conferences", instructor, r.conferenceHandler.CreateConference)
	protected.Get("/conferences/:id", r.conferenceHandler.GetConference)
	protected.Put("/conferences/:id", r.conferenceHandler.UpdateConference)
	protected.Delete("/conferences/:id", r.conferenceHandler.DeleteConference)
	protected.Post("/conferences/:id/join", r.conferenceHandler.JoinConference)
	protected.Post("/conferences/:id/end", r.conferenceHandler.EndConference)
	protected.Get("/conferences/:id/recordings", r.conferenceHandler.GetRecordings)
	protected.Get("/conferences/:id/participants", r.conferenceHandler.GetParticipants)

	// Analytics (course: instructor; department: admin)
	protected.Get("/courses/:course_id/analytics/activity", instructor, r.analyticsHandler.GetCourseActivity)
	protected.Get("/courses/:course_id/analytics/assignments", instructor, r.analyticsHandler.GetCourseAssignmentStats)
	protected.Get("/courses/:course_id/analytics/student_summaries", instructor, r.analyticsHandler.GetStudentSummaries)
	protected.Get("/courses/:course_id/analytics/users/:user_id/activity", instructor, r.analyticsHandler.GetStudentActivity)
	protected.Get("/courses/:course_id/analytics/users/:user_id/assignments", instructor, r.analyticsHandler.GetStudentAssignments)
	protected.Get("/accounts/:account_id/analytics/current/activity", admin, r.analyticsHandler.GetDepartmentActivity)
	protected.Get("/accounts/:account_id/analytics/current/grades", admin, r.analyticsHandler.GetDepartmentGrades)
	protected.Get("/accounts/:account_id/analytics/current/statistics", admin, r.analyticsHandler.GetDepartmentStatistics)
	protected.Post("/page_views", r.analyticsHandler.CreatePageView)
	protected.Get("/users/self/page_views", r.analyticsHandler.ListUserPageViews)

	// Observer/Parent Role (self or admin)
	protected.Post("/users/:user_id/observees", selfOrAdmin, r.observerHandler.LinkObservee)
	protected.Delete("/users/:user_id/observees/:observee_id", selfOrAdmin, r.observerHandler.UnlinkObservee)
	protected.Get("/users/:user_id/observees", selfOrAdmin, r.observerHandler.ListObservees)
	protected.Get("/users/:user_id/observees/:observee_id/courses", selfOrAdmin, r.observerHandler.GetObserveeCourses)
	protected.Get("/users/:user_id/observees/:child_id/overview", selfOrAdmin, r.observerHandler.GetChildOverview)

	// Parent/observer pairing codes (every authenticated user can manage their own).
	protected.Post("/users/self/pairing_codes", r.pairingCodeHandler.Generate)
	protected.Post("/users/self/pairing_codes/redeem", r.pairingCodeHandler.Redeem)
	protected.Get("/users/self/pairing_codes", r.pairingCodeHandler.List)
	protected.Delete("/users/self/pairing_codes/:id", r.pairingCodeHandler.Revoke)

	// GraphQL (any authenticated user)
	protected.Post("/graphql", r.graphqlHandler.HandleQuery)

	// Authentication Providers (admin only)
	protected.Get("/accounts/:account_id/authentication_providers", admin, r.authProviderHandler.ListProviders)
	protected.Post("/accounts/:account_id/authentication_providers", admin, r.authProviderHandler.CreateProvider)
	protected.Get("/accounts/:account_id/authentication_providers/:id", admin, r.authProviderHandler.GetProvider)
	protected.Put("/accounts/:account_id/authentication_providers/:id", admin, r.authProviderHandler.UpdateProvider)
	protected.Delete("/accounts/:account_id/authentication_providers/:id", admin, r.authProviderHandler.DeleteProvider)
	protected.Post("/accounts/:account_id/authentication_providers/:id/test", admin, r.authProviderHandler.TestConnection)

	// Discussion V2 (enhanced with read/unread, user profiles, edit history)
	protected.Get("/courses/:course_id/discussion_topics/:topic_id/view_v2", enrolled, r.discussionV2Handler.GetFullViewV2)
	protected.Post("/courses/:course_id/discussion_topics/:topic_id/entries/:entry_id/read", enrolled, r.discussionV2Handler.MarkEntryRead)
	protected.Post("/courses/:course_id/discussion_topics/:topic_id/mark_all_read", enrolled, r.discussionV2Handler.MarkTopicRead)
	protected.Get("/courses/:course_id/discussion_topics/:topic_id/unread_count", enrolled, r.discussionV2Handler.GetUnreadCount)
	protected.Put("/courses/:course_id/discussion_topics/:topic_id/subscription", enrolled, r.discussionV2Handler.ToggleSubscription)
	protected.Get("/courses/:course_id/discussion_topics/:topic_id/entries/:entry_id/versions", enrolled, r.discussionV2Handler.GetEntryVersions)
	protected.Put("/courses/:course_id/discussion_topics/:topic_id/entries/:entry_id/v2", enrolled, r.discussionV2Handler.UpdateEntryV2)

	// Content Import (IMSCC/Common Cartridge)
	protected.Post("/courses/:course_id/content_imports", middleware.ExpensiveOpRateLimit(), middleware.EnforceUploadSize(r.accountRepo), instructor, r.contentImportHandler.ImportPackage)

	// Batch Operations (instructor/admin, rate-limited)
	protected.Post("/courses/clone", middleware.ExpensiveOpRateLimit(), admin, r.batchHandler.CloneCourse)
	protected.Post("/courses/:course_id/date_shift", middleware.ExpensiveOpRateLimit(), instructor, r.batchHandler.BulkDateShift)
	protected.Post("/conversations/bulk", middleware.ExpensiveOpRateLimit(), r.batchHandler.BulkSendMessage)
	protected.Post("/courses/:course_id/enrollments/bulk", middleware.ExpensiveOpRateLimit(), instructor, r.batchHandler.BulkEnrollUsers)
	protected.Post("/courses/:course_id/assignments/bulk_update_dates", instructor, r.batchHandler.BulkUpdateAssignmentDates)

	// Announcements (view: enrolled; manage: instructor; global: admin)
	protected.Get("/courses/:course_id/announcements", enrolled, r.announcementHandler.ListCourseAnnouncements)
	protected.Post("/courses/:course_id/announcements", instructor, r.announcementHandler.CreateCourseAnnouncement)
	protected.Get("/announcements/:id", r.announcementHandler.GetAnnouncement)
	protected.Put("/announcements/:id", r.announcementHandler.UpdateAnnouncement)
	protected.Delete("/announcements/:id", r.announcementHandler.DeleteAnnouncement)
	protected.Post("/announcements/:id/read", r.announcementHandler.MarkAsRead)
	protected.Post("/announcements/:id/acknowledge", r.announcementHandler.AcknowledgeAnnouncement)
	protected.Get("/announcements/:id/read_receipts", instructor, r.announcementHandler.GetReadReceipts)
	protected.Get("/accounts/:account_id/announcements", r.announcementHandler.ListAccountAnnouncements)
	protected.Post("/accounts/:account_id/announcements", admin, r.announcementHandler.CreateAccountAnnouncement)

	// Enrollment Terms (admin only)
	protected.Get("/accounts/:account_id/terms", admin, r.enrollmentTermHandler.ListTerms)
	protected.Post("/accounts/:account_id/terms", admin, r.enrollmentTermHandler.CreateTerm)
	protected.Get("/accounts/:account_id/terms/current", admin, r.enrollmentTermHandler.GetCurrentTerm)
	protected.Get("/accounts/:account_id/terms/:id", admin, r.enrollmentTermHandler.GetTerm)
	protected.Put("/accounts/:account_id/terms/:id", admin, r.enrollmentTermHandler.UpdateTerm)
	protected.Delete("/accounts/:account_id/terms/:id", admin, r.enrollmentTermHandler.DeleteTerm)

	// Syllabus (enrolled)
	protected.Get("/courses/:course_id/syllabus", enrolled, r.syllabusHandler.GetSyllabus)

	// Notification Delivery (self or admin)
	protected.Get("/users/self/notification_deliveries", r.notificationDeliveryHandler.ListDeliveries)
	protected.Get("/admin/notification_stats", admin, r.notificationDeliveryHandler.GetDeliveryStats)
	protected.Post("/admin/notification_deliveries/retry", admin, r.notificationDeliveryHandler.RetryFailedDeliveries)
	protected.Get("/users/self/communication_channels", r.notificationDeliveryHandler.ListChannels)
	protected.Post("/users/self/communication_channels", r.notificationDeliveryHandler.CreateChannel)
	protected.Delete("/users/self/communication_channels/:id", r.notificationDeliveryHandler.DeleteChannel)

	// Audit Logs (course: instructor; account: admin)
	protected.Get("/courses/:course_id/audit_log", instructor, r.auditHandler.GetCourseAuditLog)
	protected.Get("/courses/:course_id/grade_change_log", instructor, r.auditHandler.GetCourseGradeChangeLog)
	protected.Get("/courses/:course_id/audit_log.csv", instructor, r.auditHandler.ExportCourseAuditLogCSV)
	protected.Get("/courses/:course_id/grade_change_log.csv", instructor, r.auditHandler.ExportCourseGradeChangeLogCSV)
	protected.Get("/accounts/:account_id/audit_log", admin, r.auditHandler.GetAccountAuditLog)
	protected.Get("/admin/audit_log/summary", admin, r.auditHandler.GetAuditLogSummary)

	// Custom Roles (admin only, except course permissions)
	protected.Get("/accounts/:account_id/roles", admin, r.customRoleHandler.ListRoles)
	protected.Post("/accounts/:account_id/roles", admin, r.customRoleHandler.CreateRole)
	protected.Get("/accounts/:account_id/roles/presets", admin, r.customRoleHandler.GetPresets)
	protected.Get("/accounts/:account_id/roles/:id", admin, r.customRoleHandler.GetRole)
	protected.Put("/accounts/:account_id/roles/:id", admin, r.customRoleHandler.UpdateRole)
	protected.Delete("/accounts/:account_id/roles/:id", admin, r.customRoleHandler.DeleteRole)
	protected.Post("/accounts/:account_id/roles/:id/clone", admin, r.customRoleHandler.CloneRole)
	protected.Get("/accounts/:account_id/roles/:id/overrides", admin, r.customRoleHandler.ListOverrides)
	protected.Put("/accounts/:account_id/roles/:id/overrides", admin, r.customRoleHandler.BulkSetOverrides)
	protected.Get("/courses/:course_id/permissions", enrolled, r.customRoleHandler.GetCoursePermissions)

	// OneRoster (admin only)
	protected.Get("/accounts/:account_id/oneroster_connections", admin, r.onerosterHandler.ListConnections)
	protected.Post("/accounts/:account_id/oneroster_connections", admin, r.onerosterHandler.CreateConnection)
	protected.Get("/accounts/:account_id/oneroster_connections/:id", admin, r.onerosterHandler.GetConnection)
	protected.Put("/accounts/:account_id/oneroster_connections/:id", admin, r.onerosterHandler.UpdateConnection)
	protected.Delete("/accounts/:account_id/oneroster_connections/:id", admin, r.onerosterHandler.DeleteConnection)
	protected.Post("/accounts/:account_id/oneroster_connections/:id/test", admin, r.onerosterHandler.TestConnection)
	protected.Post("/accounts/:account_id/oneroster_connections/:id/sync", admin, r.onerosterHandler.SyncFull)
	protected.Post("/accounts/:account_id/oneroster_connections/:id/sync_incremental", admin, r.onerosterHandler.SyncIncremental)
	protected.Get("/accounts/:account_id/oneroster_connections/:id/sync_logs", admin, r.onerosterHandler.GetSyncLogs)

	// Document Annotations (enrolled)
	protected.Get("/courses/:course_id/assignments/:assignment_id/submissions/:user_id/annotations", enrolled, r.documentAnnotationHandler.ListAnnotations)
	protected.Post("/courses/:course_id/assignments/:assignment_id/submissions/:user_id/annotations", enrolled, r.documentAnnotationHandler.CreateAnnotation)
	protected.Get("/courses/:course_id/assignments/:assignment_id/submissions/:user_id/annotation_summary", enrolled, r.documentAnnotationHandler.GetAnnotationSummary)
	protected.Get("/annotations/:id", r.documentAnnotationHandler.GetAnnotation)
	protected.Put("/annotations/:id", r.documentAnnotationHandler.UpdateAnnotation)
	protected.Delete("/annotations/:id", r.documentAnnotationHandler.DeleteAnnotation)
	protected.Post("/annotations/:id/resolve", r.documentAnnotationHandler.ResolveAnnotation)
	protected.Delete("/annotations/:id/resolve", r.documentAnnotationHandler.UnresolveAnnotation)
	protected.Post("/annotations/:id/replies", r.documentAnnotationHandler.ReplyToAnnotation)

	// COPPA / Parental Consent (admin + public verify)
	protected.Post("/consent/request", admin, r.coppaHandler.RequestConsent)
	protected.Get("/consent", admin, r.coppaHandler.ListConsents)
	protected.Post("/consent/verify/:token", r.coppaHandler.VerifyConsent)
	protected.Delete("/consent/:id", admin, r.coppaHandler.RevokeConsent)
	protected.Get("/data_processing_agreements", admin, r.coppaHandler.ListDPAs)
	protected.Post("/data_processing_agreements", admin, r.coppaHandler.CreateDPA)
	protected.Put("/data_processing_agreements/:id", admin, r.coppaHandler.UpdateDPA)

	// FERPA Compliance (self/admin)
	protected.Post("/users/:user_id/data_export", selfOrAdmin, r.ferpaHandler.CreateExportRequest)
	protected.Get("/users/:user_id/data_export/:id", selfOrAdmin, r.ferpaHandler.GetExportRequest)
	protected.Post("/users/:user_id/data_deletion", selfOrAdmin, r.ferpaHandler.CreateDeletionRequest)
	protected.Get("/admin/data_deletion_requests", admin, r.ferpaHandler.ListPendingDeletionRequests)
	protected.Post("/admin/data_deletion_requests/:id/approve", admin, r.ferpaHandler.ApproveDeletionRequest)
	protected.Get("/users/:user_id/pii_access_log", admin, r.ferpaHandler.GetPIIAccessLog)
	protected.Get("/admin/retention_policies", admin, r.ferpaHandler.ListRetentionPolicies)
	protected.Post("/admin/retention_policies", admin, r.ferpaHandler.CreateRetentionPolicy)
	protected.Get("/admin/retention_policies/:id", admin, r.ferpaHandler.GetRetentionPolicy)
	protected.Put("/admin/retention_policies/:id", admin, r.ferpaHandler.UpdateRetentionPolicy)
	protected.Delete("/admin/retention_policies/:id", admin, r.ferpaHandler.DeleteRetentionPolicy)

	// Student Accommodations (instructor/admin)
	protected.Get("/users/:user_id/accommodations", selfOrAdmin, r.accommodationHandler.ListUserAccommodations)
	protected.Post("/users/:user_id/accommodations", admin, r.accommodationHandler.CreateAccommodation)
	protected.Get("/accommodations/:id", r.accommodationHandler.GetAccommodation)
	protected.Put("/accommodations/:id", admin, r.accommodationHandler.UpdateAccommodation)
	protected.Delete("/accommodations/:id", admin, r.accommodationHandler.DeleteAccommodation)
	protected.Get("/courses/:course_id/accommodations", instructor, r.accommodationHandler.ListCourseAccommodations)
	protected.Post("/courses/:course_id/assignments/:assignment_id/apply_accommodations", instructor, r.accommodationHandler.ApplyAccommodationsToAssignment)

	// Attendance (view: enrolled; manage: instructor)
	protected.Post("/courses/:course_id/attendance", instructor, r.attendanceHandler.RecordAttendance)
	protected.Get("/courses/:course_id/attendance", enrolled, r.attendanceHandler.GetClassAttendance)
	protected.Get("/courses/:course_id/attendance/users/:user_id", enrolled, r.attendanceHandler.GetStudentAttendance)
	protected.Get("/courses/:course_id/attendance/users/:user_id/summary", enrolled, r.attendanceHandler.GetStudentAttendanceSummary)
	protected.Get("/courses/:course_id/attendance/export.csv", instructor, r.attendanceHandler.ExportAttendanceCSV)

	// Portfolios (self + public)
	protected.Get("/users/self/portfolios", r.portfolioHandler.ListUserPortfolios)
	protected.Post("/users/self/portfolios", r.portfolioHandler.CreatePortfolio)
	protected.Get("/portfolios/:id", r.portfolioHandler.GetPortfolio)
	protected.Put("/portfolios/:id", r.portfolioHandler.UpdatePortfolio)
	protected.Delete("/portfolios/:id", r.portfolioHandler.DeletePortfolio)
	protected.Post("/portfolios/:id/publish", r.portfolioHandler.PublishPortfolio)
	protected.Post("/portfolios/:id/sections", r.portfolioHandler.AddSection)
	protected.Put("/portfolios/:id/sections/:section_id", r.portfolioHandler.UpdateSection)
	protected.Delete("/portfolios/:id/sections/:section_id", r.portfolioHandler.DeleteSection)
	protected.Put("/portfolios/:id/sections/reorder", r.portfolioHandler.ReorderSections)
	protected.Post("/portfolios/:id/artifacts", r.portfolioHandler.AddArtifact)
	protected.Put("/portfolios/:id/artifacts/:artifact_id", r.portfolioHandler.UpdateArtifact)
	protected.Delete("/portfolios/:id/artifacts/:artifact_id", r.portfolioHandler.DeleteArtifact)
	protected.Post("/portfolios/:id/artifacts/:artifact_id/reflections", r.portfolioHandler.AddReflection)
	protected.Post("/portfolios/:id/import", r.portfolioHandler.ImportFromCourse)
	protected.Get("/portfolios/:id/export/html", r.portfolioHandler.ExportAsHTML)
	protected.Get("/portfolios/:id/export/pdf", r.portfolioHandler.ExportAsPDF)
	protected.Get("/portfolios/:id/comments", r.portfolioHandler.ListComments)
	protected.Post("/portfolios/:id/comments", r.portfolioHandler.AddComment)
	protected.Get("/portfolio_templates", r.portfolioHandler.ListTemplates)
	protected.Post("/portfolio_templates/:template_id/create", r.portfolioHandler.CreateFromTemplate)

	// Peer Reviews (assign/list: instructor; view own: enrolled; submit: enrolled)
	protected.Post("/courses/:course_id/assignments/:id/peer_reviews", instructor, r.peerReviewHandler.AssignPeerReviews)
	protected.Get("/courses/:course_id/assignments/:id/peer_reviews", instructor, r.peerReviewHandler.ListPeerReviews)
	protected.Get("/courses/:course_id/assignments/:id/peer_reviews/mine", enrolled, r.peerReviewHandler.ListMyPeerReviews)
	protected.Put("/peer_reviews/:review_id", r.peerReviewHandler.SubmitPeerReview)

	// Question Banks (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/question_banks", enrolled, r.questionBankHandler.ListBanks)
	protected.Post("/courses/:course_id/question_banks", instructor, r.questionBankHandler.CreateBank)
	protected.Get("/courses/:course_id/question_banks/:bank_id", enrolled, r.questionBankHandler.GetBank)
	protected.Put("/courses/:course_id/question_banks/:bank_id", instructor, r.questionBankHandler.UpdateBank)
	protected.Delete("/courses/:course_id/question_banks/:bank_id", instructor, r.questionBankHandler.DeleteBank)
	protected.Get("/courses/:course_id/question_banks/:bank_id/questions", enrolled, r.questionBankHandler.ListQuestions)
	protected.Post("/courses/:course_id/question_banks/:bank_id/questions", instructor, r.questionBankHandler.AddQuestion)
	protected.Put("/courses/:course_id/question_banks/:bank_id/questions/:question_id", instructor, r.questionBankHandler.UpdateQuestion)
	protected.Delete("/courses/:course_id/question_banks/:bank_id/questions/:question_id", instructor, r.questionBankHandler.DeleteQuestion)
	protected.Post("/courses/:course_id/question_banks/:bank_id/pull_to_quiz", instructor, r.questionBankHandler.PullToQuiz)

	// Module Prerequisites (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/modules/:id/prerequisites", enrolled, r.moduleHandler.GetPrerequisites)
	protected.Put("/courses/:course_id/modules/:id/prerequisites", instructor, r.moduleHandler.SetPrerequisites)

	// Public portfolio view (no auth required)
	api.Get("/portfolios/public/:slug", r.portfolioHandler.GetPublicPortfolio)

	// =====================================================================
	// P3 Features
	// =====================================================================

	// Feature Flags — Canvas-compatible API
	// Account-scoped (admin only)
	protected.Get("/accounts/:id/features", admin, r.featureFlagHandler.ListAccountFeatures)
	protected.Get("/accounts/:id/features/:feature", admin, r.featureFlagHandler.GetAccountFeature)
	protected.Put("/accounts/:id/features/:feature", admin, r.featureFlagHandler.SetAccountFeature)
	protected.Delete("/accounts/:id/features/:feature", admin, r.featureFlagHandler.DeleteAccountFeature)
	// Course-scoped (any enrolled user can read; teacher/admin can write)
	protected.Get("/courses/:id/features", enrolled, r.featureFlagHandler.ListCourseFeatures)
	protected.Get("/courses/:id/features/:feature", enrolled, r.featureFlagHandler.GetCourseFeature)
	protected.Put("/courses/:id/features/:feature", instructor, r.featureFlagHandler.SetCourseFeature)
	protected.Delete("/courses/:id/features/:feature", instructor, r.featureFlagHandler.DeleteCourseFeature)
	// Per-user (always self)
	protected.Get("/users/self/features", r.featureFlagHandler.ListUserFeatures)
	protected.Get("/users/self/features/:feature", r.featureFlagHandler.GetUserFeature)
	protected.Put("/users/self/features/:feature", r.featureFlagHandler.SetUserFeature)
	protected.Delete("/users/self/features/:feature", r.featureFlagHandler.DeleteUserFeature)

	// Custom Gradebook Columns (instructor-only)
	protected.Get("/courses/:id/custom_gradebook_columns", instructor, r.customGradebookColumnHandler.List)
	protected.Post("/courses/:id/custom_gradebook_columns", instructor, r.customGradebookColumnHandler.Create)
	protected.Put("/courses/:id/custom_gradebook_columns/:column_id", instructor, r.customGradebookColumnHandler.Update)
	protected.Delete("/courses/:id/custom_gradebook_columns/:column_id", instructor, r.customGradebookColumnHandler.Delete)
	protected.Post("/courses/:id/custom_gradebook_columns/reorder", instructor, r.customGradebookColumnHandler.Reorder)
	protected.Get("/courses/:id/custom_gradebook_columns/:column_id/data", instructor, r.customGradebookColumnHandler.ListData)
	protected.Put("/courses/:id/custom_gradebook_columns/:column_id/data/:user_id", instructor, r.customGradebookColumnHandler.SetCell)
	protected.Put("/courses/:id/custom_gradebook_columns/data", instructor, r.customGradebookColumnHandler.BulkUpdate)

	// Mastery Paths (Conditional Release) — instructor-only management
	protected.Get("/courses/:course_id/mastery_paths/rules", instructor, r.masteryPathHandler.ListRules)
	protected.Get("/courses/:course_id/mastery_paths/rules/:assignment_id", instructor, r.masteryPathHandler.GetRuleForAssignment)
	protected.Post("/courses/:course_id/mastery_paths/rules", instructor, r.masteryPathHandler.CreateRule)
	protected.Put("/courses/:course_id/mastery_paths/rules/:rule_id", instructor, r.masteryPathHandler.ReplaceRule)
	protected.Delete("/courses/:course_id/mastery_paths/rules/:rule_id", instructor, r.masteryPathHandler.DeleteRule)

	// Appointment Groups (Scheduler) — Canvas-compatible
	protected.Get("/courses/:course_id/appointment_groups", enrolled, r.appointmentGroupHandler.List)
	protected.Post("/courses/:course_id/appointment_groups", enrolled, r.appointmentGroupHandler.Create)
	protected.Get("/appointment_groups", r.appointmentGroupHandler.List) // accepts ?course_id=
	protected.Get("/appointment_groups/:id", r.appointmentGroupHandler.Get)
	protected.Put("/appointment_groups/:id", r.appointmentGroupHandler.Update)
	protected.Delete("/appointment_groups/:id", r.appointmentGroupHandler.Delete)
	protected.Get("/appointment_groups/:id/appointments", r.appointmentGroupHandler.ListSlots)
	protected.Get("/appointment_groups/:id/appointments/:slot_id/reservations", r.appointmentGroupHandler.ListReservations)
	protected.Post("/appointment_groups/:id/appointments/:slot_id/reservations", r.appointmentGroupHandler.Reserve)
	protected.Delete("/appointment_groups/:id/appointments/:slot_id/reservations/:reservation_id", r.appointmentGroupHandler.CancelReservation)

	// Outcome Proficiency — Account scope
	protected.Get("/accounts/:id/outcome_proficiency", admin, r.outcomeProficiencyHandler.GetForAccount)
	protected.Post("/accounts/:id/outcome_proficiency", admin, r.outcomeProficiencyHandler.SetForAccount)
	protected.Delete("/accounts/:id/outcome_proficiency", admin, r.outcomeProficiencyHandler.DeleteForAccount)
	// Outcome Proficiency — Course scope
	protected.Get("/courses/:id/outcome_proficiency", enrolled, r.outcomeProficiencyHandler.GetForCourse)
	protected.Post("/courses/:id/outcome_proficiency", instructor, r.outcomeProficiencyHandler.SetForCourse)
	protected.Delete("/courses/:id/outcome_proficiency", instructor, r.outcomeProficiencyHandler.DeleteForCourse)
	// Learning Mastery Gradebook
	protected.Get("/courses/:id/learning_mastery_gradebook", instructor, r.outcomeProficiencyHandler.LearningMasteryGradebook)

	// Discussion Checkpoints (Canvas-compatible multi-deadline thread participation)
	protected.Get("/courses/:course_id/discussion_topics/:topic_id/checkpoints", enrolled, r.discussionCheckpointHandler.ListCheckpoints)
	protected.Post("/courses/:course_id/discussion_topics/:topic_id/checkpoints", instructor, r.discussionCheckpointHandler.CreateCheckpoints)
	protected.Get("/courses/:course_id/discussion_topics/:topic_id/checkpoints/progress", enrolled, r.discussionCheckpointHandler.GetUserProgress)
	protected.Put("/courses/:course_id/discussion_topics/:topic_id/checkpoints/:id", instructor, r.discussionCheckpointHandler.UpdateCheckpoint)
	protected.Delete("/courses/:course_id/discussion_topics/:topic_id/checkpoints/:id", instructor, r.discussionCheckpointHandler.DeleteCheckpoint)

	// Smart Search (pgvector cosine similarity)
	protected.Get("/courses/:course_id/smart_search", enrolled, r.smartSearchHandler.Search)
	protected.Post("/courses/:course_id/smart_search/reindex", instructor, r.smartSearchHandler.Reindex)

	// Commons content library (district-scoped sharing).
	// IMPORTANT: register /commons/favorites BEFORE /commons/:id so the literal
	// path wins over the wildcard.
	protected.Get("/commons/favorites", r.commonsHandler.ListFavorites)
	protected.Get("/commons", r.commonsHandler.Browse)
	protected.Get("/commons/:id", r.commonsHandler.Get)
	protected.Post("/commons/:id/favorite", r.commonsHandler.Favorite)
	protected.Post("/commons/:id/import", r.commonsHandler.Import)
	protected.Post("/courses/:course_id/commons/publish", instructor, r.commonsHandler.Publish)

	// AI Assist proxy for RCE V2 toolbar (Anthropic Messages API).
	// Per-user rate limit (30 / 5 min) is the cost gate — any authenticated user
	// can call it, but no single account can drain the API budget.
	protected.Post("/ai_assist/:action", middleware.AIAssistRateLimit(), r.aiAssistHandler.Dispatch)

	// =====================================================================
	// Wave A2: Quiz Item Banks, Stimulus Passages, Per-Question Outcome Alignment
	// =====================================================================

	// Quiz Item Banks (course-scoped reusable question library).
	protected.Get("/courses/:course_id/quiz_item_banks", enrolled, r.quizItemBankHandler.ListBanks)
	protected.Post("/courses/:course_id/quiz_item_banks", instructor, r.quizItemBankHandler.CreateBank)
	protected.Get("/courses/:course_id/quiz_item_banks/:bank_id", enrolled, r.quizItemBankHandler.GetBank)
	protected.Put("/courses/:course_id/quiz_item_banks/:bank_id", instructor, r.quizItemBankHandler.UpdateBank)
	protected.Delete("/courses/:course_id/quiz_item_banks/:bank_id", instructor, r.quizItemBankHandler.DeleteBank)

	// Quiz Item Bank Items (the reusable templates inside a bank).
	protected.Get("/quiz_item_banks/:bank_id/items", r.quizItemBankHandler.ListBankItems)
	protected.Post("/quiz_item_banks/:bank_id/items", r.quizItemBankHandler.CreateBankItem)
	protected.Get("/quiz_item_banks/:bank_id/items/:item_id", r.quizItemBankHandler.GetBankItem)
	protected.Put("/quiz_item_banks/:bank_id/items/:item_id", r.quizItemBankHandler.UpdateBankItem)
	protected.Delete("/quiz_item_banks/:bank_id/items/:item_id", r.quizItemBankHandler.DeleteBankItem)

	// Quiz integration: copy an item into a quiz, or draw N random items from a bank.
	protected.Post("/quiz_item_banks/:bank_id/items/:item_id/add_to_quiz/:quiz_id", instructor, r.quizItemBankHandler.AddBankItemToQuiz)
	protected.Post("/quiz_item_banks/:bank_id/random_draw", instructor, r.quizItemBankHandler.RandomDraw)

	// Stimulus passages (TipTap docs shared across multiple quiz questions).
	protected.Get("/courses/:course_id/quiz_stimuli", enrolled, r.quizStimulusHandler.ListStimuli)
	protected.Post("/courses/:course_id/quiz_stimuli", instructor, r.quizStimulusHandler.CreateStimulus)
	protected.Get("/courses/:course_id/quiz_stimuli/:stimulus_id", enrolled, r.quizStimulusHandler.GetStimulus)
	protected.Put("/courses/:course_id/quiz_stimuli/:stimulus_id", instructor, r.quizStimulusHandler.UpdateStimulus)
	protected.Delete("/courses/:course_id/quiz_stimuli/:stimulus_id", instructor, r.quizStimulusHandler.DeleteStimulus)
	protected.Get("/quiz_stimuli/:stimulus_id/questions", r.quizStimulusHandler.ListQuestions)
	protected.Post("/quiz_stimuli/:stimulus_id/questions/:question_id", instructor, r.quizStimulusHandler.LinkQuestion)
	protected.Delete("/quiz_stimuli/:stimulus_id/questions/:question_id", instructor, r.quizStimulusHandler.UnlinkQuestion)

	// Per-question outcome alignment (data layer only — grader does not consume yet).
	protected.Get("/quiz_questions/:question_id/outcome_alignments", r.quizOutcomeAlignmentHandler.ListByQuestion)
	protected.Post("/quiz_questions/:question_id/outcome_alignments", instructor, r.quizOutcomeAlignmentHandler.Align)
	protected.Delete("/quiz_questions/:question_id/outcome_alignments/:outcome_id", instructor, r.quizOutcomeAlignmentHandler.Unalign)
	protected.Get("/learning_outcomes/:outcome_id/quiz_question_alignments", r.quizOutcomeAlignmentHandler.ListByOutcome)

	// Wave B: Canvas QTI / IMSCC import + export.
	// Sync-only in v1. The handler blocks while parsing + persisting;
	// Canvas-sized exports complete in well under a second.
	if r.qtiImportHandler != nil {
		protected.Post("/courses/:course_id/qti_import", instructor, r.qtiImportHandler.Import)
		protected.Get("/quizzes/:quiz_id/export.imscc", instructor, r.qtiImportHandler.Export)
	}

	// =====================================================================
	// Phase 6 Wave 1: Gamification read API.
	// Handler enforces its own self-or-admin check (no route-level middleware
	// because the user-id is in the URL, not derived from a course).
	// =====================================================================
	if r.gamificationHandler != nil {
		gam := protected.Group("/gamification")
		gam.Get("/currencies", r.gamificationHandler.ListCurrencies)
		gam.Post("/currencies", admin, r.gamificationHandler.CreateCurrency)
		gam.Patch("/currencies/:id", admin, r.gamificationHandler.UpdateCurrency)
		gam.Delete("/currencies/:id", admin, r.gamificationHandler.DeleteCurrency)
		protected.Get("/users/:id/wallet", r.gamificationHandler.GetUserWallet)
		protected.Get("/users/:id/wallet/transactions", r.gamificationHandler.ListUserWalletTransactions)

		// Course-scoped instructor surface. Same handler, scope inferred
		// from :course_id presence in the URL.
		protected.Post("/courses/:course_id/gamification/currencies", instructor, r.gamificationHandler.CreateCurrency)
		protected.Patch("/courses/:course_id/gamification/currencies/:id", instructor, r.gamificationHandler.UpdateCurrency)
		protected.Delete("/courses/:course_id/gamification/currencies/:id", instructor, r.gamificationHandler.DeleteCurrency)

		// Per-learner gamification preferences (W2-C). Self-only; the
		// handler reads user_id from Locals and never accepts another
		// user's id in the path. Currently exposes the leaderboard
		// opt-out toggle.
		protected.Get("/users/self/gamification_preferences", r.gamificationHandler.GetMyGamificationPreferences)
		protected.Put("/users/self/gamification_preferences", r.gamificationHandler.UpdateMyGamificationPreferences)
	}
}
