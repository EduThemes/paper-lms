package repository

import (
	"context"
	"errors"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// ErrCurrencyDuplicate is returned by GamificationCurrencyTypeRepository.Create
// when the (tenant_id, scope_type, scope_id, code) tuple already exists. The
// repo translates the unique-constraint hit atomically via
// `INSERT ... ON CONFLICT DO NOTHING RETURNING ...`, so callers can map this
// to a 409 without a two-query pre-check race window.
var ErrCurrencyDuplicate = errors.New("currency with this code already exists in this scope")

// ErrBadgeDuplicate is the W2-D analog of ErrCurrencyDuplicate for the
// (tenant_id, scope_type, scope_id, code) uniqueness constraint on
// gamification_badges. Same atomic INSERT ... ON CONFLICT DO NOTHING
// pattern, same handler→409 translation.
var ErrBadgeDuplicate = errors.New("badge with this code already exists in this scope")

type PaginationParams struct {
	Page    int
	PerPage int
}

type PaginatedResult[T any] struct {
	Items      []T
	TotalCount int64
	Page       int
	PerPage    int
}

type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	FindByID(ctx context.Context, id uint) (*models.User, error)
	FindByLoginID(ctx context.Context, loginID string) (*models.User, error)
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	FindBySISUserID(ctx context.Context, sisUserID string) (*models.User, error)
	FindByIDs(ctx context.Context, ids []uint) ([]models.User, error)
	Update(ctx context.Context, user *models.User) error
	List(ctx context.Context, params PaginationParams) (*PaginatedResult[models.User], error)
	FindByResetToken(ctx context.Context, token string) (*models.User, error)
	Search(ctx context.Context, searchTerm string, params PaginationParams) (*PaginatedResult[models.User], error)
	// FilterPublicLeaderboardCandidates returns the subset of `candidateIDs`
	// that have NOT opted out of public leaderboards (W2-C). Used by any
	// leaderboard query path before projection. Stacks with the data-access
	// FERPA block on `mastery_points` — opt-out is the per-learner privacy
	// control, FERPA is the field-classification control; both must allow
	// for a row to surface on a public board. Ships in W2-C so Wave 3's
	// leaderboard primitives don't retrofit the privacy guard later.
	FilterPublicLeaderboardCandidates(ctx context.Context, candidateIDs []uint) ([]uint, error)
}

type AccountRepository interface {
	Create(ctx context.Context, account *models.Account) error
	FindByID(ctx context.Context, id uint) (*models.Account, error)
	Update(ctx context.Context, account *models.Account) error
	List(ctx context.Context, params PaginationParams) (*PaginatedResult[models.Account], error)
}

type CourseRepository interface {
	Create(ctx context.Context, course *models.Course) error
	// FindByID — 13.1.D: tenant-scoped. accountID==0 means "no scope"
	// and is permitted only from internal callers that have already
	// validated tenant ownership upstream (e.g. background workers).
	// Handler-layer callers MUST pass the caller's account_id.
	FindByID(ctx context.Context, id, accountID uint) (*models.Course, error)
	FindBySISCourseID(ctx context.Context, sisCourseID string) (*models.Course, error)
	Update(ctx context.Context, course *models.Course) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, accountID uint, params PaginationParams) (*PaginatedResult[models.Course], error)
	ListByUserID(ctx context.Context, userID, accountID uint, params PaginationParams) (*PaginatedResult[models.Course], error)
}

type SectionRepository interface {
	Create(ctx context.Context, section *models.CourseSection) error
	FindByID(ctx context.Context, id uint) (*models.CourseSection, error)
	FindBySISSectionID(ctx context.Context, sisSectionID string) (*models.CourseSection, error)
	ListByCourseID(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.CourseSection], error)
}

type EnrollmentRepository interface {
	Create(ctx context.Context, enrollment *models.Enrollment) error
	FindByID(ctx context.Context, id uint) (*models.Enrollment, error)
	Update(ctx context.Context, enrollment *models.Enrollment) error
	ListByCourseID(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.Enrollment], error)
	ListByUserID(ctx context.Context, userID uint) ([]models.Enrollment, error)
	FindByUserAndCourse(ctx context.Context, userID, courseID uint) (*models.Enrollment, error)
	CountByCourseIDs(ctx context.Context, courseIDs []uint) (map[uint]int64, error)
	// ListActiveStudentUserIDsByCourse (W3-A) returns user_ids of active
	// StudentEnrollment rows for a course — the leaderboard candidate
	// set. Uses idx_enrollments_course_active (migration 000042).
	ListActiveStudentUserIDsByCourse(ctx context.Context, courseID uint) ([]uint, error)
	// ListActiveStudentEnrollmentsByCourse (W3-B) returns full
	// Enrollment rows for the same set — needed when the caller
	// also has to read per-enrollment pseudonym fields rather than
	// just user_ids.
	ListActiveStudentEnrollmentsByCourse(ctx context.Context, courseID uint) ([]models.Enrollment, error)
	// UpdatePseudonymForSelf (W3-B) writes a learner-chosen pseudonym
	// to their enrollment row in the given course. Returns
	// repository.ErrPseudonymTaken on UNIQUE collision so the handler
	// can map it to a 409.
	UpdatePseudonymForSelf(ctx context.Context, userID, courseID uint, poolCode, name string) error
}

// ErrPseudonymTaken indicates that another active enrollment in the
// same course already has the requested pseudonym name in the same
// pool. The handler maps this to 409 so the picker UI can offer the
// learner a re-roll.
var ErrPseudonymTaken = errors.New("pseudonym already taken in this course pool")

type ModuleRepository interface {
	Create(ctx context.Context, module *models.ContextModule) error
	// 13.1.D — accountID scopes the read to a single tenant via the
	// parent course's account_id. 0 means "no tenant scope" (internal
	// callers only).
	FindByID(ctx context.Context, id, accountID uint) (*models.ContextModule, error)
	Update(ctx context.Context, module *models.ContextModule) error
	Delete(ctx context.Context, id uint) error
	ListByCourseID(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.ContextModule], error)
	FindActiveByDateRange(ctx context.Context, courseID uint, date time.Time) (*models.ContextModule, error)
	ReorderModules(ctx context.Context, courseID uint, moduleIDs []uint) error
}

type ModuleItemRepository interface {
	Create(ctx context.Context, item *models.ContentTag) error
	FindByID(ctx context.Context, id uint) (*models.ContentTag, error)
	Update(ctx context.Context, item *models.ContentTag) error
	Delete(ctx context.Context, id uint) error
	ListByModuleID(ctx context.Context, moduleID uint, params PaginationParams) (*PaginatedResult[models.ContentTag], error)
	ReorderItems(ctx context.Context, moduleID uint, itemIDs []uint) error
	MoveItemToModule(ctx context.Context, itemID uint, targetModuleID uint, position int) error
}

type PageRepository interface {
	Create(ctx context.Context, page *models.WikiPage) error
	// 13.1.D — tenant scope via parent course's account_id.
	FindByID(ctx context.Context, id, accountID uint) (*models.WikiPage, error)
	FindByCourseAndURL(ctx context.Context, courseID uint, url string) (*models.WikiPage, error)
	FindPublicByCourseAndURL(ctx context.Context, courseID uint, url string) (*models.WikiPage, error)
	Update(ctx context.Context, page *models.WikiPage) error
	Delete(ctx context.Context, id uint) error
	ListByCourseID(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.WikiPage], error)
}

