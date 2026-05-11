#!/usr/bin/env bash
# Paper LMS — Emergency Rollback Script
# Usage: ./scripts/deploy-rollback.sh [--restore-db] [--yes]
#
# Rolls back to the previous deployment:
#   - Restores previous Docker images (code rollback)
#   - Optionally restores the pre-deploy database backup (--restore-db)
#
# This script is intentionally separate from deploy.sh so it can be
# run independently in an emergency.

set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_FILE="${COMPOSE_FILE:-$PROJECT_DIR/deployments/docker/docker-compose.prod.yml}"
DEPLOY_DIR="$PROJECT_DIR/.deploy"
LOG_DIR="$PROJECT_DIR/logs/deploys"
HEALTH_URL="http://127.0.0.1:3000/health"
HEALTH_TIMEOUT=30

# ---------------------------------------------------------------------------
# Parse arguments
# ---------------------------------------------------------------------------
RESTORE_DB=false
AUTO_YES=false

for arg in "$@"; do
  case "$arg" in
    --restore-db) RESTORE_DB=true ;;
    --yes|-y)     AUTO_YES=true ;;
    *)            echo "Unknown option: $arg"; exit 1 ;;
  esac
done

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log()   { echo -e "${BLUE}[ROLLBACK]${NC} $*"; }
ok()    { echo -e "${GREEN}[   OK   ]${NC} $*"; }
warn()  { echo -e "${YELLOW}[  WARN  ]${NC} $*"; }
fail()  { echo -e "${RED}[ FAILED ]${NC} $*"; exit 1; }

confirm() {
  if [ "$AUTO_YES" = true ]; then return 0; fi
  read -r -p "$1 [y/N] " answer
  [[ "$answer" =~ ^[yY]$ ]] || { log "Aborted."; exit 0; }
}

mkdir -p "$LOG_DIR"

# ---------------------------------------------------------------------------
# Step 1: Read rollback metadata
# ---------------------------------------------------------------------------
echo ""
log "================================================="
log "  Paper LMS — Emergency Rollback"
log "  Time: $(date)"
log "================================================="
echo ""

if [ ! -f "$DEPLOY_DIR/previous_deploy" ]; then
  fail "No rollback metadata found at $DEPLOY_DIR/previous_deploy"
fi

# shellcheck source=/dev/null
source "$DEPLOY_DIR/previous_deploy"

log "Previous deployment info:"
log "  Previous version:    ${previous_version:-unknown}"
log "  Migration version:   ${migration_version:-unknown}"
log "  Backup file:         ${backup_filename:-none}"
log "  Deploy timestamp:    ${deploy_timestamp:-unknown}"

if [ "${rolled_back:-false}" = "true" ]; then
  warn "This deployment has already been rolled back."
  confirm "Continue anyway?"
fi

echo ""

# ---------------------------------------------------------------------------
# Step 2: Verify previous images exist
# ---------------------------------------------------------------------------
log "Checking for previous images..."

BACKEND_EXISTS=false
FRONTEND_EXISTS=false

if docker image inspect paper-lms-backend:previous >/dev/null 2>&1; then
  BACKEND_EXISTS=true
  ok "paper-lms-backend:previous exists"
else
  fail "paper-lms-backend:previous image not found. Cannot rollback."
fi

if docker image inspect paper-lms-frontend:previous >/dev/null 2>&1; then
  FRONTEND_EXISTS=true
  ok "paper-lms-frontend:previous exists"
else
  fail "paper-lms-frontend:previous image not found. Cannot rollback."
fi

echo ""

# ---------------------------------------------------------------------------
# Step 3: DB restore warning
# ---------------------------------------------------------------------------
if [ "$RESTORE_DB" = true ]; then
  if [ "${backup_filename:-none}" = "none" ]; then
    fail "No pre-deploy backup was created. Cannot restore database."
  fi

  warn "================================================="
  warn "  DATABASE RESTORE WARNING"
  warn ""
  warn "  This will restore the database to its state"
  warn "  BEFORE the last deployment."
  warn ""
  warn "  ANY DATA ENTERED AFTER THE DEPLOYMENT WILL BE LOST."
  warn "  This includes: new users, submissions, grades,"
  warn "  enrollments, and all other changes."
  warn ""
  warn "  Backup file: ${backup_filename}"
  warn "================================================="
  echo ""
