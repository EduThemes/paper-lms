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
