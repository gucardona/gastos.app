# Credit cards (purchase in the invoice month) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A payment method can be a credit card (closing day, due day, issuer, physical/virtual, last 4), and a purchase made with it counts, everywhere in the app, in the month its invoice is due.

**Architecture:** One pure Go function computes the invoice due date from (purchase date, closing day, due day). Every write path stores it in a new nullable `transactions.invoice_due`; a virtual generated column `transactions.month = substr(COALESCE(invoice_due, date), 1, 7)` becomes the month every read filters on. "Realized" keeps meaning `date <= today`.

**Tech Stack:** Go (net/http, modernc SQLite 1.61), React 19 + Vite + TS, TanStack Query, Vitest, Playwright.

**Spec:** `docs/superpowers/specs/2026-11-25-credit-cards-design.md`

## Global Constraints

- The production DB holds real data: never reset or recreate it. Migration 0008 only adds nullable columns and an index; no existing row changes, so every existing transaction keeps its month.
- Code, comments and tests in English; UI copy and docs in pt-BR. The partner is "a Rafa".
- Store only the last 4 digits of a card (`^[0-9]{4}$`), never the full number, expiry or CVV.
- The invoice due date is computed only when a purchase is created, when it is edited **and its date or payment method changed**, when a recurrence materializes, and when a bank line is accepted. Changing a card's days never moves stored purchases.
- Month = month of the invoice **due** date. Applies to income on a card too (a refund credits the invoice).
- Go tests: `export PATH="$PATH:/c/msys64/ucrt64/bin" && go test -race ./...`. Web: `cd web && npx vitest run`.
- Each commit ends with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. A card purchase whose amount or description is edited stays in its invoice month, even after the card's closing day changed: pinned in Task 4 (`TestEditKeepsInvoiceWhenDateAndMethodUnchanged`).
2. A card recurrence whose occurrence is dated in the previous month (22/12, closing day 20) is projected in January's "Próximos" (Upcoming) and not in December's: pinned in Task 5 (`TestUpcomingCardRecurrenceLandsInInvoiceMonth`).
3. Turning the card switch off, or archiving the card: new purchases or occurrences on an archived card still use its days, and a method that is no longer a card stamps nothing. Pinned in Task 4 (`TestArchivedCardStillStamps`, `TestPlainMethodStampsNothing`).
4. Day 31 and February, including a leap year: pinned in Task 1's table (rows for 2027-02 and 2028-02).
5. last4 junk (`"12345"`, `"12a4"`, `" 1234 "` trimmed to valid) and only one of the two days filled: pinned in Task 3 (`TestCardFieldsValidation`).

---

### Task 1: Invoice due date function

**Files:**
- Create: `internal/billing/invoice.go`
- Test: `internal/billing/invoice_test.go`

**Interfaces:**
- Produces: `billing.InvoiceDue(date string, closingDay, dueDay int) (string, error)`, where `date` and the result are `YYYY-MM-DD`; it errors on a bad date or on days outside 1–31. It also produces `billing.BestPurchaseDay(closingDay int) int`, which returns `closingDay+1` and wraps 31 to 1. The UI needs it only as a hint, so it is not clamped per month.

- [ ] **Step 1: Write the failing table test `TestInvoiceDue`**

```go
cases := []struct{ date string; closing, due int; want string }{
	{"2026-11-22", 20, 27, "2026-12-27"},
	{"2026-11-20", 20, 27, "2026-11-27"}, // the closing day itself is in
	{"2026-11-05", 30, 7, "2026-12-07"},
	{"2027-02-15", 31, 10, "2027-03-10"}, // closes on 28/02
	{"2028-02-29", 31, 10, "2028-03-10"}, // leap year: 29/02 is the closing day
	{"2027-03-01", 31, 10, "2027-04-10"},
	{"2026-12-26", 25, 5, "2027-02-05"},
	{"2026-11-10", 15, 15, "2026-12-15"}, // due == closing: next month
}
```
Also add `TestInvoiceDueRejects`, which expects an error for `("2026-13-01", 20, 27)`, `("2026-11-01", 0, 27)` and `("2026-11-01", 20, 32)`, and `TestBestPurchaseDay` with `(20)=21` and `(31)=1`.

