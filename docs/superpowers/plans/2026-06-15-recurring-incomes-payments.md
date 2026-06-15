# Recurring Incomes & Recurring Payments — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add recurring templates for incomes and split payments, mirroring the existing recurring expenses pattern exactly — same toggle UX in the form, materialization on GET, template cards below the list.

**Architecture:** Two new DB tables (`recurring_incomes`, `recurring_payments`) with two named migrations adding FK columns to `incomes` and `split_payments`. Each GET handler materializes due rows before querying. Two new CRUD handler files. Frontend adds a "Recorrente?" toggle to income and payment forms with conditional field sets, template card grids below each list, and `↻ recorrente` badges on materialized rows.

**Tech Stack:** Go 1.26, `modernc.org/sqlite`, Alpine.js frontend in a single `src/web/index.html`.

---

## File Map

| File | Action | What changes |
|------|--------|--------------|
| `src/db/db.go` | Modify | Add 2 CREATE TABLE statements, 2 migration functions, 2 materialization functions |
| `src/models/models.go` | Modify | Add `RecurringIncome`, `RecurringPayment` structs; add `RecurringIncomeID *int64` to `Income`; add `RecurringPaymentID *int64` to `SplitPayment` |
| `src/handlers/validation.go` | Modify | Add `validateRecurringIncome`, `validateRecurringPayment` |
| `src/handlers/recurring_incomes.go` | Create | Full CRUD handler for `/api/recurring-incomes` |
| `src/handlers/recurring_payments.go` | Create | Full CRUD handler for `/api/recurring-payments` |
| `src/handlers/incomes.go` | Modify | Call `MaterializeRecurringIncomes` on GET; scan `recurring_income_id` |
| `src/handlers/split_payments.go` | Modify | Call `MaterializeRecurringPayments` on GET; scan `recurring_payment_id` |
| `src/main.go` | Modify | Register 4 new route pairs |
| `src/web/index.html` | Modify | State, forms, template lists, badges, JS functions |

---

## Task 1: DB — new tables and migrations

**Files:**
- Modify: `src/db/db.go`

- [ ] **Step 1: Add CREATE TABLE statements for `recurring_incomes` and `recurring_payments`**

In `src/db/db.go`, locate the `queries` slice (around the `split_payments` table definition). Insert the two new tables right after the `split_payments` block and before `schema_migrations`:

```go
`CREATE TABLE IF NOT EXISTS recurring_incomes (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id      INTEGER NOT NULL,
    account_id   INTEGER NOT NULL,
    amount       REAL NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    type         TEXT NOT NULL DEFAULT 'other',
    day_of_month INTEGER NOT NULL,
    start_date   TEXT NOT NULL,
    end_date     TEXT,
    enabled      INTEGER NOT NULL DEFAULT 1,
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(user_id)    REFERENCES users(id)    ON DELETE CASCADE,
    FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE
);`,
`CREATE TABLE IF NOT EXISTS recurring_payments (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id       INTEGER NOT NULL,
    payer_user_id    INTEGER NOT NULL,
    receiver_user_id INTEGER NOT NULL,
    amount           REAL NOT NULL,
    note             TEXT NOT NULL DEFAULT '',
    day_of_month     INTEGER NOT NULL,
    start_date       TEXT NOT NULL,
    end_date         TEXT,
    enabled          INTEGER NOT NULL DEFAULT 1,
    created_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(account_id)       REFERENCES accounts(id) ON DELETE CASCADE,
    FOREIGN KEY(payer_user_id)    REFERENCES users(id)    ON DELETE CASCADE,
    FOREIGN KEY(receiver_user_id) REFERENCES users(id)    ON DELETE CASCADE
);`,
```

Also add indexes in the same queries slice (after the existing `idx_recurring_expenses_account_enabled` line):

```go
`CREATE INDEX IF NOT EXISTS idx_recurring_incomes_account_enabled  ON recurring_incomes(account_id, enabled, day_of_month);`,
`CREATE INDEX IF NOT EXISTS idx_recurring_payments_account_enabled ON recurring_payments(account_id, enabled, day_of_month);`,
```

- [ ] **Step 2: Add migration functions**

In `src/db/db.go`, add two new migration functions after `runRecurringExpensesV1Migration`:

```go
func runRecurringIncomesV1Migration() error {
    applied, err := schemaMigrationApplied("recurring_incomes_v1")
    if err != nil || applied {
        return err
    }
    if _, err := DB.Exec(`ALTER TABLE incomes ADD COLUMN recurring_income_id INTEGER`); err != nil && !isIgnorableMigrationError(err) {
        return err
    }
    _, err = DB.Exec(`INSERT INTO schema_migrations (name) VALUES ('recurring_incomes_v1')`)
    return err
}

func runRecurringPaymentsV1Migration() error {
    applied, err := schemaMigrationApplied("recurring_payments_v1")
    if err != nil || applied {
        return err
    }
    if _, err := DB.Exec(`ALTER TABLE split_payments ADD COLUMN recurring_payment_id INTEGER`); err != nil && !isIgnorableMigrationError(err) {
        return err
    }
    _, err = DB.Exec(`INSERT INTO schema_migrations (name) VALUES ('recurring_payments_v1')`)
    return err
}
```

- [ ] **Step 3: Register migrations in `runMigrations`**

Find the `runMigrations` function. Add the two new calls at the end:

```go
func runMigrations() error {
    if err := runLegacyColumnMigrations(); err != nil {
        return err
    }
    if err := runAccountsV1Migration(); err != nil {
        return err
    }
    if err := runExpenseSplitsV1Migration(); err != nil {
        return err
    }
    if err := runRecurringExpensesV1Migration(); err != nil {
        return err
    }
    if err := runAccountCustomizationV1Migration(); err != nil {
        return err
    }
    if err := runSplittingV1Migration(); err != nil {
        return err
    }
    if err := runRecurringIncomesV1Migration(); err != nil {
        return err
    }
    return runRecurringPaymentsV1Migration()
}
```

- [ ] **Step 4: Verify build**

```bash
go build ./src/...
```

Expected: no output (clean build).

- [ ] **Step 5: Commit**

```bash
git add src/db/db.go
git commit -m "feat: add recurring_incomes and recurring_payments DB tables + migrations"
```

---

## Task 2: Models

**Files:**
- Modify: `src/models/models.go`

- [ ] **Step 1: Add `RecurringIncomeID` to `Income` and `RecurringPaymentID` to `SplitPayment`**

```go
type Income struct {
    ID                int64   `json:"id"`
    UserID            int64   `json:"-"`
    AccountID         int64   `json:"-"`
    Amount            float64 `json:"amount"`
    Description       string  `json:"description"`
    Type              string  `json:"type"`
    Date              string  `json:"date"`
    RecurringIncomeID *int64  `json:"recurringIncomeId,omitempty"`
}

type SplitPayment struct {
    ID                 int64   `json:"id"`
    AccountID          int64   `json:"-"`
    PayerUserID        int64   `json:"payerUserId"`
    PayerName          string  `json:"payerName,omitempty"`
    ReceiverUserID     int64   `json:"receiverUserId"`
    ReceiverName       string  `json:"receiverName,omitempty"`
    Amount             float64 `json:"amount"`
    Date               string  `json:"date"`
    Note               string  `json:"note"`
    RecurringPaymentID *int64  `json:"recurringPaymentId,omitempty"`
}
```

- [ ] **Step 2: Add `RecurringIncome` and `RecurringPayment` structs**

