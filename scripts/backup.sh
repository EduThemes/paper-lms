#!/usr/bin/env bash
# Paper LMS — PostgreSQL Backup Script
# Usage: ./scripts/backup.sh [backup_dir]
#
# Environment variables:
#   DATABASE_URL  — PostgreSQL connection string (required)
#   BACKUP_DIR    — Directory to store backups (default: ./backups)
#   RETENTION_DAYS — Number of days to keep backups (default: 30)

set -euo pipefail

BACKUP_DIR="${1:-${BACKUP_DIR:-./backups}}"
RETENTION_DAYS="${RETENTION_DAYS:-30}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="${BACKUP_DIR}/paper_lms_${TIMESTAMP}.sql.gz"

if [ -z "${DATABASE_URL:-}" ]; then
  echo "ERROR: DATABASE_URL environment variable is required"
  exit 1
fi

mkdir -p "$BACKUP_DIR"

echo "Starting backup at $(date)..."
pg_dump "$DATABASE_URL" --no-owner --no-privileges | gzip > "$BACKUP_FILE"

FILESIZE=$(stat -f%z "$BACKUP_FILE" 2>/dev/null || stat -c%s "$BACKUP_FILE" 2>/dev/null)
echo "Backup created: $BACKUP_FILE ($(( FILESIZE / 1024 )) KB)"

# Prune old backups
if [ "$RETENTION_DAYS" -gt 0 ]; then
  DELETED=$(find "$BACKUP_DIR" -name "paper_lms_*.sql.gz" -mtime +"$RETENTION_DAYS" -delete -print | wc -l)
  if [ "$DELETED" -gt 0 ]; then
    echo "Pruned $DELETED backup(s) older than $RETENTION_DAYS days"
  fi
fi

echo "Backup complete."
