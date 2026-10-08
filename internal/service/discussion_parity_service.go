package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"sort"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/security/htmlsanitize"
)

// DiscussionAudience decides whether a non-staff user may open a topic
// beyond the publish / delayed-post rules: section-specific topics and
// the per-group child topics of a group discussion.
type DiscussionAudience interface {
	TopicVisibleToUser(ctx context.Context, topic *models.DiscussionTopic, userID uint) bool
}

// Errors returned by the discussion parity service. Handlers map them to
// 400 responses.
var (
	ErrDiscussionInvalidSections     = errors.New("every section must belong to this course")
	ErrDiscussionSectionsRequired    = errors.New("a section-specific discussion needs at least one section")
	ErrDiscussionInvalidGroupSet     = errors.New("group set not found in this course")
	ErrDiscussionGroupSetLocked      = errors.New("the group set can't change after students have posted")
	ErrDiscussionInvalidAnonymity    = errors.New("anonymous_state must be full_anonymity, partial_anonymity or empty")
	ErrDiscussionAnonymousConflict   = errors.New("anonymous discussions can't be graded, group or section-specific")
	ErrDiscussionSectionConflict     = errors.New("graded and group discussions can't be section-specific")
	ErrDiscussionGradedHasSubmission = errors.New("a graded discussion with submissions can't be made ungraded")
	ErrDiscussionInvalidAssignment   = errors.New("assignment not found in this course")
	ErrDiscussionInvalidPoints       = errors.New("points_possible must be zero or more")
)

// DiscussionGradedInput is the "assignment" object on a topic create or
// update (Canvas discussion_topic[assignment]).
type DiscussionGradedInput struct {
	PointsPossible    *float64   `json:"points_possible"`
	GradingType       string     `json:"grading_type"`
	DueAt             *time.Time `json:"due_at"`
	UnlockAt          *time.Time `json:"unlock_at"`
	LockAt            *time.Time `json:"lock_at"`
	AssignmentGroupID *uint      `json:"assignment_group_id"`

	// present records which keys the request body carried, so an edit
	// that leaves a field out keeps the assignment's current value
	// (the topic form sends only points_possible and due_at). nil means
	// "built in code": every field counts as given.
	present map[string]bool
}

// UnmarshalJSON decodes the object and remembers which keys were sent.
func (in *DiscussionGradedInput) UnmarshalJSON(b []byte) error {
	type plain DiscussionGradedInput
	var v plain
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(b, &keys); err != nil {
		return err
	}
	*in = DiscussionGradedInput(v)
	in.present = make(map[string]bool, len(keys))
	for k := range keys {
		in.present[k] = true
	}
	return nil
}

// has reports whether the request carried the key.
func (in *DiscussionGradedInput) has(key string) bool {
	return in.present == nil || in.present[key]
}

// DiscussionIndexItem is one row of the discussion index with the
// viewer's counters.
type DiscussionIndexItem struct {
	Topic       models.DiscussionTopic
	EntryCount  int64
	UnreadCount int64
	LastReplyAt *time.Time
	SectionIDs  []uint
	GroupTopics int
}

// DiscussionParityService implements the Canvas discussion features on
// top of the base discussion services: availability for sections and
// groups, require-initial-post, group discussions, graded discussions,
// anonymity, subscriptions / @mention notifications and the index.
//
// Every method that takes a topic expects the handler to have resolved
// it inside the caller's tenant and course first.
type DiscussionParityService struct {
	topicRepo            repository.DiscussionTopicRepository
	entryRepo            repository.DiscussionEntryRepository
	parity               repository.DiscussionParityRepository
	topicParticipantRepo repository.DiscussionTopicParticipantRepository
	assignmentRepo       repository.AssignmentRepository
	submissionRepo       repository.SubmissionRepository
	attachmentRepo       repository.AttachmentRepository
	notifier             Notifier
	now                  func() time.Time
}

func NewDiscussionParityService(
	topicRepo repository.DiscussionTopicRepository,
	entryRepo repository.DiscussionEntryRepository,
	parity repository.DiscussionParityRepository,
	topicParticipantRepo repository.DiscussionTopicParticipantRepository,
	assignmentRepo repository.AssignmentRepository,
	submissionRepo repository.SubmissionRepository,
	notifier Notifier,
) *DiscussionParityService {
	return &DiscussionParityService{
		topicRepo:            topicRepo,
		entryRepo:            entryRepo,
		parity:               parity,
		topicParticipantRepo: topicParticipantRepo,
		assignmentRepo:       assignmentRepo,
		submissionRepo:       submissionRepo,
		notifier:             notifier,
		now:                  time.Now,
	}
}

// SetAttachmentRepo enables file attachments on posts.
func (s *DiscussionParityService) SetAttachmentRepo(r repository.AttachmentRepository) {
	s.attachmentRepo = r
}

// ErrDiscussionInvalidAttachment is returned when a post names a file the
// poster didn't upload.
var ErrDiscussionInvalidAttachment = errors.New("attachment not found")

// ValidatePostAttachment checks that a post's attachment is a live file
// the poster uploaded to their own files, in the caller's tenant.
func (s *DiscussionParityService) ValidatePostAttachment(ctx context.Context, attachmentID, userID, accountID uint) error {
	if s.attachmentRepo == nil {
		return ErrDiscussionInvalidAttachment
	}
	a, err := s.attachmentRepo.FindByID(ctx, attachmentID, accountID)
	if err != nil || a == nil || a.UserID != userID || a.ContextType != "User" || a.ContextID != userID ||
		a.WorkflowState == models.AttachmentDeleted {
		return ErrDiscussionInvalidAttachment
	}
	return nil
}

// Attachments loads the given attachments (missing ones are skipped).
func (s *DiscussionParityService) Attachments(ctx context.Context, ids []uint) map[uint]*models.Attachment {
	out := map[uint]*models.Attachment{}
	if s.attachmentRepo == nil {
		return out
	}
	for _, id := range discussionUniqueUints(ids) {
		// accountID=0: internal hop; the ids come from entries already
		// resolved in the caller's tenant, course and topic.
		if a, err := s.attachmentRepo.FindByID(ctx, id, 0); err == nil && a != nil {
			out[id] = a
		}
	}
	return out
}

