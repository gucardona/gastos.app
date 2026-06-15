# Unit Tests Design

**Date:** 2026-06-15
**Status:** Approved

## Context

The codebase is a Go + SQLite personal finance API (`gastos.app`). Two test files already exist:

- `src/db/db_test.go` — migration backfill (accounts_v1)
- `src/main_test.go` — integration tests: auth, accounts, members, expenses, incomes, goals, recurring expenses

Uncovered: categories, payment methods, recurring incomes, recurring payments, split payments, pure logic in validation/models/events/helpers.

## Approach

Bottom-up: pure unit tests first (logic with no I/O), then integration tests for uncovered HTTP endpoints.

## Section 1 — Pure Unit Tests (no DB, no HTTP)

### `src/models/models_test.go` (`package models`)

| Function | Cases |
|---|---|
| `PermissionsForRole` | owner → all true; editor → CanEdit only; reader → all false; unknown → all false |
| `IsValidAccountRole` | owner/editor/reader → true; empty/other → false |
| `IsValidShareRole` | editor/reader → true; owner/empty/other → false |

### `src/handlers/validation_test.go` (`package handlers`)

| Function | Cases |
|---|---|
| `validateExpense` | amount ≤ 0, empty description/category/payment, invalid date |
| `validateIncome` | amount ≤ 0, empty description/type, invalid date |
| `validateGoal` | empty category, limit ≤ 0 |
| `validateRecurringExpense` | amount, description, category, payment, invalid frequency, dayOfMonth < 1 or > 31, invalid startDate, endDate before startDate |
| `validateRecurringIncome` | amount, description, type, dayOfMonth range, date ordering |
| `validateAccountName` | empty/whitespace-only name |
| `validateCategory` | empty name, empty icon, invalid color (no `#`, wrong length, non-hex), invalid essentiality |
| `validatePaymentMethod` | empty name, empty icon |
| `validateShareRole` | invalid role string |
| `isValidDate` | valid ISO date, empty string, garbage, wrong separator |

> `validateRecurringPayment` calls `accountMemberSplits` (DB-dependent) — tested via integration tests only.

### `src/events/bus_test.go` (`package events`)

| Case | Description |
|---|---|
| Notify excludes actor | Subscriber A notifies account; subscriber B receives, A does not |
| Unsubscribe | After unsubscribe, channel receives no more events |
| Full channel | Notify with full channel does not block |
| Race safety | Concurrent Subscribe/Notify/Unsubscribe passes `-race` |

### `src/handlers/helpers_test.go` (`package handlers`)

| Function | Cases |
|---|---|
| `parseIntPathID` | valid ID, empty suffix, "0", negative, non-numeric string |
| `parseSplitParticipantIds` | empty string, single valid, comma-separated, mixed invalid, negative values ignored |
| `splitAmount` | 100×50% = 50.00; 100×33.333% rounds correctly; 0 amount |

## Section 2 — Integration Tests (new HTTP endpoints)

All follow the existing pattern in `src/main_test.go`: `newTestServer(t)` + `apiClient.request()`. New `Test*` functions appended to `main_test.go`.

### `TestCategoriesAndPaymentMethods`

- Owner creates category → 201, fields round-trip (key, name, icon, color, essentiality, sortOrder)
- Owner lists categories → includes created one
- Owner patches category → updated fields returned
- Owner deletes category → 204; subsequent list excludes it
- Reader attempt to create → 403
- Same lifecycle for payment methods

### `TestRecurringIncomesLifecycle`

- Create recurring income with all fields (dayOfMonth, startDate, endDate, type, amount, description) → 201
- List → 1 result with correct fields
- Patch → updated fields returned
- Delete → 204; list returns empty
- Reader cannot create/patch/delete → 403

### `TestRecurringPaymentsLifecycle`

- Shared account with owner + editor
- Owner creates recurring payment (payer=owner, receiver=editor) → 201
- List → 1 result
- Patch (new amount, dayOfMonth) → updated
- Delete → 204
- Same payer and receiver → 400
- Payer not in account → 400

### `TestSplitPaymentsLifecycle`

- Shared account with owner + editor
- Create split payment (payer, receiver, amount, date, note) → 201
- List → 1 result
- Patch → updated fields
- Delete → 204
- Reader attempt → 403

### `TestExpenseSplitsRounding`

- Two-member account; create expense with explicit splits summing to 100% → 201, per-member amounts correct
- Splits not summing to 100 → 400
- Split referencing non-member → 400
- No splits provided → defaults to equal split across members

## File Layout

```
src/
  models/
    models_test.go          ← NEW (package models)
  events/
    bus_test.go             ← NEW (package events)
  handlers/
    validation_test.go      ← NEW (package handlers)
    helpers_test.go         ← NEW (package handlers)
  main_test.go              ← EXTENDED (new Test* functions appended)
```

## Out of Scope

- Frontend / web assets
- `db/db_test.go` — already covers its migration logic
- `handlers/events.go` (SSE streaming) — not testable with standard `httptest`
- DB migration internals beyond what `db_test.go` already covers
