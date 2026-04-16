package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

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

	return accountID, nil
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
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			name TEXT PRIMARY KEY,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
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
		`CREATE INDEX IF NOT EXISTS idx_incomes_account_date ON incomes(account_id, date DESC);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_goals_account_category ON goals(account_id, category);`,
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
	return runAccountsV1Migration()
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
