package service

import (
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// UserInfo holds resolved user profile data for display in discussion views.
type UserInfo struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

// EntryViewV2 is an enhanced entry view with user info, read state, and edit history metadata.
type EntryViewV2 struct {
	ID                uint          `json:"id"`
	DiscussionTopicID uint          `json:"discussion_topic_id"`
	UserID            uint          `json:"user_id"`
	UserName          string        `json:"user_name"`
	UserAvatarURL     string        `json:"user_avatar_url"`
	ParentID          *uint         `json:"parent_id"`
	Message           string        `json:"message"`
	RatingCount       int           `json:"rating_count"`
	RatingSum         int           `json:"rating_sum"`
	ReadState         bool          `json:"read_state"` // true = read
	EditedAt          *time.Time    `json:"edited_at"`
	VersionCount      int           `json:"version_count"`
	WorkflowState     string        `json:"workflow_state"`
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
	Replies           []EntryViewV2 `json:"replies"`
}

// DiscussionFullViewV2 is an enhanced full view with user info and read states.
type DiscussionFullViewV2 struct {
	Topic   *models.DiscussionTopic `json:"topic"`
	Entries []EntryViewV2           `json:"entries"`
}

// v2RatingRepoWithSum extends the base interface with aggregation support.
type v2RatingRepoWithSum interface {
	repository.DiscussionEntryRatingRepository
	SumByEntryID(ctx context.Context, entryID uint) (count int64, sum int64, err error)
}

// DiscussionV2Service provides enhanced discussion functionality including
// read/unread tracking, user profile resolution, mentions, and edit history.
type DiscussionV2Service struct {
	topicRepo            repository.DiscussionTopicRepository
	entryRepo            repository.DiscussionEntryRepository
	ratingRepo           v2RatingRepoWithSum
	entryParticipantRepo repository.DiscussionEntryParticipantRepository
	topicParticipantRepo repository.DiscussionTopicParticipantRepository
	versionRepo          repository.DiscussionEntryVersionRepository
	userRepo             repository.UserRepository
}

func NewDiscussionV2Service(
	topicRepo repository.DiscussionTopicRepository,
	entryRepo repository.DiscussionEntryRepository,
	ratingRepo v2RatingRepoWithSum,
	entryParticipantRepo repository.DiscussionEntryParticipantRepository,
	topicParticipantRepo repository.DiscussionTopicParticipantRepository,
	versionRepo repository.DiscussionEntryVersionRepository,
	userRepo repository.UserRepository,
) *DiscussionV2Service {
	return &DiscussionV2Service{
		topicRepo:            topicRepo,
		entryRepo:            entryRepo,
		ratingRepo:           ratingRepo,
		entryParticipantRepo: entryParticipantRepo,
		topicParticipantRepo: topicParticipantRepo,
		versionRepo:          versionRepo,
		userRepo:             userRepo,
	}
}

// --- Existing methods (delegating to same repos as DiscussionService) ---

func (s *DiscussionV2Service) CreateTopic(ctx context.Context, topic *models.DiscussionTopic) error {
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

func (s *DiscussionV2Service) GetTopic(ctx context.Context, id uint) (*models.DiscussionTopic, error) {
	return s.topicRepo.FindByID(ctx, id, 0)
}

func (s *DiscussionV2Service) UpdateTopic(ctx context.Context, topic *models.DiscussionTopic) error {
	return s.topicRepo.Update(ctx, topic)
}

func (s *DiscussionV2Service) DeleteTopic(ctx context.Context, id uint) error {
	return s.topicRepo.Delete(ctx, id)
}

func (s *DiscussionV2Service) ListTopics(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DiscussionTopic], error) {
	return s.topicRepo.ListByCourseID(ctx, courseID, 0, params)
}

func (s *DiscussionV2Service) CreateEntry(ctx context.Context, entry *models.DiscussionEntry) error {
	if entry.Message == "" {
		return errors.New("discussion entry message is required")
	}
	if entry.WorkflowState == "" {
		entry.WorkflowState = "active"
	}
	return s.entryRepo.Create(ctx, entry)
}

func (s *DiscussionV2Service) GetEntry(ctx context.Context, id uint) (*models.DiscussionEntry, error) {
	return s.entryRepo.FindByID(ctx, id, 0)
}

func (s *DiscussionV2Service) UpdateEntry(ctx context.Context, entry *models.DiscussionEntry) error {
	return s.entryRepo.Update(ctx, entry)
}

func (s *DiscussionV2Service) DeleteEntry(ctx context.Context, id uint) error {
	return s.entryRepo.Delete(ctx, id)
}

func (s *DiscussionV2Service) ListEntries(ctx context.Context, topicID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DiscussionEntry], error) {
	return s.entryRepo.ListByTopicID(ctx, topicID, 0, params)
}

