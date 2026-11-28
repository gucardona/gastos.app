package ledger_test

import (
	"testing"
	"time"

	"gastos/internal/api/apitest"
	"gastos/internal/api/ledger"
)

// upcomingApp freezes "today" at 2026-11-14 (São Paulo). Recurrences are inserted straight into the database so the
// materializer (which runs on the real clock) never touches them: the summary alone decides what is still to come.
func upcomingApp(t *testing.T) (*apitest.App, *apitest.Client) {
	t.Helper()
	app := apitest.New(t)
	loc := app.Server().Cfg.Location
	app.SetNow(func() time.Time { return time.Date(2026, 11, 14, 12, 0, 0, 0, loc) })
	return app, app.Register("Ana", "ana@example.com")
}

// recRow is a recurrence to insert straight into the database.
type recRow struct {
	kind, description, start, end string
	amount                        int64
	day                           int
	enabled                       bool
}

// insertRecurrence stores r and returns its id.
func insertRecurrence(t *testing.T, c *apitest.Client, walletID int64, r recRow) int64 {
	t.Helper()
	var cat, to any
	if r.kind == "settlement" {
		to = c.User.ID
	} else {
		var err error
		if err = c.App().Server().DB.QueryRow(`SELECT id FROM categories WHERE wallet_id = ? AND kind = ? ORDER BY id LIMIT 1`, walletID, r.kind).Scan(&cat); err != nil {
			t.Fatal(err)
		}
	}
	var end any
	if r.end != "" {
		end = r.end
	}
	var id int64
	if err := c.App().Server().DB.QueryRow(`INSERT INTO recurrences (wallet_id, kind, amount_cents, description, category_id, paid_by_user_id,
		to_user_id, day_of_month, start_date, end_date, enabled, created_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		walletID, r.kind, r.amount, r.description, cat, c.User.ID, to, r.day, r.start, end, r.enabled, c.User.ID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// getSummary fetches the summary of month.
func getSummary(c *apitest.Client, month string) ledger.SummaryDTO {
	c.App().T().Helper()
	var sum ledger.SummaryDTO
	c.Do("GET", c.WPath("/summary?month=%s", month), nil, 200, &sum)
	return sum
}

func TestSummaryUpcomingMidMonth(t *testing.T) {
	app, ana := upcomingApp(t)
	w := ana.WalletID
	salary15 := insertRecurrence(t, ana, w, recRow{kind: "income", description: "Salário", start: "2026-01-01", amount: 500000, day: 15, enabled: true})
	insertRecurrence(t, ana, w, recRow{kind: "income", description: "Salário", start: "2026-01-01", amount: 300000, day: 10, enabled: true})                // before today
	insertRecurrence(t, ana, w, recRow{kind: "expense", start: "2026-01-01", amount: 999, day: 14, enabled: true})                                          // today: already materialized
	internet := insertRecurrence(t, ana, w, recRow{kind: "expense", start: "2026-01-01", amount: 8990, day: 31, enabled: true})                             // clamped to 30
	insertRecurrence(t, ana, w, recRow{kind: "expense", description: "Curso", start: "2026-01-01", end: "2026-11-25", amount: 100, day: 25, enabled: true}) // ends on its last day
	insertRecurrence(t, ana, w, recRow{kind: "expense", description: "Terminou", start: "2026-01-01", end: "2026-11-20", amount: 100, day: 25, enabled: true})
	insertRecurrence(t, ana, w, recRow{kind: "expense", description: "Desligada", start: "2026-01-01", amount: 100, day: 20, enabled: false})
	insertRecurrence(t, ana, w, recRow{kind: "settlement", start: "2026-01-01", amount: 100, day: 20, enabled: true})
	skipped := insertRecurrence(t, ana, w, recRow{kind: "expense", description: "Apagada", start: "2026-01-01", amount: 100, day: 21, enabled: true})
	early := insertRecurrence(t, ana, w, recRow{kind: "expense", description: "Adiantada", start: "2026-01-01", amount: 100, day: 22, enabled: true})
	db := app.Server().DB
	if _, err := db.Exec(`INSERT INTO recurrence_skips (recurrence_id, occurrence) VALUES (?, '2026-11')`, skipped); err != nil {
		t.Fatal(err)
	}
	// an occurrence already present as a transaction (moved to an earlier date by a person) is not projected twice
	var cat int64
	db.QueryRow(`SELECT category_id FROM recurrences WHERE id = ?`, early).Scan(&cat) //nolint:errcheck,gosec // moved unchanged, see docs/security.md
	if _, err := db.Exec(`INSERT INTO transactions (wallet_id, kind, amount_cents, description, category_id, date, paid_by_user_id, created_by, recurrence_id, occurrence, source)
		VALUES (?, 'expense', 100, 'Adiantada', ?, '2026-11-02', ?, ?, ?, '2026-11', 'recurring')`, w, cat, ana.User.ID, ana.User.ID, early); err != nil {
		t.Fatal(err)
	}
	// another wallet's recurrence never shows
	var other struct {
		ID int64 `json:"id"`
	}
	ana.Do("POST", "/api/wallets", map[string]any{"name": "Casa"}, 201, &other)
	insertRecurrence(t, ana, other.ID, recRow{kind: "income", description: "Outra", start: "2026-01-01", amount: 7, day: 28, enabled: true})

	sum := getSummary(ana, "2026-11")
	if sum.UpcomingIncomeCents != 500000 || sum.UpcomingExpenseCents != 8990+100 {
		t.Errorf("upcoming totals: income %d, expense %d", sum.UpcomingIncomeCents, sum.UpcomingExpenseCents)
	}
	if sum.IncomeCents != 0 || sum.ExpenseCents != 100 {
		t.Errorf("realized totals must not include projections: income %d, expense %d", sum.IncomeCents, sum.ExpenseCents)
	}
	if len(sum.Upcoming) != 3 {
		t.Fatalf("upcoming: %+v", sum.Upcoming)
	}
	u0, u1, u2 := sum.Upcoming[0], sum.Upcoming[1], sum.Upcoming[2]
	if u0.RecurrenceID != salary15 || u0.Date != "2026-11-15" || u0.Kind != "income" || u0.AmountCents != 500000 || u0.Description != "Salário" {
		t.Errorf("first: %+v", u0)
	}
	if u1.Date != "2026-11-25" || u1.Description != "Curso" {
		t.Errorf("second: %+v", u1)
	}
	if u2.RecurrenceID != internet || u2.Date != "2026-11-30" || u2.Kind != "expense" || u2.CategoryID == nil || *u2.CategoryID != cat {
		t.Errorf("third (day 31 clamped to 30): %+v", u2)
	}
	if u2.Description == "" {
		t.Errorf("an empty description shows as the category name: %+v", u2)
	}
}

func TestSummaryUpcomingFutureAndPastMonths(t *testing.T) {
	_, ana := upcomingApp(t)
	w := ana.WalletID
	a := insertRecurrence(t, ana, w, recRow{kind: "income", description: "Salário", start: "2026-01-01", amount: 500000, day: 15, enabled: true})
	b := insertRecurrence(t, ana, w, recRow{kind: "income", description: "Salário", start: "2026-01-01", amount: 500000, day: 30, enabled: true})
	c := insertRecurrence(t, ana, w, recRow{kind: "expense", description: "Aluguel", start: "2026-01-01", amount: 200000, day: 5, enabled: true})
	insertRecurrence(t, ana, w, recRow{kind: "expense", description: "Acabou", start: "2026-01-01", end: "2026-11-30", amount: 1, day: 1, enabled: true})
	insertRecurrence(t, ana, w, recRow{kind: "expense", description: "Começa depois", start: "2027-01-01", amount: 1, day: 1, enabled: true})

	dec := getSummary(ana, "2026-12")
	if len(dec.Upcoming) != 3 || dec.Upcoming[0].RecurrenceID != c || dec.Upcoming[1].RecurrenceID != a || dec.Upcoming[2].RecurrenceID != b {
		t.Fatalf("December's whole schedule: %+v", dec.Upcoming)
	}
	if dec.UpcomingIncomeCents != 1000000 || dec.UpcomingExpenseCents != 200000 || dec.IncomeCents != 0 {
		t.Errorf("December totals: %+v", dec)
	}
	oct := getSummary(ana, "2026-10")
	if len(oct.Upcoming) != 0 || oct.UpcomingIncomeCents != 0 || oct.UpcomingExpenseCents != 0 {
		t.Errorf("a past month has nothing upcoming: %+v", oct)
	}
	if oct.Upcoming == nil {
		t.Error("upcoming should be [] and not null")
	}
}