Add after the `RecurringExpense` struct:

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
    ID             int64   `json:"id"`
    AccountID      int64   `json:"-"`
    PayerUserID    int64   `json:"payerUserId"`
    PayerName      string  `json:"payerName,omitempty"`
    ReceiverUserID int64   `json:"receiverUserId"`
    ReceiverName   string  `json:"receiverName,omitempty"`
    Amount         float64 `json:"amount"`
    Note           string  `json:"note"`
    DayOfMonth     int     `json:"dayOfMonth"`
    StartDate      string  `json:"startDate"`
    EndDate        string  `json:"endDate"`
    Enabled        bool    `json:"enabled"`
}
```

- [ ] **Step 3: Verify build**

```bash
go build ./src/...
```

Expected: clean build.

- [ ] **Step 4: Commit**

```bash
git add src/models/models.go
git commit -m "feat: add RecurringIncome and RecurringPayment models"
```

---

## Task 3: Validation

**Files:**
- Modify: `src/handlers/validation.go`

- [ ] **Step 1: Add `validateRecurringIncome`**

Add after `validateRecurringExpense`:

```go
func validateRecurringIncome(ri models.RecurringIncome) error {
    if ri.Amount <= 0 {
        return errors.New("Valor da entrada recorrente deve ser maior que zero")
    }
    if strings.TrimSpace(ri.Description) == "" {
        return errors.New("Descrição é obrigatória")
    }
    if strings.TrimSpace(ri.Type) == "" {
        return errors.New("Tipo é obrigatório")
    }
    if ri.DayOfMonth < 1 || ri.DayOfMonth > 31 {
        return errors.New("Dia do mês deve estar entre 1 e 31")
    }
    if !isValidDate(ri.StartDate) {
        return errors.New("Data inicial inválida")
    }
    if strings.TrimSpace(ri.EndDate) != "" {
        if !isValidDate(ri.EndDate) {
            return errors.New("Data limite inválida")
        }
        startDate, _ := time.Parse("2006-01-02", ri.StartDate)
        endDate, _ := time.Parse("2006-01-02", ri.EndDate)
        if endDate.Before(startDate) {
            return errors.New("Data limite deve ser igual ou posterior à data inicial")
        }
    }
    return nil
}
```

- [ ] **Step 2: Add `validateRecurringPayment`**

```go
func validateRecurringPayment(accountID int64, rp models.RecurringPayment) error {
    if rp.Amount <= 0 {
        return errAmountPositive
    }
    if rp.PayerUserID <= 0 || rp.ReceiverUserID <= 0 || rp.PayerUserID == rp.ReceiverUserID {
        return errInvalidSplitPaymentMembers
    }
    if rp.DayOfMonth < 1 || rp.DayOfMonth > 31 {
        return errors.New("Dia do mês deve estar entre 1 e 31")
    }
    if !isValidDate(rp.StartDate) {
        return errors.New("Data inicial inválida")
    }
    if strings.TrimSpace(rp.EndDate) != "" {
        if !isValidDate(rp.EndDate) {
            return errors.New("Data limite inválida")
        }
        startDate, _ := time.Parse("2006-01-02", rp.StartDate)
        endDate, _ := time.Parse("2006-01-02", rp.EndDate)
        if endDate.Before(startDate) {
            return errors.New("Data limite deve ser igual ou posterior à data inicial")
        }
    }
    members, err := accountMemberSplits(accountID)
    if err != nil {
        return err
    }
    foundPayer, foundReceiver := false, false
    for _, m := range members {
        if m.UserID == rp.PayerUserID {
            foundPayer = true
        }
        if m.UserID == rp.ReceiverUserID {
            foundReceiver = true
        }
    }
    if !foundPayer || !foundReceiver {
        return errInvalidSplitPaymentMembers
    }
    return nil
}
```

- [ ] **Step 3: Verify build**

```bash
go build ./src/...
```

Expected: clean build.

- [ ] **Step 4: Commit**

```bash
git add src/handlers/validation.go
git commit -m "feat: add validateRecurringIncome and validateRecurringPayment"
```

---

## Task 4: Materialization functions

**Files:**
- Modify: `src/db/db.go`

- [ ] **Step 1: Add `MaterializeRecurringIncomes`**

Add after `MaterializeRecurringExpenses` (and its helper `createExpenseFromRecurring`):

```go
func MaterializeRecurringIncomes(accountID int64) error {
    now := time.Now()
    today := now.Format("2006-01-02")

    type entry struct {
        id          int64
        userID      int64
        amount      float64
        description string
        incType     string
        dayOfMonth  int
        startDate   string
        endDate     string
    }

    rows, err := DB.Query(`
        SELECT id, user_id, amount, description, type, day_of_month, start_date, COALESCE(end_date, '')
        FROM recurring_incomes
        WHERE account_id = ? AND enabled = 1
    `, accountID)
    if err != nil {
        return err
    }

    var entries []entry
    for rows.Next() {
        var e entry
        if err := rows.Scan(&e.id, &e.userID, &e.amount, &e.description, &e.incType, &e.dayOfMonth, &e.startDate, &e.endDate); err != nil {
            rows.Close()
            return err
        }
        entries = append(entries, e)
    }
    rows.Close()
    if err := rows.Err(); err != nil {
        return err
    }

    for _, e := range entries {
        startDate, err := time.Parse("2006-01-02", e.startDate)
        if err != nil {
            continue
        }
        cursor := time.Date(startDate.Year(), startDate.Month(), 1, 0, 0, 0, 0, time.UTC)
        limit := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

        for !cursor.After(limit) {
            year := cursor.Year()
            month := cursor.Month()

            lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
            day := e.dayOfMonth
            if day > lastDay {
                day = lastDay
            }

            targetDate := fmt.Sprintf("%04d-%02d-%02d", year, int(month), day)

            if targetDate > today || targetDate < e.startDate || (e.endDate != "" && targetDate > e.endDate) {
                cursor = cursor.AddDate(0, 1, 0)
                continue
            }

            yearMonth := fmt.Sprintf("%04d-%02d-%%", year, int(month))
            var count int
            if err := DB.QueryRow(`
                SELECT COUNT(*) FROM incomes WHERE recurring_income_id = ? AND date LIKE ?
            `, e.id, yearMonth).Scan(&count); err != nil {
                cursor = cursor.AddDate(0, 1, 0)
                continue
            }

            if count == 0 {
                if _, err := DB.Exec(`
                    INSERT INTO incomes (user_id, account_id, amount, description, type, date, recurring_income_id)
                    VALUES (?, ?, ?, ?, ?, ?, ?)
                `, e.userID, accountID, e.amount, e.description, e.incType, targetDate, e.id); err != nil {
                    log.Printf("materialize recurring income %d for %s: %v", e.id, targetDate, err)
                }
            }

            cursor = cursor.AddDate(0, 1, 0)
        }
    }

    return nil
}
```

- [ ] **Step 2: Add `MaterializeRecurringPayments`**

```go
func MaterializeRecurringPayments(accountID int64) error {
    now := time.Now()
    today := now.Format("2006-01-02")

    type entry struct {
        id             int64
        payerUserID    int64
        receiverUserID int64
        amount         float64
        note           string
        dayOfMonth     int
        startDate      string
        endDate        string
    }

    rows, err := DB.Query(`
        SELECT id, payer_user_id, receiver_user_id, amount, note, day_of_month, start_date, COALESCE(end_date, '')
        FROM recurring_payments
        WHERE account_id = ? AND enabled = 1
    `, accountID)
    if err != nil {
        return err
    }

    var entries []entry
    for rows.Next() {
        var e entry
        if err := rows.Scan(&e.id, &e.payerUserID, &e.receiverUserID, &e.amount, &e.note, &e.dayOfMonth, &e.startDate, &e.endDate); err != nil {
            rows.Close()
            return err
        }
        entries = append(entries, e)
    }
    rows.Close()
    if err := rows.Err(); err != nil {
        return err
    }

    for _, e := range entries {
        startDate, err := time.Parse("2006-01-02", e.startDate)
        if err != nil {
            continue
        }
        cursor := time.Date(startDate.Year(), startDate.Month(), 1, 0, 0, 0, 0, time.UTC)
        limit := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

        for !cursor.After(limit) {
            year := cursor.Year()
            month := cursor.Month()

            lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
            day := e.dayOfMonth
            if day > lastDay {
                day = lastDay
            }

            targetDate := fmt.Sprintf("%04d-%02d-%02d", year, int(month), day)

            if targetDate > today || targetDate < e.startDate || (e.endDate != "" && targetDate > e.endDate) {
                cursor = cursor.AddDate(0, 1, 0)
                continue
            }

            yearMonth := fmt.Sprintf("%04d-%02d-%%", year, int(month))
            var count int
            if err := DB.QueryRow(`
                SELECT COUNT(*) FROM split_payments WHERE recurring_payment_id = ? AND date LIKE ?
            `, e.id, yearMonth).Scan(&count); err != nil {
                cursor = cursor.AddDate(0, 1, 0)
                continue
            }

            if count == 0 {
                if _, err := DB.Exec(`
                    INSERT INTO split_payments (account_id, payer_user_id, receiver_user_id, amount, date, note, recurring_payment_id)
                    VALUES (?, ?, ?, ?, ?, ?, ?)
                `, accountID, e.payerUserID, e.receiverUserID, e.amount, targetDate, e.note, e.id); err != nil {
                    log.Printf("materialize recurring payment %d for %s: %v", e.id, targetDate, err)
                }
            }

            cursor = cursor.AddDate(0, 1, 0)
        }
    }

    return nil
}
```

- [ ] **Step 3: Verify build**

```bash
go build ./src/...
```

Expected: clean build.

- [ ] **Step 4: Commit**

```bash
git add src/db/db.go
git commit -m "feat: add MaterializeRecurringIncomes and MaterializeRecurringPayments"
```

---

## Task 5: Handler — recurring_incomes.go

**Files:**
- Create: `src/handlers/recurring_incomes.go`

- [ ] **Step 1: Create the file**

```go
package handlers

