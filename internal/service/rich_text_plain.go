package service

import (
	"regexp"
	"strings"
)

var (
	richTextBreak     = regexp.MustCompile(`(?i)<br\s*/?>|</(p|div|li|h[1-6]|blockquote|tr|pre)>`)
	richTextBlankRuns = regexp.MustCompile(`\n{3,}`)
)

// RichTextToPlain renders stored rich content (HTML) as plain text for
// text-only outputs such as an iCalendar DESCRIPTION: block ends and <br>
// become line breaks, markup is dropped and entities are decoded. Plain
// text written before the field became rich content passes through as is.
func RichTextToPlain(html string) string {
	if html == "" {
		return ""
	}
	text := richTextBreak.ReplaceAllString(html, "$0\n")
	text = SanitizePlainText(text)
	text = richTextBlankRuns.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}
