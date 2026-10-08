package service

// NotificationCategory groups notification types the way Canvas's
// notification preferences page does: the user picks a frequency per
// category and per channel, not per individual message template.
//
// Types lists the notification_type strings (the second argument to
// QueueNotification) that belong to the category. A type that is not
// listed anywhere falls into "other". Code that raises a new kind of
// notification adds its type to the matching category here.
type NotificationCategory struct {
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Types       []string `json:"types"`
	// legacyToggle names the pre-G091 per-type boolean on
	// NotificationPreference that also gates this category when the user
	// has no explicit per-category policy ("" = none).
	legacyToggle string
}

// NotificationCategoryOther collects notification types no category lists.
const NotificationCategoryOther = "other"

// NotificationCategories is the catalog shown on the preferences page, in
// display order.
var NotificationCategories = []NotificationCategory{
	{Key: "announcement", Name: "Announcement", Description: "New announcements in your courses.",
		Types: []string{"new_announcement", "account_notification"}, legacyToggle: "new_announcement"},
	{Key: "announcement_created_by_you", Name: "Announcement Created By You", Description: "Replies to announcements you posted.",
		Types: []string{"announcement_reply"}},
	{Key: "conversation_message", Name: "Conversation Message", Description: "New inbox messages.",
		Types: []string{"new_message"}, legacyToggle: "new_message"},
	{Key: "added_to_conversation", Name: "Added To Conversation", Description: "You are added to an existing inbox conversation.",
		Types: []string{"added_to_conversation"}, legacyToggle: "new_message"},
	{Key: "discussion_entry", Name: "Discussion Post", Description: "New replies in discussions you take part in.",
		Types: []string{"discussion_reply", "discussion_entry"}},
	{Key: "due_date", Name: "Due Date", Description: "Assignment due dates change.",
		Types: []string{"assignment_due_date_changed", "assignment_due_date_override_changed"}},
	{Key: "course_content", Name: "Course Content", Description: "New or changed course content such as assignments and pages.",
		Types: []string{"assignment_created", "assignment_changed", "content_changed"}},
	{Key: "grading", Name: "Grading", Description: "Your work is graded.",
		Types: []string{"submission_grade", "submission_graded", "grade_changed"}, legacyToggle: "submission_grade"},
	{Key: "submission_comment", Name: "Submission Comment", Description: "Comments on your submissions.",
		Types: []string{"submission_comment"}},
	{Key: "calendar", Name: "Calendar", Description: "Calendar events start or change.",
		Types: []string{"event_start", "calendar_event", "calendar_event_changed"}, legacyToggle: "event_start"},
	{Key: "invitation", Name: "Invitation", Description: "Course, group and collaboration invitations.",
		Types: []string{"enrollment_invitation", "group_invitation", "collaboration_invitation"}},
	{Key: NotificationCategoryOther, Name: "Other", Description: "Everything else."},
}

var notificationTypeCategory = func() map[string]string {
	m := map[string]string{}
	for _, c := range NotificationCategories {
		for _, t := range c.Types {
			m[t] = c.Key
		}
	}
	return m
}()

// NotificationCategoryFor maps a notification type to its category key.
func NotificationCategoryFor(notificationType string) string {
	if k, ok := notificationTypeCategory[notificationType]; ok {
		return k
	}
	return NotificationCategoryOther
}

// IsNotificationCategory reports whether key names a catalog category.
func IsNotificationCategory(key string) bool {
	for _, c := range NotificationCategories {
		if c.Key == key {
			return true
		}
	}
	return false
}

func notificationCategoryByKey(key string) (NotificationCategory, bool) {
	for _, c := range NotificationCategories {
		if c.Key == key {
			return c, true
		}
	}
	return NotificationCategory{}, false
}

// IsNotificationFrequency reports whether f is a stored frequency value.
func IsNotificationFrequency(f string) bool {
	switch f {
	case "immediately", "daily", "weekly", "never":
		return true
	}
	return false
}
