# Quarterly Release Checklist

Paper LMS follows a quarterly release cycle (YYYY.QN format, e.g., 2025.Q2).

---

## 1 Week Before Release

- [ ] All features for this release are merged to `main`
- [ ] CI pipeline is green (lint, test, build, Docker)
- [ ] Review all new database migrations for safety:
  - No `DROP` or `RENAME` on existing columns/tables (use expand-contract pattern)
  - All `.up.sql` files wrapped in `BEGIN;` / `COMMIT;`
  - New columns are nullable or have defaults (so rollback-safe)
- [ ] Update `CHANGELOG.md` with release notes
- [ ] Tag the release candidate: `git tag -a YYYY.QN-rc1 -m "Release candidate 1"`

## 3 Days Before Release — Pre-Release Testing

- [ ] Pull the latest production database backup:
  ```bash
  # On the production server
  docker compose -f deployments/docker/docker-compose.prod.yml run --rm \
    backup sh -c "pg_dump \"\$DATABASE_URL\" --no-owner --no-privileges | gzip > /backups/pre_release_test.sql.gz"
  ```
- [ ] Restore the backup to a local/staging environment
- [ ] Run migrations against the production data:
  ```bash
  ./bin/migrate up
  ```
- [ ] Verify no migration errors or dirty state
- [ ] Test critical paths manually:
  - [ ] Login (local auth, SSO if configured)
  - [ ] Dashboard loads, courses listed
  - [ ] Create/edit an assignment
  - [ ] Submit as a student
  - [ ] Grade a submission
  - [ ] File uploads work
  - [ ] Any new features specific to this release

## Release Day

### 1. Sync source to the server

```bash
rsync -avz --delete \
  --exclude '.git' \
  --exclude 'web/node_modules' \
  --exclude '.env' \
  ./ user@192.168.0.12:/opt/paper-lms/
```

### 2. Deploy

```bash
# On the production server
cd /opt/paper-lms
./scripts/deploy.sh YYYY.QN
```

The deploy script will:
1. Run pre-flight checks (Docker, disk space)
2. Create a verified database backup
3. Build new Docker images
4. Run database migrations
5. Rolling-restart services with health checks
6. Auto-rollback if anything fails

### 3. Verify

- [ ] Check the deploy log: `logs/deploys/deploy_YYYY.QN_*.log`
- [ ] Verify `/health` returns `"status":"healthy"`
- [ ] Verify `/ready` returns `"ready":true`
- [ ] Spot-check the same critical paths tested in pre-release
- [ ] Check `docker compose -f deployments/docker/docker-compose.prod.yml ps` — all services healthy

### 4. If Something Goes Wrong

**Code-only rollback** (keeps the database as-is):
```bash
./scripts/deploy-rollback.sh
```

**Full rollback** (restores code AND database to pre-deploy state):
```bash
./scripts/deploy-rollback.sh --restore-db
```

> Warning: `--restore-db` will lose any data entered after the deployment (grades, submissions, etc.).

## Post-Release (Same Day + Next Day)

- [ ] Monitor for 30 minutes after deployment
- [ ] Check application logs for errors:
  ```bash
  docker compose -f deployments/docker/docker-compose.prod.yml logs --tail=100 backend
  ```
- [ ] Confirm the daily backup job runs successfully the next morning
- [ ] Tag the final release: `git tag -a YYYY.QN -m "Release YYYY.QN"`
- [ ] Notify staff that the update is live

---

## Migration Safety Rules

These rules prevent data loss during deployments:

1. **Expand-contract pattern**: Never DROP or RENAME existing columns in the same release that changes the code. Add new columns first (nullable, with defaults), deploy, then clean up old columns in the next release. This way, rolling back the code still works against the migrated schema.

2. **Wrap in transactions**: Every `.up.sql` should use `BEGIN;` / `COMMIT;`. PostgreSQL supports transactional DDL, so a failed migration rolls back atomically.

3. **No destructive downs in production**: Down migrations should only undo additions (drop new columns/tables). The `000001_init.down.sql` is dev-only.

4. **Dirty state recovery**: If a migration fails mid-way and marks the DB as dirty:
   ```bash
   # Check current state
   docker compose -f deployments/docker/docker-compose.prod.yml run --rm backend ./migrate version
   # Force to the last clean version
   docker compose -f deployments/docker/docker-compose.prod.yml run --rm backend ./migrate force <VERSION>
   # Fix the SQL, then re-run
   docker compose -f deployments/docker/docker-compose.prod.yml run --rm backend ./migrate up
   ```
