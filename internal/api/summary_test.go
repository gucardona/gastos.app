package api

import (
	"testing"
	"time"
)

func TestSummaryScenario(t *testing.T) {
	app := newTestApp(t)
	loc := app.s.cfg.Location
	app.s.now = func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, loc) }

	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	shareWallet(ana, bia, "editor")
	exp, inc := firstCategory(t, ana, "expense"), firstCategory(t, ana, "income")

	post := func(c *client, kind string, cat int64, cents int, date string) {
		t.Helper()
		c.do("POST", c.wpath("/transactions"), map[string]any{
			"kind": kind, "amountCents": cents, "description": "x", "categoryId": cat, "date": date,
		}, 201, nil)
	}

	post(ana, "expense", exp, 2000, "2026-09-10") // before enabling splitting: not part of the balances
	ana.do("PATCH", ana.wpath(""), map[string]any{"splittingEnabled": true}, 200, nil)
	post(ana, "expense", exp, 10000, "2026-10-02") // 50/50 → 5000 each
	post(bia, "expense", exp, 3001, "2026-10-03")  // 50/50 → Ana 1501, Bia 1500 (the lowest user_id gets the remainder)
	post(ana, "income", inc, 500000, "2026-10-05")
	ana.do("POST", ana.wpath("/settlements"), map[string]any{
		"fromUserId": bia.user.ID, "toUserId": ana.user.ID, "amountCents": 1000, "date": "2026-10-06",
	}, 201, nil)
	ana.do("PUT", ana.wpath("/budgets/%d", exp), map[string]any{"limitCents": 20000}, 200, nil)

	var sum summaryDTO
	ana.do("GET", ana.wpath("/summary?month=2026-10"), nil, 200, &sum)

	if sum.IncomeCents != 500000 || sum.ExpenseCents != 13001 || sum.BalanceCents != 486999 {
		t.Errorf("totais: %+v", sum)
	}
	if sum.Previous.ExpenseCents != 2000 || sum.Previous.IncomeCents != 0 {
		t.Errorf("previous month: %+v", sum.Previous)
	}
	if sum.DailyAverageCents != 13001/15 {
		t.Errorf("daily average: %d, want %d", sum.DailyAverageCents, 13001/15)
	}
	if len(sum.ByCategory) != 2 || sum.ByCategory[0].CategoryID != inc || sum.ByCategory[0].Cents != 500000 {
		t.Fatalf("por categoria: %+v", sum.ByCategory)
	}
	if c := sum.ByCategory[1]; c.CategoryID != exp || c.Cents != 13001 || c.BudgetCents == nil || *c.BudgetCents != 20000 {
		t.Errorf("categoria de gasto: %+v", c)
	}
	if sum.ByEssentiality["essential"]+sum.ByEssentiality["nonessential"]+sum.ByEssentiality["investment"] != 13001 {
		t.Errorf("por essencialidade: %+v", sum.ByEssentiality)
	}

	// Ana paid 10000 and owes 5000+1501; Bia paid 3001 and owes 5000+1500; Bia already settled 1000.
	balances, _ := sum.MemberBalances.([]any)
	if len(balances) != 2 {
		t.Fatalf("saldos: %+v", sum.MemberBalances)
	}
	nets := map[int64]float64{}
	for _, b := range balances {
		m := b.(map[string]any)
		nets[int64(m["userId"].(float64))] = m["netCents"].(float64)
	}
	if nets[ana.user.ID] != 2499 || nets[bia.user.ID] != -2499 {
		t.Errorf("saldos: %+v", nets)
	}
	sug, _ := sum.SuggestedSettlements.([]any)
	if len(sug) != 1 {
		t.Fatalf("acertos sugeridos: %+v", sum.SuggestedSettlements)
	}
	first := sug[0].(map[string]any)
	if int64(first["fromUserId"].(float64)) != bia.user.ID || int64(first["toUserId"].(float64)) != ana.user.ID || first["amountCents"].(float64) != 2499 {
		t.Errorf("acerto sugerido: %+v", first)
	}
}

func TestSummaryWithoutSplittingOmitsBalances(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	var raw map[string]any
	ana.do("GET", ana.wpath("/summary?month=2026-10"), nil, 200, &raw)
	if _, ok := raw["memberBalances"]; ok {
		t.Errorf("should not have memberBalances: %+v", raw)
	}
	if raw["byCategory"] == nil {
		t.Error("byCategory should be [] and not null")
	}
}

func TestSummaryValidation(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	ana.do("GET", ana.wpath("/summary"), nil, 400, nil)
	ana.do("GET", ana.wpath("/summary?month=2026-13"), nil, 400, nil)
}

func TestSummaryDailyAverageForPastAndFutureMonths(t *testing.T) {
	app := newTestApp(t)
	loc := app.s.cfg.Location
	app.s.now = func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, loc) }
	ana := app.register("Ana", "ana@example.com")
	ana.do("POST", ana.wpath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 3100, "description": "x", "categoryId": firstCategory(t, ana, "expense"), "date": "2026-08-10",
	}, 201, nil)
	var sum summaryDTO
	ana.do("GET", ana.wpath("/summary?month=2026-08"), nil, 200, &sum)
	if sum.DailyAverageCents != 100 { // 3100 / 31 days
		t.Errorf("past month: %d, want 100", sum.DailyAverageCents)
	}
	ana.do("GET", ana.wpath("/summary?month=2026-12"), nil, 200, &sum)
	if sum.DailyAverageCents != 0 {
		t.Errorf("future month: %d, want 0", sum.DailyAverageCents)
	}
}
