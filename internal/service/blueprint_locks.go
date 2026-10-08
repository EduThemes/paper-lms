package service

// Blueprint locking (G069): which attributes of synced blueprint content
// associated (child) courses may not change — Canvas's master-courses
// restrictions.
//
// Each blueprint item's effective restrictions are its own lock row
// (blueprint_content_locks) when it has one, otherwise the template's
// default restrictions when the template uses them. A locked item:
//
//   - is always overwritten by the next sync, local edits or not;
//   - cannot have its locked attributes edited in a child course, and
//     cannot be deleted there (CheckChildEdit, enforced by the
//     blueprint lock guard on the content write routes).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// BlueprintRestrictions is the set of locked attribute categories.
type BlueprintRestrictions struct {
	Content           bool `json:"content"`
	Points            bool `json:"points"`
	DueDates          bool `json:"due_dates"`
	AvailabilityDates bool `json:"availability_dates"`
}

// Any reports whether anything is locked.
func (r BlueprintRestrictions) Any() bool {
	return r.Content || r.Points || r.DueDates || r.AvailabilityDates
}

// JSON encodes the restrictions for storage.
func (r BlueprintRestrictions) JSON() string {
	b, _ := json.Marshal(r)
	return string(b)
}

// ParseBlueprintRestrictions decodes stored restrictions; malformed or
// empty input locks nothing.
func ParseBlueprintRestrictions(s string) BlueprintRestrictions {
	var r BlueprintRestrictions
	if strings.TrimSpace(s) == "" {
		return r
	}
	_ = json.Unmarshal([]byte(s), &r)
	return r
}

// templateDefaultRestrictions returns what an item without its own lock
// row inherits.
func templateDefaultRestrictions(t *models.BlueprintTemplate) BlueprintRestrictions {
	if t == nil || !t.UseDefaultRestrictions {
		return BlueprintRestrictions{}
	}
	return ParseBlueprintRestrictions(t.DefaultRestrictions)
}

// blueprintLockableTypes are the content types that carry restrictions.
// Modules, module items, files and assignment groups sync but are not
// lockable (as in Canvas, a child may rearrange its own structure).
var blueprintLockableTypes = map[string]bool{
	"Assignment": true, "Quiz": true, "WikiPage": true, "DiscussionTopic": true,
}

// BlueprintLockedError is returned when a child-course edit touches a
// locked attribute of blueprint content.
type BlueprintLockedError struct {
	Attributes []string
	Delete     bool
}

func (e *BlueprintLockedError) Error() string {
	if e.Delete {
		return "this content is locked by its blueprint course and cannot be deleted"
	}
	return fmt.Sprintf("this content is locked by its blueprint course (%s)", strings.Join(e.Attributes, ", "))
}

// ErrBlueprintLockInvalidContent is returned when a lock is set on
// content that is not lockable or not in the blueprint course.
var ErrBlueprintLockInvalidContent = errors.New("content not found in the blueprint course")

// SetLockRepo wires the per-item lock store.
func (s *BlueprintService) SetLockRepo(repo repository.BlueprintContentLockRepository) {
	s.lockRepo = repo
}

// SetSyncDeps wires the optional sync dependencies: assignment groups,
// the course row (syllabus) and file copies.
func (s *BlueprintService) SetSyncDeps(groupRepo repository.AssignmentGroupRepository, attachmentRepo repository.AttachmentRepository, files *FileService) {
	s.groupRepo = groupRepo
	s.attachmentRepo = attachmentRepo
	s.files = files
}

// loadLocks returns contentKey(type, sourceID) → restrictions for every
// item of the template with its own lock row.
func (s *BlueprintService) loadLocks(ctx context.Context, templateID uint) map[string]BlueprintRestrictions {
	out := map[string]BlueprintRestrictions{}
	if s.lockRepo == nil {
		return out
	}
	locks, err := s.lockRepo.ListByTemplate(ctx, templateID)
	if err != nil {
		return out
	}
	for _, l := range locks {
		out[contentKey(l.ContentType, l.ContentID)] = ParseBlueprintRestrictions(l.Restrictions)
	}
	return out
}

