package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	"gastos/internal/testutil"
)

// createV1 builds a database that is exactly at schema version 1, as production will be when this ships.
func createV1(t *testing.T, path string) {
	t.Helper()
	d, err := sql.Open("sqlite", path+pragmas)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	mustExec(t, d, schemaSQL)
	mustExec(t, d, `PRAGMA user_version = 1`)
	mustExec(t, d, `INSERT INTO users (id, name, email, password_hash) VALUES (1, 'Ana', 'ana@example.com', 'h')`)
	mustExec(t, d, `INSERT INTO wallets (id, name) VALUES (1, 'Casa')`)
	mustExec(t, d, `INSERT INTO wallet_members (wallet_id, user_id, role) VALUES (1, 1, 'owner')`)
	mustExec(t, d, `INSERT INTO categories (id, wallet_id, kind, name, icon, color, essentiality) VALUES (1, 1, 'expense', 'Mercado', 'x', '#000000', 'essential')`)
	mustExec(t, d, `INSERT INTO recurrences (id, wallet_id, kind, amount_cents, category_id, paid_by_user_id, day_of_month, start_date, created_by)
		VALUES (1, 1, 'expense', 5000, 1, 1, 5, '2026-01-01', 1)`)
	mustExec(t, d, `INSERT INTO transactions (id, wallet_id, kind, amount_cents, description, category_id, date, paid_by_user_id, created_by)
		VALUES (1, 1, 'expense', 1000, 'typed by hand', 1, '2026-10-01', 1, 1)`)
	mustExec(t, d, `INSERT INTO transactions (id, wallet_id, kind, amount_cents, description, category_id, date, paid_by_user_id, created_by, recurrence_id, occurrence)
		VALUES (2, 1, 'expense', 5000, 'generated', 1, '2026-10-05', 1, 1, 1, '2026-10')`)
}

func TestUpgradeFromV1KeepsDataAndBackfillsSource(t *testing.T) {
	path := filepath.Join(testutil.TempDir(t), "t.db")
	createV1(t, path)

	d, err := Open(path)
	if err != nil {
		t.Fatalf("upgrading a v1 database: %v", err)
	}
	defer d.Close()

	var version int
	d.QueryRow(`PRAGMA user_version`).Scan(&version)
	if version != len(migrations)+1 {
		t.Fatalf("user_version = %d, want %d", version, len(migrations)+1)
	}
	sources := map[int64]string{}
	rows, err := d.Query(`SELECT id, source FROM transactions ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var src string
		rows.Scan(&id, &src)
		sources[id] = src
	}
	if sources[1] != "manual" || sources[2] != "recurring" {
		t.Fatalf("sources after the upgrade: %v, want 1=manual and 2=recurring", sources)
	}
}

func TestUpgradeIsIdempotent(t *testing.T) {
	path := filepath.Join(testutil.TempDir(t), "t.db")
	createV1(t, path)
	for i := 0; i < 3; i++ {
		d, err := Open(path)
		if err != nil {
			t.Fatalf("open %d: %v", i+1, err)
		}
		d.Close()
	}
}

func bankFixture(t *testing.T, d *sql.DB) {
	t.Helper()
	mustExec(t, d, `INSERT INTO users (id, name, email, password_hash) VALUES (1, 'Ana', 'ana@example.com', 'h')`)
	mustExec(t, d, `INSERT INTO wallets (id, name) VALUES (1, 'Casa')`)
	mustExec(t, d, `INSERT INTO wallet_members (wallet_id, user_id, role) VALUES (1, 1, 'owner')`)
	mustExec(t, d, `INSERT INTO categories (id, wallet_id, kind, name, icon, color, essentiality) VALUES (1, 1, 'expense', 'Mercado', 'x', '#000000', 'essential')`)
	mustExec(t, d, `INSERT INTO bank_accounts (id, wallet_id, name, paid_by_user_id) VALUES (1, 1, 'Conta corrente', 1)`)
}

func TestBankTransactionsAreUniquePerAccountAndExternalID(t *testing.T) {
	d := openTemp(t)
	bankFixture(t, d)
	insert := `INSERT INTO bank_transactions (bank_account_id, external_id, date, amount_cents, description) VALUES (1, 'FIT1', '2026-10-01', -4890, 'MERCADO')`
	mustExec(t, d, insert)
	if _, err := d.Exec(insert); err == nil {
		t.Fatal("the same external id on the same account must be rejected (re-importing a file must not duplicate)")
	}
	mustExec(t, d, `INSERT INTO bank_accounts (id, wallet_id, name, paid_by_user_id) VALUES (2, 1, 'Cartao', 1)`)
	mustExec(t, d, `INSERT INTO bank_transactions (bank_account_id, external_id, date, amount_cents, description) VALUES (2, 'FIT1', '2026-10-01', -4890, 'MERCADO')`)
}

func TestBankTransactionStatusIsRestricted(t *testing.T) {
	d := openTemp(t)
	bankFixture(t, d)
	if _, err := d.Exec(`INSERT INTO bank_transactions (bank_account_id, external_id, date, amount_cents, description, status) VALUES (1, 'X', '2026-10-01', -1, 'x', 'whatever')`); err == nil {
		t.Fatal("an unknown status must be rejected")
	}
	if _, err := d.Exec(`INSERT INTO bank_transactions (bank_account_id, external_id, date, amount_cents, description) VALUES (1, 'Y', '2026-10-01', 0, 'zero')`); err == nil {
		t.Fatal("a zero amount must be rejected")
	}
	var status string
	mustExec(t, d, `INSERT INTO bank_transactions (bank_account_id, external_id, date, amount_cents, description) VALUES (1, 'Z', '2026-10-01', -1, 'ok')`)
	d.QueryRow(`SELECT status FROM bank_transactions WHERE external_id = 'Z'`).Scan(&status)
	if status != "pending" {
		t.Fatalf("default status = %q, want pending", status)
	}
}

func TestDeletingAWalletRemovesItsBankData(t *testing.T) {
	d := openTemp(t)
	bankFixture(t, d)
	mustExec(t, d, `INSERT INTO bank_transactions (bank_account_id, external_id, date, amount_cents, description) VALUES (1, 'A', '2026-10-01', -100, 'x')`)
	mustExec(t, d, `INSERT INTO categorization_rules (wallet_id, pattern, category_id) VALUES (1, 'mercado', 1)`)
	mustExec(t, d, `DELETE FROM wallets WHERE id = 1`)
	for _, table := range []string{"bank_accounts", "bank_transactions", "categorization_rules"} {
		var n int
		d.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n)
		if n != 0 {
			t.Errorf("%s: %d rows left after deleting the wallet", table, n)
		}
	}
}

func TestRulePatternIsUniquePerWallet(t *testing.T) {
	d := openTemp(t)
	bankFixture(t, d)
	mustExec(t, d, `INSERT INTO categorization_rules (wallet_id, pattern, category_id) VALUES (1, 'uber', 1)`)
	if _, err := d.Exec(`INSERT INTO categorization_rules (wallet_id, pattern, category_id) VALUES (1, 'uber', 1)`); err == nil {
		t.Fatal("the same pattern twice in a wallet must be rejected")
	}
}