type AssignmentRepository interface {
	Create(ctx context.Context, assignment *models.Assignment) error
	// 13.1.D — tenant scope via parent course's account_id.
	FindByID(ctx context.Context, id, accountID uint) (*models.Assignment, error)
	FindByIDs(ctx context.Context, ids []uint) ([]models.Assignment, error)
	Update(ctx context.Context, assignment *models.Assignment) error
	Delete(ctx context.Context, id uint) error
	ListByCourseID(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.Assignment], error)
}

type AssignmentGroupRepository interface {
	Create(ctx context.Context, group *models.AssignmentGroup) error
	FindByID(ctx context.Context, id uint) (*models.AssignmentGroup, error)
	Update(ctx context.Context, group *models.AssignmentGroup) error
	Delete(ctx context.Context, id uint) error
	ListByCourseID(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.AssignmentGroup], error)
}

type SubmissionRepository interface {
	Create(ctx context.Context, submission *models.Submission) error
	// 13.1.D — tenant scope via parent assignment->course. 0 means no tenant scope (internal callers only).
	FindByID(ctx context.Context, id, accountID uint) (*models.Submission, error)
	FindByIDs(ctx context.Context, ids []uint) ([]models.Submission, error)
	// 13.x.2.1 — tenant-scoped via parent assignment->course->account_id.
	// 0 means no tenant scope (internal callers only).
	FindByAssignmentAndUser(ctx context.Context, assignmentID, userID, accountID uint) (*models.Submission, error)
	FindByAssignmentAndUserIDs(ctx context.Context, assignmentID uint, userIDs []uint) ([]models.Submission, error)
	// ListByUserAndAssignmentIDs is the snapshot loader's targeted read:
	// pulls one user's submissions for a small set of assignments at once,
	// avoiding the N round-trips a per-assignment loop would cost.
	ListByUserAndAssignmentIDs(ctx context.Context, userID uint, assignmentIDs []uint) ([]models.Submission, error)
	Update(ctx context.Context, submission *models.Submission) error
	ListByAssignmentID(ctx context.Context, assignmentID uint, params PaginationParams) (*PaginatedResult[models.Submission], error)
	ListByUserAndCourse(ctx context.Context, userID, courseID uint) ([]models.Submission, error)
	BulkListByCourse(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.Submission], error)
	PostGradesByAssignment(ctx context.Context, assignmentID uint, postedAt *time.Time) error
	RunInTransaction(ctx context.Context, fn func(txRepo SubmissionRepository) error) error
}

type SubmissionCommentRepository interface {
	Create(ctx context.Context, comment *models.SubmissionComment) error
	// 13.1.D — tenant scope via submission->assignment->course. 0 means no tenant scope (internal callers only).
	ListBySubmissionID(ctx context.Context, submissionID, accountID uint) ([]models.SubmissionComment, error)
}

type GradingStandardRepository interface {
	Create(ctx context.Context, standard *models.GradingStandard) error
	FindByID(ctx context.Context, id uint) (*models.GradingStandard, error)
	Update(ctx context.Context, standard *models.GradingStandard) error
	Delete(ctx context.Context, id uint) error
	ListByCourse(ctx context.Context, courseID uint) ([]models.GradingStandard, error)
	FindActiveByCourse(ctx context.Context, courseID uint) (*models.GradingStandard, error)
}

type DeveloperKeyRepository interface {
	Create(ctx context.Context, key *models.DeveloperKey) error
	FindByID(ctx context.Context, id uint) (*models.DeveloperKey, error)
	FindByClientID(ctx context.Context, clientID string) (*models.DeveloperKey, error)
	Update(ctx context.Context, key *models.DeveloperKey) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, accountID uint, params PaginationParams) (*PaginatedResult[models.DeveloperKey], error)
}

type AccessTokenRepository interface {
	Create(ctx context.Context, token *models.AccessToken) error
	FindByID(ctx context.Context, id uint) (*models.AccessToken, error)
	FindByToken(ctx context.Context, tokenHash string) (*models.AccessToken, error)
	FindByRefreshToken(ctx context.Context, refreshToken string) (*models.AccessToken, error)
	Update(ctx context.Context, token *models.AccessToken) error
	Delete(ctx context.Context, id uint) error
	ListByUserID(ctx context.Context, userID uint, params PaginationParams) (*PaginatedResult[models.AccessToken], error)
	DeleteExpired(ctx context.Context) error
}

type LTIToolConfigurationRepository interface {
	Create(ctx context.Context, config *models.LTIToolConfiguration) error
	FindByID(ctx context.Context, id uint) (*models.LTIToolConfiguration, error)
	FindByDeveloperKeyID(ctx context.Context, devKeyID uint) (*models.LTIToolConfiguration, error)
	Update(ctx context.Context, config *models.LTIToolConfiguration) error
	Delete(ctx context.Context, id uint) error
}

type ContextExternalToolRepository interface {
	Create(ctx context.Context, tool *models.ContextExternalTool) error
	// FindByID — 13.1.D: context-polymorphic tenant scope.
	// context_type='Course' → JOIN courses; context_type='Account' → direct.
	FindByID(ctx context.Context, id, accountID uint) (*models.ContextExternalTool, error)
	Update(ctx context.Context, tool *models.ContextExternalTool) error
	Delete(ctx context.Context, id uint) error
	ListByContext(ctx context.Context, contextType string, contextID uint, params PaginationParams) (*PaginatedResult[models.ContextExternalTool], error)
}

type LTIResourceLinkRepository interface {
	Create(ctx context.Context, link *models.LTIResourceLink) error
	FindByID(ctx context.Context, id uint) (*models.LTIResourceLink, error)
	FindByResourceLinkID(ctx context.Context, resourceLinkID string) (*models.LTIResourceLink, error)
	Delete(ctx context.Context, id uint) error
}

type LTILineItemRepository interface {
	Create(ctx context.Context, item *models.LTILineItem) error
	FindByID(ctx context.Context, id uint) (*models.LTILineItem, error)
	Update(ctx context.Context, item *models.LTILineItem) error
	Delete(ctx context.Context, id uint) error
	ListByCourse(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.LTILineItem], error)
	FindByAssignmentID(ctx context.Context, assignmentID uint) (*models.LTILineItem, error)
}

type LTIResultRepository interface {
	Create(ctx context.Context, result *models.LTIResult) error
	FindByID(ctx context.Context, id uint) (*models.LTIResult, error)
	Upsert(ctx context.Context, result *models.LTIResult) error // Create or update by line_item_id + user_id
	ListByLineItem(ctx context.Context, lineItemID uint, params PaginationParams) (*PaginatedResult[models.LTIResult], error)
}

type NonceRepository interface {
	Create(ctx context.Context, nonce *models.Nonce) error
	Exists(ctx context.Context, value string) (bool, error)
	DeleteExpired(ctx context.Context) error
}

// Discussions

type DiscussionTopicRepository interface {
	Create(ctx context.Context, topic *models.DiscussionTopic) error
	// FindByID — 13.1.D: tenant-scoped via the parent course's account_id.
	// accountID==0 means "no scope" and is permitted only from internal
	// callers that have already validated tenant ownership upstream (e.g.
	// background workers, service-internal hops). Handler-layer callers
	// MUST pass the caller's account_id.
	FindByID(ctx context.Context, id, accountID uint) (*models.DiscussionTopic, error)
	Update(ctx context.Context, topic *models.DiscussionTopic) error
	Delete(ctx context.Context, id uint) error
	ListByCourseID(ctx context.Context, courseID, accountID uint, params PaginationParams) (*PaginatedResult[models.DiscussionTopic], error)
}

