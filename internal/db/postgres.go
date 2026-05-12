package db

import (
	"fmt"
	"log"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Connect(databaseURL string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetMaxIdleConns(25)

	log.Println("Connected to PostgreSQL database")
	return db, nil
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.Account{},
		&models.Course{},
		&models.CourseSection{},
		&models.Enrollment{},
		&models.ContextModule{},
		&models.ContentTag{},
		&models.WikiPage{},
		&models.Assignment{},
		&models.Quiz{},
		&models.AssignmentGroup{},
		&models.Submission{},
		&models.SubmissionComment{},
		&models.GradingStandard{},
		// OAuth2, Personal Access Tokens, LTI 1.3
		&models.DeveloperKey{},
		&models.AccessToken{},
		&models.LTIToolConfiguration{},
		&models.ContextExternalTool{},
		&models.LTIResourceLink{},
		&models.LTILineItem{},
		&models.LTIResult{},
		&models.Nonce{},
		// Discussions, Files, SIS
		&models.DiscussionTopic{},
		&models.DiscussionEntry{},
		&models.DiscussionEntryRating{},
		&models.Folder{},
		&models.Attachment{},
		&models.SISBatch{},
		&models.SISBatchError{},
		// Quiz Engine, Rubrics, Grading Periods, Overrides
		&models.QuizQuestion{},
		&models.QuizSubmission{},
		&models.QuizSubmissionAnswer{},
		&models.Rubric{},
		&models.RubricAssociation{},
		&models.RubricAssessment{},
		&models.GradingPeriodGroup{},
		&models.GradingPeriod{},
		&models.AssignmentOverride{},
		&models.AssignmentOverrideStudent{},
		&models.LatePolicy{},
		// Calendar, Messaging, Notifications
		&models.CalendarEvent{},
		&models.Conversation{},
		&models.ConversationParticipant{},
		&models.ConversationMessage{},
		&models.NotificationPreference{},
		&models.Notification{},
		// Content Migration, Learning Outcomes
		&models.ContentMigration{},
		&models.LearningOutcomeGroup{},
		&models.LearningOutcome{},
		&models.LearningOutcomeResult{},
		&models.OutcomeAlignment{},
		// Groups, Blueprint Courses, Course Pacing
		&models.GroupCategory{},
		&models.Group{},
		&models.GroupMembership{},
		&models.BlueprintTemplate{},
		&models.BlueprintSubscription{},
		&models.BlueprintMigration{},
		&models.CoursePace{},
		&models.CoursePaceModuleItem{},
		// Collaborations, Conferences, Analytics
		&models.Collaboration{},
		&models.Conference{},
		&models.ConferenceParticipant{},
		&models.PageView{},
		// Authentication Providers
		&models.AuthenticationProvider{},
		// Discussion V2
		&models.DiscussionEntryParticipant{},
		&models.DiscussionTopicParticipant{},
		&models.DiscussionEntryVersion{},
		// Announcements, Enrollment Terms
		&models.Announcement{},
		&models.AnnouncementReadReceipt{},
		&models.EnrollmentTerm{},
		// Notification Delivery, Audit Logs
		&models.CommunicationChannel{},
		&models.NotificationDelivery{},
		&models.AuditLog{},
		&models.GradeChangeLog{},
		// Custom Roles, OneRoster, Document Annotations
		&models.CustomRole{},
		&models.RoleOverride{},
		&models.OneRosterConnection{},
		&models.OneRosterSyncLog{},
		&models.DocumentAnnotation{},
		// COPPA, FERPA, Accommodations, Attendance, Portfolios
		&models.ParentalConsent{},
		&models.DataProcessingAgreement{},
		&models.AgeVerification{},
		&models.DataRetentionPolicy{},
		&models.DataDeletionRequest{},
		&models.DataExportRequest{},
		&models.PIIAccessLog{},
		&models.StudentAccommodation{},
		&models.AccommodationApplication{},
		&models.AttendanceRecord{},
		&models.Portfolio{},
		&models.PortfolioSection{},
		&models.PortfolioArtifact{},
		&models.PortfolioReflection{},
		&models.PortfolioTemplate{},
		&models.PortfolioComment{},
		// Course Home Engine
		&models.CourseHomeButton{},
		&models.TodaysLessonOverride{},
		&models.CourseVisit{},
		// Peer Reviews, Question Banks, Module Prerequisites
		&models.PeerReview{},
		&models.QuestionBank{},
		&models.QuestionBankEntry{},
		&models.ModulePrerequisite{},
		// Quiz Question Groups (random selection anti-cheating)
		&models.QuizQuestionGroup{},
		// P3 Features: Feature Flags
		&models.FeatureFlag{},
		// P3 Features: Custom Gradebook Columns
		&models.CustomGradebookColumn{},
		&models.CustomColumnDatum{},
		// P3 Features: Mastery Paths (Conditional Release)
		&models.ConditionalReleaseRule{},
		&models.ConditionalReleaseScoringRange{},
		&models.ConditionalReleaseAssignmentSet{},
		&models.ConditionalReleaseAssignmentSetAssociation{},
		&models.ConditionalReleaseAssignmentSetAction{},
		// P3 Features: Appointment Groups (Scheduler)
		&models.AppointmentGroup{},
		&models.AppointmentSlot{},
		&models.AppointmentReservation{},
		// P3 Features: Outcome Proficiency
		&models.OutcomeProficiency{},
		&models.OutcomeProficiencyRating{},
		// Parent/observer pairing codes
		&models.PairingCode{},
		// Discussion Checkpoints, Smart Search, Commons
		&models.DiscussionCheckpoint{},
		&models.DiscussionCheckpointSubmission{},
		&models.ContentEmbedding{},
		&models.SharedContent{},
		&models.SharedContentFavorite{},
		// Wave A2: Quiz Item Banks, Stimulus Passages, Per-Question Outcome Alignment
		&models.QuizItemBank{},
		&models.QuizItemBankItem{},
		&models.QuizStimulus{},
		&models.QuizQuestionOutcomeAlignment{},
		// Phase 6 Wave 1: gamification foundations (migrations 000032-000035).
		// All indexes for these tables live in the SQL chain, not the GORM tags,
		// because the migrations use DESC ordering and partial WHERE clauses
		// AutoMigrate can't reproduce.
		&models.GamificationEvent{},
		&models.GamificationRule{},
		&models.GamificationRuleEvaluation{},
		&models.GamificationCurrencyType{},
		&models.GamificationWalletBalance{},
		&models.GamificationWalletTransaction{},
		&models.GamificationFerpaFieldTag{},
	)
}

func SeedDefaultAccount(db *gorm.DB) error {
	var count int64
	db.Model(&models.Account{}).Count(&count)
	if count == 0 {
		account := models.Account{
			Name:          "Paper LMS",
			WorkflowState: "active",
		}
		if err := db.Create(&account).Error; err != nil {
			return fmt.Errorf("failed to seed default account: %w", err)
		}
		log.Println("Created default account: Paper LMS")
	}
	return nil
}
