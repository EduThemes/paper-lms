package models

import "time"

// Canvas-compatible checkpoint types.
const (
	CheckpointTypeReplyToTopic = "reply_to_topic"
	CheckpointTypeReplyToEntry = "reply_to_entry"
)

// Canvas-compatible submission statuses.
const (
	CheckpointStatusNotStarted = "not_started"
	CheckpointStatusInProgress = "in_progress"
	CheckpointStatusCompleted  = "completed"
)

// DiscussionCheckpoint represents one of the multi-deadline requirements
// attached to a graded discussion topic (e.g., "post initial reply by Tue,
// reply twice by Fri"). Canvas added this in 2024.
type DiscussionCheckpoint struct {
	ID                uint       `json:"id" gorm:"primaryKey"`
	DiscussionTopicID uint       `json:"discussion_topic_id" gorm:"not null;index"`
	CheckpointType    string     `json:"checkpoint_type" gorm:"not null"` // reply_to_topic | reply_to_entry
	DueAt             *time.Time `json:"due_at"`
	PointsPossible    float64    `json:"points_possible" gorm:"not null;default:0"`
	RequiredReplies   int        `json:"required_replies" gorm:"not null;default:0"` // only meaningful for reply_to_entry
	WorkflowState     string     `json:"workflow_state" gorm:"not null;default:'active'"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// DiscussionCheckpointSubmission tracks a user's progress against a
// single checkpoint. ReplyCount stores how many qualifying replies the user
// has made (>= RequiredReplies on the parent checkpoint => completed).
type DiscussionCheckpointSubmission struct {
	ID                     uint       `json:"id" gorm:"primaryKey"`
	DiscussionCheckpointID uint       `json:"discussion_checkpoint_id" gorm:"not null;index;uniqueIndex:idx_dcs_checkpoint_user"`
	UserID                 uint       `json:"user_id" gorm:"not null;index;uniqueIndex:idx_dcs_checkpoint_user"`
	ReplyCount             int        `json:"reply_count" gorm:"not null;default:0"`
	Status                 string     `json:"status" gorm:"not null;default:'not_started'"`
	CompletedAt            *time.Time `json:"completed_at"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}
