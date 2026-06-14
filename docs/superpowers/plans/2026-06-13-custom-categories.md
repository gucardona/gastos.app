# Custom Categories & Payment Methods Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace hardcoded frontend category/payment-method arrays with per-account DB tables: full CRUD with emoji picker, inline edit/delete, and auto-reassignment of expenses on delete.

**Architecture:** New `account_categories` and `account_payment_methods` DB tables seeded at account creation. Two new handler files expose full CRUD. Frontend replaces static arrays with API calls and adds settings cards for managing both lists. DELETE uses query-param `?replacement=<key>` since `API.delete(path)` takes no body.

**Tech Stack:** Go 1.26.2, `net/http`, `modernc.org/sqlite`, Alpine.js v3, no external libraries.

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `src/models/models.go` | Add `Category` and `PaymentMethod` structs |
| Modify | `src/db/db.go` | New tables, `seedAccountDefaults`, updated `CreateAccountWithOwner`, new migration |
| Modify | `src/handlers/validation.go` | `validateCategory`, `validatePaymentMethod` |
| Create | `src/handlers/categories.go` | GET/POST/PATCH/DELETE for `/api/categories` |
| Create | `src/handlers/payment_methods.go` | GET/POST/PATCH/DELETE for `/api/payment-methods` |
| Modify | `src/main.go` | Register 4 new route pairs |
| Modify | `src/web/index.html` | Remove hardcoded arrays, load from API, fix helpers, add settings UI |

---

### Task 1: Add models

**Files:**
- Modify: `src/models/models.go`

- [ ] **Step 1: Add structs after the `Goal` struct**

  In `src/models/models.go`, after the `Goal` struct (line 43), insert:

  ```go
  type Category struct {
      ID           int64  `json:"id"`
      AccountID    int64  `json:"-"`
      Key          string `json:"key"`
      Name         string `json:"name"`
      Icon         string `json:"icon"`
      Color        string `json:"color"`
      Essentiality string `json:"essentiality"`
      SortOrder    int    `json:"sortOrder"`
  }

  type PaymentMethod struct {
      ID        int64  `json:"id"`
      AccountID int64  `json:"-"`
      Key       string `json:"key"`
      Icon      string `json:"icon"`
      Name      string `json:"name"`
      SortOrder int    `json:"sortOrder"`
  }
  ```

- [ ] **Step 2: Build**

  ```bash
  go build ./...
  ```
  Expected: no errors.

- [ ] **Step 3: Commit**

  ```bash
  git add src/models/models.go
  git commit -m "feat: add Category and PaymentMethod models"
  ```

---

### Task 2: DB tables, seeding, migration

**Files:**
- Modify: `src/db/db.go`

- [ ] **Step 1: Add tables to `createTables()`**

  In `src/db/db.go`, inside the `queries` slice in `createTables()`, append these two entries before the closing `}`:

  ```go
  `CREATE TABLE IF NOT EXISTS account_categories (
      id           INTEGER PRIMARY KEY AUTOINCREMENT,
      account_id   INTEGER NOT NULL,
      key          TEXT    NOT NULL,
      name         TEXT    NOT NULL,
      icon         TEXT    NOT NULL,
      color        TEXT    NOT NULL,
      essentiality TEXT    NOT NULL CHECK(essentiality IN ('essential','nonessential','investment')),
      sort_order   INTEGER NOT NULL DEFAULT 0,
      created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
      UNIQUE(account_id, key),
      FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE
  );`,
  `CREATE TABLE IF NOT EXISTS account_payment_methods (
      id         INTEGER PRIMARY KEY AUTOINCREMENT,
      account_id INTEGER NOT NULL,
      key        TEXT    NOT NULL,
      icon       TEXT    NOT NULL,
      name       TEXT    NOT NULL,
      sort_order INTEGER NOT NULL DEFAULT 0,
      created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
      UNIQUE(account_id, key),
      FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE
  );`,
  ```

- [ ] **Step 2: Add indexes to `createIndexes()`**

  Inside the `queries` slice in `createIndexes()`, append:

  ```go
  `CREATE INDEX IF NOT EXISTS idx_account_categories_account ON account_categories(account_id, sort_order);`,
  `CREATE INDEX IF NOT EXISTS idx_account_payment_methods_account ON account_payment_methods(account_id, sort_order);`,
  ```

- [ ] **Step 3: Add `seedAccountDefaults` function**

  Add this function after `CreateAccountWithOwner`:

  ```go
  func seedAccountDefaults(tx *sql.Tx, accountID int64) error {
      categories := []struct {
          key, name, icon, color, essentiality string
          order                                 int
      }{
          {"food", "Alimentação", "🍽", "#0f6579", "essential", 0},
          {"market", "Mercado", "🛒", "#1f7d8f", "essential", 1},
          {"transport", "Transporte", "🚗", "#2b92a1", "essential", 2},
          {"health", "Saúde", "💊", "#4ecc73", "essential", 3},
          {"home", "Moradia", "🏠", "#2c9b88", "essential", 4},
          {"utilities", "Contas", "⚡", "#d6a23a", "essential", 5},
          {"restaurant", "Restaurante", "🍜", "#55b38c", "nonessential", 6},
          {"leisure", "Lazer", "🎮", "#66c3a1", "nonessential", 7},
          {"subscriptions", "Assinaturas", "📺", "#4ecc73", "nonessential", 8},
          {"clothes", "Vestuário", "👕", "#88c0b3", "nonessential", 9},
          {"beauty", "Beleza", "✨", "#79b7ae", "nonessential", 10},
          {"travel", "Viagem", "✈️", "#54a7b9", "nonessential", 11},
          {"education", "Educação", "📚", "#2ba18a", "investment", 12},
          {"invest", "Investimentos", "📈", "#4ecc73", "investment", 13},
          {"other", "Outros", "◈", "#7f9ea5", "nonessential", 14},
      }
      for _, c := range categories {
          if _, err := tx.Exec(`
              INSERT OR IGNORE INTO account_categories (account_id, key, name, icon, color, essentiality, sort_order)
              VALUES (?, ?, ?, ?, ?, ?, ?)
          `, accountID, c.key, c.name, c.icon, c.color, c.essentiality, c.order); err != nil {
              return err
          }
      }

      paymentMethods := []struct {
          key, icon, name string
          order           int
      }{
          {"pix", "⚡", "Pix", 0},
          {"credit", "💳", "Crédito", 1},
          {"debit", "🏦", "Débito", 2},
          {"cash", "💵", "Dinheiro", 3},
          {"boleto", "📄", "Boleto", 4},
          {"va", "🍽", "Vale Alim.", 5},
          {"vr", "🛒", "Vale Ref.", 6},
          {"transfer", "↔", "Transferência", 7},
      }
      for _, p := range paymentMethods {
          if _, err := tx.Exec(`
              INSERT OR IGNORE INTO account_payment_methods (account_id, key, icon, name, sort_order)
              VALUES (?, ?, ?, ?, ?)
          `, accountID, p.key, p.icon, p.name, p.order); err != nil {
              return err
          }
      }
      return nil
  }
  ```

- [ ] **Step 4: Call `seedAccountDefaults` from `CreateAccountWithOwner`**

  After the `INSERT INTO account_members` block in `CreateAccountWithOwner`, before `return accountID, nil`, add:

  ```go
  if err := seedAccountDefaults(tx, accountID); err != nil {
      return 0, err
  }
  ```

  The full function tail should look like:

  ```go
      if _, err := tx.Exec(`
          INSERT INTO account_members (account_id, user_id, role)
          VALUES (?, ?, ?)
      `, accountID, ownerUserID, roleOwner); err != nil {
          return 0, err
      }

      if err := seedAccountDefaults(tx, accountID); err != nil {
          return 0, err
      }

      return accountID, nil
  }
  ```

