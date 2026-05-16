package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore is a cluster-shared Store implementation backed by a Redis
// server. It implements the sliding-window-counter algorithm via a single
// atomic Lua script: each key is a Redis sorted set (ZSET) of request
// timestamps (milliseconds since the Unix epoch), trimmed to the active
// window on every call.
//
// 13.6.A — required for multi-pod deploys. The per-limiter in-memory map
// in ratelimit.go keeps state on the pod that handled the request; an
// attacker who rotates between N replicas gets N × budget. RedisStore
// puts the counter in one place every pod consults.
//
// Fail-open contract (deliberate): If Redis is unreachable on a single
// Allow call, we LOG A WARNING and return allowed=true. The state
// contract is "rate limiting is a brownout protection, not a security
// boundary — the API stays up even if the limiter backend hiccups." If
// you change this to fail-closed, every Redis outage takes the entire
// API down with it. Don't.
type RedisStore struct {
	client *redis.Client
	script *redis.Script
	// instanceID disambiguates ZSET members across pods so two replicas
	// admitting requests in the same millisecond don't collide on a
	// single ZADD member.
	instanceID string
	// memberSeq is a monotonic per-process counter appended to the
	// ZADD member so successive calls within the same millisecond on
	// the SAME pod also stay distinct.
	memberSeq uint64
}

// slidingWindowScript is the atomic check-and-increment. KEYS[1] is the
// rate-limit key. ARGV is now-millis, window-millis, max-requests,
// unique-member (caller-supplied so successive calls within the same
// millisecond don't collide as ZADD members and silently overwrite).
// Returns {allowed (1|0), remaining, retry_after_ms}.
const slidingWindowScript = `
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local max = tonumber(ARGV[3])
local member = ARGV[4]
redis.call('ZREMRANGEBYSCORE', key, '-inf', now - window)
local count = redis.call('ZCARD', key)
if count < max then
    redis.call('ZADD', key, now, member)
    redis.call('PEXPIRE', key, math.ceil(window / 1000) * 1000)
    return {1, max - count - 1, 0}
end
local oldest = tonumber(redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')[2])
return {0, 0, oldest + window - now}
`

// NewRedisStore parses a Redis URL (redis://[user:pass@]host:port/db) and
// returns a Store implementation that consults the resulting Redis
// client. PING is issued at construction so a misconfigured REDIS_URL
// surfaces at boot, not on the first rate-limit check.
func NewRedisStore(url string) (*RedisStore, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse REDIS_URL: %w", err)
	}
	client := redis.NewClient(opts)

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	instanceID, err := randomInstanceID()
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("generate instance id: %w", err)
	}

	return &RedisStore{
		client:     client,
		script:     redis.NewScript(slidingWindowScript),
		instanceID: instanceID,
	}, nil
}

// randomInstanceID returns 8 random hex bytes — short enough to keep the
// ZSET cheap, random enough to collide-proof multi-pod members.
func randomInstanceID() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// Close releases the underlying Redis client. Safe to call multiple times.
func (rs *RedisStore) Close() error {
	if rs == nil || rs.client == nil {
		return nil
	}
	return rs.client.Close()
}

// Allow runs the sliding-window-counter script against Redis.
//
// Fail-open: if the Redis call errors (connection lost, timeout, script
// rejection, etc.) we log a warning and return allowed=true. See type
// docstring for the rationale. A failure mode that lets the API stay up
// with degraded rate limiting beats one that hard-fails the whole API
// when Redis hiccups.
func (rs *RedisStore) Allow(key string, maxRequests int, window time.Duration) (allowed bool, remaining int, retryAfter time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	nowMs := time.Now().UnixMilli()
	windowMs := window.Milliseconds()
	seq := atomic.AddUint64(&rs.memberSeq, 1)
	member := strconv.FormatInt(nowMs, 10) + ":" + rs.instanceID + ":" + strconv.FormatUint(seq, 10)

	res, err := rs.script.Run(ctx, rs.client, []string{key}, nowMs, windowMs, maxRequests, member).Result()
	if err != nil {
		// Fail-open: a rate-limit backend outage MUST NOT take the API
		// down. We log + admit the request so traffic keeps flowing.
		slog.Warn("redis rate-limit backend unavailable; failing open",
			"key", key, "err", err)
		return true, maxRequests, 0
	}

	arr, ok := res.([]interface{})
	if !ok || len(arr) < 3 {
		slog.Warn("redis rate-limit script returned unexpected shape; failing open",
			"key", key, "result", res)
		return true, maxRequests, 0
	}

	allowedI, _ := arr[0].(int64)
	remainingI, _ := arr[1].(int64)
	retryMsI, _ := arr[2].(int64)

	allowed = allowedI == 1
	remaining = int(remainingI)
	if retryMsI > 0 {
		retryAfter = time.Duration(retryMsI) * time.Millisecond
	}
	return allowed, remaining, retryAfter
}
