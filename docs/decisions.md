# Decisions

Every lasting decision about gastos.app, newest first, with a link to where it is argued. New decisions get a line here in the same PR that makes them. A decision that replaces an older one says so, and the older line stays.

Entries marked *reconstructed* were written later from the git history, because no design doc existed at the time.

## 2026-11

| Date | Decision | Where |
|---|---|---|
| 2026-11-27 | Internal docs, code, comments, commits, PRs and issues are in English. UI copy stays pt-BR. The README is bilingual. | [AGENTS.md](../AGENTS.md), #72 |
| 2026-11-27 | Every PR closes an issue. Issues use forms (bug, feature, task). Vulnerabilities are reported privately. | `.github/`, #72 |
| 2026-11-27 | CI supply chain: actions pinned by SHA, zizmor, CodeQL, no npm install scripts, 7-day cooldown on new releases, Dependabot security updates on, e2e required on master. | [security.md](security.md), #71 |
| 2026-11-27 | Go follows the Uber Go Style Guide, enforced by golangci-lint on new code. Every declaration has a doc comment. | [AGENTS.md](../AGENTS.md), #70 |
| 2026-11-27 | Go layout stays the go.dev one (`cmd/`, `internal/<domain>`), with no `pkg/` and no layered folders. The web app lives in `web/` in the same repo, because it ships embedded in the Go binary. | [AGENTS.md](../AGENTS.md) |
| 2026-11-27 | `web/src` splits into `app/`, `ui/`, `features/<domain>/` and `pages/`, with import rules checked by a test. | #69, #74 |
| 2026-11-25 | A credit card purchase counts in the month its bill is due, not the month it was made. Each member picks the payment method new transactions start with. | [spec](superpowers/specs/2026-11-25-credit-cards-design.md) |
| 2026-11-14 | Sign-up needs a single-use invite code from `gastos invite`. The fixed `GASTOS_INVITE_CODE` becomes optional. *Reconstructed.* | commits of 2026-11-14, [vdi-setup](ops/vdi-setup.md) |
| 2026-11-14 | A transaction dated after today is upcoming, not realized, and the month shows its upcoming recurring entries. *Reconstructed.* | commits of 2026-11-14 |
| 2026-11-09 | Security headers on every response and cross-site API writes refused by `Sec-Fetch-Site`. A failed login cannot lock out the right password. *Reconstructed.* | commits of 2026-11-09, [security.md](security.md) |
| 2026-11-09 | The app is installable (PWA manifest and icons). *Reconstructed.* | commits of 2026-11-09 |

## 2026-10

| Date | Decision | Where |
|---|---|---|
| 2026-10-15 | An assistant answers questions about the user's own data, with the user's Anthropic key, encrypted at rest under a server key. Read-only tools. | [spec](superpowers/specs/2026-10-15-query-assistant-design.md), [assistant.md](assistant.md) |
| 2026-10-15 | Pluggy's API fits the inbox, but the Open Finance integration waits for a second spike with a real account. Until then, statement import (OFX, CSV) is the way in for bank data. | [spike](superpowers/specs/2026-10-15-pluggy-spike.md) |
| 2026-10-11 | The UI is rebuilt minimalist, one path per action, Apple-like. Replaces the visual part of the 2026-10-09 frontend design. | [spec](superpowers/specs/2026-10-11-ui-redesign-design.md) |
| 2026-10-11 | Deploys run only on a push to master, to production. No staging. | [deploy spec](superpowers/specs/2026-10-09-deploy-pipeline-design.md), [vdi-setup](ops/vdi-setup.md) |
| 2026-10-11 | Daily encrypted off-site backup to a private repository. | [backup](ops/backup.md) |
| 2026-10-11 | Sign-up closes after the first user unless there is an invite code. | [plan](superpowers/plans/2026-10-11-registration-lock.md) |
| 2026-10-09 | Rewrite: new schema and API v2 under `cmd/` and `internal/`, a React frontend in `web/`, deploy from GitHub to a VPS, then bank import. From here on, schema changes are in-place migrations: there is real data to keep. | [roadmap](superpowers/specs/2026-10-09-roadmap.md), [backend](superpowers/specs/2026-10-09-backend-foundation-design.md), [frontend](superpowers/specs/2026-10-09-frontend-rewrite-design.md), [bank](superpowers/specs/2026-10-09-open-finance-design.md) |

## 2026-06

| Date | Decision | Where |
|---|---|---|
| 2026-06-15 | Unit tests for the Go API with a real SQLite. | [spec](superpowers/specs/2026-06-15-unit-tests-design.md) |
| 2026-06-15 | Recurring income and recurring split payments, built like recurring expenses. | [spec](superpowers/specs/2026-06-15-recurring-incomes-payments-design.md) |
| 2026-06-13 | Categories and payment methods are per account and editable, no longer fixed in the frontend. | [spec](superpowers/specs/2026-06-13-custom-categories-design.md) |
| 2026-06-13 | Apache-2.0 license. *Reconstructed.* | `LICENSE` |

## Before June 2026 (*reconstructed*)

| Date | Decision | Where |
|---|---|---|
| 2026-05-04 | Expenses can be split between people, and split payments (settlements) are recorded on their own page. | PRs #1 and #2 |
| 2026-04-16 | One user can have several accounts (the later wallets). | commit of 2026-04-16 |
| 2026-04-14 | Go backend with SQLite and a single-page frontend (Alpine.js, Chart.js) served by the same binary, mobile first. | first commits |