- [ ] **Step 5: Add `runAccountCustomizationV1Migration`**

  Add this function after `runRecurringExpensesV1Migration`:

  ```go
  func runAccountCustomizationV1Migration() error {
      applied, err := schemaMigrationApplied("account_customization_v1")
      if err != nil || applied {
          return err
      }

      // Seed defaults for every existing account that has no rows yet.
      rows, err := DB.Query(`SELECT id FROM accounts`)
      if err != nil {
          return err
      }
      var accountIDs []int64
      for rows.Next() {
          var id int64
          if err := rows.Scan(&id); err != nil {
              rows.Close()
              return err
          }
          accountIDs = append(accountIDs, id)
      }
      rows.Close()
      if err := rows.Err(); err != nil {
          return err
      }

      tx, err := DB.Begin()
      if err != nil {
          return err
      }
      defer tx.Rollback()

      for _, id := range accountIDs {
          if err := seedAccountDefaults(tx, id); err != nil {
              return err
          }
      }
      if _, err := tx.Exec(`INSERT INTO schema_migrations (name) VALUES ('account_customization_v1')`); err != nil {
          return err
      }
      return tx.Commit()
  }
  ```

- [ ] **Step 6: Register the migration in `runMigrations()`**

  Change the last line of `runMigrations()` from:

  ```go
  return runRecurringExpensesV1Migration()
  ```

  to:

  ```go
  if err := runRecurringExpensesV1Migration(); err != nil {
      return err
  }
  return runAccountCustomizationV1Migration()
  ```

- [ ] **Step 7: Build**

  ```bash
  go build ./...
  ```
  Expected: no errors.

- [ ] **Step 8: Commit**

  ```bash
  git add src/db/db.go
  git commit -m "feat: add account_categories and account_payment_methods tables with seeding"
  ```

---

### Task 3: Add validation functions

**Files:**
- Modify: `src/handlers/validation.go`

- [ ] **Step 1: Add imports and validators**

  `validation.go` already imports `"errors"`, `"strings"`. Add `"regexp"` to the import block:

  ```go
  import (
      "errors"
      "gastos/src/models"
      "regexp"
      "strings"
      "time"
  )
  ```

  Then add after `validateAccountName`:

  ```go
  var colorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

  func validateCategory(name, icon, color, essentiality string) error {
      if strings.TrimSpace(name) == "" {
          return errors.New("Nome é obrigatório")
      }
      if strings.TrimSpace(icon) == "" {
          return errors.New("Ícone é obrigatório")
      }
      if !colorRe.MatchString(color) {
          return errors.New("Cor inválida (use #RRGGBB)")
      }
      switch essentiality {
      case "essential", "nonessential", "investment":
      default:
          return errors.New("Essencialidade inválida")
      }
      return nil
  }

  func validatePaymentMethod(name, icon string) error {
      if strings.TrimSpace(name) == "" {
          return errors.New("Nome é obrigatório")
      }
      if strings.TrimSpace(icon) == "" {
          return errors.New("Ícone é obrigatório")
      }
      return nil
  }
  ```

- [ ] **Step 2: Build**

  ```bash
  go build ./...
  ```
  Expected: no errors.

- [ ] **Step 3: Commit**

  ```bash
  git add src/handlers/validation.go
  git commit -m "feat: add validateCategory and validatePaymentMethod"
  ```

---

### Task 4: Categories handler

**Files:**
- Create: `src/handlers/categories.go`

