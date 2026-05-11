# web/scripts

## Accessibility audit (axe-core)

`run-axe.mjs` boots a headless Chromium via Puppeteer, navigates to a fixed list of public routes, and runs [axe-core](https://github.com/dequelabs/axe-core) against each one. The CI gate fails when any violation has impact `critical` or `serious`. Screenshots of failing pages are written to `web/axe-screenshots/` and uploaded as a workflow artifact.

### Run locally

```bash
cd web
npm run axe:ci
```

That builds the production bundle, serves it on `http://localhost:5174` via `serve`, and runs the audit. To target a server you already have running (e.g. `npm run dev` on port 5173):

```bash
AXE_BASE_URL=http://localhost:5173 npm run axe
```

### Configuring routes

Edit the `ROUTES` array at the top of `run-axe.mjs`. Keep the list to routes reachable without authentication so the script stays deterministic in CI.

### CI

Wired in `.github/workflows/a11y.yml`. Runs on every push to `main` and every pull request.
