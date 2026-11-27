# AGENTS.md

How to work in this repository, for people and coding agents alike.

## Rules that never bend

1. **Production data is never reset.** The production database already holds real data. Schema changes go through a new migration in `internal/db/migrations/`, applied in place. Never edit `schema.sql` or a migration that has shipped, and never drop or recreate tables to "start clean".
2. **Merging to master deploys to production.** Only the maintainer merges.
3. **No secrets in the repo, in issues or in chat.** That covers the Anthropic key, `GASTOS_SECRET_KEY`, bank credentials, SSH keys and database copies. Store only the last 4 digits of a card.
4. **Refactors do not change behavior.** A PR that moves, renames or documents code must not change what the app does. A fix on a runtime path is its own PR.

## Workflow

1. Open an issue with one of the forms (bug, feature, task), or pick an existing one.
2. For anything bigger than a small fix, write a design doc in `docs/superpowers/specs/` and get it approved, then a plan in `docs/superpowers/plans/`. Record the decision in [docs/decisions.md](docs/decisions.md).
3. Branch from master: `feat/...`, `fix/...`, `chore/...`, `docs/...`, `refactor/...`.
4. Write the failing test first, then the code.
5. Open a PR with the template. The body says `Closes #N`.
6. CI must be green. The maintainer reviews and merges.

## Language

- English for code, comments, doc comments, tests, commit messages, PRs, issues and every doc in `docs/`.
- Brazilian Portuguese for everything the user sees in the app (labels, messages, errors shown on screen, the assistant's tool names).
- The README exists in both: `README.md` (English) and `README.pt-BR.md`, with the same sections.

## Go

- Layout from go.dev: `cmd/gastos`, `internal/<domain>`. No `pkg/`, `util` or `common` packages.
- Style: the [Uber Go Style Guide](https://github.com/uber-go/guide), enforced by `golangci-lint` (`.golangci.yml`). The lint checks only what a branch adds.
- Every package, and every top-level func, type, const and var, exported or not, has a doc comment that follows [Go Doc Comments](https://go.dev/doc/comment): it starts with the name and says what the thing does, plainly.
- Tests: table-driven where there are cases (`tests`, `tt`, `give`/`want`), `t.Parallel()` where safe, real SQLite through `httptest` for the API.
- Money is `int64` cents. Splits are basis points, allocated by `internal/money` so no cent is lost.

## Web app

- `web/src`: `app/` (shell and context), `ui/` (generic components), `features/<domain>/`, `pages/`, `api/`, `lib/`, `hooks/`, `styles/`, `test/`. `web/src/structure.test.ts` enforces which folder may import which.
- Tests with Testing Library: query by role, then label, then text. Use `userEvent`, not `fireEvent`.
- E2E with Playwright against the real Go binary, on a desktop and a phone viewport.

## Commands

```bash
go vet ./... && go test -race ./... && golangci-lint run
cd web && npm run lint && npm run typecheck && npm test
cd web && npm run build && npm run e2e
```

## Commits

Conventional Commits in English (`feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`, `ci:`), subject under 72 characters, a body that says why, and `Refs #N` for the issue.
