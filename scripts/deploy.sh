#!/usr/bin/env bash
# Paper LMS — Safe Deployment Script
# Usage: ./scripts/deploy.sh [version] [--dry-run] [--skip-backup] [--yes]
#
# Performs a full deployment with:
#   - Pre-flight checks (Docker, disk space, service status)
#   - Pre-deployment database backup (verified)
#   - Image build with version tagging
#   - Database migrations
#   - Rolling restart with health checks
#   - Auto-rollback on failure
#
# Environment:
#   COMPOSE_FILE — Path to docker-compose.prod.yml (auto-detected)
#   POSTGRES_PASSWORD — Required (loaded from .env)

set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_FILE="${COMPOSE_FILE:-$PROJECT_DIR/deployments/docker/docker-compose.prod.yml}"
# Optional override (host-specific topology — e.g. host-nginx-terminates-TLS,
# port remap, extra env passthrough). Loaded when present; ignored otherwise.
# Critical: without this, deploy.sh recreates containers without the override's
# env vars + port remap and the next deploy 502s + restart-loops.
OVERRIDE_FILE="${OVERRIDE_FILE:-$PROJECT_DIR/deployments/docker/docker-compose.override.yml}"
# Project-root .env. docker compose's default search is the compose-file's
# directory, which misses the canonical location at the repo root.
ENV_FILE="${ENV_FILE:-$PROJECT_DIR/.env}"
DEPLOY_DIR="$PROJECT_DIR/.deploy"
LOG_DIR="$PROJECT_DIR/logs/deploys"
# Health checks run INSIDE the backend container via `dc exec`. We don't
# curl the host's 127.0.0.1:3000 because (a) the backend port isn't published
# to the host on the standard topology — host nginx terminates TLS and proxies
# through the frontend container — and (b) on multi-tenant hosts something
# else may already own port 3000, which would give a false positive against
# the wrong service. Going through `dc exec` always lands on paper-lms.
HEALTH_PATH="/health"
READY_PATH="/ready"
HEALTH_TIMEOUT=30
MIN_DISK_MB=2048

# ---------------------------------------------------------------------------
# Parse arguments
# ---------------------------------------------------------------------------
VERSION="${1:-}"
DRY_RUN=false
SKIP_BACKUP=false
AUTO_YES=false

shift 2>/dev/null || true
for arg in "$@"; do
  case "$arg" in
    --dry-run)    DRY_RUN=true ;;
    --skip-backup) SKIP_BACKUP=true ;;
    --yes|-y)     AUTO_YES=true ;;
    *)            echo "Unknown option: $arg"; exit 1 ;;
  esac
done

if [ -z "$VERSION" ]; then
  VERSION="$(date +%Y.%m.%d)-$(git -C "$PROJECT_DIR" rev-parse --short HEAD 2>/dev/null || echo 'manual')"
fi

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
DEPLOY_LOG="$LOG_DIR/deploy_${VERSION}_${TIMESTAMP}.log"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# dc wraps `docker compose` with the canonical -f / --env-file flags so every
# invocation in this script picks up the override file (when present) and the
# project-root .env. Replaces the previous bare `docker compose -f $COMPOSE_FILE`
# pattern which silently dropped both, causing env-less restarts + host-port
# binding conflicts on hosts using docker-compose.override.yml.
dc() {
  local compose_args=(-f "$COMPOSE_FILE")
  [ -f "$OVERRIDE_FILE" ] && compose_args+=(-f "$OVERRIDE_FILE")
  [ -f "$ENV_FILE" ] && compose_args+=(--env-file "$ENV_FILE")
  docker compose "${compose_args[@]}" "$@"
}

# probe_backend curls the backend container directly via `dc exec`. Returns
# the response body on stdout, exit 0 on success / non-zero on failure. The
# alpine image ships wget, not curl. Errors and "set -e/pipefail" are
# swallowed so callers can branch on the actual response.
probe_backend() {
  local path="$1"
  set +e +o pipefail
  local out
  out=$(dc exec -T backend wget -qO- "http://127.0.0.1:3000${path}" 2>/dev/null)
  local rc=$?
  set -eo pipefail
  printf '%s' "$out"
  return $rc
}

log()   { echo -e "${BLUE}[DEPLOY]${NC} $*" | tee -a "$DEPLOY_LOG" 2>/dev/null || echo -e "${BLUE}[DEPLOY]${NC} $*"; }
ok()    { echo -e "${GREEN}[  OK  ]${NC} $*" | tee -a "$DEPLOY_LOG" 2>/dev/null || echo -e "${GREEN}[  OK  ]${NC} $*"; }
warn()  { echo -e "${YELLOW}[ WARN ]${NC} $*" | tee -a "$DEPLOY_LOG" 2>/dev/null || echo -e "${YELLOW}[ WARN ]${NC} $*"; }
fail()  { echo -e "${RED}[FAILED]${NC} $*" | tee -a "$DEPLOY_LOG" 2>/dev/null || echo -e "${RED}[FAILED]${NC} $*"; exit 1; }

