package scheduler

import (
	"context"
	"hash/fnv"

	"gorm.io/gorm"
)

// PGLeaderLock implements LeaderLock via Postgres session-level advisory
// locks (pg_try_advisory_lock). The lock key is a stable hash of
// "scheduler:" + jobName + ":" + window. The lock is held for the
// duration of the wrapped function and released via pg_advisory_unlock
// on return (success or failure). This needs a session-bound *sql.DB
// connection — we pin one via gorm.WithContext and a manual conn.
//
// 13.7 — replaces the implicit single-replica assumption in the
// legacy scheduler. Multi-pod deployments wire this in main.go after
// the database is up.
type PGLeaderLock struct {
	db *gorm.DB
}

func NewPGLeaderLock(db *gorm.DB) *PGLeaderLock {
	return &PGLeaderLock{db: db}
}

// WithLock acquires a Postgres advisory lock for (jobName, window).
// Returns ran=true iff the lock was acquired and the function ran.
// Returns ran=false (err=nil) when another pod holds the lock; the
// caller should treat that as a successful skip.
func (l *PGLeaderLock) WithLock(ctx context.Context, jobName, window string, fn func(context.Context) error) (bool, error) {
	if l.db == nil {
		// No-DB fallback: run unconditionally. This branch only fires
		// in tests that wire a nil DB; production always has a real
		// connection.
		return true, fn(ctx)
	}
	key := advisoryKey("scheduler:" + jobName + ":" + window)

	// Pull a dedicated session-bound connection — advisory locks live
	// on the connection, not the session, so we must hold this same
	// connection from acquire to release.
	sqlDB, err := l.db.DB()
	if err != nil {
		return false, err
	}
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close()

	var acquired bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&acquired); err != nil {
		return false, err
	}
	if !acquired {
		return false, nil
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", key)
	}()

	if err := fn(ctx); err != nil {
		return true, err
	}
	return true, nil
}

// advisoryKey hashes a string to the int8 Postgres requires.
// pg_try_advisory_lock takes a single bigint; fnv-64 spreads collisions
// across the int64 space deterministically.
func advisoryKey(s string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return int64(h.Sum64())
}
