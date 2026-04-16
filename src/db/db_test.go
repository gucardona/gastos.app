package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestAccountsV1MigrationBackfillsPersonalAccounts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")

	legacyDB, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}

	legacySchema := []string{
		`PRAGMA foreign_keys = ON;`,
		`CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE expenses (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			amount REAL NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			category TEXT NOT NULL,
			payment TEXT NOT NULL,
			date TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE incomes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			amount REAL NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL,
			date TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE goals (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			category TEXT NOT NULL,
			"limit" REAL NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, category),
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
	}
	for _, stmt := range legacySchema {
		if _, err := legacyDB.Exec(stmt); err != nil {
			t.Fatalf("create legacy schema: %v", err)
		}
	}

	if _, err := legacyDB.Exec(`INSERT INTO users (id, name, email, password) VALUES (1, 'Alice', 'alice@example.com', 'hash')`); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := legacyDB.Exec(`INSERT INTO expenses (user_id, amount, description, category, payment, date) VALUES (1, 10, 'Cafe', 'food', 'pix', '2026-04-10')`); err != nil {
		t.Fatalf("insert expense: %v", err)
	}
	if _, err := legacyDB.Exec(`INSERT INTO incomes (user_id, amount, description, type, date) VALUES (1, 100, 'Salario', 'salary', '2026-04-01')`); err != nil {
		t.Fatalf("insert income: %v", err)
	}
	if _, err := legacyDB.Exec(`INSERT INTO goals (user_id, category, "limit") VALUES (1, 'food', 500)`); err != nil {
		t.Fatalf("insert goal: %v", err)
	}
	if err := legacyDB.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	Init(path)
	t.Cleanup(func() {
		if DB != nil {
			_ = DB.Close()
			DB = nil
		}
	})

	var accountsCount int
	if err := DB.QueryRow(`SELECT COUNT(*) FROM accounts`).Scan(&accountsCount); err != nil {
		t.Fatalf("count accounts: %v", err)
	}
	if accountsCount != 1 {
		t.Fatalf("expected 1 default account, got %d", accountsCount)
	}

	var personalAccountID int64
	if err := DB.QueryRow(`
		SELECT account_id
		FROM account_members
		WHERE user_id = 1 AND role = 'owner'
	`).Scan(&personalAccountID); err != nil {
		t.Fatalf("find personal account: %v", err)
	}

	for _, table := range []string{"expenses", "incomes", "goals"} {
		var accountID int64
		if err := DB.QueryRow(`SELECT account_id FROM ` + table + ` LIMIT 1`).Scan(&accountID); err != nil {
			t.Fatalf("read account_id from %s: %v", table, err)
		}
		if accountID != personalAccountID {
			t.Fatalf("%s account_id = %d, want %d", table, accountID, personalAccountID)
		}
	}

	var migrationCount int
	if err := DB.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE name = 'accounts_v1'`).Scan(&migrationCount); err != nil {
		t.Fatalf("count schema migrations: %v", err)
	}
	if migrationCount != 1 {
		t.Fatalf("expected accounts_v1 migration to be recorded once, got %d", migrationCount)
	}

	tx, err := DB.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	secondAccountID, err := CreateAccountWithOwner(tx, "Empresa", 1)
	if err != nil {
		t.Fatalf("create second account: %v", err)
	}
	if _, err := tx.Exec(`
		INSERT INTO goals (user_id, account_id, category, "limit")
		VALUES (1, ?, 'food', 900)
	`, secondAccountID); err != nil {
		t.Fatalf("insert goal in second account: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	var goalsCount int
	if err := DB.QueryRow(`SELECT COUNT(*) FROM goals WHERE category = 'food'`).Scan(&goalsCount); err != nil {
		t.Fatalf("count goals: %v", err)
	}
	if goalsCount != 2 {
		t.Fatalf("expected same category goal to exist in two accounts, got %d rows", goalsCount)
	}
}
