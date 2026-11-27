# Query assistant

A chat inside the app for asking about your own transactions ("quanto gastei este mês?" (how much did I spend this month?), "quais foram meus maiores gastos?" (what were my biggest expenses?)). The assistant only **queries**: it does not create, change or delete anything. It uses the Anthropic API (Claude) with each person's own key, and can answer with small charts.

## How it works

- Each person enters their **own** Anthropic API key in **Ajustes > Assistente** (Settings > Assistant), picks the model (Haiku, Sonnet or Opus) and turns on **Ativar chat** (Enable chat). The **Chat** tab only appears after that.
- In the Chat, the selector at the top defines which wallets are queried (by default, the current wallet; "Selecionar todas" (Select all) includes the others). Only wallets you are a member of are included.
- The conversation is kept for a few days (default 7, from 1 to 30, in Ajustes > Assistente). **Nova conversa** (New conversation) deletes the history of that wallet.

## Server configuration

The server stores the keys encrypted (AES-256-GCM) with a **master key** taken from the `GASTOS_SECRET_KEY` variable: 32 bytes in base64.

```bash
openssl rand -base64 32
```

Put the result in `/etc/gastos/production.env` (the same file the service uses on the VPS) and restart the service:

```
GASTOS_SECRET_KEY=<output of the command above>
```

- The value stays on the server only: never in the repository, in the chat or in a log.
- Without the variable, the server starts normally and the assistant shows as unavailable. With the variable present but malformed, the server **does not start** (configuration error).
- Keep a copy of the master key in a safe place (password manager).
- If the master key **changes** (or is lost), the stored keys can no longer be read: the chat asks for the key again and Ajustes shows the key as not configured. Nothing else is lost; each person only needs to paste their own key again.
- `GASTOS_ANTHROPIC_URL` (default `https://api.anthropic.com`) exists only for automated tests; do not set it in production.
- `GASTOS_FX_URL` (default `https://economia.awesomeapi.com.br/last/USD-BRL`) is the USD-BRL exchange rate fetched once a day to estimate spending in reais. It exists for tests; do not set it in production.

## Privacy

When you ask a question, the server sends Anthropic your question, the recent conversation and the results of the queries made (for example, descriptions, categories and amounts of the transactions in the selected wallets). This happens with your key and is subject to Anthropic's terms. The key never goes back to the browser (only the last 4 characters) and does not appear in logs. If you do not want a given wallet to be queried, deselect it in the selector.

## Costs

Usage is billed to the Anthropic account that owns each key, not by the app. Larger models (Opus) cost more than smaller ones (Haiku). The model and the conversation retention are set in Ajustes > Assistente. There is a limit of messages per minute per person.

## Spending limit

In Ajustes > Assistente there are two fields, **Limite diário (R$)** (Daily limit) and **Limite mensal (R$)** (Monthly limit). Enter the amount in reais (for example `5`, `5,50` or `5.50`, from R$ 0.01 to R$ 100,000.00) and tap **Salvar** (Save); leave the field empty (or `0`) to remove the limit. Below the fields you see today's and this month's spending, and the exchange rate used.

- **It is an estimate.** The server computes the cost from the tokens of each response, the model's price table and the dollar exchange rate (AwesomeAPI, fetched once a day), with a 5% margin upward. The value can differ a little from what Anthropic charges, and a response can exceed the limit by a fraction, because the spending is only known after the response. That is why the fields have an asterisk.
- **The real cap is Anthropic's.** Also set an account spending limit in the Anthropic console.
- **When the limit is reached**, the next question is not sent to Anthropic: the chat shows "Limite diário (ou mensal) do assistente atingido" (Assistant daily (or monthly) limit reached) and works again the next day (or month), or when you raise the limit. The day and the month follow the time zone set in `GASTOS_TZ` (default `America/Sao_Paulo`).
- **Canceled or failed responses** count only the input tokens, because the output tokens only arrive at the end of the response; the estimate can be slightly low in these cases.
- **Without an exchange rate** (for example, AwesomeAPI has been down since the server started), the limits do not apply until the rate is obtained; the screen warns about it.
- **Network:** the server makes one request per day to `economia.awesomeapi.com.br` to get the exchange rate. `GASTOS_FX_URL` exists only for tests.
- **Database:** `internal/db/schema.sql` is the baseline (version 4); changes after it are migrations applied in place when the server starts, with a backup copy first (see "Banco de dados" (Database) in the README).

