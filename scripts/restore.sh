#!/usr/bin/env bash
# Paper LMS — PostgreSQL Restore Script
# Usage: ./scripts/restore.sh <backup_file>
#
# Environment variables:
#   DATABASE_URL — PostgreSQL connection string (required)

set -euo pipefail

BACKUP_FILE="${1:-}"

if [ -z "$BACKUP_FILE" ]; then
  echo "Usage: $0 <backup_file>"
  echo ""
  echo "Available backups:"
  ls -lt "${BACKUP_DIR:-./backups}"/paper_lms_*.sql.gz 2>/dev/null || echo "  No backups found."
  exit 1
fi

if [ ! -f "$BACKUP_FILE" ]; then
  echo "ERROR: Backup file not found: $BACKUP_FILE"
  exit 1
fi

if [ -z "${DATABASE_URL:-}" ]; then
  echo "ERROR: DATABASE_URL environment variable is required"
  exit 1
fi

echo "WARNING: This will drop and recreate the database content."
echo "Backup file: $BACKUP_FILE"
read -r -p "Continue? [y/N] " confirm
if [[ ! "$confirm" =~ ^[yY]$ ]]; then
  echo "Aborted."
  exit 0
fi

echo "Restoring from $BACKUP_FILE..."
gunzip -c "$BACKUP_FILE" | psql "$DATABASE_URL" --quiet --single-transaction
echo "Restore complete."
