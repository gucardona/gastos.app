package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const backupGlob = "gastos-*.db"

// Backup grava uma cópia consistente em dir com timestamp e mantém só as keep mais recentes.
// Nunca sobrescreve: VACUUM INTO falha se o destino já existe.
func Backup(ctx context.Context, d *sql.DB, dir string, keep int, now time.Time) (string, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "gastos-"+now.UTC().Format("20060102-150405")+".db")
	if _, err := d.ExecContext(ctx, `VACUUM INTO ?`, path); err != nil {
		return "", err
	}
	matches, err := filepath.Glob(filepath.Join(dir, backupGlob))
	if err != nil {
		return path, err
	}
	sort.Strings(matches) // timestamp no nome: ordem lexicográfica = cronológica
	for len(matches) > keep {
		os.Remove(matches[0])
		matches = matches[1:]
	}
	return path, nil
}

// LastBackupAge devolve há quanto tempo foi o último backup, ou um valor enorme se não há nenhum.
func LastBackupAge(dir string, now time.Time) time.Duration {
	matches, _ := filepath.Glob(filepath.Join(dir, backupGlob))
	if len(matches) == 0 {
		return 1<<63 - 1
	}
	sort.Strings(matches)
	info, err := os.Stat(matches[len(matches)-1])
	if err != nil {
		return 1<<63 - 1
	}
	return now.Sub(info.ModTime())
}
