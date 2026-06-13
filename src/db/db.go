package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	roleOwner  = "owner"
	roleEditor = "editor"
	roleReader = "reader"
)

var DB *sql.DB

func Init(path string) {
	if DB != nil {
		_ = DB.Close()
		DB = nil
	}

	var err error
	DB, err = sql.Open("sqlite", path)
	if err != nil {
		log.Fatal(err)
	}

	if err := DB.Ping(); err != nil {
		log.Fatal(err)
	}

	if err := enableForeignKeys(); err != nil {
		log.Fatal(err)
	}
	if err := createTables(); err != nil {
		log.Fatal(err)
	}
	if err := runMigrations(); err != nil {
		log.Fatal(err)
	}
	if err := createIndexes(); err != nil {
		log.Fatal(err)
	}
}

func CreateAccountWithOwner(tx *sql.Tx, name string, ownerUserID int64) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, errors.New("nome da conta é obrigatório")
	}
	if ownerUserID <= 0 {
		return 0, errors.New("owner inválido")
	}

	res, err := tx.Exec(`INSERT INTO accounts (name) VALUES (?)`, name)
	if err != nil {
		return 0, err
	}

	accountID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

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

func enableForeignKeys() error {
	_, err := DB.Exec(`PRAGMA foreign_keys = ON;`)
	return err
}

func createTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS accounts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS account_members (
			account_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			role TEXT NOT NULL CHECK(role IN ('owner','editor','reader')),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (account_id, user_id),
			FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS expenses (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			account_id INTEGER NOT NULL,
			amount REAL NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			category TEXT NOT NULL,
			payment TEXT NOT NULL,
			date TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS expense_splits (
			expense_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			percentage REAL NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (expense_id, user_id),
			FOREIGN KEY(expense_id) REFERENCES expenses(id) ON DELETE CASCADE,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS incomes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			account_id INTEGER NOT NULL,
			amount REAL NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL,
			date TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS goals (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			account_id INTEGER NOT NULL,
			category TEXT NOT NULL,
			"limit" REAL NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(account_id, category),
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS recurring_expenses (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			account_id INTEGER NOT NULL,
			amount REAL NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			category TEXT NOT NULL,
			payment TEXT NOT NULL,
			frequency TEXT NOT NULL DEFAULT 'monthly' CHECK(frequency IN ('monthly')),
			day_of_month INTEGER NOT NULL,
			start_date TEXT NOT NULL,
			end_date TEXT,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS recurring_expense_splits (
			recurring_expense_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			percentage REAL NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (recurring_expense_id, user_id),
			FOREIGN KEY(recurring_expense_id) REFERENCES recurring_expenses(id) ON DELETE CASCADE,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS split_payments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			account_id INTEGER NOT NULL,
			payer_user_id INTEGER NOT NULL,
			receiver_user_id INTEGER NOT NULL,
			amount REAL NOT NULL,
			date TEXT NOT NULL,
			note TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE,
			FOREIGN KEY(payer_user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY(receiver_user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			name TEXT PRIMARY KEY,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
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
	}

	for _, q := range queries {
		if _, err := DB.Exec(q); err != nil {
			return err
		}
	}

	return nil
}

func createIndexes() error {
	queries := []string{
		`CREATE INDEX IF NOT EXISTS idx_account_members_user ON account_members(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_account_members_account ON account_members(account_id);`,
		`CREATE INDEX IF NOT EXISTS idx_expenses_account_date ON expenses(account_id, date DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_expense_splits_user ON expense_splits(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_incomes_account_date ON incomes(account_id, date DESC);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_goals_account_category ON goals(account_id, category);`,
		`CREATE INDEX IF NOT EXISTS idx_recurring_expenses_account_enabled ON recurring_expenses(account_id, enabled, day_of_month);`,
		`CREATE INDEX IF NOT EXISTS idx_recurring_expense_splits_user ON recurring_expense_splits(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_split_payments_account_date ON split_payments(account_id, date DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_expenses_recurring ON expenses(recurring_expense_id, date);`,
		`CREATE INDEX IF NOT EXISTS idx_account_categories_account ON account_categories(account_id, sort_order);`,
		`CREATE INDEX IF NOT EXISTS idx_account_payment_methods_account ON account_payment_methods(account_id, sort_order);`,
	}

	for _, q := range queries {
		if _, err := DB.Exec(q); err != nil {
			return err
		}
	}

	return nil
}

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
	return runAccountCustomizationV1Migration()
}

func runLegacyColumnMigrations() error {
	migrations := []string{
		`ALTER TABLE expenses ADD COLUMN user_id INTEGER REFERENCES users(id) ON DELETE CASCADE;`,
		`ALTER TABLE expenses ADD COLUMN description TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE expenses ADD COLUMN category TEXT NOT NULL DEFAULT 'other';`,
		`ALTER TABLE expenses ADD COLUMN payment TEXT NOT NULL DEFAULT 'cash';`,
		`ALTER TABLE expenses ADD COLUMN date TEXT NOT NULL DEFAULT '1970-01-01';`,
		`ALTER TABLE incomes ADD COLUMN user_id INTEGER REFERENCES users(id) ON DELETE CASCADE;`,
		`ALTER TABLE incomes ADD COLUMN description TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE incomes ADD COLUMN type TEXT NOT NULL DEFAULT 'other';`,
		`ALTER TABLE incomes ADD COLUMN date TEXT NOT NULL DEFAULT '1970-01-01';`,
		`ALTER TABLE goals ADD COLUMN user_id INTEGER REFERENCES users(id) ON DELETE CASCADE;`,
		`ALTER TABLE goals ADD COLUMN category TEXT NOT NULL DEFAULT 'other';`,
		`ALTER TABLE goals ADD COLUMN "limit" REAL NOT NULL DEFAULT 0;`,
	}

	for _, stmt := range migrations {
		if _, err := DB.Exec(stmt); err != nil && !isIgnorableMigrationError(err) {
			return err
		}
	}

	return nil
}

func runAccountsV1Migration() error {
	applied, err := schemaMigrationApplied("accounts_v1")
	if err != nil {
		return err
	}
	if applied {
		return nil
	}

	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	accountMap, err := createDefaultAccounts(tx)
	if err != nil {
		return err
	}
	if err := rebuildExpensesTable(tx, accountMap); err != nil {
		return err
	}
	if err := rebuildIncomesTable(tx, accountMap); err != nil {
		return err
	}
	if err := rebuildGoalsTable(tx, accountMap); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO schema_migrations (name) VALUES ('accounts_v1')`); err != nil {
		return err
	}

	return tx.Commit()
}

func runExpenseSplitsV1Migration() error {
	applied, err := schemaMigrationApplied("expense_splits_v1")
	if err != nil {
		return err
	}
	if applied {
		return nil
	}

	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS expense_splits (
			expense_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			percentage REAL NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (expense_id, user_id),
			FOREIGN KEY(expense_id) REFERENCES expenses(id) ON DELETE CASCADE,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);
	`); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS recurring_expense_splits (
			recurring_expense_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			percentage REAL NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (recurring_expense_id, user_id),
			FOREIGN KEY(recurring_expense_id) REFERENCES recurring_expenses(id) ON DELETE CASCADE,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);
	`); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT OR IGNORE INTO expense_splits (expense_id, user_id, percentage)
		SELECT id, user_id, 100
		FROM expenses
		WHERE user_id IS NOT NULL;
	`); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT OR IGNORE INTO recurring_expense_splits (recurring_expense_id, user_id, percentage)
		SELECT id, user_id, 100
		FROM recurring_expenses
		WHERE user_id IS NOT NULL;
	`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO schema_migrations (name) VALUES ('expense_splits_v1')`); err != nil {
		return err
	}

	return tx.Commit()
}

func runRecurringExpensesV1Migration() error {
	applied, err := schemaMigrationApplied("recurring_expenses_v1")
	if err != nil || applied {
		return err
	}
	if _, err := DB.Exec(`ALTER TABLE expenses ADD COLUMN recurring_expense_id INTEGER`); err != nil && !isIgnorableMigrationError(err) {
		return err
	}
	_, err = DB.Exec(`INSERT INTO schema_migrations (name) VALUES ('recurring_expenses_v1')`)
	return err
}

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

// MaterializeRecurringExpenses creates expense rows for every recurring expense
// occurrence that is due (day_of_month ≤ today) and has not yet been recorded.
// Idempotent: skips months that already have a matching row.
func MaterializeRecurringExpenses(accountID int64) error {
	now := time.Now()
	today := now.Format("2006-01-02")

	type entry struct {
		id          int64
		userID      int64
		amount      float64
		description string
		category    string
		payment     string
		dayOfMonth  int
		startDate   string
		endDate     string
	}

	rows, err := DB.Query(`
		SELECT id, user_id, amount, description, category, payment, day_of_month, start_date, COALESCE(end_date, '')
		FROM recurring_expenses
		WHERE account_id = ? AND enabled = 1
	`, accountID)
	if err != nil {
		return err
	}

	var entries []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.id, &e.userID, &e.amount, &e.description, &e.category, &e.payment, &e.dayOfMonth, &e.startDate, &e.endDate); err != nil {
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
				SELECT COUNT(*) FROM expenses WHERE recurring_expense_id = ? AND date LIKE ?
			`, e.id, yearMonth).Scan(&count); err != nil {
				cursor = cursor.AddDate(0, 1, 0)
				continue
			}

			if count == 0 {
				if err := createExpenseFromRecurring(e.id, e.userID, accountID, e.amount, e.description, e.category, e.payment, targetDate); err != nil {
					log.Printf("materialize recurring %d for %s: %v", e.id, targetDate, err)
				}
			}

			cursor = cursor.AddDate(0, 1, 0)
		}
	}

	return nil
}

func createExpenseFromRecurring(recurringID, userID, accountID int64, amount float64, description, category, payment, date string) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`
		INSERT INTO expenses (user_id, account_id, amount, description, category, payment, date, recurring_expense_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, userID, accountID, amount, description, category, payment, date, recurringID)
	if err != nil {
		return err
	}

	expenseID, err := res.LastInsertId()
	if err != nil {
		return err
	}

	splitRows, err := tx.Query(`
		SELECT user_id, percentage FROM recurring_expense_splits WHERE recurring_expense_id = ?
	`, recurringID)
	if err != nil {
		return err
	}
	defer splitRows.Close()

	for splitRows.Next() {
		var splitUserID int64
		var pct float64
		if err := splitRows.Scan(&splitUserID, &pct); err != nil {
			return err
		}
		if _, err := tx.Exec(`
			INSERT INTO expense_splits (expense_id, user_id, percentage) VALUES (?, ?, ?)
		`, expenseID, splitUserID, pct); err != nil {
			return err
		}
	}
	if err := splitRows.Err(); err != nil {
		return err
	}

	return tx.Commit()
}

func schemaMigrationApplied(name string) (bool, error) {
	var exists int
	err := DB.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name = ?)
	`, name).Scan(&exists)
	return exists == 1, err
}

func createDefaultAccounts(tx *sql.Tx) (map[int64]int64, error) {
	rows, err := tx.Query(`SELECT id FROM users ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	accountMap := make(map[int64]int64)
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}

		accountID, err := CreateAccountWithOwner(tx, "Pessoal", userID)
		if err != nil {
			return nil, err
		}
		accountMap[userID] = accountID
	}

	return accountMap, rows.Err()
}

