# Bank import (OFX and CSV) with an inbox — plan

**Date:** 2026-10-11
**Status:** to implement
**Reference spec:** [open-finance-design](../specs/2026-10-09-open-finance-design.md) (step 4a). 4b (Pluggy) still depends on the spike.

## Goal

Stop typing what the bank already knows. The person imports the statement (OFX or CSV); the transactions land in an **inbox** with a suggested category; the person confirms in bulk. Nothing becomes a transaction without confirmation.

## Scope of this step (smallest version that solves the problem)

In:
- Import **OFX** (the standard of Brazilian banks) and **generic CSV** with a column mapping chosen on the spot (date, description, amount).
- "File" bank accounts tied to a wallet, a payment method and who pays.
- Inbox: accept, ignore, link to an existing transaction, accept everything that has a suggestion.
- Categorization rules learned when the person changes the category ("Sempre categorizar 'UBER' como Transporte" (Always categorize 'UBER' as Transport)).
- Deduplication: reimporting the same file does not duplicate; a similar manual transaction becomes a link suggestion.

Out (for later, in this order): ready-made profiles per bank (Nubank, Inter), credit card bill payment and transfers between own accounts, installments and refunds, Pluggy.

## Data (`user_version` 2)

New migration in `internal/db` (the first one using the `migrations` mechanism):

- `bank_accounts(id, wallet_id, name, payment_method_id, paid_by_user_id, sync_from, enabled)`; only the "file" type for now, so the provider columns from the spec are left for 4b.
- `bank_transactions(id, bank_account_id, external_id, date, amount_cents, description, status, transaction_id, suggested_category_id, raw, imported_at)`, with `UNIQUE (bank_account_id, external_id)` and `status IN ('pending','accepted','matched','ignored')`.
- `categorization_rules(id, wallet_id, pattern, category_id, rename_to, priority)`.
- `transactions.source` (`'manual'|'recurring'|'bank'`), used to know where it came from.

`external_id`: the OFX `FITID`; in CSV, a hash of `date|amount|description|nth identical occurrence in the file`.

## Backend

Package `internal/bankimport`:
- `ParseOFX(r io.Reader) ([]Entry, error)`: own tolerant parser (OFX 1.x is SGML without closing tags), with anonymized fixtures.
- `ParseCSV(r, mapping) ([]Entry, error)`: detected delimiter (`;` or `,`), Brazilian number format, dates `dd/mm/aaaa` and `aaaa-mm-dd` (day/month/year and year-month-day), optional header.
- `Classify(entry, rules, existing) Suggestion`: rule by pattern (accent and case insensitive, highest priority wins) and link candidates (same absolute amount, date ±2 days).

API (all under `/api/wallets/{walletID}`, minimum role `editor`):
- `GET/POST /bank-accounts`, `PUT/DELETE /bank-accounts/{id}`
- `POST /bank-accounts/{id}/import` (multipart, 5 MB maximum) → returns `{imported, duplicates}`
- `GET /inbox?status=pending` and `POST /inbox/{id}/accept` (category, description, optional split), `/ignore`, `/match` (transaction id), `POST /inbox/accept-suggested`
- `GET/POST/DELETE /rules`

Signed amounts come from the file: negative becomes `expense`, positive `income`.

## Frontend

- Route `/caixa` with a pending-items badge in the navigation.
- List of pending items by day; suggested category editable on the row; when changed, it offers to create the rule.
- In Ajustes (Settings), a "Contas bancárias" (Bank accounts) section: register the account and import the file (with the column choice for CSV).

## Tasks (TDD, each with its own commit)

- [ ] **1. Migration v2** and a test that a new database and a v1 database reach the same schema.
- [ ] **2. `ParseOFX`** with fixtures: amounts with comma and dot, dates with time zone, tags without closing, empty file and a file that is not OFX.
- [ ] **3. `ParseCSV`** with `;` and `,`, Brazilian number format, optional header, blank lines, missing columns.
- [ ] **4. `Classify`** (pure): rules, priority, accents, link suggestion and the limits of the ±2 day window.
- [ ] **5. Import and inbox** (integration): idempotent import, accept creates the transaction with `source='bank'` and correct amount/date, ignore, link, accept all suggested, permissions by role, size limit.
- [ ] **6. Rules**: creating one from a category change reapplies it to the pending items.
- [ ] **7. Frontend**: inbox and import (component tests) and one e2e: import a small OFX, accept and see the balance change.
- [ ] **8. Docs**: how to export the statement from the bank, supported formats and limitations.

## Risks

- OFX files vary a lot between banks; that is why the parser is tolerant, and real anonymized statements are needed for fixtures. Without them, only the standard format is guaranteed.
- Reimporting a whole month after entering transactions by hand generates link suggestions; anyone who accepts without looking can create duplicates. The screen shows the candidate next to the row.
- Financial data in `raw`: it stays only in the local SQLite and goes into the encrypted backup.