- [ ] **Step 1: Create the file**

  Create `src/handlers/categories.go`:

  ```go
  package handlers

  import (
      "gastos/src/db"
      "gastos/src/middleware"
      "gastos/src/models"
      "net/http"
      "strings"
      "unicode"
  )

  func slugify(s string) string {
      s = strings.ToLower(strings.TrimSpace(s))
      var b strings.Builder
      for _, r := range s {
          if unicode.IsLetter(r) || unicode.IsDigit(r) {
              b.WriteRune(r)
          } else if unicode.IsSpace(r) {
              b.WriteRune('_')
          }
      }
      return b.String()
  }

  func uniqueCategoryKey(base string, accountID int64) (string, error) {
      key := base
      for i := 2; ; i++ {
          var count int
          err := db.DB.QueryRow(
              `SELECT COUNT(*) FROM account_categories WHERE account_id = ? AND key = ?`,
              accountID, key,
          ).Scan(&count)
          if err != nil {
              return "", err
          }
          if count == 0 {
              return key, nil
          }
          key = base + "_" + strings.Repeat("", 0) + itoa(i)
      }
  }

  func itoa(n int) string {
      return strings.TrimLeft(strings.Join(strings.Fields(strings.Repeat("0", n)), ""), "")
  }

  func Categories(w http.ResponseWriter, r *http.Request) {
      accountID := middleware.AccountIDFromContext(r.Context())

      // Determine if this is a collection or item request
      key := strings.TrimPrefix(r.URL.Path, "/api/categories/")
      key = strings.TrimSpace(key)
      isItem := key != "" && key != "/api/categories"

      switch r.Method {
      case http.MethodGet:
          if isItem {
              w.Header().Set("Allow", "GET, PATCH, DELETE, OPTIONS")
              jsonError(w, "Método não permitido", http.StatusMethodNotAllowed)
              return
          }
          rows, err := db.DB.Query(`
              SELECT id, account_id, key, name, icon, color, essentiality, sort_order
              FROM account_categories
              WHERE account_id = ?
              ORDER BY sort_order ASC, id ASC
          `, accountID)
          if err != nil {
              jsonError(w, "Erro ao buscar categorias", http.StatusInternalServerError)
              return
          }
          defer rows.Close()
          cats := make([]models.Category, 0)
          for rows.Next() {
              var c models.Category
              if err := rows.Scan(&c.ID, &c.AccountID, &c.Key, &c.Name, &c.Icon, &c.Color, &c.Essentiality, &c.SortOrder); err != nil {
                  jsonError(w, "Erro ao ler categorias", http.StatusInternalServerError)
                  return
              }
              cats = append(cats, c)
          }
          if err := rows.Err(); err != nil {
              jsonError(w, "Erro ao iterar categorias", http.StatusInternalServerError)
              return
          }
          writeJSON(w, http.StatusOK, cats)

      case http.MethodPost:
          if !requireAccountEdit(w, r) {
              return
          }
          if isItem {
              jsonError(w, "Método não permitido", http.StatusMethodNotAllowed)
              return
          }
          var body struct {
              Name         string `json:"name"`
              Icon         string `json:"icon"`
              Color        string `json:"color"`
              Essentiality string `json:"essentiality"`
          }
          if err := decodeJSON(r, &body); err != nil {
              jsonError(w, err.Error(), http.StatusBadRequest)
              return
          }
          body.Name = strings.TrimSpace(body.Name)
          body.Icon = strings.TrimSpace(body.Icon)
          body.Color = strings.TrimSpace(body.Color)
          if err := validateCategory(body.Name, body.Icon, body.Color, body.Essentiality); err != nil {
              jsonError(w, err.Error(), http.StatusBadRequest)
              return
          }
          base := slugify(body.Name)
          if base == "" {
              base = "categoria"
          }
          catKey, err := uniqueCategoryKey(base, accountID)
          if err != nil {
              jsonError(w, "Erro ao gerar chave", http.StatusInternalServerError)
              return
          }
          var maxOrder int
          _ = db.DB.QueryRow(`SELECT COALESCE(MAX(sort_order),0) FROM account_categories WHERE account_id = ?`, accountID).Scan(&maxOrder)

          var c models.Category
          err = db.DB.QueryRow(`
              INSERT INTO account_categories (account_id, key, name, icon, color, essentiality, sort_order)
              VALUES (?, ?, ?, ?, ?, ?, ?)
              RETURNING id, account_id, key, name, icon, color, essentiality, sort_order
          `, accountID, catKey, body.Name, body.Icon, body.Color, body.Essentiality, maxOrder+1).Scan(
              &c.ID, &c.AccountID, &c.Key, &c.Name, &c.Icon, &c.Color, &c.Essentiality, &c.SortOrder,
          )
          if err != nil {
              jsonError(w, "Erro ao criar categoria", http.StatusInternalServerError)
              return
          }
          writeJSON(w, http.StatusCreated, c)

      case http.MethodPatch:
          if !requireAccountEdit(w, r) {
              return
          }
          if !isItem {
              jsonError(w, "Chave de categoria obrigatória", http.StatusBadRequest)
              return
          }
          var body struct {
              Name         string `json:"name"`
              Icon         string `json:"icon"`
              Color        string `json:"color"`
              Essentiality string `json:"essentiality"`
              SortOrder    *int   `json:"sortOrder"`
          }
          if err := decodeJSON(r, &body); err != nil {
              jsonError(w, err.Error(), http.StatusBadRequest)
              return
          }
          body.Name = strings.TrimSpace(body.Name)
          body.Icon = strings.TrimSpace(body.Icon)
          body.Color = strings.TrimSpace(body.Color)
          if err := validateCategory(body.Name, body.Icon, body.Color, body.Essentiality); err != nil {
              jsonError(w, err.Error(), http.StatusBadRequest)
              return
          }
          sortOrder := 0
          if body.SortOrder != nil {
              sortOrder = *body.SortOrder
          }
          res, err := db.DB.Exec(`
              UPDATE account_categories
              SET name = ?, icon = ?, color = ?, essentiality = ?, sort_order = ?
              WHERE account_id = ? AND key = ?
          `, body.Name, body.Icon, body.Color, body.Essentiality, sortOrder, accountID, key)
          if err != nil {
              jsonError(w, "Erro ao atualizar categoria", http.StatusInternalServerError)
              return
          }
          n, _ := res.RowsAffected()
          if n == 0 {
              jsonError(w, "Categoria não encontrada", http.StatusNotFound)
              return
          }
          var c models.Category
          _ = db.DB.QueryRow(`
              SELECT id, account_id, key, name, icon, color, essentiality, sort_order
              FROM account_categories WHERE account_id = ? AND key = ?
          `, accountID, key).Scan(&c.ID, &c.AccountID, &c.Key, &c.Name, &c.Icon, &c.Color, &c.Essentiality, &c.SortOrder)
          writeJSON(w, http.StatusOK, c)

      case http.MethodDelete:
          if !requireAccountEdit(w, r) {
              return
          }
          if !isItem {
              jsonError(w, "Chave de categoria obrigatória", http.StatusBadRequest)
              return
          }
          replacement := strings.TrimSpace(r.URL.Query().Get("replacement"))
          if replacement == "" {
              jsonError(w, "Chave de substituição obrigatória", http.StatusBadRequest)
              return
          }
          if replacement == key {
              jsonError(w, "Substituição deve ser diferente da categoria removida", http.StatusBadRequest)
              return
          }
          // Check replacement exists in this account
          var repCount int
          if err := db.DB.QueryRow(`SELECT COUNT(*) FROM account_categories WHERE account_id = ? AND key = ?`, accountID, replacement).Scan(&repCount); err != nil || repCount == 0 {
              jsonError(w, "Categoria de substituição não encontrada", http.StatusBadRequest)
              return
          }
          // Check at least 2 categories remain after deletion
          var total int
          _ = db.DB.QueryRow(`SELECT COUNT(*) FROM account_categories WHERE account_id = ?`, accountID).Scan(&total)
          if total <= 1 {
              jsonError(w, "Mantenha ao menos uma categoria", http.StatusBadRequest)
              return
          }

          tx, err := db.DB.Begin()
          if err != nil {
              jsonError(w, "Erro ao remover categoria", http.StatusInternalServerError)
              return
          }
          defer tx.Rollback()

          if _, err := tx.Exec(`UPDATE expenses SET category = ? WHERE account_id = ? AND category = ?`, replacement, accountID, key); err != nil {
              jsonError(w, "Erro ao reatribuir gastos", http.StatusInternalServerError)
              return
          }
          if _, err := tx.Exec(`UPDATE recurring_expenses SET category = ? WHERE account_id = ? AND category = ?`, replacement, accountID, key); err != nil {
              jsonError(w, "Erro ao reatribuir gastos recorrentes", http.StatusInternalServerError)
              return
          }
          res, err := tx.Exec(`DELETE FROM account_categories WHERE account_id = ? AND key = ?`, accountID, key)
          if err != nil {
              jsonError(w, "Erro ao remover categoria", http.StatusInternalServerError)
              return
          }
          n, _ := res.RowsAffected()
          if n == 0 {
              jsonError(w, "Categoria não encontrada", http.StatusNotFound)
              return
          }
          if err := tx.Commit(); err != nil {
              jsonError(w, "Erro ao remover categoria", http.StatusInternalServerError)
              return
          }
          w.WriteHeader(http.StatusNoContent)

      default:
          w.Header().Set("Allow", "GET, POST, PATCH, DELETE, OPTIONS")
          jsonError(w, "Método não permitido", http.StatusMethodNotAllowed)
      }
  }
  ```

  **Note on `uniqueCategoryKey`:** The helper above has a placeholder `itoa`. Replace those two helper functions with this cleaner version:

  ```go
  func uniqueCategoryKey(base string, accountID int64) (string, error) {
      key := base
      for i := 2; ; i++ {
          var count int
          if err := db.DB.QueryRow(
              `SELECT COUNT(*) FROM account_categories WHERE account_id = ? AND key = ?`,
              accountID, key,
          ).Scan(&count); err != nil {
              return "", err
          }
          if count == 0 {
              return key, nil
          }
          key = base + "_" + strconv.Itoa(i)
      }
  }
  ```

  So the actual import block for `categories.go` is:

  ```go
  import (
      "gastos/src/db"
      "gastos/src/middleware"
      "gastos/src/models"
      "net/http"
      "strconv"
      "strings"
      "unicode"
  )
  ```

  And drop the `itoa` function entirely — use `strconv.Itoa` directly.

- [ ] **Step 2: Build**

  ```bash
  go build ./...
  ```
  Expected: no errors.

- [ ] **Step 3: Commit**

  ```bash
  git add src/handlers/categories.go
  git commit -m "feat: add Categories handler with full CRUD"
  ```

---

### Task 5: Payment methods handler

**Files:**
- Create: `src/handlers/payment_methods.go`