- [ ] **Step 2: Run `go test ./internal/billing/`.** Expected: FAIL, undefined `InvoiceDue`.
- [ ] **Step 3: Implement `InvoiceDue` and `BestPurchaseDay`**

The rule, from the spec:
1. The closing is in the purchase month if `day(p) <= clamp(closingDay)`, otherwise in the next month.
2. The due date is in the closing month if `dueDay > closingDay`, otherwise in the month after.
3. Clamp every day to the last day of its month, the same way as `recurrence.Occurrences`.

- [ ] **Step 4: Run `go test ./internal/billing/`.** Expected: PASS.
- [ ] **Step 5: Commit** `feat(billing): invoice due date of a card purchase`

### Task 2: Migration 0008

**Files:**
- Create: `internal/db/migrations/0008_credit_cards.sql`
- Modify: `internal/db/db.go:54-57` (embed and append `migration0008`)
- Modify: `internal/db/db_test.go`
- Modify: `docs/assistente.md` (add the rollback 8→7, newest first)

**Interfaces:**
- Produces these columns: `payment_methods.closing_day` and `due_day`, INTEGER with a column `CHECK (… BETWEEN 1 AND 31)`. SQLite cannot add a table CHECK through ALTER, so the both-or-neither rule lives in the API (Task 3). Also `issuer TEXT`, `card_form TEXT CHECK (card_form IN ('physical','virtual'))` and `last4 TEXT CHECK (last4 GLOB '[0-9][0-9][0-9][0-9]')`. On `transactions`: `invoice_due TEXT`, plus `month TEXT GENERATED ALWAYS AS (substr(COALESCE(invoice_due, date), 1, 7)) VIRTUAL`. Index: `transactions_wallet_month ON transactions(wallet_id, month)`.

- [ ] **Step 1: Write the failing tests.**
  - Update the `7` in `TestOpenCreatesFullSchema` and `TestOpenUpgradesVersion4InPlace` to `8`.
  - In the v4 upgrade test, add `SELECT COUNT(*) FROM transactions WHERE month = '2026-10' AND invoice_due IS NULL` = 1.
  - Add `TestCardColumnsCheck`. It must accept `last4='0042'`, and reject `last4='12a4'`, `'12345'`, `closing_day=0`, `due_day=32` and `card_form='plastic'`.
- [ ] **Step 2: Run `go test ./internal/db/`.** Expected: FAIL, version 7.
- [ ] **Step 3: Write the migration, embed it, and add the rollback to `docs/assistente.md`:**
  - `DROP INDEX transactions_wallet_month;`
  - `ALTER TABLE transactions DROP COLUMN month;`
  - `ALTER TABLE transactions DROP COLUMN invoice_due;`
  - drop the five `payment_methods` columns;
  - `PRAGMA user_version=7;`
- [ ] **Step 4: Run `go test -race ./internal/db/`.** Expected: PASS, including `TestUpgradeMakesPreMigrationCopy`.
- [ ] **Step 5: Commit** `feat(db): migration 0008, cards and invoice month (in place, no data changes)`

### Task 3: Card fields on payment methods (API)

**Files:**
- Modify: `internal/api/catalog.go:255-420` (DTO, input, validate, cols, insert, update)
- Test: `internal/api/cards_test.go`

**Interfaces:**
- Produces JSON on `paymentMethodDTO` and `paymentMethodInput`:
  - `closingDay *int`, `dueDay *int`;
  - `issuer string` (trimmed, max 60);
  - `cardForm string` (`""`, `"physical"` or `"virtual"`; `""` stores NULL);
  - `last4 string` (trimmed, `""` or exactly 4 digits);
  - output only: `bestPurchaseDay *int`, from `billing.BestPurchaseDay` when it is a card.
- PUT replaces all fields. A method is a card when both days are set.

