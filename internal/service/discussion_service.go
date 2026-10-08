package service

import (
	"context"
	"errors"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// ErrDiscussionInvalidRating is returned when a rating other than 0
// (unlike) or 1 (like) is submitted. Canvas's rate_entry API accepts
// only those two values; anything else let a single user inflate a
// post's rating_sum arbitrarily.
var ErrDiscussionInvalidRating = errors.New("rating must be 0 or 1")

// ratingRepoWithSum extends the base interface with aggregation support.
// The postgres implementation provides this method on the concrete type.
type ratingRepoWithSum interface {
	repository.DiscussionEntryRatingRepository
	SumByEntryID(ctx context.Context, entryID uint) (count int64, sum int64, err error)
}

// DiscussionEntryPostedCallback fires (asynchronously) after a discussion
// entry is successfully created. Receives the new entry's ID. Fires for
// every entry — top-level posts and replies alike — so the rule-side
// predicate can filter on parent_id if it only cares about replies.
// Same contract as SubmissionGradedCallback: detached context, no panic,
// no error return.
type DiscussionEntryPostedCallback func(ctx context.Context, entryID uint)

type DiscussionService struct {
	topicRepo  repository.DiscussionTopicRepository
	entryRepo  repository.DiscussionEntryRepository
	ratingRepo ratingRepoWithSum

	// onEntryCreatedCallbacks fire (in goroutines) after a successful
	// CreateEntry. Registered via OnEntryCreated.
	onEntryCreatedCallbacks []DiscussionEntryPostedCallback
}

func NewDiscussionService(
	topicRepo repository.DiscussionTopicRepository,
	entryRepo repository.DiscussionEntryRepository,
	ratingRepo ratingRepoWithSum,
) *DiscussionService {
	return &DiscussionService{
		topicRepo:  topicRepo,
		entryRepo:  entryRepo,
		ratingRepo: ratingRepo,
	}
}

// OnEntryCreated registers a callback to fire after a successful discussion
// entry write. Callbacks run in fresh goroutines with a detached context;
// panics are recovered.
func (s *DiscussionService) OnEntryCreated(cb DiscussionEntryPostedCallback) {
	s.onEntryCreatedCallbacks = append(s.onEntryCreatedCallbacks, cb)
}

func (s *DiscussionService) fireOnEntryCreated(entryID uint) {
	for _, cb := range s.onEntryCreatedCallbacks {
		go func(cb DiscussionEntryPostedCallback) {
			defer recoverFromPanic("discussion OnEntryCreated callback")
			cb(context.Background(), entryID)
		}(cb)
	}
}

// Topic methods

func (s *DiscussionService) CreateTopic(ctx context.Context, topic *models.DiscussionTopic) error {
	if topic.Title == "" {
		return errors.New("discussion topic title is required")
	}
	if topic.WorkflowState == "" {
		topic.WorkflowState = models.DiscussionTopicActive
	}
	if topic.DiscussionType == "" {
		topic.DiscussionType = "side_comment"
	}
	return s.topicRepo.Create(ctx, topic)
}

// GetTopic — Wave F. See CourseService.GetByID for the accountID contract.
func (s *DiscussionService) GetTopic(ctx context.Context, id, accountID uint) (*models.DiscussionTopic, error) {
	return s.topicRepo.FindByID(ctx, id, accountID)
}

func (s *DiscussionService) UpdateTopic(ctx context.Context, topic *models.DiscussionTopic) error {
	return s.topicRepo.Update(ctx, topic)
}

// DeleteTopic soft-deletes a topic by id. The repo UPDATE is keyed on id
// only, so callers MUST have already verified the topic belongs to the
// caller's tenant and course (see handlers.requireTopicInCourse).
func (s *DiscussionService) DeleteTopic(ctx context.Context, id uint) error {
	return s.topicRepo.Delete(ctx, id)
}

func (s *DiscussionService) ListTopics(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DiscussionTopic], error) {
	return s.topicRepo.ListByCourseID(ctx, courseID, 0, params)
}

// Entry methods

