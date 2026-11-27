# Backend foundation: new schema and API v2

**Date:** 2026-10-09
**Status:** Draft, awaiting review
**Roadmap:** [2026-10-09-roadmap.md](2026-10-09-roadmap.md)

## Goal

Rewrite the Go backend foundation, taking advantage of the fact that there is no data to preserve:

1. A single, correct schema, with no migration history.
2. Money in cents and splits in basis points.
3. API organized by wallet (`/api/wallets/{walletID}/...`) with the Go 1.22+ router.
4. Fix the bugs found in the review (per-connection FK, recurrence race, PATCH reassigning the owner, backup, SSE).
5. Move dashboard calculations to the server (`/summary`), keeping the new frontend thin.

Out of scope: frontend (phase 3), CI/deploy (phase 2), Open Finance (phase 4). The phase 4 tables come later, via `user_version` 2.

## Folder structure

```
cmd/gastos/main.go          # reads config, opens DB, starts server, background jobs
internal/config/            # env vars → Config struct (fails early if invalid)
internal/db/                # Open (pragmas), embedded schema.sql, migrate (user_version), backup
internal/money/             # Allocate (apportioning in cents), basis point helpers
internal/recurrence/        # Occurrences (pure) + Materializer (job)
internal/auth/              # password hash, sessions, RequireUser middleware
internal/api/               # handlers per resource, RequireWallet middleware, JSON/error helpers
internal/events/            # SSE bus (the current one, with heartbeat)
web/                        # React app (phase 3); web/embed.go exposes dist/ via go:embed
```

Removed: all of `src/`, `vendor/` (CI downloads modules), the committed `gastos-app` binary, `deploy.sh` (replaced in phase 2).

## Configuration (env)

| Var | Default | Note |
|---|---|---|
| `GASTOS_ADDR` | `:8000` | |
| `GASTOS_DB_PATH` | `./data/gastos.db` | directory is created if it does not exist |
| `GASTOS_BACKUP_DIR` | `./data/backups` | |
| `GASTOS_BACKUP_KEEP` | `14` | how many daily backups to keep |
| `GASTOS_TZ` | `America/Sao_Paulo` | defines "today" for recurrences and for the current month |
| `GASTOS_COOKIE_SECURE` | `true` | `false` only in local dev (http) |
| `GASTOS_DEV` | `false` | enables CORS for `localhost:5173` and the demo seed |

No `JWT_SECRET`: the session becomes an opaque token stored hashed in the database.

## Database

### Connection

`db.Open(path)` uses the DSN with pragmas applied **on every connection**:

```
file:<path>?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)
```

`db.SetMaxOpenConns(1)` for writes is not needed with WAL + `busy_timeout`, but write transactions use `BEGIN IMMEDIATE` (via `_txlock=immediate` in the DSN) to avoid `SQLITE_BUSY` on lock upgrade.

### Versioning

`schema.sql` creates everything (version 1). `migrate.go` reads `PRAGMA user_version`:

- `0` → runs `schema.sql` in a transaction, sets `user_version = 1`.
- `N` → applies `migrations[N:]`, a slice of SQL strings, each in its own transaction, incrementing `user_version`.

Today the slice is empty. It is about 30 lines, with no control table and no "ignore duplicate column".

### Schema (version 1)

Conventions: `id INTEGER PRIMARY KEY`, calendar dates as `TEXT 'YYYY-MM-DD'`, timestamps as ISO-8601 UTC `TEXT`, booleans as `INTEGER CHECK (x IN (0,1))`, money as `*_cents INTEGER`.

