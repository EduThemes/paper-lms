# Emergency Hotfix Procedure

Quick-reference for deploying urgent fixes outside the quarterly release cycle.

---

## Severity Assessment

| Severity | Examples | Response Time | Approval |
|----------|----------|--------------|----------|
| **Critical** | Login broken, data loss, security vulnerability | Immediate | Deploy now, notify after |
| **High** | Grades not saving, submissions failing, major feature broken | Same day | Notify admin, then deploy |
| **Medium** | UI bug affecting workflow, non-critical feature broken | Next business day | Standard review process |

> Medium-severity issues can usually wait for the next quarterly release.

---

## Hotfix Steps

### 1. Create and test the fix

```bash
# Create a hotfix branch
git checkout -b hotfix/YYYY-MM-DD-description main

# Make the fix
# ... edit files ...

# Run tests locally
make test
make vet

# Commit
git add -A && git commit -m "fix: description of the issue"
git checkout main && git merge hotfix/YYYY-MM-DD-description
```

### 2. Sync to the server

```bash
rsync -avz --delete \
  --exclude '.git' \
  --exclude 'web/node_modules' \
  --exclude '.env' \
  ./ user@192.168.0.12:/opt/paper-lms/
```

### 3. Deploy

```bash
# On the production server
cd /opt/paper-lms
./scripts/deploy.sh hotfix-YYYY.MM.DD --yes
```

The `--yes` flag skips confirmation prompts for faster deployment. The script still creates a database backup before deploying.

For critical issues where you need to skip the backup (e.g., disk is full):
```bash
./scripts/deploy.sh hotfix-YYYY.MM.DD --yes --skip-backup
```

### 4. Verify

```bash
# Check health
curl -s http://localhost:3000/health | python3 -m json.tool

# Check readiness
curl -s http://localhost:3000/ready | python3 -m json.tool

# Check logs for errors
docker compose -f deployments/docker/docker-compose.prod.yml logs --tail=50 backend
```

---

## Rollback

### Code-only rollback

Restores the previous Docker images. Database stays as-is. This is safe when:
- The hotfix didn't include database migrations, OR
- The migration only added new columns (expand-contract pattern)

```bash
./scripts/deploy-rollback.sh
```

### Full rollback (code + database)

Restores both the code AND the database to the state before the hotfix. Use when:
- The migration changed or removed existing columns
- The fix caused data corruption

```bash
./scripts/deploy-rollback.sh --restore-db
```

> **Warning**: `--restore-db` will lose ALL data entered since the hotfix was deployed (grades, submissions, enrollments, etc.). Only use this if the data is already compromised.

---

## Post-Hotfix

- [ ] Monitor logs for 15 minutes
- [ ] Verify the specific issue is resolved
- [ ] Notify affected staff
- [ ] Document the incident:
  - What broke
  - Root cause
  - Fix applied
  - How to prevent recurrence
- [ ] Consider if the fix needs a follow-up migration cleanup in the next quarterly release

---

## If the Hotfix Makes Things Worse

1. **Rollback immediately**: `./scripts/deploy-rollback.sh`
2. If the rollback also fails, check the deploy log in `logs/deploys/`
3. Manual recovery:
   ```bash
   cd /opt/paper-lms

   # Check what images are available
   docker images | grep paper-lms

   # Manually retag and restart
   docker tag paper-lms-backend:previous paper-lms-backend:latest
   docker tag paper-lms-frontend:previous paper-lms-frontend:latest
   docker compose -f deployments/docker/docker-compose.prod.yml up -d --no-deps backend frontend

   # If database needs restoring, list available backups
   docker compose -f deployments/docker/docker-compose.prod.yml run --rm backup ls -lt /backups/
   ```
