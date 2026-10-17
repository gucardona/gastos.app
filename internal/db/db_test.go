package db

import (
	"context"
	"database/sql"
	"fmt"
	"gastos/internal/testutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func openTemp(t *testing.T) *sql.DB {
	t.Helper()
	d, err := Open(filepath.Join(testutil.TempDir(t), "t.db"))
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
			t.Errorf("%s: %d rows after deleting the wallet", table, n)
		}
	}
}

func TestOneOwnerPerWallet(t *testing.T) {
	d := openTemp(t)
	mustExec(t, d, `INSERT INTO users (id, name, email, password_hash) VALUES (1,'A','a@x','h'), (2,'B','b@x','h')`)
	mustExec(t, d, `INSERT INTO wallets (id, name) VALUES (1, 'W')`)
	mustExec(t, d, `INSERT INTO wallet_members (wallet_id, user_id, role) VALUES (1, 1, 'owner')`)
	if _, err := d.Exec(`INSERT INTO wallet_members (wallet_id, user_id, role) VALUES (1, 2, 'owner')`); err == nil {
		t.Fatal("a second owner should violate the unique index")
	}
}

func TestOpenFailsWhenParentIsAFile(t *testing.T) {
	file := filepath.Join(testutil.TempDir(t), "arquivo")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(filepath.Join(file, "sub", "g.db")); err == nil {
		t.Fatal("should fail when the parent directory is a file")
	}
}

func TestOpenFailsOnACorruptFile(t *testing.T) {
	path := filepath.Join(testutil.TempDir(t), "t.db")
	if err := os.WriteFile(path, []byte("this is not a sqlite database, it is just text garbage with more than a hundred bytes to get past the header..........."), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil {
		t.Fatal("a corrupt file should fail on open")
	}
}

func TestOpenCreatesFullSchema(t *testing.T) {
	d := openTemp(t)
	for _, table := range []string{"bank_accounts", "bank_transactions", "categorization_rules", "ai_settings", "chat_messages", "ai_usage", "fx_rate"} {
		var n int
		d.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&n)
		if n != 1 {
			t.Errorf("table %s missing", table)
		}
	}
	var ddl string
	if err := d.QueryRow(`SELECT sql FROM sqlite_master WHERE name='transactions'`).Scan(&ddl); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ddl, "source") || !strings.Contains(ddl, "'manual','recurring','bank'") {
		t.Errorf("transactions.source with its CHECK missing: %s", ddl)
	}
	var v int
	d.QueryRow(`PRAGMA user_version`).Scan(&v)
	if v != latestVersion() {
		t.Errorf("user_version = %d, want %d", v, latestVersion())
	}
}

