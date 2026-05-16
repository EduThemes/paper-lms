# Multi-Pod Verification Runbook (Wave E)

A state DPA reviewer will ask: "What stops two replicas of your server
from double-sending the daily digest, or letting a student burn
through 2× the rate limit by hitting a load-balancer that rotates
across pods?" This runbook is the answer — the commands a reviewer
runs to observe the cross-pod guarantees in action.

Three properties are verified:

1. **Scheduler advisory lock holds.** Only one pod fires each
   scheduled job (digest, snapshot, cleanup) per cycle. Implemented by
   `internal/scheduler/pg_leader_lock.go` using
   `pg_try_advisory_lock(key)` where the key is an FNV-64 hash of the
   job name.
2. **REDIS_URL-shared rate-limit budget held across pods.** When
   `REDIS_URL` is set, the rate-limit `Store` uses `RedisStore`
   (Wave C.4); requests through any replica decrement the same Lua-
   atomic sliding-window counter.
3. **`/metrics` scraped by Prometheus.** Each pod exposes its own
   process metrics on `/metrics`; the Prometheus job multiplexes them
   for aggregate analysis.

This is a **runbook, not executable code** — copy-paste against a
running 3-replica deploy and verify the observable signals match.

## Prerequisites

- Docker + docker-compose v2 OR kubectl against any cluster
- `REDIS_URL` env var pointing at a reachable Redis (the compose
  stack ships one under the `redis` profile)
- `DATABASE_URL` env var pointing at a Postgres ≥ 14 with the
  migration chain applied through 000056

## Section 1 — 3-replica deploy

### Docker Compose path

The repo's `docker-compose.yml` ships a single-replica `server`
service. To verify multi-pod behavior, run with `--scale`:

```bash
# From repo root:
docker compose up -d --build postgres redis
docker compose up -d --scale server=3 server

# Confirm three pods:
docker compose ps server
# Expect three rows: paper-lms-server-1, -2, -3 — all "running (healthy)".
```

### kubectl path

If your deploy is Kubernetes-backed, scale a Deployment / StatefulSet:

```bash
kubectl scale deployment/paper-lms --replicas=3
kubectl get pods -l app=paper-lms
# Expect three pods in Ready state.
```

## Section 2 — Scheduler advisory lock holds

The scheduler fires registered jobs on cron expressions defined in
`internal/scheduler/scheduler.go`. Without the advisory lock, every
replica would fire every job — three replicas means three duplicate
daily digests at 6 a.m.

### Verify

```bash
# Tail logs from all three pods simultaneously. The line we care
# about is "scheduler: job fired" (or whatever the production
# log line is — confirm against internal/scheduler/scheduler.go
# before running).
docker compose logs -f --tail=0 server | grep "scheduler: job fired"

# In a second terminal, advance the test clock by triggering a job
# manually (or wait for the next cron fire). For a manual trigger
# in dev:
docker compose exec server-1 \
  curl -s -X POST http://localhost:8080/internal/scheduler/trigger/daily_digest \
    -H "Authorization: Bearer $INTERNAL_TOKEN"
```

### Expected signal

Exactly **one** "job fired: daily_digest" line across all three pods,
within a single firing window. The other two pods log "job skipped:
lock held elsewhere" (or equivalent) and exit the work function.

### How it works

`pg_leader_lock.go:55` calls
`SELECT pg_try_advisory_lock(<fnv64(job_name)>)`. The first pod to
acquire the lock owns the firing for that cycle and holds the lock
until the worker function returns. The lock is session-scoped, so a
pod crash releases the lock automatically — the next cycle elects a
new leader without manual intervention.

## Section 3 — REDIS_URL-shared rate-limit budget

When `REDIS_URL` is unset, `middleware.RateLimit` uses the in-memory
`Store` (single-pod dev). With `REDIS_URL=redis://redis:6379/0`,
`cmd/server/main.go` swaps to `middleware.NewRedisStore(url)` — a
Lua-script atomic sliding-window counter shared by every pod.

### Verify

Pick a rate-limited endpoint (default: `/api/v1/auth/login` at 10
req/min). Hit it 8 times via pod-1, then 3 times via pod-3 — the
11th request must be 429, not the 9th.