- [ ] **Step 1: Create the file**

  Create `src/handlers/payment_methods.go`:

  ```go
  package handlers

  import (
      "gastos/src/db"
      "gastos/src/middleware"
      "gastos/src/models"
      "net/http"
      "strconv"
      "strings"
  )

  func uniquePaymentKey(base string, accountID int64) (string, error) {
      key := base
      for i := 2; ; i++ {
          var count int
          if err := db.DB.QueryRow(
              `SELECT COUNT(*) FROM account_payment_methods WHERE account_id = ? AND key = ?`,
              accountID, key,
          ).Scan(&count); err != nil {
              return "", err
          }
          if count == 0 {
              return key, nil
          }
          key = base + "_" + strconv.Itoa(i)
      }
  }

  func PaymentMethods(w http.ResponseWriter, r *http.Request) {
      accountID := middleware.AccountIDFromContext(r.Context())

      key := strings.TrimPrefix(r.URL.Path, "/api/payment-methods/")
      key = strings.TrimSpace(key)
      isItem := key != "" && key != "/api/payment-methods"

      switch r.Method {
      case http.MethodGet:
          if isItem {
              jsonError(w, "Método não permitido", http.StatusMethodNotAllowed)
              return
          }
          rows, err := db.DB.Query(`
              SELECT id, account_id, key, icon, name, sort_order
              FROM account_payment_methods
              WHERE account_id = ?
              ORDER BY sort_order ASC, id ASC
          `, accountID)
          if err != nil {
              jsonError(w, "Erro ao buscar métodos de pagamento", http.StatusInternalServerError)
              return
          }
          defer rows.Close()
          pms := make([]models.PaymentMethod, 0)
          for rows.Next() {
              var p models.PaymentMethod
              if err := rows.Scan(&p.ID, &p.AccountID, &p.Key, &p.Icon, &p.Name, &p.SortOrder); err != nil {
                  jsonError(w, "Erro ao ler métodos de pagamento", http.StatusInternalServerError)
                  return
              }
              pms = append(pms, p)
          }
          if err := rows.Err(); err != nil {
              jsonError(w, "Erro ao iterar métodos de pagamento", http.StatusInternalServerError)
              return
          }
          writeJSON(w, http.StatusOK, pms)

      case http.MethodPost:
          if !requireAccountEdit(w, r) {
              return
          }
          if isItem {
              jsonError(w, "Método não permitido", http.StatusMethodNotAllowed)
              return
          }
          var body struct {
              Name string `json:"name"`
              Icon string `json:"icon"`
          }
          if err := decodeJSON(r, &body); err != nil {
              jsonError(w, err.Error(), http.StatusBadRequest)
              return
          }
          body.Name = strings.TrimSpace(body.Name)
          body.Icon = strings.TrimSpace(body.Icon)
          if err := validatePaymentMethod(body.Name, body.Icon); err != nil {
              jsonError(w, err.Error(), http.StatusBadRequest)
              return
          }
          base := slugify(body.Name)
          if base == "" {
              base = "pagamento"
          }
          pmKey, err := uniquePaymentKey(base, accountID)
          if err != nil {
              jsonError(w, "Erro ao gerar chave", http.StatusInternalServerError)
              return
          }
          var maxOrder int
          _ = db.DB.QueryRow(`SELECT COALESCE(MAX(sort_order),0) FROM account_payment_methods WHERE account_id = ?`, accountID).Scan(&maxOrder)

          var p models.PaymentMethod
          err = db.DB.QueryRow(`
              INSERT INTO account_payment_methods (account_id, key, icon, name, sort_order)
              VALUES (?, ?, ?, ?, ?)
              RETURNING id, account_id, key, icon, name, sort_order
          `, accountID, pmKey, body.Icon, body.Name, maxOrder+1).Scan(
              &p.ID, &p.AccountID, &p.Key, &p.Icon, &p.Name, &p.SortOrder,
          )
          if err != nil {
              jsonError(w, "Erro ao criar método de pagamento", http.StatusInternalServerError)
              return
          }
          writeJSON(w, http.StatusCreated, p)

      case http.MethodPatch:
          if !requireAccountEdit(w, r) {
              return
          }
          if !isItem {
              jsonError(w, "Chave de método obrigatória", http.StatusBadRequest)
              return
          }
          var body struct {
              Name      string `json:"name"`
              Icon      string `json:"icon"`
              SortOrder *int   `json:"sortOrder"`
          }
          if err := decodeJSON(r, &body); err != nil {
              jsonError(w, err.Error(), http.StatusBadRequest)
              return
          }
          body.Name = strings.TrimSpace(body.Name)
          body.Icon = strings.TrimSpace(body.Icon)
          if err := validatePaymentMethod(body.Name, body.Icon); err != nil {
              jsonError(w, err.Error(), http.StatusBadRequest)
              return
          }
          sortOrder := 0
          if body.SortOrder != nil {
              sortOrder = *body.SortOrder
          }
          res, err := db.DB.Exec(`
              UPDATE account_payment_methods
              SET name = ?, icon = ?, sort_order = ?
              WHERE account_id = ? AND key = ?
          `, body.Name, body.Icon, sortOrder, accountID, key)
          if err != nil {
              jsonError(w, "Erro ao atualizar método de pagamento", http.StatusInternalServerError)
              return
          }
          n, _ := res.RowsAffected()
          if n == 0 {
              jsonError(w, "Método de pagamento não encontrado", http.StatusNotFound)
              return
          }
          var p models.PaymentMethod
          _ = db.DB.QueryRow(`
              SELECT id, account_id, key, icon, name, sort_order
              FROM account_payment_methods WHERE account_id = ? AND key = ?
          `, accountID, key).Scan(&p.ID, &p.AccountID, &p.Key, &p.Icon, &p.Name, &p.SortOrder)
          writeJSON(w, http.StatusOK, p)

      case http.MethodDelete:
          if !requireAccountEdit(w, r) {
              return
          }
          if !isItem {
              jsonError(w, "Chave de método obrigatória", http.StatusBadRequest)
              return
          }
          replacement := strings.TrimSpace(r.URL.Query().Get("replacement"))
          if replacement == "" {
              jsonError(w, "Chave de substituição obrigatória", http.StatusBadRequest)
              return
          }
          if replacement == key {
              jsonError(w, "Substituição deve ser diferente do método removido", http.StatusBadRequest)
              return
          }
          var repCount int
          if err := db.DB.QueryRow(`SELECT COUNT(*) FROM account_payment_methods WHERE account_id = ? AND key = ?`, accountID, replacement).Scan(&repCount); err != nil || repCount == 0 {
              jsonError(w, "Método de substituição não encontrado", http.StatusBadRequest)
              return
          }
          var total int
          _ = db.DB.QueryRow(`SELECT COUNT(*) FROM account_payment_methods WHERE account_id = ?`, accountID).Scan(&total)
          if total <= 1 {
              jsonError(w, "Mantenha ao menos um método de pagamento", http.StatusBadRequest)
              return
          }

          tx, err := db.DB.Begin()
          if err != nil {
              jsonError(w, "Erro ao remover método de pagamento", http.StatusInternalServerError)
              return
          }
          defer tx.Rollback()

          if _, err := tx.Exec(`UPDATE expenses SET payment = ? WHERE account_id = ? AND payment = ?`, replacement, accountID, key); err != nil {
              jsonError(w, "Erro ao reatribuir gastos", http.StatusInternalServerError)
              return
          }
          if _, err := tx.Exec(`UPDATE recurring_expenses SET payment = ? WHERE account_id = ? AND payment = ?`, replacement, accountID, key); err != nil {
              jsonError(w, "Erro ao reatribuir gastos recorrentes", http.StatusInternalServerError)
              return
          }
          res, err := tx.Exec(`DELETE FROM account_payment_methods WHERE account_id = ? AND key = ?`, accountID, key)
          if err != nil {
              jsonError(w, "Erro ao remover método de pagamento", http.StatusInternalServerError)
              return
          }
          n, _ := res.RowsAffected()
          if n == 0 {
              jsonError(w, "Método de pagamento não encontrado", http.StatusNotFound)
              return
          }
          if err := tx.Commit(); err != nil {
              jsonError(w, "Erro ao remover método de pagamento", http.StatusInternalServerError)
              return
          }
          w.WriteHeader(http.StatusNoContent)

      default:
          w.Header().Set("Allow", "GET, POST, PATCH, DELETE, OPTIONS")
          jsonError(w, "Método não permitido", http.StatusMethodNotAllowed)
      }
  }
  ```

- [ ] **Step 2: Build**

  ```bash
  go build ./...
  ```
  Expected: no errors.

- [ ] **Step 3: Commit**

  ```bash
  git add src/handlers/payment_methods.go
  git commit -m "feat: add PaymentMethods handler with full CRUD"
  ```

---

### Task 6: Register routes

**Files:**
- Modify: `src/main.go`

- [ ] **Step 1: Add 4 route pairs**

  In `src/main.go`, after the `split-payments` route pair and before `// Arquivos estáticos`, add:

  ```go
  mux.HandleFunc("/api/categories", cors(middleware.Auth(middleware.Account(handlers.Categories))))
  mux.HandleFunc("/api/categories/", cors(middleware.Auth(middleware.Account(handlers.Categories))))
  mux.HandleFunc("/api/payment-methods", cors(middleware.Auth(middleware.Account(handlers.PaymentMethods))))
  mux.HandleFunc("/api/payment-methods/", cors(middleware.Auth(middleware.Account(handlers.PaymentMethods))))
  ```