type DiscussionEntryRepository interface {
	Create(ctx context.Context, entry *models.DiscussionEntry) error
	// FindByID — 13.1.D: tenant-scoped via the entry → topic → course
	// chain. accountID==0 means "no scope" (internal callers only).
	FindByID(ctx context.Context, id, accountID uint) (*models.DiscussionEntry, error)
	Update(ctx context.Context, entry *models.DiscussionEntry) error
	Delete(ctx context.Context, id uint) error
	ListByTopicID(ctx context.Context, topicID, accountID uint, params PaginationParams) (*PaginatedResult[models.DiscussionEntry], error)
	ListReplies(ctx context.Context, entryID, accountID uint, params PaginationParams) (*PaginatedResult[models.DiscussionEntry], error)
	ListAllByTopicID(ctx context.Context, topicID, accountID uint) ([]models.DiscussionEntry, error)
}

type DiscussionEntryRatingRepository interface {
	Upsert(ctx context.Context, rating *models.DiscussionEntryRating) error
	Delete(ctx context.Context, entryID uint, userID uint) error
}

// Files

type FolderRepository interface {
	Create(ctx context.Context, folder *models.Folder) error
	// 13.1.D — tenant scope via polymorphic context_type/context_id.
	// accountID==0 means "no scope" (background jobs, IMSCC import).
	FindByID(ctx context.Context, id, accountID uint) (*models.Folder, error)
	Update(ctx context.Context, folder *models.Folder) error
	Delete(ctx context.Context, id uint) error
	ListByContext(ctx context.Context, contextType string, contextID uint, parentFolderID *uint, params PaginationParams) (*PaginatedResult[models.Folder], error)
	FindRootFolder(ctx context.Context, contextType string, contextID uint) (*models.Folder, error)
}

type AttachmentRepository interface {
	Create(ctx context.Context, attachment *models.Attachment) error
	// 13.1.D — tenant scope via parent folder's context (inherit-via-parent).
	FindByID(ctx context.Context, id, accountID uint) (*models.Attachment, error)
	Update(ctx context.Context, attachment *models.Attachment) error
	Delete(ctx context.Context, id uint) error
	ListByContext(ctx context.Context, contextType string, contextID uint, params PaginationParams) (*PaginatedResult[models.Attachment], error)
	ListByFolderID(ctx context.Context, folderID uint, params PaginationParams) (*PaginatedResult[models.Attachment], error)
}

// SIS Import/Export

type SISBatchRepository interface {
	Create(ctx context.Context, batch *models.SISBatch) error
	FindByID(ctx context.Context, id uint) (*models.SISBatch, error)
	Update(ctx context.Context, batch *models.SISBatch) error
	ListByAccountID(ctx context.Context, accountID uint, params PaginationParams) (*PaginatedResult[models.SISBatch], error)
}

type SISBatchErrorRepository interface {
	Create(ctx context.Context, batchError *models.SISBatchError) error
	ListByBatchID(ctx context.Context, batchID uint) ([]models.SISBatchError, error)
}

// Quiz Engine

type QuizRepository interface {
	Create(ctx context.Context, quiz *models.Quiz) error
	// 13.1.D — tenant scope via parent course's account_id.
	FindByID(ctx context.Context, id, accountID uint) (*models.Quiz, error)
	Update(ctx context.Context, quiz *models.Quiz) error
	Delete(ctx context.Context, id uint) error
	ListByCourseID(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.Quiz], error)
}

type QuizQuestionRepository interface {
	Create(ctx context.Context, question *models.QuizQuestion) error
	FindByID(ctx context.Context, id uint) (*models.QuizQuestion, error)
	FindByIDs(ctx context.Context, ids []uint) ([]models.QuizQuestion, error)
	Update(ctx context.Context, question *models.QuizQuestion) error
	Delete(ctx context.Context, id uint) error
	ListByQuizID(ctx context.Context, quizID uint, params PaginationParams) (*PaginatedResult[models.QuizQuestion], error)
	ListByGroupID(ctx context.Context, groupID uint) ([]models.QuizQuestion, error)
}

type QuizSubmissionRepository interface {
	Create(ctx context.Context, submission *models.QuizSubmission) error
	FindByID(ctx context.Context, id uint) (*models.QuizSubmission, error)
	Update(ctx context.Context, submission *models.QuizSubmission) error
	FindByQuizAndUser(ctx context.Context, quizID, userID uint) (*models.QuizSubmission, error)
	// ListByUserAndQuizIDs is the snapshot loader's targeted read for the
	// SubmittedQuiz predicate. Returns the latest attempt per quiz in the
	// supplied set; callers that need attempt history should still use
	// FindByQuizAndUser plus the attempt column.
	ListByUserAndQuizIDs(ctx context.Context, userID uint, quizIDs []uint) ([]models.QuizSubmission, error)
	ListByQuizID(ctx context.Context, quizID uint, params PaginationParams) (*PaginatedResult[models.QuizSubmission], error)
	ListCompletedByQuizID(ctx context.Context, quizID uint) ([]models.QuizSubmission, error)
}

type QuizSubmissionAnswerRepository interface {
	Create(ctx context.Context, answer *models.QuizSubmissionAnswer) error
	BulkCreate(ctx context.Context, answers []models.QuizSubmissionAnswer) error
	FindByID(ctx context.Context, id uint) (*models.QuizSubmissionAnswer, error)
	Update(ctx context.Context, answer *models.QuizSubmissionAnswer) error
	ListBySubmissionID(ctx context.Context, submissionID uint) ([]models.QuizSubmissionAnswer, error)
	FindBySubmissionAndQuestion(ctx context.Context, submissionID, questionID uint) (*models.QuizSubmissionAnswer, error)
	ListBySubmissionIDs(ctx context.Context, submissionIDs []uint) ([]models.QuizSubmissionAnswer, error)
}

// Rubrics

type RubricRepository interface {
	Create(ctx context.Context, rubric *models.Rubric) error
	// 13.1.D — tenant scope via context_type branch: Account → direct
	// account_id match; Course → JOIN through courses.account_id.
	// Rubrics are intentionally cross-course-shareable WITHIN a tenant;
	// an Account-level rubric in tenant A is reachable from any course
	// in tenant A but never from tenant B.
	FindByID(ctx context.Context, id, accountID uint) (*models.Rubric, error)
	Update(ctx context.Context, rubric *models.Rubric) error
	Delete(ctx context.Context, id uint) error
	ListByContext(ctx context.Context, contextType string, contextID, accountID uint, params PaginationParams) (*PaginatedResult[models.Rubric], error)
}

type RubricAssociationRepository interface {
	Create(ctx context.Context, assoc *models.RubricAssociation) error
	FindByID(ctx context.Context, id uint) (*models.RubricAssociation, error)
	Update(ctx context.Context, assoc *models.RubricAssociation) error
	Delete(ctx context.Context, id uint) error
	FindByAssociation(ctx context.Context, associationID uint, associationType string) (*models.RubricAssociation, error)
}

type RubricAssessmentRepository interface {
	Create(ctx context.Context, assessment *models.RubricAssessment) error
	FindByID(ctx context.Context, id uint) (*models.RubricAssessment, error)
	Update(ctx context.Context, assessment *models.RubricAssessment) error
	Delete(ctx context.Context, id uint) error
	FindByUserAndAssociation(ctx context.Context, userID, assessorID, rubricAssocID uint) (*models.RubricAssessment, error)
	ListByAssociationID(ctx context.Context, rubricAssocID uint, params PaginationParams) (*PaginatedResult[models.RubricAssessment], error)
}

// Grading Periods

type GradingPeriodGroupRepository interface {
	Create(ctx context.Context, group *models.GradingPeriodGroup) error
	// 13.1.D — tenant scope via direct account_id column. 0 means no tenant scope (internal callers only).
	FindByID(ctx context.Context, id, accountID uint) (*models.GradingPeriodGroup, error)
	Update(ctx context.Context, group *models.GradingPeriodGroup) error
	Delete(ctx context.Context, id uint) error
	ListByAccountID(ctx context.Context, accountID uint, params PaginationParams) (*PaginatedResult[models.GradingPeriodGroup], error)
}

