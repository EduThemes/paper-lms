package postgres_test

// DB tests for the discussion repo queries the thread view depends on:
// unread tracking excludes the viewer's own posts (B156), the
// including-deleted listing that lets deleted parents stay as
// placeholders (B157), and the viewer's own likes (B158). Gated on
// PARITY_DB_URL / DATABASE_URL via freshDB.

import (
	"context"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
)

func TestDiscussionRepos_UnreadDeletedAndRatings(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()

	for _, id := range []uint{1, 2} {
		if err := g.WithContext(ctx).Exec(
			`INSERT INTO accounts (id, name, workflow_state, mfa_policy, default_locale, tenant_mode, max_upload_size_mb)
			 VALUES (?, ?, 'active', 'off', 'en', 'higher_ed', 500)
			 ON CONFLICT (id) DO NOTHING`, id, "Acct").Error; err != nil {
			t.Fatalf("seed account %d: %v", id, err)
		}
	}

	userRepo := postgres.NewUserRepository(g)
	author := &models.User{Name: "Jane Doe", LoginID: "jane@example.test", Email: "jane@example.test", PasswordHash: "x", Role: "user"}
	reader := &models.User{Name: "John Doe", LoginID: "john@example.test", Email: "john@example.test", PasswordHash: "x", Role: "user"}
	for _, u := range []*models.User{author, reader} {
		if err := userRepo.Create(ctx, u); err != nil {
			t.Fatalf("create user: %v", err)
		}
	}

	course := &models.Course{AccountID: 1, Name: "Biology", CourseCode: "BIO-1", WorkflowState: models.CourseAvailable}
	if err := postgres.NewCourseRepository(g).Create(ctx, course); err != nil {
		t.Fatalf("create course: %v", err)
	}
	topic := &models.DiscussionTopic{CourseID: course.ID, UserID: author.ID, Title: "Week 1", WorkflowState: models.DiscussionTopicActive}
	if err := postgres.NewDiscussionTopicRepository(g).Create(ctx, topic); err != nil {
		t.Fatalf("create topic: %v", err)
	}

	entryRepo := postgres.NewDiscussionEntryRepository(g)
	byAuthor := &models.DiscussionEntry{DiscussionTopicID: topic.ID, UserID: author.ID, Message: "first", WorkflowState: "active"}
	byReader := &models.DiscussionEntry{DiscussionTopicID: topic.ID, UserID: reader.ID, Message: "second", WorkflowState: "active"}
	deleted := &models.DiscussionEntry{DiscussionTopicID: topic.ID, UserID: author.ID, Message: "gone", WorkflowState: "deleted"}
	for _, e := range []*models.DiscussionEntry{byAuthor, byReader, deleted} {
		if err := entryRepo.Create(ctx, e); err != nil {
			t.Fatalf("create entry: %v", err)
		}
	}

	partRepo := postgres.NewDiscussionEntryParticipantRepository(g)
	t.Run("unread excludes own and deleted entries", func(t *testing.T) {
		count, err := partRepo.CountUnreadByTopic(ctx, topic.ID, reader.ID)
		if err != nil {
			t.Fatalf("count: %v", err)
		}
		if count != 1 {
			t.Fatalf("unread count for reader = %d, want 1 (only the author's live post)", count)
		}
		ids, err := partRepo.ListUnreadByTopic(ctx, topic.ID, reader.ID)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(ids) != 1 || ids[0] != byAuthor.ID {
			t.Fatalf("unread ids = %v, want [%d]", ids, byAuthor.ID)
		}
		if err := partRepo.MarkAsRead(ctx, byAuthor.ID, reader.ID); err != nil {
			t.Fatalf("mark read: %v", err)
		}
		if count, _ := partRepo.CountUnreadByTopic(ctx, topic.ID, reader.ID); count != 0 {
			t.Fatalf("unread after mark read = %d, want 0", count)
		}
	})

	t.Run("including-deleted listing is tenant scoped", func(t *testing.T) {
		all, err := entryRepo.ListAllByTopicIDIncludingDeleted(ctx, topic.ID, 1)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(all) != 3 {
			t.Fatalf("entries = %d, want 3 (deleted included)", len(all))
		}
		other, err := entryRepo.ListAllByTopicIDIncludingDeleted(ctx, topic.ID, 2)
		if err != nil {
			t.Fatalf("list other tenant: %v", err)
		}
		if len(other) != 0 {
			t.Fatalf("account 2 saw %d entries of account 1's topic", len(other))
		}
	})

	t.Run("student-visible topic listing", func(t *testing.T) {
		topicRepo := postgres.NewDiscussionTopicRepository(g)
		now := time.Now()
		past, future := now.Add(-time.Hour), now.Add(time.Hour)
		mk := func(title string, state models.DiscussionTopicWorkflow, delayed *time.Time) *models.DiscussionTopic {
			tp := &models.DiscussionTopic{CourseID: course.ID, UserID: author.ID, Title: title, WorkflowState: state, DelayedPostAt: delayed}
			if err := topicRepo.Create(ctx, tp); err != nil {
				t.Fatalf("create topic %s: %v", title, err)
			}
			return tp
		}
		draft := mk("draft", models.DiscussionTopicUnpublished, nil)
		later := mk("later", models.DiscussionTopicActive, &future)
		released := mk("released", models.DiscussionTopicPostDelayed, &past)
		waiting := mk("waiting", models.DiscussionTopicPostDelayed, &future)
		gone := mk("gone", models.DiscussionTopicDeleted, nil)

		lister, ok := topicRepo.(interface {
			ListStudentVisibleByCourseID(context.Context, uint, uint, time.Time, repository.PaginationParams) (*repository.PaginatedResult[models.DiscussionTopic], error)
		})
		if !ok {
			t.Fatalf("topic repo lacks ListStudentVisibleByCourseID")
		}
		res, err := lister.ListStudentVisibleByCourseID(ctx, course.ID, 1, now, repository.PaginationParams{Page: 1, PerPage: 50})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		got := map[uint]bool{}
		for _, tp := range res.Items {
			got[tp.ID] = true
		}
		if !got[topic.ID] || !got[released.ID] {
			t.Fatalf("visible topics missing: got %v", got)
		}
		for _, hidden := range []*models.DiscussionTopic{draft, later, waiting, gone} {
			if got[hidden.ID] {
				t.Fatalf("topic %q listed for students", hidden.Title)
			}
		}
		if res.TotalCount != 2 {
			t.Fatalf("total = %d, want 2", res.TotalCount)
		}
		other, err := lister.ListStudentVisibleByCourseID(ctx, course.ID, 2, now, repository.PaginationParams{Page: 1, PerPage: 50})
		if err != nil || len(other.Items) != 0 {
			t.Fatalf("account 2 listed %d topics of account 1 (err %v)", len(other.Items), err)
		}
	})

	t.Run("viewer ratings", func(t *testing.T) {
		ratingRepo := postgres.NewDiscussionEntryRatingRepository(g)
		if err := ratingRepo.Upsert(ctx, &models.DiscussionEntryRating{DiscussionEntryID: byAuthor.ID, UserID: reader.ID, Rating: 1}); err != nil {
			t.Fatalf("upsert: %v", err)
		}
		mine, err := ratingRepo.ListUserRatingsByTopic(ctx, topic.ID, reader.ID)
		if err != nil {
			t.Fatalf("list ratings: %v", err)
		}
		if len(mine) != 1 || mine[byAuthor.ID] != 1 {
			t.Fatalf("reader ratings = %v, want {%d:1}", mine, byAuthor.ID)
		}
		theirs, _ := ratingRepo.ListUserRatingsByTopic(ctx, topic.ID, author.ID)
		if len(theirs) != 0 {
			t.Fatalf("author ratings = %v, want none", theirs)
		}
		if err := ratingRepo.Delete(ctx, byAuthor.ID, reader.ID); err != nil {
			t.Fatalf("delete rating: %v", err)
		}
		if count, sum, _ := ratingRepo.SumByEntryID(ctx, byAuthor.ID); count != 0 || sum != 0 {
			t.Fatalf("after unlike count=%d sum=%d, want 0/0", count, sum)
		}
	})
}
