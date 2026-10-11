package api

import "testing"

func TestBudgets(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	cat := firstCategory(t, ana, "expense")

	ana.do("PUT", ana.wpath("/budgets/%d", cat), map[string]any{"limitCents": 100000}, 200, nil)
	ana.do("PUT", ana.wpath("/budgets/%d", cat), map[string]any{"limitCents": 150000}, 200, nil)

	var list []budgetDTO
	ana.do("GET", ana.wpath("/budgets"), nil, 200, &list)
	if len(list) != 1 || list[0].LimitCents != 150000 {
		t.Fatalf("%+v", list)
	}

	ana.do("PUT", ana.wpath("/budgets/%d", cat), map[string]any{"limitCents": 0}, 400, nil)
	ana.do("PUT", ana.wpath("/budgets/%d", firstCategory(t, ana, "income")), map[string]any{"limitCents": 100}, 400, nil)

	ana.do("DELETE", ana.wpath("/budgets/%d", cat), nil, 204, nil)
	ana.do("DELETE", ana.wpath("/budgets/%d", cat), nil, 404, nil)
}