```sql
CREATE TABLE users (
  id            INTEGER PRIMARY KEY,
  name          TEXT NOT NULL,
  email         TEXT NOT NULL UNIQUE COLLATE NOCASE,
  password_hash TEXT NOT NULL,
  created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE sessions (
  token_hash BLOB PRIMARY KEY,                       -- sha256 of the token sent in the cookie
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
) WITHOUT ROWID;
CREATE INDEX sessions_user ON sessions(user_id);

CREATE TABLE wallets (
  id                INTEGER PRIMARY KEY,
  name              TEXT NOT NULL,
  splitting_enabled INTEGER NOT NULL DEFAULT 0 CHECK (splitting_enabled IN (0,1)),
  created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE wallet_members (
  wallet_id         INTEGER NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
  user_id           INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role              TEXT NOT NULL CHECK (role IN ('owner','editor','reader')),
  splits_by_default INTEGER NOT NULL DEFAULT 1 CHECK (splits_by_default IN (0,1)),
  created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  PRIMARY KEY (wallet_id, user_id)
) WITHOUT ROWID;
CREATE INDEX wallet_members_user ON wallet_members(user_id);
CREATE UNIQUE INDEX wallet_members_one_owner ON wallet_members(wallet_id) WHERE role = 'owner';

CREATE TABLE categories (
  id           INTEGER PRIMARY KEY,
  wallet_id    INTEGER NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
  kind         TEXT NOT NULL CHECK (kind IN ('expense','income')),
  name         TEXT NOT NULL,
  icon         TEXT NOT NULL,
  color        TEXT NOT NULL,
  essentiality TEXT CHECK (essentiality IN ('essential','nonessential','investment')),
  sort_order   INTEGER NOT NULL DEFAULT 0,
  archived_at  TEXT,
  UNIQUE (wallet_id, kind, name),
  CHECK ((kind = 'expense') = (essentiality IS NOT NULL))
);

CREATE TABLE payment_methods (
  id          INTEGER PRIMARY KEY,
  wallet_id   INTEGER NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
  name        TEXT NOT NULL,
  icon        TEXT NOT NULL,
  sort_order  INTEGER NOT NULL DEFAULT 0,
  archived_at TEXT,
  UNIQUE (wallet_id, name)
);

CREATE TABLE recurrences (
  id                INTEGER PRIMARY KEY,
  wallet_id         INTEGER NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
  kind              TEXT NOT NULL CHECK (kind IN ('expense','income','settlement')),
  amount_cents      INTEGER NOT NULL CHECK (amount_cents > 0),
  description       TEXT NOT NULL DEFAULT '',
  category_id       INTEGER REFERENCES categories(id),
  payment_method_id INTEGER REFERENCES payment_methods(id),
  paid_by_user_id   INTEGER NOT NULL REFERENCES users(id),   -- expense: who pays; income: who receives; settlement: who sends
  to_user_id        INTEGER REFERENCES users(id),            -- settlement only
  day_of_month      INTEGER NOT NULL CHECK (day_of_month BETWEEN 1 AND 31),
  start_date        TEXT NOT NULL,
  end_date          TEXT,
  enabled           INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0,1)),
  created_by        INTEGER NOT NULL REFERENCES users(id),
  created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  CHECK ((kind = 'settlement') = (to_user_id IS NOT NULL)),
  CHECK ((kind = 'settlement') = (category_id IS NULL)),
  CHECK (end_date IS NULL OR end_date >= start_date)
);
CREATE INDEX recurrences_wallet ON recurrences(wallet_id, enabled);

CREATE TABLE recurrence_splits (
  recurrence_id INTEGER NOT NULL REFERENCES recurrences(id) ON DELETE CASCADE,
  user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  share_bp      INTEGER NOT NULL CHECK (share_bp BETWEEN 1 AND 10000),
  PRIMARY KEY (recurrence_id, user_id)
) WITHOUT ROWID;

CREATE TABLE transactions (
  id                INTEGER PRIMARY KEY,
  wallet_id         INTEGER NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
  kind              TEXT NOT NULL CHECK (kind IN ('expense','income')),
  amount_cents      INTEGER NOT NULL CHECK (amount_cents > 0),
  description       TEXT NOT NULL,
  category_id       INTEGER NOT NULL REFERENCES categories(id),
  payment_method_id INTEGER REFERENCES payment_methods(id),
  date              TEXT NOT NULL,
  paid_by_user_id   INTEGER NOT NULL REFERENCES users(id),
  created_by        INTEGER NOT NULL REFERENCES users(id),
  recurrence_id     INTEGER REFERENCES recurrences(id) ON DELETE SET NULL,
  occurrence        TEXT,                                    -- 'YYYY-MM' when generated by a recurrence
  created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  updated_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);
CREATE INDEX transactions_wallet_date ON transactions(wallet_id, date DESC, id DESC);
CREATE UNIQUE INDEX transactions_recurrence_occurrence
  ON transactions(recurrence_id, occurrence) WHERE recurrence_id IS NOT NULL;

CREATE TABLE transaction_splits (
  transaction_id INTEGER NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
  user_id        INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  share_bp       INTEGER NOT NULL CHECK (share_bp BETWEEN 1 AND 10000),
  PRIMARY KEY (transaction_id, user_id)
) WITHOUT ROWID;
CREATE INDEX transaction_splits_user ON transaction_splits(user_id);

CREATE TABLE settlements (
  id            INTEGER PRIMARY KEY,
  wallet_id     INTEGER NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
  from_user_id  INTEGER NOT NULL REFERENCES users(id),
  to_user_id    INTEGER NOT NULL REFERENCES users(id),
  amount_cents  INTEGER NOT NULL CHECK (amount_cents > 0),
  date          TEXT NOT NULL,
  note          TEXT NOT NULL DEFAULT '',
  recurrence_id INTEGER REFERENCES recurrences(id) ON DELETE SET NULL,
  occurrence    TEXT,
  created_by    INTEGER NOT NULL REFERENCES users(id),
  created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  CHECK (from_user_id <> to_user_id)
);
CREATE INDEX settlements_wallet_date ON settlements(wallet_id, date DESC);
CREATE UNIQUE INDEX settlements_recurrence_occurrence
  ON settlements(recurrence_id, occurrence) WHERE recurrence_id IS NOT NULL;

CREATE TABLE budgets (
  wallet_id   INTEGER NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
  category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
  limit_cents INTEGER NOT NULL CHECK (limit_cents > 0),
  PRIMARY KEY (wallet_id, category_id)
) WITHOUT ROWID;
```

