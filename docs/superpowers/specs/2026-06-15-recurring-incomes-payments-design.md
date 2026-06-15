# Recurring Incomes & Recurring Payments

**Date:** 2026-06-15
**Status:** Approved

## Goal

Add recurring templates for incomes and split payments, mirroring the existing recurring expenses feature exactly. Same toggle UX, same materialization-on-GET mechanism, same template card below each list.

## Database

### New tables

```sql
CREATE TABLE recurring_incomes (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  account_id   INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  amount       REAL NOT NULL,
  description  TEXT NOT NULL DEFAULT '',
  type         TEXT NOT NULL DEFAULT 'other',
  day_of_month INTEGER NOT NULL,
  start_date   TEXT NOT NULL,
  end_date     TEXT,
  enabled      INTEGER NOT NULL DEFAULT 1,
  created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE recurring_payments (
  id               INTEGER PRIMARY KEY AUTOINCREMENT,
  account_id       INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  payer_user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  receiver_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  amount           REAL NOT NULL,
  note             TEXT NOT NULL DEFAULT '',
  day_of_month     INTEGER NOT NULL,
  start_date       TEXT NOT NULL,
  end_date         TEXT,
  enabled          INTEGER NOT NULL DEFAULT 1,
  created_at       DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### Migrations (named, idempotent)

- `recurring_incomes_v1`: adds `recurring_income_id INTEGER` nullable column to `incomes` + creates `recurring_incomes` table + index
- `recurring_payments_v1`: adds `recurring_payment_id INTEGER` nullable column to `split_payments` + creates `recurring_payments` table + index

The FK columns on `incomes` and `split_payments` serve two purposes:
1. Idempotency check — before materializing a month, query `COUNT(*) WHERE recurring_income_id = ? AND date LIKE 'YYYY-MM-%'`
2. Frontend badge — materialized rows show `↻ recorrente`

## Backend

### Materialization

Two new functions in `src/db/db.go`, following `MaterializeRecurringExpenses` exactly:

- `MaterializeRecurringIncomes(accountID int64) error` — called at the top of `GET /api/incomes`
- `MaterializeRecurringPayments(accountID int64) error` — called at the top of `GET /api/split-payments`

Same cursor logic: start at first day of `start_date` month, advance monthly until current month, compute target date (clamp to last day of month), skip if `targetDate > today` or outside `[startDate, endDate]`, skip if a row already exists for that month.

### New handler files

`src/handlers/recurring_incomes.go` — full CRUD, same structure as `recurring_expenses.go`:
- `GET /api/recurring-incomes` — list all for account
- `POST /api/recurring-incomes` — create; calls `events.Bus.Notify`
- `PATCH /api/recurring-incomes/:id` — update; calls `events.Bus.Notify`
- `DELETE /api/recurring-incomes/:id` — delete; calls `events.Bus.Notify`

`src/handlers/recurring_payments.go` — same:
- `GET /api/recurring-payments`
- `POST /api/recurring-payments`
- `PATCH /api/recurring-payments/:id`
- `DELETE /api/recurring-payments/:id`

### Validation

`validateRecurringIncome`: amount > 0, valid type, valid date format for start_date, day_of_month in 1–31, description non-empty.

`validateRecurringPayment`: amount > 0, payer ≠ receiver, both are account members, day_of_month in 1–31, valid start_date.

### Models

```go
type RecurringIncome struct {
  ID          int64   `json:"id"`
  UserID      int64   `json:"-"`
  AccountID   int64   `json:"-"`
  Amount      float64 `json:"amount"`
  Description string  `json:"description"`
  Type        string  `json:"type"`
  DayOfMonth  int     `json:"dayOfMonth"`
  StartDate   string  `json:"startDate"`
  EndDate     string  `json:"endDate"`
  Enabled     bool    `json:"enabled"`
}

type RecurringPayment struct {
  ID              int64   `json:"id"`
  AccountID       int64   `json:"-"`
  PayerUserID     int64   `json:"payerUserId"`
  PayerName       string  `json:"payerName,omitempty"`
  ReceiverUserID  int64   `json:"receiverUserId"`
  ReceiverName    string  `json:"receiverName,omitempty"`
  Amount          float64 `json:"amount"`
  Note            string  `json:"note"`
  DayOfMonth      int     `json:"dayOfMonth"`
  StartDate       string  `json:"startDate"`
  EndDate         string  `json:"endDate"`
  Enabled         bool    `json:"enabled"`
}
```

### Routes (src/main.go)

```
GET|POST|PATCH|DELETE /api/recurring-incomes
GET|POST|PATCH|DELETE /api/recurring-incomes/:id
GET|POST|PATCH|DELETE /api/recurring-payments
GET|POST|PATCH|DELETE /api/recurring-payments/:id
```

All behind `middleware.Auth` + `middleware.Account`.

### Income GET — also scan recurring_income_id

Update `GET /api/incomes` to select `COALESCE(recurring_income_id, 0)` so the frontend receives it and can show the badge. Same for `split_payments`.

## Frontend (src/web/index.html)

### State additions

```js
recurringIncomes: [],
recurringPayments: [],
editingRecurringIncomeInForm: '',
editingRecurringPaymentInForm: '',
newInc: { ..., isRecurring: false, dayOfMonth: 1, startDate: '', endDate: '', enabled: true },
newSplitPayment: { ..., isRecurring: false, dayOfMonth: 1, startDate: '', endDate: '', enabled: true },
```

### loadAllData

Add `API.get('/api/recurring-incomes')` and `API.get('/api/recurring-payments')` to the parallel fetch.

### Income form

Add "Recorrente?" toggle (same button style as expense form). When on:
- Hide `date` field
- Show `day_of_month` (number input 1–31), `start_date` (flatpickr), `end_date` (flatpickr, clearable), `enabled` toggle button
- Submit button text: "Configurar" (create) / "Salvar" (edit)

Below the income list: "Recorrências de entradas" card with `recurring-grid` layout, same `.recurring-card` style as expenses. Each card shows description, amount (green), type label, day + start/end, enabled toggle, edit/delete buttons.

Materialized incomes show `↻ recorrente` badge (same style as expenses) when `recurringIncomeId` is set.

### Payment form

Add "Recorrente?" toggle. When on:
- Hide `date` field
- Show `day_of_month`, `start_date`, `end_date`, `enabled` toggle
- Payer, receiver, amount, note stay as-is

Below the payment list: "Pagamentos recorrentes" card with same recurring-grid layout. Each card shows payer → receiver, amount, day + dates, enabled toggle, edit/delete.

Materialized payments show `↻ recorrente` badge when `recurringPaymentId` is set.

### cancelIncomeEdit / cancelSplitPaymentEdit

Reset `isRecurring`, `editingRecurringIncomeInForm`, `editingRecurringPaymentInForm` on cancel/submit.

## End-to-end behaviour

1. User toggles "Recorrente?" on in income or payment form → recurring-specific fields appear
2. Submits → POST to `/api/recurring-incomes` or `/api/recurring-payments`, template saved
3. Next time `GET /api/incomes` or `GET /api/split-payments` is called, materialization runs and inserts any due rows
4. Materialized rows appear in the normal list with `↻ recorrente` badge
5. Deleting a template removes only the template; materialized rows remain
6. End date is inclusive: the last materialization fires on `end_date` if `day_of_month` matches
