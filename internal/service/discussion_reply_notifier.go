package service

import (
	"context"
	"fmt"

	"github.com/EduThemes/paper-lms/internal/repository"
)

// DiscussionReplyNotificationCallback returns an OnEntryCreated callback
// that notifies the people a new discussion entry is aimed at (Canvas's
// "Discussion Entry" / "Discussion" notifications):
//
//   - a reply notifies the author of the entry it replies to;
//   - a top-level entry notifies the topic's author.
//
// Nobody is notified about their own post. Errors are swallowed: the
// callback runs asynchronously after the entry is already saved.
func DiscussionReplyNotificationCallback(
	entryRepo repository.DiscussionEntryRepository,
	topicRepo repository.DiscussionTopicRepository,
	notifier Notifier,
) DiscussionEntryPostedCallback {
	return func(ctx context.Context, entryID uint) {
		if notifier == nil || entryRepo == nil || topicRepo == nil {
			return
		}
		// accountID=0: background hop from a just-written, tenant-checked
		// entry id.
		entry, err := entryRepo.FindByID(ctx, entryID, 0)
		if err != nil || entry == nil {
			return
		}
		topic, err := topicRepo.FindByID(ctx, entry.DiscussionTopicID, 0)
		if err != nil || topic == nil {
			return
		}

		recipient := topic.UserID
		subject := fmt.Sprintf("New post in %s", topic.Title)
		if entry.ParentID != nil {
			parent, err := entryRepo.FindByID(ctx, *entry.ParentID, 0)
			if err != nil || parent == nil {
				return
			}
			recipient = parent.UserID
			subject = fmt.Sprintf("New reply in %s", topic.Title)
		}
		if recipient == 0 || recipient == entry.UserID {
			return
		}
		_ = notifier.QueueNotification(ctx, recipient, "discussion_reply", subject, entry.Message, "Course", topic.CourseID)
	}
}