func (s *DiscussionV2Service) ListReplies(ctx context.Context, entryID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DiscussionEntry], error) {
	return s.entryRepo.ListReplies(ctx, entryID, 0, params)
}

func (s *DiscussionV2Service) RateEntry(ctx context.Context, entryID uint, userID uint, rating int) error {
	r := &models.DiscussionEntryRating{
		DiscussionEntryID: entryID,
		UserID:            userID,
		Rating:            rating,
	}
	if err := s.ratingRepo.Upsert(ctx, r); err != nil {
		return err
	}

	count, sum, err := s.ratingRepo.SumByEntryID(ctx, entryID)
	if err != nil {
		return err
	}

	entry, err := s.entryRepo.FindByID(ctx, entryID, 0)
	if err != nil {
		return err
	}

	entry.RatingCount = int(count)
	entry.RatingSum = int(sum)
	return s.entryRepo.Update(ctx, entry)
}

// --- New V2 methods ---

// GetFullViewWithReadState returns the full discussion tree with read/unread state per entry
// and user profile info resolved for all participants.
func (s *DiscussionV2Service) GetFullViewWithReadState(ctx context.Context, topicID, userID uint) (*DiscussionFullViewV2, error) {
	topic, err := s.topicRepo.FindByID(ctx, topicID, 0)
	if err != nil {
		return nil, err
	}

	allEntries, err := s.entryRepo.ListAllByTopicID(ctx, topicID, 0)
	if err != nil {
		return nil, err
	}

	// Get unread entry IDs for the current user
	unreadIDs, err := s.entryParticipantRepo.ListUnreadByTopic(ctx, topicID, userID)
	if err != nil {
		return nil, err
	}
	unreadSet := make(map[uint]bool, len(unreadIDs))
	for _, id := range unreadIDs {
		unreadSet[id] = true
	}

	// Collect unique user IDs
	userIDSet := make(map[uint]bool)
	for _, e := range allEntries {
		userIDSet[e.UserID] = true
	}
	var userIDs []uint
	for uid := range userIDSet {
		userIDs = append(userIDs, uid)
	}

	// Resolve user info
	userInfoMap := s.ResolveUserInfo(ctx, userIDs)

	// Get version counts for all entries
	versionCounts := make(map[uint]int)
	for _, e := range allEntries {
		count, err := s.versionRepo.CountByEntryID(ctx, e.ID)
		if err == nil {
			versionCounts[e.ID] = int(count)
		}
	}

	// Build the tree
	rootEntries := s.buildTreeV2(allEntries, unreadSet, userInfoMap, versionCounts)

	return &DiscussionFullViewV2{
		Topic:   topic,
		Entries: rootEntries,
	}, nil
}

func (s *DiscussionV2Service) buildTreeV2(entries []models.DiscussionEntry, unreadSet map[uint]bool, userInfoMap map[uint]UserInfo, versionCounts map[uint]int) []EntryViewV2 {
	viewMap := make(map[uint]*EntryViewV2, len(entries))

	// First pass: create all views
	for _, e := range entries {
		info := userInfoMap[e.UserID]
		var editedAt *time.Time
		if versionCounts[e.ID] > 0 {
			editedAt = &e.UpdatedAt
		}

		view := &EntryViewV2{
			ID:                e.ID,
			DiscussionTopicID: e.DiscussionTopicID,
			UserID:            e.UserID,
			UserName:          info.Name,
			UserAvatarURL:     info.AvatarURL,
			ParentID:          e.ParentID,
			Message:           e.Message,
			RatingCount:       e.RatingCount,
			RatingSum:         e.RatingSum,
			ReadState:         !unreadSet[e.ID], // read = not in unread set
			EditedAt:          editedAt,
			VersionCount:      versionCounts[e.ID],
			WorkflowState:     e.WorkflowState,
			CreatedAt:         e.CreatedAt,
			UpdatedAt:         e.UpdatedAt,
			Replies:           []EntryViewV2{},
		}
		viewMap[e.ID] = view
	}

	// Second pass: link children to parents
	var roots []*EntryViewV2
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
	result := make([]EntryViewV2, 0, len(roots))
	for _, r := range roots {
		result = append(result, *r)
	}
	return result
}