func rebuildExpensesTable(tx *sql.Tx, accountMap map[int64]int64) error {
	if _, err := tx.Exec(`DROP TABLE IF EXISTS expenses_new;`); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		CREATE TABLE expenses_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			account_id INTEGER NOT NULL,
			amount REAL NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			category TEXT NOT NULL,
			payment TEXT NOT NULL,
			date TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE
		);
	`); err != nil {
		return err
	}

	hasAccountID, err := tableHasColumn(tx, "expenses", "account_id")
	if err != nil {
		return err
	}

	query := `
		SELECT id, user_id, amount, description, category, payment, date, created_at
		FROM expenses
		ORDER BY id ASC
	`
	if hasAccountID {
		query = `
			SELECT id, user_id, account_id, amount, description, category, payment, date, created_at
			FROM expenses
			ORDER BY id ASC
		`
	}

	rows, err := tx.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	stmt, err := tx.Prepare(`
		INSERT INTO expenses_new (id, user_id, account_id, amount, description, category, payment, date, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for rows.Next() {
		var (
			id          int64
			userID      sql.NullInt64
			accountID   sql.NullInt64
			amount      float64
			description string
			category    string
			payment     string
			date        string
			createdAt   string
		)

		if hasAccountID {
			if err := rows.Scan(&id, &userID, &accountID, &amount, &description, &category, &payment, &date, &createdAt); err != nil {
				return err
			}
		} else {
			if err := rows.Scan(&id, &userID, &amount, &description, &category, &payment, &date, &createdAt); err != nil {
				return err
			}
		}

		resolvedAccountID, err := resolveAccountID(userID, accountID, accountMap)
		if err != nil {
			return err
		}
		if !userID.Valid || userID.Int64 <= 0 {
			return fmt.Errorf("expense %d sem user_id válido", id)
		}

		if _, err := stmt.Exec(id, userID.Int64, resolvedAccountID, amount, description, category, payment, date, createdAt); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if _, err := tx.Exec(`DROP TABLE expenses;`); err != nil {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE expenses_new RENAME TO expenses;`); err != nil {
		return err
	}
	return nil
}

func rebuildIncomesTable(tx *sql.Tx, accountMap map[int64]int64) error {
	if _, err := tx.Exec(`DROP TABLE IF EXISTS incomes_new;`); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		CREATE TABLE incomes_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			account_id INTEGER NOT NULL,
			amount REAL NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL,
			date TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE
		);
	`); err != nil {
		return err
	}

	hasAccountID, err := tableHasColumn(tx, "incomes", "account_id")
	if err != nil {
		return err
	}

	query := `
		SELECT id, user_id, amount, description, type, date, created_at
		FROM incomes
		ORDER BY id ASC
	`
	if hasAccountID {
		query = `
			SELECT id, user_id, account_id, amount, description, type, date, created_at
			FROM incomes
			ORDER BY id ASC
		`
	}

	rows, err := tx.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	stmt, err := tx.Prepare(`
		INSERT INTO incomes_new (id, user_id, account_id, amount, description, type, date, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for rows.Next() {
		var (
			id          int64
			userID      sql.NullInt64
			accountID   sql.NullInt64
			amount      float64
			description string
			typ         string
			date        string
			createdAt   string
		)

		if hasAccountID {
			if err := rows.Scan(&id, &userID, &accountID, &amount, &description, &typ, &date, &createdAt); err != nil {
				return err
			}
		} else {
			if err := rows.Scan(&id, &userID, &amount, &description, &typ, &date, &createdAt); err != nil {
				return err
			}
		}

		resolvedAccountID, err := resolveAccountID(userID, accountID, accountMap)
		if err != nil {
			return err
		}
		if !userID.Valid || userID.Int64 <= 0 {
			return fmt.Errorf("income %d sem user_id válido", id)
		}

		if _, err := stmt.Exec(id, userID.Int64, resolvedAccountID, amount, description, typ, date, createdAt); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if _, err := tx.Exec(`DROP TABLE incomes;`); err != nil {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE incomes_new RENAME TO incomes;`); err != nil {
		return err
	}
	return nil
}

func rebuildGoalsTable(tx *sql.Tx, accountMap map[int64]int64) error {
	if _, err := tx.Exec(`DROP TABLE IF EXISTS goals_new;`); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		CREATE TABLE goals_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			account_id INTEGER NOT NULL,
			category TEXT NOT NULL,
			"limit" REAL NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(account_id, category),
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE
		);
	`); err != nil {
		return err
	}

	hasAccountID, err := tableHasColumn(tx, "goals", "account_id")
	if err != nil {
		return err
	}

	query := `
		SELECT id, user_id, category, "limit", created_at
		FROM goals
		ORDER BY id ASC
	`
	if hasAccountID {
		query = `
			SELECT id, user_id, account_id, category, "limit", created_at
			FROM goals
			ORDER BY id ASC
		`
	}

	rows, err := tx.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	stmt, err := tx.Prepare(`
		INSERT INTO goals_new (id, user_id, account_id, category, "limit", created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for rows.Next() {
		var (
			id        int64
			userID    sql.NullInt64
			accountID sql.NullInt64
			category  string
			limit     float64
			createdAt string
		)

		if hasAccountID {
			if err := rows.Scan(&id, &userID, &accountID, &category, &limit, &createdAt); err != nil {
				return err
			}
		} else {
			if err := rows.Scan(&id, &userID, &category, &limit, &createdAt); err != nil {
				return err
			}
		}

		resolvedAccountID, err := resolveAccountID(userID, accountID, accountMap)
		if err != nil {
			return err
		}
		if !userID.Valid || userID.Int64 <= 0 {
			return fmt.Errorf("goal %d sem user_id válido", id)
		}

		if _, err := stmt.Exec(id, userID.Int64, resolvedAccountID, category, limit, createdAt); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if _, err := tx.Exec(`DROP TABLE goals;`); err != nil {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE goals_new RENAME TO goals;`); err != nil {
		return err
	}
	return nil
}

func resolveAccountID(userID, accountID sql.NullInt64, accountMap map[int64]int64) (int64, error) {
	if accountID.Valid && accountID.Int64 > 0 {
		return accountID.Int64, nil
	}
	if !userID.Valid || userID.Int64 <= 0 {
		return 0, errors.New("não foi possível resolver account_id sem user_id")
	}
	resolved, ok := accountMap[userID.Int64]
	if !ok {
		return 0, fmt.Errorf("conta pessoal não encontrada para user_id %d", userID.Int64)
	}
	return resolved, nil
}

func tableHasColumn(tx *sql.Tx, tableName, columnName string) (bool, error) {
	rows, err := tx.Query(`PRAGMA table_info(` + tableName + `);`)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			typ        string
			notNull    int
			defaultV   sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultV, &primaryKey); err != nil {
			return false, err
		}
		if name == columnName {
			return true, nil
		}
	}

	return false, rows.Err()
}

func isIgnorableMigrationError(err error) bool {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "duplicate column name"),
		strings.Contains(msg, "already exists"):
		return true
	default:
		return false
	}
}
