package db

import (
	"context"
	"gastos/internal/testutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBackupRotation(t *testing.T) {
	d := openTemp(t)
	dir := testutil.TempDir(t)
	base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	var last string
	for i := 0; i < 3; i++ {
		p, err := Backup(context.Background(), d, dir, 2, base.Add(time.Duration(i)*time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		last = p
	}
	files, _ := filepath.Glob(filepath.Join(dir, backupGlob))
	if len(files) != 2 {
		t.Fatalf("sobraram %d backups, want 2: %v", len(files), files)
	}
	if _, err := os.Stat(last); err != nil {
		t.Fatalf("the newest should exist: %v", err)
	}
}

func TestBackupIsOpenable(t *testing.T) {
	d := openTemp(t)
	mustExec(t, d, `INSERT INTO users (id, name, email, password_hash) VALUES (1, 'A', 'a@x', 'h')`)
	p, err := Backup(context.Background(), d, testutil.TempDir(t), 5, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	b, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	var n int
	b.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	if n != 1 {
		t.Fatalf("backup has %d users, want 1", n)
	}
}

func TestBackupNeverOverwrites(t *testing.T) {
	d := openTemp(t)
	dir := testutil.TempDir(t)
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	if _, err := Backup(context.Background(), d, dir, 5, now); err != nil {
		t.Fatal(err)
	}
	if _, err := Backup(context.Background(), d, dir, 5, now); err == nil {
		t.Fatal("a second backup at the same instant should fail")
	}
}

func TestLastBackupAge(t *testing.T) {
	dir := testutil.TempDir(t)
	if LastBackupAge(dir, time.Now()) < 1000*time.Hour {
		t.Fatal("with no backups it should return a huge age")
	}
	d := openTemp(t)
	if _, err := Backup(context.Background(), d, dir, 5, time.Now()); err != nil {
		t.Fatal(err)
	}
	if age := LastBackupAge(dir, time.Now()); age > time.Minute {
		t.Fatalf("age %v, expected ~0", age)
	}
}

func TestBackupFailsWhenDirectoryCannotBeCreated(t *testing.T) {
	d := openTemp(t)
	file := filepath.Join(testutil.TempDir(t), "arquivo")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Backup(context.Background(), d, filepath.Join(file, "backups"), 3, time.Now()); err == nil {
		t.Fatal("should fail when the backup directory cannot be created")
	}
}

func TestBackupHonoursCancelledContext(t *testing.T) {
	d := openTemp(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Backup(ctx, d, testutil.TempDir(t), 3, time.Now()); err == nil {
		t.Fatal("a cancelled context should interrupt the backup")
	}
}
