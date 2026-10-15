package db

import (
	"database/sql"
	_ "embed"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

const pragmas = "?_pragma=foreign_keys(1)" +
	"&_pragma=journal_mode(WAL)" +
	"&_pragma=busy_timeout(5000)" +
	"&_pragma=synchronous(NORMAL)" +
	"&_txlock=immediate"

// Open opens SQLite with pragmas applied on every pooled connection, and creates the schema on an empty database.
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
	if err := applySchema(d); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}

// applySchema runs schema.sql once, on a database whose user_version is 0.
func applySchema(d *sql.DB) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version int
	if err := tx.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version != 0 {
		return nil
	}
	if _, err := tx.Exec(schemaSQL); err != nil {
		return err
	}
	if _, err := tx.Exec(`PRAGMA user_version = 1`); err != nil {
		return err
	}
	return tx.Commit()
}