**What changed from the current schema:**

| Before | Now | Why |
|---|---|---|
| `expenses` + `incomes` | `transactions.kind` | one list, one form, one model for bank import |
| `incomes.type` (free text) | `categories` with `kind='income'` | income gets editable categories like expenses |
| `category`/`payment` as text key | `category_id`/`payment_method_id` FK | integrity; renaming breaks nothing |
| deleting a category with "replacement" | `archived_at` (soft delete) when in use | disappears from the form, history intact |
| `recurring_expenses`/`_incomes`/`_payments` | `recurrences.kind` | one table, one materializer |
| `percentage REAL` | `share_bp INTEGER` | sum is exactly 10000 |
| ambiguous `user_id` (owner? payer?) | editable `paid_by_user_id` + immutable `created_by` | fixes the bug of PATCH reassigning the owner |
| `account_split_participants` | `wallet_members.splits_by_default` | 1 column instead of 1 table |
| `goals` | `budgets` | name reflects the use (monthly limit per category) |
| `schema_migrations` | `PRAGMA user_version` | |

### Per-wallet seed

When a wallet is created, `seedWallet(tx, walletID)` inserts the 15 current expense categories, 5 income categories (Salário (Salary), Freelance, Investimentos (Investments), Reembolso (Reimbursement), Outros (Other)) and the 8 current payment methods. The data lives in a Go table, as it does today.

### Backup

Daily job (and at boot, if the last backup is more than 24h old): `VACUUM INTO '<dir>/gastos-YYYYMMDD-HHMMSS.db'`, then removes the oldest ones beyond `GASTOS_BACKUP_KEEP`. It never overwrites an existing backup. `VACUUM INTO` produces a consistent copy even with WAL.

## Money and splits

`money.Allocate(totalCents int64, sharesBP []int64) []int64`: distributes cents using the largest remainder method. The sum of the result always equals `totalCents`, and ties go to the lowest index, so the result is deterministic.

Split rules (only when `wallets.splitting_enabled = 1`):
- No `splits` in the request: split equally among members with `splits_by_default = 1`, in bp, with the remainder going to the first one.
- With `splits`: each `userId` must be a member, with no repeats, each `shareBp` between 1 and 10000, and the sum exactly 10000.
- With `splitting_enabled = 0`: `splits` must be empty; no rows in `transaction_splits`.

Balances between members, computed in `/summary`, over the wallet's entire history:
- For each expense with splits: `paid_by` is credited `amount`; each participant is debited `Allocate(amount, shares)[i]`.
- `settlement` from A to B: A is credited, B is debited.
- Suggested settlements: a greedy algorithm that matches the largest debtor with the largest creditor (the same result the current UI shows).