// BlueprintContentItem is one lockable piece of blueprint content.
type BlueprintContentItem struct {
	ContentType  string                `json:"content_type"`
	ContentID    uint                  `json:"content_id"`
	Title        string                `json:"title"`
	Locked       bool                  `json:"locked"`
	Restrictions BlueprintRestrictions `json:"restrictions"`
	// Explicit is true when the item has its own lock setting rather
	// than inheriting the template defaults.
	Explicit bool `json:"explicit"`
}

// ListLockableContent lists the blueprint course's lockable content with
// each item's effective restrictions.
func (s *BlueprintService) ListLockableContent(ctx context.Context, template *models.BlueprintTemplate) ([]BlueprintContentItem, error) {
	if template == nil {
		return nil, errors.New("template is required")
	}
	defaults := templateDefaultRestrictions(template)
	locks := s.loadLocks(ctx, template.ID)
	var out []BlueprintContentItem
	add := func(t string, id uint, title string) {
		r, explicit := locks[contentKey(t, id)]
		if !explicit {
			r = defaults
		}
		out = append(out, BlueprintContentItem{ContentType: t, ContentID: id, Title: title, Locked: r.Any(), Restrictions: r, Explicit: explicit})
	}
	courseID := template.CourseID
	if as, err := s.assignRepo.ListByCourseID(ctx, courseID, bigPage); err == nil {
		for _, a := range as.Items {
			add("Assignment", a.ID, a.Name)
		}
	}
	if qs, err := s.quizRepo.ListByCourseID(ctx, courseID, bigPage); err == nil {
		for _, q := range qs.Items {
			add("Quiz", q.ID, q.Title)
		}
	}
	if ps, err := s.pageRepo.ListByCourseID(ctx, courseID, bigPage); err == nil {
		for _, p := range ps.Items {
			add("WikiPage", p.ID, p.Title)
		}
	}
	if ds, err := s.discRepo.ListByCourseID(ctx, courseID, 0, bigPage); err == nil {
		for _, d := range ds.Items {
			add("DiscussionTopic", d.ID, d.Title)
		}
	}
	return out, nil
}

// SetItemRestrictions sets one blueprint item's own restrictions; nil
// removes its row so it inherits the template defaults again. The item
// must be lockable content of the blueprint course (checked within
// accountID, the caller's tenant).
func (s *BlueprintService) SetItemRestrictions(ctx context.Context, template *models.BlueprintTemplate, accountID uint, contentType string, contentID uint, r *BlueprintRestrictions) error {
	if s.lockRepo == nil {
		return errors.New("blueprint locking is not configured")
	}
	contentType = normalizeModuleContentType(contentType)
	if template == nil || !blueprintLockableTypes[contentType] || contentID == 0 {
		return ErrBlueprintLockInvalidContent
	}
	var owner uint
	switch contentType {
	case "Assignment":
		if a, err := s.assignRepo.FindByID(ctx, contentID, accountID); err == nil && a != nil && a.WorkflowState != models.AssignmentDeleted {
			owner = a.CourseID
		}
	case "Quiz":
		if q, err := s.quizRepo.FindByID(ctx, contentID, accountID); err == nil && q != nil && q.WorkflowState != "deleted" {
			owner = q.CourseID
		}
	case "WikiPage":
		if p, err := s.pageRepo.FindByID(ctx, contentID, accountID); err == nil && p != nil && p.WorkflowState != "deleted" {
			owner = p.CourseID
		}
	case "DiscussionTopic":
		if d, err := s.discRepo.FindByID(ctx, contentID, accountID); err == nil && d != nil && d.WorkflowState != models.DiscussionTopicDeleted {
			owner = d.CourseID
		}
	}
	if owner == 0 || owner != template.CourseID {
		return ErrBlueprintLockInvalidContent
	}
	if r == nil {
		return s.lockRepo.Delete(ctx, template.ID, contentType, contentID)
	}
	return s.lockRepo.Upsert(ctx, &models.BlueprintContentLock{
		BlueprintTemplateID: template.ID,
		ContentType:         contentType,
		ContentID:           contentID,
		Restrictions:        r.JSON(),
	})
}

