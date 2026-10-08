package service

import "fmt"

// Shared helpers for the services that copy course content from one
// course into another (Commons import, blueprint sync). Module items
// reference their content through (content_type, content_id), and the
// copy paths remap content_id through an id map keyed by the helpers
// below.

// normalizeModuleContentType folds the content_type spellings found on
// module items (content_tags) into one canonical name per resource:
// the handler writes "Quizzes::Quiz" while the IMSCC importer writes
// "Quiz", and both mean the classic quiz resource.
func normalizeModuleContentType(contentType string) string {
	switch contentType {
	case "Quizzes::Quiz", "Quiz":
		return "Quiz"
	case "WikiPage", "Page":
		return "WikiPage"
	case "DiscussionTopic", "Discussion":
		return "DiscussionTopic"
	case "Attachment", "File":
		return "Attachment"
	default:
		return contentType
	}
}

// contentKey is the id-map key for a piece of copied content.
func contentKey(contentType string, id uint) string {
	return fmt.Sprintf("%s:%d", normalizeModuleContentType(contentType), id)
}