## Schema changes after version 4

The deploy is the normal one (merge into `master`): on start, the server keeps a copy next to the database (`gastos.db.pre-v<N>.bak`, where N is the version before the migration; an older copy with the same name is kept with a date stamp in the name) and applies the missing migrations, without recreating the database or losing data. The migrations only add things, but the previous version of the program refuses a database marked with a version it does not know. To roll back, with the service stopped (`systemctl stop gastos@production`), undo the migrations from newest to oldest and lower the mark, down to the version the previous program knows. From version 9 to 8 (the program before the default payment method): `sqlite3 /var/lib/gastos/production/gastos.db 'ALTER TABLE wallet_members DROP COLUMN default_payment_method_id; PRAGMA user_version=8;'` (each person goes back to starting with the first payment method). From version 8 to 7 (the program before credit cards): `sqlite3 /var/lib/gastos/production/gastos.db 'DROP INDEX transactions_wallet_month; ALTER TABLE transactions DROP COLUMN month; ALTER TABLE transactions DROP COLUMN invoice_due; ALTER TABLE payment_methods DROP COLUMN closing_day; ALTER TABLE payment_methods DROP COLUMN due_day; ALTER TABLE payment_methods DROP COLUMN issuer; ALTER TABLE payment_methods DROP COLUMN card_form; ALTER TABLE payment_methods DROP COLUMN last4; PRAGMA user_version=7;'` (card purchases go back to the month of their own date; the card data is lost). From version 7 to 6 (the program before single-use invite codes): `sqlite3 /var/lib/gastos/production/gastos.db 'DROP TABLE invite_codes; PRAGMA user_version=6;'`. From 6 to 5 (the program before remembering deleted occurrences): `sqlite3 /var/lib/gastos/production/gastos.db 'DROP TRIGGER transactions_remember_deleted_occurrence; DROP TRIGGER settlements_remember_deleted_occurrence; DROP TABLE recurrence_skips; PRAGMA user_version=5;'`. From 5 to 4: `sqlite3 /var/lib/gastos/production/gastos.db 'ALTER TABLE wallet_members DROP COLUMN default_share_bp; PRAGMA user_version=4;'`. Then start the previous version. Lowering the mark alone is not enough: what the migration created would stay in the database and the next start of it would fail (for example, "duplicate column"). `sqlite3` must be installed on the server (`sudo apt install sqlite3`). The alternative is to restore the `.pre-v<N>.bak` copy in place of the database (deleting the `-wal` and `-shm` files first), but that **loses everything written after the migration**.

## Deploy runbook (version with the assistant, historical)

This runbook applied only to the version that introduced the schema at version 4 (the baseline of `internal/db/schema.sql`) and created a **new** database. The database of that time (version 3) **was not migrated**: it was moved out of the way and each person recreated their own account.

**Steps 1 to 4 happen BEFORE merging the PR**, because the merge into `master` is already the production deploy. If the merge comes first, the new version refuses to open the old database ("database is at schema version 3, this binary expects 4"), `/healthz` fails and the deploy rolls back to the previous version on its own. Nothing is lost, but redo it in the right order.

1. Generate the master key on the VPS (`openssl rand -base64 32`) and put `GASTOS_SECRET_KEY` in `/etc/gastos/production.env`.
2. **Back up** the current database (see [docs/ops/backup.md](ops/backup.md)): download **a verified copy** to your computer (`scp`) and check that it opens. The new app rotates the `GASTOS_BACKUP_DIR` folder (keeps 14) and the off-site script forces a push of only the 14 newest, so within about 14 days every copy of the old data would be gone, except the one on the same disk. Also take the old backups out of that folder (for example to `/var/lib/gastos/antigo/`).
3. **Stop** the service: `systemctl stop gastos@production`.
4. **Move** the old database out of the way: `mkdir -p /var/lib/gastos/antigo && mv /var/lib/gastos/production/gastos.db* /var/lib/gastos/antigo/` (the wildcard takes the `-wal` and `-shm` files along, if they exist).
5. **Merge** the PR (normal deploy through `master`) and start the service: the app creates the new, empty database.
6. Each person **recreates their account** (the first one joins without a code; the others with `GASTOS_INVITE_CODE`) and then enters their own key in Ajustes > Assistente.
7. When everyone has an account, remove `GASTOS_INVITE_CODE` and restart, as in the VDI setup.