- [ ] **Step 1: Write the failing tests** with `newTestApp`/`register`.
  - `TestCreateCard` does a POST with `{name:"Itaú Black", icon:"💳", closingDay:20, dueDay:27, issuer:"Itaú Mastercard", cardForm:"virtual", last4:" 1234 "}` and expects 201, the same fields back with `last4 "1234"` and `bestPurchaseDay 21`, and the same values from GET.
  - `TestCardFieldsValidation` expects 400 with the field keys `closingDay` for a closing day without a due day, `dueDay` for 32, `last4` for `"12345"` and `"12a4"`, and `cardForm` for `"plastic"`.
  - `TestTurnCardOff` does a PUT with both days null and expects them back as null.
  - The pt-BR messages are `"informe o fechamento e o vencimento"`, `"dia entre 1 e 31"`, `"use os 4 últimos dígitos"` and `"físico ou virtual"`.
- [ ] **Step 2: Run `go test ./internal/api/ -run 'Card'`.** Expected: FAIL.
- [ ] **Step 3: Implement.** Extend `paymentMethodCols`/`scanPaymentMethod`, the INSERT and the UPDATE. Validate in `paymentMethodInput.validate`.
- [ ] **Step 4: Run `go test -race ./internal/api/`.** Expected: PASS.
- [ ] **Step 5: Commit** `feat(api): a payment method can be a credit card`

### Task 4: Stamp `invoice_due` on every write

**Files:**
- Modify: `internal/api/transactions.go`
  - add `invoiceDueFor`;
  - change `saveTransaction` (create and bank accept go through it);
  - change `updateTransaction`;
  - add `InvoiceDue *string \`json:"invoiceDue"\`` to `transaction` and to the list SELECT and scan.
- Modify: `internal/recurrence/materializer.go`. In `load`, add `LEFT JOIN payment_methods pm ON pm.id = r.payment_method_id` and select `pm.closing_day`, `pm.due_day` into `template`. In `insert`, set `invoice_due` from `billing.InvoiceDue(o.Date, …)` when both days are set.
- Test: `internal/api/cards_test.go`, `internal/recurrence/materializer_test.go`

**Interfaces:**
- Consumes: `billing.InvoiceDue` (Task 1) and the columns from Task 2.
- Produces: `invoiceDueFor(ctx context.Context, q querier, paymentMethodID *int64, date string) (sql.NullString, error)`. It returns NULL when the method is nil or not a card, and ignores `archived_at`. Task 7 also gets `invoiceDue` (`string|null`) on every transaction in the JSON.

- [ ] **Step 1: Write the failing tests.** The card in all of them has days 20/27.
  - `TestCardPurchaseGetsInvoiceDue`: a purchase on 2026-11-22 returns `invoiceDue "2026-12-27"`; one on 2026-11-20 returns `"2026-11-27"`; an income (refund) on 2026-11-22 also returns `"2026-12-27"`.
  - `TestPlainMethodStampsNothing`: Pix gives `invoiceDue null`.
  - `TestEditKeepsInvoiceWhenDateAndMethodUnchanged`:
    1. create on 11-22;
    2. PUT the card with closing day 25;
    3. PUT the purchase with a new amount;
    4. `invoiceDue` is still `"2026-12-27"`.
    5. Then PUT the purchase with date 2026-11-24: it becomes `"2026-11-27"` under the new rule, because 24 ≤ 25.
  - `TestEditToPlainMethodClearsInvoice`: changing to Pix makes it null.
  - `TestArchivedCardStillStamps`: archive the card, then a recurrence on it materializes with `invoice_due` set.
  - `TestBankAcceptOnCardAccount`: a bank account linked to the card; the accepted line dated 11-22 gets `"2026-12-27"`. Follow the existing bank accept tests for setup.
- [ ] **Step 2: Run `go test ./internal/api/ ./internal/recurrence/`.** Expected: FAIL.
- [ ] **Step 3: Implement.** In `updateTransaction`, read the current `date, payment_method_id` together with the existing SELECT, and recompute only if one of them changed. Otherwise keep the stored value (`invoice_due = invoice_due`).
- [ ] **Step 4: Run `go test -race ./...`.** Expected: PASS.
- [ ] **Step 5: Commit** `feat: card purchase stores the invoice due date`

### Task 5: Every month read uses `month`

