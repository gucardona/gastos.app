package ledger_test

import (
	"testing"
	"time"

	"gastos/internal/api/apitest"
	"gastos/internal/api/ledger"
)

func TestSummaryScenario(t *testing.T) {
	app := apitest.New(t)
	loc := app.Server().Cfg.Location
	app.SetNow(func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, loc) })

	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	apitest.ShareWallet(ana, bia, "editor")
	exp, inc := apitest.FirstCategory(t, ana, "expense"), apitest.FirstCategory(t, ana, "income")

	post := func(c *apitest.Client, kind string, cat int64, cents int, date string) {
		t.Helper()
		c.Do("POST", c.WPath("/transactions"), map[string]any{
			"kind": kind, "amountCents": cents, "description": "x", "categoryId": cat, "date": date,
		}, 201, nil)
	}

	post(ana, "expense", exp, 2000, "2026-09-10") // before enabling splitting: not part of the balances
	ana.Do("PATCH", ana.WPath(""), map[string]any{"splittingEnabled": true}, 200, nil)
	post(ana, "expense", exp, 10000, "2026-10-02") // 50/50 → 5000 each
	post(bia, "expense", exp, 3001, "2026-10-03")  // 50/50 → Ana 1501, Bia 1500 (the lowest user_id gets the remainder)
	post(ana, "income", inc, 500000, "2026-10-05")
	ana.Do("POST", ana.WPath("/settlements"), map[string]any{
		"fromUserId": bia.User.ID, "toUserId": ana.User.ID, "amountCents": 1000, "date": "2026-10-06",
	}, 201, nil)
	ana.Do("PUT", ana.WPath("/budgets/%d", exp), map[string]any{"limitCents": 20000}, 200, nil)

	var sum ledger.SummaryDTO
	ana.Do("GET", ana.WPath("/summary?month=2026-10"), nil, 200, &sum)

	if sum.IncomeCents != 500000 || sum.ExpenseCents != 13001 || sum.BalanceCents != 486999 {
		t.Errorf("totals: %+v", sum)
	}
	if sum.Previous.ExpenseCents != 2000 || sum.Previous.IncomeCents != 0 {
		t.Errorf("previous month: %+v", sum.Previous)
	}
	if sum.DailyAverageCents != 13001/15 {
		t.Errorf("daily average: %d, want %d", sum.DailyAverageCents, 13001/15)
	}
	if len(sum.ByCategory) != 2 || sum.ByCategory[0].CategoryID != inc || sum.ByCategory[0].Cents != 500000 {
		t.Fatalf("by category: %+v", sum.ByCategory)
	}
	if c := sum.ByCategory[1]; c.CategoryID != exp || c.Cents != 13001 || c.BudgetCents == nil || *c.BudgetCents != 20000 {
		t.Errorf("expense category: %+v", c)
	}
	if sum.ByEssentiality["essential"]+sum.ByEssentiality["nonessential"]+sum.ByEssentiality["investment"] != 13001 {
		t.Errorf("by essentiality: %+v", sum.ByEssentiality)
	}

	// Ana paid 10000 and owes 5000+1501; Bia paid 3001 and owes 5000+1500; Bia already settled 1000.
	balances, _ := sum.MemberBalances.([]any)
	if len(balances) != 2 {
		t.Fatalf("balances: %+v", sum.MemberBalances)
	}
	nets := map[int64]float64{}
	for _, b := range balances {
		m := b.(map[string]any)
		nets[int64(m["userId"].(float64))] = m["netCents"].(float64)
	}
	if nets[ana.User.ID] != 2499 || nets[bia.User.ID] != -2499 {
		t.Errorf("balances: %+v", nets)
	}
	sug, _ := sum.SuggestedSettlements.([]any)
	if len(sug) != 1 {
		t.Fatalf("suggested settlements: %+v", sum.SuggestedSettlements)
	}
	first := sug[0].(map[string]any)
	if int64(first["fromUserId"].(float64)) != bia.User.ID || int64(first["toUserId"].(float64)) != ana.User.ID || first["amountCents"].(float64) != 2499 {
		t.Errorf("suggested settlement: %+v", first)
	}
}

func TestSummaryWithoutSplittingOmitsBalances(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	var raw map[string]any
	ana.Do("GET", ana.WPath("/summary?month=2026-10"), nil, 200, &raw)
	if _, ok := raw["memberBalances"]; ok {
		t.Errorf("should not have memberBalances: %+v", raw)
	}
	if raw["byCategory"] == nil {
		t.Error("byCategory should be [] and not null")
	}
}

func TestSummaryValidation(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	ana.Do("GET", ana.WPath("/summary"), nil, 400, nil)
	ana.Do("GET", ana.WPath("/summary?month=2026-13"), nil, 400, nil)
}

func TestSummaryDailyAverageForPastAndFutureMonths(t *testing.T) {
	app := apitest.New(t)
	loc := app.Server().Cfg.Location
	app.SetNow(func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, loc) })
	ana := app.Register("Ana", "ana@example.com")
	ana.Do("POST", ana.WPath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 3100, "description": "x", "categoryId": apitest.FirstCategory(t, ana, "expense"), "date": "2026-08-10",
	}, 201, nil)
	var sum ledger.SummaryDTO
	ana.Do("GET", ana.WPath("/summary?month=2026-08"), nil, 200, &sum)
	if sum.DailyAverageCents != 100 { // 3100 / 31 days
		t.Errorf("past month: %d, want 100", sum.DailyAverageCents)
	}
	ana.Do("GET", ana.WPath("/summary?month=2026-12"), nil, 200, &sum)
	if sum.DailyAverageCents != 0 {
		t.Errorf("future month: %d, want 0", sum.DailyAverageCents)
	}
}

func TestMonthRange(t *testing.T) {
	for _, tc := range []struct{ month, from, to string }{
		{"2026-10", "2026-10-01", "2026-10-31"},
		{"2026-02", "2026-02-01", "2026-02-28"},
		{"2028-02", "2028-02-01", "2028-02-29"},
		{"2026-12", "2026-12-01", "2026-12-31"},
	} {
		from, to, ok := ledger.MonthRange(tc.month)
		if !ok || from != tc.from || to != tc.to {
			t.Errorf("ledger.MonthRange(%q) = %q, %q, %v", tc.month, from, to, ok)
		}
	}
	for _, bad := range []string{"", "2026-13", "2026-1", "outubro", "2026-10-01"} {
		if _, _, ok := ledger.MonthRange(bad); ok {
			t.Errorf("ledger.MonthRange(%q) should fail", bad)
		}
	}
}