type GradingPeriodRepository interface {
	Create(ctx context.Context, period *models.GradingPeriod) error
	// 13.1.D — tenant scope via parent grading_period_group's account_id. 0 means no tenant scope (internal callers only).
	FindByID(ctx context.Context, id, accountID uint) (*models.GradingPeriod, error)
	Update(ctx context.Context, period *models.GradingPeriod) error
	Delete(ctx context.Context, id uint) error
	ListByGroupID(ctx context.Context, groupID, accountID uint) ([]models.GradingPeriod, error)
}

// Assignment Overrides

type AssignmentOverrideRepository interface {
	Create(ctx context.Context, override *models.AssignmentOverride) error
	FindByID(ctx context.Context, id uint) (*models.AssignmentOverride, error)
	Update(ctx context.Context, override *models.AssignmentOverride) error
	Delete(ctx context.Context, id uint) error
	ListByAssignmentID(ctx context.Context, assignmentID uint) ([]models.AssignmentOverride, error)
}

type AssignmentOverrideStudentRepository interface {
	Create(ctx context.Context, student *models.AssignmentOverrideStudent) error
	Delete(ctx context.Context, overrideID, userID uint) error
	ListByOverrideID(ctx context.Context, overrideID uint) ([]models.AssignmentOverrideStudent, error)
	ListByUserAndAssignment(ctx context.Context, userID, assignmentID uint) ([]models.AssignmentOverrideStudent, error)
}

// Late Policy

type LatePolicyRepository interface {
	Create(ctx context.Context, policy *models.LatePolicy) error
	FindByCourseID(ctx context.Context, courseID uint) (*models.LatePolicy, error)
	Update(ctx context.Context, policy *models.LatePolicy) error
	Delete(ctx context.Context, courseID uint) error
}

// Calendar

type CalendarEventRepository interface {
	Create(ctx context.Context, event *models.CalendarEvent) error
	// FindByID — 13.1.D: context-polymorphic tenant scope.
	// User/Course/Group/Account context_type each filter through their tenant key.
	FindByID(ctx context.Context, id, accountID uint) (*models.CalendarEvent, error)
	Update(ctx context.Context, event *models.CalendarEvent) error
	Delete(ctx context.Context, id uint) error
	ListByContext(ctx context.Context, contextType string, contextID uint, params PaginationParams) (*PaginatedResult[models.CalendarEvent], error)
	ListByContextAndDateRange(ctx context.Context, contextType string, contextID uint, startAt, endAt time.Time) ([]models.CalendarEvent, error)
}

// Messaging

type ConversationRepository interface {
	Create(ctx context.Context, conversation *models.Conversation) error
	FindByID(ctx context.Context, id uint) (*models.Conversation, error)
	Update(ctx context.Context, conversation *models.Conversation) error
	Delete(ctx context.Context, id uint) error
	ListByUserID(ctx context.Context, userID uint, params PaginationParams) (*PaginatedResult[models.Conversation], error)
}

type ConversationParticipantRepository interface {
	Create(ctx context.Context, participant *models.ConversationParticipant) error
	FindByConversationAndUser(ctx context.Context, conversationID, userID uint) (*models.ConversationParticipant, error)
	Update(ctx context.Context, participant *models.ConversationParticipant) error
	Delete(ctx context.Context, conversationID, userID uint) error
	ListByConversationID(ctx context.Context, conversationID uint) ([]models.ConversationParticipant, error)
	ListByUserID(ctx context.Context, userID uint) ([]models.ConversationParticipant, error)
}

type ConversationMessageRepository interface {
	Create(ctx context.Context, message *models.ConversationMessage) error
	FindByID(ctx context.Context, id uint) (*models.ConversationMessage, error)
	Update(ctx context.Context, message *models.ConversationMessage) error
	Delete(ctx context.Context, id uint) error
	ListByConversationID(ctx context.Context, conversationID uint, params PaginationParams) (*PaginatedResult[models.ConversationMessage], error)
}

// Notifications

type NotificationPreferenceRepository interface {
	Create(ctx context.Context, prefs *models.NotificationPreference) error
	FindByUserID(ctx context.Context, userID uint) (*models.NotificationPreference, error)
	Update(ctx context.Context, prefs *models.NotificationPreference) error
	Delete(ctx context.Context, userID uint) error
}

type NotificationRepository interface {
	Create(ctx context.Context, notification *models.Notification) error
	FindByID(ctx context.Context, id uint) (*models.Notification, error)
	Update(ctx context.Context, notification *models.Notification) error
	Delete(ctx context.Context, id uint) error
	ListByUserID(ctx context.Context, userID uint, params PaginationParams) (*PaginatedResult[models.Notification], error)
	MarkAsRead(ctx context.Context, userID, notificationID uint) error
	MarkAllAsRead(ctx context.Context, userID uint) error
	ListUnreadByUserID(ctx context.Context, userID uint, params PaginationParams) (*PaginatedResult[models.Notification], error)
}

// Content Migration

type ContentMigrationRepository interface {
	Create(ctx context.Context, migration *models.ContentMigration) error
	FindByID(ctx context.Context, id uint) (*models.ContentMigration, error)
	Update(ctx context.Context, migration *models.ContentMigration) error
	Delete(ctx context.Context, id uint) error
	ListByCourseID(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.ContentMigration], error)
}

// Learning Outcomes

type LearningOutcomeGroupRepository interface {
	Create(ctx context.Context, group *models.LearningOutcomeGroup) error
	// 13.1.D — tenant scope via context_type branch (Account direct,
	// Course via parent courses.account_id).
	FindByID(ctx context.Context, id, accountID uint) (*models.LearningOutcomeGroup, error)
	Update(ctx context.Context, group *models.LearningOutcomeGroup) error
	Delete(ctx context.Context, id uint) error
	ListByContext(ctx context.Context, contextType string, contextID, accountID uint, params PaginationParams) (*PaginatedResult[models.LearningOutcomeGroup], error)
	FindRootGroup(ctx context.Context, contextType string, contextID, accountID uint) (*models.LearningOutcomeGroup, error)
}

type LearningOutcomeRepository interface {
	Create(ctx context.Context, outcome *models.LearningOutcome) error
	// 13.1.D — tenant scope. Outcomes at Account level are shareable
	// across every course in the same tenant; the polymorphic branch
	// enforces "Account → direct match, Course → JOIN through courses".
	FindByID(ctx context.Context, id, accountID uint) (*models.LearningOutcome, error)
	Update(ctx context.Context, outcome *models.LearningOutcome) error
	Delete(ctx context.Context, id uint) error
	ListByGroupID(ctx context.Context, groupID, accountID uint, params PaginationParams) (*PaginatedResult[models.LearningOutcome], error)
	ListByContext(ctx context.Context, contextType string, contextID, accountID uint, params PaginationParams) (*PaginatedResult[models.LearningOutcome], error)
}