**Files:**
- Modify: `internal/api/summary.go`.
  - `totals(ctx, walletID, month, today string)` filters `WHERE wallet_id = ? AND month = ? AND date <= ?`; call it for the current and the previous month.
  - byCategory and byEssentiality filter `t.month = ? AND t.date <= ?`.
  - The upcoming transactions filter `t.month = ? AND t.date > ?`.
  - Recurrence projection: also select `pm.closing_day, pm.due_day`. For each recurrence, take the last 3 occurrences up to `last` and keep those whose billing month (`InvoiceDue(o.Date)[:7]` for a card, else `o.Month`) equals the month, with `o.Date > today`, and not already materialized or skipped for **that occurrence's** `o.Month`. Move the two `NOT EXISTS` checks from the SQL to per-occurrence lookups.
- Modify: `internal/api/transactions.go:255-300`. `listTransactions` accepts `month=YYYY-MM` (filters `month = ?`) as an alternative to `from`+`to`; exactly one of the two forms is required.
- Test: `internal/api/summary_card_test.go`

**Interfaces:**
- Produces: `GET /api/wallets/{id}/transactions?month=YYYY-MM`. The `upcoming` items keep their shape, and `date` is the purchase date.

- [ ] **Step 1: Write the failing tests.** Freeze `s.now` at 2026-12-05 the way `summary_future_test.go` does. Card 20/27; restaurant purchase of 8700 on 2026-11-22.
  - `TestCardPurchaseCountsInInvoiceMonth`: the November summary does not include it (expense, byCategory, byEssentiality); December includes it in all three, and in `previous` of January.
  - `TestListByMonth`: `?month=2026-12` returns it with `invoiceDue`; `?month=2026-11` does not; giving neither or both forms returns 400.
  - `TestUpcomingCardRecurrenceLandsInInvoiceMonth`: a card expense recurrence on day 22 (start 2026-12-01). With now = 2026-12-05, the projection for 2026-12-22 is in January's `upcoming` and `upcomingExpenseCents`, and not in December's.
  - `TestOldTransactionsKeepTheirMonth`: a plain transaction on 2026-11-22 stays in November.
- [ ] **Step 2: Run `go test ./internal/api/ -run 'Card|ListByMonth|OldTransactions'`.** Expected: FAIL.
- [ ] **Step 3: Implement the changes listed under Files.**
- [ ] **Step 4: Run `go test -race ./...`.** Expected: PASS, including `summary_future_test.go` unchanged.
- [ ] **Step 5: Commit** `feat: Overview, limits and list count the purchase in the invoice month`

### Task 6: Assistant

**Files:**
- Modify: `internal/api/assistant_tools.go:245-300`. With `agrupar = mes`, the bucket is `t.month` and the filter is `t.month BETWEEN substr(?,1,7) AND substr(?,1,7)` instead of the date range. `dia` and `semana` keep the purchase date.
- Modify: `internal/api/chat.go:45`. Add one line to the system text: `"Compras no cartão de crédito contam no mês em que a fatura vence."`
- Test: the existing assistant tools test file (`grep -l totais_por_periodo internal/api/*_test.go`).

- [ ] **Step 1: Write the failing test `TestMonthlyTotalsUseInvoiceMonth`.** Card purchase on 2026-11-22 (days 20/27); `totais_por_periodo` for 2026-11-01..2026-12-31 by `mes` puts 8700 in `2026-12`.
- [ ] **Step 2: Run it.** Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run `go test -race ./internal/api/`.** Expected: PASS.
- [ ] **Step 5: Commit** `feat(assistant): monthly totals follow the invoice month`

### Task 7: Web, card registration

**Files:**
- Modify: `web/src/api/types.ts`.
  - `PaymentMethod` gets `closingDay: number | null; dueDay: number | null; issuer: string; cardForm: "" | "physical" | "virtual"; last4: string; bestPurchaseDay: number | null`.
  - `Transaction` gets `invoiceDue: string | null`.
