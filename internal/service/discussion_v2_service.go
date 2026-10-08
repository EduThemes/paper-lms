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
	Rating            int           `json:"rating"`     // the viewer's own rating: 1 = liked, 0 = not
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
	// Subscribed is the viewer's subscription state for the topic.
	Subscribed bool `json:"subscribed"`
}

// v2RatingRepoWithSum extends the base interface with aggregation support.
type v2RatingRepoWithSum interface {
	repository.DiscussionEntryRatingRepository
	SumByEntryID(ctx context.Context, entryID uint) (count int64, sum int64, err error)
	ListUserRatingsByTopic(ctx context.Context, topicID, userID uint) (map[uint]int, error)
}

// DiscussionV2Service provides enhanced discussion functionality including
// read/unread tracking, user profile resolution, mentions, and edit history.
//
// Every method that takes a topic or entry id expects the caller (the
// handler) to have resolved that resource with the caller's tenant and
// course scope first (see handlers.requireTopicInCourse /
// requireEntryInTopic).
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

// GetTopic loads a topic scoped to accountID (topic → course → account).
// Handler-layer callers MUST pass callerAccountID(c).
func (s *DiscussionV2Service) GetTopic(ctx context.Context, id, accountID uint) (*models.DiscussionTopic, error) {
	return s.topicRepo.FindByID(ctx, id, accountID)
}

// GetEntry loads an entry scoped to accountID (entry → topic → course →
// account). Handler-layer callers MUST pass callerAccountID(c).
func (s *DiscussionV2Service) GetEntry(ctx context.Context, id, accountID uint) (*models.DiscussionEntry, error) {
	return s.entryRepo.FindByID(ctx, id, accountID)
}