```bash
# Snapshot the budget across pods. Use the load balancer's pod-affinity
# disable flag if needed; or hit pods directly via their service IPs.

# Step A: 8 requests via pod-1
for i in $(seq 1 8); do
  curl -s -o /dev/null -w "%{http_code}\n" \
    http://paper-lms-server-1:8080/api/v1/auth/login \
    -X POST -d '{"login":"x","password":"x"}' \
    -H 'Content-Type: application/json'
done
# Expect: 401 401 401 401 401 401 401 401  (8 unauthorized, rate-limit
# budget not yet exhausted)

# Step B: 3 requests via pod-3
for i in $(seq 1 3); do
  curl -s -o /dev/null -w "%{http_code}\n" \
    http://paper-lms-server-3:8080/api/v1/auth/login \
    -X POST -d '{"login":"x","password":"x"}' \
    -H 'Content-Type: application/json'
done
# Expect: 401 401 429   (the 11th request — third on pod-3 —
# is the one that crosses the shared 10/min budget)
```

### Expected signal

Request #11 (regardless of which pod handled it) returns **429 Too
Many Requests**. If you see 11 successive 401s, `REDIS_URL` is
unset or the store swap in `cmd/server/main.go` did not fire — fix
that before signing the DPA.

### Cross-check Redis directly

```bash
docker compose exec redis redis-cli KEYS 'ratelimit:*'
# Expect entries like ratelimit:auth_login:<ip>:<window>
docker compose exec redis redis-cli ZCARD 'ratelimit:auth_login:<ip>'
# Expect a non-zero count that grows with each request.
```

## Section 4 — /metrics scraped by Prometheus

`internal/obs/metrics.go` registers process metrics on `/metrics`
(Prometheus exposition format). Each pod's `/metrics` is independent;
Prometheus stitches them via a `paper-lms` scrape job.

### Verify (without Prometheus)

```bash
# Hit each pod's /metrics individually; confirm metric names are
# stable across pods (cardinality test).
for pod in 1 2 3; do
  echo "--- pod-$pod ---"
  curl -s http://paper-lms-server-$pod:8080/metrics \
    | grep -E '^(go_goroutines|http_request_duration_seconds_count|paper_lms_)' \
    | head -10
done
```

### Expected signal

Same set of metric names across all three pods, with **different
values** (each pod owns its own counters/histograms). If pod-1 has
`http_request_duration_seconds_count{...}` and pod-2 doesn't, the
metrics registration is host-specific — fix before DPA.

### Verify (with Prometheus)

Add this scrape config and re-roll Prometheus:

```yaml
scrape_configs:
  - job_name: paper-lms
    static_configs:
      - targets:
          - paper-lms-server-1:8080
          - paper-lms-server-2:8080
          - paper-lms-server-3:8080
    metrics_path: /metrics
    scrape_interval: 15s
```

Then in the Prometheus UI:

```promql
# Aggregate request rate across all pods:
sum by (handler) (rate(http_request_duration_seconds_count[5m]))

# Per-pod scheduler-lock contention (only one pod should win each
# minute):
sum by (instance) (increase(scheduler_job_fired_total[10m]))
```

The per-pod scheduler counter should show one pod handling the bulk
of fires within any window (the others = 0 or near-0); the request
rate should be roughly evenly split if the load balancer is doing
its job.

## Section 5 — Tear down

```bash
docker compose down
# or:
kubectl scale deployment/paper-lms --replicas=1
```

## What this runbook does NOT cover

- **TLS termination** — assume the reverse proxy (Caddy / nginx /
  cloud LB) terminates TLS before the request reaches any replica.
  The state DPA covers this separately under the data-in-transit
  section.
- **Database failover** — Postgres HA is the platform team's
  responsibility; this runbook assumes a single primary.
- **Session affinity** — the rate-limit verification ASSUMES the
  load balancer does NOT pin sessions to a single pod. If your LB
  is sticky-by-default, disable affinity for the duration of the
  test.

## Sign-off checklist

- [ ] Section 1: 3 pods running, all healthy
- [ ] Section 2: scheduler fired exactly once per cycle across all
      pods
- [ ] Section 3: 11th request returns 429 regardless of routing
- [ ] Section 4: `/metrics` returns the expected metric families on
      every pod
- [ ] Tear-down clean