// SetClock swaps the clock (tests).
func (s *DiscussionParityService) SetClock(now func() time.Time) { s.now = now }

// ---------------------------------------------------------------------------
// Availability (G083 / G085 / G087)
// ---------------------------------------------------------------------------

// TopicVisibleToUser applies the per-user rules for a non-staff viewer
// (the publish / delayed-post rules are checked separately):
//
//   - a group's child topic is visible to that group's members only;
//   - a section-specific topic to users enrolled in one of its sections.
//
// Any lookup error fails closed.
func (s *DiscussionParityService) TopicVisibleToUser(ctx context.Context, topic *models.DiscussionTopic, userID uint) bool {
	if topic == nil || userID == 0 {
		return false
	}
	if topic.RootTopicID != nil {
		if topic.GroupID == nil {
			return false
		}
		ok, err := s.parity.IsGroupMember(ctx, *topic.GroupID, userID)
		if err != nil || !ok {
			return false
		}
	}
	if topic.IsSectionSpecific {
		return s.inTopicSections(ctx, topic, userID)
	}
	return true
}

func (s *DiscussionParityService) inTopicSections(ctx context.Context, topic *models.DiscussionTopic, userID uint) bool {
	sections, err := s.parity.ListTopicSectionIDs(ctx, []uint{topic.ID})
	if err != nil {
		return false
	}
	mine, err := s.parity.UserSectionIDs(ctx, topic.CourseID, userID)
	if err != nil {
		return false
	}
	allowed := map[uint]bool{}
	for _, id := range sections[topic.ID] {
		allowed[id] = true
	}
	for _, id := range mine {
		if allowed[id] {
			return true
		}
	}
	return false
}

// InitialPostRequired mirrors Canvas initial_post_required?: on a
// require_initial_post topic, a (non-staff) user who hasn't posted yet
// can't see anyone else's posts. Errors fail closed (posts hidden).
func (s *DiscussionParityService) InitialPostRequired(ctx context.Context, topic *models.DiscussionTopic, userID uint) bool {
	if topic == nil || !topic.RequireInitialPost {
		return false
	}
	posted, err := s.parity.UserHasPosted(ctx, topic.ID, userID)
	if err != nil {
		return true
	}
	if posted {
		return false
	}
	// Canvas: an observer sees the posts once a student they observe has
	// posted.
	observed, err := s.parity.ObservedStudentIDs(ctx, topic.CourseID, userID)
	if err != nil {
		return true
	}
	for _, sid := range observed {
		if ok, err := s.parity.UserHasPosted(ctx, topic.ID, sid); err == nil && ok {
			return false
		}
	}
	return true
}

// ValidateSections checks a section-specific topic's sections before it
// is saved: at least one, all active sections of the course.
func (s *DiscussionParityService) ValidateSections(ctx context.Context, courseID uint, sectionIDs []uint) error {
	ids := discussionUniqueUints(sectionIDs)
	if len(ids) == 0 {
		return ErrDiscussionSectionsRequired
	}
	n, err := s.parity.CountCourseSections(ctx, courseID, ids)
	if err != nil {
		return err
	}
	if int(n) != len(ids) {
		return ErrDiscussionInvalidSections
	}
	return nil
}

// SetTopicSections validates and stores the sections of a
// section-specific topic (an empty list clears them).
func (s *DiscussionParityService) SetTopicSections(ctx context.Context, topic *models.DiscussionTopic, sectionIDs []uint) error {
	ids := discussionUniqueUints(sectionIDs)
	if topic.IsSectionSpecific && len(ids) == 0 {
		return ErrDiscussionSectionsRequired
	}
	if len(ids) > 0 {
		n, err := s.parity.CountCourseSections(ctx, topic.CourseID, ids)
		if err != nil {
			return err
		}
		if int(n) != len(ids) {
			return ErrDiscussionInvalidSections
		}
	}
	if !topic.IsSectionSpecific {
		ids = nil
	}
	return s.parity.ReplaceTopicSections(ctx, topic.ID, ids)
}

// TopicSectionIDs returns the sections of each topic.
func (s *DiscussionParityService) TopicSectionIDs(ctx context.Context, topicIDs []uint) map[uint][]uint {
	out, err := s.parity.ListTopicSectionIDs(ctx, topicIDs)
	if err != nil {
		return map[uint][]uint{}
	}
	return out
}

// ValidateTopicOptions enforces the Canvas combination rules between
// anonymity, grading, groups and sections. graded is true when the topic
// has (or is about to get) an assignment.
func ValidateTopicOptions(topic *models.DiscussionTopic, graded bool) error {
	graded = graded || topic.AssignmentID != nil
	if topic.AnonymousState != nil {
		switch *topic.AnonymousState {
		case models.DiscussionFullAnonymity, models.DiscussionPartialAnonymity:
		default:
			return ErrDiscussionInvalidAnonymity
		}
		if graded || topic.GroupCategoryID != nil || topic.IsSectionSpecific {
			return ErrDiscussionAnonymousConflict
		}
	}
	if topic.IsSectionSpecific && (graded || topic.GroupCategoryID != nil) {
		return ErrDiscussionSectionConflict
	}
	return nil
}

// DiscussionStateForSchedule returns the workflow state a published
// topic should be in for its delayed_post_at: post_delayed while the
// date is in the future, active otherwise. Unpublished, locked and
// deleted topics keep their state.
func DiscussionStateForSchedule(topic *models.DiscussionTopic, now time.Time) models.DiscussionTopicWorkflow {
	switch topic.WorkflowState {
	case models.DiscussionTopicActive, models.DiscussionTopicPostDelayed:
		if topic.DelayedPostAt != nil && topic.DelayedPostAt.After(now) {
			return models.DiscussionTopicPostDelayed
		}
		return models.DiscussionTopicActive
	}
	return topic.WorkflowState
}