type LearningOutcomeResultRepository interface {
	Create(ctx context.Context, result *models.LearningOutcomeResult) error
	FindByID(ctx context.Context, id uint) (*models.LearningOutcomeResult, error)
	Update(ctx context.Context, result *models.LearningOutcomeResult) error
	// Upsert writes a result row keyed on
	// (user_id, learning_outcome_id, associated_asset_type, associated_asset_id)
	// and returns the row's Mastery value as it was BEFORE the write.
	// priorMastery is nil if no prior row existed or the prior row's
	// Mastery was nil. The implementation must serialize concurrent
	// writes to the same composite (the postgres impl uses a single
	// transaction with SELECT … FOR UPDATE) so that the
	// LearningOutcomeService.OnMasteryCrossed transition detector can
	// trust the returned value as the atomic pre-write state.
	Upsert(ctx context.Context, result *models.LearningOutcomeResult) (priorMastery *bool, err error)
	ListByOutcomeID(ctx context.Context, outcomeID uint, params PaginationParams) (*PaginatedResult[models.LearningOutcomeResult], error)
	ListByUserAndContext(ctx context.Context, userID uint, contextType string, contextID uint) ([]models.LearningOutcomeResult, error)
	// ListByUserAndOutcomeIDs is the snapshot loader's targeted read for
	// OutcomeMastery predicates. Returns every recorded result for the
	// given outcome set; the mastery package consumes them via its
	// per-method calculators.
	ListByUserAndOutcomeIDs(ctx context.Context, userID uint, outcomeIDs []uint) ([]models.LearningOutcomeResult, error)
}

type OutcomeAlignmentRepository interface {
	Create(ctx context.Context, alignment *models.OutcomeAlignment) error
	Delete(ctx context.Context, id uint) error
	// 13.1.D — accountID, when non-zero, filters alignments to those whose
	// course (or whose assignment's course) belongs to caller's tenant.
	ListByAssignmentID(ctx context.Context, assignmentID, accountID uint) ([]models.OutcomeAlignment, error)
	ListByCourseID(ctx context.Context, courseID, accountID uint) ([]models.OutcomeAlignment, error)
}

// Blueprint Courses

type BlueprintTemplateRepository interface {
	Create(ctx context.Context, template *models.BlueprintTemplate) error
	FindByID(ctx context.Context, id uint) (*models.BlueprintTemplate, error)
	FindByCourseID(ctx context.Context, courseID uint) (*models.BlueprintTemplate, error)
	Update(ctx context.Context, template *models.BlueprintTemplate) error
	Delete(ctx context.Context, id uint) error
	ListByCourseID(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.BlueprintTemplate], error)
}

type BlueprintSubscriptionRepository interface {
	Create(ctx context.Context, subscription *models.BlueprintSubscription) error
	FindByID(ctx context.Context, id uint) (*models.BlueprintSubscription, error)
	FindByTemplateAndChild(ctx context.Context, templateID, childCourseID uint) (*models.BlueprintSubscription, error)
	Update(ctx context.Context, subscription *models.BlueprintSubscription) error
	Delete(ctx context.Context, id uint) error
	ListByTemplateID(ctx context.Context, templateID uint, params PaginationParams) (*PaginatedResult[models.BlueprintSubscription], error)
	ListByChildCourseID(ctx context.Context, childCourseID uint, params PaginationParams) (*PaginatedResult[models.BlueprintSubscription], error)
}

type BlueprintMigrationRepository interface {
	Create(ctx context.Context, migration *models.BlueprintMigration) error
	FindByID(ctx context.Context, id uint) (*models.BlueprintMigration, error)
	Update(ctx context.Context, migration *models.BlueprintMigration) error
	Delete(ctx context.Context, id uint) error
	ListByTemplateID(ctx context.Context, templateID uint, params PaginationParams) (*PaginatedResult[models.BlueprintMigration], error)
	ListBySubscriptionID(ctx context.Context, subscriptionID uint, params PaginationParams) (*PaginatedResult[models.BlueprintMigration], error)
}

// OneRoster

type OneRosterConnectionRepository interface {
	Create(ctx context.Context, conn *models.OneRosterConnection) error
	// 13.1.D — direct account_id column.
	FindByID(ctx context.Context, id, accountID uint) (*models.OneRosterConnection, error)
	Update(ctx context.Context, conn *models.OneRosterConnection) error
	Delete(ctx context.Context, id uint) error
	ListByAccountID(ctx context.Context, accountID uint, params PaginationParams) (*PaginatedResult[models.OneRosterConnection], error)
	FindByAccountAndName(ctx context.Context, accountID uint, name string) (*models.OneRosterConnection, error)
	ListAutoSync(ctx context.Context) ([]models.OneRosterConnection, error)
}

type OneRosterSyncLogRepository interface {
	Create(ctx context.Context, log *models.OneRosterSyncLog) error
	Update(ctx context.Context, log *models.OneRosterSyncLog) error
	ListByConnectionID(ctx context.Context, connectionID uint, params PaginationParams) (*PaginatedResult[models.OneRosterSyncLog], error)
	GetLatestByConnectionID(ctx context.Context, connectionID uint) (*models.OneRosterSyncLog, error)
}

// Document Annotations

type DocumentAnnotationRepository interface {
	Create(ctx context.Context, annotation *models.DocumentAnnotation) error
	// 13.1.D — tenant scope via submission->assignment->course. 0 means no tenant scope (internal callers only).
	FindByID(ctx context.Context, id, accountID uint) (*models.DocumentAnnotation, error)
	Update(ctx context.Context, annotation *models.DocumentAnnotation) error
	Delete(ctx context.Context, id uint) error
	ListBySubmissionID(ctx context.Context, submissionID uint, params PaginationParams) (*PaginatedResult[models.DocumentAnnotation], error)
	ListBySubmissionAndPage(ctx context.Context, submissionID uint, pageNumber int) ([]models.DocumentAnnotation, error)
	CountBySubmissionID(ctx context.Context, submissionID uint) (int64, error)
	ListReplies(ctx context.Context, parentAnnotationID uint) ([]models.DocumentAnnotation, error)
	Resolve(ctx context.Context, annotationID uint, resolvedByUserID uint) error
	Unresolve(ctx context.Context, annotationID uint) error
}

// Portfolio interfaces

type PortfolioRepository interface {
	Create(ctx context.Context, portfolio *models.Portfolio) error
	FindByID(ctx context.Context, id uint) (*models.Portfolio, error)
	FindBySlug(ctx context.Context, slug string) (*models.Portfolio, error)
	FindByPublicURL(ctx context.Context, publicURL string) (*models.Portfolio, error)
	Update(ctx context.Context, portfolio *models.Portfolio) error
	Delete(ctx context.Context, id uint) error
	ListByUserID(ctx context.Context, userID uint, params PaginationParams) (*PaginatedResult[models.Portfolio], error)
	ListPublic(ctx context.Context, params PaginationParams) (*PaginatedResult[models.Portfolio], error)
	IncrementViewCount(ctx context.Context, id uint) error
}

type PortfolioSectionRepository interface {
	Create(ctx context.Context, section *models.PortfolioSection) error
	FindByID(ctx context.Context, id uint) (*models.PortfolioSection, error)
	FindByIDs(ctx context.Context, ids []uint) ([]models.PortfolioSection, error)
	Update(ctx context.Context, section *models.PortfolioSection) error
	Delete(ctx context.Context, id uint) error
	ListByPortfolioID(ctx context.Context, portfolioID uint) ([]models.PortfolioSection, error)
}

type PortfolioArtifactRepository interface {
	Create(ctx context.Context, artifact *models.PortfolioArtifact) error
	FindByID(ctx context.Context, id uint) (*models.PortfolioArtifact, error)
	Update(ctx context.Context, artifact *models.PortfolioArtifact) error
	Delete(ctx context.Context, id uint) error
	ListByPortfolioID(ctx context.Context, portfolioID uint, params PaginationParams) (*PaginatedResult[models.PortfolioArtifact], error)
	ListBySectionID(ctx context.Context, sectionID uint) ([]models.PortfolioArtifact, error)
	ListFeatured(ctx context.Context, portfolioID uint) ([]models.PortfolioArtifact, error)
}