## Recurrences

`recurrence.Occurrences(dayOfMonth int, start, end, until string) []Occurrence` is a pure function, with `YYYY-MM-DD` dates (empty `end` = no end). It returns `{Month "YYYY-MM", Date "YYYY-MM-DD"}` for each month between `start` and `until`. The day is clamped to the last day of the month, and months whose date falls outside `[start, end]` are left out.

`Materializer.Run(ctx, walletID *int64)`:
- Reads `enabled = 1` recurrences (from all wallets or from one).
- For each occurrence up to **today in `GASTOS_TZ`**, does `INSERT ... ON CONFLICT DO NOTHING` into `transactions` or `settlements`, together with the splits, in one transaction per occurrence.
- The unique index `(recurrence_id, occurrence)` guarantees idempotency, even with concurrent runs.
- If it inserted anything, it calls `events.Notify(walletID, 0)`.

When it runs:
- at boot;
- every hour (`time.Ticker`);
- synchronously after a recurrence is created or edited, only for that wallet, so the UI shows the transaction right away.

**GETs never write.**

Editing a recurrence does not change transactions already generated. Deleting a recurrence keeps the transactions and clears `recurrence_id`, through `SET NULL`.

## Authentication

- `POST /api/auth/register`, `POST /api/auth/login`: generate a random 32-byte token, store its `sha256` in `sessions` with a 30-day expiry, and send the `gastos_session` cookie (`HttpOnly`, `SameSite=Lax`, `Secure` per config, `Path=/`).
- `POST /api/auth/logout`: deletes the session and expires the cookie.
- `GET /api/me`: returns `{ user, wallets: [...] }`, which is the frontend bootstrap.
- Sliding renewal: if fewer than 15 days remain, extends `expires_at` and resends the cookie.
- Expired sessions are removed by the hourly job.
- CSRF: mutations require `Content-Type: application/json`. Cross-site HTML forms cannot send that header without a preflight, and the server does not answer cross-origin preflights outside `GASTOS_DEV`. That, plus `SameSite=Lax`, covers the case.
- Password with bcrypt cost 12, minimum 8 characters.
- In-memory rate limit on login: 10 attempts per 15 minutes, per IP and per email. Responds `429`.

## API v2

JSON base, errors in this format:

```json
{ "error": { "code": "validation", "message": "Dados inválidos", "fields": { "amountCents": "deve ser maior que zero" } } }
```

Codes: `validation` (400), `unauthenticated` (401), `forbidden` (403), `not_found` (404), `conflict` (409), `rate_limited` (429), `internal` (500).

Middleware chain: `logRequest → recoverPanic → RequireUser → RequireWallet(minRole)`.
`RequireWallet` reads `r.PathValue("walletID")` and puts `{walletID, role}` in the context. The roles are `reader < editor < owner`.

| Method and route | Minimum role | Note |
|---|---|---|
| `POST /api/auth/register` | — | creates the user and a "Pessoal" (Personal) wallet |
| `POST /api/auth/login` · `POST /api/auth/logout` | — / user | |
| `GET /api/me` | user | |
| `GET /api/wallets` · `POST /api/wallets` | user | |
| `PATCH /api/wallets/{walletID}` · `DELETE` | owner | |
| `GET /api/wallets/{walletID}/members` | reader | |
| `POST .../members` · `PATCH .../members/{userID}` | owner | invite by email of an existing user |
| `DELETE .../members/{userID}` | owner, or the user themselves | leaving the wallet |
| `GET/POST .../categories` · `PUT/DELETE .../categories/{id}` | reader / editor | DELETE: archives if in use, deletes if not |
| `GET/POST .../payment-methods` · `PUT/DELETE .../payment-methods/{id}` | reader / editor | same |
| `GET .../transactions?from=&to=&kind=&categoryId=&q=` | reader | `from`/`to` required (max 366 days) |
| `POST .../transactions` · `PUT/DELETE .../transactions/{id}` | editor | |
| `GET/POST .../recurrences` · `PUT/DELETE .../recurrences/{id}` | reader / editor | POST/PUT materialize immediately |
| `GET .../settlements?from=&to=` · `POST` · `PUT/DELETE .../settlements/{id}` | reader / editor | |
| `GET .../budgets` · `PUT .../budgets/{categoryId}` · `DELETE` | reader / editor | PUT does an upsert |
| `GET .../summary?month=YYYY-MM` | reader | see below |
| `GET .../events` | reader | SSE |
| `GET /healthz` | — | `200 ok` if `db.Ping()` passes |
| `GET /*` | — | embedded SPA; unknown routes return `index.html` |

