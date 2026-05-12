# Contributing to Paper LMS

Thanks for your interest! Paper LMS welcomes issues, bug reports, and pull requests from anyone — teachers, students, developers, district IT staff, accessibility advocates.

## Quick checklist before opening a PR

1. **Tests pass:** `go test ./...` and `cd web && npm run build` complete cleanly.
2. **Vet is clean:** `make vet` shows no new warnings.
3. **No new shared-file collisions:** changes to `internal/repository/interfaces.go`, `internal/api/v1/router.go`, `cmd/server/main.go`, `internal/db/postgres.go`, `web/src/App.jsx`, `web/src/services/api.js`, and `web/src/components/Layout.jsx` should be minimal and intentional. New repositories should use the local-interface pattern (a `*_interfaces.go` next to the repo) rather than growing the shared `interfaces.go`.
4. **Canvas API compatibility:** new endpoints should mirror Canvas REST shapes when there is one. Use the error format `{"errors":[{"message":"..."}]}` and Link-header pagination (RFC 5988). See `internal/api/v1/responses/`.
5. **Commit messages:** present-tense imperative ("Add discussion checkpoints", not "Added"). Reference issue numbers when relevant.

## Development setup

```bash
git clone https://github.com/EduThemes/paper-lms.git
cd paper-lms
cp .env.example .env

# Backend
go mod download
make build

# Frontend
cd web
npm install --legacy-peer-deps
npm run dev
```

Start Postgres locally (or use the docker-compose dev database) and run `make migrate-up` once.

## Architecture

A "how to add a feature" cookbook lives in [PROJECT.md](./PROJECT.md), covering:

- New API endpoint (model → repository → service → handler → router → frontend)
- Adding a field to an existing model
- Adding a React page

The same cookbook is what keeps the codebase consistent — please follow it for new work.

## Adding a new model

The SQL migration chain (`internal/db/migrations/`) and GORM's `AutoMigrate`
list (`internal/db/postgres.go`) are the two sources of truth for the schema —
production deploys read from one, dev deploys read from the other, and they
must agree. The `TestSchemaParity_Wave1` test enforces this in CI.

When you add a model:

1. Register it in `db.AutoMigrate` so dev deploys (`AUTO_MIGRATE=true`) pick it up.
2. Run `make schema-diff` — it spins up two scratch databases, compares
   AutoMigrate against the SQL chain, and prints the missing `CREATE TABLE` /
   `CREATE INDEX` statements as paste-ready SQL.
3. Save the output into a new numbered migration via `make migrate-create`
   (writes both `.up.sql` and `.down.sql`). Author the down file by reversing
   the up — drop the new tables, drop the new indexes.
4. Re-run `make schema-diff`; the diff should be empty.

## Removing or renaming a column

`TestSchemaParity_Wave1` is a hard fail on *stale columns* — columns the SQL
chain creates that no GORM model owns. That means you can't just delete a
model field; you have to bring the SQL chain with you.

When you **remove a field** from a model:

1. Run `make stale-cols`. The column you removed will appear in
   `STALE_COLUMNS.md` as the source of truth.
2. Author a `DROP COLUMN IF EXISTS` migration in the same PR. The `.down.sql`
   re-adds the column with its original type (data is lost on rollback —
   document this in the migration header).
3. Re-run `make stale-cols`. Empty report means CI will pass.

When you **rename a field** (which usually means renaming the column too,
because GORM derives column names from struct field names), the two states
need a bridge so production data isn't lost:

1. **Wave A migration**: keep the old column, add the new one (via
   AutoMigrate + a backfill migration following the `make schema-diff`
   workflow above), then `UPDATE table SET new_col = old_col WHERE new_col
   IS NULL AND old_col IS NOT NULL;`. Idempotent — safe to re-run.
2. **Wave B migration**, in a follow-up PR or at least a follow-up
   migration: `DROP COLUMN IF EXISTS old_col`. Add a deprecation-window
   comment to the `.up.sql` so operators know when the destructive change
   landed and what prerequisite migration must have run.

The `cmd/stalecols` tool prints a per-column **References** field listing
where the column name appears in Go source. Empty refs = safe to drop.
Non-empty refs = check whether it's a JSON tag, a hand-written SQL string,
or a model field that really should be reinstated.

## What we're prioritizing

The roadmap (see [CHANGELOG.md](./CHANGELOG.md) for done work):

- **Wave 2:** New Quizzes engine (LTI-based), Plagiarism Platform / originality reports, ePub / Web-Zip exports, Live Events / Caliper analytics stream.
- **Always welcome:** accessibility fixes, K-12-specific UX improvements, performance work, additional SSO providers, more thorough test coverage.

## Reporting bugs

Please include:

- Paper LMS version (commit SHA or release tag)
- Browser + OS for frontend bugs
- A minimal repro — ideally a `curl` call or a sequence of UI clicks
- Logs from the backend if available

## Reporting security issues

Please **do not** open a public issue for security reports. See [SECURITY.md](./SECURITY.md) for the disclosure process.

## Code of conduct

Be kind. Assume good faith. Paper LMS exists to make education software better for kids and teachers, and that mission deserves a friendly project. Harassment, personal attacks, or discriminatory behavior will result in a ban.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](./LICENSE) — the same license as the rest of the project.
