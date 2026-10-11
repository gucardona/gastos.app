package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func openTemp(t *testing.T) *sql.DB {
	t.Helper()
	d, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func mustExec(t *testing.T, d *sql.DB, q string) {
	t.Helper()
	if _, err := d.Exec(q); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

func TestOpenSetsUserVersion(t *testing.T) {
	d := openTemp(t)
	var v int
	if err := d.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != len(migrations)+1 {
		t.Fatalf("user_version = %d, want %d", v, len(migrations)+1)
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.db")
	for i := 0; i < 2; i++ {
		d, err := Open(path)
		if err != nil {
			t.Fatalf("abertura %d: %v", i, err)
		}
		d.Close()
	}
}

func TestPragmasOnEveryConnection(t *testing.T) {
	d := openTemp(t)
	ctx := context.Background()
	var conns []*sql.Conn
	for i := 0; i < 3; i++ {
		c, err := d.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		conns = append(conns, c)
	}
	for i, c := range conns {
		var fk, timeout int
		if err := c.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&fk); err != nil {
			t.Fatal(err)
		}
		if err := c.QueryRowContext(ctx, `PRAGMA busy_timeout`).Scan(&timeout); err != nil {
			t.Fatal(err)
		}
		if fk != 1 || timeout != 5000 {
			t.Errorf("conn %d: foreign_keys=%d busy_timeout=%d", i, fk, timeout)
		}
		c.Close()
	}
}

func TestDeleteWalletCascades(t *testing.T) {
	d := openTemp(t)
	mustExec(t, d, `INSERT INTO users (id, name, email, password_hash) VALUES (1, 'A', 'a@x', 'h')`)
	mustExec(t, d, `INSERT INTO wallets (id, name) VALUES (1, 'W')`)
	mustExec(t, d, `INSERT INTO wallet_members (wallet_id, user_id, role) VALUES (1, 1, 'owner')`)
	mustExec(t, d, `INSERT INTO categories (id, wallet_id, kind, name, icon, color, essentiality) VALUES (1, 1, 'expense', 'C', 'x', '#000000', 'essential')`)
	mustExec(t, d, `INSERT INTO transactions (wallet_id, kind, amount_cents, description, category_id, date, paid_by_user_id, created_by) VALUES (1, 'expense', 100, 'd', 1, '2026-10-01', 1, 1)`)
	mustExec(t, d, `DELETE FROM wallets WHERE id = 1`)
	for _, table := range []string{"wallet_members", "categories", "transactions"} {
		var n int
		d.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n)
		if n != 0 {
			t.Errorf("%s: %d linhas após apagar carteira", table, n)
		}
	}
}

func TestOneOwnerPerWallet(t *testing.T) {
	d := openTemp(t)
	mustExec(t, d, `INSERT INTO users (id, name, email, password_hash) VALUES (1,'A','a@x','h'), (2,'B','b@x','h')`)
	mustExec(t, d, `INSERT INTO wallets (id, name) VALUES (1, 'W')`)
	mustExec(t, d, `INSERT INTO wallet_members (wallet_id, user_id, role) VALUES (1, 1, 'owner')`)
	if _, err := d.Exec(`INSERT INTO wallet_members (wallet_id, user_id, role) VALUES (1, 2, 'owner')`); err == nil {
		t.Fatal("segundo owner deveria violar o índice único")
	}
}