type PortfolioReflectionRepository interface {
	Create(ctx context.Context, reflection *models.PortfolioReflection) error
	FindByID(ctx context.Context, id uint) (*models.PortfolioReflection, error)
	Update(ctx context.Context, reflection *models.PortfolioReflection) error
	ListByArtifactID(ctx context.Context, artifactID uint) ([]models.PortfolioReflection, error)
}

type PortfolioTemplateRepository interface {
	Create(ctx context.Context, template *models.PortfolioTemplate) error
	// 13.1.D — direct account_id column. Note: portfolio templates ARE
	// account-scoped (admin-curated). User portfolios live in
	// PortfolioRepository and stay user-scoped (private, owner-only).
	FindByID(ctx context.Context, id, accountID uint) (*models.PortfolioTemplate, error)
	Update(ctx context.Context, template *models.PortfolioTemplate) error
	ListPublic(ctx context.Context, params PaginationParams) (*PaginatedResult[models.PortfolioTemplate], error)
	ListByAccountID(ctx context.Context, accountID uint, params PaginationParams) (*PaginatedResult[models.PortfolioTemplate], error)
}

type PortfolioCommentRepository interface {
	Create(ctx context.Context, comment *models.PortfolioComment) error
	FindByID(ctx context.Context, id uint) (*models.PortfolioComment, error)
	Update(ctx context.Context, comment *models.PortfolioComment) error
	Delete(ctx context.Context, id uint) error
	ListByPortfolioID(ctx context.Context, portfolioID uint, params PaginationParams) (*PaginatedResult[models.PortfolioComment], error)
	ListByArtifactID(ctx context.Context, artifactID uint, params PaginationParams) (*PaginatedResult[models.PortfolioComment], error)
}

// Course Home Engine

type CourseHomeButtonRepository interface {
	Create(ctx context.Context, button *models.CourseHomeButton) error
	FindByID(ctx context.Context, id uint) (*models.CourseHomeButton, error)
	Update(ctx context.Context, button *models.CourseHomeButton) error
	Delete(ctx context.Context, id uint) error
	ListByCourseID(ctx context.Context, courseID uint) ([]models.CourseHomeButton, error)
	BulkUpdatePositions(ctx context.Context, courseID uint, positions map[uint]int) error
}

type TodaysLessonOverrideRepository interface {
	Create(ctx context.Context, override *models.TodaysLessonOverride) error
	FindByID(ctx context.Context, id uint) (*models.TodaysLessonOverride, error)
	Update(ctx context.Context, override *models.TodaysLessonOverride) error
	Delete(ctx context.Context, id uint) error
	FindByCourseAndDate(ctx context.Context, courseID uint, date time.Time) (*models.TodaysLessonOverride, error)
	ListByCourseID(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.TodaysLessonOverride], error)
}

type CourseVisitRepository interface {
	Upsert(ctx context.Context, visit *models.CourseVisit) error
	FindByUserAndCourse(ctx context.Context, userID, courseID uint) (*models.CourseVisit, error)
}

// Peer Reviews

type PeerReviewRepository interface {
	Create(ctx context.Context, pr *models.PeerReview) error
	FindByID(ctx context.Context, id uint) (*models.PeerReview, error)
	Update(ctx context.Context, pr *models.PeerReview) error
	ListByAssignment(ctx context.Context, assignmentID uint) ([]models.PeerReview, error)
	ListByReviewer(ctx context.Context, assignmentID, reviewerID uint) ([]models.PeerReview, error)
	FindByAssignmentAndReviewerAndReviewee(ctx context.Context, assignmentID, reviewerID, revieweeID uint) (*models.PeerReview, error)
	DeleteByAssignment(ctx context.Context, assignmentID uint) error
}

// Question Banks

type QuestionBankRepository interface {
	Create(ctx context.Context, qb *models.QuestionBank) error
	FindByID(ctx context.Context, id uint) (*models.QuestionBank, error)
	Update(ctx context.Context, qb *models.QuestionBank) error
	Delete(ctx context.Context, id uint) error
	ListByCourse(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.QuestionBank], error)
}

type QuestionBankEntryRepository interface {
	Create(ctx context.Context, entry *models.QuestionBankEntry) error
	FindByID(ctx context.Context, id uint) (*models.QuestionBankEntry, error)
	Update(ctx context.Context, entry *models.QuestionBankEntry) error
	Delete(ctx context.Context, id uint) error
	ListByBankID(ctx context.Context, bankID uint) ([]models.QuestionBankEntry, error)
}

// Quiz Question Groups

type QuizQuestionGroupRepository interface {
	Create(ctx context.Context, group *models.QuizQuestionGroup) error
	FindByID(ctx context.Context, id uint) (*models.QuizQuestionGroup, error)
	Update(ctx context.Context, group *models.QuizQuestionGroup) error
	Delete(ctx context.Context, id uint) error
	ListByQuizID(ctx context.Context, quizID uint) ([]models.QuizQuestionGroup, error)
}

// Module Prerequisites

type ModulePrerequisiteRepository interface {
	SetPrerequisites(ctx context.Context, moduleID uint, prerequisiteModuleIDs []uint) error
	GetPrerequisites(ctx context.Context, moduleID uint) ([]uint, error)
	GetModulesWithPrerequisite(ctx context.Context, prerequisiteModuleID uint) ([]uint, error)
}

// Comment Bank Items

type CommentBankItemRepository interface {
	Create(ctx context.Context, item *models.CommentBankItem) error
	FindByID(ctx context.Context, id uint) (*models.CommentBankItem, error)
	Update(ctx context.Context, item *models.CommentBankItem) error
	Delete(ctx context.Context, id uint) error
	ListByUserID(ctx context.Context, userID uint, params PaginationParams) (*PaginatedResult[models.CommentBankItem], error)
	SearchByUser(ctx context.Context, userID uint, query string) ([]models.CommentBankItem, error)
}

// Planner

type PlannerNoteRepository interface {
	Create(ctx context.Context, note *models.PlannerNote) error
	FindByID(ctx context.Context, id uint) (*models.PlannerNote, error)
	Update(ctx context.Context, note *models.PlannerNote) error
	Delete(ctx context.Context, id uint) error
	ListByUserID(ctx context.Context, userID uint, params PaginationParams) (*PaginatedResult[models.PlannerNote], error)
}

type PlannerOverrideRepository interface {
	Create(ctx context.Context, override *models.PlannerOverride) error
	FindByID(ctx context.Context, id uint) (*models.PlannerOverride, error)
	Update(ctx context.Context, override *models.PlannerOverride) error
	Delete(ctx context.Context, id uint) error
	FindByUserAndPlannable(ctx context.Context, userID uint, plannableType string, plannableID uint) (*models.PlannerOverride, error)
	ListByUserID(ctx context.Context, userID uint) ([]models.PlannerOverride, error)
}

// Wave A2: Quiz Item Banks, Stimuli, Per-Question Outcome Alignment

type QuizItemBankRepository interface {
	Create(ctx context.Context, bank *models.QuizItemBank) error
	FindByID(ctx context.Context, id uint) (*models.QuizItemBank, error)
	Update(ctx context.Context, bank *models.QuizItemBank) error
	Delete(ctx context.Context, id uint) error
	ListByCourseID(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.QuizItemBank], error)
}

type QuizItemBankItemRepository interface {
	Create(ctx context.Context, item *models.QuizItemBankItem) error
	FindByID(ctx context.Context, id uint) (*models.QuizItemBankItem, error)
	Update(ctx context.Context, item *models.QuizItemBankItem) error
	Delete(ctx context.Context, id uint) error
	ListByBankID(ctx context.Context, bankID uint) ([]models.QuizItemBankItem, error)
}

