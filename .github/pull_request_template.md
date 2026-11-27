<!-- Every PR closes an issue. Open one first if there is none. -->
Closes #

## What changes

<!-- What this PR does, in a few lines. Link the design doc if there is one. -->

## Why

<!-- The problem it solves, or the decision behind it. -->

## How it was tested

<!-- Commands you ran and what you checked by hand (screens, phone, desktop). -->

- [ ] `go test -race ./...`
- [ ] `npm run lint && npm run typecheck && npm test` (in `web/`)
- [ ] `npm run build && npm run e2e` when the web app changes

## Risk

- [ ] Changes the database schema (a new migration in `internal/db/migrations/`, never an edit to an old one)
- [ ] Touches auth, sessions, secrets or money math
- [ ] Changes what users see (screenshots below)

<!-- Anything a reviewer should look at closely, and how to roll back if needed. -->
