package recurrence

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"
	_ "time/tzdata"

	"gastos/internal/db"
	"gastos/internal/events"
)

func setup(t *testing.T, now time.Time) (*Materializer, *sql.DB) {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	for _, q := range []string{
		`INSERT INTO users (id, name, email, password_hash) VALUES (1,'Ana','a@x','h'), (2,'Bia','b@x','h')`,
		`INSERT INTO wallets (id, name, splitting_enabled) VALUES (1, 'W', 1)`,
		`INSERT INTO wallet_members (wallet_id, user_id, role) VALUES (1,1,'owner'), (1,2,'editor')`,
		`INSERT INTO categories (id, wallet_id, kind, name, icon, color, essentiality) VALUES (1,1,'expense','Contas','x','#000000','essential')`,
	} {
		if _, err := d.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	m := NewMaterializer(d, events.New(), loc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	m.now = func() time.Time { return now }
	return m, d
}

func addExpenseRecurrence(t *testing.T, d *sql.DB, day int, start string) int64 {
	t.Helper()
	var id int64
	err := d.QueryRow(`INSERT INTO recurrences (wallet_id, kind, amount_cents, description, category_id, paid_by_user_id,
		day_of_month, start_date, created_by) VALUES (1, 'expense', 10000, '', 1, 1, ?, ?, 1) RETURNING id`, day, start).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func count(t *testing.T, d *sql.DB, q string) int {
	t.Helper()
	var n int
	if err := d.QueryRow(q).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRunCreatesDueOccurrences(t *testing.T) {
	m, d := setup(t, time.Date(2026, 10, 9, 15, 0, 0, 0, time.UTC))
	addExpenseRecurrence(t, d, 5, "2026-08-01")
	if err := m.Run(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	if n := count(t, d, `SELECT COUNT(*) FROM transactions`); n != 3 {
		t.Fatalf("%d transações, want 3", n)
	}
	var desc string
	d.QueryRow(`SELECT description FROM transactions LIMIT 1`).Scan(&desc)
	if desc != "Contas" {
		t.Fatalf("descrição vazia deveria virar o nome da categoria, veio %q", desc)
	}
}

func TestRunIsIdempotent(t *testing.T) {
	m, d := setup(t, time.Date(2026, 10, 9, 15, 0, 0, 0, time.UTC))
	addExpenseRecurrence(t, d, 5, "2026-08-01")
	for i := 0; i < 3; i++ {
		if err := m.Run(context.Background(), 0); err != nil {
			t.Fatal(err)
		}
	}
	if n := count(t, d, `SELECT COUNT(*) FROM transactions`); n != 3 {
		t.Fatalf("%d transações, want 3", n)
	}
}

func TestRunConcurrent(t *testing.T) {
	m, d := setup(t, time.Date(2026, 10, 9, 15, 0, 0, 0, time.UTC))
	addExpenseRecurrence(t, d, 5, "2026-08-01")
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- m.Run(context.Background(), 0)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if n := count(t, d, `SELECT COUNT(*) FROM transactions`); n != 3 {
		t.Fatalf("%d transações, want 3", n)
	}
}

func TestRunRespectsTimezone(t *testing.T) {
	// 05/10 02:00 UTC ainda é 04/10 em São Paulo: outubro não pode ser gerado.
	m, d := setup(t, time.Date(2026, 10, 5, 2, 0, 0, 0, time.UTC))
	addExpenseRecurrence(t, d, 5, "2026-10-01")
	if err := m.Run(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	if n := count(t, d, `SELECT COUNT(*) FROM transactions`); n != 0 {
		t.Fatalf("%d transações, want 0", n)
	}
}

func TestRunSettlement(t *testing.T) {
	m, d := setup(t, time.Date(2026, 10, 9, 15, 0, 0, 0, time.UTC))
	if _, err := d.Exec(`INSERT INTO recurrences (wallet_id, kind, amount_cents, paid_by_user_id, to_user_id, day_of_month, start_date, created_by)
		VALUES (1, 'settlement', 50000, 2, 1, 1, '2026-09-01', 1)`); err != nil {
		t.Fatal(err)
	}
	if err := m.Run(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	if n := count(t, d, `SELECT COUNT(*) FROM settlements WHERE from_user_id = 2 AND to_user_id = 1`); n != 2 {
		t.Fatalf("%d acertos, want 2", n)
	}
}

func TestRunCopiesSplits(t *testing.T) {
	m, d := setup(t, time.Date(2026, 10, 9, 15, 0, 0, 0, time.UTC))
	id := addExpenseRecurrence(t, d, 5, "2026-10-01")
	if _, err := d.Exec(`INSERT INTO recurrence_splits (recurrence_id, user_id, share_bp) VALUES (?, 1, 7000), (?, 2, 3000)`, id, id); err != nil {
		t.Fatal(err)
	}
	if err := m.Run(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	var bp int
	if err := d.QueryRow(`SELECT share_bp FROM transaction_splits WHERE user_id = 1`).Scan(&bp); err != nil || bp != 7000 {
		t.Fatalf("share_bp = %d, err = %v", bp, err)
	}
}

func TestDisabledSkipped(t *testing.T) {
	m, d := setup(t, time.Date(2026, 10, 9, 15, 0, 0, 0, time.UTC))
	id := addExpenseRecurrence(t, d, 5, "2026-08-01")
	d.Exec(`UPDATE recurrences SET enabled = 0 WHERE id = ?`, id)
	if err := m.Run(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	if n := count(t, d, `SELECT COUNT(*) FROM transactions`); n != 0 {
		t.Fatalf("%d transações, want 0", n)
	}
}

func TestRunNotifiesWallet(t *testing.T) {
	m, d := setup(t, time.Date(2026, 10, 9, 15, 0, 0, 0, time.UTC))
	addExpenseRecurrence(t, d, 5, "2026-10-01")
	ch := m.bus.Subscribe(1, 1)
	if err := m.Run(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("esperava notificação")
	}
}