type QuizStimulusRepository interface {
	Create(ctx context.Context, stimulus *models.QuizStimulus) error
	FindByID(ctx context.Context, id uint) (*models.QuizStimulus, error)
	Update(ctx context.Context, stimulus *models.QuizStimulus) error
	Delete(ctx context.Context, id uint) error
	ListByCourseID(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.QuizStimulus], error)
	ListQuestionsForStimulus(ctx context.Context, stimulusID uint) ([]models.QuizQuestion, error)
	SetQuestionStimulus(ctx context.Context, questionID uint, stimulusID *uint) error
}

type QuizQuestionOutcomeAlignmentRepository interface {
	Create(ctx context.Context, alignment *models.QuizQuestionOutcomeAlignment) error
	Delete(ctx context.Context, id uint) error
	DeleteByQuestionAndOutcome(ctx context.Context, quizQuestionID, outcomeID uint) error
	FindByQuestionAndOutcome(ctx context.Context, quizQuestionID, outcomeID uint) (*models.QuizQuestionOutcomeAlignment, error)
	ListByQuestionID(ctx context.Context, quizQuestionID uint) ([]models.QuizQuestionOutcomeAlignment, error)
	ListByOutcomeID(ctx context.Context, outcomeID uint) ([]models.QuizQuestionOutcomeAlignment, error)
}

// Wiki Page Revisions

type WikiPageRevisionRepository interface {
	Create(ctx context.Context, revision *models.WikiPageRevision) error
	FindByID(ctx context.Context, id uint) (*models.WikiPageRevision, error)
	ListByPageID(ctx context.Context, pageID uint, params PaginationParams) (*PaginatedResult[models.WikiPageRevision], error)
	GetLatestRevision(ctx context.Context, pageID uint) (*models.WikiPageRevision, error)
	GetRevisionByNumber(ctx context.Context, pageID uint, revisionNumber int) (*models.WikiPageRevision, error)
}

// Phase 6 Wave 1: gamification foundations.
// See docs/research/gamification-2026-05/PHASE6-WAVE1-PLAN.md.

// GamificationEventFilter narrows queries against the xAPI event store.
// Empty fields are ignored; multiple fields AND together.
type GamificationEventFilter struct {
	TenantID     *uint
	ActorID      *uint
	Verb         string
	ObjectType   string
	ObjectID     *uint
	OccurredFrom *time.Time
	OccurredTo   *time.Time
}

type GamificationEventRepository interface {
	Create(ctx context.Context, event *models.GamificationEvent) error
	FindByID(ctx context.Context, id uint) (*models.GamificationEvent, error)
	// FindBySourceEventID supports idempotent ingest of external systems:
	// re-deliveries of the same (source, source_event_id) pair return the
	// original row rather than inserting a duplicate.
	FindBySourceEventID(ctx context.Context, source, sourceEventID string) (*models.GamificationEvent, error)
	List(ctx context.Context, filter GamificationEventFilter, params PaginationParams) (*PaginatedResult[models.GamificationEvent], error)
}

type GamificationRuleRepository interface {
	Create(ctx context.Context, rule *models.GamificationRule) error
	FindByID(ctx context.Context, id uint) (*models.GamificationRule, error)
	Update(ctx context.Context, rule *models.GamificationRule) error
	Delete(ctx context.Context, id uint) error
	// ListEnabledByScope returns enabled rules at the exact (scope_type, scope_id).
	// The dispatch loop (Wave 1 task 10) walks up the org tree itself.
	ListEnabledByScope(ctx context.Context, scopeType models.GamificationScopeType, scopeID uint) ([]models.GamificationRule, error)
	ListByTenantID(ctx context.Context, tenantID uint, params PaginationParams) (*PaginatedResult[models.GamificationRule], error)
	// ListByScope returns every rule (enabled OR disabled) at a precise
	// (tenant, scope_type, scope_id) tuple. Backs the W2-E.1 recipe
	// builder list view — admin sees site rules, instructor sees their
	// own course/section rules, neither sees the other's slice.
	ListByScope(ctx context.Context, tenantID uint, scopeType models.GamificationScopeType, scopeID uint, params PaginationParams) (*PaginatedResult[models.GamificationRule], error)

	// RecordEvaluation appends an audit row. The (rule_id, user_id, evaluated_at)
	// tuple is uniquely indexed; a same-microsecond duplicate is a bug, not a retry.
	RecordEvaluation(ctx context.Context, eval *models.GamificationRuleEvaluation) error
	ListEvaluationsForUserRule(ctx context.Context, userID, ruleID uint, params PaginationParams) (*PaginatedResult[models.GamificationRuleEvaluation], error)
	// LastFiringForUserRule returns the most recent successful evaluation
	// (result=true) for (rule_id, user_id) — the cooldown check's input.
	// Returns (nil, nil) when the rule has never successfully fired for
	// this user.
	LastFiringForUserRule(ctx context.Context, userID, ruleID uint) (*models.GamificationRuleEvaluation, error)
	// CountFiringsInWindow counts successful evaluations for
	// (rule_id, user_id) since `since`. Powers the max_per_window guard.
	CountFiringsInWindow(ctx context.Context, userID, ruleID uint, since time.Time) (int64, error)
}

// ContentViewRepository persists per-user content-view aggregates that the
// ViewedContent predicate reads at rule-evaluation time. Schema lives at
// migration 000036.
type ContentViewRepository interface {
	// IncrementView upserts the (user, object_type, object_id) row,
	// incrementing view_count and total_seconds and bumping
	// last_viewed_at. Atomic via ON CONFLICT … DO UPDATE.
	IncrementView(ctx context.Context, userID uint, objectType string, objectID uint, durationSeconds int64) error
	// ListByUserAndObjectIDs is the snapshot loader's targeted read.
	ListByUserAndObjectIDs(ctx context.Context, userID uint, objectType string, objectIDs []uint) ([]models.ContentView, error)
	// GetByUserAndObject returns (nil, nil) when no row exists; callers
	// treat that as zero views.
	GetByUserAndObject(ctx context.Context, userID uint, objectType string, objectID uint) (*models.ContentView, error)
}

type GamificationCurrencyTypeRepository interface {
	Create(ctx context.Context, currency *models.GamificationCurrencyType) error
	FindByID(ctx context.Context, id uint) (*models.GamificationCurrencyType, error)
	// FindByCode exact-matches (tenant_id, scope_type, scope_id, code).
	// The resolution-order walk (section → course → school → district → site)
	// is the caller's job; this is the single-lookup primitive.
	FindByCode(ctx context.Context, tenantID uint, scopeType models.GamificationScopeType, scopeID uint, code string) (*models.GamificationCurrencyType, error)
	Update(ctx context.Context, currency *models.GamificationCurrencyType) error
	Delete(ctx context.Context, id uint) error
	ListByTenant(ctx context.Context, tenantID uint) ([]models.GamificationCurrencyType, error)
	ListInTopbar(ctx context.Context, tenantID uint) ([]models.GamificationCurrencyType, error)
}

// GamificationBadgeRepository persists admin/instructor-authored badge
// definitions. Create returns ErrBadgeDuplicate when the
// (tenant_id, scope_type, scope_id, code) tuple is already taken — the
// translation is atomic at the SQL layer (INSERT ... ON CONFLICT
// DO NOTHING RETURNING).
type GamificationBadgeRepository interface {
	Create(ctx context.Context, badge *models.GamificationBadge) error
	FindByID(ctx context.Context, id uint) (*models.GamificationBadge, error)
	FindByCode(ctx context.Context, tenantID uint, scopeType models.GamificationScopeType, scopeID uint, code string) (*models.GamificationBadge, error)
	Update(ctx context.Context, badge *models.GamificationBadge) error
	Delete(ctx context.Context, id uint) error
	ListByTenant(ctx context.Context, tenantID uint) ([]models.GamificationBadge, error)
}

