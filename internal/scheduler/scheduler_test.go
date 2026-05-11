package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestStartStopNoPanic(t *testing.T) {
	s := NewScheduler(10 * time.Millisecond)
	s.Register("noop", func(time.Time) bool { return false }, func(context.Context) error { return nil })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)
	// Let a couple of ticks elapse.
	time.Sleep(30 * time.Millisecond)
	s.Stop()
	// Idempotent stop should be safe.
	s.Stop()
}

func TestWeeklyAtPredicate(t *testing.T) {
	pred := WeeklyAt(time.Monday, 7)
	monday7 := time.Date(2026, 5, 4, 7, 30, 0, 0, time.Local) // a Monday
	monday8 := time.Date(2026, 5, 4, 8, 30, 0, 0, time.Local)
	tuesday7 := time.Date(2026, 5, 5, 7, 30, 0, 0, time.Local)
	if !pred(monday7) {
		t.Errorf("expected Monday 7am to fire")
	}
	if pred(monday8) {
		t.Errorf("expected Monday 8am to NOT fire")
	}
	if pred(tuesday7) {
		t.Errorf("expected Tuesday 7am to NOT fire")
	}
}

func TestDailyAtPredicate(t *testing.T) {
	pred := DailyAt(7)
	for d := 0; d < 7; d++ {
		ts := time.Date(2026, 5, 4+d, 7, 0, 0, 0, time.Local)
		if !pred(ts) {
			t.Errorf("expected daily 7am to fire on offset %d", d)
		}
	}
	if pred(time.Date(2026, 5, 4, 6, 59, 0, 0, time.Local)) {
		t.Errorf("expected 6:59am to NOT fire")
	}
}

func TestJobFiresWhenPredicateTrue(t *testing.T) {
	s := NewScheduler(5 * time.Millisecond)
	// Inject a fake clock that always returns "now is firing time".
	s.now = func() time.Time { return time.Date(2026, 5, 4, 7, 0, 0, 0, time.Local) }
	var hits int32
	s.Register("always", func(time.Time) bool { return true }, func(context.Context) error {
		atomic.AddInt32(&hits, 1)
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)
	time.Sleep(20 * time.Millisecond)
	s.Stop()
	if atomic.LoadInt32(&hits) < 1 {
		t.Errorf("expected job to fire at least once, got %d", hits)
	}
}