confirm() {
  if [ "$AUTO_YES" = true ]; then return 0; fi
  read -r -p "$1 [y/N] " answer
  [[ "$answer" =~ ^[yY]$ ]] || { log "Aborted by user."; exit 0; }
}

dry_run_exec() {
  if [ "$DRY_RUN" = true ]; then
    log "[DRY RUN] $*"
    return 0
  fi
  "$@"
}

# ---------------------------------------------------------------------------
# Auto-rollback
# ---------------------------------------------------------------------------
rollback() {
  warn "================================================="
  warn "  DEPLOYMENT FAILED — STARTING AUTO-ROLLBACK"
  warn "================================================="

  local reason="${1:-unknown failure}"
  log "Rollback reason: $reason"

  # Retag previous images back to latest
  if docker image inspect paper-lms-backend:previous >/dev/null 2>&1; then
    log "Retagging paper-lms-backend:previous → latest"
    docker tag paper-lms-backend:previous paper-lms-backend:latest
  fi
  if docker image inspect paper-lms-frontend:previous >/dev/null 2>&1; then
    log "Retagging paper-lms-frontend:previous → latest"
    docker tag paper-lms-frontend:previous paper-lms-frontend:latest
  fi

  # Restart services with previous images
  log "Restarting backend with previous image..."
  dc up -d --no-deps backend

  log "Waiting for backend health..."
  local i=0
  while [ $i -lt $HEALTH_TIMEOUT ]; do
    if probe_backend "$HEALTH_PATH" >/dev/null 2>&1; then
      ok "Backend recovered after rollback"
      break
    fi
    sleep 1
    i=$((i + 1))
  done

  dc up -d --no-deps frontend

  warn "================================================="
  warn "  ROLLBACK COMPLETE (code only)"
  warn ""
  warn "  Database migrations were NOT rolled back."
  warn "  If the migration changed the schema in a way"
  warn "  incompatible with the previous code, run:"
  warn ""
  warn "    ./scripts/deploy-rollback.sh --restore-db"
  warn ""
  warn "  This will restore the pre-deploy DB backup."
  warn "================================================="

  exit 1
}

# ---------------------------------------------------------------------------
# Step 0: Setup
# ---------------------------------------------------------------------------
mkdir -p "$DEPLOY_DIR" "$LOG_DIR"

echo ""
log "================================================="
log "  Paper LMS Deployment"
log "  Version:  $VERSION"
log "  Time:     $(date)"
if [ "$DRY_RUN" = true ]; then
  log "  Mode:     DRY RUN (no changes will be made)"
fi
log "================================================="
echo ""

# ---------------------------------------------------------------------------
# Step 1: Pre-flight checks
# ---------------------------------------------------------------------------
log "Step 1: Pre-flight checks"

# Docker running?
docker info >/dev/null 2>&1 || fail "Docker is not running"
ok "Docker is running"

# Compose file exists?
[ -f "$COMPOSE_FILE" ] || fail "Compose file not found: $COMPOSE_FILE"
ok "Compose file found: $COMPOSE_FILE"

# .env exists?
if [ -f "$PROJECT_DIR/.env" ]; then
  ok ".env file found"
else
  warn ".env file not found — ensure environment variables are set"
fi

# Disk space check
AVAILABLE_MB=$(df -m "$PROJECT_DIR" | awk 'NR==2 {print $4}')
if [ "$AVAILABLE_MB" -lt "$MIN_DISK_MB" ]; then
  fail "Insufficient disk space: ${AVAILABLE_MB}MB available, ${MIN_DISK_MB}MB required"
fi
ok "Disk space: ${AVAILABLE_MB}MB available (minimum: ${MIN_DISK_MB}MB)"

# Record current service status
log "Current service status:"
dc ps 2>/dev/null | tee -a "$DEPLOY_LOG" || warn "No services currently running"

echo ""

# ---------------------------------------------------------------------------
# Step 2: Pre-deployment database backup
# ---------------------------------------------------------------------------
if [ "$SKIP_BACKUP" = true ]; then
  warn "Step 2: Skipping backup (--skip-backup flag)"