func (s *DiscussionService) CreateEntry(ctx context.Context, entry *models.DiscussionEntry) error {
	if entry.Message == "" {
		return errors.New("discussion entry message is required")
	}
	if entry.WorkflowState == "" {
		entry.WorkflowState = "active"
	}
	if err := s.entryRepo.Create(ctx, entry); err != nil {
		return err
	}
	s.fireOnEntryCreated(entry.ID)
	return nil
}

// GetEntry loads an entry scoped to accountID (entry → topic → course →
// account). Handler-layer callers MUST pass callerAccountID(c).
func (s *DiscussionService) GetEntry(ctx context.Context, id, accountID uint) (*models.DiscussionEntry, error) {
	return s.entryRepo.FindByID(ctx, id, accountID)
}

func (s *DiscussionService) UpdateEntry(ctx context.Context, entry *models.DiscussionEntry) error {
	return s.entryRepo.Update(ctx, entry)
}

// DeleteEntry soft-deletes an entry by id. Callers MUST have verified
// the entry belongs to the topic/course/tenant first.
func (s *DiscussionService) DeleteEntry(ctx context.Context, id uint) error {
	return s.entryRepo.Delete(ctx, id)
}

func (s *DiscussionService) ListEntries(ctx context.Context, topicID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DiscussionEntry], error) {
	return s.entryRepo.ListByTopicID(ctx, topicID, accountID, params)
}

func (s *DiscussionService) ListReplies(ctx context.Context, entryID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DiscussionEntry], error) {
	return s.entryRepo.ListReplies(ctx, entryID, accountID, params)
}

// Full view with nested tree

