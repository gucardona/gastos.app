# Security

A short threat model for gastos.app: what it protects, from whom, how, and what is still open.

To report a vulnerability, use GitHub's private reporting (Security > Report a vulnerability). Do not open a public issue.

## What it protects

- Each household's financial data: transactions, splits, cards (last 4 digits only), bank statements.
- Accounts: passwords (bcrypt) and session tokens (only the SHA-256 is stored).
- Each user's Anthropic API key, encrypted with AES-GCM under `GASTOS_SECRET_KEY`.
- The production server and the deploy path into it.

## From whom

- Someone on the internet with no account.
- A signed-in user trying to reach a wallet they are not a member of, or to act above their role.
- A malicious or hijacked dependency, GitHub Action or npm package.
- A coding agent with too much access to secrets on the developer's machine.

## Controls in place

**HTTP** (`internal/api/server.go`, `auth.go`, `httpx.go`)

- Security headers on every response: CSP, `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: same-origin`, and HSTS when cookies are secure.
- CSRF: the session cookie is `SameSite=Lax` and `HttpOnly`, writes accept only JSON, and unsafe requests from another site are rejected by `Sec-Fetch-Site`.
- Login is rate-limited by IP and by e-mail (10 per 15 minutes), and the assistant by user (20 per minute, one request in flight). An unknown e-mail takes as long as a wrong password.
- JSON bodies are capped at 1 MB and reject unknown fields. Statement uploads have their own cap.
- `ReadHeaderTimeout` against slow clients.

**Authorization**

- Every wallet route goes through `requireWallet`, which checks membership and a minimum role (reader, editor, owner) before the handler runs.
- Sign-up closes after the first user unless there is an invite code. Codes are single-use and stored hashed.

**Data**

- SQLite on the server, with daily local backups and an encrypted off-site copy (see [ops/backup.md](ops/backup.md)).
- Schema changes are in-place migrations, with a copy of the database saved before the first one runs.

**Supply chain** (OWASP Top 10 2025, A03)

- Every GitHub Action is pinned to a commit SHA. Workflows default to `contents: read`. Checkouts do not keep the token.
- zizmor audits the workflows, CodeQL scans Go and TypeScript, golangci-lint runs gosec, `govulncheck` and `npm audit` run on every push.
- npm install scripts are off, and no dependency version younger than 7 days is taken (`web/.npmrc`, Dependabot cooldown). Dependabot security updates are on.
- The release binary is built without a dependency cache.
- Deploys need the `production` environment, and only master deploys.
- GitHub secret scanning and push protection are on.

**Development machine**

- `.claude/settings.json` denies coding agents read access to `.env` files, keys, `data/` and database copies, and denies `sudo`.

## Known gaps

Found by gosec when golangci-lint was added (#70). None is known to be exploitable. Each needs its own issue before a fix.

- SQL built by concatenation (G202) in `assistant_tools.go`, `chat.go`, `transactions.go` and `db.go`. The concatenated parts are fixed fragments chosen in code, not user input, but each should be checked again.
- 178 unchecked errors (errcheck), among them a `DELETE FROM budgets` in `deleteCategory`.
- Go 1.25 added `http.CrossOriginProtection`. It does what the hand-written `Sec-Fetch-Site` check does, and could replace it.
- Reported but likely false positives: G120 in `bank.go` (`MaxBytesReader` is already there), G124 in `auth.go` (cookie attributes are set), G101 in `ai_settings.go`, G115 in `money.go`.

## When to update this file

When a change adds a new kind of data, a new external service, a new way in (route, upload, webhook), or a new secret.