// ChildRestrictions returns the restrictions on a child-course row that
// is a synced blueprint copy. ok=false when the row is not a copy (or
// its course is no longer associated).
func (s *BlueprintService) ChildRestrictions(ctx context.Context, childCourseID uint, contentType string, childID uint) (BlueprintRestrictions, bool, error) {
	contentType = normalizeModuleContentType(contentType)
	if s.tagRepo == nil || !blueprintLockableTypes[contentType] {
		return BlueprintRestrictions{}, false, nil
	}
	tag, err := s.tagRepo.FindByChildItem(ctx, childCourseID, contentType, childID)
	if err != nil || tag == nil {
		return BlueprintRestrictions{}, false, err
	}
	sub, err := s.subRepo.FindByTemplateAndChild(ctx, tag.BlueprintTemplateID, childCourseID)
	if err != nil || sub == nil || sub.WorkflowState != "active" {
		return BlueprintRestrictions{}, false, nil
	}
	template, err := s.tmplRepo.FindByID(ctx, tag.BlueprintTemplateID)
	if err != nil || template == nil {
		return BlueprintRestrictions{}, false, nil
	}
	r := templateDefaultRestrictions(template)
	if s.lockRepo != nil {
		if l, err := s.lockRepo.Find(ctx, template.ID, contentType, tag.SourceID); err == nil && l != nil {
			r = ParseBlueprintRestrictions(l.Restrictions)
		}
	}
	return r, true, nil
}

// lockedFieldCategories maps, per content type, the JSON attribute names
// of a write body onto restriction categories.
var lockedFieldCategories = map[string]map[string]string{
	"Assignment": {
		"name": "content", "description": "content", "submission_types": "content", "grading_type": "content",
		"points_possible": "points",
		"due_at":          "due_dates",
		"unlock_at":       "availability_dates", "lock_at": "availability_dates",
	},
	"Quiz": {
		"title": "content", "description": "content", "quiz_type": "content",
		"points_possible": "points",
		"due_at":          "due_dates",
		"unlock_at":       "availability_dates", "lock_at": "availability_dates",
	},
	"WikiPage": {
		"title": "content", "body": "content",
	},
	"DiscussionTopic": {
		"title": "content", "message": "content", "discussion_type": "content",
		"delayed_post_at": "availability_dates", "lock_at": "availability_dates",
	},
}

func (r BlueprintRestrictions) has(category string) bool {
	switch category {
	case "content":
		return r.Content
	case "points":
		return r.Points
	case "due_dates":
		return r.DueDates
	case "availability_dates":
		return r.AvailabilityDates
	}
	return false
}

// ChildEdit describes a write to a child-course row.
type ChildEdit struct {
	ContentType string
	ChildID     uint
	Delete      bool
	// ContentWrite marks writes that always change content (quiz
	// questions and question groups).
	ContentWrite bool
	// Changes is the write body's attribute map (already unwrapped from
	// its {"assignment": {...}} envelope); nil when not JSON.
	Changes map[string]json.RawMessage
}

// CheckChildEdit returns a *BlueprintLockedError when the edit touches a
// locked attribute of a synced blueprint copy, nil otherwise. Attribute
// writes are compared with the stored value: re-sending an unchanged
// locked field (a full-form save) is not an edit.
func (s *BlueprintService) CheckChildEdit(ctx context.Context, childCourseID uint, e ChildEdit) error {
	r, isCopy, err := s.ChildRestrictions(ctx, childCourseID, e.ContentType, e.ChildID)
	if err != nil || !isCopy || !r.Any() {
		return err
	}
	if e.Delete {
		return &BlueprintLockedError{Delete: true}
	}
	if e.ContentWrite {
		if r.Content {
			return &BlueprintLockedError{Attributes: []string{"content"}}
		}
		return nil
	}
	if len(e.Changes) == 0 {
		return nil
	}
	current, err := s.childRowJSON(ctx, normalizeModuleContentType(e.ContentType), e.ChildID)
	if err != nil || current == nil {
		return err
	}
	fields := lockedFieldCategories[normalizeModuleContentType(e.ContentType)]
	hit := map[string]bool{}
	for key, raw := range e.Changes {
		cat, ok := fields[key]
		if !ok || !r.has(cat) {
			continue
		}
		if jsonValueChanged(raw, current[key]) {
			hit[cat] = true
		}
	}
	if len(hit) == 0 {
		return nil
	}
	cats := make([]string, 0, len(hit))
	for c := range hit {
		cats = append(cats, c)
	}
	sort.Strings(cats)
	return &BlueprintLockedError{Attributes: cats}
}

