package v1

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/auth"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

// Router holds every dependency Register needs to wire HTTP routes. All
// fields are exported so callers can build the struct as a literal,
// avoiding a multi-page positional constructor. Add a new handler by
// adding a field here and assigning it at the call site.
type Router struct {
	// Core domain
	UserHandler            *handlers.UserHandler
	AccountHandler         *handlers.AccountHandler
	CourseHandler          *handlers.CourseHandler
	SectionHandler         *handlers.SectionHandler
	EnrollmentHandler      *handlers.EnrollmentHandler
	ModuleHandler          *handlers.ModuleHandler
	ModuleItemHandler      *handlers.ModuleItemHandler
	PageHandler            *handlers.PageHandler
	AssignmentHandler      *handlers.AssignmentHandler
	AssignmentGroupHandler *handlers.AssignmentGroupHandler
	SubmissionHandler      *handlers.SubmissionHandler
	GradebookHandler       *handlers.GradebookHandler
	GradingStandardHandler *handlers.GradingStandardHandler

	// Developer keys, tokens, OAuth, external tools, LTI
	DeveloperKeyHandler *handlers.DeveloperKeyHandler
	AccessTokenHandler  *handlers.AccessTokenHandler
	OAuth2Handler       *handlers.OAuth2Handler
	ExternalToolHandler *handlers.ExternalToolHandler
	LTIHandler          *handlers.LTIHandler

	// Discussions, files
	DiscussionHandler      *handlers.DiscussionHandler
	DiscussionEntryHandler *handlers.DiscussionEntryHandler
	FileHandler            *handlers.FileHandler
	FolderHandler          *handlers.FolderHandler
	SISImportHandler       *handlers.SISImportHandler

	// Quizzing
	QuizHandler               *handlers.QuizHandler
	QuizQuestionHandler       *handlers.QuizQuestionHandler
	QuizSubmissionHandler     *handlers.QuizSubmissionHandler
	RubricHandler             *handlers.RubricHandler
	RubricAssessmentHandler   *handlers.RubricAssessmentHandler
	GradingPeriodHandler      *handlers.GradingPeriodHandler
	AssignmentOverrideHandler *handlers.AssignmentOverrideHandler
	LatePolicyHandler         *handlers.LatePolicyHandler

	// Communication
	CalendarEventHandler *handlers.CalendarEventHandler
	ConversationHandler  *handlers.ConversationHandler
	NotificationHandler  *handlers.NotificationHandler

	// Content migration, outcomes, speed grader
	ContentMigrationHandler *handlers.ContentMigrationHandler
	LearningOutcomeHandler  *handlers.LearningOutcomeHandler
	SpeedGraderHandler      *handlers.SpeedGraderHandler

	// Groups, blueprints, pacing
	GroupHandler         *handlers.GroupHandler
	BlueprintHandler     *handlers.BlueprintHandler
	CoursePaceHandler    *handlers.CoursePaceHandler
	CollaborationHandler *handlers.CollaborationHandler
	ConferenceHandler    *handlers.ConferenceHandler
	AnalyticsHandler     *handlers.AnalyticsHandler
	ObserverHandler      *handlers.ObserverHandler

	// GraphQL, auth providers, discussions v2, content import, batch
	GraphQLHandler       *handlers.GraphQLHandler
	AuthProviderHandler  *handlers.AuthProviderHandler
	DiscussionV2Handler  *handlers.DiscussionV2Handler
	ContentImportHandler *handlers.ContentImportHandler
	BatchHandler         *handlers.BatchHandler

	// SSO / federated auth / MFA / passkeys
	SSOHandler     *auth.SSOHandler
	OIDCHandler    *auth.OIDCHandler
	MFAHandler     *handlers.MFAHandler
	PasskeyHandler *handlers.PasskeyHandler

	// Announcements, terms, syllabus
	AnnouncementHandler   *handlers.AnnouncementHandler
	EnrollmentTermHandler *handlers.EnrollmentTermHandler
	SyllabusHandler       *handlers.SyllabusHandler

	// Notifications, audit
	NotificationDeliveryHandler *handlers.NotificationDeliveryHandler
	AuditHandler                *handlers.AuditHandler

	// Roles, rostering, document annotations
	CustomRoleHandler         *handlers.CustomRoleHandler
	OneRosterHandler          *handlers.OneRosterHandler
	DocumentAnnotationHandler *handlers.DocumentAnnotationHandler

	// Compliance (COPPA / FERPA), accommodations, attendance, portfolio
	COPPAHandler         *handlers.COPPAHandler
	FERPAHandler         *handlers.FERPAHandler
	AccommodationHandler *handlers.AccommodationHandler
	AttendanceHandler    *handlers.AttendanceHandler
	PortfolioHandler     *handlers.PortfolioHandler

	// Course home engine
	CourseHomeHandler *handlers.CourseHomeHandler

	// Peer reviews, question banks, quiz groups + statistics, setup
	PeerReviewHandler        *handlers.PeerReviewHandler
	QuestionBankHandler      *handlers.QuestionBankHandler
	QuizQuestionGroupHandler *handlers.QuizQuestionGroupHandler
	QuizStatisticsHandler    *handlers.QuizStatisticsHandler
	SetupHandler             *handlers.SetupHandler

	// P3 features
	FeatureFlagHandler           *handlers.FeatureFlagHandler
	CustomGradebookColumnHandler *handlers.CustomGradebookColumnHandler
	MasteryPathHandler           *handlers.MasteryPathHandler
	AppointmentGroupHandler      *handlers.AppointmentGroupHandler
	OutcomeProficiencyHandler    *handlers.OutcomeProficiencyHandler

	// Pairing codes (parent/observer)
	PairingCodeHandler *handlers.PairingCodeHandler

	// Discussion checkpoints, smart search, commons, AI assist
	DiscussionCheckpointHandler *handlers.DiscussionCheckpointHandler
	SmartSearchHandler          *handlers.SmartSearchHandler
	CommonsHandler              *handlers.CommonsHandler
	AIAssistHandler             *handlers.AIAssistHandler

	// Quiz item banks, stimuli, per-question outcome alignments
	QuizItemBankHandler         *handlers.QuizItemBankHandler
	QuizStimulusHandler         *handlers.QuizStimulusHandler
	QuizOutcomeAlignmentHandler *handlers.QuizOutcomeAlignmentHandler

	// QTI / IMSCC import + export
	QTIImportHandler *handlers.QTIImportHandler

	// Phase 6: gamification
	GamificationHandler *handlers.GamificationHandler

	// Super-Admin Settings Engine — Wave 2 read-only surface for
	// /api/v1/superadmin/settings*. Mounted via registerSuperAdminRoutes,
	// which is the only authorized place to attach /superadmin/*
	// routes; that helper carries the RequireSuperAdmin gate.
	SuperAdminSettingsHandler *handlers.SuperAdminSettingsHandler

	// Middleware
	AuthMiddleware *middleware.AuthMiddleware
	PermMiddleware *middleware.PermissionMiddleware

	// AccountRepo is needed by middleware mounted in Register (tenant
	// scope resolution on a few read paths).
	AccountRepo repository.AccountRepository

	// AuditService is wired so the global AuditWrites middleware can
	// emit an audit_log row on every successful 2xx write inside the
	// protected group. Single mount; covers ~333 write routes.
	AuditService *service.AuditService

	// SettingsLookup resolves catalog keys through the Settings Engine
	// for middlewares mounted in Register. Today: EnforceUploadSize on
	// the two upload routes reads `quotas.max_upload_size_mb`. Shared
	// closure wired up in cmd/server/main.go alongside the SMTP / AI
	// Assist / OIDC / passkey consumers.
	SettingsLookup middleware.UploadSizeLookupFunc
}

// NewRouter is kept as an identity helper so callers reading older
// examples still find a constructor. New code can build &Router{...}
// directly.
func NewRouter(r Router) *Router {
	return &r
}