func TestOpenRefusesOldSchemaVersions(t *testing.T) {
	for _, old := range []int{1, 2} {
		path := filepath.Join(testutil.TempDir(t), "t.db")
		raw, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		mustExec(t, raw, `CREATE TABLE marker (x INTEGER)`)
		mustExec(t, raw, fmt.Sprintf(`PRAGMA user_version = %d`, old))
		raw.Close()

		_, err = Open(path)
		if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("schema version %d", old)) {
			t.Fatalf("v%d: want a schema version error, got %v", old, err)
		}
		raw, err = sql.Open("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		var v, n int
		raw.QueryRow(`PRAGMA user_version`).Scan(&v)
		raw.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name <> 'marker'`).Scan(&n)
		raw.Close()
		if v != old || n != 0 {
			t.Errorf("v%d database was modified: version %d, extra objects %d", old, v, n)
		}
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(testutil.TempDir(t), "t.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, d, `INSERT INTO users (id, name, email, password_hash) VALUES (1, 'A', 'a@x', 'h')`)
	d.Close()
	d, err = Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer d.Close()
	var n int
	d.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	if n != 1 {
		t.Fatalf("users = %d after reopen, want 1", n)
	}
}

func TestOpenRefusesNewerSchemaVersion(t *testing.T) {
	path := filepath.Join(testutil.TempDir(t), "t.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	newer := latestVersion() + 1
	mustExec(t, raw, fmt.Sprintf(`PRAGMA user_version = %d`, newer))
	raw.Close()
	if _, err := Open(path); err == nil || !strings.Contains(err.Error(), fmt.Sprintf("schema version %d", newer)) {
		t.Fatalf("want a schema version error, got %v", err)
	}
}

// TestOpenUpgradesVersion3InPlace mirrors production: a v3 database holding real rows is migrated, never recreated.
func TestOpenUpgradesVersion3InPlace(t *testing.T) {
	path := filepath.Join(testutil.TempDir(t), "t.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, raw, schemaSQL)
	mustExec(t, raw, `PRAGMA user_version = 3`)
	mustExec(t, raw, `INSERT INTO users (id, name, email, password_hash) VALUES (1, 'A', 'a@x', 'h'), (2, 'B', 'b@x', 'h')`)
	mustExec(t, raw, `INSERT INTO wallets (id, name) VALUES (1, 'W')`)
	mustExec(t, raw, `INSERT INTO wallet_members (wallet_id, user_id, role) VALUES (1, 1, 'owner')`)
	mustExec(t, raw, `INSERT INTO ai_settings (user_id, api_key_enc, key_hint, model, enabled, retention_days) VALUES (1, x'0102', '1234', 'claude-opus-5-5', 1, 9)`)
	mustExec(t, raw, `INSERT INTO chat_messages (user_id, wallet_id, role, content) VALUES (1, 1, 'user', 'oi')`)
	raw.Close()

	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	var v, users, wallets, msgs int
	d.QueryRow(`PRAGMA user_version`).Scan(&v)
	if v != 4 {
		t.Fatalf("user_version = %d, want 4", v)
	}
	d.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&users)
	d.QueryRow(`SELECT COUNT(*) FROM wallet_members`).Scan(&wallets)
	d.QueryRow(`SELECT COUNT(*) FROM chat_messages WHERE content = 'oi'`).Scan(&msgs)
	if users != 2 || wallets != 1 || msgs != 1 {
		t.Fatalf("rows lost: users %d, members %d, messages %d", users, wallets, msgs)
	}
	var enc []byte
	var model string
	var retention int
	var daily, monthly sql.NullInt64
	if err := d.QueryRow(`SELECT api_key_enc, model, retention_days, daily_limit_cents, monthly_limit_cents FROM ai_settings WHERE user_id = 1`).
		Scan(&enc, &model, &retention, &daily, &monthly); err != nil {
		t.Fatal(err)
	}
	if string(enc) != "\x01\x02" || model != "claude-opus-5-5" || retention != 9 || daily.Valid || monthly.Valid {
		t.Fatalf("ai_settings after upgrade: %x %s %d %v %v", enc, model, retention, daily, monthly)
	}
	mustExec(t, d, `UPDATE ai_settings SET daily_limit_cents = 500, monthly_limit_cents = 10000 WHERE user_id = 1`)
	for _, col := range []string{"daily_limit_cents", "monthly_limit_cents"} {
		if _, err := d.Exec(`UPDATE ai_settings SET ` + col + ` = 0 WHERE user_id = 1`); err == nil {
			t.Errorf("%s = 0 should violate the CHECK", col)
		}
	}
	mustExec(t, d, `INSERT INTO ai_usage (user_id, day, cost_micro_brl, input_tokens, output_tokens) VALUES (1, '2026-10-17', 1, 2, 3)`)
	mustExec(t, d, `INSERT INTO fx_rate (id, usd_brl_micro, fetched_at) VALUES (1, 5063100, '2026-10-17T00:00:00Z')`)
	if _, err := d.Exec(`INSERT INTO fx_rate (id, usd_brl_micro, fetched_at) VALUES (2, 1, 'x')`); err == nil {
		t.Error("fx_rate should hold a single row")
	}
	mustExec(t, d, `DELETE FROM users WHERE id = 1`)
	var usage int
	d.QueryRow(`SELECT COUNT(*) FROM ai_usage`).Scan(&usage)
	if usage != 0 {
		t.Error("ai_usage should cascade on user delete")
	}
}

func TestFailedMigrationKeepsVersion(t *testing.T) {
	path := filepath.Join(testutil.TempDir(t), "t.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, raw, schemaSQL)
	mustExec(t, raw, `CREATE TABLE ai_usage (x INTEGER)`) // collides with migration 0004
	mustExec(t, raw, `PRAGMA user_version = 3`)
	raw.Close()
	if _, err := Open(path); err == nil {
		t.Fatal("the colliding migration should fail")
	}
	raw, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	var v, cols int
	raw.QueryRow(`PRAGMA user_version`).Scan(&v)
	raw.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('ai_settings') WHERE name = 'daily_limit_cents'`).Scan(&cols)
	if v != 3 || cols != 0 {
		t.Fatalf("partial migration: version %d, new columns %d", v, cols)
	}
}

func TestAISettingsConstraints(t *testing.T) {
	d := openTemp(t)
	mustExec(t, d, `INSERT INTO users (id, name, email, password_hash) VALUES (1, 'A', 'a@x', 'h')`)
	ins := func(retention int, model string) error {
		_, err := d.Exec(`INSERT OR REPLACE INTO ai_settings (user_id, api_key_enc, key_hint, model, retention_days) VALUES (1, x'00', '1234', ?, ?)`, model, retention)
		return err
	}
	for _, r := range []int{0, 31} {
		if ins(r, "claude-sonnet-5-5") == nil {
			t.Errorf("retention %d should be refused", r)
		}
	}
	if ins(7, "gpt-4") == nil {
		t.Error("unknown model should be refused")
	}
	for _, r := range []int{1, 30} {
		if err := ins(r, "claude-opus-5-5"); err != nil {
			t.Errorf("retention %d: %v", r, err)
		}
	}
}
