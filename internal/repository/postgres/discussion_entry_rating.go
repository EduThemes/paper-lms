package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"gorm.io/gorm"
)

type DiscussionEntryRatingRepo struct {
	db *gorm.DB
}

func NewDiscussionEntryRatingRepository(db *gorm.DB) *DiscussionEntryRatingRepo {
	return &DiscussionEntryRatingRepo{db: db}
}

func (r *DiscussionEntryRatingRepo) Upsert(ctx context.Context, rating *models.DiscussionEntryRating) error {
	return r.db.WithContext(ctx).Exec(
		"INSERT INTO discussion_entry_ratings (discussion_entry_id, user_id, rating) VALUES (?, ?, ?) ON CONFLICT (discussion_entry_id, user_id) DO UPDATE SET rating = ?",
		rating.DiscussionEntryID, rating.UserID, rating.Rating, rating.Rating,
	).Error
}

func (r *DiscussionEntryRatingRepo) Delete(ctx context.Context, entryID uint, userID uint) error {
	return r.db.WithContext(ctx).Where("discussion_entry_id = ? AND user_id = ?", entryID, userID).Delete(&models.DiscussionEntryRating{}).Error
}

func (r *DiscussionEntryRatingRepo) SumByEntryID(ctx context.Context, entryID uint) (count int64, sum int64, err error) {
	var result struct {
		Count int64
		Sum   int64
	}
	err = r.db.WithContext(ctx).Model(&models.DiscussionEntryRating{}).
		Select("COUNT(*) as count, COALESCE(SUM(rating), 0) as sum").
		Where("discussion_entry_id = ?", entryID).
		Scan(&result).Error
	return result.Count, result.Sum, err
}

// ListUserRatingsByTopic returns the viewer's own rating per entry on a
// topic (entry_id -> rating), so the thread view can show whether the
// viewer already liked a post and let them unlike it.
func (r *DiscussionEntryRatingRepo) ListUserRatingsByTopic(ctx context.Context, topicID, userID uint) (map[uint]int, error) {
	var rows []models.DiscussionEntryRating
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND discussion_entry_id IN (SELECT id FROM discussion_entries WHERE discussion_topic_id = ?)", userID, topicID).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[uint]int, len(rows))
	for _, row := range rows {
		out[row.DiscussionEntryID] = row.Rating
	}
	return out, nil
}