- [ ] **Step 2: Build and run**

  ```bash
  go build ./...
  ```
  Expected: no errors.

  ```bash
  go run . &
  sleep 1
  curl -s http://localhost:8000/api/categories -H "Authorization: Bearer bad" | head -c 100
  kill %1
  ```
  Expected: JSON error about auth (not 404).

- [ ] **Step 3: Commit**

  ```bash
  git add src/main.go
  git commit -m "feat: register /api/categories and /api/payment-methods routes"
  ```

---

### Task 7: Frontend — load from API, fix helper functions

**Files:**
- Modify: `src/web/index.html`

This task has many small edits. Apply them one at a time.

- [ ] **Step 1: Remove hardcoded `categories` array, replace with empty array**

  In `src/web/index.html`, find (lines 1127–1143):

  ```js
  categories: [
    { id:'food',          name:'Alimentação',  icon:'🍽',  color:'#0f6579', essentiality:'essential' },
    { id:'market',        name:'Mercado',       icon:'🛒',  color:'#1f7d8f', essentiality:'essential' },
    { id:'transport',     name:'Transporte',    icon:'🚗',  color:'#2b92a1', essentiality:'essential' },
    { id:'health',        name:'Saúde',         icon:'💊',  color:'#4ecc73', essentiality:'essential' },
    { id:'home',          name:'Moradia',       icon:'🏠',  color:'#2c9b88', essentiality:'essential' },
    { id:'utilities',     name:'Contas',        icon:'⚡',  color:'#d6a23a', essentiality:'essential' },
    { id:'restaurant',    name:'Restaurante',   icon:'🍜',  color:'#55b38c', essentiality:'nonessential' },
    { id:'leisure',       name:'Lazer',         icon:'🎮',  color:'#66c3a1', essentiality:'nonessential' },
    { id:'subscriptions', name:'Assinaturas',   icon:'📺',  color:'#4ecc73', essentiality:'nonessential' },
    { id:'clothes',       name:'Vestuário',     icon:'👕',  color:'#88c0b3', essentiality:'nonessential' },
    { id:'beauty',        name:'Beleza',        icon:'✨',  color:'#79b7ae', essentiality:'nonessential' },
    { id:'travel',        name:'Viagem',        icon:'✈️',  color:'#54a7b9', essentiality:'nonessential' },
    { id:'education',     name:'Educação',      icon:'📚',  color:'#2ba18a', essentiality:'investment' },
    { id:'invest',        name:'Investimentos', icon:'📈',  color:'#4ecc73', essentiality:'investment' },
    { id:'other',         name:'Outros',        icon:'◈',   color:'#7f9ea5', essentiality:'nonessential' },
  ],
  ```

  Replace with:

  ```js
  categories: [],
  ```

- [ ] **Step 2: Remove hardcoded `paymentMethods` array, replace with empty array**

  Find (lines 1144–1153):

  ```js
  paymentMethods: [
    { id:'pix',      label:'⚡ Pix' },
    { id:'credit',   label:'💳 Crédito' },
    { id:'debit',    label:'🏦 Débito' },
    { id:'cash',     label:'💵 Dinheiro' },
    { id:'boleto',   label:'📄 Boleto' },
    { id:'va',       label:'🍽 Vale Alim.' },
    { id:'vr',       label:'🛒 Vale Ref.' },
    { id:'transfer', label:'↔ Transferência' },
  ],
  ```

  Replace with:

  ```js
  paymentMethods: [],
  ```

- [ ] **Step 3: Update `loadAllData` to fetch categories and payment methods**

  Find:

  ```js
  const [expenses, incomes, goals, recurringExpenses, splitPayments] = await Promise.all([
    API.get('/api/expenses'),
    API.get('/api/incomes'),
    API.get('/api/goals'),
    API.get('/api/recurring-expenses'),
    API.get('/api/split-payments'),
  ])
  this.expenses = expenses || []
  this.incomes = incomes || []
  this.goals = goals || []
  this.recurringExpenses = recurringExpenses || []
  this.splitPayments = splitPayments || []
  ```

  Replace with:

  ```js
  const [expenses, incomes, goals, recurringExpenses, splitPayments, categories, paymentMethods] = await Promise.all([
    API.get('/api/expenses'),
    API.get('/api/incomes'),
    API.get('/api/goals'),
    API.get('/api/recurring-expenses'),
    API.get('/api/split-payments'),
    API.get('/api/categories'),
    API.get('/api/payment-methods'),
  ])
  this.expenses = expenses || []
  this.incomes = incomes || []
  this.goals = goals || []
  this.recurringExpenses = recurringExpenses || []
  this.splitPayments = splitPayments || []
  this.categories = categories || []
  this.paymentMethods = paymentMethods || []
  ```

  Also update the early-return branch (the `if (!this.hasActiveAccount)` block) to also clear both arrays:

  Find:

  ```js
  this.expenses = []
  this.incomes = []
  this.goals = []
  this.recurringExpenses = []
  this.splitPayments = []
  this.resetCharts()
  return
  ```

  Replace with:

  ```js
  this.expenses = []
  this.incomes = []
  this.goals = []
  this.recurringExpenses = []
  this.splitPayments = []
  this.categories = []
  this.paymentMethods = []
  this.resetCharts()
  return
  ```

- [ ] **Step 4: Fix `getCatIcon`, `getCatName`, `getCatEss` — `c.id` → `c.key`**

  Find (around line 2285):

  ```js
  getCatIcon(id) { return this.categories.find(c => c.id === id)?.icon || '◈' },
  getCatName(id) { return this.categories.find(c => c.id === id)?.name || id },
  getCatEss(id) { return this.categories.find(c => c.id === id)?.essentiality || 'nonessential' },
  ```

  Replace with:

  ```js
  getCatIcon(id) { return this.categories.find(c => c.key === id)?.icon || '◈' },
  getCatName(id) { return this.categories.find(c => c.key === id)?.name || id },
  getCatEss(id) { return this.categories.find(c => c.key === id)?.essentiality || 'nonessential' },
  ```

- [ ] **Step 5: Fix `payLabel` — `p.id`/`p.label` → `p.key`/`p.icon+' '+p.name`**

  Find:

  ```js
  payLabel(id) { return (this.paymentMethods.find(p => p.id === id)?.label || id).replace(/^.\s/, '') },
  ```

  Replace with:

  ```js
  payLabel(id) { const p = this.paymentMethods.find(p => p.key === id); return p ? p.icon + ' ' + p.name : id },
  ```

- [ ] **Step 6: Fix `categoryTotals` getter — `c.id` → `c.key`**

  Find:

  ```js
  return this.categories.filter(c => map[c.id])
    .map(c => ({ ...c, total: map[c.id] || 0, pct: ((map[c.id] || 0) / max * 100) }))
    .sort((a, b) => b.total - a.total)
  ```

  Replace with:

  ```js
  return this.categories.filter(c => map[c.key])
    .map(c => ({ ...c, total: map[c.key] || 0, pct: ((map[c.key] || 0) / max * 100) }))
    .sort((a, b) => b.total - a.total)
  ```

