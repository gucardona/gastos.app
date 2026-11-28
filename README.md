**English** · [Português](README.pt-BR.md)

# gastos.app

Personal and shared money tracking. One Go binary serves the API and the web app, and the data lives in a single SQLite file.

## What it does

- Wallets with members and roles (owner, editor, reader)
- Expenses and income, with categories and payment methods you can edit
- Splitting expenses between members (in basis points) and recording settlements
- Monthly recurring expenses, income and settlements, split the same way
- Credit cards: a purchase lands on the right bill from the card's closing and due days
- A monthly budget per category and a summary of the month
- Bank statement import (OFX and CSV) into an inbox, with categorization rules. See [Importing a bank statement](docs/bank-import.md)
- An assistant that answers questions about your own transactions in plain language, with your Anthropic API key. See [Query assistant](docs/assistant.md)
- Live updates across tabs, devices and members, without reloading (SSE on `/api/events`)

### Default split per person

In Ajustes > Pessoas (Settings > People), the wallet owner can set a "Parte padrão (%)" (default share) for each member who splits by default, for example 60% and 40%. When everyone who splits by default has a share, a new expense is split by those shares, scaled to add up to 100%. If anyone is left blank, the split is equal. On a transaction (and on each recurrence), "Ajustar" (Adjust) picks other shares for that expense only, such as 50% for one person and 25% for each of the other two.

## Stack

- Go (1.26 or later), `net/http` with the Go 1.22+ router, `log/slog`
- SQLite through `modernc.org/sqlite` (pure Go, no CGO)
- Sessions in an httpOnly cookie, passwords hashed with bcrypt
- Web app: React, Vite and TypeScript, in `web/`

## Layout

```text
cmd/gastos/           main: config, database, server and background jobs
internal/api/         HTTP wiring and middleware, with one package per area: wallet, ledger, bank, assistant, auth, and httpx for the shared plumbing
internal/anthropic/   Anthropic Messages API client for the assistant
internal/bankimport/  OFX and CSV statement parsing and categorization rules
internal/billing/     credit card bill dates
internal/config/      environment variables
internal/db/          schema.sql, migrations, opening and backing up the database
internal/events/      event bus behind SSE
internal/money/       splitting amounts in cents
internal/recurrence/  monthly occurrences and the materializer
internal/secret/      encryption of stored API keys
web/                  web app (web/dist is embedded in the binary)
docs/                 design docs, plans, operations and decisions
```

## Running

```bash
GASTOS_COOKIE_SECURE=false go run ./cmd/gastos
```

In PowerShell:

```powershell
$env:GASTOS_COOKIE_SECURE = 'false'; go run ./cmd/gastos
```

The server listens on `:8000`. Check it with `curl localhost:8000/healthz`.

## Configuration

| Variable | Default | |
|---|---|---|
| `GASTOS_ADDR` | `:8000` | listen address |
| `GASTOS_DB_PATH` | `./data/gastos.db` | SQLite file |
| `GASTOS_BACKUP_DIR` | `./data/backups` | daily backups |
| `GASTOS_BACKUP_KEEP` | `14` | how many backups to keep |
| `GASTOS_TZ` | `America/Sao_Paulo` | time zone that decides "today" for recurrences |
| `GASTOS_COOKIE_SECURE` | `true` | `false` only for local development over http |
| `GASTOS_DEV` | `false` | allows CORS from `localhost:5173` |
| `GASTOS_INVITE_CODE` | empty | a fixed sign-up code accepted after the first account (at least 8 characters). Optional: the usual way is single-use codes from `gastos invite [duration]` (on the server, `sudo gastos-invite`, after installing the script as root: see `docs/ops/vdi-setup.md`). With neither, sign-up closes after the first user |
| `GASTOS_SECRET_KEY` | empty | master key (32 bytes, base64) that encrypts the stored Anthropic keys. Without it the assistant is unavailable. See [Query assistant](docs/assistant.md) |
| `GASTOS_ANTHROPIC_URL` | `https://api.anthropic.com` | tests only (e2e): points the assistant at a fake server |
| `GASTOS_FX_URL` | `https://economia.awesomeapi.com.br/last/USD-BRL` | USD to BRL rate (AwesomeAPI) used to estimate the assistant's cost in reais; changed only in tests |

## Tests

```bash
# backend
go vet ./...
go test -race ./...

# web app (inside web/)
npm run lint        # Biome
npm run typecheck
npm test            # unit and component tests
npm run build && npm run e2e   # browser against the real Go binary, desktop and phone
```

CI runs all of this, plus `gofmt`, `golangci-lint`, `govulncheck`, `npm audit`, zizmor and CodeQL.

## Database

`internal/db/schema.sql` is the frozen base, at version 4 (`PRAGMA user_version`). Every change after it is a migration in `internal/db/migrations/` (`0005_...` takes the database from version 4 to 5, and so on), listed in `internal/db/db.go`. On start, the server builds a new database (version 0) from `schema.sql` plus every migration. A database between version 4 and the latest gets only the missing migrations, in place and without losing data, each in its own transaction. Before the first pending migration on a database with data, the server saves a copy next to it (`gastos.db.pre-v<N>.bak`, where N is the version before). An earlier copy with the same name is kept with a timestamp in its name, and if the copy fails the app does not start. Versions 1 to 3, or newer than the binary knows, make the app refuse to start without touching the database. To change the schema, add the next migration. Never edit `schema.sql` or a migration that has shipped.

## API

Every route is under `/api`. The user and the active wallet come from the session cookie and the path (`/api/wallets/{walletID}/...`). The full reference is in the [backend design doc](docs/superpowers/specs/2026-10-09-backend-foundation-design.md).

## Contributing

Every change starts from an issue and ends in a pull request that links it (`Closes #N`). Opening an issue shows forms for bugs, features and tasks. See [AGENTS.md](AGENTS.md) for the conventions, [docs/decisions.md](docs/decisions.md) for why things are the way they are, and [docs/security.md](docs/security.md) for the threat model.

## License

Apache-2.0.