else
  log "Step 2: Pre-deployment database backup"

  # Check postgres is healthy
  if ! dc ps postgres 2>/dev/null | grep -q "healthy"; then
    # Try to check if it's at least running
    if ! dc ps postgres 2>/dev/null | grep -q "Up"; then
      fail "PostgreSQL is not running. Start it first: cd $PROJECT_DIR && ./scripts/deploy.sh $VERSION --skip-backup"
    fi
    warn "PostgreSQL health status unclear, attempting backup anyway..."
  else
    ok "PostgreSQL is healthy"
  fi

  BACKUP_FILENAME="pre_deploy_${VERSION}_${TIMESTAMP}.sql.gz"

  if [ "$DRY_RUN" = true ]; then
    log "[DRY RUN] Would create backup: $BACKUP_FILENAME"
  else
    log "Creating backup: $BACKUP_FILENAME"

    # Run backup via the backup container's postgres tools
    dc run --rm \
      -e BACKUP_DIR=/backups \
      -e RETENTION_DAYS=0 \
      backup sh -c "pg_dump \"\$DATABASE_URL\" --no-owner --no-privileges | gzip > /backups/$BACKUP_FILENAME"

    # Verify backup exists and has content
    BACKUP_SIZE=$(dc run --rm backup sh -c "stat -c%s /backups/$BACKUP_FILENAME 2>/dev/null || stat -f%z /backups/$BACKUP_FILENAME 2>/dev/null || echo 0")
    BACKUP_SIZE=$(echo "$BACKUP_SIZE" | tr -d '[:space:]')

    if [ "$BACKUP_SIZE" -lt 1024 ]; then
      fail "Backup file is too small (${BACKUP_SIZE} bytes). Backup may have failed."
    fi
    ok "Backup created: $BACKUP_FILENAME ($(( BACKUP_SIZE / 1024 )) KB)"

    # Verify gzip integrity
    dc run --rm backup sh -c "gzip -t /backups/$BACKUP_FILENAME"
    ok "Backup integrity verified (gzip test passed)"
  fi
fi

echo ""

# ---------------------------------------------------------------------------
# Step 3: Save rollback metadata
# ---------------------------------------------------------------------------
log "Step 3: Saving rollback metadata"

# Get current migration version if possible
CURRENT_MIGRATE_VERSION="unknown"
if dc ps backend 2>/dev/null | grep -q "Up"; then
  CURRENT_MIGRATE_VERSION=$(dc exec -T backend ./migrate version 2>/dev/null | head -1 || echo "unknown")
fi

PREVIOUS_VERSION="unknown"
if [ -f "$DEPLOY_DIR/current_version" ]; then
  PREVIOUS_VERSION=$(cat "$DEPLOY_DIR/current_version")
fi

if [ "$DRY_RUN" = true ]; then
  log "[DRY RUN] Would save rollback metadata"
else
  cat > "$DEPLOY_DIR/previous_deploy" <<EOF
previous_version=$PREVIOUS_VERSION
migration_version=$CURRENT_MIGRATE_VERSION
backup_filename=${BACKUP_FILENAME:-none}
deploy_timestamp=$TIMESTAMP
rolled_back=false
EOF

  echo "$VERSION" > "$DEPLOY_DIR/current_version"
  ok "Rollback metadata saved to $DEPLOY_DIR/previous_deploy"
fi

echo ""

# ---------------------------------------------------------------------------
# Step 4: Tag previous images
# ---------------------------------------------------------------------------
log "Step 4: Tagging previous images for rollback"

for svc in backend frontend; do
  IMAGE="paper-lms-${svc}:latest"
  if docker image inspect "$IMAGE" >/dev/null 2>&1; then
    dry_run_exec docker tag "$IMAGE" "paper-lms-${svc}:previous"
    ok "Tagged paper-lms-${svc}:latest → :previous"
  else
    warn "No existing image paper-lms-${svc}:latest (first deploy?)"
  fi
done

echo ""

# ---------------------------------------------------------------------------
# Step 5: Build new images
# ---------------------------------------------------------------------------
log "Step 5: Building new images"

if [ "$DRY_RUN" = true ]; then
  log "[DRY RUN] Would build backend and frontend images with VERSION=$VERSION"
else
  confirm "Ready to build. This will take a few minutes. Continue?"

  VERSION="$VERSION" dc build --no-cache backend frontend \
    || fail "Image build failed. Previous containers are still running."

  # Tag with version and latest
  for svc in backend frontend; do
    docker tag "paper-lms-${svc}:${VERSION}" "paper-lms-${svc}:latest" 2>/dev/null || true
    ok "Built and tagged paper-lms-${svc}:${VERSION}"
  done
fi

echo ""

# ---------------------------------------------------------------------------
# Step 6: Run database migrations
# ---------------------------------------------------------------------------
log "Step 6: Running database migrations"