- Modify: `web/src/api/queries.ts:38-44`. `useTransactions` fetches `?month=${month}`.
- Modify: `web/src/components/PaymentSheet.tsx`. Add a `Switch` "Cartão de crédito" (Credit card). When it is on, show, in order:
  - Banco / bandeira (Bank / card network);
  - Tipo (Type), a `Segmented` with Físico (Physical) and Virtual;
  - Final (Last digits) (`inputMode="numeric"`, maxLength 4);
  - Compras até o dia (Purchases up to day);
  - Vence no dia (Due on day) (`inputMode="numeric"`, 1–31);
  - the line "Melhor dia de compra: N" (Best purchase day), computed locally with the same wrap as `BestPurchaseDay`.
  When it is off, send both days as null. Field errors come from `useAction`.
- Modify: `web/src/pages/settings/PaymentsSection.tsx`. A card's row subtitle: `fecha 20 · vence 27 · final 1234` (closes 20 · due 27 · last digits 1234) (omit the empty parts).
- Test: `web/src/components/PaymentSheet.test.tsx` (create, following AddSheet.test.tsx's mocking)

- [ ] **Step 1: Write the failing tests.**
  - Turning the switch on reveals the five fields.
  - Typing 20 shows "Melhor dia de compra: 21".
  - Submitting POSTs `{closingDay:20, dueDay:27, cardForm:"virtual", last4:"1234", issuer:"Itaú"}`.
  - An existing card opens with the switch on.
  - Turning it off sends `closingDay:null, dueDay:null`.
- [ ] **Step 2: Run `cd web && npx vitest run src/components/PaymentSheet.test.tsx`.** Expected: FAIL.
- [ ] **Step 3: Implement.** Reuse the house `Switch`, `Segmented` and `.list-row` rows. The style guard tests in `web/src/styles/*.test.ts` must stay green.
- [ ] **Step 4: Run `cd web && npx vitest run && npx tsc -b`.** Expected: PASS.
- [ ] **Step 5: Commit** `feat(web): credit card registration in payment methods`

### Task 8: Web, purchase in another month

**Files:**
- Modify: `web/src/hooks/useAction.ts`. `run<T>(fn: () => Promise<T>, done?: string | ((result: T) => string))`.
- Modify: `web/src/components/AddSheet.tsx`. For a one-off purchase saved with a card, if `res.invoiceDue` falls in a month other than `res.date`'s, the toast is `Lançado na fatura de ${monthName}` (Posted to the ... bill), with the month name in full and lowercase via `Intl` pt-BR (e.g. "dezembro"). Otherwise the toast keeps its current text.
- Modify: `web/src/components/TransactionList.tsx`. In the meta line, when `invoiceDue` is set and its month differs from `date`'s, add `fatura de dez` (December bill): the short month without the dot, from the same `Intl` helper. Add that helper to `web/src/lib/dates.ts` as `monthShort(iso: string): string` and `monthLong(iso: string): string`.
- Modify: `web/e2e/app.spec.ts`. In one test, create a card (20/27), add a purchase dated the 22nd of the current month, and check that it appears with "fatura de" in the next month's Lançamentos (Transactions) and not in the current month's.
- Test: `web/src/components/TransactionList.test.tsx`, `web/src/components/AddSheet.test.tsx`

- [ ] **Step 1: Write the failing tests.**
  - TransactionList: an item `{date:"2026-11-22", invoiceDue:"2026-12-27"}` shows "fatura de dez"; one with `invoiceDue:"2026-11-27"` and one with `null` show no such text.
  - AddSheet: when the POST response carries `invoiceDue:"2026-12-27"` and `date:"2026-11-22"`, the toast reads "Lançado na fatura de dezembro".
- [ ] **Step 2: Run `cd web && npx vitest run src/components/TransactionList.test.tsx src/components/AddSheet.test.tsx`.** Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run `cd web && npx vitest run`, then `npm run build && npx playwright test`, then restore `web/dist/.gitkeep`.** Expected: PASS on desktop and Pixel 5.
- [ ] **Step 5: Commit** `feat(web): card purchase shows the invoice it landed in`

---

After Task 8: run the full Go and web suites, open the PR to master. Do not merge without the owner's explicit ok, because merging deploys to production and runs migration 0008 on real data.
