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

// baseVersion is the user_version of schema.sql, the frozen baseline. Schema changes after it are migrations.
const baseVersion = 3

//go:embed migrations/0004_usage_limits.sql
var migration0004 string

// migrations[i] takes the database from version baseVersion+i to baseVersion+i+1. Only append to it.
var migrations = []string{migration0004}

func latestVersion() int { return baseVersion + len(migrations) }

// applySchema builds an empty database (user_version 0) from schema.sql and brings a baseline-or-later one
// up to date, one step per transaction so a failed step leaves the previous version intact. Any other
// version is refused so the server fails to start instead of running on an unknown schema.
func applySchema(d *sql.DB) error {
	for {
		done, err := applyStep(d)
		if err != nil || done {
			return err
		}
	}
}

func applyStep(d *sql.DB) (done bool, err error) {
	tx, err := d.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var version int
	if err := tx.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return false, err
	}
	var step string
	switch {
	case version == latestVersion():
		return true, nil
	case version == 0:
		step = schemaSQL
	case version >= baseVersion && version < latestVersion():
		step = migrations[version-baseVersion]
	default:
		return false, fmt.Errorf("database is at schema version %d, this binary knows versions %d to %d (or an empty database): refusing to open it",
			version, baseVersion, latestVersion())
	}
	next := version + 1
	if version == 0 {
		next = baseVersion
	}
	if _, err := tx.Exec(step); err != nil {
		return false, fmt.Errorf("migrate schema from version %d: %w", version, err)
	}
	if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, next)); err != nil {
		return false, err
	}
	return false, tx.Commit()
}