// NormalizeAnonymousState maps "" to nil.
func NormalizeAnonymousState(v *string) *string {
	if v == nil {
		return nil
	}
	t := strings.TrimSpace(*v)
	if t == "" {
		return nil
	}
	return &t
}

// ---------------------------------------------------------------------------
// Group discussions (G085)
// ---------------------------------------------------------------------------

// ValidateGroupCategory checks that categoryID is a live group set of
// the topic's course.
func (s *DiscussionParityService) ValidateGroupCategory(ctx context.Context, courseID, categoryID uint) error {
	ok, err := s.parity.GroupCategoryInCourse(ctx, categoryID, courseID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrDiscussionInvalidGroupSet
	}
	return nil
}

// GroupCategoryChangeAllowed refuses to move a group discussion to
// another group set (or off groups) once anyone has posted in it.
func (s *DiscussionParityService) GroupCategoryChangeAllowed(ctx context.Context, root *models.DiscussionTopic) error {
	ids := []uint{root.ID}
	children, err := s.parity.ListChildTopics(ctx, root.ID)
	if err != nil {
		return err
	}
	for _, c := range children {
		ids = append(ids, c.ID)
	}
	stats, err := s.parity.TopicStats(ctx, ids, 0)
	if err != nil {
		return err
	}
	for _, st := range stats {
		if st.EntryCount > 0 {
			return ErrDiscussionGroupSetLocked
		}
	}
	return nil
}

// copyIntoChild copies the root's content and settings onto a child.
func copyIntoChild(root, child *models.DiscussionTopic) {
	child.CourseID = root.CourseID
	child.UserID = root.UserID
	child.Title = root.Title
	child.Message = root.Message
	child.DiscussionType = root.DiscussionType
	child.PostedAt = root.PostedAt
	child.DelayedPostAt = root.DelayedPostAt
	child.LockAt = root.LockAt
	child.Locked = root.Locked
	child.AllowRating = root.AllowRating
	child.OnlyGradersCanRate = root.OnlyGradersCanRate
	child.SortByRating = root.SortByRating
	child.RequireInitialPost = root.RequireInitialPost
	child.WorkflowState = root.WorkflowState
	child.Pinned = false
	child.AssignmentID = nil
	child.GroupCategoryID = nil
	child.AnonymousState = nil
	child.IsSectionSpecific = false
	rootID := root.ID
	child.RootTopicID = &rootID
}

func childNeedsSync(root, child *models.DiscussionTopic) bool {
	want := *child
	copyIntoChild(root, &want)
	return want.Title != child.Title || want.Message != child.Message ||
		want.DiscussionType != child.DiscussionType ||
		!sameTimePtr(want.DelayedPostAt, child.DelayedPostAt) ||
		!sameTimePtr(want.LockAt, child.LockAt) ||
		want.Locked != child.Locked || want.AllowRating != child.AllowRating ||
		want.OnlyGradersCanRate != child.OnlyGradersCanRate ||
		want.SortByRating != child.SortByRating ||
		want.RequireInitialPost != child.RequireInitialPost ||
		want.WorkflowState != child.WorkflowState ||
		want.UserID != child.UserID
}

