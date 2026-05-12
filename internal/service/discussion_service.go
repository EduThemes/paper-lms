package service

import (
	"context"
	"errors"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

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
		topic.WorkflowState = "active"
	}
	if topic.DiscussionType == "" {
		topic.DiscussionType = "side_comment"
	}
	return s.topicRepo.Create(ctx, topic)
}

func (s *DiscussionService) GetTopic(ctx context.Context, id uint) (*models.DiscussionTopic, error) {
	return s.topicRepo.FindByID(ctx, id)
}

func (s *DiscussionService) UpdateTopic(ctx context.Context, topic *models.DiscussionTopic) error {
	return s.topicRepo.Update(ctx, topic)
}

func (s *DiscussionService) DeleteTopic(ctx context.Context, id uint) error {
	return s.topicRepo.Delete(ctx, id)
}

func (s *DiscussionService) ListTopics(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DiscussionTopic], error) {
	return s.topicRepo.ListByCourseID(ctx, courseID, params)
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

func (s *DiscussionService) GetEntry(ctx context.Context, id uint) (*models.DiscussionEntry, error) {
	return s.entryRepo.FindByID(ctx, id)
}

func (s *DiscussionService) UpdateEntry(ctx context.Context, entry *models.DiscussionEntry) error {
	return s.entryRepo.Update(ctx, entry)
}

func (s *DiscussionService) DeleteEntry(ctx context.Context, id uint) error {
	return s.entryRepo.Delete(ctx, id)
}

func (s *DiscussionService) ListEntries(ctx context.Context, topicID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DiscussionEntry], error) {
	return s.entryRepo.ListByTopicID(ctx, topicID, params)
}

func (s *DiscussionService) ListReplies(ctx context.Context, entryID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DiscussionEntry], error) {
	return s.entryRepo.ListReplies(ctx, entryID, params)
}

// Full view with nested tree

type EntryView struct {
	ID                uint       `json:"id"`
	DiscussionTopicID uint       `json:"discussion_topic_id"`
	UserID            uint       `json:"user_id"`
	ParentID          *uint      `json:"parent_id"`
	Message           string     `json:"message"`
	RatingCount       int        `json:"rating_count"`
	RatingSum         int        `json:"rating_sum"`
	WorkflowState     string     `json:"workflow_state"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	Replies           []EntryView `json:"replies"`
}

type DiscussionFullView struct {
	Topic   *models.DiscussionTopic `json:"topic"`
	Entries []EntryView             `json:"entries"`
}

func (s *DiscussionService) GetFullView(ctx context.Context, topicID uint) (*DiscussionFullView, error) {
	topic, err := s.topicRepo.FindByID(ctx, topicID)
	if err != nil {
		return nil, err
	}

	allEntries, err := s.entryRepo.ListAllByTopicID(ctx, topicID)
	if err != nil {
		return nil, err
	}

	rootEntries := buildTree(allEntries)

	return &DiscussionFullView{
		Topic:   topic,
		Entries: rootEntries,
	}, nil
}

func buildTree(entries []models.DiscussionEntry) []EntryView {
	viewMap := make(map[uint]*EntryView, len(entries))

	// First pass: create all views
	for _, e := range entries {
		view := &EntryView{
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
		viewMap[e.ID] = view
	}

	// Second pass: link children to parents
	var roots []*EntryView
	for _, e := range entries {
		view := viewMap[e.ID]
		if e.ParentID == nil {
			roots = append(roots, view)
		} else if parent, ok := viewMap[*e.ParentID]; ok {
			parent.Replies = append(parent.Replies, *view)
		} else {
			roots = append(roots, view)
		}
	}

	// Convert pointers to values for root entries
	result := make([]EntryView, 0, len(roots))
	for _, r := range roots {
		result = append(result, *r)
	}
	return result
}

// Rating

func (s *DiscussionService) RateEntry(ctx context.Context, entryID uint, userID uint, rating int) error {
	r := &models.DiscussionEntryRating{
		DiscussionEntryID: entryID,
		UserID:            userID,
		Rating:            rating,
	}
	if err := s.ratingRepo.Upsert(ctx, r); err != nil {
		return err
	}

	// Recalculate entry rating_count and rating_sum
	count, sum, err := s.ratingRepo.SumByEntryID(ctx, entryID)
	if err != nil {
		return err
	}

	entry, err := s.entryRepo.FindByID(ctx, entryID)
	if err != nil {
		return err
	}

	entry.RatingCount = int(count)
	entry.RatingSum = int(sum)
	return s.entryRepo.Update(ctx, entry)
}