// GetFullViewWithReadState returns the full discussion tree with read/unread state per entry
// and user profile info resolved for all participants. The topic must
// already have been resolved with the caller's tenant + course scope.
func (s *DiscussionV2Service) GetFullViewWithReadState(ctx context.Context, topic *models.DiscussionTopic, userID, accountID uint) (*DiscussionFullViewV2, error) {
	topicID := topic.ID

	allEntries, err := s.entryRepo.ListAllByTopicIDIncludingDeleted(ctx, topicID, accountID)
	if err != nil {
		return nil, err
	}
	visible := visibleThreadEntries(allEntries)

	// Get unread entry IDs for the current user
	unreadIDs, err := s.entryParticipantRepo.ListUnreadByTopic(ctx, topicID, userID)
	if err != nil {
		return nil, err
	}
	unreadSet := make(map[uint]bool, len(unreadIDs))
	for _, id := range unreadIDs {
		unreadSet[id] = true
	}

	// The viewer's own likes, so the UI can render liked state / unlike.
	myRatings, err := s.ratingRepo.ListUserRatingsByTopic(ctx, topicID, userID)
	if err != nil {
		myRatings = map[uint]int{}
	}

	// Collect unique user IDs
	userIDSet := make(map[uint]bool)
	for _, e := range visible {
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
	for _, e := range visible {
		count, err := s.versionRepo.CountByEntryID(ctx, e.ID)
		if err == nil {
			versionCounts[e.ID] = int(count)
		}
	}

	// Build the tree
	rootEntries := s.buildTreeV2(visible, unreadSet, myRatings, userInfoMap, versionCounts)

	return &DiscussionFullViewV2{
		Topic:      topic,
		Entries:    rootEntries,
		Subscribed: s.isSubscribed(ctx, topic, userID, allEntries),
	}, nil
}

func (s *DiscussionV2Service) buildTreeV2(entries []models.DiscussionEntry, unreadSet map[uint]bool, myRatings map[uint]int, userInfoMap map[uint]UserInfo, versionCounts map[uint]int) []EntryViewV2 {
	return assembleThread(entries, func(e models.DiscussionEntry) EntryViewV2 {
		info := userInfoMap[e.UserID]
		var editedAt *time.Time
		if versionCounts[e.ID] > 0 {
			updatedAt := e.UpdatedAt
			editedAt = &updatedAt
		}
		return EntryViewV2{
			ID:                e.ID,
			DiscussionTopicID: e.DiscussionTopicID,
			UserID:            e.UserID,
			UserName:          info.Name,
			UserAvatarURL:     info.AvatarURL,
			ParentID:          e.ParentID,
			Message:           e.Message,
			RatingCount:       e.RatingCount,
			RatingSum:         e.RatingSum,
			Rating:            myRatings[e.ID],
			ReadState:         !unreadSet[e.ID], // read = not in unread set
			EditedAt:          editedAt,
			VersionCount:      versionCounts[e.ID],
			WorkflowState:     e.WorkflowState,
			CreatedAt:         e.CreatedAt,
			UpdatedAt:         e.UpdatedAt,
			Replies:           []EntryViewV2{},
		}
	}, func(parent *EntryViewV2, replies []EntryViewV2) {
		parent.Replies = replies
	})
}

// isSubscribed mirrors Canvas DiscussionTopic#subscribed?: an explicit
// participant row wins; without one, the topic author and anyone who
// has posted are treated as subscribed.
func (s *DiscussionV2Service) isSubscribed(ctx context.Context, topic *models.DiscussionTopic, userID uint, entries []models.DiscussionEntry) bool {
	if p, err := s.topicParticipantRepo.FindByTopicAndUser(ctx, topic.ID, userID); err == nil && p != nil {
		return p.Subscribed
	}
	return defaultSubscribed(topic, userID, entries)
}

func defaultSubscribed(topic *models.DiscussionTopic, userID uint, entries []models.DiscussionEntry) bool {
	if topic.UserID == userID {
		return true
	}
	for _, e := range entries {
		if e.UserID == userID && e.WorkflowState != discussionEntryDeleted {
			return true
		}
	}
	return false
}

// MarkEntryAsRead marks a single entry as read for the given user.
func (s *DiscussionV2Service) MarkEntryAsRead(ctx context.Context, entryID, userID uint) error {
	return s.entryParticipantRepo.MarkAsRead(ctx, entryID, userID)
}

// MarkTopicAsRead marks all entries in a topic as read for the given user
// and stamps last_read_at. The user's existing subscription choice (and
// forced read state) is preserved: marking a thread read must never
// re-subscribe someone who unsubscribed.
func (s *DiscussionV2Service) MarkTopicAsRead(ctx context.Context, topic *models.DiscussionTopic, userID uint) error {
	if err := s.entryParticipantRepo.MarkTopicAsRead(ctx, topic.ID, userID); err != nil {
		return err
	}

	now := time.Now()
	participant := &models.DiscussionTopicParticipant{
		DiscussionTopicID: topic.ID,
		UserID:            userID,
		LastReadAt:        &now,
	}
	if existing, err := s.topicParticipantRepo.FindByTopicAndUser(ctx, topic.ID, userID); err == nil && existing != nil {
		participant.Subscribed = existing.Subscribed
		participant.ForcedReadState = existing.ForcedReadState
	} else {
		// No row yet: seed it with the Canvas default rather than an
		// unconditional "subscribed".
		// accountID=0: internal hop; the handler resolved this topic
		// inside the caller's tenant and course.
		entries, lerr := s.entryRepo.ListAllByTopicID(ctx, topic.ID, 0)
		if lerr != nil {
			entries = nil
		}
		participant.Subscribed = defaultSubscribed(topic, userID, entries)
	}
	return s.topicParticipantRepo.Upsert(ctx, participant)
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

// UpdateEntryWithHistory replaces an entry's message and records the edit
// in discussion_entry_versions, following Canvas
// DiscussionEntry#create_discussion_entry_versions: the history holds
// every version of the message, including the current one, each
// attributed to the user who wrote it. On the first edit the original
// message is stored as version 1 (by the entry author) and the new
// message as version 2 (by the editor).
//
// The entry must already have been resolved with the caller's tenant,
// course and topic scope, and the caller authorized to edit it.
func (s *DiscussionV2Service) UpdateEntryWithHistory(ctx context.Context, entry *models.DiscussionEntry, editorID uint, newMessage string) error {
	if newMessage == "" {
		return errors.New("message cannot be empty")
	}
	if entry == nil {
		return errors.New("entry is required")
	}
	if newMessage == entry.Message {
		return nil
	}

	versions, err := s.versionRepo.ListByEntryID(ctx, entry.ID)
	if err != nil {
		return err
	}

	next := 1
	if len(versions) > 0 {
		next = versions[len(versions)-1].Version + 1
	}

	// Make sure the message being replaced is in the history. This is
	// the original post on the first edit; for rows versioned under the
	// old scheme (which stored only replaced messages) it back-fills the
	// current text so no version is lost.
	if len(versions) == 0 || versions[len(versions)-1].Message != entry.Message {
		current := &models.DiscussionEntryVersion{
			DiscussionEntryID: entry.ID,
			UserID:            entry.UserID,
			Message:           entry.Message,
			Version:           next,
		}
		if err := s.versionRepo.Create(ctx, current); err != nil {
			return err
		}
		next++
	}

	edited := &models.DiscussionEntryVersion{
		DiscussionEntryID: entry.ID,
		UserID:            editorID,
		Message:           newMessage,
		Version:           next,
	}
	if err := s.versionRepo.Create(ctx, edited); err != nil {
		return err
	}

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
		// accountID=0: internal hop. The user IDs come from entries /
		// topics that the handler already resolved inside the caller's
		// tenant and course, so the authors are known to be in scope.
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