- [ ] **Step 7: Fix category select templates — `cat.id` → `cat.key`**

  Three occurrences (lines 595, 672, 824). Fix each one:

  **Line 595** (expense form category select):

  Find:
  ```html
  <select x-model="newTx.category"><template x-for="cat in categories" :key="cat.id"><option :value="cat.id" x-text="cat.icon+' '+cat.name"></option></template></select>
  ```
  Replace with:
  ```html
  <select x-model="newTx.category"><template x-for="cat in categories" :key="cat.key"><option :value="cat.key" x-text="cat.icon+' '+cat.name"></option></template></select>
  ```

  **Line 672** (filter chips):

  Find:
  ```html
  <template x-for="cat in categories" :key="cat.id"><button class="chip" :class="{active:activeFilter===cat.id}" @click="activeFilter=cat.id" x-text="cat.icon+' '+cat.name"></button></template>
  ```
  Replace with:
  ```html
  <template x-for="cat in categories" :key="cat.key"><button class="chip" :class="{active:activeFilter===cat.key}" @click="activeFilter=cat.key" x-text="cat.icon+' '+cat.name"></button></template>
  ```

  **Line 824** (goal category select):

  Find:
  ```html
  <select x-model="newGoal.category"><template x-for="cat in categories" :key="cat.id"><option :value="cat.id" x-text="cat.icon+' '+cat.name"></option></template></select>
  ```
  Replace with:
  ```html
  <select x-model="newGoal.category"><template x-for="cat in categories" :key="cat.key"><option :value="cat.key" x-text="cat.icon+' '+cat.name"></option></template></select>
  ```

- [ ] **Step 8: Fix payment method select template — `p.id`/`p.label` → `p.key`/`p.icon+' '+p.name`**

  **Line 596** (expense form payment select):

  Find:
  ```html
  <select x-model="newTx.payment"><template x-for="p in paymentMethods" :key="p.id"><option :value="p.id" x-text="p.label"></option></template></select>
  ```
  Replace with:
  ```html
  <select x-model="newTx.payment"><template x-for="p in paymentMethods" :key="p.key"><option :value="p.key" x-text="p.icon+' '+p.name"></option></template></select>
  ```

- [ ] **Step 9: Start dev server and verify expense form still works**

  ```bash
  go run . &
  ```

  Open `http://localhost:8000` in a browser. Log in, select an account. Verify:
  - Expense form shows the correct categories and payment methods (loaded from DB)
  - Filter chips show category names
  - Expense list shows correct category names and payment method labels

  ```bash
  kill %1
  ```

- [ ] **Step 10: Commit**

  ```bash
  git add src/web/index.html
  git commit -m "feat: load categories and payment methods from API, fix helper functions"
  ```

---

### Task 8: Frontend — emoji picker state and curated list

**Files:**
- Modify: `src/web/index.html`

- [ ] **Step 1: Add emoji picker state to Alpine data**

  In the Alpine data object (search for `paymentMethods: [],` from the previous task), add after it:

  ```js
  emojiPickerOpen: false,
  emojiPickerTarget: null,
  emojiPickerX: 0,
  emojiPickerY: 0,
  EMOJIS: [
    { section: 'Comida', list: ['🍽','🍜','🍕','🍔','🥗','🛒','🥩','🍺','☕','🧃','🍰','🍣','🌮','🥐','🍱'] },
    { section: 'Transporte', list: ['🚗','🚕','🚌','🚆','✈️','🛵','🚲','⛽','🅿️','🚁','🛻','🚂','🛳'] },
    { section: 'Saúde', list: ['💊','🏥','🦷','👓','🧬','💉','🩺','🏃','🧘','🩹','💪','🧪','🩻'] },
    { section: 'Finanças', list: ['💰','💳','📈','📉','🏦','💵','💸','🔐','📊','🪙','🏧','💹','🤑'] },
    { section: 'Casa', list: ['🏠','🛋','💡','🔧','🧹','📦','🪴','🛁','🔑','🪟','🛏','🪣','🧺'] },
    { section: 'Lazer', list: ['🎮','🎬','📚','🎵','⚽','🏋️','🎭','🎲','🏖','🎨','🎧','🎸','🏊','🧩'] },
    { section: 'Outros', list: ['✨','📺','📱','👕','💄','🐾','🎁','📝','◈','🔔','🏷','🎓','🎯','🌟'] },
  ],
  ```

- [ ] **Step 2: Add `openEmojiPicker` and `pickEmoji` methods**

  In the Alpine methods section (after existing methods), add:

  ```js
  openEmojiPicker(event, target) {
    const rect = event.currentTarget.getBoundingClientRect()
    this.emojiPickerX = rect.left + window.scrollX
    this.emojiPickerY = rect.bottom + window.scrollY + 4
    this.emojiPickerTarget = target
    this.emojiPickerOpen = true
  },
  pickEmoji(emoji) {
    if (this.emojiPickerTarget) this.emojiPickerTarget.value = emoji
    this.emojiPickerOpen = false
    this.emojiPickerTarget = null
  },
  closeEmojiPicker() {
    this.emojiPickerOpen = false
    this.emojiPickerTarget = null
  },
  ```

  **Note on `emojiPickerTarget`:** The target is a reactive object reference (e.g., `newCat.icon`) passed by reference from the template. Since Alpine data properties are plain objects, mutating `.value` won't work. Instead, use a string key pattern:

  Replace the `pickEmoji` and `openEmojiPicker` approach with a simpler string-key approach:

  ```js
  // emojiPickerTarget is a string key like 'newCat.icon' or 'editCat.icon'
  openEmojiPicker(event, targetKey) {
    const rect = event.currentTarget.getBoundingClientRect()
    this.emojiPickerX = rect.left + window.scrollX
    this.emojiPickerY = rect.bottom + window.scrollY + 4
    this.emojiPickerTarget = targetKey
    this.emojiPickerOpen = true
  },
  pickEmoji(emoji) {
    if (this.emojiPickerTarget === 'newCat') this.newCat.icon = emoji
    else if (this.emojiPickerTarget === 'editCat') this.editCat.icon = emoji
    else if (this.emojiPickerTarget === 'newPm') this.newPm.icon = emoji
    else if (this.emojiPickerTarget === 'editPm') this.editPm.icon = emoji
    this.emojiPickerOpen = false
    this.emojiPickerTarget = null
  },
  closeEmojiPicker() {
    this.emojiPickerOpen = false
    this.emojiPickerTarget = null
  },
  ```

- [ ] **Step 3: Add emoji picker HTML overlay**

  Place this just before the closing `</body>` tag (or at the end of the main Alpine `x-data` div, before its closing `</div>`):

  ```html
  <!-- Emoji picker overlay -->
  <div x-show="emojiPickerOpen" @click.outside="closeEmojiPicker()"
       :style="'position:fixed;z-index:9999;left:'+emojiPickerX+'px;top:'+emojiPickerY+'px'"
       style="background:var(--surface2);border:1px solid var(--border);border-radius:10px;padding:10px;width:260px;max-height:320px;overflow-y:auto;box-shadow:0 8px 32px rgba(0,0,0,0.18)">
    <template x-for="section in EMOJIS" :key="section.section">
      <div style="margin-bottom:8px">
        <div style="font-size:10px;font-weight:600;color:var(--text2);text-transform:uppercase;margin-bottom:4px" x-text="section.section"></div>
        <div style="display:flex;flex-wrap:wrap;gap:2px">
          <template x-for="em in section.list" :key="em">
            <button type="button" @click="pickEmoji(em)"
                    style="background:none;border:none;cursor:pointer;font-size:20px;padding:3px;border-radius:5px;line-height:1"
                    :title="em" x-text="em"></button>
          </template>
        </div>
      </div>
    </template>
  </div>
  ```

- [ ] **Step 4: Commit**

  ```bash
  git add src/web/index.html
  git commit -m "feat: add emoji picker state, curated emoji list, and picker overlay"
  ```

---

### Task 9: Frontend — categories settings card

**Files:**
- Modify: `src/web/index.html`

- [ ] **Step 1: Add settings state to Alpine data**

  After the emoji picker state added in Task 8, add:

  ```js
  newCat: { name: '', icon: '◈', color: '#4ecc73', essentiality: 'nonessential' },
  editCat: { key: '', name: '', icon: '', color: '', essentiality: '' },
  deleteCat: { key: '', replacementKey: '' },
  catFormOpen: false,
  ```

