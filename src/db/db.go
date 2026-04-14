package db

import (
	"database/sql"
	"log"
	"strings"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

func Init(path string) {
	var err error
	DB, err = sql.Open("sqlite", path)
	if err != nil {
		log.Fatal(err)
	}

	if err := DB.Ping(); err != nil {
		log.Fatal(err)
	}

	enableForeignKeys()
	createTables()
	runMigrations()
}

func enableForeignKeys() {
	if _, err := DB.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		log.Fatal(err)
	}
}

func createTables() {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS expenses (
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
		`CREATE TABLE IF NOT EXISTS incomes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			amount REAL NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL,
			date TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS goals (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			category TEXT NOT NULL,
			"limit" REAL NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, category),
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
	}

	for _, q := range queries {
		if _, err := DB.Exec(q); err != nil {
			log.Fatal(err)
		}
	}
}

func runMigrations() {
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
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_goals_user_category ON goals(user_id, category);`,
		`CREATE INDEX IF NOT EXISTS idx_expenses_user_date ON expenses(user_id, date DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_incomes_user_date ON incomes(user_id, date DESC);`,
	}

	for _, stmt := range migrations {
		if _, err := DB.Exec(stmt); err != nil && !isIgnorableMigrationError(err) {
			log.Fatal(err)
		}
	}
}

func isIgnorableMigrationError(err error) bool {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "duplicate column name"),
		strings.Contains(msg, "already exists"):
		return true
	default:
		return false
	}
}
