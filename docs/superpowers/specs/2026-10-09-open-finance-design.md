# Bank import and Open Finance

**Date:** 2026-10-09
**Status:** Draft. Stage 4b depends on a spike and on open questions 1 and 3 of the roadmap.
**Roadmap:** [2026-10-09-roadmap.md](2026-10-09-roadmap.md)

## Goal

Stop typing transactions that the bank already knows. Bank transactions arrive in an **inbox**. There they get a suggested category, are deduplicated against what was already entered by hand, and become `transactions` with one tap. Nothing enters the history without confirmation, except through an explicit auto-accept rule.

## Open Finance Brasil constraints

- Only institutions authorized by the Central Bank access Open Finance directly. A personal app needs an aggregator.
- Commercial aggregators charge amounts that are unviable for personal use. Public reports mention about R$ 2.5k/month (Pluggy) and R$ 6k/month (Belvo).
- There is **Meu Pluggy**, free for personal use. You connect your banks in Meu Pluggy and authorize your application, which reads the data through the Pluggy API. This comes from user reports, not official documentation, and needs a spike (see 4b).

So the feature is split into two stages. **4a** delivers value without depending on anyone. **4b** plugs an automatic source into the same funnel.

## Data model (`user_version` 2)

```sql
CREATE TABLE bank_connections (
  id             INTEGER PRIMARY KEY,
  user_id        INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider       TEXT NOT NULL CHECK (provider IN ('file','pluggy')),
  external_id    TEXT,                       -- itemId in Pluggy; NULL for 'file'
  institution    TEXT NOT NULL,              -- "Nubank", "Itaú"...
  status         TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','needs_reauth','error','disabled')),
  last_synced_at TEXT,
  last_error     TEXT,
  created_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  UNIQUE (provider, external_id)
);

CREATE TABLE bank_accounts (
  id                INTEGER PRIMARY KEY,
  connection_id     INTEGER NOT NULL REFERENCES bank_connections(id) ON DELETE CASCADE,
  external_id       TEXT NOT NULL,           -- account id at the provider or "branch/account" from the OFX
  name              TEXT NOT NULL,           -- "Conta corrente", "Cartão Ultravioleta"
  type              TEXT NOT NULL CHECK (type IN ('checking','savings','credit_card')),
  wallet_id         INTEGER REFERENCES wallets(id) ON DELETE SET NULL,    -- target wallet
  payment_method_id INTEGER REFERENCES payment_methods(id),               -- e.g. card → "Crédito"
  paid_by_user_id   INTEGER NOT NULL REFERENCES users(id),
  sync_from         TEXT NOT NULL,           -- nothing before this date is imported
  enabled           INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0,1)),
  UNIQUE (connection_id, external_id)
);

CREATE TABLE bank_transactions (
  id                INTEGER PRIMARY KEY,
  bank_account_id   INTEGER NOT NULL REFERENCES bank_accounts(id) ON DELETE CASCADE,
  external_id       TEXT NOT NULL,           -- provider id, OFX FITID or hash for CSV
  date              TEXT NOT NULL,
  amount_cents      INTEGER NOT NULL,        -- signed: negative = outflow
  description       TEXT NOT NULL,
  provider_category TEXT,
  installment       TEXT,                    -- "3/10" when the provider reports it
  raw               TEXT NOT NULL,           -- original JSON, to reprocess rules
  status            TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','accepted','matched','ignored')),
  transaction_id    INTEGER REFERENCES transactions(id) ON DELETE SET NULL,
  ignore_reason     TEXT CHECK (ignore_reason IN ('user','card_bill_payment','internal_transfer','duplicate')),
  suggested_category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL,
  imported_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  UNIQUE (bank_account_id, external_id)
);
CREATE INDEX bank_transactions_inbox ON bank_transactions(status, date DESC);

CREATE TABLE categorization_rules (
  id                INTEGER PRIMARY KEY,
  wallet_id         INTEGER NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
  pattern           TEXT NOT NULL,           -- case/accent-insensitive substring of the description
  category_id       INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
  rename_to         TEXT,                    -- "IFD*IFOOD 123" → "iFood"
  auto_accept       INTEGER NOT NULL DEFAULT 0 CHECK (auto_accept IN (0,1)),
  priority          INTEGER NOT NULL DEFAULT 0,
  created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);
```

`transactions` gets `source TEXT NOT NULL DEFAULT 'manual' CHECK (source IN ('manual','recurring','bank'))`.

A connection belongs to a **user** (it is their bank login). Each bank account points to a **wallet**: the personal card goes to "Pessoal" (Personal) and the joint account to "Casa" (Home). In the inbox, each member sees the transactions of the accounts mapped to the active wallet.

## Ingestion funnel (shared by 4a and 4b)

```
source (OFX/CSV | Pluggy) → normalize → UPSERT bank_transactions (unique external_id)
   → classify: 1) internal transfer? 2) card bill payment? 3) duplicate of a manual transaction?
               4) categorization rule → suggestion (+ auto-accept)
   → inbox
```

Classification rules, run in order:

