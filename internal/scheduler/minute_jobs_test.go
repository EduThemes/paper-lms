package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakePublisher struct {
	calls int
	err   error
}

func (f *fakePublisher) PublishScheduledAnnouncements(context.Context) (int, error) {
	f.calls++
	return 1, f.err
}

type fakeDeliverer struct{ calls int }

func (f *fakeDeliverer) ProcessPendingDeliveries(context.Context) (int, error) {
	f.calls++
	return 2, nil
}

// B053/B055: the per-minute jobs drive delayed announcement publishing and
// "immediately" notification delivery.
func TestNewMinuteJobs_RunServices(t *testing.T) {
	pub := &fakePublisher{}
	del := &fakeDeliverer{}
	publish, deliver := NewMinuteJobs(pub, del)
	if err := publish(context.Background()); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if err := deliver(context.Background()); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if pub.calls != 1 || del.calls != 1 {
		t.Fatalf("calls = %d/%d, want 1/1", pub.calls, del.calls)
	}

	pub.err = errors.New("db down")
	if err := publish(context.Background()); err == nil {
		t.Fatalf("publish error must surface to the scheduler")
	}

	nilPublish, nilDeliver := NewMinuteJobs(nil, nil)
	if nilPublish(context.Background()) != nil || nilDeliver(context.Background()) != nil {
		t.Fatalf("nil services must be no-ops")
	}
}

func TestEveryTickPredicate(t *testing.T) {
	pred := EveryTick()
	for _, ts := range []time.Time{time.Date(2026, 5, 4, 3, 1, 0, 0, time.UTC), time.Date(2026, 5, 9, 23, 59, 0, 0, time.UTC)} {
		if !pred(ts) {
			t.Fatalf("EveryTick must fire at %v", ts)
		}
	}
}

// Ticker jitter (a tick landing just under one interval after the last)
// must not skip an EveryTick job.
func TestTickJitterDoesNotSkipRuns(t *testing.T) {
	s := NewScheduler(time.Minute)
	base := time.Date(2026, 5, 4, 7, 0, 0, 0, time.UTC)
	runs := 0
	s.Register("every", EveryTick(), func(context.Context) error { runs++; return nil })

	s.now = func() time.Time { return base }
	s.tick(context.Background())
	s.now = func() time.Time { return base.Add(time.Minute - time.Millisecond) }
	s.tick(context.Background())
	if runs != 2 {
		t.Fatalf("runs = %d, want 2", runs)
	}
}