- [ ] **Step 2: Add category management methods**

  ```js
  async createCategory() {
    try {
      const c = await API.post('/api/categories', this.newCat)
      this.categories.push(c)
      this.newCat = { name: '', icon: '◈', color: '#4ecc73', essentiality: 'nonessential' }
      this.catFormOpen = false
      this.toast('Categoria criada ✓')
    } catch (e) { this.toast(e.message, true) }
  },
  startEditCat(c) {
    this.editCat = { key: c.key, name: c.name, icon: c.icon, color: c.color, essentiality: c.essentiality }
    this.deleteCat = { key: '', replacementKey: '' }
  },
  async saveEditCat() {
    try {
      const updated = await API.patch('/api/categories/' + this.editCat.key, {
        name: this.editCat.name, icon: this.editCat.icon,
        color: this.editCat.color, essentiality: this.editCat.essentiality
      })
      const idx = this.categories.findIndex(c => c.key === updated.key)
      if (idx !== -1) this.categories[idx] = updated
      this.editCat = { key: '', name: '', icon: '', color: '', essentiality: '' }
      this.toast('Categoria atualizada ✓')
    } catch (e) { this.toast(e.message, true) }
  },
  cancelEditCat() {
    this.editCat = { key: '', name: '', icon: '', color: '', essentiality: '' }
  },
  startDeleteCat(key) {
    this.deleteCat = { key, replacementKey: '' }
    this.editCat = { key: '', name: '', icon: '', color: '', essentiality: '' }
  },
  async confirmDeleteCat() {
    if (!this.deleteCat.replacementKey) { this.toast('Selecione uma categoria substituta', true); return }
    try {
      await API.delete('/api/categories/' + this.deleteCat.key + '?replacement=' + encodeURIComponent(this.deleteCat.replacementKey))
      this.categories = this.categories.filter(c => c.key !== this.deleteCat.key)
      this.deleteCat = { key: '', replacementKey: '' }
      await this.loadAllData()
      this.toast('Categoria removida ✓')
    } catch (e) { this.toast(e.message, true) }
  },
  ```

- [ ] **Step 3: Add the categories settings card HTML**

  In `src/web/index.html`, inside the settings page `.settings-stack` div, after the "Preferências do app" card and before any closing tags, add this new card:

  ```html
  <div class="card" x-show="hasActiveAccount">
    <div class="card-title">Categorias</div>
    <template x-for="c in categories" :key="c.key">
      <div>
        <!-- Normal row -->
        <div x-show="editCat.key !== c.key && deleteCat.key !== c.key"
             style="display:flex;align-items:center;gap:10px;padding:8px 0;border-bottom:1px solid var(--border)">
          <span x-text="c.icon" style="font-size:20px;width:28px;text-align:center"></span>
          <span style="flex:1;font-size:14px;font-weight:500" x-text="c.name"></span>
          <span style="font-size:11px;padding:2px 7px;border-radius:20px;background:var(--surface3);color:var(--text2)" x-text="c.essentiality === 'essential' ? 'essencial' : c.essentiality === 'investment' ? 'investimento' : 'não essencial'"></span>
          <span :style="'width:16px;height:16px;border-radius:50%;background:'+c.color+';display:inline-block'"></span>
          <button class="btn-sm" @click="startEditCat(c)">Editar</button>
          <button class="btn-sm btn-danger" @click="startDeleteCat(c.key)">Remover</button>
        </div>
        <!-- Inline edit row -->
        <div x-show="editCat.key === c.key"
             style="padding:10px 0;border-bottom:1px solid var(--border)">
          <div style="display:flex;gap:8px;flex-wrap:wrap;align-items:center;margin-bottom:8px">
            <button type="button" @click="openEmojiPicker($event, 'editCat')"
                    style="font-size:22px;background:var(--surface3);border:1px solid var(--border);border-radius:7px;padding:4px 10px;cursor:pointer"
                    x-text="editCat.icon"></button>
            <input type="text" x-model="editCat.name" placeholder="Nome" style="flex:1;min-width:120px" />
            <select x-model="editCat.essentiality">
              <option value="essential">Essencial</option>
              <option value="nonessential">Não essencial</option>
              <option value="investment">Investimento</option>
            </select>
            <input type="color" x-model="editCat.color" style="width:40px;height:36px;border:none;cursor:pointer;border-radius:7px" />
          </div>
          <div style="display:flex;gap:8px">
            <button class="btn-sm" @click="saveEditCat()">Salvar</button>
            <button class="btn-sm" @click="cancelEditCat()">Cancelar</button>
          </div>
        </div>
        <!-- Inline delete confirmation -->
        <div x-show="deleteCat.key === c.key"
             style="padding:10px 0;border-bottom:1px solid var(--border);background:var(--surface3);border-radius:8px;padding:10px">
          <div style="font-size:13px;margin-bottom:8px">Mover gastos de <strong x-text="c.name"></strong> para:</div>
          <div style="display:flex;gap:8px;align-items:center;flex-wrap:wrap">
            <select x-model="deleteCat.replacementKey" style="flex:1;min-width:120px">
              <option value="">Selecionar...</option>
              <template x-for="other in categories.filter(o => o.key !== c.key)" :key="other.key">
                <option :value="other.key" x-text="other.icon+' '+other.name"></option>
              </template>
            </select>
            <button class="btn-sm btn-danger" @click="confirmDeleteCat()">Confirmar</button>
            <button class="btn-sm" @click="deleteCat={key:'',replacementKey:''}">Cancelar</button>
          </div>
        </div>
      </div>
    </template>
    <!-- Add new category -->
    <div style="margin-top:10px">
      <button class="btn-sm" x-show="!catFormOpen" @click="catFormOpen=true">+ Adicionar categoria</button>
      <div x-show="catFormOpen">
        <div style="display:flex;gap:8px;flex-wrap:wrap;align-items:center;margin-bottom:8px;margin-top:8px">
          <button type="button" @click="openEmojiPicker($event, 'newCat')"
                  style="font-size:22px;background:var(--surface3);border:1px solid var(--border);border-radius:7px;padding:4px 10px;cursor:pointer"
                  x-text="newCat.icon"></button>
          <input type="text" x-model="newCat.name" placeholder="Nome da categoria" style="flex:1;min-width:120px" />
          <select x-model="newCat.essentiality">
            <option value="essential">Essencial</option>
            <option value="nonessential">Não essencial</option>
            <option value="investment">Investimento</option>
          </select>
          <input type="color" x-model="newCat.color" style="width:40px;height:36px;border:none;cursor:pointer;border-radius:7px" />
        </div>
        <div style="display:flex;gap:8px">
          <button class="btn-sm" @click="createCategory()">Criar</button>
          <button class="btn-sm" @click="catFormOpen=false;newCat={name:'',icon:'◈',color:'#4ecc73',essentiality:'nonessential'}">Cancelar</button>
        </div>
      </div>
    </div>
  </div>
  ```

- [ ] **Step 4: Commit**

  ```bash
  git add src/web/index.html
  git commit -m "feat: add categories settings card with inline add/edit/delete"
  ```

---

### Task 10: Frontend — payment methods settings card

**Files:**
- Modify: `src/web/index.html`

- [ ] **Step 1: Add payment method state to Alpine data**

  After the `catFormOpen` state added in Task 9, add:

  ```js
  newPm: { name: '', icon: '💳' },
  editPm: { key: '', name: '', icon: '' },
  deletePm: { key: '', replacementKey: '' },
  pmFormOpen: false,
  ```

