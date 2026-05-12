package service

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/repository"
)

// ContentViewedCallback fires asynchronously after RecordView writes to
// content_views. The callback receives the same identifying tuple the
// view row carries so the gamification engine can build a `verb=viewed`
// event without re-querying.
type ContentViewedCallback func(ctx context.Context, userID uint, objectType string, objectID uint, durationSeconds int64)

// ContentViewService is the thin orchestrator handlers call when a page
// (or any other viewable object) is rendered for a user. It owns the
// upsert into content_views and the post-write callback fan-out — keeping
// every page-view side-effect in one place so handlers don't drift apart.
//
// The aggregate repo is the only required dependency; callbacks are
// opt-in via OnViewed (the gamification engine registers one in
// cmd/server/main.go to fire ViewedContent-predicated rules).
type ContentViewService struct {
	contentViewRepo repository.ContentViewRepository

	onViewedCallbacks []ContentViewedCallback
}

func NewContentViewService(contentViewRepo repository.ContentViewRepository) *ContentViewService {
	return &ContentViewService{contentViewRepo: contentViewRepo}
}

// OnViewed registers a callback to fire after a successful RecordView.
// The callback runs in a fresh goroutine with a detached context so it
// survives the originating HTTP request being cancelled. Multiple
// registrations stack; order is registration order.
func (s *ContentViewService) OnViewed(cb ContentViewedCallback) {
	s.onViewedCallbacks = append(s.onViewedCallbacks, cb)
}

// RecordView upserts the (user, object_type, object_id) row in
// content_views, incrementing view_count and total_seconds and bumping
// last_viewed_at. After a successful write it fans out to every
// registered callback (gamification emit, future analytics, etc.).
//
// durationSeconds is "seconds the user spent on this page during *this*
// view." Callers that don't track duration pass 0 and let the aggregate
// reflect raw view counts only.
func (s *ContentViewService) RecordView(ctx context.Context, userID uint, objectType string, objectID uint, durationSeconds int64) error {
	if err := s.contentViewRepo.IncrementView(ctx, userID, objectType, objectID, durationSeconds); err != nil {
		return err
	}
	for _, cb := range s.onViewedCallbacks {
		go func(cb ContentViewedCallback) {
			defer recoverFromPanic("content view OnViewed callback")
			cb(context.Background(), userID, objectType, objectID, durationSeconds)
		}(cb)
	}
	return nil
}
