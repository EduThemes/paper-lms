package models

import (
	"bytes"
	"encoding/json"
	"strings"

	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/security/htmlsanitize"
)

// Rich-content write chokepoint.
//
// Handlers already wrap rich-content input in service.SanitizeHTML on Create
// and Update, but several write paths never pass through a handler: IMSCC /
// Canvas course imports, QTI imports, Commons imports, blueprint syncs and
// course copies, question-bank add/update and "pull to quiz". Those stored
// author-controlled HTML verbatim, and the web client renders question text,
// page bodies, descriptions and messages as HTML (B000/B001).
//
// These BeforeSave hooks make "rich-content columns are always stored
// sanitized" a model invariant: GORM runs them on Create and Save for every
// caller, so a new import or copy path cannot forget it. The policy is
// idempotent, so the double pass for handler writes is harmless.
//
// Only HTML (rich-content) columns belong here. Plain-text columns must never
// be sanitized: the policy entity-encodes text and the client renders those
// fields as text. Submission bodies are sanitized explicitly at submit time
// instead (a hook would re-run on every grading save of a legacy plain-text
// submission and strip "x<y and y>z").

// BeforeSave sanitizes the page body.
func (p *WikiPage) BeforeSave(_ *gorm.DB) error {
	p.Body = htmlsanitize.Sanitize(p.Body)
	return nil
}

// BeforeSave sanitizes the revision body (revisions are restorable).
func (r *WikiPageRevision) BeforeSave(_ *gorm.DB) error {
	r.Body = htmlsanitize.Sanitize(r.Body)
	return nil
}

// BeforeSave sanitizes the assignment description.
func (a *Assignment) BeforeSave(_ *gorm.DB) error {
	a.Description = htmlsanitize.Sanitize(a.Description)
	return nil
}

// BeforeSave sanitizes the quiz description.
func (q *Quiz) BeforeSave(_ *gorm.DB) error {
	q.Description = htmlsanitize.Sanitize(q.Description)
	return nil
}

// BeforeSave sanitizes the question text and its feedback (the feedback
// is edited with the rich content editor, G010).
func (q *QuizQuestion) BeforeSave(_ *gorm.DB) error {
	q.QuestionText = htmlsanitize.Sanitize(q.QuestionText)
	q.CorrectComments = htmlsanitize.Sanitize(q.CorrectComments)
	q.IncorrectComments = htmlsanitize.Sanitize(q.IncorrectComments)
	q.NeutralComments = htmlsanitize.Sanitize(q.NeutralComments)
	return nil
}

// BeforeSave sanitizes the question-bank entry text and feedback,
// including the per-outcome comments item-bank items are stored with
// (they are copied onto quiz questions, whose feedback is rich content).
func (e *QuestionBankEntry) BeforeSave(_ *gorm.DB) error {
	e.QuestionText = htmlsanitize.Sanitize(e.QuestionText)
	e.Feedback = htmlsanitize.Sanitize(e.Feedback)
	e.CorrectComments = htmlsanitize.Sanitize(e.CorrectComments)
	e.IncorrectComments = htmlsanitize.Sanitize(e.IncorrectComments)
	e.NeutralComments = htmlsanitize.Sanitize(e.NeutralComments)
	return nil
}

// BeforeSave sanitizes the passage content (shown with SafeHTML to every
// student taking the quiz). Covers the handler, QTI import and any copy.
func (s *QuizStimulus) BeforeSave(_ *gorm.DB) error {
	s.Content = SanitizeStimulusContent(s.Content)
	return nil
}

// SanitizeStimulusContent sanitizes a stimulus content value. The column
// is JSONB, so the value is either a JSON string holding HTML (sanitized
// and re-encoded), a TipTap-style document (every string under an "html"
// key is sanitized; text nodes are plain text and left alone), or — for a
// value that is not JSON at all — raw HTML, sanitized as such.
func SanitizeStimulusContent(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return content
	}
	var v interface{}
	if err := json.Unmarshal([]byte(trimmed), &v); err != nil {
		return htmlsanitize.Sanitize(content)
	}
	if s, ok := v.(string); ok {
		b, err := marshalJSONNoEscape(htmlsanitize.Sanitize(s))
		if err != nil {
			return "{}"
		}
		return string(b)
	}
	if !sanitizeStimulusHTMLNodes(v) {
		return content // nothing changed: keep the author's formatting
	}
	b, err := marshalJSONNoEscape(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// marshalJSONNoEscape encodes v without <-escaping <, > and &, so the
// stored passage stays readable HTML inside its JSON.
func marshalJSONNoEscape(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// sanitizeStimulusHTMLNodes sanitizes, in place, every string stored
// under an "html" key; it reports whether anything changed.
func sanitizeStimulusHTMLNodes(v interface{}) bool {
	changed := false
	switch t := v.(type) {
	case map[string]interface{}:
		for k, val := range t {
			if s, ok := val.(string); ok && strings.EqualFold(k, "html") {
				if clean := htmlsanitize.Sanitize(s); clean != s {
					t[k] = clean
					changed = true
				}
				continue
			}
			if sanitizeStimulusHTMLNodes(val) {
				changed = true
			}
		}
	case []interface{}:
		for _, val := range t {
			if sanitizeStimulusHTMLNodes(val) {
				changed = true
			}
		}
	}
	return changed
}

// BeforeSave sanitizes the event description (rich content, G010). Covers
// IMSCC / course-copy imports that never pass through the handler.
func (e *CalendarEvent) BeforeSave(_ *gorm.DB) error {
	e.Description = htmlsanitize.Sanitize(e.Description)
	return nil
}

// BeforeSave sanitizes the item-bank item text and feedback (copied onto
// quiz questions, whose feedback is rich content).
func (i *QuizItemBankItem) BeforeSave(_ *gorm.DB) error {
	i.QuestionText = htmlsanitize.Sanitize(i.QuestionText)
	i.CorrectComments = htmlsanitize.Sanitize(i.CorrectComments)
	i.IncorrectComments = htmlsanitize.Sanitize(i.IncorrectComments)
	i.NeutralComments = htmlsanitize.Sanitize(i.NeutralComments)
	return nil
}

// BeforeSave sanitizes the topic message.
func (d *DiscussionTopic) BeforeSave(_ *gorm.DB) error {
	d.Message = htmlsanitize.Sanitize(d.Message)
	return nil
}

// BeforeSave sanitizes the entry message.
func (e *DiscussionEntry) BeforeSave(_ *gorm.DB) error {
	e.Message = htmlsanitize.Sanitize(e.Message)
	return nil
}

// BeforeSave sanitizes the archived entry message.
func (v *DiscussionEntryVersion) BeforeSave(_ *gorm.DB) error {
	v.Message = htmlsanitize.Sanitize(v.Message)
	return nil
}

// BeforeSave sanitizes the announcement message.
func (a *Announcement) BeforeSave(_ *gorm.DB) error {
	a.Message = htmlsanitize.Sanitize(a.Message)
	return nil
}

// BeforeSave sanitizes the syllabus body.
func (c *Course) BeforeSave(_ *gorm.DB) error {
	c.SyllabusBody = htmlsanitize.Sanitize(c.SyllabusBody)
	return nil
}
