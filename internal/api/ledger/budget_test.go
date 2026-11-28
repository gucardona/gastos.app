package ledger_test

import (
	"testing"

	"gastos/internal/api/apitest"
	"gastos/internal/api/ledger"
)

func TestBudgets(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	cat := apitest.FirstCategory(t, ana, "expense")

	ana.Do("PUT", ana.WPath("/budgets/%d", cat), map[string]any{"limitCents": 100000}, 200, nil)
	ana.Do("PUT", ana.WPath("/budgets/%d", cat), map[string]any{"limitCents": 150000}, 200, nil)

	var list []ledger.BudgetDTO
	ana.Do("GET", ana.WPath("/budgets"), nil, 200, &list)
	if len(list) != 1 || list[0].LimitCents != 150000 {
		t.Fatalf("%+v", list)
	}

	ana.Do("PUT", ana.WPath("/budgets/%d", cat), map[string]any{"limitCents": 0}, 400, nil)
	ana.Do("PUT", ana.WPath("/budgets/%d", apitest.FirstCategory(t, ana, "income")), map[string]any{"limitCents": 100}, 400, nil)

	ana.Do("DELETE", ana.WPath("/budgets/%d", cat), nil, 204, nil)
	ana.Do("DELETE", ana.WPath("/budgets/%d", cat), nil, 404, nil)
}