// GamificationBadgeAwardRepository persists (user, badge) issuances.
// Award is idempotent (atomic via the uniq_gam_badge_award constraint);
// double-awarding the same badge to the same user is a no-op.
type GamificationBadgeAwardRepository interface {
	// Award inserts a (user, badge) row. If the user already holds the
	// badge, the call is a no-op (no error, no duplicate row, no update
	// to AwardedAt). The bool return tells the caller whether a new
	// award actually happened — useful for any future "first time only"
	// emit hook.
	Award(ctx context.Context, award *models.GamificationBadgeAward) (created bool, err error)
	Revoke(ctx context.Context, userID, badgeID uint) error
	ListForUser(ctx context.Context, userID uint) ([]models.GamificationBadgeAward, error)
	FindByUserAndBadge(ctx context.Context, userID, badgeID uint) (*models.GamificationBadgeAward, error)
}

type GamificationWalletRepository interface {
	// GetBalance returns nil (no error) when the (user, currency) pair has
	// never transacted. Callers treat that as a zero balance.
	GetBalance(ctx context.Context, userID, currencyTypeID uint) (*models.GamificationWalletBalance, error)
	ListBalancesForUser(ctx context.Context, userID uint) ([]models.GamificationWalletBalance, error)
	// ApplyTransaction is the single atomic mutation primitive: appends a
	// transaction row and updates the corresponding balance row in one DB
	// transaction. The Wave 1 task-8 AwardCurrency effect calls this.
	ApplyTransaction(ctx context.Context, tx *models.GamificationWalletTransaction) error
	ListTransactionsForUser(ctx context.Context, userID uint, params PaginationParams) (*PaginatedResult[models.GamificationWalletTransaction], error)
	// ListTransactionsForUserAndCurrency narrows the ledger to a single
	// currency. Powers the wallet drawer's per-currency tab in Wave 2 —
	// avoids over-fetching when a user has years of cross-currency
	// transactions.
	ListTransactionsForUserAndCurrency(ctx context.Context, userID, currencyTypeID uint, params PaginationParams) (*PaginatedResult[models.GamificationWalletTransaction], error)
	// RankByCurrency (W3-A) returns candidateUserIDs ranked by
	// lifetime_earned DESC for a single currency. Ties resolved by
	// earliest most-recent positive transaction (the earlier-completer
	// ranks higher; doesn't reward sandbagging). Rows with no balance
	// row for this currency surface with lifetime_earned = 0 and rank
	// at the tail.
	//
	// Composition note: callers MUST narrow candidateUserIDs through
	// UserRepository.FilterPublicLeaderboardCandidates first. Opt-out
	// privacy lives in the user repo; this method is rank-only.
	RankByCurrency(ctx context.Context, currencyTypeID uint, candidateUserIDs []uint) ([]RankRow, error)
}

// RankRow is the wallet-repo-level rank tuple. Rank starts at 1.
// LifetimeEarned == 0 for candidates with no balance row in this currency.
type RankRow struct {
	UserID         uint
	LifetimeEarned int64
	Rank           int
}

type GamificationFerpaFieldTagRepository interface {
	Upsert(ctx context.Context, tag *models.GamificationFerpaFieldTag) error
	Find(ctx context.Context, objectType, fieldPath string) (*models.GamificationFerpaFieldTag, error)
	ListByObjectType(ctx context.Context, objectType string) ([]models.GamificationFerpaFieldTag, error)
}

// UserRecoveryCodeRepository (Phase 9-B) persists single-use TOTP
// recovery codes. Generated in bulk at MFA enrollment; one row
// marked used per successful recovery-code login.
type UserRecoveryCodeRepository interface {
	CreateBatch(ctx context.Context, userID uint, codeHashes []string) error
	ListUnusedForUser(ctx context.Context, userID uint) ([]models.UserRecoveryCode, error)
	MarkUsed(ctx context.Context, id uint) error
	DeleteAllForUser(ctx context.Context, userID uint) error
}

// UserWebauthnCredentialRepository (Phase 10-B) persists registered
// passkey credentials. Lookups happen on (a) credential_id for the
// assertion path and (b) user_id for the management UI. The
// assertion path also bumps SignCount + LastUsedAt on every login.
type UserWebauthnCredentialRepository interface {
	Create(ctx context.Context, cred *models.UserWebauthnCredential) error
	FindByCredentialID(ctx context.Context, credentialID []byte) (*models.UserWebauthnCredential, error)
	FindByID(ctx context.Context, id uint) (*models.UserWebauthnCredential, error)
	ListForUser(ctx context.Context, userID uint) ([]models.UserWebauthnCredential, error)
	// UpdateSignCount bumps sign_count and last_used_at after a
	// successful assertion. Replay-counter regression is the
	// library's concern, not the repo's — callers pass the verified
	// new counter through.
	UpdateSignCount(ctx context.Context, id uint, newSignCount uint32) error
	UpdateNickname(ctx context.Context, id, userID uint, nickname string) error
	Delete(ctx context.Context, id, userID uint) error
}

// FederatedIdentityRepository (Phase 9-PRE) anchors external IdP
// subjects to local user rows. Every federation handler (SAML, LDAP,
// CAS, OIDC, future WebAuthn) writes through this surface; the
// LoginPipeline reads it first when resolving an SSOOutcome to a user.
//
// Idempotent Create: re-authenticating with the same (provider,
// subject) updates last_seen_at but doesn't create a duplicate. The
// UNIQUE constraint on (provider_id, external_subject) gates it.
type FederatedIdentityRepository interface {
	// FindByProviderAndSubject returns the existing federation row or
	// (nil, nil) when no binding exists. Callers fall back to email
	// auto-link or JIT provisioning.
	FindByProviderAndSubject(ctx context.Context, providerID uint, externalSubject string) (*models.FederatedIdentity, error)
	// Create writes a fresh (user, provider, subject) binding. Caller
	// has already resolved or created the user_id.
	Create(ctx context.Context, fi *models.FederatedIdentity) error
	// TouchLastSeen bumps the last_seen_at timestamp + optionally
	// refreshes the claims_snapshot when the IdP sent richer data
	// than what was captured at first-login.
	TouchLastSeen(ctx context.Context, id uint, claimsSnapshot []byte) error
	// ListForUser is the "manage your linked accounts" view a user
	// sees in settings.
	ListForUser(ctx context.Context, userID uint) ([]models.FederatedIdentity, error)
}

// GamificationLeaderboardSnapshotRepository persists ranked-window
// snapshots (Sprint 7-B). Writes are idempotent via ON CONFLICT DO
// NOTHING on the (scope, currency, window_kind, window_end) UNIQUE
// constraint — the CLI can be re-run for the same window without
// duplicating rows.
type GamificationLeaderboardSnapshotRepository interface {
	// Upsert inserts the snapshot row, no-op on conflict. Returns
	// `created=true` only when a new row was actually written so the
	// CLI can log per-window outcomes accurately.
	Upsert(ctx context.Context, snap *models.GamificationLeaderboardSnapshot) (created bool, err error)
	// FindByWindow returns the snapshot for the exact (scope,
	// currency, kind, end) tuple, or nil if no snapshot exists for
	// that window. The handler uses this to serve `?offset_weeks=N`
	// reads; nil triggers a 404 at the handler.
	FindByWindow(ctx context.Context, scopeType models.GamificationScopeType, scopeID, currencyTypeID uint, kind string, windowEnd time.Time) (*models.GamificationLeaderboardSnapshot, error)
}
