package api

import (
	"testing"
	"time"
)

// A transaction typed with a date after today is still to come: it stays out of every realized total and shows
// under upcoming with its transactionId, like a projected recurring occurrence. Today is frozen at 2026-11-14.
func TestSummaryFutureDatedTransactions(t *testing.T) {
	app, ana := upcomingApp(t)
	bia := app.register("Bia", "bia@example.com")
	shareWallet(ana, bia, "editor")
	ana.do("PATCH", ana.wpath(""), map[string]any{"splittingEnabled": true}, 200, nil)
	exp, inc := firstCategory(t, ana, "expense"), firstCategory(t, ana, "income")
	ana.do("PUT", ana.wpath("/budgets/%d", exp), map[string]any{"limitCents": 20000}, 200, nil)

	post := func(c *client, kind string, cat int64, cents int, desc, date string) int64 {
		t.Helper()
		var tx struct {
			ID int64 `json:"id"`
		}
		c.do("POST", c.wpath("/transactions"), map[string]any{
			"kind": kind, "amountCents": cents, "description": desc, "categoryId": cat, "date": date,
		}, 201, &tx)
		return tx.ID
	}
	post(ana, "expense", exp, 1000, "Hoje", "2026-11-14")                     // today: realized
	post(ana, "income", inc, 50000, "Salário", "2026-11-05")                  // realized
	later := post(ana, "expense", exp, 7000, "Conserto", "2026-11-20")        // later this month
	bonus := post(ana, "income", inc, 30000, "Bônus", "2026-11-20")           // later this month
	solar := post(bia, "expense", exp, 27500, "Placas solares", "2027-01-11") // a future month
	post(bia, "expense", exp, 4000, "Dezembro", "2026-12-03")                 // the month before January, all future
	ana.do("POST", ana.wpath("/settlements"), map[string]any{
		"fromUserId": bia.user.ID, "toUserId": ana.user.ID, "amountCents": 300, "date": "2026-11-30",
	}, 201, nil)
	// another wallet's future transaction never shows
	var other struct {
		ID int64 `json:"id"`
	}
	ana.do("POST", "/api/wallets", map[string]any{"name": "Casa"}, 201, &other)
	var otherCat int64
	if err := app.s.db.QueryRow(`SELECT id FROM categories WHERE wallet_id = ? AND kind = 'expense' ORDER BY id LIMIT 1`, other.ID).Scan(&otherCat); err != nil {
		t.Fatal(err)
	}
	ana.do("POST", "/api/wallets/"+itoa(other.ID)+"/transactions", map[string]any{
		"kind": "expense", "amountCents": 1, "description": "Outra", "categoryId": otherCat, "date": "2026-11-25",
	}, 201, nil)

	nov := getSummary(ana, "2026-11")
	if nov.IncomeCents != 50000 || nov.ExpenseCents != 1000 || nov.BalanceCents != 49000 {
		t.Errorf("November realized: income %d, expense %d, balance %d", nov.IncomeCents, nov.ExpenseCents, nov.BalanceCents)
	}
	if nov.DailyAverageCents != 1000/14 {
		t.Errorf("daily average: %d", nov.DailyAverageCents)
	}
	for _, c := range nov.ByCategory {
		if c.CategoryID == exp && c.Cents != 1000 || c.CategoryID == inc && c.Cents != 50000 {
			t.Errorf("by category (and budget usage) must be realized only: %+v", c)
		}
	}
	if s := nov.ByEssentiality["essential"] + nov.ByEssentiality["nonessential"] + nov.ByEssentiality["investment"]; s != 1000 {
		t.Errorf("by essentiality: %+v", nov.ByEssentiality)
	}
	if nov.UpcomingExpenseCents != 7000 || nov.UpcomingIncomeCents != 30000 {
		t.Errorf("November upcoming: income %d, expense %d", nov.UpcomingIncomeCents, nov.UpcomingExpenseCents)
	}
	if len(nov.Upcoming) != 2 || nov.Upcoming[0].TransactionID == nil || *nov.Upcoming[0].TransactionID != later ||
		nov.Upcoming[1].TransactionID == nil || *nov.Upcoming[1].TransactionID != bonus {
		t.Fatalf("November upcoming (by date then id): %+v", nov.Upcoming)
	}
	if u := nov.Upcoming[0]; u.Date != "2026-11-20" || u.Kind != "expense" || u.AmountCents != 7000 || u.Description != "Conserto" ||
		u.CategoryID == nil || *u.CategoryID != exp || u.RecurrenceID != 0 {
		t.Errorf("future-dated item: %+v", u)
	}

	// Only the expense dated today is split so far (500 each); the future expenses and the future settlement wait for their day.
	nets := map[int64]float64{}
	for _, b := range nov.MemberBalances.([]any) {
		m := b.(map[string]any)
		nets[int64(m["userId"].(float64))] = m["netCents"].(float64)
	}
	if nets[ana.user.ID] != 500 || nets[bia.user.ID] != -500 {
		t.Errorf("member balances must count only what is dated up to today: %+v", nets)
	}
	sug := nov.SuggestedSettlements.([]any)
	if len(sug) != 1 || sug[0].(map[string]any)["amountCents"].(float64) != 500 {
		t.Errorf("suggested settlements: %+v", sug)
	}

	jan := getSummary(ana, "2027-01")
	if jan.IncomeCents != 0 || jan.ExpenseCents != 0 || jan.BalanceCents != 0 || len(jan.ByCategory) != 1 || jan.ByCategory[0].Cents != 0 {
		t.Errorf("January realized must be empty (the budget row stays): %+v", jan)
	}
	if jan.Previous.ExpenseCents != 0 || jan.Previous.IncomeCents != 0 {
		t.Errorf("December has not closed yet: %+v", jan.Previous)
	}
	if jan.UpcomingExpenseCents != 27500 || len(jan.Upcoming) != 1 || *jan.Upcoming[0].TransactionID != solar || jan.Upcoming[0].Date != "2027-01-11" {
		t.Errorf("January upcoming: %+v", jan)
	}

	// the projected recurring occurrences keep no transactionId
	insertRecurrence(t, ana, ana.walletID, recRow{kind: "expense", description: "Internet", start: "2026-01-01", amount: 9000, day: 11, enabled: true})
	jan = getSummary(ana, "2027-01")
	if len(jan.Upcoming) != 2 || jan.UpcomingExpenseCents != 36500 {
		t.Fatalf("January with a recurrence on the same day: %+v", jan.Upcoming)
	}
	if jan.Upcoming[0].TransactionID == nil || *jan.Upcoming[0].TransactionID != solar || jan.Upcoming[1].TransactionID != nil || jan.Upcoming[1].RecurrenceID == 0 {
		t.Errorf("same day: the transaction (recurrenceId 0) first, then the projection without a transactionId: %+v", jan.Upcoming)
	}

	// once the day comes, everything counts
	app.s.now = func() time.Time { return time.Date(2026, 11, 30, 12, 0, 0, 0, app.s.cfg.Location) }
	nov = getSummary(ana, "2026-11")
	if nov.ExpenseCents != 8000 || nov.IncomeCents != 80000 || len(nov.Upcoming) != 0 {
		t.Errorf("November on its last day: %+v", nov)
	}
	nets = map[int64]float64{}
	for _, b := range nov.MemberBalances.([]any) {
		m := b.(map[string]any)
		nets[int64(m["userId"].(float64))] = m["netCents"].(float64)
	}
	if nets[ana.user.ID] != 3700 || nets[bia.user.ID] != -3700 { // Bia owes 500+3500 and settled 300 today
		t.Errorf("balances on the settlement's day: %+v", nets)
	}
}
