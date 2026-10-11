package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// migrations[i] takes the database from version i+1 to i+2. schema.sql is always version 1.
// To change the schema: append an item here; never edit old items.
var migrations = []string{}

const pragmas = "?_pragma=foreign_keys(1)" +
	"&_pragma=journal_mode(WAL)" +
	"&_pragma=busy_timeout(5000)" +
	"&_pragma=synchronous(NORMAL)" +
	"&_txlock=immediate"

// Open opens SQLite with pragmas applied on every pooled connection, and applies the schema.
func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	d, err := sql.Open("sqlite", path+pragmas)
	if err != nil {
		return nil, err
	}
	if err := d.Ping(); err != nil {
		d.Close()
		return nil, err
	}
	if err := migrate(d); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}

func migrate(d *sql.DB) error {
	var version int
	if err := d.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	steps := append([]string{schemaSQL}, migrations...)
	if version > len(steps) {
		return fmt.Errorf("database is at version %d, this binary only knows up to %d", version, len(steps))
	}
	for i := version; i < len(steps); i++ {
		tx, err := d.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(steps[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("schema v%d: %w", i+1, err)
		}
		if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, i+1)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