func (r *Router) Register(app *fiber.App) {
	api := app.Group("/api/v1", middleware.PaginationParams())

	// Permission middleware aliases for readability
	admin := r.PermMiddleware.RequireAdmin()
	enrolled := r.PermMiddleware.RequireEnrolled()
	instructor := r.PermMiddleware.RequireInstructor()
	selfOrAdmin := r.PermMiddleware.RequireSelfOrAdmin()

	authLimit := middleware.AuthRateLimit()
	r.registerPublicRoutes(api, authLimit)

	// Protected routes (authentication required)
	protected := api.Group("", r.AuthMiddleware.Protected(), middleware.CSRFProtection())

	// 13.5 — global AuditWrites mount. Filters by HTTP method (POST/PUT/
	// PATCH/DELETE) and 2xx status inside the middleware, so a single
	// `protected.Use(...)` covers every authenticated write route
	// (~333 of them) without per-route plumbing. MUST sit before any
	// route declarations on this group.
	protected.Use(middleware.AuditWrites(r.AuditService, "http.write"))

	// Users (self access or admin)
	protected.Get("/users/self", r.UserHandler.GetSelf)
	protected.Post("/users/self/change_password", r.UserHandler.ChangePassword)
	protected.Get("/users", admin, r.UserHandler.ListUsers)
	protected.Get("/users/:id", selfOrAdmin, r.UserHandler.GetUser)
	protected.Get("/users/:id/profile", selfOrAdmin, r.UserHandler.GetUserProfile)
	protected.Put("/users/:id", selfOrAdmin, r.UserHandler.UpdateUser)
	protected.Put("/users/:id/role", admin, r.UserHandler.UpdateUserRole)

	// Masquerade (admin only)
	protected.Post("/users/:id/masquerade", admin, r.UserHandler.StartMasquerade)
	protected.Delete("/masquerade", r.UserHandler.EndMasquerade)

	// Personal Access Tokens (self or admin)
	protected.Get("/users/:user_id/tokens", selfOrAdmin, r.AccessTokenHandler.ListAccessTokens)
	protected.Post("/users/:user_id/tokens", selfOrAdmin, r.AccessTokenHandler.CreateAccessToken)
	protected.Delete("/users/:user_id/tokens/:id", selfOrAdmin, r.AccessTokenHandler.DeleteAccessToken)

	// Accounts (admin only)
	protected.Get("/accounts", admin, r.AccountHandler.ListAccounts)
	protected.Get("/accounts/:id", admin, r.AccountHandler.GetAccount)
	protected.Put("/accounts/:id", admin, r.AccountHandler.UpdateAccount)

	// Developer Keys (admin only)
	protected.Get("/accounts/:account_id/developer_keys", admin, r.DeveloperKeyHandler.ListDeveloperKeys)
	protected.Post("/accounts/:account_id/developer_keys", admin, r.DeveloperKeyHandler.CreateDeveloperKey)
	protected.Get("/accounts/:account_id/developer_keys/:id", admin, r.DeveloperKeyHandler.GetDeveloperKey)
	protected.Put("/accounts/:account_id/developer_keys/:id", admin, r.DeveloperKeyHandler.UpdateDeveloperKey)
	protected.Delete("/accounts/:account_id/developer_keys/:id", admin, r.DeveloperKeyHandler.DeleteDeveloperKey)

	// OAuth2 Authorization (requires auth for consent)
	protected.Get("/login/oauth2/auth", r.OAuth2Handler.Authorize)
	protected.Post("/login/oauth2/auth", r.OAuth2Handler.AuthorizePost)

	// Courses (list: any user sees their own; create: admin; manage: instructor)
	protected.Get("/courses", r.CourseHandler.ListCourses)
	protected.Post("/courses", admin, r.CourseHandler.CreateCourse)
	protected.Get("/courses/:id", enrolled, r.CourseHandler.GetCourse)
	protected.Put("/courses/:id", instructor, r.CourseHandler.UpdateCourse)
	protected.Delete("/courses/:id", instructor, r.CourseHandler.DeleteCourse)

	// External Tools (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/external_tools", enrolled, r.ExternalToolHandler.ListExternalTools)
	protected.Post("/courses/:course_id/external_tools", instructor, r.ExternalToolHandler.CreateExternalTool)
	protected.Get("/courses/:course_id/external_tools/:id", enrolled, r.ExternalToolHandler.GetExternalTool)
	protected.Put("/courses/:course_id/external_tools/:id", instructor, r.ExternalToolHandler.UpdateExternalTool)
	protected.Delete("/courses/:course_id/external_tools/:id", instructor, r.ExternalToolHandler.DeleteExternalTool)

	// Sections (view: enrolled; create: instructor)
	protected.Get("/courses/:course_id/sections", enrolled, r.SectionHandler.ListSections)
	protected.Post("/courses/:course_id/sections", instructor, r.SectionHandler.CreateSection)
	protected.Get("/sections/:id", r.SectionHandler.GetSection)

	// Enrollments (view: enrolled; create: instructor)
	protected.Get("/courses/:course_id/enrollments", enrolled, r.EnrollmentHandler.ListEnrollments)
	protected.Post("/courses/:course_id/enrollments", instructor, r.EnrollmentHandler.CreateEnrollment)

	// Modules (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/modules", enrolled, r.ModuleHandler.ListModules)
	protected.Post("/courses/:course_id/modules", instructor, r.ModuleHandler.CreateModule)
	protected.Get("/courses/:course_id/modules/:id", enrolled, r.ModuleHandler.GetModule)
	protected.Put("/courses/:course_id/modules/:id", instructor, r.ModuleHandler.UpdateModule)
	protected.Delete("/courses/:course_id/modules/:id", instructor, r.ModuleHandler.DeleteModule)
	protected.Post("/courses/:course_id/modules/reorder", instructor, r.ModuleHandler.ReorderModules)

	// Course Home Engine (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/home", enrolled, r.CourseHomeHandler.GetHomeData)
	protected.Post("/courses/:course_id/home/visit", enrolled, r.CourseHomeHandler.RecordVisit)
	protected.Get("/courses/:course_id/home/buttons", enrolled, r.CourseHomeHandler.ListButtons)
	protected.Post("/courses/:course_id/home/buttons", instructor, r.CourseHomeHandler.CreateButton)
	protected.Put("/courses/:course_id/home/buttons/reorder", instructor, r.CourseHomeHandler.ReorderButtons)
	protected.Put("/courses/:course_id/home/buttons/:id", instructor, r.CourseHomeHandler.UpdateButton)
	protected.Delete("/courses/:course_id/home/buttons/:id", instructor, r.CourseHomeHandler.DeleteButton)
	protected.Get("/courses/:course_id/home/overrides", instructor, r.CourseHomeHandler.ListOverrides)
	protected.Post("/courses/:course_id/home/overrides", instructor, r.CourseHomeHandler.CreateOverride)
	protected.Put("/courses/:course_id/home/overrides/:id", instructor, r.CourseHomeHandler.UpdateOverride)
	protected.Delete("/courses/:course_id/home/overrides/:id", instructor, r.CourseHomeHandler.DeleteOverride)

	// Module Items (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/modules/:module_id/items", enrolled, r.ModuleItemHandler.ListModuleItems)
	protected.Post("/courses/:course_id/modules/:module_id/items", instructor, r.ModuleItemHandler.CreateModuleItem)
	protected.Get("/courses/:course_id/modules/:module_id/items/:item_id", enrolled, r.ModuleItemHandler.GetModuleItem)
	protected.Put("/courses/:course_id/modules/:module_id/items/:item_id", instructor, r.ModuleItemHandler.UpdateModuleItem)
	protected.Delete("/courses/:course_id/modules/:module_id/items/:item_id", instructor, r.ModuleItemHandler.DeleteModuleItem)
	protected.Post("/courses/:course_id/modules/:module_id/items/reorder", instructor, r.ModuleItemHandler.ReorderItems)
	protected.Post("/courses/:course_id/modules/:module_id/items/:item_id/move", instructor, r.ModuleItemHandler.MoveItem)

	// Pages (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/pages", enrolled, r.PageHandler.ListPages)
	protected.Post("/courses/:course_id/pages", instructor, r.PageHandler.CreatePage)
	protected.Get("/courses/:course_id/pages/:url_or_id", enrolled, r.PageHandler.GetPage)
	protected.Put("/courses/:course_id/pages/:url_or_id", instructor, r.PageHandler.UpdatePage)
	protected.Delete("/courses/:course_id/pages/:url_or_id", instructor, r.PageHandler.DeletePage)

	// Assignments (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/assignments", enrolled, r.AssignmentHandler.ListAssignments)
	protected.Post("/courses/:course_id/assignments", instructor, r.AssignmentHandler.CreateAssignment)
	protected.Get("/courses/:course_id/assignments/:id", enrolled, r.AssignmentHandler.GetAssignment)
	protected.Put("/courses/:course_id/assignments/:id", instructor, r.AssignmentHandler.UpdateAssignment)
	protected.Delete("/courses/:course_id/assignments/:id", instructor, r.AssignmentHandler.DeleteAssignment)

	// Assignment Groups (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/assignment_groups", enrolled, r.AssignmentGroupHandler.ListAssignmentGroups)
	protected.Post("/courses/:course_id/assignment_groups", instructor, r.AssignmentGroupHandler.CreateAssignmentGroup)
	protected.Get("/courses/:course_id/assignment_groups/:id", enrolled, r.AssignmentGroupHandler.GetAssignmentGroup)
	protected.Put("/courses/:course_id/assignment_groups/:id", instructor, r.AssignmentGroupHandler.UpdateAssignmentGroup)
	protected.Delete("/courses/:course_id/assignment_groups/:id", instructor, r.AssignmentGroupHandler.DeleteAssignmentGroup)

	// Course-wide submissions (enrolled users; students see only their own)
	protected.Get("/courses/:course_id/submissions", enrolled, r.SubmissionHandler.ListCourseSubmissions)
	protected.Post("/courses/:course_id/submissions/bulk_grade", instructor, r.SubmissionHandler.BulkGrade)

	// Submissions (view: enrolled; create: enrolled; grade: instructor)
	protected.Get("/courses/:course_id/assignments/:assignment_id/submissions", enrolled, r.SubmissionHandler.ListSubmissions)
	protected.Post("/courses/:course_id/assignments/:assignment_id/submissions", enrolled, r.SubmissionHandler.CreateSubmission)
	protected.Get("/courses/:course_id/assignments/:assignment_id/submissions/:user_id", enrolled, r.SubmissionHandler.GetSubmission)
	protected.Put("/courses/:course_id/assignments/:assignment_id/submissions/:user_id", instructor, r.SubmissionHandler.UpdateSubmission)

	// Submission Comments (view/create: enrolled)
	protected.Get("/courses/:course_id/assignments/:assignment_id/submissions/:user_id/comments", enrolled, r.SubmissionHandler.ListSubmissionComments)
	protected.Post("/courses/:course_id/assignments/:assignment_id/submissions/:user_id/comments", enrolled, r.SubmissionHandler.CreateSubmissionComment)

	// Gradebook (instructor only)
	protected.Get("/courses/:course_id/gradebook", instructor, r.GradebookHandler.GetGradebook)
	protected.Get("/courses/:course_id/students/:student_id/grade", instructor, r.GradebookHandler.GetStudentGrade)

	// Grading Standards (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/grading_standards", enrolled, r.GradingStandardHandler.ListGradingStandards)
	protected.Post("/courses/:course_id/grading_standards", instructor, r.GradingStandardHandler.CreateGradingStandard)
	protected.Put("/courses/:course_id/grading_standards/:id", instructor, r.GradingStandardHandler.UpdateGradingStandard)
	protected.Delete("/courses/:course_id/grading_standards/:id", instructor, r.GradingStandardHandler.DeleteGradingStandard)

	// LTI AGS (Assignment and Grade Services) - protected via OAuth2 token + enrollment
	protected.Get("/lti/courses/:course_id/line_items", enrolled, r.LTIHandler.ListLineItems)
	protected.Post("/lti/courses/:course_id/line_items", instructor, r.LTIHandler.CreateLineItem)
	protected.Get("/lti/courses/:course_id/line_items/:id", enrolled, r.LTIHandler.GetLineItem)
	protected.Put("/lti/courses/:course_id/line_items/:id", instructor, r.LTIHandler.UpdateLineItem)
	protected.Delete("/lti/courses/:course_id/line_items/:id", instructor, r.LTIHandler.DeleteLineItem)
	protected.Post("/lti/courses/:course_id/line_items/:id/scores", instructor, r.LTIHandler.PostScore)
	protected.Get("/lti/courses/:course_id/line_items/:id/results", enrolled, r.LTIHandler.GetResults)

	// LTI NRPS (Names and Role Provisioning Services)
	protected.Get("/lti/courses/:course_id/memberships", enrolled, r.LTIHandler.GetMemberships)

	// Discussion Topics (view: enrolled; manage: instructor; post: enrolled)
	protected.Get("/courses/:course_id/discussion_topics", enrolled, r.DiscussionHandler.ListTopics)
	protected.Post("/courses/:course_id/discussion_topics", instructor, r.DiscussionHandler.CreateTopic)
	protected.Get("/courses/:course_id/discussion_topics/:topic_id", enrolled, r.DiscussionHandler.GetTopic)
	protected.Put("/courses/:course_id/discussion_topics/:topic_id", instructor, r.DiscussionHandler.UpdateTopic)
	protected.Delete("/courses/:course_id/discussion_topics/:topic_id", instructor, r.DiscussionHandler.DeleteTopic)
	protected.Get("/courses/:course_id/discussion_topics/:topic_id/view", enrolled, r.DiscussionHandler.GetFullView)

	// Discussion Entries (view/post: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/discussion_topics/:topic_id/entries", enrolled, r.DiscussionEntryHandler.ListEntries)
	protected.Post("/courses/:course_id/discussion_topics/:topic_id/entries", enrolled, r.DiscussionEntryHandler.CreateEntry)
	protected.Put("/courses/:course_id/discussion_topics/:topic_id/entries/:id", enrolled, r.DiscussionEntryHandler.UpdateEntry)
	protected.Delete("/courses/:course_id/discussion_topics/:topic_id/entries/:id", instructor, r.DiscussionEntryHandler.DeleteEntry)
	protected.Get("/courses/:course_id/discussion_topics/:topic_id/entries/:entry_id/replies", enrolled, r.DiscussionEntryHandler.ListReplies)
	protected.Post("/courses/:course_id/discussion_topics/:topic_id/entries/:entry_id/replies", enrolled, r.DiscussionEntryHandler.CreateReply)
	protected.Post("/courses/:course_id/discussion_topics/:topic_id/entries/:entry_id/rating", enrolled, r.DiscussionEntryHandler.RateEntry)

	// Files (view: enrolled; upload/delete: instructor)
	protected.Get("/courses/:course_id/files", enrolled, r.FileHandler.ListCourseFiles)
	protected.Post("/courses/:course_id/files", middleware.UploadRateLimit(), middleware.EnforceUploadSize(r.SettingsLookup), instructor, r.FileHandler.UploadCourseFile)
	protected.Get("/courses/:course_id/files/:id", enrolled, r.FileHandler.GetFile)
	protected.Delete("/courses/:course_id/files/:id", instructor, r.FileHandler.DeleteFile)
	protected.Get("/files/:id/download", r.FileHandler.DownloadFile)
	protected.Get("/folders/:folder_id/files", r.FileHandler.ListFolderFiles)

	// Folders (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/folders", enrolled, r.FolderHandler.ListCourseFolders)
	protected.Post("/courses/:course_id/folders", instructor, r.FolderHandler.CreateCourseFolder)
	protected.Get("/folders/:id", r.FolderHandler.GetFolder)
	protected.Put("/folders/:id", r.FolderHandler.UpdateFolder)
	protected.Delete("/folders/:id", r.FolderHandler.DeleteFolder)
	protected.Get("/folders/:folder_id/folders", r.FolderHandler.ListSubfolders)

	// SIS Import/Export (admin only)
	protected.Post("/accounts/:account_id/sis_imports", middleware.UploadRateLimit(), admin, r.SISImportHandler.CreateSISImport)
	protected.Get("/accounts/:account_id/sis_imports", admin, r.SISImportHandler.ListSISImports)
	protected.Get("/accounts/:account_id/sis_imports/:id", admin, r.SISImportHandler.GetSISImport)
	protected.Get("/accounts/:account_id/sis_imports/:id/errors", admin, r.SISImportHandler.GetSISImportErrors)
	protected.Get("/accounts/:account_id/sis_exports/users.csv", admin, r.SISImportHandler.ExportUsersCSV)
	protected.Get("/accounts/:account_id/sis_exports/courses.csv", admin, r.SISImportHandler.ExportCoursesCSV)
	protected.Get("/accounts/:account_id/sis_exports/sections.csv", admin, r.SISImportHandler.ExportSectionsCSV)
	protected.Get("/accounts/:account_id/sis_exports/enrollments.csv", admin, r.SISImportHandler.ExportEnrollmentsCSV)

	// Quizzes (view: enrolled; manage: instructor; take: enrolled)
	protected.Get("/courses/:course_id/quizzes", enrolled, r.QuizHandler.ListQuizzes)
	protected.Post("/courses/:course_id/quizzes", instructor, r.QuizHandler.CreateQuiz)
	protected.Get("/courses/:course_id/quizzes/:id", enrolled, r.QuizHandler.GetQuiz)
	protected.Put("/courses/:course_id/quizzes/:id", instructor, r.QuizHandler.UpdateQuiz)
	protected.Delete("/courses/:course_id/quizzes/:id", instructor, r.QuizHandler.DeleteQuiz)

	// Quiz Questions (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/quizzes/:quiz_id/questions", enrolled, r.QuizQuestionHandler.ListQuestions)
	protected.Post("/courses/:course_id/quizzes/:quiz_id/questions", instructor, r.QuizQuestionHandler.CreateQuestion)
	protected.Get("/courses/:course_id/quizzes/:quiz_id/questions/:question_id", enrolled, r.QuizQuestionHandler.GetQuestion)
	protected.Put("/courses/:course_id/quizzes/:quiz_id/questions/:question_id", instructor, r.QuizQuestionHandler.UpdateQuestion)
	protected.Delete("/courses/:course_id/quizzes/:quiz_id/questions/:question_id", instructor, r.QuizQuestionHandler.DeleteQuestion)

	// Quiz Submissions (take: enrolled; view: enrolled)
	protected.Post("/courses/:course_id/quizzes/:quiz_id/submissions", enrolled, r.QuizSubmissionHandler.StartSubmission)
	protected.Get("/courses/:course_id/quizzes/:quiz_id/submissions", enrolled, r.QuizSubmissionHandler.ListSubmissions)
	protected.Get("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id", enrolled, r.QuizSubmissionHandler.GetSubmission)
	protected.Put("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/questions/:question_id", enrolled, r.QuizSubmissionHandler.AnswerQuestion)
	protected.Post("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/complete", enrolled, r.QuizSubmissionHandler.CompleteSubmission)
	protected.Get("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/answers", enrolled, r.QuizSubmissionHandler.GetSubmissionAnswers)
	protected.Get("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/questions", enrolled, r.QuizSubmissionHandler.GetSubmissionQuestions)

	// Quiz Statistics (instructor only)
	protected.Get("/courses/:course_id/quizzes/:quiz_id/statistics", instructor, r.QuizStatisticsHandler.GetQuizStatistics)

	// Quiz Question Groups (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/quizzes/:quiz_id/groups", enrolled, r.QuizQuestionGroupHandler.ListGroups)
	protected.Post("/courses/:course_id/quizzes/:quiz_id/groups", instructor, r.QuizQuestionGroupHandler.CreateGroup)
	protected.Get("/courses/:course_id/quizzes/:quiz_id/groups/:group_id", enrolled, r.QuizQuestionGroupHandler.GetGroup)
	protected.Put("/courses/:course_id/quizzes/:quiz_id/groups/:group_id", instructor, r.QuizQuestionGroupHandler.UpdateGroup)
	protected.Delete("/courses/:course_id/quizzes/:quiz_id/groups/:group_id", instructor, r.QuizQuestionGroupHandler.DeleteGroup)

	// Rubrics (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/rubrics", enrolled, r.RubricHandler.ListCourseRubrics)
	protected.Post("/courses/:course_id/rubrics", instructor, r.RubricHandler.CreateCourseRubric)
	protected.Get("/courses/:course_id/rubrics/:rubric_id", enrolled, r.RubricHandler.GetRubric)
	protected.Put("/courses/:course_id/rubrics/:rubric_id", instructor, r.RubricHandler.UpdateRubric)
	protected.Delete("/courses/:course_id/rubrics/:rubric_id", instructor, r.RubricHandler.DeleteRubric)
	protected.Post("/courses/:course_id/rubrics/:rubric_id/associations", instructor, r.RubricHandler.AssociateRubric)

	// Rubric Assessments (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/rubric_associations/:association_id/rubric_assessments", enrolled, r.RubricAssessmentHandler.ListAssessments)
	protected.Post("/courses/:course_id/rubric_associations/:association_id/rubric_assessments", instructor, r.RubricAssessmentHandler.CreateAssessment)
	protected.Get("/courses/:course_id/rubric_associations/:association_id/rubric_assessments/:assessment_id", enrolled, r.RubricAssessmentHandler.GetAssessment)
	protected.Put("/courses/:course_id/rubric_associations/:association_id/rubric_assessments/:assessment_id", instructor, r.RubricAssessmentHandler.UpdateAssessment)

	// Grading Periods (admin only)
	protected.Get("/accounts/:account_id/grading_period_groups", admin, r.GradingPeriodHandler.ListGroups)
	protected.Post("/accounts/:account_id/grading_period_groups", admin, r.GradingPeriodHandler.CreateGroup)
	protected.Get("/accounts/:account_id/grading_period_groups/:group_id", admin, r.GradingPeriodHandler.GetGroup)
	protected.Put("/accounts/:account_id/grading_period_groups/:group_id", admin, r.GradingPeriodHandler.UpdateGroup)
	protected.Delete("/accounts/:account_id/grading_period_groups/:group_id", admin, r.GradingPeriodHandler.DeleteGroup)
	protected.Get("/accounts/:account_id/grading_period_groups/:group_id/grading_periods", admin, r.GradingPeriodHandler.ListPeriods)
	protected.Post("/accounts/:account_id/grading_period_groups/:group_id/grading_periods", admin, r.GradingPeriodHandler.CreatePeriod)
	protected.Get("/accounts/:account_id/grading_period_groups/:group_id/grading_periods/:period_id", admin, r.GradingPeriodHandler.GetPeriod)
	protected.Put("/accounts/:account_id/grading_period_groups/:group_id/grading_periods/:period_id", admin, r.GradingPeriodHandler.UpdatePeriod)
	protected.Delete("/accounts/:account_id/grading_period_groups/:group_id/grading_periods/:period_id", admin, r.GradingPeriodHandler.DeletePeriod)

	// Assignment Rubric (view: enrolled)
	protected.Get("/courses/:course_id/assignments/:assignment_id/rubric", enrolled, r.RubricHandler.GetAssignmentRubric)

	// Assignment Overrides (instructor only)
	protected.Get("/courses/:course_id/assignments/:assignment_id/overrides", instructor, r.AssignmentOverrideHandler.ListOverrides)
	protected.Post("/courses/:course_id/assignments/:assignment_id/overrides", instructor, r.AssignmentOverrideHandler.CreateOverride)
	protected.Get("/courses/:course_id/assignments/:assignment_id/overrides/:override_id", instructor, r.AssignmentOverrideHandler.GetOverride)
	protected.Put("/courses/:course_id/assignments/:assignment_id/overrides/:override_id", instructor, r.AssignmentOverrideHandler.UpdateOverride)
	protected.Delete("/courses/:course_id/assignments/:assignment_id/overrides/:override_id", instructor, r.AssignmentOverrideHandler.DeleteOverride)

	// Late Policy (instructor only)
	protected.Get("/courses/:course_id/late_policy", instructor, r.LatePolicyHandler.GetLatePolicy)
	protected.Post("/courses/:course_id/late_policy", instructor, r.LatePolicyHandler.CreateLatePolicy)
	protected.Put("/courses/:course_id/late_policy", instructor, r.LatePolicyHandler.UpdateLatePolicy)
	protected.Delete("/courses/:course_id/late_policy", instructor, r.LatePolicyHandler.DeleteLatePolicy)

	// Calendar Events (any authenticated user)
	protected.Get("/calendar_events", r.CalendarEventHandler.ListEvents)
	protected.Get("/calendar_events.ics", r.CalendarEventHandler.ExportAsICal)
	protected.Post("/calendar_events", r.CalendarEventHandler.CreateEvent)
	protected.Get("/calendar_events/:id", r.CalendarEventHandler.GetEvent)
	protected.Put("/calendar_events/:id", r.CalendarEventHandler.UpdateEvent)
	protected.Delete("/calendar_events/:id", r.CalendarEventHandler.DeleteEvent)
	protected.Get("/courses/:course_id/calendar_events", enrolled, r.CalendarEventHandler.ListEvents)

	// Conversations (any authenticated user)
	protected.Get("/conversations", r.ConversationHandler.ListConversations)
	protected.Post("/conversations", r.ConversationHandler.CreateConversation)
	protected.Get("/conversations/:id", r.ConversationHandler.GetConversation)
	protected.Put("/conversations/:id", r.ConversationHandler.UpdateConversation)
	protected.Get("/conversations/:id/messages", r.ConversationHandler.ListMessages)
	protected.Post("/conversations/:id/messages", r.ConversationHandler.CreateMessage)
	protected.Put("/conversations/:id/mark_as_read", r.ConversationHandler.MarkAsRead)

	// Notifications (any authenticated user)
	protected.Get("/notifications", r.NotificationHandler.ListNotifications)
	protected.Put("/notifications/mark_all_as_read", r.NotificationHandler.MarkAllAsRead)
	protected.Put("/notifications/:id/mark_as_read", r.NotificationHandler.MarkAsRead)
	protected.Get("/users/self/notification_preferences", r.NotificationHandler.GetPreferences)
	protected.Put("/users/self/notification_preferences", r.NotificationHandler.UpdatePreferences)

	// Content Migrations (instructor only)
	protected.Get("/courses/:course_id/content_migrations", instructor, r.ContentMigrationHandler.ListMigrations)
	protected.Post("/courses/:course_id/content_migrations", middleware.ExpensiveOpRateLimit(), instructor, r.ContentMigrationHandler.CreateMigration)
	protected.Get("/courses/:course_id/content_migrations/:id", instructor, r.ContentMigrationHandler.GetMigration)
	protected.Put("/courses/:course_id/content_migrations/:id", instructor, r.ContentMigrationHandler.UpdateMigration)

	// Learning Outcomes (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/outcome_groups", enrolled, r.LearningOutcomeHandler.ListGroups)
	protected.Post("/courses/:course_id/outcome_groups", instructor, r.LearningOutcomeHandler.CreateGroup)
	protected.Get("/courses/:course_id/outcome_groups/:group_id", enrolled, r.LearningOutcomeHandler.GetGroup)
	protected.Put("/courses/:course_id/outcome_groups/:group_id", instructor, r.LearningOutcomeHandler.UpdateGroup)
	protected.Delete("/courses/:course_id/outcome_groups/:group_id", instructor, r.LearningOutcomeHandler.DeleteGroup)
	protected.Get("/courses/:course_id/outcome_groups/:group_id/outcomes", enrolled, r.LearningOutcomeHandler.ListOutcomes)
	protected.Post("/courses/:course_id/outcome_groups/:group_id/outcomes", instructor, r.LearningOutcomeHandler.CreateOutcome)
	protected.Get("/courses/:course_id/outcomes/:outcome_id", enrolled, r.LearningOutcomeHandler.GetOutcome)
	protected.Put("/courses/:course_id/outcomes/:outcome_id", instructor, r.LearningOutcomeHandler.UpdateOutcome)
	protected.Delete("/courses/:course_id/outcomes/:outcome_id", instructor, r.LearningOutcomeHandler.DeleteOutcome)
	protected.Get("/courses/:course_id/outcome_results", enrolled, r.LearningOutcomeHandler.ListResults)
	protected.Post("/courses/:course_id/outcome_results", instructor, r.LearningOutcomeHandler.CreateResult)
	protected.Get("/courses/:course_id/outcome_rollups", enrolled, r.LearningOutcomeHandler.GetMasteryGradebook)
	protected.Get("/courses/:course_id/outcome_alignments", enrolled, r.LearningOutcomeHandler.ListAlignments)
	protected.Post("/courses/:course_id/outcome_alignments", instructor, r.LearningOutcomeHandler.CreateAlignment)
	protected.Delete("/courses/:course_id/outcome_alignments/:alignment_id", instructor, r.LearningOutcomeHandler.DeleteAlignment)

	// SpeedGrader (instructor only)
	protected.Get("/courses/:course_id/assignments/:assignment_id/speedgrader", instructor, r.SpeedGraderHandler.GetSpeedGraderData)
	protected.Get("/courses/:course_id/assignments/:assignment_id/speedgrader/submissions/:user_id", instructor, r.SpeedGraderHandler.GetStudentSubmission)

	// Grade posting (instructor only)
	protected.Post("/courses/:course_id/assignments/:id/post_grades", instructor, r.SubmissionHandler.PostGrades)
	protected.Post("/courses/:course_id/assignments/:id/hide_grades", instructor, r.SubmissionHandler.HideGrades)

	// Groups (view: enrolled; manage categories: instructor; join: enrolled)
	protected.Get("/courses/:course_id/group_categories", enrolled, r.GroupHandler.ListGroupCategories)
	protected.Post("/courses/:course_id/group_categories", instructor, r.GroupHandler.CreateGroupCategory)
	protected.Get("/group_categories/:id", r.GroupHandler.GetGroupCategory)
	protected.Put("/group_categories/:id", r.GroupHandler.UpdateGroupCategory)
	protected.Delete("/group_categories/:id", r.GroupHandler.DeleteGroupCategory)
	protected.Get("/group_categories/:group_category_id/groups", r.GroupHandler.ListGroupsByCategory)
	protected.Post("/group_categories/:group_category_id/groups", r.GroupHandler.CreateGroup)
	protected.Get("/groups/:id", r.GroupHandler.GetGroup)
	protected.Put("/groups/:id", r.GroupHandler.UpdateGroup)
	protected.Delete("/groups/:id", r.GroupHandler.DeleteGroup)
	protected.Get("/groups/:group_id/memberships", r.GroupHandler.ListGroupMemberships)
	protected.Post("/groups/:group_id/memberships", r.GroupHandler.CreateGroupMembership)
	protected.Put("/groups/:group_id/memberships/:membership_id", r.GroupHandler.UpdateGroupMembership)
	protected.Delete("/groups/:group_id/memberships/:membership_id", r.GroupHandler.DeleteGroupMembership)
	protected.Get("/users/self/groups", r.GroupHandler.ListUserGroups)

	// Blueprint Courses (instructor only)
	protected.Get("/courses/:course_id/blueprint_templates", instructor, r.BlueprintHandler.ListTemplates)
	protected.Post("/courses/:course_id/blueprint_templates", instructor, r.BlueprintHandler.CreateTemplate)
	protected.Get("/courses/:course_id/blueprint_templates/default", instructor, r.BlueprintHandler.GetDefaultTemplate)
	protected.Put("/courses/:course_id/blueprint_templates/default", instructor, r.BlueprintHandler.UpdateDefaultTemplate)
	protected.Get("/courses/:course_id/blueprint_templates/default/associated_courses", instructor, r.BlueprintHandler.GetAssociatedCourses)
	protected.Put("/courses/:course_id/blueprint_templates/default/associated_courses", instructor, r.BlueprintHandler.UpdateAssociations)
	protected.Get("/courses/:course_id/blueprint_templates/default/migrations", instructor, r.BlueprintHandler.ListMigrations)
	protected.Post("/courses/:course_id/blueprint_templates/default/migrations", instructor, r.BlueprintHandler.CreateMigration)
	protected.Get("/courses/:course_id/blueprint_templates/default/migrations/:migration_id", instructor, r.BlueprintHandler.GetMigration)
	protected.Get("/courses/:course_id/blueprint_templates/default/unsynced_changes", instructor, r.BlueprintHandler.GetUnsyncedChanges)
	protected.Get("/courses/:course_id/blueprint_subscriptions", enrolled, r.BlueprintHandler.ListSubscriptions)
	protected.Get("/courses/:course_id/blueprint_subscriptions/:subscription_id/migrations", enrolled, r.BlueprintHandler.GetSubscriptionMigrations)
	protected.Get("/courses/:course_id/blueprint_subscriptions/:subscription_id/migrations/:migration_id", enrolled, r.BlueprintHandler.GetSubscriptionMigration)

	// Course Pacing (instructor only)
	protected.Get("/courses/:course_id/course_pacing", instructor, r.CoursePaceHandler.ListCoursePaces)
	protected.Post("/courses/:course_id/course_pacing", instructor, r.CoursePaceHandler.CreateCoursePace)
	protected.Get("/courses/:course_id/course_pacing/:id", instructor, r.CoursePaceHandler.GetCoursePace)
	protected.Put("/courses/:course_id/course_pacing/:id", instructor, r.CoursePaceHandler.UpdateCoursePace)
	protected.Delete("/courses/:course_id/course_pacing/:id", instructor, r.CoursePaceHandler.DeleteCoursePace)
	protected.Post("/courses/:course_id/course_pacing/:id/publish", instructor, r.CoursePaceHandler.PublishCoursePace)
	protected.Get("/courses/:course_id/course_pacing/:id/module_items", instructor, r.CoursePaceHandler.GetPaceModuleItems)
	protected.Put("/courses/:course_id/course_pacing/:id/module_items", instructor, r.CoursePaceHandler.UpdatePaceModuleItems)

	// Collaborations (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/collaborations", enrolled, r.CollaborationHandler.ListCollaborations)
	protected.Post("/courses/:course_id/collaborations", instructor, r.CollaborationHandler.CreateCollaboration)
	protected.Get("/collaborations/:id", r.CollaborationHandler.GetCollaboration)
	protected.Put("/collaborations/:id", r.CollaborationHandler.UpdateCollaboration)
	protected.Delete("/collaborations/:id", r.CollaborationHandler.DeleteCollaboration)

	// Conferences (view: enrolled; manage: instructor; join: enrolled)
	protected.Get("/courses/:course_id/conferences", enrolled, r.ConferenceHandler.ListConferences)
	protected.Post("/courses/:course_id/conferences", instructor, r.ConferenceHandler.CreateConference)
	protected.Get("/conferences/:id", r.ConferenceHandler.GetConference)
	protected.Put("/conferences/:id", r.ConferenceHandler.UpdateConference)
	protected.Delete("/conferences/:id", r.ConferenceHandler.DeleteConference)
	protected.Post("/conferences/:id/join", r.ConferenceHandler.JoinConference)
	protected.Post("/conferences/:id/end", r.ConferenceHandler.EndConference)
	protected.Get("/conferences/:id/recordings", r.ConferenceHandler.GetRecordings)
	protected.Get("/conferences/:id/participants", r.ConferenceHandler.GetParticipants)

	// Analytics (course: instructor; department: admin)
	protected.Get("/courses/:course_id/analytics/activity", instructor, r.AnalyticsHandler.GetCourseActivity)
	protected.Get("/courses/:course_id/analytics/assignments", instructor, r.AnalyticsHandler.GetCourseAssignmentStats)
	protected.Get("/courses/:course_id/analytics/student_summaries", instructor, r.AnalyticsHandler.GetStudentSummaries)
	protected.Get("/courses/:course_id/analytics/users/:user_id/activity", instructor, r.AnalyticsHandler.GetStudentActivity)
	protected.Get("/courses/:course_id/analytics/users/:user_id/assignments", instructor, r.AnalyticsHandler.GetStudentAssignments)
	protected.Get("/accounts/:account_id/analytics/current/activity", admin, r.AnalyticsHandler.GetDepartmentActivity)
	protected.Get("/accounts/:account_id/analytics/current/grades", admin, r.AnalyticsHandler.GetDepartmentGrades)
	protected.Get("/accounts/:account_id/analytics/current/statistics", admin, r.AnalyticsHandler.GetDepartmentStatistics)
	protected.Post("/page_views", r.AnalyticsHandler.CreatePageView)
	protected.Get("/users/self/page_views", r.AnalyticsHandler.ListUserPageViews)

	// Observer/Parent Role (self or admin)
	protected.Post("/users/:user_id/observees", selfOrAdmin, r.ObserverHandler.LinkObservee)
	protected.Delete("/users/:user_id/observees/:observee_id", selfOrAdmin, r.ObserverHandler.UnlinkObservee)
	protected.Get("/users/:user_id/observees", selfOrAdmin, r.ObserverHandler.ListObservees)
	protected.Get("/users/:user_id/observees/:observee_id/courses", selfOrAdmin, r.ObserverHandler.GetObserveeCourses)
	protected.Get("/users/:user_id/observees/:child_id/overview", selfOrAdmin, r.ObserverHandler.GetChildOverview)

	// Parent/observer pairing codes (every authenticated user can manage their own).
	protected.Post("/users/self/pairing_codes", r.PairingCodeHandler.Generate)
	protected.Post("/users/self/pairing_codes/redeem", r.PairingCodeHandler.Redeem)
	protected.Get("/users/self/pairing_codes", r.PairingCodeHandler.List)
	protected.Delete("/users/self/pairing_codes/:id", r.PairingCodeHandler.Revoke)
	// Teacher-mediated pairing-code mint (item 12.6). Authorization is
	// inside the handler — a teacher in the student's course OR the
	// student themselves in an adult-mode tenant.
	protected.Post("/users/:student_id/observer-pairing-codes", r.PairingCodeHandler.MintForStudent)

	// GraphQL (any authenticated user)
	protected.Post("/graphql", r.GraphQLHandler.HandleQuery)

	// Authentication Providers (admin only)
	protected.Get("/accounts/:account_id/authentication_providers", admin, r.AuthProviderHandler.ListProviders)
	protected.Post("/accounts/:account_id/authentication_providers", admin, r.AuthProviderHandler.CreateProvider)
	protected.Get("/accounts/:account_id/authentication_providers/:id", admin, r.AuthProviderHandler.GetProvider)
	protected.Put("/accounts/:account_id/authentication_providers/:id", admin, r.AuthProviderHandler.UpdateProvider)
	protected.Delete("/accounts/:account_id/authentication_providers/:id", admin, r.AuthProviderHandler.DeleteProvider)
	protected.Post("/accounts/:account_id/authentication_providers/:id/test", admin, r.AuthProviderHandler.TestConnection)

	// Discussion V2 (enhanced with read/unread, user profiles, edit history)
	protected.Get("/courses/:course_id/discussion_topics/:topic_id/view_v2", enrolled, r.DiscussionV2Handler.GetFullViewV2)
	protected.Post("/courses/:course_id/discussion_topics/:topic_id/entries/:entry_id/read", enrolled, r.DiscussionV2Handler.MarkEntryRead)
	protected.Post("/courses/:course_id/discussion_topics/:topic_id/mark_all_read", enrolled, r.DiscussionV2Handler.MarkTopicRead)
	protected.Get("/courses/:course_id/discussion_topics/:topic_id/unread_count", enrolled, r.DiscussionV2Handler.GetUnreadCount)
	protected.Put("/courses/:course_id/discussion_topics/:topic_id/subscription", enrolled, r.DiscussionV2Handler.ToggleSubscription)
	protected.Get("/courses/:course_id/discussion_topics/:topic_id/entries/:entry_id/versions", enrolled, r.DiscussionV2Handler.GetEntryVersions)
	protected.Put("/courses/:course_id/discussion_topics/:topic_id/entries/:entry_id/v2", enrolled, r.DiscussionV2Handler.UpdateEntryV2)

	// Content Import (IMSCC/Common Cartridge)
	protected.Post("/courses/:course_id/content_imports", middleware.ExpensiveOpRateLimit(), middleware.EnforceUploadSize(r.SettingsLookup), instructor, r.ContentImportHandler.ImportPackage)

	// Batch Operations (instructor/admin, rate-limited)
	protected.Post("/courses/clone", middleware.ExpensiveOpRateLimit(), admin, r.BatchHandler.CloneCourse)
	protected.Post("/courses/:course_id/date_shift", middleware.ExpensiveOpRateLimit(), instructor, r.BatchHandler.BulkDateShift)
	protected.Post("/conversations/bulk", middleware.ExpensiveOpRateLimit(), r.BatchHandler.BulkSendMessage)
	protected.Post("/courses/:course_id/enrollments/bulk", middleware.ExpensiveOpRateLimit(), instructor, r.BatchHandler.BulkEnrollUsers)
	protected.Post("/courses/:course_id/assignments/bulk_update_dates", instructor, r.BatchHandler.BulkUpdateAssignmentDates)

	// Announcements (view: enrolled; manage: instructor; global: admin)
	protected.Get("/courses/:course_id/announcements", enrolled, r.AnnouncementHandler.ListCourseAnnouncements)
	protected.Post("/courses/:course_id/announcements", instructor, r.AnnouncementHandler.CreateCourseAnnouncement)
	protected.Get("/announcements/:id", r.AnnouncementHandler.GetAnnouncement)
	protected.Put("/announcements/:id", r.AnnouncementHandler.UpdateAnnouncement)
	protected.Delete("/announcements/:id", r.AnnouncementHandler.DeleteAnnouncement)
	protected.Post("/announcements/:id/read", r.AnnouncementHandler.MarkAsRead)
	protected.Post("/announcements/:id/acknowledge", r.AnnouncementHandler.AcknowledgeAnnouncement)
	protected.Get("/announcements/:id/read_receipts", instructor, r.AnnouncementHandler.GetReadReceipts)
	protected.Get("/accounts/:account_id/announcements", r.AnnouncementHandler.ListAccountAnnouncements)
	protected.Post("/accounts/:account_id/announcements", admin, r.AnnouncementHandler.CreateAccountAnnouncement)

	// Enrollment Terms (admin only)
	protected.Get("/accounts/:account_id/terms", admin, r.EnrollmentTermHandler.ListTerms)
	protected.Post("/accounts/:account_id/terms", admin, r.EnrollmentTermHandler.CreateTerm)
	protected.Get("/accounts/:account_id/terms/current", admin, r.EnrollmentTermHandler.GetCurrentTerm)
	protected.Get("/accounts/:account_id/terms/:id", admin, r.EnrollmentTermHandler.GetTerm)
	protected.Put("/accounts/:account_id/terms/:id", admin, r.EnrollmentTermHandler.UpdateTerm)
	protected.Delete("/accounts/:account_id/terms/:id", admin, r.EnrollmentTermHandler.DeleteTerm)

	// Syllabus (enrolled)
	protected.Get("/courses/:course_id/syllabus", enrolled, r.SyllabusHandler.GetSyllabus)

	// Notification Delivery (self or admin)
	protected.Get("/users/self/notification_deliveries", r.NotificationDeliveryHandler.ListDeliveries)
	protected.Get("/admin/notification_stats", admin, r.NotificationDeliveryHandler.GetDeliveryStats)
	protected.Post("/admin/notification_deliveries/retry", admin, r.NotificationDeliveryHandler.RetryFailedDeliveries)
	protected.Get("/users/self/communication_channels", r.NotificationDeliveryHandler.ListChannels)
	protected.Post("/users/self/communication_channels", r.NotificationDeliveryHandler.CreateChannel)
	protected.Delete("/users/self/communication_channels/:id", r.NotificationDeliveryHandler.DeleteChannel)

	// Audit Logs (course: instructor; account: admin)
	protected.Get("/courses/:course_id/audit_log", instructor, r.AuditHandler.GetCourseAuditLog)
	protected.Get("/courses/:course_id/grade_change_log", instructor, r.AuditHandler.GetCourseGradeChangeLog)
	protected.Get("/courses/:course_id/audit_log.csv", instructor, r.AuditHandler.ExportCourseAuditLogCSV)
	protected.Get("/courses/:course_id/grade_change_log.csv", instructor, r.AuditHandler.ExportCourseGradeChangeLogCSV)
	protected.Get("/accounts/:account_id/audit_log", admin, r.AuditHandler.GetAccountAuditLog)
	protected.Get("/admin/audit_log/summary", admin, r.AuditHandler.GetAuditLogSummary)

	// Custom Roles (admin only, except course permissions)
	protected.Get("/accounts/:account_id/roles", admin, r.CustomRoleHandler.ListRoles)
	protected.Post("/accounts/:account_id/roles", admin, r.CustomRoleHandler.CreateRole)
	protected.Get("/accounts/:account_id/roles/presets", admin, r.CustomRoleHandler.GetPresets)
	protected.Get("/accounts/:account_id/roles/:id", admin, r.CustomRoleHandler.GetRole)
	protected.Put("/accounts/:account_id/roles/:id", admin, r.CustomRoleHandler.UpdateRole)
	protected.Delete("/accounts/:account_id/roles/:id", admin, r.CustomRoleHandler.DeleteRole)
	protected.Post("/accounts/:account_id/roles/:id/clone", admin, r.CustomRoleHandler.CloneRole)
	protected.Get("/accounts/:account_id/roles/:id/overrides", admin, r.CustomRoleHandler.ListOverrides)
	protected.Put("/accounts/:account_id/roles/:id/overrides", admin, r.CustomRoleHandler.BulkSetOverrides)
	protected.Get("/courses/:course_id/permissions", enrolled, r.CustomRoleHandler.GetCoursePermissions)

	// OneRoster (admin only)
	protected.Get("/accounts/:account_id/oneroster_connections", admin, r.OneRosterHandler.ListConnections)
	protected.Post("/accounts/:account_id/oneroster_connections", admin, r.OneRosterHandler.CreateConnection)
	protected.Get("/accounts/:account_id/oneroster_connections/:id", admin, r.OneRosterHandler.GetConnection)
	protected.Put("/accounts/:account_id/oneroster_connections/:id", admin, r.OneRosterHandler.UpdateConnection)
	protected.Delete("/accounts/:account_id/oneroster_connections/:id", admin, r.OneRosterHandler.DeleteConnection)
	protected.Post("/accounts/:account_id/oneroster_connections/:id/test", admin, r.OneRosterHandler.TestConnection)
	protected.Post("/accounts/:account_id/oneroster_connections/:id/sync", admin, r.OneRosterHandler.SyncFull)
	protected.Post("/accounts/:account_id/oneroster_connections/:id/sync_incremental", admin, r.OneRosterHandler.SyncIncremental)
	protected.Get("/accounts/:account_id/oneroster_connections/:id/sync_logs", admin, r.OneRosterHandler.GetSyncLogs)

	// Document Annotations (enrolled)
	protected.Get("/courses/:course_id/assignments/:assignment_id/submissions/:user_id/annotations", enrolled, r.DocumentAnnotationHandler.ListAnnotations)
	protected.Post("/courses/:course_id/assignments/:assignment_id/submissions/:user_id/annotations", enrolled, r.DocumentAnnotationHandler.CreateAnnotation)
	protected.Get("/courses/:course_id/assignments/:assignment_id/submissions/:user_id/annotation_summary", enrolled, r.DocumentAnnotationHandler.GetAnnotationSummary)
	protected.Get("/annotations/:id", r.DocumentAnnotationHandler.GetAnnotation)
	protected.Put("/annotations/:id", r.DocumentAnnotationHandler.UpdateAnnotation)
	protected.Delete("/annotations/:id", r.DocumentAnnotationHandler.DeleteAnnotation)
	protected.Post("/annotations/:id/resolve", r.DocumentAnnotationHandler.ResolveAnnotation)
	protected.Delete("/annotations/:id/resolve", r.DocumentAnnotationHandler.UnresolveAnnotation)
	protected.Post("/annotations/:id/replies", r.DocumentAnnotationHandler.ReplyToAnnotation)

	// COPPA / Parental Consent (admin + public verify)
	protected.Post("/consent/request", admin, r.COPPAHandler.RequestConsent)
	protected.Get("/consent", admin, r.COPPAHandler.ListConsents)
	protected.Post("/consent/verify/:token", r.COPPAHandler.VerifyConsent)
	protected.Delete("/consent/:id", admin, r.COPPAHandler.RevokeConsent)
	protected.Get("/data_processing_agreements", admin, r.COPPAHandler.ListDPAs)
	protected.Post("/data_processing_agreements", admin, r.COPPAHandler.CreateDPA)
	protected.Put("/data_processing_agreements/:id", admin, r.COPPAHandler.UpdateDPA)

	// FERPA Compliance (self/admin)
	protected.Post("/users/:user_id/data_export", selfOrAdmin, r.FERPAHandler.CreateExportRequest)
	protected.Get("/users/:user_id/data_export/:id", selfOrAdmin, r.FERPAHandler.GetExportRequest)
	// Item 12.8 — wires the route that FERPAService.ProcessExport
	// already advertises as the download URL. Authorization is inside
	// the handler (requestor / subject / admin).
	protected.Get("/data_exports/:id/download", r.FERPAHandler.DownloadDataExport)
	protected.Post("/users/:user_id/data_deletion", selfOrAdmin, r.FERPAHandler.CreateDeletionRequest)
	protected.Get("/admin/data_deletion_requests", admin, r.FERPAHandler.ListPendingDeletionRequests)
	protected.Post("/admin/data_deletion_requests/:id/approve", admin, r.FERPAHandler.ApproveDeletionRequest)
	protected.Get("/users/:user_id/pii_access_log", admin, r.FERPAHandler.GetPIIAccessLog)
	protected.Get("/admin/retention_policies", admin, r.FERPAHandler.ListRetentionPolicies)
	protected.Post("/admin/retention_policies", admin, r.FERPAHandler.CreateRetentionPolicy)
	protected.Get("/admin/retention_policies/:id", admin, r.FERPAHandler.GetRetentionPolicy)
	protected.Put("/admin/retention_policies/:id", admin, r.FERPAHandler.UpdateRetentionPolicy)
	protected.Delete("/admin/retention_policies/:id", admin, r.FERPAHandler.DeleteRetentionPolicy)

	// Student Accommodations (instructor/admin)
	protected.Get("/users/:user_id/accommodations", selfOrAdmin, r.AccommodationHandler.ListUserAccommodations)
	protected.Post("/users/:user_id/accommodations", admin, r.AccommodationHandler.CreateAccommodation)
	protected.Get("/accommodations/:id", r.AccommodationHandler.GetAccommodation)
	protected.Put("/accommodations/:id", admin, r.AccommodationHandler.UpdateAccommodation)
	protected.Delete("/accommodations/:id", admin, r.AccommodationHandler.DeleteAccommodation)
	protected.Get("/courses/:course_id/accommodations", instructor, r.AccommodationHandler.ListCourseAccommodations)
	protected.Post("/courses/:course_id/assignments/:assignment_id/apply_accommodations", instructor, r.AccommodationHandler.ApplyAccommodationsToAssignment)

	// Attendance (view: enrolled; manage: instructor)
	protected.Post("/courses/:course_id/attendance", instructor, r.AttendanceHandler.RecordAttendance)
	protected.Get("/courses/:course_id/attendance", enrolled, r.AttendanceHandler.GetClassAttendance)
	protected.Get("/courses/:course_id/attendance/users/:user_id", enrolled, r.AttendanceHandler.GetStudentAttendance)
	protected.Get("/courses/:course_id/attendance/users/:user_id/summary", enrolled, r.AttendanceHandler.GetStudentAttendanceSummary)
	protected.Get("/courses/:course_id/attendance/export.csv", instructor, r.AttendanceHandler.ExportAttendanceCSV)

	// Portfolios (self + public)
	protected.Get("/users/self/portfolios", r.PortfolioHandler.ListUserPortfolios)
	protected.Post("/users/self/portfolios", r.PortfolioHandler.CreatePortfolio)
	protected.Get("/portfolios/:id", r.PortfolioHandler.GetPortfolio)
	protected.Put("/portfolios/:id", r.PortfolioHandler.UpdatePortfolio)
	protected.Delete("/portfolios/:id", r.PortfolioHandler.DeletePortfolio)
	protected.Post("/portfolios/:id/publish", r.PortfolioHandler.PublishPortfolio)
	protected.Post("/portfolios/:id/sections", r.PortfolioHandler.AddSection)
	protected.Put("/portfolios/:id/sections/:section_id", r.PortfolioHandler.UpdateSection)
	protected.Delete("/portfolios/:id/sections/:section_id", r.PortfolioHandler.DeleteSection)
	protected.Put("/portfolios/:id/sections/reorder", r.PortfolioHandler.ReorderSections)
	protected.Post("/portfolios/:id/artifacts", r.PortfolioHandler.AddArtifact)
	protected.Put("/portfolios/:id/artifacts/:artifact_id", r.PortfolioHandler.UpdateArtifact)
	protected.Delete("/portfolios/:id/artifacts/:artifact_id", r.PortfolioHandler.DeleteArtifact)
	protected.Post("/portfolios/:id/artifacts/:artifact_id/reflections", r.PortfolioHandler.AddReflection)
	protected.Post("/portfolios/:id/import", r.PortfolioHandler.ImportFromCourse)
	protected.Get("/portfolios/:id/export/html", r.PortfolioHandler.ExportAsHTML)
	protected.Get("/portfolios/:id/export/pdf", r.PortfolioHandler.ExportAsPDF)
	protected.Get("/portfolios/:id/comments", r.PortfolioHandler.ListComments)
	protected.Post("/portfolios/:id/comments", r.PortfolioHandler.AddComment)
	protected.Get("/portfolio_templates", r.PortfolioHandler.ListTemplates)
	protected.Post("/portfolio_templates/:template_id/create", r.PortfolioHandler.CreateFromTemplate)

	// Peer Reviews (assign/list: instructor; view own: enrolled; submit: enrolled)
	protected.Post("/courses/:course_id/assignments/:id/peer_reviews", instructor, r.PeerReviewHandler.AssignPeerReviews)
	protected.Get("/courses/:course_id/assignments/:id/peer_reviews", instructor, r.PeerReviewHandler.ListPeerReviews)
	protected.Get("/courses/:course_id/assignments/:id/peer_reviews/mine", enrolled, r.PeerReviewHandler.ListMyPeerReviews)
	protected.Put("/peer_reviews/:review_id", r.PeerReviewHandler.SubmitPeerReview)

	// Question Banks (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/question_banks", enrolled, r.QuestionBankHandler.ListBanks)
	protected.Post("/courses/:course_id/question_banks", instructor, r.QuestionBankHandler.CreateBank)
	protected.Get("/courses/:course_id/question_banks/:bank_id", enrolled, r.QuestionBankHandler.GetBank)
	protected.Put("/courses/:course_id/question_banks/:bank_id", instructor, r.QuestionBankHandler.UpdateBank)
	protected.Delete("/courses/:course_id/question_banks/:bank_id", instructor, r.QuestionBankHandler.DeleteBank)
	protected.Get("/courses/:course_id/question_banks/:bank_id/questions", enrolled, r.QuestionBankHandler.ListQuestions)
	protected.Post("/courses/:course_id/question_banks/:bank_id/questions", instructor, r.QuestionBankHandler.AddQuestion)
	protected.Put("/courses/:course_id/question_banks/:bank_id/questions/:question_id", instructor, r.QuestionBankHandler.UpdateQuestion)
	protected.Delete("/courses/:course_id/question_banks/:bank_id/questions/:question_id", instructor, r.QuestionBankHandler.DeleteQuestion)
	protected.Post("/courses/:course_id/question_banks/:bank_id/pull_to_quiz", instructor, r.QuestionBankHandler.PullToQuiz)

	// Module Prerequisites (view: enrolled; manage: instructor)
	protected.Get("/courses/:course_id/modules/:id/prerequisites", enrolled, r.ModuleHandler.GetPrerequisites)
	protected.Put("/courses/:course_id/modules/:id/prerequisites", instructor, r.ModuleHandler.SetPrerequisites)

	// Public portfolio view (no auth required)
	api.Get("/portfolios/public/:slug", r.PortfolioHandler.GetPublicPortfolio)

	r.registerP3FeatureRoutes(protected, admin, enrolled, instructor)
	r.registerQuizExtensionRoutes(protected, enrolled, instructor)
	r.registerGamificationRoutes(protected, admin, instructor, selfOrAdmin)
	r.registerSuperAdminRoutes(protected)
}
