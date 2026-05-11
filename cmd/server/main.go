package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	fiberlogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"

	v1 "github.com/EduThemes/paper-lms/internal/api/v1"
	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/auth"
	"github.com/EduThemes/paper-lms/internal/config"
	"github.com/EduThemes/paper-lms/internal/db"
	"github.com/EduThemes/paper-lms/internal/graphql"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/scheduler"
	"github.com/EduThemes/paper-lms/internal/service"
	storageLib "github.com/EduThemes/paper-lms/internal/storage"
)

// Version is set at build time via -ldflags
var Version = "dev"

func main() {
	_ = godotenv.Load()

	cfg := config.Load()
	cfg.Validate()

	// Configure structured logging
	logLevel := slog.LevelInfo
	if cfg.Environment == "development" {
		logLevel = slog.LevelDebug
	}
	var logHandler slog.Handler
	if cfg.Environment == "production" {
		logHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	} else {
		logHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	}
	slog.SetDefault(slog.New(logHandler))

	// Connect to PostgreSQL
	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Database schema management
	if cfg.AutoMigrate {
		// Development mode: use GORM AutoMigrate for fast iteration
		log.Println("AUTO_MIGRATE=true: using GORM AutoMigrate (development mode)")
		if err := db.AutoMigrate(database); err != nil {
			log.Fatalf("Failed to auto-migrate database: %v", err)
		}
	} else {
		// Production mode: use versioned SQL migrations
		log.Println("AUTO_MIGRATE=false: using versioned SQL migrations")
		if err := db.MigrateUp(database); err != nil {
			log.Fatalf("Failed to run database migrations: %v", err)
		}
	}

	// Seed default data
	if err := db.SeedDefaultAccount(database); err != nil {
		log.Fatalf("Failed to seed database: %v", err)
	}

	// Initialize repositories
	userRepo := postgres.NewUserRepository(database)
	accountRepo := postgres.NewAccountRepository(database)
	courseRepo := postgres.NewCourseRepository(database)
	sectionRepo := postgres.NewSectionRepository(database)
	enrollmentRepo := postgres.NewEnrollmentRepository(database)
	moduleRepo := postgres.NewModuleRepository(database)
	moduleItemRepo := postgres.NewModuleItemRepository(database)
	pageRepo := postgres.NewPageRepository(database)
	assignmentRepo := postgres.NewAssignmentRepository(database)
	assignmentGroupRepo := postgres.NewAssignmentGroupRepository(database)
	submissionRepo := postgres.NewSubmissionRepository(database)
	submissionCommentRepo := postgres.NewSubmissionCommentRepository(database)
	gradingStandardRepo := postgres.NewGradingStandardRepository(database)
	devKeyRepo := postgres.NewDeveloperKeyRepository(database)
	accessTokenRepo := postgres.NewAccessTokenRepository(database)
	externalToolRepo := postgres.NewContextExternalToolRepository(database)
	ltiConfigRepo := postgres.NewLTIToolConfigurationRepository(database)
	lineItemRepo := postgres.NewLTILineItemRepository(database)
	resultRepo := postgres.NewLTIResultRepository(database)
	nonceRepo := postgres.NewNonceRepository(database)
	// Additional repositories
	discussionTopicRepo := postgres.NewDiscussionTopicRepository(database)
	discussionEntryRepo := postgres.NewDiscussionEntryRepository(database)
	discussionRatingRepo := postgres.NewDiscussionEntryRatingRepository(database)
	folderRepo := postgres.NewFolderRepository(database)
	attachmentRepo := postgres.NewAttachmentRepository(database)
	sisBatchRepo := postgres.NewSISBatchRepository(database)
	sisBatchErrorRepo := postgres.NewSISBatchErrorRepository(database)
	// Additional repositories
	quizRepo := postgres.NewQuizRepository(database)
	quizQuestionRepo := postgres.NewQuizQuestionRepository(database)
	quizSubmissionRepo := postgres.NewQuizSubmissionRepository(database)
	quizSubmissionAnswerRepo := postgres.NewQuizSubmissionAnswerRepository(database)
	rubricRepo := postgres.NewRubricRepository(database)
	rubricAssocRepo := postgres.NewRubricAssociationRepository(database)
	rubricAssessRepo := postgres.NewRubricAssessmentRepository(database)
	gradingPeriodGroupRepo := postgres.NewGradingPeriodGroupRepository(database)
	gradingPeriodRepo := postgres.NewGradingPeriodRepository(database)
	assignmentOverrideRepo := postgres.NewAssignmentOverrideRepository(database)
	assignmentOverrideStudentRepo := postgres.NewAssignmentOverrideStudentRepository(database)
	latePolicyRepo := postgres.NewLatePolicyRepository(database)
	// Additional repositories
	calendarEventRepo := postgres.NewCalendarEventRepository(database)
	conversationRepo := postgres.NewConversationRepository(database)
	conversationParticipantRepo := postgres.NewConversationParticipantRepository(database)
	conversationMessageRepo := postgres.NewConversationMessageRepository(database)
	notificationPrefRepo := postgres.NewNotificationPreferenceRepository(database)
	notificationRepo := postgres.NewNotificationRepository(database)
	// Additional repositories
	contentMigrationRepo := postgres.NewContentMigrationRepository(database)
	outcomeGroupRepo := postgres.NewLearningOutcomeGroupRepository(database)
	outcomeRepo := postgres.NewLearningOutcomeRepository(database)
	outcomeResultRepo := postgres.NewLearningOutcomeResultRepository(database)
	outcomeAlignmentRepo := postgres.NewOutcomeAlignmentRepository(database)
	// Additional repositories
	groupCategoryRepo := postgres.NewGroupCategoryRepository(database)
	groupRepo := postgres.NewGroupRepository(database)
	groupMembershipRepo := postgres.NewGroupMembershipRepository(database)
	blueprintTemplateRepo := postgres.NewBlueprintTemplateRepository(database)
	blueprintSubscriptionRepo := postgres.NewBlueprintSubscriptionRepository(database)
	blueprintMigrationRepo := postgres.NewBlueprintMigrationRepository(database)
	coursePaceRepo := postgres.NewCoursePaceRepository(database)
	coursePaceModuleItemRepo := postgres.NewCoursePaceModuleItemRepository(database)
	// Additional repositories
	collaborationRepo := postgres.NewCollaborationRepository(database)
	conferenceRepo := postgres.NewConferenceRepository(database)
	conferenceParticipantRepo := postgres.NewConferenceParticipantRepository(database)
	pageViewRepo := postgres.NewPageViewRepository(database)
	// Additional repositories
	authProviderRepo := postgres.NewAuthenticationProviderRepository(database)
	// Additional repositories
	announcementRepo := postgres.NewAnnouncementRepository(database)
	announcementReceiptRepo := postgres.NewAnnouncementReadReceiptRepository(database)
	enrollmentTermRepo := postgres.NewEnrollmentTermRepository(database)
	// Additional repositories
	discussionEntryParticipantRepo := postgres.NewDiscussionEntryParticipantRepository(database)
	discussionTopicParticipantRepo := postgres.NewDiscussionTopicParticipantRepository(database)
	discussionEntryVersionRepo := postgres.NewDiscussionEntryVersionRepository(database)
	// Additional repositories
	customRoleRepo := postgres.NewCustomRoleRepository(database)
	roleOverrideRepo := postgres.NewRoleOverrideRepository(database)
	onerosterConnRepo := postgres.NewOneRosterConnectionRepository(database)
	onerosterSyncLogRepo := postgres.NewOneRosterSyncLogRepository(database)
	documentAnnotationRepo := postgres.NewDocumentAnnotationRepository(database)
	// Additional repositories
	communicationChannelRepo := postgres.NewCommunicationChannelRepository(database)
	notificationDeliveryRepo := postgres.NewNotificationDeliveryRepository(database)
	auditLogRepo := postgres.NewAuditLogRepository(database)
	gradeChangeLogRepo := postgres.NewGradeChangeLogRepository(database)
	// Additional repositories
	parentalConsentRepo := postgres.NewParentalConsentRepository(database)
	dpaRepo := postgres.NewDataProcessingAgreementRepository(database)
	ageVerificationRepo := postgres.NewAgeVerificationRepository(database)
	retentionPolicyRepo := postgres.NewDataRetentionPolicyRepository(database)
	deletionRequestRepo := postgres.NewDataDeletionRequestRepository(database)
	exportRequestRepo := postgres.NewDataExportRequestRepository(database)
	piiAccessLogRepo := postgres.NewPIIAccessLogRepository(database)
	studentAccommodationRepo := postgres.NewStudentAccommodationRepository(database)
	accommodationApplicationRepo := postgres.NewAccommodationApplicationRepository(database)
	attendanceRepo := postgres.NewAttendanceRepository(database)
	portfolioRepo := postgres.NewPortfolioRepository(database)
	portfolioSectionRepo := postgres.NewPortfolioSectionRepository(database)
	portfolioArtifactRepo := postgres.NewPortfolioArtifactRepository(database)
	portfolioReflectionRepo := postgres.NewPortfolioReflectionRepository(database)
	portfolioTemplateRepo := postgres.NewPortfolioTemplateRepository(database)
	portfolioCommentRepo := postgres.NewPortfolioCommentRepository(database)
	// Course Home Engine repositories
	courseHomeButtonRepo := postgres.NewCourseHomeButtonRepository(database)
	todaysLessonOverrideRepo := postgres.NewTodaysLessonOverrideRepository(database)
	courseVisitRepo := postgres.NewCourseVisitRepository(database)
	// Peer Review, Question Bank, Module Prerequisite repositories
	peerReviewRepo := postgres.NewPeerReviewRepository(database)
	questionBankRepo := postgres.NewQuestionBankRepository(database)
	questionBankEntryRepo := postgres.NewQuestionBankEntryRepository(database)
	modulePrerequisiteRepo := postgres.NewModulePrerequisiteRepository(database)
	// Quiz Question Group repository
	quizQuestionGroupRepo := postgres.NewQuizQuestionGroupRepository(database)
	// P3 Feature repositories
	featureFlagRepo := postgres.NewFeatureFlagRepository(database)
	customGradebookColumnRepo := postgres.NewCustomGradebookColumnRepository(database)
	customColumnDatumRepo := postgres.NewCustomColumnDatumRepository(database)
	masteryPathRepo := postgres.NewMasteryPathRepository(database)
	appointmentGroupRepo := postgres.NewAppointmentGroupRepository(database)
	appointmentSlotRepo := postgres.NewAppointmentSlotRepository(database)
	appointmentReservationRepo := postgres.NewAppointmentReservationRepository(database)
	outcomeProficiencyRepo := postgres.NewOutcomeProficiencyRepository(database)
	// Parent/observer pairing codes
	pairingCodeRepo := postgres.NewPairingCodeRepository(database)
	// Discussion Checkpoints, Smart Search, Commons
	discussionCheckpointRepo := postgres.NewDiscussionCheckpointRepository(database)
	discussionCheckpointSubmissionRepo := postgres.NewDiscussionCheckpointSubmissionRepository(database)
	contentEmbeddingRepo := postgres.NewContentEmbeddingRepository(database)
	sharedContentRepo := postgres.NewSharedContentRepository(database)

	// Initialize services
	userService := service.NewUserService(userRepo)
	courseService := service.NewCourseService(courseRepo, enrollmentRepo, sectionRepo)
	enrollmentService := service.NewEnrollmentService(enrollmentRepo)
	moduleService := service.NewModuleService(moduleRepo, moduleItemRepo, service.WithPrerequisiteRepo(modulePrerequisiteRepo))
	pageService := service.NewPageService(pageRepo)
	assignmentService := service.NewAssignmentService(assignmentRepo)
	assignmentGroupService := service.NewAssignmentGroupService(assignmentGroupRepo, assignmentRepo)
	submissionService := service.NewSubmissionService(submissionRepo, assignmentRepo, enrollmentRepo, latePolicyRepo, courseRepo, gradingPeriodGroupRepo, gradingPeriodRepo, groupMembershipRepo)
	gradingService := service.NewGradingService(submissionRepo, assignmentRepo, assignmentGroupRepo, enrollmentRepo, courseRepo, gradingStandardRepo)
	devKeyService := service.NewDeveloperKeyService(devKeyRepo)
	accessTokenService := service.NewAccessTokenService(accessTokenRepo)
	oauth2Service := service.NewOAuth2Service(devKeyService, accessTokenService)
	externalToolService := service.NewExternalToolService(externalToolRepo, devKeyRepo)

	// Determine platform issuer URL from frontend URL or fallback
	platformIssuer := cfg.FrontendURL
	ltiService, err := service.NewLTIService(devKeyRepo, ltiConfigRepo, nonceRepo, enrollmentRepo, courseRepo, platformIssuer)
	if err != nil {
		log.Fatalf("Failed to initialize LTI service: %v", err)
	}

	agsService := service.NewLTIAGSService(lineItemRepo, resultRepo, submissionRepo, assignmentRepo)
	nrpsService := service.NewLTINRPSService(enrollmentRepo, userRepo)
	// services
	discussionService := service.NewDiscussionService(discussionTopicRepo, discussionEntryRepo, discussionRatingRepo)
	// Initialize file storage backend
	var storageBackend storageLib.Backend
	switch cfg.StorageBackend {
	case "s3":
		s3Cfg := storageLib.S3Config{
			Bucket:    cfg.S3Bucket,
			Region:    cfg.S3Region,
			Endpoint:  cfg.S3Endpoint,
			AccessKey: cfg.S3AccessKey,
			SecretKey: cfg.S3SecretKey,
		}
		s3Backend, err := storageLib.NewS3Backend(context.Background(), s3Cfg)
		if err != nil {
			log.Fatalf("Failed to initialize S3 storage: %v", err)
		}
		storageBackend = s3Backend
		slog.Info("Using S3 storage backend", "bucket", cfg.S3Bucket, "region", cfg.S3Region)
	default:
		storageBackend = storageLib.NewLocalBackend(cfg.FileStoragePath)
		slog.Info("Using local storage backend", "path", cfg.FileStoragePath)
	}
	fileService := service.NewFileServiceWithBackend(folderRepo, attachmentRepo, storageBackend)
	sisImportService := service.NewSISImportService(sisBatchRepo, sisBatchErrorRepo, userRepo, courseRepo, sectionRepo, enrollmentRepo, database)
	// services
	quizService := service.NewQuizService(quizRepo, quizQuestionRepo, quizSubmissionRepo, quizSubmissionAnswerRepo,
		service.WithQuestionGroupRepo(quizQuestionGroupRepo),
		service.WithBankEntryRepo(questionBankEntryRepo),
	)
	rubricService := service.NewRubricService(rubricRepo, rubricAssocRepo, rubricAssessRepo)
	gradingPeriodService := service.NewGradingPeriodService(gradingPeriodGroupRepo, gradingPeriodRepo)
	overrideService := service.NewOverrideService(assignmentOverrideRepo, assignmentOverrideStudentRepo, enrollmentRepo, sectionRepo)
	latePolicyService := service.NewLatePolicyService(latePolicyRepo)
	// services
	calendarService := service.NewCalendarService(calendarEventRepo)
	conversationService := service.NewConversationService(conversationRepo, conversationParticipantRepo, conversationMessageRepo)
	notificationService := service.NewNotificationService(notificationPrefRepo, notificationRepo)
	// services
	contentMigrationService := service.NewContentMigrationService(contentMigrationRepo)
	learningOutcomeService := service.NewLearningOutcomeService(outcomeGroupRepo, outcomeRepo, outcomeResultRepo)
	speedGraderService := service.NewSpeedGraderService(submissionRepo, submissionCommentRepo, assignmentRepo, enrollmentRepo, rubricAssessRepo)
	// services
	groupService := service.NewGroupService(groupCategoryRepo, groupRepo, groupMembershipRepo, enrollmentRepo)
	blueprintService := service.NewBlueprintService(
		blueprintTemplateRepo, blueprintSubscriptionRepo, blueprintMigrationRepo,
		moduleRepo, moduleItemRepo, assignmentRepo, pageRepo,
		quizRepo, quizQuestionRepo, discussionTopicRepo,
	)
	coursePaceService := service.NewCoursePaceService(coursePaceRepo, coursePaceModuleItemRepo, moduleItemRepo, assignmentRepo)
	// services
	collaborationService := service.NewCollaborationService(collaborationRepo)
	conferenceService := service.NewConferenceService(conferenceRepo, conferenceParticipantRepo)
	analyticsService := service.NewAnalyticsService(pageViewRepo, submissionRepo, enrollmentRepo, assignmentRepo)
	observerService := service.NewObserverService(enrollmentRepo, courseRepo, userRepo)
	observerService.SetOverviewDeps(
		assignmentRepo,
		submissionRepo,
		quizRepo,
		announcementRepo,
		pageRepo,
	)
	// services
	authProviderService := service.NewAuthProviderService(authProviderRepo)
	// services
	announcementService := service.NewAnnouncementService(announcementRepo, announcementReceiptRepo, enrollmentRepo)
	enrollmentTermService := service.NewEnrollmentTermService(enrollmentTermRepo, database)
	// services
	smtpConfig := service.SMTPConfig{
		Host:     cfg.SMTPHost,
		Port:     cfg.SMTPPort,
		Username: cfg.SMTPUsername,
		Password: cfg.SMTPPassword,
		From:     cfg.SMTPFrom,
		Enabled:  cfg.SMTPEnabled,
	}
	notificationDeliveryService := service.NewNotificationDeliveryService(
		notificationDeliveryRepo, communicationChannelRepo, notificationPrefRepo,
		notificationRepo, userRepo, smtpConfig,
	)
	auditService := service.NewAuditService(auditLogRepo, gradeChangeLogRepo)
	// services
	customRoleService := service.NewCustomRoleService(customRoleRepo, roleOverrideRepo, enrollmentRepo)
	onerosterService := service.NewOneRosterService(onerosterConnRepo, onerosterSyncLogRepo, userRepo, courseRepo, sectionRepo, enrollmentRepo, accountRepo, database)
	documentAnnotationService := service.NewDocumentAnnotationService(documentAnnotationRepo, submissionRepo, attachmentRepo, enrollmentRepo)
	// services
	discussionV2Service := service.NewDiscussionV2Service(
		discussionTopicRepo, discussionEntryRepo, discussionRatingRepo,
		discussionEntryParticipantRepo, discussionTopicParticipantRepo,
		discussionEntryVersionRepo, userRepo,
	)
	imsccParser := service.NewIMSCCParser(
		courseRepo, moduleRepo, moduleItemRepo, pageRepo, assignmentRepo,
		quizRepo, quizQuestionRepo, fileService, folderRepo, discussionTopicRepo,
		questionBankRepo, questionBankEntryRepo, assignmentGroupRepo, announcementRepo,
		rubricRepo, rubricAssocRepo, outcomeGroupRepo, outcomeRepo, calendarEventRepo,
	)
	// services
	coppaService := service.NewCOPPAService(parentalConsentRepo, dpaRepo, ageVerificationRepo)
	ferpaService := service.NewFERPAService(retentionPolicyRepo, deletionRequestRepo, exportRequestRepo, piiAccessLogRepo)
	accommodationService := service.NewAccommodationService(studentAccommodationRepo, accommodationApplicationRepo)
	// Wire accommodation service into quiz engine for IEP/504 time extensions
	service.WithAccommodationService(accommodationService)(quizService)
	attendanceService := service.NewAttendanceService(attendanceRepo)
	portfolioService := service.NewPortfolioService(portfolioRepo, portfolioSectionRepo, portfolioArtifactRepo, portfolioReflectionRepo, portfolioTemplateRepo, portfolioCommentRepo, submissionRepo, assignmentRepo)
	courseHomeService := service.NewCourseHomeService(courseRepo, courseHomeButtonRepo, todaysLessonOverrideRepo, courseVisitRepo, moduleRepo)
	peerReviewService := service.NewPeerReviewService(peerReviewRepo, submissionRepo, enrollmentRepo)
	questionBankService := service.NewQuestionBankService(questionBankRepo, questionBankEntryRepo, quizQuestionRepo)
	// P3 Feature services
	featureFlagService := service.NewFeatureFlagService(featureFlagRepo, courseRepo, accountRepo, userRepo)
	customGradebookColumnService := service.NewCustomGradebookColumnService(customGradebookColumnRepo, customColumnDatumRepo)
	masteryPathService := service.NewMasteryPathService(masteryPathRepo, submissionRepo, assignmentRepo)
	appointmentGroupService := service.NewAppointmentGroupService(appointmentGroupRepo, appointmentSlotRepo, appointmentReservationRepo, database)
	outcomeProficiencyService := service.NewOutcomeProficiencyService(outcomeProficiencyRepo)
	masteryGradebookService := service.NewMasteryGradebookService(enrollmentRepo, outcomeRepo, outcomeResultRepo, userRepo, outcomeProficiencyService)

	// Wire mastery-paths evaluation to fire after every successful grade.
	submissionService.OnGraded(func(ctx context.Context, subID uint) {
		if err := masteryPathService.EvaluateForStudent(ctx, subID); err != nil {
			slog.Warn("MasteryPathService.EvaluateForStudent failed",
				"submission_id", subID, "err", err)
		}
	})

	// Pairing codes (parent/observer linking).
	pairingCodeService := service.NewPairingCodeService(pairingCodeRepo, observerService)

	// Discussion Checkpoints, Smart Search, Commons, AI Assist
	discussionCheckpointService := service.NewDiscussionCheckpointService(
		discussionCheckpointRepo,
		discussionCheckpointSubmissionRepo,
		discussionTopicRepo,
		discussionEntryRepo,
		assignmentRepo,
	)
	smartSearchEmbedder := service.NewHashingEmbedder(0) // 0 -> default 384 dims
	smartSearchService, err := service.NewSmartSearchService(contentEmbeddingRepo, smartSearchEmbedder)
	if err != nil {
		log.Fatalf("Failed to initialize smart search service: %v", err)
	}
	commonsService := service.NewCommonsService(
		sharedContentRepo,
		courseRepo,
		assignmentRepo,
		pageRepo,
		quizRepo,
		moduleRepo,
		discussionTopicRepo,
	)
	aiAssistService := service.NewAIAssistService(cfg.AnthropicAPIKey)

	batchService := service.NewBatchService(
		courseRepo, moduleRepo, moduleItemRepo, assignmentRepo, quizRepo, quizQuestionRepo,
		pageRepo, discussionTopicRepo, calendarEventRepo, enrollmentRepo,
		conversationRepo, conversationParticipantRepo, conversationMessageRepo,
		userRepo, sectionRepo,
	)
	// SSO
	samlCfg := auth.SAMLConfig{
		EntityID:    cfg.SAMLEntityID,
		CertPEM:     cfg.SAMLCertFile,
		KeyPEM:      cfg.SAMLKeyFile,
		ACSURL:      cfg.FrontendURL + "/api/v1/auth/saml/acs",
		FrontendURL: cfg.FrontendURL,
		JWTSecret:   cfg.JWTSecret,
	}
	samlHandler := auth.NewSAMLHandler(samlCfg, userRepo, authProviderRepo)
	ldapAuth := auth.NewLDAPAuthenticator(userRepo)
	casAuth := auth.NewCASAuthenticator(userRepo)
	ssoHandler := auth.NewSSOHandler(samlHandler, ldapAuth, casAuth, userRepo, authProviderRepo, cfg)

	// Initialize token blacklist for session revocation on logout
	tokenBlacklist := service.NewTokenBlacklist()

	// Initialize handlers
	userHandler := handlers.NewUserHandler(userService, cfg.JWTSecret, cfg.Environment, tokenBlacklist, auditService)
	accountHandler := handlers.NewAccountHandler(accountRepo)
	courseHandler := handlers.NewCourseHandler(courseService, enrollmentService)
	sectionHandler := handlers.NewSectionHandler(sectionRepo)
	enrollmentHandler := handlers.NewEnrollmentHandler(enrollmentService)
	moduleHandler := handlers.NewModuleHandler(moduleService)
	moduleItemHandler := handlers.NewModuleItemHandler(moduleService, pageService)
	pageHandler := handlers.NewPageHandler(pageService)
	assignmentHandler := handlers.NewAssignmentHandler(assignmentService)
	assignmentGroupHandler := handlers.NewAssignmentGroupHandler(assignmentGroupService)
	submissionHandler := handlers.NewSubmissionHandler(submissionService, submissionCommentRepo, attachmentRepo, userRepo, assignmentRepo, notificationDeliveryService, observerService, outcomeAlignmentRepo, learningOutcomeService)
	gradebookHandler := handlers.NewGradebookHandler(gradingService)
	gradingStandardHandler := handlers.NewGradingStandardHandler(gradingStandardRepo)
	developerKeyHandler := handlers.NewDeveloperKeyHandler(devKeyService)
	accessTokenHandler := handlers.NewAccessTokenHandler(accessTokenService)
	oauth2Handler := handlers.NewOAuth2Handler(oauth2Service, devKeyService, accessTokenService, userService)
	externalToolHandler := handlers.NewExternalToolHandler(externalToolService, devKeyService)
	ltiHandler := handlers.NewLTIHandler(ltiService, agsService, nrpsService, externalToolRepo, ltiConfigRepo)
	// handlers
	discussionHandler := handlers.NewDiscussionHandler(discussionService)
	discussionEntryHandler := handlers.NewDiscussionEntryHandler(discussionService)
	fileHandler := handlers.NewFileHandler(fileService, enrollmentRepo)
	authz := handlers.NewResourceAuthorizer(enrollmentRepo, userRepo)
	folderHandler := handlers.NewFolderHandler(fileService, authz)
	sisImportHandler := handlers.NewSISImportHandler(sisImportService)
	// handlers
	quizHandler := handlers.NewQuizHandler(quizRepo)
	quizQuestionHandler := handlers.NewQuizQuestionHandler(quizService)
	quizSubmissionHandler := handlers.NewQuizSubmissionHandler(quizService, observerService)
	rubricHandler := handlers.NewRubricHandler(rubricService)
	rubricAssessmentHandler := handlers.NewRubricAssessmentHandler(rubricService)
	gradingPeriodHandler := handlers.NewGradingPeriodHandler(gradingPeriodService)
	assignmentOverrideHandler := handlers.NewAssignmentOverrideHandler(overrideService)
	latePolicyHandler := handlers.NewLatePolicyHandler(latePolicyService)
	// handlers
	calendarEventHandler := handlers.NewCalendarEventHandler(calendarService, authz)
	conversationHandler := handlers.NewConversationHandler(conversationService, userService)
	notificationHandler := handlers.NewNotificationHandler(notificationService)
	// handlers
	contentMigrationHandler := handlers.NewContentMigrationHandler(contentMigrationService)
	learningOutcomeHandler := handlers.NewLearningOutcomeHandler(learningOutcomeService, outcomeAlignmentRepo)
	speedGraderHandler := handlers.NewSpeedGraderHandler(speedGraderService)
	// handlers
	groupHandler := handlers.NewGroupHandler(groupService, authz)
	blueprintHandler := handlers.NewBlueprintHandler(blueprintService)
	coursePaceHandler := handlers.NewCoursePaceHandler(coursePaceService)
	// handlers
	collaborationHandler := handlers.NewCollaborationHandler(collaborationService, authz)
	conferenceHandler := handlers.NewConferenceHandler(conferenceService, authz)
	analyticsHandler := handlers.NewAnalyticsHandler(analyticsService)
	observerHandler := handlers.NewObserverHandler(observerService)
	// handlers
	graphqlResolver := graphql.NewResolver(courseService, assignmentService, userService, enrollmentService, moduleService, submissionService)
	graphqlHandler := handlers.NewGraphQLHandler(graphqlResolver)
	authProviderHandler := handlers.NewAuthProviderHandler(authProviderService)
	// handlers
	announcementHandler := handlers.NewAnnouncementHandler(announcementService, authz)
	enrollmentTermHandler := handlers.NewEnrollmentTermHandler(enrollmentTermService)
	syllabusHandler := handlers.NewSyllabusHandler(courseService, assignmentService, assignmentGroupService, calendarService, gradingService, enrollmentService, submissionService)
	// handlers
	notificationDeliveryHandler := handlers.NewNotificationDeliveryHandler(notificationDeliveryService, communicationChannelRepo)
	auditHandler := handlers.NewAuditHandler(auditService)
	// handlers
	customRoleHandler := handlers.NewCustomRoleHandler(customRoleService)
	onerosterHandler := handlers.NewOneRosterHandler(onerosterService)
	documentAnnotationHandler := handlers.NewDocumentAnnotationHandler(documentAnnotationService, submissionService, assignmentRepo, submissionRepo, authz)
	// handlers
	discussionV2Handler := handlers.NewDiscussionV2Handler(discussionV2Service)
	contentImportHandler := handlers.NewContentImportHandler(imsccParser, contentMigrationService, cfg.FileStoragePath)
	batchHandler := handlers.NewBatchHandler(batchService, authz)
	// handlers
	coppaHandler := handlers.NewCOPPAHandler(coppaService)
	ferpaHandler := handlers.NewFERPAHandler(ferpaService)
	accommodationHandler := handlers.NewAccommodationHandler(accommodationService, assignmentService, authz)
	attendanceHandler := handlers.NewAttendanceHandler(attendanceService)
	portfolioHandler := handlers.NewPortfolioHandler(portfolioService, authz)
	courseHomeHandler := handlers.NewCourseHomeHandler(courseHomeService)
	peerReviewHandler := handlers.NewPeerReviewHandler(peerReviewService)
	questionBankHandler := handlers.NewQuestionBankHandler(questionBankService)
	quizQuestionGroupHandler := handlers.NewQuizQuestionGroupHandler(quizService)
	quizStatisticsHandler := handlers.NewQuizStatisticsHandler(quizService)
	setupHandler := handlers.NewSetupHandler(userService, accountRepo, userRepo, cfg.JWTSecret, cfg.Environment)
	// P3 Feature handlers
	featureFlagHandler := handlers.NewFeatureFlagHandler(featureFlagService, enrollmentRepo, userRepo)
	customGradebookColumnHandler := handlers.NewCustomGradebookColumnHandler(customGradebookColumnService)
	masteryPathHandler := handlers.NewMasteryPathHandler(masteryPathService)
	appointmentGroupHandler := handlers.NewAppointmentGroupHandler(appointmentGroupService, authz)
	outcomeProficiencyHandler := handlers.NewOutcomeProficiencyHandler(outcomeProficiencyService, masteryGradebookService)
	pairingCodeHandler := handlers.NewPairingCodeHandler(pairingCodeService)
	// Wave 1 handlers
	discussionCheckpointHandler := handlers.NewDiscussionCheckpointHandler(discussionCheckpointService)
	// TODO: implement reindex source adapter wiring (announcement /
	// assignment / page / discussion topic listers). Until then Search works,
	// Reindex returns 501.
	smartSearchHandler := handlers.NewSmartSearchHandler(smartSearchService, nil)
	commonsHandler := handlers.NewCommonsHandler(commonsService, courseRepo)
	aiAssistHandler := handlers.NewAIAssistHandler(aiAssistService)
	authMiddleware := middleware.NewAuthMiddleware(cfg.JWTSecret, accessTokenService, userRepo, tokenBlacklist)
	permMiddleware := middleware.NewPermissionMiddleware(enrollmentRepo, userRepo)

	// Create router
	router := v1.NewRouter(
		userHandler,
		accountHandler,
		courseHandler,
		sectionHandler,
		enrollmentHandler,
		moduleHandler,
		moduleItemHandler,
		pageHandler,
		assignmentHandler,
		assignmentGroupHandler,
		submissionHandler,
		gradebookHandler,
		gradingStandardHandler,
		developerKeyHandler,
		accessTokenHandler,
		oauth2Handler,
		externalToolHandler,
		ltiHandler,
		discussionHandler,
		discussionEntryHandler,
		fileHandler,
		folderHandler,
		sisImportHandler,
		quizHandler,
		quizQuestionHandler,
		quizSubmissionHandler,
		rubricHandler,
		rubricAssessmentHandler,
		gradingPeriodHandler,
		assignmentOverrideHandler,
		latePolicyHandler,
		calendarEventHandler,
		conversationHandler,
		notificationHandler,
		contentMigrationHandler,
		learningOutcomeHandler,
		speedGraderHandler,
		groupHandler,
		blueprintHandler,
		coursePaceHandler,
		collaborationHandler,
		conferenceHandler,
		analyticsHandler,
		observerHandler,
		graphqlHandler,
		authProviderHandler,
		discussionV2Handler,
		contentImportHandler,
		batchHandler,
		ssoHandler,
		announcementHandler,
		enrollmentTermHandler,
		syllabusHandler,
		notificationDeliveryHandler,
		auditHandler,
		customRoleHandler,
		onerosterHandler,
		documentAnnotationHandler,
		coppaHandler,
		ferpaHandler,
		accommodationHandler,
		attendanceHandler,
		portfolioHandler,
		// Course Home Engine
		courseHomeHandler,
		// Peer Reviews, Question Banks
		peerReviewHandler,
		questionBankHandler,
		// Quiz Question Groups
		quizQuestionGroupHandler,
		// Quiz Statistics
		quizStatisticsHandler,
		// Setup
		setupHandler,
		// P3 Features
		featureFlagHandler,
		customGradebookColumnHandler,
		masteryPathHandler,
		appointmentGroupHandler,
		outcomeProficiencyHandler,
		// Pairing codes
		pairingCodeHandler,
				discussionCheckpointHandler,
		smartSearchHandler,
		commonsHandler,
		aiAssistHandler,
		authMiddleware,
		permMiddleware,
		accountRepo,
	)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		// Hard ceiling — admins can tune the actual cap via Account.MaxUploadSizeMB
		// (enforced by middleware.EnforceUploadSize). 5 GB safety net.
		BodyLimit: 5 * 1024 * 1024 * 1024,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"errors": []fiber.Map{{"message": err.Error()}},
			})
		},
	})

	// Health check endpoints (no auth, no middleware)
	healthHandler := handlers.NewHealthHandler(database, Version)
	app.Get("/health", healthHandler.Health)
	app.Get("/ready", healthHandler.Ready)

	// Middleware
	app.Use(middleware.RequestID())
	app.Use(middleware.SecurityHeaders(middleware.SecurityConfig{Environment: cfg.Environment}))
	app.Use(middleware.InputValidation())
	if cfg.Environment == "production" {
		app.Use(middleware.StructuredLogger())
	} else {
		app.Use(fiberlogger.New())
	}
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.FrontendURL,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-Request-ID, X-CSRF-Token",
		AllowMethods:     "GET, POST, PUT, DELETE, OPTIONS",
		ExposeHeaders:    "Link, X-Request-ID",
		AllowCredentials: true,
	}))

	// Register routes
	router.Register(app)

	// --- Background scheduler (weekly + daily digest jobs) ---------------------
	// DISABLE_SCHEDULER=1 short-circuits this for test/CI environments where the
	// scheduler would otherwise spam the notifications table.
	if os.Getenv("DISABLE_SCHEDULER") != "1" {
		sched := scheduler.NewScheduler(time.Hour)
		weeklyJob, dailyJob := scheduler.NewDigestJobs(notificationDeliveryService)
		sched.Register("weeklyDigest", scheduler.WeeklyAt(time.Monday, 7), weeklyJob)
		sched.Register("dailyDigest", scheduler.DailyAt(7), dailyJob)

		schedCtx, schedCancel := context.WithCancel(context.Background())
		defer schedCancel()
		sched.Start(schedCtx)

		// Graceful shutdown: stop the scheduler when the process receives
		// SIGINT/SIGTERM. Fiber's app.Listen blocks the main goroutine, so we
		// run signal handling in its own goroutine.
		go func() {
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh
			sched.Stop()
			_ = app.Shutdown()
		}()
	}
	// --------------------------------------------------------------------------

	// Start server
	slog.Info("Paper LMS starting", "port", cfg.Port, "environment", cfg.Environment, "version", Version)
	log.Fatal(app.Listen(":" + cfg.Port))
}
