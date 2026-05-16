package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/gofiber/fiber/v2/middleware/cors"
	fiberlogger "github.com/gofiber/fiber/v2/middleware/logger"
	fiberrecover "github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"

	v1 "github.com/EduThemes/paper-lms/internal/api/v1"
	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/auth"
	"github.com/EduThemes/paper-lms/internal/config"
	"github.com/EduThemes/paper-lms/internal/db"
	"github.com/EduThemes/paper-lms/internal/graphql"
	"github.com/EduThemes/paper-lms/internal/obs"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/scheduler"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
	gamificationEffects "github.com/EduThemes/paper-lms/internal/service/gamification/effects"
	"github.com/EduThemes/paper-lms/internal/service/gamification/wiring"
	storageLib "github.com/EduThemes/paper-lms/internal/storage"
)

// Version is set at build time via -ldflags
var Version = "dev"

func main() {
	_ = godotenv.Load()

	cfg := config.Load()
	cfg.Validate()

	// Encryption-at-rest keys (MFA_ENCRYPTION_KEY). Loaded eagerly so a
	// missing or malformed key surfaces at boot, not on the first MFA
	// enrollment. Fatal in production; warning elsewhere so the dev
	// loop still works before an operator has provisioned the key.
	if err := auth.EnsureKeysLoaded(); err != nil {
		if cfg.Environment == "production" {
			log.Fatalf("encryption keys unavailable: %v", err)
		}
		log.Printf("warning: encryption keys unavailable (%v) — MFA, OIDC client secrets, and passkeys will fail; set MFA_ENCRYPTION_KEY before exercising those flows", err)
	}

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

	// Observability plumbing (G2). Tracer provider + Prometheus
	// registry. No-op exporter unless OBSERVABILITY_OTLP_ENDPOINT is
	// set; /metrics is always served. Per-request instrumentation lands
	// via middleware.Observability below.
	obsShutdown, err := obs.Init(context.Background(), obs.LoadConfig(Version, cfg.Environment))
	if err != nil {
		log.Fatalf("Failed to initialize observability: %v", err)
	}

	// Connect to PostgreSQL
	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Database schema management. The SQL chain (cmd/migrate, run via
	// `make migrate-up`) is the source of truth. AUTO_MIGRATE is a
	// developer convenience for fast model-add iteration; production
	// always runs the SQL chain.
	//
	// AutoMigrate is intentionally non-fatal when it fails: GORM model
	// declarations and the SQL chain can disagree on constraint names
	// (e.g. GORM's `uniqueIndex` produces a `uni_*` *constraint* while
	// the SQL chain may have created an `idx_*` *index* with the same
	// semantics — see late_policies). On a freshly-SQL-migrated DB
	// AutoMigrate would try to drop the missing constraint and crash
	// the server. We log + continue: the schema is already at HEAD.
	if cfg.AutoMigrate {
		log.Println("AUTO_MIGRATE=true: running GORM AutoMigrate (development convenience)")
		if err := db.AutoMigrate(database); err != nil {
			log.Printf("warning: AutoMigrate reported a constraint mismatch (non-fatal): %v", err)
			log.Println("note: SQL migration chain is the source of truth; constraint-shape mismatches between GORM tags and SQL DDL are expected and don't affect runtime.")
		}
	} else {
		log.Println("AUTO_MIGRATE=false: relying on versioned SQL migrations (run `make migrate-up` separately)")
		if err := db.MigrateUp(database); err != nil {
			log.Fatalf("Failed to run database migrations: %v", err)
		}
	}

	// Seed default data
	if err := db.SeedDefaultAccount(database); err != nil {
		log.Fatalf("Failed to seed database: %v", err)
	}

	// Backfill system gamification currencies for every tenant. Idempotent —
	// re-runs on every boot, no-ops on already-populated tenants thanks to
	// the uniq_gam_currency_scope_code index + ON CONFLICT DO NOTHING.
	{
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		var accounts []models.Account
		if err := database.WithContext(ctx).Select("id").Find(&accounts).Error; err != nil {
			cancel()
			log.Fatalf("Failed to list accounts for gamification seed: %v", err)
		}
		for _, a := range accounts {
			if err := gamification.SeedSystemCurrenciesForTenant(ctx, database, a.ID); err != nil {
				cancel()
				log.Fatalf("Failed to seed gamification currencies for account %d: %v", a.ID, err)
			}
		}
		cancel()
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

	// Phase 6 Wave 1 Sprint D: gamification repositories + the rule-
	// engine Emitter. Constructed here so any service that needs to fire
	// xAPI events (via the callback hooks added in Sprint D) can have
	// the emitter registered against it.
	gamificationEventRepo := postgres.NewGamificationEventRepository(database)
	gamificationRuleRepo := postgres.NewGamificationRuleRepository(database)
	gamificationCurrencyTypeRepo := postgres.NewGamificationCurrencyTypeRepository(database)
	gamificationWalletRepo := postgres.NewGamificationWalletRepository(database)
	gamificationFerpaTagRepo := postgres.NewGamificationFerpaFieldTagRepository(database)
	gamificationBadgeRepo := postgres.NewGamificationBadgeRepository(database)
	gamificationBadgeAwardRepo := postgres.NewGamificationBadgeAwardRepository(database)
	contentViewRepo := postgres.NewContentViewRepository(database)

	gamificationEmitter := gamification.NewEmitter(gamification.EmitterDeps{
		Dispatch: gamification.DispatchDeps{
			Snapshot: gamification.SnapshotDeps{
				Submissions:     submissionRepo,
				QuizSubmissions: quizSubmissionRepo,
				OutcomeResults:  outcomeResultRepo,
				ContentViews:    contentViewRepo,
				Wallet:          gamificationWalletRepo,
				CurrencyType:    gamificationCurrencyTypeRepo,
			},
			Rules: gamificationRuleRepo,
			Effects: gamificationEffects.EffectDeps{
				Wallet:       gamificationWalletRepo,
				CurrencyType: gamificationCurrencyTypeRepo,
				Badge:        gamificationBadgeRepo,
				BadgeAward:   gamificationBadgeAwardRepo,
				// BadgeEmit is wired below via SetBadgeEmitter: the
				// emitter itself is the chain-emit sink (it satisfies
				// effects.BadgeEarnedEmitter), but it can't reference
				// itself inside its own constructor literal.
			},
		},
		Events:    gamificationEventRepo,
		FerpaTags: gamificationFerpaTagRepo,
	})
	// W2-E.1: close the badge.earned chain so AwardBadge can fire a
	// downstream event on first-time awards.
	gamificationEmitter.SetBadgeEmitter(gamificationEmitter)

	// Initialize services
	userService := service.NewUserService(userRepo)
	courseService := service.NewCourseService(courseRepo, enrollmentRepo, sectionRepo)
	enrollmentService := service.NewEnrollmentService(enrollmentRepo)
	contentViewService := service.NewContentViewService(contentViewRepo)
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
	// Phase 6 Wave 1 Sprint D-1: register every gamification emit
	// callback against the services that own the lifecycle event. The
	// wiring functions in internal/service/gamification/wiring build
	// callbacks closed over the right repositories; failures inside
	// these callbacks are logged via slog and never propagate to the
	// originating request (the goal is "rule fires never break grading
	// or submission writes").
	submissionService.OnGraded(wiring.GradedSubmissionEmitCallback(
		gamificationEmitter, submissionRepo, assignmentRepo, courseRepo,
	))
	quizService.OnCompleted(wiring.CompletedQuizEmitCallback(
		gamificationEmitter, quizSubmissionRepo, quizRepo, courseRepo,
	))
	enrollmentService.OnCreated(wiring.EnrolledCourseEmitCallback(
		gamificationEmitter, enrollmentRepo, courseRepo,
	))
	contentViewService.OnViewed(wiring.ViewedContentEmitCallback(
		gamificationEmitter, pageRepo, courseRepo,
	))

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

	// Phase 6 Wave 1 Sprint D-2: register the remaining three emit
	// callbacks (discussion entries, rubric assessments, outcome-mastery
	// transitions) against the services that own each lifecycle event.
	// Same async + slog.Error-only error contract as the Sprint D-1 block
	// above — a failed emit never breaks the originating write.
	discussionService.OnEntryCreated(wiring.DiscussionEntryPostedEmitCallback(
		gamificationEmitter, discussionEntryRepo, discussionTopicRepo, courseRepo,
	))
	rubricService.OnAssessmentCreated(wiring.RubricAssessmentCreatedEmitCallback(
		gamificationEmitter, rubricAssessRepo, rubricRepo, courseRepo,
	))
	learningOutcomeService.OnMasteryCrossed(wiring.OutcomeMasteryCrossedEmitCallback(
		gamificationEmitter, outcomeResultRepo, outcomeRepo, courseRepo,
	))

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
	// 12.8 — wire user/enrollment repos so BuildExportZip can assemble
	// the right-of-access ZIP.
	ferpaService.SetExportDataDeps(userRepo, enrollmentRepo)
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
	// 12.6 — wire repos needed for the teacher-or-self mint consent
	// check on POST /users/:student_id/observer-pairing-codes.
	pairingCodeService.SetAuthzDeps(enrollmentRepo, courseRepo, accountRepo)

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
	// Phase 9-PRE — federated auth + MFA foundations. Constructed
	// before SAML/LDAP/CAS so the legacy SSO handlers can take it as
	// a dependency (Sprint 10-C).
	federatedIdentityRepo := postgres.NewFederatedIdentityRepository(database)
	authAudit := auth.NewAuthAudit(auditService)
	loginPipeline := auth.NewLoginPipeline(
		userRepo,
		federatedIdentityRepo,
		authProviderRepo,
		accountRepo,
		authAudit,
		cfg.JWTSecret,
	)

	samlHandler := auth.NewSAMLHandler(samlCfg, userRepo, authProviderRepo, loginPipeline)
	ldapAuth := auth.NewLDAPAuthenticator()
	casAuth := auth.NewCASAuthenticator()
	ssoHandler := auth.NewSSOHandler(samlHandler, ldapAuth, casAuth, userRepo, authProviderRepo, cfg, loginPipeline)

	// Initialize token blacklist for session revocation on logout
	tokenBlacklist := service.NewTokenBlacklist()
	// Phase 9-A — OIDC client. Reads OIDC_REDIRECT_BASE from env; in
	// dev defaults to http://localhost:3000 inside the handler.
	oidcHandler := auth.NewOIDCHandler(authProviderRepo, loginPipeline, "", os.Getenv("OIDC_REDIRECT_BASE"))
	// Phase 9-B / 10-A — TOTP MFA + per-token rate limiting.
	userRecoveryCodeRepo := postgres.NewUserRecoveryCodeRepository(database)
	mfaRateLimit := auth.NewMFAAttemptTracker(nil)
	mfaHandler := handlers.NewMFAHandler(userRepo, userRecoveryCodeRepo, cfg.JWTSecret, userService, mfaRateLimit)

	// Phase 10-B — passkeys (WebAuthn). RPID is the eTLD+1 of the
	// site origin; in dev that's "localhost". RPOrigins is the full
	// origin set the browser will connect from — frontend dev server
	// + backend. PASSKEY_RPID / PASSKEY_RPORIGINS override at deploy
	// time.
	passkeyRPID := os.Getenv("PASSKEY_RPID")
	if passkeyRPID == "" {
		passkeyRPID = "localhost"
	}
	passkeyOrigins := []string{cfg.FrontendURL}
	if extra := os.Getenv("PASSKEY_RPORIGINS"); extra != "" {
		for _, o := range strings.Split(extra, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				passkeyOrigins = append(passkeyOrigins, o)
			}
		}
	}
	userWebauthnCredRepo := postgres.NewUserWebauthnCredentialRepository(database)
	passkeyEngine, err := auth.NewPasskeyEngine("Paper LMS", passkeyRPID, passkeyOrigins, userRepo, userWebauthnCredRepo)
	if err != nil {
		log.Fatalf("Failed to initialize passkey engine: %v", err)
	}
	passkeyHandler := handlers.NewPasskeyHandler(passkeyEngine, userRepo, userWebauthnCredRepo, loginPipeline, authAudit, cfg.JWTSecret)

	// Initialize handlers
	userHandler := handlers.NewUserHandler(userService, cfg.JWTSecret, cfg.Environment, tokenBlacklist, auditService, loginPipeline)
	accountHandler := handlers.NewAccountHandler(accountRepo)
	courseHandler := handlers.NewCourseHandler(courseService, enrollmentService)
	sectionHandler := handlers.NewSectionHandler(sectionRepo)
	enrollmentHandler := handlers.NewEnrollmentHandler(enrollmentService)
	moduleHandler := handlers.NewModuleHandler(moduleService)
	moduleItemHandler := handlers.NewModuleItemHandler(moduleService, pageService)
	pageHandler := handlers.NewPageHandler(pageService)
	// Sprint D-1: instrument the authenticated page-fetch path so every
	// render upserts content_views and fans out to the ViewedContent
	// callback that fires gamification rules.
	pageHandler.SetContentViewService(contentViewService)
	gamificationLeaderboardSnapshotRepo := postgres.NewGamificationLeaderboardSnapshotRepository(database)
	gamificationHandler := handlers.NewGamificationHandler(gamificationWalletRepo, gamificationCurrencyTypeRepo, userRepo, gamificationBadgeRepo, gamificationBadgeAwardRepo, gamificationRuleRepo, enrollmentRepo, accountRepo, gamificationLeaderboardSnapshotRepo)
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
	observerHandler := handlers.NewObserverHandler(observerService, pairingCodeService)
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
	aiAssistHandler := handlers.NewAIAssistHandler(aiAssistService, accountRepo)

	// Wave A2: Quiz Item Banks, Stimuli, per-question Outcome Alignments
	quizItemBankRepo := postgres.NewQuizItemBankRepository(database)
	quizItemBankItemRepo := postgres.NewQuizItemBankItemRepository(database)
	quizStimulusRepo := postgres.NewQuizStimulusRepository(database)
	quizQuestionOutcomeAlignmentRepo := postgres.NewQuizQuestionOutcomeAlignmentRepository(database)
	quizItemBankService := service.NewQuizItemBankService(quizItemBankRepo, quizItemBankItemRepo, quizQuestionRepo)
	quizStimulusService := service.NewQuizStimulusService(quizStimulusRepo, quizQuestionRepo)
	quizOutcomeAlignmentService := service.NewQuizOutcomeAlignmentService(quizQuestionOutcomeAlignmentRepo, quizQuestionRepo, outcomeRepo)
	quizItemBankHandler := handlers.NewQuizItemBankHandler(quizItemBankService)
	quizStimulusHandler := handlers.NewQuizStimulusHandler(quizStimulusService)
	quizOutcomeAlignmentHandler := handlers.NewQuizOutcomeAlignmentHandler(quizOutcomeAlignmentService)

	// Wave B: QTI / IMSCC importer + exporter. Sync only in v1 — no
	// import-history table; partial-failure surfaces in the response
	// summary.
	qtiImportService := service.NewQTIImportService(quizRepo, quizQuestionRepo, quizItemBankService, quizStimulusService, cfg.FileStoragePath)
	qtiImportHandler := handlers.NewQTIImportHandler(qtiImportService)

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
		oidcHandler,
		mfaHandler,
		passkeyHandler,
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
		// Wave A2
		quizItemBankHandler,
		quizStimulusHandler,
		quizOutcomeAlignmentHandler,
		// Wave B: QTI import + export.
		qtiImportHandler,
		gamificationHandler,
		authMiddleware,
		permMiddleware,
		accountRepo,
	)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		// Per-account upload caps live on Account.MaxUploadSizeMB and are
		// enforced by middleware.EnforceUploadSize on upload routes only
		// (POST /courses/:id/files, POST /courses/:id/content_imports).
		// This BodyLimit is the global default for every OTHER route —
		// JSON, form bodies, etc. 100 MB is far above any realistic
		// non-file body and keeps DoS-class POST bombs out of fasthttp's
		// memory.
		BodyLimit: 100 * 1024 * 1024,
		// fasthttp's default ReadBufferSize is 4 KB, which is too small
		// for browsers that accumulate cookies across local dev sessions
		// (each session leaves a JWT cookie; localhost shares cookies
		// across all apps on the same host). When the Cookie header
		// exceeds 4 KB, fasthttp rejects with HTTP 431. Bump to 64 KB —
		// well above any realistic cookie load, still small enough that
		// truly malformed requests get rejected.
		ReadBufferSize: 64 * 1024,
		// Slowloris-class defenses. fasthttp's defaults are "no
		// deadline", which lets a malicious peer hold a socket open
		// indefinitely by drip-feeding bytes. Real browser uploads
		// finish well under WriteTimeout; the IdleTimeout caps keep-
		// alive between requests.
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			msg := err.Error()
			// 5xx responses must NOT echo err.Error() — GORM error
			// text, SQL fragments, and stack frames can carry
			// privileged data (IPs, user emails, internal hostnames).
			// Log the full text server-side keyed by request_id; the
			// client gets a generic envelope.
			if code >= 500 {
				reqID, _ := c.Locals("request_id").(string)
				slog.Error("unhandled server error",
					"request_id", reqID,
					"path", c.Path(),
					"method", c.Method(),
					"err", err.Error(),
				)
				msg = "internal server error"
			}
			return c.Status(code).JSON(fiber.Map{
				"errors": []fiber.Map{{"message": msg}},
			})
		},
	})

	// Health check endpoints (no auth, no middleware)
	healthHandler := handlers.NewHealthHandler(database, Version)
	app.Get("/health", healthHandler.Health)
	app.Get("/ready", healthHandler.Ready)
	// Prometheus exposition. No auth (matches /health) — scrapers
	// reach it via the cluster network only; production deployments
	// keep this port behind the ingress allowlist.
	app.Get("/metrics", adaptor.HTTPHandler(obs.MetricsHandler()))

	// Panic recovery must come BEFORE everything else so a panic in any
	// downstream middleware or handler produces a 500 JSON envelope
	// rather than dropping the connection. Stack traces are only
	// surfaced outside production.
	app.Use(fiberrecover.New(fiberrecover.Config{
		EnableStackTrace: cfg.Environment != "production",
	}))

	// Middleware. RequestID must precede Observability (the obs span
	// reads request_id) and Observability must precede StructuredLogger
	// (so the log line can carry the trace_id).
	app.Use(middleware.RequestID())
	app.Use(middleware.Observability())
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

	// Dev-only smoke endpoint for the panic-recovery middleware. NEVER
	// mounted outside development. See 12.1 in
	// /Users/alfred/.claude/plans/phase-12-tier-1-university-hardening.md.
	if cfg.Environment == "development" {
		app.Get("/_test/panic", func(c *fiber.Ctx) error {
			panic("intentional panic for recover.New() smoke test")
		})
	}

	// --- Background scheduler (weekly + daily digest jobs) ---------------------
	// DISABLE_SCHEDULER=1 short-circuits this for test/CI environments where the
	// scheduler would otherwise spam the notifications table. Graceful-shutdown
	// wiring is mounted UNCONDITIONALLY below so DISABLE_SCHEDULER does not
	// silently disable in-flight request draining.
	var sched *scheduler.Scheduler
	if os.Getenv("DISABLE_SCHEDULER") != "1" {
		sched = scheduler.NewScheduler(time.Hour)
		// 13.7 — Postgres advisory-lock leader election. Multi-pod
		// deploys all attempt each job's window; only one wins the
		// lock and runs.
		sched.SetLeaderLock(scheduler.NewPGLeaderLock(database))
		weeklyJob, dailyJob := scheduler.NewDigestJobs(notificationDeliveryService)
		sched.Register("weeklyDigest", scheduler.WeeklyAt(time.Monday, 7), weeklyJob)
		sched.Register("dailyDigest", scheduler.DailyAt(7), dailyJob)

		schedCtx, schedCancel := context.WithCancel(context.Background())
		defer schedCancel()
		sched.Start(schedCtx)
	}

	// Graceful shutdown — wired even when the scheduler is disabled so
	// in-flight requests still drain on SIGTERM. `app.Listen` blocks the
	// main goroutine, so signal handling runs in its own goroutine.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		slog.Info("shutdown signal received, draining…")
		if sched != nil {
			sched.Stop()
		}
		_ = app.ShutdownWithTimeout(30 * time.Second)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = obsShutdown(shutdownCtx)
	}()
	// --------------------------------------------------------------------------

	// Start server
	slog.Info("Paper LMS starting", "port", cfg.Port, "environment", cfg.Environment, "version", Version)
	if err := app.Listen(":" + cfg.Port); err != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = obsShutdown(shutdownCtx)
		cancel()
		log.Fatalf("server exited: %v", err)
	}
}

