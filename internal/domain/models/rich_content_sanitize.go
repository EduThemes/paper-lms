package models

import (
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

// BeforeSave sanitizes the question text.
func (q *QuizQuestion) BeforeSave(_ *gorm.DB) error {
	q.QuestionText = htmlsanitize.Sanitize(q.QuestionText)
	return nil
}

// BeforeSave sanitizes the question-bank entry text.
func (e *QuestionBankEntry) BeforeSave(_ *gorm.DB) error {
	e.QuestionText = htmlsanitize.Sanitize(e.QuestionText)
	return nil
}

// BeforeSave sanitizes the item-bank item text.
func (i *QuizItemBankItem) BeforeSave(_ *gorm.DB) error {
	i.QuestionText = htmlsanitize.Sanitize(i.QuestionText)
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