import (
	"gastos/src/db"
	"gastos/src/events"
	"gastos/src/middleware"
	"gastos/src/models"
	"net/http"
	"strings"
)

func RecurringIncomes(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	accountID := middleware.AccountIDFromContext(r.Context())

	switch r.Method {
	case http.MethodGet:
		rows, err := db.DB.Query(`
			SELECT id, user_id, account_id, amount, description, type, day_of_month, start_date, COALESCE(end_date, ''), enabled
			FROM recurring_incomes
			WHERE account_id = ?
			ORDER BY enabled DESC, day_of_month ASC, id ASC
		`, accountID)
		if err != nil {
			jsonError(w, "Erro ao buscar entradas recorrentes", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		result := make([]models.RecurringIncome, 0)
		for rows.Next() {
			var ri models.RecurringIncome
			if err := rows.Scan(&ri.ID, &ri.UserID, &ri.AccountID, &ri.Amount, &ri.Description, &ri.Type, &ri.DayOfMonth, &ri.StartDate, &ri.EndDate, &ri.Enabled); err != nil {
				jsonError(w, "Erro ao ler entradas recorrentes", http.StatusInternalServerError)
				return
			}
			result = append(result, ri)
		}
		if err := rows.Err(); err != nil {
			jsonError(w, "Erro ao iterar entradas recorrentes", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, result)

	case http.MethodPost:
		if !requireAccountEdit(w, r) {
			return
		}
		var ri models.RecurringIncome
		if err := decodeJSON(r, &ri); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		ri.UserID = userID
		ri.AccountID = accountID
		ri.Description = strings.TrimSpace(ri.Description)
		ri.Type = strings.TrimSpace(ri.Type)
		ri.EndDate = strings.TrimSpace(ri.EndDate)
		if err := validateRecurringIncome(ri); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		res, err := db.DB.Exec(`
			INSERT INTO recurring_incomes (user_id, account_id, amount, description, type, day_of_month, start_date, end_date, enabled)
			VALUES (?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?)
		`, ri.UserID, ri.AccountID, ri.Amount, ri.Description, ri.Type, ri.DayOfMonth, ri.StartDate, ri.EndDate, ri.Enabled)
		if err != nil {
			jsonError(w, "Erro ao salvar entrada recorrente", http.StatusInternalServerError)
			return
		}
		id, err := res.LastInsertId()
		if err != nil {
			jsonError(w, "Erro ao obter entrada recorrente salva", http.StatusInternalServerError)
			return
		}
		ri.ID = id
		events.Bus.Notify(accountID, userID)
		writeJSON(w, http.StatusCreated, ri)

	case http.MethodPatch:
		if !requireAccountEdit(w, r) {
			return
		}
		id, err := parseIntPathID(r.URL.Path, "/api/recurring-incomes/")
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		var ri models.RecurringIncome
		if err := decodeJSON(r, &ri); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		ri.ID = id
		ri.UserID = userID
		ri.AccountID = accountID
		ri.Description = strings.TrimSpace(ri.Description)
		ri.Type = strings.TrimSpace(ri.Type)
		ri.EndDate = strings.TrimSpace(ri.EndDate)
		if err := validateRecurringIncome(ri); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		res, err := db.DB.Exec(`
			UPDATE recurring_incomes
			SET user_id = ?, amount = ?, description = ?, type = ?, day_of_month = ?, start_date = ?, end_date = NULLIF(?, ''), enabled = ?
			WHERE id = ? AND account_id = ?
		`, ri.UserID, ri.Amount, ri.Description, ri.Type, ri.DayOfMonth, ri.StartDate, ri.EndDate, ri.Enabled, ri.ID, accountID)
		if err != nil {
			jsonError(w, "Erro ao atualizar entrada recorrente", http.StatusInternalServerError)
			return
		}
		rowsAffected, err := res.RowsAffected()
		if err != nil {
			jsonError(w, "Erro ao confirmar atualização", http.StatusInternalServerError)
			return
		}
		if rowsAffected == 0 {
			jsonError(w, "Entrada recorrente não encontrada", http.StatusNotFound)
			return
		}
		events.Bus.Notify(accountID, userID)
		writeJSON(w, http.StatusOK, ri)

	case http.MethodDelete:
		if !requireAccountEdit(w, r) {
			return
		}
		id, err := parseIntPathID(r.URL.Path, "/api/recurring-incomes/")
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		res, err := db.DB.Exec(`DELETE FROM recurring_incomes WHERE id = ? AND account_id = ?`, id, accountID)
		if err != nil {
			jsonError(w, "Erro ao remover entrada recorrente", http.StatusInternalServerError)
			return
		}
		rowsAffected, err := res.RowsAffected()
		if err != nil {
			jsonError(w, "Erro ao confirmar remoção", http.StatusInternalServerError)
			return
		}
		if rowsAffected == 0 {
			jsonError(w, "Entrada recorrente não encontrada", http.StatusNotFound)
			return
		}
		events.Bus.Notify(accountID, userID)
		w.WriteHeader(http.StatusNoContent)

	default:
		w.Header().Set("Allow", "GET, POST, PATCH, DELETE, OPTIONS")
		jsonError(w, "Método não permitido", http.StatusMethodNotAllowed)
	}
}
```

- [ ] **Step 2: Verify build**

```bash
go build ./src/...
```

Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add src/handlers/recurring_incomes.go
git commit -m "feat: add RecurringIncomes CRUD handler"
```

---

## Task 6: Handler — recurring_payments.go

**Files:**
- Create: `src/handlers/recurring_payments.go`

- [ ] **Step 1: Create the file**

```go
package handlers

import (
	"gastos/src/db"
	"gastos/src/events"
	"gastos/src/middleware"
	"gastos/src/models"
	"net/http"
	"strings"
)

func RecurringPayments(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	accountID := middleware.AccountIDFromContext(r.Context())

	switch r.Method {
	case http.MethodGet:
		rows, err := db.DB.Query(`
			SELECT rp.id, rp.account_id, rp.payer_user_id, payer.name, rp.receiver_user_id, receiver.name,
			       rp.amount, rp.note, rp.day_of_month, rp.start_date, COALESCE(rp.end_date, ''), rp.enabled
			FROM recurring_payments rp
			INNER JOIN users payer    ON payer.id    = rp.payer_user_id
			INNER JOIN users receiver ON receiver.id = rp.receiver_user_id
			WHERE rp.account_id = ?
			ORDER BY rp.enabled DESC, rp.day_of_month ASC, rp.id ASC
		`, accountID)
		if err != nil {
			jsonError(w, "Erro ao buscar pagamentos recorrentes", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		result := make([]models.RecurringPayment, 0)
		for rows.Next() {
			var rp models.RecurringPayment
			if err := rows.Scan(&rp.ID, &rp.AccountID, &rp.PayerUserID, &rp.PayerName, &rp.ReceiverUserID, &rp.ReceiverName, &rp.Amount, &rp.Note, &rp.DayOfMonth, &rp.StartDate, &rp.EndDate, &rp.Enabled); err != nil {
				jsonError(w, "Erro ao ler pagamentos recorrentes", http.StatusInternalServerError)
				return
			}
			result = append(result, rp)
		}
		if err := rows.Err(); err != nil {
			jsonError(w, "Erro ao iterar pagamentos recorrentes", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, result)

	case http.MethodPost:
		if !requireAccountEdit(w, r) {
			return
		}
		var rp models.RecurringPayment
		if err := decodeJSON(r, &rp); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		rp.AccountID = accountID
		rp.Note = strings.TrimSpace(rp.Note)
		rp.EndDate = strings.TrimSpace(rp.EndDate)
		if err := validateRecurringPayment(accountID, rp); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		res, err := db.DB.Exec(`
			INSERT INTO recurring_payments (account_id, payer_user_id, receiver_user_id, amount, note, day_of_month, start_date, end_date, enabled)
			VALUES (?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?)
		`, rp.AccountID, rp.PayerUserID, rp.ReceiverUserID, rp.Amount, rp.Note, rp.DayOfMonth, rp.StartDate, rp.EndDate, rp.Enabled)
		if err != nil {
			jsonError(w, "Erro ao salvar pagamento recorrente", http.StatusInternalServerError)
			return
		}
		id, err := res.LastInsertId()
		if err != nil {
			jsonError(w, "Erro ao obter pagamento recorrente salvo", http.StatusInternalServerError)
			return
		}
		rp.ID = id
		rp.PayerName = accountMemberName(accountID, rp.PayerUserID)
		rp.ReceiverName = accountMemberName(accountID, rp.ReceiverUserID)
		events.Bus.Notify(accountID, userID)
		writeJSON(w, http.StatusCreated, rp)

	case http.MethodPatch:
		if !requireAccountEdit(w, r) {
			return
		}
		id, err := parseIntPathID(r.URL.Path, "/api/recurring-payments/")
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		var rp models.RecurringPayment
		if err := decodeJSON(r, &rp); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		rp.ID = id
		rp.AccountID = accountID
		rp.Note = strings.TrimSpace(rp.Note)
		rp.EndDate = strings.TrimSpace(rp.EndDate)
		if err := validateRecurringPayment(accountID, rp); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		res, err := db.DB.Exec(`
			UPDATE recurring_payments
			SET payer_user_id = ?, receiver_user_id = ?, amount = ?, note = ?, day_of_month = ?, start_date = ?, end_date = NULLIF(?, ''), enabled = ?
			WHERE id = ? AND account_id = ?
		`, rp.PayerUserID, rp.ReceiverUserID, rp.Amount, rp.Note, rp.DayOfMonth, rp.StartDate, rp.EndDate, rp.Enabled, rp.ID, accountID)
		if err != nil {
			jsonError(w, "Erro ao atualizar pagamento recorrente", http.StatusInternalServerError)
			return
		}
		rowsAffected, err := res.RowsAffected()
		if err != nil {
			jsonError(w, "Erro ao confirmar atualização", http.StatusInternalServerError)
			return
		}
		if rowsAffected == 0 {
			jsonError(w, "Pagamento recorrente não encontrado", http.StatusNotFound)
			return
		}
		rp.PayerName = accountMemberName(accountID, rp.PayerUserID)
		rp.ReceiverName = accountMemberName(accountID, rp.ReceiverUserID)
		events.Bus.Notify(accountID, userID)
		writeJSON(w, http.StatusOK, rp)

	case http.MethodDelete:
		if !requireAccountEdit(w, r) {
			return
		}
		id, err := parseIntPathID(r.URL.Path, "/api/recurring-payments/")
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		res, err := db.DB.Exec(`DELETE FROM recurring_payments WHERE id = ? AND account_id = ?`, id, accountID)
		if err != nil {
			jsonError(w, "Erro ao remover pagamento recorrente", http.StatusInternalServerError)
			return
		}
		rowsAffected, err := res.RowsAffected()
		if err != nil {
			jsonError(w, "Erro ao confirmar remoção", http.StatusInternalServerError)
			return
		}
		if rowsAffected == 0 {
			jsonError(w, "Pagamento recorrente não encontrado", http.StatusNotFound)
			return
		}
		events.Bus.Notify(accountID, userID)
		w.WriteHeader(http.StatusNoContent)

	default:
		w.Header().Set("Allow", "GET, POST, PATCH, DELETE, OPTIONS")
		jsonError(w, "Método não permitido", http.StatusMethodNotAllowed)
	}
}
```

- [ ] **Step 2: Verify build**

```bash
go build ./src/...
```

Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add src/handlers/recurring_payments.go
git commit -m "feat: add RecurringPayments CRUD handler"
```

---

## Task 7: Update incomes.go and split_payments.go GET handlers

**Files:**
- Modify: `src/handlers/incomes.go`
- Modify: `src/handlers/split_payments.go`

- [ ] **Step 1: Update `incomes.go` GET — call materialization and expose `recurring_income_id`**

At the top of the `case http.MethodGet:` block in `Incomes`, add the materialization call:

```go
case http.MethodGet:
    _ = db.MaterializeRecurringIncomes(accountID)

    rows, err := db.DB.Query(`
        SELECT id, user_id, account_id, amount, description, type, date, recurring_income_id
        FROM incomes
        WHERE account_id = ?
        ORDER BY date DESC, id DESC
    `, accountID)
```

Update the `rows.Scan` call to include `&income.RecurringIncomeID`:

```go
if err := rows.Scan(
    &income.ID,
    &income.UserID,
    &income.AccountID,
    &income.Amount,
    &income.Description,
    &income.Type,
    &income.Date,
    &income.RecurringIncomeID,
); err != nil {
```

- [ ] **Step 2: Update `split_payments.go` GET — call materialization and expose `recurring_payment_id`**

At the top of the `case http.MethodGet:` block in `SplitPayments`:

```go
case http.MethodGet:
    _ = db.MaterializeRecurringPayments(accountID)

    rows, err := db.DB.Query(`
        SELECT sp.id, sp.account_id, sp.payer_user_id, payer.name, sp.receiver_user_id, receiver.name,
               sp.amount, sp.date, sp.note, sp.recurring_payment_id
        FROM split_payments sp
        INNER JOIN users payer    ON payer.id    = sp.payer_user_id
        INNER JOIN users receiver ON receiver.id = sp.receiver_user_id
        WHERE sp.account_id = ?
        ORDER BY sp.date DESC, sp.id DESC
    `, accountID)
```

Update the `rows.Scan` call:

```go
if err := rows.Scan(
    &payment.ID,
    &payment.AccountID,
    &payment.PayerUserID,
    &payment.PayerName,
    &payment.ReceiverUserID,
    &payment.ReceiverName,
    &payment.Amount,
    &payment.Date,
    &payment.Note,
    &payment.RecurringPaymentID,
); err != nil {
```

- [ ] **Step 3: Verify build**

```bash
go build ./src/...
```

Expected: clean build.

- [ ] **Step 4: Commit**

```bash
git add src/handlers/incomes.go src/handlers/split_payments.go
git commit -m "feat: materialize recurring incomes/payments on GET, expose FK fields"
```

---

## Task 8: Routes in main.go

**Files:**
- Modify: `src/main.go`

- [ ] **Step 1: Register the four new route pairs**

Find the block of account-scoped routes in `newMux()` and add before the `// Eventos em tempo real` comment:

```go
mux.HandleFunc("/api/recurring-incomes",    cors(middleware.Auth(middleware.Account(handlers.RecurringIncomes))))
mux.HandleFunc("/api/recurring-incomes/",   cors(middleware.Auth(middleware.Account(handlers.RecurringIncomes))))
mux.HandleFunc("/api/recurring-payments",   cors(middleware.Auth(middleware.Account(handlers.RecurringPayments))))
mux.HandleFunc("/api/recurring-payments/",  cors(middleware.Auth(middleware.Account(handlers.RecurringPayments))))
```

- [ ] **Step 2: Verify build**

```bash
go build ./src/...
```

Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add src/main.go
git commit -m "feat: register /api/recurring-incomes and /api/recurring-payments routes"
```

---

## Task 9: Frontend — recurring incomes

**Files:**
- Modify: `src/web/index.html`

- [ ] **Step 1: Extend state — `recurringIncomes`, `editingRecurringIncomeInForm`, `newInc` fields**

Find `recurringExpenses: [],` in the data object (around line 1269). Add below it:

```js
recurringIncomes: [],
recurringPayments: [],
editingRecurringIncomeInForm: '',
editingRecurringPaymentInForm: '',
```

Find `newInc: { amount: '0,00', description: '', type: 'salary', date: '' },` and replace with:

```js
newInc: { amount: '0,00', description: '', type: 'salary', date: '', isRecurring: false, dayOfMonth: 1, startDate: '', endDate: '', enabled: true },
```

- [ ] **Step 2: Add to `loadAllData` — fetch recurring-incomes and recurring-payments**

Find the `const [expenses, incomes, ...]` destructuring in `loadAllData`. Replace it with:

```js
const [expenses, incomes, goals, recurringExpenses, splitPayments, categories, paymentMethods, recurringIncomes, recurringPayments] = await Promise.all([
    API.get('/api/expenses'),
    API.get('/api/incomes'),
    API.get('/api/goals'),
    API.get('/api/recurring-expenses'),
    API.get('/api/split-payments'),
    API.get('/api/categories'),
    API.get('/api/payment-methods'),
    API.get('/api/recurring-incomes'),
    API.get('/api/recurring-payments'),
])
this.expenses = expenses || []
this.incomes = incomes || []
this.goals = goals || []
this.recurringExpenses = recurringExpenses || []
this.splitPayments = splitPayments || []
this.categories = categories || []
this.paymentMethods = paymentMethods || []
this.recurringIncomes = recurringIncomes || []
this.recurringPayments = recurringPayments || []
```

Also in the `if (!this.hasActiveAccount)` block at the top of `loadAllData`, add:

```js
this.recurringIncomes = []
this.recurringPayments = []
```

- [ ] **Step 3: Update `resetIncomeForm`**

Replace the existing `resetIncomeForm` body:

```js
resetIncomeForm() {
    this.newInc.amount = '0,00'
    this.newInc.description = ''
    this.newInc.type = 'salary'
    this.newInc.isRecurring = false
    this.newInc.dayOfMonth = 1
    this.newInc.startDate = this.todayDate()
    this.newInc.endDate = ''
    this.newInc.enabled = true
    this.$nextTick(() => this.syncDatePickers())
},
```

- [ ] **Step 4: Update `cancelIncomeEdit`**

Replace the existing `cancelIncomeEdit` body:

```js
cancelIncomeEdit() {
    this.editingIncomeId = ''
    this.editingRecurringIncomeInForm = ''
    this.newInc.isRecurring = false
    this.resetIncomeForm()
},
```

- [ ] **Step 5: Update `submitIncome` to branch on `isRecurring`**

Replace the entire `submitIncome` method:

```js
async submitIncome() {
    if (!this.canEditActiveAccount || !this.hasActiveAccount) {
        this.toast('Você não pode editar esta conta.', true)
        return
    }
    const amount = this.parseCurrencyInput(this.newInc.amount)
    if (amount <= 0) {
        this.toast('Informe um valor válido.', true)
        return
    }
    this.savingIncome = true
    try {
        if (this.newInc.isRecurring) {
            const payload = {
                amount,
                description: this.newInc.description.trim(),
                type: this.newInc.type,
                dayOfMonth: this.newInc.dayOfMonth,
                startDate: this.newInc.startDate || this.todayDate(),
                endDate: this.newInc.endDate || '',
                enabled: this.newInc.enabled,
            }
            const ri = this.editingRecurringIncomeInForm
                ? await API.patch('/api/recurring-incomes/' + this.editingRecurringIncomeInForm, payload)
                : await API.post('/api/recurring-incomes', payload)
            if (this.editingRecurringIncomeInForm) {
                this.recurringIncomes = this.recurringIncomes.map(item => item.id === ri.id ? ri : item)
                this.toast('Entrada recorrente atualizada ✓')
            } else {
                this.recurringIncomes.push(ri)
                this.toast('Entrada recorrente configurada ✓')
            }
            this.editingRecurringIncomeInForm = ''
            this.resetIncomeForm()
        } else {
            const payload = {
                amount,
                description: this.newInc.description.trim(),
                type: this.newInc.type,
                date: this.newInc.date || this.todayDate(),
            }
            const inc = this.editingIncomeId
                ? await API.patch('/api/incomes/' + this.editingIncomeId, payload)
                : await API.post('/api/incomes', payload)
            if (this.editingIncomeId) {
                this.incomes = this.incomes.map(item => item.id === inc.id ? inc : item)
                this.toast('Entrada atualizada ✓')
            } else {
                this.incomes.push(inc)
                this.toast('Entrada adicionada ✓')
            }
            this.editingIncomeId = ''
            this.resetIncomeForm()
        }
    } catch (err) {
        this.toast(err.message, true)
    } finally {
        this.savingIncome = false
    }
},
```

- [ ] **Step 6: Add `startEditRecurringIncome` and `deleteRecurringIncome`**

Add after the existing `deleteIncome` method:

```js
startEditRecurringIncome(item) {
    this.editingRecurringIncomeInForm = String(item.id)
    this.newInc.isRecurring = true
    this.newInc.amount = this.moneyToInput(item.amount)
    this.newInc.description = item.description
    this.newInc.type = item.type
    this.newInc.dayOfMonth = item.dayOfMonth
    this.newInc.startDate = item.startDate
    this.newInc.endDate = item.endDate || ''
    this.newInc.enabled = item.enabled
    this.$nextTick(() => this.syncDatePickers())
},

async deleteRecurringIncome(id) {
    if (!this.canEditActiveAccount || !this.hasActiveAccount) return
    try {
        await API.delete('/api/recurring-incomes/' + id)
        this.recurringIncomes = this.recurringIncomes.filter(item => item.id !== id)
        if (String(id) === this.editingRecurringIncomeInForm) this.cancelIncomeEdit()
        this.toast('Entrada recorrente removida.')
    } catch (err) {
        this.toast(err.message, true)
    }
},
```

- [ ] **Step 7: Update `doLogout` to clear new state**

In `doLogout`, after `this.recurringExpenses = []`, add:

```js
this.recurringIncomes = []
this.recurringPayments = []
this.editingRecurringIncomeInForm = ''
this.editingRecurringPaymentInForm = ''
```

- [ ] **Step 8: Replace the income form HTML**

Find the income form card (around line 706–718):

```html
<div class="card" x-show="canEditActiveAccount">
  <div class="card-title" x-text="editingIncomeId ? 'Editar entrada' : 'Adicionar entrada'"></div>
  <div class="form-grid" style="grid-template-columns:110px 1fr 160px 160px auto">
    <div class="field"><label>Valor (R$)</label><input class="money-input" type="text" inputmode="numeric" placeholder="0,00" :value="newInc.amount" @input="newInc.amount=formatCurrencyInput($event.target.value)" @keyup.enter="submitIncome()" /></div>
    <div class="field"><label>Descrição</label><input type="text" placeholder="salário, freelance..." x-model="newInc.description" @keyup.enter="submitIncome()" /></div>
    <div class="field"><label>Tipo</label><select x-model="newInc.type"><option value="salary">💼 Salário</option><option value="freelance">💻 Freelance</option><option value="investment">📈 Rendimentos</option><option value="gift">🎁 Presente</option><option value="other">◈ Outros</option></select></div>
    <div class="field"><label>Data</label><input class="date-picker" type="text" x-model="newInc.date" x-init="bindDatePicker($el, newInc, 'date')" readonly /></div>
    <div class="field"><label>&nbsp;</label><button class="btn btn-income" @click="submitIncome()" :disabled="savingIncome" x-text="savingIncome?'...':(editingIncomeId?'Salvar':'Adicionar')"></button></div>
  </div>
  <div class="panel-actions" x-show="editingIncomeId" style="margin-top:12px">
    <button class="btn btn-ghost" @click="cancelIncomeEdit()">Cancelar edição</button>
  </div>
</div>
```

Replace with:

```html
<div class="card" x-show="canEditActiveAccount">
  <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:14px">
    <div class="card-title" style="margin-bottom:0" x-text="editingIncomeId?'Editar entrada':(editingRecurringIncomeInForm?'Editar entrada recorrente':(newInc.isRecurring?'Nova entrada recorrente':'Adicionar entrada'))"></div>
    <div style="display:flex;align-items:center;gap:8px">
      <span style="font-size:11px;color:var(--text2);font-weight:500">Recorrente?</span>
      <button class="btn btn-ghost btn-sm" :class="{active:newInc.isRecurring}" @click="newInc.isRecurring=!newInc.isRecurring" x-text="newInc.isRecurring?'Sim':'Não'" :disabled="!!editingIncomeId" style="min-width:42px"></button>
    </div>
  </div>
  <div class="form-grid" style="grid-template-columns:110px 1fr 160px">
    <div class="field"><label>Valor (R$)</label><input class="money-input" type="text" inputmode="numeric" placeholder="0,00" :value="newInc.amount" @input="newInc.amount=formatCurrencyInput($event.target.value)" @keyup.enter="submitIncome()" /></div>
    <div class="field"><label>Descrição</label><input type="text" placeholder="salário, freelance..." x-model="newInc.description" @keyup.enter="submitIncome()" /></div>
    <div class="field"><label>Tipo</label><select x-model="newInc.type"><option value="salary">💼 Salário</option><option value="freelance">💻 Freelance</option><option value="investment">📈 Rendimentos</option><option value="gift">🎁 Presente</option><option value="other">◈ Outros</option></select></div>
  </div>
  <div x-show="!newInc.isRecurring" class="form-grid" style="grid-template-columns:160px auto;margin-top:10px">
    <div class="field"><label>Data</label><input class="date-picker" type="text" x-model="newInc.date" x-init="bindDatePicker($el, newInc, 'date')" readonly /></div>
    <div class="field"><label>&nbsp;</label><button class="btn btn-income" @click="submitIncome()" :disabled="savingIncome" x-text="savingIncome?'...':(editingIncomeId?'Salvar':'Adicionar')" style="align-self:flex-end"></button></div>
  </div>
  <div x-show="newInc.isRecurring" class="form-grid" style="grid-template-columns:110px 160px 160px auto auto;margin-top:10px">
    <div class="field"><label>Dia do mês</label><input type="number" min="1" max="31" x-model.number="newInc.dayOfMonth" @keyup.enter="submitIncome()" /></div>
    <div class="field"><label>Início</label><input class="date-picker" type="text" x-model="newInc.startDate" x-init="bindDatePicker($el, newInc, 'startDate')" readonly /></div>
    <div class="field"><label>Limite</label><input class="date-picker" type="text" x-model="newInc.endDate" x-init="bindDatePicker($el, newInc, 'endDate', { allowInput: false, allowClear: true })" readonly /></div>
    <div class="field"><label>&nbsp;</label><button class="btn btn-ghost" @click="newInc.enabled=!newInc.enabled" x-text="newInc.enabled?'Ativa':'Pausada'" style="align-self:flex-end;height:37px"></button></div>
    <div class="field"><label>&nbsp;</label><button class="btn btn-income" @click="submitIncome()" :disabled="savingIncome" x-text="savingIncome?'...':(editingRecurringIncomeInForm?'Salvar':'Configurar')" style="align-self:flex-end"></button></div>
  </div>
  <div class="panel-actions" x-show="editingIncomeId || editingRecurringIncomeInForm" style="margin-top:12px">
    <button class="btn btn-ghost" @click="cancelIncomeEdit()">Cancelar edição</button>
  </div>
</div>
```

- [ ] **Step 9: Add `↻ recorrente` badge to income list items**

Find the income tx-row2 (around line 735):

```html
<div class="tx-row2"><span class="tx-meta" x-text="incTypeLabel(tx.type)"></span></div>
```

Replace with:

```html
<div class="tx-row2"><span class="tx-meta" x-text="incTypeLabel(tx.type)"></span><span class="pay-badge" x-show="tx.recurringIncomeId" style="background:var(--accent-dim);color:var(--accent)">↻ recorrente</span></div>
```

- [ ] **Step 10: Add recurring incomes template card below the income tx-list**

Find the closing `</div>` of the income `tx-list` div (after the grouped income template, around line 748). Insert after it and before the closing `</div>` of the income page:

```html
<div class="card" x-show="canEditActiveAccount && recurringIncomes.length > 0">
  <div class="card-title">Entradas recorrentes</div>
  <div class="recurring-grid">
    <template x-for="item in recurringIncomes" :key="'ri-'+item.id">
      <div class="recurring-card">
        <div class="recurring-top">
          <div>
            <div class="recurring-title" x-text="item.description || incTypeLabel(item.type)"></div>
            <div class="recurring-value" style="color:var(--accent)" x-text="'+ '+fmt(item.amount)"></div>
          </div>
          <div class="panel-actions">
            <button class="btn btn-ghost btn-sm" @click="startEditRecurringIncome(item)">editar</button>
            <button class="btn btn-ghost btn-sm" @click="deleteRecurringIncome(item.id)">remover</button>
          </div>
        </div>
        <div class="recurring-meta-line" x-text="'Todo dia '+item.dayOfMonth+' · '+incTypeLabel(item.type)"></div>
        <div class="recurring-meta-line" x-text="'Início: '+formatDateLong(item.startDate)+(item.endDate ? ' · até '+formatDateLong(item.endDate) : ' · sem limite')"></div>
        <div class="recurring-meta-line"><span :class="item.enabled?'goal-status ok':'goal-status warn'" x-text="item.enabled?'✓ Ativa':'⏸ Pausada'"></span></div>
      </div>
    </template>
  </div>
</div>
```

- [ ] **Step 11: Verify build and quick visual check**

```bash
go build ./src/... && go run ./src/main.go &
```

Open browser at `http://localhost:8000`. Log in, go to Entradas, confirm:
- "Recorrente?" toggle appears
- Toggling shows/hides date vs dayOfMonth+startDate+endDate fields
- Creating a recurring income saves and shows the template card
- Materialized income appears on next GET with `↻ recorrente` badge

Kill the dev server: `pkill -f "go run ./src/main.go"` (or use `fg` + Ctrl-C).

- [ ] **Step 12: Commit**

```bash
git add src/web/index.html
git commit -m "feat: recurring incomes UI — toggle, template card, recorrente badge"
```

---

## Task 10: Frontend — recurring payments

**Files:**
- Modify: `src/web/index.html`

- [ ] **Step 1: Extend `newSplitPayment` state**

Find `newSplitPayment: { ... }` in the data object and add the recurring fields:

```js
newSplitPayment: { payerUserId: '', receiverUserId: '', amount: '0,00', date: '', note: '', isRecurring: false, dayOfMonth: 1, startDate: '', endDate: '', enabled: true },
```

- [ ] **Step 2: Update `resetSplitPaymentForm`**

Replace the existing body:

```js
resetSplitPaymentForm() {
    this.editingSplitPaymentId = ''
    this.editingRecurringPaymentInForm = ''
    this.newSplitPayment.payerUserId = this.accountMembers[0]?.userId || ''
    this.newSplitPayment.receiverUserId = this.accountMembers[1]?.userId || this.accountMembers[0]?.userId || ''
    this.newSplitPayment.amount = '0,00'
    this.newSplitPayment.date = this.todayDate()
    this.newSplitPayment.note = ''
    this.newSplitPayment.isRecurring = false
    this.newSplitPayment.dayOfMonth = 1
    this.newSplitPayment.startDate = this.todayDate()
    this.newSplitPayment.endDate = ''
    this.newSplitPayment.enabled = true
    this.$nextTick(() => this.syncDatePickers())
},
```

- [ ] **Step 3: Update `cancelSplitPaymentEdit`**

```js
cancelSplitPaymentEdit() {
    this.editingRecurringPaymentInForm = ''
    this.newSplitPayment.isRecurring = false
    this.resetSplitPaymentForm()
},
```

- [ ] **Step 4: Update `submitSplitPayment` to branch on `isRecurring`**

Replace the entire `submitSplitPayment` method:

```js
async submitSplitPayment() {
    if (!this.canEditActiveAccount || !this.hasActiveAccount) {
        this.toast('Você não pode editar esta conta.', true)
        return
    }
    const amount = this.parseCurrencyInput(this.newSplitPayment.amount)
    if (amount <= 0) {
        this.toast('Informe um valor válido.', true)
        return
    }
    if (!this.newSplitPayment.payerUserId || !this.newSplitPayment.receiverUserId || this.newSplitPayment.payerUserId === this.newSplitPayment.receiverUserId) {
        this.toast('Escolha participantes diferentes.', true)
        return
    }
    this.savingSplitPayment = true
    try {
        if (this.newSplitPayment.isRecurring) {
            const payload = {
                payerUserId: Number(this.newSplitPayment.payerUserId),
                receiverUserId: Number(this.newSplitPayment.receiverUserId),
                amount,
                note: this.newSplitPayment.note.trim(),
                dayOfMonth: this.newSplitPayment.dayOfMonth,
                startDate: this.newSplitPayment.startDate || this.todayDate(),
                endDate: this.newSplitPayment.endDate || '',
                enabled: this.newSplitPayment.enabled,
            }
            const rp = this.editingRecurringPaymentInForm
                ? await API.patch('/api/recurring-payments/' + this.editingRecurringPaymentInForm, payload)
                : await API.post('/api/recurring-payments', payload)
            if (this.editingRecurringPaymentInForm) {
                this.recurringPayments = this.recurringPayments.map(item => item.id === rp.id ? rp : item)
                this.toast('Pagamento recorrente atualizado ✓')
            } else {
                this.recurringPayments.push(rp)
                this.toast('Pagamento recorrente configurado ✓')
            }
            this.resetSplitPaymentForm()
        } else {
            const payload = {
                payerUserId: Number(this.newSplitPayment.payerUserId),
                receiverUserId: Number(this.newSplitPayment.receiverUserId),
                amount,
                date: this.newSplitPayment.date || this.todayDate(),
                note: this.newSplitPayment.note.trim(),
            }
            const payment = this.editingSplitPaymentId
                ? await API.patch('/api/split-payments/' + this.editingSplitPaymentId, payload)
                : await API.post('/api/split-payments', payload)
            if (this.editingSplitPaymentId) {
                this.splitPayments = this.splitPayments.map(item => item.id === payment.id ? payment : item)
                this.toast('Pagamento atualizado ✓')
            } else {
                this.splitPayments.push(payment)
                this.toast('Pagamento registrado ✓')
            }
            this.resetSplitPaymentForm()
        }
    } catch (err) {
        this.toast(err.message, true)
    } finally {
        this.savingSplitPayment = false
    }
},
```

- [ ] **Step 5: Add `startEditRecurringPayment` and `deleteRecurringPayment`**

Add after the existing `deleteSplitPayment` method:

```js
startEditRecurringPayment(item) {
    this.editingRecurringPaymentInForm = String(item.id)
    this.newSplitPayment.isRecurring = true
    this.newSplitPayment.payerUserId = item.payerUserId
    this.newSplitPayment.receiverUserId = item.receiverUserId
    this.newSplitPayment.amount = this.moneyToInput(item.amount)
    this.newSplitPayment.note = item.note || ''
    this.newSplitPayment.dayOfMonth = item.dayOfMonth
    this.newSplitPayment.startDate = item.startDate
    this.newSplitPayment.endDate = item.endDate || ''
    this.newSplitPayment.enabled = item.enabled
    this.$nextTick(() => {
        this.syncDatePickers()
        this.$refs.splitPaymentForm?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    })
},

async deleteRecurringPayment(id) {
    if (!this.canEditActiveAccount || !this.hasActiveAccount) return
    try {
        await API.delete('/api/recurring-payments/' + id)
        this.recurringPayments = this.recurringPayments.filter(item => item.id !== id)
        if (String(id) === this.editingRecurringPaymentInForm) this.cancelSplitPaymentEdit()
        this.toast('Pagamento recorrente removido.')
    } catch (err) {
        this.toast(err.message, true)
    }
},
```

- [ ] **Step 6: Replace the payment form HTML**

Find the split payment form card (around line 778–791):

```html
<div class="card" x-show="canEditActiveAccount" x-ref="splitPaymentForm">
  <div class="card-title" x-text="editingSplitPaymentId ? 'Editar pagamento' : 'Registrar pagamento'"></div>
  <div class="form-grid" style="grid-template-columns:1fr 1fr 130px 160px 1fr auto">
    <div class="field"><label>Quem pagou</label>...</div>
    <div class="field"><label>Recebeu</label>...</div>
    <div class="field"><label>Valor (R$)</label>...</div>
    <div class="field"><label>Data</label>...</div>
    <div class="field"><label>Nota</label>...</div>
    <div class="field"><label>&nbsp;</label><button ...>Registrar</button></div>
  </div>
  <div class="panel-actions" x-show="editingSplitPaymentId" ...>
    <button @click="cancelSplitPaymentEdit()">Cancelar edição</button>
  </div>
</div>
```

Replace with:

```html
<div class="card" x-show="canEditActiveAccount" x-ref="splitPaymentForm">
  <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:14px">
    <div class="card-title" style="margin-bottom:0" x-text="editingSplitPaymentId?'Editar pagamento':(editingRecurringPaymentInForm?'Editar pagamento recorrente':(newSplitPayment.isRecurring?'Novo pagamento recorrente':'Registrar pagamento'))"></div>
    <div style="display:flex;align-items:center;gap:8px">
      <span style="font-size:11px;color:var(--text2);font-weight:500">Recorrente?</span>
      <button class="btn btn-ghost btn-sm" :class="{active:newSplitPayment.isRecurring}" @click="newSplitPayment.isRecurring=!newSplitPayment.isRecurring" x-text="newSplitPayment.isRecurring?'Sim':'Não'" :disabled="!!editingSplitPaymentId" style="min-width:42px"></button>
    </div>
  </div>
  <div class="form-grid" style="grid-template-columns:1fr 1fr 130px 1fr">
    <div class="field"><label>Quem pagou</label><select x-model.number="newSplitPayment.payerUserId"><template x-for="member in accountMembers" :key="'payer-'+member.userId"><option :value="member.userId" x-text="member.name"></option></template></select></div>
    <div class="field"><label>Recebeu</label><select x-model.number="newSplitPayment.receiverUserId"><template x-for="member in accountMembers" :key="'receiver-'+member.userId"><option :value="member.userId" x-text="member.name"></option></template></select></div>
    <div class="field"><label>Valor (R$)</label><input class="money-input" type="text" inputmode="numeric" :value="newSplitPayment.amount" @input="newSplitPayment.amount=formatCurrencyInput($event.target.value)" @keyup.enter="submitSplitPayment()" /></div>
    <div class="field"><label>Nota</label><input type="text" placeholder="pix, transferência..." x-model="newSplitPayment.note" @keyup.enter="submitSplitPayment()" /></div>
  </div>
  <div x-show="!newSplitPayment.isRecurring" class="form-grid" style="grid-template-columns:160px auto;margin-top:10px">
    <div class="field"><label>Data</label><input class="date-picker" type="text" x-model="newSplitPayment.date" x-init="bindDatePicker($el, newSplitPayment, 'date')" readonly /></div>
    <div class="field"><label>&nbsp;</label><button class="btn btn-primary" @click="submitSplitPayment()" :disabled="savingSplitPayment" x-text="savingSplitPayment?'...':(editingSplitPaymentId?'Salvar':'Registrar')" style="align-self:flex-end"></button></div>
  </div>
  <div x-show="newSplitPayment.isRecurring" class="form-grid" style="grid-template-columns:110px 160px 160px auto auto;margin-top:10px">
    <div class="field"><label>Dia do mês</label><input type="number" min="1" max="31" x-model.number="newSplitPayment.dayOfMonth" @keyup.enter="submitSplitPayment()" /></div>
    <div class="field"><label>Início</label><input class="date-picker" type="text" x-model="newSplitPayment.startDate" x-init="bindDatePicker($el, newSplitPayment, 'startDate')" readonly /></div>
    <div class="field"><label>Limite</label><input class="date-picker" type="text" x-model="newSplitPayment.endDate" x-init="bindDatePicker($el, newSplitPayment, 'endDate', { allowInput: false, allowClear: true })" readonly /></div>
    <div class="field"><label>&nbsp;</label><button class="btn btn-ghost" @click="newSplitPayment.enabled=!newSplitPayment.enabled" x-text="newSplitPayment.enabled?'Ativo':'Pausado'" style="align-self:flex-end;height:37px"></button></div>
    <div class="field"><label>&nbsp;</label><button class="btn btn-primary" @click="submitSplitPayment()" :disabled="savingSplitPayment" x-text="savingSplitPayment?'...':(editingRecurringPaymentInForm?'Salvar':'Configurar')" style="align-self:flex-end"></button></div>
  </div>
  <div class="panel-actions" x-show="editingSplitPaymentId || editingRecurringPaymentInForm" style="margin-top:12px">
    <button class="btn btn-ghost" @click="cancelSplitPaymentEdit()">Cancelar edição</button>
  </div>
</div>
```

- [ ] **Step 7: Add `↻ recorrente` badge to payment list items**

Find the payment tx-row2 (around line 807):

```html
<div class="tx-row2"><span class="tx-meta" x-text="formatDate(payment.date)"></span><span class="pay-badge" x-text="payment.note || 'pagamento do racha'"></span></div>
```

Replace with:

```html
<div class="tx-row2"><span class="tx-meta" x-text="formatDate(payment.date)"></span><span class="pay-badge" x-text="payment.note || 'pagamento do racha'"></span><span class="pay-badge" x-show="payment.recurringPaymentId" style="background:var(--accent-dim);color:var(--accent)">↻ recorrente</span></div>
```

- [ ] **Step 8: Add recurring payments template card below the payment tx-list**

Find the closing `</div>` of the payments page (after the tx-list, around line 819). Insert the template card before the `<!-- METAS -->` comment:

```html
<div class="card" x-show="canEditActiveAccount && recurringPayments.length > 0">
  <div class="card-title">Pagamentos recorrentes</div>
  <div class="recurring-grid">
    <template x-for="item in recurringPayments" :key="'rp-'+item.id">
      <div class="recurring-card">
        <div class="recurring-top">
          <div>
            <div class="recurring-title" x-text="item.payerName+' → '+item.receiverName"></div>
            <div class="recurring-value" x-text="fmt(item.amount)"></div>
          </div>
          <div class="panel-actions">
            <button class="btn btn-ghost btn-sm" @click="startEditRecurringPayment(item)">editar</button>
            <button class="btn btn-ghost btn-sm" @click="deleteRecurringPayment(item.id)">remover</button>
          </div>
        </div>
        <div class="recurring-meta-line" x-text="(item.note || 'pagamento recorrente')+' · Todo dia '+item.dayOfMonth"></div>
        <div class="recurring-meta-line" x-text="'Início: '+formatDateLong(item.startDate)+(item.endDate ? ' · até '+formatDateLong(item.endDate) : ' · sem limite')"></div>
        <div class="recurring-meta-line"><span :class="item.enabled?'goal-status ok':'goal-status warn'" x-text="item.enabled?'✓ Ativo':'⏸ Pausado'"></span></div>
      </div>
    </template>
  </div>
</div>
```

- [ ] **Step 9: Final build and full visual check**

```bash
go build ./src/... && go run ./src/main.go &
```

Check:
1. Pagamentos page — "Recorrente?" toggle appears in the payment form
2. Toggle switches date field to dayOfMonth+startDate+endDate
3. Creating a recurring payment saves and shows template card
4. Template card shows payer → receiver, amount, dates, status badge
5. Materialized payments on next GET show `↻ recorrente` badge
6. Recurring incomes still work (no regression from Task 9)
7. Logout clears all state; re-login restores templates and lists

Kill dev server.

- [ ] **Step 10: Commit**

```bash
git add src/web/index.html
git commit -m "feat: recurring payments UI — toggle, template card, recorrente badge"
```

---

## Task 11: Final push

- [ ] **Step 1: Push all commits**

```bash
git push
```

Expected: all commits from tasks 1–10 pushed to remote.