if [ "$DRY_RUN" = true ]; then
  log "[DRY RUN] Would run: docker compose run --rm backend ./migrate up"
else
  log "Running migrations (old backend is still serving traffic)..."

  if ! VERSION="$VERSION" dc run --rm backend ./migrate up; then
    warn "================================================="
    warn "  MIGRATION FAILED"
    warn ""
    warn "  The old backend is still running."
    warn "  The database may be in a dirty state."
    warn ""
    # Build the manual-recovery command line with whatever -f / --env-file
    # flags this run was using, so a copy-paste actually works.
    DC_HINT="docker compose -f $COMPOSE_FILE"
    [ -f "$OVERRIDE_FILE" ] && DC_HINT="$DC_HINT -f $OVERRIDE_FILE"
    [ -f "$ENV_FILE" ] && DC_HINT="$DC_HINT --env-file $ENV_FILE"
    warn "  To check migration status:"
    warn "    $DC_HINT run --rm backend ./migrate version"
    warn ""
    warn "  If dirty, force to the last clean version:"
    warn "    $DC_HINT run --rm backend ./migrate force <VERSION>"
    warn ""
    warn "  Then fix the migration SQL and re-run this script."
    warn "================================================="
    fail "Migration failed. Deployment halted."
  fi

  ok "Migrations applied successfully"
fi

echo ""

# ---------------------------------------------------------------------------
# Step 7: Rolling restart
# ---------------------------------------------------------------------------
log "Step 7: Rolling restart"

if [ "$DRY_RUN" = true ]; then
  log "[DRY RUN] Would restart backend, wait for health, then restart frontend"
else
  # Restart backend (postgres stays up)
  log "Restarting backend..."
  VERSION="$VERSION" dc up -d --no-deps backend

  # Wait for backend health
  log "Waiting for backend health check (timeout: ${HEALTH_TIMEOUT}s)..."
  HEALTHY=false
  for i in $(seq 1 $HEALTH_TIMEOUT); do
    if probe_backend "$HEALTH_PATH" >/dev/null 2>&1; then
      HEALTHY=true
      ok "Backend healthy after ${i}s"
      break
    fi
    sleep 1
  done

  if [ "$HEALTHY" = false ]; then
    rollback "Backend failed health check after ${HEALTH_TIMEOUT}s"
  fi

  # Restart frontend
  log "Restarting frontend..."
  VERSION="$VERSION" dc up -d --no-deps frontend

  # Brief wait for frontend to start
  sleep 3

  # Check frontend is running
  if ! dc ps frontend 2>/dev/null | grep -q "Up"; then
    rollback "Frontend failed to start"
  fi
  ok "Frontend started"
fi

echo ""

# ---------------------------------------------------------------------------
# Step 8: Post-deployment verification
# ---------------------------------------------------------------------------
log "Step 8: Post-deployment verification"

if [ "$DRY_RUN" = true ]; then
  log "[DRY RUN] Would verify /health and /ready endpoints"
else
  # Check /health. Test for the substring in an `if` so a non-match doesn't
  # trip set -e/pipefail before we can branch into rollback.
  HEALTH_RESPONSE=$(probe_backend "$HEALTH_PATH" || true)
  if printf '%s' "$HEALTH_RESPONSE" | grep -q '"status":"healthy"'; then
    ok "/health → healthy"
  else
    warn "/health returned: ${HEALTH_RESPONSE:-<empty>}"
    rollback "Health check returned unhealthy status"
  fi

  # Check /ready. Same pattern; /ready returns `"ready":true` when every
  # load-bearing dependency (DB, encryption keys) is up.
  READY_RESPONSE=$(probe_backend "$READY_PATH" || true)
  if printf '%s' "$READY_RESPONSE" | grep -q '"ready":true'; then
    ok "/ready → ready"
  else
    warn "/ready returned: ${READY_RESPONSE:-<empty>}"
    rollback "Readiness check failed"
  fi

  # List all service states
  log "Service status after deployment:"
  dc ps | tee -a "$DEPLOY_LOG"
fi

echo ""

# ---------------------------------------------------------------------------
# Step 9: Success
# ---------------------------------------------------------------------------
log "================================================="
log "  DEPLOYMENT SUCCESSFUL"
log ""
log "  Version:    $VERSION"
log "  Time:       $(date)"
log "  Log:        $DEPLOY_LOG"
if [ "$SKIP_BACKUP" != true ] && [ "$DRY_RUN" != true ]; then
  log "  Backup:     $BACKUP_FILENAME"
fi
log ""
log "  To rollback (code only):"
log "    ./scripts/deploy-rollback.sh"
log ""
log "  To rollback (code + database):"
log "    ./scripts/deploy-rollback.sh --restore-db"
log "================================================="