1. **Credit card bill payment:** an outflow on the checking account with a description like "PAGTO FATURA", "PAGAMENTO CARTAO" or similar, when a connected `credit_card` account exists. It goes to `ignored/card_bill_payment`. The purchases already come in through the card.
2. **Internal transfer:** an outflow on one account and an inflow of the same amount on another connected account of the same user, up to 2 days apart. Both sides go to `ignored/internal_transfer`.
3. **Duplicate of a manual transaction:** a `transaction` exists in the target wallet with the same absolute amount, date ±2 days, `source='manual'` and no bank link. The bank transaction stays `status='pending'` with a link suggestion ("Já lançado: Mercado 09/10?" (Already entered: Mercado 09/10?)). Confirming turns it into `matched`, without creating a new transaction.
4. **Categorization rule:** the highest-priority rule whose `pattern` appears in the normalized description sets `suggested_category_id` (and `rename_to`). With `auto_accept`, it creates the `transaction` directly as `accepted`.
5. **No rule:** uses the `provider_category` mapped to a category of the wallet (mapping table in the code). If there is no mapping, it stays without a suggestion.

A negative amount becomes `kind='expense'`, a positive one becomes `kind='income'`. Refunds (positive amounts on the card) come in as `income` with a suggestion to link to the original expense, when the amount matches. Installments use the installment's date and amount, with "(3/10)" in the description.

## Inbox (UI, route `/caixa`)

- List of pending items grouped by day. Each row shows description, amount, source account and suggested category (editable inline).
- Actions:
  - **Aceitar** (Accept) (`A`);
  - **Ignorar** (Ignore) (`I`);
  - **Link to an existing transaction**, when there is a suggestion;
  - **Accept all with a suggestion**.
- When a row's category is changed, the UI offers **"Sempre categorizar 'UBER' como Transporte?"** (Always categorize 'UBER' as Transport?). This creates a `categorization_rule` and reapplies the rule to the other pending items.
- Badge with the number of pending items in the navigation.
- In Ajustes (Settings), the **Conexões bancárias** (Bank connections) section shows:
  - status;
  - last sync;
  - "sincronizar agora" (sync now);
  - account → wallet, payment method and payer mapping;
  - start date (`sync_from`).

## 4a: File import (no third parties)

- `POST /api/bank-accounts/{id}/import` (multipart) accepts:
  - **OFX:** the standard format of Brazilian banks, with FITID as `external_id`;
  - **CSV:** with known profiles (Nubank account, Nubank card, Inter) and `external_id = sha256(date|amount|description|nth occurrence in the file)`.
- The `file` bank account is created by hand in Ajustes.
- Reimporting the same file is idempotent, through `UNIQUE (bank_account_id, external_id)`.
- Own OFX parser (simple SGML/XML, about 150 lines) with anonymized fixture tests.

## 4b: Pluggy (Meu Pluggy)

**Spike before any code (1 day, timeboxed):**
- Create the application in the Pluggy dashboard and connect Meu Pluggy.
- Confirm that the transactions API works at no cost for the user's own accounts, and with what limits.
- Confirm the authorization flow (Pluggy Connect widget with the Meu Pluggy connector? manual `itemId`?).
- Confirm the webhook format and its authentication.
- Record everything in `docs/superpowers/specs/<date>-pluggy-spike.md`.
- If the spike fails, 4a remains the solution and 4b is archived.

**Integration (assuming the spike is ok):**
- `internal/bank/pluggy`: HTTP client with `client_id`/`client_secret` (env `PLUGGY_CLIENT_ID`/`PLUGGY_CLIENT_SECRET`). The short-lived API key is cached in memory.
- Endpoints used: items, accounts, transactions (paginated, by `from`).
- Connect: the frontend opens Pluggy Connect with a connect token generated by the backend. On `onSuccess`, it sends the `itemId` to `POST /api/bank-connections`. The backend fetches the accounts, and the user maps each one to a wallet.
- Sync:
  - job every 6 hours;
  - manual `POST /api/bank-connections/{id}/sync`;
  - `POST /api/webhooks/pluggy` (public route, with webhook secret validation) schedules the item's sync.
- Incremental fetch since `last_synced_at - 7 days`, because banks post late and change pending transactions. It upserts by `external_id`.
- A transaction whose amount changes after being accepted: flags the `transaction` with a warning. It does not change it automatically.
- `status='needs_reauth'` when the provider asks for new consent; the UI shows the warning. Open Finance consent expires in up to 12 months.

## Security

- `client_secret` only in the VDI's `.env`, never in the frontend or in git.
- The `raw` field may contain sensitive data (Pix counterparty name). It stays only in the local SQLite. The backup gets the same care as the database.
- The webhook requires public HTTPS, which depends on the phase 2 proxy.

## Tests

- Anonymized OFX and CSV fixtures and parser tests.
- Table tests of the classification rules: bill, internal transfer, duplicate, rule, refund and installment.
- Idempotency: importing twice does not change counts.
- Pluggy client tested against an `httptest.Server` with responses recorded in the spike.

## Acceptance criteria

- **4a:** importing a month's checking account OFX and the card CSV produces the inbox with:
  - card bill payment ignored;
  - transfers between own accounts ignored;
  - expenses already entered by hand suggested as links.
  Accepting everything produces a `/summary` without double counting.
- **4b:** connect through Meu Pluggy and, within 6 hours (or on manual sync), see the new transactions in the inbox with no action.
