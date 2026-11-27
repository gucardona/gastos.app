# CI/CD hygiene — plan

**Date:** 2026-10-11
**Status:** to implement

## Goal

Catch early what the tests do not catch (formatting, static bugs, vulnerable dependencies) and make the deploy safer. No paid services or new accounts: everything runs on GitHub Actions.

## What is included

| Item | How | Why |
|---|---|---|
| Go formatting | `test -z "$(gofmt -l .)"` in the `backend` job | a file once got out of format and only `gofmt` caught it |
| Go lint | `golangci-lint` with the default set (`govet`, `errcheck`, `staticcheck`, `ineffassign`, `unused`) | ignored errors and dead code |
| Go vulnerabilities | `govulncheck ./...` | warns about CVEs in dependencies **that the code actually calls** |
| Frontend lint | ESLint (`typescript-eslint`, `react-hooks`) in `npm run lint` | today there are `eslint-disable` comments with no ESLint configured |
| npm vulnerabilities | `npm audit --omit=dev --audit-level=high` | production dependencies |
| Updates | Weekly Dependabot for `gomod`, `npm` (in `web/`) and `github-actions`, grouping minor and patch | small PRs instead of accumulated debt |
| Deploy without races | `concurrency: { group: deploy, cancel-in-progress: false }` | two merges in a row do not run the script at the same time |
| Smoke test | after the deploy, `curl -fsS https://gastos.gupa.dev/healthz` with 10 attempts | the script's local `healthz` does not prove that Caddy and DNS work |
| e2e on mobile | second Playwright project with a 390×844 viewport | we only test at 1280 px |
| Backup test | `bash scripts/offsite-backup.test.sh` in the `backend` job | the backup script must keep working |

## Tasks

- [ ] **1. gofmt and golangci-lint** in the `backend` job; fix whatever shows up (the repository must be clean for the job to pass).
- [ ] **2. govulncheck** as a separate job (`vuln`), so a vulnerability failure does not hide a test failure.
- [ ] **3. ESLint** in `web/`: `eslint.config.js`, `lint` script, step in the `web` job; fix whatever shows up.
- [ ] **4. `npm audit`** in the `vuln` job.
- [ ] **5. Dependabot** (`.github/dependabot.yml`).
- [ ] **6. Deploy:** `concurrency` and a smoke test through the public URL. The smoke test only runs when `DEPLOY_HOST` exists; the URL comes from the repository variable `PUBLIC_URL` (without it, the step is skipped).
- [ ] **7. e2e mobile:** `mobile` project in `playwright.config.ts`; adjust selectors that depend on the desktop layout (the "Novo lançamento" (New transaction) button becomes the floating `+`).
- [ ] **8. Backup script test** in the `backend` job.

## Out of scope

CodeQL (`govulncheck` and `npm audit` already cover dependencies; deep static analysis can come later), SBOM, binary signing, Docker image.

## Risk

A new lint usually finds things in existing code. The rule is to fix the finding instead of relaxing the rule; only if it is real noise is the specific rule turned off, with a comment saying why.