fi

# ---------------------------------------------------------------------------
# Step 4: Confirmation
# ---------------------------------------------------------------------------
if [ "$RESTORE_DB" = true ]; then
  confirm "Rollback code AND restore database from ${backup_filename}? THIS WILL LOSE DATA."
else
  confirm "Rollback to previous version (${previous_version:-unknown})?"
fi

echo ""

# ---------------------------------------------------------------------------
# Step 5: Stop backend and frontend
# ---------------------------------------------------------------------------
log "Stopping backend and frontend..."
docker compose -f "$COMPOSE_FILE" stop backend frontend
ok "Services stopped"

echo ""

# ---------------------------------------------------------------------------
# Step 6: Restore database (if requested)
# ---------------------------------------------------------------------------
if [ "$RESTORE_DB" = true ]; then
  log "Restoring database from ${backup_filename}..."

  # Ensure postgres is still running
  if ! docker compose -f "$COMPOSE_FILE" ps postgres 2>/dev/null | grep -q "Up"; then
    log "Starting PostgreSQL..."
    docker compose -f "$COMPOSE_FILE" up -d postgres
    sleep 5
  fi

  # Restore via the backup container
  docker compose -f "$COMPOSE_FILE" run --rm \
    backup sh -c "gunzip -c /backups/${backup_filename} | psql \"\$DATABASE_URL\" --quiet --single-transaction"

  ok "Database restored from ${backup_filename}"
  echo ""
fi

# ---------------------------------------------------------------------------
# Step 7: Retag previous images
# ---------------------------------------------------------------------------
log "Retagging images..."
docker tag paper-lms-backend:previous paper-lms-backend:latest
ok "paper-lms-backend:previous → :latest"

docker tag paper-lms-frontend:previous paper-lms-frontend:latest
ok "paper-lms-frontend:previous → :latest"

echo ""

# ---------------------------------------------------------------------------
# Step 8: Start backend, wait for health
# ---------------------------------------------------------------------------
log "Starting backend..."
docker compose -f "$COMPOSE_FILE" up -d --no-deps backend

log "Waiting for backend health check (timeout: ${HEALTH_TIMEOUT}s)..."
HEALTHY=false
for i in $(seq 1 $HEALTH_TIMEOUT); do
  if curl -sf "$HEALTH_URL" >/dev/null 2>&1; then
    HEALTHY=true
    ok "Backend healthy after ${i}s"
    break
  fi
  sleep 1
done

if [ "$HEALTHY" = false ]; then
  fail "Backend failed health check after rollback. Manual intervention required."
fi

echo ""

# ---------------------------------------------------------------------------
# Step 9: Start frontend
# ---------------------------------------------------------------------------
log "Starting frontend..."
docker compose -f "$COMPOSE_FILE" up -d --no-deps frontend
sleep 3

if docker compose -f "$COMPOSE_FILE" ps frontend 2>/dev/null | grep -q "Up"; then
  ok "Frontend started"
else
  warn "Frontend may not have started correctly. Check: docker compose -f $COMPOSE_FILE ps"
fi

# Mark as rolled back
if [ -f "$DEPLOY_DIR/previous_deploy" ]; then
  sed -i.bak 's/rolled_back=false/rolled_back=true/' "$DEPLOY_DIR/previous_deploy" 2>/dev/null || \
    sed -i '' 's/rolled_back=false/rolled_back=true/' "$DEPLOY_DIR/previous_deploy" 2>/dev/null || true
  rm -f "$DEPLOY_DIR/previous_deploy.bak"
fi

# Restore previous version marker
if [ -n "${previous_version:-}" ] && [ "$previous_version" != "unknown" ]; then
  echo "$previous_version" > "$DEPLOY_DIR/current_version"
fi

echo ""
log "================================================="
log "  ROLLBACK COMPLETE"
log ""
log "  Rolled back to: ${previous_version:-unknown}"
if [ "$RESTORE_DB" = true ]; then
  log "  Database restored from: ${backup_filename}"
fi
log ""
log "  Service status:"
docker compose -f "$COMPOSE_FILE" ps
log "================================================="