// MarkEntryAsRead marks a single entry as read for the given user.
func (s *DiscussionV2Service) MarkEntryAsRead(ctx context.Context, entryID, userID uint) error {
	return s.entryParticipantRepo.MarkAsRead(ctx, entryID, userID)
}

// MarkTopicAsRead marks all entries in a topic as read for the given user.
func (s *DiscussionV2Service) MarkTopicAsRead(ctx context.Context, topicID, userID uint) error {
	if err := s.entryParticipantRepo.MarkTopicAsRead(ctx, topicID, userID); err != nil {
		return err
	}

	// Also update topic participant's last_read_at
	now := time.Now()
	return s.topicParticipantRepo.Upsert(ctx, &models.DiscussionTopicParticipant{
		DiscussionTopicID: topicID,
		UserID:            userID,
		Subscribed:        true,
		LastReadAt:        &now,
	})
}

// GetUnreadCount returns the number of unread entries for a user in a topic.
func (s *DiscussionV2Service) GetUnreadCount(ctx context.Context, topicID, userID uint) (int64, error) {
	return s.entryParticipantRepo.CountUnreadByTopic(ctx, topicID, userID)
}

// ToggleSubscription sets the subscription state for a user on a topic.
func (s *DiscussionV2Service) ToggleSubscription(ctx context.Context, topicID, userID uint, subscribed bool) error {
	return s.topicParticipantRepo.UpdateSubscription(ctx, topicID, userID, subscribed)
}

// GetEntryVersions returns the edit history for a discussion entry.
func (s *DiscussionV2Service) GetEntryVersions(ctx context.Context, entryID uint) ([]models.DiscussionEntryVersion, error) {
	return s.versionRepo.ListByEntryID(ctx, entryID)
}

// UpdateEntryWithHistory saves the current message as a version before updating the entry.
func (s *DiscussionV2Service) UpdateEntryWithHistory(ctx context.Context, entryID, userID uint, newMessage string) error {
	if newMessage == "" {
		return errors.New("message cannot be empty")
	}

	entry, err := s.entryRepo.FindByID(ctx, entryID, 0)
	if err != nil {
		return err
	}

	// Determine the next version number
	currentCount, err := s.versionRepo.CountByEntryID(ctx, entryID)
	if err != nil {
		return err
	}

	// Save the current message as a version
	version := &models.DiscussionEntryVersion{
		DiscussionEntryID: entryID,
		UserID:            entry.UserID,
		Message:           entry.Message,
		Version:           int(currentCount) + 1,
	}
	if err := s.versionRepo.Create(ctx, version); err != nil {
		return err
	}

	// Update the entry with the new message
	entry.Message = newMessage
	return s.entryRepo.Update(ctx, entry)
}

// mentionRegex matches @username patterns (alphanumeric, underscores, hyphens, dots).
var mentionRegex = regexp.MustCompile(`@([a-zA-Z0-9_.\-]+)`)

// ParseMentions extracts @username mentions from message text.
func ParseMentions(message string) []string {
	matches := mentionRegex.FindAllStringSubmatch(message, -1)
	seen := make(map[string]bool)
	var usernames []string
	for _, match := range matches {
		if len(match) >= 2 && !seen[match[1]] {
			seen[match[1]] = true
			usernames = append(usernames, match[1])
		}
	}
	return usernames
}

// ResolveUserInfo batch-fetches user Name and AvatarURL for a set of user IDs.
func (s *DiscussionV2Service) ResolveUserInfo(ctx context.Context, userIDs []uint) map[uint]UserInfo {
	result := make(map[uint]UserInfo, len(userIDs))
	for _, uid := range userIDs {
		// Wave 2 widening: this service doesn't yet thread accountID
		// from its handler. accountID=0 preserves the pre-widening
		// semantics. Sprint 2.3 leftover: thread tenant context
		// through DiscussionV2Service from the handler.
		user, err := s.userRepo.FindByID(ctx, uid, 0)
		if err != nil {
			result[uid] = UserInfo{ID: uid, Name: "Unknown User", AvatarURL: ""}
			continue
		}
		result[uid] = UserInfo{
			ID:        user.ID,
			Name:      user.Name,
			AvatarURL: user.AvatarURL,
		}
	}
	return result
}
