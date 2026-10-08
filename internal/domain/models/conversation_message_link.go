package models

import "time"

// ConversationMessageAttachment links a file the sender uploaded to an
// inbox message (migration 000128). The attachment itself lives in the
// sender's User context; participants who can see the message download it
// through the conversation route, never through the generic file route.
type ConversationMessageAttachment struct {
	ID                    uint      `json:"id" gorm:"column:id;primaryKey"`
	ConversationMessageID uint      `json:"conversation_message_id" gorm:"not null;uniqueIndex:idx_conv_msg_attachment,priority:1"`
	AttachmentID          uint      `json:"attachment_id" gorm:"not null;uniqueIndex:idx_conv_msg_attachment,priority:2;index:idx_conversation_message_attachments_attachment_id"`
	CreatedAt             time.Time `json:"created_at"`
}

// ConversationMessageRecipient restricts a message to a subset of the
// conversation's participants ("Reply" to one person inside a group
// conversation, as opposed to "Reply all"). A message with no recipient
// rows is visible to every participant; with rows, only to the author and
// the listed users (migration 000128).
type ConversationMessageRecipient struct {
	ID                    uint      `json:"id" gorm:"column:id;primaryKey"`
	ConversationMessageID uint      `json:"conversation_message_id" gorm:"not null;uniqueIndex:idx_conv_msg_recipient,priority:1"`
	UserID                uint      `json:"user_id" gorm:"not null;uniqueIndex:idx_conv_msg_recipient,priority:2;index:idx_conversation_message_recipients_user_id"`
	CreatedAt             time.Time `json:"created_at"`
}