// childRowJSON loads a child row and returns its JSON attribute map.
// accountID=0: auth-internal — the guard resolved the row from the
// route's own course id and only reads it to compare values.
func (s *BlueprintService) childRowJSON(ctx context.Context, contentType string, id uint) (map[string]interface{}, error) {
	var row interface{}
	switch contentType {
	case "Assignment":
		a, err := s.assignRepo.FindByID(ctx, id, 0)
		if err != nil {
			return nil, nil
		}
		row = a
	case "Quiz":
		q, err := s.quizRepo.FindByID(ctx, id, 0)
		if err != nil {
			return nil, nil
		}
		row = q
	case "WikiPage":
		p, err := s.pageRepo.FindByID(ctx, id, 0)
		if err != nil {
			return nil, nil
		}
		row = p
	case "DiscussionTopic":
		d, err := s.discRepo.FindByID(ctx, id, 0)
		if err != nil {
			return nil, nil
		}
		row = d
	default:
		return nil, nil
	}
	b, err := json.Marshal(row)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// jsonValueChanged compares a submitted JSON value with the stored one:
// null and "" are both "empty", numbers compare numerically and
// timestamps as instants.
func jsonValueChanged(raw json.RawMessage, current interface{}) bool {
	var next interface{}
	if err := json.Unmarshal(raw, &next); err != nil {
		return true
	}
	empty := func(v interface{}) bool {
		if v == nil {
			return true
		}
		s, ok := v.(string)
		return ok && strings.TrimSpace(s) == ""
	}
	if empty(next) && empty(current) {
		return false
	}
	if ns, ok := next.(string); ok {
		if cs, ok := current.(string); ok {
			if nt, err1 := time.Parse(time.RFC3339Nano, ns); err1 == nil {
				if ct, err2 := time.Parse(time.RFC3339Nano, cs); err2 == nil {
					return !nt.Equal(ct)
				}
			}
			return ns != cs
		}
		// A number sent as a string ("10") against a stored number.
		if cf, ok := current.(float64); ok {
			var nf float64
			if _, err := fmt.Sscanf(ns, "%g", &nf); err == nil {
				return nf != cf
			}
		}
		return true
	}
	return !reflect.DeepEqual(next, current)
}

// BlueprintLockedChildItem is a locked blueprint copy in a child course.
type BlueprintLockedChildItem struct {
	ContentType  string                `json:"content_type"`
	ChildID      uint                  `json:"content_id"`
	Title        string                `json:"title"`
	Restrictions BlueprintRestrictions `json:"restrictions"`
}

// ListChildLockedContent lists the locked blueprint copies in a child
// course (for the child course's blueprint panel).
func (s *BlueprintService) ListChildLockedContent(ctx context.Context, childCourseID uint) ([]BlueprintLockedChildItem, error) {
	var out []BlueprintLockedChildItem
	if s.tagRepo == nil {
		return out, nil
	}
	subs, err := s.subRepo.ListByChildCourseID(ctx, childCourseID, bigPage)
	if err != nil {
		return nil, err
	}
	for _, sub := range subs.Items {
		if sub.WorkflowState != "active" {
			continue
		}
		template, err := s.tmplRepo.FindByID(ctx, sub.BlueprintTemplateID)
		if err != nil || template == nil {
			continue
		}
		defaults := templateDefaultRestrictions(template)
		locks := s.loadLocks(ctx, template.ID)
		tags, err := s.tagRepo.ListByChildCourse(ctx, template.ID, childCourseID)
		if err != nil {
			continue
		}
		for _, tag := range tags {
			if !blueprintLockableTypes[tag.ContentType] {
				continue
			}
			r, ok := locks[contentKey(tag.ContentType, tag.SourceID)]
			if !ok {
				r = defaults
			}
			if !r.Any() {
				continue
			}
			title := ""
			if row, err := s.childRowJSON(ctx, tag.ContentType, tag.ChildID); err == nil && row != nil {
				if st, _ := row["workflow_state"].(string); st == "deleted" {
					continue
				}
				if v, ok := row["name"].(string); ok {
					title = v
				} else if v, ok := row["title"].(string); ok {
					title = v
				}
			} else {
				continue
			}
			out = append(out, BlueprintLockedChildItem{ContentType: tag.ContentType, ChildID: tag.ChildID, Title: title, Restrictions: r})
		}
	}
	return out, nil
}