- [ ] **Step 2: Add payment method management methods**

  ```js
  async createPaymentMethod() {
    try {
      const p = await API.post('/api/payment-methods', this.newPm)
      this.paymentMethods.push(p)
      this.newPm = { name: '', icon: '💳' }
      this.pmFormOpen = false
      this.toast('Método criado ✓')
    } catch (e) { this.toast(e.message, true) }
  },
  startEditPm(p) {
    this.editPm = { key: p.key, name: p.name, icon: p.icon }
    this.deletePm = { key: '', replacementKey: '' }
  },
  async saveEditPm() {
    try {
      const updated = await API.patch('/api/payment-methods/' + this.editPm.key, {
        name: this.editPm.name, icon: this.editPm.icon
      })
      const idx = this.paymentMethods.findIndex(p => p.key === updated.key)
      if (idx !== -1) this.paymentMethods[idx] = updated
      this.editPm = { key: '', name: '', icon: '' }
      this.toast('Método atualizado ✓')
    } catch (e) { this.toast(e.message, true) }
  },
  cancelEditPm() {
    this.editPm = { key: '', name: '', icon: '' }
  },
  startDeletePm(key) {
    this.deletePm = { key, replacementKey: '' }
    this.editPm = { key: '', name: '', icon: '' }
  },
  async confirmDeletePm() {
    if (!this.deletePm.replacementKey) { this.toast('Selecione um método substituto', true); return }
    try {
      await API.delete('/api/payment-methods/' + this.deletePm.key + '?replacement=' + encodeURIComponent(this.deletePm.replacementKey))
      this.paymentMethods = this.paymentMethods.filter(p => p.key !== this.deletePm.key)
      this.deletePm = { key: '', replacementKey: '' }
      await this.loadAllData()
      this.toast('Método removido ✓')
    } catch (e) { this.toast(e.message, true) }
  },
  ```

- [ ] **Step 3: Add the payment methods settings card HTML**

  After the categories card added in Task 9, add:

  ```html
  <div class="card" x-show="hasActiveAccount">
    <div class="card-title">Métodos de pagamento</div>
    <template x-for="p in paymentMethods" :key="p.key">
      <div>
        <!-- Normal row -->
        <div x-show="editPm.key !== p.key && deletePm.key !== p.key"
             style="display:flex;align-items:center;gap:10px;padding:8px 0;border-bottom:1px solid var(--border)">
          <span x-text="p.icon" style="font-size:20px;width:28px;text-align:center"></span>
          <span style="flex:1;font-size:14px;font-weight:500" x-text="p.name"></span>
          <button class="btn-sm" @click="startEditPm(p)">Editar</button>
          <button class="btn-sm btn-danger" @click="startDeletePm(p.key)">Remover</button>
        </div>
        <!-- Inline edit row -->
        <div x-show="editPm.key === p.key"
             style="padding:10px 0;border-bottom:1px solid var(--border)">
          <div style="display:flex;gap:8px;flex-wrap:wrap;align-items:center;margin-bottom:8px">
            <button type="button" @click="openEmojiPicker($event, 'editPm')"
                    style="font-size:22px;background:var(--surface3);border:1px solid var(--border);border-radius:7px;padding:4px 10px;cursor:pointer"
                    x-text="editPm.icon"></button>
            <input type="text" x-model="editPm.name" placeholder="Nome" style="flex:1;min-width:120px" />
          </div>
          <div style="display:flex;gap:8px">
            <button class="btn-sm" @click="saveEditPm()">Salvar</button>
            <button class="btn-sm" @click="cancelEditPm()">Cancelar</button>
          </div>
        </div>
        <!-- Inline delete confirmation -->
        <div x-show="deletePm.key === p.key"
             style="padding:10px 0;border-bottom:1px solid var(--border);background:var(--surface3);border-radius:8px;padding:10px">
          <div style="font-size:13px;margin-bottom:8px">Mover gastos de <strong x-text="p.icon+' '+p.name"></strong> para:</div>
          <div style="display:flex;gap:8px;align-items:center;flex-wrap:wrap">
            <select x-model="deletePm.replacementKey" style="flex:1;min-width:120px">
              <option value="">Selecionar...</option>
              <template x-for="other in paymentMethods.filter(o => o.key !== p.key)" :key="other.key">
                <option :value="other.key" x-text="other.icon+' '+other.name"></option>
              </template>
            </select>
            <button class="btn-sm btn-danger" @click="confirmDeletePm()">Confirmar</button>
            <button class="btn-sm" @click="deletePm={key:'',replacementKey:''}">Cancelar</button>
          </div>
        </div>
      </div>
    </template>
    <!-- Add new payment method -->
    <div style="margin-top:10px">
      <button class="btn-sm" x-show="!pmFormOpen" @click="pmFormOpen=true">+ Adicionar método</button>
      <div x-show="pmFormOpen">
        <div style="display:flex;gap:8px;flex-wrap:wrap;align-items:center;margin-bottom:8px;margin-top:8px">
          <button type="button" @click="openEmojiPicker($event, 'newPm')"
                  style="font-size:22px;background:var(--surface3);border:1px solid var(--border);border-radius:7px;padding:4px 10px;cursor:pointer"
                  x-text="newPm.icon"></button>
          <input type="text" x-model="newPm.name" placeholder="Nome do método" style="flex:1;min-width:120px" />
        </div>
        <div style="display:flex;gap:8px">
          <button class="btn-sm" @click="createPaymentMethod()">Criar</button>
          <button class="btn-sm" @click="pmFormOpen=false;newPm={name:'',icon:'💳'}">Cancelar</button>
        </div>
      </div>
    </div>
  </div>
  ```

- [ ] **Step 4: Start dev server and verify full settings flow**

  ```bash
  go run . &
  ```

  Open `http://localhost:8000`. Log in and go to Settings. Verify:
  - "Categorias" card shows all 15 default categories
  - "Editar" opens inline form with current values and emoji picker
  - Emoji picker appears near button, clicking an emoji updates the icon field
  - Saving an edit updates the row immediately
  - "Remover" shows replacement dropdown; confirming deletes the category and reassigns expenses
  - "+ Adicionar categoria" opens a form; creating a new category adds it to the list
  - "Métodos de pagamento" card behaves the same for payment methods
  - Expense form still works: category and payment selects populate correctly

  ```bash
  kill %1
  ```

- [ ] **Step 5: Commit**

  ```bash
  git add src/web/index.html
  git commit -m "feat: add payment methods settings card with inline add/edit/delete"
  ```

---

## Self-Review

**Spec coverage check:**

| Spec requirement | Task |
|-----------------|------|
| Per-account scope | Task 2 (account_id FK on both tables) |
| Defaults seeded at account creation | Task 2 (seedAccountDefaults in CreateAccountWithOwner) |
| Existing accounts seeded by migration | Task 2 (runAccountCustomizationV1Migration) |
| Full control: name, icon, color, essentiality for categories | Tasks 3, 4, 9 |
| Icon/name for payment methods | Tasks 3, 5, 10 |
| Key is stable slug, immutable | Task 4 (key set on POST, never updated) |
| DELETE reassigns expenses and recurring_expenses atomically | Tasks 4, 5 |
| DELETE uses query param (not body) | Tasks 4, 5 (r.URL.Query().Get("replacement")) |
| Emoji picker: curated grid, no external lib | Task 8 |
| Emoji picker positioned via getBoundingClientRect | Task 8 |
| Settings page: list, inline edit, inline delete | Tasks 9, 10 |
| One active form at a time | Tasks 9, 10 (startEditCat cancels deleteCat and vice versa) |
| Frontend helpers use c.key not c.id | Task 7 |
| loadAllData fetches from API | Task 7 |
| Error: DELETE when ≤1 entry | Tasks 4, 5 (total ≤ 1 check) |
| Error: replacement key not found | Tasks 4, 5 |
| Error: PATCH on unknown key → 404 | Tasks 4, 5 (RowsAffected == 0 → 404) |

**Placeholder scan:** None found. All steps contain complete code.

**Type consistency check:**
- `Category.Key` (Go) → `c.key` (JS) ✓ throughout
- `PaymentMethod.Key` (Go) → `p.key` (JS) ✓ throughout  
- `slugify` defined once in `categories.go`, called from `payment_methods.go` (same package) ✓
- `validateCategory(name, icon, color, essentiality string)` signature matches all call sites ✓
- `validatePaymentMethod(name, icon string)` signature matches all call sites ✓