type EntryView struct {
	ID                uint        `json:"id"`
	DiscussionTopicID uint        `json:"discussion_topic_id"`
	UserID            uint        `json:"user_id"`
	ParentID          *uint       `json:"parent_id"`
	Message           string      `json:"message"`
	RatingCount       int         `json:"rating_count"`
	RatingSum         int         `json:"rating_sum"`
	WorkflowState     string      `json:"workflow_state"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
	Replies           []EntryView `json:"replies"`
}

type DiscussionFullView struct {
	Topic   *models.DiscussionTopic `json:"topic"`
	Entries []EntryView             `json:"entries"`
}

// GetFullView returns the threaded view of a topic the caller has already
// resolved (tenant + course verified by the handler). Entries are loaded
// with the same accountID scope as a second line of defense.
func (s *DiscussionService) GetFullView(ctx context.Context, topic *models.DiscussionTopic, accountID uint) (*DiscussionFullView, error) {
	allEntries, err := s.entryRepo.ListAllByTopicIDIncludingDeleted(ctx, topic.ID, accountID)
	if err != nil {
		return nil, err
	}

	rootEntries := buildTree(visibleThreadEntries(allEntries))

	return &DiscussionFullView{
		Topic:   topic,
		Entries: rootEntries,
	}, nil
}

func buildTree(entries []models.DiscussionEntry) []EntryView {
	return assembleThread(entries, func(e models.DiscussionEntry) EntryView {
		return EntryView{
			ID:                e.ID,
			DiscussionTopicID: e.DiscussionTopicID,
			UserID:            e.UserID,
			ParentID:          e.ParentID,
			Message:           e.Message,
			RatingCount:       e.RatingCount,
			RatingSum:         e.RatingSum,
			WorkflowState:     e.WorkflowState,
			CreatedAt:         e.CreatedAt,
			UpdatedAt:         e.UpdatedAt,
			Replies:           []EntryView{},
		}
	}, func(parent *EntryView, replies []EntryView) {
		parent.Replies = replies
	})
}

// discussionEntryDeleted is the soft-delete workflow_state on entries.
const discussionEntryDeleted = "deleted"

// visibleThreadEntries filters a topic's full entry list (soft-deleted
// rows included) down to what a thread view renders: every live entry,
// plus each deleted entry that still has a live descendant. Those are
// kept as placeholders with the message blanked, so their replies stay
// nested under them instead of being promoted to top-level posts (the
// Canvas "This entry has been deleted" behavior). Deleted entries with
// no live descendants are dropped.
func visibleThreadEntries(all []models.DiscussionEntry) []models.DiscussionEntry {
	index := make(map[uint]int, len(all))
	for i, e := range all {
		index[e.ID] = i
	}

	hasLiveDescendant := make(map[uint]bool)
	for _, e := range all {
		if e.WorkflowState == discussionEntryDeleted {
			continue
		}
		// Walk up the ancestor chain. The step bound guards against a
		// malformed parent cycle.
		parent := e.ParentID
		for steps := 0; parent != nil && steps < len(all); steps++ {
			if hasLiveDescendant[*parent] {
				break
			}
			hasLiveDescendant[*parent] = true
			i, ok := index[*parent]
			if !ok {
				break
			}
			parent = all[i].ParentID
		}
	}

	out := make([]models.DiscussionEntry, 0, len(all))
	for _, e := range all {
		if e.WorkflowState != discussionEntryDeleted {
			out = append(out, e)
			continue
		}
		if hasLiveDescendant[e.ID] {
			placeholder := e
			placeholder.Message = ""
			placeholder.RatingCount = 0
			placeholder.RatingSum = 0
			out = append(out, placeholder)
		}
	}
	return out
}

// assembleThread nests entries under their parents. Children are
// attached recursively (not copied into the parent while the child is
// still being filled in), so replies at any depth are kept. An entry
// whose parent is absent from the list is treated as a root.
func assembleThread[T any](entries []models.DiscussionEntry, mk func(models.DiscussionEntry) T, setReplies func(parent *T, replies []T)) []T {
	present := make(map[uint]bool, len(entries))
	for _, e := range entries {
		present[e.ID] = true
	}

	children := make(map[uint][]models.DiscussionEntry)
	var roots []models.DiscussionEntry
	for _, e := range entries {
		if e.ParentID != nil && *e.ParentID != e.ID && present[*e.ParentID] {
			children[*e.ParentID] = append(children[*e.ParentID], e)
		} else {
			roots = append(roots, e)
		}
	}

	visited := make(map[uint]bool, len(entries))
	var build func(e models.DiscussionEntry) T
	build = func(e models.DiscussionEntry) T {
		visited[e.ID] = true
		view := mk(e)
		kids := children[e.ID]
		replies := make([]T, 0, len(kids))
		for _, k := range kids {
			if visited[k.ID] {
				continue
			}
			replies = append(replies, build(k))
		}
		setReplies(&view, replies)
		return view
	}

	result := make([]T, 0, len(roots))
	for _, r := range roots {
		result = append(result, build(r))
	}
	return result
}

// Rating

// RateEntry sets (rating=1) or clears (rating=0) the user's like on an
// entry and recomputes the entry's rating_count / rating_sum. Callers
// MUST have verified the entry is in a topic the user can see and that
// the topic allows the user to rate (allow_rating / only_graders_can_rate).
func (s *DiscussionService) RateEntry(ctx context.Context, entryID uint, userID uint, rating int) error {
	switch rating {
	case 0:
		if err := s.ratingRepo.Delete(ctx, entryID, userID); err != nil {
			return err
		}
	case 1:
		r := &models.DiscussionEntryRating{
			DiscussionEntryID: entryID,
			UserID:            userID,
			Rating:            rating,
		}
		if err := s.ratingRepo.Upsert(ctx, r); err != nil {
			return err
		}
	default:
		return ErrDiscussionInvalidRating
	}

	// Recalculate entry rating_count and rating_sum
	count, sum, err := s.ratingRepo.SumByEntryID(ctx, entryID)
	if err != nil {
		return err
	}

	// accountID=0: internal hop; the handler already resolved this entry
	// with the caller's tenant scope.
	entry, err := s.entryRepo.FindByID(ctx, entryID, 0)
	if err != nil {
		return err
	}

	entry.RatingCount = int(count)
	entry.RatingSum = int(sum)
	return s.entryRepo.Update(ctx, entry)
}