Transaction format, the same on input and output:

```json
{
  "id": 42, "kind": "expense", "amountCents": 12990, "description": "Mercado",
  "categoryId": 3, "paymentMethodId": 1, "date": "2026-10-09",
  "paidByUserId": 1, "createdBy": 1,
  "recurrenceId": null,
  "splits": [{ "userId": 1, "shareBp": 5000, "amountCents": 6495 }, { "userId": 2, "shareBp": 5000, "amountCents": 6495 }]
}
```

`PUT` replaces all editable fields; the form always sends the whole object, which avoids the ambiguity of "missing field" versus `null` in a partial PATCH. `PATCH` is kept only for wallet (`name`, `splittingEnabled`) and member (`role`, `splitsByDefault`). `createdBy`, `recurrenceId` and `id` are read-only. An omitted or `0` `paidByUserId` means the current user. An archived category is accepted in `PUT` only if it was already the transaction's category.

The transactions GET loads the splits with **one** query (`WHERE transaction_id IN (...)`), with no N+1.

### `/summary`

```json
{
  "month": "2026-10",
  "incomeCents": 850000, "expenseCents": 412300, "balanceCents": 437700,
  "dailyAverageCents": 45811,
  "byCategory": [{ "categoryId": 3, "kind": "expense", "cents": 98000, "budgetCents": 120000 }],
  "byEssentiality": { "essential": 300000, "nonessential": 90000, "investment": 22300 },
  "previous": { "incomeCents": 850000, "expenseCents": 398000 },
  "memberBalances": [{ "userId": 1, "netCents": 15000 }, { "userId": 2, "netCents": -15000 }],
  "suggestedSettlements": [{ "fromUserId": 2, "toUserId": 1, "amountCents": 15000 }]
}
```

`dailyAverageCents` uses the elapsed days of the month if it is the current month in `GASTOS_TZ`, or the days in the month if it is a past month.
`memberBalances` and `suggestedSettlements` appear only with `splitting_enabled`.

### Events (SSE)

- Authenticates through the cookie and `RequireWallet`, with no token in the URL.
- Sends `: ping\n\n` every 25 seconds.
- Every successful mutation on a wallet resource calls `events.Notify(walletID, actorUserID)`, including categories, payment methods, budgets, members and wallet deletion. The notification is done by a `mutated(r)` helper called at the end of the handlers, not by hand in each one.
- Payload: `data: {"type":"changed"}`. The frontend invalidates the wallet's queries.

## Observability

`log/slog` in JSON. Each request produces a log with method, route (the pattern, not the raw path), status, duration and `userID`. Panics are logged with a stack trace and respond `500`.

## Tests

- **Pure unit tests:** `money.Allocate`, `recurrence.Occurrences`, settlement suggestion, validations.
- **Integration** (`httptest` + SQLite in `t.TempDir()`), one file per resource in `internal/api`, covering:
  - auth flow with cookie;
  - permissions by role on each route;
  - CRUD of each resource;
  - split rules;
  - materializer idempotency, including two concurrent runs;
  - `ON DELETE CASCADE` (deleting a wallet deletes its transactions);
  - archiving a category in use;
  - `/summary` numbers with a fixed scenario.
- The current tests in `src/` are discarded along with the old code.

## Acceptance criteria

- `go test ./...` passes; `go vet ./...` is clean.
- A new database comes up from `schema.sql` alone, with `user_version = 1`.
- Two simultaneous materializer calls do not duplicate transactions (test with `t.Parallel` and goroutines).
- Deleting a wallet removes all dependent rows (the test counts the rows).
- No `float64` for money values in the `api` package (checked with `grep` in the plan).
- `curl /healthz` responds 200; a GET of an unknown SPA route returns the embedded `index.html`.
