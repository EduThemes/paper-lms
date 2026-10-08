package handlers_test

// Topic visibility follow-ups to B050: once students get 404 for
// unpublished and delayed topics, the topic index must not list them
// either, and teachers need a way to publish an unpublished topic (course
// copies and imports create topics in that state). Uses the fixture from
// discussions_scope_test.go.

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

func listedTopicIDs(t *testing.T, env *discEnv) map[uint]map[string]any {
	t.Helper()
	status, body := env.do(t, http.MethodGet, "/courses/1/discussion_topics", nil)
	if status != http.StatusOK {
		t.Fatalf("list status = %d (%s)", status, body)
	}
	var topics []map[string]any
	if err := json.Unmarshal(body, &topics); err != nil {
		t.Fatalf("decode: %v (%s)", err, body)
	}
	out := map[uint]map[string]any{}
	for _, tp := range topics {
		out[uint(tp["id"].(float64))] = tp
	}
	return out
}

func TestDiscussionTopicList_HidesUnreleasedTopicsFromStudents(t *testing.T) {
	t.Run("student list matches what they can open", func(t *testing.T) {
		env := newDiscEnv(t, dStudent1, "StudentEnrollment")
		listed := listedTopicIDs(t, env)
		for _, hidden := range []uint{dTopicDraft, dTopicDelayed, dTopicDeleted} {
			if _, ok := listed[hidden]; ok {
				t.Errorf("student list includes topic %d, which they can't open", hidden)
			}
			if status, _ := env.do(t, http.MethodGet, topicPath(dCourseA1, hidden, ""), nil); status != http.StatusNotFound {
				t.Errorf("GET topic %d = %d, want 404", hidden, status)
			}
		}
		for _, shown := range []uint{dTopicOpen, dTopicLocked, dTopicReleased} {
			if _, ok := listed[shown]; !ok {
				t.Errorf("student list is missing topic %d", shown)
			}
			if status, _ := env.do(t, http.MethodGet, topicPath(dCourseA1, shown, ""), nil); status != http.StatusOK {
				t.Errorf("GET topic %d = %d, want 200", shown, status)
			}
		}
	})

	t.Run("teacher list includes drafts and reports published", func(t *testing.T) {
		env := newDiscEnv(t, dTeacher, "TeacherEnrollment")
		listed := listedTopicIDs(t, env)
		draft, ok := listed[dTopicDraft]
		if !ok {
			t.Fatalf("teacher list is missing the unpublished topic")
		}
		if draft["published"] != false {
			t.Fatalf("draft published = %v, want false", draft["published"])
		}
		if _, ok := listed[dTopicDelayed]; !ok {
			t.Fatalf("teacher list is missing the delayed topic")
		}
		if listed[dTopicOpen]["published"] != true {
			t.Fatalf("active topic published = %v, want true", listed[dTopicOpen]["published"])
		}
	})
}

func TestDiscussionTopicPublishToggle(t *testing.T) {
	publish := func(v bool) map[string]any {
		return map[string]any{"discussion_topic": map[string]any{"published": v}}
	}

	t.Run("teacher publishes a draft", func(t *testing.T) {
		env := newDiscEnv(t, dTeacher, "TeacherEnrollment")
		status, body := env.do(t, http.MethodPut, topicPath(dCourseA1, dTopicDraft, ""), publish(true))
		if status != http.StatusOK {
			t.Fatalf("publish status = %d (%s)", status, body)
		}
		if got := env.store.topics[dTopicDraft].WorkflowState; got != models.DiscussionTopicActive {
			t.Fatalf("workflow_state = %q, want active", got)
		}
		var out map[string]any
		_ = json.Unmarshal(body, &out)
		if out["published"] != true {
			t.Fatalf("response published = %v, want true", out["published"])
		}
	})

	t.Run("TA can publish too", func(t *testing.T) {
		env := newDiscEnv(t, dTeacher, "TaEnrollment")
		if status, body := env.do(t, http.MethodPut, topicPath(dCourseA1, dTopicDraft, ""), publish(true)); status != http.StatusOK {
			t.Fatalf("TA publish status = %d (%s)", status, body)
		}
	})

	t.Run("unpublishing an empty topic works", func(t *testing.T) {
		env := newDiscEnv(t, dTeacher, "TeacherEnrollment")
		if status, body := env.do(t, http.MethodPut, topicPath(dCourseA1, dTopicLocked, ""), publish(false)); status != http.StatusOK {
			t.Fatalf("unpublish status = %d (%s)", status, body)
		}
		if got := env.store.topics[dTopicLocked].WorkflowState; got != models.DiscussionTopicUnpublished {
			t.Fatalf("workflow_state = %q, want unpublished", got)
		}
	})

	t.Run("a topic with replies can't be unpublished", func(t *testing.T) {
		env := newDiscEnv(t, dTeacher, "TeacherEnrollment")
		if status, _ := env.do(t, http.MethodPut, topicPath(dCourseA1, dTopicOpen, ""), publish(false)); status != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", status)
		}
		if got := env.store.topics[dTopicOpen].WorkflowState; got != models.DiscussionTopicActive {
			t.Fatalf("workflow_state = %q, want active", got)
		}
	})

	t.Run("published=true leaves a post_delayed topic alone", func(t *testing.T) {
		env := newDiscEnv(t, dTeacher, "TeacherEnrollment")
		if status, body := env.do(t, http.MethodPut, topicPath(dCourseA1, dTopicReleased, ""), publish(true)); status != http.StatusOK {
			t.Fatalf("status = %d (%s)", status, body)
		}
		if got := env.store.topics[dTopicReleased].WorkflowState; got != models.DiscussionTopicPostDelayed {
			t.Fatalf("workflow_state = %q, want post_delayed", got)
		}
	})
}