func sameTimePtr(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

// SyncGroupTopics makes the root's child topics match its group set:
// one child per live group, carrying the root's content and settings.
// Children of groups that no longer exist are removed when nobody
// posted in them. When the root has no group set, children without
// posts are removed. Returns the live children.
func (s *DiscussionParityService) SyncGroupTopics(ctx context.Context, root *models.DiscussionTopic) ([]models.DiscussionTopic, error) {
	if root == nil || root.RootTopicID != nil {
		return nil, nil
	}
	children, err := s.parity.ListChildTopics(ctx, root.ID)
	if err != nil {
		return nil, err
	}
	byGroup := map[uint]*models.DiscussionTopic{}
	for i := range children {
		if children[i].GroupID != nil {
			byGroup[*children[i].GroupID] = &children[i]
		}
	}

	wanted := map[uint]bool{}
	var live []models.DiscussionTopic
	if root.GroupCategoryID != nil && root.WorkflowState != models.DiscussionTopicDeleted {
		groups, err := s.parity.ListActiveGroups(ctx, *root.GroupCategoryID)
		if err != nil {
			return nil, err
		}
		for _, g := range groups {
			wanted[g.ID] = true
			if child, ok := byGroup[g.ID]; ok {
				if childNeedsSync(root, child) {
					copyIntoChild(root, child)
					if err := s.topicRepo.Update(ctx, child); err != nil {
						return nil, err
					}
				}
				live = append(live, *child)
				continue
			}
			gid := g.ID
			child := &models.DiscussionTopic{GroupID: &gid}
			copyIntoChild(root, child)
			if err := s.parity.CreateChildTopic(ctx, child); err != nil {
				return nil, err
			}
			live = append(live, *child)
		}
	}

	for gid, child := range byGroup {
		if wanted[gid] {
			continue
		}
		if root.WorkflowState == models.DiscussionTopicDeleted {
			_ = s.topicRepo.Delete(ctx, child.ID)
			continue
		}
		stats, err := s.parity.TopicStats(ctx, []uint{child.ID}, 0)
		if err == nil && stats[child.ID].EntryCount == 0 {
			_ = s.topicRepo.Delete(ctx, child.ID)
		}
	}
	return live, nil
}

// ChildTopics lists a root's live child topics.
func (s *DiscussionParityService) ChildTopics(ctx context.Context, rootID uint) ([]models.DiscussionTopic, error) {
	return s.parity.ListChildTopics(ctx, rootID)
}

// ChildTopicForUser returns the child topic of the user's group in the
// root's group set, or nil when the user is in none of its groups.
func (s *DiscussionParityService) ChildTopicForUser(ctx context.Context, root *models.DiscussionTopic, userID uint) (*models.DiscussionTopic, error) {
	if root.GroupCategoryID == nil {
		return nil, nil
	}
	groupIDs, err := s.parity.UserGroupIDs(ctx, *root.GroupCategoryID, userID)
	if err != nil || len(groupIDs) == 0 {
		return nil, err
	}
	find := func(children []models.DiscussionTopic) *models.DiscussionTopic {
		for _, gid := range groupIDs {
			for i := range children {
				if children[i].GroupID != nil && *children[i].GroupID == gid {
					return &children[i]
				}
			}
		}
		return nil
	}
	children, err := s.parity.ListChildTopics(ctx, root.ID)
	if err != nil {
		return nil, err
	}
	if child := find(children); child != nil {
		return child, nil
	}
	// The group was added after the topic was saved: create its topic.
	children, err = s.SyncGroupTopics(ctx, root)
	if err != nil {
		return nil, err
	}
	return find(children), nil
}

// ChildRoot returns the root topic of a group's child topic.
func (s *DiscussionParityService) ChildRoot(ctx context.Context, child *models.DiscussionTopic) (*models.DiscussionTopic, error) {
	if child == nil || child.RootTopicID == nil {
		return nil, nil
	}
	root := s.rootOf(ctx, child)
	if root == nil {
		return nil, errors.New("root topic not found")
	}
	return root, nil
}

// TopicStatsFor returns the entry / unread counters of the given topics
// for userID.
func (s *DiscussionParityService) TopicStatsFor(ctx context.Context, topics []models.DiscussionTopic, userID uint) map[uint]repository.DiscussionTopicStats {
	ids := make([]uint, len(topics))
	for i := range topics {
		ids[i] = topics[i].ID
	}
	stats, err := s.parity.TopicStats(ctx, ids, userID)
	if err != nil {
		return map[uint]repository.DiscussionTopicStats{}
	}
	return stats
}

// GroupNames maps the given child topics to their group's name.
func (s *DiscussionParityService) GroupNames(ctx context.Context, root *models.DiscussionTopic) map[uint]string {
	out := map[uint]string{}
	if root.GroupCategoryID == nil {
		return out
	}
	groups, err := s.parity.ListActiveGroups(ctx, *root.GroupCategoryID)
	if err != nil {
		return out
	}
	for _, g := range groups {
		out[g.ID] = g.Name
	}
	return out
}

// ---------------------------------------------------------------------------
// Graded discussions (G084)
// ---------------------------------------------------------------------------

// ValidateLinkedAssignment checks that an assignment_id sent on a topic
// names an assignment of the same course and tenant.
//
// Only a discussion_topic assignment no other topic uses may be linked:
// posts write discussion_topic submissions, which must never overwrite a
// student's submission to an ordinary assignment.
func (s *DiscussionParityService) ValidateLinkedAssignment(ctx context.Context, topicID, assignmentID, courseID, accountID uint) error {
	a, err := s.assignmentRepo.FindByID(ctx, assignmentID, accountID)
	if err != nil || a == nil || a.CourseID != courseID || a.WorkflowState == models.AssignmentDeleted ||
		!assignmentTakesDiscussionPosts(a) {
		return ErrDiscussionInvalidAssignment
	}
	if other, err := s.parity.FindTopicByAssignment(ctx, assignmentID, courseID); err == nil && other != nil && other.ID != topicID {
		return ErrDiscussionInvalidAssignment
	}
	return nil
}

// assignmentTakesDiscussionPosts reports whether the assignment's
// submission types include discussion_topic.
func assignmentTakesDiscussionPosts(a *models.Assignment) bool {
	for _, t := range strings.Split(a.SubmissionTypes, ",") {
		if strings.TrimSpace(t) == "discussion_topic" {
			return true
		}
	}
	return false
}

func assignmentStateForTopic(topic *models.DiscussionTopic) (models.AssignmentWorkflow, bool) {
	if topic.WorkflowState == models.DiscussionTopicUnpublished {
		return models.AssignmentUnpublished, false
	}
	return models.AssignmentPublished, true
}

// ValidateGradedInput checks a topic's "assignment" object before the
// topic is saved.
func (s *DiscussionParityService) ValidateGradedInput(ctx context.Context, courseID uint, in *DiscussionGradedInput) error {
	if in == nil {
		return nil
	}
	if in.PointsPossible != nil && *in.PointsPossible < 0 {
		return ErrDiscussionInvalidPoints
	}
	switch strings.TrimSpace(in.GradingType) {
	case "", "points", "percent", "letter_grade", "gpa_scale", "pass_fail":
	default:
		return errors.New("grading_type is not supported")
	}
	if in.AssignmentGroupID != nil {
		ok, err := s.parity.AssignmentGroupInCourse(ctx, *in.AssignmentGroupID, courseID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrDiscussionInvalidAssignment
		}
	}
	return nil
}

// TopicHasPosts reports whether anyone has posted in the topic or, for a
// group discussion, in any of its group topics.
func (s *DiscussionParityService) TopicHasPosts(ctx context.Context, topic *models.DiscussionTopic) (bool, error) {
	ids := []uint{topic.ID}
	if children, err := s.parity.ListChildTopics(ctx, topic.ID); err == nil {
		for _, c := range children {
			ids = append(ids, c.ID)
		}
	}
	stats, err := s.parity.TopicStats(ctx, ids, 0)
	if err != nil {
		return false, err
	}
	for _, st := range stats {
		if st.EntryCount > 0 {
			return true, nil
		}
	}
	return false, nil
}

// ApplyGraded makes the topic graded: it creates the backing assignment
// (submission type discussion_topic) or updates the one already linked.
// The topic is not saved; the caller persists topic.AssignmentID.
func (s *DiscussionParityService) ApplyGraded(ctx context.Context, topic *models.DiscussionTopic, in *DiscussionGradedInput, accountID uint) error {
	if in == nil {
		return nil
	}
	if in.PointsPossible != nil && *in.PointsPossible < 0 {
		return ErrDiscussionInvalidPoints
	}
	if in.AssignmentGroupID != nil {
		ok, err := s.parity.AssignmentGroupInCourse(ctx, *in.AssignmentGroupID, topic.CourseID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrDiscussionInvalidAssignment
		}
	}
	gradingType := strings.TrimSpace(in.GradingType)
	switch gradingType {
	case "":
		gradingType = "points"
	case "points", "percent", "letter_grade", "gpa_scale", "pass_fail":
	default:
		return errors.New("grading_type is not supported")
	}

	state, published := assignmentStateForTopic(topic)

	if topic.AssignmentID != nil {
		a, err := s.assignmentRepo.FindByID(ctx, *topic.AssignmentID, accountID)
		if err == nil && a != nil && a.CourseID == topic.CourseID && a.WorkflowState != models.AssignmentDeleted {
			a.Name = topic.Title
			a.Description = topic.Message
			if in.PointsPossible != nil {
				a.PointsPossible = in.PointsPossible
			}
			// Only what the request names changes: an edit from the topic
			// form (points and due date) must not reset a grading type or
			// availability dates set elsewhere.
			if strings.TrimSpace(in.GradingType) != "" {
				a.GradingType = gradingType
			}
			if in.has("due_at") {
				a.DueAt = in.DueAt
			}
			if in.has("unlock_at") {
				a.UnlockAt = in.UnlockAt
			}
			if in.has("lock_at") {
				a.LockAt = in.LockAt
			}
			if in.AssignmentGroupID != nil {
				a.AssignmentGroupID = in.AssignmentGroupID
			}
			a.SubmissionTypes = "discussion_topic"
			a.WorkflowState = state
			a.Published = published
			return s.assignmentRepo.Update(ctx, a)
		}
	}

	points := 0.0
	if in.PointsPossible != nil {
		points = *in.PointsPossible
	}
	a := &models.Assignment{
		CourseID:          topic.CourseID,
		Name:              topic.Title,
		Description:       topic.Message,
		DueAt:             in.DueAt,
		UnlockAt:          in.UnlockAt,
		LockAt:            in.LockAt,
		PointsPossible:    &points,
		GradingType:       gradingType,
		SubmissionTypes:   "discussion_topic",
		AssignmentGroupID: in.AssignmentGroupID,
		WorkflowState:     state,
		Published:         published,
	}
	if err := s.assignmentRepo.Create(ctx, a); err != nil {
		return err
	}
	id := a.ID
	topic.AssignmentID = &id
	return nil
}

// RemoveGraded unlinks and deletes the topic's assignment. Refused once
// a student has submitted (posted) or been graded.
func (s *DiscussionParityService) RemoveGraded(ctx context.Context, topic *models.DiscussionTopic) error {
	if topic.AssignmentID == nil {
		return nil
	}
	n, err := s.parity.CountActiveSubmissions(ctx, *topic.AssignmentID)
	if err != nil {
		return err
	}
	if n > 0 {
		return ErrDiscussionGradedHasSubmission
	}
	if err := s.assignmentRepo.Delete(ctx, *topic.AssignmentID); err != nil {
		return err
	}
	topic.AssignmentID = nil
	return nil
}

// SyncAssignmentWithTopic keeps a graded topic's assignment title,
// description and publish state in step with the topic (topic edits
// that don't carry an assignment object), and deletes it with the topic.
func (s *DiscussionParityService) SyncAssignmentWithTopic(ctx context.Context, topic *models.DiscussionTopic) {
	if topic == nil || topic.AssignmentID == nil {
		return
	}
	// accountID=0: internal hop; the topic (and so its linked assignment,
	// validated on link) was resolved in the caller's tenant.
	a, err := s.assignmentRepo.FindByID(ctx, *topic.AssignmentID, 0)
	if err != nil || a == nil || a.CourseID != topic.CourseID || a.WorkflowState == models.AssignmentDeleted {
		return
	}
	if topic.WorkflowState == models.DiscussionTopicDeleted {
		_ = s.assignmentRepo.Delete(ctx, a.ID)
		return
	}
	state, published := assignmentStateForTopic(topic)
	if a.Name == topic.Title && a.Description == topic.Message && a.WorkflowState == state && a.Published == published {
		return
	}
	a.Name = topic.Title
	a.Description = topic.Message
	a.WorkflowState = state
	a.Published = published
	_ = s.assignmentRepo.Update(ctx, a)
}

// GradedAssignment returns the topic's live assignment, if any.
func (s *DiscussionParityService) GradedAssignment(ctx context.Context, topic *models.DiscussionTopic) *models.Assignment {
	if topic == nil || topic.AssignmentID == nil {
		return nil
	}
	a, err := s.assignmentRepo.FindByID(ctx, *topic.AssignmentID, 0)
	if err != nil || a == nil || a.CourseID != topic.CourseID || a.WorkflowState == models.AssignmentDeleted {
		return nil
	}
	return a
}

// rootOf returns the root topic of a group child (or the topic itself).
func (s *DiscussionParityService) rootOf(ctx context.Context, topic *models.DiscussionTopic) *models.DiscussionTopic {
	if topic.RootTopicID == nil {
		return topic
	}
	root, err := s.topicRepo.FindByID(ctx, *topic.RootTopicID, 0)
	if err != nil || root == nil || root.CourseID != topic.CourseID {
		return nil
	}
	return root
}

// gradedTopicIDs lists the topics whose posts count toward the root's
// assignment: the root and its group child topics.
func (s *DiscussionParityService) gradedTopicIDs(ctx context.Context, root *models.DiscussionTopic) []uint {
	ids := []uint{root.ID}
	if root.GroupCategoryID != nil {
		if children, err := s.parity.ListChildTopics(ctx, root.ID); err == nil {
			for _, c := range children {
				ids = append(ids, c.ID)
			}
		}
	}
	return ids
}

// SyncSubmissionForUser mirrors Canvas DiscussionTopic#ensure_submission:
// on a graded discussion a student's posts make a discussion_topic
// submission, submitted at their first post; once every post is gone
// the (ungraded) submission goes back to unsubmitted. Only active
// students of the course get submissions.
func (s *DiscussionParityService) SyncSubmissionForUser(ctx context.Context, topic *models.DiscussionTopic, userID uint) error {
	root := s.rootOf(ctx, topic)
	if root == nil || root.AssignmentID == nil || userID == 0 {
		return nil
	}
	assignment := s.GradedAssignment(ctx, root)
	if assignment == nil || !assignmentTakesDiscussionPosts(assignment) {
		return nil
	}
	student, err := s.parity.IsActiveStudent(ctx, root.CourseID, userID)
	if err != nil || !student {
		return err
	}
	entries, err := s.parity.ListUserEntries(ctx, s.gradedTopicIDs(ctx, root), userID)
	if err != nil {
		return err
	}
	// accountID=0: internal hop; the assignment belongs to the topic's
	// course (checked above) and (assignment_id, user_id) names the row.
	existing, _ := s.submissionRepo.FindByAssignmentAndUser(ctx, assignment.ID, userID, 0)

	if len(entries) == 0 {
		if existing == nil || existing.WorkflowState != models.SubmissionSubmitted ||
			existing.SubmissionType == nil || *existing.SubmissionType != "discussion_topic" {
			return nil
		}
		existing.WorkflowState = models.SubmissionUnsubmitted
		existing.SubmittedAt = nil
		existing.SubmissionType = nil
		existing.Late = false
		return s.submissionRepo.Update(ctx, existing)
	}

	submittedAt := entries[0].CreatedAt
	late := assignment.DueAt != nil && submittedAt.After(*assignment.DueAt)
	subType := "discussion_topic"
	if existing != nil {
		if existing.SubmittedAt != nil && existing.SubmissionType != nil && *existing.SubmissionType == subType {
			return nil
		}
		existing.SubmissionType = &subType
		existing.SubmittedAt = &submittedAt
		existing.Late = late
		if existing.Attempt == 0 {
			existing.Attempt = 1
		}
		if existing.WorkflowState != models.SubmissionGraded {
			existing.WorkflowState = models.SubmissionSubmitted
		}
		return s.submissionRepo.Update(ctx, existing)
	}
	return s.submissionRepo.Create(ctx, &models.Submission{
		AssignmentID:   assignment.ID,
		UserID:         userID,
		SubmissionType: &subType,
		SubmittedAt:    &submittedAt,
		Attempt:        1,
		Late:           late,
		WorkflowState:  models.SubmissionSubmitted,
	})
}

// SubmissionCallback is an OnEntryCreated callback that keeps graded
// discussion submissions in step with new posts.
func (s *DiscussionParityService) SubmissionCallback() DiscussionEntryPostedCallback {
	return func(ctx context.Context, entryID uint) {
		// accountID=0: background hop from a just-written entry.
		entry, err := s.entryRepo.FindByID(ctx, entryID, 0)
		if err != nil || entry == nil {
			return
		}
		topic, err := s.topicRepo.FindByID(ctx, entry.DiscussionTopicID, 0)
		if err != nil || topic == nil {
			return
		}
		_ = s.SyncSubmissionForUser(ctx, topic, entry.UserID)
	}
}

// StudentEntries returns a student's live posts on a graded discussion
// (the root and its group topics) for SpeedGrader, with the group name
// of each child topic.
func (s *DiscussionParityService) StudentEntries(ctx context.Context, root *models.DiscussionTopic, userID uint) ([]models.DiscussionEntry, map[uint]string, error) {
	ids := s.gradedTopicIDs(ctx, root)
	entries, err := s.parity.ListUserEntries(ctx, ids, userID)
	if err != nil {
		return nil, nil, err
	}
	topicGroup := map[uint]string{}
	if root.GroupCategoryID != nil {
		names := s.GroupNames(ctx, root)
		if children, err := s.parity.ListChildTopics(ctx, root.ID); err == nil {
			for _, c := range children {
				if c.GroupID != nil {
					topicGroup[c.ID] = names[*c.GroupID]
				}
			}
		}
	}
	return entries, topicGroup, nil
}

// FindTopicByAssignment returns the graded discussion backing an
// assignment of the course.
func (s *DiscussionParityService) FindTopicByAssignment(ctx context.Context, assignmentID, courseID uint) (*models.DiscussionTopic, error) {
	return s.parity.FindTopicByAssignment(ctx, assignmentID, courseID)
}

// ---------------------------------------------------------------------------
// Anonymity (G087)
// ---------------------------------------------------------------------------

// EntryIsAnonymous reports whether the entry's author is hidden.
func EntryIsAnonymous(topic *models.DiscussionTopic, entry *models.DiscussionEntry) bool {
	if topic == nil || topic.AnonymousState == nil {
		return false
	}
	if *topic.AnonymousState == models.DiscussionFullAnonymity {
		return true
	}
	return *topic.AnonymousState == models.DiscussionPartialAnonymity && entry != nil && entry.IsAnonymousAuthor
}

// AnonymousAliases returns the alias of each user in the topic.
func (s *DiscussionParityService) AnonymousAliases(ctx context.Context, topic *models.DiscussionTopic, userIDs []uint) map[uint]string {
	out, err := s.parity.AnonymousIDs(ctx, topic.ID, discussionUniqueUints(userIDs))
	if err != nil {
		return map[uint]string{}
	}
	return out
}

// AnonymousName is the display name for an alias.
func AnonymousName(alias string) string {
	if alias == "" {
		return "Anonymous"
	}
	return "Anonymous " + alias
}

// ---------------------------------------------------------------------------
// Subscriptions and @mentions (G088)
// ---------------------------------------------------------------------------

// subscribers resolves who is subscribed to a topic, following Canvas
// DiscussionTopic#subscribers: an explicit participant row wins; without
// one, the topic author and everyone who has posted are subscribed.
func (s *DiscussionParityService) subscribers(ctx context.Context, topic *models.DiscussionTopic) ([]uint, error) {
	explicit := map[uint]bool{}
	if parts, err := s.topicParticipantRepo.ListByTopicID(ctx, topic.ID); err == nil {
		for _, p := range parts {
			explicit[p.UserID] = p.Subscribed
		}
	}
	candidates := map[uint]bool{}
	if topic.UserID != 0 {
		candidates[topic.UserID] = true
	}
	posters, err := s.parity.ListPosterIDs(ctx, topic.ID)
	if err != nil {
		return nil, err
	}
	for _, id := range posters {
		candidates[id] = true
	}
	for id, sub := range explicit {
		if sub {
			candidates[id] = true
		}
	}
	var out []uint
	for id := range candidates {
		if sub, ok := explicit[id]; ok && !sub {
			continue
		}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

// SubscribeAuthor subscribes a poster to the topic, as Canvas does on
// every new entry (DiscussionEntry#subscribe_author): posting re-subscribes
// someone who had unsubscribed, or whose participant row was seeded
// unsubscribed before they posted (e.g. by "mark all as read"), so they
// hear about the replies to what they wrote.
func (s *DiscussionParityService) SubscribeAuthor(ctx context.Context, topicID, userID uint) error {
	if s.topicParticipantRepo == nil || topicID == 0 || userID == 0 {
		return nil
	}
	return s.topicParticipantRepo.UpdateSubscription(ctx, topicID, userID, true)
}

// audienceFilter keeps the users who can still open the topic: active
// members of the course, and for non-staff the section / group rules.
func (s *DiscussionParityService) audienceFilter(ctx context.Context, topic *models.DiscussionTopic, userIDs []uint) []uint {
	active, err := s.parity.ActiveCourseUserIDs(ctx, topic.CourseID, userIDs)
	if err != nil {
		return nil
	}
	staff := map[uint]bool{}
	if ids, err := s.parity.CourseStaffIDs(ctx, topic.CourseID); err == nil {
		for _, id := range ids {
			staff[id] = true
		}
	}
	now := s.now()
	var out []uint
	for _, id := range active {
		if staff[id] {
			out = append(out, id)
			continue
		}
		if !DiscussionTopicVisibleToStudents(topic, now) || !s.TopicVisibleToUser(ctx, topic, id) {
			continue
		}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func notificationTitle(topic *models.DiscussionTopic) string {
	return topic.Title
}

// EntryNotificationCallback is the OnEntryCreated callback that notifies
// the topic's subscribers (Canvas "Discussion Entry" notifications). It
// replaces notifying only the parent author: someone who unsubscribed is
// left alone, and the poster is never notified of their own post.
func (s *DiscussionParityService) EntryNotificationCallback() DiscussionEntryPostedCallback {
	return func(ctx context.Context, entryID uint) {
		if s.notifier == nil {
			return
		}
		// accountID=0: background hop from a just-written, tenant-checked entry.
		entry, err := s.entryRepo.FindByID(ctx, entryID, 0)
		if err != nil || entry == nil {
			return
		}
		topic, err := s.topicRepo.FindByID(ctx, entry.DiscussionTopicID, 0)
		if err != nil || topic == nil {
			return
		}
		subs, err := s.subscribers(ctx, topic)
		if err != nil {
			return
		}
		var recipients []uint
		for _, id := range subs {
			if id != entry.UserID {
				recipients = append(recipients, id)
			}
		}
		subject := fmt.Sprintf("New post in %s", notificationTitle(topic))
		if entry.ParentID != nil {
			subject = fmt.Sprintf("New reply in %s", notificationTitle(topic))
		}
		for _, id := range s.audienceFilter(ctx, topic, recipients) {
			_ = s.notifier.QueueNotification(ctx, id, "discussion_reply", subject, entry.Message, "Course", topic.CourseID)
		}
	}
}

// mentionText is the plain text of a post, for matching "@Name".
func mentionText(message string) string {
	return strings.ToLower(html.UnescapeString(htmlsanitize.PlainText(message)))
}

// ProcessMentions notifies the users a post @mentions (Canvas
// DiscussionMention). mentionedIDs come from the editor's mention
// picker; a user is notified only when "@<their name>" is in the post,
// they can open the topic, and this post hasn't already notified them
// (so an edit notifies only newly mentioned people). Returns the ids
// that were notified.
func (s *DiscussionParityService) ProcessMentions(ctx context.Context, topic *models.DiscussionTopic, entry *models.DiscussionEntry, mentionedIDs []uint) ([]uint, error) {
	ids := discussionUniqueUints(mentionedIDs)
	if len(ids) == 0 || topic == nil || entry == nil {
		return nil, nil
	}
	if len(ids) > 50 {
		ids = ids[:50]
	}
	already, err := s.parity.ListMentionedUserIDs(ctx, entry.ID)
	if err != nil {
		return nil, err
	}
	skip := map[uint]bool{entry.UserID: true}
	for _, id := range already {
		skip[id] = true
	}
	var fresh []uint
	for _, id := range ids {
		if !skip[id] {
			fresh = append(fresh, id)
		}
	}
	if len(fresh) == 0 {
		return nil, nil
	}
	names, err := s.parity.UserNames(ctx, append(fresh, entry.UserID))
	if err != nil {
		return nil, err
	}
	text := mentionText(entry.Message)
	var named []uint
	for _, id := range fresh {
		name := strings.ToLower(strings.TrimSpace(names[id]))
		if name != "" && strings.Contains(text, "@"+name) {
			named = append(named, id)
		}
	}
	recipients := s.audienceFilter(ctx, topic, named)
	if len(recipients) == 0 {
		return nil, nil
	}
	if err := s.parity.AddMentions(ctx, entry.ID, recipients); err != nil {
		return nil, err
	}
	who := names[entry.UserID]
	if EntryIsAnonymous(topic, entry) || who == "" {
		who = "Someone"
	}
	subject := fmt.Sprintf("%s mentioned you in %s", who, notificationTitle(topic))
	if s.notifier != nil {
		for _, id := range recipients {
			_ = s.notifier.QueueNotification(ctx, id, "discussion_mention", subject, entry.Message, "Course", topic.CourseID)
		}
	}
	return recipients, nil
}

// NotifyNewTopic tells the course about a newly available topic (Canvas
// "Discussion" notifications). Delayed or unpublished topics are not
// announced here.
func (s *DiscussionParityService) NotifyNewTopic(ctx context.Context, topic *models.DiscussionTopic) {
	if s.notifier == nil || topic == nil || topic.RootTopicID != nil {
		return
	}
	if !DiscussionTopicVisibleToStudents(topic, s.now()) {
		return
	}
	users, err := s.parity.ListActiveCourseUserIDs(ctx, topic.CourseID)
	if err != nil {
		return
	}
	ids := make([]uint, 0, len(users))
	for _, id := range users {
		if id != topic.UserID {
			ids = append(ids, id)
		}
	}
	// A group discussion is announced through its root, which is what
	// the index lists; members then land on their group's topic.
	subject := fmt.Sprintf("New discussion: %s", topic.Title)
	for _, id := range s.audienceFilter(ctx, topic, ids) {
		_ = s.notifier.QueueNotification(ctx, id, "new_discussion_topic", subject, topic.Message, "Course", topic.CourseID)
	}
}

// ReleaseDelayedTopics moves topics whose delayed_post_at has passed
// from post_delayed to active (with their group topics and graded
// assignment) and announces them, like Canvas's delayed posting job.
// Visibility already follows the date; this keeps the state honest and
// sends the "new discussion" notification on time. Returns how many
// topics were released.
func (s *DiscussionParityService) ReleaseDelayedTopics(ctx context.Context) (int, error) {
	due, err := s.parity.ListDueDelayedTopics(ctx, s.now(), 200)
	if err != nil {
		return 0, err
	}
	released := 0
	for i := range due {
		topic := &due[i]
		topic.WorkflowState = models.DiscussionTopicActive
		if err := s.topicRepo.Update(ctx, topic); err != nil {
			return released, err
		}
		released++
		if topic.GroupCategoryID != nil {
			_, _ = s.SyncGroupTopics(ctx, topic)
		}
		s.SyncAssignmentWithTopic(ctx, topic)
		s.NotifyNewTopic(ctx, topic)
	}
	return released, nil
}

// ReleaseDelayedTopicsJob is ReleaseDelayedTopics as a scheduler job.
func (s *DiscussionParityService) ReleaseDelayedTopicsJob(ctx context.Context) error {
	_, err := s.ReleaseDelayedTopics(ctx)
	return err
}

// MentionableUsers powers the @mention picker: active members of the
// course matching search who can open the topic (at most 20).
func (s *DiscussionParityService) MentionableUsers(ctx context.Context, topic *models.DiscussionTopic, search string) ([]repository.DiscussionUserBrief, error) {
	users, err := s.parity.SearchCourseUsers(ctx, topic.CourseID, search, 50)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, len(users))
	for i, u := range users {
		ids[i] = u.ID
	}
	allowed := map[uint]bool{}
	for _, id := range s.audienceFilter(ctx, topic, ids) {
		allowed[id] = true
	}
	out := make([]repository.DiscussionUserBrief, 0, len(users))
	for _, u := range users {
		if allowed[u.ID] {
			out = append(out, u)
		}
		if len(out) == 20 {
			break
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Index (G089)
// ---------------------------------------------------------------------------

// ListIndex returns a page of the course's discussion index with the
// viewer's reply and unread counts. A group discussion's counts add up
// the group topics the viewer can see (every group for staff, their own
// groups for a student).
func (s *DiscussionParityService) ListIndex(ctx context.Context, q repository.DiscussionIndexQuery) ([]DiscussionIndexItem, int64, error) {
	if q.Now.IsZero() {
		q.Now = s.now()
	}
	res, err := s.parity.ListTopicsIndex(ctx, q)
	if err != nil {
		return nil, 0, err
	}

	statIDs := []uint{}
	childrenOf := map[uint][]uint{}
	for _, t := range res.Items {
		statIDs = append(statIDs, t.ID)
		if t.GroupCategoryID == nil {
			continue
		}
		children, err := s.parity.ListChildTopics(ctx, t.ID)
		if err != nil {
			continue
		}
		var mine map[uint]bool
		if q.StudentView {
			mine = map[uint]bool{}
			if gids, err := s.parity.UserGroupIDs(ctx, *t.GroupCategoryID, q.UserID); err == nil {
				for _, g := range gids {
					mine[g] = true
				}
			}
		}
		for _, c := range children {
			if mine != nil && (c.GroupID == nil || !mine[*c.GroupID]) {
				continue
			}
			childrenOf[t.ID] = append(childrenOf[t.ID], c.ID)
			statIDs = append(statIDs, c.ID)
		}
	}
	stats, err := s.parity.TopicStats(ctx, statIDs, q.UserID)
	if err != nil {
		return nil, 0, err
	}
	var sections map[uint][]uint
	if !q.StudentView {
		ids := make([]uint, 0, len(res.Items))
		for _, t := range res.Items {
			if t.IsSectionSpecific {
				ids = append(ids, t.ID)
			}
		}
		sections = s.TopicSectionIDs(ctx, ids)
	}

	items := make([]DiscussionIndexItem, 0, len(res.Items))
	for _, t := range res.Items {
		item := DiscussionIndexItem{Topic: t, SectionIDs: sections[t.ID]}
		add := func(st repository.DiscussionTopicStats) {
			item.EntryCount += st.EntryCount
			item.UnreadCount += st.UnreadCount
			if st.LastReplyAt != nil && (item.LastReplyAt == nil || st.LastReplyAt.After(*item.LastReplyAt)) {
				lr := *st.LastReplyAt
				item.LastReplyAt = &lr
			}
		}
		add(stats[t.ID])
		for _, cid := range childrenOf[t.ID] {
			add(stats[cid])
		}
		item.GroupTopics = len(childrenOf[t.ID])
		items = append(items, item)
	}
	return items, res.TotalCount, nil
}

func discussionUniqueUints(in []uint) []uint {
	seen := map[uint]bool{}
	out := make([]uint, 0, len(in))
	for _, v := range in {
		if v == 0 || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// discussionAudienceHolder is embedded in the discussion services so the
// shared topic resolver (handlers.requireTopicInCourse) can apply the
// section / group visibility rules whichever service it is handed.
type discussionAudienceHolder struct {
	audience DiscussionAudience
}

// SetAudience installs the section / group visibility rules.
func (h *discussionAudienceHolder) SetAudience(a DiscussionAudience) { h.audience = a }

// Audience returns the installed visibility rules (nil when none).
func (h *discussionAudienceHolder) Audience() DiscussionAudience { return h.audience }
