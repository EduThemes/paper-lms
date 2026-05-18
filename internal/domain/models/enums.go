package models

// This file declares typed string aliases for the `workflow_state` column on
// the highest-traffic models. Each alias replaces what was previously a bare
// `string` field plus a free-floating documentation comment listing the
// valid values. Promoting the value space into the type system gives Go-side
// callers compile-time assistance and makes drift visible at code review.
//
// The aliases are still string-typed under the hood — GORM serializes them
// the same way as `string`, and `db.Where("workflow_state = ?", "active")`
// continues to work. The `type:text` GORM tag on each adoption site mirrors
// the convention established by `GamificationScopeType` (see
// `gamification_rule.go` and `gamification_currency_type.go`): the tag tells
// AutoMigrate "this is plain TEXT" so the parity test doesn't try to
// reconcile a Postgres enum that doesn't exist.
//
// Scope: the top-10 high-traffic models. 49 other models still carry
// `WorkflowState string` and can adopt the same pattern incrementally
// without changing SQL or migrations.

// AssignmentWorkflow tracks publication state of an Assignment row.
// Values: AssignmentUnpublished | AssignmentPublished | AssignmentDeleted.
type AssignmentWorkflow string

const (
	AssignmentUnpublished AssignmentWorkflow = "unpublished"
	AssignmentPublished   AssignmentWorkflow = "published"
	AssignmentDeleted     AssignmentWorkflow = "deleted"
)

// SubmissionWorkflow tracks the lifecycle of a student Submission.
// Values: SubmissionSubmitted | SubmissionUnsubmitted | SubmissionGraded |
// SubmissionPendingReview.
type SubmissionWorkflow string

const (
	SubmissionSubmitted     SubmissionWorkflow = "submitted"
	SubmissionUnsubmitted   SubmissionWorkflow = "unsubmitted"
	SubmissionGraded        SubmissionWorkflow = "graded"
	SubmissionPendingReview SubmissionWorkflow = "pending_review"
)

// EnrollmentWorkflow tracks the lifecycle of an Enrollment row.
// Values: EnrollmentActive | EnrollmentInvited | EnrollmentCompleted |
// EnrollmentRejected | EnrollmentDeleted | EnrollmentInactive.
type EnrollmentWorkflow string

const (
	EnrollmentActive    EnrollmentWorkflow = "active"
	EnrollmentInvited   EnrollmentWorkflow = "invited"
	EnrollmentCompleted EnrollmentWorkflow = "completed"
	EnrollmentRejected  EnrollmentWorkflow = "rejected"
	EnrollmentDeleted   EnrollmentWorkflow = "deleted"
	EnrollmentInactive  EnrollmentWorkflow = "inactive"
)

// CourseWorkflow tracks the lifecycle of a Course row.
// Values: CourseClaimed | CourseCreated | CourseUnpublished |
// CourseAvailable | CourseCompleted | CourseDeleted.
//
// `unpublished` is the row default established by migration 000001 and is
// the state a freshly-imported copy lands in (see
// internal/service/batch_service.go::CopyCourse).
type CourseWorkflow string

const (
	CourseClaimed     CourseWorkflow = "claimed"
	CourseCreated     CourseWorkflow = "created"
	CourseUnpublished CourseWorkflow = "unpublished"
	CourseAvailable   CourseWorkflow = "available"
	CourseCompleted   CourseWorkflow = "completed"
	CourseDeleted     CourseWorkflow = "deleted"
)

// AccountWorkflow tracks the lifecycle of an Account row.
// Values: AccountActive | AccountDeleted.
type AccountWorkflow string

const (
	AccountActive  AccountWorkflow = "active"
	AccountDeleted AccountWorkflow = "deleted"
)

// DiscussionTopicWorkflow tracks the lifecycle of a DiscussionTopic row.
// Values: DiscussionTopicUnpublished | DiscussionTopicActive |
// DiscussionTopicDeleted | DiscussionTopicPostDelayed | DiscussionTopicLocked.
//
// `unpublished` is the state a freshly-cloned topic lands in
// (see internal/service/batch_service.go::cloneDiscussions); imscc imports
// can also flip a topic to `unpublished` if the source marked it so.
type DiscussionTopicWorkflow string

const (
	DiscussionTopicUnpublished DiscussionTopicWorkflow = "unpublished"
	DiscussionTopicActive      DiscussionTopicWorkflow = "active"
	DiscussionTopicDeleted     DiscussionTopicWorkflow = "deleted"
	DiscussionTopicPostDelayed DiscussionTopicWorkflow = "post_delayed"
	DiscussionTopicLocked      DiscussionTopicWorkflow = "locked"
)

// AttachmentWorkflow tracks the lifecycle of an Attachment row.
// Values: AttachmentPendingUpload | AttachmentProcessing | AttachmentProcessed |
// AttachmentBroken | AttachmentDeleted.
//
// Note: Attachment also carries `file_state` and `upload_status` columns
// for finer-grained status; `WorkflowState` is the row-level lifecycle.
type AttachmentWorkflow string

const (
	AttachmentPendingUpload AttachmentWorkflow = "pending_upload"
	AttachmentProcessing    AttachmentWorkflow = "processing"
	AttachmentProcessed     AttachmentWorkflow = "processed"
	AttachmentBroken        AttachmentWorkflow = "broken"
	AttachmentDeleted       AttachmentWorkflow = "deleted"
)

// ContentMigrationWorkflow tracks the lifecycle of a ContentMigration job.
// Values: ContentMigrationQueued | ContentMigrationCreated |
// ContentMigrationPreProcessing | ContentMigrationRunning |
// ContentMigrationCompleted | ContentMigrationFailed.
//
// `created` is the default after the row lands but before the worker picks
// it up; `queued` is the worker-ack state; `pre_processing` is the parse
// phase before the actual import begins. The handler layer in
// internal/api/v1/handlers/content_import.go drives the running →
// completed/failed transitions.
type ContentMigrationWorkflow string

const (
	ContentMigrationQueued        ContentMigrationWorkflow = "queued"
	ContentMigrationCreated       ContentMigrationWorkflow = "created"
	ContentMigrationPreProcessing ContentMigrationWorkflow = "pre_processing"
	ContentMigrationRunning       ContentMigrationWorkflow = "running"
	ContentMigrationCompleted     ContentMigrationWorkflow = "completed"
	ContentMigrationFailed        ContentMigrationWorkflow = "failed"
)

// CoursePaceWorkflow tracks the lifecycle of a CoursePace row.
// Values: CoursePaceUnpublished | CoursePaceActive | CoursePaceDeleted.
//
// `unpublished` is the row default established by migration 000017; pace
// rows become `active` once an instructor publishes them, and `deleted`
// on soft delete.
type CoursePaceWorkflow string

const (
	CoursePaceUnpublished CoursePaceWorkflow = "unpublished"
	CoursePaceActive      CoursePaceWorkflow = "active"
	CoursePaceDeleted     CoursePaceWorkflow = "deleted"
)

// ContentTagWorkflow tracks the lifecycle of a ContentTag row.
// Values: ContentTagActive | ContentTagUnpublished | ContentTagDeleted.
type ContentTagWorkflow string

const (
	ContentTagActive      ContentTagWorkflow = "active"
	ContentTagUnpublished ContentTagWorkflow = "unpublished"
	ContentTagDeleted     ContentTagWorkflow = "deleted"
)
