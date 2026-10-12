package service

import (
	"strings"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

func discStrPtr(s string) *string { return &s }

func TestValidateTopicOptions_CanvasCombinationRules(t *testing.T) {
	cat := uint(3)
	aid := uint(9)
	cases := []struct {
		name   string
		topic  models.DiscussionTopic
		graded bool
		want   error
	}{
		{"plain", models.DiscussionTopic{}, false, nil},
		{"full anonymity", models.DiscussionTopic{AnonymousState: discStrPtr(models.DiscussionFullAnonymity)}, false, nil},
		{"bad anonymity value", models.DiscussionTopic{AnonymousState: discStrPtr("sometimes")}, false, ErrDiscussionInvalidAnonymity},
		{"anonymous graded", models.DiscussionTopic{AnonymousState: discStrPtr(models.DiscussionPartialAnonymity)}, true, ErrDiscussionAnonymousConflict},
		{"anonymous linked assignment", models.DiscussionTopic{AnonymousState: discStrPtr(models.DiscussionFullAnonymity), AssignmentID: &aid}, false, ErrDiscussionAnonymousConflict},
		{"anonymous group", models.DiscussionTopic{AnonymousState: discStrPtr(models.DiscussionFullAnonymity), GroupCategoryID: &cat}, false, ErrDiscussionAnonymousConflict},
		{"sections + graded", models.DiscussionTopic{IsSectionSpecific: true}, true, ErrDiscussionSectionConflict},
		{"sections + group", models.DiscussionTopic{IsSectionSpecific: true, GroupCategoryID: &cat}, false, ErrDiscussionSectionConflict},
		{"graded group", models.DiscussionTopic{GroupCategoryID: &cat}, true, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ValidateTopicOptions(&tc.topic, tc.graded); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDiscussionStateForSchedule(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	later := now.Add(time.Hour)
	earlier := now.Add(-time.Hour)
	cases := []struct {
		state models.DiscussionTopicWorkflow
		at    *time.Time
		want  models.DiscussionTopicWorkflow
	}{
		{models.DiscussionTopicActive, &later, models.DiscussionTopicPostDelayed},
		{models.DiscussionTopicActive, nil, models.DiscussionTopicActive},
		{models.DiscussionTopicPostDelayed, &earlier, models.DiscussionTopicActive},
		{models.DiscussionTopicPostDelayed, nil, models.DiscussionTopicActive},
		{models.DiscussionTopicUnpublished, &later, models.DiscussionTopicUnpublished},
		{models.DiscussionTopicLocked, &later, models.DiscussionTopicLocked},
	}
	for _, tc := range cases {
		topic := &models.DiscussionTopic{WorkflowState: tc.state, DelayedPostAt: tc.at}
		if got := DiscussionStateForSchedule(topic, now); got != tc.want {
			t.Errorf("%s with %v: got %s, want %s", tc.state, tc.at, got, tc.want)
		}
	}
}

func TestEntryIsAnonymous(t *testing.T) {
	full := &models.DiscussionTopic{AnonymousState: discStrPtr(models.DiscussionFullAnonymity)}
	partial := &models.DiscussionTopic{AnonymousState: discStrPtr(models.DiscussionPartialAnonymity)}
	named := &models.DiscussionTopic{}
	if !EntryIsAnonymous(full, &models.DiscussionEntry{}) {
		t.Error("every post in a fully anonymous topic is anonymous")
	}
	if EntryIsAnonymous(partial, &models.DiscussionEntry{}) || !EntryIsAnonymous(partial, &models.DiscussionEntry{IsAnonymousAuthor: true}) {
		t.Error("partial anonymity follows the post's flag")
	}
	if EntryIsAnonymous(named, &models.DiscussionEntry{IsAnonymousAuthor: true}) {
		t.Error("the flag means nothing outside an anonymous topic")
	}
	if got := AnonymousName("1a2b3"); got != "Anonymous 1a2b3" {
		t.Errorf("AnonymousName = %q", got)
	}
}

func TestMentionText_MatchesNamesInMarkup(t *testing.T) {
	got := mentionText(`<p>Thanks <strong>@Jane Doe</strong> &amp; @John</p>`)
	for _, want := range []string{"@jane doe", "& @john"} {
		if !strings.Contains(got, want) {
			t.Errorf("mentionText = %q, missing %q", got, want)
		}
	}
}
