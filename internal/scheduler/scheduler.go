// Package scheduler provides a lightweight cron-style job runner for periodic
// background tasks (e.g. digest notification delivery).
//
// The scheduler ticks once per hour and asks each registered job whether it
// should run "now" via a predicate function. This is intentionally simple — it
// is not a replacement for a full cron library, but it is dependency-free,
// deterministic, and easy to test.
package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// JobFunc is the unit of work executed by the scheduler.
type JobFunc func(ctx context.Context) error

// PredicateFunc decides whether the job should fire for a given wall-clock
// time. The clock is injected so tests can use a fake.
type PredicateFunc func(now time.Time) bool

// Job is a registered scheduler entry.
type Job struct {
	Name      string
	Predicate PredicateFunc
	Run       JobFunc
}

// Scheduler runs registered jobs on a fixed tick interval.
type Scheduler struct {
	jobs     []Job
	interval time.Duration
	now      func() time.Time

	mu      sync.Mutex
	cancel  context.CancelFunc
	stopped chan struct{}
	// lastRun guards against running the same job twice within one window
	// when the predicate would otherwise be true for several ticks in a row.
	lastRun map[string]time.Time
}

// NewScheduler creates a scheduler that ticks every interval. Pass 0 to use
// the default 1-hour interval suitable for the day-of-week / hour-of-day
// predicates used by digest jobs.
func NewScheduler(interval time.Duration) *Scheduler {
	if interval <= 0 {
		interval = time.Hour
	}
	return &Scheduler{
		jobs:     nil,
		interval: interval,
		now:      time.Now,
		lastRun:  make(map[string]time.Time),
	}
}

// Register adds a job to the scheduler. Must be called before Start.
func (s *Scheduler) Register(name string, predicate PredicateFunc, run JobFunc) {
	s.jobs = append(s.jobs, Job{Name: name, Predicate: predicate, Run: run})
}

// Start begins ticking. It returns immediately; the loop runs in its own
// goroutine and exits when ctx is cancelled or Stop is called.
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return // already running
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.stopped = make(chan struct{})
	s.mu.Unlock()

	go s.loop(runCtx)
}

// Stop cancels the scheduler and waits for the loop goroutine to exit.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	stopped := s.stopped
	s.cancel = nil
	s.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	if stopped != nil {
		<-stopped
	}
}

func (s *Scheduler) loop(ctx context.Context) {
	defer close(s.stopped)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	slog.Info("scheduler started", "jobs", len(s.jobs), "interval", s.interval)
	// Fire predicates immediately so jobs scheduled for the current hour
	// don't have to wait a full interval after process start.
	s.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			slog.Info("scheduler stopping")
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	now := s.now()
	for _, job := range s.jobs {
		if !job.Predicate(now) {
			continue
		}
		// Coalesce: skip if we already ran this hour.
		if last, ok := s.lastRun[job.Name]; ok && now.Sub(last) < s.interval {
			continue
		}
		s.lastRun[job.Name] = now
		slog.Info("scheduler job starting", "job", job.Name, "at", now.Format(time.RFC3339))
		start := time.Now()
		if err := job.Run(ctx); err != nil {
			slog.Error("scheduler job failed", "job", job.Name, "err", err, "duration", time.Since(start))
			continue
		}
		slog.Info("scheduler job finished", "job", job.Name, "duration", time.Since(start))
	}
}

// DailyAt returns a predicate that fires every day at the specified hour
// (0-23) in the local timezone.
func DailyAt(hour int) PredicateFunc {
	return func(now time.Time) bool {
		return now.Hour() == hour
	}
}

// WeeklyAt returns a predicate that fires on the given weekday at the given
// hour (0-23) in the local timezone.
func WeeklyAt(day time.Weekday, hour int) PredicateFunc {
	return func(now time.Time) bool {
		return now.Weekday() == day && now.Hour() == hour
	}
}
