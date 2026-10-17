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

// schemaVersion is the user_version of the single schema.sql. Bump it when the schema changes.
const schemaVersion = 4

// applySchema runs schema.sql on an empty database (user_version 0) and accepts a current one;
// any other version is refused so the server fails to start instead of running on a missing schema.
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
	if version == schemaVersion {
		return nil
	}
	if version != 0 {
		return fmt.Errorf("database is at schema version %d, this binary expects %d: recreate it (see docs/assistente.md)", version, schemaVersion)
	}
	if _, err := tx.Exec(schemaSQL); err != nil {
		return err
	}
	if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, schemaVersion)); err != nil {
		return err
	}
	return tx.Commit()
}
